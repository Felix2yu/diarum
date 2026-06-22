import { writable, get } from 'svelte/store';
import { browser } from '$app/environment';
import type { Diary } from '$lib/api/client';
import {
	loadPersistedData,
	persistEntries,
	removePersistedEntry,
	removePersistedEntries,
	type PersistedEntry
} from './persistence';
import { checkOnlineStatus, initOnlineStatus } from './onlineStatus';

export interface CacheEntry {
	content: string;
	mood: number;
	mood_states: string[];
	weather: string;
	tags: string[];
	localUpdatedAt: number;
	serverUpdatedAt: string | null;
	isDirty: boolean;
}

interface DiaryCache {
	[date: string]: CacheEntry;
}

interface SyncState {
	isSyncing: boolean;
	currentDate: string | null;
	status: 'idle' | 'saving' | 'saved' | 'error';
	message: string;
}

export interface CacheStats {
	totalCached: number;
	pendingSync: number;
	entries: { date: string; isDirty: boolean; localUpdatedAt: number }[];
}

// Cache store
export const diaryCache = writable<DiaryCache>({});

// Global sync state
export const syncState = writable<SyncState>({
	isSyncing: false,
	currentDate: null,
	status: 'idle',
	message: ''
});

// Cache statistics store
export const cacheStats = writable<CacheStats>({
	totalCached: 0,
	pendingSync: 0,
	entries: []
});

// Sync timer
let syncTimer: ReturnType<typeof setTimeout> | null = null;
let persistTimer: ReturnType<typeof setTimeout> | null = null;
let initialized = false;
let cleanupOnlineStatus: (() => void) | null = null;
let storageEventHandler: ((e: StorageEvent) => void) | null = null;

// Retry state for offline sync
let retryCount = 0;
const MAX_RETRY_INTERVAL = 60000; // Max 60 seconds between retries
const BASE_RETRY_INTERVAL = 3000; // Start with 3 seconds
const AUTO_SAVE_DEBOUNCE_INTERVAL = 1000; // Save 1s after user stops typing
const MAX_RETRIES = 20; // 超过此次数后停止自动重试，改为手动

// Pending persistence queue
let pendingPersist: Map<string, PersistedEntry> = new Map();

// Storage key for cross-tab detection
const STORAGE_KEY = 'diarum_diary_cache';

/**
 * Reload cache from localStorage (for cross-tab sync)
 */
function reloadFromStorage(): void {
	const persisted = loadPersistedData();
	const cache: DiaryCache = {};
	const legacySyncedDates: string[] = [];

	for (const [date, entry] of Object.entries(persisted.entries)) {
		if (!entry.isDirty) {
			legacySyncedDates.push(date);
			continue;
		}

		cache[date] = {
			content: entry.content,
			mood: entry.mood || 0,
			mood_states: Array.isArray(entry.mood_states) ? entry.mood_states : [],
			weather: entry.weather || '',
			tags: Array.isArray(entry.tags) ? entry.tags : [],
			localUpdatedAt: entry.localUpdatedAt,
			serverUpdatedAt: entry.serverUpdatedAt,
			isDirty: entry.isDirty
		};
	}

	if (legacySyncedDates.length > 0) {
		removePersistedEntries(legacySyncedDates);
	}

	diaryCache.set(cache);
	updateCacheStats();
}

/**
 * Initialize the diary cache system
 */
export function initDiaryCache(): void {
	if (!browser || initialized) return;
	initialized = true;

	// Initialize dependencies
	cleanupOnlineStatus = initOnlineStatus();

	// Load persisted data
	reloadFromStorage();

	// Listen for storage changes from other tabs
	storageEventHandler = (e: StorageEvent) => {
		if (e.key === STORAGE_KEY) {
			// Flush pending writes before reloading to avoid data loss
			flushPendingPersist();
			reloadFromStorage();
		}
	};
	window.addEventListener('storage', storageEventHandler);
}

/**
 * Cleanup function for when app unmounts
 */
export function cleanupDiaryCache(): void {
	if (syncTimer) {
		clearTimeout(syncTimer);
		syncTimer = null;
	}
	if (persistTimer) {
		clearTimeout(persistTimer);
		persistTimer = null;
	}
	if (cleanupOnlineStatus) {
		cleanupOnlineStatus();
		cleanupOnlineStatus = null;
	}
	if (storageEventHandler) {
		window.removeEventListener('storage', storageEventHandler);
		storageEventHandler = null;
	}
	// Flush any pending persistence
	flushPendingPersist();
	retryCount = 0;
	initialized = false;
}

/**
 * Flush pending persistence immediately
 */
function flushPendingPersist(): void {
	if (pendingPersist.size > 0) {
		const entries = Array.from(pendingPersist.values());
		persistEntries(entries);
		pendingPersist.clear();
	}
}

/**
 * Debounced persistence - batches writes to localStorage
 */
function debouncedPersist(entry: PersistedEntry): void {
	pendingPersist.set(entry.date, entry);

	if (persistTimer) {
		clearTimeout(persistTimer);
	}

	persistTimer = setTimeout(() => {
		flushPendingPersist();
		persistTimer = null;
	}, 300); // 300ms debounce for persistence
}

/**
 * Update cache statistics
 */
function updateCacheStats(): void {
	const cache = get(diaryCache);
	const entries = Object.entries(cache).map(([date, entry]) => ({
		date,
		isDirty: entry.isDirty,
		localUpdatedAt: entry.localUpdatedAt
	}));

	// Sort by date descending
	entries.sort((a, b) => b.date.localeCompare(a.date));

	cacheStats.set({
		totalCached: entries.length,
		pendingSync: entries.filter(e => e.isDirty).length,
		entries
	});
}

/**
 * Get cached content for a date
 */
export function getCachedContent(date: string): CacheEntry | null {
	const cache = get(diaryCache);
	return cache[date] || null;
}

/**
 * Update local cache with edited content
 */
export function updateLocalCache(
	date: string,
	updates: { content: string; mood?: number; mood_states?: string[]; weather?: string; tags?: string[] }
): void {
	const existing = getCachedContent(date);

	const entry: CacheEntry = {
		content: updates.content,
		mood: updates.mood ?? existing?.mood ?? 0,
		mood_states: Array.isArray(updates.mood_states) ? updates.mood_states : existing?.mood_states ?? [],
		weather: updates.weather ?? existing?.weather ?? '',
		tags: Array.isArray(updates.tags) ? updates.tags : existing?.tags ?? [],
		localUpdatedAt: Date.now(),
		serverUpdatedAt: existing?.serverUpdatedAt || null,
		isDirty: true
	};

	diaryCache.update(cache => ({
		...cache,
		[date]: entry
	}));

	// Debounced persist to localStorage
	debouncedPersist({
		date,
		content: entry.content,
		mood: entry.mood,
		mood_states: entry.mood_states,
		weather: entry.weather,
		tags: entry.tags,
		localUpdatedAt: entry.localUpdatedAt,
		serverUpdatedAt: entry.serverUpdatedAt,
		isDirty: true
	});

	updateCacheStats();

	// Schedule sync
	scheduleSyncToServer();
}

/**
 * Update cache from server data
 */
export function updateFromServer(date: string, diary: Diary | null): void {
	const cache = get(diaryCache);
	const existing = cache[date];

	// If local cache exists and is dirty, keep local changes
	if (existing && existing.isDirty) {
		return;
	}

	// Browser cache is disabled: only keep unsynced local drafts.
	if (existing && !existing.isDirty) {
		clearCache(date);
	}
	if (!diary) {
		removePersistedEntry(date);
	}

	updateCacheStats();
}

/**
 * Check if date has dirty cache
 */
export function hasDirtyCache(date: string): boolean {
	const cache = get(diaryCache);
	return cache[date]?.isDirty || false;
}

/**
 * Get all dirty entries
 */
export function getDirtyEntries(): {
	date: string;
	content: string;
	mood: number;
	mood_states: string[];
	weather: string;
	tags: string[];
}[] {
	const cache = get(diaryCache);
	return Object.entries(cache)
		.filter(([_, entry]) => entry.isDirty)
		.map(([date, entry]) => ({
			date,
			content: entry.content,
			mood: entry.mood || 0,
			mood_states: entry.mood_states || [],
			weather: entry.weather || '',
			tags: entry.tags || []
		}));
}

/**
 * Mark entry as synced
 */
export function markAsSynced(date: string, serverUpdatedAt: string): void {
	void serverUpdatedAt;
	// Browser cache is disabled: once synced, remove local draft snapshot.
	clearCache(date);

	updateCacheStats();
}

/**
 * 判断失败原因是否为"可重试的网络/临时性错误"。
 * 权限错误、4xx 客户端错误等不应该重试，避免无限循环写入失败。
 */
function isRetryableError(error: unknown): boolean {
	if (!error) return false;

	// 导航器离线 → 明显的网络问题
	if (typeof navigator !== 'undefined' && navigator.onLine === false) return true;

	const message = error instanceof Error ? error.message : String(error);

	// 典型的网络/超时错误关键词
	const retryablePatterns = [
		'fetch',
		'Failed to fetch',
		'NetworkError',
		'network',
		'offline',
		'timeout',
		'Timeout',
		'ETIMEDOUT',
		'ECONNRESET',
		'ECONNREFUSED',
		'ENOTFOUND',
		'EAI_AGAIN',
		'502',
		'503',
		'504'
	];
	return retryablePatterns.some((p) => message.toLowerCase().includes(p.toLowerCase()));
}

/**
 * Schedule sync to server with exponential backoff for retries.
 * 若已达最大重试次数，则停止自动重试，仅在用户下一次编辑时恢复。
 */
function scheduleSyncToServer(isRetry: boolean = false): void {
	if (syncTimer) {
		clearTimeout(syncTimer);
		syncTimer = null;
	}

	// 超过最大重试次数后：停止自动重试，保持脏数据状态，由用户下次编辑触发。
	if (isRetry && retryCount >= MAX_RETRIES) {
		syncState.set({
			isSyncing: false,
			currentDate: null,
			status: 'error',
			message: '无法同步，请检查网络后手动重试'
		});
		return;
	}

	let interval = AUTO_SAVE_DEBOUNCE_INTERVAL;

	if (isRetry) {
		interval = Math.min(BASE_RETRY_INTERVAL * Math.pow(2, retryCount), MAX_RETRY_INTERVAL);
		retryCount++;
	} else {
		retryCount = 0; // Reset retry count for new edits
	}

	syncTimer = setTimeout(() => {
		syncDirtyEntries();
	}, interval);
}

/**
 * Sync all dirty entries to server.
 *
 * 行为调整：
 * 1) 单条日记同步失败，继续同步其他条目，不因为一条失败而阻塞全部
 * 2) 根据失败原因决定是否加入下次自动重试：
 *    - 网络/超时类错误 → 指数退避重试
 *    - 权限/认证/服务器业务错误 → 停止自动重试（由用户手动触发）
 */
async function syncDirtyEntries(): Promise<void> {
	const dirtyEntries = getDirtyEntries();

	if (dirtyEntries.length === 0) {
		syncState.set({
			isSyncing: false,
			currentDate: null,
			status: 'idle',
			message: ''
		});
		return;
	}

	// Check online status first
	const online = await checkOnlineStatus();
	if (!online) {
		syncState.set({
			isSyncing: false,
			currentDate: null,
			status: 'error',
			message: 'Offline'
		});
		// 明确的离线 → 总是可以重试
		scheduleSyncToServer(true);
		return;
	}

	retryCount = 0;

	syncState.set({
		isSyncing: true,
		currentDate: dirtyEntries[0].date,
		status: 'saving',
		message: 'Saving...'
	});

	const { saveDiary, getDiaryByDateResult } = await import('$lib/api/diaries');

	// 记录本次同步结果，用于决定是否需要整体重试
	let syncedCount = 0;
	let failedDates: string[] = [];
	let encounteredRetryable = false;
	let encounteredNonRetryable = false;
	let latestMessage = 'Saved';

	for (const entry of dirtyEntries) {
		try {
			const success = await saveDiary({
				date: entry.date,
				content: entry.content,
				mood: entry.mood,
				weather: entry.weather,
				tags: entry.tags
			});

			if (success) {
				// 服务器校验数据
				const serverState = await getDiaryByDateResult(entry.date);
				if (serverState.status === 'error') {
					// 保存成功但校验失败，保守处理为可重试
					failedDates.push(entry.date);
					encounteredRetryable = true;
					latestMessage = '部分日记同步失败，稍后重试';
					continue; // 继续同步其他条目
				}
				if (serverState.status === 'not_found') {
					clearCache(entry.date);
				} else {
					markAsSynced(entry.date, serverState.diary.updated || new Date().toISOString());
				}
				syncedCount++;
			} else {
				// saveDiary 返回 false：这一般是 4xx/5xx 类错误
				failedDates.push(entry.date);
				encounteredRetryable = true;
				latestMessage = '部分日记同步失败，稍后重试';
				// 继续同步其他条目
			}
		} catch (error) {
			console.error(`Failed to sync diary for ${entry.date}:`, error);
			failedDates.push(entry.date);

			if (isRetryableError(error)) {
				encounteredRetryable = true;
				latestMessage = '网络不稳定，稍后重试';
			} else {
				// 认证/权限等非重试类错误
				encounteredNonRetryable = true;
				latestMessage = '同步失败，请检查账户后手动重试';
			}
			// 继续同步其他条目
		}
	}

	// 全部成功
	if (failedDates.length === 0) {
		syncState.set({
			isSyncing: false,
			currentDate: null,
			status: 'saved',
			message: 'Saved'
		});

		setTimeout(() => {
			syncState.update(s => {
				if (s.status === 'saved') {
					return { ...s, status: 'idle', message: '' };
				}
				return s;
			});
		}, 2000);
		return;
	}

	// 部分失败：根据错误类型决定是否自动重试
	if (encounteredNonRetryable) {
		// 存在权限/业务错误，不自动重试，但保留脏数据
		syncState.set({
			isSyncing: false,
			currentDate: null,
			status: 'error',
			message: `有 ${failedDates.length} 条日记需要手动同步`
		});
		return;
	}

	if (encounteredRetryable) {
		// 仅网络类错误，加入退避重试
		syncState.set({
			isSyncing: false,
			currentDate: null,
			status: 'error',
			message: latestMessage
		});
		scheduleSyncToServer(true);
		return;
	}

	// 兜底
	syncState.set({
		isSyncing: false,
		currentDate: null,
		status: 'error',
		message: latestMessage
	});
	scheduleSyncToServer(true);
}

/**
 * Force sync immediately —— 用户手动触发的同步。
 * 单条失败不会终止，会继续尝试其他条目；最后返回整体结果。
 */
export async function forceSyncNow(): Promise<boolean> {
	if (syncTimer) {
		clearTimeout(syncTimer);
		syncTimer = null;
	}

	const dirtyEntries = getDirtyEntries();
	if (dirtyEntries.length === 0) return true;

	const online = await checkOnlineStatus();
	if (!online) {
		syncState.set({
			isSyncing: false,
			currentDate: null,
			status: 'error',
			message: 'Offline'
		});
		return false;
	}

	// 手动触发同步时，重置重试计数，给一次完整的尝试机会
	retryCount = 0;

	syncState.set({
		isSyncing: true,
		currentDate: dirtyEntries[0].date,
		status: 'saving',
		message: 'Saving...'
	});

	const { saveDiary, getDiaryByDateResult } = await import('$lib/api/diaries');

	let syncedCount = 0;
	let failedCount = 0;

	for (const entry of dirtyEntries) {
		try {
			const success = await saveDiary({
				date: entry.date,
				content: entry.content,
				mood: entry.mood,
				weather: entry.weather,
				tags: entry.tags
			});

			if (success) {
				const serverState = await getDiaryByDateResult(entry.date);
				if (serverState.status === 'not_found') {
					clearCache(entry.date);
					syncedCount++;
				} else if (serverState.status === 'error') {
					failedCount++;
					// 继续同步其他条目
				} else {
					markAsSynced(entry.date, serverState.diary.updated || new Date().toISOString());
					syncedCount++;
				}
			} else {
				failedCount++;
			}
		} catch (error) {
			console.error(`Failed to sync diary for ${entry.date}:`, error);
			failedCount++;
		}
	}

	const overallSuccess = failedCount === 0;

	syncState.set({
		isSyncing: false,
		currentDate: null,
		status: overallSuccess ? 'saved' : 'error',
		message: overallSuccess
			? 'Saved'
			: `已同步 ${syncedCount} 条，${failedCount} 条失败`
	});

	setTimeout(() => {
		syncState.update(s => {
			if (s.status === 'saved' || s.status === 'error') {
				return { ...s, status: 'idle', message: '' };
			}
			return s;
		});
	}, 2000);

	return overallSuccess;
}

/**
 * Clear cache for a specific date
 */
export function clearCache(date: string): void {
	diaryCache.update(cache => {
		const { [date]: _, ...rest } = cache;
		return rest;
	});
	removePersistedEntry(date);
	updateCacheStats();
}


import { browser } from '$app/environment';
import { get } from 'svelte/store';
import { reminderSettings } from '$lib/stores/notifications';
import {
	getVapidPublicKey,
	savePushSubscription,
	deletePushSubscription,
	syncReminderSchedule,
	sendTestNotification,
	type ReminderSettings
} from '$lib/api/notifications';

const SW_PATH = '/sw/notify.js';
const SW_SCOPE = '/sw/';

// Reflect browser permission state (shared with the UI).
export const notificationPermission = {
	current(): NotificationPermission {
		if (!browser || !('Notification' in window)) return 'denied';
		return Notification.permission;
	}
};

function isSupported(): boolean {
	return browser && 'Notification' in window && 'serviceWorker' in navigator;
}

/**
 * Safari 内核判定。macOS 的 Safari“网页 App”（文件 > 添加到程序坞）与普通
 * Safari 标签页在通知授权上有不同的交互要求，需要据此给出针对性提示。
 */
export function isSafari(): boolean {
	if (!browser) return false;
	const ua = navigator.userAgent;
	// Safari 可能伪装成 Chrome 以满足网站的 UA 嗅探，需要额外排除 Chromium 系。
	return /Safari/i.test(ua) && !/Chrome|Chromium|CriOS|Edg|OPR/i.test(ua);
}

/** 是否以独立（standalone）窗口运行，如 macOS Safari 网页 App 或已安装的 PWA。 */
function isStandalone(): boolean {
	if (!browser) return false;
	return window.matchMedia('(display-mode: standalone)').matches;
}

export type NotificationFailureReason =
	| 'unsupported'
	| 'not-secure'
	| 'denied'
	| 'not-granted'
	| 'worker'
	| 'subscribe'
	| 'server';

export interface EnableNotificationResult {
	ok: boolean;
	reason?: NotificationFailureReason;
	/** 授权操作后浏览器返回的最终权限状态。 */
	permission: NotificationPermission;
	/** 是否在独立窗口（网页 App / 已安装 PWA）中运行。 */
	standalone: boolean;
}

/**
 * 输出当前环境的通知相关诊断信息，用于定位 Safari「不弹授权框」等问题。
 * 失败时由 UI 附加到错误提示中，方便用户直接回传。
 */
export function notificationEnvironmentInfo(): string {
	if (!browser) return 'SSR';
	const parts = [
		`secure=${window.isSecureContext}`,
		`notification=${'Notification' in window}`,
		`permission=${'Notification' in window ? Notification.permission : 'n/a'}`,
		`sw=${'serviceWorker' in navigator}`,
		`standalone=${window.matchMedia('(display-mode: standalone)').matches}`,
		`displayMode=${window.matchMedia('(display-mode: standalone)').matches ? 'standalone' : 'browser'}`,
		`ua=${navigator.userAgent}`
	];
	return parts.join(' | ');
}

/**
 * 将失败原因映射为面向用户的中文提示。针对 macOS Safari 网页 App 场景，
 * 额外给出“在网页 App 窗口内授权 + 检查系统通知”的可执行指引。
 */
export function describeNotificationFailure(
	reason: NotificationFailureReason | undefined,
	standalone: boolean
): string {
	const macWebAppHint = isSafari()
		? ' 提示：请先到「Safari 设置 > 网站 > 通知」，确认勾选窗口底部的「允许网站请求发送通知的权限」——若未勾选，所有网站都静默拒绝、不弹框。勾选后刷新本页重试，若仍未弹框，再将本站点从「拒绝」改回「允许/询问」（若之前拒绝过，网页 App 会直接沿用拒绝而不再弹窗）。macOS 网页 App 的通知权限继承自 Safari：请在网页 App 自己的窗口（Safari「文件 > 添加到程序坞」安装，当前' +
		  (standalone ? '已以独立窗口运行' : '未以独立窗口运行，请先删除旧实例并用 Safari「文件 > 添加到程序坞」重新安装') +
		  '）里点击开启并回应授权，最后到「系统设置 > 通知」中允许该网页 App 发送通知。'
		: '';
	switch (reason) {
		case 'unsupported':
			return '当前浏览器不支持通知功能（需要支持 Notification 与 Service Worker）。' + macWebAppHint;
		case 'not-secure':
			return '当前环境不支持推送：通知需要 HTTPS（或 localhost）才能注册订阅，请通过 HTTPS 访问本站点。';
		case 'denied':
			return '通知权限已被拒绝，无法开启提醒。' + macWebAppHint;
		case 'not-granted':
			return '未获得通知权限：请在弹出的系统授权框中点击“允许”。若没有弹出授权框，' + (macWebAppHint || '请确认已授予权限后重试。');
		case 'worker':
			return '通知 Service Worker 注册失败，可能当前环境不支持 Service Worker。' + macWebAppHint;
		case 'subscribe':
			return '已获得通知权限，但注册推送订阅失败，请稍后重试或检查网络连接。' + macWebAppHint;
		case 'server':
			return '推送订阅已注册，但同步到服务器失败，请检查是否能正常访问本站点后重试。';
		default:
			return '未能开启通知，原因未知，请稍后重试。';
	}
}

/**
 * Register the dedicated reminder service worker (scope '/sw/') so it does not
 * replace the main workbox service worker (scope '/').
 */
export async function registerNotifyWorker(): Promise<ServiceWorkerRegistration | null> {
	if (!isSupported()) return null;
	if (!window.isSecureContext) return null;
	try {
		return await navigator.serviceWorker.register(SW_PATH, { scope: SW_SCOPE });
	} catch (e) {
		console.warn('[Notifications] failed to register notify worker:', e);
		return null;
	}
}

/**
 * Request notification permission and register a push subscription.
 * Returns a detailed result so the UI can show a targeted message instead of
 * collapsing every failure into one generic hint.
 */
export async function enableNotifications(): Promise<EnableNotificationResult> {
	const standalone = isStandalone();

	// 1. Environment support.
	if (!isSupported()) {
		return { ok: false, reason: 'unsupported', permission: 'denied', standalone };
	}
	if (!window.isSecureContext) {
		return { ok: false, reason: 'not-secure', permission: 'denied', standalone };
	}

	// 2. Request permission (must be triggered by a user gesture).
	let permission: NotificationPermission;
	try {
		permission = await Notification.requestPermission();
	} catch (e) {
		console.warn('[Notifications] requestPermission threw:', e);
		return { ok: false, reason: 'not-granted', permission: 'default', standalone };
	}
	if (permission !== 'granted') {
		return {
			ok: false,
			reason: permission === 'denied' ? 'denied' : 'not-granted',
			permission,
			standalone
		};
	}

	// 3. Register the dedicated reminder service worker.
	const registration = await registerNotifyWorker();
	if (!registration) {
		return { ok: false, reason: 'worker', permission, standalone };
	}

	// 4. Fetch the VAPID public key, then (re)subscribe to push.
	let publicKey: string;
	try {
		publicKey = await getVapidPublicKey();
	} catch (e) {
		console.warn('[Notifications] failed to fetch VAPID key:', e);
		return { ok: false, reason: 'server', permission, standalone };
	}

	let subscription: PushSubscription | null;
	try {
		subscription = await registration.pushManager.getSubscription();
		if (!subscription) {
			subscription = await registration.pushManager.subscribe({
				userVisibleOnly: true,
				applicationServerKey: urlBase64ToUint8Array(publicKey) as unknown as BufferSource
			});
		}
	} catch (e) {
		console.warn('[Notifications] subscription failed:', e);
		return { ok: false, reason: 'subscribe', permission, standalone };
	}
	if (!subscription) {
		return { ok: false, reason: 'subscribe', permission, standalone };
	}

	// 5. Persist the subscription on the server.
	try {
		await savePushSubscription(subscription);
	} catch (e) {
		console.warn('[Notifications] failed to save server subscription:', e);
		return { ok: false, reason: 'server', permission, standalone };
	}

	return { ok: true, permission, standalone };
}

/**
 * Disable notifications: unsubscribe from push and remove the server record.
 */
export async function disableNotifications(): Promise<void> {
	if (!isSupported()) return;

	try {
		const registration = await registerNotifyWorker();
		const subscription = registration ? await registration.pushManager.getSubscription() : null;
		if (subscription) {
			const endpoint = subscription.endpoint;
			await subscription.unsubscribe();
			try {
				await deletePushSubscription(endpoint);
			} catch (e) {
				console.warn('[Notifications] failed to delete server subscription:', e);
			}
		}
	} catch (e) {
		console.warn('[Notifications] failed to unsubscribe:', e);
	}
}

/**
 * Detect whether the current browser has an active push subscription.
 */
export async function hasActiveSubscription(): Promise<boolean> {
	if (!isSupported()) return false;
	try {
		const registration = await navigator.serviceWorker.getRegistration(SW_SCOPE);
		if (!registration) return false;
		const subscription = await registration.pushManager.getSubscription();
		return !!subscription;
	} catch {
		return false;
	}
}

/**
 * Persist reminder settings locally and mirror them to the backend scheduler.
 */
export async function saveReminderSettings(settings: ReminderSettings): Promise<void> {
	try {
		await syncReminderSchedule(settings);
	} catch (e) {
		console.warn('[Notifications] failed to sync schedule to backend:', e);
	}
}

/**
 * On app start, re-register the notify worker if reminders are enabled and the
 * browser already granted permission. This keeps push working after the browser
 * evicts idle service workers.
 */
export async function ensureNotifyWorker(): Promise<void> {
	if (!isSupported()) return;
	if (!get(reminderSettings).enabled) return;
	if (notificationPermission.current() !== 'granted') return;
	try {
		const reg = await navigator.serviceWorker.getRegistration(SW_SCOPE);
		if (!reg) {
			await registerNotifyWorker();
		}
	} catch (e) {
		console.warn('[Notifications] failed to re-register notify worker:', e);
	}
}

export async function sendTest(): Promise<void> {
	await sendTestNotification();
}

function urlBase64ToUint8Array(base64String: string): Uint8Array {
	const padding = '='.repeat((4 - (base64String.length % 4)) % 4);
	const base64 = (base64String + padding).replace(/-/g, '+').replace(/_/g, '/');
	const rawData = atob(base64);
	const outputArray = new Uint8Array(rawData.length);
	for (let i = 0; i < rawData.length; ++i) {
		outputArray[i] = rawData.charCodeAt(i);
	}
	return outputArray;
}

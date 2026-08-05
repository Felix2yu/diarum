import { writable, get } from 'svelte/store';
import { browser } from '$app/environment';
import type { ReminderSettings } from '$lib/api/notifications';

const STORAGE_KEY = 'diarum_reminder_settings';

export const DEFAULT_REMINDER_SETTINGS: ReminderSettings = {
	enabled: false,
	time: '21:00',
	timezone: '',
	message: '该写今天的日记啦 ✍️'
};

function detectTimezone(): string {
	if (!browser) return '';
	try {
		return Intl.DateTimeFormat().resolvedOptions().timeZone || '';
	} catch {
		return '';
	}
}

function loadSettings(): ReminderSettings {
	const defaults: ReminderSettings = {
		...DEFAULT_REMINDER_SETTINGS,
		timezone: detectTimezone()
	};
	if (!browser) return defaults;
	try {
		const raw = localStorage.getItem(STORAGE_KEY);
		if (!raw) return defaults;
		const parsed = JSON.parse(raw) as Partial<ReminderSettings>;
		return { ...defaults, ...parsed };
	} catch {
		return defaults;
	}
}

function persist(settings: ReminderSettings) {
	if (!browser) return;
	try {
		localStorage.setItem(STORAGE_KEY, JSON.stringify(settings));
	} catch {
		// ignore storage errors
	}
}

/**
 * A persistence-aware store: every update (including Svelte 5 property
 * bindings like `bind:value={$reminderSettings.time}`, which call .set/.update
 * directly) is mirrored to localStorage immediately. Without this, edits to the
 * reminder time/message that bypassed updateReminderSettings were never
 * persisted and reverted on the next page load ("保存后丢失").
 */
function createReminderSettingsStore() {
	const { subscribe, set, update } = writable<ReminderSettings>(loadSettings());

	function emit(next: ReminderSettings) {
		set(next);
		persist(next);
	}

	return {
		subscribe,
		set(value: ReminderSettings) {
			emit(value);
		},
		update(fn: (value: ReminderSettings) => ReminderSettings) {
			update((v) => {
				const next = fn(v);
				persist(next);
				return next;
			});
		}
	};
}

export const reminderSettings = createReminderSettingsStore();

export function getReminderSettings(): ReminderSettings {
	return get(reminderSettings);
}

export function updateReminderSettings(partial: Partial<ReminderSettings>): ReminderSettings {
	const next = { ...get(reminderSettings), ...partial };
	reminderSettings.set(next);
	return next;
}

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

export const reminderSettings = writable<ReminderSettings>(loadSettings());

export function getReminderSettings(): ReminderSettings {
	return get(reminderSettings);
}

export function updateReminderSettings(partial: Partial<ReminderSettings>): ReminderSettings {
	const next = { ...get(reminderSettings), ...partial };
	reminderSettings.set(next);
	persist(next);
	return next;
}

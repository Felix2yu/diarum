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
 * Returns true when the browser is fully subscribed.
 */
export async function enableNotifications(): Promise<boolean> {
	if (!isSupported()) return false;
	if (!window.isSecureContext) return false;

	const permission = await Notification.requestPermission();
	if (permission !== 'granted') return false;

	const registration = await registerNotifyWorker();
	if (!registration) return false;

	try {
		const publicKey = await getVapidPublicKey();
		let subscription = await registration.pushManager.getSubscription();
		if (!subscription) {
			subscription = await registration.pushManager.subscribe({
				userVisibleOnly: true,
				applicationServerKey: urlBase64ToUint8Array(publicKey) as unknown as BufferSource
			});
		}
		await savePushSubscription(subscription);
		return true;
	} catch (e) {
		console.warn('[Notifications] subscription failed:', e);
		return false;
	}
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

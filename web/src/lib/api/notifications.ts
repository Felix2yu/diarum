import { pb } from './client';

export interface ReminderSettings {
	enabled: boolean;
	time: string;
	timezone: string;
	message: string;
}

async function authFetch(url: string, init?: RequestInit): Promise<Response> {
	return fetch(url, {
		...init,
		headers: {
			...init?.headers,
			'Authorization': `Bearer ${pb.authStore.token}`
		}
	});
}

export async function getVapidPublicKey(): Promise<string> {
	const response = await authFetch('/api/v1/push/vapid-public-key');
	if (!response.ok) {
		throw new Error('Failed to get VAPID public key');
	}
	const data = await response.json();
	return data.public_key as string;
}

export async function savePushSubscription(subscription: PushSubscription): Promise<void> {
	const json = subscription.toJSON();
	const endpoint = json.endpoint ?? '';
	const keys = json.keys as { p256dh?: string; auth?: string } | undefined;
	const response = await authFetch('/api/v1/push/subscriptions', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({
			endpoint,
			keys: {
				p256dh: keys?.p256dh ?? '',
				auth: keys?.auth ?? ''
			}
		})
	});
	if (!response.ok) {
		const data = await response.json().catch(() => ({}));
		throw new Error(data.message || 'Failed to register notification subscription');
	}
}

export async function deletePushSubscription(endpoint: string): Promise<void> {
	if (!endpoint) return;
	const response = await authFetch('/api/v1/push/subscriptions', {
		method: 'DELETE',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ endpoint })
	});
	if (!response.ok) {
		const data = await response.json().catch(() => ({}));
		throw new Error(data.message || 'Failed to remove notification subscription');
	}
}

export async function sendTestNotification(): Promise<void> {
	const response = await authFetch('/api/v1/push/test', {
		method: 'POST'
	});
	if (!response.ok) {
		const data = await response.json().catch(() => ({}));
		throw new Error(data.message || 'Failed to send test notification');
	}
}

export async function syncReminderSchedule(settings: ReminderSettings): Promise<void> {
	const response = await authFetch('/api/v1/push/schedule', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(settings)
	});
	if (!response.ok) {
		const data = await response.json().catch(() => ({}));
		throw new Error(data.message || 'Failed to save reminder schedule');
	}
}

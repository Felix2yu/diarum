// PWA utility functions
import { writable } from 'svelte/store';
import { get } from 'svelte/store';
import { browser } from '$app/environment';

interface BeforeInstallPromptEvent extends Event {
	prompt(): Promise<void>;
	userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }>;
}

// Store for installation prompt
export const deferredPrompt = writable<BeforeInstallPromptEvent | null>(null);
export const canInstall = writable(false);
export const isUpdateAvailable = writable(false);

// Service worker registration (for skipWaiting on update)
let registration: ServiceWorkerRegistration | undefined;

/**
 * Register the service worker produced by @vite-pwa/sveltekit
 * (build-time generated via `strategies: 'generateSW'`).
 */
async function registerServiceWorker() {
	if (!browser) return;
	if (!('serviceWorker' in navigator)) return;

	try {
		// Import the virtual module exposed by vite-plugin-pwa.
		// The `registerSW` function registers `/sw.js` on the client side.
		const { registerSW } = await import('virtual:pwa-register');

		const updateSW = registerSW({
			immediate: true,
			onNeedRefresh() {
				// A new service worker is waiting to take control.
				isUpdateAvailable.set(true);
			},
			onOfflineReady() {
				// The service worker is installed and precaching is complete.
				console.log('[PWA] Offline ready');
			},
			onRegisterError(error) {
				console.error('[PWA] Service worker registration failed:', error);
			},
			onRegistered(r) {
				registration = r;

				// Periodically check for updates (every 60 minutes).
				// The browser also checks on navigation, but for long-lived apps
				// (installed PWA opened for hours) an explicit check helps.
				if (r) {
					setInterval(() => {
						try {
							r.update();
						} catch (e) {
							// ignore update errors
						}
					}, 60 * 60 * 1000);
				}
			}
		});

		// Expose a way for the UI to trigger an update reload.
		// We capture `updateSW` through closure so `applyUpdate` can call it.
		// Using a module-scoped variable keeps the API surface stable.
		(applyUpdate as unknown as { _updateSW?: typeof updateSW })._updateSW = updateSW;
	} catch (error) {
		console.error('[PWA] Failed to load service worker registration module:', error);
	}
}

// Initialize PWA features
export function initPWA() {
	if (!browser) return;

	// Start service worker registration
	registerServiceWorker();

	// Listen for beforeinstallprompt event
	window.addEventListener('beforeinstallprompt', (e) => {
		e.preventDefault();
		deferredPrompt.set(e as BeforeInstallPromptEvent);
		canInstall.set(true);
	});

	// Listen for app installed event
	window.addEventListener('appinstalled', () => {
		deferredPrompt.set(null);
		canInstall.set(false);
		console.log('PWA installed successfully');
	});

	// Check if app is already installed
	if (window.matchMedia('(display-mode: standalone)').matches) {
		canInstall.set(false);
	}
}

// Trigger installation prompt
export async function installPWA() {
	const prompt = get(deferredPrompt);

	if (!prompt) {
		console.log('Installation prompt not available');
		return false;
	}

	prompt.prompt();
	const { outcome } = await prompt.userChoice;

	if (outcome === 'accepted') {
		console.log('User accepted the install prompt');
		deferredPrompt.set(null);
		canInstall.set(false);
		return true;
	} else {
		console.log('User dismissed the install prompt');
		return false;
	}
}

// Reload to apply updates — activates the waiting service worker first.
export function applyUpdate() {
	isUpdateAvailable.set(false);

	// If registerSW gave us an updater, use it. Otherwise fall back to manual skipWaiting + reload.
	const updater = (applyUpdate as unknown as { _updateSW?: (_reload?: boolean) => Promise<void> })._updateSW;
	if (updater) {
		updater(true);
		return;
	}

	if (registration?.waiting) {
		// Ask the waiting worker to activate; reload when it takes control.
		const reloadOnce = () => {
			navigator.serviceWorker.removeEventListener('controllerchange', reloadOnce);
			window.location.reload();
		};
		navigator.serviceWorker.addEventListener('controllerchange', reloadOnce);
		registration.waiting.postMessage({ type: 'SKIP_WAITING' });
		return;
	}

	// Fallback: plain reload.
	window.location.reload();
}

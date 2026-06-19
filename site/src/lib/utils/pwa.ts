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
// 表示 applyUpdate 正在执行中，供 UI 显示 loading 状态
export const isApplyingUpdate = writable(false);

// Service worker registration (for skipWaiting on update)
let registration: ServiceWorkerRegistration | undefined;
// 定时更新检查器的句柄，确保不会重复创建
let periodicUpdateHandle: ReturnType<typeof setInterval> | undefined;
// 模块级 updater 引用，用于 applyUpdate 调用
let updateSWFn: ((reload?: boolean) => Promise<void>) | undefined;

/**
 * Register the service worker produced by @vite-pwa/sveltekit
 * (build-time generated via `strategies: 'generateSW'`).
 *
 * 注意：这里使用 `immediate: false`，让新的 Service Worker 进入 waiting 状态，
 * 由用户通过 "立即更新" 按钮手动激活，避免在编辑日记时被强制刷新打断。
 */
async function registerServiceWorker() {
	if (!browser) return;
	if (!window.isSecureContext) return;
	if (!('serviceWorker' in navigator)) return;

	try {
		// 清理可能存在的旧定时器，确保热更新或多次调用时不会叠加
		if (periodicUpdateHandle) {
			clearInterval(periodicUpdateHandle);
			periodicUpdateHandle = undefined;
		}

		// Import the virtual module exposed by vite-plugin-pwa.
		// The `registerSW` function registers `/sw.js` on the client side.
		const { registerSW } = await import('virtual:pwa-register');

		const updateSW = registerSW({
			immediate: false,
			onNeedRefresh() {
				// A new service worker is waiting to take control.
				isUpdateAvailable.set(true);
				console.log('[PWA] New version available, waiting for user action');
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
					periodicUpdateHandle = setInterval(() => {
						try {
							r.update();
						} catch (e) {
							// ignore update errors
							console.warn('[PWA] Periodic update check failed:', e);
						}
					}, 60 * 60 * 1000);
				}
			}
		});

		// Expose a way for the UI to trigger an update reload.
		updateSWFn = updateSW;
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

/**
 * 清理 PWA 相关资源（主要用于测试/调试场景，常规生产环境不需要主动调用）
 */
export function cleanupPWA() {
	if (periodicUpdateHandle) {
		clearInterval(periodicUpdateHandle);
		periodicUpdateHandle = undefined;
	}
	updateSWFn = undefined;
	registration = undefined;
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

/**
 * 应用更新 —— 先激活等待中的 Service Worker，再刷新页面。
 * 展示 loading 状态并确保整个流程异步、可追踪。
 */
export async function applyUpdate() {
	if (get(isApplyingUpdate)) return;

	isUpdateAvailable.set(false);
	isApplyingUpdate.set(true);

	try {
		if (updateSWFn) {
			// registerSW 提供的 updater 会处理 skipWaiting + reload。
			await updateSWFn(true);
			return;
		}

		if (registration?.waiting) {
			// Ask the waiting worker to activate; reload when it takes control.
			await new Promise<void>((resolve) => {
				const reloadOnce = () => {
					navigator.serviceWorker.removeEventListener('controllerchange', reloadOnce);
					resolve();
					window.location.reload();
				};
				navigator.serviceWorker.addEventListener('controllerchange', reloadOnce);
				registration!.waiting!.postMessage({ type: 'SKIP_WAITING' });

				// 3 秒兜底：若 waiting worker 迟迟未激活，直接刷新
				setTimeout(() => {
					navigator.serviceWorker.removeEventListener('controllerchange', reloadOnce);
					window.location.reload();
					resolve();
				}, 3000);
			});
			return;
		}

		// Fallback: plain reload.
		window.location.reload();
	} finally {
		isApplyingUpdate.set(false);
	}
}

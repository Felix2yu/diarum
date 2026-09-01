<script lang="ts">
	import '../app.css';
	import { onMount, type Snippet } from 'svelte';
	import { goto } from '$app/navigation';
	import { installUnauthorizedApiHandler } from '$lib/api/client';
	import { initTheme } from '$lib/stores/theme';
	import { initPWA } from '$lib/utils/pwa';
	import { ensureNotifyWorker } from '$lib/utils/notifications';
	import { initDiaryCache } from '$lib/stores/diaryCache';
	import PWAInstallPrompt from '$lib/components/PWAInstallPrompt.svelte';
	import PWAUpdatePrompt from '$lib/components/PWAUpdatePrompt.svelte';
	import OnlineStatusBanner from '$lib/components/OnlineStatusBanner.svelte';

	let { children }: { children: Snippet } = $props();

	function initFontSize() {
		const stored = localStorage.getItem('editor_font_size') as 'small' | 'medium' | 'large' | null;
		if (stored && ['small', 'medium', 'large'].includes(stored)) {
			const sizeMap = { small: '14px', medium: '16px', large: '18px' };
			document.documentElement.style.fontSize = sizeMap[stored];
		}
	}

	onMount(() => {
		initTheme();
		initFontSize();
		initPWA();
		initDiaryCache();
		ensureNotifyWorker();
		return installUnauthorizedApiHandler(() => {
			if (window.location.pathname !== '/login') {
				goto('/login');
			}
		});
	});
</script>

<a
	href="#main-content"
	class="sr-only focus:not-sr-only focus:fixed focus:top-safe focus:left-3 focus:z-[200] focus:rounded-md focus:bg-primary focus:px-4 focus:py-2 focus:text-primary-foreground focus:shadow-lg focus:outline-none focus:ring-2 focus:ring-ring focus-visible:outline-none"
>跳过导航，直接阅读内容</a
>
<OnlineStatusBanner />
{@render children()}
<PWAInstallPrompt />
<PWAUpdatePrompt />

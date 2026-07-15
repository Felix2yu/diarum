<script lang="ts">
	import '../app.css';
	import { onMount, type Snippet } from 'svelte';
	import { goto } from '$app/navigation';
	import { installUnauthorizedApiHandler } from '$lib/api/client';
	import { initTheme } from '$lib/stores/theme';
	import { initPWA } from '$lib/utils/pwa';
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
		return installUnauthorizedApiHandler(() => {
			if (window.location.pathname !== '/login') {
				goto('/login');
			}
		});
	});
</script>

<OnlineStatusBanner />
{@render children()}
<PWAInstallPrompt />
<PWAUpdatePrompt />

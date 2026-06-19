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

	let { children }: { children: Snippet } = $props();

	onMount(() => {
		initTheme();
		initPWA();
		initDiaryCache();
		return installUnauthorizedApiHandler(() => {
			if (window.location.pathname !== '/login') {
				goto('/login');
			}
		});
	});
</script>

{@render children()}

<PWAInstallPrompt />
<PWAUpdatePrompt />

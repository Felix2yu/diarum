<script lang="ts">
	import { onMount } from 'svelte';
	import ThemeToggle from './ThemeToggle.svelte';

	let {
		tagline = '',
		fullWidth = false
	}: {
		tagline?: string;
		fullWidth?: boolean;
	} = $props();

	let version = $state('');

	onMount(() => {
		fetchVersion();
	});

	async function fetchVersion() {
		try {
			const res = await fetch('/api/version');
			if (res.ok) {
				const data = await res.json();
				version = data.version;
			}
		} catch (e) {
			// Silently fail
		}
	}
</script>

<footer class="border-t border-border/50 mt-auto">
	<div class="container-responsive py-3">
		<div class="flex flex-row items-center justify-center sm:justify-between gap-2 sm:gap-4 flex-wrap">
			<div class="flex flex-wrap items-center justify-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
				{#if tagline}
					<span class="whitespace-nowrap">{tagline}</span>
				{/if}
				<span class="whitespace-nowrap">© {new Date().getFullYear()} 吾身</span>
				{#if version}
					<a href="https://github.com/songtianlun/diarum" target="_blank" rel="noopener noreferrer" class="font-mono text-[10px] text-muted-foreground/70 whitespace-nowrap hover:text-foreground transition-colors">{version}</a>
				{/if}
			</div>
			<ThemeToggle />
		</div>
	</div>
</footer>

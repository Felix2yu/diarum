<script lang="ts">
	import ThemeToggle from './ThemeToggle.svelte';

	let {
		tagline = '',
		fullWidth = false
	}: {
		tagline?: string;
		fullWidth?: boolean;
	} = $props();
</script>

<footer class="border-t border-border/50 mt-auto">
	<div class="container-responsive py-4">
		<div class="flex flex-row items-center justify-center sm:justify-between gap-2 sm:gap-4 flex-wrap">
			<div class="flex flex-wrap items-center justify-center gap-x-3 gap-y-1 text-xs text-muted-foreground/80">
				{#if tagline}
					<span class="whitespace-nowrap">{tagline}</span>
				{/if}
				<span class="whitespace-nowrap">© {new Date().getFullYear()} 吾身</span>
			</div>
			<ThemeToggle />
		</div>
	</div>
</footer>

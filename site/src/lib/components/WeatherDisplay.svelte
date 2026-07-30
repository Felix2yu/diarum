<script lang="ts">
	import { getWeatherInfo, formatWeatherDisplay } from '$lib/utils/weatherCodes';

	interface Props {
		wmoCode: number;
		tempMin?: number;
		tempMax?: number;
		showTemp?: boolean;
		size?: 'sm' | 'md' | 'lg';
	}

	let { wmoCode, tempMin, tempMax, showTemp = true, size = 'md' }: Props = $props();

	let info = $derived(getWeatherInfo(wmoCode));
	let displayText = $derived(formatWeatherDisplay(wmoCode, tempMin, tempMax));

	let sizeClasses = $derived(
		size === 'sm'
			? 'text-xs'
			: size === 'lg'
				? 'text-base'
				: 'text-sm'
	);
</script>

<div class="inline-flex items-center gap-1.5 {sizeClasses}">
	<span class="leading-none">{info.icon}</span>
	<span class="text-foreground">{info.label}</span>
	{#if showTemp && tempMin !== undefined && tempMax !== undefined}
		<span class="text-muted-foreground">
			{Math.round(tempMin)}°~{Math.round(tempMax)}°
		</span>
	{/if}
</div>

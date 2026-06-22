<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import Footer from '$lib/components/ui/Footer.svelte';
	import PageHeader from '$lib/components/ui/PageHeader.svelte';
	import { filterDiaries } from '$lib/api/diaries';
	import { isAuthenticated } from '$lib/api/client';
	import { formatDisplayDate, getDayOfWeek } from '$lib/utils/date';
	import { MOOD_SCALE, moodToEmoji, getMoodStatesForLevel, SCENARIO_OPTIONS } from '$lib/utils/diaryEmoji';

	interface FilterResult {
		id: string;
		date: string;
		snippet: string;
		mood: number;
		scenarios: string[];
		weather: string;
		tags: string[];
	}

	let selectedMood: number = 0;
	let selectedScenario = '';
	let results: FilterResult[] = [];
	let loading = false;
	let searched = false;

	function handleMoodSelect(mood: number) {
		selectedMood = selectedMood === mood ? 0 : mood;
		performFilter();
	}

	function handleScenarioSelect(scenario: string) {
		selectedScenario = selectedScenario === scenario ? '' : scenario;
		performFilter();
	}

	function clearAll() {
		selectedMood = 0;
		selectedScenario = '';
		results = [];
		searched = false;
	}

	async function performFilter() {
		if (selectedMood === 0 && !selectedScenario) {
			results = [];
			searched = false;
			return;
		}

		loading = true;
		searched = true;

		try {
			const data = await filterDiaries(
				selectedMood || undefined,
				selectedScenario || undefined
			);
			results = data.map((item: any) => ({
				id: item.id,
				date: item.date?.split(' ')[0] || item.date,
				snippet: (item.snippet || '').replace(/<[^>]*>/g, ' ').replace(/\s+/g, ' ').trim(),
				mood: item.mood || 0,
				scenarios: Array.isArray(item.scenarios) ? item.scenarios : [],
				weather: item.weather || '',
				tags: Array.isArray(item.tags) ? item.tags : []
			}));
		} catch (error) {
			console.error('Filter error:', error);
			results = [];
		} finally {
			loading = false;
		}
	}

	function navigateToDiary(date: string) {
		goto(`/diary/${date}`);
	}

	onMount(() => {
		if (!$isAuthenticated) {
			goto('/login');
			return;
		}
	});
</script>

<svelte:head>
	<title>心情筛选 - 吾身</title>
</svelte:head>

<div class="flex flex-col min-h-screen min-h-[100dvh] bg-background">
	<PageHeader title="心情筛选" />

	<main class="container-responsive py-8 flex-1">
		<!-- Header -->
		<div class="mb-8 animate-fade-in">
			<h1 class="text-2xl font-bold text-foreground mb-2">心情筛选</h1>
			<p class="text-sm text-muted-foreground">按心情档位和情景筛选日记</p>
		</div>

		<!-- Mood Slider -->
		<div class="bg-card rounded-xl shadow-sm border border-border/50 p-5 mb-4 animate-fade-in stagger-1">
			<div class="flex items-center justify-between mb-3">
				<div class="text-sm font-semibold text-foreground">心情档位</div>
				{#if selectedMood > 0 || selectedScenario}
					<button
						onclick={clearAll}
						class="text-[11px] px-2 py-1 rounded-full bg-muted/70 hover:bg-muted border border-border/70 transition-colors text-muted-foreground"
					>
						清除全部
					</button>
				{/if}
			</div>
			<div class="mood-slider-container">
				<div class="mood-slider-track">
					{#each MOOD_SCALE as level}
						<button
							onclick={() => handleMoodSelect(level.value)}
							class="mood-slider-stop {selectedMood === level.value ? 'mood-slider-stop-active' : ''} {selectedMood > 0 && level.value <= selectedMood ? 'mood-slider-stop-filled' : ''}"
							title={level.label}
							aria-label={`心情 ${level.label}`}
						>
							<span class="mood-slider-emoji">{level.emoji}</span>
						</button>
					{/each}
				</div>
				{#if selectedMood > 0}
					<div class="text-center text-xs text-muted-foreground mt-2">
						{moodToEmoji(selectedMood)} {MOOD_SCALE.find(l => l.value === selectedMood)?.label || ''}
					</div>
				{/if}
			</div>
		</div>

		<!-- Scenario Filters -->
		<div class="bg-card rounded-xl shadow-sm border border-border/50 p-5 mb-6 animate-fade-in stagger-2">
			<div class="text-sm font-semibold text-foreground mb-3">情景</div>
			<div class="flex flex-wrap gap-1.5">
				{#each SCENARIO_OPTIONS as scenario}
					<button
						onclick={() => handleScenarioSelect(scenario)}
						class="mood-state-chip {selectedScenario === scenario ? 'mood-state-chip-active' : ''}"
					>
						{scenario}
					</button>
				{/each}
			</div>
		</div>

		<!-- Results -->
		{#if loading}
			<div class="flex flex-col items-center justify-center py-12 gap-3 animate-fade-in">
				<svg class="w-6 h-6 animate-spin text-primary" fill="none" viewBox="0 0 24 24">
					<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
					<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
				</svg>
				<div class="text-muted-foreground text-sm">正在筛选...</div>
			</div>
		{:else if searched && results.length === 0}
			<div class="flex flex-col items-center justify-center py-12 gap-4 animate-fade-in">
				<div class="w-16 h-16 rounded-full bg-muted/50 flex items-center justify-center">
					<svg class="w-8 h-8 text-muted-foreground" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5"
							d="M9.172 16.172a4 4 0 015.656 0M9 10h.01M15 10h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
					</svg>
				</div>
				<div class="text-center">
					<p class="text-foreground font-medium">未找到匹配的日记</p>
					<p class="text-sm text-muted-foreground mt-1">试试其他心情档位或情景</p>
				</div>
			</div>
		{:else if searched && results.length > 0}
			<div class="mb-4 text-sm text-muted-foreground animate-fade-in">
				找到 {results.length} 条匹配的日记
			</div>
			<div class="space-y-3">
				{#each results as result (result.id)}
					<button
						onclick={() => navigateToDiary(result.date)}
						class="w-full text-left bg-card rounded-xl shadow-sm border border-border/50 p-4 hover:shadow-md hover:border-primary/30 transition-all duration-200 group animate-fade-in"
					>
						<div class="flex items-center justify-between mb-2">
							<div class="flex items-center gap-2">
								<span class="text-sm font-medium text-foreground">
									{formatDisplayDate(result.date)}
								</span>
								<span class="text-xs text-muted-foreground">周{getDayOfWeek(result.date)}</span>
								{#if result.mood}
									<span class="text-sm">{moodToEmoji(result.mood)}</span>
								{/if}
								{#if result.weather}
									<span class="text-sm">{result.weather}</span>
								{/if}
							</div>
							<svg class="w-4 h-4 text-muted-foreground group-hover:text-foreground group-hover:translate-x-0.5 transition-all" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
							</svg>
						</div>
						{#if result.scenarios && result.scenarios.length > 0}
							<div class="flex flex-wrap gap-1 mb-2">
								{#each result.scenarios as scenario}
									<span class="text-[10px] px-1.5 py-0.5 bg-primary/10 text-primary rounded-full">{scenario}</span>
								{/each}
							</div>
						{/if}
						<p class="text-sm text-muted-foreground leading-relaxed line-clamp-2">
							{result.snippet}
						</p>
					</button>
				{/each}
			</div>
		{:else}
			<div class="flex flex-col items-center justify-center py-12 gap-4 animate-fade-in">
				<div class="w-16 h-16 rounded-full bg-muted/50 flex items-center justify-center">
					<svg class="w-8 h-8 text-muted-foreground" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5"
							d="M3 4a1 1 0 011-1h16a1 1 0 011 1v2.586a1 1 0 01-.293.707l-6.414 6.414a1 1 0 00-.293.707V17l-4 4v-6.586a1 1 0 00-.293-.707L3.293 7.293A1 1 0 013 6.586V4z" />
					</svg>
				</div>
				<div class="text-center">
					<p class="text-foreground font-medium">选择心情或情景开始筛选</p>
					<p class="text-sm text-muted-foreground mt-1">点击上方的心情档位或情景标签</p>
				</div>
			</div>
		{/if}
	</main>

	<Footer />
</div>

<style>
	.mood-slider-container {
		padding: 0.25rem 0;
	}

	.mood-slider-track {
		display: flex;
		align-items: center;
		justify-content: space-between;
		position: relative;
		padding: 0.25rem 0;
	}

	.mood-slider-track::before {
		content: '';
		position: absolute;
		top: 50%;
		left: 0;
		right: 0;
		height: 3px;
		background: hsl(var(--border) / 0.6);
		border-radius: 2px;
		transform: translateY(-50%);
		z-index: 0;
	}

	.mood-slider-stop {
		position: relative;
		z-index: 1;
		display: flex;
		align-items: center;
		justify-content: center;
		width: 2.5rem;
		height: 2.5rem;
		border-radius: 50%;
		border: 2px solid hsl(var(--border) / 0.6);
		background: hsl(var(--background));
		cursor: pointer;
		transition: all 0.2s ease;
	}

	.mood-slider-stop:hover {
		transform: scale(1.15);
		border-color: hsl(var(--primary) / 0.4);
	}

	.mood-slider-stop-active {
		border-color: hsl(var(--primary));
		background: hsl(var(--primary) / 0.15);
		box-shadow: 0 0 0 3px hsl(var(--primary) / 0.2);
		transform: scale(1.1);
	}

	.mood-slider-stop-filled {
		background: hsl(var(--primary) / 0.08);
	}

	.mood-slider-emoji {
		font-size: 1.25rem;
		line-height: 1;
	}

	:global(.mood-state-chip) {
		display: inline-flex;
		align-items: center;
		padding: 0.3rem 0.65rem;
		border-radius: 9999px;
		border: 1px solid hsl(var(--border) / 0.5);
		background: hsl(var(--muted) / 0.3);
		font-size: 0.75rem;
		color: hsl(var(--muted-foreground));
		cursor: pointer;
		transition: all 0.15s ease;
		white-space: nowrap;
	}

	:global(.mood-state-chip:hover) {
		background: hsl(var(--muted) / 0.6);
		border-color: hsl(var(--primary) / 0.3);
	}

	:global(.mood-state-chip-active) {
		background: hsl(var(--primary) / 0.12);
		border-color: hsl(var(--primary) / 0.5);
		color: hsl(var(--primary));
		font-weight: 500;
	}

	.line-clamp-2 {
		display: -webkit-box;
		-webkit-line-clamp: 2;
		line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
	}
</style>

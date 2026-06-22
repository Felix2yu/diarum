<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import Footer from '$lib/components/ui/Footer.svelte';
	import PageHeader from '$lib/components/ui/PageHeader.svelte';
	import { searchDiaries } from '$lib/api/diaries';
	import { isAuthenticated } from '$lib/api/client';
	import { formatDisplayDate, formatShortDate, getDayOfWeek } from '$lib/utils/date';
	import { moodToEmoji } from '$lib/utils/diaryEmoji';

	interface SearchResult {
		id: string;
		date: string;
		snippet: string;
		mood?: number;
		scenarios?: string[];
		weather?: string;
		tags?: string[];
	}

	let query = '';
	let results: SearchResult[] = [];
	let loading = false;
	let searched = false;
	let searchTimeout: ReturnType<typeof setTimeout>;
	let inputElement: HTMLInputElement;

	function handleInput() {
		clearTimeout(searchTimeout);
		if (query.trim().length === 0) {
			results = [];
			searched = false;
			return;
		}
		if (query.trim().length < 2) {
			return;
		}
		searchTimeout = setTimeout(() => {
			performSearch();
		}, 300);
	}

	async function performSearch() {
		if (query.trim().length < 2) return;

		loading = true;
		searched = true;

		try {
			const data = await searchDiaries(query.trim());
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
			console.error('Search error:', error);
			results = [];
		} finally {
			loading = false;
		}
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter' && query.trim().length >= 2) {
			clearTimeout(searchTimeout);
			performSearch();
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
		const q = $page.url.searchParams.get('q');
		if (q) {
			query = q;
			performSearch();
		} else {
			inputElement?.focus();
		}
	});
</script>

<svelte:head>
	<title>搜索 - 吾身</title>
</svelte:head>

<div class="flex flex-col min-h-screen min-h-[100dvh] bg-background">
	<PageHeader title="搜索" />

	<main class="container-responsive py-8 flex-1">
		<div class="mb-8 animate-fade-in">
			<h1 class="text-2xl font-bold text-foreground mb-2">搜索日记</h1>
			<p class="text-sm text-muted-foreground">通过关键词查找你的日记条目</p>
		</div>

		<!-- Search Input -->
		<div class="relative mb-6 animate-fade-in stagger-1">
			<div class="absolute inset-y-0 left-0 pl-4 flex items-center pointer-events-none">
				<svg class="w-5 h-5 text-muted-foreground" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
						d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
				</svg>
			</div>
			<input
				bind:this={inputElement}
				bind:value={query}
				oninput={handleInput}
				onkeydown={handleKeydown}
				type="text"
				placeholder="搜索你的日记..."
				class="w-full pl-12 pr-4 py-3.5 bg-card border border-border/50 rounded-xl text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring/50 focus:border-transparent shadow-sm transition-all duration-200"
			/>
			{#if query.length > 0}
				<button
					onclick={() => { query = ''; results = []; searched = false; inputElement?.focus(); }}
					class="absolute inset-y-0 right-0 pr-4 flex items-center text-muted-foreground hover:text-foreground transition-colors"
					title="清空搜索"
				>
					<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
					</svg>
				</button>
			{/if}
		</div>

		<!-- Loading State -->
		{#if loading}
			<div class="flex flex-col items-center justify-center py-12 gap-3 animate-fade-in">
				<svg class="w-6 h-6 animate-spin text-primary" fill="none" viewBox="0 0 24 24">
					<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
					<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
				</svg>
				<div class="text-muted-foreground text-sm">正在搜索...</div>
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
					<p class="text-sm text-muted-foreground mt-1">试试其他关键词</p>
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
							d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
					</svg>
				</div>
				<div class="text-center">
					<p class="text-foreground font-medium">输入关键词搜索日记</p>
					<p class="text-sm text-muted-foreground mt-1">支持全文内容搜索</p>
				</div>
			</div>
		{/if}
	</main>

	<Footer />
</div>

<style>
	.line-clamp-2 {
		display: -webkit-box;
		-webkit-line-clamp: 2;
		line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
	}
</style>

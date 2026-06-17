<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import Calendar from '$lib/components/calendar/Calendar.svelte';
	import Footer from '$lib/components/ui/Footer.svelte';
	import PageHeader from '$lib/components/ui/PageHeader.svelte';
	import { getDatesWithDiaries, getRecentDiaries, getDiaryStats, type CalendarDiaryMeta } from '$lib/api/diaries';
	import { isAuthenticated } from '$lib/api/client';
	import { getMonthRange, formatDisplayDate } from '$lib/utils/date';

	let currentYear = new Date().getFullYear();
	let currentMonth = new Date().getMonth() + 1;
	let diaryMeta: CalendarDiaryMeta[] = [];
	let recentDiaries: Array<{ date: string; content: string }> = [];
	let stats: { streak: number; total: number } | null = null;
	let loading = true;
	let recentLoading = true;
	let statsLoading = true;
	let mounted = false;
	let prevYear = currentYear;
	let prevMonth = currentMonth;
	let yearViewActive = false;
	let yearDiaryMeta: CalendarDiaryMeta[] = [];

	async function loadDatesWithDiaries() {
		loading = true;
		const range = getMonthRange(currentYear, currentMonth);
		diaryMeta = await getDatesWithDiaries(range.start, range.end);
		loading = false;
	}
	$: datesWithDiaries = diaryMeta.map(item => item.date);


	async function loadRecentDiaries() {
		recentLoading = true;
		try {
			recentDiaries = await getRecentDiaries(5);
		} catch (e) {
			recentDiaries = [];
		}
		recentLoading = false;
	}

	async function loadStats() {
		statsLoading = true;
		stats = await getDiaryStats();
		statsLoading = false;
	}

	function getPreview(content: string): string {
		const text = content.replace(/<[^>]*>/g, '').trim();
		return text.length > 80 ? text.slice(0, 80) + '...' : text;
	}

	onMount(() => {
		if (!$isAuthenticated) {
			goto('/login');
			return;
		}
		loadDatesWithDiaries();
		loadRecentDiaries();
		loadStats();
		mounted = true;
	});

	$: {
		if (mounted && (currentYear !== prevYear || currentMonth !== prevMonth)) {
			prevYear = currentYear;
			prevMonth = currentMonth;
			if (!yearViewActive) {
				loadDatesWithDiaries();
			}
		}
	}
</script>

<svelte:head>
	<title>日历 - 吾身</title>
</svelte:head>

<div class="min-h-screen bg-background">
	<PageHeader title="日历" />

	<!-- Calendar -->
	<main class="container-responsive py-6">
		<div class="flex flex-col lg:flex-row gap-6 lg:h-[540px]">
			<!-- Left: Calendar -->
			<div class="lg:flex-1 lg:min-w-0">
				<div class="bg-card rounded-xl shadow-sm border border-border/50 p-5 h-full relative overflow-hidden">
					{#if loading}
						<div class="absolute inset-0 flex flex-col items-center justify-center gap-3">
							<svg class="w-6 h-6 animate-spin text-primary" fill="none" viewBox="0 0 24 24">
								<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
								<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
							</svg>
							<div class="text-muted-foreground text-sm">加载中...</div>
						</div>
					{:else}
						<div class="animate-fade-in-only">
							<Calendar bind:currentYear bind:currentMonth bind:yearViewActive bind:yearDiaryMeta {diaryMeta} onmonthchange={loadDatesWithDiaries} />
						</div>
					{/if}
				</div>
			</div>

			<!-- Right: Stats and Recent Entries -->
			<div class="lg:w-[340px] xl:w-[380px] flex flex-col gap-4 flex-shrink-0">
				<!-- Stats -->
				<div class="grid grid-cols-3 gap-4">
					<div class="bg-card rounded-xl shadow-sm border border-border/50 p-4">
						<div class="text-xs text-muted-foreground">{yearViewActive ? '今年' : '本月'}</div>
						<div class="text-xl font-bold text-foreground mt-1 h-7 flex items-center">
							{#if yearViewActive}
								<span class="animate-fade-in-only">{yearDiaryMeta.length}</span>
							{:else if loading}
								<span class="inline-block w-4 h-4 border-2 border-primary/30 border-t-primary rounded-full animate-spin"></span>
							{:else}
								<span class="animate-fade-in-only">{datesWithDiaries.length}</span>
							{/if}
						</div>
					</div>

					<div class="bg-card rounded-xl shadow-sm border border-border/50 p-4">
						<div class="text-xs text-muted-foreground">连续天数</div>
						<div class="text-xl font-bold text-foreground mt-1 h-7 flex items-center">
							{#if statsLoading}
								<span class="inline-block w-4 h-4 border-2 border-primary/30 border-t-primary rounded-full animate-spin"></span>
							{:else}
								<span class="animate-fade-in-only">{stats?.streak ?? 0}</span>
							{/if}
						</div>
					</div>

					<div class="bg-card rounded-xl shadow-sm border border-border/50 p-4">
						<div class="text-xs text-muted-foreground">总计</div>
						<div class="text-xl font-bold text-foreground mt-1 h-7 flex items-center">
							{#if statsLoading}
								<span class="inline-block w-4 h-4 border-2 border-primary/30 border-t-primary rounded-full animate-spin"></span>
							{:else}
								<span class="animate-fade-in-only">{stats?.total ?? 0}</span>
							{/if}
						</div>
					</div>
				</div>

				<!-- Recent Entries -->
				<div class="bg-card rounded-xl shadow-sm border border-border/50 p-4 flex-1 min-h-0 flex flex-col overflow-hidden">
					<h3 class="text-sm font-medium text-foreground mb-3">最近条目</h3>
					{#if recentLoading}
						<div class="flex-1 flex flex-col items-center justify-center gap-3">
							<svg class="w-6 h-6 animate-spin text-primary" fill="none" viewBox="0 0 24 24">
								<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
								<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
							</svg>
							<div class="text-muted-foreground text-sm">加载中...</div>
						</div>
					{:else if recentDiaries.length > 0}
						<div class="space-y-2 overflow-y-auto flex-1 animate-fade-in-only">
							{#each recentDiaries as diary}
								<a
									href="/diary/{diary.date}"
									class="block p-3 rounded-lg hover:bg-muted/50 transition-colors border border-border/30"
								>
									<div class="text-xs text-muted-foreground mb-1">
										{formatDisplayDate(diary.date)}
									</div>
									<div class="text-sm text-foreground line-clamp-2">
										{getPreview(diary.content)}
									</div>
								</a>
							{/each}
						</div>
					{:else}
						<div class="flex-1 flex items-center justify-center animate-fade-in-only">
							<div class="text-sm text-muted-foreground text-center">
								还没有条目。今天就开始记录吧！
							</div>
						</div>
					{/if}
				</div>
			</div>
		</div>
	</main>

	<!-- Footer -->
	<Footer tagline="记录生活的点滴" />
</div>

<script lang="ts">
	import { onMount } from 'svelte';
	import PageHeader from '$lib/components/ui/PageHeader.svelte';
	import Footer from '$lib/components/ui/Footer.svelte';
	import { isAuthenticated } from '$lib/api/client';
	import { getTagCloud, getDiariesByTag } from '$lib/api/diaries';
	import type { Diary } from '$lib/api/client';
	import { formatDisplayDate } from '$lib/utils/date';
	import { goto } from '$app/navigation';

	interface TagCount {
		tag: string;
		count: number;
	}

	let loading = true;
	let tags: TagCount[] = [];
	let totalDiaryCount = 0;
	let selectedTag: string | null = null;
	let tagDiaries: Diary[] = [];
	let loadingTag = false;
	let sortBy: 'count' | 'name' = 'count';

	async function loadTags() {
		loading = true;
		try {
			tags = await getTagCloud();
			totalDiaryCount = tags.reduce((sum, t) => sum + t.count, 0);
		} catch (err) {
			console.error('Failed to load tag cloud:', err);
		} finally {
			loading = false;
		}
	}

	async function loadDiariesForTag(tag: string) {
		if (selectedTag === tag) {
			clearTagFilter();
			return;
		}
		selectedTag = tag;
		loadingTag = true;
		try {
			tagDiaries = await getDiariesByTag(tag);
		} catch (err) {
			console.error('Failed to load diaries for tag:', err);
			tagDiaries = [];
		} finally {
			loadingTag = false;
		}
	}

	function clearTagFilter() {
		selectedTag = null;
		tagDiaries = [];
	}

	function goToDiary(date: string) {
		goto(`/diary/${date}`);
	}

	function getTagOpacity(count: number, max: number): number {
		if (max <= 0) return 0.5;
		const ratio = count / max;
		return 0.4 + ratio * 0.6;
	}

	function getTagScale(count: number, max: number): string {
		if (max <= 0) return '1';
		const ratio = count / max;
		return (0.85 + ratio * 0.35).toFixed(2);
	}

	function getTagBg(count: number, max: number): string {
		const ratio = max > 0 ? count / max : 0;
		if (ratio > 0.7) return 'bg-primary/15 border-primary/30';
		if (ratio > 0.4) return 'bg-primary/10 border-primary/20';
		return 'bg-muted/50 border-border/50';
	}

	function getTagText(count: number, max: number): string {
		const ratio = max > 0 ? count / max : 0;
		if (ratio > 0.7) return 'text-primary';
		if (ratio > 0.4) return 'text-foreground';
		return 'text-muted-foreground';
	}

	$: maxCount = Math.max(...tags.map(t => t.count), 1);
	$: sortedTags = [...tags].sort((a, b) => {
		if (sortBy === 'count') return b.count - a.count;
		return a.tag.localeCompare(b.tag, 'zh-CN');
	});

	onMount(() => {
		if (!$isAuthenticated) {
			goto('/login');
			return;
		}
		void loadTags();
	});
</script>

<svelte:head>
	<title>标签云 - 吾身</title>
</svelte:head>

<div class="flex flex-col min-h-screen min-h-[100dvh] bg-background">
	<PageHeader title="标签" />
	<div class="container-responsive py-6 flex-1 flex flex-col">
		{#if loading}
			<div class="flex flex-col items-center justify-center py-20 gap-3">
				<svg class="w-6 h-6 animate-spin text-primary" fill="none" viewBox="0 0 24 24">
					<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
					<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
				</svg>
				<div class="text-muted-foreground text-sm">加载中...</div>
			</div>
		{:else if tags.length === 0}
			<div class="flex-1 flex items-center justify-center">
				<div class="bg-card rounded-2xl border border-border/50 p-12 shadow-sm flex flex-col items-center text-center max-w-md animate-fade-in">
					<div class="w-20 h-20 rounded-2xl bg-gradient-to-br from-primary/10 to-primary/5 flex items-center justify-center mb-5">
						<svg class="w-10 h-10 text-primary/50" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M7 7h.01M7 3h5a1.99 1.99 0 011.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A1.99 1.99 0 013 12V7a4 4 0 014-4z" />
						</svg>
					</div>
					<h2 class="text-xl font-semibold text-foreground mb-2">还没有标签</h2>
					<p class="text-sm text-muted-foreground mb-6">
						打开日记，在右侧边栏的标签输入框里添加你的第一个标签吧。
					</p>
					<button
						type="button"
						onclick={() => goto('/diary')}
						class="px-5 py-2.5 rounded-xl bg-primary text-primary-foreground text-sm font-medium hover:opacity-90 active:scale-[0.98] transition-all shadow-sm"
					>
						开始写日记
					</button>
				</div>
			</div>
		{:else}
			<!-- Stats bar -->
			<div class="flex items-center justify-between mb-6 animate-fade-in">
				<div class="flex items-center gap-4">
					<div class="flex items-center gap-1.5">
						<span class="text-2xl font-bold text-foreground">{tags.length}</span>
						<span class="text-sm text-muted-foreground">个标签</span>
					</div>
					<div class="w-px h-5 bg-border/50"></div>
					<div class="flex items-center gap-1.5">
						<span class="text-2xl font-bold text-foreground">{totalDiaryCount}</span>
						<span class="text-sm text-muted-foreground">条日记</span>
					</div>
				</div>
				<div class="flex items-center gap-1 bg-muted/50 rounded-lg p-0.5">
					<button
						type="button"
						onclick={() => sortBy = 'count'}
						class="px-3 py-1.5 text-xs font-medium rounded-md transition-all duration-200 {sortBy === 'count' ? 'bg-card text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'}"
					>
						按热度
					</button>
					<button
						type="button"
						onclick={() => sortBy = 'name'}
						class="px-3 py-1.5 text-xs font-medium rounded-md transition-all duration-200 {sortBy === 'name' ? 'bg-card text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'}"
					>
						按名称
					</button>
				</div>
			</div>

			<!-- Tag cloud -->
			<div class="bg-card rounded-2xl border border-border/50 p-6 shadow-sm mb-6 animate-fade-in stagger-1">
				<div class="flex flex-wrap gap-2.5 items-center justify-center min-h-[6rem]">
					{#each sortedTags as t, i (t.tag)}
						<button
							type="button"
							onclick={() => loadDiariesForTag(t.tag)}
							class="inline-flex items-center gap-1.5 px-3.5 py-2 rounded-xl border transition-all duration-200 hover:shadow-md active:scale-95 {selectedTag === t.tag ? 'bg-primary text-primary-foreground border-primary shadow-md ring-2 ring-primary/20' : getTagBg(t.count, maxCount) + ' ' + getTagText(t.count, maxCount)}"
							style="font-size: {getTagScale(t.count, maxCount)}rem; animation-delay: {Math.min(i * 30, 400)}ms"
							title={`${t.tag} · ${t.count} 篇日记`}
						>
							<span class="font-medium">{t.tag}</span>
							<span class="text-[0.65em] font-semibold px-1.5 py-0.5 rounded-md {selectedTag === t.tag ? 'bg-white/20' : 'bg-primary/10 text-primary'}">
								{t.count}
							</span>
						</button>
					{/each}
				</div>
			</div>

			<!-- Diaries filtered by tag -->
			{#if selectedTag !== null}
				<div class="bg-card rounded-2xl border border-border/50 shadow-sm overflow-hidden animate-slide-up">
					<div class="flex items-center justify-between p-4 border-b border-border/50">
						<div class="flex items-center gap-3">
							<div class="w-8 h-8 rounded-lg bg-primary/10 flex items-center justify-center">
								<svg class="w-4 h-4 text-primary" fill="none" stroke="currentColor" viewBox="0 0 24 24">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 7h.01M7 3h5a1.99 1.99 0 011.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A1.99 1.99 0 013 12V7a4 4 0 014-4z" />
								</svg>
							</div>
							<div>
								<h2 class="text-base font-semibold text-foreground">
									「{selectedTag}」
								</h2>
								<span class="text-xs text-muted-foreground">{tagDiaries.length} 篇相关日记</span>
							</div>
						</div>
						<button
							type="button"
							onclick={clearTagFilter}
							class="px-3 py-1.5 rounded-lg border border-border/70 text-xs font-medium text-muted-foreground hover:text-foreground hover:bg-muted/50 transition-all"
						>
							清除筛选
						</button>
					</div>

					{#if loadingTag}
						<div class="flex flex-col items-center justify-center py-12 gap-3">
							<svg class="w-5 h-5 animate-spin text-primary" fill="none" viewBox="0 0 24 24">
								<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
								<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
							</svg>
							<div class="text-muted-foreground text-sm">加载中...</div>
						</div>
					{:else if tagDiaries.length === 0}
						<div class="py-12 text-center">
							<svg class="w-12 h-12 mx-auto text-muted-foreground/30 mb-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M9.172 16.172a4 4 0 015.656 0M9 10h.01M15 10h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
							</svg>
							<p class="text-muted-foreground text-sm">没有找到相关日记</p>
						</div>
					{:else}
						<div class="divide-y divide-border/30">
							{#each tagDiaries as diary, i (diary.id || diary.date)}
								<button
									type="button"
									onclick={() => goToDiary(diary.date)}
									class="w-full text-left p-4 hover:bg-muted/30 transition-all duration-200 group"
									style="animation-delay: {i * 40}ms"
								>
									<div class="flex items-start justify-between gap-3">
										<div class="min-w-0 flex-1">
											<div class="flex items-center gap-2 mb-1.5">
												<span class="text-sm font-semibold text-foreground group-hover:text-primary transition-colors">
													{formatDisplayDate(diary.date)}
												</span>
												{#if diary.mood}
													<span class="text-sm">{diary.mood}</span>
												{/if}
												{#if diary.weather}
													<span class="text-sm">{diary.weather}</span>
												{/if}
											</div>
											<p class="text-sm text-muted-foreground leading-relaxed line-clamp-2">
												{diary.content
													? diary.content.replace(/<[^>]+>/g, '').trim().slice(0, 180)
													: '（无内容）'}
											</p>
											{#if Array.isArray(diary.tags) && diary.tags.length > 0}
												<div class="flex flex-wrap gap-1.5 mt-2">
													{#each diary.tags as tagItem (tagItem)}
														<span
															class="inline-flex text-[10px] px-2 py-0.5 rounded-md {tagItem === selectedTag ? 'bg-primary/15 text-primary font-medium' : 'bg-muted text-muted-foreground'}"
														>
															#{tagItem}
														</span>
													{/each}
												</div>
											{/if}
										</div>
										<svg class="w-4 h-4 text-muted-foreground/40 group-hover:text-primary group-hover:translate-x-0.5 transition-all mt-1 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
											<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
										</svg>
									</div>
								</button>
							{/each}
						</div>
					{/if}
				</div>
			{/if}
		{/if}
	</div>
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

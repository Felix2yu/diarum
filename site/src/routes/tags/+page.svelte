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

	function fontSizeFor(count: number, max: number): string {
		// Scale from 0.85rem to 1.8rem based on relative frequency
		if (max <= 0) return '1rem';
		const ratio = count / max;
		const size = 0.85 + ratio * 0.95;
		return `${size.toFixed(2)}rem`;
	}

	$: maxCount = Math.max(...tags.map(t => t.count), 1);

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

<div class="flex flex-col min-h-screen bg-background">
	<PageHeader title="标签" />
	<div class="container-responsive py-6 flex-1 flex flex-col">
		<div class="mb-6">
			<h1 class="text-2xl font-semibold text-foreground">标签云</h1>
			<p class="text-sm text-muted-foreground mt-1">
				共 {tags.length} 个标签，涉及 {totalDiaryCount} 条日记
			</p>
		</div>

		{#if loading}
			<div class="flex flex-col items-center justify-center py-16 gap-3">
				<svg class="w-6 h-6 animate-spin text-primary" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path></svg>
				<div class="text-muted-foreground text-sm">加载中...</div>
			</div>
		{:else if tags.length === 0}
			<div class="bg-card rounded-xl border border-border/50 p-12 shadow-sm flex flex-col items-center text-center">
				<svg class="w-16 h-16 text-muted-foreground/40 mb-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M7 7h.01M7 3h5a1.99 1.99 0 011.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A1.99 1.99 0 013 12V7a4 4 0 014-4z" />
				</svg>
				<h2 class="text-lg font-semibold text-foreground">还没有标签</h2>
				<p class="text-sm text-muted-foreground mt-1">
					打开日记，在右侧边栏的标签输入框里添加你的第一个标签吧。
				</p>
				<button
					type="button"
					onclick={() => goto('/diary')}
					class="mt-6 px-4 py-2 rounded-lg bg-primary text-primary-foreground text-sm hover:opacity-90 transition-opacity"
				>
					开始写日记
				</button>
			</div>
		{:else}
			<div class="flex flex-col gap-6">
				<!-- Tag cloud -->
				<div class="bg-card rounded-xl border border-border/50 p-6 shadow-sm">
					<div class="flex flex-wrap gap-2 items-center justify-center min-h-[8rem]">
						{#each tags as t (t.tag)}
							<button
								type="button"
								onclick={() => loadDiariesForTag(t.tag)}
								class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full border transition-all duration-200"
								class:bg-primary={selectedTag === t.tag}
								class:text-primary-foreground={selectedTag === t.tag}
								class:border-primary={selectedTag === t.tag}
								class:bg-primary/5={selectedTag !== t.tag}
								class:text-primary={selectedTag !== t.tag}
								class:border-primary/20={selectedTag !== t.tag}
								class:hover:scale-105={selectedTag !== t.tag}
								style="font-size: {fontSizeFor(t.count, maxCount)}"
								title={`${t.tag} · ${t.count} 篇日记`}
							>
								<span class="font-medium">{t.tag}</span>
								<span
									class="text-[0.65em] font-semibold px-1.5 py-0.5 rounded-full"
									class:bg-white/20={selectedTag === t.tag}
									class:bg-primary/15={selectedTag !== t.tag}
									class:text-primary={selectedTag !== t.tag}
								>
									{t.count}
								</span>
							</button>
						{/each}
					</div>
				</div>

				<!-- Diaries filtered by tag -->
				{#if selectedTag !== null}
					<div class="bg-card rounded-xl border border-border/50 shadow-sm overflow-hidden">
						<div class="flex items-center justify-between p-4 border-b border-border/50">
							<div class="flex items-center gap-2">
								<h2 class="text-lg font-semibold text-foreground">
									「{selectedTag}」相关日记
								</h2>
								<span class="text-xs text-muted-foreground">共 {tagDiaries.length} 篇</span>
							</div>
							<button
								type="button"
								onclick={clearTagFilter}
								class="text-xs px-3 py-1.5 rounded-md border border-border/70 text-muted-foreground hover:text-foreground hover:bg-muted/50 transition-colors"
							>
								清除筛选
							</button>
						</div>

						{#if loadingTag}
							<div class="flex flex-col items-center justify-center py-10 gap-3">
								<svg class="w-5 h-5 animate-spin text-primary" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path></svg>
								<div class="text-muted-foreground text-sm">加载中...</div>
							</div>
						{:else if tagDiaries.length === 0}
							<div class="p-8 text-center text-muted-foreground text-sm">
								没有找到相关日记
							</div>
						{:else}
							<ul class="divide-y divide-border/50 max-h-[28rem] overflow-y-auto">
								{#each tagDiaries as diary (diary.id || diary.date)}
									<li
										class="p-4 hover:bg-muted/40 transition-colors cursor-pointer"
										onclick={() => goToDiary(diary.date)}
									>
										<div class="flex items-start justify-between gap-4">
											<div class="min-w-0 flex-1">
												<div class="text-sm font-semibold text-foreground">
													{formatDisplayDate(diary.date)}
												</div>
												<p class="text-sm text-muted-foreground mt-1 line-clamp-2">
													{diary.content
														? diary.content.replace(/<[^>]+>/g, '').trim().slice(0, 180)
														: '（无内容）'}
												</p>
												{#if Array.isArray(diary.tags) && diary.tags.length > 0}
													<div class="flex flex-wrap gap-1 mt-2">
														{#each diary.tags as tagItem (tagItem)}
															<span
																class="inline-flex text-[10px] px-1.5 py-0.5 rounded-full bg-muted text-muted-foreground"
															>
																#{tagItem}
															</span>
														{/each}
													</div>
												{/if}
											</div>
											<div class="flex items-center gap-1 text-xs shrink-0">
												{#if diary.mood}
													<span class="text-lg">{diary.mood}</span>
												{/if}
												{#if diary.weather}
													<span class="text-lg">{diary.weather}</span>
												{/if}
											</div>
										</div>
									</li>
								{/each}
							</ul>
						{/if}
					</div>
				{/if}
			</div>
		{/if}
	</div>
	<Footer />
</div>
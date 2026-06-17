<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import TiptapEditor from '$lib/components/editor/TiptapEditor.svelte';
	import TableOfContents from '$lib/components/ui/TableOfContents.svelte';
import PageHeader from '$lib/components/ui/PageHeader.svelte';
	import Footer from '$lib/components/ui/Footer.svelte';
	import DiaryShareModal from '$lib/components/share/DiaryShareModal.svelte';
	import { getDiaryByDate } from '$lib/api/diaries';
	import { isAuthenticated } from '$lib/api/client';
	import { getDiaryEmojiSettings } from '$lib/api/settings';
	import {
		formatDisplayDate,
		formatShortDate,
		getDayOfWeek,
		getPreviousDay,
		getNextDay,
		getToday,
		isToday
	} from '$lib/utils/date';
	import {
		diaryCache,
		syncState,
		updateLocalCache,
		updateFromServer,
		getCachedContent,
		forceSyncNow,
		hasDirtyCache,
		initDiaryCache,
		cleanupDiaryCache
	} from '$lib/stores/diaryCache';
	import { onlineState } from '$lib/stores/onlineStatus';
	import { DEFAULT_MOOD_OPTIONS, DEFAULT_WEATHER_OPTIONS } from '$lib/utils/diaryEmoji';

	let moodPresets: string[] = [...DEFAULT_MOOD_OPTIONS];
	let weatherPresets: string[] = [...DEFAULT_WEATHER_OPTIONS];

	let content = '';
	let loading = true;
	let loadRequestId = 0;
	let showDrawer = false;
	let showDesktopToc = true;
	let showShareModal = false;
	let selectedContent = '';
	let selectedMood = '';
	let selectedWeather = '';
	// Snapshot taken on mousedown (before blur clears selectedContent)
	let shareSelectedContent = '';
	let shareOpenedByMouse = false;
	let date = getToday();
	let cacheReady = false;

	function captureShareSelection() {
		shareSelectedContent = selectedContent;
		shareOpenedByMouse = true;
	}

	function openShareModal() {
		// Keyboard path (Enter/Space): mousedown never fired, so clear any stale snapshot
		if (!shareOpenedByMouse) {
			shareSelectedContent = '';
		}
		shareOpenedByMouse = false;
		showShareModal = true;
	}

	$: date = $page.params.date ?? getToday();
	$: canGoNext = !isToday(date);
	$: currentDateIsDirty = date ? $diaryCache[date]?.isDirty || false : false;
	$: isAnySyncing = $syncState.isSyncing;

	function goToPreviousDay() {
		const prevDate = getPreviousDay(date);
		goto(`/diary/${prevDate}`);
	}

	function goToNextDay() {
		const currentDate = date;
		if (isToday(currentDate)) return;
		const nextDate = getNextDay(currentDate);
		goto(`/diary/${nextDate}`);
	}

	function goToCalendar() {
		const url = new URL('/diary', window.location.origin);
		if (date) url.searchParams.set('date', date);
		goto(url.pathname + url.search);
	}

	async function loadDiary(targetDate: string) {
		const currentRequestId = ++loadRequestId;
		const cached = getCachedContent(targetDate);

		// Keep unsynced local draft and skip server fetch.
		if (cached?.isDirty) {
			content = cached.content;
			selectedMood = cached.mood || '';
			selectedWeather = cached.weather || '';
			loading = false;
			return;
		}

		content = '';
		selectedMood = '';
		selectedWeather = '';

		// Browser cache is disabled; fetch current content from server.
		loading = true;
		try {
			const diary = await getDiaryByDate(targetDate);
			if (currentRequestId !== loadRequestId) return;
			updateFromServer(targetDate, diary);
			if (currentRequestId !== loadRequestId) return;
			content = diary?.content || '';
			selectedMood = diary?.mood || '';
			selectedWeather = diary?.weather || '';
		} catch (error) {
			console.error('Failed to load diary:', error);
			// Keep local draft on fetch failure if one exists.
			if (cached?.isDirty) {
				content = cached.content;
				selectedMood = cached.mood || '';
				selectedWeather = cached.weather || '';
			}
		}
		loading = false;
	}

	async function loadDiaryEmojiPresets() {
		try {
			const settings = await getDiaryEmojiSettings();
			moodPresets = [...settings.mood_options];
			weatherPresets = [...settings.weather_options];
		} catch (error) {
			console.error('Failed to load mood/weather presets:', error);
		}
	}

	function handleContentChange(newContent: string) {
		content = newContent;
		updateLocalCache(date, {
			content,
			mood: selectedMood,
			weather: selectedWeather
		});
	}

	function handleMoodSelect(emoji: string) {
		selectedMood = selectedMood === emoji ? '' : emoji;
		updateLocalCache(date, {
			content,
			mood: selectedMood,
			weather: selectedWeather
		});
	}

	function handleWeatherSelect(emoji: string) {
		selectedWeather = selectedWeather === emoji ? '' : emoji;
		updateLocalCache(date, {
			content,
			mood: selectedMood,
			weather: selectedWeather
		});
	}

	async function handleManualSave() {
		await forceSyncNow();
	}

	function handleKeyboard(event: KeyboardEvent) {
		if ((event.ctrlKey || event.metaKey) && event.key === 's') {
			event.preventDefault();
			handleManualSave();
		}
	}

	let previousDate = '';

	onMount(() => {
		if (!$isAuthenticated) {
			goto('/login');
			return;
		}

		// Initialize diary cache (includes online status)
		initDiaryCache();
		cacheReady = true;
		void loadDiaryEmojiPresets();

		window.addEventListener('keydown', handleKeyboard);
		return () => {
			window.removeEventListener('keydown', handleKeyboard);
			// Note: Don't cleanup diaryCache here as it's shared across pages
		};
	});

	// Load diary only in browser (not during SSR)
	$: if (cacheReady && date && date !== previousDate && typeof window !== 'undefined') {
		previousDate = date;
		loadDiary(date);
	}
</script>

<svelte:head>
	<title>{formatDisplayDate(date)} - 吾身</title>
</svelte:head>

<div class="flex flex-col min-h-screen bg-background">
	<PageHeader title="日记" />
<!-- Main Content -->
	<div class="container-responsive py-6 flex-1">
		<!-- 日期导航 -->
		<div class="diary-layout flex gap-6 mx-auto transition-all duration-300 mb-4" class:with-desktop-sidebar={showDesktopToc}>
			<main class="diary-main w-full min-w-0">
				<div class="flex items-center justify-between bg-card rounded-xl border border-border/50 px-3 py-2.5 shadow-sm">
					<button
						type="button"
						on:click={goToPreviousDay}
						class="flex items-center gap-1 px-3 py-1.5 text-sm text-foreground/80 hover:text-foreground hover:bg-muted/50 rounded-lg transition-all duration-200"
						title="上一天"
					>
						<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
						</svg>
						<span>前一天</span>
					</button>

					<div class="flex items-center gap-2 text-center">
						<button
							type="button"
							on:click={goToCalendar}
							class="text-sm font-semibold text-foreground hover:opacity-80 transition-opacity"
							title="返回日历"
						>
							{formatDisplayDate(date)}
						</button>
						{#if isToday(date)}
							<span class="text-[10px] px-1.5 py-0.5 bg-primary/15 text-primary rounded-full font-medium">今天</span>
						{/if}
						<span class="text-[11px] text-muted-foreground hidden sm:inline">星期{getDayOfWeek(date)}</span>
					</div>

					<button
						type="button"
						on:click={goToNextDay}
						class="flex items-center gap-1 px-3 py-1.5 text-sm text-foreground/80 hover:text-foreground hover:bg-muted/50 rounded-lg transition-all duration-200 disabled:opacity-30 disabled:cursor-not-allowed"
						disabled={!canGoNext}
						title="下一天"
					>
						<span>后一天</span>
						<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
						</svg>
					</button>
				</div>
			</main>
			<!-- 占位保持右侧边栏对齐 -->
			{#if showDesktopToc}
				<div class="hidden lg:block w-[19rem] flex-shrink-0" aria-hidden="true"></div>
			{/if}
		</div>

		<!-- 原布局 -->
		<div class="diary-layout flex gap-6 mx-auto transition-all duration-300" class:with-desktop-sidebar={showDesktopToc}>
			<!-- Editor -->
			<main class="diary-main w-full min-w-0">
				{#if loading}
					<div class="flex flex-col items-center justify-center py-20 gap-3 animate-fade-in">
						<svg class="w-6 h-6 animate-spin text-primary" fill="none" viewBox="0 0 24 24">
							<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
							<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
						</svg>
						<div class="text-muted-foreground text-sm">加载中...</div>
					</div>
				{:else}
					<div class="bg-card rounded-xl shadow-sm border border-border/50 overflow-hidden animate-fade-in">
						<TiptapEditor
							{content}
							bind:selectedContent
							onChange={handleContentChange}
							placeholder="今天有什么想说的？"
							emptyStatePrompt="✨ 回顾今天... 这一天你会记住什么？"
							diaryDate={date}
						/>
					</div>
				{/if}
			</main>

			<!-- Desktop Right Sidebar -->
			{#if showDesktopToc}
				<aside class="hidden lg:block w-[19rem] flex-shrink-0">
					<div class="sticky top-11 space-y-3 animate-slide-in-right">
						<div class="bg-card/50 rounded-xl border border-border/50 p-4 shadow-sm">
							<div class="flex items-center justify-between mb-2">
								<div>
									<div class="text-sm font-semibold text-foreground">心情</div>
								</div>
								{#if selectedMood}
									<button
										on:click={() => handleMoodSelect(selectedMood)}
										class="text-[11px] px-2 py-1 rounded-full bg-background/70 hover:bg-background border border-border/70 transition-colors"
									>
										清除
									</button>
								{/if}
							</div>
							<div class="grid grid-cols-4 gap-2">
								{#each moodPresets as option}
									<button
										on:click={() => handleMoodSelect(option)}
										class="emoji-option {selectedMood === option ? 'emoji-option-active' : ''}"
										title={option}
										aria-label={`心情 ${option}`}
									>
										<span class="text-xl leading-none">{option}</span>
									</button>
								{/each}
							</div>
						</div>

						<div class="bg-card/50 rounded-xl border border-border/50 p-4 shadow-sm">
							<div class="flex items-center justify-between mb-2">
								<div>
									<div class="text-sm font-semibold text-foreground">天气</div>
								</div>
								{#if selectedWeather}
									<button
										on:click={() => handleWeatherSelect(selectedWeather)}
										class="text-[11px] px-2 py-1 rounded-full bg-background/70 hover:bg-background border border-border/70 transition-colors"
									>
										清除
									</button>
								{/if}
							</div>
							<div class="grid grid-cols-4 gap-2">
								{#each weatherPresets as option}
									<button
										on:click={() => handleWeatherSelect(option)}
										class="emoji-option {selectedWeather === option ? 'emoji-option-active' : ''}"
										title={option}
										aria-label={`天气 ${option}`}
									>
										<span class="text-xl leading-none">{option}</span>
									</button>
								{/each}
							</div>
						</div>

						<div class="bg-card/50 rounded-xl border border-border/50 p-4">
							<TableOfContents {content} />
						</div>
					</div>
				</aside>
			{/if}
		</div>
	</div>

	<!-- Footer -->
	<Footer />
</div>

<!-- Left Drawer -->
{#if showDrawer}
	<!-- Backdrop -->
	<button
		class="fixed inset-0 bg-black/40 backdrop-blur-sm z-40 lg:hidden"
		on:click={() => showDrawer = false}
		aria-label="关闭菜单"
	></button>

	<!-- Drawer Panel -->
	<div class="fixed inset-y-0 left-0 w-72 bg-card border-r border-border shadow-2xl z-50 lg:hidden animate-slide-in-left">
		<!-- Drawer Header -->
		<div class="flex items-center justify-between px-5 py-4 border-b border-border/50">
			<div class="flex items-center gap-2">
				<img src="/logo.png" alt="吾身" class="w-6 h-6" />
				<span class="font-semibold text-foreground">菜单</span>
			</div>
			<button
				on:click={() => showDrawer = false}
				class="p-2 hover:bg-muted rounded-lg transition-colors"
				aria-label="关闭"
			>
				<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
				</svg>
			</button>
		</div>

		<!-- Drawer Content -->
		<div class="flex flex-col h-[calc(100%-57px)]">
			<!-- Actions Section -->
			<div class="px-3 py-3">
				<div class="text-[10px] font-semibold text-muted-foreground uppercase tracking-wider mb-2 px-1">
					快捷操作
				</div>
				<div class="space-y-0.5">
					<a
						href="/assistant"
						class="flex items-center gap-2.5 px-2 py-1.5 rounded-lg hover:bg-muted/70 transition-all duration-200 group"
						on:click={() => showDrawer = false}
					>
						<div class="p-1.5 rounded-md bg-primary/10 text-primary group-hover:bg-primary/20 transition-colors">
							<svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<rect x="4" y="6" width="16" height="12" rx="2" stroke-width="2"/>
								<circle cx="9" cy="11" r="1.5" fill="currentColor"/>
								<circle cx="15" cy="11" r="1.5" fill="currentColor"/>
							</svg>
						</div>
						<div class="min-w-0">
							<div class="text-xs font-medium text-foreground">AI 助手</div>
							<div class="text-[10px] text-muted-foreground truncate">与 AI 聊聊你的日记</div>
						</div>
					</a>

					<button
						on:mousedown={captureShareSelection}
						on:click={() => { showDrawer = false; openShareModal(); }}
						class="w-full flex items-center gap-2.5 px-2 py-1.5 rounded-lg hover:bg-muted/70 transition-all duration-200 group"
					>
						<div class="p-1.5 rounded-md bg-blue-500/10 text-blue-500 group-hover:bg-blue-500/20 transition-colors">
							<svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8.684 13.342C8.886 12.938 9 12.482 9 12c0-.482-.114-.938-.316-1.342m0 2.684a3 3 0 110-2.684m0 2.684l6.632 3.316m-6.632-6l6.632-3.316m0 0a3 3 0 105.367-2.684 3 3 0 00-5.367 2.684zm0 9.316a3 3 0 105.368 2.684 3 3 0 00-5.368-2.684z" />
							</svg>
						</div>
						<div class="min-w-0 text-left">
							<div class="text-xs font-medium text-foreground">分享</div>
							<div class="text-[10px] text-muted-foreground truncate">导出为精美图片</div>
						</div>
					</button>

					<a
						href="/diary"
						class="flex items-center gap-2.5 px-2 py-1.5 rounded-lg hover:bg-muted/70 transition-all duration-200 group"
						on:click={() => showDrawer = false}
					>
						<div class="p-1.5 rounded-md bg-green-500/10 text-green-500 group-hover:bg-green-500/20 transition-colors">
							<svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
									d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
							</svg>
						</div>
						<div class="min-w-0">
							<div class="text-xs font-medium text-foreground">日历</div>
							<div class="text-[10px] text-muted-foreground truncate">查看所有日记条目</div>
						</div>
					</a>

					<a
						href="/settings"
						class="flex items-center gap-2.5 px-2 py-1.5 rounded-lg hover:bg-muted/70 transition-all duration-200 group"
						on:click={() => showDrawer = false}
					>
						<div class="p-1.5 rounded-md bg-gray-500/10 text-gray-500 group-hover:bg-gray-500/20 transition-colors">
							<svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
							</svg>
						</div>
						<div class="min-w-0">
							<div class="text-xs font-medium text-foreground">设置</div>
							<div class="text-[10px] text-muted-foreground truncate">偏好与同步</div>
						</div>
					</a>
				</div>
			</div>

			<!-- Divider -->
			<div class="mx-3 border-t border-border/50"></div>

			<!-- Mood & Weather -->
			<div class="px-3 py-3 space-y-3 border-b border-border/50">
				<div>
					<div class="text-[10px] font-semibold text-muted-foreground uppercase tracking-wider mb-2 px-1">心情</div>
					<div class="grid grid-cols-4 gap-1.5">
						{#each moodPresets as option}
							<button
								on:click={() => handleMoodSelect(option)}
								class="emoji-option {selectedMood === option ? 'emoji-option-active' : ''}"
								title={option}
								aria-label={`心情 ${option}`}
							>
								<span class="text-lg">{option}</span>
							</button>
						{/each}
					</div>
				</div>

				<div>
					<div class="text-[10px] font-semibold text-muted-foreground uppercase tracking-wider mb-2 px-1">天气</div>
					<div class="grid grid-cols-4 gap-1.5">
						{#each weatherPresets as option}
							<button
								on:click={() => handleWeatherSelect(option)}
								class="emoji-option {selectedWeather === option ? 'emoji-option-active' : ''}"
								title={option}
								aria-label={`天气 ${option}`}
							>
								<span class="text-lg">{option}</span>
							</button>
						{/each}
					</div>
				</div>
			</div>

			<!-- TOC Section -->
			<div class="flex-1 overflow-y-auto px-3 py-3">
				<TableOfContents {content} onNavigate={() => showDrawer = false} />
			</div>
		</div>
	</div>
{/if}

<!-- Share Modal -->
<DiaryShareModal
	isOpen={showShareModal}
	{date}
	{content}
	selectedContent={shareSelectedContent}
	onClose={() => showShareModal = false}
/>

<style>
	.emoji-option {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		padding: 0.5rem;
		border-radius: 0.8rem;
		border: 1px solid hsl(var(--border) / 0.6);
		background: hsl(var(--background) / 0.72);
		transition: transform 0.18s ease, border-color 0.18s ease, background-color 0.18s ease, box-shadow 0.18s ease;
	}

	.emoji-option:hover {
		transform: translateY(-1px);
		background: hsl(var(--muted) / 0.65);
		border-color: hsl(var(--primary) / 0.3);
	}

	.emoji-option-active {
		border-color: hsl(var(--primary) / 0.65);
		background: hsl(var(--primary) / 0.12);
		box-shadow: 0 8px 16px hsl(var(--primary) / 0.12);
	}

	.diary-layout {
		max-width: 48rem;
	}

	@media (min-width: 1024px) {
		.diary-main {
			flex: 1 1 auto;
			max-width: 48rem;
		}
	}

	@media (min-width: 1024px) {
		.diary-layout.with-desktop-sidebar {
			max-width: calc(48rem + 19rem + 1.5rem);
		}
	}
</style>

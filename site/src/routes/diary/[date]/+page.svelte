<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import TiptapEditor from '$lib/components/editor/TiptapEditor.svelte';
	import TableOfContents from '$lib/components/ui/TableOfContents.svelte';
	import TextPolisher from '$lib/components/TextPolisher.svelte';
	import { getAISettings, transcribeAudio, isSpeechConfigured, type AISettings } from '$lib/api/ai';
	import PageHeader from '$lib/components/ui/PageHeader.svelte';
	import Footer from '$lib/components/ui/Footer.svelte';
	import DiaryShareModal from '$lib/components/share/DiaryShareModal.svelte';
	import { getDiaryByDate, getTagCloud } from '$lib/api/diaries';
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
	let showPolisher = false;
	let polishSourceText = '';
	let selectedContent = '';
	let selectedMood = '';
	let selectedWeather = '';
	let tags: string[] = [];
	let tagInput = '';
	let allTags: string[] = [];
	let tagSuggestions: string[] = [];
	let showTagSuggestions = false;
	let selectedSuggestionIndex = -1;

	// Speech recognition
	let speechSettings: AISettings | null = null;
	let speechEnabled = false;
	let speechError = '';
	let isRecording = false;
	let isTranscribing = false;
	let recordingSeconds = 0;
	let mediaRecorder: MediaRecorder | null = null;
	let recordedChunks: Blob[] = [];
	let recordingTimer: ReturnType<typeof setInterval> | null = null;
	let micSupported = typeof window !== 'undefined' && !!(window as any).navigator?.mediaDevices?.getUserMedia;
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

	function addTagFromInput() {
		const raw = tagInput.trim();
		if (!raw) return;
		// Split by comma so users can enter multiple at once
		const newTags = raw.split(/[,，]/).map(s => s.trim()).filter(Boolean);
		const seen = new Set(tags);
		const merged = [...tags];
		for (const t of newTags) {
			if (!seen.has(t)) {
				seen.add(t);
				merged.push(t);
			}
		}
		tags = merged;
		tagInput = '';
		updateLocalCache(date, { content, mood: selectedMood, weather: selectedWeather, tags });
	}

	function removeTag(tag: string) {
		tags = tags.filter(t => t !== tag);
		updateLocalCache(date, { content, mood: selectedMood, weather: selectedWeather, tags });
	}

	function handleTagKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter' || e.key === ',') {
			e.preventDefault();
			if (showTagSuggestions && selectedSuggestionIndex >= 0) {
				applySuggestion(tagSuggestions[selectedSuggestionIndex]);
			} else {
				addTagFromInput();
			}
		} else if (e.key === 'ArrowDown') {
			e.preventDefault();
			if (showTagSuggestions) {
				selectedSuggestionIndex = Math.min(selectedSuggestionIndex + 1, tagSuggestions.length - 1);
			}
		} else if (e.key === 'ArrowUp') {
			e.preventDefault();
			if (showTagSuggestions) {
				selectedSuggestionIndex = Math.max(selectedSuggestionIndex - 1, -1);
			}
		} else if (e.key === 'Escape') {
			showTagSuggestions = false;
			selectedSuggestionIndex = -1;
		}
	}

	function handleTagInput(e: Event) {
		const value = (e.target as HTMLInputElement).value;
		tagInput = value;
		updateTagSuggestions(value);
	}

	function updateTagSuggestions(value: string) {
		const q = value.trim().toLowerCase();
		if (!q || q.length < 1) {
			tagSuggestions = [];
			showTagSuggestions = false;
			selectedSuggestionIndex = -1;
			return;
		}
		const existingSet = new Set(tags);
		tagSuggestions = allTags
			.filter(t => t.toLowerCase().includes(q) && !existingSet.has(t))
			.slice(0, 6);
		showTagSuggestions = tagSuggestions.length > 0;
		selectedSuggestionIndex = -1;
	}

	function applySuggestion(tag: string) {
		if (!tags.includes(tag)) {
			tags = [...tags, tag];
			updateLocalCache(date, { content, mood: selectedMood, weather: selectedWeather, tags });
		}
		tagInput = '';
		showTagSuggestions = false;
		selectedSuggestionIndex = -1;
	}

	function hideTagSuggestions() {
		setTimeout(() => {
			showTagSuggestions = false;
			selectedSuggestionIndex = -1;
		}, 150);
	}

	async function loadAllTags() {
		try {
			const result = await getTagCloud();
			allTags = result.map(t => t.tag);
		} catch {
			allTags = [];
		}
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
			tags = cached.tags || [];
			loading = false;
			return;
		}

		content = '';
		selectedMood = '';
		selectedWeather = '';
		tags = [];

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
			tags = diary?.tags || [];
		} catch (error) {
			console.error('Failed to load diary:', error);
			// Keep local draft on fetch failure if one exists.
			if (cached?.isDirty) {
				content = cached.content;
				selectedMood = cached.mood || '';
				selectedWeather = cached.weather || '';
				tags = cached.tags || [];
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
			weather: selectedWeather,
			tags
		});
	}

	function handleMoodSelect(emoji: string) {
		selectedMood = selectedMood === emoji ? '' : emoji;
		updateLocalCache(date, {
			content,
			mood: selectedMood,
			weather: selectedWeather,
			tags
		});
	}

	function handleWeatherSelect(emoji: string) {
		selectedWeather = selectedWeather === emoji ? '' : emoji;
		updateLocalCache(date, {
			content,
			mood: selectedMood,
			weather: selectedWeather,
			tags
		});
	}

	async function loadSpeechSettings() {
		try {
			const s = await getAISettings();
			speechSettings = s;
			speechEnabled = isSpeechConfigured(s);
		} catch (e) {
			speechSettings = null;
			speechEnabled = false;
		}
	}

	function insertTextAtCursor(inserted: string) {
		if (!inserted) return;
		const ta = document.querySelector<HTMLTextAreaElement>('.markdown-editor textarea');
		let base = content || '';
		let insertAt = base.length;
		if (ta) {
			const start = ta.selectionStart ?? base.length;
			const end = ta.selectionEnd ?? base.length;
			// Ensure we insert at proper position — if editor is focused use cursor
			const isInFocus = document.activeElement === ta;
			if (isInFocus || start !== end || start !== base.length) {
				insertAt = start;
			}
			let before = base.substring(0, Math.min(insertAt, base.length));
			let after = base.substring(insertAt);
			// Insert separator (space) if preceding character is non-whitespace and non-empty
			if (before && !/\s$/.test(before)) {
				before += ' ';
			}
			const newText = before + inserted + after;
			handleContentChange(newText);
			// Restore cursor
			setTimeout(() => {
				const finalTa = document.querySelector<HTMLTextAreaElement>('.markdown-editor textarea');
				if (finalTa) {
					const pos = before.length + inserted.length;
					finalTa.focus();
					try {
						finalTa.setSelectionRange(pos, pos);
					} catch {
						// ignore
					}
				}
			}, 0);
			return;
		}
		// Fallback: append
		let prefix = base && !/\s$/.test(base) ? ' ' : '';
		handleContentChange(base + prefix + inserted);
	}

	function stopRecordingTimer() {
		if (recordingTimer) {
			clearInterval(recordingTimer);
			recordingTimer = null;
		}
	}

	async function startRecording() {
		speechError = '';
		if (!micSupported) {
			speechError = '当前浏览器不支持录音（需要 HTTPS 或支持 navigator.mediaDevices.getUserMedia）';
			return;
		}
		try {
			const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
			recordedChunks = [];
			const mr = new MediaRecorder(stream, { mimeType: 'audio/webm' });
			mediaRecorder = mr;
			mr.ondataavailable = (e) => {
				if (e.data && e.data.size > 0) {
					recordedChunks.push(e.data);
				}
			};
			mr.onstop = async () => {
				stream.getTracks().forEach((t) => t.stop());
				isRecording = false;
				stopRecordingTimer();
				await transcribeRecording();
			};
			recordingSeconds = 0;
			mr.start();
			isRecording = true;
			recordingTimer = setInterval(() => {
				recordingSeconds += 1;
				// Safety cap: 5 minutes
				if (recordingSeconds >= 5 * 60 && mediaRecorder && mediaRecorder.state !== 'inactive') {
					mediaRecorder.stop();
				}
			}, 1000);
		} catch (e) {
			speechError = e instanceof Error ? e.message : '无法打开麦克风';
			isRecording = false;
			stopRecordingTimer();
		}
	}

	async function stopRecording() {
		if (mediaRecorder && mediaRecorder.state !== 'inactive') {
			try {
				mediaRecorder.stop();
			} catch {
				isRecording = false;
				stopRecordingTimer();
			}
		} else {
			isRecording = false;
			stopRecordingTimer();
		}
	}

	async function transcribeRecording() {
		if (recordedChunks.length === 0) {
			return;
		}
		const audioBlob = new Blob(recordedChunks, { type: 'audio/webm' });
		if (audioBlob.size < 512) {
			// Too short — likely just silence / click
			speechError = '录音时间过短，请重新尝试';
			setTimeout(() => { speechError = ''; }, 2500);
			return;
		}
		isTranscribing = true;
		speechError = '';
		try {
			const result = await transcribeAudio(audioBlob, {
				language: speechSettings?.speech_language || undefined
			});
			let text = (result.text || '').trim();
			if (text) {
				// Basic sentence-style capitalization for English if first char is ASCII letter
				if (/^[a-z]/.test(text)) {
					text = text.charAt(0).toUpperCase() + text.slice(1);
				}
				// Add trailing punctuation if missing
				if (!/[。.!?！？\s]$/.test(text)) {
					text += '。';
				}
				insertTextAtCursor(text);
			} else {
				speechError = '未识别到内容，请重试';
				setTimeout(() => { speechError = ''; }, 2500);
			}
		} catch (e) {
			speechError = e instanceof Error ? e.message : '语音识别失败';
			setTimeout(() => { speechError = ''; }, 3000);
		} finally {
			isTranscribing = false;
		}
	}

	function formatRecordingTime(seconds: number): string {
		const m = Math.floor(seconds / 60).toString().padStart(2, '0');
		const s = Math.floor(seconds % 60).toString().padStart(2, '0');
		return `${m}:${s}`;
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

	function handleOpenPolisher() {
		polishSourceText =
			selectedContent && selectedContent.trim() !== '' ? selectedContent : content;
		showPolisher = true;
	}

	function handleApplyPolished(text: string) {
		const toReplace = selectedContent && selectedContent.trim() !== '' ? selectedContent : content;
		if (selectedContent && selectedContent.trim() !== '' && selectedContent !== content) {
			// 只替换选中的部分
			const newContent = content.replace(selectedContent, text);
			handleContentChange(newContent);
		} else {
			// 替换整篇
			handleContentChange(text);
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
		void loadSpeechSettings();
		void loadAllTags();

		window.addEventListener('keydown', handleKeyboard);
		return () => {
			window.removeEventListener('keydown', handleKeyboard);
			if (mediaRecorder && mediaRecorder.state !== 'inactive') {
				try { mediaRecorder.stop(); } catch { /* ignore */ }
			}
			stopRecordingTimer();
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

<div class="flex flex-col min-h-screen min-h-[100dvh] bg-background">
	<PageHeader title="日记" />
<!-- Main Content -->
	<div class="container-responsive py-6 flex-1 flex flex-col">
		<!-- 主内容布局：导航条与编辑器同宽 -->
		<div class="diary-layout flex gap-6 mx-auto transition-all duration-300 flex-1 min-h-0" class:with-desktop-sidebar={showDesktopToc}>
			<main class="diary-main w-full min-w-0 flex flex-col min-h-0">
			<!-- 日期导航：紧贴编辑器上方，宽度随编辑器一致 -->
			<div class="mb-4">
				<div class="flex items-center bg-card rounded-xl border border-border/50 px-3 py-2.5 shadow-sm hover:shadow-md transition-shadow">
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

						<div class="flex-1 flex items-center justify-center gap-2 min-w-0">
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
				</div>
				{#if loading}
					<div class="flex flex-col items-center justify-center py-20 gap-3 animate-fade-in">
						<svg class="w-6 h-6 animate-spin text-primary" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path></svg>
						<div class="text-muted-foreground text-sm">加载中...</div>
					</div>
				{:else}
					<div class="bg-card rounded-xl shadow-sm border border-border/50 overflow-hidden animate-fade-in flex flex-col flex-1 min-h-0 relative ring-1 ring-border/20">
						<TiptapEditor
							{content}
							bind:selectedContent
							onChange={handleContentChange}
							placeholder="今天有什么想说的？"
							emptyStatePrompt="✨ 回顾今天... 这一天你会记住什么？"
							diaryDate={date}
						/>
					{#if speechEnabled || isRecording || isTranscribing}
						<div class="absolute bottom-3 right-3 flex items-center gap-2 z-10">
							{#if speechError}
								<div class="px-3 py-1.5 rounded-lg bg-destructive/90 text-destructive-foreground text-xs shadow-md">
									{speechError}
								</div>
							{/if}
							{#if isRecording}
								<button
									type="button"
									on:click={stopRecording}
									title="停止录音并转文字"
									class="inline-flex items-center gap-2 px-3 py-2 bg-destructive/90 hover:bg-destructive text-destructive-foreground rounded-full text-sm font-medium shadow-lg shadow-destructive/20 transition-all"
								>
									<span class="relative flex items-center justify-center w-3 h-3">
										<span class="absolute inline-flex h-full w-full rounded-full bg-white/70 animate-ping opacity-75"></span>
										<span class="relative inline-flex rounded-full w-2 h-2 bg-white"></span>
									</span>
									<span>停止录音</span>
									<span class="text-xs text-white/80 tabular-nums">{formatRecordingTime(recordingSeconds)}</span>
								</button>
							{:else if isTranscribing}
								<button
									type="button"
									disabled
									class="inline-flex items-center gap-2 px-3 py-2 bg-primary/90 text-primary-foreground rounded-full text-sm font-medium shadow-lg shadow-primary/20"
								>
									<svg class="w-4 h-4 animate-spin" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
										<circle class="opacity-25" cx="12" cy="12" r="10" stroke-width="4" stroke="currentColor"/>
										<path class="opacity-75" fill="currentColor" stroke="currentColor" stroke-width="4" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"/>
									</svg>
									<span>正在转写</span>
								</button>
							{:else}
								<button
									type="button"
									on:click={startRecording}
									title="语音输入日记"
									class="inline-flex items-center gap-2 px-3 py-2 bg-primary/90 hover:bg-primary text-primary-foreground rounded-full text-sm font-medium shadow-lg shadow-primary/20 transition-all"
								>
									<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
										<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 1a3 3 0 00-3 3v5a3 3 0 006 0V4a3 3 0 00-3-3z"/>
										<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 10v1a7 7 0 01-14 0v-1M12 18v3M8 21h8"/>
									</svg>
									<span>语音输入</span>
								</button>
							{/if}
						</div>
					{/if}
					</div>

				<!-- Mobile: Mood, Weather, Tags panel below editor -->
				<div class="lg:hidden mt-4 space-y-3 animate-slide-up">
						<!-- AI 整理入口 -->
						<button
							type="button"
							on:click={handleOpenPolisher}
							class="w-full flex items-center gap-2 bg-card hover:bg-card/80 rounded-xl border border-border/50 p-3 shadow-sm text-left group transition-all"
						>
							<div class="p-1.5 rounded-md bg-primary/10 text-primary group-hover:bg-primary/20 transition-colors">
								<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0018 18.469V19a2 2 0 01-2 2H8a2 2 0 01-2-2v-.531c0-.895.357-1.753.988-2.357l-.547-.546z" />
								</svg>
							</div>
							<div>
								<div class="text-xs font-semibold text-foreground">AI 整理文本</div>
								<div class="text-[11px] text-muted-foreground">去语气词 · 纠错 · 自动分段</div>
							</div>
						</button>

						<!-- Mood -->
						<div class="bg-card rounded-xl shadow-sm border border-border/50 p-4">
							<div class="flex items-center justify-between mb-2">
								<div class="text-sm font-semibold text-foreground">心情</div>
								{#if selectedMood}
									<button
										on:click={() => handleMoodSelect(selectedMood)}
										class="text-[11px] px-2 py-1 rounded-full bg-muted/70 hover:bg-muted border border-border/70 transition-colors text-muted-foreground"
									>
										清除
									</button>
								{/if}
							</div>
							<div class="grid grid-cols-4 gap-2">
								{#each moodPresets as option}
									<button
										on:click={() => handleMoodSelect(option)}
										class="emoji-option-mobile {selectedMood === option ? 'emoji-option-active' : ''}"
										title={option}
										aria-label={`心情 ${option}`}
									>
										<span class="text-xl leading-none">{option}</span>
									</button>
								{/each}
							</div>
						</div>

						<!-- Weather -->
						<div class="bg-card rounded-xl shadow-sm border border-border/50 p-4">
							<div class="flex items-center justify-between mb-2">
								<div class="text-sm font-semibold text-foreground">天气</div>
								{#if selectedWeather}
									<button
										on:click={() => handleWeatherSelect(selectedWeather)}
										class="text-[11px] px-2 py-1 rounded-full bg-muted/70 hover:bg-muted border border-border/70 transition-colors text-muted-foreground"
									>
										清除
									</button>
								{/if}
							</div>
							<div class="grid grid-cols-4 gap-2">
								{#each weatherPresets as option}
									<button
										on:click={() => handleWeatherSelect(option)}
										class="emoji-option-mobile {selectedWeather === option ? 'emoji-option-active' : ''}"
										title={option}
										aria-label={`天气 ${option}`}
									>
										<span class="text-xl leading-none">{option}</span>
									</button>
								{/each}
							</div>
						</div>

						<!-- Tags -->
						<div class="bg-card rounded-xl shadow-sm border border-border/50 p-4">
							<div class="flex items-center justify-between mb-3">
								<div class="text-sm font-semibold text-foreground">标签</div>
								{#if tags.length > 0}
									<span class="text-[10px] text-muted-foreground">
										{tags.length} 个
									</span>
								{/if}
							</div>
							<div class="flex flex-wrap gap-1.5 mb-3">
								{#each tags as tag (tag)}
									<span
										class="inline-flex items-center gap-1 group bg-primary/10 text-primary border border-primary/20 rounded-full px-2 py-0.5 text-xs hover:bg-primary/15 transition-colors"
									>
										{tag}
										<button
											type="button"
											on:click={() => removeTag(tag)}
											class="opacity-60 hover:opacity-100 hover:text-destructive transition-opacity flex-shrink-0"
											aria-label={`移除标签 ${tag}`}
										>
											<svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
												<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M6 18L18 6M6 6l12 12" />
											</svg>
										</button>
									</span>
								{:else}
									<div class="text-xs text-muted-foreground/60">尚未添加标签</div>
								{/each}
							</div>
							<div class="relative">
								<input
									type="text"
									value={tagInput}
									on:input={handleTagInput}
									on:keydown={handleTagKeydown}
									on:focus={() => updateTagSuggestions(tagInput)}
									on:blur={hideTagSuggestions}
									placeholder="添加标签，按回车或逗号确认"
									class="w-full text-xs px-3 py-2 rounded-lg bg-muted/30 border border-border/60 focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary/30 transition-colors placeholder:text-muted-foreground/50"
								/>
								{#if showTagSuggestions && tagSuggestions.length > 0}
									<div class="absolute left-0 right-0 top-full mt-1 bg-card border border-border/50 rounded-lg shadow-lg z-50 max-h-40 overflow-y-auto">
										{#each tagSuggestions as suggestion, i}
											<button
												type="button"
												on:mousedown|preventDefault={() => applySuggestion(suggestion)}
												class="w-full text-left px-3 py-1.5 text-xs hover:bg-muted/50 transition-colors {i === selectedSuggestionIndex ? 'bg-muted/50 text-foreground' : 'text-muted-foreground'}"
											>
												<span class="text-foreground font-medium">{suggestion.slice(0, tagInput.trim().length)}</span><span>{suggestion.slice(tagInput.trim().length)}</span>
											</button>
										{/each}
									</div>
								{/if}
							</div>
						</div>
					</div>
				{/if}
			</main>

			<!-- Desktop Right Sidebar -->
			{#if showDesktopToc}
				<aside class="hidden lg:block w-[19rem] flex-shrink-0">
					<div class="sticky top-11 space-y-3 animate-slide-in-right">
						<button
							type="button"
							on:click={handleOpenPolisher}
							class="w-full flex items-center gap-2 bg-card/50 hover:bg-card rounded-xl border border-border/50 hover:border-primary/30 p-3 shadow-sm transition-all text-left group"
						>
							<div class="p-1.5 rounded-md bg-primary/10 text-primary group-hover:bg-primary/20 transition-colors">
								<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0018 18.469V19a2 2 0 01-2 2H8a2 2 0 01-2-2v-.531c0-.895.357-1.753.988-2.357l-.547-.546z" />
								</svg>
							</div>
							<div class="min-w-0">
								<div class="text-xs font-semibold text-foreground">AI 整理文本</div>
								<div class="text-[11px] text-muted-foreground truncate">去语气词 · 纠错 · 重组</div>
							</div>
						</button>

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
							<TableOfContents
								{content}
								tags={tags}
								tagInputValue={tagInput}
								onTagInput={(v) => (tagInput = v)}
								onTagAdd={addTagFromInput}
								onTagRemove={removeTag}
								onTagKeydown={handleTagKeydown}
								{allTags}
								{tagSuggestions}
								{showTagSuggestions}
								{selectedSuggestionIndex}
								onSuggestionSelect={applySuggestion}
								onSuggestionFocus={() => updateTagSuggestions(tagInput)}
								onSuggestionBlur={hideTagSuggestions}
							/>
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
						type="button"
						on:click={() => { showDrawer = false; handleOpenPolisher(); }}
						class="w-full flex items-center gap-2.5 px-2 py-1.5 rounded-lg hover:bg-muted/70 transition-all duration-200 group text-left"
					>
						<div class="p-1.5 rounded-md bg-purple-500/10 text-purple-500 group-hover:bg-purple-500/20 transition-colors">
							<svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0018 18.469V19a2 2 0 01-2 2H8a2 2 0 01-2-2v-.531c0-.895.357-1.753.988-2.357l-.547-.546z" />
							</svg>
						</div>
						<div class="min-w-0 text-left">
							<div class="text-xs font-medium text-foreground">AI 整理文本</div>
							<div class="text-[10px] text-muted-foreground truncate">去语气词 · 纠错 · 重组</div>
						</div>
					</button>

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
						href="/tags"
						class="flex items-center gap-2.5 px-2 py-1.5 rounded-lg hover:bg-muted/70 transition-all duration-200 group"
						on:click={() => showDrawer = false}
					>
						<div class="p-1.5 rounded-md bg-purple-500/10 text-purple-500 group-hover:bg-purple-500/20 transition-colors">
							<svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
									d="M7 7h.01M7 3h5a1.99 1.99 0 011.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A1.99 1.99 0 013 12V7a4 4 0 014-4z" />
							</svg>
						</div>
						<div class="min-w-0">
							<div class="text-xs font-medium text-foreground">标签云</div>
							<div class="text-[10px] text-muted-foreground truncate">浏览你的全部标签</div>
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
				<TableOfContents
					{content}
					tags={tags}
					tagInputValue={tagInput}
					onTagInput={(v) => (tagInput = v)}
					onTagAdd={addTagFromInput}
					onTagRemove={removeTag}
					onTagKeydown={handleTagKeydown}
					onNavigate={() => showDrawer = false}
					{allTags}
					{tagSuggestions}
					{showTagSuggestions}
					{selectedSuggestionIndex}
					onSuggestionSelect={applySuggestion}
					onSuggestionFocus={() => updateTagSuggestions(tagInput)}
					onSuggestionBlur={hideTagSuggestions}
				/>
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

<!-- AI Text Polisher Modal -->
{#if showPolisher}
	<TextPolisher
		bind:open={showPolisher}
		bind:sourceText={polishSourceText}
		onApply={handleApplyPolished}
	/>
{/if}

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

	.emoji-option-mobile {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		padding: 0.5rem;
		border-radius: 0.75rem;
		border: 1px solid hsl(var(--border) / 0.5);
		background: hsl(var(--muted) / 0.3);
		transition: transform 0.15s ease, border-color 0.15s ease, background-color 0.15s ease;
	}

	.emoji-option-mobile:hover {
		transform: translateY(-1px);
		background: hsl(var(--muted) / 0.6);
		border-color: hsl(var(--primary) / 0.25);
	}

	.diary-layout {
		width: 100%;
	}

	@media (min-width: 1024px) {
		.diary-main {
			flex: 1 1 auto;
			min-width: 0;
		}
	}
</style>

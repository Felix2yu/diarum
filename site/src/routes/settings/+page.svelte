<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { isAuthenticated } from '$lib/api/client';
	import {
		getApiToken,
		toggleApiToken,
		resetApiToken,
		getDiaryEmojiSettings,
		saveDiaryEmojiSettings,
		getMemosSettings,
		saveMemosSettings,
		resetMemosWebhookToken,
		type MemosSettings,
		type ApiTokenStatus
	} from '$lib/api/settings';
	import { getAISettings, saveAISettings, fetchModels, buildVectors, buildVectorsIncremental, getVectorStats, type AISettings, type ModelInfo, type BuildVectorsResult, type VectorStats } from '$lib/api/ai';
	import { exportDiaries, importDiaries, type ExportStats, type ImportStats, type ExportOptions } from '$lib/api/exportImport';
	import { defaultImageUploadSettings, getImageUploadSettings, saveImageUploadSettings, testCheveretoConnection, type ImageUploadProvider, type ImageUploadSettings } from '$lib/api/imageUpload';
	import { loadImageUploadSettings } from '$lib/stores/imageUpload';
	import Footer from '$lib/components/ui/Footer.svelte';
	import PageHeader from '$lib/components/ui/PageHeader.svelte';
	import {
		DEFAULT_MOOD_OPTIONS,
		DEFAULT_WEATHER_OPTIONS,
		MAX_DIARY_EMOJI_OPTION_COUNT,
		MAX_DIARY_EMOJI_OPTION_LENGTH,
		countDisplayChars,
		sanitizeMoodOptions,
		sanitizeWeatherOptions
	} from '$lib/utils/diaryEmoji';

	type SettingsTab = 'api-access' | 'mood-weather' | 'ai-assistant' | 'image-upload' | 'memos-sync' | 'data-management';

	const settingsTabs: { id: SettingsTab; label: string }[] = [
		{ id: 'ai-assistant', label: 'AI 助手' },
		{ id: 'mood-weather', label: '心情与天气' },
		{ id: 'api-access', label: 'API 访问' },
		{ id: 'memos-sync', label: 'Memos 同步' },
		{ id: 'image-upload', label: '图片上传' },
		{ id: 'data-management', label: '数据管理' }
	];

	let activeTab: SettingsTab = 'ai-assistant';

	function isSettingsTab(value: string): value is SettingsTab {
		return settingsTabs.some((tab) => tab.id === value);
	}

	function syncActiveTabFromHash() {
		if (typeof window === 'undefined') return;
		const hash = window.location.hash.replace('#', '');
		if (hash && isSettingsTab(hash)) {
			activeTab = hash;
		}
	}

	function setActiveTab(tab: SettingsTab) {
		activeTab = tab;

		if (typeof window === 'undefined') return;

		const url = new URL(window.location.href);
		url.hash = tab;
		history.replaceState(null, '', url);
	}

	let loading = true;
	let tokenStatus: ApiTokenStatus = { exists: false, enabled: false, token: '' };
	let copied = false;
	let resetting = false;
	let toggling = false;

	// Memos sync settings
	let memosSettings: MemosSettings = { enabled: false, base_url: '', webhook_url: '', token_exists: false };
	let originalMemosSettings: MemosSettings = { ...memosSettings };
	let memosSaving = false;
	let memosResetting = false;
	let memosCopied = false;
	let memosError = '';
	let memosSuccess = '';

	// Diary emoji settings
	let moodOptions: string[] = [];
	let weatherOptions: string[] = [];
	let originalMoodOptions: string[] = [];
	let originalWeatherOptions: string[] = [];
	let moodInput = '';
	let weatherInput = '';
	let emojiSettingsSaving = false;
	let emojiSettingsError = '';
	let emojiSettingsSuccess = '';
	type EmojiListType = 'mood' | 'weather';
	let draggingType: EmojiListType | null = null;
	let draggingIndex: number | null = null;
	let dragOverType: EmojiListType | null = null;
	let dragOverIndex: number | null = null;

	// AI Settings
	let aiSettings: AISettings = {
		api_key: '',
		base_url: '',
		chat_model: '',
		embedding_model: '',
		analysis_system_prompt: '',
		analysis_user_prefix: '',
		enabled: false
	};
	let originalAISettings: AISettings = { ...aiSettings };
	let aiSaving = false;
	let aiError = '';
	let aiSuccess = '';
	let models: ModelInfo[] = [];
	let fetchingModels = false;
	let modelsError = '';

	// Vector building
	let buildingVectors = false;
	let buildResult: BuildVectorsResult | null = null;
	let buildError = '';

	// Vector stats
	let vectorStats: VectorStats | null = null;
	let loadingStats = false;

	// Image upload settings
	let imageUploadSettingsLocal: ImageUploadSettings = structuredClone(defaultImageUploadSettings);
	let originalImageUploadSettings: ImageUploadSettings = structuredClone(defaultImageUploadSettings);
	let imageUploadSaving = false;
	let imageUploadError = '';
	let imageUploadSuccess = '';
	let cheveretoTesting = false;
	let cheveretoTestResult: { success: boolean; message: string } | null = null;

	// Data management (export/import)
	let exporting = false;
	let exportStats: ExportStats | null = null;
	let exportError = '';
	let importing = false;
	let importStats: ImportStats | null = null;
	let importError = '';
	let importFile: File | null = null;

	// Export options
	let exportOptions: ExportOptions = {
		date_range: '3m',
		include_diaries: true,
		include_media: true,
		include_conversations: true
	};
	let customStartDate = '';
	let customEndDate = '';
	let showExportOptions = true;

	async function loadTokenStatus() {
		tokenStatus = await getApiToken();
	}

	async function loadDiaryEmojiSettingsLocal() {
		const settings = await getDiaryEmojiSettings();
		moodOptions = [...settings.mood_options];
		weatherOptions = [...settings.weather_options];
		originalMoodOptions = [...settings.mood_options];
		originalWeatherOptions = [...settings.weather_options];
	}

	function addMoodOption() {
		emojiSettingsError = '';
		const value = moodInput.trim();
		if (!value) return;
		if (moodOptions.length >= MAX_DIARY_EMOJI_OPTION_COUNT) {
			emojiSettingsError = `最多可添加 ${MAX_DIARY_EMOJI_OPTION_COUNT} 个心情选项`;
			return;
		}
		if (countDisplayChars(value) > MAX_DIARY_EMOJI_OPTION_LENGTH) {
			emojiSettingsError = `心情选项最多 ${MAX_DIARY_EMOJI_OPTION_LENGTH} 个字符`;
			return;
		}
		if (moodOptions.includes(value)) {
			emojiSettingsError = '心情选项已存在';
			return;
		}
		moodOptions = [...moodOptions, value];
		moodInput = '';
	}

	function removeMoodOption(value: string) {
		emojiSettingsError = '';
		if (moodOptions.length <= 1) {
			emojiSettingsError = '至少保留一个心情选项';
			return;
		}
		moodOptions = moodOptions.filter((item) => item !== value);
	}

	function addWeatherOption() {
		emojiSettingsError = '';
		const value = weatherInput.trim();
		if (!value) return;
		if (weatherOptions.length >= MAX_DIARY_EMOJI_OPTION_COUNT) {
			emojiSettingsError = `最多可添加 ${MAX_DIARY_EMOJI_OPTION_COUNT} 个天气选项`;
			return;
		}
		if (countDisplayChars(value) > MAX_DIARY_EMOJI_OPTION_LENGTH) {
			emojiSettingsError = `天气选项最多 ${MAX_DIARY_EMOJI_OPTION_LENGTH} 个字符`;
			return;
		}
		if (weatherOptions.includes(value)) {
			emojiSettingsError = '天气选项已存在';
			return;
		}
		weatherOptions = [...weatherOptions, value];
		weatherInput = '';
	}

	function removeWeatherOption(value: string) {
		emojiSettingsError = '';
		if (weatherOptions.length <= 1) {
			emojiSettingsError = '至少保留一个天气选项';
			return;
		}
		weatherOptions = weatherOptions.filter((item) => item !== value);
	}

	function restoreMoodDefaults() {
		emojiSettingsError = '';
		emojiSettingsSuccess = '';
		moodOptions = [...DEFAULT_MOOD_OPTIONS];
	}

	function restoreWeatherDefaults() {
		emojiSettingsError = '';
		emojiSettingsSuccess = '';
		weatherOptions = [...DEFAULT_WEATHER_OPTIONS];
	}

	function restoreAllDefaults() {
		emojiSettingsError = '';
		emojiSettingsSuccess = '';
		moodOptions = [...DEFAULT_MOOD_OPTIONS];
		weatherOptions = [...DEFAULT_WEATHER_OPTIONS];
	}

	function reorderOptions(options: string[], fromIndex: number, toIndex: number): string[] {
		if (fromIndex === toIndex) return options;
		const next = [...options];
		const [moved] = next.splice(fromIndex, 1);
		next.splice(toIndex, 0, moved);
		return next;
	}

	function handleDragStart(type: EmojiListType, index: number) {
		draggingType = type;
		draggingIndex = index;
		dragOverType = type;
		dragOverIndex = index;
	}

	function handleDragOver(event: DragEvent, type: EmojiListType, index: number) {
		event.preventDefault();
		if (draggingType !== type) return;
		dragOverType = type;
		dragOverIndex = index;
	}

	function handleDrop(type: EmojiListType, index: number) {
		if (draggingType !== type || draggingIndex === null) {
			clearDragState();
			return;
		}

		emojiSettingsError = '';
		emojiSettingsSuccess = '';
		if (type === 'mood') {
			moodOptions = reorderOptions(moodOptions, draggingIndex, index);
		} else {
			weatherOptions = reorderOptions(weatherOptions, draggingIndex, index);
		}

		clearDragState();
	}

	function handleDropToEnd(type: EmojiListType) {
		if (draggingType !== type || draggingIndex === null) {
			clearDragState();
			return;
		}

		emojiSettingsError = '';
		emojiSettingsSuccess = '';
		if (type === 'mood') {
			moodOptions = reorderOptions(moodOptions, draggingIndex, moodOptions.length - 1);
		} else {
			weatherOptions = reorderOptions(weatherOptions, draggingIndex, weatherOptions.length - 1);
		}

		clearDragState();
	}

	function clearDragState() {
		draggingType = null;
		draggingIndex = null;
		dragOverType = null;
		dragOverIndex = null;
	}

	async function handleSaveEmojiSettings() {
		emojiSettingsError = '';
		emojiSettingsSuccess = '';

		if (moodOptions.length < 1 || weatherOptions.length < 1) {
			emojiSettingsError = '心情和天气至少各保留一个选项';
			return;
		}

		emojiSettingsSaving = true;
		try {
			const sanitizedMoodOptions = sanitizeMoodOptions(moodOptions);
			const sanitizedWeatherOptions = sanitizeWeatherOptions(weatherOptions);

			await saveDiaryEmojiSettings({
				mood_options: sanitizedMoodOptions,
				weather_options: sanitizedWeatherOptions
			});

			moodOptions = [...sanitizedMoodOptions];
			weatherOptions = [...sanitizedWeatherOptions];
			originalMoodOptions = [...sanitizedMoodOptions];
			originalWeatherOptions = [...sanitizedWeatherOptions];
			emojiSettingsSuccess = '心情与天气选项已成功保存';
			setTimeout(() => emojiSettingsSuccess = '', 3000);
		} catch (e) {
			emojiSettingsError = e instanceof Error ? e.message : '保存心情与天气选项失败';
		}
		emojiSettingsSaving = false;
	}

	async function handleToggle() {
		toggling = true;
		try {
			tokenStatus = await toggleApiToken();
		} catch (e) {
			console.error('切换 API token 失败');
		}
		toggling = false;
	}

	async function handleReset() {
		if (!confirm('确定要重置您的 API token 吗？任何现有的集成都将停止工作。')) {
			return;
		}
		resetting = true;
		try {
			tokenStatus = await resetApiToken();
		} catch (e) {
			console.error('重置 API token 失败');
		}
		resetting = false;
	}

	async function copyToken() {
		if (tokenStatus.token) {
			await navigator.clipboard.writeText(tokenStatus.token);
			copied = true;
			setTimeout(() => copied = false, 2000);
		}
	}

	async function loadMemosSettingsLocal() {
		memosSettings = await getMemosSettings();
		originalMemosSettings = JSON.parse(JSON.stringify(memosSettings));
	}

	async function handleSaveMemosSettings() {
		memosSaving = true;
		memosError = '';
		memosSuccess = '';
		try {
			memosSettings = await saveMemosSettings({
				enabled: memosSettings.enabled,
				base_url: memosSettings.base_url
			});
			originalMemosSettings = JSON.parse(JSON.stringify(memosSettings));
			memosSuccess = 'Memos 同步设置已成功保存';
			setTimeout(() => memosSuccess = '', 3000);
		} catch (e) {
			memosError = e instanceof Error ? e.message : '保存 Memos 设置失败';
		}
		memosSaving = false;
	}

	async function handleResetMemosWebhookToken() {
		if (!confirm('确定要重置 Memos webhook URL 吗？之前在 Memos 中配置的旧 URL 将停止工作。')) {
			return;
		}
		memosResetting = true;
		memosError = '';
		try {
			memosSettings = await resetMemosWebhookToken();
			originalMemosSettings = JSON.parse(JSON.stringify(memosSettings));
			memosSuccess = 'Memos Webhook URL 已成功重置';
			setTimeout(() => memosSuccess = '', 3000);
		} catch (e) {
			memosError = e instanceof Error ? e.message : '重置 Memos Webhook URL 失败';
		}
		memosResetting = false;
	}

	async function copyMemosWebhookURL() {
		if (memosSettings.webhook_url) {
			await navigator.clipboard.writeText(memosSettings.webhook_url);
			memosCopied = true;
			setTimeout(() => memosCopied = false, 2000);
		}
	}

	function getBaseUrl(): string {
		if (typeof window !== 'undefined') {
			return window.location.origin;
		}
		return '';
	}

	// AI Settings functions
	async function loadAISettings() {
		aiSettings = await getAISettings();
		originalAISettings = JSON.parse(JSON.stringify(aiSettings));
		// Initialize models array with configured models so they display before refresh
		const initialModels: ModelInfo[] = [];
		if (aiSettings.chat_model) {
			initialModels.push({ id: aiSettings.chat_model, object: 'model' });
		}
		if (aiSettings.embedding_model && aiSettings.embedding_model !== aiSettings.chat_model) {
			initialModels.push({ id: aiSettings.embedding_model, object: 'model' });
		}
		models = initialModels;
	}

	async function handleFetchModels() {
		if (!aiSettings.api_key || !aiSettings.base_url) {
			modelsError = '请先输入 API Key 和 Base URL';
			return;
		}

		fetchingModels = true;
		modelsError = '';
		try {
			models = await fetchModels(aiSettings.api_key, aiSettings.base_url);
		} catch (e) {
			modelsError = e instanceof Error ? e.message : '获取模型列表失败';
		}
		fetchingModels = false;
	}

	async function handleSaveAISettings() {
		aiError = '';
		aiSuccess = '';

		// Validate: if enabling, all fields must be filled
		if (aiSettings.enabled) {
			if (!aiSettings.api_key || !aiSettings.base_url || !aiSettings.chat_model || !aiSettings.embedding_model) {
				aiError = '启用 AI 功能前请填写所有字段';
				return;
			}
		}

		aiSaving = true;
		try {
			await saveAISettings(aiSettings);
			originalAISettings = JSON.parse(JSON.stringify(aiSettings));
			aiSuccess = 'AI 设置已成功保存';
			setTimeout(() => aiSuccess = '', 3000);
		} catch (e) {
			aiError = e instanceof Error ? e.message : '保存 AI 设置失败';
		}
		aiSaving = false;
	}

	async function handleBuildVectors(incremental: boolean = false) {
		if (!aiSettings.enabled) {
			buildError = '请先启用 AI 功能';
			return;
		}

		buildingVectors = true;
		buildError = '';
		buildResult = null;

		try {
			if (incremental) {
				buildResult = await buildVectorsIncremental();
			} else {
				buildResult = await buildVectors();
			}
			// Refresh stats after building
			await loadVectorStats();
		} catch (e) {
			buildError = e instanceof Error ? e.message : '构建向量失败';
		}
		buildingVectors = false;
	}

	async function loadVectorStats() {
		if (!aiSettings.enabled) return;

		loadingStats = true;
		try {
			vectorStats = await getVectorStats();
		} catch (e) {
			console.error('Failed to load vector stats:', e);
			vectorStats = null;
		}
		loadingStats = false;
	}

	// Check if AI can be enabled
	$: canEnableAI = aiSettings.api_key && aiSettings.base_url && aiSettings.chat_model && aiSettings.embedding_model;

	$: emojiSettingsChanged =
		JSON.stringify(moodOptions) !== JSON.stringify(originalMoodOptions) ||
		JSON.stringify(weatherOptions) !== JSON.stringify(originalWeatherOptions);

	$: memosSettingsChanged = memosSettings.enabled !== originalMemosSettings.enabled ||
		memosSettings.base_url !== originalMemosSettings.base_url;

	// Check if AI settings have changed
	$: aiSettingsChanged = aiSettings.api_key !== originalAISettings.api_key ||
		aiSettings.base_url !== originalAISettings.base_url ||
		aiSettings.chat_model !== originalAISettings.chat_model ||
		aiSettings.embedding_model !== originalAISettings.embedding_model ||
		(aiSettings.analysis_system_prompt ?? '') !== (originalAISettings.analysis_system_prompt ?? '') ||
		(aiSettings.analysis_user_prefix ?? '') !== (originalAISettings.analysis_user_prefix ?? '') ||
		aiSettings.enabled !== originalAISettings.enabled;

	// Embedding model keywords for sorting
	const embeddingKeywords = ['embed', 'bge', 'e5', 'voyage', 'jina'];

	// Check if a model is likely an embedding model
	function isEmbeddingModel(modelId: string): boolean {
		const lower = modelId.toLowerCase();
		return embeddingKeywords.some(keyword => lower.includes(keyword));
	}

	// Check if a model is likely a chat model (not embedding)
	function isChatModel(modelId: string): boolean {
		return !isEmbeddingModel(modelId);
	}

	// Sorted models for embedding selection (embedding models first)
	$: embeddingModels = [...models].sort((a, b) => {
		const aIsEmbed = isEmbeddingModel(a.id);
		const bIsEmbed = isEmbeddingModel(b.id);
		if (aIsEmbed && !bIsEmbed) return -1;
		if (!aIsEmbed && bIsEmbed) return 1;
		return a.id.localeCompare(b.id);
	});

	// Sorted models for chat selection (chat models first)
	$: chatModels = [...models].sort((a, b) => {
		const aIsChat = isChatModel(a.id);
		const bIsChat = isChatModel(b.id);
		if (aIsChat && !bIsChat) return -1;
		if (!aIsChat && bIsChat) return 1;
		return a.id.localeCompare(b.id);
	});

	// Image upload functions
	async function loadImageUploadSettingsLocal() {
		imageUploadSettingsLocal = await getImageUploadSettings();
		originalImageUploadSettings = JSON.parse(JSON.stringify(imageUploadSettingsLocal));
	}

	$: canTestChevereto = imageUploadSettingsLocal.chevereto.domain && imageUploadSettingsLocal.chevereto.api_key;

	$: imageUploadSettingsChanged = JSON.stringify(imageUploadSettingsLocal) !== JSON.stringify(originalImageUploadSettings);

	async function handleTestChevereto() {
		if (!imageUploadSettingsLocal.chevereto.domain || !imageUploadSettingsLocal.chevereto.api_key) {
			imageUploadError = '请先输入域名和 API Key';
			return;
		}
		cheveretoTesting = true;
		cheveretoTestResult = null;
		imageUploadError = '';
		try {
			cheveretoTestResult = await testCheveretoConnection(
				imageUploadSettingsLocal.chevereto.domain,
				imageUploadSettingsLocal.chevereto.api_key
			);
		} catch (e) {
			imageUploadError = e instanceof Error ? e.message : '连接测试失败';
		}
		cheveretoTesting = false;
	}

	async function handleSaveImageUploadSettings() {
		imageUploadError = '';
		imageUploadSuccess = '';

		if (imageUploadSettingsLocal.provider === 's3') {
			if (!imageUploadSettingsLocal.s3.bucket || !imageUploadSettingsLocal.s3.region || !imageUploadSettingsLocal.s3.access_key || !imageUploadSettingsLocal.s3.secret) {
				imageUploadError = 'S3 需要填写 Bucket、region、access key 和 secret';
				return;
			}
		}
		if (imageUploadSettingsLocal.provider === 'chevereto') {
			if (!imageUploadSettingsLocal.chevereto.domain || !imageUploadSettingsLocal.chevereto.api_key) {
				imageUploadError = 'Chevereto 需要填写域名和 API Key';
				return;
			}
		}

		imageUploadSaving = true;
		try {
			const result = await saveImageUploadSettings(imageUploadSettingsLocal);
			imageUploadSettingsLocal = result.settings ?? imageUploadSettingsLocal;
			originalImageUploadSettings = JSON.parse(JSON.stringify(imageUploadSettingsLocal));
			await loadImageUploadSettings();
			imageUploadSuccess = '图片上传设置已成功保存';
			setTimeout(() => imageUploadSuccess = '', 3000);
		} catch (e) {
			imageUploadError = e instanceof Error ? e.message : '保存图片上传设置失败';
		}
		imageUploadSaving = false;
	}

	async function handleExport() {
		exporting = true;
		exportError = '';
		exportStats = null;
		try {
			// Build options with custom dates if needed
			const options: ExportOptions = { ...exportOptions };
			if (options.date_range === 'custom') {
				options.start_date = customStartDate;
				options.end_date = customEndDate;
			}
			exportStats = await exportDiaries(options);
		} catch (e) {
			exportError = e instanceof Error ? e.message : '导出失败';
		}
		exporting = false;
	}

	function handleImportFileChange(e: Event) {
		const input = e.target as HTMLInputElement;
		importFile = input.files?.[0] || null;
	}

	async function handleImport() {
		if (!importFile) return;
		importing = true;
		importError = '';
		importStats = null;
		try {
			importStats = await importDiaries(importFile);
		} catch (e) {
			importError = e instanceof Error ? e.message : '导入失败';
		}
		importing = false;
	}

	onMount(() => {
		syncActiveTabFromHash();

		const handleHashChange = () => {
			syncActiveTabFromHash();
		};

		window.addEventListener('hashchange', handleHashChange);

		const initialize = async () => {
		if (!$isAuthenticated) {
			goto('/login');
			return;
		}

		loading = true;
		await Promise.all([loadTokenStatus(), loadDiaryEmojiSettingsLocal(), loadMemosSettingsLocal(), loadAISettings(), loadImageUploadSettingsLocal()]);
		loading = false;
		// Load vector stats if AI is enabled
		if (aiSettings.enabled) {
			await loadVectorStats();
		}
		};

		void initialize();

		return () => {
			window.removeEventListener('hashchange', handleHashChange);
		};
	});
</script>

<svelte:head>
	<title>设置 - 吾身</title>
</svelte:head>

<div class="min-h-screen bg-background">
	<PageHeader title="设置" />

	<!-- Main Content -->
	<div class="max-w-6xl mx-auto px-4 py-6">
		<div class="flex justify-center">
			<main class="w-full max-w-4xl">
				<div class="mb-4 space-y-3">
					<div class="sm:hidden">
						<label for="settings-tab-select" class="sr-only">选择设置分区</label>
						<div class="relative">
							<select
								id="settings-tab-select"
								value={activeTab}
								on:change={(event) => setActiveTab((event.currentTarget as HTMLSelectElement).value as SettingsTab)}
								class="w-full pl-3 pr-9 py-2 bg-card border border-border/60 rounded-lg text-sm text-foreground appearance-none focus:outline-none focus:ring-2 focus:ring-primary"
							>
								{#each settingsTabs as tab}
									<option value={tab.id}>{tab.label}</option>
								{/each}
							</select>
							<svg class="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
							</svg>
						</div>
					</div>
					<div class="hidden sm:flex gap-2 overflow-x-auto pb-1">
						{#each settingsTabs as tab}
							<button
								on:click={() => setActiveTab(tab.id)}
								class="px-3 py-1.5 rounded-lg text-sm whitespace-nowrap border transition-colors {activeTab === tab.id ? 'bg-primary text-primary-foreground border-primary' : 'bg-card text-foreground border-border/60 hover:bg-muted/50'}"
							>
								{tab.label}
							</button>
						{/each}
					</div>
				</div>
				{#if loading}
			<div class="flex flex-col items-center justify-center py-20 gap-3">
				<svg class="w-6 h-6 animate-spin text-primary" fill="none" viewBox="0 0 24 24">
					<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
					<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
				</svg>
				<div class="text-muted-foreground text-sm">加载中...</div>
			</div>
		{:else}
			<div class="space-y-6">
				{#if activeTab === 'api-access'}
				<!-- API Settings Section -->
				<div id="api-access" class="bg-card rounded-xl shadow-sm border border-border/50 p-6 animate-fade-in scroll-mt-16">
					<h2 class="text-lg font-semibold text-foreground mb-4">API 访问</h2>
					<p class="text-sm text-muted-foreground mb-6">
						Enable API access to read and write your diary entries programmatically. Use your API token to authenticate requests.
					</p>

					<!-- Enable/Disable Toggle -->
					<div class="flex items-center justify-between py-4 border-b border-border/50">
						<div>
							<div class="font-medium text-foreground">启用 API</div>
							<div class="text-sm text-muted-foreground">允许外部读取和写入您的日记数据</div>
						</div>
						<button
							on:click={handleToggle}
							disabled={toggling}
							aria-label="切换 API 访问"
							class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors duration-200 focus:outline-none focus:ring-2 focus:ring-primary focus:ring-offset-2 {tokenStatus.enabled ? 'bg-primary' : 'bg-muted'}"
						>
							<span
								class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform duration-200 {tokenStatus.enabled ? 'translate-x-6' : 'translate-x-1'}"
							></span>
						</button>
					</div>

					{#if tokenStatus.enabled && tokenStatus.token}
						<!-- API Token Display -->
						<div class="py-4 border-b border-border/50">
							<div class="font-medium text-foreground mb-2">您的 API Token</div>
							<div class="flex items-center gap-2">
								<code class="flex-1 px-3 py-2 bg-muted rounded-lg text-sm font-mono text-foreground overflow-x-auto">
									{tokenStatus.token}
								</code>
								<button
									on:click={copyToken}
									class="px-3 py-2 text-sm bg-muted hover:bg-muted/80 rounded-lg transition-colors duration-200"
								>
									{copied ? '已复制！' : '复制'}
								</button>
							</div>
							<p class="text-xs text-muted-foreground mt-2">
								请妥善保管此 token。任何持有此 token 的人都可以读取、修改或删除您的日记内容。
							</p>
						</div>

						<!-- 重置 Token -->
						<div class="py-4 border-b border-border/50">
							<div class="flex items-center justify-between">
								<div>
									<div class="font-medium text-foreground">重置 Token</div>
									<div class="text-sm text-muted-foreground">生成新的 API token</div>
								</div>
								<button
									on:click={handleReset}
									disabled={resetting}
									class="px-4 py-2 text-sm bg-destructive/10 text-destructive hover:bg-destructive/20 rounded-lg transition-colors duration-200 disabled:opacity-50"
								>
									{resetting ? '重置中...' : '重置 Token'}
								</button>
							</div>
						</div>

						<!-- API Documentation -->
						<div class="py-4">
							<div class="font-medium text-foreground mb-3">API 使用说明</div>
							<div class="space-y-4 text-sm">
								<div>
									<div class="text-muted-foreground mb-1">按日期获取日记（GET）：</div>
									<code class="block px-3 py-2 bg-muted rounded-lg font-mono text-xs overflow-x-auto">
										GET {getBaseUrl()}/api/v1/diaries?token={tokenStatus.token}&date=YYYY-MM-DD
									</code>
								</div>
								<div>
									<div class="text-muted-foreground mb-1">按日期范围获取日记（GET）：</div>
									<code class="block px-3 py-2 bg-muted rounded-lg font-mono text-xs overflow-x-auto">
										GET {getBaseUrl()}/api/v1/diaries?token={tokenStatus.token}&start=YYYY-MM-DD&end=YYYY-MM-DD
									</code>
								</div>
								<div>
									<div class="text-muted-foreground mb-1">创建或更新日记（POST）：</div>
									<code class="block px-3 py-2 bg-muted rounded-lg font-mono text-xs overflow-x-auto whitespace-pre-wrap">
POST {getBaseUrl()}/api/v1/diaries?token={tokenStatus.token}
Content-Type: application/json

{"{"}"date":"2025-01-15","content":"今天是个好日子","mood":"开心","weather":"晴"{"}"}
									</code>
								</div>
								<div>
									<div class="text-muted-foreground mb-1">按 ID 更新日记（PUT）：</div>
									<code class="block px-3 py-2 bg-muted rounded-lg font-mono text-xs overflow-x-auto whitespace-pre-wrap">
PUT {getBaseUrl()}/api/v1/diaries/&lt;diary-id&gt;?token={tokenStatus.token}
Content-Type: application/json

{"{"}"content":"更新后的内容","mood":"平静"{"}"}
									</code>
								</div>
								<div>
									<div class="text-muted-foreground mb-1">按 ID 删除日记（DELETE）：</div>
									<code class="block px-3 py-2 bg-muted rounded-lg font-mono text-xs overflow-x-auto">
										DELETE {getBaseUrl()}/api/v1/diaries/&lt;diary-id&gt;?token={tokenStatus.token}
									</code>
								</div>
								<div>
									<div class="text-muted-foreground mb-1">按日期删除日记（DELETE）：</div>
									<code class="block px-3 py-2 bg-muted rounded-lg font-mono text-xs overflow-x-auto">
										DELETE {getBaseUrl()}/api/v1/diaries?token={tokenStatus.token}&date=YYYY-MM-DD
									</code>
								</div>
								<div>
									<div class="text-muted-foreground mb-1">curl 读取示例：</div>
									<code class="block px-3 py-2 bg-muted rounded-lg font-mono text-xs overflow-x-auto whitespace-pre-wrap">
curl "{getBaseUrl()}/api/v1/diaries?token={tokenStatus.token}&date={new Date().toISOString().split('T')[0]}"
									</code>
								</div>
								<div>
									<div class="text-muted-foreground mb-1">curl 写入示例：</div>
									<code class="block px-3 py-2 bg-muted rounded-lg font-mono text-xs overflow-x-auto whitespace-pre-wrap">
curl -X POST "{getBaseUrl()}/api/v1/diaries?token={tokenStatus.token}" \
  -H "Content-Type: application/json" \
  -d '{"{"}"date":"{new Date().toISOString().split('T')[0]}","content":"API 测试写入","mood":"","weather":""{"}"}'
									</code>
								</div>
							</div>
						</div>
					{/if}
				</div>
				{/if}

				{#if activeTab === 'memos-sync'}
				<!-- Memos 同步 Section -->
				<div id="memos-sync" class="bg-card rounded-xl shadow-sm border border-border/50 p-6 animate-fade-in scroll-mt-16">
					<h2 class="text-lg font-semibold text-foreground mb-4">Memos 同步</h2>
					<p class="text-sm text-muted-foreground mb-6">
						接收 Memos webhook 事件，并根据每条 memo 的创建日期将同步的 memo 块追加到日记中。更新操作会按 memo ID 替换匹配的块，删除操作会移除该块。
					</p>

					{#if memosError}
						<div class="mb-4 p-3 bg-destructive/10 text-destructive rounded-lg text-sm">
							{memosError}
						</div>
					{/if}

					{#if memosSuccess}
						<div class="mb-4 p-3 bg-green-500/10 text-green-600 rounded-lg text-sm">
							{memosSuccess}
						</div>
					{/if}

					<div class="flex items-center justify-between py-4 border-b border-border/50">
						<div>
							<div class="font-medium text-foreground">启用 Memos Webhook</div>
							<div class="text-sm text-muted-foreground">为 Memos 生成专用的 Webhook URL 用于接收同步消息</div>
						</div>
						<button
							on:click={() => memosSettings.enabled = !memosSettings.enabled}
							aria-label="切换 Memos 同步"
							class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors duration-200 focus:outline-none focus:ring-2 focus:ring-primary focus:ring-offset-2 {memosSettings.enabled ? 'bg-primary' : 'bg-muted'}"
						>
							<span
								class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform duration-200 {memosSettings.enabled ? 'translate-x-6' : 'translate-x-1'}"
							></span>
						</button>
					</div>

					<div class="py-4 border-b border-border/50">
						<label for="memos-base-url" class="block font-medium text-foreground mb-2">Memos 基础 URL</label>
						<input
							id="memos-base-url"
							type="url"
							bind:value={memosSettings.base_url}
							placeholder="https://memos.example.com"
							class="w-full px-3 py-2 bg-muted rounded-lg text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary"
						/>
						<p class="text-xs text-muted-foreground mt-1">可选。用于在每个同步块中记录 memo URL，例如 https://memos.example.com/m/123。</p>
					</div>

					{#if memosSettings.enabled && memosSettings.webhook_url}
						<div class="py-4 border-b border-border/50">
							<div class="font-medium text-foreground mb-2">Webhook URL</div>
							<div class="flex items-center gap-2">
								<code class="flex-1 px-3 py-2 bg-muted rounded-lg text-sm font-mono text-foreground overflow-x-auto">
									{memosSettings.webhook_url}
								</code>
								<button
									on:click={copyMemosWebhookURL}
									class="px-3 py-2 text-sm bg-muted hover:bg-muted/80 rounded-lg transition-colors duration-200"
								>
									{memosCopied ? '已复制！' : '复制'}
								</button>
							</div>
							<p class="text-xs text-muted-foreground mt-2">将此 URL 粘贴到 Memos webhook 设置中。请妥善保密，因为它可以向您的日记写入同步的 memo 块。</p>
						</div>

						<div class="py-4 border-b border-border/50">
							<div class="flex items-center justify-between gap-4">
								<div>
									<div class="font-medium text-foreground">重置 Webhook URL</div>
									<div class="text-sm text-muted-foreground">如果旧 URL 已泄露，请重新生成专用的 Webhook URL</div>
								</div>
								<button
									on:click={handleResetMemosWebhookToken}
									disabled={memosResetting}
									class="px-4 py-2 text-sm bg-destructive/10 text-destructive hover:bg-destructive/20 rounded-lg transition-colors duration-200 disabled:opacity-50"
								>
									{memosResetting ? '重置中...' : '重置 URL'}
								</button>
							</div>
						</div>
					{/if}

					<div class="pt-4 flex items-center gap-3">
						<button
							on:click={handleSaveMemosSettings}
							disabled={memosSaving || !memosSettingsChanged}
							class="px-4 py-2 bg-primary text-primary-foreground rounded-lg hover:bg-primary/90 transition-colors duration-200 disabled:opacity-50 disabled:cursor-not-allowed"
						>
							{memosSaving ? '保存中...' : '保存 Memos 设置'}
						</button>
					</div>
				</div>
				{/if}

				{#if activeTab === 'mood-weather'}
				<!-- 心情与天气 Section -->
				<div id="mood-weather" class="bg-card rounded-xl shadow-sm border border-border/50 p-6 animate-fade-in scroll-mt-16">
					<div class="flex items-center justify-between gap-3 mb-4">
						<h2 class="text-lg font-semibold text-foreground">心情与天气</h2>
						<button
							on:click={restoreAllDefaults}
							class="px-3 py-1.5 text-xs bg-muted hover:bg-muted/80 rounded-lg transition-colors duration-200"
						>
							恢复所有默认值
						</button>
					</div>
					<p class="text-sm text-muted-foreground mb-6">
						自定义日记编辑器中显示的选项。添加任意表情或最多 {MAX_DIARY_EMOJI_OPTION_LENGTH} 个字符的短文本，每个列表至少保留 1 个、最多 {MAX_DIARY_EMOJI_OPTION_COUNT} 个条目，然后拖动排序并保存。
					</p>

					{#if emojiSettingsError}
						<div class="mb-4 p-3 bg-red-500/10 text-red-600 rounded-lg text-sm">
							{emojiSettingsError}
						</div>
					{/if}

					{#if emojiSettingsSuccess}
						<div class="mb-4 p-3 bg-green-500/10 text-green-600 rounded-lg text-sm">
							{emojiSettingsSuccess}
						</div>
					{/if}

					<div class="grid grid-cols-1 lg:grid-cols-2 gap-4 pb-4 border-b border-border/50">
						<div class="rounded-xl border border-border/50 p-4">
							<div class="flex items-center justify-between gap-3 mb-2">
								<div class="font-medium text-foreground">心情选项</div>
								<button
									on:click={restoreMoodDefaults}
									class="px-2.5 py-1 text-xs bg-muted hover:bg-muted/80 rounded-lg transition-colors duration-200"
								>
									恢复默认值
								</button>
							</div>
							<div class="flex items-center gap-2 mb-3">
								<input
									type="text"
									bind:value={moodInput}
									maxlength={MAX_DIARY_EMOJI_OPTION_LENGTH}
									placeholder="例如 😊"
									class="flex-1 px-3 py-2 bg-muted rounded-lg text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary"
									on:keydown={(event) => {
										if (event.key === 'Enter') {
											event.preventDefault();
											addMoodOption();
										}
									}}
								/>
								<button
									on:click={addMoodOption}
									class="px-3 py-2 text-sm bg-muted hover:bg-muted/80 rounded-lg transition-colors duration-200"
								>
									添加
								</button>
							</div>
							<div class="text-xs text-muted-foreground mb-3">每个选项最多 {MAX_DIARY_EMOJI_OPTION_LENGTH} 个字符，总计最多 {MAX_DIARY_EMOJI_OPTION_COUNT} 个心情选项。请保留至少一个。拖动可重新排序。</div>
							<div class="flex flex-wrap gap-2">
								{#if moodOptions.length === 0}
									<div class="text-sm text-muted-foreground">暂无心情选项</div>
								{:else}
									{#each moodOptions as option, index}
										<div
											draggable="true"
											role="listitem"
											on:dragstart={() => handleDragStart('mood', index)}
											on:dragover={(event) => handleDragOver(event, 'mood', index)}
											on:drop={() => handleDrop('mood', index)}
											on:dragend={clearDragState}
											class="relative w-14 h-14 rounded-xl border transition-colors flex items-center justify-center cursor-grab select-none {dragOverType === 'mood' && dragOverIndex === index ? 'border-primary bg-primary/10' : 'bg-muted/70 border-border/60'}"
											title={option}
										>
											<button
												on:click|stopPropagation={() => removeMoodOption(option)}
												class="absolute -top-1.5 -right-1.5 w-4 h-4 rounded-full bg-background border border-border text-muted-foreground hover:text-destructive hover:border-destructive/50 transition-colors flex items-center justify-center"
												aria-label={`Remove mood option ${option}`}
											>
												<svg class="w-2.5 h-2.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
													<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.25" d="M6 6l12 12M18 6l-12 12" />
												</svg>
											</button>
											<span class="text-xl leading-none">{option}</span>
										</div>
									{/each}
									<div
										role="status"
										on:dragover={(event) => handleDragOver(event, 'mood', moodOptions.length - 1)}
										on:drop={() => handleDropToEnd('mood')}
										class="h-14 px-3 rounded-xl border border-dashed text-xs text-muted-foreground flex items-center {dragOverType === 'mood' ? 'border-primary bg-primary/5' : 'border-border/60'}"
									>
										拖到末尾
									</div>
								{/if}
							</div>
						</div>

						<div class="rounded-xl border border-border/50 p-4">
							<div class="flex items-center justify-between gap-3 mb-2">
								<div class="font-medium text-foreground">天气选项</div>
								<button
									on:click={restoreWeatherDefaults}
									class="px-2.5 py-1 text-xs bg-muted hover:bg-muted/80 rounded-lg transition-colors duration-200"
								>
									恢复默认值
								</button>
							</div>
							<div class="flex items-center gap-2 mb-3">
								<input
									type="text"
									bind:value={weatherInput}
									maxlength={MAX_DIARY_EMOJI_OPTION_LENGTH}
									placeholder="例如 ☀️"
									class="flex-1 px-3 py-2 bg-muted rounded-lg text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary"
									on:keydown={(event) => {
										if (event.key === 'Enter') {
											event.preventDefault();
											addWeatherOption();
										}
									}}
								/>
								<button
									on:click={addWeatherOption}
									class="px-3 py-2 text-sm bg-muted hover:bg-muted/80 rounded-lg transition-colors duration-200"
								>
									添加
								</button>
							</div>
							<div class="text-xs text-muted-foreground mb-3">每个选项最多 {MAX_DIARY_EMOJI_OPTION_LENGTH} 个字符，总计最多 {MAX_DIARY_EMOJI_OPTION_COUNT} 个天气选项。请保留至少一个。拖动可重新排序。</div>
							<div class="flex flex-wrap gap-2">
								{#if weatherOptions.length === 0}
									<div class="text-sm text-muted-foreground">暂无天气选项</div>
								{:else}
									{#each weatherOptions as option, index}
										<div
											draggable="true"
											role="listitem"
											on:dragstart={() => handleDragStart('weather', index)}
											on:dragover={(event) => handleDragOver(event, 'weather', index)}
											on:drop={() => handleDrop('weather', index)}
											on:dragend={clearDragState}
											class="relative w-14 h-14 rounded-xl border transition-colors flex items-center justify-center cursor-grab select-none {dragOverType === 'weather' && dragOverIndex === index ? 'border-primary bg-primary/10' : 'bg-muted/70 border-border/60'}"
											title={option}
										>
											<button
												on:click|stopPropagation={() => removeWeatherOption(option)}
												class="absolute -top-1.5 -right-1.5 w-4 h-4 rounded-full bg-background border border-border text-muted-foreground hover:text-destructive hover:border-destructive/50 transition-colors flex items-center justify-center"
												aria-label={`Remove weather option ${option}`}
											>
												<svg class="w-2.5 h-2.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
													<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.25" d="M6 6l12 12M18 6l-12 12" />
												</svg>
											</button>
											<span class="text-xl leading-none">{option}</span>
										</div>
									{/each}
									<div
										role="status"
										on:dragover={(event) => handleDragOver(event, 'weather', weatherOptions.length - 1)}
										on:drop={() => handleDropToEnd('weather')}
										class="h-14 px-3 rounded-xl border border-dashed text-xs text-muted-foreground flex items-center {dragOverType === 'weather' ? 'border-primary bg-primary/5' : 'border-border/60'}"
									>
										拖到末尾
									</div>
								{/if}
							</div>
						</div>
					</div>

					<div class="pt-4 flex items-center gap-3">
						<button
							on:click={handleSaveEmojiSettings}
							disabled={emojiSettingsSaving || !emojiSettingsChanged}
							class="px-4 py-2 bg-primary text-primary-foreground rounded-lg hover:bg-primary/90 transition-colors duration-200 disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
						>
							{#if emojiSettingsSaving}
								<svg class="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
									<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
									<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
								</svg>
								保存中...
							{:else}
								保存心情与天气设置
							{/if}
						</button>
						{#if emojiSettingsSuccess}
							<span class="text-sm text-green-600 flex items-center gap-1 animate-fade-in">
								<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
								</svg>
								已保存
							</span>
						{/if}
					</div>
				</div>
				{/if}

				{#if activeTab === 'ai-assistant'}
				<!-- AI Settings Section -->
				<div id="ai-assistant" class="bg-card rounded-xl shadow-sm border border-border/50 p-6 animate-fade-in scroll-mt-16">
					<h2 class="text-lg font-semibold text-foreground mb-4">AI 助手</h2>
					<p class="text-sm text-muted-foreground mb-6">
						配置 AI 服务以实现智能日记分析与对话。支持与 OpenAI 兼容的 API。
					</p>

					{#if aiError}
						<div class="mb-4 p-3 bg-destructive/10 text-destructive rounded-lg text-sm">
							{aiError}
						</div>
					{/if}

					{#if aiSuccess}
						<div class="mb-4 p-3 bg-green-500/10 text-green-600 rounded-lg text-sm">
							{aiSuccess}
						</div>
					{/if}

					<!-- API Key -->
					<div class="py-4 border-b border-border/50">
						<label for="ai-api-key" class="block font-medium text-foreground mb-2">API 密钥</label>
						<input
							id="ai-api-key"
							type="password"
							bind:value={aiSettings.api_key}
							placeholder="sk-..."
							class="w-full px-3 py-2 bg-muted rounded-lg text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary"
						/>
						<p class="text-xs text-muted-foreground mt-1">AI 服务的 API Key。OpenAI key 以 sk- 开头，例如 sk-xxx...</p>
					</div>

					<!-- Base URL -->
					<div class="py-4 border-b border-border/50">
						<label for="ai-base-url" class="block font-medium text-foreground mb-2">API Base URL</label>
						<input
							id="ai-base-url"
							type="text"
							bind:value={aiSettings.base_url}
							placeholder="https://api.openai.com"
							class="w-full px-3 py-2 bg-muted rounded-lg text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary"
						/>
						<p class="text-xs text-muted-foreground mt-1">与 OpenAI 兼容的 API Base URL，例如 https://api.openai.com</p>
					</div>

					{#if modelsError}
						<div class="mt-4 p-3 bg-destructive/10 text-destructive rounded-lg text-sm">
							{modelsError}
						</div>
					{/if}

					<!-- 聊天模型 -->
					<div class="py-4 border-b border-border/50">
						<label for="ai-chat-model" class="block font-medium text-foreground mb-2">聊天模型</label>
						<div class="flex items-center gap-2">
							<div class="relative flex-1">
								<select
									id="ai-chat-model"
									bind:value={aiSettings.chat_model}
									class="w-full pl-3 pr-9 py-2 bg-muted rounded-lg text-sm text-foreground appearance-none focus:outline-none focus:ring-2 focus:ring-primary"
								>
									<option value="">选择一个模型</option>
									{#each chatModels as model}
										<option value={model.id}>{model.id}</option>
									{/each}
								</select>
								<svg class="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
								</svg>
							</div>
							<button
								on:click={handleFetchModels}
								disabled={fetchingModels}
								class="p-2 bg-muted hover:bg-muted/80 rounded-lg transition-colors duration-200 disabled:opacity-50"
								title="刷新模型列表"
							>
								<svg class="w-5 h-5 {fetchingModels ? 'animate-spin' : ''}" fill="none" stroke="currentColor" viewBox="0 0 24 24">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
								</svg>
							</button>
						</div>
						<p class="text-xs text-muted-foreground mt-1">AI 对话使用的模型，例如 gpt-4o、deepseek-chat</p>
					</div>

					<!-- 嵌入模型 -->
					<div class="py-4 border-b border-border/50">
						<label for="ai-embedding-model" class="block font-medium text-foreground mb-2">嵌入模型</label>
						<div class="flex items-center gap-2">
							<div class="relative flex-1">
								<select
									id="ai-embedding-model"
									bind:value={aiSettings.embedding_model}
									class="w-full pl-3 pr-9 py-2 bg-muted rounded-lg text-sm text-foreground appearance-none focus:outline-none focus:ring-2 focus:ring-primary"
								>
									<option value="">选择一个模型</option>
									{#each embeddingModels as model}
										<option value={model.id}>{model.id}</option>
									{/each}
								</select>
								<svg class="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
								</svg>
							</div>
							<button
								on:click={handleFetchModels}
								disabled={fetchingModels}
								class="p-2 bg-muted hover:bg-muted/80 rounded-lg transition-colors duration-200 disabled:opacity-50"
								title="刷新模型列表"
							>
								<svg class="w-5 h-5 {fetchingModels ? 'animate-spin' : ''}" fill="none" stroke="currentColor" viewBox="0 0 24 24">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
								</svg>
							</button>
						</div>
						<p class="text-xs text-muted-foreground mt-1">文本向量化使用的模型，例如 text-embedding-3-small</p>
					</div>

					<!-- Enable AI Toggle -->
					<div class="py-4 border-b border-border/50">
						<div class="flex items-center justify-between gap-4">
							<div class="min-w-0 flex-1">
								<div class="font-medium text-foreground">启用 AI 功能</div>
								<div class="text-sm text-muted-foreground">
									{#if !canEnableAI}
										请先填写以上所有字段以启用
									{:else if aiSettings.enabled}
										AI 助手已激活。保存日记条目时将自动构建向量数据。
									{:else}
										启用后可使用 AI 助手。保存日记条目时将在后台自动构建向量数据。
									{/if}
								</div>
							</div>
							<button
								on:click={() => { if (canEnableAI) aiSettings.enabled = !aiSettings.enabled; }}
								disabled={!canEnableAI && !aiSettings.enabled}
								aria-label="切换 AI 功能"
								class="relative inline-flex h-6 w-11 flex-shrink-0 items-center rounded-full transition-colors duration-200 focus:outline-none focus:ring-2 focus:ring-primary focus:ring-offset-2 {aiSettings.enabled ? 'bg-primary' : 'bg-muted'} {!canEnableAI && !aiSettings.enabled ? 'opacity-50 cursor-not-allowed' : ''}"
							>
								<span
									class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform duration-200 {aiSettings.enabled ? 'translate-x-6' : 'translate-x-1'}"
								></span>
							</button>
						</div>
					</div>

					<!-- Build Vectors -->
					{#if aiSettings.enabled}
						<div class="py-4 border-b border-border/50">
							<div class="flex items-center justify-between">
								<div>
									<div class="font-medium text-foreground">构建向量索引</div>
									<div class="text-sm text-muted-foreground">
										为日记条目生成嵌入向量
									</div>
								</div>
								<div class="flex items-center gap-2">
									<button
										on:click={() => handleBuildVectors(true)}
										disabled={buildingVectors}
										class="px-3 py-1.5 text-sm bg-primary text-primary-foreground hover:bg-primary/90 rounded-lg transition-colors duration-200 disabled:opacity-50 flex items-center gap-1.5"
										title="仅构建新增和过时的条目"
									>
										{#if buildingVectors}
											<svg class="w-3.5 h-3.5 animate-spin" fill="none" viewBox="0 0 24 24">
												<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
												<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
											</svg>
										{:else}
											<svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
												<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
											</svg>
										{/if}
										更新
									</button>
									<button
										on:click={() => handleBuildVectors(false)}
										disabled={buildingVectors}
										class="px-3 py-1.5 text-sm bg-muted hover:bg-muted/80 rounded-lg transition-colors duration-200 disabled:opacity-50 flex items-center gap-1.5"
										title="从头重建所有条目"
									>
										<svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
											<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
										</svg>
										全部重建
									</button>
								</div>
							</div>

							{#if buildError}
								<div class="mt-3 p-3 bg-destructive/10 text-destructive rounded-lg text-sm">
									{buildError}
								</div>
							{/if}

							{#if buildResult}
								<div class="mt-3 p-3 bg-muted rounded-lg text-sm">
									<div class="font-medium text-foreground mb-2">构建结果</div>
									<div class="space-y-1 text-muted-foreground">
										<div>日记总数：{buildResult.total}</div>
										<div class="text-green-600">成功：{buildResult.success}</div>
										{#if buildResult.failed > 0}
											<div class="text-destructive">失败：{buildResult.failed}</div>
										{/if}
									</div>
									{#if buildResult.error_details && buildResult.error_details.length > 0}
										<div class="mt-2 pt-2 border-t border-border/50">
											<div class="font-medium text-destructive mb-1">错误：</div>
											<div class="text-xs text-muted-foreground space-y-1 max-h-32 overflow-y-auto">
												{#each buildResult.error_details as error}
													<div>{error}</div>
												{/each}
											</div>
										</div>
									{/if}
								</div>
							{/if}
						</div>

						<!-- 向量索引状态 -->
						<div class="py-4 border-b border-border/50">
							<div class="font-medium text-foreground mb-2">向量索引状态</div>
							{#if loadingStats}
								<div class="flex items-center gap-2 text-sm text-muted-foreground">
									<svg class="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
										<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
										<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
									</svg>
									加载中...
								</div>
							{:else if vectorStats}
								<div class="space-y-3">
									<!-- Segmented Progress Bar -->
									<div class="space-y-2">
										<div class="flex items-center justify-between text-sm">
											<span class="text-muted-foreground">日记总数：</span>
											<span class="font-medium text-foreground">{vectorStats.diary_count}</span>
										</div>
										<div class="w-full bg-muted rounded-full h-2 flex overflow-hidden">
											{#if vectorStats.diary_count > 0}
												{#if vectorStats.indexed_count > 0}
													<div
														class="h-2 bg-green-500 transition-all duration-300"
														style="width: {(vectorStats.indexed_count / vectorStats.diary_count * 100)}%"
													></div>
												{/if}
												{#if vectorStats.outdated_count > 0}
													<div
														class="h-2 bg-amber-500 transition-all duration-300"
														style="width: {(vectorStats.outdated_count / vectorStats.diary_count * 100)}%"
													></div>
												{/if}
												{#if vectorStats.pending_count > 0}
													<div
														class="h-2 bg-gray-400 transition-all duration-300"
														style="width: {(vectorStats.pending_count / vectorStats.diary_count * 100)}%"
													></div>
												{/if}
											{/if}
										</div>
									</div>

									<!-- Stats Legend -->
									<div class="flex flex-wrap gap-4 text-xs">
										<div class="flex items-center gap-1.5">
											<div class="w-2.5 h-2.5 rounded-full bg-green-500"></div>
											<span class="text-muted-foreground">已索引： <span class="font-medium text-foreground">{vectorStats.indexed_count}</span></span>
										</div>
										<div class="flex items-center gap-1.5">
											<div class="w-2.5 h-2.5 rounded-full bg-amber-500"></div>
											<span class="text-muted-foreground">过时： <span class="font-medium text-foreground">{vectorStats.outdated_count}</span></span>
										</div>
										<div class="flex items-center gap-1.5">
											<div class="w-2.5 h-2.5 rounded-full bg-gray-400"></div>
											<span class="text-muted-foreground">待处理： <span class="font-medium text-foreground">{vectorStats.pending_count}</span></span>
										</div>
									</div>

									<!-- Status Message -->
									{#if vectorStats.indexed_count === vectorStats.diary_count && vectorStats.diary_count > 0}
										<div class="text-xs text-green-600 flex items-center gap-1">
											<svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
												<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
											</svg>
											所有日记均已索引且为最新
										</div>
									{:else if vectorStats.outdated_count > 0 || vectorStats.pending_count > 0}
										<div class="text-xs text-amber-600 flex items-center gap-1">
											<svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
												<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
											</svg>
											{vectorStats.outdated_count + vectorStats.pending_count} 篇日记需要建立索引
										</div>
									{:else if vectorStats.diary_count === 0}
										<div class="text-xs text-muted-foreground">
											暂无日记可索引
										</div>
									{/if}
								</div>
							{:else}
								<div class="text-sm text-muted-foreground">
									暂无索引数据
								</div>
							{/if}
						</div>
					{/if}

					<!-- 周/月分析提示词 -->
					<div class="py-4 border-b border-border/50">
						<div class="font-medium text-foreground mb-3">周/月分析提示词</div>
						<div class="mb-4">
							<div class="flex items-center justify-between mb-1.5">
								<label for="analysis-system-prompt" class="text-sm text-muted-foreground">系统提示词 (System Prompt)</label>
								<button
									type="button"
									on:click={() => { aiSettings.analysis_system_prompt = '你是一个贴心的日记分析助手，基于用户提供的日记内容进行深入分析。你需要：\n1) 归纳总结日记的主要内容；\n2) 分析用户的情绪变化、生活模式；\n3) 找出亮点和值得改进的地方；\n4) 给出具体、可操作的建议。\n请用温暖、鼓励且理性的语气，分段输出，便于阅读。使用中文回答。'; }}
									class="text-xs text-muted-foreground hover:text-primary transition-colors px-2 py-0.5 rounded hover:bg-primary/10"
								>
									恢复默认
								</button>
							</div>
							<textarea
								id="analysis-system-prompt"
								rows={6}
								bind:value={aiSettings.analysis_system_prompt}
								placeholder="你是一个贴心的日记分析助手……使用中文回答。"
								class="w-full px-3 py-2 bg-muted/50 border border-border/70 rounded-lg text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary resize-y font-mono leading-relaxed"
							/>
							<p class="text-xs text-muted-foreground mt-1">用于 AI 分析日记的系统角色与行为指令；留空时使用系统默认提示词。</p>
						</div>
						<div>
							<label for="analysis-user-prefix" class="text-sm text-muted-foreground block mb-1.5">内容引导语 (User Prefix，可选)</label>
							<textarea
								id="analysis-user-prefix"
								rows={3}
								bind:value={aiSettings.analysis_user_prefix}
								placeholder="以下是本周（起始日期 ~ 结束日期）的日记……"
								class="w-full px-3 py-2 bg-muted/50 border border-border/70 rounded-lg text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary resize-y font-mono leading-relaxed"
							/>
							<p class="text-xs text-muted-foreground mt-1">
								出现在每篇日记内容之前的引导语；留空时使用默认的"周/月"格式化提示。可在其中加入自己的强调重点。
							</p>
						</div>
					</div>

					<!-- Save Button -->
					<div class="pt-4 flex items-center gap-3">
						<button
							on:click={handleSaveAISettings}
							disabled={aiSaving || !aiSettingsChanged}
							class="px-4 py-2 bg-primary text-primary-foreground rounded-lg hover:bg-primary/90 transition-colors duration-200 disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
						>
							{#if aiSaving}
								<svg class="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
									<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
									<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
								</svg>
								保存中...
							{:else}
								保存 AI 设置
							{/if}
						</button>
						{#if aiSuccess}
							<span class="text-sm text-green-600 flex items-center gap-1 animate-fade-in">
								<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
								</svg>
								已保存
							</span>
						{/if}
					</div>
				</div>
				{/if}

				{#if activeTab === 'image-upload'}
				<!-- 图片上传 Section -->
				<div id="image-upload" class="bg-card rounded-xl shadow-sm border border-border/50 p-6 animate-fade-in scroll-mt-16">
					<h2 class="text-lg font-semibold text-foreground mb-4">图片上传</h2>
					<p class="text-sm text-muted-foreground mb-6">
						选择日记图片的存储位置。切换提供方时，现有的本地、S3 和 Chevereto 设置将保留，以便迁移后仍可访问旧的媒体文件。
					</p>

					{#if imageUploadError}
						<div class="mb-4 p-3 bg-destructive/10 text-destructive rounded-lg text-sm">
							{imageUploadError}
						</div>
					{/if}

					{#if imageUploadSuccess}
						<div class="mb-4 p-3 bg-green-500/10 text-green-600 rounded-lg text-sm">
							{imageUploadSuccess}
						</div>
					{/if}

					<div class="py-4 border-b border-border/50">
						<div class="font-medium text-foreground mb-3">存储提供商</div>
						<div class="grid gap-3 md:grid-cols-3">
							{#each [
								{ id: 'local', label: '本地存储', description: '将图片保存在磁盘上，并在内置媒体库中管理。' },
								{ id: 's3', label: 'S3 对象存储', description: '将媒体对象存储在 S3 兼容的对象存储服务中。' },
								{ id: 'chevereto', label: 'Chevereto', description: '上传图片到 Chevereto，并插入外部 URL。' }
							] as option}
								<button
									type="button"
									on:click={() => imageUploadSettingsLocal.provider = option.id as ImageUploadProvider}
									class="text-left rounded-xl border p-4 transition-colors duration-200 {imageUploadSettingsLocal.provider === option.id ? 'border-primary bg-primary/5' : 'border-border/50 hover:border-border'}"
								>
									<div class="font-medium text-foreground">{option.label}</div>
									<div class="text-sm text-muted-foreground mt-1">{option.description}</div>
								</button>
							{/each}
						</div>
					</div>

					{#if imageUploadSettingsLocal.provider === 'local'}
						<div class="py-4 border-b border-border/50 space-y-4">
							<div>
								<label for="local-media-path" class="block font-medium text-foreground mb-2">本地存储路径</label>
								<input
									id="local-media-path"
									type="text"
									bind:value={imageUploadSettingsLocal.local.path}
									placeholder="./diarum_data/storage/media"
									class="w-full px-3 py-2 bg-muted rounded-lg text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary"
								/>
								<p class="text-xs text-muted-foreground mt-1">默认迁移路径指向现有的吾身媒体存储目录。</p>
							</div>
						</div>
					{:else if imageUploadSettingsLocal.provider === 's3'}
						<div class="py-4 border-b border-border/50 space-y-4">
							<div class="grid gap-4 md:grid-cols-2">
								<div>
									<label for="s3-bucket" class="block font-medium text-foreground mb-2">Bucket 名称</label>
									<input id="s3-bucket" type="text" bind:value={imageUploadSettingsLocal.s3.bucket} class="w-full px-3 py-2 bg-muted rounded-lg text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-primary" />
								</div>
								<div>
									<label for="s3-region" class="block font-medium text-foreground mb-2">区域</label>
									<input id="s3-region" type="text" bind:value={imageUploadSettingsLocal.s3.region} class="w-full px-3 py-2 bg-muted rounded-lg text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-primary" />
								</div>
								<div>
									<label for="s3-endpoint" class="block font-medium text-foreground mb-2">Endpoint（可选）</label>
									<input id="s3-endpoint" type="text" bind:value={imageUploadSettingsLocal.s3.endpoint} placeholder="https://s3.amazonaws.com" class="w-full px-3 py-2 bg-muted rounded-lg text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-primary" />
								</div>
								<div class="flex items-end">
									<label class="inline-flex items-center gap-2 text-sm text-foreground">
										<input type="checkbox" bind:checked={imageUploadSettingsLocal.s3.force_path_style} class="rounded border-border text-primary focus:ring-primary" />
										使用路径样式请求
									</label>
								</div>
							</div>
							<div class="grid gap-4 md:grid-cols-2">
								<div>
									<label for="s3-access-key" class="block font-medium text-foreground mb-2">访问密钥</label>
									<input id="s3-access-key" type="text" bind:value={imageUploadSettingsLocal.s3.access_key} class="w-full px-3 py-2 bg-muted rounded-lg text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-primary" />
								</div>
								<div>
									<label for="s3-secret" class="block font-medium text-foreground mb-2">秘密访问密钥</label>
									<input id="s3-secret" type="password" bind:value={imageUploadSettingsLocal.s3.secret} class="w-full px-3 py-2 bg-muted rounded-lg text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-primary" />
								</div>
							</div>
							<p class="text-xs text-muted-foreground">如果您是从 PocketBase S3 存储迁移而来，这些凭据也将用于保持旧相册图片的可访问性。</p>
						</div>
					{:else}
						<div class="py-4 border-b border-border/50 space-y-4">
							<div>
								<label for="chevereto-domain" class="block font-medium text-foreground mb-2">域名</label>
								<input
									id="chevereto-domain"
									type="text"
									bind:value={imageUploadSettingsLocal.chevereto.domain}
									placeholder="https://img.example.com"
									class="w-full px-3 py-2 bg-muted rounded-lg text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary"
								/>
							</div>
							<div>
								<label for="chevereto-api-key" class="block font-medium text-foreground mb-2">API 密钥</label>
								<input
									id="chevereto-api-key"
									type="password"
									bind:value={imageUploadSettingsLocal.chevereto.api_key}
									placeholder="chv-key-..."
									class="w-full px-3 py-2 bg-muted rounded-lg text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary"
								/>
							</div>
							<div>
								<label for="chevereto-album-id" class="block font-medium text-foreground mb-2">相册 ID（可选）</label>
								<input
									id="chevereto-album-id"
									type="text"
									bind:value={imageUploadSettingsLocal.chevereto.album_id}
									class="w-full px-3 py-2 bg-muted rounded-lg text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary"
								/>
							</div>
							<div class="flex items-center justify-between gap-4 rounded-lg bg-muted/40 p-4">
								<div>
									<div class="font-medium text-foreground">测试连接</div>
									<div class="text-sm text-muted-foreground">保存前请确认您的 Chevereto 服务器可以访问。</div>
								</div>
								<button
									on:click={handleTestChevereto}
									disabled={cheveretoTesting || !canTestChevereto}
									class="px-4 py-2 text-sm bg-background hover:bg-background/80 rounded-lg transition-colors duration-200 disabled:opacity-50 flex items-center gap-2"
								>
									{#if cheveretoTesting}
										<svg class="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
											<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
											<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
										</svg>
										测试中...
									{:else}
										测试
									{/if}
								</button>
							</div>
							{#if cheveretoTestResult}
								<div class="p-3 rounded-lg text-sm {cheveretoTestResult.success ? 'bg-green-500/10 text-green-600' : 'bg-destructive/10 text-destructive'}">
									{cheveretoTestResult.message}
								</div>
							{/if}
							<p class="text-xs text-muted-foreground">Chevereto 上传会将外部图片 URL 插入日记内容。这些图片不会被内置媒体库追踪，也不会包含在导出文件中。</p>
						</div>
					{/if}

					<!-- Save Button -->
					<div class="pt-4 flex items-center gap-3">
						<button
							on:click={handleSaveImageUploadSettings}
							disabled={imageUploadSaving || !imageUploadSettingsChanged}
							class="px-4 py-2 bg-primary text-primary-foreground rounded-lg hover:bg-primary/90 transition-colors duration-200 disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
						>
							{#if imageUploadSaving}
								<svg class="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
									<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
									<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
								</svg>
								保存中...
							{:else}
								保存图片上传设置
							{/if}
						</button>
						{#if imageUploadSuccess}
							<span class="text-sm text-green-600 flex items-center gap-1 animate-fade-in">
								<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
								</svg>
								已保存
							</span>
						{/if}
					</div>
				</div>
				{/if}

				{#if activeTab === 'data-management'}
				<!-- 数据管理 Section -->
				<div id="data-management" class="bg-card rounded-xl shadow-sm border border-border/50 p-6 animate-fade-in scroll-mt-16">
					<h2 class="text-lg font-semibold text-foreground mb-4">数据管理</h2>
					<p class="text-sm text-muted-foreground mb-6">
						导入和导出您的日记数据。为避免导出文件过大，您可以按日期范围分段导出。
					</p>

					<!-- 导出 -->
					<div class="py-4 border-b border-border/50">
						<div class="flex items-center justify-between mb-1">
							<div class="font-medium text-foreground">导出</div>
							<button
								on:click={() => showExportOptions = !showExportOptions}
								class="text-xs text-primary hover:underline"
							>
								{showExportOptions ? '隐藏选项' : '显示选项'}
							</button>
						</div>
						<div class="text-sm text-muted-foreground mb-3">将您的日记数据下载为 ZIP 文件</div>

						{#if showExportOptions}
							<div class="mb-4 p-4 bg-muted/50 rounded-lg space-y-4">
								<div class="text-xs text-amber-600 bg-amber-500/10 p-2 rounded">
									为避免导出文件过大，请考虑选择特定日期范围分段导出。
								</div>

								<!-- 日期范围 -->
								<div>
									<label for="export-date-range" class="block text-sm font-medium text-foreground mb-2">日期范围</label>
									<div class="relative">
										<select
											id="export-date-range"
											bind:value={exportOptions.date_range}
											class="w-full pl-3 pr-9 py-2 bg-background rounded-lg text-sm text-foreground appearance-none focus:outline-none focus:ring-2 focus:ring-primary border border-border/50"
										>
											<option value="1m">过去 1 个月</option>
											<option value="3m">过去 3 个月</option>
											<option value="6m">过去 6 个月</option>
											<option value="1y">过去 1 年</option>
											<option value="all">全部时间</option>
											<option value="custom">自定义范围</option>
										</select>
										<svg class="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
											<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
										</svg>
									</div>
								</div>

								{#if exportOptions.date_range === 'custom'}
									<div class="grid grid-cols-2 gap-3">
										<div>
											<label for="export-start-date" class="block text-xs text-muted-foreground mb-1">开始日期</label>
											<input
												id="export-start-date"
												type="date"
												bind:value={customStartDate}
												class="w-full px-3 py-2 bg-background rounded-lg text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-primary border border-border/50"
											/>
										</div>
										<div>
											<label for="export-end-date" class="block text-xs text-muted-foreground mb-1">结束日期</label>
											<input
												id="export-end-date"
												type="date"
												bind:value={customEndDate}
												class="w-full px-3 py-2 bg-background rounded-lg text-sm text-foreground focus:outline-none focus:ring-2 focus:ring-primary border border-border/50"
											/>
										</div>
									</div>
								{/if}

								<!-- 要导出的内容 -->
								<div>
									<div class="block text-sm font-medium text-foreground mb-2">要导出的内容</div>
									<div class="space-y-2">
										<label class="flex items-center gap-2 cursor-pointer">
											<input type="checkbox" bind:checked={exportOptions.include_diaries} class="rounded" />
											<span class="text-sm text-foreground">日记</span>
										</label>
										<label class="flex items-center gap-2 cursor-pointer">
											<input type="checkbox" bind:checked={exportOptions.include_media} class="rounded" />
											<span class="text-sm text-foreground">媒体文件</span>
										</label>
										<label class="flex items-center gap-2 cursor-pointer">
											<input type="checkbox" bind:checked={exportOptions.include_conversations} class="rounded" />
											<span class="text-sm text-foreground">AI 对话</span>
										</label>
									</div>
								</div>
							</div>
						{/if}

						{#if exportError}
							<div class="mb-3 p-3 bg-destructive/10 text-destructive rounded-lg text-sm">
								{exportError}
							</div>
						{/if}

						<button
							on:click={handleExport}
							disabled={exporting}
							class="px-4 py-2 bg-primary text-primary-foreground rounded-lg hover:bg-primary/90 transition-colors duration-200 disabled:opacity-50 flex items-center gap-2"
						>
							{#if exporting}
								<svg class="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
									<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
									<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
								</svg>
								导出中...
							{:else}
								<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
								</svg>
								导出数据
							{/if}
						</button>

						{#if exportStats}
							<div class="mt-3 p-3 bg-muted rounded-lg text-sm">
								<div class="font-medium text-foreground mb-2">导出完成</div>
								<div class="text-xs text-muted-foreground mb-2">
									时段： {exportStats.start_date} 至 {exportStats.end_date}
								</div>
								<div class="space-y-2 text-muted-foreground">
									<div class="flex justify-between">
										<span>日记：</span>
										<span>
											<span class="text-foreground font-medium">{exportStats.diaries.actual_exported}</span>
											<span class="text-xs">/ {exportStats.diaries.should_export} 已选择 / {exportStats.diaries.total_in_system} 总计</span>
										</span>
									</div>
									<div class="flex justify-between">
										<span>媒体：</span>
										<span>
											<span class="text-foreground font-medium">{exportStats.media.actual_exported}</span>
											<span class="text-xs">/ {exportStats.media.should_export} 已选择 / {exportStats.media.total_in_system} 总计</span>
										</span>
									</div>
									<div class="flex justify-between">
										<span>对话：</span>
										<span>
											<span class="text-foreground font-medium">{exportStats.conversations.actual_exported}</span>
											<span class="text-xs">/ {exportStats.conversations.should_export} 已选择 / {exportStats.conversations.total_in_system} 总计</span>
											<span class="text-xs">（{exportStats.messages} 条消息）</span>
										</span>
									</div>
								</div>
								{#if exportStats.failed_items && exportStats.failed_items.length > 0}
									<div class="mt-3 pt-2 border-t border-border/50">
										<div class="font-medium text-destructive mb-1">失败项目：</div>
										<div class="text-xs space-y-1 max-h-24 overflow-y-auto">
											{#each exportStats.failed_items as item}
												<div class="text-muted-foreground">
													<span class="text-destructive">[{item.type}]</span> {item.id}: {item.reason}
												</div>
											{/each}
										</div>
									</div>
								{/if}
							</div>
						{/if}
					</div>

					<!-- 导入 -->
					<div class="py-4">
						<div class="font-medium text-foreground mb-1">导入</div>
						<div class="text-sm text-muted-foreground mb-3">从之前导出的 ZIP 文件中恢复日记数据。日期已存在的日记将被跳过。</div>

						{#if importError}
							<div class="mb-3 p-3 bg-destructive/10 text-destructive rounded-lg text-sm">
								{importError}
							</div>
						{/if}

						<div class="flex items-center gap-3 flex-wrap">
							<label class="px-4 py-2 text-sm bg-muted hover:bg-muted/80 rounded-lg transition-colors duration-200 cursor-pointer">
								<span>{importFile ? importFile.name : '选择文件'}</span>
								<input
									type="file"
									accept=".zip"
									class="hidden"
									on:change={handleImportFileChange}
								/>
							</label>
							<button
								on:click={handleImport}
								disabled={importing || !importFile}
								class="px-4 py-2 bg-primary text-primary-foreground rounded-lg hover:bg-primary/90 transition-colors duration-200 disabled:opacity-50 flex items-center gap-2"
							>
								{#if importing}
									<svg class="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
										<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
										<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
									</svg>
									导入中...
								{:else}
									<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
										<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0l-4 4m4-4v12" />
									</svg>
									导入
								{/if}
							</button>
						</div>

						{#if importStats}
							<div class="mt-3 p-3 bg-muted rounded-lg text-sm">
								<div class="font-medium text-foreground mb-2">导入完成</div>
								<div class="space-y-1 text-muted-foreground">
									<div>
										日记：
										<span class="text-green-600 font-medium">{importStats.diaries.imported} 已导入</span>
										{#if importStats.diaries.skipped > 0}
											, <span class="text-amber-600 font-medium">{importStats.diaries.skipped} 已跳过</span>
										{/if}
										{#if importStats.diaries.failed > 0}
											, <span class="text-destructive font-medium">{importStats.diaries.failed} 失败</span>
										{/if}
										<span class="text-muted-foreground">（共 {importStats.diaries.total} 条）</span>
									</div>
									<div>
										媒体：
										<span class="text-green-600 font-medium">{importStats.media.imported} 已导入</span>
										{#if importStats.media.skipped > 0}
											, <span class="text-amber-600 font-medium">{importStats.media.skipped} 已跳过</span>
										{/if}
										{#if importStats.media.failed > 0}
											, <span class="text-destructive font-medium">{importStats.media.failed} 失败</span>
										{/if}
										<span class="text-muted-foreground">（共 {importStats.media.total} 个）</span>
									</div>
									<div>
										AI 对话：
										<span class="text-green-600 font-medium">{importStats.conversations.imported} 已导入</span>
										{#if importStats.conversations.skipped > 0}
											, <span class="text-orange-500 font-medium">{importStats.conversations.skipped} 已跳过</span>
										{/if}
										{#if importStats.conversations.failed > 0}
											, <span class="text-destructive font-medium">{importStats.conversations.failed} 失败</span>
										{/if}
										<span class="text-muted-foreground">（共 {importStats.conversations.total} 条）</span>
									</div>
								</div>
							</div>
						{/if}
					</div>
				</div>
				{/if}
			</div>
		{/if}
	</main>
		</div>
	</div>

	<Footer maxWidth="6xl" tagline="Manage your settings" />
</div>

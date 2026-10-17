<script lang="ts">
  import { resolve } from '$app/paths';
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { isAuthenticated } from '#lib/api/client.js';
  import { theme, setTheme, type Theme } from '#lib/stores/theme.js';
  import {
    getApiToken,
    toggleApiToken,
    resetApiToken,
    getMemosSettings,
    saveMemosSettings,
    resetMemosWebhookToken,
    getGeneralSettings,
    saveGeneralSettings,
    type MemosSettings,
    type ApiTokenStatus
  } from '#lib/api/settings.js';
  import {
    getAISettings,
    saveAISettings,
    fetchModels,
    buildVectors,
    buildVectorsIncremental,
    getVectorStats,
    DEFAULT_ANALYSIS_SYSTEM_PROMPT,
    type AISettings,
    type ModelInfo,
    type BuildVectorsResult,
    type VectorStats
  } from '#lib/api/ai.js';
  import {
    exportDiaries,
    importDiaries,
    resolveConflict,
    resolveConflictBatch,
    type ExportStats,
    type ImportStats,
    type ImportDiaryDetail,
    type ExportOptions
  } from '#lib/api/exportImport.js';
  import {
    defaultImageUploadSettings,
    getImageUploadSettings,
    saveImageUploadSettings,
    testCheveretoConnection,
    type ImageUploadProvider,
    type ImageUploadSettings
  } from '#lib/api/imageUpload.js';
  import { loadImageUploadSettings } from '#lib/stores/imageUpload.js';
  import { pb } from '#lib/api/client.js';
  import { reminderSettings } from '#lib/stores/notifications.js';
  import {
    enableNotifications,
    disableNotifications,
    saveReminderSettings,
    sendTest,
    notificationPermission
  } from '#lib/utils/notifications.js';
  import PageHeader from '#lib/components/ui/PageHeader.svelte';
  import MoodIcon from '#lib/components/ui/MoodIcon.svelte';

  type SettingsTab =
    | 'general'
    | 'notifications'
    | 'api-access'
    | 'weather'
    | 'ai-assistant'
    | 'ai-vectors'
    | 'ai-speech'
    | 'image-upload'
    | 'memos-sync'
    | 'data-management'
    | 'backup';

  // 基础 tab 列表（始终显示）
  const baseSettingsTabs: { id: SettingsTab; label: string }[] = [
    { id: 'general', label: '通用' },
    { id: 'notifications', label: '通知提醒' },
    { id: 'ai-assistant', label: 'AI 助手' },
    { id: 'weather', label: '天气设置' },
    { id: 'backup', label: '自动备份' },
    { id: 'api-access', label: 'API 访问' },
    { id: 'memos-sync', label: 'Memos 同步' },
    { id: 'image-upload', label: '图片上传' },
    { id: 'data-management', label: '数据管理' }
  ];

  // AI 启用后额外显示的 tab
  const aiExtraTabs: { id: SettingsTab; label: string }[] = [
    { id: 'ai-speech', label: '语音输入' },
    { id: 'ai-vectors', label: '向量索引' }
  ];

  $: settingsTabs = aiSettings.enabled
    ? [...baseSettingsTabs.slice(0, 3), ...aiExtraTabs, ...baseSettingsTabs.slice(3)]
    : baseSettingsTabs;

  let activeTab: SettingsTab = 'general';

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
    // Kit 3 起，原生 history.replaceState 会把 Kit 写入的 history 元数据一并覆盖掉，
    // 使前进/后退时 Kit 的内部索引跟踪错位（官方已弃用该写法）。
    // 改用浅路由 goto：只改可见 URL、不重新渲染，history 元数据由 Kit 维护。
    // 目标是当前页自身（只切换 hash），url.pathname 来自 window.location，
    // 已是 Kit 解析出的站内绝对路径，无需再 resolve。
    // eslint-disable-next-line svelte/no-navigation-without-resolve
    goto(`${url.pathname}${url.search}#${tab}`, {
      replace: true,
      shallow: true,
      state: {}
    }).catch(() => {
      // URL 必然属于本应用，解析失败仅理论可能；失败时保持 URL 不变即可。
    });
  }

  let loading = true;
  let tokenStatus: ApiTokenStatus = { exists: false, enabled: false, token: '' };
  let copied = false;
  let resetting = false;
  let toggling = false;

  // General settings
  let defaultView: 'diary' | 'calendar' = 'diary';
  let originalDefaultView: 'diary' | 'calendar' = 'diary';
  let versionRetentionDays = 30;
  let originalVersionRetentionDays = 30;
  let fontSize: 'small' | 'medium' | 'large' = 'medium';
  let originalFontSize: 'small' | 'medium' | 'large' = 'medium';
  let currentTheme: Theme = 'system';
  let originalTheme: Theme = 'system';
  let savingGeneral = false;
  let generalError = '';
  let generalSuccess = '';
  $: generalSettingsChanged =
    defaultView !== originalDefaultView ||
    fontSize !== originalFontSize ||
    currentTheme !== originalTheme ||
    versionRetentionDays !== originalVersionRetentionDays;

  // Memos sync settings
  let memosSettings: MemosSettings = {
    enabled: false,
    base_url: '',
    webhook_url: '',
    token_exists: false
  };
  let originalMemosSettings: MemosSettings = { ...memosSettings };
  let memosSaving = false;
  let memosResetting = false;
  let memosCopied = false;
  let memosError = '';
  let memosSuccess = '';

  // Weather settings
  let weatherEnabled = false;
  let weatherDefaultCity = '';
  let weatherAutoFetch = false;
  let weatherAutoFetchTime = '20:00';
  let qweatherEnabled = false;
  let weatherSaving = false;
  let weatherError = '';
  let weatherSuccess = '';
  let backfillWeather = false;
  let backfillResult: {
    updated: number;
    skipped: number;
    failed: number;
    skippedReasons?: Record<string, number>;
  } | null = null;
  let backfillError = '';
  let backfillStartDate = `${new Date().getFullYear()}-01-01`;
  let backfillMode: 'content' | 'range' = 'content';
  let backfillProgress = { current: 0, total: 0, status: '', date: '' };
  let skippedReasons: Record<string, number> = {};

  // AI Settings
  let aiSettings: AISettings = {
    api_key: '',
    base_url: '',
    chat_model: '',
    embedding_model: '',
    analysis_system_prompt: '',
    analysis_user_prefix: '',
    enabled: false,
    speech_provider: 'none',
    speech_base_url: '',
    speech_api_key: '',
    speech_model: 'whisper-1',
    speech_language: 'zh'
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
  let originalImageUploadSettings: ImageUploadSettings = structuredClone(
    defaultImageUploadSettings
  );
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
  let resolvingConflict = false;
  let resolvingBatch = false;
  let expandedConflictDate: string | null = null;
  let conflictViewMode: 'diff' | 'side' = 'diff';
  let selectedConflicts: Set<string> = new Set();
  let isDragOver = false;

  // Backup settings
  let backupSettingsLocal = {
    enabled: false,
    frequency: 'daily',
    time: '00:00',
    day_of_week: 1,
    day_of_month: 1,
    retention_days: 90,
    upload_s3: false
  };
  let backupSettingsLoading = true;
  let backupTriggering = false;
  let backupError = '';
  let backupSuccess = '';
  let backupRecords: Backup[] = [];
  let backupCurrentPage = 1;
  let backupTotalPages = 0;

  // Export options
  const EXPORT_OPTIONS_KEY = 'diarum_export_options';
  const savedExportOptions = (() => {
    try {
      const raw = localStorage.getItem(EXPORT_OPTIONS_KEY);
      return raw ? JSON.parse(raw) : null;
    } catch {
      return null;
    }
  })();
  let exportOptions: ExportOptions = savedExportOptions || {
    date_range: '3m',
    include_diaries: true,
    include_media: true,
    include_conversations: true,
    include_analysis: false
  };
  let customStartDate = '';
  let customEndDate = '';
  let showExportOptions = true;

  function persistExportOptions() {
    try {
      localStorage.setItem(EXPORT_OPTIONS_KEY, JSON.stringify(exportOptions));
    } catch {
      // 隐私模式下 localStorage 写入会抛配额错误，选项持久化失败不影响导出本身
    }
  }

  async function loadTokenStatus() {
    tokenStatus = await getApiToken();
  }

  async function loadGeneralSettings() {
    try {
      const settings = await getGeneralSettings();
      defaultView = settings.default_view;
      originalDefaultView = settings.default_view;
      versionRetentionDays = settings.version_retention_days;
      originalVersionRetentionDays = settings.version_retention_days;
    } catch (error) {
      console.error('Failed to load general settings:', error);
    }
    // Load client-side settings from localStorage
    const storedFontSize = localStorage.getItem('editor_font_size') as 'small' | 'medium' | 'large';
    if (storedFontSize && ['small', 'medium', 'large'].includes(storedFontSize)) {
      fontSize = storedFontSize;
    }
    originalFontSize = fontSize;
    applyFontSize(fontSize);

    currentTheme = $theme;
    originalTheme = $theme;
  }

  function applyFontSize(size: 'small' | 'medium' | 'large') {
    const root = document.documentElement;
    const sizeMap = { small: '14px', medium: '16px', large: '18px' };
    root.style.fontSize = sizeMap[size];
  }

  function handleFontSizeChange(size: 'small' | 'medium' | 'large') {
    fontSize = size;
    applyFontSize(size);
  }

  function handleThemeChange(t: Theme) {
    currentTheme = t;
    setTheme(t);
  }

  async function handleSaveGeneralSettings() {
    generalError = '';
    generalSuccess = '';
    savingGeneral = true;
    try {
      const retention = Math.min(3650, Math.max(1, Math.round(Number(versionRetentionDays) || 30)));
      versionRetentionDays = retention;
      await saveGeneralSettings({ default_view: defaultView, version_retention_days: retention });
      originalDefaultView = defaultView;
      originalVersionRetentionDays = retention;
      // Save client-side settings
      localStorage.setItem('editor_font_size', fontSize);
      originalFontSize = fontSize;
      originalTheme = currentTheme;
      generalSuccess = '通用设置已保存';
      setTimeout(() => {
        generalSuccess = '';
      }, 3000);
    } catch (e) {
      generalError = e instanceof Error ? e.message : '保存失败';
    } finally {
      savingGeneral = false;
    }
  }

  async function loadWeatherSettingsLocal() {
    try {
      // Load weather settings from the API
      const [settingsRes, providerRes] = await Promise.all([
        fetch('/api/v1/settings', {
          headers: { Authorization: `Bearer ${pb.authStore.token}` }
        }),
        fetch('/api/v1/weather/provider', {
          headers: { Authorization: `Bearer ${pb.authStore.token}` }
        })
      ]);
      if (settingsRes.ok) {
        const data = await settingsRes.json();
        weatherEnabled = data.settings?.['weather.enabled'] ?? false;
        weatherDefaultCity = data.settings?.['weather.default_city'] ?? '';
        weatherAutoFetch = data.settings?.['weather.auto_fetch'] ?? false;
        weatherAutoFetchTime = data.settings?.['weather.auto_fetch_time'] ?? '20:00';
      }
      if (providerRes.ok) {
        const data = await providerRes.json();
        qweatherEnabled = data.qweather_enabled ?? false;
      }
    } catch (error) {
      console.error('Failed to load weather settings:', error);
    }
  }

  async function handleSaveWeatherSettings() {
    weatherError = '';
    weatherSuccess = '';

    weatherSaving = true;
    try {
      const response = await fetch('/api/v1/settings/batch', {
        method: 'PUT',
        headers: {
          Authorization: `Bearer ${pb.authStore.token}`,
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          settings: {
            'weather.enabled': weatherEnabled,
            'weather.default_city': weatherDefaultCity,
            'weather.auto_fetch': weatherAutoFetch,
            'weather.auto_fetch_time': weatherAutoFetchTime
          }
        })
      });

      if (!response.ok) {
        throw new Error('Failed to save weather settings');
      }

      weatherSuccess = '天气设置已成功保存';
      setTimeout(() => (weatherSuccess = ''), 3000);
    } catch (e) {
      weatherError = e instanceof Error ? e.message : '保存天气设置失败';
    }
    weatherSaving = false;
  }

  async function handleBackfillWeather() {
    backfillError = '';
    backfillResult = null;
    backfillProgress = { current: 0, total: 0, status: '', date: '' };
    skippedReasons = {};

    // Validate start date for range mode
    if (backfillMode === 'range' && !backfillStartDate) {
      backfillError = '请选择起始日期';
      return;
    }

    backfillWeather = true;
    try {
      const response = await fetch('/api/v1/weather/backfill', {
        method: 'POST',
        headers: {
          Authorization: `Bearer ${pb.authStore.token}`,
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          start_date: backfillMode === 'range' ? backfillStartDate : undefined,
          skip_empty: backfillMode === 'content'
        })
      });

      if (!response.ok) {
        const data = await response.json().catch(() => ({}));
        throw new Error(data.error || '请求失败');
      }

      const reader = response.body?.getReader();
      if (!reader) throw new Error('无法读取响应流');

      const decoder = new TextDecoder();
      let buffer = '';

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split('\n');
        buffer = lines.pop() || '';

        let eventType = '';
        for (const line of lines) {
          if (line.startsWith('event: ')) {
            eventType = line.slice(7);
          } else if (line.startsWith('data: ')) {
            try {
              const data = JSON.parse(line.slice(6));
              if (eventType === 'progress') {
                backfillProgress = {
                  current: data.current || 0,
                  total: data.total || 0,
                  status: data.status || '',
                  date: data.date || ''
                };
              } else if (eventType === 'updated') {
                backfillProgress.status = `已更新 ${data.date}`;
              } else if (eventType === 'error') {
                console.error(`补全 ${data.date} 失败:`, data.error);
              } else if (eventType === 'skipped') {
                const reason = data.reason || '未知原因';
                skippedReasons[reason] = (skippedReasons[reason] || 0) + 1;
              } else if (eventType === 'complete') {
                backfillResult = {
                  updated: data.updated,
                  skipped: data.skipped,
                  failed: data.failed,
                  skippedReasons: { ...skippedReasons }
                };
              }
            } catch (e) {
              // Ignore parse errors
            }
          }
        }
      }
    } catch (e) {
      backfillError = e instanceof Error ? e.message : '补全天气失败';
    }
    backfillWeather = false;
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
      setTimeout(() => (copied = false), 2000);
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
      setTimeout(() => (memosSuccess = ''), 3000);
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
      setTimeout(() => (memosSuccess = ''), 3000);
    } catch (e) {
      memosError = e instanceof Error ? e.message : '重置 Memos Webhook URL 失败';
    }
    memosResetting = false;
  }

  async function copyMemosWebhookURL() {
    if (memosSettings.webhook_url) {
      await navigator.clipboard.writeText(memosSettings.webhook_url);
      memosCopied = true;
      setTimeout(() => (memosCopied = false), 2000);
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
    // 若后端未保存自定义提示词，则预填充系统默认值，方便在默认基础上修改
    if (!aiSettings.analysis_system_prompt) {
      aiSettings.analysis_system_prompt = DEFAULT_ANALYSIS_SYSTEM_PROMPT;
    }
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
      if (
        !aiSettings.api_key ||
        !aiSettings.base_url ||
        !aiSettings.chat_model ||
        !aiSettings.embedding_model
      ) {
        aiError = '启用 AI 功能前请填写所有字段';
        return;
      }
    }

    aiSaving = true;
    try {
      await saveAISettings(aiSettings);
      originalAISettings = JSON.parse(JSON.stringify(aiSettings));
      aiSuccess = 'AI 设置已成功保存';
      setTimeout(() => (aiSuccess = ''), 3000);
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
  $: canEnableAI =
    aiSettings.api_key &&
    aiSettings.base_url &&
    aiSettings.chat_model &&
    aiSettings.embedding_model;

  $: memosSettingsChanged =
    memosSettings.enabled !== originalMemosSettings.enabled ||
    memosSettings.base_url !== originalMemosSettings.base_url;

  // Check if AI settings have changed
  $: aiSettingsChanged =
    aiSettings.api_key !== originalAISettings.api_key ||
    aiSettings.base_url !== originalAISettings.base_url ||
    aiSettings.chat_model !== originalAISettings.chat_model ||
    aiSettings.embedding_model !== originalAISettings.embedding_model ||
    (aiSettings.analysis_system_prompt ?? '') !==
      (originalAISettings.analysis_system_prompt ?? '') ||
    (aiSettings.analysis_user_prefix ?? '') !== (originalAISettings.analysis_user_prefix ?? '') ||
    aiSettings.enabled !== originalAISettings.enabled ||
    (aiSettings.speech_provider ?? '') !== (originalAISettings.speech_provider ?? '') ||
    (aiSettings.speech_base_url ?? '') !== (originalAISettings.speech_base_url ?? '') ||
    (aiSettings.speech_api_key ?? '') !== (originalAISettings.speech_api_key ?? '') ||
    (aiSettings.speech_model ?? '') !== (originalAISettings.speech_model ?? '') ||
    (aiSettings.speech_language ?? '') !== (originalAISettings.speech_language ?? '');

  // Embedding model keywords for sorting
  const embeddingKeywords = ['embed', 'bge', 'e5', 'voyage', 'jina'];

  // Check if a model is likely an embedding model
  function isEmbeddingModel(modelId: string): boolean {
    const lower = modelId.toLowerCase();
    return embeddingKeywords.some((keyword) => lower.includes(keyword));
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

  $: canTestChevereto =
    imageUploadSettingsLocal.chevereto.domain && imageUploadSettingsLocal.chevereto.api_key;

  $: imageUploadSettingsChanged =
    JSON.stringify(imageUploadSettingsLocal) !== JSON.stringify(originalImageUploadSettings);

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
      if (
        !imageUploadSettingsLocal.s3.bucket ||
        !imageUploadSettingsLocal.s3.region ||
        !imageUploadSettingsLocal.s3.access_key ||
        !imageUploadSettingsLocal.s3.secret
      ) {
        imageUploadError = 'S3 需要填写 Bucket、region、access key 和 secret';
        return;
      }
    }
    if (imageUploadSettingsLocal.provider === 'chevereto') {
      if (
        !imageUploadSettingsLocal.chevereto.domain ||
        !imageUploadSettingsLocal.chevereto.api_key
      ) {
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
      setTimeout(() => (imageUploadSuccess = ''), 3000);
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

  function handleImportDragOver(e: DragEvent) {
    e.preventDefault();
    e.stopPropagation();
    isDragOver = true;
  }

  function handleImportDragLeave(e: DragEvent) {
    e.preventDefault();
    e.stopPropagation();
    isDragOver = false;
  }

  function handleImportDrop(e: DragEvent) {
    e.preventDefault();
    e.stopPropagation();
    isDragOver = false;
    const files = e.dataTransfer?.files;
    if (files && files.length > 0) {
      const file = files[0];
      if (file.name.endsWith('.json') || file.name.endsWith('.zip')) {
        importFile = file;
      } else {
        importError = '请拖入 .json 或 .zip 格式的文件';
      }
    }
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

  // Notification reminder settings
  let notificationPermissionState: NotificationPermission = 'default';
  let hasPushSubscription = false;
  let notificationBusy = false;
  let notificationError = '';
  let notificationSuccess = '';

  async function loadNotificationState() {
    notificationPermissionState = notificationPermission.current();
    try {
      const { hasActiveSubscription } = await import('#lib/utils/notifications.js');
      hasPushSubscription = await hasActiveSubscription();
    } catch (e) {
      hasPushSubscription = false;
    }
  }

  async function handleRequestNotificationPermission() {
    notificationError = '';
    notificationSuccess = '';
    notificationBusy = true;
    try {
      const ok = await enableNotifications();
      notificationPermissionState = notificationPermission.current();
      const { hasActiveSubscription } = await import('#lib/utils/notifications.js');
      hasPushSubscription = await hasActiveSubscription();
      if (ok && hasPushSubscription) {
        const { updateReminderSettings, getReminderSettings } =
          await import('#lib/stores/notifications.js');
        updateReminderSettings({ enabled: true });
        // Sync schedule so the backend knows reminders are on.
        await saveReminderSettings(getReminderSettings());
        notificationSuccess = '通知已开启 ✓';
      } else if (!ok) {
        notificationError = '未获得通知权限，或当前环境不支持推送（需 HTTPS）。';
      }
    } catch (e) {
      notificationError = e instanceof Error ? e.message : '开启通知失败';
    }
    setTimeout(() => (notificationSuccess = ''), 3000);
    notificationBusy = false;
  }

  async function handleDisableNotification() {
    notificationError = '';
    notificationSuccess = '';
    notificationBusy = true;
    try {
      await disableNotifications();
      hasPushSubscription = false;
      const { updateReminderSettings, getReminderSettings } =
        await import('#lib/stores/notifications.js');
      updateReminderSettings({ enabled: false });
      await saveReminderSettings({ ...getReminderSettings(), enabled: false });
      notificationSuccess = '通知已关闭';
    } catch (e) {
      notificationError = e instanceof Error ? e.message : '关闭通知失败';
    }
    setTimeout(() => (notificationSuccess = ''), 3000);
    notificationBusy = false;
  }

  async function handleSaveNotification() {
    notificationError = '';
    notificationSuccess = '';
    notificationBusy = true;
    try {
      const { getReminderSettings } = await import('#lib/stores/notifications.js');
      await saveReminderSettings(getReminderSettings());
      notificationSuccess = '提醒设置已保存';
    } catch (e) {
      notificationError = e instanceof Error ? e.message : '保存提醒设置失败';
    }
    setTimeout(() => (notificationSuccess = ''), 3000);
    notificationBusy = false;
  }

  async function handleTestNotification() {
    notificationError = '';
    notificationSuccess = '';
    notificationBusy = true;
    try {
      if (!hasPushSubscription) {
        notificationError = '此浏览器尚未开启通知，请先点击“开启通知”。';
        notificationBusy = false;
        return;
      }
      await sendTest();
      notificationSuccess = '测试通知已发送，请留意系统通知栏';
    } catch (e) {
      notificationError = e instanceof Error ? e.message : '发送测试通知失败';
    }
    setTimeout(() => (notificationSuccess = ''), 3000);
    notificationBusy = false;
  }

  // Backup functions
  import { get } from 'svelte/store';
  import {
    backups as backupsStore,
    backupPage as backupPageStore,
    backupPages as backupPagesStore,
    backupSettings as backupSettingsStore,
    loadBackups as fetchBackups,
    loadBackupSettings as fetchBackupSettings,
    saveBackupSettingsLocal as saveBackupSettingsAPI,
    triggerBackupNow,
    deleteBackupById,
    downloadBackupById,
    type Backup
  } from '#lib/stores/backup.js';

  function syncBackupRecordsFromStore() {
    backupRecords = get(backupsStore);
    backupCurrentPage = get(backupPageStore);
    backupTotalPages = get(backupPagesStore);
  }

  async function loadBackupRecords(page = 1) {
    try {
      await fetchBackups(page);
      syncBackupRecordsFromStore();
    } catch (e) {
      backupError = '加载备份记录失败';
      setTimeout(() => (backupError = ''), 3000);
    }
  }

  async function loadBackupSettingsData() {
    backupSettingsLoading = true;
    try {
      await fetchBackupSettings();
      backupSettingsLocal = { ...get(backupSettingsStore) };
      await loadBackupRecords();
    } catch (e) {
      backupError = '加载备份设置失败';
      setTimeout(() => (backupError = ''), 3000);
    } finally {
      backupSettingsLoading = false;
    }
  }

  let backupSettingTimeout: ReturnType<typeof setTimeout> | null = null;

  function handleBackupSettingChange() {
    if (backupSettingTimeout) clearTimeout(backupSettingTimeout);
    backupSettingTimeout = setTimeout(async () => {
      try {
        await saveBackupSettingsAPI(backupSettingsLocal);
        backupSuccess = '设置已保存';
        setTimeout(() => (backupSuccess = ''), 3000);
      } catch (e) {
        backupError = '保存失败';
        setTimeout(() => (backupError = ''), 3000);
      }
    }, 500);
  }

  async function handleTriggerBackup() {
    backupTriggering = true;
    backupError = '';
    backupSuccess = '';
    try {
      await triggerBackupNow();
      syncBackupRecordsFromStore();
      backupSuccess = '备份完成';
      setTimeout(() => (backupSuccess = ''), 3000);
    } catch (e) {
      backupError = e instanceof Error && e.message ? e.message : '备份失败';
      setTimeout(() => (backupError = ''), 5000);
    }
    backupTriggering = false;
  }

  async function handleDownloadBackup(id: string) {
    try {
      await downloadBackupById(id);
    } catch (e) {
      backupError = '下载失败';
      setTimeout(() => (backupError = ''), 3000);
    }
  }

  async function handleDeleteBackup(id: string) {
    if (!confirm('确定要删除这个备份吗？')) return;
    try {
      await deleteBackupById(id);
      syncBackupRecordsFromStore();
    } catch (e) {
      backupError = '删除失败';
      setTimeout(() => (backupError = ''), 3000);
    }
  }

  function formatFileSize(bytes: number): string {
    if (bytes < 1024) return bytes + ' B';
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
    return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
  }

  function formatDate(dateStr: string): string {
    const d = new Date(dateStr);
    return d.toLocaleString('zh-CN', {
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit'
    });
  }

  interface DiffSegment {
    type: 'same' | 'added' | 'removed';
    text: string;
  }

  function computeLineDiff(oldText: string, newText: string): DiffSegment[] {
    const oldLines = oldText.split('\n');
    const newLines = newText.split('\n');
    const result: DiffSegment[] = [];
    let oi = 0;
    let ni = 0;

    while (oi < oldLines.length || ni < newLines.length) {
      if (oi < oldLines.length && ni < newLines.length && oldLines[oi] === newLines[ni]) {
        result.push({ type: 'same', text: oldLines[oi] });
        oi++;
        ni++;
      } else if (
        ni >= newLines.length ||
        (oi < oldLines.length && (ni >= newLines.length || oldLines[oi] < newLines[ni]))
      ) {
        result.push({ type: 'removed', text: oldLines[oi] });
        oi++;
      } else {
        result.push({ type: 'added', text: newLines[ni] });
        ni++;
      }
    }
    return result;
  }

  function hasContentChanged(oldC: string | undefined, newC: string | undefined): boolean {
    return (oldC || '') !== (newC || '');
  }

  function hasMoodChanged(oldM: number | undefined, newM: number | undefined): boolean {
    return (oldM || 0) !== (newM || 0);
  }

  function hasWeatherChanged(oldW: string | undefined, newW: string | undefined): boolean {
    return (oldW || '') !== (newW || '');
  }

  async function handleResolveConflict(detail: ImportDiaryDetail, action: 'keep_old' | 'replace') {
    if (!importStats) return;
    resolvingConflict = true;
    try {
      await resolveConflict(
        detail.date,
        action,
        detail.new_content,
        detail.new_mood,
        detail.new_weather
      );
      if (action === 'replace') {
        importStats.diaries.conflict--;
        importStats.diaries.imported++;
      } else {
        importStats.diaries.conflict--;
        importStats.diaries.skipped++;
      }
      importStats.diary_details = importStats.diary_details?.map((d) =>
        d.date === detail.date ? { ...d, status: action === 'replace' ? 'imported' : 'skipped' } : d
      );
      importStats = { ...importStats };
    } catch (e) {
      importError = e instanceof Error ? e.message : '解决冲突失败';
    }
    resolvingConflict = false;
  }

  function toggleConflictSelection(date: string) {
    const next = new Set(selectedConflicts);
    if (next.has(date)) {
      next.delete(date);
    } else {
      next.add(date);
    }
    selectedConflicts = next;
  }

  function toggleAllConflicts() {
    const conflictDates =
      importStats?.diary_details?.filter((d) => d.status === 'conflict').map((d) => d.date) || [];
    if (selectedConflicts.size === conflictDates.length) {
      selectedConflicts = new Set();
    } else {
      selectedConflicts = new Set(conflictDates);
    }
  }

  async function handleBatchResolveConflict(action: 'keep_old' | 'replace') {
    if (!importStats) return;
    const conflicts =
      importStats.diary_details?.filter(
        (d) => d.status === 'conflict' && selectedConflicts.has(d.date)
      ) || [];
    if (conflicts.length === 0) return;

    resolvingBatch = true;
    importError = '';
    try {
      const result = await resolveConflictBatch(conflicts, action);
      const resolvedDates = new Set(conflicts.map((c) => c.date));
      const newStatus = action === 'replace' ? 'imported' : 'skipped';

      if (action === 'replace') {
        importStats.diaries.conflict -= result.resolved;
        importStats.diaries.imported += result.resolved;
      } else {
        importStats.diaries.conflict -= result.resolved;
        importStats.diaries.skipped += result.resolved;
      }
      importStats.diaries.failed += result.failed;

      importStats.diary_details = importStats.diary_details?.map((d) =>
        resolvedDates.has(d.date) ? { ...d, status: newStatus } : d
      );
      importStats = { ...importStats };
      selectedConflicts = new Set();
    } catch (e) {
      importError = e instanceof Error ? e.message : '批量解决冲突失败';
    }
    resolvingBatch = false;
  }

  onMount(() => {
    syncActiveTabFromHash();

    const handleHashChange = () => {
      syncActiveTabFromHash();
    };

    window.addEventListener('hashchange', handleHashChange);

    const initialize = async () => {
      if (!$isAuthenticated) {
        goto(resolve('/login'));
        return;
      }

      loading = true;
      await Promise.all([
        loadGeneralSettings(),
        loadTokenStatus(),
        loadWeatherSettingsLocal(),
        loadMemosSettingsLocal(),
        loadAISettings(),
        loadImageUploadSettingsLocal(),
        loadNotificationState()
      ]);
      loading = false;
      // Load backup settings separately (may fail if endpoint not deployed)
      loadBackupSettingsData().catch(() => {});
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

<!-- eslint-disable svelte/no-useless-mustaches -->
<!-- 本文件 API 文档区用字面花括号展示 JSON / curl 请求体，Svelte 里必须这样转义 -->

<svelte:head>
  <title>设置 - 吾身</title>
</svelte:head>

<div class="flex min-h-[100dvh] min-h-screen flex-col bg-background">
  <PageHeader title="设置" />

  <!-- Main Content -->
  <div class="container-responsive flex-1 py-6">
    <div class="flex justify-center">
      <main id="main-content" tabindex="-1" class="w-full max-w-5xl">
        <!-- 页面标题 - 视觉隐藏，用于屏幕阅读器的文档结构 -->
        <h1 class="sr-only">设置 — 个性化你的吾身应用</h1>
        <div class="mb-4 space-y-3">
          <div class="sm:hidden">
            <label for="settings-tab-select" class="sr-only">选择设置分区</label>
            <div class="relative">
              <select
                id="settings-tab-select"
                value={activeTab}
                onchange={(event) =>
                  setActiveTab((event.currentTarget as HTMLSelectElement).value as SettingsTab)}
                class="w-full appearance-none rounded-lg border border-border/60 bg-card py-2 pr-9 pl-3 text-sm text-foreground focus:ring-2 focus:ring-primary focus:outline-none"
              >
                {#each settingsTabs as tab (tab.id)}
                  <option value={tab.id}>{tab.label}</option>
                {/each}
              </select>
              <svg
                class="pointer-events-none absolute top-1/2 right-3 h-4 w-4 -translate-y-1/2 text-muted-foreground"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
                aria-hidden="true"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M19 9l-7 7-7-7"
                />
              </svg>
            </div>
          </div>
          <div class="hidden gap-2 overflow-x-auto pb-1 sm:flex">
            {#each settingsTabs as tab (tab.id)}
              <button
                onclick={() => setActiveTab(tab.id)}
                class="rounded-lg border px-3 py-1.5 text-sm whitespace-nowrap transition-colors {activeTab ===
                tab.id
                  ? 'border-primary bg-primary text-primary-foreground'
                  : 'border-border/60 bg-card text-foreground hover:bg-muted/50'}"
              >
                {tab.label}
              </button>
            {/each}
          </div>
        </div>
        {#if loading}
          <div class="flex flex-col items-center justify-center gap-3 py-20">
            <svg class="h-6 w-6 animate-spin text-primary" fill="none" viewBox="0 0 24 24">
              <circle
                class="opacity-25"
                cx="12"
                cy="12"
                r="10"
                stroke="currentColor"
                stroke-width="4"
              ></circle>
              <path
                class="opacity-75"
                fill="currentColor"
                d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
              ></path>
            </svg>
            <div class="text-sm text-muted-foreground">加载中...</div>
          </div>
        {:else}
          <div class="space-y-6">
            {#if activeTab === 'api-access'}
              <!-- API Settings Section -->
              <div
                id="api-access"
                class="animate-fade-in scroll-mt-16 rounded-xl border border-border/50 bg-card p-6 shadow-sm"
              >
                <h2 class="mb-4 text-lg font-semibold text-foreground">API 访问</h2>
                <p class="mb-6 text-sm text-muted-foreground">
                  启用 API 访问以程序化地读取和写入你的日记条目。使用你的 API Token 来认证请求。
                </p>

                <!-- Enable/Disable Toggle -->
                <div class="flex items-center justify-between border-b border-border/50 py-4">
                  <div>
                    <div class="font-medium text-foreground">启用 API</div>
                    <div class="text-sm text-muted-foreground">允许外部读取和写入您的日记数据</div>
                  </div>
                  <button
                    onclick={handleToggle}
                    disabled={toggling}
                    aria-label="切换 API 访问"
                    class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors duration-200 focus:ring-2 focus:ring-primary focus:ring-offset-2 focus:outline-none {tokenStatus.enabled
                      ? 'bg-switch-on'
                      : 'bg-border'}"
                  >
                    <span
                      class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform duration-200 {tokenStatus.enabled
                        ? 'translate-x-6'
                        : 'translate-x-1'}"
                    ></span>
                  </button>
                </div>

                {#if tokenStatus.enabled && tokenStatus.exists}
                  <!-- API Token Display -->
                  <div class="border-b border-border/50 py-4">
                    <div class="mb-2 font-medium text-foreground">您的 API Token</div>
                    <div class="flex items-center gap-2">
                      <code
                        class="flex-1 overflow-x-auto rounded-lg bg-muted px-3 py-2 font-mono text-sm text-foreground"
                      >
                        {tokenStatus.token || '••••••••••••••••••••••••••••••••'}
                      </code>
                      <button
                        onclick={copyToken}
                        disabled={!tokenStatus.token}
                        class="rounded-lg bg-muted px-3 py-2 text-sm transition-colors duration-200 hover:bg-muted/80 disabled:opacity-50"
                      >
                        {copied ? '已复制！' : '复制'}
                      </button>
                    </div>
                    <p class="mt-2 text-xs text-muted-foreground">
                      {#if tokenStatus.token}
                        请妥善保管此 token。任何持有此 token
                        的人都可以读取、修改或删除您的日记内容。
                      {:else}
                        为安全起见，token 仅在创建或重置时显示一次。如需查看 token，请点击下方"重置
                        Token"。
                      {/if}
                    </p>
                  </div>

                  <!-- 重置 Token -->
                  <div class="border-b border-border/50 py-4">
                    <div class="flex items-center justify-between">
                      <div>
                        <div class="font-medium text-foreground">重置 Token</div>
                        <div class="text-sm text-muted-foreground">
                          生成新的 API token（当前 token 将立即失效）
                        </div>
                      </div>
                      <button
                        onclick={handleReset}
                        disabled={resetting}
                        class="bg-destructive/10 text-destructive hover:bg-destructive/20 rounded-lg px-4 py-2 text-sm transition-colors duration-200 disabled:opacity-50"
                      >
                        {resetting ? '重置中...' : '重置 Token'}
                      </button>
                    </div>
                  </div>

                  <!-- API Documentation -->
                  <div class="py-4">
                    <div class="mb-3 font-medium text-foreground">API 使用说明</div>
                    <div class="space-y-4 text-sm">
                      <div>
                        <div class="mb-1 text-muted-foreground">按日期获取日记（GET）：</div>
                        <code
                          class="block overflow-x-auto rounded-lg bg-muted px-3 py-2 font-mono text-xs"
                        >
                          GET {getBaseUrl()}/api/v1/diaries?token={tokenStatus.token ||
                            '<your-token>'}&date=YYYY-MM-DD
                        </code>
                      </div>
                      <div>
                        <div class="mb-1 text-muted-foreground">按日期范围获取日记（GET）：</div>
                        <code
                          class="block overflow-x-auto rounded-lg bg-muted px-3 py-2 font-mono text-xs"
                        >
                          GET {getBaseUrl()}/api/v1/diaries?token={tokenStatus.token ||
                            '<your-token>'}&start=YYYY-MM-DD&end=YYYY-MM-DD
                        </code>
                      </div>
                      <div>
                        <div class="mb-1 text-muted-foreground">创建或更新日记（POST）：</div>
                        <code
                          class="block overflow-x-auto rounded-lg bg-muted px-3 py-2 font-mono text-xs whitespace-pre-wrap"
                        >
                          POST {getBaseUrl()}/api/v1/diaries?token={tokenStatus.token ||
                            '<your-token>'}
                          Content-Type: application/json

                          {'{'}"date":"2025-01-15","content":"今天是个好日子","mood":"开心","weather":"晴"{'}'}
                        </code>
                      </div>
                      <div>
                        <div class="mb-1 text-muted-foreground">按 ID 更新日记（PUT）：</div>
                        <code
                          class="block overflow-x-auto rounded-lg bg-muted px-3 py-2 font-mono text-xs whitespace-pre-wrap"
                        >
                          PUT {getBaseUrl()}/api/v1/diaries/&lt;diary-id&gt;?token={tokenStatus.token ||
                            '<your-token>'}
                          Content-Type: application/json

                          {'{'}"content":"更新后的内容","mood":"平静"{'}'}
                        </code>
                      </div>
                      <div>
                        <div class="mb-1 text-muted-foreground">按 ID 删除日记（DELETE）：</div>
                        <code
                          class="block overflow-x-auto rounded-lg bg-muted px-3 py-2 font-mono text-xs"
                        >
                          DELETE {getBaseUrl()}/api/v1/diaries/&lt;diary-id&gt;?token={tokenStatus.token ||
                            '<your-token>'}
                        </code>
                      </div>
                      <div>
                        <div class="mb-1 text-muted-foreground">按日期删除日记（DELETE）：</div>
                        <code
                          class="block overflow-x-auto rounded-lg bg-muted px-3 py-2 font-mono text-xs"
                        >
                          DELETE {getBaseUrl()}/api/v1/diaries?token={tokenStatus.token ||
                            '<your-token>'}&date=YYYY-MM-DD
                        </code>
                      </div>
                      <div>
                        <div class="mb-1 text-muted-foreground">curl 读取示例：</div>
                        <code
                          class="block overflow-x-auto rounded-lg bg-muted px-3 py-2 font-mono text-xs whitespace-pre-wrap"
                        >
                          curl "{getBaseUrl()}/api/v1/diaries?token={tokenStatus.token ||
                            '<your-token>'}&date={new Date().toISOString().split('T')[0]}"
                        </code>
                      </div>
                      <div>
                        <div class="mb-1 text-muted-foreground">curl 写入示例：</div>
                        <code
                          class="block overflow-x-auto rounded-lg bg-muted px-3 py-2 font-mono text-xs whitespace-pre-wrap"
                        >
                          curl -X POST "{getBaseUrl()}/api/v1/diaries?token={tokenStatus.token ||
                            '<your-token>'}" \ -H "Content-Type: application/json" \ -d '{'{'}"date":"{new Date()
                            .toISOString()
                            .split('T')[0]}","content":"API 测试写入","mood":"","weather":""{'}'}'
                        </code>
                      </div>
                    </div>
                  </div>
                {/if}
              </div>
            {/if}

            {#if activeTab === 'memos-sync'}
              <!-- Memos 同步 Section -->
              <div
                id="memos-sync"
                class="animate-fade-in scroll-mt-16 rounded-xl border border-border/50 bg-card p-6 shadow-sm"
              >
                <h2 class="mb-4 text-lg font-semibold text-foreground">Memos 同步</h2>
                <p class="mb-6 text-sm text-muted-foreground">
                  接收 Memos webhook 事件，并根据每条 memo 的创建日期将同步的 memo
                  块追加到日记中。更新操作会按 memo ID 替换匹配的块，删除操作会移除该块。
                </p>

                {#if memosError}
                  <div class="bg-destructive/10 text-destructive mb-4 rounded-lg p-3 text-sm">
                    {memosError}
                  </div>
                {/if}

                {#if memosSuccess}
                  <div class="mb-4 rounded-lg bg-green-500/10 p-3 text-sm text-green-600">
                    {memosSuccess}
                  </div>
                {/if}

                <div class="flex items-center justify-between border-b border-border/50 py-4">
                  <div>
                    <div class="font-medium text-foreground">启用 Memos Webhook</div>
                    <div class="text-sm text-muted-foreground">
                      为 Memos 生成专用的 Webhook URL 用于接收同步消息
                    </div>
                  </div>
                  <button
                    onclick={() => (memosSettings.enabled = !memosSettings.enabled)}
                    aria-label="切换 Memos 同步"
                    class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors duration-200 focus:ring-2 focus:ring-primary focus:ring-offset-2 focus:outline-none {memosSettings.enabled
                      ? 'bg-switch-on'
                      : 'bg-border'}"
                  >
                    <span
                      class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform duration-200 {memosSettings.enabled
                        ? 'translate-x-6'
                        : 'translate-x-1'}"
                    ></span>
                  </button>
                </div>

                <div class="border-b border-border/50 py-4">
                  <label for="memos-base-url" class="mb-2 block font-medium text-foreground"
                    >Memos 基础 URL</label
                  >
                  <input
                    id="memos-base-url"
                    type="url"
                    bind:value={memosSettings.base_url}
                    placeholder="https://memos.example.com"
                    class="w-full rounded-lg bg-muted px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:ring-2 focus:ring-primary focus:outline-none"
                  />
                  <p class="mt-1 text-xs text-muted-foreground">
                    可选。用于在每个同步块中记录 memo URL，例如 https://memos.example.com/m/123。
                  </p>
                </div>

                {#if memosSettings.enabled && memosSettings.webhook_url}
                  <div class="border-b border-border/50 py-4">
                    <div class="mb-2 font-medium text-foreground">Webhook URL</div>
                    <div class="flex items-center gap-2">
                      <code
                        class="flex-1 overflow-x-auto rounded-lg bg-muted px-3 py-2 font-mono text-sm text-foreground"
                      >
                        {memosSettings.webhook_url}
                      </code>
                      <button
                        onclick={copyMemosWebhookURL}
                        class="rounded-lg bg-muted px-3 py-2 text-sm transition-colors duration-200 hover:bg-muted/80"
                      >
                        {memosCopied ? '已复制！' : '复制'}
                      </button>
                    </div>
                    <p class="mt-2 text-xs text-muted-foreground">
                      将此 URL 粘贴到 Memos webhook
                      设置中。请妥善保密，因为它可以向您的日记写入同步的 memo 块。
                    </p>
                  </div>

                  <div class="border-b border-border/50 py-4">
                    <div class="flex items-center justify-between gap-4">
                      <div>
                        <div class="font-medium text-foreground">重置 Webhook URL</div>
                        <div class="text-sm text-muted-foreground">
                          如果旧 URL 已泄露，请重新生成专用的 Webhook URL
                        </div>
                      </div>
                      <button
                        onclick={handleResetMemosWebhookToken}
                        disabled={memosResetting}
                        class="bg-destructive/10 text-destructive hover:bg-destructive/20 rounded-lg px-4 py-2 text-sm transition-colors duration-200 disabled:opacity-50"
                      >
                        {memosResetting ? '重置中...' : '重置 URL'}
                      </button>
                    </div>
                  </div>
                {/if}

                <div class="flex items-center gap-3 pt-4">
                  <button
                    onclick={handleSaveMemosSettings}
                    disabled={memosSaving || !memosSettingsChanged}
                    class="rounded-lg bg-primary px-4 py-2 text-primary-foreground transition-colors duration-200 hover:bg-primary/90 disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    {memosSaving ? '保存中...' : '保存 Memos 设置'}
                  </button>
                </div>
              </div>
            {/if}

            {#if activeTab === 'weather'}
              <!-- 天气设置 Section -->
              <div
                id="weather"
                class="animate-fade-in scroll-mt-16 rounded-xl border border-border/50 bg-card p-6 shadow-sm"
              >
                <div class="mb-4 flex items-center justify-between gap-3">
                  <h2 class="text-lg font-semibold text-foreground">天气设置</h2>
                </div>
                <p class="mb-6 text-sm text-muted-foreground">
                  配置天气服务。数据来源：{qweatherEnabled
                    ? '和风天气（已配置 API Key，优先使用）'
                    : 'Open-Meteo（免费、无需 API Key）'}。
                </p>
                {#if qweatherEnabled}
                  <div
                    class="mb-4 flex items-center gap-2 rounded-lg bg-blue-500/10 p-3 text-sm text-blue-600"
                  >
                    <svg
                      class="h-5 w-5 flex-shrink-0"
                      fill="none"
                      stroke="currentColor"
                      viewBox="0 0 24 24"
                    >
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                      />
                    </svg>
                    已检测到服务端配置了和风天气 API Key，将优先使用和风天气获取实时天气数据（Open-Meteo
                    作为降级方案）。
                  </div>
                {/if}

                {#if weatherError}
                  <div class="mb-4 rounded-lg bg-red-500/10 p-3 text-sm text-red-600">
                    {weatherError}
                  </div>
                {/if}

                {#if weatherSuccess}
                  <div class="mb-4 rounded-lg bg-green-500/10 p-3 text-sm text-green-600">
                    {weatherSuccess}
                  </div>
                {/if}

                <div class="space-y-4">
                  <!-- Enable Weather -->
                  <div class="flex items-center justify-between rounded-lg bg-muted/30 p-4">
                    <div>
                      <div class="font-medium text-foreground">启用天气功能</div>
                      <div class="text-sm text-muted-foreground">
                        开启后可在日记中选择城市并自动获取天气
                      </div>
                    </div>
                    <button
                      type="button"
                      onclick={() => (weatherEnabled = !weatherEnabled)}
                      class="relative inline-flex h-6 w-11 flex-shrink-0 items-center rounded-full transition-colors duration-200 focus:ring-2 focus:ring-primary focus:ring-offset-2 focus:outline-none {weatherEnabled
                        ? 'bg-switch-on'
                        : 'bg-border'}"
                    >
                      <span
                        class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform duration-200 {weatherEnabled
                          ? 'translate-x-6'
                          : 'translate-x-1'}"
                      ></span>
                    </button>
                  </div>

                  {#if weatherEnabled}
                    <!-- Default City -->
                    <div class="rounded-lg bg-muted/30 p-4">
                      <label
                        for="weather-default-city"
                        class="mb-2 block text-sm font-medium text-foreground">默认城市</label
                      >
                      <input
                        id="weather-default-city"
                        type="text"
                        bind:value={weatherDefaultCity}
                        placeholder="例如：北京"
                        class="w-full rounded-lg bg-muted px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:ring-2 focus:ring-primary focus:outline-none"
                      />
                      <p class="mt-1 text-xs text-muted-foreground">
                        设置后新建日记时会自动选择该城市
                      </p>
                    </div>

                    <!-- Auto Fetch -->
                    <div class="flex items-center justify-between rounded-lg bg-muted/30 p-4">
                      <div>
                        <div class="font-medium text-foreground">自动获取天气</div>
                        <div class="text-sm text-muted-foreground">
                          每天定时自动获取当天天气并更新到日记
                        </div>
                      </div>
                      <button
                        type="button"
                        onclick={() => (weatherAutoFetch = !weatherAutoFetch)}
                        class="relative inline-flex h-6 w-11 flex-shrink-0 items-center rounded-full transition-colors duration-200 focus:ring-2 focus:ring-primary focus:ring-offset-2 focus:outline-none {weatherAutoFetch
                          ? 'bg-switch-on'
                          : 'bg-border'}"
                      >
                        <span
                          class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform duration-200 {weatherAutoFetch
                            ? 'translate-x-6'
                            : 'translate-x-1'}"
                        ></span>
                      </button>
                    </div>

                    {#if weatherAutoFetch}
                      <div class="rounded-lg bg-muted/30 p-4">
                        <label
                          for="weather-auto-fetch-time"
                          class="mb-2 block text-sm font-medium text-foreground">自动获取时间</label
                        >
                        <input
                          id="weather-auto-fetch-time"
                          type="time"
                          bind:value={weatherAutoFetchTime}
                          class="w-32 rounded-lg bg-muted px-3 py-2 text-sm text-foreground focus:ring-2 focus:ring-primary focus:outline-none"
                        />
                        <p class="mt-1 text-xs text-muted-foreground">
                          设置后服务器将在指定时间自动获取当天天气
                        </p>
                      </div>
                    {/if}
                  {/if}
                </div>

                <div class="flex items-center gap-3 pt-4">
                  <button
                    onclick={handleSaveWeatherSettings}
                    disabled={weatherSaving}
                    class="flex items-center gap-2 rounded-lg bg-primary px-4 py-2 text-primary-foreground transition-colors duration-200 hover:bg-primary/90 disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    {#if weatherSaving}
                      <svg class="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
                        <circle
                          class="opacity-25"
                          cx="12"
                          cy="12"
                          r="10"
                          stroke="currentColor"
                          stroke-width="4"
                        ></circle>
                        <path
                          class="opacity-75"
                          fill="currentColor"
                          d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                        ></path>
                      </svg>
                      保存中...
                    {:else}
                      保存天气设置
                    {/if}
                  </button>
                  {#if weatherSuccess}
                    <span class="animate-fade-in flex items-center gap-1 text-sm text-green-600">
                      <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M5 13l4 4L19 7"
                        />
                      </svg>
                      已保存
                    </span>
                  {/if}
                </div>

                <!-- Backfill Weather -->
                <div class="mt-4 rounded-lg bg-muted/30 p-4">
                  <h3 class="mb-2 text-sm font-medium text-foreground">补全历史天气</h3>
                  <p class="mb-3 text-xs text-muted-foreground">
                    一键为缺失天气数据的日记补全天气（基于默认城市）
                  </p>

                  <!-- Options -->
                  <div class="mb-4 space-y-3">
                    <div class="flex items-center gap-2">
                      <input
                        id="backfill-mode-content"
                        type="radio"
                        name="backfillMode"
                        value="content"
                        bind:group={backfillMode}
                        class="h-4 w-4 border-muted-foreground text-primary focus:ring-primary"
                      />
                      <label for="backfill-mode-content" class="text-xs text-muted-foreground"
                        >补全所有有内容的日记</label
                      >
                    </div>
                    <div class="flex items-center gap-2">
                      <input
                        id="backfill-mode-range"
                        type="radio"
                        name="backfillMode"
                        value="range"
                        bind:group={backfillMode}
                        class="h-4 w-4 border-muted-foreground text-primary focus:ring-primary"
                      />
                      <label for="backfill-mode-range" class="text-xs text-muted-foreground"
                        >补全指定日期至今的所有日记</label
                      >
                    </div>
                    {#if backfillMode === 'range'}
                      <div class="ml-6">
                        <label
                          for="backfill-start-date"
                          class="mb-1 block text-xs text-muted-foreground">起始日期 *</label
                        >
                        <input
                          id="backfill-start-date"
                          type="date"
                          bind:value={backfillStartDate}
                          required
                          class="w-full rounded-lg bg-muted px-3 py-2 text-sm text-foreground focus:ring-2 focus:ring-primary focus:outline-none"
                        />
                        <p class="mt-1 text-xs text-muted-foreground">
                          将补全从该日期到今天的所有日记天气
                        </p>
                      </div>
                    {/if}
                  </div>

                  <button
                    onclick={handleBackfillWeather}
                    disabled={backfillWeather}
                    class="flex items-center gap-2 rounded-lg bg-secondary px-4 py-2 text-sm text-secondary-foreground transition-colors duration-200 hover:bg-secondary/80 disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    {#if backfillWeather}
                      <svg class="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
                        <circle
                          class="opacity-25"
                          cx="12"
                          cy="12"
                          r="10"
                          stroke="currentColor"
                          stroke-width="4"
                        ></circle>
                        <path
                          class="opacity-75"
                          fill="currentColor"
                          d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                        ></path>
                      </svg>
                      补全中...
                    {:else}
                      <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
                        />
                      </svg>
                      补全天气
                    {/if}
                  </button>

                  <!-- Progress bar -->
                  {#if backfillWeather && backfillProgress.total > 0}
                    <div class="mt-3">
                      <div class="mb-1 flex justify-between text-xs text-muted-foreground">
                        <span>{backfillProgress.status}</span>
                        <span>{backfillProgress.current} / {backfillProgress.total}</span>
                      </div>
                      <div class="h-2 w-full rounded-full bg-muted">
                        <div
                          class="h-2 rounded-full bg-primary transition-all duration-300"
                          style="width: {backfillProgress.total > 0
                            ? (backfillProgress.current / backfillProgress.total) * 100
                            : 0}%"
                        ></div>
                      </div>
                    </div>
                  {/if}

                  {#if backfillResult}
                    <div class="mt-3 rounded-lg bg-background p-3 text-sm">
                      <p class="text-foreground">
                        补全完成：更新 {backfillResult.updated} 篇，跳过 {backfillResult.skipped} 篇，失败
                        {backfillResult.failed} 篇
                      </p>
                      {#if backfillResult.skippedReasons && Object.keys(backfillResult.skippedReasons).length > 0}
                        <div class="mt-2 text-xs text-muted-foreground">
                          <p class="mb-1 font-medium">跳过原因：</p>
                          {#each Object.entries(backfillResult.skippedReasons) as [reason, count] (reason)}
                            <p>- {reason}：{count} 篇</p>
                          {/each}
                        </div>
                      {/if}
                    </div>
                  {/if}
                  {#if backfillError}
                    <div class="bg-destructive/10 text-destructive mt-3 rounded-lg p-3 text-sm">
                      {backfillError}
                    </div>
                  {/if}
                </div>
              </div>
            {/if}

            {#if activeTab === 'general'}
              <!-- General Settings Section -->
              <div
                id="general"
                class="animate-fade-in scroll-mt-16 rounded-xl border border-border/50 bg-card p-6 shadow-sm"
              >
                <h2 class="mb-4 text-lg font-semibold text-foreground">通用设置</h2>
                <p class="mb-6 text-sm text-muted-foreground">配置应用的基本行为和偏好。</p>

                {#if generalError}
                  <div class="bg-destructive/10 text-destructive mb-4 rounded-lg p-3 text-sm">
                    {generalError}
                  </div>
                {/if}

                {#if generalSuccess}
                  <div class="mb-4 rounded-lg bg-green-500/10 p-3 text-sm text-green-600">
                    {generalSuccess}
                  </div>
                {/if}

                <div class="space-y-6">
                  <!-- Default View -->
                  <div>
                    <label class="mb-2 block text-sm font-medium text-foreground">默认页面</label>
                    <p class="mb-3 text-xs text-muted-foreground">选择应用打开时显示的页面</p>
                    <div class="flex gap-2">
                      <button
                        type="button"
                        onclick={() => {
                          defaultView = 'diary';
                        }}
                        class="rounded-lg px-4 py-2 text-sm transition-colors {defaultView ===
                        'diary'
                          ? 'bg-primary text-primary-foreground'
                          : 'bg-muted text-muted-foreground hover:bg-muted/80'}"
                      >
                        当天编辑
                      </button>
                      <button
                        type="button"
                        onclick={() => {
                          defaultView = 'calendar';
                        }}
                        class="rounded-lg px-4 py-2 text-sm transition-colors {defaultView ===
                        'calendar'
                          ? 'bg-primary text-primary-foreground'
                          : 'bg-muted text-muted-foreground hover:bg-muted/80'}"
                      >
                        日历视图
                      </button>
                    </div>
                  </div>

                  <!-- Diary version retention -->
                  <div>
                    <label
                      class="mb-2 block text-sm font-medium text-foreground"
                      for="version-retention-days">日记版本保留天数</label
                    >
                    <p class="mb-3 text-xs text-muted-foreground">
                      每次进入编辑框保存时自动记录编辑前的内容，超过保留天数的版本会被自动删除
                    </p>
                    <div class="flex items-center gap-2">
                      <input
                        id="version-retention-days"
                        type="number"
                        min="1"
                        max="3650"
                        bind:value={versionRetentionDays}
                        class="w-28 rounded-lg border border-border/60 bg-background px-3 py-2 text-sm text-foreground focus:ring-2 focus:ring-primary/40 focus:outline-none"
                      />
                      <span class="text-xs text-muted-foreground">天（默认 30）</span>
                    </div>
                  </div>

                  <!-- Font Size -->
                  <div>
                    <label class="mb-2 block text-sm font-medium text-foreground">字体大小</label>
                    <p class="mb-3 text-xs text-muted-foreground">调整编辑器和内容的显示字体大小</p>
                    <div class="flex gap-2">
                      <button
                        type="button"
                        onclick={() => handleFontSizeChange('small')}
                        class="rounded-lg px-4 py-2 text-sm transition-colors {fontSize === 'small'
                          ? 'bg-primary text-primary-foreground'
                          : 'bg-muted text-muted-foreground hover:bg-muted/80'}"
                      >
                        小
                      </button>
                      <button
                        type="button"
                        onclick={() => handleFontSizeChange('medium')}
                        class="rounded-lg px-4 py-2 text-sm transition-colors {fontSize === 'medium'
                          ? 'bg-primary text-primary-foreground'
                          : 'bg-muted text-muted-foreground hover:bg-muted/80'}"
                      >
                        中
                      </button>
                      <button
                        type="button"
                        onclick={() => handleFontSizeChange('large')}
                        class="rounded-lg px-4 py-2 text-sm transition-colors {fontSize === 'large'
                          ? 'bg-primary text-primary-foreground'
                          : 'bg-muted text-muted-foreground hover:bg-muted/80'}"
                      >
                        大
                      </button>
                    </div>
                  </div>

                  <!-- Theme -->
                  <div>
                    <label class="mb-2 block text-sm font-medium text-foreground">外观模式</label>
                    <p class="mb-3 text-xs text-muted-foreground">选择浅色、深色或跟随系统</p>
                    <div class="flex gap-2">
                      <button
                        type="button"
                        onclick={() => handleThemeChange('light')}
                        class="flex items-center gap-1.5 rounded-lg px-4 py-2 text-sm transition-colors {currentTheme ===
                        'light'
                          ? 'bg-primary text-primary-foreground'
                          : 'bg-muted text-muted-foreground hover:bg-muted/80'}"
                      >
                        <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            stroke-width="2"
                            d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z"
                          />
                        </svg>
                        浅色
                      </button>
                      <button
                        type="button"
                        onclick={() => handleThemeChange('dark')}
                        class="flex items-center gap-1.5 rounded-lg px-4 py-2 text-sm transition-colors {currentTheme ===
                        'dark'
                          ? 'bg-primary text-primary-foreground'
                          : 'bg-muted text-muted-foreground hover:bg-muted/80'}"
                      >
                        <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            stroke-width="2"
                            d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z"
                          />
                        </svg>
                        深色
                      </button>
                      <button
                        type="button"
                        onclick={() => handleThemeChange('system')}
                        class="flex items-center gap-1.5 rounded-lg px-4 py-2 text-sm transition-colors {currentTheme ===
                        'system'
                          ? 'bg-primary text-primary-foreground'
                          : 'bg-muted text-muted-foreground hover:bg-muted/80'}"
                      >
                        <svg
                          class="h-4 w-4"
                          viewBox="0 0 24 24"
                          fill="none"
                          stroke="currentColor"
                          stroke-width="2"
                          stroke-linecap="round"
                          stroke-linejoin="round"
                        >
                          <path d="M12 3a9 9 0 010 18" fill="currentColor" />
                          <circle cx="12" cy="12" r="9" />
                        </svg>
                        跟随系统
                      </button>
                    </div>
                  </div>
                </div>

                <div class="mt-6 border-t border-border/50 pt-4">
                  <button
                    type="button"
                    onclick={handleSaveGeneralSettings}
                    disabled={!generalSettingsChanged || savingGeneral}
                    class="rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    {savingGeneral ? '保存中...' : '保存'}
                  </button>
                </div>
              </div>
            {/if}

            {#if activeTab === 'notifications'}
              <!-- Notification Reminder Section -->
              <div
                id="notifications"
                class="animate-fade-in scroll-mt-16 rounded-xl border border-border/50 bg-card p-6 shadow-sm"
              >
                <h2 class="mb-4 text-lg font-semibold text-foreground">通知提醒</h2>
                <p class="mb-6 text-sm text-muted-foreground">
                  设置每天固定的提醒时间，通过系统通知提醒你写日记。当天已经写过日记时不会打扰你。
                </p>

                {#if notificationError}
                  <div class="bg-destructive/10 text-destructive mb-4 rounded-lg p-3 text-sm">
                    {notificationError}
                  </div>
                {/if}

                {#if notificationSuccess}
                  <div class="mb-4 rounded-lg bg-green-500/10 p-3 text-sm text-green-600">
                    {notificationSuccess}
                  </div>
                {/if}

                <div class="space-y-4">
                  <!-- Enable toggle -->
                  <div class="flex items-center justify-between rounded-lg bg-muted/30 p-4">
                    <div>
                      <div class="font-medium text-foreground">开启日记提醒</div>
                      <div class="text-sm text-muted-foreground">
                        {$reminderSettings.enabled ? '已开启' : '已关闭'}
                        {#if notificationPermissionState === 'granted' && hasPushSubscription}
                          · 此设备已订阅推送
                        {:else if notificationPermissionState === 'denied'}
                          · 系统通知权限已被拒绝
                        {/if}
                      </div>
                    </div>
                    <button
                      type="button"
                      onclick={() => {
                        if (!$reminderSettings.enabled) {
                          handleRequestNotificationPermission();
                        } else {
                          handleDisableNotification();
                        }
                      }}
                      disabled={notificationBusy}
                      class="relative inline-flex h-6 w-11 flex-shrink-0 items-center rounded-full transition-colors duration-200 focus:ring-2 focus:ring-primary focus:ring-offset-2 focus:outline-none {$reminderSettings.enabled
                        ? 'bg-switch-on'
                        : 'bg-border'}"
                    >
                      <span
                        class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform duration-200 {$reminderSettings.enabled
                          ? 'translate-x-6'
                          : 'translate-x-1'}"
                      ></span>
                    </button>
                  </div>

                  {#if $reminderSettings.enabled}
                    <!-- Reminder time -->
                    <div class="rounded-lg bg-muted/30 p-4">
                      <label
                        for="reminder-time"
                        class="mb-2 block text-sm font-medium text-foreground">每日提醒时间</label
                      >
                      <input
                        id="reminder-time"
                        type="time"
                        bind:value={$reminderSettings.time}
                        class="w-32 rounded-lg bg-muted px-3 py-2 text-sm text-foreground focus:ring-2 focus:ring-primary focus:outline-none"
                      />
                      <p class="mt-1 text-xs text-muted-foreground">
                        到达该时间时若当天还未写日记，将收到系统通知。
                      </p>
                    </div>

                    <!-- Reminder message -->
                    <div class="rounded-lg bg-muted/30 p-4">
                      <label
                        for="reminder-message"
                        class="mb-2 block text-sm font-medium text-foreground">提醒文案</label
                      >
                      <input
                        id="reminder-message"
                        type="text"
                        maxlength="80"
                        bind:value={$reminderSettings.message}
                        placeholder="该写今天的日记啦 ✍️"
                        class="w-full rounded-lg bg-muted px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:ring-2 focus:ring-primary focus:outline-none"
                      />
                    </div>

                    <div class="flex flex-wrap items-center gap-3 pt-1">
                      <button
                        type="button"
                        onclick={handleSaveNotification}
                        disabled={notificationBusy}
                        class="rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 disabled:cursor-not-allowed disabled:opacity-50"
                      >
                        {notificationBusy ? '保存中...' : '保存提醒设置'}
                      </button>
                      <button
                        type="button"
                        onclick={handleTestNotification}
                        disabled={notificationBusy || !hasPushSubscription}
                        class="rounded-lg bg-muted px-4 py-2 text-sm font-medium text-foreground transition-colors hover:bg-muted/80 disabled:cursor-not-allowed disabled:opacity-50"
                      >
                        发送测试通知
                      </button>
                    </div>
                  {/if}

                  <!-- HTTPS/unsupported notice -->
                  <div class="space-y-1 rounded-lg bg-muted/30 p-4 text-xs text-muted-foreground">
                    <p>
                      推送通知基于 Web Push，需要 <span class="text-foreground">HTTPS</span>（或
                      localhost）环境才能注册订阅。 若在设置中无法开启，请确认站点通过 HTTPS 访问。
                    </p>
                    <p>当天已写过日记（有内容）时，提醒将自动跳过，不会打扰你。</p>
                  </div>
                </div>
              </div>
            {/if}

            {#if activeTab === 'ai-assistant'}
              <!-- AI 助手核心设置 -->
              <div
                id="ai-assistant"
                class="animate-fade-in scroll-mt-16 rounded-xl border border-border/50 bg-card p-6 shadow-sm"
              >
                <h2 class="mb-4 text-lg font-semibold text-foreground">AI 助手</h2>
                <p class="mb-6 text-sm text-muted-foreground">
                  配置 AI 服务以实现智能日记分析与对话。支持与 OpenAI 兼容的 API。
                </p>

                {#if aiError}
                  <div class="bg-destructive/10 text-destructive mb-4 rounded-lg p-3 text-sm">
                    {aiError}
                  </div>
                {/if}

                {#if aiSuccess}
                  <div class="mb-4 rounded-lg bg-green-500/10 p-3 text-sm text-green-600">
                    {aiSuccess}
                  </div>
                {/if}

                <!-- API Key -->
                <div class="border-b border-border/50 py-4">
                  <label for="ai-api-key" class="mb-2 block font-medium text-foreground"
                    >API 密钥</label
                  >
                  <input
                    id="ai-api-key"
                    type="password"
                    bind:value={aiSettings.api_key}
                    placeholder="sk-..."
                    class="w-full rounded-lg bg-muted px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:ring-2 focus:ring-primary focus:outline-none"
                  />
                  <p class="mt-1 text-xs text-muted-foreground">
                    AI 服务的 API Key。OpenAI key 以 sk- 开头，例如 sk-xxx...
                  </p>
                </div>

                <!-- Base URL -->
                <div class="border-b border-border/50 py-4">
                  <label for="ai-base-url" class="mb-2 block font-medium text-foreground"
                    >API Base URL</label
                  >
                  <input
                    id="ai-base-url"
                    type="text"
                    bind:value={aiSettings.base_url}
                    placeholder="https://api.openai.com"
                    class="w-full rounded-lg bg-muted px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:ring-2 focus:ring-primary focus:outline-none"
                  />
                  <p class="mt-1 text-xs text-muted-foreground">
                    与 OpenAI 兼容的 API Base URL，例如 https://api.openai.com
                  </p>
                </div>

                {#if modelsError}
                  <div class="bg-destructive/10 text-destructive mt-4 rounded-lg p-3 text-sm">
                    {modelsError}
                  </div>
                {/if}

                <!-- 聊天模型 -->
                <div class="border-b border-border/50 py-4">
                  <label for="ai-chat-model" class="mb-2 block font-medium text-foreground"
                    >聊天模型</label
                  >
                  <div class="flex items-center gap-2">
                    <div class="relative flex-1">
                      <select
                        id="ai-chat-model"
                        bind:value={aiSettings.chat_model}
                        class="w-full appearance-none rounded-lg bg-muted py-2 pr-9 pl-3 text-sm text-foreground focus:ring-2 focus:ring-primary focus:outline-none"
                      >
                        <option value="">选择一个模型</option>
                        {#each chatModels as model (model.id)}
                          <option value={model.id}>{model.id}</option>
                        {/each}
                      </select>
                      <svg
                        class="pointer-events-none absolute top-1/2 right-3 h-4 w-4 -translate-y-1/2 text-muted-foreground"
                        fill="none"
                        stroke="currentColor"
                        viewBox="0 0 24 24"
                        aria-hidden="true"
                      >
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M19 9l-7 7-7-7"
                        />
                      </svg>
                    </div>
                    <button
                      onclick={handleFetchModels}
                      disabled={fetchingModels}
                      class="rounded-lg bg-muted p-2 transition-colors duration-200 hover:bg-muted/80 disabled:opacity-50"
                      title="刷新模型列表"
                    >
                      <svg
                        class="h-5 w-5 {fetchingModels ? 'animate-spin' : ''}"
                        fill="none"
                        stroke="currentColor"
                        viewBox="0 0 24 24"
                      >
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
                        />
                      </svg>
                    </button>
                  </div>
                  <p class="mt-1 text-xs text-muted-foreground">
                    AI 对话使用的模型，例如 gpt-4o、deepseek-chat
                  </p>
                </div>

                <!-- 嵌入模型 -->
                <div class="border-b border-border/50 py-4">
                  <label for="ai-embedding-model" class="mb-2 block font-medium text-foreground"
                    >嵌入模型</label
                  >
                  <div class="flex items-center gap-2">
                    <div class="relative flex-1">
                      <select
                        id="ai-embedding-model"
                        bind:value={aiSettings.embedding_model}
                        class="w-full appearance-none rounded-lg bg-muted py-2 pr-9 pl-3 text-sm text-foreground focus:ring-2 focus:ring-primary focus:outline-none"
                      >
                        <option value="">选择一个模型</option>
                        {#each embeddingModels as model (model.id)}
                          <option value={model.id}>{model.id}</option>
                        {/each}
                      </select>
                      <svg
                        class="pointer-events-none absolute top-1/2 right-3 h-4 w-4 -translate-y-1/2 text-muted-foreground"
                        fill="none"
                        stroke="currentColor"
                        viewBox="0 0 24 24"
                        aria-hidden="true"
                      >
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M19 9l-7 7-7-7"
                        />
                      </svg>
                    </div>
                    <button
                      onclick={handleFetchModels}
                      disabled={fetchingModels}
                      class="rounded-lg bg-muted p-2 transition-colors duration-200 hover:bg-muted/80 disabled:opacity-50"
                      title="刷新模型列表"
                    >
                      <svg
                        class="h-5 w-5 {fetchingModels ? 'animate-spin' : ''}"
                        fill="none"
                        stroke="currentColor"
                        viewBox="0 0 24 24"
                      >
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
                        />
                      </svg>
                    </button>
                  </div>
                  <p class="mt-1 text-xs text-muted-foreground">
                    文本向量化使用的模型，例如 text-embedding-3-small
                  </p>
                </div>

                <!-- Enable AI Toggle -->
                <div class="border-b border-border/50 py-4">
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
                      onclick={() => {
                        if (canEnableAI) aiSettings.enabled = !aiSettings.enabled;
                      }}
                      disabled={!canEnableAI && !aiSettings.enabled}
                      aria-label="切换 AI 功能"
                      class="relative inline-flex h-6 w-11 flex-shrink-0 items-center rounded-full transition-colors duration-200 focus:ring-2 focus:ring-primary focus:ring-offset-2 focus:outline-none {aiSettings.enabled
                        ? 'bg-switch-on'
                        : 'bg-border'} {!canEnableAI && !aiSettings.enabled
                        ? 'cursor-not-allowed opacity-50'
                        : ''}"
                    >
                      <span
                        class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform duration-200 {aiSettings.enabled
                          ? 'translate-x-6'
                          : 'translate-x-1'}"
                      ></span>
                    </button>
                  </div>
                </div>

                <!-- 周/月分析提示词 -->
                <div class="border-b border-border/50 py-4">
                  <div class="mb-3 font-medium text-foreground">周/月分析提示词</div>
                  <div class="mb-4">
                    <div class="mb-1.5 flex items-center justify-between">
                      <label for="analysis-system-prompt" class="text-sm text-muted-foreground"
                        >系统提示词 (System Prompt)</label
                      >
                      <button
                        type="button"
                        onclick={() => {
                          aiSettings.analysis_system_prompt = DEFAULT_ANALYSIS_SYSTEM_PROMPT;
                        }}
                        class="rounded px-2 py-0.5 text-xs text-muted-foreground transition-colors duration-200 hover:bg-primary/10 hover:text-primary"
                      >
                        恢复默认
                      </button>
                    </div>
                    <textarea
                      id="analysis-system-prompt"
                      rows={6}
                      bind:value={aiSettings.analysis_system_prompt}
                      placeholder="你是一个贴心的日记分析助手……使用中文回答。"
                      class="w-full resize-y rounded-lg border border-border/70 bg-muted/50 px-3 py-2 font-mono text-sm leading-relaxed text-foreground placeholder:text-muted-foreground focus:border-primary focus:ring-2 focus:ring-primary focus:outline-none"
                    />
                    <p class="mt-1 text-xs text-muted-foreground">
                      用于 AI 分析日记的系统角色与行为指令；留空时使用系统默认提示词。
                    </p>
                  </div>
                  <div>
                    <label
                      for="analysis-user-prefix"
                      class="mb-1.5 block text-sm text-muted-foreground"
                      >内容引导语 (User Prefix，可选)</label
                    >
                    <textarea
                      id="analysis-user-prefix"
                      rows={3}
                      bind:value={aiSettings.analysis_user_prefix}
                      placeholder="以下是本周（起始日期 ~ 结束日期）的日记……"
                      class="w-full resize-y rounded-lg border border-border/70 bg-muted/50 px-3 py-2 font-mono text-sm leading-relaxed text-foreground placeholder:text-muted-foreground focus:border-primary focus:ring-2 focus:ring-primary focus:outline-none"
                    />
                    <p class="mt-1 text-xs text-muted-foreground">
                      出现在每篇日记内容之前的引导语；留空时使用默认的"周/月"格式化提示。可在其中加入自己的强调重点。
                    </p>
                  </div>
                </div>

                <!-- Save Button -->
                <div class="flex items-center gap-3 pt-4">
                  <button
                    onclick={handleSaveAISettings}
                    disabled={aiSaving || !aiSettingsChanged}
                    class="flex items-center gap-2 rounded-lg bg-primary px-4 py-2 text-primary-foreground transition-colors duration-200 hover:bg-primary/90 disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    {#if aiSaving}
                      <svg class="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
                        <circle
                          class="opacity-25"
                          cx="12"
                          cy="12"
                          r="10"
                          stroke="currentColor"
                          stroke-width="4"
                        ></circle>
                        <path
                          class="opacity-75"
                          fill="currentColor"
                          d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                        ></path>
                      </svg>
                      保存中...
                    {:else}
                      保存设置
                    {/if}
                  </button>
                  {#if aiSuccess}
                    <span class="animate-fade-in flex items-center gap-1 text-sm text-green-600">
                      <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M5 13l4 4L19 7"
                        />
                      </svg>
                      已保存
                    </span>
                  {/if}
                </div>
              </div>
            {/if}

            {#if activeTab === 'ai-speech'}
              <!-- 语音输入设置 -->
              <div
                id="ai-speech"
                class="animate-fade-in scroll-mt-16 rounded-xl border border-border/50 bg-card p-6 shadow-sm"
              >
                <h2 class="mb-4 text-lg font-semibold text-foreground">语音输入</h2>
                <p class="mb-6 text-sm text-muted-foreground">
                  配置 AI 语音转文字，在日记编辑器中用录音按钮直接语音输入。仅支持 OpenAI 兼容的
                  /v1/audio/transcriptions 接口（如 whisper、groq、本地 Whisper 部署等）。
                </p>

                {#if aiError}
                  <div class="bg-destructive/10 text-destructive mb-4 rounded-lg p-3 text-sm">
                    {aiError}
                  </div>
                {/if}

                {#if aiSuccess}
                  <div class="mb-4 rounded-lg bg-green-500/10 p-3 text-sm text-green-600">
                    {aiSuccess}
                  </div>
                {/if}

                <div class="space-y-6">
                  <!-- 启用语音识别 -->
                  <div>
                    <label for="speech-provider" class="mb-2 block font-medium text-foreground"
                      >语音识别服务</label
                    >
                    <select
                      id="speech-provider"
                      bind:value={aiSettings.speech_provider}
                      class="w-full rounded-lg bg-muted px-3 py-2 text-sm text-foreground focus:ring-2 focus:ring-primary focus:outline-none"
                    >
                      <option value="none">不启用</option>
                      <option value="openai">OpenAI 兼容接口（推荐）</option>
                    </select>
                    <p class="mt-1 text-xs text-muted-foreground">
                      若未单独填写语音 API 密钥与地址，将回退使用"AI 助手"的 Base URL 与密钥。
                    </p>
                  </div>

                  <div class="grid gap-4 md:grid-cols-2">
                    <div>
                      <label for="speech-base-url" class="mb-2 block font-medium text-foreground"
                        >API Base URL（可选）</label
                      >
                      <input
                        id="speech-base-url"
                        type="text"
                        bind:value={aiSettings.speech_base_url}
                        placeholder="例如 https://api.openai.com"
                        class="w-full rounded-lg bg-muted px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:ring-2 focus:ring-primary focus:outline-none"
                      />
                    </div>
                    <div>
                      <label for="speech-api-key" class="mb-2 block font-medium text-foreground"
                        >API 密钥（可选）</label
                      >
                      <input
                        id="speech-api-key"
                        type="password"
                        bind:value={aiSettings.speech_api_key}
                        placeholder="留空则复用 AI 助手密钥"
                        class="w-full rounded-lg bg-muted px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:ring-2 focus:ring-primary focus:outline-none"
                      />
                    </div>
                  </div>

                  <div class="grid gap-4 md:grid-cols-2">
                    <div>
                      <label for="speech-model" class="mb-2 block font-medium text-foreground"
                        >模型名称</label
                      >
                      <input
                        id="speech-model"
                        type="text"
                        bind:value={aiSettings.speech_model}
                        placeholder="whisper-1"
                        class="w-full rounded-lg bg-muted px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:ring-2 focus:ring-primary focus:outline-none"
                      />
                    </div>
                    <div>
                      <label for="speech-language" class="mb-2 block font-medium text-foreground"
                        >默认语言 (ISO-639-1)</label
                      >
                      <input
                        id="speech-language"
                        type="text"
                        bind:value={aiSettings.speech_language}
                        placeholder="zh / en / ja … 留空自动识别"
                        class="w-full rounded-lg bg-muted px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:ring-2 focus:ring-primary focus:outline-none"
                      />
                    </div>
                  </div>
                </div>

                <!-- Save Button -->
                <div class="flex items-center gap-3 pt-6">
                  <button
                    onclick={handleSaveAISettings}
                    disabled={aiSaving || !aiSettingsChanged}
                    class="flex items-center gap-2 rounded-lg bg-primary px-4 py-2 text-primary-foreground transition-colors duration-200 hover:bg-primary/90 disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    {#if aiSaving}
                      <svg class="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
                        <circle
                          class="opacity-25"
                          cx="12"
                          cy="12"
                          r="10"
                          stroke="currentColor"
                          stroke-width="4"
                        ></circle>
                        <path
                          class="opacity-75"
                          fill="currentColor"
                          d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                        ></path>
                      </svg>
                      保存中...
                    {:else}
                      保存设置
                    {/if}
                  </button>
                  {#if aiSuccess}
                    <span class="animate-fade-in flex items-center gap-1 text-sm text-green-600">
                      <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M5 13l4 4L19 7"
                        />
                      </svg>
                      已保存
                    </span>
                  {/if}
                </div>
              </div>
            {/if}

            {#if activeTab === 'ai-vectors'}
              <!-- 向量索引管理 -->
              <div
                id="ai-vectors"
                class="animate-fade-in scroll-mt-16 rounded-xl border border-border/50 bg-card p-6 shadow-sm"
              >
                <h2 class="mb-4 text-lg font-semibold text-foreground">向量索引</h2>
                <p class="mb-6 text-sm text-muted-foreground">
                  为日记条目构建嵌入向量索引，支持 AI
                  助手的语义搜索与智能分析。保存日记时会自动增量更新。
                </p>

                {#if buildError}
                  <div class="bg-destructive/10 text-destructive mb-4 rounded-lg p-3 text-sm">
                    {buildError}
                  </div>
                {/if}

                <!-- 索引状态 -->
                <div class="mb-6 space-y-3 rounded-lg bg-muted/40 p-4">
                  <div class="font-medium text-foreground">索引状态</div>
                  {#if loadingStats}
                    <div class="flex items-center gap-2 text-sm text-muted-foreground">
                      <svg class="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
                        <circle
                          class="opacity-25"
                          cx="12"
                          cy="12"
                          r="10"
                          stroke="currentColor"
                          stroke-width="4"
                        ></circle>
                        <path
                          class="opacity-75"
                          fill="currentColor"
                          d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                        ></path>
                      </svg>
                      加载中...
                    </div>
                  {:else if vectorStats}
                    <div class="space-y-3">
                      <!-- Segmented Progress Bar -->
                      <div class="space-y-2">
                        <div class="flex items-center justify-between text-sm">
                          <span class="text-muted-foreground">日记总数</span>
                          <span class="font-medium text-foreground">{vectorStats.diary_count}</span>
                        </div>
                        <div class="flex h-2 w-full overflow-hidden rounded-full bg-background">
                          {#if vectorStats.diary_count > 0}
                            {#if vectorStats.indexed_count > 0}
                              <div
                                class="h-2 bg-green-500 transition-all duration-300"
                                style="width: {(vectorStats.indexed_count /
                                  vectorStats.diary_count) *
                                  100}%"
                              ></div>
                            {/if}
                            {#if vectorStats.outdated_count > 0}
                              <div
                                class="h-2 bg-amber-500 transition-all duration-300"
                                style="width: {(vectorStats.outdated_count /
                                  vectorStats.diary_count) *
                                  100}%"
                              ></div>
                            {/if}
                            {#if vectorStats.pending_count > 0}
                              <div
                                class="h-2 bg-gray-400 transition-all duration-300"
                                style="width: {(vectorStats.pending_count /
                                  vectorStats.diary_count) *
                                  100}%"
                              ></div>
                            {/if}
                          {/if}
                        </div>
                      </div>

                      <!-- Stats Legend -->
                      <div class="flex flex-wrap gap-4 text-xs">
                        <div class="flex items-center gap-1.5">
                          <div class="h-2.5 w-2.5 rounded-full bg-green-500"></div>
                          <span class="text-muted-foreground"
                            >已索引 <span class="font-medium text-foreground"
                              >{vectorStats.indexed_count}</span
                            ></span
                          >
                        </div>
                        <div class="flex items-center gap-1.5">
                          <div class="h-2.5 w-2.5 rounded-full bg-amber-500"></div>
                          <span class="text-muted-foreground"
                            >过时 <span class="font-medium text-foreground"
                              >{vectorStats.outdated_count}</span
                            ></span
                          >
                        </div>
                        <div class="flex items-center gap-1.5">
                          <div class="h-2.5 w-2.5 rounded-full bg-gray-400"></div>
                          <span class="text-muted-foreground"
                            >待处理 <span class="font-medium text-foreground"
                              >{vectorStats.pending_count}</span
                            ></span
                          >
                        </div>
                      </div>

                      <!-- Status Message -->
                      {#if vectorStats.indexed_count === vectorStats.diary_count && vectorStats.diary_count > 0}
                        <div class="flex items-center gap-1 text-xs text-green-600">
                          <svg
                            class="h-3 w-3"
                            fill="none"
                            stroke="currentColor"
                            viewBox="0 0 24 24"
                          >
                            <path
                              stroke-linecap="round"
                              stroke-linejoin="round"
                              stroke-width="2"
                              d="M5 13l4 4L19 7"
                            />
                          </svg>
                          所有日记均已索引且为最新
                        </div>
                      {:else if vectorStats.outdated_count > 0 || vectorStats.pending_count > 0}
                        <div class="flex items-center gap-1 text-xs text-amber-600">
                          <svg
                            class="h-3 w-3"
                            fill="none"
                            stroke="currentColor"
                            viewBox="0 0 24 24"
                          >
                            <path
                              stroke-linecap="round"
                              stroke-linejoin="round"
                              stroke-width="2"
                              d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                            />
                          </svg>
                          {vectorStats.outdated_count + vectorStats.pending_count} 篇日记需要建立索引
                        </div>
                      {:else if vectorStats.diary_count === 0}
                        <div class="text-xs text-muted-foreground">暂无日记可索引</div>
                      {/if}
                    </div>
                  {:else}
                    <div class="text-sm text-muted-foreground">暂无索引数据</div>
                  {/if}
                </div>

                <!-- 操作按钮 -->
                <div class="space-y-4">
                  <div class="flex items-center gap-3">
                    <button
                      onclick={() => handleBuildVectors(true)}
                      disabled={buildingVectors}
                      class="flex items-center gap-2 rounded-lg bg-primary px-4 py-2 text-primary-foreground transition-colors duration-200 hover:bg-primary/90 disabled:opacity-50"
                      title="仅更新过时和新增的条目"
                    >
                      {#if buildingVectors}
                        <svg class="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
                          <circle
                            class="opacity-25"
                            cx="12"
                            cy="12"
                            r="10"
                            stroke="currentColor"
                            stroke-width="4"
                          ></circle>
                          <path
                            class="opacity-75"
                            fill="currentColor"
                            d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                          ></path>
                        </svg>
                        更新索引
                      {:else}
                        <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            stroke-width="2"
                            d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
                          />
                        </svg>
                        更新索引
                      {/if}
                    </button>
                    <button
                      onclick={() => handleBuildVectors(false)}
                      disabled={buildingVectors}
                      class="flex items-center gap-2 rounded-lg bg-muted px-4 py-2 transition-colors duration-200 hover:bg-muted/80 disabled:opacity-50"
                      title="从头重建所有条目"
                    >
                      <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                        />
                      </svg>
                      全部重建
                    </button>
                  </div>
                  <p class="text-xs text-muted-foreground">
                    "更新索引"仅处理过时和未索引的日记；"全部重建"会删除现有索引并从头构建，适合切换嵌入模型后使用。
                  </p>
                </div>

                <!-- 构建结果 -->
                {#if buildResult}
                  <div class="mt-4 rounded-lg bg-muted p-3 text-sm">
                    <div class="mb-2 font-medium text-foreground">构建完成</div>
                    <div class="space-y-1 text-muted-foreground">
                      <div>日记总数：{buildResult.total}</div>
                      <div class="text-green-600">成功：{buildResult.success}</div>
                      {#if buildResult.failed > 0}
                        <div class="text-destructive">失败：{buildResult.failed}</div>
                      {/if}
                    </div>
                    {#if buildResult.error_details && buildResult.error_details.length > 0}
                      <div class="mt-2 border-t border-border/50 pt-2">
                        <div class="text-destructive mb-1 font-medium">错误详情：</div>
                        <div
                          class="max-h-32 space-y-1 overflow-y-auto text-xs text-muted-foreground"
                        >
                          {#each buildResult.error_details as error (JSON.stringify(error))}
                            <div>{error}</div>
                          {/each}
                        </div>
                      </div>
                    {/if}
                  </div>
                {/if}
              </div>
            {/if}

            {#if activeTab === 'backup'}
              <div
                id="backup"
                class="animate-fade-in scroll-mt-16 rounded-xl border border-border/50 bg-card p-6 shadow-sm"
              >
                <h2 class="mb-1 text-lg font-semibold text-foreground">自动备份</h2>
                <p class="mb-6 text-sm text-muted-foreground">
                  按设定频率自动备份日记数据到服务器本地，并提供手动备份与历史记录管理。
                </p>

                {#if backupSettingsLoading}
                  <div class="flex items-center gap-2 text-sm text-muted-foreground">
                    <svg class="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
                      <circle
                        class="opacity-25"
                        cx="12"
                        cy="12"
                        r="10"
                        stroke="currentColor"
                        stroke-width="4"
                      ></circle>
                      <path
                        class="opacity-75"
                        fill="currentColor"
                        d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                      ></path>
                    </svg>
                    加载中...
                  </div>
                {:else}
                  <label class="mb-4 flex cursor-pointer items-center gap-3">
                    <input
                      type="checkbox"
                      bind:checked={backupSettingsLocal.enabled}
                      onchange={handleBackupSettingChange}
                      class="rounded"
                    />
                    <span class="text-sm text-foreground">启用自动备份</span>
                  </label>

                  {#if backupSettingsLocal.enabled}
                    <div class="mb-4 space-y-4 rounded-lg bg-muted/50 p-4">
                      <!-- 频率 -->
                      <div>
                        <label
                          for="backup-frequency"
                          class="mb-2 block text-sm font-medium text-foreground">备份频率</label
                        >
                        <div class="relative">
                          <select
                            id="backup-frequency"
                            bind:value={backupSettingsLocal.frequency}
                            onchange={handleBackupSettingChange}
                            class="w-full appearance-none rounded-lg border border-border/50 bg-background py-2 pr-9 pl-3 text-sm text-foreground focus:ring-2 focus:ring-primary focus:outline-none"
                          >
                            <option value="daily">每日</option>
                            <option value="weekly">每周</option>
                            <option value="monthly">每月</option>
                          </select>
                          <svg
                            class="pointer-events-none absolute top-1/2 right-3 h-4 w-4 -translate-y-1/2 text-muted-foreground"
                            fill="none"
                            stroke="currentColor"
                            viewBox="0 0 24 24"
                            aria-hidden="true"
                          >
                            <path
                              stroke-linecap="round"
                              stroke-linejoin="round"
                              stroke-width="2"
                              d="M19 9l-7 7-7-7"
                            />
                          </svg>
                        </div>
                      </div>

                      <!-- 时间 -->
                      <div>
                        <label
                          for="backup-time"
                          class="mb-2 block text-sm font-medium text-foreground">执行时间</label
                        >
                        <input
                          id="backup-time"
                          type="time"
                          bind:value={backupSettingsLocal.time}
                          onchange={handleBackupSettingChange}
                          class="w-full rounded-lg border border-border/50 bg-background px-3 py-2 text-sm text-foreground focus:ring-2 focus:ring-primary focus:outline-none"
                        />
                      </div>

                      <!-- 周几 (weekly) -->
                      {#if backupSettingsLocal.frequency === 'weekly'}
                        <div>
                          <label
                            for="backup-dow"
                            class="mb-2 block text-sm font-medium text-foreground">每周几</label
                          >
                          <div class="relative">
                            <select
                              id="backup-dow"
                              bind:value={backupSettingsLocal.day_of_week}
                              onchange={handleBackupSettingChange}
                              class="w-full appearance-none rounded-lg border border-border/50 bg-background py-2 pr-9 pl-3 text-sm text-foreground focus:ring-2 focus:ring-primary focus:outline-none"
                            >
                              <option value={1}>周一</option>
                              <option value={2}>周二</option>
                              <option value={3}>周三</option>
                              <option value={4}>周四</option>
                              <option value={5}>周五</option>
                              <option value={6}>周六</option>
                              <option value={0}>周日</option>
                            </select>
                            <svg
                              class="pointer-events-none absolute top-1/2 right-3 h-4 w-4 -translate-y-1/2 text-muted-foreground"
                              fill="none"
                              stroke="currentColor"
                              viewBox="0 0 24 24"
                              aria-hidden="true"
                            >
                              <path
                                stroke-linecap="round"
                                stroke-linejoin="round"
                                stroke-width="2"
                                d="M19 9l-7 7-7-7"
                              />
                            </svg>
                          </div>
                        </div>
                      {/if}

                      <!-- 几号 (monthly) -->
                      {#if backupSettingsLocal.frequency === 'monthly'}
                        <div>
                          <label
                            for="backup-dom"
                            class="mb-2 block text-sm font-medium text-foreground">每月几号</label
                          >
                          <input
                            id="backup-dom"
                            type="number"
                            min="1"
                            max="28"
                            bind:value={backupSettingsLocal.day_of_month}
                            onchange={handleBackupSettingChange}
                            class="w-full rounded-lg border border-border/50 bg-background px-3 py-2 text-sm text-foreground focus:ring-2 focus:ring-primary focus:outline-none"
                          />
                        </div>
                      {/if}

                      <!-- 保留天数 -->
                      <div>
                        <label
                          for="backup-retention"
                          class="mb-2 block text-sm font-medium text-foreground"
                          >保留天数（0 = 永久）</label
                        >
                        <input
                          id="backup-retention"
                          type="number"
                          min="0"
                          max="3650"
                          bind:value={backupSettingsLocal.retention_days}
                          onchange={handleBackupSettingChange}
                          class="w-full rounded-lg border border-border/50 bg-background px-3 py-2 text-sm text-foreground focus:ring-2 focus:ring-primary focus:outline-none"
                        />
                      </div>

                      <!-- S3 上传 -->
                      {#if imageUploadSettingsLocal.provider === 's3'}
                        <label class="flex cursor-pointer items-center gap-3">
                          <input
                            type="checkbox"
                            bind:checked={backupSettingsLocal.upload_s3}
                            onchange={handleBackupSettingChange}
                            class="rounded"
                          />
                          <span class="text-sm text-foreground">同时上传到 S3</span>
                        </label>
                      {/if}
                    </div>
                  {/if}
                {/if}

                {#if backupError}
                  <div class="bg-destructive/10 text-destructive mb-3 rounded-lg p-3 text-sm">
                    {backupError}
                  </div>
                {/if}
                {#if backupSuccess}
                  <div class="mb-3 rounded-lg bg-green-500/10 p-3 text-sm text-green-600">
                    {backupSuccess}
                  </div>
                {/if}

                <!-- 手动触发 -->
                <div class="mb-3 flex items-center gap-3">
                  <button
                    onclick={handleTriggerBackup}
                    disabled={backupTriggering}
                    class="flex items-center gap-2 rounded-lg bg-primary px-4 py-2 text-primary-foreground transition-colors duration-200 hover:bg-primary/90 disabled:opacity-50"
                  >
                    {#if backupTriggering}
                      <svg class="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
                        <circle
                          class="opacity-25"
                          cx="12"
                          cy="12"
                          r="10"
                          stroke="currentColor"
                          stroke-width="4"
                        ></circle>
                        <path
                          class="opacity-75"
                          fill="currentColor"
                          d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                        ></path>
                      </svg>
                      备份中...
                    {:else}
                      手动备份
                    {/if}
                  </button>
                </div>

                <!-- 备份记录 -->
                {#if backupRecords.length > 0}
                  <div class="mt-4">
                    <div class="mb-2 text-sm font-medium text-foreground">备份记录</div>
                    <div class="overflow-x-auto">
                      <table class="w-full text-sm">
                        <thead>
                          <tr class="border-b border-border/50 text-left text-muted-foreground">
                            <th class="pb-2 font-medium">文件名</th>
                            <th class="pb-2 font-medium">大小</th>
                            <th class="pb-2 font-medium">时间</th>
                            <th class="pb-2 font-medium">操作</th>
                          </tr>
                        </thead>
                        <tbody>
                          {#each backupRecords as record (record.id ?? record.filename)}
                            <tr class="border-b border-border/30">
                              <td class="max-w-[200px] truncate py-2 text-foreground"
                                >{record.filename}</td
                              >
                              <td class="py-2 text-muted-foreground"
                                >{formatFileSize(record.size)}</td
                              >
                              <td class="py-2 text-muted-foreground"
                                >{formatDate(record.created)}</td
                              >
                              <td class="py-2">
                                <div class="flex items-center gap-2">
                                  <button
                                    onclick={() => handleDownloadBackup(record.id)}
                                    class="text-xs text-primary hover:underline"
                                  >
                                    下载
                                  </button>
                                  <button
                                    onclick={() => handleDeleteBackup(record.id)}
                                    class="text-destructive text-xs hover:underline"
                                  >
                                    删除
                                  </button>
                                </div>
                              </td>
                            </tr>
                          {/each}
                        </tbody>
                      </table>
                    </div>

                    {#if backupTotalPages > 1}
                      <div class="mt-3 flex items-center justify-center gap-2">
                        <button
                          onclick={() => loadBackupRecords(backupCurrentPage - 1)}
                          disabled={backupCurrentPage <= 1}
                          class="rounded bg-muted px-3 py-1 text-xs hover:bg-muted/80 disabled:opacity-50"
                        >
                          上一页
                        </button>
                        <span class="text-xs text-muted-foreground"
                          >{backupCurrentPage} / {backupTotalPages}</span
                        >
                        <button
                          onclick={() => loadBackupRecords(backupCurrentPage + 1)}
                          disabled={backupCurrentPage >= backupTotalPages}
                          class="rounded bg-muted px-3 py-1 text-xs hover:bg-muted/80 disabled:opacity-50"
                        >
                          下一页
                        </button>
                      </div>
                    {/if}
                  </div>
                {:else if backupSettingsLocal.enabled}
                  <div class="mt-4 text-sm text-muted-foreground">暂无备份记录</div>
                {/if}
              </div>
            {/if}

            {#if activeTab === 'image-upload'}
              <!-- 图片上传 Section -->
              <div
                id="image-upload"
                class="animate-fade-in scroll-mt-16 rounded-xl border border-border/50 bg-card p-6 shadow-sm"
              >
                <h2 class="mb-4 text-lg font-semibold text-foreground">图片上传</h2>
                <p class="mb-6 text-sm text-muted-foreground">
                  选择日记图片的存储位置。切换提供方时，现有的本地、S3 和 Chevereto
                  设置将保留，以便迁移后仍可访问旧的媒体文件。
                </p>

                {#if imageUploadError}
                  <div class="bg-destructive/10 text-destructive mb-4 rounded-lg p-3 text-sm">
                    {imageUploadError}
                  </div>
                {/if}

                {#if imageUploadSuccess}
                  <div class="mb-4 rounded-lg bg-green-500/10 p-3 text-sm text-green-600">
                    {imageUploadSuccess}
                  </div>
                {/if}

                <div class="border-b border-border/50 py-4">
                  <div class="mb-3 font-medium text-foreground">存储提供商</div>
                  <div class="grid gap-3 md:grid-cols-3">
                    {#each [{ id: 'local', label: '本地存储', description: '将图片保存在磁盘上，并在内置媒体库中管理。' }, { id: 's3', label: 'S3 对象存储', description: '将媒体对象存储在 S3 兼容的对象存储服务中。' }, { id: 'chevereto', label: 'Chevereto', description: '上传图片到 Chevereto，并插入外部 URL。' }] as option (option.id)}
                      <button
                        type="button"
                        onclick={() =>
                          (imageUploadSettingsLocal.provider = option.id as ImageUploadProvider)}
                        class="rounded-xl border p-4 text-left transition-colors duration-200 {imageUploadSettingsLocal.provider ===
                        option.id
                          ? 'border-primary bg-primary/5'
                          : 'border-border/50 hover:border-border'}"
                      >
                        <div class="font-medium text-foreground">{option.label}</div>
                        <div class="mt-1 text-sm text-muted-foreground">{option.description}</div>
                      </button>
                    {/each}
                  </div>
                </div>

                {#if imageUploadSettingsLocal.provider === 'local'}
                  <div class="space-y-4 border-b border-border/50 py-4">
                    <div>
                      <label for="local-media-path" class="mb-2 block font-medium text-foreground"
                        >本地存储路径</label
                      >
                      <input
                        id="local-media-path"
                        type="text"
                        bind:value={imageUploadSettingsLocal.local.path}
                        placeholder="./diarum_data/storage/media"
                        class="w-full rounded-lg bg-muted px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:ring-2 focus:ring-primary focus:outline-none"
                      />
                      <p class="mt-1 text-xs text-muted-foreground">
                        默认迁移路径指向现有的吾身媒体存储目录。
                      </p>
                    </div>
                  </div>
                {:else if imageUploadSettingsLocal.provider === 's3'}
                  <div class="space-y-4 border-b border-border/50 py-4">
                    <div class="grid gap-4 md:grid-cols-2">
                      <div>
                        <label for="s3-bucket" class="mb-2 block font-medium text-foreground"
                          >Bucket 名称</label
                        >
                        <input
                          id="s3-bucket"
                          type="text"
                          bind:value={imageUploadSettingsLocal.s3.bucket}
                          class="w-full rounded-lg bg-muted px-3 py-2 text-sm text-foreground focus:ring-2 focus:ring-primary focus:outline-none"
                        />
                      </div>
                      <div>
                        <label for="s3-region" class="mb-2 block font-medium text-foreground"
                          >区域</label
                        >
                        <input
                          id="s3-region"
                          type="text"
                          bind:value={imageUploadSettingsLocal.s3.region}
                          class="w-full rounded-lg bg-muted px-3 py-2 text-sm text-foreground focus:ring-2 focus:ring-primary focus:outline-none"
                        />
                      </div>
                      <div>
                        <label for="s3-endpoint" class="mb-2 block font-medium text-foreground"
                          >Endpoint（可选）</label
                        >
                        <input
                          id="s3-endpoint"
                          type="text"
                          bind:value={imageUploadSettingsLocal.s3.endpoint}
                          placeholder="https://s3.amazonaws.com"
                          class="w-full rounded-lg bg-muted px-3 py-2 text-sm text-foreground focus:ring-2 focus:ring-primary focus:outline-none"
                        />
                      </div>
                      <div class="flex items-end">
                        <label class="inline-flex items-center gap-2 text-sm text-foreground">
                          <input
                            type="checkbox"
                            bind:checked={imageUploadSettingsLocal.s3.force_path_style}
                            class="rounded border-border text-primary focus:ring-primary"
                          />
                          使用路径样式请求
                        </label>
                      </div>
                    </div>
                    <div class="grid gap-4 md:grid-cols-2">
                      <div>
                        <label for="s3-access-key" class="mb-2 block font-medium text-foreground"
                          >访问密钥</label
                        >
                        <input
                          id="s3-access-key"
                          type="text"
                          bind:value={imageUploadSettingsLocal.s3.access_key}
                          class="w-full rounded-lg bg-muted px-3 py-2 text-sm text-foreground focus:ring-2 focus:ring-primary focus:outline-none"
                        />
                      </div>
                      <div>
                        <label for="s3-secret" class="mb-2 block font-medium text-foreground"
                          >秘密访问密钥</label
                        >
                        <input
                          id="s3-secret"
                          type="password"
                          bind:value={imageUploadSettingsLocal.s3.secret}
                          class="w-full rounded-lg bg-muted px-3 py-2 text-sm text-foreground focus:ring-2 focus:ring-primary focus:outline-none"
                        />
                      </div>
                    </div>
                    <p class="text-xs text-muted-foreground">
                      如果您是从 PocketBase S3
                      存储迁移而来，这些凭据也将用于保持旧相册图片的可访问性。
                    </p>
                  </div>
                {:else}
                  <div class="space-y-4 border-b border-border/50 py-4">
                    <div>
                      <label for="chevereto-domain" class="mb-2 block font-medium text-foreground"
                        >域名</label
                      >
                      <input
                        id="chevereto-domain"
                        type="text"
                        bind:value={imageUploadSettingsLocal.chevereto.domain}
                        placeholder="https://img.example.com"
                        class="w-full rounded-lg bg-muted px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:ring-2 focus:ring-primary focus:outline-none"
                      />
                    </div>
                    <div>
                      <label for="chevereto-api-key" class="mb-2 block font-medium text-foreground"
                        >API 密钥</label
                      >
                      <input
                        id="chevereto-api-key"
                        type="password"
                        bind:value={imageUploadSettingsLocal.chevereto.api_key}
                        placeholder="chv-key-..."
                        class="w-full rounded-lg bg-muted px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:ring-2 focus:ring-primary focus:outline-none"
                      />
                    </div>
                    <div>
                      <label for="chevereto-album-id" class="mb-2 block font-medium text-foreground"
                        >相册 ID（可选）</label
                      >
                      <input
                        id="chevereto-album-id"
                        type="text"
                        bind:value={imageUploadSettingsLocal.chevereto.album_id}
                        class="w-full rounded-lg bg-muted px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:ring-2 focus:ring-primary focus:outline-none"
                      />
                    </div>
                    <div class="flex items-center justify-between gap-4 rounded-lg bg-muted/40 p-4">
                      <div>
                        <div class="font-medium text-foreground">测试连接</div>
                        <div class="text-sm text-muted-foreground">
                          保存前请确认您的 Chevereto 服务器可以访问。
                        </div>
                      </div>
                      <button
                        onclick={handleTestChevereto}
                        disabled={cheveretoTesting || !canTestChevereto}
                        class="flex items-center gap-2 rounded-lg bg-background px-4 py-2 text-sm transition-colors duration-200 hover:bg-background/80 disabled:opacity-50"
                      >
                        {#if cheveretoTesting}
                          <svg class="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
                            <circle
                              class="opacity-25"
                              cx="12"
                              cy="12"
                              r="10"
                              stroke="currentColor"
                              stroke-width="4"
                            ></circle>
                            <path
                              class="opacity-75"
                              fill="currentColor"
                              d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                            ></path>
                          </svg>
                          测试中...
                        {:else}
                          测试
                        {/if}
                      </button>
                    </div>
                    {#if cheveretoTestResult}
                      <div
                        class="rounded-lg p-3 text-sm {cheveretoTestResult.success
                          ? 'bg-green-500/10 text-green-600'
                          : 'bg-destructive/10 text-destructive'}"
                      >
                        {cheveretoTestResult.message}
                      </div>
                    {/if}
                    <p class="text-xs text-muted-foreground">
                      Chevereto 上传会将外部图片 URL
                      插入日记内容。这些图片不会被内置媒体库追踪，也不会包含在导出文件中。
                    </p>
                  </div>
                {/if}

                <!-- Save Button -->
                <div class="flex items-center gap-3 pt-4">
                  <button
                    onclick={handleSaveImageUploadSettings}
                    disabled={imageUploadSaving || !imageUploadSettingsChanged}
                    class="flex items-center gap-2 rounded-lg bg-primary px-4 py-2 text-primary-foreground transition-colors duration-200 hover:bg-primary/90 disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    {#if imageUploadSaving}
                      <svg class="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
                        <circle
                          class="opacity-25"
                          cx="12"
                          cy="12"
                          r="10"
                          stroke="currentColor"
                          stroke-width="4"
                        ></circle>
                        <path
                          class="opacity-75"
                          fill="currentColor"
                          d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                        ></path>
                      </svg>
                      保存中...
                    {:else}
                      保存图片上传设置
                    {/if}
                  </button>
                  {#if imageUploadSuccess}
                    <span class="animate-fade-in flex items-center gap-1 text-sm text-green-600">
                      <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M5 13l4 4L19 7"
                        />
                      </svg>
                      已保存
                    </span>
                  {/if}
                </div>
              </div>
            {/if}

            {#if activeTab === 'data-management'}
              <!-- 数据管理 Section -->
              <div
                id="data-management"
                class="animate-fade-in scroll-mt-16 rounded-xl border border-border/50 bg-card p-6 shadow-sm"
              >
                <h2 class="mb-4 text-lg font-semibold text-foreground">数据管理</h2>
                <p class="mb-6 text-sm text-muted-foreground">
                  导入和导出您的日记数据。为避免导出文件过大，您可以按日期范围分段导出。
                </p>

                <!-- 导出 -->
                <div class="border-b border-border/50 py-4">
                  <div class="mb-1 flex items-center justify-between">
                    <div class="font-medium text-foreground">导出</div>
                    <button
                      onclick={() => (showExportOptions = !showExportOptions)}
                      class="text-xs text-primary hover:underline"
                    >
                      {showExportOptions ? '隐藏选项' : '显示选项'}
                    </button>
                  </div>
                  <div class="mb-3 text-sm text-muted-foreground">
                    将您的日记数据下载为 ZIP 文件
                  </div>

                  {#if showExportOptions}
                    <div class="mb-4 space-y-4 rounded-lg bg-muted/50 p-4">
                      <div class="rounded bg-amber-500/10 p-2 text-xs text-amber-600">
                        为避免导出文件过大，请考虑选择特定日期范围分段导出。
                      </div>

                      <!-- 日期范围 -->
                      <div>
                        <label
                          for="export-date-range"
                          class="mb-2 block text-sm font-medium text-foreground">日期范围</label
                        >
                        <div class="relative">
                          <select
                            id="export-date-range"
                            bind:value={exportOptions.date_range}
                            onchange={persistExportOptions}
                            class="w-full appearance-none rounded-lg border border-border/50 bg-background py-2 pr-9 pl-3 text-sm text-foreground focus:ring-2 focus:ring-primary focus:outline-none"
                          >
                            <option value="1m">过去 1 个月</option>
                            <option value="3m">过去 3 个月</option>
                            <option value="6m">过去 6 个月</option>
                            <option value="1y">过去 1 年</option>
                            <option value="all">全部时间</option>
                            <option value="custom">自定义范围</option>
                          </select>
                          <svg
                            class="pointer-events-none absolute top-1/2 right-3 h-4 w-4 -translate-y-1/2 text-muted-foreground"
                            fill="none"
                            stroke="currentColor"
                            viewBox="0 0 24 24"
                            aria-hidden="true"
                          >
                            <path
                              stroke-linecap="round"
                              stroke-linejoin="round"
                              stroke-width="2"
                              d="M19 9l-7 7-7-7"
                            />
                          </svg>
                        </div>
                      </div>

                      {#if exportOptions.date_range === 'custom'}
                        <div class="grid grid-cols-2 gap-3">
                          <div>
                            <label
                              for="export-start-date"
                              class="mb-1 block text-xs text-muted-foreground">开始日期</label
                            >
                            <input
                              id="export-start-date"
                              type="date"
                              bind:value={customStartDate}
                              class="w-full rounded-lg border border-border/50 bg-background px-3 py-2 text-sm text-foreground focus:ring-2 focus:ring-primary focus:outline-none"
                            />
                          </div>
                          <div>
                            <label
                              for="export-end-date"
                              class="mb-1 block text-xs text-muted-foreground">结束日期</label
                            >
                            <input
                              id="export-end-date"
                              type="date"
                              bind:value={customEndDate}
                              class="w-full rounded-lg border border-border/50 bg-background px-3 py-2 text-sm text-foreground focus:ring-2 focus:ring-primary focus:outline-none"
                            />
                          </div>
                        </div>
                      {/if}

                      <!-- 要导出的内容 -->
                      <div>
                        <div class="mb-2 block text-sm font-medium text-foreground">
                          要导出的内容
                        </div>
                        <div class="space-y-2">
                          <label class="flex cursor-pointer items-center gap-2">
                            <input
                              type="checkbox"
                              bind:checked={exportOptions.include_diaries}
                              onchange={persistExportOptions}
                              class="rounded"
                            />
                            <span class="text-sm text-foreground">日记</span>
                          </label>
                          <label class="flex cursor-pointer items-center gap-2">
                            <input
                              type="checkbox"
                              bind:checked={exportOptions.include_media}
                              onchange={persistExportOptions}
                              class="rounded"
                            />
                            <span class="text-sm text-foreground">媒体文件</span>
                          </label>
                          <label class="flex cursor-pointer items-center gap-2">
                            <input
                              type="checkbox"
                              bind:checked={exportOptions.include_analysis}
                              onchange={persistExportOptions}
                              class="rounded"
                            />
                            <span class="text-sm text-foreground">分析报告</span>
                          </label>
                          <label class="flex cursor-pointer items-center gap-2">
                            <input
                              type="checkbox"
                              bind:checked={exportOptions.include_conversations}
                              onchange={persistExportOptions}
                              class="rounded"
                            />
                            <span class="text-sm text-foreground">AI 对话</span>
                          </label>
                        </div>
                      </div>
                    </div>
                  {/if}

                  {#if exportError}
                    <div class="bg-destructive/10 text-destructive mb-3 rounded-lg p-3 text-sm">
                      {exportError}
                    </div>
                  {/if}

                  <button
                    onclick={handleExport}
                    disabled={exporting}
                    class="flex items-center gap-2 rounded-lg bg-primary px-4 py-2 text-primary-foreground transition-colors duration-200 hover:bg-primary/90 disabled:opacity-50"
                  >
                    {#if exporting}
                      <svg class="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
                        <circle
                          class="opacity-25"
                          cx="12"
                          cy="12"
                          r="10"
                          stroke="currentColor"
                          stroke-width="4"
                        ></circle>
                        <path
                          class="opacity-75"
                          fill="currentColor"
                          d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                        ></path>
                      </svg>
                      导出中...
                    {:else}
                      <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"
                        />
                      </svg>
                      导出数据
                    {/if}
                  </button>

                  {#if exportStats}
                    <div class="mt-3 rounded-lg bg-muted p-3 text-sm">
                      <div class="mb-2 font-medium text-foreground">导出完成</div>
                      <div class="mb-2 text-xs text-muted-foreground">
                        时段： {exportStats.start_date} 至 {exportStats.end_date}
                      </div>
                      <div class="space-y-2 text-muted-foreground">
                        <div class="flex justify-between">
                          <span>日记：</span>
                          <span>
                            <span class="font-medium text-foreground"
                              >{exportStats.diaries.actual_exported}</span
                            >
                            <span class="text-xs"
                              >/ {exportStats.diaries.should_export} 已选择 / {exportStats.diaries
                                .total_in_system} 总计</span
                            >
                          </span>
                        </div>
                        <div class="flex justify-between">
                          <span>媒体：</span>
                          <span>
                            <span class="font-medium text-foreground"
                              >{exportStats.media.actual_exported}</span
                            >
                            <span class="text-xs"
                              >/ {exportStats.media.should_export} 已选择 / {exportStats.media
                                .total_in_system} 总计</span
                            >
                          </span>
                        </div>
                        <div class="flex justify-between">
                          <span>对话：</span>
                          <span>
                            <span class="font-medium text-foreground"
                              >{exportStats.conversations.actual_exported}</span
                            >
                            <span class="text-xs"
                              >/ {exportStats.conversations.should_export} 已选择 / {exportStats
                                .conversations.total_in_system} 总计</span
                            >
                            <span class="text-xs">（{exportStats.messages} 条消息）</span>
                          </span>
                        </div>
                      </div>
                      {#if exportStats.failed_items && exportStats.failed_items.length > 0}
                        <div class="mt-3 border-t border-border/50 pt-2">
                          <div class="text-destructive mb-1 font-medium">失败项目：</div>
                          <div class="max-h-24 space-y-1 overflow-y-auto text-xs">
                            {#each exportStats.failed_items as item (JSON.stringify(item))}
                              <div class="text-muted-foreground">
                                <span class="text-destructive">[{item.type}]</span>
                                {item.id}: {item.reason}
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
                  <div class="mb-1 font-medium text-foreground">导入</div>
                  <div class="mb-3 text-sm text-muted-foreground">
                    从 JSON 或 ZIP 文件中恢复日记数据。支持导出的 diarum_export.json、ZIP 或包含多个
                    .md 文件的 ZIP。日期冲突时可选择保留哪个版本。
                  </div>

                  {#if importError}
                    <div class="bg-destructive/10 text-destructive mb-3 rounded-lg p-3 text-sm">
                      {importError}
                    </div>
                  {/if}

                  <div
                    class="rounded-lg border-2 border-dashed p-6 text-center transition-colors duration-200 {isDragOver
                      ? 'border-primary bg-primary/5'
                      : 'border-border/50 hover:border-primary/50'}"
                    ondragover={handleImportDragOver}
                    ondragleave={handleImportDragLeave}
                    ondrop={handleImportDrop}
                    role="region"
                    aria-label="拖放区域"
                  >
                    {#if importFile}
                      <div class="mb-3 flex items-center justify-center gap-2">
                        <svg
                          class="h-5 w-5 text-primary"
                          fill="none"
                          stroke="currentColor"
                          viewBox="0 0 24 24"
                        >
                          <path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            stroke-width="2"
                            d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
                          />
                        </svg>
                        <span class="text-sm font-medium text-foreground">{importFile.name}</span>
                        <button
                          onclick={() => {
                            importFile = null;
                            importStats = null;
                            importError = '';
                          }}
                          class="hover:text-destructive text-xs text-muted-foreground transition-colors"
                        >
                          移除
                        </button>
                      </div>
                    {:else}
                      <svg
                        class="mx-auto mb-2 h-8 w-8 text-muted-foreground"
                        fill="none"
                        stroke="currentColor"
                        viewBox="0 0 24 24"
                      >
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12"
                        />
                      </svg>
                      <p class="mb-1 text-sm text-muted-foreground">
                        拖放 .json 或 .zip 文件到此处，或
                      </p>
                      <label
                        class="inline-block cursor-pointer rounded-lg bg-muted px-4 py-2 text-sm transition-colors duration-200 hover:bg-muted/80"
                      >
                        <span>选择文件</span>
                        <input
                          type="file"
                          accept=".json,.zip"
                          class="hidden"
                          onchange={handleImportFileChange}
                        />
                      </label>
                    {/if}
                  </div>

                  <div class="mt-3 flex flex-wrap items-center gap-3">
                    <button
                      onclick={handleImport}
                      disabled={importing || !importFile}
                      class="flex items-center gap-2 rounded-lg bg-primary px-4 py-2 text-primary-foreground transition-colors duration-200 hover:bg-primary/90 disabled:opacity-50"
                    >
                      {#if importing}
                        <svg class="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
                          <circle
                            class="opacity-25"
                            cx="12"
                            cy="12"
                            r="10"
                            stroke="currentColor"
                            stroke-width="4"
                          ></circle>
                          <path
                            class="opacity-75"
                            fill="currentColor"
                            d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                          ></path>
                        </svg>
                        导入中...
                      {:else}
                        <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path
                            stroke-linecap="round"
                            stroke-linejoin="round"
                            stroke-width="2"
                            d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0l-4 4m4-4v12"
                          />
                        </svg>
                        导入
                      {/if}
                    </button>
                  </div>

                  {#if importStats}
                    <div class="mt-3 rounded-lg bg-muted p-3 text-sm">
                      <div class="mb-2 font-medium text-foreground">导入完成</div>
                      <div class="space-y-1 text-muted-foreground">
                        <div>
                          日记：
                          <span class="font-medium text-green-600"
                            >{importStats.diaries.imported} 已导入</span
                          >
                          {#if importStats.diaries.skipped > 0}
                            , <span class="font-medium text-amber-600"
                              >{importStats.diaries.skipped} 已跳过</span
                            >
                          {/if}
                          {#if importStats.diaries.conflict > 0}
                            , <span class="font-medium text-orange-500"
                              >{importStats.diaries.conflict} 日期冲突</span
                            >
                          {/if}
                          {#if importStats.diaries.failed > 0}
                            , <span class="text-destructive font-medium"
                              >{importStats.diaries.failed} 失败</span
                            >
                          {/if}
                          <span class="text-muted-foreground"
                            >（共 {importStats.diaries.total} 条）</span
                          >
                        </div>
                        <div>
                          媒体：
                          <span class="font-medium text-green-600"
                            >{importStats.media.imported} 已导入</span
                          >
                          {#if importStats.media.skipped > 0}
                            , <span class="font-medium text-amber-600"
                              >{importStats.media.skipped} 已跳过</span
                            >
                          {/if}
                          {#if importStats.media.failed > 0}
                            , <span class="text-destructive font-medium"
                              >{importStats.media.failed} 失败</span
                            >
                          {/if}
                          <span class="text-muted-foreground"
                            >（共 {importStats.media.total} 个）</span
                          >
                        </div>
                        <div>
                          AI 对话：
                          <span class="font-medium text-green-600"
                            >{importStats.conversations.imported} 已导入</span
                          >
                          {#if importStats.conversations.skipped > 0}
                            , <span class="font-medium text-orange-500"
                              >{importStats.conversations.skipped} 已跳过</span
                            >
                          {/if}
                          {#if importStats.conversations.failed > 0}
                            , <span class="text-destructive font-medium"
                              >{importStats.conversations.failed} 失败</span
                            >
                          {/if}
                          <span class="text-muted-foreground"
                            >（共 {importStats.conversations.total} 条）</span
                          >
                        </div>
                        {#if importStats.analyses}
                          <div>
                            AI 总结：
                            <span class="font-medium text-green-600"
                              >{importStats.analyses.imported} 已导入</span
                            >
                            {#if importStats.analyses.skipped > 0}
                              , <span class="font-medium text-amber-600"
                                >{importStats.analyses.skipped} 已跳过</span
                              >
                            {/if}
                            {#if importStats.analyses.conflict > 0}
                              , <span class="font-medium text-orange-500"
                                >{importStats.analyses.conflict} 已存在</span
                              >
                            {/if}
                            {#if importStats.analyses.failed > 0}
                              , <span class="text-destructive font-medium"
                                >{importStats.analyses.failed} 失败</span
                              >
                            {/if}
                            <span class="text-muted-foreground"
                              >（共 {importStats.analyses.total} 条）</span
                            >
                          </div>
                        {/if}
                      </div>

                      {#if importStats.diary_details && importStats.diary_details.length > 0}
                        {@const conflictDetails = importStats.diary_details.filter(
                          (d) => d.status === 'conflict'
                        )}
                        <div class="mt-3 border-t border-border pt-3">
                          <div class="mb-2 font-medium text-foreground">详细结果</div>
                          {#if conflictDetails.length > 0}
                            <div
                              class="mb-2 flex items-center gap-3 rounded-lg border border-orange-200 bg-orange-50 px-2 py-1.5 text-xs dark:border-orange-800 dark:bg-orange-950/20"
                            >
                              <label class="flex cursor-pointer items-center gap-1.5 select-none">
                                <input
                                  type="checkbox"
                                  checked={selectedConflicts.size === conflictDetails.length}
                                  onchange={() => toggleAllConflicts()}
                                  class="h-3.5 w-3.5 rounded border-orange-300 text-orange-500 focus:ring-orange-400"
                                />
                                <span class="text-orange-700 dark:text-orange-300">全选</span>
                              </label>
                              {#if selectedConflicts.size > 0}
                                <span class="text-orange-600 dark:text-orange-400"
                                  >已选 {selectedConflicts.size} 条</span
                                >
                              {/if}
                              <div class="flex-1"></div>
                              <button
                                onclick={() => handleBatchResolveConflict('keep_old')}
                                disabled={resolvingBatch || selectedConflicts.size === 0}
                                class="rounded border border-orange-300 bg-background px-3 py-1 text-xs font-medium text-foreground transition-colors hover:bg-orange-100 disabled:opacity-50 dark:border-orange-700 dark:hover:bg-orange-900/30"
                              >
                                全部保留
                              </button>
                              <button
                                onclick={() => handleBatchResolveConflict('replace')}
                                disabled={resolvingBatch || selectedConflicts.size === 0}
                                class="rounded bg-orange-500 px-3 py-1 text-xs font-medium text-white shadow-sm transition-colors hover:bg-orange-600 disabled:opacity-50"
                              >
                                全部替换
                              </button>
                              {#if resolvingBatch}
                                <span class="flex items-center gap-1 text-muted-foreground">
                                  <svg class="h-3 w-3 animate-spin" fill="none" viewBox="0 0 24 24"
                                    ><circle
                                      class="opacity-25"
                                      cx="12"
                                      cy="12"
                                      r="10"
                                      stroke="currentColor"
                                      stroke-width="4"
                                    ></circle><path
                                      class="opacity-75"
                                      fill="currentColor"
                                      d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                                    ></path></svg
                                  >
                                  处理中...
                                </span>
                              {/if}
                            </div>
                          {/if}
                          <div class="space-y-1">
                            {#each importStats.diary_details as detail (JSON.stringify(detail))}
                              {#if detail.status === 'imported'}
                                <div class="flex items-center gap-2 text-green-600">
                                  <svg
                                    class="h-4 w-4 shrink-0"
                                    fill="none"
                                    stroke="currentColor"
                                    viewBox="0 0 24 24"
                                    ><path
                                      stroke-linecap="round"
                                      stroke-linejoin="round"
                                      stroke-width="2"
                                      d="M5 13l4 4L19 7"
                                    /></svg
                                  >
                                  <span>{detail.date}</span>
                                  <span class="text-muted-foreground">已导入</span>
                                </div>
                              {:else if detail.status === 'skipped'}
                                <div class="flex items-center gap-2 text-amber-600">
                                  <svg
                                    class="h-4 w-4 shrink-0"
                                    fill="none"
                                    stroke="currentColor"
                                    viewBox="0 0 24 24"
                                    ><path
                                      stroke-linecap="round"
                                      stroke-linejoin="round"
                                      stroke-width="2"
                                      d="M12 9v2m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                                    /></svg
                                  >
                                  <span>{detail.date}</span>
                                  <span class="text-muted-foreground">已跳过（保留旧版本）</span>
                                </div>
                              {:else if detail.status === 'conflict'}
                                <div
                                  class="rounded-lg border border-orange-300 bg-orange-50 dark:border-orange-800 dark:bg-orange-950/20"
                                >
                                  <div class="flex items-center">
                                    <div class="shrink-0 pl-3">
                                      <input
                                        type="checkbox"
                                        checked={selectedConflicts.has(detail.date)}
                                        onchange={() => toggleConflictSelection(detail.date)}
                                        class="h-3.5 w-3.5 rounded border-orange-300 text-orange-500 focus:ring-orange-400"
                                      />
                                    </div>
                                    <button
                                      class="flex w-full items-center justify-between p-3 text-left"
                                      onclick={() =>
                                        (expandedConflictDate =
                                          expandedConflictDate === detail.date
                                            ? null
                                            : detail.date)}
                                    >
                                      <div class="flex items-center gap-2 text-orange-600">
                                        <svg
                                          class="h-4 w-4 shrink-0"
                                          fill="none"
                                          stroke="currentColor"
                                          viewBox="0 0 24 24"
                                          ><path
                                            stroke-linecap="round"
                                            stroke-linejoin="round"
                                            stroke-width="2"
                                            d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4"
                                          /></svg
                                        >
                                        <span class="font-medium">{detail.date}</span>
                                        <span class="text-muted-foreground">日期冲突</span>
                                        {#if hasContentChanged(detail.old_content, detail.new_content)}
                                          <span
                                            class="rounded bg-orange-200 px-1.5 py-0.5 text-[10px] text-orange-700 dark:bg-orange-800 dark:text-orange-300"
                                            >内容不同</span
                                          >
                                        {/if}
                                        {#if hasMoodChanged(detail.old_mood, detail.new_mood)}
                                          <span
                                            class="rounded bg-purple-200 px-1.5 py-0.5 text-[10px] text-purple-700 dark:bg-purple-800 dark:text-purple-300"
                                            >心情不同</span
                                          >
                                        {/if}
                                        {#if hasWeatherChanged(detail.old_weather, detail.new_weather)}
                                          <span
                                            class="rounded bg-blue-200 px-1.5 py-0.5 text-[10px] text-blue-700 dark:bg-blue-800 dark:text-blue-300"
                                            >天气不同</span
                                          >
                                        {/if}
                                      </div>
                                      <svg
                                        class="h-4 w-4 text-muted-foreground transition-transform {expandedConflictDate ===
                                        detail.date
                                          ? 'rotate-180'
                                          : ''}"
                                        fill="none"
                                        stroke="currentColor"
                                        viewBox="0 0 24 24"
                                        ><path
                                          stroke-linecap="round"
                                          stroke-linejoin="round"
                                          stroke-width="2"
                                          d="M19 9l-7 7-7-7"
                                        /></svg
                                      >
                                    </button>
                                  </div>
                                  {#if expandedConflictDate === detail.date}
                                    <div class="space-y-3 px-3 pb-3">
                                      <div
                                        class="flex items-center gap-2 border-t border-orange-200 pt-2 dark:border-orange-800"
                                      >
                                        <span class="text-xs text-muted-foreground">视图：</span>
                                        <button
                                          onclick={() => (conflictViewMode = 'diff')}
                                          class="rounded px-2 py-1 text-xs transition-colors {conflictViewMode ===
                                          'diff'
                                            ? 'bg-orange-500 text-white'
                                            : 'bg-muted hover:bg-muted/80'}"
                                        >
                                          差异对比
                                        </button>
                                        <button
                                          onclick={() => (conflictViewMode = 'side')}
                                          class="rounded px-2 py-1 text-xs transition-colors {conflictViewMode ===
                                          'side'
                                            ? 'bg-orange-500 text-white'
                                            : 'bg-muted hover:bg-muted/80'}"
                                        >
                                          并排查看
                                        </button>
                                      </div>

                                      {#if conflictViewMode === 'diff'}
                                        <div
                                          class="max-h-60 overflow-y-auto rounded border bg-background font-mono text-xs"
                                        >
                                          {#each computeLineDiff(detail.old_content || '', detail.new_content || '') as seg, si (si)}
                                            {#if seg.type === 'same'}
                                              <div class="px-2 py-0.5 whitespace-pre-wrap">
                                                {seg.text}
                                              </div>
                                            {:else if seg.type === 'removed'}
                                              <div
                                                class="border-l-2 border-red-400 bg-red-100 px-2 py-0.5 whitespace-pre-wrap text-red-700 dark:bg-red-950/40 dark:text-red-300"
                                              >
                                                - {seg.text}
                                              </div>
                                            {:else}
                                              <div
                                                class="border-l-2 border-green-400 bg-green-100 px-2 py-0.5 whitespace-pre-wrap text-green-700 dark:bg-green-950/40 dark:text-green-300"
                                              >
                                                + {seg.text}
                                              </div>
                                            {/if}
                                          {/each}
                                          {#if (detail.old_content || '') === (detail.new_content || '')}
                                            <div class="px-2 py-1 text-muted-foreground italic">
                                              内容相同
                                            </div>
                                          {/if}
                                        </div>
                                      {:else}
                                        <div class="grid grid-cols-2 gap-3">
                                          <div class="space-y-1">
                                            <div
                                              class="flex items-center gap-1 text-xs font-medium text-orange-600"
                                            >
                                              <svg
                                                class="h-3 w-3"
                                                fill="none"
                                                stroke="currentColor"
                                                viewBox="0 0 24 24"
                                                ><path
                                                  stroke-linecap="round"
                                                  stroke-linejoin="round"
                                                  stroke-width="2"
                                                  d="M12 9v2m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                                                /></svg
                                              >
                                              现有版本
                                            </div>
                                            <div
                                              class="max-h-48 overflow-y-auto rounded border bg-background p-2 text-xs whitespace-pre-wrap"
                                            >
                                              {detail.old_content || '（无内容）'}
                                            </div>
                                          </div>
                                          <div class="space-y-1">
                                            <div
                                              class="flex items-center gap-1 text-xs font-medium text-green-600"
                                            >
                                              <svg
                                                class="h-3 w-3"
                                                fill="none"
                                                stroke="currentColor"
                                                viewBox="0 0 24 24"
                                                ><path
                                                  stroke-linecap="round"
                                                  stroke-linejoin="round"
                                                  stroke-width="2"
                                                  d="M12 9v2m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                                                /></svg
                                              >
                                              导入版本
                                            </div>
                                            <div
                                              class="max-h-48 overflow-y-auto rounded border bg-background p-2 text-xs whitespace-pre-wrap"
                                            >
                                              {detail.new_content || '（无内容）'}
                                            </div>
                                          </div>
                                        </div>
                                      {/if}

                                      {#if hasMoodChanged(detail.old_mood, detail.new_mood) || hasWeatherChanged(detail.old_weather, detail.new_weather)}
                                        <div class="flex gap-4 text-xs">
                                          {#if hasMoodChanged(detail.old_mood, detail.new_mood)}
                                            <div class="flex items-center gap-2">
                                              <span class="text-muted-foreground">心情：</span>
                                              <span
                                                class="rounded bg-red-100 px-1.5 py-0.5 text-red-600 line-through dark:bg-red-950/40"
                                                >{#if detail.old_mood}<MoodIcon
                                                    mood={detail.old_mood}
                                                    size={14}
                                                  />{:else}无{/if}</span
                                              >
                                              <svg
                                                class="h-3 w-3 text-muted-foreground"
                                                fill="none"
                                                stroke="currentColor"
                                                viewBox="0 0 24 24"
                                                ><path
                                                  stroke-linecap="round"
                                                  stroke-linejoin="round"
                                                  stroke-width="2"
                                                  d="M13 7l5 5m0 0l-5 5m5-5H6"
                                                /></svg
                                              >
                                              <span
                                                class="rounded bg-green-100 px-1.5 py-0.5 text-green-600 dark:bg-green-950/40"
                                                >{#if detail.new_mood}<MoodIcon
                                                    mood={detail.new_mood}
                                                    size={14}
                                                  />{:else}无{/if}</span
                                              >
                                            </div>
                                          {/if}
                                          {#if hasWeatherChanged(detail.old_weather, detail.new_weather)}
                                            <div class="flex items-center gap-2">
                                              <span class="text-muted-foreground">天气：</span>
                                              <span
                                                class="rounded bg-red-100 px-1.5 py-0.5 text-red-600 line-through dark:bg-red-950/40"
                                                >{detail.old_weather || '无'}</span
                                              >
                                              <svg
                                                class="h-3 w-3 text-muted-foreground"
                                                fill="none"
                                                stroke="currentColor"
                                                viewBox="0 0 24 24"
                                                ><path
                                                  stroke-linecap="round"
                                                  stroke-linejoin="round"
                                                  stroke-width="2"
                                                  d="M13 7l5 5m0 0l-5 5m5-5H6"
                                                /></svg
                                              >
                                              <span
                                                class="rounded bg-green-100 px-1.5 py-0.5 text-green-600 dark:bg-green-950/40"
                                                >{detail.new_weather || '无'}</span
                                              >
                                            </div>
                                          {/if}
                                        </div>
                                      {/if}

                                      <div class="flex gap-2 pt-1">
                                        <button
                                          onclick={() => handleResolveConflict(detail, 'keep_old')}
                                          disabled={resolvingConflict}
                                          class="flex items-center gap-1.5 rounded-lg border border-orange-300 bg-background px-4 py-2 text-xs font-medium text-foreground transition-colors hover:bg-orange-100 disabled:opacity-50 dark:border-orange-700 dark:hover:bg-orange-900/30"
                                        >
                                          <svg
                                            class="h-3.5 w-3.5 text-orange-500"
                                            fill="none"
                                            stroke="currentColor"
                                            viewBox="0 0 24 24"
                                            ><path
                                              stroke-linecap="round"
                                              stroke-linejoin="round"
                                              stroke-width="2"
                                              d="M12 9v2m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                                            /></svg
                                          >
                                          保留现有版本
                                        </button>
                                        <button
                                          onclick={() => handleResolveConflict(detail, 'replace')}
                                          disabled={resolvingConflict}
                                          class="flex items-center gap-1.5 rounded-lg bg-orange-500 px-4 py-2 text-xs font-medium text-white shadow-sm transition-colors hover:bg-orange-600 disabled:opacity-50"
                                        >
                                          <svg
                                            class="h-3.5 w-3.5"
                                            fill="none"
                                            stroke="currentColor"
                                            viewBox="0 0 24 24"
                                            ><path
                                              stroke-linecap="round"
                                              stroke-linejoin="round"
                                              stroke-width="2"
                                              d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0l-4 4m4-4v12"
                                            /></svg
                                          >
                                          替换为导入版本
                                        </button>
                                        {#if resolvingConflict}
                                          <div
                                            class="flex items-center gap-1 text-xs text-muted-foreground"
                                          >
                                            <svg
                                              class="h-3 w-3 animate-spin"
                                              fill="none"
                                              viewBox="0 0 24 24"
                                              ><circle
                                                class="opacity-25"
                                                cx="12"
                                                cy="12"
                                                r="10"
                                                stroke="currentColor"
                                                stroke-width="4"
                                              ></circle><path
                                                class="opacity-75"
                                                fill="currentColor"
                                                d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                                              ></path></svg
                                            >
                                            处理中...
                                          </div>
                                        {/if}
                                      </div>
                                    </div>
                                  {/if}
                                </div>
                              {:else if detail.status === 'failed'}
                                <div class="text-destructive flex items-center gap-2">
                                  <svg
                                    class="h-4 w-4 shrink-0"
                                    fill="none"
                                    stroke="currentColor"
                                    viewBox="0 0 24 24"
                                    ><path
                                      stroke-linecap="round"
                                      stroke-linejoin="round"
                                      stroke-width="2"
                                      d="M6 18L18 6M6 6l12 12"
                                    /></svg
                                  >
                                  <span>{detail.date || '（无日期）'}</span>
                                  <span class="text-muted-foreground">失败：{detail.reason}</span>
                                </div>
                              {/if}
                            {/each}
                          </div>
                        </div>
                      {/if}
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
</div>
<!-- eslint-enable svelte/no-useless-mustaches -->

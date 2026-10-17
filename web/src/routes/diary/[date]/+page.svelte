<script lang="ts">
  import { resolve } from '$app/paths';
  import { page } from '$app/state';
  import { afterNavigate, goto } from '$app/navigation';
  import { onMount } from 'svelte';
  import TiptapEditor from '#lib/components/editor/TiptapEditor.svelte';
  import TableOfContents from '#lib/components/ui/TableOfContents.svelte';
  import TextPolisher from '#lib/components/TextPolisher.svelte';
  import CityPicker from '#lib/components/CityPicker.svelte';
  import WeatherDisplay from '#lib/components/WeatherDisplay.svelte';
  import {
    getAISettings,
    transcribeAudio,
    isSpeechConfigured,
    type AISettings
  } from '#lib/api/ai.js';
  import PageHeader from '#lib/components/ui/PageHeader.svelte';
  import DiaryShareModal from '#lib/components/share/DiaryShareModal.svelte';
  import DiaryVersionHistoryModal from '#lib/components/DiaryVersionHistoryModal.svelte';
  import { getDiaryByDate, getTagCloud } from '#lib/api/diaries.js';
  import { isAuthenticated, pb, type Diary } from '#lib/api/client.js';
  import { getDiaryEmojiSettings } from '#lib/api/settings.js';
  import { fetchWeather, fetchWeatherByCoords, type WeatherResult } from '#lib/api/weather.js';
  import type { CityInfo } from '#lib/types/city.js';
  import { WEATHER_CODES, manualWeatherToWMO } from '#lib/utils/weatherCodes.js';
  import {
    formatDisplayDate,
    getDayOfWeek,
    getPreviousDay,
    getNextDay,
    getToday,
    isToday
  } from '#lib/utils/date.js';
  import {
    updateLocalCache,
    updateFromServer,
    getCachedContent,
    forceSyncNow,
    hasDirtyCache,
    initDiaryCache,
    beginDiaryEditSession,
    clearCache
  } from '#lib/stores/diaryCache.js';
  import { onlineState } from '#lib/stores/onlineStatus.js';
  import { MOOD_SCALE, getMoodStatesForLevel, SCENARIO_OPTIONS } from '#lib/utils/diaryEmoji.js';
  import MoodIcon from '#lib/components/ui/MoodIcon.svelte';

  let weatherPresets: string[] = [];

  let content = '';
  let loading = true;
  let loadRequestId = 0;
  let showDrawer = false;
  let showDesktopToc = true;
  let showShareModal = false;
  let showPolisher = false;
  let showVersionHistory = false;
  let currentDiaryId = '';
  let polishSourceText = '';
  let selectedContent = '';
  let selectedMood: number = 0;
  let selectedMoodStates: string[] = [];
  let selectedScenarios: string[] = [];
  let selectedWeather = '';
  let selectedCity = '';
  let weatherData: WeatherResult | null = null;
  let isLoadingWeather = false;
  let showManualWeather = false;
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
  let micSupported =
    typeof window !== 'undefined' && !!(window as any).navigator?.mediaDevices?.getUserMedia;
  let wakeLock: WakeLockSentinel | null = null;
  let recordingMimeType = 'audio/webm';
  let editorFocused = false;
  // Snapshot taken on mousedown (before blur clears selectedContent)
  let shareSelectedContent = '';
  let shareOpenedByMouse = false;
  // 首次挂载时 Kit 已把当前路由的 params 写入 page，这里直接读取即可
  let date = page.params.date ?? getToday();
  let cacheReady = false;

  function captureShareSelection() {
    shareSelectedContent = selectedContent;
    shareOpenedByMouse = true;
  }

  async function acquireWakeLock() {
    if (!('wakeLock' in navigator)) return;
    if (wakeLock) return;
    try {
      const lock = await navigator.wakeLock.request('screen');
      lock.onrelease = () => {
        wakeLock = null;
        if (editorFocused || isRecording) {
          acquireWakeLock().catch(() => {});
        }
      };
      wakeLock = lock;
    } catch {
      // Wake lock not supported or denied
    }
  }

  async function releaseWakeLock() {
    if (wakeLock) {
      const lock = wakeLock;
      wakeLock = null;
      lock.onrelease = null;
      try {
        await lock.release();
      } catch {
        /* ignore */
      }
    }
  }

  // Keep screen awake when editor is focused or recording audio
  $: {
    if (editorFocused || isRecording) {
      acquireWakeLock();
    } else {
      releaseWakeLock();
    }
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
    const newTags = raw
      .split(/[,，]/)
      .map((s) => s.trim())
      .filter(Boolean);
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
    updateLocalCache(date, {
      content,
      mood: selectedMood,
      mood_states: selectedMoodStates,
      scenarios: selectedScenarios,
      weather: selectedWeather,
      city: selectedCity,
      temp_min: weatherData?.temp_min ?? 0,
      temp_max: weatherData?.temp_max ?? 0,
      tags
    });
  }

  function removeTag(tag: string) {
    tags = tags.filter((t) => t !== tag);
    updateLocalCache(date, {
      content,
      mood: selectedMood,
      mood_states: selectedMoodStates,
      scenarios: selectedScenarios,
      weather: selectedWeather,
      city: selectedCity,
      temp_min: weatherData?.temp_min ?? 0,
      temp_max: weatherData?.temp_max ?? 0,
      tags
    });
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
      .filter((t) => t.toLowerCase().includes(q) && !existingSet.has(t))
      .slice(0, 6);
    showTagSuggestions = tagSuggestions.length > 0;
    selectedSuggestionIndex = -1;
  }

  function applySuggestion(tag: string) {
    if (!tags.includes(tag)) {
      tags = [...tags, tag];
      updateLocalCache(date, {
        content,
        mood: selectedMood,
        mood_states: selectedMoodStates,
        scenarios: selectedScenarios,
        weather: selectedWeather,
        city: selectedCity,
        temp_min: weatherData?.temp_min ?? 0,
        temp_max: weatherData?.temp_max ?? 0,
        tags
      });
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
      allTags = result.map((t) => t.tag);
    } catch {
      allTags = [];
    }
  }

  // `page` 来自 $app/state，是 runes 响应式对象：在 legacy `$:` 中读取它不会建立依赖
  // （编译器只把裸引用 `page` 放进依赖函数，不会触发属性 getter），语句仅在初始化时执行一次，
  // 之后路由参数变化不再触发（Kit 文档：legacy 响应式语法不会反映 page 的变化）。
  // 因此「前一天 / 后一天」这类同路由参数切换需在导航完成后显式同步。
  afterNavigate(() => {
    date = page.params.date ?? getToday();
  });

  $: canGoNext = !isToday(date);

  function goToPreviousDay() {
    const prevDate = getPreviousDay(date);
    goto(resolve('/diary/[date]', { date: prevDate }));
  }

  function goToNextDay() {
    const currentDate = date;
    if (isToday(currentDate)) return;
    const nextDate = getNextDay(currentDate);
    goto(resolve('/diary/[date]', { date: nextDate }));
  }

  function goToCalendar() {
    const url = new URL(resolve('/diary'), window.location.origin);
    if (date) url.searchParams.set('date', date);
    // url 来自 window.location.origin 解析，pathname 已是 Kit 解析出的绝对站内路径，
    // 再套 resolve 会把 '/diary' 当成动态段参数处理。
    // eslint-disable-next-line svelte/no-navigation-without-resolve
    goto(url.pathname + url.search);
  }

  async function loadDiary(targetDate: string) {
    const currentRequestId = ++loadRequestId;
    const cached = getCachedContent(targetDate);

    // Keep unsynced local draft and skip server fetch.
    if (cached?.isDirty) {
      content = cached.content;
      currentDiaryId = '';
      selectedMood = cached.mood || 0;
      selectedMoodStates = cached.mood_states || [];
      selectedScenarios = cached.scenarios || [];
      selectedWeather = cached.weather || '';
      selectedCity = cached.city || '';
      tags = cached.tags || [];
      if (cached.weather) {
        weatherData = {
          city: cached.city || '未知',
          wmo_code: parseInt(cached.weather) || 0,
          temp_min: cached.temp_min,
          temp_max: cached.temp_max,
          date: targetDate
        };
      } else {
        weatherData = null;
      }
      loading = false;
      return;
    }

    content = '';
    currentDiaryId = '';
    selectedMood = 0;
    selectedMoodStates = [];
    selectedScenarios = [];
    selectedWeather = '';
    selectedCity = '';
    tags = [];

    // Browser cache is disabled; fetch current content from server.
    loading = true;
    try {
      const diary = await getDiaryByDate(targetDate);
      if (currentRequestId !== loadRequestId) return;
      updateFromServer(targetDate, diary);
      if (currentRequestId !== loadRequestId) return;
      currentDiaryId = diary?.id || '';
      content = diary?.content || '';
      selectedMood = diary?.mood || 0;
      selectedMoodStates = diary?.mood_states || [];
      selectedScenarios = diary?.scenarios || [];
      selectedWeather = diary?.weather || '';
      selectedCity = diary?.city || '';
      tags = diary?.tags || [];
      // Restore weatherData if weather info exists
      if (diary?.weather !== '' && diary?.weather != null) {
        weatherData = {
          city: selectedCity || '未知',
          wmo_code: parseInt(diary.weather) || 0,
          temp_min: diary.temp_min ?? 0,
          temp_max: diary.temp_max ?? 0,
          date: targetDate
        };
      } else {
        weatherData = null;
      }
    } catch (error) {
      console.error('Failed to load diary:', error);
      // Keep local draft on fetch failure if one exists.
      if (cached?.isDirty) {
        content = cached.content;
        selectedMood = cached.mood || 0;
        selectedMoodStates = cached.mood_states || [];
        selectedScenarios = cached.scenarios || [];
        selectedWeather = cached.weather || '';
        selectedCity = cached.city || '';
        tags = cached.tags || [];
      }
    }
    loading = false;

    // Auto-fetch weather if no city is selected but default city exists
    if (!selectedCity && !selectedWeather) {
      void autoFetchWeather();
    }
  }

  async function loadDiaryEmojiPresets() {
    try {
      const settings = await getDiaryEmojiSettings();
      weatherPresets = [...settings.weather_options];
    } catch (error) {
      console.error('Failed to load weather presets:', error);
    }
  }

  function handleContentChange(newContent: string) {
    content = newContent;
    updateLocalCache(date, {
      content,
      mood: selectedMood,
      mood_states: selectedMoodStates,
      scenarios: selectedScenarios,
      weather: selectedWeather,
      city: selectedCity,
      temp_min: weatherData?.temp_min ?? 0,
      temp_max: weatherData?.temp_max ?? 0,
      tags
    });
  }

  function handleMoodChange(mood: number) {
    selectedMood = selectedMood === mood ? 0 : mood;
    // Clear mood states when mood level changes (states are category-dependent)
    selectedMoodStates = [];
    updateLocalCache(date, {
      content,
      mood: selectedMood,
      mood_states: selectedMoodStates,
      scenarios: selectedScenarios,
      weather: selectedWeather,
      city: selectedCity,
      temp_min: weatherData?.temp_min ?? 0,
      temp_max: weatherData?.temp_max ?? 0,
      tags
    });
  }

  function handleMoodStateToggle(state: string) {
    if (selectedMoodStates.includes(state)) {
      selectedMoodStates = selectedMoodStates.filter((s) => s !== state);
    } else {
      selectedMoodStates = [...selectedMoodStates, state];
    }
    updateLocalCache(date, {
      content,
      mood: selectedMood,
      mood_states: selectedMoodStates,
      scenarios: selectedScenarios,
      weather: selectedWeather,
      city: selectedCity,
      temp_min: weatherData?.temp_min ?? 0,
      temp_max: weatherData?.temp_max ?? 0,
      tags
    });
  }

  function handleScenarioToggle(scenario: string) {
    if (selectedScenarios.includes(scenario)) {
      selectedScenarios = selectedScenarios.filter((s) => s !== scenario);
    } else {
      selectedScenarios = [...selectedScenarios, scenario];
    }
    updateLocalCache(date, {
      content,
      mood: selectedMood,
      mood_states: selectedMoodStates,
      scenarios: selectedScenarios,
      weather: selectedWeather,
      city: selectedCity,
      temp_min: weatherData?.temp_min ?? 0,
      temp_max: weatherData?.temp_max ?? 0,
      tags
    });
  }

  function handleWeatherSelect(code: string) {
    selectedWeather = selectedWeather === code ? '' : code;
    updateLocalCache(date, {
      content,
      mood: selectedMood,
      mood_states: selectedMoodStates,
      scenarios: selectedScenarios,
      weather: selectedWeather,
      city: selectedCity,
      temp_min: weatherData?.temp_min ?? 0,
      temp_max: weatherData?.temp_max ?? 0,
      tags
    });
  }

  async function handleCitySelect(city: CityInfo) {
    selectedCity = city.name;
    isLoadingWeather = true;
    try {
      // Use coordinates if available (from geolocation), otherwise search by name
      let result;
      if (city.lat && city.lon) {
        result = await fetchWeatherByCoords(city.name, city.lat, city.lon, date);
      } else {
        result = await fetchWeather(city.name, date);
      }
      weatherData = result;
      selectedWeather = String(result.wmo_code);
      updateLocalCache(date, {
        content,
        mood: selectedMood,
        mood_states: selectedMoodStates,
        scenarios: selectedScenarios,
        weather: selectedWeather,
        city: selectedCity,
        temp_min: result.temp_min,
        temp_max: result.temp_max,
        tags
      });
    } catch (error) {
      console.error('Failed to fetch weather:', error);
    } finally {
      isLoadingWeather = false;
    }
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

      // Use audio/mp4 on iOS Safari (audio/webm is not supported)
      const isIOS =
        /iPad|iPhone|iPod/.test(navigator.userAgent) ||
        (navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1);
      recordingMimeType =
        isIOS && MediaRecorder.isTypeSupported('audio/mp4') ? 'audio/mp4' : 'audio/webm';
      const mr = new MediaRecorder(stream, { mimeType: recordingMimeType });
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
    const audioBlob = new Blob(recordedChunks, { type: recordingMimeType });
    if (audioBlob.size < 512) {
      // Too short — likely just silence / click
      speechError = '录音时间过短，请重新尝试';
      setTimeout(() => {
        speechError = '';
      }, 2500);
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
        setTimeout(() => {
          speechError = '';
        }, 2500);
      }
    } catch (e) {
      speechError = e instanceof Error ? e.message : '语音识别失败';
      setTimeout(() => {
        speechError = '';
      }, 3000);
    } finally {
      isTranscribing = false;
    }
  }

  function formatRecordingTime(seconds: number): string {
    const m = Math.floor(seconds / 60)
      .toString()
      .padStart(2, '0');
    const s = Math.floor(seconds % 60)
      .toString()
      .padStart(2, '0');
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
    polishSourceText = selectedContent && selectedContent.trim() !== '' ? selectedContent : content;
    showPolisher = true;
  }

  async function handleOpenVersionHistory() {
    if (!currentDiaryId) {
      // 本地草稿分支未记录 id，打开时按日期补查（日记不存在则显示空态）
      try {
        const diary = await getDiaryByDate(date);
        currentDiaryId = diary?.id || '';
      } catch {
        currentDiaryId = '';
      }
    }
    showVersionHistory = true;
  }

  function handleVersionRestored(diary: Diary) {
    // 恢复结果以服务端为准：丢弃本地草稿，避免未同步内容把恢复结果再次覆盖
    clearCache(date);
    currentDiaryId = diary.id || currentDiaryId;
    content = diary.content || '';
    selectedMood = diary.mood || 0;
    selectedMoodStates = diary.mood_states || [];
    selectedScenarios = diary.scenarios || [];
    selectedWeather = diary.weather || '';
    selectedCity = diary.city || '';
    tags = diary.tags || [];
    if (diary.weather) {
      weatherData = {
        city: selectedCity || '未知',
        wmo_code: parseInt(diary.weather) || 0,
        temp_min: diary.temp_min ?? 0,
        temp_max: diary.temp_max ?? 0,
        date
      };
    } else {
      weatherData = null;
    }
    updateFromServer(date, diary);
  }

  function handleApplyPolished(text: string) {
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
  let defaultCity = '';

  async function loadDefaultCity() {
    try {
      const response = await fetch('/api/v1/settings/weather.default_city', {
        headers: { Authorization: `Bearer ${pb.authStore.token}` }
      });
      if (response.ok) {
        const data = await response.json();
        defaultCity = data.value || '';
      }
    } catch (e) {
      // Ignore error
    }
    // Trigger auto-fetch after default city is loaded (fixes race with loadDiary)
    if (defaultCity && !selectedCity && !selectedWeather) {
      await autoFetchWeather();
    }
  }

  async function autoFetchWeather() {
    // If no city selected and default city is set, auto-fetch
    if (!selectedCity && defaultCity && !weatherData) {
      let cityName = defaultCity;
      let cityLat = 0;
      let cityLon = 0;
      try {
        const parsed = JSON.parse(defaultCity);
        if (parsed.name) {
          cityName = parsed.name;
          cityLat = parsed.lat ?? 0;
          cityLon = parsed.lon ?? 0;
        }
      } catch {
        // plain string city name (backward compatible)
      }
      selectedCity = cityName;
      await handleCitySelect({
        name: cityName,
        lat: cityLat,
        lon: cityLon,
        province: '',
        country: ''
      });
    }
  }

  onMount(() => {
    if (!$isAuthenticated) {
      goto(resolve('/login'));
      return;
    }

    // Initialize diary cache (includes online status)
    initDiaryCache();
    cacheReady = true;
    void loadDiaryEmojiPresets();
    void loadSpeechSettings();
    void loadAllTags();
    void loadDefaultCity();

    window.addEventListener('keydown', handleKeyboard);

    const onVisibilityChange = () => {
      if (document.visibilityState === 'visible' && (editorFocused || isRecording)) {
        acquireWakeLock().catch(() => {});
      }
    };
    document.addEventListener('visibilitychange', onVisibilityChange);

    return () => {
      window.removeEventListener('keydown', handleKeyboard);
      document.removeEventListener('visibilitychange', onVisibilityChange);
      releaseWakeLock();
      if (mediaRecorder && mediaRecorder.state !== 'inactive') {
        try {
          mediaRecorder.stop();
        } catch {
          /* ignore */
        }
      }
      stopRecordingTimer();
    };
  });

  // Load diary only in browser (not during SSR)
  $: if (cacheReady && date && date !== previousDate && typeof window !== 'undefined') {
    previousDate = date;
    loadDiary(date);
  }

  // 断网重连后自动刷新：离线时编辑页天气为空，重新联网后应自动重新加载
  // 日记与天气，而不需要用户手动刷新页面或切换日期。
  let wasOffline = false;
  $: if ($onlineState.isOnline && wasOffline && cacheReady && typeof window !== 'undefined') {
    wasOffline = false;
    if (hasDirtyCache(date)) {
      // 本地有未同步编辑：保留用户内容，仅尝试补充天气
      if (selectedCity && !weatherData) {
        void handleCitySelect({ name: selectedCity, lat: 0, lon: 0, province: '', country: '' });
      } else if (!selectedCity) {
        void autoFetchWeather();
      }
    } else {
      void loadDiary(date);
    }
  }
  $: if (!$onlineState.isOnline) {
    wasOffline = true;
  }
</script>

<svelte:head>
  <title>{formatDisplayDate(date)} - 吾身</title>
</svelte:head>

<div class="flex min-h-[100dvh] min-h-screen flex-col bg-background">
  <PageHeader title="日记" />
  <!-- Main Content -->
  <div class="container-responsive flex flex-1 flex-col py-6">
    <!-- 主内容布局：导航条与编辑器同宽 -->
    <div
      class="diary-layout mx-auto flex min-h-0 flex-1 gap-6 transition-all duration-300"
      class:with-desktop-sidebar={showDesktopToc}
    >
      <main id="main-content" tabindex="-1" class="diary-main flex min-h-0 w-full min-w-0 flex-col">
        <!-- 页面标题 - 视觉隐藏，用于屏幕阅读器的文档结构 -->
        <h1 class="sr-only">日记：{formatDisplayDate(date)} — 编辑与记录</h1>
        <!-- 日期导航：紧贴编辑器上方，宽度随编辑器一致 -->
        <div class="mb-4">
          <div
            class="flex items-center rounded-xl border border-border/50 bg-card px-3 py-2.5 shadow-sm transition-shadow hover:shadow-md"
          >
            <button
              type="button"
              onclick={goToPreviousDay}
              class="flex items-center gap-1 rounded-lg px-3 py-1.5 text-sm text-foreground/80 transition-all duration-200 hover:bg-muted/50 hover:text-foreground"
              title="上一天"
            >
              <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M15 19l-7-7 7-7"
                />
              </svg>
              <span>前一天</span>
            </button>

            <div class="flex min-w-0 flex-1 items-center justify-center gap-2">
              <button
                type="button"
                onclick={goToCalendar}
                class="text-sm font-semibold text-foreground transition-opacity hover:opacity-80"
                title="返回日历"
              >
                {formatDisplayDate(date)}
              </button>
              {#if isToday(date)}
                <span
                  class="rounded-full bg-primary/15 px-1.5 py-0.5 text-[10px] font-medium text-primary"
                  >今天</span
                >
              {/if}
              <span class="text-[11px] text-muted-foreground sm:hidden">周{getDayOfWeek(date)}</span
              >
              <span class="hidden text-[11px] text-muted-foreground sm:inline"
                >星期{getDayOfWeek(date)}</span
              >
            </div>

            <button
              type="button"
              onclick={goToNextDay}
              class="flex items-center gap-1 rounded-lg px-3 py-1.5 text-sm text-foreground/80 transition-all duration-200 hover:bg-muted/50 hover:text-foreground disabled:cursor-not-allowed disabled:opacity-30"
              disabled={!canGoNext}
              title="下一天"
            >
              <span>后一天</span>
              <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M9 5l7 7-7 7"
                />
              </svg>
            </button>
          </div>
        </div>
        {#if loading}
          <div class="animate-fade-in flex flex-col items-center justify-center gap-3 py-20">
            <svg class="h-6 w-6 animate-spin text-primary" fill="none" viewBox="0 0 24 24"
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
            <div class="text-sm text-muted-foreground">加载中...</div>
          </div>
        {:else}
          <div
            class="animate-fade-in relative flex min-h-0 flex-1 flex-col overflow-hidden rounded-xl border border-border/50 bg-card shadow-sm ring-1 ring-border/20"
          >
            <TiptapEditor
              {content}
              bind:selectedContent
              onChange={handleContentChange}
              placeholder="今天有什么想说的？"
              emptyStatePrompt="✨ 回顾今天... 这一天你会记住什么？"
              diaryDate={date}
              onFocusChange={(focused: boolean) => {
                editorFocused = focused;
                if (focused) {
                  // 每次进入编辑框开启新的编辑会话，会话内只产生一个版本
                  beginDiaryEditSession(date);
                }
              }}
            />
            {#if speechEnabled || isRecording || isTranscribing}
              <div class="absolute right-3 bottom-3 z-10 flex items-center gap-2">
                {#if speechError}
                  <div
                    class="bg-destructive/90 text-destructive-foreground rounded-lg px-3 py-1.5 text-xs shadow-md"
                  >
                    {speechError}
                  </div>
                {/if}
                {#if isRecording}
                  <button
                    type="button"
                    onclick={stopRecording}
                    title="停止录音并转文字"
                    class="bg-destructive/90 hover:bg-destructive text-destructive-foreground shadow-destructive/20 inline-flex items-center gap-2 rounded-full px-3 py-2 text-sm font-medium shadow-lg transition-all"
                  >
                    <span class="relative flex h-3 w-3 items-center justify-center">
                      <span
                        class="absolute inline-flex h-full w-full animate-ping rounded-full bg-white/70 opacity-75"
                      ></span>
                      <span class="relative inline-flex h-2 w-2 rounded-full bg-white"></span>
                    </span>
                    <span>停止录音</span>
                    <span class="text-xs text-white/80 tabular-nums"
                      >{formatRecordingTime(recordingSeconds)}</span
                    >
                  </button>
                {:else if isTranscribing}
                  <button
                    type="button"
                    disabled
                    class="inline-flex items-center gap-2 rounded-full bg-primary/90 px-3 py-2 text-sm font-medium text-primary-foreground shadow-lg shadow-primary/20"
                  >
                    <svg
                      class="h-4 w-4 animate-spin"
                      fill="none"
                      stroke="currentColor"
                      viewBox="0 0 24 24"
                      aria-hidden="true"
                    >
                      <circle
                        class="opacity-25"
                        cx="12"
                        cy="12"
                        r="10"
                        stroke-width="4"
                        stroke="currentColor"
                      />
                      <path
                        class="opacity-75"
                        fill="currentColor"
                        stroke="currentColor"
                        stroke-width="4"
                        d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                      />
                    </svg>
                    <span>正在转写</span>
                  </button>
                {:else}
                  <button
                    type="button"
                    onclick={startRecording}
                    title="语音输入日记"
                    class="inline-flex items-center gap-2 rounded-full bg-primary/90 px-3 py-2 text-sm font-medium text-primary-foreground shadow-lg shadow-primary/20 transition-all hover:bg-primary"
                  >
                    <svg
                      class="h-4 w-4"
                      fill="none"
                      stroke="currentColor"
                      viewBox="0 0 24 24"
                      aria-hidden="true"
                    >
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M12 1a3 3 0 00-3 3v5a3 3 0 006 0V4a3 3 0 00-3-3z"
                      />
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M19 10v1a7 7 0 01-14 0v-1M12 18v3M8 21h8"
                      />
                    </svg>
                    <span>语音输入</span>
                  </button>
                {/if}
              </div>
            {/if}
          </div>

          <!-- Mobile: Mood, Weather, Tags panel below editor -->
          <div class="animate-slide-up mt-4 space-y-3 lg:hidden">
            <!-- AI 整理入口 -->
            <button
              type="button"
              onclick={handleOpenPolisher}
              class="group flex w-full items-center gap-2 rounded-xl border border-border/50 bg-card p-3 text-left shadow-sm transition-all hover:bg-card/80"
            >
              <div
                class="rounded-md bg-primary/10 p-1.5 text-primary transition-colors group-hover:bg-primary/20"
              >
                <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0018 18.469V19a2 2 0 01-2 2H8a2 2 0 01-2-2v-.531c0-.895.357-1.753.988-2.357l-.547-.546z"
                  />
                </svg>
              </div>
              <div>
                <div class="text-xs font-semibold text-foreground">AI 整理文本</div>
                <div class="text-[11px] text-muted-foreground">去语气词 · 纠错 · 自动分段</div>
              </div>
            </button>

            <!-- 历史版本入口 -->
            <button
              type="button"
              onclick={handleOpenVersionHistory}
              class="group flex w-full items-center gap-2 rounded-xl border border-border/50 bg-card p-3 text-left shadow-sm transition-all hover:bg-card/80"
            >
              <div
                class="rounded-md bg-primary/10 p-1.5 text-primary transition-colors group-hover:bg-primary/20"
              >
                <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
                  />
                </svg>
              </div>
              <div>
                <div class="text-xs font-semibold text-foreground">历史版本</div>
                <div class="text-[11px] text-muted-foreground">查看并恢复编辑前的内容</div>
              </div>
            </button>

            <!-- Mood -->
            <div class="rounded-xl border border-border/50 bg-card p-4 shadow-sm">
              <div class="mb-3 flex items-center justify-between">
                <div class="text-sm font-semibold text-foreground">心情</div>
                {#if selectedMood > 0}
                  <button
                    onclick={() => handleMoodChange(0)}
                    class="rounded-full border border-border/70 bg-muted/70 px-2 py-1 text-[11px] text-muted-foreground transition-colors hover:bg-muted"
                  >
                    清除
                  </button>
                {/if}
              </div>
              <div class="mood-slider-container">
                <div class="mood-slider-track">
                  {#each MOOD_SCALE as level (level.value)}
                    <button
                      onclick={() => handleMoodChange(level.value)}
                      class="mood-slider-stop {selectedMood === level.value
                        ? 'mood-slider-stop-active'
                        : ''} {selectedMood > 0 && level.value <= selectedMood
                        ? 'mood-slider-stop-filled'
                        : ''}"
                      title={level.label}
                      aria-label={`心情 ${level.label}`}
                    >
                      <span class="mood-slider-emoji"
                        ><MoodIcon mood={level.value} size={24} /></span
                      >
                    </button>
                  {/each}
                </div>
                {#if selectedMood > 0}
                  <div
                    class="mt-2 flex items-center justify-center gap-1 text-center text-xs text-muted-foreground"
                  >
                    <MoodIcon mood={selectedMood} size={20} />
                    <span>{MOOD_SCALE.find((l) => l.value === selectedMood)?.label || ''}</span>
                  </div>
                {/if}
              </div>
              {#if selectedMood > 0}
                {@const availableStates = getMoodStatesForLevel(selectedMood)}
                {#if availableStates.length > 0}
                  <div class="mt-3 border-t border-border/50 pt-3">
                    <div class="mb-2 text-[11px] text-muted-foreground">心情状态（可多选）</div>
                    <div class="flex flex-wrap gap-1.5">
                      {#each availableStates as state (state)}
                        <button
                          onclick={() => handleMoodStateToggle(state)}
                          class="mood-state-chip {selectedMoodStates.includes(state)
                            ? 'mood-state-chip-active'
                            : ''}"
                        >
                          {state}
                        </button>
                      {/each}
                    </div>
                  </div>
                {/if}
              {/if}
            </div>

            <!-- Scenarios -->
            <div class="rounded-xl border border-border/50 bg-card p-4 shadow-sm">
              <div class="mb-2 flex items-center justify-between">
                <div class="text-sm font-semibold text-foreground">情景</div>
                {#if selectedScenarios.length > 0}
                  <button
                    onclick={() => {
                      selectedScenarios = [];
                      updateLocalCache(date, {
                        content,
                        mood: selectedMood,
                        mood_states: selectedMoodStates,
                        scenarios: [],
                        weather: selectedWeather,
                        city: selectedCity,
                        temp_min: weatherData?.temp_min ?? 0,
                        temp_max: weatherData?.temp_max ?? 0,
                        tags
                      });
                    }}
                    class="rounded-full border border-border/70 bg-muted/70 px-2 py-1 text-[11px] text-muted-foreground transition-colors hover:bg-muted"
                  >
                    清除
                  </button>
                {/if}
              </div>
              <div class="flex flex-wrap gap-1.5">
                {#each SCENARIO_OPTIONS as scenario (scenario)}
                  <button
                    onclick={() => handleScenarioToggle(scenario)}
                    class="mood-state-chip {selectedScenarios.includes(scenario)
                      ? 'mood-state-chip-active'
                      : ''}"
                  >
                    {scenario}
                  </button>
                {/each}
              </div>
            </div>

            <!-- Weather -->
            <div class="rounded-xl border border-border/50 bg-card p-4 shadow-sm">
              <div class="mb-2 flex items-center justify-between">
                <div class="text-sm font-semibold text-foreground">天气</div>
                {#if selectedWeather || selectedCity}
                  <button
                    onclick={() => {
                      selectedWeather = '';
                      selectedCity = '';
                      weatherData = null;
                      updateLocalCache(date, {
                        content,
                        mood: selectedMood,
                        mood_states: selectedMoodStates,
                        scenarios: selectedScenarios,
                        weather: '',
                        tags,
                        city: '',
                        temp_min: 0,
                        temp_max: 0
                      });
                    }}
                    class="rounded-full border border-border/70 bg-muted/70 px-2 py-1 text-[11px] text-muted-foreground transition-colors hover:bg-muted"
                  >
                    清除
                  </button>
                {/if}
              </div>
              <!-- City Picker -->
              <div class="mb-3">
                <CityPicker {selectedCity} onCitySelect={handleCitySelect} />
              </div>
              <!-- Weather Display -->
              {#if isLoadingWeather}
                <div class="flex items-center justify-center py-4">
                  <svg
                    class="h-5 w-5 animate-spin text-muted-foreground"
                    fill="none"
                    viewBox="0 0 24 24"
                  >
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
                  <span class="ml-2 text-xs text-muted-foreground">获取天气中...</span>
                </div>
              {:else if weatherData}
                <div class="flex items-center justify-center py-2">
                  <WeatherDisplay
                    wmoCode={weatherData.wmo_code}
                    tempMin={weatherData.temp_min}
                    tempMax={weatherData.temp_max}
                    size="md"
                  />
                </div>
              {:else if selectedWeather}
                <div class="flex items-center justify-center py-2">
                  <WeatherDisplay wmoCode={parseInt(selectedWeather)} size="md" showTemp={false} />
                </div>
              {:else}
                <div class="py-2 text-center text-xs text-muted-foreground">选择城市获取天气</div>
              {/if}

              <!-- Manual Weather Selection Toggle -->
              <div class="mt-2 border-t border-border/30 pt-2">
                <button
                  type="button"
                  onclick={() => (showManualWeather = !showManualWeather)}
                  class="text-[10px] text-muted-foreground transition-colors hover:text-foreground"
                >
                  {showManualWeather ? '收起' : '手动选择天气'}
                </button>
              </div>

              <!-- Manual Weather Selection Grid -->
              {#if showManualWeather}
                <div class="mt-2 grid grid-cols-4 gap-1.5">
                  {#each Object.values(WEATHER_CODES) as wmo (wmo.code)}
                    <button
                      type="button"
                      onclick={() => {
                        selectedWeather = String(manualWeatherToWMO(wmo.code));
                        if (weatherData) {
                          weatherData = { ...weatherData, wmo_code: manualWeatherToWMO(wmo.code) };
                        }
                        updateLocalCache(date, {
                          content,
                          mood: selectedMood,
                          mood_states: selectedMoodStates,
                          scenarios: selectedScenarios,
                          weather: selectedWeather,
                          city: selectedCity,
                          temp_min: weatherData?.temp_min ?? 0,
                          temp_max: weatherData?.temp_max ?? 0,
                          tags
                        });
                      }}
                      class="flex flex-col items-center rounded-lg p-1.5 text-[10px] transition-colors {selectedWeather ===
                      String(manualWeatherToWMO(wmo.code))
                        ? 'bg-primary/20 text-primary'
                        : 'bg-muted/30 text-muted-foreground hover:bg-muted/50'}"
                      title={wmo.label}
                    >
                      <span class="text-sm leading-none">{wmo.icon}</span>
                      <span class="mt-0.5 w-full truncate text-center">{wmo.label}</span>
                    </button>
                  {/each}
                </div>
              {/if}
            </div>

            <!-- Tags -->
            <div class="rounded-xl border border-border/50 bg-card p-4 shadow-sm">
              <div class="mb-3 flex items-center justify-between">
                <div class="text-sm font-semibold text-foreground">标签</div>
                {#if tags.length > 0}
                  <span class="text-[10px] text-muted-foreground">
                    {tags.length} 个
                  </span>
                {/if}
              </div>
              <div class="mb-3 flex flex-wrap gap-1.5">
                {#each tags as tag (tag)}
                  <span
                    class="group inline-flex items-center gap-1 rounded-full border border-primary/20 bg-primary/10 px-2 py-0.5 text-xs text-primary transition-colors hover:bg-primary/15"
                  >
                    {tag}
                    <button
                      type="button"
                      onclick={() => removeTag(tag)}
                      class="hover:text-destructive flex-shrink-0 opacity-60 transition-opacity hover:opacity-100"
                      aria-label={`移除标签 ${tag}`}
                    >
                      <svg class="h-3 w-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2.5"
                          d="M6 18L18 6M6 6l12 12"
                        />
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
                  oninput={handleTagInput}
                  onkeydown={handleTagKeydown}
                  onfocus={() => updateTagSuggestions(tagInput)}
                  onblur={hideTagSuggestions}
                  placeholder="添加标签，按回车或逗号确认"
                  class="w-full rounded-lg border border-border/60 bg-muted/30 px-3 py-2 text-xs transition-colors placeholder:text-muted-foreground/50 focus:border-primary focus:ring-1 focus:ring-primary/30 focus:outline-none"
                />
                {#if showTagSuggestions && tagSuggestions.length > 0}
                  <div
                    class="absolute top-full right-0 left-0 z-50 mt-1 max-h-40 overflow-y-auto rounded-lg border border-border/50 bg-card shadow-lg"
                  >
                    {#each tagSuggestions as suggestion, i (i)}
                      <button
                        type="button"
                        onmousedown={(e) => {
                          e.preventDefault();
                          applySuggestion(suggestion);
                        }}
                        class="w-full px-3 py-1.5 text-left text-xs transition-colors hover:bg-muted/50 {i ===
                        selectedSuggestionIndex
                          ? 'bg-muted/50 text-foreground'
                          : 'text-muted-foreground'}"
                      >
                        <span class="font-medium text-foreground"
                          >{suggestion.slice(0, tagInput.trim().length)}</span
                        ><span>{suggestion.slice(tagInput.trim().length)}</span>
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
        <aside class="hidden w-[19rem] flex-shrink-0 lg:block">
          <div class="animate-slide-in-right sticky top-11 space-y-3">
            <button
              type="button"
              onclick={handleOpenPolisher}
              class="group flex w-full items-center gap-2 rounded-xl border border-border/50 bg-card/50 p-3 text-left shadow-sm transition-all hover:border-primary/30 hover:bg-card"
            >
              <div
                class="rounded-md bg-primary/10 p-1.5 text-primary transition-colors group-hover:bg-primary/20"
              >
                <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0018 18.469V19a2 2 0 01-2 2H8a2 2 0 01-2-2v-.531c0-.895.357-1.753.988-2.357l-.547-.546z"
                  />
                </svg>
              </div>
              <div class="min-w-0">
                <div class="text-xs font-semibold text-foreground">AI 整理文本</div>
                <div class="truncate text-[11px] text-muted-foreground">去语气词 · 纠错 · 重组</div>
              </div>
            </button>

            <button
              type="button"
              onclick={handleOpenVersionHistory}
              class="group flex w-full items-center gap-2 rounded-xl border border-border/50 bg-card/50 p-3 text-left shadow-sm transition-all hover:border-primary/30 hover:bg-card"
            >
              <div
                class="rounded-md bg-primary/10 p-1.5 text-primary transition-colors group-hover:bg-primary/20"
              >
                <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
                  />
                </svg>
              </div>
              <div class="min-w-0">
                <div class="text-xs font-semibold text-foreground">历史版本</div>
                <div class="truncate text-[11px] text-muted-foreground">查看并恢复编辑前的内容</div>
              </div>
            </button>

            <div class="rounded-xl border border-border/50 bg-card/50 p-4 shadow-sm">
              <div class="mb-3 flex items-center justify-between">
                <div>
                  <div class="text-sm font-semibold text-foreground">心情</div>
                </div>
                {#if selectedMood > 0}
                  <button
                    onclick={() => handleMoodChange(0)}
                    class="rounded-full border border-border/70 bg-background/70 px-2 py-1 text-[11px] transition-colors hover:bg-background"
                  >
                    清除
                  </button>
                {/if}
              </div>
              <div class="mood-slider-container">
                <div class="mood-slider-track">
                  {#each MOOD_SCALE as level (level.value)}
                    <button
                      onclick={() => handleMoodChange(level.value)}
                      class="mood-slider-stop {selectedMood === level.value
                        ? 'mood-slider-stop-active'
                        : ''} {selectedMood > 0 && level.value <= selectedMood
                        ? 'mood-slider-stop-filled'
                        : ''}"
                      title={level.label}
                      aria-label={`心情 ${level.label}`}
                    >
                      <span class="mood-slider-emoji"
                        ><MoodIcon mood={level.value} size={24} /></span
                      >
                    </button>
                  {/each}
                </div>
                {#if selectedMood > 0}
                  <div
                    class="mt-2 flex items-center justify-center gap-1 text-center text-xs text-muted-foreground"
                  >
                    <MoodIcon mood={selectedMood} size={20} />
                    <span>{MOOD_SCALE.find((l) => l.value === selectedMood)?.label || ''}</span>
                  </div>
                {/if}
              </div>
              {#if selectedMood > 0}
                {@const availableStates = getMoodStatesForLevel(selectedMood)}
                {#if availableStates.length > 0}
                  <div class="mt-3 border-t border-border/50 pt-3">
                    <div class="mb-2 text-[11px] text-muted-foreground">心情状态（可多选）</div>
                    <div class="flex flex-wrap gap-1.5">
                      {#each availableStates as state (state)}
                        <button
                          onclick={() => handleMoodStateToggle(state)}
                          class="mood-state-chip {selectedMoodStates.includes(state)
                            ? 'mood-state-chip-active'
                            : ''}"
                        >
                          {state}
                        </button>
                      {/each}
                    </div>
                  </div>
                {/if}
              {/if}
            </div>

            <!-- Scenarios -->
            <div class="rounded-xl border border-border/50 bg-card/50 p-4 shadow-sm">
              <div class="mb-2 flex items-center justify-between">
                <div class="text-sm font-semibold text-foreground">情景</div>
                {#if selectedScenarios.length > 0}
                  <button
                    onclick={() => {
                      selectedScenarios = [];
                      updateLocalCache(date, {
                        content,
                        mood: selectedMood,
                        mood_states: selectedMoodStates,
                        scenarios: [],
                        weather: selectedWeather,
                        city: selectedCity,
                        temp_min: weatherData?.temp_min ?? 0,
                        temp_max: weatherData?.temp_max ?? 0,
                        tags
                      });
                    }}
                    class="rounded-full border border-border/70 bg-background/70 px-2 py-1 text-[11px] transition-colors hover:bg-background"
                  >
                    清除
                  </button>
                {/if}
              </div>
              <div class="flex flex-wrap gap-1.5">
                {#each SCENARIO_OPTIONS as scenario (scenario)}
                  <button
                    onclick={() => handleScenarioToggle(scenario)}
                    class="mood-state-chip {selectedScenarios.includes(scenario)
                      ? 'mood-state-chip-active'
                      : ''}"
                  >
                    {scenario}
                  </button>
                {/each}
              </div>
            </div>

            <div class="rounded-xl border border-border/50 bg-card/50 p-4 shadow-sm">
              <div class="mb-2 flex items-center justify-between">
                <div>
                  <div class="text-sm font-semibold text-foreground">天气</div>
                </div>
                {#if selectedWeather || selectedCity}
                  <button
                    onclick={() => {
                      selectedWeather = '';
                      selectedCity = '';
                      weatherData = null;
                      updateLocalCache(date, {
                        content,
                        mood: selectedMood,
                        mood_states: selectedMoodStates,
                        scenarios: selectedScenarios,
                        weather: '',
                        tags,
                        city: '',
                        temp_min: 0,
                        temp_max: 0
                      });
                    }}
                    class="rounded-full border border-border/70 bg-background/70 px-2 py-1 text-[11px] transition-colors hover:bg-background"
                  >
                    清除
                  </button>
                {/if}
              </div>
              <!-- City Picker -->
              <div class="mb-3">
                <CityPicker {selectedCity} onCitySelect={handleCitySelect} />
              </div>
              <!-- Weather Display -->
              {#if isLoadingWeather}
                <div class="flex items-center justify-center py-4">
                  <svg
                    class="h-5 w-5 animate-spin text-muted-foreground"
                    fill="none"
                    viewBox="0 0 24 24"
                  >
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
                  <span class="ml-2 text-xs text-muted-foreground">获取天气中...</span>
                </div>
              {:else if weatherData}
                <div class="flex items-center justify-center py-2">
                  <WeatherDisplay
                    wmoCode={weatherData.wmo_code}
                    tempMin={weatherData.temp_min}
                    tempMax={weatherData.temp_max}
                    size="md"
                  />
                </div>
              {:else if selectedWeather}
                <div class="flex items-center justify-center py-2">
                  <WeatherDisplay wmoCode={parseInt(selectedWeather)} size="md" showTemp={false} />
                </div>
              {:else}
                <div class="py-2 text-center text-xs text-muted-foreground">选择城市获取天气</div>
              {/if}

              <!-- Manual Weather Selection Toggle -->
              <div class="mt-2 border-t border-border/30 pt-2">
                <button
                  type="button"
                  onclick={() => (showManualWeather = !showManualWeather)}
                  class="text-[10px] text-muted-foreground transition-colors hover:text-foreground"
                >
                  {showManualWeather ? '收起' : '手动选择天气'}
                </button>
              </div>

              <!-- Manual Weather Selection Grid -->
              {#if showManualWeather}
                <div class="mt-2 grid grid-cols-4 gap-1.5">
                  {#each Object.values(WEATHER_CODES) as wmo (wmo.code)}
                    <button
                      type="button"
                      onclick={() => {
                        selectedWeather = String(manualWeatherToWMO(wmo.code));
                        if (weatherData) {
                          weatherData = { ...weatherData, wmo_code: manualWeatherToWMO(wmo.code) };
                        }
                        updateLocalCache(date, {
                          content,
                          mood: selectedMood,
                          mood_states: selectedMoodStates,
                          scenarios: selectedScenarios,
                          weather: selectedWeather,
                          city: selectedCity,
                          temp_min: weatherData?.temp_min ?? 0,
                          temp_max: weatherData?.temp_max ?? 0,
                          tags
                        });
                      }}
                      class="flex flex-col items-center rounded-lg p-1.5 text-[10px] transition-colors {selectedWeather ===
                      String(manualWeatherToWMO(wmo.code))
                        ? 'bg-primary/20 text-primary'
                        : 'bg-muted/30 text-muted-foreground hover:bg-muted/50'}"
                      title={wmo.label}
                    >
                      <span class="text-sm leading-none">{wmo.icon}</span>
                      <span class="mt-0.5 w-full truncate text-center">{wmo.label}</span>
                    </button>
                  {/each}
                </div>
              {/if}
            </div>

            <div class="rounded-xl border border-border/50 bg-card/50 p-4">
              <TableOfContents
                {content}
                {tags}
                tagInputValue={tagInput}
                onTagInput={(v) => {
                  tagInput = v;
                  updateTagSuggestions(v);
                }}
                onTagRemove={removeTag}
                onTagKeydown={handleTagKeydown}
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
</div>

<!-- Left Drawer -->
{#if showDrawer}
  <!-- Backdrop -->
  <button
    class="fixed inset-0 z-40 bg-black/40 backdrop-blur-sm lg:hidden"
    onclick={() => (showDrawer = false)}
    aria-label="关闭菜单"
  ></button>

  <!-- Drawer Panel -->
  <div
    class="animate-slide-in-left fixed inset-y-0 left-0 z-50 w-72 border-r border-border bg-card shadow-2xl lg:hidden"
  >
    <!-- Drawer Header -->
    <div class="flex items-center justify-between border-b border-border/50 px-5 py-4">
      <div class="flex items-center gap-2">
        <img src="/logo.png" alt="吾身" class="h-6 w-6" />
        <span class="font-semibold text-foreground">菜单</span>
      </div>
      <button
        onclick={() => (showDrawer = false)}
        class="rounded-lg p-2 transition-colors hover:bg-muted"
        aria-label="关闭"
      >
        <svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M6 18L18 6M6 6l12 12"
          />
        </svg>
      </button>
    </div>

    <!-- Drawer Content -->
    <div class="flex h-[calc(100%-57px)] flex-col">
      <!-- Actions Section -->
      <div class="px-3 py-3">
        <div
          class="mb-2 px-1 text-[10px] font-semibold tracking-wider text-muted-foreground uppercase"
        >
          快捷操作
        </div>
        <div class="space-y-0.5">
          <a
            href={resolve('/assistant')}
            class="group flex items-center gap-2.5 rounded-lg px-2 py-1.5 transition-all duration-200 hover:bg-muted/70"
            onclick={() => (showDrawer = false)}
          >
            <div
              class="rounded-md bg-primary/10 p-1.5 text-primary transition-colors group-hover:bg-primary/20"
            >
              <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <rect x="4" y="6" width="16" height="12" rx="2" stroke-width="2" />
                <circle cx="9" cy="11" r="1.5" fill="currentColor" />
                <circle cx="15" cy="11" r="1.5" fill="currentColor" />
              </svg>
            </div>
            <div class="min-w-0">
              <div class="text-xs font-medium text-foreground">AI 助手</div>
              <div class="truncate text-[10px] text-muted-foreground">与 AI 聊聊你的日记</div>
            </div>
          </a>

          <button
            type="button"
            onclick={() => {
              showDrawer = false;
              handleOpenPolisher();
            }}
            class="group flex w-full items-center gap-2.5 rounded-lg px-2 py-1.5 text-left transition-all duration-200 hover:bg-muted/70"
          >
            <div
              class="rounded-md bg-purple-500/10 p-1.5 text-purple-500 transition-colors group-hover:bg-purple-500/20"
            >
              <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0018 18.469V19a2 2 0 01-2 2H8a2 2 0 01-2-2v-.531c0-.895.357-1.753.988-2.357l-.547-.546z"
                />
              </svg>
            </div>
            <div class="min-w-0 text-left">
              <div class="text-xs font-medium text-foreground">AI 整理文本</div>
              <div class="truncate text-[10px] text-muted-foreground">去语气词 · 纠错 · 重组</div>
            </div>
          </button>

          <button
            onmousedown={captureShareSelection}
            onclick={() => {
              showDrawer = false;
              openShareModal();
            }}
            class="group flex w-full items-center gap-2.5 rounded-lg px-2 py-1.5 transition-all duration-200 hover:bg-muted/70"
          >
            <div
              class="rounded-md bg-blue-500/10 p-1.5 text-blue-500 transition-colors group-hover:bg-blue-500/20"
            >
              <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M8.684 13.342C8.886 12.938 9 12.482 9 12c0-.482-.114-.938-.316-1.342m0 2.684a3 3 0 110-2.684m0 2.684l6.632 3.316m-6.632-6l6.632-3.316m0 0a3 3 0 105.367-2.684 3 3 0 00-5.367 2.684zm0 9.316a3 3 0 105.368 2.684 3 3 0 00-5.368-2.684z"
                />
              </svg>
            </div>
            <div class="min-w-0 text-left">
              <div class="text-xs font-medium text-foreground">分享</div>
              <div class="truncate text-[10px] text-muted-foreground">导出为精美图片</div>
            </div>
          </button>

          <a
            href={resolve('/diary')}
            class="group flex items-center gap-2.5 rounded-lg px-2 py-1.5 transition-all duration-200 hover:bg-muted/70"
            onclick={() => (showDrawer = false)}
          >
            <div
              class="rounded-md bg-green-500/10 p-1.5 text-green-500 transition-colors group-hover:bg-green-500/20"
            >
              <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z"
                />
              </svg>
            </div>
            <div class="min-w-0">
              <div class="text-xs font-medium text-foreground">日历</div>
              <div class="truncate text-[10px] text-muted-foreground">查看所有日记条目</div>
            </div>
          </a>

          <a
            href={resolve('/tags')}
            class="group flex items-center gap-2.5 rounded-lg px-2 py-1.5 transition-all duration-200 hover:bg-muted/70"
            onclick={() => (showDrawer = false)}
          >
            <div
              class="rounded-md bg-purple-500/10 p-1.5 text-purple-500 transition-colors group-hover:bg-purple-500/20"
            >
              <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M7 7h.01M7 3h5a1.99 1.99 0 011.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A1.99 1.99 0 013 12V7a4 4 0 014-4z"
                />
              </svg>
            </div>
            <div class="min-w-0">
              <div class="text-xs font-medium text-foreground">标签云</div>
              <div class="truncate text-[10px] text-muted-foreground">浏览你的全部标签</div>
            </div>
          </a>

          <a
            href={resolve('/filter')}
            class="group flex items-center gap-2.5 rounded-lg px-2 py-1.5 transition-all duration-200 hover:bg-muted/70"
            onclick={() => (showDrawer = false)}
          >
            <div
              class="rounded-md bg-amber-500/10 p-1.5 text-amber-500 transition-colors group-hover:bg-amber-500/20"
            >
              <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M3 4a1 1 0 011-1h16a1 1 0 011 1v2.586a1 1 0 01-.293.707l-6.414 6.414a1 1 0 00-.293.707V17l-4 4v-6.586a1 1 0 00-.293-.707L3.293 7.293A1 1 0 013 6.586V4z"
                />
              </svg>
            </div>
            <div class="min-w-0">
              <div class="text-xs font-medium text-foreground">心情筛选</div>
              <div class="truncate text-[10px] text-muted-foreground">按心情和情景筛选</div>
            </div>
          </a>

          <a
            href={resolve('/settings')}
            class="group flex items-center gap-2.5 rounded-lg px-2 py-1.5 transition-all duration-200 hover:bg-muted/70"
            onclick={() => (showDrawer = false)}
          >
            <div
              class="rounded-md bg-gray-500/10 p-1.5 text-gray-500 transition-colors group-hover:bg-gray-500/20"
            >
              <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"
                />
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
                />
              </svg>
            </div>
            <div class="min-w-0">
              <div class="text-xs font-medium text-foreground">设置</div>
              <div class="truncate text-[10px] text-muted-foreground">偏好与同步</div>
            </div>
          </a>
        </div>
      </div>

      <!-- Divider -->
      <div class="mx-3 border-t border-border/50"></div>

      <!-- Mood & Weather -->
      <div class="space-y-3 border-b border-border/50 px-3 py-3">
        <div>
          <div
            class="mb-2 px-1 text-[10px] font-semibold tracking-wider text-muted-foreground uppercase"
          >
            心情
          </div>
          <div class="mood-slider-container">
            <div class="mood-slider-track">
              {#each MOOD_SCALE as level (level.value)}
                <button
                  onclick={() => handleMoodChange(level.value)}
                  class="mood-slider-stop {selectedMood === level.value
                    ? 'mood-slider-stop-active'
                    : ''} {selectedMood > 0 && level.value <= selectedMood
                    ? 'mood-slider-stop-filled'
                    : ''}"
                  title={level.label}
                  aria-label={`心情 ${level.label}`}
                >
                  <span class="mood-slider-emoji"><MoodIcon mood={level.value} size={24} /></span>
                </button>
              {/each}
            </div>
          </div>
          {#if selectedMood > 0}
            {@const availableStates = getMoodStatesForLevel(selectedMood)}
            {#if availableStates.length > 0}
              <div class="mt-2">
                <div class="mb-1.5 px-1 text-[10px] text-muted-foreground">心情状态（可多选）</div>
                <div class="flex flex-wrap gap-1">
                  {#each availableStates as state (state)}
                    <button
                      onclick={() => handleMoodStateToggle(state)}
                      class="mood-state-chip text-[11px] {selectedMoodStates.includes(state)
                        ? 'mood-state-chip-active'
                        : ''}"
                    >
                      {state}
                    </button>
                  {/each}
                </div>
              </div>
            {/if}
          {/if}
        </div>

        <div>
          <div
            class="mb-2 px-1 text-[10px] font-semibold tracking-wider text-muted-foreground uppercase"
          >
            情景
          </div>
          <div class="flex flex-wrap gap-1">
            {#each SCENARIO_OPTIONS as scenario (scenario)}
              <button
                onclick={() => handleScenarioToggle(scenario)}
                class="mood-state-chip text-[11px] {selectedScenarios.includes(scenario)
                  ? 'mood-state-chip-active'
                  : ''}"
              >
                {scenario}
              </button>
            {/each}
          </div>
        </div>

        <div>
          <div
            class="mb-2 px-1 text-[10px] font-semibold tracking-wider text-muted-foreground uppercase"
          >
            天气
          </div>
          <div class="grid grid-cols-4 gap-1.5">
            {#each weatherPresets as option (option)}
              <button
                onclick={() => handleWeatherSelect(option)}
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
          {tags}
          tagInputValue={tagInput}
          onTagInput={(v) => {
            tagInput = v;
            updateTagSuggestions(v);
          }}
          onTagRemove={removeTag}
          onTagKeydown={handleTagKeydown}
          onNavigate={() => (showDrawer = false)}
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
  onClose={() => (showShareModal = false)}
/>

<!-- History Version Modal -->
<DiaryVersionHistoryModal
  bind:open={showVersionHistory}
  diaryId={currentDiaryId}
  {date}
  onRestored={handleVersionRestored}
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
    transition:
      transform,
      background-color,
      border-color,
      opacity 0.2s ease;
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

  .mood-state-chip {
    display: inline-flex;
    align-items: center;
    padding: 0.2rem 0.5rem;
    border-radius: 9999px;
    border: 1px solid hsl(var(--border) / 0.5);
    background: hsl(var(--muted) / 0.3);
    font-size: 0.7rem;
    color: hsl(var(--muted-foreground));
    cursor: pointer;
    transition:
      transform,
      background-color,
      color,
      opacity 0.15s ease;
    white-space: nowrap;
  }

  .mood-state-chip:hover {
    background: hsl(var(--muted) / 0.6);
    border-color: hsl(var(--primary) / 0.3);
  }

  .mood-state-chip-active {
    background: hsl(var(--primary) / 0.12);
    border-color: hsl(var(--primary) / 0.5);
    color: hsl(var(--primary));
    font-weight: 500;
  }

  .emoji-option {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    padding: 0.5rem;
    border-radius: 0.8rem;
    border: 1px solid hsl(var(--border) / 0.6);
    background: hsl(var(--background) / 0.72);
    transition:
      transform 0.18s ease,
      border-color 0.18s ease,
      background-color 0.18s ease,
      box-shadow 0.18s ease;
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
    transition:
      transform 0.15s ease,
      border-color 0.15s ease,
      background-color 0.15s ease;
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

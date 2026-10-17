<script lang="ts">
  import { resolve } from '$app/paths';
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import PageHeader from '#lib/components/ui/PageHeader.svelte';
  import WeatherDisplay from '#lib/components/WeatherDisplay.svelte';
  import { filterDiaries } from '#lib/api/diaries.js';
  import { isAuthenticated } from '#lib/api/client.js';
  import { formatDisplayDate, getDayOfWeek } from '#lib/utils/date.js';
  import { MOOD_SCALE, SCENARIO_OPTIONS } from '#lib/utils/diaryEmoji.js';
  import MoodIcon from '#lib/components/ui/MoodIcon.svelte';
  import { isWMOCode } from '#lib/utils/weatherCodes.js';

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
      const data = await filterDiaries(selectedMood || undefined, selectedScenario || undefined);
      results = data.map((item: any) => ({
        id: item.id,
        date: item.date?.split(' ')[0] || item.date,
        snippet: (item.snippet || '')
          .replace(/<[^>]*>/g, ' ')
          .replace(/\s+/g, ' ')
          .trim(),
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
    goto(resolve('/diary/[date]', { date: date }));
  }

  onMount(() => {
    if (!$isAuthenticated) {
      goto(resolve('/login'));
      return;
    }
  });
</script>

<svelte:head>
  <title>心情筛选 - 吾身</title>
</svelte:head>

<div class="flex min-h-[100dvh] min-h-screen flex-col bg-background">
  <PageHeader title="心情筛选" />

  <main id="main-content" tabindex="-1" class="container-responsive flex-1 py-8">
    <!-- Header -->
    <div class="animate-fade-in mb-8">
      <h1 class="mb-2 text-2xl font-bold text-foreground">心情筛选</h1>
      <p class="text-sm text-muted-foreground">按心情档位和情景筛选日记</p>
    </div>

    <!-- Mood Slider -->
    <div
      class="animate-fade-in stagger-1 mb-4 rounded-xl border border-border/50 bg-card p-5 shadow-sm"
    >
      <div class="mb-3 flex items-center justify-between">
        <div class="text-sm font-semibold text-foreground">心情档位</div>
        {#if selectedMood > 0 || selectedScenario}
          <button
            onclick={clearAll}
            class="rounded-full border border-border/70 bg-muted/70 px-2 py-1 text-[11px] text-muted-foreground transition-colors hover:bg-muted"
          >
            清除全部
          </button>
        {/if}
      </div>
      <div class="mood-slider-container">
        <div class="mood-slider-track">
          {#each MOOD_SCALE as level (level.value)}
            <button
              onclick={() => handleMoodSelect(level.value)}
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
        {#if selectedMood > 0}
          <div
            class="mt-2 flex items-center justify-center gap-1 text-center text-xs text-muted-foreground"
          >
            <MoodIcon mood={selectedMood} size={20} />
            <span>{MOOD_SCALE.find((l) => l.value === selectedMood)?.label || ''}</span>
          </div>
        {/if}
      </div>
    </div>

    <!-- Scenario Filters -->
    <div
      class="animate-fade-in stagger-2 mb-6 rounded-xl border border-border/50 bg-card p-5 shadow-sm"
    >
      <div class="mb-3 text-sm font-semibold text-foreground">情景</div>
      <div class="flex flex-wrap gap-1.5">
        {#each SCENARIO_OPTIONS as scenario (scenario)}
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
      <div class="animate-fade-in flex flex-col items-center justify-center gap-3 py-12">
        <svg class="h-6 w-6 animate-spin text-primary" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"
          ></circle>
          <path
            class="opacity-75"
            fill="currentColor"
            d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
          ></path>
        </svg>
        <div class="text-sm text-muted-foreground">正在筛选...</div>
      </div>
    {:else if searched && results.length === 0}
      <div class="animate-fade-in flex flex-col items-center justify-center gap-4 py-12">
        <div class="flex h-16 w-16 items-center justify-center rounded-full bg-muted/50">
          <svg
            class="h-8 w-8 text-muted-foreground"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="1.5"
              d="M9.172 16.172a4 4 0 015.656 0M9 10h.01M15 10h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
            />
          </svg>
        </div>
        <div class="text-center">
          <p class="font-medium text-foreground">未找到匹配的日记</p>
          <p class="mt-1 text-sm text-muted-foreground">试试其他心情档位或情景</p>
        </div>
      </div>
    {:else if searched && results.length > 0}
      <div class="animate-fade-in mb-4 text-sm text-muted-foreground">
        找到 {results.length} 条匹配的日记
      </div>
      <div class="space-y-3">
        {#each results as result (result.id)}
          <button
            onclick={() => navigateToDiary(result.date)}
            class="group animate-fade-in w-full rounded-xl border border-border/50 bg-card p-4 text-left shadow-sm transition-all duration-200 hover:border-primary/30 hover:shadow-md"
          >
            <div class="mb-2 flex items-center justify-between">
              <div class="flex items-center gap-2">
                <span class="text-sm font-medium text-foreground">
                  {formatDisplayDate(result.date)}
                </span>
                <span class="text-xs text-muted-foreground">周{getDayOfWeek(result.date)}</span>
                {#if result.mood}
                  <span class="text-sm"><MoodIcon mood={result.mood} size={16} /></span>
                {/if}
                {#if result.weather && isWMOCode(result.weather)}
                  <WeatherDisplay wmoCode={parseInt(result.weather)} size="sm" showTemp={false} />
                {/if}
              </div>
              <svg
                class="h-4 w-4 text-muted-foreground transition-all group-hover:translate-x-0.5 group-hover:text-foreground"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M9 5l7 7-7 7"
                />
              </svg>
            </div>
            {#if result.scenarios && result.scenarios.length > 0}
              <div class="mb-2 flex flex-wrap gap-1">
                {#each result.scenarios as scenario (scenario)}
                  <span class="rounded-full bg-primary/10 px-1.5 py-0.5 text-[10px] text-primary"
                    >{scenario}</span
                  >
                {/each}
              </div>
            {/if}
            <p class="line-clamp-2 text-sm leading-relaxed text-muted-foreground">
              {result.snippet}
            </p>
          </button>
        {/each}
      </div>
    {:else}
      <div class="animate-fade-in flex flex-col items-center justify-center gap-4 py-12">
        <div class="flex h-16 w-16 items-center justify-center rounded-full bg-muted/50">
          <svg
            class="h-8 w-8 text-muted-foreground"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="1.5"
              d="M3 4a1 1 0 011-1h16a1 1 0 011 1v2.586a1 1 0 01-.293.707l-6.414 6.414a1 1 0 00-.293.707V17l-4 4v-6.586a1 1 0 00-.293-.707L3.293 7.293A1 1 0 013 6.586V4z"
            />
          </svg>
        </div>
        <div class="text-center">
          <p class="font-medium text-foreground">选择心情或情景开始筛选</p>
          <p class="mt-1 text-sm text-muted-foreground">点击上方的心情档位或情景标签</p>
        </div>
      </div>
    {/if}
  </main>
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
    transition:
      transform,
      background-color,
      color,
      opacity 0.15s ease;
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

<script lang="ts">
  import { resolve } from '$app/paths';
  import { onMount } from 'svelte';
  import { page } from '$app/state';
  import { goto } from '$app/navigation';
  import Calendar from '#lib/components/calendar/Calendar.svelte';
  import PageHeader from '#lib/components/ui/PageHeader.svelte';
  import {
    getDatesWithDiaries,
    getRecentDiaries,
    getDiaryStats,
    type CalendarDiaryMeta
  } from '#lib/api/diaries.js';
  import { isAuthenticated } from '#lib/api/client.js';
  import { getMonthRange, formatDisplayDate, parseDate } from '#lib/utils/date.js';

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
  $: datesWithDiaries = diaryMeta
    .filter((item) => item.has_content !== false)
    .map((item) => item.date);
  $: yearDiaryCount = yearDiaryMeta.filter((item) => item.has_content !== false).length;

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
      goto(resolve('/login'));
      return;
    }

    const dateParam = page.url.searchParams.get('date');
    if (dateParam) {
      try {
        const parsed = parseDate(dateParam);
        if (!isNaN(parsed.getTime())) {
          currentYear = parsed.getFullYear();
          currentMonth = parsed.getMonth() + 1;
          prevYear = currentYear;
          prevMonth = currentMonth;
        }
      } catch (e) {
        // 忽略解析错误，使用当前日期
      }
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

<div class="flex min-h-[100dvh] min-h-screen flex-col bg-background">
  <PageHeader title="日历" />

  <!-- Calendar -->
  <main
    id="main-content"
    tabindex="-1"
    class="container-responsive flex min-h-0 flex-1 flex-col py-4 lg:py-6"
  >
    <div class="flex min-h-0 flex-1 flex-col gap-4 xl:flex-row xl:gap-6">
      <!-- Left: Calendar -->
      <div class="flex min-h-0 w-full flex-col xl:min-w-0 xl:flex-1">
        <!-- 页面标题 - 视觉隐藏，用于屏幕阅读器的文档结构 -->
        <h1 class="sr-only">日历 — 查看与浏览我的日记记录</h1>
        <div
          class="relative rounded-xl border border-border/50 bg-card p-4 shadow-sm transition-shadow duration-300 hover:shadow-md lg:p-5 xl:flex xl:min-h-0 xl:flex-col"
        >
          {#if loading}
            <div class="absolute inset-0 flex flex-col items-center justify-center gap-3">
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
            <div class="animate-fade-in-only xl:flex xl:min-h-0 xl:flex-1 xl:flex-col">
              <Calendar
                bind:currentYear
                bind:currentMonth
                bind:yearViewActive
                bind:yearDiaryMeta
                {diaryMeta}
                onmonthchange={loadDatesWithDiaries}
              />
            </div>
          {/if}
        </div>
      </div>

      <!-- Right: Stats and Recent Entries -->
      <div
        class="flex w-full flex-shrink-0 flex-col gap-4 xl:min-h-0 xl:w-[340px] xl:self-stretch 2xl:w-[380px]"
      >
        <!-- Stats -->
        <div class="grid grid-cols-3 gap-3">
          <div
            class="rounded-xl border border-border/50 bg-card p-4 shadow-sm transition-all duration-200 hover:border-border/70 hover:shadow-md"
          >
            <div class="text-xs text-muted-foreground">{yearViewActive ? '今年' : '本月'}</div>
            <div class="mt-1 flex h-7 items-center text-xl font-bold text-foreground">
              {#if yearViewActive}
                <span class="animate-fade-in-only">{yearDiaryCount}</span>
              {:else if loading}
                <span
                  class="inline-block h-4 w-4 animate-spin rounded-full border-2 border-primary/30 border-t-primary"
                ></span>
              {:else}
                <span class="animate-fade-in-only">{datesWithDiaries.length}</span>
              {/if}
            </div>
          </div>

          <div
            class="rounded-xl border border-border/50 bg-card p-4 shadow-sm transition-all duration-200 hover:border-border/70 hover:shadow-md"
          >
            <div class="text-xs text-muted-foreground">连续天数</div>
            <div class="mt-1 flex h-7 items-center text-xl font-bold text-foreground">
              {#if statsLoading}
                <span
                  class="inline-block h-4 w-4 animate-spin rounded-full border-2 border-primary/30 border-t-primary"
                ></span>
              {:else}
                <span class="animate-fade-in-only">{stats?.streak ?? 0}</span>
              {/if}
            </div>
          </div>

          <div
            class="rounded-xl border border-border/50 bg-card p-4 shadow-sm transition-all duration-200 hover:border-border/70 hover:shadow-md"
          >
            <div class="text-xs text-muted-foreground">总计</div>
            <div class="mt-1 flex h-7 items-center text-xl font-bold text-foreground">
              {#if statsLoading}
                <span
                  class="inline-block h-4 w-4 animate-spin rounded-full border-2 border-primary/30 border-t-primary"
                ></span>
              {:else}
                <span class="animate-fade-in-only">{stats?.total ?? 0}</span>
              {/if}
            </div>
          </div>
        </div>

        <!-- Recent Entries -->
        <div
          class="flex min-h-0 flex-1 flex-col overflow-hidden rounded-xl border border-border/50 bg-card p-4 shadow-sm"
        >
          <h3 class="mb-3 text-sm font-semibold text-foreground">最近条目</h3>
          {#if recentLoading}
            <div class="flex flex-1 flex-col items-center justify-center gap-3">
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
          {:else if recentDiaries.length > 0}
            <div class="animate-fade-in-only flex-1 space-y-2 overflow-y-auto">
              {#each recentDiaries as diary (diary.date)}
                <a
                  href={resolve('/diary/[date]', { date: diary.date })}
                  class="group block rounded-lg border border-border/30 p-3 transition-all duration-200 hover:border-border/60 hover:bg-muted/60 hover:shadow-sm"
                >
                  <div
                    class="mb-1 text-xs text-muted-foreground transition-colors group-hover:text-foreground/70"
                  >
                    {formatDisplayDate(diary.date)}
                  </div>
                  <div class="line-clamp-2 text-sm text-foreground">
                    {getPreview(diary.content)}
                  </div>
                </a>
              {/each}
            </div>
          {:else}
            <div class="animate-fade-in-only flex flex-1 items-center justify-center">
              <div class="text-center text-sm text-muted-foreground">
                还没有条目。今天就开始记录吧！
              </div>
            </div>
          {/if}
        </div>
      </div>
    </div>
  </main>
</div>

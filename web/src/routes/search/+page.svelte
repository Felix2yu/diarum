<script lang="ts">
  import { resolve } from '$app/paths';
  import { onMount } from 'svelte';
  import { page } from '$app/state';
  import { afterNavigate, goto } from '$app/navigation';
  import PageHeader from '#lib/components/ui/PageHeader.svelte';
  import { searchDiaries } from '#lib/api/diaries.js';
  import { isAuthenticated } from '#lib/api/client.js';
  import { formatDisplayDate, getDayOfWeek } from '#lib/utils/date.js';
  import MoodIcon from '#lib/components/ui/MoodIcon.svelte';

  interface SearchResult {
    id: string;
    date: string;
    snippet: string;
    mood?: number;
    scenarios?: string[];
    weather?: string;
    tags?: string[];
  }

  let query = '';
  let results: SearchResult[] = [];
  let loading = false;
  let searched = false;
  let searchTimeout: ReturnType<typeof setTimeout>;
  let inputElement: HTMLInputElement;

  function handleInput() {
    clearTimeout(searchTimeout);
    if (query.trim().length === 0) {
      results = [];
      searched = false;
      return;
    }
    if (query.trim().length < 2) {
      return;
    }
    searchTimeout = setTimeout(() => {
      performSearch();
    }, 300);
  }

  async function performSearch() {
    if (query.trim().length < 2) return;

    loading = true;
    searched = true;

    try {
      const data = await searchDiaries(query.trim());
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
      console.error('Search error:', error);
      results = [];
    } finally {
      loading = false;
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter' && query.trim().length >= 2) {
      clearTimeout(searchTimeout);
      performSearch();
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
    const q = page.url.searchParams.get('q');
    if (q) {
      query = q;
      performSearch();
    } else {
      inputElement?.focus();
    }
  });

  // 同路由再搜一次（顶栏弹窗 goto('/search?q=…')）不会重建组件，onMount 不会重跑，
  // 因此 URL 上的 q 变化必须在这里同步。初始进入（type: 'enter'）已由 onMount 处理；
  // 跨路由进入本页时 onMount 已把 query 设为同一个值，相等判断可避免重复请求。
  afterNavigate((nav) => {
    if (nav.type === 'enter') return;

    const q = page.url.searchParams.get('q') ?? '';
    if (q === query) return;

    clearTimeout(searchTimeout);
    query = q;

    if (q) {
      performSearch();
    } else {
      results = [];
      searched = false;
    }
  });
</script>

<svelte:head>
  <title>搜索 - 吾身</title>
</svelte:head>

<div class="flex min-h-[100dvh] min-h-screen flex-col bg-background">
  <PageHeader title="搜索" />

  <main id="main-content" tabindex="-1" class="container-responsive flex-1 py-8">
    <div class="animate-fade-in mb-8">
      <h1 class="mb-2 text-2xl font-bold text-foreground">搜索日记</h1>
      <p class="text-sm text-muted-foreground">通过关键词查找你的日记条目</p>
    </div>

    <!-- Search Input -->
    <div class="animate-fade-in stagger-1 relative mb-6">
      <div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-4">
        <svg
          class="h-5 w-5 text-muted-foreground"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
          />
        </svg>
      </div>
      <input
        bind:this={inputElement}
        bind:value={query}
        oninput={handleInput}
        onkeydown={handleKeydown}
        type="text"
        placeholder="搜索你的日记..."
        class="w-full rounded-xl border border-border/50 bg-card py-3.5 pr-4 pl-12 text-foreground shadow-sm transition-all duration-200 placeholder:text-muted-foreground focus:border-transparent focus:ring-2 focus:ring-ring/50 focus:outline-none"
      />
      {#if query.length > 0}
        <button
          onclick={() => {
            query = '';
            results = [];
            searched = false;
            inputElement?.focus();
          }}
          class="absolute inset-y-0 right-0 flex items-center pr-4 text-muted-foreground transition-colors hover:text-foreground"
          title="清空搜索"
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
      {/if}
    </div>

    <!-- Loading State -->
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
        <div class="text-sm text-muted-foreground">正在搜索...</div>
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
          <p class="mt-1 text-sm text-muted-foreground">试试其他关键词</p>
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
                {#if result.weather}
                  <span class="text-sm">{result.weather}</span>
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
              d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
            />
          </svg>
        </div>
        <div class="text-center">
          <p class="font-medium text-foreground">输入关键词搜索日记</p>
          <p class="mt-1 text-sm text-muted-foreground">支持全文内容搜索</p>
        </div>
      </div>
    {/if}
  </main>
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

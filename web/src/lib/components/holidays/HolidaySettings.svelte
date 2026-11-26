<script lang="ts">
  /**
   * 设置 → 节假日：中国法定节假日与调休（休/班）标记。
   * 包含实例级开关、官方数据同步、离线 JSON 导入和手工日期覆盖。
   */
  import { onMount } from 'svelte';
  import {
    getCNHolidayOverview,
    getCNHolidayYear,
    setCNHolidaysEnabled,
    refreshCNHolidays,
    importCNHolidayJSON,
    upsertCNHolidayOverride,
    deleteCNHolidayOverride,
    type CNHolidayOverview,
    type CNHolidayDay,
    type CNHolidayYearResponse
  } from '#lib/api/holidays.js';
  import { invalidateHolidayCache, setCNHolidaysGlobalEnabledLocal } from '#lib/stores/holidays.js';

  let overview = $state<CNHolidayOverview | null>(null);
  let loading = $state(true);
  let error = $state('');
  let toggling = $state(false);
  let refreshing = $state(false);
  let importing = $state(false);
  let importInput = $state<HTMLInputElement | null>(null);
  let message = $state<{ ok: boolean; text: string } | null>(null);

  // 手工覆盖表单
  let formDate = $state('');
  let formName = $state('');
  let formIsOffDay = $state(true);
  let savingOverride = $state(false);
  let deletingDate = $state('');

  // 年度有效日程预览
  const currentYear = new Date().getFullYear();
  let previewYear = $state(currentYear);
  let previewData = $state<CNHolidayYearResponse | null>(null);
  let previewLoading = $state(false);

  let yearOptions = $derived.by(() => {
    const years: number[] = [currentYear, currentYear + 1];
    const add = (year: number) => {
      if (!years.includes(year)) years.push(year);
    };
    for (const y of overview?.years ?? []) {
      if (y.published) add(y.year);
    }
    for (const ov of overview?.overrides ?? []) {
      const y = Number(ov.date.slice(0, 4));
      if (Number.isInteger(y)) add(y);
    }
    return years.sort((a, b) => b - a);
  });

  let coveredRange = $derived.by(() => {
    const published = (overview?.years ?? []).filter((y) => y.published).map((y) => y.year);
    if (published.length === 0) return '';
    const min = published[0];
    const max = published[published.length - 1];
    return min === max ? `${min}` : `${min}–${max}`;
  });

  onMount(() => {
    void loadAll();
  });

  function flash(ok: boolean, text: string) {
    message = { ok, text };
    setTimeout(
      () => {
        if (message?.text === text) message = null;
      },
      ok ? 4000 : 8000
    );
  }

  function formatTime(iso?: string): string {
    if (!iso) return '';
    const d = new Date(iso);
    if (Number.isNaN(d.getTime())) return iso;
    return d.toLocaleString('zh-CN', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit'
    });
  }

  async function loadAll() {
    loading = true;
    error = '';
    try {
      overview = await getCNHolidayOverview();
      setCNHolidaysGlobalEnabledLocal(overview.enabled);
      await loadPreview(previewYear);
    } catch (e) {
      error = e instanceof Error ? e.message : '加载失败';
    } finally {
      loading = false;
    }
  }

  async function loadPreview(year: number) {
    previewLoading = true;
    try {
      previewData = await getCNHolidayYear(year);
    } catch {
      previewData = null;
    } finally {
      previewLoading = false;
    }
  }

  async function handleToggle() {
    if (!overview || toggling) return;
    const next = !overview.enabled;
    toggling = true;
    try {
      await setCNHolidaysEnabled(next);
      overview = { ...overview, enabled: next };
      setCNHolidaysGlobalEnabledLocal(next);
      flash(true, next ? '已启用节假日标记' : '已停用节假日标记');
    } catch (e) {
      flash(false, e instanceof Error ? e.message : '切换失败');
    } finally {
      toggling = false;
    }
  }

  async function handleRefresh() {
    if (refreshing) return;
    refreshing = true;
    try {
      overview = await refreshCNHolidays();
      invalidateHolidayCache();
      await loadPreview(previewYear);
      flash(true, '官方节假日数据同步完成');
    } catch (e) {
      flash(false, e instanceof Error ? e.message : '同步失败');
    } finally {
      refreshing = false;
    }
  }

  async function handleImportFiles(event: Event) {
    const input = event.currentTarget as HTMLInputElement;
    const files = Array.from(input.files ?? []);
    input.value = '';
    if (files.length === 0 || importing) return;

    importing = true;
    try {
      const importedYears: number[] = [];
      let lastDaysCount = 0;
      let latestOverview: CNHolidayOverview | null = null;
      for (const file of files) {
        if (file.size > 512 * 1024) {
          throw new Error(`${file.name} 超过 512 KB 限制`);
        }
        const res = await importCNHolidayJSON(file);
        importedYears.push(res.imported.year);
        lastDaysCount = res.imported.days.length;
        latestOverview = res.overview;
      }
      if (latestOverview) {
        overview = latestOverview;
      }
      invalidateHolidayCache();
      const targetYear = Math.max(...importedYears);
      previewYear = targetYear;
      await loadPreview(targetYear);
      if (importedYears.length === 1) {
        flash(true, `已导入 ${importedYears[0]} 年数据（${lastDaysCount} 天）`);
      } else {
        flash(true, `已导入 ${importedYears.sort((a, b) => a - b).join('、')} 年数据`);
      }
    } catch (e) {
      flash(false, e instanceof Error ? e.message : '导入失败');
    } finally {
      importing = false;
    }
  }

  function editDay(day: CNHolidayDay) {
    formDate = day.date;
    formName = day.name;
    formIsOffDay = day.isOffDay;
  }

  async function handleSaveOverride(event: SubmitEvent) {
    event.preventDefault();
    const date = formDate.trim();
    const name = formName.trim();
    if (!date || !name || savingOverride) return;

    savingOverride = true;
    try {
      const saved = await upsertCNHolidayOverride({ date, name, isOffDay: formIsOffDay });
      if (overview) {
        const filtered = overview.overrides.filter((item) => item.date !== saved.date);
        const nextOverrides = [...filtered, saved].sort((a, b) => a.date.localeCompare(b.date));
        overview = { ...overview, overrides: nextOverrides };
      }
      invalidateHolidayCache();
      const savedYear = Number(saved.date.slice(0, 4));
      if (savedYear === previewYear) {
        await loadPreview(previewYear);
      }
      formDate = '';
      formName = '';
      formIsOffDay = true;
      flash(true, `已保存 ${saved.date} 的覆盖`);
    } catch (e) {
      flash(false, e instanceof Error ? e.message : '保存覆盖失败');
    } finally {
      savingOverride = false;
    }
  }

  async function handleDeleteOverride(date: string) {
    if (deletingDate) return;
    deletingDate = date;
    try {
      await deleteCNHolidayOverride(date);
      if (overview) {
        overview = {
          ...overview,
          overrides: overview.overrides.filter((item) => item.date !== date)
        };
      }
      invalidateHolidayCache();
      if (Number(date.slice(0, 4)) === previewYear) {
        await loadPreview(previewYear);
      }
      flash(true, `已删除 ${date} 的覆盖`);
    } catch (e) {
      flash(false, e instanceof Error ? e.message : '删除覆盖失败');
    } finally {
      deletingDate = '';
    }
  }

  function handlePreviewYearChange(event: Event) {
    const nextYear = Number((event.currentTarget as HTMLSelectElement).value);
    if (Number.isInteger(nextYear)) {
      previewYear = nextYear;
      void loadPreview(nextYear);
    }
  }
</script>

<div class="space-y-4">
  {#if message}
    <div
      class="sticky top-12 z-10 rounded-lg border p-3 text-sm break-words shadow-sm {message.ok
        ? 'border-emerald-500/20 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300'
        : 'border-destructive/20 bg-destructive/10 text-destructive'}"
    >
      {message.text}
    </div>
  {/if}

  {#if error}
    <div
      class="bg-destructive/10 text-destructive flex items-center justify-between gap-3 rounded-lg p-3 text-sm"
    >
      <span>{error}</span>
      <button onclick={loadAll} class="underline">重试</button>
    </div>
  {:else if loading && !overview}
    <div class="space-y-3">
      {#each Array(3) as _, i (i)}
        <div class="h-32 animate-pulse rounded-xl bg-muted/40"></div>
      {/each}
    </div>
  {:else if overview}
    <!-- 卡片 1：开关与官方数据同步 -->
    <section class="rounded-xl border border-border/50 bg-card p-4 shadow-sm sm:p-6">
      <div class="flex items-start justify-between gap-3">
        <div class="min-w-0">
          <h2 class="flex flex-wrap items-center gap-2 text-lg font-semibold text-foreground">
            中国法定节假日
            <span
              class="rounded-full px-2 py-0.5 text-[11px] font-medium ring-1 ring-border/50 ring-inset {overview.enabled
                ? 'bg-emerald-500/10 text-emerald-700 ring-emerald-500/20 dark:text-emerald-300'
                : 'bg-muted text-muted-foreground'}"
            >
              {overview.enabled ? '已启用' : '已停用'}
            </span>
          </h2>
          <p class="mt-1 text-sm text-muted-foreground">
            在日历中标记中国大陆法定节假日（休）和调休补班日（班）。数据来自国务院公开通知。
          </p>
        </div>
        <button
          onclick={loadAll}
          disabled={loading}
          title="刷新状态"
          aria-label="刷新状态"
          class="flex-shrink-0 rounded-lg p-2 text-muted-foreground transition-colors hover:bg-muted/60 hover:text-foreground disabled:opacity-50"
        >
          <svg
            class="h-4 w-4 {loading ? 'animate-spin' : ''}"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
            aria-hidden="true"
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

      <div class="mt-5 flex items-center justify-between gap-4 border-t border-border/50 pt-4">
        <div class="min-w-0">
          <div class="text-sm font-medium text-foreground">启用节假日标记</div>
          <p class="mt-0.5 text-xs text-muted-foreground">
            启用后，日历、往昔今朝和最近条目会显示休/班标记。
          </p>
        </div>
        <button
          type="button"
          role="switch"
          aria-checked={overview.enabled}
          aria-label="启用节假日标记"
          disabled={toggling}
          onclick={handleToggle}
          class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 focus:ring-2 focus:ring-primary/40 focus:outline-none disabled:opacity-50 {overview.enabled
            ? 'bg-primary'
            : 'bg-muted-foreground/30'}"
        >
          <span
            class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow transition duration-200 {overview.enabled
              ? 'translate-x-5'
              : 'translate-x-0'}"
          ></span>
        </button>
      </div>

      <div
        class="mt-4 flex flex-wrap items-center justify-between gap-3 border-t border-border/50 pt-4"
      >
        <div class="space-y-1 text-xs text-muted-foreground">
          <div class="flex flex-wrap items-center gap-2">
            {#if coveredRange}
              <span class="font-medium text-foreground">已覆盖 {coveredRange} 年</span>
              <span>·</span>
            {/if}
            {#if overview.last_fetched_at}
              <span>最近同步：{formatTime(overview.last_fetched_at)}</span>
            {:else}
              <span>当前使用内置基线数据</span>
            {/if}
          </div>
          <div>官方数据每年 10–12 月发布次年安排，启用后会在后台自动检查更新。</div>
        </div>
        <button
          type="button"
          onclick={handleRefresh}
          disabled={refreshing}
          class="rounded-lg bg-primary px-3 py-1.5 text-sm text-primary-foreground transition-colors hover:bg-primary/90 disabled:opacity-50"
        >
          {refreshing ? '同步中…' : '同步官方数据'}
        </button>
      </div>

      <div
        class="mt-4 flex flex-wrap items-center justify-between gap-3 rounded-lg border border-border/50 bg-muted/40 p-3.5"
      >
        <div class="min-w-0 flex-1 space-y-1 text-xs text-muted-foreground">
          <div class="font-medium text-foreground">离线导入</div>
          <p class="leading-relaxed">
            无法访问外网时，可从
            <a
              href="https://github.com/NateScarlet/holiday-cn"
              target="_blank"
              rel="noopener noreferrer"
              class="font-mono text-primary hover:underline">NateScarlet/holiday-cn</a
            >
            下载年度 JSON 文件后在此上传，支持一次选择多个年份。
          </p>
        </div>
        <div class="flex-shrink-0">
          <input
            bind:this={importInput}
            type="file"
            accept=".json,application/json"
            multiple
            onchange={handleImportFiles}
            class="sr-only"
          />
          <button
            type="button"
            disabled={importing}
            onclick={() => importInput?.click()}
            class="rounded-lg border border-border bg-background px-3 py-1.5 text-xs font-medium text-foreground transition-colors hover:bg-muted/60 disabled:opacity-50"
          >
            {importing ? '导入中…' : '选择 JSON 文件'}
          </button>
        </div>
      </div>
    </section>

    <!-- 卡片 2：手工覆盖 -->
    <section class="rounded-xl border border-border/50 bg-card p-4 shadow-sm sm:p-6">
      <div class="mb-4">
        <h3 class="font-semibold text-foreground">自定义节假日 / 调休</h3>
        <p class="mt-1 text-xs text-muted-foreground">
          添加官方安排之外的休息日（如公司团建假）或补班日。手工覆盖不会被官方同步覆盖。
        </p>
      </div>

      <form onsubmit={handleSaveOverride} class="grid grid-cols-1 items-end gap-3 sm:grid-cols-12">
        <div class="sm:col-span-3">
          <label
            for="holiday-override-date"
            class="mb-1 block text-xs font-medium text-muted-foreground"
          >
            日期
          </label>
          <input
            id="holiday-override-date"
            type="date"
            required
            bind:value={formDate}
            class="w-full rounded-lg border border-border bg-background px-3 py-2 text-sm focus:ring-2 focus:ring-primary/40 focus:outline-none"
          />
        </div>
        <div class="sm:col-span-4">
          <label
            for="holiday-override-name"
            class="mb-1 block text-xs font-medium text-muted-foreground"
          >
            名称
          </label>
          <input
            id="holiday-override-name"
            type="text"
            required
            maxlength="40"
            placeholder="例如：公司团建假"
            bind:value={formName}
            class="w-full rounded-lg border border-border bg-background px-3 py-2 text-sm focus:ring-2 focus:ring-primary/40 focus:outline-none"
          />
        </div>
        <div class="sm:col-span-3">
          <span class="mb-1 block text-xs font-medium text-muted-foreground">类型</span>
          <div class="grid grid-cols-2 rounded-lg border border-border bg-muted/40 p-0.5">
            <button
              type="button"
              onclick={() => (formIsOffDay = true)}
              class="rounded-md py-1.5 text-xs font-medium transition-colors {formIsOffDay
                ? 'bg-rose-500/15 text-rose-600 shadow-sm dark:text-rose-400'
                : 'text-muted-foreground hover:text-foreground'}"
            >
              休
            </button>
            <button
              type="button"
              onclick={() => (formIsOffDay = false)}
              class="rounded-md py-1.5 text-xs font-medium transition-colors {!formIsOffDay
                ? 'bg-slate-500/20 text-slate-700 shadow-sm dark:text-slate-200'
                : 'text-muted-foreground hover:text-foreground'}"
            >
              班
            </button>
          </div>
        </div>
        <div class="sm:col-span-2">
          <button
            type="submit"
            disabled={savingOverride || !formDate || !formName.trim()}
            class="w-full rounded-lg bg-primary px-3 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 disabled:opacity-50"
          >
            {savingOverride ? '保存中…' : '保存'}
          </button>
        </div>
      </form>

      <div class="mt-5 border-t border-border/50 pt-4">
        {#if overview.overrides.length === 0}
          <div class="py-3 text-center text-xs text-muted-foreground">还没有自定义覆盖</div>
        {:else}
          <div class="divide-y divide-border/40">
            {#each overview.overrides as ov (ov.date)}
              <div class="flex items-center justify-between gap-3 py-2.5">
                <div class="flex min-w-0 flex-wrap items-center gap-2.5">
                  <span class="font-mono text-xs text-foreground">{ov.date}</span>
                  <span
                    class="rounded px-1.5 py-0.5 text-[11px] font-semibold {ov.isOffDay
                      ? 'bg-rose-500/10 text-rose-600 dark:text-rose-400'
                      : 'bg-slate-500/15 text-slate-600 dark:text-slate-300'}"
                  >
                    {ov.isOffDay ? '休' : '班'}
                  </span>
                  <span class="truncate text-sm text-foreground">{ov.name}</span>
                </div>
                <div class="flex flex-shrink-0 items-center gap-2">
                  <button
                    type="button"
                    onclick={() => editDay(ov)}
                    class="rounded-md border border-border/60 px-2 py-1 text-xs text-foreground transition-colors hover:bg-muted/50"
                  >
                    编辑
                  </button>
                  <button
                    type="button"
                    disabled={deletingDate === ov.date}
                    onclick={() => handleDeleteOverride(ov.date)}
                    class="rounded-md px-2 py-1 text-xs text-rose-600 transition-colors hover:bg-rose-500/10 disabled:opacity-50 dark:text-rose-400"
                  >
                    删除
                  </button>
                </div>
              </div>
            {/each}
          </div>
        {/if}
      </div>
    </section>

    <!-- 卡片 3：年度有效日程预览 -->
    <section class="rounded-xl border border-border/50 bg-card p-4 shadow-sm sm:p-6">
      <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div>
          <h3 class="font-semibold text-foreground">年度有效日程</h3>
          <p class="mt-0.5 text-xs text-muted-foreground">官方安排与自定义覆盖合并后的结果</p>
        </div>
        <select
          value={previewYear}
          onchange={handlePreviewYearChange}
          aria-label="选择预览年份"
          class="rounded-lg border border-border bg-background px-3 py-1.5 text-sm focus:ring-2 focus:ring-primary/40 focus:outline-none"
        >
          {#each yearOptions as y (y)}
            <option value={y}>{y}</option>
          {/each}
        </select>
      </div>

      {#if previewLoading}
        <div class="py-8 text-center text-xs text-muted-foreground">加载中…</div>
      {:else if previewData && previewData.days.length > 0}
        {#if previewData.papers && previewData.papers.length > 0}
          <div class="mb-3 flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
            <span>政策来源：</span>
            {#each previewData.papers as paper, idx (paper)}
              <!-- eslint-disable svelte/no-navigation-without-resolve -->
              <a
                href={paper}
                target="_blank"
                rel="noopener noreferrer"
                class="break-all text-primary hover:underline">通知 {idx + 1}</a
              >
              <!-- eslint-enable svelte/no-navigation-without-resolve -->
            {/each}
          </div>
        {/if}
        <div
          class="max-h-72 divide-y divide-border/40 overflow-y-auto rounded-lg border border-border/40 px-3"
        >
          {#each previewData.days as day (day.date)}
            <div class="flex items-center justify-between gap-2 py-2 text-sm">
              <div class="flex min-w-0 flex-wrap items-center gap-2">
                <span class="font-mono text-xs text-muted-foreground">{day.date}</span>
                <span
                  class="rounded px-1.5 py-px text-[10px] font-semibold {day.isOffDay
                    ? 'bg-rose-500/10 text-rose-600 dark:text-rose-400'
                    : 'bg-slate-500/15 text-slate-600 dark:text-slate-300'}"
                >
                  {day.isOffDay ? '休' : '班'}
                </span>
                <span class="truncate text-foreground">{day.name}</span>
                {#if day.custom}
                  <span
                    class="rounded bg-primary/10 px-1.5 py-px text-[10px] font-medium text-primary"
                  >
                    自定义
                  </span>
                {/if}
              </div>
              <button
                type="button"
                onclick={() => editDay(day)}
                class="flex-shrink-0 rounded px-2 py-0.5 text-xs text-muted-foreground hover:bg-muted/60 hover:text-foreground"
              >
                覆盖
              </button>
            </div>
          {/each}
        </div>
      {:else}
        <div class="py-6 text-center text-xs text-muted-foreground">
          该年份暂无数据，可点击“同步官方数据”或离线导入 JSON。
        </div>
      {/if}
    </section>
  {/if}
</div>

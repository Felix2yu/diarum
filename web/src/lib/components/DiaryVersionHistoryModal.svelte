<script lang="ts">
  import {
    listDiaryVersions,
    getDiaryVersion,
    restoreDiaryVersion,
    type DiaryVersionSummary,
    type DiaryVersionDetail
  } from '#lib/api/diaries.js';
  import type { Diary } from '#lib/api/client.js';

  let {
    open = $bindable(false),
    diaryId = '',
    date = '',
    onRestored = (_diary: Diary) => {}
  } = $props();

  let versions = $state<DiaryVersionSummary[]>([]);
  let loading = $state(false);
  let error = $state('');
  let selected = $state<DiaryVersionDetail | null>(null);
  let detailLoading = $state(false);
  let restoring = $state(false);

  async function load() {
    if (!diaryId) {
      versions = [];
      return;
    }
    loading = true;
    error = '';
    selected = null;
    const result = await listDiaryVersions(diaryId);
    loading = false;
    if (result === null) {
      error = '加载历史版本失败，请稍后再试';
      return;
    }
    versions = result;
  }

  $effect(() => {
    if (open && diaryId) {
      load();
    }
  });

  async function handleSelect(version: DiaryVersionSummary) {
    if (selected?.id === version.id) {
      selected = null;
      return;
    }
    detailLoading = true;
    selected = null;
    const detail = await getDiaryVersion(diaryId, version.id);
    detailLoading = false;
    if (detail === null) {
      error = '加载版本内容失败，请稍后再试';
      return;
    }
    selected = detail;
  }

  async function handleRestore() {
    if (!selected) return;
    const ok = window.confirm(
      `确定把日记恢复到 ${formatTime(selected.created)} 的版本吗？\n当前内容会先保存为新版本，此操作可以再次撤销。`
    );
    if (!ok) return;
    restoring = true;
    error = '';
    const diary = await restoreDiaryVersion(diaryId, selected.id);
    restoring = false;
    if (!diary) {
      error = '恢复失败，请稍后再试';
      return;
    }
    onRestored(diary);
    open = false;
  }

  function formatTime(created: string): string {
    // 服务端时间为 "2006-01-02 15:04:05.000Z"，转成标准 ISO 以便各浏览器解析
    const parsed = new Date(created.includes('T') ? created : created.replace(' ', 'T'));
    if (Number.isNaN(parsed.getTime())) return created;
    return parsed.toLocaleString(undefined, {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit'
    });
  }

  function formatLength(len: number): string {
    if (len < 1000) return `${len} 字`;
    return `${(len / 1000).toFixed(1)}k 字`;
  }
</script>

{#if open}
  <div
    class="animate-fade-in fixed inset-0 z-50 flex items-start justify-center overflow-y-auto bg-black/50 p-4 backdrop-blur-sm"
    role="presentation"
    onclick={(e) => {
      if (e.target === e.currentTarget) open = false;
    }}
  >
    <div
      class="animate-slide-in-up my-8 w-full max-w-2xl rounded-xl border border-border/60 bg-card shadow-2xl"
      role="dialog"
      aria-modal="true"
      aria-label="历史版本"
    >
      <div class="flex items-center justify-between border-b border-border/60 px-5 py-4">
        <div class="flex items-center gap-2">
          <svg class="h-5 w-5 text-primary" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
            />
          </svg>
          <h2 class="text-base font-semibold text-foreground">历史版本</h2>
          {#if date}
            <span class="text-xs text-muted-foreground">{date}</span>
          {/if}
        </div>
        <button
          type="button"
          class="rounded-lg p-1.5 text-muted-foreground transition-colors hover:bg-muted"
          onclick={() => (open = false)}
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

      <div class="max-h-[65vh] overflow-y-auto px-5 py-4">
        {#if loading}
          <div class="flex flex-col items-center justify-center gap-3 py-10">
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
        {:else if error}
          <div class="text-destructive py-6 text-center text-sm">{error}</div>
        {:else if versions.length === 0}
          <div class="py-10 text-center text-sm text-muted-foreground">
            暂无历史版本
            <p class="mt-1 text-xs">进入编辑框并保存后，系统会自动保留编辑前的内容</p>
          </div>
        {:else}
          <ul class="space-y-2">
            {#each versions as version (version.id)}
              <li>
                <button
                  type="button"
                  class="w-full rounded-lg border text-left transition-colors {selected?.id ===
                  version.id
                    ? 'border-primary/60 bg-primary/5'
                    : 'border-border/60 hover:bg-muted/60'}"
                  onclick={() => handleSelect(version)}
                >
                  <div class="flex items-center justify-between gap-3 px-3 py-2.5">
                    <div class="min-w-0">
                      <div class="text-sm font-medium text-foreground">
                        {formatTime(version.created)}
                      </div>
                      <div class="mt-0.5 truncate text-xs text-muted-foreground">
                        {version.preview || '（空内容）'}
                      </div>
                    </div>
                    <div class="shrink-0 text-xs text-muted-foreground tabular-nums">
                      {formatLength(version.content_length)}
                    </div>
                  </div>
                </button>

                {#if selected?.id === version.id}
                  <div class="mt-2 rounded-lg border border-primary/40 bg-background">
                    {#if detailLoading}
                      <div class="py-6 text-center text-sm text-muted-foreground">
                        加载版本内容...
                      </div>
                    {:else if selected}
                      <div
                        class="max-h-64 overflow-y-auto px-3 py-3 text-sm break-words whitespace-pre-wrap text-foreground"
                      >
                        {selected.content || '（空内容）'}
                      </div>
                      <div class="flex justify-end gap-2 border-t border-border/60 px-3 py-2.5">
                        <button
                          type="button"
                          class="rounded-lg border border-border/60 px-3 py-1.5 text-xs text-muted-foreground transition-colors hover:bg-muted"
                          onclick={() => (selected = null)}
                        >
                          收起
                        </button>
                        <button
                          type="button"
                          disabled={restoring}
                          class="rounded-lg bg-primary px-3 py-1.5 text-xs text-primary-foreground transition-colors hover:bg-primary/90 disabled:opacity-50"
                          onclick={handleRestore}
                        >
                          {restoring ? '恢复中...' : '恢复到此版本'}
                        </button>
                      </div>
                    {/if}
                  </div>
                {/if}
              </li>
            {/each}
          </ul>
          <p class="mt-4 text-center text-[11px] text-muted-foreground">版本按保留期自动过期删除</p>
        {/if}
      </div>
    </div>
  </div>
{/if}

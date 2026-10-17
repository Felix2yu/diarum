<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { onlineState } from '#lib/stores/onlineStatus.js';
  import { syncState, cacheStats, forceSyncNow } from '#lib/stores/diaryCache.js';
  import { checkOnlineStatus } from '#lib/stores/onlineStatus.js';

  let visible = false;
  let isSyncing = false;
  let syncMessage = '';
  let pendingSyncCount = 0;
  let syncingNow = false;

  async function handleSync() {
    if (syncingNow) return;
    syncingNow = true;
    try {
      await forceSyncNow();
    } finally {
      syncingNow = false;
    }
  }

  let unsubOnline: (() => void) | undefined;
  let unsubSync: (() => void) | undefined;
  let unsubStats: (() => void) | undefined;

  onMount(() => {
    unsubOnline = onlineState.subscribe((s) => {
      visible = !s.isOnline;
    });

    unsubSync = syncState.subscribe((s) => {
      isSyncing = s.isSyncing;
      syncMessage = s.message;
    });

    unsubStats = cacheStats.subscribe((s) => {
      pendingSyncCount = s.pendingSync;
    });

    checkOnlineStatus();
  });

  onDestroy(() => {
    unsubOnline?.();
    unsubSync?.();
    unsubStats?.();
  });
</script>

{#if visible}
  <div
    class="safe-top flex items-center justify-center gap-3 border-b border-border/50 bg-accent/80 px-4 py-2 text-sm text-foreground/70 backdrop-blur-md"
  >
    <svg class="animate-pulse-subtle h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path
        stroke-linecap="round"
        stroke-linejoin="round"
        stroke-width="2"
        d="M1 10a1 12 0 1 0 22 0M5 12l-4 0"
      />
    </svg>
    <span class="font-medium">当前离线 — 内容已自动保存到本地</span>
    {#if pendingSyncCount > 0}
      <span class="opacity-70">（{pendingSyncCount} 条等待同步）</span>
    {/if}
    <button
      type="button"
      onclick={handleSync}
      disabled={syncingNow || pendingSyncCount === 0}
      class="ml-2 rounded-md bg-secondary px-3 py-1 text-secondary-foreground transition-opacity hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-50"
    >
      {syncingNow ? '同步中...' : '立即同步'}
    </button>
  </div>
{/if}

{#if (isSyncing || syncMessage) && !visible}
  <div
    class="fixed right-4 bottom-4 z-30 rounded-lg border border-border/50 bg-card/90 px-3 py-2 text-xs shadow-md"
  >
    {#if isSyncing}
      <span class="inline-flex items-center gap-2 text-muted-foreground">
        <svg class="h-3 w-3 animate-spin" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v6l4 2" />
        </svg>
        {syncMessage || '正在同步'}
      </span>
    {:else}
      <span class="text-muted-foreground">{syncMessage}</span>
    {/if}
  </div>
{/if}

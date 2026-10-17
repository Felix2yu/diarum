<script lang="ts">
  import { canInstall, installPWA } from '#lib/utils/pwa.js';
  import { onMount } from 'svelte';

  let showPrompt = false;
  let installing = false;

  onMount(() => {
    const unsubscribe = canInstall.subscribe((value) => {
      showPrompt = value;
    });

    return unsubscribe;
  });

  async function handleInstall() {
    installing = true;
    try {
      await installPWA();
    } finally {
      installing = false;
    }
  }

  function dismiss() {
    showPrompt = false;
  }
</script>

{#if showPrompt}
  <div class="animate-fade-in fixed right-4 bottom-4 left-4 z-50 md:right-4 md:left-auto md:w-96">
    <div class="glass rounded-xl border border-border/50 bg-card p-4 shadow-xl">
      <div class="flex items-start gap-3">
        <div class="flex-shrink-0">
          <svg class="h-5 w-5 text-primary" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M12 3v12m0 0l-4-4m4 4l4-4M4 21h16"
            />
          </svg>
        </div>
        <div class="min-w-0 flex-1">
          <h3 class="text-sm font-semibold text-foreground">安装 吾身</h3>
          <p class="mt-1 text-sm text-muted-foreground">
            将应用安装到主屏幕以获得更快的访问速度和离线使用支持
          </p>
          <div class="mt-3 flex items-center gap-2">
            <button
              type="button"
              onclick={handleInstall}
              disabled={installing}
              class="inline-flex items-center gap-2 rounded-lg bg-primary px-3 py-2 text-sm text-primary-foreground transition-opacity hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {installing ? '安装中...' : '安装'}
            </button>
            <button
              type="button"
              onclick={dismiss}
              class="rounded-lg bg-secondary px-3 py-2 text-sm text-secondary-foreground transition-opacity hover:opacity-90"
            >
              稍后
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
{/if}

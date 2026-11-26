<script lang="ts">
  import type { UploadJob } from '#lib/utils/uploadQueue.js';

  let {
    jobs = [],
    onCancel = (_id: string) => {},
    onRetry = (_id: string) => {},
    onRemove = (_id: string) => {},
    onClear = () => {}
  }: {
    jobs: UploadJob[];
    onCancel?: (id: string) => void;
    onRetry?: (id: string) => void;
    onRemove?: (id: string) => void;
    onClear?: () => void;
  } = $props();

  const hasFinished = $derived(
    jobs.some(
      (job) => job.status === 'success' || job.status === 'error' || job.status === 'canceled'
    )
  );

  function statusLabel(job: UploadJob): string {
    switch (job.status) {
      case 'queued':
        return '等待上传';
      case 'uploading':
        return `上传中 ${job.progress}%`;
      case 'success':
        return '已完成';
      case 'canceled':
        return '已取消';
      case 'error':
        return job.error || '上传失败';
    }
  }
</script>

{#if jobs.length > 0}
  <div
    class="animate-fade-in absolute bottom-16 left-3 z-20 w-72 rounded-xl border border-border/60 bg-card/95 p-3 shadow-xl backdrop-blur"
    aria-live="polite"
  >
    <div class="mb-2 flex items-center justify-between">
      <span class="text-xs font-medium text-foreground">图片上传 ({jobs.length})</span>
      {#if hasFinished}
        <button
          type="button"
          class="text-xs text-muted-foreground transition-colors hover:text-foreground"
          onclick={onClear}
        >
          清除已完成
        </button>
      {/if}
    </div>

    <div class="max-h-52 space-y-2 overflow-y-auto">
      {#each jobs as job (job.id)}
        <div class="rounded-lg bg-muted/40 px-2 py-1.5">
          <div class="flex items-center gap-2">
            <span
              class="h-1.5 w-1.5 shrink-0 rounded-full {job.status === 'error'
                ? 'bg-destructive'
                : job.status === 'success'
                  ? 'bg-emerald-500'
                  : job.status === 'canceled'
                    ? 'bg-muted-foreground'
                    : 'animate-pulse bg-primary'}"
            ></span>
            <span class="min-w-0 flex-1 truncate text-xs text-foreground" title={job.file.name}>
              {job.file.name}
            </span>
            {#if job.status === 'uploading' || job.status === 'queued'}
              <button
                type="button"
                class="shrink-0 text-xs text-muted-foreground hover:text-foreground"
                onclick={() => onCancel(job.id)}
              >
                取消
              </button>
            {:else if job.status === 'error' || job.status === 'canceled'}
              <button
                type="button"
                class="shrink-0 text-xs text-primary hover:underline"
                onclick={() => onRetry(job.id)}
              >
                重试
              </button>
              <button
                type="button"
                class="shrink-0 text-xs text-muted-foreground hover:text-foreground"
                onclick={() => onRemove(job.id)}
              >
                移除
              </button>
            {:else}
              <button
                type="button"
                class="shrink-0 text-xs text-muted-foreground hover:text-foreground"
                onclick={() => onRemove(job.id)}
              >
                移除
              </button>
            {/if}
          </div>

          {#if job.status === 'uploading' || job.status === 'queued'}
            <div class="mt-1.5 h-1 overflow-hidden rounded-full bg-border/60">
              <div
                class="h-full rounded-full bg-primary transition-[width] duration-200"
                style="width: {job.progress}%"
              ></div>
            </div>
          {/if}

          {#if job.status === 'error'}
            <p class="text-destructive mt-1 truncate text-[11px]" title={statusLabel(job)}>
              {statusLabel(job)}
            </p>
          {/if}
        </div>
      {/each}
    </div>
  </div>
{/if}

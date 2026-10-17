<script lang="ts">
  import { polishText, type PolishResult } from '#lib/api/ai.js';

  let {
    open = $bindable(false),
    sourceText = $bindable(''),
    onApply = (text: string) => {}
  } = $props();

  type Mode = 'medium' | 'strong' | 'custom';
  let mode: Mode = $state('medium');
  let customPrompt = $state('');
  let loading = $state(false);
  let error = $state('');
  let result = $state<PolishResult | null>(null);

  function handleClose() {
    open = false;
  }

  function handleModeChange(next: Mode) {
    mode = next;
  }

  async function handlePolish() {
    if (!sourceText.trim()) {
      error = '没有可整理的内容';
      return;
    }
    if (mode === 'custom' && !customPrompt.trim()) {
      error = '请输入自定义提示词';
      return;
    }
    loading = true;
    error = '';
    result = null;
    try {
      const res = await polishText(sourceText, mode, mode === 'custom' ? customPrompt : undefined);
      result = res;
    } catch (err) {
      error = (err as Error)?.message || 'AI 整理失败，请稍后再试';
    } finally {
      loading = false;
    }
  }

  function handleApply() {
    if (!result) return;
    onApply(result.content);
    open = false;
  }
</script>

{#if open}
  <div
    class="animate-fade-in fixed inset-0 z-50 flex items-start justify-center overflow-y-auto bg-black/50 p-4 backdrop-blur-sm"
    role="presentation"
    onclick={(e) => {
      if (e.target === e.currentTarget) handleClose();
    }}
  >
    <div
      class="animate-slide-in-up my-8 w-full max-w-3xl rounded-xl border border-border/60 bg-card shadow-2xl"
    >
      <div class="flex items-center justify-between border-b border-border/60 px-5 py-4">
        <div class="flex items-center gap-2">
          <svg class="h-5 w-5 text-primary" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M15.232 5.232l3.536 3.536M9 11l6.586-6.586a2 2 0 112.828 2.828L11.828 13.828 8 14l.172-3.828z"
            />
          </svg>
          <h2 class="text-base font-semibold text-foreground">AI 整理文本</h2>
        </div>
        <button
          type="button"
          onclick={handleClose}
          class="text-muted-foreground transition-colors hover:text-foreground"
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

      <div class="space-y-4 px-5 py-4">
        <div>
          <div class="mb-2 text-xs font-medium text-foreground/80">选择整理方式</div>
          <div class="grid grid-cols-1 gap-2 sm:grid-cols-3">
            {#each [{ key: 'medium' as Mode, title: '中等', desc: '去语气词 · 纠错 · 自动分段' }, { key: 'strong' as Mode, title: '强力', desc: '重组句子 · 精简冗余' }, { key: 'custom' as Mode, title: '自定义', desc: '使用你自己的提示词' }] as option (option.key)}
              <button
                type="button"
                onclick={() => handleModeChange(option.key)}
                class="rounded-lg border px-3 py-2.5 text-left transition-all {mode === option.key
                  ? 'border-primary/60 bg-primary/10 shadow-sm ring-1 ring-primary/20'
                  : 'border-border/60 hover:border-primary/30 hover:bg-muted/40'}"
              >
                <div class="text-sm font-medium text-foreground">{option.title}</div>
                <div class="mt-0.5 text-[11px] text-muted-foreground">{option.desc}</div>
              </button>
            {/each}
          </div>
        </div>

        {#if mode === 'custom'}
          <div>
            <label
              class="mb-1.5 block text-xs font-medium text-foreground/80"
              for="customPromptInput">自定义提示词（system prompt）</label
            >
            <textarea
              id="customPromptInput"
              bind:value={customPrompt}
              placeholder="例如：将下面的日记改成第三人称叙事，保持简洁..."
              rows="3"
              class="w-full resize-y rounded-lg border border-border/60 bg-muted/30 px-3 py-2 text-sm placeholder:text-muted-foreground/50 focus:border-primary focus:ring-1 focus:ring-primary/30 focus:outline-none"
            ></textarea>
          </div>
        {/if}

        <div class="grid grid-cols-1 gap-3 md:grid-cols-2">
          <div>
            <div class="mb-1.5 flex items-center justify-between">
              <span class="text-xs font-medium text-foreground/80">原文</span>
              <span class="text-[10px] text-muted-foreground">{sourceText.length} 字</span>
            </div>
            <div
              class="h-64 overflow-y-auto rounded-lg border border-border/40 bg-muted/30 p-3 text-sm leading-relaxed whitespace-pre-wrap text-foreground/90"
            >
              {sourceText || '（空）'}
            </div>
          </div>
          <div>
            <div class="mb-1.5 flex items-center justify-between">
              <span class="text-xs font-medium text-foreground/80">AI 整理结果</span>
              {#if result}
                <span class="text-[10px] text-muted-foreground">{result.content.length} 字</span>
              {/if}
            </div>
            <div
              class="h-64 overflow-y-auto rounded-lg border border-border/40 bg-muted/30 p-3 text-sm leading-relaxed whitespace-pre-wrap text-foreground/90"
            >
              {#if loading}
                <div class="flex items-center gap-2 text-xs text-muted-foreground">
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
                      d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
                    ></path>
                  </svg>
                  正在整理，请稍候...
                </div>
              {:else if result}
                {result.content}
              {:else if error}
                <div class="text-destructive text-xs">{error}</div>
              {:else}
                <div class="text-xs text-muted-foreground">点击下方「开始整理」获取结果</div>
              {/if}
            </div>
          </div>
        </div>
      </div>

      <div
        class="flex items-center justify-between gap-2 rounded-b-xl border-t border-border/60 bg-muted/20 px-5 py-3"
      >
        <button
          type="button"
          onclick={handleClose}
          class="rounded-lg px-4 py-2 text-sm text-muted-foreground transition-colors hover:bg-muted/50 hover:text-foreground"
        >
          取消
        </button>
        <div class="flex items-center gap-2">
          <button
            type="button"
            onclick={handlePolish}
            disabled={loading}
            class="inline-flex items-center gap-1.5 rounded-lg border border-primary/20 bg-primary/10 px-4 py-2 text-sm text-primary transition-colors hover:bg-primary/20 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {#if loading}
              <svg class="h-3.5 w-3.5 animate-spin" fill="none" viewBox="0 0 24 24">
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
                  d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
                ></path>
              </svg>
            {:else}
              <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M13 10V3L4 14h7v7l9-11h-7z"
                />
              </svg>
            {/if}
            {loading ? '整理中...' : '开始整理'}
          </button>
          <button
            type="button"
            onclick={handleApply}
            disabled={!result}
            class="inline-flex items-center gap-1.5 rounded-lg bg-primary px-4 py-2 text-sm text-primary-foreground transition-opacity hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-40"
          >
            <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M5 13l4 4L19 7"
              />
            </svg>
            应用到日记
          </button>
        </div>
      </div>
    </div>
  </div>
{/if}

<style>
  .animate-slide-in-up {
    animation: slide-in-up 0.22s ease-out;
  }
  @keyframes slide-in-up {
    from {
      transform: translateY(16px);
      opacity: 0;
    }
    to {
      transform: translateY(0);
      opacity: 1;
    }
  }
</style>

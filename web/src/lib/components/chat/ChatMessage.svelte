<script lang="ts">
  import { resolve } from '$app/paths';
  import type { Message } from '#lib/api/chat.js';
  import type { Diary } from '#lib/api/client.js';
  import { getDiariesByIds } from '#lib/api/diaries.js';
  import { goto } from '$app/navigation';
  import { renderMarkdown } from '#lib/utils/renderMarkdown.js';
  import { formatDisplayDate, formatTime } from '#lib/utils/date.js';

  export let message: Message;
  export let isStreaming = false;

  let expanded = false;

  let diaries: Diary[] = [];
  let loading = false;
  let loaded = false;

  function formatDate(dateStr: string): string {
    return formatTime(dateStr);
  }

  function formatDiaryDate(dateStr: string): string {
    return formatDisplayDate(dateStr);
  }

  function truncateContent(content: string, maxLength: number = 80): string {
    if (content.length <= maxLength) return content;
    return content.substring(0, maxLength) + '...';
  }

  async function toggleExpanded() {
    expanded = !expanded;
    if (expanded && !loaded && message.referenced_diaries) {
      loading = true;
      try {
        diaries = await getDiariesByIds(message.referenced_diaries);
        loaded = true;
      } catch (error) {
        console.error('Failed to load diaries:', error);
      } finally {
        loading = false;
      }
    }
  }

  function navigateToDiary(date: string) {
    goto(resolve('/diary/[date]', { date: date }));
  }
</script>

<!-- eslint-disable svelte/no-at-html-tags -->
<!-- 本文件所有 {@html} 出口均已过 DOMPurify 净化 -->

<div class="flex {message.role === 'user' ? 'justify-end' : 'justify-start'} mb-4">
  <div class="max-w-[80%] {message.role === 'user' ? 'order-2' : 'order-1'}">
    <div
      class="rounded-2xl px-4 py-3 {message.role === 'user'
        ? 'rounded-br-md bg-primary text-primary-foreground shadow-sm'
        : 'rounded-bl-md bg-muted text-foreground'}"
    >
      {#if message.role === 'user'}
        <div class="text-sm break-words whitespace-pre-wrap">
          {message.content}
        </div>
      {:else}
        <div class="prose prose-sm dark:prose-invert markdown-content max-w-none break-words">
          {@html renderMarkdown(message.content)}
        </div>
        {#if isStreaming}
          <div class="mt-3 flex justify-center border-t border-border/30 pt-2">
            <div class="flex items-center gap-2 text-xs text-muted-foreground">
              <svg class="h-4 w-4 animate-spin" viewBox="0 0 24 24" fill="none">
                <circle
                  class="opacity-25"
                  cx="12"
                  cy="12"
                  r="10"
                  stroke="currentColor"
                  stroke-width="3"
                ></circle>
                <path
                  class="opacity-75"
                  fill="currentColor"
                  d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                ></path>
              </svg>
              <span>正在思考...</span>
            </div>
          </div>
        {/if}
      {/if}
    </div>

    <div
      class="mt-1 flex items-center gap-2 px-1 {message.role === 'user'
        ? 'justify-end'
        : 'justify-start'}"
    >
      {#if message.created}
        <span class="text-xs text-muted-foreground">{formatDate(message.created)}</span>
      {/if}

      {#if message.referenced_diaries && message.referenced_diaries.length > 0}
        <button
          type="button"
          class="flex items-center gap-1 text-xs text-muted-foreground transition-colors hover:text-foreground"
          onclick={toggleExpanded}
        >
          <svg
            class="h-3 w-3 transition-transform {expanded ? 'rotate-90' : ''}"
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
          已引用 {message.referenced_diaries.length} 篇日记
        </button>
      {/if}
    </div>

    {#if expanded && message.referenced_diaries && message.referenced_diaries.length > 0}
      <div class="mt-2 space-y-2 px-1">
        {#if loading}
          <div class="text-xs text-muted-foreground">加载中...</div>
        {:else}
          {#each diaries as diary (diary.id)}
            <button
              type="button"
              class="w-full rounded-lg border border-border/50 bg-background/50 p-2 text-left transition-colors hover:border-border hover:bg-background"
              onclick={() => navigateToDiary(diary.date)}
            >
              <div class="text-xs font-medium text-foreground">{formatDiaryDate(diary.date)}</div>
              <div class="mt-1 line-clamp-2 text-xs text-muted-foreground">
                {truncateContent(diary.content)}
              </div>
            </button>
          {/each}
        {/if}
      </div>
    {/if}
  </div>
</div>

<!-- eslint-enable svelte/no-at-html-tags -->
<style>
  .markdown-content :global(h1) {
    font-size: 1.25rem;
    font-weight: 700;
    margin-top: 1rem;
    margin-bottom: 0.5rem;
  }

  .markdown-content :global(h2) {
    font-size: 1.125rem;
    font-weight: 600;
    margin-top: 0.875rem;
    margin-bottom: 0.375rem;
  }

  .markdown-content :global(h3) {
    font-size: 1rem;
    font-weight: 600;
    margin-top: 0.75rem;
    margin-bottom: 0.25rem;
  }

  .markdown-content :global(p) {
    margin-bottom: 0.5rem;
    line-height: 1.6;
  }

  .markdown-content :global(p:last-child) {
    margin-bottom: 0;
  }

  .markdown-content :global(ul),
  .markdown-content :global(ol) {
    margin-left: 1.25rem;
    margin-bottom: 0.5rem;
  }

  .markdown-content :global(ul) {
    list-style-type: disc;
  }

  .markdown-content :global(ol) {
    list-style-type: decimal;
  }

  .markdown-content :global(li) {
    margin-bottom: 0.25rem;
    line-height: 1.5;
  }

  .markdown-content :global(strong) {
    font-weight: 600;
  }

  .markdown-content :global(em) {
    font-style: italic;
  }

  .markdown-content :global(code) {
    background-color: hsl(var(--background) / 0.5);
    padding: 0.125rem 0.375rem;
    border-radius: 0.25rem;
    font-size: 0.875em;
    font-family: ui-monospace, monospace;
  }

  .markdown-content :global(pre) {
    background-color: hsl(var(--background) / 0.5);
    padding: 0.75rem;
    border-radius: 0.5rem;
    overflow-x: auto;
    margin-bottom: 0.5rem;
  }

  .markdown-content :global(pre code) {
    background: none;
    padding: 0;
  }

  .markdown-content :global(blockquote) {
    border-left: 3px solid hsl(var(--border));
    padding-left: 0.75rem;
    margin-left: 0;
    margin-bottom: 0.5rem;
    opacity: 0.9;
  }

  .markdown-content :global(a) {
    color: hsl(var(--primary));
    text-decoration: underline;
  }

  .markdown-content :global(hr) {
    border: none;
    border-top: 1px solid hsl(var(--border));
    margin: 0.75rem 0;
  }
</style>

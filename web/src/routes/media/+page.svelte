<script lang="ts">
  import { resolve } from '$app/paths';
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { isAuthenticated } from '#lib/api/client.js';
  import {
    getAllMedia,
    getMediaFileUrl,
    deleteMediaById,
    type MediaWithDiary
  } from '#lib/api/media.js';
  import PageHeader from '#lib/components/ui/PageHeader.svelte';
  import { formatDisplayDate, formatTime } from '#lib/utils/date.js';

  let mediaList: MediaWithDiary[] = [];
  let loading = true;
  let currentPage = 1;
  let totalPages = 1;
  let totalItems = 0;

  // Modal state
  let selectedMedia: MediaWithDiary | null = null;
  let showModal = false;
  let deleting = false;
  let showDeleteConfirm = false;

  async function loadMedia() {
    loading = true;
    const result = await getAllMedia(currentPage, 30);
    mediaList = result.items;
    totalPages = result.totalPages;
    totalItems = result.totalItems;
    loading = false;
  }

  function openModal(media: MediaWithDiary) {
    selectedMedia = media;
    showModal = true;
    showDeleteConfirm = false;
  }

  function closeModal() {
    showModal = false;
    selectedMedia = null;
    showDeleteConfirm = false;
  }

  function handleModalBackdropClick(e: MouseEvent) {
    if (e.target === e.currentTarget) {
      closeModal();
    }
  }

  function groupByDate(items: MediaWithDiary[]): Map<string, MediaWithDiary[]> {
    const groups = new Map<string, MediaWithDiary[]>();
    for (const item of items) {
      const dateKey = item.created?.split(' ')[0] || '';
      if (!groups.has(dateKey)) {
        groups.set(dateKey, []);
      }
      groups.get(dateKey)!.push(item);
    }
    return groups;
  }

  async function handleDelete() {
    if (!selectedMedia) return;

    deleting = true;
    const success = await deleteMediaById(selectedMedia.id!);
    deleting = false;

    if (success) {
      mediaList = mediaList.filter((m) => m.id !== selectedMedia!.id);
      totalItems--;
      closeModal();
    }
  }

  function goToDiary(date: string) {
    closeModal();
    goto(resolve('/diary/[date]', { date: date }));
  }

  onMount(() => {
    if (!$isAuthenticated) {
      goto(resolve('/login'));
      return;
    }
    loadMedia();
  });

  $: groupedMedia = groupByDate(mediaList);
</script>

<svelte:head>
  <title>媒体库 - 吾身</title>
</svelte:head>

<div class="flex min-h-[100dvh] min-h-screen flex-col bg-background">
  <PageHeader title="媒体库">
    <span slot="subtitle" class="text-sm text-muted-foreground">
      {#if !loading}({totalItems}){/if}
    </span>
  </PageHeader>

  <!-- Main Content -->
  <main id="main-content" tabindex="-1" class="container-responsive flex-1 py-8">
    <!-- 页面标题 - 视觉隐藏，用于屏幕阅读器的文档结构 -->
    <h1 class="sr-only">媒体库 — 管理日记中上传的图片与文件</h1>
    {#if loading}
      <div class="flex flex-col items-center justify-center gap-3 py-20">
        <svg class="h-6 w-6 animate-spin text-primary" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"
          ></circle>
          <path
            class="opacity-75"
            fill="currentColor"
            d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
          ></path>
        </svg>
        <div class="text-sm text-muted-foreground">正在加载...</div>
      </div>
    {:else if mediaList.length === 0}
      <div class="flex flex-col items-center justify-center gap-4 py-20">
        <svg
          class="h-16 w-16 text-muted-foreground/30"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="1.5"
            d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z"
          />
        </svg>
        <div class="text-center text-muted-foreground">
          <p class="text-lg font-medium">暂无媒体</p>
          <p class="mt-1 text-sm">在你的日记条目中上传图片</p>
        </div>
      </div>
    {:else}
      <!-- Timeline View -->
      <div class="space-y-8">
        {#each [...groupedMedia.entries()] as [dateKey, items], gi (dateKey)}
          <div class="animate-fade-in" style="animation-delay: {gi * 80}ms">
            <!-- Date Header -->
            <div class="mb-4 flex items-center gap-3">
              <div class="text-sm font-medium text-foreground">{formatDisplayDate(dateKey)}</div>
              <div class="h-px flex-1 bg-border/50"></div>
              <div class="text-xs text-muted-foreground">{items.length} 项</div>
            </div>

            <!-- Media Grid -->
            <div class="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
              {#each items as media (media.id)}
                <button
                  class="group relative aspect-square overflow-hidden rounded-xl border border-border/50 bg-muted/30 transition-all duration-200 hover:border-primary/50 hover:shadow-md"
                  onclick={() => openModal(media)}
                >
                  <img
                    src={getMediaFileUrl(media, '300x300')}
                    alt={media.alt || media.name || '媒体'}
                    class="h-full w-full object-cover transition-transform duration-300 group-hover:scale-105"
                    loading="lazy"
                  />
                  <!-- Overlay -->
                  <div
                    class="absolute inset-0 bg-gradient-to-t from-black/40 via-transparent to-transparent opacity-0 transition-opacity duration-200 group-hover:opacity-100"
                  ></div>
                  <!-- Diary indicator -->
                  {#if media.expand?.diary && media.expand.diary.length > 0}
                    <div
                      class="absolute bottom-2 left-2 rounded-lg bg-black/60 px-2 py-0.5 text-xs text-white backdrop-blur-sm"
                    >
                      {media.expand.diary[0].date?.split(' ')[0]}{media.expand.diary.length > 1
                        ? ` +${media.expand.diary.length - 1}`
                        : ''}
                    </div>
                  {/if}
                </button>
              {/each}
            </div>
          </div>
        {/each}
      </div>

      <!-- Pagination -->
      {#if totalPages > 1}
        <div class="mt-8 flex justify-center gap-2">
          <button
            disabled={currentPage === 1}
            onclick={() => {
              currentPage--;
              loadMedia();
            }}
            class="rounded-lg border border-border/50 px-3 py-1.5 text-sm transition-colors hover:bg-muted/50 disabled:cursor-not-allowed disabled:opacity-50"
          >
            上一页
          </button>
          <span class="px-3 py-1.5 text-sm text-muted-foreground">
            {currentPage} / {totalPages}
          </span>
          <button
            disabled={currentPage === totalPages}
            onclick={() => {
              currentPage++;
              loadMedia();
            }}
            class="rounded-lg border border-border/50 px-3 py-1.5 text-sm transition-colors hover:bg-muted/50 disabled:cursor-not-allowed disabled:opacity-50"
          >
            下一页
          </button>
        </div>
      {/if}
    {/if}
  </main>
</div>

<!-- Media Detail Modal -->
{#if showModal && selectedMedia}
  <div
    class="animate-fade-in fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm"
    onclick={handleModalBackdropClick}
    onkeydown={(e) => e.key === 'Escape' && closeModal()}
    role="dialog"
    tabindex="-1"
  >
    <div
      class="animate-scale-in max-h-[90vh] w-full max-w-3xl overflow-hidden rounded-2xl bg-card shadow-2xl"
      role="document"
    >
      <!-- Modal Header -->
      <div class="flex items-center justify-between border-b border-border/50 p-4">
        <h3 class="font-medium text-foreground">
          {selectedMedia.name || '媒体'}
        </h3>
        <button
          onclick={closeModal}
          class="rounded-lg p-1.5 transition-colors hover:bg-muted/50"
          title="关闭"
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

      <!-- Modal Content -->
      <div class="max-h-[calc(90vh-8rem)] overflow-y-auto p-4">
        <!-- Image -->
        <div class="mb-4 overflow-hidden rounded-lg bg-muted/30">
          <img
            src={getMediaFileUrl(selectedMedia)}
            alt={selectedMedia.alt || selectedMedia.name || '媒体'}
            class="h-auto max-h-[50vh] w-full object-contain"
          />
        </div>

        <!-- Info -->
        <div class="space-y-3 text-sm">
          <div class="flex items-center justify-between border-b border-border/30 py-2">
            <span class="text-muted-foreground">上传时间</span>
            <span class="text-foreground">
              {formatDisplayDate(selectedMedia.created || '')}
              {formatTime(selectedMedia.created || '')}
            </span>
          </div>

          {#if selectedMedia.expand?.diary && selectedMedia.expand.diary.length > 0}
            <div class="border-b border-border/30 py-2">
              <span class="text-muted-foreground">关联日记</span>
              <div class="mt-2 flex flex-wrap gap-2">
                {#each selectedMedia.expand.diary as diary (diary.id)}
                  <button
                    onclick={() => goToDiary(diary.date.split(' ')[0])}
                    class="rounded bg-primary/10 px-2 py-1 text-sm text-primary transition-colors hover:bg-primary/20"
                  >
                    {diary.date?.split(' ')[0]}
                  </button>
                {/each}
              </div>
            </div>
          {:else}
            <div class="flex items-center justify-between border-b border-border/30 py-2">
              <span class="text-muted-foreground">关联日记</span>
              <span class="text-muted-foreground/60">未关联</span>
            </div>
          {/if}
        </div>
      </div>

      <!-- Modal Footer -->
      <div class="flex items-center justify-between border-t border-border/50 bg-muted/20 p-4">
        {#if !showDeleteConfirm}
          <button
            onclick={() => (showDeleteConfirm = true)}
            class="rounded-lg border border-red-500 px-4 py-2 text-sm font-medium text-red-600 transition-colors hover:bg-red-50"
          >
            删除
          </button>
        {:else}
          <div class="flex items-center gap-3">
            {#if selectedMedia.expand?.diary && selectedMedia.expand.diary.length > 0}
              <span class="text-sm font-medium text-red-600"
                >此媒体关联了 {selectedMedia.expand.diary.length} 篇日记。确定删除吗？</span
              >
            {:else}
              <span class="text-sm font-medium text-red-600">确定删除吗？</span>
            {/if}
            <button
              onclick={handleDelete}
              disabled={deleting}
              class="rounded-lg border border-red-500 px-4 py-2 text-sm font-medium text-red-600 transition-colors hover:bg-red-50 disabled:opacity-50"
            >
              {deleting ? '删除中...' : '确认'}
            </button>
            <button
              onclick={() => (showDeleteConfirm = false)}
              class="rounded-lg border border-border px-4 py-2 text-sm font-medium transition-colors hover:bg-muted/50"
            >
              取消
            </button>
          </div>
        {/if}
        <button
          onclick={closeModal}
          class="rounded-lg border border-border/50 bg-muted px-4 py-2 text-sm transition-colors hover:bg-muted/80"
        >
          关闭
        </button>
      </div>
    </div>
  </div>
{/if}

<style>
  @keyframes scale-in {
    from {
      transform: scale(0.95);
      opacity: 0;
    }
    to {
      transform: scale(1);
      opacity: 1;
    }
  }

  .animate-scale-in {
    animation: scale-in 0.2s ease-out;
  }
</style>

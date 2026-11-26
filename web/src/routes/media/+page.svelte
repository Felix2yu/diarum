<script lang="ts">
  import { resolve } from '$app/paths';
  import { onDestroy, onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { isAuthenticated } from '#lib/api/client.js';
  import {
    getAllMedia,
    getTrashMedia,
    getUnreferencedMedia,
    getMediaDisplayUrl,
    deleteMediaById,
    restoreMediaById,
    permanentlyDeleteMediaById,
    emptyMediaTrash,
    type MediaWithDiary
  } from '#lib/api/media.js';
  import PageHeader from '#lib/components/ui/PageHeader.svelte';
  import { formatDisplayDate, formatTime } from '#lib/utils/date.js';
  import { openLightbox } from '#lib/stores/lightbox.js';

  let mediaList: MediaWithDiary[] = [];
  let loading = true;
  let currentPage = 1;
  let totalPages = 1;
  let totalItems = 0;
  let viewMode: 'active' | 'trash' | 'unreferenced' = 'active';

  // Modal state
  let selectedMedia: MediaWithDiary | null = null;
  let showModal = false;
  let deleting = false;
  let restoring = false;
  let showDeleteConfirm = false;
  let showPermanentConfirm = false;
  let showEmptyConfirm = false;
  let emptying = false;
  let showTrashAllConfirm = false;
  let trashingAll = false;

  let toastMessage = '';
  let toastTimer: ReturnType<typeof setTimeout> | undefined;
  let undeletableMedia: MediaWithDiary | null = null;
  let loadRequestId = 0;

  async function loadMedia() {
    const requestId = ++loadRequestId;
    loading = true;
    const result =
      viewMode === 'trash'
        ? await getTrashMedia(currentPage, 30)
        : viewMode === 'unreferenced'
          ? await getUnreferencedMedia(currentPage, 30)
          : await getAllMedia(currentPage, 30);
    if (requestId !== loadRequestId) return;
    mediaList = result.items;
    totalPages = result.totalPages;
    totalItems = result.totalItems;
    loading = false;
  }

  function switchView(mode: 'active' | 'trash' | 'unreferenced') {
    if (viewMode === mode) return;
    viewMode = mode;
    currentPage = 1;
    closeModal();
    showEmptyConfirm = false;
    showTrashAllConfirm = false;
    clearToast();
    loadMedia();
  }

  function showToast(message: string, undoMedia: MediaWithDiary | null = null) {
    clearToast();
    toastMessage = message;
    undeletableMedia = undoMedia;
    toastTimer = setTimeout(() => {
      toastMessage = '';
      undeletableMedia = null;
    }, 5000);
  }

  function clearToast() {
    if (toastTimer) {
      clearTimeout(toastTimer);
      toastTimer = undefined;
    }
    toastMessage = '';
    undeletableMedia = null;
  }

  async function undoDelete() {
    const mediaId = undeletableMedia?.id;
    if (!mediaId) return;
    clearToast();
    const success = await restoreMediaById(mediaId);
    if (success) {
      await loadMedia();
      showToast('已恢复到媒体库');
    }
  }

  function openModal(media: MediaWithDiary) {
    selectedMedia = media;
    showModal = true;
    showDeleteConfirm = false;
    showPermanentConfirm = false;
  }

  function closeModal() {
    showModal = false;
    selectedMedia = null;
    showDeleteConfirm = false;
    showPermanentConfirm = false;
  }

  function handleModalBackdropClick(e: MouseEvent) {
    if (e.target === e.currentTarget) {
      closeModal();
    }
  }

  function groupByDate(
    items: MediaWithDiary[],
    field: 'created' | 'deleted_at' = 'created'
  ): Map<string, MediaWithDiary[]> {
    const groups = new Map<string, MediaWithDiary[]>();
    for (const item of items) {
      const dateKey = item[field]?.split(' ')[0] || '';
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
    const media = selectedMedia;
    const success = await deleteMediaById(media.id!);
    deleting = false;

    if (success) {
      mediaList = mediaList.filter((m) => m.id !== media.id);
      totalItems = Math.max(0, totalItems - 1);
      closeModal();
      showToast('已移入回收站', media);
    }
  }

  async function handleRestore() {
    if (!selectedMedia?.id) return;
    restoring = true;
    const success = await restoreMediaById(selectedMedia.id);
    restoring = false;
    if (success) {
      mediaList = mediaList.filter((m) => m.id !== selectedMedia!.id);
      totalItems = Math.max(0, totalItems - 1);
      closeModal();
      showToast('已恢复到媒体库');
    }
  }

  async function handlePermanentDelete() {
    if (!selectedMedia?.id) return;
    deleting = true;
    const success = await permanentlyDeleteMediaById(selectedMedia.id);
    deleting = false;
    if (success) {
      mediaList = mediaList.filter((m) => m.id !== selectedMedia!.id);
      totalItems = Math.max(0, totalItems - 1);
      closeModal();
      showToast('已永久删除');
    }
  }

  async function handleEmptyTrash() {
    emptying = true;
    const result = await emptyMediaTrash();
    emptying = false;
    showEmptyConfirm = false;
    if (result) {
      currentPage = 1;
      await loadMedia();
      showToast(
        result.failed > 0
          ? `已删除 ${result.deleted} 项，${result.failed} 项删除失败`
          : `已清空回收站（${result.deleted} 项）`
      );
    }
  }

  async function handleTrashAllUnreferenced() {
    trashingAll = true;
    let moved = 0;
    for (const media of [...mediaList]) {
      if (!media.id) continue;
      if (await deleteMediaById(media.id)) moved++;
    }
    trashingAll = false;
    showTrashAllConfirm = false;
    currentPage = 1;
    await loadMedia();
    showToast(`已移入回收站 ${moved} 项`);
  }

  function goToDiary(date: string) {
    closeModal();
    goto(resolve('/diary/[date]', { date: date }));
  }

  function openMediaLightbox(media: MediaWithDiary) {
    openLightbox({
      src: getMediaDisplayUrl(media),
      alt: media.alt || media.name || '媒体'
    });
  }

  onMount(() => {
    if (!$isAuthenticated) {
      goto(resolve('/login'));
      return;
    }
    loadMedia();
  });

  onDestroy(() => {
    if (toastTimer) clearTimeout(toastTimer);
  });

  $: groupedMedia = groupByDate(mediaList, viewMode === 'trash' ? 'deleted_at' : 'created');
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

    <div class="mb-6 flex flex-wrap items-center justify-between gap-3">
      <div class="inline-flex rounded-lg bg-muted/50 p-1">
        <button
          type="button"
          class="rounded-md px-4 py-1.5 text-sm font-medium transition-colors {viewMode === 'active'
            ? 'bg-card text-foreground shadow-sm'
            : 'text-muted-foreground hover:text-foreground'}"
          onclick={() => switchView('active')}
        >
          媒体库
        </button>
        <button
          type="button"
          class="rounded-md px-4 py-1.5 text-sm font-medium transition-colors {viewMode === 'trash'
            ? 'bg-card text-foreground shadow-sm'
            : 'text-muted-foreground hover:text-foreground'}"
          onclick={() => switchView('trash')}
        >
          回收站{viewMode === 'trash' && totalItems > 0 ? ` (${totalItems})` : ''}
        </button>
        <button
          type="button"
          class="rounded-md px-4 py-1.5 text-sm font-medium transition-colors {viewMode ===
          'unreferenced'
            ? 'bg-card text-foreground shadow-sm'
            : 'text-muted-foreground hover:text-foreground'}"
          onclick={() => switchView('unreferenced')}
        >
          未引用
        </button>
      </div>

      {#if viewMode === 'trash' && totalItems > 0}
        <button
          type="button"
          class="rounded-lg border border-red-500/60 px-3 py-1.5 text-sm font-medium text-red-600 transition-colors hover:bg-red-50 dark:hover:bg-red-950/30"
          onclick={() => (showEmptyConfirm = true)}
        >
          清空回收站
        </button>
      {:else if viewMode === 'unreferenced' && totalItems > 0 && !showTrashAllConfirm}
        <button
          type="button"
          class="rounded-lg border border-border px-3 py-1.5 text-sm font-medium transition-colors hover:bg-muted/50"
          onclick={() => (showTrashAllConfirm = true)}
        >
          本页全部移入回收站
        </button>
      {/if}
    </div>

    {#if viewMode === 'trash'}
      <p class="mb-6 text-xs text-muted-foreground">回收站内的媒体会在 30 天后自动永久删除。</p>
    {:else if viewMode === 'unreferenced'}
      <p class="mb-6 text-xs text-muted-foreground">
        以下媒体没有被任何日记正文引用（例如上传后未保存正文），可移入回收站清理。
      </p>
    {/if}

    {#if showTrashAllConfirm}
      <div
        class="mb-6 flex flex-wrap items-center justify-between gap-3 rounded-xl border border-border bg-muted/40 p-4 text-sm"
      >
        <span class="text-foreground">
          会把当前页的 {mediaList.length} 项未引用媒体移入回收站，可在回收站恢复。
        </span>
        <div class="flex items-center gap-2">
          <button
            type="button"
            onclick={handleTrashAllUnreferenced}
            disabled={trashingAll}
            class="rounded-lg bg-primary px-3 py-1.5 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 disabled:opacity-50"
          >
            {trashingAll ? '处理中...' : '确认移入'}
          </button>
          <button
            type="button"
            onclick={() => (showTrashAllConfirm = false)}
            class="rounded-lg border border-border px-3 py-1.5 text-sm transition-colors hover:bg-muted/50"
          >
            取消
          </button>
        </div>
      </div>
    {/if}

    {#if showEmptyConfirm}
      <div
        class="mb-6 flex flex-wrap items-center justify-between gap-3 rounded-xl border border-red-500/40 bg-red-50/60 p-4 text-sm dark:bg-red-950/20"
      >
        <span class="text-red-700 dark:text-red-300">
          清空回收站会永久删除全部 {totalItems} 项媒体及其文件，无法恢复。
        </span>
        <div class="flex items-center gap-2">
          <button
            type="button"
            onclick={handleEmptyTrash}
            disabled={emptying}
            class="rounded-lg bg-red-600 px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-red-700 disabled:opacity-50"
          >
            {emptying ? '清空中...' : '确认清空'}
          </button>
          <button
            type="button"
            onclick={() => (showEmptyConfirm = false)}
            class="rounded-lg border border-border px-3 py-1.5 text-sm transition-colors hover:bg-muted/50"
          >
            取消
          </button>
        </div>
      </div>
    {/if}
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
          {#if viewMode === 'trash'}
            <p class="text-lg font-medium">回收站为空</p>
            <p class="mt-1 text-sm">删除的图片会先进入这里，可随时恢复</p>
          {:else if viewMode === 'unreferenced'}
            <p class="text-lg font-medium">没有未引用的媒体</p>
            <p class="mt-1 text-sm">媒体库里所有图片都已被日记正文引用</p>
          {:else}
            <p class="text-lg font-medium">暂无媒体</p>
            <p class="mt-1 text-sm">在你的日记条目中上传图片</p>
          {/if}
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
                    src={getMediaDisplayUrl(media, '300x300')}
                    alt={media.alt || media.name || '媒体'}
                    class="h-full w-full object-cover transition-transform duration-300 group-hover:scale-105"
                    loading="lazy"
                  />
                  <!-- Overlay -->
                  <div
                    class="absolute inset-0 bg-gradient-to-t from-black/40 via-transparent to-transparent opacity-0 transition-opacity duration-200 group-hover:opacity-100"
                  ></div>
                  {#if viewMode === 'trash'}
                    <div
                      class="absolute top-2 right-2 rounded bg-red-600/85 px-1.5 py-0.5 text-[10px] font-medium text-white backdrop-blur-sm"
                    >
                      回收站
                    </div>
                  {:else if viewMode === 'unreferenced'}
                    <div
                      class="absolute top-2 right-2 rounded bg-amber-600/85 px-1.5 py-0.5 text-[10px] font-medium text-white backdrop-blur-sm"
                    >
                      未引用
                    </div>
                  {/if}
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
        <button
          type="button"
          class="mb-4 block w-full cursor-zoom-in overflow-hidden rounded-lg bg-muted/30"
          aria-label="查看大图"
          onclick={() => {
            if (selectedMedia) openMediaLightbox(selectedMedia);
          }}
        >
          <img
            src={getMediaDisplayUrl(selectedMedia)}
            alt={selectedMedia.alt || selectedMedia.name || '媒体'}
            class="h-auto max-h-[50vh] w-full object-contain"
          />
        </button>

        <!-- Info -->
        <div class="space-y-3 text-sm">
          <div class="flex items-center justify-between border-b border-border/30 py-2">
            <span class="text-muted-foreground">上传时间</span>
            <span class="text-foreground">
              {formatDisplayDate(selectedMedia.created || '')}
              {formatTime(selectedMedia.created || '')}
            </span>
          </div>

          {#if viewMode === 'trash' && selectedMedia.deleted_at}
            <div class="flex items-center justify-between border-b border-border/30 py-2">
              <span class="text-muted-foreground">删除时间</span>
              <span class="text-foreground">
                {formatDisplayDate(selectedMedia.deleted_at)}
                {formatTime(selectedMedia.deleted_at)}
              </span>
            </div>
          {/if}

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
      <div
        class="flex flex-wrap items-center justify-between gap-3 border-t border-border/50 bg-muted/20 p-4"
      >
        <div class="flex flex-wrap items-center gap-3">
          {#if viewMode === 'trash'}
            <button
              onclick={handleRestore}
              disabled={restoring || deleting}
              class="rounded-lg border border-primary/60 bg-primary/10 px-4 py-2 text-sm font-medium text-primary transition-colors hover:bg-primary/20 disabled:opacity-50"
            >
              {restoring ? '恢复中...' : '恢复'}
            </button>
            {#if !showPermanentConfirm}
              <button
                onclick={() => (showPermanentConfirm = true)}
                class="rounded-lg border border-red-500 px-4 py-2 text-sm font-medium text-red-600 transition-colors hover:bg-red-50"
              >
                永久删除
              </button>
            {:else}
              <span class="text-sm font-medium text-red-600">
                永久删除后无法恢复{selectedMedia.expand?.diary?.length
                  ? `，且仍被 ${selectedMedia.expand.diary.length} 篇日记引用`
                  : ''}，确定吗？
              </span>
              <button
                onclick={handlePermanentDelete}
                disabled={deleting}
                class="rounded-lg border border-red-500 px-4 py-2 text-sm font-medium text-red-600 transition-colors hover:bg-red-50 disabled:opacity-50"
              >
                {deleting ? '删除中...' : '确认永久删除'}
              </button>
              <button
                onclick={() => (showPermanentConfirm = false)}
                class="rounded-lg border border-border px-4 py-2 text-sm font-medium transition-colors hover:bg-muted/50"
              >
                取消
              </button>
            {/if}
          {:else if !showDeleteConfirm}
            <button
              onclick={() => (showDeleteConfirm = true)}
              class="rounded-lg border border-red-500 px-4 py-2 text-sm font-medium text-red-600 transition-colors hover:bg-red-50"
            >
              移到回收站
            </button>
          {:else}
            <span class="text-sm font-medium text-red-600"> 移入回收站后可随时恢复，确定吗？ </span>
            <button
              onclick={handleDelete}
              disabled={deleting}
              class="rounded-lg border border-red-500 px-4 py-2 text-sm font-medium text-red-600 transition-colors hover:bg-red-50 disabled:opacity-50"
            >
              {deleting ? '处理中...' : '确认'}
            </button>
            <button
              onclick={() => (showDeleteConfirm = false)}
              class="rounded-lg border border-border px-4 py-2 text-sm font-medium transition-colors hover:bg-muted/50"
            >
              取消
            </button>
          {/if}
        </div>
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

{#if toastMessage}
  <div
    class="animate-fade-in fixed bottom-6 left-1/2 z-[120] flex -translate-x-1/2 items-center gap-4 rounded-xl bg-foreground px-4 py-2.5 text-sm text-background shadow-xl"
    role="status"
  >
    <span>{toastMessage}</span>
    {#if undeletableMedia}
      <button
        type="button"
        class="font-medium underline underline-offset-2 transition-opacity hover:opacity-80"
        onclick={undoDelete}
      >
        撤销
      </button>
    {/if}
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

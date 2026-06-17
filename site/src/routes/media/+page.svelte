<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { isAuthenticated } from '$lib/api/client';
	import { getAllMedia, getMediaFileUrl, deleteMediaById, type MediaWithDiary } from '$lib/api/media';
	import Footer from '$lib/components/ui/Footer.svelte';
	import PageHeader from '$lib/components/ui/PageHeader.svelte';
	import { formatDisplayDate, formatTime } from '$lib/utils/date';

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
			mediaList = mediaList.filter(m => m.id !== selectedMedia!.id);
			totalItems--;
			closeModal();
		}
	}

	function goToDiary(date: string) {
		closeModal();
		goto(`/diary/${date}`);
	}

	onMount(() => {
		if (!$isAuthenticated) {
			goto('/login');
			return;
		}
		loadMedia();
	});

	$: groupedMedia = groupByDate(mediaList);
</script>

<svelte:head>
	<title>媒体库 - 吾身</title>
</svelte:head>

<div class="min-h-screen bg-background">
	<PageHeader title="媒体库">
		<span slot="subtitle" class="text-sm text-muted-foreground">
			{#if !loading}({totalItems}){/if}
		</span>
	</PageHeader>

	<!-- Main Content -->
	<main class="container-responsive py-6">
		{#if loading}
			<div class="flex flex-col items-center justify-center py-20 gap-3">
				<svg class="w-6 h-6 animate-spin text-primary" fill="none" viewBox="0 0 24 24">
					<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
					<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
				</svg>
				<div class="text-muted-foreground text-sm">正在加载...</div>
			</div>
		{:else if mediaList.length === 0}
			<div class="flex flex-col items-center justify-center py-20 gap-4">
				<svg class="w-16 h-16 text-muted-foreground/30" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" />
				</svg>
				<div class="text-muted-foreground text-center">
					<p class="text-lg font-medium">暂无媒体</p>
					<p class="text-sm mt-1">在你的日记条目中上传图片</p>
				</div>
			</div>
		{:else}
			<!-- Timeline View -->
			<div class="space-y-8">
				{#each [...groupedMedia.entries()] as [dateKey, items]}
					<div class="animate-fade-in">
						<!-- Date Header -->
						<div class="flex items-center gap-3 mb-4">
							<div class="text-sm font-medium text-foreground">{formatDisplayDate(dateKey)}</div>
							<div class="flex-1 h-px bg-border/50"></div>
							<div class="text-xs text-muted-foreground">{items.length} 项</div>
						</div>

						<!-- Media Grid -->
						<div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 gap-3">
							{#each items as media}
								<button
									class="group relative aspect-square rounded-lg overflow-hidden bg-muted/30 border border-border/50 hover:border-primary/50 transition-all duration-200"
									on:click={() => openModal(media)}
								>
									<img
										src={getMediaFileUrl(media, '300x300')}
										alt={media.alt || media.name || '媒体'}
										class="w-full h-full object-cover group-hover:scale-105 transition-transform duration-200"
										loading="lazy"
									/>
									<!-- Overlay -->
									<div class="absolute inset-0 bg-black/0 group-hover:bg-black/20 transition-colors duration-200"></div>
									<!-- Diary indicator -->
									{#if media.expand?.diary && media.expand.diary.length > 0}
										<div class="absolute bottom-2 left-2 px-2 py-0.5 bg-black/60 rounded text-xs text-white">
											{media.expand.diary[0].date?.split(' ')[0]}{media.expand.diary.length > 1 ? ` +${media.expand.diary.length - 1}` : ''}
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
				<div class="flex justify-center gap-2 mt-8">
					<button
						disabled={currentPage === 1}
						on:click={() => { currentPage--; loadMedia(); }}
						class="px-3 py-1.5 text-sm rounded-lg border border-border/50 hover:bg-muted/50 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
					>
						上一页
					</button>
					<span class="px-3 py-1.5 text-sm text-muted-foreground">
						{currentPage} / {totalPages}
					</span>
					<button
						disabled={currentPage === totalPages}
						on:click={() => { currentPage++; loadMedia(); }}
						class="px-3 py-1.5 text-sm rounded-lg border border-border/50 hover:bg-muted/50 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
					>
						下一页
					</button>
				</div>
			{/if}
		{/if}
	</main>

	<Footer tagline="你的媒体库" />
</div>

<!-- Media Detail Modal -->
{#if showModal && selectedMedia}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm animate-fade-in"
		on:click={handleModalBackdropClick}
		on:keydown={(e) => e.key === 'Escape' && closeModal()}
		role="dialog"
		tabindex="-1"
	>
		<div
			class="bg-card rounded-xl shadow-xl max-w-3xl w-full max-h-[90vh] overflow-hidden animate-scale-in"
			role="document"
		>
			<!-- Modal Header -->
			<div class="flex items-center justify-between p-4 border-b border-border/50">
				<h3 class="font-medium text-foreground">
					{selectedMedia.name || '媒体'}
				</h3>
				<button
					on:click={closeModal}
					class="p-1.5 hover:bg-muted/50 rounded-lg transition-colors"
					title="关闭"
				>
					<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
					</svg>
				</button>
			</div>

			<!-- Modal Content -->
			<div class="p-4 overflow-y-auto max-h-[calc(90vh-8rem)]">
				<!-- Image -->
				<div class="rounded-lg overflow-hidden bg-muted/30 mb-4">
					<img
						src={getMediaFileUrl(selectedMedia)}
						alt={selectedMedia.alt || selectedMedia.name || '媒体'}
						class="w-full h-auto max-h-[50vh] object-contain"
					/>
				</div>

				<!-- Info -->
				<div class="space-y-3 text-sm">
					<div class="flex items-center justify-between py-2 border-b border-border/30">
						<span class="text-muted-foreground">上传时间</span>
						<span class="text-foreground">
							{formatDisplayDate(selectedMedia.created || '')} {formatTime(selectedMedia.created || '')}
						</span>
					</div>

					{#if selectedMedia.expand?.diary && selectedMedia.expand.diary.length > 0}
						<div class="py-2 border-b border-border/30">
							<span class="text-muted-foreground">关联日记</span>
							<div class="flex flex-wrap gap-2 mt-2">
								{#each selectedMedia.expand.diary as diary}
									<button
										on:click={() => goToDiary(diary.date.split(' ')[0])}
										class="px-2 py-1 text-sm bg-primary/10 text-primary rounded hover:bg-primary/20 transition-colors"
									>
										{diary.date?.split(' ')[0]}
									</button>
								{/each}
							</div>
						</div>
					{:else}
						<div class="flex items-center justify-between py-2 border-b border-border/30">
							<span class="text-muted-foreground">关联日记</span>
							<span class="text-muted-foreground/60">未关联</span>
						</div>
					{/if}
				</div>
			</div>

			<!-- Modal Footer -->
			<div class="flex items-center justify-between p-4 border-t border-border/50 bg-muted/20">
				{#if !showDeleteConfirm}
					<button
						on:click={() => showDeleteConfirm = true}
						class="px-4 py-2 text-sm text-red-600 border border-red-500 hover:bg-red-50 rounded-lg transition-colors font-medium"
					>
						删除
					</button>
				{:else}
					<div class="flex items-center gap-3">
						{#if selectedMedia.expand?.diary && selectedMedia.expand.diary.length > 0}
							<span class="text-sm text-red-600 font-medium">此媒体关联了 {selectedMedia.expand.diary.length} 篇日记。确定删除吗？</span>
						{:else}
							<span class="text-sm text-red-600 font-medium">确定删除吗？</span>
						{/if}
						<button
							on:click={handleDelete}
							disabled={deleting}
							class="px-4 py-2 text-sm text-red-600 border border-red-500 hover:bg-red-50 disabled:opacity-50 rounded-lg transition-colors font-medium"
						>
							{deleting ? '删除中...' : '确认'}
						</button>
						<button
							on:click={() => showDeleteConfirm = false}
							class="px-4 py-2 text-sm border border-border hover:bg-muted/50 rounded-lg transition-colors font-medium"
						>
							取消
						</button>
					</div>
				{/if}
				<button
					on:click={closeModal}
					class="px-4 py-2 text-sm bg-muted hover:bg-muted/80 rounded-lg transition-colors border border-border/50"
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

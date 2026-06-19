<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { onlineState } from '$lib/stores/onlineStatus';
	import { syncState, cacheStats, forceSyncNow } from '$lib/stores/diaryCache';
	import { checkOnlineStatus } from '$lib/stores/onlineStatus';

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
			// 只有在离线或状态有问题时才让整个 banner 可见，
			// 在线 + 空闲状态时隐藏，避免干扰用户
			visible = !s.isOnline;
		});

		unsubSync = syncState.subscribe((s) => {
			isSyncing = s.isSyncing;
			syncMessage = s.message;
		});

		unsubStats = cacheStats.subscribe((s) => {
			pendingSyncCount = s.pendingSync;
		});

		// 启动时立即检查一次
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
		class="fixed top-0 left-0 right-0 z-40 bg-yellow-600 text-white text-sm px-4 py-2 flex items-center justify-center gap-3 shadow-md"
	>
		<svg
			class="w-4 h-4 text-white animate-pulse"
			fill="none"
			stroke="currentColor"
			viewBox="0 0 24 24"
		>
			<path
				stroke-linecap="round"
				stroke-linejoin="round"
				stroke-width="2"
				d="M8.111 16.404a5.5 5.5 0 017.778 0M12 20h.01m-7.08-7.071c3.904-3.905 10.236-3.905 14.14 0M1.394 9.393c5.857-5.857 15.355-5.857 21.213 0"
			/>
		</svg>
		<span class="font-medium">当前离线 —— 内容已自动保存到本地</span>
		{#if pendingSyncCount > 0}
			<span class="opacity-80">（{pendingSyncCount} 条等待同步）</span>
		{/if}
		<button
			type="button"
			onclick={handleSync}
			disabled={syncingNow || pendingSyncCount === 0}
			class="ml-2 px-3 py-1 bg-white/20 hover:bg-white/30 rounded-md transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
		>
			{syncingNow ? '同步中...' : '立即同步'}
		</button>
	</div>
	<!-- 顶部 banner 占据空间，避免遮挡导航栏内容 -->
	<div class="h-10" aria-hidden="true" />
{/if}

<!-- 同步状态指示器（底部）：仅在同步或有明确消息时显示 -->
{#if (isSyncing || syncMessage) && !visible}
	<div
		class="fixed bottom-4 right-4 z-30 text-xs rounded-md px-3 py-2 shadow-md {isSyncing
			? 'bg-blue-50 text-blue-800 dark:bg-blue-900 dark:text-blue-100'
			: 'bg-gray-800 text-white'}"
	>
		{#if isSyncing}
			<span class="inline-flex items-center gap-2">
				<svg
					class="w-3 h-3 animate-spin"
					fill="none"
					stroke="currentColor"
					viewBox="0 0 24 24"
				>
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v6l4 2" />
				</svg>
				{syncMessage || '正在同步'}
			</span>
		{:else}
			{syncMessage}
		{/if}
	</div>
{/if}

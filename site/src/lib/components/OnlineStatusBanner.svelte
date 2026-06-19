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
	<div class="fixed top-0 left-0 right-0 z-40">
		<div class="bg-accent/80 backdrop-blur-md border-b border-border/50 text-foreground/70 text-sm px-4 py-2 flex items-center justify-center gap-3">
			<svg
				class="w-4 h-4 animate-pulse-subtle"
				fill="none"
				stroke="currentColor"
				viewBox="0 0 24 24"
			>
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
				on:click={handleSync}
				disabled={syncingNow || pendingSyncCount === 0}
				class="ml-2 px-3 py-1 bg-secondary text-secondary-foreground rounded-md hover:opacity-90 transition-opacity disabled:opacity-50 disabled:cursor-not-allowed"
			>
				{syncingNow ? '同步中...' : '立即同步'}
			</button>
		</div>
	</div>
	<div class="h-10" aria-hidden="true" />
{/if}

{#if (isSyncing || syncMessage) && !visible}
	<div class="fixed bottom-4 right-4 z-30 text-xs rounded-lg px-3 py-2 shadow-md border border-border/50 bg-card/90">
		{#if isSyncing}
			<span class="inline-flex items-center gap-2 text-muted-foreground">
				<svg
					class="w-3 h-3 animate-spin" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v6l4 2" />
				</svg>
				{syncMessage || '正在同步'}
			</span>
		{:else}
			<span class="text-muted-foreground">{syncMessage}</span>
		{/if}
	</div>
{/if}

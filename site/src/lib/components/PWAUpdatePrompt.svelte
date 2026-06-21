<script lang="ts">
	import { isUpdateAvailable, isApplyingUpdate, applyUpdate } from '$lib/utils/pwa';
	import { onMount } from 'svelte';

	const DISMISS_KEY = 'pwa_update_dismissed_at';
	const DISMISS_DURATION_MS = 24 * 60 * 60 * 1000; // 24 小时不打扰

	let showUpdate = false;
	let applying = false;

	function shouldShow(): boolean {
		if (typeof window === 'undefined') return true;
		try {
			const raw = window.localStorage.getItem(DISMISS_KEY);
			if (!raw) return true;
			const dismissedAt = Number(raw);
			if (!Number.isFinite(dismissedAt)) return true;
			return Date.now() - dismissedAt > DISMISS_DURATION_MS;
		} catch {
			return true;
		}
	}

	function rememberDismiss() {
		try {
			window.localStorage.setItem(DISMISS_KEY, String(Date.now()));
		} catch {
			// ignore storage errors
		}
	}

	onMount(() => {
		const unsubUpdate = isUpdateAvailable.subscribe((v) => {
			showUpdate = v && shouldShow();
		});
		const unsubApplying = isApplyingUpdate.subscribe((v) => {
			applying = v;
		});
		return () => {
			unsubUpdate();
			unsubApplying();
		};
	});

	async function handleUpdate() {
		await applyUpdate();
	}

	function dismiss() {
		showUpdate = false;
		isUpdateAvailable.set(false);
		rememberDismiss();
	}
</script>

{#if showUpdate}
	<div class="fixed top-4 left-4 right-4 md:left-auto md:right-4 md:w-96 z-50 animate-fade-in">
		<div class="bg-card glass rounded-xl shadow-xl border border-border/50 p-4">
			<div class="flex items-start gap-3">
				<div class="flex-shrink-0">
					<svg
						class="w-5 h-5 text-primary animate-pulse-subtle"
						fill="none"
						stroke="currentColor"
						viewBox="0 0 24 24"
					>
						<path
							stroke-linecap="round"
							stroke-linejoin="round"
							stroke-width="2"
							d="M7 16V8a5 5 0 0110 0v8a3 3 0 01-3 3H10a3 3 0 01-3-3zM7 12h10M12 8v4"
						/>
					</svg>
				</div>
				<div class="flex-1 min-w-0">
					<h3 class="text-sm font-semibold text-foreground">发现新版本</h3>
					<p class="mt-1 text-sm text-muted-foreground">
						点击更新以获取最新功能和改进
					</p>
					<div class="mt-3 flex items-center gap-2">
						<button
							type="button"
							onclick={handleUpdate}
							disabled={applying}
							class="inline-flex items-center gap-2 px-3 py-2 text-sm bg-primary text-primary-foreground rounded-lg hover:opacity-90 transition-opacity disabled:opacity-50 disabled:cursor-not-allowed"
						>
							{#if applying}
								<svg class="w-4 h-4 animate-spin" fill="none" stroke="currentColor" viewBox="0 0 24 24">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v6l4 2" />
								</svg>
								正在更新...
							{:else}
								立即更新
							{/if}
						</button>
						<button
							type="button"
							onclick={dismiss}
							disabled={applying}
							class="px-3 py-2 text-sm bg-secondary text-secondary-foreground rounded-lg hover:opacity-90 transition-opacity disabled:opacity-50 disabled:cursor-not-allowed"
						>
							稍后
						</button>
					</div>
				</div>
			</div>
		</div>
	</div>
{/if}

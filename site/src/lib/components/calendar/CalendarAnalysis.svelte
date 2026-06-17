<script lang="ts">
	import { analyzePeriod, type PeriodAnalysisResult } from '$lib/api/ai';

	let {
		period,
		start,
		end,
		onClose
	}: {
		period: 'week' | 'month';
		start: string;
		end: string;
		onClose: () => void;
	} = $props();

	let loading = $state(false);
	let error: string | null = $state(null);
	let result: PeriodAnalysisResult | null = $state(null);

	async function runAnalysis() {
		loading = true;
		error = null;
		result = null;
		try {
			result = await analyzePeriod(period, start, end);
		} catch (e: unknown) {
			if (e instanceof Error) {
				error = e.message;
			} else {
				error = 'AI 分析失败';
			}
		} finally {
			loading = false;
		}
	}

	const label = period === 'week' ? '本周分析' : '本月分析';

	$effect(() => {
		runAnalysis();
	});

	function formatSummary(text: string): string {
		return text.trim();
	}
</script>

<div class="analysis-overlay" role="dialog" aria-label={label} onclick={onClose}>
	<div class="analysis-panel" onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.key === 'Escape' && onClose()}>
		<div class="analysis-header">
			<h3>{label}</h3>
			<span class="analysis-range">{start} ~ {end}</span>
			<button class="analysis-close" onclick={onClose} aria-label="关闭">×</button>
		</div>

		<div class="analysis-body">
			{#if loading}
				<div class="analysis-loading">
					<div class="spinner" aria-hidden="true"></div>
					<p>正在分析日记内容…</p>
				</div>
			{:else if error}
				<div class="analysis-error">
					<p>{error}</p>
					<button class="analysis-retry" onclick={runAnalysis}>重试</button>
				</div>
			{:else if result}
				<div class="analysis-meta">共 {result.count} 篇日记</div>
				<div class="analysis-summary">
					{#each formatSummary(result.summary).split('\n') as line}
						{#if line.trim() !== ''}
							<p>{line}</p>
						{/if}
					{/each}
				</div>
			{/if}
		</div>
	</div>
</div>

<style>
	.analysis-overlay {
		position: fixed;
		inset: 0;
		background: hsl(var(--background) / 0.6);
		backdrop-filter: blur(6px);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 50;
		padding: 1rem;
		animation: fade-in 0.15s ease-out;
	}

	.analysis-panel {
		background: hsl(var(--card));
		border: 1px solid hsl(var(--border) / 0.6);
		border-radius: 1rem;
		width: 100%;
		max-width: 640px;
		max-height: 80vh;
		display: flex;
		flex-direction: column;
		box-shadow: 0 20px 50px hsl(0 0% 0% / 0.15);
		animation: panel-in 0.2s ease-out;
	}

	.analysis-header {
		padding: 1rem 1.25rem;
		border-bottom: 1px solid hsl(var(--border) / 0.5);
		display: flex;
		align-items: center;
		gap: 0.75rem;
		position: relative;
	}

	.analysis-header h3 {
		margin: 0;
		font-size: 1.05rem;
		font-weight: 600;
		color: hsl(var(--foreground));
	}

	.analysis-range {
		color: hsl(var(--muted-foreground));
		font-size: 0.85rem;
	}

	.analysis-close {
		margin-left: auto;
		width: 2rem;
		height: 2rem;
		border-radius: 9999px;
		border: none;
		background: transparent;
		color: hsl(var(--muted-foreground));
		font-size: 1.25rem;
		cursor: pointer;
		transition: background 0.15s ease;
	}

	.analysis-close:hover {
		background: hsl(var(--muted) / 0.6);
		color: hsl(var(--foreground));
	}

	.analysis-body {
		padding: 1.25rem;
		overflow-y: auto;
		flex: 1;
	}

	.analysis-loading,
	.analysis-error {
		padding: 2rem 1rem;
		text-align: center;
		color: hsl(var(--muted-foreground));
	}

	.analysis-loading p {
		margin-top: 0.75rem;
	}

	.spinner {
		width: 1.75rem;
		height: 1.75rem;
		border: 2px solid hsl(var(--muted));
		border-top-color: hsl(var(--primary));
		border-radius: 9999px;
		animation: spin 0.8s linear infinite;
		margin: 0 auto;
	}

	.analysis-error p {
		margin-bottom: 1rem;
		color: hsl(var(--destructive) / 0.9);
	}

	.analysis-retry {
		padding: 0.5rem 1.25rem;
		border-radius: 0.5rem;
		border: 1px solid hsl(var(--primary) / 0.3);
		background: hsl(var(--primary) / 0.08);
		color: hsl(var(--primary));
		cursor: pointer;
		font-size: 0.875rem;
	}

	.analysis-retry:hover {
		background: hsl(var(--primary) / 0.15);
	}

	.analysis-meta {
		font-size: 0.8rem;
		color: hsl(var(--muted-foreground));
		margin-bottom: 0.75rem;
	}

	.analysis-summary {
		line-height: 1.75;
		color: hsl(var(--foreground) / 0.9);
		font-size: 0.95rem;
	}

	.analysis-summary p {
		margin: 0 0 0.75rem;
	}

	@keyframes fade-in {
		from { opacity: 0; }
		to { opacity: 1; }
	}

	@keyframes panel-in {
		from { opacity: 0; transform: translateY(10px) scale(0.98); }
		to { opacity: 1; transform: translateY(0) scale(1); }
	}

	@keyframes spin {
		from { transform: rotate(0deg); }
		to { transform: rotate(360deg); }
	}
</style>

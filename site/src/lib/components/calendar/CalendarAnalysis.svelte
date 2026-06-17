<script lang="ts">
	import { analyzePeriod, DEFAULT_ANALYSIS_SYSTEM_PROMPT, type PeriodAnalysisResult } from '$lib/api/ai';

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

	type Stage = 'idle' | 'loading' | 'ready' | 'error';
	let stage: Stage = $state('idle');
	let result: PeriodAnalysisResult | null = $state(null);
	let errorMsg: string | null = $state(null);
	let showPromptEditor = $state(false);
	let systemPrompt = $state(DEFAULT_ANALYSIS_SYSTEM_PROMPT);
	let userPrefix = $state('');

	async function runAnalysis() {
		stage = 'loading';
		errorMsg = null;
		result = null;
		try {
			result = await analyzePeriod(period, start, end, {
				system_prompt: systemPrompt,
				user_prefix: userPrefix
			});
			stage = 'ready';
		} catch (e: unknown) {
			errorMsg = e instanceof Error ? e.message : 'AI 分析失败';
			stage = 'error';
		}
	}

	const label = period === 'week' ? '本周分析' : '本月分析';

	function useDefaultPrompt() {
		systemPrompt = DEFAULT_ANALYSIS_SYSTEM_PROMPT;
		userPrefix = '';
	}

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

		<div class="analysis-toolbar">
			<button
				onclick={() => (showPromptEditor = !showPromptEditor)}
				class="analysis-toggle"
			>
				{showPromptEditor ? '收起提示词' : '编辑提示词'}
			</button>
			<button onclick={useDefaultPrompt} class="analysis-toggle">恢复默认</button>
			<button onclick={runAnalysis} disabled={stage === 'loading'} class="analysis-reanalyze">
				{stage === 'loading' ? '分析中…' : '开始分析'}
			</button>
		</div>

		{#if showPromptEditor}
			<div class="analysis-prompt">
				<label for="cas-system-prompt" class="analysis-prompt-label">系统提示词</label>
				<textarea
					id="cas-system-prompt"
					rows={5}
					bind:value={systemPrompt}
					placeholder="留空则使用系统默认提示词"
					class="analysis-prompt-textarea"
				/>
				<label for="cas-user-prefix" class="analysis-prompt-label analysis-prompt-label--indented">内容引导语 (可选)</label>
				<textarea
					id="cas-user-prefix"
					rows={3}
					bind:value={userPrefix}
					placeholder="留空则使用默认的周/月格式化提示语"
					class="analysis-prompt-textarea"
				/>
				<p class="analysis-prompt-hint">修改后点击"重新分析"以应用提示词；保存为持久默认请前往设置 → AI 助手。</p>
			</div>
		{/if}

		<div class="analysis-body">
			{#if stage === 'idle'}
				<div class="analysis-idle">
					<p class="analysis-idle-title">准备开始 AI 分析</p>
					<p class="analysis-idle-sub">
						系统将基于此时间段的日记内容生成一份结构化的总结与建议。你可先点击上方
						"编辑提示词" 自定义分析风格，然后点击"开始分析"。
					</p>
				</div>
			{:else if stage === 'loading'}
				<div class="analysis-loading">
					<div class="spinner" aria-hidden="true"></div>
					<p>正在分析日记内容…</p>
				</div>
			{:else if stage === 'error'}
				<div class="analysis-error">
					<p>{errorMsg}</p>
					<button class="analysis-retry" onclick={runAnalysis}>重试</button>
				</div>
			{:else if stage === 'ready' && result}
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
		background: hsl(var(--background) / 0.7);
		backdrop-filter: blur(6px);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 2147483647;
		padding: 1rem;
		animation: fade-in 0.15s ease-out;
	}

	.analysis-panel {
		background: hsl(var(--card));
		border: 1px solid hsl(var(--border) / 0.6);
		border-radius: 1rem;
		width: 100%;
		max-width: 680px;
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

	.analysis-toolbar {
		padding: 0.75rem 1.25rem;
		display: flex;
		gap: 0.5rem;
		flex-wrap: wrap;
		border-bottom: 1px solid hsl(var(--border) / 0.4);
	}

	.analysis-toggle {
		padding: 0.4rem 0.75rem;
		font-size: 0.8rem;
		border: 1px solid hsl(var(--border) / 0.7);
		background: hsl(var(--muted) / 0.3);
		color: hsl(var(--foreground) / 0.85);
		border-radius: 0.5rem;
		cursor: pointer;
		transition: background 0.15s ease, color 0.15s ease;
	}

	.analysis-toggle:hover {
		background: hsl(var(--muted) / 0.7);
		color: hsl(var(--foreground));
	}

	.analysis-reanalyze {
		padding: 0.4rem 0.85rem;
		font-size: 0.8rem;
		margin-left: auto;
		border: 1px solid hsl(var(--primary) / 0.3);
		background: hsl(var(--primary) / 0.1);
		color: hsl(var(--primary));
		border-radius: 0.5rem;
		cursor: pointer;
		font-weight: 500;
		transition: background 0.15s ease;
	}

	.analysis-reanalyze:hover:not(:disabled) {
		background: hsl(var(--primary) / 0.2);
	}

	.analysis-reanalyze:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.analysis-prompt {
		padding: 0.85rem 1.25rem 1rem;
		background: hsl(var(--muted) / 0.25);
		border-bottom: 1px solid hsl(var(--border) / 0.4);
	}

	.analysis-prompt-label {
		display: block;
		font-size: 0.78rem;
		color: hsl(var(--muted-foreground));
		margin-bottom: 0.35rem;
	}

	.analysis-prompt-label--indented {
		margin-top: 0.75rem;
	}

	.analysis-prompt-textarea {
		width: 100%;
		padding: 0.5rem 0.65rem;
		font-size: 0.82rem;
		line-height: 1.6;
		background: hsl(var(--background));
		border: 1px solid hsl(var(--border) / 0.7);
		color: hsl(var(--foreground));
		border-radius: 0.5rem;
		resize: vertical;
		font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
		outline: none;
		transition: border-color 0.15s ease, box-shadow 0.15s ease;
	}

	.analysis-prompt-textarea:focus {
		border-color: hsl(var(--primary) / 0.8);
		box-shadow: 0 0 0 2px hsl(var(--primary) / 0.15);
	}

	.analysis-prompt-hint {
		font-size: 0.72rem;
		color: hsl(var(--muted-foreground));
		margin: 0.6rem 0 0;
	}

	.analysis-body {
		padding: 1.25rem;
		overflow-y: auto;
		flex: 1;
	}

	.analysis-loading,
	.analysis-idle,
	.analysis-error {
		padding: 2rem 1rem;
		text-align: center;
		color: hsl(var(--muted-foreground));
	}

	.analysis-idle-title {
		margin: 0 0 0.5rem;
		font-size: 1rem;
		font-weight: 600;
		color: hsl(var(--foreground));
	}

	.analysis-idle-sub {
		margin: 0;
		font-size: 0.9rem;
		line-height: 1.6;
		max-width: 36rem;
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

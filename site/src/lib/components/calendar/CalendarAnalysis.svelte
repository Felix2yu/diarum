<script lang="ts">
	import {
		analyzePeriod,
		getSavedAnalysis,
		getSavedAnalyses,
		DEFAULT_ANALYSIS_SYSTEM_PROMPT,
		type PeriodAnalysisResult,
		type SavedPeriodAnalysisResult
	} from '$lib/api/ai';

	let {
		mode = 'single',
		period,
		start,
		end,
		onClose
	}: {
		mode?: 'single' | 'history';
		period: 'week' | 'month';
		start: string;
		end: string;
		onClose: () => void;
	} = $props();

	// ------- 通用状态 -------
	type Stage = 'checking' | 'idle' | 'loading' | 'ready' | 'error' | 'list-loading' | 'list-ready' | 'list-error';
	let stage: Stage = $state('checking');
	let errorMsg: string | null = $state(null);

	// ------- 单条分析视图 -------
	let result: PeriodAnalysisResult | null = $state(null);
	let savedLabel: string | null = $state(null);
	let showPromptEditor = $state(false);
	let systemPrompt = $state(DEFAULT_ANALYSIS_SYSTEM_PROMPT);
	let userPrefix = $state('');

	// ------- 历史列表视图 -------
	type Filter = 'all' | 'week' | 'month';
	let filter: Filter = $state('all');
	let savedList: SavedPeriodAnalysisResult[] = $state([]);
	let selected: SavedPeriodAnalysisResult | null = $state(null);

	let overlayEl: HTMLDivElement | null = $state(null);
	let hostEl: HTMLDivElement | null = null;

	// Portal: 将弹窗挂载到 document.body，脱离日历容器的堆叠上下文
	$effect(() => {
		if (!overlayEl) return;
		if (!hostEl) {
			hostEl = document.createElement('div');
			hostEl.style.position = 'static';
		}
		document.body.appendChild(hostEl);
		hostEl.appendChild(overlayEl);
		return () => {
			if (hostEl && hostEl.parentNode) {
				hostEl.parentNode.removeChild(hostEl);
			}
			if (overlayEl && overlayEl.parentNode) {
				overlayEl.parentNode.removeChild(overlayEl);
			}
			hostEl = null;
		};
	});

	function resetSingleState() {
		result = null;
		savedLabel = null;
		systemPrompt = DEFAULT_ANALYSIS_SYSTEM_PROMPT;
		userPrefix = '';
		showPromptEditor = false;
	}

	// 打开时：根据 mode 决定加载什么内容
	async function tryLoadSingle(per: 'week' | 'month', s: string, e: string) {
		resetSingleState();
		stage = 'checking';
		errorMsg = null;
		try {
			const saved = await getSavedAnalysis(per, s, e);
			if (saved) {
				result = saved;
				if (saved.system_prompt) systemPrompt = saved.system_prompt;
				if (saved.user_prefix) userPrefix = saved.user_prefix;
				savedLabel = saved.updated
					? `已保存 · ${saved.updated.replace('T', ' ').slice(0, 19)}`
					: '已保存';
				stage = 'ready';
				return;
			}
			stage = 'idle';
		} catch (e: unknown) {
			stage = 'idle';
		}
	}

	async function loadList(per: Filter) {
		stage = 'list-loading';
		errorMsg = null;
		selected = null;
		filter = per;
		try {
			const items = await getSavedAnalyses(per === 'all' ? undefined : per);
			savedList = items;
			stage = 'list-ready';
		} catch (e: unknown) {
			errorMsg = e instanceof Error ? e.message : '获取历史分析失败';
			stage = 'list-error';
		}
	}

	// 根据 mode 初始化加载（组件挂载/切换 mode 时执行一次；避免监听 stage 以免循环）
	$effect(() => {
		if (mode === 'history') {
			// 只在初次挂载且尚未加载过时调用，避免 effect 依赖追踪导致循环
			loadList(filter);
		} else {
			tryLoadSingle(period, start, end);
		}
	});

	// ESC 关闭
	function onKey(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			if (selected) {
				selected = null;
			} else {
				onClose();
			}
		}
	}

	// ------- 单条分析逻辑 -------
	async function runAnalysis(per: 'week' | 'month', s: string, e: string) {
		stage = 'loading';
		errorMsg = null;
		result = null;
		savedLabel = null;
		try {
			const r = await analyzePeriod(per, s, e, {
				system_prompt: systemPrompt,
				user_prefix: userPrefix
			});
			result = r;
			if (r?.id) {
				savedLabel = r.updated
					? `已保存 · ${r.updated.replace('T', ' ').slice(0, 19)}`
					: '已保存';
			}
			stage = 'ready';
		} catch (e: unknown) {
			errorMsg = e instanceof Error ? e.message : 'AI 分析失败';
			stage = 'error';
		}
	}

	function useDefaultPrompt() {
		systemPrompt = DEFAULT_ANALYSIS_SYSTEM_PROMPT;
		userPrefix = '';
	}

	function formatSummary(text: string): string {
		return text.trim();
	}

	function openSaved(item: SavedPeriodAnalysisResult) {
		// 在弹窗内切换到单条分析视图，展示已保存内容
		result = item;
		if (item.system_prompt) systemPrompt = item.system_prompt;
		if (item.user_prefix) userPrefix = item.user_prefix;
		savedLabel = item.updated
			? `已保存 · ${item.updated.replace('T', ' ').slice(0, 19)}`
			: '已保存';
		selected = item;
		stage = 'ready';
	}

	function backToList() {
		result = null;
		savedLabel = null;
		selected = null;
		// 恢复列表状态，不重新请求网络，savedList 已在内存
		stage = 'list-ready';
	}

	function periodTitle(per: string): string {
		return per === 'week' ? '周分析' : per === 'month' ? '月分析' : '分析';
	}

	const mainLabel =
		mode === 'history' ? '历史分析' : period === 'week' ? '本周分析' : '本月分析';

	// 绑定到根元素引用
	function setRef(node: HTMLDivElement | null) {
		overlayEl = node;
	}
</script>

<div
	use:setRef
	role="dialog"
	aria-label={mainLabel}
	class="analysis-overlay"
	onclick={onClose}
	onkeydown={onKey}
>
	<div class="analysis-panel" onclick={(e) => e.stopPropagation()}>
		<div class="analysis-header">
			<h3>{mainLabel}</h3>
			<span class="analysis-range">
				{#if mode === 'history'}
					{#if selected}
						{selected.start} ~ {selected.end}
						{#if selected.updated}
							<span class="analysis-saved-badge" title="该分析已保存">
								已保存 · {selected.updated.replace('T', ' ').slice(0, 19)}
							</span>
						{/if}
					{:else}
						共 {savedList.length} 份历史分析
					{/if}
				{:else}
					{start} ~ {end}
					{#if savedLabel}
						<span class="analysis-saved-badge" title="该分析已保存">{savedLabel}</span>
					{/if}
				{/if}
			</span>
			<button class="analysis-close" onclick={onClose} aria-label="关闭">×</button>
		</div>

		<!-- ---- 历史列表模式 ---- -->
		{#if mode === 'history'}
			{#if !selected}
				<div class="analysis-toolbar">
					<button
						class={filter === 'all' ? 'analysis-toggle analysis-toggle--active' : 'analysis-toggle'}
						onclick={() => loadList('all')}
					>全部</button>
					<button
						class={filter === 'week' ? 'analysis-toggle analysis-toggle--active' : 'analysis-toggle'}
						onclick={() => loadList('week')}
					>周分析</button>
					<button
						class={filter === 'month' ? 'analysis-toggle analysis-toggle--active' : 'analysis-toggle'}
						onclick={() => loadList('month')}
					>月分析</button>
					<button onclick={() => loadList(filter)} class="analysis-toggle ml-auto" title="刷新">
						刷新
					</button>
				</div>

				<div class="analysis-body">
					{#if stage === 'list-loading'}
						<div class="analysis-loading">
							<div class="spinner" aria-hidden="true"></div>
							<p>正在加载历史分析…</p>
						</div>
					{:else if stage === 'list-error'}
						<div class="analysis-error">
							<p>{errorMsg}</p>
							<button class="analysis-retry" onclick={() => loadList(filter)}>重试</button>
						</div>
					{:else if savedList.length === 0}
						<div class="analysis-idle">
							<p class="analysis-idle-title">暂无已保存的分析</p>
							<p class="analysis-idle-sub">
								返回日历界面点击"周分析"或"月分析"，生成分析后将自动保存到此处。
							</p>
						</div>
					{:else}
						<ul class="analysis-list">
							{#each savedList as item (item.id)}
								<li class="analysis-list-item" onclick={() => openSaved(item)}>
									<div class="analysis-list-head">
										<span class="analysis-list-tag" data-period={item.period}>
											{periodTitle(item.period)}
										</span>
										<span class="analysis-list-range">{item.start} ~ {item.end}</span>
										<span class="analysis-list-count">{item.count} 篇日记</span>
									</div>
									<p class="analysis-list-preview">{item.summary.slice(0, 120)}{item.summary.length > 120 ? '…' : ''}</p>
									{#if item.updated}
										<span class="analysis-list-date">更新于 {item.updated.replace('T', ' ').slice(0, 19)}</span>
									{/if}
								</li>
							{/each}
						</ul>
					{/if}
				</div>
			{:else}
				<!-- 选中某条历史：显示详细内容 + 可重新生成 -->
				<div class="analysis-toolbar">
					<button onclick={backToList} class="analysis-toggle">← 返回列表</button>
					<button onclick={() => (showPromptEditor = !showPromptEditor)} class="analysis-toggle">
						{showPromptEditor ? '收起提示词' : '编辑提示词'}
					</button>
					<button onclick={useDefaultPrompt} class="analysis-toggle">恢复默认</button>
					<button
						onclick={() => runAnalysis(selected.period, selected.start, selected.end)}
						disabled={stage === 'loading' || stage === 'checking'}
						class="analysis-reanalyze"
					>
						{stage === 'loading' ? '分析中…' : '重新生成分析'}
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
						<label for="cas-user-prefix" class="analysis-prompt-label analysis-prompt-label--indented">内容引导语（可选）</label>
						<textarea
							id="cas-user-prefix"
							rows={3}
							bind:value={userPrefix}
							placeholder="留空则使用默认的周/月格式化提示语"
							class="analysis-prompt-textarea"
						/>
						<p class="analysis-prompt-hint">修改后点击"重新生成分析"以应用提示词。</p>
					</div>
				{/if}

				<div class="analysis-body">
					{#if stage === 'loading'}
						<div class="analysis-loading">
							<div class="spinner" aria-hidden="true"></div>
							<p>正在分析日记内容…</p>
						</div>
					{:else if stage === 'error'}
						<div class="analysis-error">
							<p>{errorMsg}</p>
							<button class="analysis-retry" onclick={() => runAnalysis(selected.period, selected.start, selected.end)}>重试</button>
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
			{/if}

		<!-- ---- 单条分析模式（原有逻辑） ---- -->
		{:else}
			<div class="analysis-toolbar">
				<button onclick={() => (showPromptEditor = !showPromptEditor)} class="analysis-toggle">
					{showPromptEditor ? '收起提示词' : '编辑提示词'}
				</button>
				<button onclick={useDefaultPrompt} class="analysis-toggle">恢复默认</button>
				<button
					onclick={() => runAnalysis(period, start, end)}
					disabled={stage === 'loading' || stage === 'checking'}
					class="analysis-reanalyze"
				>
					{stage === 'loading' ? '分析中…' : stage === 'checking' ? '加载中…' : result?.id ? '重新生成分析' : '开始分析'}
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
					<label for="cas-user-prefix" class="analysis-prompt-label analysis-prompt-label--indented">内容引导语（可选）</label>
					<textarea
						id="cas-user-prefix"
						rows={3}
						bind:value={userPrefix}
						placeholder="留空则使用默认的周/月格式化提示语"
						class="analysis-prompt-textarea"
					/>
					<p class="analysis-prompt-hint">修改后点击"开始分析"以应用提示词；保存为持久默认请前往设置 → AI 助手。</p>
				</div>
			{/if}

			<div class="analysis-body">
				{#if stage === 'checking'}
					<div class="analysis-loading">
						<div class="spinner" aria-hidden="true"></div>
						<p>正在读取之前保存的分析…</p>
					</div>
				{:else if stage === 'idle'}
					<div class="analysis-idle">
						<p class="analysis-idle-title">准备开始 AI 分析</p>
						<p class="analysis-idle-sub">
							系统将基于此时间段的日记内容生成一份结构化的总结与建议。可先点击上方
							"编辑提示词" 自定义分析风格，然后点击"开始分析"。
							分析结果会自动保存，下次打开时可直接查看。
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
						<button class="analysis-retry" onclick={() => runAnalysis(period, start, end)}>重试</button>
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
		{/if}
	</div>
</div>

<style>
	.analysis-overlay {
		position: fixed;
		inset: 0;
		background: hsl(var(--background) / 0.72);
		backdrop-filter: blur(8px);
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
		box-shadow: 0 20px 60px hsl(0 0% 0% / 0.25);
		animation: panel-in 0.2s ease-out;
		/* 确保弹窗本身在 body 下依然覆盖其他元素 */
		position: relative;
		z-index: 1;
	}

	.analysis-header {
		padding: 1rem 1.25rem;
		border-bottom: 1px solid hsl(var(--border) / 0.5);
		display: flex;
		align-items: center;
		gap: 0.75rem;
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
		display: inline-flex;
		align-items: center;
		gap: 0.5rem;
	}

	.analysis-saved-badge {
		font-size: 0.72rem;
		padding: 0.15rem 0.5rem;
		background: hsl(var(--primary) / 0.12);
		color: hsl(var(--primary));
		border-radius: 9999px;
		border: 1px solid hsl(var(--primary) / 0.2);
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

	.analysis-toggle--active {
		background: hsl(var(--primary) / 0.15);
		color: hsl(var(--primary));
		border-color: hsl(var(--primary) / 0.4);
	}

	.ml-auto {
		margin-left: auto;
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

	.analysis-list {
		list-style: none;
		padding: 0;
		margin: 0;
		display: flex;
		flex-direction: column;
		gap: 0.6rem;
	}

	.analysis-list-item {
		padding: 0.85rem 1rem;
		border: 1px solid hsl(var(--border) / 0.55);
		border-radius: 0.65rem;
		background: hsl(var(--muted) / 0.25);
		cursor: pointer;
		transition: background 0.15s ease, border-color 0.15s ease, transform 0.1s ease;
	}

	.analysis-list-item:hover {
		background: hsl(var(--muted) / 0.5);
		border-color: hsl(var(--primary) / 0.35);
	}

	.analysis-list-item:active {
		transform: scale(0.997);
	}

	.analysis-list-head {
		display: flex;
		align-items: center;
		gap: 0.6rem;
		flex-wrap: wrap;
		margin-bottom: 0.35rem;
	}

	.analysis-list-tag {
		font-size: 0.72rem;
		padding: 0.1rem 0.5rem;
		border-radius: 9999px;
		background: hsl(var(--primary) / 0.12);
		color: hsl(var(--primary));
		border: 1px solid hsl(var(--primary) / 0.25);
	}

	.analysis-list-tag[data-period='month'] {
		background: hsl(var(--accent, 200 80% 60%) / 0.12);
		color: hsl(var(--accent-foreground, 200 80% 30%));
		border-color: hsl(var(--accent, 200 80% 60%) / 0.3);
	}

	.analysis-list-range {
		font-size: 0.82rem;
		color: hsl(var(--foreground) / 0.9);
		font-weight: 500;
	}

	.analysis-list-count {
		margin-left: auto;
		font-size: 0.75rem;
		color: hsl(var(--muted-foreground));
	}

	.analysis-list-preview {
		margin: 0.15rem 0 0.4rem;
		font-size: 0.85rem;
		line-height: 1.55;
		color: hsl(var(--foreground) / 0.75);
		white-space: pre-wrap;
	}

	.analysis-list-date {
		display: block;
		font-size: 0.72rem;
		color: hsl(var(--muted-foreground));
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

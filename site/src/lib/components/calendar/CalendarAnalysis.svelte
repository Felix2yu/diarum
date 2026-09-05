<script lang="ts">
	import {
		analyzePeriod,
		getSavedAnalysis,
		getSavedAnalyses,
		saveAnalysisSummary,
		DEFAULT_ANALYSIS_SYSTEM_PROMPT,
		type PeriodAnalysisResult,
		type SavedPeriodAnalysisResult,
		type PeriodType
	} from '$lib/api/ai';
	import { formatSpecialDateLabel, periodKeyRange, formatDate, type CalendarPeriod } from '$lib/utils/date';
	import { marked } from 'marked';

	// 把 markdown 文本渲染为安全的 HTML
	function renderMarkdown(text: string): string {
		try {
			const safe = (text ?? '').trim();
			if (!safe) return '';
			return marked.parse(safe, { async: false, breaks: true, gfm: true }) as string;
		} catch (e) {
			// 渲染失败则回退到保留换行的纯文本
			return (text ?? '')
				.split('\n')
				.map((line) => `<p>${line}</p>`)
				.join('\n');
		}
	}

	let {
		mode = 'single',
		period,
		key = '',
		start = '',
		end = '',
		onClose
	}: {
		mode?: 'single' | 'history';
		period: PeriodType;
		/** 周期键（周/月/年分析，ISO 8601 周 / 月 / 年）：2026-W36 / 2026-09 / 2026 */
		key?: string;
		/** 自定义分析的初始日期范围 */
		start?: string;
		end?: string;
		onClose: () => void;
	} = $props();

	// 周/月/年分析按周期键寻址（第一周、第一月……），日期范围由键推导；
	// 仅自定义分析使用显式的起止日期。
	function isKeyedPeriod(p: PeriodType): p is CalendarPeriod {
		return p === 'week' || p === 'month' || p === 'year';
	}

	function keyRangeOf(p: PeriodType, k: string) {
		return isKeyedPeriod(p) && k ? periodKeyRange(p, k) : null;
	}

	// ------- 通用状态 -------
	type Stage = 'checking' | 'idle' | 'loading' | 'ready' | 'error' | 'list-loading' | 'list-ready' | 'list-error';
	let stage: Stage = $state('checking');
	let errorMsg: string | null = $state(null);

	// 视图状态：历史分析弹窗内可发起弱化入口的自定义分析
	let view = $state<'single' | 'history'>(mode);
	let fromHistory = $state(false);

	// ------- 单条分析视图 -------
	let result: PeriodAnalysisResult | null = $state(null);
	let savedLabel: string | null = $state(null);
	let showPromptEditor = $state(false);
	let systemPrompt = $state(DEFAULT_ANALYSIS_SYSTEM_PROMPT);
	let userPrefix = $state('');
	let keywords = $state('');

	// 手动填写周报/月报/年报（跳过 AI，直接填写内容保存）
	let showManualEditor = $state(false);
	let manualSummary = $state('');
	let savingManual = $state(false);

	// 当前编辑的分析（周期键或自定义区间）
	let customPeriod = $state<PeriodType>(period);
	let customKey = $state(key);
	let customStart = $state('');
	let customEnd = $state('');
	let showFilters = $state(false);

	// ------- 历史列表视图 -------
	type Filter = 'all' | 'week' | 'month' | 'year' | 'custom';
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
		keywords = '';
		showPromptEditor = false;
		showFilters = false;
		showManualEditor = false;
		manualSummary = '';
	}

	// 打开时：根据 mode 决定加载什么内容
	async function tryLoadSingle(per: PeriodType, k: string, s: string, e: string, kw?: string) {
		resetSingleState();
		customPeriod = per;
		customKey = k;
		const range = keyRangeOf(per, k);
		customStart = range?.start ?? s;
		customEnd = range?.end ?? e;
		if (kw !== undefined) keywords = kw;
		stage = 'checking';
		errorMsg = null;
		try {
			const saved = isKeyedPeriod(per)
				? await getSavedAnalysis(per, { key: k })
				: await getSavedAnalysis(per, { start: customStart, end: customEnd, keywords: kw ?? '' });
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
			const items = await getSavedAnalyses(per === 'all' ? undefined : (per as PeriodType));
			savedList = items;
			stage = 'list-ready';
		} catch (e: unknown) {
			errorMsg = e instanceof Error ? e.message : '获取历史分析失败';
			stage = 'list-error';
		}
	}

	// 根据 mode 初始化加载（组件挂载时执行一次；避免监听 stage 以免循环）
	$effect(() => {
		if (mode === 'history') {
			// 只在初次挂载且尚未加载过时调用，避免 effect 依赖追踪导致循环
			loadList(filter);
		} else {
			tryLoadSingle(period, key, start, end);
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

	function validateDates(): string | null {
		const s = customStart.trim();
		const e = customEnd.trim();
		if (!/^\d{4}-\d{2}-\d{2}$/.test(s)) return '开始日期格式应为 YYYY-MM-DD';
		if (!/^\d{4}-\d{2}-\d{2}$/.test(e)) return '结束日期格式应为 YYYY-MM-DD';
		if (new Date(s).getTime() > new Date(e).getTime()) return '开始日期不能晚于结束日期';
		return null;
	}

	// ------- 单条分析逻辑 -------
	async function runAnalysis() {
		if (!isKeyedPeriod(customPeriod)) {
			const dateErr = validateDates();
			if (dateErr) {
				errorMsg = dateErr;
				stage = 'error';
				return;
			}
		}
		stage = 'loading';
		errorMsg = null;
		result = null;
		savedLabel = null;
		try {
			const r = isKeyedPeriod(customPeriod)
				? await analyzePeriod(customPeriod, {
						key: customKey,
						system_prompt: systemPrompt,
						user_prefix: userPrefix
					})
				: await analyzePeriod(customPeriod, {
						start: customStart,
						end: customEnd,
						keywords,
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

	function openSaved(item: SavedPeriodAnalysisResult) {
		// 在弹窗内切换到单条分析视图，展示已保存内容
		result = item;
		if (item.system_prompt) systemPrompt = item.system_prompt;
		if (item.user_prefix) userPrefix = item.user_prefix;
		customPeriod = item.period;
		customKey = item.key ?? '';
		customStart = item.start;
		customEnd = item.end;
		keywords = item.keywords ?? '';
		showFilters = item.period === 'custom';
		showManualEditor = false;
		manualSummary = item.summary ?? '';
		savedLabel = item.updated
			? `已保存 · ${item.updated.replace('T', ' ').slice(0, 19)}`
			: '已保存';
		selected = item;
		view = 'single';
		stage = 'ready';
	}

	function backToList() {
		result = null;
		savedLabel = null;
		selected = null;
		showManualEditor = false;
		manualSummary = '';
		view = 'history';
		fromHistory = false;
		// 恢复列表状态，不重新请求网络，savedList 已在内存
		stage = 'list-ready';
	}

	// 弱化入口：从历史分析内发起自定义（非整周/整月，如旅行）分析
	function startCustomFromHistory() {
		fromHistory = true;
		selected = null;
		view = 'single';
		resetSingleState();
		customPeriod = 'custom';
		customKey = '';
		const today = new Date();
		const startDay = new Date(today.getTime() - 29 * 24 * 60 * 60 * 1000);
		customStart = formatDate(startDay);
		customEnd = formatDate(today);
		stage = 'idle';
	}

	function useDefaultPrompt() {
		systemPrompt = DEFAULT_ANALYSIS_SYSTEM_PROMPT;
		userPrefix = '';
	}

	function periodTitle(per: string): string {
		if (per === 'week') return '周分析';
		if (per === 'month') return '月分析';
		if (per === 'year') return '年分析';
		return '自定义分析';
	}

	// 周/月/年条目的中文周期标签（如 2026年第36周），自定义分析无
	function itemLabel(per: string, k: string | undefined): string {
		return per === 'week' || per === 'month' || per === 'year' ? formatSpecialDateLabel(k ?? '') : '';
	}

	function openManualEditor() {
		manualSummary = result?.summary ?? '';
		showManualEditor = true;
	}

	async function saveManual() {
		const trimmed = manualSummary.trim();
		if (!trimmed) {
			errorMsg = '报告内容不能为空';
			return;
		}
		savingManual = true;
		errorMsg = null;
		try {
			const r = await saveAnalysisSummary(customPeriod, customKey, trimmed);
			result = r;
			manualSummary = r.summary;
			showManualEditor = false;
			savedLabel = r.updated
				? `已保存 · ${r.updated.replace('T', ' ').slice(0, 19)}`
				: '已保存';
		} catch (e: unknown) {
			errorMsg = e instanceof Error ? e.message : '保存报告失败';
		} finally {
			savingManual = false;
		}
	}

	const mainLabel = $derived(
		view === 'history'
			? selected
				? periodTitle(selected.period)
				: '历史分析'
			: periodTitle(customPeriod)
	);

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
			<div class="analysis-header-main">
				<h3>{mainLabel}</h3>
				{#if view === 'history'}
					{#if selected}
						<p class="analysis-header-sub">
							{#if itemLabel(selected.period, selected.key)}
								<span class="analysis-list-tag">{itemLabel(selected.period, selected.key)}</span>
							{/if}
							{selected.start} ~ {selected.end}
							{#if selected.updated}
								<span class="analysis-saved-badge" title="该分析已保存">
									已保存 · {selected.updated.replace('T', ' ').slice(0, 19)}
								</span>
							{/if}
						</p>
					{:else}
						<p class="analysis-header-sub">共 {savedList.length} 份历史分析</p>
					{/if}
				{:else}
					<p class="analysis-header-sub">
						{#if isKeyedPeriod(customPeriod) && customKey}
							<span class="analysis-list-tag">{formatSpecialDateLabel(customKey)}</span>
						{/if}
						{customStart} ~ {customEnd}
						{#if savedLabel}
							<span class="analysis-saved-badge" title="该分析已保存">{savedLabel}</span>
						{/if}
					</p>
				{/if}
			</div>
			<button class="analysis-close" onclick={onClose} aria-label="关闭">×</button>
		</div>

		<!-- ---- 历史列表模式 ---- -->
		{#if view === 'history'}
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
					<button
						class={filter === 'year' ? 'analysis-toggle analysis-toggle--active' : 'analysis-toggle'}
						onclick={() => loadList('year')}
					>年分析</button>
					<button
						class={filter === 'custom' ? 'analysis-toggle analysis-toggle--active' : 'analysis-toggle'}
						onclick={() => loadList('custom')}
					>自定义</button>
					<!-- 弱化入口：自定义分析仅供旅行等非整周/整月时间段使用 -->
					<button
						onclick={startCustomFromHistory}
						class="analysis-toggle ml-auto"
						title="为旅行等非整周/整月的时间段创建 AI 分析"
					>
						+ 自定义分析
					</button>
					<button onclick={() => loadList(filter)} class="analysis-toggle" title="刷新">
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
								返回日历界面点击"分析"按钮（每行右侧的周报列，或下方的月报/年报卡片），生成或填写分析后将自动保存到此处。
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
										<span class="analysis-list-range">
											{#if itemLabel(item.period, item.key)}
												<span class="analysis-list-key" title={item.key}>
													{itemLabel(item.period, item.key)}
												</span>
												·
											{/if}
											{item.start} ~ {item.end}
										</span>
										<span class="analysis-list-count">{item.count} 篇日记</span>
										{#if item.keywords}
											<span class="analysis-list-keywords" title="关键词">🏷 {item.keywords}</span>
										{/if}
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
					{#if customPeriod === 'custom'}
						<button
							class={showFilters ? 'analysis-toggle analysis-toggle--active' : 'analysis-toggle'}
							onclick={() => (showFilters = !showFilters)}
						>
							{showFilters ? '收起筛选' : '自定义筛选'}
						</button>
					{:else}
						<button
							class={showManualEditor ? 'analysis-toggle analysis-toggle--active' : 'analysis-toggle'}
							onclick={() => (showManualEditor ? (showManualEditor = false) : openManualEditor())}
						>
							{showManualEditor ? '收起填写' : '编辑报告'}
						</button>
					{/if}
					<button onclick={() => (showPromptEditor = !showPromptEditor)} class="analysis-toggle">
						{showPromptEditor ? '收起提示词' : '编辑提示词'}
					</button>
					<button onclick={useDefaultPrompt} class="analysis-toggle">恢复默认</button>
					<button
						onclick={() => runAnalysis()}
						disabled={stage === 'loading' || stage === 'checking'}
						class="analysis-reanalyze"
					>
						{stage === 'loading' ? '分析中…' : '重新生成分析'}
					</button>
				</div>

				{#if customPeriod === 'custom' && showFilters}
					<div class="analysis-filters">
					<div class="analysis-filter-row">
						<label class="analysis-filter-label" for="cas-history-start">开始日期</label>
						<input
							id="cas-history-start"
							type="date"
							bind:value={customStart}
							class="analysis-filter-input"
						/>
					</div>
					<div class="analysis-filter-row">
						<label class="analysis-filter-label" for="cas-history-end">结束日期</label>
						<input
							id="cas-history-end"
							type="date"
							bind:value={customEnd}
							class="analysis-filter-input"
						/>
					</div>
					<div class="analysis-filter-row">
						<label class="analysis-filter-label" for="cas-history-keywords">关键词过滤</label>
						<input
							id="cas-history-keywords"
							type="text"
							bind:value={keywords}
							placeholder="多个关键词用逗号或空格分隔；留空则不做关键词过滤"
							class="analysis-filter-input"
						/>
					</div>
					</div>
				{/if}

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

				{#if isKeyedPeriod(customPeriod) && showManualEditor}
					<div class="analysis-prompt">
						<label for="cas-history-manual-summary" class="analysis-prompt-label">
							{formatSpecialDateLabel(customKey)}（{customStart} ~ {customEnd}）报告内容
						</label>
						<textarea
							id="cas-history-manual-summary"
							rows={8}
							bind:value={manualSummary}
							placeholder="直接填写本周/本月/今年的日记分析，保存后可在历史分析中随时查看。"
							class="analysis-prompt-textarea"
						/>
						<div class="analysis-manual-actions">
							<button class="analysis-retry" onclick={saveManual} disabled={savingManual}>
								{savingManual ? '保存中…' : '保存报告'}
							</button>
						</div>
						{#if errorMsg}
							<p class="analysis-manual-error">{errorMsg}</p>
						{/if}
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
							<button class="analysis-retry" onclick={() => runAnalysis()}>重试</button>
						</div>
					{:else if result}
						<div class="analysis-meta">
							共 {result.count} 篇日记
							{#if result.keywords} · 关键词：{result.keywords}{/if}
						</div>
						<div class="analysis-summary markdown-body">
							{@html renderMarkdown(result.summary)}
						</div>
					{/if}
				</div>
			{/if}

		<!-- ---- 单条分析模式 ---- -->
		{:else}
			<div class="analysis-toolbar">
				{#if fromHistory && customPeriod === 'custom'}
					<button onclick={backToList} class="analysis-toggle">← 历史分析</button>
				{/if}
				{#if isKeyedPeriod(customPeriod)}
					<button
						class={showManualEditor ? 'analysis-toggle analysis-toggle--active' : 'analysis-toggle'}
						onclick={() => (showManualEditor ? (showManualEditor = false) : openManualEditor())}
					>
						{showManualEditor ? '收起填写' : result ? '编辑报告' : '填写报告'}
					</button>
					<!-- 周/月/年分析：支持手动填写报告与提示词编辑，不提供自定义筛选 -->
				{:else}
					<!-- 自定义分析：只保留筛选/提示词按钮 -->
					<button
						class={showFilters ? 'analysis-toggle analysis-toggle--active' : 'analysis-toggle'}
						onclick={() => (showFilters = !showFilters)}
					>
						{showFilters ? '收起筛选' : '自定义筛选'}
					</button>
				{/if}
				<button onclick={() => (showPromptEditor = !showPromptEditor)} class="analysis-toggle">
					{showPromptEditor ? '收起提示词' : '编辑提示词'}
				</button>
				<button onclick={useDefaultPrompt} class="analysis-toggle">恢复默认</button>
				<button
					onclick={() => runAnalysis()}
					disabled={stage === 'loading' || stage === 'checking'}
					class="analysis-reanalyze"
				>
					{stage === 'loading' ? '分析中…' : stage === 'checking' ? '加载中…' : result?.id ? '重新生成分析' : '开始分析'}
				</button>
			</div>

			{#if customPeriod === 'custom' && showFilters}
				<div class="analysis-filters">
					<div class="analysis-filter-row">
						<label class="analysis-filter-label" for="cas-custom-start">开始日期</label>
						<input
							id="cas-custom-start"
							type="date"
							bind:value={customStart}
							class="analysis-filter-input"
						/>
					</div>
					<div class="analysis-filter-row">
						<label class="analysis-filter-label" for="cas-custom-end">结束日期</label>
						<input
							id="cas-custom-end"
							type="date"
							bind:value={customEnd}
							class="analysis-filter-input"
						/>
					</div>
					<div class="analysis-filter-row">
						<label class="analysis-filter-label" for="cas-keywords">关键词过滤</label>
						<input
							id="cas-keywords"
							type="text"
							bind:value={keywords}
							placeholder="多个关键词用逗号或空格分隔，例如：运动, 阅读, 工作"
							class="analysis-filter-input"
						/>
					</div>
					<p class="analysis-prompt-hint">
						自定义筛选：只分析在指定日期范围内且日记内容中包含任一关键词的条目。留空则分析整个区间的日记。
					</p>
				</div>
			{/if}

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

			{#if isKeyedPeriod(customPeriod) && showManualEditor}
				<div class="analysis-prompt">
					<label for="cas-manual-summary" class="analysis-prompt-label">
						{formatSpecialDateLabel(customKey)}（{customStart} ~ {customEnd}）报告内容
					</label>
					<textarea
						id="cas-manual-summary"
						rows={8}
						bind:value={manualSummary}
						placeholder="直接填写本周/本月/今年的日记分析，保存后可在历史分析中随时查看；后续也可以用 AI 重新生成。"
						class="analysis-prompt-textarea"
					/>
					<div class="analysis-manual-actions">
						<button class="analysis-retry" onclick={saveManual} disabled={savingManual}>
							{savingManual ? '保存中…' : '保存报告'}
						</button>
					</div>
					{#if errorMsg}
						<p class="analysis-manual-error">{errorMsg}</p>
					{/if}
				</div>
			{/if}

			<div class="analysis-body">
				{#if stage === 'checking'}
					<div class="analysis-loading">
						<div class="spinner" aria-hidden="true"></div>
						<p>正在读取之前保存的分析…</p>
					</div>
				{:else if stage === 'idle'}
					{#if !showFilters || !showPromptEditor}
						{#if showFilters}
							<div class="analysis-idle">
								<p class="analysis-idle-title">准备开始 AI 分析</p>
								<p class="analysis-idle-sub">已启用自定义筛选。请确认上方的日期范围和关键词（可留空），然后点击"开始分析"。系统将只分析符合筛选条件的日记，分析结果会自动保存，下次打开时可直接查看。</p>
							</div>
						{:else}
							<div class="analysis-idle">
								<p class="analysis-idle-title">准备开始 AI 分析</p>
								<p class="analysis-idle-sub">
									{#if isKeyedPeriod(customPeriod)}
										系统将基于该周/该月/该年的日记内容生成一份结构化的总结与建议。可在"编辑提示词"中自定义分析风格，也可直接"填写报告"手动记录，然后点击"开始分析"或"保存报告"。分析结果会自动保存，下次打开时可直接查看。
									{:else}
										系统将基于所选时间段的日记内容生成一份结构化的总结与建议。可点击"自定义筛选"按日期范围或关键词进行更精细的分析，也可在"编辑提示词"中自定义分析风格，然后点击"开始分析"。分析结果会自动保存，下次打开时可直接查看。
									{/if}
								</p>
							</div>
						{/if}
					{/if}
				{:else if stage === 'loading'}
					<div class="analysis-loading">
						<div class="spinner" aria-hidden="true"></div>
						<p>正在分析日记内容…</p>
					</div>
				{:else if stage === 'error'}
					<div class="analysis-error">
						<p>{errorMsg}</p>
						<button class="analysis-retry" onclick={() => runAnalysis()}>重试</button>
					</div>
				{:else if stage === 'ready' && result}
					<div class="analysis-meta">
						共 {result.count} 篇日记
						{#if result.keywords} · 关键词：{result.keywords}{/if}
					</div>
					<div class="analysis-summary markdown-body">
						{@html renderMarkdown(result.summary)}
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
		background: hsl(0 0% 0% / 0.5);
		backdrop-filter: blur(8px);
		-webkit-backdrop-filter: blur(8px);
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
		max-width: min(56rem, 92vw);
		max-height: 80vh;
		overflow-y: auto;
		display: flex;
		flex-direction: column;
		box-shadow: 0 20px 60px hsl(0 0% 0% / 0.25);
		animation: panel-in 0.2s ease-out;
		/* 确保弹窗本身在 body 下依然覆盖其他元素 */
		position: relative;
		z-index: 1;
	}

	.analysis-header {
		padding: 1rem 3rem 1rem 1.25rem;
		border-bottom: 1px solid hsl(var(--border) / 0.5);
		text-align: center;
		position: relative;
	}

	.analysis-header-main {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 0.25rem;
	}

	.analysis-header h3 {
		margin: 0;
		font-size: 1.05rem;
		font-weight: 600;
		color: hsl(var(--foreground));
	}

	.analysis-header-sub {
		margin: 0;
		color: hsl(var(--muted-foreground));
		font-size: 0.8rem;
		display: inline-flex;
		align-items: center;
		gap: 0.5rem;
		flex-wrap: wrap;
		justify-content: center;
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
		position: absolute;
		top: 0.6rem;
		right: 0.6rem;
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

	/* ---- 自定义筛选（日期 / 关键词） ---- */
	.analysis-filters {
		padding: 0.85rem 1.25rem 1rem;
		border-bottom: 1px solid hsl(var(--border) / 0.4);
		background: hsl(var(--muted) / 0.15);
	}

	.analysis-filter-row {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		margin-bottom: 0.5rem;
	}

	.analysis-filter-row:last-child {
		margin-bottom: 0;
	}

	.analysis-filter-label {
		flex: 0 0 5.5rem;
		font-size: 0.8rem;
		color: hsl(var(--muted-foreground));
	}

	.analysis-filter-input {
		flex: 1;
		width: 100%;
		padding: 0.5rem 0.75rem;
		font-size: 0.85rem;
		line-height: 1.5;
		background: hsl(var(--background));
		border: 1px solid hsl(var(--border) / 0.6);
		color: hsl(var(--foreground));
		border-radius: 0.5rem;
		outline: none;
		transition: border-color 0.15s ease, box-shadow 0.15s ease;
	}

	.analysis-filter-input:focus {
		border-color: hsl(var(--primary) / 0.7);
		box-shadow: 0 0 0 1px hsl(var(--primary) / 0.3);
	}

	/* ---- 提示词样式 ---- */
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
		padding: 0.5rem 0.75rem;
		font-size: 0.85rem;
		line-height: 1.6;
		background: hsl(var(--background));
		border: 1px solid hsl(var(--border) / 0.6);
		color: hsl(var(--foreground));
		border-radius: 0.5rem;
		resize: vertical;
		font-family: inherit;
		outline: none;
		transition: border-color 0.15s ease, box-shadow 0.15s ease;
	}

	.analysis-prompt-textarea:focus {
		border-color: hsl(var(--primary) / 0.7);
		box-shadow: 0 0 0 1px hsl(var(--primary) / 0.3);
	}

	.analysis-prompt-hint {
		font-size: 0.72rem;
		color: hsl(var(--muted-foreground));
		margin: 0.6rem 0 0;
	}

	.analysis-manual-actions {
		display: flex;
		justify-content: flex-end;
		margin-top: 0.6rem;
	}

	.analysis-manual-error {
		font-size: 0.75rem;
		color: hsl(var(--destructive) / 0.9);
		margin: 0.5rem 0 0;
	}

	.analysis-list-key {
		color: hsl(var(--primary));
		font-weight: 500;
	}

	.analysis-body {
		padding: 1.25rem;
		overflow-y: auto;
		flex: 1;
		display: flex;
		flex-direction: column;
		align-items: center;
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

	.analysis-idle--compact {
		padding: 1rem 0.75rem;
	}

	.analysis-idle--compact .analysis-idle-title {
		font-size: 0.9rem;
		margin-bottom: 0.25rem;
	}

	.analysis-idle--compact .analysis-idle-sub {
		font-size: 0.8rem;
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

	/* ---- Markdown 渲染样式 ---- */
	.markdown-body {
		line-height: 1.75;
		color: hsl(var(--foreground) / 0.92);
		font-size: 0.95rem;
		word-break: break-word;
	}

	.markdown-body > *:first-child {
		margin-top: 0;
	}

	.markdown-body > *:last-child {
		margin-bottom: 0;
	}

	.markdown-body p {
		margin: 0 0 0.75rem;
	}

	.markdown-body strong {
		font-weight: 600;
		color: hsl(var(--foreground));
	}

	.markdown-body em {
		font-style: italic;
	}

	.markdown-body h1,
	.markdown-body h2,
	.markdown-body h3,
	.markdown-body h4 {
		font-weight: 600;
		line-height: 1.3;
		color: hsl(var(--foreground));
		margin: 1.25rem 0 0.6rem;
	}

	.markdown-body h1 {
		font-size: 1.35rem;
		border-bottom: 1px solid hsl(var(--border) / 0.6);
		padding-bottom: 0.4rem;
	}

	.markdown-body h2 {
		font-size: 1.15rem;
	}

	.markdown-body h3 {
		font-size: 1.02rem;
	}

	.markdown-body h4 {
		font-size: 0.95rem;
	}

	.markdown-body ul,
	.markdown-body ol {
		margin: 0.25rem 0 0.75rem;
		padding-left: 1.5rem;
	}

	.markdown-body li {
		margin: 0.2rem 0;
	}

	.markdown-body ul li {
		list-style: disc;
	}

	.markdown-body ol li {
		list-style: decimal;
	}

	.markdown-body blockquote {
		margin: 0.5rem 0 0.75rem;
		padding: 0.4rem 0.9rem;
		border-left: 3px solid hsl(var(--primary) / 0.5);
		background: hsl(var(--muted) / 0.3);
		color: hsl(var(--muted-foreground));
		border-radius: 0 0.4rem 0.4rem 0;
	}

	.markdown-body blockquote p {
		margin: 0;
	}

	.markdown-body code {
		font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
		font-size: 0.85em;
		padding: 0.1rem 0.35rem;
		background: hsl(var(--muted) / 0.5);
		border-radius: 0.3rem;
		color: hsl(var(--foreground) / 0.95);
	}

	.markdown-body pre {
		background: hsl(var(--muted) / 0.35);
		border: 1px solid hsl(var(--border) / 0.4);
		border-radius: 0.5rem;
		padding: 0.75rem 0.9rem;
		overflow-x: auto;
		margin: 0.5rem 0 0.75rem;
	}

	.markdown-body pre code {
		background: transparent;
		border: none;
		padding: 0;
		font-size: 0.82rem;
		color: hsl(var(--foreground) / 0.95);
	}

	.markdown-body hr {
		border: none;
		border-top: 1px solid hsl(var(--border) / 0.55);
		margin: 0.9rem 0;
	}

	.markdown-body a {
		color: hsl(var(--primary));
		text-decoration: none;
	}

	.markdown-body a:hover {
		text-decoration: underline;
	}

	.markdown-body table {
		width: 100%;
		border-collapse: collapse;
		margin: 0.5rem 0 0.75rem;
		font-size: 0.88rem;
	}

	.markdown-body th,
	.markdown-body td {
		border: 1px solid hsl(var(--border) / 0.5);
		padding: 0.4rem 0.6rem;
		text-align: left;
	}

	.markdown-body th {
		background: hsl(var(--muted) / 0.5);
		font-weight: 600;
	}

	.markdown-body tr:nth-child(even) td {
		background: hsl(var(--muted) / 0.15);
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

	.analysis-list-keywords {
		margin-left: 0.6rem;
		padding: 0.15rem 0.45rem;
		font-size: 0.72rem;
		background: hsl(var(--primary) / 0.08);
		color: hsl(var(--primary));
		border-radius: 0.35rem;
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

<script lang="ts">
	import { goto } from '$app/navigation';
	import { formatDate, getCalendarDays, getToday, getYearRange, formatISOWeekKey, getISOWeek, formatMonthYear, parseDate } from '$lib/utils/date';
	import { getDatesWithDiaries, getDiariesOnThisDay, getRandomDiary, type CalendarDiaryMeta, type Diary } from '$lib/api/diaries';
	import { getSavedAnalyses } from '$lib/api/ai';
	import CalendarAnalysis from './CalendarAnalysis.svelte';
	import CalendarYearPicker from './CalendarYearPicker.svelte';
	import { moodToLabel } from '$lib/utils/diaryEmoji';
	import MoodIcon from '$lib/components/ui/MoodIcon.svelte';
	import { isWMOCode, getWeatherInfo } from '$lib/utils/weatherCodes';

	let {
		currentYear = $bindable(new Date().getFullYear()),
		currentMonth = $bindable(new Date().getMonth() + 1),
		diaryMeta = $bindable([] as CalendarDiaryMeta[]),
		yearDiaryMeta = $bindable([] as CalendarDiaryMeta[]),
		yearViewActive = $bindable(false),
		onmonthchange = (() => {}) as () => void
	} = $props();

	type ViewMode = 'month' | 'year';
	let viewMode = $state<ViewMode>('month');
	// 让 yearViewActive 与 viewMode 保持同步（父组件通过 bind 监控）
	$effect(() => {
		yearViewActive = viewMode === 'year';
	});
	let yearLoading = $state(false);
	let loadedYear = $state<number | null>(null);
	let transitionDirection = $state<'forward' | 'backward'>('forward');
	let yearPickerOpen = $state(false);

	type AnalysisState = {
		active: boolean;
		mode?: 'single' | 'history';
		period: 'week' | 'month' | 'year' | 'custom';
		key: string;
		start: string;
		end: string;
	} | null;
	let analysis = $state<AnalysisState>(null);

	// 已保存分析的周期键集合：用于在日历上标记某周/某月/某年是否已有分析
	let savedWeekKeys = $state<Set<string>>(new Set());
	let savedMonthKeys = $state<Set<string>>(new Set());
	let savedYearKeys = $state<Set<string>>(new Set());

	async function loadSavedAnalyses() {
		try {
			const [weeks, months, years] = await Promise.all([
				getSavedAnalyses('week'),
				getSavedAnalyses('month'),
				getSavedAnalyses('year')
			]);
			savedWeekKeys = new Set(weeks.map((i) => i.key ?? ''));
			savedMonthKeys = new Set(months.map((i) => i.key ?? ''));
			savedYearKeys = new Set(years.map((i) => i.key ?? ''));
		} catch {
			// 获取失败时按“无已保存分析”处理，不影响日历展示
		}
	}

	function hasWeekKey(key: string): boolean {
		return savedWeekKeys.has(key);
	}
	function hasMonthKey(key: string): boolean {
		return savedMonthKeys.has(key);
	}
	function hasYearKey(key: string): boolean {
		return savedYearKeys.has(key);
	}

	function weekKeyOf(isoYear: number, week: number): string {
		return `${isoYear}-W${String(week).padStart(2, '0')}`;
	}

	const monthKey = $derived(`${currentYear}-${String(currentMonth).padStart(2, '0')}`);

	// 挂载时加载已保存分析，用于按钮“查看/分析”状态标记
	$effect(() => {
		loadSavedAnalyses();
	});

	// 往昔今朝 / 时空穿越
	type OnThisDayState = {
		active: boolean;
		date: string;
		total: number;
		diaries: Diary[];
		loading: boolean;
	};
	let onThisDay = $state<OnThisDayState>({
		active: false,
		date: '',
		total: 0,
		diaries: [],
		loading: false
	});

	type RandomState = {
		active: boolean;
		exists: boolean;
		diary: Diary | null;
		loading: boolean;
	};
	let randomState = $state<RandomState>({
		active: false,
		exists: false,
		diary: null,
		loading: false
	});

	async function openOnThisDay() {
		const today = getToday();
		const queryDate = onThisDay.date || today;
		onThisDay.active = true;
		onThisDay.date = queryDate;
		onThisDay.loading = true;
		onThisDay.diaries = [];
		onThisDay.total = 0;
		const result = await getDiariesOnThisDay(queryDate);
		onThisDay.date = result.date;
		onThisDay.total = result.total;
		onThisDay.diaries = result.diaries;
		onThisDay.loading = false;
	}

	function closeOnThisDay() {
		onThisDay.active = false;
	}

	async function openRandom() {
		randomState.active = true;
		randomState.loading = true;
		randomState.exists = false;
		randomState.diary = null;
		const result = await getRandomDiary(getToday());
		randomState.exists = result.exists;
		randomState.diary = result.diary;
		randomState.loading = false;
	}

	async function rerollRandom() {
		const current = randomState.diary;
		randomState.loading = true;
		const result = await getRandomDiary(current?.date ?? getToday());
		randomState.exists = result.exists;
		randomState.diary = result.diary;
		randomState.loading = false;
	}

	function closeRandom() {
		randomState.active = false;
	}

	// Portal refs for 往昔今朝 / 时空穿越 modals
	let onThisDayOverlayEl: HTMLDivElement | null = $state(null);
	let onThisDayHostEl: HTMLDivElement | null = null;
	let randomOverlayEl: HTMLDivElement | null = $state(null);
	let randomHostEl: HTMLDivElement | null = null;

	// Portal: mount 往昔今朝 to document.body
	$effect(() => {
		if (!onThisDayOverlayEl) return;
		if (!onThisDayHostEl) {
			onThisDayHostEl = document.createElement('div');
			onThisDayHostEl.style.position = 'static';
		}
		document.body.appendChild(onThisDayHostEl);
		onThisDayHostEl.appendChild(onThisDayOverlayEl);
		return () => {
			if (onThisDayHostEl && onThisDayHostEl.parentNode) {
				onThisDayHostEl.parentNode.removeChild(onThisDayHostEl);
			}
			if (onThisDayOverlayEl && onThisDayOverlayEl.parentNode) {
				onThisDayOverlayEl.parentNode.removeChild(onThisDayOverlayEl);
			}
			onThisDayHostEl = null;
		};
	});

	// Portal: mount 时空穿越 to document.body
	$effect(() => {
		if (!randomOverlayEl) return;
		if (!randomHostEl) {
			randomHostEl = document.createElement('div');
			randomHostEl.style.position = 'static';
		}
		document.body.appendChild(randomHostEl);
		randomHostEl.appendChild(randomOverlayEl);
		return () => {
			if (randomHostEl && randomHostEl.parentNode) {
				randomHostEl.parentNode.removeChild(randomHostEl);
			}
			if (randomOverlayEl && randomOverlayEl.parentNode) {
				randomOverlayEl.parentNode.removeChild(randomOverlayEl);
			}
			randomHostEl = null;
		};
	});

	function formatDisplayDate(dateStr: string): string {
		const d = parseDate(dateStr);
		return `${d.getFullYear()}年${d.getMonth() + 1}月${d.getDate()}日`;
	}

	function diaryContentPreview(content: string, maxLength = 140): string {
		const text = content.replace(/<[^>]*>/g, '').trim();
		if (text.length <= maxLength) return text;
		return text.slice(0, maxLength) + '…';
	}

	// 周/月/年分析以周期键寻址（ISO 8601 周：2026-W36；月：2026-09；年：2026），
	// 日期范围由后端/组件从键推导。自定义分析的入口收敛在"历史分析"弹窗内。
	// 打开指定 ISO 8601 周的周报（供周报按钮列与年份选择器的"周报"页签调用）
	function openWeekAnalysisFor(year: number, week: number) {
		analysis = { active: true, mode: 'single', period: 'week', key: `${year}-W${String(week).padStart(2, '0')}`, start: '', end: '' };
	}
	function openMonthAnalysis() {
		analysis = { active: true, mode: 'single', period: 'month', key: `${currentYear}-${String(currentMonth).padStart(2, '0')}`, start: '', end: '' };
	}
	// 打开指定月份的月报（供年份选择器的"月报"页签调用）
	function openMonthAnalysisFor(year: number, month: number) {
		analysis = { active: true, mode: 'single', period: 'month', key: `${year}-${String(month).padStart(2, '0')}`, start: '', end: '' };
	}
	function openYearAnalysis() {
		openYearAnalysisFor(currentYear);
	}
	// 打开指定年份的年报（供年份选择器的"年报"页签调用）
	function openYearAnalysisFor(year: number) {
		analysis = { active: true, mode: 'single', period: 'year', key: String(year), start: '', end: '' };
	}
	function openHistoryAnalysis() {
		analysis = { active: true, mode: 'history', period: 'week', key: '', start: '', end: '' };
	}
	function closeAnalysis() {
		analysis = null;
		// 关闭弹窗后刷新已保存分析集合，更新“查看/分析该周”等按钮状态
		loadSavedAnalyses();
	}

	const weekDays = ['周一', '周二', '周三', '周四', '周五', '周六', '周日'];
	const weekDaysShort = ['一', '二', '三', '四', '五', '六', '日'];
	const monthNamesShort = [
		'一', '二', '三', '四', '五', '六',
		'七', '八', '九', '十', '十一', '十二'
	];

	const calendarDays = $derived(getCalendarDays(currentYear, currentMonth));
	// 日历按周一起始补齐为整周，每 7 天一行恰好是一个 ISO 8601 周
	const calendarWeeks = $derived(
		Array.from({ length: calendarDays.length / 7 }, (_, w) => calendarDays.slice(w * 7, w * 7 + 7))
	);
	const todayStr = $derived(getToday());
	const metaByDate = $derived(new Map(diaryMeta.map((item) => [item.date, item])));
	const yearMetaByDate = $derived(new Map(yearDiaryMeta.map((item) => [item.date, item])));

	function isCurrentMonth(date: Date): boolean {
		return date.getMonth() === currentMonth - 1;
	}

	function isToday(date: Date): boolean {
		return formatDate(date) === todayStr;
	}

	function hasDiary(date: Date): boolean {
		return metaByDate.get(formatDate(date))?.has_content ?? false;
	}

	function getDateMeta(date: Date): CalendarDiaryMeta | undefined {
		return metaByDate.get(formatDate(date));
	}

	function handleDateClick(date: Date) {
		goto(`/diary/${formatDate(date)}`);
	}

	function goToPreviousMonth() {
		transitionDirection = 'backward';
		if (currentMonth === 1) {
			currentMonth = 12;
			currentYear--;
		} else {
			currentMonth--;
		}
	}

	function goToNextMonth() {
		transitionDirection = 'forward';
		if (currentMonth === 12) {
			currentMonth = 1;
			currentYear++;
		} else {
			currentMonth++;
		}
	}

	function goToToday() {
		const today = new Date();
		currentYear = today.getFullYear();
		currentMonth = today.getMonth() + 1;
	}

	function openYearPicker() {
		yearPickerOpen = true;
	}

	function closeYearPicker() {
		yearPickerOpen = false;
	}

	async function enterYearView() {
		viewMode = 'year';
		await loadYearData(currentYear);
	}

	function exitYearView(month: number) {
		currentMonth = month;
		viewMode = 'month';
		onmonthchange();
	}

	async function loadYearData(year: number) {
		if (loadedYear === year) return;
		yearLoading = true;
		const range = getYearRange(year);
		yearDiaryMeta = await getDatesWithDiaries(range.start, range.end);
		loadedYear = year;
		yearLoading = false;
	}

	async function goToPreviousYear() {
		transitionDirection = 'backward';
		currentYear--;
		await loadYearData(currentYear);
	}

	async function goToNextYear() {
		transitionDirection = 'forward';
		currentYear++;
		await loadYearData(currentYear);
	}

	function goToCurrentYear() {
		const today = new Date();
		currentYear = today.getFullYear();
		loadYearData(currentYear);
	}

	function getMiniCalendarDays(year: number, month: number): (number | null)[] {
		const firstDay = new Date(year, month, 1);
		const lastDay = new Date(year, month + 1, 0);
		const startDay = (firstDay.getDay() + 6) % 7;
		const daysInMonth = lastDay.getDate();

		const days: (number | null)[] = [];
		for (let i = 0; i < startDay; i++) {
			days.push(null);
		}
		for (let i = 1; i <= daysInMonth; i++) {
			days.push(i);
		}
		return days;
	}

	function yearHasDiary(month: number, day: number): boolean {
		const dateStr = `${currentYear}-${String(month + 1).padStart(2, '0')}-${String(day).padStart(2, '0')}`;
		return yearMetaByDate.get(dateStr)?.has_content ?? false;
	}

	function yearGetMeta(month: number, day: number): CalendarDiaryMeta | undefined {
		const dateStr = `${currentYear}-${String(month + 1).padStart(2, '0')}-${String(day).padStart(2, '0')}`;
		return yearMetaByDate.get(dateStr);
	}

	function isTodayMini(month: number, day: number): boolean {
		const dateStr = `${currentYear}-${String(month + 1).padStart(2, '0')}-${String(day).padStart(2, '0')}`;
		return dateStr === todayStr;
	}

	function isCurrentMonthMini(month: number): boolean {
		const today = new Date();
		return currentYear === today.getFullYear() && month === today.getMonth();
	}

	function handleMiniDayClick(e: Event, month: number, _day: number) {
		e.stopPropagation();
		exitYearView(month + 1);
	}
</script>

<div class="calendar">
	{#if viewMode === 'month'}
		<!-- Month View -->
		<div class="view-container animate-fade-in-only">
			<!-- Calendar Header -->
			<div class="flex flex-col gap-2 sm:gap-3 mb-4 sm:mb-5 px-2">
				<!-- 第一行：月份导航 -->
				<div class="flex items-center justify-between">
					<button
						onclick={goToPreviousMonth}
						class="p-1.5 sm:p-2 rounded-lg hover:bg-muted/50 transition-all duration-200 shrink-0"
						title="上一月"
					>
						<svg class="w-5 h-5 text-foreground" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
						</svg>
					</button>

					<div class="flex items-center gap-1 sm:gap-2 min-w-0 flex-1 justify-center">
						<h2 class="text-base sm:text-lg font-semibold text-foreground flex items-center gap-1 sm:gap-1.5 min-w-0">
							<button
								onclick={openYearPicker}
								class="hover:bg-muted/50 transition-all duration-200 rounded-md px-1.5 sm:px-2 py-1 flex items-center gap-0.5 sm:gap-1 whitespace-nowrap shrink-0"
								title="点击选择年月"
							>
								<span>{formatMonthYear(currentYear, currentMonth)}</span>
								<svg class="w-3.5 h-3.5 text-muted-foreground shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
								</svg>
							</button>
							<button
								onclick={enterYearView}
								class="year-button"
								title="切换到年视图"
							>
								全年
							</button>
						</h2>
						<div class="flex items-center gap-1 sm:gap-1.5 shrink-0">
							<button
								onclick={goToToday}
								class="px-2 sm:px-3 py-1 text-xs sm:text-sm bg-primary text-primary-foreground rounded-md hover:opacity-90 transition-all duration-200 whitespace-nowrap"
							>
								今天
							</button>
						</div>
					</div>

					<button
						onclick={goToNextMonth}
						class="p-1.5 sm:p-2 rounded-lg hover:bg-muted/50 transition-all duration-200 shrink-0"
						title="下一月"
					>
						<svg class="w-5 h-5 text-foreground" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
						</svg>
					</button>
				</div>

				<!-- 第二行：历史分析 / 往昔今朝 / 时空穿越。
				     周/月/年分析入口已移入日历本体：周报在每行右侧列，月报/年报在日历下方的独立行。 -->
				<div class="flex items-center justify-center gap-1.5 mt-0.5 overflow-x-auto scrollbar-none pb-0.5">
					<!-- 自定义分析的弱化入口收敛在"历史分析"弹窗内，用于旅行等非整周/整月时间段 -->
					<button
						onclick={openHistoryAnalysis}
						class="px-3 py-1 text-xs font-medium rounded-md border border-primary/40 bg-primary/10 text-primary hover:bg-primary/15 hover:border-primary/60 transition-all duration-200 whitespace-nowrap shrink-0"
						title="查看历史分析"
					>
						<span class="inline-flex items-center gap-1">
							<svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
							</svg>
							历史分析
						</span>
					</button>
					<button
						onclick={openOnThisDay}
						class="px-2.5 py-1 text-xs rounded-md border border-border bg-muted/30 text-muted-foreground hover:text-foreground hover:bg-muted/60 transition-all duration-200"
						title="查看往年同一日的日记"
					>
						<span class="inline-flex items-center gap-1">
							<svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
							</svg>
							往昔今朝
						</span>
					</button>
					<button
						onclick={openRandom}
						class="px-2.5 py-1 text-xs rounded-md border border-border bg-muted/30 text-muted-foreground hover:text-foreground hover:bg-muted/60 transition-all duration-200"
						title="随机翻阅一条过去的日记"
					>
						<span class="inline-flex items-center gap-1">
							<svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9H4m16 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H20" />
							</svg>
							时空穿越
						</span>
					</button>
				</div>
			</div>

			<!-- Week Days -->
			<div class="weekdays-grid">
				{#each weekDays as day}
					<div class="text-center font-medium text-muted-foreground text-sm py-2">{day}</div>
				{/each}
				<div class="week-header-action">周报</div>
			</div>

			<!-- Calendar Days：按 ISO 周分行，整行可点打开该周周报 -->
			<div class="days-grid">
				{#each calendarWeeks as weekDays, wi}
					{@const weekStart = weekDays[0]}
					{@const iso = getISOWeek(weekStart)}
					{@const weekKey = weekKeyOf(iso.year, iso.week)}
					<!-- svelte-ignore a11y-click-events-have-key-events, a11y_no_static_element_interactions -->
					<div
						class="week-row"
						title="{iso.year}年第{iso.week}周（{formatDate(weekStart)} ~ {formatDate(weekDays[6])}）周分析"
						onclick={() => openWeekAnalysisFor(iso.year, iso.week)}
					>
						{#each weekDays as date, di}
							{@const meta = getDateMeta(date)}
							{@const dayIndex = wi * 7 + di}
							<button
								onclick={(e) => { e.stopPropagation(); handleDateClick(date); }}
								class="day aspect-square rounded-lg transition-all duration-200 flex flex-col items-center justify-center relative
									   {isCurrentMonth(date) ? 'text-foreground' : 'text-muted-foreground/40'}
									   {isToday(date) ? 'bg-primary/10 ring-2 ring-primary font-semibold' : ''}
									   {hasDiary(date) && !isToday(date) ? 'bg-amber-500/10 dark:bg-amber-500/20' : ''}
									   {!isToday(date) && !hasDiary(date) ? 'hover:bg-muted/50' : ''}
									   {hasDiary(date) && !isToday(date) ? 'hover:bg-amber-500/20 dark:hover:bg-amber-500/30' : ''}"
								style="animation-delay: {dayIndex * 10}ms"
							>
								<span class="text-sm">{date.getDate()}</span>

								{#if (meta?.weather && isWMOCode(meta.weather)) || meta?.mood}
									<div class="absolute inset-x-0 top-1.5 flex items-center justify-center gap-1 text-[11px] leading-none">
										{#if meta?.weather && isWMOCode(meta.weather)}
											{@const weatherInfo = getWeatherInfo(parseInt(meta.weather))}
											<span class="emoji-chip" title="天气：{weatherInfo.label}{meta?.temp_min != null && meta?.temp_max != null ? ` ${Math.round(meta.temp_min)}°~${Math.round(meta.temp_max)}°` : ''}">{weatherInfo.icon}</span>
										{/if}
										{#if meta?.mood}
											<span class="emoji-chip" title="心情：{moodToLabel(meta.mood)}">
												<MoodIcon mood={meta.mood} size={14} />
											</span>
										{/if}
									</div>
								{:else if hasDiary(date)}
									<span class="absolute bottom-1 w-1 h-1 bg-amber-500 rounded-full"></span>
								{/if}
							</button>
						{/each}
						<button
							class="week-action"
							class:week-action--view={hasWeekKey(weekKey)}
							onclick={(e) => { e.stopPropagation(); openWeekAnalysisFor(iso.year, iso.week); }}
							title={hasWeekKey(weekKey)
								? `查看 ${iso.year}年第${iso.week}周已保存的分析`
								: `分析 ${iso.year}年第${iso.week}周（${formatDate(weekStart)} ~ ${formatDate(weekDays[6])}）`}
						>
							{hasWeekKey(weekKey) ? '查看' : '分析'}
						</button>
					</div>
				{/each}
			</div>

			<!-- 月报 / 年报：单行两张紧凑卡片，右侧状态胶囊直观显示是否已有已保存的分析 -->
			<div class="period-grid">
				<button
					class="period-card"
					onclick={openMonthAnalysis}
					title={hasMonthKey(monthKey) ? `查看 ${formatMonthYear(currentYear, currentMonth)}已保存的分析` : `生成 ${formatMonthYear(currentYear, currentMonth)}的 AI 分析`}
				>
					<span class="period-card-info">
						<span class="period-card-name">月报</span>
						<span class="period-card-desc">{formatMonthYear(currentYear, currentMonth)}</span>
					</span>
					<span class="period-card-state" class:period-card-state--saved={hasMonthKey(monthKey)}>
						{hasMonthKey(monthKey) ? '查看' : '分析'}
					</span>
				</button>
				<button
					class="period-card"
					onclick={openYearAnalysis}
					title={hasYearKey(String(currentYear)) ? `查看 ${currentYear} 年已保存的分析` : `生成 ${currentYear} 年的 AI 分析`}
				>
					<span class="period-card-info">
						<span class="period-card-name">年报</span>
						<span class="period-card-desc">{currentYear}年</span>
					</span>
					<span class="period-card-state" class:period-card-state--saved={hasYearKey(String(currentYear))}>
						{hasYearKey(String(currentYear)) ? '查看' : '分析'}
					</span>
				</button>
			</div>
		</div>
	{:else}
		<!-- Year View -->
		<div class="view-container year-mode animate-fade-in-only">
			<!-- Year Header -->
			<div class="flex items-center justify-between mb-4 sm:mb-5 px-2">
				<button
					onclick={goToPreviousYear}
					class="p-1.5 sm:p-2 rounded-lg hover:bg-muted/50 transition-all duration-200 shrink-0"
					title="上一年"
				>
					<svg class="w-5 h-5 text-foreground" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
					</svg>
				</button>

				<div class="flex items-center gap-1.5 sm:gap-3 min-w-0 flex-1 justify-center">
					<h2 class="text-base sm:text-lg font-semibold text-foreground whitespace-nowrap">{currentYear}</h2>
					<div class="flex items-center gap-1 sm:gap-1.5 shrink-0">
						<button
							onclick={goToCurrentYear}
							class="px-2 sm:px-3 py-1 text-xs sm:text-sm bg-primary text-primary-foreground rounded-md hover:opacity-90 transition-all duration-200 whitespace-nowrap"
						>
							本年
						</button>
					</div>
				</div>

				<button
					onclick={goToNextYear}
					class="p-1.5 sm:p-2 rounded-lg hover:bg-muted/50 transition-all duration-200 shrink-0"
					title="下一年"
				>
					<svg class="w-5 h-5 text-foreground" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
					</svg>
				</button>
			</div>

			<!-- 年视图专属行：年分析 / 往昔今朝 / 时空穿越 -->
			<div class="flex items-center justify-center gap-1.5 mb-5 px-2">
				<button
					onclick={openYearAnalysis}
					class="px-2.5 py-1 text-xs rounded-md border transition-all duration-200 {hasYearKey(String(currentYear))
						? 'border-primary/40 bg-primary/10 text-primary hover:bg-primary/15 hover:border-primary/60'
						: 'border-border bg-muted/30 text-muted-foreground hover:text-foreground hover:bg-muted/60'}"
					title={hasYearKey(String(currentYear)) ? `查看 ${currentYear} 年已保存的分析` : `${currentYear} 年 AI 分析`}
				>
					<span class="inline-flex items-center gap-1">
						<svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
						</svg>
						{hasYearKey(String(currentYear)) ? '查看' : '分析'}
					</span>
				</button>
				<button
					onclick={openOnThisDay}
					class="px-2.5 py-1 text-xs rounded-md border border-border bg-muted/30 text-muted-foreground hover:text-foreground hover:bg-muted/60 transition-all duration-200"
					title="查看往年同一日的日记"
				>
					<span class="inline-flex items-center gap-1">
						<svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
						</svg>
						往昔今朝
					</span>
				</button>
				<button
					onclick={openRandom}
					class="px-2.5 py-1 text-xs rounded-md border border-border bg-muted/30 text-muted-foreground hover:text-foreground hover:bg-muted/60 transition-all duration-200"
					title="随机翻阅一条过去的日记"
				>
					<span class="inline-flex items-center gap-1">
						<svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9H4m16 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H20" />
						</svg>
						时空穿越
					</span>
				</button>
			</div>

			<!-- Year Grid -->
			<div class="year-scroll-container">
				{#if yearLoading}
					<div class="flex items-center justify-center py-12">
						<svg class="w-6 h-6 animate-spin text-primary" fill="none" viewBox="0 0 24 24">
							<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
							<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
						</svg>
					</div>
				{:else}
					<div class="year-grid">
						{#each Array(12) as _, monthIdx}
							<button
								class="mini-month"
								class:mini-month-current={isCurrentMonthMini(monthIdx)}
								onclick={() => exitYearView(monthIdx + 1)}
								style="animation-delay: {monthIdx * 30}ms"
							>
								<div class="mini-month-name" class:text-primary={isCurrentMonthMini(monthIdx)}>
									{monthNamesShort[monthIdx]}
									{#if hasMonthKey(`${currentYear}-${String(monthIdx + 1).padStart(2, '0')}`)}
										<span class="mini-month-dot" title="该月已有已保存的分析"></span>
									{/if}
								</div>
								<div class="mini-cal-grid">
									{#each weekDaysShort as wd}
										<div class="mini-weekday">{wd}</div>
									{/each}
									{#each getMiniCalendarDays(currentYear, monthIdx) as day}
										{#if day === null}
											<div class="mini-day-empty"></div>
										{:else}
											<!-- svelte-ignore a11y-click-events-have-key-events -->
											<div
												class="mini-day"
												class:mini-day-today={isTodayMini(monthIdx, day)}
												class:mini-day-has-diary={yearHasDiary(monthIdx, day)}
												onclick={(e) => handleMiniDayClick(e, monthIdx, day)}
												role="button"
												tabindex="-1"
											>
												<span class="mini-day-number">{day}</span>
											</div>
										{/if}
									{/each}
								</div>
							</button>
						{/each}
					</div>
				{/if}
			</div>
		</div>
	{/if}

	{#if analysis}
		<CalendarAnalysis
			mode={analysis.mode}
			period={analysis.period}
			key={analysis.key}
			start={analysis.start}
			end={analysis.end}
			onClose={closeAnalysis}
		/>
	{/if}

	{#if yearPickerOpen}
		<CalendarYearPicker
			currentYear={{
				set: (v: number) => { currentYear = v; },
				value: currentYear
			}}
			currentMonth={{
				set: (v: number) => { currentMonth = v; },
				value: currentMonth
			}}
			onClose={closeYearPicker}
			onMonthChange={onmonthchange}
			onSelectWeek={openWeekAnalysisFor}
			onSelectMonthReport={openMonthAnalysisFor}
			onSelectYearReport={openYearAnalysisFor}
		/>
	{/if}

	<!-- 往昔今朝 Modal -->
{#if onThisDay.active}
    <div class="analysis-overlay" bind:this={onThisDayOverlayEl} onclick={closeOnThisDay}>
        <div class="analysis-panel" onclick={(e) => e.stopPropagation()}>
            <div class="analysis-header">
                <div class="analysis-header-main">
                    <div class="flex items-center gap-2 justify-center">
                        <svg class="w-5 h-5 text-primary" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                        </svg>
                        <h3>往昔今朝</h3>
                    </div>
                    <p class="analysis-header-sub">{formatDisplayDate(onThisDay.date)}</p>
                </div>
                <button class="analysis-close" onclick={closeOnThisDay} aria-label="关闭">×</button>
            </div>

            <div class="analysis-body">
                {#if onThisDay.loading}
                    <div class="analysis-loading">
                        <div class="spinner" aria-hidden="true"></div>
                        <p>正在翻找往年的日记…</p>
                    </div>
                {:else if onThisDay.diaries.length === 0}
                    <div class="analysis-idle">
                        <p class="analysis-idle-title">这一天在往年还没有日记</p>
                        <p class="analysis-idle-sub">继续记录每一天，明年的今天就会有可以回顾的内容了。</p>
                    </div>
                {:else}
                    <div class="analysis-idle-sub" style="margin-bottom:0.75rem">共 {onThisDay.diaries.length} 个不同年份的今日</div>
                    <div class="analysis-list">
                        {#each onThisDay.diaries as diary}
                            <a href="/diary/{diary.date}" class="analysis-list-item">
                                <div class="analysis-list-head">
                                    <span class="analysis-list-date">{formatDisplayDate(diary.date)}</span>
                                    <div class="flex items-center gap-2 text-xs text-muted-foreground">
                                        {#if diary.mood}<span title="心情"><MoodIcon mood={diary.mood} size={14} /></span>{/if}
                                        {#if diary.weather && isWMOCode(diary.weather)}
                                            {@const weatherInfo = getWeatherInfo(parseInt(diary.weather))}
                                            <span title="天气：{weatherInfo.label}{diary.temp_min != null && diary.temp_max != null ? ` ${Math.round(diary.temp_min)}°~${Math.round(diary.temp_max)}°` : ''}">{weatherInfo.icon}</span>
                                        {/if}
                                    </div>
                                </div>
                                {#if diary.content}
                                    <p class="analysis-list-preview">{diaryContentPreview(diary.content)}</p>
                                {/if}
                                {#if diary.tags && diary.tags.length > 0}
                                    <div class="flex flex-wrap gap-1 mt-2">
                                        {#each diary.tags as tag}
                                            <span class="analysis-list-tag">#{tag}</span>
                                        {/each}
                                    </div>
                                {/if}
                            </a>
                        {/each}
                    </div>
                {/if}
            </div>
        </div>
    </div>
{/if}

<!-- 时空穿越 Modal -->
{#if randomState.active}
    <div class="analysis-overlay" bind:this={randomOverlayEl} onclick={closeRandom}>
        <div class="analysis-panel" onclick={(e) => e.stopPropagation()}>
            <div class="analysis-header">
                <div class="analysis-header-main">
                    <div class="flex items-center gap-2 justify-center">
                        <svg class="w-5 h-5 text-primary" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9H4m16 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H20" />
                        </svg>
                        <h3>时空穿越</h3>
                    </div>
                    <p class="analysis-header-sub">随机翻阅一条过去的日记</p>
                </div>
                <button class="analysis-close" onclick={closeRandom} aria-label="关闭">×</button>
            </div>

            <div class="analysis-toolbar">
                <button class="analysis-toggle" onclick={rerollRandom}>
                    <span class="inline-flex items-center gap-1.5">
                        <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9H4m16 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H20" />
                        </svg>
                        再抽一次
                    </span>
                </button>
                <a
                    href={randomState.diary ? `/diary/${randomState.diary.date}` : ''}
                    class="analysis-toggle analysis-toggle--active ml-auto"
                >
                    查看完整日记
                </a>
            </div>

            <div class="analysis-body">
                {#if randomState.loading}
                    <div class="analysis-loading">
                        <div class="spinner" aria-hidden="true"></div>
                        <p>随机抽取一段过往日记…</p>
                    </div>
                {:else if !randomState.exists || !randomState.diary}
                    <div class="analysis-idle">
                        <p class="analysis-idle-title">还没有可翻阅的日记</p>
                        <p class="analysis-idle-sub">先开始记录你的生活吧，有了足够的日记后再试试这个功能。</p>
                    </div>
                {:else}
                    <div class="analysis-idle-sub" style="margin-bottom:0.75rem">{formatDisplayDate(randomState.diary.date)}</div>
                    <div class="analysis-list">
                        <div class="analysis-list-item">
                            <div class="text-sm text-foreground/90 leading-relaxed whitespace-pre-wrap">
                                {diaryContentPreview(randomState.diary.content, 280)}
                            </div>
                            {#if randomState.diary.tags && randomState.diary.tags.length > 0}
                                <div class="flex flex-wrap gap-1 mt-3">
                                    {#each randomState.diary.tags as tag}
                                        <span class="analysis-list-tag">#{tag}</span>
                                    {/each}
                                </div>
                            {/if}
                        </div>
                    </div>
                {/if}
            </div>
        </div>
    </div>
{/if}
</div>

<style>
	.calendar {
		width: 100%;
		/* 周报按钮列宽度：与周行末尾的独立按钮列对齐 */
		--week-action-w: 4rem;
	}

	/* Two-column layout (xl+): let calendar fill the left column height. */
	@media (min-width: 1280px) {
		.calendar {
			height: 100%;
			display: flex;
			flex-direction: column;
			min-height: 0;
			flex: 1;
		}
	}

	@media (max-width: 480px) {
		.calendar {
			--week-action-w: 3.4rem;
		}
	}

	.view-container {
		width: 100%;
		max-width: none;
		margin-left: auto;
		margin-right: auto;
		display: flex;
		flex-direction: column;
		flex: 1;
		min-height: 0;
	}

	/* Two-column layout (xl+): cap calendar width to keep cells reasonable. */
	@media (min-width: 1280px) {
		.view-container:not(.year-mode) {
			max-width: 900px;
		}
	}

	/* On truly wide screens (~1440px+), let the calendar spread out further. */
	@media (min-width: 1440px) {
		.view-container:not(.year-mode) {
			max-width: 1000px;
		}
	}

	/* Year view: use a slightly wider container on larger screens so the
	   4-column month grid has enough room for each day cell. */
	@media (min-width: 780px) {
		.view-container.year-mode {
			max-width: 760px;
		}
	}

	/* Compact month styling when 4 columns are in use — less padding
	   inside each mini month so the 7 day-columns fit horizontally. */
	@media (min-width: 780px) {
		.view-container.year-mode .mini-month {
			padding: 0.375rem;
		}
		.view-container.year-mode .mini-month-name {
			font-size: 0.75rem;
			margin-bottom: 0.2rem;
		}
		.view-container.year-mode .mini-day {
			font-size: 0.65rem;
		}
		.view-container.year-mode .mini-weekday {
			font-size: 0.5rem;
		}
	}

	/* Month view: weekdays header + days grid
	   Cap the overall grid width so cells don't get too huge on 2K+ displays,
	   while still using 100% on normal/laptop sizes. */
	.weekdays-grid {
		display: grid;
		grid-template-columns: repeat(7, 1fr) var(--week-action-w);
		gap: 0.5rem;
		margin-bottom: 0.5rem;
	}

	.week-header-action {
		text-align: center;
		font-size: 0.75rem;
		font-weight: 500;
		color: hsl(var(--muted-foreground) / 0.7);
		padding: 0.5rem 0;
	}

	.days-grid {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}

	/* Two-column layout (xl+): let the day grid stretch to fill height. */
	@media (min-width: 1280px) {
		.days-grid {
			flex: 1;
			min-height: 0;
		}
	}

	/* 周（行）：ISO 8601 一周一行，7 天 + 末尾独立的周报按钮列 */
	.week-row {
		position: relative;
		display: grid;
		grid-template-columns: repeat(7, 1fr) var(--week-action-w);
		gap: 0.5rem;
		border-radius: 0.5rem;
		cursor: pointer;
		transition: background-color 0.15s ease;
	}

	/* Two-column layout (xl+): distribute extra height across rows. */
	@media (min-width: 1280px) {
		.week-row {
			flex: 1;
			min-height: 0;
		}
	}

	/* Calendar day cell — on desktop (two-column) fill the row height instead
	   of staying square. On tablets/phones keep aspect-square so cells look
	   balanced and don't turn into tall thin rectangles. */
	.day {
		aspect-ratio: 1;
	}
	@media (min-width: 1280px) {
		.day {
			aspect-ratio: auto;
			height: 100%;
			min-height: 48px;
		}
	}

	.week-row:hover {
		background: hsl(var(--primary) / 0.05);
	}

	/* 周报按钮：日历右侧独立列，不占用日期格子。
	   无已保存分析 → “分析该周”（弱化样式）；已有 → “查看”（主色填充）。 */
	.week-action {
		align-self: center;
		width: 100%;
		padding: 0.3rem 0.2rem;
		font-size: 0.65rem;
		line-height: 1.2;
		text-align: center;
		border-radius: 0.5rem;
		border: 1px solid hsl(var(--border) / 0.7);
		background: hsl(var(--muted) / 0.3);
		color: hsl(var(--muted-foreground));
		cursor: pointer;
		white-space: nowrap;
		transition: background-color 0.15s ease, color 0.15s ease, border-color 0.15s ease;
	}

	.week-action:hover {
		background: hsl(var(--primary) / 0.1);
		border-color: hsl(var(--primary) / 0.4);
		color: hsl(var(--primary));
	}

	.week-action--view {
		background: hsl(var(--primary) / 0.85);
		border-color: hsl(var(--primary));
		color: hsl(var(--primary-foreground));
		font-weight: 500;
	}

	.week-action--view:hover {
		background: hsl(var(--primary));
		color: hsl(var(--primary-foreground));
	}

	/* 月报 / 年报：单行两张紧凑卡片 */
	.period-grid {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 0.5rem;
		margin-top: 0.75rem;
		flex-shrink: 0;
	}

	.period-card {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 0.5rem;
		padding: 0.5rem 0.7rem;
		border: 1px solid hsl(var(--border) / 0.6);
		border-radius: 0.625rem;
		background: hsl(var(--muted) / 0.2);
		cursor: pointer;
		text-align: left;
		transition: background-color 0.15s ease, border-color 0.15s ease;
	}

	.period-card:hover {
		border-color: hsl(var(--primary) / 0.4);
		background: hsl(var(--primary) / 0.05);
	}

	.period-card-info {
		display: flex;
		flex-direction: column;
		gap: 0.1rem;
		min-width: 0;
	}

	.period-card-name {
		font-size: 0.8rem;
		font-weight: 600;
		color: hsl(var(--foreground));
	}

	.period-card-desc {
		font-size: 0.72rem;
		color: hsl(var(--muted-foreground));
	}

	/* 状态胶囊：未生成 → 灰色“分析”；已保存 → 主题色“查看” */
	.period-card-state {
		flex-shrink: 0;
		font-size: 0.7rem;
		line-height: 1.2;
		padding: 0.22rem 0.6rem;
		border-radius: 9999px;
		border: 1px solid hsl(var(--border) / 0.7);
		background: hsl(var(--muted) / 0.3);
		color: hsl(var(--muted-foreground));
		white-space: nowrap;
	}

	.period-card-state--saved {
		background: hsl(var(--primary) / 0.85);
		border-color: hsl(var(--primary));
		color: hsl(var(--primary-foreground));
		font-weight: 500;
	}

	/* Year button in month view header */
	.year-button {
		display: inline-flex;
		align-items: center;
		padding: 0.0625rem 0.375rem;
		border-radius: 0.375rem;
		transition: background-color, color, transform, opacity 0.2s ease;
		position: relative;
		white-space: nowrap;
		flex-shrink: 0;
		font-size: 0.75rem;
	}
	@media (min-width: 640px) {
		.year-button {
			padding: 0.125rem 0.5rem;
			font-size: 0.875rem;
		}
	}

	.year-button::after {
		content: '';
		position: absolute;
		bottom: 0;
		left: 50%;
		transform: translateX(-50%);
		width: 0;
		height: 1.5px;
		background: hsl(var(--primary));
		transition: width 0.2s ease;
	}

	.year-button:hover {
		color: hsl(var(--primary));
		background: hsl(var(--primary) / 0.08);
	}

	.year-button:hover::after {
		width: 70%;
	}

	/* Year grid container: no height cap so all 12 months are visible */
	.year-scroll-container {
		width: 100%;
		position: relative;
	}

	/* Year grid: default is 3 columns × 4 rows; on screens wide enough
	   for 4 columns per row and all 7 day-columns to fit inside, switch
	   to 4×3 to keep the overall layout shorter. */
	.year-grid {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: 0.5rem;
	}

	@media (min-width: 780px) {
		.year-grid {
			grid-template-columns: repeat(4, 1fr);
		}
	}

	@media (max-width: 500px) {
		.year-grid {
			grid-template-columns: repeat(2, 1fr);
			gap: 0.375rem;
		}
	}

	/* Mini month card */
	.mini-month {
		display: flex;
		flex-direction: column;
		padding: 0.5rem;
		border-radius: 0.625rem;
		border: 1px solid hsl(var(--border) / 0.5);
		background: hsl(var(--card));
		transition: background-color, border-color, color, transform, opacity 0.2s ease;
		cursor: pointer;
		text-align: left;
		animation: mini-month-in 0.3s ease-out both;
	}

	.mini-month:hover {
		border-color: hsl(var(--primary) / 0.4);
		background: hsl(var(--primary) / 0.04);
		box-shadow: 0 2px 8px hsl(var(--primary) / 0.08);
		transform: translateY(-1px);
	}

	.mini-month-current {
		border-color: hsl(var(--primary) / 0.3);
		background: hsl(var(--primary) / 0.04);
	}

	@keyframes mini-month-in {
		from {
			opacity: 0;
			transform: scale(0.95);
		}
		to {
			opacity: 1;
			transform: scale(1);
		}
	}

	/* Mini month name */
	.mini-month-name {
		display: flex;
		align-items: center;
		gap: 0.3rem;
		font-size: 0.8125rem;
		font-weight: 600;
		margin-bottom: 0.25rem;
		padding-left: 0.125rem;
		color: hsl(var(--foreground));
	}

	/* 月报标记点：该月已有已保存的分析 */
	.mini-month-dot {
		width: 6px;
		height: 6px;
		border-radius: 9999px;
		background: hsl(var(--primary));
		flex-shrink: 0;
	}

	/* Mini calendar grid */
	.mini-cal-grid {
		display: grid;
		grid-template-columns: repeat(7, 1fr);
		gap: 1px;
	}

	.mini-weekday {
		text-align: center;
		font-size: 0.55rem;
		color: hsl(var(--muted-foreground) / 0.7);
		padding: 2px 0;
	}

	.mini-day {
		aspect-ratio: 1;
		display: flex;
		align-items: center;
		justify-content: center;
		font-size: 0.7rem;
		border-radius: 3px;
		color: hsl(var(--foreground) / 0.85);
		cursor: pointer;
		transition: background-color, color, opacity 0.15s ease;
	}

	.mini-day:hover {
		background: hsl(var(--primary) / 0.1);
		color: hsl(var(--foreground));
	}

	.mini-day-today {
		background: hsl(var(--primary) / 0.2);
		color: hsl(var(--primary));
		font-weight: 600;
	}

	.mini-day-has-diary {
		background: hsl(38, 100%, 50% / 0.15);
	}

	.mini-day-empty {
		aspect-ratio: 1;
	}

	/* 分析弹窗 / 往昔今朝 / 时空穿越 — 复用 CalendarAnalysis 样式 */
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
    animation: fade-in 0.15s ease-out both;
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
    margin: 0 auto;
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
    justify-content: flex-end;
    gap: 0.5rem;
    flex-wrap: wrap;
    border-bottom: 1px solid hsl(var(--border) / 0.4);
}

.analysis-toggle {
    padding: 0.4rem 0.85rem;
    font-size: 0.8rem;
    border: 1px solid hsl(var(--border) / 0.7);
    background: hsl(var(--muted) / 0.3);
    color: hsl(var(--foreground) / 0.85);
    border-radius: 0.5rem;
    cursor: pointer;
    transition: background 0.15s ease, color 0.15s ease;
    text-decoration: none;
}

.analysis-toggle:hover {
    background: hsl(var(--muted) / 0.7);
    color: hsl(var(--foreground));
}

.analysis-toggle--active {
    border-color: hsl(var(--primary) / 0.3);
    background: hsl(var(--primary) / 0.1);
    color: hsl(var(--primary));
    font-weight: 500;
}

.analysis-body {
    padding: 1.25rem;
    overflow-y: auto;
    flex: 1;
}

.analysis-loading {
    display: flex;
    flex-direction: column;
    align-items: center;
    padding: 2.5rem 1rem;
    color: hsl(var(--muted-foreground));
    gap: 0.75rem;
}

.analysis-idle {
    padding: 2.5rem 1rem;
    text-align: center;
    color: hsl(var(--muted-foreground));
    display: flex;
    flex-direction: column;
    align-items: center;
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

.spinner {
    width: 1.5rem;
    height: 1.5rem;
    border: 3px solid hsl(var(--border));
    border-top-color: hsl(var(--primary));
    border-radius: 50%;
    animation: spin 0.7s linear infinite;
}

@keyframes spin {
    to { transform: rotate(360deg); }
}

.analysis-list {
    width: 100%;
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
    transition: background 0.15s ease, border-color 0.15s ease;
    text-decoration: none;
    color: inherit;
}

.analysis-list-item:hover {
    background: hsl(var(--muted) / 0.5);
    border-color: hsl(var(--primary) / 0.35);
}

.analysis-list-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.6rem;
    flex-wrap: wrap;
    margin-bottom: 0.35rem;
}

.analysis-list-date {
    font-size: 0.9rem;
    color: hsl(var(--foreground));
    font-weight: 500;
}

.analysis-list-preview {
    margin: 0.15rem 0 0.4rem;
    font-size: 0.85rem;
    line-height: 1.55;
    color: hsl(var(--foreground) / 0.75);
    white-space: pre-wrap;
}

.analysis-list-tag {
    display: inline-flex;
    align-items: center;
    gap: 0.15rem;
    background: hsl(var(--primary) / 0.10);
    color: hsl(var(--primary));
    border: 1px solid hsl(var(--primary) / 0.2);
    border-radius: 9999px;
    padding: 0.1rem 0.45rem;
    font-size: 0.7rem;
}

@keyframes fade-in {
    from { opacity: 0; }
    to { opacity: 1; }
}

@keyframes panel-in {
    from { opacity: 0; transform: translateY(8px) scale(0.98); }
    to { opacity: 1; transform: translateY(0) scale(1); }
}
</style>

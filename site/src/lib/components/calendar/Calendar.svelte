<script lang="ts">
	import { goto } from '$app/navigation';
	import { tick } from 'svelte';
	import { formatDate, getCalendarDays, getToday, getYearRange, getMonthRange, getWeekRange, formatMonthYear } from '$lib/utils/date';
	import { getDatesWithDiaries, type CalendarDiaryMeta } from '$lib/api/diaries';
	import CalendarAnalysis from './CalendarAnalysis.svelte';

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
	let yearGridEl: $state<HTMLDivElement | null> = null;
	let wheelCooldown = $state(false);
	let yearPickerOpen = $state(false);
	let tempPickerYear = $state(currentYear);

	type AnalysisState = {
		active: boolean;
		mode?: 'single' | 'history';
		period: 'week' | 'month';
		start: string;
		end: string;
	} | null;
	let analysis = $state<AnalysisState>(null);

	function openWeekAnalysis() {
		const { start, end } = getWeekRange(new Date());
		analysis = { active: true, mode: 'single', period: 'week', start, end };
	}

	function openMonthAnalysis() {
		const { start, end } = getMonthRange(currentYear, currentMonth);
		analysis = { active: true, mode: 'single', period: 'month', start, end };
	}

	function openHistoryAnalysis() {
		const { start, end } = getMonthRange(currentYear, currentMonth);
		analysis = { active: true, mode: 'history', period: 'month', start, end };
	}

	function closeAnalysis() {
		analysis = null;
	}

	const weekDays = ['周一', '周二', '周三', '周四', '周五', '周六', '周日'];
	const weekDaysShort = ['一', '二', '三', '四', '五', '六', '日'];
	const monthNamesShort = [
		'一', '二', '三', '四', '五', '六',
		'七', '八', '九', '十', '十一', '十二'
	];

	const calendarDays = $derived(getCalendarDays(currentYear, currentMonth));
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
		return metaByDate.has(formatDate(date));
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
		tempPickerYear = currentYear;
		yearPickerOpen = true;
	}

	function closeYearPicker() {
		yearPickerOpen = false;
	}

	function selectMonthFromPicker(month: number) {
		currentYear = tempPickerYear;
		currentMonth = month;
		yearPickerOpen = false;
		onmonthchange();
	}

	function pickerPrevYear() {
		tempPickerYear = tempPickerYear - 1;
	}

	function pickerNextYear() {
		tempPickerYear = tempPickerYear + 1;
	}

	function pickerGoCurrent() {
		const today = new Date();
		tempPickerYear = today.getFullYear();
	}

	const monthShortNames = [
		'一月', '二月', '三月', '四月', '五月', '六月',
		'七月', '八月', '九月', '十月', '十一月', '十二月'
	];

	async function enterYearView() {
		viewMode = 'year';
		await loadYearData(currentYear);
		scrollToCurrentMonth();
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
		return yearMetaByDate.has(dateStr);
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

	function handleYearWheel(e: WheelEvent) {
		if (!yearGridEl || wheelCooldown || yearLoading) return;

		const el = yearGridEl;
		const atTop = el.scrollTop <= 0;
		const atBottom = el.scrollTop + el.clientHeight >= el.scrollHeight - 1;

		if ((atTop && e.deltaY < 0) || (atBottom && e.deltaY > 0)) {
			e.preventDefault();
			wheelCooldown = true;
			if (e.deltaY < 0) {
				goToPreviousYear();
			} else {
				goToNextYear();
			}
			setTimeout(() => {
				wheelCooldown = false;
			}, 600);
		}
	}

	async function scrollToCurrentMonth() {
		await tick();
		if (!yearGridEl) return;
		const today = new Date();
		if (currentYear !== today.getFullYear()) return;
		const targetMonth = today.getMonth();
		const cards = yearGridEl.querySelectorAll('.mini-month');
		if (cards[targetMonth]) {
			cards[targetMonth].scrollIntoView({ block: 'center', behavior: 'smooth' });
		}
	}
</script>

<div class="calendar">
	{#if viewMode === 'month'}
		<!-- Month View -->
		<div class="view-container animate-fade-in-only">
			<!-- Calendar Header -->
			<div class="flex flex-col gap-3 mb-5 px-2">
				<!-- 第一行：月份导航 -->
				<div class="flex items-center justify-between">
					<button
						onclick={goToPreviousMonth}
						class="p-2 rounded-lg hover:bg-muted/50 transition-all duration-200"
						title="上一月"
					>
						<svg class="w-5 h-5 text-foreground" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
						</svg>
					</button>

					<div class="flex items-center gap-2">
						<h2 class="text-lg font-semibold text-foreground flex items-center gap-1.5">
							<button
								onclick={openYearPicker}
								class="hover:bg-muted/50 transition-all duration-200 rounded-md px-2 py-1 flex items-center gap-1"
								title="点击选择年月"
							>
								<span>{formatMonthYear(currentYear, currentMonth)}</span>
								<svg class="w-3.5 h-3.5 text-muted-foreground" fill="none" stroke="currentColor" viewBox="0 0 24 24">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
								</svg>
							</button>
							<button
								onclick={enterYearView}
								class="year-button"
								title="切换到年视图"
							>
								查看全年
							</button>
						</h2>
						<button
							onclick={goToToday}
							class="px-3 py-1 text-sm bg-primary text-primary-foreground rounded-md hover:opacity-90 transition-all duration-200"
						>
							今天
						</button>
					</div>

					<button
						onclick={goToNextMonth}
						class="p-2 rounded-lg hover:bg-muted/50 transition-all duration-200"
						title="下一月"
					>
						<svg class="w-5 h-5 text-foreground" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
						</svg>
					</button>
				</div>

				<!-- 第二行：紧凑 AI 分析按钮 -->
				<div class="flex items-center justify-center gap-1.5">
					<button
						onclick={openWeekAnalysis}
						class="px-2.5 py-1 text-xs rounded-md border border-border bg-muted/30 text-muted-foreground hover:text-foreground hover:bg-muted/60 transition-all duration-200"
						title="本周 AI 分析"
					>
						<span class="inline-flex items-center gap-1">
							<svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
							</svg>
							周分析
						</span>
					</button>
					<button
						onclick={openMonthAnalysis}
						class="px-2.5 py-1 text-xs rounded-md border border-border bg-muted/30 text-muted-foreground hover:text-foreground hover:bg-muted/60 transition-all duration-200"
						title="本月 AI 分析"
					>
						<span class="inline-flex items-center gap-1">
							<svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 7h.01M7 3h5a1.99 1.99 0 011.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A1.99 1.99 0 013 12V7a4 4 0 014-4z" />
							</svg>
							月分析
						</span>
					</button>
					<button
						onclick={openHistoryAnalysis}
						class="px-2.5 py-1 text-xs rounded-md border border-border bg-muted/30 text-muted-foreground hover:text-foreground hover:bg-muted/60 transition-all duration-200"
						title="查看历史分析"
					>
						<span class="inline-flex items-center gap-1">
							<svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
							</svg>
							历史分析
						</span>
					</button>
				</div>
			</div>

			<!-- Week Days -->
			<div class="grid grid-cols-7 gap-2 mb-2">
				{#each weekDays as day}
					<div class="text-center font-medium text-muted-foreground text-sm py-2">{day}</div>
				{/each}
			</div>

			<!-- Calendar Days -->
			<div class="grid grid-cols-7 gap-2">
				{#each calendarDays as date, i}
					<button
						onclick={() => handleDateClick(date)}
						class="day aspect-square rounded-lg transition-all duration-200 flex flex-col items-center justify-center relative
							   {isCurrentMonth(date) ? 'text-foreground' : 'text-muted-foreground/40'}
							   {isToday(date) ? 'bg-primary/10 ring-2 ring-primary font-semibold' : ''}
							   {hasDiary(date) && !isToday(date) ? 'bg-amber-500/10 dark:bg-amber-500/20' : ''}
							   {!isToday(date) && !hasDiary(date) ? 'hover:bg-muted/50' : ''}
							   {hasDiary(date) && !isToday(date) ? 'hover:bg-amber-500/20 dark:hover:bg-amber-500/30' : ''}"
						style="animation-delay: {i * 10}ms"
					>
						<span class="text-sm">{date.getDate()}</span>

						{#if hasDiary(date)}
							{@const meta = getDateMeta(date)}
							{#if meta?.weather || meta?.mood}
								<div class="absolute inset-x-0 top-1.5 flex items-center justify-center gap-1 text-[11px] leading-none">
									{#if meta?.weather}
										<span class="emoji-chip" title="天气：{meta.weather}">{meta.weather}</span>
									{/if}
									{#if meta?.mood}
										<span class="emoji-chip" title="心情：{meta.mood}">{meta.mood}</span>
									{/if}
								</div>
							{:else}
								<span class="absolute bottom-1 w-1 h-1 bg-amber-500 rounded-full"></span>
							{/if}
						{/if}
					</button>
				{/each}
			</div>
		</div>
	{:else}
		<!-- Year View -->
		<div class="view-container animate-fade-in-only">
			<!-- Year Header -->
			<div class="flex items-center justify-between mb-5 px-2">
				<button
					onclick={goToPreviousYear}
					class="p-2 rounded-lg hover:bg-muted/50 transition-all duration-200"
					title="上一年"
				>
					<svg class="w-5 h-5 text-foreground" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
					</svg>
				</button>

				<div class="flex items-center gap-3">
					<h2 class="text-lg font-semibold text-foreground">{currentYear}</h2>
					<button
						onclick={goToCurrentYear}
						class="px-3 py-1 text-sm bg-primary text-primary-foreground rounded-md hover:opacity-90 transition-all duration-200"
					>
						本年
					</button>
				</div>

				<button
					onclick={goToNextYear}
					class="p-2 rounded-lg hover:bg-muted/50 transition-all duration-200"
					title="下一年"
				>
					<svg class="w-5 h-5 text-foreground" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
					</svg>
				</button>
			</div>

			<!-- Year Grid (scrollable) -->
			<!-- svelte-ignore a11y-no-static-element-interactions -->
			<div
				class="year-scroll-container"
				bind:this={yearGridEl}
				onwheel={handleYearWheel}
			>
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

					<!-- Scroll hint -->
					<div class="scroll-hint">
						<span class="scroll-hint-text">滚动切换年份</span>
					</div>
				{/if}
			</div>
		</div>
	{/if}

	{#if analysis}
		<CalendarAnalysis
			mode={analysis.mode}
			period={analysis.period}
			start={analysis.start}
			end={analysis.end}
			onClose={closeAnalysis}
		/>
	{/if}

	{#if yearPickerOpen}
		<div class="year-picker-overlay" onclick={closeYearPicker} onkeydown={(e) => { if (e.key === 'Escape') closeYearPicker(); }} tabindex="0">
			<div class="year-picker-panel" onclick={(e) => e.stopPropagation()}>
				<!-- 顶部：年份切换 -->
				<div class="year-picker-header">
					<button
						type="button"
						onclick={pickerPrevYear}
						class="year-picker-nav"
						title="上一年"
					>
						<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
						</svg>
					</button>

					<div class="year-picker-title">
						<button
							type="button"
							onclick={pickerGoCurrent}
							class="year-picker-year-btn"
							title="回到今年"
						>
							{tempPickerYear} 年
						</button>
					</div>

					<button
						type="button"
						onclick={pickerNextYear}
						class="year-picker-nav"
						title="下一年"
					>
						<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
						</svg>
					</button>
				</div>

				<!-- 月份网格 -->
				<div class="year-picker-grid">
					{#each monthShortNames as monthName, i}
						{@const month = i + 1}
						{@const isSelected = tempPickerYear === currentYear && month === currentMonth}
						<button
							type="button"
							onclick={() => selectMonthFromPicker(month)}
							class="year-picker-month"
							class:year-picker-month-selected={isSelected}
							title={`${tempPickerYear} 年 ${month} 月`}
						>
							{monthName}
						</button>
					{/each}
				</div>

				<!-- 底部快捷：快速跳转到今年 + 关闭 -->
				<div class="year-picker-footer">
					<button
						type="button"
						onclick={() => {
							const today = new Date();
							currentYear = today.getFullYear();
							currentMonth = today.getMonth() + 1;
							yearPickerOpen = false;
							onmonthchange();
						}}
						class="year-picker-today-btn"
					>
						跳转到今天
					</button>
					<button
						type="button"
						onclick={closeYearPicker}
						class="year-picker-cancel-btn"
					>
						取消
					</button>
				</div>
			</div>
		</div>
	{/if}
</div>

<style>
	.calendar {
		width: 100%;
	}

	.view-container {
		width: 100%;
	}

	/* Year button in month view header */
	.year-button {
		display: inline-flex;
		align-items: center;
		padding: 0.125rem 0.5rem;
		border-radius: 0.375rem;
		transition: all 0.2s ease;
		position: relative;
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

	/* Scrollable year container */
	.year-scroll-container {
		max-height: 440px;
		overflow-y: auto;
		overflow-x: hidden;
		scroll-behavior: smooth;
		-webkit-overflow-scrolling: touch;
		padding-right: 2px;
		position: relative;
	}

	/* Fade edges to hint scrollability */
	.year-scroll-container::after {
		content: '';
		position: sticky;
		bottom: 0;
		left: 0;
		right: 0;
		display: block;
		height: 24px;
		background: linear-gradient(to top, hsl(var(--card)), transparent);
		pointer-events: none;
		margin-top: -24px;
	}

	/* Scroll hint text */
	.scroll-hint {
		display: flex;
		justify-content: center;
		padding: 0.5rem 0 0.25rem;
	}

	.scroll-hint-text {
		font-size: 0.6875rem;
		color: hsl(var(--muted-foreground) / 0.5);
		user-select: none;
	}

	/* Year grid */
	.year-grid {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: 0.5rem;
	}

	@media (max-width: 640px) {
		.year-grid {
			grid-template-columns: repeat(2, 1fr);
			gap: 0.375rem;
		}

		.year-scroll-container {
			max-height: 380px;
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
		transition: all 0.2s ease;
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
		font-size: 0.8125rem;
		font-weight: 600;
		margin-bottom: 0.25rem;
		padding-left: 0.125rem;
		color: hsl(var(--foreground));
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
		transition: all 0.15s ease;
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

	/* ===================== 年月选择器 ===================== */
	.year-picker-overlay {
		position: fixed;
		inset: 0;
		background: hsl(var(--background) / 0.6);
		backdrop-filter: blur(4px);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 2147483647;
		animation: fadeIn 0.15s ease-out;
	}

	@keyframes fadeIn {
		from { opacity: 0; }
		to { opacity: 1; }
	}

	.year-picker-panel {
		background: hsl(var(--card));
		border: 1px solid hsl(var(--border));
		border-radius: 14px;
		padding: 1rem 1.25rem 1.1rem;
		min-width: 300px;
		box-shadow: 0 18px 45px hsl(var(--foreground) / 0.18), 0 0 0 1px hsl(var(--border) / 0.4);
		animation: pickerIn 0.18s ease-out;
	}

	@keyframes pickerIn {
		from {
			opacity: 0;
			transform: translateY(-6px) scale(0.98);
		}
		to {
			opacity: 1;
			transform: translateY(0) scale(1);
		}
	}

	.year-picker-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		margin-bottom: 0.9rem;
		padding-bottom: 0.75rem;
		border-bottom: 1px solid hsl(var(--border) / 0.7);
	}

	.year-picker-nav {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 2rem;
		height: 2rem;
		border-radius: 8px;
		color: hsl(var(--foreground) / 0.75);
		transition: all 0.15s ease;
	}

	.year-picker-nav:hover {
		background: hsl(var(--muted) / 0.6);
		color: hsl(var(--foreground));
	}

	.year-picker-title {
		display: flex;
		align-items: center;
		justify-content: center;
		flex: 1;
	}

	.year-picker-year-btn {
		font-size: 1.05rem;
		font-weight: 600;
		color: hsl(var(--foreground));
		padding: 0.25rem 0.75rem;
		border-radius: 8px;
		transition: background 0.15s ease;
	}

	.year-picker-year-btn:hover {
		background: hsl(var(--primary) / 0.08);
		color: hsl(var(--primary));
	}

	.year-picker-grid {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: 0.5rem;
	}

	.year-picker-month {
		padding: 0.7rem 0.5rem;
		border-radius: 10px;
		font-size: 0.9rem;
		color: hsl(var(--foreground) / 0.85);
		background: hsl(var(--muted) / 0.3);
		border: 1px solid transparent;
		transition: all 0.15s ease;
		font-weight: 500;
	}

	.year-picker-month:hover {
		background: hsl(var(--primary) / 0.1);
		color: hsl(var(--primary));
		transform: translateY(-1px);
		border-color: hsl(var(--primary) / 0.25);
	}

	.year-picker-month-selected {
		background: hsl(var(--primary)) !important;
		color: hsl(var(--primary-foreground)) !important;
		font-weight: 600;
		border-color: transparent;
	}

	.year-picker-footer {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 0.75rem;
		margin-top: 1rem;
		padding-top: 0.75rem;
		border-top: 1px solid hsl(var(--border) / 0.7);
	}

	.year-picker-today-btn {
		flex: 1;
		padding: 0.5rem 0.75rem;
		border-radius: 8px;
		font-size: 0.85rem;
		color: hsl(var(--primary-foreground));
		background: hsl(var(--primary));
		transition: opacity 0.15s ease;
	}

	.year-picker-today-btn:hover {
		opacity: 0.9;
	}

	.year-picker-cancel-btn {
		padding: 0.5rem 0.9rem;
		border-radius: 8px;
		font-size: 0.85rem;
		color: hsl(var(--muted-foreground));
		background: hsl(var(--muted) / 0.5);
		transition: background 0.15s ease;
	}

	.year-picker-cancel-btn:hover {
		background: hsl(var(--muted) / 0.8);
		color: hsl(var(--foreground));
	}
</style>

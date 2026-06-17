<script lang="ts">
	let {
		currentYear: currentYearBindable,
		currentMonth: currentMonthBindable,
		onClose,
		onMonthChange = (() => {}) as () => void
	}: {
		currentYear: { set: (v: number) => void; value: number };
		currentMonth: { set: (v: number) => void; value: number };
		onClose: () => void;
		onMonthChange?: () => void;
	} = $props();

	// 使用独立的本地变量：因为 $bindable 属性是可读写的
	// 这里我们只关心传入值的 "当前" 只读版本
	let tempYear = $state(0);
	let initialized = $state(false);

	$effect(() => {
		if (!initialized) {
			tempYear = currentYearBindable.value;
			initialized = true;
		}
	});

	const monthShortNames = [
		'一月', '二月', '三月', '四月', '五月', '六月',
		'七月', '八月', '九月', '十月', '十一月', '十二月'
	];

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

	function setRef(node: HTMLDivElement | null) {
		overlayEl = node;
	}

	function handleKey(e: KeyboardEvent) {
		if (e.key === 'Escape') onClose();
	}

	function pickerPrevYear() {
		tempYear = tempYear - 1;
	}

	function pickerNextYear() {
		tempYear = tempYear + 1;
	}

	function pickerGoCurrent() {
		tempYear = new Date().getFullYear();
	}

	function selectMonth(month: number) {
		currentYearBindable.set(tempYear);
		currentMonthBindable.set(month);
		onMonthChange();
		onClose();
	}

	function goToToday() {
		const today = new Date();
		currentYearBindable.set(today.getFullYear());
		currentMonthBindable.set(today.getMonth() + 1);
		onMonthChange();
		onClose();
	}
</script>

<div
	use:setRef
	class="year-picker-overlay"
	onclick={onClose}
	onkeydown={handleKey}
	tabindex="0"
	role="dialog"
	aria-label="选择年月"
>
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
					{tempYear} 年
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
				{@const isSelected = tempYear === currentYearBindable.value && month === currentMonthBindable.value}
				<button
					type="button"
					onclick={() => selectMonth(month)}
					class="year-picker-month"
					class:year-picker-month-selected={isSelected}
					title={`${tempYear} 年 ${month} 月`}
				>
					{monthName}
				</button>
			{/each}
		</div>

		<!-- 底部快捷 -->
		<div class="year-picker-footer">
			<button
				type="button"
				onclick={goToToday}
				class="year-picker-today-btn"
			>
				跳转到今天
			</button>
			<button
				type="button"
				onclick={onClose}
				class="year-picker-cancel-btn"
			>
				取消
			</button>
		</div>
	</div>
</div>

<style>
	.year-picker-overlay {
		position: fixed;
		inset: 0;
		background: hsl(var(--background) / 0.6);
		backdrop-filter: blur(4px);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 2147483647;
		animation: yearPickerFadeIn 0.15s ease-out;
	}

	@keyframes yearPickerFadeIn {
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
		animation: yearPickerIn 0.18s ease-out;
	}

	@keyframes yearPickerIn {
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

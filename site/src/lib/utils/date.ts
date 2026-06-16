/**
 * Format date to YYYY-MM-DD (local timezone)
 */
export function formatDate(date: Date): string {
	const year = date.getFullYear();
	const month = String(date.getMonth() + 1).padStart(2, '0');
	const day = String(date.getDate()).padStart(2, '0');
	return `${year}-${month}-${day}`;
}

/**
 * Parse YYYY-MM-DD string to Date
 */
export function parseDate(dateStr: string): Date {
	return new Date(dateStr + 'T00:00:00');
}

/**
 * Get today's date string
 */
export function getToday(): string {
	return formatDate(new Date());
}

/**
 * Get previous day
 */
export function getPreviousDay(dateStr: string): string {
	const date = parseDate(dateStr);
	date.setDate(date.getDate() - 1);
	return formatDate(date);
}

/**
 * Get next day
 */
export function getNextDay(dateStr: string): string {
	const date = parseDate(dateStr);
	date.setDate(date.getDate() + 1);
	return formatDate(date);
}

/**
 * Format date for display in Chinese format (e.g., "2026年6月16日")
 */
export function formatDisplayDate(dateStr: string): string {
	const date = parseDate(dateStr);
	const year = date.getFullYear();
	const month = date.getMonth() + 1;
	const day = date.getDate();
	return `${year}年${month}月${day}日`;
}

/**
 * Format short date for mobile display in Chinese format (e.g., "6月16日")
 */
export function formatShortDate(dateStr: string): string {
	const date = parseDate(dateStr);
	const month = date.getMonth() + 1;
	const day = date.getDate();
	return `${month}月${day}日`;
}

/**
 * Get day of week in Chinese short format (e.g., "周二")
 */
export function getDayOfWeek(dateStr: string): string {
	const days = ['周日', '周一', '周二', '周三', '周四', '周五', '周六'];
	const date = parseDate(dateStr);
	return days[date.getDay()];
}

/**
 * Check if date is today
 */
export function isToday(dateStr: string): boolean {
	return dateStr === getToday();
}

/**
 * Get start and end of month
 */
export function getMonthRange(year: number, month: number): { start: string; end: string } {
	const start = new Date(year, month - 1, 1);
	const end = new Date(year, month, 0);
	return {
		start: formatDate(start),
		end: formatDate(end)
	};
}

/**
 * Get start and end of year
 */
export function getYearRange(year: number): { start: string; end: string } {
	return {
		start: `${year}-01-01`,
		end: `${year}-12-31`
	};
}

/**
 * Get calendar days for a month (including padding days), week starts on Monday
 */
export function getCalendarDays(year: number, month: number): Date[] {
	const firstDay = new Date(year, month - 1, 1);
	const lastDay = new Date(year, month, 0);
	// 0 = Sunday -> shift to Monday-first: Monday=0, Sunday=6
	const startDay = (firstDay.getDay() + 6) % 7;
	const daysInMonth = lastDay.getDate();

	const days: Date[] = [];

	// Add padding days from previous month
	for (let i = 0; i < startDay; i++) {
		const day = new Date(year, month - 1, -startDay + i + 1);
		days.push(day);
	}

	// Add days of current month
	for (let i = 1; i <= daysInMonth; i++) {
		days.push(new Date(year, month - 1, i));
	}

	// Add padding days from next month
	const endDay = (lastDay.getDay() + 6) % 7;
	const remainingDays = 6 - endDay;
	for (let i = 1; i <= remainingDays; i++) {
		days.push(new Date(year, month, i));
	}

	return days;
}

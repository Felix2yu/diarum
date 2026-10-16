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
 * Get day of week in Chinese short format (e.g., "二")
 */
export function getDayOfWeek(dateStr: string): string {
	const days = ['日', '一', '二', '三', '四', '五', '六'];
	const date = parseDate(dateStr);
	return days[date.getDay()];
}

/**
 * Format month and year in Chinese (e.g., "2026年6月")
 */
export function formatMonthYear(year: number, month: number): string {
	return `${year}年${month}月`;
}

/**
 * Get Chinese month name (e.g., "六月" for month 6)
 */
export function getMonthName(month: number): string {
	const months = ['一月', '二月', '三月', '四月', '五月', '六月', '七月', '八月', '九月', '十月', '十一月', '十二月'];
	return months[month - 1];
}

/**
 * Format time in Chinese (e.g., "15:30")
 */
export function formatTime(dateStr: string): string {
	const date = parseDate(dateStr);
	const hour = String(date.getHours()).padStart(2, '0');
	const minute = String(date.getMinutes()).padStart(2, '0');
	return `${hour}:${minute}`;
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
 * Get start (Monday) and end (Sunday) of the week containing the given date.
 */
export function getWeekRange(date: Date = new Date()): { start: string; end: string } {
	const input = new Date(date.getFullYear(), date.getMonth(), date.getDate());
	const day = input.getDay(); // 0 = Sunday, 1 = Monday ...
	// Monday = 0 adjustment
	const offsetFromMonday = day === 0 ? 6 : day - 1;
	const start = new Date(input);
	start.setDate(input.getDate() - offsetFromMonday);
	const end = new Date(start);
	end.setDate(start.getDate() + 6);
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

// ---------- ISO 8601 周与特殊日期键 ----------
// 周报使用 ISO 8601 周编号（YYYY-Www，如 2026-W36），月报使用 YYYY-MM。
// ISO 8601：一周从周一开始，包含当年第一个星期四（即 1 月 4 日）的那一周为第 1 周。

/** Parse an ISO 8601 week key like "2026-W36". */
export function parseISOWeekKey(key: string): { year: number; week: number } | null {
	const m = /^(\d{4})-W(\d{2})$/.exec(key);
	if (!m) return null;
	const year = Number(m[1]);
	const week = Number(m[2]);
	if (week < 1 || week > getISOWeeksInYear(year)) return null;
	return { year, week };
}

/** Parse a month key like "2026-01". */
export function parseMonthKey(key: string): { year: number; month: number } | null {
	const m = /^(\d{4})-(\d{2})$/.exec(key);
	if (!m) return null;
	const year = Number(m[1]);
	const month = Number(m[2]);
	if (month < 1 || month > 12) return null;
	return { year, month };
}

/**
 * Get the ISO 8601 week number and week-numbering year of a date.
 * Week 1 is the week containing the first Thursday of the year.
 */
export function getISOWeek(date: Date): { year: number; week: number } {
	const d = new Date(date.getFullYear(), date.getMonth(), date.getDate());
	// Shift to the Thursday of the current ISO week; its year is the ISO year.
	d.setDate(d.getDate() + 3 - ((d.getDay() + 6) % 7));
	const isoYear = d.getFullYear();
	const week1Thu = new Date(isoYear, 0, 4);
	week1Thu.setDate(week1Thu.getDate() + 3 - ((week1Thu.getDay() + 6) % 7));
	const week = Math.round((d.getTime() - week1Thu.getTime()) / (7 * 24 * 3600 * 1000)) + 1;
	return { year: isoYear, week };
}

/** Number of ISO 8601 weeks in a year (52, or 53 when Jan 1 / Dec 31 is a Thursday). */
export function getISOWeeksInYear(year: number): number {
	const jan1 = new Date(year, 0, 1).getDay();
	const dec31 = new Date(year, 11, 31).getDay();
	return jan1 === 4 || dec31 === 4 ? 53 : 52;
}

/** Monday–Sunday range of a given ISO 8601 week. */
export function isoWeekRange(year: number, week: number): { start: string; end: string } {
	const jan4 = new Date(year, 0, 4);
	const offsetFromMonday = (jan4.getDay() + 6) % 7;
	const start = new Date(jan4);
	start.setDate(jan4.getDate() - offsetFromMonday + (week - 1) * 7);
	const end = new Date(start);
	end.setDate(start.getDate() + 6);
	return { start: formatDate(start), end: formatDate(end) };
}

/** Special date key of a date: ISO week "2026-W36". */
export function formatISOWeekKey(date: Date | string): string {
	const d = typeof date === 'string' ? parseDate(date) : date;
	const { year, week } = getISOWeek(d);
	return `${year}-W${String(week).padStart(2, '0')}`;
}

/** Special date key of a date: month "2026-01". */
export function formatMonthKey(date: Date | string): string {
	const d = typeof date === 'string' ? parseDate(date) : date;
	return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`;
}

/** Parse a year key like "2026". */
export function parseYearKey(key: string): { year: number } | null {
	const m = /^(\d{4})$/.exec(key);
	return m ? { year: Number(m[1]) } : null;
}

/** Special date key of a date: year "2026". */
export function formatYearKey(date: Date | string): string {
	const d = typeof date === 'string' ? parseDate(date) : date;
	return String(d.getFullYear());
}

/** Calendar periods addressed by a special date key. */
export type CalendarPeriod = 'week' | 'month' | 'year';

/**
 * Date range for a special date key: "2026-W36" → Monday–Sunday of ISO week 36,
 * "2026-01" → the whole month, "2026" → the whole year. Returns null for
 * malformed keys or mismatched period.
 */
export function periodKeyRange(period: CalendarPeriod, key: string): { start: string; end: string } | null {
	if (period === 'week') {
		const wk = parseISOWeekKey(key);
		return wk ? isoWeekRange(wk.year, wk.week) : null;
	}
	if (period === 'month') {
		const mk = parseMonthKey(key);
		return mk ? getMonthRange(mk.year, mk.month) : null;
	}
	if (period === 'year') {
		const yk = parseYearKey(key);
		return yk ? getYearRange(yk.year) : null;
	}
	return null;
}

/**
 * Chinese label for a special date key: "2026-W36" → "2026年第36周",
 * "2026-01" → "2026年1月", "2026" → "2026年"; anything else is returned unchanged.
 */
export function formatSpecialDateLabel(key: string): string {
	const isoWeek = parseISOWeekKey(key);
	if (isoWeek) return `${isoWeek.year}年第${isoWeek.week}周`;
	const month = parseMonthKey(key);
	if (month) return `${month.year}年${month.month}月`;
	const year = parseYearKey(key);
	if (year) return `${year.year}年`;
	return key;
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

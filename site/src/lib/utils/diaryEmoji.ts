export const MAX_DIARY_EMOJI_OPTION_LENGTH = 2;
export const MAX_DIARY_EMOJI_OPTION_COUNT = 12;

export interface MoodLevel {
	value: number;
	emoji: string;
	label: string;
}

export const MOOD_SCALE: MoodLevel[] = [
	{ value: 1, emoji: '😞', label: '非常不愉快' },
	{ value: 2, emoji: '😔', label: '不愉快' },
	{ value: 3, emoji: '😐', label: '一般' },
	{ value: 4, emoji: '😊', label: '愉快' },
	{ value: 5, emoji: '🤩', label: '非常愉快' }
];

export function moodToEmoji(mood: number): string {
	const level = MOOD_SCALE.find(l => l.value === mood);
	return level?.emoji ?? '';
}

export function moodToLabel(mood: number): string {
	const level = MOOD_SCALE.find(l => l.value === mood);
	return level?.label ?? '';
}

export const DEFAULT_WEATHER_OPTIONS = [
	'☀️',
	'⛅',
	'☁️',
	'🌧️',
	'⛈️',
	'🌫️',
	'❄️',
	'🌬️'
];

export function countDisplayChars(value: string): number {
	return Array.from(value).length;
}

function sanitizeOptions(input: unknown, defaults: string[]): string[] {
	if (!Array.isArray(input)) {
		return [...defaults];
	}

	const seen = new Set<string>();
	const cleaned: string[] = [];

	for (const raw of input) {
		if (typeof raw !== 'string') continue;
		const value = raw.trim();
		if (!value) continue;
		if (countDisplayChars(value) > MAX_DIARY_EMOJI_OPTION_LENGTH) continue;
		if (seen.has(value)) continue;
		if (cleaned.length >= MAX_DIARY_EMOJI_OPTION_COUNT) break;
		seen.add(value);
		cleaned.push(value);
	}

	return cleaned.length > 0 ? cleaned : [...defaults];
}

export function sanitizeWeatherOptions(input: unknown): string[] {
	return sanitizeOptions(input, DEFAULT_WEATHER_OPTIONS);
}

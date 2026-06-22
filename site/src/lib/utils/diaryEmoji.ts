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
	{ value: 3, emoji: '😐', label: '不悲不喜' },
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

export interface MoodStateCategory {
	moodRange: [number, number];
	states: string[];
}

export const MOOD_STATES: MoodStateCategory[] = [
	{
		moodRange: [1, 2],
		states: ['愤怒', '厌恶', '有压力', '烦躁', '伤心', '焦虑', '尴尬', '忧虑', '孤独', '害怕', '沮丧', '内疚', '挫败', '不堪重负', '恼火', '失望', '羞愧', '嫉妒', '无望', '精疲力尽']
	},
	{
		moodRange: [3, 3],
		states: ['满足', '平静', '平和']
	},
	{
		moodRange: [4, 5],
		states: ['惊奇', '欢乐', '愉悦', '平静', '兴奋', '勇敢', '满意', '平和', '自豪', '如释重负', '热忱', '自信', '感恩', '开心', '满怀希望', '满足']
	}
];

export function getMoodStatesForLevel(mood: number): string[] {
	for (const cat of MOOD_STATES) {
		if (mood >= cat.moodRange[0] && mood <= cat.moodRange[1]) {
			return cat.states;
		}
	}
	return [];
}

export function isMoodStateActive(state: string, selectedStates: string[]): boolean {
	return selectedStates.includes(state);
}

export const SCENARIO_OPTIONS = [
	'健康', '心灵', '社群', '家务', '时事', '健身', '家人', '工作',
	'金钱', '照顾', '朋友', '教育', '爱好', '约会', '天气', '身份',
	'伴侣', '旅行'
];

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

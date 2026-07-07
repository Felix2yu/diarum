export interface WMOWeatherCode {
	code: number;
	label: string;
	icon: string;
}

export const WMO_CODES: Record<number, WMOWeatherCode> = {
	0: { code: 0, label: '晴', icon: '☀️' },
	1: { code: 1, label: '少云', icon: '🌤️' },
	2: { code: 2, label: '多云', icon: '⛅' },
	3: { code: 3, label: '阴天', icon: '☁️' },
	45: { code: 45, label: '雾', icon: '🌫️' },
	48: { code: 48, label: '冻雾', icon: '🌫️' },
	51: { code: 51, label: '毛毛雨', icon: '🌦️' },
	53: { code: 53, label: '毛毛雨', icon: '🌦️' },
	55: { code: 55, label: '毛毛雨', icon: '🌦️' },
	56: { code: 56, label: '冻毛毛雨', icon: '🌧️' },
	57: { code: 57, label: '冻毛毛雨', icon: '🌧️' },
	61: { code: 61, label: '小雨', icon: '🌧️' },
	63: { code: 63, label: '中雨', icon: '🌧️' },
	65: { code: 65, label: '大雨', icon: '🌧️' },
	66: { code: 66, label: '冻雨', icon: '🌧️' },
	67: { code: 67, label: '冻雨', icon: '🌧️' },
	71: { code: 71, label: '小雪', icon: '❄️' },
	73: { code: 73, label: '中雪', icon: '❄️' },
	75: { code: 75, label: '大雪', icon: '❄️' },
	77: { code: 77, label: '雪粒', icon: '❄️' },
	80: { code: 80, label: '小阵雨', icon: '🌦️' },
	81: { code: 81, label: '阵雨', icon: '🌦️' },
	82: { code: 82, label: '强阵雨', icon: '⛈️' },
	85: { code: 85, label: '小阵雪', icon: '🌨️' },
	86: { code: 86, label: '阵雪', icon: '🌨️' },
	95: { code: 95, label: '雷雨', icon: '⛈️' },
	96: { code: 96, label: '雷雨冰雹', icon: '⛈️' },
	99: { code: 99, label: '雷雨冰雹', icon: '⛈️' }
};

export function getWMOInfo(code: number): WMOWeatherCode {
	return WMO_CODES[code] ?? { code, label: '未知', icon: '❓' };
}

export function isWMOCode(value: string): boolean {
	const num = Number(value);
	return !isNaN(num) && num in WMO_CODES;
}

export function formatWeatherDisplay(code: number, tempMin?: number, tempMax?: number): string {
	const info = getWMOInfo(code);
	let display = `${info.icon} ${info.label}`;
	if (tempMin !== undefined && tempMax !== undefined) {
		display += ` ${Math.round(tempMin)}°~${Math.round(tempMax)}°`;
	}
	return display;
}

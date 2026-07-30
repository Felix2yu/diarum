export interface WeatherCode {
	code: number;
	label: string;
	icon: string;
}

export const WEATHER_CODES: Record<number, WeatherCode> = {
	0: { code: 0, label: '晴', icon: '☀️' },
	1: { code: 1, label: '多云', icon: '⛅' },
	2: { code: 2, label: '阴', icon: '☁️' },
	3: { code: 3, label: '雾/霾', icon: '🌫️' },
	4: { code: 4, label: '雨', icon: '🌧️' },
	5: { code: 5, label: '雪', icon: '❄️' },
	6: { code: 6, label: '雨夹雪', icon: '🌨️' },
	7: { code: 7, label: '雷暴', icon: '⛈️' },
	8: { code: 8, label: '大风', icon: '💨' },
	9: { code: 9, label: '沙尘', icon: '🌪️' },
};

export function wmoToSimple(wmo: number): number {
	if (wmo === 0) return 0;
	if (wmo === 1 || wmo === 2) return 1;
	if (wmo === 3) return 2;
	if ((wmo >= 4 && wmo <= 7) || (wmo >= 10 && wmo <= 12) || (wmo >= 40 && wmo <= 49) || wmo === 27 || wmo === 28) return 3;
	if (wmo === 8 || wmo === 9 || (wmo >= 30 && wmo <= 35) || wmo === 19) return 9;
	if (wmo === 18) return 8;
	if ((wmo >= 13 && wmo <= 17) || wmo === 29 || (wmo >= 90 && wmo <= 99)) return 7;
	if (wmo === 68 || wmo === 69 || wmo === 83 || wmo === 84) return 6;
	if ((wmo >= 50 && wmo <= 69) || (wmo >= 80 && wmo <= 82)) return 4;
	if ((wmo >= 70 && wmo <= 79) || (wmo >= 85 && wmo <= 86)) {
		if (wmo === 76) return 0;
		return 5;
	}
	if (wmo >= 20 && wmo <= 26) {
		if (wmo === 20 || wmo === 21 || wmo === 24 || wmo === 26) return 4;
		if (wmo === 22 || wmo === 25) return 5;
		if (wmo === 23) return 6;
	}
	if (wmo >= 36 && wmo <= 39) return 5;
	if (wmo >= 87 && wmo <= 89) return 4;
	return 0;
}

export function getWeatherInfo(code: number): WeatherCode {
	const simple = wmoToSimple(code);
	return WEATHER_CODES[simple] ?? { code, label: '未知', icon: '❓' };
}

export function isWMOCode(value: string): boolean {
	const num = Number(value);
	return !isNaN(num) && num >= 0 && num <= 99;
}

export function formatWeatherDisplay(code: number, tempMin?: number, tempMax?: number): string {
	const info = getWeatherInfo(code);
	let display = `${info.icon} ${info.label}`;
	if (tempMin !== undefined && tempMax !== undefined) {
		display += ` ${Math.round(tempMin)}°~${Math.round(tempMax)}°`;
	}
	return display;
}

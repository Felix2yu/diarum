export interface WMOWeatherCode {
	code: number;
	label: string;
	icon: string;
}

// WMO Code Table 4677 (Present Weather)
// Based on internal/weather/weather.go
export const WMO_CODES: Record<number, WMOWeatherCode> = {
	// 00-09: 无降水、无雾、无沙尘
	0: { code: 0, label: '晴', icon: '☀️' },
	1: { code: 1, label: '晴转多云', icon: '🌤️' },
	2: { code: 2, label: '多云', icon: '⛅' },
	3: { code: 3, label: '阴天', icon: '☁️' },
	4: { code: 4, label: '烟雾', icon: '🌫️' },
	5: { code: 5, label: '霾', icon: '🌫️' },
	6: { code: 6, label: '浮尘', icon: '🌫️' },
	7: { code: 7, label: '扬沙', icon: '🌫️' },
	8: { code: 8, label: '沙尘暴', icon: '🌪️' },
	9: { code: 9, label: '强沙尘暴', icon: '🌪️' },

	// 10-19: 特殊天气现象
	10: { code: 10, label: '薄雾', icon: '🌫️' },
	11: { code: 11, label: '浅雾', icon: '🌫️' },
	12: { code: 12, label: '雾凇', icon: '🌫️' },
	13: { code: 13, label: '远处闪电', icon: '⚡' },
	14: { code: 14, label: '远处降水', icon: '🌦️' },
	15: { code: 15, label: '远处雷暴', icon: '⛈️' },
	16: { code: 16, label: '近处雷暴', icon: '⛈️' },
	17: { code: 17, label: '雷暴', icon: '⛈️' },
	18: { code: 18, label: '飑', icon: '🌬️' },
	19: { code: 19, label: '漏斗云', icon: '🌪️' },

	// 20-29: 过去一小时曾出现但已停止
	20: { code: 20, label: '毛毛雨已停', icon: '🌦️' },
	21: { code: 21, label: '小雨已停', icon: '🌧️' },
	22: { code: 22, label: '雪已停', icon: '❄️' },
	23: { code: 23, label: '雨夹雪已停', icon: '🌨️' },
	24: { code: 24, label: '阵雨已停', icon: '🌦️' },
	25: { code: 25, label: '阵雪已停', icon: '🌨️' },
	26: { code: 26, label: '冰雹已停', icon: '🧊' },
	27: { code: 27, label: '雾已停', icon: '🌫️' },
	28: { code: 28, label: '雾凇已停', icon: '🌫️' },
	29: { code: 29, label: '雷暴已停', icon: '⛈️' },

	// 30-39: 沙尘暴与吹雪
	30: { code: 30, label: '弱沙尘暴', icon: '🌪️' },
	31: { code: 31, label: '沙尘暴', icon: '🌪️' },
	32: { code: 32, label: '强沙尘暴', icon: '🌪️' },
	33: { code: 33, label: '特强沙尘暴', icon: '🌪️' },
	34: { code: 34, label: '减弱沙尘暴', icon: '🌪️' },
	35: { code: 35, label: '增强沙尘暴', icon: '🌪️' },
	36: { code: 36, label: '低吹雪', icon: '🌨️' },
	37: { code: 37, label: '强低吹雪', icon: '🌨️' },
	38: { code: 38, label: '高吹雪', icon: '❄️' },
	39: { code: 39, label: '强高吹雪', icon: '❄️' },

	// 40-49: 雾与冰雾
	40: { code: 40, label: '近处有雾', icon: '🌫️' },
	41: { code: 41, label: '雾', icon: '🌫️' },
	42: { code: 42, label: '雾变薄', icon: '🌫️' },
	43: { code: 43, label: '雾无变化', icon: '🌫️' },
	44: { code: 44, label: '雾变厚', icon: '🌫️' },
	45: { code: 45, label: '雾', icon: '🌫️' },
	46: { code: 46, label: '冻雾', icon: '🌫️' },
	47: { code: 47, label: '浓冻雾', icon: '🌫️' },
	48: { code: 48, label: '雾凇', icon: '🌫️' },
	49: { code: 49, label: '浓雾凇', icon: '🌫️' },

	// 50-59: 毛毛雨
	50: { code: 50, label: '间歇毛毛雨', icon: '🌦️' },
	51: { code: 51, label: '连续毛毛雨', icon: '🌦️' },
	52: { code: 52, label: '中毛毛雨', icon: '🌦️' },
	53: { code: 53, label: '中连续毛毛雨', icon: '🌦️' },
	54: { code: 54, label: '重毛毛雨', icon: '🌧️' },
	55: { code: 55, label: '重连续毛毛雨', icon: '🌧️' },
	56: { code: 56, label: '冻毛毛雨', icon: '🌧️' },
	57: { code: 57, label: '重冻毛毛雨', icon: '🌧️' },
	58: { code: 58, label: '轻毛毛雨夹雨', icon: '🌦️' },
	59: { code: 59, label: '重毛毛雨夹雨', icon: '🌧️' },

	// 60-69: 雨
	60: { code: 60, label: '间歇小雨', icon: '🌧️' },
	61: { code: 61, label: '连续小雨', icon: '🌧️' },
	62: { code: 62, label: '中雨', icon: '🌧️' },
	63: { code: 63, label: '中连续雨', icon: '🌧️' },
	64: { code: 64, label: '大雨', icon: '🌧️' },
	65: { code: 65, label: '重连续雨', icon: '🌧️' },
	66: { code: 66, label: '冻雨', icon: '🌧️' },
	67: { code: 67, label: '重冻雨', icon: '🌧️' },
	68: { code: 68, label: '雨夹雪', icon: '🌨️' },
	69: { code: 69, label: '重雨夹雪', icon: '🌨️' },

	// 70-79: 固态降水（非阵性）
	70: { code: 70, label: '间歇小雪', icon: '❄️' },
	71: { code: 71, label: '连续小雪', icon: '❄️' },
	72: { code: 72, label: '中雪', icon: '❄️' },
	73: { code: 73, label: '中连续雪', icon: '❄️' },
	74: { code: 74, label: '大雪', icon: '❄️' },
	75: { code: 75, label: '重连续雪', icon: '❄️' },
	76: { code: 76, label: '钻石尘', icon: '✨' },
	77: { code: 77, label: '雪粒', icon: '❄️' },
	78: { code: 78, label: '星形雪晶', icon: '❄️' },
	79: { code: 79, label: '冰粒', icon: '🧊' },

	// 80-89: 阵性降水
	80: { code: 80, label: '小阵雨', icon: '🌦️' },
	81: { code: 81, label: '阵雨', icon: '🌦️' },
	82: { code: 82, label: '强阵雨', icon: '⛈️' },
	83: { code: 83, label: '小阵雨夹雪', icon: '🌨️' },
	84: { code: 84, label: '大阵雨夹雪', icon: '🌨️' },
	85: { code: 85, label: '小阵雪', icon: '🌨️' },
	86: { code: 86, label: '阵雪', icon: '🌨️' },
	87: { code: 87, label: '小阵冰雹', icon: '🧊' },
	88: { code: 88, label: '大阵冰雹', icon: '🧊' },
	89: { code: 89, label: '小阵雨夹冰雹', icon: '🧊' },

	// 90-99: 雷暴
	90: { code: 90, label: '大阵雨夹冰雹', icon: '🧊' },
	91: { code: 91, label: '小雷暴有雨', icon: '⛈️' },
	92: { code: 92, label: '大雷暴有雨', icon: '⛈️' },
	93: { code: 93, label: '小雷暴有冰雹', icon: '⛈️' },
	94: { code: 94, label: '大雷暴有冰雹', icon: '⛈️' },
	95: { code: 95, label: '雷暴', icon: '⛈️' },
	96: { code: 96, label: '雷暴有冰雹', icon: '⛈️' },
	97: { code: 97, label: '重雷暴有冰雹', icon: '⛈️' },
	98: { code: 98, label: '雷暴有沙尘', icon: '⛈️' },
	99: { code: 99, label: '重雷暴有冰雹', icon: '⛈️' }
};

export function getWMOInfo(code: number): WMOWeatherCode {
	return WMO_CODES[code] ?? { code, label: '未知', icon: '❓' };
}

export function isWMOCode(value: string): boolean {
	const num = Number(value);
	return !isNaN(num) && num >= 0 && num <= 99;
}

export function formatWeatherDisplay(code: number, tempMin?: number, tempMax?: number): string {
	const info = getWMOInfo(code);
	let display = `${info.icon} ${info.label}`;
	if (tempMin !== undefined && tempMax !== undefined) {
		display += ` ${Math.round(tempMin)}°~${Math.round(tempMax)}°`;
	}
	return display;
}

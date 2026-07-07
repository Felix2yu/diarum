export interface CityInfo {
	name: string;
	lat: number;
	lon: number;
	province: string;
}

export const CHINESE_CITIES: CityInfo[] = [
	{ name: '北京', lat: 39.9042, lon: 116.4074, province: '北京' },
	{ name: '上海', lat: 31.2304, lon: 121.4737, province: '上海' },
	{ name: '广州', lat: 23.1291, lon: 113.2644, province: '广东' },
	{ name: '深圳', lat: 22.5431, lon: 114.0579, province: '广东' },
	{ name: '成都', lat: 30.5728, lon: 104.0668, province: '四川' },
	{ name: '重庆', lat: 29.4316, lon: 106.9123, province: '重庆' },
	{ name: '杭州', lat: 30.2741, lon: 120.1551, province: '浙江' },
	{ name: '武汉', lat: 30.5928, lon: 114.3055, province: '湖北' },
	{ name: '西安', lat: 34.3416, lon: 108.9398, province: '陕西' },
	{ name: '苏州', lat: 31.2990, lon: 120.5853, province: '江苏' },
	{ name: '南京', lat: 32.0603, lon: 118.7969, province: '江苏' },
	{ name: '天津', lat: 39.3434, lon: 117.3616, province: '天津' },
	{ name: '长沙', lat: 28.2282, lon: 112.9388, province: '湖南' },
	{ name: '沈阳', lat: 41.8057, lon: 123.4315, province: '辽宁' },
	{ name: '哈尔滨', lat: 45.8038, lon: 126.5350, province: '黑龙江' },
	{ name: '昆明', lat: 25.0389, lon: 102.7183, province: '云南' },
	{ name: '大连', lat: 38.9140, lon: 121.6147, province: '辽宁' },
	{ name: '厦门', lat: 24.4798, lon: 118.0894, province: '福建' },
	{ name: '青岛', lat: 36.0671, lon: 120.3826, province: '山东' },
	{ name: '郑州', lat: 34.7466, lon: 113.6253, province: '河南' },
	{ name: '济南', lat: 36.6512, lon: 116.9972, province: '山东' },
	{ name: '福州', lat: 26.0745, lon: 119.2965, province: '福建' },
	{ name: '东莞', lat: 23.0208, lon: 113.7518, province: '广东' },
	{ name: '无锡', lat: 31.4912, lon: 120.3119, province: '江苏' },
	{ name: '合肥', lat: 31.8206, lon: 117.2272, province: '安徽' },
	{ name: '佛山', lat: 23.0218, lon: 113.1218, province: '广东' },
	{ name: '长春', lat: 43.8171, lon: 125.3235, province: '吉林' },
	{ name: '温州', lat: 27.9939, lon: 120.6993, province: '浙江' },
	{ name: '石家庄', lat: 38.0428, lon: 114.5149, province: '河北' },
	{ name: '南宁', lat: 22.8170, lon: 108.3665, province: '广西' },
	{ name: '常州', lat: 31.8106, lon: 119.9741, province: '江苏' },
	{ name: '泉州', lat: 24.8741, lon: 118.6759, province: '福建' },
	{ name: '南昌', lat: 28.6820, lon: 115.8579, province: '江西' },
	{ name: '贵阳', lat: 26.6470, lon: 106.6302, province: '贵州' },
	{ name: '太原', lat: 37.8706, lon: 112.5489, province: '山西' },
	{ name: '烟台', lat: 37.4638, lon: 121.4479, province: '山东' },
	{ name: '嘉兴', lat: 30.7539, lon: 120.7585, province: '浙江' },
	{ name: '南通', lat: 32.0603, lon: 120.8649, province: '江苏' },
	{ name: '金华', lat: 29.0789, lon: 119.6495, province: '浙江' },
	{ name: '惠州', lat: 23.1116, lon: 114.4160, province: '广东' },
	{ name: '徐州', lat: 34.2610, lon: 117.1947, province: '江苏' },
	{ name: '石家庄', lat: 38.0428, lon: 114.5149, province: '河北' },
	{ name: '中山', lat: 22.5170, lon: 113.3926, province: '广东' },
	{ name: '台州', lat: 28.6563, lon: 121.4208, province: '浙江' },
	{ name: '兰州', lat: 36.0611, lon: 103.8343, province: '甘肃' },
	{ name: '绍兴', lat: 30.0300, lon: 120.5800, province: '浙江' },
	{ name: '乌鲁木齐', lat: 43.8256, lon: 87.6168, province: '新疆' },
	{ name: '扬州', lat: 32.3942, lon: 119.4130, province: '江苏' },
	{ name: '廊坊', lat: 39.5168, lon: 116.7044, province: '河北' },
	{ name: '洛阳', lat: 34.6197, lon: 112.4540, province: '河南' },
	{ name: '汕头', lat: 23.3541, lon: 116.6819, province: '广东' },
	{ name: '呼和浩特', lat: 40.8424, lon: 111.7490, province: '内蒙古' },
	{ name: '海口', lat: 20.0174, lon: 110.3492, province: '海南' },
	{ name: '银川', lat: 38.4872, lon: 106.2309, province: '宁夏' },
	{ name: '西宁', lat: 36.6171, lon: 101.7782, province: '青海' },
	{ name: '拉萨', lat: 29.6500, lon: 91.1000, province: '西藏' },
	{ name: '澳门', lat: 22.1987, lon: 113.5439, province: '澳门' },
	{ name: '香港', lat: 22.3193, lon: 114.1694, province: '香港' },
	{ name: '台北', lat: 25.0330, lon: 121.5654, province: '台湾' },
	{ name: '高雄', lat: 22.6273, lon: 120.3014, province: '台湾' }
];

export function searchCities(query: string): CityInfo[] {
	if (!query.trim()) return [];
	const q = query.toLowerCase().trim();
	return CHINESE_CITIES.filter(
		c =>
			c.name.toLowerCase().includes(q) ||
			c.province.toLowerCase().includes(q)
	).slice(0, 10);
}

export function getCityByName(name: string): CityInfo | undefined {
	return CHINESE_CITIES.find(c => c.name === name);
}

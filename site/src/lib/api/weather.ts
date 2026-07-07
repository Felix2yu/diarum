import { pb } from './client';

export interface WeatherResult {
	city: string;
	wmo_code: number;
	temp_min: number;
	temp_max: number;
	date: string;
}

export async function fetchWeather(city: string): Promise<WeatherResult> {
	const response = await fetch(`/api/v1/weather?city=${encodeURIComponent(city)}`, {
		headers: {
			Authorization: `Bearer ${pb.authStore.token}`
		}
	});

	if (!response.ok) {
		const data = await response.json().catch(() => ({}));
		throw new Error(data.error || 'Failed to fetch weather');
	}

	return response.json();
}

export async function fetchWeatherByCoords(
	city: string,
	lat: number,
	lon: number
): Promise<WeatherResult> {
	const response = await fetch(
		`/api/v1/weather/coords?city=${encodeURIComponent(city)}&lat=${lat}&lon=${lon}`,
		{
			headers: {
				Authorization: `Bearer ${pb.authStore.token}`
			}
		}
	);

	if (!response.ok) {
		const data = await response.json().catch(() => ({}));
		throw new Error(data.error || 'Failed to fetch weather');
	}

	return response.json();
}

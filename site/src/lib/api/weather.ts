import { pb } from './client';
import { getCachedWeather, setCachedWeather } from '$lib/stores/weatherCache';

export interface WeatherResult {
	city: string;
	wmo_code: number;
	temp_min: number;
	temp_max: number;
	date: string;
}

export async function fetchWeather(city: string, useCache = true): Promise<WeatherResult> {
	// Check cache first
	if (useCache) {
		const cached = getCachedWeather(city);
		if (cached) {
			return cached;
		}
	}

	const response = await fetch(`/api/v1/weather?city=${encodeURIComponent(city)}`, {
		headers: {
			Authorization: `Bearer ${pb.authStore.token}`
		}
	});

	if (!response.ok) {
		const data = await response.json().catch(() => ({}));
		throw new Error(data.error || 'Failed to fetch weather');
	}

	const result = await response.json();

	// Cache the result
	setCachedWeather(city, result);

	return result;
}

export async function fetchWeatherByCoords(
	city: string,
	lat: number,
	lon: number,
	useCache = true
): Promise<WeatherResult> {
	// Check cache first
	if (useCache) {
		const cached = getCachedWeather(city);
		if (cached) {
			return cached;
		}
	}

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

	const result = await response.json();

	// Cache the result
	setCachedWeather(city, result);

	return result;
}

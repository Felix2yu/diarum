import { pb } from './client';
import { getCachedWeather, setCachedWeather } from '$lib/stores/weatherCache';

export interface WeatherResult {
	city: string;
	wmo_code: number;
	temp_min: number;
	temp_max: number;
	date: string;
}

export async function fetchWeather(city: string, date?: string, useCache = true): Promise<WeatherResult> {
	// Check cache first (only for today's weather without specific date)
	if (useCache && !date) {
		const cached = getCachedWeather(city);
		if (cached) {
			return cached;
		}
	}

	const params = new URLSearchParams({ city });
	if (date) {
		params.set('date', date);
	}

	const response = await fetch(`/api/v1/weather?${params.toString()}`, {
		headers: {
			Authorization: `Bearer ${pb.authStore.token}`
		}
	});

	if (!response.ok) {
		const data = await response.json().catch(() => ({}));
		throw new Error(data.error || 'Failed to fetch weather');
	}

	const result = await response.json();

	// Cache the result (only for today)
	if (!date) {
		setCachedWeather(city, result);
	}

	return result;
}

export async function fetchWeatherByCoords(
	city: string,
	lat: number,
	lon: number,
	date?: string,
	useCache = true
): Promise<WeatherResult> {
	// Check cache first (only for today's weather without specific date)
	if (useCache && !date) {
		const cached = getCachedWeather(city);
		if (cached) {
			return cached;
		}
	}

	const params = new URLSearchParams({ city, lat: String(lat), lon: String(lon) });
	if (date) {
		params.set('date', date);
	}

	const response = await fetch(
		`/api/v1/weather/coords?${params.toString()}`,
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

	// Cache the result (only for today)
	if (!date) {
		setCachedWeather(city, result);
	}

	return result;
}

import { browser } from '$app/environment';
import type { WeatherResult } from '$lib/api/weather';

const CACHE_KEY = 'diarum_weather_cache';
const CACHE_DURATION = 30 * 60 * 1000; // 30 minutes

interface CacheEntry {
	city: string;
	data: WeatherResult;
	timestamp: number;
}

interface WeatherCache {
	[city: string]: CacheEntry;
}

let cache: WeatherCache = {};

function loadCache(): void {
	if (!browser) return;
	try {
		const stored = localStorage.getItem(CACHE_KEY);
		if (stored) {
			cache = JSON.parse(stored);
			// Clean expired entries
			const now = Date.now();
			for (const key in cache) {
				if (now - cache[key].timestamp > CACHE_DURATION) {
					delete cache[key];
				}
			}
		}
	} catch {
		cache = {};
	}
}

function saveCache(): void {
	if (!browser) return;
	try {
		localStorage.setItem(CACHE_KEY, JSON.stringify(cache));
	} catch {
		// Ignore storage errors
	}
}

export function getCachedWeather(city: string): WeatherResult | null {
	if (!browser) return null;

	if (Object.keys(cache).length === 0) {
		loadCache();
	}

	const entry = cache[city];
	if (!entry) return null;

	// Check if expired
	if (Date.now() - entry.timestamp > CACHE_DURATION) {
		delete cache[city];
		saveCache();
		return null;
	}

	return entry.data;
}

export function setCachedWeather(city: string, data: WeatherResult): void {
	if (!browser) return;

	if (Object.keys(cache).length === 0) {
		loadCache();
	}

	cache[city] = {
		city,
		data,
		timestamp: Date.now()
	};

	saveCache();
}

export function clearWeatherCache(): void {
	cache = {};
	if (browser) {
		localStorage.removeItem(CACHE_KEY);
	}
}

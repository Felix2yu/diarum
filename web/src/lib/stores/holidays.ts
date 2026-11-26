import { browser } from '$app/env';
import { get, writable } from 'svelte/store';
import { isAuthenticated } from '../api/client.js';
import {
  fetchCNHolidays,
  getCNHolidayFeatureStatus,
  type CNHolidayDay,
  type CNHolidayYearResponse
} from '../api/holidays.js';
import { getPreviousDay, parseDate } from '../utils/date.js';

const CACHE_STORAGE_PREFIX = 'diarum.holidays.cn.v2.';
const CACHE_TTL_MS = 10 * 60 * 1000; // 短 TTL，让手工覆盖能较快生效

interface CachedYearEntry {
  savedAt: number;
  data: CNHolidayYearResponse;
}

export interface CNDayStatus {
  kind: 'holiday' | 'workday_override' | 'weekend' | 'workday';
  badge?: '休' | '班';
  name?: string;
  shortName?: string;
  isOffDay: boolean;
  isFirstHolidayDay: boolean;
  custom?: boolean;
}

/** 实例是否启用了中国法定节假日功能。 */
export const cnHolidaysGlobalEnabled = writable<boolean>(false);

/** 是否显示节假日/调休标记（跟随实例开关）。 */
export const cnHolidaysEnabled = cnHolidaysGlobalEnabled;

/** 日期 -> 节假日/调休信息。 */
export const holidayDaysByDate = writable<Record<string, CNHolidayDay>>({});

const loadedYears = new Set<number>();
const inFlightYears = new Set<number>();
let initPromise: Promise<void> | null = null;
let initialized = false;

function readYearFromStorage(year: number): CachedYearEntry | null {
  if (!browser) return null;
  try {
    const raw = localStorage.getItem(`${CACHE_STORAGE_PREFIX}${year}`);
    if (!raw) return null;
    const parsed = JSON.parse(raw) as CachedYearEntry;
    if (
      !parsed ||
      typeof parsed.savedAt !== 'number' ||
      !parsed.data ||
      parsed.data.year !== year
    ) {
      return null;
    }
    return parsed;
  } catch {
    return null;
  }
}

function writeYearToStorage(year: number, data: CNHolidayYearResponse): void {
  if (!browser) return;
  try {
    const entry: CachedYearEntry = { savedAt: Date.now(), data };
    localStorage.setItem(`${CACHE_STORAGE_PREFIX}${year}`, JSON.stringify(entry));
  } catch {
    // 忽略配额错误
  }
}

function mergeYearData(year: number, data: CNHolidayYearResponse): void {
  const prefix = `${year}-`;
  holidayDaysByDate.update((prev) => {
    const next: Record<string, CNHolidayDay> = {};
    for (const [k, v] of Object.entries(prev)) {
      if (!k.startsWith(prefix)) {
        next[k] = v;
      }
    }
    for (const day of data.days ?? []) {
      next[day.date] = day;
    }
    return next;
  });
}

export async function initHolidaySettings(force = false): Promise<void> {
  if (!browser || !get(isAuthenticated)) return;
  if (initialized && !force) return;
  if (initPromise && !force) return initPromise;

  initPromise = (async () => {
    try {
      const status = await getCNHolidayFeatureStatus();
      cnHolidaysGlobalEnabled.set(!!status.enabled);
      initialized = true;
    } catch {
      // 失败时保持关闭，后续可重试
    } finally {
      initPromise = null;
    }
  })();

  return initPromise;
}

export function setCNHolidaysGlobalEnabledLocal(enabled: boolean): void {
  cnHolidaysGlobalEnabled.set(enabled);
  initialized = true;
  if (!enabled) {
    holidayDaysByDate.set({});
    loadedYears.clear();
  }
}

export async function ensureHolidayYears(years: number[]): Promise<void> {
  if (!browser || !get(isAuthenticated) || !get(cnHolidaysEnabled)) return;

  const toFetch: number[] = [];

  for (const y of years) {
    if (!Number.isInteger(y) || y < 2000 || y > 2100) continue;
    if (loadedYears.has(y) || inFlightYears.has(y)) continue;

    const cached = readYearFromStorage(y);
    if (cached) {
      mergeYearData(y, cached.data);
      if (Date.now() - cached.savedAt < CACHE_TTL_MS) {
        loadedYears.add(y);
        continue;
      }
    }
    toFetch.push(y);
  }

  const uniqueFetch = Array.from(new Set(toFetch));
  if (uniqueFetch.length === 0) return;

  for (const y of uniqueFetch) {
    inFlightYears.add(y);
  }
  try {
    const res = await fetchCNHolidays(uniqueFetch);
    if (!res.enabled) {
      setCNHolidaysGlobalEnabledLocal(false);
      return;
    }
    for (const y of uniqueFetch) {
      const data = res.years[y];
      if (data) {
        mergeYearData(y, data);
        writeYearToStorage(y, data);
        loadedYears.add(y);
      }
    }
  } catch (error) {
    console.error('加载节假日数据失败:', error);
  } finally {
    for (const y of uniqueFetch) {
      inFlightYears.delete(y);
    }
  }
}

export function invalidateHolidayCache(): void {
  const yearsToReload = Array.from(loadedYears);
  loadedYears.clear();
  if (browser) {
    const keysToRemove: string[] = [];
    for (let i = 0; i < localStorage.length; i++) {
      const k = localStorage.key(i);
      if (k && k.startsWith(CACHE_STORAGE_PREFIX)) {
        keysToRemove.push(k);
      }
    }
    for (const k of keysToRemove) {
      localStorage.removeItem(k);
    }
  }
  if (yearsToReload.length > 0 && get(cnHolidaysEnabled)) {
    void ensureHolidayYears(yearsToReload);
  }
}

export function getHolidayShortName(name: string): string {
  const trimmed = name.trim();
  if (!trimmed) return '';
  if (trimmed.length >= 3 && trimmed.endsWith('节')) {
    return trimmed.slice(0, -1);
  }
  return trimmed.length > 3 ? trimmed.slice(0, 3) : trimmed;
}

export function getCNDayStatus(
  dateStr: string,
  daysMap: Record<string, CNHolidayDay>
): CNDayStatus {
  const special = daysMap[dateStr];
  if (special) {
    if (special.isOffDay) {
      const prev = daysMap[getPreviousDay(dateStr)];
      const isFirstHolidayDay = !(prev && prev.isOffDay && prev.name === special.name);
      return {
        kind: 'holiday',
        badge: '休',
        name: special.name,
        shortName: getHolidayShortName(special.name),
        isOffDay: true,
        isFirstHolidayDay,
        custom: special.custom
      };
    }
    return {
      kind: 'workday_override',
      badge: '班',
      name: special.name,
      shortName: getHolidayShortName(special.name),
      isOffDay: false,
      isFirstHolidayDay: false,
      custom: special.custom
    };
  }

  const dayOfWeek = parseDate(dateStr).getDay();
  if (dayOfWeek === 0 || dayOfWeek === 6) {
    return {
      kind: 'weekend',
      isOffDay: true,
      isFirstHolidayDay: false
    };
  }

  return {
    kind: 'workday',
    isOffDay: false,
    isFirstHolidayDay: false
  };
}

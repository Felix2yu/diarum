import { pb } from './client';

export interface CNHolidayDay {
  name: string;
  date: string;
  isOffDay: boolean;
  custom?: boolean;
}

export interface CNHolidayYearResponse {
  region: string;
  year: number;
  published: boolean;
  days: CNHolidayDay[];
  papers: string[];
  source: string;
  fetched_at?: string;
}

export interface CNHolidayYearSummary {
  year: number;
  published: boolean;
  day_count: number;
  source: string;
  fetched_at?: string;
  papers?: string[];
}

export interface CNHolidayFeatureStatus {
  region: string;
  enabled: boolean;
}

export interface CNHolidayOverview {
  region: string;
  enabled: boolean;
  years: CNHolidayYearSummary[];
  last_fetched_at?: string;
  overrides: CNHolidayDay[];
}

async function parseError(response: Response, fallback: string): Promise<Error> {
  const data = await response.json().catch(() => ({}));
  return new Error(data?.message || `${fallback} (${response.status})`);
}

/** 功能是否已在实例中启用。 */
export async function getCNHolidayFeatureStatus(): Promise<CNHolidayFeatureStatus> {
  const response = await fetch('/api/v1/holidays/cn/status', {
    headers: {
      Authorization: `Bearer ${pb.authStore.token}`
    }
  });
  if (!response.ok) {
    throw await parseError(response, '加载节假日功能状态失败');
  }
  return await response.json();
}

/** 批量获取合并手工覆盖后的年度节假日数据。 */
export async function fetchCNHolidays(
  years: number[]
): Promise<{ enabled: boolean; years: Record<number, CNHolidayYearResponse> }> {
  const unique = Array.from(
    new Set(years.filter((y) => Number.isInteger(y) && y >= 2000 && y <= 2100))
  );
  if (unique.length === 0) return { enabled: true, years: {} };

  const response = await fetch(`/api/v1/holidays/cn?years=${unique.join(',')}`, {
    headers: {
      Authorization: `Bearer ${pb.authStore.token}`
    }
  });
  if (!response.ok) {
    throw await parseError(response, '加载节假日数据失败');
  }
  const data = await response.json();
  const rawYears = (data?.years ?? {}) as Record<string, CNHolidayYearResponse>;
  const out: Record<number, CNHolidayYearResponse> = {};
  for (const [k, v] of Object.entries(rawYears)) {
    const year = Number(k);
    if (Number.isInteger(year) && v) {
      out[year] = v;
    }
  }
  return { enabled: !!data?.enabled, years: out };
}

/** 读取实例级节假日设置概览。 */
export async function getCNHolidayOverview(): Promise<CNHolidayOverview> {
  const response = await fetch('/api/v1/holidays/cn/settings', {
    headers: {
      Authorization: `Bearer ${pb.authStore.token}`
    }
  });
  if (!response.ok) {
    throw await parseError(response, '加载节假日设置失败');
  }
  return await response.json();
}

/** 读取某年合并后的有效日程（用于设置页预览）。 */
export async function getCNHolidayYear(year: number): Promise<CNHolidayYearResponse> {
  const response = await fetch(`/api/v1/holidays/cn/year?year=${year}`, {
    headers: {
      Authorization: `Bearer ${pb.authStore.token}`
    }
  });
  if (!response.ok) {
    throw await parseError(response, '加载年度节假日失败');
  }
  return await response.json();
}

export async function setCNHolidaysEnabled(enabled: boolean): Promise<{ enabled: boolean }> {
  const response = await fetch('/api/v1/holidays/cn/settings', {
    method: 'PUT',
    headers: {
      Authorization: `Bearer ${pb.authStore.token}`,
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({ enabled })
  });
  if (!response.ok) {
    throw await parseError(response, '保存节假日设置失败');
  }
  return await response.json();
}

export async function refreshCNHolidays(): Promise<CNHolidayOverview> {
  const response = await fetch('/api/v1/holidays/cn/refresh', {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${pb.authStore.token}`
    }
  });
  if (!response.ok) {
    throw await parseError(response, '同步官方节假日数据失败');
  }
  return await response.json();
}

export async function upsertCNHolidayOverride(payload: {
  date: string;
  name: string;
  isOffDay: boolean;
}): Promise<CNHolidayDay> {
  const response = await fetch('/api/v1/holidays/cn/overrides', {
    method: 'PUT',
    headers: {
      Authorization: `Bearer ${pb.authStore.token}`,
      'Content-Type': 'application/json'
    },
    body: JSON.stringify(payload)
  });
  if (!response.ok) {
    throw await parseError(response, '保存节假日覆盖失败');
  }
  return await response.json();
}

export async function deleteCNHolidayOverride(date: string): Promise<void> {
  const response = await fetch(`/api/v1/holidays/cn/overrides/${encodeURIComponent(date)}`, {
    method: 'DELETE',
    headers: {
      Authorization: `Bearer ${pb.authStore.token}`
    }
  });
  if (!response.ok) {
    throw await parseError(response, '删除节假日覆盖失败');
  }
}

export async function importCNHolidayJSON(file: File): Promise<{
  imported: CNHolidayYearResponse;
  overview: CNHolidayOverview;
}> {
  const formData = new FormData();
  formData.append('file', file);
  const response = await fetch('/api/v1/holidays/cn/import', {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${pb.authStore.token}`
    },
    body: formData
  });
  if (!response.ok) {
    throw await parseError(response, '导入节假日 JSON 失败');
  }
  return await response.json();
}

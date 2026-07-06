import { getApiToken } from './settings';

const API_BASE = '/api/v1';

export interface Backup {
	id: string;
	owner: string;
	filename: string;
	filepath: string;
	size: number;
	s3_key: string;
	created: string;
}

export interface BackupSettings {
	enabled: boolean;
	frequency: string;
	time: string;
	day_of_week: number;
	day_of_month: number;
	retention_days: number;
	upload_s3: boolean;
}

export interface BackupListResponse {
	backups: Backup[];
	total: number;
	page: number;
	per_page: number;
	pages: number;
}

async function authHeaders(): Promise<HeadersInit> {
	const token = await getApiToken();
	return {
		'Content-Type': 'application/json',
		Authorization: `Bearer ${token}`
	};
}

export async function listBackups(page = 1, perPage = 20): Promise<BackupListResponse> {
	const headers = await authHeaders();
	const res = await fetch(`${API_BASE}/backups?page=${page}&per_page=${perPage}`, { headers });
	if (!res.ok) throw new Error(`Failed to list backups: ${res.statusText}`);
	return res.json();
}

export async function getBackup(id: string): Promise<Backup> {
	const headers = await authHeaders();
	const res = await fetch(`${API_BASE}/backups/${id}`, { headers });
	if (!res.ok) throw new Error(`Failed to get backup: ${res.statusText}`);
	return res.json();
}

export async function downloadBackup(id: string): Promise<void> {
	const token = await getApiToken();
	const res = await fetch(`${API_BASE}/backups/${id}/download`, {
		headers: { Authorization: `Bearer ${token}` }
	});
	if (!res.ok) throw new Error(`Failed to download backup: ${res.statusText}`);
	const blob = await res.blob();
	const url = URL.createObjectURL(blob);
	const a = document.createElement('a');
	a.href = url;
	a.download = (await getBackup(id)).filename;
	a.click();
	URL.revokeObjectURL(url);
}

export async function deleteBackup(id: string): Promise<void> {
	const headers = await authHeaders();
	const res = await fetch(`${API_BASE}/backups/${id}`, { method: 'DELETE', headers });
	if (!res.ok) throw new Error(`Failed to delete backup: ${res.statusText}`);
}

export async function triggerBackup(): Promise<void> {
	const headers = await authHeaders();
	const res = await fetch(`${API_BASE}/backups/trigger`, { method: 'POST', headers });
	if (!res.ok) throw new Error(`Failed to trigger backup: ${res.statusText}`);
}

export async function getBackupSettings(): Promise<BackupSettings> {
	const headers = await authHeaders();
	const res = await fetch(`${API_BASE}/backups/settings`, { headers });
	if (!res.ok) throw new Error(`Failed to get backup settings: ${res.statusText}`);
	return res.json();
}

export async function saveBackupSettings(settings: Partial<BackupSettings>): Promise<void> {
	const headers = await authHeaders();
	const res = await fetch(`${API_BASE}/backups/settings`, {
		method: 'POST',
		headers,
		body: JSON.stringify(settings)
	});
	if (!res.ok) throw new Error(`Failed to save backup settings: ${res.statusText}`);
}

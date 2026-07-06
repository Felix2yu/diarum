import { writable, get } from 'svelte/store';
import {
	listBackups,
	getBackupSettings,
	saveBackupSettings,
	triggerBackup,
	deleteBackup,
	downloadBackup,
	type Backup,
	type BackupSettings
} from '$lib/api/backup';

export const backups = writable<Backup[]>([]);
export const backupTotal = writable(0);
export const backupPage = writable(1);
export const backupPages = writable(0);
export const backupSettings = writable<BackupSettings>({
	enabled: false,
	frequency: 'daily',
	time: '00:00',
	day_of_week: 1,
	day_of_month: 1,
	retention_days: 90,
	upload_s3: false
});
export const backupLoading = writable(false);

let loaded = false;

export async function loadBackups(page = 1) {
	backupLoading.set(true);
	try {
		const res = await listBackups(page);
		backups.set(res.backups || []);
		backupTotal.set(res.total);
		backupPage.set(res.page);
		backupPages.set(res.pages);
	} catch (e) {
		console.error('Failed to load backups:', e);
	} finally {
		backupLoading.set(false);
	}
}

export async function loadBackupSettings() {
	try {
		const settings = await getBackupSettings();
		backupSettings.set(settings);
		loaded = true;
	} catch (e) {
		console.error('Failed to load backup settings:', e);
	}
}

export async function saveBackupSettingsLocal(settings: Partial<BackupSettings>) {
	await saveBackupSettings(settings);
	backupSettings.update((s) => ({ ...s, ...settings }));
}

export async function triggerBackupNow() {
	await triggerBackup();
	await loadBackups();
}

export async function deleteBackupById(id: string) {
	await deleteBackup(id);
	const page = get(backupPage);
	await loadBackups(page);
}

export async function downloadBackupById(id: string) {
	await downloadBackup(id);
}

export function isBackupLoaded() {
	return loaded;
}

export function resetBackupStore() {
	loaded = false;
	backups.set([]);
	backupTotal.set(0);
	backupPage.set(1);
	backupPages.set(0);
}

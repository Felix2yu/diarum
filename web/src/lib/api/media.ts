import { pb, type Media, type Diary } from './client';

export interface MediaWithDiary extends Media {
  expand?: {
    diary?: Diary[];
  };
}

export async function getAllMedia(
  page: number = 1,
  perPage: number = 50
): Promise<{
  items: MediaWithDiary[];
  totalPages: number;
  totalItems: number;
}> {
  try {
    const response = await fetch(`/api/v1/media?page=${page}&perPage=${perPage}`, {
      headers: { Authorization: `Bearer ${pb.authStore.token}` }
    });
    if (!response.ok) return { items: [], totalPages: 0, totalItems: 0 };
    const result = await response.json();
    return {
      items: result.items || [],
      totalPages: result.totalPages || 0,
      totalItems: result.totalItems || 0
    };
  } catch (error) {
    console.error('Error fetching media:', error);
    return { items: [], totalPages: 0, totalItems: 0 };
  }
}

export async function deleteMediaById(id: string): Promise<boolean> {
  try {
    const response = await fetch(`/api/v1/media/${encodeURIComponent(id)}`, {
      method: 'DELETE',
      headers: { Authorization: `Bearer ${pb.authStore.token}` }
    });
    return response.ok;
  } catch (error) {
    console.error('Error deleting media:', error);
    return false;
  }
}

export async function getTrashMedia(
  page: number = 1,
  perPage: number = 50
): Promise<{
  items: MediaWithDiary[];
  totalPages: number;
  totalItems: number;
}> {
  try {
    const response = await fetch(`/api/v1/media/trash?page=${page}&perPage=${perPage}`, {
      headers: { Authorization: `Bearer ${pb.authStore.token}` }
    });
    if (!response.ok) return { items: [], totalPages: 0, totalItems: 0 };
    const result = await response.json();
    return {
      items: result.items || [],
      totalPages: result.totalPages || 0,
      totalItems: result.totalItems || 0
    };
  } catch (error) {
    console.error('Error fetching trashed media:', error);
    return { items: [], totalPages: 0, totalItems: 0 };
  }
}

export async function getUnreferencedMedia(
  page: number = 1,
  perPage: number = 50
): Promise<{
  items: MediaWithDiary[];
  totalPages: number;
  totalItems: number;
}> {
  try {
    const response = await fetch(`/api/v1/media/unreferenced?page=${page}&perPage=${perPage}`, {
      headers: { Authorization: `Bearer ${pb.authStore.token}` }
    });
    if (!response.ok) return { items: [], totalPages: 0, totalItems: 0 };
    const result = await response.json();
    return {
      items: result.items || [],
      totalPages: result.totalPages || 0,
      totalItems: result.totalItems || 0
    };
  } catch (error) {
    console.error('Error fetching unreferenced media:', error);
    return { items: [], totalPages: 0, totalItems: 0 };
  }
}

/**
 * 把全部未被日记正文引用的媒体一次性移入回收站。
 */
export async function trashAllUnreferencedMedia(): Promise<{ deleted: number } | null> {
  try {
    const response = await fetch('/api/v1/media/unreferenced', {
      method: 'DELETE',
      headers: { Authorization: `Bearer ${pb.authStore.token}` }
    });
    if (!response.ok) return null;
    return await response.json();
  } catch (error) {
    console.error('Error trashing unreferenced media:', error);
    return null;
  }
}

export async function restoreMediaById(id: string): Promise<boolean> {
  try {
    const response = await fetch(`/api/v1/media/${encodeURIComponent(id)}/restore`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${pb.authStore.token}` }
    });
    return response.ok;
  } catch (error) {
    console.error('Error restoring media:', error);
    return false;
  }
}

export async function permanentlyDeleteMediaById(id: string): Promise<boolean> {
  try {
    const response = await fetch(`/api/v1/media/${encodeURIComponent(id)}/permanent`, {
      method: 'DELETE',
      headers: { Authorization: `Bearer ${pb.authStore.token}` }
    });
    return response.ok;
  } catch (error) {
    console.error('Error permanently deleting media:', error);
    return false;
  }
}

export async function emptyMediaTrash(): Promise<{ deleted: number; failed: number } | null> {
  try {
    const response = await fetch('/api/v1/media/trash', {
      method: 'DELETE',
      headers: { Authorization: `Bearer ${pb.authStore.token}` }
    });
    if (!response.ok) return null;
    return await response.json();
  } catch (error) {
    console.error('Error emptying media trash:', error);
    return null;
  }
}

export function getMediaFileUrl(media: Media, thumb?: string): string {
  if (!media.id || !media.file) return '';
  const url = `/api/v1/files/media/${encodeURIComponent(media.id)}/${encodeURIComponent(media.file)}`;
  return thumb ? `${url}?thumb=${encodeURIComponent(thumb)}` : url;
}

/**
 * 用于 <img> 等无法携带 Authorization header 的场景。
 * 仅在展示时拼接 token，不写入日记正文。
 */
export function getMediaDisplayUrl(media: Media, thumb?: string): string {
  const url = getMediaFileUrl(media, thumb);
  if (!url) return '';
  const token = pb.authStore.token;
  if (!token) return url;
  return `${url}${url.includes('?') ? '&' : '?'}token=${encodeURIComponent(token)}`;
}

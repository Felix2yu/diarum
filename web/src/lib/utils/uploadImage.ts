import { pb } from '#lib/api/client.js';
import type { Media, UploadProgress } from '#lib/api/client.js';
import { get } from 'svelte/store';
import {
  imageUploadSettings,
  loadImageUploadSettings,
  isImageUploadLoaded
} from '#lib/stores/imageUpload.js';
import { uploadToChevereto } from '#lib/api/chevereto.js';

/**
 * Get or create diary ID for a given date
 */
export async function getOrCreateDiaryId(date: string): Promise<string | undefined> {
  try {
    // Try to find existing diary
    const response = await fetch(`/api/v1/diaries/by-date/${date}`, {
      headers: {
        Authorization: `Bearer ${pb.authStore.token}`
      }
    });

    if (response.ok) {
      const data = await response.json();
      if (data.exists && data.id) {
        return data.id;
      }
    }

    // Create new diary if not exists via business API
    const createResponse = await fetch('/api/v1/diaries/upsert', {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${pb.authStore.token}`,
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({
        date,
        content: '',
        mood: '',
        weather: ''
      })
    });

    if (!createResponse.ok) {
      return undefined;
    }

    const newDiary = await createResponse.json();

    return newDiary.id;
  } catch (error) {
    console.error('Failed to get/create diary:', error);
    return undefined;
  }
}

export interface UploadOptions {
  diaryId?: string;
  diaryDate?: string; // Date string (YYYY-MM-DD) to auto-link diary
  alt?: string;
  onProgress?: (progress: UploadProgress) => void;
  signal?: AbortSignal;
}

export interface CheveretoUploadResult {
  cheveretoUrl: string;
}

export function isCheveretoResult(
  result: Media | CheveretoUploadResult
): result is CheveretoUploadResult {
  return 'cheveretoUrl' in result;
}

/**
 * Upload an image file to Diarum or Chevereto
 * @param file - The image file to upload
 * @param options - Upload options
 * @returns The created media record or Chevereto URL
 */
export async function uploadImage(
  file: File,
  options: UploadOptions = {}
): Promise<Media | CheveretoUploadResult> {
  const { diaryId, diaryDate, alt, signal, onProgress } = options;

  // Validate file type
  const allowedTypes = ['image/jpeg', 'image/png', 'image/gif', 'image/webp', 'image/svg+xml'];
  if (!allowedTypes.includes(file.type)) {
    throw new Error(`Invalid file type: ${file.type}. Allowed types: ${allowedTypes.join(', ')}`);
  }

  // Validate file size (50MB max - must match backend media limit)
  const maxSize = 50 * 1024 * 1024;
  if (file.size > maxSize) {
    throw new Error(
      `File size exceeds 50MB limit. File size: ${(file.size / 1024 / 1024).toFixed(2)}MB`
    );
  }

  // Check active image upload provider
  if (!isImageUploadLoaded()) {
    await loadImageUploadSettings();
  }
  const settings = get(imageUploadSettings);

  if (settings.provider === 'chevereto') {
    try {
      onProgress?.({ loaded: 0, total: file.size, percentage: 0 });
      const result = await uploadToChevereto(file);
      onProgress?.({ loaded: file.size, total: file.size, percentage: 100 });
      return { cheveretoUrl: result.url };
    } catch (error) {
      console.error('Chevereto upload failed:', error);
      throw new Error('Failed to upload image to Chevereto. Please try again.', { cause: error });
    }
  }

  // Fallback to local Diarum upload
  let resolvedDiaryId = diaryId;
  if (!resolvedDiaryId && diaryDate) {
    resolvedDiaryId = await getOrCreateDiaryId(diaryDate);
  }

  const formData = new FormData();
  formData.append('file', file);
  formData.append('name', file.name);
  formData.append('owner', pb.authStore.model?.id || '');

  if (alt) {
    formData.append('alt', alt);
  }

  if (resolvedDiaryId) {
    formData.append('diary', resolvedDiaryId);
  }

  try {
    return await uploadMediaWithProgress(formData, signal, onProgress);
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') {
      throw error;
    }
    console.error('Upload failed:', error);
    throw new Error('Failed to upload image. Please try again.', { cause: error });
  }
}

function uploadMediaWithProgress(
  formData: FormData,
  signal: AbortSignal | undefined,
  onProgress: ((progress: UploadProgress) => void) | undefined
): Promise<Media> {
  return new Promise((resolve, reject) => {
    if (signal?.aborted) {
      reject(new DOMException('Aborted', 'AbortError'));
      return;
    }

    const xhr = new XMLHttpRequest();
    xhr.open('POST', '/api/v1/media');
    xhr.setRequestHeader('Authorization', `Bearer ${pb.authStore.token}`);
    xhr.responseType = 'text';

    const handleAbort = () => xhr.abort();
    signal?.addEventListener('abort', handleAbort, { once: true });

    xhr.upload.onprogress = (event) => {
      if (!event.lengthComputable) return;
      const total = event.total || 1;
      onProgress?.({
        loaded: event.loaded,
        total,
        percentage: Math.min(100, Math.round((event.loaded / total) * 100))
      });
    };

    xhr.onload = () => {
      signal?.removeEventListener('abort', handleAbort);
      if (xhr.status >= 200 && xhr.status < 300) {
        try {
          resolve(JSON.parse(xhr.responseText) as Media);
        } catch {
          reject(new Error('Invalid upload response'));
        }
        return;
      }
      reject(new Error(xhr.responseText || `Upload failed with status ${xhr.status}`));
    };

    xhr.onerror = () => {
      signal?.removeEventListener('abort', handleAbort);
      reject(new Error('Network error while uploading'));
    };

    xhr.onabort = () => {
      signal?.removeEventListener('abort', handleAbort);
      reject(new DOMException('Aborted', 'AbortError'));
    };

    xhr.send(formData);
  });
}

/**
 * Get the full URL for a media file
 * @param media - The media record
 * @param thumb - Optional thumbnail size (e.g., "100x100", "300x300", "800x600")
 * @returns The full URL to the image
 */
export function getMediaUrl(media: Media, thumb?: string): string {
  if (!media.id || !media.file) {
    throw new Error('Invalid media record');
  }

  const url = `/api/v1/files/media/${encodeURIComponent(media.id)}/${encodeURIComponent(media.file)}`;
  return thumb ? `${url}?thumb=${encodeURIComponent(thumb)}` : url;
}

/**
 * Delete a media record
 * @param mediaId - The ID of the media record to delete
 */
export async function deleteMedia(mediaId: string): Promise<void> {
  try {
    const response = await fetch(`/api/v1/media/${encodeURIComponent(mediaId)}`, {
      method: 'DELETE',
      headers: {
        Authorization: `Bearer ${pb.authStore.token}`
      }
    });
    if (!response.ok) {
      throw new Error(await response.text());
    }
  } catch (error) {
    console.error('Delete failed:', error);
    throw new Error('Failed to delete media. Please try again.', { cause: error });
  }
}

/**
 * Upload image from URL
 * @param url - The image URL
 * @param options - Upload options
 */
export async function uploadImageFromUrl(
  url: string,
  options: UploadOptions = {}
): Promise<Media | CheveretoUploadResult> {
  try {
    const response = await fetch(url);
    if (!response.ok) {
      throw new Error('Failed to fetch image from URL');
    }

    const blob = await response.blob();
    const filename = url.split('/').pop() || 'image.jpg';
    const file = new File([blob], filename, { type: blob.type });

    return await uploadImage(file, options);
  } catch (error) {
    console.error('Upload from URL failed:', error);
    throw new Error('Failed to upload image from URL. Please try again.', { cause: error });
  }
}

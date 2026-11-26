import { get, writable } from 'svelte/store';

export interface LightboxImage {
  src: string;
  alt?: string;
}

interface LightboxState {
  open: boolean;
  images: LightboxImage[];
  index: number;
}

const initialState: LightboxState = {
  open: false,
  images: [],
  index: 0
};

export const lightbox = writable<LightboxState>(initialState);

/**
 * 打开灯箱。支持单张图片、图片数组或带索引的图片数组。
 */
export function openLightbox(
  input: string | LightboxImage | LightboxImage[],
  index: number = 0
): void {
  const images: LightboxImage[] =
    typeof input === 'string'
      ? [{ src: input }]
      : Array.isArray(input)
        ? input.filter((image) => Boolean(image?.src))
        : [input];

  if (images.length === 0) return;

  lightbox.set({
    open: true,
    images,
    index: Math.min(Math.max(index, 0), images.length - 1)
  });
}

export function closeLightbox(): void {
  lightbox.set(initialState);
}

export function showLightboxImage(index: number): void {
  const state = get(lightbox);
  if (!state.open || state.images.length === 0) return;
  lightbox.set({
    ...state,
    index: Math.min(Math.max(index, 0), state.images.length - 1)
  });
}

export function showNextLightboxImage(): void {
  const state = get(lightbox);
  if (!state.open || state.images.length <= 1) return;
  lightbox.set({ ...state, index: (state.index + 1) % state.images.length });
}

export function showPrevLightboxImage(): void {
  const state = get(lightbox);
  if (!state.open || state.images.length <= 1) return;
  lightbox.set({
    ...state,
    index: (state.index - 1 + state.images.length) % state.images.length
  });
}

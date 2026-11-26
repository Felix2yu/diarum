<script lang="ts">
  import {
    closeLightbox,
    lightbox,
    showLightboxImage,
    showNextLightboxImage,
    showPrevLightboxImage
  } from '#lib/stores/lightbox.js';

  $effect(() => {
    if (!$lightbox.open) return;
    const previous = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    return () => {
      document.body.style.overflow = previous;
    };
  });

  function handleKeydown(event: KeyboardEvent) {
    if (!$lightbox.open) return;
    if (event.key === 'Escape') {
      event.preventDefault();
      closeLightbox();
    } else if (event.key === 'ArrowRight') {
      event.preventDefault();
      showNextLightboxImage();
    } else if (event.key === 'ArrowLeft') {
      event.preventDefault();
      showPrevLightboxImage();
    }
  }

  function imageLabel(index: number): string {
    return $lightbox.images[index]?.alt || `图片 ${index + 1}`;
  }
</script>

<svelte:window onkeydown={handleKeydown} />

{#if $lightbox.open}
  <div
    class="animate-fade-in fixed inset-0 z-[200] flex items-center justify-center bg-black/90 p-4 backdrop-blur-sm"
    role="dialog"
    aria-modal="true"
    aria-label="图片预览"
  >
    <button
      type="button"
      class="absolute inset-0 h-full w-full cursor-zoom-out"
      aria-label="关闭图片预览"
      onclick={closeLightbox}
    ></button>

    <div class="relative z-10 flex max-h-full max-w-full flex-col items-center gap-3">
      <img
        src={$lightbox.images[$lightbox.index]?.src}
        alt={imageLabel($lightbox.index)}
        class="max-h-[86vh] max-w-[92vw] rounded-lg object-contain shadow-2xl"
        ondblclick={closeLightbox}
      />
      {#if $lightbox.images.length > 1}
        <div class="rounded-full bg-black/60 px-3 py-1 text-xs text-white/90">
          {$lightbox.index + 1} / {$lightbox.images.length}
        </div>
      {/if}
    </div>

    <button
      type="button"
      class="absolute top-4 right-4 z-20 rounded-full bg-black/60 p-2 text-white transition-colors hover:bg-black/80"
      aria-label="关闭图片预览"
      onclick={closeLightbox}
    >
      <svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M6 18L18 6M6 6l12 12"
        />
      </svg>
    </button>

    {#if $lightbox.images.length > 1}
      <button
        type="button"
        class="absolute top-1/2 left-4 z-20 -translate-y-1/2 rounded-full bg-black/60 p-2 text-white transition-colors hover:bg-black/80"
        aria-label="上一张"
        onclick={showPrevLightboxImage}
      >
        <svg class="h-6 w-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M15 19l-7-7 7-7"
          />
        </svg>
      </button>
      <button
        type="button"
        class="absolute top-1/2 right-4 z-20 -translate-y-1/2 rounded-full bg-black/60 p-2 text-white transition-colors hover:bg-black/80"
        aria-label="下一张"
        onclick={showNextLightboxImage}
      >
        <svg class="h-6 w-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
        </svg>
      </button>

      <div class="absolute bottom-4 left-1/2 z-20 flex -translate-x-1/2 gap-2">
        {#each $lightbox.images as image, index (image.src + index)}
          <button
            type="button"
            class="h-2.5 w-2.5 rounded-full transition-colors {index === $lightbox.index
              ? 'bg-white'
              : 'bg-white/40 hover:bg-white/70'}"
            aria-label="查看第 {index + 1} 张图片"
            onclick={() => showLightboxImage(index)}
          ></button>
        {/each}
      </div>
    {/if}
  </div>
{/if}

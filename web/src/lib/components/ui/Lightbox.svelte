<script lang="ts">
  import {
    closeLightbox,
    lightbox,
    showLightboxImage,
    showNextLightboxImage,
    showPrevLightboxImage
  } from '#lib/stores/lightbox.js';

  const MIN_SCALE = 1;
  const MAX_SCALE = 4;
  // 拖动/缩放的过渡时间，与 transform 上的 transition 保持一致
  const ANIMATION_MS = 180;
  // 横向滑动超过该距离且耗时较短时切换图片
  const SWIPE_THRESHOLD = 60;
  // 纵向下拉超过该距离时关闭灯箱
  const SWIPE_CLOSE_THRESHOLD = 100;

  let imageEl = $state<HTMLImageElement | null>(null);
  let scale = $state(1);
  let translateX = $state(0);
  let translateY = $state(0);
  // 移动端单指横向滑动时的实时位移
  let swipeOffset = $state(0);
  let animating = $state(false);
  let dragging = $state(false);

  let touchStart: { x: number; y: number; at: number; tx: number; ty: number } | null = null;
  let pinchStart: { distance: number; scale: number } | null = null;
  let mouseStart: { x: number; y: number; tx: number; ty: number } | null = null;

  $effect(() => {
    if (!$lightbox.open) return;
    const previous = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    return () => {
      document.body.style.overflow = previous;
    };
  });

  // 切换图片或关闭时复位缩放与位移
  $effect(() => {
    if (!$lightbox.open) return;
    void $lightbox.index;
    resetZoom(false);
  });

  function resetZoom(animated = true) {
    animate(animated);
    scale = MIN_SCALE;
    translateX = 0;
    translateY = 0;
    swipeOffset = 0;
  }

  function animate(enabled: boolean) {
    animating = enabled;
    if (enabled) {
      window.setTimeout(() => {
        animating = false;
      }, ANIMATION_MS);
    }
  }

  // 缩放后限制平移范围，避免图片被拖出可视区域
  function clampTranslate(x: number, y: number, currentScale: number) {
    if (!imageEl || currentScale <= MIN_SCALE) return { x: 0, y: 0 };
    const maxX = ((currentScale - 1) * imageEl.offsetWidth) / 2;
    const maxY = ((currentScale - 1) * imageEl.offsetHeight) / 2;
    return {
      x: Math.max(-maxX, Math.min(maxX, x)),
      y: Math.max(-maxY, Math.min(maxY, y))
    };
  }

  // 以视口坐标 (clientX, clientY) 为锚点缩放，保证该点下方的内容不移动
  function zoomAt(clientX: number, clientY: number, nextScale: number, animated = true) {
    if (!imageEl) return;
    const target = Math.min(MAX_SCALE, Math.max(MIN_SCALE, nextScale));
    const rect = imageEl.getBoundingClientRect();
    const centerX = rect.left + rect.width / 2;
    const centerY = rect.top + rect.height / 2;
    const pointX = clientX - centerX;
    const pointY = clientY - centerY;
    const ratio = target / scale;
    const nextX = pointX - (pointX - translateX) * ratio;
    const nextY = pointY - (pointY - translateY) * ratio;
    animate(animated);
    if (target <= MIN_SCALE) {
      scale = MIN_SCALE;
      translateX = 0;
      translateY = 0;
      swipeOffset = 0;
      return;
    }
    const bounded = clampTranslate(nextX, nextY, target);
    scale = target;
    translateX = bounded.x;
    translateY = bounded.y;
  }

  function zoomBy(factor: number) {
    if (!imageEl) return;
    const rect = imageEl.getBoundingClientRect();
    zoomAt(rect.left + rect.width / 2, rect.top + rect.height / 2, scale * factor);
  }

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
    } else if (event.key === '+' || event.key === '=') {
      event.preventDefault();
      zoomBy(1.25);
    } else if (event.key === '-') {
      event.preventDefault();
      zoomBy(1 / 1.25);
    } else if (event.key === '0') {
      event.preventDefault();
      resetZoom();
    }
  }

  function handleWheel(event: WheelEvent) {
    event.preventDefault();
    zoomAt(event.clientX, event.clientY, event.deltaY < 0 ? scale * 1.2 : scale / 1.2, false);
  }

  function handleDoubleClick(event: MouseEvent) {
    event.stopPropagation();
    if (scale > MIN_SCALE) {
      resetZoom();
      return;
    }
    zoomAt(event.clientX, event.clientY, 2.5);
  }

  // ---- 鼠标：放大后可拖动平移 ----
  function handlePointerDown(event: PointerEvent) {
    if (event.pointerType !== 'mouse' || event.button !== 0 || scale <= MIN_SCALE) return;
    mouseStart = { x: event.clientX, y: event.clientY, tx: translateX, ty: translateY };
    dragging = true;
    (event.currentTarget as HTMLElement).setPointerCapture(event.pointerId);
  }

  function handlePointerMove(event: PointerEvent) {
    if (!mouseStart || event.pointerType !== 'mouse') return;
    const bounded = clampTranslate(
      mouseStart.tx + (event.clientX - mouseStart.x),
      mouseStart.ty + (event.clientY - mouseStart.y),
      scale
    );
    translateX = bounded.x;
    translateY = bounded.y;
  }

  function handlePointerUp(event: PointerEvent) {
    if (event.pointerType !== 'mouse') return;
    mouseStart = null;
    dragging = false;
  }

  // ---- 触摸：双指缩放、放大后单指平移、未放大时滑动切换 ----
  function touchDistance(touches: TouchList): number {
    const [first, second] = [touches[0], touches[1]];
    return Math.hypot(first.clientX - second.clientX, first.clientY - second.clientY);
  }

  function handleTouchStart(event: TouchEvent) {
    if (event.touches.length >= 2) {
      touchStart = null;
      swipeOffset = 0;
      pinchStart = { distance: touchDistance(event.touches), scale };
      return;
    }
    const touch = event.touches[0];
    if (!touch) return;
    touchStart = { x: touch.clientX, y: touch.clientY, at: Date.now(), tx: translateX, ty: translateY };
  }

  function handleTouchMove(event: TouchEvent) {
    if (event.touches.length >= 2 && pinchStart) {
      const distance = touchDistance(event.touches);
      if (pinchStart.distance <= 0) return;
      const first = event.touches[0];
      const second = event.touches[1];
      const midX = (first.clientX + second.clientX) / 2;
      const midY = (first.clientY + second.clientY) / 2;
      zoomAt(midX, midY, pinchStart.scale * (distance / pinchStart.distance), false);
      return;
    }
    if (event.touches.length !== 1 || !touchStart) return;
    const touch = event.touches[0];
    const deltaX = touch.clientX - touchStart.x;
    const deltaY = touch.clientY - touchStart.y;
    if (scale > MIN_SCALE) {
      const bounded = clampTranslate(touchStart.tx + deltaX, touchStart.ty + deltaY, scale);
      translateX = bounded.x;
      translateY = bounded.y;
      return;
    }
    if (Math.abs(deltaX) > Math.abs(deltaY)) {
      // 横向跟手，形成滑动反馈
      swipeOffset = deltaX;
    }
  }

  function handleTouchEnd(event: TouchEvent) {
    if (event.touches.length > 0) return;

    if (pinchStart) {
      pinchStart = null;
      if (scale < 1.05) resetZoom();
      return;
    }
    if (!touchStart) {
      swipeOffset = 0;
      return;
    }

    const touch = event.changedTouches[0];
    const start = touchStart;
    touchStart = null;
    if (!touch) {
      swipeOffset = 0;
      return;
    }

    const deltaX = touch.clientX - start.x;
    const deltaY = touch.clientY - start.y;
    const elapsed = Date.now() - start.at;
    if (scale > MIN_SCALE) return;

    if (Math.abs(deltaX) > SWIPE_THRESHOLD && Math.abs(deltaX) > Math.abs(deltaY) && elapsed < 800) {
      animate(false);
      swipeOffset = 0;
      if (deltaX < 0) {
        showNextLightboxImage();
      } else {
        showPrevLightboxImage();
      }
      return;
    }
    if (deltaY > SWIPE_CLOSE_THRESHOLD && Math.abs(deltaY) > Math.abs(deltaX)) {
      closeLightbox();
      return;
    }
    animate(true);
    swipeOffset = 0;
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
        bind:this={imageEl}
        src={$lightbox.images[$lightbox.index]?.src}
        alt={imageLabel($lightbox.index)}
        class="max-h-[86vh] max-w-[92vw] rounded-lg object-contain shadow-2xl select-none"
        style="transform: translate3d({swipeOffset + translateX}px, {translateY}px, 0) scale({scale});
               transition: {animating ? `transform ${ANIMATION_MS}ms ease-out` : 'none'};
               touch-action: none;
               cursor: {scale > MIN_SCALE ? (dragging ? 'grabbing' : 'grab') : 'zoom-in'};"
        draggable="false"
        ondragstart={(event) => event.preventDefault()}
        onwheel={handleWheel}
        ondblclick={handleDoubleClick}
        onpointerdown={handlePointerDown}
        onpointermove={handlePointerMove}
        onpointerup={handlePointerUp}
        onpointercancel={handlePointerUp}
        ontouchstart={handleTouchStart}
        ontouchmove={handleTouchMove}
        ontouchend={handleTouchEnd}
        ontouchcancel={handleTouchEnd}
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

    <div
      class="absolute right-4 bottom-4 z-20 flex items-center gap-1 rounded-full bg-black/60 p-1 text-white"
    >
      <button
        type="button"
        class="flex h-8 w-8 items-center justify-center rounded-full transition-colors hover:bg-white/15 disabled:opacity-40"
        aria-label="缩小"
        disabled={scale <= MIN_SCALE}
        onclick={() => zoomBy(1 / 1.25)}
      >
        <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 12h14" />
        </svg>
      </button>
      <button
        type="button"
        class="min-w-12 px-1 text-xs tabular-nums transition-colors hover:text-white"
        aria-label="重置缩放"
        onclick={() => resetZoom()}
      >
        {Math.round(scale * 100)}%
      </button>
      <button
        type="button"
        class="flex h-8 w-8 items-center justify-center rounded-full transition-colors hover:bg-white/15 disabled:opacity-40"
        aria-label="放大"
        disabled={scale >= MAX_SCALE}
        onclick={() => zoomBy(1.25)}
      >
        <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 5v14M5 12h14" />
        </svg>
      </button>
    </div>
  </div>
{/if}

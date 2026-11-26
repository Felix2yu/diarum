<script lang="ts">
  import { onDestroy } from 'svelte';
  import { SvelteMap } from 'svelte/reactivity';
  import { renderMarkdown } from '#lib/utils/renderMarkdown.js';
  import MediaPicker from '#lib/components/editor/MediaPicker.svelte';
  import UploadQueuePanel from '#lib/components/editor/UploadQueuePanel.svelte';
  import { isCheveretoResult } from '#lib/utils/uploadImage.js';
  import { getMediaFileUrl } from '#lib/api/media.js';
  import type { MediaWithDiary } from '#lib/api/media.js';
  import { UploadQueue, createUploadJobId, type UploadJob } from '#lib/utils/uploadQueue.js';
  import { openLightbox } from '#lib/stores/lightbox.js';
  import { mediaAuth } from '#lib/utils/mediaAuth.js';

  let {
    content = $bindable(''),
    onChange = (value: string) => {},
    placeholder = '开始书写...',
    selectedContent = $bindable(''),
    emptyStatePrompt = '',
    onFocusChange = (focused: boolean) => {}
  } = $props();

  let textareaEl = $state<HTMLTextAreaElement | null>(null);
  let isFocused = $state(false);
  let uploadError = $state('');
  let uploadJobs = $state<UploadJob[]>([]);
  let showMediaPicker = $state(false);

  let fileInput = $state<HTMLInputElement | null>(null);

  const uploadQueue = new UploadQueue({
    concurrency: 3,
    onUpdate: (jobs) => {
      uploadJobs = jobs;
    }
  });
  const placeholders = new SvelteMap<string, string>();

  onDestroy(() => uploadQueue.destroy());

  function handleInput(e: Event) {
    const ta = e.target as HTMLTextAreaElement;
    const val = ta.value;
    content = val;
    onChange(val);
  }

  function handleFocus() {
    isFocused = true;
    onFocusChange(true);
  }

  function handleBlur() {
    isFocused = false;
    onFocusChange(false);
  }

  function updateSelectedText() {
    if (!textareaEl) {
      selectedContent = '';
      return;
    }
    const start = textareaEl.selectionStart ?? 0;
    const end = textareaEl.selectionEnd ?? 0;
    if (start === end) {
      selectedContent = '';
      return;
    }
    selectedContent = textareaEl.value.substring(start, end);
  }

  function handlePreviewClick(event: MouseEvent) {
    const image = (event.target as HTMLElement).closest('img');
    if (image) {
      event.preventDefault();
      event.stopPropagation();
      openLightbox({ src: image.currentSrc || image.src, alt: image.alt });
      return;
    }
    if (!textareaEl) return;
    textareaEl.focus();
    const len = textareaEl.value.length;
    try {
      textareaEl.setSelectionRange(len, len);
    } catch (e) {
      // ignore
    }
  }

  function insertTextAtCursor(textToInsert: string, cursorOffset: number = 0) {
    if (!textareaEl) {
      content = (content ?? '') + textToInsert;
      onChange(content);
      return;
    }

    const start = textareaEl.selectionStart ?? content.length;
    const end = textareaEl.selectionEnd ?? content.length;
    const currentValue = textareaEl.value;

    const newValue = currentValue.substring(0, start) + textToInsert + currentValue.substring(end);

    textareaEl.value = newValue;
    content = newValue;
    onChange(newValue);

    const newCursor = start + textToInsert.length + cursorOffset;
    requestAnimationFrame(() => {
      if (textareaEl) {
        textareaEl.focus();
        try {
          textareaEl.setSelectionRange(newCursor, newCursor);
        } catch (e) {
          // ignore
        }
      }
    });
  }

  function triggerFileSelect() {
    uploadError = '';
    if (fileInput) {
      fileInput.value = '';
      fileInput.click();
    }
  }

  function triggerMediaPicker() {
    uploadError = '';
    showMediaPicker = true;
  }

  async function handleFileSelect(e: Event) {
    const input = e.target as HTMLInputElement;
    const files = Array.from(input.files ?? []);
    if (files.length === 0) return;

    uploadError = '';
    const validFiles: File[] = [];
    const errors: string[] = [];
    for (const file of files) {
      const error = validateUploadFile(file);
      if (error) {
        errors.push(`${file.name}：${error}`);
      } else {
        validFiles.push(file);
      }
    }
    if (errors.length > 0) {
      uploadError = errors.slice(0, 2).join('；');
    }
    if (validFiles.length === 0) {
      input.value = '';
      return;
    }

    const prepared = validFiles.map((file) => {
      const id = createUploadJobId();
      const token = `![上传中：${file.name}](uploading:${id})`;
      placeholders.set(id, token);
      return { file, id, token };
    });

    insertTextAtCursor(`\n${prepared.map((item) => item.token).join('\n')}\n`);

    for (const item of prepared) {
      uploadQueue.enqueue(
        item.file,
        {},
        {
          onSuccess: handleUploadSuccess,
          onError: handleUploadError,
          onCancel: handleUploadCancel
        },
        item.id
      );
    }

    input.value = '';
  }

  function validateUploadFile(file: File): string | null {
    const allowedTypes = ['image/jpeg', 'image/png', 'image/gif', 'image/webp', 'image/svg+xml'];
    if (!allowedTypes.includes(file.type)) {
      return '不支持的图片格式';
    }
    if (file.size > 50 * 1024 * 1024) {
      return '文件超过 50MB';
    }
    return null;
  }

  function fileNameWithoutExtension(name: string): string {
    return name.replace(/\.[^/.]+$/, '');
  }

  function replaceToken(token: string, replacement: string) {
    const current = textareaEl?.value ?? content ?? '';
    const index = current.indexOf(token);
    if (index === -1) {
      if (replacement) {
        insertTextAtCursor(`\n${replacement}\n`);
      }
      return;
    }

    const newValue = current.slice(0, index) + replacement + current.slice(index + token.length);
    const cursor = textareaEl?.selectionStart ?? newValue.length;
    const delta = replacement.length - token.length;

    if (textareaEl) {
      textareaEl.value = newValue;
    }
    content = newValue;
    onChange(newValue);

    requestAnimationFrame(() => {
      if (!textareaEl) return;
      const next = cursor > index ? Math.max(0, cursor + delta) : cursor;
      try {
        textareaEl.setSelectionRange(next, next);
      } catch {
        // ignore
      }
    });
  }

  function handleUploadSuccess(job: UploadJob) {
    const token = placeholders.get(job.id);
    if (!token || !job.result) return;

    let imageUrl = '';
    let altText = fileNameWithoutExtension(job.file.name);

    if (isCheveretoResult(job.result)) {
      imageUrl = job.result.cheveretoUrl;
    } else {
      imageUrl = getMediaFileUrl(job.result);
      if (job.result.name) {
        altText = fileNameWithoutExtension(job.result.name);
      }
    }

    if (!imageUrl) return;
    replaceToken(token, `![${altText}](${imageUrl})`);
    placeholders.delete(job.id);
    setTimeout(() => uploadQueue.remove(job.id), 1800);
  }

  function handleUploadError(job: UploadJob) {
    const token = placeholders.get(job.id);
    if (!token) return;
    const failedToken = `![上传失败：${job.file.name}](uploading:${job.id})`;
    replaceToken(token, failedToken);
    placeholders.set(job.id, failedToken);
  }

  function handleUploadCancel(job: UploadJob) {
    const token = placeholders.get(job.id);
    if (token) {
      replaceToken(token, '');
    }
    placeholders.delete(job.id);
  }

  function cancelUpload(id: string) {
    uploadQueue.cancel(id);
  }

  function retryUpload(id: string) {
    uploadQueue.retry(id);
  }

  function removeUpload(id: string) {
    const token = placeholders.get(id);
    if (token) {
      replaceToken(token, '');
    }
    placeholders.delete(id);
    uploadQueue.remove(id);
  }

  function clearFinishedUploads() {
    for (const job of uploadJobs) {
      if (job.status === 'uploading' || job.status === 'queued') continue;
      const token = placeholders.get(job.id);
      if (token) {
        replaceToken(token, '');
      }
      placeholders.delete(job.id);
    }
    uploadQueue.clearFinished();
  }

  function handleMediaSelect(media: MediaWithDiary) {
    const imageUrl = getMediaFileUrl(media);
    const altText = media.alt || media.name || '图片';
    const markdown = `\n![${altText}](${imageUrl})\n`;
    insertTextAtCursor(markdown);
    showMediaPicker = false;
  }

  function handleMediaPickerClose() {
    showMediaPicker = false;
  }

  $effect(() => {
    const cur = content;
    if (textareaEl && textareaEl.value !== cur) {
      if (!isFocused) {
        textareaEl.value = cur;
      } else {
        const len = textareaEl.value.length;
        if (cur === '' || Math.abs(len - cur.length) > 100) {
          const pos = textareaEl.selectionStart ?? len;
          textareaEl.value = cur;
          const newPos = Math.min(pos, cur.length);
          try {
            textareaEl.setSelectionRange(newPos, newPos);
          } catch (e) {
            // ignore
          }
        }
      }
    }
  });
</script>

<!-- eslint-disable svelte/no-at-html-tags -->
<!-- 本文件所有 {@html} 出口均已过 DOMPurify 净化 -->

<div class="markdown-editor">
  <input
    type="file"
    bind:this={fileInput}
    accept="image/jpeg,image/png,image/gif,image/webp,image/svg+xml"
    multiple
    onchange={handleFileSelect}
    class="hidden-file-input"
  />

  <textarea
    bind:this={textareaEl}
    class="markdown-textarea {isFocused ? 'is-focused' : 'is-blurred'}"
    {placeholder}
    oninput={handleInput}
    onfocus={handleFocus}
    onblur={handleBlur}
    onselect={updateSelectedText}
    onclick={updateSelectedText}
    spellcheck="false"></textarea>

  {#if !isFocused && content}
    <div class="markdown-preview" use:mediaAuth onclick={handlePreviewClick}>
      {@html renderMarkdown(content)}
    </div>
  {:else if !isFocused && !content && emptyStatePrompt}
    <button
      type="button"
      class="empty-state-overlay"
      onclick={handlePreviewClick}
      aria-label="编辑器焦点"
    >
      <div class="text-center text-muted-foreground">
        <p class="text-sm">{emptyStatePrompt}</p>
      </div>
    </button>
  {/if}

  <!-- 左下角：图片上传 + 媒体库（与右下角语音按钮风格一致） -->
  <div class="absolute bottom-3 left-3 z-10 flex items-center gap-2">
    {#if uploadError}
      <div
        class="bg-destructive/90 text-destructive-foreground rounded-lg px-3 py-1.5 text-xs shadow-md"
      >
        {uploadError}
      </div>
    {/if}
    <button
      type="button"
      onclick={triggerFileSelect}
      title="上传图片（可多选）"
      class="inline-flex items-center gap-2 rounded-full bg-primary/90 px-3 py-2 text-sm font-medium text-primary-foreground shadow-lg shadow-primary/20 transition-all hover:bg-primary"
    >
      <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z"
        />
      </svg>
      <span class="hidden sm:inline">上传图片</span>
    </button>
    <button
      type="button"
      onclick={triggerMediaPicker}
      title="从媒体库选择"
      class="inline-flex items-center gap-2 rounded-full bg-muted/80 px-3 py-2 text-sm font-medium text-foreground shadow-lg transition-all hover:bg-muted"
    >
      <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10"
        />
      </svg>
      <span class="hidden sm:inline">媒体库</span>
    </button>
  </div>

  <UploadQueuePanel
    jobs={uploadJobs}
    onCancel={cancelUpload}
    onRetry={retryUpload}
    onRemove={removeUpload}
    onClear={clearFinishedUploads}
  />

  {#if showMediaPicker}
    <MediaPicker onSelect={handleMediaSelect} onClose={handleMediaPickerClose} />
  {/if}
</div>

<!-- eslint-enable svelte/no-at-html-tags -->
<style>
  .markdown-editor {
    position: relative;
    width: 100%;
    flex: 1 1 auto;
    height: 100%;
    min-height: 500px;
    display: flex;
    flex-direction: column;
  }

  .hidden-file-input {
    display: none;
  }

  .markdown-textarea {
    display: block;
    width: 100%;
    flex: 1 1 auto;
    min-height: 500px;
    padding: 1.5rem 1.75rem;
    padding-bottom: 3.5rem;
    border: none;
    border-radius: 0.75rem;
    background: transparent;
    color: hsl(var(--foreground) / 0.92);
    font-size: 1rem;
    line-height: 1.8;
    font-family:
      ui-sans-serif,
      system-ui,
      -apple-system,
      'Segoe UI',
      sans-serif;
    outline: none;
    resize: none;
    white-space: pre-wrap;
    word-break: break-word;
    tab-size: 2;
    box-sizing: border-box;
    transition: opacity 0.15s ease;
  }

  .markdown-textarea.is-blurred {
    opacity: 0;
    pointer-events: none;
  }

  .markdown-textarea.is-focused {
    opacity: 1;
  }

  .markdown-preview {
    position: absolute;
    inset: 0;
    padding: 1.5rem 1.75rem;
    padding-bottom: 3.5rem;
    overflow-y: auto;
    cursor: text;
    color: hsl(var(--foreground) / 0.92);
    line-height: 1.8;
    font-size: 1rem;
    box-sizing: border-box;
  }

  .markdown-preview :global(> *:first-child) {
    margin-top: 0;
  }

  .markdown-preview :global(> *:last-child) {
    margin-bottom: 0;
  }

  .markdown-preview :global(p) {
    margin: 0 0 0.9rem;
  }

  /* 单个换行符（<br>）的高度，介于同一段落和新段落之间 */
  .markdown-preview :global(br) {
    display: block;
    content: '';
    margin-top: 0.45rem;
  }

  .markdown-preview :global(h1) {
    font-size: 1.65rem;
    font-weight: 600;
    margin: 1.8rem 0 0.9rem;
    padding-bottom: 0.35rem;
    border-bottom: 1px solid hsl(var(--border) / 0.55);
    color: hsl(var(--foreground));
  }

  .markdown-preview :global(h2) {
    font-size: 1.35rem;
    font-weight: 600;
    margin: 1.4rem 0 0.7rem;
    color: hsl(var(--foreground));
  }

  .markdown-preview :global(h3) {
    font-size: 1.15rem;
    font-weight: 600;
    margin: 1.2rem 0 0.55rem;
    color: hsl(var(--foreground));
  }

  .markdown-preview :global(h4),
  .markdown-preview :global(h5),
  .markdown-preview :global(h6) {
    font-size: 1rem;
    font-weight: 600;
    margin: 1rem 0 0.5rem;
    color: hsl(var(--foreground));
  }

  .markdown-preview :global(strong) {
    font-weight: 600;
    color: hsl(var(--foreground));
  }

  .markdown-preview :global(em) {
    font-style: italic;
  }

  .markdown-preview :global(ul),
  .markdown-preview :global(ol) {
    margin: 0.25rem 0 0.9rem;
    padding-left: 1.6rem;
  }

  .markdown-preview :global(li) {
    margin: 0.15rem 0;
  }

  .markdown-preview :global(ul li) {
    list-style: disc;
  }

  .markdown-preview :global(ol li) {
    list-style: decimal;
  }

  .markdown-preview :global(blockquote) {
    margin: 0.6rem 0 0.9rem;
    padding: 0.3rem 0.9rem;
    border-left: 3px solid hsl(var(--primary) / 0.55);
    background: hsl(var(--muted) / 0.35);
    color: hsl(var(--muted-foreground));
    border-radius: 0 0.5rem 0.5rem 0;
  }

  .markdown-preview :global(blockquote p) {
    margin: 0;
  }

  .markdown-preview :global(code) {
    padding: 0.1rem 0.38rem;
    background: hsl(var(--muted) / 0.55);
    border-radius: 0.3rem;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    font-size: 0.86em;
    color: hsl(var(--foreground) / 0.92);
  }

  .markdown-preview :global(pre) {
    background: hsl(var(--muted) / 0.35);
    border: 1px solid hsl(var(--border) / 0.45);
    border-radius: 0.5rem;
    padding: 0.85rem 1rem;
    overflow-x: auto;
    margin: 0.6rem 0 0.9rem;
  }

  .markdown-preview :global(pre code) {
    background: transparent;
    border: none;
    padding: 0;
    font-size: 0.86rem;
  }

  .markdown-preview :global(hr) {
    border: none;
    border-top: 1px solid hsl(var(--border) / 0.55);
    margin: 1rem 0;
  }

  .markdown-preview :global(a) {
    color: hsl(var(--primary));
    text-decoration: none;
  }

  .markdown-preview :global(a:hover) {
    text-decoration: underline;
  }

  .markdown-preview :global(img) {
    max-width: 100%;
    height: auto;
    border-radius: 0.5rem;
    display: block;
    margin: 0.6rem auto;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
  }

  .empty-state-overlay {
    position: absolute;
    inset: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    width: 100%;
    background: transparent;
    border: 0;
    padding: 0;
    cursor: text;
  }
</style>

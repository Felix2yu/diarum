<script lang="ts">
	import { marked } from 'marked';
	import MediaPicker from '$lib/components/editor/MediaPicker.svelte';
	import { uploadImage, isCheveretoResult, type UploadOptions } from '$lib/utils/uploadImage';
	import { getMediaFileUrl } from '$lib/api/media';
	import type { MediaWithDiary } from '$lib/api/media';

	let {
		content = $bindable(''),
		onChange = (value: string) => {},
		placeholder = '开始书写...',
		selectedContent = $bindable(''),
		emptyStatePrompt = '',
		diaryDate = $bindable(undefined as string | undefined)
	} = $props();

	let textareaEl = $state<HTMLTextAreaElement | null>(null);
	let isFocused = $state(false);
	let isUploading = $state(false);
	let uploadError = $state('');
	let showMediaPicker = $state(false);

	let fileInput = $state<HTMLInputElement | null>(null);

	marked.setOptions({
		breaks: true,
		gfm: true
	});

	function renderMarkdown(text: string): string {
		const trimmed = (text ?? '').trim();
		if (!trimmed) return '';
		try {
			return marked.parse(trimmed, { async: false }) as string;
		} catch (e) {
			return (text ?? '')
				.split('\n')
				.map((line) => `<p>${line}</p>`)
				.join('\n');
		}
	}

	function handleInput(e: Event) {
		const ta = e.target as HTMLTextAreaElement;
		const val = ta.value;
		content = val;
		onChange(val);
	}

	function handleFocus() {
		isFocused = true;
	}

	function handleBlur() {
		isFocused = false;
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

	function handlePreviewClick() {
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

		const newValue =
			currentValue.substring(0, start) + textToInsert + currentValue.substring(end);

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
		const files = input.files;
		if (!files || files.length === 0) return;

		const file = files[0];
		await handleUpload(file);
	}

	async function handleUpload(file: File) {
		isUploading = true;
		uploadError = '';

		try {
			const options: UploadOptions = {};
			if (diaryDate) {
				options.diaryDate = diaryDate;
			}

			const result = await uploadImage(file, options);

			let imageUrl: string;
			let altText: string = file.name.replace(/\.[^/.]+$/, '');

			if (isCheveretoResult(result)) {
				imageUrl = result.cheveretoUrl;
			} else {
				imageUrl = getMediaFileUrl(result);
				if (result.name) {
					altText = result.name.replace(/\.[^/.]+$/, '');
				}
			}

			const markdown = `\n![${altText}](${imageUrl})\n`;
			insertTextAtCursor(markdown);
		} catch (error: any) {
			console.error('Image upload failed:', error);
			uploadError = error.message || '上传失败，请重试';
		} finally {
			isUploading = false;
		}
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

<div class="markdown-editor">
	<input
		type="file"
		bind:this={fileInput}
		accept="image/jpeg,image/png,image/gif,image/webp,image/svg+xml"
		onchange={handleFileSelect}
		class="hidden-file-input"
	/>

	<textarea
		bind:this={textareaEl}
		class="markdown-textarea {isFocused ? 'is-focused' : 'is-blurred'}"
		placeholder={placeholder}
		oninput={handleInput}
		onfocus={handleFocus}
		onblur={handleBlur}
		onselect={updateSelectedText}
		onclick={updateSelectedText}
		spellcheck="false"
	></textarea>

	{#if !isFocused && content}
		<div class="markdown-preview" onclick={handlePreviewClick}>
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
	<div class="absolute bottom-3 left-3 flex items-center gap-2 z-10">
		{#if uploadError}
			<div class="px-3 py-1.5 rounded-lg bg-destructive/90 text-destructive-foreground text-xs shadow-md">
				{uploadError}
			</div>
		{/if}
		{#if isUploading}
			<button
				type="button"
				disabled
				class="inline-flex items-center gap-2 px-3 py-2 bg-muted/80 text-muted-foreground rounded-full text-sm font-medium shadow-lg transition-all"
			>
				<svg class="w-4 h-4 animate-spin" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<circle class="opacity-25" cx="12" cy="12" r="10" stroke-width="4" stroke="currentColor"/>
					<path class="opacity-75" fill="currentColor" stroke="currentColor" stroke-width="4" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"/>
				</svg>
				<span class="hidden sm:inline">上传中...</span>
			</button>
		{:else}
			<button
				type="button"
				onclick={triggerFileSelect}
				title="上传图片"
				class="inline-flex items-center gap-2 px-3 py-2 bg-primary/90 hover:bg-primary text-primary-foreground rounded-full text-sm font-medium shadow-lg shadow-primary/20 transition-all"
			>
				<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z"/>
				</svg>
				<span class="hidden sm:inline">上传图片</span>
			</button>
			<button
				type="button"
				onclick={triggerMediaPicker}
				title="从媒体库选择"
				class="inline-flex items-center gap-2 px-3 py-2 bg-muted/80 hover:bg-muted text-foreground rounded-full text-sm font-medium shadow-lg transition-all"
			>
				<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10"/>
				</svg>
				<span class="hidden sm:inline">媒体库</span>
			</button>
		{/if}
	</div>

	{#if showMediaPicker}
		<MediaPicker onSelect={handleMediaSelect} onClose={handleMediaPickerClose} />
	{/if}
</div>

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
		font-family: ui-sans-serif, system-ui, -apple-system, "Segoe UI", sans-serif;
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

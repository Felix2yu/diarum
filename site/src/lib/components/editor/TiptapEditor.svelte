<script lang="ts">
	import { onMount, onDestroy, tick } from 'svelte';
	import { marked } from 'marked';

	export let content = '';
	export let onChange: (value: string) => void = () => {};
	export let placeholder = '开始书写...';
	export let diaryDate: string | undefined = undefined;
	export let selectedContent: string = '';
	export let emptyStatePrompt: string = '';

	let textareaEl: HTMLTextAreaElement | null = null;
	let isFocused = false;

	// Markdown 渲染配置：只在组件实例初始化一次
	marked.setOptions({
		breaks: true,
		gfm: true,
		mangle: false,
		headerIds: false
	});

	function renderMarkdown(text: string): string {
		const trimmed = (text ?? '').trim();
		if (!trimmed) return '';
		try {
			return marked.parse(trimmed, { async: false }) as string;
		} catch (e) {
			// 回退：保留换行的纯文本
			return (text ?? '')
				.split('\n')
				.map((line) => `<p>${line}</p>`)
				.join('\n');
		}
	}

	function handleInput() {
		if (!textareaEl) return;
		const val = textareaEl.value;
		content = val;
		onChange(val);
	}

	function handleFocus() {
		isFocused = true;
	}

	function handleBlur() {
		isFocused = false;
	}

	function handleSelect() {
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

	// 当点击预览区域（非焦点状态），聚焦到 textarea 的末尾并移动光标
	function handlePreviewClick() {
		if (!textareaEl) return;
		textareaEl.focus();
		const len = textareaEl.value.length;
		// 把光标放到末尾
		try {
			textareaEl.setSelectionRange(len, len);
		} catch (e) {
			// ignore
		}
	}

	// 外部 content 变化时，同步 textarea 的值与滚动位置（避免光标跳动）
	let internalContent = '';
	$effect(() => {
		if (textareaEl && textareaEl.value !== content) {
			// 只有在非聚焦状态下才完全替换内容；聚焦时只替换非当前选择前后的内容，避免打断输入
			if (!isFocused) {
				textareaEl.value = content;
			} else {
				// 聚焦状态下不覆盖（输入由 handleInput 驱动）
				// 但如果外部来源差异过大（例如清空），跟随更新并尽量保留光标位置
				const cur = textareaEl.value;
				if (content === '' || cur.length === 0 || Math.abs(cur.length - content.length) > 100) {
					const pos = textareaEl.selectionStart ?? cur.length;
					textareaEl.value = content;
					const newPos = Math.min(pos, content.length);
					try {
						textareaEl.setSelectionRange(newPos, newPos);
					} catch (e) {
						// ignore
					}
				}
			}
		}
		internalContent = content;
	});

	$effect(() => {
		// 初始加载：设置 textarea 的 value
		if (textareaEl) {
			textareaEl.value = content;
		}
	});

	function toggleFocusFromClick() {
		handlePreviewClick();
	}
</script>

<div class="markdown-editor">
	<!-- 编辑模式：聚焦时显示 textarea -->
	<textarea
		bind:this={textareaEl}
		class="markdown-textarea {isFocused ? 'is-focused' : 'is-blurred'}"
		placeholder={placeholder}
		oninput={handleInput}
		onfocus={handleFocus}
		onblur={handleBlur}
		onselect={handleSelect}
		onclick={handleSelect}
		spellcheck="false"
	></textarea>

	<!-- 预览模式：失焦时覆盖渲染 Markdown -->
	{#if !isFocused && content}
		<div class="markdown-preview" onclick={toggleFocusFromClick}>
			{@html renderMarkdown(content)}
		</div>
	{:else if !isFocused && !content && emptyStatePrompt}
		<!-- 空状态 -->
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
</div>

<style>
	.markdown-editor {
		position: relative;
		width: 100%;
		min-height: 500px;
	}

	.markdown-textarea {
		display: block;
		width: 100%;
		min-height: 500px;
		padding: 1.5rem 1.75rem;
		border: none;
		background: transparent;
		color: hsl(var(--foreground) / 0.92);
		font-size: 1rem;
		line-height: 1.8;
		font-family: ui-sans-serif, system-ui, -apple-system, "Segoe UI", sans-serif;
		outline: none;
		resize: vertical;
		white-space: pre-wrap;
		word-break: break-word;
		tab-size: 2;
		box-sizing: border-box;
		transition: opacity 0.15s ease;
		/* 失焦时降低透明度并移到后方（由预览层覆盖可见） */
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

	.markdown-preview :global(table) {
		width: 100%;
		border-collapse: collapse;
		margin: 0.6rem 0 0.9rem;
		font-size: 0.95rem;
	}

	.markdown-preview :global(th),
	.markdown-preview :global(td) {
		border: 1px solid hsl(var(--border) / 0.5);
		padding: 0.45rem 0.6rem;
		text-align: left;
	}

	.markdown-preview :global(th) {
		background: hsl(var(--muted) / 0.5);
		font-weight: 600;
	}

	.markdown-preview :global(tr:nth-child(even) td) {
		background: hsl(var(--muted) / 0.15);
	}

	.markdown-preview :global(img) {
		max-width: 100%;
		height: auto;
		border-radius: 0.5rem;
		display: block;
		margin: 0.6rem auto;
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

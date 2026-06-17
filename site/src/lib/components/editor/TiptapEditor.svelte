<script lang="ts">
	import { marked } from 'marked';

	// Props — using runes syntax: $props() + $bindable() for two-way binding
	let {
		content = '',
		onChange = (_v: string) => {},
		placeholder = '开始书写...',
		diaryDate = undefined as string | undefined,
		selectedContent = $bindable(''),
		emptyStatePrompt = ''
	}: {
		content?: string;
		onChange?: (value: string) => void;
		placeholder?: string;
		diaryDate?: string;
		selectedContent?: string;
		emptyStatePrompt?: string;
	} = $props();

	// Reactive state
	let textareaEl = $state<HTMLTextAreaElement | null>(null);
	let isFocused = $state(false);

	// Marked is configured once per instance via module init — no state needed
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
			return (text ?? '')
				.split('\n')
				.map((line) => `<p>${line}</p>`)
				.join('\n');
		}
	}

	function handleInput() {
		if (!textareaEl) return;
		onChange(textareaEl.value);
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

	// Sync textarea value when content prop changes (e.g. navigating between dates)
	// — avoid disrupting the user while typing (focused state)
	$effect(() => {
		const el = textareaEl;
		if (!el) return;
		if (el.value === content) return;
		if (!isFocused) {
			el.value = content ?? '';
		} else if (Math.abs((el.value.length ?? 0) - (content?.length ?? 0)) > 80) {
			// Significant external change while focused (e.g. cache restore) — sync but keep cursor
			const pos = el.selectionStart ?? el.value.length;
			el.value = content ?? '';
			const newPos = Math.min(pos, el.value.length);
			try {
				el.setSelectionRange(newPos, newPos);
			} catch (e) {
				// ignore
			}
		}
	});
</script>

<div class="markdown-editor">
	<!-- Editing layer — always present; gets hidden when not focused -->
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

	<!-- Preview layer — shown when textarea is blurred + has content -->
	{#if !isFocused && content}
		<div class="markdown-preview" onclick={handlePreviewClick}>
			{@html renderMarkdown(content)}
		</div>
	{/if}

	<!-- Empty state layer -->
	{#if !isFocused && !content && emptyStatePrompt}
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

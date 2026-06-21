<script lang="ts">
	let {
		content = '',
		className = '',
		tags: rawTags = [] as string[],
		tagInputValue = '',
		onNavigate: onNavigateProp = (() => {}) as () => void,
		onTagInput = ((_s: string) => {}) as (s: string) => void,
		onTagAdd = (() => {}) as () => void,
		onTagRemove = ((_t: string) => {}) as (t: string) => void,
		onTagKeydown = ((_e: KeyboardEvent) => {}) as (e: KeyboardEvent) => void,
		allTags = [] as string[],
		tagSuggestions = [] as string[],
		showTagSuggestions = false,
		selectedSuggestionIndex = -1,
		onSuggestionSelect = ((_t: string) => {}) as (t: string) => void,
		onSuggestionFocus = (() => {}) as () => void,
		onSuggestionBlur = (() => {}) as () => void
	} = $props();

	interface TocItem {
		id: string;
		text: string;
		level: number;
	}

	let headings = $derived<TocItem[]>(extractHeadings(content));

	function extractHeadings(text: string): TocItem[] {
		if (!text) return [];

		const items: TocItem[] = [];
		const mdRegex = /^(#{1,3})\s+(.+?)\s*#*\s*$/gm;
		const htmlRegex = /<h([1-3])[^>]*>([^<]+)<\/h[1-3]>/gi;

		let match;
		let index = 0;

		while ((match = mdRegex.exec(text)) !== null) {
			const level = match[1].length;
			const headingText = match[2].trim();
			if (!headingText) continue;
			items.push({ id: `heading-${index++}`, text: headingText, level });
		}

		if (items.length === 0) {
			while ((match = htmlRegex.exec(text)) !== null) {
				const level = parseInt(match[1]);
				const headingText = match[2].trim();
				if (!headingText) continue;
				items.push({ id: `heading-${index++}`, text: headingText, level });
			}
		}

		return items;
	}

	function scrollToHeading(id: string) {
		const headingIndex = parseInt(id.replace('heading-', ''));
		const editorEl = document.querySelector('.markdown-preview');
		if (!editorEl) return;

		const headingEls = editorEl.querySelectorAll('h1, h2, h3');
		const targetEl = headingEls[headingIndex];

		if (targetEl) {
			const headerOffset = 60;
			const elementPosition = targetEl.getBoundingClientRect().top;
			const offsetPosition = elementPosition + window.pageYOffset - headerOffset;

			window.scrollTo({
				top: offsetPosition,
				behavior: 'smooth'
			});

			onNavigateProp();
		}
	}

	function handleTagInputChange(e: Event) {
		const target = e.target as HTMLInputElement;
		onTagInput(target.value);
	}
</script>

<div class={className}>
	<div class="mb-4">
		<div class="text-muted-foreground/60 text-xs uppercase font-semibold mb-2">标签</div>
		<div class="flex flex-wrap gap-1.5 mb-2">
			{#each rawTags as tag (tag)}
				<span
					class="inline-flex items-center gap-1 group bg-primary/10 text-primary border border-primary/20 rounded-full px-2 py-0.5 text-xs hover:bg-primary/15 transition-colors"
				>
					{tag}
					<button
						type="button"
						onclick={() => onTagRemove(tag)}
						class="opacity-60 hover:opacity-100 hover:text-destructive transition-opacity"
						aria-label={`移除标签 ${tag}`}
					>
						<svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M6 18L18 6M6 6l12 12" />
						</svg>
					</button>
				</span>
			{/each}
		</div>
		<div class="relative">
			<input
				type="text"
				value={tagInputValue}
				oninput={handleTagInputChange}
				onkeydown={onTagKeydown}
				onfocus={onSuggestionFocus}
				onblur={onSuggestionBlur}
				placeholder="添加标签，回车或逗号分隔"
				class="w-full text-xs px-2 py-1.5 rounded-md bg-background border border-border/70 focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary/30 transition-colors placeholder:text-muted-foreground/50"
			/>
			{#if showTagSuggestions && tagSuggestions.length > 0}
				<div class="absolute left-0 right-0 top-full mt-1 bg-card border border-border/50 rounded-lg shadow-lg z-50 max-h-40 overflow-y-auto">
					{#each tagSuggestions as suggestion, i}
						<button
							type="button"
							onmousedown={(e) => { e.preventDefault(); onSuggestionSelect(suggestion); }}
							class="w-full text-left px-3 py-1.5 text-xs hover:bg-muted/50 transition-colors {i === selectedSuggestionIndex ? 'bg-muted/50 text-foreground' : 'text-muted-foreground'}"
						>
							<span class="text-foreground font-medium">{suggestion.slice(0, tagInputValue.trim().length)}</span><span>{suggestion.slice(tagInputValue.trim().length)}</span>
						</button>
					{/each}
				</div>
			{/if}
		</div>
	</div>

	{#if headings.length > 0}
		<div class="text-muted-foreground/60 text-xs uppercase font-semibold mb-2">目录</div>
		<nav>
			<ul class="space-y-1">
				{#each headings as heading (heading.id)}
					<li>
						<button
							type="button"
							onclick={() => scrollToHeading(heading.id)}
							class="w-full text-left px-2 py-1 rounded-md hover:bg-accent/60 transition-colors text-sm text-muted-foreground hover:text-foreground truncate"
							class:pl-4={heading.level === 2}
							class:pl-6={heading.level === 3}
							title={heading.text}
						>
							{heading.text}
						</button>
					</li>
				{/each}
			</ul>
		</nav>
	{/if}
</div>

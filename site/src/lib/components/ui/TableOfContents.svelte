<script lang="ts">
	let {
		content = '',
		className = '',
		onNavigate: onNavigateProp = (() => {}) as () => void
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
</script>

<div class={className}>
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
	{:else}
		<div class="text-muted-foreground/50 text-sm">
			<svg class="w-8 h-8 mx-auto mb-2 opacity-50" fill="none" stroke="currentColor" viewBox="0 0 24 24">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5"
					d="M4 6h16M4 12h16M4 18h7" />
			</svg>
			<p>暂无标题</p>
			<p class="text-xs mt-1">使用 # 来创建标题</p>
		</div>
	{/if}
</div>

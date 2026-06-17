<script lang="ts">
	export let content = '';
	export let className = '';
	export let onNavigate: (() => void) | undefined = undefined;

	interface TocItem {
		id: string;
		text: string;
		level: number;
	}

	$: headings = extractHeadings(content);

	function extractHeadings(text: string): TocItem[] {
		if (!text) return [];

		const items: TocItem[] = [];
		// 匹配 Markdown 标题：行首 #/##/### + 空格 + 标题文本
		// 同时兼顾 HTML 标题（向后兼容 Tiptap 时代的旧内容）
		const mdRegex = /^(#{1,3})\s+(.+?)\s*#*\s*$/gm;
		const htmlRegex = /<h([1-3])[^>]*>([^<]+)<\/h[1-3]>/gi;

		let match;
		let index = 0;

		while ((match = mdRegex.exec(text)) !== null) {
			const level = match[1].length; // '#' 的数量
			const headingText = match[2].trim();
			if (!headingText) continue;
			items.push({ id: `heading-${index++}`, text: headingText, level });
		}

		// 如果没有 Markdown 标题，尝试 HTML 标题（兼容旧内容）
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
		// Markdown 预览区域的标题
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

			onNavigate?.();
		}
	}
</script>

{#if headings.length > 0}
	<nav class="toc {className}">
		<div class="text-xs font-semibold text-muted-foreground uppercase tracking-wider mb-3 px-2">
			Contents
		</div>
		<ul class="space-y-1">
			{#each headings as heading, i}
				<li style="animation-delay: {i * 30}ms" class="animate-fade-in opacity-0">
					<button
						on:click={() => scrollToHeading(heading.id)}
						class="w-full text-left px-2 py-1 text-sm rounded-md
							   text-muted-foreground hover:text-foreground hover:bg-muted/50
							   transition-all duration-200 truncate
							   {heading.level === 1 ? 'font-medium' : ''}
							   {heading.level === 2 ? 'pl-4' : ''}
							   {heading.level === 3 ? 'pl-6 text-xs' : ''}"
					>
						{heading.text}
					</button>
				</li>
			{/each}
		</ul>
	</nav>
{:else}
	<div class="toc {className} text-center py-8">
		<div class="text-muted-foreground/50 text-sm">
			<svg class="w-8 h-8 mx-auto mb-2 opacity-50" fill="none" stroke="currentColor" viewBox="0 0 24 24">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5"
					d="M4 6h16M4 12h16M4 18h7" />
			</svg>
			<p>暂无标题</p>
			<p class="text-xs mt-1">使用 # 来创建标题</p>
		</div>
	</div>
{/if}

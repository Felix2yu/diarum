<script lang="ts">
  let {
    content = '',
    className = '',
    tags: rawTags = [] as string[],
    tagInputValue = '',
    onNavigate: onNavigateProp = (() => {}) as () => void,
    onTagInput = ((_s: string) => {}) as (s: string) => void,
    onTagRemove = ((_t: string) => {}) as (t: string) => void,
    onTagKeydown = ((_e: KeyboardEvent) => {}) as (e: KeyboardEvent) => void,
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
    <div class="mb-2 text-xs font-semibold text-muted-foreground/60 uppercase">标签</div>
    <div class="mb-2 flex flex-wrap gap-1.5">
      {#each rawTags as tag (tag)}
        <span
          class="group inline-flex items-center gap-1 rounded-full border border-primary/20 bg-primary/10 px-2 py-0.5 text-xs text-primary transition-colors hover:bg-primary/15"
        >
          {tag}
          <button
            type="button"
            onclick={() => onTagRemove(tag)}
            class="hover:text-destructive opacity-60 transition-opacity hover:opacity-100"
            aria-label={`移除标签 ${tag}`}
          >
            <svg class="h-3 w-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2.5"
                d="M6 18L18 6M6 6l12 12"
              />
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
        class="w-full rounded-md border border-border/70 bg-background px-2 py-1.5 text-xs transition-colors placeholder:text-muted-foreground/50 focus:border-primary focus:ring-1 focus:ring-primary/30 focus:outline-none"
      />
      {#if showTagSuggestions && tagSuggestions.length > 0}
        <div
          class="absolute top-full right-0 left-0 z-50 mt-1 max-h-40 overflow-y-auto rounded-lg border border-border/50 bg-card shadow-lg"
        >
          {#each tagSuggestions as suggestion, i (i)}
            <button
              type="button"
              onmousedown={(e) => {
                e.preventDefault();
                onSuggestionSelect(suggestion);
              }}
              class="w-full px-3 py-1.5 text-left text-xs transition-colors hover:bg-muted/50 {i ===
              selectedSuggestionIndex
                ? 'bg-muted/50 text-foreground'
                : 'text-muted-foreground'}"
            >
              <span class="font-medium text-foreground"
                >{suggestion.slice(0, tagInputValue.trim().length)}</span
              ><span>{suggestion.slice(tagInputValue.trim().length)}</span>
            </button>
          {/each}
        </div>
      {/if}
    </div>
  </div>

  {#if headings.length > 0}
    <div class="mb-2 text-xs font-semibold text-muted-foreground/60 uppercase">目录</div>
    <nav>
      <ul class="space-y-1">
        {#each headings as heading (heading.id)}
          <li>
            <button
              type="button"
              onclick={() => scrollToHeading(heading.id)}
              class="w-full truncate rounded-md px-2 py-1 text-left text-sm text-muted-foreground transition-colors hover:bg-accent/60 hover:text-foreground"
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

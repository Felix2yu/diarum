<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { searchDiaries } from '$lib/api/diaries';
	import { moodToEmoji } from '$lib/utils/diaryEmoji';

	export let title: string = '';
	export let sticky: boolean = true;
	export let showTitle: boolean = true;

	let showSearchOverlay = false;
	let searchQuery = '';
	let searchResults: any[] = [];
	let searchLoading = false;
	let searchTimeout: ReturnType<typeof setTimeout>;
	let searchInput: HTMLInputElement;

	const navItems = [
		{
			href: '/diary',
			label: '日记',
			match: (path: string) => path === '/' || path.startsWith('/diary'),
			svg: `<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" /></svg>`
		},
		{
			href: '/filter',
			label: '筛选',
			match: (path: string) => path.startsWith('/filter'),
			svg: `<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M3 4a1 1 0 011-1h16a1 1 0 011 1v2.586a1 1 0 01-.293.707l-6.414 6.414a1 1 0 00-.293.707V17l-4 4v-6.586a1 1 0 00-.293-.707L3.293 7.293A1 1 0 013 6.586V4z" /></svg>`
		},
		{
			href: '/tags',
			label: '标签',
			match: (path: string) => path.startsWith('/tags'),
			svg: `<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M7 7h.01M7 3h5a1.99 1.99 0 011.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A1.99 1.99 0 013 12V7a4 4 0 014-4z" /></svg>`
		},
		{
			href: '/assistant',
			label: '助手',
			match: (path: string) => path.startsWith('/assistant'),
			svg: `<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" /></svg>`
		},
		{
			href: '/media',
			label: '媒体',
			match: (path: string) => path.startsWith('/media'),
			svg: `<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-4.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" /></svg>`
		},
		{
			href: '/settings',
			label: '设置',
			match: (path: string) => path.startsWith('/settings'),
			svg: `<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" /><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" /></svg>`
		}
	];

	function openSearch() {
		showSearchOverlay = true;
		searchQuery = '';
		searchResults = [];
		setTimeout(() => searchInput?.focus(), 50);
	}

	function closeSearch() {
		showSearchOverlay = false;
		searchQuery = '';
		searchResults = [];
	}

	function handleSearchInput() {
		clearTimeout(searchTimeout);
		if (searchQuery.trim().length < 2) {
			searchResults = [];
			return;
		}
		searchTimeout = setTimeout(async () => {
			searchLoading = true;
			try {
				const data = await searchDiaries(searchQuery.trim());
				searchResults = data.slice(0, 8).map((item: any) => ({
					id: item.id,
					date: item.date?.split(' ')[0] || item.date,
					snippet: (item.snippet || '').replace(/<[^>]*>/g, ' ').replace(/\s+/g, ' ').trim(),
					mood: item.mood || 0
				}));
			} catch {
				searchResults = [];
			} finally {
				searchLoading = false;
			}
		}, 300);
	}

	function goToSearch() {
		if (searchQuery.trim()) {
			goto(`/search?q=${encodeURIComponent(searchQuery.trim())}`);
			closeSearch();
		}
	}

	function goToResult(date: string) {
		goto(`/diary/${date}`);
		closeSearch();
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			closeSearch();
		} else if (e.key === 'Enter' && searchQuery.trim()) {
			goToSearch();
		}
	}
</script>

<header class="glass border-b border-border/60 flex-shrink-0 z-20 safe-top {sticky ? 'sticky top-0' : ''}">
	<div class="container-responsive h-14 relative flex items-center">
		<!-- 左侧：Logo — 编辑性标识 -->
		<div class="flex items-center gap-2 z-10 flex-shrink-0">
			<a href="/" class="flex items-center gap-2 hover:opacity-80 transition-opacity group" title="吾身首页">
				<img src="/logo.png" alt="吾身" class="w-7 h-7" />
				<div class="flex items-baseline gap-2">
					<span class="hidden sm:inline font-serif text-lg font-medium text-foreground group-hover:text-sienna transition-colors">吾身</span>
					<span class="hidden lg:inline font-mono text-[9px] uppercase tracking-widest text-muted-foreground">Diarum</span>
				</div>
			</a>
		</div>

		<!-- 中间：标题 — 编辑性刊头 -->
		{#if showTitle && title}
			<div class="hidden sm:flex absolute inset-0 items-center justify-center px-48 pointer-events-none">
				<div class="flex items-center justify-center gap-3 min-w-0 max-w-full overflow-hidden pointer-events-auto">
					<span class="hidden md:inline h-px w-4 bg-sienna/40"></span>
					<div class="font-serif text-base font-medium text-foreground truncate italic">{title}</div>
					<slot name="subtitle" />
				</div>
			</div>
			<div class="flex-1 min-w-0 flex sm:hidden items-center justify-center px-2">
				<div class="flex items-center justify-center gap-2 min-w-0 max-w-full overflow-hidden">
					<div class="font-serif text-sm font-medium text-foreground truncate">{title}</div>
					<slot name="subtitle" />
				</div>
			</div>
		{:else}
			<div class="flex-1 sm:hidden"></div>
		{/if}

		<!-- 右侧：导航与操作 — 编辑性工具栏 -->
		<div class="ml-auto flex items-center justify-end gap-0.5 z-10 flex-shrink-0">
			{#each navItems as item, i}
				{@const active = item.match($page.url.pathname)}
				<a
					href={item.href}
					class="group relative p-2 transition-all duration-200 {active ? 'text-sienna' : 'text-foreground/60 hover:text-foreground'}"
					title={item.label}
					aria-label={item.label}
					aria-current={active ? 'page' : null}
				>
					{@html item.svg}
					{#if active}
						<span class="absolute -bottom-px left-1/2 -translate-x-1/2 h-px w-5 bg-sienna"></span>
					{/if}
					<!-- 桌面端文字标签 -->
					<span class="hidden xl:inline sr-only">{item.label}</span>
				</a>
			{/each}
			<!-- 分隔线 -->
			<span class="hidden sm:inline-block h-4 w-px bg-border mx-1"></span>
			<!-- 搜索按钮 -->
			<button
				onclick={openSearch}
				class="p-2 rounded-none transition-all duration-200 hover:bg-muted/40 text-foreground/60 hover:text-foreground"
				title="搜索"
				aria-label="搜索"
			>
				<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" /></svg>
			</button>
			<slot name="actions" />
		</div>
	</div>
</header>

<!-- 搜索弹窗 — 编辑性查找 -->
{#if showSearchOverlay}
	<div class="fixed inset-0 z-50 flex items-start justify-center pt-20 animate-fade-in-only">
		<!-- 背景遮罩 -->
		<button
			class="absolute inset-0 bg-background/80 backdrop-blur-sm"
			onclick={closeSearch}
			aria-label="关闭搜索"
		></button>

		<!-- 搜索面板 — 编辑性纸面 -->
		<div class="relative w-full max-w-xl mx-4 bg-card border border-border shadow-[0_20px_60px_-15px_hsl(var(--foreground)/0.2)] animate-slide-in-down">
			<!-- 搜索输入 — 编辑性行内 -->
			<div class="flex items-center gap-4 px-6 py-4 border-b border-border">
				<svg class="w-4 h-4 text-sienna flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
				</svg>
				<input
					bind:this={searchInput}
					bind:value={searchQuery}
					oninput={handleSearchInput}
					onkeydown={handleKeydown}
					type="text"
					placeholder="搜索日记内容…"
					class="flex-1 bg-transparent text-foreground placeholder:text-muted-foreground/60 focus:outline-none font-serif text-lg"
				/>
				{#if searchQuery.length > 0}
				<button
					onclick={() => { searchQuery = ''; searchResults = []; }}
					class="p-1 hover:bg-muted/50 text-muted-foreground hover:text-foreground transition-colors"
					aria-label="清除搜索内容"
					title="清除"
				>
					<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M6 18L18 6M6 6l12 12" />
					</svg>
				</button>
			{/if}
				<kbd class="font-mono text-[10px] uppercase tracking-widest text-muted-foreground border border-border px-1.5 py-0.5">ESC</kbd>
			</div>

			<!-- 搜索结果 — 编辑性条目 -->
			{#if searchLoading}
				<div class="px-6 py-10 text-center">
					<span class="font-mono text-[10px] uppercase tracking-widest text-muted-foreground">搜索中…</span>
				</div>
			{:else if searchResults.length > 0}
				<div class="max-h-80 overflow-y-auto">
					{#each searchResults as result, i}
						<button
							onclick={() => goToResult(result.date)}
							class="w-full text-left px-6 py-3 hover:bg-muted/40 transition-colors border-b border-border/40 last:border-0 group"
						>
							<div class="flex items-baseline gap-3 mb-1">
								<span class="font-mono text-[10px] text-sienna">{String(i + 1).padStart(2, '0')}</span>
								<span class="font-mono text-xs text-foreground">{result.date}</span>
								{#if result.mood}
									<span class="text-xs">{moodToEmoji(result.mood)}</span>
								{/if}
							</div>
							<p class="font-serif text-sm text-muted-foreground line-clamp-1 italic group-hover:text-foreground transition-colors">{result.snippet}</p>
						</button>
					{/each}
				</div>
				<button
					onclick={goToSearch}
					class="group w-full px-6 py-3 text-center border-t border-border hover:bg-muted/30 transition-colors flex items-center justify-center gap-3"
				>
					<span class="font-serif text-sm text-foreground italic">查看全部搜索结果</span>
					<span class="font-mono text-xs text-sienna group-hover:translate-x-1 transition-transform">→</span>
				</button>
			{:else if searchQuery.length >= 2}
				<div class="px-6 py-10 text-center">
					<span class="font-serif italic text-sm text-muted-foreground">未找到匹配的日记</span>
				</div>
			{:else}
				<div class="px-6 py-10 text-center">
					<span class="font-serif italic text-sm text-muted-foreground">输入至少 2 个字符开始搜索</span>
				</div>
			{/if}
		</div>
	</div>
{/if}

<style>
	.line-clamp-1 {
		display: -webkit-box;
		-webkit-line-clamp: 1;
		line-clamp: 1;
		-webkit-box-orient: vertical;
		overflow: hidden;
	}
</style>

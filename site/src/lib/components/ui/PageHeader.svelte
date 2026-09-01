<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { searchDiaries } from '$lib/api/diaries';
	import MoodIcon from '$lib/components/ui/MoodIcon.svelte';

	export let title: string = '';
	export let sticky: boolean = true;
	export let showTitle: boolean = true;

	let showSearchOverlay = false;
	let searchQuery = '';
	let searchResults: any[] = [];
	let searchLoading = false;
	let searchTimeout: ReturnType<typeof setTimeout>;
	let searchInput: HTMLInputElement;
	let mobileMoreOpen = false;
	let moreMenuButton: HTMLButtonElement | undefined;

	// 6 个导航项 + 1 个搜索按钮：大屏全显示，<640px 精简为"日记 + 搜索 + 更多"，
	// 其余 5 个收纳到"更多"抽屉中，避免 header 挤压换行
	const navItems = [
		{
			href: '/diary',
			label: '日记',
			priority: 'always' as 'always' | 'more-only',
			match: (path: string) => path === '/' || path.startsWith('/diary'),
			svg: `<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" /></svg>`
		},
		{
			href: '/filter',
			label: '心情筛选',
			priority: 'more-only',
			match: (path: string) => path.startsWith('/filter'),
			svg: `<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4.318 6.318a4.5 4.5 0 000 6.364L12 20.364l7.682-7.682a4.5 4.5 0 00-6.364-6.364L12 7.636l-1.318-1.318a4.5 4.5 0 00-6.364 0z" /></svg>`
		},
		{
			href: '/tags',
			label: '标签云',
			priority: 'more-only',
			match: (path: string) => path.startsWith('/tags'),
			svg: `<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 7h.01M7 3h5a1.99 1.99 0 011.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A1.99 1.99 0 013 12V7a4 4 0 014-4z" /></svg>`
		},
		{
			href: '/assistant',
			label: 'AI 助手',
			priority: 'more-only',
			match: (path: string) => path.startsWith('/assistant'),
			svg: `<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" /></svg>`
		},
		{
			href: '/media',
			label: '媒体库',
			priority: 'more-only',
			match: (path: string) => path.startsWith('/media'),
			svg: `<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-4.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" /></svg>`
		},
		{
			href: '/settings',
			label: '设置',
			priority: 'more-only',
			match: (path: string) => path.startsWith('/settings'),
			svg: `<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" /><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" /></svg>`
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

	function toggleMobileMore(e: Event) {
		// 让点击更多按钮不会立即触发 document 点击关闭
		e.stopPropagation();
		mobileMoreOpen = !mobileMoreOpen;
	}

	function closeMobileMore() {
		mobileMoreOpen = false;
	}
</script>

<svelte:window onclick={closeMobileMore} />

<header class="glass border-b border-border/50 flex-shrink-0 z-20 safe-top {sticky ? 'sticky top-0' : ''}">
	<div class="container-responsive h-14 relative flex items-center">
		<!-- 左侧：Logo -->
		<div class="flex items-center gap-2 z-10 flex-shrink-0">
			<a href="/" class="flex items-center gap-2 hover:opacity-80 transition-opacity" title="吾身首页">
				<img src="/logo.png" alt="吾身" class="w-7 h-7" />
				<span class="hidden sm:inline text-lg font-semibold text-foreground hover:text-primary transition-colors">吾身</span>
			</a>
		</div>

		<!-- 中间：标题 -->
		{#if showTitle && title}
			<div class="hidden sm:flex absolute inset-0 items-center justify-center px-48 pointer-events-none">
				<div class="flex items-center justify-center gap-2 min-w-0 max-w-full overflow-hidden pointer-events-auto">
					<div class="text-sm font-medium text-foreground truncate">{title}</div>
					<slot name="subtitle" />
				</div>
			</div>
			<div class="flex-1 min-w-0 flex sm:hidden items-center justify-center px-2">
				<div class="flex items-center justify-center gap-2 min-w-0 max-w-full overflow-hidden">
					<div class="text-sm font-medium text-foreground truncate">{title}</div>
					<slot name="subtitle" />
				</div>
			</div>
		{:else}
			<div class="flex-1 sm:hidden" />
		{/if}

		<!-- 右侧：导航图标与操作 -->
		<div class="ml-auto flex items-center justify-end gap-1 z-10 flex-shrink-0">
			{#each navItems as item}
				{@const active = item.match($page.url.pathname)}
				<a
					href={item.href}
					class="p-2 rounded-lg transition-colors duration-200
						   {item.priority === 'more-only' ? 'hidden sm:inline-flex' : 'inline-flex'}
						   {active
						? 'bg-primary/15 text-primary ring-1 ring-primary/30 shadow-sm'
						: 'hover:bg-muted/50 text-foreground/70 hover:text-foreground'}"
					title={item.label}
					aria-label={item.label}
					aria-current={active ? 'page' : null}
				>
					{@html item.svg}
				</a>
			{/each}

			<!-- 搜索按钮 -->
			<button
				onclick={openSearch}
				class="p-2 rounded-lg transition-colors duration-200 hover:bg-muted/50 text-foreground/70 hover:text-foreground"
				title="搜索"
				aria-label="搜索"
				type="button"
			>
				<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" /></svg>
			</button>

			<!-- 移动端"更多"按钮：sm 断点以下显示，收纳其余 5 个导航项到抽屉 -->
			<div class="sm:hidden relative">
				<button
					bind:this={moreMenuButton}
					type="button"
					onclick={toggleMobileMore}
					onkeydown={(e) => e.key === 'Escape' && closeMobileMore()}
					class="p-2 rounded-lg transition-colors duration-200 hover:bg-muted/50 text-foreground/70 hover:text-foreground"
					aria-haspopup="menu"
					aria-expanded={mobileMoreOpen}
					aria-controls="mobile-more-menu"
					aria-label="更多导航项"
				>
					<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16" />
					</svg>
				</button>

				{#if mobileMoreOpen}
					<div
						id="mobile-more-menu"
						role="menu"
						aria-label="站点导航"
						class="absolute right-0 top-full mt-2 w-52 origin-top-right bg-card border border-border/60 rounded-xl shadow-2xl py-1.5 overflow-hidden animate-slide-down z-50"
						onclick={(e) => e.stopPropagation()}
					>
						{#each navItems.filter(it => it.priority === 'more-only') as item}
							{@const active = item.match($page.url.pathname)}
							<a
								role="menuitem"
								href={item.href}
								onclick={closeMobileMore}
								class="flex items-center gap-3 px-3 py-2.5 text-sm transition-colors {active
									? 'bg-primary/10 text-primary'
									: 'text-foreground/80 hover:bg-muted/50 hover:text-foreground'}"
								aria-current={active ? 'page' : null}
							>
								<span class="text-foreground/70">{@html item.svg}</span>
								<span class="font-medium">{item.label}</span>
							</a>
						{/each}
					</div>
				{/if}
			</div>

			<slot name="actions" />
		</div>
	</div>
</header>

<!-- 搜索弹窗 -->
{#if showSearchOverlay}
	<div
		role="dialog"
		aria-modal="true"
		aria-labelledby="search-dialog-title"
		class="fixed inset-0 z-50 flex items-start justify-center pt-16 animate-fade-in-only"
		onkeydown={handleKeydown}
	>
		<!-- 背景遮罩 -->
		<button
			type="button"
			class="absolute inset-0 bg-black/40 backdrop-blur-sm"
			onclick={closeSearch}
			aria-label="关闭搜索"
		></button>

		<!-- 搜索面板 -->
		<div class="relative w-full max-w-lg mx-4 bg-card rounded-xl shadow-2xl border border-border/50 overflow-hidden animate-slide-down">
			<!-- 搜索输入 -->
			<div class="flex items-center gap-3 px-4 py-3 border-b border-border/50">
				<svg class="w-5 h-5 text-muted-foreground flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
				</svg>
				<div class="sr-only" id="search-dialog-title">搜索日记</div>
				<input
					bind:this={searchInput}
					bind:value={searchQuery}
					oninput={handleSearchInput}
					onkeydown={handleKeydown}
					type="search"
					aria-label="搜索日记内容"
					placeholder="搜索日记内容…"
					class="flex-1 bg-transparent text-foreground placeholder:text-muted-foreground focus:outline-none text-sm"
				/>
				{#if searchQuery.length > 0}
					<button
						type="button"
						onclick={() => {
							searchQuery = '';
							searchResults = [];
						}}
						class="p-1 rounded hover:bg-muted/50 text-muted-foreground hover:text-foreground transition-colors"
						aria-label="清空搜索内容"
					>
						<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
						</svg>
					</button>
				{/if}
				<button
					type="button"
					onclick={closeSearch}
					class="text-xs text-muted-foreground hover:text-foreground transition-colors"
					aria-label="按 ESC 关闭搜索"
				>
					ESC
				</button>
			</div>

			<!-- 搜索结果 -->
			{#if searchLoading}
				<div class="px-4 py-6 text-center text-sm text-muted-foreground">搜索中…</div>
			{:else if searchResults.length > 0}
				<div class="max-h-80 overflow-y-auto" role="listbox" aria-label="搜索结果">
					{#each searchResults as result}
						<button
							type="button"
							role="option"
							onclick={() => goToResult(result.date)}
							class="w-full text-left px-4 py-3 hover:bg-muted/50 transition-colors border-b border-border/30 last:border-0"
						>
							<div class="flex items-center gap-2 mb-1">
								<span class="text-xs font-medium text-foreground">{result.date}</span>
								{#if result.mood}
									<span class="text-xs"><MoodIcon mood={result.mood} size={14} /></span>
								{/if}
							</div>
							<p class="text-xs text-muted-foreground line-clamp-1">{result.snippet}</p>
						</button>
					{/each}
				</div>
				<button
					type="button"
					onclick={goToSearch}
					class="w-full px-4 py-3 text-center text-sm text-primary hover:bg-muted/30 transition-colors border-t border-border/50"
				>
					查看全部搜索结果 →
				</button>
			{:else if searchQuery.length >= 2}
				<div class="px-4 py-6 text-center text-sm text-muted-foreground" role="status">
					未找到匹配的日记
				</div>
			{:else}
				<div class="px-4 py-6 text-center text-sm text-muted-foreground" role="status">
					输入至少 2 个字符开始搜索
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

	/* sm (640px) 以下进入窄屏模式：仅展示 always 优先级的导航 + 更多菜单 */
	:global(.isNarrowHeader) :global([data-nav-priority='more-only']) {
		display: none;
	}
</style>

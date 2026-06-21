<script lang="ts">
	import { page } from '$app/stores';

	export let title: string = '';
	export let sticky: boolean = true;
	export let showTitle: boolean = true;

	const navItems = [
		{
			href: '/diary',
			label: '日记',
			match: (path: string) => path === '/' || path.startsWith('/diary'),
			svg: `<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" /></svg>`
		},
		{
			href: '/search',
			label: '搜索',
			match: (path: string) => path.startsWith('/search'),
			svg: `<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" /></svg>`
		},
		{
			href: '/tags',
			label: '标签云',
			match: (path: string) => path.startsWith('/tags'),
			svg: `<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 7h.01M7 3h5a1.99 1.99 0 011.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A1.99 1.99 0 013 12V7a4 4 0 014-4z" /></svg>`
		},
		{
			href: '/assistant',
			label: 'AI 助手',
			match: (path: string) => path.startsWith('/assistant'),
			svg: `<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" /></svg>`
		},
		{
			href: '/media',
			label: '媒体库',
			match: (path: string) => path.startsWith('/media'),
			svg: `<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-4.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" /></svg>`
		},
		{
			href: '/settings',
			label: '设置',
			match: (path: string) => path.startsWith('/settings'),
			svg: `<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" /><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" /></svg>`
		}
	];
</script>

<header class="glass border-b border-border/50 flex-shrink-0 z-20 safe-top {sticky ? 'sticky top-0' : ''}">
	<div class="container-responsive h-14 relative flex items-center">
		<!-- 左侧：Logo -->
		<div class="flex items-center gap-2 z-10 flex-shrink-0">
			<a href="/" class="flex items-center gap-2 hover:opacity-80 transition-opacity" title="吾身首页">
				<img src="/logo.png" alt="吾身" class="w-7 h-7" />
				<span class="hidden sm:inline text-lg font-semibold text-foreground hover:text-primary transition-colors">吾身</span>
			</a>
		</div>

		<!-- 中间：标题
		     · 桌面端（sm 及以上）：绝对定位，在整个 header 宽度上居中显示
		     · 手机端（sm 以下）：在剩余空间内三栏 flex，避免与两侧导航图标重叠 -->
		{#if showTitle && title}
			<!-- 桌面：绝对居中 -->
			<div class="hidden sm:flex absolute inset-0 items-center justify-center px-48 pointer-events-none">
				<div class="flex items-center justify-center gap-2 min-w-0 max-w-full overflow-hidden pointer-events-auto">
					<div class="text-sm font-medium text-foreground truncate">{title}</div>
					<slot name="subtitle" />
				</div>
			</div>
			<!-- 手机：flex 占剩余空间，在剩余空间内居中 -->
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
					class="p-2 rounded-lg transition-all duration-200 {active ? 'bg-primary/15 text-primary ring-1 ring-primary/30 shadow-sm' : 'hover:bg-muted/50 text-foreground/70 hover:text-foreground'}"
					title={item.label}
					aria-label={item.label}
					aria-current={active ? 'page' : null}
				>
					{@html item.svg}
				</a>
			{/each}
			<slot name="actions" />
		</div>
	</div>
</header>

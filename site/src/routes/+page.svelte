<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { getToday } from '$lib/utils/date';
	import { isAuthenticated } from '$lib/api/client';
	import { getGeneralSettings } from '$lib/api/settings';

	let ready = $state(false);
	let visibleSections = $state<Set<number>>(new Set());
	let mousePos = $state({ x: 0.5, y: 0.5 });

	onMount(() => {
		if ($isAuthenticated) {
			getGeneralSettings()
				.then((s) => goto(s.default_view === 'calendar' ? '/diary' : `/diary/${getToday()}`))
				.catch(() => goto(`/diary/${getToday()}`))
				.catch(() => { ready = true; });
		} else {
			ready = true;
		}

		const observer = new IntersectionObserver(
			(entries) => {
				entries.forEach((entry) => {
					if (entry.isIntersecting) {
						const idx = parseInt(entry.target.getAttribute('data-idx') || '0');
						visibleSections = new Set([...visibleSections, idx]);
					}
				});
			},
			{ threshold: 0.1, rootMargin: '0px 0px -50px 0px' }
		);

		setTimeout(() => {
			document.querySelectorAll('[data-observe]').forEach((el) => observer.observe(el));
		}, 100);

		const onMove = (e: MouseEvent) => {
			mousePos = {
				x: e.clientX / window.innerWidth,
				y: e.clientY / window.innerHeight
			};
		};
		window.addEventListener('mousemove', onMove);

		return () => {
			observer.disconnect();
			window.removeEventListener('mousemove', onMove);
		};
	});

	const features = [
		{
			num: '01',
			title: '日常记录',
			description: '使用富文本编辑器捕捉你的思绪。支持格式化、列表、引用块——一切都为流畅书写而设计。',
			tag: 'EDITOR'
		},
		{
			num: '02',
			title: 'AI 助手',
			description: '与一位读过你所有日记的智能助手对话。发现模式，提出问题，让反思有迹可循。',
			tag: 'INTELLIGENCE'
		},
		{
			num: '03',
			title: '日历视图',
			description: '直观的日历浏览。一眼看到写作连续天数、活动节奏与岁月的纹路。',
			tag: 'CHRONOLOGY'
		},
		{
			num: '04',
			title: '强大搜索',
			description: '全文检索所有记录。让过去的自己，随时可被回访。',
			tag: 'RECALL'
		},
		{
			num: '05',
			title: '媒体库',
			description: '将照片和图片附加到记录中，构建属于你生活瞬间的视觉时间线。',
			tag: 'ARCHIVE'
		},
		{
			num: '06',
			title: '深色模式',
			description: '白昼与深夜，两种纸面。在浅色与深色主题之间无缝切换。',
			tag: 'APPEARANCE'
		}
	];
</script>

{#if !ready}
	<div class="flex items-center justify-center min-h-screen">
		<p class="text-muted-foreground font-mono text-sm tracking-wider">LOADING…</p>
	</div>
{:else}
	<div class="min-h-screen min-h-[100dvh] flex flex-col bg-background relative overflow-hidden">
		<!-- 背景装饰：移动的渐变光晕 -->
		<div
			class="pointer-events-none fixed inset-0 z-0 opacity-60 transition-all duration-1000"
			style={`background: radial-gradient(circle at ${mousePos.x * 100}% ${mousePos.y * 100}%, hsl(var(--sienna) / 0.08) 0%, transparent 40%);`}
		></div>

		<!-- Navigation — 极简编辑性导航 -->
		<nav class="fixed top-0 left-0 right-0 z-50 glass border-b border-border/40 safe-top">
			<div class="container-responsive">
				<div class="flex items-center justify-between h-14">
					<div class="flex items-center gap-3">
						<img src="/logo.png" alt="吾身" class="w-7 h-7" />
						<div class="flex items-baseline gap-2">
							<span class="font-serif text-xl font-medium text-foreground">吾身</span>
							<span class="hidden sm:inline font-mono text-[10px] uppercase tracking-widest text-muted-foreground">Diarum</span>
						</div>
					</div>
					<div class="flex items-center gap-6">
						<a
							href="#features"
							class="hidden sm:inline text-xs font-mono uppercase tracking-wider text-muted-foreground hover:text-foreground transition-colors"
						>
							目录
						</a>
						<a
							href="#preview"
							class="hidden sm:inline text-xs font-mono uppercase tracking-wider text-muted-foreground hover:text-foreground transition-colors"
						>
							预览
						</a>
						<a
							href="/login"
							class="text-xs font-mono uppercase tracking-wider text-foreground hover:text-sienna transition-colors relative group"
						>
							登录
							<span class="absolute -bottom-0.5 left-0 right-0 h-px bg-sienna scale-x-0 group-hover:scale-x-100 transition-transform origin-left"></span>
						</a>
					</div>
				</div>
			</div>
		</nav>

		<!-- Hero — 编辑性头版 -->
		<section class="relative pt-32 pb-24 container-responsive z-10">
			<div class="max-w-6xl mx-auto">
				<!-- 期刊式标记 -->
				<div class="flex items-center gap-4 mb-10 animate-fade-in-only">
					<span class="editorial-stamp">VOL. I</span>
					<span class="font-mono text-[10px] uppercase tracking-widest text-muted-foreground">
						每日反思 · 自托管 · AI 增强
					</span>
					<span class="flex-1 h-px bg-border"></span>
					<span class="font-mono text-[10px] uppercase tracking-widest text-muted-foreground hidden sm:inline">
						吾日三省吾身
					</span>
				</div>

				<!-- 巨型标题 — 不对称编辑性布局 -->
				<div class="grid lg:grid-cols-12 gap-8 items-end">
					<div class="lg:col-span-8">
						<h1 class="font-serif font-medium text-foreground leading-[0.95] tracking-tight animate-editorial-rise"
							style="font-size: clamp(3rem, 8vw, 7rem);"
						>
							你的个人空间，<br/>
							用于<span class="italic text-sienna font-normal">每日</span><br/>
							<span class="ink-underline ink-underline-animate">反思</span>。
						</h1>
					</div>
					<div class="lg:col-span-4 lg:pb-4 animate-fade-in" style="animation-delay: 0.4s;">
						<p class="font-serif text-lg sm:text-xl text-foreground/80 leading-relaxed mb-6 italic">
							记录你的思绪，追踪你的成长，借助 AI 驱动的洞察获得反思。
						</p>
						<p class="text-sm text-muted-foreground leading-relaxed mb-8">
							一本美丽私密的日记，与你共同成长。极简、私有、跨平台。
						</p>
						<div class="flex flex-col gap-3">
							<a
								href="/login"
								class="group inline-flex items-center justify-between gap-3 px-6 py-3.5 bg-foreground text-background hover:bg-sienna transition-colors duration-300"
							>
								<span class="font-serif text-base font-medium">立即开始写作</span>
								<span class="font-mono text-xs uppercase tracking-wider opacity-70 group-hover:translate-x-1 transition-transform">→</span>
							</a>
							<a
								href="#features"
								class="inline-flex items-center justify-between gap-3 px-6 py-3.5 border border-border text-foreground hover:border-foreground transition-colors duration-300"
							>
								<span class="font-serif text-base font-medium">了解更多</span>
								<span class="font-mono text-xs uppercase tracking-wider opacity-70">↓</span>
							</a>
						</div>
					</div>
				</div>

				<!-- 编辑性脚注 -->
				<div class="mt-20 flex flex-wrap items-center justify-between gap-4 pt-6 border-t border-border">
					<div class="flex items-center gap-8">
						<div>
							<div class="font-mono text-[10px] uppercase tracking-widest text-muted-foreground mb-1">哲学</div>
							<div class="font-serif text-sm text-foreground">一天一篇，打开即写</div>
						</div>
						<div class="hidden sm:block">
							<div class="font-mono text-[10px] uppercase tracking-widest text-muted-foreground mb-1">许可</div>
							<div class="font-serif text-sm text-foreground">Apache 2.0 · 开源</div>
						</div>
						<div class="hidden md:block">
							<div class="font-mono text-[10px] uppercase tracking-widest text-muted-foreground mb-1">部署</div>
							<div class="font-serif text-sm text-foreground">自托管 · Docker</div>
						</div>
					</div>
					<div class="font-mono text-[10px] uppercase tracking-widest text-muted-foreground">
						↓ 向下滚动
					</div>
				</div>
			</div>
		</section>

		<!-- Screenshots — 编辑性版式 -->
		<section id="preview" class="py-24 relative z-10 border-t border-border" data-observe data-idx="0">
			<div class="container-responsive">
				<!-- 章节标头 — 杂志风格 -->
				<div class="flex items-end justify-between mb-12 flex-wrap gap-4"
					class:animate-fade-in={visibleSections.has(0)}
					style:opacity={visibleSections.has(0) ? 1 : 0}
				>
					<div>
						<div class="font-mono text-[10px] uppercase tracking-widest text-sienna mb-3">§ 01 · 预览</div>
						<h2 class="font-serif text-4xl sm:text-5xl font-medium text-foreground tracking-tight">
							所见即所写
						</h2>
					</div>
					<p class="font-serif italic text-muted-foreground max-w-md text-lg">
						干净、无干扰的写作体验。在所有设备上保持一致。
					</p>
				</div>

				<!-- Desktop Screenshots -->
				<div class="hidden md:block mb-16"
					class:animate-fade-in={visibleSections.has(0)}
					style:opacity={visibleSections.has(0) ? 1 : 0}
				>
					<div class="relative overflow-hidden border border-border shadow-[0_30px_60px_-15px_hsl(var(--foreground)/0.15)] group">
						<img
							src="/screenshots/desktop-light.png"
							alt="吾身桌面界面"
							class="w-full h-auto dark:hidden transition-transform duration-700 group-hover:scale-[1.02]"
							loading="lazy"
						/>
						<img
							src="/screenshots/desktop-dark.png"
							alt="吾身桌面界面"
							class="w-full h-auto hidden dark:block transition-transform duration-700 group-hover:scale-[1.02]"
							loading="lazy"
						/>
						<!-- 编辑性角标 -->
						<div class="absolute top-4 left-4 editorial-stamp bg-background/80 backdrop-blur-sm">
							DESKTOP · 01
						</div>
					</div>
				</div>

				<!-- Mobile Screenshots -->
				<div class="md:hidden mb-12"
					class:animate-fade-in={visibleSections.has(0)}
					style:opacity={visibleSections.has(0) ? 1 : 0}
				>
					<div class="relative overflow-hidden border border-border max-w-sm mx-auto shadow-[0_20px_40px_-10px_hsl(var(--foreground)/0.15)]">
						<img
							src="/screenshots/mobile-light.png"
							alt="吾身移动界面"
							class="w-full h-auto dark:hidden"
							loading="lazy"
						/>
						<img
							src="/screenshots/mobile-dark.png"
							alt="吾身移动界面"
							class="w-full h-auto hidden dark:block"
							loading="lazy"
						/>
						<div class="absolute top-3 left-3 editorial-stamp bg-background/80 backdrop-blur-sm">
							MOBILE · 01
						</div>
					</div>
				</div>

				<!-- Feature Highlights — 编辑性三栏 -->
				<div class="grid grid-cols-1 md:grid-cols-3 gap-px bg-border border border-border">
					{#each [
						{ num: 'i.', title: '美观的编辑器', desc: '富文本格式化，界面直观无干扰。每一次书写，都像在精致的纸面上落笔。' },
						{ num: 'ii.', title: '智能日历', desc: '追踪你的写作连续天数。让习惯，被时间看见。' },
						{ num: 'iii.', title: '响应式设计', desc: '桌面、平板、移动设备——三种尺寸，同一种专注。' }
					] as hl, i}
						<div
							class="bg-background p-8 hover:bg-card transition-colors duration-300"
							class:animate-fade-in={visibleSections.has(0)}
							style:opacity={visibleSections.has(0) ? 1 : 0}
							style:animation-delay="{(i + 1) * 100}ms"
						>
							<div class="font-serif italic text-2xl text-sienna mb-4">{hl.num}</div>
							<h3 class="font-serif text-xl font-medium text-foreground mb-3">{hl.title}</h3>
							<p class="text-sm text-muted-foreground leading-relaxed">{hl.desc}</p>
						</div>
					{/each}
				</div>
			</div>
		</section>

		<!-- Features Section — 编辑性目录 -->
		<section id="features" class="py-24 container-responsive relative z-10 border-t border-border" data-observe data-idx="1">
			<div class="w-full">
				<div class="grid lg:grid-cols-12 gap-8 mb-16"
					class:animate-fade-in={visibleSections.has(1)}
					style:opacity={visibleSections.has(1) ? 1 : 0}
				>
					<div class="lg:col-span-4">
						<div class="font-mono text-[10px] uppercase tracking-widest text-sienna mb-3">§ 02 · 目录</div>
						<h2 class="font-serif text-4xl sm:text-5xl font-medium text-foreground tracking-tight leading-[1.05]">
							写日记所需<br/>的一切。
						</h2>
					</div>
					<div class="lg:col-span-7 lg:col-start-6 lg:pt-4">
						<p class="editorial-lead font-serif text-xl text-foreground/85 leading-relaxed">
							<span class="editorial-drop-cap">强</span>大的功能设计，让每日写作轻松而有意义。
							每一项都为长期记录与深度反思而打造——不是堆砌，而是恰到好处。
						</p>
					</div>
				</div>

				<!-- Features Grid — 编辑性条目 -->
				<div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-px bg-border border border-border">
					{#each features as feature, i}
						<article
							class="bg-background p-8 hover:bg-card transition-colors duration-300 group relative"
							class:animate-fade-in={visibleSections.has(1)}
							style:opacity={visibleSections.has(1) ? 1 : 0}
							style:animation-delay="{i * 60}ms"
						>
							<div class="flex items-start justify-between mb-6">
								<span class="font-mono text-xs text-sienna tracking-wider">{feature.num}</span>
								<span class="font-mono text-[9px] uppercase tracking-widest text-muted-foreground/70">{feature.tag}</span>
							</div>
							<h3 class="font-serif text-2xl font-medium text-foreground mb-3 group-hover:text-sienna transition-colors">
								{feature.title}
							</h3>
							<p class="text-sm text-muted-foreground leading-relaxed">{feature.description}</p>
							<!-- 底部装饰线 -->
							<div class="mt-6 h-px bg-border group-hover:bg-sienna/40 transition-colors"></div>
						</article>
					{/each}
				</div>
			</div>
		</section>

		<!-- AI Assistant Preview — 编辑性双栏 -->
		<section class="py-24 relative z-10 border-t border-border bg-muted/20" data-observe data-idx="2">
			<div class="container-responsive">
				<div class="grid lg:grid-cols-12 gap-12 items-start">
					<!-- 左侧：标题与说明 -->
					<div class="lg:col-span-5"
						class:animate-slide-up={visibleSections.has(2)}
						style:opacity={visibleSections.has(2) ? 1 : 0}
					>
						<div class="font-mono text-[10px] uppercase tracking-widest text-sienna mb-3">§ 03 · 智能</div>
						<h2 class="font-serif text-4xl sm:text-5xl font-medium text-foreground mb-6 leading-[1.05] tracking-tight">
							你的 AI<br/>
							<span class="italic text-sienna font-normal">反思伙伴</span>
						</h2>
						<p class="editorial-lead font-serif text-lg text-foreground/85 leading-relaxed mb-8">
							吾身的智能助手会阅读你的日记记录，帮助你发现模式、获得洞察，
							并反思个人成长旅程。它读你写过的每一个字。
						</p>
						<ul class="space-y-3">
							{#each [
								'询问过去记录的相关问题',
								'获取个性化写作提示',
								'发现情绪模式与趋势',
								'私密且安全的对话'
							] as item, i}
								<li class="flex items-baseline gap-4 group">
									<span class="font-mono text-xs text-sienna">{String(i + 1).padStart(2, '0')}</span>
									<span class="font-serif text-base text-foreground group-hover:text-sienna transition-colors">{item}</span>
								</li>
							{/each}
						</ul>
					</div>

					<!-- 右侧：对话预览 — 编辑性卡片 -->
					<div class="lg:col-span-6 lg:col-start-7 relative"
						class:animate-slide-in-right={visibleSections.has(2)}
						style:opacity={visibleSections.has(2) ? 1 : 0}
					>
						<!-- 装饰：纸张折角 -->
						<div class="absolute -top-4 -right-4 w-20 h-20 border-t border-r border-sienna/40 hidden lg:block"></div>
						<div class="absolute -bottom-4 -left-4 w-20 h-20 border-b border-l border-sienna/40 hidden lg:block"></div>

						<div class="bg-card border border-border relative">
							<!-- 卡片头部 — 编辑性标头 -->
							<div class="flex items-center justify-between px-5 py-3 border-b border-border bg-muted/30">
								<div class="flex items-center gap-2">
									<span class="w-2 h-2 rounded-full bg-sienna"></span>
									<span class="font-mono text-[10px] uppercase tracking-widest text-muted-foreground">AI 助手 · 对话</span>
								</div>
								<span class="font-mono text-[10px] text-muted-foreground">— LIVE</span>
							</div>

							<!-- 对话内容 -->
							<div class="p-6 space-y-5 min-h-[320px]">
								<div class="flex gap-4">
									<div class="w-8 h-8 rounded-full bg-sienna/10 border border-sienna/30 flex items-center justify-center text-xs shrink-0 font-serif italic text-sienna">A</div>
									<div class="flex-1 bg-muted/40 border-l-2 border-sienna/40 px-4 py-3">
										<p class="font-serif text-sm text-foreground italic leading-relaxed">基于你最近的记录，我注意到本周你感觉更加精力充沛。是否愿意探索一下可能带来这一积极变化的因素？</p>
									</div>
								</div>
								<div class="flex gap-4 justify-end">
									<div class="bg-foreground text-background px-4 py-3 max-w-[80%]">
										<p class="text-sm">是的，我很想更好地了解这一点！</p>
									</div>
								</div>
								<div class="flex gap-4">
									<div class="w-8 h-8 rounded-full bg-sienna/10 border border-sienna/30 flex items-center justify-center text-xs shrink-0 font-serif italic text-sienna">A</div>
									<div class="flex-1 bg-muted/40 border-l-2 border-sienna/40 px-4 py-3">
										<p class="font-serif text-sm text-foreground italic leading-relaxed">回顾你过去两周的记录，我发现你开始了晨间例行活动，并且在锻炼方面更加一致……</p>
									</div>
								</div>
							</div>

							<!-- 卡片底部 -->
							<div class="px-5 py-3 border-t border-border bg-muted/30 flex items-center justify-between">
								<span class="font-mono text-[10px] uppercase tracking-widest text-muted-foreground">RAG · 私有向量检索</span>
								<span class="font-mono text-[10px] text-muted-foreground">v1.12</span>
							</div>
						</div>
					</div>
				</div>
			</div>
		</section>

		<!-- Quote / Manifesto — 编辑性大段引言 -->
		<section class="py-32 container-responsive relative z-10 border-t border-border" data-observe data-idx="3">
			<div class="max-w-4xl mx-auto text-center"
				class:animate-fade-in={visibleSections.has(3)}
				style:opacity={visibleSections.has(3) ? 1 : 0}
			>
				<div class="font-serif text-6xl text-sienna mb-6 leading-none">"</div>
				<blockquote class="font-serif text-3xl sm:text-4xl lg:text-5xl font-medium text-foreground leading-[1.2] tracking-tight italic">
					一天一篇，<br/>
					打开即写，<br/>
					<span class="not-italic text-sienna">刚刚好。</span>
				</blockquote>
				<div class="mt-8 flex items-center justify-center gap-4">
					<span class="h-px w-12 bg-border"></span>
					<span class="font-mono text-[10px] uppercase tracking-widest text-muted-foreground">吾身 · DIARUM</span>
					<span class="h-px w-12 bg-border"></span>
				</div>
			</div>
		</section>

		<!-- CTA Section — 最终召唤 -->
		<section class="py-24 container-responsive relative z-10 border-t border-border" data-observe data-idx="4">
			<div class="max-w-3xl mx-auto text-center"
				class:animate-fade-in={visibleSections.has(4)}
				style:opacity={visibleSections.has(4) ? 1 : 0}
			>
				<div class="font-mono text-[10px] uppercase tracking-widest text-sienna mb-4">§ FIN · 终章</div>
				<h2 class="font-serif text-4xl sm:text-5xl lg:text-6xl font-medium text-foreground mb-8 leading-[1.05] tracking-tight">
					立即开始你的<br/>
					<span class="italic text-sienna">日记旅程</span>。
				</h2>
				<p class="font-serif text-lg text-muted-foreground mb-10 italic max-w-xl mx-auto">
					加入成千上万使用「吾身」记录日常想法并通过反思成长的人们。
				</p>
				<a
					href="/login"
					class="group inline-flex items-center gap-4 px-10 py-4 bg-foreground text-background hover:bg-sienna transition-colors duration-300"
				>
					<span class="font-serif text-lg font-medium">创建你的免费账户</span>
					<span class="font-mono text-xs uppercase tracking-wider opacity-70 group-hover:translate-x-2 transition-transform">→</span>
				</a>
				<p class="mt-6 font-mono text-[10px] uppercase tracking-widest text-muted-foreground">
					无需信用卡 · 数据始终私密
				</p>
			</div>
		</section>

		<!-- Footer — 编辑性页脚 -->
		<footer class="border-t border-border py-12 relative z-10 bg-background">
			<div class="container-responsive">
				<div class="flex flex-col md:flex-row items-start md:items-center justify-between gap-6">
					<div class="flex items-center gap-3">
						<img src="/logo.png" alt="吾身" class="w-6 h-6" />
						<div>
							<div class="font-serif text-base font-medium text-foreground">吾身 · Diarum</div>
							<div class="font-mono text-[10px] uppercase tracking-widest text-muted-foreground">自托管 · 开源 · Apache 2.0</div>
						</div>
					</div>
					<div class="font-mono text-[10px] uppercase tracking-widest text-muted-foreground">
						© {new Date().getFullYear()} · 一天一篇，打开即写
					</div>
				</div>
			</div>
		</footer>
	</div>
{/if}

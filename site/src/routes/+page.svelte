<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { getToday } from '$lib/utils/date';
	import { isAuthenticated } from '$lib/api/client';
	import Footer from '$lib/components/ui/Footer.svelte';

	let ready = $state(false);

	onMount(() => {
		if ($isAuthenticated) {
			goto(`/diary/${getToday()}`).catch(() => {
				ready = true;
			});
		} else {
			ready = true;
		}
	});

	const features = [
		{
			icon: '📝',
			title: '日常记录',
			description: '使用美观的富文本编辑器记录并整理你的思绪，支持文本格式化、列表等功能。'
		},
		{
			icon: '🤖',
			title: 'AI 助手',
			description: '与一位能理解你日记内容的智能助手对话，帮助你反思自己的旅程。'
		},
		{
			icon: '📅',
			title: '日历视图',
			description: '通过直观的日历浏览你的记录，一目了然地查看写作连续天数和活动。'
		},
		{
			icon: '🔍',
			title: '强大搜索',
			description: '即时找到任何记忆，通过全文搜索能力检索所有记录。'
		},
		{
			icon: '🖼️',
			title: '媒体库',
			description: '将照片和图片附加到记录中，构建你生活瞬间的视觉时间线。'
		},
		{
			icon: '🌙',
			title: '深色模式',
			description: '无论白天夜晚都不刺眼，可在浅色与深色主题之间无缝切换。'
		}
	];
</script>

{#if !ready}
	<div class="flex items-center justify-center min-h-screen">
		<p class="text-muted-foreground">加载中...</p>
	</div>
{:else}
	<div class="min-h-screen flex flex-col bg-background">
		<!-- Navigation -->
		<nav class="fixed top-0 left-0 right-0 z-50 glass border-b border-border/50">
			<div class="container-responsive h-16">
				<div class="flex items-center justify-between h-full">
					<div class="flex items-center gap-3">
						<img src="/logo.png" alt="吾身" class="w-10 h-10" />
						<span class="text-2xl font-bold text-foreground">吾身</span>
					</div>
					<a
						href="/login"
						class="px-5 py-2 text-sm font-medium text-foreground hover:text-primary transition-colors"
					>
						登录
					</a>
				</div>
			</div>
		</nav>

		<!-- Hero Section -->
		<section class="pt-32 pb-20 container-responsive">
			<div class="max-w-4xl mx-auto text-center animate-fade-in">
				<h1 class="text-4xl sm:text-5xl lg:text-6xl font-bold text-foreground mb-6 leading-tight">
					你的个人空间，用于
					<span class="text-primary">每日反思</span>
				</h1>
				<p class="text-lg sm:text-xl text-muted-foreground mb-8 max-w-2xl mx-auto">
					记录你的思绪，追踪你的成长，借助 AI 驱动的日记功能获得深入洞察。
					一本美丽私密的日记，与你共同成长。
				</p>
				<div class="flex flex-col sm:flex-row items-center justify-center gap-4">
					<a
						href="/login"
						class="w-full sm:w-auto px-8 py-3 text-lg font-medium bg-primary text-primary-foreground rounded-xl hover:opacity-90 transition-all shadow-lg hover:shadow-xl"
					>
						立即开始写作
					</a>
					<a
						href="#features"
						class="w-full sm:w-auto px-8 py-3 text-lg font-medium text-foreground border border-border rounded-xl hover:bg-accent transition-all"
					>
						了解更多
					</a>
				</div>
			</div>
		</section>

		<!-- Screenshots Section -->
		<section class="py-16 bg-muted/30">
			<div class="container-responsive">
				<!-- Desktop Screenshots -->
				<div class="hidden md:block mb-12 animate-fade-in">
					<div class="relative rounded-2xl overflow-hidden shadow-2xl border border-border/50">
						<!-- Light Mode Screenshot -->
						<img
							src="/screenshots/desktop-light.png"
							alt="吾身桌面界面"
							class="w-full h-auto dark:hidden"
							loading="lazy"
						/>
						<!-- Dark Mode Screenshot -->
						<img
							src="/screenshots/desktop-dark.png"
							alt="吾身桌面界面"
							class="w-full h-auto hidden dark:block"
							loading="lazy"
						/>
					</div>
				</div>

				<!-- Mobile Screenshots -->
				<div class="md:hidden mb-8 animate-fade-in">
					<div class="relative rounded-2xl overflow-hidden shadow-2xl border border-border/50 max-w-sm mx-auto">
						<!-- Light Mode Screenshot -->
						<img
							src="/screenshots/mobile-light.png"
							alt="吾身移动界面"
							class="w-full h-auto dark:hidden"
							loading="lazy"
						/>
						<!-- Dark Mode Screenshot -->
						<img
							src="/screenshots/mobile-dark.png"
							alt="吾身移动界面"
							class="w-full h-auto hidden dark:block"
							loading="lazy"
						/>
					</div>
				</div>

				<!-- Feature Highlights -->
				<div class="grid grid-cols-1 md:grid-cols-3 gap-6 mt-12">
					<div class="text-center p-6 bg-card/50 rounded-xl border border-border/30">
						<div class="w-12 h-12 mx-auto mb-4 rounded-xl bg-primary/10 flex items-center justify-center">
							<svg class="w-6 h-6 text-primary" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"/>
							</svg>
						</div>
						<h3 class="font-semibold text-foreground mb-2">美观的编辑器</h3>
						<p class="text-sm text-muted-foreground">富文本格式化，界面直观无干扰</p>
					</div>

					<div class="text-center p-6 bg-card/50 rounded-xl border border-border/30">
						<div class="w-12 h-12 mx-auto mb-4 rounded-xl bg-primary/10 flex items-center justify-center">
							<svg class="w-6 h-6 text-primary" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z"/>
							</svg>
						</div>
						<h3 class="font-semibold text-foreground mb-2">智能日历</h3>
						<p class="text-sm text-muted-foreground">追踪你的写作连续天数，轻松浏览记录</p>
					</div>

					<div class="text-center p-6 bg-card/50 rounded-xl border border-border/30">
						<div class="w-12 h-12 mx-auto mb-4 rounded-xl bg-primary/10 flex items-center justify-center">
							<svg class="w-6 h-6 text-primary" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 18h.01M8 21h8a2 2 0 002-2V5a2 2 0 00-2-2H8a2 2 0 00-2 2v14a2 2 0 002 2z"/>
							</svg>
						</div>
						<h3 class="font-semibold text-foreground mb-2">响应式设计</h3>
						<p class="text-sm text-muted-foreground">在桌面、平板和移动设备上均有完美体验</p>
					</div>
				</div>

				<p class="mt-6 text-center text-sm text-muted-foreground">
					在所有设备上提供干净、无干扰的写作体验
				</p>
			</div>
		</section>

		<!-- Features Section -->
		<section id="features" class="py-20 container-responsive">
			<div class="w-full">
				<div class="text-center mb-16 animate-fade-in">
					<h2 class="text-3xl sm:text-4xl font-bold text-foreground mb-4">
						写日记所需的一切
					</h2>
					<p class="text-lg text-muted-foreground max-w-2xl mx-auto">
						强大的功能设计，让每日写作轻松而有意义。
					</p>
				</div>

				<div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
					{#each features as feature, i}
						<div
							class="p-6 bg-card rounded-xl border border-border/50 hover:border-primary/30 hover:shadow-lg transition-all duration-300"
							style="animation-delay: {i * 100}ms"
						>
							<div class="text-4xl mb-4">{feature.icon}</div>
							<h3 class="text-xl font-semibold text-foreground mb-2">{feature.title}</h3>
							<p class="text-muted-foreground">{feature.description}</p>
						</div>
					{/each}
				</div>
			</div>
		</section>

		<!-- AI Assistant Preview -->
		<section class="py-20 bg-muted/30">
			<div class="container-responsive">
				<div class="grid lg:grid-cols-2 gap-12 items-center">
					<div class="animate-fade-in">
						<h2 class="text-3xl sm:text-4xl font-bold text-foreground mb-6">
							你的 AI 驱动反思伙伴
						</h2>
						<p class="text-lg text-muted-foreground mb-6">
							吾身的智能助手会阅读你的日记记录，帮助你发现模式、
							获得洞察，并反思个人成长旅程。
						</p>
						<ul class="space-y-4">
							<li class="flex items-start gap-3">
								<span class="text-primary text-xl">✓</span>
								<span class="text-foreground">询问过去记录的相关问题</span>
							</li>
							<li class="flex items-start gap-3">
								<span class="text-primary text-xl">✓</span>
								<span class="text-foreground">获取个性化写作提示</span>
							</li>
							<li class="flex items-start gap-3">
								<span class="text-primary text-xl">✓</span>
								<span class="text-foreground">发现情绪模式与趋势</span>
							</li>
							<li class="flex items-start gap-3">
								<span class="text-primary text-xl">✓</span>
								<span class="text-foreground">私密且安全的对话</span>
							</li>
						</ul>
					</div>
					<div class="relative">
						<div class="bg-card rounded-2xl border border-border/50 shadow-xl overflow-hidden">
							<!-- Mock Chat Interface -->
							<div class="bg-secondary/30 px-4 py-3 border-b border-border/50">
								<span class="font-medium text-foreground">AI 助手</span>
							</div>
							<div class="p-4 space-y-4 min-h-[300px]">
								<div class="flex gap-3">
									<div class="w-8 h-8 rounded-full bg-primary/20 flex items-center justify-center text-sm">🤖</div>
									<div class="flex-1 bg-muted/50 rounded-lg p-3">
										<p class="text-sm text-foreground">基于你最近的记录，我注意到本周你感觉更加精力充沛。是否愿意探索一下可能带来这一积极变化的因素？</p>
									</div>
								</div>
								<div class="flex gap-3 justify-end">
									<div class="bg-primary/10 rounded-lg p-3 max-w-[80%]">
										<p class="text-sm text-foreground">是的，我很想更好地了解这一点！</p>
									</div>
								</div>
								<div class="flex gap-3">
									<div class="w-8 h-8 rounded-full bg-primary/20 flex items-center justify-center text-sm">🤖</div>
									<div class="flex-1 bg-muted/50 rounded-lg p-3">
										<p class="text-sm text-foreground">回顾你过去两周的记录，我发现你开始了晨间例行活动，并且在锻炼方面更加一致……</p>
									</div>
								</div>
							</div>
						</div>
					</div>
				</div>
			</div>
		</section>

		<!-- CTA Section -->
		<section class="py-20 container-responsive">
			<div class="max-w-3xl mx-auto text-center">
				<h2 class="text-3xl sm:text-4xl font-bold text-foreground mb-6">
					立即开始你的日记旅程
				</h2>
				<p class="text-lg text-muted-foreground mb-8">
					加入成千上万使用「吾身」记录日常想法并通过反思成长的人们。
				</p>
				<a
					href="/login"
					class="inline-block px-8 py-4 text-lg font-medium bg-primary text-primary-foreground rounded-xl hover:opacity-90 transition-all shadow-lg hover:shadow-xl"
				>
					创建你的免费账户
				</a>
				<p class="mt-4 text-sm text-muted-foreground">
					无需信用卡，你的数据始终私密。
				</p>
			</div>
		</section>

		<!-- Footer -->
		<Footer tagline="你的个人日记，由 AI 提供强大动力" />
</div>
{/if}

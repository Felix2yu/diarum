<script lang="ts">
	import { goto } from '$app/navigation';
	import { login, register, type LoginCredentials, type RegisterData } from '$lib/api/auth';
	import { onMount } from 'svelte';
	import { isAuthenticated } from '$lib/api/client';

	let activeTab: 'login' | 'register' = 'login';
	let loading = false;
	let error = '';

	let loginForm: LoginCredentials = {
		usernameOrEmail: '',
		password: ''
	};

	let registerForm: RegisterData = {
		username: '',
		email: '',
		password: '',
		passwordConfirm: ''
	};

	onMount(() => {
		if ($isAuthenticated) {
			const today = new Date().toISOString().split('T')[0];
			goto(`/diary/${today}`);
		}
	});

	async function handleLogin() {
		loading = true;
		error = '';
		const result = await login(loginForm);
		if (result.success) {
			const today = new Date().toISOString().split('T')[0];
			goto(`/diary/${today}`);
		} else {
			error = result.error || '登录失败';
		}
		loading = false;
	}

	async function handleRegister() {
		loading = true;
		error = '';
		if (registerForm.password !== registerForm.passwordConfirm) {
			error = '两次输入的密码不一致';
			loading = false;
			return;
		}
		const result = await register(registerForm);
		if (result.success) {
			const today = new Date().toISOString().split('T')[0];
			goto(`/diary/${today}`);
		} else {
			error = result.error || '注册失败';
		}
		loading = false;
	}
</script>

<div class="min-h-screen flex flex-col bg-background relative overflow-hidden">
	<!-- 背景装饰：纸张光晕 -->
	<div class="pointer-events-none absolute inset-0 opacity-50">
		<div class="absolute top-0 left-0 w-[40rem] h-[40rem] rounded-full bg-sienna/5 blur-3xl"></div>
		<div class="absolute bottom-0 right-0 w-[35rem] h-[35rem] rounded-full bg-sienna/4 blur-3xl"></div>
	</div>

	<!-- 顶部细线导航 -->
	<header class="relative z-10 safe-top">
		<div class="container-responsive">
			<div class="flex items-center justify-between h-16">
				<a href="/" class="flex items-center gap-3 hover:opacity-80 transition-opacity">
					<img src="/logo.png" alt="吾身" class="w-7 h-7" />
					<div class="flex items-baseline gap-2">
						<span class="font-serif text-xl font-medium text-foreground">吾身</span>
						<span class="hidden sm:inline font-mono text-[10px] uppercase tracking-widest text-muted-foreground">Diarum</span>
					</div>
				</a>
				<a
					href="/"
					class="font-mono text-[10px] uppercase tracking-widest text-muted-foreground hover:text-foreground transition-colors"
				>
					← 返回首页
				</a>
			</div>
		</div>
		<div class="h-px bg-border"></div>
	</header>

	<!-- 主体：编辑性登录卡片 -->
	<div class="flex-1 flex items-center justify-center px-4 py-12 relative z-10">
		<div class="w-full max-w-md animate-fade-in">
			<!-- 编辑性头部 -->
			<div class="text-center mb-10">
				<div class="inline-flex items-center gap-3 mb-6">
					<span class="h-px w-8 bg-sienna"></span>
					<span class="font-mono text-[10px] uppercase tracking-widest text-sienna">
						{activeTab === 'login' ? 'SIGN IN · 登录' : 'CREATE · 注册'}
					</span>
					<span class="h-px w-8 bg-sienna"></span>
				</div>
				<img src="/logo.png" alt="吾身" class="w-14 h-14 mx-auto mb-4" />
				<h1 class="font-serif text-4xl font-medium text-foreground mb-2 tracking-tight">
					{#if activeTab === 'login'}
						<span class="italic text-sienna">欢迎</span>回来
					{:else}
						开始<span class="italic text-sienna">书写</span>
					{/if}
				</h1>
				<p class="font-serif italic text-muted-foreground text-sm">
					{#if activeTab === 'login'}
						继续你未完成的反思之旅
					{:else}
						一本属于你自己的日记，从今天开始
					{/if}
				</p>
			</div>

			<!-- 卡片：编辑性纸面 -->
			<div class="bg-card border border-border relative">
				<!-- 装饰：纸张折角 -->
				<div class="absolute -top-px -right-px w-12 h-12 border-t border-r border-sienna/30 hidden sm:block"></div>

				<!-- 标签切换 — 编辑性页签 -->
				<div class="grid grid-cols-2 border-b border-border">
					<button
						class="relative py-4 text-center font-mono text-[10px] uppercase tracking-widest transition-colors duration-200
							   {activeTab === 'login' ? 'text-foreground bg-background' : 'text-muted-foreground hover:text-foreground bg-muted/30'}"
						onclick={() => { activeTab = 'login'; error = ''; }}
					>
						<span class="font-serif text-base normal-case tracking-normal font-medium block mb-0.5">登录</span>
						<span class="opacity-70">Sign In</span>
						{#if activeTab === 'login'}
							<div class="absolute bottom-0 left-0 right-0 h-0.5 bg-sienna"></div>
						{/if}
					</button>
					<button
						class="relative py-4 text-center font-mono text-[10px] uppercase tracking-widest transition-colors duration-200
							   {activeTab === 'register' ? 'text-foreground bg-background' : 'text-muted-foreground hover:text-foreground bg-muted/30'}"
						onclick={() => { activeTab = 'register'; error = ''; }}
					>
						<span class="font-serif text-base normal-case tracking-normal font-medium block mb-0.5">注册</span>
						<span class="opacity-70">Sign Up</span>
						{#if activeTab === 'register'}
							<div class="absolute bottom-0 left-0 right-0 h-0.5 bg-sienna"></div>
						{/if}
					</button>
				</div>

				<div class="p-8">
					{#if error}
						<div class="mb-6 flex items-start gap-3 px-4 py-3 bg-destructive/5 border-l-2 border-destructive text-destructive animate-fade-in">
							<span class="font-serif text-sm italic shrink-0">!</span>
							<span class="text-sm">{error}</span>
						</div>
					{/if}

					{#if activeTab === 'login'}
						<form onsubmit={(e) => { e.preventDefault(); handleLogin(); }} class="space-y-5">
							<div>
								<label for="usernameOrEmail" class="block font-mono text-[10px] uppercase tracking-widest text-muted-foreground mb-2">
									用户名 / 邮箱
								</label>
								<input
									id="usernameOrEmail"
									type="text"
									bind:value={loginForm.usernameOrEmail}
									required
									class="w-full px-0 py-2.5 bg-transparent border-0 border-b border-border
										   focus:outline-none focus:border-sienna
										   text-foreground placeholder:text-muted-foreground/50 transition-colors duration-200 font-serif text-lg"
									placeholder="例如：吾身"
								/>
							</div>

							<div>
								<label for="password" class="block font-mono text-[10px] uppercase tracking-widest text-muted-foreground mb-2">
									密码
								</label>
								<input
									id="password"
									type="password"
									bind:value={loginForm.password}
									required
									class="w-full px-0 py-2.5 bg-transparent border-0 border-b border-border
										   focus:outline-none focus:border-sienna
										   text-foreground placeholder:text-muted-foreground/50 transition-colors duration-200 font-serif text-lg"
									placeholder="••••••••"
								/>
							</div>

							<button
								type="submit"
								disabled={loading}
								class="group w-full mt-6 inline-flex items-center justify-between gap-3 px-6 py-4 bg-foreground text-background hover:bg-sienna transition-colors duration-300 disabled:opacity-50 disabled:cursor-not-allowed"
							>
								<span class="font-serif text-base font-medium">{loading ? '登录中…' : '登录'}</span>
								<span class="font-mono text-xs uppercase tracking-wider opacity-70 group-hover:translate-x-1 transition-transform">
									{loading ? '...' : '→'}
								</span>
							</button>
						</form>
					{:else}
						<form onsubmit={(e) => { e.preventDefault(); handleRegister(); }} class="space-y-5">
							<div>
								<label for="username" class="block font-mono text-[10px] uppercase tracking-widest text-muted-foreground mb-2">
									用户名
								</label>
								<input
									id="username"
									type="text"
									bind:value={registerForm.username}
									required
									minlength="3"
									class="w-full px-0 py-2.5 bg-transparent border-0 border-b border-border
										   focus:outline-none focus:border-sienna
										   text-foreground placeholder:text-muted-foreground/50 transition-colors duration-200 font-serif text-lg"
									placeholder="选择一个用户名"
								/>
							</div>

							<div>
								<label for="email" class="block font-mono text-[10px] uppercase tracking-widest text-muted-foreground mb-2">
									邮箱
								</label>
								<input
									id="email"
									type="email"
									bind:value={registerForm.email}
									required
									class="w-full px-0 py-2.5 bg-transparent border-0 border-b border-border
										   focus:outline-none focus:border-sienna
										   text-foreground placeholder:text-muted-foreground/50 transition-colors duration-200 font-serif text-lg"
									placeholder="you@example.com"
								/>
							</div>

							<div class="grid grid-cols-1 sm:grid-cols-2 gap-5">
								<div>
									<label for="registerPassword" class="block font-mono text-[10px] uppercase tracking-widest text-muted-foreground mb-2">
										密码
									</label>
									<input
										id="registerPassword"
										type="password"
										bind:value={registerForm.password}
										required
										minlength="8"
										class="w-full px-0 py-2.5 bg-transparent border-0 border-b border-border
											   focus:outline-none focus:border-sienna
											   text-foreground placeholder:text-muted-foreground/50 transition-colors duration-200 font-serif text-lg"
										placeholder="至少 8 位"
									/>
								</div>

								<div>
									<label for="passwordConfirm" class="block font-mono text-[10px] uppercase tracking-widest text-muted-foreground mb-2">
										确认
									</label>
									<input
										id="passwordConfirm"
										type="password"
										bind:value={registerForm.passwordConfirm}
										required
										class="w-full px-0 py-2.5 bg-transparent border-0 border-b border-border
											   focus:outline-none focus:border-sienna
											   text-foreground placeholder:text-muted-foreground/50 transition-colors duration-200 font-serif text-lg"
										placeholder="再次输入"
									/>
								</div>
							</div>

							<button
								type="submit"
								disabled={loading}
								class="group w-full mt-6 inline-flex items-center justify-between gap-3 px-6 py-4 bg-foreground text-background hover:bg-sienna transition-colors duration-300 disabled:opacity-50 disabled:cursor-not-allowed"
							>
								<span class="font-serif text-base font-medium">{loading ? '创建中…' : '创建账户'}</span>
								<span class="font-mono text-xs uppercase tracking-wider opacity-70 group-hover:translate-x-1 transition-transform">
									{loading ? '...' : '→'}
								</span>
							</button>
						</form>
					{/if}
				</div>

				<!-- 卡片底部 — 编辑性签名 -->
				<div class="px-8 py-4 border-t border-border bg-muted/20 flex items-center justify-between">
					<span class="font-mono text-[10px] uppercase tracking-widest text-muted-foreground">
						吾身 · DIARUM
					</span>
					<span class="font-mono text-[10px] uppercase tracking-widest text-muted-foreground">
						私有 · 安全
					</span>
				</div>
			</div>

			<!-- 编辑性底部说明 -->
			<p class="mt-8 text-center font-serif italic text-sm text-muted-foreground">
				数据始终私密 · 自托管 · Apache 2.0
			</p>
		</div>
	</div>
</div>

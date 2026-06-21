<script lang="ts">
	import { goto } from '$app/navigation';
	import { login, register, type LoginCredentials, type RegisterData } from '$lib/api/auth';
	import { onMount } from 'svelte';
	import { isAuthenticated } from '$lib/api/client';
	import Footer from '$lib/components/ui/Footer.svelte';

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

<div class="min-h-screen flex flex-col bg-background">
	<div class="flex-1 flex items-center justify-center p-4 sm:p-8">
		<div class="w-full max-w-lg animate-fade-in">
			<div class="text-center mb-8">
				<img src="/logo.png" alt="吾身" class="w-20 h-20 mx-auto mb-4" />
				<h1 class="text-4xl font-bold text-foreground mb-2">吾身</h1>
				<p class="text-muted-foreground">你的个人日记</p>
			</div>

			<div class="bg-card rounded-2xl shadow-xl border border-border/50 p-8">
				<!-- Tabs -->
				<div class="flex border-b border-border mb-6 relative">
					<button
						class="flex-1 py-2.5 px-4 text-center text-sm font-medium transition-all duration-200
							   {activeTab === 'login'
							? 'text-primary'
							: 'text-muted-foreground hover:text-foreground'}"
						on:click={() => { activeTab = 'login'; error = ''; }}
					>
						登录
					</button>
					<button
						class="flex-1 py-2.5 px-4 text-center text-sm font-medium transition-all duration-200
							   {activeTab === 'register'
							? 'text-primary'
							: 'text-muted-foreground hover:text-foreground'}"
						on:click={() => { activeTab = 'register'; error = ''; }}
					>
						注册
					</button>
					<div
						class="absolute bottom-0 h-0.5 bg-primary rounded-full transition-all duration-300 ease-out"
						style="width: 50%; left: {activeTab === 'login' ? '0%' : '50%'};"
					></div>
				</div>

				{#if error}
					<div class="mb-4 p-3 bg-destructive/10 border border-destructive/20 text-destructive rounded-xl text-sm animate-fade-in">
						{error}
					</div>
				{/if}

				{#if activeTab === 'login'}
					<form on:submit|preventDefault={handleLogin} class="space-y-4">
						<div>
							<label for="usernameOrEmail" class="block text-sm font-medium text-foreground mb-1.5">
								用户名或邮箱
							</label>
							<input
								id="usernameOrEmail"
								type="text"
								bind:value={loginForm.usernameOrEmail}
								required
								class="w-full px-4 py-2.5 bg-background border border-border rounded-xl
									   focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary
									   text-foreground placeholder:text-muted-foreground transition-all duration-200"
								placeholder="输入你的用户名或邮箱"
							/>
						</div>

						<div>
							<label for="password" class="block text-sm font-medium text-foreground mb-1.5">
								密码
							</label>
							<input
								id="password"
								type="password"
								bind:value={loginForm.password}
								required
								class="w-full px-4 py-2.5 bg-background border border-border rounded-xl
									   focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary
									   text-foreground placeholder:text-muted-foreground transition-all duration-200"
								placeholder="输入你的密码"
							/>
						</div>

						<button
							type="submit"
							disabled={loading}
							class="w-full py-3 px-4 bg-primary text-primary-foreground rounded-xl font-medium
								   hover:opacity-90 active:scale-[0.98] transition-all duration-200 disabled:opacity-50 shadow-sm"
						>
							{loading ? '登录中...' : '登录'}
						</button>
					</form>
				{:else}
					<form on:submit|preventDefault={handleRegister} class="space-y-4">
						<div>
							<label for="username" class="block text-sm font-medium text-foreground mb-1.5">
								用户名
							</label>
							<input
								id="username"
								type="text"
								bind:value={registerForm.username}
								required
								minlength="3"
								class="w-full px-4 py-2.5 bg-background border border-border rounded-xl
									   focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary
									   text-foreground placeholder:text-muted-foreground transition-all duration-200"
								placeholder="选择一个用户名"
							/>
						</div>

						<div>
							<label for="email" class="block text-sm font-medium text-foreground mb-1.5">
								邮箱
							</label>
							<input
								id="email"
								type="email"
								bind:value={registerForm.email}
								required
								class="w-full px-4 py-2.5 bg-background border border-border rounded-xl
									   focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary
									   text-foreground placeholder:text-muted-foreground transition-all duration-200"
								placeholder="输入你的邮箱"
							/>
						</div>

						<div>
							<label for="registerPassword" class="block text-sm font-medium text-foreground mb-1.5">
								密码
							</label>
							<input
								id="registerPassword"
								type="password"
								bind:value={registerForm.password}
								required
								minlength="8"
								class="w-full px-4 py-2.5 bg-background border border-border rounded-xl
									   focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary
									   text-foreground placeholder:text-muted-foreground transition-all duration-200"
								placeholder="选择一个密码（至少 8 个字符）"
							/>
						</div>

						<div>
							<label for="passwordConfirm" class="block text-sm font-medium text-foreground mb-1.5">
								确认密码
							</label>
							<input
								id="passwordConfirm"
								type="password"
								bind:value={registerForm.passwordConfirm}
								required
								class="w-full px-4 py-2.5 bg-background border border-border rounded-xl
									   focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary
									   text-foreground placeholder:text-muted-foreground transition-all duration-200"
								placeholder="再次输入密码"
							/>
						</div>

						<button
							type="submit"
							disabled={loading}
							class="w-full py-3 px-4 bg-primary text-primary-foreground rounded-xl font-medium
								   hover:opacity-90 active:scale-[0.98] transition-all duration-200 disabled:opacity-50 shadow-sm"
						>
							{loading ? '创建账户中...' : '创建账户'}
						</button>
					</form>
				{/if}
			</div>
		</div>
	</div>

	<!-- Footer -->
	<Footer />
</div>

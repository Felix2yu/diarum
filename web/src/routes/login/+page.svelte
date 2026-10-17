<script lang="ts">
  import { resolve } from '$app/paths';
  import { goto } from '$app/navigation';
  import { login, register, type LoginCredentials, type RegisterData } from '#lib/api/auth.js';
  import { onMount } from 'svelte';
  import { isAuthenticated } from '#lib/api/client.js';

  let activeTab: 'login' | 'register' = 'login';
  let loading = false;
  let error = '';
  let loginTabBtn: HTMLButtonElement | undefined;
  let registerTabBtn: HTMLButtonElement | undefined;

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
      goto(resolve('/diary/[date]', { date: today }));
    }
  });

  async function handleLogin() {
    loading = true;
    error = '';
    const result = await login(loginForm);
    if (result.success) {
      const today = new Date().toISOString().split('T')[0];
      goto(resolve('/diary/[date]', { date: today }));
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
      goto(resolve('/diary/[date]', { date: today }));
    } else {
      error = result.error || '注册失败';
    }
    loading = false;
  }

  function handleTabKeydown(e: KeyboardEvent) {
    // Left / Right arrow 切换 Tab; Home / End 跳到第一个或最后一个
    const tabs: Array<'login' | 'register'> = ['login', 'register'];
    const curIdx = tabs.indexOf(activeTab);
    let targetIdx: number;
    if (e.key === 'ArrowRight') {
      targetIdx = (curIdx + 1) % tabs.length;
    } else if (e.key === 'ArrowLeft') {
      targetIdx = (curIdx - 1 + tabs.length) % tabs.length;
    } else if (e.key === 'Home') {
      targetIdx = 0;
    } else if (e.key === 'End') {
      targetIdx = tabs.length - 1;
    } else {
      return;
    }
    e.preventDefault();
    activeTab = tabs[targetIdx];
    error = '';
    // 焦点移到对应的 tab 按钮
    const targetBtn = tabs[targetIdx] === 'login' ? loginTabBtn : registerTabBtn;
    targetBtn?.focus();
  }
</script>

<div class="flex min-h-screen flex-col bg-background">
  <main id="main-content" tabindex="-1" class="flex flex-1 items-center justify-center p-4 sm:p-8">
    <div class="animate-fade-in w-full max-w-lg">
      <div class="mb-8 text-center">
        <img src="/logo.png" alt="吾身" class="mx-auto mb-4 h-20 w-20" />
        <h1 class="mb-2 text-4xl font-bold text-foreground">吾身</h1>
        <p class="text-muted-foreground">你的个人日记</p>
      </div>

      <div class="rounded-2xl border border-border/50 bg-card p-8 shadow-xl">
        <!-- Tabs: 使用 role=tablist/tab + aria-selected/aria-controls，配合方向键切换 -->
        <div
          role="tablist"
          aria-label="登录或注册切换"
          class="relative mb-6 flex border-b border-border"
        >
          <button
            bind:this={loginTabBtn}
            role="tab"
            id="tab-login"
            aria-selected={activeTab === 'login'}
            aria-controls="panel-login"
            tabindex={activeTab === 'login' ? 0 : -1}
            class="-mb-[1px] flex-1 border-b-2 px-4 py-2.5 text-center text-sm font-medium transition-colors duration-200
							   {activeTab === 'login'
              ? 'border-primary text-primary'
              : 'border-transparent text-muted-foreground hover:text-foreground'}"
            onclick={() => {
              activeTab = 'login';
              error = '';
            }}
            onkeydown={handleTabKeydown}
          >
            登录
          </button>
          <button
            bind:this={registerTabBtn}
            role="tab"
            id="tab-register"
            aria-selected={activeTab === 'register'}
            aria-controls="panel-register"
            tabindex={activeTab === 'register' ? 0 : -1}
            class="-mb-[1px] flex-1 border-b-2 px-4 py-2.5 text-center text-sm font-medium transition-colors duration-200
							   {activeTab === 'register'
              ? 'border-primary text-primary'
              : 'border-transparent text-muted-foreground hover:text-foreground'}"
            onclick={() => {
              activeTab = 'register';
              error = '';
            }}
            onkeydown={handleTabKeydown}
          >
            注册
          </button>
        </div>

        {#if error}
          <div
            role="alert"
            aria-live="assertive"
            class="bg-destructive/10 border-destructive/20 text-destructive animate-fade-in mb-4 rounded-xl border p-3 text-sm"
          >
            {error}
          </div>
        {/if}

        {#if activeTab === 'login'}
          <form
            id="panel-login"
            role="tabpanel"
            aria-labelledby="tab-login"
            tabindex={0}
            onsubmit={(e) => {
              e.preventDefault();
              handleLogin();
            }}
            class="space-y-4"
          >
            <div>
              <label for="usernameOrEmail" class="mb-1.5 block text-sm font-medium text-foreground">
                用户名或邮箱
              </label>
              <input
                id="usernameOrEmail"
                type="text"
                bind:value={loginForm.usernameOrEmail}
                required
                autocomplete="username"
                class="w-full rounded-xl border border-border bg-background px-4 py-2.5
									   text-foreground transition-colors duration-200 placeholder:text-muted-foreground
									   focus:border-primary focus:ring-2 focus:ring-primary/50 focus:outline-none"
                placeholder="输入你的用户名或邮箱…"
              />
            </div>

            <div>
              <label for="password" class="mb-1.5 block text-sm font-medium text-foreground">
                密码
              </label>
              <input
                id="password"
                type="password"
                bind:value={loginForm.password}
                required
                autocomplete="current-password"
                class="w-full rounded-xl border border-border bg-background px-4 py-2.5
									   text-foreground transition-colors duration-200 placeholder:text-muted-foreground
									   focus:border-primary focus:ring-2 focus:ring-primary/50 focus:outline-none"
                placeholder="输入你的密码…"
              />
            </div>

            <button
              type="submit"
              disabled={loading}
              class="w-full rounded-xl bg-primary px-4 py-3 font-medium text-primary-foreground
								   shadow-sm transition-opacity duration-200 hover:opacity-90 active:scale-[0.98] disabled:opacity-50"
            >
              {loading ? '登录中…' : '登录'}
            </button>
          </form>
        {:else}
          <form
            id="panel-register"
            role="tabpanel"
            aria-labelledby="tab-register"
            tabindex={0}
            onsubmit={(e) => {
              e.preventDefault();
              handleRegister();
            }}
            class="space-y-4"
          >
            <div>
              <label for="username" class="mb-1.5 block text-sm font-medium text-foreground">
                用户名
              </label>
              <input
                id="username"
                type="text"
                bind:value={registerForm.username}
                required
                minlength="3"
                autocomplete="username"
                class="w-full rounded-xl border border-border bg-background px-4 py-2.5
									   text-foreground transition-colors duration-200 placeholder:text-muted-foreground
									   focus:border-primary focus:ring-2 focus:ring-primary/50 focus:outline-none"
                placeholder="选择一个用户名…"
              />
            </div>

            <div>
              <label for="email" class="mb-1.5 block text-sm font-medium text-foreground">
                邮箱
              </label>
              <input
                id="email"
                type="email"
                bind:value={registerForm.email}
                required
                autocomplete="email"
                class="w-full rounded-xl border border-border bg-background px-4 py-2.5
									   text-foreground transition-colors duration-200 placeholder:text-muted-foreground
									   focus:border-primary focus:ring-2 focus:ring-primary/50 focus:outline-none"
                placeholder="输入你的邮箱…"
              />
            </div>

            <div>
              <label
                for="registerPassword"
                class="mb-1.5 block text-sm font-medium text-foreground"
              >
                密码
              </label>
              <input
                id="registerPassword"
                type="password"
                bind:value={registerForm.password}
                required
                minlength="8"
                autocomplete="new-password"
                class="w-full rounded-xl border border-border bg-background px-4 py-2.5
									   text-foreground transition-colors duration-200 placeholder:text-muted-foreground
									   focus:border-primary focus:ring-2 focus:ring-primary/50 focus:outline-none"
                placeholder="选择一个密码（至少 8 个字符）…"
              />
            </div>

            <div>
              <label for="passwordConfirm" class="mb-1.5 block text-sm font-medium text-foreground">
                确认密码
              </label>
              <input
                id="passwordConfirm"
                type="password"
                bind:value={registerForm.passwordConfirm}
                required
                autocomplete="new-password"
                class="w-full rounded-xl border border-border bg-background px-4 py-2.5
									   text-foreground transition-colors duration-200 placeholder:text-muted-foreground
									   focus:border-primary focus:ring-2 focus:ring-primary/50 focus:outline-none"
                placeholder="再次输入密码…"
              />
            </div>

            <button
              type="submit"
              disabled={loading}
              class="w-full rounded-xl bg-primary px-4 py-3 font-medium text-primary-foreground
								   shadow-sm transition-opacity duration-200 hover:opacity-90 active:scale-[0.98] disabled:opacity-50"
            >
              {loading ? '创建账户中…' : '创建账户'}
            </button>
          </form>
        {/if}
      </div>
    </div>
  </main>
</div>

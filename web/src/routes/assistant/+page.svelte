<script lang="ts">
  import { resolve } from '$app/paths';
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { isAuthenticated } from '#lib/api/client.js';
  import { getAISettings } from '#lib/api/ai.js';
  import {
    getConversations,
    createConversation,
    deleteConversation,
    type Conversation
  } from '#lib/api/chat.js';
  import ChatInput from '#lib/components/chat/ChatInput.svelte';
  import ConversationList from '#lib/components/chat/ConversationList.svelte';
  import PageHeader from '#lib/components/ui/PageHeader.svelte';

  let conversations: Conversation[] = [];
  let isCreating = false;
  let loading = true;
  let aiEnabled = false;
  let sidebarOpen = false;
  let chatError = '';

  function closeSidebarOnMobile() {
    if (window.innerWidth < 1024) {
      sidebarOpen = false;
    }
  }

  async function loadConversations() {
    try {
      conversations = await getConversations();
    } catch (e) {
      console.error('Failed to load conversations:', e);
    }
  }

  function handleStartNewChat() {
    closeSidebarOnMobile();
  }

  async function handleSelectConversation(convId: string) {
    goto(resolve('/assistant/[id]', { id: convId }));
  }

  async function handleDeleteConversation(convId: string) {
    try {
      await deleteConversation(convId);
      conversations = conversations.filter((c) => c.id !== convId);
    } catch (e) {
      console.error('Failed to delete conversation:', e);
    }
  }

  async function handleSendMessage(content: string) {
    if (isCreating) return;

    isCreating = true;
    chatError = '';
    try {
      const conv = await createConversation();
      conversations = [conv, ...conversations];
      const url = new URL(resolve('/assistant/[id]', { id: conv.id }), window.location.origin);
      url.searchParams.set('message', content);
      // url.pathname 来自 window.location.origin 解析，站内绝对路径，无需再 resolve。
      // eslint-disable-next-line svelte/no-navigation-without-resolve
      goto(url.pathname + url.search);
    } catch (e) {
      console.error('Failed to create conversation:', e);
      chatError = e instanceof Error ? e.message : '创建对话失败';
    }
    isCreating = false;
  }

  onMount(async () => {
    if (!$isAuthenticated) {
      goto(resolve('/login'));
      return;
    }

    const settings = await getAISettings();
    aiEnabled = settings.enabled;

    if (!aiEnabled) {
      loading = false;
      return;
    }

    await loadConversations();
    loading = false;
  });
</script>

<svelte:head>
  <title>AI 助手 - 吾身</title>
</svelte:head>

<div class="flex h-screen flex-col overflow-hidden bg-background">
  <PageHeader title="AI 助手" sticky={false}>
    <button
      slot="actions"
      onclick={() => (sidebarOpen = !sidebarOpen)}
      class="rounded-lg p-1.5 transition-all duration-200 hover:bg-muted/50 lg:hidden"
      aria-label="切换侧边栏"
    >
      <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M4 6h16M4 12h16M4 18h16"
        />
      </svg>
    </button>
  </PageHeader>

  <!-- Main Content -->
  {#if loading}
    <div class="flex flex-1 items-center justify-center">
      <svg class="h-8 w-8 animate-spin text-primary" fill="none" viewBox="0 0 24 24">
        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"
        ></circle>
        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
        ></path>
      </svg>
    </div>
  {:else if !aiEnabled}
    <div class="flex flex-1 items-center justify-center p-4">
      <div class="max-w-md text-center">
        <svg
          class="mx-auto mb-4 h-16 w-16 text-muted-foreground"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="1.5"
            d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
          />
        </svg>
        <h2 class="mb-2 text-xl font-semibold">AI 功能未启用</h2>
        <p class="mb-4 text-muted-foreground">请在设置中启用 AI 功能以使用 AI 助手。</p>
        <a
          href={resolve('/settings')}
          class="inline-flex items-center gap-2 rounded-lg bg-primary px-4 py-2 text-primary-foreground hover:opacity-90"
        >
          前往设置
        </a>
      </div>
    </div>
  {:else}
    <div class="relative flex flex-1 overflow-hidden lg:gap-4 lg:p-4">
      <!-- Mobile Overlay -->
      {#if sidebarOpen}
        <button
          class="fixed inset-0 z-30 bg-black/40 backdrop-blur-sm lg:hidden"
          onclick={() => (sidebarOpen = false)}
          aria-label="关闭侧边栏"
        ></button>
      {/if}

      <!-- Sidebar - Desktop -->
      <aside
        class="hidden w-72 flex-shrink-0 overflow-hidden
				rounded-2xl border border-border bg-card/50 lg:relative lg:block"
      >
        <ConversationList
          {conversations}
          selectedId={null}
          {loading}
          on:select={(e) => handleSelectConversation(e.detail)}
          on:create={handleStartNewChat}
          on:delete={(e) => handleDeleteConversation(e.detail)}
        />
      </aside>

      <!-- Mobile Drawer -->
      {#if sidebarOpen}
        <div
          class="animate-slide-in-left fixed inset-y-0 left-0 z-40 w-72 border-r border-border bg-card shadow-2xl lg:hidden"
        >
          <!-- Drawer Header -->
          <div class="flex items-center justify-between border-b border-border/50 px-5 py-4">
            <div class="flex items-center gap-2">
              <img src="/logo.png" alt="吾身" class="h-6 w-6" />
              <span class="font-semibold text-foreground">AI 助手</span>
            </div>
            <button
              onclick={() => (sidebarOpen = false)}
              class="rounded-lg p-2 transition-colors hover:bg-muted"
              aria-label="关闭"
            >
              <svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M6 18L18 6M6 6l12 12"
                />
              </svg>
            </button>
          </div>

          <!-- Drawer Content -->
          <div class="flex h-[calc(100%-57px)] flex-col">
            <!-- Actions Section -->
            <div class="px-3 py-3">
              <div
                class="mb-2 px-1 text-[10px] font-semibold tracking-wider text-muted-foreground uppercase"
              >
                快捷操作
              </div>
              <div class="space-y-0.5">
                <a
                  href={resolve('/diary')}
                  class="group flex items-center gap-2.5 rounded-lg px-2 py-1.5 transition-all duration-200 hover:bg-muted/70"
                  onclick={() => (sidebarOpen = false)}
                >
                  <div
                    class="rounded-md bg-green-500/10 p-1.5 text-green-500 transition-colors group-hover:bg-green-500/20"
                  >
                    <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z"
                      />
                    </svg>
                  </div>
                  <div class="min-w-0">
                    <div class="text-xs font-medium text-foreground">日历</div>
                    <div class="truncate text-[10px] text-muted-foreground">查看所有日记条目</div>
                  </div>
                </a>

                <a
                  href={resolve('/search')}
                  class="group flex items-center gap-2.5 rounded-lg px-2 py-1.5 transition-all duration-200 hover:bg-muted/70"
                  onclick={() => (sidebarOpen = false)}
                >
                  <div
                    class="rounded-md bg-blue-500/10 p-1.5 text-blue-500 transition-colors group-hover:bg-blue-500/20"
                  >
                    <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
                      />
                    </svg>
                  </div>
                  <div class="min-w-0">
                    <div class="text-xs font-medium text-foreground">搜索</div>
                    <div class="truncate text-[10px] text-muted-foreground">查找日记条目</div>
                  </div>
                </a>

                <a
                  href={resolve('/media')}
                  class="group flex items-center gap-2.5 rounded-lg px-2 py-1.5 transition-all duration-200 hover:bg-muted/70"
                  onclick={() => (sidebarOpen = false)}
                >
                  <div
                    class="rounded-md bg-purple-500/10 p-1.5 text-purple-500 transition-colors group-hover:bg-purple-500/20"
                  >
                    <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z"
                      />
                    </svg>
                  </div>
                  <div class="min-w-0">
                    <div class="text-xs font-medium text-foreground">媒体</div>
                    <div class="truncate text-[10px] text-muted-foreground">浏览照片和文件</div>
                  </div>
                </a>

                <a
                  href={resolve('/settings')}
                  class="group flex items-center gap-2.5 rounded-lg px-2 py-1.5 transition-all duration-200 hover:bg-muted/70"
                  onclick={() => (sidebarOpen = false)}
                >
                  <div
                    class="rounded-md bg-gray-500/10 p-1.5 text-gray-500 transition-colors group-hover:bg-gray-500/20"
                  >
                    <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"
                      />
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
                      />
                    </svg>
                  </div>
                  <div class="min-w-0">
                    <div class="text-xs font-medium text-foreground">设置</div>
                    <div class="truncate text-[10px] text-muted-foreground">偏好设置和 AI 配置</div>
                  </div>
                </a>
              </div>
            </div>

            <!-- Divider -->
            <div class="mx-3 border-t border-border/50"></div>

            <!-- Conversations Section -->
            <div class="flex-1 overflow-hidden">
              <ConversationList
                {conversations}
                selectedId={null}
                {loading}
                on:select={(e) => {
                  sidebarOpen = false;
                  handleSelectConversation(e.detail);
                }}
                on:create={() => {
                  sidebarOpen = false;
                  handleStartNewChat();
                }}
                on:delete={(e) => handleDeleteConversation(e.detail)}
              />
            </div>
          </div>
        </div>
      {/if}

      <!-- Chat Area - New Chat Mode -->
      <main
        id="main-content"
        tabindex="-1"
        class="flex min-w-0 flex-1 flex-col overflow-hidden lg:rounded-2xl lg:border lg:border-border lg:bg-card/50"
      >
        <!-- 页面标题 - 视觉隐藏，用于屏幕阅读器的文档结构 -->
        <h1 class="sr-only">AI 助手 — 与智能助手对话反思你的日记</h1>
        <!-- Empty state with prompt -->
        <div class="flex-1 overflow-y-auto p-4 lg:p-6">
          <div class="flex h-full flex-col items-center justify-center py-12 text-center">
            <div
              class="mb-4 flex h-20 w-20 items-center justify-center rounded-2xl bg-gradient-to-br from-primary/10 to-primary/5"
            >
              <svg
                class="h-10 w-10 text-primary/60"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="1.5"
                  d="M7 8h10M7 12h4m1 8l-4-4H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-3l-4 4z"
                />
              </svg>
            </div>
            <h3 class="mb-1 text-lg font-medium text-foreground">开始新的对话</h3>
            <p class="max-w-xs text-sm text-muted-foreground">
              向 AI 助手提问关于你的日记内容，获得个性化洞察
            </p>
          </div>
        </div>

        <!-- Input -->
        <div
          class="flex-shrink-0 border-t border-border/50 bg-gradient-to-t from-card/80 to-card/50 p-4 backdrop-blur-sm lg:p-6"
        >
          <div class="mx-auto max-w-3xl">
            {#if chatError}
              <div
                class="bg-destructive/10 text-destructive mb-3 flex items-start gap-2 rounded-lg p-3 text-sm"
              >
                <svg
                  class="mt-0.5 h-4 w-4 flex-shrink-0"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                  />
                </svg>
                <span>{chatError}</span>
                <button
                  onclick={() => (chatError = '')}
                  class="hover:bg-destructive/20 ml-auto rounded p-0.5"
                  aria-label="关闭错误"
                >
                  <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M6 18L18 6M6 6l12 12"
                    />
                  </svg>
                </button>
              </div>
            {/if}
            <ChatInput
              disabled={isCreating}
              placeholder="询问关于你的日记..."
              on:send={(e) => handleSendMessage(e.detail)}
            />
            <div
              class="mt-3 flex items-center justify-center gap-2 text-xs text-muted-foreground/60"
            >
              <span class="hidden sm:inline">按 Enter 发送,Shift+Enter 换行</span>
            </div>
          </div>
        </div>
      </main>
    </div>
  {/if}
</div>

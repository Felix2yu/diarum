<script lang="ts">
  import { resolve } from '$app/paths';
  import { onMount } from 'svelte';
  import { afterNavigate, goto } from '$app/navigation';
  import { page } from '$app/state';
  import { isAuthenticated } from '#lib/api/client.js';
  import { getAISettings } from '#lib/api/ai.js';
  import {
    getConversations,
    getConversation,
    deleteConversation,
    streamChat,
    type Conversation,
    type Message
  } from '#lib/api/chat.js';
  import ChatMessage from '#lib/components/chat/ChatMessage.svelte';
  import ChatInput from '#lib/components/chat/ChatInput.svelte';
  import ConversationList from '#lib/components/chat/ConversationList.svelte';
  import PageHeader from '#lib/components/ui/PageHeader.svelte';

  let conversations: Conversation[] = [];
  let selectedConversationId: string | null = null;
  let messages: Message[] = [];
  let streamingContent = '';
  let isStreaming = false;
  let loading = true;
  let messagesLoading = false;
  let aiEnabled = false;
  let sidebarOpen = false;
  let messagesContainer: HTMLDivElement;
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

  async function loadMessages(convId: string) {
    messagesLoading = true;
    try {
      const detail = await getConversation(convId);
      messages = detail.messages;
      scrollToBottom();
    } catch (e) {
      console.error('Failed to load messages:', e);
      // If conversation not found, redirect to assistant main page
      goto(resolve('/assistant'));
    }
    messagesLoading = false;
  }

  function scrollToBottom() {
    setTimeout(() => {
      if (messagesContainer) {
        messagesContainer.scrollTop = messagesContainer.scrollHeight;
      }
    }, 50);
  }

  function handleStartNewChat() {
    goto(resolve('/assistant'));
  }

  async function handleSelectConversation(convId: string) {
    if (convId !== selectedConversationId) {
      goto(resolve('/assistant/[id]', { id: convId }));
    }
    closeSidebarOnMobile();
  }

  async function handleDeleteConversation(convId: string) {
    try {
      await deleteConversation(convId);
      conversations = conversations.filter((c) => c.id !== convId);
      if (selectedConversationId === convId) {
        goto(resolve('/assistant'));
      }
    } catch (e) {
      console.error('Failed to delete conversation:', e);
    }
  }

  async function handleSendMessage(content: string) {
    if (isStreaming || !selectedConversationId) return;

    const convId = selectedConversationId;
    chatError = '';

    // Add user message with unique ID
    const userMsg: Message = {
      id: `temp-user-${Date.now()}`,
      role: 'user',
      content,
      created: new Date().toISOString()
    };
    messages = [...messages, userMsg];
    scrollToBottom();

    isStreaming = true;
    streamingContent = '';

    try {
      for await (const chunk of streamChat(convId, content)) {
        if (chunk.error) {
          console.error('Stream error:', chunk.error);
          chatError = chunk.error;
          break;
        }
        if (chunk.title && convId) {
          conversations = conversations.map((c) =>
            c.id === convId ? { ...c, title: chunk.title! } : c
          );
        }
        if (chunk.content) {
          streamingContent += chunk.content;
          scrollToBottom();
        }
        if (chunk.done) {
          const assistantMsg: Message = {
            id: `temp-assistant-${Date.now()}`,
            role: 'assistant',
            content: streamingContent,
            referenced_diaries: chunk.referenced_diaries,
            created: new Date().toISOString()
          };
          messages = [...messages, assistantMsg];
          streamingContent = '';
        }
      }
    } catch (e) {
      console.error('Failed to send message:', e);
      chatError = e instanceof Error ? e.message : '发送消息失败';
    }

    isStreaming = false;
    await loadConversations();
  }

  // `page`（$app/state）是 runes 响应式对象，legacy `$:` 中读取不会建立依赖，
  // 原来的 `$: if (page.params.id ...)` 在会话之间切换时永远不会触发。
  // 改为在导航完成后同步会话 id（首次进入由下面的 onMount 负责，此时 loading 仍为 true）。
  afterNavigate(() => {
    const id = page.params.id;
    if (id && id !== selectedConversationId && !loading && aiEnabled) {
      selectedConversationId = id;
      loadMessages(id);
    }
  });

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

    // Get conversation ID from URL
    const convId = page.params.id;
    if (convId) {
      selectedConversationId = convId;
      await loadMessages(convId);
    }

    loading = false;

    // Check for message query parameter (from new chat redirect)
    const messageParam = page.url.searchParams.get('message');
    if (messageParam && convId) {
      // 清掉 URL 上的 ?message=：Kit 3 中原生 history.replaceState 会覆盖 Kit 写入的
      // history 元数据，导致前进/后退时内部索引跟踪错位（官方已弃用该写法）。
      // 改用浅路由 goto —— 只改 URL、不重新渲染，语义与原「不触发导航」一致。
      await goto(resolve('/assistant/[id]', { id: convId }), {
        replace: true,
        shallow: true,
        state: {}
      });
      // Send the message
      await handleSendMessage(messageParam);
    }
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
          class="fixed inset-0 z-30 bg-black/50 lg:hidden"
          onclick={() => (sidebarOpen = false)}
          aria-label="关闭侧边栏"
        ></button>
      {/if}

      <!-- Sidebar -->
      <aside
        class="fixed inset-y-0 left-0 z-40 w-72 flex-shrink-0 transform
				border-r border-border bg-card transition-transform duration-300 ease-in-out lg:relative
				lg:w-72 lg:rounded-2xl lg:border lg:bg-card/50
				{sidebarOpen ? 'translate-x-0' : '-translate-x-full'} top-11
				h-[calc(100vh-2.75rem)] overflow-hidden lg:top-0 lg:h-full lg:translate-x-0"
      >
        <ConversationList
          {conversations}
          selectedId={selectedConversationId}
          {loading}
          on:select={(e) => handleSelectConversation(e.detail)}
          on:create={handleStartNewChat}
          on:delete={(e) => handleDeleteConversation(e.detail)}
        />
      </aside>

      <!-- Chat Area -->
      <main
        id="main-content"
        tabindex="-1"
        class="overflow:hidden flex min-w-0 flex-1 flex-col lg:rounded-2xl lg:border lg:border-border lg:bg-card/50"
      >
        <!-- 页面标题 - 视觉隐藏，用于屏幕阅读器的文档结构 -->
        <h1 class="sr-only">AI 助手对话 — 查看历史对话并继续</h1>
        <!-- Messages -->
        <div bind:this={messagesContainer} class="flex-1 overflow-y-auto p-4 lg:p-6">
          {#if messagesLoading}
            <div class="flex justify-center py-12">
              <div class="flex flex-col items-center gap-3">
                <svg class="h-8 w-8 animate-spin text-primary" fill="none" viewBox="0 0 24 24">
                  <circle
                    class="opacity-25"
                    cx="12"
                    cy="12"
                    r="10"
                    stroke="currentColor"
                    stroke-width="4"
                  ></circle>
                  <path
                    class="opacity-75"
                    fill="currentColor"
                    d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
                  ></path>
                </svg>
                <span class="text-sm text-muted-foreground">正在加载消息...</span>
              </div>
            </div>
          {:else if messages.length === 0 && !streamingContent}
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
              <h3 class="mb-1 text-lg font-medium text-foreground">开始对话</h3>
              <p class="max-w-xs text-sm text-muted-foreground">向 AI 助手提问关于你的日记内容</p>
            </div>
          {:else}
            <div class="mx-auto max-w-3xl space-y-4">
              {#each messages as message (message.id)}
                <ChatMessage {message} />
              {/each}
              {#if isStreaming}
                <ChatMessage
                  message={{
                    id: 'streaming',
                    role: 'assistant',
                    content: streamingContent,
                    created: ''
                  }}
                  isStreaming={true}
                />
              {/if}
            </div>
          {/if}
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
              disabled={isStreaming}
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

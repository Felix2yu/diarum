<script lang="ts">
  import { resolve } from '$app/paths';
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { getToday } from '#lib/utils/date.js';
  import { isAuthenticated } from '#lib/api/client.js';
  import { getGeneralSettings } from '#lib/api/settings.js';

  let ready = $state(false);
  let visibleSections = $state<Set<number>>(new Set());

  onMount(() => {
    if ($isAuthenticated) {
      getGeneralSettings()
        .then((s) =>
          goto(
            s.default_view === 'calendar'
              ? resolve('/diary')
              : resolve('/diary/[date]', { date: getToday() })
          )
        )
        .catch(() => goto(resolve('/diary/[date]', { date: getToday() })))
        .catch(() => {
          ready = true;
        });
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

    return () => observer.disconnect();
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
  <div class="flex min-h-screen items-center justify-center">
    <p class="text-muted-foreground">加载中…</p>
  </div>
{:else}
  <div class="flex min-h-[100dvh] min-h-screen flex-col bg-background">
    <!-- Navigation -->
    <nav class="glass safe-top fixed top-0 right-0 left-0 z-50 border-b border-border/50">
      <div class="container-responsive h-16">
        <div class="flex h-full items-center justify-between">
          <div class="flex items-center gap-3">
            <img src="/logo.png" alt="吾身" class="h-10 w-10" />
            <span class="text-2xl font-bold text-foreground">吾身</span>
          </div>
          <a
            href={resolve('/login')}
            class="px-5 py-2 text-sm font-medium text-foreground transition-colors hover:text-primary"
          >
            登录
          </a>
        </div>
      </div>
    </nav>

    <main id="main-content" tabindex="-1">
      <!-- Hero Section -->
      <section class="container-responsive pt-32 pb-20">
        <div class="animate-fade-in mx-auto max-w-4xl text-center">
          <h1 class="mb-6 text-4xl leading-tight font-bold text-foreground sm:text-5xl lg:text-6xl">
            你的个人空间，用于
            <span class="text-primary">每日反思</span>
          </h1>
          <p class="mx-auto mb-8 max-w-2xl text-lg text-muted-foreground sm:text-xl">
            记录你的思绪，追踪你的成长，借助 AI 驱动的日记功能获得深入洞察。
            一本美丽私密的日记，与你共同成长。
          </p>
          <div class="flex flex-col items-center justify-center gap-4 sm:flex-row">
            <a
              href={resolve('/login')}
              class="w-full rounded-xl bg-primary px-8 py-3 text-lg font-medium text-primary-foreground shadow-lg transition-all hover:opacity-90 hover:shadow-xl sm:w-auto"
            >
              立即开始写作
            </a>
            <a
              href="#features"
              class="w-full rounded-xl border border-border px-8 py-3 text-lg font-medium text-foreground transition-all hover:bg-accent sm:w-auto"
            >
              了解更多
            </a>
          </div>
        </div>
      </section>

      <!-- Screenshots Section -->
      <section class="bg-muted/30 py-16" data-observe data-idx="0">
        <div class="container-responsive">
          <!-- Desktop Screenshots -->
          <div
            class="mb-12 hidden md:block"
            class:animate-fade-in={visibleSections.has(0)}
            style:opacity={visibleSections.has(0) ? 1 : 0}
          >
            <div class="relative overflow-hidden rounded-2xl border border-border/50 shadow-2xl">
              <img
                src="/screenshots/desktop-light.png"
                alt="吾身桌面界面"
                class="h-auto w-full dark:hidden"
                loading="lazy"
              />
              <img
                src="/screenshots/desktop-dark.png"
                alt="吾身桌面界面"
                class="hidden h-auto w-full dark:block"
                loading="lazy"
              />
            </div>
          </div>

          <!-- Mobile Screenshots -->
          <div
            class="mb-8 md:hidden"
            class:animate-fade-in={visibleSections.has(0)}
            style:opacity={visibleSections.has(0) ? 1 : 0}
          >
            <div
              class="relative mx-auto max-w-sm overflow-hidden rounded-2xl border border-border/50 shadow-2xl"
            >
              <img
                src="/screenshots/mobile-light.png"
                alt="吾身移动界面"
                class="h-auto w-full dark:hidden"
                loading="lazy"
              />
              <img
                src="/screenshots/mobile-dark.png"
                alt="吾身移动界面"
                class="hidden h-auto w-full dark:block"
                loading="lazy"
              />
            </div>
          </div>

          <!-- Feature Highlights -->
          <div class="mt-12 grid grid-cols-1 gap-6 md:grid-cols-3">
            {#each [{ icon: 'M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z', title: '美观的编辑器', desc: '富文本格式化，界面直观无干扰' }, { icon: 'M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z', title: '智能日历', desc: '追踪你的写作连续天数，轻松浏览记录' }, { icon: 'M12 18h.01M8 21h8a2 2 0 002-2V5a2 2 0 00-2-2H8a2 2 0 00-2 2v14a2 2 0 002 2z', title: '响应式设计', desc: '在桌面、平板和移动设备上均有完美体验' }] as hl, i (i)}
              <div
                class="rounded-xl border border-border/30 bg-card/50 p-6 text-center transition-all duration-300 hover:border-primary/20 hover:shadow-md"
                class:animate-fade-in={visibleSections.has(0)}
                style:opacity={visibleSections.has(0) ? 1 : 0}
                style:animation-delay="{(i + 1) * 100}ms"
              >
                <div
                  class="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-primary/10"
                >
                  <svg
                    class="h-6 w-6 text-primary"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d={hl.icon}
                    />
                  </svg>
                </div>
                <h3 class="mb-2 font-semibold text-foreground">{hl.title}</h3>
                <p class="text-sm text-muted-foreground">{hl.desc}</p>
              </div>
            {/each}
          </div>

          <p class="mt-6 text-center text-sm text-muted-foreground">
            在所有设备上提供干净、无干扰的写作体验
          </p>
        </div>
      </section>

      <!-- Features Section -->
      <section id="features" class="container-responsive py-20" data-observe data-idx="1">
        <div class="w-full">
          <div
            class="mb-16 text-center"
            class:animate-fade-in={visibleSections.has(1)}
            style:opacity={visibleSections.has(1) ? 1 : 0}
          >
            <h2 class="mb-4 text-3xl font-bold text-foreground sm:text-4xl">写日记所需的一切</h2>
            <p class="mx-auto max-w-2xl text-lg text-muted-foreground">
              强大的功能设计，让每日写作轻松而有意义。
            </p>
          </div>

          <div class="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-3">
            {#each features as feature, i (i)}
              <div
                class="group rounded-xl border border-border/50 bg-card p-6 transition-all duration-300 hover:border-primary/30 hover:shadow-lg"
                class:animate-fade-in={visibleSections.has(1)}
                style:opacity={visibleSections.has(1) ? 1 : 0}
                style:animation-delay="{i * 80}ms"
              >
                <div class="mb-4 text-4xl transition-transform duration-300 group-hover:scale-110">
                  {feature.icon}
                </div>
                <h3 class="mb-2 text-xl font-semibold text-foreground">{feature.title}</h3>
                <p class="text-muted-foreground">{feature.description}</p>
              </div>
            {/each}
          </div>
        </div>
      </section>

      <!-- AI Assistant Preview -->
      <section class="bg-muted/30 py-20" data-observe data-idx="2">
        <div class="container-responsive">
          <div class="grid items-center gap-12 lg:grid-cols-2">
            <div
              class:animate-slide-up={visibleSections.has(2)}
              style:opacity={visibleSections.has(2) ? 1 : 0}
            >
              <h2 class="mb-6 text-3xl font-bold text-foreground sm:text-4xl">
                你的 AI 驱动反思伙伴
              </h2>
              <p class="mb-6 text-lg text-muted-foreground">
                吾身的智能助手会阅读你的日记记录，帮助你发现模式、 获得洞察，并反思个人成长旅程。
              </p>
              <ul class="space-y-4">
                {#each ['询问过去记录的相关问题', '获取个性化写作提示', '发现情绪模式与趋势', '私密且安全的对话'] as item (item)}
                  <li class="flex items-start gap-3">
                    <span class="mt-0.5 text-xl text-primary">✓</span>
                    <span class="text-foreground">{item}</span>
                  </li>
                {/each}
              </ul>
            </div>
            <div
              class="relative"
              class:animate-slide-in-right={visibleSections.has(2)}
              style:opacity={visibleSections.has(2) ? 1 : 0}
            >
              <div class="overflow-hidden rounded-2xl border border-border/50 bg-card shadow-xl">
                <div class="border-b border-border/50 bg-secondary/30 px-4 py-3">
                  <span class="font-medium text-foreground">AI 助手</span>
                </div>
                <div class="min-h-[300px] space-y-4 p-4">
                  <div class="flex gap-3">
                    <div
                      class="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-primary/20 text-sm"
                    >
                      🤖
                    </div>
                    <div class="flex-1 rounded-lg bg-muted/50 p-3">
                      <p class="text-sm text-foreground">
                        基于你最近的记录，我注意到本周你感觉更加精力充沛。是否愿意探索一下可能带来这一积极变化的因素？
                      </p>
                    </div>
                  </div>
                  <div class="flex justify-end gap-3">
                    <div class="max-w-[80%] rounded-lg bg-primary/10 p-3">
                      <p class="text-sm text-foreground">是的，我很想更好地了解这一点！</p>
                    </div>
                  </div>
                  <div class="flex gap-3">
                    <div
                      class="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-primary/20 text-sm"
                    >
                      🤖
                    </div>
                    <div class="flex-1 rounded-lg bg-muted/50 p-3">
                      <p class="text-sm text-foreground">
                        回顾你过去两周的记录，我发现你开始了晨间例行活动，并且在锻炼方面更加一致……
                      </p>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <!-- CTA Section -->
      <section class="container-responsive py-20" data-observe data-idx="3">
        <div
          class="mx-auto max-w-3xl text-center"
          class:animate-fade-in={visibleSections.has(3)}
          style:opacity={visibleSections.has(3) ? 1 : 0}
        >
          <h2 class="mb-6 text-3xl font-bold text-foreground sm:text-4xl">立即开始你的日记旅程</h2>
          <p class="mb-8 text-lg text-muted-foreground">
            加入成千上万使用「吾身」记录日常想法并通过反思成长的人们。
          </p>
          <a
            href={resolve('/login')}
            class="inline-block rounded-xl bg-primary px-8 py-4 text-lg font-medium text-primary-foreground shadow-lg transition-all hover:opacity-90 hover:shadow-xl"
          >
            创建你的免费账户
          </a>
          <p class="mt-4 text-sm text-muted-foreground">无需信用卡，你的数据始终私密。</p>
        </div>
      </section>
    </main>
  </div>
{/if}

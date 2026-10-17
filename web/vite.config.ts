import { sveltekit } from '@sveltejs/kit/vite';
import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';
import { defineConfig } from 'vite';
import { VitePWA } from 'vite-plugin-pwa';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes('node_modules/marked')) {
            return 'marked';
          }
        }
      }
    }
  },
  plugins: [
    tailwindcss(),
    // SvelteKit 3 起不再读取 svelte.config.js，全部配置内联在此。
    sveltekit({
      preprocess: vitePreprocess(),
      adapter: adapter({
        pages: 'build',
        assets: 'build',
        fallback: 'index.html',
        // adapter-static 4 起precompress 从字符串枚举改为 boolean：
        // true 同时产出 .br 与 .gz（原先 'brotli' 只出 .br）。
        precompress: true,
        strict: true
      })
    }),
    // PWA：这里只借用插件提供 `virtual:pwa-register` 虚拟模块，因此用 `disable` 关掉它
    // 自身的全部生成逻辑（sw、manifest、注入脚本）；实际 sw 由 scripts/build-sw.mjs 在
    // vite build 之后生成。
    // 原因：Kit 3 移除了 SSR 构建阶段，@vite-pwa/sveltekit 的 sw 生成挂不上；且
    // adapter-static 收尾会 `rm -rf build` 再重写，把插件已生成的 sw 一并清掉，
    // 预缓存清单还会因 build/ 当时为空而恒为 0 条。
    VitePWA({
      disable: true,
      mode: 'production'
    })
  ],
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8090',
        changeOrigin: true
      },
      '/_': {
        target: 'http://localhost:8090',
        changeOrigin: true
      }
    }
  }
});

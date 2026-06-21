import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';
import { SvelteKitPWA } from '@vite-pwa/sveltekit';

// 使用构建时间戳作为 index.html 的 revision，
// 确保每次部署都会刷新 Service Worker 对离线入口页面的缓存。
const BUILD_REVISION = Date.now().toString();

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
		sveltekit(),
		SvelteKitPWA({
			srcDir: './src',
			mode: 'production',
			strategies: 'generateSW',
			scope: '/',
			base: '/',
			selfDestroying: false,
			manifest: {
				name: 'Diarum - Personal Diary',
				short_name: 'Diarum',
				description: 'A simple, elegant, and self-hosted diary application with AI-powered insights.',
				lang: 'zh-CN',
				dir: 'ltr',
				theme_color: '#F7F3E8',
				background_color: '#F7F3E8',
				display: 'standalone',
				orientation: 'portrait-primary',
				scope: '/',
				start_url: '/',
				categories: ['productivity', 'lifestyle'],
				icons: [
					{
						src: '/favicon.svg',
						sizes: 'any',
						type: 'image/svg+xml',
						purpose: 'any'
					},
					{
						src: '/favicon.svg',
						sizes: 'any',
						type: 'image/svg+xml',
						purpose: 'maskable'
					},
					{
						src: '/android-chrome-192x192.png',
						sizes: '192x192',
						type: 'image/png',
						purpose: 'any'
					},
					{
						src: '/android-chrome-192x192.png',
						sizes: '192x192',
						type: 'image/png',
						purpose: 'maskable'
					},
					{
						src: '/android-chrome-384x384.png',
						sizes: '384x384',
						type: 'image/png',
						purpose: 'any'
					},
					{
						src: '/android-chrome-512x512.png',
						sizes: '512x512',
						type: 'image/png',
						purpose: 'any'
					},
					{
						src: '/android-chrome-512x512.png',
						sizes: '512x512',
						type: 'image/png',
						purpose: 'maskable'
					}
				],
				shortcuts: [
					{
						name: '新建今日日记',
						short_name: '今日',
						description: '快速打开今天的日记编辑页',
						url: '/',
						icons: [{ src: '/android-chrome-192x192.png', sizes: '192x192', type: 'image/png' }]
					}
				],
				screenshots: [
					{
						src: '/screenshots/mobile-light.png',
						sizes: '930x1734',
						type: 'image/png',
						form_factor: 'narrow',
						label: 'Mobile view - Light theme'
					},
					{
						src: '/screenshots/mobile-dark.png',
						sizes: '924x1734',
						type: 'image/png',
						form_factor: 'narrow',
						label: 'Mobile view - Dark theme'
					},
					{
						src: '/screenshots/desktop-light.png',
						sizes: '2522x2012',
						type: 'image/png',
						form_factor: 'wide',
						label: 'Desktop view - Light theme'
					},
					{
						src: '/screenshots/desktop-dark.png',
						sizes: '2544x2018',
						type: 'image/png',
						form_factor: 'wide',
						label: 'Desktop view - Dark theme'
					}
				]
			},
			injectManifest: {
				globPatterns: ['**/*.{js,css,html,ico,png,svg,webp,woff,woff2}']
			},
			workbox: {
				globPatterns: ['**/*.{js,css,html,ico,png,svg,webp,woff,woff2}'],
				cleanupOutdatedCaches: true,
				clientsClaim: true,
				skipWaiting: false, // 不自动 skipWaiting：等用户点击"立即更新"后由 updateSW(true) 触发
				// adapter-static produces a single `index.html` at the site root.
				// @vite-pwa/sveltekit's default `globDirectory` is SvelteKit's
				// `output/client/`, which does not contain it, so we inject
				// it into the precache manifest here and point all offline
				// navigation requests back at it. Without this, navigating
				// while offline returns a browser error page ("空白页面").
				// 使用构建时间戳作为 revision，确保每次部署都会刷新缓存。
				additionalManifestEntries: [
					{ url: '/index.html', revision: BUILD_REVISION }
				],
				navigateFallback: '/index.html',
				// 严格控制离线回退：只对常规的内容/编辑路径使用入口 HTML，
				// 排除 API、认证端点、SvelteKit 内部端点等非导航请求。
				navigateFallbackAllowlist: [/^(?!\/(?:api|auth|collections|_)(?:\/|$))\/.*$/],
				runtimeCaching: [
					{
						urlPattern: /^https:\/\/fonts\.googleapis\.com\/.*/i,
						handler: 'CacheFirst',
						options: {
							cacheName: 'google-fonts-cache',
							expiration: {
								maxEntries: 10,
								maxAgeSeconds: 60 * 60 * 24 * 365 // 365 days
							},
							cacheableResponse: {
								statuses: [0, 200]
							}
						}
					},
					{
						urlPattern: /^https:\/\/fonts\.gstatic\.com\/.*/i,
						handler: 'CacheFirst',
						options: {
							cacheName: 'gstatic-fonts-cache',
							expiration: {
								maxEntries: 10,
								maxAgeSeconds: 60 * 60 * 24 * 365 // 365 days
							},
							cacheableResponse: {
								statuses: [0, 200]
							}
						}
					},
					{
						// 认证/登录端点必须走网络，绝不能被离线回退或缓存影响。
						urlPattern: /\/(?:api\/auth|auth)\/.*/i,
						handler: 'NetworkOnly',
						options: {
							cacheName: 'auth-endpoints'
						}
					},
					{
						// SvelteKit 内部端点（_app、collections 等）使用 NetworkFirst，
						// 以确保数据新鲜，同时在离线时可以读取上次缓存。
						urlPattern: /\/_(?:\/|$).*/i,
						handler: 'NetworkFirst',
						options: {
							cacheName: 'internal-endpoints',
							networkTimeoutSeconds: 5,
							expiration: {
								maxEntries: 20,
								maxAgeSeconds: 60 * 60 * 24 // 1 day
							},
							cacheableResponse: {
								statuses: [0, 200]
							}
						}
					},
					{
						urlPattern: /\/api\/.*/i,
						handler: 'NetworkFirst',
						options: {
							cacheName: 'api-cache',
							networkTimeoutSeconds: 10,
							expiration: {
								maxEntries: 50,
								maxAgeSeconds: 60 * 60 * 24 * 7 // 7 days
							},
							cacheableResponse: {
								statuses: [0, 200]
							}
						}
					},
					{
						// 同源构建产物（带 hash 的静态资源）：一旦缓存就长期不变。
						urlPattern: /\/_app\/immutable\/.*/i,
						handler: 'CacheFirst',
						options: {
							cacheName: 'immutable-assets',
							expiration: {
								maxEntries: 100,
								maxAgeSeconds: 60 * 60 * 24 * 365 // 365 days
							},
							cacheableResponse: {
								statuses: [0, 200]
							}
						}
					},
					{
						// 用户上传内容与站点图片：后台刷新，不阻塞离线访问。
						urlPattern: /\.(?:png|jpg|jpeg|webp|gif|svg|ico)(?:\?.*)?$/i,
						handler: 'StaleWhileRevalidate',
						options: {
							cacheName: 'image-assets',
							expiration: {
								maxEntries: 80,
								maxAgeSeconds: 60 * 60 * 24 * 30 // 30 days
							},
							cacheableResponse: {
								statuses: [0, 200]
							}
						}
					}
				]
			},
			devOptions: {
				enabled: process.env.VITE_ENABLE_SW === 'true',
				suppressWarnings: true,
				type: 'module'
			}
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

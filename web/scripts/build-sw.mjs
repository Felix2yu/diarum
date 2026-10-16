/**
 * 在 `vite build` 之后生成 Service Worker。
 *
 * 为什么不能直接用 vite-plugin-pwa 的构建期钩子：
 *   1. SvelteKit 3 移除了 SSR 构建阶段，而插件的 sw 生成挂在该阶段
 *      （这也是 @vite-pwa/sveltekit@1.1.0 无法用于 Kit 3 的原因）。
 *   2. 即使换成通用 vite-plugin-pwa，它也在 adapter-static 收尾之前运行；
 *      而 adapter 收尾会 `rm -rf build` 再重写，把插件已生成的 sw 一并清掉，
 *      预缓存清单也会因为 build/ 当时为空而恒为 0 条。
 *   实测证据：precache 1 entries (0.00 KiB)，sw.js 被写到 dist/ 而非 build/。
 *
 * 因此改为构建完成后单独调用 workbox-build 的 generateSW，
 * 此时 build/ 已有完整产物，预缓存清单才是真实的。
 */
import { generateSW } from 'workbox-build';

const BUILD_DIR = 'build';

// 与 vite.config.ts 中注入 index.html 时使用的 revision 保持一致的思路：
// 用构建时间戳保证每次部署都会让旧缓存失效。
const BUILD_REVISION = Date.now().toString();

const result = await generateSW({
	swDest: `${BUILD_DIR}/sw.js`,
	globDirectory: BUILD_DIR,
	globPatterns: ['**/*.{js,css,html,ico,png,svg,webp,woff,woff2}'],
	// sw 自身与 workbox 运行时不能进预缓存，否则版本升级会自锁。
	globIgnores: ['sw.js', 'workbox-*.js'],
	// adapter-static 只产出一个根部 index.html，需显式补进预缓存清单，
	// 否则离线导航会落到浏览器错误页。
	additionalManifestEntries: [{ url: '/index.html', revision: BUILD_REVISION }],
	navigateFallback: '/index.html',
	// 严格控制离线回退：只对常规的内容/编辑路径使用入口 HTML，
	// 排除 API、认证端点、SvelteKit 内部端点等非导航请求。
	navigateFallbackAllowlist: [/^(?!\/(?:api|auth|collections|_)(?:\/|$))\/.*$/],
	cleanupOutdatedCaches: true,
	clientsClaim: true,
	// 不自动 skipWaiting：等用户点击"立即更新"后由 updateSW(true) 触发，
	// 避免在编辑日记时被强制刷新打断。
	skipWaiting: false,
	runtimeCaching: [
		{
			urlPattern: /^https:\/\/fonts\.googleapis\.com\/.*/i,
			handler: 'CacheFirst',
			options: {
				cacheName: 'google-fonts-cache',
				expiration: { maxEntries: 10, maxAgeSeconds: 60 * 60 * 24 * 365 },
				cacheableResponse: { statuses: [0, 200] }
			}
		},
		{
			urlPattern: /^https:\/\/fonts\.gstatic\.com\/.*/i,
			handler: 'CacheFirst',
			options: {
				cacheName: 'gstatic-fonts-cache',
				expiration: { maxEntries: 10, maxAgeSeconds: 60 * 60 * 24 * 365 },
				cacheableResponse: { statuses: [0, 200] }
			}
		},
		{
			// 认证/登录端点必须走网络，绝不能被离线回退或缓存影响。
			urlPattern: /\/(?:api\/auth|auth)\/.*/i,
			handler: 'NetworkOnly',
			options: { cacheName: 'auth-endpoints' }
		},
		{
			// SvelteKit 内部端点（_app 等）使用 NetworkFirst，
			// 以确保数据新鲜，同时在离线时可以读取上次缓存。
			urlPattern: /\/_(?:\/|$).*/i,
			handler: 'NetworkFirst',
			options: {
				cacheName: 'internal-endpoints',
				networkTimeoutSeconds: 5,
				expiration: { maxEntries: 20, maxAgeSeconds: 60 * 60 * 24 },
				cacheableResponse: { statuses: [0, 200] }
			}
		},
		{
			urlPattern: /\/api\/.*/i,
			handler: 'NetworkFirst',
			options: {
				cacheName: 'api-cache',
				networkTimeoutSeconds: 10,
				expiration: { maxEntries: 50, maxAgeSeconds: 60 * 60 * 24 * 7 },
				cacheableResponse: { statuses: [0, 200] }
			}
		},
		{
			// 同源构建产物（带 hash 的静态资源）：一旦缓存就长期不变。
			urlPattern: /\/_app\/immutable\/.*/i,
			handler: 'CacheFirst',
			options: {
				cacheName: 'immutable-assets',
				expiration: { maxEntries: 100, maxAgeSeconds: 60 * 60 * 24 * 365 },
				cacheableResponse: { statuses: [0, 200] }
			}
		},
		{
			// 用户上传内容与站点图片：后台刷新，不阻塞离线访问。
			urlPattern: /\.(?:png|jpg|jpeg|webp|gif|svg|ico)(?:\?.*)?$/i,
			handler: 'StaleWhileRevalidate',
			options: {
				cacheName: 'image-assets',
				expiration: { maxEntries: 80, maxAgeSeconds: 60 * 60 * 24 * 30 },
				cacheableResponse: { statuses: [0, 200] }
			}
		}
	]
});

const count = result.count ?? result.size;
const size = ((result.size ?? 0) / 1024).toFixed(1);
console.log(`[build-sw] precache ${count} entries (${size} KiB) -> build/sw.js`);

if (!count) {
	console.error('[build-sw] 预缓存清单为空，说明 build/ 目录内容不完整，构建视为失败');
	process.exit(1);
}
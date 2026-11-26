import { pb } from '#lib/api/client.js';

const MEDIA_PREFIX = '/api/v1/files/media/';

/**
 * 为 Markdown 预览中的本地媒体图片补充访问 token。
 * 日记正文只保存无 token 的稳定 URL，避免 token 过期后内容失效。
 */
export function mediaAuth(node: HTMLElement) {
  function apply() {
    const token = pb.authStore.token;
    if (!token) return;
    for (const image of node.querySelectorAll<HTMLImageElement>('img[src]')) {
      const src = image.getAttribute('src') || '';
      if (!src.startsWith(MEDIA_PREFIX)) continue;
      const url = new URL(src, window.location.origin);
      if (url.searchParams.get('token') === token) continue;
      url.searchParams.set('token', token);
      image.src = `${url.pathname}${url.search}`;
    }
  }

  apply();
  const observer = new MutationObserver(apply);
  observer.observe(node, { childList: true, subtree: true });

  return {
    destroy() {
      observer.disconnect();
    }
  };
}

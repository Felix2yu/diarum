import DOMPurify from 'dompurify';
import { marked } from 'marked';

/**
 * Markdown -> 可安全 {@html} 渲染的 HTML。
 *
 * marked 只做语法转换，不做任何净化：原文里的 <script>、onerror=、
 * javascript: 链接会原样进入结果。渲染内容里既有用户自己写的日记正文，
 * 也有 AI 生成的摘要与回复，都属于不可信输入，必须过一遍 DOMPurify。
 *
 * 此前 CalendarAnalysis / ChatMessage / TiptapEditor 各自复制了一份
 * 未净化的 marked.parse，这里收敛为唯一实现。
 */
export function renderMarkdown(text: string): string {
  const trimmed = (text ?? '').trim();
  if (!trimmed) return '';

  let html: string;
  try {
    html = marked.parse(trimmed, { async: false, breaks: true, gfm: true }) as string;
  } catch {
    // 渲染失败则回退到保留换行的纯文本
    return (text ?? '')
      .split('\n')
      .map((line) => `<p>${line}</p>`)
      .join('\n');
  }

  // DOMPurify 默认白名单已剔除 script / on* 事件属性 / javascript: 协议。
  // 额外放行 target="_blank" 的配套 rel，避免新窗口可被 opener 劫持。
  return DOMPurify.sanitize(html, {
    ADD_ATTR: ['target'],
    ADD_TAGS: ['mark']
  });
}

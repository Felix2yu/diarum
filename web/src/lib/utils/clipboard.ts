/**
 * 复制文本到剪贴板。
 *
 * 优先使用 Clipboard API；自托管场景常用 http://192.168.x.x 访问，
 * 此时属于非安全上下文，navigator.clipboard 不可用，需要回退到
 * 临时 textarea + document.execCommand('copy')。
 */
export async function copyText(text: string): Promise<boolean> {
  if (!text) {
    return false;
  }

  if (typeof navigator !== 'undefined' && navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch {
      // 权限被拒或非安全上下文，继续走回退方案
    }
  }

  if (typeof document === 'undefined') {
    return false;
  }

  const textarea = document.createElement('textarea');
  textarea.value = text;
  textarea.setAttribute('readonly', '');
  textarea.style.position = 'fixed';
  textarea.style.top = '0';
  textarea.style.left = '0';
  textarea.style.opacity = '0';
  textarea.style.pointerEvents = 'none';
  document.body.appendChild(textarea);

  try {
    textarea.select();
    textarea.setSelectionRange(0, textarea.value.length);
    return document.execCommand('copy');
  } catch {
    return false;
  } finally {
    document.body.removeChild(textarea);
  }
}

/**
 * 复制文本到剪贴板，支持 Clipboard API 和 execCommand 回退
 */
export async function copyToClipboard(text: string): Promise<boolean> {
  if (!text) return false

  // 优先使用 Clipboard API（需要 HTTPS 或 localhost）
  if (navigator.clipboard && window.isSecureContext) {
    try {
      await navigator.clipboard.writeText(text)
      return true
    } catch {
      // 回退到 execCommand
    }
  }

  // 回退方案：使用 execCommand
  try {
    const textarea = document.createElement('textarea')
    textarea.value = text
    textarea.style.position = 'fixed'
    textarea.style.left = '-9999px'
    textarea.style.top = '-9999px'
    document.body.appendChild(textarea)
    textarea.focus()
    textarea.select()
    const success = document.execCommand('copy')
    document.body.removeChild(textarea)
    return success
  } catch {
    return false
  }
}

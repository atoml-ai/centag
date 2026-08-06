/**
 * 复制文本到剪贴板。
 * - HTTPS / localhost：优先 Clipboard API
 * - HTTP 局域网（非 secure context）：回退 execCommand
 * 注意：调用方应在用户点击同步链路上调用；勿在 dynamic import / 长 await 之后再复制。
 */
export async function copyToClipboard(text: string): Promise<boolean> {
  if (!text) return false

  if (typeof navigator !== 'undefined' && navigator.clipboard && window.isSecureContext) {
    try {
      await navigator.clipboard.writeText(text)
      return true
    } catch {
      // fall through
    }
  }

  return copyWithExecCommand(text)
}

function copyWithExecCommand(text: string): boolean {
  try {
    const textarea = document.createElement('textarea')
    textarea.value = text
    textarea.setAttribute('readonly', '')
    textarea.setAttribute('aria-hidden', 'true')
    // 部分浏览器会拒绝完全移出视口的节点；用近透明覆盖代替 left:-9999px
    Object.assign(textarea.style, {
      position: 'fixed',
      top: '0',
      left: '0',
      width: '1px',
      height: '1px',
      padding: '0',
      margin: '0',
      border: 'none',
      outline: 'none',
      boxShadow: 'none',
      opacity: '0',
      zIndex: '-1',
    })
    document.body.appendChild(textarea)
    textarea.focus({ preventScroll: true })
    textarea.select()
    textarea.setSelectionRange(0, textarea.value.length)
    const success = document.execCommand('copy')
    document.body.removeChild(textarea)
    return success
  } catch {
    return false
  }
}

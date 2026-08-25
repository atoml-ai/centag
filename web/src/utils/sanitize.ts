import DOMPurify from 'dompurify'

// 允许渲染的标签白名单：覆盖 formatMessage/renderMarkdown 产出的
// <br>/<strong>/<em>/<code>/<pre class="code-block">，以及常见 Markdown 结构。
const ALLOWED_TAGS = [
  'p', 'br', 'ul', 'ol', 'li',
  'code', 'pre', 'strong', 'em', 'b', 'i',
  'a', 'blockquote',
  'h1', 'h2', 'h3', 'h4', 'h5', 'h6',
  'table', 'thead', 'tbody', 'tr', 'td', 'th',
]

// class 仅用于保留代码块样式（code-block）；style/on* 一律禁止，
// javascript:/data: 等危险协议由 DOMPurify 默认 URI 校验拦截。
const ALLOWED_ATTR = ['href', 'title', 'target', 'class']

export function sanitizeHtml(dirty: string): string {
  if (!dirty) return ''
  return DOMPurify.sanitize(dirty, {
    ALLOWED_TAGS,
    ALLOWED_ATTR,
    FORBID_ATTR: ['style'],
  })
}

/** How clients route requests through Centag pipelines. */
export type ChatAccessMode = 'default' | 'keyword' | 'model' | 'header'

export const BUILTIN_SHORTCUTS: { code: string; label: string; type: '' | 'info' | 'success' | 'warning' | 'danger' | 'primary' }[] = [
  { code: '#d', label: '指定后端', type: 'info' },
  { code: '#s', label: '智能调度', type: 'success' },
  { code: '#m', label: '模型匹配', type: 'warning' },
  { code: '#c', label: '意图分类', type: 'danger' },
  { code: '#t', label: '透明代理', type: '' },
  { code: '#a', label: '审核模式', type: 'primary' },
  { code: '#f', label: '降级容错', type: 'info' },
  { code: '#o', label: '优化模式', type: 'success' }
]

export const KEYWORD_TO_MODE: Record<string, string> = Object.fromEntries(
  BUILTIN_SHORTCUTS.map((s) => [s.code, s.code])
)

export const MODE_NAMES: Record<string, string> = {
  '#d': '指定后端 (#d)',
  '#s': '智能调度 (#s)',
  '#m': '模型匹配 (#m)',
  '#c': '意图分类 (#c)',
  '#t': '透明代理 (#t)',
  '#a': '审核模式 (#a)',
  '#f': '降级容错 (#f)',
  '#o': '优化模式 (#o)',
  '#mem0': 'Mem0 记忆 (#mem0)',
  pipeline: '流水线',
  default: '默认流水线'
}

export const MODE_TAG_TYPES: Record<string, string> = {
  '#d': 'info',
  '#s': 'success',
  '#m': 'warning',
  '#c': 'danger',
  '#t': '',
  '#a': 'primary',
  '#f': 'info',
  '#o': 'success',
  pipeline: 'primary',
  default: 'success'
}

export function getModeTagType(mode: string): string {
  return MODE_TAG_TYPES[mode] || 'info'
}

/** OpenAI-compatible model field that routes to a pipeline; nodes supply backend/model. */
export function buildPipelineModelField(pipelineId: string): string {
  const id = pipelineId.trim()
  return id ? `pipeline.${id}` : 'auto'
}

export type PipelineShortcut = { code: string; pipelineId?: string }

/** Merge built-in and pipeline shortcut codes for keyword detection. */
export function buildKeywordToModeMap(
  pipelineShortcuts: PipelineShortcut[] = []
): Record<string, string> {
  const map: Record<string, string> = { ...KEYWORD_TO_MODE }
  for (const item of pipelineShortcuts) {
    const code = (item.code || '').trim()
    if (code) map[code] = code
  }
  return map
}

function isShortcutBoundary(content: string, keyword: string): boolean {
  const next = content[keyword.length]
  return next === undefined || next === ' ' || next === '\n' || next === '\t' || next === '\r'
}

/** Parse leading shortcut code; longest match wins (#mem0 before #m). */
export function extractProxyMode(
  content: string,
  keywordToModeMap: Record<string, string> = KEYWORD_TO_MODE
): { content: string; proxyMode: string | null } {
  const keywords = Object.keys(keywordToModeMap).sort((a, b) => b.length - a.length)
  for (const keyword of keywords) {
    if (!content.startsWith(keyword) || !isShortcutBoundary(content, keyword)) continue
    const remaining = content.slice(keyword.length).replace(/^\s+/, '')
    return { content: remaining, proxyMode: keywordToModeMap[keyword] }
  }
  return { content, proxyMode: null }
}
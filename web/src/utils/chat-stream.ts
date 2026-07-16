/** Extract assistant text from an OpenAI-style stream chunk (delta or message). */
export function extractStreamDeltaContent(choice: Record<string, unknown> | undefined): string {
  if (!choice) return ''

  const delta = choice.delta
  if (delta && typeof delta === 'object') {
    const text = extractContentField((delta as Record<string, unknown>).content)
    if (text) return text
  }

  const message = choice.message
  if (message && typeof message === 'object') {
    return extractContentField((message as Record<string, unknown>).content)
  }

  return ''
}

function extractContentField(value: unknown): string {
  if (value == null) return ''
  if (typeof value === 'string') return value
  if (Array.isArray(value)) {
    let out = ''
    for (const item of value) {
      if (item && typeof item === 'object') {
        const text = (item as Record<string, unknown>).text
        if (typeof text === 'string') out += text
      }
    }
    return out
  }
  return ''
}

/** True when chunk only signals stream start (role) without finish_reason. */
export function isStreamRolePlaceholder(choice: Record<string, unknown> | undefined): boolean {
  if (!choice?.delta || typeof choice.delta !== 'object') return false
  const delta = choice.delta as Record<string, unknown>
  if (choice.finish_reason != null) return false
  const role = delta.role
  const hasRole = typeof role === 'string' && role.length > 0
  const hasContent = extractContentField(delta.content).length > 0
  return hasRole && !hasContent
}
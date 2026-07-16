import { MODE_NAMES } from '@/utils/chat-access'

/** Request routing placeholders — not the LLM model executed by pipeline nodes. */
export function isRoutingModel(model?: string): boolean {
  const m = (model || '').trim()
  if (!m || m === 'auto') return true
  return m.startsWith('pipeline.') || m.startsWith('pipeline_')
}

export function resolveDisplayModel(meta?: Record<string, any> | null): string {
  if (!meta) return '—'
  const model = (meta.model || '').trim()
  if (model && !isRoutingModel(model)) return model
  const executor = (meta.executorModel || '').trim()
  if (executor) return executor
  return '—'
}

export function resolveDisplayBackend(meta?: Record<string, any> | null): string {
  if (!meta) return '—'
  return (meta.backendName || meta.backendId || '').trim() || '—'
}

export function mergeCentagMeta(
  base: Record<string, any> | undefined,
  centag: Record<string, any> | undefined
): Record<string, any> {
  const merged = { ...(base || {}) }
  if (!centag) return merged

  if (centag.model) merged.model = centag.model
  if (centag.backend_id) {
    merged.backendId = centag.backend_id
    if (!merged.backendName) merged.backendName = centag.backend_id
  }
  if (centag.pipeline_id) {
    merged.pipelineId = centag.pipeline_id
    merged.proxyMode = 'pipeline'
  }
  if (centag.pipeline_duration_ms != null) {
    merged.pipelineDuration = String(centag.pipeline_duration_ms)
  }
  return merged
}

export function extractResponseMeta(headers: Headers): Record<string, any> {
  const cacheStatus = headers.get('x-cache') || headers.get('X-Cache') || ''
  const cacheHit = cacheStatus.startsWith('HIT')

  const rawBackendName = headers.get('x-backend-name') || headers.get('X-Backend-Name') || ''
  let backendName = ''
  try {
    backendName = rawBackendName ? decodeURIComponent(rawBackendName) : ''
  } catch {
    backendName = rawBackendName
  }

  const pipelineId = headers.get('x-pipeline-id') || headers.get('X-Pipeline-ID') || ''
  const pipelineDuration = headers.get('x-pipeline-duration-ms') || headers.get('X-Pipeline-Duration-Ms') || ''
  const pipelineExecuted = headers.get('x-pipeline-executed') || headers.get('X-Pipeline-Executed') || ''

  const splitTotal = parseInt(headers.get('x-split-sub-questions') || headers.get('X-Split-Sub-Questions') || '0', 10)
  const splitHits = parseInt(headers.get('x-split-cache-hits') || headers.get('X-Split-Cache-Hits') || '0', 10)

  const auditorBackend = headers.get('x-auditor-backend') || headers.get('X-Auditor-Backend') || ''
  const auditorModel = headers.get('x-auditor-model') || headers.get('X-Auditor-Model') || ''
  const auditPassed = headers.get('x-audit-passed') || headers.get('X-Audit-Passed') || ''
  const auditScore = headers.get('x-audit-score') || headers.get('X-Audit-Score') || ''
  const auditFeedback = headers.get('x-audit-feedback') || headers.get('X-Audit-Feedback') || ''

  const analyzerBackend = headers.get('x-analyzer-backend') || headers.get('X-Analyzer-Backend') || ''
  const analyzerModel = headers.get('x-analyzer-model') || headers.get('X-Analyzer-Model') || ''
  const matchStrategy = headers.get('x-match-strategy') || headers.get('X-Match-Strategy') || ''

  const proxyMode = (headers.get('x-proxy-mode') || headers.get('X-Proxy-Mode') || '').trim()
  const headerModel = (headers.get('x-model') || headers.get('X-Model') || '').trim()
  const backendId = headers.get('x-backend-id') || headers.get('X-Backend-ID') || ''
  const executorModel = headers.get('x-executor-model') || headers.get('X-Executor-Model') || ''

  const model = headerModel && !isRoutingModel(headerModel) ? headerModel : executorModel || ''

  const optimizerBackend = headers.get('x-optimizer-backend') || headers.get('X-Optimizer-Backend') || ''
  const optimizerModel = headers.get('x-optimizer-model') || headers.get('X-Optimizer-Model') || ''
  const optimizeApplied = headers.get('x-optimize-applied') || headers.get('X-Optimize-Applied') || ''

  const resolvedProxyMode = proxyMode || (pipelineId ? 'pipeline' : '')
  const modeLabel =
    MODE_NAMES[resolvedProxyMode] ||
    (pipelineId ? MODE_NAMES.pipeline : '') ||
    resolvedProxyMode

  return {
    backendName: backendName || backendId,
    backendId,
    model,
    proxyMode: resolvedProxyMode,
    pipelineId,
    pipelineDuration,
    pipelineExecuted,
    modeLabel,
    cacheStatus,
    cacheHit,
    splitTotal,
    splitHits,
    auditorBackend,
    auditorModel,
    auditPassed,
    auditScore,
    auditFeedback,
    analyzerBackend,
    analyzerModel,
    matchStrategy,
    executorModel,
    optimizerBackend,
    optimizerModel,
    optimizeApplied
  }
}
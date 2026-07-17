import type { AgentPatternPipeline, PipelineNodeConfig } from '../api/pipeline'

export const SYSTEM_DEFAULT_BACKEND = '{{system.default_backend}}'
export const SYSTEM_DEFAULT_MODEL = '{{system.default_model}}'

export type CapabilitySlotKind = 'route' | 'role' | 'stage'

/** metadata.capability_slots[] 契约 */
export interface CapabilitySlot {
  slot_id: string
  node_id: string
  label: string
  kind?: CapabilitySlotKind
  tags?: string[]
  hint?: string
  default_follow_system?: boolean
}

export interface CapabilitySlotRow {
  slotId: string
  nodeId: string
  label: string
  hint: string
  tags: string[]
  kind: CapabilitySlotKind
  followSystem: boolean
  backend: string
  model: string
}

export type SlotResolveSource = 'capability_slots' | 'route_model_targets' | 'router_defaults' | 'discover'

export interface SlotResolveResult {
  slots: CapabilitySlot[]
  source: SlotResolveSource
}

/** Legacy router-mode fallback when metadata has no targets yet */
export const DEFAULT_ROUTE_MODEL_TARGETS: CapabilitySlot[] = [
  {
    slot_id: 'code-generator',
    node_id: 'code-generator',
    label: '代码生成',
    hint: '建议绑定强代码能力模型',
    tags: ['code'],
    kind: 'route'
  },
  {
    slot_id: 'translate-gen',
    node_id: 'translate-gen',
    label: '翻译',
    hint: '建议绑定通用/多语言模型',
    tags: ['multilingual'],
    kind: 'route'
  },
  {
    slot_id: 'summary-gen',
    node_id: 'summary-gen',
    label: '摘要',
    hint: '可用更便宜或长上下文模型',
    tags: ['cheap'],
    kind: 'route'
  },
  {
    slot_id: 'chat-generator',
    node_id: 'chat-generator',
    label: '通用对话',
    hint: '日常对话；默认可跟随系统默认',
    tags: ['default'],
    kind: 'route',
    default_follow_system: true
  }
]

export interface BackendLike {
  id?: string
  name?: string
  enabled?: boolean
  supported_models?: Array<string | { id?: string; name?: string }>
}

export interface AddCategoryInput {
  label: string
  keywords: string[]
  nodeId?: string
  routerNodeId?: string
  isDefault?: boolean
  systemPrompt?: string
  tags?: string[]
  appendSlot?: boolean
}

type NodeWithRoute = PipelineNodeConfig & {
  route_config?: {
    router_node_id?: string
    route_value?: string
    is_default?: boolean
  }
}

function asNode(n: PipelineNodeConfig): NodeWithRoute {
  return n as NodeWithRoute
}

function normalizeTags(tags: unknown): string[] {
  if (!Array.isArray(tags) || !tags.length) return ['default']
  return tags.map((t) => String(t)).filter(Boolean)
}

function mapRouteTarget(t: any): CapabilitySlot | null {
  if (!t || typeof t.node_id !== 'string') return null
  return {
    slot_id: String(t.slot_id || t.node_id),
    node_id: String(t.node_id),
    label: String(t.label || t.node_id),
    hint: t.hint ? String(t.hint) : '',
    kind: (t.kind as CapabilitySlotKind) || 'route',
    tags: normalizeTags(t.tags),
    default_follow_system: !!t.default_follow_system
  }
}

export function isRouterModePipeline(pipeline: AgentPatternPipeline | null | undefined): boolean {
  if (!pipeline) return false
  if (pipeline.id === 'router-mode') return true
  const mode = pipeline.metadata?.aligned_proxy_mode
  return mode === 'router-mode' || mode === 'router'
}

/** Collect router target node IDs from config.custom_config.routes values. */
export function collectRouterRouteTargets(pipeline: AgentPatternPipeline): Set<string> {
  const ids = new Set<string>()
  for (const node of pipeline.nodes || []) {
    if (node.type !== 'router') continue
    const routes = node.config?.custom_config?.routes
    if (!routes || typeof routes !== 'object') continue
    for (const target of Object.values(routes)) {
      if (typeof target === 'string' && target.trim()) ids.add(target.trim())
    }
  }
  return ids
}

/**
 * Discover slots from router.routes targets ∪ nodes with route_config.
 * Priority for resolve: capability_slots → route_model_targets → (router defaults) → discover.
 */
export function discoverCapabilitySlots(pipeline: AgentPatternPipeline): CapabilitySlot[] {
  const byId = new Map((pipeline.nodes || []).map((n) => [n.id, n]))
  const allowed = collectRouterRouteTargets(pipeline)
  for (const node of pipeline.nodes || []) {
    const rc = asNode(node).route_config
    if (rc?.router_node_id) allowed.add(node.id)
  }

  const slots: CapabilitySlot[] = []
  for (const nodeId of allowed) {
    const node = byId.get(nodeId)
    if (!node) continue
    // Skip pure router nodes if somehow listed as target of itself
    if (node.type === 'router') continue
    slots.push({
      slot_id: nodeId,
      node_id: nodeId,
      label: node.name || nodeId,
      kind: 'route',
      tags: ['default'],
      hint: ''
    })
  }
  return slots
}

/**
 * Resolve slots with source.
 * 1) metadata.capability_slots (non-empty)
 * 2) metadata.route_model_targets → map
 * 3) router-mode legacy DEFAULT_ROUTE_MODEL_TARGETS
 * 4) auto-discover
 */
export function resolveCapabilitySlotsWithSource(
  pipeline: AgentPatternPipeline | null | undefined
): SlotResolveResult {
  if (!pipeline) return { slots: [], source: 'discover' }

  const fromSlots = pipeline.metadata?.capability_slots
  if (Array.isArray(fromSlots) && fromSlots.length > 0) {
    const slots = fromSlots.map(mapRouteTarget).filter(Boolean) as CapabilitySlot[]
    if (slots.length) return { slots, source: 'capability_slots' }
  }

  const fromTargets = pipeline.metadata?.route_model_targets
  if (Array.isArray(fromTargets) && fromTargets.length > 0) {
    const slots = fromTargets.map(mapRouteTarget).filter(Boolean) as CapabilitySlot[]
    if (slots.length) return { slots, source: 'route_model_targets' }
  }

  if (isRouterModePipeline(pipeline)) {
    return { slots: DEFAULT_ROUTE_MODEL_TARGETS, source: 'router_defaults' }
  }

  return { slots: discoverCapabilitySlots(pipeline), source: 'discover' }
}

export function resolveCapabilitySlots(pipeline: AgentPatternPipeline | null | undefined): CapabilitySlot[] {
  return resolveCapabilitySlotsWithSource(pipeline).slots
}

/** Explicit declaration (≥1) or pure discover (≥2). */
export function canConfigureCapabilitySlots(pipeline: AgentPatternPipeline | null | undefined): boolean {
  const { slots, source } = resolveCapabilitySlotsWithSource(pipeline)
  if (!slots.length) return false
  if (source === 'discover') return slots.length >= 2
  return slots.length >= 1
}

export function isFollowSystemBinding(backend?: string, model?: string): boolean {
  const b = (backend || '').trim()
  const m = (model || '').trim()
  const backendFollow = !b || b === SYSTEM_DEFAULT_BACKEND
  const modelFollow = !m || m === SYSTEM_DEFAULT_MODEL
  return backendFollow && modelFollow
}

export function buildCapabilitySlotRows(pipeline: AgentPatternPipeline): CapabilitySlotRow[] {
  const slots = resolveCapabilitySlots(pipeline)
  const byId = new Map((pipeline.nodes || []).map((n) => [n.id, n]))
  return slots.map((s) => {
    const node = byId.get(s.node_id)
    const backend = node?.backend || node?.config?.backend || ''
    const model = node?.model || node?.config?.model || ''
    const follow = isFollowSystemBinding(backend, model)
    return {
      slotId: s.slot_id,
      nodeId: s.node_id,
      label: s.label,
      hint: s.hint || '',
      tags: s.tags?.length ? s.tags : ['default'],
      kind: s.kind || 'route',
      followSystem: follow,
      backend: follow ? '' : backend,
      model: follow ? '' : model
    }
  })
}

function ensureNodeConfig(node: PipelineNodeConfig): PipelineNodeConfig {
  if (!node.config) {
    node.config = { backend: '', model: '' }
  }
  if (!node.config.template_vars) {
    node.config.template_vars = {}
  }
  return node
}

/** Apply binding rows onto a deep-cloned pipeline; returns the mutated clone. */
export function applyCapabilitySlotBindings(
  pipeline: AgentPatternPipeline,
  rows: CapabilitySlotRow[]
): AgentPatternPipeline {
  const next: AgentPatternPipeline = JSON.parse(JSON.stringify(pipeline))
  const byId = new Map((next.nodes || []).map((n) => [n.id, n]))

  for (const row of rows) {
    const node = byId.get(row.nodeId)
    if (!node) {
      throw new Error(`流水线中找不到节点 ${row.nodeId}`)
    }
    ensureNodeConfig(node)

    if (row.followSystem) {
      node.backend = SYSTEM_DEFAULT_BACKEND
      node.model = SYSTEM_DEFAULT_MODEL
      node.config.backend = SYSTEM_DEFAULT_BACKEND
      node.config.model = SYSTEM_DEFAULT_MODEL
      node.config.template_vars = {
        ...(node.config.template_vars || {}),
        backend: 'system.default_backend',
        model: 'system.default_model'
      }
    } else {
      const backend = (row.backend || '').trim()
      const model = (row.model || '').trim()
      if (!backend) throw new Error(`「${row.label}」请选择后端，或勾选跟随系统默认`)
      if (!model) throw new Error(`「${row.label}」请选择模型，或勾选跟随系统默认`)
      node.backend = backend
      node.model = model
      node.config.backend = backend
      node.config.model = model
      const tv = { ...(node.config.template_vars || {}) }
      delete tv.backend
      delete tv.model
      node.config.template_vars = tv
    }
  }

  return next
}

export function listRouterNodes(pipeline: AgentPatternPipeline): PipelineNodeConfig[] {
  return (pipeline.nodes || []).filter((n) => n.type === 'router')
}

export function slugifyNodeId(label: string): string {
  const raw = (label || '')
    .trim()
    .toLowerCase()
    .replace(/[\s_]+/g, '-')
    .replace(/[^a-z0-9\u4e00-\u9fff-]/g, '')
    .replace(/-+/g, '-')
    .replace(/^-|-$/g, '')
  if (!raw) return `category-${Date.now().toString(36)}`
  // Prefer ASCII-ish ids; if mostly CJK, prefix
  if (/^[\u4e00-\u9fff-]+$/.test(raw)) {
    return `cat-${raw.slice(0, 24)}`
  }
  return raw.slice(0, 48)
}

/**
 * Atomically add a category: generator + route_config + router.routes + optional capability_slots.
 */
export function applyAddCategory(
  pipeline: AgentPatternPipeline,
  input: AddCategoryInput
): AgentPatternPipeline {
  const label = (input.label || '').trim()
  if (!label) throw new Error('分类名称不能为空')
  const keywords = (input.keywords || []).map((k) => String(k).trim()).filter(Boolean)
  if (!keywords.length) throw new Error('至少填写一个触发关键词')

  const next: AgentPatternPipeline = JSON.parse(JSON.stringify(pipeline))
  const routers = listRouterNodes(next)
  if (!routers.length) throw new Error('请先添加路由节点')

  let router = routers[0]
  if (input.routerNodeId) {
    const found = routers.find((r) => r.id === input.routerNodeId)
    if (!found) throw new Error(`找不到路由节点 ${input.routerNodeId}`)
    router = found
  }

  const nodeId = (input.nodeId || slugifyNodeId(label)).trim()
  if (!nodeId) throw new Error('节点 ID 无效')
  if ((next.nodes || []).some((n) => n.id === nodeId)) {
    throw new Error(`节点 ID 已存在：${nodeId}`)
  }

  ensureNodeConfig(router)
  if (!router.config.custom_config) router.config.custom_config = {}
  const cc = router.config.custom_config
  if (!cc.routes || typeof cc.routes !== 'object') cc.routes = {}
  const routes = cc.routes as Record<string, string>
  for (const kw of keywords) {
    routes[kw] = nodeId
  }

  const routeValue = keywords[0]
  if (input.isDefault) {
    cc.default_route = nodeId
  }

  const generator: NodeWithRoute = {
    id: nodeId,
    type: 'generator',
    kind: 'llm.generate',
    implementation: 'builtin.generator',
    name: label,
    backend: SYSTEM_DEFAULT_BACKEND,
    model: SYSTEM_DEFAULT_MODEL,
    timeout: 120,
    depends_on: [router.id],
    route_config: {
      router_node_id: router.id,
      route_value: routeValue,
      is_default: !!input.isDefault
    },
    config: {
      backend: SYSTEM_DEFAULT_BACKEND,
      model: SYSTEM_DEFAULT_MODEL,
      prompt_template: '{{input}}',
      system_prompt: (input.systemPrompt || '').trim() || `你是「${label}」助手，请准确回答用户问题。`,
      template_vars: {
        backend: 'system.default_backend',
        model: 'system.default_model'
      }
    }
  }

  next.nodes = [...(next.nodes || []), generator]

  const appendSlot = input.appendSlot !== false
  if (appendSlot) {
    if (!next.metadata) next.metadata = {}
    const tags = input.tags?.length ? input.tags : ['default']
    const slot: CapabilitySlot = {
      slot_id: nodeId,
      node_id: nodeId,
      label,
      kind: 'route',
      tags,
      hint: '',
      default_follow_system: true
    }
    const existing = Array.isArray(next.metadata.capability_slots)
      ? [...next.metadata.capability_slots]
      : []
    // If pipeline previously relied on discover / route_model_targets only, seed from resolve then append
    if (!existing.length) {
      const resolved = resolveCapabilitySlots(pipeline)
      for (const s of resolved) {
        if (s.node_id !== nodeId) existing.push(s)
      }
    }
    existing.push(slot)
    next.metadata.capability_slots = existing
  }

  return next
}

function modelIdOf(m: string | { id?: string; name?: string }): string {
  if (typeof m === 'string') return m
  return String(m.id || m.name || '')
}

function scoreModelForTags(backendId: string, modelId: string, tags: string[]): number {
  const text = `${backendId} ${modelId}`.toLowerCase()
  let score = 0
  const tagSet = new Set(tags.map((t) => t.toLowerCase()))

  if (tagSet.has('code')) {
    if (/code|coder|codex|deepseek-coder|starcoder|codestral/.test(text)) score += 50
    if (/deepseek|qwen.*coder|gpt-4o|claude|sonnet/.test(text)) score += 20
  }
  if (tagSet.has('reasoning')) {
    if (/gpt-4o|claude|sonnet|opus|glm-4|qwen-max|o1|o3|gemini-2/.test(text)) score += 45
    if (/flash|mini|lite|haiku|nano/.test(text)) score -= 15
  }
  if (tagSet.has('review')) {
    if (/gpt-4o|claude|sonnet|glm-4|qwen/.test(text)) score += 35
    if (/flash|mini|lite/.test(text)) score += 5
  }
  if (tagSet.has('cheap') || tagSet.has('fast')) {
    if (/flash|mini|haiku|lite|nano|small|turbo/.test(text)) score += 50
    if (/opus|o1|o3|pro/.test(text)) score -= 20
  }
  if (tagSet.has('multilingual') || tagSet.has('explain')) {
    if (/gpt-4o|claude|glm-4|qwen|gemini/.test(text)) score += 40
  }
  if (tagSet.has('default')) {
    score += 1
  }
  return score
}

/**
 * Fill draft rows by tag scoring. Does NOT call API / mutate saved pipeline.
 * Returns new row array; followSystem=false when a candidate is found.
 */
export function recommendCapabilitySlotRows(
  rows: CapabilitySlotRow[],
  backends: BackendLike[],
  opts?: { preferFollowDefaultForDefaultTag?: boolean }
): { rows: CapabilitySlotRow[]; warned?: string } {
  const enabled = (backends || []).filter((b) => b && b.enabled !== false && b.id)
  if (!enabled.length) {
    return { rows: rows.map((r) => ({ ...r })), warned: '没有可用的已启用后端，无法推荐' }
  }

  const candidates: Array<{ backendId: string; modelId: string }> = []
  for (const b of enabled) {
    const models = Array.isArray(b.supported_models) ? b.supported_models : []
    for (const m of models) {
      const mid = modelIdOf(m)
      if (mid) candidates.push({ backendId: String(b.id), modelId: mid })
    }
  }
  if (!candidates.length) {
    return { rows: rows.map((r) => ({ ...r })), warned: '已启用后端没有可用模型列表，无法推荐' }
  }

  const preferDefault = opts?.preferFollowDefaultForDefaultTag !== false
  return {
    rows: rows.map((row) => {
      const tags = row.tags?.length ? row.tags : ['default']
      if (preferDefault && tags.length === 1 && tags[0] === 'default') {
        return { ...row, followSystem: true, backend: '', model: '' }
      }
      let best = candidates[0]
      let bestScore = -Infinity
      for (const c of candidates) {
        const s = scoreModelForTags(c.backendId, c.modelId, tags)
        if (s > bestScore) {
          bestScore = s
          best = c
        }
      }
      return {
        ...row,
        followSystem: false,
        backend: best.backendId,
        model: best.modelId
      }
    })
  }
}

// ── Compatibility aliases (v0.2.2 routeModelAssign) ──────────────────────────

export type RouteModelTarget = CapabilitySlot
export type RouteModelRow = CapabilitySlotRow

export function getRouteModelTargets(pipeline: AgentPatternPipeline | null | undefined): CapabilitySlot[] {
  return resolveCapabilitySlots(pipeline)
}

export function buildRouteModelRows(pipeline: AgentPatternPipeline): CapabilitySlotRow[] {
  return buildCapabilitySlotRows(pipeline)
}

export function applyRouteModelAssignments(
  pipeline: AgentPatternPipeline,
  rows: CapabilitySlotRow[]
): AgentPatternPipeline {
  return applyCapabilitySlotBindings(pipeline, rows)
}

export function canAssignRouteModels(pipeline: AgentPatternPipeline | null | undefined): boolean {
  return canConfigureCapabilitySlots(pipeline)
}

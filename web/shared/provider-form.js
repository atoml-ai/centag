/**
 * Provider Form — Gateway / Minimal 共用的 Provider 添加/编辑流程
 *
 * 策略：
 *   1) 共用：选预设 → 填表 → 校验 → 构建中间态（BackendEntry）
 *   2) 分叉：仅在最终持久化时区分
 *      - Minimal: toBackendEntry() → ConfigBuilder → PUT /api/config/backends → YAML + reload
 *      - Gateway: toApiBackendPayload() → POST/PUT /api/v1/backends → DB (BackendStore)
 *
 * 依赖解析（禁止裸引用 getProviderById / ConfigBuilder，否则 Vite ESM 会 ReferenceError）：
 *   - WebUI：shared-modules.ts 调用 initProviderFormDeps(...)
 *   - WebUI：classic script 下从 globalThis 读取（registry/builder 已挂载）
 */

/** @type {{ getProviderById: null | Function, ConfigBuilder: null | { buildSingleBackend: Function } }} */
var __providerFormDeps = {
  getProviderById: null,
  ConfigBuilder: null,
}

/**
 * 注入依赖（WebUI ESM 必调；classic script 可选）
 * @param {{ getProviderById?: Function, ConfigBuilder?: object }} deps
 */
function initProviderFormDeps(deps) {
  if (!deps) return
  if (typeof deps.getProviderById === 'function') {
    __providerFormDeps.getProviderById = deps.getProviderById
  }
  if (deps.ConfigBuilder) {
    __providerFormDeps.ConfigBuilder = deps.ConfigBuilder
  }
}

function resolveGetProviderById() {
  if (typeof __providerFormDeps.getProviderById === 'function') {
    return __providerFormDeps.getProviderById
  }
  var g = typeof globalThis !== 'undefined' ? globalThis : null
  if (g && typeof g.getProviderById === 'function') {
    return g.getProviderById
  }
  throw new Error(
    'getProviderById is not defined: call initProviderFormDeps({ getProviderById }) ' +
      'or load provider-registry.js before provider-form.js'
  )
}

function resolveConfigBuilder() {
  if (__providerFormDeps.ConfigBuilder && typeof __providerFormDeps.ConfigBuilder.buildSingleBackend === 'function') {
    return __providerFormDeps.ConfigBuilder
  }
  var g = typeof globalThis !== 'undefined' ? globalThis : null
  if (g && g.ConfigBuilder && typeof g.ConfigBuilder.buildSingleBackend === 'function') {
    return g.ConfigBuilder
  }
  throw new Error(
    'ConfigBuilder is not defined: call initProviderFormDeps({ ConfigBuilder }) ' +
      'or load config-builder.js before provider-form.js'
  )
}

/**
 * @typedef {Object} ProviderFormModel
 * @property {string} providerId
 * @property {string} name
 * @property {string} type
 * @property {string} base_url
 * @property {string} api_key
 * @property {string} default_model
 * @property {Array<{name:string,supports_tools?:boolean,supports_images?:boolean,supports_thinking?:boolean,max_context_tokens?:number,actual_model?:string}>} models
 * @property {boolean} isPreset
 * @property {string} [id]              Gateway 编辑时使用
 * @property {boolean} [has_api_key]    Gateway：服务端是否已有 Key
 * @property {string} [description]
 * @property {number} [timeout]
 * @property {number} [max_retries]
 * @property {boolean} [enabled]
 * @property {boolean} [auto_fetch_models]
 * @property {string} [probe_model]
 */

/** @returns {ProviderFormModel} */
function createEmptyProviderForm() {
  return {
    providerId: '',
    name: '',
    type: 'openai',
    base_url: '',
    api_key: '',
    default_model: '',
    models: [],
    isPreset: false,
    id: '',
    has_api_key: false,
    description: '',
    timeout: 120,
    max_retries: 3,
    enabled: true,
    auto_fetch_models: false,
    probe_model: '',
  }
}

/**
 * 应用预设 Provider（两端共用）
 * @param {ProviderFormModel} form
 * @param {string} providerId
 * @returns {boolean}
 */
function applyProviderPreset(form, providerId) {
  var provider = resolveGetProviderById()(providerId)
  if (!provider) return false

  form.providerId = provider.id
  form.name = provider.name
  form.type = provider.type
  form.base_url = provider.base_url
  form.api_key = ''
  form.models = (provider.default_models || []).map(function (m) { return Object.assign({}, m) })
  form.isPreset = true
  form.description = provider.description || ''
  // 默认模型 / 探测模型同源：后端只持久化 probe_model（PreferredDefaultModel）
  // 二者必须一起初始化，避免用户改了 default_model 后仍保存预设首项
  var firstModel = form.models.length > 0 ? form.models[0].name : ''
  form.default_model = firstModel
  form.probe_model = firstModel
  return true
}

/**
 * @param {Array} providers
 * @param {string} query
 * @returns {Array}
 */
function filterProviders(providers, query) {
  const q = (query || '').trim().toLowerCase()
  if (!q) return providers || []
  return (providers || []).filter((p) =>
    (p.name || '').toLowerCase().includes(q) ||
    (p.id || '').toLowerCase().includes(q) ||
    (p.description || '').toLowerCase().includes(q)
  )
}

/**
 * @param {ProviderFormModel} form
 * @param {string} name
 * @returns {boolean}
 */
function addModelToForm(form, name) {
  const n = (name || '').trim()
  if (!n) return false
  if ((form.models || []).some((m) => m.name === n)) return false
  form.models.push({
    name: n,
    supports_tools: false,
    supports_images: false,
    supports_thinking: false,
    max_context_tokens: 4096,
  })
  return true
}

/**
 * @param {ProviderFormModel} form
 * @param {number} index
 */
function removeModelFromForm(form, index) {
  const removed = form.models[index]
  if (!removed) return
  form.models.splice(index, 1)
  if (form.default_model === removed.name) {
    form.default_model = ''
  }
  if (form.probe_model === removed.name) {
    form.probe_model = form.default_model || (form.models[0] && form.models[0].name) || ''
  }
}

/**
 * @param {string} name
 * @returns {string}
 */
function slugifyProviderId(name) {
  const s = (name || '')
    .toLowerCase()
    .trim()
    .replace(/\s+/g, '-')
    .replace(/[^a-z0-9-_]/g, '')
  return s || 'custom'
}

/**
 * 表单校验（两端共用规则；Gateway 编辑时可通过 options 放宽 API Key）
 * @param {ProviderFormModel} form
 * @param {{ isCreate?: boolean, requireApiKey?: boolean }} [options]
 * @returns {{ ok: boolean, errors: string[] }}
 */
function validateProviderForm(form, options) {
  const opts = options || {}
  const errors = []

  if (!(form.name || '').trim()) {
    errors.push('请填写 Provider 名称')
  }
  if (!(form.base_url || '').trim()) {
    errors.push('请填写 Base URL')
  }
  if (!(form.default_model || '').trim()) {
    errors.push('请填写默认模型')
  }

  const requireApiKey = opts.requireApiKey !== false
  const isOllama = form.type === 'ollama'
  if (requireApiKey && !isOllama) {
    const hasInput = !!(form.api_key || '').trim()
    const canKeepExisting = !opts.isCreate && !!form.has_api_key
    if (!hasInput && !canKeepExisting) {
      errors.push('请填写 API Key')
    }
  }

  return { ok: errors.length === 0, errors }
}

/**
 * 中间态：供 Minimal 的 ConfigBuilder / 本地列表使用
 * @param {ProviderFormModel} form
 * @returns {Object} BackendEntry
 */
function toBackendEntry(form) {
  var getById = resolveGetProviderById()
  var preset = form.providerId ? getById(form.providerId) : null
  var id = form.providerId || form.id || slugifyProviderId(form.name)

  return {
    provider: {
      id: id,
      name: form.name,
      type: form.type || 'openai',
      base_url: form.base_url,
      env_key: (preset && preset.env_key) || '',
      icon: (preset && preset.icon) || '⚙️',
      description: form.description || (preset && preset.description) || '',
      default_models: form.models || [],
    },
    apiKey: form.api_key || '',
    defaultModel: form.default_model || '',
    models: (form.models || []).map(function (m) { return Object.assign({}, m) }),
    isPreset: !!form.isPreset,
    isDefault: false,
    settings: {
      timeout: form.timeout > 0 ? form.timeout : 120,
      maxRetries: form.max_retries != null ? form.max_retries : 3,
      weight: 100,
      priority: 10,
    },
  }
}

/**
 * Gateway 持久化载荷：复用 ConfigBuilder 字段语义，再叠加 Gateway 专有字段。
 * 调用方负责 POST/PUT /api/v1/backends（写入 DB）。
 *
 * @param {ProviderFormModel} form
 * @param {{ isCreate?: boolean }} [options]
 * @returns {Object}
 */
function toApiBackendPayload(form, options) {
  var opts = options || {}
  var entry = toBackendEntry(form)
  var Builder = resolveConfigBuilder()
  var payload = Builder.buildSingleBackend(entry)

  // Gateway：尊重表单启用状态（Minimal YAML 里用「有 key 才 enabled」）
  payload.enabled = form.enabled !== false
  payload.auto_fetch_models = !!form.auto_fetch_models
  // 表单「默认模型」是用户可见真源；探测模型留空时回退到它。
  // 注意：不要用「预设首项 probe_model」覆盖用户已选的 default_model。
  payload.probe_model = (form.default_model || form.probe_model || '').trim()
  if (form.description) {
    payload.description = form.description
  }

  if (!opts.isCreate && form.id) {
    payload.id = form.id
  } else if (opts.isCreate) {
    // Gateway 创建：不要用 preset id（如 bigmodel）当后端 id。
    // 种子数据里常已有同名 id；交给服务端 generateBackendID 生成唯一 id。
    // Minimal YAML 路径仍通过 toBackendEntry().provider.id 使用 preset id。
    delete payload.id
  } else if (form.providerId) {
    payload.id = form.providerId
  }

  // 编辑时留空 api_key：后端 Manager.Update 视为「不修改」
  if (!opts.isCreate && !(form.api_key || '').trim()) {
    delete payload.api_key
  }

  return payload
}

/**
 * 从 Gateway API 响应回填表单（编辑）
 * @param {Object} row - BackendConfigResponse
 * @returns {ProviderFormModel}
 */
function fromApiBackend(row) {
  const form = createEmptyProviderForm()
  if (!row) return form

  form.id = row.id || ''
  form.providerId = row.id || ''
  form.name = row.name || ''
  form.type = row.type || 'openai'
  form.base_url = row.base_url || ''
  form.api_key = ''
  form.has_api_key = !!row.has_api_key
  form.description = row.description || ''
  form.timeout = row.timeout > 0 ? row.timeout : 120
  form.max_retries = row.max_retries != null ? row.max_retries : 3
  form.enabled = row.enabled !== false
  form.auto_fetch_models = !!row.auto_fetch_models
  form.probe_model = row.probe_model || ''
  form.isPreset = !!resolveGetProviderById()(row.id)

  const rawModels = Array.isArray(row.supported_models) ? row.supported_models : []
  form.models = rawModels.map((m) => {
    if (typeof m === 'string') {
      return {
        name: m,
        supports_tools: false,
        supports_images: false,
        supports_thinking: false,
        max_context_tokens: 0,
      }
    }
    return {
      name: m.requested_model || m.actual_model || '',
      actual_model: m.actual_model || m.requested_model || '',
      supports_tools: !!m.supports_tools,
      supports_images: !!m.supports_images,
      supports_thinking: !!m.supports_thinking,
      max_context_tokens: m.max_context_tokens || 0,
    }
  }).filter((m) => m.name)

  form.default_model = form.probe_model || (form.models[0] && form.models[0].name) || ''
  return form
}

/**
 * 从 Minimal 本地 BackendEntry 回填表单（编辑）
 * @param {Object} backend - BackendEntry
 * @returns {ProviderFormModel}
 */
function fromBackendEntry(backend) {
  const form = createEmptyProviderForm()
  if (!backend || !backend.provider) return form

  form.providerId = backend.provider.id || ''
  form.name = backend.provider.name || ''
  form.type = backend.provider.type || 'openai'
  form.base_url = backend.provider.base_url || ''
  form.api_key = backend.apiKey || ''
  form.default_model = backend.defaultModel || ''
  form.models = Array.isArray(backend.models) ? backend.models.map((m) => ({ ...m })) : []
  form.isPreset = !!backend.isPreset
  form.description = backend.provider.description || ''
  form.timeout = (backend.settings && backend.settings.timeout) || 120
  form.max_retries = (backend.settings && backend.settings.maxRetries) != null
    ? backend.settings.maxRetries
    : 3
  form.probe_model = form.default_model
  form.has_api_key = !!form.api_key
  return form
}

// UMD: script 标签 (WebUI) + CJS/ESM (webui via Vite)
if (typeof module !== 'undefined' && module.exports) {
  module.exports = {
    initProviderFormDeps,
    createEmptyProviderForm,
    applyProviderPreset,
    filterProviders,
    addModelToForm,
    removeModelFromForm,
    slugifyProviderId,
    validateProviderForm,
    toBackendEntry,
    toApiBackendPayload,
    fromApiBackend,
    fromBackendEntry,
  }
}

if (typeof globalThis !== 'undefined') {
  globalThis.initProviderFormDeps = initProviderFormDeps
  globalThis.createEmptyProviderForm = createEmptyProviderForm
  globalThis.applyProviderPreset = applyProviderPreset
  globalThis.filterProviders = filterProviders
  globalThis.addModelToForm = addModelToForm
  globalThis.removeModelFromForm = removeModelFromForm
  globalThis.validateProviderForm = validateProviderForm
  globalThis.toBackendEntry = toBackendEntry
  globalThis.toApiBackendPayload = toApiBackendPayload
  globalThis.fromApiBackend = fromApiBackend
  globalThis.fromBackendEntry = fromBackendEntry
}

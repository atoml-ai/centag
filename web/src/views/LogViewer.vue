<template>
  <div class="log-viewer">
    <div class="log-header">
      <div class="log-header-main">
        <h2>{{ t('logViewer.title') }}</h2>
        <p class="log-subtitle">
          {{ t('logViewer.subtitle') }}
          <span v-if="isLLMScope" class="scope-hint">{{ t('logViewer.scopeHint') }}</span>
        </p>
      </div>
      <div class="header-actions">
        <button
          type="button"
          class="btn"
          :class="liveTail ? 'btn-live-on' : 'btn-secondary'"
          @click="toggleLiveTail"
        >
          {{ liveTail ? t('logViewer.liveTracking') : t('logViewer.liveTrackingOff') }}
        </button>
        <button type="button" class="btn btn-secondary" @click="jumpToLatest">{{ t('logViewer.jumpToLatest') }}</button>
        <button type="button" class="btn btn-primary" @click="refreshLogs">{{ t('logViewer.refresh') }}</button>
        <button type="button" class="btn btn-success" @click="showExportModal = true">{{ t('logViewer.export') }}</button>
        <button type="button" class="btn btn-danger" :disabled="clearing" @click="confirmClearLogs">
          {{ clearing ? t('logViewer.clearing') : t('logViewer.clearLogs') }}
        </button>
      </div>
    </div>

    <div v-if="fileLogMismatchWarning" class="log-output-warning" role="alert">
      <p>{{ fileLogMismatchWarning }}</p>
    </div>

    <div v-if="logPath" class="log-path-hint">
      {{ t('logViewer.logFile') }} <code>{{ logPath }}</code>
    </div>

    <div class="stats-cards" v-if="stats">
      <div class="stat-card">
        <div class="stat-value">{{ stats.total_logs }}</div>
        <div class="stat-label">{{ statsTimeLabel }} {{ t('logViewer.stats.total') }}</div>
      </div>
      <div class="stat-card error">
        <div class="stat-value">{{ stats.error_count }}</div>
        <div class="stat-label">{{ t('logViewer.stats.error') }}</div>
      </div>
      <div class="stat-card warning">
        <div class="stat-value">{{ stats.warn_count }}</div>
        <div class="stat-label">{{ t('logViewer.stats.warning') }}</div>
      </div>
      <div class="stat-card info">
        <div class="stat-value">{{ matchedTotal }}</div>
        <div class="stat-label">{{ t('logViewer.stats.currentFilter') }}</div>
      </div>
    </div>

    <div class="search-bar">
      <div class="search-field search-field--wide">
        <label>{{ t('logViewer.filters.keyword') }}</label>
        <input
          v-model.trim="filters.q"
          type="search"
          class="filter-text"
          :placeholder="t('logViewer.filters.keywordPlaceholder')"
          @keyup.enter="onFilterChange"
        />
      </div>
      <div class="search-field">
        <label>{{ t('logViewer.filters.requestId') }}</label>
        <input
          v-model.trim="filters.request_id"
          type="search"
          class="filter-text"
          :placeholder="t('logViewer.filters.requestIdPlaceholder')"
          @keyup.enter="onFilterChange"
        />
      </div>
      <button type="button" class="btn btn-primary" @click="onFilterChange">{{ t('logViewer.filters.search') }}</button>
    </div>

    <div class="quick-filters">
      <span class="quick-label">{{ t('logViewer.filters.quick') }}</span>
      <button
        v-for="preset in quickPresets"
        :key="preset.id"
        type="button"
        class="chip"
        :class="{ active: activePreset === preset.id }"
        @click="applyPreset(preset.id)"
      >
        {{ preset.label }}
      </button>
    </div>

    <div class="filters">
      <div class="filter-group filter-scope">
        <label>{{ t('logViewer.filters.logScope') }}</label>
        <select v-model="filters.category" @change="onScopeChange">
          <option value="llm">{{ t('logViewer.filters.llmScope') }}</option>
          <option value="">{{ t('logViewer.filters.allScope') }}</option>
          <option value="system">{{ t('logViewer.filters.systemScope') }}</option>
        </select>
      </div>

      <div class="filter-group">
        <label>{{ t('logViewer.filters.time') }}</label>
        <select v-model="filters.from" @change="onFilterChange">
          <option value="">{{ t('logViewer.filters.timeUnlimited') }}</option>
          <option value="1h">{{ t('logViewer.filters.last1h') }}</option>
          <option value="6h">{{ t('logViewer.filters.last6h') }}</option>
          <option value="24h">{{ t('logViewer.filters.last24h') }}</option>
          <option value="7d">{{ t('logViewer.filters.last7d') }}</option>
        </select>
      </div>

      <div class="filter-group">
        <label>{{ t('logViewer.filters.level') }}</label>
        <select v-model="filters.level" @change="onFilterChange">
          <option value="">{{ t('logViewer.filters.levelAll') }}</option>
          <option value="error">{{ t('logViewer.filters.levelError') }}</option>
          <option value="warn">{{ t('logViewer.filters.levelWarn') }}</option>
          <option value="info">{{ t('logViewer.filters.levelInfo') }}</option>
          <option value="debug">{{ t('logViewer.filters.levelDebug') }}</option>
        </select>
      </div>

      <div class="filter-group">
        <label>{{ t('logViewer.filters.backend') }}</label>
        <select v-model="filters.backend_id" @change="onFilterChange">
          <option value="">{{ t('logViewer.filters.levelAll') }}</option>
          <option v-for="b in backendOptions" :key="b.id" :value="b.id">{{ b.name }}</option>
        </select>
      </div>

      <div class="filter-group filter-model">
        <label>{{ t('logViewer.filters.model') }}</label>
        <input
          v-model.trim="filters.model"
          type="text"
          class="filter-text"
          list="log-model-suggestions"
          :placeholder="t('logViewer.filters.modelPlaceholder')"
          @change="onFilterChange"
          @keyup.enter="onFilterChange"
        />
        <datalist id="log-model-suggestions">
          <option v-for="model in modelOptions" :key="model" :value="model" />
        </datalist>
      </div>

      <div class="filter-group">
        <label>{{ t('logViewer.filters.perPage') }}</label>
        <select v-model.number="filters.limit" @change="onFilterChange">
          <option :value="50">50</option>
          <option :value="100">100</option>
          <option :value="200">200</option>
          <option :value="500">500</option>
        </select>
      </div>
    </div>

    <div class="list-meta" v-if="!loading && logs.length">
      <span>{{ t('logViewer.pagination.page', { page, total: totalPages }) }}</span>
      <span v-if="page === 1" class="newest-badge">{{ t('logViewer.pagination.newest') }}</span>
      <span v-else class="older-hint">{{ t('logViewer.pagination.olderHint') }} <button type="button" class="link-btn" @click="jumpToLatest">{{ t('logViewer.pagination.returnToLatest') }}</button></span>
    </div>

    <div class="log-list">
      <div class="log-table-wrap">
        <table class="log-table">
          <thead>
            <tr>
              <th class="col-expand"></th>
              <th class="col-time">{{ t('logViewer.table.time') }}</th>
              <th class="col-level">{{ t('logViewer.table.level') }}</th>
              <th class="col-message">{{ t('logViewer.table.message') }}</th>
              <th class="col-request">{{ t('logViewer.table.requestId') }}</th>
              <th class="col-status">{{ t('logViewer.table.status') }}</th>
              <th class="col-duration">{{ t('logViewer.table.duration') }}</th>
              <th class="col-backend">{{ t('logViewer.table.backend') }}</th>
              <th class="col-model">{{ t('logViewer.table.model') }}</th>
            </tr>
          </thead>
          <tbody>
            <template v-for="(log, index) in logs" :key="rowKey(log, index)">
              <tr :class="['log-row', 'level-' + log.level, { expanded: expandedRows.has(index) }]">
                <td class="col-expand">
                  <button type="button" class="expand-btn" :aria-expanded="expandedRows.has(index)" @click="toggleExpand(index)">
                    {{ expandedRows.has(index) ? '▼' : '▶' }}
                  </button>
                </td>
                <td class="col-time" :title="formatAbsoluteTime(log.timestamp)">
                  <span class="rel-time">{{ formatRelativeTime(log.timestamp) }}</span>
                  <span class="abs-time">{{ formatAbsoluteTime(log.timestamp) }}</span>
                </td>
                <td class="col-level">
                  <span :class="'badge badge-' + log.level">{{ log.level }}</span>
                </td>
                <td class="col-message">
                  <span class="message-preview">{{ messagePreview(log) }}</span>
                </td>
                <td class="col-request">
                  <button
                    v-if="displayField(log, 'request_id')"
                    type="button"
                    class="request-id-btn"
                    :title="t('logViewer.table.viewTrace', { id: displayField(log, 'request_id') })"
                    @click="onRequestIdClick($event, displayField(log, 'request_id'))"
                  >
                    {{ shortId(displayField(log, 'request_id')) }}
                  </button>
                  <span v-else class="muted">-</span>
                </td>
                <td class="col-status">
                  <span v-if="displayStatus(log)" :class="statusClass(displayStatus(log)!)">{{ displayStatus(log) }}</span>
                  <span v-else class="muted">-</span>
                </td>
                <td class="col-duration">
                  <span v-if="displayDuration(log)">{{ displayDuration(log) }}ms</span>
                  <span v-else class="muted">-</span>
                </td>
                <td class="col-backend">{{ displayField(log, 'backend_id') || '-' }}</td>
                <td class="col-model">{{ displayField(log, 'model') || '-' }}</td>
              </tr>
              <tr v-if="expandedRows.has(index)" class="detail-row">
                <td colspan="9">
                  <div class="detail-panel">
                    <div class="detail-grid">
                      <div><strong>{{ t('logViewer.detail.time') }}</strong> {{ formatAbsoluteTime(log.timestamp) }}</div>
                      <div v-if="log.request_id"><strong>{{ t('logViewer.detail.requestId') }}</strong> <code>{{ log.request_id }}</code></div>
                      <div v-if="log.client_ip"><strong>{{ t('logViewer.detail.clientIp') }}</strong> {{ log.client_ip }}</div>
                      <div v-if="log.caller"><strong>{{ t('logViewer.detail.caller') }}</strong> {{ log.caller }}</div>
                    </div>
                    <div v-if="bodyPreview(log, 'request')" class="detail-body-block">
                      <strong>{{ t('logViewer.detail.requestContent') }}</strong>
                      <pre class="detail-body">{{ bodyPreview(log, 'request') }}</pre>
                    </div>
                    <div v-if="bodyPreview(log, 'response')" class="detail-body-block">
                      <strong>{{ t('logViewer.detail.responseContent') }}</strong>
                      <pre class="detail-body">{{ bodyPreview(log, 'response') }}</pre>
                    </div>
                    <div v-if="detailExtraEntries(log).length" class="detail-extra-grid">
                      <div v-for="item in detailExtraEntries(log)" :key="item.key" class="detail-extra-item">
                        <strong>{{ item.label }}</strong>
                        <span>{{ item.value }}</span>
                      </div>
                    </div>
                    <pre class="detail-message">{{ log.message }}</pre>
                  </div>
                </td>
              </tr>
            </template>
          </tbody>
        </table>
      </div>

      <div v-if="logs.length === 0 && !loading" class="empty-state">
        <p>{{ t('logViewer.empty') }}</p>
        <p v-if="hasActiveFilters" class="empty-hint">{{ t('logViewer.emptyHint') }}</p>
      </div>

      <div v-if="loading" class="loading-state">
        <p>{{ t('logViewer.loading') }}</p>
      </div>
    </div>

    <div class="pagination" v-if="totalPages > 1">
      <button type="button" @click="jumpToLatest" :disabled="page === 1">{{ t('logViewer.pagination.latest') }}</button>
      <button type="button" @click="prevPage" :disabled="page === 1">{{ t('logViewer.pagination.prevPage') }}</button>
      <span>{{ page }} / {{ totalPages }}</span>
      <button type="button" @click="nextPage" :disabled="page >= totalPages">{{ t('logViewer.pagination.nextPage') }}</button>
    </div>

    <div v-if="showExportModal" class="modal-overlay" @click="showExportModal = false">
      <div class="modal" @click.stop>
        <h3>{{ t('logViewer.exportModal.title') }}</h3>
        <div class="modal-content">
          <div class="form-group">
            <label>{{ t('logViewer.exportModal.format') }}</label>
            <select v-model="exportFormat">
              <option value="json">JSON</option>
              <option value="csv">CSV</option>
              <option value="txt">TXT</option>
            </select>
          </div>
          <p class="note">{{ t('logViewer.exportModal.note') }}</p>
        </div>
        <div class="modal-actions">
          <button type="button" class="btn btn-primary" :disabled="exporting" @click="exportLogs">
            {{ exporting ? t('logViewer.exportModal.exporting') : t('logViewer.exportModal.exportBtn') }}
          </button>
          <button type="button" class="btn btn-secondary" :disabled="exporting" @click="showExportModal = false">{{ t('logViewer.exportModal.cancel') }}</button>
        </div>
        <p v-if="exportFeedback" class="export-feedback" :class="{ error: exportFeedbackError }">{{ exportFeedback }}</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { logsApi, getBackends, type LogEntry } from '../api'
import { isPersonalEdition } from '@/utils/edition'
import { saveBlobAsFile } from '@/utils/downloadFile'

const { t } = useI18n()
const router = useRouter()
const LOG_SCOPE_KEY = 'centag.logViewer.category'

const logs = ref<LogEntry[]>([])
const stats = ref<any>(null)
const fileLogMismatchWarning = ref('')
const logPath = ref('')
const matchedTotal = ref(0)
const loading = ref(false)
const page = ref(1)
const totalPages = ref(1)
const showExportModal = ref(false)
const exportFormat = ref('json')
const exporting = ref(false)
const clearing = ref(false)
const exportFeedback = ref('')
const exportFeedbackError = ref(false)
const liveTail = ref(false)
const expandedRows = ref(new Set<number>())
let liveTimer: ReturnType<typeof setInterval> | null = null

const backendOptions = ref<{ id: string; name: string }[]>([])

function initialLogCategory(): string {
  try {
    const saved = localStorage.getItem(LOG_SCOPE_KEY)
    if (saved === 'llm' || saved === 'api' || saved === 'system' || saved === '') {
      return saved === 'api' ? 'llm' : saved
    }
  } catch {
    /* ignore */
  }
  return isPersonalEdition() ? 'llm' : ''
}

const filters = ref({
  from: '1h',
  level: '',
  backend_id: '',
  model: '',
  q: '',
  request_id: '',
  category: initialLogCategory(),
  limit: 100
})

const quickPresets = computed(() => [
  { id: 'all', label: t('logViewer.filters.allLogs') },
  { id: 'llm', label: t('logViewer.filters.llmOnly') },
  { id: 'error', label: t('logViewer.filters.errorOnly') }
] as const)

type PresetId = 'all' | 'llm' | 'error'

const isLLMScope = computed(() => {
  const c = filters.value.category
  return c === 'llm' || c === 'api'
})

const activePreset = computed<PresetId>(() => {
  if (filters.value.level === 'error') return 'error'
  if (isLLMScope.value) return 'llm'
  return 'all'
})

const hasActiveFilters = computed(() => {
  const f = filters.value
  return !!(f.q || f.request_id || f.level || f.backend_id || f.model || f.category)
})

const statsTimeLabel = computed(() => {
  const labels: Record<string, string> = {
    '1h': t('logViewer.statsTimeLabel.1h'),
    '6h': t('logViewer.statsTimeLabel.6h'),
    '24h': t('logViewer.statsTimeLabel.24h'),
    '7d': t('logViewer.statsTimeLabel.7d'),
    '': t('logViewer.statsTimeLabel.')
  }
  return labels[filters.value.from] ?? t('logViewer.statsTimeLabel.24h')
})

function emptyStats() {
  return {
    total_logs: 0,
    error_count: 0,
    warn_count: 0,
    info_count: 0,
    debug_count: 0,
    backend_stats: {},
    model_stats: {},
    hourly_stats: {}
  }
}

const modelOptions = computed(() => {
  const m = stats.value?.model_stats
  if (m && typeof m === 'object') {
    return Object.keys(m).filter(Boolean).sort((a, b) => a.localeCompare(b))
  }
  return []
})

function rowKey(log: LogEntry, index: number) {
  return `${log.timestamp}-${log.request_id || ''}-${index}`
}

function parseFieldsFromMessage(message: string): Partial<LogEntry> {
  const out: Partial<LogEntry> = {}
  const id = message.match(/(?:\[request\]\s*)?id:\s*(\S+)/i)
  if (id) out.request_id = id[1].replace(/\|$/, '')
  const model = message.match(/(?:\[request details\]\s*)?model:\s*(\S+)/i) ||
    message.match(/requested_model[=:]\s*"?([^",\s|]+)"?/i)
  if (model) out.model = model[1].replace(/\|$/, '').replace(/"/g, '')
  const backend = message.match(/(?:backend_id|selected_backend|backend)[=:]\s*"?([^",\s|]+)"?/i)
  if (backend) out.backend_id = backend[1].replace(/\|$/, '').replace(/"/g, '')
  const status = message.match(/status_code[=:]\s*(\d{3})/i)
  if (status) out.status_code = Number(status[1])
  const duration = message.match(/(?:duration_ms|latency_ms|total_latency_ms)[=:]\s*(\d+)/i)
  if (duration) out.duration_ms = Number(duration[1])
  return out
}

function enrichLogsForDisplay(entries: LogEntry[]): LogEntry[] {
  const rows = entries.map((e) => ({
    ...e,
    ...parseFieldsFromMessage(e.message || ''),
    extra: { ...(e.extra ?? {}) }
  }))
  const order = rows
    .map((row, index) => ({ row, index, ts: Date.parse(row.timestamp || '') || 0 }))
    .sort((a, b) => a.ts - b.ts || a.index - b.index)

  let ctx = { request_id: '', model: '', backend_id: '' }
  for (const { row } of order) {
    const msgTrim = (row.message || '').trim()
    if (/^request$/i.test(msgTrim) || /^request\s+\{/i.test(msgTrim)) {
      ctx = { request_id: '', model: '', backend_id: '' }
      continue
    }
    if (/request started/i.test(row.message || '')) {
      if (row.request_id && row.request_id !== ctx.request_id) {
        ctx = { request_id: row.request_id, model: '', backend_id: '' }
      }
    }
    if (row.request_id) ctx.request_id = row.request_id
    if (row.model) ctx.model = row.model
    if (row.backend_id) ctx.backend_id = row.backend_id
    if (!row.request_id && ctx.request_id) row.request_id = ctx.request_id
    if (!row.model && ctx.model) row.model = ctx.model
    if (!row.backend_id && ctx.backend_id) row.backend_id = ctx.backend_id
    if (/\[request\] completed/i.test(row.message || '')) {
      ctx = { request_id: '', model: '', backend_id: '' }
    }
  }
  return rows
}

const extraFieldLabels: Record<string, string> = computed(() => ({
  node_id: t('logViewer.extraLabels.node_id'),
  backend_id: t('logViewer.extraLabels.backend_id'),
  message_count: t('logViewer.extraLabels.message_count'),
  tokens: t('logViewer.extraLabels.tokens'),
  proxy_mode: t('logViewer.extraLabels.proxy_mode'),
  source: t('logViewer.extraLabels.source'),
  stream: t('logViewer.extraLabels.stream'),
  temperature: t('logViewer.extraLabels.temperature'),
  max_tokens: t('logViewer.extraLabels.max_tokens'),
  system_prompt_set: t('logViewer.extraLabels.system_prompt_set'),
  method: t('logViewer.extraLabels.method'),
  user_agent: t('logViewer.extraLabels.user_agent'),
  has_system_message: t('logViewer.extraLabels.has_system_message'),
  cache_read: t('logViewer.extraLabels.cache_read'),
  cache_write: t('logViewer.extraLabels.cache_write'),
  kind: t('logViewer.extraLabels.kind'),
  duration_ms: t('logViewer.extraLabels.duration_ms'),
  response_model: t('logViewer.extraLabels.response_model')
}))

const bodyFieldKeys = {
  request: [
    'messages_preview',
    'user_input_preview',
    'request_body_preview',
    'request_body',
    'question_preview',
    'prompt_preview'
  ],
  response: ['response_preview', 'response_body', 'answer_preview']
}

function logExtra(log: LogEntry): Record<string, string> {
  return log.extra ?? {}
}

function bodyPreview(log: LogEntry, kind: 'request' | 'response'): string {
  const extra = logExtra(log)
  for (const key of bodyFieldKeys[kind]) {
    const v = extra[key]
    if (v && v.trim()) return v.trim()
  }
  return ''
}

function messagePreview(log: LogEntry): string {
  const req = bodyPreview(log, 'request')
  const resp = bodyPreview(log, 'response')
  if (req) return `${log.message} · ${truncatePreview(req, 80)}`
  if (resp) return `${log.message} · ${truncatePreview(resp, 80)}`
  return log.message
}

function truncatePreview(text: string, max: number): string {
  const oneLine = text.replace(/\s+/g, ' ').trim()
  if (oneLine.length <= max) return oneLine
  return oneLine.slice(0, max) + '…'
}

function detailExtraEntries(log: LogEntry): { key: string; label: string; value: string }[] {
  const skip = new Set<string>([...bodyFieldKeys.request, ...bodyFieldKeys.response])
  const out: { key: string; label: string; value: string }[] = []
  for (const [key, value] of Object.entries(logExtra(log))) {
    if (!value?.trim() || skip.has(key)) continue
    out.push({
      key,
      label: extraFieldLabels.value[key] ?? key,
      value: value.trim()
    })
  }
  return out.sort((a, b) => a.label.localeCompare(b.label))
}

function displayField(log: LogEntry, key: 'request_id' | 'backend_id' | 'model'): string {
  const v = log[key]
  return v ? String(v) : ''
}

function displayStatus(log: LogEntry): number | undefined {
  return log.status_code || undefined
}

function displayDuration(log: LogEntry): number | undefined {
  return log.duration_ms || undefined
}

function appendTimeRangeToParams(params: Record<string, unknown>) {
  if (!filters.value.from) return
  const now = new Date()
  const offsets: Record<string, number> = {
    '1h': 60 * 60 * 1000,
    '6h': 6 * 60 * 60 * 1000,
    '24h': 24 * 60 * 60 * 1000,
    '7d': 7 * 24 * 60 * 60 * 1000
  }
  const ms = offsets[filters.value.from] ?? offsets['1h']
  params.from = new Date(now.getTime() - ms).toISOString()
}

function buildListFilterParams(): Record<string, unknown> {
  const params: Record<string, unknown> = {
    page: page.value,
    limit: filters.value.limit
  }
  appendTimeRangeToParams(params)
  if (filters.value.level) params.level = filters.value.level
  if (filters.value.backend_id) params.backend_id = filters.value.backend_id
  if (filters.value.model) params.model = filters.value.model
  if (filters.value.q) params.q = filters.value.q
  if (filters.value.request_id) params.request_id = filters.value.request_id
  if (filters.value.category) params.category = filters.value.category
  return params
}

async function loadBackendOptions() {
  try {
    const list = await getBackends()
    const arr = Array.isArray(list) ? list : []
    backendOptions.value = arr
      .filter((b: any) => b.enabled !== false)
      .map((b: any) => ({ id: b.id, name: b.name || b.id }))
  } catch (e) {
    console.error('Failed to load backends for log filters:', e)
    backendOptions.value = []
  }
}

const loadLogs = async (opts?: { silent?: boolean }) => {
  if (!opts?.silent) loading.value = true
  try {
    const data = await logsApi.getLogs(buildListFilterParams())
    logs.value = enrichLogsForDisplay(data.logs ?? [])
    totalPages.value = data.total_pages ?? 1
    matchedTotal.value = data.total ?? logs.value.length
    logPath.value = data.log_path ?? ''
    fileLogMismatchWarning.value = data.warning ?? ''
    if (page.value === 1) {
      expandedRows.value = new Set()
    }
  } catch (error) {
    console.error('Failed to load logs:', error)
  } finally {
    if (!opts?.silent) loading.value = false
  }
}

function buildStatsFilterParams(): Record<string, unknown> {
  const params: Record<string, unknown> = {}
  appendTimeRangeToParams(params)
  if (filters.value.category) params.category = filters.value.category
  if (filters.value.level) params.level = filters.value.level
  if (filters.value.backend_id) params.backend_id = filters.value.backend_id
  if (filters.value.model) params.model = filters.value.model
  if (filters.value.q) params.q = filters.value.q
  if (filters.value.request_id) params.request_id = filters.value.request_id
  return params
}

const loadStats = async () => {
  try {
    const data = await logsApi.getStats(buildStatsFilterParams())
    stats.value = data
    const w = typeof (data as any)?.warning === 'string' ? (data as any).warning : ''
    if (w) fileLogMismatchWarning.value = w
  } catch (error) {
    console.error('Failed to load stats:', error)
    throw error
  }
}

function persistLogCategory() {
  try {
    localStorage.setItem(LOG_SCOPE_KEY, filters.value.category)
  } catch {
    /* ignore */
  }
}

function onFilterChange() {
  page.value = 1
  loadLogs()
  loadStats()
}

function onScopeChange() {
  persistLogCategory()
  onFilterChange()
}

function applyPreset(id: PresetId) {
  page.value = 1
  switch (id) {
    case 'llm':
      filters.value.category = 'llm'
      filters.value.level = ''
      break
    case 'error':
      filters.value.category = ''
      filters.value.level = 'error'
      break
    default:
      filters.value.category = ''
      filters.value.level = ''
      break
  }
  persistLogCategory()
  loadLogs()
  loadStats()
}

function filterByRequestId(id: string) {
  filters.value.request_id = id
  filters.value.category = 'llm'
  persistLogCategory()
  page.value = 1
  loadLogs()
}

function onRequestIdClick(event: MouseEvent, id: string) {
  if (event.shiftKey) {
    filterByRequestId(id)
    return
  }
  router.push({ name: 'RequestTrace', params: { requestId: id } })
}

function jumpToLatest() {
  page.value = 1
  loadLogs()
}

const refreshLogs = () => {
  page.value = 1
  loadLogs()
  loadStats()
}

const prevPage = () => {
  if (page.value > 1) {
    page.value--
    loadLogs()
  }
}

const nextPage = () => {
  if (page.value < totalPages.value) {
    page.value++
    loadLogs()
  }
}

function toggleExpand(index: number) {
  const next = new Set(expandedRows.value)
  if (next.has(index)) next.delete(index)
  else next.add(index)
  expandedRows.value = next
}

function toggleLiveTail() {
  liveTail.value = !liveTail.value
  if (liveTail.value) {
    page.value = 1
    loadLogs()
    liveTimer = setInterval(() => {
      if (page.value === 1) {
        loadLogs({ silent: true })
      }
    }, 3000)
  } else if (liveTimer) {
    clearInterval(liveTimer)
    liveTimer = null
  }
}

function shortId(id: string) {
  if (id.length <= 14) return id
  return id.slice(0, 8) + '…' + id.slice(-4)
}

function statusClass(code: number) {
  if (code >= 500) return 'status-error'
  if (code >= 400) return 'status-warn'
  if (code >= 200 && code < 300) return 'status-ok'
  return ''
}

function parseTimestamp(ts: string): Date | null {
  if (!ts) return null
  const d = new Date(ts)
  return Number.isNaN(d.getTime()) ? null : d
}

function formatAbsoluteTime(timestamp: string) {
  const d = parseTimestamp(timestamp)
  if (!d) return '-'
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

function formatRelativeTime(timestamp: string) {
  const d = parseTimestamp(timestamp)
  if (!d) return '-'
  const diff = Date.now() - d.getTime()
  if (diff < 0) return t('logViewer.timeAgo.justNow')
  const sec = Math.floor(diff / 1000)
  if (sec < 60) return t('logViewer.timeAgo.secondsAgo', { sec })
  const min = Math.floor(sec / 60)
  if (min < 60) return t('logViewer.timeAgo.minutesAgo', { min })
  const hr = Math.floor(min / 60)
  if (hr < 24) return t('logViewer.timeAgo.hoursAgo', { hr })
  const day = Math.floor(hr / 24)
  return t('logViewer.timeAgo.daysAgo', { day })
}

const confirmClearLogs = async () => {
  try {
    await ElMessageBox.confirm(
      t('logViewer.message.clearConfirm'),
      t('logViewer.message.clearTitle'),
      { type: 'warning', confirmButtonText: t('logViewer.message.clearConfirmBtn'), cancelButtonText: t('logViewer.message.clearCancelBtn') }
    )
  } catch {
    return
  }

  clearing.value = true
  if (liveTail.value) {
    toggleLiveTail()
  }
  try {
    const data = await logsApi.clearLogs()
    const cleared = data?.cleared_files ?? 0
    ElMessage.success(data?.message || t('logViewer.message.clearedSuccess', { count: cleared }))
    if (data?.warning) {
      ElMessage.warning(data.warning)
    }
    page.value = 1
    expandedRows.value = new Set()
    logs.value = []
    matchedTotal.value = 0
    stats.value = emptyStats()
    await Promise.all([loadStats(), loadLogs()])
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : t('logViewer.message.clearFailed'))
    console.error('Clear logs failed:', error)
  } finally {
    clearing.value = false
  }
}

const exportLogs = async () => {
  exporting.value = true
  exportFeedback.value = ''
  exportFeedbackError.value = false
  try {
    const params: Record<string, unknown> = { format: exportFormat.value }
    appendTimeRangeToParams(params)
    if (filters.value.level) params.level = filters.value.level
    if (filters.value.backend_id) params.backend_id = filters.value.backend_id
    if (filters.value.model) params.model = filters.value.model
    if (filters.value.q) params.q = filters.value.q
    if (filters.value.request_id) params.request_id = filters.value.request_id
    if (filters.value.category) params.category = filters.value.category

    const blob = await logsApi.exportLogs(params)
    const filename = `logs_${new Date().toISOString().slice(0, 10)}.${exportFormat.value}`
    const result = await saveBlobAsFile(blob, filename)

    if (result.mode === 'cancelled') {
      exportFeedback.value = t('logViewer.message.exportCancelled')
      return
    }
    if (result.mode === 'desktop') {
      exportFeedback.value = t('logViewer.message.exportSavedDesktop', { path: result.path })
      showExportModal.value = false
      return
    }
    exportFeedback.value = t('logViewer.message.exportSavedBrowser', { filename: result.filename })
    showExportModal.value = false
  } catch (error) {
    exportFeedbackError.value = true
    exportFeedback.value = error instanceof Error ? error.message : t('logViewer.message.exportFailed')
    console.error('Export failed:', error)
  } finally {
    exporting.value = false
  }
}

onMounted(async () => {
  await loadBackendOptions()
  await loadStats()
  await loadLogs()
})

onUnmounted(() => {
  if (liveTimer) clearInterval(liveTimer)
})
</script>

<style scoped>
.log-viewer {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  padding: 0;
  max-width: 100%;
  box-sizing: border-box;
}

.log-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}

.log-header h2 {
  margin: 0 0 4px;
  font-size: 1.35rem;
}

.log-subtitle {
  margin: 0;
  font-size: 13px;
  color: #64748b;
}

.scope-hint {
  color: #1d4ed8;
  font-weight: 600;
}

.filter-scope select {
  min-width: 180px;
  font-weight: 500;
}

.header-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.log-output-warning {
  margin-bottom: 12px;
  padding: 12px 14px;
  border-radius: 8px;
  border: 1px solid #d4a574;
  background: #fff8ed;
  color: #5c3d1e;
  font-size: 14px;
  line-height: 1.5;
}

.log-output-warning p {
  margin: 0;
}

.log-path-hint {
  margin-bottom: 12px;
  font-size: 12px;
  color: #64748b;
}

.log-path-hint code {
  font-size: 12px;
  background: #f1f5f9;
  padding: 2px 6px;
  border-radius: 4px;
}

.stats-cards {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
  margin-bottom: 16px;
}

.stat-card {
  background: #f8fafc;
  padding: 12px;
  border-radius: 8px;
  text-align: center;
  border: 1px solid #e2e8f0;
}

.stat-card.error { background: #fef2f2; border-color: #fecaca; color: #b91c1c; }
.stat-card.warning { background: #fffbeb; border-color: #fde68a; color: #b45309; }
.stat-card.info { background: #eff6ff; border-color: #bfdbfe; color: #1d4ed8; }

.stat-value {
  font-size: 22px;
  font-weight: 700;
}

.stat-label {
  font-size: 12px;
  opacity: 0.8;
}

.search-bar {
  display: flex;
  gap: 12px;
  align-items: flex-end;
  margin-bottom: 12px;
  flex-wrap: wrap;
}

.search-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.search-field label {
  font-size: 12px;
  color: #64748b;
}

.search-field--wide {
  flex: 1;
  min-width: 220px;
}

.quick-filters {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}

.quick-label {
  font-size: 13px;
  color: #64748b;
}

.chip {
  padding: 4px 12px;
  border-radius: 999px;
  border: 1px solid #cbd5e1;
  background: #fff;
  font-size: 13px;
  cursor: pointer;
}

.chip.active {
  background: #3b82f6;
  border-color: #3b82f6;
  color: #fff;
}

.filters {
  display: flex;
  gap: 12px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}

.filter-group {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.filter-group label {
  font-size: 12px;
  color: #64748b;
}

.filter-group select,
.filter-text {
  padding: 6px 10px;
  border: 1px solid #cbd5e1;
  border-radius: 6px;
  min-width: 120px;
  font-size: 13px;
}

.filter-model .filter-text {
  min-width: 160px;
}

.list-meta {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 8px;
  font-size: 13px;
  color: #64748b;
}

.newest-badge {
  color: #15803d;
  font-weight: 600;
}

.older-hint {
  color: #b45309;
}

.link-btn {
  border: none;
  background: none;
  color: #3b82f6;
  cursor: pointer;
  padding: 0;
  text-decoration: underline;
  font-size: inherit;
}

.log-list {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.log-table-wrap {
  flex: 1;
  min-height: 280px;
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  overflow: auto;
}

.log-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
  table-layout: fixed;
}

.log-table th,
.log-table td {
  padding: 8px 10px;
  text-align: left;
  border-bottom: 1px solid #f1f5f9;
  vertical-align: top;
}

.log-table th {
  background: #f8fafc;
  font-weight: 600;
  position: sticky;
  top: 0;
  z-index: 1;
}

.col-expand { width: 3%; }
.col-time { width: 10%; }
.col-level { width: 5%; }
.col-message { width: 36%; }
.col-request { width: 10%; }
.col-status { width: 5%; }
.col-duration { width: 6%; }
.col-backend { width: 8%; }
.col-model { width: 9%; }

.rel-time {
  display: block;
  font-weight: 600;
  color: #0f172a;
}

.abs-time {
  display: block;
  font-size: 11px;
  color: #94a3b8;
}

.log-table td.col-backend,
.log-table td.col-model,
.log-table td.col-request {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.message-preview {
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 4;
  overflow: hidden;
  white-space: normal;
  word-break: break-word;
  line-height: 1.45;
}

.request-id-btn {
  border: none;
  background: #eff6ff;
  color: #1d4ed8;
  padding: 2px 6px;
  border-radius: 4px;
  font-family: ui-monospace, monospace;
  font-size: 11px;
  cursor: pointer;
}

.request-id-btn:hover {
  background: #dbeafe;
}

.muted {
  color: #94a3b8;
}

.badge {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 600;
  text-transform: lowercase;
}

.badge-error { background: #fee2e2; color: #b91c1c; }
.badge-warn { background: #fef3c7; color: #b45309; }
.badge-info { background: #dbeafe; color: #1d4ed8; }
.badge-debug { background: #f1f5f9; color: #64748b; }

.level-error { background: #fff5f5; }
.level-warn { background: #fffbeb; }

.status-ok { color: #15803d; font-weight: 600; }
.status-warn { color: #b45309; font-weight: 600; }
.status-error { color: #b91c1c; font-weight: 600; }

.expand-btn {
  border: none;
  background: none;
  cursor: pointer;
  color: #64748b;
  font-size: 10px;
  padding: 2px 4px;
}

.detail-row td {
  background: #f8fafc;
  padding: 0;
}

.detail-panel {
  padding: 12px 16px 16px 42px;
}

.detail-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  margin-bottom: 8px;
  font-size: 12px;
  color: #475569;
}

.detail-body-block {
  margin-bottom: 10px;
}

.detail-body-block strong {
  display: block;
  font-size: 12px;
  color: #334155;
  margin-bottom: 4px;
}

.detail-body {
  margin: 0;
  padding: 10px 12px;
  background: #f0fdf4;
  border: 1px solid #bbf7d0;
  border-radius: 6px;
  white-space: pre-wrap;
  word-break: break-word;
  font-size: 12px;
  line-height: 1.5;
  max-height: 320px;
  overflow: auto;
}

.detail-body-block + .detail-body-block .detail-body {
  background: #eff6ff;
  border-color: #bfdbfe;
}

.detail-extra-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 8px 16px;
  margin-bottom: 10px;
  font-size: 12px;
}

.detail-extra-item strong {
  display: block;
  color: #64748b;
  font-size: 11px;
  margin-bottom: 2px;
}

.detail-extra-item span {
  color: #1e293b;
  word-break: break-word;
}

.detail-message {
  margin: 0;
  padding: 10px 12px;
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 6px;
  white-space: pre-wrap;
  word-break: break-word;
  font-size: 12px;
  line-height: 1.5;
  max-height: 240px;
  overflow: auto;
}

.empty-state,
.loading-state {
  padding: 40px;
  text-align: center;
  color: #94a3b8;
}

.empty-hint {
  font-size: 13px;
  margin-top: 8px;
}

.pagination {
  display: flex;
  justify-content: center;
  align-items: center;
  gap: 12px;
  margin-top: 16px;
}

.pagination button {
  padding: 8px 14px;
  border: 1px solid #cbd5e1;
  border-radius: 6px;
  background: #fff;
  cursor: pointer;
  font-size: 13px;
}

.pagination button:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(15, 23, 42, 0.45);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.modal {
  background: white;
  border-radius: 10px;
  padding: 20px;
  min-width: 400px;
}

.modal h3 {
  margin-top: 0;
}

.modal-content {
  padding: 16px 0;
}

.form-group {
  margin-bottom: 12px;
}

.form-group label {
  display: block;
  margin-bottom: 4px;
  font-size: 13px;
}

.form-group select {
  width: 100%;
  padding: 8px;
  border: 1px solid #cbd5e1;
  border-radius: 6px;
}

.note {
  color: #64748b;
  font-size: 13px;
}

.export-feedback {
  margin: 12px 0 0;
  padding: 10px 12px;
  border-radius: 6px;
  background: #ecfdf5;
  color: #047857;
  font-size: 13px;
  word-break: break-all;
}

.export-feedback.error {
  background: #fef2f2;
  color: #b91c1c;
}

.modal-actions {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
}

.btn {
  padding: 8px 14px;
  border: none;
  border-radius: 6px;
  cursor: pointer;
  font-size: 13px;
}

.btn-primary { background: #3b82f6; color: white; }
.btn-secondary { background: #e2e8f0; color: #334155; }
.btn-success { background: #10b981; color: white; }
.btn-danger { background: #ef4444; color: white; }
.btn-danger:hover:not(:disabled) { background: #dc2626; }
.btn-danger:disabled { opacity: 0.6; cursor: not-allowed; }
.btn-live-on { background: #dc2626; color: white; }

@media (max-width: 900px) {
  .stats-cards {
    grid-template-columns: repeat(2, 1fr);
  }
}
</style>

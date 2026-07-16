<template>
  <div class="backends-page">
    <div class="hermes-header">
      <div class="hermes-header-left">
        <h1 class="hermes-title">后端网关配置</h1>
        <p class="hermes-subtitle">管理 LLM 后端服务，配置路由与健康检查</p>
      </div>
      <div class="hermes-header-right">
        <button class="btn btn-outline" @click="handleImport" :disabled="loading">
          导入
        </button>
        <el-dropdown trigger="click" :disabled="loading" @command="handleExportCommand">
          <button class="btn btn-outline" :disabled="loading">
            导出 ▾
          </button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="json">JSON 备份</el-dropdown-item>
              <el-dropdown-item command="yaml">backends YAML</el-dropdown-item>
              <el-dropdown-item command="zip">initdata 打包 (ZIP)</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
        <button class="btn btn-primary" @click="editorRef?.openCreate()">
          + 添加 Provider
        </button>
      </div>
    </div>

    <div class="toolbar-row" v-if="selectedProviders.length > 0">
      <span class="toolbar-count">已选 {{ selectedProviders.length }} 项</span>
      <button class="btn btn-sm btn-danger" @click="batchDelete" :disabled="loading">批量删除</button>
      <button class="btn btn-sm btn-outline" @click="batchProbe" :disabled="loading">批量检测</button>
      <button class="btn btn-sm btn-text" @click="clearSelection">取消选择</button>
    </div>

    <div class="card-grid" v-if="providers.length > 0">
      <div
        v-for="p in providers"
        :key="p.id"
        class="provider-card"
        :class="{ selected: selectedProviders.includes(p.id), 'is-loading': healthProbing[p.id] }"
      >
        <div class="card-select-indicator" @click="toggleSelect(p.id)">
          <span class="check-box" :class="{ checked: selectedProviders.includes(p.id) }"></span>
        </div>
        <div class="card-main">
          <div class="card-body">
            <div class="card-header">
              <div class="card-title-row">
                <span
                  class="health-dot"
                  :class="healthStatusClass(p.id)"
                  :title="healthStatusText(p.id)"
                ></span>
                <h4 class="card-title">{{ p.name }}</h4>
                <span class="provider-type-badge">{{ getTypeDisplayName(p.type) }}</span>
              </div>
              <div class="card-meta">
                <template v-if="p.base_url">Base URL: <code>{{ p.base_url }}</code></template>
                <template v-else>通过 {{ p.type }} 默认端点</template>
              </div>
            </div>
            <div class="model-tags" v-if="modelNames(p).length > 0">
              <span
                v-for="m in modelNames(p).slice(0, 8)"
                :key="m"
                class="model-tag"
                :class="{ highlight: m === p.probe_model }"
              >
                {{ m }}
                <template v-if="m === p.probe_model"> (探测)</template>
              </span>
              <span v-if="modelNames(p).length > 8" class="model-tag more">+{{ modelNames(p).length - 8 }}</span>
            </div>
          </div>
          <div class="card-footer">
            <button class="btn btn-sm btn-outline" @click="probeOne(p.id)" :disabled="healthProbing[p.id]">
              {{ healthProbing[p.id] ? '检测中...' : '检测' }}
            </button>
            <button class="btn btn-sm btn-outline" @click="editorRef?.openEdit(p)">编辑</button>
            <button class="btn btn-sm btn-danger-text" @click="deleteProvider(p)" :disabled="loading">删除</button>
          </div>
        </div>
      </div>
    </div>

    <div class="empty-state" v-else-if="!loading">
      <div class="empty-icon">&#9881;</div>
      <h3>还没有配置任何后端</h3>
      <p>点击"添加 Provider"开始配置第一个 LLM 后端服务</p>
      <button class="btn btn-primary" @click="editorRef?.openCreate()">
        + 添加 Provider
      </button>
    </div>

    <template v-if="loading && providers.length === 0">
      <div class="loading-state">加载中...</div>
    </template>

    <BackendEditorDialog
      ref="editorRef"
      v-model="editorVisible"
      @saved="fetchProviders"
    />

    <input
      ref="importInput"
      type="file"
      accept=".json,.yaml,.yml"
      style="display: none"
      @change="handleImportFile"
    />
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import * as yaml from 'js-yaml'
import {
  getProviderById,
  ConfigBuilder,
  fromApiBackend,
  toBackendEntry,
} from '@/utils/shared-modules'
import BackendEditorDialog from '@/components/backends/BackendEditorDialog.vue'
import api from '@/api'
import { listBackendTypes } from '@/api/backend'

const loading = ref(false)
/** @type {import('vue').Ref<any[]>} 保留 API 原始结构，供编辑对话框使用 */
const providers = ref([])
const selectedProviders = ref([])
const healthProbing = ref({})
const healthStatuses = ref({})
const healthErrors = ref({})
/** @type {import('vue').Ref<Record<string,string>>} */
const backendTypeNames = ref({})

const editorRef = ref(null)
const editorVisible = ref(false)
const importInput = ref(null)

onMounted(() => {
  fetchProviders()
  listBackendTypes()
    .then((types) => {
      if (Array.isArray(types)) {
        /** @type {Record<string,string>} */
        const map = {}
        types.forEach((bt) => { map[bt.type] = bt.name })
        backendTypeNames.value = map
      }
    })
    .catch((err) => {
      console.error('Failed to load backend types', err)
    })
})

function fetchProviders() {
  loading.value = true
  api.get('/api/v1/backends')
    .then(res => {
      // api 拦截器已解包 {success,data} → data，此处 res 即为后端数组
      providers.value = Array.isArray(res) ? res : (Array.isArray(res?.data) ? res.data : [])
    })
    .catch(err => {
      console.error('Failed to fetch backends', err)
      ElMessage.error('获取后端列表失败')
    })
    .finally(() => { loading.value = false })
}

function modelNames(p) {
  const raw = p?.supported_models
  if (!Array.isArray(raw)) return []
  return raw
    .map((m) => {
      if (typeof m === 'string') return m
      return m?.requested_model || m?.actual_model || ''
    })
    .filter(Boolean)
}

function getTypeDisplayName(typeId) {
  if (!typeId) return ''
  if (backendTypeNames.value[typeId]) return backendTypeNames.value[typeId]
  const info = getProviderById(typeId)
  if (info) return info.name
  /** @type {Record<string,string>} */
  const labels = { openai: 'OpenAI 兼容', ollama: 'Ollama', anthropic: 'Anthropic' }
  return labels[typeId] || typeId
}

function deleteProvider(p) {
  ElMessageBox.confirm(
    `确定删除 "${p.name}" 吗？此操作不可恢复。`,
    '确认删除',
    { confirmButtonText: '删除', cancelButtonText: '取消', type: 'warning' }
  ).then(() => {
    loading.value = true
    return api.delete(`/api/v1/backends/${encodeURIComponent(p.id)}`)
  }).then(() => {
    ElMessage.success('删除成功')
    selectedProviders.value = selectedProviders.value.filter(id => id !== p.id)
    delete healthStatuses.value[p.id]
    fetchProviders()
  }).catch(err => {
    if (err !== 'cancel' && err !== 'close') {
      console.error('Delete backend failed', err)
      ElMessage.error(err?.response?.data?.message || '删除失败')
    }
  }).finally(() => { loading.value = false })
}

function batchDelete() {
  ElMessageBox.confirm(
    `确定删除选中的 ${selectedProviders.value.length} 个后端吗？`,
    '批量删除',
    { confirmButtonText: '删除', cancelButtonText: '取消', type: 'warning' }
  ).then(() => {
    loading.value = true
    return Promise.all(selectedProviders.value.map(id =>
      api.delete(`/api/v1/backends/${encodeURIComponent(id)}`)
    ))
  }).then(() => {
    ElMessage.success('批量删除成功')
    const ids = new Set(selectedProviders.value)
    selectedProviders.value = []
    ids.forEach(id => { delete healthStatuses.value[id] })
    fetchProviders()
  }).catch(err => {
    if (err !== 'cancel' && err !== 'close') {
      console.error('Batch delete failed', err)
      ElMessage.error('批量删除失败')
    }
  }).finally(() => { loading.value = false })
}

function toggleSelect(id) {
  const idx = selectedProviders.value.indexOf(id)
  if (idx >= 0) {
    selectedProviders.value.splice(idx, 1)
  } else {
    selectedProviders.value.push(id)
  }
}

function clearSelection() {
  selectedProviders.value = []
}

function healthStatusClass(id) {
  if (healthProbing.value[id]) return 'probing'
  if (healthStatuses.value[id] === true) return 'healthy'
  if (healthStatuses.value[id] === false) return 'unhealthy'
  return 'unknown'
}

function healthStatusText(id) {
  if (healthProbing.value[id]) return '检测中...'
  if (healthStatuses.value[id] === true) return '健康'
  if (healthStatuses.value[id] === false) return healthErrors.value[id] || '不健康'
  return '未检测'
}

function probeOne(id) {
  const p = providers.value.find(x => x.id === id)
  if (!p) return
  healthProbing.value[id] = true
  healthProbing.value = { ...healthProbing.value }
  api.post(`/api/v1/backends/${encodeURIComponent(id)}/probe`)
    .then(res => {
      const body = res && typeof res === 'object' && 'data' in res && !('healthy' in res) ? res.data : res
      healthStatuses.value[id] = body?.healthy !== false
      healthErrors.value[id] = body?.error || ''
    })
    .catch(err => {
      healthStatuses.value[id] = false
      healthErrors.value[id] = err?.response?.data?.message || err?.message || '检测失败'
    })
    .finally(() => {
      healthProbing.value[id] = false
      healthProbing.value = { ...healthProbing.value }
    })
}

function batchProbe() {
  selectedProviders.value.forEach(id => probeOne(id))
}

function triggerDownload(blob, filename) {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  URL.revokeObjectURL(url)
}

/** 导出接口含 api_key；转为 ConfigBuilder 中间态 */
function exportRowsToEntries(rows) {
  return (Array.isArray(rows) ? rows : []).map((row) => {
    const form = fromApiBackend(row)
    form.api_key = row.api_key || ''
    return toBackendEntry(form)
  })
}

async function fetchExportRows() {
  const res = await api.get('/api/v1/backends/export')
  if (Array.isArray(res)) return res
  if (Array.isArray(res?.backends)) return res.backends
  if (typeof res === 'string') {
    const parsed = JSON.parse(res)
    return Array.isArray(parsed) ? parsed : (parsed?.backends || [])
  }
  return []
}

function handleExportCommand(command) {
  if (command === 'json') return handleExportJSON()
  if (command === 'yaml') return handleExportYAML()
  if (command === 'zip') return handleExportZip()
}

function handleExportJSON() {
  loading.value = true
  fetchExportRows()
    .then((rows) => {
      const blob = new Blob([JSON.stringify(rows, null, 2)], { type: 'application/json' })
      triggerDownload(blob, 'centag-backends.json')
      ElMessage.success('已导出 JSON 备份')
    })
    .catch((err) => {
      console.error('Export failed', err)
      ElMessage.error('导出失败')
    })
    .finally(() => { loading.value = false })
}

function handleExportYAML() {
  loading.value = true
  fetchExportRows()
    .then((rows) => {
      const entries = exportRowsToEntries(rows)
      if (entries.length === 0) {
        ElMessage.warning('没有可导出的后端')
        return
      }
      const content = ConfigBuilder.buildBackendsYaml(entries)
      triggerDownload(new Blob([content], { type: 'text/yaml;charset=utf-8' }), 'initial-backends.yaml')
      ElMessage.success('已导出 backends YAML')
    })
    .catch((err) => {
      console.error('Export YAML failed', err)
      ElMessage.error('导出 YAML 失败')
    })
    .finally(() => { loading.value = false })
}

function handleExportZip() {
  loading.value = true
  fetchExportRows()
    .then(async (rows) => {
      const entries = exportRowsToEntries(rows)
      if (entries.length === 0) {
        ElMessage.warning('没有可导出的后端')
        return
      }
      const templateIds = Object.keys(ConfigBuilder.PIPELINE_TEMPLATES_DATA || {})
      const blob = await ConfigBuilder.exportAsArchive(entries, templateIds)
      triggerDownload(blob, 'centag-initdata.zip')
      ElMessage.success('已打包导出 initdata（backends + 流水线模板）')
    })
    .catch((err) => {
      console.error('Export ZIP failed', err)
      ElMessage.error('打包导出失败')
    })
    .finally(() => { loading.value = false })
}

function handleImport() {
  importInput.value?.click()
}

function parseImportPayload(text, fileName) {
  const trimmed = (text || '').trim()
  if (!trimmed) throw new Error('文件为空')

  const lower = (fileName || '').toLowerCase()
  const looksYaml = lower.endsWith('.yaml') || lower.endsWith('.yml') || (!trimmed.startsWith('{') && !trimmed.startsWith('['))

  let data
  if (looksYaml) {
    data = yaml.load(trimmed)
  } else {
    data = JSON.parse(trimmed)
  }

  if (Array.isArray(data)) return data
  if (data && Array.isArray(data.backends)) return data.backends
  if (data && typeof data === 'object') return [data]
  throw new Error('无法识别的后端配置格式')
}

function handleImportFile(e) {
  const file = e.target.files?.[0]
  if (!file) return
  const reader = new FileReader()
  reader.onload = (ev) => {
    try {
      const data = parseImportPayload(ev.target.result, file.name)
      ElMessageBox.confirm(
        `即将导入 ${data.length} 个后端配置。如果存在同名后端，将跳过导入。是否继续？`,
        '确认导入',
        { confirmButtonText: '导入', cancelButtonText: '取消', type: 'info' }
      ).then(() => {
        loading.value = true
        return api.post('/api/v1/backends/import', { backends: data })
      }).then(() => {
        ElMessage.success('导入成功')
        fetchProviders()
      }).catch(err => {
        if (err !== 'cancel' && err !== 'close') {
          console.error('Import failed', err)
          ElMessage.error(err?.response?.data?.message || '导入失败')
        }
      }).finally(() => { loading.value = false })
    } catch (err) {
      ElMessage.error('文件格式错误：' + err.message)
    }
  }
  reader.readAsText(file)
  e.target.value = ''
}
</script>

<style scoped>
.backends-page {
  max-width: 1200px;
  margin: 0 auto;
  padding: 24px;
}

.hermes-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 28px;
  flex-wrap: wrap;
  gap: 16px;
}
.hermes-header-left { flex: 1; }
.hermes-title {
  font-size: 28px;
  font-weight: 700;
  color: var(--el-text-color-primary, #303133);
  margin: 0 0 4px 0;
}
.hermes-subtitle {
  font-size: 14px;
  color: var(--el-text-color-secondary, #909399);
  margin: 0;
}
.hermes-header-right {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.toolbar-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 16px;
  background: var(--el-color-primary-light-9, #ecf5ff);
  border-radius: 8px;
  margin-bottom: 16px;
}
.toolbar-count {
  font-size: 13px;
  color: var(--el-color-primary, #409eff);
  font-weight: 500;
  margin-right: auto;
}

.card-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
  gap: 16px;
  margin-bottom: 24px;
}
.provider-card {
  background: var(--el-bg-color, #fff);
  border: 1.5px solid var(--el-border-color-lighter, #ebeef5);
  border-radius: 12px;
  padding: 16px;
  transition: all 0.2s;
  cursor: default;
  display: flex;
  gap: 12px;
}
.provider-card:hover {
  border-color: var(--el-color-primary-light-3, #a0cfff);
  box-shadow: 0 2px 12px rgba(0,0,0,0.06);
}
.provider-card.selected {
  border-color: var(--el-color-primary, #409eff);
  background: var(--el-color-primary-light-9, #ecf5ff);
}
.provider-card.is-loading { opacity: 0.7; }
.card-select-indicator {
  flex-shrink: 0;
  padding-top: 2px;
  cursor: pointer;
}
.check-box {
  display: inline-block;
  width: 18px;
  height: 18px;
  border: 2px solid var(--el-border-color, #dcdfe6);
  border-radius: 4px;
  transition: all 0.2s;
}
.check-box.checked {
  background: var(--el-color-primary, #409eff);
  border-color: var(--el-color-primary, #409eff);
}
.check-box.checked::after {
  content: '';
  display: block;
  width: 5px;
  height: 9px;
  border: solid #fff;
  border-width: 0 2px 2px 0;
  transform: rotate(45deg);
  margin: 2px 0 0 5px;
}
.card-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
}
.card-body { flex: 1; min-width: 0; }
.card-header { margin-bottom: 8px; }
.card-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
}
.health-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex-shrink: 0;
}
.health-dot.healthy { background: #67c23a; }
.health-dot.unhealthy { background: #f56c6c; }
.health-dot.unknown { background: #c0c4cc; }
.health-dot.probing {
  background: #e6a23c;
  animation: pulse 1s infinite;
}
@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.4; }
}
.card-title {
  font-size: 15px;
  font-weight: 600;
  margin: 0;
  color: var(--el-text-color-primary, #303133);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.provider-type-badge {
  font-size: 11px;
  padding: 1px 8px;
  border-radius: 10px;
  background: var(--el-fill-color, #f0f2f5);
  color: var(--el-text-color-secondary, #909399);
  white-space: nowrap;
}
.card-meta {
  font-size: 12px;
  color: var(--el-text-color-secondary, #909399);
  word-break: break-all;
}
.card-meta code {
  background: var(--el-fill-color, #f0f2f5);
  padding: 1px 4px;
  border-radius: 3px;
  font-size: 11px;
}
.model-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-top: 8px;
}
.model-tag {
  font-size: 11px;
  padding: 2px 8px;
  border-radius: 10px;
  background: var(--el-fill-color, #f0f2f5);
  color: var(--el-text-color-regular, #606266);
  white-space: nowrap;
}
.model-tag.highlight {
  background: var(--el-color-primary-light-9, #ecf5ff);
  color: var(--el-color-primary, #409eff);
  border: 1px solid var(--el-color-primary-light-3, #a0cfff);
}
.model-tag.more {
  background: transparent;
  color: var(--el-color-primary, #409eff);
  cursor: default;
}
.card-footer {
  display: flex;
  gap: 6px;
  margin-top: auto;
  padding-top: 12px;
  border-top: 1px solid var(--el-border-color-lighter, #ebeef5);
}

.empty-state {
  text-align: center;
  padding: 60px 20px;
  color: var(--el-text-color-secondary, #909399);
}
.empty-icon { font-size: 48px; margin-bottom: 16px; }
.empty-state h3 {
  font-size: 18px;
  margin: 0 0 8px 0;
  color: var(--el-text-color-primary, #303133);
}
.empty-state p { margin: 0 0 20px 0; }

.loading-state {
  text-align: center;
  padding: 40px;
  color: var(--el-text-color-secondary, #909399);
}

.btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 18px;
  border: 1.5px solid transparent;
  border-radius: 8px;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
  background: transparent;
  white-space: nowrap;
}
.btn:disabled { opacity: 0.5; cursor: not-allowed; }
.btn-primary {
  background: var(--el-color-primary, #409eff);
  color: #fff;
  border-color: var(--el-color-primary, #409eff);
}
.btn-primary:hover:not(:disabled) {
  background: var(--el-color-primary-light-3, #79bbff);
}
.btn-outline {
  background: var(--el-bg-color, #fff);
  color: var(--el-text-color-regular, #606266);
  border-color: var(--el-border-color, #dcdfe6);
}
.btn-outline:hover:not(:disabled) {
  border-color: var(--el-color-primary, #409eff);
  color: var(--el-color-primary, #409eff);
}
.btn-sm {
  padding: 4px 12px;
  font-size: 13px;
  border-radius: 6px;
}
.btn-danger {
  background: #f56c6c;
  color: #fff;
  border-color: #f56c6c;
}
.btn-danger:hover:not(:disabled) {
  background: #f78989;
}
.btn-danger-text {
  color: #f56c6c;
  border: none;
  background: none;
  padding: 4px 8px;
}
.btn-danger-text:hover:not(:disabled) {
  background: rgba(245,108,108,0.1);
}
.btn-text {
  border: none;
  background: none;
  padding: 4px 8px;
  color: var(--el-text-color-secondary, #909399);
}
.btn-text:hover:not(:disabled) {
  color: var(--el-text-color-primary, #303133);
}
</style>

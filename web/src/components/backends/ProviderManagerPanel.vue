<template>
  <div class="provider-manager" v-loading="busy">
    <div v-if="!canWrite" class="readonly-tip">
      当前为只读：团队版仅管理员可添加、修改或探测后端
    </div>
    <div v-if="canWrite" class="mgr-toolbar">
      <div class="mgr-toolbar-left">
        <el-checkbox
          v-if="backends.length > 0"
          :model-value="allSelected"
          :indeterminate="partialSelected"
          @change="toggleSelectAll"
        >
          全选
        </el-checkbox>
        <template v-if="selectedIds.length > 0">
          <span class="toolbar-count">已选 {{ selectedIds.length }} 项</span>
          <el-button size="small" :loading="batchProbing" @click="handleBatchProbe">
            批量探测
          </el-button>
          <el-button size="small" type="danger" :loading="batchDeleting" @click="handleBatchDelete">
            批量删除
          </el-button>
          <el-button size="small" text @click="clearSelection">取消选择</el-button>
        </template>
      </div>
      <div class="mgr-toolbar-right">
        <el-button size="small" :disabled="busy" @click="handleImport">导入</el-button>
        <el-dropdown trigger="click" :disabled="busy" @command="handleExportCommand">
          <el-button size="small" :disabled="busy">导出 ▾</el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="json">JSON 备份</el-dropdown-item>
              <el-dropdown-item command="yaml">backends YAML</el-dropdown-item>
              <el-dropdown-item command="zip">initdata 打包 (ZIP)</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
        <el-button
          v-if="backends.length > 0"
          size="small"
          :loading="probingAll"
          @click="handleProbeAll"
        >
          全部探测
        </el-button>
        <el-button type="primary" size="small" @click="openCreate">+ 添加 Provider</el-button>
      </div>
    </div>

    <div class="backend-grid">
      <div
        v-for="b in backends"
        :key="b.id"
        class="backend-card"
        :class="{ 'is-default': defaultBackendId === b.id, selected: selectedIds.includes(b.id) }"
      >
        <div class="backend-card-head">
          <div class="backend-card-title">
            <el-checkbox
              v-if="canWrite"
              :model-value="selectedIds.includes(b.id)"
              class="row-checkbox"
              @change="(checked) => toggleSelect(b.id, !!checked)"
              @click.stop
            />
            <span
              class="health-dot"
              :class="healthStatusClass(b.id)"
              :title="healthStatusText(b.id)"
            />
            <div class="backend-info">
              <div class="backend-name">
                {{ b.name }}
                <el-tag
                  v-if="defaultBackendId === b.id"
                  type="success"
                  size="small"
                  effect="light"
                  class="default-tag"
                >
                  默认
                </el-tag>
              </div>
              <div class="backend-meta">{{ b.type }} · {{ b.base_url || '默认端点' }}</div>
            </div>
          </div>
          <el-switch
            :model-value="b.enabled"
            :loading="togglingMap[b.id]"
            :disabled="!canWrite"
            active-color="#10b981"
            size="small"
            @change="(enabled) => handleToggle(b, enabled)"
          />
        </div>

        <div class="backend-card-body">
          <div v-if="b.probe_model || b.default_model" class="backend-default-model">
            默认模型：<span class="model-name">{{ b.default_model || b.probe_model }}</span>
          </div>
          <div class="backend-stats">
            <span>权重 {{ b.weight ?? 1 }}</span>
            <span v-if="b.supported_models?.length">{{ b.supported_models.length }} 模型</span>
          </div>
        </div>

        <div v-if="canWrite" class="backend-actions">
          <el-button
            size="small"
            :type="defaultBackendId === b.id ? 'success' : 'default'"
            :plain="defaultBackendId !== b.id"
            :disabled="defaultBackendId === b.id || settingDefaultMap[b.id]"
            :loading="settingDefaultMap[b.id]"
            @click="handleSetDefault(b)"
          >
            {{ defaultBackendId === b.id ? '当前默认' : '设为默认' }}
          </el-button>
          <el-dropdown
            trigger="click"
            @command="(cmd: string) => handleCardAction(cmd, b)"
          >
            <el-button size="small">
              操作
              <el-icon class="el-icon--right"><ArrowDown /></el-icon>
            </el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="test" :disabled="!!testingMap[b.id]">
                  测试
                </el-dropdown-item>
                <el-dropdown-item command="edit">编辑</el-dropdown-item>
                <el-dropdown-item
                  command="delete"
                  divided
                  :disabled="defaultBackendId === b.id"
                >
                  删除
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </div>
      <div v-if="!backends.length" class="empty-tip">
        {{ canWrite ? '暂无后端配置，点击「+ 添加 Provider」开始' : '暂无后端配置' }}
      </div>
    </div>

    <BackendEditorDialog
      ref="editorRef"
      v-model="editorVisible"
      @saved="emit('refresh')"
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

<script setup lang="ts">
import { ref, reactive, computed, onMounted, watch } from 'vue'
import { ArrowDown } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import * as yaml from 'js-yaml'
import BackendEditorDialog from '@/components/backends/BackendEditorDialog.vue'
import { updateBackend, deleteBackend } from '@/api'
import { getBackendTestMessage, testBackendConnection } from '@/utils/backendTest'
import {
  ConfigBuilder,
  fromApiBackend,
  toBackendEntry
} from '@/utils/shared-modules'
import api from '@/api'
import { useAuthStore } from '@/stores/auth'
import { useEdition } from '@/composables/useEdition'

const props = defineProps<{
  backends: any[]
}>()

const emit = defineEmits<{
  refresh: []
  'backend-updated': [backend: any]
}>()

const authStore = useAuthStore()
const { isPersonal, isMinimal } = useEdition()
/** team 仅 admin 可写；personal / minimal 保持原有写权限 */
const canWrite = computed(() => isPersonal.value || isMinimal.value || authStore.isAdmin)

const editorRef = ref<InstanceType<typeof BackendEditorDialog> | null>(null)
const editorVisible = ref(false)
const importInput = ref<HTMLInputElement | null>(null)
const busy = ref(false)
const batchDeleting = ref(false)
const batchProbing = ref(false)
const probingAll = ref(false)
const testingMap = reactive<Record<string, boolean>>({})
const togglingMap = reactive<Record<string, boolean>>({})
const settingDefaultMap = reactive<Record<string, boolean>>({})
const probingMap = reactive<Record<string, boolean>>({})
const healthStatuses = reactive<Record<string, boolean | undefined>>({})
const healthErrors = reactive<Record<string, string>>({})
const defaultBackendId = ref('')
const selectedIds = ref<string[]>([])

const allSelected = computed(
  () => props.backends.length > 0 && selectedIds.value.length === props.backends.length
)
const partialSelected = computed(
  () => selectedIds.value.length > 0 && selectedIds.value.length < props.backends.length
)

watch(
  () => props.backends,
  (list) => {
    const valid = new Set(list.map((b: any) => b.id))
    selectedIds.value = selectedIds.value.filter((id) => valid.has(id))
  },
  { deep: true }
)

onMounted(() => {
  loadDefaultBackend()
})

async function loadDefaultBackend() {
  try {
    const res: any = await api.get('/api/v1/config/proxy')
    const data = res?.data ?? res
    defaultBackendId.value = data?.default_backend_id || ''
  } catch {
    /* ignore */
  }
}

function toggleSelect(id: string, checked: boolean) {
  if (checked) {
    if (!selectedIds.value.includes(id)) selectedIds.value = [...selectedIds.value, id]
  } else {
    selectedIds.value = selectedIds.value.filter((x) => x !== id)
  }
}

function toggleSelectAll(checked: boolean | string | number) {
  selectedIds.value = checked ? props.backends.map((b: any) => b.id) : []
}

function clearSelection() {
  selectedIds.value = []
}

function healthStatusClass(id: string) {
  if (probingMap[id]) return 'probing'
  if (healthStatuses[id] === true) return 'healthy'
  if (healthStatuses[id] === false) return 'unhealthy'
  return 'unknown'
}

function healthStatusText(id: string) {
  if (probingMap[id]) return '检测中...'
  if (healthStatuses[id] === true) return '健康'
  if (healthStatuses[id] === false) return healthErrors[id] || '不健康'
  return '未检测'
}

async function probeOne(id: string) {
  probingMap[id] = true
  try {
    const res: any = await api.post(`/api/v1/backends/${encodeURIComponent(id)}/probe`)
    const body = res && typeof res === 'object' && 'data' in res && !('healthy' in res) ? res.data : res
    healthStatuses[id] = body?.healthy !== false
    healthErrors[id] = body?.error || ''
    if (healthStatuses[id]) ElMessage.success(`「${id}」探测成功`)
    else ElMessage.warning(`「${id}」探测异常：${healthErrors[id] || '不健康'}`)
    emit('refresh')
  } catch (err: any) {
    healthStatuses[id] = false
    healthErrors[id] = err?.response?.data?.message || err?.message || '检测失败'
    ElMessage.error(`「${id}」探测失败：${healthErrors[id]}`)
  } finally {
    probingMap[id] = false
  }
}

async function handleBatchProbe() {
  if (!selectedIds.value.length) return
  batchProbing.value = true
  try {
    await Promise.all(selectedIds.value.map((id) => probeOne(id)))
  } finally {
    batchProbing.value = false
  }
}

async function handleProbeAll() {
  probingAll.value = true
  try {
    // Prefer probe-all when available; fallback to per-id
    try {
      const res: any = await api.post('/api/v1/backends/probe-all')
      const results = Array.isArray(res) ? res : res?.results || res?.data || []
      if (Array.isArray(results) && results.length) {
        for (const r of results) {
          const id = r.backend_id || r.id
          if (!id) continue
          healthStatuses[id] = r.success !== false && r.healthy !== false
          healthErrors[id] = r.error || ''
        }
        ElMessage.success('全部探测完成')
        emit('refresh')
        return
      }
    } catch {
      /* fallback */
    }
    await Promise.all(props.backends.map((b: any) => probeOne(b.id)))
  } finally {
    probingAll.value = false
  }
}

async function handleBatchDelete() {
  if (!selectedIds.value.length) return
  const ids = selectedIds.value.filter((id) => id !== defaultBackendId.value)
  if (!ids.length) {
    ElMessage.warning('不能删除当前默认后端，请先设置其他后端为默认')
    return
  }
  const skippedDefault = ids.length < selectedIds.value.length
  try {
    await ElMessageBox.confirm(
      skippedDefault
        ? `将删除选中的 ${ids.length} 个后端（已跳过当前默认），此操作不可恢复。`
        : `确定删除选中的 ${ids.length} 个后端吗？此操作不可恢复。`,
      '批量删除',
      { confirmButtonText: '删除', cancelButtonText: '取消', type: 'warning' }
    )
  } catch {
    return
  }
  batchDeleting.value = true
  try {
    await Promise.all(ids.map((id) => deleteBackend(id)))
    ElMessage.success(`已删除 ${ids.length} 个后端`)
    selectedIds.value = []
    emit('refresh')
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.message || '批量删除失败')
    emit('refresh')
  } finally {
    batchDeleting.value = false
  }
}

async function handleSetDefault(backend: any) {
  if (defaultBackendId.value === backend.id) return
  settingDefaultMap[backend.id] = true
  try {
    const sm = Array.isArray(backend.supported_models) ? backend.supported_models : []
    const defaultModel =
      backend.default_model ||
      backend.probe_model ||
      (sm[0] && (sm[0].actual_model || sm[0].requested_model)) ||
      ''
    await api.put('/api/v1/config/proxy', {
      default_backend_id: backend.id,
      default_model: defaultModel
    })
    defaultBackendId.value = backend.id
    ElMessage.success(
      `已将「${backend.name || backend.id}」设为默认后端，模型「${defaultModel || '未设置'}」`
    )
    emit('refresh')
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.message || '设置默认后端失败')
  } finally {
    settingDefaultMap[backend.id] = false
  }
}

async function handleToggle(backend: any, enabled: boolean) {
  if (!canWrite.value) return
  togglingMap[backend.id] = true
  try {
    const updated = await updateBackend(backend.id, { ...backend, enabled })
    emit('backend-updated', updated?.id ? updated : { id: backend.id, enabled })
    ElMessage.success(`${backend.name || backend.id} ${enabled ? '已启用' : '已禁用'}`)
  } catch (error: any) {
    ElMessage.error('操作失败：' + (error.message || '未知错误'))
    emit('refresh')
  } finally {
    togglingMap[backend.id] = false
  }
}

function showTestResult(updatedBackend: any, fallbackName: string) {
  const { level, text } = getBackendTestMessage(updatedBackend, fallbackName)
  if (level === 'success') ElMessage.success(text)
  else ElMessage.warning(text)
}

async function handleTest(backend: any) {
  testingMap[backend.id] = true
  try {
    const apiKey = editorRef.value?.getPendingApiKey?.(backend.id)
    const updatedBackend = await testBackendConnection(backend, apiKey ? { apiKey } : undefined)
    if (updatedBackend && updatedBackend.id) {
      emit('backend-updated', updatedBackend)
      showTestResult(updatedBackend, backend.id)
    } else {
      ElMessage.warning(`${backend.name || backend.id} 连接测试完成，但未返回完整数据`)
      emit('refresh')
    }
  } catch (error: any) {
    ElMessage.error(`${backend.name || backend.id} 连接失败：${error.message}`)
  } finally {
    testingMap[backend.id] = false
  }
}

function handleCardAction(command: string, backend: any) {
  switch (command) {
    case 'test':
      void handleTest(backend)
      break
    case 'edit':
      handleEdit(backend)
      break
    case 'delete':
      void handleDelete(backend)
      break
  }
}

function handleEdit(backend: any) {
  editorRef.value?.openEdit(backend)
}

async function handleDelete(backend: any) {
  if (defaultBackendId.value === backend.id) {
    ElMessage.warning('不能删除当前默认后端，请先设置其他后端为默认')
    return
  }
  try {
    await ElMessageBox.confirm(
      `确定删除后端「${backend.name || backend.id}」吗？此操作不可恢复。`,
      '确认删除',
      { confirmButtonText: '删除', cancelButtonText: '取消', type: 'warning' }
    )
  } catch {
    return
  }
  try {
    await deleteBackend(backend.id)
    ElMessage.success(`已删除后端「${backend.name || backend.id}」`)
    emit('refresh')
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.message || '删除后端失败')
  }
}

function openCreate() {
  if (!canWrite.value) {
    ElMessage.warning('团队版仅管理员可添加后端')
    return
  }
  editorRef.value?.openCreate()
}

function reloadDefault() {
  loadDefaultBackend()
}

function triggerDownload(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  URL.revokeObjectURL(url)
}

function exportRowsToEntries(rows: any[]) {
  return (Array.isArray(rows) ? rows : []).map((row) => {
    const form = fromApiBackend(row)
    form.api_key = row.api_key || ''
    return toBackendEntry(form)
  })
}

async function fetchExportRows() {
  const res: any = await api.get('/api/v1/backends/export')
  if (Array.isArray(res)) return res
  if (Array.isArray(res?.backends)) return res.backends
  if (typeof res === 'string') {
    const parsed = JSON.parse(res)
    return Array.isArray(parsed) ? parsed : parsed?.backends || []
  }
  return []
}

function handleExportCommand(command: string) {
  if (command === 'json') return handleExportJSON()
  if (command === 'yaml') return handleExportYAML()
  if (command === 'zip') return handleExportZip()
}

async function handleExportJSON() {
  busy.value = true
  try {
    const rows = await fetchExportRows()
    triggerDownload(
      new Blob([JSON.stringify(rows, null, 2)], { type: 'application/json' }),
      'centag-backends.json'
    )
    ElMessage.success('已导出 JSON 备份（含密钥，请妥善保管）')
  } catch {
    ElMessage.error('导出失败')
  } finally {
    busy.value = false
  }
}

async function handleExportYAML() {
  busy.value = true
  try {
    const rows = await fetchExportRows()
    const entries = exportRowsToEntries(rows)
    if (!entries.length) {
      ElMessage.warning('没有可导出的后端')
      return
    }
    const content = ConfigBuilder.buildBackendsYaml(entries)
    triggerDownload(new Blob([content], { type: 'text/yaml;charset=utf-8' }), 'initial-backends.yaml')
    ElMessage.success('已导出 backends YAML')
  } catch {
    ElMessage.error('导出 YAML 失败')
  } finally {
    busy.value = false
  }
}

async function handleExportZip() {
  busy.value = true
  try {
    const rows = await fetchExportRows()
    const entries = exportRowsToEntries(rows)
    if (!entries.length) {
      ElMessage.warning('没有可导出的后端')
      return
    }
    const templateIds = Object.keys(ConfigBuilder.PIPELINE_TEMPLATES_DATA || {})
    const blob = await ConfigBuilder.exportAsArchive(entries, templateIds)
    triggerDownload(blob, 'centag-initdata.zip')
    ElMessage.success('已打包导出 initdata')
  } catch {
    ElMessage.error('打包导出失败')
  } finally {
    busy.value = false
  }
}

function handleImport() {
  importInput.value?.click()
}

function parseImportPayload(text: string, fileName: string) {
  const trimmed = (text || '').trim()
  if (!trimmed) throw new Error('文件为空')
  const lower = (fileName || '').toLowerCase()
  const looksYaml =
    lower.endsWith('.yaml') ||
    lower.endsWith('.yml') ||
    (!trimmed.startsWith('{') && !trimmed.startsWith('['))
  const data = looksYaml ? yaml.load(trimmed) : JSON.parse(trimmed)
  if (Array.isArray(data)) return data
  if (data && Array.isArray((data as any).backends)) return (data as any).backends
  if (data && typeof data === 'object') return [data]
  throw new Error('无法识别的后端配置格式')
}

function handleImportFile(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  const reader = new FileReader()
  reader.onload = (ev) => {
    try {
      const data = parseImportPayload(String(ev.target?.result || ''), file.name)
      ElMessageBox.confirm(
        `即将导入 ${data.length} 个后端配置。同名 id 已存在则跳过。是否继续？`,
        '确认导入',
        { confirmButtonText: '导入', cancelButtonText: '取消', type: 'info' }
      )
        .then(() => {
          busy.value = true
          return api.post('/api/v1/backends/import', { backends: data })
        })
        .then(() => {
          ElMessage.success('导入成功')
          emit('refresh')
        })
        .catch((err) => {
          if (err !== 'cancel' && err !== 'close') {
            ElMessage.error(err?.response?.data?.message || err?.message || '导入失败')
          }
        })
        .finally(() => {
          busy.value = false
        })
    } catch (err: any) {
      ElMessage.error('文件格式错误：' + err.message)
    }
  }
  reader.readAsText(file)
  input.value = ''
}

defineExpose({ openCreate, reloadDefault })
</script>

<style scoped>
.provider-manager {
  display: flex;
  flex-direction: column;
  gap: 12px;
  width: 100%;
  min-height: 0;
}

.readonly-tip {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  line-height: 1.5;
  padding: 8px 10px;
  background: var(--el-fill-color-light);
  border-radius: 6px;
}

.mgr-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  flex-wrap: wrap;
}

.mgr-toolbar-left,
.mgr-toolbar-right {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.toolbar-count {
  font-size: 0.75rem;
  color: #6b7280;
}

.backend-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 12px;
  width: 100%;
}

.backend-card {
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-height: 160px;
  padding: 14px;
  background: #f9fafb;
  border-radius: 10px;
  border: 1px solid #eef0f3;
  transition: border-color 0.2s, box-shadow 0.2s, background 0.2s;
}

.backend-card:hover {
  border-color: #dbe3f0;
  box-shadow: 0 2px 8px rgba(15, 23, 42, 0.04);
}

.backend-card.is-default {
  background: #f0f9eb;
  border-color: #b3e19d;
}

.backend-card.selected {
  border-color: #93c5fd;
  background: #eff6ff;
}

.backend-card-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
}

.backend-card-title {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  min-width: 0;
  flex: 1;
}

.row-checkbox {
  flex-shrink: 0;
  margin-top: 2px;
}

.health-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
  margin-top: 6px;
  background: #d1d5db;
}
.health-dot.healthy {
  background: #10b981;
}
.health-dot.unhealthy {
  background: #ef4444;
}
.health-dot.probing {
  background: #f59e0b;
  animation: pulse 1s infinite;
}
.health-dot.unknown {
  background: #d1d5db;
}

@keyframes pulse {
  50% {
    opacity: 0.4;
  }
}

.default-tag {
  margin-left: 6px;
}

.backend-info {
  min-width: 0;
  flex: 1;
}

.backend-name {
  font-size: 0.9rem;
  font-weight: 600;
  color: #111827;
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 2px;
  line-height: 1.3;
}

.backend-meta {
  font-size: 0.75rem;
  color: #9ca3af;
  margin-top: 4px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.backend-card-body {
  flex: 1;
  min-height: 0;
}

.backend-default-model {
  font-size: 0.75rem;
  color: #6b7280;
}

.backend-default-model .model-name {
  color: #409eff;
  font-weight: 500;
}

.backend-stats {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 8px;
  font-size: 0.75rem;
  color: #9ca3af;
}

.backend-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding-top: 8px;
  border-top: 1px solid rgba(15, 23, 42, 0.06);
}

.empty-tip {
  grid-column: 1 / -1;
  text-align: center;
  color: #9ca3af;
  font-size: 0.8rem;
  padding: 24px 0;
}
</style>

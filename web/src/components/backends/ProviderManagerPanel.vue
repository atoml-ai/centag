<template>
  <div class="provider-manager" v-loading="busy">
    <div v-if="!canWrite" class="readonly-tip">
      {{ t('providerManager.readOnlyNotice') }}
    </div>
    <div v-if="canWrite" class="mgr-toolbar">
      <div class="mgr-toolbar-left">
        <el-button type="primary" size="small" @click="openCreate">
          <el-icon><Plus /></el-icon>
          {{ t('providerManager.addProvider') }}
        </el-button>
        <el-button size="small" :disabled="busy" @click="handleImport">
          <el-icon><Upload /></el-icon>
          {{ t('providerManager.import') }}
        </el-button>
        <el-dropdown trigger="click" :disabled="busy" @command="handleExportCommand">
          <el-button size="small" :disabled="busy">
            <el-icon><Download /></el-icon>
            {{ t('providerManager.export') }}
          </el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="json">{{ t('providerManager.jsonBackup') }}</el-dropdown-item>
              <el-dropdown-item command="yaml">{{ t('providerManager.yamlExport') }}</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
        <el-button
          v-if="backends.length > 0"
          size="small"
          :loading="probingAll"
          @click="handleProbeAll"
        >
          <el-icon><Connection /></el-icon>
          {{ t('providerManager.allProbe') }}
        </el-button>
        <el-button size="small" @click="router.push('/model-config')">
          <el-icon><Setting /></el-icon>
          {{ t('providerManager.modelVariables') }}
        </el-button>
      </div>
      <div class="mgr-toolbar-right">
        <el-checkbox
          v-if="backends.length > 0"
          :model-value="allSelected"
          :indeterminate="partialSelected"
          @change="toggleSelectAll"
        >
          {{ t('providerManager.selectAll') }}
        </el-checkbox>
        <template v-if="selectedIds.length > 0">
          <span class="toolbar-count">{{ t('providerManager.selectedCount', { count: selectedIds.length }) }}</span>
          <el-button size="small" :loading="batchProbing" @click="handleBatchProbe">
            {{ t('providerManager.batchProbe') }}
          </el-button>
          <el-button size="small" type="danger" :loading="batchDeleting" @click="handleBatchDelete">
            {{ t('providerManager.batchDelete') }}
          </el-button>
          <el-button size="small" text @click="clearSelection">{{ t('providerManager.cancelSelection') }}</el-button>
        </template>
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
                  v-if="!b.tenant_id"
                  type="info"
                  size="small"
                  effect="plain"
                  class="default-tag"
                >
                  {{ t('providerManager.scopeSystem') }}
                </el-tag>
                <el-tag
                  v-else
                  type="warning"
                  size="small"
                  effect="plain"
                  class="default-tag"
                >
                  {{ t('providerManager.scopeMine') }}
                </el-tag>
                <el-tag
                  v-if="defaultBackendId === b.id"
                  type="success"
                  size="small"
                  effect="light"
                  class="default-tag"
                >
                   {{ t('providerManager.default') }}
                 </el-tag>
              </div>
              <div class="backend-meta">{{ b.type }} · {{ b.base_url || t('providerManager.defaultEndpoint') }}</div>
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
          <div class="backend-stats">
            <span>{{ t('providerManager.weight') }} {{ b.weight ?? 1 }}</span>
            <span v-if="b.supported_models?.length">{{ t('providerManager.modelsCount', { count: b.supported_models.length }) }}</span>
            <span
              v-if="b.account_pool_summary && b.account_pool_summary.total_accounts > 0"
              class="account-pool-badge"
              :class="'pool-' + b.account_pool_summary.health_status"
            >
              {{ t('providerManager.keysCount', { enabled: b.account_pool_summary.enabled_accounts, total: b.account_pool_summary.total_accounts }) }}
            </span>
          </div>
          <div class="default-model-row">
            <div class="default-model-label">
              {{ t('providerManager.defaultModelVar') }}
            </div>
            <el-select
              :model-value="backendDefaultModel(b)"
              size="small"
              filterable
              allow-create
              default-first-option
              placeholder="默认模型"
              style="width: 100%"
              :disabled="!canEditBackendModel(b)"
              @change="(m) => handleBackendDefaultModelChange(b, m)"
            >
              <el-option
                v-for="m in b.supported_models"
                :key="m.actual_model || m.requested_model || m"
                :label="m.actual_model || m.requested_model || m"
                :value="m.actual_model || m.requested_model || m"
              />
            </el-select>
          </div>
        </div>

        <div v-if="canWrite" class="backend-actions">
          <el-tooltip :content="defaultBackendId === b.id ? t('providerManager.currentDefault') : t('providerManager.setDefault')" placement="top">
            <el-icon
              class="action-star"
              :class="{ 'is-default': defaultBackendId === b.id }"
              :disabled="defaultBackendId === b.id || settingDefaultMap[b.id]"
              @click="handleSetDefault(b)"
            >
              <StarFilled v-if="defaultBackendId === b.id" />
              <Star v-else />
            </el-icon>
          </el-tooltip>
          <el-dropdown
            trigger="click"
            @command="(cmd: string) => handleCardAction(cmd, b)"
          >
            <el-button circle plain size="small">
              <el-icon><MoreFilled /></el-icon>
            </el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="test" :disabled="!!testingMap[b.id]">
                  {{ t('providerManager.test') }}
                </el-dropdown-item>
                <el-dropdown-item command="edit">{{ t('providerManager.edit') }}</el-dropdown-item>
                <el-dropdown-item
                  command="delete"
                  divided
                >
                  {{ t('providerManager.delete') }}
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </div>
      <div v-if="!backends.length" class="empty-tip">
        {{ canWrite ? t('providerManager.emptyWithWrite') : t('providerManager.emptyWithoutWrite') }}
      </div>
    </div>

    <BackendEditorDialog
      ref="editorRef"
      v-model="editorVisible"
      @saved="handleEditorSaved"
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
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Plus, Upload, Download, Connection, Setting, Star, StarFilled, MoreFilled } from '@element-plus/icons-vue'
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
import { useUserResourceAccess } from '@/composables/useUserResourceAccess'

const props = defineProps<{
  backends: any[]
}>()

const emit = defineEmits<{
  refresh: []
  'backend-updated': [backend: any]
}>()

const router = useRouter()
const { t } = useI18n()
// personal / minimal / 超管不受限；team 普通用户受 can_add_own_backends 控制
const { canAddOwnBackends, unrestricted } = useUserResourceAccess()
const canWrite = computed(() => canAddOwnBackends.value)

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
const defaultModel = ref('')
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
    defaultModel.value = data?.default_model || ''
  } catch {
    /* ignore */
  }
}

// 卡片展示的默认模型：后端自身 default_model，无则回退 probe_model / 首个支持模型
function backendDefaultModel(b: any): string {
  const first = b?.supported_models?.[0]
  const firstModel = first && (first.actual_model || first.requested_model || first)
  return b?.default_model || b?.probe_model || firstModel || ''
}

// team 普通用户：自己添加的后端（有 tenant_id）可编辑；管理权限添加的系统后端只读
function canEditBackendModel(b: any): boolean {
  if (!canWrite.value) return false
  if (unrestricted.value) return true
  return !!b?.tenant_id
}

// 修改卡片默认模型：写入后端 default_model 并同步 probe_model；
// 若该卡片为全局默认后端，同时写入 config/proxy.default_model，供 {{system.default_model}} 模板变量使用
async function handleBackendDefaultModelChange(b: any, model: string) {
  try {
    const updated = await updateBackend(b.id, { ...b, default_model: model, probe_model: model })
    emit('backend-updated', updated)
    if (defaultBackendId.value === b.id && model !== defaultModel.value) {
      defaultModel.value = model
      await api.put('/api/v1/config/proxy', {
        default_backend_id: defaultBackendId.value,
        default_model: model
      })
    }
    ElMessage.success(t('providerManager.defaultModelSaved', { model: model || '（空）' }))
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.message || t('providerManager.defaultModelSaveFailed'))
  }
}

// 新增/编辑后端后同步默认后端与模型（否则新设的默认卡片要刷新才变绿）
async function handleEditorSaved() {
  await loadDefaultBackend()
  emit('refresh')
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
  if (probingMap[id]) return t('providerManager.statusProbing')
  if (healthStatuses[id] === true) return t('providerManager.statusHealthy')
  if (healthStatuses[id] === false) return healthErrors[id] || t('providerManager.statusUnhealthy')
  return t('providerManager.statusUnknown')
}

async function probeOne(id: string) {
  probingMap[id] = true
  try {
    const res: any = await api.post(`/api/v1/backends/${encodeURIComponent(id)}/probe`)
    const body = res && typeof res === 'object' && 'data' in res && !('healthy' in res) ? res.data : res
    healthStatuses[id] = body?.healthy !== false
    healthErrors[id] = body?.error || ''
    if (healthStatuses[id]) ElMessage.success(t('providerManager.probeSuccess'))
    else ElMessage.warning(t('providerManager.probeAbnormal') + (healthErrors[id] || t('providerManager.statusUnhealthy')))
    emit('refresh')
  } catch (err: any) {
    healthStatuses[id] = false
    healthErrors[id] = err?.response?.data?.message || err?.message || t('providerManager.statusUnhealthy')
    ElMessage.error(t('providerManager.probeFailed') + healthErrors[id])
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
        ElMessage.success(t('providerManager.allProbeCompleted'))
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
  const ids = selectedIds.value
  try {
    await ElMessageBox.confirm(
      t('providerManager.batchDeleteConfirm', { count: ids.length }),
      t('providerManager.batchDeleteTitle'),
      { confirmButtonText: t('providerManager.confirmDelete'), cancelButtonText: t('providerManager.cancel'), type: 'warning' }
    )
  } catch {
    return
  }
  batchDeleting.value = true
  try {
    await Promise.all(ids.map((id) => deleteBackend(id)))
    ElMessage.success(t('providerManager.deletedCount', { count: ids.length }))
    selectedIds.value = []
    emit('refresh')
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.message || t('providerManager.batchDeleteFailed'))
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
    // 设为默认时同步写入默认模型，保证 {{system.default_model}} 模板变量可用
    await api.put('/api/v1/config/proxy', {
      default_backend_id: backend.id,
      default_model: defaultModel
    })
    defaultBackendId.value = backend.id
    ElMessage.success(
      t('providerManager.setDefaultSuccess', { name: backend.name || backend.id, model: defaultModel || '' })
    )
    emit('refresh')
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.message || t('providerManager.setDefaultFailed'))
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
    ElMessage.success(enabled ? t('providerManager.toggledEnabled', { name: backend.name || backend.id }) : t('providerManager.toggledDisabled', { name: backend.name || backend.id }))
  } catch (error: any) {
    ElMessage.error(t('providerManager.operationFailed') + (error.message || t('providerManager.unknownError')))
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
      ElMessage.warning(t('providerManager.testIncomplete'))
      emit('refresh')
    }
  } catch (error: any) {
    ElMessage.error(t('providerManager.testFailed', { name: backend.name || backend.id, message: error.message }))
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
  try {
    await ElMessageBox.confirm(
      t('providerManager.confirmDeleteText', { name: backend.name || backend.id }),
      t('providerManager.confirmDeleteTitle'),
      { confirmButtonText: t('providerManager.confirmDelete'), cancelButtonText: t('providerManager.cancel'), type: 'warning' }
    )
  } catch {
    return
  }
  try {
    await deleteBackend(backend.id)
    ElMessage.success(t('providerManager.deleteSuccess', { name: backend.name || backend.id }))
    emit('refresh')
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.message || t('providerManager.deleteFailed'))
  }
}

function openCreate() {
  if (!canWrite.value) {
    ElMessage.warning(t('providerManager.noPermission'))
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
}

async function handleExportJSON() {
  busy.value = true
  try {
    const rows = await fetchExportRows()
    triggerDownload(
      new Blob([JSON.stringify(rows, null, 2)], { type: 'application/json' }),
      'centag-backends.json'
    )
    ElMessage.success(t('providerManager.exportJsonSuccess'))
  } catch {
    ElMessage.error(t('providerManager.exportFailed'))
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
      ElMessage.warning(t('providerManager.noExportData'))
      return
    }
    const content = ConfigBuilder.buildBackendsYaml(entries)
    triggerDownload(new Blob([content], { type: 'text/yaml;charset=utf-8' }), 'initial-backends.yaml')
    ElMessage.success(t('providerManager.exportYamlSuccess'))
  } catch {
    ElMessage.error(t('providerManager.exportYamlFailed'))
  } finally {
    busy.value = false
  }
}

function handleImport() {
  importInput.value?.click()
}

function parseImportPayload(text: string, fileName: string) {
  const trimmed = (text || '').trim()
  if (!trimmed) throw new Error(t('providerManager.fileFormatError') + ' empty')
  const lower = (fileName || '').toLowerCase()
  const looksYaml =
    lower.endsWith('.yaml') ||
    lower.endsWith('.yml') ||
    (!trimmed.startsWith('{') && !trimmed.startsWith('['))
  const data = looksYaml ? yaml.load(trimmed) : JSON.parse(trimmed)
  if (Array.isArray(data)) return data
  if (data && Array.isArray((data as any).backends)) return (data as any).backends
  if (data && typeof data === 'object') return [data]
  throw new Error(t('providerManager.fileFormatError') + ' unrecognized format')
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
        t('providerManager.importConfirm', { count: data.length }),
        t('providerManager.importTitle'),
        { confirmButtonText: t('providerManager.importConfirmBtn'), cancelButtonText: t('providerManager.importCancelBtn'), type: 'info' }
      )
        .then(() => {
          busy.value = true
          return api.post('/api/v1/backends/import', { backends: data })
        })
        .then(() => {
          ElMessage.success(t('providerManager.importSuccess'))
          emit('refresh')
        })
        .catch((err) => {
          if (err !== 'cancel' && err !== 'close') {
            ElMessage.error(err?.response?.data?.message || err?.message || t('providerManager.importFailed'))
          }
        })
        .finally(() => {
          busy.value = false
        })
    } catch (err: any) {
      ElMessage.error(t('providerManager.fileFormatError') + err.message)
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
  padding: 12px 16px;
  background: linear-gradient(135deg, #f8fafc 0%, #f1f5f9 100%);
  border-radius: 12px;
  border: 1px solid #e2e8f0;
}

.mgr-toolbar-left,
.mgr-toolbar-right {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.mgr-toolbar-left .el-button {
  border-radius: 8px;
  font-weight: 500;
  transition: all 0.2s ease;
}

.mgr-toolbar-left .el-button:hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
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

.backend-stats {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 8px;
  font-size: 0.75rem;
  color: #9ca3af;
}

.default-model-row {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-top: 10px;
}

.default-model-label {
  font-size: 0.75rem;
  color: #6b7280;
}

.account-pool-badge {
  display: inline-flex;
  align-items: center;
  padding: 1px 6px;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 500;
}

.pool-healthy {
  background: #d1fae5;
  color: #065f46;
}

.pool-partial {
  background: #fef3c7;
  color: #92400e;
}

.pool-unhealthy {
  background: #fee2e2;
  color: #991b1b;
}

.backend-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  padding-top: 8px;
  border-top: 1px solid rgba(15, 23, 42, 0.06);
}

.action-star {
  width: 32px;
  height: 32px;
  font-size: 14px;
  cursor: pointer;
  color: #9ca3af;
  transition: color 0.2s, background 0.2s;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  border: 1px solid var(--el-border-color-lighter);
  background: var(--el-bg-color);
}

.action-star:hover {
  color: #f59e0b;
  border-color: #f59e0b;
  background: rgba(245, 158, 11, 0.1);
}

.action-star.is-default {
  color: #f59e0b;
  border-color: #f59e0b;
}

.action-star[disabled] {
  cursor: not-allowed;
  opacity: 0.5;
}

.backend-actions :deep(.el-button.is-circle) {
  width: 32px;
  height: 32px;
  padding: 8px;
}

.empty-tip {
  grid-column: 1 / -1;
  text-align: center;
  color: #9ca3af;
  font-size: 0.8rem;
  padding: 24px 0;
}
</style>

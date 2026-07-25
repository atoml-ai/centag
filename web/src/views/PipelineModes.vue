<template>
  <div class="pipeline-modes-page">
    <div class="header-with-toolbar">
      <h1 class="page-title">
        <el-icon><SetUp /></el-icon>
        {{ t('pipelineModes.title') }}
      </h1>
      <div class="toolbar-actions">
        <el-input
          v-model="searchText"
          :placeholder="t('pipelineModes.searchPlaceholder')"
          clearable
          style="width: 200px"
        >
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>
        <span class="search-count" v-if="searchText">
          {{ t('pipelineModes.searchCount', { count: filteredPipelines.length }) }}
        </span>
        <el-button
          v-if="selectedPipelines.length > 0"
          :loading="batchExporting"
          @click="handleBatchExport"
        >
          <el-icon><Download /></el-icon>
          {{ t('pipelineModes.batchExport', { count: selectedPipelines.length }) }}
        </el-button>
        <el-tooltip v-if="selectedPipelines.length > 0" :content="batchDeleteTooltip" placement="top" :disabled="canBatchDeleteSelected">
          <span>
            <el-button
              type="danger"
              :disabled="!canBatchDeleteSelected"
              @click="handleBatchDelete"
            >
              <el-icon><Delete /></el-icon>
              {{ t('pipelineModes.batchDelete', { count: selectedPipelines.length }) }}
            </el-button>
          </span>
        </el-tooltip>
        <el-button :loading="loading" @click="loadData">
          <el-icon><Refresh /></el-icon>
          {{ t('pipelineModes.refresh') }}
        </el-button>
        <el-button @click="openTemplateDialog">
          <el-icon><DocumentCopy /></el-icon>
          {{ t('pipelineModes.createFromTemplate') }}
        </el-button>
        <el-button @click="triggerImportTemplate">
          <el-icon><Upload /></el-icon>
          {{ t('pipelineModes.importPipeline') }}
        </el-button>
        <input
          ref="importTemplateInputRef"
          type="file"
          accept=".yaml,.yml,text/yaml"
          multiple
          style="display: none"
          @change="handleImportTemplateFile"
        />
        <el-button v-if="canAddOwnPipelines" type="primary" @click="openCreate">
          <el-icon><Plus /></el-icon>
          {{ t('pipelineModes.createPipeline') }}
        </el-button>
      </div>
    </div>

    <!-- 流水线列表 -->
    <el-card class="table-card" v-loading="loading">
      <el-table
        :data="filteredPipelines"
        stripe
        size="large"
        highlight-current-row
        row-key="id"
        @selection-change="handleSelectionChange"
      >
        <el-table-column type="selection" width="50" />
        <el-table-column prop="id" :label="t('pipelineModes.table.id')" width="120" show-overflow-tooltip sortable>
          <template #default="{ row }">
            <el-tag type="info" size="small">{{ row.id }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="shortcut_code" :label="t('pipelineModes.table.shortcutCode')" width="150" align="center" sortable>
          <template #default="{ row }">
            <div style="display: flex; align-items: center; gap: 4px;">
              <el-input
                v-model="row.shortcut_code"
                :placeholder="t('pipelineModes.table.shortcutPlaceholder')"
                clearable
                size="small"
                :loading="row.shortcutLoading"
                style="flex: 1"
              >
                <template #prefix>
                  <el-icon><SetUp /></el-icon>
                </template>
              </el-input>
              <PipelineFeatureGuard
                v-if="(row.shortcut_code || '') !== (row._originalShortcutCode || '')"
                feature="pipelineShortcutUpdate"
                :pipeline="row"
                :is-admin="authStore.isAdmin"
                :action-label="t('pipelineModes.table.saveShortcut')"
              >
                <template #default="{ disabled }">
                  <el-button
                    size="small"
                    type="primary"
                    :loading="row.shortcutLoading"
                    :disabled="disabled"
                    @click="handleShortcutSave(row)"
                    :title="t('pipelineModes.table.save')"
                  >
                    <el-icon><Check /></el-icon>
                  </el-button>
                </template>
              </PipelineFeatureGuard>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="name" :label="t('pipelineModes.table.name')" min-width="140" sortable>
          <template #default="{ row }">
            <span style="font-weight: 500">{{ row.name }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="description" :label="t('pipelineModes.table.description')" min-width="200" show-overflow-tooltip sortable />
        <el-table-column :label="t('pipelineModes.table.nodeCount')" width="80" align="center" sortable :sort-method="sortByNodeCount">
          <template #default="{ row }">
            <el-tag type="primary" size="small">{{ row.nodes?.length || 0 }} {{ t('pipelineModes.table.nodeUnit') }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="version" :label="t('pipelineModes.table.version')" width="80" align="center" sortable />
        <el-table-column :label="t('pipelineModes.table.actions')" width="300" align="center" fixed="right">
          <template #default="{ row }">
            <div class="action-btns">
              <el-tooltip v-if="canConfigureCapabilitySlots(row)" :content="t('pipelineModes.table.configureModel')" placement="top">
                <el-button
                  circle
                  size="small"
                  type="warning"
                  @click="openRouteAssign(row)"
                >
                  <el-icon><Connection /></el-icon>
                </el-button>
              </el-tooltip>
              <PipelineFeatureGuard
                feature="pipelineEdit"
                :pipeline="row"
                :is-admin="authStore.isAdmin"
                :action-label="t('pipelineModes.table.edit')"
              >
                <template #default="{ disabled }">
                  <el-button
                    circle
                    size="small"
                    :disabled="disabled"
                    @click="openEdit(row)"
                  >
                    <el-icon><Edit /></el-icon>
                  </el-button>
                </template>
              </PipelineFeatureGuard>
              <PipelineFeatureGuard
                feature="executionHistory"
                :pipeline="row"
                :is-admin="authStore.isAdmin"
                :action-label="t('pipelineModes.table.history')"
              >
                <template #default="{ disabled }">
                  <el-button
                    circle
                    size="small"
                    type="info"
                    :disabled="disabled"
                    @click="openHistory(row)"
                  >
                    <el-icon><Timer /></el-icon>
                  </el-button>
                </template>
              </PipelineFeatureGuard>
              <PipelineFeatureGuard
                feature="pipelineExport"
                :pipeline="row"
                :is-admin="authStore.isAdmin"
                :action-label="t('pipelineModes.table.export')"
              >
                <template #default="{ disabled }">
                  <el-button
                    circle
                    size="small"
                    type="primary"
                    :disabled="disabled"
                    @click="handleExport(row)"
                  >
                    <el-icon><Download /></el-icon>
                  </el-button>
                </template>
              </PipelineFeatureGuard>
              <PipelineFeatureGuard
                feature="pipelineDelete"
                :pipeline="row"
                :is-admin="authStore.isAdmin"
                :action-label="t('pipelineModes.table.delete')"
              >
                <template #default="{ disabled }">
                  <el-button
                    circle
                    size="small"
                    type="danger"
                    :disabled="disabled"
                    @click="handleDelete(row)"
                  >
                    <el-icon><Delete /></el-icon>
                  </el-button>
                </template>
              </PipelineFeatureGuard>
            </div>
          </template>
        </el-table-column>
      </el-table>

      <el-empty v-if="!loading && pipelines.length === 0" :description="t('pipelineModes.empty')" :image-size="120" />
    </el-card>

    <PipelineCreateDialog
      v-model="createInfoVisible"
      :existing-ids="pipelines.map(p => p.id)"
      @confirm="startCreateFromInfo"
    />

    <PipelineEditorDialog
      v-model="canvasVisible"
      :pipeline="currentPipeline"
      :is-create="isCreating"
      @saved="handleEditorSaved"
      @update:pipeline="currentPipeline = $event"
      @closed="handleEditorClosed"
    />

    <CapabilitySlotsDialog
      v-model="routeAssignVisible"
      :pipeline-id="routeAssignPipelineId"
      @saved="handleRouteAssignSaved"
    />

    <!-- 执行历史对话框 -->
    <ExecutionHistory
      v-model="historyVisible"
      :pipeline-id="historyPipelineId"
      :pipeline-name="historyPipelineName"
    />

    <!-- 从模板创建弹窗 -->
    <el-dialog v-model="templateDialogVisible" :title="t('pipelineModes.templateDialog.title')" width="600px">
      <el-alert type="info" :closable="false" style="margin-bottom: 16px">
        {{ t('pipelineModes.templateDialog.hint') }}
      </el-alert>
      <el-row :gutter="12">
        <el-col :span="12" v-for="tmpl in templateList" :key="tmpl.id" style="margin-bottom: 12px">
          <el-card shadow="hover" :body-style="{ padding: '16px', cursor: 'pointer' }" @click="createFromTemplate(tmpl)">
            <div style="font-weight: 600; margin-bottom: 6px">{{ tmpl.name }}</div>
            <div style="font-size: 13px; color: #666">{{ tmpl.description }}</div>
            <div style="margin-top: 8px">
              <el-tag size="small" type="primary">{{ t('pipelineModes.templateDialog.nodes', { count: tmpl.nodes?.length || 0 }) }}</el-tag>
            </div>
          </el-card>
        </el-col>
      </el-row>
    </el-dialog>

    <!-- 导入冲突对话框 -->
    <el-dialog v-model="importConflictVisible" :title="t('pipelineModes.importConflict.title')" width="580px" :close-on-click-modal="false" :close-on-press-escape="false" :before-close="handleConflictCancel">
      <el-alert type="warning" :closable="false" show-icon style="margin-bottom: 14px">
        {{ t('pipelineModes.importConflict.warning') }}
      </el-alert>
      <el-table :data="importConflictItems" size="small" border max-height="320" stripe>
        <el-table-column type="index" label="#" width="46" align="center" />
        <el-table-column prop="id" :label="t('pipelineModes.importConflict.id')" width="120">
          <template #default="{ row }">
            <el-tag type="warning" effect="plain" size="small">{{ row.id }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="name" :label="t('pipelineModes.importConflict.name')" min-width="130">
          <template #default="{ row }">
            <span style="font-weight: 500">{{ row.name }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="t('pipelineModes.importConflict.nodeCount')" width="72" align="center">
          <template #default="{ row }">
            <el-tag type="primary" size="small">{{ row.nodes?.length || 0 }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="description" :label="t('pipelineModes.importConflict.description')" min-width="140" show-overflow-tooltip />
      </el-table>
      <div style="color: #909399; font-size: 13px; margin-top: 10px;">
        <el-icon style="vertical-align: middle; margin-right: 4px;"><WarningFilled /></el-icon>
        {{ t('pipelineModes.importConflict.conflictCount', { count: importConflictItems.length }) }}
      </div>
      <template #footer>
        <div style="display: flex; justify-content: center; gap: 12px;">
          <el-button @click="handleConflictCancel">{{ t('pipelineModes.importConflict.cancelImport') }}</el-button>
          <el-button @click="handleConflictSkip">
            <el-icon style="margin-right: 4px;"><CircleClose /></el-icon>
            {{ t('pipelineModes.importConflict.skipDuplicates', { count: importConflictItems.length }) }}
          </el-button>
          <el-button type="primary" @click="handleConflictOverwrite">
            <el-icon style="margin-right: 4px;"><Select /></el-icon>
            {{ t('pipelineModes.importConflict.overwriteAll') }}
          </el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Search, SetUp, Refresh, Plus, Edit, Delete, DocumentCopy, Upload, Timer, Check, Download, WarningFilled, CircleClose, Select, Connection } from '@element-plus/icons-vue'
import * as yaml from 'js-yaml'
import {
  getPipelines,
  createPipeline,
  updatePipeline,
  deletePipeline,
  getPipelineTemplates,
  parsePipelinesResponse,
  type Pipeline,
  type AgentPatternPipeline
} from '@/api/pipeline'
import PipelineEditorDialog from '@/components/pipeline/PipelineEditorDialog.vue'
import PipelineCreateDialog from '@/components/pipeline/PipelineCreateDialog.vue'
import PipelineFeatureGuard from '@/components/pipeline/PipelineFeatureGuard.vue'
import CapabilitySlotsDialog from '@/components/pipeline/CapabilitySlotsDialog.vue'
import type { PipelineCreateInfo } from '@/components/pipeline/PipelineCreateDialog.vue'
import ExecutionHistory from '@/components/pipeline/ExecutionHistory.vue'
import { useAuthStore } from '@/stores/auth'
import { useUserResourceAccess } from '@/composables/useUserResourceAccess'
import { resolvePipelineFeatureSupport } from '@/utils/pipeline/features'
import { canConfigureCapabilitySlots } from '@/utils/capabilitySlots'
import { downloadPipelineYaml, downloadPipelinesAsZip } from '@/utils/pipeline/importExport'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const { canAddOwnPipelines } = useUserResourceAccess()

const loading = ref(false)
const canvasVisible = ref(false)
const createInfoVisible = ref(false)
const isCreating = ref(false)
const templateDialogVisible = ref(false)
const pipelines = ref<Pipeline[]>([])
const templates = ref<Record<string, any>>({})
const currentPipeline = ref<any>(null)
const searchText = ref('')
const selectedPipelines = ref<Pipeline[]>([])
const importTemplateInputRef = ref<HTMLInputElement | null>(null)
const batchExporting = ref(false)
const routeAssignVisible = ref(false)
const routeAssignPipelineId = ref('')

const importConflictVisible = ref(false)
const importConflictItems = ref<any[]>([])
const importConflictResolve = ref<((value: 'overwrite' | 'skip' | 'cancel') => void) | null>(null)

const historyVisible = ref(false)
const historyPipelineId = ref('')
const historyPipelineName = ref('')

type PipelineRow = Pipeline & { shortcutLoading?: boolean; _originalShortcutCode?: string }

const filteredPipelines = computed(() => {
  if (!searchText.value.trim()) return pipelines.value
  const q = searchText.value.trim().toLowerCase()
  return pipelines.value.filter(p =>
    p.id?.toLowerCase().includes(q) ||
    p.name?.toLowerCase().includes(q) ||
    p.description?.toLowerCase().includes(q)
  )
})

function upsertPipelineInList(pipeline: Pipeline) {
  if (!pipeline?.id) return
  const row: PipelineRow = {
    ...pipeline,
    shortcutLoading: false,
    _originalShortcutCode: pipeline.shortcut_code || ''
  }
  const idx = pipelines.value.findIndex(p => p.id === pipeline.id)
  if (idx >= 0) {
    const next = [...pipelines.value]
    next[idx] = row
    pipelines.value = next
  } else {
    pipelines.value = [...pipelines.value, row]
  }
}

const handleSelectionChange = (rows: Pipeline[]) => {
  selectedPipelines.value = rows
}

const getPipelineFeatureSupport = (feature: 'pipelineBatchDelete', row: Pipeline) => {
  return resolvePipelineFeatureSupport(feature, row, { isAdmin: authStore.isAdmin })
}

const canBatchDeleteSelected = computed(() => {
  if (selectedPipelines.value.length === 0) return false
  return selectedPipelines.value.every((row) =>
    getPipelineFeatureSupport('pipelineBatchDelete', row).enabled
  )
})

const batchDeleteTooltip = computed(() => {
  if (canBatchDeleteSelected.value) return t('pipelineModes.batchDeleteTooltip')
  const unsupported = selectedPipelines.value.find((row) => !getPipelineFeatureSupport('pipelineBatchDelete', row).enabled)
  if (!unsupported) return t('pipelineModes.batchDeleteTooltip')
  return getPipelineFeatureSupport('pipelineBatchDelete', unsupported).reason || t('pipelineModes.batchDeleteTooltipDisabled')
})

const handleBatchDelete = async () => {
  if (selectedPipelines.value.length === 0) return
  try {
    await ElMessageBox.confirm(
      t('pipelineModes.message.batchDeleteConfirm', { count: selectedPipelines.value.length }),
      t('pipelineModes.message.batchDeleteTitle'),
      { type: 'warning' }
    )
    const ids = selectedPipelines.value.map(p => p.id)
    for (const id of ids) {
      await deletePipeline(id)
    }
    pipelines.value = pipelines.value.filter(p => !ids.includes(p.id))
    ElMessage.success(t('pipelineModes.message.batchDeleteSuccess', { count: ids.length }))
    selectedPipelines.value = []
  } catch (error: any) {
    if (error === 'cancel' || error === 'close') return
    ElMessage.error(t('pipelineModes.message.deleteFailed') + '：' + (error.message || error))
    await loadData()
  }
}

const showImportConflictDialog = (duplicates: any[]): Promise<'overwrite' | 'skip' | 'cancel'> => {
  return new Promise(resolve => {
    importConflictItems.value = duplicates
    importConflictResolve.value = resolve
    importConflictVisible.value = true
  })
}

const handleConflictOverwrite = () => {
  importConflictVisible.value = false
  importConflictResolve.value?.('overwrite')
}

const handleConflictSkip = () => {
  importConflictVisible.value = false
  importConflictResolve.value?.('skip')
}

const handleConflictCancel = () => {
  importConflictVisible.value = false
  importConflictResolve.value?.('cancel')
}

const triggerImportTemplate = () => {
  importTemplateInputRef.value?.click()
}

const handleImportTemplateFile = async (e: Event) => {
  const input = e.target as HTMLInputElement
  const files = input.files
  if (!files || files.length === 0) return

  const parsedTemplates: any[] = []
  for (const file of files) {
    try {
      const text = await file.text()
      const data = yaml.load(text)
      if (!data || typeof data !== 'object' || !data.id || !data.name) continue
      parsedTemplates.push(data)
    } catch {
      // 跳过解析失败的文件
    }
  }

  if (parsedTemplates.length === 0) {
    ElMessage.error(t('pipelineModes.message.importFailed'))
    input.value = ''
    return
  }

  let overwriteStrategy = 'all'
  let existingIds: Set<string> | undefined
  try {
    const res = await getPipelines()
    existingIds = new Set(parsePipelinesResponse(res).map((p) => p.id))
    const duplicates = parsedTemplates.filter(t => existingIds!.has(t.id))

    if (duplicates.length > 0) {
      const action = await showImportConflictDialog(duplicates)
      if (action === 'cancel') {
        input.value = ''
        return
      }
      overwriteStrategy = action
    }
  } catch {
    // 查询失败则静默继续，默认全部覆盖
  }

  let successCount = 0
  let failCount = 0
  for (const data of parsedTemplates) {
    const isDuplicate = overwriteStrategy === 'skip' && existingIds?.has(data.id)
    if (isDuplicate) {
      continue
    }
    try {
      const payload: Pipeline = {
        id: data.id,
        name: data.name,
        description: data.description || '',
        version: data.version || '1.0',
        shortcut_code: data.shortcut_code || '',
        nodes: data.nodes || [],
        global_config: data.global_config || { timeout: 120, max_retries: 3, bypass_on_error: true, stream_mode: false, parallel_limit: 4, fallback_groups: [], storage: undefined, hooks: undefined },
        metadata: data.metadata || {}
      }
      await createPipeline(payload, overwriteStrategy === 'overwrite')
      upsertPipelineInList(payload)
      successCount++
    } catch {
      failCount++
    }
  }

  input.value = ''
  if (successCount > 0) {
    ElMessage.success(t('pipelineModes.message.importSuccess', { count: successCount }) + (failCount > 0 ? `，${t('pipelineModes.message.importPartialFailed', { count: failCount })}` : ''))
  } else {
    ElMessage.error(t('pipelineModes.message.importFailed'))
  }
}

const templateList = computed(() => {
  return Object.values(templates.value)
    .map((t: any) => ({
      id: t.id || '',
      name: t.name || '',
      description: t.description || '',
      nodes: t.nodes || [],
      global_config: t.global_config || {},
      metadata: t.metadata || {}
    }))
    .sort((a, b) => a.id.localeCompare(b.id))
})

const loadData = async () => {
  loading.value = true
  try {
    const pipelinesRes = await getPipelines()
    const list = parsePipelinesResponse(pipelinesRes)
    list.forEach((p: any) => {
      p.shortcutLoading = false
      p._originalShortcutCode = (p.shortcut_code || '')
    })
    pipelines.value = list
  } catch (error: any) {
    ElMessage.error(t('pipelineModes.message.loadFailed') + '：' + error.message)
    pipelines.value = []
  } finally {
    loading.value = false
  }
}

const loadAllData = async () => {
  await Promise.all([loadData(), loadTemplates()])
}

const handleShortcutSave = async (row: any) => {
  if (row.shortcutLoading) return

  const code = (row.shortcut_code || '').trim()
  const original = row._originalShortcutCode || ''

  if (code === original) {
    return
  }

  if (code && !code.startsWith('#')) {
    ElMessage.warning(t('pipelineModes.message.shortcutFormatError'))
    row.shortcut_code = original
    return
  }

  row.shortcutLoading = true
  try {
    const payload = JSON.parse(JSON.stringify({
      ...row,
      shortcut_code: code
    }))
    delete payload.shortcutLoading
    delete payload._originalShortcutCode

    await updatePipeline(row.id, payload)
    row._originalShortcutCode = code
    ElMessage.success(code ? t('pipelineModes.message.shortcutSet') : t('pipelineModes.message.shortcutCleared'))
  } catch (error: any) {
    const backendError = error.response?.data?.error || error.message || t('storage.healthStatus.unknownError')
    console.error(`[ShortcutChange] 更新流水线 ${row.id} 失败:`, error.response?.data || error)
    ElMessage.error(t('pipelineModes.message.shortcutSaveFailed') + '：' + backendError)
    row.shortcut_code = original
  } finally {
    row.shortcutLoading = false
  }
}

const loadTemplates = async () => {
  try {
    const res = await getPipelineTemplates()
    templates.value = res || {}
  } catch (error) {
    console.warn('加载模板失败', error)
  }
}

const buildEmptyPipeline = (info: Partial<PipelineCreateInfo> = {}) => ({
  id: info.id || `pipeline-${Date.now()}`,
  name: info.name || '',
  description: info.description || '',
  version: info.version || '1.0',
  shortcut_code: info.shortcut_code || '',
  nodes: [] as Pipeline['nodes'],
  global_config: {
    timeout: 120,
    max_retries: 3,
    bypass_on_error: true,
    stream_mode: false,
    parallel_limit: 4,
    log_level: 'info',
    fallback_groups: [],
    storage: undefined,
    hooks: undefined
  },
  metadata: {}
})

const openCreate = () => {
  createInfoVisible.value = true
}

const startCreateFromInfo = async (info: PipelineCreateInfo) => {
  isCreating.value = true
  canvasVisible.value = false
  currentPipeline.value = null
  await nextTick()
  currentPipeline.value = buildEmptyPipeline(info)
  await nextTick()
  canvasVisible.value = true
}

const openTemplateDialog = () => {
  templateDialogVisible.value = true
}

const createFromTemplate = (tmpl: any) => {
  templateDialogVisible.value = false
  isCreating.value = true
  const templateGlobalConfig = tmpl.global_config || {}
  const templateMetadata = tmpl.metadata && typeof tmpl.metadata === 'object' ? tmpl.metadata : {}
  currentPipeline.value = {
    id: `${tmpl.id}-${Date.now()}`,
    name: tmpl.name,
    description: tmpl.description,
    version: '1.0',
    nodes: JSON.parse(JSON.stringify(tmpl.nodes || [])),
    global_config: {
      timeout: templateGlobalConfig.timeout ?? 120,
      max_retries: templateGlobalConfig.max_retries ?? 3,
      bypass_on_error: templateGlobalConfig.bypass_on_error ?? true,
      stream_mode: templateGlobalConfig.stream_mode ?? false,
      parallel_limit: templateGlobalConfig.parallel_limit ?? 4,
      log_level: templateGlobalConfig.log_level ?? 'info'
    },
    metadata: { from_template: tmpl.id, ...templateMetadata }
  }
  canvasVisible.value = true
  ElMessage.success(t('pipelineModes.message.createFromTemplateSuccess', { name: tmpl.name }))
}

const openEdit = (row: Pipeline) => {
  isCreating.value = false
  currentPipeline.value = JSON.parse(JSON.stringify(row))
  canvasVisible.value = true
}

function openRouteAssign(row: Pipeline) {
  routeAssignPipelineId.value = row.id
  routeAssignVisible.value = true
}

function handleRouteAssignSaved(saved: AgentPatternPipeline) {
  const idx = pipelines.value.findIndex((p) => p.id === saved.id)
  if (idx >= 0) {
    const next = [...pipelines.value]
    next[idx] = { ...pipelines.value[idx], ...saved, nodes: saved.nodes }
    pipelines.value = next
  }
}

const openHistory = (row: Pipeline) => {
  historyPipelineId.value = row.id
  historyPipelineName.value = row.name
  historyVisible.value = true
}

const sortByNodeCount = (a: Pipeline, b: Pipeline): number => {
  return (a.nodes?.length || 0) - (b.nodes?.length || 0)
}

const handleExport = async (row: Pipeline) => {
  try {
    await downloadPipelineYaml(row.id, row.name || row.id)
    ElMessage.success(t('pipelineModes.message.exportSuccess'))
  } catch (error: any) {
    ElMessage.error(t('pipelineModes.message.exportFailed') + '：' + (error.message || error))
  }
}

const handleBatchExport = async () => {
  if (!selectedPipelines.value.length) return
  batchExporting.value = true
  try {
    await downloadPipelinesAsZip(
      selectedPipelines.value.map((p) => ({ id: p.id, name: p.name || p.id }))
    )
    ElMessage.success(t('pipelineModes.message.batchExportSuccess', { count: selectedPipelines.value.length }))
  } catch (error: any) {
    ElMessage.error(t('pipelineModes.message.batchExportFailed') + '：' + (error.message || error))
  } finally {
    batchExporting.value = false
  }
}

const openEditById = (id: string) => {
  const pipeline = pipelines.value.find(p => p.id === id)
  if (pipeline) {
    openEdit(pipeline)
  } else {
    ElMessage.error(t('pipelineModes.message.pipelineNotFound', { id }))
  }
}

watch(() => route.params.id, (id) => {
  if (id && canvasVisible.value === false) {
    if (pipelines.value.length > 0) {
      openEditById(id as string)
    } else {
      loadData().then(() => {
        openEditById(id as string)
      })
    }
  }
}, { immediate: true })

watch(() => route.path, (path) => {
  if (
    path === '/pipelines/create' &&
    !canvasVisible.value &&
    !createInfoVisible.value
  ) {
    openCreate()
  }
}, { immediate: true })

const handleEditorSaved = async (savedPipeline?: Pipeline) => {
  isCreating.value = false
  if (savedPipeline?.id) {
    upsertPipelineInList(savedPipeline)
  } else if (currentPipeline.value?.id) {
    upsertPipelineInList(currentPipeline.value)
  }
  if (route.path === '/pipelines/create') {
    await router.replace('/pipelines')
  }
}

const handleEditorClosed = () => {
  isCreating.value = false
  if (route.path === '/pipelines/create') {
    router.replace('/pipelines')
  }
}

const handleDelete = async (row: Pipeline) => {
  try {
    await ElMessageBox.confirm(t('pipelineModes.message.deleteConfirm', { name: row.name }), t('pipelineModes.message.deleteTitle'), {
      type: 'warning',
      confirmButtonText: t('pipelineModes.message.deleteBtn'),
      cancelButtonText: t('pipelineModes.message.cancelBtn')
    })
    const deletedId = row.id
    await deletePipeline(deletedId)
    pipelines.value = pipelines.value.filter(p => p.id !== deletedId)
    selectedPipelines.value = selectedPipelines.value.filter(p => p.id !== deletedId)
    ElMessage.success(t('pipelineModes.message.deleteSuccess'))
  } catch (error: any) {
    if (error === 'cancel' || error === 'close') return
    ElMessage.error(t('pipelineModes.message.deleteFailed') + '：' + (error.message || error))
    await loadData()
  }
}

onMounted(() => {
  loadAllData()
})
</script>

<style scoped>
.pipeline-modes-page {
  width: 100%;
  padding: 0 0 24px;
}

.header-with-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
  flex-wrap: wrap;
  gap: 12px;
}

.page-title {
  display: flex;
  align-items: center;
  gap: 10px;
  margin: 0;
  font-size: 26px;
  font-weight: 600;
  color: #1f2937;
  flex-shrink: 0;
}

.toolbar-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.table-card {
  border-radius: 12px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}

.toolbar-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.search-count {
  font-size: 13px;
  color: #64748b;
  white-space: nowrap;
}

.action-btns {
  display: flex;
  justify-content: center;
  align-items: center;
  gap: 6px;
}

</style>

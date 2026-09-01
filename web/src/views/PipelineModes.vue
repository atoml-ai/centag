<template>
  <div class="pipeline-modes-page">
    <div class="page-header">
      <h1 class="page-title">{{ t('pipelineModes.title') }}</h1>
      <p class="page-subtitle">{{ t('pipelineModes.subtitle') }}</p>
    </div>
    <div class="header-with-toolbar">
      <div class="toolbar-actions">
        <el-button v-if="canAddOwnPipelines" type="primary" @click="openCreate">
          <el-icon><Plus /></el-icon>
          {{ t('pipelineModes.createPipeline') }}
        </el-button>
        <el-button @click="openTemplateDialog">
          <el-icon><DocumentCopy /></el-icon>
          {{ t('pipelineModes.createFromTemplate') }}
        </el-button>
        <el-button @click="triggerImportTemplate">
          <el-icon><Upload /></el-icon>
          {{ t('pipelineModes.importPipeline') }}
        </el-button>
        <el-button :loading="loading" @click="loadData">
          <el-icon><Refresh /></el-icon>
          {{ t('pipelineModes.refresh') }}
        </el-button>
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
      </div>
      <div class="toolbar-right">
        <el-checkbox
          v-if="filteredPipelines.length > 0"
          v-model="allSelected"
          :indeterminate="partialSelected"
          @change="toggleSelectAll"
        >
          {{ t('pipelineModes.selectAll') }}
        </el-checkbox>
      </div>
    </div>

    <!-- 流水线卡片视图 -->
    <div class="pipeline-cards-wrap" v-loading="loading">
      <el-empty v-if="!loading && filteredPipelines.length === 0" :description="t('pipelineModes.empty')" :image-size="120" />
      <div class="pipeline-cards">
        <div
          v-for="row in filteredPipelines"
          :key="row.id"
          class="pipeline-card"
          :class="{ 'is-default': row.id === defaultPipelineId }"
        >
          <div class="pipeline-card__head">
            <el-checkbox
              :model-value="isCardSelected(row)"
              @change="(v) => toggleCardSelection(row, v)"
            />
            <div class="pipeline-card__title">
              <span class="pipeline-card__name">{{ row.name }}</span>
              <el-tag v-if="isSystemPipeline(row)" type="info" size="small" effect="plain">
                {{ t('pipelineModes.table.scopeSystem') }}
              </el-tag>
              <el-tag v-else type="warning" size="small" effect="plain">
                {{ t('pipelineModes.table.scopeMine') }}
              </el-tag>
              <el-tag v-if="row.id === defaultPipelineId" type="success" size="small" effect="light">
                <el-icon style="margin-right: 2px; vertical-align: -2px;"><StarFilled /></el-icon>
                {{ t('pipelineModes.table.defaultPipeline') }}
              </el-tag>
            </div>
            <div class="pipeline-card__icons">
              <el-tooltip :content="row.id === defaultPipelineId ? t('pipelineModes.table.currentDefault') : t('pipelineModes.table.setDefault')" placement="top">
                <el-icon
                  class="pipeline-card__star"
                  :class="{ 'is-default': row.id === defaultPipelineId }"
                  @click="handleSetDefault(row)"
                >
                  <StarFilled v-if="row.id === defaultPipelineId" />
                  <Star v-else />
                </el-icon>
              </el-tooltip>
              <el-tooltip :content="t('pipelineModes.table.test')" placement="top">
                <el-icon class="pipeline-card__test" @click="openPipelineTest(row)">
                  <ChatDotRound />
                </el-icon>
              </el-tooltip>
            </div>
            <PipelineRowActions
              class="pipeline-card__actions"
              :row="row"
              :unrestricted="unrestricted"
              :default-pipeline-id="defaultPipelineId"
              @command="(cmd) => handleRowCommand(cmd, row)"
            />
          </div>
          <div class="pipeline-card__body">
            <div class="pipeline-card__row">
              <el-tag type="info" size="small">{{ row.id }}</el-tag>
            </div>
            <p class="pipeline-card__desc">{{ row.description || '—' }}</p>
            <div class="pipeline-card__shortcut">
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
                :unrestricted="unrestricted"
                :action-label="t('pipelineModes.table.saveShortcut')"
              >
                <template #default="{ disabled }">
                  <el-button
                    size="small"
                    type="primary"
                    :loading="row.shortcutLoading"
                    :disabled="disabled"
                    @click="handleShortcutSave(row)"
                  >
                    <el-icon><Check /></el-icon>
                  </el-button>
                </template>
              </PipelineFeatureGuard>
            </div>
          </div>
          <div class="pipeline-card__footer">
            <span class="pipeline-card__stat">
              <el-icon><Connection /></el-icon>
              {{ row.nodes?.length || 0 }} {{ t('pipelineModes.table.nodeUnit') }}
            </span>
            <span class="pipeline-card__stat">v{{ row.version }}</span>
          </div>
        </div>
      </div>
    </div>

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

    <!-- 与 Personal/Minimal 共用 MinimalChat 抽屉，避免 Team 单次问答对话框不一致 -->
    <MinimalChat
      v-model="testChatVisible"
      :initial-pipeline-id="testChatPipelineId"
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
    <input
      ref="importTemplateInputRef"
      type="file"
      accept=".yaml,.yml"
      multiple
      style="display: none"
      @change="handleImportTemplateFile"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Search, SetUp, Refresh, Plus, Delete, DocumentCopy, Upload, Check, Download, WarningFilled, CircleClose, Select, Connection, Star, StarFilled, ChatDotRound } from '@element-plus/icons-vue'
import * as yaml from 'js-yaml'
import {
  getPipelines,
  createPipeline,
  updatePipeline,
  deletePipeline,
  clonePipeline,
  getPipelineTemplates,
  getPipelineDefaults,
  updatePipelineDefaults,
  parsePipelinesResponse,
  type Pipeline,
  type AgentPatternPipeline
} from '@/api/pipeline'
import PipelineEditorDialog from '@/components/pipeline/PipelineEditorDialog.vue'
import PipelineCreateDialog from '@/components/pipeline/PipelineCreateDialog.vue'
import PipelineFeatureGuard from '@/components/pipeline/PipelineFeatureGuard.vue'
import PipelineRowActions from '@/components/pipeline/PipelineRowActions.vue'
import CapabilitySlotsDialog from '@/components/pipeline/CapabilitySlotsDialog.vue'
import type { PipelineCreateInfo } from '@/components/pipeline/PipelineCreateDialog.vue'
import ExecutionHistory from '@/components/pipeline/ExecutionHistory.vue'
import MinimalChat from '@/views/MinimalChat.vue'
import { useUserResourceAccess } from '@/composables/useUserResourceAccess'
import { resolvePipelineFeatureSupport, isSystemPipeline } from '@/utils/pipeline/features'
import { downloadPipelineYaml, downloadPipelinesAsZip } from '@/utils/pipeline/importExport'
const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const { canAddOwnPipelines, unrestricted } = useUserResourceAccess()

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

const defaultPipelineId = ref('')
const settingDefaultId = ref('')
const cloningId = ref('')
const testChatVisible = ref(false)
const testChatPipelineId = ref('')

type PipelineRow = Pipeline & { shortcutLoading?: boolean; _originalShortcutCode?: string }

function isCardSelected(row: Pipeline): boolean {
  return selectedPipelines.value.some((p) => p.id === row.id)
}

function toggleCardSelection(row: Pipeline, checked: boolean | string | number) {
  if (checked) {
    if (!isCardSelected(row)) selectedPipelines.value = [...selectedPipelines.value, row]
  } else {
    selectedPipelines.value = selectedPipelines.value.filter((p) => p.id !== row.id)
  }
}

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

const getPipelineFeatureSupport = (feature: 'pipelineBatchDelete', row: Pipeline) => {
  return resolvePipelineFeatureSupport(feature, row, { unrestricted })
}

const canBatchDeleteSelected = computed(() => {
  if (selectedPipelines.value.length === 0) return false
  return selectedPipelines.value.every((row) =>
    getPipelineFeatureSupport('pipelineBatchDelete', row).enabled
  )
})

const allSelected = computed({
  get: () => filteredPipelines.value.length > 0 && selectedPipelines.value.length === filteredPipelines.value.length,
  set: () => {}
})

const partialSelected = computed(() =>
  selectedPipelines.value.length > 0 && selectedPipelines.value.length < filteredPipelines.value.length
)

function toggleSelectAll(checked: boolean | string | number) {
  if (checked) {
    selectedPipelines.value = [...filteredPipelines.value]
  } else {
    selectedPipelines.value = []
  }
}

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
    await refreshDefaultPipeline()
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

async function refreshDefaultPipeline() {
  const d = await getPipelineDefaults().catch(() => null)
  const data = (d as { data?: { default_pipeline_id?: string } } | null)?.data ?? d
  defaultPipelineId.value = (data as { default_pipeline_id?: string } | null)?.default_pipeline_id || ''
}

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
    await refreshDefaultPipeline()
  } catch (error: any) {
    ElMessage.error(t('pipelineModes.message.loadFailed') + '：' + error.message)
    pipelines.value = []
  } finally {
    loading.value = false
  }
}

const handleSetDefault = async (row: Pipeline) => {
  if (settingDefaultId.value || row.id === defaultPipelineId.value) return
  settingDefaultId.value = row.id
  try {
    await updatePipelineDefaults({ default_pipeline_id: row.id })
    defaultPipelineId.value = row.id
    ElMessage.success(t('pipelineModes.message.setDefaultSuccess', { name: row.name || row.id }))
  } catch (error: any) {
    ElMessage.error(t('pipelineModes.message.setDefaultFailed') + '：' + (error.response?.data?.error || error.message || error))
  } finally {
    settingDefaultId.value = ''
  }
}

const openPipelineTest = (row: Pipeline) => {
  testChatPipelineId.value = row.id || ''
  testChatVisible.value = true
}

watch(testChatVisible, (open, wasOpen) => {
  if (wasOpen && !open) {
    testChatPipelineId.value = ''
  }
})

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

const handleClone = async (row: Pipeline) => {
  if (cloningId.value) return
  cloningId.value = row.id
  try {
    const res: any = await clonePipeline(row.id)
    const data = res?.data?.data ?? res?.data ?? res
    ElMessage.success(t('pipelineModes.message.cloneSuccess', { name: data?.name || data?.id || row.name }))
    await loadData()
  } catch (error: any) {
    ElMessage.error(t('pipelineModes.message.cloneFailed') + '：' + (error.response?.data?.error || error.message || error))
  } finally {
    cloningId.value = ''
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
    if (defaultPipelineId.value === deletedId) {
      await refreshDefaultPipeline()
    }
    ElMessage.success(t('pipelineModes.message.deleteSuccess'))
  } catch (error: any) {
    if (error === 'cancel' || error === 'close') return
    ElMessage.error(t('pipelineModes.message.deleteFailed') + '：' + (error.message || error))
    await loadData()
  }
}

const handleRowCommand = (command: string, row: Pipeline) => {
  switch (command) {
    case 'configure':
      openRouteAssign(row)
      break
    case 'edit':
      openEdit(row)
      break
    case 'history':
      openHistory(row)
      break
    case 'export':
      handleExport(row)
      break
    case 'clone':
      handleClone(row)
      break
    case 'delete':
      handleDelete(row)
      break
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

.page-header {
  margin-bottom: 16px;
}

.page-title {
  margin: 0 0 4px;
  font-size: 22px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.page-subtitle {
  margin: 0;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.header-with-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
  gap: 10px;
  padding: 12px 16px;
  background: linear-gradient(135deg, #f8fafc 0%, #f1f5f9 100%);
  border-radius: 12px;
  border: 1px solid #e2e8f0;
}

.toolbar-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.toolbar-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

.toolbar-actions .el-button {
  border-radius: 8px;
  font-weight: 500;
  transition: all 0.2s ease;
}

.toolbar-actions .el-button:hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}

.pipeline-cards-wrap {
  min-height: 200px;
}

.pipeline-cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 16px;
}

.pipeline-card {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 16px;
  border: 1px solid #e4e7ed;
  border-radius: 12px;
  background: #fff;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
  transition: box-shadow 0.2s ease, border-color 0.2s ease;
}

.pipeline-card:hover {
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.08);
}

.pipeline-card.is-default {
  border-color: #67c23a;
  background: linear-gradient(180deg, #f0fdf4 0%, #ffffff 60%);
}

.pipeline-card__head {
  display: flex;
  align-items: flex-start;
  gap: 10px;
}

.pipeline-card__title {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
  flex: 1;
  min-width: 0;
}

.pipeline-card__name {
  font-size: 0.9375rem;
  font-weight: 600;
  color: #1f2937;
  min-width: 0;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pipeline-card__icons {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.pipeline-card__star,
.pipeline-card__test {
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

.pipeline-card__star:hover {
  color: #f59e0b;
  border-color: #f59e0b;
  background: rgba(245, 158, 11, 0.1);
}

.pipeline-card__star.is-default {
  color: #f59e0b;
  border-color: #f59e0b;
}

.pipeline-card__test:hover {
  color: #3b82f6;
  border-color: #3b82f6;
  background: rgba(59, 130, 246, 0.1);
}

.pipeline-card__actions {
  flex-shrink: 0;
}

.pipeline-card__body {
  display: flex;
  flex-direction: column;
  gap: 8px;
  flex: 1;
}

.pipeline-card__row {
  display: flex;
  align-items: center;
  gap: 6px;
}

.pipeline-card__desc {
  margin: 0;
  font-size: 0.8125rem;
  color: #6b7280;
  line-height: 1.5;
  min-height: 2.4em;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.pipeline-card__shortcut {
  display: flex;
  align-items: center;
  gap: 8px;
}

.pipeline-card__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding-top: 12px;
  border-top: 1px solid #f1f3f5;
}

.pipeline-card__stat {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 0.8125rem;
  color: #6b7280;
}

.search-count {
  font-size: 13px;
  color: #64748b;
  white-space: nowrap;
}

</style>

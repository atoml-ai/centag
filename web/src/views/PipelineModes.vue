<template>
  <div class="pipeline-modes-page">
    <div class="header-with-toolbar">
      <h1 class="page-title">
        <el-icon><SetUp /></el-icon>
        策略管理
      </h1>
      <div class="toolbar-actions">
        <el-input
          v-model="searchText"
          placeholder="搜索名称、ID、描述..."
          clearable
          style="width: 200px"
        >
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>
        <span class="search-count" v-if="searchText">
          {{ filteredPipelines.length }} 条
        </span>
        <el-tooltip v-if="selectedPipelines.length > 0" :content="batchDeleteTooltip" placement="top" :disabled="canBatchDeleteSelected">
          <span>
            <el-button
              type="danger"
              :disabled="!canBatchDeleteSelected"
              @click="handleBatchDelete"
            >
              <el-icon><Delete /></el-icon>
              批量删除（{{ selectedPipelines.length }}）
            </el-button>
          </span>
        </el-tooltip>
        <el-button :loading="loading" @click="loadData">
          <el-icon><Refresh /></el-icon>
          刷新
        </el-button>
        <el-button @click="openTemplateDialog">
          <el-icon><DocumentCopy /></el-icon>
          从模板创建
        </el-button>
        <el-button @click="triggerImportTemplate">
          <el-icon><Upload /></el-icon>
          导入模板
        </el-button>
        <input
          ref="importTemplateInputRef"
          type="file"
          accept=".yaml,.yml,text/yaml"
          multiple
          style="display: none"
          @change="handleImportTemplateFile"
        />
        <el-button type="primary" @click="openCreate">
          <el-icon><Plus /></el-icon>
          创建流水线
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
        <el-table-column prop="id" label="ID" width="120" show-overflow-tooltip sortable>
          <template #default="{ row }">
            <el-tag type="info" size="small">{{ row.id }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="shortcut_code" label="快捷码" width="150" align="center" sortable>
          <template #default="{ row }">
            <div style="display: flex; align-items: center; gap: 4px;">
              <el-input
                v-model="row.shortcut_code"
                placeholder="#xxx"
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
                action-label="保存快捷码"
              >
                <template #default="{ disabled }">
                  <el-button
                    size="small"
                    type="primary"
                    :loading="row.shortcutLoading"
                    :disabled="disabled"
                    @click="handleShortcutSave(row)"
                    title="保存"
                  >
                    <el-icon><Check /></el-icon>
                  </el-button>
                </template>
              </PipelineFeatureGuard>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="name" label="名称" min-width="140" sortable>
          <template #default="{ row }">
            <span style="font-weight: 500">{{ row.name }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="description" label="描述" min-width="200" show-overflow-tooltip sortable />
        <el-table-column label="节点数" width="80" align="center" sortable :sort-method="sortByNodeCount">
          <template #default="{ row }">
            <el-tag type="primary" size="small">{{ row.nodes?.length || 0 }} 个</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="version" label="版本" width="80" align="center" sortable />
        <el-table-column label="操作" width="300" align="center" fixed="right">
          <template #default="{ row }">
            <div class="action-btns">
              <el-tooltip v-if="canConfigureCapabilitySlots(row)" content="配置模型" placement="top">
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
                action-label="编辑"
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
                action-label="历史"
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
                action-label="导出"
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
                action-label="删除"
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

      <el-empty v-if="!loading && pipelines.length === 0" description="暂无流水线配置，使用右侧按钮创建" :image-size="120" />
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
    <el-dialog v-model="templateDialogVisible" title="从模板创建流水线" width="600px">
      <el-alert type="info" :closable="false" style="margin-bottom: 16px">
        选择以下预设模板快速创建流水线。带多分类的模板可用「配置模型」绑定各分支后端/模型；也可在画布「新增分类」后配置。
      </el-alert>
      <el-row :gutter="12">
        <el-col :span="12" v-for="tmpl in templateList" :key="tmpl.id" style="margin-bottom: 12px">
          <el-card shadow="hover" :body-style="{ padding: '16px', cursor: 'pointer' }" @click="createFromTemplate(tmpl)">
            <div style="font-weight: 600; margin-bottom: 6px">{{ tmpl.name }}</div>
            <div style="font-size: 13px; color: #666">{{ tmpl.description }}</div>
            <div style="margin-top: 8px">
              <el-tag size="small" type="primary">{{ tmpl.nodes?.length || 0 }} 个节点</el-tag>
            </div>
          </el-card>
        </el-col>
      </el-row>
    </el-dialog>

    <!-- 导入冲突对话框 -->
    <el-dialog v-model="importConflictVisible" title="导入冲突" width="580px" :close-on-click-modal="false" :close-on-press-escape="false" :before-close="handleConflictCancel">
      <el-alert type="warning" :closable="false" show-icon style="margin-bottom: 14px">
        以下模板 ID 已存在于系统中，请选择导入方式：
      </el-alert>
      <el-table :data="importConflictItems" size="small" border max-height="320" stripe>
        <el-table-column type="index" label="#" width="46" align="center" />
        <el-table-column prop="id" label="ID" width="120">
          <template #default="{ row }">
            <el-tag type="warning" effect="plain" size="small">{{ row.id }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="name" label="名称" min-width="130">
          <template #default="{ row }">
            <span style="font-weight: 500">{{ row.name }}</span>
          </template>
        </el-table-column>
        <el-table-column label="节点数" width="72" align="center">
          <template #default="{ row }">
            <el-tag type="primary" size="small">{{ row.nodes?.length || 0 }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="description" label="描述" min-width="140" show-overflow-tooltip />
      </el-table>
      <div style="color: #909399; font-size: 13px; margin-top: 10px;">
        <el-icon style="vertical-align: middle; margin-right: 4px;"><WarningFilled /></el-icon>
        共 <strong>{{ importConflictItems.length }}</strong> 个冲突模板
      </div>
      <template #footer>
        <div style="display: flex; justify-content: center; gap: 12px;">
          <el-button @click="handleConflictCancel">取消导入</el-button>
          <el-button @click="handleConflictSkip">
            <el-icon style="margin-right: 4px;"><CircleClose /></el-icon>
            跳过重复（{{ importConflictItems.length }} 个）
          </el-button>
          <el-button type="primary" @click="handleConflictOverwrite">
            <el-icon style="margin-right: 4px;"><Select /></el-icon>
            全部覆盖
          </el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Search, SetUp, Refresh, Plus, Edit, Delete, DocumentCopy, Upload, Timer, Check, Download, WarningFilled, CircleClose, Select, Connection } from '@element-plus/icons-vue'
import * as yaml from 'js-yaml'
import {
  getPipelines,
  createPipeline,
  updatePipeline,
  deletePipeline,
  exportPipeline,
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
import { resolvePipelineFeatureSupport } from '@/utils/pipeline/features'
import { canConfigureCapabilitySlots } from '@/utils/capabilitySlots'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()

// ── 状态 ─────────────────────────────────────────────────────────────────────
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
const routeAssignVisible = ref(false)
const routeAssignPipelineId = ref('')

// 导入冲突对话框状态
const importConflictVisible = ref(false)
const importConflictItems = ref<any[]>([])
const importConflictResolve = ref<((value: 'overwrite' | 'skip' | 'cancel') => void) | null>(null)

// 执行历史对话框
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

/** 将保存/导入的流水线立即合并进列表，避免依赖可能滞后的全量刷新 */
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
  if (canBatchDeleteSelected.value) return '批量删除'
  const unsupported = selectedPipelines.value.find((row) => !getPipelineFeatureSupport('pipelineBatchDelete', row).enabled)
  if (!unsupported) return '批量删除'
  return getPipelineFeatureSupport('pipelineBatchDelete', unsupported).reason || '存在不可删除的流水线'
})

const handleBatchDelete = async () => {
  if (selectedPipelines.value.length === 0) return
  try {
    await ElMessageBox.confirm(
      `确定删除选中的 ${selectedPipelines.value.length} 个流水线吗？`,
      '批量删除',
      { type: 'warning' }
    )
    const ids = selectedPipelines.value.map(p => p.id)
    for (const id of ids) {
      await deletePipeline(id)
    }
    pipelines.value = pipelines.value.filter(p => !ids.includes(p.id))
    ElMessage.success(`成功删除 ${ids.length} 个流水线`)
    selectedPipelines.value = []
  } catch (error: any) {
    if (error === 'cancel' || error === 'close') return
    ElMessage.error('删除失败：' + (error.message || error))
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

  // 1. 解析所有文件为模板数据
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
    ElMessage.error('导入失败，请确认文件是有效的流水线模板 YAML 格式')
    input.value = ''
    return
  }

  // 2. 检查哪些 ID 已存在
  let overwriteStrategy = 'all' // 'all' | 'skip' | 'cancel'
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
      overwriteStrategy = action // 'overwrite' 或 'skip'
    }
  } catch {
    // 查询失败则静默继续，默认全部覆盖
  }

  // 3. 执行导入
  let successCount = 0
  let failCount = 0
  for (const data of parsedTemplates) {
    const isDuplicate = overwriteStrategy === 'skip' && existingIds?.has(data.id)
    if (isDuplicate) {
      continue // 跳过重复
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
    ElMessage.success(`成功导入 ${successCount} 个流水线${failCount > 0 ? `，${failCount} 个失败` : ''}`)
  } else {
    ElMessage.error('导入失败，请确认文件是有效的流水线模板 YAML 格式')
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

// ── 数据加载 ─────────────────────────────────────────────────────────────────
const loadData = async () => {
  loading.value = true
  try {
    const pipelinesRes = await getPipelines()
    const list = parsePipelinesResponse(pipelinesRes)
    // 保存原始 shortcut_code，用于变化检测和失败回滚
    list.forEach((p: any) => {
      p.shortcutLoading = false
      p._originalShortcutCode = (p.shortcut_code || '')
    })
    pipelines.value = list
  } catch (error: any) {
    ElMessage.error('加载流水线失败：' + error.message)
    pipelines.value = []
  } finally {
    loading.value = false
  }
}

const loadAllData = async () => {
  await Promise.all([loadData(), loadTemplates()])
}

// ── 快捷码处理 ──────────────────────────────────────────────────────────────
const handleShortcutSave = async (row: any) => {
  // 防止重复提交或误触发
  if (row.shortcutLoading) return

  const code = (row.shortcut_code || '').trim()
  const original = row._originalShortcutCode || ''

  // 值未变化则不发送请求（避免点击/失焦误触发）
  if (code === original) {
    return
  }

  // 格式校验：如果填写了内容，必须以 # 开头
  if (code && !code.startsWith('#')) {
    ElMessage.warning('快捷码必须以 # 开头')
    row.shortcut_code = original
    return
  }

  row.shortcutLoading = true
  try {
    // 深拷贝 payload，避免 Vue Proxy 对象导致后端解析异常
    const payload = JSON.parse(JSON.stringify({
      ...row,
      shortcut_code: code
    }))
    // 去除前端注入的辅助字段
    delete payload.shortcutLoading
    delete payload._originalShortcutCode

    await updatePipeline(row.id, payload)
    row._originalShortcutCode = code
    ElMessage.success(code ? '快捷码已设置' : '快捷码已清除')
  } catch (error: any) {
    const backendError = error.response?.data?.error || error.message || '未知错误'
    console.error(`[ShortcutChange] 更新流水线 ${row.id} 失败:`, error.response?.data || error)
    ElMessage.error('操作失败：' + backendError)
    // 仅恢复当前行的快捷码，避免 loadData() 导致表格重渲染连锁反应
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

// ── 操作方法 ─────────────────────────────────────────────────────────────────
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
  ElMessage.success(`已基于「${tmpl.name}」模板创建流水线，请配置各节点的后端和模型`)
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

const handleExport = (row: Pipeline) => {
  const filename = `${row.name || 'pipeline'}-${row.id}.yaml`
  exportPipeline(row.id).then((response: any) => {
    const content = typeof response === 'string' ? response : response?.data || ''
    const blob = new Blob([content], { type: 'text/yaml' })
    const url = window.URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = filename
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    window.URL.revokeObjectURL(url)
    ElMessage.success('导出成功')
  }).catch((error: any) => {
    ElMessage.error('导出失败：' + (error.message || error))
  })
}

// 支持通过路由参数打开特定流水线
const openEditById = (id: string) => {
  const pipeline = pipelines.value.find(p => p.id === id)
  if (pipeline) {
    openEdit(pipeline)
  } else {
    ElMessage.error(`流水线 ${id} 不存在`)
  }
}

// 监听路由参数变化
watch(() => route.params.id, (id) => {
  if (id && canvasVisible.value === false) {
    // 等待数据加载完成后打开编辑
    if (pipelines.value.length > 0) {
      openEditById(id)
    } else {
      // 数据未加载，先加载再打开
      loadData().then(() => {
        openEditById(id)
      })
    }
  }
}, { immediate: true })

// 监听路由路径变化（创建模式）：先弹出基础信息表单，而非直接打开画布
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
    await ElMessageBox.confirm(`确定删除流水线 "${row.name}" 吗？`, '确认删除', {
      type: 'warning',
      confirmButtonText: '删除',
      cancelButtonText: '取消'
    })
    const deletedId = row.id
    await deletePipeline(deletedId)
    pipelines.value = pipelines.value.filter(p => p.id !== deletedId)
    selectedPipelines.value = selectedPipelines.value.filter(p => p.id !== deletedId)
    ElMessage.success('删除成功')
  } catch (error: any) {
    if (error === 'cancel' || error === 'close') return
    ElMessage.error('删除失败：' + (error.message || error))
    await loadData()
  }
}

// ── 初始化 ───────────────────────────────────────────────────────────────────
onMounted(() => {
  loadAllData()
})
</script>

<style scoped>
.pipeline-modes-page {
  padding: 20px;
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

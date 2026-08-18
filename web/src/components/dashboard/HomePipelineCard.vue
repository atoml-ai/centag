<template>
  <div class="home-pipeline-card" v-loading="loading">
    <input
      ref="importInputRef"
      type="file"
      accept=".yaml,.yml,text/yaml,application/x-yaml"
      multiple
      class="hidden-file-input"
      @change="handleImportFiles"
    />
    <template v-if="pipelines.length > 0">
      <div class="pipeline-list-head">
        <span class="section-label">{{ t('homePipelineCard.pipelineList') }}</span>
        <span class="list-count">{{ t('homePipelineCard.pipelineCount', { count: pipelines.length }) }}</span>
      </div>

      <div v-if="selectable" class="list-toolbar">
        <div class="list-toolbar-left">
          <template v-if="props.showCreateButton">
            <el-button v-if="canCreatePipeline" type="primary" size="small" @click="openCreate">
              <el-icon><Plus /></el-icon>
              {{ t('homePipelineCard.createPipeline') }}
            </el-button>
            <el-button size="small" :loading="importing" @click="triggerImport">
              <el-icon><Upload /></el-icon>
              {{ t('homePipelineCard.importPipeline') }}
            </el-button>
          </template>
        </div>
        <div class="list-toolbar-right">
          <el-checkbox
            :model-value="allSelected"
            :indeterminate="partialSelected"
            @change="toggleSelectAll"
          >
            {{ t('homePipelineCard.select') }}
          </el-checkbox>
          <template v-if="selectedIds.length > 0">
            <span class="toolbar-count">{{ t('homePipelineCard.selectedCount', { count: selectedIds.length }) }}</span>
            <el-button size="small" :loading="batchExporting" @click="handleBatchExport">
              {{ t('homePipelineCard.batchExport') }}
            </el-button>
            <el-button size="small" type="danger" :loading="batchDeleting" @click="handleBatchDelete">
              {{ t('homePipelineCard.batchDelete') }}
            </el-button>
            <el-button size="small" text @click="clearSelection">{{ t('homePipelineCard.cancelSelect') }}</el-button>
          </template>
        </div>
      </div>

      <div class="pipeline-cards">
        <div
          v-for="pipeline in pipelines"
          :key="pipeline.id"
          class="pipeline-card"
          :class="{
            'is-default': pipeline.id === selectedDefaultId,
            'is-selected': selectedIds.includes(pipeline.id)
          }"
        >
          <div class="pipeline-card__head">
            <el-checkbox
              v-if="selectable"
              :model-value="selectedIds.includes(pipeline.id)"
              @change="(checked) => toggleSelect(pipeline.id, !!checked)"
              @click.stop
            />
            <div class="pipeline-card__title">
              <span class="pipeline-card__name">{{ pipeline.name }}</span>
              <el-tag v-if="isSystemPipeline(pipeline)" type="info" size="small" effect="plain">
                {{ t('pipelineModes.table.scopeSystem') }}
              </el-tag>
              <el-tag v-else type="warning" size="small" effect="plain">
                {{ t('pipelineModes.table.scopeMine') }}
              </el-tag>
              <el-tag
                v-if="pipeline.id === selectedDefaultId"
                type="success"
                size="small"
                effect="light"
              >
                <el-icon style="margin-right: 2px; vertical-align: -2px;"><StarFilled /></el-icon>
                {{ t('pipelineModes.table.defaultPipeline') }}
              </el-tag>
            </div>
            <div class="pipeline-card__icons">
              <el-tooltip :content="pipeline.id === selectedDefaultId ? t('pipelineModes.table.currentDefault') : t('pipelineModes.table.setDefault')" placement="top">
                <el-icon
                  class="pipeline-card__star"
                  :class="{ 'is-default': pipeline.id === selectedDefaultId }"
                  @click="handleSetDefault(pipeline)"
                >
                  <StarFilled v-if="pipeline.id === selectedDefaultId" />
                  <Star v-else />
                </el-icon>
              </el-tooltip>
              <el-tooltip :content="t('pipelineModes.table.test')" placement="top">
                <el-icon class="pipeline-card__test" @click="handleTest(pipeline)">
                  <ChatDotRound />
                </el-icon>
              </el-tooltip>
            </div>
            <PipelineRowActions
              class="pipeline-card__actions"
              :row="pipeline"
              :unrestricted="unrestricted"
              :default-pipeline-id="selectedDefaultId"
              @command="(cmd) => handleRowCommand(cmd, pipeline)"
            />
          </div>
          <div class="pipeline-card__body">
            <div class="pipeline-card__row">
              <el-tag type="info" size="small">{{ pipeline.id }}</el-tag>
            </div>
            <p class="pipeline-card__desc">{{ pipeline.description || '—' }}</p>
          </div>
          <div class="pipeline-card__footer">
            <span class="pipeline-card__stat">
              <el-icon><Connection /></el-icon>
              {{ pipeline.nodes?.length || 0 }} {{ t('pipelineModes.table.nodeUnit') }}
            </span>
            <span class="pipeline-card__stat">v{{ pipeline.version }}</span>
          </div>
        </div>
      </div>
    </template>

    <el-empty v-else-if="!loading" :description="t('homePipelineCard.emptyState')" :image-size="56">
      <div class="empty-actions">
        <el-button v-if="canCreatePipeline" type="primary" plain size="small" @click="openCreate">
          {{ t('homePipelineCard.createPipeline') }}
        </el-button>
        <el-button
          v-if="canCreatePipeline"
          plain
          size="small"
          :loading="importing"
          @click="triggerImport"
        >
          {{ t('homePipelineCard.importPipeline') }}
        </el-button>
      </div>
    </el-empty>

    <PipelineCreateDialog
      v-model="createInfoVisible"
      :existing-ids="pipelines.map(p => p.id)"
      @confirm="startCreateFromInfo"
    />

    <PipelineEditorDialog
      v-model="editorVisible"
      :pipeline="editingPipeline"
      :is-create="isCreating"
      @saved="handleEditorSaved"
    />

    <CapabilitySlotsDialog
      v-model="routeAssignVisible"
      :pipeline-id="routeAssignPipelineId"
      @saved="handleRouteAssignSaved"
    />

    <ExecutionHistory
      v-model="historyVisible"
      :pipeline-id="historyPipelineId"
      :pipeline-name="historyPipelineName"
    />

    <MinimalChat
      v-model="testChatVisible"
      :initial-pipeline-id="testChatPipelineId"
    />

    <el-dialog
      v-model="importConflictVisible"
      :title="t('homePipelineCard.importConflictTitle')"
      width="580px"
      :close-on-click-modal="false"
      :close-on-press-escape="false"
      :before-close="handleConflictCancel"
    >
      <p class="conflict-hint">{{ t('homePipelineCard.conflictHint') }}</p>
      <el-table :data="importConflictItems" size="small" border max-height="320" stripe>
        <el-table-column prop="id" label="ID" min-width="140" show-overflow-tooltip />
        <el-table-column prop="name" :label="t('homePipelineCard.table.name', 'Name')" min-width="160" show-overflow-tooltip />
      </el-table>
      <p class="conflict-summary">
        {{ t('homePipelineCard.conflictCount', { count: importConflictItems.length }) }}
      </p>
      <template #footer>
        <el-button @click="handleConflictCancel">{{ t('homePipelineCard.cancel') }}</el-button>
        <el-button @click="handleConflictSkip">
          {{ t('homePipelineCard.skipDuplicates', { count: importConflictItems.length }) }}
        </el-button>
        <el-button type="primary" @click="handleConflictOverwrite">{{ t('homePipelineCard.overwriteImport') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Connection, Plus, Star, StarFilled, ChatDotRound, Upload } from '@element-plus/icons-vue'
import PipelineCreateDialog from '@/components/pipeline/PipelineCreateDialog.vue'
import type { PipelineCreateInfo } from '@/components/pipeline/PipelineCreateDialog.vue'
import PipelineEditorDialog from '@/components/pipeline/PipelineEditorDialog.vue'
import PipelineRowActions from '@/components/pipeline/PipelineRowActions.vue'
import CapabilitySlotsDialog from '@/components/pipeline/CapabilitySlotsDialog.vue'
import ExecutionHistory from '@/components/pipeline/ExecutionHistory.vue'
import MinimalChat from '@/views/MinimalChat.vue'
import { useUserResourceAccess } from '@/composables/useUserResourceAccess'
import { isSystemPipeline } from '@/utils/pipeline/features'
import {
  getPipelines,
  getPipeline,
  getPipelineDefaults,
  updatePipelineDefaults,
  deletePipeline,
  clonePipeline,
  parsePipelinesResponse,
  type AgentPatternPipeline
} from '@/api/pipeline'
import {
  downloadPipelineYaml,
  downloadPipelinesAsZip,
  parsePipelineYamlFiles,
  importPipelineTemplates,
  type ParsedPipelineTemplate
} from '@/utils/pipeline/importExport'

const props = withDefaults(defineProps<{
  showCreateButton?: boolean
}>(), {
  showCreateButton: false
})

const emit = defineEmits<{
  'update:count': [count: number]
}>()

const { t } = useI18n()

const pipelines = ref<AgentPatternPipeline[]>([])
const selectedDefaultId = ref('')
const pendingDefaultId = ref('')
const loading = ref(false)
const savingDefault = ref(false)
const editorVisible = ref(false)
const editingPipeline = ref<AgentPatternPipeline | null>(null)
const isCreating = ref(false)
const createInfoVisible = ref(false)
const selectedIds = ref<string[]>([])
const batchDeleting = ref(false)
const batchExporting = ref(false)
const exportingId = ref('')
const cloningId = ref('')
const importing = ref(false)
const importInputRef = ref<HTMLInputElement | null>(null)
const importConflictVisible = ref(false)
const importConflictItems = ref<ParsedPipelineTemplate[]>([])
const importConflictResolve = ref<((value: 'overwrite' | 'skip' | 'cancel') => void) | null>(null)
const routeAssignVisible = ref(false)
const routeAssignPipelineId = ref('')
const historyVisible = ref(false)
const historyPipelineId = ref('')
const historyPipelineName = ref('')
const testChatVisible = ref(false)
const testChatPipelineId = ref('')

const { canAddOwnPipelines, canChangeDefaultPipeline, unrestricted } = useUserResourceAccess()
const canEdit = computed(() => canAddOwnPipelines.value)
const canCreatePipeline = computed(() => canEdit.value)
const canSetDefault = computed(() => canChangeDefaultPipeline.value)
const selectable = computed(() => canEdit.value)

const allSelected = computed(() =>
  pipelines.value.length > 0 && selectedIds.value.length === pipelines.value.length
)
const partialSelected = computed(() =>
  selectedIds.value.length > 0 && selectedIds.value.length < pipelines.value.length
)

watch(() => pipelines.value.length, (len) => {
  emit('update:count', len)
}, { immediate: true })

watch(pipelines, (list) => {
  const valid = new Set(list.map(p => p.id))
  selectedIds.value = selectedIds.value.filter(id => valid.has(id))
})

function toggleSelect(id: string, checked: boolean) {
  if (checked) {
    if (!selectedIds.value.includes(id)) selectedIds.value = [...selectedIds.value, id]
  } else {
    selectedIds.value = selectedIds.value.filter(x => x !== id)
  }
}

function toggleSelectAll(checked: boolean | string | number) {
  selectedIds.value = checked ? pipelines.value.map(p => p.id) : []
}

function clearSelection() {
  selectedIds.value = []
}

async function loadPipelines() {
  loading.value = true
  try {
    const [listRes, defaultsRes] = await Promise.all([getPipelines(), getPipelineDefaults()])
    pipelines.value = parsePipelinesResponse(listRes)
    const defaults = (defaultsRes as { data?: { default_pipeline_id?: string } })?.data ?? defaultsRes
    selectedDefaultId.value = (defaults as { default_pipeline_id?: string })?.default_pipeline_id || ''

    if (
      selectedDefaultId.value &&
      !pipelines.value.some(p => p.id === selectedDefaultId.value)
    ) {
      console.warn(
        `[HomePipelineCard] default pipeline "${selectedDefaultId.value}" is missing from the registry`
      )
    }
  } catch (error: any) {
    ElMessage.error(t('homePipelineCard.loadFailed', { msg: error.message || t('common.unknownError') }))
    pipelines.value = []
  } finally {
    loading.value = false
  }
}

async function persistDefault(pipelineId: string) {
  if (!canSetDefault.value || !pipelineId) return
  savingDefault.value = true
  pendingDefaultId.value = pipelineId
  try {
    await updatePipelineDefaults({ default_pipeline_id: pipelineId })
    selectedDefaultId.value = pipelineId
    const found = pipelines.value.find(p => p.id === pipelineId)
    ElMessage.success(
      found ? t('homePipelineCard.setDefaultSuccess', { name: found.name }) : t('homePipelineCard.setDefaultSuccessFallback')
    )
  } catch (error: any) {
    ElMessage.error(t('homePipelineCard.setDefaultFailed', { msg: error.message || t('common.unknownError') }))
    await loadPipelines()
  } finally {
    savingDefault.value = false
    pendingDefaultId.value = ''
  }
}

function selectDefault(pipelineId: string) {
  if (pipelineId === selectedDefaultId.value) return
  selectedDefaultId.value = pipelineId
  persistDefault(pipelineId)
}

function handleSetDefault(pipeline: AgentPatternPipeline) {
  selectDefault(pipeline.id)
}

function handleTest(pipeline: AgentPatternPipeline) {
  testChatPipelineId.value = pipeline.id || ''
  testChatVisible.value = true
}

async function reassignDefaultIfNeeded(deletedIds: string[]) {
  const deletedDefault = deletedIds.includes(selectedDefaultId.value)
  if (!deletedDefault) return
  await loadPipelines()
  if (!canSetDefault.value || pipelines.value.length === 0) return
  const next = pipelines.value[0]
  if (next?.id) {
    await persistDefault(next.id)
  }
}

async function handleDelete(pipeline: AgentPatternPipeline) {
  const wasDefault = pipeline.id === selectedDefaultId.value
  try {
    await ElMessageBox.confirm(
      wasDefault
        ? t('homePipelineCard.confirmDeleteDefault', { name: pipeline.name || pipeline.id })
        : t('homePipelineCard.confirmDelete', { name: pipeline.name || pipeline.id }),
      t('homePipelineCard.confirmDeleteTitle'),
      { confirmButtonText: t('homePipelineCard.deleteAction'), cancelButtonText: t('homePipelineCard.cancel'), type: 'warning' }
    )
  } catch {
    return
  }
  try {
    await deletePipeline(pipeline.id)
    ElMessage.success(t('homePipelineCard.deleteSuccess', { name: pipeline.name || pipeline.id }))
    if (wasDefault) {
      await reassignDefaultIfNeeded([pipeline.id])
    } else {
      await loadPipelines()
    }
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.message || t('homePipelineCard.deleteFailed'))
  }
}

async function handleBatchDelete() {
  if (!selectedIds.value.length) return
  const ids = [...selectedIds.value]
  const includesDefault = ids.includes(selectedDefaultId.value)
  try {
    await ElMessageBox.confirm(
      includesDefault
        ? t('homePipelineCard.batchDeleteConfirmDefault', { count: ids.length })
        : t('homePipelineCard.batchDeleteConfirm', { count: ids.length }),
      t('homePipelineCard.batchDeleteTitle'),
      { confirmButtonText: t('homePipelineCard.deleteAction'), cancelButtonText: t('homePipelineCard.cancel'), type: 'warning' }
    )
  } catch {
    return
  }
  batchDeleting.value = true
  try {
    for (const id of ids) {
      await deletePipeline(id)
    }
    ElMessage.success(t('homePipelineCard.batchDeleteSuccess', { count: ids.length }))
    selectedIds.value = []
    if (includesDefault) {
      await reassignDefaultIfNeeded(ids)
    } else {
      await loadPipelines()
    }
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.message || t('homePipelineCard.batchDeleteFailed'))
    await loadPipelines()
  } finally {
    batchDeleting.value = false
  }
}

async function handleExportOne(pipeline: AgentPatternPipeline) {
  exportingId.value = pipeline.id
  try {
    await downloadPipelineYaml(pipeline.id, pipeline.name || pipeline.id)
    ElMessage.success(t('homePipelineCard.exportSuccess', { name: pipeline.name || pipeline.id }))
  } catch (error: any) {
    ElMessage.error(t('homePipelineCard.exportFailed', { msg: error?.message || error }))
  } finally {
    exportingId.value = ''
  }
}

async function handleClone(pipeline: AgentPatternPipeline) {
  if (!pipeline?.id) return
  cloningId.value = pipeline.id
  try {
    const res: any = await clonePipeline(pipeline.id)
    const data = res?.data?.data || res?.data || res
    ElMessage.success(t('homePipelineCard.cloneSuccess', { name: data?.name || data?.id || pipeline.name }))
    await loadPipelines()
  } catch (error: any) {
    ElMessage.error(
      t('homePipelineCard.cloneFailed', {
        msg: error?.response?.data?.error || error?.message || error
      })
    )
  } finally {
    cloningId.value = ''
  }
}

async function handleBatchExport() {
  if (!selectedIds.value.length) return
  const items = selectedIds.value
    .map((id) => {
      const p = pipelines.value.find((x) => x.id === id)
      return p ? { id: p.id, name: p.name || p.id } : null
    })
    .filter((x): x is { id: string; name: string } => !!x)
  if (!items.length) return
  batchExporting.value = true
  try {
    await downloadPipelinesAsZip(items)
    ElMessage.success(t('homePipelineCard.batchExportSuccess', { count: items.length }))
  } catch (error: any) {
    ElMessage.error(t('homePipelineCard.batchExportFailed', { msg: error?.message || error }))
  } finally {
    batchExporting.value = false
  }
}

function triggerImport() {
  importInputRef.value?.click()
}

function showImportConflictDialog(
  duplicates: ParsedPipelineTemplate[]
): Promise<'overwrite' | 'skip' | 'cancel'> {
  return new Promise((resolve) => {
    importConflictItems.value = duplicates
    importConflictResolve.value = resolve
    importConflictVisible.value = true
  })
}

function handleConflictOverwrite() {
  importConflictVisible.value = false
  importConflictResolve.value?.('overwrite')
  importConflictResolve.value = null
}

function handleConflictSkip() {
  importConflictVisible.value = false
  importConflictResolve.value?.('skip')
  importConflictResolve.value = null
}

function handleConflictCancel() {
  importConflictVisible.value = false
  importConflictResolve.value?.('cancel')
  importConflictResolve.value = null
}

async function handleImportFiles(e: Event) {
  const input = e.target as HTMLInputElement
  const files = input.files
  if (!files?.length) return

  importing.value = true
  try {
    const { templates, failedFiles } = await parsePipelineYamlFiles(files)
    if (!templates.length) {
      ElMessage.error(t('homePipelineCard.importFailedInvalidYaml'))
      return
    }

    const existingIds = new Set(pipelines.value.map((p) => p.id))
    const duplicates = templates.filter((t) => existingIds.has(t.id))
    let onDuplicate: 'overwrite' | 'skip' = 'overwrite'
    if (duplicates.length > 0) {
      const action = await showImportConflictDialog(duplicates)
      if (action === 'cancel') return
      onDuplicate = action
    }

    const { successCount, failCount, skippedCount } = await importPipelineTemplates(templates, {
      existingIds,
      onDuplicate
    })

    if (successCount > 0) {
      await loadPipelines()
      const parts = [t('homePipelineCard.importSuccess', { count: successCount })]
      if (skippedCount > 0) parts.push(t('homePipelineCard.importSkipped', { count: skippedCount }))
      if (failCount > 0) parts.push(t('homePipelineCard.importFailedCount', { count: failCount }))
      if (failedFiles.length > 0) parts.push(t('homePipelineCard.importFilesFailed', { count: failedFiles.length }))
      ElMessage.success(parts.join('，'))
    } else if (skippedCount > 0 && failCount === 0) {
      ElMessage.warning(t('homePipelineCard.importAllSkipped', { count: skippedCount }))
    } else {
      ElMessage.error(t('homePipelineCard.importFailedYaml'))
    }
  } catch (error: any) {
    ElMessage.error(t('homePipelineCard.importFailedError', { msg: error?.message || error }))
  } finally {
    importing.value = false
    input.value = ''
  }
}

async function openEditor(pipeline: AgentPatternPipeline) {
  isCreating.value = false
  try {
    const detail = await getPipeline(pipeline.id)
    editingPipeline.value = (detail as { data?: AgentPatternPipeline })?.data ?? detail as AgentPatternPipeline
    editorVisible.value = true
  } catch {
    editingPipeline.value = JSON.parse(JSON.stringify(pipeline))
    editorVisible.value = true
    ElMessage.warning(t('homePipelineCard.editorFallbackWarning'))
  }
}

function openRouteAssign(pipeline: AgentPatternPipeline) {
  routeAssignPipelineId.value = pipeline.id
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

function handleRowCommand(command: string, pipeline: AgentPatternPipeline) {
  switch (command) {
    case 'configure':
      openRouteAssign(pipeline)
      break
    case 'edit':
      openEditor(pipeline)
      break
    case 'history':
      historyPipelineId.value = pipeline.id
      historyPipelineName.value = pipeline.name || pipeline.id
      historyVisible.value = true
      break
    case 'export':
      handleExportOne(pipeline)
      break
    case 'clone':
      handleClone(pipeline)
      break
    case 'delete':
      handleDelete(pipeline)
      break
  }
}

function openCreate() {
  createInfoVisible.value = true
}

function buildEmptyPipeline(info: Partial<PipelineCreateInfo> = {}): AgentPatternPipeline {
  return {
    id: info.id || `pipeline-${Date.now()}`,
    name: info.name || '',
    description: info.description || '',
    version: info.version || '1.0',
    shortcut_code: info.shortcut_code || '',
    nodes: [],
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
  }
}

async function startCreateFromInfo(info: PipelineCreateInfo) {
  isCreating.value = true
  editorVisible.value = false
  editingPipeline.value = null
  await nextTick()
  editingPipeline.value = buildEmptyPipeline(info)
  await nextTick()
  editorVisible.value = true
}

function handleEditorSaved(savedPipeline?: AgentPatternPipeline) {
  if (!savedPipeline?.id) {
    loadPipelines()
    return
  }
  const idx = pipelines.value.findIndex(p => p.id === savedPipeline.id)
  if (idx >= 0) {
    const next = [...pipelines.value]
    next[idx] = savedPipeline
    pipelines.value = next
  } else {
    pipelines.value = [...pipelines.value, savedPipeline]
  }
}

onMounted(() => {
  loadPipelines()
})

defineExpose({ reload: loadPipelines, openCreate, openImport: triggerImport })
</script>

<style scoped>
.home-pipeline-card {
  display: flex;
  flex-direction: column;
  gap: 4px;
  width: 100%;
  height: 100%;
  min-height: 0;
}

.pipeline-list-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.section-label {
  font-size: 0.8125rem;
  font-weight: 600;
  color: #374151;
}

.empty-actions {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  flex-wrap: wrap;
}

.list-count {
  font-size: 0.75rem;
  color: #9ca3af;
}

.hidden-file-input {
  display: none;
}

.conflict-hint {
  margin: 0 0 10px;
  font-size: 0.875rem;
  color: #4b5563;
}

.conflict-summary {
  margin: 10px 0 0;
  font-size: 0.8125rem;
  color: #6b7280;
}

.list-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  flex-wrap: wrap;
  padding: 12px 16px;
  background: linear-gradient(135deg, #f8fafc 0%, #f1f5f9 100%);
  border-radius: 12px;
  border: 1px solid #e2e8f0;
  margin-bottom: 8px;
}

.list-toolbar-left,
.list-toolbar-right {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.list-toolbar .el-button {
  border-radius: 8px;
  font-weight: 500;
  transition: all 0.2s ease;
}

.list-toolbar .el-button:hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}

.toolbar-count {
  font-size: 0.75rem;
  color: #6b7280;
}

.pipeline-cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 16px;
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding-right: 4px;
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

.pipeline-card.is-selected {
  border-color: #93c5fd;
  background: #eff6ff;
}

.pipeline-card.is-default.is-selected {
  background: linear-gradient(180deg, #ecfdf5 0%, #f0f9ff 60%);
  border-color: #67c23a;
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

.pipeline-card__body {
  display: flex;
  flex-direction: column;
  gap: 8px;
  flex: 1;
}

.pipeline-card__row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
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

.pipeline-card__actions {
  flex-shrink: 0;
}
</style>

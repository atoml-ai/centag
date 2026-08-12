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

      <div class="pipeline-list">
        <div
          v-for="pipeline in pipelines"
          :key="pipeline.id"
          class="pipeline-row"
          :class="{
            'is-default': pipeline.id === selectedDefaultId,
            selected: selectedIds.includes(pipeline.id)
          }"
        >
          <el-checkbox
            v-if="selectable"
            :model-value="selectedIds.includes(pipeline.id)"
            class="row-checkbox"
            @change="(checked) => toggleSelect(pipeline.id, !!checked)"
            @click.stop
          />
          <div class="pipeline-main">
            <div class="pipeline-title-row">
              <span class="pipeline-name">{{ pipeline.name }}</span>
              <el-tag size="small" :type="pipeline.tenant_id ? 'warning' : 'info'" effect="plain">
                {{ pipeline.tenant_id ? t('pipelineModes.table.scopeMine') : t('pipelineModes.table.scopeSystem') }}
              </el-tag>
              <el-tag
                v-if="pipeline.id === selectedDefaultId"
                size="small"
                type="success"
                effect="light"
              >
                {{ t('homePipelineCard.defaultTag') }}
              </el-tag>
            </div>
            <div class="pipeline-meta">
              <span class="mono">{{ pipeline.id }}</span>
              <span class="meta-sep">·</span>
              <span>{{ t('homePipelineCard.nodeCount', { count: pipeline.nodes?.length || 0 }) }}</span>
            </div>
          </div>
          <div class="pipeline-actions">
            <el-button
              v-if="canTest"
              size="small"
              @click="handleTest(pipeline)"
            >
              {{ t('homePipelineCard.test') }}
            </el-button>
            <el-button
              v-if="canSetDefault"
              size="small"
              :type="pipeline.id === selectedDefaultId ? 'success' : 'default'"
              :plain="pipeline.id !== selectedDefaultId"
              :disabled="pipeline.id === selectedDefaultId || (savingDefault && pendingDefaultId === pipeline.id)"
              :loading="savingDefault && pendingDefaultId === pipeline.id"
              @click="selectDefault(pipeline.id)"
            >
              {{ pipeline.id === selectedDefaultId ? t('homePipelineCard.currentDefault') : t('homePipelineCard.setDefault') }}
            </el-button>
            <el-button
              v-if="canEdit && canConfigureCapabilitySlots(pipeline)"
              size="small"
              type="warning"
              plain
              @click="openRouteAssign(pipeline)"
            >
              {{ t('homePipelineCard.configureModel') }}
            </el-button>
            <PipelineFeatureGuard
              feature="pipelineExport"
              :pipeline="pipeline"
              :unrestricted="unrestricted"
              :action-label="t('homePipelineCard.exportAction')"
            >
              <template #default="{ disabled }">
                <el-button
                  size="small"
                  plain
                  :disabled="disabled || exportingId === pipeline.id"
                  :loading="exportingId === pipeline.id"
                  @click="handleExportOne(pipeline)"
                >
                  {{ t('homePipelineCard.exportAction') }}
                </el-button>
              </template>
            </PipelineFeatureGuard>
            <el-tooltip
              v-if="canCreatePipeline && !unrestricted"
              :content="isSystemPipeline(pipeline) ? t('homePipelineCard.cloneFromSystem') : t('homePipelineCard.cloneAction')"
              placement="top"
            >
              <el-button
                size="small"
                :type="isSystemPipeline(pipeline) ? 'warning' : 'default'"
                :plain="!isSystemPipeline(pipeline)"
                :loading="cloningId === pipeline.id"
                @click="handleClone(pipeline)"
              >
                <el-icon><CopyDocument /></el-icon>
                {{ isSystemPipeline(pipeline) ? t('homePipelineCard.cloneFromSystemShort') : t('homePipelineCard.cloneAction') }}
              </el-button>
            </el-tooltip>
            <PipelineFeatureGuard
              feature="pipelineEdit"
              :pipeline="pipeline"
              :unrestricted="unrestricted"
              :action-label="t('homePipelineCard.editAction')"
            >
              <template #default="{ disabled }">
                <el-button
                  size="small"
                  type="primary"
                  plain
                  :disabled="disabled"
                  @click="openEditor(pipeline)"
                >
                  <el-icon><Edit /></el-icon>
                  {{ t('homePipelineCard.editAction') }}
                </el-button>
              </template>
            </PipelineFeatureGuard>
            <el-button
              v-if="canEdit"
              size="small"
              type="danger"
              plain
              @click="handleDelete(pipeline)"
            >
              {{ t('homePipelineCard.deleteAction') }}
            </el-button>
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
import { CopyDocument, Edit } from '@element-plus/icons-vue'
import PipelineCreateDialog from '@/components/pipeline/PipelineCreateDialog.vue'
import type { PipelineCreateInfo } from '@/components/pipeline/PipelineCreateDialog.vue'
import PipelineEditorDialog from '@/components/pipeline/PipelineEditorDialog.vue'
import PipelineFeatureGuard from '@/components/pipeline/PipelineFeatureGuard.vue'
import CapabilitySlotsDialog from '@/components/pipeline/CapabilitySlotsDialog.vue'
import { useUserResourceAccess } from '@/composables/useUserResourceAccess'
import { canConfigureCapabilitySlots } from '@/utils/capabilitySlots'
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

const emit = defineEmits<{
  'update:count': [count: number]
  test: [pipelineId: string]
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

const { canAddOwnPipelines, canChangeDefaultPipeline, unrestricted } = useUserResourceAccess()
const canEdit = computed(() => canAddOwnPipelines.value)
const canCreatePipeline = computed(() => canEdit.value)
const canSetDefault = computed(() => canChangeDefaultPipeline.value)
const selectable = computed(() => canEdit.value)
const canTest = computed(() => true)

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

function handleTest(pipeline: AgentPatternPipeline) {
  emit('test', pipeline.id)
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
  gap: 10px;
  flex-wrap: wrap;
  padding: 4px 0 2px;
}

.toolbar-count {
  font-size: 0.75rem;
  color: #6b7280;
}

.pipeline-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  flex: 1;
  min-height: 0;
  max-height: none;
  overflow-y: auto;
  padding-right: 4px;
}

.pipeline-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 12px;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  background: #f9fafb;
  transition: all 0.2s;
}

.pipeline-row.is-default {
  background: #f0f9eb;
  border-color: #b3e19d;
}

.pipeline-row.selected {
  border-color: #93c5fd;
  background: #eff6ff;
}

.pipeline-row.is-default.selected {
  background: #ecfdf5;
  border-color: #86efac;
}

.row-checkbox {
  flex-shrink: 0;
}

.pipeline-main {
  min-width: 0;
  flex: 1;
}

.pipeline-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.pipeline-name {
  font-weight: 500;
  color: #1f2937;
}

.pipeline-meta {
  margin-top: 4px;
  font-size: 0.75rem;
  color: #6b7280;
}

.meta-sep {
  margin: 0 4px;
}

.pipeline-actions {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.mono {
  font-family: 'SF Mono', 'Fira Code', monospace;
}
</style>

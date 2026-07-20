<template>
  <div class="home-pipeline-card" v-loading="loading">
    <template v-if="pipelines.length > 0">
      <div class="pipeline-list-head">
        <span class="section-label">流水线列表</span>
        <span class="list-count">{{ pipelines.length }} 个</span>
      </div>

      <div v-if="selectable" class="list-toolbar">
        <el-checkbox
          :model-value="allSelected"
          :indeterminate="partialSelected"
          @change="toggleSelectAll"
        >
          全选
        </el-checkbox>
        <template v-if="selectedIds.length > 0">
          <span class="toolbar-count">已选 {{ selectedIds.length }} 项</span>
          <el-button size="small" type="danger" :loading="batchDeleting" @click="handleBatchDelete">
            批量删除
          </el-button>
          <el-button size="small" text @click="clearSelection">取消选择</el-button>
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
              <el-tag
                v-if="pipeline.id === selectedDefaultId"
                size="small"
                type="success"
                effect="light"
              >
                默认
              </el-tag>
            </div>
            <div class="pipeline-meta">
              <span class="mono">{{ pipeline.id }}</span>
              <span class="meta-sep">·</span>
              <span>{{ pipeline.nodes?.length || 0 }} 节点</span>
            </div>
          </div>
          <div class="pipeline-actions">
            <el-button
              v-if="canTest"
              size="small"
              @click="handleTest(pipeline)"
            >
              测试
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
              {{ pipeline.id === selectedDefaultId ? '当前默认' : '设为默认' }}
            </el-button>
            <el-button
              v-if="canEdit && canConfigureCapabilitySlots(pipeline)"
              size="small"
              type="warning"
              plain
              @click="openRouteAssign(pipeline)"
            >
              配置模型
            </el-button>
            <PipelineFeatureGuard
              feature="pipelineEdit"
              :pipeline="pipeline"
              :is-admin="authStore.isAdmin"
              action-label="编辑"
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
                  编辑
                </el-button>
              </template>
            </PipelineFeatureGuard>
            <el-button
              v-if="canEdit"
              size="small"
              type="danger"
              plain
              :disabled="pipeline.id === selectedDefaultId"
              @click="handleDelete(pipeline)"
            >
              删除
            </el-button>
          </div>
        </div>
      </div>
    </template>

    <el-empty v-else-if="!loading" description="暂无流水线" :image-size="56">
      <el-button v-if="canCreatePipeline" type="primary" plain size="small" @click="openCreate">
        创建流水线
      </el-button>
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
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Edit } from '@element-plus/icons-vue'
import PipelineCreateDialog from '@/components/pipeline/PipelineCreateDialog.vue'
import type { PipelineCreateInfo } from '@/components/pipeline/PipelineCreateDialog.vue'
import PipelineEditorDialog from '@/components/pipeline/PipelineEditorDialog.vue'
import PipelineFeatureGuard from '@/components/pipeline/PipelineFeatureGuard.vue'
import CapabilitySlotsDialog from '@/components/pipeline/CapabilitySlotsDialog.vue'
import { useEdition } from '@/composables/useEdition'
import { useAuthStore } from '@/stores/auth'
import { useUserResourceAccess } from '@/composables/useUserResourceAccess'
import { canConfigureCapabilitySlots } from '@/utils/capabilitySlots'
import {
  getPipelines,
  getPipeline,
  getPipelineDefaults,
  updatePipelineDefaults,
  deletePipeline,
  parsePipelinesResponse,
  type AgentPatternPipeline
} from '@/api/pipeline'

const emit = defineEmits<{
  'update:count': [count: number]
  test: [pipelineId: string]
}>()

const { isPersonal, isMinimal, isTeam } = useEdition()
const authStore = useAuthStore()

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
const routeAssignVisible = ref(false)
const routeAssignPipelineId = ref('')

const { canAddOwnPipelines, canChangeDefaultPipeline } = useUserResourceAccess()
// team 普通用户可编辑租户内流水线（受 can_add_own_pipelines 控制）
const canEdit = computed(
  () =>
    authStore.isAdmin ||
    isPersonal.value ||
    isMinimal.value ||
    (isTeam.value && canAddOwnPipelines.value)
)
const canCreatePipeline = computed(() => canEdit.value)
const canSetDefault = computed(
  () =>
    authStore.isAdmin ||
    isPersonal.value ||
    isMinimal.value ||
    (isTeam.value && canChangeDefaultPipeline.value)
)
/** 批量选择仅 minimal（精简台高频清理）；personal/team 走单条操作 */
const selectable = computed(() => isMinimal.value)
/** 全角色流水线「测试」→ MinimalChat（含 team admin） */
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
      pipelines.value = [
        {
          id: selectedDefaultId.value,
          name: selectedDefaultId.value,
          description: '',
          version: '1.0',
          nodes: []
        },
        ...pipelines.value
      ]
    }
  } catch (error: any) {
    ElMessage.error('加载流水线失败：' + (error.message || '未知错误'))
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
      found ? `已将「${found.name}」设为默认流水线` : '默认流水线已更新'
    )
  } catch (error: any) {
    ElMessage.error('设置失败：' + (error.message || '未知错误'))
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

async function handleDelete(pipeline: AgentPatternPipeline) {
  if (pipeline.id === selectedDefaultId.value) {
    ElMessage.warning('不能删除当前默认流水线，请先设置其他流水线为默认')
    return
  }
  try {
    await ElMessageBox.confirm(
      `确定删除流水线「${pipeline.name || pipeline.id}」吗？此操作不可恢复。`,
      '确认删除',
      { confirmButtonText: '删除', cancelButtonText: '取消', type: 'warning' }
    )
  } catch {
    return
  }
  try {
    await deletePipeline(pipeline.id)
    ElMessage.success(`已删除流水线「${pipeline.name || pipeline.id}」`)
    loadPipelines()
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.message || '删除流水线失败')
  }
}

async function handleBatchDelete() {
  if (!selectedIds.value.length) return
  const ids = selectedIds.value.filter(id => id !== selectedDefaultId.value)
  if (!ids.length) {
    ElMessage.warning('不能删除当前默认流水线，请先设置其他流水线为默认')
    return
  }
  const skippedDefault = ids.length < selectedIds.value.length
  try {
    await ElMessageBox.confirm(
      skippedDefault
        ? `将删除选中的 ${ids.length} 个流水线（已跳过当前默认），此操作不可恢复。`
        : `确定删除选中的 ${ids.length} 个流水线吗？此操作不可恢复。`,
      '批量删除',
      { confirmButtonText: '删除', cancelButtonText: '取消', type: 'warning' }
    )
  } catch {
    return
  }
  batchDeleting.value = true
  try {
    for (const id of ids) {
      await deletePipeline(id)
    }
    ElMessage.success(`已删除 ${ids.length} 个流水线`)
    selectedIds.value = []
    await loadPipelines()
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.message || '批量删除失败')
    await loadPipelines()
  } finally {
    batchDeleting.value = false
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
    ElMessage.warning('已使用列表数据打开编辑器')
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

defineExpose({ reload: loadPipelines, openCreate })
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

.list-count {
  font-size: 0.75rem;
  color: #9ca3af;
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

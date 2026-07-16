<template>
  <div class="home-pipeline-card" v-loading="loading">
    <template v-if="pipelines.length > 0">
      <div class="pipeline-list-head">
        <span class="section-label">流水线列表</span>
        <span class="list-count">{{ pipelines.length }} 个</span>
      </div>
      <div class="pipeline-list">
        <div
          v-for="pipeline in pipelines"
          :key="pipeline.id"
          class="pipeline-row"
          :class="{ 'is-default': pipeline.id === selectedDefaultId }"
        >
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
              v-if="canEdit && pipeline.id !== selectedDefaultId"
              size="small"
              text
              :loading="savingDefault && pendingDefaultId === pipeline.id"
              @click="selectDefault(pipeline.id)"
            >
              设为默认
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
      <el-button v-if="canEdit" type="primary" plain size="small" @click="openCreate">
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
import { useEdition } from '@/composables/useEdition'
import { useAuthStore } from '@/stores/auth'
import {
  getPipelines,
  getPipeline,
  getPipelineDefaults,
  updatePipelineDefaults,
  deletePipeline,
  parsePipelinesResponse,
  type AgentPatternPipeline
} from '@/api/pipeline'

const DISALLOWED_DEFAULT_IDS = new Set(['raw-forward', 'cache-hit', 'cache-mode'])

const emit = defineEmits<{
  'update:count': [count: number]
}>()

const { isPersonal, isMinimal } = useEdition()
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

const canEdit = computed(() => isPersonal.value || isMinimal.value || authStore.isAdmin)

watch(() => pipelines.value.length, (len) => {
  emit('update:count', len)
}, { immediate: true })

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
  if (!canEdit.value || !pipelineId) return
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
}

.mono {
  font-family: 'SF Mono', 'Fira Code', monospace;
}
</style>
<template>
  <div class="home-pipeline-card" v-loading="loading">
    <div class="default-section">
      <div class="default-head">
        <span class="section-label">系统默认流水线</span>
        <el-button type="primary" link size="small" @click="$router.push('/pipelines')">
          管理策略
        </el-button>
      </div>

      <el-select
        v-model="selectedDefaultId"
        placeholder="选择默认流水线"
        filterable
        :disabled="!canEdit"
        :loading="savingDefault"
        class="default-select"
        @change="handleDefaultChange"
      >
        <el-option
          v-for="pipeline in pipelineOptions"
          :key="pipeline.id"
          :label="pipeline.name"
          :value="pipeline.id"
        >
          <div class="option-row">
            <span class="option-name">{{ pipeline.name }}</span>
            <span class="option-meta">{{ pipeline.id }}</span>
          </div>
        </el-option>
      </el-select>

      <p v-if="canEdit" class="form-tip">
        客户端未指定流水线时，将自动使用此项处理请求
      </p>
      <p v-else class="form-tip muted">
        仅管理员可在首页修改系统默认流水线
      </p>

      <div v-if="currentDefault" class="current-default">
        <el-tag size="small" type="warning" effect="light">当前生效</el-tag>
        <span class="current-name">{{ currentDefault.name }}</span>
        <span class="current-meta mono">{{ currentDefault.id }} · {{ currentDefault.nodes?.length || 0 }} 节点</span>
      </div>
    </div>

    <template v-if="pipelines.length > 0">
      <el-divider style="margin: 12px 0" />
      <div class="pipeline-list-head">
        <span class="section-label">流水线列表</span>
        <span class="list-count">{{ pipelines.length }} 个</span>
      </div>
      <div class="pipeline-list">
        <div v-for="pipeline in pipelines" :key="pipeline.id" class="pipeline-row">
          <div class="pipeline-main">
            <div class="pipeline-title-row">
              <span class="pipeline-name">{{ pipeline.name }}</span>
              <el-tag
                v-if="pipeline.id === selectedDefaultId"
                size="small"
                type="warning"
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
          </div>
        </div>
      </div>
    </template>

    <el-empty v-else-if="!loading" description="暂无流水线，请先在策略管理中创建" :image-size="56">
      <el-button type="primary" plain size="small" @click="$router.push('/pipelines')">
        去创建
      </el-button>
    </el-empty>

    <PipelineEditorDialog
      v-model="editorVisible"
      :pipeline="editingPipeline"
      @saved="handleEditorSaved"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Edit } from '@element-plus/icons-vue'
import PipelineEditorDialog from '@/components/pipeline/PipelineEditorDialog.vue'
import PipelineFeatureGuard from '@/components/pipeline/PipelineFeatureGuard.vue'
import { useEdition } from '@/composables/useEdition'
import { useAuthStore } from '@/stores/auth'
import {
  getPipelines,
  getPipeline,
  getPipelineDefaults,
  updatePipelineDefaults,
  parsePipelinesResponse,
  type AgentPatternPipeline
} from '@/api/pipeline'

const DISALLOWED_DEFAULT_IDS = new Set(['raw-forward', 'cache-hit', 'cache-mode'])

const { isPersonal, isMinimal } = useEdition()
const authStore = useAuthStore()

const pipelines = ref<AgentPatternPipeline[]>([])
const selectedDefaultId = ref('')
const pendingDefaultId = ref('')
const loading = ref(false)
const savingDefault = ref(false)
const editorVisible = ref(false)
const editingPipeline = ref<AgentPatternPipeline | null>(null)

const canEdit = computed(() => isPersonal.value || isMinimal.value || authStore.isAdmin)

const pipelineOptions = computed(() =>
  pipelines.value
    .filter(p => p.id && !DISALLOWED_DEFAULT_IDS.has(p.id))
    .map(p => ({
      id: p.id,
      name: p.name || p.id,
      nodes: p.nodes
    }))
)

const currentDefault = computed(() =>
  pipelines.value.find(p => p.id === selectedDefaultId.value) || null
)

async function loadPipelines() {
  loading.value = true
  try {
    const [listRes, defaultsRes] = await Promise.all([getPipelines(), getPipelineDefaults()])
    pipelines.value = parsePipelinesResponse(listRes)
    const defaults = (defaultsRes as { data?: { default_pipeline_id?: string } })?.data ?? defaultsRes
    selectedDefaultId.value = (defaults as { default_pipeline_id?: string })?.default_pipeline_id || ''

    if (
      selectedDefaultId.value &&
      !pipelineOptions.value.some(p => p.id === selectedDefaultId.value)
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

function handleDefaultChange(pipelineId: string) {
  persistDefault(pipelineId)
}

function selectDefault(pipelineId: string) {
  selectedDefaultId.value = pipelineId
  persistDefault(pipelineId)
}

async function openEditor(pipeline: AgentPatternPipeline) {
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

defineExpose({ reload: loadPipelines })
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

.default-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.default-head,
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

.default-select {
  width: 100%;
}

.form-tip {
  margin: 0;
  font-size: 0.75rem;
  color: #6b7280;
  line-height: 1.4;
}

.form-tip.muted {
  color: #9ca3af;
}

.current-default {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  padding: 8px 10px;
  border-radius: 8px;
  background: #fffbeb;
  border: 1px solid #fde68a;
}

.current-name {
  font-weight: 500;
  color: #92400e;
}

.current-meta {
  font-size: 0.75rem;
  color: #b45309;
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

.option-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  width: 100%;
}

.option-name {
  font-weight: 500;
}

.option-meta {
  font-size: 0.75rem;
  color: #9ca3af;
}

.mono {
  font-family: 'SF Mono', 'Fira Code', monospace;
}
</style>
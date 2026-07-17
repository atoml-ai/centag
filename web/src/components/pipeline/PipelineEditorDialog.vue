<template>
  <el-dialog
    :model-value="modelValue"
    :title="dialogTitle"
    :width="dialogWidth"
    :close-on-click-modal="false"
    :close-on-press-escape="false"
    :top="dialogTop"
    class="pipeline-editor-dialog"
    modal-class="pipeline-editor-modal"
    :before-close="handleBeforeClose"
    @update:model-value="emit('update:modelValue', $event)"
  >
    <div v-if="modelValue && localPipeline" class="pipeline-editor-actions">
      <el-tooltip
        :disabled="hasRouterNode"
        content="请先添加路由节点，再新增分类"
        placement="bottom"
      >
        <span>
          <el-button
            size="small"
            type="primary"
            plain
            :disabled="!hasRouterNode"
            @click="addCategoryVisible = true"
          >
            新增分类
          </el-button>
        </span>
      </el-tooltip>
      <el-button
        v-if="canConfigureCapabilitySlots(localPipeline)"
        size="small"
        type="warning"
        plain
        :disabled="!localPipeline.id || isCreateMode"
        @click="slotsDialogVisible = true"
      >
        配置模型
      </el-button>
      <PipelineFeatureGuard
        feature="routeAutoBuild"
        :pipeline="localPipeline"
        :is-admin="authStore.isAdmin"
        action-label="自动配置路由"
      >
        <template #default="{ disabled }">
          <el-button
            size="small"
            type="warning"
            :disabled="disabled || !canRunRouteAutoBuild"
            @click="openRouteAutoBuildDialog"
          >
            自动配置路由
          </el-button>
        </template>
      </PipelineFeatureGuard>
    </div>

    <PipelineCanvas
      v-if="modelValue && localPipeline"
      ref="canvasRef"
      :pipeline="localPipeline"
      :backends="backends"
      :storages="storages"
      :is-create="isCreateMode"
      @save="onCanvasSave"
      @update:pipeline="onPipelineUpdate"
      @dirty-change="canvasDirty = $event"
    />

    <el-dialog v-model="routeAutoBuildVisible" title="自动配置路由后端模型" width="760px" append-to-body>
      <el-alert
        type="info"
        :closable="false"
        style="margin-bottom: 12px"
        description="仅对包含 router.routes 的流水线生效。可先预览，再应用。"
      />
      <el-form :model="routeAutoBuildForm" label-width="100px">
        <el-form-item label="目标流水线">
          <el-tag type="primary">{{ localPipeline?.id || '-' }}</el-tag>
        </el-form-item>
        <el-form-item label="策略">
          <el-select v-model="routeAutoBuildForm.strategy" style="width: 220px">
            <el-option label="快速匹配（fast）" value="fast" />
            <el-option label="平衡（balance）" value="balance" />
            <el-option label="成本优先（cost）" value="cost" />
            <el-option label="质量优先（quality）" value="quality" />
            <el-option label="延迟优先（latency）" value="latency" />
          </el-select>
        </el-form-item>
        <el-form-item label="探测后端">
          <el-switch v-model="routeAutoBuildForm.probe_backends" />
          <div style="color: #909399; margin-top: 4px;">
            建议仅在刚改过后端配置时开启，避免应用时等待过久
          </div>
        </el-form-item>
        <el-form-item label="灰度模式">
          <el-switch v-model="routeAutoBuildForm.canary" />
        </el-form-item>
        <el-form-item label="最大更新数">
          <el-input-number
            v-model="routeAutoBuildForm.max_updates"
            :min="0"
            :max="100"
            :disabled="!routeAutoBuildForm.canary"
          />
        </el-form-item>
      </el-form>

      <el-divider />
      <div v-if="routeAutoBuildResult" style="margin-bottom: 10px;">
        <el-space wrap>
          <el-tag type="success">策略: {{ routeAutoBuildResult.strategy }}</el-tag>
          <el-tag :type="routeAutoBuildResult.applied ? 'warning' : 'info'">
            {{ routeAutoBuildResult.applied ? '已应用' : '预览结果' }}
          </el-tag>
          <el-tag>更新项: {{ routeAutoBuildResult.updates?.length || 0 }}</el-tag>
          <el-tag>告警: {{ routeAutoBuildResult.warnings?.length || 0 }}</el-tag>
        </el-space>
      </div>
      <el-table
        v-if="routeAutoBuildResult?.updates?.length"
        :data="routeAutoBuildResult.updates"
        size="small"
        border
        max-height="300"
        style="margin-bottom: 10px"
      >
        <el-table-column type="index" label="#" width="50" />
        <el-table-column prop="target_node" label="节点" width="140" />
        <el-table-column prop="new_backend" label="新后端" width="120" />
        <el-table-column prop="new_model" label="新模型" width="120" />
        <el-table-column prop="reason" label="推荐理由" min-width="200" show-overflow-tooltip />
        <el-table-column label="策略因子" min-width="280">
          <template #default="{ row }">
            <el-space v-if="hasStrategyFactors(row.strategy_factors)" size="small" wrap>
              <el-tag size="small" type="info">{{ strategyFactorSummary(row.strategy_factors) }}</el-tag>
              <el-popover placement="left" :width="420" trigger="hover">
                <template #reference>
                  <el-button link type="primary">查看详情</el-button>
                </template>
                <div style="max-height: 320px; overflow: auto;">
                  <template v-for="section in strategyFactorSections(row.strategy_factors)" :key="section.title">
                    <div style="font-weight: 600; margin: 6px 0 4px;">{{ section.title }}</div>
                    <div style="display: grid; grid-template-columns: 130px 1fr; row-gap: 4px; column-gap: 8px; margin-bottom: 6px;">
                      <template v-for="item in section.items" :key="`${section.title}-${item.key}`">
                        <div style="color: #909399;">{{ item.label }}</div>
                        <div>
                          <el-tag v-if="item.tagType" size="small" :type="item.tagType">{{ item.value }}</el-tag>
                          <span v-else>{{ item.value }}</span>
                        </div>
                      </template>
                    </div>
                  </template>
                </div>
              </el-popover>
            </el-space>
            <span v-else>-</span>
          </template>
        </el-table-column>
        <el-table-column prop="category" label="分类" width="200" show-overflow-tooltip />
      </el-table>
      <el-alert
        v-for="(w, idx) in routeAutoBuildResult?.warnings || []"
        :key="idx"
        type="warning"
        :title="w"
        :closable="false"
        style="margin-bottom: 8px"
      />

      <template #footer>
        <el-button :loading="routeAutoBuildSubmitting" @click="runRouteAutoBuild(true)">预览</el-button>
        <el-button type="primary" :loading="routeAutoBuildSubmitting" @click="runRouteAutoBuild(false)">应用</el-button>
        <el-button type="danger" plain :loading="routeAutoBuildSubmitting" @click="rollbackRouteAutoBuild">一键回滚</el-button>
      </template>
    </el-dialog>

    <AddCategoryDialog
      v-model="addCategoryVisible"
      :pipeline="localPipeline"
      @applied="onCategoryApplied"
    />

    <CapabilitySlotsDialog
      v-if="localPipeline?.id"
      v-model="slotsDialogVisible"
      :pipeline-id="localPipeline.id"
      @saved="onSlotsSaved"
    />
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import PipelineCanvas from '@/components/PipelineCanvas.vue'
import PipelineFeatureGuard from '@/components/pipeline/PipelineFeatureGuard.vue'
import AddCategoryDialog from '@/components/pipeline/AddCategoryDialog.vue'
import CapabilitySlotsDialog from '@/components/pipeline/CapabilitySlotsDialog.vue'
import { useAuthStore } from '@/stores/auth'
import {
  createPipeline,
  getPipeline,
  autoBuildPipeline,
  rollbackAutoBuildPipeline,
  type AutoBuildRouteResponse,
  updatePipeline,
  type AgentPatternPipeline
} from '@/api/pipeline'
import api from '@/api'
import { canConfigureCapabilitySlots, listRouterNodes } from '@/utils/capabilitySlots'

const authStore = useAuthStore()

const props = defineProps<{
  modelValue: boolean
  pipeline: AgentPatternPipeline | null
  /** 是否为新建模式（首次保存走 create，之后走 update） */
  isCreate?: boolean
}>()

const dialogWidth = computed(() => '98%')
const dialogTop = computed(() => '1.5vh')

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  saved: [pipeline: AgentPatternPipeline]
  'update:pipeline': [pipeline: AgentPatternPipeline]
  closed: []
}>()

const canvasRef = ref<InstanceType<typeof PipelineCanvas> | null>(null)
const localPipeline = ref<AgentPatternPipeline | null>(null)
const backends = ref<any[]>([])
const storages = ref<any[]>([])
const canvasDirty = ref(false)
const saving = ref(false)
const depsLoaded = ref(false)
const isCreateMode = ref(false)
const routeAutoBuildVisible = ref(false)
const routeAutoBuildSubmitting = ref(false)
const routeAutoBuildResult = ref<AutoBuildRouteResponse | null>(null)
const routeAutoBuildPreviewSignature = ref('')
const routeAutoBuildForm = ref({
  strategy: 'fast' as 'balance' | 'cost' | 'quality' | 'latency' | 'fast',
  probe_backends: false,
  canary: false,
  max_updates: 10
})
const addCategoryVisible = ref(false)
const slotsDialogVisible = ref(false)

const hasRouterNode = computed(() =>
  !!localPipeline.value && listRouterNodes(localPipeline.value).length > 0
)

function onCategoryApplied(pipeline: AgentPatternPipeline) {
  localPipeline.value = pipeline
  canvasDirty.value = true
  emit('update:pipeline', pipeline)
}

function onSlotsSaved(pipeline: AgentPatternPipeline) {
  localPipeline.value = pipeline
  canvasDirty.value = false
  emit('update:pipeline', pipeline)
  emit('saved', pipeline)
}

const dialogTitle = computed(() => {
  const p = localPipeline.value
  if (!p) return '流水线可视化编辑'
  const id = p.id || ''
  const name = p.name || ''
  if (id && name) return `流水线可视化编辑 — ${id} · ${name}`
  if (id) return `流水线可视化编辑 — ${id}`
  if (name) return `流水线可视化编辑 — ${name}`
  return '流水线可视化编辑'
})

const canRunRouteAutoBuild = computed(() => {
  return !!localPipeline.value?.id && !isCreateMode.value
})

function syncLocalPipelineFromProps() {
  if (!props.pipeline) {
    if (isCreateMode.value) {
      localPipeline.value = {
        id: '',
        name: '',
        description: '',
        version: '1.0',
        nodes: [],
        created_at: '',
        updated_at: ''
      }
    } else {
      localPipeline.value = null
    }
    return
  }
  localPipeline.value = JSON.parse(JSON.stringify(props.pipeline))
  canvasDirty.value = false
}

watch(
  () => props.isCreate,
  (creating) => {
    isCreateMode.value = !!creating
  },
  { immediate: true }
)

watch(
  () => props.modelValue,
  async (visible, wasVisible) => {
    if (visible) {
      syncLocalPipelineFromProps()
      if (!depsLoaded.value) {
        await loadDeps()
      }
    } else {
      localPipeline.value = null
    }
    if (wasVisible && !visible) {
      emit('closed')
    }
  },
  { flush: 'sync' }
)

watch(
  () => props.pipeline?.id,
  (id, prevId) => {
    if (props.modelValue && id && id !== prevId) {
      syncLocalPipelineFromProps()
    }
  }
)

function onPipelineUpdate(pipeline: AgentPatternPipeline) {
  localPipeline.value = pipeline
  emit('update:pipeline', pipeline)
}

async function onCanvasSave(pipeline: AgentPatternPipeline) {
  const merged: AgentPatternPipeline = {
    ...(localPipeline.value || {}),
    ...pipeline,
    id: pipeline.id || localPipeline.value?.id || '',
    name: pipeline.name || localPipeline.value?.name || '',
    shortcut_code: pipeline.shortcut_code ?? localPipeline.value?.shortcut_code ?? ''
  }
  localPipeline.value = merged
  const ok = await handleSave(merged, false)
  if (ok) {
    canvasRef.value?.clearDirtyState?.()
    emit('update:pipeline', merged)
  }
}

async function loadDeps() {
  try {
    const [backendsRes, storagesRes] = await Promise.allSettled([
      api.get('/api/v1/backends'),
      api.get('/api/v1/storage')
    ])
    if (backendsRes.status === 'fulfilled') {
      const res = backendsRes.value
      backends.value = Array.isArray(res) ? res : res?.data || []
    }
    if (storagesRes.status === 'fulfilled') {
      const res = storagesRes.value
      let storageData = res?.data || res
      if (storageData && typeof storageData === 'object' && 'storages' in storageData) {
        storages.value = Array.isArray(storageData.storages) ? storageData.storages : []
      } else {
        storages.value = Array.isArray(storageData) ? storageData : []
      }
    }
    depsLoaded.value = true
  } catch {
    backends.value = []
    storages.value = []
  }
}

function validateBeforeSave(pipeline: AgentPatternPipeline): string | null {
  if (!pipeline.id?.trim()) return '流水线 ID 不能为空'
  if (!pipeline.name?.trim()) return '流水线名称不能为空'
  return null
}

async function handleSave(pipeline: AgentPatternPipeline, closeAfterSave = true) {
  const validationError = validateBeforeSave(pipeline)
  if (validationError) {
    ElMessage.warning(validationError)
    return false
  }

  saving.value = true
  try {
    if (isCreateMode.value) {
      await createPipeline(pipeline)
      isCreateMode.value = false
      ElMessage.success('流水线创建成功')
    } else {
      await updatePipeline(pipeline.id, pipeline)
      ElMessage.success('流水线更新成功')
    }
    canvasDirty.value = false
    emit('saved', pipeline)
    if (closeAfterSave) {
      emit('update:modelValue', false)
    }
    return true
  } catch (error: any) {
    canvasDirty.value = true
    ElMessage.error('保存失败：' + (error.message || error))
    return false
  } finally {
    saving.value = false
  }
}

function openRouteAutoBuildDialog() {
  routeAutoBuildResult.value = null
  routeAutoBuildPreviewSignature.value = ''
  routeAutoBuildVisible.value = true
}

function hasStrategyFactors(factors?: Record<string, any>) {
  return !!factors && typeof factors === 'object' && Object.keys(factors).length > 0
}

function factorValue(factors: Record<string, any>, key: string) {
  const value = factors[key]
  if (value === undefined || value === null || value === '') return ''
  if (key === 'strategy') {
    const mapping: Record<string, string> = {
      fast: '快速匹配',
      balance: '平衡',
      cost: '成本优先',
      quality: '质量优先',
      latency: '延迟优先',
      speed: '延迟优先',
      price: '成本优先'
    }
    const raw = String(value).toLowerCase()
    return mapping[raw] || String(value)
  }
  if (key === 'task_type') {
    const mapping: Record<string, string> = {
      code_generation: '代码生成',
      simple_chat: '简单对话',
      complex_reasoning: '复杂推理',
      long_text: '长文本',
      translation: '翻译',
      analysis: '分析',
      creative: '创意写作',
      embedding: '向量'
    }
    const raw = String(value).toLowerCase()
    return mapping[raw] || String(value)
  }
  if (key === 'health_status') {
    const mapping: Record<string, string> = {
      healthy: '健康',
      unhealthy: '异常',
      unknown: '未知',
      checking: '检测中'
    }
    const raw = String(value).toLowerCase()
    return mapping[raw] || String(value)
  }
  if (key === 'local_backend') return value ? '是' : '否'
  if (typeof value === 'boolean') return value ? '是' : '否'
  if (typeof value === 'number') return Number.isInteger(value) ? `${value}` : value.toFixed(4)
  return String(value)
}

function factorTagType(key: string, value: string) {
  if (!value) return ''
  if (key === 'health_status') {
    if (value === '健康') return 'success'
    if (value === '检测中') return 'warning'
    if (value === '异常') return 'danger'
    return 'info'
  }
  if (key === 'local_backend') {
    return value === '是' ? 'success' : 'info'
  }
  if (key === 'strategy') {
    if (value === '质量优先') return 'danger'
    if (value === '成本优先') return 'warning'
    if (value === '延迟优先') return 'success'
    if (value === '平衡') return 'primary'
    return 'info'
  }
  return ''
}

function strategyFactorSummary(factors?: Record<string, any>) {
  if (!hasStrategyFactors(factors)) return '-'
  const strategy = factorValue(factors!, 'strategy') || '-'
  const backend = factorValue(factors!, 'backend_id') || '-'
  const task = factorValue(factors!, 'task_type') || '-'
  return `${strategy} · ${task} · ${backend}`
}

function strategyFactorSections(factors?: Record<string, any>) {
  if (!hasStrategyFactors(factors)) return []
  const grouped = [
    {
      title: '基础',
      items: [
        { key: 'strategy', label: '策略' },
        { key: 'task_type', label: '任务类型' },
        { key: 'backend_id', label: '后端' },
        { key: 'priority', label: '优先级' },
        { key: 'local_backend', label: '本地后端' }
      ]
    },
    {
      title: '质量与健康',
      items: [
        { key: 'health_status', label: '健康状态' },
        { key: 'quality_score', label: '质量分' },
        { key: 'weight', label: '权重' }
      ]
    },
    {
      title: '成本与时延',
      items: [
        { key: 'unit_price', label: '单价' },
        { key: 'cost_score', label: '成本分' },
        { key: 'observed_latency_ms', label: '观测延迟(ms)' },
        { key: 'score_hint', label: '综合分提示' }
      ]
    }
  ]
  return grouped
    .map((section) => ({
      title: section.title,
      items: section.items
        .map((item) => {
          const value = factorValue(factors!, item.key)
          return {
          key: item.key,
          label: item.label,
          value,
          tagType: factorTagType(item.key, value)
        }
        })
        .filter((item) => item.value !== '')
    }))
    .filter((section) => section.items.length > 0)
}

async function refreshPipelineFromServer() {
  if (!localPipeline.value?.id) return
  try {
    const detail = await getPipeline(localPipeline.value.id)
    const latest = ((detail as any)?.data ?? detail) as AgentPatternPipeline
    localPipeline.value = JSON.parse(JSON.stringify(latest))
    await nextTick()
    canvasRef.value?.reloadFromPipeline?.(localPipeline.value)
    emit('update:pipeline', localPipeline.value)
    emit('saved', localPipeline.value)
    canvasRef.value?.clearDirtyState?.()
    canvasDirty.value = false
  } catch (error: any) {
    ElMessage.warning('已应用但刷新画布失败，请手动重开编辑器')
    console.warn(error)
  }
}

function autoBuildFormSignature(pipelineID: string) {
  return JSON.stringify({
    pipeline_id: pipelineID,
    strategy: routeAutoBuildForm.value.strategy,
    canary: !!routeAutoBuildForm.value.canary,
    max_updates: routeAutoBuildForm.value.canary ? routeAutoBuildForm.value.max_updates : 0
  })
}

async function applyRouteAutoBuildPreview(id: string): Promise<boolean> {
  const preview = routeAutoBuildResult.value
  if (!preview || preview.applied || preview.pipeline_id !== id) {
    return false
  }
  if (routeAutoBuildPreviewSignature.value !== autoBuildFormSignature(id)) {
    return false
  }
  const payload = {
    strategy: routeAutoBuildForm.value.strategy,
    dry_run: false,
    apply: true,
    probe_backends: false,
    canary: routeAutoBuildForm.value.canary,
    max_updates: routeAutoBuildForm.value.canary ? routeAutoBuildForm.value.max_updates : 0,
    preview_updates: preview.updates || []
  }
  const result = await autoBuildPipeline(id, payload)
  routeAutoBuildResult.value = result
  routeAutoBuildPreviewSignature.value = ''
  ElMessage.success('已直接应用预览结果')
  await refreshPipelineFromServer()
  return true
}

async function runRouteAutoBuild(dryRun: boolean) {
  const id = localPipeline.value?.id
  if (!id) {
    ElMessage.warning('请先保存流水线，再执行自动配置')
    return
  }
  routeAutoBuildSubmitting.value = true
  try {
    if (!dryRun && await applyRouteAutoBuildPreview(id)) {
      return
    }
    const payload = {
      strategy: routeAutoBuildForm.value.strategy,
      dry_run: dryRun,
      apply: !dryRun,
      probe_backends: routeAutoBuildForm.value.probe_backends,
      canary: routeAutoBuildForm.value.canary,
      max_updates: routeAutoBuildForm.value.canary ? routeAutoBuildForm.value.max_updates : 0
    }
    const result = await autoBuildPipeline(id, payload)
    console.log('Auto-build result:', result)
    console.log('Updates:', result?.updates)
    routeAutoBuildResult.value = result
    if (dryRun) {
      routeAutoBuildPreviewSignature.value = autoBuildFormSignature(id)
    } else {
      routeAutoBuildPreviewSignature.value = ''
    }
    ElMessage.success(dryRun ? '自动构建预览完成' : '自动构建已应用')
    if (!dryRun) {
      await refreshPipelineFromServer()
    }
  } catch (error: any) {
    ElMessage.error('自动构建失败：' + (error.message || error))
  } finally {
    routeAutoBuildSubmitting.value = false
  }
}

async function rollbackRouteAutoBuild() {
  const id = localPipeline.value?.id
  if (!id) {
    ElMessage.warning('请先保存流水线，再执行回滚')
    return
  }
  routeAutoBuildSubmitting.value = true
  try {
    await rollbackAutoBuildPipeline(id, false)
    ElMessage.success('已回滚最近一次自动构建')
    await refreshPipelineFromServer()
  } catch (error: any) {
    ElMessage.error('回滚失败：' + (error.message || error))
  } finally {
    routeAutoBuildSubmitting.value = false
  }
}

async function handleBeforeClose(done: () => void) {
  if (!canvasDirty.value) {
    done()
    return
  }

  try {
    await ElMessageBox.confirm(
      '检测到当前流水线有未保存修改，是否先保存再关闭？',
      '未保存的更改',
      {
        type: 'warning',
        confirmButtonText: '保存并关闭',
        cancelButtonText: '不保存关闭',
        distinguishCancelAndClose: true
      }
    )
    if (localPipeline.value) {
      const saved = await handleSave(localPipeline.value, false)
      if (saved) done()
    }
  } catch (action: any) {
    if (action === 'cancel') {
      canvasDirty.value = false
      done()
    }
  }
}
</script>

<style>
.pipeline-editor-dialog {
  max-width: 98vw;
}

.pipeline-editor-dialog .el-dialog__body {
  padding: 1.5%;
  overflow: hidden;
  height: calc(100vh - 11rem);
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.pipeline-editor-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}
</style>
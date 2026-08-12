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
        :content="t('pipelineEditor.addCategoryTooltip')"
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
            {{ t('pipelineEditor.addCategory') }}
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
        {{ t('pipelineEditor.configureModel') }}
      </el-button>
      <PipelineFeatureGuard
        feature="routeAutoBuild"
        :pipeline="localPipeline"
        :unrestricted="unrestricted"
        :action-label="t('pipelineEditor.autoConfigureRoute')"
      >
        <template #default="{ disabled }">
          <el-button
            size="small"
            type="warning"
            :disabled="disabled || !canRunRouteAutoBuild"
            @click="openRouteAutoBuildDialog"
          >
            {{ t('pipelineEditor.autoConfigureRoute') }}
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

    <el-dialog v-model="routeAutoBuildVisible" :title="t('pipelineEditor.autoConfigureRouteBackendModel')" width="760px" append-to-body>
      <el-alert
        type="info"
        :closable="false"
        style="margin-bottom: 12px"
        :description="t('pipelineEditor.autoConfigureRouteAlert')"
      />
      <el-form :model="routeAutoBuildForm" label-width="100px">
        <el-form-item :label="t('pipelineEditor.targetPipeline')">
          <el-tag type="primary">{{ localPipeline?.id || '-' }}</el-tag>
        </el-form-item>
        <el-form-item :label="t('pipelineEditor.strategy')">
          <el-select v-model="routeAutoBuildForm.strategy" style="width: 220px">
            <el-option :label="t('pipelineEditor.strategyFast')" value="fast" />
            <el-option :label="t('pipelineEditor.strategyBalance')" value="balance" />
            <el-option :label="t('pipelineEditor.strategyCost')" value="cost" />
            <el-option :label="t('pipelineEditor.strategyQuality')" value="quality" />
            <el-option :label="t('pipelineEditor.strategyLatency')" value="latency" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('pipelineEditor.probeBackends')">
          <el-switch v-model="routeAutoBuildForm.probe_backends" />
          <div style="color: #909399; margin-top: 4px;">
            {{ t('pipelineEditor.probeBackendsTip') }}
          </div>
        </el-form-item>
        <el-form-item :label="t('pipelineEditor.canaryMode')">
          <el-switch v-model="routeAutoBuildForm.canary" />
        </el-form-item>
        <el-form-item :label="t('pipelineEditor.maxUpdates')">
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
          <el-tag type="success">{{ t('pipelineEditor.strategyLabel') }}: {{ routeAutoBuildResult.strategy }}</el-tag>
          <el-tag :type="routeAutoBuildResult.applied ? 'warning' : 'info'">
            {{ routeAutoBuildResult.applied ? t('pipelineEditor.applied') : t('pipelineEditor.previewResult') }}
          </el-tag>
          <el-tag>{{ t('pipelineEditor.updateCount') }}: {{ routeAutoBuildResult.updates?.length || 0 }}</el-tag>
          <el-tag>{{ t('pipelineEditor.warningCount') }}: {{ routeAutoBuildResult.warnings?.length || 0 }}</el-tag>
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
        <el-table-column type="index" :label="t('pipelineEditor.columnIndex')" width="50" />
        <el-table-column prop="target_node" :label="t('pipelineEditor.columnNode')" width="140" />
        <el-table-column prop="new_backend" :label="t('pipelineEditor.columnNewBackend')" width="120" />
        <el-table-column prop="new_model" :label="t('pipelineEditor.columnNewModel')" width="120" />
        <el-table-column prop="reason" :label="t('pipelineEditor.columnReason')" min-width="200" show-overflow-tooltip />
        <el-table-column :label="t('pipelineEditor.columnStrategyFactors')" min-width="280">
          <template #default="{ row }">
            <el-space v-if="hasStrategyFactors(row.strategy_factors)" size="small" wrap>
              <el-tag size="small" type="info">{{ strategyFactorSummary(row.strategy_factors) }}</el-tag>
              <el-popover placement="left" :width="420" trigger="hover">
                <template #reference>
                  <el-button link type="primary">{{ t('pipelineEditor.viewDetails') }}</el-button>
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
        <el-table-column prop="category" :label="t('pipelineEditor.columnCategory')" width="200" show-overflow-tooltip />
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
        <el-button :loading="routeAutoBuildSubmitting" @click="runRouteAutoBuild(true)">{{ t('pipelineEditor.preview') }}</el-button>
        <el-button type="primary" :loading="routeAutoBuildSubmitting" @click="runRouteAutoBuild(false)">{{ t('pipelineEditor.apply') }}</el-button>
        <el-button type="danger" plain :loading="routeAutoBuildSubmitting" @click="rollbackRouteAutoBuild">{{ t('pipelineEditor.rollback') }}</el-button>
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
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import PipelineCanvas from '@/components/PipelineCanvas.vue'
import PipelineFeatureGuard from '@/components/pipeline/PipelineFeatureGuard.vue'
import AddCategoryDialog from '@/components/pipeline/AddCategoryDialog.vue'
import CapabilitySlotsDialog from '@/components/pipeline/CapabilitySlotsDialog.vue'
import { useUserResourceAccess } from '@/composables/useUserResourceAccess'
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

const { t } = useI18n()
const { unrestricted } = useUserResourceAccess()

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
  if (!p) return t('pipelineEditor.dialogTitle')
  const id = p.id || ''
  const name = p.name || ''
  if (id && name) return t('pipelineEditor.dialogTitleWithIdAndName', { id, name })
  if (id) return t('pipelineEditor.dialogTitleWithId', { id })
  if (name) return t('pipelineEditor.dialogTitleWithName', { name })
  return t('pipelineEditor.dialogTitle')
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
  if (!pipeline.id?.trim()) return t('pipelineEditor.validationIdEmpty')
  if (!pipeline.name?.trim()) return t('pipelineEditor.validationNameEmpty')
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
      ElMessage.success(t('pipelineEditor.createSuccess'))
    } else {
      await updatePipeline(pipeline.id, pipeline)
      ElMessage.success(t('pipelineEditor.updateSuccess'))
    }
    canvasDirty.value = false
    emit('saved', pipeline)
    if (closeAfterSave) {
      emit('update:modelValue', false)
    }
    return true
  } catch (error: any) {
    canvasDirty.value = true
    ElMessage.error(t('pipelineEditor.saveFailed') + (error.message || error))
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
      fast: t('pipelineEditor.factorStrategyFast'),
      balance: t('pipelineEditor.factorStrategyBalance'),
      cost: t('pipelineEditor.factorStrategyCost'),
      quality: t('pipelineEditor.factorStrategyQuality'),
      latency: t('pipelineEditor.factorStrategyLatency'),
      speed: t('pipelineEditor.factorStrategyLatency'),
      price: t('pipelineEditor.factorStrategyCost')
    }
    const raw = String(value).toLowerCase()
    return mapping[raw] || String(value)
  }
  if (key === 'task_type') {
    const mapping: Record<string, string> = {
      code_generation: t('pipelineEditor.factorTaskTypeCodeGeneration'),
      simple_chat: t('pipelineEditor.factorTaskTypeSimpleChat'),
      complex_reasoning: t('pipelineEditor.factorTaskTypeComplexReasoning'),
      long_text: t('pipelineEditor.factorTaskTypeLongText'),
      translation: t('pipelineEditor.factorTaskTypeTranslation'),
      analysis: t('pipelineEditor.factorTaskTypeAnalysis'),
      creative: t('pipelineEditor.factorTaskTypeCreative'),
      embedding: t('pipelineEditor.factorTaskTypeEmbedding')
    }
    const raw = String(value).toLowerCase()
    return mapping[raw] || String(value)
  }
  if (key === 'health_status') {
    const mapping: Record<string, string> = {
      healthy: t('pipelineEditor.factorHealthHealthy'),
      unhealthy: t('pipelineEditor.factorHealthUnhealthy'),
      unknown: t('pipelineEditor.factorHealthUnknown'),
      checking: t('pipelineEditor.factorHealthChecking')
    }
    const raw = String(value).toLowerCase()
    return mapping[raw] || String(value)
  }
  if (key === 'local_backend') return value ? t('pipelineEditor.factorYes') : t('pipelineEditor.factorNo')
  if (typeof value === 'boolean') return value ? t('pipelineEditor.factorYes') : t('pipelineEditor.factorNo')
  if (typeof value === 'number') return Number.isInteger(value) ? `${value}` : value.toFixed(4)
  return String(value)
}

function factorTagType(key: string, value: string) {
  if (!value) return ''
  if (key === 'health_status') {
    if (value === t('pipelineEditor.factorHealthHealthy')) return 'success'
    if (value === t('pipelineEditor.factorHealthChecking')) return 'warning'
    if (value === t('pipelineEditor.factorHealthUnhealthy')) return 'danger'
    return 'info'
  }
  if (key === 'local_backend') {
    return value === t('pipelineEditor.factorYes') ? 'success' : 'info'
  }
  if (key === 'strategy') {
    if (value === t('pipelineEditor.factorStrategyQuality')) return 'danger'
    if (value === t('pipelineEditor.factorStrategyCost')) return 'warning'
    if (value === t('pipelineEditor.factorStrategyLatency')) return 'success'
    if (value === t('pipelineEditor.factorStrategyBalance')) return 'primary'
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
      title: t('pipelineEditor.sectionBasic'),
      items: [
        { key: 'strategy', label: t('pipelineEditor.factorLabelStrategy') },
        { key: 'task_type', label: t('pipelineEditor.factorLabelTaskType') },
        { key: 'backend_id', label: t('pipelineEditor.factorLabelBackend') },
        { key: 'priority', label: t('pipelineEditor.factorLabelPriority') },
        { key: 'local_backend', label: t('pipelineEditor.factorLabelLocalBackend') }
      ]
    },
    {
      title: t('pipelineEditor.sectionQualityAndHealth'),
      items: [
        { key: 'health_status', label: t('pipelineEditor.factorLabelHealthStatus') },
        { key: 'quality_score', label: t('pipelineEditor.factorLabelQualityScore') },
        { key: 'weight', label: t('pipelineEditor.factorLabelWeight') }
      ]
    },
    {
      title: t('pipelineEditor.sectionCostAndLatency'),
      items: [
        { key: 'unit_price', label: t('pipelineEditor.factorLabelUnitPrice') },
        { key: 'cost_score', label: t('pipelineEditor.factorLabelCostScore') },
        { key: 'observed_latency_ms', label: t('pipelineEditor.factorLabelObservedLatencyMs') },
        { key: 'score_hint', label: t('pipelineEditor.factorLabelScoreHint') }
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
    ElMessage.warning(t('pipelineEditor.refreshCanvasFailed'))
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
  ElMessage.success(t('pipelineEditor.directApplyPreview'))
  await refreshPipelineFromServer()
  return true
}

async function runRouteAutoBuild(dryRun: boolean) {
  const id = localPipeline.value?.id
  if (!id) {
    ElMessage.warning(t('pipelineEditor.savePipelineFirst'))
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
    ElMessage.success(dryRun ? t('pipelineEditor.autoBuildPreviewDone') : t('pipelineEditor.autoBuildApplied'))
    if (!dryRun) {
      await refreshPipelineFromServer()
    }
  } catch (error: any) {
    ElMessage.error(t('pipelineEditor.autoBuildFailed') + (error.message || error))
  } finally {
    routeAutoBuildSubmitting.value = false
  }
}

async function rollbackRouteAutoBuild() {
  const id = localPipeline.value?.id
  if (!id) {
    ElMessage.warning(t('pipelineEditor.savePipelineFirstRollback'))
    return
  }
  routeAutoBuildSubmitting.value = true
  try {
    await rollbackAutoBuildPipeline(id, false)
    ElMessage.success(t('pipelineEditor.rollbackSuccess'))
    await refreshPipelineFromServer()
  } catch (error: any) {
    ElMessage.error(t('pipelineEditor.rollbackFailed') + (error.message || error))
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
      t('pipelineEditor.unsavedChangesConfirm'),
      t('pipelineEditor.unsavedChangesTitle'),
      {
        type: 'warning',
        confirmButtonText: t('pipelineEditor.saveAndClose'),
        cancelButtonText: t('pipelineEditor.closeWithoutSaving'),
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
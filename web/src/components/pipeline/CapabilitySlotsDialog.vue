<template>
  <el-dialog
    v-model="visible"
    :title="t('capabilitySlotsDialog.title')"
    width="760px"
    destroy-on-close
    class="capability-slots-dialog"
    @closed="onClosed"
  >
    <p class="dialog-desc">
      {{ t('capabilitySlotsDialog.desc') }}
    </p>

    <div v-loading="loading" class="assign-body">
      <div v-if="!rows.length && !loading" class="empty-hint">
        <p>{{ t('capabilitySlotsDialog.noSlots') }}</p>
        <p class="empty-sub">{{ t('capabilitySlotsDialog.noSlotsHint') }}</p>
      </div>

      <div v-for="row in rows" :key="row.slotId + ':' + row.nodeId" class="assign-row">
        <div class="row-head">
          <div class="row-title">
            <span class="label">{{ row.label }}</span>
            <span class="node-id mono">{{ row.nodeId }}</span>
            <el-tag v-for="tag in row.tags" :key="tag" size="small" effect="plain" class="tag-chip">{{ tag }}</el-tag>
          </div>
          <el-switch
            v-model="row.followSystem"
            inline-prompt
            :active-text="t('capabilitySlotsDialog.followDefault')"
            :inactive-text="t('capabilitySlotsDialog.custom')"
            @change="() => onFollowChange(row)"
          />
        </div>
        <p v-if="row.hint" class="row-hint">{{ row.hint }}</p>
        <div class="row-fields" :class="{ disabled: row.followSystem }">
          <div class="field">
            <span class="field-label">{{ t('capabilitySlotsDialog.backend') }}</span>
            <BackendSelector
              v-model="row.backend"
              :disabled="row.followSystem || saving"
              :placeholder="t('capabilitySlotsDialog.backendPlaceholder')"
              @change="() => { row.model = '' }"
            />
          </div>
          <div class="field">
            <span class="field-label">{{ t('capabilitySlotsDialog.model') }}</span>
            <ModelSelector
              v-model="row.model"
              :backend-id="row.backend"
              :disabled="row.followSystem || saving"
              :placeholder="t('capabilitySlotsDialog.modelPlaceholder')"
              :allow-create="true"
              :default-first-option="true"
            />
          </div>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="footer-bar">
        <div class="footer-left">
          <el-button text :disabled="saving || loading || !rows.length" @click="resetAllFollowSystem">
            {{ t('capabilitySlotsDialog.resetAll') }}
          </el-button>
          <el-button text :disabled="saving || loading || !rows.length" @click="handleRecommend">
            {{ t('capabilitySlotsDialog.recommend') }}
          </el-button>
        </div>
        <div class="footer-actions">
          <el-button :disabled="saving" @click="visible = false">{{ t('capabilitySlotsDialog.cancel') }}</el-button>
          <el-button type="primary" :loading="saving" :disabled="!rows.length" @click="handleSave">
            {{ t('capabilitySlotsDialog.save') }}
          </el-button>
        </div>
      </div>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import BackendSelector from '@/components/BackendSelector.vue'
import ModelSelector from '@/components/ModelSelector.vue'
import { getPipeline, updatePipeline, type AgentPatternPipeline } from '@/api/pipeline'
import { getBackends } from '@/api/backend'
import {
  applyCapabilitySlotBindings,
  buildCapabilitySlotRows,
  recommendCapabilitySlotRows,
  type BackendLike,
  type CapabilitySlotRow
} from '@/utils/capabilitySlots'

const { t } = useI18n()

const visible = defineModel<boolean>({ default: false })

const props = defineProps<{
  pipelineId: string
}>()

const emit = defineEmits<{
  saved: [pipeline: AgentPatternPipeline]
}>()

const loading = ref(false)
const saving = ref(false)
const pipeline = ref<AgentPatternPipeline | null>(null)
const rows = ref<CapabilitySlotRow[]>([])
const backends = ref<BackendLike[]>([])

watch(visible, async (open) => {
  if (!open) return
  await loadDetail()
})

async function loadBackends() {
  try {
    const res = await getBackends()
    const list = Array.isArray(res) ? res : (res as { data?: BackendLike[] })?.data || []
    backends.value = Array.isArray(list) ? list : []
  } catch {
    backends.value = []
  }
}

async function loadDetail() {
  if (!props.pipelineId) {
    rows.value = []
    return
  }
  loading.value = true
  try {
    const [detail] = await Promise.all([getPipeline(props.pipelineId), loadBackends()])
    const data = (detail as { data?: AgentPatternPipeline })?.data ?? (detail as AgentPatternPipeline)
    pipeline.value = data
    rows.value = buildCapabilitySlotRows(data)
  } catch (err: any) {
    ElMessage.error(err?.message || t('capabilitySlotsDialog.loadFailed'))
    rows.value = []
  } finally {
    loading.value = false
  }
}

function onFollowChange(row: CapabilitySlotRow) {
  if (row.followSystem) {
    row.backend = ''
    row.model = ''
  }
}

function resetAllFollowSystem() {
  for (const row of rows.value) {
    row.followSystem = true
    row.backend = ''
    row.model = ''
  }
}

async function handleRecommend() {
  try {
    await ElMessageBox.confirm(
      t('capabilitySlotsDialog.confirmRecommend'),
      t('capabilitySlotsDialog.recommendTitle'),
      { type: 'info', confirmButtonText: t('capabilitySlotsDialog.confirmButton'), cancelButtonText: t('capabilitySlotsDialog.cancelButton') }
    )
  } catch {
    return
  }
  const result = recommendCapabilitySlotRows(rows.value, backends.value)
  rows.value = result.rows
  if (result.warned) {
    ElMessage.warning(result.warned)
  } else {
    ElMessage.success(t('capabilitySlotsDialog.recommendSuccess'))
  }
}

async function handleSave() {
  if (!pipeline.value) return
  saving.value = true
  try {
    const next = applyCapabilitySlotBindings(pipeline.value, rows.value)
    await updatePipeline(next.id, next)
    pipeline.value = next
    ElMessage.success(t('capabilitySlotsDialog.saveSuccess'))
    emit('saved', next)
    visible.value = false
  } catch (err: any) {
    const msg = err?.response?.data?.message || err?.message || t('capabilitySlotsDialog.saveFailed')
    ElMessage.error(msg)
  } finally {
    saving.value = false
  }
}

function onClosed() {
  pipeline.value = null
  rows.value = []
}
</script>

<style scoped>
.dialog-desc {
  margin: 0 0 16px;
  font-size: 0.8125rem;
  line-height: 1.5;
  color: var(--el-text-color-secondary);
}

.assign-body {
  display: flex;
  flex-direction: column;
  gap: 14px;
  min-height: 120px;
}

.empty-hint {
  padding: 24px;
  text-align: center;
  color: var(--el-text-color-secondary);
  font-size: 0.875rem;
}

.empty-sub {
  margin-top: 8px;
  font-size: 0.8125rem;
}

.assign-row {
  padding: 12px 14px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;
  background: var(--el-fill-color-blank);
}

.row-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.row-title {
  display: flex;
  align-items: baseline;
  flex-wrap: wrap;
  gap: 8px;
  min-width: 0;
}

.label {
  font-weight: 600;
  font-size: 0.9375rem;
}

.node-id {
  font-size: 0.75rem;
  color: var(--el-text-color-secondary);
}

.tag-chip {
  margin: 0;
}

.mono {
  font-family: var(--font-mono, ui-monospace, monospace);
}

.row-hint {
  margin: 6px 0 10px;
  font-size: 0.75rem;
  color: var(--el-text-color-secondary);
}

.row-fields {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}

.row-fields.disabled {
  opacity: 0.55;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.field-label {
  font-size: 0.75rem;
  color: var(--el-text-color-secondary);
}

.footer-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  width: 100%;
  flex-wrap: wrap;
}

.footer-left {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.footer-actions {
  display: flex;
  gap: 8px;
}

@media (max-width: 640px) {
  .row-fields {
    grid-template-columns: 1fr;
  }
}
</style>

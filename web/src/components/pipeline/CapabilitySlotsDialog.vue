<template>
  <el-dialog
    v-model="visible"
    title="配置模型"
    width="760px"
    destroy-on-close
    class="capability-slots-dialog"
    @closed="onClosed"
  >
    <p class="dialog-desc">
      为各分类/阶段指定后端与模型。勾选「跟随系统默认」会随全局默认变化；保存后立即热加载。
      拓扑（关键词与节点）请在画布用「新增分类」调整，本面板不改路由。
    </p>

    <div v-loading="loading" class="assign-body">
      <div v-if="!rows.length && !loading" class="empty-hint">
        <p>当前流水线没有可配置的能力槽。</p>
        <p class="empty-sub">请先在画布用「新增分类」添加分类，或从带多分支的模板创建。</p>
      </div>

      <div v-for="row in rows" :key="row.slotId + ':' + row.nodeId" class="assign-row">
        <div class="row-head">
          <div class="row-title">
            <span class="label">{{ row.label }}</span>
            <span class="node-id mono">{{ row.nodeId }}</span>
            <el-tag v-for="t in row.tags" :key="t" size="small" effect="plain" class="tag-chip">{{ t }}</el-tag>
          </div>
          <el-switch
            v-model="row.followSystem"
            inline-prompt
            active-text="跟随默认"
            inactive-text="指定"
            @change="() => onFollowChange(row)"
          />
        </div>
        <p v-if="row.hint" class="row-hint">{{ row.hint }}</p>
        <div class="row-fields" :class="{ disabled: row.followSystem }">
          <div class="field">
            <span class="field-label">后端</span>
            <BackendSelector
              v-model="row.backend"
              :disabled="row.followSystem || saving"
              placeholder="选择后端"
              @change="() => { row.model = '' }"
            />
          </div>
          <div class="field">
            <span class="field-label">模型</span>
            <ModelSelector
              v-model="row.model"
              :backend-id="row.backend"
              :disabled="row.followSystem || saving"
              placeholder="选择模型"
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
            全部跟随系统默认
          </el-button>
          <el-button text :disabled="saving || loading || !rows.length" @click="handleRecommend">
            按标签重新推荐
          </el-button>
        </div>
        <div class="footer-actions">
          <el-button :disabled="saving" @click="visible = false">取消</el-button>
          <el-button type="primary" :loading="saving" :disabled="!rows.length" @click="handleSave">
            保存并生效
          </el-button>
        </div>
      </div>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
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
    // Always re-GET to avoid overwriting unsaved canvas edits with stale list row (R06)
    const [detail] = await Promise.all([getPipeline(props.pipelineId), loadBackends()])
    const data = (detail as { data?: AgentPatternPipeline })?.data ?? (detail as AgentPatternPipeline)
    pipeline.value = data
    rows.value = buildCapabilitySlotRows(data)
  } catch (err: any) {
    ElMessage.error(err?.message || '加载流水线失败')
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
      '将按各槽位 tags 打分填入表格草稿，不会立刻保存。是否覆盖当前表格选择？',
      '按标签重新推荐',
      { type: 'info', confirmButtonText: '覆盖表格', cancelButtonText: '取消' }
    )
  } catch {
    return
  }
  const result = recommendCapabilitySlotRows(rows.value, backends.value)
  rows.value = result.rows
  if (result.warned) {
    ElMessage.warning(result.warned)
  } else {
    ElMessage.success('已填入推荐结果，请确认后点击「保存并生效」')
  }
}

async function handleSave() {
  if (!pipeline.value) return
  saving.value = true
  try {
    const next = applyCapabilitySlotBindings(pipeline.value, rows.value)
    await updatePipeline(next.id, next)
    pipeline.value = next
    ElMessage.success('已保存并生效，下一次请求将使用新绑定')
    emit('saved', next)
    visible.value = false
  } catch (err: any) {
    const msg = err?.response?.data?.message || err?.message || '保存失败'
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

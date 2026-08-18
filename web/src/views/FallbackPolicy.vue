<template>
  <div class="fallback-policy-page" :class="{ embedded }">
    <div v-if="!embedded" class="hermes-header">
      <div class="hermes-header-left">
        <el-button type="primary" @click="openCreateDialog">
          <el-icon><Plus /></el-icon>
          {{ t('fallbackPolicy.newPolicy') }}
        </el-button>
      </div>
      <div class="hermes-header-right">
        <h1 class="hermes-title">{{ t('fallbackPolicy.pageTitle') }}</h1>
        <p class="hermes-subtitle">{{ t('fallbackPolicy.pageDescription') }}</p>
      </div>
    </div>
    <div v-else class="embedded-toolbar">
      <p class="embedded-desc">{{ t('fallbackPolicy.embeddedDesc') }}</p>
      <el-button type="primary" size="small" @click="openCreateDialog">
        <el-icon><Plus /></el-icon>
        {{ t('fallbackPolicy.newPolicy') }}
      </el-button>
    </div>

    <div class="fallback-policy-body" v-loading="loading">
      <el-table
        class="policy-table"
        :data="policies"
        style="width: 100%"
        :empty-text="t('fallbackPolicy.emptyState')"
      >
        <el-table-column prop="id" :label="t('fallbackPolicy.table.id')" min-width="110" show-overflow-tooltip />
        <el-table-column prop="enabled" :label="t('fallbackPolicy.table.status')" width="72" align="center">
          <template #default="{ row }">
            <el-switch v-model="row.enabled" size="small" @change="toggleEnabled(row)" />
          </template>
        </el-table-column>
        <el-table-column prop="name" :label="t('fallbackPolicy.table.name')" min-width="100" show-overflow-tooltip />
        <el-table-column prop="strategy" :label="t('fallbackPolicy.table.type')" min-width="120" show-overflow-tooltip>
          <template #default="{ row }">
            <el-tag size="small" :type="strategyTagType(row.strategy)">{{ strategyLabel(row.strategy) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('fallbackPolicy.table.ruleCount')" width="64" align="center">
          <template #default="{ row }">
            <el-tag size="small">{{ row.rules?.length || 0 }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('fallbackPolicy.table.actions')" width="64" align="center">
          <template #default="{ row }">
            <el-dropdown trigger="click">
              <el-button circle plain>
                <el-icon><MoreFilled /></el-icon>
              </el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item @click="openEditDialog(row)">
                    <el-icon><Edit /></el-icon>{{ t('fallbackPolicy.actions.edit') }}
                  </el-dropdown-item>
                  <el-dropdown-item @click="testPolicy(row)">
                    <el-icon><View /></el-icon>{{ t('fallbackPolicy.actions.test') }}
                  </el-dropdown-item>
                  <el-dropdown-item divided @click="confirmDeletePolicy(row.id)">
                    <el-icon><Delete /></el-icon>{{ t('fallbackPolicy.actions.delete') }}
                  </el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </template>
        </el-table-column>
      </el-table>

      <!-- 测试结果弹窗 -->
      <el-dialog v-model="testResultVisible" :title="t('fallbackPolicy.testResultDialog.title')" width="600px">
        <div v-if="testResult">
          <el-descriptions :column="2" border>
            <el-descriptions-item :label="t('fallbackPolicy.testResultDialog.policyId')">{{ testResult.policy_id }}</el-descriptions-item>
            <el-descriptions-item :label="t('fallbackPolicy.testResultDialog.policyType')">{{ strategyLabel(testResult.strategy) }}</el-descriptions-item>
          </el-descriptions>
          <el-table :data="testResult.rules" style="margin-top: 16px">
            <el-table-column prop="priority" :label="t('fallbackPolicy.testResultDialog.priority')" width="80" />
            <el-table-column prop="backend_id" :label="t('fallbackPolicy.testResultDialog.backend')" />
            <el-table-column prop="model" :label="t('fallbackPolicy.testResultDialog.model')" />
            <el-table-column prop="timeout_sec" :label="t('fallbackPolicy.testResultDialog.timeout')" width="80" />
          </el-table>
          <p style="margin-top: 12px; color: #909399; font-size: 12px">{{ testResult.note }}</p>
        </div>
      </el-dialog>
    </div>

    <!-- 新建/编辑对话框 -->
    <el-dialog v-model="dialogVisible" :title="editingPolicy ? t('fallbackPolicy.formDialog.editTitle') : t('fallbackPolicy.formDialog.addTitle')" width="700px" destroy-on-close>
      <el-form :model="form" label-width="100px">
        <el-form-item :label="t('fallbackPolicy.formDialog.idLabel')" required>
          <el-input v-model="form.id" :disabled="!!editingPolicy" :placeholder="t('fallbackPolicy.formDialog.idPlaceholder')" />
        </el-form-item>
        <el-form-item :label="t('fallbackPolicy.formDialog.nameLabel')" required>
          <el-input v-model="form.name" :placeholder="t('fallbackPolicy.formDialog.namePlaceholder')" />
        </el-form-item>
        <el-form-item :label="t('fallbackPolicy.formDialog.descriptionLabel')">
          <el-input v-model="form.description" type="textarea" :rows="2" />
        </el-form-item>
        <el-form-item :label="t('fallbackPolicy.formDialog.typeLabel')" required>
          <el-select v-model="form.strategy" style="width: 100%">
            <el-option :label="t('fallbackPolicy.strategyOptions.same_model_different_backend')" value="same_model_different_backend" />
            <el-option :label="t('fallbackPolicy.strategyOptions.same_backend_different_model')" value="same_backend_different_model" />
            <el-option :label="t('fallbackPolicy.strategyOptions.custom_chain')" value="custom_chain" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('fallbackPolicy.formDialog.enabledLabel')">
          <el-switch v-model="form.enabled" />
        </el-form-item>

        <!-- 规则列表 -->
        <el-form-item :label="t('fallbackPolicy.formDialog.rulesLabel')">
          <div style="width: 100%">
            <div v-for="(rule, idx) in form.rules" :key="idx" class="rule-row">
              <el-input-number v-model="rule.priority" :min="1" size="small" style="width: 80px" :placeholder="t('fallbackPolicy.formDialog.priorityPlaceholder')" />
              <el-input v-model="rule.backend_id" size="small" style="flex: 1" :placeholder="t('fallbackPolicy.formDialog.backendPlaceholder')" />
              <el-input v-model="rule.model" size="small" style="flex: 1" :placeholder="t('fallbackPolicy.formDialog.modelPlaceholder')" />
              <el-input-number v-model="rule.timeout_sec" :min="0" size="small" style="width: 100px" :placeholder="t('fallbackPolicy.formDialog.timeoutPlaceholder')" />
              <el-button size="small" type="danger" circle @click="removeRule(idx)">
                <el-icon><Delete /></el-icon>
              </el-button>
            </div>
            <el-button size="small" @click="addRule">
              <el-icon><Plus /></el-icon>
              {{ t('fallbackPolicy.formDialog.addRule') }}
            </el-button>
          </div>
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="dialogVisible = false">{{ t('fallbackPolicy.formDialog.cancel') }}</el-button>
        <el-button type="primary" @click="savePolicy" :loading="saving">{{ t('fallbackPolicy.formDialog.save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Delete, MoreFilled, Edit, View } from '@element-plus/icons-vue'
import {
  getFallbackPolicies,
  createFallbackPolicy,
  updateFallbackPolicy,
  deleteFallbackPolicy,
  testFallbackPolicy,
  type FallbackPolicy,
  type FallbackPolicyTestResult,
  type FallbackStrategyType,
  type FallbackRule
} from '@/api/fallback'

const { t } = useI18n()

withDefaults(
  defineProps<{
    /** 嵌入系统配置韧性页时隐藏独立页头 */
    embedded?: boolean
  }>(),
  { embedded: false }
)

const loading = ref(false)
const saving = ref(false)
const policies = ref<FallbackPolicy[]>([])
const dialogVisible = ref(false)
const editingPolicy = ref<FallbackPolicy | null>(null)
const testResultVisible = ref(false)
const testResult = ref<FallbackPolicyTestResult | null>(null)

const form = ref<{
  id: string
  name: string
  description: string
  strategy: FallbackStrategyType
  enabled: boolean
  rules: FallbackRule[]
}>({
  id: '',
  name: '',
  description: '',
  strategy: 'same_model_different_backend',
  enabled: true,
  rules: []
})

onMounted(() => {
  fetchPolicies()
})

async function fetchPolicies() {
  loading.value = true
  try {
    const res = await getFallbackPolicies()
    policies.value = Array.isArray(res) ? res : Array.isArray((res as any)?.data) ? (res as any).data : []
  } catch (err) {
    console.error('Failed to fetch fallback policies', err)
    ElMessage.error(t('fallbackPolicy.message.loadFailed'))
  } finally {
    loading.value = false
  }
}

function strategyLabel(strategy: FallbackStrategyType): string {
  const map: Record<FallbackStrategyType, string> = {
    same_model_different_backend: t('fallbackPolicy.strategyOptions.same_model_different_backend'),
    same_backend_different_model: t('fallbackPolicy.strategyOptions.same_backend_different_model'),
    custom_chain: t('fallbackPolicy.strategyOptions.custom_chain')
  }
  return map[strategy] || strategy
}

function strategyTagType(strategy: FallbackStrategyType): string {
  const map: Record<FallbackStrategyType, string> = {
    same_model_different_backend: 'primary',
    same_backend_different_model: 'success',
    custom_chain: 'warning'
  }
  return map[strategy] || 'info'
}

function openCreateDialog() {
  editingPolicy.value = null
  form.value = {
    id: '',
    name: '',
    description: '',
    strategy: 'same_model_different_backend',
    enabled: true,
    rules: [{ priority: 1, backend_id: '', model: '{{requested_model}}', timeout_sec: 0 }]
  }
  dialogVisible.value = true
}

function openEditDialog(policy: FallbackPolicy) {
  editingPolicy.value = policy
  form.value = {
    id: policy.id,
    name: policy.name,
    description: policy.description || '',
    strategy: policy.strategy,
    enabled: policy.enabled,
    rules: [...policy.rules.map(r => ({ ...r }))]
  }
  dialogVisible.value = true
}

function addRule() {
  form.value.rules.push({
    priority: form.value.rules.length + 1,
    backend_id: '',
    model: '{{requested_model}}',
    timeout_sec: 0
  })
}

function removeRule(idx: number) {
  form.value.rules.splice(idx, 1)
}

async function savePolicy() {
  if (!form.value.id || !form.value.name) {
    ElMessage.warning(t('fallbackPolicy.validation.fillRequired'))
    return
  }
  saving.value = true
  try {
    if (editingPolicy.value) {
      await updateFallbackPolicy(form.value.id, form.value)
      ElMessage.success(t('fallbackPolicy.message.updated'))
    } else {
      await createFallbackPolicy(form.value)
      ElMessage.success(t('fallbackPolicy.message.created'))
    }
    dialogVisible.value = false
    fetchPolicies()
  } catch (err) {
    console.error('Failed to save policy', err)
    ElMessage.error(t('fallbackPolicy.message.saveFailed'))
  } finally {
    saving.value = false
  }
}

async function confirmDeletePolicy(id: string) {
  try {
    await ElMessageBox.confirm(
      t('fallbackPolicy.confirm.deleteTitle'),
      t('fallbackPolicy.actions.delete'),
      { type: 'warning', confirmButtonText: t('fallbackPolicy.actions.delete'), cancelButtonText: t('fallbackPolicy.formDialog.cancel') }
    )
  } catch {
    return
  }
  await deletePolicy(id)
}

async function deletePolicy(id: string) {
  try {
    await deleteFallbackPolicy(id)
    ElMessage.success(t('fallbackPolicy.message.deleted'))
    fetchPolicies()
  } catch (err) {
    ElMessage.error(t('fallbackPolicy.message.deleteFailed'))
  }
}

async function toggleEnabled(policy: FallbackPolicy) {
  try {
    await updateFallbackPolicy(policy.id, policy)
    ElMessage.success(t('fallbackPolicy.message.statusUpdated'))
  } catch (err) {
    policy.enabled = !policy.enabled
    ElMessage.error(t('fallbackPolicy.message.updateFailed'))
  }
}

async function testPolicy(policy: FallbackPolicy) {
  try {
    const res = await testFallbackPolicy(policy.id)
    testResult.value = res
    testResultVisible.value = true
  } catch (err) {
    ElMessage.error(t('fallbackPolicy.message.testFailed'))
  }
}
</script>

<style scoped>
.fallback-policy-page {
  width: 100%;
  padding: 0 0 24px;
}

.fallback-policy-page.embedded {
  padding: 0;
}

.fallback-policy-body {
  margin-top: 20px;
}

.policy-table {
  width: 100%;
}

.policy-table :deep(.el-table__body-wrapper) {
  overflow-x: hidden;
}

.fallback-policy-page.embedded .fallback-policy-body {
  margin-top: 12px;
}

.embedded-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  margin-bottom: 4px;
}

.embedded-desc {
  margin: 0;
  color: #909399;
  font-size: 13px;
  line-height: 1.4;
}

.rule-row {
  display: flex;
  gap: 8px;
  margin-bottom: 8px;
  align-items: center;
}

.hermes-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  background: linear-gradient(135deg, #f8fafc 0%, #f1f5f9 100%);
  border-radius: 12px;
  border: 1px solid #e2e8f0;
  margin-bottom: 16px;
}

.hermes-title {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
}

.hermes-subtitle {
  margin: 4px 0 0;
  color: #909399;
  font-size: 14px;
}

.hermes-header-left .el-button {
  border-radius: 8px;
  font-weight: 500;
  transition: all 0.2s ease;
}

.el-button.is-circle {
  width: 32px;
  height: 32px;
  padding: 8px;
}

.hermes-header-left .el-button:hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}
</style>

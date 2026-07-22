<template>
  <div class="fallback-policy-page">
    <div class="hermes-header">
      <div class="hermes-header-left">
        <h1 class="hermes-title">降级策略管理</h1>
        <p class="hermes-subtitle">配置全局降级策略，支持同模型跨后端、同后端跨模型、自定义链路</p>
      </div>
      <div class="hermes-header-right">
        <el-button type="primary" @click="openCreateDialog">
          <el-icon><Plus /></el-icon>
          新建策略
        </el-button>
      </div>
    </div>

    <div class="fallback-policy-body" v-loading="loading">
      <el-table :data="policies" style="width: 100%" empty-text="暂无降级策略">
        <el-table-column prop="id" label="策略 ID" width="200" />
        <el-table-column prop="name" label="策略名称" min-width="180" />
        <el-table-column prop="strategy" label="策略类型" width="200">
          <template #default="{ row }">
            <el-tag :type="strategyTagType(row.strategy)">{{ strategyLabel(row.strategy) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="规则数" width="80" align="center">
          <template #default="{ row }">
            <el-tag size="small">{{ row.rules?.length || 0 }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="enabled" label="状态" width="80" align="center">
          <template #default="{ row }">
            <el-switch v-model="row.enabled" @change="toggleEnabled(row)" />
          </template>
        </el-table-column>
        <el-table-column label="操作" width="200" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click="openEditDialog(row)">编辑</el-button>
            <el-button size="small" @click="testPolicy(row)">测试</el-button>
            <el-popconfirm title="确定删除此策略？" @confirm="deletePolicy(row.id)">
              <template #reference>
                <el-button size="small" type="danger">删除</el-button>
              </template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>

      <!-- 测试结果弹窗 -->
      <el-dialog v-model="testResultVisible" title="降级路径预览" width="600px">
        <div v-if="testResult">
          <el-descriptions :column="2" border>
            <el-descriptions-item label="策略 ID">{{ testResult.policy_id }}</el-descriptions-item>
            <el-descriptions-item label="策略类型">{{ strategyLabel(testResult.strategy) }}</el-descriptions-item>
          </el-descriptions>
          <el-table :data="testResult.rules" style="margin-top: 16px">
            <el-table-column prop="priority" label="优先级" width="80" />
            <el-table-column prop="backend_id" label="后端" />
            <el-table-column prop="model" label="模型" />
            <el-table-column prop="timeout_sec" label="超时(s)" width="80" />
          </el-table>
          <p style="margin-top: 12px; color: #909399; font-size: 12px">{{ testResult.note }}</p>
        </div>
      </el-dialog>
    </div>

    <!-- 新建/编辑对话框 -->
    <el-dialog v-model="dialogVisible" :title="editingPolicy ? '编辑降级策略' : '新建降级策略'" width="700px" destroy-on-close>
      <el-form :model="form" label-width="100px">
        <el-form-item label="策略 ID" required>
          <el-input v-model="form.id" :disabled="!!editingPolicy" placeholder="如 gpt4o-fallback" />
        </el-form-item>
        <el-form-item label="策略名称" required>
          <el-input v-model="form.name" placeholder="如 GPT-4o 多后端降级" />
        </el-form-item>
        <el-form-item label="策略描述">
          <el-input v-model="form.description" type="textarea" :rows="2" />
        </el-form-item>
        <el-form-item label="策略类型" required>
          <el-select v-model="form.strategy" style="width: 100%">
            <el-option label="同模型跨后端" value="same_model_different_backend" />
            <el-option label="同后端跨模型" value="same_backend_different_model" />
            <el-option label="自定义降级链" value="custom_chain" />
          </el-select>
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="form.enabled" />
        </el-form-item>

        <!-- 规则列表 -->
        <el-form-item label="降级规则">
          <div style="width: 100%">
            <div v-for="(rule, idx) in form.rules" :key="idx" class="rule-row">
              <el-input-number v-model="rule.priority" :min="1" size="small" style="width: 80px" placeholder="优先级" />
              <el-input v-model="rule.backend_id" size="small" style="flex: 1" placeholder="后端 ID" />
              <el-input v-model="rule.model" size="small" style="flex: 1" placeholder="模型名（或 {{requested_model}}）" />
              <el-input-number v-model="rule.timeout_sec" :min="0" size="small" style="width: 100px" placeholder="超时(s)" />
              <el-button size="small" type="danger" circle @click="removeRule(idx)">
                <el-icon><Delete /></el-icon>
              </el-button>
            </div>
            <el-button size="small" @click="addRule">
              <el-icon><Plus /></el-icon>
              添加规则
            </el-button>
          </div>
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="savePolicy" :loading="saving">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Plus, Delete } from '@element-plus/icons-vue'
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
    ElMessage.error('获取降级策略列表失败')
  } finally {
    loading.value = false
  }
}

function strategyLabel(strategy: FallbackStrategyType): string {
  const map: Record<FallbackStrategyType, string> = {
    same_model_different_backend: '同模型跨后端',
    same_backend_different_model: '同后端跨模型',
    custom_chain: '自定义降级链'
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
    ElMessage.warning('请填写策略 ID 和名称')
    return
  }
  saving.value = true
  try {
    if (editingPolicy.value) {
      await updateFallbackPolicy(form.value.id, form.value)
      ElMessage.success('策略已更新')
    } else {
      await createFallbackPolicy(form.value)
      ElMessage.success('策略已创建')
    }
    dialogVisible.value = false
    fetchPolicies()
  } catch (err) {
    console.error('Failed to save policy', err)
    ElMessage.error('保存失败')
  } finally {
    saving.value = false
  }
}

async function deletePolicy(id: string) {
  try {
    await deleteFallbackPolicy(id)
    ElMessage.success('策略已删除')
    fetchPolicies()
  } catch (err) {
    ElMessage.error('删除失败')
  }
}

async function toggleEnabled(policy: FallbackPolicy) {
  try {
    await updateFallbackPolicy(policy.id, policy)
    ElMessage.success('状态已更新')
  } catch (err) {
    policy.enabled = !policy.enabled
    ElMessage.error('更新失败')
  }
}

async function testPolicy(policy: FallbackPolicy) {
  try {
    const res = await testFallbackPolicy(policy.id)
    testResult.value = res
    testResultVisible.value = true
  } catch (err) {
    ElMessage.error('测试失败')
  }
}
</script>

<style scoped>
.fallback-policy-page {
  padding: 20px;
}

.fallback-policy-body {
  margin-top: 20px;
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
</style>

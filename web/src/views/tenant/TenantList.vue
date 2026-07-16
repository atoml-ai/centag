<template>
  <div class="tenant-list">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>租户管理</span>
          <el-button :loading="loading" @click="loadTenants">
            <el-icon><Refresh /></el-icon>
            刷新
          </el-button>
        </div>
      </template>

      <el-alert
        type="info"
        :closable="false"
        show-icon
        style="margin-bottom: 16px"
        title="租户随用户账号自动创建，此处用于查看与管理配额。"
      />

      <el-table
        v-loading="loading"
        :data="tenants"
        stripe
        style="width: 100%"
      >
        <el-table-column prop="id" label="租户 ID" width="180" />
        <el-table-column prop="user_id" label="用户 ID" width="100" />
        <el-table-column prop="name" label="名称" min-width="120" />
        <el-table-column prop="description" label="描述" show-overflow-tooltip />
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="getStatusType(row.status)">
              {{ getStatusLabel(row.status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="日请求" width="120" align="center">
          <template #default="{ row }">
            <div v-if="row.quota" class="quota-mini">
              <el-progress
                :percentage="calcPercent(row.quota.used_today_requests, row.quota.daily_request_limit)"
                :stroke-width="12"
                :status="progressStatus(calcPercent(row.quota.used_today_requests, row.quota.daily_request_limit))"
              />
              <span class="quota-mini-text">{{ row.quota.used_today_requests ?? 0 }} / {{ row.quota.daily_request_limit ?? '∞' }}</span>
            </div>
            <span v-else>-</span>
          </template>
        </el-table-column>
        <el-table-column label="月 Token" width="150" align="center">
          <template #default="{ row }">
            <div v-if="row.quota" class="quota-mini">
              <el-progress
                :percentage="calcPercent(row.quota.used_month_tokens, row.quota.monthly_token_limit)"
                :stroke-width="12"
                :status="progressStatus(calcPercent(row.quota.used_month_tokens, row.quota.monthly_token_limit))"
              />
              <span class="quota-mini-text">{{ formatNumber(row.quota.used_month_tokens ?? 0) }} / {{ row.quota.monthly_token_limit ? formatNumber(row.quota.monthly_token_limit) : '∞' }}</span>
            </div>
            <span v-else>-</span>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="180">
          <template #default="{ row }">
            {{ formatDate(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="200" fixed="right">
          <template #default="{ row }">
            <el-button-group>
              <el-button size="small" @click="handleEdit(row)">
                <el-icon><Edit /></el-icon>
              </el-button>
              <el-button size="small" @click="handleQuota(row)">
                <el-icon><Setting /></el-icon>
              </el-button>
              <el-button size="small" type="danger" @click="handleDelete(row)">
                <el-icon><Delete /></el-icon>
              </el-button>
            </el-button-group>
          </template>
        </el-table-column>
      </el-table>

      <el-empty v-if="!loading && tenants.length === 0" description="暂无租户" />
    </el-card>

    <el-dialog v-model="dialogVisible" title="编辑租户" width="520px">
      <el-form :model="form" :rules="rules" ref="formRef" label-width="80px">
        <el-form-item label="ID">
          <el-input v-model="form.id" disabled />
        </el-form-item>
        <el-form-item label="名称" prop="name">
          <el-input v-model="form.name" placeholder="租户名称" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input
            v-model="form.description"
            type="textarea"
            :rows="2"
            placeholder="租户描述"
          />
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="form.status">
            <el-option label="活跃" value="active" />
            <el-option label="暂停" value="suspended" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">
          保存
        </el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="quotaVisible" title="配额管理" width="640px">
      <el-form :model="quotaForm" label-width="140px">
        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item label="日请求上限">
              <el-input-number v-model="quotaForm.daily_request_limit" :min="0" :step="100" style="width: 100%" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="月请求上限">
              <el-input-number v-model="quotaForm.monthly_request_limit" :min="0" :step="1000" style="width: 100%" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item label="日 Token 上限">
              <el-input-number v-model="quotaForm.daily_token_limit" :min="0" :step="10000" style="width: 100%" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="月 Token 上限">
              <el-input-number v-model="quotaForm.monthly_token_limit" :min="0" :step="100000" style="width: 100%" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item label="最大后端数">
              <el-input-number v-model="quotaForm.max_backends" :min="0" :max="1000" style="width: 100%" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="最大 API Key 数">
              <el-input-number v-model="quotaForm.max_api_keys" :min="0" :max="1000" style="width: 100%" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-divider />
        <el-descriptions :column="2" size="small" border>
          <el-descriptions-item label="今日请求">{{ quotaForm.used_today_requests ?? 0 }}</el-descriptions-item>
          <el-descriptions-item label="今日 Token">{{ quotaForm.used_today_tokens ?? 0 }}</el-descriptions-item>
          <el-descriptions-item label="本月请求">{{ quotaForm.used_month_requests ?? 0 }}</el-descriptions-item>
          <el-descriptions-item label="本月 Token">{{ quotaForm.used_month_tokens ?? 0 }}</el-descriptions-item>
        </el-descriptions>
      </el-form>
      <template #footer>
        <el-button @click="quotaVisible = false">取消</el-button>
        <el-button type="primary" :loading="quotaSubmitting" @click="handleQuotaSubmit">
          保存配额
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Edit, Delete, Setting, Refresh } from '@element-plus/icons-vue'
import {
  listTenants,
  updateTenant,
  deleteTenant,
  getTenantQuota,
  updateTenantQuota,
  type Tenant,
  type TenantQuota
} from '@/api/tenant'

const loading = ref(false)
const tenants = ref<Tenant[]>([])

const dialogVisible = ref(false)
const submitting = ref(false)
const currentTenant = ref<Tenant | null>(null)

const formRef = ref()
const form = ref({
  id: '',
  name: '',
  description: '',
  status: 'active' as 'active' | 'suspended'
})

const rules = {
  name: [{ required: true, message: '请输入名称', trigger: 'blur' }]
}

const quotaVisible = ref(false)
const quotaSubmitting = ref(false)
const quotaForm = ref<TenantQuota>({
  tenant_id: '',
  daily_token_limit: 0,
  monthly_token_limit: 0,
  daily_request_limit: 0,
  monthly_request_limit: 0,
  max_backends: 0,
  max_api_keys: 0
})

const loadTenants = async () => {
  loading.value = true
  try {
    const data = await listTenants()
    tenants.value = Array.isArray(data) ? data : []
  } catch (error) {
    ElMessage.error('加载租户失败')
    console.error(error)
  } finally {
    loading.value = false
  }
}

const getStatusType = (status: string) => {
  switch (status) {
    case 'active': return 'success'
    case 'suspended': return 'warning'
    case 'deleted': return 'danger'
    default: return 'info'
  }
}

const getStatusLabel = (status: string) => {
  switch (status) {
    case 'active': return '活跃'
    case 'suspended': return '暂停'
    case 'deleted': return '已删除'
    default: return status
  }
}

function calcPercent(used?: number, limit?: number): number {
  if (!limit || limit <= 0) return 0
  return Math.min(Math.round(((used ?? 0) / limit) * 100), 100)
}

function progressStatus(pct: number): 'success' | 'warning' | 'exception' {
  if (pct >= 90) return 'exception'
  if (pct >= 70) return 'warning'
  return 'success'
}

const formatNumber = (n?: number) => {
  if (n == null) return '0'
  if (n >= 1000000) return (n / 1000000).toFixed(1) + 'M'
  if (n >= 1000) return (n / 1000).toFixed(1) + 'K'
  return String(n)
}

const formatDate = (date?: string) => {
  if (!date) return '-'
  return new Date(date).toLocaleString()
}

const handleEdit = (row: Tenant) => {
  currentTenant.value = row
  form.value = {
    id: row.id,
    name: row.name,
    description: row.description || '',
    status: row.status === 'suspended' ? 'suspended' : 'active'
  }
  dialogVisible.value = true
}

const handleSubmit = async () => {
  const valid = await formRef.value?.validate()
  if (!valid) return

  submitting.value = true
  try {
    await updateTenant(form.value.id, {
      name: form.value.name,
      description: form.value.description,
      status: form.value.status
    })
    ElMessage.success('更新成功')
    dialogVisible.value = false
    loadTenants()
  } catch (error) {
    ElMessage.error('操作失败')
    console.error(error)
  } finally {
    submitting.value = false
  }
}

const handleDelete = async (row: Tenant) => {
  try {
    await ElMessageBox.confirm('确定要删除这个租户吗？相关数据将被清除。', '提示', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })

    await deleteTenant(row.id)
    ElMessage.success('删除成功')
    loadTenants()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('删除失败')
      console.error(error)
    }
  }
}

const handleQuota = async (row: Tenant) => {
  currentTenant.value = row
  quotaVisible.value = true
  try {
    const quota = await getTenantQuota(row.id)
    quotaForm.value = {
      tenant_id: row.id,
      daily_token_limit: 0,
      monthly_token_limit: 0,
      daily_request_limit: 0,
      monthly_request_limit: 0,
      max_backends: 0,
      max_api_keys: 0,
      ...(quota && typeof quota === 'object' ? quota : {})
    }
  } catch {
    quotaForm.value = {
      tenant_id: row.id,
      daily_token_limit: 0,
      monthly_token_limit: 0,
      daily_request_limit: 0,
      monthly_request_limit: 0,
      max_backends: 0,
      max_api_keys: 0
    }
  }
}

const handleQuotaSubmit = async () => {
  if (!currentTenant.value) return

  quotaSubmitting.value = true
  try {
    await updateTenantQuota(currentTenant.value.id, {
      daily_token_limit: quotaForm.value.daily_token_limit,
      monthly_token_limit: quotaForm.value.monthly_token_limit,
      daily_request_limit: quotaForm.value.daily_request_limit,
      monthly_request_limit: quotaForm.value.monthly_request_limit,
      max_backends: quotaForm.value.max_backends,
      max_api_keys: quotaForm.value.max_api_keys
    })
    ElMessage.success('配额更新成功')
    quotaVisible.value = false
  } catch (error) {
    ElMessage.error('配额更新失败')
    console.error(error)
  } finally {
    quotaSubmitting.value = false
  }
}

onMounted(() => {
  loadTenants()
})
</script>

<style scoped>
.tenant-list {
  padding: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.quota-mini {
  display: flex;
  flex-direction: column;
  gap: 2px;
  align-items: center;
}

.quota-mini-text {
  font-size: 0.75rem;
  color: #909399;
  white-space: nowrap;
}
</style>
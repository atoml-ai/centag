<template>
  <div class="virtual-keys-page">
    <div class="header-with-toolbar">
      <div class="header-left">
        <h1 class="page-title">虚拟密钥管理</h1>
        <p class="page-description">管理所有 API Key 的预算、速率限制和模型白名单</p>
      </div>
      <div class="toolbar-actions">
        <el-select v-model="pageSize" style="width: 100px" @change="loadKeys">
          <el-option label="20条/页" :value="20" />
          <el-option label="50条/页" :value="50" />
          <el-option label="100条/页" :value="100" />
        </el-select>
        <el-button :loading="loading" @click="loadKeys">
          <el-icon><Refresh /></el-icon>刷新
        </el-button>
        <el-button type="primary" @click="openCreateDialog">
          <el-icon><Plus /></el-icon>新建密钥
        </el-button>
      </div>
    </div>

    <el-card class="table-card" shadow="never" v-loading="loading">
      <el-table :data="keys" stripe size="large" empty-text="暂无 API Key 数据">
        <el-table-column label="ID" prop="id" width="56" align="center" />
        <el-table-column label="名称" prop="name" min-width="120" />
        <el-table-column label="密钥前缀" prop="key_prefix" width="140" />
        <el-table-column label="状态" width="70" align="center">
          <template #default="{ row }">
            <el-tag :type="row.enabled ? 'success' : 'info'" size="small" effect="plain">
              {{ row.enabled ? '启用' : '禁用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="预算 ($)" width="90" align="right">
          <template #default="{ row }">
            <span :class="{ 'text-danger': row.used_usd >= row.budget_usd && row.budget_usd > 0 }">
              {{ row.used_usd.toFixed(4) }} / {{ row.budget_usd === 0 ? '∞' : row.budget_usd.toFixed(2) }}
            </span>
          </template>
        </el-table-column>
        <el-table-column label="速率限制" width="130" align="center">
          <template #default="{ row }">
            <span class="text-muted" v-if="row.rate_limit_rpm === 0 && row.rate_limit_tpm === 0">无限制</span>
            <span v-else>
              <el-tag size="small" effect="plain">{{ row.rate_limit_rpm }} RPM</el-tag>
              <el-tag size="small" effect="plain" style="margin-left:4px">{{ row.rate_limit_tpm }} TPM</el-tag>
            </span>
          </template>
        </el-table-column>
        <el-table-column label="模型白名单" min-width="130">
          <template #default="{ row }">
            <span class="text-muted" v-if="!row.model_whitelist || row.model_whitelist === '*'">全部允许</span>
            <el-tooltip v-else :content="row.model_whitelist" placement="top">
              <el-tag size="small" type="info" effect="plain">
                {{ row.model_whitelist.length > 30 ? row.model_whitelist.slice(0, 30) + '…' : row.model_whitelist }}
              </el-tag>
            </el-tooltip>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" prop="created_at" width="150" />
        <el-table-column label="操作" width="100" align="center" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link @click="openEditDialog(row)">
              <el-icon><Edit /></el-icon>
            </el-button>
            <el-button type="primary" link @click="viewFullKey(row)">
              <el-icon><View /></el-icon>
            </el-button>
            <el-button type="primary" link @click="deleteKey(row)">
              <el-icon><Delete /></el-icon>
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="table-footer" v-if="total > 0">
        <el-pagination
          v-model:current-page="currentPage"
          :page-size="pageSize"
          :total="total"
          layout="prev, pager, next, total"
          background
          small
          @current-change="loadKeys"
        />
      </div>
    </el-card>

    <!-- 创建 / 编辑 -->
    <el-dialog
      v-model="dialogVisible"
      :title="editingKey ? '编辑密钥' : '新建密钥'"
      width="560px"
      :close-on-click-modal="false"
      destroy-on-close
    >
      <el-form :model="keyForm" label-width="110px">
        <el-form-item label="用户 ID" prop="user_id" v-if="!editingKey">
          <el-input-number v-model="keyForm.user_id" :min="1" placeholder="所属用户 ID" style="width:200px" />
        </el-form-item>
        <el-form-item label="名称" prop="name">
          <el-input v-model="keyForm.name" placeholder="密钥标识名称" />
        </el-form-item>
        <el-form-item label="过期时间">
          <el-select v-model="keyForm.expires_in" placeholder="永不过期" clearable style="width:200px">
            <el-option label="7 天后" :value="7" />
            <el-option label="30 天后" :value="30" />
            <el-option label="90 天后" :value="90" />
            <el-option label="365 天后" :value="365" />
          </el-select>
        </el-form-item>
        <el-divider content-position="left">预算与速率限制</el-divider>
        <el-form-item label="预算 ($)">
          <el-input-number v-model="keyForm.budget_usd" :min="0" :step="1" :precision="2" style="width:200px" />
          <span class="text-muted form-hint">0 表示无限制</span>
        </el-form-item>
        <el-form-item label="速率限制 (RPM)">
          <el-input-number v-model="keyForm.rate_limit_rpm" :min="0" :step="10" style="width:200px" />
          <span class="text-muted form-hint">每分钟请求数，0 表示无限制</span>
        </el-form-item>
        <el-form-item label="速率限制 (TPM)">
          <el-input-number v-model="keyForm.rate_limit_tpm" :min="0" :step="1000" style="width:200px" />
          <span class="text-muted form-hint">每分钟 Token 数，0 表示无限制</span>
        </el-form-item>
        <el-form-item label="模型白名单">
          <el-input v-model="keyForm.model_whitelist" placeholder='留空或 * 表示全部允许；JSON 数组如 ["gpt-4","gpt-3.5-turbo"]' type="textarea" :rows="2" />
        </el-form-item>
        <el-form-item label="启用" v-if="editingKey">
          <el-switch v-model="keyForm.enabled" active-text="启用" inactive-text="禁用" inline-prompt />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveKey">
          {{ editingKey ? '保存修改' : '创建密钥' }}
        </el-button>
      </template>
    </el-dialog>

    <!-- 查看完整密钥 -->
    <el-dialog
      v-model="fullKeyVisible"
      title="完整密钥信息"
      width="600px"
      :close-on-click-modal="false"
      destroy-on-close
    >
      <el-alert type="warning" :closable="false" style="margin-bottom: 20px">
        这是该密钥唯一一次明文展示，关闭后将无法再次查看完整密钥（除非服务端已配置加密存储）。
      </el-alert>
      <el-descriptions :column="1" border>
        <el-descriptions-item label="名称">{{ fullKeyInfo?.name }}</el-descriptions-item>
        <el-descriptions-item label="完整密钥">
          <el-input :model-value="fullKeyInfo?.full_key || ''" readonly>
            <template #suffix>
              <el-button link @click="copyFullKey">复制</el-button>
            </template>
          </el-input>
        </el-descriptions-item>
        <el-descriptions-item label="密钥前缀">{{ fullKeyInfo?.key_prefix }}</el-descriptions-item>
        <el-descriptions-item label="状态">
          <el-tag :type="fullKeyInfo?.enabled ? 'success' : 'info'" size="small">
            {{ fullKeyInfo?.enabled ? '启用' : '禁用' }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="预算">
          {{ fullKeyInfo ? `已用 $${fullKeyInfo.used_usd.toFixed(4)} / 预算 $${fullKeyInfo.budget_usd === 0 ? '∞' : fullKeyInfo.budget_usd.toFixed(2)}` : '' }}
        </el-descriptions-item>
        <el-descriptions-item label="速率限制">
          {{ fullKeyInfo ? `${fullKeyInfo.rate_limit_rpm} RPM / ${fullKeyInfo.rate_limit_tpm} TPM` : '' }}
        </el-descriptions-item>
        <el-descriptions-item label="模型白名单">{{ fullKeyInfo?.model_whitelist || '*' }}</el-descriptions-item>
        <el-descriptions-item label="创建时间">{{ fullKeyInfo?.created_at }}</el-descriptions-item>
      </el-descriptions>
      <template #footer>
        <el-button @click="fullKeyVisible = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, Plus, Edit, Delete, View } from '@element-plus/icons-vue'
import type { APIKey, APIKeyWithFull } from '@/api/user'
import { listAllAPIKeys, createAdminAPIKey, updateAdminAPIKey, deleteAdminAPIKey } from '@/api/api-keys'

const keys = ref<APIKey[]>([])
const loading = ref(false)
const currentPage = ref(1)
const pageSize = ref(20)
const total = ref(0)

onMounted(loadKeys)

async function loadKeys() {
  loading.value = true
  try {
    const res = await listAllAPIKeys((currentPage.value - 1) * pageSize.value, pageSize.value)
    keys.value = res.keys
    total.value = res.total
  } catch (e: any) {
    ElMessage.error(e.message || '获取密钥列表失败')
  } finally {
    loading.value = false
  }
}

// ── Create / Edit ─────────────────────────────────────────────────────────────
const dialogVisible = ref(false)
const editingKey = ref<APIKey | null>(null)
const saving = ref(false)
const keyForm = reactive({
  user_id: 1,
  name: '',
  expires_in: undefined as number | undefined,
  budget_usd: 0,
  rate_limit_rpm: 0,
  rate_limit_tpm: 0,
  model_whitelist: '',
  enabled: true,
})

function openCreateDialog() {
  editingKey.value = null
  Object.assign(keyForm, { user_id: 1, name: '', expires_in: undefined, budget_usd: 0, rate_limit_rpm: 0, rate_limit_tpm: 0, model_whitelist: '', enabled: true })
  dialogVisible.value = true
}

function openEditDialog(key: APIKey) {
  editingKey.value = key
  Object.assign(keyForm, {
    user_id: 1,
    name: key.name,
    expires_in: undefined,
    budget_usd: key.budget_usd,
    rate_limit_rpm: key.rate_limit_rpm,
    rate_limit_tpm: key.rate_limit_tpm,
    model_whitelist: key.model_whitelist,
    enabled: key.enabled,
  })
  dialogVisible.value = true
}

async function saveKey() {
  saving.value = true
  try {
    if (editingKey.value) {
      await updateAdminAPIKey(editingKey.value.id, {
        name: keyForm.name || undefined,
        enabled: keyForm.enabled,
        budget_usd: keyForm.budget_usd,
        rate_limit_rpm: keyForm.rate_limit_rpm,
        rate_limit_tpm: keyForm.rate_limit_tpm,
        model_whitelist: keyForm.model_whitelist || '*',
      })
      ElMessage.success('密钥已更新')
    } else {
      await createAdminAPIKey({
        user_id: keyForm.user_id,
        name: keyForm.name,
        expires_in: keyForm.expires_in,
        budget_usd: keyForm.budget_usd,
        rate_limit_rpm: keyForm.rate_limit_rpm,
        rate_limit_tpm: keyForm.rate_limit_tpm,
        model_whitelist: keyForm.model_whitelist || '*',
      })
      ElMessage.success('密钥创建成功')
    }
    dialogVisible.value = false
    loadKeys()
  } catch (e: any) {
    if (e?.message) ElMessage.error(e.message)
  } finally {
    saving.value = false
  }
}

// ── Delete ────────────────────────────────────────────────────────────────────
async function deleteKey(key: APIKey) {
  try {
    await ElMessageBox.confirm(`确定删除密钥「${key.name}」(ID: ${key.id})？此操作不可撤销。`, '删除确认', {
      confirmButtonText: '删除', cancelButtonText: '取消', type: 'warning',
    })
    await deleteAdminAPIKey(key.id)
    ElMessage.success('密钥已删除')
    loadKeys()
  } catch (e: any) {
    if (e !== 'cancel' && e?.message) ElMessage.error(e.message)
  }
}

// ── View Full Key ─────────────────────────────────────────────────────────────
const fullKeyVisible = ref(false)
const fullKeyInfo = ref<APIKeyWithFull | null>(null)

async function viewFullKey(key: APIKey) {
  try {
    const detail = await import('@/api/user').then(m => m.getAPIKey(key.id))
    fullKeyInfo.value = { ...key, ...detail, full_key: detail.full_key || '' }
    fullKeyVisible.value = true
  } catch (e: any) {
    ElMessage.error(e.message || '获取密钥详情失败')
  }
}

function copyFullKey() {
  if (fullKeyInfo.value?.full_key) {
    navigator.clipboard.writeText(fullKeyInfo.value.full_key)
    ElMessage.success('已复制到剪贴板')
  }
}
</script>

<style scoped>
.header-with-toolbar {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  margin-bottom: var(--spacing-lg);
  flex-wrap: wrap;
  gap: var(--spacing-md);
}
.header-left { flex: 1; min-width: 0; }
.toolbar-actions {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  flex-wrap: wrap;
}
.table-card { width: 100%; }
.text-danger { color: var(--el-color-danger); }
.text-muted { color: var(--el-text-color-secondary); font-size: 12px; }
.form-hint { margin-left: 8px; }
.table-footer {
  margin-top: 16px;
  padding-top: 12px;
  border-top: 1px solid var(--el-border-color-lighter);
  display: flex;
  justify-content: center;
}
@media (max-width: 768px) {
  .toolbar-actions {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: var(--spacing-sm);
  }
  .toolbar-actions :deep(.el-input),
  .toolbar-actions :deep(.el-select),
  .toolbar-actions :deep(.el-button) { width: 100% !important; }
}
</style>

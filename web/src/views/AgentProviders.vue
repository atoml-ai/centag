<template>
  <div class="agent-providers-page">
    <div class="page-header">
      <h1 class="page-title">
        <el-icon><Connection /></el-icon>
        Agent 供应商管理
      </h1>
      <p class="page-description">管理 Agent 工具的后端和流水线路由配置</p>
    </div>

    <el-card>
      <template #header>
        <div class="card-header">
          <span>供应商配置列表</span>
          <el-button type="primary" @click="showCreateDialog">
            <el-icon class="el-icon--left"><Plus /></el-icon>
            新增配置
          </el-button>
        </div>
      </template>

      <el-table :data="providers" v-loading="loading" stripe>
        <el-table-column prop="agent_type" label="Agent 类型" width="150" />
        <el-table-column prop="display_name" label="显示名称" width="150" />
        <el-table-column prop="backend_id" label="后端 ID" width="180">
          <template #default="{ row }">
            <el-tag v-if="row.backend_id" type="success">{{ row.backend_id }}</el-tag>
            <span v-else class="text-muted">默认</span>
          </template>
        </el-table-column>
        <el-table-column prop="pipeline_id" label="流水线 ID" width="180">
          <template #default="{ row }">
            <el-tag v-if="row.pipeline_id" type="warning">{{ row.pipeline_id }}</el-tag>
            <span v-else class="text-muted">默认</span>
          </template>
        </el-table-column>
        <el-table-column prop="model" label="模型覆盖" width="150">
          <template #default="{ row }">
            <span v-if="row.model">{{ row.model }}</span>
            <span v-else class="text-muted">-</span>
          </template>
        </el-table-column>
        <el-table-column prop="enabled" label="状态" width="80">
          <template #default="{ row }">
            <el-tag :type="row.enabled ? 'success' : 'info'" size="small">
              {{ row.enabled ? '启用' : '禁用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="160" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click="showEditDialog(row)">编辑</el-button>
            <el-popconfirm title="确定删除此配置？" @confirm="deleteProvider(row.id)">
              <template #reference>
                <el-button size="small" type="danger">删除</el-button>
              </template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 创建/编辑对话框 -->
    <el-dialog
      v-model="dialogVisible"
      :title="isEditing ? '编辑供应商配置' : '新增供应商配置'"
      width="600px"
    >
      <el-form :model="form" label-width="120px">
        <el-form-item label="Agent 类型" required>
          <el-select v-model="form.agent_type" :disabled="isEditing" placeholder="选择 Agent 类型">
            <el-option v-for="at in agentTypes" :key="at.type" :label="at.display_name" :value="at.type" />
          </el-select>
        </el-form-item>
        <el-form-item label="显示名称">
          <el-input v-model="form.display_name" placeholder="可选的显示名称" />
        </el-form-item>
        <el-form-item label="后端 ID">
          <el-select v-model="form.backend_id" clearable placeholder="留空使用默认后端">
            <el-option v-for="be in backends" :key="be.id" :label="be.name" :value="be.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="流水线 ID">
          <el-select v-model="form.pipeline_id" clearable placeholder="留空使用默认流水线">
            <el-option v-for="pl in pipelines" :key="pl.id" :label="pl.name || pl.id" :value="pl.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="模型覆盖">
          <el-input v-model="form.model" placeholder="留空使用后端默认模型" />
        </el-form-item>
        <el-form-item label="API Key 覆盖">
          <el-input v-model="form.api_key" type="password" show-password placeholder="留空使用后端 API Key" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="form.description" type="textarea" :rows="2" />
        </el-form-item>
        <el-form-item label="启用">
          <el-switch v-model="form.enabled" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveProvider">
          {{ isEditing ? '保存' : '创建' }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Connection, Plus } from '@element-plus/icons-vue'
import api from '@/api'

interface AgentProviderConfig {
  id: string
  agent_type: string
  display_name: string
  backend_id: string
  pipeline_id: string
  model: string
  api_key: string
  enabled: boolean
  description: string
}

const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const isEditing = ref(false)
const providers = ref<AgentProviderConfig[]>([])
const agentTypes = ref<Array<{ type: string; display_name: string }>>([])
const backends = ref<Array<{ id: string; name: string }>>([])
const pipelines = ref<Array<{ id: string; name: string }>>([])

const defaultForm = (): AgentProviderConfig => ({
  id: '',
  agent_type: '',
  display_name: '',
  backend_id: '',
  pipeline_id: '',
  model: '',
  api_key: '',
  enabled: true,
  description: ''
})
const form = ref<AgentProviderConfig>(defaultForm())

async function loadProviders() {
  loading.value = true
  try {
    const res: any = await api.get('/api/v1/agent-providers')
    providers.value = res.agent_providers || []
  } catch (e: any) {
    ElMessage.error('加载供应商配置失败：' + e.message)
  } finally {
    loading.value = false
  }
}

async function loadAgentTypes() {
  try {
    const res: any = await api.get('/api/v1/agent/types')
    agentTypes.value = res.agent_types || []
  } catch {}
}

async function loadBackends() {
  try {
    const res: any = await api.get('/api/v1/backends', {
      params: { _ts: Date.now() },
    })
    backends.value = (Array.isArray(res) ? res : []).filter((b: any) => b.enabled)
  } catch {}
}

async function loadPipelines() {
  try {
    const res: any = await api.get('/api/v1/pipelines')
    pipelines.value = (Array.isArray(res) ? res : res.data || []).filter((p: any) => p.enabled !== false)
  } catch {}
}

function showCreateDialog() {
  loadBackends()
  isEditing.value = false
  form.value = defaultForm()
  dialogVisible.value = true
}

function showEditDialog(row: AgentProviderConfig) {
  loadBackends()
  isEditing.value = true
  form.value = { ...row, api_key: '' }
  dialogVisible.value = true
}

async function saveProvider() {
  if (!form.value.agent_type) {
    ElMessage.warning('请选择 Agent 类型')
    return
  }
  saving.value = true
  try {
    if (isEditing.value) {
      await api.put(`/api/v1/agent-providers/${form.value.id}`, form.value)
      ElMessage.success('保存成功')
    } else {
      form.value.id = form.value.agent_type
      await api.post('/api/v1/agent-providers', form.value)
      ElMessage.success('创建成功')
    }
    dialogVisible.value = false
    loadProviders()
  } catch (e: any) {
    ElMessage.error('操作失败：' + e.message)
  } finally {
    saving.value = false
  }
}

async function deleteProvider(id: string) {
  try {
    await api.delete(`/api/v1/agent-providers/${id}`)
    ElMessage.success('删除成功')
    loadProviders()
  } catch (e: any) {
    ElMessage.error('删除失败：' + e.message)
  }
}

onMounted(() => {
  loadProviders()
  loadAgentTypes()
  loadBackends()
  loadPipelines()
})
</script>

<style scoped>
.agent-providers-page {
  padding: 24px;
}

.page-header {
  margin-bottom: 24px;
}

.page-title {
  font-size: 1.5rem;
  font-weight: 600;
  margin: 0;
  display: flex;
  align-items: center;
  gap: 8px;
}

.page-description {
  color: #6b7280;
  font-size: 0.875rem;
  margin: 4px 0 0;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.text-muted {
  color: #909399;
  font-size: 0.85rem;
}
</style>

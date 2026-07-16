<template>
  <div class="data-stores">
    <div class="header-with-toolbar">
      <div class="header-left">
        <h1 class="page-title">数据存储管理</h1>
        <p class="page-description">配置知识库/数据存储接口，关联底层存储并管理默认存储</p>
      </div>
      <div class="toolbar-actions">
        <el-input
          v-model="searchText"
          placeholder="搜索数据存储..."
          clearable
          style="width: 200px"
        >
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>
        <el-select v-model="filterType" placeholder="类型筛选" clearable style="width: 120px">
          <el-option label="全部" value="" />
          <el-option label="知识库" value="knowledge_base" />
          <el-option label="向量存储" value="vector" />
          <el-option label="文档存储" value="document" />
          <el-option label="Key-Value" value="kv" />
          <el-option label="全文搜索" value="search" />
          <el-option label="图数据库" value="graph" />
          <el-option label="自定义" value="custom" />
        </el-select>
        <el-button :loading="loading" @click="load">
          <el-icon><Refresh /></el-icon>
          刷新
        </el-button>
        <el-button :loading="loading" @click="load">
          <el-icon><CircleCheck /></el-icon>
          检查健康状态
        </el-button>
        <el-button type="primary" @click="openCreate">
          <el-icon><Plus /></el-icon>
          添加数据存储
        </el-button>
      </div>
    </div>

    <div class="content-wrapper">
      <el-card class="table-card" v-loading="loading">
        <el-table :data="filteredDataStores" stripe size="large">
          <el-table-column prop="name" label="名称" min-width="150">
            <template #default="{ row }">
              <div class="name-cell">
                <span class="name-title">{{ row.name }}</span>
                <span class="name-subtitle" v-if="row.description">{{ row.description }}</span>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="类型" width="160" align="center">
            <template #default="{ row }">
              <el-tag :type="getTypeColor(row.type)" size="small" effect="plain">
                {{ getTypeLabel(row.type) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="关联存储" width="160" align="center" show-overflow-tooltip>
            <template #default="{ row }">
              <el-tag v-if="row.storage_name" type="primary" size="small" effect="plain">
                {{ row.storage_name }}
              </el-tag>
              <span v-else class="text-muted">-</span>
            </template>
          </el-table-column>
          <el-table-column label="启用状态" width="90" align="center">
            <template #default="{ row }">
              <el-switch
                :model-value="row.enabled"
                @change="toggleStatus(row)"
                active-color="#10b981"
              />
            </template>
          </el-table-column>
          <el-table-column label="默认" width="150" align="center">
            <template #default="{ row }">
              <el-tag v-if="row.is_default" type="success" size="small" effect="plain">
                是
              </el-tag>
              <span v-else class="text-muted">-</span>
            </template>
          </el-table-column>
          <el-table-column label="健康状态" width="200" align="center">
            <template #default="{ row }">
              <span v-if="row.healthy === undefined" class="health-badge health-badge--info">
                未检查
              </span>
              <span v-else-if="row.healthy" class="health-badge health-badge--success">
                <el-icon><SuccessFilled /></el-icon>正常
              </span>
              <el-tooltip v-else effect="dark" placement="top">
                <template #content>
                  <div class="error-tooltip">{{ row.error || '未知错误' }}</div>
                </template>
                <span class="health-badge health-badge--danger" style="cursor: help;">
                  <el-icon><CircleCloseFilled /></el-icon>异常
                </span>
              </el-tooltip>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="80" align="center" fixed="right">
            <template #default="{ row }">
              <el-dropdown trigger="click" @command="(command) => handleCommand(command, row)">
                <el-button type="primary" link>
                  <el-icon><MoreFilled /></el-icon>
                </el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item :command="'toggle'">
                      <el-icon><Switch /></el-icon>
                      {{ row.enabled ? '禁用' : '启用' }}
                    </el-dropdown-item>
                    <el-dropdown-item v-if="!row.is_default" :command="'setDefault'">
                      <el-icon><Check /></el-icon>
                      设为默认
                    </el-dropdown-item>
                    <el-dropdown-item v-if="row.is_default" :command="'removeDefault'">
                      <el-icon><Close /></el-icon>
                      取消默认
                    </el-dropdown-item>
                    <el-dropdown-item :command="'test'">
                      <el-icon><Link /></el-icon>
                      测试
                    </el-dropdown-item>
                    <el-dropdown-item :command="'edit'">
                      <el-icon><Edit /></el-icon>
                      编辑
                    </el-dropdown-item>
                    <el-dropdown-item :command="'delete'" divided>
                      <el-icon><Delete /></el-icon>
                      删除
                    </el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </template>
          </el-table-column>
        </el-table>

        <el-empty
          v-if="!loading && filteredDataStores.length === 0"
          description="暂无数据存储配置"
          :image-size="120"
        />
      </el-card>

      <!-- 编辑/创建对话框 -->
      <el-dialog
        v-model="editing"
        :title="isCreate ? '添加数据存储' : '编辑数据存储'"
        width="600px"
        @close="resetForm"
      >
        <el-form label-width="100px" :model="form" :rules="rules" ref="formRef">
          <el-form-item label="名称" prop="name">
            <el-input v-model="form.name" placeholder="请输入数据存储名称" />
          </el-form-item>
          <el-form-item label="类型" prop="type">
            <el-select v-model="form.type" style="width: 100%" placeholder="请选择类型">
              <el-option label="知识库" value="knowledge_base" />
              <el-option label="向量存储" value="vector" />
              <el-option label="文档存储" value="document" />
              <el-option label="Key-Value" value="kv" />
              <el-option label="全文搜索" value="search" />
              <el-option label="图数据库" value="graph" />
              <el-option label="自定义" value="custom" />
            </el-select>
          </el-form-item>
          <el-form-item label="关联存储" prop="storage_name">
            <el-select v-model="form.storage_name" style="width: 100%" placeholder="选择底层存储后端">
              <el-option
                v-for="s in availableStorages"
                :key="s.name"
                :label="`${s.name} (${s.type})`"
                :value="s.name"
                :disabled="!s.enabled"
              />
            </el-select>
            <div class="form-tip">选择此前已在"存储管理"中配置好的后端</div>
          </el-form-item>
          <el-form-item label="额外配置">
            <el-input
              v-model="form.configText"
              type="textarea"
              :rows="4"
              placeholder='{"key": "value"}'
            />
            <div class="form-tip">JSON格式的可选配置参数</div>
          </el-form-item>
          <el-form-item label="描述">
            <el-input
              v-model="form.description"
              type="textarea"
              :rows="2"
              placeholder="可选，描述此数据存储的用途"
            />
          </el-form-item>
          <el-form-item label="启用状态">
            <el-switch v-model="form.enabled" />
          </el-form-item>
          <el-form-item label="设为默认">
            <el-switch v-model="form.is_default" />
          </el-form-item>
        </el-form>
        <template #footer>
          <div style="display: flex; justify-content: space-between; align-items: center;">
            <el-button :loading="testing" @click="handleTestConnection">
              <el-icon><Link /></el-icon>
              测试连接
            </el-button>
            <div>
              <el-button @click="editing = false">取消</el-button>
              <el-button type="primary" :loading="saving" @click="save">
                保存
              </el-button>
            </div>
          </div>
        </template>
      </el-dialog>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Search,
  Refresh,
  Plus,
  Edit,
  Delete,
  Switch,
  Check,
  Close,
  Link,
  MoreFilled,
  SuccessFilled,
  CircleCloseFilled,
  CircleCheck
} from '@element-plus/icons-vue'
import {
  getDataStores,
  addDataStore,
  updateDataStore,
  deleteDataStore,
  toggleDataStore,
  setDefaultDataStore,
  removeDefaultDataStore,
  testDataStore
} from '@/api'

const loading = ref(false)
const saving = ref(false)
const testing = ref(false)
const searchText = ref('')
const filterType = ref('')
const list = ref<any[]>([])
const availableStorages = ref<any[]>([])

const editing = ref(false)
const isCreate = ref(false)
const form = ref<any>({})
const formRef = ref()

const rules = {
  name: [{ required: true, message: '请输入数据存储名称', trigger: 'blur' }],
  type: [{ required: true, message: '请选择类型', trigger: 'change' }],
  storage_name: [{ required: true, message: '请选择关联存储', trigger: 'change' }]
}

const filteredDataStores = computed(() => {
  let result = list.value
  if (searchText.value) {
    const text = searchText.value.toLowerCase()
    result = result.filter(
      (item) =>
        item.name?.toLowerCase().includes(text) ||
        item.storage_name?.toLowerCase().includes(text) ||
        (item.description && item.description.toLowerCase().includes(text))
    )
  }
  if (filterType.value) {
    result = result.filter((item) => item.type === filterType.value)
  }
  return result
})

function getTypeLabel(type: string) {
  const typeMap: Record<string, string> = {
    knowledge_base: '知识库',
    vector: '向量存储',
    document: '文档存储',
    kv: 'Key-Value',
    search: '全文搜索',
    graph: '图数据库',
    custom: '自定义'
  }
  return typeMap[type] || type
}

function getTypeColor(type: string) {
  const colorMap: Record<string, string> = {
    knowledge_base: 'primary',
    vector: 'success',
    document: 'warning',
    kv: 'info',
    search: 'danger',
    graph: 'primary',
    custom: ''
  }
  return colorMap[type] || ''
}

async function load() {
  loading.value = true
  try {
    const res = await getDataStores()
    const data = res?.data || res
    if (data && typeof data === 'object') {
      list.value = Array.isArray(data.data_stores) ? data.data_stores : []
      availableStorages.value = Array.isArray(data.available_storages) ? data.available_storages : []
    } else {
      list.value = Array.isArray(data) ? data : []
    }
  } catch (error: any) {
    ElMessage.error('加载失败: ' + (error.message || '未知错误'))
    list.value = []
  } finally {
    loading.value = false
  }
}

function openCreate() {
  isCreate.value = true
  form.value = {
    name: '',
    type: 'knowledge_base',
    storage_name: '',
    description: '',
    enabled: true,
    is_default: false,
    config: {},
    configText: '{}'
  }
  editing.value = true
}

function openEdit(row: any) {
  isCreate.value = false
  const config = row.config || {}
  form.value = {
    ...row,
    configText: typeof config === 'string' ? config : JSON.stringify(config, null, 2),
    config: config
  }
  editing.value = true
}

async function save() {
  try {
    await formRef.value?.validate()
  } catch {
    return
  }

  saving.value = true
  try {
    let parsedConfig: any = {}
    if (form.value.configText) {
      try {
        parsedConfig = JSON.parse(form.value.configText)
      } catch {
        ElMessage.warning('额外配置必须是有效的JSON格式')
        saving.value = false
        return
      }
    }

    const payload = {
      name: form.value.name,
      type: form.value.type,
      storage_name: form.value.storage_name,
      enabled: form.value.enabled,
      description: form.value.description || '',
      config: parsedConfig
    }

    if (isCreate.value) {
      await addDataStore(payload)
      ElMessage.success('添加成功')
    } else {
      await updateDataStore(payload)
      ElMessage.success('更新成功')
    }

    if (form.value.is_default) {
      try {
        await setDefaultDataStore(form.value.name)
      } catch (error: any) {
        ElMessage.warning('保存成功但设置默认失败: ' + (error.message || '未知错误'))
      }
    }

    editing.value = false
    await load()
  } catch (error: any) {
    ElMessage.error('保存失败: ' + (error.message || '未知错误'))
  } finally {
    saving.value = false
  }
}

function resetForm() {
  form.value = {}
  formRef.value?.resetFields()
}

async function toggleStatus(row: any) {
  try {
    await toggleDataStore(row.name, !row.enabled)
    ElMessage.success(row.enabled ? '已禁用' : '已启用')
    await load()
  } catch (error: any) {
    ElMessage.error('操作失败: ' + (error.message || '未知错误'))
  }
}

async function handleDelete(row: any) {
  try {
    await ElMessageBox.confirm(`确定要删除数据存储 "${row.name}" 吗?`, '确认删除', {
      type: 'warning'
    })
    await deleteDataStore(row.name)
    ElMessage.success('删除成功')
    await load()
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error('删除失败: ' + (error.message || '未知错误'))
    }
  }
}

async function handleSetDefault(row: any) {
  try {
    await setDefaultDataStore(row.name)
    ElMessage.success(`已将 "${row.name}" 设置为默认数据存储`)
    await load()
  } catch (error: any) {
    ElMessage.error('操作失败: ' + (error.message || '未知错误'))
  }
}

async function handleRemoveDefault(row: any) {
  try {
    await removeDefaultDataStore(row.name)
    ElMessage.success(`已取消 "${row.name}" 的默认数据存储`)
    await load()
  } catch (error: any) {
    ElMessage.error('操作失败: ' + (error.message || '未知错误'))
  }
}

async function testConnectionForRow(row: any) {
  testing.value = true
  try {
    await testDataStore({
      name: row.name,
      type: row.type,
      storage_name: row.storage_name,
      config: row.config || {}
    })
    ElMessage.success(`数据存储 "${row.name}" 连接测试成功`)
  } catch (error: any) {
    ElMessage.error(`连接测试失败: ${error.message || '未知错误'}`)
  } finally {
    testing.value = false
  }
}

async function handleTestConnection() {
  try {
    await formRef.value?.validate()
  } catch {
    ElMessage.warning('请先填写完整信息')
    return
  }

  testing.value = true
  try {
    let parsedConfig: any = {}
    if (form.value.configText) {
      try {
        parsedConfig = JSON.parse(form.value.configText)
      } catch {
        ElMessage.warning('额外配置必须是有效的JSON格式')
        testing.value = false
        return
      }
    }

    await testDataStore({
      name: form.value.name,
      type: form.value.type,
      storage_name: form.value.storage_name,
      config: parsedConfig
    })
    ElMessage.success('连接测试成功')
  } catch (error: any) {
    ElMessage.error(`连接测试失败: ${error.message || '未知错误'}`)
  } finally {
    testing.value = false
  }
}

function handleCommand(command: string, row: any) {
  switch (command) {
    case 'toggle':
      toggleStatus(row)
      break
    case 'setDefault':
      handleSetDefault(row)
      break
    case 'removeDefault':
      handleRemoveDefault(row)
      break
    case 'test':
      testConnectionForRow(row)
      break
    case 'edit':
      openEdit(row)
      break
    case 'delete':
      handleDelete(row)
      break
  }
}

onMounted(() => {
  load()
})
</script>

<style scoped>
.header-with-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: var(--spacing-lg);
  gap: var(--spacing-lg);
}

.header-left {
  flex-shrink: 0;
}

.toolbar-actions {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
}

.content-wrapper {
  width: 100%;
}

.table-card {
  width: 100%;
}

.el-table {
  width: 100%;
}

.health-badge {
  display: inline-flex;
  flex-direction: row;
  align-items: center;
  gap: 4px;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 12px;
  line-height: 20px;
  white-space: nowrap;
}

.health-badge--info {
  color: var(--el-color-info);
  background-color: var(--el-color-info-light-9);
  border: 1px solid var(--el-color-info-light-5);
}

.health-badge--success {
  color: var(--el-color-success);
  background-color: var(--el-color-success-light-9);
  border: 1px solid var(--el-color-success-light-5);
}

.health-badge--danger {
  color: var(--el-color-danger);
  background-color: var(--el-color-danger-light-9);
  border: 1px solid var(--el-color-danger-light-5);
}

.name-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.name-title {
  font-weight: 600;
  color: var(--color-gray-900);
  font-size: 0.9375rem;
}

.name-subtitle {
  font-size: 0.8125rem;
  color: var(--color-gray-500);
  line-height: 1.4;
}

.text-muted {
  color: var(--color-gray-400);
}

.error-tooltip {
  max-width: 400px;
  word-wrap: break-word;
  line-height: 1.5;
}

.unit {
  margin-left: var(--spacing-sm);
  color: var(--color-gray-600);
}

.form-tip {
  font-size: 0.75rem;
  color: var(--color-gray-500);
  margin-top: 4px;
  line-height: 1.4;
}

:deep(.el-dropdown-menu__item) {
  display: flex;
  align-items: center;
  gap: 8px;
}

@media (max-width: 1200px) {
  .toolbar-actions :deep(.el-input) {
    width: 160px !important;
  }
}

@media (max-width: 1024px) {
  .header-with-toolbar {
    flex-direction: column;
    align-items: stretch;
  }

  .toolbar-actions {
    flex-wrap: wrap;
  }

  .toolbar-actions :deep(.el-input),
  .toolbar-actions :deep(.el-select) {
    width: 180px !important;
  }
}

@media (max-width: 768px) {
  .toolbar-actions {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: var(--spacing-sm);
  }

  .toolbar-actions :deep(.el-input),
  .toolbar-actions :deep(.el-select),
  .toolbar-actions :deep(.el-button) {
    width: 100% !important;
  }
}
</style>

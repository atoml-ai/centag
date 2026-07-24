<template>
  <div class="data-stores">
    <div class="header-with-toolbar">
      <div class="header-left">
        <h1 class="page-title">{{ t('dataStores.pageTitle') }}</h1>
        <p class="page-description">{{ t('dataStores.pageDescription') }}</p>
      </div>
      <div class="toolbar-actions">
        <el-input
          v-model="searchText"
          :placeholder="t('dataStores.searchPlaceholder')"
          clearable
          style="width: 200px"
        >
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>
        <el-select v-model="filterType" :placeholder="t('dataStores.filterPlaceholder')" clearable style="width: 120px">
          <el-option :label="t('dataStores.filterOptions.all')" value="" />
          <el-option :label="t('dataStores.filterOptions.knowledgeBase')" value="knowledge_base" />
          <el-option :label="t('dataStores.filterOptions.vector')" value="vector" />
          <el-option :label="t('dataStores.filterOptions.document')" value="document" />
          <el-option :label="t('dataStores.filterOptions.kv')" value="kv" />
          <el-option :label="t('dataStores.filterOptions.search')" value="search" />
          <el-option :label="t('dataStores.filterOptions.graph')" value="graph" />
          <el-option :label="t('dataStores.filterOptions.custom')" value="custom" />
        </el-select>
        <el-button :loading="loading" @click="load">
          <el-icon><Refresh /></el-icon>
          {{ t('dataStores.refresh') }}
        </el-button>
        <el-button :loading="loading" @click="load">
          <el-icon><CircleCheck /></el-icon>
          {{ t('dataStores.healthCheck') }}
        </el-button>
        <el-button type="primary" @click="openCreate">
          <el-icon><Plus /></el-icon>
          {{ t('dataStores.addDataStore') }}
        </el-button>
      </div>
    </div>

    <div class="content-wrapper">
      <el-card class="table-card" v-loading="loading">
        <el-table :data="filteredDataStores" stripe size="large">
          <el-table-column prop="name" :label="t('dataStores.table.name')" min-width="150">
            <template #default="{ row }">
              <div class="name-cell">
                <span class="name-title">{{ row.name }}</span>
                <span class="name-subtitle" v-if="row.description">{{ row.description }}</span>
              </div>
            </template>
          </el-table-column>
          <el-table-column :label="t('dataStores.table.type')" width="160" align="center">
            <template #default="{ row }">
              <el-tag :type="getTypeColor(row.type)" size="small" effect="plain">
                {{ getTypeLabel(row.type) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column :label="t('dataStores.table.linkedStorage')" width="160" align="center" show-overflow-tooltip>
            <template #default="{ row }">
              <el-tag v-if="row.storage_name" type="primary" size="small" effect="plain">
                {{ row.storage_name }}
              </el-tag>
              <span v-else class="text-muted">-</span>
            </template>
          </el-table-column>
          <el-table-column :label="t('dataStores.table.enabledStatus')" width="90" align="center">
            <template #default="{ row }">
              <el-switch
                :model-value="row.enabled"
                @change="toggleStatus(row)"
                active-color="#10b981"
              />
            </template>
          </el-table-column>
          <el-table-column :label="t('dataStores.table.isDefault')" width="150" align="center">
            <template #default="{ row }">
              <el-tag v-if="row.is_default" type="success" size="small" effect="plain">
                {{ t('dataStores.table.yes') }}
              </el-tag>
              <span v-else class="text-muted">-</span>
            </template>
          </el-table-column>
          <el-table-column :label="t('dataStores.table.healthStatus')" width="200" align="center">
            <template #default="{ row }">
              <span v-if="row.healthy === undefined" class="health-badge health-badge--info">
                {{ t('dataStores.table.notChecked') }}
              </span>
              <span v-else-if="row.healthy" class="health-badge health-badge--success">
                <el-icon><SuccessFilled /></el-icon>{{ t('dataStores.table.healthy') }}
              </span>
              <el-tooltip v-else effect="dark" placement="top">
                <template #content>
                  <div class="error-tooltip">{{ row.error || t('dataStores.table.unknownError') }}</div>
                </template>
                <span class="health-badge health-badge--danger" style="cursor: help;">
                  <el-icon><CircleCloseFilled /></el-icon>{{ t('dataStores.table.unhealthy') }}
                </span>
              </el-tooltip>
            </template>
          </el-table-column>
          <el-table-column :label="t('dataStores.table.actions')" width="80" align="center" fixed="right">
            <template #default="{ row }">
              <el-dropdown trigger="click" @command="(command) => handleCommand(command, row)">
                <el-button type="primary" link>
                  <el-icon><MoreFilled /></el-icon>
                </el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item :command="'toggle'">
                      <el-icon><Switch /></el-icon>
                      {{ row.enabled ? t('dataStores.actions.disable') : t('dataStores.actions.enable') }}
                    </el-dropdown-item>
                    <el-dropdown-item v-if="!row.is_default" :command="'setDefault'">
                      <el-icon><Check /></el-icon>
                      {{ t('dataStores.actions.setDefault') }}
                    </el-dropdown-item>
                    <el-dropdown-item v-if="row.is_default" :command="'removeDefault'">
                      <el-icon><Close /></el-icon>
                      {{ t('dataStores.actions.removeDefault') }}
                    </el-dropdown-item>
                    <el-dropdown-item :command="'test'">
                      <el-icon><Link /></el-icon>
                      {{ t('dataStores.actions.test') }}
                    </el-dropdown-item>
                    <el-dropdown-item :command="'edit'">
                      <el-icon><Edit /></el-icon>
                      {{ t('dataStores.actions.edit') }}
                    </el-dropdown-item>
                    <el-dropdown-item :command="'delete'" divided>
                      <el-icon><Delete /></el-icon>
                      {{ t('dataStores.actions.delete') }}
                    </el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </template>
          </el-table-column>
        </el-table>

        <el-empty
          v-if="!loading && filteredDataStores.length === 0"
          :description="t('dataStores.emptyState')"
          :image-size="120"
        />
      </el-card>

      <!-- 编辑/创建对话框 -->
      <el-dialog
        v-model="editing"
        :title="isCreate ? t('dataStores.formDialog.addTitle') : t('dataStores.formDialog.editTitle')"
        width="600px"
        @close="resetForm"
      >
        <el-form label-width="100px" :model="form" :rules="rules" ref="formRef">
          <el-form-item :label="t('dataStores.formDialog.nameLabel')" prop="name">
            <el-input v-model="form.name" :placeholder="t('dataStores.formDialog.namePlaceholder')" />
          </el-form-item>
          <el-form-item :label="t('dataStores.formDialog.typeLabel')" prop="type">
            <el-select v-model="form.type" style="width: 100%" :placeholder="t('dataStores.formDialog.typePlaceholder')">
              <el-option :label="t('dataStores.filterOptions.knowledgeBase')" value="knowledge_base" />
              <el-option :label="t('dataStores.filterOptions.vector')" value="vector" />
              <el-option :label="t('dataStores.filterOptions.document')" value="document" />
              <el-option :label="t('dataStores.filterOptions.kv')" value="kv" />
              <el-option :label="t('dataStores.filterOptions.search')" value="search" />
              <el-option :label="t('dataStores.filterOptions.graph')" value="graph" />
              <el-option :label="t('dataStores.filterOptions.custom')" value="custom" />
            </el-select>
          </el-form-item>
          <el-form-item :label="t('dataStores.formDialog.linkedStorageLabel')" prop="storage_name">
            <el-select v-model="form.storage_name" style="width: 100%" :placeholder="t('dataStores.formDialog.linkedStoragePlaceholder')">
              <el-option
                v-for="s in availableStorages"
                :key="s.name"
                :label="`${s.name} (${s.type})`"
                :value="s.name"
                :disabled="!s.enabled"
              />
            </el-select>
            <div class="form-tip">{{ t('dataStores.formDialog.linkedStorageTip') }}</div>
          </el-form-item>
          <el-form-item :label="t('dataStores.formDialog.extraConfigLabel')">
            <el-input
              v-model="form.configText"
              type="textarea"
              :rows="4"
              :placeholder="t('dataStores.formDialog.extraConfigPlaceholder')"
            />
            <div class="form-tip">{{ t('dataStores.formDialog.extraConfigTip') }}</div>
          </el-form-item>
          <el-form-item :label="t('dataStores.formDialog.descriptionLabel')">
            <el-input
              v-model="form.description"
              type="textarea"
              :rows="2"
              :placeholder="t('dataStores.formDialog.descriptionPlaceholder')"
            />
          </el-form-item>
          <el-form-item :label="t('dataStores.formDialog.enabledLabel')">
            <el-switch v-model="form.enabled" />
          </el-form-item>
          <el-form-item :label="t('dataStores.formDialog.defaultLabel')">
            <el-switch v-model="form.is_default" />
          </el-form-item>
        </el-form>
        <template #footer>
          <div style="display: flex; justify-content: space-between; align-items: center;">
            <el-button :loading="testing" @click="handleTestConnection">
              <el-icon><Link /></el-icon>
              {{ t('dataStores.formDialog.testConnection') }}
            </el-button>
            <div>
              <el-button @click="editing = false">{{ t('dataStores.formDialog.cancel') }}</el-button>
              <el-button type="primary" :loading="saving" @click="save">
                {{ t('dataStores.formDialog.save') }}
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
import { useI18n } from 'vue-i18n'
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

const { t } = useI18n()

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
  name: [{ required: true, message: t('dataStores.validation.nameRequired'), trigger: 'blur' }],
  type: [{ required: true, message: t('dataStores.validation.typeRequired'), trigger: 'change' }],
  storage_name: [{ required: true, message: t('dataStores.validation.storageRequired'), trigger: 'change' }]
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
    knowledge_base: t('dataStores.filterOptions.knowledgeBase'),
    vector: t('dataStores.filterOptions.vector'),
    document: t('dataStores.filterOptions.document'),
    kv: t('dataStores.filterOptions.kv'),
    search: t('dataStores.filterOptions.search'),
    graph: t('dataStores.filterOptions.graph'),
    custom: t('dataStores.filterOptions.custom')
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
    ElMessage.error(t('dataStores.message.loadFailed') + ': ' + (error.message || t('dataStores.message.unknownError')))
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
        ElMessage.warning(t('dataStores.message.invalidJson'))
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
      ElMessage.success(t('dataStores.message.addSuccess'))
    } else {
      await updateDataStore(payload)
      ElMessage.success(t('dataStores.message.updateSuccess'))
    }

    if (form.value.is_default) {
      try {
        await setDefaultDataStore(form.value.name)
      } catch (error: any) {
        ElMessage.warning(t('dataStores.message.setDefaultFailed', { error: error.message || t('dataStores.message.unknownError') }))
      }
    }

    editing.value = false
    await load()
  } catch (error: any) {
    ElMessage.error(t('dataStores.message.saveFailed', { error: error.message || t('dataStores.message.unknownError') }))
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
    ElMessage.success(row.enabled ? t('dataStores.message.disabled') : t('dataStores.message.enabled'))
    await load()
  } catch (error: any) {
    ElMessage.error(t('dataStores.message.operationFailed', { error: error.message || t('dataStores.message.unknownError') }))
  }
}

async function handleDelete(row: any) {
  try {
    await ElMessageBox.confirm(t('dataStores.confirm.deleteMessage', { name: row.name }), t('dataStores.confirm.deleteTitle'), {
      type: 'warning'
    })
    await deleteDataStore(row.name)
    ElMessage.success(t('dataStores.message.deleteSuccess'))
    await load()
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error(t('dataStores.message.deleteFailed', { error: error.message || t('dataStores.message.unknownError') }))
    }
  }
}

async function handleSetDefault(row: any) {
  try {
    await setDefaultDataStore(row.name)
    ElMessage.success(t('dataStores.message.setDefaultSuccess', { name: row.name }))
    await load()
  } catch (error: any) {
    ElMessage.error(t('dataStores.message.operationFailed', { error: error.message || t('dataStores.message.unknownError') }))
  }
}

async function handleRemoveDefault(row: any) {
  try {
    await removeDefaultDataStore(row.name)
    ElMessage.success(t('dataStores.message.removeDefaultSuccess', { name: row.name }))
    await load()
  } catch (error: any) {
    ElMessage.error(t('dataStores.message.operationFailed', { error: error.message || t('dataStores.message.unknownError') }))
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
    ElMessage.success(t('dataStores.message.testSuccess', { name: row.name }))
  } catch (error: any) {
    ElMessage.error(t('dataStores.message.testFailed', { error: error.message || t('dataStores.message.unknownError') }))
  } finally {
    testing.value = false
  }
}

async function handleTestConnection() {
  try {
    await formRef.value?.validate()
  } catch {
    ElMessage.warning(t('dataStores.message.fillRequired'))
    return
  }

  testing.value = true
  try {
    let parsedConfig: any = {}
    if (form.value.configText) {
      try {
        parsedConfig = JSON.parse(form.value.configText)
      } catch {
        ElMessage.warning(t('dataStores.message.invalidJson'))
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
    ElMessage.success(t('dataStores.message.testSuccess', { name: form.value.name }))
  } catch (error: any) {
    ElMessage.error(t('dataStores.message.testFailed', { error: error.message || t('dataStores.message.unknownError') }))
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

<template>
  <div class="proxy-modes-page">
    <!-- 页面头部 -->
    <div class="header-with-toolbar">
      <div class="header-left">
        <h1 class="page-title">
          <el-icon><Connection /></el-icon>
          {{ t('proxyModes.pageTitle') }}
        </h1>
        <p class="page-description">
          {{ t('proxyModes.pageDescription') }}
          <br/>
          <el-tag type="info" size="small" style="margin-left: 8px">{{ t('proxyModes.keywordFormatHint') }}</el-tag>
        </p>
      </div>
      <div class="toolbar-actions">
        <el-button :loading="loading" @click="loadData">
          <el-icon><Refresh /></el-icon>
          {{ t('proxyModes.refresh') }}
        </el-button>
        <el-button type="primary" @click="openCreate">
          <el-icon><Plus /></el-icon>
          {{ t('proxyModes.addMode') }}
        </el-button>
      </div>
    </div>

    <div class="proxy-modes-content">
      <!-- 模式关键字列表 -->
      <el-card class="table-card" v-loading="loading">
        <el-table :data="keywords" stripe size="large">
          <el-table-column prop="mode_key" :label="t('proxyModes.table.keyword')" width="100" align="center">
            <template #default="{ row }">
              <el-tag type="warning" size="large">{{ row.mode_key }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="mode_name" :label="t('proxyModes.table.name')" min-width="120">
            <template #default="{ row }">
              <span style="font-weight: 500">{{ row.mode_name }}</span>
            </template>
          </el-table-column>
          <el-table-column prop="mode_type" :label="t('proxyModes.table.type')" width="120" align="center">
            <template #default="{ row }">
              <el-tag :type="getTypeTag(row.mode_type)" size="small">
                {{ getTypeLabel(row.mode_type) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="description" :label="t('proxyModes.table.description')" min-width="200" show-overflow-tooltip />
          <el-table-column prop="is_enabled" :label="t('proxyModes.table.enabled')" width="80" align="center">
            <template #default="{ row }">
              <el-switch
                :model-value="row.is_enabled"
                @change="toggleStatus(row)"
                active-color="#10b981"
              />
            </template>
          </el-table-column>
          <el-table-column prop="sort_order" :label="t('proxyModes.table.sortOrder')" width="80" align="center">
            <template #default="{ row }">
              {{ row.sort_order }}
            </template>
          </el-table-column>
          <el-table-column :label="t('proxyModes.table.actions')" width="180" align="center" fixed="right">
            <template #default="{ row }">
              <el-button type="primary" size="small" @click="openEdit(row)">
                <el-icon><Edit /></el-icon>
                {{ t('proxyModes.actions.edit') }}
              </el-button>
              <el-button type="danger" size="small" @click="handleDelete(row)">
                <el-icon><Delete /></el-icon>
                {{ t('proxyModes.actions.delete') }}
              </el-button>
            </template>
          </el-table-column>
        </el-table>

        <el-empty v-if="!loading && keywords.length === 0" :description="t('proxyModes.emptyState')" :image-size="120" />
      </el-card>

      <!-- 使用示例 -->
      <el-card class="example-card" style="margin-top: 20px">
        <template #header>
          <div class="card-header">
            <span>📖 {{ t('proxyModes.usageExamples.title') }}</span>
          </div>
        </template>
        <el-descriptions :column="1" border>
          <el-descriptions-item :label="t('proxyModes.usageExamples.headerMethod')">
            <code>X-Centag-Mode: #d</code>
            <br/>
            <small>{{ t('proxyModes.usageExamples.headerDesc') }}</small>
          </el-descriptions-item>
          <el-descriptions-item :label="t('proxyModes.usageExamples.bodyExtension')">
            <code>{"centag": {"mode": "#s"}}</code>
            <br/>
            <small>{{ t('proxyModes.usageExamples.bodyDesc') }}</small>
          </el-descriptions-item>
          <el-descriptions-item :label="t('proxyModes.usageExamples.contentPrefix')">
            <code>#d 你好，请帮我...</code>
            <br/>
            <code>#m /backend:ollama 这个问题...</code>
            <br/>
            <small>{{ t('proxyModes.usageExamples.contentDesc') }}</small>
          </el-descriptions-item>
          <el-descriptions-item :label="t('proxyModes.usageExamples.apiSession')">
            <code>POST /api/v1/session/proxy-mode</code>
            <br/>
            <small>{{ t('proxyModes.usageExamples.apiDesc') }}</small>
          </el-descriptions-item>
        </el-descriptions>
      </el-card>
    </div>

    <!-- 编辑/创建对话框 -->
    <el-dialog
      v-model="editing"
      :title="isCreate ? t('proxyModes.formDialog.addTitle') : t('proxyModes.formDialog.editTitle')"
      width="600px"
      @close="resetForm"
    >
      <el-form :model="form" :rules="rules" ref="formRef" label-width="100px">
        <el-form-item :label="t('proxyModes.formDialog.keywordLabel')" prop="mode_key">
          <el-input 
            v-model="form.mode_key" 
            placeholder="#d"
            maxlength="10"
            :disabled="!isCreate"
          />
          <div class="form-tip">{{ t('proxyModes.formDialog.keywordTip') }}</div>
        </el-form-item>
        <el-form-item :label="t('proxyModes.formDialog.nameLabel')" prop="mode_name">
          <el-input v-model="form.mode_name" :placeholder="t('proxyModes.formDialog.namePlaceholder')" />
        </el-form-item>
        <el-form-item :label="t('proxyModes.formDialog.typeLabel')" prop="mode_type">
          <el-select v-model="form.mode_type" :placeholder="t('proxyModes.formDialog.typePlaceholder')" style="width: 100%">
            <el-option :label="t('proxyModes.typeOptions.direct')" value="direct" />
            <el-option :label="t('proxyModes.typeOptions.schedule')" value="schedule" />
            <el-option :label="t('proxyModes.typeOptions.match')" value="match" />
            <el-option :label="t('proxyModes.typeOptions.classify')" value="classify" />
            <el-option :label="t('proxyModes.typeOptions.transparent')" value="transparent" />
            <el-option :label="t('proxyModes.typeOptions.fallback')" value="fallback" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('proxyModes.formDialog.descriptionLabel')" prop="description">
          <el-input 
            v-model="form.description" 
            type="textarea" 
            :rows="3"
            :placeholder="t('proxyModes.formDialog.descriptionPlaceholder')"
          />
        </el-form-item>
        <el-form-item :label="t('proxyModes.formDialog.enabledLabel')">
          <el-switch v-model="form.is_enabled" active-color="#10b981" />
          <div class="form-tip">{{ t('proxyModes.formDialog.enabledTip') }}</div>
        </el-form-item>
        <el-form-item :label="t('proxyModes.formDialog.sortOrderLabel')">
          <el-input-number v-model="form.sort_order" :min="0" :max="999" />
          <div class="form-tip">{{ t('proxyModes.formDialog.sortOrderTip') }}</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editing = false">{{ t('proxyModes.formDialog.cancel') }}</el-button>
        <el-button type="primary" :loading="saving" @click="save">{{ t('proxyModes.formDialog.save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '@/api'

const { t } = useI18n()

interface ModeKeyword {
  id: number
  mode_key: string
  mode_name: string
  mode_type: string
  description: string
  is_enabled: boolean
  sort_order: number
  config?: any
}

const loading = ref(false)
const saving = ref(false)
const editing = ref(false)
const isCreate = ref(false)
const keywords = ref<ModeKeyword[]>([])
const formRef = ref()

const form = ref<ModeKeyword>({
  id: 0,
  mode_key: '',
  mode_name: '',
  mode_type: 'direct',
  description: '',
  is_enabled: true,
  sort_order: 0,
})

const rules = {
  mode_key: [
    { required: true, message: t('proxyModes.validation.keywordRequired'), trigger: 'blur' },
    { pattern: /^#[a-zA-Z]$/, message: t('proxyModes.validation.keywordPattern'), trigger: 'blur' }
  ],
  mode_name: [{ required: true, message: t('proxyModes.validation.nameRequired'), trigger: 'blur' }],
  mode_type: [{ required: true, message: t('proxyModes.validation.typeRequired'), trigger: 'change' }]
}

const loadData = async () => {
  loading.value = true
  try {
    const res = await api.get('/api/v1/proxy-modes')
    keywords.value = res || []
  } catch (error: any) {
    ElMessage.error(t('proxyModes.message.loadFailed') + ': ' + error.message)
  } finally {
    loading.value = false
  }
}

const openCreate = () => {
  isCreate.value = true
  editing.value = true
  form.value = {
    id: 0,
    mode_key: '',
    mode_name: '',
    mode_type: 'direct',
    description: '',
    is_enabled: true,
    sort_order: keywords.value.length + 1,
  }
}

const openEdit = (row: ModeKeyword) => {
  isCreate.value = false
  editing.value = true
  form.value = { ...row }
}

const save = async () => {
  if (!formRef.value) return
  
  await formRef.value.validate(async (valid: boolean) => {
    if (!valid) return
    
    saving.value = true
    try {
      if (isCreate.value) {
        await api.post('/api/v1/proxy-modes', form.value)
        ElMessage.success(t('proxyModes.message.createSuccess'))
      } else {
        await api.put(`/api/v1/proxy-modes/${form.value.mode_key}`, form.value)
        ElMessage.success(t('proxyModes.message.updateSuccess'))
      }
      editing.value = false
      loadData()
    } catch (error: any) {
      ElMessage.error(t('proxyModes.message.saveFailed') + ': ' + error.message)
    } finally {
      saving.value = false
    }
  })
}

const handleDelete = async (row: ModeKeyword) => {
  try {
    await ElMessageBox.confirm(t('proxyModes.confirm.deleteMessage', { name: row.mode_name }), t('proxyModes.confirm.deleteTitle'), {
      type: 'warning'
    })
    await api.delete(`/api/v1/proxy-modes/${row.mode_key}`)
    ElMessage.success(t('proxyModes.message.deleteSuccess'))
    loadData()
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error(t('proxyModes.message.operationFailed') + ': ' + error.message)
    }
  }
}

const toggleStatus = async (row: ModeKeyword) => {
  try {
    row.is_enabled = !row.is_enabled
    await api.put(`/api/v1/proxy-modes/${row.mode_key}`, row)
    ElMessage.success(row.is_enabled ? t('proxyModes.message.enabled') : t('proxyModes.message.disabled'))
  } catch (error: any) {
    row.is_enabled = !row.is_enabled
    ElMessage.error(t('proxyModes.message.operationFailed') + ': ' + error.message)
  }
}

const resetForm = () => {
  if (formRef.value) {
    formRef.value.resetFields()
  }
}

const getTypeTag = (type: string) => {
  const map: Record<string, string> = {
    direct: 'success',
    schedule: 'primary',
    match: 'warning',
    classify: 'info',
    transparent: '',
    fallback: 'danger'
  }
  return map[type] || ''
}

const getTypeLabel = (type: string) => {
  const map: Record<string, string> = {
    direct: t('proxyModes.typeLabels.direct'),
    schedule: t('proxyModes.typeLabels.schedule'),
    match: t('proxyModes.typeLabels.match'),
    classify: t('proxyModes.typeLabels.classify'),
    transparent: t('proxyModes.typeLabels.transparent'),
    'transparent-fast': t('proxyModes.typeLabels.transparent-fast'),
    'fixed-egress': t('proxyModes.typeLabels.fixed-egress'),
    fallback: t('proxyModes.typeLabels.fallback')
  }
  return map[type] || type
}

onMounted(() => {
  loadData()
})
</script>

<style scoped>
.proxy-modes-page {
  width: 100%;
  padding: 0 0 24px;
}

.header-with-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 20px;
}

.header-left {
  flex: 1;
}

.page-title {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0 0 8px 0;
  font-size: 24px;
  font-weight: 600;
}

.page-description {
  margin: 0;
  color: #666;
  font-size: 14px;
  line-height: 1.6;
}

.toolbar-actions {
  display: flex;
  gap: 10px;
}

.proxy-modes-content {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.table-card {
  min-height: 400px;
}

.form-tip {
  margin-top: 4px;
  font-size: 12px;
  color: #999;
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-weight: 600;
}

code {
  background: #f5f7fa;
  padding: 2px 6px;
  border-radius: 4px;
  font-family: 'Courier New', monospace;
  font-size: 13px;
}
</style>

<template>
  <div class="model-config-page">
    <div class="page-header">
      <h2>{{ t('modelConfig.title') }}</h2>
      <p class="description">{{ t('modelConfig.subtitle') }}</p>
    </div>

    <!-- 系统变量 -->
    <el-card v-loading="store.loading" class="section-card">
      <template #header>
        <div class="card-header">
          <span>{{ t('modelConfig.systemVariables') }}</span>
          <el-tag size="small" type="info">{{ t('modelConfig.systemVariablesHint') }}</el-tag>
        </div>
      </template>
      <el-table :data="store.systemVariables" style="width: 100%">
        <el-table-column :label="t('modelConfig.varName')" width="240">
          <template #default="{ row }">
            <code class="var-name">{{ row.name }}</code>
          </template>
        </el-table-column>
        <el-table-column :label="t('modelConfig.varValue')">
          <template #default="{ row }">
            <template v-if="isBackendVar(row.name)">
              <el-select
                v-model="row.value"
                filterable
                allow-create
                default-first-option
                :placeholder="t('modelConfig.selectOrInputValue')"
                size="small"
                style="width: 100%"
              >
                <el-option
                  v-for="b in backends"
                  :key="b.id"
                  :label="b.name || b.id"
                  :value="b.id"
                />
              </el-select>
            </template>
            <template v-else-if="isModelVar(row.name)">
              <el-select
                v-model="row.value"
                filterable
                allow-create
                default-first-option
                :placeholder="t('modelConfig.selectOrInputValue')"
                size="small"
                style="width: 100%"
              >
                <el-option
                  v-for="m in modelsForBackend(getRelatedBackendId(row.name))"
                  :key="m"
                  :label="m"
                  :value="m"
                />
              </el-select>
            </template>
            <template v-else>
              <el-input
                v-model="row.value"
                size="small"
                :placeholder="t('modelConfig.selectOrInputValue')"
              />
            </template>
          </template>
        </el-table-column>
        <el-table-column :label="t('modelConfig.description')" min-width="260">
          <template #default="{ row }">
            <span class="var-desc">{{ varDescription(row.name) }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="t('modelConfig.source')" width="120">
          <template #default>
            <el-tag size="small" type="info">
              {{ t('modelConfig.sourceSystem') }}
            </el-tag>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 用户变量 -->
    <el-card class="section-card">
      <template #header>
        <div class="card-header">
          <span>{{ t('modelConfig.userVariables') }}</span>
          <el-button type="primary" size="small" @click="openAddDialog">
            {{ t('modelConfig.addVariable') }}
          </el-button>
        </div>
      </template>
      <el-table :data="store.userVariables" style="width: 100%">
        <el-table-column :label="t('modelConfig.varName')" width="240">
          <template #default="{ row }">
            <code class="var-name">{{ row.name }}</code>
          </template>
        </el-table-column>
        <el-table-column :label="t('modelConfig.varValue')">
          <template #default="{ row }">
            <span class="var-value">{{ row.value }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="t('modelConfig.actions')" width="120">
          <template #default="{ row }">
            <el-button type="danger" size="small" @click="handleDelete(row.name)">
              {{ t('modelConfig.delete') }}
            </el-button>
          </template>
        </el-table-column>
      </el-table>
      <div v-if="store.userVariables.length === 0" class="empty-state">
        {{ t('modelConfig.noUserVariables') }}
      </div>
    </el-card>



    <!-- 使用说明 -->
    <el-card class="section-card usage-card">
      <template #header>
        <span>{{ t('modelConfig.howToUse') }}</span>
      </template>
      <div class="usage-content">
        <p>{{ t('modelConfig.usageDescription') }}</p>
        <div class="usage-example">
          <span>{{ t('modelConfig.varSyntaxHint') }}</span>
          <code class="var-ref">system.default_model</code>
          <span> — {{ t('modelConfig.usageExample') }}</span>
        </div>
      </div>
    </el-card>

    <!-- 添加变量对话框 -->
    <el-dialog
      v-model="showAddDialog"
      :title="t('modelConfig.addVariable')"
      width="500px"
    >
      <el-form :model="newVar" label-width="100px">
        <el-form-item :label="t('modelConfig.varName')">
          <el-select
            v-model="newVar.name"
            filterable
            allow-create
            default-first-option
            :placeholder="t('modelConfig.selectOrInputName')"
            style="width: 100%"
          >
            <el-option-group :label="t('modelConfig.commonNames')">
              <el-option
                v-for="n in presetVarNames"
                :key="n"
                :label="n"
                :value="n"
              />
            </el-option-group>
          </el-select>
        </el-form-item>
        <el-form-item :label="t('modelConfig.varValue')">
          <el-select
            v-if="isBackendValueVar(newVar.name)"
            v-model="newVar.value"
            filterable
            allow-create
            default-first-option
            :placeholder="t('modelConfig.selectOrInputValue')"
            style="width: 100%"
          >
            <el-option
              v-for="b in backends"
              :key="b.id"
              :label="b.name || b.id"
              :value="b.id"
            />
          </el-select>
          <el-select
            v-else-if="isModelValueVar(newVar.name)"
            v-model="newVar.value"
            filterable
            allow-create
            default-first-option
            :placeholder="t('modelConfig.selectOrInputValue')"
            style="width: 100%"
          >
            <el-option
              v-for="m in allModels"
              :key="m"
              :label="m"
              :value="m"
            />
          </el-select>
          <el-input
            v-else
            v-model="newVar.value"
            :placeholder="t('modelConfig.inputValue')"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showAddDialog = false">{{ t('modelConfig.cancel') }}</el-button>
        <el-button type="primary" @click="handleAdd">{{ t('modelConfig.confirm') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useModelConfigStore } from '@/stores/model-config'
import { getBackends } from '@/api/backend'

const { t } = useI18n()
const store = useModelConfigStore()

const showAddDialog = ref(false)
const newVar = ref({ name: '', value: '' })

const backends = ref<any[]>([])
const backendModels = ref<Record<string, string[]>>({})

const presetVarNames = [
  'user.preferred_backend',
  'user.preferred_model',
  'user.custom_endpoint',
  'user.api_key_alias',
  'user.language',
  'user.tone'
]

const editableSystemVars = ['system.rerank_backend', 'system.rerank_model', 'system.classify_backend', 'system.classify_model']

function isEditableSystemVar(name: string): boolean {
  return editableSystemVars.includes(name)
}

const systemVarDescKeys: Record<string, string> = {
  'system.default_backend': 'nodeConfig.systemVarInfo.default_backend.usage',
  'system.default_model': 'nodeConfig.systemVarInfo.default_model.usage',
  'system.fallback_backend': 'nodeConfig.systemVarInfo.fallback_backend.usage',
  'system.fallback_model': 'nodeConfig.systemVarInfo.fallback_model.usage',
  'system.classify_backend': 'nodeConfig.systemVarInfo.classify_backend.usage',
  'system.classify_model': 'nodeConfig.systemVarInfo.classify_model.usage',
  'system.embedding_backend': 'nodeConfig.systemVarInfo.embedding_backend.usage',
  'system.embedding_model': 'nodeConfig.systemVarInfo.embedding_model.usage',
  'system.rerank_backend': 'nodeConfig.systemVarInfo.rerank_backend.usage',
  'system.rerank_model': 'nodeConfig.systemVarInfo.rerank_model.usage',
}

function varDescription(name: string): string {
  const key = systemVarDescKeys[name]
  return key ? t(key) : ''
}

function isBackendVar(name: string): boolean {
  return name.includes('backend')
}

function isModelVar(name: string): boolean {
  return name.includes('model')
}

function getRelatedBackendId(modelVarName: string): string {
  const prefix = modelVarName.replace('_model', '_backend')
  const found = store.systemVariables.find(v => v.name === prefix)
  return found?.value || ''
}

function isBackendValueVar(name: string): boolean {
  return name.includes('backend')
}

function isModelValueVar(name: string): boolean {
  return name.includes('model')
}

function modelsForBackend(backendId: string): string[] {
  if (!backendId) return allModels.value
  return backendModels.value[backendId] || []
}

const allModels = computed(() => {
  const set = new Set<string>()
  for (const models of Object.values(backendModels.value)) {
    for (const m of models) set.add(m)
  }
  return Array.from(set).sort()
})

async function loadBackends() {
  try {
    const res = await getBackends()
    backends.value = Array.isArray(res) ? res : []
    for (const b of backends.value) {
      const models = (b.supported_models || [])
        .map((m: any) => m.actual_model || m.requested_model || m.name || m)
        .filter(Boolean)
      backendModels.value[b.id] = models
    }
  } catch {
    backends.value = []
  }
}

const handleSave = async () => {
  const variables: Record<string, string> = {}
  for (const item of store.systemVariables) {
    variables[item.name] = item.value
  }
  for (const item of store.userVariables) {
    variables[item.name] = item.value
  }
  await store.saveConfig(variables)
}

let saveTimeout: ReturnType<typeof setTimeout> | null = null

function debouncedSave() {
  if (saveTimeout) clearTimeout(saveTimeout)
  saveTimeout = setTimeout(() => {
    handleSave()
  }, 500)
}

watch(
  () => store.systemVariables.map(v => v.value),
  () => {
    if (!store.skipWatch) {
      debouncedSave()
    }
  },
  { deep: true, flush: 'sync' }
)

function openAddDialog() {
  newVar.value = { name: '', value: '' }
  showAddDialog.value = true
}

const handleAdd = () => {
  if (newVar.value.name && newVar.value.value) {
    store.addVariable(newVar.value.name, newVar.value.value)
    newVar.value = { name: '', value: '' }
    showAddDialog.value = false
  }
}

const handleDelete = (name: string) => {
  store.deleteVariable(name)
}

onMounted(() => {
  store.loadConfig()
  loadBackends()
})
</script>

<style scoped>
.model-config-page {
  padding: 20px;
}

.page-header {
  margin-bottom: 20px;
}

.page-header h2 {
  margin: 0 0 8px 0;
}

.description {
  color: #666;
  margin: 0;
  font-size: 14px;
}

.section-card {
  margin-bottom: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.var-name {
  font-family: 'Monaco', 'Menlo', 'Consolas', monospace;
  font-size: 13px;
  color: #409eff;
  background: #f0f7ff;
  padding: 2px 6px;
  border-radius: 4px;
}

.var-value {
  font-family: 'Monaco', 'Menlo', 'Consolas', monospace;
  font-size: 13px;
  color: #333;
}

.var-desc {
  color: #666;
  font-size: 13px;
}

.empty-state {
  text-align: center;
  padding: 20px;
  color: #999;
}

.usage-card {
  border: 1px solid #e6f7ff;
  background: #f6ffed;
}

.usage-content p {
  margin: 0 0 12px 0;
  color: #666;
}

.usage-example {
  background: #f5f5f5;
  padding: 12px;
  border-radius: 6px;
}

.usage-example code {
  font-family: 'Monaco', 'Menlo', 'Consolas', monospace;
  font-size: 13px;
  color: #333;
}

.var-ref {
  font-family: 'Monaco', 'Menlo', 'Consolas', monospace;
  font-size: 13px;
  color: #409eff;
  background: #f0f7ff;
  padding: 2px 6px;
  border-radius: 4px;
}
</style>

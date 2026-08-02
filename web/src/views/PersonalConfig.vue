<template>
  <div class="personal-config-page">
    <el-card shadow="never">
      <template #header>
        <div class="card-header">
          <span>{{ t('personalConfig.title') }}</span>
          <div class="header-actions">
            <el-button size="small" @click="load">{{ t('personalConfig.refresh') }}</el-button>
            <el-button size="small" type="primary" @click="openEditor">{{ t('personalConfig.editYaml') }}</el-button>
          </div>
        </div>
      </template>

      <div class="config-description">
        {{ t('personalConfig.description') }}
      </div>

      <el-table
        v-loading="loading"
        :data="configRules"
        stripe
        size="small"
        :empty-text="t('personalConfig.emptyText')"
        class="config-table"
      >
        <el-table-column prop="name" :label="t('personalConfig.table.name')" min-width="120" />
        <el-table-column prop="backend_id" :label="t('personalConfig.table.backend')" width="120" />
        <el-table-column prop="model" :label="t('personalConfig.table.model')" width="140" />
        <el-table-column prop="price_type" :label="t('personalConfig.table.priceType')" width="90">
          <template #default="{ row }">
            <el-tag :type="row.price_type === 'revenue' ? 'warning' : 'info'" size="small">
              {{ row.price_type || 'cost' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('personalConfig.table.inputPrice')" width="110">
          <template #default="{ row }">${{ row.input_price_per_m?.toFixed(4) || '0.0000' }}</template>
        </el-table-column>
        <el-table-column :label="t('personalConfig.table.outputPrice')" width="110">
          <template #default="{ row }">${{ row.output_price_per_m?.toFixed(4) || '0.0000' }}</template>
        </el-table-column>
        <el-table-column prop="priority" :label="t('personalConfig.table.priority')" width="70" />
        <el-table-column :label="t('personalConfig.table.enabled')" width="70">
          <template #default="{ row }">
            <el-tag :type="row.enabled ? 'success' : 'info'" size="small">
              {{ row.enabled ? t('personalConfig.table.enabledYes') : t('personalConfig.table.enabledNo') }}
            </el-tag>
          </template>
        </el-table-column>
      </el-table>

      <div class="allowed-resources">
        <h4>{{ t('personalConfig.allowedResources') }}</h4>
        <el-row :gutter="16">
          <el-col :span="8">
            <div class="resource-section">
              <div class="resource-label">{{ t('personalConfig.allowedBackends') }}</div>
              <div class="resource-tags">
                <el-tag
                  v-for="backend in configData.allowed_backends"
                  :key="backend"
                  size="small"
                  type="info"
                >
                  {{ backend }}
                </el-tag>
                <span v-if="!configData.allowed_backends?.length" class="no-resource">
                  {{ t('personalConfig.allAllowed') }}
                </span>
              </div>
            </div>
          </el-col>
          <el-col :span="8">
            <div class="resource-section">
              <div class="resource-label">{{ t('personalConfig.allowedModels') }}</div>
              <div class="resource-tags">
                <el-tag
                  v-for="model in configData.allowed_models"
                  :key="model"
                  size="small"
                  type="info"
                >
                  {{ model }}
                </el-tag>
                <span v-if="!configData.allowed_models?.length" class="no-resource">
                  {{ t('personalConfig.allAllowed') }}
                </span>
              </div>
            </div>
          </el-col>
          <el-col :span="8">
            <div class="resource-section">
              <div class="resource-label">{{ t('personalConfig.allowedPriceTypes') }}</div>
              <div class="resource-tags">
                <el-tag
                  v-for="priceType in configData.allowed_price_types"
                  :key="priceType"
                  size="small"
                  type="info"
                >
                  {{ priceType }}
                </el-tag>
                <span v-if="!configData.allowed_price_types?.length" class="no-resource">
                  {{ t('personalConfig.allAllowed') }}
                </span>
              </div>
            </div>
          </el-col>
        </el-row>
      </div>
    </el-card>

    <!-- YAML Editor Dialog -->
    <el-dialog
      v-model="editorVisible"
      :title="t('personalConfig.editor.title')"
      width="80%"
      destroy-on-close
    >
      <div class="editor-container">
        <div class="editor-toolbar">
          <el-button size="small" @click="formatYaml">{{ t('personalConfig.editor.format') }}</el-button>
          <el-button size="small" @click="validateYaml">{{ t('personalConfig.editor.validate') }}</el-button>
          <el-button size="small" @click="resetToDefault">{{ t('personalConfig.editor.reset') }}</el-button>
        </div>
        <el-input
          v-model="yamlContent"
          type="textarea"
          :rows="30"
          :placeholder="t('personalConfig.editor.placeholder')"
          class="yaml-editor"
        />
        <div v-if="validationError" class="validation-error">
          {{ validationError }}
        </div>
      </div>
      <template #footer>
        <el-button @click="editorVisible = false">{{ t('personalConfig.editor.cancel') }}</el-button>
        <el-button type="primary" :loading="saving" @click="saveYaml">{{ t('personalConfig.editor.save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import yaml from 'js-yaml'

const { t } = useI18n()

const loading = ref(false)
const saving = ref(false)
const editorVisible = ref(false)
const yamlContent = ref('')
const validationError = ref('')

interface PricingRule {
  name: string
  backend_id: string
  model: string
  price_type?: string
  input_price_per_m: number
  output_price_per_m: number
  priority: number
  enabled: boolean
}

interface ConfigData {
  version: string
  pricing_rules: PricingRule[]
  allowed_backends: string[]
  allowed_models: string[]
  allowed_price_types: string[]
  self_limit?: {
    enabled: boolean
    daily_token_limit?: number
    monthly_budget_limit?: number
  }
}

const configData = ref<ConfigData>({
  version: '2.0',
  pricing_rules: [],
  allowed_backends: [],
  allowed_models: [],
  allowed_price_types: [],
  self_limit: undefined
})

const configRules = ref<PricingRule[]>([])

async function load() {
  loading.value = true
  try {
    const response = await fetch('/api/v1/billing/config')
    if (response.ok) {
      const data = await response.json()
      configData.value = data
      configRules.value = data.pricing_rules || []
    }
  } catch (e: unknown) {
    ElMessage.error(e instanceof Error ? e.message : t('personalConfig.message.loadFailed'))
  } finally {
    loading.value = false
  }
}

function openEditor() {
  yamlContent.value = yaml.dump(configData.value, { indent: 2 })
  validationError.value = ''
  editorVisible.value = true
}

function formatYaml() {
  try {
    const parsed = yaml.load(yamlContent.value)
    yamlContent.value = yaml.dump(parsed, { indent: 2 })
    validationError.value = ''
  } catch (e: unknown) {
    validationError.value = e instanceof Error ? e.message : t('personalConfig.editor.invalidYaml')
  }
}

function validateYaml() {
  try {
    const parsed = yaml.load(yamlContent.value) as ConfigData
    if (!parsed.version) {
      validationError.value = t('personalConfig.editor.missingVersion')
      return
    }
    if (!Array.isArray(parsed.pricing_rules)) {
      validationError.value = t('personalConfig.editor.missingRules')
      return
    }
    validationError.value = ''
    ElMessage.success(t('personalConfig.editor.validYaml'))
  } catch (e: unknown) {
    validationError.value = e instanceof Error ? e.message : t('personalConfig.editor.invalidYaml')
  }
}

function resetToDefault() {
  const defaultConfig: ConfigData = {
    version: '2.0',
    pricing_rules: [],
    allowed_backends: [],
    allowed_models: [],
    allowed_price_types: []
  }
  yamlContent.value = yaml.dump(defaultConfig, { indent: 2 })
  validationError.value = ''
}

async function saveYaml() {
  try {
    const parsed = yaml.load(yamlContent.value) as ConfigData
    if (!parsed.version || !Array.isArray(parsed.pricing_rules)) {
      ElMessage.error(t('personalConfig.editor.invalidConfig'))
      return
    }

    saving.value = true
    const response = await fetch('/api/v1/billing/config/edit', {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-yaml' },
      body: yamlContent.value
    })

    if (response.ok) {
      ElMessage.success(t('personalConfig.message.saveSuccess'))
      editorVisible.value = false
      await load()
    } else {
      const error = await response.text()
      ElMessage.error(error || t('personalConfig.message.saveFailed'))
    }
  } catch (e: unknown) {
    ElMessage.error(e instanceof Error ? e.message : t('personalConfig.message.saveFailed'))
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  load()
})
</script>

<style scoped>
.personal-config-page {
  padding: 16px;
}
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.header-actions {
  display: flex;
  gap: 8px;
}
.config-description {
  color: var(--el-text-color-secondary);
  font-size: 14px;
  margin-bottom: 16px;
}
.config-table {
  width: 100%;
  margin-bottom: 24px;
}
.allowed-resources {
  margin-top: 24px;
}
.allowed-resources h4 {
  margin: 0 0 16px;
  font-size: 16px;
  font-weight: 500;
}
.resource-section {
  margin-bottom: 16px;
}
.resource-label {
  font-weight: 500;
  margin-bottom: 8px;
  color: var(--el-text-color-primary);
}
.resource-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.no-resource {
  color: var(--el-text-color-secondary);
  font-style: italic;
}
.editor-container {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.editor-toolbar {
  display: flex;
  gap: 8px;
}
.yaml-editor {
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  font-size: 13px;
}
.validation-error {
  color: var(--el-color-danger);
  font-size: 13px;
  padding: 8px;
  background: var(--el-color-danger-light-9);
  border-radius: 4px;
}
</style>

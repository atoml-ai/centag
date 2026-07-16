<template>
  <div class="node-test-panel">
    <el-card header="Plugin Test">
      <template #header>
        <div class="panel-header">
          <span>Test: {{ plugin?.name }}</span>
          <el-tag size="small">{{ plugin?.implementation }}</el-tag>
          <el-tag v-if="plugin?.deprecated" type="warning" size="small">Deprecated</el-tag>
        </div>
      </template>

      <el-alert
        v-if="plugin"
        class="plugin-test-hint"
        type="info"
        :closable="false"
        show-icon
      >
        <template #default>
          <div class="hint-line">
            <strong>Permissions:</strong>
            <template v-if="plugin.permissions?.length">
              <el-tag
                v-for="perm in plugin.permissions"
                :key="perm"
                size="small"
                class="permission-tag"
              >
                {{ perm }}
              </el-tag>
            </template>
            <span v-else>none</span>
          </div>
          <div v-if="plugin.min_centag_version" class="hint-line">
            <strong>Minimum Centag:</strong> {{ plugin.min_centag_version }}
          </div>
        </template>
      </el-alert>

      <el-tabs v-model="activeTab">
        <el-tab-pane label="Config" name="config">
          <SchemaForm
            ref="configFormRef"
            :schema="configSchema"
            v-model="configData"
          />
        </el-tab-pane>

        <el-tab-pane label="Input" name="input">
          <el-form label-position="top">
            <el-form-item label="Input Data (JSON)">
              <el-input
                v-model="inputJson"
                type="textarea"
                :rows="10"
                placeholder='{"content": "test input"}'
                @blur="parseInput"
              />
            </el-form-item>
          </el-form>
        </el-tab-pane>

        <el-tab-pane label="Results" name="results">
          <div v-if="executing" class="result-loading">
            <el-icon class="is-loading"><Loading /></el-icon>
            <span>Executing...</span>
          </div>
          <div v-else-if="error" class="result-error">
            <el-alert type="error" :title="errorTitle" :closable="false" show-icon>
              <template #default>
                <div class="error-detail" v-if="errorDetail">
                  <div v-if="errorDetail.code"><strong>Error Code:</strong> {{ errorDetail.code }}</div>
                  <div v-if="errorDetail.retryable !== undefined">
                    <strong>Retryable:</strong> {{ errorDetail.retryable ? 'Yes' : 'No' }}
                    <el-button v-if="errorDetail.retryable" size="small" @click="runTest" style="margin-left: 8px;">Retry</el-button>
                  </div>
                  <div v-if="errorDetail.details">
                    <strong>Details:</strong>
                    <pre>{{ formatJson(errorDetail.details) }}</pre>
                  </div>
                  <div v-if="errorDetail.errors?.length" class="field-errors">
                    <strong>Field Errors:</strong>
                    <el-timeline>
                      <el-timeline-item
                        v-for="(err, idx) in errorDetail.errors"
                        :key="idx"
                        :timestamp="err.code"
                        placement="top"
                      >
                        {{ err.message }}
                        <div v-if="err.field_path" class="field-path">Field: {{ err.field_path }}</div>
                      </el-timeline-item>
                    </el-timeline>
                  </div>
                </div>
              </template>
            </el-alert>
          </div>
          <div v-else-if="result" class="result-content">
            <el-divider>Output</el-divider>
            <pre>{{ formatJson(result.output) }}</pre>
            <template v-if="result.events?.length">
              <el-divider>Events</el-divider>
              <el-timeline>
                <el-timeline-item
                  v-for="(event, idx) in result.events"
                  :key="idx"
                  :timestamp="event.type"
                  placement="top"
                >
                  {{ event.message }}
                </el-timeline-item>
              </el-timeline>
            </template>
          </div>
          <div v-else class="result-empty">
            <el-empty description="No execution results yet" />
          </div>
        </el-tab-pane>
      </el-tabs>

      <div class="panel-actions">
        <el-button type="primary" :loading="executing" @click="runTest">
          Execute Test
        </el-button>
        <el-button @click="reset">Reset</el-button>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { Loading } from '@element-plus/icons-vue'
import SchemaForm from './SchemaForm.vue'
import { testNodePlugin } from '@/api/pipeline'

interface PluginDescriptor {
  name: string
  implementation: string
  config_schema?: Record<string, any>
  input_schema?: Record<string, any>
  permissions?: string[]
  deprecated?: boolean
  min_centag_version?: string
}

const props = defineProps<{
  plugin: PluginDescriptor | null
}>()

const emit = defineEmits<{
  close: []
}>()

const activeTab = ref('config')
const configFormRef = ref()
const configData = ref<Record<string, any>>({})
const inputJson = ref('{\n  "content": "test input"\n}')
const inputData = ref<Record<string, any>>({ content: 'test input' })

const executing = ref(false)
const error = ref<any>(null)
const result = ref<any>(null)

const configSchema = computed(() => props.plugin?.config_schema || {})
const errorDetail = computed(() => normalizeError(error.value))
const errorTitle = computed(() => errorDetail.value.message || 'Execution failed')

watch(() => props.plugin, () => {
  reset()
})

function parseInput() {
  try {
    inputData.value = JSON.parse(inputJson.value)
    return true
  } catch (e) {
    error.value = {
      code: 'input.invalid_json',
      message: e instanceof Error ? e.message : 'Input JSON is invalid',
      retryable: false,
    }
    activeTab.value = 'results'
    return false
  }
}

async function runTest() {
  if (!props.plugin) return

  const valid = await configFormRef.value?.validate()
  if (!valid) {
    activeTab.value = 'config'
    return
  }

  if (!parseInput()) return
  executing.value = true
  error.value = null
  result.value = null

  try {
    const resp = await testNodePlugin(props.plugin.implementation, {
      config: configData.value,
      input: inputData.value
    })
    result.value = resp.data.data
  } catch (e: any) {
    error.value = e.response?.data?.error || e.response?.data || e.message || 'Execution failed'
  } finally {
    executing.value = false
    activeTab.value = 'results'
  }
}

function reset() {
  configData.value = {}
  inputJson.value = '{\n  "content": "test input"\n}'
  inputData.value = { content: 'test input' }
  error.value = null
  result.value = null
  activeTab.value = 'config'
}

function formatJson(data: any): string {
  try {
    return JSON.stringify(data, null, 2)
  } catch {
    return String(data)
  }
}

function normalizeError(value: any): Record<string, any> {
  if (!value) return {}
  if (typeof value === 'string') {
    return { message: value }
  }
  if (value.error && typeof value.error === 'object') {
    return normalizeError(value.error)
  }
  if (value.message || value.code || value.details || value.errors) {
    return value
  }
  return { message: formatJson(value), details: value }
}
</script>

<style scoped>
.node-test-panel {
  margin-top: 16px;
}
.panel-header {
  display: flex;
  align-items: center;
  gap: 12px;
}
.panel-actions {
  margin-top: 16px;
  display: flex;
  gap: 8px;
}
.plugin-test-hint {
  margin-bottom: 12px;
}
.hint-line {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
  margin: 2px 0;
}
.permission-tag {
  margin-right: 4px;
}
.result-loading {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 20px;
  color: #409eff;
}
.result-error {
  margin-bottom: 16px;
}
.result-content pre {
  background: #f5f7fa;
  padding: 12px;
  border-radius: 4px;
  overflow-x: auto;
  font-size: 12px;
}
.result-empty {
  padding: 40px 0;
}
.field-path {
  margin-top: 4px;
  color: #909399;
  font-size: 12px;
}
</style>
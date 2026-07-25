<template>
  <div class="evaluation">
    <div class="header">
      <h1 class="page-title">{{ $t('evaluation.title') }}</h1>
      <p class="page-description">{{ $t('evaluation.subtitle') }}</p>
    </div>

    <div class="content-wrapper">
      <el-row :gutter="16">
        <!-- 左侧：评估统计 -->
        <el-col :xs="24" :sm="24" :md="8" :lg="8" :xl="8">
          <div class="left-panel">
            <!-- 评估统计 -->
            <el-card class="info-card">
              <template #header>
                <div class="card-header">
                  <span class="card-title">{{ $t('evaluation.evalStats') }}</span>
                </div>
              </template>
              <el-descriptions :column="1" border size="small">
                <el-descriptions-item :label="$t('evaluation.evalStatus')">
                  <el-tag :type="evaluationStats.enabled ? 'success' : 'info'">
                    {{ evaluationStats.enabled ? $t('evaluation.enabled') : $t('evaluation.disabled') }}
                  </el-tag>
                </el-descriptions-item>
                <el-descriptions-item :label="$t('evaluation.exactMatchCache')">
                  <el-switch
                    v-model="exactMatchEnabled"
                    @change="handleExactMatchChange"
                    :loading="exactMatchLoading"
                  />
                </el-descriptions-item>
                <el-descriptions-item :label="$t('evaluation.totalEvals')">
                  {{ formatNumber(evaluationStats.total_executions || 0) }}
                </el-descriptions-item>
                <el-descriptions-item :label="$t('evaluation.enabledPlugins')">
                  {{ evaluationStats.enabled_plugins || 0 }}
                </el-descriptions-item>
                <el-descriptions-item :label="$t('evaluation.cacheHitRate')">
                  {{ cacheHitRate }}%
                </el-descriptions-item>
              </el-descriptions>
              <div class="card-actions">
                <el-button :loading="loading" @click="load" style="width: 100%">
                  <el-icon><Refresh /></el-icon>
                  {{ $t('evaluation.refresh') }}
                </el-button>
              </div>
            </el-card>

            <!-- 评估结果分布 -->
            <el-card class="chart-card">
              <template #header>
                <span class="card-title">{{ $t('evaluation.evalResultDistribution') }}</span>
              </template>
              <div class="chart-container">
                <el-progress
                  type="dashboard"
                  :percentage="cacheHitRate"
                  :color="progressColors"
                  :width="180"
                >
                  <template #default="{ percentage }">
                    <span class="percentage-value">{{ percentage }}%</span>
                  </template>
                </el-progress>
                <div class="chart-legend">
                  <div class="legend-item">
                    <span class="legend-dot success"></span>
                    <span class="legend-text">{{ $t('evaluation.allow') }}: {{ formatNumber(evaluationStats.allowed_count) }}</span>
                  </div>
                  <div class="legend-item">
                    <span class="legend-dot warning"></span>
                    <span class="legend-text">{{ $t('evaluation.reject') }}: {{ formatNumber(evaluationStats.rejected_count) }}</span>
                  </div>
                </div>
              </div>
            </el-card>
          </div>
        </el-col>

        <!-- 右侧：插件列表 -->
        <el-col :xs="24" :sm="24" :md="16" :lg="16" :xl="16">
          <el-card class="plugins-card" v-loading="listLoading">
            <template #header>
              <div class="card-header">
                <span class="card-title">{{ $t('evaluation.evalPlugins') }}</span>
                <div>
                  <el-button @click="handleTest" type="primary" size="small">
                    <el-icon><Operation /></el-icon>
                    {{ $t('evaluation.testEval') }}
                  </el-button>
                  <el-button :loading="listLoading" @click="loadPlugins">
                    <el-icon><Refresh /></el-icon>
                    {{ $t('evaluation.refresh') }}
                  </el-button>
                </div>
              </div>
            </template>

            <el-table :data="plugins" stripe border>
              <el-table-column :label="$t('evaluation.status')" width="80" align="center">
                <template #default="{ row }">
                  <el-switch
                    v-model="row.enabled"
                    @change="handleEnableChange(row)"
                  />
                </template>
              </el-table-column>
              <el-table-column prop="name" :label="$t('evaluation.pluginName')" min-width="120" show-overflow-tooltip>
                <template #default="{ row }">
                  <el-icon><component :is="row.icon || 'Box'" /></el-icon>
                  <span style="margin-left: 8px">{{ row.label || row.name }}</span>
                </template>
              </el-table-column>
              <el-table-column prop="description" :label="$t('evaluation.description')" min-width="200" show-overflow-tooltip>
              </el-table-column>
              <el-table-column prop="type" :label="$t('evaluation.type')" width="100">
                <template #default="{ row }">
                  <el-tag :type="row.type === 'aggregator' ? 'danger' : 'primary'" size="small">
                    {{ row.type === 'aggregator' ? $t('evaluation.aggregator') : $t('evaluation.evaluator') }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column :label="$t('evaluation.actions')" width="180" fixed="right">
                <template #default="{ row }">
                  <el-button
                    type="primary"
                    size="small"
                    link
                    @click="handleConfig(row)"
                  >
                    <el-icon><Setting /></el-icon>
                    {{ $t('evaluation.configure') }}
                  </el-button>
                </template>
              </el-table-column>
            </el-table>

            <!-- 拖拽排序提示 -->
            <div class="sort-hint">
              <el-icon><InfoFilled /></el-icon>
              <span>{{ $t('evaluation.pluginOrderHint') }}</span>
            </div>
          </el-card>
        </el-col>
      </el-row>
    </div>

    <!-- 配置对话框 -->
    <el-dialog
      v-model="configDialogVisible"
      :title="$t('evaluation.configurePlugin')"
      width="700px"
      style="min-height: 520px;"
    >
      <!-- 插件信息 -->
      <el-descriptions v-if="currentPlugin" :column="2" border size="small" class="plugin-info">
        <el-descriptions-item :label="$t('evaluation.pluginNameLabel')">{{ currentPlugin.name }}</el-descriptions-item>
        <el-descriptions-item :label="$t('evaluation.typeLabel')">
          <el-tag size="small">{{ currentPlugin.type }}</el-tag>
        </el-descriptions-item>
        <el-descriptions-item :label="$t('evaluation.statusLabel')">
          <el-tag :type="currentPlugin.enabled ? 'success' : 'info'" size="small">
            {{ currentPlugin.enabled ? $t('evaluation.enabled') : $t('evaluation.disabled') }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item :label="$t('evaluation.descriptionLabel')" :span="2">{{ currentPlugin.description }}</el-descriptions-item>
      </el-descriptions>

      <el-divider>{{ $t('evaluation.paramConfig') }}</el-divider>

      <div v-if="!pluginConfigSchema?.fields?.length" class="config-empty">
        <el-empty :description="$t('evaluation.noConfigParams')" />
      </div>
      <div v-else-if="currentPlugin" class="config-container">
        <!-- 数字类型参数：每行最多2个 -->
        <div class="config-row config-row-grid2">
          <div
            v-for="field in pluginConfigSchema.fields.filter(f => f.type === 'number')"
            :key="field.name"
            class="config-item"
          >
            <div class="config-item-header">
              <span class="config-item-label">{{ field.description || field.name }}</span>
              <span v-if="field.default !== undefined" class="config-item-hint">{{ $t('evaluation.default') }}: {{ field.default }}</span>
            </div>
            <el-input-number
              v-model="configForm[field.name]"
              :min="field.min"
              :max="field.max"
              :step="1"
              size="small"
              controls-position="right"
              style="width: 100%"
            />
          </div>
        </div>
        <!-- 其他类型参数 -->
        <div
          v-for="field in pluginConfigSchema.fields.filter(f => f.type !== 'number')"
          :key="field.name"
          class="config-item config-item-full"
        >
          <div class="config-item-header">
            <span class="config-item-label">{{ field.description || field.name }}</span>
            <span class="config-item-name">{{ field.name }}</span>
          </div>
          <el-switch
            v-if="field.type === 'boolean'"
            v-model="configForm[field.name]"
            size="small"
          />
          <!-- 有 options 枚举时渲染下拉框 -->
          <el-select
            v-else-if="field.options && field.options.length"
            v-model="configForm[field.name]"
            size="small"
            style="width: 100%"
          >
            <el-option
              v-for="opt in field.options"
              :key="opt"
              :label="opt"
              :value="opt"
            />
          </el-select>
          <el-input
            v-else-if="field.type === 'string'"
            v-model="configForm[field.name]"
            size="small"
            :placeholder="field.description"
          />
          <el-input
            v-else-if="field.type === 'array'"
            v-model="configForm[field.name]"
            type="textarea"
            :rows="4"
            size="small"
            :placeholder="$t('evaluation.oneValuePerLine')"
          />
        </div>
      </div>
      <template #footer>
        <el-button @click="configDialogVisible = false">{{ $t('evaluation.cancel') }}</el-button>
        <el-button type="primary" @click="handleSaveConfig" :loading="configSaving">
          {{ $t('evaluation.save') }}
        </el-button>
      </template>
    </el-dialog>

    <!-- 测试评估对话框 -->
    <el-dialog
      v-model="testDialogVisible"
      :title="$t('evaluation.testCacheEval')"
      width="700px"
    >
      <el-form :model="testForm" label-width="100px">
        <el-form-item :label="$t('evaluation.question')">
          <el-input
            v-model="testForm.question"
            type="textarea"
            :rows="3"
          />
        </el-form-item>
        <el-form-item :label="$t('evaluation.answer')">
          <el-input
            v-model="testForm.answer"
            type="textarea"
            :rows="5"
          />
        </el-form-item>
      </el-form>
      <div v-if="testResult" class="test-result">
        <el-divider>{{ $t('evaluation.evalResult') }}</el-divider>
        <el-descriptions :column="1" border>
          <el-descriptions-item :label="$t('evaluation.allowCache')">
            <el-tag :type="testResult.passed ? 'success' : 'danger'">
              {{ testResult.passed ? $t('evaluation.allow') : $t('evaluation.reject') }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item :label="$t('evaluation.score')">
            {{ testResult.score?.toFixed(2) || '-' }}
          </el-descriptions-item>
          <el-descriptions-item :label="$t('evaluation.tags')">
            <el-tag
              v-for="label in testResult.labels"
              :key="label"
              style="margin-right: 4px"
              size="small"
            >
              {{ label }}
            </el-tag>
            <span v-if="!testResult.labels || testResult.labels.length === 0">-</span>
          </el-descriptions-item>
          <el-descriptions-item v-if="testResult.details" :label="$t('evaluation.details')">
            <pre class="details-json">{{ JSON.stringify(testResult.details, null, 2) }}</pre>
          </el-descriptions-item>
        </el-descriptions>
      </div>
      <template #footer>
        <el-button @click="testDialogVisible = false">{{ $t('evaluation.close') }}</el-button>
        <el-button type="primary" @click="handleRunTest" :loading="testRunning">
          {{ $t('evaluation.runTest') }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Refresh,
  Operation,
  Setting,
  InfoFilled,
  Box
} from '@element-plus/icons-vue'
import {
  getEvaluationStats,
  getEvaluationPlugins,
  enableEvaluationPlugin,
  disableEvaluationPlugin,
  getPluginConfig,
  updatePluginConfig,
  getPluginSchema,
  testEvaluation,
  setExactMatchEnabled
} from '@/api/evaluation'

const { t } = useI18n()

// 插件列表
const plugins = ref<any[]>([])

// 评估统计
const evaluationStats = ref<any>({
  enabled: false,
  total_executions: 0,
  enabled_plugins: 0
})

// 精确匹配缓存开关
const exactMatchEnabled = ref(false)
const exactMatchLoading = ref(false)

// 加载状态
const loading = ref(false)
const listLoading = ref(false)

// 配置对话框
const configDialogVisible = ref(false)
const configForm = ref<Record<string, any>>({})
const currentPlugin = ref<any>(null)
const pluginConfigSchema = ref<any>(null)
const configSaving = ref(false)

// 测试对话框
const testDialogVisible = ref(false)
const testForm = ref({
  question: '你好，今天天气怎么样？',
  answer: '作为AI助手，我无法实时查询天气信息。建议您查看天气预报应用或网站获取准确的天气情况。如果您需要其他帮助，请随时告诉我！'
})
const testResult = ref<any>(null)
const testRunning = ref(false)

// 缓存命中率（基于评估结果计算，需要从后端获取更多数据）
const cacheHitRate = ref(0)

// 进度条颜色
const progressColors = computed(() => [
  { color: '#f56c6c', percentage: 20 },
  { color: '#e6a23c', percentage: 40 },
  { color: '#5cb87a', percentage: 60 },
  { color: '#1989fa', percentage: 80 },
  { color: '#6f7ad3', percentage: 100 }
])


// 格式化数字
function formatNumber(num: number) {
  if (num >= 10000) {
    return (num / 10000).toFixed(1) + '万'
  } else if (num >= 1000) {
    return (num / 1000).toFixed(1) + 'k'
  }
  return num.toString()
}

// 格式化默认值
function formatDefault(value: any) {
  if (value === null || value === undefined) return '-'
  if (Array.isArray(value)) return JSON.stringify(value)
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value)
}

// 加载数据
async function load() {
  await Promise.all([loadStats(), loadPlugins()])
}

// 加载评估统计
async function loadStats() {
  try {
    loading.value = true
    const res = await getEvaluationStats()
    // 后端返回 { stats: {...} }
    const stats = res.stats || res
    evaluationStats.value = stats
    exactMatchEnabled.value = stats.exact_match_enabled ?? false
  } catch (error: any) {
    console.error('Failed to load evaluation stats:', error)
    ElMessage.error(t('evaluation.loadStatsFailed') + ': ' + error.message)
  } finally {
    loading.value = false
  }
}

// 切换精确匹配缓存开关
async function handleExactMatchChange() {
  try {
    exactMatchLoading.value = true
    await setExactMatchEnabled(exactMatchEnabled.value)
    ElMessage.success(t('evaluation.exactMatchToggled'))
  } catch (error: any) {
    console.error('Failed to set exact match:', error)
    ElMessage.error(t('evaluation.exactMatchToggleFailed') + ': ' + error.message)
    exactMatchEnabled.value = !exactMatchEnabled.value // 恢复原状态
  } finally {
    exactMatchLoading.value = false
  }
}

// 加载插件列表
async function loadPlugins() {
  try {
    listLoading.value = true
    const res = await getEvaluationPlugins()
    plugins.value = res.plugins || []
  } catch (error: any) {
    console.error('Failed to load plugins:', error)
    ElMessage.error(t('evaluation.loadPluginsFailed') + ': ' + error.message)
  } finally {
    listLoading.value = false
  }
}

// 启用/禁用插件
async function handleEnableChange(row: any) {
  const actionText = row.enabled ? t('evaluation.enabled') : t('evaluation.disabled')

  try {
    if (row.enabled) {
      await enableEvaluationPlugin(row.name)
    } else {
      await disableEvaluationPlugin(row.name)
    }
    ElMessage.success(t('evaluation.pluginActionSuccess', { action: actionText }))
  } catch (error: any) {
    console.error('Failed to change plugin status:', error)
    ElMessage.error(t('evaluation.pluginActionFailed', { action: actionText }) + ': ' + error.message)
    row.enabled = !row.enabled // 恢复原状态
  }
}

// 配置插件
async function handleConfig(row: any) {
  currentPlugin.value = row
  configForm.value = {}

  try {
    // 获取配置 Schema
    const schemaRes = await getPluginSchema(row.name)
    pluginConfigSchema.value = schemaRes.schema || schemaRes

    // 先用默认值初始化
    if (pluginConfigSchema.value?.fields) {
      for (const field of pluginConfigSchema.value.fields) {
        if (field.default !== undefined) {
          let defaultVal = field.default
          if (field.type === 'array' && Array.isArray(defaultVal)) {
            defaultVal = defaultVal.join('\n')
          }
          configForm.value[field.name] = defaultVal
        }
      }
    }

    // 尝试获取当前配置并覆盖
    try {
      const configRes = await getPluginConfig(row.name)
      const config = configRes.config || configRes
      for (const field of pluginConfigSchema.value?.fields || []) {
        if (config[field.name] !== undefined) {
          if (field.type === 'array' && Array.isArray(config[field.name])) {
            configForm.value[field.name] = config[field.name].join('\n')
          } else {
            configForm.value[field.name] = config[field.name]
          }
        }
      }
    } catch (e) {
      // 获取配置失败，保持默认值
    }
  } catch (error: any) {
    console.error('Failed to load plugin schema:', error)
    ElMessage.error(t('evaluation.loadPluginConfigFailed') + ': ' + error.message)
    return
  }

  configDialogVisible.value = true
}

// 保存配置
async function handleSaveConfig() {
  try {
    configSaving.value = true

    // 将 array 类型的文本转换回数组
    const configToSave = { ...configForm.value }
    for (const field of pluginConfigSchema.value?.fields || []) {
      if (field.type === 'array' && typeof configToSave[field.name] === 'string') {
        const text = configToSave[field.name] as string
        configToSave[field.name] = text.split('\n').map(s => s.trim()).filter(s => s)
      }
    }

    await updatePluginConfig(currentPlugin.value.name, configToSave)
    ElMessage.success(t('evaluation.configSaved'))
    configDialogVisible.value = false
    await loadPlugins()
  } catch (error: any) {
    console.error('Failed to save plugin config:', error)
    ElMessage.error(t('evaluation.saveFailed') + ': ' + error.message)
  } finally {
    configSaving.value = false
  }
}

// 测试评估
function handleTest() {
  testResult.value = null
  testDialogVisible.value = true
}

// 运行测试
async function handleRunTest() {
  try {
    testRunning.value = true
    const res = await testEvaluation({
      question: testForm.value.question,
      answer: testForm.value.answer,
      history_messages: []
    })
    testResult.value = res
    ElMessage.success(t('evaluation.evalTestComplete'))
  } catch (error: any) {
    console.error('Failed to run test:', error)
    ElMessage.error(t('evaluation.testFailed') + ': ' + error.message)
  } finally {
    testRunning.value = false
  }
}

// 组件挂载
onMounted(() => {
  load()
})
</script>

<style scoped>
.evaluation {
  width: 100%;
  padding: 0 0 24px;
}

.config-empty {
  padding: 20px 0;
}

.plugin-info {
  margin-bottom: 16px;
}

.config-container {
  display: flex;
  flex-direction: column;
  gap: 12px;
  max-height: 420px;
  overflow-y: auto;
  padding-right: 4px;
}

.config-row {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

/* 每行最多2列网格布局，用于数字参数 */
.config-row-grid2 {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 10px;
}

.config-item {
  background: #f5f7fa;
  border-radius: 6px;
  padding: 10px 12px;
}

.config-item-full {
  width: 100%;
}

.config-item-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 6px;
}

.config-item-label {
  font-weight: 500;
  font-size: 13px;
  color: #303133;
}

.config-item-name {
  font-size: 11px;
  color: #909399;
  font-family: monospace;
}

.config-item-content {
  display: flex;
  align-items: center;
  gap: 8px;
}

.config-item-content .el-input-number,
.config-item-content .el-input,
.config-item-content .el-switch {
  flex: 1;
}

.config-item-content .el-textarea {
  flex: 1;
}

.config-item-hint {
  font-size: 11px;
  color: #909399;
  white-space: nowrap;
  min-width: 60px;
}

.header {
  margin-bottom: 24px;
}

.page-title {
  font-size: 24px;
  font-weight: 600;
  margin: 0 0 8px 0;
  color: #303133;
}

.page-description {
  font-size: 14px;
  color: #909399;
  margin: 0;
}

.content-wrapper {
  margin-top: 16px;
}

.left-panel {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.info-card,
.plugins-card,
.chart-card {
  margin-bottom: 0;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.card-title {
  font-size: 16px;
  font-weight: 600;
}

.card-actions {
  margin-top: 16px;
}

.chart-container {
  text-align: center;
}

.percentage-value {
  font-size: 28px;
  font-weight: 600;
}

.chart-legend {
  margin-top: 16px;
}

.legend-item {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  margin: 4px 0;
}

.legend-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.legend-dot.success {
  background-color: #67c23a;
}

.legend-dot.warning {
  background-color: #e6a23c;
}

.legend-text {
  font-size: 14px;
  color: #606266;
}

.success-text {
  color: #67c23a;
  font-weight: 600;
}

.warning-text {
  color: #e6a23c;
  font-weight: 600;
}

.sort-hint {
  margin-top: 12px;
  padding: 12px;
  background-color: #f4f4f5;
  border-radius: 4px;
  font-size: 13px;
  color: #909399;
  display: flex;
  align-items: center;
  gap: 8px;
}

.test-result {
  margin-top: 16px;
}

.details-json {
  background-color: #f5f7fa;
  padding: 12px;
  border-radius: 4px;
  font-size: 12px;
  max-height: 200px;
  overflow-y: auto;
}
</style>

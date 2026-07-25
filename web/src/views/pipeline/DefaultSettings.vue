<template>
  <div class="pipeline-default-settings">
    <div class="page-header">
      <h2>{{ t('defaultSettings.title') }}</h2>
      <p class="description">{{ t('defaultSettings.subtitle') }}</p>
    </div>

    <el-card v-loading="loading">
      <el-form :model="form" label-width="160px" style="max-width: 500px">
        <el-form-item :label="t('defaultSettings.systemDefaultPipeline')">
          <el-select v-model="form.default_pipeline_id" :placeholder="t('defaultSettings.selectDefaultPipeline')" style="width: 100%">
            <el-option
              v-for="pipeline in availablePipelines"
              :key="pipeline.id"
              :label="pipeline.name"
              :value="pipeline.id"
            >
              <span>{{ pipeline.name }}</span>
              <span style="color: #999; margin-left: 8px; font-size: 12px">{{ pipeline.description }}</span>
            </el-option>
          </el-select>
        </el-form-item>

        <el-form-item :label="t('defaultSettings.allowUserOverride')">
          <el-switch v-model="form.allow_user_override" />
          <span style="margin-left: 8px; color: #999; font-size: 12px">
            {{ t('defaultSettings.allowUserOverrideDesc') }}
          </span>
        </el-form-item>

        <el-form-item>
          <el-button type="primary" :loading="saving" @click="handleSave">
            {{ t('defaultSettings.saveConfig') }}
          </el-button>
          <el-button @click="loadDefaults">{{ t('defaultSettings.reset') }}</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card style="margin-top: 20px">
      <template #header>
        <span>{{ t('defaultSettings.currentEffectiveConfig') }}</span>
      </template>
      <el-descriptions :column="1" border>
        <el-descriptions-item :label="t('defaultSettings.defaultPipeline')">
          <el-tag>{{ getPipelineName(form.default_pipeline_id) }}</el-tag>
        </el-descriptions-item>
        <el-descriptions-item :label="t('defaultSettings.allowUserOverride')">
          <el-tag :type="form.allow_user_override ? 'success' : 'info'">
            {{ form.allow_user_override ? t('defaultSettings.yes') : t('defaultSettings.no') }}
          </el-tag>
        </el-descriptions-item>
      </el-descriptions>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import {
  getPipelineDefaults,
  updatePipelineDefaults,
  AVAILABLE_PIPELINES,
  type PipelineDefaults
} from '@/api/pipeline'

const { t } = useI18n()
const loading = ref(false)
const saving = ref(false)

const form = ref<PipelineDefaults>({
  default_pipeline_id: 'smart-scheduling',
  allow_user_override: true
})

const availablePipelines = AVAILABLE_PIPELINES

const getPipelineName = (id: string) => {
  const pipeline = availablePipelines.find(p => p.id === id)
  return pipeline ? pipeline.name : id
}

const loadDefaults = async () => {
  loading.value = true
  try {
    const res = await getPipelineDefaults()
    if (res.data) {
      form.value = {
        default_pipeline_id: res.data.default_pipeline_id || 'smart-scheduling',
        allow_user_override: res.data.allow_user_override ?? true
      }
    }
  } catch (error) {
    console.error('Failed to load defaults:', error)
  } finally {
    loading.value = false
  }
}

const handleSave = async () => {
  saving.value = true
  try {
    await updatePipelineDefaults({
      default_pipeline_id: form.value.default_pipeline_id
    })
    ElMessage.success(t('defaultSettings.configSaved'))
  } catch (error) {
    ElMessage.error(t('defaultSettings.saveFailed'))
    console.error('Failed to save defaults:', error)
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  loadDefaults()
})
</script>

<style scoped>
.pipeline-default-settings {
  width: 100%;
  padding: 0 0 24px;
}

.page-header {
  margin-bottom: 20px;
}

.page-header h2 {
  margin: 0 0 8px 0;
}

.page-header .description {
  color: #666;
  margin: 0;
  font-size: 14px;
}
</style>

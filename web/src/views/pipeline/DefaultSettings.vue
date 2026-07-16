<template>
  <div class="pipeline-default-settings">
    <div class="page-header">
      <h2>默认流水线配置</h2>
      <p class="description">配置系统默认使用的流水线，当请求未指定模式时将使用此流水线</p>
    </div>

    <el-card v-loading="loading">
      <el-form :model="form" label-width="160px" style="max-width: 500px">
        <el-form-item label="系统默认流水线">
          <el-select v-model="form.default_pipeline_id" placeholder="选择默认流水线" style="width: 100%">
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

        <el-form-item label="允许用户覆盖">
          <el-switch v-model="form.allow_user_override" />
          <span style="margin-left: 8px; color: #999; font-size: 12px">
            启用后，用户可以在个人中心设置自己的默认流水线
          </span>
        </el-form-item>

        <el-form-item>
          <el-button type="primary" :loading="saving" @click="handleSave">
            保存配置
          </el-button>
          <el-button @click="loadDefaults">重置</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card style="margin-top: 20px">
      <template #header>
        <span>当前生效配置</span>
      </template>
      <el-descriptions :column="1" border>
        <el-descriptions-item label="默认流水线">
          <el-tag>{{ getPipelineName(form.default_pipeline_id) }}</el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="允许用户覆盖">
          <el-tag :type="form.allow_user_override ? 'success' : 'info'">
            {{ form.allow_user_override ? '是' : '否' }}
          </el-tag>
        </el-descriptions-item>
      </el-descriptions>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import {
  getPipelineDefaults,
  updatePipelineDefaults,
  AVAILABLE_PIPELINES,
  type PipelineDefaults
} from '@/api/pipeline'

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
    ElMessage.success('配置已保存')
  } catch (error) {
    ElMessage.error('保存失败')
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
  padding: 20px;
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

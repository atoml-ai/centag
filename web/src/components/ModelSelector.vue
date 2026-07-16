<template>
  <el-select
    v-model="selectedModel"
    :placeholder="placeholder"
    :disabled="disabled || !backendId"
    :loading="loading"
    :filterable="filterable"
    :allow-create="allowCreate"
    :default-first-option="defaultFirstOption"
    clearable
    style="width: 100%"
    @change="handleModelChange"
    @visible-change="handleVisibleChange"
  >
    <el-option
      v-for="model in modelOptions"
      :key="model"
      :label="model"
      :value="model"
    >
      <div class="model-option">
        <span class="model-name">{{ model }}</span>
        <el-tag 
          v-if="isRecommendedModel(model)" 
          size="small" 
          type="success"
          style="margin-left: 8px"
        >
          推荐
        </el-tag>
      </div>
    </el-option>
    
    <template #empty>
      <div class="model-select-empty">
        <el-empty 
          v-if="!backendId"
          :image-size="50"
          description="请先选择后端"
        />
        <el-empty 
          v-else-if="loading"
          :image-size="50"
          description="加载中..."
        />
        <el-empty 
          v-else
          :image-size="50"
          :description="modelOptions.length === 0 ? '该后端暂无模型' : '未找到匹配的模型'"
        >
          <el-button 
            v-if="backendId && modelOptions.length === 0"
            type="primary" 
            size="small" 
            @click="fetchModels"
          >
            获取模型
          </el-button>
        </el-empty>
      </div>
    </template>
  </el-select>
</template>

<script setup lang="ts">
import { ref, onMounted, watch, computed } from 'vue'
import { ElMessage } from 'element-plus'
import api from '@/api'

// Props
interface Props {
  modelValue?: string | null
  backendId?: string | null
  placeholder?: string
  disabled?: boolean
  filterable?: boolean
  allowCreate?: boolean
  defaultFirstOption?: boolean
  autoLoad?: boolean
  recommendedModels?: string[]  // 推荐模型列表
}

const props = withDefaults(defineProps<Props>(), {
  modelValue: null,
  backendId: null,
  placeholder: '选择模型',
  disabled: false,
  filterable: true,
  allowCreate: true,
  defaultFirstOption: true,
  autoLoad: true,
  recommendedModels: () => [],
})

// Emits
const emit = defineEmits<{
  'update:modelValue': [value: string | null]
  'change': [model: string | null]
  'load': [models: string[]]
}>()

// State
const selectedModel = ref<string | null>(props.modelValue)
const modelOptions = ref<string[]>([])
const loading = ref(false)
const loadedBackends = ref<Set<string>>(new Set())

// 判断是否是推荐模型
const isRecommendedModel = (model: string) => {
  return props.recommendedModels?.includes(model) || false
}

// 加载后端模型
const loadModels = async (backendId: string) => {
  if (!backendId) {
    return
  }

  loading.value = true
  try {
    const res = await api.get(`/api/v1/backends/${backendId}/models`)
    // axios 拦截器已解包 {success: true, data: [...]} → res 已经是数组
    const models = Array.isArray(res) ? res : []

    if (Array.isArray(models) && models.length > 0) {
      modelOptions.value = models
      loadedBackends.value.add(backendId)
      emit('load', models)
    } else {
      modelOptions.value = []
    }
  } catch (error: any) {
    modelOptions.value = []
    console.warn('Failed to load models for backend:', backendId, error)
  } finally {
    loading.value = false
  }
}

// 手动获取模型
const fetchModels = async () => {
  if (!props.backendId) return
  await loadModels(props.backendId)
}

// 处理模型选择变化
const handleModelChange = (model: string | null) => {
  emit('update:modelValue', model)
  emit('change', model)
}

// 处理下拉框显示/隐藏
const handleVisibleChange = (visible: boolean) => {
  if (visible && props.backendId && props.autoLoad) {
    loadModels(props.backendId)
  }
}

// 监听后端 ID 变化
watch(() => props.backendId, (newBackendId) => {
  if (newBackendId && props.autoLoad) {
    loadModels(newBackendId)
  } else if (!newBackendId) {
    modelOptions.value = []
    selectedModel.value = null
  }
})

// 监听外部值变化
watch(() => props.modelValue, (newVal) => {
  if (newVal !== selectedModel.value) {
    selectedModel.value = newVal
  }
  // 如果有值但模型列表为空，且后端已设置，自动加载
  if (newVal && props.backendId && modelOptions.value.length === 0 && props.autoLoad) {
    loadModels(props.backendId)
  }
})

// 初始化
onMounted(() => {
  if (props.backendId && props.autoLoad) {
    loadModels(props.backendId)
  }
})

// 暴露方法给父组件
defineExpose({
  loadModels,
  fetchModels,
  modelOptions,
})
</script>

<style scoped>
.model-option {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.model-name {
  flex: 1;
  font-family: monospace;
}

.model-select-empty {
  padding: 12px 0;
}
</style>

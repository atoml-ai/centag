<template>
  <el-select
    v-model="selectedBackendId"
    :placeholder="placeholder"
    :disabled="disabled"
    :loading="loading"
    :filterable="filterable"
    clearable
    style="width: 100%"
    @change="handleBackendChange"
  >
    <el-option
      v-for="backend in backends"
      :key="backend.id"
      :label="backend.name"
      :value="backend.id"
      :disabled="!backend.enabled && !includeDisabled"
    >
      <div class="backend-option">
        <span class="backend-name">{{ backend.name }}</span>
        <span class="backend-id">{{ backend.id }}</span>
        <el-tag 
          v-if="!backend.enabled" 
          size="small" 
          type="info"
          style="margin-left: 8px"
        >
          已禁用
        </el-tag>
        <el-tag 
          v-if="backend.type === 'ollama'" 
          size="small" 
          type="success"
          style="margin-left: 8px"
        >
          本地
        </el-tag>
      </div>
    </el-option>
    
    <template #empty>
      <el-empty 
        :image-size="60"
        :description="includeDisabled ? '暂无后端配置' : '暂无启用的后端'"
      >
        <el-button type="primary" size="small" @click="goToBackendManagement">
          去配置后端
        </el-button>
      </el-empty>
    </template>
  </el-select>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { ElMessage } from 'element-plus'
import api from '@/api'

// Props
interface Props {
  modelValue?: string | null
  placeholder?: string
  disabled?: boolean
  filterable?: boolean
  includeDisabled?: boolean  // 是否包含禁用的后端
  autoLoad?: boolean        // 是否自动加载后端列表
  filter?: (backend: any) => boolean  // 自定义筛选函数
}

const props = withDefaults(defineProps<Props>(), {
  modelValue: null,
  placeholder: '选择后端',
  disabled: false,
  filterable: true,
  includeDisabled: false,
  autoLoad: true,
})

// Emits
const emit = defineEmits<{
  'update:modelValue': [value: string | null]
  'change': [backendId: string | null, backend: any | null]
  'load': [backends: any[]]
}>()

// State
const selectedBackendId = ref<string | null>(props.modelValue)
const backends = ref<any[]>([])
const loading = ref(false)

// 加载后端列表
const loadBackends = async () => {
  loading.value = true
  try {
    const res = await api.get('/api/v1/backends')
    // axios 拦截器已解包 {success: true, data: [...]} → res 已经是数组
    let allBackends = Array.isArray(res) ? res : []
    
    // 1. 如果不包含禁用的后端，先过滤
    if (!props.includeDisabled) {
      allBackends = allBackends.filter((b: any) => b.enabled === true)
    }
    
    // 2. 如果有自定义筛选函数，应用它
    if (props.filter) {
      allBackends = allBackends.filter((b: any) => props.filter!(b))
    }
    
    backends.value = allBackends
    emit('load', backends.value)
  } catch (error: any) {
    ElMessage.error('加载后端列表失败：' + error.message)
    backends.value = []
  } finally {
    loading.value = false
  }
}

// 处理后端选择变化
const handleBackendChange = (backendId: string | null) => {
  emit('update:modelValue', backendId)
  
  const selectedBackend = backends.value.find(b => b.id === backendId) || null
  emit('change', backendId, selectedBackend)
}

// 跳转到后端管理页面
const goToBackendManagement = () => {
  // 使用 router 跳转，如果没有 router 则提示
  try {
    window.location.href = '/static/#/backends'
  } catch (e) {
    ElMessage.info('请前往后端管理页面配置后端')
  }
}

// 监听外部值变化
watch(() => props.modelValue, (newVal) => {
  if (newVal !== selectedBackendId.value) {
    selectedBackendId.value = newVal
  }
})

// 初始化
onMounted(() => {
  if (props.autoLoad) {
    loadBackends()
  }
})

// 暴露方法给父组件
defineExpose({
  loadBackends,
  backends,
})
</script>

<style scoped>
.backend-option {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.backend-name {
  flex: 1;
  font-weight: 500;
}

.backend-id {
  font-size: 12px;
  color: #848484;
  font-family: monospace;
}
</style>

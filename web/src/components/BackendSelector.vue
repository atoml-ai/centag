<template>
  <el-select
    v-model="selectedBackendId"
    :placeholder="placeholder || t('backendSelector.placeholder')"
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
          {{ t('backendSelector.disabled') }}
        </el-tag>
        <el-tag 
          v-if="backend.type === 'ollama'" 
          size="small" 
          type="success"
          style="margin-left: 8px"
        >
          {{ t('backendSelector.local') }}
        </el-tag>
      </div>
    </el-option>
    
    <template #empty>
      <el-empty 
        :image-size="60"
        :description="includeDisabled ? t('backendSelector.emptyDescAll') : t('backendSelector.emptyDescEnabled')"
      >
        <el-button type="primary" size="small" @click="goToBackendManagement">
          {{ t('backendSelector.goConfig') }}
        </el-button>
      </el-empty>
    </template>
  </el-select>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import api from '@/api'

const { t } = useI18n()

interface Props {
  modelValue?: string | null
  placeholder?: string
  disabled?: boolean
  filterable?: boolean
  includeDisabled?: boolean
  autoLoad?: boolean
  filter?: (backend: any) => boolean
}

const props = withDefaults(defineProps<Props>(), {
  modelValue: null,
  placeholder: '',
  disabled: false,
  filterable: true,
  includeDisabled: false,
  autoLoad: true,
})

const emit = defineEmits<{
  'update:modelValue': [value: string | null]
  'change': [backendId: string | null, backend: any | null]
  'load': [backends: any[]]
}>()

const selectedBackendId = ref<string | null>(props.modelValue)
const backends = ref<any[]>([])
const loading = ref(false)

const loadBackends = async () => {
  loading.value = true
  try {
    const res = await api.get('/api/v1/backends')
    let allBackends = Array.isArray(res) ? res : []
    
    if (!props.includeDisabled) {
      allBackends = allBackends.filter((b: any) => b.enabled === true)
    }
    
    if (props.filter) {
      allBackends = allBackends.filter((b: any) => props.filter!(b))
    }
    
    backends.value = allBackends
    emit('load', backends.value)
  } catch (error: any) {
    ElMessage.error(t('backendSelector.loadFailed', { msg: error.message }))
    backends.value = []
  } finally {
    loading.value = false
  }
}

const handleBackendChange = (backendId: string | null) => {
  emit('update:modelValue', backendId)
  
  const selectedBackend = backends.value.find(b => b.id === backendId) || null
  emit('change', backendId, selectedBackend)
}

const goToBackendManagement = () => {
  try {
    window.location.href = '/static/#/backends'
  } catch (e) {
    ElMessage.info(t('backendSelector.navigateHint'))
  }
}

watch(() => props.modelValue, (newVal) => {
  if (newVal !== selectedBackendId.value) {
    selectedBackendId.value = newVal
  }
})

onMounted(() => {
  if (props.autoLoad) {
    loadBackends()
  }
})

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

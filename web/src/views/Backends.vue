<template>
  <div class="backends-page">
    <div class="hermes-header">
      <div class="hermes-header-left">
        <h1 class="hermes-title">后端网关配置</h1>
        <p class="hermes-subtitle">管理 LLM 后端服务，配置路由与健康检查</p>
      </div>
    </div>

    <div class="backends-body" v-loading="loading">
      <ProviderManagerPanel
        :backends="providers"
        @refresh="fetchProviders"
        @backend-updated="patchProvider"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import ProviderManagerPanel from '@/components/backends/ProviderManagerPanel.vue'
import api from '@/api'

const loading = ref(false)
const providers = ref<any[]>([])

onMounted(() => {
  fetchProviders()
})

function fetchProviders() {
  loading.value = true
  api
    .get('/api/v1/backends')
    .then((res: any) => {
      providers.value = Array.isArray(res) ? res : Array.isArray(res?.data) ? res.data : []
    })
    .catch((err) => {
      console.error('Failed to fetch backends', err)
      ElMessage.error('获取后端列表失败')
    })
    .finally(() => {
      loading.value = false
    })
}

function patchProvider(updated: any) {
  if (!updated?.id) {
    fetchProviders()
    return
  }
  const idx = providers.value.findIndex((b) => b.id === updated.id)
  if (idx >= 0) {
    providers.value[idx] = { ...providers.value[idx], ...updated }
  } else {
    fetchProviders()
  }
}
</script>

<style scoped>
.backends-page {
  width: 100%;
  max-width: none;
  margin: 0;
  padding: 24px 20px 40px;
}

.hermes-header {
  margin-bottom: 16px;
}

.hermes-title {
  margin: 0 0 6px;
  font-size: 22px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.hermes-subtitle {
  margin: 0;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.backends-body {
  min-height: 200px;
}
</style>

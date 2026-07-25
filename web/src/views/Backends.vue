<template>
  <div class="backends-page">
    <div class="hermes-header">
      <div class="hermes-header-left">
        <h1 class="hermes-title">{{ $t('backends.title') }}</h1>
        <p class="hermes-subtitle">{{ $t('backends.subtitle') }}</p>
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
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import ProviderManagerPanel from '@/components/backends/ProviderManagerPanel.vue'
import api from '@/api'

const { t } = useI18n()
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
      ElMessage.error(t('backends.fetchFailed'))
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
  padding: 0 0 24px;
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

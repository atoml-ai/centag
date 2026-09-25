<template>
  <div class="free-discovery" v-loading="loading">
    <div class="fd-header">
      <div class="fd-title-wrap">
        <span class="fd-badge">FREE</span>
        <div>
          <div class="fd-title">{{ t('backends.freeProviders.title') }}</div>
          <div class="fd-subtitle">{{ t('backends.freeProviders.subtitle') }}</div>
        </div>
      </div>
      <el-button
        v-if="canWrite"
        type="success"
        :loading="scanning"
        @click="handleScanAll"
      >
        <el-icon><MagicStick /></el-icon>
        {{ t('backends.freeProviders.scanAll') }}
      </el-button>
    </div>

    <div class="fd-tabs">
      <el-radio-group v-model="authFilter" size="small" @change="fetchCatalog">
        <el-radio-button label="all">{{ t('backends.freeProviders.tabAll') }}</el-radio-button>
        <el-radio-button label="keyless">{{ t('backends.freeProviders.tabKeyless') }}</el-radio-button>
        <el-radio-button label="apikey">{{ t('backends.freeProviders.tabKeyed') }}</el-radio-button>
      </el-radio-group>
      <span class="fd-count">{{ providers.length }} {{ t('backends.freeProviders.items') }}</span>
    </div>

    <div v-if="providers.length" class="fd-grid">
      <div
        v-for="p in providers"
        :key="p.id"
        class="fd-card"
        :class="{ 'is-added': registeredPlatforms.has(p.id) }"
      >
        <div class="fd-card-head">
          <span class="fd-name">{{ p.name }}</span>
          <el-tag
            :type="p.auth_type === 'keyless' ? 'success' : 'warning'"
            size="small"
            effect="light"
          >
            {{ p.auth_type === 'keyless' ? t('backends.freeProviders.authKeyless') : t('backends.freeProviders.authApikey') }}
          </el-tag>
        </div>

        <div class="fd-models-label">{{ t('backends.freeProviders.modelsLabel') }}</div>
        <div class="fd-models">
          <el-tag
            v-for="m in p.known_free_models"
            :key="m"
            size="small"
            type="info"
            effect="plain"
            class="fd-model-tag"
          >{{ m }}</el-tag>
          <span v-if="!p.known_free_models?.length" class="fd-muted">—</span>
        </div>

        <div v-if="p.rate_limit_notes" class="fd-rate">
          <el-icon><InfoFilled /></el-icon>
          <span>{{ p.rate_limit_notes }}</span>
        </div>

        <div class="fd-card-foot">
          <a v-if="p.doc_url" :href="p.doc_url" target="_blank" rel="noopener" class="fd-doc">
            <el-icon><Link /></el-icon>{{ t('backends.freeProviders.docLink') }}
          </a>
          <el-button
            v-if="canWrite"
            size="small"
            :type="registeredPlatforms.has(p.id) ? 'info' : (p.auth_type === 'keyless' ? 'success' : 'primary')"
            :disabled="registeredPlatforms.has(p.id)"
            :loading="addingMap[p.id]"
            @click="handleAdd(p)"
          >
            {{ registeredPlatforms.has(p.id) ? t('backends.freeProviders.added') : t('backends.freeProviders.add') }}
          </el-button>
        </div>
      </div>
    </div>

    <div v-else-if="!loading" class="fd-empty">
      {{ t('backends.freeProviders.empty') }}
    </div>

    <el-dialog
      v-model="keyDialog.visible"
      :title="t('backends.freeProviders.addTitle')"
      width="420px"
      @closed="keyDialog.key = ''"
    >
      <div class="fd-dialog-name">{{ keyDialog.name }}</div>
      <el-form label-position="top">
        <el-form-item :label="t('backends.freeProviders.apiKeyLabel')">
          <el-input
            v-model="keyDialog.key"
            type="password"
            show-password
            :placeholder="t('backends.freeProviders.apiKeyPlaceholder')"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="keyDialog.visible = false">{{ t('backends.freeProviders.cancel') }}</el-button>
        <el-button
          type="primary"
          :loading="addingMap[keyDialog.platform]"
          :disabled="!keyDialog.key"
          @click="confirmKeyedAdd"
        >{{ t('backends.freeProviders.add') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { MagicStick, InfoFilled, Link } from '@element-plus/icons-vue'
import api from '@/api'
import { useUserResourceAccess } from '@/composables/useUserResourceAccess'

interface FreeProvider {
  id: string
  name: string
  base_url: string
  auth_type: string
  type: string
  known_free_models: string[]
  probe_model?: string
  rate_limit_notes?: string
  doc_url?: string
  tags?: string[]
}

const { t } = useI18n()
const { canAddOwnBackends } = useUserResourceAccess()
const canWrite = computed(() => canAddOwnBackends.value)

const emit = defineEmits<{ registered: [] }>()

const loading = ref(false)
const scanning = ref(false)
const providers = ref<FreeProvider[]>([])
const authFilter = ref<'all' | 'keyless' | 'apikey'>('all')
const registeredPlatforms = reactive<Set<string>>(new Set())
const addingMap = reactive<Record<string, boolean>>({})

const keyDialog = reactive<{ visible: boolean; platform: string; name: string; key: string }>({
  visible: false,
  platform: '',
  name: '',
  key: ''
})

function authQuery(): string {
  if (authFilter.value === 'all') return ''
  return authFilter.value
}

async function fetchCatalog() {
  loading.value = true
  try {
    const qs = authQuery()
    const res: any = await api.get('/api/v1/backends/free' + (qs ? `?auth_type=${qs}` : ''))
    const body = res?.data ?? res
    providers.value = Array.isArray(body?.providers) ? body.providers : []
  } catch (err: any) {
    ElMessage.error(err?.response?.data?.message || t('backends.freeProviders.fetchFailed'))
    providers.value = []
  } finally {
    loading.value = false
  }
}

function markRegistered(platform: string) {
  registeredPlatforms.add(platform)
}

async function registerOne(platform: string, apiKey = '') {
  addingMap[platform] = true
  try {
    const res: any = await api.post('/api/v1/backends/free/register', {
      platform,
      api_key: apiKey
    })
    const body = res?.data ?? res
    const name = body?.name || platform
    ElMessage.success(t('backends.freeProviders.registerSuccess', { name }))
    markRegistered(platform)
    emit('registered')
    return true
  } catch (err: any) {
    ElMessage.error(t('backends.freeProviders.registerFailed', { msg: err?.response?.data?.message || err?.message || '' }))
    return false
  } finally {
    addingMap[platform] = false
  }
}

function handleAdd(p: FreeProvider) {
  if (registeredPlatforms.has(p.id)) return
  if (p.auth_type === 'keyless') {
    void registerOne(p.id, '')
  } else {
    keyDialog.platform = p.id
    keyDialog.name = p.name
    keyDialog.key = ''
    keyDialog.visible = true
  }
}

async function confirmKeyedAdd() {
  if (!keyDialog.key) return
  const ok = await registerOne(keyDialog.platform, keyDialog.key)
  if (ok) keyDialog.visible = false
}

async function handleScanAll() {
  scanning.value = true
  try {
    const res: any = await api.post('/api/v1/backends/free/scan')
    const body = res?.data ?? res
    const count: number = body?.count ?? 0
    const ids: string[] = Array.isArray(body?.registered) ? body.registered : []
    // 标记全部 keyless 平台为已添加
    for (const p of providers.value) {
      if (p.auth_type === 'keyless') markRegistered(p.id)
    }
    ElMessage.success(t('backends.freeProviders.scanAllDone', { count }))
    emit('registered')
  } catch (err: any) {
    ElMessage.error(t('backends.freeProviders.scanFailed', { msg: err?.response?.data?.message || err?.message || '' }))
  } finally {
    scanning.value = false
  }
}

onMounted(fetchCatalog)
</script>

<style scoped>
.free-discovery {
  display: flex;
  flex-direction: column;
  gap: 12px;
  width: 100%;
  padding: 16px;
  background: linear-gradient(135deg, #f0fff4 0%, #f8fafc 100%);
  border: 1px solid #d7f0dd;
  border-radius: 12px;
}

.fd-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.fd-title-wrap {
  display: flex;
  align-items: center;
  gap: 10px;
}

.fd-badge {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.5px;
  color: #047857;
  background: #d1fae5;
  border: 1px solid #6ee7b7;
  border-radius: 6px;
  padding: 3px 7px;
}

.fd-title {
  font-size: 16px;
  font-weight: 600;
  color: #065f46;
  line-height: 1.3;
}

.fd-subtitle {
  font-size: 12px;
  color: #6b7280;
  margin-top: 2px;
}

.fd-tabs {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  flex-wrap: wrap;
}

.fd-count {
  font-size: 12px;
  color: #6b7280;
}

.fd-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: 12px;
}

.fd-card {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px 14px;
  background: #ffffff;
  border: 1px solid #e6eef2;
  border-radius: 10px;
  transition: border-color 0.2s, box-shadow 0.2s;
}

.fd-card:hover {
  border-color: #9ae6b4;
  box-shadow: 0 2px 10px rgba(16, 185, 129, 0.08);
}

.fd-card.is-added {
  background: #f6fef9;
  border-color: #b3e6c8;
}

.fd-card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.fd-name {
  font-size: 0.9rem;
  font-weight: 600;
  color: #111827;
}

.fd-models-label {
  font-size: 11px;
  color: #9ca3af;
}

.fd-models {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  min-height: 22px;
}

.fd-model-tag {
  font-size: 11px;
}

.fd-rate {
  display: flex;
  align-items: flex-start;
  gap: 4px;
  font-size: 11px;
  color: #b45309;
  background: #fffbeb;
  border-radius: 6px;
  padding: 5px 7px;
  line-height: 1.4;
}

.fd-rate .el-icon {
  flex-shrink: 0;
  margin-top: 1px;
}

.fd-card-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-top: auto;
  padding-top: 6px;
  border-top: 1px solid rgba(15, 23, 42, 0.05);
}

.fd-doc {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  font-size: 12px;
  color: #2563eb;
  text-decoration: none;
}

.fd-doc:hover {
  text-decoration: underline;
}

.fd-muted {
  font-size: 12px;
  color: #9ca3af;
}

.fd-empty {
  text-align: center;
  color: #9ca3af;
  font-size: 0.8rem;
  padding: 16px 0;
}

.fd-dialog-name {
  font-size: 14px;
  font-weight: 600;
  color: #111827;
  margin-bottom: 12px;
}
</style>

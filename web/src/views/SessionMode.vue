<template>
  <div class="session-mode-page">
    <h1 class="page-title">
      <el-icon><Timer /></el-icon>
      {{ t('sessionMode.title') }}
    </h1>
    <p class="page-description">{{ t('sessionMode.description') }}</p>

    <div class="session-content">
      <!-- 当前会话状态 -->
      <el-card class="status-card">
        <template #header>
          <div class="card-header">
            <span>{{ t('sessionMode.currentSessionStatus') }}</span>
            <el-button 
              v-if="sessionMode"
              type="danger" 
              size="small" 
              @click="clearSession"
            >
              {{ t('sessionMode.clearSession') }}
            </el-button>
          </div>
        </template>
        
        <el-empty v-if="!sessionMode" :description="t('sessionMode.noSessionModeSet')" :image-size="80" />
        
        <el-descriptions v-else :column="1" border>
          <el-descriptions-item :label="t('sessionMode.modeKeyword')">
            <el-tag type="warning" size="large">{{ sessionMode.mode_key }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item :label="t('sessionMode.modeName')">
            {{ sessionMode.mode_name }}
          </el-descriptions-item>
          <el-descriptions-item :label="t('sessionMode.backendOverride')">
            {{ sessionMode.backend_id || t('sessionMode.none') }}
          </el-descriptions-item>
          <el-descriptions-item :label="t('sessionMode.modelOverride')">
            {{ sessionMode.model_name || t('sessionMode.none') }}
          </el-descriptions-item>
          <el-descriptions-item :label="t('sessionMode.remainingValidity')">
            <el-progress 
              :percentage="remainingPercent" 
              :status="remainingPercent < 20 ? 'exception' : 'success'"
            />
            <small>{{ formatTime(sessionMode.expires_in) }}</small>
          </el-descriptions-item>
        </el-descriptions>
      </el-card>

      <!-- 设置会话模式 -->
      <el-card class="form-card">
        <template #header>
          <span>{{ t('sessionMode.setSessionMode') }}</span>
        </template>
        
        <el-form :model="form" label-width="120px">
          <el-form-item :label="t('sessionMode.modeKeywordLabel')">
            <el-select 
              v-model="form.mode" 
              :placeholder="t('sessionMode.selectMode')"
              style="width: 100%"
            >
              <el-option 
                v-for="kw in keywords" 
                :key="kw.mode_key"
                :label="`${kw.mode_key} - ${kw.mode_name}`"
                :value="kw.mode_key"
                :disabled="!kw.is_enabled"
              />
            </el-select>
          </el-form-item>
          <el-form-item :label="t('sessionMode.specifyBackendOptional')">
            <el-select 
              v-model="form.backend" 
              :placeholder="t('sessionMode.leaveEmptyDefaultBackend')"
              clearable
              style="width: 100%"
            >
              <el-option 
                v-for="backend in backends" 
                :key="backend.id"
                :label="backend.name"
                :value="backend.id"
                :disabled="!backend.enabled"
              />
            </el-select>
          </el-form-item>
          <el-form-item :label="t('sessionMode.specifyModelOptional')">
            <el-input 
              v-model="form.model" 
              :placeholder="t('sessionMode.leaveEmptyDefaultModel')"
              clearable
            />
          </el-form-item>
          <el-form-item :label="t('sessionMode.validityPeriod')">
            <el-input-number 
              v-model="form.ttl" 
              :min="60" 
              :max="86400" 
              :step="60"
            />
            <span style="margin-left: 10px">{{ t('sessionMode.seconds') }}（{{ formatTime(form.ttl * 1000) }}）</span>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :loading="setting" @click="setSession">
              {{ t('sessionMode.setSessionModeBtn') }}
            </el-button>
          </el-form-item>
        </el-form>
      </el-card>

      <!-- 测试请求 -->
      <el-card class="test-card">
        <template #header>
          <span>{{ t('sessionMode.testRequest') }}</span>
        </template>
        
        <el-form label-width="100px">
          <el-form-item :label="t('sessionMode.testContent')">
            <el-input 
              v-model="testContent" 
              type="textarea" 
              :rows="4"
              :placeholder="t('sessionMode.testContentPlaceholder')"
            />
          </el-form-item>
          <el-form-item :label="t('sessionMode.testMethod')">
            <el-radio-group v-model="testMethod">
              <el-radio value="prefix">{{ t('sessionMode.contentPrefix') }}</el-radio>
              <el-radio value="header">{{ t('sessionMode.requestHeader') }}</el-radio>
              <el-radio value="body">{{ t('sessionMode.bodyExtension') }}</el-radio>
            </el-radio-group>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :loading="testing" @click="sendTestRequest">
              {{ t('sessionMode.sendTest') }}
            </el-button>
          </el-form-item>
        </el-form>

        <!-- 解析结果 -->
        <el-card v-if="testResult" class="result-card">
          <template #header>
            <span>{{ t('sessionMode.parseResult') }}</span>
          </template>
          <pre>{{ JSON.stringify(testResult, null, 2) }}</pre>
        </el-card>
      </el-card>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import api from '@/api'

const { t } = useI18n()

interface ModeKeyword {
  mode_key: string
  mode_name: string
  mode_type: string
  is_enabled: boolean
}

interface Backend {
  id: string
  name: string
  enabled: boolean
}

interface SessionMode {
  mode_key: string
  mode_name: string
  backend_id?: string
  model_name?: string
  expires_in: number
}

const loading = ref(false)
const setting = ref(false)
const testing = ref(false)

const keywords = ref<ModeKeyword[]>([])
const backends = ref<Backend[]>([])
const sessionMode = ref<SessionMode | null>(null)

const form = ref({
  mode: '',
  backend: '',
  model: '',
  ttl: 3600
})

const testContent = ref('')
const testMethod = ref('prefix')
const testResult = ref<any>(null)

const remainingPercent = computed(() => {
  if (!sessionMode.value) return 0
  const total = form.value.ttl * 1000
  const remaining = sessionMode.value.expires_in * 1000
  return Math.min(100, Math.max(0, (remaining / total) * 100))
})

const loadKeywords = async () => {
  try {
    const res = await api.get('/api/v1/proxy-modes')
    keywords.value = res || []
    
    // 默认选择第一个启用的模式
    const enabled = keywords.value.find(k => k.is_enabled)
    if (enabled && !form.value.mode) {
      form.value.mode = enabled.mode_key
    }
  } catch (error: any) {
    ElMessage.error(t('sessionMode.loadModeListFailed') + '：' + error.message)
  }
}

const loadBackends = async () => {
  try {
    const res = await api.get('/api/v1/backends')
    backends.value = (res || []).filter((b: any) => b.enabled)
  } catch (error: any) {
    console.warn('加载后端列表失败:', error)
  }
}

const loadSession = async () => {
  try {
    const res = await api.get('/api/v1/session/proxy-mode')
    sessionMode.value = res
  } catch (error: any) {
    sessionMode.value = null
  }
}

const setSession = async () => {
  if (!form.value.mode) {
    ElMessage.warning(t('sessionMode.pleaseSelectModeKeyword'))
    return
  }
  
  setting.value = true
  try {
    await api.post('/api/v1/session/proxy-mode', form.value)
    ElMessage.success(t('sessionMode.sessionModeSetSuccess'))
    loadSession()
  } catch (error: any) {
    ElMessage.error(t('sessionMode.sessionModeSetFailed') + '：' + error.message)
  } finally {
    setting.value = false
  }
}

const clearSession = async () => {
  try {
    await api.delete('/api/v1/session/proxy-mode')
    ElMessage.success(t('sessionMode.sessionModeCleared'))
    sessionMode.value = null
  } catch (error: any) {
    ElMessage.error(t('sessionMode.sessionClearFailed') + '：' + error.message)
  }
}

const sendTestRequest = async () => {
  if (!testContent.value) {
    ElMessage.warning(t('sessionMode.pleaseEnterTestContent'))
    return
  }
  
  testing.value = true
  testResult.value = null
  
  try {
    // 模拟解析请求
    const result = {
      original_content: testContent.value,
      method: testMethod.value,
      parsed: {} as any
    }
    
    if (testMethod.value === 'prefix') {
      // 模拟前缀解析
      const match = testContent.value.match(/^(#[a-zA-Z])\s+(.*)/)
      if (match) {
        result.parsed = {
          mode_key: match[1],
          content: match[2],
          source: 'content_prefix'
        }
      } else {
        result.parsed = {
          mode_key: null,
          content: testContent.value,
          source: 'none'
        }
      }
    } else if (testMethod.value === 'header') {
      result.parsed = {
        mode_key: form.value.mode || '#d',
        content: testContent.value,
        source: 'header'
      }
    } else {
      result.parsed = {
        mode_key: form.value.mode || '#d',
        content: testContent.value,
        source: 'body_extension'
      }
    }
    
    testResult.value = result
  } catch (error: any) {
    ElMessage.error(t('sessionMode.testFailed') + '：' + error.message)
  } finally {
    testing.value = false
  }
}

const formatTime = (ms: number) => {
  const seconds = Math.floor(ms / 1000)
  if (seconds < 60) return t('sessionMode.formatSeconds', { n: seconds })
  if (seconds < 3600) return t('sessionMode.formatMinutes', { n: Math.floor(seconds / 60) })
  return t('sessionMode.formatHours', { n: Math.floor(seconds / 3600) })
}

onMounted(() => {
  loadKeywords()
  loadBackends()
  loadSession()
})
</script>

<style scoped>
.session-mode-page {
  width: 100%;
  padding: 0 0 24px;
}

.page-title {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0 0 8px 0;
  font-size: 24px;
  font-weight: 600;
}

.page-description {
  margin: 0 0 20px 0;
  color: #666;
  font-size: 14px;
}

.session-content {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 20px;
}

.status-card {
  grid-column: 1 / -1;
}

.form-card, .test-card {
  min-height: 400px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-weight: 600;
}

.result-card {
  margin-top: 20px;
  background: #f5f7fa;
}

.result-card pre {
  background: #2d2d2d;
  color: #f8f8f2;
  padding: 15px;
  border-radius: 6px;
  overflow-x: auto;
  font-size: 13px;
  line-height: 1.5;
}

small {
  color: #999;
  margin-left: 10px;
}
</style>

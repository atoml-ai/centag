<template>
  <div class="session-mode-page">
    <h1 class="page-title">
      <el-icon><Timer /></el-icon>
      会话模式测试
    </h1>
    <p class="page-description">设置当前会话的代理模式，有效期内的所有请求都将使用该模式</p>

    <div class="session-content">
      <!-- 当前会话状态 -->
      <el-card class="status-card">
        <template #header>
          <div class="card-header">
            <span>📊 当前会话状态</span>
            <el-button 
              v-if="sessionMode"
              type="danger" 
              size="small" 
              @click="clearSession"
            >
              清除会话
            </el-button>
          </div>
        </template>
        
        <el-empty v-if="!sessionMode" description="当前未设置会话模式" :image-size="80" />
        
        <el-descriptions v-else :column="1" border>
          <el-descriptions-item label="模式关键字">
            <el-tag type="warning" size="large">{{ sessionMode.mode_key }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="模式名称">
            {{ sessionMode.mode_name }}
          </el-descriptions-item>
          <el-descriptions-item label="后端覆盖">
            {{ sessionMode.backend_id || '无' }}
          </el-descriptions-item>
          <el-descriptions-item label="模型覆盖">
            {{ sessionMode.model_name || '无' }}
          </el-descriptions-item>
          <el-descriptions-item label="剩余有效期">
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
          <span>⚙️ 设置会话模式</span>
        </template>
        
        <el-form :model="form" label-width="120px">
          <el-form-item label="模式关键字">
            <el-select 
              v-model="form.mode" 
              placeholder="选择模式"
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
          <el-form-item label="指定后端（可选）">
            <el-select 
              v-model="form.backend" 
              placeholder="留空则使用默认后端"
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
          <el-form-item label="指定模型（可选）">
            <el-input 
              v-model="form.model" 
              placeholder="留空则使用默认模型"
              clearable
            />
          </el-form-item>
          <el-form-item label="有效期">
            <el-input-number 
              v-model="form.ttl" 
              :min="60" 
              :max="86400" 
              :step="60"
            />
            <span style="margin-left: 10px">秒（{{ formatTime(form.ttl * 1000) }}）</span>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :loading="setting" @click="setSession">
              设置会话模式
            </el-button>
          </el-form-item>
        </el-form>
      </el-card>

      <!-- 测试请求 -->
      <el-card class="test-card">
        <template #header>
          <span>🧪 测试请求</span>
        </template>
        
        <el-form label-width="100px">
          <el-form-item label="测试内容">
            <el-input 
              v-model="testContent" 
              type="textarea" 
              :rows="4"
              placeholder="输入测试内容，如：#d 你好，请帮我写一个 Python 脚本"
            />
          </el-form-item>
          <el-form-item label="测试方式">
            <el-radio-group v-model="testMethod">
              <el-radio value="prefix">内容前缀</el-radio>
              <el-radio value="header">请求头</el-radio>
              <el-radio value="body">请求体扩展</el-radio>
            </el-radio-group>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :loading="testing" @click="sendTestRequest">
              发送测试
            </el-button>
          </el-form-item>
        </el-form>

        <!-- 解析结果 -->
        <el-card v-if="testResult" class="result-card">
          <template #header>
            <span>📋 解析结果</span>
          </template>
          <pre>{{ JSON.stringify(testResult, null, 2) }}</pre>
        </el-card>
      </el-card>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { ElMessage } from 'element-plus'
import api from '@/api'

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
    ElMessage.error('加载模式列表失败：' + error.message)
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
    ElMessage.warning('请选择模式关键字')
    return
  }
  
  setting.value = true
  try {
    await api.post('/api/v1/session/proxy-mode', form.value)
    ElMessage.success('会话模式设置成功')
    loadSession()
  } catch (error: any) {
    ElMessage.error('设置失败：' + error.message)
  } finally {
    setting.value = false
  }
}

const clearSession = async () => {
  try {
    await api.delete('/api/v1/session/proxy-mode')
    ElMessage.success('会话模式已清除')
    sessionMode.value = null
  } catch (error: any) {
    ElMessage.error('清除失败：' + error.message)
  }
}

const sendTestRequest = async () => {
  if (!testContent.value) {
    ElMessage.warning('请输入测试内容')
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
    ElMessage.error('测试失败：' + error.message)
  } finally {
    testing.value = false
  }
}

const formatTime = (ms: number) => {
  const seconds = Math.floor(ms / 1000)
  if (seconds < 60) return `${seconds}秒`
  if (seconds < 3600) return `${Math.floor(seconds / 60)}分钟`
  return `${Math.floor(seconds / 3600)}小时`
}

onMounted(() => {
  loadKeywords()
  loadBackends()
  loadSession()
})
</script>

<style scoped>
.session-mode-page {
  padding: 20px;
  max-width: 1200px;
  margin: 0 auto;
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

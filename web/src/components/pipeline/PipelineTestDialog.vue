<template>
  <el-dialog
    v-model="visible"
    :title="dialogTitle"
    width="680px"
    destroy-on-close
    class="pipeline-test-dialog"
    @closed="onClosed"
  >
    <p class="dialog-desc">{{ t('pipelineModes.test.desc') }}</p>

    <div class="pipeline-info" v-if="pipeline">
      <el-tag type="info" size="small" effect="plain">{{ pipeline.id }}</el-tag>
      <span class="pipeline-name">{{ pipeline.name }}</span>
    </div>

    <el-collapse class="curl-section">
      <el-collapse-item :title="t('pipelineModes.test.curlTitle')" name="curl">
        <div class="curl-box">
          <pre class="curl-code">{{ curlCommand }}</pre>
          <el-button type="primary" plain size="small" class="curl-copy-btn" @click="copyCurl">
            <el-icon style="margin-right: 4px"><CopyDocument /></el-icon>
            {{ t('pipelineModes.test.copyCurl') }}
          </el-button>
        </div>
      </el-collapse-item>
    </el-collapse>

    <el-input
      v-model="prompt"
      type="textarea"
      :rows="4"
      resize="vertical"
      :placeholder="t('pipelineModes.test.promptPlaceholder')"
      :disabled="testing"
    />

    <div class="test-actions">
      <el-button type="primary" :loading="testing" :disabled="!prompt.trim() || testing" @click="runTest">
        <el-icon v-if="!testing" style="margin-right: 4px"><Promotion /></el-icon>
        {{ t('pipelineModes.test.run') }}
      </el-button>
    </div>

    <el-card v-if="responseText || errorText || testing" shadow="never" class="result-card" :class="{ 'is-error': !!errorText }">
      <template #header>
        <div class="result-header">
          <span class="result-label">{{ t('pipelineModes.test.result') }}</span>
          <el-tag v-if="testing" type="warning" size="small" effect="plain">{{ t('pipelineModes.test.running') }}</el-tag>
          <el-tag v-else-if="errorText" type="danger" size="small">{{ t('pipelineModes.test.failed') }}</el-tag>
          <el-tag v-else type="success" size="small">{{ t('pipelineModes.test.success') }}</el-tag>
        </div>
      </template>
      <pre v-if="testing" class="result-text is-muted">{{ t('pipelineModes.test.waiting') }}</pre>
      <pre v-else-if="errorText" class="result-text is-error">{{ errorText }}</pre>
      <pre v-else class="result-text">{{ responseText }}</pre>
    </el-card>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Promotion, CopyDocument } from '@element-plus/icons-vue'
import { useAuthStore } from '@/stores/auth'
import type { Pipeline } from '@/api/pipeline'

const props = defineProps<{
  modelValue: boolean
  pipeline: Pipeline | null
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

const { t } = useI18n()
const authStore = useAuthStore()

const visible = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v)
})

const prompt = ref('')
const testing = ref(false)
const responseText = ref('')
const errorText = ref('')

const dialogTitle = computed(() => {
  const name = props.pipeline?.name || props.pipeline?.id || ''
  return t('pipelineModes.test.title', { name })
})

watch(
  () => props.modelValue,
  (v) => {
    if (v) {
      prompt.value = ''
      responseText.value = ''
      errorText.value = ''
    }
  }
)

function onClosed() {
  prompt.value = ''
  responseText.value = ''
  errorText.value = ''
}

const curlCommand = computed(() => {
  if (!props.pipeline) return ''
  const baseUrl = window.location.origin
  const token = authStore.accessToken || 'YOUR_API_KEY'
  const pipelineId = props.pipeline.id
  const testPrompt = prompt.value.trim() || 'Hello, introduce yourself'
  return `curl -X POST ${baseUrl}/v1/chat/completions \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer ${token}" \\
  -H "X-Pipeline-ID: ${pipelineId}" \\
  -d '{
    "model": "auto",
    "messages": [{"role": "user", "content": "${testPrompt.replace(/"/g, '\\"')}"}],
    "stream": false
  }'`
})

async function copyCurl() {
  const { copyToClipboard } = await import('@/utils/clipboard')
  if (await copyToClipboard(curlCommand.value)) {
    const { ElMessage } = await import('element-plus')
    ElMessage.success(t('pipelineModes.test.curlCopied'))
  }
}

async function runTest() {
  if (!props.pipeline) return
  const content = prompt.value.trim()
  if (!content) return
  testing.value = true
  responseText.value = ''
  errorText.value = ''
  try {
    const res = await fetch('/v1/chat/completions', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${authStore.accessToken || ''}`,
        'X-Pipeline-ID': props.pipeline.id
      },
      body: JSON.stringify({
        model: 'auto',
        messages: [{ role: 'user', content }],
        stream: false
      })
    })
    if (!res.ok) {
      const text = await res.text().catch(() => '')
      let detail = text
      try {
        const j = JSON.parse(text)
        detail = j?.error?.message || j?.error || j?.message || text
      } catch {
        /* keep raw text */
      }
      throw new Error(`${res.status}: ${String(detail).slice(0, 600)}`)
    }
    const data = await res.json()
    responseText.value =
      data?.choices?.[0]?.message?.content || data?.choices?.[0]?.text || JSON.stringify(data, null, 2)
  } catch (e: any) {
    errorText.value = e?.message || String(e)
  } finally {
    testing.value = false
  }
}
</script>

<style scoped>
.dialog-desc {
  margin: 0 0 12px;
  color: #64748b;
  font-size: 13px;
}

.pipeline-info {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
}

.pipeline-name {
  font-weight: 600;
  color: #1f2937;
}

.test-actions {
  margin-top: 12px;
  display: flex;
  justify-content: flex-end;
}

.result-card {
  margin-top: 16px;
  border-radius: 8px;
}

.result-card.is-error {
  border-color: #f56c6c;
}

.result-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.result-label {
  font-weight: 600;
  color: #1f2937;
}

.result-text {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 13px;
  line-height: 1.6;
  color: #1f2937;
  max-height: 320px;
  overflow: auto;
}

.result-text.is-error {
  color: #f56c6c;
}

.result-text.is-muted {
  color: #94a3b8;
}

.curl-section {
  margin-bottom: 12px;
  border: none;
}

.curl-section :deep(.el-collapse-item__header) {
  height: 32px;
  font-size: 13px;
  color: #64748b;
  background: transparent;
  border: none;
}

.curl-section :deep(.el-collapse-item__wrap) {
  border: none;
}

.curl-box {
  position: relative;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 6px;
  padding: 12px;
}

.curl-code {
  margin: 0;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
  line-height: 1.5;
  color: #334155;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 200px;
  overflow: auto;
}

.curl-copy-btn {
  margin-top: 8px;
}
</style>

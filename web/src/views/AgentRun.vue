<template>
  <div class="agent-run-page">
    <div class="page-header">
      <h1 class="page-title">{{ $t('agentRun.title') }}</h1>
      <p class="page-description">{{ $t('agentRun.subtitle') }}</p>
    </div>

    <el-alert
      v-if="blockedReason"
      type="warning"
      :closable="false"
      show-icon
      class="block-alert"
      :title="blockedReason"
    />

    <el-alert type="info" :closable="false" show-icon class="block-alert">
      {{ $t('agentRun.copyFirstHint') }}
    </el-alert>

    <section class="section">
      <h2 class="section-title">{{ $t('agentRun.presetsTitle') }}</h2>
      <p class="section-desc">{{ $t('agentRun.presetsDesc') }}</p>
      <div v-loading="loadingPresets" class="preset-grid">
        <button
          v-for="p in presets"
          :key="p.id"
          type="button"
          class="preset-card"
          :class="{ 'is-selected': selectedId === p.id }"
          :disabled="busy || !!blockedReason"
          @click="selectPreset(p)"
        >
          <span class="preset-name">{{ p.display_name }}</span>
          <span class="preset-desc">{{ p.description }}</span>
          <span class="preset-cmd">centag wrap run -- {{ p.argv.join(' ') }}</span>
        </button>
      </div>
    </section>

    <section class="section">
      <h2 class="section-title">{{ $t('agentRun.customTitle') }}</h2>
      <p class="section-desc">{{ $t('agentRun.customDesc') }}</p>
      <el-input
        v-model="customCommand"
        :placeholder="$t('agentRun.customPlaceholder')"
        :disabled="busy || !!blockedReason"
        @input="onCustomInput"
      />
    </section>

    <section v-if="previewCommand" class="section command-box">
      <h2 class="section-title">{{ $t('agentRun.commandTitle') }}</h2>
      <pre class="command-pre">{{ previewCommand }}</pre>
      <div class="command-actions">
        <el-button type="primary" :disabled="!!blockedReason" @click="copyCommand">
          {{ $t('agentRun.copy') }}
        </el-button>
        <el-button :loading="opening" :disabled="!!blockedReason" @click="openInTerminal">
          {{ $t('agentRun.openTerminal') }}
        </el-button>
      </div>
      <p v-if="lastOpenError" class="open-error">{{ lastOpenError }}</p>
    </section>

    <p class="hint">{{ $t('agentRun.desktopHint') }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { buildWrapRunCopyCommand, listWrapPresets, runWrapAgent, type WrapPreset } from '@/api/wrap'

const { t } = useI18n()

const presets = ref<WrapPreset[]>([])
const loadingPresets = ref(false)
const opening = ref(false)
const selectedId = ref('')
const customCommand = ref('')
const apiError = ref('')
const lastCommand = ref('')
const lastOpenError = ref('')

const busy = computed(() => loadingPresets.value || opening.value)
const blockedReason = computed(() => apiError.value || '')

const previewCommand = computed(() => {
  if (lastCommand.value) return lastCommand.value
  if (customCommand.value.trim()) {
    return buildWrapRunCopyCommand(customCommand.value.trim().split(/\s+/).filter(Boolean))
  }
  const p = presets.value.find((x) => x.id === selectedId.value)
  if (p) return buildWrapRunCopyCommand(p.argv)
  return ''
})

async function loadPresets() {
  loadingPresets.value = true
  apiError.value = ''
  try {
    const res = await listWrapPresets()
    presets.value = res.presets || []
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e)
    apiError.value = msg || t('agentRun.loadFailed')
    presets.value = []
  } finally {
    loadingPresets.value = false
  }
}

function selectPreset(p: WrapPreset) {
  selectedId.value = p.id
  customCommand.value = ''
  lastCommand.value = buildWrapRunCopyCommand(p.argv)
  lastOpenError.value = ''
}

function onCustomInput() {
  selectedId.value = ''
  lastCommand.value = ''
  lastOpenError.value = ''
}

async function copyCommand() {
  const cmd = previewCommand.value
  if (!cmd) return
  const { copyToClipboard } = await import('@/utils/clipboard')
  if (await copyToClipboard(cmd)) {
    ElMessage.success(t('agentRun.copied'))
  } else {
    ElMessage.error(t('agentRun.copyFailed'))
  }
}

async function openInTerminal() {
  opening.value = true
  lastOpenError.value = ''
  try {
    const body: { preset_id?: string; command?: string; open_terminal: boolean } = {
      open_terminal: true
    }
    if (selectedId.value) body.preset_id = selectedId.value
    else if (customCommand.value.trim()) body.command = customCommand.value.trim()
    else {
      ElMessage.warning(t('agentRun.customRequired'))
      return
    }
    const res = await runWrapAgent(body)
    lastCommand.value = res.user_command || res.command
    if (res.opened) {
      ElMessage.success(t('agentRun.terminalOpened'))
    } else {
      lastOpenError.value = res.open_error || res.hint || t('agentRun.terminalOpenFailed')
      ElMessage.warning(t('agentRun.terminalOpenFailedCopy'))
    }
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e)
    lastOpenError.value = msg
    ElMessage.error(msg || t('agentRun.launchFailed'))
  } finally {
    opening.value = false
  }
}

onMounted(() => {
  void loadPresets()
})
</script>

<style scoped>
.agent-run-page {
  width: 100%;
  padding: 0 0 32px;
}

.page-header {
  margin-bottom: 20px;
}

.page-title {
  margin: 0 0 8px;
  font-size: 1.5rem;
  font-weight: 600;
}

.page-description {
  margin: 0;
  color: var(--el-text-color-secondary);
  line-height: 1.5;
}

.block-alert {
  margin-bottom: 16px;
}

.section {
  margin-bottom: 24px;
}

.section-title {
  margin: 0 0 6px;
  font-size: 1.1rem;
  font-weight: 600;
}

.section-desc {
  margin: 0 0 14px;
  color: var(--el-text-color-secondary);
  font-size: 0.9rem;
}

.preset-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 12px;
  min-height: 80px;
}

.preset-card {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 4px;
  text-align: left;
  padding: 14px 14px 12px;
  border: 1px solid var(--el-border-color);
  border-radius: 8px;
  background: var(--el-bg-color);
  cursor: pointer;
  transition: border-color 0.15s, box-shadow 0.15s;
}

.preset-card:hover:not(:disabled),
.preset-card.is-selected {
  border-color: var(--el-color-primary);
  box-shadow: 0 0 0 1px var(--el-color-primary-light-7);
}

.preset-card:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.preset-name {
  font-weight: 600;
  font-size: 0.95rem;
}

.preset-desc {
  color: var(--el-text-color-secondary);
  font-size: 0.8rem;
  line-height: 1.35;
}

.preset-cmd {
  margin-top: 6px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.72rem;
  color: var(--el-text-color-regular);
  word-break: break-all;
}

.command-box {
  padding: 14px 16px;
  border: 1px solid var(--el-border-color);
  border-radius: 8px;
  background: var(--el-fill-color-blank);
}

.command-pre {
  margin: 0 0 12px;
  padding: 12px;
  border-radius: 6px;
  background: var(--el-fill-color-light);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.85rem;
  white-space: pre-wrap;
  word-break: break-all;
}

.command-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.open-error {
  margin: 10px 0 0;
  color: var(--el-color-warning);
  font-size: 0.85rem;
}

.hint {
  margin: 8px 0 0;
  font-size: 0.85rem;
  color: var(--el-text-color-secondary);
  line-height: 1.45;
}
</style>

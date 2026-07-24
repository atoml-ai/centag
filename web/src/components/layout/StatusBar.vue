<template>
  <div class="status-bar">
    <div class="status-left">
      <span class="status-item">
        <span class="status-dot" :class="statusClass"></span>
        {{ statusText }}
      </span>
      <span class="status-sep">·</span>
      <span class="status-item">{{ t('statusBar.version', { version: status.version || '--' }) }}</span>
      <span class="status-sep">·</span>
      <span class="status-item">{{ t('statusBar.uptime', { uptime: formatUptime(status.uptime) }) }}</span>
      <span class="status-sep">·</span>
      <span class="status-item">{{ t('statusBar.startedAt', { time: status.start_time || '--' }) }}</span>

      <span class="status-sep status-sep-group">|</span>
      <span
        class="status-item status-clickable"
        :title="backendId ? t('statusBar.backendTitle', { id: backendId }) : t('statusBar.backendTitleNone')"
        @click="router.push('/dashboard')"
      >
        <el-icon :size="12"><Cpu /></el-icon>
        <span>{{ t('statusBar.backendLabel') }}</span>
        <span class="mono truncate">{{ backendName || t('statusBar.backendNone') }}</span>
      </span>
      <span
        class="status-item status-clickable"
        :title="model ? t('statusBar.modelTitle', { model }) : t('statusBar.modelTitleNone')"
        @click="router.push('/dashboard')"
      >
        <el-icon :size="12"><Coin /></el-icon>
        <span>{{ t('statusBar.modelLabel') }}</span>
        <span class="mono truncate">{{ model || t('statusBar.modelNone') }}</span>
      </span>
      <span
        class="status-item status-clickable"
        :title="pipelineId ? t('statusBar.pipelineTitle', { id: pipelineId }) : t('statusBar.pipelineTitleNone')"
        @click="router.push('/dashboard')"
      >
        <el-icon :size="12"><Share /></el-icon>
        <span>{{ t('statusBar.pipelineLabel') }}</span>
        <span class="mono truncate">{{ pipelineName || t('statusBar.pipelineNone') }}</span>
      </span>

      <span class="status-sep status-sep-group">|</span>
      <span
        class="status-item status-clickable"
        :title="t('statusBar.totalCostTitle')"
        @click="goUsage"
      >
        <el-icon :size="12"><Money /></el-icon>
        <span>{{ t('statusBar.totalCost') }}</span>
        <span class="mono">{{ costText }}</span>
      </span>
      <span
        class="status-item status-clickable"
        :title="t('statusBar.totalTokensTitle')"
        @click="goUsage"
      >
        <el-icon :size="12"><DataLine /></el-icon>
        <span>{{ t('statusBar.totalTokens') }}</span>
        <span class="mono">{{ tokensText }}</span>
      </span>
    </div>
    <div class="status-right">
      <el-dropdown trigger="click" @command="(cmd) => handleLocaleChange(cmd as AppLocale)" size="small">
        <span class="status-item status-clickable">
          <el-icon :size="12"><Link /></el-icon>
          <span>{{ localeLabels[localeStore.getLocale()] }}</span>
        </span>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item
              v-for="locale in supportedLocales"
              :key="locale"
              :command="locale"
              :class="{ 'is-active': localeStore.getLocale() === locale }"
            >
              {{ localeLabels[locale] }}
            </el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
      <span class="status-sep status-sep-group">|</span>
      <span
        class="status-item status-log-toggle"
        :class="{ 'status-log-active': logPanelVisible }"
        :title="t('statusBar.logTitle')"
        @click="toggleLogPanel"
      >
        <el-icon :size="12"><Monitor /></el-icon>
        <span>{{ t('statusBar.logLabel') }}</span>
      </span>
      <span class="status-sep status-sep-group">|</span>
      <span class="status-item">{{ t('statusBar.buildTime', { time: status.build_time || '--' }) }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Cpu, Coin, Share, Money, DataLine, Monitor, Link } from '@element-plus/icons-vue'
import { getStatus } from '@/api'
import { formatUptime } from '@/utils/format'
import { useActivePipeline } from '@/composables/useActivePipeline'
import { useDefaultProxySettings } from '@/composables/useDefaultProxySettings'
import { useUsageTotals } from '@/composables/useUsageTotals'
import { useLogPanel } from '@/composables/useLogPanel'
import { useEdition } from '@/composables/useEdition'
import { useAuthStore } from '@/stores/auth'
import { useLocaleStore } from '@/stores/locale'
import { supportedLocales, localeLabels, type AppLocale } from '@/i18n'

const { t } = useI18n()
const router = useRouter()
const authStore = useAuthStore()
const localeStore = useLocaleStore()
const { isMinimal, isTeam } = useEdition()
const status = ref<any>({})
const { visible: logPanelVisible, toggle: toggleLogPanel } = useLogPanel()

const { pipelineId, pipelineName } = useActivePipeline({ enabled: true })
const { backendId, backendName, model } = useDefaultProxySettings({ enabled: true })
const { costText, tokensText } = useUsageTotals({ enabled: true })

const statusClass = computed(() => {
  return status.value.status === 'healthy' ? 'status-healthy' : 'status-error'
})

const statusText = computed(() => {
  return status.value.status === 'healthy' ? t('statusBar.healthy') : t('statusBar.error')
})

function goUsage() {
  if (isMinimal.value) {
    router.push('/dashboard')
    return
  }
  if (isTeam.value && authStore.isAdmin) {
    router.push('/cost')
    return
  }
  router.push('/token-usage')
}

function handleLocaleChange(locale: AppLocale) {
  localeStore.setLocale(locale)
}

async function loadStatus() {
  try {
    const res = await getStatus()
    if (res) {
      status.value = res
    }
  } catch (e) {
    console.error('Failed to load status:', e)
  }
}

onMounted(() => {
  loadStatus()
})
</script>

<style scoped>
.status-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 6px 16px;
  background: var(--shell-sidebar-bg, #f5f7fa);
  border-top: 1px solid var(--shell-sidebar-border, #e4e7ed);
  font-size: 0.75rem;
  color: #909399;
  flex-shrink: 0;
  gap: 8px;
}

.status-left,
.status-right {
  display: flex;
  align-items: center;
  gap: 4px 10px;
  flex-wrap: wrap;
  min-width: 0;
}

.status-left {
  flex: 1;
}

.status-item {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  white-space: nowrap;
}

.status-sep {
  color: #dcdfe6;
}

.status-sep-group {
  color: #c0c4cc;
  margin: 0 2px;
}

.status-dot {
  display: inline-block;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  margin-right: 2px;
}

.status-healthy {
  background: #67c23a;
}

.status-error {
  background: #f56c6c;
}

.status-clickable {
  cursor: pointer;
  transition: color 0.15s ease;
}

.status-clickable:hover {
  color: #606266;
}

.mono {
  font-family: 'SF Mono', 'Fira Code', monospace;
}

.truncate {
  max-width: 140px;
  overflow: hidden;
  text-overflow: ellipsis;
}

.status-log-toggle {
  cursor: pointer;
  transition: color 0.15s ease;
  color: #909399;
}

.status-log-toggle:hover {
  color: #606266;
}

.status-log-active {
  color: #409eff;
  font-weight: 500;
}
</style>

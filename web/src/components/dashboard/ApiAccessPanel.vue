<template>
  <div class="api-access-panel">
    <div class="panel-header">
      <div>
        <div class="panel-label">客户端接入地址</div>
        <p class="panel-desc">选择你使用的协议，复制完整 URL 到客户端配置中。</p>
      </div>
      <el-button v-if="showBaseCopy" size="small" plain @click="copyText(baseUrl, 'Base URL')">
        <el-icon><CopyDocument /></el-icon>
        复制 Base URL
      </el-button>
    </div>

    <div class="base-url-row">
      <span class="base-prefix">Base</span>
      <code class="base-url">{{ baseUrl }}</code>
    </div>

    <div class="endpoint-list">
      <div v-for="ep in endpoints" :key="ep.path" class="endpoint-row">
        <div class="endpoint-meta">
          <el-tag size="small" :type="ep.tagType || undefined" class="endpoint-tag">{{ ep.label }}</el-tag>
          <span v-if="ep.hint" class="endpoint-hint">{{ ep.hint }}</span>
        </div>
        <div class="endpoint-url-row">
          <code class="endpoint-url">{{ buildEndpointUrl(baseUrl, ep.path) }}</code>
          <el-button size="small" type="primary" plain @click="copyText(buildEndpointUrl(baseUrl, ep.path), ep.label)">
            <el-icon><CopyDocument /></el-icon>
            复制
          </el-button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ElMessage } from 'element-plus'
import { CopyDocument } from '@element-plus/icons-vue'
import { API_ENDPOINTS, buildEndpointUrl, type ApiEndpoint } from '@/utils/apiBaseUrl'

withDefaults(
  defineProps<{
    baseUrl: string
    endpoints?: ApiEndpoint[]
    showBaseCopy?: boolean
  }>(),
  {
    endpoints: () => API_ENDPOINTS,
    showBaseCopy: true
  }
)

async function copyText(text: string, label: string) {
  try {
    await navigator.clipboard.writeText(text)
    ElMessage.success(`已复制 ${label}`)
  } catch {
    ElMessage.error('复制失败，请手动复制')
  }
}
</script>

<style scoped>
.api-access-panel {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-md);
}

.panel-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--spacing-md);
}

.panel-label {
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--shell-sidebar-muted);
  margin-bottom: 4px;
}

.panel-desc {
  margin: 0;
  font-size: 0.8125rem;
  color: var(--shell-sidebar-muted);
}

.base-url-row {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  padding: 8px 12px;
  border-radius: var(--radius-md);
  background: var(--shell-sidebar-bg);
  border: 1px solid var(--shell-sidebar-border);
}

.base-prefix {
  flex-shrink: 0;
  font-size: 0.6875rem;
  font-weight: 600;
  text-transform: uppercase;
  color: var(--shell-sidebar-muted);
}

.base-url {
  flex: 1;
  min-width: 0;
  font-family: var(--font-mono);
  font-size: 0.8125rem;
  word-break: break-all;
}

.endpoint-list {
  display: flex;
  flex-direction: column;
  gap: var(--spacing-sm);
}

.endpoint-row {
  padding: 10px 12px;
  border-radius: var(--radius-md);
  border: 1px solid var(--shell-sidebar-border);
  background: #fff;
}

.endpoint-meta {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  flex-wrap: wrap;
  margin-bottom: 6px;
}

.endpoint-hint {
  font-size: 0.75rem;
  color: var(--shell-sidebar-muted);
}

.endpoint-url-row {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  flex-wrap: wrap;
}

.endpoint-url {
  flex: 1;
  min-width: 0;
  font-family: var(--font-mono);
  font-size: 0.8125rem;
  word-break: break-all;
  color: var(--shell-sidebar-text);
}
</style>
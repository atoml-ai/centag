<template>
  <div class="minimal-api-keys">
    <div class="panel-head">
      <div class="head-left">
        <span class="title">{{ t('minimalApiKeysPanel.title') }}</span>
        <el-tag size="small" :type="keyStatus.auth_required ? 'warning' : 'success'">
          {{ keyStatus.auth_required ? t('minimalApiKeysPanel.authEnabled') : t('minimalApiKeysPanel.openAccess') }}
        </el-tag>
      </div>
      <el-button type="primary" size="small" :loading="creating" @click="createKey">{{ t('minimalApiKeysPanel.createKey') }}</el-button>
    </div>
    <p class="hint">
      {{ t('minimalApiKeysPanel.hint') }}
    </p>
    <el-table :data="keys" size="small" :empty-text="t('minimalApiKeysPanel.noKeys')">
      <el-table-column prop="name" :label="t('minimalApiKeysPanel.nameColumn')" min-width="100" />
      <el-table-column :label="t('minimalApiKeysPanel.keyColumn')" min-width="180">
        <template #default="{ row }">
          <code class="mono">{{ displayKey(row) }}</code>
        </template>
      </el-table-column>
      <el-table-column :label="t('minimalApiKeysPanel.actionsColumn')" width="130" fixed="right">
        <template #default="{ row }">
          <el-button
            type="primary"
            link
            size="small"
            :disabled="!row.api_key"
            @click="copyKey(row.api_key)"
          >{{ t('minimalApiKeysPanel.copy') }}</el-button>
          <el-button type="danger" link size="small" @click="deleteKey(row)">{{ t('minimalApiKeysPanel.delete') }}</el-button>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import api from '@/api'

const { t } = useI18n()

const keys = ref<any[]>([])
const keyStatus = ref<{ auth_required?: boolean }>({})
const creating = ref(false)

function displayKey(row: { masked?: string; prefix?: string }) {
  return row.masked || (row.prefix ? `${row.prefix}****` : '****')
}

async function loadStatus() {
  try {
    const res: any = await api.get('/api/v1/settings/api-keys/status')
    keyStatus.value = res?.data || res || {}
  } catch {
    keyStatus.value = {}
  }
}

async function loadKeys() {
  try {
    const res: any = await api.get('/api/v1/settings/api-keys')
    keys.value = Array.isArray(res?.data) ? res.data : Array.isArray(res) ? res : []
  } catch {
    keys.value = []
  }
}

async function refresh() {
  await Promise.all([loadKeys(), loadStatus()])
}

async function createKey() {
  let name = 'default'
  try {
    const { value } = await ElMessageBox.prompt(t('minimalApiKeysPanel.promptMessage'), t('minimalApiKeysPanel.promptTitle'), {
      confirmButtonText: t('minimalApiKeysPanel.promptConfirm'),
      cancelButtonText: t('minimalApiKeysPanel.promptCancel'),
      inputValue: 'default',
      inputPlaceholder: 'default'
    })
    if (value?.trim()) name = value.trim()
  } catch {
    return
  }
  creating.value = true
  try {
    await api.post('/api/v1/settings/api-keys', { name })
    ElMessage.success(t('minimalApiKeysPanel.createSuccess'))
    await refresh()
  } catch (e: any) {
    ElMessage.error(e?.message || t('minimalApiKeysPanel.createFailed'))
  } finally {
    creating.value = false
  }
}

async function deleteKey(row: { id: string; name?: string }) {
  try {
    await ElMessageBox.confirm(t('minimalApiKeysPanel.deleteConfirm', { name: row.name || row.id }), t('minimalApiKeysPanel.deleteConfirmTitle'), { type: 'warning' })
  } catch {
    return
  }
  try {
    await api.delete(`/api/v1/settings/api-keys/${row.id}`)
    ElMessage.success(t('minimalApiKeysPanel.deleteSuccess'))
    await refresh()
  } catch (e: any) {
    ElMessage.error(e?.message || t('minimalApiKeysPanel.deleteFailed'))
  }
}

async function copyKey(key: string) {
  if (!key) {
    ElMessage.warning(t('minimalApiKeysPanel.copyNoPlaintext'))
    return
  }
  const { copyToClipboard } = await import('@/utils/clipboard')
  if (await copyToClipboard(key)) {
    ElMessage.success(t('minimalApiKeysPanel.copySuccess'))
  } else {
    ElMessage.warning(t('minimalApiKeysPanel.copyFailed'))
  }
}

onMounted(refresh)

defineExpose({ refresh })
</script>

<style scoped>
.panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 8px;
}
.head-left {
  display: flex;
  align-items: center;
  gap: 8px;
}
.title {
  font-weight: 600;
  font-size: 0.95rem;
}
.hint {
  margin: 0 0 10px;
  color: var(--el-text-color-secondary);
  font-size: 0.8rem;
  line-height: 1.45;
}
.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.8rem;
}
</style>

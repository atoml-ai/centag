<template>
  <div class="minimal-api-keys">
    <div class="panel-head">
      <div class="head-left">
        <span class="title">API 访问密钥</span>
        <el-tag size="small" :type="keyStatus.auth_required ? 'warning' : 'success'">
          {{ keyStatus.auth_required ? '已启用鉴权' : '开放访问' }}
        </el-tag>
      </div>
      <el-button type="primary" size="small" :loading="creating" @click="createKey">创建密钥</el-button>
    </div>
    <p class="hint">
      未配置时 <code>/v1</code> 开放；配置后须携带
      <code>Authorization: Bearer &lt;key&gt;</code>。列表仅脱敏显示，点击复制可拿到完整密钥。
    </p>
    <el-table :data="keys" size="small" empty-text="暂无密钥">
      <el-table-column prop="name" label="名称" min-width="100" />
      <el-table-column label="密钥" min-width="180">
        <template #default="{ row }">
          <code class="mono">{{ displayKey(row) }}</code>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="130" fixed="right">
        <template #default="{ row }">
          <el-button
            type="primary"
            link
            size="small"
            :disabled="!row.api_key"
            @click="copyKey(row.api_key)"
          >复制</el-button>
          <el-button type="danger" link size="small" @click="deleteKey(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import api from '@/api'

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
    const { value } = await ElMessageBox.prompt('为密钥起个名字（可选）', '创建 API Key', {
      confirmButtonText: '创建',
      cancelButtonText: '取消',
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
    ElMessage.success('密钥已创建，可点击复制')
    await refresh()
  } catch (e: any) {
    ElMessage.error(e?.message || '创建失败')
  } finally {
    creating.value = false
  }
}

async function deleteKey(row: { id: string; name?: string }) {
  try {
    await ElMessageBox.confirm(`确定删除密钥「${row.name || row.id}」？`, '删除确认', { type: 'warning' })
  } catch {
    return
  }
  try {
    await api.delete(`/api/v1/settings/api-keys/${row.id}`)
    ElMessage.success('已删除')
    await refresh()
  } catch (e: any) {
    ElMessage.error(e?.message || '删除失败')
  }
}

async function copyKey(key: string) {
  if (!key) {
    ElMessage.warning('旧密钥未保存明文，请删除后重新创建')
    return
  }
  try {
    await navigator.clipboard.writeText(key)
    ElMessage.success('已复制完整密钥')
  } catch {
    ElMessage.warning('复制失败，请手动选中复制')
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

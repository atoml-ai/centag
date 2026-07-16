<template>
  <div class="node-plugin-list">
    <el-table :data="plugins" style="width: 100%" v-loading="loading">
      <el-table-column prop="name" label="名称" min-width="150">
        <template #default="{ row }">
          <div class="plugin-name">
            <span>{{ row.name }}</span>
            <el-tag v-if="row.deprecated" type="warning" size="small">已弃用</el-tag>
          </div>
        </template>
      </el-table-column>
      <el-table-column prop="implementation" label="实现" min-width="180">
        <template #default="{ row }">
          <code class="implementation-code">{{ row.implementation }}</code>
        </template>
      </el-table-column>
      <el-table-column prop="kind" label="类型" min-width="120" />
      <el-table-column prop="version" label="版本" width="100" />
      <el-table-column label="权限" min-width="150">
        <template #default="{ row }">
          <el-tooltip
            v-for="perm in row.permissions"
            :key="perm"
            placement="top"
            :content="getPermissionDescription(perm)"
          >
            <el-tag
              size="small"
              type="info"
              class="permission-tag"
            >
              {{ perm }}
            </el-tag>
          </el-tooltip>
        </template>
      </el-table-column>
      <el-table-column label="兼容性" width="130">
        <template #default="{ row }">
          <el-tooltip v-if="row.min_centag_version" :content="`最低版本：${row.min_centag_version}`">
            <el-tag :type="checkVersionCompatibility(row.min_centag_version) ? 'success' : 'danger'" size="small">
              {{ checkVersionCompatibility(row.min_centag_version) ? '兼容' : '不兼容' }}
            </el-tag>
          </el-tooltip>
          <span v-else class="text-muted">-</span>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="180" fixed="right">
        <template #default="{ row }">
          <el-button size="small" @click="$emit('view', row)">查看</el-button>
          <el-button size="small" type="primary" @click="$emit('test', row)">测试</el-button>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script setup lang="ts">

interface PluginDescriptor {
  name: string
  implementation: string
  kind: string
  version: string
  description?: string
  permissions?: string[]
  deprecated?: boolean
  min_centag_version?: string
  supports_stream?: boolean
  tags?: string[]
}

defineProps<{
  plugins: PluginDescriptor[]
  loading?: boolean
}>()

defineEmits<{
  view: [plugin: PluginDescriptor]
  test: [plugin: PluginDescriptor]
}>()

function checkVersionCompatibility(minVersion: string): boolean {
  const currentVersion = '1.0.0'
  const min = minVersion.split('.').map(Number)
  const curr = currentVersion.split('.').map(Number)
  for (let i = 0; i < 3; i++) {
    if (curr[i] < min[i]) return false
    if (curr[i] > min[i]) return true
  }
  return true
}

function getPermissionDescription(perm: string): string {
  const descriptions: Record<string, string> = {
    'llm.call': '调用 LLM 后端（生成文本）',
    'storage.read': '读取存储（缓存、KV 存储）',
    'storage.write': '写入存储（缓存、KV 存储）',
    'memory.read': '读取记忆（Mem0、智能体记忆）',
    'memory.write': '写入记忆（Mem0、智能体记忆）',
    'network.outbound': '发起出站 HTTP 请求',
    'secrets.read': '读取密钥（API 密钥、令牌）',
    'network.inbound': '接受入站请求（Webhook、回调）',
  }
  return descriptions[perm] || `权限：${perm}`
}
</script>

<style scoped>
.plugin-name {
  display: flex;
  align-items: center;
  gap: 8px;
}
.implementation-code {
  font-size: 12px;
  background: #f5f7fa;
  padding: 2px 6px;
  border-radius: 4px;
}
.permission-tag {
  margin-right: 4px;
}
.text-muted {
  color: #909399;
  font-size: 12px;
}
</style>
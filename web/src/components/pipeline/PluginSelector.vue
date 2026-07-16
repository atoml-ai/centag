<template>
  <div class="plugin-selector">
    <div class="plugin-selector-header">
      <span class="plugin-selector-title">插件实现选择</span>
      <el-button
        v-if="showAllPlugins"
        size="small"
        text
        @click="$emit('view-all')"
      >
        浏览全部插件
      </el-button>
    </div>

    <div class="plugin-selector-content">
      <!-- Kind 选择 -->
      <el-form-item label="插件类型 (Kind)" v-if="showKindSelector">
        <el-select
          v-model="selectedKind"
          placeholder="选择插件类型"
          style="width: 100%"
          @change="onKindChange"
        >
          <el-option
            v-for="kind in uniqueKinds"
            :key="kind"
            :label="kind"
            :value="kind"
          />
        </el-select>
        <div class="help-text">
          插件类型，如 llm.generate、content.transform、quality.review 等
        </div>
      </el-form-item>

      <!-- Implementation 选择 -->
      <el-form-item label="插件实现 (Implementation)">
        <el-select
          v-model="selectedImplementation"
          placeholder="选择插件实现"
          style="width: 100%"
          filterable
          @change="onImplementationChange"
        >
          <el-option
            v-for="plugin in filteredPlugins"
            :key="plugin.implementation"
            :label="plugin.name"
            :value="plugin.implementation"
          >
            <div class="plugin-option">
              <span class="plugin-name">{{ plugin.name }}</span>
              <span class="plugin-impl">{{ plugin.implementation }}</span>
            </div>
          </el-option>
        </el-select>
      </el-form-item>

      <!-- 选中插件详情 -->
      <div v-if="selectedPlugin" class="plugin-details">
        <el-alert
          type="info"
          :closable="false"
          style="margin-bottom: 12px"
        >
          <template #default>
            <div style="font-size: 13px; line-height: 1.5">
              <strong>📋 插件信息：</strong><br>
              <span v-if="selectedPlugin.description">{{ selectedPlugin.description }}<br></span>
              <span>版本：{{ selectedPlugin.version }}</span>
            </div>
          </template>
        </el-alert>

        <!-- 权限提示 -->
        <div v-if="selectedPlugin.permissions && selectedPlugin.permissions.length > 0" class="permissions-section">
          <div class="permissions-label">
            <el-icon><Lock /></el-icon>
            <span>权限要求：</span>
          </div>
          <div class="permissions-list">
            <el-tooltip
              v-for="perm in selectedPlugin.permissions"
              :key="perm"
              placement="top"
              :content="getPermissionDescription(perm)"
            >
              <el-tag size="small" type="info" class="permission-tag">
                {{ perm }}
              </el-tag>
            </el-tooltip>
          </div>
        </div>

        <!-- 兼容性提示 -->
        <div v-if="selectedPlugin.min_centag_version" class="compatibility-section">
          <el-alert
            :type="checkVersionCompatibility(selectedPlugin.min_centag_version) ? 'success' : 'warning'"
            :closable="false"
            style="margin-top: 8px"
          >
            <template #default>
              <div style="font-size: 13px">
                <span v-if="checkVersionCompatibility(selectedPlugin.min_centag_version)">
                  ✅ 与当前 Centag 版本兼容（最低版本：{{ selectedPlugin.min_centag_version }}）
                </span>
                <span v-else>
                  ⚠️ 当前 Centag 版本低于最低要求（需要：{{ selectedPlugin.min_centag_version }}）
                </span>
              </div>
            </template>
          </el-alert>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { Lock } from '@element-plus/icons-vue'
import { PluginDescriptor } from '@/api/pipeline'

const props = defineProps<{
  plugins: PluginDescriptor[]
  modelValue?: string
  kind?: string
  showKindSelector?: boolean
  showAllPlugins?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: string | undefined)
  (e: 'update:kind', value: string | undefined)
  (e: 'view-all')
}>()

const selectedImplementation = ref<string | undefined>(props.modelValue)
const selectedKind = ref<string | undefined>(props.kind)

// 获取唯一的 Kind 列表
const uniqueKinds = computed(() => {
  const kinds = new Set(props.plugins.map(p => p.kind).filter(Boolean))
  return Array.from(kinds).sort()
})

// 根据 Kind 过滤插件；若当前 Kind 无匹配项则回退展示全部，避免下拉为空
const filteredPlugins = computed(() => {
  if (!selectedKind.value) {
    return props.plugins
  }
  const matched = props.plugins.filter(p => p.kind === selectedKind.value)
  return matched.length > 0 ? matched : props.plugins
})

// 获取选中的插件详情
const selectedPlugin = computed(() => {
  if (!selectedImplementation.value) return undefined
  return props.plugins.find(p => p.implementation === selectedImplementation.value)
})

// Kind 变化时重置 Implementation
const onKindChange = (kind: string | undefined) => {
  selectedKind.value = kind
  selectedImplementation.value = undefined
  emit('update:modelValue', undefined)
  emit('update:kind', kind)
}

// Implementation 变化时更新 Kind
const onImplementationChange = (impl: string | undefined) => {
  selectedImplementation.value = impl
  const plugin = props.plugins.find(p => p.implementation === impl)
  if (plugin) {
    selectedKind.value = plugin.kind
    emit('update:kind', plugin.kind)
  } else {
    selectedKind.value = undefined
    emit('update:kind', undefined)
  }
  emit('update:modelValue', impl)
}

// 监听外部 modelValue 变化
watch(() => props.modelValue, (newVal) => {
  if (newVal !== selectedImplementation.value) {
    selectedImplementation.value = newVal
    const plugin = props.plugins.find(p => p.implementation === newVal)
    if (plugin) {
      selectedKind.value = plugin.kind
    }
  }
})

// 监听外部 kind 变化
watch(() => props.kind, (newVal) => {
  if (newVal !== selectedKind.value) {
    selectedKind.value = newVal
  }
})

// 版本兼容性检查
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

// 权限描述
function getPermissionDescription(perm: string): string {
  const descriptions: Record<string, string> = {
    'llm.call': '调用 LLM 后端（生成文本）',
    'storage.read': '读取存储（缓存、KV 存储）',
    'storage.write': '写入存储（缓存、KV 存储）',
    'memory.read': '读取记忆（Mem0、智能体记忆）',
    'memory.write': '写入记忆（Mem0、智能体记忆）',
    'network.outbound': '发起出站 HTTP 请求',
    'system.admin': '系统管理权限',
  }
  return descriptions[perm] || perm
}
</script>

<style scoped>
.plugin-selector {
  border: 1px solid #dcdfe6;
  border-radius: 6px;
  padding: 16px;
  margin-bottom: 16px;
  background: #f5f7fa;
}

.plugin-selector-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.plugin-selector-title {
  font-weight: 600;
  color: #303133;
  font-size: 14px;
}

.plugin-selector-content {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.plugin-option {
  display: flex;
  flex-direction: column;
}

.plugin-name {
  font-weight: 500;
  color: #303133;
}

.plugin-impl {
  font-size: 12px;
  color: #606266;
  font-family: monospace;
}

.help-text {
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
}

.plugin-details {
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid #e4e7ed;
}

.permissions-section {
  margin-top: 12px;
}

.permissions-label {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: #606266;
  margin-bottom: 8px;
}

.permissions-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.permission-tag {
  cursor: pointer;
}

.compatibility-section {
  margin-top: 8px;
}
</style>
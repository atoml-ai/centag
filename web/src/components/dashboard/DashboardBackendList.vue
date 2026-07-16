<template>
  <div class="backend-list">
    <div v-for="b in backends" :key="b.id" class="backend-item">
      <div class="backend-left">
        <el-switch
          :model-value="b.enabled"
          :loading="togglingMap[b.id]"
          active-color="#10b981"
          size="small"
          class="backend-enabled-switch"
          @change="(enabled) => handleToggle(b, enabled)"
        />
        <div class="backend-info">
          <div class="backend-name">{{ b.name }}</div>
          <div class="backend-meta">{{ b.type }} · {{ b.base_url }}</div>
        </div>
      </div>
      <div class="backend-right">
        <span class="backend-weight">权重 {{ b.weight }}</span>
        <span class="backend-models" v-if="b.supported_models?.length">
          {{ b.supported_models.length }} 模型
        </span>
        <div class="backend-actions">
          <el-button
            size="small"
            :loading="testingMap[b.id]"
            @click="handleTest(b)"
          >
            测试
          </el-button>
          <el-button size="small" type="primary" plain @click="handleEdit(b)">
            编辑
          </el-button>
        </div>
      </div>
    </div>
    <div v-if="!backends.length" class="empty-tip">暂无后端配置</div>

    <BackendEditorDialog
      ref="editorRef"
      v-model="editorVisible"
      @saved="emit('refresh')"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { ElMessage } from 'element-plus'
import BackendEditorDialog from '@/components/backends/BackendEditorDialog.vue'
import { updateBackend } from '@/api'
import { getBackendTestMessage, testBackendConnection } from '@/utils/backendTest'

defineProps<{
  backends: any[]
}>()

const emit = defineEmits<{
  refresh: []
  'backend-updated': [backend: any]
}>()

const editorRef = ref<InstanceType<typeof BackendEditorDialog> | null>(null)
const editorVisible = ref(false)
const testingMap = reactive<Record<string, boolean>>({})
const togglingMap = reactive<Record<string, boolean>>({})

async function handleToggle(backend: any, enabled: boolean) {
  togglingMap[backend.id] = true
  try {
    const updated = await updateBackend(backend.id, { ...backend, enabled })
    emit('backend-updated', updated?.id ? updated : { id: backend.id, enabled })
    ElMessage.success(`${backend.name || backend.id} ${enabled ? '已启用' : '已禁用'}`)
  } catch (error: any) {
    ElMessage.error('操作失败：' + (error.message || '未知错误'))
    emit('refresh')
  } finally {
    togglingMap[backend.id] = false
  }
}

function showTestResult(updatedBackend: any, fallbackName: string) {
  const { level, text } = getBackendTestMessage(updatedBackend, fallbackName)
  if (level === 'success') ElMessage.success(text)
  else ElMessage.warning(text)
}

async function handleTest(backend: any) {
  testingMap[backend.id] = true
  try {
    const apiKey = editorRef.value?.getPendingApiKey(backend.id)
    const updatedBackend = await testBackendConnection(backend, apiKey ? { apiKey } : undefined)

    if (updatedBackend && updatedBackend.id) {
      emit('backend-updated', updatedBackend)
      showTestResult(updatedBackend, backend.id)
    } else {
      ElMessage.warning(`${backend.name || backend.id} 连接测试完成，但未返回完整数据`)
      emit('refresh')
    }
  } catch (error: any) {
    ElMessage.error(`${backend.name || backend.id} 连接失败：${error.message}`)
  } finally {
    testingMap[backend.id] = false
  }
}

function handleEdit(backend: any) {
  editorRef.value?.openEdit(backend)
}
</script>

<style scoped>
.backend-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
  width: 100%;
  height: 100%;
  min-height: 0;
  max-height: none;
  overflow-y: auto;
  padding-right: 4px;
}

.backend-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  width: 100%;
  padding: 10px 12px;
  background: #f9fafb;
  border-radius: 6px;
  border: 1px solid #f3f4f6;
}

.backend-left {
  display: flex;
  align-items: center;
  gap: 10px;
  flex: 1;
  min-width: 0;
}

.backend-enabled-switch {
  flex-shrink: 0;
}

.backend-info {
  flex: 1;
  min-width: 0;
}

.backend-name {
  font-size: 0.875rem;
  font-weight: 500;
  color: #111827;
  display: flex;
  align-items: center;
}

.backend-meta {
  font-size: 0.75rem;
  color: #9ca3af;
  margin-top: 2px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.backend-right {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 4px;
  flex-shrink: 0;
  margin-left: 12px;
}

.backend-weight,
.backend-models {
  font-size: 0.75rem;
  color: #9ca3af;
}

.backend-actions {
  display: flex;
  gap: 6px;
  margin-top: 2px;
}

.empty-tip {
  text-align: center;
  color: #9ca3af;
  font-size: 0.8rem;
  padding: 16px 0;
}
</style>
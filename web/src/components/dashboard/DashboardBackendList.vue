<template>
  <div class="backend-list">
    <div v-for="b in backends" :key="b.id" class="backend-item" :class="{ 'is-default': defaultBackendId === b.id }">
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
          <div class="backend-name">
            {{ b.name }}
            <el-tag v-if="defaultBackendId === b.id" type="success" size="small" effect="light" class="default-tag">
              默认
            </el-tag>
          </div>
          <div class="backend-meta">{{ b.type }} · {{ b.base_url }}</div>
          <div class="backend-default-model" v-if="b.probe_model || b.default_model">
            默认模型：<span class="model-name">{{ b.default_model || b.probe_model }}</span>
          </div>
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
          <el-button
            size="small"
            :type="defaultBackendId === b.id ? 'success' : 'default'"
            :plain="defaultBackendId !== b.id"
            :disabled="defaultBackendId === b.id || settingDefaultMap[b.id]"
            :loading="settingDefaultMap[b.id]"
            @click="handleSetDefault(b)"
          >
            {{ defaultBackendId === b.id ? '当前默认' : '设为默认' }}
          </el-button>
          <el-button size="small" type="primary" plain @click="handleEdit(b)">
            编辑
          </el-button>
          <el-button
            size="small"
            type="danger"
            plain
            :disabled="defaultBackendId === b.id"
            @click="handleDelete(b)"
          >
            删除
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
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import BackendEditorDialog from '@/components/backends/BackendEditorDialog.vue'
import { updateBackend, deleteBackend } from '@/api'
import { getBackendTestMessage, testBackendConnection } from '@/utils/backendTest'
import api from '@/api'

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
const settingDefaultMap = reactive<Record<string, boolean>>({})
const defaultBackendId = ref('')

onMounted(() => {
  loadDefaultBackend()
})

async function loadDefaultBackend() {
  try {
    const res: any = await api.get('/api/v1/config/proxy')
    const data = res?.data ?? res
    defaultBackendId.value = data?.default_backend_id || ''
  } catch { /* ignore */ }
}

async function handleSetDefault(backend: any) {
  if (defaultBackendId.value === backend.id) return
  settingDefaultMap[backend.id] = true
  try {
    const sm = Array.isArray(backend.supported_models) ? backend.supported_models : []
    const defaultModel = backend.default_model || backend.probe_model
      || (sm[0] && (sm[0].actual_model || sm[0].requested_model)) || ''
    await api.put('/api/v1/config/proxy', {
      default_backend_id: backend.id,
      default_model: defaultModel
    })
    defaultBackendId.value = backend.id
    ElMessage.success(`已将「${backend.name || backend.id}」设为默认后端，模型「${defaultModel || '未设置'}」`)
    emit('refresh')
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.message || '设置默认后端失败')
  } finally {
    settingDefaultMap[backend.id] = false
  }
}

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

async function handleDelete(backend: any) {
  if (defaultBackendId.value === backend.id) {
    ElMessage.warning('不能删除当前默认后端，请先设置其他后端为默认')
    return
  }
  try {
    await ElMessageBox.confirm(
      `确定删除后端「${backend.name || backend.id}」吗？此操作不可恢复。`,
      '确认删除',
      { confirmButtonText: '删除', cancelButtonText: '取消', type: 'warning' }
    )
  } catch {
    return
  }
  try {
    await deleteBackend(backend.id)
    ElMessage.success(`已删除后端「${backend.name || backend.id}」`)
    emit('refresh')
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.message || '删除后端失败')
  }
}

function openCreate() {
  editorRef.value?.openCreate()
}

function reloadDefault() {
  loadDefaultBackend()
}

defineExpose({ openCreate, reloadDefault })
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
  transition: all 0.2s;
}

.backend-item.is-default {
  background: #f0f9eb;
  border-color: #b3e19d;
}

.default-tag {
  margin-left: 6px;
  flex-shrink: 0;
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

.backend-default-model {
  font-size: 0.75rem;
  color: #6b7280;
  margin-top: 4px;
}

.backend-default-model .model-name {
  color: #409eff;
  font-weight: 500;
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
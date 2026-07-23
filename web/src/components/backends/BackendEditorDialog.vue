<template>
  <el-dialog
    v-model="dialogVisible"
    :title="isCreate ? '添加 Provider' : '编辑 Provider'"
    width="680px"
    class="provider-editor-dialog"
    @close="onDialogClose"
  >
    <div class="provider-form-body">
      <el-tabs v-model="activeTab" class="editor-tabs">
        <!-- ═══════════ Tab 1: 基本设置 ═══════════ -->
        <el-tab-pane label="连接配置" name="basic">
          <!-- Provider 搜索选择（创建模式） -->
          <div v-if="isCreate" class="form-group">
            <label class="form-label">Provider</label>
            <div class="provider-dropdown" v-click-outside="() => (showProviderList = false)">
              <el-input
                v-model="form.name"
                placeholder="搜索或输入 Provider 名称..."
                autocomplete="off"
                @focus="showProviderList = true"
                @click="showProviderList = true"
              />
              <div v-if="showProviderList" class="provider-list">
                <div
                  v-for="p in filteredProviders"
                  :key="p.id"
                  class="provider-option"
                  :class="{ selected: form.providerId === p.id }"
                  @mousedown.prevent="selectProvider(p.id)"
                >
                  <span class="provider-icon">{{ p.icon }}</span>
                  <div class="provider-option-text">
                    <span class="provider-name">{{ p.name }}</span>
                    <span class="provider-desc">{{ p.description }}</span>
                  </div>
                </div>
                <div v-if="filteredProviders.length === 0" class="provider-empty">
                  未找到匹配的 Provider，可继续手动填写
                </div>
              </div>
            </div>
          </div>

          <!-- 编辑模式 -->
          <template v-else>
            <div class="form-group" v-if="form.id">
              <label class="form-label">后端 ID</label>
              <el-input :model-value="form.id" readonly class="id-input">
                <template #prefix>
                  <el-icon><Document /></el-icon>
                </template>
                <template #append>
                  <el-button @click="copyBackendId(form.id)" :icon="CopyDocument" />
                </template>
              </el-input>
            </div>
            <div class="form-group">
              <label class="form-label">名称</label>
              <el-input v-model="form.name" placeholder="Provider 名称" autocomplete="off" />
            </div>
          </template>

          <div class="form-row-2">
            <div class="form-group">
              <label class="form-label">Base URL</label>
              <el-input v-model="form.base_url" placeholder="https://api.example.com/v1" autocomplete="off" />
            </div>
            <div class="form-group">
              <label class="form-label">类型</label>
              <el-select v-model="form.type" style="width: 100%" @change="onTypeChanged">
                <el-option
                  v-for="bt in backendTypes"
                  :key="bt.type"
                  :label="bt.name + (bt.type === 'gemini' ? '（原生）' : '')"
                  :value="bt.type"
                />
              </el-select>
            </div>
          </div>

          <el-alert
            v-if="isKimiForCodingEndpoint"
            type="info"
            :closable="false"
            show-icon
            class="kimi-hint"
            title="Kimi for Coding 需单独会员与专用 API Key"
            description="请在 kimi.com/code 开通会员，并使用该页面生成的 Key。"
          />

          <!-- API Key 列表 -->
          <div class="form-group">
            <div class="key-list-header">
              <label class="form-label">API Key</label>
              <el-button type="primary" link size="small" @click="addApiKey">
                <el-icon><Plus /></el-icon> 添加
              </el-button>
            </div>
            <div v-if="apiKeys.length === 0" class="key-empty">
              <el-icon :size="20" color="#c0c4cc"><WarningFilled /></el-icon>
              <span>请添加至少一个 API Key</span>
            </div>
            <TransitionGroup name="key-list" tag="div" class="key-list">
              <div v-for="(key, idx) in apiKeys" :key="key.id" class="key-card">
                <div class="key-card-row">
                  <div class="key-badge">{{ idx + 1 }}</div>
                  <el-input
                    v-model="key.api_key"
                    type="password"
                    show-password
                    :placeholder="key.has_key ? '已设置，留空保持不变' : 'sk-...'"
                    autocomplete="new-password"
                    class="key-input"
                  />
                  <el-tag v-if="key.has_key && !key.api_key" size="small" type="success" effect="light" round>
                    已设置
                  </el-tag>
                  <el-button
                    v-if="apiKeys.length > 1"
                    type="danger"
                    :icon="Delete"
                    circle
                    size="small"
                    plain
                    @click="removeApiKey(idx)"
                  />
                </div>
              </div>
            </TransitionGroup>
          </div>

          <!-- 多 Key 时自动显示轮转策略 -->
          <Transition name="el-fade-in">
            <div v-if="apiKeys.length > 1" class="strategy-section">
              <div class="form-group" style="margin-bottom: 0">
                <label class="form-label">轮转策略</label>
                <el-select v-model="accountPool.strategy" style="width: 100%">
                  <el-option label="加权轮询 (Round Robin)" value="round_robin" />
                  <el-option label="最少使用 (Least Usage)" value="least_usage" />
                  <el-option label="会话亲和 (Sticky Session)" value="sticky_session" />
                </el-select>
              </div>
              <div class="strategy-desc">
                <template v-if="accountPool.strategy === 'round_robin'">
                  按权重依次将请求分配给各 Key，权重越高分配越多，确保所有 Key 均匀使用。
                </template>
                <template v-else-if="accountPool.strategy === 'least_usage'">
                  每次请求选择窗口内调用次数最少的 Key，自动平衡各 Key 的负载压力。
                </template>
                <template v-else>
                  根据请求中的用户标识（Header X-Session-ID / user 字段）绑定固定 Key，保证同一用户始终使用同一凭证。
                </template>
              </div>
              <div class="strategy-hint">
                已添加 {{ apiKeys.length }} 个 Key，请求将按策略自动轮转
              </div>
            </div>
          </Transition>
        </el-tab-pane>

        <!-- ═══════════ Tab 2: 模型管理 ═══════════ -->
        <el-tab-pane label="模型管理" name="models">
          <div class="form-group">
            <label class="form-label">默认模型</label>
            <el-input
              v-model="form.default_model"
              placeholder="输入模型名称，如 gpt-4o"
              autocomplete="off"
              list="personal-model-suggestions"
            />
            <datalist id="personal-model-suggestions">
              <option v-for="m in form.models" :key="m.name" :value="m.name" />
            </datalist>
          </div>

          <div class="model-section">
            <div class="model-list-head">
              <label class="form-label">
                模型列表
                <span v-if="form.models.length" class="model-count">{{ form.models.length }}</span>
              </label>
              <el-button
                type="primary"
                link
                size="small"
                :loading="fetchingModels"
                :disabled="!canFetchModels"
                @click="fetchModelsNow"
              >
                <el-icon><Refresh /></el-icon> 刷新
              </el-button>
            </div>
            <div class="model-add-row">
              <el-input
                v-model="newModelName"
                placeholder="输入模型名称，回车添加"
                autocomplete="off"
                @keyup.enter="onAddModel"
              />
              <el-button @click="onAddModel" :disabled="!newModelName.trim()" type="primary" plain>
                添加
              </el-button>
            </div>
            <div class="model-preview" :class="{ empty: form.models.length === 0 }">
              <div v-if="form.models.length > 0" class="model-tags">
                <span
                  v-for="(model, mi) in form.models"
                  :key="model.name + '-' + mi"
                  class="model-tag"
                  :class="{ active: model.name === form.default_model }"
                  @click="setDefaultModel(model.name)"
                >
                  <el-icon v-if="model.name === form.default_model" :size="12"><StarFilled /></el-icon>
                  {{ model.name }}
                  <span class="tag-remove" @click.stop="onRemoveModel(mi)">
                    <el-icon :size="12"><Close /></el-icon>
                  </span>
                </span>
              </div>
              <div v-else class="model-empty-tip">
                <el-icon :size="24" color="#dcdfe6"><FolderOpened /></el-icon>
                <span>暂无模型</span>
              </div>
            </div>
            <div class="form-tip">点击标签设为默认，带 <el-icon :size="12" color="#409eff"><StarFilled /></el-icon> 为当前默认</div>
          </div>
        </el-tab-pane>

        <!-- ═══════════ Tab 3: 高级配置 ═══════════ -->
        <el-tab-pane label="高级配置" name="advanced">
          <div class="advanced-section">
            <div class="section-title">基础参数</div>
            <div class="form-group">
              <label class="form-label">类型</label>
              <el-select v-model="form.type" style="width: 100%" @change="onTypeChanged">
                <el-option
                  v-for="bt in backendTypes"
                  :key="bt.type"
                  :label="bt.name + (bt.type === 'gemini' ? '（原生）' : '')"
                  :value="bt.type"
                />
              </el-select>
              <div v-if="selectedTypeMeta" class="type-hint">
                <template v-if="selectedTypeMeta.key_help">
                  <span class="hint-label">Key 格式：</span>{{ selectedTypeMeta.key_help }}
                </template>
                <br v-if="selectedTypeMeta.key_help && selectedTypeMeta.default_base_url" />
                <template v-if="selectedTypeMeta.default_base_url">
                  <span class="hint-label">默认端点：</span><code>{{ selectedTypeMeta.default_base_url }}</code>
                </template>
              </div>
            </div>

            <div class="form-group">
              <label class="form-label">默认探测模型</label>
              <el-input v-model="form.probe_model" placeholder="留空则使用默认模型" autocomplete="off" />
            </div>

            <div class="form-group">
              <label class="form-label">描述</label>
              <el-input v-model="form.description" type="textarea" :rows="2" placeholder="可选备注" />
            </div>
          </div>

          <div class="advanced-section">
            <div class="section-title">运行参数</div>
            <div class="form-row-3">
              <div class="form-group">
                <label class="form-label">超时（秒）</label>
                <el-input-number v-model="form.timeout" :min="1" :max="600" style="width: 100%" />
              </div>
              <div class="form-group">
                <label class="form-label">最大重试</label>
                <el-input-number v-model="form.max_retries" :min="0" :max="10" style="width: 100%" />
              </div>
            </div>
            <div class="form-row-2">
              <div class="toggle-card">
                <div class="toggle-info">
                  <span class="toggle-label">启用</span>
                  <span class="toggle-desc">关闭后不参与请求路由</span>
                </div>
                <el-switch v-model="form.enabled" />
              </div>
              <div class="toggle-card">
                <div class="toggle-info">
                  <span class="toggle-label">自动同步模型</span>
                  <span class="toggle-desc">探测成功后写入模型列表</span>
                </div>
                <el-switch v-model="form.auto_fetch_models" />
              </div>
            </div>
          </div>

          <el-alert
            v-if="form.type === 'gemini'"
            type="success"
            :closable="false"
            show-icon
            class="gemini-hint"
            title="Gemini 原生 — 官方 / CLI 推荐"
            description="直接接入 Gemini 官方 API，支持所有原生特性。"
          />
        </el-tab-pane>
      </el-tabs>
    </div>

    <template #footer>
      <div class="dialog-footer">
        <el-button @click="dialogVisible = false" size="large">取消</el-button>
        <el-button type="primary" :loading="saving" :disabled="!canSave" @click="save" size="large">
          {{ isCreate ? '添加 Provider' : '保存' }}
        </el-button>
      </div>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import {
  Delete,
  Plus,
  CopyDocument,
  Document,
  Refresh,
  StarFilled,
  Close,
  FolderOpened,
  WarningFilled,
} from '@element-plus/icons-vue'
import api from '@/api'
import { listBackendTypes, type BackendTypeMeta } from '@/api/backend'
import {
  getProviderList,
  applyProviderPreset,
  filterProviders,
  addModelToForm,
  removeModelFromForm,
  validateProviderForm,
  createEmptyProviderForm,
  toApiBackendPayload,
  fromApiBackend,
} from '@/utils/shared-modules'
import type { ProviderFormModel, ProviderDef } from '@/utils/shared-modules'

const props = withDefaults(defineProps<{
  modelValue: boolean
  backend?: any
  create?: boolean
}>(), {
  create: false,
})

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  saved: []
}>()

const vClickOutside = {
  mounted(el: any, binding: any) {
    el.__clickOutside = (e: MouseEvent) => {
      if (!el.contains(e.target as Node)) binding.value?.(e)
    }
    document.addEventListener('mousedown', el.__clickOutside)
  },
  unmounted(el: any) {
    document.removeEventListener('mousedown', el.__clickOutside)
  },
}

const saving = ref(false)
const isCreate = ref(false)
const dialogVisible = ref(false)
const showProviderList = ref(false)
const newModelName = ref('')
const fetchingModels = ref(false)
const activeTab = ref('basic')
const backendTypes = ref<BackendTypeMeta[]>([])

const apiKeys = ref<Array<{
  id: string
  api_key: string
  has_key: boolean
}>>([])

const accountPool = reactive({
  strategy: 'round_robin' as string,
})

type ConnectivitySnapshot = {
  type: string
  base_url: string
  api_key: string
  enabled: boolean
}
const connectivitySnapshot = ref<ConnectivitySnapshot | null>(null)

const providerList = ref<ProviderDef[]>(getProviderList())
const form = reactive<ProviderFormModel>(createEmptyProviderForm())

watch(
  () => props.modelValue,
  (visible) => { dialogVisible.value = !!visible },
  { immediate: true }
)

watch(dialogVisible, (visible) => {
  if (visible !== props.modelValue) emit('update:modelValue', visible)
})

const filteredProviders = computed(() => filterProviders(providerList.value, form.name))

const isKimiForCodingEndpoint = computed(() =>
  (form.base_url || '').toLowerCase().includes('api.kimi.com/coding')
)

const selectedTypeMeta = computed(() =>
  backendTypes.value.find((bt) => bt.type === form.type) || null
)

const canFetchModels = computed(() => {
  if (!(form.base_url || '').trim()) return false
  if (form.type === 'ollama') return true
  if (apiKeys.value.some((k) => (k.api_key || '').trim())) return true
  return !isCreate.value && apiKeys.value.some((k) => k.has_key)
})

const canSave = computed(() =>
  validateProviderForm(form, {
    isCreate: isCreate.value,
    requireApiKey: true,
  }).ok
)

function resetForm() {
  Object.assign(form, createEmptyProviderForm())
  newModelName.value = ''
  showProviderList.value = false
  activeTab.value = 'basic'
  connectivitySnapshot.value = null
  apiKeys.value = []
  accountPool.strategy = 'round_robin'
}

function captureConnectivitySnapshot() {
  connectivitySnapshot.value = {
    type: form.type || '',
    base_url: (form.base_url || '').trim(),
    api_key: (form.api_key || '').trim(),
    enabled: !!form.enabled,
  }
}

function connectivityChanged(): boolean {
  const snap = connectivitySnapshot.value
  if (!snap) return true
  if ((form.type || '') !== snap.type) return true
  if ((form.base_url || '').trim() !== snap.base_url) return true
  if ((form.api_key || '').trim() !== '' && (form.api_key || '').trim() !== snap.api_key) return true
  if (form.enabled && !snap.enabled) return true
  return false
}

function loadBackendTypes() {
  listBackendTypes()
    .then((types) => { backendTypes.value = Array.isArray(types) ? types : [] })
    .catch((err) => { console.error('Failed to load backend types', err) })
}

function onTypeChanged(type: string) {
  const meta = backendTypes.value.find((bt) => bt.type === type)
  if (meta && meta.default_base_url && !form.base_url) {
    form.base_url = meta.default_base_url
  }
}

function selectProvider(providerId: string) {
  applyProviderPreset(form, providerId)
  showProviderList.value = false
}

function onAddModel() {
  if (addModelToForm(form, newModelName.value)) {
    newModelName.value = ''
  }
}

function onRemoveModel(index: number) {
  removeModelFromForm(form, index)
}

function setDefaultModel(name: string) {
  form.default_model = name
  form.probe_model = name
}

// ── API Key 管理 ──────────────────────────────────────────────

function addApiKey() {
  const idx = apiKeys.value.length + 1
  apiKeys.value.push({ id: `key-${idx}`, api_key: '', has_key: false })
}

function removeApiKey(index: number) {
  apiKeys.value.splice(index, 1)
}

function getPrimaryApiKey(): string {
  return apiKeys.value.length > 0 ? apiKeys.value[0].api_key || '' : form.api_key || ''
}

// ── 数据加载 ──────────────────────────────────────────────────

function populateFromApi(row: any) {
  Object.assign(form, fromApiBackend(row))
  captureConnectivitySnapshot()

  apiKeys.value = []
  if (row.account_pool_summary && row.account_pool_summary.total_accounts > 0) {
    accountPool.strategy = row.account_pool_summary.strategy || 'round_robin'
    loadAccountDetails(row.id)
  } else {
    accountPool.strategy = 'round_robin'
    if (form.has_api_key) {
      apiKeys.value.push({ id: 'key-1', api_key: '', has_key: true })
    }
  }
}

async function loadAccountDetails(backendId: string) {
  try {
    const res: any = await api.get(`/api/v1/backends/${backendId}/accounts`)
    const accounts = res?.accounts || []
    apiKeys.value = accounts.map((acc: any) => ({
      id: acc.id,
      api_key: '',
      has_key: true,
    }))
  } catch {
    if (apiKeys.value.length === 0) {
      apiKeys.value.push({ id: 'key-1', api_key: '', has_key: form.has_api_key })
    }
  }
}

// ── 保存 ──────────────────────────────────────────────────────

async function saveAccountPool(backendId: string) {
  try {
    const existing: any = await api.get(`/api/v1/backends/${backendId}/accounts`)
    const existingAccounts = existing?.accounts || []
    const existingIds = new Set(existingAccounts.map((a: any) => a.id))
    const newIds = new Set(apiKeys.value.map((k) => k.id))

    for (const key of apiKeys.value) {
      const payload: any = { id: key.id, enabled: true, weight: 1 }
      if (key.api_key) payload.api_key = key.api_key
      if (existingIds.has(key.id)) {
        await api.put(`/api/v1/backends/${backendId}/accounts/${key.id}`, payload)
      } else {
        await api.post(`/api/v1/backends/${backendId}/accounts`, payload)
      }
    }

    for (const existingAcc of existingAccounts) {
      if (!newIds.has(existingAcc.id)) {
        await api.delete(`/api/v1/backends/${backendId}/accounts/${existingAcc.id}`)
      }
    }
  } catch (err: any) {
    console.error('Failed to save account pool', err)
    ElMessage.warning('账户池保存失败：' + (err.message || '未知错误'))
  }
}

const openCreate = () => {
  isCreate.value = true
  resetForm()
  apiKeys.value.push({ id: 'key-1', api_key: '', has_key: false })
  dialogVisible.value = true
}

const openEdit = (row: any) => {
  isCreate.value = false
  resetForm()
  populateFromApi(row)
  dialogVisible.value = true
}

function onDialogClose() {
  resetForm()
}

onMounted(() => {
  loadBackendTypes()
})

watch(
  () => [props.modelValue, props.backend, props.create] as const,
  ([visible, backend, create]) => {
    if (!visible) return
    if (create) {
      isCreate.value = true
      resetForm()
      apiKeys.value.push({ id: 'key-1', api_key: '', has_key: false })
    } else if (backend) {
      isCreate.value = false
      populateFromApi(backend)
    }
  }
)

const save = async () => {
  const check = validateProviderForm(form, {
    isCreate: isCreate.value,
    requireApiKey: true,
  })
  if (!check.ok) {
    ElMessage.warning(check.errors[0] || '请完善表单')
    return
  }

  form.api_key = getPrimaryApiKey()
  if (form.default_model) form.probe_model = form.default_model

  saving.value = true
  try {
    if (!isCreate.value && form.enabled && connectivityChanged()) {
      const ok = await ensureEnabledBackendCanBeSaved()
      if (!ok) return
    }

    const payload = toApiBackendPayload(form, { isCreate: isCreate.value })
    let backendId = form.id
    if (isCreate.value) {
      delete (payload as { id?: string }).id
      const created: any = await api.post('/api/v1/backends', payload)
      backendId = created.id
      ElMessage.success('Provider 已添加')
      try {
        const proxyData: any = await api.get('/api/v1/config/proxy')
        const currentDefault = (proxyData?.default_backend_id ?? proxyData?.data?.default_backend_id) || ''
        if (!currentDefault && created?.id) {
          const sm = Array.isArray(created.supported_models) ? created.supported_models : (form.models || [])
          const firstSupported = sm[0] && (sm[0].actual_model || sm[0].requested_model || sm[0].name)
          const defaultModel = created.default_model || created.probe_model || form.default_model || form.probe_model || firstSupported || ''
          await api.put('/api/v1/config/proxy', { default_backend_id: created.id, default_model: defaultModel })
          ElMessage.success(`已自动设为默认后端，模型「${defaultModel || '未设置'}」`)
        }
      } catch { /* ignore */ }
    } else {
      await api.put(`/api/v1/backends/${form.id}`, payload)
      ElMessage.success('Provider 已更新')
    }

    if (!isCreate.value && backendId && apiKeys.value.length > 1) {
      await saveAccountPool(backendId)
    }

    dialogVisible.value = false
    emit('saved')
  } catch (error: any) {
    ElMessage.error('保存失败：' + (error.message || '未知错误'))
  } finally {
    saving.value = false
  }
}

const ensureEnabledBackendCanBeSaved = async (): Promise<boolean> => {
  const payload: any = {
    type: form.type,
    base_url: form.base_url,
    api_key: getPrimaryApiKey() || '',
    timeout: form.timeout || 30,
    probe_model: (form.default_model || form.probe_model || '').trim(),
  }
  if (form.id) payload.id = form.id
  try {
    await api.post('/api/v1/backends/test?update_and_save=false', payload)
    return true
  } catch (error: any) {
    ElMessage.error('保存失败：启用前连接测试未通过（' + (error.message || '未知错误') + '）')
    return false
  }
}

const fetchModelsNow = async () => {
  if (!canFetchModels.value) {
    ElMessage.warning(!(form.base_url || '').trim() ? '请先填写 Base URL' : '请先填写 API Key')
    return
  }
  fetchingModels.value = true
  try {
    const reqBody: any = {
      base_url: form.base_url,
      api_key: getPrimaryApiKey() || '',
      type: form.type || 'openai',
      timeout: form.timeout || 30,
      replace: true,
    }
    if (!isCreate.value && form.id) reqBody.backend_id = form.id
    const res = await api.post('/api/v1/backends/fetch-models', reqBody)
    const rawNames: string[] = Array.isArray(res) ? res : Array.isArray((res as any)?.data) ? (res as any).data : []
    const names = rawNames.map((n) => String(n || '').trim()).filter(Boolean)
    if (names.length === 0) {
      ElMessage.warning('远端未返回模型，可手动添加')
      return
    }
    form.models = names.map((name) => ({ name }))
    if (!names.includes(form.default_model)) {
      form.default_model = names[0]
      form.probe_model = names[0]
    }
    ElMessage.success(`已刷新 ${names.length} 个模型`)
  } catch (error: any) {
    ElMessage.error('获取模型失败：' + (error.message || '未知错误'))
  } finally {
    fetchingModels.value = false
  }
}

const copyBackendId = (id: string) => {
  navigator.clipboard.writeText(id)
  ElMessage.success('后端 ID 已复制')
}

function getPendingApiKey(backendId: string): string | undefined {
  if (dialogVisible.value && form.id === backendId) {
    return getPrimaryApiKey() || undefined
  }
  return undefined
}

defineExpose({ openEdit, openCreate, getPendingApiKey })
</script>

<style scoped>
/* ── 对话框整体 ────────────────────────────────── */

.provider-editor-dialog :deep(.el-dialog) {
  border-radius: 16px;
  overflow: hidden;
}

.provider-editor-dialog :deep(.el-dialog__header) {
  padding: 20px 24px 0;
  margin: 0;
}

.provider-editor-dialog :deep(.el-dialog__title) {
  font-size: 18px;
  font-weight: 600;
  color: #111827;
}

.provider-editor-dialog :deep(.el-dialog__body) {
  padding: 16px 24px 0;
}

.provider-editor-dialog :deep(.el-dialog__footer) {
  padding: 12px 24px 20px;
}

.provider-form-body {
  display: flex;
  flex-direction: column;
}

/* ── Tabs ──────────────────────────────────────── */

.editor-tabs :deep(.el-tabs__header) {
  margin: 0 0 16px 0;
}

.editor-tabs :deep(.el-tabs__nav-wrap::after) {
  height: 1px;
  background: #f0f0f0;
}

.editor-tabs :deep(.el-tabs__item) {
  font-size: 14px;
  font-weight: 500;
  color: #9ca3af;
  height: 40px;
}

.editor-tabs :deep(.el-tabs__item.is-active) {
  color: #111827;
  font-weight: 600;
}

.editor-tabs :deep(.el-tabs__active-bar) {
  height: 3px;
  border-radius: 3px;
}

.editor-tabs :deep(.el-tab-pane) {
  padding: 0 4px 0 0;
  max-height: 460px;
  overflow-y: auto;
  overflow-x: hidden;
}

/* ── 表单元素 ──────────────────────────────────── */

.form-group {
  margin-bottom: 16px;
  padding: 0 12px;
}

.form-label {
  display: block;
  font-size: 13px;
  font-weight: 500;
  color: #374151;
  margin-bottom: 6px;
}

.form-tip {
  font-size: 12px;
  color: #9ca3af;
  margin-top: 6px;
  display: flex;
  align-items: center;
  gap: 4px;
}

.form-row-2 {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

.form-row-3 {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

/* ── Provider 下拉 ───────────────────────────── */

.provider-dropdown {
  position: relative;
}

.provider-list {
  position: absolute;
  z-index: 20;
  left: 0;
  right: 0;
  top: calc(100% + 4px);
  max-height: 280px;
  overflow-y: auto;
  background: #fff;
  border: 1px solid #e5e7eb;
  border-radius: 12px;
  box-shadow: 0 12px 32px rgba(0, 0, 0, 0.1);
}

.provider-option {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 14px;
  cursor: pointer;
  transition: background 0.15s;
}

.provider-option:hover,
.provider-option.selected {
  background: #f8fafc;
}

.provider-icon {
  flex-shrink: 0;
  font-size: 20px;
}

.provider-option-text {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.provider-name {
  font-weight: 500;
  color: #111827;
  font-size: 13px;
}

.provider-desc {
  font-size: 12px;
  color: #9ca3af;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.provider-empty {
  padding: 14px;
  font-size: 13px;
  color: #9ca3af;
  text-align: center;
}

/* ── ID 输入框 ────────────────────────────────── */

.id-input :deep(.el-input__wrapper) {
  background: #f9fafb;
}

/* ── API Key 列表 ────────────────────────────── */

.key-list-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
  padding: 0 12px;
}

.key-list-header label {
  margin-bottom: 0;
}

.key-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: #c0c4cc;
  padding: 20px;
  text-align: center;
  background: #fafbfc;
  border-radius: 10px;
  border: 1px dashed #e4e7ed;
  margin: 0 12px;
}

.key-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 0 12px;
}

.key-card {
  background: #fafbfc;
  border: 1px solid #f0f0f0;
  border-radius: 10px;
  padding: 10px 12px;
  transition: border-color 0.2s, box-shadow 0.2s;
}

.key-card:hover {
  border-color: #e0e6ed;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.04);
}

.key-card-row {
  display: flex;
  align-items: center;
  gap: 10px;
}

.key-badge {
  flex-shrink: 0;
  width: 26px;
  height: 26px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: #fff;
  font-size: 12px;
  font-weight: 700;
}

.key-input {
  flex: 1;
}

/* ── 轮转策略 ────────────────────────────────── */

.strategy-section {
  margin: 16px 12px 0;
  padding: 14px;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 10px;
}

.strategy-desc {
  margin-top: 8px;
  font-size: 12px;
  color: #64748b;
  line-height: 1.6;
  padding: 8px 10px;
  background: #fff;
  border-radius: 6px;
  border: 1px solid #f1f5f9;
}

.strategy-hint {
  font-size: 12px;
  color: #94a3b8;
  margin-top: 10px;
}

/* ── 模型管理 ────────────────────────────────── */

.model-section {
  margin-top: 4px;
  padding: 0 12px;
}

.model-list-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 8px;
}

.model-list-head label {
  margin-bottom: 0;
  display: flex;
  align-items: center;
  gap: 6px;
}

.model-count {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 20px;
  height: 20px;
  padding: 0 6px;
  border-radius: 10px;
  background: #f3f4f6;
  color: #6b7280;
  font-size: 11px;
  font-weight: 600;
}

.model-add-row {
  display: flex;
  gap: 8px;
  margin-bottom: 10px;
}

.model-add-row :deep(.el-input) {
  flex: 1;
}

.model-preview {
  height: 160px;
  min-height: 100px;
  max-height: 320px;
  overflow-x: hidden;
  overflow-y: auto;
  resize: vertical;
  padding: 12px;
  border: 1px solid #f0f0f0;
  border-radius: 10px;
  background: #fafbfc;
  overscroll-behavior: contain;
}

.model-preview.empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  height: 120px;
  min-height: 120px;
  resize: none;
  color: #c0c4cc;
  font-size: 13px;
}

.model-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  align-content: flex-start;
}

.model-tag {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 5px 12px;
  border-radius: 20px;
  background: #f3f4f6;
  color: #4b5563;
  font-size: 12px;
  cursor: pointer;
  transition: all 0.2s;
  border: 1px solid transparent;
}

.model-tag:hover {
  background: #e5e7eb;
  border-color: #d1d5db;
}

.model-tag.active {
  background: #eff6ff;
  color: #2563eb;
  border-color: #93c5fd;
}

.tag-remove {
  display: inline-flex;
  align-items: center;
  margin-left: 2px;
  color: #9ca3af;
  border-radius: 50%;
  transition: all 0.15s;
}

.tag-remove:hover {
  color: #ef4444;
  background: #fef2f2;
}

/* ── 高级配置 ────────────────────────────────── */

.advanced-section {
  margin-bottom: 20px;
  padding: 0 12px;
}

.advanced-section:last-child {
  margin-bottom: 0;
}

.section-title {
  font-size: 12px;
  font-weight: 600;
  color: #9ca3af;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  margin-bottom: 12px;
  padding-bottom: 8px;
  border-bottom: 1px solid #f0f0f0;
}

.type-hint {
  margin-top: 8px;
  padding: 8px 10px;
  background: #f9fafb;
  border-radius: 6px;
  font-size: 12px;
  color: #6b7280;
  line-height: 1.5;
}

.type-hint .hint-label {
  font-weight: 500;
  color: #4b5563;
}

.type-hint code {
  background: #f3f4f6;
  padding: 1px 5px;
  border-radius: 3px;
  font-size: 11px;
  font-family: 'SF Mono', Monaco, monospace;
}

.toggle-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 14px;
  background: #fafbfc;
  border: 1px solid #f0f0f0;
  border-radius: 10px;
  transition: border-color 0.2s;
}

.toggle-card:hover {
  border-color: #e0e6ed;
}

.toggle-info {
  display: flex;
  flex-direction: column;
}

.toggle-label {
  font-size: 13px;
  font-weight: 500;
  color: #374151;
}

.toggle-desc {
  font-size: 12px;
  color: #9ca3af;
  margin-top: 1px;
}

.kimi-hint {
  margin-bottom: 16px;
}

.gemini-hint {
  margin-top: 16px;
}

/* ── Footer ───────────────────────────────────── */

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

/* ── 过渡动画 ──────────────────────────────────── */

.key-list-enter-active,
.key-list-leave-active {
  transition: all 0.3s ease;
}

.key-list-enter-from {
  opacity: 0;
  transform: translateY(-8px);
}

.key-list-leave-to {
  opacity: 0;
  transform: translateX(20px);
}
</style>

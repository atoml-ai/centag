<template>
  <el-dialog
    v-model="dialogVisible"
    :title="isCreate ? '添加 Provider' : '编辑 Provider'"
    width="560px"
    class="provider-editor-dialog"
    @close="onDialogClose"
  >
    <!-- 共用主流程：与 backends export 同一套字段与交互 -->
    <div class="provider-form-body">
      <!-- Provider 搜索选择（创建模式） -->
      <div v-if="isCreate" class="form-group">
        <label>Provider *</label>
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
              <span class="provider-name">{{ p.name }}</span>
              <span class="provider-desc">{{ p.description }}</span>
            </div>
            <div v-if="filteredProviders.length === 0" class="provider-empty">
              未找到匹配的 Provider，可继续手动填写
            </div>
          </div>
        </div>
      </div>

      <!-- 编辑模式：显示 ID + 名称 -->
      <template v-else>
        <div class="form-group" v-if="form.id">
          <label>后端 ID</label>
          <el-input :model-value="form.id" readonly>
            <template #append>
              <el-button @click="copyBackendId(form.id)">复制</el-button>
            </template>
          </el-input>
        </div>
        <div class="form-group">
          <label>名称 *</label>
          <el-input v-model="form.name" placeholder="Provider 名称" autocomplete="off" />
        </div>
      </template>

      <div class="form-group">
        <label>Base URL *</label>
        <el-input v-model="form.base_url" placeholder="https://api.example.com/v1" autocomplete="off" />
        <el-alert
          v-if="isKimiForCodingEndpoint"
          type="info"
          :closable="false"
          show-icon
          class="kimi-hint"
          title="Kimi for Coding 需单独会员与专用 API Key"
          description="请在 kimi.com/code 开通会员，并使用该页面生成的 Key。"
        />
      </div>

      <div class="form-group">
        <label>API Key {{ isCreate && form.type !== 'ollama' ? '*' : '' }}</label>
        <div class="api-key-row">
          <el-input
            v-model="form.api_key"
            type="password"
            show-password
            :placeholder="apiKeyPlaceholder"
            autocomplete="new-password"
          />
          <el-tag v-if="!isCreate && form.has_api_key && !form.api_key" size="small" type="success">已设置</el-tag>
        </div>
      </div>

      <div class="form-group">
        <label>默认模型 *</label>
        <el-input
          v-model="form.default_model"
          placeholder="输入模型名称，如 gpt-4o"
          autocomplete="off"
          list="gateway-model-suggestions"
        />
        <datalist id="gateway-model-suggestions">
          <option v-for="m in form.models" :key="m.name" :value="m.name" />
        </datalist>
        <div class="form-tip">点击下方模型标签可快速设为默认</div>
      </div>

      <div class="form-group">
        <label>模型列表</label>
        <div class="model-add-row">
          <el-input
            v-model="newModelName"
            placeholder="输入模型名称"
            autocomplete="off"
            @keyup.enter="onAddModel"
          />
          <el-button @click="onAddModel" :disabled="!newModelName.trim()">添加</el-button>
        </div>
        <div class="model-preview">
          <div v-if="form.models.length > 0" class="model-tags">
            <span
              v-for="(model, mi) in form.models"
              :key="model.name + '-' + mi"
              class="model-tag clickable"
              :class="{ active: model.name === form.default_model }"
              @click="setDefaultModel(model.name)"
            >
              {{ model.name }}
              <span class="tag-remove" @click.stop="onRemoveModel(mi)">×</span>
            </span>
          </div>
          <div v-else class="model-empty-tip">暂无模型，请在上方添加或选择预设 Provider</div>
        </div>
      </div>

      <!-- Gemini 原生说明 -->
      <el-alert
        v-if="form.type === 'gemini'"
        type="success"
        :closable="false"
        show-icon
        class="gemini-hint"
        title="Gemini 原生 — 官方 / CLI 推荐"
        description="直接接入 Gemini 官方 API，支持所有原生特性。Gemini CLI 用户应选此类型。"
      />

      <!-- Gateway 专有能力：折叠，不打断主流程 -->
      <el-collapse v-model="advancedOpen" class="advanced-collapse">
        <el-collapse-item title="高级选项（Gateway）" name="advanced">
          <div class="form-group">
            <label>类型</label>
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
            <label>默认探测模型</label>
            <el-input
              v-model="form.probe_model"
              placeholder="留空则使用默认模型"
              autocomplete="off"
            />
          </div>
          <div class="form-group">
            <label>描述</label>
            <el-input v-model="form.description" type="textarea" :rows="2" placeholder="可选" />
          </div>
          <div class="advanced-grid">
            <div class="form-group">
              <label>超时（秒）</label>
              <el-input-number v-model="form.timeout" :min="1" :max="600" style="width: 100%" />
            </div>
            <div class="form-group">
              <label>最大重试</label>
              <el-input-number v-model="form.max_retries" :min="0" :max="10" style="width: 100%" />
            </div>
            <div class="form-group">
              <label>启用</label>
              <div><el-switch v-model="form.enabled" /></div>
            </div>
          </div>
          <div class="form-group">
            <label>模型同步</label>
            <div class="fetch-row">
              <el-switch v-model="form.auto_fetch_models" />
              <span class="switch-label">自动获取模型</span>
              <el-button
                size="small"
                :loading="fetchingModels"
                :disabled="!form.base_url"
                @click="fetchModelsNow"
              >
                从服务器获取
              </el-button>
            </div>
          </div>
        </el-collapse-item>
      </el-collapse>
    </div>

    <template #footer>
      <el-button @click="dialogVisible = false">取消</el-button>
      <el-button type="primary" :loading="saving" :disabled="!canSave" @click="save">
        {{ isCreate ? '添加' : '保存' }}
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
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

/** 简易 click-outside */
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
const advancedOpen = ref<string[]>([])
const backendTypes = ref<BackendTypeMeta[]>([])

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

const apiKeyPlaceholder = computed(() => {
  if (isCreate.value) return 'sk-...（勿含 Bearer 前缀）'
  if (form.has_api_key) return '已设置，输入新值可替换，留空保持不变'
  return '未设置，请输入 API Key'
})

const selectedTypeMeta = computed(() =>
  backendTypes.value.find((bt) => bt.type === form.type) || null
)

function onTypeChanged(type: string) {
  const meta = backendTypes.value.find((bt) => bt.type === type)
  if (meta && meta.default_base_url && !form.base_url) {
    form.base_url = meta.default_base_url
  }
}

/** 与 Minimal 共用校验；Gateway 编辑时允许沿用已有 Key */
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
  advancedOpen.value = []
}

function loadBackendTypes() {
  listBackendTypes()
    .then((types) => {
      backendTypes.value = Array.isArray(types) ? types : []
    })
    .catch((err) => {
      console.error('Failed to load backend types', err)
    })
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
  // 与 default_model 同步：后端以 probe_model 作为 PreferredDefaultModel 持久化
  form.probe_model = name
}

function populateFromApi(row: any) {
  Object.assign(form, fromApiBackend(row))
}

const openCreate = () => {
  isCreate.value = true
  resetForm()
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
    } else if (backend) {
      isCreate.value = false
      populateFromApi(backend)
    }
  }
)

/**
 * Gateway 分叉点：共用 toApiBackendPayload 构建语义一致的载荷，
 * 然后 POST/PUT /api/v1/backends → DB（BackendStore / DBBackendStore）。
 * Minimal 则在 backends export 用 toBackendEntry → ConfigBuilder → YAML。
 */
const save = async () => {
  const check = validateProviderForm(form, {
    isCreate: isCreate.value,
    requireApiKey: true,
  })
  if (!check.ok) {
    ElMessage.warning(check.errors[0] || '请完善表单')
    return
  }

  // 默认模型是真源：始终同步到 probe_model（高级项「探测模型」未单独设计为独立持久化字段）
  if (form.default_model) {
    form.probe_model = form.default_model
  }

  saving.value = true
  try {
    if (!isCreate.value && form.enabled) {
      const ok = await ensureEnabledBackendCanBeSaved()
      if (!ok) return
    }

    const payload = toApiBackendPayload(form, { isCreate: isCreate.value })
    if (isCreate.value) {
      // 双重保险：创建时绝不用 preset id（如 bigmodel），避免与种子数据冲突
      delete (payload as { id?: string }).id
      const created: any = await api.post('/api/v1/backends', payload)
      ElMessage.success('Provider 已添加')
      // 检查是否已有默认后端，没有则自动设为默认
      try {
        const proxyData: any = await api.get('/api/v1/config/proxy')
        // 拦截器可能返回 {success,data} 或直接返回 data
        const currentDefault = (proxyData?.default_backend_id ?? proxyData?.data?.default_backend_id) || ''
        if (!currentDefault && created?.id) {
          const sm = Array.isArray(created.supported_models) ? created.supported_models : (form.models || [])
          const firstSupported = sm[0] && (sm[0].actual_model || sm[0].requested_model || sm[0].name)
          const defaultModel = created.default_model || created.probe_model || form.default_model || form.probe_model || firstSupported || ''
          await api.put('/api/v1/config/proxy', {
            default_backend_id: created.id,
            default_model: defaultModel
          })
          ElMessage.success(`已自动将「${created.name || created.id}」设为默认后端，模型「${defaultModel || '未设置'}」`)
        }
      } catch { /* ignore */ }
    } else {
      const updated: any = await api.put(`/api/v1/backends/${form.id}`, payload)
      ElMessage.success('Provider 已更新')
      // 若正在编辑的是系统默认后端，同步 proxy 配置中的 default_model
      // 优先用保存回包的 probe/default_model，避免表单态与落盘不一致
      const savedModel =
        updated?.default_model ||
        updated?.probe_model ||
        form.default_model ||
        form.probe_model ||
        ''
      await syncProxyDefaultIfNeeded(form.id, savedModel)
    }
    dialogVisible.value = false
    emit('saved')
  } catch (error: any) {
    ElMessage.error('保存失败：' + (error.message || '未知错误'))
  } finally {
    saving.value = false
  }
}

/** 编辑默认后端时，把用户新选的默认模型写回 /api/v1/config/proxy */
async function syncProxyDefaultIfNeeded(backendId: string, defaultModel: string) {
  if (!backendId) return
  try {
    const proxyData: any = await api.get('/api/v1/config/proxy')
    const data = proxyData?.data ?? proxyData
    const currentDefaultId = (data?.default_backend_id || '').trim()
    if (currentDefaultId !== backendId) return

    const nextModel = (defaultModel || '').trim()
    const currentModel = (data?.default_model || '').trim()
    if (!nextModel || nextModel === currentModel) return

    await api.put('/api/v1/config/proxy', {
      default_backend_id: backendId,
      default_model: nextModel
    })
  } catch {
    /* 后端已保存成功；proxy 同步失败不阻断主流程 */
  }
}

const ensureEnabledBackendCanBeSaved = async (): Promise<boolean> => {
  const payload: any = {
    type: form.type,
    base_url: form.base_url,
    api_key: form.api_key || '',
    timeout: form.timeout || 30,
    probe_model: (form.default_model || form.probe_model || '').trim(),
  }
  if (form.id) payload.id = form.id

  try {
    await api.post('/api/v1/backends/test', payload)
    return true
  } catch (error: any) {
    ElMessage.error('保存失败：启用前连接测试未通过（' + (error.message || '未知错误') + '）')
    return false
  }
}

const fetchModelsNow = async () => {
  if (!form.base_url) {
    ElMessage.warning('请先填写 Base URL')
    return
  }
  fetchingModels.value = true
  try {
    const reqBody: any = {
      base_url: form.base_url,
      api_key: form.api_key || '',
      type: form.type,
      timeout: form.timeout || 30,
    }
    if (!isCreate.value && form.id) {
      reqBody.backend_id = form.id
    }
    const res = await api.post('/api/v1/backends/fetch-models', reqBody)
    const rawNames: string[] = Array.isArray(res) ? res : []
    if (rawNames.length === 0) {
      ElMessage.warning('远端未返回模型，可手动添加')
      return
    }
    const existing = new Set(form.models.map((m) => m.name))
    for (const name of rawNames) {
      if (!existing.has(name)) {
        addModelToForm(form, name)
      }
    }
    if (!form.default_model && form.models.length > 0) {
      form.default_model = form.models[0].name
    }
    ElMessage.success(`已同步 ${rawNames.length} 个模型`)
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
  if (dialogVisible.value && form.id === backendId && form.api_key) {
    return form.api_key
  }
  return undefined
}

defineExpose({
  openEdit,
  openCreate,
  getPendingApiKey,
})
</script>

<style scoped>
.provider-form-body {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.form-group label {
  display: block;
  font-size: 13px;
  font-weight: 500;
  color: #374151;
  margin-bottom: 6px;
}

.form-tip {
  font-size: 12px;
  color: #9ca3af;
  margin-top: 4px;
}

.provider-dropdown {
  position: relative;
}

.provider-list {
  position: absolute;
  z-index: 20;
  left: 0;
  right: 0;
  top: calc(100% + 4px);
  max-height: 260px;
  overflow-y: auto;
  background: #fff;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.08);
}

.provider-option {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  cursor: pointer;
}

.provider-option:hover,
.provider-option.selected {
  background: #f3f4f6;
}

.provider-icon {
  flex-shrink: 0;
}

.provider-name {
  font-weight: 500;
  color: #111827;
}

.provider-desc {
  margin-left: auto;
  font-size: 12px;
  color: #9ca3af;
  max-width: 45%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.provider-empty {
  padding: 12px;
  font-size: 13px;
  color: #9ca3af;
  text-align: center;
}

.api-key-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.api-key-row :deep(.el-input) {
  flex: 1;
}

.model-add-row {
  display: flex;
  gap: 8px;
}

.model-add-row :deep(.el-input) {
  flex: 1;
}

.model-preview {
  margin-top: 10px;
  min-height: 36px;
}

.model-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.model-tag {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 10px;
  border-radius: 999px;
  background: #f3f4f6;
  color: #4b5563;
  font-size: 12px;
}

.model-tag.clickable {
  cursor: pointer;
}

.model-tag.clickable:hover {
  background: #e5e7eb;
}

.model-tag.active {
  background: #ecf5ff;
  color: #409eff;
  border: 1px solid #a0cfff;
}

.tag-remove {
  margin-left: 2px;
  font-weight: 700;
  color: #9ca3af;
}

.tag-remove:hover {
  color: #f56c6c;
}

.model-empty-tip {
  font-size: 12px;
  color: #9ca3af;
}

.kimi-hint {
  margin-top: 8px;
}

.gemini-hint {
  margin-bottom: 4px;
}

.type-hint {
  margin-top: 6px;
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
}

.advanced-collapse {
  margin-top: 4px;
  border: none;
}

.advanced-collapse :deep(.el-collapse-item__header) {
  font-size: 13px;
  color: #6b7280;
  height: 36px;
  border: none;
}

.advanced-collapse :deep(.el-collapse-item__wrap) {
  border: none;
}

.advanced-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
}

.fetch-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.switch-label {
  font-size: 13px;
  color: #4b5563;
}
</style>

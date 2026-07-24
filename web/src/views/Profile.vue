<template>
  <div class="profile-page">
    <div class="page-header" style="margin-bottom:var(--spacing-lg)">
      <h1 class="page-title">{{ $t('profile.title') }}</h1>
      <p class="page-description">{{ $t('profile.subtitle') }}</p>
    </div>

    <el-row :gutter="24">
      <el-col :xs="24" :lg="9">
        <el-card shadow="never" class="p-card">
          <template #header>
            <div class="p-hd">
              <span class="p-icon p-icon--info"><el-icon><User /></el-icon></span>
              <div>
                <div class="p-hd-title">{{ $t('profile.basicInfo') }}</div>
                <div class="p-hd-sub">{{ $t('profile.basicInfoDesc') }}</div>
              </div>
            </div>
          </template>

          <div class="p-hero">
            <el-avatar :size="64" :style="avatarStyle">{{ avatarText }}</el-avatar>
            <div class="p-hero-info">
              <div class="p-hero-name">{{ authStore.displayName }}</div>
              <div class="p-hero-meta">
                <span class="p-hero-uname">@{{ authStore.user?.username }}</span>
                <el-tag :type="authStore.isAdmin ? 'danger' : 'primary'" size="small" effect="light">
                  {{ authStore.isAdmin ? $t('profile.admin') : $t('profile.regularUser') }}
                </el-tag>
              </div>
            </div>
          </div>

          <el-divider style="margin:20px 0" />

          <el-form :model="profileForm" label-width="90px">
            <el-form-item :label="$t('profile.username')">
              <el-input v-model="profileForm.username" disabled>
                <template #suffix><el-icon style="color:var(--el-text-color-secondary)"><Lock /></el-icon></template>
              </el-input>
            </el-form-item>
            <el-form-item :label="$t('profile.displayName')">
              <el-input v-model="profileForm.display_name" :placeholder="$t('profile.displayNamePlaceholder')" />
            </el-form-item>
            <el-form-item :label="$t('profile.email')">
              <el-input v-model="profileForm.email" :placeholder="$t('profile.emailPlaceholder')" />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="savingProfile" @click="saveProfile">{{ $t('profile.saveInfo') }}</el-button>
            </el-form-item>
          </el-form>
        </el-card>

        <el-card shadow="never" class="p-card" style="margin-top:20px">
          <template #header>
            <div class="p-hd">
              <span class="p-icon p-icon--pwd"><el-icon><Lock /></el-icon></span>
              <div>
                <div class="p-hd-title">{{ $t('profile.changePassword') }}</div>
                <div class="p-hd-sub">{{ $t('profile.changePasswordDesc') }}</div>
              </div>
            </div>
          </template>

          <el-form :model="pwdForm" :rules="pwdRules" ref="pwdFormRef" label-width="90px">
            <el-form-item :label="$t('profile.currentPassword')" prop="old_password">
              <el-input v-model="pwdForm.old_password" type="password" show-password :placeholder="$t('profile.currentPasswordPlaceholder')" />
            </el-form-item>
            <el-form-item :label="$t('profile.newPassword')" prop="new_password">
              <el-input v-model="pwdForm.new_password" type="password" show-password :placeholder="$t('profile.newPasswordPlaceholder')" />
            </el-form-item>
            <el-form-item :label="$t('profile.confirmPassword')" prop="confirm_password">
              <el-input v-model="pwdForm.confirm_password" type="password" show-password :placeholder="$t('profile.confirmPasswordPlaceholder')" />
            </el-form-item>
            <el-form-item>
              <el-button type="warning" :loading="changingPwd" @click="changePassword">{{ $t('profile.updatePassword') }}</el-button>
            </el-form-item>
          </el-form>
        </el-card>

        <el-card shadow="never" class="p-card" style="margin-top:20px">
          <template #header>
            <div class="p-hd">
              <span class="p-icon p-icon--tenant"><el-icon><OfficeBuilding /></el-icon></span>
              <div>
                <div class="p-hd-title">{{ $t('profile.resourceStats') }}</div>
                <div class="p-hd-sub">{{ $t('profile.resourceStatsDesc') }}</div>
              </div>
              <el-tag v-if="tenant" :type="tenant.status === 'active' ? 'success' : 'warning'" size="small" style="margin-left:auto">
                {{ tenant.status === 'active' ? $t('profile.active') : tenant.status === 'suspended' ? $t('profile.paused') : tenant.status }}
              </el-tag>
            </div>
          </template>

          <el-skeleton v-if="tenantLoading" :rows="4" animated />

          <template v-else-if="tenant">
            <el-descriptions :column="1" size="small" border>
              <el-descriptions-item :label="$t('profile.tenantId')">
                <code style="font-size:12px">{{ tenant.id }}</code>
              </el-descriptions-item>
              <el-descriptions-item :label="$t('profile.tenantName')">{{ tenant.name }}</el-descriptions-item>
              <el-descriptions-item :label="$t('profile.todayRequests')">
                <span class="stat-value">{{ tenant.used_today_requests || 0 }}</span>
                <span v-if="tenant.daily_request_limit" class="stat-limit"> / {{ tenant.daily_request_limit }}</span>
              </el-descriptions-item>
              <el-descriptions-item :label="$t('profile.todayTokens')">
                <span class="stat-value">{{ tenant.used_today_tokens || 0 }}</span>
                <span v-if="tenant.daily_token_limit" class="stat-limit"> / {{ tenant.daily_token_limit }}</span>
              </el-descriptions-item>
            </el-descriptions>
          </template>

          <el-empty v-else :description="$t('profile.noTenantInfo')" :image-size="60" />
        </el-card>
      </el-col>

      <el-col :xs="24" :lg="15">
        <el-card shadow="never" class="p-card">
          <template #header>
            <div class="p-hd">
              <span class="p-icon p-icon--key"><el-icon><Key /></el-icon></span>
              <div>
                <div class="p-hd-title">{{ $t('profile.apiKeyManagement') }}</div>
                <div class="p-hd-sub">{{ $t('profile.apiKeyManagementDesc') }}</div>
              </div>
              <el-button type="primary" size="small" @click="showCreateDialog = true" style="margin-left:auto">
                <el-icon><Plus /></el-icon>{{ $t('profile.createKey') }}
              </el-button>
            </div>
          </template>

          <el-alert type="info" :closable="false" show-icon style="margin-bottom:16px;border-radius:6px">
            <template #title>{{ $t('profile.apiKeyHint') }}</template>
          </el-alert>

          <el-table :data="apiKeys" v-loading="loadingKeys" :empty-text="$t('profile.noApiKeys')" stripe size="large">
            <el-table-column :label="$t('profile.name')" prop="name" min-width="110">
              <template #default="{ row }"><span style="font-weight:500">{{ row.name }}</span></template>
            </el-table-column>
            <el-table-column :label="$t('profile.key')" min-width="220">
              <template #default="{ row }">
                <div class="key-cell">
                  <code class="masked-key">{{ row.masked_key }}</code>
                  <el-tooltip
                    :content="row.reveal_available ? $t('profile.viewFullKey') : $t('profile.fullKeyHint')"
                    placement="top"
                  >
                    <el-button
                      type="primary" link size="small"
                      class="copy-key-btn"
                      @click="handleCopyAPIKey(row)"
                    >
                      <el-icon><CopyDocument /></el-icon>
                    </el-button>
                  </el-tooltip>
                </div>
              </template>
            </el-table-column>
            <el-table-column :label="$t('profile.status')" width="80" align="center">
              <template #default="{ row }">
                <el-tag :type="row.enabled ? 'success' : 'info'" size="small" effect="plain">
                  {{ row.enabled ? $t('profile.enabled') : $t('profile.disabled') }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column :label="$t('profile.expiresAt')" min-width="130">
              <template #default="{ row }">
                <span v-if="row.expires_at">{{ row.expires_at }}</span>
                <span v-else class="text-muted">{{ $t('profile.neverExpires') }}</span>
              </template>
            </el-table-column>
            <el-table-column :label="$t('profile.lastUsed')" min-width="130">
              <template #default="{ row }">
                <span v-if="row.last_used_at">{{ row.last_used_at }}</span>
                <span v-else class="text-muted">{{ $t('profile.neverUsed') }}</span>
              </template>
            </el-table-column>
            <el-table-column :label="$t('profile.actions')" width="130" align="center" fixed="right">
              <template #default="{ row }">
                <el-button
                  v-if="row.reveal_available"
                  type="primary"
                  link
                  size="small"
                  @click="handleCopyAPIKey(row)"
                >
                  {{ $t('profile.copyKey') }}
                </el-button>
                <el-dropdown trigger="click">
                  <el-button type="primary" link>
                    <el-icon><MoreFilled /></el-icon>
                  </el-button>
                  <template #dropdown>
                    <el-dropdown-menu>
                      <el-dropdown-item v-if="row.reveal_available" @click="viewFullKey(row)">
                        <el-icon><View /></el-icon>{{ $t('profile.viewFullKeyAction') }}
                      </el-dropdown-item>
                      <el-dropdown-item @click="toggleKey(row)">
                        <el-icon><Edit /></el-icon>
                        {{ row.enabled ? $t('profile.disableKey') : $t('profile.enableKey') }}
                      </el-dropdown-item>
                      <el-dropdown-item divided @click="deleteKey(row)">
                        <el-icon><Delete /></el-icon>{{ $t('profile.deleteKey') }}
                      </el-dropdown-item>
                    </el-dropdown-menu>
                  </template>
                </el-dropdown>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-col>
    </el-row>

    <el-dialog v-model="showCreateDialog" :title="$t('profile.createApiKey')" width="440px" :close-on-click-modal="false" destroy-on-close>
      <el-form :model="createKeyForm" :rules="createKeyRules" ref="createKeyFormRef" label-width="80px">
        <el-form-item :label="$t('profile.name')" prop="name">
          <el-input v-model="createKeyForm.name" :placeholder="$t('profile.keyNamePlaceholder')" />
        </el-form-item>
        <el-form-item :label="$t('profile.validity')">
          <el-select v-model="createKeyForm.expires_in" style="width:100%">
            <el-option :label="$t('profile.neverExpires')" :value="0" />
            <el-option :label="$t('profile.days7')" :value="7" />
            <el-option :label="$t('profile.days30')" :value="30" />
            <el-option :label="$t('profile.days90')" :value="90" />
            <el-option :label="$t('profile.days180')" :value="180" />
            <el-option :label="$t('profile.year1')" :value="365" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreateDialog = false">{{ $t('profile.cancel') }}</el-button>
        <el-button type="primary" :loading="creatingKey" @click="createAPIKey">{{ $t('profile.createKeyButton') }}</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="showKeyDialog" :title="$t('profile.keyCreated')" width="540px" :close-on-click-modal="false" :close-on-press-escape="false" :show-close="false">
      <el-alert type="warning" :closable="false" show-icon style="margin-bottom:16px;border-radius:6px">
        <template #title>{{ $t('profile.keyCreatedHint') }}</template>
      </el-alert>
      <div class="new-key-box">
        <code class="new-key">{{ newFullKey }}</code>
        <el-button type="primary" plain size="small" @click="copyKey"><el-icon><CopyDocument /></el-icon>{{ $t('profile.copyKey') }}</el-button>
      </div>
      <template #footer>
        <el-button type="primary" @click="showKeyDialog = false">{{ $t('profile.savedAndClose') }}</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="showRevealDialog" :title="$t('profile.fullApiKey')" width="540px" :close-on-click-modal="false" destroy-on-close>
      <el-skeleton v-if="revealLoading" :rows="2" animated />
      <div v-else class="new-key-box">
        <code class="new-key">{{ revealedFullKey }}</code>
        <el-button type="primary" plain size="small" @click="copyRevealedKey">
          <el-icon><CopyDocument /></el-icon>{{ $t('profile.copyKey') }}
        </el-button>
      </div>
      <template #footer>
        <el-button type="primary" @click="showRevealDialog = false">{{ $t('profile.cancel') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { User, Lock, Key, Plus, CopyDocument, MoreFilled, Edit, Delete, View } from '@element-plus/icons-vue'
import type { FormInstance, FormRules } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import {
  getProfile, updateProfile, changePassword as apiChangePassword,
  listAPIKeys, getAPIKey, createAPIKey as apiCreateAPIKey, updateAPIKey, deleteAPIKey as apiDeleteAPIKey
} from '@/api/user'
import type { APIKey } from '@/api/user'
import {
  getCurrentTenant,
  getCurrentQuota,
  type Tenant,
  type TenantQuota
} from '@/api/tenant'

const { t } = useI18n()

const authStore = useAuthStore()

const avatarText = computed(() => authStore.displayName.charAt(0).toUpperCase() || 'U')
const avatarStyle = computed(() => ({
  background: authStore.isAdmin
    ? 'linear-gradient(135deg,#f093fb 0%,#f5576c 100%)'
    : 'linear-gradient(135deg,#4facfe 0%,#00f2fe 100%)',
  color: '#fff', fontWeight: '700', fontSize: '24px'
}))

const profileForm = reactive({ username: '', display_name: '', email: '' })
const savingProfile = ref(false)

const tenantLoading = ref(false)
const tenant = ref<(Tenant & TenantQuota) | null>(null)
const quota = ref<TenantQuota | null>(null)

const formatDate = (date?: string) => {
  if (!date) return '-'
  return new Date(date).toLocaleString()
}

async function writeClipboard(text: string): Promise<boolean> {
  if (!text) return false
  try {
    await navigator.clipboard.writeText(text)
    return true
  } catch {
    try {
      const ta = document.createElement('textarea')
      ta.value = text
      ta.setAttribute('readonly', '')
      ta.style.position = 'fixed'
      ta.style.left = '-9999px'
      document.body.appendChild(ta)
      ta.select()
      const ok = document.execCommand('copy')
      document.body.removeChild(ta)
      return ok
    } catch {
      return false
    }
  }
}

const copyText = async (text: string) => {
  if (await writeClipboard(text)) ElMessage.success(t('profile.copied'))
  else ElMessage.warning(t('profile.copyFailed'))
}

const loadTenantInfo = async () => {
  tenantLoading.value = true
  try {
    const [tenantRes, quotaRes] = await Promise.all([
      getCurrentTenant().catch(() => null),
      getCurrentQuota().catch(() => null)
    ])
    const detail = tenantRes && typeof tenantRes === 'object' ? tenantRes : null
    const quotaData = quotaRes ?? detail?.quota ?? null
    if (detail?.tenant) {
      tenant.value = { ...detail.tenant, ...(quotaData ?? {}) }
    }
    if (quotaData) quota.value = quotaData
  } catch (error) {
    console.error('加载租户信息失败:', error)
  } finally {
    tenantLoading.value = false
  }
}

onMounted(async () => {
  try {
    const u = await getProfile()
    profileForm.username = u.username
    profileForm.display_name = u.display_name
    profileForm.email = u.email
  } catch (e: any) { ElMessage.error(e.message || t('profile.loadUserInfoFailed')) }
  loadAPIKeys()
  loadTenantInfo()
})

async function saveProfile() {
  savingProfile.value = true
  try {
    const updated = await updateProfile({ display_name: profileForm.display_name, email: profileForm.email })
    authStore.updateUser(updated)
    ElMessage.success(t('profile.infoSaved'))
  } catch (e: any) { ElMessage.error(e.message || t('profile.saveFailed')) }
  finally { savingProfile.value = false }
}

const pwdFormRef = ref<FormInstance>()
const pwdForm = reactive({ old_password: '', new_password: '', confirm_password: '' })
const changingPwd = ref(false)
const pwdRules: FormRules = {
  old_password: [{ required: true, message: t('profile.enterCurrentPassword'), trigger: 'blur' }],
  new_password: [{ required: true, message: t('profile.enterNewPassword'), trigger: 'blur' }, { min: 6, message: t('profile.passwordMinLength'), trigger: 'blur' }],
  confirm_password: [
    { required: true, message: t('profile.confirmNewPassword'), trigger: 'blur' },
    { validator: (_: any, v: string, cb: Function) => { v !== pwdForm.new_password ? cb(new Error(t('profile.passwordMismatch'))) : cb() }, trigger: 'blur' }
  ]
}
async function changePassword() {
  if (!pwdFormRef.value) return
  try {
    await pwdFormRef.value.validate()
    changingPwd.value = true
    await apiChangePassword({ old_password: pwdForm.old_password, new_password: pwdForm.new_password })
    ElMessage.success(t('profile.passwordChanged'))
    pwdForm.old_password = ''; pwdForm.new_password = ''; pwdForm.confirm_password = ''
    setTimeout(() => authStore.logout(), 1800)
  } catch (e: any) { if (e?.message) ElMessage.error(e.message) }
  finally { changingPwd.value = false }
}

const apiKeys = ref<APIKey[]>([])
const loadingKeys = ref(false)
async function loadAPIKeys() {
  loadingKeys.value = true
  try { apiKeys.value = await listAPIKeys() }
  catch (e: any) { ElMessage.error(e.message || t('profile.operationFailed')) }
  finally { loadingKeys.value = false }
}

const showCreateDialog = ref(false)
const createKeyFormRef = ref<FormInstance>()
const createKeyForm = reactive({ name: '', expires_in: 0 })
const createKeyRules: FormRules = { name: [{ required: true, message: t('profile.enterKeyName'), trigger: 'blur' }] }
const creatingKey = ref(false)
const showKeyDialog = ref(false)
const newFullKey = ref('')
const showRevealDialog = ref(false)
const revealLoading = ref(false)
const revealedFullKey = ref('')

async function createAPIKey() {
  if (!createKeyFormRef.value) return
  try {
    await createKeyFormRef.value.validate()
    creatingKey.value = true
    const created = await apiCreateAPIKey({ name: createKeyForm.name, expires_in: createKeyForm.expires_in > 0 ? createKeyForm.expires_in : undefined })
    newFullKey.value = created.full_key
    showCreateDialog.value = false; showKeyDialog.value = true
    createKeyForm.name = ''; createKeyForm.expires_in = 0
    loadAPIKeys()
  } catch (e: any) { if (e?.message) ElMessage.error(e.message) }
  finally { creatingKey.value = false }
}

async function toggleKey(key: APIKey) {
  try {
    await updateAPIKey(key.id, { enabled: !key.enabled })
    ElMessage.success(t('profile.keyDisabledEnabled'))
    loadAPIKeys()
  } catch (e: any) { ElMessage.error(e.message || t('profile.operationFailed')) }
}

async function deleteKey(key: APIKey) {
  try {
    await ElMessageBox.confirm(t('profile.confirmDelete', { name: key.name }), t('profile.deleteConfirm'), { confirmButtonText: t('profile.deleteKey'), cancelButtonText: t('profile.cancel'), type: 'warning' })
    await apiDeleteAPIKey(key.id)
    ElMessage.success(t('profile.keyDeleted')); loadAPIKeys()
  } catch (e: any) { if (e !== 'cancel' && e?.message) ElMessage.error(e.message) }
}

async function copyKey() {
  await copyText(newFullKey.value)
}

async function revealFullKey(row: APIKey, autoCopy: boolean) {
  showRevealDialog.value = true
  revealLoading.value = true
  revealedFullKey.value = ''
  try {
    const d = await getAPIKey(row.id)
    if (d.full_key) {
      revealedFullKey.value = d.full_key
      if (autoCopy) await copyText(d.full_key)
    } else {
      ElMessage.warning(t('profile.cannotViewFullKey'))
      showRevealDialog.value = false
    }
  } catch (e: any) {
    ElMessage.error(e.message || t('profile.operationFailed'))
    showRevealDialog.value = false
  } finally {
    revealLoading.value = false
  }
}

async function handleCopyAPIKey(row: APIKey) {
  if (!row.reveal_available) {
    ElMessage.warning(t('profile.keyViewWarning'))
    return
  }
  await revealFullKey(row, true)
}

async function viewFullKey(row: APIKey) {
  await revealFullKey(row, false)
}

async function copyRevealedKey() {
  await copyText(revealedFullKey.value)
}
</script>

<style scoped>
.p-card { width: 100%; }

.p-hd {
  display: flex;
  align-items: center;
  gap: 12px;
}

.p-icon {
  width: 36px;
  height: 36px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 17px;
  flex-shrink: 0;
}

.p-icon--info { background: rgba(79,172,254,.12); color: #4facfe; }
.p-icon--pwd  { background: rgba(240,147,251,.12); color: #f5576c; }
.p-icon--key  { background: rgba(102,126,234,.12); color: #667eea; }
.p-icon--tenant { background: rgba(103,194,58,.12); color: #67c23a; }

.p-hd-title { font-size: .9375rem; font-weight: 600; color: var(--el-text-color-primary); }
.p-hd-sub   { font-size: .8125rem; color: var(--el-text-color-secondary); margin-top: 2px; }

.p-hero {
  display: flex;
  align-items: center;
  gap: 16px;
}
.p-hero-name { font-size: 1.125rem; font-weight: 600; color: var(--el-text-color-primary); margin-bottom: 6px; }
.p-hero-meta { display: flex; align-items: center; gap: 8px; }
.p-hero-uname { font-size: .8125rem; color: var(--el-text-color-secondary); }

.key-cell {
  display: flex;
  align-items: center;
  gap: 6px;
}
.masked-key {
  font-family: 'Menlo','Monaco','Consolas',monospace;
  font-size: .8125rem;
  background: var(--el-fill-color-light);
  padding: 3px 8px;
  border-radius: 4px;
  letter-spacing: .5px;
}
.copy-key-btn {
  opacity: 0.55;
  transition: opacity .15s;
  flex-shrink: 0;
}
.key-cell:hover .copy-key-btn {
  opacity: 1;
}

.new-key-box {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  background: var(--el-fill-color-lighter);
  border: 1px solid var(--el-border-color-light);
  padding: 14px 16px;
  border-radius: 8px;
}
.new-key {
  flex: 1;
  font-family: 'Menlo','Monaco','Consolas',monospace;
  font-size: .875rem;
  word-break: break-all;
  color: var(--el-color-success-dark-2);
  line-height: 1.6;
}

.stat-mini {
  text-align: center;
  padding: 8px;
  background: var(--el-fill-color-lighter);
  border-radius: 6px;
}
.stat-mini-value {
  font-size: 18px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}
.stat-mini-label {
  font-size: 11px;
  color: var(--el-text-color-secondary);
  margin-top: 4px;
}
</style>

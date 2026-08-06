<template>
  <div class="profile-page">
    <div class="page-header">
      <h1 class="page-title">{{ $t('profile.title') }}</h1>
      <p class="page-description">{{ $t('profile.subtitle') }}</p>
    </div>

    <div class="profile-layout">
      <!-- 左侧导航（类 GitHub Settings） -->
      <aside class="profile-nav" aria-label="Profile sections">
        <nav class="nav-list">
          <button
            v-for="item in navItems"
            :key="item.id"
            type="button"
            class="nav-item"
            :class="{ 'is-active': activeSection === item.id }"
            @click="selectSection(item.id)"
          >
            <el-icon class="nav-icon"><component :is="item.icon" /></el-icon>
            <span class="nav-label">{{ $t(item.labelKey) }}</span>
          </button>
        </nav>
      </aside>

      <!-- 右侧内容 -->
      <main class="profile-main">
        <!-- 基本信息 -->
        <section v-show="activeSection === 'basic'" class="section-panel section-panel--center">
          <div class="section-center">
            <header class="section-header">
              <h2 class="section-title">{{ $t('profile.basicInfo') }}</h2>
              <p class="section-desc">{{ $t('profile.basicInfoDesc') }}</p>
            </header>

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

            <el-divider />

            <el-form :model="profileForm" label-width="100px" class="section-form">
              <el-form-item :label="$t('profile.username')">
                <el-input v-model="profileForm.username" disabled>
                  <template #suffix>
                    <el-icon style="color:var(--el-text-color-secondary)"><Lock /></el-icon>
                  </template>
                </el-input>
              </el-form-item>
              <el-form-item :label="$t('profile.displayName')">
                <el-input v-model="profileForm.display_name" :placeholder="$t('profile.displayNamePlaceholder')" />
              </el-form-item>
              <el-form-item :label="$t('profile.email')">
                <el-input v-model="profileForm.email" :placeholder="$t('profile.emailPlaceholder')" />
              </el-form-item>
              <el-form-item>
                <el-button type="primary" :loading="savingProfile" @click="saveProfile">
                  {{ $t('profile.saveInfo') }}
                </el-button>
              </el-form-item>
            </el-form>
          </div>
        </section>

        <!-- 修改密码 -->
        <section v-show="activeSection === 'password'" class="section-panel section-panel--center">
          <div class="section-center section-center--narrow">
            <header class="section-header">
              <h2 class="section-title">{{ $t('profile.changePassword') }}</h2>
              <p class="section-desc">{{ $t('profile.changePasswordDesc') }}</p>
            </header>

            <el-form
              ref="pwdFormRef"
              :model="pwdForm"
              :rules="pwdRules"
              label-width="100px"
              class="section-form"
            >
              <el-form-item :label="$t('profile.currentPassword')" prop="old_password">
                <el-input
                  v-model="pwdForm.old_password"
                  type="password"
                  show-password
                  :placeholder="$t('profile.currentPasswordPlaceholder')"
                />
              </el-form-item>
              <el-form-item :label="$t('profile.newPassword')" prop="new_password">
                <el-input
                  v-model="pwdForm.new_password"
                  type="password"
                  show-password
                  :placeholder="$t('profile.newPasswordPlaceholder')"
                />
              </el-form-item>
              <el-form-item :label="$t('profile.confirmPassword')" prop="confirm_password">
                <el-input
                  v-model="pwdForm.confirm_password"
                  type="password"
                  show-password
                  :placeholder="$t('profile.confirmPasswordPlaceholder')"
                />
              </el-form-item>
              <el-form-item>
                <el-button type="warning" :loading="changingPwd" @click="changePassword">
                  {{ $t('profile.updatePassword') }}
                </el-button>
              </el-form-item>
            </el-form>
          </div>
        </section>

        <!-- API Key -->
        <section v-show="activeSection === 'api-keys'" class="section-panel">
          <header class="section-header section-header--row">
            <div>
              <h2 class="section-title">{{ $t('profile.apiKeyManagement') }}</h2>
              <p class="section-desc">{{ $t('profile.apiKeyManagementDesc') }}</p>
            </div>
            <el-button type="primary" size="small" @click="showCreateDialog = true">
              <el-icon><Plus /></el-icon>{{ $t('profile.createKey') }}
            </el-button>
          </header>

          <el-alert type="info" :closable="false" show-icon class="section-alert">
            <template #title>{{ $t('profile.apiKeyHint') }}</template>
          </el-alert>

          <el-table
            class="api-keys-table"
            :data="apiKeys"
            v-loading="loadingKeys"
            :empty-text="$t('profile.noApiKeys')"
            stripe
            size="large"
            style="width: 100%"
          >
            <el-table-column :label="$t('profile.name')" prop="name" min-width="96" show-overflow-tooltip>
              <template #default="{ row }"><span style="font-weight:500">{{ row.name }}</span></template>
            </el-table-column>
            <el-table-column :label="$t('profile.key')" min-width="134">
              <template #default="{ row }">
                <div class="key-cell">
                  <el-tooltip :content="row.masked_key || row.key_prefix" placement="top" :show-after="400">
                    <code class="masked-key">{{ formatShortAPIKey(row) }}</code>
                  </el-tooltip>
                  <el-tooltip :content="$t('profile.copyKey')" placement="top">
                    <el-button
                      type="primary"
                      link
                      size="small"
                      class="copy-key-btn"
                      :disabled="!row.reveal_available"
                      @click.stop="handleCopyAPIKey(row)"
                    >
                      <el-icon><CopyDocument /></el-icon>
                    </el-button>
                  </el-tooltip>
                </div>
              </template>
            </el-table-column>
            <el-table-column :label="$t('profile.status')" width="72" align="center">
              <template #default="{ row }">
                <el-tag :type="row.enabled ? 'success' : 'info'" size="small" effect="plain">
                  {{ row.enabled ? $t('profile.enabled') : $t('profile.disabled') }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column :label="$t('profile.expiresAt')" width="105" show-overflow-tooltip>
              <template #default="{ row }">
                <span v-if="row.expires_at" class="cell-compact">{{ row.expires_at }}</span>
                <span v-else class="text-muted">{{ $t('profile.neverExpires') }}</span>
              </template>
            </el-table-column>
            <el-table-column :label="$t('profile.lastUsed')" width="105" show-overflow-tooltip>
              <template #default="{ row }">
                <span v-if="row.last_used_at" class="cell-compact">{{ row.last_used_at }}</span>
                <span v-else class="text-muted">{{ $t('profile.neverUsed') }}</span>
              </template>
            </el-table-column>
            <el-table-column :label="$t('profile.actions')" width="64" align="center">
              <template #default="{ row }">
                <el-dropdown trigger="click">
                  <el-button type="primary" link>
                    <el-icon><MoreFilled /></el-icon>
                  </el-button>
                  <template #dropdown>
                    <el-dropdown-menu>
                      <el-dropdown-item
                        :disabled="!row.reveal_available"
                        @click="viewFullKey(row)"
                      >
                        <el-icon><View /></el-icon>{{ $t('profile.viewFullKeyAction') }}
                      </el-dropdown-item>
                      <el-dropdown-item
                        :disabled="!row.reveal_available"
                        @click="handleCopyAPIKey(row)"
                      >
                        <el-icon><CopyDocument /></el-icon>{{ $t('profile.copyKey') }}
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
        </section>
      </main>
    </div>

    <el-dialog
      v-model="showCreateDialog"
      :title="$t('profile.createApiKey')"
      width="440px"
      :close-on-click-modal="false"
      destroy-on-close
    >
      <el-form ref="createKeyFormRef" :model="createKeyForm" :rules="createKeyRules" label-width="80px">
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
        <el-button type="primary" :loading="creatingKey" @click="createAPIKey">
          {{ $t('profile.createKeyButton') }}
        </el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="showKeyDialog"
      :title="$t('profile.keyCreated')"
      width="540px"
      :close-on-click-modal="false"
      :close-on-press-escape="false"
      :show-close="false"
    >
      <el-alert type="warning" :closable="false" show-icon style="margin-bottom:16px;border-radius:6px">
        <template #title>{{ $t('profile.keyCreatedHint') }}</template>
      </el-alert>
      <div class="new-key-box">
        <code class="new-key">{{ newFullKey }}</code>
        <el-button type="primary" plain size="small" @click="copyKey">
          <el-icon><CopyDocument /></el-icon>{{ $t('profile.copyKey') }}
        </el-button>
      </div>
      <template #footer>
        <el-button type="primary" @click="showKeyDialog = false">{{ $t('profile.savedAndClose') }}</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="showRevealDialog"
      :title="$t('profile.fullApiKey')"
      width="540px"
      :close-on-click-modal="false"
      destroy-on-close
    >
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
import { ref, reactive, computed, onMounted, watch, type Component } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  User, Lock, Key, Plus, CopyDocument, MoreFilled, Edit, Delete, View,
} from '@element-plus/icons-vue'
import type { FormInstance, FormRules } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import {
  getProfile, updateProfile, changePassword as apiChangePassword,
  listAPIKeys, getAPIKey, createAPIKey as apiCreateAPIKey, updateAPIKey, deleteAPIKey as apiDeleteAPIKey,
} from '@/api/user'
import type { APIKey } from '@/api/user'
import { copyToClipboard } from '@/utils/clipboard'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()

type ProfileSection = 'basic' | 'password' | 'api-keys'

const navItems: Array<{ id: ProfileSection; labelKey: string; icon: Component }> = [
  { id: 'basic', labelKey: 'profile.navBasic', icon: User },
  { id: 'password', labelKey: 'profile.navPassword', icon: Lock },
  { id: 'api-keys', labelKey: 'profile.navApiKeys', icon: Key },
]

const validSections = new Set<ProfileSection>(navItems.map(i => i.id))

function parseSection(raw: unknown): ProfileSection {
  const v = String(raw || '')
  return validSections.has(v as ProfileSection) ? (v as ProfileSection) : 'basic'
}

const activeSection = ref<ProfileSection>(parseSection(route.query.section))

function selectSection(id: ProfileSection) {
  activeSection.value = id
  router.replace({ query: { ...route.query, section: id } })
}

watch(
  () => route.query.section,
  (v) => {
    const next = parseSection(v)
    if (next !== activeSection.value) activeSection.value = next
  }
)

const avatarText = computed(() => authStore.displayName.charAt(0).toUpperCase() || 'U')
const avatarStyle = computed(() => ({
  background: authStore.isAdmin
    ? 'linear-gradient(135deg,#f093fb 0%,#f5576c 100%)'
    : 'linear-gradient(135deg,#4facfe 0%,#00f2fe 100%)',
  color: '#fff',
  fontWeight: '700',
  fontSize: '24px',
}))

const profileForm = reactive({ username: '', display_name: '', email: '' })
const savingProfile = ref(false)

const copyText = async (text: string) => {
  // 静态导入 copyToClipboard：动态 import 会打断用户手势，HTTP 局域网下 Clipboard API 不可用时回退也会失败
  if (await copyToClipboard(text)) ElMessage.success(t('profile.copied'))
  else ElMessage.warning(t('profile.copyFailed'))
}

onMounted(async () => {
  if (!route.query.section) {
    router.replace({ query: { ...route.query, section: activeSection.value } })
  }
  try {
    const u = await getProfile()
    profileForm.username = u.username
    profileForm.display_name = u.display_name
    profileForm.email = u.email
  } catch (e: any) {
    ElMessage.error(e.message || t('profile.loadUserInfoFailed'))
  }
  loadAPIKeys()
})

async function saveProfile() {
  savingProfile.value = true
  try {
    const updated = await updateProfile({
      display_name: profileForm.display_name,
      email: profileForm.email,
    })
    authStore.updateUser(updated)
    ElMessage.success(t('profile.infoSaved'))
  } catch (e: any) {
    ElMessage.error(e.message || t('profile.saveFailed'))
  } finally {
    savingProfile.value = false
  }
}

const pwdFormRef = ref<FormInstance>()
const pwdForm = reactive({ old_password: '', new_password: '', confirm_password: '' })
const changingPwd = ref(false)
const pwdRules: FormRules = {
  old_password: [{ required: true, message: t('profile.enterCurrentPassword'), trigger: 'blur' }],
  new_password: [
    { required: true, message: t('profile.enterNewPassword'), trigger: 'blur' },
    { min: 6, message: t('profile.passwordMinLength'), trigger: 'blur' },
  ],
  confirm_password: [
    { required: true, message: t('profile.confirmNewPassword'), trigger: 'blur' },
    {
      validator: (_: any, v: string, cb: Function) => {
        v !== pwdForm.new_password ? cb(new Error(t('profile.passwordMismatch'))) : cb()
      },
      trigger: 'blur',
    },
  ],
}

async function changePassword() {
  if (!pwdFormRef.value) return
  try {
    await pwdFormRef.value.validate()
    changingPwd.value = true
    await apiChangePassword({
      old_password: pwdForm.old_password,
      new_password: pwdForm.new_password,
    })
    ElMessage.success(t('profile.passwordChanged'))
    pwdForm.old_password = ''
    pwdForm.new_password = ''
    pwdForm.confirm_password = ''
    setTimeout(() => authStore.logout(), 1800)
  } catch (e: any) {
    if (e?.message) ElMessage.error(e.message)
  } finally {
    changingPwd.value = false
  }
}

const apiKeys = ref<APIKey[]>([])
const loadingKeys = ref(false)

/** 列表展示：直接用后端脱敏（可解密时前后位数更长）；单元格内铺满 */
function formatShortAPIKey(row: APIKey): string {
  const masked = (row.masked_key || row.key_prefix || '').trim().replace(/\.\.\./g, '…')
  return masked || '••••'
}

async function loadAPIKeys() {
  loadingKeys.value = true
  try {
    apiKeys.value = await listAPIKeys()
  } catch (e: any) {
    ElMessage.error(e.message || t('profile.operationFailed'))
  } finally {
    loadingKeys.value = false
  }
}

const showCreateDialog = ref(false)
const createKeyFormRef = ref<FormInstance>()
const createKeyForm = reactive({ name: '', expires_in: 0 })
const createKeyRules: FormRules = {
  name: [{ required: true, message: t('profile.enterKeyName'), trigger: 'blur' }],
}
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
    const created = await apiCreateAPIKey({
      name: createKeyForm.name,
      expires_in: createKeyForm.expires_in > 0 ? createKeyForm.expires_in : undefined,
    })
    newFullKey.value = created.full_key
    showCreateDialog.value = false
    showKeyDialog.value = true
    createKeyForm.name = ''
    createKeyForm.expires_in = 0
    loadAPIKeys()
  } catch (e: any) {
    if (e?.message) ElMessage.error(e.message)
  } finally {
    creatingKey.value = false
  }
}

async function toggleKey(key: APIKey) {
  try {
    await updateAPIKey(key.id, { enabled: !key.enabled })
    ElMessage.success(t('profile.keyDisabledEnabled'))
    loadAPIKeys()
  } catch (e: any) {
    ElMessage.error(e.message || t('profile.operationFailed'))
  }
}

async function deleteKey(key: APIKey) {
  try {
    await ElMessageBox.confirm(
      t('profile.confirmDelete', { name: key.name }),
      t('profile.deleteConfirm'),
      {
        confirmButtonText: t('profile.deleteKey'),
        cancelButtonText: t('profile.cancel'),
        type: 'warning',
      }
    )
    await apiDeleteAPIKey(key.id)
    ElMessage.success(t('profile.keyDeleted'))
    loadAPIKeys()
  } catch (e: any) {
    if (e !== 'cancel' && e?.message) ElMessage.error(e.message)
  }
}

async function copyKey() {
  await copyText(newFullKey.value)
}

async function viewFullKey(row: APIKey) {
  if (!row.reveal_available) {
    ElMessage.warning(t('profile.keyViewWarning'))
    return
  }
  showRevealDialog.value = true
  revealLoading.value = true
  revealedFullKey.value = ''
  try {
    const d = await getAPIKey(row.id)
    if (d.full_key) {
      revealedFullKey.value = d.full_key
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

/** 静默拉取并复制，不打开「查看密钥」对话框 */
async function handleCopyAPIKey(row: APIKey) {
  if (!row.reveal_available) {
    ElMessage.warning(t('profile.keyViewWarning'))
    return
  }
  try {
    const d = await getAPIKey(row.id)
    if (d.full_key) {
      await copyText(d.full_key)
    } else {
      ElMessage.warning(t('profile.cannotViewFullKey'))
    }
  } catch (e: any) {
    ElMessage.error(e.message || t('profile.operationFailed'))
  }
}

async function copyRevealedKey() {
  await copyText(revealedFullKey.value)
}
</script>

<style scoped>
.profile-page {
  max-width: 1100px;
  margin: 0 auto;
  width: 100%;
  padding: 0 0 24px;
}

.page-header {
  margin-bottom: 24px;
}

.page-title {
  margin: 0;
  font-size: 1.5rem;
  font-weight: 600;
}

.page-description {
  margin: 6px 0 0;
  color: var(--el-text-color-secondary);
  font-size: 0.875rem;
}

.profile-layout {
  display: grid;
  grid-template-columns: 220px minmax(0, 1fr);
  gap: 24px;
  align-items: start;
}

.profile-nav {
  position: sticky;
  top: 16px;
}

.nav-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 8px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 10px;
  background: var(--el-bg-color);
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  border: none;
  background: transparent;
  text-align: left;
  padding: 10px 12px;
  border-radius: 8px;
  cursor: pointer;
  color: var(--el-text-color-regular);
  font-size: 0.875rem;
  transition: background 0.15s, color 0.15s;
}

.nav-item:hover {
  background: var(--el-fill-color-light);
  color: var(--el-text-color-primary);
}

.nav-item.is-active {
  background: var(--el-color-primary-light-9);
  color: var(--el-color-primary);
  font-weight: 600;
}

.nav-icon {
  font-size: 16px;
  flex-shrink: 0;
}

.profile-main {
  min-width: 0;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 10px;
  background: var(--el-bg-color);
  padding: 24px;
}

.section-panel--center {
  display: flex;
  justify-content: center;
}

.section-center {
  width: 100%;
  max-width: 520px;
}

.section-center--narrow {
  max-width: 440px;
}

.section-header {
  margin-bottom: 20px;
}

.section-header--row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.section-title {
  margin: 0;
  font-size: 1.125rem;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.section-desc {
  margin: 6px 0 0;
  font-size: 0.8125rem;
  color: var(--el-text-color-secondary);
}

.section-alert {
  margin-bottom: 16px;
  border-radius: 6px;
}

.section-form {
  width: 100%;
}

.cell-compact {
  display: inline-block;
  max-width: 100%;
  font-size: 0.75rem;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  vertical-align: bottom;
}

.p-hero {
  display: flex;
  align-items: center;
  gap: 16px;
}

.p-hero-name {
  font-size: 1.125rem;
  font-weight: 600;
  color: var(--el-text-color-primary);
  margin-bottom: 6px;
}

.p-hero-meta {
  display: flex;
  align-items: center;
  gap: 8px;
}

.p-hero-uname {
  font-size: 0.8125rem;
  color: var(--el-text-color-secondary);
}

.api-keys-table {
  width: 100%;
}

.api-keys-table :deep(.el-table__body-wrapper) {
  overflow-x: hidden;
}

.key-cell {
  display: flex;
  align-items: center;
  gap: 4px;
  min-width: 0;
}

.masked-key {
  flex: 1;
  min-width: 0;
  display: block;
  box-sizing: border-box;
  font-family: 'Menlo', 'Monaco', 'Consolas', monospace;
  font-size: 0.75rem;
  background: var(--el-fill-color-light);
  padding: 3px 8px;
  border-radius: 4px;
  letter-spacing: 0.2px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  cursor: default;
}

.copy-key-btn {
  flex-shrink: 0;
  padding: 0 2px;
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
  font-family: 'Menlo', 'Monaco', 'Consolas', monospace;
  font-size: 0.875rem;
  word-break: break-all;
  color: var(--el-color-success-dark-2);
  line-height: 1.6;
}

.text-muted {
  color: var(--el-text-color-secondary);
}

@media (max-width: 768px) {
  .profile-layout {
    grid-template-columns: 1fr;
  }

  .profile-nav {
    position: static;
  }

  .nav-list {
    flex-direction: row;
    overflow-x: auto;
    gap: 4px;
  }

  .nav-item {
    white-space: nowrap;
    flex-shrink: 0;
  }

  .profile-main {
    padding: 16px;
  }
}
</style>

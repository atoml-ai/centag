<template>
  <div class="profile-page">
    <div class="page-header" style="margin-bottom:var(--spacing-lg)">
      <h1 class="page-title">个人中心</h1>
      <p class="page-description">管理您的账号信息与 API 访问密钥</p>
    </div>

    <el-row :gutter="24">
      <!-- 左侧：基本信息 + 修改密码 -->
      <el-col :xs="24" :lg="9">
        <!-- 基本信息 -->
        <el-card shadow="never" class="p-card">
          <template #header>
            <div class="p-hd">
              <span class="p-icon p-icon--info"><el-icon><User /></el-icon></span>
              <div>
                <div class="p-hd-title">基本信息</div>
                <div class="p-hd-sub">个人资料与联系方式</div>
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
                  {{ authStore.isAdmin ? '管理员' : '普通用户' }}
                </el-tag>
              </div>
            </div>
          </div>

          <el-divider style="margin:20px 0" />

          <el-form :model="profileForm" label-width="90px">
            <el-form-item label="用户名">
              <el-input v-model="profileForm.username" disabled>
                <template #suffix><el-icon style="color:var(--el-text-color-secondary)"><Lock /></el-icon></template>
              </el-input>
            </el-form-item>
            <el-form-item label="显示名称">
              <el-input v-model="profileForm.display_name" placeholder="显示在界面上的名称" />
            </el-form-item>
            <el-form-item label="邮箱">
              <el-input v-model="profileForm.email" placeholder="联系邮箱（可选）" />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="savingProfile" @click="saveProfile">保存信息</el-button>
            </el-form-item>
          </el-form>
        </el-card>

        <!-- 修改密码 -->
        <el-card shadow="never" class="p-card" style="margin-top:20px">
          <template #header>
            <div class="p-hd">
              <span class="p-icon p-icon--pwd"><el-icon><Lock /></el-icon></span>
              <div>
                <div class="p-hd-title">修改密码</div>
                <div class="p-hd-sub">定期更换密码可提升账号安全性</div>
              </div>
            </div>
          </template>

          <el-form :model="pwdForm" :rules="pwdRules" ref="pwdFormRef" label-width="90px">
            <el-form-item label="当前密码" prop="old_password">
              <el-input v-model="pwdForm.old_password" type="password" show-password placeholder="请输入当前密码" />
            </el-form-item>
            <el-form-item label="新密码" prop="new_password">
              <el-input v-model="pwdForm.new_password" type="password" show-password placeholder="至少 6 位" />
            </el-form-item>
            <el-form-item label="确认密码" prop="confirm_password">
              <el-input v-model="pwdForm.confirm_password" type="password" show-password placeholder="再次输入新密码" />
            </el-form-item>
            <el-form-item>
              <el-button type="warning" :loading="changingPwd" @click="changePassword">修改密码</el-button>
            </el-form-item>
          </el-form>
        </el-card>

        <!-- 租户信息 -->
        <el-card shadow="never" class="p-card" style="margin-top:20px">
          <template #header>
            <div class="p-hd">
              <span class="p-icon p-icon--tenant"><el-icon><OfficeBuilding /></el-icon></span>
              <div>
                <div class="p-hd-title">资源统计</div>
                <div class="p-hd-sub">用量统计与配额信息</div>
              </div>
              <el-tag v-if="tenant" :type="tenant.status === 'active' ? 'success' : 'warning'" size="small" style="margin-left:auto">
                {{ tenant.status === 'active' ? '活跃' : tenant.status === 'suspended' ? '暂停' : tenant.status }}
              </el-tag>
            </div>
          </template>

          <el-skeleton v-if="tenantLoading" :rows="4" animated />
          
          <template v-else-if="tenant">
            <el-descriptions :column="1" size="small" border>
              <el-descriptions-item label="租户 ID">
                <code style="font-size:12px">{{ tenant.id }}</code>
              </el-descriptions-item>
              <el-descriptions-item label="租户名称">{{ tenant.name }}</el-descriptions-item>
              <el-descriptions-item label="今日请求">
                <span class="stat-value">{{ tenant.used_today_requests || 0 }}</span>
                <span v-if="tenant.daily_request_limit" class="stat-limit"> / {{ tenant.daily_request_limit }}</span>
              </el-descriptions-item>
              <el-descriptions-item label="今日 Token">
                <span class="stat-value">{{ tenant.used_today_tokens || 0 }}</span>
                <span v-if="tenant.daily_token_limit" class="stat-limit"> / {{ tenant.daily_token_limit }}</span>
              </el-descriptions-item>
            </el-descriptions>
          </template>

          <el-empty v-else description="暂无租户信息" :image-size="60" />
        </el-card>
      </el-col>

      <!-- 右侧：API Key 管理 -->
      <el-col :xs="24" :lg="15">
        <el-card shadow="never" class="p-card">
          <template #header>
            <div class="p-hd">
              <span class="p-icon p-icon--key"><el-icon><Key /></el-icon></span>
              <div>
                <div class="p-hd-title">API Key 管理</div>
                <div class="p-hd-sub">生成密钥用于访问代理接口</div>
              </div>
              <el-button type="primary" size="small" @click="showCreateDialog = true" style="margin-left:auto">
                <el-icon><Plus /></el-icon>新建密钥
              </el-button>
            </div>
          </template>

          <el-alert type="info" :closable="false" show-icon style="margin-bottom:16px;border-radius:6px">
            <template #title>点击密钥旁的复制按钮或操作菜单中的「查看完整密钥」，可在对话框中复制完整 API Key。桌面版与网页版行为一致。</template>
          </el-alert>

          <el-table :data="apiKeys" v-loading="loadingKeys" empty-text="暂无 API Key，点击「新建密钥」创建" stripe size="large">
            <el-table-column label="名称" prop="name" min-width="110">
              <template #default="{ row }"><span style="font-weight:500">{{ row.name }}</span></template>
            </el-table-column>
            <el-table-column label="密钥" min-width="220">
              <template #default="{ row }">
                <div class="key-cell">
                  <code class="masked-key">{{ row.masked_key }}</code>
                  <el-tooltip
                    :content="row.reveal_available ? '查看并复制完整密钥' : '完整密钥仅创建时可见，请新建密钥'"
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
            <el-table-column label="状态" width="80" align="center">
              <template #default="{ row }">
                <el-tag :type="row.enabled ? 'success' : 'info'" size="small" effect="plain">
                  {{ row.enabled ? '启用' : '禁用' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="过期时间" min-width="130">
              <template #default="{ row }">
                <span v-if="row.expires_at">{{ row.expires_at }}</span>
                <span v-else class="text-muted">永不过期</span>
              </template>
            </el-table-column>
            <el-table-column label="最近使用" min-width="130">
              <template #default="{ row }">
                <span v-if="row.last_used_at">{{ row.last_used_at }}</span>
                <span v-else class="text-muted">从未使用</span>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="130" align="center" fixed="right">
              <template #default="{ row }">
                <el-button
                  v-if="row.reveal_available"
                  type="primary"
                  link
                  size="small"
                  @click="handleCopyAPIKey(row)"
                >
                  复制
                </el-button>
                <el-dropdown trigger="click">
                  <el-button type="primary" link>
                    <el-icon><MoreFilled /></el-icon>
                  </el-button>
                  <template #dropdown>
                    <el-dropdown-menu>
                      <el-dropdown-item v-if="row.reveal_available" @click="viewFullKey(row)">
                        <el-icon><View /></el-icon>查看完整密钥
                      </el-dropdown-item>
                      <el-dropdown-item @click="toggleKey(row)">
                        <el-icon><Edit /></el-icon>
                        {{ row.enabled ? '禁用密钥' : '启用密钥' }}
                      </el-dropdown-item>
                      <el-dropdown-item divided @click="deleteKey(row)">
                        <el-icon><Delete /></el-icon>删除密钥
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

    <!-- 创建 API Key 对话框 -->
    <el-dialog v-model="showCreateDialog" title="新建 API Key" width="440px" :close-on-click-modal="false" destroy-on-close>
      <el-form :model="createKeyForm" :rules="createKeyRules" ref="createKeyFormRef" label-width="80px">
        <el-form-item label="名称" prop="name">
          <el-input v-model="createKeyForm.name" placeholder="为这个密钥起个名字，方便识别" />
        </el-form-item>
        <el-form-item label="有效期">
          <el-select v-model="createKeyForm.expires_in" style="width:100%">
            <el-option label="永不过期" :value="0" />
            <el-option label="7 天" :value="7" />
            <el-option label="30 天" :value="30" />
            <el-option label="90 天" :value="90" />
            <el-option label="180 天" :value="180" />
            <el-option label="1 年" :value="365" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreateDialog = false">取消</el-button>
        <el-button type="primary" :loading="creatingKey" @click="createAPIKey">创建密钥</el-button>
      </template>
    </el-dialog>

    <!-- 新 Key 展示（仅一次） -->
    <el-dialog v-model="showKeyDialog" title="密钥创建成功" width="540px" :close-on-click-modal="false" :close-on-press-escape="false" :show-close="false">
      <el-alert type="warning" :closable="false" show-icon style="margin-bottom:16px;border-radius:6px">
        <template #title>请保存此密钥。默认可在列表中再次查看/复制完整内容；仅当服务端开启「仅创建时展示」（LLM_PROXY_API_KEY_REVEAL_ONCE）时例外。</template>
      </el-alert>
      <div class="new-key-box">
        <code class="new-key">{{ newFullKey }}</code>
        <el-button type="primary" plain size="small" @click="copyKey"><el-icon><CopyDocument /></el-icon>复制</el-button>
      </div>
      <template #footer>
        <el-button type="primary" @click="showKeyDialog = false">我已保存，关闭</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="showRevealDialog" title="完整 API Key" width="540px" :close-on-click-modal="false" destroy-on-close>
      <el-skeleton v-if="revealLoading" :rows="2" animated />
      <div v-else class="new-key-box">
        <code class="new-key">{{ revealedFullKey }}</code>
        <el-button type="primary" plain size="small" @click="copyRevealedKey">
          <el-icon><CopyDocument /></el-icon>复制
        </el-button>
      </div>
      <template #footer>
        <el-button type="primary" @click="showRevealDialog = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
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

const authStore = useAuthStore()

const avatarText = computed(() => authStore.displayName.charAt(0).toUpperCase() || 'U')
const avatarStyle = computed(() => ({
  background: authStore.isAdmin
    ? 'linear-gradient(135deg,#f093fb 0%,#f5576c 100%)'
    : 'linear-gradient(135deg,#4facfe 0%,#00f2fe 100%)',
  color: '#fff', fontWeight: '700', fontSize: '24px'
}))

// ── Profile ────────────────────────────────────────────────────────────────────
const profileForm = reactive({ username: '', display_name: '', email: '' })
const savingProfile = ref(false)

// ── Tenant Info ────────────────────────────────────────────────────────────────
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
  if (await writeClipboard(text)) ElMessage.success('已复制')
  else ElMessage.warning('复制失败，请手动复制对话框中的密钥')
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
  } catch (e: any) { ElMessage.error(e.message || '获取用户信息失败') }
  loadAPIKeys()
  loadTenantInfo()
})

async function saveProfile() {
  savingProfile.value = true
  try {
    const updated = await updateProfile({ display_name: profileForm.display_name, email: profileForm.email })
    authStore.updateUser(updated)
    ElMessage.success('个人信息已保存')
  } catch (e: any) { ElMessage.error(e.message || '保存失败') }
  finally { savingProfile.value = false }
}

// ── Password ──────────────────────────────────────────────────────────────────
const pwdFormRef = ref<FormInstance>()
const pwdForm = reactive({ old_password: '', new_password: '', confirm_password: '' })
const changingPwd = ref(false)
const pwdRules: FormRules = {
  old_password: [{ required: true, message: '请输入当前密码', trigger: 'blur' }],
  new_password: [{ required: true, message: '请输入新密码', trigger: 'blur' }, { min: 6, message: '密码至少 6 位', trigger: 'blur' }],
  confirm_password: [
    { required: true, message: '请再次输入新密码', trigger: 'blur' },
    { validator: (_: any, v: string, cb: Function) => { v !== pwdForm.new_password ? cb(new Error('两次密码不一致')) : cb() }, trigger: 'blur' }
  ]
}
async function changePassword() {
  if (!pwdFormRef.value) return
  try {
    await pwdFormRef.value.validate()
    changingPwd.value = true
    await apiChangePassword({ old_password: pwdForm.old_password, new_password: pwdForm.new_password })
    ElMessage.success('密码修改成功，即将重新登录…')
    pwdForm.old_password = ''; pwdForm.new_password = ''; pwdForm.confirm_password = ''
    setTimeout(() => authStore.logout(), 1800)
  } catch (e: any) { if (e?.message) ElMessage.error(e.message) }
  finally { changingPwd.value = false }
}

// ── API Keys ──────────────────────────────────────────────────────────────────
const apiKeys = ref<APIKey[]>([])
const loadingKeys = ref(false)
async function loadAPIKeys() {
  loadingKeys.value = true
  try { apiKeys.value = await listAPIKeys() }
  catch (e: any) { ElMessage.error(e.message || '获取失败') }
  finally { loadingKeys.value = false }
}

const showCreateDialog = ref(false)
const createKeyFormRef = ref<FormInstance>()
const createKeyForm = reactive({ name: '', expires_in: 0 })
const createKeyRules: FormRules = { name: [{ required: true, message: '请输入密钥名称', trigger: 'blur' }] }
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
    ElMessage.success(`密钥已${key.enabled ? '禁用' : '启用'}`)
    loadAPIKeys()
  } catch (e: any) { ElMessage.error(e.message || '操作失败') }
}

async function deleteKey(key: APIKey) {
  try {
    await ElMessageBox.confirm(`确定删除密钥「${key.name}」？`, '删除确认', { confirmButtonText: '删除', cancelButtonText: '取消', type: 'warning' })
    await apiDeleteAPIKey(key.id)
    ElMessage.success('密钥已删除'); loadAPIKeys()
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
      ElMessage.warning('无法展示完整密钥（可能创建于「仅创建时展示」模式，或存储密钥已变更；请新建密钥）')
      showRevealDialog.value = false
    }
  } catch (e: any) {
    ElMessage.error(e.message || '获取失败')
    showRevealDialog.value = false
  } finally {
    revealLoading.value = false
  }
}

async function handleCopyAPIKey(row: APIKey) {
  if (!row.reveal_available) {
    ElMessage.warning('此密钥无法再次查看完整内容，请新建密钥并在创建对话框中立即保存')
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

/* Section header in cards */
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

/* User hero */
.p-hero {
  display: flex;
  align-items: center;
  gap: 16px;
}
.p-hero-name { font-size: 1.125rem; font-weight: 600; color: var(--el-text-color-primary); margin-bottom: 6px; }
.p-hero-meta { display: flex; align-items: center; gap: 8px; }
.p-hero-uname { font-size: .8125rem; color: var(--el-text-color-secondary); }

/* API Key table */
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

/* New key */
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

/* Tenant stats */
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

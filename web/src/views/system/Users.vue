<template>
  <div class="users-page">
    <div class="header-with-toolbar">
      <div class="header-left">
        <h1 class="page-title">用户管理</h1>
        <p class="page-description">管理系统用户账号与权限</p>
      </div>
      <div class="toolbar-actions">
        <el-input
          v-model="searchText"
          placeholder="搜索用户名或显示名..."
          clearable
          style="width: 220px"
        >
          <template #prefix><el-icon><Search /></el-icon></template>
        </el-input>
        <el-select v-model="filterRole" placeholder="角色筛选" clearable style="width: 120px">
          <el-option label="全部" value="" />
          <el-option label="管理员" value="admin" />
          <el-option label="普通用户" value="normal" />
        </el-select>
        <el-button :loading="loading" @click="loadUsers">
          <el-icon><Refresh /></el-icon>刷新
        </el-button>
        <el-button type="primary" @click="openCreateDialog">
          <el-icon><Plus /></el-icon>新建用户
        </el-button>
      </div>
    </div>

    <el-card class="table-card" shadow="never" v-loading="loading">
      <el-table :data="filteredUsers" stripe size="large" empty-text="暂无用户数据">
        <el-table-column label="ID" prop="id" width="64" align="center" />

        <el-table-column label="用户" min-width="180">
          <template #default="{ row }">
            <div class="user-cell">
              <el-avatar :size="32" :style="getAvatarStyle(row)">
                {{ getAvatarText(row) }}
              </el-avatar>
              <div>
                <div class="name-title">{{ row.display_name || row.username }}</div>
                <div class="name-subtitle">@{{ row.username }}</div>
              </div>
            </div>
          </template>
        </el-table-column>

        <el-table-column label="邮箱" prop="email" min-width="160">
          <template #default="{ row }">
            <span class="text-muted">{{ row.email || '—' }}</span>
          </template>
        </el-table-column>

        <el-table-column label="角色" width="110" align="center">
          <template #default="{ row }">
            <el-tag
              :type="row.role === 'admin' ? 'danger' : 'primary'"
              size="small"
              effect="light"
            >
              {{ row.role === 'admin' ? '管理员' : '普通用户' }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column label="状态" width="90" align="center">
          <template #default="{ row }">
            <el-tag :type="row.enabled ? 'success' : 'info'" size="small" effect="plain">
              {{ row.enabled ? '启用' : '禁用' }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column label="创建时间" prop="created_at" min-width="140" />

        <el-table-column label="操作" width="80" align="center" fixed="right">
          <template #default="{ row }">
            <el-dropdown trigger="click">
              <el-button type="primary" link>
                <el-icon><MoreFilled /></el-icon>
              </el-button>
              <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item @click="openEditDialog(row)">
                  <el-icon><Edit /></el-icon>编辑用户
                </el-dropdown-item>
                <el-dropdown-item @click="openResetDialog(row)">
                  <el-icon><Key /></el-icon>重置密码
                </el-dropdown-item>
                <el-dropdown-item @click="viewUserAPIKeys(row)">
                  <el-icon><Key /></el-icon>查看API密钥
                </el-dropdown-item>
                <el-dropdown-item
                  divided
                  :disabled="row.id === authStore.user?.id"
                  @click="deleteUser(row)"
                >
                  <el-icon><Delete /></el-icon>删除用户
                </el-dropdown-item>
              </el-dropdown-menu>
              </template>
            </el-dropdown>
          </template>
        </el-table-column>
      </el-table>

      <div class="table-footer">
        <span class="text-muted">共 {{ filteredUsers.length }} 个用户</span>
      </div>
    </el-card>

    <!-- 创建 / 编辑 -->
    <el-dialog
      v-model="dialogVisible"
      :title="editingUser ? '编辑用户' : '新建用户'"
      width="480px"
      :close-on-click-modal="false"
      destroy-on-close
    >
      <el-form :model="userForm" :rules="userRules" ref="userFormRef" label-width="90px">
        <el-form-item label="用户名" prop="username">
          <el-input v-model="userForm.username" :disabled="!!editingUser" placeholder="登录用户名（创建后不可更改）" />
        </el-form-item>
        <el-form-item label="初始密码" prop="password" v-if="!editingUser">
          <el-input v-model="userForm.password" type="password" show-password placeholder="至少 6 位" />
        </el-form-item>
        <el-form-item label="显示名称">
          <el-input v-model="userForm.display_name" placeholder="用于界面展示（可选）" />
        </el-form-item>
        <el-form-item label="邮箱">
          <el-input v-model="userForm.email" placeholder="可选" />
        </el-form-item>
        <el-form-item label="角色" prop="role">
          <el-radio-group v-model="userForm.role">
            <el-radio-button value="normal">普通用户</el-radio-button>
            <el-radio-button value="admin">管理员</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="账号状态" v-if="editingUser">
          <el-switch v-model="userForm.enabled" active-text="启用" inactive-text="禁用" inline-prompt />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveUser">
          {{ editingUser ? '保存修改' : '创建用户' }}
        </el-button>
      </template>
    </el-dialog>

    <!-- 重置密码 -->
    <el-dialog
      v-model="resetDialogVisible"
      :title="`重置密码 — ${resetTargetUser?.username}`"
      width="400px"
      :close-on-click-modal="false"
      destroy-on-close
    >
      <el-alert type="warning" :closable="false" style="margin-bottom: 20px">
        重置后该用户所有登录会话将失效，需重新登录。
      </el-alert>
      <el-form :model="resetForm" :rules="resetRules" ref="resetFormRef" label-width="90px">
        <el-form-item label="新密码" prop="new_password">
          <el-input v-model="resetForm.new_password" type="password" show-password placeholder="至少 6 位" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="resetDialogVisible = false">取消</el-button>
        <el-button type="warning" :loading="resetting" @click="resetPassword">重置密码</el-button>
      </template>
    </el-dialog>
    
    <!-- 查看API密钥 -->
    <el-dialog
      v-model="apiKeyDialogVisible"
      :title="`API密钥 — ${targetUser?.username || ''}`"
      width="700px"
      :close-on-click-modal="false"
      destroy-on-close
    >
      <el-table 
        :data="userApiKeys" 
        v-loading="apiKeyLoading" 
        empty-text="该用户暂无API密钥"
        stripe 
        size="large"
      >
        <el-table-column label="名称" prop="name" min-width="120" />
        <el-table-column label="密钥前缀" prop="key_prefix" min-width="120" />
        <el-table-column label="状态" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="row.enabled ? 'success' : 'info'" size="small" effect="plain">
              {{ row.enabled ? '启用' : '禁用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" prop="created_at" width="150" />
        <el-table-column label="到期时间" width="150">
          <template #default="{ row }">
            <span>{{ row.expires_at || '—' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="最后使用" width="150">
          <template #default="{ row }">
            <span>{{ row.last_used_at || '—' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="80" align="center" fixed="right">
          <template #default="{ row }">
            <el-button 
              type="primary" 
              link 
              :disabled="!row.reveal_available"
              @click="viewFullApiKey(row)"
              v-tooltip="'查看完整密钥'"
            >
              <el-icon><View /></el-icon>
            </el-button>
          </template>
        </el-table-column>
      </el-table>
      
      <template #footer>
        <el-button @click="apiKeyDialogVisible = false">关闭</el-button>
      </template>
    </el-dialog>
    
    <!-- 查看完整API密钥 -->
    <el-dialog
      v-model="fullKeyVisible"
      title="查看完整API密钥"
      width="600px"
      :close-on-click-modal="false"
      destroy-on-close
    >
      <el-alert type="info" :closable="false" style="margin-bottom: 20px">
        请注意保护好您的API密钥，不要泄露给他人。
      </el-alert>
      <el-descriptions :column="1" border>
        <el-descriptions-item label="名称">
          {{ selectedApiKey?.name }}
        </el-descriptions-item>
        <el-descriptions-item label="完整密钥">
          <div style="position: relative;">
            <el-input 
              :model-value="selectedApiKey?.full_key || ''"
              @update:model-value="handleFullKeyUpdate"
              type="text" 
              readonly 
              :show-password="!!selectedApiKey?.full_key"
            />
          </div>
        </el-descriptions-item>
        <el-descriptions-item label="状态">
          <el-tag :type="selectedApiKey?.enabled ? 'success' : 'info'" size="small" effect="plain">
            {{ selectedApiKey?.enabled ? '启用' : '禁用' }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="创建时间">
          {{ selectedApiKey?.created_at }}
        </el-descriptions-item>
        <el-descriptions-item label="到期时间">
          {{ selectedApiKey?.expires_at || '—' }}
        </el-descriptions-item>
      </el-descriptions>
      
      <template #footer>
        <el-button @click="fullKeyVisible = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Search, Refresh, Plus, Edit, Key, Delete, MoreFilled, View } from '@element-plus/icons-vue'
import type { FormInstance, FormRules } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import {
  listUsers, createUser, updateUser,
  deleteUser as apiDeleteUser, adminResetPassword,
  listUserAPIKeys, getAdminAPIKey
} from '@/api/user'
import type { UserInfo, APIKey, APIKeyDetail } from '@/api/auth'

const authStore = useAuthStore()
const users = ref<UserInfo[]>([])
const loading = ref(false)
const searchText = ref('')
const filterRole = ref('')

const filteredUsers = computed(() =>
  users.value.filter((u) => {
    const matchRole = !filterRole.value || u.role === filterRole.value
    const q = searchText.value.toLowerCase()
    const matchText = !q || u.username.toLowerCase().includes(q) ||
      (u.display_name || '').toLowerCase().includes(q)
    return matchRole && matchText
  })
)

onMounted(loadUsers)

async function loadUsers() {
  loading.value = true
  try { users.value = await listUsers() }
  catch (e: any) { ElMessage.error(e.message || '获取用户列表失败') }
  finally { loading.value = false }
}

function getAvatarText(u: UserInfo) {
  return (u.display_name || u.username).charAt(0).toUpperCase()
}
function getAvatarStyle(u: UserInfo) {
  return {
    background: u.role === 'admin'
      ? 'linear-gradient(135deg,#f093fb 0%,#f5576c 100%)'
      : 'linear-gradient(135deg,#4facfe 0%,#00f2fe 100%)',
    color: '#fff', fontWeight: '600', fontSize: '13px', flexShrink: '0'
  }
}

// ── Create / Edit ─────────────────────────────────────────────────────────────
const dialogVisible = ref(false)
const editingUser = ref<UserInfo | null>(null)
const userFormRef = ref<FormInstance>()
const saving = ref(false)
const userForm = reactive({ username: '', password: '', display_name: '', email: '', role: 'normal' as 'admin'|'normal', enabled: true })
const userRules: FormRules = {
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }, { min: 6, message: '密码至少 6 位', trigger: 'blur' }]
}

function openCreateDialog() {
  editingUser.value = null
  Object.assign(userForm, { username: '', password: '', display_name: '', email: '', role: 'normal', enabled: true })
  dialogVisible.value = true
}
function openEditDialog(u: UserInfo) {
  editingUser.value = u
  Object.assign(userForm, { username: u.username, display_name: u.display_name, email: u.email, role: u.role, enabled: u.enabled })
  dialogVisible.value = true
}
async function saveUser() {
  if (!userFormRef.value) return
  try {
    await userFormRef.value.validate()
    saving.value = true
    if (editingUser.value) {
      await updateUser(editingUser.value.id, { display_name: userForm.display_name, email: userForm.email, role: userForm.role, enabled: userForm.enabled })
      ElMessage.success('用户信息已更新')
    } else {
      await createUser({ username: userForm.username, password: userForm.password, display_name: userForm.display_name, email: userForm.email, role: userForm.role })
      ElMessage.success('用户创建成功')
    }
    dialogVisible.value = false
    loadUsers()
  } catch (e: any) { if (e?.message) ElMessage.error(e.message) }
  finally { saving.value = false }
}

// ── Delete ────────────────────────────────────────────────────────────────────
async function deleteUser(u: UserInfo) {
  try {
    await ElMessageBox.confirm(`确定删除用户「${u.display_name || u.username}」？此操作不可撤销。`, '删除确认', { confirmButtonText: '删除', cancelButtonText: '取消', type: 'warning' })
    await apiDeleteUser(u.id)
    ElMessage.success('用户已删除')
    loadUsers()
  } catch (e: any) { if (e !== 'cancel' && e?.message) ElMessage.error(e.message) }
}

// ── Reset Password ────────────────────────────────────────────────────────────
const resetDialogVisible = ref(false)
const resetFormRef = ref<FormInstance>()
const resetting = ref(false)
const resetTargetUser = ref<UserInfo | null>(null)
const resetForm = reactive({ new_password: '' })
const resetRules: FormRules = {
  new_password: [{ required: true, message: '请输入新密码', trigger: 'blur' }, { min: 6, message: '密码至少 6 位', trigger: 'blur' }]
}
function openResetDialog(u: UserInfo) { resetTargetUser.value = u; resetForm.new_password = ''; resetDialogVisible.value = true }
async function resetPassword() {
  if (!resetFormRef.value || !resetTargetUser.value) return
  try {
    await resetFormRef.value.validate()
    resetting.value = true
    await adminResetPassword(resetTargetUser.value.id, { new_password: resetForm.new_password })
    ElMessage.success('密码已重置，用户需重新登录')
    resetDialogVisible.value = false
  } catch (e: any) { if (e?.message) ElMessage.error(e.message) }
  finally { resetting.value = false }
}

// ── View API Keys ─────────────────────────────────────────────────────────────
const apiKeyDialogVisible = ref(false)
const apiKeyLoading = ref(false)
const targetUser = ref<UserInfo | null>(null)
const userApiKeys = ref<APIKey[]>([])
const selectedApiKey = ref<APIKeyDetail | null>(null)
const fullKeyVisible = ref(false)

async function viewUserAPIKeys(user: UserInfo) {
  targetUser.value = user
  apiKeyLoading.value = true
  try {
    userApiKeys.value = await listUserAPIKeys(user.id)
    apiKeyDialogVisible.value = true
  } catch (e: any) {
    ElMessage.error(e.message || '获取API密钥列表失败')
  } finally {
    apiKeyLoading.value = false
  }
}

async function viewFullApiKey(key: APIKey) {
  try {
    const detail = await getAdminAPIKey(targetUser.value!.id, key.id)
    selectedApiKey.value = detail
    fullKeyVisible.value = true
  } catch (e: any) {
    ElMessage.error(e.message || '获取API密钥详情失败')
  }
}

function handleFullKeyUpdate(value: string) {
  if (selectedApiKey.value) {
    selectedApiKey.value.full_key = value
  }
}
</script>

<style scoped>
.header-with-toolbar {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  margin-bottom: var(--spacing-lg);
  flex-wrap: wrap;
  gap: var(--spacing-md);
}

.header-left { flex: 1; min-width: 0; }

.toolbar-actions {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
  flex-wrap: wrap;
}

.table-card { width: 100%; }

.user-cell {
  display: flex;
  align-items: center;
  gap: 10px;
}

.table-footer {
  margin-top: 16px;
  padding-top: 12px;
  border-top: 1px solid var(--el-border-color-lighter);
  display: flex;
  justify-content: flex-end;
}

@media (max-width: 768px) {
  .toolbar-actions {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: var(--spacing-sm);
  }
  .toolbar-actions :deep(.el-input),
  .toolbar-actions :deep(.el-select),
  .toolbar-actions :deep(.el-button) { width: 100% !important; }
}
</style>

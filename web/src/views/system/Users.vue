<template>
  <div class="users-page">
    <div class="header-with-toolbar">
      <div class="header-left">
        <h1 class="page-title">用户管理</h1>
        <p class="page-description">管理用户账号、限额、可用资源与自建权限</p>
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

        <el-table-column label="Token 限额" min-width="160">
          <template #default="{ row }">
            <div class="quota-cell">
              <span>日 {{ formatLimit(row.daily_token_limit) }}</span>
              <span class="text-muted">月 {{ formatLimit(row.monthly_token_limit) }}</span>
            </div>
          </template>
        </el-table-column>

        <el-table-column label="默认流水线" min-width="160">
          <template #default="{ row }">
            <span class="text-muted">{{ pipelineLabel(row.default_pipeline_id) }}</span>
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
                <el-dropdown-item @click="openManageDrawer(row)">
                  <el-icon><Setting /></el-icon>资源配置
                </el-dropdown-item>
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
    
    <!-- 查看 / 管理用户 API 密钥 -->
    <el-dialog
      v-model="apiKeyDialogVisible"
      :title="`API 密钥 — ${targetUser?.username || ''}`"
      width="780px"
      :close-on-click-modal="false"
      destroy-on-close
    >
      <el-alert type="info" :closable="false" style="margin-bottom: 12px">
        默认可再次查看/复制完整密钥。若创建于「仅创建时展示」模式或历史未加密落库，则无法二次查看，请为用户新建密钥。
      </el-alert>
      <div style="margin-bottom: 12px; text-align: right">
        <el-button type="primary" size="small" @click="openCreateKeyDialog">为该用户新建密钥</el-button>
      </div>
      <el-table
        :data="userApiKeys"
        v-loading="apiKeyLoading"
        empty-text="该用户暂无 API 密钥"
        stripe
        size="large"
      >
        <el-table-column label="名称" prop="name" min-width="100" />
        <el-table-column label="密钥" min-width="160">
          <template #default="{ row }">
            <code class="masked-key">{{ row.masked_key || row.key_prefix }}</code>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="72" align="center">
          <template #default="{ row }">
            <el-tag :type="row.enabled ? 'success' : 'info'" size="small" effect="plain">
              {{ row.enabled ? '启用' : '禁用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="预算" width="110" align="right">
          <template #default="{ row }">
            {{ Number(row.used_usd || 0).toFixed(2) }} /
            {{ row.budget_usd === 0 ? '∞' : Number(row.budget_usd || 0).toFixed(2) }}
          </template>
        </el-table-column>
        <el-table-column label="创建时间" prop="created_at" width="140" />
        <el-table-column label="操作" width="120" align="center" fixed="right">
          <template #default="{ row }">
            <el-button
              type="primary"
              link
              @click="viewFullApiKey(row)"
            >
              {{ row.reveal_available ? '查看' : '详情' }}
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <template #footer>
        <el-button @click="apiKeyDialogVisible = false">关闭</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="createKeyVisible"
      title="为用户新建 API 密钥"
      width="440px"
      :close-on-click-modal="false"
      destroy-on-close
    >
      <el-form label-width="90px">
        <el-form-item label="名称">
          <el-input v-model="createKeyForm.name" placeholder="便于识别，如 default" />
        </el-form-item>
        <el-form-item label="有效期">
          <el-select v-model="createKeyForm.expires_in" style="width: 100%">
            <el-option label="永不过期" :value="0" />
            <el-option label="30 天" :value="30" />
            <el-option label="90 天" :value="90" />
            <el-option label="365 天" :value="365" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createKeyVisible = false">取消</el-button>
        <el-button type="primary" :loading="creatingKey" @click="createKeyForUser">创建</el-button>
      </template>
    </el-dialog>
    
    <!-- 资源配置：限额 + 默认流水线 + 租户请求配额 -->
    <el-drawer
      v-model="manageVisible"
      :title="`资源配置 — ${manageUser?.display_name || manageUser?.username || ''}`"
      size="640px"
      destroy-on-close
      class="manage-drawer"
    >
      <el-tabs v-model="manageTab">
        <el-tab-pane label="Token 限额" name="token">
          <el-form label-width="120px" class="manage-form">
            <el-form-item label="限额档位">
              <el-select v-model="tokenTier" style="width: 100%" @change="onTokenTierChange">
                <el-option
                  v-for="t in TOKEN_TIERS"
                  :key="t.id"
                  :label="t.label"
                  :value="t.id"
                />
              </el-select>
            </el-form-item>
            <el-form-item label="日 Token 限额">
              <el-input-number
                v-model="manageForm.daily_token_limit"
                :min="0"
                :step="10000"
                style="width: 100%"
                @change="onTokenManualChange"
              />
              <div class="form-tip">0 表示不限制。已用：{{ formatNumber(manageUser?.daily_token_used) }}</div>
            </el-form-item>
            <el-form-item label="月 Token 限额">
              <el-input-number
                v-model="manageForm.monthly_token_limit"
                :min="0"
                :step="100000"
                style="width: 100%"
                @change="onTokenManualChange"
              />
              <div class="form-tip">0 表示不限制。已用：{{ formatNumber(manageUser?.monthly_token_used) }}</div>
            </el-form-item>
          </el-form>
        </el-tab-pane>

        <el-tab-pane label="资源与权限" name="access">
          <div class="access-panel">
            <p class="access-summary">
              已授权共用资源：
              <strong>{{ manageForm.allowed_backend_ids.length }}</strong> 后端 ·
              <strong>{{ manageForm.allowed_model_ids.length }}</strong> 模型 ·
              <strong>{{ manageForm.allowed_pipeline_ids.length }}</strong> 流水线
            </p>
            <p class="access-hint">未勾选的共用资源对该用户不可用；其自建资源不受白名单限制（需打开下方自建开关）。</p>

            <section class="access-section">
              <h3 class="access-section-title">操作权限</h3>
              <div class="perm-list">
                <div class="perm-row">
                  <div class="perm-text">
                    <div class="perm-name">添加自有后端</div>
                    <div class="perm-desc">允许用户使用自己的 API Key 创建/修改/删除后端</div>
                  </div>
                  <el-switch v-model="manageForm.can_add_own_backends" />
                </div>
                <div class="perm-row">
                  <div class="perm-text">
                    <div class="perm-name">添加自有流水线</div>
                    <div class="perm-desc">允许用户创建/修改/删除自己的流水线</div>
                  </div>
                  <el-switch v-model="manageForm.can_add_own_pipelines" />
                </div>
                <div class="perm-row">
                  <div class="perm-text">
                    <div class="perm-name">修改默认流水线</div>
                    <div class="perm-desc">允许用户在首页自行切换默认流水线</div>
                  </div>
                  <el-switch v-model="manageForm.can_change_default_pipeline" />
                </div>
              </div>
            </section>

            <section class="access-section">
              <h3 class="access-section-title">默认流水线</h3>
              <el-select
                v-model="manageForm.default_pipeline_id"
                clearable
                filterable
                placeholder="不指定则使用系统默认"
                style="width: 100%"
              >
                <el-option
                  v-for="p in defaultPipelineSelectOptions"
                  :key="p.id"
                  :label="p.name ? `${p.name}` : p.id"
                  :value="p.id"
                >
                  <span>{{ p.name || p.id }}</span>
                  <span class="option-id">{{ p.id }}</span>
                </el-option>
              </el-select>
              <p class="form-tip">仅能选择该用户可用的流水线（已授权共用 + 其自建）。</p>
            </section>

            <section class="access-section">
              <div class="access-section-head">
                <h3 class="access-section-title">共用后端与模型</h3>
                <div class="access-section-actions">
                  <el-button link type="primary" @click="selectAllSharedBackends">全选</el-button>
                  <el-button link @click="clearSharedBackends">清空</el-button>
                </div>
              </div>
              <el-input
                v-model="backendFilter"
                clearable
                placeholder="搜索后端…"
                class="access-filter"
              >
                <template #prefix><el-icon><Search /></el-icon></template>
              </el-input>
              <div v-if="filteredSharedBackends.length === 0" class="access-empty">暂无已启用的共用后端</div>
              <div v-else class="backend-pick-list">
                <div
                  v-for="b in filteredSharedBackends"
                  :key="b.id"
                  class="backend-pick"
                  :class="{ active: isBackendAllowed(b.id) }"
                >
                  <div class="backend-pick-head">
                    <el-checkbox
                      :model-value="isBackendAllowed(b.id)"
                      @change="(v: boolean | string | number) => toggleBackend(b.id, !!v)"
                    >
                      <span class="backend-pick-name">{{ b.name || b.id }}</span>
                    </el-checkbox>
                    <span class="option-id">{{ b.id }}</span>
                  </div>
                  <div v-if="isBackendAllowed(b.id)" class="model-pick-list">
                    <div class="model-pick-toolbar">
                      <span class="model-pick-label">可用模型</span>
                      <el-button
                        link
                        type="primary"
                        size="small"
                        @click="selectAllModelsForBackend(b)"
                      >全选模型</el-button>
                      <el-button
                        link
                        size="small"
                        @click="clearModelsForBackend(b)"
                      >清空</el-button>
                    </div>
                    <el-checkbox-group
                      v-if="modelsOfBackend(b).length"
                      :model-value="selectedModelsOf(b)"
                      class="model-checks"
                      @change="(vals) => setModelsForBackend(b, vals as Array<string | number | boolean>)"
                    >
                      <el-checkbox
                        v-for="m in modelsOfBackend(b)"
                        :key="m"
                        :label="m"
                        :value="m"
                      >
                        {{ m }}
                      </el-checkbox>
                    </el-checkbox-group>
                    <p v-else class="form-tip">该后端暂无模型列表，请先在后端管理中探测/配置。</p>
                  </div>
                </div>
              </div>
            </section>

            <section class="access-section">
              <div class="access-section-head">
                <h3 class="access-section-title">共用流水线</h3>
                <div class="access-section-actions">
                  <el-button link type="primary" @click="selectAllSharedPipelines">全选</el-button>
                  <el-button link @click="clearSharedPipelines">清空</el-button>
                </div>
              </div>
              <el-input
                v-model="pipelineFilter"
                clearable
                placeholder="搜索流水线…"
                class="access-filter"
              >
                <template #prefix><el-icon><Search /></el-icon></template>
              </el-input>
              <div v-if="filteredSharedPipelines.length === 0" class="access-empty">暂无共用流水线</div>
              <el-checkbox-group
                v-else
                v-model="manageForm.allowed_pipeline_ids"
                class="pipeline-checks"
              >
                <div
                  v-for="p in filteredSharedPipelines"
                  :key="p.id"
                  class="pipeline-check-row"
                >
                  <el-checkbox :label="p.id" :value="p.id">
                    <span class="pipeline-check-name">{{ p.name || p.id }}</span>
                    <span class="option-id">{{ p.id }}</span>
                  </el-checkbox>
                </div>
              </el-checkbox-group>
            </section>
          </div>
        </el-tab-pane>

        <el-tab-pane label="租户配额" name="tenant">
          <el-alert
            v-if="!manageTenantId"
            type="warning"
            :closable="false"
            title="该用户尚未关联租户，无法设置请求/资源配额。"
            style="margin-bottom: 16px"
          />
          <el-form v-else label-width="140px" class="manage-form" v-loading="tenantQuotaLoading">
            <el-form-item label="日请求限额">
              <el-input-number v-model="tenantQuotaForm.daily_request_limit" :min="0" :step="100" style="width: 100%" />
            </el-form-item>
            <el-form-item label="月请求限额">
              <el-input-number v-model="tenantQuotaForm.monthly_request_limit" :min="0" :step="1000" style="width: 100%" />
            </el-form-item>
            <el-form-item label="最大后端数">
              <el-input-number v-model="tenantQuotaForm.max_backends" :min="0" :max="1000" style="width: 100%" />
            </el-form-item>
            <el-form-item label="最大 API Key 数">
              <el-input-number v-model="tenantQuotaForm.max_api_keys" :min="0" :max="1000" style="width: 100%" />
            </el-form-item>
            <el-descriptions :column="2" size="small" border>
              <el-descriptions-item label="今日请求">{{ tenantQuotaForm.used_today_requests ?? 0 }}</el-descriptions-item>
              <el-descriptions-item label="本月请求">{{ tenantQuotaForm.used_month_requests ?? 0 }}</el-descriptions-item>
            </el-descriptions>
          </el-form>
        </el-tab-pane>
      </el-tabs>
      <template #footer>
        <div class="drawer-footer">
          <el-button @click="manageVisible = false">取消</el-button>
          <el-button type="primary" :loading="manageSaving" @click="saveManage">保存</el-button>
        </div>
      </template>
    </el-drawer>

    <!-- 查看完整API密钥 -->
    <el-dialog
      v-model="fullKeyVisible"
      title="查看完整API密钥"
      width="600px"
      :close-on-click-modal="false"
      destroy-on-close
    >
      <el-alert
        :type="selectedApiKey?.full_key ? 'warning' : 'info'"
        :closable="false"
        style="margin-bottom: 20px"
      >
        {{
          selectedApiKey?.full_key
            ? '请妥善保管完整密钥，关闭后未必能再次查看。'
            : '完整密钥不可用（创建时未加密落库）。请为用户新建密钥，创建瞬间会展示一次明文。'
        }}
      </el-alert>
      <el-descriptions :column="1" border>
        <el-descriptions-item label="名称">
          {{ selectedApiKey?.name }}
        </el-descriptions-item>
        <el-descriptions-item label="完整密钥">
          <el-input
            v-if="selectedApiKey?.full_key"
            :model-value="selectedApiKey.full_key"
            type="text"
            readonly
            show-password
          >
            <template #append>
              <el-button @click="copyFullKey">复制</el-button>
            </template>
          </el-input>
          <span v-else class="text-muted">不可查看 — {{ selectedApiKey?.masked_key || selectedApiKey?.key_prefix || '—' }}</span>
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
        <el-button v-if="!selectedApiKey?.full_key" type="primary" @click="openCreateKeyDialog">新建密钥</el-button>
        <el-button @click="fullKeyVisible = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Search, Refresh, Plus, Edit, Key, Delete, MoreFilled, View, Setting } from '@element-plus/icons-vue'
import type { FormInstance, FormRules } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import {
  listUsers, createUser, updateUser,
  deleteUser as apiDeleteUser, adminResetPassword,
  listUserAPIKeys, getAdminAPIKey, adminCreateAPIKey
} from '@/api/user'
import type { UserInfo, APIKey, APIKeyDetail } from '@/api/auth'
import { listTenants, getTenantQuota, updateTenantQuota, type TenantQuota } from '@/api/tenant'
import { getPipelines } from '@/api/pipeline'
import { getBackends } from '@/api/backend'

const TOKEN_TIERS = [
  { id: 'unlimited', label: '不限制', daily: 0, monthly: 0 },
  { id: 'light', label: '轻量（日 10万 / 月 300万）', daily: 100_000, monthly: 3_000_000 },
  { id: 'standard', label: '标准（日 50万 / 月 1500万）', daily: 500_000, monthly: 15_000_000 },
  { id: 'pro', label: '高级（日 200万 / 月 6000万）', daily: 2_000_000, monthly: 60_000_000 },
  { id: 'custom', label: '自定义', daily: -1, monthly: -1 }
] as const

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

onMounted(async () => {
  await Promise.all([loadUsers(), loadPipelineOptions()])
})

async function loadUsers() {
  loading.value = true
  try { users.value = await listUsers() }
  catch (e: any) { ElMessage.error(e.message || '获取用户列表失败') }
  finally { loading.value = false }
}

function pipelineLabel(id?: string) {
  if (!id) return '—'
  const hit = pipelineOptions.value.find((p) => p.id === id)
  return hit?.name ? `${hit.name}` : id
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
    if (!detail.full_key && !detail.reveal_available) {
      ElMessage.info('完整密钥不可用，可新建密钥后在创建瞬间复制')
    }
  } catch (e: any) {
    ElMessage.error(e.message || '获取 API 密钥详情失败')
  }
}

async function copyFullKey() {
  const text = selectedApiKey.value?.full_key
  if (!text) return
  try {
    await navigator.clipboard.writeText(text)
    ElMessage.success('已复制到剪贴板')
  } catch {
    ElMessage.error('复制失败，请手动选择复制')
  }
}

const createKeyVisible = ref(false)
const creatingKey = ref(false)
const createKeyForm = reactive({ name: 'default', expires_in: 0 })

function openCreateKeyDialog() {
  createKeyForm.name = 'default'
  createKeyForm.expires_in = 0
  createKeyVisible.value = true
}

async function createKeyForUser() {
  if (!targetUser.value) return
  if (!createKeyForm.name.trim()) {
    ElMessage.warning('请填写密钥名称')
    return
  }
  creatingKey.value = true
  try {
    const created = await adminCreateAPIKey({
      user_id: targetUser.value.id,
      name: createKeyForm.name.trim(),
      expires_in: createKeyForm.expires_in > 0 ? createKeyForm.expires_in : undefined
    })
    createKeyVisible.value = false
    selectedApiKey.value = {
      ...created,
      full_key: created.full_key,
      reveal_available: true
    }
    fullKeyVisible.value = true
    ElMessage.success('密钥已创建，请立即复制完整密钥')
    // 静默刷新列表，避免打断完整密钥展示
    try {
      userApiKeys.value = await listUserAPIKeys(targetUser.value.id)
    } catch {
      /* ignore */
    }
  } catch (e: any) {
    if (e?.message) ElMessage.error(e.message)
  } finally {
    creatingKey.value = false
  }
}

function formatLimit(n?: number) {
  if (n == null || n === 0) return '∞'
  return formatNumber(n)
}

function formatNumber(n?: number) {
  if (n == null) return '0'
  return Number(n).toLocaleString()
}

// ── 资源配置抽屉 ─────────────────────────────────────────────────────────────
const manageVisible = ref(false)
const manageSaving = ref(false)
const manageTab = ref('token')
const manageUser = ref<UserInfo | null>(null)
const manageTenantId = ref('')
const tenantQuotaLoading = ref(false)
const pipelineOptions = ref<{ id: string; name?: string; tenant_id?: string }[]>([])
type SharedBackendOpt = {
  id: string
  name?: string
  enabled?: boolean
  tenant_id?: string
  supported_models?: { requested_model?: string; actual_model?: string }[]
}
const sharedBackendOptions = ref<SharedBackendOpt[]>([])
const manageForm = reactive({
  daily_token_limit: 0,
  monthly_token_limit: 0,
  default_pipeline_id: '' as string,
  allowed_backend_ids: [] as string[],
  allowed_model_ids: [] as string[],
  allowed_pipeline_ids: [] as string[],
  can_add_own_backends: true,
  can_add_own_pipelines: true,
  can_change_default_pipeline: true
})
const tokenTier = ref<string>('unlimited')

const backendFilter = ref('')
const pipelineFilter = ref('')

const sharedPipelineOptions = computed(() =>
  pipelineOptions.value.filter((p) => !p.tenant_id)
)

const sharedModelOptions = computed(() => {
  const selected = new Set(manageForm.allowed_backend_ids)
  const models = new Set<string>()
  for (const b of sharedBackendOptions.value) {
    if (!selected.has(b.id)) continue
    for (const m of modelsOfBackend(b)) models.add(m)
  }
  return Array.from(models).sort()
})

const filteredSharedBackends = computed(() => {
  const q = backendFilter.value.trim().toLowerCase()
  if (!q) return sharedBackendOptions.value
  return sharedBackendOptions.value.filter(
    (b) =>
      b.id.toLowerCase().includes(q) ||
      (b.name || '').toLowerCase().includes(q)
  )
})

const filteredSharedPipelines = computed(() => {
  const q = pipelineFilter.value.trim().toLowerCase()
  const list = sharedPipelineOptions.value
  if (!q) return list
  return list.filter(
    (p) =>
      p.id.toLowerCase().includes(q) ||
      (p.name || '').toLowerCase().includes(q)
  )
})

const defaultPipelineSelectOptions = computed(() => {
  const allowed = new Set(manageForm.allowed_pipeline_ids)
  return pipelineOptions.value.filter((p) => {
    if (!p.tenant_id) return allowed.has(p.id)
    return true
  })
})

function modelsOfBackend(b: SharedBackendOpt): string[] {
  const out: string[] = []
  for (const m of b.supported_models || []) {
    const id = (m.requested_model || m.actual_model || '').trim()
    if (id && !out.includes(id)) out.push(id)
  }
  return out
}

function isBackendAllowed(id: string) {
  return manageForm.allowed_backend_ids.includes(id)
}

function pruneModelsToAllowedBackends() {
  const allowedModels = new Set(sharedModelOptions.value)
  manageForm.allowed_model_ids = manageForm.allowed_model_ids.filter((m) => allowedModels.has(m))
}

function toggleBackend(id: string, checked: boolean) {
  if (checked) {
    if (!manageForm.allowed_backend_ids.includes(id)) {
      manageForm.allowed_backend_ids.push(id)
    }
    const b = sharedBackendOptions.value.find((x) => x.id === id)
    if (b) {
      for (const m of modelsOfBackend(b)) {
        if (!manageForm.allowed_model_ids.includes(m)) {
          manageForm.allowed_model_ids.push(m)
        }
      }
    }
  } else {
    manageForm.allowed_backend_ids = manageForm.allowed_backend_ids.filter((x) => x !== id)
    pruneModelsToAllowedBackends()
  }
}

function selectedModelsOf(b: SharedBackendOpt): string[] {
  const own = new Set(modelsOfBackend(b))
  return manageForm.allowed_model_ids.filter((m) => own.has(m))
}

function setModelsForBackend(b: SharedBackendOpt, vals: Array<string | number | boolean>) {
  const drop = new Set(modelsOfBackend(b))
  const kept = manageForm.allowed_model_ids.filter((m) => !drop.has(m))
  const next = vals.map(String).filter((m) => drop.has(m))
  manageForm.allowed_model_ids = [...kept, ...next]
}

function selectAllModelsForBackend(b: SharedBackendOpt) {
  setModelsForBackend(b, modelsOfBackend(b))
}

function clearModelsForBackend(b: SharedBackendOpt) {
  setModelsForBackend(b, [])
}

function selectAllSharedBackends() {
  for (const b of sharedBackendOptions.value) {
    toggleBackend(b.id, true)
  }
}

function clearSharedBackends() {
  manageForm.allowed_backend_ids = []
  manageForm.allowed_model_ids = []
}

function selectAllSharedPipelines() {
  manageForm.allowed_pipeline_ids = sharedPipelineOptions.value.map((p) => p.id)
}

function clearSharedPipelines() {
  manageForm.allowed_pipeline_ids = []
}

function matchTokenTier(daily: number, monthly: number): string {
  const hit = TOKEN_TIERS.find((t) => t.id !== 'custom' && t.daily === daily && t.monthly === monthly)
  return hit?.id ?? 'custom'
}

function onTokenTierChange(id: string) {
  const tier = TOKEN_TIERS.find((t) => t.id === id)
  if (!tier || tier.id === 'custom') return
  manageForm.daily_token_limit = tier.daily
  manageForm.monthly_token_limit = tier.monthly
}

function onTokenManualChange() {
  tokenTier.value = matchTokenTier(manageForm.daily_token_limit, manageForm.monthly_token_limit)
}
const tenantQuotaForm = reactive({
  daily_token_limit: 0,
  monthly_token_limit: 0,
  daily_request_limit: 0,
  monthly_request_limit: 0,
  max_backends: 0,
  max_api_keys: 0,
  used_today_requests: 0,
  used_month_requests: 0
})

async function loadPipelineOptions() {
  try {
    const res: any = await getPipelines()
    const list = Array.isArray(res) ? res : res?.pipelines ?? res?.data ?? []
    pipelineOptions.value = (list as any[])
      .map((p) => ({
        id: String(p.id || p.pipeline_id || ''),
        name: p.name,
        tenant_id: p.tenant_id || ''
      }))
      .filter((p) => p.id)
  } catch {
    pipelineOptions.value = []
  }
}

async function loadSharedBackendOptions() {
  try {
    const res: any = await getBackends()
    const list = Array.isArray(res) ? res : res?.data ?? []
    sharedBackendOptions.value = (list as any[])
      .filter((b) => b && b.enabled !== false && !b.tenant_id)
      .map((b) => ({
        id: String(b.id),
        name: b.name,
        enabled: b.enabled !== false,
        tenant_id: b.tenant_id || '',
        supported_models: Array.isArray(b.supported_models) ? b.supported_models : []
      }))
      .filter((b) => b.id)
  } catch {
    sharedBackendOptions.value = []
  }
}

async function openManageDrawer(u: UserInfo) {
  manageUser.value = u
  manageTab.value = 'token'
  manageForm.daily_token_limit = Number(u.daily_token_limit || 0)
  manageForm.monthly_token_limit = Number(u.monthly_token_limit || 0)
  manageForm.default_pipeline_id = u.default_pipeline_id || ''
  manageForm.allowed_backend_ids = [...(u.allowed_backend_ids || [])]
  manageForm.allowed_model_ids = [...(u.allowed_model_ids || [])]
  manageForm.allowed_pipeline_ids = [...(u.allowed_pipeline_ids || [])]
  manageForm.can_add_own_backends = u.can_add_own_backends !== false
  manageForm.can_add_own_pipelines = u.can_add_own_pipelines !== false
  manageForm.can_change_default_pipeline = u.can_change_default_pipeline !== false
  tokenTier.value = matchTokenTier(manageForm.daily_token_limit, manageForm.monthly_token_limit)
  manageTenantId.value = ''
  Object.assign(tenantQuotaForm, {
    daily_token_limit: 0,
    monthly_token_limit: 0,
    daily_request_limit: 0,
    monthly_request_limit: 0,
    max_backends: 0,
    max_api_keys: 0,
    used_today_requests: 0,
    used_month_requests: 0
  })
  backendFilter.value = ''
  pipelineFilter.value = ''
  manageVisible.value = true
  await Promise.all([
    loadPipelineOptions(),
    loadSharedBackendOptions(),
    loadTenantQuotaForUser(u.id)
  ])
  pruneModelsToAllowedBackends()
}

async function loadTenantQuotaForUser(userId: number) {
  tenantQuotaLoading.value = true
  try {
    const tenants = await listTenants()
    const list = Array.isArray(tenants) ? tenants : (tenants as any)?.data ?? []
    const tenant = (list as any[]).find((t) => Number(t.user_id) === userId)
    if (!tenant?.id) {
      manageTenantId.value = ''
      return
    }
    manageTenantId.value = tenant.id
    const quota = await getTenantQuota(tenant.id)
    const q = (quota && typeof quota === 'object' ? quota : {}) as TenantQuota
    // 保留租户侧 Token 字段，避免 PUT 全量覆盖时被清零（用户 Token 限额以 users 表为准）
    tenantQuotaForm.daily_token_limit = Number(q.daily_token_limit || 0)
    tenantQuotaForm.monthly_token_limit = Number(q.monthly_token_limit || 0)
    tenantQuotaForm.daily_request_limit = Number(q.daily_request_limit || 0)
    tenantQuotaForm.monthly_request_limit = Number(q.monthly_request_limit || 0)
    tenantQuotaForm.max_backends = Number(q.max_backends || 0)
    tenantQuotaForm.max_api_keys = Number(q.max_api_keys || 0)
    tenantQuotaForm.used_today_requests = Number(q.used_today_requests || 0)
    tenantQuotaForm.used_month_requests = Number(q.used_month_requests || 0)
  } catch (e: any) {
    ElMessage.warning(e?.message || '加载租户配额失败')
  } finally {
    tenantQuotaLoading.value = false
  }
}

async function saveManage() {
  if (!manageUser.value) return
  manageSaving.value = true
  try {
    await updateUser(manageUser.value.id, {
      daily_token_limit: manageForm.daily_token_limit,
      monthly_token_limit: manageForm.monthly_token_limit,
      default_pipeline_id: manageForm.default_pipeline_id || '',
      allowed_backend_ids: [...manageForm.allowed_backend_ids],
      allowed_model_ids: [...manageForm.allowed_model_ids],
      allowed_pipeline_ids: [...manageForm.allowed_pipeline_ids],
      can_add_own_backends: manageForm.can_add_own_backends,
      can_add_own_pipelines: manageForm.can_add_own_pipelines,
      can_change_default_pipeline: manageForm.can_change_default_pipeline
    })
    if (manageTenantId.value) {
      await updateTenantQuota(manageTenantId.value, {
        daily_token_limit: tenantQuotaForm.daily_token_limit,
        monthly_token_limit: tenantQuotaForm.monthly_token_limit,
        daily_request_limit: tenantQuotaForm.daily_request_limit,
        monthly_request_limit: tenantQuotaForm.monthly_request_limit,
        max_backends: tenantQuotaForm.max_backends,
        max_api_keys: tenantQuotaForm.max_api_keys
      })
    }
    ElMessage.success('资源配置已保存')
    manageVisible.value = false
    await loadUsers()
  } catch (e: any) {
    if (e?.message) ElMessage.error(e.message)
  } finally {
    manageSaving.value = false
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

.quota-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
  font-size: 13px;
}

.form-tip {
  margin-top: 4px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  line-height: 1.4;
}

.manage-form {
  padding-top: 8px;
}

.drawer-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.masked-key {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
}

.access-panel {
  display: flex;
  flex-direction: column;
  gap: 20px;
  padding-top: 4px;
}

.access-summary {
  margin: 0;
  font-size: 13px;
  color: var(--el-text-color-regular);
}

.access-summary strong {
  color: var(--el-color-primary);
  font-weight: 600;
}

.access-hint {
  margin: -12px 0 0;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  line-height: 1.5;
}

.access-section {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.access-section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.access-section-title {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.access-section-actions {
  display: flex;
  gap: 4px;
}

.access-filter {
  width: 100%;
}

.access-empty {
  padding: 20px;
  text-align: center;
  font-size: 13px;
  color: var(--el-text-color-secondary);
  background: var(--el-fill-color-lighter);
  border-radius: 8px;
}

.perm-list {
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;
  overflow: hidden;
}

.perm-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 12px 14px;
  border-bottom: 1px solid var(--el-border-color-lighter);
}

.perm-row:last-child {
  border-bottom: none;
}

.perm-name {
  font-size: 14px;
  font-weight: 500;
  color: var(--el-text-color-primary);
}

.perm-desc {
  margin-top: 2px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  line-height: 1.4;
}

.backend-pick-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-height: 360px;
  overflow: auto;
  padding-right: 2px;
}

.backend-pick {
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;
  padding: 10px 12px;
  background: var(--el-bg-color);
  transition: border-color 0.15s ease;
}

.backend-pick.active {
  border-color: var(--el-color-primary-light-5);
  background: var(--el-color-primary-light-9);
}

.backend-pick-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.backend-pick-name {
  font-weight: 500;
}

.option-id {
  margin-left: 8px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.model-pick-list {
  margin-top: 10px;
  padding-top: 10px;
  border-top: 1px dashed var(--el-border-color-lighter);
}

.model-pick-toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}

.model-pick-label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-right: auto;
}

.model-checks {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 12px;
}

.pipeline-checks {
  display: flex;
  flex-direction: column;
  gap: 2px;
  max-height: 260px;
  overflow: auto;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;
  padding: 6px 8px;
}

.pipeline-check-row {
  display: block;
  padding: 4px 6px;
  border-radius: 6px;
}

.pipeline-check-row:hover {
  background: var(--el-fill-color-light);
}

.pipeline-check-name {
  font-weight: 500;
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

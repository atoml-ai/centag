<template>
  <div class="clash-page">
    <div class="page-header">
      <div class="header-info">
        <h1 class="page-title">Clash 订阅管理</h1>
        <p class="page-description">
          为每条规则生成独立订阅链接，填入 Clash / Mihomo 客户端即可自动拉取代理规则
        </p>
      </div>
      <el-button type="primary" @click="openCreate">
        <el-icon><Plus /></el-icon>新建规则
      </el-button>
    </div>

    <!-- 说明卡片 -->
    <el-alert type="info" :closable="false" show-icon style="margin-bottom:20px;border-radius:8px">
      <template #title>
        订阅链接无需鉴权，任何人持有链接均可下载对应规则。如需撤销，请重新生成令牌。
        未自定义内容的规则将自动下发系统默认 <code>rule.yaml</code>。
      </template>
    </el-alert>

    <!-- 规则列表 -->
    <div v-loading="loading" class="rules-grid">
      <!-- 空状态 -->
      <el-empty
        v-if="!loading && rules.length === 0"
        description="暂无订阅规则，点击「新建规则」创建第一条"
        :image-size="120"
      />

      <el-card
        v-for="rule in rules"
        :key="rule.id"
        shadow="never"
        class="rule-card"
      >
        <!-- 卡片头 -->
        <template #header>
          <div class="rule-card-header">
            <div class="rule-title-wrap">
              <el-icon class="rule-icon"><Document /></el-icon>
              <span class="rule-name">{{ rule.name }}</span>
              <el-tag
                :type="rule.has_custom_rule ? 'success' : 'info'"
                size="small"
                effect="plain"
              >
                {{ rule.has_custom_rule ? '自定义规则' : '系统默认' }}
              </el-tag>
            </div>
            <el-dropdown trigger="click" @command="(cmd: string) => handleCmd(cmd, rule)">
              <el-button type="primary" link>
                <el-icon :size="18"><MoreFilled /></el-icon>
              </el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="edit-name">
                    <el-icon><EditPen /></el-icon>重命名
                  </el-dropdown-item>
                  <el-dropdown-item command="edit-content">
                    <el-icon><Edit /></el-icon>编辑规则内容
                  </el-dropdown-item>
                  <el-dropdown-item v-if="rule.has_custom_rule" command="reset">
                    <el-icon><RefreshLeft /></el-icon>恢复系统默认
                  </el-dropdown-item>
                  <el-dropdown-item divided command="regen-token">
                    <el-icon><Key /></el-icon>重新生成令牌
                  </el-dropdown-item>
                  <el-dropdown-item divided command="delete" style="color:var(--el-color-danger)">
                    <el-icon><Delete /></el-icon>删除
                  </el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </div>
        </template>

        <!-- 订阅 URL -->
        <div class="url-section">
          <div class="url-label">
            <el-icon><Link /></el-icon>
            <span>订阅链接</span>
          </div>
          <div class="url-row">
            <code class="url-text">{{ rule.subscribe_url }}</code>
            <el-tooltip content="复制链接" placement="top">
              <el-button type="primary" link @click="copyUrl(rule.subscribe_url)">
                <el-icon><CopyDocument /></el-icon>
              </el-button>
            </el-tooltip>
          </div>
        </div>

        <!-- 令牌 -->
        <div class="token-section">
          <span class="meta-label">令牌</span>
          <code class="token-text">{{ rule.subscribe_token.slice(0, 16) }}…</code>
          <span class="meta-label" style="margin-left:16px">更新时间</span>
          <span class="meta-val">{{ formatTime(rule.updated_at) }}</span>
        </div>

        <!-- 快速操作 -->
        <div class="quick-actions">
          <el-button size="small" @click="openEditContent(rule)">
            <el-icon><Edit /></el-icon>编辑内容
          </el-button>
          <el-button size="small" @click="copyUrl(rule.subscribe_url)">
            <el-icon><CopyDocument /></el-icon>复制链接
          </el-button>
          <el-button size="small" type="danger" plain @click="confirmDelete(rule)">
            <el-icon><Delete /></el-icon>删除
          </el-button>
        </div>
      </el-card>
    </div>

    <!-- ── 新建对话框 ── -->
    <el-dialog
      v-model="showCreate"
      title="新建订阅规则"
      width="480px"
      :close-on-click-modal="false"
      destroy-on-close
    >
      <el-form :model="createForm" :rules="createRules" ref="createFormRef" label-width="90px">
        <el-form-item label="规则名称" prop="name">
          <el-input v-model="createForm.name" placeholder="给这条规则起个名字，方便识别" />
        </el-form-item>
        <el-form-item label="初始内容">
          <el-radio-group v-model="createForm.useDefault">
            <el-radio :value="true">使用系统默认 rule.yaml</el-radio>
            <el-radio :value="false">稍后手动编辑</el-radio>
          </el-radio-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreate = false">取消</el-button>
        <el-button type="primary" :loading="creating" @click="doCreate">创建</el-button>
      </template>
    </el-dialog>

    <!-- ── 重命名对话框 ── -->
    <el-dialog
      v-model="showRename"
      title="重命名规则"
      width="400px"
      :close-on-click-modal="false"
      destroy-on-close
    >
      <el-form :model="renameForm" :rules="renameRules" ref="renameFormRef" label-width="90px">
        <el-form-item label="规则名称" prop="name">
          <el-input v-model="renameForm.name" placeholder="规则名称" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showRename = false">取消</el-button>
        <el-button type="primary" :loading="renaming" @click="doRename">保存</el-button>
      </template>
    </el-dialog>

    <!-- ── 编辑内容对话框 ── -->
    <el-dialog
      v-model="showEditContent"
      :title="`编辑规则内容 — ${editTarget?.name}`"
      width="82%"
      :close-on-click-modal="false"
      destroy-on-close
      class="content-dialog"
      :style="{ '--dialog-body-height': dialogBodyHeight }"
    >
      <div class="editor-hint">
        <el-icon><InfoFilled /></el-icon>
        <span>编辑 Clash 兼容的 YAML 规则。当前显示的是系统默认规则，修改后保存即变为您的自定义规则。</span>
      </div>
      <div v-loading="loadingDefaultRule" element-loading-text="正在加载默认规则..." class="editor-wrap">
        <el-input
          v-model="editContent"
          type="textarea"
          placeholder="正在加载默认规则内容..."
          class="yaml-editor"
          spellcheck="false"
          :disabled="loadingDefaultRule"
        />
      </div>
      <template #footer>
        <div class="editor-footer">
          <el-button
            v-if="editTarget?.has_custom_rule"
            type="warning"
            plain
            :loading="resetting"
            @click="doReset"
          >
            <el-icon><RefreshLeft /></el-icon>恢复系统默认
          </el-button>
          <span style="flex:1" />
          <el-button @click="showEditContent = false">取消</el-button>
          <el-button type="primary" :loading="savingContent" @click="doSaveContent">保存</el-button>
        </div>
      </template>
    </el-dialog>

    <!-- ── 令牌重新生成确认 ── -->
    <el-dialog
      v-model="showNewToken"
      title="令牌已重新生成"
      width="520px"
      :close-on-click-modal="false"
    >
      <el-alert type="warning" :closable="false" show-icon style="margin-bottom:16px;border-radius:6px">
        <template #title>旧订阅链接已失效！请在 Clash 中更新为以下新链接。</template>
      </el-alert>
      <div class="new-token-box">
        <code class="new-url">{{ newTokenUrl }}</code>
        <el-button type="primary" plain size="small" @click="copyUrl(newTokenUrl)">
          <el-icon><CopyDocument /></el-icon>复制
        </el-button>
      </div>
      <template #footer>
        <el-button type="primary" @click="showNewToken = false">已更新，关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { FormInstance, FormRules } from 'element-plus'
import {
  Plus, Document, MoreFilled, Edit, EditPen, Delete, CopyDocument,
  Link, Key, RefreshLeft, InfoFilled
} from '@element-plus/icons-vue'
import {
  listClashRules, createClashRule, updateClashRule, deleteClashRule,
  resetClashRuleContent, regenerateClashToken, getDefaultClashRule
} from '@/api/clash'
import type { ClashRule } from '@/api/clash'

// ── 编辑器高度：视口高度 - 标题栏(54px) - 提示行(36px) - 底部按钮(72px) - 对话框头尾内边距(48px) - 安全边距(32px)
const dialogBodyHeight = computed(() => `${Math.max(300, window.innerHeight * 0.9 - 242)}px`)

// ── 列表 ─────────────────────────────────────────────────────────────────────

const rules = ref<ClashRule[]>([])
const loading = ref(false)

async function load() {
  loading.value = true
  try {
    rules.value = await listClashRules()
  } catch (e: any) {
    ElMessage.error(e.message || '获取规则列表失败')
  } finally {
    loading.value = false
  }
}

onMounted(load)

// ── 工具函数 ──────────────────────────────────────────────────────────────────

function formatTime(iso: string) {
  if (!iso) return '—'
  try {
    return new Date(iso).toLocaleString('zh-CN', { hour12: false })
  } catch {
    return iso
  }
}

async function copyUrl(url: string) {
  try {
    await navigator.clipboard.writeText(url)
    ElMessage.success('链接已复制')
  } catch {
    ElMessage.warning('复制失败，请手动复制')
  }
}

// ── 下拉命令分发 ──────────────────────────────────────────────────────────────

function handleCmd(cmd: string, rule: ClashRule) {
  switch (cmd) {
    case 'edit-name':    openRename(rule); break
    case 'edit-content': openEditContent(rule); break
    case 'reset':        confirmReset(rule); break
    case 'regen-token':  confirmRegenToken(rule); break
    case 'delete':       confirmDelete(rule); break
  }
}

// ── 新建 ─────────────────────────────────────────────────────────────────────

const showCreate = ref(false)
const createFormRef = ref<FormInstance>()
const createForm = reactive({ name: '', useDefault: true })
const createRules: FormRules = {
  name: [{ required: true, message: '请输入规则名称', trigger: 'blur' }]
}
const creating = ref(false)

function openCreate() {
  createForm.name = ''
  createForm.useDefault = true
  showCreate.value = true
}

async function doCreate() {
  if (!createFormRef.value) return
  try {
    await createFormRef.value.validate()
    creating.value = true
    await createClashRule({ name: createForm.name, rule_content: '' })
    showCreate.value = false
    ElMessage.success('规则已创建')
    load()
  } catch (e: any) {
    if (e?.message) ElMessage.error(e.message)
  } finally {
    creating.value = false
  }
}

// ── 重命名 ────────────────────────────────────────────────────────────────────

const showRename = ref(false)
const renameFormRef = ref<FormInstance>()
const renameForm = reactive({ name: '' })
const renameRules: FormRules = {
  name: [{ required: true, message: '请输入规则名称', trigger: 'blur' }]
}
const renaming = ref(false)
let renameTarget: ClashRule | null = null

function openRename(rule: ClashRule) {
  renameTarget = rule
  renameForm.name = rule.name
  showRename.value = true
}

async function doRename() {
  if (!renameFormRef.value || !renameTarget) return
  try {
    await renameFormRef.value.validate()
    renaming.value = true
    await updateClashRule(renameTarget.id, { name: renameForm.name })
    showRename.value = false
    ElMessage.success('已重命名')
    load()
  } catch (e: any) {
    if (e?.message) ElMessage.error(e.message)
  } finally {
    renaming.value = false
  }
}

// ── 编辑内容 ──────────────────────────────────────────────────────────────────

const showEditContent = ref(false)
const editTarget = ref<ClashRule | null>(null)
const editContent = ref('')
const savingContent = ref(false)
const loadingDefaultRule = ref(false)

async function openEditContent(rule: ClashRule) {
  editTarget.value = rule
  showEditContent.value = true

  if (rule.rule_content) {
    // 已有自定义内容，直接显示
    editContent.value = rule.rule_content
  } else {
    // 无自定义内容，从后端拉取系统默认规则作为初始内容
    editContent.value = ''
    loadingDefaultRule.value = true
    try {
      const res = await getDefaultClashRule()
      editContent.value = res.content
    } catch (e: any) {
      ElMessage.warning('默认规则加载失败，请手动粘贴内容')
    } finally {
      loadingDefaultRule.value = false
    }
  }
}

async function doSaveContent() {
  if (!editTarget.value) return
  savingContent.value = true
  try {
    await updateClashRule(editTarget.value.id, { rule_content: editContent.value })
    showEditContent.value = false
    ElMessage.success('规则内容已保存')
    load()
  } catch (e: any) {
    ElMessage.error(e.message || '保存失败')
  } finally {
    savingContent.value = false
  }
}

// ── 恢复默认 ──────────────────────────────────────────────────────────────────

const resetting = ref(false)

async function confirmReset(rule: ClashRule) {
  try {
    await ElMessageBox.confirm(
      `确定将「${rule.name}」的规则内容恢复为系统默认 rule.yaml？自定义内容将被清除。`,
      '恢复确认',
      { confirmButtonText: '恢复', cancelButtonText: '取消', type: 'warning' }
    )
    await doResetById(rule.id)
  } catch { /* cancel */ }
}

async function doReset() {
  if (!editTarget.value) return
  try {
    await ElMessageBox.confirm(
      '确定清除自定义内容，恢复为系统默认 rule.yaml？',
      '恢复确认',
      { confirmButtonText: '恢复', cancelButtonText: '取消', type: 'warning' }
    )
    await doResetById(editTarget.value.id)
    showEditContent.value = false
  } catch { /* cancel */ }
}

async function doResetById(id: number) {
  resetting.value = true
  try {
    await resetClashRuleContent(id)
    ElMessage.success('已恢复系统默认')
    load()
  } catch (e: any) {
    ElMessage.error(e.message || '操作失败')
  } finally {
    resetting.value = false
  }
}

// ── 重新生成令牌 ──────────────────────────────────────────────────────────────

const showNewToken = ref(false)
const newTokenUrl = ref('')

async function confirmRegenToken(rule: ClashRule) {
  try {
    await ElMessageBox.confirm(
      `重新生成令牌后，「${rule.name}」的旧订阅链接将立即失效，需要在 Clash 中更新链接。确定继续？`,
      '重新生成令牌',
      { confirmButtonText: '生成', cancelButtonText: '取消', type: 'warning' }
    )
    const res = await regenerateClashToken(rule.id)
    newTokenUrl.value = res.subscribe_url
    showNewToken.value = true
    load()
  } catch (e: any) {
    if (e !== 'cancel' && e?.message) ElMessage.error(e.message)
  }
}

// ── 删除 ─────────────────────────────────────────────────────────────────────

async function confirmDelete(rule: ClashRule) {
  try {
    await ElMessageBox.confirm(
      `确定删除规则「${rule.name}」？对应的订阅链接将同时失效。`,
      '删除确认',
      { confirmButtonText: '删除', cancelButtonText: '取消', type: 'danger' }
    )
    await deleteClashRule(rule.id)
    ElMessage.success('规则已删除')
    load()
  } catch (e: any) {
    if (e !== 'cancel' && e?.message) ElMessage.error(e.message)
  }
}
</script>

<style scoped>
.clash-page {
  max-width: 1200px;
  margin: 0 auto;
}

.page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  margin-bottom: 20px;
  gap: 16px;
}

.header-info { flex: 1; }

.page-title {
  font-size: 1.5rem;
  font-weight: 700;
  color: var(--el-text-color-primary);
  margin: 0 0 6px;
}

.page-description {
  font-size: 0.875rem;
  color: var(--el-text-color-secondary);
  margin: 0;
}

/* Grid */
.rules-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(380px, 1fr));
  gap: 20px;
  min-height: 120px;
}

/* Rule card */
.rule-card {
  border-radius: 10px;
  transition: box-shadow 0.2s;
}

.rule-card:hover {
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.1);
}

.rule-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.rule-title-wrap {
  display: flex;
  align-items: center;
  gap: 8px;
  overflow: hidden;
}

.rule-icon {
  color: #667eea;
  font-size: 18px;
  flex-shrink: 0;
}

.rule-name {
  font-size: 0.9375rem;
  font-weight: 600;
  color: var(--el-text-color-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* Subscribe URL */
.url-section {
  margin-bottom: 14px;
}

.url-label {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 0.75rem;
  color: var(--el-text-color-secondary);
  margin-bottom: 6px;
}

.url-row {
  display: flex;
  align-items: center;
  gap: 6px;
  background: var(--el-fill-color-lighter);
  border: 1px solid var(--el-border-color-light);
  border-radius: 6px;
  padding: 8px 10px;
}

.url-text {
  flex: 1;
  font-family: 'Menlo', 'Monaco', 'Consolas', monospace;
  font-size: 0.75rem;
  color: var(--el-color-primary);
  word-break: break-all;
  line-height: 1.5;
}

/* Token + meta */
.token-section {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
  margin-bottom: 14px;
}

.meta-label {
  font-size: 0.75rem;
  color: var(--el-text-color-secondary);
}

.token-text {
  font-family: 'Menlo', 'Monaco', 'Consolas', monospace;
  font-size: 0.75rem;
  background: var(--el-fill-color-light);
  padding: 2px 6px;
  border-radius: 4px;
  color: var(--el-text-color-regular);
}

.meta-val {
  font-size: 0.75rem;
  color: var(--el-text-color-regular);
}

/* Quick actions */
.quick-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  border-top: 1px solid var(--el-border-color-lighter);
  padding-top: 12px;
}

/* YAML editor */
.editor-hint {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 0.8125rem;
  color: var(--el-text-color-secondary);
  margin-bottom: 10px;
}

.editor-wrap {
  width: 100%;
}

:deep(.yaml-editor .el-textarea__inner) {
  font-family: 'Menlo', 'Monaco', 'Consolas', monospace;
  font-size: 0.8125rem;
  line-height: 1.6;
  resize: none;
  height: var(--dialog-body-height, 60vh) !important;
  overflow-y: auto;
}

.editor-footer {
  display: flex;
  align-items: center;
  gap: 8px;
}

/* New token display */
.new-token-box {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  background: var(--el-fill-color-lighter);
  border: 1px solid var(--el-border-color-light);
  padding: 14px 16px;
  border-radius: 8px;
}

.new-url {
  flex: 1;
  font-family: 'Menlo', 'Monaco', 'Consolas', monospace;
  font-size: 0.8125rem;
  word-break: break-all;
  color: var(--el-color-success-dark-2);
  line-height: 1.6;
}

@media (max-width: 768px) {
  .rules-grid {
    grid-template-columns: 1fr;
  }

  .page-header {
    flex-direction: column;
    align-items: stretch;
  }
}
</style>

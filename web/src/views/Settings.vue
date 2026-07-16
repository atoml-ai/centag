<template>
  <div class="settings-page">
    <div class="page-header">
      <div>
        <h1 class="page-title">设置</h1>
        <p class="page-description">管理密码与服务基本信息（Minimal 精简模式）</p>
      </div>
    </div>

    <el-card class="settings-card">
      <template #header>
        <span>服务信息</span>
      </template>
      <div class="info-rows">
        <div class="info-row">
          <span class="label">版本</span>
          <span class="mono">{{ status.version || '--' }}</span>
        </div>
        <div class="info-row">
          <span class="label">发行版</span>
          <el-tag size="small" type="info">{{ status.edition || 'minimal' }}</el-tag>
        </div>
        <div class="info-row">
          <span class="label">运行时长</span>
          <span>{{ status.uptime || '--' }}</span>
        </div>
        <div class="info-row">
          <span class="label">API 地址</span>
          <code>{{ apiBase }}</code>
        </div>
      </div>
    </el-card>

    <el-card class="settings-card">
      <template #header>
        <span>修改管理密码</span>
      </template>
      <el-form label-width="100px" style="max-width: 420px" @submit.prevent="changePassword">
        <el-form-item label="原密码">
          <el-input v-model="pwd.oldPassword" type="password" show-password autocomplete="current-password" />
        </el-form-item>
        <el-form-item label="新密码">
          <el-input v-model="pwd.newPassword" type="password" show-password autocomplete="new-password" />
        </el-form-item>
        <el-form-item label="确认密码">
          <el-input v-model="pwd.confirm" type="password" show-password autocomplete="new-password" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="saving" @click="changePassword">保存</el-button>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, onMounted, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { getStatus } from '@/api'
import api from '@/api'
import { resolveApiBaseUrl } from '@/utils/apiBaseUrl'

const status = ref<any>({})
const saving = ref(false)
const pwd = reactive({ oldPassword: '', newPassword: '', confirm: '' })

const apiBase = computed(() => resolveApiBaseUrl(status.value))

onMounted(async () => {
  try {
    status.value = (await getStatus()) || {}
  } catch {
    status.value = {}
  }
})

async function changePassword() {
  if (!pwd.oldPassword || !pwd.newPassword) {
    ElMessage.warning('请填写完整')
    return
  }
  if (pwd.newPassword.length < 6) {
    ElMessage.warning('新密码至少 6 位')
    return
  }
  if (pwd.newPassword !== pwd.confirm) {
    ElMessage.warning('两次输入的新密码不一致')
    return
  }
  saving.value = true
  try {
    await api.post('/api/v1/settings/password', {
      old_password: pwd.oldPassword,
      new_password: pwd.newPassword
    })
    ElMessage.success('密码已更新')
    pwd.oldPassword = ''
    pwd.newPassword = ''
    pwd.confirm = ''
  } catch (e: any) {
    ElMessage.error(e?.message || '修改失败')
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.settings-page {
  max-width: 800px;
  margin: 0 auto;
  padding: 24px;
}
.page-header {
  margin-bottom: 20px;
}
.page-title {
  margin: 0 0 6px;
  font-size: 1.5rem;
}
.page-description {
  margin: 0;
  color: var(--el-text-color-secondary);
  font-size: 0.9rem;
}
.settings-card {
  margin-bottom: 16px;
}
.info-rows {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.info-row {
  display: flex;
  gap: 16px;
  align-items: center;
}
.info-row .label {
  width: 88px;
  color: var(--el-text-color-secondary);
}
.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}
</style>

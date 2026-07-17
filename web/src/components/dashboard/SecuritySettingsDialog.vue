<template>
  <el-dialog
    v-model="visible"
    title="安全设置"
    width="640px"
    destroy-on-close
    class="security-settings-dialog"
    @closed="reset"
  >
    <el-tabs v-model="activeTab">
      <el-tab-pane label="登录密码" name="password">
        <el-form label-width="88px" class="pwd-form" @submit.prevent="submitPassword">
          <el-form-item label="原密码">
            <el-input
              v-model="form.oldPassword"
              type="password"
              show-password
              autocomplete="current-password"
            />
          </el-form-item>
          <el-form-item label="新密码">
            <el-input
              v-model="form.newPassword"
              type="password"
              show-password
              autocomplete="new-password"
            />
          </el-form-item>
          <el-form-item label="确认密码">
            <el-input
              v-model="form.confirm"
              type="password"
              show-password
              autocomplete="new-password"
            />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :loading="saving" @click="submitPassword">更新密码</el-button>
          </el-form-item>
        </el-form>
      </el-tab-pane>

      <el-tab-pane label="API 访问密钥" name="api-keys">
        <MinimalApiKeysPanel />
      </el-tab-pane>
    </el-tabs>

    <template #footer>
      <el-button @click="visible = false">关闭</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import api from '@/api'
import MinimalApiKeysPanel from '@/components/dashboard/MinimalApiKeysPanel.vue'

const visible = defineModel<boolean>({ default: false })

const activeTab = ref('password')
const saving = ref(false)
const form = reactive({ oldPassword: '', newPassword: '', confirm: '' })

function reset() {
  form.oldPassword = ''
  form.newPassword = ''
  form.confirm = ''
  saving.value = false
  activeTab.value = 'password'
}

watch(visible, (v) => {
  if (!v) reset()
})

async function submitPassword() {
  if (!form.oldPassword || !form.newPassword) {
    ElMessage.warning('请填写完整')
    return
  }
  if (form.newPassword.length < 6) {
    ElMessage.warning('新密码至少 6 位')
    return
  }
  if (form.newPassword !== form.confirm) {
    ElMessage.warning('两次输入的新密码不一致')
    return
  }
  saving.value = true
  try {
    await api.post('/api/v1/settings/password', {
      old_password: form.oldPassword,
      new_password: form.newPassword
    })
    ElMessage.success('密码已更新')
    form.oldPassword = ''
    form.newPassword = ''
    form.confirm = ''
  } catch (e: any) {
    ElMessage.error(e?.message || '修改失败')
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.pwd-form {
  padding-top: 8px;
  max-width: 420px;
}
</style>

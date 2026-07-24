<template>
  <el-dialog
    v-model="visible"
    :title="t('securitySettingsDialog.dialogTitle')"
    width="640px"
    destroy-on-close
    class="security-settings-dialog"
    @closed="reset"
  >
    <el-tabs v-model="activeTab">
      <el-tab-pane :label="t('securitySettingsDialog.tabPassword')" name="password">
        <el-form label-width="88px" class="pwd-form" @submit.prevent="submitPassword">
          <el-form-item :label="t('securitySettingsDialog.oldPassword')">
            <el-input
              v-model="form.oldPassword"
              type="password"
              show-password
              autocomplete="current-password"
            />
          </el-form-item>
          <el-form-item :label="t('securitySettingsDialog.newPassword')">
            <el-input
              v-model="form.newPassword"
              type="password"
              show-password
              autocomplete="new-password"
            />
          </el-form-item>
          <el-form-item :label="t('securitySettingsDialog.confirmPassword')">
            <el-input
              v-model="form.confirm"
              type="password"
              show-password
              autocomplete="new-password"
            />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :loading="saving" @click="submitPassword">{{ t('securitySettingsDialog.updatePassword') }}</el-button>
          </el-form-item>
        </el-form>
      </el-tab-pane>

      <el-tab-pane :label="t('securitySettingsDialog.tabApiKeys')" name="api-keys">
        <MinimalApiKeysPanel />
      </el-tab-pane>
    </el-tabs>

    <template #footer>
      <el-button @click="visible = false">{{ t('securitySettingsDialog.close') }}</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import api from '@/api'
import MinimalApiKeysPanel from '@/components/dashboard/MinimalApiKeysPanel.vue'

const { t } = useI18n()

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
    ElMessage.warning(t('securitySettingsDialog.message.fillComplete'))
    return
  }
  if (form.newPassword.length < 6) {
    ElMessage.warning(t('securitySettingsDialog.message.passwordTooShort'))
    return
  }
  if (form.newPassword !== form.confirm) {
    ElMessage.warning(t('securitySettingsDialog.message.passwordMismatch'))
    return
  }
  saving.value = true
  try {
    await api.post('/api/v1/settings/password', {
      old_password: form.oldPassword,
      new_password: form.newPassword
    })
    ElMessage.success(t('securitySettingsDialog.message.passwordUpdated'))
    form.oldPassword = ''
    form.newPassword = ''
    form.confirm = ''
  } catch (e: any) {
    ElMessage.error(e?.message || t('securitySettingsDialog.message.updateFailed'))
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

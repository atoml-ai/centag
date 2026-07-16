<template>
  <el-dialog
    v-model="visible"
    title="修改管理密码"
    width="420px"
    destroy-on-close
    @closed="reset"
  >
    <el-form label-width="88px" @submit.prevent="submit">
      <el-form-item label="原密码">
        <el-input v-model="form.oldPassword" type="password" show-password autocomplete="current-password" />
      </el-form-item>
      <el-form-item label="新密码">
        <el-input v-model="form.newPassword" type="password" show-password autocomplete="new-password" />
      </el-form-item>
      <el-form-item label="确认密码">
        <el-input v-model="form.confirm" type="password" show-password autocomplete="new-password" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" :loading="saving" @click="submit">保存</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import api from '@/api'

const visible = defineModel<boolean>({ default: false })

const saving = ref(false)
const form = reactive({ oldPassword: '', newPassword: '', confirm: '' })

function reset() {
  form.oldPassword = ''
  form.newPassword = ''
  form.confirm = ''
  saving.value = false
}

watch(visible, (v) => {
  if (!v) reset()
})

async function submit() {
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
    visible.value = false
  } catch (e: any) {
    ElMessage.error(e?.message || '修改失败')
  } finally {
    saving.value = false
  }
}
</script>

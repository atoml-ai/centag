<template>
  <div class="login-page">
    <div class="login-card">
      <div class="login-header">
        <div class="logo-row">
          <CentagMark :size="36" color="#667eea" />
          <h1 class="title">Centag</h1>
        </div>
        <p class="subtitle" :class="{ 'subtitle--tip': isMinimalEdition() && !isSetup }">{{ subtitle }}</p>
      </div>

      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        class="login-form"
        @submit.prevent="handleSubmit"
      >
        <el-form-item v-if="!isSetup && !hideUsername" prop="username">
          <el-input
            v-model="form.username"
            placeholder="用户名"
            size="large"
            :prefix-icon="User"
            autocomplete="username"
            @keyup.enter="handleSubmit"
          />
        </el-form-item>

        <el-form-item prop="password">
          <el-input
            v-model="form.password"
            type="password"
            :placeholder="passwordPlaceholder"
            size="large"
            :prefix-icon="Lock"
            show-password
            autocomplete="current-password"
            @keyup.enter="handleSubmit"
          />
        </el-form-item>

        <el-form-item v-if="isSetup" prop="confirm">
          <el-input
            v-model="form.confirm"
            type="password"
            placeholder="确认密码"
            size="large"
            :prefix-icon="Lock"
            show-password
            autocomplete="new-password"
            @keyup.enter="handleSubmit"
          />
        </el-form-item>

        <el-form-item>
          <el-button
            type="primary"
            class="login-btn"
            size="large"
            :loading="authStore.loading || bootLoading"
            @click="handleSubmit"
          >
            {{ isSetup ? '设 置 并 进 入' : '登 录' }}
          </el-button>
        </el-form-item>
      </el-form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { User, Lock } from '@element-plus/icons-vue'
import CentagMark from '@/components/icons/CentagMark.vue'
import type { FormInstance, FormRules } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import { getBootstrapStatus } from '@/api/auth'
import { isMinimalEdition } from '@/utils/edition'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()

const formRef = ref<FormInstance>()
const bootLoading = ref(true)
const isSetup = ref(false)
const form = reactive({ username: 'admin', password: '', confirm: '' })

const hideUsername = computed(() => isMinimalEdition() || isSetup.value)
const passwordPlaceholder = computed(() => {
  if (isSetup.value) {
    return isMinimalEdition() ? '设置登录令牌（至少 6 位）' : '设置管理密码（至少 6 位）'
  }
  return isMinimalEdition() ? '登录令牌' : '密码'
})
const subtitle = computed(() => {
  if (isSetup.value) return isMinimalEdition() ? '首次使用：请设置登录令牌' : '首次使用：请设置管理密码'
  if (isMinimalEdition()) {
    return '忘记令牌可删除 data/admin.password.hash 后重启重设'
  }
  return '大模型代理服务管理平台'
})

const rules = computed<FormRules>(() => {
  const base: FormRules = {
    password: [{ required: true, message: '请输入密码', trigger: 'blur' }]
  }
  if (!hideUsername.value) {
    base.username = [{ required: true, message: '请输入用户名', trigger: 'blur' }]
  }
  if (isSetup.value) {
    base.confirm = [
      { required: true, message: '请确认密码', trigger: 'blur' },
      {
        validator: (_r, v, cb) => {
          if (v !== form.password) cb(new Error('两次密码不一致'))
          else cb()
        },
        trigger: 'blur'
      }
    ]
  }
  return base
})

onMounted(async () => {
  try {
    const status = await getBootstrapStatus()
    if (status && status.initialized === false) {
      isSetup.value = true
    }
  } catch {
    // gateway/team may not expose bootstrap-status — ignore
  } finally {
    bootLoading.value = false
  }
})

async function handleSubmit() {
  if (!formRef.value) return
  try {
    await formRef.value.validate()
    if (isSetup.value) {
      if (form.password.length < 6) {
        ElMessage.warning('密码至少 6 位')
        return
      }
      await authStore.setup(form.password)
    } else {
      await authStore.login(form.username || 'admin', form.password)
    }
    const redirect = (route.query.redirect as string) || '/dashboard'
    router.push(redirect)
  } catch (err: any) {
    if (err?.message) {
      ElMessage.error(err.message || '操作失败')
    }
  }
}
</script>

<style scoped>
.login-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
}

.login-card {
  background: #fff;
  border-radius: 16px;
  padding: 48px 40px;
  width: 400px;
  max-width: 90vw;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.15);
}

.login-header {
  text-align: center;
  margin-bottom: 36px;
}

.logo-row {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  margin-bottom: 8px;
}

.title {
  font-size: 1.75rem;
  font-weight: 700;
  color: #1a1a2e;
  margin: 0;
}

.subtitle {
  font-size: 0.875rem;
  color: #888;
  margin: 0;
}

.subtitle--tip {
  margin-top: 10px;
  font-size: 0.8125rem;
  line-height: 1.5;
  color: #999;
  max-width: 320px;
  margin-left: auto;
  margin-right: auto;
}

.login-form {
  margin-top: 0;
}

.login-btn {
  width: 100%;
  height: 44px;
  font-size: 1rem;
  letter-spacing: 4px;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  border: none;
}

.login-btn:hover {
  opacity: 0.9;
}
</style>

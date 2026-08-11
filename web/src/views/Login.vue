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
            :placeholder="$t('login.username')"
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
            :placeholder="$t('login.confirmPassword')"
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
            {{ isSetup ? t('login.setupButton') : t('login.loginButton') }}
          </el-button>
        </el-form-item>
      </el-form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { User, Lock } from '@element-plus/icons-vue'
import CentagMark from '@/components/icons/CentagMark.vue'
import type { FormInstance, FormRules } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import { getBootstrapStatus } from '@/api/auth'
import { isMinimalEdition, isPersonalEdition } from '@/utils/edition'

const { t } = useI18n()
const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()

const formRef = ref<FormInstance>()
const bootLoading = ref(true)
const isSetup = ref(false)
const form = reactive({ username: 'admin', password: '', confirm: '' })

const hideUsername = computed(() => isMinimalEdition() || isPersonalEdition() || isSetup.value)
const passwordPlaceholder = computed(() => {
  if (isSetup.value) {
    return (isMinimalEdition() || isPersonalEdition()) ? t('login.setupPasswordPlaceholder') : t('login.adminPasswordPlaceholder')
  }
  return (isMinimalEdition() || isPersonalEdition()) ? t('login.passwordPlaceholder') : t('login.password')
})
const subtitle = computed(() => {
  if (isSetup.value) return (isMinimalEdition() || isPersonalEdition()) ? t('login.firstTimeTitle') : t('login.firstTimeAdminTitle')
  if (isMinimalEdition() || isPersonalEdition()) {
    return t('login.resetHint')
  }
  return t('login.subtitle')
})

const rules = computed<FormRules>(() => {
  const base: FormRules = {
    password: [{ required: true, message: t('login.pleaseEnterPassword'), trigger: 'blur' }]
  }
  if (!hideUsername.value) {
    base.username = [{ required: true, message: t('login.pleaseEnterUsername'), trigger: 'blur' }]
  }
  if (isSetup.value) {
    base.confirm = [
      { required: true, message: t('login.pleaseConfirmPassword'), trigger: 'blur' },
      {
        validator: (_r, v, cb) => {
          if (v !== form.password) cb(new Error(t('login.passwordMismatch')))
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
    // personal/team may not expose bootstrap-status — ignore
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
        ElMessage.warning(t('login.passwordMinLength'))
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
      ElMessage.error(err.message || t('login.operationFailed'))
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

.default-hint {
  text-align: center;
  font-size: 0.75rem;
  color: #aaa;
  margin-top: 20px;
  line-height: 1.6;
}
</style>

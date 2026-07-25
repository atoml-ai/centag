<template>
  <div class="settings-page">
    <div class="page-header">
      <div>
        <h1 class="page-title">{{ $t('settings.title') }}</h1>
        <p class="page-description">{{ $t('settings.title') }}</p>
      </div>
    </div>

    <el-card class="settings-card">
      <template #header>
        <span>{{ $t('settings.language') }}</span>
      </template>
      <div class="info-rows">
        <div class="info-row">
          <span class="label">{{ $t('settings.language') }}</span>
          <el-select v-model="currentLocale" @change="handleLocaleChange" style="width: 200px">
            <el-option
              v-for="locale in supportedLocales"
              :key="locale"
              :label="localeLabels[locale]"
              :value="locale"
            />
          </el-select>
        </div>
      </div>
    </el-card>

    <el-card class="settings-card">
      <template #header>
        <span>{{ $t('settings.serviceInfo') }}</span>
      </template>
      <div class="info-rows">
        <div class="info-row">
          <span class="label">{{ $t('settings.version') }}</span>
          <span class="mono">{{ status.version || '--' }}</span>
        </div>
        <div class="info-row">
          <span class="label">{{ $t('settings.edition') }}</span>
          <el-tag size="small" type="info">{{ status.edition || 'minimal' }}</el-tag>
        </div>
        <div class="info-row">
          <span class="label">{{ $t('settings.uptime') }}</span>
          <span>{{ formatUptime(status.uptime) }}</span>
        </div>
        <div class="info-row">
          <span class="label">{{ $t('settings.apiAddress') }}</span>
          <code>{{ apiBase }}</code>
        </div>
      </div>
    </el-card>

    <el-card class="settings-card">
      <template #header>
        <span>{{ $t('settings.changePassword') }}</span>
      </template>
      <el-form label-width="100px" style="max-width: 420px" @submit.prevent="changePassword">
        <el-form-item :label="$t('settings.oldPassword')">
          <el-input v-model="pwd.oldPassword" type="password" show-password autocomplete="current-password" />
        </el-form-item>
        <el-form-item :label="$t('settings.newPassword')">
          <el-input v-model="pwd.newPassword" type="password" show-password autocomplete="new-password" />
        </el-form-item>
        <el-form-item :label="$t('settings.confirmPassword')">
          <el-input v-model="pwd.confirm" type="password" show-password autocomplete="new-password" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="saving" @click="changePassword">{{ $t('common.save') }}</el-button>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { getStatus } from '@/api'
import api from '@/api'
import { resolveApiBaseUrl } from '@/utils/apiBaseUrl'
import { formatUptime } from '@/utils/format'
import { useLocaleStore } from '@/stores/locale'
import { supportedLocales, localeLabels, type AppLocale } from '@/i18n'

const { t } = useI18n()
const localeStore = useLocaleStore()
const currentLocale = ref<AppLocale>(localeStore.getLocale())

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

function handleLocaleChange(locale: AppLocale) {
  localeStore.setLocale(locale)
  ElMessage.success(t('settings.languageUpdated'))
}

async function changePassword() {
  if (!pwd.oldPassword || !pwd.newPassword) {
    ElMessage.warning(t('settings.fillAllFields'))
    return
  }
  if (pwd.newPassword.length < 6) {
    ElMessage.warning(t('settings.passwordMinLength'))
    return
  }
  if (pwd.newPassword !== pwd.confirm) {
    ElMessage.warning(t('settings.passwordMismatch'))
    return
  }
  saving.value = true
  try {
    await api.post('/api/v1/settings/password', {
      old_password: pwd.oldPassword,
      new_password: pwd.newPassword
    })
    ElMessage.success(t('settings.passwordUpdated'))
    pwd.oldPassword = ''
    pwd.newPassword = ''
    pwd.confirm = ''
  } catch (e: any) {
    ElMessage.error(e?.message || t('settings.updateFailed'))
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.settings-page {
  width: 100%;
  padding: 0 0 24px;
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

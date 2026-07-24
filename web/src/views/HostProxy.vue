<template>
  <div class="host-proxy">
    <div class="header-with-toolbar">
      <div class="header-left">
        <h1 class="page-title">{{ t('hostProxy.title') }}</h1>
        <p class="page-description">{{ t('hostProxy.subtitle') }}</p>
      </div>
      <div class="toolbar-actions">
        <el-button :loading="loading" @click="load">
          <el-icon><Refresh /></el-icon>
          {{ t('hostProxy.refresh') }}
        </el-button>
        <el-button type="primary" @click="openSetupWizard">
          <el-icon><MagicStick /></el-icon>
          {{ t('hostProxy.quickSetup') }}
        </el-button>
      </div>
    </div>

    <div class="content-wrapper">
      <!-- 状态卡片 -->
      <el-row :gutter="12" class="metrics-grid">
        <el-col :xs="12" :sm="8" :md="6" :lg="4">
          <div class="metric-card" :class="{ 'status-active': status.enabled, 'status-inactive': !status.enabled }">
            <div class="metric-icon" :class="status.enabled ? 'status-active-icon' : 'status-inactive-icon'">
              <el-icon :size="22"><Connection /></el-icon>
            </div>
            <div class="metric-content">
              <div class="metric-value">{{ status.enabled ? t('hostProxy.status.enabled') : t('hostProxy.status.disabled') }}</div>
              <div class="metric-label">{{ t('hostProxy.status.proxyStatus') }}</div>
            </div>
          </div>
        </el-col>
        <el-col :xs="12" :sm="8" :md="6" :lg="4">
          <div class="metric-card">
            <div class="metric-icon domain-icon">
              <el-icon :size="22"><Link /></el-icon>
            </div>
            <div class="metric-content">
              <div class="metric-value">{{ domainCount }}</div>
              <div class="metric-label">{{ t('hostProxy.status.proxyDomains') }}</div>
            </div>
          </div>
        </el-col>
        <el-col :xs="12" :sm="8" :md="6" :lg="4">
          <div class="metric-card">
            <div class="metric-icon http-icon">
              <el-icon :size="22"><Lock /></el-icon>
            </div>
            <div class="metric-content">
              <div class="metric-value">{{ status.http_port }}</div>
              <div class="metric-label">{{ t('hostProxy.status.httpPort') }}</div>
            </div>
          </div>
        </el-col>
        <el-col :xs="12" :sm="8" :md="6" :lg="4">
          <div class="metric-card">
            <div class="metric-icon https-icon">
              <el-icon :size="22"><Lock /></el-icon>
            </div>
            <div class="metric-content">
              <div class="metric-value">{{ status.https_port }}</div>
              <div class="metric-label">{{ t('hostProxy.status.httpsPort') }}</div>
            </div>
          </div>
        </el-col>
      </el-row>

      <!-- 代理控制 -->
      <el-card class="control-card">
        <template #header>
          <div class="card-header">
            <span class="card-title">{{ t('hostProxy.control.title') }}</span>
            <el-switch
              v-model="status.enabled"
              :loading="toggling"
              @change="toggleProxy"
              :active-text="t('hostProxy.control.activeText')"
              :inactive-text="t('hostProxy.control.inactiveText')"
            />
          </div>
        </template>
        <div class="control-content">
          <el-form label-width="90px" class="port-form">
            <el-row :gutter="16">
              <el-col :span="11">
                <el-form-item :label="t('hostProxy.control.httpPort')" style="margin-bottom: 8px;">
                  <el-input-number
                    v-model="portForm.httpPort"
                    :min="1024"
                    :max="65535"
                    :disabled="status.enabled"
                    controls-position="right"
                    style="width: 100%"
                  />
                </el-form-item>
              </el-col>
              <el-col :span="11">
                <el-form-item :label="t('hostProxy.control.httpsPort')" style="margin-bottom: 8px;">
                  <el-input-number
                    v-model="portForm.httpsPort"
                    :min="1024"
                    :max="65535"
                    :disabled="status.enabled"
                    controls-position="right"
                    style="width: 100%"
                  />
                </el-form-item>
              </el-col>
            </el-row>
          </el-form>

          <el-divider />

          <el-alert
            :title="status.enabled ? t('hostProxy.control.proxyEnabled') : t('hostProxy.control.proxyDisabled')"
            :type="status.enabled ? 'success' : 'info'"
            :closable="false"
            show-icon
            class="status-alert"
          >
            <template #default>
              <span v-if="status.enabled">{{ t('hostProxy.control.enabledHint') }}</span>
              <span v-else>{{ t('hostProxy.control.disabledHint') }}</span>
            </template>
          </el-alert>
        </div>
      </el-card>

      <!-- 域名管理 -->
      <el-card class="domains-card">
        <template #header>
          <div class="card-header">
            <span class="card-title">{{ t('hostProxy.domains.title') }}</span>
            <el-button type="primary" @click="openAddDomain" :disabled="!status.enabled">
              <el-icon><Plus /></el-icon>
              {{ t('hostProxy.domains.addDomain') }}
            </el-button>
          </div>
        </template>
        <el-table :data="domainList" stripe v-loading="loading">
          <el-table-column prop="domain" :label="t('hostProxy.domains.domain')" min-width="200" />
          <el-table-column prop="target" :label="t('hostProxy.domains.target')" min-width="200" />
          <el-table-column :label="t('hostProxy.domains.test')" width="100" align="center">
            <template #default="{ row }">
              <el-button
                type="primary"
                link
                @click="testDomain(row.domain)"
                :loading="testingDomains[row.domain]"
              >
                <el-icon><Position /></el-icon>
                {{ t('hostProxy.domains.test') }}
              </el-button>
            </template>
          </el-table-column>
          <el-table-column :label="t('storage.table.actions')" width="120" align="center">
            <template #default="{ row, $index }">
              <el-button type="danger" link @click="removeDomain(row, $index)" :disabled="!status.enabled">
                <el-icon><Delete /></el-icon>
                {{ t('hostProxy.domains.delete') }}
              </el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-card>

      <!-- CA证书管理 -->
      <el-card class="cert-card">
        <template #header>
          <div class="card-title">{{ t('hostProxy.cert.title') }}</div>
        </template>
        <div class="cert-content">
          <el-alert
            :title="t('hostProxy.cert.httpsWarning')"
            type="warning"
            :closable="false"
            show-icon
          >
            <template #default>
              <p>{{ t('hostProxy.cert.httpsWarningDesc') }}</p>
            </template>
          </el-alert>
          <div class="cert-actions mt-md">
            <el-button type="primary" @click="downloadCACert">
              <el-icon><Download /></el-icon>
              {{ t('hostProxy.cert.downloadCert') }}
            </el-button>
            <el-button @click="copyCertCommand">
              <el-icon><DocumentCopy /></el-icon>
              {{ t('hostProxy.cert.copyInstallCmd') }}
            </el-button>
          </div>
          <el-divider />
          <div class="cert-info">
            <el-descriptions :column="2" border>
              <el-descriptions-item :label="t('hostProxy.cert.status')">
                <el-tag :type="certInfo.valid ? 'success' : 'danger'" size="small">
                  {{ certInfo.valid ? t('hostProxy.cert.valid') : t('hostProxy.cert.invalid') }}
                </el-tag>
              </el-descriptions-item>
              <el-descriptions-item :label="t('hostProxy.cert.issuer')">{{ certInfo.issuer }}</el-descriptions-item>
              <el-descriptions-item :label="t('hostProxy.cert.expires')">{{ certInfo.expires }}</el-descriptions-item>
              <el-descriptions-item :label="t('hostProxy.cert.daysLeft')">{{ certInfo.daysLeft }}{{ t('hostProxy.cert.days') }}</el-descriptions-item>
            </el-descriptions>
          </div>
        </div>
      </el-card>

      <!-- 配置指南 -->
      <el-card class="guide-card">
        <template #header>
          <div class="card-header">
            <span class="card-title">{{ t('hostProxy.guide.title') }}</span>
            <el-button link @click="toggleGuide">
              {{ showGuide ? t('hostProxy.guide.collapse') : t('hostProxy.guide.expand') }}
              <el-icon><ArrowDown v-if="!showGuide" /><ArrowUp v-else /></el-icon>
            </el-button>
          </div>
        </template>
        <div v-show="showGuide" class="guide-content">
          <el-steps :active="setupSteps.active" align-center>
            <el-step :title="t('hostProxy.guide.step1Title')" :description="t('hostProxy.guide.step1Desc')" />
            <el-step :title="t('hostProxy.guide.step2Title')" :description="t('hostProxy.guide.step2Desc')" />
            <el-step :title="t('hostProxy.guide.step3Title')" :description="t('hostProxy.guide.step3Desc')" />
            <el-step :title="t('hostProxy.guide.step4Title')" :description="t('hostProxy.guide.step4Desc')" />
          </el-steps>

          <el-divider />

          <div class="step-details">
            <h3>{{ t('hostProxy.guide.hostsStepTitle') }}</h3>
            <el-alert type="warning" :closable="false" class="mb-md">
              <template #default>
                <p>{{ t('hostProxy.guide.hostsStepDesc') }}</p>
              </template>
            </el-alert>
            <el-input
              type="textarea"
              :rows="3"
              :model-value="hostsCommand"
              readonly
              class="mb-md"
            />
            <el-button type="primary" @click="copyCommand(hostsCommand)">
              <el-icon><DocumentCopy /></el-icon>
              {{ t('hostProxy.guide.copyCommand') }}
            </el-button>

            <el-divider />

            <h3>{{ t('hostProxy.guide.certStepTitle') }}</h3>
            <el-alert type="info" :closable="false" class="mb-md">
              <template #default>
                <p>{{ t('hostProxy.guide.certStepDesc') }}</p>
              </template>
            </el-alert>
            <el-input
              type="textarea"
              :rows="2"
              model-value="curl -o ca.crt http://127.0.0.1:20060/api/v1/host-proxy/ca-cert"
              readonly
              class="mb-md"
            />
            <el-button type="primary" @click="copyCommand('curl -o ca.crt http://127.0.0.1:20060/api/v1/host-proxy/ca-cert')">
              <el-icon><DocumentCopy /></el-icon>
              {{ t('hostProxy.guide.copyDownloadCmd') }}
            </el-button>
            <div class="mt-md">
              <h4>{{ t('hostProxy.guide.linuxMac') }}</h4>
              <el-input
                type="textarea"
                :rows="2"
                model-value="sudo cp ca.crt /usr/local/share/ca-certificates/ca.crt && sudo update-ca-certificates"
                readonly
                class="mb-md"
              />
              <el-button @click="copyCommand('sudo cp ca.crt /usr/local/share/ca-certificates/ca.crt && sudo update-ca-certificates')">
                <el-icon><DocumentCopy /></el-icon>
                {{ t('hostProxy.guide.copyInstallCmd') }}
              </el-button>
            </div>

            <el-divider />

            <h3>{{ t('hostProxy.guide.testStepTitle') }}</h3>
            <el-alert type="success" :closable="false" class="mb-md">
              <template #default>
                <p>{{ t('hostProxy.guide.testStepDesc') }}</p>
              </template>
            </el-alert>
            <el-input
              type="textarea"
              :rows="1"
              model-value="curl https://api.openai.com/v1/models"
              readonly
              class="mb-md"
            />
            <el-button type="primary" @click="copyCommand('curl https://api.openai.com/v1/models')">
              <el-icon><DocumentCopy /></el-icon>
              {{ t('hostProxy.guide.copyTestCmd') }}
            </el-button>
          </div>
        </div>
      </el-card>
    </div>

    <!-- 添加域名对话框 -->
    <el-dialog v-model="showAddDialog" :title="t('hostProxy.addDialog.title')" width="500px">
      <el-form :model="newDomain" label-width="100px">
        <el-form-item :label="t('hostProxy.addDialog.domain')">
          <el-input v-model="newDomain.domain" :placeholder="t('hostProxy.addDialog.domainPlaceholder')" />
        </el-form-item>
        <el-form-item :label="t('hostProxy.addDialog.target')">
          <el-input v-model="newDomain.target" :placeholder="t('hostProxy.addDialog.targetPlaceholder')" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showAddDialog = false">{{ t('hostProxy.addDialog.cancel') }}</el-button>
        <el-button type="primary" @click="addDomain" :loading="adding">{{ t('hostProxy.addDialog.add') }}</el-button>
      </template>
    </el-dialog>

    <!-- 配置向导 -->
    <el-dialog v-model="showWizard" :title="t('hostProxy.wizard.title')" width="600px">
      <el-steps :active="wizardStep" finish-status="success" align-center class="mb-lg">
        <el-step :title="t('hostProxy.wizard.step1Title')" />
        <el-step :title="t('hostProxy.wizard.step2Title')" />
        <el-step :title="t('hostProxy.wizard.step3Title')" />
      </el-steps>

      <div v-if="wizardStep === 0" class="wizard-step">
        <el-descriptions :column="1" border>
          <el-descriptions-item :label="t('hostProxy.wizard.proxyStatus')">
            <el-tag :type="status.enabled ? 'success' : 'info'">{{ status.enabled ? t('hostProxy.status.enabled') : t('hostProxy.status.disabled') }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item :label="t('hostProxy.wizard.domainCount')">{{ domainCount }}</el-descriptions-item>
          <el-descriptions-item :label="t('hostProxy.wizard.httpPort')">{{ status.http_port }}</el-descriptions-item>
          <el-descriptions-item :label="t('hostProxy.wizard.httpsPort')">{{ status.https_port }}</el-descriptions-item>
        </el-descriptions>
        <el-alert type="info" class="mt-md" :closable="false">
          {{ t('hostProxy.wizard.nonPrivilegedHint') }}
        </el-alert>
      </div>

      <div v-if="wizardStep === 1" class="wizard-step">
        <h3>{{ t('hostProxy.wizard.configHostsTitle') }}</h3>
        <p class="mb-md">{{ t('hostProxy.wizard.configHostsDesc') }}</p>
        <el-input
          type="textarea"
          :rows="domainCount + 1"
          :model-value="hostsCommand"
          readonly
          class="mb-md"
        />
        <el-button @click="copyCommand(hostsCommand)">
          <el-icon><DocumentCopy /></el-icon>
          {{ t('hostProxy.wizard.copyCommand') }}
        </el-button>
      </div>

      <div v-if="wizardStep === 2" class="wizard-step">
        <el-result icon="success" :title="t('hostProxy.wizard.configComplete')" :sub-title="t('hostProxy.wizard.configCompleteSub')">
          <template #extra>
            <el-button type="primary" @click="showWizard = false">{{ t('hostProxy.wizard.finish') }}</el-button>
            <el-button @click="testProxy">{{ t('hostProxy.wizard.testProxy') }}</el-button>
          </template>
        </el-result>
      </div>

      <template #footer>
        <el-button @click="wizardStep--" :disabled="wizardStep === 0">{{ t('hostProxy.wizard.prevStep') }}</el-button>
        <el-button type="primary" @click="wizardStep++" :disabled="wizardStep === 2">
          {{ t('hostProxy.wizard.nextStep') }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Refresh,
  MagicStick,
  Connection,
  Link,
  Lock,
  Plus,
  Position,
  Delete,
  Download,
  DocumentCopy,
  ArrowDown,
  ArrowUp
} from '@element-plus/icons-vue'
import api from '@/api'

const { t } = useI18n()

interface HostProxyStatus {
  enabled: boolean
  http_port: number
  https_port: number
  backend_addr: string
  domains: Record<string, string>
}

interface CertInfo {
  valid: boolean
  issuer: string
  expires: string
  daysLeft: number
}

const loading = ref(false)
const toggling = ref(false)
const status = ref<HostProxyStatus>({
  enabled: false,
  http_port: 80,
  https_port: 443,
  backend_addr: '127.0.0.1:20060',
  domains: {}
})
const portForm = ref({
  httpPort: 8080,
  httpsPort: 8443
})
const certInfo = ref<CertInfo>({
  valid: true,
  issuer: 'Centag',
  expires: '2036-02-17',
  daysLeft: 3650
})
const testingDomains = ref<Record<string, boolean>>({})
const showGuide = ref(false)
const showAddDialog = ref(false)
const showWizard = ref(false)
const wizardStep = ref(0)
const adding = ref(false)
const newDomain = ref({
  domain: '',
  target: 'http://127.0.0.1:20060'
})

const domainCount = computed(() => Object.keys(status.value.domains).length)
const domainList = computed(() => {
  return Object.entries(status.value.domains).map(([domain, target]) => ({
    domain,
    target
  }))
})

const hostsCommand = computed(() => {
  const entries = Object.keys(status.value.domains).map(domain => `127.0.0.1 ${domain}`)
  return entries.join('\n')
})

const setupSteps = ref({
  active: -1
})

const load = async () => {
  loading.value = true
  try {
    const data = await api.get('/api/v1/host-proxy/status')
    status.value = data
    portForm.value.httpPort = data.http_port || 8080
    portForm.value.httpsPort = data.https_port || 8443
  } catch (error: any) {
    ElMessage.error(t('hostProxy.message.loadStatusFailed') + ': ' + error.message)
  } finally {
    loading.value = false
  }
}

const toggleProxy = async () => {
  toggling.value = true
  try {
    await api.put('/api/v1/config', {
      host_proxy: {
        enabled: status.value.enabled,
        http_port: portForm.value.httpPort,
        https_port: portForm.value.httpsPort
      }
    })
    ElMessage.success(status.value.enabled ? t('hostProxy.message.proxyEnabled') : t('hostProxy.message.proxyDisabled'))
    await load()
  } catch (error: any) {
    ElMessage.error(t('hostProxy.message.operationFailed') + ': ' + error.message)
    status.value.enabled = !status.value.enabled
  } finally {
    toggling.value = false
  }
}

const openAddDomain = () => {
  newDomain.value = {
    domain: '',
    target: 'http://127.0.0.1:20060'
  }
  showAddDialog.value = true
}

const addDomain = async () => {
  if (!newDomain.value.domain) {
    ElMessage.warning(t('hostProxy.validation.pleaseEnterDomain'))
    return
  }

  adding.value = true
  try {
    const domains = { ...status.value.domains }
    domains[newDomain.value.domain] = newDomain.value.target
    await api.put('/api/v1/host-proxy/domains', { domains })
    ElMessage.success(t('hostProxy.message.domainAdded'))
    showAddDialog.value = false
    await load()
  } catch (error: any) {
    ElMessage.error(t('hostProxy.message.addFailed') + ': ' + error.message)
  } finally {
    adding.value = false
  }
}

const removeDomain = async (row: any, index: number) => {
  try {
    await ElMessageBox.confirm(t('hostProxy.message.deleteDomainConfirm', { domain: row.domain }), t('hostProxy.message.deleteConfirmTitle'), {
      type: 'warning'
    })

    const domains = { ...status.value.domains }
    delete domains[row.domain]
    await api.put('/api/v1/host-proxy/domains', { domains })
    ElMessage.success(t('hostProxy.message.domainDeleted'))
    await load()
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error(t('hostProxy.message.deleteFailed') + ': ' + error.message)
    }
  }
}

const testDomain = async (domain: string) => {
  testingDomains.value[domain] = true
  try {
    const response = await api.get(`http://${domain}/v1/models`)
    ElMessage.success(t('hostProxy.message.domainTestSuccess', { domain }))
  } catch (error: any) {
    ElMessage.error(t('hostProxy.message.testFailed', { message: error.message }))
  } finally {
    testingDomains.value[domain] = false
  }
}

const downloadCACert = async () => {
  try {
    const response = await api.get('/api/v1/host-proxy/ca-cert', { responseType: 'blob' })
    const url = window.URL.createObjectURL(new Blob([response.data]))
    const link = document.createElement('a')
    link.href = url
    link.setAttribute('download', 'ca.crt')
    document.body.appendChild(link)
    link.click()
    link.remove()
    window.URL.revokeObjectURL(url)
    ElMessage.success(t('hostProxy.message.caCertDownloaded'))
  } catch (error: any) {
    ElMessage.error(t('hostProxy.message.downloadFailed') + ': ' + error.message)
  }
}

const copyCommand = async (command: string) => {
  try {
    await navigator.clipboard.writeText(command)
    ElMessage.success(t('hostProxy.message.copiedToClipboard'))
  } catch (error) {
    ElMessage.error(t('hostProxy.message.copyFailed'))
  }
}

const copyCertCommand = async () => {
  const commands = [
    t('hostProxy.certComment.download'),
    'curl -o ca.crt http://127.0.0.1:20060/api/v1/host-proxy/ca-cert',
    '',
    t('hostProxy.certComment.linuxMac'),
    'sudo cp ca.crt /usr/local/share/ca-certificates/ca.crt',
    'sudo update-ca-certificates',
    '',
    t('hostProxy.certComment.windows')
  ].join('\n')
  await copyCommand(commands)
}

const toggleGuide = () => {
  showGuide.value = !showGuide.value
}

const openSetupWizard = () => {
  wizardStep.value = 0
  showWizard.value = true
}

const testProxy = async () => {
  try {
    await api.get('https://api.openai.com/v1/models')
    ElMessage.success(t('hostProxy.message.proxyTestSuccess'))
  } catch (error: any) {
    ElMessage.error(t('hostProxy.message.testFailed', { message: error.message }))
  }
}

onMounted(() => {
  load()
})
</script>

<style scoped>
.host-proxy {
  min-height: 100vh;
}

.header-with-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 24px;
  flex-wrap: wrap;
  gap: 16px;
}

.header-left {
  flex: 1;
  min-width: 200px;
}

.page-title {
  font-size: 24px;
  font-weight: 600;
  margin: 0 0 8px 0;
  color: var(--el-text-color-primary);
}

.page-description {
  font-size: 14px;
  color: var(--el-text-color-secondary);
  margin: 0;
}

.toolbar-actions {
  display: flex;
  gap: 12px;
}

.content-wrapper {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.metrics-grid {
  margin-bottom: 16px;
}

.metric-card {
  background: var(--el-bg-color);
  border-radius: 8px;
  padding: 12px 14px;
  display: flex;
  align-items: center;
  gap: 10px;
  transition: all 0.3s;
  border: 1px solid var(--el-border-color);
}

.metric-card:hover {
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
  transform: translateY(-2px);
}

.metric-card.status-active {
  border-color: var(--el-color-success);
  background: linear-gradient(135deg, rgba(103, 194, 58, 0.1) 0%, var(--el-bg-color) 100%);
}

.metric-card.status-inactive {
  border-color: var(--el-color-info);
  background: linear-gradient(135deg, rgba(144, 147, 153, 0.1) 0%, var(--el-bg-color) 100%);
}

.metric-icon {
  width: 40px;
  height: 40px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--el-color-primary-light-9);
  color: var(--el-color-primary);
  flex-shrink: 0;
}

.metric-icon.status-active-icon {
  background: var(--el-color-success-light-9);
  color: var(--el-color-success);
}

.metric-icon.status-inactive-icon {
  background: var(--el-color-info-light-9);
  color: var(--el-color-info);
}

.metric-icon.domain-icon {
  background: var(--el-color-primary-light-9);
  color: var(--el-color-primary);
}

.metric-icon.http-icon {
  background: var(--el-color-warning-light-9);
  color: var(--el-color-warning);
}

.metric-icon.https-icon {
  background: var(--el-color-success-light-9);
  color: var(--el-color-success);
}

.metric-content {
  flex: 1;
}

.metric-value {
  font-size: 16px;
  font-weight: 600;
  color: var(--el-text-color-primary);
  margin-bottom: 2px;
}

.metric-label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.control-card,
.domains-card,
.cert-card,
.guide-card {
  border-radius: 8px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.card-title {
  font-weight: 600;
  font-size: 16px;
}

.control-content {
  margin-top: 16px;
}

.port-form {
  :deep(.el-form-item__label) {
    white-space: nowrap;
  }
}

.status-alert {
  :deep(.el-alert__title) {
    font-size: 13px;
  }
  :deep(.el-alert__description) {
    font-size: 12px;
    margin-top: 2px;
  }
}

.cert-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.cert-actions {
  display: flex;
  gap: 12px;
}

.cert-info {
  margin-top: 16px;
}

.guide-content {
  padding: 16px 0;
}

.step-details {
  margin-top: 24px;
}

.step-details h3 {
  margin: 24px 0 12px 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.step-details h4 {
  margin: 16px 0 8px 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.wizard-step {
  padding: 20px 0;
  min-height: 200px;
}

.text-warning {
  color: var(--el-color-warning);
  font-size: 13px;
}

.mt-lg {
  margin-top: 24px;
}

.mt-md {
  margin-top: 16px;
}

.mb-md {
  margin-bottom: 16px;
}

@media (max-width: 768px) {
  .header-with-toolbar {
    flex-direction: column;
    align-items: flex-start;
  }

  .toolbar-actions {
    width: 100%;
    flex-wrap: wrap;
  }

  .toolbar-actions .el-button {
    flex: 1;
    min-width: 100px;
  }
}
</style>

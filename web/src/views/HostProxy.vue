<template>
  <div class="host-proxy">
    <div class="header-with-toolbar">
      <div class="header-left">
        <h1 class="page-title">Host代理管理</h1>
        <p class="page-description">通过修改hosts文件实现透明代理,无需修改客户端代码</p>
      </div>
      <div class="toolbar-actions">
        <el-button :loading="loading" @click="load">
          <el-icon><Refresh /></el-icon>
          刷新
        </el-button>
        <el-button type="primary" @click="openSetupWizard">
          <el-icon><MagicStick /></el-icon>
          快速配置向导
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
              <div class="metric-value">{{ status.enabled ? '已启用' : '已禁用' }}</div>
              <div class="metric-label">代理状态</div>
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
              <div class="metric-label">代理域名数</div>
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
              <div class="metric-label">HTTP端口</div>
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
              <div class="metric-label">HTTPS端口</div>
            </div>
          </div>
        </el-col>
      </el-row>

      <!-- 代理控制 -->
      <el-card class="control-card">
        <template #header>
          <div class="card-header">
            <span class="card-title">代理控制</span>
            <el-switch
              v-model="status.enabled"
              :loading="toggling"
              @change="toggleProxy"
              active-text="已启用"
              inactive-text="已禁用"
            />
          </div>
        </template>
        <div class="control-content">
          <el-form label-width="90px" class="port-form">
            <el-row :gutter="16">
              <el-col :span="11">
                <el-form-item label="HTTP端口" style="margin-bottom: 8px;">
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
                <el-form-item label="HTTPS端口" style="margin-bottom: 8px;">
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
            :title="status.enabled ? 'Host代理已启用' : 'Host代理已禁用'"
            :type="status.enabled ? 'success' : 'info'"
            :closable="false"
            show-icon
            class="status-alert"
          >
            <template #default>
              <span v-if="status.enabled">所有配置的域名将解析到本地代理服务器(127.0.0.1)</span>
              <span v-else>禁用后,域名将正常解析到真实服务器。端口修改后，启用时自动保存生效。</span>
            </template>
          </el-alert>
        </div>
      </el-card>

      <!-- 域名管理 -->
      <el-card class="domains-card">
        <template #header>
          <div class="card-header">
            <span class="card-title">域名配置</span>
            <el-button type="primary" @click="openAddDomain" :disabled="!status.enabled">
              <el-icon><Plus /></el-icon>
              添加域名
            </el-button>
          </div>
        </template>
        <el-table :data="domainList" stripe v-loading="loading">
          <el-table-column prop="domain" label="域名" min-width="200" />
          <el-table-column prop="target" label="目标地址" min-width="200" />
          <el-table-column label="测试" width="100" align="center">
            <template #default="{ row }">
              <el-button
                type="primary"
                link
                @click="testDomain(row.domain)"
                :loading="testingDomains[row.domain]"
              >
                <el-icon><Position /></el-icon>
                测试
              </el-button>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="120" align="center">
            <template #default="{ row, $index }">
              <el-button type="danger" link @click="removeDomain(row, $index)" :disabled="!status.enabled">
                <el-icon><Delete /></el-icon>
                删除
              </el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-card>

      <!-- CA证书管理 -->
      <el-card class="cert-card">
        <template #header>
          <div class="card-title">CA证书管理</div>
        </template>
        <div class="cert-content">
          <el-alert
            title="HTTPS请求需要信任CA证书"
            type="warning"
            :closable="false"
            show-icon
          >
            <template #default>
              <p>代理使用自签名证书拦截HTTPS流量,需要将CA证书安装到系统。</p>
            </template>
          </el-alert>
          <div class="cert-actions mt-md">
            <el-button type="primary" @click="downloadCACert">
              <el-icon><Download /></el-icon>
              下载CA证书
            </el-button>
            <el-button @click="copyCertCommand">
              <el-icon><DocumentCopy /></el-icon>
              复制安装命令
            </el-button>
          </div>
          <el-divider />
          <div class="cert-info">
            <el-descriptions :column="2" border>
              <el-descriptions-item label="证书状态">
                <el-tag :type="certInfo.valid ? 'success' : 'danger'" size="small">
                  {{ certInfo.valid ? '有效' : '无效' }}
                </el-tag>
              </el-descriptions-item>
              <el-descriptions-item label="颁发者">{{ certInfo.issuer }}</el-descriptions-item>
              <el-descriptions-item label="有效期至">{{ certInfo.expires }}</el-descriptions-item>
              <el-descriptions-item label="剩余天数">{{ certInfo.daysLeft }}天</el-descriptions-item>
            </el-descriptions>
          </div>
        </div>
      </el-card>

      <!-- 配置指南 -->
      <el-card class="guide-card">
        <template #header>
          <div class="card-header">
            <span class="card-title">配置指南</span>
            <el-button link @click="toggleGuide">
              {{ showGuide ? '收起' : '展开' }}
              <el-icon><ArrowDown v-if="!showGuide" /><ArrowUp v-else /></el-icon>
            </el-button>
          </div>
        </template>
        <div v-show="showGuide" class="guide-content">
          <el-steps :active="setupSteps.active" align-center>
            <el-step title="启用代理" description="开启Host代理功能" />
            <el-step title="配置hosts" description="修改/etc/hosts文件" />
            <el-step title="安装证书" description="信任CA证书" />
            <el-step title="测试验证" description="验证代理是否工作" />
          </el-steps>

          <el-divider />

          <div class="step-details">
            <h3>步骤1: 配置hosts文件</h3>
            <el-alert type="warning" :closable="false" class="mb-md">
              <template #default>
                <p>将代理的域名添加到/etc/hosts文件中,将域名解析到127.0.0.1</p>
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
              复制命令
            </el-button>

            <el-divider />

            <h3>步骤2: 安装CA证书</h3>
            <el-alert type="info" :closable="false" class="mb-md">
              <template #default>
                <p>下载CA证书并安装到系统信任库</p>
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
              复制下载命令
            </el-button>
            <div class="mt-md">
              <h4>Linux/Mac:</h4>
              <el-input
                type="textarea"
                :rows="2"
                model-value="sudo cp ca.crt /usr/local/share/ca-certificates/ca.crt && sudo update-ca-certificates"
                readonly
                class="mb-md"
              />
              <el-button @click="copyCommand('sudo cp ca.crt /usr/local/share/ca-certificates/ca.crt && sudo update-ca-certificates')">
                <el-icon><DocumentCopy /></el-icon>
                复制安装命令
              </el-button>
            </div>

            <el-divider />

            <h3>步骤3: 测试代理</h3>
            <el-alert type="success" :closable="false" class="mb-md">
              <template #default>
                <p>验证代理是否正常工作</p>
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
              复制测试命令
            </el-button>
          </div>
        </div>
      </el-card>
    </div>

    <!-- 添加域名对话框 -->
    <el-dialog v-model="showAddDialog" title="添加域名" width="500px">
      <el-form :model="newDomain" label-width="100px">
        <el-form-item label="域名">
          <el-input v-model="newDomain.domain" placeholder="例如: api.openai.com" />
        </el-form-item>
        <el-form-item label="目标地址">
          <el-input v-model="newDomain.target" placeholder="例如: http://127.0.0.1:20060" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showAddDialog = false">取消</el-button>
        <el-button type="primary" @click="addDomain" :loading="adding">添加</el-button>
      </template>
    </el-dialog>

    <!-- 配置向导 -->
    <el-dialog v-model="showWizard" title="Host代理快速配置向导" width="600px">
      <el-steps :active="wizardStep" finish-status="success" align-center class="mb-lg">
        <el-step title="检查配置" />
        <el-step title="配置hosts" />
        <el-step title="完成" />
      </el-steps>

      <div v-if="wizardStep === 0" class="wizard-step">
        <el-descriptions :column="1" border>
          <el-descriptions-item label="代理状态">
            <el-tag :type="status.enabled ? 'success' : 'info'">{{ status.enabled ? '已启用' : '已禁用' }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="域名数量">{{ domainCount }}</el-descriptions-item>
          <el-descriptions-item label="HTTP端口">{{ status.http_port }}</el-descriptions-item>
          <el-descriptions-item label="HTTPS端口">{{ status.https_port }}</el-descriptions-item>
        </el-descriptions>
        <el-alert type="info" class="mt-md" :closable="false">
          使用非特权端口(8080/8443)，无需配置端口权限
        </el-alert>
      </div>

      <div v-if="wizardStep === 1" class="wizard-step">
        <h3>配置hosts文件</h3>
        <p class="mb-md">将以下内容添加到/etc/hosts文件:</p>
        <el-input
          type="textarea"
          :rows="domainCount + 1"
          :model-value="hostsCommand"
          readonly
          class="mb-md"
        />
        <el-button @click="copyCommand(hostsCommand)">
          <el-icon><DocumentCopy /></el-icon>
          复制命令
        </el-button>
      </div>

      <div v-if="wizardStep === 2" class="wizard-step">
        <el-result icon="success" title="配置完成" sub-title="Host代理已配置完成">
          <template #extra>
            <el-button type="primary" @click="showWizard = false">完成</el-button>
            <el-button @click="testProxy">测试代理</el-button>
          </template>
        </el-result>
      </div>

      <template #footer>
        <el-button @click="wizardStep--" :disabled="wizardStep === 0">上一步</el-button>
        <el-button type="primary" @click="wizardStep++" :disabled="wizardStep === 2">
          下一步
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
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

// 加载状态
const load = async () => {
  loading.value = true
  try {
    const data = await api.get('/api/v1/host-proxy/status')
    status.value = data
    // 同步端口表单
    portForm.value.httpPort = data.http_port || 8080
    portForm.value.httpsPort = data.https_port || 8443
  } catch (error: any) {
    ElMessage.error('加载状态失败: ' + error.message)
  } finally {
    loading.value = false
  }
}

// 切换代理状态，同时保存端口配置（与 SystemProxy 风格一致）
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
    ElMessage.success(status.value.enabled ? 'Host代理已启用' : 'Host代理已禁用')
    await load()
  } catch (error: any) {
    ElMessage.error('操作失败: ' + error.message)
    status.value.enabled = !status.value.enabled
  } finally {
    toggling.value = false
  }
}

// 添加域名
const openAddDomain = () => {
  newDomain.value = {
    domain: '',
    target: 'http://127.0.0.1:20060'
  }
  showAddDialog.value = true
}

const addDomain = async () => {
  if (!newDomain.value.domain) {
    ElMessage.warning('请输入域名')
    return
  }

  adding.value = true
  try {
    const domains = { ...status.value.domains }
    domains[newDomain.value.domain] = newDomain.value.target
    await api.put('/api/v1/host-proxy/domains', { domains })
    ElMessage.success('域名添加成功')
    showAddDialog.value = false
    await load()
  } catch (error: any) {
    ElMessage.error('添加失败: ' + error.message)
  } finally {
    adding.value = false
  }
}

// 删除域名
const removeDomain = async (row: any, index: number) => {
  try {
    await ElMessageBox.confirm(`确定删除域名 ${row.domain} 吗?`, '确认删除', {
      type: 'warning'
    })

    const domains = { ...status.value.domains }
    delete domains[row.domain]
    await api.put('/api/v1/host-proxy/domains', { domains })
    ElMessage.success('域名删除成功')
    await load()
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error('删除失败: ' + error.message)
    }
  }
}

// 测试域名
const testDomain = async (domain: string) => {
  testingDomains.value[domain] = true
  try {
    const response = await api.get(`http://${domain}/v1/models`)
    ElMessage.success(`域名 ${domain} 测试成功`)
  } catch (error: any) {
    ElMessage.error(`测试失败: ${error.message}`)
  } finally {
    testingDomains.value[domain] = false
  }
}

// 下载CA证书
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
    ElMessage.success('CA证书下载成功')
  } catch (error: any) {
    ElMessage.error('下载失败: ' + error.message)
  }
}

// 复制命令
const copyCommand = async (command: string) => {
  try {
    await navigator.clipboard.writeText(command)
    ElMessage.success('已复制到剪贴板')
  } catch (error) {
    ElMessage.error('复制失败')
  }
}

// 复制证书安装命令
const copyCertCommand = async () => {
  const commands = [
    '# 下载CA证书',
    'curl -o ca.crt http://127.0.0.1:20060/api/v1/host-proxy/ca-cert',
    '',
    '# 安装到系统(Linux/Mac)',
    'sudo cp ca.crt /usr/local/share/ca-certificates/ca.crt',
    'sudo update-ca-certificates',
    '',
    '# Windows: 双击ca.crt证书,安装到"受信任的根证书颁发机构"'
  ].join('\n')
  await copyCommand(commands)
}

// 切换指南显示
const toggleGuide = () => {
  showGuide.value = !showGuide.value
}

// 打开配置向导
const openSetupWizard = () => {
  wizardStep.value = 0
  showWizard.value = true
}

// 测试代理
const testProxy = async () => {
  try {
    await api.get('https://api.openai.com/v1/models')
    ElMessage.success('代理测试成功!')
  } catch (error: any) {
    ElMessage.error(`测试失败: ${error.message}`)
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

/* 统计卡片 */
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

/* 卡片样式 */
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

/* CA证书 */
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

/* 配置指南 */
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

/* 工具类 */
.mt-lg {
  margin-top: 24px;
}

.mt-md {
  margin-top: 16px;
}

.mb-md {
  margin-bottom: 16px;
}

/* 响应式 */
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

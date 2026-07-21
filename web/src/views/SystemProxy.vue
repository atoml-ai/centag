<template>
  <div class="system-proxy">
    <div class="header-with-toolbar">
      <div class="header-left">
        <h1 class="page-title">本机代理出口</h1>
        <p class="page-description">
          将 Agent 的大模型流量导入 Centag。推荐用进程级代理（wrap run）；认系统代理的客户端再用 PAC。
        </p>
      </div>
      <div class="toolbar-actions">
        <el-button :loading="loading" @click="load">
          <el-icon><Refresh /></el-icon>
          刷新
        </el-button>
      </div>
    </div>

    <!-- 顶部状态条：一眼看是否正常 -->
    <div class="status-strip">
      <div class="status-item" :class="status.enabled ? 'ok' : 'warn'">
        <span class="status-label">MITM</span>
        <span class="status-value">{{ status.enabled ? '运行中' : '未启动' }}</span>
      </div>
      <div class="status-item" :class="egressConfigured ? 'ok' : 'warn'">
        <span class="status-label">出口 Key</span>
        <span class="status-value">{{ egressConfigured ? '已配置' : '未配置' }}</span>
      </div>
      <div class="status-item">
        <span class="status-label">监听</span>
        <span class="status-value">{{ listenDisplay }}</span>
      </div>
      <div class="status-item">
        <span class="status-label">PAC 域名</span>
        <span class="status-value">{{ domainCount }}</span>
      </div>
      <div class="status-item">
        <span class="status-label">路径模式</span>
        <span class="status-value">{{ patternCount }}</span>
      </div>
    </div>

    <el-tabs v-model="mainTab" class="main-tabs">
      <!-- ========== Tab 1: 配置向导 ========== -->
      <el-tab-pane label="配置向导" name="wizard">
        <el-alert
          class="mb-md"
          type="info"
          :closable="false"
          show-icon
          title="按下列 4 步完成即可。多数 CLI（如 OpenCode）用第 3 步的 wrap run，不必改系统 PAC，也别把代理写进全局 shell。"
        />
        <el-alert
          v-if="setupStatus?.in_container"
          class="mb-md"
          type="warning"
          :closable="false"
          show-icon
          title="当前 Centag 运行在 Docker：宿主机必须映射 8081，且容器内 MITM 监听 0.0.0.0。请用 ./start.sh docker run personal（已含 -p 8081:8081）重启后再测。"
        />

        <el-alert class="mb-md" type="success" :closable="false" show-icon>
          <template #title>Key 策略（三层，不要混）</template>
          <ul class="key-policy-list">
            <li>
              <strong>Centag 出口 Key（llmproxy_*）</strong>：只在本页「一键绑定」配置。
              MITM 转发时自动注入；OpenCode / Agent <em>不要</em>填这个。
            </li>
            <li>
              <strong>上游 Provider Key</strong>：在 Web「后端 / Provider」里配置（DeepSeek、百炼等真实密钥）。
              Centag 用它去调大模型。personal 首启为空，必须先加至少一个。
            </li>
            <li>
              <strong>Agent 里的 Key</strong>：可填任意占位或原厂 Token；走 wrap 时会被 MITM 换成出口 Key。
              报「无效的 API key」通常是出口 Key 未绑定；报无后端则是 Provider 未配。
            </li>
          </ul>
        </el-alert>

        <el-steps :active="wizardProgress" align-center finish-status="success" class="mb-lg">
          <el-step title="出口 Key" :description="egressConfigured ? '已就绪' : '待绑定'" />
          <el-step title="启动 MITM" :description="status.enabled ? '运行中' : '未启动'" />
          <el-step title="接入 Agent" description="复制命令运行" />
          <el-step title="验证" description="确认正常" />
        </el-steps>

        <!-- 步骤 1 -->
        <el-card class="step-card" shadow="never">
          <div class="step-head">
            <span class="step-num">1</span>
            <div class="step-title-block">
              <h3>绑定 Centag 出口 Key（仅服务端）</h3>
              <p>
                位置：本页。生成/绑定名为 system-proxy-egress 的 llmproxy_* Key，由 MITM 注入到
                :20060。不要写进 OpenCode 配置。
              </p>
            </div>
            <el-tag :type="egressConfigured ? 'success' : 'danger'" size="small">
              {{ egressConfigured ? '正常：已配置' : '异常：未配置' }}
            </el-tag>
          </div>
          <el-space wrap>
            <el-button type="primary" :loading="ensuringEgress" @click="ensureEgressKey">
              一键绑定出口 Key
            </el-button>
            <el-select
              v-model="selectedEgressKeyId"
              clearable
              filterable
              placeholder="或选择已有 Key"
              style="width: 220px"
              :loading="loadingKeys"
            >
              <el-option
                v-for="k in apiKeyOptions"
                :key="k.id"
                :label="`${k.name} (${k.key_prefix}…)`"
                :value="k.id"
              />
            </el-select>
            <el-button :disabled="!selectedEgressKeyId" :loading="bindingEgress" @click="bindSelectedEgressKey">
              绑定所选
            </el-button>
          </el-space>
        </el-card>

        <!-- 步骤 2 -->
        <el-card class="step-card" shadow="never">
          <div class="step-head">
            <span class="step-num">2</span>
            <div class="step-title-block">
              <h3>启动 MITM 服务</h3>
              <p>默认只监听本机 127.0.0.1。开启后即可被代理流量命中。</p>
            </div>
            <el-tag :type="status.enabled ? 'success' : 'info'" size="small">
              {{ status.enabled ? '正常：运行中' : '待启动' }}
            </el-tag>
          </div>

          <div class="step-row">
            <span class="step-row-label">MITM 服务</span>
            <el-switch
              v-model="status.enabled"
              :loading="toggling"
              @change="toggleProxy"
              active-text="已开"
              inactive-text="已关"
            />
            <span class="form-hint">监听 {{ listenDisplay }}</span>
          </div>

          <div class="step-row">
            <span class="step-row-label">允许局域网访问</span>
            <el-switch v-model="allowLanClients" :loading="savingLan" @change="onAllowLanChange" />
            <span class="form-hint">
              关闭 = 仅本机（127.0.0.1）；开启 = 监听 0.0.0.0，同网段其它设备可连
            </span>
          </div>

          <template v-if="allowLanClients">
            <el-form label-width="120px" class="lan-form">
              <el-form-item label="本机局域网 IP" required>
                <el-input v-model="advertiseHost" placeholder="如 192.168.1.50" style="max-width: 240px" />
                <el-button class="ml-sm" :loading="detectingIP" @click="detectLanIP">探测</el-button>
                <el-button type="primary" class="ml-sm" :loading="savingLan" @click="saveLanConfig">保存</el-button>
                <div v-if="suggestedLanHosts.length" class="form-hint lan-suggest">
                  可选：
                  <el-button
                    v-for="ip in suggestedLanHosts"
                    :key="ip"
                    link
                    type="primary"
                    @click="pickLanHost(ip)"
                  >
                    {{ ip }}
                  </el-button>
                </div>
              </el-form-item>
              <el-form-item label="对外访问地址">
                <el-input v-model="employeeServer" placeholder="http://192.168.1.50:20060" style="max-width: 320px" />
              </el-form-item>
            </el-form>
            <el-alert type="warning" :closable="false" show-icon class="mb-sm">
              必须填写本机局域网 IP 后再保存。仅在可信内网开启，并放行防火墙端口 {{ apiPort }} / {{ listenPort }}。
            </el-alert>
          </template>
        </el-card>

        <!-- 步骤 3 -->
        <el-card class="step-card" shadow="never">
          <div class="step-head">
            <span class="step-num">3</span>
            <div class="step-title-block">
              <h3>接入 Agent（推荐进程代理）</h3>
              <p>一条命令自动处理 CA + HTTPS_PROXY，启动你的 Agent。多数 CLI 到这一步就够了。</p>
            </div>
            <el-tag type="success" size="small">推荐</el-tag>
          </div>

          <div class="cmd-block">
            <code>{{ runCommand }}</code>
            <el-button type="primary" size="small" @click="copyRunCmd">复制命令</el-button>
          </div>

          <el-collapse class="optional-collapse">
            <el-collapse-item title="可选：系统 PAC（仅认系统代理的桌面客户端）" name="pac">
              <p class="mb-sm form-hint">
                OpenCode 等多数 CLI 不读系统 PAC。PAC URL：{{ apiPACURL }}
              </p>
              <el-space wrap>
                <el-button @click="copyProxyctlCmd('enable')">复制：PAC 启用</el-button>
                <el-button type="danger" plain @click="copyProxyctlCmd('disable')">复制：停用并恢复</el-button>
                <el-button @click="copyEnvProxyCmd">复制：手写 HTTPS_PROXY</el-button>
              </el-space>
            </el-collapse-item>
          </el-collapse>
        </el-card>

        <!-- 步骤 4 -->
        <el-card class="step-card" shadow="never">
          <div class="step-head">
            <span class="step-num">4</span>
            <div class="step-title-block">
              <h3>验证是否正常</h3>
              <p>下列全部满足即为配置成功。</p>
            </div>
          </div>
          <ul class="check-list">
            <li :class="egressConfigured ? 'pass' : 'fail'">
              出口 Key {{ egressConfigured ? '已配置' : '未配置' }}（否则 Agent 会看到「无效的 API key」）
            </li>
            <li :class="status.enabled ? 'pass' : 'fail'">
              MITM {{ status.enabled ? '运行中' : '未启动' }}
            </li>
            <li class="info">Web「后端 / Provider」至少有一个已启用且带有效上游 Key（否则 503 无可用后端）</li>
            <li class="info">Agent 用 wrap run 启动后，请求出现在 Centag 日志</li>
            <li class="info">证书报错 → 到「其它」页下载并信任 CA</li>
          </ul>
          <el-space wrap>
            <el-button @click="copyProxyctlCmd('doctor')">复制：诊断命令</el-button>
            <el-button type="primary" :loading="testing" @click="testProxy">立即测试</el-button>
          </el-space>
          <el-alert
            v-if="testResult"
            class="mt-md"
            :type="testResult.ok ? 'success' : 'error'"
            :closable="false"
            show-icon
            :title="testResult.title"
          >
            <ul class="check-list" style="margin-bottom: 0">
              <li v-for="(line, i) in testResult.lines" :key="i" :class="line.ok ? 'pass' : 'fail'">
                {{ line.text }}
              </li>
            </ul>
          </el-alert>
          <p class="form-hint mt-md">
            说明：页面测试检查 MITM/PAC/出口 Key/CA 是否就绪，不会从浏览器直连 openai.com（那会 CORS/超时）。
            端到端连通请用上方「复制：诊断命令」或 wrap run 实测。
          </p>
        </el-card>
      </el-tab-pane>

      <!-- ========== Tab 2: 域名与路径 ========== -->
      <el-tab-pane label="域名与路径" name="rules">
        <el-card class="pac-card" shadow="never">
          <template #header>
            <div class="card-header">
              <span class="card-title">PAC 域名白名单</span>
              <div>
                <el-button type="primary" link @click="showPACPreview">
                  <el-icon><View /></el-icon>
                  查看 PAC
                </el-button>
                <el-button type="primary" link @click="downloadPAC">
                  <el-icon><Download /></el-icon>
                  下载 PAC
                </el-button>
                <el-button :loading="ensuringDefaults" @click="ensureDefaultRules">
                  补全默认列表
                </el-button>
                <el-button type="primary" @click="openAddDomain" :disabled="!status.enabled">
                  <el-icon><Plus /></el-icon>
                  添加域名
                </el-button>
              </div>
            </div>
          </template>
          <el-alert type="info" :closable="false" class="mb-md" show-icon>
            仅白名单内域名会走代理。初始化列表已覆盖 Provider 目录中的大模型服务域名，可按需增删。
          </el-alert>
          <el-table :data="domainList" stripe v-loading="loading" max-height="420">
            <el-table-column prop="domain" label="域名" min-width="220" />
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
              <template #default="{ row }">
                <el-button type="danger" link @click="removeDomain(row.domain)" :disabled="!status.enabled">
                  <el-icon><Delete /></el-icon>
                  删除
                </el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>

        <el-card class="pattern-card mt-md" shadow="never">
          <template #header>
            <div class="card-header">
              <span class="card-title">路径模式</span>
              <el-button type="primary" @click="openAddPattern" :disabled="!status.enabled">
                <el-icon><Plus /></el-icon>
                添加路径模式
              </el-button>
            </div>
          </template>
          <el-alert type="info" :closable="false" class="mb-md" show-icon>
            域名须在白名单。命中路径后会改写到 Centag 的 /v1/*；白名单域名上识别到常见 LLM 路径也会转发。
          </el-alert>
          <el-table :data="patternList" stripe v-loading="loading" max-height="320">
            <el-table-column prop="pattern" label="路径模式" min-width="220" />
            <el-table-column label="操作" width="120" align="center">
              <template #default="{ row }">
                <el-button type="danger" link @click="removePattern(row.pattern)" :disabled="!status.enabled">
                  <el-icon><Delete /></el-icon>
                  删除
                </el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-tab-pane>

      <!-- ========== Tab 3: 其它 ========== -->
      <el-tab-pane label="其它" name="advanced">
        <el-card class="control-card" shadow="never">
          <template #header>
            <span class="card-title">MITM 高级选项</span>
          </template>
          <el-form :model="config" label-width="120px">
            <el-form-item label="监听端口">
              <el-input-number
                v-model="listenPort"
                :min="1024"
                :max="65535"
                :disabled="status.enabled"
                controls-position="right"
              />
              <span class="form-hint">修改后需关闭再开启 MITM</span>
            </el-form-item>
            <el-form-item label="代理模式">
              <el-select
                v-model="status.pac_enabled"
                :disabled="status.enabled"
                style="max-width: 360px"
                @change="onPacModeChange"
              >
                <el-option label="PAC 模式（仅代理指定域名，推荐）" :value="true" />
                <el-option label="全局模式（代理所有流量）" :value="false" />
              </el-select>
            </el-form-item>
            <el-form-item label="Listen Addr" v-if="allowLanClients">
              <el-input v-model="listenAddr" placeholder="0.0.0.0" style="max-width: 240px" />
              <el-button type="primary" class="ml-sm" :loading="savingLan" @click="saveLanConfig">保存</el-button>
            </el-form-item>
          </el-form>
          <el-descriptions :column="2" size="small" border>
            <el-descriptions-item label="PAC URL">{{ apiPACURL }}</el-descriptions-item>
            <el-descriptions-item label="MITM PROXY">
              {{ setupStatus?.mitm_proxy || `http://127.0.0.1:${listenPort}` }}
            </el-descriptions-item>
            <el-descriptions-item label="CA 指纹">
              {{ setupStatus?.ca_fingerprint_sha256 || '（启 MITM 后生成）' }}
            </el-descriptions-item>
            <el-descriptions-item label="Loopback">
              {{ setupStatus?.listen_is_loopback !== false ? '是' : '否' }}
            </el-descriptions-item>
          </el-descriptions>
        </el-card>

        <el-card class="cert-card mt-md" shadow="never">
          <template #header>
            <span class="card-title">CA 证书</span>
          </template>
          <el-alert title="HTTPS 需要信任本 CA，否则会证书错误" type="warning" :closable="false" show-icon class="mb-md" />
          <el-space wrap class="mb-md">
            <el-button type="primary" @click="downloadCACert">
              <el-icon><Download /></el-icon>
              下载 CA 证书
            </el-button>
            <el-button @click="copyCertCommand">
              <el-icon><DocumentCopy /></el-icon>
              复制安装命令
            </el-button>
          </el-space>
          <el-descriptions :column="2" border size="small">
            <el-descriptions-item label="证书状态">
              <el-tag :type="certInfo.valid ? 'success' : 'danger'" size="small">
                {{ certInfo.valid ? '有效' : '无效' }}
              </el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="颁发者">{{ certInfo.issuer }}</el-descriptions-item>
            <el-descriptions-item label="有效期至">{{ certInfo.expires }}</el-descriptions-item>
            <el-descriptions-item label="剩余天数">{{ certInfo.daysLeft }} 天</el-descriptions-item>
          </el-descriptions>
        </el-card>
      </el-tab-pane>
    </el-tabs>

    <!-- 添加域名 -->
    <el-dialog v-model="showAddDialog" title="添加域名" width="500px">
      <el-form :model="newDomain" label-width="100px">
        <el-form-item label="域名">
          <el-input v-model="newDomain.domain" placeholder="例如: api.openai.com" />
          <el-alert type="warning" :closable="false" class="mt-md">
            只输入域名，不要包含协议或路径
          </el-alert>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showAddDialog = false">取消</el-button>
        <el-button type="primary" @click="addDomain" :loading="adding">添加</el-button>
      </template>
    </el-dialog>

    <!-- 添加路径模式 -->
    <el-dialog v-model="showAddPatternDialog" title="添加路径模式" width="500px">
      <el-form :model="newPattern" label-width="100px">
        <el-form-item label="路径模式">
          <el-input v-model="newPattern.pattern" placeholder="例如: /v1 或 /api/paas/v4" />
          <el-alert type="info" :closable="false" class="mt-md">
            常见：/v1、/v2、/openai/v1、/api/paas/v4、/zen/v1
          </el-alert>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showAddPatternDialog = false">取消</el-button>
        <el-button type="primary" @click="addPattern" :loading="addingPattern">添加</el-button>
      </template>
    </el-dialog>

    <!-- PAC 预览 -->
    <el-dialog v-model="showPACDialog" title="PAC 文件预览" width="800px">
      <el-input type="textarea" :rows="20" v-model="pacContent" readonly class="pac-preview" />
      <template #footer>
        <el-button @click="showPACDialog = false">关闭</el-button>
        <el-button type="primary" @click="downloadPAC">
          <el-icon><Download /></el-icon>
          下载
        </el-button>
        <el-button @click="copyPACContent">
          <el-icon><DocumentCopy /></el-icon>
          复制
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
  Plus,
  Position,
  Delete,
  Download,
  DocumentCopy,
  View
} from '@element-plus/icons-vue'
import api from '@/api'
import { listAPIKeys, type APIKey } from '@/api/user'
import {
  bindEgressAPIKey,
  ensureEgressAPIKey,
  getProxySetupStatus,
  type ProxySetupStatus
} from '@/api/system-proxy'

interface SystemProxyStatus {
  enabled: boolean
  pac_enabled: boolean
  pac_domains: string[]
  pac_patterns: string[]
}

interface CertInfo {
  valid: boolean
  issuer: string
  expires: string
  daysLeft: number
}

const loading = ref(false)
const toggling = ref(false)
const testing = ref(false)
const testResult = ref<{ ok: boolean; title: string; lines: { ok: boolean; text: string }[] } | null>(null)
const savingLan = ref(false)
const detectingIP = ref(false)
const ensuringEgress = ref(false)
const bindingEgress = ref(false)
const loadingKeys = ref(false)
const mainTab = ref('wizard')
const setupStatus = ref<ProxySetupStatus | null>(null)
const allowLanClients = ref(false)
const advertiseHost = ref('')
const suggestedLanHosts = ref<string[]>([])
const listenAddr = ref('127.0.0.1')
const employeeServer = ref('')
const selectedEgressKeyId = ref<number | undefined>()
const apiKeyOptions = ref<APIKey[]>([])
const egressConfigured = computed(() => !!setupStatus.value?.egress_api_key_configured)
const status = ref<SystemProxyStatus>({
  enabled: false,
  pac_enabled: true,
  pac_domains: [],
  pac_patterns: []
})
const certInfo = ref<CertInfo>({
  valid: true,
  issuer: 'Centag',
  expires: '2036-02-17',
  daysLeft: 3650
})
const testingDomains = ref<Record<string, boolean>>({})
const showAddDialog = ref(false)
const showAddPatternDialog = ref(false)
const showPACDialog = ref(false)
const adding = ref(false)
const addingPattern = ref(false)
const ensuringDefaults = ref(false)
const listenPort = ref(8081)
const apiPort = ref(20060)
const pacContent = ref('')
const config = ref({
  listen_port: 8081
})
const newDomain = ref({
  domain: ''
})
const newPattern = ref({
  pattern: ''
})

const domainCount = computed(() => status.value.pac_domains.length)
const patternCount = computed(() => status.value.pac_patterns.length)
const domainList = computed(() => status.value.pac_domains.map(domain => ({ domain })))
const patternList = computed(() => status.value.pac_patterns.map(pattern => ({ pattern })))

const listenDisplay = computed(() => {
  const addr = setupStatus.value?.listen_addr || `${listenAddr.value || '127.0.0.1'}:${listenPort.value}`
  return addr.includes(':') ? addr : `${addr}:${listenPort.value}`
})

const wizardProgress = computed(() => {
  if (!egressConfigured.value) return 0
  if (!status.value.enabled) return 1
  return 3
})

const apiPACURL = computed(() => {
  if (setupStatus.value?.pac_url) return setupStatus.value.pac_url
  return `http://127.0.0.1:${apiPort.value}/api/v1/proxy/pac`
})

const runCommand = computed(() => {
  if (allowLanClients.value) {
    return `${wrapBin()} run --server ${employeeAPIBase()} -- opencode`
  }
  return `${wrapBin()} run -- opencode`
})

function wrapBin() {
  // Prefer main-binary subcommand (personal ships wrap; no separate centag-wrap asset).
  return 'centag wrap'
}

function employeeAPIBase() {
  const base = (employeeServer.value || setupStatus.value?.pac_url || '').replace(/\/api\/v1\/proxy\/pac$/, '')
  return base || `http://127.0.0.1:${apiPort.value}`
}

function copyProxyctlCmd(kind: 'enable' | 'disable' | 'doctor') {
  const bin = wrapBin()
  let cmd =
    kind === 'enable' ? `${bin} enable` : kind === 'disable' ? `${bin} disable` : `${bin} doctor`
  if (allowLanClients.value) {
    const server = employeeAPIBase()
    if (kind === 'enable') cmd = `${bin} enable --server ${server}`
    else if (kind === 'doctor') cmd = `${bin} doctor --server ${server}`
  }
  copyCommand(cmd)
}

function copyEnvProxyCmd() {
  const host =
    allowLanClients.value && advertiseHost.value.trim()
      ? advertiseHost.value.trim()
      : '127.0.0.1'
  const port = listenPort.value || 8081
  const proxy = `http://${host}:${port}`
  copyCommand(
    [
      `# 仅本次终端 / 启动 Agent 时使用，不要写进 ~/.zshrc`,
      `export NO_PROXY=localhost,127.0.0.1,::1`,
      `export no_proxy="$NO_PROXY"`,
      `export HTTPS_PROXY=${proxy}`,
      `export HTTP_PROXY=${proxy}`,
      `export https_proxy="$HTTPS_PROXY"`,
      `export http_proxy="$HTTP_PROXY"`,
      `# 然后启动你的 Agent，例如：opencode`
    ].join('\n')
  )
}

function copyRunCmd() {
  copyCommand(runCommand.value)
}

async function loadAPIKeys() {
  loadingKeys.value = true
  try {
    apiKeyOptions.value = await listAPIKeys()
  } catch {
    apiKeyOptions.value = []
  } finally {
    loadingKeys.value = false
  }
}

async function ensureEgressKey() {
  ensuringEgress.value = true
  try {
    const res = await ensureEgressAPIKey()
    if (res.configured) {
      ElMessage.success(res.changed ? '出口 Key 已绑定（热生效）' : '出口 Key 已就绪')
    } else {
      ElMessage.warning('出口 Key 仍未配置，请检查 API Key 存储密钥或手动绑定')
    }
    await load()
  } catch (error: any) {
    ElMessage.error('绑定失败: ' + (error.message || error))
  } finally {
    ensuringEgress.value = false
  }
}

async function bindSelectedEgressKey() {
  if (!selectedEgressKeyId.value) return
  bindingEgress.value = true
  try {
    await bindEgressAPIKey(selectedEgressKeyId.value)
    ElMessage.success('已绑定所选出口 Key（热生效）')
    await load()
  } catch (error: any) {
    ElMessage.error('绑定失败: ' + (error.message || error))
  } finally {
    bindingEgress.value = false
  }
}

async function onAllowLanChange(val: boolean) {
  if (!val) {
    await saveLanConfig()
    return
  }
  try {
    await ElMessageBox.confirm(
      '开启后 MITM 将对局域网可达（监听 0.0.0.0），请仅在可信内网使用。是否继续？',
      '允许局域网访问',
      { type: 'warning', confirmButtonText: '确认开启', cancelButtonText: '取消' }
    )
    if (!listenAddr.value || listenAddr.value === '127.0.0.1') {
      listenAddr.value = '0.0.0.0'
    }
    if (!advertiseHost.value.trim()) {
      await detectLanIP()
    }
    // 探测失败时不要立刻保存：留在表单让用户填 IP / 点候选
    if (!advertiseHost.value.trim()) {
      ElMessage.warning('请选择或填写本机局域网 IP，再点「保存」')
      return
    }
    await saveLanConfig()
  } catch {
    allowLanClients.value = false
  }
}

async function onPacModeChange(val: boolean) {
  if (val) return
  try {
    await ElMessageBox.confirm(
      '全局模式会代理所有 HTTP/HTTPS 流量，可能影响其它应用。确定切换？',
      '全局模式警告',
      { type: 'warning' }
    )
  } catch {
    status.value.pac_enabled = true
  }
}

function pickLanHost(ip: string) {
  advertiseHost.value = ip
  employeeServer.value = `http://${ip}:${apiPort.value}`
}

async function detectLanIP() {
  detectingIP.value = true
  try {
    // 1) 浏览器地址栏若已是局域网 IP，直接用
    const host = window.location.hostname
    if (host && host !== 'localhost' && host !== '127.0.0.1' && host !== '[::1]') {
      pickLanHost(host)
      ElMessage.success(`已填入 ${host}`)
      return
    }

    // 2) 向服务端询问本机网卡 IP（localhost 打开页面时也能用）
    try {
      const st = await getProxySetupStatus()
      const list = st.suggested_lan_hosts || []
      suggestedLanHosts.value = list
      if (list.length === 1) {
        pickLanHost(list[0])
        ElMessage.success(`已填入 ${list[0]}`)
        return
      }
      if (list.length > 1) {
        pickLanHost(list[0])
        ElMessage.info(`已填入 ${list[0]}，如有多个网卡可点下方候选切换`)
        return
      }
    } catch {
      // ignore, fall through
    }

    ElMessage.warning('未能自动探测，请手动填写局域网 IP（如 192.168.x.x）')
  } finally {
    detectingIP.value = false
  }
}

async function saveLanConfig() {
  if (allowLanClients.value) {
    const host = advertiseHost.value.trim()
    if (!host) {
      ElMessage.warning('开启局域网访问时必须填写本机局域网 IP')
      return
    }
    if (host === 'localhost' || host === '127.0.0.1' || host === '::1') {
      ElMessage.warning('局域网 IP 不能是 127.0.0.1 / localhost')
      return
    }
  }

  savingLan.value = true
  try {
    await api.put('/api/v1/config', {
      system_proxy: {
        enabled: status.value.enabled,
        listen_port: listenPort.value,
        pac_enabled: status.value.pac_enabled,
        allow_lan_clients: allowLanClients.value,
        listen_addr: allowLanClients.value ? listenAddr.value || '0.0.0.0' : '127.0.0.1',
        advertise_host: allowLanClients.value ? advertiseHost.value.trim() : ''
      }
    })
    ElMessage.success(allowLanClients.value ? '局域网访问已开启' : '已恢复为仅本机访问')
    await load()
  } catch (error: any) {
    ElMessage.error('保存失败: ' + error.message)
    allowLanClients.value = !!setupStatus.value?.allow_lan_clients
  } finally {
    savingLan.value = false
  }
}

const load = async () => {
  loading.value = true
  try {
    const configData = await api.get('/api/v1/config')
    if (configData.system_proxy) {
      status.value.enabled = configData.system_proxy.enabled
      status.value.pac_enabled = configData.system_proxy.pac_enabled
      listenPort.value = configData.system_proxy.listen_port || 8081
      allowLanClients.value = !!configData.system_proxy.allow_lan_clients
      advertiseHost.value = configData.system_proxy.advertise_host || ''
      listenAddr.value = configData.system_proxy.listen_addr || '127.0.0.1'
    }
    if (configData.server?.port) {
      apiPort.value = configData.server.port
    }

    const proxyData = await api.get('/api/v1/proxy/status')
    status.value.pac_domains = proxyData.pac_domains || []
    status.value.pac_patterns = proxyData.pac_patterns || []

    try {
      setupStatus.value = await getProxySetupStatus()
      suggestedLanHosts.value = setupStatus.value?.suggested_lan_hosts || []
      if (setupStatus.value?.pac_url && !employeeServer.value) {
        employeeServer.value = setupStatus.value.pac_url.replace(/\/api\/v1\/proxy\/pac$/, '')
      }
    } catch {
      setupStatus.value = null
      suggestedLanHosts.value = []
    }
    void loadAPIKeys()
  } catch (error: any) {
    ElMessage.error('加载状态失败: ' + error.message)
  } finally {
    loading.value = false
  }
}

const loadDomains = async () => {
  try {
    const data = await api.get('/api/v1/proxy/domains')
    status.value.pac_domains = data.domains
  } catch (error: any) {
    console.error('加载域名失败:', error)
  }
}

const ensureDefaultRules = async () => {
  ensuringDefaults.value = true
  try {
    const res = await api.post('/api/v1/proxy/domains/ensure-defaults')
    const d = res.added_domains ?? 0
    const p = res.added_patterns ?? 0
    ElMessage.success(d + p === 0 ? '默认列表已是最新' : `已补全 ${d} 个域名、${p} 个路径`)
    await load()
  } catch (error: any) {
    ElMessage.error('补全失败: ' + (error.message || error))
  } finally {
    ensuringDefaults.value = false
  }
}

const toggleProxy = async () => {
  toggling.value = true
  try {
    await api.put('/api/v1/config', {
      system_proxy: {
        enabled: status.value.enabled,
        listen_port: listenPort.value,
        pac_enabled: status.value.pac_enabled,
        allow_lan_clients: allowLanClients.value,
        listen_addr: allowLanClients.value ? listenAddr.value || '0.0.0.0' : '127.0.0.1',
        advertise_host: allowLanClients.value ? advertiseHost.value : ''
      }
    })
    ElMessage.success(status.value.enabled ? 'MITM 服务已启用' : 'MITM 服务已禁用')
    await load()
  } catch (error: any) {
    ElMessage.error('操作失败: ' + error.message)
    status.value.enabled = !status.value.enabled
  } finally {
    toggling.value = false
  }
}

const openAddDomain = () => {
  newDomain.value = { domain: '' }
  showAddDialog.value = true
}

const addDomain = async () => {
  if (!newDomain.value.domain) {
    ElMessage.warning('请输入域名')
    return
  }
  adding.value = true
  try {
    await api.post('/api/v1/proxy/domains/add', { domain: newDomain.value.domain })
    ElMessage.success('域名添加成功')
    showAddDialog.value = false
    await loadDomains()
  } catch (error: any) {
    ElMessage.error('添加失败: ' + error.message)
  } finally {
    adding.value = false
  }
}

const removeDomain = async (domain: string) => {
  try {
    await ElMessageBox.confirm(`确定删除域名 ${domain} 吗?`, '确认删除', { type: 'warning' })
    await api.post('/api/v1/proxy/domains/remove', { domain })
    ElMessage.success('域名删除成功')
    await loadDomains()
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error('删除失败: ' + error.message)
    }
  }
}

const openAddPattern = () => {
  newPattern.value = { pattern: '' }
  showAddPatternDialog.value = true
}

const addPattern = async () => {
  if (!newPattern.value.pattern) {
    ElMessage.warning('请输入路径模式')
    return
  }
  let pattern = newPattern.value.pattern.trim()
  if (!pattern.startsWith('/')) {
    pattern = '/' + pattern
  }
  addingPattern.value = true
  try {
    await api.post('/api/v1/proxy/patterns/add', { pattern })
    ElMessage.success('路径模式添加成功')
    showAddPatternDialog.value = false
    await load()
  } catch (error: any) {
    ElMessage.error('添加失败: ' + error.message)
  } finally {
    addingPattern.value = false
  }
}

const removePattern = async (pattern: string) => {
  try {
    await ElMessageBox.confirm(`确定删除路径模式 ${pattern} 吗?`, '确认删除', { type: 'warning' })
    await api.post('/api/v1/proxy/patterns/remove', { pattern })
    ElMessage.success('路径模式删除成功')
    await load()
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error('删除失败: ' + error.message)
    }
  }
}

const testDomain = async (domain: string) => {
  testingDomains.value[domain] = true
  try {
    await api.get(`https://${domain}/v1/models`)
    ElMessage.success(`域名 ${domain} 测试成功`)
  } catch (error: any) {
    ElMessage.error(`测试失败: ${error.message}`)
  } finally {
    testingDomains.value[domain] = false
  }
}

const downloadCACert = async () => {
  try {
    const response = await fetch('/api/v1/proxy/ca.crt')
    const blob = await response.blob()
    const url = window.URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.setAttribute('download', 'centag-ca.crt')
    document.body.appendChild(link)
    link.click()
    link.remove()
    window.URL.revokeObjectURL(url)
    ElMessage.success('CA证书下载成功')
  } catch (error: any) {
    ElMessage.error('下载失败: ' + error.message)
  }
}

const copyCommand = async (command: string) => {
  try {
    await navigator.clipboard.writeText(command)
    ElMessage.success('已复制到剪贴板')
  } catch {
    ElMessage.error('复制失败')
  }
}

const copyCertCommand = async () => {
  const commands = [
    '# 下载CA证书',
    `curl -o centag-ca.crt http://127.0.0.1:${apiPort.value}/api/v1/proxy/ca.crt`,
    '',
    '# 安装到系统(Linux/Mac)',
    'sudo cp centag-ca.crt /usr/local/share/ca-certificates/centag-ca.crt',
    'sudo update-ca-certificates',
    '',
    '# Windows: 双击centag-ca.crt证书,安装到"受信任的根证书颁发机构"'
  ].join('\n')
  await copyCommand(commands)
}

const showPACPreview = async () => {
  try {
    const response = await fetch(apiPACURL.value)
    pacContent.value = await response.text()
    showPACDialog.value = true
  } catch (error: any) {
    ElMessage.error('获取PAC文件失败: ' + error.message)
  }
}

const downloadPAC = async () => {
  try {
    const response = await fetch(apiPACURL.value)
    const content = await response.text()
    const blob = new Blob([content], { type: 'application/x-ns-proxy-autoconfig' })
    const url = window.URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.setAttribute('download', 'proxy.pac')
    document.body.appendChild(link)
    link.click()
    link.remove()
    window.URL.revokeObjectURL(url)
    ElMessage.success('PAC文件下载成功')
  } catch (error: any) {
    ElMessage.error('下载失败: ' + error.message)
  }
}

const copyPACContent = async () => {
  await copyCommand(pacContent.value)
}

const testProxy = async () => {
  testing.value = true
  testResult.value = null
  const lines: { ok: boolean; text: string }[] = []
  try {
    await load()
    lines.push({
      ok: !!status.value.enabled,
      text: status.value.enabled ? 'MITM 服务运行中' : 'MITM 未启动（请先在步骤 2 开启）'
    })
    lines.push({
      ok: egressConfigured.value,
      text: egressConfigured.value ? '出口 Key 已配置' : '出口 Key 未配置（MITM 无法注入鉴权）'
    })

    try {
      const pacResp = await fetch(apiPACURL.value, { cache: 'no-store' })
      const pacText = pacResp.ok ? await pacResp.text() : ''
      const hasProxy = /PROXY\s+\S+:\d+/i.test(pacText)
      lines.push({
        ok: pacResp.ok && hasProxy,
        text: pacResp.ok && hasProxy
          ? `PAC 可访问（${pacText.length} bytes，含 PROXY）`
          : `PAC 异常（HTTP ${pacResp.status}）`
      })
    } catch (e: any) {
      lines.push({ ok: false, text: `PAC 拉取失败: ${e.message || e}` })
    }

    try {
      const caResp = await fetch('/api/v1/proxy/ca.crt', { cache: 'no-store' })
      lines.push({
        ok: caResp.ok,
        text: caResp.ok ? 'CA 证书可下载' : `CA 下载失败（HTTP ${caResp.status}）`
      })
    } catch (e: any) {
      lines.push({ ok: false, text: `CA 下载失败: ${e.message || e}` })
    }

    if (setupStatus.value?.in_container) {
      lines.push({
        ok: true,
        text: 'Docker 部署：请确认宿主机已映射 8081（./start.sh docker run personal）'
      })
    }

    const ok = lines.every(l => l.ok)
    testResult.value = {
      ok,
      title: ok ? '本机代理出口就绪（页面自检通过）' : '自检未通过，请按下列项处理',
      lines
    }
    if (ok) ElMessage.success('自检通过')
    else ElMessage.warning('自检未全部通过')
  } catch (error: any) {
    testResult.value = {
      ok: false,
      title: '自检失败',
      lines: [{ ok: false, text: error.message || String(error) }]
    }
    ElMessage.error(`测试失败: ${error.message || error}`)
  } finally {
    testing.value = false
  }
}

onMounted(() => {
  load()
})
</script>

<style scoped>
.system-proxy {
  min-height: 100vh;
}

.header-with-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 16px;
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

.status-strip {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-bottom: 16px;
}

.status-item {
  display: flex;
  align-items: baseline;
  gap: 8px;
  padding: 8px 14px;
  border-radius: 8px;
  border: 1px solid var(--el-border-color);
  background: var(--el-bg-color);
  min-width: 120px;
}

.status-item.ok {
  border-color: var(--el-color-success-light-5);
  background: var(--el-color-success-light-9);
}

.status-item.warn {
  border-color: var(--el-color-warning-light-5);
  background: var(--el-color-warning-light-9);
}

.status-label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.status-value {
  font-size: 14px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.main-tabs :deep(.el-tabs__content) {
  padding-top: 8px;
}

.step-card {
  margin-bottom: 12px;
  border-radius: 8px;
}

.step-head {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  margin-bottom: 16px;
}

.step-num {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  background: var(--el-color-primary);
  color: #fff;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-weight: 700;
  flex-shrink: 0;
}

.step-title-block {
  flex: 1;
  min-width: 0;
}

.step-title-block h3 {
  margin: 0 0 4px;
  font-size: 16px;
  font-weight: 600;
}

.step-title-block p {
  margin: 0;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.step-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
  margin-bottom: 12px;
}

.step-row-label {
  width: 120px;
  font-size: 14px;
  color: var(--el-text-color-regular);
}

.lan-form {
  margin-top: 8px;
}

.cmd-block {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  padding: 12px 14px;
  border-radius: 8px;
  background: var(--el-fill-color-light);
  margin-bottom: 12px;
}

.cmd-block code {
  flex: 1;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 13px;
  word-break: break-all;
}

.optional-collapse {
  border: none;
}

.check-list {
  margin: 0 0 16px;
  padding-left: 20px;
}

.check-list li {
  margin-bottom: 6px;
  font-size: 14px;
}

.check-list li.pass {
  color: var(--el-color-success);
}

.check-list li.fail {
  color: var(--el-color-danger);
}

.check-list li.info {
  color: var(--el-text-color-secondary);
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

.pac-preview {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
}

.mt-md {
  margin-top: 16px;
}

.mb-md {
  margin-bottom: 16px;
}

.mb-lg {
  margin-bottom: 24px;
}

.mb-sm {
  margin-bottom: 8px;
}

.ml-sm {
  margin-left: 8px;
}

.form-hint {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

.lan-suggest {
  width: 100%;
  margin-top: 6px;
  margin-left: 0;
}

.key-policy-list {
  margin: 8px 0 0;
  padding-left: 18px;
  font-size: 13px;
  line-height: 1.55;
}

.key-policy-list li {
  margin-bottom: 4px;
}

@media (max-width: 768px) {
  .header-with-toolbar {
    flex-direction: column;
  }

  .step-row-label {
    width: 100%;
  }
}
</style>

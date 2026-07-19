<template>
  <div class="system-proxy">
    <div class="header-with-toolbar">
      <div class="header-left">
        <h1 class="page-title">本机代理出口</h1>
        <p class="page-description">将 Agent 的大模型流量导入 Centag：多数 CLI 用进程级 HTTPS_PROXY；认系统代理的客户端再用 PAC。均不改 Agent 内模型配置</p>
      </div>
      <div class="toolbar-actions">
        <el-button :loading="loading" @click="load">
          <el-icon><Refresh /></el-icon>
          刷新
        </el-button>
        <el-button @click="openSetupWizard">
          <el-icon><MagicStick /></el-icon>
          手动排查向导
        </el-button>
      </div>
    </div>

    <div class="content-wrapper">
      <!-- 一键主路径 -->
      <el-card class="control-card hero-card">
        <template #header>
          <div class="card-header">
            <span class="card-title">一键配置（推荐）</span>
            <el-tag size="small" :type="setupStatus?.mode === 'lan' ? 'warning' : 'success'">
              {{ setupStatus?.mode === 'lan' ? '团队局域网出口' : '本机模式' }}
            </el-tag>
          </div>
        </template>
        <el-radio-group v-model="uiMode" class="mb-md">
          <el-radio-button value="local">本机 Centag</el-radio-button>
          <el-radio-button value="team">团队服务器</el-radio-button>
        </el-radio-group>
        <el-alert
          class="mb-md"
          type="info"
          :closable="false"
          show-icon
          title="多数 Agent（如 OpenCode）不读系统 PAC：请只在启动该进程时设置 HTTPS_PROXY=http://127.0.0.1:8081（勿写进全局 shell）。非白名单域名由 MITM 隧道直通，不干扰其它上网。"
        />
        <el-alert
          class="mb-md"
          type="warning"
          :closable="false"
          show-icon
          title="MITM 服务开启 ≠ 本机系统代理已写入。认 PAC 的客户端请用下方 proxyctl；仅用环境变量时不必 enable 系统代理。"
        />

        <template v-if="uiMode === 'local'">
          <p class="mb-md">本机 Centag：MITM 默认仅监听 127.0.0.1。进程代理指向 <code>http://127.0.0.1:8081</code>；系统 PAC 仅代理白名单 LLM 域名。</p>
          <el-space wrap class="mb-md">
            <el-button @click="copyEnvProxyCmd">复制：进程 HTTPS_PROXY 启动示例</el-button>
          </el-space>
          <el-space wrap>
            <el-button type="primary" @click="copyProxyctlCmd('enable')">复制：一键启用</el-button>
            <el-button type="danger" plain @click="copyProxyctlCmd('disable')">复制：一键停用并恢复</el-button>
            <el-button @click="copyProxyctlCmd('doctor')">复制：诊断</el-button>
          </el-space>
        </template>

        <template v-else>
          <el-form label-width="140px" class="mb-md">
            <el-form-item label="允许局域网客户端">
              <el-switch v-model="allowLanClients" :loading="savingLan" @change="onAllowLanChange" />
              <span class="form-hint">Team 主场景：员工电脑 PAC 指向本机 advertise 地址</span>
            </el-form-item>
            <el-form-item label="Advertise Host" v-if="allowLanClients">
              <el-input v-model="advertiseHost" placeholder="如 192.168.1.50" style="max-width: 280px" />
              <el-button class="ml-sm" @click="detectLanIP" :loading="detectingIP">探测局域网 IP</el-button>
              <el-button type="primary" class="ml-sm" :loading="savingLan" @click="saveLanConfig">保存</el-button>
            </el-form-item>
            <el-form-item label="Listen Addr" v-if="allowLanClients">
              <el-input v-model="listenAddr" placeholder="0.0.0.0" style="max-width: 280px" />
            </el-form-item>
          </el-form>
          <el-descriptions v-if="setupStatus" :column="1" border size="small" class="mb-md">
            <el-descriptions-item label="员工 PAC URL">{{ setupStatus.pac_url }}</el-descriptions-item>
            <el-descriptions-item label="CA 下载">{{ setupStatus.ca_download_url }}</el-descriptions-item>
            <el-descriptions-item label="CA 指纹">{{ setupStatus.ca_fingerprint_sha256 || '（启 MITM 后生成）' }}</el-descriptions-item>
            <el-descriptions-item label="MITM PROXY">{{ setupStatus.mitm_proxy }}</el-descriptions-item>
          </el-descriptions>
          <el-form label-width="140px" class="mb-md">
            <el-form-item label="员工连接 API">
              <el-input v-model="employeeServer" placeholder="http://192.168.1.50:20060" style="max-width: 360px" />
              <el-button class="ml-sm" type="primary" @click="copyEmployeeEnable">复制员工启用命令</el-button>
            </el-form-item>
          </el-form>
          <el-alert type="warning" :closable="false" show-icon title="员工 disable 只恢复自己电脑，不会关闭服务器 MITM。" />
        </template>

        <el-divider />
        <el-descriptions :column="2" size="small" border>
          <el-descriptions-item label="MITM 服务">{{ setupStatus?.mitm_enabled || status.enabled ? '运行中' : '未运行' }}</el-descriptions-item>
          <el-descriptions-item label="监听">{{ setupStatus?.listen_addr || `${listenAddr || '127.0.0.1'}:${listenPort}` }}</el-descriptions-item>
          <el-descriptions-item label="PAC URL">{{ apiPACURL }}</el-descriptions-item>
          <el-descriptions-item label="Loopback">{{ setupStatus?.listen_is_loopback !== false ? '是' : '否' }}</el-descriptions-item>
        </el-descriptions>
      </el-card>

      <!-- 状态卡片 -->
      <el-row :gutter="12" class="metrics-grid">
        <el-col :xs="12" :sm="8" :md="8" :lg="4">
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
        <el-col :xs="12" :sm="8" :md="8" :lg="4">
          <div class="metric-card" :class="{ 'pac-active': status.pac_enabled, 'pac-inactive': !status.pac_enabled }">
            <div class="metric-icon" :class="status.pac_enabled ? 'pac-active-icon' : 'pac-inactive-icon'">
              <el-icon :size="22"><DocumentChecked /></el-icon>
            </div>
            <div class="metric-content">
              <div class="metric-value">{{ status.pac_enabled ? 'PAC模式' : '全局模式' }}</div>
              <div class="metric-label">代理模式</div>
            </div>
          </div>
        </el-col>
        <el-col :xs="12" :sm="8" :md="8" :lg="4">
          <div class="metric-card">
            <div class="metric-icon domain-icon">
              <el-icon :size="22"><Link /></el-icon>
            </div>
            <div class="metric-content">
              <div class="metric-value">{{ domainCount }}</div>
              <div class="metric-label">PAC域名数</div>
            </div>
          </div>
        </el-col>
        <el-col :xs="12" :sm="8" :md="8" :lg="4">
          <div class="metric-card">
            <div class="metric-icon pattern-icon">
              <el-icon :size="22"><FolderOpened /></el-icon>
            </div>
            <div class="metric-content">
              <div class="metric-value">{{ patternCount }}</div>
              <div class="metric-label">路径模式数</div>
            </div>
          </div>
        </el-col>
        <el-col :xs="12" :sm="8" :md="8" :lg="4">
          <div class="metric-card">
            <div class="metric-icon port-icon">
              <el-icon :size="22"><Operation /></el-icon>
            </div>
            <div class="metric-content">
              <div class="metric-value">{{ listenPort }}</div>
              <div class="metric-label">监听端口</div>
            </div>
          </div>
        </el-col>
      </el-row>

      <!-- 高级：仅 MITM 服务 -->
      <el-card class="control-card">
        <template #header>
          <div class="card-header">
            <span class="card-title">高级：MITM 服务（非系统代理写入）</span>
            <el-switch
              v-model="status.enabled"
              :loading="toggling"
              @change="toggleProxy"
              active-text="服务已开"
              inactive-text="服务已关"
            />
          </div>
        </template>
        <div class="control-content">
          <el-form :model="config" label-width="120px">
            <el-row :gutter="16">
              <el-col :span="12">
                <el-form-item label="监听端口">
                  <el-input-number
                    v-model="listenPort"
                    :min="1024"
                    :max="65535"
                    :disabled="status.enabled"
                    controls-position="right"
                  />
                </el-form-item>
              </el-col>
              <el-col :span="12">
                <el-form-item label="代理模式">
                  <el-select v-model="status.pac_enabled" :disabled="status.enabled" style="width: 100%" @change="onPacModeChange">
                    <el-option label="PAC模式(仅代理指定域名)" :value="true" />
                    <el-option label="全局模式(代理所有流量)" :value="false" />
                  </el-select>
                </el-form-item>
              </el-col>
            </el-row>
          </el-form>
          <el-divider />
          <el-alert
            :title="status.enabled ? '系统代理已启用' : '系统代理已禁用'"
            :type="status.enabled ? 'success' : 'info'"
            :closable="false"
            show-icon
          >
            <template #default>
              <div v-if="status.enabled">
                <p>MITM代理服务器正在运行,监听端口: {{ listenPort }}</p>
                <p v-if="status.pac_enabled">当前为PAC模式,仅代理PAC文件中配置的域名</p>
                <p v-else>当前为全局模式,将代理所有HTTP/HTTPS流量</p>
              </div>
              <div v-else>
                <p>禁用后,代理服务器停止监听,所有流量将直连</p>
              </div>
            </template>
          </el-alert>
        </div>
      </el-card>

      <!-- PAC规则管理 -->
      <el-card class="pac-card">
        <template #header>
          <div class="card-header">
            <span class="card-title">PAC规则管理</span>
            <div>
              <el-button type="primary" link @click="showPACPreview">
                <el-icon><View /></el-icon>
                查看PAC文件
              </el-button>
              <el-button type="primary" link @click="downloadPAC">
                <el-icon><Download /></el-icon>
                下载PAC文件
              </el-button>
              <el-button type="primary" @click="openAddDomain" :disabled="!status.enabled">
                <el-icon><Plus /></el-icon>
                添加域名
              </el-button>
            </div>
          </div>
        </template>
        <el-table :data="domainList" stripe v-loading="loading" max-height="300">
          <el-table-column prop="domain" label="域名" min-width="200" />
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

      <!-- 路径模式管理 -->
      <el-card class="pattern-card">
        <template #header>
          <div class="card-header">
            <span class="card-title">路径模式管理</span>
            <el-button type="primary" @click="openAddPattern" :disabled="!status.enabled">
              <el-icon><Plus /></el-icon>
              添加路径模式
            </el-button>
          </div>
        </template>
        <el-alert type="info" :closable="false" class="mb-md">
          <template #default>
            <p>路径模式用于匹配需要代理的API路径。例如: /v1、/openai、/api（可选；白名单域名上识别到 /v1、/chat/completions、/responses 等也会转发）</p>
            <p>域名须在白名单；命中后会统一改写到 Centag 的 /v1/*，无需为每个 Agent 单独适配路径（变更立即同步 MITM）</p>
          </template>
        </el-alert>
        <el-table :data="patternList" stripe v-loading="loading" max-height="300">
          <el-table-column prop="pattern" label="路径模式" min-width="200" />
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
              <p>代理使用自签名证书拦截HTTPS流量,需要将CA证书安装到系统信任库。</p>
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
            <el-step title="启用代理" description="启动系统代理功能" />
            <el-step title="配置PAC" description="添加代理域名" />
            <el-step title="安装证书" description="信任CA证书" />
            <el-step title="配置系统" description="设置系统代理" />
            <el-step title="测试验证" description="验证代理工作" />
          </el-steps>

          <el-divider />

          <div class="step-details">
            <h3>步骤1: 下载CA证书</h3>
            <el-alert type="info" :closable="false" class="mb-md">
              <template #default>
                <p>首先下载CA证书,后续步骤需要安装到系统</p>
              </template>
            </el-alert>
            <el-input
              type="textarea"
              :rows="2"
              :model-value="`curl -o centag-ca.crt http://127.0.0.1:${apiPort}/api/v1/proxy/ca.crt`"
              readonly
              class="mb-md"
            />
            <el-button type="primary" @click="copyCommand(`curl -o centag-ca.crt http://127.0.0.1:${apiPort}/api/v1/proxy/ca.crt`)">
              <el-icon><DocumentCopy /></el-icon>
              复制下载命令
            </el-button>

            <el-divider />

            <h3>步骤2: 安装CA证书</h3>
            <el-alert type="warning" :closable="false" class="mb-md">
              <template #default>
                <p>将CA证书安装到系统信任库,否则HTTPS请求会报证书错误</p>
              </template>
            </el-alert>
            <div class="mt-md">
              <h4>Linux/Mac:</h4>
              <el-input
                type="textarea"
                :rows="2"
                model-value="sudo cp centag-ca.crt /usr/local/share/ca-certificates/centag-ca.crt && sudo update-ca-certificates"
                readonly
                class="mb-md"
              />
              <el-button @click="copyCommand('sudo cp centag-ca.crt /usr/local/share/ca-certificates/centag-ca.crt && sudo update-ca-certificates')">
                <el-icon><DocumentCopy /></el-icon>
                复制安装命令
              </el-button>
            </div>
            <div class="mt-md">
              <h4>Windows:</h4>
              <el-input
                type="textarea"
                :rows="2"
                model-value="双击 centag-ca.crt 文件,选择 安装到 受信任的根证书颁发机构"
                readonly
                class="mb-md"
              />
            </div>

            <el-divider />

            <h3>步骤3: 配置系统代理</h3>
            <el-alert type="info" :closable="false" class="mb-md">
              <template #default>
                <p>配置系统使用代理服务器</p>
              </template>
            </el-alert>
            <div class="mt-md">
              <h4>Linux:</h4>
              <el-input
                type="textarea"
                :rows="4"
                :model-value="`export http_proxy=http://127.0.0.1:${listenPort}
export https_proxy=http://127.0.0.1:${listenPort}
export no_proxy=localhost,127.0.0.1`"
                readonly
                class="mb-md"
              />
              <el-button @click="copyCommand(`export http_proxy=http://127.0.0.1:${listenPort}\nexport https_proxy=http://127.0.0.1:${listenPort}\nexport no_proxy=localhost,127.0.0.1`)">
                <el-icon><DocumentCopy /></el-icon>
                复制配置命令
              </el-button>
            </div>
            <div class="mt-md">
              <h4>Mac:</h4>
              <p class="mb-md">系统设置 → 网络 → 代理 → HTTP/HTTPS代理 → 输入: 127.0.0.1:{{ listenPort }}</p>
              <el-button @click="copyPACURL">
                <el-icon><DocumentCopy /></el-icon>
                复制PAC URL
              </el-button>
            </div>
            <div class="mt-md">
              <h4>Windows:</h4>
              <p class="mb-md">设置 → 网络和Internet → 代理 → 手动设置代理 → 输入: 127.0.0.1:{{ listenPort }}</p>
              <el-button @click="copyPACURL">
                <el-icon><DocumentCopy /></el-icon>
                复制PAC URL
              </el-button>
            </div>

            <el-divider />

            <h3>步骤4: 测试代理</h3>
            <el-alert type="success" :closable="false" class="mb-md">
              <template #default>
                <p>验证代理是否正常工作</p>
              </template>
            </el-alert>
            <el-input
              type="textarea"
              :rows="1"
              model-value="curl -x http://127.0.0.1:8081 https://api.openai.com/v1/models"
              readonly
              class="mb-md"
            />
            <el-button type="primary" @click="copyCommand('curl -x http://127.0.0.1:8081 https://api.openai.com/v1/models')">
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
          <el-alert type="warning" :closable="false" class="mt-md">
            <template #default>
              <p>注意: 只输入域名，不要包含协议(https://)或路径(/path)</p>
            </template>
          </el-alert>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showAddDialog = false">取消</el-button>
        <el-button type="primary" @click="addDomain" :loading="adding">添加</el-button>
      </template>
    </el-dialog>

    <!-- 添加路径模式对话框 -->
    <el-dialog v-model="showAddPatternDialog" title="添加路径模式" width="500px">
      <el-form :model="newPattern" label-width="100px">
        <el-form-item label="路径模式">
          <el-input v-model="newPattern.pattern" placeholder="例如: /v1 或 /openai" />
          <el-alert type="info" :closable="false" class="mt-md">
            <template #default>
              <p>路径模式用于匹配API路径前缀</p>
              <p>常见模式: /v1, /openai, /api, /v3</p>
            </template>
          </el-alert>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showAddPatternDialog = false">取消</el-button>
        <el-button type="primary" @click="addPattern" :loading="addingPattern">添加</el-button>
      </template>
    </el-dialog>

    <!-- PAC文件预览对话框 -->
    <el-dialog v-model="showPACDialog" title="PAC文件预览" width="800px">
      <el-input
        type="textarea"
        :rows="20"
        v-model="pacContent"
        readonly
        class="pac-preview"
      />
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

    <!-- 配置向导 -->
    <el-dialog v-model="showWizard" title="系统代理快速配置向导" width="600px">
      <el-steps :active="wizardStep" finish-status="success" align-center class="mb-lg">
        <el-step title="检查配置" />
        <el-step title="安装证书" />
        <el-step title="配置系统" />
        <el-step title="测试验证" />
        <el-step title="完成" />
      </el-steps>

      <div v-if="wizardStep === 0" class="wizard-step">
        <el-descriptions :column="1" border>
          <el-descriptions-item label="代理状态">
            <el-tag :type="status.enabled ? 'success' : 'info'">{{ status.enabled ? '已启用' : '已禁用' }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="代理模式">
            <el-tag type="primary">{{ status.pac_enabled ? 'PAC模式' : '全局模式' }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="监听端口">{{ listenPort }}</el-descriptions-item>
          <el-descriptions-item label="PAC域名数">{{ domainCount }}</el-descriptions-item>
        </el-descriptions>
        <el-alert type="info" class="mt-md" :closable="false">
          点击"下一步"开始配置CA证书安装
        </el-alert>
      </div>

      <div v-if="wizardStep === 1" class="wizard-step">
        <h3>安装CA证书</h3>
        <p class="mb-md">首先下载并安装CA证书到系统信任库</p>
        <el-input
          type="textarea"
          :rows="2"
          :model-value="`curl -o centag-ca.crt http://127.0.0.1:${apiPort}/api/v1/proxy/ca.crt`"
          readonly
          class="mb-md"
        />
        <el-button @click="downloadCACert">
          <el-icon><Download /></el-icon>
          下载CA证书
        </el-button>
        <el-button @click="copyCommand(`curl -o centag-ca.crt http://127.0.0.1:${apiPort}/api/v1/proxy/ca.crt`)">
          <el-icon><DocumentCopy /></el-icon>
          复制命令
        </el-button>
        <el-divider />
        <p class="text-warning">重要: 安装证书后需要重启浏览器或应用才能生效</p>
      </div>

      <div v-if="wizardStep === 2" class="wizard-step">
        <h3>配置系统代理</h3>
        <el-tabs v-model="activeTab" class="mb-md">
          <el-tab-pane label="Linux" name="linux">
            <el-input
              type="textarea"
              :rows="4"
              :model-value="`export http_proxy=http://127.0.0.1:${listenPort}\nexport https_proxy=http://127.0.0.1:${listenPort}\nexport no_proxy=localhost,127.0.0.1`"
              readonly
              class="mb-md"
            />
            <el-button @click="copyCommand(`export http_proxy=http://127.0.0.1:${listenPort}\nexport https_proxy=http://127.0.0.1:${listenPort}\nexport no_proxy=localhost,127.0.0.1`)">
              <el-icon><DocumentCopy /></el-icon>
              复制配置
            </el-button>
          </el-tab-pane>
          <el-tab-pane label="Mac" name="mac">
            <p class="mb-md">系统设置 → 网络 → 代理 → HTTP/HTTPS代理</p>
            <el-input
              type="textarea"
              :rows="2"
              :model-value="`地址: 127.0.0.1\n端口: ${listenPort}`"
              readonly
              class="mb-md"
            />
            <el-button @click="copyPACURL">
              <el-icon><DocumentCopy /></el-icon>
              复制PAC URL
            </el-button>
          </el-tab-pane>
          <el-tab-pane label="Windows" name="windows">
            <p class="mb-md">设置 → 网络和Internet → 代理 → 手动设置代理</p>
            <el-input
              type="textarea"
              :rows="2"
              :model-value="`地址: 127.0.0.1\n端口: ${listenPort}`"
              readonly
              class="mb-md"
            />
            <el-button @click="copyPACURL">
              <el-icon><DocumentCopy /></el-icon>
              复制PAC URL
            </el-button>
          </el-tab-pane>
        </el-tabs>
      </div>

      <div v-if="wizardStep === 3" class="wizard-step">
        <h3>测试代理</h3>
        <p class="mb-md">测试代理是否正常工作</p>
        <el-input
          type="textarea"
          :rows="1"
          model-value="curl -x http://127.0.0.1:8081 https://api.openai.com/v1/models"
          readonly
          class="mb-md"
        />
        <el-button @click="copyCommand('curl -x http://127.0.0.1:8081 https://api.openai.com/v1/models')">
          <el-icon><DocumentCopy /></el-icon>
          复制测试命令
        </el-button>
        <el-button type="primary" @click="testProxy" :loading="testing">
          <el-icon><Position /></el-icon>
          立即测试
        </el-button>
      </div>

      <div v-if="wizardStep === 4" class="wizard-step">
        <el-result icon="success" title="配置完成" sub-title="系统代理已配置完成">
          <template #extra>
            <el-button type="primary" @click="showWizard = false">完成</el-button>
            <el-button @click="testProxy">再次测试</el-button>
          </template>
        </el-result>
      </div>

      <template #footer>
        <el-button @click="wizardStep--" :disabled="wizardStep === 0">上一步</el-button>
        <el-button type="primary" @click="wizardStep++" :disabled="wizardStep === 4">
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
  Operation,
  DocumentChecked,
  Plus,
  Position,
  Delete,
  Download,
  DocumentCopy,
  ArrowDown,
  ArrowUp,
  View,
  FolderOpened
} from '@element-plus/icons-vue'
import api from '@/api'
import { getProxySetupStatus, type ProxySetupStatus } from '@/api/system-proxy'

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
const savingLan = ref(false)
const detectingIP = ref(false)
const uiMode = ref<'local' | 'team'>('local')
const setupStatus = ref<ProxySetupStatus | null>(null)
const allowLanClients = ref(false)
const advertiseHost = ref('')
const listenAddr = ref('127.0.0.1')
const employeeServer = ref('')
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
const showGuide = ref(false)
const showAddDialog = ref(false)
const showAddPatternDialog = ref(false)
const showWizard = ref(false)
const showPACDialog = ref(false)
const wizardStep = ref(0)
const activeTab = ref('linux')
const adding = ref(false)
const addingPattern = ref(false)
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
const domainList = computed(() => {
  return status.value.pac_domains.map(domain => ({ domain }))
})
const patternList = computed(() => {
  return status.value.pac_patterns.map(pattern => ({ pattern }))
})

const setupSteps = ref({
  active: -1
})

// PAC / CA 一律走 API 口（非 MITM 口）
const apiPACURL = computed(() => {
  if (setupStatus.value?.pac_url) return setupStatus.value.pac_url
  return `http://127.0.0.1:${apiPort.value}/api/v1/proxy/pac`
})

const pacURL = computed(() => apiPACURL.value)

const apiCACertURL = computed(() => {
  if (setupStatus.value?.ca_download_url) return setupStatus.value.ca_download_url
  return `http://127.0.0.1:${apiPort.value}/api/v1/proxy/ca.crt`
})

function proxyctlBin() {
  return 'centag-proxyctl'
}

function copyProxyctlCmd(kind: 'enable' | 'disable' | 'doctor') {
  const bin = proxyctlBin()
  const cmd =
    kind === 'enable' ? `${bin} enable` : kind === 'disable' ? `${bin} disable` : `${bin} doctor`
  copyCommand(cmd)
}

/** 仅进程级代理：勿写入全局 shell，避免影响浏览器与其它 App */
function copyEnvProxyCmd() {
  const host =
    uiMode.value === 'team' && advertiseHost.value
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
      `# 然后启动你的 Agent，例如：opencode`,
    ].join('\n')
  )
}

function copyEmployeeEnable() {
  const base = (employeeServer.value || setupStatus.value?.pac_url || '').replace(/\/api\/v1\/proxy\/pac$/, '')
  const server = base || `http://127.0.0.1:${apiPort.value}`
  copyCommand(`${proxyctlBin()} enable --server ${server}`)
}

async function onAllowLanChange(val: boolean) {
  if (!val) {
    await saveLanConfig()
    return
  }
  try {
    await ElMessageBox.confirm(
      '开启后 MITM 将对局域网可达，请仅在可信内网使用，并配置防火墙。是否继续？',
      '允许局域网客户端',
      { type: 'warning', confirmButtonText: '确认开启', cancelButtonText: '取消' }
    )
    if (!listenAddr.value || listenAddr.value === '127.0.0.1') {
      listenAddr.value = '0.0.0.0'
    }
    if (!advertiseHost.value) {
      await detectLanIP()
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

async function detectLanIP() {
  detectingIP.value = true
  try {
    // Best-effort: use host from current location if not loopback
    const host = window.location.hostname
    if (host && host !== 'localhost' && host !== '127.0.0.1') {
      advertiseHost.value = host
      employeeServer.value = `${window.location.protocol}//${host}:${apiPort.value}`
      ElMessage.success(`已填入 ${host}`)
      return
    }
    ElMessage.info('请手动填写局域网 IP（如 192.168.x.x）')
  } finally {
    detectingIP.value = false
  }
}

async function saveLanConfig() {
  savingLan.value = true
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
    ElMessage.success('局域网出口配置已保存')
    await load()
  } catch (error: any) {
    ElMessage.error('保存失败: ' + error.message)
    allowLanClients.value = !!setupStatus.value?.allow_lan_clients
  } finally {
    savingLan.value = false
  }
}

// 加载状态
const load = async () => {
  loading.value = true
  try {
    // 从config获取完整配置
    const configData = await api.get('/api/v1/config')
    if (configData.system_proxy) {
      status.value.enabled = configData.system_proxy.enabled
      status.value.pac_enabled = configData.system_proxy.pac_enabled
      listenPort.value = configData.system_proxy.listen_port || 8081
      allowLanClients.value = !!configData.system_proxy.allow_lan_clients
      advertiseHost.value = configData.system_proxy.advertise_host || ''
      listenAddr.value = configData.system_proxy.listen_addr || '127.0.0.1'
      if (allowLanClients.value) uiMode.value = 'team'
    }
    if (configData.server?.port) {
      apiPort.value = configData.server.port
    }

    // 从proxy status获取域名等信息
    const proxyData = await api.get('/api/v1/proxy/status')
    status.value.pac_domains = proxyData.pac_domains || []
    status.value.pac_patterns = proxyData.pac_patterns || []

    try {
      setupStatus.value = await getProxySetupStatus()
      if (setupStatus.value?.pac_url && !employeeServer.value) {
        employeeServer.value = setupStatus.value.pac_url.replace(/\/api\/v1\/proxy\/pac$/, '')
      }
    } catch {
      setupStatus.value = null
    }
  } catch (error: any) {
    ElMessage.error('加载状态失败: ' + error.message)
  } finally {
    loading.value = false
  }
}

// 加载域名列表
const loadDomains = async () => {
  try {
    const data = await api.get('/api/v1/proxy/domains')
    status.value.pac_domains = data.domains
  } catch (error: any) {
    console.error('加载域名失败:', error)
  }
}

// 切换代理状态
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

// 添加域名
const openAddDomain = () => {
  newDomain.value = {
    domain: ''
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

// 删除域名
const removeDomain = async (domain: string) => {
  try {
    await ElMessageBox.confirm(`确定删除域名 ${domain} 吗?`, '确认删除', {
      type: 'warning'
    })

    await api.post('/api/v1/proxy/domains/remove', { domain })
    ElMessage.success('域名删除成功')
    await loadDomains()
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error('删除失败: ' + error.message)
    }
  }
}

// 添加路径模式
const openAddPattern = () => {
  newPattern.value = {
    pattern: ''
  }
  showAddPatternDialog.value = true
}

const addPattern = async () => {
  if (!newPattern.value.pattern) {
    ElMessage.warning('请输入路径模式')
    return
  }

  // 确保路径以/开头
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

// 删除路径模式
const removePattern = async (pattern: string) => {
  try {
    await ElMessageBox.confirm(`确定删除路径模式 ${pattern} 吗?`, '确认删除', {
      type: 'warning'
    })

    await api.post('/api/v1/proxy/patterns/remove', { pattern })
    ElMessage.success('路径模式删除成功')
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
    await api.get(`https://${domain}/v1/models`)
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
    // 使用原生fetch下载，避免axios响应拦截器处理blob数据
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

// 复制PAC URL
const copyPACURL = async () => {
  await copyCommand(`PAC URL: ${pacURL.value}`)
}

// 查看PAC文件
const showPACPreview = async () => {
  try {
    const response = await fetch(apiPACURL.value)
    pacContent.value = await response.text()
    showPACDialog.value = true
  } catch (error: any) {
    ElMessage.error('获取PAC文件失败: ' + error.message)
  }
}

// 下载PAC文件
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

// 复制PAC内容
const copyPACContent = async () => {
  await copyCommand(pacContent.value)
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
  testing.value = true
  try {
    await api.get('https://api.openai.com/v1/models')
    ElMessage.success('代理测试成功!')
  } catch (error: any) {
    ElMessage.error(`测试失败: ${error.message}`)
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

.metric-card.pac-active {
  border-color: var(--el-color-primary);
  background: linear-gradient(135deg, rgba(64, 158, 255, 0.1) 0%, var(--el-bg-color) 100%);
}

.metric-card.pac-inactive {
  border-color: var(--el-color-warning);
  background: linear-gradient(135deg, rgba(230, 162, 60, 0.1) 0%, var(--el-bg-color) 100%);
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

.metric-icon.pac-active-icon {
  background: var(--el-color-primary-light-9);
  color: var(--el-color-primary);
}

.metric-icon.pac-inactive-icon {
  background: var(--el-color-warning-light-9);
  color: var(--el-color-warning);
}

.metric-icon.domain-icon {
  background: var(--el-color-primary-light-9);
  color: var(--el-color-primary);
}

.metric-icon.port-icon {
  background: var(--el-color-info-light-9);
  color: var(--el-color-info);
}

.metric-icon.pattern-icon {
  background: var(--el-color-warning-light-9);
  color: var(--el-color-warning);
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
.pac-card,
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

/* PAC预览 */
.pac-preview {
  font-family: 'Courier New', monospace;
  font-size: 12px;
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

.ml-sm {
  margin-left: 8px;
}

.form-hint {
  margin-left: 12px;
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

.hero-card {
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

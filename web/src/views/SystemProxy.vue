<template>
  <div class="system-proxy">
    <div class="page-shell">
      <header class="page-header">
        <div class="header-left">
          <h1 class="page-title">{{ t('systemProxy.pageTitle') }}</h1>
          <p class="page-description">{{ t('systemProxy.pageDescription') }}</p>
        </div>
        <div class="toolbar-actions">
          <el-button :loading="loading" @click="load">
            <el-icon><Refresh /></el-icon>
            {{ t('systemProxy.refresh') }}
          </el-button>
        </div>
      </header>

      <!-- 主引导：告诉用户现在该做什么 -->
      <section class="guide-banner" :class="guideToneClass">
        <div class="guide-copy">
          <span class="guide-kicker">{{ t('systemProxy.guide.kicker') }}</span>
          <h2 class="guide-title">{{ t(nextAction.titleKey) }}</h2>
          <p class="guide-desc">{{ t(nextAction.descKey) }}</p>
        </div>
        <el-button
          :type="nextAction.tone === 'ready' ? 'success' : 'primary'"
          size="large"
          @click="goNextAction"
        >
          {{ t(nextAction.ctaKey) }}
        </el-button>
      </section>

      <!-- 就绪清单：已就绪 / 需配置 -->
      <section class="ready-grid" aria-label="setup readiness">
        <button
          type="button"
          class="ready-card"
          :class="status.enabled ? 'is-ready' : 'needs-action'"
          @click="focusStep(1)"
        >
          <span class="ready-state">{{ status.enabled ? t('systemProxy.guide.ready') : t('systemProxy.guide.needsAction') }}</span>
          <span class="ready-name">{{ t('systemProxy.status.mitm') }}</span>
          <span class="ready-meta">{{ status.enabled ? t('systemProxy.status.running') : t('systemProxy.status.stopped') }}</span>
        </button>
        <button
          type="button"
          class="ready-card"
          :class="egressConfigured ? 'is-ready' : 'needs-action'"
          @click="focusStep(1)"
        >
          <span class="ready-state">{{ egressConfigured ? t('systemProxy.guide.ready') : t('systemProxy.guide.needsAction') }}</span>
          <span class="ready-name">{{ t('systemProxy.status.egressKey') }}</span>
          <span class="ready-meta">{{ egressConfigured ? t('systemProxy.status.autoReady') : t('systemProxy.status.notReady') }}</span>
        </button>
        <button
          type="button"
          class="ready-card"
          :class="testResult?.ok ? 'is-ready' : 'needs-action'"
          @click="focusStep(2)"
        >
          <span class="ready-state">{{ testResult?.ok ? t('systemProxy.guide.ready') : t('systemProxy.guide.needsAction') }}</span>
          <span class="ready-name">{{ t('systemProxy.guide.selfCheck') }}</span>
          <span class="ready-meta">{{ testResult?.ok ? t('systemProxy.guide.selfCheckPassed') : t('systemProxy.guide.selfCheckPending') }}</span>
        </button>
        <button
          type="button"
          class="ready-card is-ready"
          @click="mainTab = 'rules'"
        >
          <span class="ready-state">{{ t('systemProxy.guide.ready') }}</span>
          <span class="ready-name">{{ t('systemProxy.guide.rulesSummary') }}</span>
          <span class="ready-meta">{{ t('systemProxy.guide.rulesMeta', { domains: domainCount, patterns: patternCount }) }}</span>
        </button>
      </section>

      <p v-if="listenDisplay" class="listen-line">
        <span class="listen-label">{{ t('systemProxy.status.listen') }}</span>
        <code>{{ listenDisplay }}</code>
      </p>

      <el-tabs v-model="mainTab" class="main-tabs">
      <!-- ========== Tab 1: 配置向导 ========== -->
      <el-tab-pane :label="t('systemProxy.tabs.wizard')" name="wizard">
        <el-alert
          v-if="setupStatus?.in_container"
          class="mb-md"
          type="warning"
          :closable="false"
          show-icon
          :title="t('systemProxy.wizard.dockerWarning')"
        />

        <el-steps :active="wizardProgress" align-center finish-status="success" class="wizard-steps">
          <el-step
            :title="t('systemProxy.wizard.step1Title')"
            :description="step1Ready ? t('systemProxy.guide.ready') : t('systemProxy.guide.needsAction')"
          />
          <el-step
            :title="t('systemProxy.wizard.step2Title')"
            :description="testResult?.ok ? t('systemProxy.guide.ready') : t('systemProxy.wizard.step2Desc')"
          />
          <el-step
            :title="t('systemProxy.wizard.step3Title')"
            :description="step1Ready && testResult?.ok ? t('systemProxy.guide.doNow') : t('systemProxy.wizard.step3Desc')"
          />
        </el-steps>

        <!-- 步骤 1：启动/关闭 MITM -->
        <el-card
          id="spy-step-1"
          class="step-card"
          :class="stepCardClass(1)"
          shadow="never"
        >
          <div class="step-head">
            <span class="step-num">1</span>
            <div class="step-title-block">
              <div class="step-title-row">
                <h3>{{ status.enabled ? t('systemProxy.wizard.step1HeadingRunning') : t('systemProxy.wizard.step1Heading') }}</h3>
                <el-tag :type="step1Ready ? 'success' : 'warning'" size="small" effect="plain">
                  {{ step1Ready ? t('systemProxy.guide.ready') : t('systemProxy.guide.needsAction') }}
                </el-tag>
              </div>
              <p class="step-summary">{{ t('systemProxy.guide.step1Summary') }}</p>
            </div>
            <el-button
              v-if="status.enabled"
              type="danger"
              plain
              size="small"
              :loading="toggling"
              @click="stopMitm"
            >
              {{ t('systemProxy.wizard.stopMitm') }}
            </el-button>
          </div>

          <div class="primary-action-row">
            <div class="switch-cell switch-cell-primary">
              <span class="switch-label">{{ t('systemProxy.form.mitmService') }}</span>
              <el-switch
                v-model="status.enabled"
                :loading="toggling"
                @change="toggleProxy"
                :active-text="t('systemProxy.status.running')"
                :inactive-text="t('systemProxy.status.stopped')"
              />
              <span class="form-hint">{{ listenDisplay }}</span>
            </div>
            <div class="switch-cell switch-cell-primary">
              <span class="switch-label">{{ t('systemProxy.form.allowLan') }}</span>
              <el-switch v-model="allowLanClients" :loading="savingLan" @change="onAllowLanChange" />
              <span class="form-hint">{{ t('systemProxy.form.lanHint') }}</span>
            </div>
          </div>

          <template v-if="allowLanClients">
            <el-form label-width="110px" class="lan-form" size="small">
              <el-form-item :label="t('systemProxy.form.lanIP')" required>
                <el-input v-model="advertiseHost" :placeholder="t('systemProxy.form.lanIPPlaceholder')" style="max-width: 220px" />
                <el-button class="ml-sm" :loading="detectingIP" @click="detectLanIP">{{ t('systemProxy.detect') }}</el-button>
                <el-button type="primary" class="ml-sm" :loading="savingLan" @click="saveLanConfig">{{ t('systemProxy.save') }}</el-button>
                <div v-if="suggestedLanHosts.length" class="form-hint lan-suggest">
                  {{ t('systemProxy.optional') }}：
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
              <el-form-item :label="t('systemProxy.form.externalAddress')">
                <el-input v-model="employeeServer" placeholder="http://192.168.1.50:20060" style="max-width: 300px" />
              </el-form-item>
            </el-form>
            <p class="form-hint" v-html="t('systemProxy.wizard.lanHint', { apiPort, listenPort })" />
          </template>

          <el-collapse class="optional-collapse mt-sm">
            <el-collapse-item :title="t('systemProxy.guide.moreOptions')" name="step1-more">
              <p class="form-hint" v-html="t('systemProxy.wizard.permissionHint')" />
              <p class="form-hint mt-sm" v-html="t('systemProxy.wizard.step1Description', { port: apiPort })" />

              <div v-if="!egressConfigured || showEgressAdvanced" class="egress-tools mt-sm">
                <div class="sub-label">{{ t('systemProxy.egress.notReadyTitle') }}</div>
                <el-space wrap>
                  <el-button type="primary" size="small" :loading="ensuringEgress" @click="ensureEgressKey()">
                    {{ t('systemProxy.egress.autoBind') }}
                  </el-button>
                  <el-select
                    v-model="selectedEgressKeyId"
                    clearable
                    filterable
                    size="small"
                    :placeholder="t('systemProxy.egress.selectKey')"
                    style="width: 200px"
                    :loading="loadingKeys"
                  >
                    <el-option
                      v-for="k in apiKeyOptions"
                      :key="k.id"
                      :label="`${k.name} (${k.key_prefix}…)`"
                      :value="k.id"
                    />
                  </el-select>
                  <el-button
                    size="small"
                    :disabled="!selectedEgressKeyId"
                    :loading="bindingEgress"
                    @click="bindSelectedEgressKey"
                  >
                    {{ t('systemProxy.egress.bindSelected') }}
                  </el-button>
                </el-space>
              </div>
            </el-collapse-item>
          </el-collapse>
        </el-card>

        <!-- 步骤 2：验证 -->
        <el-card
          id="spy-step-2"
          class="step-card"
          :class="stepCardClass(2)"
          shadow="never"
        >
          <div class="step-head">
            <span class="step-num">2</span>
            <div class="step-title-block">
              <div class="step-title-row">
                <h3>{{ t('systemProxy.wizard.step2Heading') }}</h3>
                <el-tag :type="testResult?.ok ? 'success' : 'warning'" size="small" effect="plain">
                  {{ testResult?.ok ? t('systemProxy.guide.ready') : t('systemProxy.guide.needsAction') }}
                </el-tag>
              </div>
              <p class="step-summary">{{ t('systemProxy.wizard.step2Description') }}</p>
            </div>
            <el-button type="primary" :loading="testing" @click="testProxy">
              {{ t('systemProxy.wizard.testNow') }}
            </el-button>
          </div>

          <ul class="check-list">
            <li :class="status.enabled ? 'pass' : 'fail'">
              {{ status.enabled ? t('systemProxy.wizard.checkMitmRunning') : t('systemProxy.wizard.checkMitmStopped') }}
            </li>
            <li :class="egressConfigured ? 'pass' : 'fail'">
              {{ egressConfigured ? t('systemProxy.wizard.checkEgressReady') : t('systemProxy.wizard.checkEgressNotReady') }}
            </li>
            <li class="info">{{ t('systemProxy.wizard.checkProvider') }}</li>
            <li class="info">{{ t('systemProxy.wizard.checkCert') }}</li>
          </ul>

          <el-alert
            v-if="testResult"
            class="mt-sm"
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

          <el-collapse class="optional-collapse mt-sm">
            <el-collapse-item :title="t('systemProxy.guide.moreOptions')" name="step2-more">
              <el-button size="small" @click="copyProxyctlCmd('doctor')">{{ t('systemProxy.wizard.copyDiagCmd') }}</el-button>
              <p class="form-hint mt-sm">{{ t('systemProxy.wizard.selfCheckHint') }}</p>
            </el-collapse-item>
          </el-collapse>
        </el-card>

        <!-- 步骤 3：接入客户端 -->
        <el-card
          id="spy-step-3"
          class="step-card"
          :class="stepCardClass(3)"
          shadow="never"
        >
          <div class="step-head">
            <span class="step-num">3</span>
            <div class="step-title-block">
              <div class="step-title-row">
                <h3>{{ t('systemProxy.wizard.step3Heading') }}</h3>
                <el-tag type="success" size="small" effect="plain">{{ t('systemProxy.recommended') }}</el-tag>
              </div>
              <p class="step-summary" v-html="t('systemProxy.wizard.step3Description')" />
            </div>
          </div>

          <div class="sub-block is-primary-cmd">
            <div class="sub-label">{{ t('systemProxy.wizard.runWithEnv') }}</div>
            <p class="form-hint mb-sm">{{ t('systemProxy.wizard.runWithEnvHint') }}</p>
            <div class="cmd-block cmd-block-stack">
              <pre class="cmd-pre">{{ envRunScript }}</pre>
              <el-button type="primary" size="small" @click="copyEnvRunScript">{{ t('systemProxy.copy') }}</el-button>
            </div>
          </div>

          <div class="sub-block">
            <div class="sub-label">{{ t('systemProxy.wizard.runWithFlags') }}</div>
            <p class="form-hint mb-sm">{{ t('systemProxy.wizard.runWithFlagsHint') }}</p>
            <div class="cmd-block">
              <code>{{ runCommand }}</code>
              <el-button size="small" @click="copyRunCmd">{{ t('systemProxy.copy') }}</el-button>
            </div>
          </div>

          <el-collapse class="optional-collapse">
            <el-collapse-item :title="t('systemProxy.guide.installAndToken')" name="step3-more">
              <div class="sub-block">
                <div class="sub-label">{{ t('systemProxy.wizard.subStep1') }}</div>
                <div class="cmd-block">
                  <code>{{ installCommand }}</code>
                  <el-button type="primary" size="small" @click="copyInstallCmd">{{ t('systemProxy.copy') }}</el-button>
                </div>
                <p class="form-hint" v-html="t('systemProxy.wizard.subStep1Hint')" />
              </div>

              <div class="sub-block">
                <div class="sub-label">{{ t('systemProxy.wizard.subStep2') }}</div>
                <div class="cmd-block">
                  <code>export CENTAG_WRAP_TOKEN='llmproxy_xxxx'   # Web → API Keys 创建</code>
                  <el-button size="small" @click="copyWrapTokenHint">{{ t('systemProxy.copy') }}</el-button>
                </div>
                <p v-if="setupStatus?.proxy_auth_required || allowLanClients" class="form-hint">
                  {{ t('systemProxy.wizard.subStep2Hint') }}
                </p>
              </div>

              <div class="sub-block">
                <div class="sub-label">{{ t('systemProxy.wizard.optionalPac') }}</div>
                <p class="mb-sm form-hint" v-html="t('systemProxy.wizard.pacHint', { url: apiPACURL })" />
                <el-space wrap>
                  <el-button size="small" @click="copyProxyctlCmd('enable')">{{ t('systemProxy.wizard.pacEnable') }}</el-button>
                  <el-button size="small" type="danger" plain @click="copyProxyctlCmd('disable')">{{ t('systemProxy.wizard.pacDisable') }}</el-button>
                  <el-button size="small" @click="copyEnvProxyCmd">{{ t('systemProxy.wizard.manualProxy') }}</el-button>
                </el-space>
              </div>
            </el-collapse-item>
          </el-collapse>
        </el-card>
      </el-tab-pane>

      <!-- ========== Tab 2: 域名与路径 ========== -->
      <el-tab-pane :label="t('systemProxy.tabs.rules')" name="rules">
        <el-card class="pac-card" shadow="never">
          <template #header>
            <div class="card-header">
              <span class="card-title">{{ t('systemProxy.cards.pacWhitelist') }}</span>
              <div>
                <el-button type="primary" link @click="showPACPreview">
                  <el-icon><View /></el-icon>
                  {{ t('systemProxy.wizard.pacEnable') }}
                </el-button>
                <el-button type="primary" link @click="downloadPAC">
                  <el-icon><Download /></el-icon>
                  {{ t('systemProxy.wizard.pacDisable') }}
                </el-button>
                <el-button :loading="ensuringDefaults" @click="ensureDefaultRules">
                  {{ t('systemProxy.egress.autoBind') }}
                </el-button>
                <el-button type="primary" @click="openAddDomain" :disabled="!status.enabled">
                  <el-icon><Plus /></el-icon>
                  {{ t('systemProxy.add') }}
                </el-button>
              </div>
            </div>
          </template>
          <el-alert type="info" :closable="false" class="mb-md" show-icon>
            {{ t('systemProxy.cards.pacWhitelistHint') }}
          </el-alert>
          <el-table :data="domainList" stripe v-loading="loading" max-height="420">
            <el-table-column prop="domain" :label="t('systemProxy.table.domain')" min-width="220" />
            <el-table-column :label="t('systemProxy.table.test')" width="100" align="center">
              <template #default="{ row }">
                <el-button
                  type="primary"
                  link
                  @click="testDomain(row.domain)"
                  :loading="testingDomains[row.domain]"
                >
                  <el-icon><Position /></el-icon>
                  {{ t('systemProxy.test') }}
                </el-button>
              </template>
            </el-table-column>
            <el-table-column :label="t('systemProxy.table.action')" width="120" align="center">
              <template #default="{ row }">
                <el-button type="danger" link @click="removeDomain(row.domain)" :disabled="!status.enabled">
                  <el-icon><Delete /></el-icon>
                  {{ t('systemProxy.delete') }}
                </el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>

        <el-card class="pattern-card mt-md" shadow="never">
          <template #header>
            <div class="card-header">
              <span class="card-title">{{ t('systemProxy.cards.pathPatterns') }}</span>
              <el-button type="primary" @click="openAddPattern" :disabled="!status.enabled">
                <el-icon><Plus /></el-icon>
                {{ t('systemProxy.dialog.addPattern') }}
              </el-button>
            </div>
          </template>
          <el-alert type="info" :closable="false" class="mb-md" show-icon>
            {{ t('systemProxy.cards.pathPatternsHint') }}
          </el-alert>
          <el-table :data="patternList" stripe v-loading="loading" max-height="320">
            <el-table-column prop="pattern" :label="t('systemProxy.table.pathPattern')" min-width="220" />
            <el-table-column :label="t('systemProxy.table.action')" width="120" align="center">
              <template #default="{ row }">
                <el-button type="danger" link @click="removePattern(row.pattern)" :disabled="!status.enabled">
                  <el-icon><Delete /></el-icon>
                  {{ t('systemProxy.delete') }}
                </el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-tab-pane>

      <!-- ========== Tab 3: 其它 ========== -->
      <el-tab-pane :label="t('systemProxy.tabs.advanced')" name="advanced">
        <el-card class="control-card" shadow="never">
          <template #header>
            <span class="card-title">{{ t('systemProxy.cards.mitmAdvanced') }}</span>
          </template>
          <el-form :model="config" label-width="120px">
            <el-form-item :label="t('systemProxy.form.listenPort')">
              <el-input-number
                v-model="listenPort"
                :min="1024"
                :max="65535"
                :disabled="status.enabled"
                controls-position="right"
              />
              <span class="form-hint">{{ t('systemProxy.form.listenPortHint') }}</span>
            </el-form-item>
            <el-form-item :label="t('systemProxy.form.proxyMode')">
              <el-select
                v-model="status.pac_enabled"
                :disabled="status.enabled"
                style="max-width: 360px"
                @change="onPacModeChange"
              >
                <el-option :label="t('systemProxy.options.pacMode')" :value="true" />
                <el-option :label="t('systemProxy.options.globalMode')" :value="false" />
              </el-select>
            </el-form-item>
            <el-form-item :label="t('systemProxy.form.listenAddr')" v-if="allowLanClients">
              <el-input v-model="listenAddr" placeholder="0.0.0.0" style="max-width: 240px" />
              <el-button type="primary" class="ml-sm" :loading="savingLan" @click="saveLanConfig">{{ t('systemProxy.save') }}</el-button>
            </el-form-item>
          </el-form>
          <el-descriptions :column="2" size="small" border>
            <el-descriptions-item :label="t('systemProxy.form.pacUrl')">{{ apiPACURL }}</el-descriptions-item>
            <el-descriptions-item :label="t('systemProxy.form.mitmProxy')">
              {{ setupStatus?.mitm_proxy || `http://127.0.0.1:${listenPort}` }}
            </el-descriptions-item>
            <el-descriptions-item :label="t('systemProxy.form.caFingerprint')">
              {{ setupStatus?.ca_fingerprint_sha256 || t('systemProxy.form.caFingerprintPlaceholder') }}
            </el-descriptions-item>
            <el-descriptions-item :label="t('systemProxy.form.loopback')">
              {{ setupStatus?.listen_is_loopback !== false ? t('systemProxy.yes') : t('systemProxy.no') }}
            </el-descriptions-item>
          </el-descriptions>
        </el-card>

        <el-card class="cert-card mt-md" shadow="never">
          <template #header>
            <span class="card-title">{{ t('systemProxy.cards.caCert') }}</span>
          </template>
          <el-alert :title="t('systemProxy.cards.caCertHint')" type="warning" :closable="false" show-icon class="mb-md" />
          <el-space wrap class="mb-md">
            <el-button type="primary" @click="downloadCACert">
              <el-icon><Download /></el-icon>
              {{ t('systemProxy.cert.downloadCert') }}
            </el-button>
            <el-button @click="copyCertCommand">
              <el-icon><DocumentCopy /></el-icon>
              {{ t('systemProxy.cert.copyInstallCmd') }}
            </el-button>
          </el-space>
          <el-descriptions :column="2" border size="small">
            <el-descriptions-item :label="t('systemProxy.cert.status')">
              <el-tag :type="certInfo.valid ? 'success' : 'danger'" size="small">
                {{ certInfo.valid ? t('systemProxy.cert.valid') : t('systemProxy.cert.invalid') }}
              </el-tag>
            </el-descriptions-item>
            <el-descriptions-item :label="t('systemProxy.cert.issuer')">{{ certInfo.issuer }}</el-descriptions-item>
            <el-descriptions-item :label="t('systemProxy.cert.expires')">{{ certInfo.expires }}</el-descriptions-item>
            <el-descriptions-item :label="t('systemProxy.cert.daysLeft')">{{ certInfo.daysLeft }} {{ t('systemProxy.days') }}</el-descriptions-item>
          </el-descriptions>
        </el-card>
      </el-tab-pane>
    </el-tabs>

    <!-- 添加域名 -->
    <el-dialog v-model="showAddDialog" :title="t('systemProxy.dialog.addDomain')" width="500px">
      <el-form :model="newDomain" label-width="100px">
        <el-form-item :label="t('systemProxy.table.domain')">
          <el-input v-model="newDomain.domain" :placeholder="t('systemProxy.domainPlaceholder')" />
          <el-alert type="warning" :closable="false" class="mt-md">
            {{ t('systemProxy.dialog.addDomainHint') }}
          </el-alert>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showAddDialog = false">{{ t('systemProxy.cancel') }}</el-button>
        <el-button type="primary" @click="addDomain" :loading="adding">{{ t('systemProxy.add') }}</el-button>
      </template>
    </el-dialog>

    <!-- 添加路径模式 -->
    <el-dialog v-model="showAddPatternDialog" :title="t('systemProxy.dialog.addPattern')" width="500px">
      <el-form :model="newPattern" label-width="100px">
        <el-form-item :label="t('systemProxy.table.pathPattern')">
          <el-input v-model="newPattern.pattern" :placeholder="t('systemProxy.patternPlaceholder')" />
          <el-alert type="info" :closable="false" class="mt-md">
            {{ t('systemProxy.dialog.addPatternHint') }}
          </el-alert>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showAddPatternDialog = false">{{ t('systemProxy.cancel') }}</el-button>
        <el-button type="primary" @click="addPattern" :loading="addingPattern">{{ t('systemProxy.add') }}</el-button>
      </template>
    </el-dialog>

    <!-- PAC 预览 -->
    <el-dialog v-model="showPACDialog" :title="t('systemProxy.dialog.pacPreview')" width="800px">
      <el-input type="textarea" :rows="20" v-model="pacContent" readonly class="pac-preview" />
      <template #footer>
        <el-button @click="showPACDialog = false">{{ t('systemProxy.close') }}</el-button>
        <el-button type="primary" @click="downloadPAC">
          <el-icon><Download /></el-icon>
          {{ t('systemProxy.wizard.pacDisable') }}
        </el-button>
        <el-button @click="copyPACContent">
          <el-icon><DocumentCopy /></el-icon>
          {{ t('systemProxy.copy') }}
        </el-button>
      </template>
    </el-dialog>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
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

const { t } = useI18n()

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
/** 出口 Key 已就绪时隐藏手动补绑；失败时可展开 */
const showEgressAdvanced = ref(false)
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

const step1Ready = computed(() => status.value.enabled && egressConfigured.value)

type NextAction = {
  step: 1 | 2 | 3
  tone: 'action' | 'ready'
  titleKey: string
  descKey: string
  ctaKey: string
}

const nextAction = computed<NextAction>(() => {
  if (!status.value.enabled) {
    return {
      step: 1,
      tone: 'action',
      titleKey: 'systemProxy.guide.nextStartMitm',
      descKey: 'systemProxy.guide.nextStartMitmDesc',
      ctaKey: 'systemProxy.guide.ctaGoStep1'
    }
  }
  if (!egressConfigured.value) {
    return {
      step: 1,
      tone: 'action',
      titleKey: 'systemProxy.guide.nextEgress',
      descKey: 'systemProxy.guide.nextEgressDesc',
      ctaKey: 'systemProxy.guide.ctaGoStep1'
    }
  }
  if (!testResult.value?.ok) {
    return {
      step: 2,
      tone: 'action',
      titleKey: 'systemProxy.guide.nextVerify',
      descKey: 'systemProxy.guide.nextVerifyDesc',
      ctaKey: 'systemProxy.guide.ctaGoStep2'
    }
  }
  return {
    step: 3,
    tone: 'ready',
    titleKey: 'systemProxy.guide.nextClient',
    descKey: 'systemProxy.guide.nextClientDesc',
    ctaKey: 'systemProxy.guide.ctaGoStep3'
  }
})

const guideToneClass = computed(() =>
  nextAction.value.tone === 'ready' ? 'is-ready' : 'needs-action'
)

function stepCardClass(step: 1 | 2 | 3) {
  const current = nextAction.value.step
  const done =
    (step === 1 && step1Ready.value) ||
    (step === 2 && !!testResult.value?.ok) ||
    (step === 3 && step1Ready.value && !!testResult.value?.ok)
  return {
    'is-current': current === step,
    'is-done': done && current !== step,
    'is-pending': current < step
  }
}

function focusStep(step: 1 | 2 | 3) {
  mainTab.value = 'wizard'
  requestAnimationFrame(() => {
    document.getElementById(`spy-step-${step}`)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  })
}

function goNextAction() {
  if (nextAction.value.step === 2 && step1Ready.value && !testResult.value) {
    focusStep(2)
    void testProxy()
    return
  }
  focusStep(nextAction.value.step)
}

const domainCount = computed(() => status.value.pac_domains.length)
const patternCount = computed(() => status.value.pac_patterns.length)
const domainList = computed(() => status.value.pac_domains.map(domain => ({ domain })))
const patternList = computed(() => status.value.pac_patterns.map(pattern => ({ pattern })))

const listenDisplay = computed(() => {
  const addr = setupStatus.value?.listen_addr || `${listenAddr.value || '127.0.0.1'}:${listenPort.value}`
  return addr.includes(':') ? addr : `${addr}:${listenPort.value}`
})

const wizardProgress = computed(() => {
  if (!step1Ready.value) return 0
  // MITM + 出口 Key 就绪后停留在「验证」；自检通过再指向接入
  return testResult.value?.ok ? 2 : 1
})

const apiPACURL = computed(() => {
  if (setupStatus.value?.pac_url) return setupStatus.value.pac_url
  return `http://127.0.0.1:${apiPort.value}/api/v1/proxy/pac`
})

const installCommand = computed(
  () =>
    'curl -fsSL https://raw.githubusercontent.com/atoml-ai/centag/v0.2.9/scripts/install.sh | bash && . "$HOME/.centag/env"'
)

function wrapBin() {
  // Prefer main-binary subcommand (personal ships wrap; no separate centag-wrap asset).
  return 'centag wrap'
}

function normalizeAPIBase(raw: string) {
  return raw.trim().replace(/\/api\/v1\/proxy\/pac\/?$/, '').replace(/\/$/, '')
}

/** 与「对外访问地址」同步；空时回退局域网 IP / PAC */
const employeeAPIBase = computed(() => {
  const fromField = normalizeAPIBase(employeeServer.value)
  if (fromField) return fromField
  const host = advertiseHost.value.trim()
  if (allowLanClients.value && host && host !== 'localhost' && host !== '127.0.0.1' && host !== '::1') {
    return `http://${host}:${apiPort.value}`
  }
  const fromPac = normalizeAPIBase(setupStatus.value?.pac_url || '')
  if (fromPac) return fromPac
  return `http://127.0.0.1:${apiPort.value}`
})

/** 本机局域网 IP 变更时，对外访问地址跟随（与探测按钮行为一致） */
function syncEmployeeServerFromAdvertiseHost() {
  const host = advertiseHost.value.trim()
  if (!host || host === 'localhost' || host === '127.0.0.1' || host === '::1') return
  employeeServer.value = `http://${host}:${apiPort.value}`
}

watch([advertiseHost, apiPort], () => {
  if (!allowLanClients.value) return
  syncEmployeeServerFromAdvertiseHost()
})

watch(allowLanClients, (on) => {
  if (on) syncEmployeeServerFromAdvertiseHost()
})

const runCommand = computed(() => {
  if (allowLanClients.value) {
    return `${wrapBin()} run --server ${employeeAPIBase.value} --token llmproxy_xxxx -- opencode`
  }
  return `${wrapBin()} run -- opencode`
})

/** 先 export 再 run：日常命令更短 */
const envRunScript = computed(() => {
  const lines: string[] = [
    `# ${t('systemProxy.wizard.envScriptComment')}`,
    "export CENTAG_WRAP_TOKEN='llmproxy_xxxx'   # Web → API Keys"
  ]
  if (allowLanClients.value) {
    lines.push(`export CENTAG_API_BASE='${employeeAPIBase.value}'`)
  }
  lines.push(`${wrapBin()} run -- opencode`)
  return lines.join('\n')
})

function copyProxyctlCmd(kind: 'enable' | 'disable' | 'doctor') {
  const bin = wrapBin()
  let cmd =
    kind === 'enable' ? `${bin} enable` : kind === 'disable' ? `${bin} disable` : `${bin} doctor`
  if (allowLanClients.value) {
    const server = employeeAPIBase.value
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

function copyEnvRunScript() {
  copyCommand(envRunScript.value)
}

function copyInstallCmd() {
  copyCommand(installCommand.value)
}

function copyWrapTokenHint() {
  copyCommand(
    [
      '# 推荐：命令行传入 Token（也可用环境变量 CENTAG_WRAP_TOKEN）',
      `${wrapBin()} run --server ${employeeAPIBase.value} --token llmproxy_xxxx -- opencode`,
      '',
      '# 或先 export 再 run：',
      "export CENTAG_WRAP_TOKEN='llmproxy_xxxx'",
      `export CENTAG_API_BASE='${employeeAPIBase.value}'`,
      `${wrapBin()} run -- opencode`
    ].join('\n')
  )
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

async function ensureEgressKey(opts?: { silent?: boolean }) {
  ensuringEgress.value = true
  try {
    const res = await ensureEgressAPIKey()
    if (res.configured) {
      showEgressAdvanced.value = false
      if (!opts?.silent) {
        ElMessage.success(res.changed ? t('systemProxy.egress.autoBound') : t('systemProxy.egress.alreadyReady'))
      }
    } else {
      showEgressAdvanced.value = true
      if (!opts?.silent) {
        ElMessage.warning(t('systemProxy.egress.stillNotReady'))
      }
    }
    await load({ skipAutoEgress: true })
  } catch (error: any) {
    showEgressAdvanced.value = true
    if (!opts?.silent) {
      ElMessage.error(t('systemProxy.egress.autoBindFailed') + ': ' + (error.message || error))
    }
  } finally {
    ensuringEgress.value = false
  }
}

async function bindSelectedEgressKey() {
  if (!selectedEgressKeyId.value) return
  bindingEgress.value = true
  try {
    await bindEgressAPIKey(selectedEgressKeyId.value)
    ElMessage.success(t('systemProxy.egress.boundSuccess'))
    await load()
  } catch (error: any) {
    ElMessage.error(t('systemProxy.egress.bindFailed') + ': ' + (error.message || error))
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
      t('systemProxy.message.lanConfirmMessage'),
      t('systemProxy.message.lanConfirmTitle'),
      { type: 'warning', confirmButtonText: t('systemProxy.confirm'), cancelButtonText: t('systemProxy.cancel') }
    )
    if (!listenAddr.value || listenAddr.value === '127.0.0.1') {
      listenAddr.value = '0.0.0.0'
    }
    if (!advertiseHost.value.trim()) {
      await detectLanIP()
    }
    // 探测失败时不要立刻保存：留在表单让用户填 IP / 点候选
    if (!advertiseHost.value.trim()) {
      ElMessage.warning(t('systemProxy.message.fillLanIP'))
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
      t('systemProxy.message.globalModeWarning'),
      t('systemProxy.message.globalModeTitle'),
      { type: 'warning' }
    )
  } catch {
    status.value.pac_enabled = true
  }
}

function pickLanHost(ip: string) {
  advertiseHost.value = ip
  syncEmployeeServerFromAdvertiseHost()
}

async function detectLanIP() {
  detectingIP.value = true
  try {
    // 1) 浏览器地址栏若已是局域网 IP，直接用
    const host = window.location.hostname
    if (host && host !== 'localhost' && host !== '127.0.0.1' && host !== '[::1]') {
      pickLanHost(host)
      ElMessage.success(t('systemProxy.message.filledIP', { ip: host }))
      return
    }

    // 2) 向服务端询问本机网卡 IP（localhost 打开页面时也能用）
    try {
      const st = await getProxySetupStatus()
      const list = st.suggested_lan_hosts || []
      suggestedLanHosts.value = list
      if (list.length === 1) {
        pickLanHost(list[0])
        ElMessage.success(t('systemProxy.message.filledIP', { ip: list[0] }))
        return
      }
      if (list.length > 1) {
        pickLanHost(list[0])
        ElMessage.info(t('systemProxy.message.multipleInterfaces', { ip: list[0] }))
        return
      }
    } catch {
      // ignore, fall through
    }

    ElMessage.warning(t('systemProxy.message.autoDetectFailed'))
  } finally {
    detectingIP.value = false
  }
}

async function saveLanConfig() {
  if (allowLanClients.value) {
    const host = advertiseHost.value.trim()
    if (!host) {
      ElMessage.warning(t('systemProxy.message.lanIPRequired'))
      return
    }
    if (host === 'localhost' || host === '127.0.0.1' || host === '::1') {
      ElMessage.warning(t('systemProxy.message.lanIPInvalid'))
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
    ElMessage.success(allowLanClients.value ? t('systemProxy.message.lanEnabled') : t('systemProxy.message.lanDisabled'))
    await load()
  } catch (error: any) {
    ElMessage.error(t('systemProxy.message.saveFailed') + ': ' + error.message)
    allowLanClients.value = !!setupStatus.value?.allow_lan_clients
  } finally {
    savingLan.value = false
  }
}

const load = async (opts?: { skipAutoEgress?: boolean }) => {
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
      // 对外访问地址与局域网 IP / PAC 对齐（优先 advertise_host）
      if (allowLanClients.value && advertiseHost.value.trim()) {
        syncEmployeeServerFromAdvertiseHost()
      } else if (setupStatus.value?.pac_url) {
        employeeServer.value = normalizeAPIBase(setupStatus.value.pac_url)
      }
    } catch {
      setupStatus.value = null
      suggestedLanHosts.value = []
    }
    void loadAPIKeys()

    // MITM 已开但出口 Key 未就绪时补一次自动绑定（与后端开启 MITM 时的 Ensure 对齐）
    if (
      !opts?.skipAutoEgress &&
      status.value.enabled &&
      !setupStatus.value?.egress_api_key_configured
    ) {
      void ensureEgressKey({ silent: true })
    }
    showEgressAdvanced.value = status.value.enabled && !setupStatus.value?.egress_api_key_configured
  } catch (error: any) {
    ElMessage.error(t('systemProxy.message.loadFailed') + ': ' + error.message)
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
    ElMessage.success(d + p === 0 ? t('systemProxy.message.defaultsUpdated') : t('systemProxy.message.defaultsUpdatedDetail', { domains: d, patterns: p }))
    await load()
  } catch (error: any) {
    ElMessage.error(t('systemProxy.message.defaultsFailed') + ': ' + (error.message || error))
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
    ElMessage.success(
      status.value.enabled ? t('systemProxy.message.mitmEnabled') : t('systemProxy.message.mitmDisabled')
    )
    await load()
  } catch (error: any) {
    ElMessage.error(t('systemProxy.message.operationFailed') + ': ' + error.message)
    status.value.enabled = !status.value.enabled
  } finally {
    toggling.value = false
  }
}

/** 运行中时提供显式「停止」入口（避免用户以为步骤 1 消失了）。 */
const stopMitm = async () => {
  if (!status.value.enabled || toggling.value) return
  status.value.enabled = false
  await toggleProxy()
}

const openAddDomain = () => {
  newDomain.value = { domain: '' }
  showAddDialog.value = true
}

const addDomain = async () => {
  if (!newDomain.value.domain) {
    ElMessage.warning(t('systemProxy.message.pleaseEnterDomain'))
    return
  }
  adding.value = true
  try {
    await api.post('/api/v1/proxy/domains/add', { domain: newDomain.value.domain })
    ElMessage.success(t('systemProxy.message.domainAdded'))
    showAddDialog.value = false
    await loadDomains()
  } catch (error: any) {
    ElMessage.error(t('systemProxy.message.addFailed') + ': ' + error.message)
  } finally {
    adding.value = false
  }
}

const removeDomain = async (domain: string) => {
  try {
    await ElMessageBox.confirm(t('systemProxy.confirmDeleteDomain', { domain }), t('systemProxy.confirmDelete'), { type: 'warning' })
    await api.post('/api/v1/proxy/domains/remove', { domain })
    ElMessage.success(t('systemProxy.message.domainDeleted'))
    await loadDomains()
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error(t('systemProxy.message.deleteFailed') + ': ' + error.message)
    }
  }
}

const openAddPattern = () => {
  newPattern.value = { pattern: '' }
  showAddPatternDialog.value = true
}

const addPattern = async () => {
  if (!newPattern.value.pattern) {
    ElMessage.warning(t('systemProxy.message.pleaseEnterPattern'))
    return
  }
  let pattern = newPattern.value.pattern.trim()
  if (!pattern.startsWith('/')) {
    pattern = '/' + pattern
  }
  addingPattern.value = true
  try {
    await api.post('/api/v1/proxy/patterns/add', { pattern })
    ElMessage.success(t('systemProxy.message.patternAdded'))
    showAddPatternDialog.value = false
    await load()
  } catch (error: any) {
    ElMessage.error(t('systemProxy.message.addFailed') + ': ' + error.message)
  } finally {
    addingPattern.value = false
  }
}

const removePattern = async (pattern: string) => {
  try {
    await ElMessageBox.confirm(t('systemProxy.confirmDeletePattern', { pattern }), t('systemProxy.confirmDelete'), { type: 'warning' })
    await api.post('/api/v1/proxy/patterns/remove', { pattern })
    ElMessage.success(t('systemProxy.message.patternDeleted'))
    await load()
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error(t('systemProxy.message.deleteFailed') + ': ' + error.message)
    }
  }
}

const testDomain = async (domain: string) => {
  testingDomains.value[domain] = true
  try {
    await api.get(`https://${domain}/v1/models`)
    ElMessage.success(t('systemProxy.message.domainTestSuccess', { domain }))
  } catch (error: any) {
    ElMessage.error(t('systemProxy.message.testFailed', { message: error.message }))
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
    ElMessage.success(t('systemProxy.message.caCertDownloaded'))
  } catch (error: any) {
    ElMessage.error(t('systemProxy.message.downloadFailed') + ': ' + error.message)
  }
}

const copyCommand = async (command: string) => {
  try {
    await navigator.clipboard.writeText(command)
    ElMessage.success(t('systemProxy.message.copiedToClipboard'))
  } catch {
    ElMessage.error(t('systemProxy.message.copyFailed'))
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
    ElMessage.error(t('systemProxy.message.pacFileFetched') + ': ' + error.message)
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
    ElMessage.success(t('systemProxy.message.pacDownloaded'))
  } catch (error: any) {
    ElMessage.error(t('systemProxy.message.downloadFailed') + ': ' + error.message)
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
      text: status.value.enabled ? t('systemProxy.wizardCheck.mitmRunning') : t('systemProxy.wizardCheck.mitmStopped')
    })
    lines.push({
      ok: egressConfigured.value,
      text: egressConfigured.value ? t('systemProxy.wizardCheck.egressConfigured') : t('systemProxy.egress.notConfigured')
    })

    try {
      const pacResp = await fetch(apiPACURL.value, { cache: 'no-store' })
      const pacText = pacResp.ok ? await pacResp.text() : ''
      const hasProxy = /PROXY\s+\S+:\d+/i.test(pacText)
      lines.push({
        ok: pacResp.ok && hasProxy,
        text: pacResp.ok && hasProxy
          ? t('systemProxy.wizardCheck.pacAccessible', { size: pacText.length })
          : t('systemProxy.wizardCheck.pacError', { status: pacResp.status })
      })
    } catch (e: any) {
      lines.push({ ok: false, text: t('systemProxy.wizardCheck.pacFetchFailed', { message: e.message || e }) })
    }

    try {
      const caResp = await fetch('/api/v1/proxy/ca.crt', { cache: 'no-store' })
      lines.push({
        ok: caResp.ok,
        text: caResp.ok ? t('systemProxy.wizardCheck.caDownloadOk') : t('systemProxy.wizardCheck.caDownloadFail', { status: caResp.status })
      })
    } catch (e: any) {
      lines.push({ ok: false, text: t('systemProxy.wizardCheck.caDownloadError', { message: e.message || e }) })
    }

    if (setupStatus.value?.in_container) {
      lines.push({
        ok: true,
        text: t('systemProxy.wizardCheck.dockerHint')
      })
    }

    const ok = lines.every(l => l.ok)
    testResult.value = {
      ok,
      title: ok ? t('systemProxy.wizardCheck.allPassed') : t('systemProxy.wizardCheck.hasFailed'),
      lines
    }
    if (ok) ElMessage.success(t('systemProxy.wizardCheck.selfCheckPassed'))
    else ElMessage.warning(t('systemProxy.wizardCheck.selfCheckPartial'))
  } catch (error: any) {
    testResult.value = {
      ok: false,
      title: t('systemProxy.wizardCheck.selfCheckFailed'),
      lines: [{ ok: false, text: error.message || String(error) }]
    }
    ElMessage.error(t('systemProxy.wizardCheck.testFailed') + ': ' + (error.message || error))
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
  min-height: 100%;
  padding: 8px 24px 40px;
}

.page-shell {
  max-width: 960px;
  margin: 0 auto;
  width: 100%;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 20px;
  flex-wrap: wrap;
}

.header-left {
  flex: 1;
  min-width: 220px;
}

.page-title {
  margin: 0;
  font-size: 1.5rem;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.page-description {
  margin: 6px 0 0;
  font-size: 0.875rem;
  line-height: 1.5;
  color: var(--el-text-color-secondary);
  max-width: 46rem;
}

.toolbar-actions {
  display: flex;
  gap: 12px;
}

.guide-banner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  padding: 18px 22px;
  margin-bottom: 16px;
  border-radius: 12px;
  border: 1px solid var(--el-border-color-lighter);
  background: var(--el-fill-color-blank);
}

.guide-banner.needs-action {
  border-color: var(--el-color-primary-light-5);
  background: linear-gradient(135deg, var(--el-color-primary-light-9), var(--el-bg-color));
}

.guide-banner.is-ready {
  border-color: var(--el-color-success-light-5);
  background: linear-gradient(135deg, var(--el-color-success-light-9), var(--el-bg-color));
}

.guide-kicker {
  display: inline-block;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.04em;
  color: var(--el-text-color-secondary);
  margin-bottom: 4px;
}

.guide-title {
  margin: 0;
  font-size: 1.125rem;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.guide-desc {
  margin: 6px 0 0;
  font-size: 0.875rem;
  line-height: 1.5;
  color: var(--el-text-color-secondary);
  max-width: 36rem;
}

.ready-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 12px;
}

.ready-card {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 4px;
  padding: 14px 14px 12px;
  border-radius: 10px;
  border: 1px solid var(--el-border-color-lighter);
  background: var(--el-bg-color);
  text-align: left;
  cursor: pointer;
  transition: border-color 0.15s, box-shadow 0.15s, transform 0.15s;
}

.ready-card:hover {
  border-color: var(--el-color-primary-light-5);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.04);
}

.ready-card.is-ready {
  border-color: var(--el-color-success-light-5);
  background: var(--el-color-success-light-9);
}

.ready-card.needs-action {
  border-color: var(--el-color-warning-light-5);
  background: var(--el-color-warning-light-9);
}

.ready-state {
  font-size: 11px;
  font-weight: 600;
  color: var(--el-text-color-secondary);
}

.ready-card.is-ready .ready-state {
  color: var(--el-color-success);
}

.ready-card.needs-action .ready-state {
  color: var(--el-color-warning-dark-2);
}

.ready-name {
  font-size: 0.9375rem;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.ready-meta {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.listen-line {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0 0 16px;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.listen-label {
  flex-shrink: 0;
}

.listen-line code {
  font-size: 12px;
  padding: 2px 8px;
  border-radius: 6px;
  background: var(--el-fill-color-light);
}

.main-tabs :deep(.el-tabs__header) {
  margin-bottom: 16px;
}

.main-tabs :deep(.el-tabs__content) {
  padding-top: 4px;
}

.wizard-steps {
  margin-bottom: 20px;
}

.wizard-steps :deep(.el-step__title) {
  font-size: 13px;
  line-height: 1.3;
}

.wizard-steps :deep(.el-step__description) {
  font-size: 11px;
}

.step-card {
  margin-bottom: 14px;
  border-radius: 12px;
  border: 1px solid var(--el-border-color-lighter);
  transition: border-color 0.15s, opacity 0.15s, box-shadow 0.15s;
}

.step-card.is-current {
  border-color: var(--el-color-primary-light-5);
  box-shadow: 0 0 0 1px var(--el-color-primary-light-7);
}

.step-card.is-done {
  border-color: var(--el-color-success-light-5);
}

.step-card.is-pending {
  opacity: 0.72;
}

.step-card :deep(.el-card__body) {
  padding: 18px 20px;
}

.step-head {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  margin-bottom: 14px;
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
  font-size: 13px;
  flex-shrink: 0;
  margin-top: 2px;
}

.step-card.is-done .step-num {
  background: var(--el-color-success);
}

.step-title-block {
  flex: 1;
  min-width: 0;
}

.step-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.step-title-block h3 {
  margin: 0;
  font-size: 1rem;
  font-weight: 600;
}

.step-summary {
  margin: 6px 0 0;
  font-size: 0.8125rem;
  line-height: 1.5;
  color: var(--el-text-color-secondary);
}

.step-summary code {
  font-size: 11px;
}

.primary-action-row {
  display: flex;
  align-items: flex-start;
  flex-wrap: wrap;
  gap: 14px 28px;
  padding: 14px 16px;
  border-radius: 10px;
  background: var(--el-fill-color-lighter);
}

.primary-action-row .switch-cell {
  display: grid;
  grid-template-columns: auto 1fr;
  grid-template-rows: auto auto;
  column-gap: 10px;
  row-gap: 4px;
  align-items: center;
  min-width: 220px;
}

.primary-action-row .switch-label {
  grid-column: 1;
  grid-row: 1;
}

.primary-action-row .el-switch {
  grid-column: 2;
  grid-row: 1;
  justify-self: start;
}

.primary-action-row .form-hint {
  grid-column: 1 / -1;
  grid-row: 2;
}

.switch-row {
  display: flex;
  flex-wrap: wrap;
  gap: 12px 28px;
  margin-bottom: 8px;
}

.switch-cell {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 240px;
}

.switch-cell-primary {
  min-width: 0;
}

.switch-label {
  font-size: 13px;
  font-weight: 500;
  color: var(--el-text-color-regular);
  white-space: nowrap;
}

.lan-form {
  margin-top: 8px;
}

.sub-block {
  margin-bottom: 12px;
}

.sub-block.is-primary-cmd .cmd-block {
  border: 1px solid var(--el-color-primary-light-5);
  background: var(--el-color-primary-light-9);
}

.sub-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--el-text-color-regular);
  margin-bottom: 6px;
}

.cmd-block {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  padding: 10px 12px;
  border-radius: 8px;
  background: var(--el-fill-color-light);
  margin-bottom: 6px;
}

.cmd-block code,
.cmd-pre {
  flex: 1;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  word-break: break-all;
}

.cmd-block-stack {
  align-items: flex-start;
}

.cmd-pre {
  margin: 0;
  white-space: pre-wrap;
  line-height: 1.5;
  color: var(--el-text-color-primary);
}

.optional-collapse {
  border: none;
}

.optional-collapse :deep(.el-collapse-item__header) {
  font-size: 12px;
  height: 36px;
  line-height: 36px;
  color: var(--el-text-color-secondary);
}

.optional-collapse :deep(.el-collapse-item__wrap) {
  border-bottom: none;
}

.check-list {
  margin: 0 0 10px;
  padding-left: 18px;
}

.check-list li {
  margin-bottom: 4px;
  font-size: 13px;
  line-height: 1.45;
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
  gap: 12px;
  flex-wrap: wrap;
}

.card-title {
  font-weight: 600;
  font-size: 1rem;
}

.pac-preview {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
}

.mt-md {
  margin-top: 12px;
}

.mt-sm {
  margin-top: 8px;
}

.mb-md {
  margin-bottom: 12px;
}

.mb-sm {
  margin-bottom: 6px;
}

.ml-sm {
  margin-left: 8px;
}

.form-hint {
  color: var(--el-text-color-secondary);
  font-size: 12px;
  line-height: 1.45;
}

.form-hint code {
  font-size: 11px;
}

.lan-suggest {
  width: 100%;
  margin-top: 4px;
  margin-left: 0;
}

.egress-tools {
  padding-top: 4px;
}

@media (max-width: 900px) {
  .ready-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 768px) {
  .system-proxy {
    padding: 8px 16px 32px;
  }

  .page-header,
  .guide-banner {
    flex-direction: column;
    align-items: stretch;
  }

  .ready-grid {
    grid-template-columns: 1fr;
  }

  .switch-cell {
    min-width: 100%;
  }
}
</style>

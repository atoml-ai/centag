<template>
  <div class="agent-setup-page">
    <div class="page-header">
      <div>
        <h1 class="page-title">
          <el-icon><Link /></el-icon>
          {{ $t('agentSetup.title') }}
        </h1>
        <p class="page-description">{{ $t('agentSetup.subtitle') }}</p>
      </div>
    </div>

    <el-tabs v-model="activeTab" class="setup-tabs">
      <!-- Tab 1: 快速接入 -->
      <el-tab-pane :label="$t('agentSetup.quickSetup')" name="setup">
        <div class="section-block">
          <p class="section-hint section-hint--multiline">{{ $t('agentSetup.quickSetupHint') }}</p>

          <div class="agent-filter-bar">
            <el-input
              v-model="filterText"
              :placeholder="$t('agentSetup.filterPlaceholder')"
              clearable
              prefix-icon="Search"
              class="agent-filter-input"
            />
          </div>

          <div
            v-for="group in agentGroups"
            :key="group.id"
            class="agent-group"
          >
            <div class="agent-group-header">
              <h3 class="agent-group-title">{{ group.title }}</h3>
              <p v-if="group.hint" class="agent-group-hint">{{ group.hint }}</p>
            </div>
            <el-row :gutter="16" class="agent-card-row">
              <el-col
                v-for="agent in group.agents"
                :key="agent.type"
                :xs="24" :sm="12" :md="12"
                class="agent-card-col"
              >
                <div
                  class="agent-card"
                  :class="{ 'agent-card--verified': agentHasAnyVerified(agent) }"
                >
                  <div class="agent-card-head">
                    <div class="agent-icon">
                      <el-icon :size="20">
                        <component :is="agentIcon(agent.type)" />
                      </el-icon>
                    </div>
                    <div class="agent-head-text">
                      <div class="agent-name">
                        <span class="agent-name-text">{{ agent.display_name }}</span>
                      </div>
                      <div class="agent-desc">{{ agentLocalized(agent, 'description') }}</div>
                    </div>
                  </div>

                  <div class="access-methods" @click.stop>
                    <template v-for="(method, idx) in agentAccessMethods(agent)" :key="method">
                      <!-- 写入配置 -->
                      <div v-if="method === 'write_config'" class="access-method">
                        <div class="access-method-head">
                          <div class="access-method-titles">
                            <span class="access-method-index">{{ idx + 1 }}</span>
                            <div>
                              <div class="access-method-title">
                                {{ $t('agentSetup.methodWriteConfig') }}
                                <el-tag
                                  size="small"
                                  :type="agent.verified_write ? 'success' : 'info'"
                                  effect="plain"
                                  class="method-status-tag"
                                >
                                  {{ agent.verified_write ? $t('agentSetup.verified') : $t('agentSetup.unverified') }}
                                </el-tag>
                              </div>
                              <p class="access-method-hint">
                                {{ agent.verified_write
                                  ? $t('agentSetup.methodWriteConfigHint')
                                  : $t('agentSetup.methodWriteConfigUnverifiedHint') }}
                              </p>
                            </div>
                          </div>
                          <div class="access-method-actions">
                            <el-button type="primary" size="small" @click="openWizard(agent)">
                              {{ $t('agentSetup.writeConfigAction') }}
                            </el-button>
                            <el-button
                              size="small"
                              :loading="restoringAgent === agent.type"
                              @click="restoreDefaults(agent)"
                            >
                              {{ $t('agentSetup.restoreDefault') }}
                            </el-button>
                          </div>
                        </div>
                      </div>

                      <!-- UI 指引：只给可填参数（流水线驱动模型 ID） -->
                      <div v-else-if="method === 'ui_guide'" class="access-method">
                        <div class="access-method-head">
                          <div class="access-method-titles">
                            <span class="access-method-index">{{ idx + 1 }}</span>
                            <div>
                              <div class="access-method-title">
                                {{ $t('agentSetup.methodUIGuide') }}
                                <el-tag
                                  size="small"
                                  :type="agent.verified_ui ? 'success' : 'info'"
                                  effect="plain"
                                  class="method-status-tag"
                                >
                                  {{ agent.verified_ui ? $t('agentSetup.verified') : $t('agentSetup.unverified') }}
                                </el-tag>
                              </div>
                              <p class="access-method-hint">
                                {{ agent.ui_guide?.summary
                                  || (agent.verified_ui
                                    ? $t('agentSetup.methodUIGuideHint')
                                    : $t('agentSetup.methodUIGuideUnverifiedHint')) }}
                              </p>
                            </div>
                          </div>
                          <div class="access-method-actions">
                            <el-button type="primary" size="small" @click="openWizard(agent)">
                              {{ $t('agentSetup.uiGuideAction') }}
                            </el-button>
                          </div>
                        </div>
                      </div>

                      <!-- wrap CLI -->
                      <div
                        v-else-if="method === 'wrap_cli' && agent.wrap_command"
                        class="access-method"
                        :class="{ 'access-method--disabled': !wrapAvailable }"
                      >
                        <div class="access-method-head">
                          <div class="access-method-titles">
                            <span class="access-method-index">{{ idx + 1 }}</span>
                            <div>
                              <div class="access-method-title">
                                {{ $t('agentSetup.methodWrap') }}
                                <el-tag
                                  size="small"
                                  :type="agent.verified_wrap ? 'success' : 'info'"
                                  effect="plain"
                                  class="method-status-tag"
                                >
                                  {{ agent.verified_wrap ? $t('agentSetup.verified') : $t('agentSetup.unverified') }}
                                </el-tag>
                                <el-tag
                                  v-if="proxySetupLoaded"
                                  size="small"
                                  :type="wrapAvailable ? 'success' : 'warning'"
                                  effect="plain"
                                  class="method-status-tag"
                                >
                                  {{ wrapAvailable ? $t('agentSetup.wrapReady') : $t('agentSetup.wrapUnavailable') }}
                                </el-tag>
                              </div>
                              <p class="access-method-hint access-method-hint--multiline">
                                {{ wrapMethodHint(agent) }}
                              </p>
                              <p v-if="companionInstallHint(agent)" class="wrap-install-hint">
                                {{ $t('agentSetup.installCLIHint') }}：{{ companionInstallHint(agent) }}
                              </p>
                              <a
                                v-if="agent.companion_cli?.install_url"
                                class="wrap-dep-link"
                                :href="agent.companion_cli.install_url"
                                target="_blank"
                                rel="noopener noreferrer"
                              >{{ $t('agentSetup.installGuide') }}</a>
                              <p class="wrap-dep-line">
                                {{ $t('agentSetup.wrapDependsOnSystemProxy') }}
                                <a class="wrap-dep-link" href="/system-proxy" @click.prevent="goSystemProxy">
                                  {{ $t('agentSetup.goSystemProxyPage') }}
                                </a>
                              </p>
                            </div>
                          </div>
                        </div>
                        <div v-if="wrapAvailable" class="wrap-cmd-row">
                          <code class="wrap-cmd">{{ agent.wrap_command }}</code>
                          <el-button
                            class="wrap-copy-btn"
                            link
                            type="primary"
                            :icon="DocumentCopy"
                            :title="$t('agentSetup.copyWrapCommand')"
                            @click="copyWrapCommand(agent.wrap_command!)"
                          />
                        </div>
                        <div v-else class="wrap-unavailable">
                          <p class="wrap-unavailable-text">{{ wrapUnavailableReason }}</p>
                          <el-button type="warning" size="small" plain @click="goSystemProxy">
                            {{ $t('agentSetup.goEnableSystemProxy') }}
                          </el-button>
                        </div>
                      </div>

                      <!-- 内置 -->
                      <div v-else-if="method === 'builtin'" class="access-method">
                        <div class="access-method-head">
                          <div class="access-method-titles">
                            <span class="access-method-index">{{ idx + 1 }}</span>
                            <div>
                              <div class="access-method-title">{{ $t('agentSetup.methodBuiltin') }}</div>
                              <p class="access-method-hint">{{ $t('agentSetup.methodBuiltinHint') }}</p>
                            </div>
                          </div>
                          <el-button type="primary" size="small" @click="openWizard(agent)">
                            {{ $t('agentSetup.connectProxy') }}
                          </el-button>
                        </div>
                      </div>
                    </template>
                  </div>

                  <el-collapse class="agent-meta-collapse" @click.stop>
                    <el-collapse-item :title="$t('agentSetup.moreDetails')" name="details">
                      <div class="meta-block">
                        <div class="meta-row">
                          <span class="meta-label">{{ $t('agentSetup.writeMode') }}</span>
                          <span>{{ writeModeLabel(agent.write_mode) }}</span>
                        </div>
                        <div v-if="agent.install_url || agentLocalized(agent, 'installHint')" class="meta-row meta-row-col">
                          <span class="meta-label">{{ $t('agentSetup.installGuide') }}</span>
                          <a
                            v-if="agent.install_url"
                            class="install-link"
                            :href="agent.install_url"
                            target="_blank"
                            rel="noopener noreferrer"
                          >{{ agent.install_url }}</a>
                          <span v-if="agentLocalized(agent, 'installHint')" class="install-hint">
                            {{ agentLocalized(agent, 'installHint') }}
                          </span>
                        </div>
                        <div v-if="agent.config_paths?.length" class="meta-row meta-row-col">
                          <span class="meta-label">{{ $t('agentSetup.configPaths') }}</span>
                          <code v-for="p in agent.config_paths" :key="p" class="meta-path">{{ p }}</code>
                        </div>
                        <div v-if="agent.key_fields?.length" class="meta-row meta-row-col">
                          <span class="meta-label">{{ $t('agentSetup.keyFields') }}</span>
                          <span class="meta-fields">{{ agent.key_fields.join(', ') }}</span>
                        </div>
                        <p class="meta-method">{{ agentLocalized(agent, 'configMethod') || $t('agentSetup.noConfigMethod') }}</p>
                      </div>
                    </el-collapse-item>
                  </el-collapse>
                </div>
              </el-col>
            </el-row>
          </div>
        </div>
      </el-tab-pane>

      <!-- Tab 2: 供应商配置（仅管理员，只读视图） -->
      <el-tab-pane :label="$t('agentSetup.providerConfig')" name="providers" v-if="isAdmin">
        <el-card>
          <template #header>
            <div class="card-header">
              <span>{{ $t('agentSetup.builtinProviderRoute') }}</span>
            </div>
          </template>

          <el-table :data="providers" v-loading="loadingProviders" stripe>
            <el-table-column prop="agent_type" :label="$t('agentSetup.agentType')" width="150" />
            <el-table-column prop="display_name" :label="$t('agentSetup.displayName')" width="150" />
            <el-table-column prop="backend_id" :label="$t('agentSetup.backendId')" width="180">
              <template #default="{ row }">
                <el-tag v-if="row.backend_id" type="success">{{ row.backend_id }}</el-tag>
                <span v-else class="text-muted">{{ $t('agentSetup.default') }}</span>
              </template>
            </el-table-column>
            <el-table-column prop="pipeline_id" :label="$t('agentSetup.pipelineId')" width="180">
              <template #default="{ row }">
                <el-tag v-if="row.pipeline_id" type="warning">{{ row.pipeline_id }}</el-tag>
                <span v-else class="text-muted">{{ $t('agentSetup.default') }}</span>
              </template>
            </el-table-column>
            <el-table-column prop="model" :label="$t('agentSetup.modelOverride')" width="150">
              <template #default="{ row }">
                <span v-if="row.model">{{ row.model }}</span>
                <span v-else class="text-muted">-</span>
              </template>
            </el-table-column>
            <el-table-column prop="enabled" :label="$t('agentSetup.status')" width="80">
              <template #default="{ row }">
                <el-tag :type="row.enabled ? 'success' : 'info'" size="small">
                  {{ row.enabled ? $t('agentSetup.enabled') : $t('agentSetup.disabled') }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column :label="$t('agentSetup.actions')" width="120" fixed="right">
              <template #default="{ row }">
                <el-button size="small" type="primary" @click="handleHotSwap(row)">
                  {{ $t('agentSetup.setDefault') }}
                </el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-tab-pane>
    </el-tabs>

    <!-- 接入向导对话框 -->
    <el-dialog
      v-model="wizardVisible"
      :title="wizardTitle"
      width="720px"
      :close-on-click-modal="false"
      destroy-on-close
      class="agent-wizard-dialog"
      @closed="resetWizard"
    >
      <el-steps :active="wizardStep" finish-status="success" align-center class="wizard-steps">
        <el-step :title="$t('agentSetup.selectPipeline')" :description="$t('agentSetup.decideRouteAndModel')" />
        <el-step
          v-if="!isUIGuideOnlyWizard"
          :title="$t('agentSetup.writeConfig')"
          :description="$t('agentSetup.applyToAgent')"
        />
        <el-step
          :title="isUIGuideOnlyWizard ? $t('agentSetup.uiGuideFillParams') : $t('agentSetup.verify')"
          :description="isUIGuideOnlyWizard ? $t('agentSetup.uiGuideFillParamsDesc') : $t('agentSetup.confirmProxyAvailable')"
        />
      </el-steps>

      <!-- Step 1: 选择流水线（UI 指引时同页展示可复制参数） -->
      <div v-show="wizardStep === 0" class="wizard-step" v-loading="connectMode === 'proxy' ? loadingPipelines : loadingBackends">
        <el-alert type="info" :closable="false" show-icon class="step-alert">
          {{ connectMode === 'direct' ? $t('agentSetup.selectBackendHint') : (isUIGuideOnlyWizard ? $t('agentSetup.uiGuidePipelineHint') : $t('agentSetup.selectPipelineHint')) }}
        </el-alert>

        <div class="mode-switch">
          <el-radio-group v-model="connectMode">
            <el-radio-button label="proxy">{{ $t('agentSetup.modeProxy') }}</el-radio-button>
            <el-radio-button label="direct">{{ $t('agentSetup.modeDirect') }}</el-radio-button>
          </el-radio-group>
          <span class="mode-hint">{{ connectMode === 'direct' ? $t('agentSetup.modeDirectHint') : $t('agentSetup.modeProxyHint') }}</span>
        </div>

        <el-alert
          v-if="connectMode === 'proxy' && hasEnabledAPIKey === false"
          type="warning"
          :closable="false"
          show-icon
          class="step-alert"
        >
          <template #title>{{ $t('agentSetup.noApiKey') }}</template>
          <template v-if="isUIGuideOnlyWizard">
            {{ $t('agentSetup.uiGuideNeedApiKey') }}
          </template>
          <template v-else>
            {{ $t('agentSetup.apiKeyAutoWrite') }}
            {{ $t('agentSetup.apiKeyCreateFirst') }}
          </template>
          <div class="alert-actions">
            <el-button type="warning" size="small" @click="goProfileForAPIKey">{{ $t('agentSetup.goToCreateApiKey') }}</el-button>
          </div>
        </el-alert>
        <el-alert
          v-else-if="connectMode === 'proxy' && hasEnabledAPIKey === true && !isUIGuideOnlyWizard"
          type="success"
          :closable="false"
          show-icon
          class="step-alert"
        >
          {{ $t('agentSetup.apiKeyDetected') }}
        </el-alert>

        <!-- 代理模式：走 Centag 代理（含计量/配额/failover/语义缓存）。
             子模式二选一：选流水线（模型=centag/<流水线>）；或指定后端+模型（写入时同步为系统默认出站，
             替换透明流水线的 {{system.default_backend}} / {{system.default_model}}）。 -->
        <div v-if="connectMode === 'proxy'">
          <div class="mode-switch">
            <el-radio-group v-model="proxyTargetMode">
              <el-radio-button label="pipeline">{{ $t('agentSetup.proxyTargetPipeline') }}</el-radio-button>
              <el-radio-button label="backend">{{ $t('agentSetup.proxyTargetBackend') }}</el-radio-button>
            </el-radio-group>
            <span class="mode-hint">{{ proxyTargetMode === 'backend' ? $t('agentSetup.proxyTargetBackendHint') : $t('agentSetup.proxyTargetPipelineHint') }}</span>
          </div>

          <template v-if="proxyTargetMode === 'pipeline'">
            <div v-if="pipelines.length === 0 && !loadingPipelines" class="empty-pipelines">
              <el-empty :description="$t('agentSetup.noPipeline')">
                <el-button type="primary" @click="goPipelines">{{ $t('agentSetup.goToPolicy') }}</el-button>
              </el-empty>
            </div>
            <div v-else>
              <h4 class="select-label">{{ $t('agentSetup.selectPipelineLabel') }}</h4>
              <el-select
                v-model="selectedPipeline"
                :placeholder="$t('agentSetup.selectPipelinePlaceholder')"
                filterable
                fit-input-width
                style="width: 100%"
                @change="onPipelineChange"
              >
                <el-option
                  v-for="pipe in pipelines"
                  :key="pipe.id"
                  :label="pipe.name || pipe.id"
                  :value="pipe.id"
                >
                  <div class="pipeline-option">
                    <span>{{ pipe.name || pipe.id }}</span>
                    <span v-if="pipe.description" class="pipeline-desc">{{ pipe.description }}</span>
                  </div>
                </el-option>
              </el-select>

              <div v-if="selectedPipeline && !isUIGuideOnlyWizard" class="pipeline-summary">
                <el-tag type="success" size="default">{{ $t('agentSetup.pipeline') }} {{ selectedPipelineName }}</el-tag>
                <el-tag type="info" size="default">
                  <el-icon><Cpu /></el-icon>
                  {{ $t('agentSetup.modelRoute') }} centag/{{ selectedPipeline }}
                </el-tag>
                <span v-if="pipelineModel" class="model-hint">{{ $t('agentSetup.pipelineModelHint') }} {{ pipelineModel }}</span>
              </div>
            </div>
          </template>

          <template v-else>
            <h4 class="select-label">{{ $t('agentSetup.selectBackendLabel') }}</h4>
            <el-select
              v-model="selectedBackend"
              :placeholder="$t('agentSetup.selectBackendPlaceholder')"
              filterable
              fit-input-width
              style="width: 100%"
              @change="onBackendChange"
            >
              <el-option
                v-for="b in usableBackends"
                :key="b.id"
                :label="b.name || b.id"
                :value="b.id"
              >
                <div class="pipeline-option">
                  <span>{{ b.name || b.id }}</span>
                  <span v-if="b.base_url" class="pipeline-desc">{{ b.base_url }}</span>
                </div>
              </el-option>
            </el-select>

            <template v-if="selectedBackend">
              <h4 class="select-label">{{ $t('agentSetup.selectModelLabel') }}</h4>
              <el-select
                v-model="selectedModel"
                :placeholder="$t('agentSetup.selectModelPlaceholder')"
                filterable
                allow-create
                default-first-option
                fit-input-width
                style="width: 100%"
              >
                <el-option
                  v-for="m in backendModelOptions"
                  :key="m"
                  :label="m"
                  :value="m"
                />
              </el-select>
            </template>

            <div v-if="backends.length && !usableBackends.length" class="empty-pipelines">
              <el-empty :description="$t('agentSetup.noUsableBackend')" />
            </div>

            <div v-if="selectedBackend && !isUIGuideOnlyWizard" class="pipeline-summary">
              <el-tag type="success" size="default">{{ $t('agentSetup.backendTag') }} {{ selectedBackendName }}</el-tag>
              <el-tag type="info" size="default">
                <el-icon><Cpu /></el-icon>
                {{ $t('agentSetup.modelRoute') }} {{ selectedModel || $t('agentSetup.backendDefaultModel') }}
              </el-tag>
              <span class="model-hint">{{ $t('agentSetup.pinnedProxySummaryHint') }}</span>
            </div>
          </template>

          <!-- UI 指引：可复制填表参数（两种子模式共用；模型 ID 随子模式变化） -->
          <div v-if="isUIGuideOnlyWizard && uiGuideModelID" class="ui-params-panel">
              <div class="ui-params-header">
                <h4 class="ui-params-title">{{ $t('agentSetup.uiGuideParamsTitle') }}</h4>
                <p class="ui-params-summary">
                  {{ selectedAgentMeta?.ui_guide?.summary || $t('agentSetup.uiGuideParamsHint') }}
                </p>
              </div>
              <div class="ui-params-list">
                <div
                  v-for="row in uiGuideParamRows"
                  :key="row.label"
                  class="ui-param-item"
                >
                  <div class="ui-param-item-main">
                    <div class="ui-param-label">{{ row.label }}</div>
                    <div class="ui-param-body">
                      <code class="ui-param-value">{{ row.value }}</code>
                      <div class="ui-param-actions">
                        <el-button
                          v-if="row.copyable"
                          size="small"
                          :icon="DocumentCopy"
                          @click="copyText(row.value)"
                        >
                          {{ $t('agentSetup.copyValue') }}
                        </el-button>
                        <el-button
                          v-if="row.action === 'goto_api_key'"
                          size="small"
                          type="primary"
                          plain
                          @click="goProfileForAPIKey"
                        >
                          {{ $t('agentSetup.goCopyApiKey') }}
                        </el-button>
                      </div>
                    </div>
                  </div>
                  <p v-if="row.hint" class="ui-param-hint">{{ row.hint }}</p>
                </div>
              </div>
              <p v-if="selectedAgentMeta?.ui_guide?.restart_hint" class="ui-params-footer">
                {{ selectedAgentMeta.ui_guide.restart_hint }}
              </p>
            </div>
        </div>

        <!-- 直连模式：直接将已配置后端的真实地址与密钥写入 Agent（绕过 Centag 代理） -->
        <div v-else v-loading="loadingBackends">
          <el-alert type="warning" :closable="false" show-icon class="step-alert">
            {{ $t('agentSetup.directModeWarning') }}
          </el-alert>
          <h4 class="select-label">{{ $t('agentSetup.selectBackendLabel') }}</h4>
          <el-select
            v-model="selectedBackend"
            :placeholder="$t('agentSetup.selectBackendPlaceholder')"
            filterable
            fit-input-width
            style="width: 100%"
            @change="onBackendChange"
          >
            <el-option
              v-for="b in usableBackends"
              :key="b.id"
              :label="b.name || b.id"
              :value="b.id"
            >
              <div class="pipeline-option">
                <span>{{ b.name || b.id }}</span>
                <span v-if="b.base_url" class="pipeline-desc">{{ b.base_url }}</span>
              </div>
            </el-option>
          </el-select>

          <template v-if="selectedBackend">
            <h4 class="select-label">{{ $t('agentSetup.selectModelLabel') }}</h4>
            <el-select
              v-model="selectedModel"
              :placeholder="$t('agentSetup.selectModelPlaceholder')"
              filterable
              allow-create
              default-first-option
              fit-input-width
              style="width: 100%"
            >
              <el-option
                v-for="m in backendModelOptions"
                :key="m"
                :label="m"
                :value="m"
              />
            </el-select>
          </template>

          <div v-if="backends.length && !usableBackends.length" class="empty-pipelines">
            <el-empty :description="$t('agentSetup.noUsableBackend')" />
          </div>
        </div>
      </div>

      <!-- Step 2: 写入配置（仅 write_config Agent） -->
      <div v-show="!isUIGuideOnlyWizard && wizardStep === 1" class="wizard-step" v-loading="loadingConfig">
        <el-alert type="info" :closable="false" show-icon class="step-alert">
          {{ $t('agentSetup.writeConfigHint', { agent: currentAgentName }) }}
          {{ $t('agentSetup.writeConfigKeySource') }}
        </el-alert>

        <div v-if="selectedAgentMeta" class="wizard-meta">
          <div class="meta-row">
            <span class="meta-label">{{ $t('agentSetup.writeMode') }}</span>
            <el-tag size="small" type="info">{{ writeModeLabel(selectedAgentMeta.write_mode) }}</el-tag>
          </div>
          <div v-if="selectedAgentMeta.config_paths?.length" class="meta-row">
            <span class="meta-label">{{ $t('agentSetup.configPaths') }}</span>
            <code v-for="p in selectedAgentMeta.config_paths" :key="p" class="meta-path">{{ p }}</code>
          </div>
          <p class="meta-method">{{ agentLocalized(selectedAgentMeta, 'configMethod') }}</p>
          <a
            v-if="selectedAgentMeta.install_url"
            class="install-link"
            :href="selectedAgentMeta.install_url"
            target="_blank"
            rel="noopener noreferrer"
          >
            {{ $t('agentSetup.installGuide') }}
          </a>
          <div v-if="agentLocalized(selectedAgentMeta, 'installHint')" class="install-hint">
            {{ agentLocalized(selectedAgentMeta, 'installHint') }}
          </div>
        </div>

        <div v-if="configResult" class="config-result">
          <div class="config-header">
            <h3>{{ configResult.description }}</h3>
            <el-tag>{{ $t('agentSetup.route') }} {{ configResult.backend_name }}</el-tag>
          </div>

          <div class="config-section">
            <h4>{{ $t('agentSetup.oneClickConfig') }}</h4>
            <el-button type="primary" :loading="writingConfig" @click="writeToConfig">
              <el-icon class="el-icon--left"><Plus /></el-icon>
              {{ $t('agentSetup.writeConfigFile') }}
            </el-button>
            <p class="write-hint" v-if="writeResult">
              <span v-if="writeResult.success" style="color: #67c23a">✓ {{ writeResult.message }}</span>
              <span v-else style="color: #f56c6c">✗ {{ writeResult.message }}</span>
            </p>
            <el-alert
              v-if="writeResult?.success && (writeResult.restart_required || selectedAgentMeta?.category === 'desktop')"
              type="warning"
              :closable="false"
              show-icon
              class="step-alert"
              style="margin-top: 12px"
              :title="$t('agentSetup.restartClientTitle')"
            >
              {{ $t('agentSetup.restartClientHint') }}
            </el-alert>
          </div>

          <div v-if="writeSucceeded && writePreviewFiles.length" class="config-section">
            <h4>{{ $t('agentSetup.configWritten') }}</h4>
            <el-collapse>
              <el-collapse-item
                v-for="file in writePreviewFiles"
                :key="file.path"
                :title="file.path"
              >
                <div class="code-block">
                  <pre><code>{{ file.preview }}</code></pre>
                  <el-button class="copy-btn" :icon="DocumentCopy" @click="copyText(file.preview)" />
                </div>
              </el-collapse-item>
            </el-collapse>
          </div>

          <template v-if="!writeSucceeded">
            <div v-if="sanitizedConfigFiles.length" class="config-section">
              <h4>{{ $t('agentSetup.configPreview') }}</h4>
              <p class="section-subhint">{{ $t('agentSetup.configPreviewHint') }}</p>
              <el-collapse>
                <el-collapse-item
                  v-for="file in sanitizedConfigFiles"
                  :key="file.path"
                  :title="file.path"
                >
                  <div class="code-block">
                    <pre><code>{{ file.preview }}</code></pre>
                    <el-button class="copy-btn" :icon="DocumentCopy" @click="copyText(file.preview)" />
                  </div>
                </el-collapse-item>
              </el-collapse>
            </div>

            <div v-if="!isDesktopEdition && configResult.commands" class="config-section">
              <h4>{{ $t('agentSetup.configCommand') }}</h4>
              <el-tabs v-model="platformTab" type="border-card">
                <el-tab-pane label="macOS" name="macos">
                  <div class="code-block">
                    <pre><code>{{ configResult.commands.macos }}</code></pre>
                    <el-button class="copy-btn" :icon="DocumentCopy" @click="copyText(configResult.commands.macos)" />
                  </div>
                </el-tab-pane>
                <el-tab-pane label="Linux" name="linux">
                  <div class="code-block">
                    <pre><code>{{ configResult.commands.linux }}</code></pre>
                    <el-button class="copy-btn" :icon="DocumentCopy" @click="copyText(configResult.commands.linux)" />
                  </div>
                </el-tab-pane>
                <el-tab-pane label="Windows" name="windows">
                  <div class="code-block">
                    <pre><code>{{ configResult.commands.windows }}</code></pre>
                    <el-button class="copy-btn" :icon="DocumentCopy" @click="copyText(configResult.commands.windows)" />
                  </div>
                </el-tab-pane>
              </el-tabs>
            </div>
          </template>
        </div>
        <el-empty v-else-if="!loadingConfig" :description="$t('agentSetup.configGenFailed')" />
      </div>

      <!-- 最后一步：验证 -->
      <div v-show="wizardStep === wizardLastStep" class="wizard-step">
        <el-result
          icon="success"
          :title="isUIGuideOnlyWizard ? $t('agentSetup.uiGuideParamsReady') : $t('agentSetup.configReady')"
          :sub-title="$t('agentSetup.verifyHint', { agent: currentAgentName })"
        />

        <el-alert type="success" :closable="false" show-icon class="step-alert">
          {{ isUIGuideOnlyWizard ? $t('agentSetup.uiGuideVerifyHint') : $t('agentSetup.verifySuccessHint') }}
        </el-alert>

        <div v-if="!isUIGuideOnlyWizard && configResult?.verify_cmd" class="config-section">
          <h4>{{ $t('agentSetup.verifyCommand') }}</h4>
          <p class="verify-desc">{{ $t('agentSetup.verifyCommandHint') }}</p>
          <div class="code-block">
            <pre><code>{{ configResult.verify_cmd }}</code></pre>
            <el-button class="copy-btn" :icon="DocumentCopy" @click="copyText(configResult.verify_cmd)" />
          </div>
        </div>

        <div class="config-section">
          <h4>{{ $t('agentSetup.verifyChecklist') }}</h4>
          <ol class="verify-checklist">
            <li v-if="isUIGuideOnlyWizard">{{ $t('agentSetup.uiGuideVerifyStep1') }}</li>
            <li v-else>{{ $t('agentSetup.verifyChecklistStep1', { agent: currentAgentName }) }}</li>
            <li>{{ $t('agentSetup.verifyChecklistStep2') }}</li>
            <li v-if="connectMode === 'direct'">{{ $t('agentSetup.verifyChecklistStep3Direct', { backend: selectedBackendName }) }}</li>
            <li v-else-if="connectMode === 'proxy' && proxyTargetMode === 'backend'">{{ $t('agentSetup.verifyChecklistStep3Pinned', { backend: selectedBackendName, model: selectedModel || t('agentSetup.backendDefaultModel') }) }}</li>
            <li v-else>{{ $t('agentSetup.verifyChecklistStep3', { pipeline: selectedPipeline }) }}</li>
            <li>{{ $t('agentSetup.verifyChecklistStep4') }}</li>
          </ol>
        </div>
      </div>

      <template #footer>
        <div class="wizard-footer">
          <el-button @click="wizardVisible = false">{{ $t('agentSetup.cancel') }}</el-button>
          <div class="wizard-footer-right">
            <el-button :disabled="wizardStep === 0 || loadingConfig || writingConfig" @click="prevStep">
              {{ $t('agentSetup.prevStep') }}
            </el-button>
            <el-button
              v-if="wizardStep < wizardLastStep"
              type="primary"
              :loading="loadingConfig"
              :disabled="!canGoNext"
              @click="nextStep"
            >
              {{ $t('agentSetup.nextStep') }}
            </el-button>
            <el-button v-else type="primary" @click="wizardVisible = false">
              {{ $t('agentSetup.finish') }}
            </el-button>
          </div>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useRouter } from 'vue-router'
import {
  Link, DocumentCopy, Plus, Search,
  Monitor, ChatDotRound, DataLine, Connection, Cpu
} from '@element-plus/icons-vue'
import { isPersonalEdition } from '@/utils/edition'
import { resolveApiBaseUrl } from '@/utils/apiBaseUrl'
import { copyToClipboard } from '@/utils/clipboard'
import { useAuthStore } from '@/stores/auth'
import { listAPIKeys } from '@/api/user'
import { getProxySetupStatus, type ProxySetupStatus } from '@/api/system-proxy'
import api from '@/api'

const { t, te } = useI18n()
const router = useRouter()
const authStore = useAuthStore()
const isAdmin = computed(() => authStore.isAdmin)

const activeTab = ref('setup')

/** 系统默认流水线：透明模式（transparent 为合并后的现行 ID，旧名 transparent-proxy 已废弃） */
const DEFAULT_PIPELINE_ID = 'transparent'

// ===================== 快速接入向导 =====================

const wizardVisible = ref(false)
const wizardStep = ref(0)
const selectedAgent = ref('')
const selectedAgentDisplay = ref('')
const selectedPipeline = ref('')
const pipelineModel = ref('')
const loadingPipelines = ref(false)
const loadingBackends = ref(false)
/** 接入模式：proxy=走 Centag 代理；direct=直连已配置后端（绕过代理） */
const connectMode = ref<'proxy' | 'direct'>('proxy')
/** 代理子模式（互斥）：pipeline=选流水线；backend=指定后端+模型（写入时同步为系统默认出站） */
const proxyTargetMode = ref<'pipeline' | 'backend'>('pipeline')
const selectedBackend = ref('')
const selectedModel = ref('')
const backends = ref<Array<{
  id: string
  name: string
  base_url: string
  enabled: boolean
  has_api_key: boolean
  supported_models?: Array<{ requested_model?: string; actual_model?: string }>
}>>([])
const loadingConfig = ref(false)
const writingConfig = ref(false)
const restoringAgent = ref('')
const platformTab = ref('macos')
const filterText = ref('')
interface CompanionCLIInfo {
  binary?: string
  argv?: string[]
  install_url?: string
  install_hint?: string
  note?: string
}

interface UIGuideInfo {
  title?: string
  summary?: string
  doc_url?: string
  steps?: string[]
  fields?: Array<{ label: string; value: string; hint?: string }>
  /** openai_base = …/v1（UI 指引默认）；chat_completions 仅兼容旧数据 */
  request_url_kind?: 'openai_base' | 'chat_completions' | string
  /** on / off；空则不展示「完整 URL」行 */
  full_url_mode?: 'on' | 'off' | string
  url_hint?: string
  export_hint?: string
  restart_hint?: string
}

interface AgentTypeInfo {
  type: string
  display_name: string
  description: string
  category?: string
  vendor?: string
  write_mode?: string
  config_paths?: string[]
  key_fields?: string[]
  config_method?: string
  install_url?: string
  install_hint?: string
  access_methods?: string[]
  companion_cli?: CompanionCLIInfo
  ui_guide?: UIGuideInfo
  verified?: boolean
  verified_write?: boolean
  verified_wrap?: boolean
  verified_ui?: boolean
  wrap_command?: string
  guide_only?: boolean
}

const agentTypes = ref<AgentTypeInfo[]>([])
const pipelines = ref<Array<{ id: string; name: string; description?: string; nodes?: any[] }>>([])
const configResult = ref<any>(null)
const writeResult = ref<{
  success: boolean
  message: string
  written?: Array<{ path: string; content: string }>
  restart_required?: boolean
} | null>(null)
/** null=未检测；true/false=是否有启用中的 API Key（列表级提示，最终以服务端解密结果为准） */
const hasEnabledAPIKey = ref<boolean | null>(null)

/** 系统代理 setup/status：wrap 依赖 MITM 已启 + 出口 Key 已配置 */
const proxySetup = ref<ProxySetupStatus | null>(null)
const proxySetupLoaded = ref(false)
const wrapAvailable = computed(() =>
  !!proxySetup.value?.mitm_enabled && !!proxySetup.value?.egress_api_key_configured
)
const wrapUnavailableReason = computed(() => {
  if (!proxySetupLoaded.value) return t('agentSetup.wrapChecking')
  if (!proxySetup.value) return t('agentSetup.wrapStatusUnknown')
  const mitm = !!proxySetup.value.mitm_enabled
  const egress = !!proxySetup.value.egress_api_key_configured
  if (!mitm && !egress) return t('agentSetup.wrapNeedMitmAndEgress')
  if (!mitm) return t('agentSetup.wrapNeedMitm')
  if (!egress) return t('agentSetup.wrapNeedEgress')
  return t('agentSetup.wrapUnavailable')
})

/** 直连模式：仅可选启用且已配置密钥的后端 */
const usableBackends = computed(() =>
  backends.value.filter(b => b.enabled && b.has_api_key)
)
/** 直连模式：当前后端支持的模型列表（用于模型下拉；留空则用后端默认模型） */
const backendModelOptions = computed(() => {
  const b = backends.value.find(x => x.id === selectedBackend.value)
  if (!b?.supported_models?.length) return []
  const set = new Set<string>()
  for (const m of b.supported_models) {
    const name = m.actual_model || m.requested_model
    if (name) set.add(name)
  }
  return Array.from(set)
})

interface AgentGroup {
  id: string
  title: string
  hint: string
  agents: AgentTypeInfo[]
}

function agentVerifyScore(a: AgentTypeInfo): number {
  let n = 0
  if (a.verified_write) n += 2
  if (a.verified_ui) n += 2
  if (a.verified_wrap) n += 1
  return n
}

function agentHasAnyVerified(a: AgentTypeInfo): boolean {
  return !!(a.verified_write || a.verified_wrap || a.verified_ui || a.verified)
}

/** 优先用后端 access_methods；旧接口按 write_mode / wrap 兼容推导 */
function agentAccessMethods(agent: AgentTypeInfo | null | undefined): string[] {
  if (!agent) return []
  if (agent.access_methods?.length) return agent.access_methods
  const methods: string[] = []
  if (agent.write_mode && agent.write_mode !== 'none') methods.push('write_config')
  if (agent.wrap_command) methods.push('wrap_cli')
  if (!methods.length) methods.push('builtin')
  return methods
}

function agentHasAccess(agent: AgentTypeInfo | null | undefined, method: string): boolean {
  return agentAccessMethods(agent).includes(method)
}

function companionInstallHint(agent: AgentTypeInfo): string {
  return agent.companion_cli?.install_hint || agent.install_hint || ''
}

/** 品牌分组顺序 */
const vendorOrder = [
  'Anthropic', 'OpenAI', 'Google', 'xAI',
  '腾讯云', '字节跳动',
  'OpenCode', 'OpenClaw', 'Pi', 'Hermes',
]

/** 按品牌分组；隐藏内置 agent（tui/web）；支持搜索过滤 */
const agentGroups = computed<AgentGroup[]>(() => {
  const q = filterText.value.trim().toLowerCase()

  const sortInGroup = (agents: AgentTypeInfo[]) =>
    [...agents].sort((a, b) => {
      const d = agentVerifyScore(b) - agentVerifyScore(a)
      if (d !== 0) return d
      return a.type.localeCompare(b.type)
    })

  // 过滤：排除内置 agent，按关键字搜索
  const filtered = agentTypes.value.filter(a => {
    if (a.category === 'tui' || a.category === 'web') return false
    if (!q) return true
    const name = (a.display_name || a.type || '').toLowerCase()
    const desc = (a.description || '').toLowerCase()
    const vendor = (a.vendor || '').toLowerCase()
    return name.includes(q) || desc.includes(q) || vendor.includes(q)
  })

  const groups: AgentGroup[] = []
  const knownVendors = new Set(vendorOrder)

  for (const v of vendorOrder) {
    const agents = sortInGroup(filtered.filter(a => a.vendor === v))
    if (!agents.length) continue
    groups.push({ id: v, title: v, hint: '', agents })
  }

  const other = sortInGroup(
    filtered.filter(a => !knownVendors.has(a.vendor || ''))
  )
  if (other.length) {
    groups.push({
      id: 'other',
      title: t('agentSetup.groupOther'),
      hint: t('agentSetup.groupOtherHint'),
      agents: other,
    })
  }
  return groups
})

const isDesktopEdition = computed(() => isPersonalEdition())

const currentAgentName = computed(() => selectedAgentDisplay.value || selectedAgent.value || 'Agent')

const selectedAgentMeta = computed(() =>
  agentTypes.value.find(a => a.type === selectedAgent.value) || null
)

/** 仅 UI 指引、无写配置：向导只展示可复制参数 */
const isUIGuideOnlyWizard = computed(() => {
  const meta = selectedAgentMeta.value
  if (!meta) return false
  if (meta.guide_only) return true
  return agentHasAccess(meta, 'ui_guide') && !agentHasAccess(meta, 'write_config')
})

const wizardTitle = computed(() => {
  if (!selectedAgentDisplay.value) return t('agentSetup.connectCentagProxy')
  if (isUIGuideOnlyWizard.value) {
    return `${t('agentSetup.uiGuideAction')} — ${selectedAgentDisplay.value}`
  }
  return `${t('agentSetup.connectCentagProxy')} — ${selectedAgentDisplay.value}`
})

const wizardLastStep = computed(() => (isUIGuideOnlyWizard.value ? 1 : 2))

/** UI 指引统一推荐 OpenAI base …/v1（不带 /chat/completions） */
const uiGuideRequestURL = computed(() => {
  const origin = resolveApiBaseUrl({
    host: typeof window !== 'undefined' ? window.location.hostname : '127.0.0.1',
    port: 20060,
  }).replace(/\/$/, '')
  return `${origin}/v1`
})

/** UI 指引模型 ID：流水线方式=centag/<流水线>；指定后端和模型=真实模型名 */
const uiGuideModelID = computed(() => {
  if (connectMode.value === 'proxy' && proxyTargetMode.value === 'backend') {
    const model = selectedModel.value || backendModelOptions.value[0] || ''
    return selectedBackend.value ? `${selectedBackend.value}/${model}` : model
  }
  return selectedPipeline.value ? `centag/${selectedPipeline.value}` : ''
})

interface UIGuideParamRow {
  label: string
  value: string
  copyable?: boolean
  hint?: string
  action?: 'goto_api_key'
}

const uiGuideParamRows = computed((): UIGuideParamRow[] => {
  const pinnedBackendSelected = connectMode.value === 'proxy' && proxyTargetMode.value === 'backend' && !!selectedBackend.value
  if (!selectedPipeline.value && !pinnedBackendSelected) return []
  const guide = selectedAgentMeta.value?.ui_guide
  const rows: UIGuideParamRow[] = [
    { label: t('agentSetup.uiParamApiFormat'), value: 'OpenAI', copyable: true },
  ]
  const fullMode = guide?.full_url_mode
  if (fullMode === 'on' || fullMode === 'off') {
    rows.push({
      label: t('agentSetup.uiParamFullURL'),
      value: fullMode === 'on' ? t('agentSetup.uiParamFullURLOn') : t('agentSetup.uiParamFullURLOff'),
    })
  }
  rows.push({
    label: t('agentSetup.uiParamRequestURLBase'),
    value: uiGuideRequestURL.value,
    copyable: true,
    hint: guide?.url_hint || t('agentSetup.uiParamRequestURLHint'),
  })
  rows.push(
    { label: t('agentSetup.uiParamModelID'), value: uiGuideModelID.value, copyable: true },
    {
      label: t('agentSetup.uiParamAPIKey'),
      value: t('agentSetup.uiParamAPIKeyHint'),
      action: 'goto_api_key',
    },
  )
  return rows
})

const selectedPipelineName = computed(() => {
  const pipe = pipelines.value.find(p => p.id === selectedPipeline.value)
  return pipe?.name || selectedPipeline.value
})

const selectedBackendName = computed(() => {
  const b = backends.value.find(x => x.id === selectedBackend.value)
  return b?.name || selectedBackend.value || ''
})

const writeSucceeded = computed(() => !!writeResult.value?.success)

const canGoNext = computed(() => {
  if (wizardStep.value === 0) {
    if (connectMode.value === 'direct') {
      if (!selectedBackend.value || usableBackends.value.length === 0) return false
      return true
    }
    if (proxyTargetMode.value === 'backend') {
      if (!selectedBackend.value || usableBackends.value.length === 0) return false
      if (isUIGuideOnlyWizard.value) return true
      return hasEnabledAPIKey.value !== false
    }
    if (!selectedPipeline.value || pipelines.value.length === 0) return false
    // UI 指引：Key 在客户端填写，不强制本机已有可解密 Key
    if (isUIGuideOnlyWizard.value) return true
    return hasEnabledAPIKey.value !== false
  }
  if (!isUIGuideOnlyWizard.value && wizardStep.value === 1) {
    return !!configResult.value && !loadingConfig.value
  }
  return true
})

const writePreviewFiles = computed(() => {
  if (!writeResult.value?.success || !Array.isArray(writeResult.value.written)) return []
  return writeResult.value.written
    .filter((f: any) => f?.path && typeof f.content === 'string')
    .map((f: any) => ({
      path: f.path,
      preview: buildSanitizedConfigPreview(f.content),
    }))
})

const sanitizedConfigFiles = computed(() => {
  if (!Array.isArray(configResult.value?.files)) return []
  return configResult.value.files
    .filter((f: any) => f?.path && typeof f.content === 'string')
    .map((f: any) => ({
      path: f.path,
      preview: buildSanitizedConfigPreview(f.content),
    }))
})

async function loadAgentTypes() {
  try {
    const res: any = await api.get('/api/v1/agent/types')
    agentTypes.value = res.agent_types || []
  } catch (e: any) {
    ElMessage.error(t('agentSetup.loadAgentTypeFailed') + e.message)
  }
}

function pickDefaultPipelineID(list: Array<{ id: string; name: string }>): string {
  const byID = list.find(p => p.id === DEFAULT_PIPELINE_ID)
  if (byID) return byID.id
  const byName = list.find(p => p.name === 'transparent' || /transparent/i.test(p.id))
  return byName?.id || ''
}

async function loadPipelines() {
  loadingPipelines.value = true
  try {
    const res: any = await api.get('/api/v1/pipelines')
    const data = res?.data || res
    pipelines.value = Array.isArray(data) ? data : []
    if (!selectedPipeline.value) {
      const defaultID = pickDefaultPipelineID(pipelines.value)
      if (defaultID) {
        selectedPipeline.value = defaultID
        onPipelineChange(defaultID)
      }
    }
  } catch (e: any) {
    pipelines.value = []
    console.warn(t('agentSetup.loadPipelineFailed'), e.message)
  } finally {
    loadingPipelines.value = false
  }
}

async function loadBackends() {
  loadingBackends.value = true
  try {
    const res: any = await api.get('/api/v1/backends')
    const data = res?.data || res
    const list = Array.isArray(data?.backends)
      ? data.backends
      : (Array.isArray(data) ? data : [])
    backends.value = list
    if (!selectedBackend.value && usableBackends.value.length) {
      selectedBackend.value = usableBackends.value[0].id
      onBackendChange(selectedBackend.value)
    }
  } catch (e: any) {
    backends.value = []
    console.warn(t('agentSetup.loadBackendFailed'), e.message)
  } finally {
    loadingBackends.value = false
  }
}

function onBackendChange(id: string) {
  selectedModel.value = ''
  configResult.value = null
  writeResult.value = null
}

async function checkAPIKeys() {
  try {
    const keys = await listAPIKeys()
    hasEnabledAPIKey.value = Array.isArray(keys) && keys.some(k => k.enabled)
  } catch (e: any) {
    hasEnabledAPIKey.value = null
    console.warn(t('agentSetup.detectApiKeyFailed'), e.message)
  }
}

function goProfileForAPIKey() {
  wizardVisible.value = false
  router.push({ path: '/profile', query: { section: 'api-keys' } })
}

async function restoreDefaults(agent: { type: string; display_name: string }) {
  try {
    await ElMessageBox.confirm(
      t('agentSetup.restoreConfirm', { name: agent.display_name }),
      t('agentSetup.restoreDefaultConfig'),
      {
        confirmButtonText: t('agentSetup.restoreDefault'),
        cancelButtonText: t('agentSetup.cancel'),
        type: 'warning',
      }
    )
  } catch {
    return
  }

  restoringAgent.value = agent.type
  try {
    const res: any = await api.post('/api/v1/agent/configs/restore', {
      agent_type: agent.type,
    })
    if (res?.success) {
      ElMessage.success(res.message || t('agentSetup.restoreSuccess'))
    } else {
      ElMessage.error(res?.message || t('agentSetup.restoreFailed'))
    }
  } catch (e: any) {
    ElMessage.error(t('agentSetup.restoreFailed') + e.message)
  } finally {
    restoringAgent.value = ''
  }
}

function extractPipelineModel(pipe: any): string {
  if (!pipe?.nodes?.length) return ''
  for (const node of pipe.nodes) {
    const model = node?.config?.model || node?.model
    if (model) return model
  }
  return ''
}

function onPipelineChange(pipeId: string) {
  if (!pipeId) {
    pipelineModel.value = ''
    return
  }
  const pipe = pipelines.value.find(p => p.id === pipeId)
  pipelineModel.value = pipe ? extractPipelineModel(pipe) : ''
  configResult.value = null
  writeResult.value = null
}

function openWizard(agent: { type: string; display_name: string }) {
  selectedAgent.value = agent.type
  selectedAgentDisplay.value = agent.display_name
  wizardStep.value = 0
  connectMode.value = 'proxy'
  proxyTargetMode.value = 'pipeline'
  selectedPipeline.value = ''
  pipelineModel.value = ''
  selectedBackend.value = ''
  selectedModel.value = ''
  configResult.value = null
  writeResult.value = null
  hasEnabledAPIKey.value = null
  wizardVisible.value = true
  loadPipelines()
  loadBackends()
  checkAPIKeys()
}

function resetWizard() {
  wizardStep.value = 0
  selectedAgent.value = ''
  selectedAgentDisplay.value = ''
  connectMode.value = 'proxy'
  proxyTargetMode.value = 'pipeline'
  selectedPipeline.value = ''
  pipelineModel.value = ''
  selectedBackend.value = ''
  selectedModel.value = ''
  configResult.value = null
  writeResult.value = null
  hasEnabledAPIKey.value = null
}

function goPipelines() {
  wizardVisible.value = false
  router.push('/pipelines')
}

async function nextStep() {
  if (wizardStep.value === 0) {
    if (!connectSelectionReady()) return
    if (isUIGuideOnlyWizard.value) {
      wizardStep.value = 1
      return
    }
    const ok = await generateConfig()
    if (!ok) return
    wizardStep.value = 1
    return
  }
  if (!isUIGuideOnlyWizard.value && wizardStep.value === 1 && configResult.value) {
    wizardStep.value = 2
  }
}

function prevStep() {
  if (wizardStep.value > 0) {
    wizardStep.value -= 1
  }
}

function buildSanitizedConfigPreview(content: string): string {
  const masked = content
    .replace(/llmproxy_[a-zA-Z0-9]+/g, 'llmproxy_***')
    .replace(/("api[_-]?key"\s*:\s*")[^"]*(")/gi, '$1***$2')
    .replace(/(api[_-]?key\s*:\s*")[^"]*(")/gi, '$1***$2')
    .replace(/(api[_-]?key\s*=\s*)[^\n]*/gi, '$1***')
  const lines = masked.split('\n')
  const maxLines = 28
  if (lines.length <= maxLines) return masked
  return `${lines.slice(0, maxLines).join('\n')}\n...`
}

function isMissingProxyAPIKeyError(message: string): boolean {
  return message.includes(t('agentSetup.noApiKeyForAgent'))
}

async function maybeHandleMissingProxyAPIKeyError(message: string): Promise<boolean> {
  if (!isMissingProxyAPIKeyError(message)) return false
  try {
    await ElMessageBox.confirm(
      t('agentSetup.noApiKeyHint'),
      t('agentSetup.missingApiKey'),
      {
        confirmButtonText: t('agentSetup.goToProfile'),
        cancelButtonText: t('agentSetup.later'),
        type: 'warning',
      }
    )
    wizardVisible.value = false
    router.push({ path: '/profile', query: { section: 'api-keys' } })
  } catch {
    // 用户取消时不做跳转
  }
  return true
}

/** 当前接入选择是否完整（direct=后端；proxy+backend=后端；proxy+pipeline=流水线） */
function connectSelectionReady(): boolean {
  if (!selectedAgent.value) return false
  if (connectMode.value === 'direct') return !!selectedBackend.value
  if (proxyTargetMode.value === 'backend') return !!selectedBackend.value
  return !!selectedPipeline.value
}

/**
 * 组装 generate/write 请求：
 * - direct：backend_id(+model)，写真实地址与密钥；
 * - proxy+pipeline：pipeline_id，模型 centag/<流水线>；
 * - proxy+backend（经代理钉死默认出站）：backend_id(+model)+via_proxy，
 *   Agent 配置写 Centag 地址、代理 Key 与真实模型名；服务端把所选后端/模型
 *   落盘为系统默认（透明流水线兜底出站），备用配置不变。
 */
function buildConnectPayload(): Record<string, any> {
  const payload: Record<string, any> = { agent_type: selectedAgent.value }
  if (connectMode.value === 'direct') {
    payload.backend_id = selectedBackend.value
    if (selectedModel.value) payload.model = selectedModel.value
    return payload
  }
  if (proxyTargetMode.value === 'backend' && selectedBackend.value) {
    payload.backend_id = selectedBackend.value
    payload.via_proxy = true
    if (selectedModel.value) payload.model = selectedModel.value
    return payload
  }
  payload.pipeline_id = selectedPipeline.value || DEFAULT_PIPELINE_ID
  return payload
}

async function generateConfig(): Promise<boolean> {
  if (!connectSelectionReady()) return false
  loadingConfig.value = true
  configResult.value = null
  writeResult.value = null
  try {
    const res: any = await api.post('/api/v1/agent/configs/generate', buildConnectPayload())
    configResult.value = res
    return true
  } catch (e: any) {
    if (await maybeHandleMissingProxyAPIKeyError(e.message)) return false
    ElMessage.error(t('agentSetup.genConfigFailed') + e.message)
    return false
  } finally {
    loadingConfig.value = false
  }
}

async function writeToConfig() {
  if (!connectSelectionReady()) return
  writingConfig.value = true
  writeResult.value = null
  try {
    const res: any = await api.post('/api/v1/agent/configs/write', buildConnectPayload())
    writeResult.value = res
    if (res.success) {
      ElMessage.success(res.message || t('agentSetup.configWriteSuccess'))
      const needRestart =
        !!res.restart_required || selectedAgentMeta.value?.category === 'desktop'
      if (needRestart) {
        await ElMessageBox.alert(
          res.message || t('agentSetup.restartClientHint'),
          t('agentSetup.restartClientTitle'),
          { type: 'warning', confirmButtonText: t('agentSetup.gotIt') }
        ).catch(() => { /* closed */ })
      }
    } else {
      ElMessage.error(t('agentSetup.configWriteFailed') + res.message)
    }
  } catch (e: any) {
    writeResult.value = { success: false, message: e.message }
    if (await maybeHandleMissingProxyAPIKeyError(e.message)) return
    ElMessage.error(t('agentSetup.writeRequestFailed') + e.message)
  } finally {
    writingConfig.value = false
  }
}

function copyText(text: string) {
  copyToClipboard(text).then(ok => {
    if (ok) {
      ElMessage.success(t('agentSetup.copiedToClipboard'))
    } else {
      ElMessage.error(t('agentSetup.copyFailed'))
    }
  })
}

function copyWrapCommand(text: string) {
  if (!wrapAvailable.value) {
    ElMessage.warning(wrapUnavailableReason.value)
    return
  }
  copyText(text)
}

/** 桌面 Agent 的 wrap 启动的是配套 CLI，不是 .app；个别 Agent（如 OpenClaw）可有专用说明 */
function wrapMethodHint(agent: AgentTypeInfo): string {
  if (agent.type) {
    const key = `agentSetup.agents.${agent.type}.wrapHint`
    try {
      if (te(key) || te(key, 'en')) return t(key)
    } catch {
      /* fall through */
    }
  }
  if (agent.companion_cli?.note) {
    return agent.companion_cli.note
  }
  if (agent.category === 'desktop') {
    return agent.verified_wrap
      ? t('agentSetup.methodWrapDesktopCLIHint')
      : t('agentSetup.methodWrapDesktopCLIUnverifiedHint')
  }
  return agent.verified_wrap
    ? t('agentSetup.methodWrapHint')
    : t('agentSetup.methodWrapUnverifiedHint')
}

function goSystemProxy() {
  router.push('/system-proxy')
}

async function loadProxySetupStatus() {
  proxySetupLoaded.value = false
  try {
    proxySetup.value = await getProxySetupStatus()
  } catch {
    proxySetup.value = null
  } finally {
    proxySetupLoaded.value = true
  }
}

function agentIcon(type: string) {
  const map: Record<string, any> = {
    'claude-code': ChatDotRound,
    'claude-desktop': ChatDotRound,
    'codex': Monitor,
    'gemini-cli': DataLine,
    'grok-build': Connection,
    'opencode': Connection,
    'openclaw': Connection,
    'pi': Connection,
    'hermes': Connection,
    'codebuddy': Monitor,
    'workbuddy': Monitor,
    'trae': ChatDotRound,
  }
  return map[type] || Connection
}

function writeModeLabel(mode?: string): string {
  switch (mode) {
    case 'merge':
      return t('agentSetup.writeModeMerge')
    case 'none':
      return t('agentSetup.writeModeNone')
    case 'overwrite':
      return t('agentSetup.writeModeOverwrite')
    default:
      return mode || '-'
  }
}

/** 卡片/向导文案走前端 i18n；路径与 install_url 仍用后端。 */
function agentLocalized(
  agent: { type?: string; description?: string; config_method?: string; install_hint?: string; config_paths?: string[] } | null,
  field: 'description' | 'configMethod' | 'installHint'
): string {
  if (!agent?.type) return ''
  if (field === 'configMethod' && agent.type === 'claude-desktop' && (!agent.config_paths || agent.config_paths.length === 0)) {
    const unsupportedKey = 'agentSetup.agents.claude-desktop.configMethodUnsupported'
    if (te(unsupportedKey) || te(unsupportedKey, 'en')) return t(unsupportedKey)
  }
  const key = `agentSetup.agents.${agent.type}.${field}`
  // 当前语言缺失时回退 en，避免显示后端硬编码中文。
  // vue-i18n 会把未转义的 @ / | 当成链接/复数语法，解析失败时回退后端文案。
  try {
    if (te(key) || te(key, 'en')) return t(key)
  } catch {
    /* fall through */
  }
  if (field === 'description') return agent.description || ''
  if (field === 'configMethod') return agent.config_method || ''
  return agent.install_hint || ''
}

// ===================== 供应商配置 (只读视图) =====================

interface AgentProviderConfig {
  id: string
  agent_type: string
  display_name: string
  backend_id: string
  pipeline_id: string
  model: string
  api_key: string
  enabled: boolean
  description: string
}

const loadingProviders = ref(false)
const providers = ref<AgentProviderConfig[]>([])

async function loadProviders() {
  loadingProviders.value = true
  try {
    const res: any = await api.get('/api/v1/agent-providers')
    providers.value = res.agent_providers || []
  } catch (e: any) {
    ElMessage.error(t('agentSetup.loadProviderFailed') + e.message)
  } finally {
    loadingProviders.value = false
  }
}

async function handleHotSwap(provider: AgentProviderConfig) {
  try {
    await api.post(`/api/v1/agent-providers/${provider.id}/hotswap`, {
      agent_type: provider.agent_type,
      backend_id: provider.backend_id,
    })
    ElMessage.success(t('agentSetup.setDefaultSuccess', { name: provider.display_name || provider.agent_type }))
    loadProviders()
  } catch (e: any) {
    ElMessage.error(t('agentSetup.hotSwapFailed') + e.message)
  }
}

// ===================== Init =====================

onMounted(() => {
  loadAgentTypes()
  loadProxySetupStatus()
  if (isAdmin.value) {
    loadProviders()
  }
})
</script>

<style scoped>
.agent-setup-page {
  min-height: 100%;
  width: 100%;
  padding: 0 0 24px;
}

.page-header {
  margin-bottom: 24px;
}

.page-title {
  font-size: 1.5rem;
  font-weight: 600;
  margin: 0;
  display: flex;
  align-items: center;
  gap: 8px;
}

.page-description {
  color: #6b7280;
  font-size: 0.875rem;
  margin: 4px 0 0;
}

.setup-tabs {
  margin-top: 8px;
}

.section-hint {
  color: #606266;
  font-size: 0.875rem;
  margin: 0 0 16px;
}

.section-hint--multiline {
  white-space: pre-line;
}

.agent-filter-bar {
  margin-bottom: 16px;
  max-width: 400px;
}

.agent-filter-input {
  width: 100%;
}

.agent-group {
  margin-bottom: 24px;
  padding-top: 2px;
}

.agent-group + .agent-group {
  border-top: 1px solid #ebeef5;
  padding-top: 16px;
}

.agent-group-header {
  margin-bottom: 10px;
}

.agent-group-title {
  margin: 0;
  font-size: 1.05rem;
  font-weight: 600;
  color: #303133;
}

.agent-group-hint {
  margin: 2px 0 0;
  font-size: 0.8rem;
  color: #909399;
  line-height: 1.45;
  max-width: 720px;
}

/* Agent cards — 纵向间距放在列上，避免 height:100% 把卡片 margin 顶没 */
.agent-card-col {
  margin-bottom: 12px;
}

.agent-card {
  height: 100%;
  border: 1px solid #e4e7ed;
  border-radius: 8px;
  padding: 10px 14px 4px;
  cursor: default;
  transition: border-color 0.2s, box-shadow 0.2s, background 0.2s;
  display: flex;
  flex-direction: column;
  gap: 6px;
  background: #fff;
  box-sizing: border-box;
}

.agent-card:hover {
  border-color: var(--el-color-primary-light-5);
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.04);
}

.agent-card--verified {
  border-color: var(--el-color-success-light-5);
  background: linear-gradient(180deg, var(--el-color-success-light-9) 0%, #fff 48px);
}

.agent-card--verified:hover {
  border-color: var(--el-color-success);
}

.agent-card-head {
  display: flex;
  align-items: flex-start;
  gap: 8px;
}

.agent-icon {
  width: 32px;
  height: 32px;
  border-radius: 6px;
  background: #f5f7fa;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #606266;
  flex-shrink: 0;
}

.agent-card:hover .agent-icon {
  background: var(--el-color-primary-light-9);
  color: var(--el-color-primary);
}

.agent-card--verified .agent-icon {
  background: var(--el-color-success-light-9);
  color: var(--el-color-success);
}

.agent-head-text {
  flex: 1;
  min-width: 0;
}

.agent-name {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-wrap: wrap;
  margin-bottom: 1px;
}

.agent-name-text {
  font-weight: 600;
  font-size: 1rem;
  color: #303133;
  line-height: 1.3;
}

.agent-desc {
  color: #909399;
  font-size: 0.82rem;
  line-height: 1.35;
  display: -webkit-box;
  -webkit-line-clamp: 1;
  line-clamp: 1;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.access-methods {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.access-method {
  padding: 5px 8px;
  border-radius: 6px;
  background: #f8f9fb;
  border: 1px solid #eef0f4;
}

.access-method--disabled {
  background: #fafafa;
  border-style: dashed;
}

.method-status-tag {
  margin-left: 6px;
  vertical-align: middle;
}

.wrap-dep-line {
  margin: 4px 0 0;
  font-size: 0.72rem;
  line-height: 1.45;
  color: #909399;
}

.wrap-dep-link {
  color: var(--el-color-primary);
  text-decoration: none;
  margin-left: 2px;
}

.wrap-dep-link:hover {
  text-decoration: underline;
}

.wrap-unavailable {
  margin-top: 8px;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 8px;
}

.wrap-unavailable-text {
  margin: 0;
  font-size: 0.75rem;
  line-height: 1.45;
  color: #909399;
}

.access-method-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
}

.access-method-actions {
  flex-shrink: 0;
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  align-items: center;
  gap: 4px;
}

.access-method-titles {
  display: flex;
  align-items: flex-start;
  gap: 6px;
  min-width: 0;
}

.access-method-index {
  flex-shrink: 0;
  width: 16px;
  height: 16px;
  margin-top: 2px;
  border-radius: 50%;
  background: #e4e7ed;
  color: #606266;
  font-size: 0.7rem;
  font-weight: 600;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  line-height: 1;
}

.access-method-title {
  font-size: 0.85rem;
  font-weight: 600;
  color: #303133;
  line-height: 1.3;
}

.access-method-hint {
  margin: 1px 0 0;
  font-size: 0.75rem;
  color: #909399;
  line-height: 1.35;
}

.access-method-hint--multiline {
  white-space: pre-line;
}

.wrap-install-hint {
  margin: 4px 0 0;
  font-size: 0.72rem;
  color: #606266;
  line-height: 1.4;
}

.ui-params-panel {
  margin-top: 20px;
  padding: 18px 20px;
  border-radius: 10px;
  background: #fafbfc;
  border: 1px solid #e4e7ed;
}

.ui-params-header {
  margin-bottom: 14px;
  padding-bottom: 12px;
  border-bottom: 1px solid #ebeef5;
}

.ui-params-title {
  margin: 0 0 6px;
  font-size: 1rem;
  font-weight: 600;
  color: #303133;
  line-height: 1.4;
}

.ui-params-summary {
  margin: 0;
  font-size: 0.875rem;
  color: #606266;
  line-height: 1.55;
}

.ui-params-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.ui-param-item {
  padding: 12px 14px;
  border-radius: 8px;
  background: #fff;
  border: 1px solid #ebeef5;
}

.ui-param-item-main {
  display: grid;
  grid-template-columns: 96px minmax(0, 1fr);
  gap: 12px 16px;
  align-items: start;
}

.ui-param-label {
  padding-top: 8px;
  font-size: 0.875rem;
  font-weight: 600;
  color: #606266;
  line-height: 1.4;
}

.ui-param-body {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.ui-param-value {
  flex: 1;
  min-width: 0;
  margin: 0;
  padding: 8px 12px;
  border-radius: 6px;
  background: #f5f7fa;
  border: 1px solid #e4e7ed;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.875rem;
  color: #303133;
  line-height: 1.45;
  word-break: break-all;
  user-select: all;
}

.ui-param-actions {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 6px;
}

.ui-param-hint {
  margin: 8px 0 0 112px;
  font-size: 0.8125rem;
  color: #909399;
  line-height: 1.45;
}

.ui-params-footer {
  margin: 14px 0 0;
  padding-top: 12px;
  border-top: 1px solid #ebeef5;
  font-size: 0.875rem;
  color: #909399;
  line-height: 1.5;
}

@media (max-width: 640px) {
  .ui-param-item-main {
    grid-template-columns: 1fr;
    gap: 6px;
  }

  .ui-param-label {
    padding-top: 0;
  }

  .ui-param-body {
    flex-wrap: wrap;
  }

  .ui-param-hint {
    margin-left: 0;
  }
}

.wrap-cmd-row {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 8px;
  padding: 6px 8px;
  border-radius: 6px;
  background: #fff;
  border: 1px dashed #dcdfe6;
}

.wrap-cmd {
  flex: 1;
  min-width: 0;
  font-size: 0.72rem;
  line-height: 1.35;
  color: #303133;
  word-break: break-all;
  user-select: all;
}

.wrap-copy-btn {
  flex-shrink: 0;
  position: static;
  padding: 0 2px;
  height: auto;
  min-height: 0;
}

.install-link {
  display: inline-block;
  font-size: 0.78rem;
  color: var(--el-color-primary);
  text-decoration: none;
  word-break: break-all;
}

.install-link:hover {
  text-decoration: underline;
}

.install-hint {
  margin-top: 2px;
  font-size: 0.75rem;
  color: #a8abb2;
  line-height: 1.35;
  word-break: break-all;
}

.agent-meta-collapse {
  border: none;
  margin-top: auto;
  --el-collapse-header-height: 26px;
}

.agent-meta-collapse :deep(.el-collapse-item__header) {
  font-size: 0.78rem;
  color: #909399;
  border: none;
  background: transparent;
  height: 26px;
  line-height: 26px;
}

.agent-meta-collapse :deep(.el-collapse-item__wrap) {
  border: none;
  background: transparent;
}

.agent-meta-collapse :deep(.el-collapse-item__content) {
  padding-bottom: 4px;
}

.meta-block,
.wizard-meta {
  font-size: 0.8rem;
  color: #606266;
  line-height: 1.5;
}

.wizard-meta {
  margin-bottom: 16px;
  padding: 12px 14px;
  background: #f5f7fa;
  border-radius: 8px;
}

.meta-row {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 6px 8px;
  margin-bottom: 6px;
}

.meta-row-col {
  flex-direction: column;
  align-items: flex-start;
  gap: 2px;
}

.meta-label {
  font-weight: 600;
  color: #303133;
  flex-shrink: 0;
}

.meta-path {
  font-size: 0.75rem;
  background: #eef1f6;
  padding: 1px 6px;
  border-radius: 4px;
  word-break: break-all;
}

.meta-fields {
  word-break: break-all;
  color: #909399;
}

.meta-method {
  margin: 4px 0 0;
  white-space: pre-wrap;
}

/* Wizard */
.wizard-steps {
  margin-bottom: 24px;
}

.wizard-step {
  min-height: 240px;
}

.step-alert {
  margin-bottom: 16px;
}

.alert-actions {
  margin-top: 8px;
}

.mode-switch {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 8px;
  margin-bottom: 16px;
}

.mode-hint {
  font-size: 0.85rem;
  color: #909399;
  line-height: 1.5;
}

.section-subhint {
  margin: 0 0 8px;
  font-size: 0.8rem;
  color: #909399;
}

.select-label {
  font-size: 0.9rem;
  font-weight: 600;
  color: #303133;
  margin: 0 0 8px;
}

.pipeline-option {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
  max-width: 100%;
}

.pipeline-option > span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pipeline-desc {
  font-size: 0.75rem;
  color: #909399;
}

.pipeline-summary {
  margin-top: 12px;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}

.pipeline-summary .el-tag {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.model-hint {
  font-size: 0.8rem;
  color: #909399;
}

.pipeline-summary .model-hint {
  flex-basis: 100%;
}

.empty-pipelines {
  padding: 24px 0;
}

.config-result {
  margin-top: 8px;
}

.config-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 20px;
}

.config-header h3 {
  margin: 0;
  font-size: 1rem;
}

.config-section {
  margin-bottom: 24px;
}

.config-section h4 {
  margin: 0 0 12px;
  font-size: 0.95rem;
  color: #303133;
}

.code-block {
  position: relative;
  background: #f5f7fa;
  border: 1px solid #e4e7ed;
  border-radius: 8px;
  overflow: hidden;
}

.code-block pre {
  margin: 0;
  padding: 16px;
  overflow-x: auto;
  font-size: 0.85rem;
  line-height: 1.6;
}

.code-block code {
  font-family: 'Menlo', 'Monaco', 'Consolas', monospace;
  color: #303133;
}

.copy-btn {
  position: absolute;
  top: 8px;
  right: 8px;
  background: white;
  border: 1px solid #dcdfe6;
  border-radius: 4px;
  padding: 4px 8px;
}

.copy-btn:hover {
  background: #f5f7fa;
}

.write-hint {
  margin-top: 12px;
  font-size: 0.85rem;
}

.write-preview {
  margin-top: 12px;
}

.write-preview h5 {
  margin: 0 0 8px;
  font-size: 0.85rem;
  color: #606266;
}

.verify-desc {
  margin: 0 0 8px;
  font-size: 0.875rem;
  color: #606266;
}

.verify-checklist {
  margin: 0;
  padding-left: 1.25rem;
  color: #303133;
  font-size: 0.875rem;
  line-height: 1.8;
}

.verify-checklist code {
  background: #f5f7fa;
  padding: 1px 6px;
  border-radius: 4px;
  font-size: 0.8rem;
}

.wizard-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
}

.wizard-footer-right {
  display: flex;
  gap: 8px;
}

/* Provider table */
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.text-muted {
  color: #909399;
  font-size: 0.85rem;
}
</style>

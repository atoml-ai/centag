<template>
  <el-drawer
    :model-value="visible"
    @update:model-value="emit('update:visible', $event)"
    :title="node?.name || t('nodeConfig.title')"
    size="960px"
    direction="rtl"
    :before-close="handleClose"
  >
    <el-form
      ref="formRef"
      :model="localNode"
      label-position="top"
      :rules="rules"
      class="node-drawer-form"
    >
      <!-- ═══════ 1. Basic Info ═══════ -->
      <section class="drawer-section">
        <div class="section-title">{{ t('nodeConfig.sectionBasic') }}</div>
        <el-row :gutter="12">
          <el-col :span="14">
            <el-form-item :label="t('nodeConfig.nodeName')" prop="name">
              <el-input v-model="localNode.name" :placeholder="t('nodeConfig.nodeNamePlaceholder')" />
            </el-form-item>
          </el-col>
          <el-col :span="10">
            <el-form-item :label="t('nodeConfig.nodeType')" prop="type">
              <el-select v-model="localNode.type" style="width: 100%" @change="onTypeChange">
                <el-option :label="t('nodeConfig.typeTransparentForward')" value="transparent_forward" />
                <el-option :label="t('nodeConfig.typeGenerator')" value="generator" />
                <el-option :label="t('nodeConfig.typeProcessor')" value="processor" />
                <el-option :label="t('nodeConfig.typeReviewer')" value="reviewer" />
                <el-option :label="t('nodeConfig.typeRouter')" value="router" />
                <el-option :label="t('nodeConfig.typeAggregator')" value="aggregator" />
                <el-option :label="t('nodeConfig.typeParallel')" value="parallel" />
                <el-option :label="t('nodeConfig.typeCache')" value="cache" />
                <el-option :label="t('nodeConfig.typeTokenUsage')" value="token_usage" />
                <el-option :label="t('nodeConfig.typeToolCallInjector')" value="tool_call_injector" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <div class="type-desc">{{ getTypeDescription(localNode.type) }}</div>
      </section>

      <!-- ═══════ 2. Outbound Forward ═══════ -->
      <section v-if="localNode.type === 'transparent_forward'" class="drawer-section">
        <div class="section-title">{{ t('nodeConfig.sectionEgress') }}</div>
        <el-alert type="info" :closable="false" class="section-alert">
          <template #default>
            <div style="font-size: 13px; line-height: 1.6">
              <div v-html="t('nodeConfig.egressDescOutbound')" />
              <div style="margin-top: 6px" v-html="t('nodeConfig.egressDescMatchModel')" />
              <div style="margin-top: 6px" v-html="t('nodeConfig.egressDescFixed')" />
            </div>
          </template>
        </el-alert>

        <el-form-item :label="t('nodeConfig.routePolicy')">
          <el-radio-group v-model="egressConfig.route_policy">
            <el-radio-button value="match_model">{{ t('nodeConfig.routePolicyMatchModel') }}</el-radio-button>
            <el-radio-button value="fixed">{{ t('nodeConfig.routePolicyFixed') }}</el-radio-button>
          </el-radio-group>
        </el-form-item>

        <el-row :gutter="12">
          <el-col :span="12">
            <el-form-item :label="t('nodeConfig.backend')" prop="backend">
              <BackendSelector
                v-model="localNode.backend"
                :placeholder="t('nodeConfig.backendPlaceholder')"
                style="width: 100%"
                @change="onBackendChange"
              />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item :label="t('nodeConfig.model')" prop="model">
              <ModelSelector
                v-model="localNode.model"
                :backend-id="localNode.backend"
                :placeholder="t('nodeConfig.modelPlaceholder')"
                :allow-create="true"
                :default-first-option="true"
              />
            </el-form-item>
          </el-col>
        </el-row>
        <div class="help-text" style="margin: -6px 0 12px">
          {{ egressConfig.route_policy === 'fixed' ? t('nodeConfig.fixedEgressHelp') : t('nodeConfig.matchModelFallbackHelp') }}
        </div>

        <div class="egress-prompt-block">
          <el-form-item class="egress-inject-item" label-width="0">
            <div class="egress-inject-row">
              <span class="egress-inject-label">{{ t('nodeConfig.injectSystemPrompt') }}</span>
              <el-switch v-model="egressConfig.inject_system_prompt" />
            </div>
          </el-form-item>

          <div v-if="egressConfig.inject_system_prompt" class="egress-prompt-body">
            <div class="egress-prompt-label">{{ t('nodeConfig.systemPromptContent') }}</div>
            <div class="system-prompt-toolbar">
              <el-select
                v-model="selectedPromptPreset"
                clearable
                :placeholder="t('nodeConfig.personalityPreset')"
                style="width: 160px"
                @change="onPromptPresetChange"
              >
                <el-option v-for="p in systemPromptPresets" :key="p.id" :label="p.label" :value="p.id" />
              </el-select>
              <el-button size="small" @click="restoreDefaultSystemPrompt">{{ t('nodeConfig.restoreDefault') }}</el-button>
            </div>
            <el-input
              ref="promptInputRef"
              v-model="promptFieldValue"
              type="textarea"
              :rows="6"
              :placeholder="getPromptPlaceholder(localNode.type)"
            />
            <div class="help-text">{{ t('nodeConfig.systemPromptHelp') }}</div>
          </div>
        </div>
      </section>

      <!-- ═══════ 2b. LLM Node (non-outbound) ═══════ -->
      <section v-if="needsLLM(localNode.type) && localNode.type !== 'transparent_forward'" class="drawer-section">
        <div class="section-title">{{ t('nodeConfig.sectionModelPrompt') }}</div>
        <el-row :gutter="12">
          <el-col :span="12">
            <el-form-item :label="t('nodeConfig.backend')" prop="backend">
              <BackendSelector
                v-model="localNode.backend"
                :placeholder="t('nodeConfig.backendPlaceholder')"
                style="width: 100%"
                @change="onBackendChange"
              />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item :label="t('nodeConfig.model')" prop="model">
              <ModelSelector
                v-model="localNode.model"
                :backend-id="localNode.backend"
                :placeholder="t('nodeConfig.modelPlaceholder')"
                :allow-create="true"
                :default-first-option="true"
              />
            </el-form-item>
          </el-col>
        </el-row>

        <el-form-item v-if="showPromptEditor" :label="getPromptLabel(localNode.type)">
          <div v-if="usesSystemPrompt" class="system-prompt-toolbar">
            <el-select
              v-model="selectedPromptPreset"
              clearable
              :placeholder="t('nodeConfig.personalityPreset')"
              style="width: 160px"
              @change="onPromptPresetChange"
            >
              <el-option v-for="p in systemPromptPresets" :key="p.id" :label="p.label" :value="p.id" />
            </el-select>
            <el-button size="small" @click="restoreDefaultSystemPrompt">{{ t('nodeConfig.restoreDefault') }}</el-button>
          </div>
          <el-input
            ref="promptInputRef"
            v-model="promptFieldValue"
            type="textarea"
            :rows="6"
            :placeholder="getPromptPlaceholder(localNode.type)"
          />
          <div v-if="localNode.type === 'generator'" class="help-text">{{ t('nodeConfig.generatorPromptHelp') }}</div>
          <div v-if="localNode.type === 'aggregator'" class="help-text">
            {{ t('nodeConfig.aggregatorPromptHelp') }}
          </div>
        </el-form-item>
      </section>

      <!-- ═══════ 2c. Plugin Implementation ═══════ -->
      <section v-if="needsPluginSelection(localNode.type)" class="drawer-section">
        <div class="section-title">{{ t('nodeConfig.sectionPlugin') }}</div>
        <PluginSelector
          v-model="localNode.implementation"
          v-model:kind="localNode.kind"
          :plugins="availablePlugins"
          show-kind-selector
          show-all-plugins
          @view-all="openPluginManager"
        />
      </section>

      <!-- ═══════ 2d. Router Core ═══════ -->
      <section v-if="localNode.type === 'router'" class="drawer-section">
        <div class="section-title">{{ t('nodeConfig.sectionRouter') }}</div>
        <el-form-item :label="t('nodeConfig.routerStrategy')">
          <el-select v-model="routerConfig.strategy" style="width: 100%">
            <el-option :label="t('nodeConfig.strategyKeywordContains')" value="keyword_contains" />
            <el-option :label="t('nodeConfig.strategyKeywordPrefix')" value="keyword_prefix" />
            <el-option :label="t('nodeConfig.strategyOrdered')" value="ordered" />
            <el-option :label="t('nodeConfig.strategyRegex')" value="regex_only" />
            <el-option :label="t('nodeConfig.strategyKeywordThenIntent')" value="keyword_then_intent" />
            <el-option :label="t('nodeConfig.strategyLlmClassify')" value="llm_classify" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('nodeConfig.defaultRouteNode')">
          <el-input v-model="routerConfig.defaultRoute" :placeholder="t('nodeConfig.defaultRoutePlaceholder')" />
        </el-form-item>
        <el-form-item :label="t('nodeConfig.routeRuleMapping')">
          <div
            v-for="(rule, idx) in routerConfig.routes"
            :key="idx"
            class="custom-config-row"
          >
            <el-input
              v-model="rule.keyword"
              :placeholder="routerConfig.strategy === 'llm_classify' ? t('nodeConfig.keywordPlaceholderCategory') : t('nodeConfig.keywordPlaceholderKeyword')"
              size="small"
              style="width: 180px; flex-shrink: 0"
            />
            <span class="var-eq">→</span>
            <el-input v-model="rule.target" :placeholder="t('nodeConfig.targetNodeIdPlaceholder')" size="small" style="flex: 1" />
            <el-button type="danger" text size="small" @click="removeRouterRoute(idx)">
              <el-icon><Delete /></el-icon>
            </el-button>
          </div>
          <el-button size="small" @click="addRouterRoute" style="margin-top: 6px">
            <el-icon><Plus /></el-icon>
            {{ t('nodeConfig.addRule') }}
          </el-button>
        </el-form-item>
      </section>

      <!-- ═══════ 2e. Cache Core ═══════ -->
      <section v-if="localNode.type === 'cache'" class="drawer-section">
        <div class="section-title">{{ t('nodeConfig.sectionCache') }}</div>
        <el-row :gutter="12">
          <el-col :span="12">
            <el-form-item :label="t('nodeConfig.operation')">
              <el-select v-model="localNode.config.customConfig.operation" style="width: 100%">
                <el-option :label="t('nodeConfig.operationRead')" value="read" />
                <el-option :label="t('nodeConfig.operationWrite')" value="write" />
                <el-option :label="t('nodeConfig.operationDelete')" value="delete" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item :label="t('nodeConfig.strategyLabel')">
              <el-select v-model="localNode.config.customConfig.strategy" style="width: 100%">
                <el-option :label="t('nodeConfig.strategyExact')" value="exact" />
                <el-option :label="t('nodeConfig.strategySemantic')" value="semantic" />
                <el-option :label="t('nodeConfig.strategyHybrid')" value="hybrid" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item
          v-if="localNode.config.customConfig.operation === 'read' || localNode.config.customConfig.operation === 'delete'"
          :label="t('nodeConfig.readStorage')"
        >
          <el-select v-model="localNode.config.customConfig.read_storage_name" style="width: 100%" clearable :placeholder="t('nodeConfig.storagePlaceholder')">
            <el-option v-for="s in availableStorages" :key="s.name" :label="`${s.name} (${s.type})`" :value="s.name" />
          </el-select>
        </el-form-item>
        <el-form-item
          v-if="localNode.config.customConfig.operation === 'write' || localNode.config.customConfig.operation === 'delete'"
          :label="t('nodeConfig.writeStorage')"
        >
          <el-select v-model="localNode.config.customConfig.write_storage_name" style="width: 100%" clearable :placeholder="t('nodeConfig.storagePlaceholder')">
            <el-option v-for="s in availableStorages" :key="s.name" :label="`${s.name} (${s.type})`" :value="s.name" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('nodeConfig.cacheKeyTemplate')">
          <el-input v-model="localNode.config.customConfig.key_template" :placeholder="'{{model}}:{{hash}}'" />
        </el-form-item>
        <el-form-item v-if="localNode.config.customConfig.operation === 'write'" :label="t('nodeConfig.ttlSeconds')">
          <el-input-number v-model="localNode.config.customConfig.ttl" :min="60" :max="86400" style="width: 100%" />
        </el-form-item>
      </section>

      <!-- ═══════ 2f. Token Usage Core ═══════ -->
      <section v-if="localNode.type === 'token_usage'" class="drawer-section">
        <div class="section-title">{{ t('nodeConfig.sectionTokenUsage') }}</div>
        <el-alert type="info" :closable="false" class="section-alert">
          {{ t('nodeConfig.tokenUsageAlert') }}
        </el-alert>
        <el-row :gutter="12">
          <el-col :span="12">
            <el-form-item :label="t('nodeConfig.operation')">
              <el-select v-model="localNode.config.customConfig.operation" style="width: 100%">
                <el-option :label="t('nodeConfig.operationRecord')" value="record" />
                <el-option :label="t('nodeConfig.operationQuery')" value="query" />
                <el-option :label="t('nodeConfig.operationAggregate')" value="aggregate" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item :label="t('nodeConfig.storageType')">
              <el-select v-model="localNode.config.customConfig.storage_type" style="width: 100%">
                <el-option :label="t('nodeConfig.storageTypeSqlite')" value="sqlite" />
                <el-option label="PostgreSQL" value="postgresql" />
                <el-option :label="t('nodeConfig.storageTypeMemory')" value="memory" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item v-if="localNode.config.customConfig.operation === 'record'" :label="t('nodeConfig.recordFields')">
          <el-checkbox-group v-model="localNode.config.customConfig.record_fields">
            <el-checkbox label="prompt_tokens">{{ t('nodeConfig.fieldPromptTokens') }}</el-checkbox>
            <el-checkbox label="completion_tokens">{{ t('nodeConfig.fieldCompletionTokens') }}</el-checkbox>
            <el-checkbox label="total_tokens">{{ t('nodeConfig.fieldTotalTokens') }}</el-checkbox>
            <el-checkbox label="model">{{ t('nodeConfig.fieldModel') }}</el-checkbox>
            <el-checkbox label="backend_id">{{ t('nodeConfig.fieldBackendId') }}</el-checkbox>
          </el-checkbox-group>
        </el-form-item>
      </section>

      <!-- ═══════ 2g. Tool Call Injector Core ═══════ -->
      <section v-if="localNode.type === 'tool_call_injector'" class="drawer-section">
        <div class="section-title">{{ t('nodeConfig.sectionToolInjector') }}</div>
        <el-form-item :label="t('nodeConfig.injectCondition')">
          <el-input
            v-model="injectorCondition"
            :placeholder="t('nodeConfig.injectConditionPlaceholder')"
            type="textarea"
            :rows="2"
          />
        </el-form-item>
        <el-form-item :label="t('nodeConfig.toolCallList')">
          <div class="injector-tc-list">
            <div v-for="(tc, idx) in injectorToolCalls" :key="idx" class="injector-tc-item">
              <div class="injector-tc-header">
                <span>{{ t('nodeConfig.toolNumber', { n: idx + 1 }) }}</span>
                <el-button type="danger" text size="small" @click="removeInjectorToolCall(idx)">
                  <el-icon><Delete /></el-icon>
                </el-button>
              </div>
              <el-input v-model="tc.id" :placeholder="t('nodeConfig.callIdPlaceholder')" size="small" style="margin-bottom: 6px" />
              <el-input v-model="tc.functionName" :placeholder="t('nodeConfig.functionNamePlaceholder')" size="small" style="margin-bottom: 6px" />
              <el-input v-model="tc.arguments" :placeholder="t('nodeConfig.argumentsPlaceholder')" type="textarea" :rows="2" size="small" />
            </div>
            <el-button size="small" @click="addInjectorToolCall" style="margin-top: 8px">
              <el-icon><Plus /></el-icon>
              {{ t('nodeConfig.addToolCall') }}
            </el-button>
          </div>
        </el-form-item>
      </section>

      <!-- ═══════ 3. Advanced Options (collapsed by default) ═══════ -->
      <section class="drawer-section drawer-section-advanced">
        <el-collapse v-model="advancedPanels" class="advanced-collapse">
          <el-collapse-item :title="t('nodeConfig.advancedTopology')" name="topo">
            <el-form-item :label="t('nodeConfig.nodeId')" prop="id">
              <el-input v-model="localNode.id" :placeholder="t('nodeConfig.nodeIdPlaceholder')" />
            </el-form-item>
            <el-row :gutter="12">
              <el-col :span="12">
                <el-form-item :label="t('nodeConfig.dependsOn')">
                  <el-select v-model="localNode.depends_on" multiple style="width: 100%">
                    <el-option v-for="n in otherNodes" :key="n" :label="n" :value="n" />
                  </el-select>
                </el-form-item>
              </el-col>
              <el-col :span="12">
                <el-form-item :label="t('nodeConfig.nextNodes')">
                  <el-select v-model="localNode.next_nodes" multiple style="width: 100%">
                    <el-option v-for="n in otherNodes" :key="n" :label="n" :value="n" />
                  </el-select>
                </el-form-item>
              </el-col>
            </el-row>
            <el-form-item v-if="localNode.type !== 'tool_call_injector'" :label="t('nodeConfig.execCondition')">
              <el-input v-model="localNode.config.condition" :placeholder="conditionPlaceholder" type="textarea" :rows="2" />
            </el-form-item>
          </el-collapse-item>

          <el-collapse-item :title="t('nodeConfig.advancedReliability')" name="reliability">
            <el-form-item :label="t('nodeConfig.timeoutSeconds')">
              <el-input-number v-model="localNode.timeout" :min="5" :max="300" style="width: 100%" />
            </el-form-item>
            <el-form-item :label="t('nodeConfig.retry')">
              <div class="retry-grid">
                <el-input-number v-model="localNode.retry.max_attempts" :min="0" :max="10" :placeholder="t('nodeConfig.retryAttemptsPlaceholder')" />
                <el-select v-model="localNode.retry.backoff_strategy">
                  <el-option :label="t('nodeConfig.backoffExponential')" value="exponential" />
                  <el-option :label="t('nodeConfig.backoffLinear')" value="linear" />
                  <el-option :label="t('nodeConfig.backoffFixed')" value="fixed" />
                </el-select>
              </div>
            </el-form-item>
            <el-form-item :label="t('nodeConfig.fallbackPolicy')">
              <el-select v-model="localNode.fallback_policy_id" clearable :placeholder="t('nodeConfig.fallbackPolicyPlaceholder')" style="width: 100%">
                <el-option :label="t('nodeConfig.fallbackPolicyInherit')" value="" />
                <el-option
                  v-for="policy in fallbackPolicies"
                  :key="policy.id"
                  :label="`${policy.name} (${policy.id})`"
                  :value="policy.id"
                />
              </el-select>
            </el-form-item>
          </el-collapse-item>

          <el-collapse-item
            v-if="localNode.type === 'transparent_forward'"
            :title="t('nodeConfig.advancedEgressRedirect')"
            name="egress_adv"
          >
            <el-form-item :label="t('nodeConfig.redirectPolicy')">
              <el-select v-model="egressConfig.redirect_policy" style="width: 100%">
                <el-option :label="t('nodeConfig.redirectNever')" value="never" />
                <el-option :label="t('nodeConfig.redirectAlways')" value="always" />
                <el-option :label="t('nodeConfig.redirectSmart')" value="smart" />
              </el-select>
            </el-form-item>
            <el-form-item v-if="egressConfig.redirect_policy !== 'never'" :label="t('nodeConfig.maxRedirects')">
              <el-input-number v-model="egressConfig.max_redirects" :min="1" :max="20" style="width: 100%" />
            </el-form-item>
            <el-form-item :label="t('nodeConfig.defaultUrlScheme')">
              <el-select v-model="egressConfig.default_scheme" style="width: 100%">
                <el-option label="https" value="https" />
                <el-option label="http" value="http" />
              </el-select>
            </el-form-item>
          </el-collapse-item>

          <el-collapse-item
            v-if="showPromptEditor"
            :title="t('nodeConfig.advancedPromptVars')"
            name="prompt_vars"
          >
            <div class="var-panel">
              <div class="var-panel-header">
                <span class="var-panel-title">{{ t('nodeConfig.availableVars') }}</span>
                <span class="var-panel-hint">{{ t('nodeConfig.clickToInsert') }}</span>
              </div>
              <table class="var-table">
                <colgroup>
                  <col style="width: 190px" />
                  <col />
                </colgroup>
                <thead>
                  <tr>
                    <th>{{ t('nodeConfig.varName') }}</th>
                    <th>{{ t('nodeConfig.description') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr class="var-section-row">
                    <td colspan="2" class="var-section-label">{{ t('nodeConfig.builtinVars') }}</td>
                  </tr>
                  <tr v-for="v in builtinVars" :key="v.name">
                    <td>
                      <el-tag class="var-chip" type="primary" size="small" @click="insertVar(v.name)">
                        {{ varLabel(v.name) }}
                      </el-tag>
                    </td>
                    <td class="var-desc">{{ v.desc }}</td>
                  </tr>
                  <template v-if="upstreamVars.length > 0">
                    <tr class="var-section-row">
                      <td colspan="2" class="var-section-label">{{ t('nodeConfig.upstreamNodesBound') }}</td>
                    </tr>
                    <tr v-for="v in upstreamVars" :key="v.name">
                      <td>
                        <el-tag class="var-chip" type="success" size="small" @click="insertVar(v.name)">
                          {{ varLabel(v.name) }}
                        </el-tag>
                      </td>
                      <td class="var-desc">{{ v.desc }}</td>
                    </tr>
                  </template>
                  <tr class="var-section-row">
                    <td colspan="2" class="var-section-label">{{ t('nodeConfig.execContextBound') }}</td>
                  </tr>
                  <tr v-for="v in contextVars" :key="v.name">
                    <td>
                      <el-tag class="var-chip" type="warning" size="small" @click="insertVar(v.name)">
                        {{ varLabel(v.name) }}
                      </el-tag>
                    </td>
                    <td class="var-desc">{{ v.desc }}</td>
                  </tr>
                </tbody>
              </table>
            </div>

            <el-form-item
              v-if="localNode.type === 'reviewer' || localNode.type === 'processor'"
              :label="t('nodeConfig.customVarBinding')"
              style="margin-top: 12px"
            >
              <div class="template-vars-editor">
                <div
                  v-for="(binding, idx) in templateVarBindings"
                  :key="idx"
                  class="template-var-row"
                >
                  <el-input v-model="binding.key" :placeholder="t('nodeConfig.bindingVarPlaceholder')" size="small" style="width: 140px; flex-shrink: 0" />
                  <span class="var-eq">=</span>
                  <el-select
                    v-model="binding.source"
                    :placeholder="t('nodeConfig.bindingSource')"
                    size="small"
                    style="width: 130px; flex-shrink: 0"
                    @change="(v: string) => onSourceChange(binding, v)"
                  >
                    <el-option :label="t('nodeConfig.sourceInputContent')" value="input.content" />
                    <el-option :label="t('nodeConfig.sourceTimestamp')" value="context.timestamp" />
                    <el-option :label="t('nodeConfig.sourceUserId')" value="context.user_id" />
                    <el-option :label="t('nodeConfig.sourceSessionId')" value="context.session_id" />
                    <el-option
                      v-for="n in upstreamNodeIds"
                      :key="n + '_content'"
                      :label="t('nodeConfig.sourceNodeOutput', { n })"
                      :value="`node.${n}.content`"
                    />
                    <el-option :label="t('nodeConfig.sourceCustomPath')" value="__custom__" />
                  </el-select>
                  <el-input
                    v-if="binding.source === '__custom__'"
                    v-model="binding.customPath"
                    :placeholder="t('nodeConfig.customPathPlaceholder')"
                    size="small"
                    style="flex: 1"
                  />
                  <el-button type="danger" text size="small" @click="removeVarBinding(idx)">
                    <el-icon><Delete /></el-icon>
                  </el-button>
                </div>
                <el-button size="small" @click="addVarBinding" style="margin-top: 6px">
                  <el-icon><Plus /></el-icon>
                  {{ t('nodeConfig.addBinding') }}
                </el-button>
              </div>
            </el-form-item>
          </el-collapse-item>

          <el-collapse-item
            v-if="localNode.type === 'router' && routerConfig.strategy === 'llm_classify'"
            :title="t('nodeConfig.advancedLlmClassify')"
            name="router_llm"
          >
            <el-form-item :label="t('nodeConfig.classifyBackend')">
              <BackendSelector v-model="localNode.backend" :placeholder="t('nodeConfig.classifyBackendPlaceholder')" style="width: 100%" @change="onBackendChange" />
            </el-form-item>
            <el-form-item :label="t('nodeConfig.classifyModel')">
              <ModelSelector
                v-model="localNode.model"
                :backend-id="localNode.backend"
                :placeholder="t('nodeConfig.classifyModelPlaceholder')"
                :allow-create="true"
                :default-first-option="true"
              />
            </el-form-item>
            <el-form-item :label="t('nodeConfig.classifyPrompt')">
              <el-input
                v-model="routerConfig.classifyPrompt"
                type="textarea"
                :rows="5"
                :placeholder="t('nodeConfig.classifyPromptPlaceholder')"
              />
            </el-form-item>
          </el-collapse-item>

          <el-collapse-item
            v-if="localNode.type === 'cache' && isSemanticStrategy"
            :title="t('nodeConfig.advancedSemanticCache')"
            name="cache_semantic"
          >
            <el-form-item :label="t('nodeConfig.embeddingBackend')">
              <BackendSelector
                v-model="localNode.config.customConfig.embedding_backend_id"
                :placeholder="t('nodeConfig.embeddingBackendPlaceholder')"
                :filter="embeddingBackendFilter"
                style="width: 100%"
                @change="onEmbeddingBackendChange"
              />
            </el-form-item>
            <el-form-item v-if="localNode.config.customConfig.embedding_backend_id" :label="t('nodeConfig.embeddingModel')">
              <el-select
                v-model="localNode.config.customConfig.embedding_model"
                style="width: 100%"
                clearable
                :placeholder="t('nodeConfig.embeddingModelPlaceholder')"
                :allow-create="true"
                :default-first-option="true"
              >
                <el-option v-for="m in embeddingModels" :key="m" :label="m" :value="m" />
              </el-select>
            </el-form-item>
            <el-form-item :label="t('nodeConfig.semanticThreshold')">
              <el-input-number
                v-model="localNode.config.customConfig.semantic_threshold"
                :min="0"
                :max="1"
                :step="0.05"
                :precision="2"
                style="width: 100%"
              />
            </el-form-item>
            <el-form-item :label="t('nodeConfig.topK')">
              <el-input-number v-model="localNode.config.customConfig.semantic_top_k" :min="1" :max="100" style="width: 100%" />
            </el-form-item>
          </el-collapse-item>

          <el-collapse-item
            v-if="needsPluginSelection(localNode.type)"
            :title="t('nodeConfig.advancedPluginParams')"
            name="plugin_cc"
          >
            <template v-if="localNode.implementation === 'business.mem0'">
              <el-form-item :label="t('nodeConfig.mem0ApiKey')">
                <el-input v-model="mem0PresetConfig.api_key" type="password" show-password />
              </el-form-item>
              <el-form-item :label="t('nodeConfig.mem0BaseUrl')">
                <el-input v-model="mem0PresetConfig.base_url" placeholder="http://localhost:20061" />
              </el-form-item>
              <el-form-item :label="t('nodeConfig.mem0Namespace')">
                <el-input v-model="mem0PresetConfig.namespace" placeholder="default" />
              </el-form-item>
              <el-form-item :label="t('nodeConfig.mem0SearchMode')">
                <el-select v-model="mem0PresetConfig.search_mode" style="width: 100%">
                  <el-option :label="t('nodeConfig.mem0SearchSemantic')" value="semantic" />
                  <el-option :label="t('nodeConfig.mem0SearchKeyword')" value="keyword" />
                  <el-option :label="t('nodeConfig.mem0SearchHybrid')" value="hybrid" />
                </el-select>
              </el-form-item>
              <el-form-item :label="t('nodeConfig.mem0EmbeddingBackend')">
                <BackendSelector
                  v-model="mem0PresetConfig.embedding_backend_id"
                  :placeholder="t('nodeConfig.mem0EmbeddingBackendPlaceholder')"
                  :filter="embeddingBackendFilter"
                  style="width: 100%"
                  @change="onMem0EmbeddingBackendChange"
                />
              </el-form-item>
              <el-form-item v-if="mem0PresetConfig.embedding_backend_id" :label="t('nodeConfig.mem0EmbeddingModel')">
                <el-select
                  v-model="mem0PresetConfig.embedding_model"
                  style="width: 100%"
                  clearable
                  :allow-create="true"
                  :default-first-option="true"
                >
                  <el-option v-for="m in embeddingModels" :key="m" :label="m" :value="m" />
                </el-select>
              </el-form-item>
            </template>

            <el-form-item :label="t('nodeConfig.customParams')">
              <div class="custom-config-editor">
                <div
                  v-for="(item, idx) in customConfigItems"
                  :key="idx"
                  class="custom-config-row"
                >
                  <el-input v-model="item.key" :placeholder="t('nodeConfig.paramNamePlaceholder')" size="small" style="width: 160px; flex-shrink: 0" />
                  <span class="var-eq">=</span>
                  <el-input
                    v-model="item.value"
                    :placeholder="t('nodeConfig.paramValuePlaceholder')"
                    size="small"
                    style="flex: 1"
                    :type="isSecretKey(item.key) ? 'password' : 'text'"
                    :show-password="isSecretKey(item.key)"
                  />
                  <el-button type="danger" text size="small" @click="removeCustomConfigItem(idx)">
                    <el-icon><Delete /></el-icon>
                  </el-button>
                </div>
                <div class="custom-config-actions">
                  <el-button size="small" @click="addCustomConfigItem">
                    <el-icon><Plus /></el-icon>
                    {{ t('nodeConfig.addParam') }}
                  </el-button>
                  <el-button
                    v-if="localNode.implementation === 'business.mem0'"
                    size="small"
                    @click="fillMem0Defaults"
                  >
                    {{ t('nodeConfig.fillMem0Defaults') }}
                  </el-button>
                </div>
              </div>
            </el-form-item>
          </el-collapse-item>

          <el-collapse-item
            v-if="localNode.type === 'generator'"
            :title="t('nodeConfig.advancedBranchRoute')"
            name="route_config"
          >
            <el-form-item :label="t('nodeConfig.enableRouteConfig')">
              <el-switch v-model="hasRouteConfig" />
            </el-form-item>
            <template v-if="hasRouteConfig">
              <el-form-item :label="t('nodeConfig.upstreamRouterNode')">
                <el-select v-model="routeConfig.router_node_id" style="width: 100%" :placeholder="t('nodeConfig.selectRouter')">
                  <el-option v-for="n in routerNodes" :key="n" :label="n" :value="n" />
                </el-select>
              </el-form-item>
              <el-form-item :label="t('nodeConfig.routeMatchValue')">
                <el-input v-model="routeConfig.route_value" :placeholder="t('nodeConfig.routeMatchPlaceholder')" />
              </el-form-item>
              <el-form-item :label="t('nodeConfig.defaultBranch')">
                <el-switch v-model="routeConfig.is_default" />
              </el-form-item>
            </template>
          </el-collapse-item>
        </el-collapse>
      </section>
    </el-form>


    <template #footer>
      <div class="footer-buttons">
        <el-button @click="handleClose">{{ t('nodeConfig.cancel') }}</el-button>
        <el-button type="primary" @click="saveNode">{{ t('nodeConfig.saveNodeConfig') }}</el-button>
      </div>
    </template>
  </el-drawer>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { Delete, Plus } from '@element-plus/icons-vue'
import BackendSelector from './BackendSelector.vue'
import ModelSelector from './ModelSelector.vue'
import PluginSelector from './pipeline/PluginSelector.vue'
import { getBackends, getBackendModels } from '../api/backend'
import { getNodePlugins, parseNodePluginsResponse, PluginDescriptor, updateNodeConfig } from '../api/pipeline'
import {
  DEFAULT_SYSTEM_PROMPT,
  SYSTEM_PROMPT_PRESETS,
  getPresetById,
} from '../utils/system-prompt-presets'

const { t } = useI18n()

const props = defineProps<{
  visible: boolean
  node: any
  allNodes: any[]
  backends: any[]
  storages?: any[]  // 新增：存储列表
  pipelineId?: string // 新增：流水线ID，用于持久化节点配置
  /** 父组件预加载的插件列表（优先使用，避免抽屉 v-if 挂载时 watch 未触发） */
  plugins?: PluginDescriptor[]
}>()

const emit = defineEmits(['update:visible', 'update:node'])

const localNode = ref<any>({
  retry: { max_attempts: 0, backoff_strategy: 'exponential', initial_delay: 1000, max_delay: 30000 },
  config: {},
  depends_on: [],
  next_nodes: [],
})
const formRef = ref()
const promptInputRef = ref<any>(null)
const systemPromptPresets = SYSTEM_PROMPT_PRESETS
const selectedPromptPreset = ref<string>('')

// 插件列表（父组件传入时优先，否则本地加载）
const loadedPlugins = ref<PluginDescriptor[]>([])
const loadingPlugins = ref(false)
const availablePlugins = computed(() => {
  if (props.plugins && props.plugins.length > 0) {
    return props.plugins
  }
  return loadedPlugins.value
})

// ─── 路由节点配置 (Router Config) ──────────────────────────────────────────

interface RouterRouteRule {
  keyword: string
  target: string
}

interface RouterConfigState {
  strategy: string
  defaultRoute: string
  routes: RouterRouteRule[]
  classifyPrompt: string
}

const routerConfig = ref<RouterConfigState>({
  strategy: 'keyword_contains',
  defaultRoute: '',
  routes: [],
  classifyPrompt: '',
})

const addRouterRoute = () => {
  routerConfig.value.routes.push({ keyword: '', target: '' })
}

const removeRouterRoute = (idx: number) => {
  routerConfig.value.routes.splice(idx, 1)
}

// 从节点的 custom_config 加载路由配置
const loadRouterConfig = (node: any) => {
  const cc = node.config?.custom_config || {}
  const strategy = String(cc.routing_strategy || 'keyword_contains')
  const defaultRoute = String(cc.default_route || '')
  const classifyPrompt = String(cc.classify_prompt || '')
  const rawRoutes = cc.routes || {}
  const routes: RouterRouteRule[] = []
  if (rawRoutes && typeof rawRoutes === 'object') {
    for (const [keyword, target] of Object.entries(rawRoutes)) {
      routes.push({ keyword, target: String(target || '') })
    }
  }
  routerConfig.value = { strategy, defaultRoute, routes, classifyPrompt }
}

// 将路由配置构建为 custom_config 对象
const buildRouterCustomConfig = (): Record<string, any> => {
  const routesMap: Record<string, string> = {}
  for (const r of routerConfig.value.routes) {
    if (r.keyword && r.target) {
      routesMap[r.keyword] = r.target
    }
  }
  const result: Record<string, any> = {
    routing_strategy: routerConfig.value.strategy,
    default_route: routerConfig.value.defaultRoute,
    routes: routesMap,
  }
  // llm_classify 策略下，写入用户自定义 Prompt（非空时）
  if (
    routerConfig.value.strategy === 'llm_classify' &&
    routerConfig.value.classifyPrompt &&
    routerConfig.value.classifyPrompt.trim() !== ''
  ) {
    result.classify_prompt = routerConfig.value.classifyPrompt
  }
  return result
}

// ─── 出站转发策略 (transparent_forward) ───────────────────────────────────
const defaultEgressConfig = () => ({
  route_policy: 'match_model' as 'match_model' | 'fixed',
  inject_system_prompt: false,
  redirect_policy: 'never',
  max_redirects: 5,
  default_scheme: 'https',
})
const egressConfig = ref(defaultEgressConfig())
/** 高级折叠面板：默认全部收起，聚焦常用配置 */
const advancedPanels = ref<string[]>([])

const loadEgressConfig = (node: any) => {
  const cc = node?.config?.custom_config || {}
  let route = String(cc.route_policy || '').trim()
  if (!route) {
    route = cc.fixed_egress === true ? 'fixed' : 'match_model'
  }
  if (route !== 'fixed' && route !== 'match_model') {
    route = 'match_model'
  }
  egressConfig.value = {
    route_policy: route as 'match_model' | 'fixed',
    inject_system_prompt: cc.inject_system_prompt === true,
    redirect_policy: String(cc.redirect_policy || 'never').trim() || 'never',
    max_redirects: Number(cc.max_redirects) > 0 ? Number(cc.max_redirects) : 5,
    default_scheme: String(cc.default_scheme || 'https').trim() || 'https',
  }
}

const buildEgressCustomConfig = (): Record<string, any> => {
  const prev = localNode.value.config?.custom_config || {}
  const next: Record<string, any> = { ...prev }
  next.route_policy = egressConfig.value.route_policy
  next.inject_system_prompt = egressConfig.value.inject_system_prompt
  next.redirect_policy = egressConfig.value.redirect_policy
  next.max_redirects = egressConfig.value.max_redirects
  next.default_scheme = egressConfig.value.default_scheme
  if (egressConfig.value.route_policy === 'fixed') {
    next.fixed_egress = true
  } else {
    delete next.fixed_egress
  }
  return next
}

const ensureTransparentForwardNodeFields = () => {
  localNode.value.kind = 'proxy.transparent_forward'
  localNode.value.implementation = 'builtin.transparent_forward'
  localNode.value.config = localNode.value.config || {}
}

// ─── 插件自定义配置 (Custom Config) ────────────────────────────────────────

// Embedding 后端筛选函数（用于 BackendSelector 的 filter prop）
const embeddingBackendFilter = (b: any): boolean => {
  if (!b || b.enabled !== true) return false
  const backendType = (b.type || '').toLowerCase()
  if (backendType === 'embedding' || backendType === 'ollama') return true
  const features = b.capabilities?.features || []
  if (features.includes('embedding') || features.includes('embeddings')) return true
  return false
}

interface CustomConfigItem {
  key: string
  value: string
}

const customConfigItems = ref<CustomConfigItem[]>([])

// mem0 插件预设字段
const mem0PresetConfig = ref({
  api_key: '',
  base_url: '',
  namespace: 'default',
  search_mode: 'semantic',
  embedding_backend_id: '',
  embedding_model: '',
})

// 判断 key 是否为敏感字段，用密码框显示
const isSecretKey = (key: string): boolean => {
  const lower = (key || '').toLowerCase()
  return lower.includes('key') || lower.includes('secret') || lower.includes('token') || lower.includes('password') || lower.includes('api_key')
}

const addCustomConfigItem = () => {
  customConfigItems.value.push({ key: '', value: '' })
}

const removeCustomConfigItem = (idx: number) => {
  customConfigItems.value.splice(idx, 1)
}

// 填充 mem0 默认字段到动态编辑器
const fillMem0Defaults = () => {
  const defaults = [
    { key: 'api_key', value: mem0PresetConfig.value.api_key },
    { key: 'base_url', value: mem0PresetConfig.value.base_url || 'http://localhost:20061' },
    { key: 'namespace', value: mem0PresetConfig.value.namespace || 'default' },
    { key: 'search_mode', value: mem0PresetConfig.value.search_mode || 'semantic' },
    { key: 'embedding_backend_id', value: mem0PresetConfig.value.embedding_backend_id },
    { key: 'embedding_model', value: mem0PresetConfig.value.embedding_model || 'bge-m3' },
  ]
  for (const d of defaults) {
    const existing = customConfigItems.value.find(i => i.key === d.key)
    if (existing) {
      existing.value = d.value
    } else {
      customConfigItems.value.push({ ...d })
    }
  }
}

// ─── 工具调用注入 (ToolCallInjector) 配置 ───────────────────────────────

interface InjectorToolCall {
  id: string
  functionName: string
  arguments: string
}

const injectorToolCalls = ref<InjectorToolCall[]>([])
const injectorCondition = ref('')

const addInjectorToolCall = () => {
  injectorToolCalls.value.push({ id: '', functionName: '', arguments: '' })
}

const removeInjectorToolCall = (idx: number) => {
  injectorToolCalls.value.splice(idx, 1)
}

const loadInjectorConfig = (node: any) => {
  const cc = node.config?.custom_config || {}
  injectorToolCalls.value = (cc.tool_calls || []).map((tc: any) => ({
    id: tc.id || '',
    functionName: tc.function?.name || '',
    arguments: tc.function?.arguments || '',
  }))
  injectorCondition.value = cc.condition || ''
}

const buildInjectorCustomConfig = (): Record<string, any> => {
  const toolCalls = injectorToolCalls.value
    .filter(tc => tc.functionName.trim())
    .map(tc => ({
      id: tc.id.trim() || `call_${Date.now()}`,
      type: 'function',
      function: {
        name: tc.functionName.trim(),
        arguments: tc.arguments.trim(),
      },
    }))
  return {
    tool_calls: toolCalls,
    condition: injectorCondition.value,
  }
}

// 从 customConfigItems 构建 custom_config 对象（同时合并 mem0 预设字段）
const buildCustomConfig = (): Record<string, any> => {
  const result: Record<string, any> = {}

  // 1. 如果是 mem0 插件，先将预设字段合并进去（预设字段优先）
  if (localNode.value.implementation === 'business.mem0') {
    if (mem0PresetConfig.value.api_key) {
      result['api_key'] = mem0PresetConfig.value.api_key
    }
    if (mem0PresetConfig.value.base_url) {
      result['base_url'] = mem0PresetConfig.value.base_url
    }
    if (mem0PresetConfig.value.namespace) {
      result['namespace'] = mem0PresetConfig.value.namespace
    }
    if (mem0PresetConfig.value.search_mode) {
      result['search_mode'] = mem0PresetConfig.value.search_mode
    }
    if (mem0PresetConfig.value.embedding_backend_id) {
      result['embedding_backend_id'] = mem0PresetConfig.value.embedding_backend_id
    }
    if (mem0PresetConfig.value.embedding_model) {
      result['embedding_model'] = mem0PresetConfig.value.embedding_model
    }
  }

  // 2. 再合并动态编辑器的 key-value（预设字段优先，动态编辑器中的值作为补充）
  for (const item of customConfigItems.value) {
    if (!item.key) continue
    // 只有当预设字段中没有该 key 时，才使用动态编辑器的值
    if (!result.hasOwnProperty(item.key)) {
      result[item.key] = item.value
    }
  }

  return result
}

// 从节点的 custom_config 加载到编辑器状态
const loadCustomConfig = (node: any) => {
  const cc = node.config?.custom_config || {}
  customConfigItems.value = Object.entries(cc).map(([key, value]) => ({
    key,
    value: String(value ?? ''),
  }))

  // 如果是 mem0 插件，同时加载到预设字段
  if (node.implementation === 'business.mem0') {
    mem0PresetConfig.value = {
      api_key: String(cc.api_key ?? ''),
      base_url: String(cc.base_url ?? ''),
      namespace: String(cc.namespace ?? 'default'),
      search_mode: String(cc.search_mode ?? 'semantic'),
      embedding_backend_id: String(cc.embedding_backend_id ?? ''),
      embedding_model: String(cc.embedding_model ?? ''),
    }
    
    // 确保 embedding 后端列表已加载
    loadEmbeddingBackends().then(() => {
      // 如果已配置 embedding_backend_id，加载对应的模型列表
      if (mem0PresetConfig.value.embedding_backend_id) {
        onMem0EmbeddingBackendChange(mem0PresetConfig.value.embedding_backend_id)
      }
    })
  } else {
    mem0PresetConfig.value = { api_key: '', base_url: '', namespace: 'default', search_mode: 'semantic', embedding_backend_id: '', embedding_model: '' }
  }
}

// 加载可用插件列表
const loadPlugins = async () => {
  if (props.plugins && props.plugins.length > 0) {
    return
  }
  loadingPlugins.value = true
  try {
    const res = await getNodePlugins()
    loadedPlugins.value = parseNodePluginsResponse(res)
    if (loadedPlugins.value.length === 0) {
      ElMessage.warning(t('nodeConfig.warnNoPlugins'))
    }
  } catch (err) {
    console.error('Failed to load plugins:', err)
    loadedPlugins.value = []
    ElMessage.error(t('nodeConfig.errorLoadPlugins'))
  } finally {
    loadingPlugins.value = false
  }
}

// 打开插件管理器
const openPluginManager = () => {
  // TODO: 跳转到插件管理页面或打开插件管理对话框
  ElMessage.info(t('nodeConfig.infoPluginManager'))
}

// 判断节点类型是否需要插件选择
const needsPluginSelection = (type: string) => {
  // generator, processor, reviewer, aggregator 需要插件选择
  return ['generator', 'processor', 'reviewer', 'aggregator'].includes(type)
}

// ─── 语义缓存相关状态 ─────────────────────────────────────────────
// 是否语义/混合策略
const isSemanticStrategy = computed(() => {
  const strategy = localNode.value?.config?.customConfig?.strategy
  return strategy === 'semantic' || strategy === 'hybrid'
})

// 条件表达式 placeholder（避免 {{ 被 Vue 模板解析）
const conditionPlaceholder = computed(() => {
  return "e.g.: {{.cache_read.metadata.cache_hit}} == false"
})

// Embedding 后端列表
const embeddingBackends = ref<any[]>([])

// 当前选中 Embedding 后端的模型列表
const embeddingModels = ref<string[]>([])

// ─── 自定义变量绑定（template_vars UI 状态）───────────────────────────────
interface VarBinding {
  key: string
  source: string
  customPath: string
}
const templateVarBindings = ref<VarBinding[]>([])

const addVarBinding = () => {
  templateVarBindings.value.push({ key: '', source: '', customPath: '' })
}

const removeVarBinding = (idx: number) => {
  templateVarBindings.value.splice(idx, 1)
}

const onSourceChange = (binding: VarBinding, val: string) => {
  if (val !== '__custom__') {
    binding.customPath = ''
  }
}

// 将 templateVarBindings 转换为后端需要的 { [key]: path } map
const buildTemplateVarsMap = (): Record<string, string> => {
  const result: Record<string, string> = {}
  for (const b of templateVarBindings.value) {
    if (!b.key) continue
    const path = b.source === '__custom__' ? b.customPath : b.source
    if (path) result[b.key] = path
  }
  return result
}

// 从 map 初始化 templateVarBindings
const loadTemplateVars = (vars: Record<string, string> | undefined) => {
  if (!vars || Object.keys(vars).length === 0) {
    templateVarBindings.value = []
    return
  }
  templateVarBindings.value = Object.entries(vars).map(([key, path]) => {
    const knownSources = [
      'input.content', 'context.timestamp', 'context.user_id', 'context.session_id'
    ]
    const isKnown = knownSources.includes(path) ||
      /^node\.[^.]+\.(content|metadata\.\w+|score|passed|feedback)$/.test(path)
    return {
      key,
      source: isKnown ? path : '__custom__',
      customPath: isKnown ? '' : path
    }
  })
}

// ─── 可用变量面板 ─────────────────────────────────────────────────────────

// 上游节点 ID 列表（依赖节点）
const upstreamNodeIds = computed<string[]>(() => {
  return (localNode.value.depends_on || []).filter(Boolean)
})

// 按节点类型确定内置变量（开箱即用，无需添加绑定）
const builtinVars = computed(() => {
  const type = localNode.value.type
  // 所有节点共用的基础变量
  const common = [
    { name: 'question', desc: t('nodeConfig.builtinVarQuestionDesc') },
  ]
  if (type === 'reviewer') {
    const vars = [
      ...common,
      { name: 'answer',    desc: t('nodeConfig.builtinVarAnswerDesc') },
      { name: 'timestamp', desc: t('nodeConfig.builtinVarTimestampDesc') },
      { name: 'criteria',  desc: t('nodeConfig.builtinVarCriteriaDesc') },
    ]
    for (const nodeId of upstreamNodeIds.value) {
      vars.push({
        name: `${nodeId}_content`,
        desc: t('nodeConfig.nodeOutputDesc', { nodeId }),
      })
    }
    return vars
  }
  if (type === 'processor') {
    const vars = [
      ...common,
      { name: 'input',       desc: t('nodeConfig.builtinVarInputDesc') },
      { name: 'timestamp',   desc: t('nodeConfig.builtinVarTimestampDesc') },
      { name: 'target_lang', desc: t('nodeConfig.builtinVarTargetLangDesc') },
      { name: 'metadata',    desc: t('nodeConfig.builtinVarMetadataDesc') },
    ]
    for (const nodeId of upstreamNodeIds.value) {
      vars.push({
        name: `${nodeId}_content`,
        desc: t('nodeConfig.nodeOutputDesc', { nodeId }),
      })
    }
    return vars
  }
  if (type === 'generator') {
    return [
      { name: 'question', desc: t('nodeConfig.builtinVarQuestionDescGenerator') },
    ]
  }
  return common
})

// 上游节点需要通过 template_vars 绑定才能使用的变量（score/passed/feedback 等非文本字段）
// 注意：{nodeID}_content 已作为内置变量自动注入，无需在此绑定，所以不在此列出。
const upstreamVars = computed(() => {
  const vars: { name: string; desc: string }[] = []
  for (const nodeId of upstreamNodeIds.value) {
    vars.push(
      {
        name: `${nodeId}_score`,
        desc: t('nodeConfig.upstreamVarScoreDesc', { nodeId }),
      },
      {
        name: `${nodeId}_passed`,
        desc: t('nodeConfig.upstreamVarPassedDesc', { nodeId }),
      },
      {
        name: `${nodeId}_feedback`,
        desc: t('nodeConfig.upstreamVarFeedbackDesc', { nodeId }),
      },
    )
  }
  return vars
})

// 执行上下文变量（除 timestamp 对 processor/reviewer 已自动注入外，其余均需在下方添加 template_vars 绑定后使用）
const contextVars = [
  { name: 'timestamp',  desc: t('nodeConfig.contextVarTimestampDesc') },
  { name: 'user_id',    desc: t('nodeConfig.contextVarUserIdDesc') },
  { name: 'session_id', desc: t('nodeConfig.contextVarSessionIdDesc') },
  { name: 'pipeline_id',desc: t('nodeConfig.contextVarPipelineIdDesc') },
]

const usesSystemPrompt = computed(() =>
  localNode.value.type === 'generator'
  || (localNode.value.type === 'transparent_forward' && egressConfig.value.inject_system_prompt),
)

const showPromptEditor = computed(() => {
  if (localNode.value.type === 'transparent_forward') {
    return egressConfig.value.inject_system_prompt
  }
  // transparent_forward 以外：原逻辑（LLM 节点显示 prompt，但 transparent 已单独处理）
  return localNode.value.type !== 'transparent_forward' && needsLLM(localNode.value.type)
})

// generator / 出站注入：绑 system_prompt；其他节点绑 prompt_template
const promptFieldValue = computed({
  get: () => {
    if (!localNode.value.config) return ''
    return usesSystemPrompt.value
      ? (localNode.value.config.system_prompt ?? '')
      : (localNode.value.config.prompt_template ?? '')
  },
  set: (val: string) => {
    localNode.value.config = localNode.value.config || {}
    if (usesSystemPrompt.value) {
      localNode.value.config.system_prompt = val
    } else {
      localNode.value.config.prompt_template = val
    }
  },
})

function onPromptPresetChange(id: string) {
  const preset = getPresetById(id)
  if (!preset) return
  promptFieldValue.value = preset.prompt
}

function restoreDefaultSystemPrompt() {
  selectedPromptPreset.value = 'general'
  promptFieldValue.value = DEFAULT_SYSTEM_PROMPT
}

// 生成模板变量的显示标签（避免 Vue 把 {{ }} 当插值处理）
const varLabel = (name: string) => `{{.${name}}}`

// ─── 插入变量到光标处 ──────────────────────────────────────────────────────
const insertVar = async (varName: string) => {
  const snippet = `{{.${varName}}}`
  const isGenerator = localNode.value.type === 'generator'
  const textarea = promptInputRef.value?.$el?.querySelector('textarea')

  if (!textarea) {
    // 降级：直接追加到对应字段
    promptFieldValue.value = (promptFieldValue.value || '') + snippet
    return
  }

  const start = textarea.selectionStart ?? 0
  const end = textarea.selectionEnd ?? 0
  const current = promptFieldValue.value || ''
  promptFieldValue.value = current.slice(0, start) + snippet + current.slice(end)

  // 恢复光标到插入点之后
  await nextTick()
  const newPos = start + snippet.length
  textarea.setSelectionRange(newPos, newPos)
  textarea.focus()
}

// ─── 表单基础逻辑 ─────────────────────────────────────────────────────────

const rules = {
  id:   [{ required: true, message: t('nodeConfig.validateNodeId'), trigger: 'blur' }],
  name: [{ required: true, message: t('nodeConfig.validateNodeName'), trigger: 'blur' }],
  type: [{ required: true, message: t('nodeConfig.validateNodeType'), trigger: 'change' }],
}

const otherNodes = computed(() => {
  return props.allNodes
    .filter((n: any) => n.id && n.id !== props.node?.id)
    .map((n: any) => n.id)
})

// 获取所有 router 节点（用于 route_config 选择）
const routerNodes = computed(() => {
  return props.allNodes
    .filter((n: any) => n.type === 'router' && n.id !== props.node?.id)
    .map((n: any) => n.id)
})

// 可用的存储列表（过滤出已启用的存储）
const availableStorages = computed(() => {
  if (!props.storages) return []
  return props.storages.filter((s: any) => s.enabled)
})



// 路由配置状态
const hasRouteConfig = ref(false)
const routeConfig = ref({
  router_node_id: '',
  route_value: '',
  is_default: false
})

// 从节点加载路由配置
const loadRouteConfig = (node: any) => {
  if (node.route_config) {
    hasRouteConfig.value = true
    routeConfig.value = {
      router_node_id: node.route_config.router_node_id || '',
      route_value: node.route_config.route_value || '',
      is_default: node.route_config.is_default || false
    }
  } else {
    hasRouteConfig.value = false
    routeConfig.value = { router_node_id: '', route_value: '', is_default: false }
  }
}

// 保存路由配置到节点
const saveRouteConfig = () => {
  if (hasRouteConfig.value && routeConfig.value.router_node_id && routeConfig.value.route_value) {
    localNode.value.route_config = {
      router_node_id: routeConfig.value.router_node_id,
      route_value: routeConfig.value.route_value,
      is_default: routeConfig.value.is_default
    }
  } else {
    delete localNode.value.route_config
  }
}

const needsLLM = (type: string) => {
  return ['generator', 'processor', 'reviewer', 'aggregator', 'transparent_forward'].includes(type)
}

const getPromptLabel = (type: string) => {
  const labels: Record<string, string> = {
    generator:  t('nodeConfig.promptLabelGenerator'),
    transparent_forward: t('nodeConfig.promptLabelTransparentForward'),
    aggregator: t('nodeConfig.promptLabelAggregator'),
  }
  return labels[type] || t('nodeConfig.promptLabelDefault')
}

const getPromptPlaceholder = (type: string) => {
  const placeholders: Record<string, string> = {
    generator:  t('nodeConfig.promptPlaceholderGenerator'),
    transparent_forward: t('nodeConfig.promptPlaceholderTransparentForward'),
    aggregator: t('nodeConfig.promptPlaceholderAggregator'),
  }
  return placeholders[type] || t('nodeConfig.promptPlaceholderDefault')
}

const getTypeDescription = (type: string) => {
  const desc: Record<string, string> = {
    transparent_forward: t('nodeConfig.typeDescTransparentForward'),
    generator:  t('nodeConfig.typeDescGenerator'),
    processor:  t('nodeConfig.typeDescProcessor'),
    reviewer:   t('nodeConfig.typeDescReviewer'),
    router:     t('nodeConfig.typeDescRouter'),
    aggregator: t('nodeConfig.typeDescAggregator'),
    parallel:   t('nodeConfig.typeDescParallel'),
    cache:       t('nodeConfig.typeDescCache'),
    token_usage: t('nodeConfig.typeDescTokenUsage'),
    tool_call_injector: t('nodeConfig.typeDescToolCallInjector'),
  }
  return desc[type] || t('nodeConfig.typeDescDefault')
}

const defaultTokenUsageCustomConfig = () => ({
  operation: 'record',
  storage_type: 'sqlite',
  record_fields: ['prompt_tokens', 'completion_tokens', 'total_tokens', 'model', 'backend_id'],
})

const ensureTokenUsageNodeFields = () => {
  localNode.value.kind = 'metrics.token_usage'
  localNode.value.implementation = 'builtin.token_usage'
  localNode.value.config = localNode.value.config || {}
  const backendCC = localNode.value.config.custom_config || {}
  const frontendCC = localNode.value.config.customConfig || {}
  localNode.value.config.customConfig = {
    ...defaultTokenUsageCustomConfig(),
    ...backendCC,
    ...frontendCC,
    record_fields: frontendCC.record_fields?.length
      ? frontendCC.record_fields
      : backendCC.record_fields?.length
        ? backendCC.record_fields
        : defaultTokenUsageCustomConfig().record_fields,
  }
}

const onBackendChange = (backendId: string | null) => {
  if (backendId !== localNode.value.backend) {
    localNode.value.model = ''
  }
}

const llmUpstreamTypes = new Set(['generator', 'transparent_forward', 'processor', 'reviewer'])

const findLLMUpstreamNodeIDs = () => {
  return props.allNodes
    .filter((n: any) => n.id && n.id !== localNode.value.id && llmUpstreamTypes.has(n.type))
    .map((n: any) => n.id)
}

const ensureTokenUsageDependsOn = () => {
  const upstream = findLLMUpstreamNodeIDs()
  if (upstream.length === 0) {
    return
  }
  const current = new Set(localNode.value.depends_on || [])
  let changed = false
  for (const id of upstream) {
    if (!current.has(id)) {
      current.add(id)
      changed = true
    }
  }
  if (changed) {
    localNode.value.depends_on = Array.from(current)
    ElMessage.info(t('nodeConfig.infoAutoDependTokenUsage'))
  }
}

const onTypeChange = (newType: string) => {
  if (newType === 'token_usage') {
    const wasGenerator = props.node?.type === 'generator'
    const hadBackend = !!(props.node?.backend || props.node?.config?.backend)
    const otherGenerators = props.allNodes.filter(
      (n: any) => n.id !== localNode.value.id && n.type === 'generator' && (n.backend || n.config?.backend),
    )
    if (wasGenerator && hadBackend && otherGenerators.length === 0) {
      ElMessage.warning(t('nodeConfig.warnCannotConvertSoleGenerator'))
    }
    if (!localNode.value.name || localNode.value.name === t('nodeConfig.defaultNameNewNode')) {
      localNode.value.name = t('nodeConfig.typeTokenUsage')
    }
    ensureTokenUsageNodeFields()
    ensureTokenUsageDependsOn()
    localNode.value.backend = ''
    localNode.value.model = ''
    return
  }

  // router 在 llm_classify 策略下也需要 backend/model，切换时保留
  const needsLLMOrRouter = needsLLM(newType) || newType === 'router'
  if (!needsLLMOrRouter) {
    localNode.value.backend = ''
    localNode.value.model = ''
    localNode.value.config = localNode.value.config || {}
    delete localNode.value.config.prompt_template
    return
  }
  if (newType === 'generator') {
    localNode.value.config = localNode.value.config || {}
    // generator 节点只使用 system_prompt，不使用 prompt_template，清除旧值避免误解
    delete localNode.value.config.prompt_template
  }
  if (newType === 'transparent_forward') {
    ensureTransparentForwardNodeFields()
    delete localNode.value.config.prompt_template
    if (!localNode.value.name || localNode.value.name === t('nodeConfig.defaultNameNewNode')) {
      localNode.value.name = t('nodeConfig.defaultNameEgressForward')
    }
    // 默认透明策略；用户可在「出站策略」改成直连/跳板
    egressConfig.value = defaultEgressConfig()
  }
}

// ─── Embedding 配置相关方法 ───────────────────────────────

// 加载支持 Embedding 的后端列表
const loadEmbeddingBackends = async () => {
  try {
    const res = await getBackends()
    // API 拦截器已处理：res 直接是后端数组
    const allBackends = Array.isArray(res) ? res : (res.data || [])
    
    console.log(`[Embedding] Total backends loaded: ${allBackends.length}`)
    
    // 严格筛选：1. 必须启用  2. 必须支持 embedding
    embeddingBackends.value = allBackends.filter((b: any) => {
      // 1. 必须启用（严格要求 enabled === true）
      if (b.enabled !== true) {
        console.log(`[Embedding] Skipping disabled backend: ${b.id}`)
        return false
      }
      
      const backendType = (b.type || '').toLowerCase()
      const capabilities = b.capabilities || {}
      const features = Array.isArray(capabilities.features) ? capabilities.features : []
      const metadata = b.metadata || {}
      
      // 2. 后端类型明确为 embedding
      if (backendType === 'embedding') {
        console.log(`[Embedding] Including backend ${b.id}: type=embedding`)
        return true
      }

      // 3. Capabilities 中声明支持 embeddings 功能
      if (features.includes('embeddings') || features.includes('embedding')) {
        console.log(`[Embedding] Including backend ${b.id}: has embedding feature`)
        return true
      }

      // 4. Ollama 后端：天然支持 /api/embeddings 端点
      if (backendType === 'ollama') {
        console.log(`[Embedding] Including backend ${b.id}: type=ollama`)
        return true
      }

      // 5. 元数据中标明用途为 embedding
      const metaUsage = (metadata.usage || metadata.purpose || '').toLowerCase()
      if (metaUsage.includes('embedding') || metaUsage.includes('embed')) {
        console.log(`[Embedding] Including backend ${b.id}: metadata usage contains embedding`)
        return true
      }

      // 6. 名称/描述中包含常见 embedding 模型关键词
      const name = (b.name || '').toLowerCase()
      const desc = (b.description || '').toLowerCase()
      const embeddingKeywords = ['embedding', 'embed', 'bge', 'gte', 'e5', 'sentence', 'nomic', 'mxbai', 'all-minilm']
      for (const keyword of embeddingKeywords) {
        if (name.includes(keyword) || desc.includes(keyword)) {
          console.log(`[Embedding] Including backend ${b.id}: name/desc contains ${keyword}`)
          return true
        }
      }

      console.log(`[Embedding] Excluding backend ${b.id}: does not support embedding`)
      return false
    })
    
    console.log(`[Embedding] Final filtered backends: ${embeddingBackends.value.length}`, 
      embeddingBackends.value.map(b => ({ id: b.id, name: b.name, type: b.type, enabled: b.enabled })))
  } catch (err) {
    console.error('[Embedding] Failed to load backends:', err)
    embeddingBackends.value = []
  }
}

// 当 Embedding 后端选择变化时，加载对应的模型列表
const onEmbeddingBackendChange = async (backendId: string | null) => {
  if (!backendId) {
    embeddingModels.value = []
    localNode.value.config = localNode.value.config || {}
    localNode.value.config.customConfig = localNode.value.config.customConfig || {}
    localNode.value.config.customConfig.embedding_model = ''
    return
  }
  try {
    // 先尝试获取 embedding 类型的模型
    const res = await getBackendModels(backendId, 'embedding')
    embeddingModels.value = res.data || res || []
    console.log(`[Cache] Loaded ${embeddingModels.value.length} embedding models for backend ${backendId}`)
    
    // 如果 embedding 类型模型为空，fallback 到获取所有模型
    if (embeddingModels.value.length === 0) {
      console.log(`[Cache] No embedding models found, falling back to all models for backend ${backendId}`)
      const allRes = await getBackendModels(backendId)
      embeddingModels.value = allRes.data || allRes || []
      console.log(`[Cache] Loaded ${embeddingModels.value.length} total models (fallback)`)
    }
  } catch (err) {
    console.error('[Cache] Failed to load models:', err)
    // 如果加载失败，尝试不带 embedding 类型参数的通用模型加载
    try {
      const res = await getBackendModels(backendId)
      embeddingModels.value = res.data || res || []
      console.log(`[Cache] Loaded ${embeddingModels.value.length} models (error fallback)`)
    } catch (fallbackErr) {
      console.error('[Cache] Failed to load models (fallback):', fallbackErr)
      embeddingModels.value = []
    }
  }
}

// Mem0 插件：当 Embedding 后端选择变化时，加载对应的模型列表
const onMem0EmbeddingBackendChange = async (backendId: string | null) => {
  if (!backendId) {
    embeddingModels.value = []
    mem0PresetConfig.value.embedding_model = ''
    return
  }
  try {
    // 先尝试获取 embedding 类型的模型
    const res = await getBackendModels(backendId, 'embedding')
    embeddingModels.value = res.data || res || []
    console.log(`[Mem0] Loaded ${embeddingModels.value.length} embedding models for backend ${backendId}`)
    
    // 如果 embedding 类型模型为空，fallback 到获取所有模型
    if (embeddingModels.value.length === 0) {
      console.log(`[Mem0] No embedding models found, falling back to all models for backend ${backendId}`)
      const allRes = await getBackendModels(backendId)
      embeddingModels.value = allRes.data || allRes || []
      console.log(`[Mem0] Loaded ${embeddingModels.value.length} total models (fallback)`)
    }
  } catch (err) {
    console.error('[Mem0] Failed to load models:', err)
    // 如果加载失败，尝试不带 embedding 类型参数的通用模型加载
    try {
      const res = await getBackendModels(backendId)
      embeddingModels.value = res.data || res || []
      console.log(`[Mem0] Loaded ${embeddingModels.value.length} models (error fallback)`)
    } catch (fallbackErr) {
      console.error('[Mem0] Failed to load models (fallback):', fallbackErr)
      embeddingModels.value = []
    }
  }
}

// 监听策略变化，如果是语义/混合策略则加载 Embedding 后端列表
watch(() => localNode.value?.config?.customConfig?.strategy, (newStrategy) => {
  if (newStrategy === 'semantic' || newStrategy === 'hybrid') {
    loadEmbeddingBackends()
  }
})

// 初始化时如果已经是语义/混合策略，加载 Embedding 后端列表
if (isSemanticStrategy.value) {
  loadEmbeddingBackends()
}

const saveNode = async () => {
  if (!formRef.value) return
  await formRef.value.validate(async (valid: boolean) => {
    if (!valid) return
    
    // 将 templateVarBindings 序列化回 config.template_vars
    const tvMap = buildTemplateVarsMap()
    if (Object.keys(tvMap).length > 0) {
      localNode.value.config = localNode.value.config || {}
      localNode.value.config.template_vars = tvMap
    } else {
      delete localNode.value.config?.template_vars
    }
    // 保存路由配置
    saveRouteConfig()

    // Router 节点：保存 custom_config（路由策略、默认路由、规则映射）
    if (localNode.value.type === 'router') {
      const cc = buildRouterCustomConfig()
      localNode.value.config = localNode.value.config || {}
      localNode.value.config.custom_config = cc
    }

    if (localNode.value.type === 'token_usage') {
      ensureTokenUsageNodeFields()
      ensureTokenUsageDependsOn()
      const cc = localNode.value.config.customConfig
      localNode.value.config.custom_config = {
        operation: cc.operation || 'record',
        storage_type: cc.storage_type || 'sqlite',
        record_fields: cc.record_fields?.length
          ? cc.record_fields
          : defaultTokenUsageCustomConfig().record_fields,
      }
      delete localNode.value.config.customConfig
    }

    // 缓存节点：将 customConfig (camelCase) 转换为 custom_config (snake_case) 以兼容后端
    if (localNode.value.type === 'cache' && localNode.value.config?.customConfig) {
      const cc = localNode.value.config.customConfig
      localNode.value.config.custom_config = {
        operation: cc.operation,
        strategy: cc.strategy,
        storage_type: cc.storage_type || 'memory',
        key_template: cc.key_template,
        ttl: cc.ttl,
        read_storage_name: cc.read_storage_name || '',
        write_storage_name: cc.write_storage_name || '',
        // 向量存储：优先使用单独配置的 vector_storage_name，否则使用 write_storage_name
        vector_storage_name: cc.vector_storage_name || cc.write_storage_name || '',
        embedding_backend_id: cc.embedding_backend_id || '',
        embedding_model: cc.embedding_model || '',
        semantic_threshold: cc.semantic_threshold || 0.85,
        semantic_top_k: cc.semantic_top_k || 5
      }
      // 删除前端使用的 camelCase 版本，避免混淆
      delete localNode.value.config.customConfig
    }

    // ToolCallInjector：构建 custom_config（工具调用列表 + 注入条件）
    if (localNode.value.type === 'tool_call_injector') {
      const cc = buildInjectorCustomConfig()
      localNode.value.config = localNode.value.config || {}
      localNode.value.config.custom_config = cc
    }

    // 出站转发：写入 route_policy / inject_system_prompt 等开关
    if (localNode.value.type === 'transparent_forward') {
      ensureTransparentForwardNodeFields()
      localNode.value.config.custom_config = buildEgressCustomConfig()
    }

    // 消除双源字段歧义：将顶层 backend/model 同步到 config 内层，确保执行引擎和回填逻辑读取一致
    if (localNode.value.backend !== undefined) {
      localNode.value.config = localNode.value.config || {}
      localNode.value.config.backend = localNode.value.backend
    }
    if (localNode.value.model !== undefined) {
      localNode.value.config = localNode.value.config || {}
      localNode.value.config.model = localNode.value.model
    }

    // 带插件的节点：保存 custom_config
    if (needsPluginSelection(localNode.value.type)) {
      const cc = buildCustomConfig()
      if (Object.keys(cc).length > 0) {
        localNode.value.config = localNode.value.config || {}
        localNode.value.config.custom_config = cc
      } else {
        delete localNode.value.config?.custom_config
      }
    }

    // 发送更新到父组件（仅一次，避免重复）
    emit('update:node', { ...localNode.value })

    // 节点数据已在画布中更新，保存由主保存按钮统一处理
    ElMessage.success(t('nodeConfig.successNodeUpdated'))
    
    handleClose()
    
    handleClose()
  })
}

const handleClose = () => {
  emit('update:visible', false)
}

// ─── 监听 ─────────────────────────────────────────────────────────────────

watch(() => props.node, (newNode) => {
  if (newNode) {
    localNode.value = {
      ...newNode,
      timeout: newNode.timeout ?? 120,
      retry: newNode.retry || {
        max_attempts: 0, backoff_strategy: 'exponential',
        initial_delay: 1000, max_delay: 30000,
      },
      config:     newNode.config || {},
      depends_on: newNode.depends_on || [],
      next_nodes: newNode.next_nodes || [],
    }
    // 消除双源字段歧义：如果 config 内层为空但顶层有值，将顶层同步到 config（backward compatibility）
    if (localNode.value.backend && !localNode.value.config?.backend) {
      localNode.value.config = localNode.value.config || {}
      localNode.value.config.backend = localNode.value.backend
    }
    if (localNode.value.model && !localNode.value.config?.model) {
      localNode.value.config = localNode.value.config || {}
      localNode.value.config.model = localNode.value.model
    }
    if (localNode.value.type === 'token_usage') {
      ensureTokenUsageNodeFields()
    }
    if (localNode.value.type === 'tool_call_injector') {
      loadInjectorConfig(newNode)
    }
    // 缓存节点默认配置
    if (localNode.value.type === 'cache') {
      localNode.value.config = localNode.value.config || {}
      // 后端使用 custom_config (snake_case)，前端使用 customConfig (camelCase)
      // 需要映射两者的数据
      const backendCustomConfig = localNode.value.config.custom_config || {}
      const frontendCustomConfig = localNode.value.config.customConfig || {}
      
      // 合并后端数据和前端数据（前端数据优先）
      localNode.value.config.customConfig = {
        operation: backendCustomConfig.operation || frontendCustomConfig.operation || 'read',
        strategy: backendCustomConfig.strategy || frontendCustomConfig.strategy || 'exact',
        storage_type: backendCustomConfig.storage_type || frontendCustomConfig.storage_type || 'memory',
        key_template: backendCustomConfig.key_template || frontendCustomConfig.key_template || '{{model}}:{{hash}}',
        ttl: backendCustomConfig.ttl || frontendCustomConfig.ttl || 3600,
        read_storage_name: backendCustomConfig.read_storage_name || frontendCustomConfig.read_storage_name || '',
        write_storage_name: backendCustomConfig.write_storage_name || frontendCustomConfig.write_storage_name || '',
        // 语义缓存配置（添加默认值）
        vector_storage_name: backendCustomConfig.vector_storage_name || frontendCustomConfig.vector_storage_name || '',
        embedding_backend_id: backendCustomConfig.embedding_backend_id || frontendCustomConfig.embedding_backend_id || '',
        embedding_model: backendCustomConfig.embedding_model || frontendCustomConfig.embedding_model || '',
        semantic_threshold: backendCustomConfig.semantic_threshold || frontendCustomConfig.semantic_threshold || 0.85,
        semantic_top_k: backendCustomConfig.semantic_top_k || frontendCustomConfig.semantic_top_k || 5
      }
      
      // 如果是语义/混合策略，加载 embedding 后端列表
      const strategy = localNode.value.config.customConfig.strategy
      if (strategy === 'semantic' || strategy === 'hybrid') {
        loadEmbeddingBackends().then(() => {
          // 如果已配置 embedding_backend_id，加载对应的模型列表
          if (localNode.value.config.customConfig.embedding_backend_id) {
            onEmbeddingBackendChange(localNode.value.config.customConfig.embedding_backend_id)
          }
        })
      }
    }
    if (localNode.value.type === 'generator') {
      localNode.value.config = localNode.value.config || {}
      if (!localNode.value.config.prompt_template) {
        localNode.value.config.prompt_template = '{{input}}'
      }
    }
    // 加载 template_vars
    loadTemplateVars(localNode.value.config?.template_vars)
    // 加载路由配置
    loadRouteConfig(newNode)
    // 加载 Router 节点专用配置
    if (localNode.value.type === 'router') {
      loadRouterConfig(newNode)
    }
    // 加载出站转发策略
    if (localNode.value.type === 'transparent_forward') {
      ensureTransparentForwardNodeFields()
      loadEgressConfig(newNode)
    } else {
      egressConfig.value = defaultEgressConfig()
    }
    // 加载插件自定义配置
    loadCustomConfig(newNode)
  }
}, { immediate: true })

function onDrawerOpened() {
  loadPlugins()
  // 延迟执行，确保 localNode 已由 props.node watch 填充
  setTimeout(() => {
    if (isSemanticStrategy.value) {
      loadEmbeddingBackends()
    }
    if (localNode.value.implementation === 'business.mem0') {
      loadEmbeddingBackends()
      if (mem0PresetConfig.value.embedding_backend_id) {
        onMem0EmbeddingBackendChange(mem0PresetConfig.value.embedding_backend_id)
      }
    }
  }, 0)
}

watch(() => props.visible, (val) => {
  if (!val) {
    localNode.value = { retry: {}, config: {}, depends_on: [], next_nodes: [] }
    templateVarBindings.value = []
    customConfigItems.value = []
    injectorToolCalls.value = []
    injectorCondition.value = ''
    egressConfig.value = defaultEgressConfig()
    advancedPanels.value = []
    mem0PresetConfig.value = { api_key: '', base_url: '', namespace: 'default', search_mode: 'semantic', embedding_backend_id: '', embedding_model: '' }
    return
  }
  onDrawerOpened()
}, { immediate: true })

// 降级策略列表
const fallbackPolicies = ref<any[]>([])
const loadingFallbackPolicies = ref(false)

async function loadFallbackPolicies() {
  loadingFallbackPolicies.value = true
  try {
    const { getFallbackPolicies } = await import('../api/fallback')
    const res = await getFallbackPolicies()
    fallbackPolicies.value = Array.isArray(res) ? res : Array.isArray((res as any)?.data) ? (res as any).data : []
  } catch (err) {
    console.error('Failed to load fallback policies', err)
  } finally {
    loadingFallbackPolicies.value = false
  }
}

onMounted(() => {
  loadFallbackPolicies()
  if (props.visible) {
    onDrawerOpened()
  }
})
</script>

<style scoped>
.node-drawer-form {
  padding-bottom: 8px;
}

.drawer-section {
  margin-bottom: 18px;
  padding-bottom: 4px;
}

.drawer-section + .drawer-section {
  border-top: 1px solid #ebeef5;
  padding-top: 16px;
}

.section-title {
  font-size: 14px;
  font-weight: 600;
  color: #303133;
  margin-bottom: 12px;
  letter-spacing: 0.02em;
}

.section-alert {
  margin-bottom: 12px;
}

.egress-prompt-block {
  margin-top: 4px;
}

.egress-inject-item {
  margin-bottom: 8px;
}

.egress-inject-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.egress-inject-label {
  font-size: 14px;
  color: #606266;
  line-height: 1;
  white-space: nowrap;
}

.egress-prompt-body {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-top: 4px;
  padding: 14px 14px 12px;
  background: #fafbfc;
  border: 1px solid #ebeef5;
  border-radius: 8px;
}

.egress-prompt-label {
  font-size: 13px;
  font-weight: 500;
  color: #606266;
}

.egress-prompt-body .system-prompt-toolbar {
  margin-bottom: 0;
}

.egress-prompt-body .help-text {
  margin-top: 0;
}

.drawer-section-advanced {
  border-top: 1px solid #ebeef5;
  padding-top: 8px;
}

.advanced-collapse {
  border: none;
  --el-collapse-header-height: 42px;
}

.advanced-collapse :deep(.el-collapse-item__header) {
  font-size: 13px;
  color: #606266;
  background: #fafafa;
  padding: 0 12px;
  border-radius: 6px;
  margin-bottom: 6px;
  border: 1px solid #ebeef5;
}

.advanced-collapse :deep(.el-collapse-item__wrap) {
  border: none;
  background: transparent;
}

.advanced-collapse :deep(.el-collapse-item__content) {
  padding: 8px 4px 12px;
}

.type-desc {
  font-size: 12px;
  color: #606266;
  line-height: 1.4;
  margin-top: 4px;
}

.retry-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

.system-prompt-toolbar {
  display: flex;
  gap: 8px;
  align-items: center;
  margin-bottom: 8px;
}
.help-text {
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
}

.footer-buttons {
  display: flex;
  gap: 12px;
  justify-content: flex-end;
}

/* ── 可用变量面板 ── */
.var-panel {
  margin-top: 8px;
  border-radius: 6px;
  border: 1px solid #e4e7ed;
  overflow: hidden;
}

.var-panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  background: #f5f7fa;
  border-bottom: 1px solid #e4e7ed;
}

.var-panel-title {
  font-size: 12px;
  font-weight: 600;
  color: #303133;
}

.var-panel-hint {
  font-size: 11px;
  color: #909399;
}

.var-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12px;
  background: #fff;
}

.var-table thead tr {
  background: #fafafa;
}

.var-table th {
  padding: 6px 10px;
  color: #909399;
  font-weight: 500;
  text-align: left;
  border-bottom: 1px solid #ebeef5;
}

.var-table td {
  padding: 5px 10px;
  border-bottom: 1px solid #f2f2f2;
  vertical-align: middle;
}

.var-table tr:last-child td {
  border-bottom: none;
}

.var-section-row td.var-section-label {
  padding: 4px 10px;
  font-size: 11px;
  font-weight: 600;
  color: #606266;
  background: #f5f7fa;
  border-bottom: 1px solid #ebeef5;
}

.var-desc {
  color: #606266;
  line-height: 1.5;
}

.var-chip {
  cursor: pointer;
  font-family: 'SFMono-Regular', Consolas, monospace;
  font-size: 11px;
  transition: opacity 0.15s;
  white-space: nowrap;
}

.var-chip:hover {
  opacity: 0.72;
}

/* ── 自定义变量绑定编辑器 ── */
.template-vars-editor {
  width: 100%;
}

.template-vars-hint {
  font-size: 12px;
  color: #606266;
  margin-bottom: 10px;
  line-height: 1.5;
}

.template-vars-hint code {
  background: #f0f2f5;
  padding: 1px 5px;
  border-radius: 3px;
  font-family: 'SFMono-Regular', Consolas, monospace;
  color: #e6a23c;
  font-size: 12px;
}

.template-var-row {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 8px;
}

.var-eq {
  font-size: 14px;
  color: #909399;
  flex-shrink: 0;
}

/* ── 插件自定义配置编辑器 ── */
.custom-config-editor {
  width: 100%;
}

.custom-config-hint {
  font-size: 12px;
  color: #606266;
  margin-bottom: 10px;
  line-height: 1.5;
}

.custom-config-row {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 8px;
}

.custom-config-actions {
  display: flex;
  gap: 8px;
  margin-top: 6px;
}

/* ── 工具调用注入配置 ── */
.injector-tc-list {
  width: 100%;
}

.injector-tc-item {
  border: 1px solid #e4e7ed;
  border-radius: 4px;
  padding: 10px;
  margin-bottom: 8px;
  background: #fafafa;
}

.injector-tc-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
  font-size: 13px;
  font-weight: 500;
  color: #606266;
}

.injector-tc-body {
  display: flex;
  flex-direction: column;
}
</style>

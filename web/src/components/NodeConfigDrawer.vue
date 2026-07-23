<template>
  <el-drawer
    :model-value="visible"
    @update:model-value="emit('update:visible', $event)"
    :title="node?.name || '节点配置'"
    size="960px"
    direction="rtl"
    :before-close="handleClose"
  >
    <el-form 
      ref="formRef" 
      :model="localNode" 
      label-position="top"
      :rules="rules"
    >
      <!-- 基本信息 -->
      <el-form-item label="节点ID" prop="id">
        <el-input v-model="localNode.id" placeholder="唯一标识符" />
      </el-form-item>

      <el-form-item label="节点名称" prop="name">
        <el-input v-model="localNode.name" placeholder="显示名称" />
      </el-form-item>

      <el-form-item label="节点类型" prop="type">
        <el-select v-model="localNode.type" style="width: 100%" @change="onTypeChange">
          <el-option label="出站转发 (Transparent Forward)" value="transparent_forward" />
          <el-option label="生成器 (Generator)" value="generator" />
          <el-option label="处理器 (Processor)" value="processor" />
          <el-option label="审核器 (Reviewer)" value="reviewer" />
          <el-option label="路由器 (Router)" value="router" />
          <el-option label="聚合器 (Aggregator)" value="aggregator" />
          <el-option label="并行节点 (Parallel)" value="parallel" />
          <el-option label="缓存 (Cache)" value="cache" />
          <el-option label="Token 计量 (Token Usage)" value="token_usage" />
          <el-option label="工具调用注入 (Tool Call Injector)" value="tool_call_injector" />
        </el-select>
        <div class="type-desc">
          {{ getTypeDescription(localNode.type) }}
        </div>
      </el-form-item>

      <!-- 插件实现选择（仅对需要插件的节点类型显示） -->
      <template v-if="needsPluginSelection(localNode.type)">
        <el-divider>插件实现选择</el-divider>
        <PluginSelector
          v-model="localNode.implementation"
          v-model:kind="localNode.kind"
          :plugins="availablePlugins"
          show-kind-selector
          show-all-plugins
          @view-all="openPluginManager"
        />
      </template>

      <!-- LLM 配置 - 仅对需要大模型的节点显示 -->
      <template v-if="needsLLM(localNode.type)">
        <el-divider>大模型配置</el-divider>

        <el-form-item label="后端服务" prop="backend">
          <BackendSelector
            v-model="localNode.backend"
            placeholder="选择后端"
            style="width: 100%"
            @change="onBackendChange"
          />
        </el-form-item>

        <el-form-item label="模型名称" prop="model">
          <ModelSelector
            v-model="localNode.model"
            :backend-id="localNode.backend"
            placeholder="选择或输入模型"
            :allow-create="true"
            :default-first-option="true"
          />
        </el-form-item>

        <el-form-item v-if="showPromptEditor" :label="getPromptLabel(localNode.type)">
          <div v-if="usesSystemPrompt" class="system-prompt-toolbar">
            <el-select
              v-model="selectedPromptPreset"
              clearable
              placeholder="人格预设"
              style="width: 160px"
              @change="onPromptPresetChange"
            >
              <el-option
                v-for="p in systemPromptPresets"
                :key="p.id"
                :label="p.label"
                :value="p.id"
              />
            </el-select>
            <el-button size="small" @click="restoreDefaultSystemPrompt">恢复默认</el-button>
          </div>
          <el-input
            ref="promptInputRef"
            v-model="promptFieldValue"
            type="textarea"
            :rows="8"
            :placeholder="getPromptPlaceholder(localNode.type)"
          />
          <div v-if="localNode.type === 'generator'" class="help-text">
            非空 system prompt 会覆盖客户端 system 消息。
          </div>
          <div v-if="localNode.type === 'transparent_forward'" class="help-text">
            开启「注入 System Prompt」后生效：用网关人格替换客户端 system 消息（对应直连 #d）。
          </div>
          <div v-if="localNode.type === 'aggregator'" class="help-text">
            可选。用于指导 LLM 如何聚合多个上游节点的输出。留空将使用默认总结 prompt。可用变量：&#123;&#123;.combined_content&#125;&#125;（所有上游输出的拼接）
          </div>

          <!-- 可用变量面板 -->
          <div class="var-panel">
            <div class="var-panel-header">
              <span class="var-panel-title">可用变量</span>
              <span class="var-panel-hint">点击变量名插入到光标处</span>
            </div>

            <table class="var-table">
              <colgroup>
                <col style="width: 190px" />
                <col />
              </colgroup>
              <thead>
                <tr>
                  <th>变量名（点击插入）</th>
                  <th>说明</th>
                </tr>
              </thead>

              <!-- 内置变量 -->
              <tbody>
                <tr class="var-section-row">
                  <td colspan="2" class="var-section-label">📌 内置变量（开箱即用）</td>
                </tr>
                <tr v-for="v in builtinVars" :key="v.name">
                  <td>
                    <el-tag class="var-chip" type="primary" size="small" @click="insertVar(v.name)">
                      {{ varLabel(v.name) }}
                    </el-tag>
                  </td>
                  <td class="var-desc">{{ v.desc }}</td>
                </tr>

                <!-- 上游节点变量 -->
                <template v-if="upstreamVars.length > 0">
                  <tr class="var-section-row">
                    <td colspan="2" class="var-section-label">🔗 上游节点（需在下方添加变量绑定后使用）</td>
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

                <!-- 执行上下文变量 -->
                <tr class="var-section-row">
                  <td colspan="2" class="var-section-label">⚙️ 执行上下文（需在下方添加变量绑定后使用）</td>
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
        </el-form-item>

        <!-- 自定义变量绑定（template_vars） -->
        <el-form-item
          v-if="localNode.type === 'reviewer' || localNode.type === 'processor'"
          label="自定义变量绑定"
        >
          <div class="template-vars-editor">
            <div class="template-vars-hint">
              将数据路径绑定为自定义变量名，在 Prompt 中用 <code v-text="'{{.变量名}}'"></code> 引用
            </div>

            <div
              v-for="(binding, idx) in templateVarBindings"
              :key="idx"
              class="template-var-row"
            >
              <el-input
                v-model="binding.key"
                placeholder="变量名，如 my_answer"
                size="small"
                style="width: 140px; flex-shrink: 0"
              />
              <span class="var-eq">=</span>
              <el-select
                v-model="binding.source"
                placeholder="数据来源"
                size="small"
                style="width: 130px; flex-shrink: 0"
                @change="(v: string) => onSourceChange(binding, v)"
              >
                <el-option label="原始用户输入" value="input.content" />
                <el-option label="当前时间戳" value="context.timestamp" />
                <el-option label="用户 ID" value="context.user_id" />
                <el-option label="会话 ID" value="context.session_id" />
                <el-option
                  v-for="n in upstreamNodeIds"
                  :key="n + '_content'"
                  :label="`节点 ${n} 的输出内容`"
                  :value="`node.${n}.content`"
                />
                <el-option
                  v-for="n in upstreamNodeIds"
                  :key="n + '_score'"
                  :label="`节点 ${n} 的评分`"
                  :value="`node.${n}.score`"
                />
                <el-option
                  v-for="n in upstreamNodeIds"
                  :key="n + '_passed'"
                  :label="`节点 ${n} 是否通过审核`"
                  :value="`node.${n}.passed`"
                />
                <el-option
                  v-for="n in upstreamNodeIds"
                  :key="n + '_feedback'"
                  :label="`节点 ${n} 的审核反馈`"
                  :value="`node.${n}.feedback`"
                />
                <el-option label="自定义路径..." value="__custom__" />
              </el-select>
              <el-input
                v-if="binding.source === '__custom__'"
                v-model="binding.customPath"
                placeholder="如 node.generator.metadata.tokens"
                size="small"
                style="flex: 1"
              />
              <el-button
                type="danger"
                text
                size="small"
                @click="removeVarBinding(idx)"
              >
                <el-icon><Delete /></el-icon>
              </el-button>
            </div>

            <el-button size="small" @click="addVarBinding" style="margin-top: 6px">
              <el-icon><Plus /></el-icon>
              添加变量绑定
            </el-button>
          </div>
        </el-form-item>
      </template>

      <!-- 出站转发策略（直连 #d / 透明 #t / 跳板 #j 共用节点） -->
      <template v-if="localNode.type === 'transparent_forward'">
        <el-divider>出站策略</el-divider>
        <el-alert type="info" :closable="false" style="margin-bottom: 12px">
          <template #default>
            <div style="font-size: 13px; line-height: 1.6">
              与预置模板对应：<strong>透明 #t</strong>＝按模型选路 + 不注入；
              <strong>跳板 #j</strong>＝固定出站 + 不注入；
              <strong>直连 #d</strong>＝固定出站 + 注入 System Prompt。
            </div>
          </template>
        </el-alert>

        <el-form-item label="选路策略">
          <el-radio-group v-model="egressConfig.route_policy">
            <el-radio-button value="match_model">按模型匹配（透明）</el-radio-button>
            <el-radio-button value="fixed">固定出站（直连/跳板）</el-radio-button>
          </el-radio-group>
          <div class="help-text">
            固定出站：只走上方选中的后端/模型（或系统默认）。按模型匹配：根据客户端 model 跨后端松匹配。
          </div>
        </el-form-item>

        <el-form-item label="注入 System Prompt">
          <el-switch v-model="egressConfig.inject_system_prompt" />
          <div class="help-text">
            开启后用下方网关 System Prompt 替换客户端 system（直连模式）。关闭则为透明/跳板透传。
          </div>
        </el-form-item>

        <el-form-item label="重定向策略">
          <el-select v-model="egressConfig.redirect_policy" style="width: 100%">
            <el-option label="不跟随（never，默认）" value="never" />
            <el-option label="始终跟随（always）" value="always" />
            <el-option label="仅 GET/HEAD 跟随（smart）" value="smart" />
          </el-select>
        </el-form-item>

        <el-form-item v-if="egressConfig.redirect_policy !== 'never'" label="最大重定向次数">
          <el-input-number v-model="egressConfig.max_redirects" :min="1" :max="20" style="width: 100%" />
        </el-form-item>

        <el-form-item label="默认 URL Scheme">
          <el-select v-model="egressConfig.default_scheme" style="width: 100%">
            <el-option label="https" value="https" />
            <el-option label="http" value="http" />
          </el-select>
        </el-form-item>
      </template>

      <!-- Token 计量节点配置 -->
      <template v-if="localNode.type === 'token_usage'">
        <el-divider>Token 计量配置</el-divider>

        <el-alert type="info" :closable="false" style="margin-bottom: 12px">
          <template #default>
            <div style="font-size: 13px; line-height: 1.6">
              内置插件 <code>builtin.token_usage</code>，必须依赖上游 <strong>生成器</strong> 或 <strong>透明转发</strong> 节点
              （在「依赖节点」中选择），确保在 LLM 返回后再计量；勿将唯一生成器改成计量节点。
            </div>
          </template>
        </el-alert>

        <el-form-item label="操作类型">
          <el-select v-model="localNode.config.customConfig.operation" style="width: 100%">
            <el-option label="记录用量 (Record)" value="record" />
            <el-option label="查询统计 (Query)" value="query" />
            <el-option label="聚合统计 (Aggregate)" value="aggregate" />
          </el-select>
        </el-form-item>

        <el-form-item label="存储类型">
          <el-select v-model="localNode.config.customConfig.storage_type" style="width: 100%">
            <el-option label="应用数据库 (推荐)" value="sqlite" />
            <el-option label="PostgreSQL" value="postgresql" />
            <el-option label="内存 (仅调试)" value="memory" />
          </el-select>
          <div class="help-text">桌面版使用 SQLite；团队部署可选 PostgreSQL。实际持久化走系统 token_usage 表。</div>
        </el-form-item>

        <el-form-item v-if="localNode.config.customConfig.operation === 'record'" label="记录字段">
          <el-checkbox-group v-model="localNode.config.customConfig.record_fields">
            <el-checkbox label="prompt_tokens">输入 Token</el-checkbox>
            <el-checkbox label="completion_tokens">输出 Token</el-checkbox>
            <el-checkbox label="total_tokens">总 Token</el-checkbox>
            <el-checkbox label="model">模型</el-checkbox>
            <el-checkbox label="backend_id">后端</el-checkbox>
          </el-checkbox-group>
        </el-form-item>
      </template>

      <!-- 缓存节点配置 -->
      <template v-if="localNode.type === 'cache'">
        <el-divider>缓存配置</el-divider>

        <el-form-item label="操作类型" prop="config.customConfig.operation">
          <el-select v-model="localNode.config.customConfig.operation" style="width: 100%">
            <el-option label="读取缓存 (Read)" value="read" />
            <el-option label="写入缓存 (Write)" value="write" />
            <el-option label="删除缓存 (Delete)" value="delete" />
          </el-select>
          <div class="help-text">选择此节点执行的缓存操作</div>
        </el-form-item>

        <el-form-item label="缓存策略" prop="config.customConfig.strategy">
          <el-select v-model="localNode.config.customConfig.strategy" style="width: 100%">
            <el-option label="精确匹配 (Exact)" value="exact" />
            <el-option label="语义匹配 (Semantic)" value="semantic" />
            <el-option label="混合策略 (Hybrid)" value="hybrid" />
          </el-select>
          <div class="help-text">
            精确匹配：基于内容哈希；语义匹配：基于向量相似度；混合：先精确后语义
          </div>
        </el-form-item>

        <el-form-item label="读取存储后端" v-if="localNode.config.customConfig.operation === 'read' || localNode.config.customConfig.operation === 'delete'">
          <el-select 
            v-model="localNode.config.customConfig.read_storage_name" 
            style="width: 100%" 
            clearable
            placeholder="选择读取存储（留空使用默认）"
          >
            <el-option 
              v-for="s in availableStorages" 
              :key="s.name" 
              :label="`${s.name} (${s.type})`" 
              :value="s.name"
            />
          </el-select>
          <div class="help-text">
            从此存储读取缓存。留空则使用默认缓存管理器。
          </div>
        </el-form-item>

        <el-form-item label="写入存储后端" v-if="localNode.config.customConfig.operation === 'write' || localNode.config.customConfig.operation === 'delete'">
          <el-select 
            v-model="localNode.config.customConfig.write_storage_name" 
            style="width: 100%" 
            clearable
            placeholder="选择写入存储（留空使用默认）"
          >
            <el-option 
              v-for="s in availableStorages" 
              :key="s.name" 
              :label="`${s.name} (${s.type})`" 
              :value="s.name"
            />
          </el-select>
          <div class="help-text">
            写入缓存到此存储。留空则使用默认缓存管理器。<br>
            <strong>语义/混合策略</strong>：此存储也用于向量化缓存的向量存储，请确保所选存储支持向量检索（如 pgvector、Elasticsearch、Chroma）。
          </div>
        </el-form-item>

        <el-form-item label="缓存键模板">
          <el-input 
            v-model="localNode.config.customConfig.key_template" 
            :placeholder="'{{model}}:{{hash}}'"
          />
          <div class="help-text">
            缓存键生成规则，支持变量：&#123;&#123;model&#125;&#125;（模型名）、&#123;&#123;hash&#125;&#125;（内容哈希）
          </div>
        </el-form-item>

        <el-form-item v-if="localNode.config.customConfig.operation === 'write'" label="缓存 TTL（秒）">
          <el-input-number 
            v-model="localNode.config.customConfig.ttl" 
            :min="60" 
            :max="86400" 
            style="width: 100%" 
          />
          <div class="help-text">缓存过期时间，单位秒。默认 3600（1小时）</div>
        </el-form-item>

        <el-alert type="info" :closable="false" style="margin-top: 12px">
          <template #default>
            <div style="font-size: 13px; line-height: 1.6">
              <strong>💡 存储配置提示：</strong><br>
              • 需要先在 <strong>后端策略 > 存储管理</strong> 中添加存储后端（如 PostgreSQL、Redis 等）<br>
              • 读取操作只需要配置读取存储，写入操作只需要配置写入存储<br>
              • 语义/混合策略需要存储后端支持向量检索（如 pgvector、Milvus）
            </div>
          </template>
        </el-alert>

        <!-- 语义缓存配置（语义/混合策略时显示） -->
        <template v-if="isSemanticStrategy">
          <el-divider>语义缓存配置</el-divider>

          <el-alert type="warning" :closable="false" style="margin-bottom: 16px">
            <template #default>
              语义缓存需要存储后端支持向量检索（如 PostgreSQL + pgvector、Elasticsearch、Chroma）。
              请确保所选存储后端已正确配置并支持向量操作。
            </template>
          </el-alert>

          <el-form-item label="向量化模型服务 (Embedding)">
            <BackendSelector
              v-model="localNode.config.customConfig.embedding_backend_id"
              placeholder="选择 Embedding 后端服务"
              :filter="embeddingBackendFilter"
              style="width: 100%"
              @change="onEmbeddingBackendChange"
            />
            <div class="help-text">
              用于将文本向量化的模型服务。仅显示已启用且支持向量化的后端。
            </div>
          </el-form-item>

          <el-form-item label="向量化模型名称" v-if="localNode.config.customConfig.embedding_backend_id">
            <el-select
              v-model="localNode.config.customConfig.embedding_model"
              style="width: 100%"
              clearable
              placeholder="选择或输入模型名称"
              :allow-create="true"
              :default-first-option="true"
            >
              <el-option
                v-for="m in embeddingModels"
                :key="m"
                :label="m"
                :value="m"
              />
            </el-select>
            <div class="help-text">
              指定用于向量化的模型名称。留空则使用后端默认模型。
            </div>
          </el-form-item>

          <el-form-item label="语义阈值 (0-1)">
            <el-input-number
              v-model="localNode.config.customConfig.semantic_threshold"
              :min="0"
              :max="1"
              :step="0.05"
              :precision="2"
              style="width: 100%"
            />
            <div class="help-text">
              向量相似度阈值，范围 0-1。高于此值才认为匹配。推荐 0.85。
            </div>
          </el-form-item>

          <el-form-item label="语义 Top K">
            <el-input-number
              v-model="localNode.config.customConfig.semantic_top_k"
              :min="1"
              :max="100"
              style="width: 100%"
            />
            <div class="help-text">
              语义搜索返回的候选结果数量。推荐 5。
            </div>
          </el-form-item>
        </template>
      </template>

      <!-- 路由节点专用配置 —— 仅 router 类型显示 -->
      <template v-if="localNode.type === 'router'">
        <el-divider>路由配置 (Router Config)</el-divider>

        <el-form-item label="路由策略">
          <el-select v-model="routerConfig.strategy" style="width: 100%">
            <el-option label="关键词包含匹配 (keyword_contains)" value="keyword_contains" />
            <el-option label="关键词前缀匹配 (keyword_prefix)" value="keyword_prefix" />
            <el-option label="有序规则 (ordered)" value="ordered" />
            <el-option label="正则匹配 (regex_only)" value="regex_only" />
            <el-option label="关键字+轻量意图 (keyword_then_intent)" value="keyword_then_intent" />
            <el-option label="LLM 意图分类 (llm_classify)" value="llm_classify" />
          </el-select>
          <div class="help-text">
            <span v-if="routerConfig.strategy === 'llm_classify'">
              LLM 语义分类：会发起一次额外的 LLM 调用（约 500ms-2s 延迟），准确率高，能处理多样表达。
            </span>
            <span v-else-if="routerConfig.strategy === 'keyword_then_intent'">
              先关键字/规则命中，未命中再轻量意图分类；小模型分类默认关闭。
            </span>
            <span v-else>
              关键词/规则匹配：零成本、低延迟。llm_classify 适合复杂意图场景。
            </span>
          </div>
        </el-form-item>

        <el-form-item label="默认路由节点">
          <el-input v-model="routerConfig.defaultRoute" placeholder="例如: chat-generator" />
          <div class="help-text">当输入未命中任何规则时，默认执行的分支节点 ID</div>
        </el-form-item>

        <el-form-item label="路由规则映射">
          <div class="help-text" style="margin-bottom: 8px">
            <span v-if="routerConfig.strategy === 'llm_classify'">
              类别名 → 目标节点 ID。LLM 返回类别名（如 code）时，路由到对应节点。
            </span>
            <span v-else>
              关键词 → 目标节点 ID。输入包含左侧关键词时，路由到右侧节点。
            </span>
          </div>
          <div
            v-for="(rule, idx) in routerConfig.routes"
            :key="idx"
            class="custom-config-row"
          >
            <el-input
              v-model="rule.keyword"
              :placeholder="routerConfig.strategy === 'llm_classify' ? '类别名，如 code' : '关键词，如 python'"
              size="small"
              style="width: 180px; flex-shrink: 0"
            />
            <span class="var-eq">→</span>
            <el-input
              v-model="rule.target"
              placeholder="目标节点 ID，如 code-generator"
              size="small"
              style="flex: 1"
            />
            <el-button
              type="danger"
              text
              size="small"
              @click="removeRouterRoute(idx)"
            >
              <el-icon><Delete /></el-icon>
            </el-button>
          </div>
          <el-button size="small" @click="addRouterRoute" style="margin-top: 6px">
            <el-icon><Plus /></el-icon>
            添加路由规则
          </el-button>
        </el-form-item>

        <!-- llm_classify 专属配置 -->
        <template v-if="routerConfig.strategy === 'llm_classify'">
          <el-divider>LLM 分类配置</el-divider>

          <el-form-item label="后端服务" prop="backend">
            <BackendSelector
              v-model="localNode.backend"
              placeholder="选择用于意图分类的后端"
              style="width: 100%"
              @change="onBackendChange"
            />
            <div class="help-text">执行 LLM 意图分类时调用的后端服务（推荐使用轻量、低延迟模型）</div>
          </el-form-item>

          <el-form-item label="模型名称" prop="model">
            <ModelSelector
              v-model="localNode.model"
              :backend-id="localNode.backend"
              placeholder="选择或输入模型"
              :allow-create="true"
              :default-first-option="true"
            />
            <div class="help-text">执行意图分类的模型，如 glm-4-flash、qwen-turbo 等轻量模型</div>
          </el-form-item>

          <el-form-item label="分类 Prompt">
            <el-input
              v-model="routerConfig.classifyPrompt"
              type="textarea"
              :rows="6"
              :placeholder="'留空使用默认 Prompt。可用变量：{{.input}}'"
            />
            <div class="help-text">
              自定义 LLM 分类 Prompt（可选）。留空则使用内置默认分类 Prompt。
              Prompt 中可通过 <code v-text="'{{.input}}'"></code> 引用用户原始输入。
            </div>
          </el-form-item>

          <el-alert type="warning" :closable="false" style="margin-bottom: 12px">
            <template #default>
              <div>使用 llm_classify 需确保节点已配置 <strong>后端服务</strong> 和 <strong>模型</strong>，且 routes 非空。</div>
              <div style="margin-top: 4px">每次请求会增加一次 LLM 调用（推荐使用轻量模型如 glm-4-flash）。</div>
            </template>
          </el-alert>
        </template>
      </template>

      <!-- 插件自定义配置 (Custom Config) —— 通用，所有带插件的节点类型均显示 -->
      <template v-if="needsPluginSelection(localNode.type)">
        <el-divider>插件自定义配置 (Custom Config)</el-divider>

        <!-- 针对 business.mem0 插件的快捷预设字段 -->
        <template v-if="localNode.implementation === 'business.mem0'">
          <el-form-item label="Mem0 API Key">
            <el-input
              v-model="mem0PresetConfig.api_key"
              type="password"
              show-password
              placeholder="输入 Mem0 Admin API Key"
            />
            <div class="help-text">Mem0 服务的认证密钥，仅在当前节点生效</div>
          </el-form-item>

          <el-form-item label="Mem0 Base URL">
            <el-input
              v-model="mem0PresetConfig.base_url"
              placeholder="http://localhost:20061"
            />
            <div class="help-text">Mem0 服务地址，留空使用默认值</div>
          </el-form-item>

          <el-form-item label="Namespace">
            <el-input
              v-model="mem0PresetConfig.namespace"
              placeholder="default"
            />
            <div class="help-text">Mem0 命名空间，用于隔离不同业务的数据</div>
          </el-form-item>

          <el-form-item label="搜索模式">
            <el-select v-model="mem0PresetConfig.search_mode" style="width: 100%">
              <el-option label="语义搜索 (semantic)" value="semantic" />
              <el-option label="关键词搜索 (keyword)" value="keyword" />
              <el-option label="混合搜索 (hybrid)" value="hybrid" />
            </el-select>
            <div class="help-text">Mem0 记忆检索模式</div>
          </el-form-item>

          <el-divider style="margin: 12px 0">向量化配置（可选）</el-divider>
          
          <el-alert type="info" :closable="false" style="margin-bottom: 16px">
            <template #default>
              使用 Centag 内部 embedding 服务生成向量并传给 Mem0 服务端，避免依赖 Mem0 内部的 Google/OpenAI API key。
              请确保所选存储后端已正确配置并支持向量操作。
            </template>
          </el-alert>

          <el-form-item label="向量化后端">
            <BackendSelector
              v-model="mem0PresetConfig.embedding_backend_id"
              placeholder="选择 Embedding 后端服务"
              :filter="embeddingBackendFilter"
              style="width: 100%"
              @change="onMem0EmbeddingBackendChange"
            />
            <div class="help-text">
              用于将文本向量化的模型服务。仅显示已启用且支持向量化的后端。
            </div>
          </el-form-item>

          <el-form-item label="向量化模型" v-if="mem0PresetConfig.embedding_backend_id">
            <el-select
              v-model="mem0PresetConfig.embedding_model"
              style="width: 100%"
              clearable
              placeholder="选择或输入模型名称"
              :allow-create="true"
              :default-first-option="true"
            >
              <el-option
                v-for="m in embeddingModels"
                :key="m"
                :label="m"
                :value="m"
              />
            </el-select>
            <div class="help-text">
              指定用于向量化的模型名称。留空则使用后端默认模型。
            </div>
          </el-form-item>
        </template>

        <!-- 通用动态 key-value 编辑器（所有插件节点均显示） -->
        <el-form-item label="自定义参数">
          <div class="custom-config-editor">
            <div class="custom-config-hint">
              配置插件所需的额外参数，如 API Key、URL、超时等。点击右侧按钮可添加预设字段。
            </div>

            <div
              v-for="(item, idx) in customConfigItems"
              :key="idx"
              class="custom-config-row"
            >
              <el-input
                v-model="item.key"
                placeholder="参数名，如 api_key"
                size="small"
                style="width: 160px; flex-shrink: 0"
              />
              <span class="var-eq">=</span>
              <el-input
                v-model="item.value"
                placeholder="参数值"
                size="small"
                style="flex: 1"
                :type="isSecretKey(item.key) ? 'password' : 'text'"
                :show-password="isSecretKey(item.key)"
              />
              <el-button
                type="danger"
                text
                size="small"
                @click="removeCustomConfigItem(idx)"
              >
                <el-icon><Delete /></el-icon>
              </el-button>
            </div>

            <div class="custom-config-actions">
              <el-button size="small" @click="addCustomConfigItem">
                <el-icon><Plus /></el-icon>
                添加参数
              </el-button>
              <el-button
                v-if="localNode.implementation === 'business.mem0'"
                size="small"
                @click="fillMem0Defaults"
              >
                <el-icon><Plus /></el-icon>
                填充 Mem0 默认值
              </el-button>
            </div>
          </div>
        </el-form-item>
      </template>

      <!-- 超时与重试 -->
      <el-form-item label="超时（秒）">
        <el-input-number v-model="localNode.timeout" :min="5" :max="300" style="width: 100%" />
      </el-form-item>

      <el-form-item label="重试配置">
        <div class="retry-grid">
          <el-input-number 
            v-model="localNode.retry.max_attempts" 
            :min="0" 
            :max="10" 
            placeholder="重试次数"
          />
          <el-select v-model="localNode.retry.backoff_strategy">
            <el-option label="指数退避" value="exponential" />
            <el-option label="线性退避" value="linear" />
            <el-option label="固定延迟" value="fixed" />
          </el-select>
        </div>
      </el-form-item>

      <el-form-item label="降级策略">
        <el-select
          v-model="localNode.fallback_policy_id"
          clearable
          placeholder="继承流水线默认"
          style="width: 100%"
        >
          <el-option label="继承流水线默认" value="" />
          <el-option
            v-for="policy in fallbackPolicies"
            :key="policy.id"
            :label="`${policy.name} (${policy.id})`"
            :value="policy.id"
          />
        </el-select>
        <div class="form-tip">为空时使用流水线全局默认策略；可单独指定此节点的降级策略</div>
      </el-form-item>

      <!-- 执行条件 (tool_call_injector 使用专属条件字段) -->
      <el-form-item v-if="localNode.type !== 'tool_call_injector'" label="执行条件 (Condition)">
        <el-input 
          v-model="localNode.config.condition" 
          :placeholder="conditionPlaceholder"
          type="textarea"
          :rows="2"
        />
        <div class="help-text">支持节点引用表达式，如 &#123;&#123;.node_id.metadata.field&#125;&#125; == false</div>
      </el-form-item>

      <!-- 工具调用注入配置 -->
      <template v-if="localNode.type === 'tool_call_injector'">
        <el-divider>工具调用注入配置</el-divider>

        <el-form-item label="注入条件 (Condition)">
          <el-input
            v-model="injectorCondition"
            placeholder='例如: {{node.reviewer.score}} < 0.8 （留空则无条件注入）'
            type="textarea"
            :rows="2"
          />
          <div class="help-text">支持模板变量，满足条件时才注入工具调用。留空表示始终注入。</div>
        </el-form-item>

        <el-form-item label="工具调用列表">
          <div class="injector-tc-list">
            <div
              v-for="(tc, idx) in injectorToolCalls"
              :key="idx"
              class="injector-tc-item"
            >
              <div class="injector-tc-header">
                <span>工具调用 #{{ idx + 1 }}</span>
                <el-button type="danger" text size="small" @click="removeInjectorToolCall(idx)">
                  <el-icon><Delete /></el-icon>
                </el-button>
              </div>
              <div class="injector-tc-body">
                <el-input
                  v-model="tc.id"
                  placeholder="调用 ID，如 read_project"
                  size="small"
                  style="margin-bottom: 6px"
                />
                <el-input
                  v-model="tc.functionName"
                  placeholder="函数名，如 read_file"
                  size="small"
                  style="margin-bottom: 6px"
                />
                <el-input
                  v-model="tc.arguments"
                  placeholder='函数参数 (JSON)，如 {"path": "/workspace/README.md"}'
                  type="textarea"
                  :rows="3"
                  size="small"
                />
              </div>
            </div>
            <el-button size="small" @click="addInjectorToolCall" style="margin-top: 8px">
              <el-icon><Plus /></el-icon>
              添加工具调用
            </el-button>
          </div>
        </el-form-item>
      </template>

      <!-- 依赖与下游 -->
      <el-row :gutter="12">
        <el-col :span="12">
          <el-form-item label="依赖节点 (Depends On)">
            <el-select 
              v-model="localNode.depends_on" 
              multiple 
              style="width: 100%"
            >
              <el-option 
                v-for="n in otherNodes" 
                :key="n" 
                :label="n" 
                :value="n"
              />
            </el-select>
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="下游节点 (Next)">
            <el-select 
              v-model="localNode.next_nodes" 
              multiple 
              style="width: 100%"
            >
              <el-option 
                v-for="n in otherNodes" 
                :key="n" 
                :label="n" 
                :value="n"
              />
            </el-select>
          </el-form-item>
        </el-col>
      </el-row>

      <!-- 路由配置 (仅对 generator 节点显示) -->
      <el-divider v-if="localNode.type === 'generator'">路由配置 (RouteConfig)</el-divider>
      <template v-if="localNode.type === 'generator'">
        <el-alert type="info" :closable="false" style="margin-bottom: 16px">
          <template #default>
            <div style="font-size: 13px; line-height: 1.5">
              <strong>路由配置说明：</strong><br>
              当上游有 Router 节点时，配置此项可使本节点仅在特定路由值匹配时执行。<br>
              例如：Router 节点根据关键词将请求路由到不同分支，各分支节点配置对应的 route_value。
            </div>
          </template>
        </el-alert>
        
        <el-form-item label="启用路由配置">
          <el-switch v-model="hasRouteConfig" />
        </el-form-item>

        <template v-if="hasRouteConfig">
          <el-form-item label="上游路由节点 ID">
            <el-select 
              v-model="routeConfig.router_node_id" 
              style="width: 100%"
              placeholder="选择上游的 Router 节点"
            >
              <el-option 
                v-for="n in routerNodes" 
                :key="n" 
                :label="n" 
                :value="n"
              />
            </el-select>
            <div class="help-text">选择提供路由决策的上游 Router 节点</div>
          </el-form-item>

          <el-form-item label="路由匹配值">
            <el-input 
              v-model="routeConfig.route_value" 
              placeholder="例如: code, translate, chat"
            />
            <div class="help-text">当 Router 节点选择此值时，本节点才会执行</div>
          </el-form-item>

          <el-form-item label="是否为默认分支">
            <el-switch v-model="routeConfig.is_default" />
            <div class="help-text">当没有分支匹配时，默认分支会被执行</div>
          </el-form-item>
        </template>
      </template>
    </el-form>

    <template #footer>
      <div class="footer-buttons">
        <el-button @click="handleClose">取消</el-button>
        <el-button type="primary" @click="saveNode">保存节点配置</el-button>
      </div>
    </template>
  </el-drawer>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted } from 'vue'
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
      ElMessage.warning('未获取到已注册的节点插件，请确认后端已启动并完成插件注册')
    }
  } catch (err) {
    console.error('Failed to load plugins:', err)
    loadedPlugins.value = []
    ElMessage.error('加载节点插件失败，请检查后端服务')
  } finally {
    loadingPlugins.value = false
  }
}

// 打开插件管理器
const openPluginManager = () => {
  // TODO: 跳转到插件管理页面或打开插件管理对话框
  ElMessage.info('插件管理器功能待实现')
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
  return "例如: {{.cache_read.metadata.cache_hit}} == false"
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
    { name: 'question', desc: '用户发送的原始问题原文，引擎全程自动传递，任何节点都能用' },
  ]
  if (type === 'reviewer') {
    const vars = [
      ...common,
      { name: 'answer',    desc: '上游执行节点（generator）传来的内容，即待审核的回答' },
      { name: 'timestamp', desc: '当前执行时间，格式 RFC3339，如 2026-04-24T15:30:00+08:00' },
      { name: 'criteria',  desc: '审核维度列表（数组），来自节点配置的 custom_config.criteria，用 {{range .criteria}}-{{.}}{{end}} 遍历' },
    ]
    // 自动为每个上游节点注入 {nodeID}_content（无需 template_vars 绑定）
    for (const nodeId of upstreamNodeIds.value) {
      vars.push({
        name: `${nodeId}_content`,
        desc: `节点"${nodeId}"的输出文本内容（自动注入，无需添加变量绑定）`,
      })
    }
    return vars
  }
  if (type === 'processor') {
    const vars = [
      ...common,
      { name: 'input',       desc: '上游节点传来的内容（待处理文本），通常是 generator 的回答' },
      { name: 'timestamp',   desc: '当前执行时间，格式 RFC3339，如 2026-04-24T15:30:00+08:00' },
      { name: 'target_lang', desc: '目标语言，仅在 operation=translate 时有效，取自节点 custom_config.target_lang' },
      { name: 'metadata',    desc: '上游节点的完整元数据对象，可用 {{.metadata.key}} 访问其中任意字段' },
    ]
    // 自动为每个上游节点注入 {nodeID}_content（无需 template_vars 绑定）
    for (const nodeId of upstreamNodeIds.value) {
      vars.push({
        name: `${nodeId}_content`,
        desc: `节点"${nodeId}"的输出文本内容（自动注入，无需添加变量绑定）`,
      })
    }
    return vars
  }
  if (type === 'generator') {
    return [
      { name: 'question', desc: '用户发送的原始问题原文（在 System Prompt 中可引用，用于告知模型上下文）' },
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
        desc: `节点"${nodeId}"的评分（0~1，审核节点才有）。需在下方添加绑定：变量名=${nodeId}_score，来源路径=node.${nodeId}.score`,
      },
      {
        name: `${nodeId}_passed`,
        desc: `节点"${nodeId}"的审核是否通过（true/false）。需在下方添加绑定：变量名=${nodeId}_passed，来源路径=node.${nodeId}.passed`,
      },
      {
        name: `${nodeId}_feedback`,
        desc: `节点"${nodeId}"的审核反馈文本。需在下方添加绑定：变量名=${nodeId}_feedback，来源路径=node.${nodeId}.feedback`,
      },
    )
  }
  return vars
})

// 执行上下文变量（除 timestamp 对 processor/reviewer 已自动注入外，其余均需在下方添加 template_vars 绑定后使用）
const contextVars = [
  { name: 'timestamp',  desc: '当前执行时间，格式 RFC3339（processor/reviewer 节点已自动注入，可直接使用；其他节点需绑定路径：context.timestamp）' },
  { name: 'user_id',    desc: '当前登录用户的 ID。需绑定路径：context.user_id' },
  { name: 'session_id', desc: '当前对话的会话 ID。需绑定路径：context.session_id' },
  { name: 'pipeline_id',desc: '当前流水线的 ID。需绑定路径：context.pipeline_id' },
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
  id:   [{ required: true, message: '请输入节点ID', trigger: 'blur' }],
  name: [{ required: true, message: '请输入节点名称', trigger: 'blur' }],
  type: [{ required: true, message: '请选择节点类型', trigger: 'change' }],
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
    generator:  'System Prompt（可选）',
    transparent_forward: 'System Prompt（注入时生效）',
    aggregator: '聚合 Prompt（可选）',
  }
  return labels[type] || 'Prompt 模板'
}

const getPromptPlaceholder = (type: string) => {
  const placeholders: Record<string, string> = {
    generator:  '可选。设置模型的角色或行为规范，如：你是一名专业的技术助手…',
    transparent_forward: '开启注入后使用。网关人格会替换客户端 system 消息…',
    aggregator: '可选。输入指导 LLM 如何聚合多个上游输出的指令，如：请综合以下回答，生成一个更全面、结构更清晰的答案…',
  }
  return placeholders[type] || '在此编写 Prompt，点击下方变量名可快速插入'
}

const getTypeDescription = (type: string) => {
  const desc: Record<string, string> = {
    transparent_forward: '出站转发：直连/透明/跳板共用节点，用「出站策略」开关区分行为',
    generator:  '生成初始内容（重组请求调用 LLM）；日常出站请优先用「出站转发」',
    processor:  '对内容进行优化、翻译、摘要等后处理操作',
    reviewer:   '对生成结果进行质量审核、打分，可配置评分标准',
    router:     '根据条件路由到不同分支节点',
    aggregator: '合并多个上游节点的结果',
    parallel:   '并行执行多个节点',
    cache:       '读取或写入缓存，支持精确/语义匹配，可配置不同的读写存储后端',
    token_usage: '内置 Token 计量插件，记录上游 LLM 请求的 token 用量到数据库',
    tool_call_injector: '在 Pipeline 中注入工具调用指令，支持条件触发和模板变量解析',
  }
  return desc[type] || '自定义处理节点'
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
    ElMessage.info('已自动将 Token 计量节点依赖到上游 LLM 节点，确保在生成之后执行')
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
      ElMessage.warning('请勿将唯一的生成器改成 Token 计量。请保留生成器节点，并单独新增计量节点。')
    }
    if (!localNode.value.name || localNode.value.name === '新节点') {
      localNode.value.name = 'Token 计量'
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
    if (!localNode.value.name || localNode.value.name === '新节点') {
      localNode.value.name = '出站转发'
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
    ElMessage.success('节点配置已更新，请点击"保存"按钮持久化到服务器')
    
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

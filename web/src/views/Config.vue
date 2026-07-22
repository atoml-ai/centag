<template>
  <div class="config">
    <div class="header config-header">
      <div class="header-left">
        <h1 class="page-title">系统配置</h1>
        <p class="page-description">配置系统参数和功能开关</p>
      </div>
      <div v-if="isConfigTab" class="header-actions">
        <el-button :loading="loading" @click="load">
          <el-icon><Refresh /></el-icon>
          刷新
        </el-button>
        <el-button type="primary" :loading="saving" @click="save">
          <el-icon><Check /></el-icon>
          保存配置
        </el-button>
      </div>
    </div>

    <div class="config-tabs-wrapper" v-loading="isConfigTab && loading">
      <el-tabs v-model="activeTab" type="border-card" class="config-tabs">
        <!-- 服务配置 -->
        <el-tab-pane label="服务配置" name="server">
          <el-form label-width="120px">
            <el-divider content-position="left">基本设置</el-divider>
            <el-form-item label="服务主机">
              <el-input v-model="config.server.host" placeholder="0.0.0.0" />
              <div class="form-tip">服务监听的主机地址</div>
            </el-form-item>
            <el-form-item label="服务端口">
              <el-input-number v-model="config.server.port" :min="1" :max="65535" style="width: 200px" />
              <div class="form-tip">服务监听的端口号</div>
            </el-form-item>
            <el-divider content-position="left">代理设置</el-divider>
            <el-form-item label="启用代理">
              <el-switch v-model="config.proxy.enabled" />
              <div class="form-tip">是否启用 LLM 代理功能</div>
            </el-form-item>
            <el-form-item label="默认模式">
              <el-select v-model="config.proxy.default_mode" style="width: 220px">
                <el-option label="指定默认后端" value="direct-backend" />
                <el-option label="智能调度" value="smart-scheduling" />
                <el-option label="透明模式（不注入 system prompt）" value="transparent-proxy" />
              </el-select>
              <div class="form-tip">与 API / X-Proxy-Mode 一致：direct-backend、smart-scheduling、transparent-proxy；HTTP 透传请用 raw-forward</div>
            </el-form-item>
            <el-form-item label="代理超时">
              <el-input-number v-model="config.proxy.timeout" :min="1" :max="300" style="width: 200px" />
              <span class="unit">秒</span>
              <div class="form-tip">代理请求的超时时间</div>
            </el-form-item>

            <el-divider content-position="left">降级与重试</el-divider>
            <el-form-item label="可重试状态码">
              <el-select
                v-model="config.proxy.retryable_status_codes"
                multiple
                filterable
                allow-create
                default-first-option
                style="width: 400px"
                placeholder="如 429, 500, 502, 503, 504"
              >
                <el-option v-for="code in [400,401,403,404,408,429,500,502,503,504]" :key="code" :label="code" :value="code" />
              </el-select>
              <div class="form-tip">上游返回这些状态码时触发重试/降级（热生效）</div>
            </el-form-item>
            <el-form-item label="超时触发降级">
              <el-switch v-model="config.proxy.timeout_retryable" />
              <div class="form-tip">上游超时时是否触发降级（热生效）</div>
            </el-form-item>
            <el-form-item label="网络错误降级">
              <el-switch v-model="config.proxy.network_retryable" />
              <div class="form-tip">网络连接失败时是否触发降级（热生效）</div>
            </el-form-item>

            <el-divider content-position="left">熔断器</el-divider>
            <el-form-item label="失败阈值">
              <el-input-number v-model="config.proxy.circuit_breaker.failure_threshold" :min="1" :max="20" style="width: 150px" />
              <div class="form-tip">窗口内失败次数触发熔断（默认 3，热生效）</div>
            </el-form-item>
            <el-form-item label="恢复成功数">
              <el-input-number v-model="config.proxy.circuit_breaker.success_threshold" :min="1" :max="10" style="width: 150px" />
              <div class="form-tip">半开状态恢复所需成功次数（默认 2，热生效）</div>
            </el-form-item>
            <el-form-item label="熔断持续时间">
              <el-input-number v-model="config.proxy.circuit_breaker.timeout_sec" :min="10" :max="300" style="width: 150px" />
              <span class="unit">秒</span>
              <div class="form-tip">熔断持续时间（默认 60s，热生效）</div>
            </el-form-item>
            <el-form-item label="滑动窗口">
              <el-input-number v-model="config.proxy.circuit_breaker.window_sec" :min="10" :max="300" style="width: 150px" />
              <span class="unit">秒</span>
              <div class="form-tip">失败计数窗口大小（默认 60s，热生效）</div>
            </el-form-item>
            <el-form-item label="429 加重系数">
              <el-input-number v-model="config.proxy.circuit_breaker.rate_limit_weight" :min="1" :max="10" style="width: 150px" />
              <div class="form-tip">1 次 429 计为 N 次失败（默认 2，热生效）</div>
            </el-form-item>

            <el-divider content-position="left">嵌入模型</el-divider>
            <el-form-item label="启用嵌入">
              <el-switch v-model="config.embedding.enabled" />
              <div class="form-tip">是否启用文本嵌入功能（用于语义缓存）</div>
            </el-form-item>
            <el-form-item label="选择后端服务">
              <el-select
                v-model="config.embedding.backend_id"
                style="width: 300px"
                placeholder="请选择后端服务"
                :disabled="!config.embedding.enabled"
              >
                <el-option
                  v-for="backend in backends.filter(b => b.enabled)"
                  :key="backend.id"
                  :label="`${backend.name} (${backend.type})`"
                  :value="backend.id"
                />
              </el-select>
              <div class="form-tip">选择提供向量化服务的后端</div>
            </el-form-item>
            <el-form-item label="向量化模型">
              <el-select
                v-model="config.embedding.model"
                style="width: 300px"
                placeholder="请先选择后端服务"
                :disabled="!config.embedding.backend_id || !config.embedding.enabled"
                :loading="loadingEmbeddingModels"
                filterable
                allow-create
              >
                <el-option
                  v-for="model in embeddingModels"
                  :key="model"
                  :label="model"
                  :value="model"
                />
              </el-select>
              <div class="form-tip">选择用于文本向量化的模型（支持手动输入）</div>
            </el-form-item>
          </el-form>
        </el-tab-pane>

        <!-- 输入配置 -->
        <el-tab-pane label="输入配置" name="question_split">
          <el-form label-width="170px">
            <!-- 缓存命中开关 -->
            <el-card shadow="never" class="config-card">
              <template #header>
                <div class="card-header">
                  <span>缓存命中</span>
                </div>
              </template>
              <el-form-item label="启用缓存命中">
                <el-switch v-model="config.cache.enable_cache_read" :disabled="!config.cache.enabled" />
                <div class="form-tip">关闭后完全不走缓存命中流程，直接转发请求到后端</div>
              </el-form-item>
            </el-card>

            <!-- 问题拆分开关（依赖缓存命中开启） -->
            <el-card v-if="config.cache.enable_cache_read && config.cache.enabled" shadow="never" class="config-card">
              <template #header>
                <div class="card-header">
                  <span>问题拆分</span>
                </div>
              </template>
              <el-form-item label="启用问题拆分">
                <el-switch v-model="config.question_split.enabled" />
                <div class="form-tip">对用户提问进行拆分，分别命中缓存，提升缓存利用率（默认关闭）</div>
              </el-form-item>
            </el-card>

            <!-- 问题拆分配置子框 -->
            <template v-if="config.cache.enable_cache_read && config.cache.enabled && config.question_split.enabled">
              <el-card shadow="never" class="config-card">
                <template #header>
                  <div class="card-header">
                    <span>拆分策略</span>
                  </div>
                </template>
                <el-form-item label="快速规则拆分">
                  <el-switch v-model="config.question_split.fast_split_enabled" />
                  <div class="form-tip">使用纯算法（无 LLM）快速判断是否需要拆分，延迟 &lt;1ms</div>
                </el-form-item>
                <el-form-item label="模型辅助拆分">
                  <el-switch v-model="config.question_split.llm_split_enabled" />
                  <div class="form-tip">使用 LLM 进行更精准的语义拆分（需配置后端，可提升命中率）</div>
                </el-form-item>
                <el-form-item label="拆分策略">
                  <el-select v-model="config.question_split.split_strategy" style="width: 200px" placeholder="选择策略">
                    <el-option label="规则拆分（推荐）" value="rule" />
                    <el-option label="模型拆分" value="llm" />
                    <el-option label="混合（规则+模型）" value="hybrid" />
                  </el-select>
                  <div class="form-tip">rule=纯算法快速拆分，llm=模型拆分，hybrid=先规则后模型</div>
                </el-form-item>
                <el-form-item label="复杂度阈值">
                  <el-slider v-model="config.question_split.complexity_threshold"
                    :min="0" :max="1" :step="0.05"
                    :format-tooltip="(val: number) => (val * 100).toFixed(0) + '%'"
                    style="width: 300px"
                  />
                  <span style="margin-left:12px;color:#606266;">{{ (config.question_split.complexity_threshold * 100).toFixed(0) }}%</span>
                  <div class="form-tip">问题复杂度超过此阈值才触发拆分（越低越容易拆分）</div>
                </el-form-item>
                <el-form-item label="最大子问题数">
                  <el-input-number v-model="config.question_split.max_sub_questions"
                    :min="2" :max="10" style="width: 200px" />
                  <div class="form-tip">一次请求最多拆分的子问题数量</div>
                </el-form-item>
                <el-form-item label="兜底超时">
                  <el-input-number v-model="config.question_split.timeout"
                    :min="1" :max="30" style="width: 200px" />
                  <span class="unit">秒</span>
                  <div class="form-tip">拆分流程超时后自动降级为全量 LLM 请求</div>
                </el-form-item>
              </el-card>

              <el-card v-if="config.question_split.llm_split_enabled" shadow="never" class="config-card">
                <template #header>
                  <div class="card-header">
                    <span>模型辅助拆分后端</span>
                  </div>
                </template>
                <el-form-item label="拆分后端服务">
                  <el-select v-model="config.question_split.backend_id" style="width: 300px"
                    placeholder="请选择后端服务">
                    <el-option v-for="backend in backends.filter((b: any) => b.enabled)"
                      :key="backend.id" :label="`${backend.name} (${backend.type})`" :value="backend.id" />
                  </el-select>
                  <div class="form-tip">提供模型拆分服务的后端</div>
                </el-form-item>
                <el-form-item label="拆分模型">
                  <el-input v-model="config.question_split.model" style="width: 300px"
                    placeholder="如 qwen2.5:1.5b" />
                  <div class="form-tip">用于问题拆分的模型（建议使用轻量小模型）</div>
                </el-form-item>
              </el-card>

              <el-card shadow="never" class="config-card">
                <template #header>
                  <div class="card-header">
                    <span>答案整合策略</span>
                  </div>
                </template>
                <el-form-item label="整合策略">
                  <el-select v-model="config.question_split.synthesis_strategy" style="width: 200px" placeholder="选择策略">
                    <el-option label="直接拼接（推荐）" value="concat" />
                    <el-option label="模板整合" value="template" />
                    <el-option label="模型整合" value="llm" />
                  </el-select>
                  <div class="form-tip">concat=直接拼接各子问题答案，template=模板整合，llm=模型整合</div>
                </el-form-item>
                <el-form-item label="合成后端服务" v-if="config.question_split.synthesis_strategy === 'llm'">
                  <el-select v-model="config.question_split.synthesis_backend_id" style="width: 300px"
                    placeholder="请选择后端服务">
                    <el-option v-for="backend in backends.filter((b: any) => b.enabled)"
                      :key="backend.id" :label="`${backend.name} (${backend.type})`" :value="backend.id" />
                  </el-select>
                  <div class="form-tip">提供答案合成服务的后端</div>
                </el-form-item>
                <el-form-item label="合成模型" v-if="config.question_split.synthesis_strategy === 'llm'">
                  <el-input v-model="config.question_split.synthesis_model" style="width: 300px"
                    placeholder="如 qwen2.5:1.5b" />
                  <div class="form-tip">用于答案合成的模型</div>
                </el-form-item>
              </el-card>
            </template>
          </el-form>
        </el-tab-pane>

        <!-- 缓存配置 -->
        <el-tab-pane label="缓存配置" name="cache">
          <el-form label-width="150px">
            <!-- 启用缓存开关 -->
            <el-card shadow="never" class="config-card">
              <template #header>
                <div class="card-header">
                  <span>功能开关</span>
                </div>
              </template>
              <el-form-item label="启用缓存">
                <el-switch v-model="config.cache.enabled" />
                <div class="form-tip">是否启用缓存功能</div>
              </el-form-item>
            </el-card>

            <template v-if="config.cache.enabled">
              <!-- 缓存写入模式 -->
              <el-card shadow="never" class="config-card">
                <template #header>
                  <div class="card-header">
                    <span>缓存写入</span>
                  </div>
                </template>
                <el-form-item label="缓存写入模式">
                  <el-radio-group v-model="cacheWriteMode">
                    <el-radio value="normal">
                      <div>
                        <div style="font-weight: 500;">正常缓存</div>
                        <div style="font-size: 12px; color: #909399;">写入缓存，可命中、可拆分、向量化</div>
                      </div>
                    </el-radio>
                    <el-radio value="save_only">
                      <div>
                        <div style="font-weight: 500;">仅保存</div>
                        <div style="font-size: 12px; color: #909399;">只保存问答数据用于浏览，不参与缓存命中</div>
                      </div>
                    </el-radio>
                    <el-radio value="disabled">
                      <div>
                        <div style="font-weight: 500;">关闭写入</div>
                        <div style="font-size: 12px; color: #909399;">不写入任何缓存数据</div>
                      </div>
                    </el-radio>
                  </el-radio-group>
                </el-form-item>
              </el-card>

              <!-- 基本设置（仅正常缓存模式显示） -->
              <template v-if="cacheWriteMode === 'normal'">
                <el-card shadow="never" class="config-card">
                  <template #header>
                    <div class="card-header">
                      <span>基本设置</span>
                    </div>
                  </template>
                  <el-form-item label="缓存策略">
                    <el-select v-model="config.cache.strategy" style="width: 200px" placeholder="选择策略">
                      <el-option label="仅精确匹配" value="exact">
                        <div>
                          <div style="font-weight: 500;">仅精确匹配</div>
                          <div style="font-size: 12px; color: #909399;">只匹配完全相同的请求</div>
                        </div>
                      </el-option>
                      <el-option label="仅语义匹配" value="semantic">
                        <div>
                          <div style="font-weight: 500;">仅语义匹配</div>
                          <div style="font-size: 12px; color: #909399;">按向量相似度匹配，阈值见下方「语义命中阈值」（非对话温度）</div>
                        </div>
                      </el-option>
                      <el-option label="混合策略" value="hybrid">
                        <div>
                          <div style="font-weight: 500;">混合策略</div>
                          <div style="font-size: 12px; color: #909399;">先尝试精确匹配，失败后尝试语义匹配</div>
                        </div>
                      </el-option>
                    </el-select>
                    <div class="form-tip">选择缓存匹配策略</div>
                  </el-form-item>
                  <el-form-item label="默认过期时间">
                    <el-input-number v-model="config.cache.default_ttl" :min="0" :max="86400" style="width: 200px" />
                    <span class="unit">秒</span>
                    <div class="form-tip">缓存条目的默认过期时间（0表示永不过期）</div>
                  </el-form-item>
                  <el-form-item label="最大缓存数">
                    <el-input-number v-model="config.cache.max_cache_size" :min="0" :max="1000000" style="width: 200px" />
                    <div class="form-tip">最大缓存条目数（0表示无限制）</div>
                  </el-form-item>
                  <el-form-item label="清理间隔">
                    <el-input-number v-model="config.cache.cleanup_interval" :min="0" :max="3600" style="width: 200px" />
                    <span class="unit">秒</span>
                    <div class="form-tip">过期缓存清理间隔（0表示不自动清理）</div>
                  </el-form-item>
                </el-card>

                <!-- 语义缓存设置（仅正常缓存模式显示） -->
                <el-card shadow="never" class="config-card">
                  <template #header>
                    <div class="card-header">
                      <span>语义缓存设置</span>
                    </div>
                  </template>
                  <el-alert
                    type="info"
                    :closable="false"
                    show-icon
                    class="semantic-cache-intro"
                    title="与「对话里的生成温度」无关"
                    description="此处配置的是：用户问题与缓存条目的向量相似度达到多少才算命中语义缓存。对话页中的「生成温度」是发给大模型的采样参数，二者不要混淆。"
                  />
                  <el-form-item label="自动向量化">
                    <el-switch v-model="config.cache.semantic.enable_auto_embedding" />
                    <div class="form-tip">是否自动生成向量（需要配置嵌入模型）</div>
                  </el-form-item>
                  <el-form-item label="语义命中阈值（相似度）">
                    <el-slider
                      v-model="config.cache.semantic.threshold"
                      :min="0"
                      :max="1"
                      :step="0.01"
                      :format-tooltip="(val) => (val * 100).toFixed(0) + '%'"
                      style="width: 300px"
                    />
                    <span style="margin-left: 12px; color: #606266;">{{ (config.cache.semantic.threshold * 100).toFixed(0) }}%</span>
                    <div class="form-tip">
                      向量相似度下限：只有 ≥ 该值的候选才会视为语义缓存命中；越高越严格（常见 0.75～0.85）。
                      <strong>不是</strong>大模型请求参数里的 temperature。
                    </div>
                  </el-form-item>
              <el-form-item label="返回结果数">
                <el-input-number v-model="config.cache.semantic.top_k" :min="1" :max="10" style="width: 200px" />
                <div class="form-tip">语义搜索返回的最大结果数</div>
              </el-form-item>
              <el-form-item label="距离算法">
                <el-select v-model="config.cache.semantic.distance_type" style="width: 200px" placeholder="选择算法">
                  <el-option label="余弦相似度 (推荐)" value="cosine" />
                  <el-option label="欧氏距离" value="euclidean" />
                  <el-option label="点积" value="dot_product" />
                </el-select>
                <div class="form-tip">向量相似度计算方法</div>
                </el-form-item>
                </el-card>

                <!-- 输出拆分设置（仅正常缓存模式显示） -->
                <el-card shadow="never" class="config-card">
                  <template #header>
                    <div class="card-header">
                      <span>输出拆分</span>
                    </div>
                  </template>
                  <el-form-item label="启用拆分">
                    <el-switch v-model="config.qa_split.enabled" />
                    <div class="form-tip">是否启用问答自动拆分功能（将LLM输出拆分为Q&A对）</div>
                  </el-form-item>
                  <template v-if="config.qa_split.enabled">
                    <el-form-item label="选择后端服务">
                      <el-select
                        v-model="config.qa_split.backend_id"
                        style="width: 300px"
                        placeholder="请选择后端服务"
                      >
                        <el-option
                          v-for="backend in backends.filter(b => b.enabled)"
                          :key="backend.id"
                          :label="`${backend.name} (${backend.type})`"
                          :value="backend.id"
                        />
                      </el-select>
                    <div class="form-tip">选择提供拆分服务的后端</div>
                  </el-form-item>
                  <el-form-item label="拆分模型">
                    <el-select
                      v-model="config.qa_split.model"
                      style="width: 300px"
                      placeholder="请先选择后端服务"
                      :disabled="!config.qa_split.backend_id"
                      :loading="loadingQAModels"
                      filterable
                      allow-create
                    >
                      <el-option
                        v-for="model in qaModels"
                        :key="model"
                        :label="model"
                        :value="model"
                      />
                    </el-select>
                    <div class="form-tip">选择用于问答拆分的模型（建议使用小模型，支持手动输入）</div>
                  </el-form-item>
                  <el-form-item label="拆分用生成温度">
                    <el-input-number
                      v-model="config.qa_split.temperature"
                      :min="0"
                      :max="2"
                      :step="0.1"
                      :precision="1"
                      style="width: 200px"
                    />
                    <div class="form-tip">
                      仅作用于问答拆分调用的小模型（0～2，越低越稳定）。与上方「语义命中阈值」及对话页「生成温度」是不同参数。
                    </div>
                  </el-form-item>
                  <el-form-item label="最大 Token 数">
                    <el-input-number v-model="config.qa_split.max_tokens" :min="100" :max="100000" style="width: 200px" />
                    <div class="form-tip">单次生成的最大 Token 数量</div>
                  </el-form-item>
                  <el-form-item label="超时时间">
                    <el-input-number v-model="config.qa_split.timeout" :min="10" :max="300" style="width: 200px" />
                    <span class="unit">秒</span>
                    <div class="form-tip">请求超时时间</div>
                  </el-form-item>
                  <el-form-item label="系统提示词">
                    <el-input
                      v-model="config.qa_split.prompt"
                      type="textarea"
                      :rows="8"
                      placeholder="输入系统提示词..."
                      style="width: 100%; font-family: monospace;"
                    />
                    <div class="form-tip" v-pre>
                      拆分模型的系统提示词。可用变量：{{question}} - 原始问题，{{answer}} - 原始答案
                    </div>
                  </el-form-item>
                </template>
              </el-card>
              </template>
            </template>
          </el-form>
        </el-tab-pane>

        <!-- 代理设置 -->
        <el-tab-pane label="代理设置" name="proxy_settings">
          <el-tabs type="border-card">
            <!-- Host 代理 -->
            <el-tab-pane label="Host代理（高级）">
              <HostProxyView />
            </el-tab-pane>

            <!-- 系统代理 -->
            <el-tab-pane label="本机代理出口（PAC）">
              <SystemProxyView />
            </el-tab-pane>

            <!-- Clash订阅 -->
            <el-tab-pane label="Clash订阅">
              <ClashRulesView />
            </el-tab-pane>
          </el-tabs>
        </el-tab-pane>
      </el-tabs>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh, Check } from '@element-plus/icons-vue'
import { getConfig, saveConfig, getBackends } from '@/api'
import { getBackendModels } from '@/api/backend'
import HostProxyView from './HostProxy.vue'
import SystemProxyView from './SystemProxy.vue'
import ClashRulesView from './ClashRules.vue'

const loading = ref(false)
const saving = ref(false)
const activeTab = ref('server')
const backends = ref<any[]>([])
const embeddingModels = ref<string[]>([])
const qaModels = ref<string[]>([])
const loadingEmbeddingModels = ref(false)
const loadingQAModels = ref(false)
const isInitialLoad = ref(false)

const CONFIG_TABS = ['server', 'question_split', 'cache', 'proxy_settings']
const isConfigTab = computed(() => CONFIG_TABS.includes(activeTab.value))

/** 与后端 ProxyConfig.default_mode 一致；旧版页面曾使用 direct/cache/fallback */
const PROXY_DEFAULT_MODES = ['direct-backend', 'smart-scheduling', 'transparent-proxy'] as const

function normalizeProxyDefaultMode(mode: string | undefined) {
  if (!mode || (PROXY_DEFAULT_MODES as readonly string[]).includes(mode)) return mode
  const legacy: Record<string, string> = {
    direct: 'direct-backend',
    cache: 'smart-scheduling',
    fallback: 'transparent-proxy'
  }
  return legacy[mode] || 'transparent-proxy'
}

const config = ref<any>({
  server: {
    host: '0.0.0.0',
    port: 20060
  },
  proxy: {
    enabled: true,
    default_mode: 'transparent-proxy',
    timeout: 30,
    retryable_status_codes: [429, 500, 502, 503, 504],
    timeout_retryable: true,
    network_retryable: true,
    circuit_breaker: {
      failure_threshold: 3,
      success_threshold: 2,
      timeout_sec: 60,
      window_sec: 60,
      rate_limit_weight: 2
    }
  },
  system_proxy: {
    enabled: false,
    listen_port: 8080,
    pac_enabled: false
  },
  host_proxy: {
    enabled: false,
    http_port: 8081,
    https_port: 8082
  },
  qa_split: {
    enabled: false,
    backend_id: '',
    model: '',
    prompt: '',
    temperature: 0.3,
    max_tokens: 2000,
    timeout: 120
  },
  question_split: {
    enabled: false,
    fast_split_enabled: true,
    llm_split_enabled: false,
    split_strategy: 'rule',
    backend_id: '',
    model: '',
    synthesis_strategy: 'concat',
    synthesis_backend_id: '',
    synthesis_model: '',
    max_sub_questions: 5,
    timeout: 3,
    complexity_threshold: 0.2
  },
  embedding: {
    enabled: true,
    provider: 'ollama',
    backend_id: 'ollama-local',
    model: 'bge-m3:latest',
    base_url: 'http://localhost:21434'
  },
  cache: {
    enabled: true,
    enable_cache_read: true,
    enable_cache_write: true,
    save_only_mode: false,
    strategy: 'semantic',
    default_ttl: 3600,
    max_cache_size: 0,
    cleanup_interval: 300,
    semantic: {
      enable_auto_embedding: true,
      threshold: 0.8,
      top_k: 5,
      distance_type: 'cosine'
    }
  }
})

// 缓存写入模式（计算属性，用于 UI 显示）
const cacheWriteMode = computed({
  get: () => {
    if (!config.value.cache.enabled) return 'disabled'
    if (config.value.cache.save_only_mode) return 'save_only'
    if (config.value.cache.enable_cache_write === false) return 'disabled'
    return 'normal'
  },
  set: (val: string) => {
    if (val === 'save_only') {
      config.value.cache.save_only_mode = true
      config.value.cache.enable_cache_write = true
    } else if (val === 'disabled') {
      config.value.cache.save_only_mode = false
      config.value.cache.enable_cache_write = false
    } else {
      config.value.cache.save_only_mode = false
      config.value.cache.enable_cache_write = true
    }
  }
})

// 监听 embedding backend_id 变化，加载对应的模型
watch(() => config.value.embedding.backend_id, async (newBackendId, oldBackendId) => {
  if (isInitialLoad.value) return
  if (newBackendId && newBackendId !== oldBackendId) {
    await loadEmbeddingModels(newBackendId)
    config.value.embedding.model = ''
  } else if (!newBackendId) {
    embeddingModels.value = []
    config.value.embedding.model = ''
  }
})

// 监听 qa_split backend_id 变化，加载对应的模型
watch(() => config.value.qa_split.backend_id, async (newBackendId, oldBackendId) => {
  if (isInitialLoad.value) return
  if (newBackendId && newBackendId !== oldBackendId) {
    await loadQAModels(newBackendId)
    config.value.qa_split.model = ''
  } else if (!newBackendId) {
    qaModels.value = []
    config.value.qa_split.model = ''
  }
})

async function loadBackends() {
  try {
    const data = await getBackends()
    backends.value = Array.isArray(data) ? data : []
  } catch (error: any) {
    console.error('Failed to load backends:', error)
  }
}

async function loadEmbeddingModels(backendId: string) {
  if (!backendId) return
  loadingEmbeddingModels.value = true
  try {
    const models = await getBackendModels(backendId, 'embedding')
    embeddingModels.value = models && models.length > 0 ? models : []
  } catch (error: any) {
    console.error('Failed to load embedding models:', error)
    embeddingModels.value = []
  } finally {
    loadingEmbeddingModels.value = false
  }
}

async function loadQAModels(backendId: string) {
  if (!backendId) return
  loadingQAModels.value = true
  try {
    const models = await getBackendModels(backendId, 'chat')
    qaModels.value = models && models.length > 0 ? models : []
  } catch (error: any) {
    console.error('Failed to load QA models:', error)
    qaModels.value = []
  } finally {
    loadingQAModels.value = false
  }
}

async function load() {
  loading.value = true
  try {
    const data = await getConfig()
    isInitialLoad.value = true
    config.value = {
      server: { ...config.value.server, ...data.server },
      proxy: { ...config.value.proxy, ...data.proxy },
      system_proxy: { ...config.value.system_proxy, ...data.system_proxy },
      host_proxy: { ...config.value.host_proxy, ...data.host_proxy },
      qa_split: { ...config.value.qa_split, ...data.qa_split },
      question_split: { ...config.value.question_split, ...(data.question_split || {}) },
      embedding: { ...config.value.embedding, ...data.embedding },
      cache: {
        ...config.value.cache,
        ...data.cache,
        semantic: { ...config.value.cache.semantic, ...(data.cache?.semantic || {}) }
      }
    }
    config.value.proxy.default_mode = normalizeProxyDefaultMode(config.value.proxy.default_mode)
    if (config.value.embedding.backend_id) {
      await loadEmbeddingModels(config.value.embedding.backend_id)
    }
    if (config.value.qa_split.backend_id) {
      await loadQAModels(config.value.qa_split.backend_id)
    }
    await nextTick()
    isInitialLoad.value = false
  } catch (error: any) {
    console.error('Failed to load config:', error)
    ElMessage.error('加载失败: ' + (error.message || '未知错误'))
  } finally {
    loading.value = false
  }
}

async function save() {
  saving.value = true
  try {
    await saveConfig(config.value)
    ElMessage.success('配置已保存')
  } catch (error: any) {
    console.error('Failed to save config:', error)
    ElMessage.error('保存失败: ' + (error.message || '未知错误'))
  } finally {
    saving.value = false
  }
}

onMounted(async () => {
  await loadBackends()
  await load()
})
</script>

<style scoped>
.config-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
}

.header-left {
  flex: 1;
}

.header-actions {
  display: flex;
  gap: 8px;
  align-items: center;
  padding-top: 4px;
}

.config-tabs-wrapper {
  max-width: 1800px;
  margin: 0 auto;
  padding: 0 var(--spacing-sm);
}

.config-tabs {
  width: 100%;
}

.semantic-cache-intro {
  margin-bottom: 16px;
}

.semantic-cache-intro :deep(.el-alert__title) {
  font-size: 0.85rem;
}

.form-tip {
  font-size: 0.75rem;
  color: var(--color-gray-500);
  margin-top: var(--spacing-xs);
}

.unit {
  margin-left: var(--spacing-sm);
  color: var(--color-gray-600);
}

:deep(.el-divider__text) {
  font-weight: 600;
  color: var(--color-gray-700);
}

:deep(.el-tabs--border-card) {
  border: 1px solid var(--color-gray-200);
  box-shadow: none;
}

.config-card {
  margin-bottom: 16px;
}

.config-card :deep(.el-card__header) {
  padding: 12px 16px;
  background-color: var(--el-fill-color-light);
  border-bottom: 1px solid var(--el-border-color-light);
}

.config-card :deep(.el-card__body) {
  padding: 16px;
}

.card-header {
  font-weight: 600;
  color: var(--color-gray-700);
}

:deep(.el-tabs__content) {
  padding: var(--spacing-lg);
}

/* 嵌入子页面：隐藏其自身的页面标题区，保留功能区域 */
:deep(.host-proxy .header-with-toolbar),
:deep(.system-proxy .header-with-toolbar) {
  display: none;
}

:deep(.host-proxy),
:deep(.system-proxy) {
  min-height: unset;
}

:deep(.clash-page) {
  max-width: unset;
  margin: 0;
}
</style>

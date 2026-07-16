/**
 * Provider Registry - Centag 预设 Provider 数据
 * 共享模块：WebUI (ES import） 均可使用
 *
 * @typedef {Object} ProviderModel
 * @property {string} name
 * @property {boolean} supports_tools
 * @property {boolean} supports_images
 * @property {boolean} supports_thinking
 * @property {number} max_context_tokens
 *
 * @typedef {Object} ProviderDef
 * @property {string} id
 * @property {string} name
 * @property {string} type
 * @property {string} base_url
 * @property {string} env_key
 * @property {string} icon
 * @property {string} description
 * @property {ProviderModel[]} default_models
 */

/** @type {Record<string, ProviderDef>} */
const PROVIDER_REGISTRY = {
  // ============ 国际主流 ============
  openai: {
    id: 'openai',
    name: 'OpenAI',
    type: 'openai',
    base_url: 'https://api.openai.com/v1',
    env_key: 'OPENAI_API_KEY',
    icon: '🟢',
    description: 'OpenAI 官方 API（GPT 系列）',
    default_models: [
      { name: 'gpt-4o', supports_tools: true, supports_images: true, supports_thinking: false, max_context_tokens: 128000 },
      { name: 'gpt-4o-mini', supports_tools: true, supports_images: true, supports_thinking: false, max_context_tokens: 128000 },
      { name: 'gpt-4-turbo', supports_tools: true, supports_images: true, supports_thinking: false, max_context_tokens: 128000 },
      { name: 'gpt-3.5-turbo', supports_tools: true, supports_images: false, supports_thinking: false, max_context_tokens: 16384 },
    ],
  },
  anthropic: {
    id: 'anthropic',
    name: 'Anthropic',
    type: 'anthropic',
    base_url: 'https://api.anthropic.com',
    env_key: 'ANTHROPIC_API_KEY',
    icon: '🟣',
    description: 'Anthropic Claude API',
    default_models: [
      { name: 'claude-sonnet-4-20250514', supports_tools: true, supports_images: true, supports_thinking: true, max_context_tokens: 200000 },
      { name: 'claude-opus-4-20250514', supports_tools: true, supports_images: true, supports_thinking: true, max_context_tokens: 200000 },
      { name: 'claude-3-5-haiku-20241022', supports_tools: true, supports_images: true, supports_thinking: false, max_context_tokens: 200000 },
    ],
  },
  google: {
    id: 'google',
    name: 'Google Gemini',
    type: 'openai',
    base_url: 'https://generativelanguage.googleapis.com/v1beta/openai',
    env_key: 'GOOGLE_API_KEY',
    icon: '🔵',
    description: 'Google AI Studio (Gemini)',
    default_models: [
      { name: 'gemini-2.5-pro', supports_tools: true, supports_images: true, supports_thinking: true, max_context_tokens: 1000000 },
      { name: 'gemini-2.5-flash', supports_tools: true, supports_images: true, supports_thinking: true, max_context_tokens: 1000000 },
      { name: 'gemini-2.0-flash', supports_tools: true, supports_images: true, supports_thinking: false, max_context_tokens: 1000000 },
    ],
  },
  openrouter: {
    id: 'openrouter',
    name: 'OpenRouter',
    type: 'openai',
    base_url: 'https://openrouter.ai/api/v1',
    env_key: 'OPENROUTER_API_KEY',
    icon: '🌐',
    description: 'OpenRouter 聚合平台（200+ 模型）',
    default_models: [
      { name: 'anthropic/claude-sonnet-4', supports_tools: true, supports_images: true, supports_thinking: true, max_context_tokens: 200000 },
      { name: 'google/gemini-2.5-pro', supports_tools: true, supports_images: true, supports_thinking: true, max_context_tokens: 1000000 },
      { name: 'openai/gpt-4o', supports_tools: true, supports_images: true, supports_thinking: false, max_context_tokens: 128000 },
      { name: 'deepseek/deepseek-chat', supports_tools: true, supports_images: false, supports_thinking: false, max_context_tokens: 64000 },
    ],
  },
  // ============ 本地服务 ============
  ollama: {
    id: 'ollama',
    name: 'Ollama',
    type: 'ollama',
    base_url: 'http://localhost:11434',
    env_key: '',
    icon: '🦙',
    description: '本地 Ollama 模型服务',
    default_models: [
      { name: 'llama3.1', supports_tools: false, supports_images: false, supports_thinking: false, max_context_tokens: 128000 },
      { name: 'qwen2.5', supports_tools: false, supports_images: false, supports_thinking: false, max_context_tokens: 128000 },
      { name: 'gemma2', supports_tools: false, supports_images: false, supports_thinking: false, max_context_tokens: 8192 },
      { name: 'mistral', supports_tools: false, supports_images: false, supports_thinking: false, max_context_tokens: 32000 },
    ],
  },
  lmstudio: {
    id: 'lmstudio',
    name: 'LM Studio',
    type: 'openai',
    base_url: 'http://127.0.0.1:1234/v1',
    env_key: '',
    icon: '🖥️',
    description: 'LM Studio 本地服务器',
    default_models: [],
  },
  // ============ 中国厂商 ============
  deepseek: {
    id: 'deepseek',
    name: 'DeepSeek (深度求索)',
    type: 'openai',
    base_url: 'https://api.deepseek.com/v1',
    env_key: 'DEEPSEEK_API_KEY',
    icon: '🔵',
    description: 'DeepSeek API（深度求索）',
    default_models: [
      { name: 'deepseek-v4-flash', supports_tools: true, supports_images: false, supports_thinking: false, max_context_tokens: 128000 },
      { name: 'deepseek-v4-pro', supports_tools: true, supports_images: false, supports_thinking: false, max_context_tokens: 128000 },
    ],
  },
  dashscope: {
    id: 'dashscope',
    name: 'DashScope (阿里云百炼)',
    type: 'openai',
    base_url: 'https://dashscope.aliyuncs.com/compatible-mode/v1',
    env_key: 'DASHSCOPE_API_KEY',
    icon: '🟠',
    description: 'DashScope API（通义千问系列）',
    default_models: [
      { name: 'qwen-max', supports_tools: true, supports_images: false, supports_thinking: false, max_context_tokens: 32000 },
      { name: 'qwen-plus', supports_tools: true, supports_images: false, supports_thinking: false, max_context_tokens: 131072 },
      { name: 'qwen-turbo', supports_tools: true, supports_images: false, supports_thinking: false, max_context_tokens: 131072 },
      { name: 'qwen-long', supports_tools: false, supports_images: false, supports_thinking: false, max_context_tokens: 10000000 },
      { name: 'qwen-vl-plus', supports_tools: false, supports_images: true, supports_thinking: false, max_context_tokens: 131072 },
    ],
  },
  bigmodel: {
    id: 'bigmodel',
    name: 'BigModel (智谱 AI)',
    type: 'openai',
    base_url: 'https://open.bigmodel.cn/api/paas/v4',
    env_key: 'ZHIPU_API_KEY',
    icon: '🟤',
    description: '智谱 AI（GLM 系列）',
    default_models: [
      { name: 'glm-4-plus', supports_tools: true, supports_images: true, supports_thinking: false, max_context_tokens: 128000 },
      { name: 'glm-4-flash', supports_tools: true, supports_images: false, supports_thinking: false, max_context_tokens: 128000 },
      { name: 'glm-4-long', supports_tools: false, supports_images: false, supports_thinking: false, max_context_tokens: 1000000 },
      { name: 'glm-4v-plus', supports_tools: false, supports_images: true, supports_thinking: false, max_context_tokens: 8192 },
    ],
  },
  moonshot: {
    id: 'moonshot',
    name: 'Moonshot (月之暗面)',
    type: 'openai',
    base_url: 'https://api.moonshot.cn/v1',
    env_key: 'MOONSHOT_API_KEY',
    icon: '🌙',
    description: 'Moonshot API（Kimi 系列）',
    default_models: [
      { name: 'moonshot-v1-128k', supports_tools: true, supports_images: false, supports_thinking: false, max_context_tokens: 128000 },
      { name: 'moonshot-v1-auto', supports_tools: true, supports_images: false, supports_thinking: false, max_context_tokens: 128000 },
    ],
  },
  minimax: {
    id: 'minimax',
    name: 'MiniMax (稀宇科技)',
    type: 'openai',
    base_url: 'https://api.minimax.chat/v1',
    env_key: 'MINIMAX_API_KEY',
    icon: '💎',
    description: 'MiniMax API（稀宇科技）',
    default_models: [
      { name: 'MiniMax-M2.5', supports_tools: true, supports_images: false, supports_thinking: false, max_context_tokens: 1000000 },
      { name: 'MiniMax-Text-01', supports_tools: true, supports_images: false, supports_thinking: false, max_context_tokens: 4000000 },
    ],
  },
  minimax_cn: {
    id: 'minimax_cn',
    name: 'MiniMax CN (稀宇科技国内版)',
    type: 'openai',
    base_url: 'https://api.minimax.chat/v1',
    env_key: 'MINIMAX_CN_API_KEY',
    icon: '💎',
    description: 'MiniMax CN 国内版',
    default_models: [
      { name: 'MiniMax-M2.5', supports_tools: true, supports_images: false, supports_thinking: false, max_context_tokens: 1000000 },
    ],
  },
  siliconflow: {
    id: 'siliconflow',
    name: 'SiliconFlow (硅基流动)',
    type: 'openai',
    base_url: 'https://api.siliconflow.cn/v1',
    env_key: 'SILICONFLOW_API_KEY',
    icon: '⚡',
    description: 'SiliconFlow API（硅基流动）',
    default_models: [
      { name: 'deepseek-ai/DeepSeek-V3', supports_tools: true, supports_images: false, supports_thinking: false, max_context_tokens: 64000 },
      { name: 'Qwen/Qwen2.5-72B-Instruct', supports_tools: true, supports_images: false, supports_thinking: false, max_context_tokens: 131072 },
      { name: 'THUDM/glm-4-9b-chat', supports_tools: false, supports_images: false, supports_thinking: false, max_context_tokens: 128000 },
    ],
  },
  ppio: {
    id: 'ppio',
    name: 'PPIO (派欧云)',
    type: 'openai',
    base_url: 'https://api.ppinfra.com/v3/openai',
    env_key: 'PPIO_API_KEY',
    icon: '🔶',
    description: 'PPIO API（派欧云）',
    default_models: [
      { name: 'deepseek-ai/deepseek-v3/community', supports_tools: true, supports_images: false, supports_thinking: false, max_context_tokens: 64000 },
    ],
  },
  // ============ 其他国际 ============
  huggingface: {
    id: 'huggingface',
    name: 'Hugging Face',
    type: 'openai',
    base_url: 'https://api-inference.huggingface.co/v1',
    env_key: 'HF_TOKEN',
    icon: '🤗',
    description: 'Hugging Face Inference API',
    default_models: [
      { name: 'meta-llama/Llama-3.1-405B-Instruct', supports_tools: false, supports_images: false, supports_thinking: false, max_context_tokens: 128000 },
      { name: 'Qwen/Qwen2.5-72B-Instruct', supports_tools: false, supports_images: false, supports_thinking: false, max_context_tokens: 128000 },
    ],
  },
  nvidia: {
    id: 'nvidia',
    name: 'NVIDIA NIM',
    type: 'openai',
    base_url: 'https://integrate.api.nvidia.com/v1',
    env_key: 'NVIDIA_API_KEY',
    icon: '💚',
    description: 'NVIDIA NIM / build.nvidia.com',
    default_models: [
      { name: 'nvidia/llama-3.1-405b-instruct', supports_tools: false, supports_images: false, supports_thinking: false, max_context_tokens: 128000 },
      { name: 'nvidia/llama-3.1-70b-instruct', supports_tools: false, supports_images: false, supports_thinking: false, max_context_tokens: 128000 },
    ],
  },
  xai: {
    id: 'xai',
    name: 'xAI',
    type: 'openai',
    base_url: 'https://api.x.ai/v1',
    env_key: 'XAI_API_KEY',
    icon: '✖️',
    description: 'xAI（Grok 系列）',
    default_models: [
      { name: 'grok-2', supports_tools: true, supports_images: true, supports_thinking: false, max_context_tokens: 128000 },
      { name: 'grok-2-mini', supports_tools: true, supports_images: false, supports_thinking: false, max_context_tokens: 128000 },
    ],
  },
  arcee: {
    id: 'arcee',
    name: 'Arcee AI',
    type: 'openai',
    base_url: 'https://api.arcee.ai/v2',
    env_key: 'ARCEEAI_API_KEY',
    icon: '🎯',
    description: 'Arcee AI Trinity Models',
    default_models: [
      { name: 'trinity-large', supports_tools: true, supports_images: false, supports_thinking: false, max_context_tokens: 128000 },
      { name: 'trinity-small', supports_tools: false, supports_images: false, supports_thinking: false, max_context_tokens: 32000 },
    ],
  },
  xiaomi: {
    id: 'xiaomi',
    name: 'Xiaomi MiMo',
    type: 'openai',
    base_url: 'https://api.xiaomi.com/v1',
    env_key: 'XIAOMI_API_KEY',
    icon: '📱',
    description: 'Xiaomi MiMo Models',
    default_models: [
      { name: 'mimo-v2-pro', supports_tools: true, supports_images: false, supports_thinking: false, max_context_tokens: 128000 },
    ],
  },
  bedrock: {
    id: 'bedrock',
    name: 'AWS Bedrock',
    type: 'openai',
    base_url: 'https://bedrock-runtime.us-east-1.amazonaws.com/v1',
    env_key: 'AWS_ACCESS_KEY_ID',
    icon: '☁️',
    description: 'AWS Bedrock',
    default_models: [
      { name: 'anthropic.claude-3-5-sonnet-20241022-v2:0', supports_tools: true, supports_images: true, supports_thinking: false, max_context_tokens: 200000 },
      { name: 'amazon.titan-text-express-v1', supports_tools: false, supports_images: false, supports_thinking: false, max_context_tokens: 8000 },
    ],
  },
  azure: {
    id: 'azure',
    name: 'Azure OpenAI',
    type: 'openai',
    base_url: 'https://your-resource.openai.azure.com/openai/v1',
    env_key: 'AZURE_API_KEY',
    icon: '🔷',
    description: 'Azure OpenAI Service',
    default_models: [
      { name: 'gpt-4o', supports_tools: true, supports_images: true, supports_thinking: false, max_context_tokens: 128000 },
      { name: 'gpt-4o-mini', supports_tools: true, supports_images: true, supports_thinking: false, max_context_tokens: 128000 },
    ],
  },
  copilot: {
    id: 'copilot',
    name: 'GitHub Copilot',
    type: 'openai',
    base_url: 'https://api.githubcopilot.com/v1',
    env_key: 'GITHUB_TOKEN',
    icon: '🐙',
    description: 'GitHub Copilot / GitHub Models',
    default_models: [
      { name: 'gpt-4o', supports_tools: true, supports_images: true, supports_thinking: false, max_context_tokens: 128000 },
      { name: 'claude-3.5-sonnet', supports_tools: true, supports_images: true, supports_thinking: false, max_context_tokens: 200000 },
    ],
  },
  kilocode: {
    id: 'kilocode',
    name: 'KiloCode',
    type: 'openai',
    base_url: 'https://api.kilocode.com/v1',
    env_key: 'KILOCODE_API_KEY',
    icon: '💻',
    description: 'KiloCode Gateway',
    default_models: [],
  },
  custom: {
    id: 'custom',
    name: '自定义 (OpenAI 兼容)',
    type: 'openai',
    base_url: '',
    env_key: 'CUSTOM_API_KEY',
    icon: '⚙️',
    description: '任何 OpenAI 兼容的 API 端点',
    default_models: [],
  },
}

/**
 * 获取所有 Provider 列表
 * @returns {ProviderDef[]}
 */
function getProviderList() {
  return Object.values(PROVIDER_REGISTRY)
}

/**
 * 根据 ID 获取 Provider
 * @param {string} id
 * @returns {ProviderDef | null}
 */
function getProviderById(id) {
  return PROVIDER_REGISTRY[id] || null
}

/**
 * 获取 Provider 的默认模型列表
 * @param {string} providerId
 * @returns {ProviderModel[]}
 */
function getDefaultModels(providerId) {
  const provider = getProviderById(providerId)
  return provider ? [...provider.default_models] : []
}

// UMD: 兼容 script 标签 (WebUI) 和 ES module (webui)
if (typeof module !== 'undefined' && module.exports) {
  module.exports = { PROVIDER_REGISTRY, getProviderList, getProviderById, getDefaultModels }
}

// classic script + provider-form 依赖解析用
if (typeof globalThis !== 'undefined') {
  globalThis.PROVIDER_REGISTRY = PROVIDER_REGISTRY
  globalThis.getProviderList = getProviderList
  globalThis.getProviderById = getProviderById
  globalThis.getDefaultModels = getDefaultModels
}

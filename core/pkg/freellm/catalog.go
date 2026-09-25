// Package freellm 提供 centag 自动获取（聚合）各 LLM 后端免费大模型的能力。
//
// 设计参考：
//   - freellmapi：provider 以 register(new OpenAICompatProvider({platform, name, baseUrl, keyless, validateUrl})) 注册，
//     模型目录外部化到签名 catalog，路由按 (platform,model,key) 做速率/配额跟踪与 failover。
//   - OmniRoute / 9Router：provider catalog 以 hasFree / No-auth 标签标记免费层，auto-combo 评分路由，四层回退级联。
//
// 本包把它们落到 centag 已有的 BackendConfig 模型上：每个免费 provider 直接转成一个 BackendConfig，
// 复用 centag 既有的 FallbackBackends（故障转移）、AccountPool（多 key 轮转）、TenantID（租户隔离）、
// AutoFetchModels + ProbeModel（自动发现模型）能力。零配置（keyless）后端可一键/批量注册，无卡免费层由用户填 key。
package freellm

// AuthType 描述免费 provider 的鉴权方式，对应 OmniRoute 的 Tags（No-auth / API key / OAuth）。
const (
	// AuthKeyless 无需任何密钥即可使用（匿名/永久免费）。
	AuthKeyless = "keyless"
	// AuthAPIKey 需要 API Key，但通常为无信用卡免费层（用户自行提供）。
	AuthAPIKey = "apikey"
	// AuthOAuth 需要 OAuth 登录换取免费层。
	AuthOAuth = "oauth"
)

// ProviderCatalogEntry 描述一个可提供免费模型的 LLM 后端。
// 字段刻意保持与 centag BackendConfig 解耦，便于后续从远程签名 catalog 同步（对标 freellmapi 的 catalog-sync）。
type ProviderCatalogEntry struct {
	// ID 平台唯一标识，如 "pollinations"；注册到 centag 后后端 ID 为 "freellm-<ID>"。
	ID string `json:"id"`
	// Name 展示名称。
	Name string `json:"name"`
	// BaseURL OpenAI 兼容端点。
	BaseURL string `json:"base_url"`
	// AuthType 鉴权方式（keyless / apikey / oauth）。
	AuthType string `json:"auth_type"`
	// Type centag 后端类型，免费聚合后端均为 openai 兼容。
	Type string `json:"type"`
	// KnownFreeModels 该 provider 已知的免费模型，用于种子 SupportedModels；
	// 注册后 centag 会通过 AutoFetchModels 实时拉取并覆盖为真实列表。
	KnownFreeModels []string `json:"known_free_models"`
	// ProbeModel 连通性探测用的免费模型；为空时取 KnownFreeModels[0]。
	ProbeModel string `json:"probe_model,omitempty"`
	// DummyKey keyless 但要求非空 key 的 provider 使用的占位 key（如 llm7 接受任意非空串）。
	DummyKey string `json:"-"`
	// RateLimitNotes 免费层限额/速率说明（写入后端 metadata，供前端展示）。
	RateLimitNotes string `json:"rate_limit_notes,omitempty"`
	// DocURL 官方/文档链接。
	DocURL string `json:"doc_url,omitempty"`
	// Tags 额外标签（如 image / audio / aggregator）。
	Tags []string `json:"tags,omitempty"`
}

// Catalog 是内置的免费 LLM 后端目录（v1 手工维护，后续可改为远程签名 catalog 同步）。
// 数据来源于对 freellmapi / OmniRoute(9Router) / free-llm-api-resources 的公开调研，
// 仅收录官方正规免费层，拒绝逆向/破解类灰色资源。
var Catalog = []ProviderCatalogEntry{
	// ───────────────────────── 零配置（keyless）后端 ─────────────────────────
	{
		ID:              "pollinations",
		Name:            "Pollinations AI",
		BaseURL:         "https://text.pollinations.ai/openai/v1",
		AuthType:        AuthKeyless,
		Type:            "openai",
		KnownFreeModels: []string{"openai-fast"},
		ProbeModel:      "openai-fast",
		RateLimitNotes:  "匿名访问免费模型（GPT-OSS 20B），best-effort，无 SLA",
		DocURL:          "https://pollinations.ai",
		Tags:            []string{"chat", "image"},
	},
	{
		ID:              "llm7",
		Name:            "LLM7.io",
		BaseURL:         "https://api.llm7.io/v1",
		AuthType:        AuthKeyless,
		Type:            "openai",
		KnownFreeModels: []string{"gpt-oss-20b", "llama-3.1-8b-instruct", "codestral-latest", "ministral-8b"},
		ProbeModel:      "gpt-oss-20b",
		DummyKey:        "unused",
		RateLimitNotes:  "匿名/任意非空 key，约 100 req/hr 免费",
		DocURL:          "https://llm7.io",
		Tags:            []string{"chat"},
	},
	{
		ID:              "kilo-gateway",
		Name:            "Kilo Gateway",
		BaseURL:         "https://api.kilo.ai/api/gateway/v1",
		AuthType:        AuthKeyless,
		Type:            "openai",
		KnownFreeModels: []string{"kilo-free"},
		ProbeModel:      "kilo-free",
		RateLimitNotes:  ":free 路由匿名访问，约 200 req/hr/IP，best-effort",
		DocURL:          "https://kilo.ai",
		Tags:            []string{"chat"},
	},
	{
		ID:              "ovh",
		Name:            "OVHcloud AI Endpoints",
		BaseURL:         "https://oai.endpoints.kepler.ai.cloud.ovh.net/v1",
		AuthType:        AuthKeyless,
		Type:            "openai",
		KnownFreeModels: []string{"Llama-3.3-70B-Instruct", "Deepseek-R1-Distill-Llama-70B"},
		ProbeModel:      "Llama-3.3-70B-Instruct",
		RateLimitNotes:  "匿名访问，约 2 req/min/IP（实测更严），无信用卡",
		DocURL:          "https://endpoints.ai.cloud.ovh.net",
		Tags:            []string{"chat"},
	},
	{
		ID:              "opencode-free",
		Name:            "OpenCode Free",
		BaseURL:         "https://opencode.ai/zen/v1",
		AuthType:        AuthKeyless,
		Type:            "openai",
		KnownFreeModels: []string{"oc-free"},
		ProbeModel:      "oc-free",
		RateLimitNotes:  "免密钥公开免费端点（best-effort，可能限流）",
		DocURL:          "https://opencode.ai",
		Tags:            []string{"chat"},
	},
	{
		ID:              "felo",
		Name:            "Felo",
		BaseURL:         "https://api.felo.ai/v1",
		AuthType:        AuthKeyless,
		Type:            "openai",
		KnownFreeModels: []string{"felo-search"},
		ProbeModel:      "felo-search",
		RateLimitNotes:  "免注册免费聊天/搜索聚合，best-effort",
		DocURL:          "https://felo.ai",
		Tags:            []string{"chat", "search"},
	},

	// ───────────────────────── 无卡免费层（需用户提供 key） ─────────────────────────
	{
		ID:              "groq",
		Name:            "Groq",
		BaseURL:         "https://api.groq.com/openai/v1",
		AuthType:        AuthAPIKey,
		Type:            "openai",
		KnownFreeModels: []string{"llama-3.3-70b-versatile", "llama-3.1-8b-instant", "gemma2-9b-it"},
		ProbeModel:      "llama-3.1-8b-instant",
		RateLimitNotes:  "免费层每日数千万 token，无需信用卡",
		DocURL:          "https://console.groq.com",
		Tags:            []string{"chat"},
	},
	{
		ID:              "cerebras",
		Name:            "Cerebras",
		BaseURL:         "https://api.cerebras.ai/v1",
		AuthType:        AuthAPIKey,
		Type:            "openai",
		KnownFreeModels: []string{"llama-3.3-70b", "llama3.1-8b", "gemma-9b-it"},
		ProbeModel:      "llama3.1-8b",
		RateLimitNotes:  "免费层无需信用卡",
		DocURL:          "https://cerebras.ai",
		Tags:            []string{"chat"},
	},
	{
		ID:              "mistral",
		Name:            "Mistral AI",
		BaseURL:         "https://api.mistral.ai/v1",
		AuthType:        AuthAPIKey,
		Type:            "openai",
		KnownFreeModels: []string{"mistral-small-latest", "codestral-latest"},
		ProbeModel:      "mistral-small-latest",
		RateLimitNotes:  "免费层无需信用卡",
		DocURL:          "https://console.mistral.ai",
		Tags:            []string{"chat"},
	},
	{
		ID:              "nvidia-nim",
		Name:            "NVIDIA NIM",
		BaseURL:         "https://integrate.api.nvidia.com/v1",
		AuthType:        AuthAPIKey,
		Type:            "openai",
		KnownFreeModels: []string{"meta/llama-3.3-70b-instruct", "nvidia/llama-3.1-nemotron-70b-instruct"},
		ProbeModel:      "meta/llama-3.3-70b-instruct",
		RateLimitNotes:  "免费层无需信用卡",
		DocURL:          "https://build.nvidia.com",
		Tags:            []string{"chat"},
	},
	{
		ID:              "siliconflow",
		Name:            "SiliconFlow",
		BaseURL:         "https://api.siliconflow.com/v1",
		AuthType:        AuthAPIKey,
		Type:            "openai",
		KnownFreeModels: []string{"Qwen/Qwen3-8B", "deepseek-ai/DeepSeek-V3"},
		ProbeModel:      "Qwen/Qwen3-8B",
		RateLimitNotes:  "无卡，含免费媒体模型",
		DocURL:          "https://siliconflow.com",
		Tags:            []string{"chat", "image", "audio"},
	},
	{
		ID:              "huggingface",
		Name:            "Hugging Face Router",
		BaseURL:         "https://router.huggingface.co/v1",
		AuthType:        AuthAPIKey,
		Type:            "openai",
		KnownFreeModels: []string{"google/gemma-3-12b-it", "meta-llama/Llama-3.3-70B-Instruct"},
		ProbeModel:      "meta-llama/Llama-3.3-70B-Instruct",
		RateLimitNotes:  "月 $0.10 额度，无需信用卡",
		DocURL:          "https://huggingface.co",
		Tags:            []string{"chat"},
	},
	{
		ID:              "openrouter",
		Name:            "OpenRouter",
		BaseURL:         "https://openrouter.ai/api/v1",
		AuthType:        AuthAPIKey,
		Type:            "openai",
		KnownFreeModels: []string{"meta-llama/llama-3.3-70b-instruct:free", "google/gemini-2.0-flash-exp:free"},
		ProbeModel:      "meta-llama/llama-3.3-70b-instruct:free",
		RateLimitNotes:  "部分 :free 模型，限流但免费",
		DocURL:          "https://openrouter.ai",
		Tags:            []string{"chat"},
	},
	{
		ID:              "github-models",
		Name:            "GitHub Models",
		BaseURL:         "https://models.github.ai/inference",
		AuthType:        AuthAPIKey,
		Type:            "openai",
		KnownFreeModels: []string{"gpt-4.1", "gpt-4o", "meta/llama-3.3-70b-instruct"},
		ProbeModel:      "gpt-4o",
		RateLimitNotes:  "GitHub 账号免费，限实验用途",
		DocURL:          "https://github.com/marketplace/models",
		Tags:            []string{"chat"},
	},
	{
		ID:              "cloudflare",
		Name:            "Cloudflare Workers AI",
		BaseURL:         "https://api.cloudflare.com/client/v4/workers/ai/v1",
		AuthType:        AuthAPIKey,
		Type:            "openai",
		KnownFreeModels: []string{"@cf/meta/llama-3.3-70b-instruct", "@cf/qwen/qwq-32b"},
		ProbeModel:      "@cf/meta/llama-3.3-70b-instruct",
		RateLimitNotes:  "每日免费额度，account_id:token 鉴权",
		DocURL:          "https://workers.cloudflare.com",
		Tags:            []string{"chat"},
	},
	{
		ID:              "zhipu",
		Name:            "Z.ai (智谱)",
		BaseURL:         "https://api.z.ai/v1",
		AuthType:        AuthAPIKey,
		Type:            "openai",
		KnownFreeModels: []string{"glm-4.6", "glm-4.7-flash"},
		ProbeModel:      "glm-4.7-flash",
		RateLimitNotes:  "无卡免费层（含 flash 模型）",
		DocURL:          "https://z.ai",
		Tags:            []string{"chat"},
	},
	{
		ID:              "moonshot",
		Name:            "Moonshot AI (Kimi)",
		BaseURL:         "https://api.moonshot.cn/v1",
		AuthType:        AuthAPIKey,
		Type:            "openai",
		KnownFreeModels: []string{"kimi-k2.5", "kimi-k2.6"},
		ProbeModel:      "kimi-k2.5",
		RateLimitNotes:  "免费额度（注意：Kimi 属 Moonshot AI，非 DashScope，端点不同）",
		DocURL:          "https://platform.moonshot.cn",
		Tags:            []string{"chat"},
	},
	{
		ID:              "google",
		Name:            "Google Gemini",
		BaseURL:         "https://generativelanguage.googleapis.com/v1beta/openai/",
		AuthType:        AuthAPIKey,
		Type:            "openai",
		KnownFreeModels: []string{"gemini-2.5-flash", "gemini-2.0-flash"},
		ProbeModel:      "gemini-2.5-flash",
		RateLimitNotes:  "免费层强，需注意使用条款（偏向非消费级）",
		DocURL:          "https://ai.google.dev",
		Tags:            []string{"chat"},
	},
}

// defaultTimeout 免费后端默认请求超时（秒）。
const defaultTimeout = 60

// FindEntry 按 ID 查找目录条目。
func FindEntry(id string) (ProviderCatalogEntry, bool) {
	for _, e := range Catalog {
		if e.ID == id {
			return e, true
		}
	}
	return ProviderCatalogEntry{}, false
}

// ListCatalog 返回全部目录条目，可选按 AuthType 过滤（"" 表示不过滤）。
func ListCatalog(authType string) []ProviderCatalogEntry {
	if authType == "" {
		out := make([]ProviderCatalogEntry, len(Catalog))
		copy(out, Catalog)
		return out
	}
	out := make([]ProviderCatalogEntry, 0, len(Catalog))
	for _, e := range Catalog {
		if e.AuthType == authType {
			out = append(out, e)
		}
	}
	return out
}

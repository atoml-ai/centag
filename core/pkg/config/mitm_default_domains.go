package config

import "strings"

// DefaultMITMDomains returns the default MITM/PAC domain allowlist.
//
// Sources consolidated from:
//   - config/profiles/_shared/initdata/backends-catalog.yaml
//   - LiteLLM / OpenRouter-style provider catalogs (frontier + inference platforms)
//   - Common China LLM APIs and aggregators used by coding agents
//
// Matching: PAC uses dnsDomainIs (exact or subdomain). MITM whitelist uses the
// same semantics — listing "openai.azure.com" covers "*.openai.azure.com".
// Prefer listing API host roots only; keep marketing/docs/consoles off the list
// so CONNECT tunnels them without decryption.
func DefaultMITMDomains() []string {
	return []string{
		// --- backends-catalog / China majors ---
		"api.baichuan-ai.com",
		"api.deepseek.com",
		"api.kimi.com",
		"api.minimax.chat",
		"api.minimax.io",
		"api.moonshot.ai",
		"api.moonshot.cn",
		"api.ppinfra.com",
		"api.ppio.com",
		"api.siliconflow.cn",
		"api.siliconflow.com",
		"api.stepfun.com",
		"ark.cn-beijing.volces.com",
		"ark.ap-southeast.bytepluses.com",
		"coding.dashscope.aliyuncs.com",
		"dashscope-intl.aliyuncs.com",
		"dashscope.aliyuncs.com",
		"hunyuan.tencentcloudapi.com",
		"tokenhub-intl.tencentcloudmaas.com",
		"integrate.api.nvidia.com",
		"open.bigmodel.cn",
		"api.bigmodel.cn",
		"api.z.ai",
		"opencode.ai",
		"qianfan.baidubce.com",
		"api.baiduqianfan.ai",
		"spark-api.xf-yun.com",
		"spark-api-open.xf-yun.com",
		"api.lingyiwanwu.com",
		"api.01.ai",
		"api.sensenova.cn",
		"api.infini-ai.com",
		"cloud.infini-ai.com",
		"api-inference.modelscope.cn",
		"api.modelverse.cn",
		"api.lkeap.cloud.tencent.com",
		"hunyuanapi.tencentcs.com",
		"wishub-x1.ctyun.cn",
		"maas-api.cn-huabei-1.xfyun.cn",
		"api.n1n.ai",
		"api.tokenpony.cn",
		"api.gptgod.online",
		"api.closeai-proxy.xyz",
		"api.ohmygpt.com",
		"openai.api2d.net",
		"api.aiproxy.io",
		"api.chatanywhere.tech",
		"api.chatanywhere.com.cn",

		// --- Frontier / global first-party ---
		"api.openai.com",
		"api.anthropic.com",
		"api.mistral.ai",
		"codestral.mistral.ai",
		"api.x.ai",
		"api.groq.com",
		"api.cohere.ai",
		"api.cohere.com",
		"generativelanguage.googleapis.com",
		"aiplatform.googleapis.com",
		"api.ai21.com",
		"api.perplexity.ai",
		"api.reka.ai",
		"api.upstage.ai",
		"api.voyageai.com",
		"api.jina.ai",
		"api.nomic.ai",
		"api.writer.com",
		"api.aleph-alpha.com",
		"api.llama.com",
		"api.deepgram.com",
		"api.nscale.com",
		"api.publicai.co",
		"serverless.tensormesh.ai",
		"endpoints.ai.cloud.ovh.net",

		// --- Cloud regional roots (subdomain match covers tenants/regions) ---
		"openai.azure.com",
		"services.ai.azure.com",
		"models.ai.azure.com",
		"inference.ai.azure.com",
		"cognitiveservices.azure.com",
		"models.inference.ai.azure.com",
		"models.github.ai",
		// AWS Bedrock Runtime (common regions; custom regions can be added in UI)
		"bedrock-runtime.us-east-1.amazonaws.com",
		"bedrock-runtime.us-east-2.amazonaws.com",
		"bedrock-runtime.us-west-2.amazonaws.com",
		"bedrock-runtime.eu-west-1.amazonaws.com",
		"bedrock-runtime.eu-central-1.amazonaws.com",
		"bedrock-runtime.ap-northeast-1.amazonaws.com",
		"bedrock-runtime.ap-southeast-1.amazonaws.com",
		"bedrock-runtime.ap-southeast-2.amazonaws.com",

		// --- Inference platforms / GPU clouds ---
		"api.together.xyz",
		"api.fireworks.ai",
		"api.deepinfra.com",
		"api.cerebras.ai",
		"api.sambanova.ai",
		"api.studio.nebius.ai",
		"api.tokenfactory.nebius.com",
		"api.friendli.ai",
		"api.novita.ai",
		"api.hyperbolic.xyz",
		"api.lepton.ai",
		"api.huggingface.co",
		"router.huggingface.co",
		"api.replicate.com",
		"api.morphllm.com",
		"api.inference.net",
		"api.featherless.ai",
		"api.lambda.ai",
		"api.lambdalabs.com",
		"inference.baseten.co",
		"api.endpoints.anyscale.com",
		"oai.endpoints.kepler.ai.cloud.ovh.net",
		"inference.do-ai.run",
		"api.inference.wandb.ai",
		"api.scaleway.ai",
		"api.parasail.io",
		"api.saas.parasail.io",
		"api.akashml.com",
		"api.gmi-serving.com",
		"api.kluster.ai",
		"api.cortecs.ai",
		"api.nextbit256.com",
		"api.nlpcloud.io",
		"api.relace.ai",
		"api.inceptionlabs.ai",
		"api.intelligence.io.solutions",
		"api.aionlabs.ai",
		"api.lemondata.ai",
		"api.ppq.ai",
		"api.sakana.ai",
		"api.atlascloud.ai",
		"api.galadriel.com",
		"api.synthetic.new",
		"api.soniox.com",
		"api.stima.tech",
		"api.v0.dev",
		"nano-gpt.com",
		"mancer.tech",
		"app.empower.dev",

		// --- Aggregators / gateways / agents ---
		"openrouter.ai",
		"api.portkey.ai",
		"api.llmgateway.io",
		"api.opper.ai",
		"ai-gateway.vercel.sh",
		"gateway.ai.cloudflare.com",
		"api.302.ai",
		"llm.chutes.ai",
		"zenmux.ai",
		"router.requesty.ai",
		"api.aimlapi.com",
		"api.edenai.co",
		"api.unify.ai",
		"api.venice.ai",
		"api.totalgpt.ai",
		"api.coze.com",
		"api.coze.cn",
		"api.puter.com",
		"api.poe.com",
		"api.manus.im",
		"api.groqcloud.com",
	}
}

// DefaultMITMPathPatterns returns path prefixes that identify LLM API calls on
// whitelisted domains (used together with looksLikeLLMAPIPath).
func DefaultMITMPathPatterns() []string {
	return []string{
		"/v1",
		"/v1/chat/completions",
		"/v1/completions",
		"/v1/embeddings",
		"/v1/models",
		"/v1/messages",
		"/v1/responses",
		"/v1/moderations",
		"/v1/assistants",
		"/v1/threads",
		"/v1/runs",
		"/v1/batches",
		"/v1beta",
		"/v1beta/openai",
		"/v2",
		"/v3",
		"/v3/openai/chat/completions",
		"/v4",
		"/api",
		"/api/v1",
		"/api/v1/cursor",
		"/api/paas/v4",
		"/openai/v1",
		"/openai/deployments",
		"/compatible-mode/v1",
		"/coding/v1",
		"/zen/v1",
		"/inference/v1",
		"/studio/v1",
		"/serverless/v1",
		"/anthropic",
		"/anthropic/v1",
		"/model",
		"/models",
	}
}

// DefaultHostProxyDomainMapping maps MITM domains to the local Centag API for host-hijack mode.
func DefaultHostProxyDomainMapping(backendAddr string) map[string]string {
	if backendAddr == "" {
		backendAddr = "http://127.0.0.1:20060"
	}
	out := make(map[string]string, len(DefaultMITMDomains()))
	for _, d := range DefaultMITMDomains() {
		out[d] = backendAddr
	}
	return out
}

// HostMatchesMITMDomain reports whether host equals domain or is a subdomain of it
// (same semantics as PAC dnsDomainIs).
func HostMatchesMITMDomain(host, domain string) bool {
	host = normalizeMITMHost(host)
	domain = normalizeMITMHost(domain)
	if host == "" || domain == "" {
		return false
	}
	if host == domain {
		return true
	}
	return strings.HasSuffix(host, "."+domain)
}

func normalizeMITMHost(host string) string {
	h := strings.ToLower(strings.TrimSpace(host))
	if i := strings.LastIndex(h, ":"); i >= 0 {
		// strip :port but keep IPv6-ish brackets out of scope (LLM APIs are DNS names)
		if !strings.Contains(h, "]") {
			h = h[:i]
		}
	}
	return h
}

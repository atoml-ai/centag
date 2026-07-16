package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"centag/core/internal/httpclient"
	"centag/core/pkg/logger"
)

func truncateErrBody(b []byte, max int) string {
	s := strings.TrimSpace(string(b))
	if s == "" {
		return "(空响应体)"
	}
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func pickOpenAITestModel(cfg *BackendConfig) string {
	if cfg != nil {
		if probe := strings.TrimSpace(cfg.ProbeModel); probe != "" {
			return probe
		}
	}
	for _, m := range cfg.SupportedModels {
		if m.ActualModel != "" {
			return m.ActualModel
		}
		if m.RequestedModel != "" {
			return m.RequestedModel
		}
	}
	return "gpt-3.5-turbo"
}

func firstOpenAIModelFromListJSON(body []byte, fallback string) string {
	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if len(body) > 0 && json.Unmarshal(body, &result) == nil && len(result.Data) > 0 && result.Data[0].ID != "" {
		return result.Data[0].ID
	}
	return fallback
}

func preferOpenAIModelFromListJSON(body []byte, preferred, fallback string) string {
	if strings.TrimSpace(preferred) == "" {
		return firstOpenAIModelFromListJSON(body, fallback)
	}
	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if len(body) > 0 && json.Unmarshal(body, &result) == nil {
		for _, m := range result.Data {
			if strings.EqualFold(strings.TrimSpace(m.ID), strings.TrimSpace(preferred)) {
				return m.ID
			}
		}
	}
	return firstOpenAIModelFromListJSON(body, fallback)
}

func openAIProbeHTTPError(statusCode int, body []byte, method, url string, cfg *BackendConfig) error {
	if statusCode == http.StatusUnauthorized {
		return fmt.Errorf("鉴权失败（无效 API Key），请求: %s %s 响应: %q", method, url, truncateErrBody(body, 600))
	}
	if statusCode == 402 {
		return kimiMembershipOrPaymentError(cfg, url, body)
	}
	return nil
}

func kimiMembershipOrPaymentError(cfg *BackendConfig, url string, body []byte) error {
	bodyStr := truncateErrBody(body, 400)
	base := strings.ToLower(cfg.BaseURL)
	if strings.Contains(base, "api.kimi.com") || strings.Contains(strings.ToLower(bodyStr), "membership") {
		return fmt.Errorf(
			"Kimi for Coding 会员权益未通过验证（HTTP 402）。API Key 已送达服务端，但当前账户无法使用 coding 接口。请确认："+
				"1) 已在 kimi.com/code 开通有效的 Kimi Code 会员；"+
				"2) 使用的是 Kimi Code 专用 Key（在 Kimi Code 设置页创建），不要用 Moonshot 开放平台 api.moonshot.cn 的密钥；"+
				"3) 若只需通用 Kimi API，请将 Base URL 改为 https://api.moonshot.cn/v1 并配置模型如 kimi-k2.5。请求: %s 响应: %q",
			url, bodyStr,
		)
	}
	return fmt.Errorf("账户权益或余额不足（HTTP 402），请求: %s 响应: %q", url, bodyStr)
}

func openAIProbeChat(ctx context.Context, client *httpclient.Client, root, model string, headers map[string]string) (int, []byte, error) {
	chatURL := root + "/chat/completions"
	chatBody := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": "hi"},
		},
		"max_tokens": 4,
	}
	bodyBytes, err := json.Marshal(chatBody)
	if err != nil {
		return 0, nil, err
	}
	resp, err := client.Do(ctx, &httpclient.RequestConfig{
		Method:  "POST",
		URL:     chatURL,
		Headers: headers,
		Body:    bodyBytes,
	})
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, resp.Body, nil
}

// testOpenAIConnection 测试 OpenAI 兼容后端：多候选根路径；无 GET /models 时仅用 chat/completions 探测。
// 只要模型列表获取成功，连接测试即为通过。对话探测可选（可能因余额、模型限制等临时问题失败）。
func testOpenAIConnection(client *httpclient.Client, cfg *BackendConfig, headers map[string]string) error {
	return testOpenAIConnectionWithContext(context.Background(), client, cfg, headers)
}

func testOpenAIConnectionWithContext(ctx context.Context, client *httpclient.Client, cfg *BackendConfig, headers map[string]string) error {
	roots := CandidateOpenAIAPIRoots(cfg.BaseURL)
	if len(roots) == 0 {
		return fmt.Errorf("base URL 为空")
	}
	testModel := pickOpenAITestModel(cfg)
	var notes []string
	for _, root := range roots {
		modelsURL := root + "/models"
		logger.Info("OpenAI test: GET models", logger.GetField("url", modelsURL))

		resp, err := client.Do(ctx, &httpclient.RequestConfig{
			Method:  "GET",
			URL:     modelsURL,
			Headers: headers,
		})
		if err != nil {
			notes = append(notes, fmt.Sprintf("%s: %v", modelsURL, err))
			continue
		}
		switch {
		case resp.StatusCode == http.StatusUnauthorized:
			return fmt.Errorf("鉴权失败（无效 API Key），请求: %s 响应: %q", modelsURL, truncateErrBody(resp.Body, 600))
		case resp.StatusCode == 402:
			return kimiMembershipOrPaymentError(cfg, modelsURL, resp.Body)
		case resp.StatusCode >= 200 && resp.StatusCode < 300:
			// 模型列表获取成功，连接测试通过
			// 对话探测可选（可能因余额、模型限制等临时问题失败）
			useModel := preferOpenAIModelFromListJSON(resp.Body, cfg.ProbeModel, testModel)
			code, body, err := openAIProbeChat(ctx, client, root, useModel, headers)
			if err != nil {
				logger.Warn("OpenAI 兼容连接：模型列表获取成功，但对话探测失败",
					logger.GetField("name", cfg.Name),
					logger.GetField("root", root),
					logger.GetField("error", err.Error()))
			} else if code >= 200 && code < 300 {
				logger.Info("OpenAI-compatible connection test successful (models + chat)",
					logger.GetField("name", cfg.Name),
					logger.GetField("root", root))
			} else {
				logger.Info("OpenAI 兼容连接：模型列表获取成功，对话探测非 2xx",
					logger.GetField("name", cfg.Name),
					logger.GetField("root", root),
					logger.GetField("http_code", code),
					logger.GetField("response", truncateErrBody(body, 200)))
			}
			// 只要模型列表成功，连接测试即为通过
			return nil
		case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed:
			notes = append(notes, fmt.Sprintf("GET %s -> %d（跳过模型列表，尝试对话探测）", modelsURL, resp.StatusCode))
			continue
		default:
			notes = append(notes, fmt.Sprintf("GET %s -> HTTP %d: %q", modelsURL, resp.StatusCode, truncateErrBody(resp.Body, 300)))
			continue
		}
	}

	for _, root := range roots {
		chatURL := root + "/chat/completions"
		code, body, err := openAIProbeChat(ctx, client, root, testModel, headers)
		if err != nil {
			notes = append(notes, fmt.Sprintf("POST %s: %v", chatURL, err))
			continue
		}
		if err := openAIProbeHTTPError(code, body, "POST", chatURL, cfg); err != nil {
			return err
		}
		if code >= 200 && code < 300 {
			logger.Info("OpenAI-compatible connection test successful (chat-only)", logger.GetField("name", cfg.Name), logger.GetField("root", root))
			return nil
		}
		notes = append(notes, fmt.Sprintf("POST %s -> HTTP %d: %q", chatURL, code, truncateErrBody(body, 400)))
	}

	return fmt.Errorf("OpenAI 兼容连接失败（已尝试根路径: %s）。细节：%s。请核对 Base URL（例如 PPIO 常见为 …/openai/v1）、API Key 与 supported_models 中的模型名",
		strings.Join(roots, ", "), strings.Join(notes, " | "))
}

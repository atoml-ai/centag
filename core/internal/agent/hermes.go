package agent

import "fmt"

// HermesTemplate Hermes Agent 配置模板
type HermesTemplate struct{}

func (t *HermesTemplate) AgentType() AgentType { return AgentHermes }
func (t *HermesTemplate) DisplayName() string  { return "Hermes Agent" }
func (t *HermesTemplate) Description() string  { return "面向任务的 AI Agent 框架" }

func (t *HermesTemplate) ConfigFiles(info *BackendInfo) ([]ConfigFile, error) {
	url := proxyURL(info.Host, info.Port)
	model := defaultModel(info)
	content := fmt.Sprintf(`model:
  default: "%s"
  provider: "centag"
  base_url: "%s"

custom_providers:
  - name: centag
    base_url: "%s"
    api_key: "%s"
    api_mode: "chat_completions"
    model: "%s"
`, model, url, url, info.APIKey, model)
	return []ConfigFile{
		{Path: "~/.hermes/config.yaml", Content: content},
	}, nil
}

func (t *HermesTemplate) SetupCommand(info *BackendInfo) string {
	url := proxyURL(info.Host, info.Port)
	model := defaultModel(info)
	return fmt.Sprintf(`# Hermes Agent 一键配置
mkdir -p ~/.hermes
cat >> ~/.hermes/config.yaml << 'EOF'
model:
  default: "%s"
  provider: "centag"
  base_url: "%s"
custom_providers:
  - name: centag
    base_url: "%s"
    api_key: "%s"
    api_mode: "chat_completions"
    model: "%s"
EOF
`, model, url, url, info.APIKey, model)
}

func (t *HermesTemplate) PlatformCommands(info *BackendInfo) PlatformCommands {
	files, _ := t.ConfigFiles(info)
	return PlatformCommands{
		MacOS:   "#!/bin/bash\n" + generateShellCommands(files),
		Linux:   "#!/bin/bash\n" + generateShellCommands(files),
		Windows: generatePowerShellCommands(files),
	}
}

func (t *HermesTemplate) VerifyCommand(info *BackendInfo) string {
	return `hermes "Hello, can you hear me?"`
}

func (t *HermesTemplate) Steps(info *BackendInfo) []ConfigStep {
	url := proxyURL(info.Host, info.Port)
	return []ConfigStep{
		{Title: "编辑 config.yaml", Description: fmt.Sprintf("在 ~/.hermes/config.yaml 中添加 ProxyClaw: %s", url)},
		{Title: "启动 Hermes", Code: "hermes"},
	}
}

func (t *HermesTemplate) WriteConfig(info *BackendInfo) error {
	files, err := t.ConfigFiles(info)
	if err != nil {
		return err
	}
	return writeFiles(files)
}

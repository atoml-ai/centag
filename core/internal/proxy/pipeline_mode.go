package proxy

// ResolvePipelineID 根据模式字符串解析流水线 ID。
// Deprecated: 运行时应使用 ModeDispatcher + PipelineResolver；无注册表上下文时通常返回空。
func ResolvePipelineID(mode string) string {
	return NewPipelineResolver(nil, nil, nil).Resolve(ProxyMode(mode), "")
}
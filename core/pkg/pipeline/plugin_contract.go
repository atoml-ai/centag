package pipeline

import (
	"context"
	"fmt"
	"strings"
)

const (
	// PipelinePluginSchemaVersion is the first public contract version for node plugins.
	PipelinePluginSchemaVersion = "centag.pipeline.node/v1alpha1"

	BuiltinImplementationPrefix = "builtin."

	DefaultMaxInputBytes  int64 = 10 << 20 // 10 MiB
	DefaultMaxOutputBytes int64 = 10 << 20 // 10 MiB
)

// JSONSchema is intentionally kept as a generic map so plugins can expose
// draft-compatible schemas without coupling the core engine to a schema library.
type JSONSchema map[string]interface{}

// NodePluginDescriptor describes a pipeline node implementation that can be
// discovered by API clients and validated before execution.
type NodePluginDescriptor struct {
	Name               string                `json:"name"`
	Implementation     string                `json:"implementation"`
	Kind               string                `json:"kind"`
	Version            string                `json:"version"`
	Description        string                `json:"description,omitempty"`
	ConfigSchema       JSONSchema            `json:"config_schema,omitempty"`
	InputSchema        JSONSchema            `json:"input_schema,omitempty"`
	OutputSchema       JSONSchema            `json:"output_schema,omitempty"`
	Permissions        []string              `json:"permissions,omitempty"`
	SupportsStream     bool                  `json:"supports_stream"`
	Concurrent         bool                  `json:"concurrent"`
	Remote             *RemoteNodePluginSpec `json:"remote,omitempty"`
	APIVersion         string                `json:"api_version,omitempty"`
	MinProxyclawVersion string               `json:"min_proxyclaw_version,omitempty"`
	Deprecated         bool                  `json:"deprecated,omitempty"`
	Tags               []string             `json:"tags,omitempty"`
	// 哈希锁定：期望的 manifest SHA-256 哈希值
	ExpectedHash      string                `json:"expected_hash,omitempty"`
	// 签名：manifest 内容的 Ed25519 签名（base64 编码）
	Signature         string                `json:"signature,omitempty"`
}

type RemoteNodePluginSpec struct {
	BaseURL     string `json:"base_url,omitempty"`
	ManifestURL string `json:"manifest_url,omitempty"`
}

type NodePlugin interface {
	Descriptor() NodePluginDescriptor
	ValidateConfig(config NodeConfig) error
	Execute(ctx context.Context, req *NodeExecutionRequest) (*NodeExecutionResponse, error)
}

type NodeExecutionRequest struct {
	SchemaVersion      string                 `json:"schema_version"`
	PipelineID         string                 `json:"pipeline_id"`
	NodeID             string                 `json:"node_id"`
	NodeName           string                 `json:"node_name,omitempty"`
	NodeType           NodeType               `json:"node_type,omitempty"`
	Kind               string                 `json:"kind,omitempty"`
	Implementation     string                 `json:"implementation"`
	Config             NodeConfig             `json:"config"`
	Input              *NodeInput             `json:"input"`
	Context            map[string]interface{} `json:"context,omitempty"`
	CapabilityBroker   CapabilityBroker       `json:"-"`
	Secrets           map[string]string      `json:"-"` // 解析后的密钥，key=secret_key, value=secret_value
	TraceID            string                 `json:"trace_id,omitempty"`
	RequestID          string                 `json:"request_id,omitempty"`
	Deadline           int64                  `json:"deadline_nanos,omitempty"`
	MaxInputBytes      int64                  `json:"max_input_bytes,omitempty"`
	MaxOutputBytes     int64                  `json:"max_output_bytes,omitempty"`
}

type NodeExecutionResponse struct {
	Output *NodeOutput          `json:"output"`
	Events []NodeExecutionEvent `json:"events,omitempty"`
}

type NodeExecutionEvent struct {
	Type     string                 `json:"type"`
	Message  string                 `json:"message,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type NodeValidateResponse struct {
	Valid  bool              `json:"valid"`
	Code   string            `json:"code,omitempty"`
	Message string           `json:"message,omitempty"`
	Retryable bool           `json:"retryable,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
	Errors  []NodeValidateError `json:"errors,omitempty"`
}

type NodeValidateError struct {
	Code      string                 `json:"code,omitempty"`
	Message   string                 `json:"message"`
	Field     string                 `json:"field,omitempty"`
	Retryable bool                   `json:"retryable,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

func BuiltinImplementationForType(nodeType NodeType) string {
	if nodeType == "" {
		return ""
	}
	return BuiltinImplementationPrefix + nodeType.String()
}

func NormalizeImplementation(implementation string) string {
	return strings.TrimSpace(implementation)
}

func IsRemoteImplementation(implementation string) bool {
	impl := strings.ToLower(strings.TrimSpace(implementation))
	return strings.HasPrefix(impl, "http://") || strings.HasPrefix(impl, "https://")
}

func KindForBuiltinType(nodeType NodeType) string {
	switch nodeType {
	case NodeTypeGenerator:
		return "llm.generate"
	case NodeTypeProcessor:
		return "content.transform"
	case NodeTypeReviewer:
		return "quality.review"
	case NodeTypeRouter:
		return "route.decide"
	case NodeTypeAggregator:
		return "aggregate.merge"
	case NodeTypeMemory:
		return "memory.query"
	case NodeTypeAudit:
		return "audit.safety"
	case NodeTypeOptimize:
		return "optimize.enhance"
	case NodeTypeCache:
		return "cache.access"
	case NodeTypeTokenUsage:
		return "metrics.token_usage"
	case NodeTypeScheduler:
		return "scheduling.decide"
	case NodeTypeTransparentForward:
		return "proxy.transparent_forward"
	case NodeTypeToolCallInjector:
		return "inject.tool_call"
	default:
		return ""
	}
}

func NewBuiltinNodePlugin(nodeType NodeType, factory NodeFactory, descriptor NodePluginDescriptor) (NodePlugin, error) {
	if !nodeType.IsValid() {
		return nil, fmt.Errorf("invalid builtin node type: %s", nodeType)
	}
	if factory == nil {
		return nil, fmt.Errorf("builtin node factory cannot be nil")
	}
	if descriptor.Implementation == "" {
		descriptor.Implementation = BuiltinImplementationForType(nodeType)
	}
	if descriptor.Kind == "" {
		descriptor.Kind = KindForBuiltinType(nodeType)
	}
	if descriptor.Version == "" {
		descriptor.Version = "1.0.0"
	}
	if descriptor.APIVersion == "" {
		descriptor.APIVersion = PipelinePluginSchemaVersion
	}
	descriptor.Concurrent = true
	return &builtinNodePlugin{
		nodeType:   nodeType,
		factory:    factory,
		descriptor: descriptor,
	}, nil
}

type builtinNodePlugin struct {
	nodeType   NodeType
	factory    NodeFactory
	descriptor NodePluginDescriptor
}

func (p *builtinNodePlugin) Descriptor() NodePluginDescriptor {
	return p.descriptor
}

func (p *builtinNodePlugin) ValidateConfig(config NodeConfig) error {
	node, err := p.factory(config)
	if err != nil {
		return err
	}
	return node.Validate()
}

func (p *builtinNodePlugin) Execute(ctx context.Context, req *NodeExecutionRequest) (*NodeExecutionResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("node execution request cannot be nil")
	}
	node, err := p.factory(req.Config)
	if err != nil {
		return nil, err
	}
	if setter, ok := node.(interface{ SetID(string) }); ok {
		setter.SetID(req.NodeID)
	}
	if setter, ok := node.(interface{ SetName(string) }); ok {
		setter.SetName(req.NodeName)
	}
	if setter, ok := node.(interface{ SetType(NodeType) }); ok {
		setter.SetType(p.nodeType)
	}
	if req.CapabilityBroker != nil {
		if setter, ok := node.(interface{ SetCapabilityBroker(CapabilityBroker) }); ok {
			setter.SetCapabilityBroker(req.CapabilityBroker)
		}
	}
	output, err := node.Execute(ctx, req.Input)
	if err != nil {
		return nil, err
	}
	return &NodeExecutionResponse{Output: output}, nil
}

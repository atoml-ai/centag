package evolution

import (
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"
) // Omega 提案目标常量（工具名即契约的目标面；与任务计划 manifest 例一致）。
const (
	TargetPipelineWeight = "pipeline_weight"
	TargetRetryPolicy    = "retry_policy"
	TargetCacheTTL       = "cache_ttl"
	TargetBackendSwitch  = "backend_switch"
)

// targetValidators 每目标的参数 schema 校验（TC-EVO-002 越界拒绝）。
var targetValidators = map[string]func(map[string]any) error{
	TargetRetryPolicy: func(p map[string]any) error {
		v, ok := p["max_retries"]
		if !ok {
			return fmt.Errorf("retry_policy: 缺少 max_retries")
		}
		n, ok := v.(float64)
		if !ok {
			return fmt.Errorf("retry_policy: max_retries 必须为数值")
		}
		if n < 1 || n > 5 {
			return fmt.Errorf("retry_policy: max_retries %.0f 越界（1~5）", n)
		}
		return nil
	},
	TargetCacheTTL: func(p map[string]any) error {
		v, ok := p["seconds"]
		if !ok {
			return fmt.Errorf("cache_ttl: 缺少 seconds")
		}
		n, ok := v.(float64)
		if !ok {
			return fmt.Errorf("cache_ttl: seconds 必须为数值")
		}
		if n < 30 || n > 86400 {
			return fmt.Errorf("cache_ttl: seconds %.0f 越界（30~86400）", n)
		}
		return nil
	},
	TargetPipelineWeight: func(p map[string]any) error {
		w, ok := p["weights"].(map[string]any)
		if !ok || len(w) == 0 {
			return fmt.Errorf("pipeline_weight: 缺少 weights 对象")
		}
		sum := 0.0
		for k, v := range w {
			n, ok := v.(float64)
			if !ok {
				return fmt.Errorf("pipeline_weight: %s 必须为数值", k)
			}
			if n < 0 || n > 100 {
				return fmt.Errorf("pipeline_weight: %s %.0f 越界（0~100）", k, n)
			}
			sum += n
		}
		if sum > 100 {
			return fmt.Errorf("pipeline_weight: 权重合计 %.0f 越界（≤100）", sum)
		}
		return nil
	},
	TargetBackendSwitch: func(p map[string]any) error {
		b, ok := p["backend"].(string)
		if !ok || b == "" {
			return fmt.Errorf("backend_switch: 缺少 backend 标识")
		}
		return nil
	},
}

// Target 在目标清单内的判断。
func Target(targets []string, t string) bool {
	for _, x := range targets {
		if x == t {
			return true
		}
	}
	return false
}

// ValidateTarget 全目标白名单 + 参数 schema 校验。
func ValidateTarget(target string, params map[string]any) error {
	known := []string{TargetPipelineWeight, TargetRetryPolicy, TargetCacheTTL, TargetBackendSwitch}
	if !Target(known, target) {
		return fmt.Errorf("未知提案目标 %q", target)
	}
	if params == nil {
		return fmt.Errorf("提案参数不能为空")
	}
	fn, ok := targetValidators[target]
	if !ok {
		return fmt.Errorf("目标 %q 无参数校验器", target)
	}
	return fn(params)
}

// proposeSeq 进程内单调计数器，保证同一纳秒内的提案 ID 不冲突。
var proposeSeq atomic.Uint64

// proposeID 生成本地唯一提案 ID。
// 前缀为 UnixNano（时间有序，供按 id 的确定性排序兜底），后缀为进程内计数器
// （Windows 等粗时钟平台两次 UnixNano 可能相同，仅靠时间戳会 ID 冲突）。
func proposeID() string {
	return fmt.Sprintf("egno-%d-%08d", time.Now().UnixNano(), proposeSeq.Add(1))
}

// marshalProposal 提案 JSON 序列化。
func marshalProposal(p *Proposal) ([]byte, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("proposal 序列化: %w", err)
	}
	return b, nil
}

// unmarshalProposal 从日志行的 JSON blob 还原提案。
func unmarshalProposal(b []byte, dst *Proposal) error {
	if err := json.Unmarshal(b, dst); err != nil {
		return fmt.Errorf("proposal 反序列化: %w", err)
	}
	return nil
}

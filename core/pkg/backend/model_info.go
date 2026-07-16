package backend

import (
	"fmt"
	"log"
	"regexp"
	"strings"
)

var (
	// 模型名称正则表达式
	modelNameRegex = regexp.MustCompile(`^([a-z]+)[-/]?(\d+\.?\d*)([a-z0-9]*)[-:]?([0-9.]*[bkmb]*)`)
	
	// 模型参数量基准表（单位：B = billion parameters）
	modelCapacityReference = map[string]float64{
		// GPT 系列
		"gpt-4":          1750,
		"gpt-4-turbo":    1300,
		"gpt-3.5-turbo":  175,
		"gpt-3.5":        175,
		
		// Qwen 系列
		"qwen2.5:72b":    72,
		"qwen2.5:32b":    32,
		"qwen2.5:14b":    14,
		"qwen2.5:7b":     7,
		"qwen2.5:3b":     3,
		"qwen2.5:1.5b":   1.5,
		"qwen2.5:0.5b":   0.5,
		"qwen2.5":        72,
		"qwen2:72b":      72,
		"qwen2:7b":       7,
		"qwen2:1.5b":     1.5,
		"qwen2":          72,
		
		// Llama 系列
		"llama3.2:3b":    3,
		"llama3.2:1b":    1,
		"llama3.1:405b":  405,
		"llama3.1:70b":   70,
		"llama3.1:8b":    8,
		"llama3.1":       8,
		"llama3:70b":     70,
		"llama3:8b":      8,
		"llama3":         8,
		
		// Claude 系列
		"claude-3.5-sonnet": 175,
		"claude-3.5-opus":   400,
		"claude-3-opus":     400,
		"claude-3-sonnet":   200,
		
		// DeepSeek 系列
		"deepseek-r1:70b":  70,
		"deepseek-r1:32b":  32,
		"deepseek-r1:14b":  14,
		"deepseek-r1:8b":   8,
		"deepseek-r1:1.5b": 1.5,
		"deepseek-r1:1b":   1,
		"deepseek-r1:0.5b": 0.5,
		"deepseek-r1":      70,
		"deepseek-chat":    32,
		
		// Mistral 系列
		"mistral-large":   123,
		"mistral-medium":  35,
		"mistral-small":   7,
		
		// Gemma 系列
		"gemma-2:27b":     27,
		"gemma-2:9b":      9,
		"gemma-2:2b":      2,
		"gemma:7b":        7,
		"gemma:2b":        2,
		
		// Yi 系列
		"yi-1.5:34b":      34,
		"yi-1.5:9b":       9,
		"yi-1.5:6b":       6,
		"yi:34b":          34,
		"yi:6b":           6,
		
		// Baichuan 系列
		"baichuan2:13b":   13,
		"baichuan2:7b":    7,
		"baichuan:13b":    13,
		"baichuan:7b":     7,
		
		// InternLM 系列
		"internlm2:20b":   20,
		"internlm2:7b":    7,
		"internlm2:1.8b":  1.8,
		"internlm:20b":    20,
		"internlm:7b":     7,
	}
)

// ParseModelInfo 解析模型名称为结构化信息
func ParseModelInfo(modelName string) ModelInfo {
	// 标准化模型名
	original := modelName
	modelName = strings.ToLower(strings.TrimSpace(modelName))

	log.Printf("[ModelInfo] Parsing model: %s (original: %s)", modelName, original)

	// 移除特殊前缀和后缀
	modelName = strings.TrimPrefix(modelName, "text-")
	modelName = strings.TrimPrefix(modelName, "gpt-")
	modelName = strings.TrimPrefix(modelName, "v1/")

	// 使用正则表达式解析
	matches := modelNameRegex.FindStringSubmatch(modelName)

	info := ModelInfo{}

	if len(matches) >= 5 {
		info.Provider = matches[1]
		info.Family = matches[2]
		info.Variant = matches[3]

		log.Printf("[ModelInfo] Regex match: Provider=%s, Family=%s, Variant=%s, SizeRaw=%s",
			info.Provider, info.Family, info.Variant, matches[4])

		// 解析参数量
		if size := matches[4]; size != "" {
			// 统一参数量格式
			size = strings.ToLower(size)
			size = strings.TrimSuffix(size, "b")
			size = strings.TrimSuffix(size, "k")
			size = strings.TrimSuffix(size, "m")

			// 检查是否包含小数点
			if strings.Contains(size, ".") {
				parts := strings.Split(size, ".")
				if len(parts) == 2 {
					info.Size = parts[0] + "." + parts[1]
				}
			} else {
				info.Size = size
			}
			log.Printf("[ModelInfo] Parsed size: %s", info.Size)
		}
	}

	// 如果正则没有匹配到，尝试简单解析
	if info.Provider == "" {
		parts := strings.Split(modelName, "-")
		if len(parts) > 0 {
			info.Provider = parts[0]
		}
		if len(parts) > 1 {
			info.Family = parts[1]
		}
		log.Printf("[ModelInfo] Fallback parsing: Provider=%s, Family=%s", info.Provider, info.Family)
	}

	// 提取精度信息
	if strings.Contains(modelName, "int4") {
		info.Precision = "int4"
	} else if strings.Contains(modelName, "int8") {
		info.Precision = "int8"
	} else if strings.Contains(modelName, "fp16") {
		info.Precision = "fp16"
	} else if strings.Contains(modelName, "fp8") {
		info.Precision = "fp8"
	} else if strings.Contains(modelName, "fp32") {
		info.Precision = "fp32"
	}
	if info.Precision != "" {
		log.Printf("[ModelInfo] Detected precision: %s", info.Precision)
	}

	// 提取版本
	if strings.Contains(modelName, "v2") {
		info.Version = "v2"
	} else if strings.Contains(modelName, "v3") {
		info.Version = "v3"
	} else if strings.Contains(modelName, "v1") {
		info.Version = "v1"
	}
	if info.Version != "" {
		log.Printf("[ModelInfo] Detected version: %s", info.Version)
	}

	log.Printf("[ModelInfo] Parsed info: Provider=%s, Family=%s, Variant=%s, Size=%s, Precision=%s, Version=%s",
		info.Provider, info.Family, info.Variant, info.Size, info.Precision, info.Version)

	return info
}

// GetModelCapacity 获取模型参数量（单位：B）
func GetModelCapacity(modelName string) float64 {
	modelName = strings.ToLower(strings.TrimSpace(modelName))

	log.Printf("[ModelInfo] Getting capacity for: %s", modelName)

	// 直接查找
	if capacity, ok := modelCapacityReference[modelName]; ok {
		log.Printf("[ModelInfo] Found in reference table: %.2fB", capacity)
		return capacity
	}

	// 尝试不带版本后缀的名称
	if idx := strings.Index(modelName, ":"); idx > 0 {
		baseName := modelName[:idx]
		log.Printf("[ModelInfo] Trying base name: %s", baseName)
		if capacity, ok := modelCapacityReference[baseName]; ok {
			log.Printf("[ModelInfo] Found base name in reference: %.2fB", capacity)
			return capacity
		}
	}

	// 尝试解析参数量后缀
	info := ParseModelInfo(modelName)
	if info.Size != "" {
		log.Printf("[ModelInfo] Parsing size from info: %s", info.Size)
		// 简单的参数量解析
		var capacity float64
		if strings.Contains(info.Size, ".") {
			if _, err := fmt.Sscanf(info.Size, "%f", &capacity); err == nil && capacity > 0 {
				log.Printf("[ModelInfo] Parsed float capacity: %.2fB", capacity)
				return capacity
			}
		} else {
			var intCapacity int
			if _, err := fmt.Sscanf(info.Size, "%d", &intCapacity); err == nil && intCapacity > 0 {
				capacity = float64(intCapacity)
				log.Printf("[ModelInfo] Parsed int capacity: %.2fB", capacity)
				return capacity
			}
		}
	}

	log.Printf("[ModelInfo] Capacity not found for: %s", modelName)
	return 0
}

// GetModelFamily 获取模型族
func GetModelFamily(modelName string) string {
	info := ParseModelInfo(modelName)
	return info.Family
}

// IsSameFamily 判断两个模型是否属于同一族
func IsSameFamily(model1, model2 string) bool {
	family1 := GetModelFamily(model1)
	family2 := GetModelFamily(model2)
	result := family1 != "" && family1 == family2
	log.Printf("[ModelInfo] IsSameFamily: %s (family=%s) vs %s (family=%s) => %v",
		model1, family1, model2, family2, result)
	return result
}

// NormalizeModelName 规范化模型名称（用于比较）
func NormalizeModelName(modelName string) string {
	// 移除空格和特殊字符
	normalized := strings.ToLower(strings.TrimSpace(modelName))

	// 替换常见的别名映射
	aliases := map[string]string{
		"gpt-4-turbo-2024-04-09": "gpt-4-turbo",
		"gpt-4-0125-preview":      "gpt-4-turbo",
		"gpt-3.5-turbo-0125":      "gpt-3.5-turbo",
		"gpt-3.5-turbo-1106":      "gpt-3.5-turbo",
		"claude-3-5-sonnet":       "claude-3.5-sonnet",
		"claude-3-opus-20240229":  "claude-3-opus",
	}

	if alias, ok := aliases[normalized]; ok {
		log.Printf("[ModelInfo] Normalized with alias: %s -> %s", normalized, alias)
		normalized = alias
	}

	log.Printf("[ModelInfo] Normalized: %s", normalized)
	return normalized
}

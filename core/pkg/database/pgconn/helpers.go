package pgconn

import (
	"os"
	"strconv"
)

// envFirst 返回第一个非空的环境变量
// 支持多种命名约定（如 PG_HOST 和 POSTGRES_HOST）
func envFirst(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

// envIntFirst 返回第一个非空的环境变量的整数值
// 如果环境变量不存在或不是有效整数，返回默认值
func envIntFirst(defaultVal int, keys ...string) int {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			if i, err := strconv.Atoi(v); err == nil {
				return i
			}
		}
	}
	return defaultVal
}

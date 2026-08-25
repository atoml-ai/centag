package proxy

import (
	"centag/core/pkg/logger"
)

func init() {
	// 初始化日志系统，避免测试时panic
	logger.Init(logger.Config{
		Level:  "debug",
		Format: "console",
		Output: "stdout",
	})
}

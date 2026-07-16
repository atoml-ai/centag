package logger

import (
	"go.uber.org/zap"
)

// LogOperation 记录操作日志
func LogOperation(operation string, fields ...zap.Field) {
	Info(operation, fields...)
}

// LogError 记录操作错误
func LogError(operation string, err error, fields ...zap.Field) {
	allFields := append(fields, zap.Error(err))
	Error("Failed to "+operation, allFields...)
}

// LogOperationWithError 记录操作和错误
func LogOperationWithError(operation string, name string, err error) {
	if err != nil {
		Error("Failed to "+operation,
			zap.String("name", name),
			zap.Error(err))
	} else {
		Info(operation+" succeeded",
			zap.String("name", name))
	}
}

// GetField 创建日志字段的便捷函数
func GetField(key string, value interface{}) zap.Field {
	return zap.Any(key, value)
}

package pipeline

import (
	"centag/core/pkg/logger"

	"go.uber.org/zap"
)

// PipelineLogger 流水线日志实现，接入项目统一日志系统
type PipelineLogger struct{}

// NewPipelineLogger 创建流水线日志器
func NewPipelineLogger() Logger {
	return &PipelineLogger{}
}

func (l *PipelineLogger) Debug(msg string, fields ...interface{}) {
	if len(fields) > 0 {
		logger.Debug(msg, toZapFields(fields...)...)
	} else {
		logger.Debug(msg)
	}
}

func (l *PipelineLogger) Info(msg string, fields ...interface{}) {
	if len(fields) > 0 {
		logger.Info(msg, toZapFields(fields...)...)
	} else {
		logger.Info(msg)
	}
}

func (l *PipelineLogger) Warn(msg string, fields ...interface{}) {
	if len(fields) > 0 {
		logger.Warn(msg, toZapFields(fields...)...)
	} else {
		logger.Warn(msg)
	}
}

func (l *PipelineLogger) Error(msg string, fields ...interface{}) {
	if len(fields) > 0 {
		logger.Error(msg, toZapFields(fields...)...)
	} else {
		logger.Error(msg)
	}
}

func toZapFields(fields ...interface{}) []zap.Field {
	if len(fields) == 0 {
		return nil
	}

	zfields := make([]zap.Field, 0, len(fields)/2+1)
	for i := 0; i < len(fields); i += 2 {
		if i+1 >= len(fields) {
			zfields = append(zfields, zap.Any("extra", fields[i]))
			break
		}
		key, ok := fields[i].(string)
		if !ok || key == "" {
			key = "field"
		}
		zfields = append(zfields, zap.Any(key, fields[i+1]))
	}
	return zfields
}

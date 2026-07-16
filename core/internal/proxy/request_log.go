package proxy

import (
	"time"

	"centag/core/pkg/logger"
	"centag/core/pkg/utils"

	"go.uber.org/zap"
)

const defaultResponsePreviewMax = 4000

func requestLogFields(requestID string, fields ...zap.Field) []zap.Field {
	out := make([]zap.Field, 0, len(fields)+1)
	out = append(out, zap.String("request_id", requestID))
	out = append(out, fields...)
	return out
}

func logRequestInfo(requestID, msg string, fields ...zap.Field) {
	logger.Info(msg, requestLogFields(requestID, fields...)...)
}

func logRequestWarn(requestID, msg string, fields ...zap.Field) {
	logger.Warn(msg, requestLogFields(requestID, fields...)...)
}

func logRequestError(requestID, msg string, fields ...zap.Field) {
	logger.Error(msg, requestLogFields(requestID, fields...)...)
}

func logRequestComplete(requestID, model, backendID string, statusCode int, started time.Time, responsePreview string) {
	fields := []zap.Field{
		zap.String("model", model),
		zap.Int("status_code", statusCode),
		zap.Int64("duration_ms", time.Since(started).Milliseconds()),
	}
	if backendID != "" {
		fields = append(fields, zap.String("backend_id", backendID))
	}
	if preview := utils.TruncateString(responsePreview, defaultResponsePreviewMax); preview != "" {
		fields = append(fields, zap.String("response_preview", preview))
	}
	logRequestInfo(requestID, "[Request] completed", fields...)
}

func logRequestResponse(requestID, model, backendID string, statusCode int, responsePreview string) {
	fields := []zap.Field{
		zap.Int("status_code", statusCode),
	}
	if model != "" {
		fields = append(fields, zap.String("model", model))
	}
	if backendID != "" {
		fields = append(fields, zap.String("backend_id", backendID))
	}
	if preview := utils.TruncateString(responsePreview, defaultResponsePreviewMax); preview != "" {
		fields = append(fields, zap.String("response_preview", preview))
	}
	logRequestInfo(requestID, "[Response] details", fields...)
}

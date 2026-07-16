package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	Logger *zap.Logger
	Sugar  *zap.SugaredLogger
)

// Config 日志配置
type Config struct {
	Level  string
	Format string // json, console
	Output string // stdout, file
	Path   string
	File   FileConfig
}

type FileConfig struct {
	Filename   string
	MaxSize    int
	MaxBackups int
	MaxAge     int
	Compress   bool
}

// Init 初始化日志系统
func Init(cfg Config) error {
	// 解析日志级别
	level, err := parseLevel(cfg.Level)
	if err != nil {
		return err
	}

	// 创建编码器配置
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// 创建编码器
	var encoder zapcore.Encoder
	if cfg.Format == "json" {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	// 创建输出写入器
	var writer zapcore.WriteSyncer
	fileWriter := func() zapcore.WriteSyncer {
		return zapcore.AddSync(&lumberjack.Logger{
			Filename:   cfg.Path + "/" + cfg.File.Filename,
			MaxSize:    cfg.File.MaxSize,
			MaxBackups: cfg.File.MaxBackups,
			MaxAge:     cfg.File.MaxAge,
			Compress:   cfg.File.Compress,
		})
	}
	switch cfg.Output {
	case "file":
		// 仅写入文件，不写 stdout（避免 daemon 下 nohup 重定向造成日志重复）
		writer = fileWriter()
	case "both":
		// 同时写入文件和控制台
		writer = zapcore.NewMultiWriteSyncer(fileWriter(), zapcore.AddSync(os.Stdout))
	default:
		// stdout 模式（默认）
		writer = zapcore.AddSync(os.Stdout)
	}

	// 创建 Core
	core := zapcore.NewCore(encoder, writer, level)

	// 创建 Logger
	Logger = zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	Sugar = Logger.Sugar()

	return nil
}

// parseLevel 解析日志级别
func parseLevel(level string) (zapcore.Level, error) {
	switch level {
	case "debug":
		return zapcore.DebugLevel, nil
	case "info":
		return zapcore.InfoLevel, nil
	case "warn":
		return zapcore.WarnLevel, nil
	case "error":
		return zapcore.ErrorLevel, nil
	default:
		return zapcore.InfoLevel, nil
	}
}

// Sync 同步日志
func Sync() error {
	if Logger != nil {
		return Logger.Sync()
	}
	return nil
}

// Debug 日志方法
func Debug(msg string, fields ...zap.Field) {
	Logger.Debug(msg, fields...)
}

func Info(msg string, fields ...zap.Field) {
	Logger.Info(msg, fields...)
}

func Warn(msg string, fields ...zap.Field) {
	Logger.Warn(msg, fields...)
}

func Error(msg string, fields ...zap.Field) {
	Logger.Error(msg, fields...)
}

func Fatal(msg string, fields ...zap.Field) {
	Logger.Fatal(msg, fields...)
}

// Sugar 简化日志方法
func Debugf(template string, args ...interface{}) {
	Sugar.Debugf(template, args...)
}

func Infof(template string, args ...interface{}) {
	Sugar.Infof(template, args...)
}

func Warnf(template string, args ...interface{}) {
	Sugar.Warnf(template, args...)
}

func Errorf(template string, args ...interface{}) {
	Sugar.Errorf(template, args...)
}

func Fatalf(template string, args ...interface{}) {
	Sugar.Fatalf(template, args...)
}

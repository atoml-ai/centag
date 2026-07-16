package config

// 错误类型定义

// ConfigError 配置错误
type ConfigError struct {
	Message string
	Err     error
}

func (e *ConfigError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *ConfigError) Unwrap() error {
	return e.Err
}

// NewConfigError 创建配置错误
func NewConfigError(message string, err error) *ConfigError {
	return &ConfigError{
		Message: message,
		Err:     err,
	}
}

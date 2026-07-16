package internal

var (
	// Version 版本号（构建时通过 ldflags 注入）
	Version = "dev"
	// BuildTime 构建时间（构建时通过 ldflags 注入）
	BuildTime = "unknown"
)

// SetBuildInfo 设置构建信息
func SetBuildInfo(version, buildTime string) {
	if version != "" {
		Version = version
	}
	if buildTime != "" {
		BuildTime = buildTime
	}
}

// GetVersion 获取版本号
func GetVersion() string {
	return Version
}

// GetBuildTime 获取构建时间
func GetBuildTime() string {
	return BuildTime
}

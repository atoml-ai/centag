// 部署级配置（fnOS 等安装包）的落盘实现。
//
// 与保存在数据库中的运行配置不同，部署级配置写入 ${CENTAG_DATA_DIR}/centag.conf：
//   - fnOS 启动脚本 cmd/main 的 load_app_config() 读取 db_driver / pg_* 并注入
//     环境变量（LLM_PROXY_DB_DRIVER / POSTGRES_*），决定二进制以 SQLite 还是
//     PostgreSQL 启动；
//   - fnOS uninstall_callback 读取 clean_data_on_uninstall，决定卸载时是否清除数据。
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// deploymentConfigFile 是部署级配置文件的相对文件名（位于数据目录下）。
// 注意：文件名必须与 deploy/fnos/native/cmd/{main,uninstall_callback} 保持一致。
const deploymentConfigFile = "centag.conf"

// LoadDeploymentConfig 从数据目录读取部署级配置。文件不存在或无法解析时返回默认值，
// 保证首次安装 / 普通服务环境使用 SQLite 且卸载保留数据。
func LoadDeploymentConfig() DeploymentConfig {
	def := DefaultDeploymentConfig()
	dataDir := ResolveDataDir()
	if dataDir == "" {
		return def
	}
	data, err := os.ReadFile(filepath.Join(dataDir, deploymentConfigFile))
	if err != nil {
		return def
	}
	var dep DeploymentConfig
	if err := json.Unmarshal(data, &dep); err != nil {
		return def
	}
	// 兜底默认值：旧文件可能缺少新增字段
	if dep.DBDriver == "" {
		dep.DBDriver = def.DBDriver
	}
	if dep.PGHost == "" {
		dep.PGHost = def.PGHost
	}
	if dep.PGPort == "" {
		dep.PGPort = def.PGPort
	}
	if dep.PGUser == "" {
		dep.PGUser = def.PGUser
	}
	if dep.PGDB == "" {
		dep.PGDB = def.PGDB
	}
	return dep
}

// SaveDeploymentConfig 将部署级配置以单行紧凑 JSON 写入数据目录下的 centag.conf。
// 单行格式兼容 fnOS 脚本中基于 grep/sed 的简单 JSON 解析。
func SaveDeploymentConfig(dep DeploymentConfig) error {
	dataDir := ResolveDataDir()
	if dataDir == "" {
		return fmt.Errorf("deployment config: data directory is empty")
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("deployment config: create data dir %s: %w", dataDir, err)
	}
	data, err := json.Marshal(dep)
	if err != nil {
		return fmt.Errorf("deployment config: marshal: %w", err)
	}
	path := filepath.Join(dataDir, deploymentConfigFile)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("deployment config: write %s: %w", path, err)
	}
	return nil
}

// UpdateDeploymentConfig 更新全局运行配置中的 Deployment 字段（不改写 DB）。
// 由 ConfigHandler 在 PUT /api/v1/config 保存部署设置后调用，保证后续 GET 返回新值。
func UpdateDeploymentConfig(dep DeploymentConfig) {
	mu.Lock()
	defer mu.Unlock()
	if globalConfig != nil {
		globalConfig.Deployment = dep
	}
}

package pgconn

import (
	"strconv"
	"time"
)

// Config PostgreSQL 连接配置
type Config struct {
	// 连接配置
	Host     string `yaml:"host" json:"host"`
	Port     int    `yaml:"port" json:"port"`
	User     string `yaml:"user" json:"user"`
	Password string `yaml:"password" json:"password"`
	Database string `yaml:"database" json:"database"`
	SSLMode  string `yaml:"ssl_mode" json:"ssl_mode"`

	// 连接池配置（database/sql）
	MaxOpenConns    int           `yaml:"max_open_conns" json:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns" json:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" json:"conn_max_lifetime"`

	// 连接池配置（pgxpool）
	MaxConnLifetime int `yaml:"max_conn_lifetime" json:"max_conn_lifetime"` // 秒
	MaxConnIdleTime int `yaml:"max_conn_idle_time" json:"max_conn_idle_time"` // 秒
	MaxConns        int `yaml:"max_conns" json:"max_conns"`
	MinConns        int `yaml:"min_conns" json:"min_conns"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Host:            "localhost",
		Port:            5432,
		User:            "postgres",
		Password:        "",
		Database:        "centag",
		SSLMode:         "disable",
		MaxOpenConns:    20,
		MaxIdleConns:    5,
		ConnMaxLifetime: time.Hour,
		MaxConnLifetime: 3600,
		MaxConnIdleTime: 600,
		MaxConns:        20,
		MinConns:        5,
	}
}

// DSN 构建 PostgreSQL 连接字符串
func (c *Config) DSN() string {
	return "host=" + c.Host +
		" port=" + strconv.Itoa(c.Port) +
		" user=" + c.User +
		" password=" + c.Password +
		" dbname=" + c.Database +
		" sslmode=" + c.SSLMode +
		" connect_timeout=5"
}

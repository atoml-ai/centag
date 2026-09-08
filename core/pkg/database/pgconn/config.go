package pgconn

import (
	"net"
	"net/url"
	"strconv"
	"strings"
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

// DSN 构建 PostgreSQL 连接字符串。
// 统一使用 URL 形态并对 user/password 做转义：关键字形态直接拼接会导致
// 密码含空格/引号/特殊字符时连接解析失败（产品发布后的真实用户必踩）。
// Host 以 "/" 开头时视为 unix socket 目录（pgx 支持 host=<dir>）。
func (c *Config) DSN() string {
	u := url.URL{Scheme: "postgres"}
	if c.Host != "" && strings.HasPrefix(c.Host, "/") {
		// unix socket 目录：Path 为数据库名
		u.Path = "/" + c.Database
		q := u.Query()
		q.Set("host", c.Host)
		q.Set("port", strconv.Itoa(c.Port))
		q.Set("sslmode", c.SSLMode)
		q.Set("connect_timeout", "5")
		u.RawQuery = q.Encode()
		u.User = url.UserPassword(c.User, c.Password)
		return u.String()
	}
	u.Host = net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
	u.Path = "/" + c.Database
	q := u.Query()
	q.Set("sslmode", c.SSLMode)
	q.Set("connect_timeout", "5")
	u.RawQuery = q.Encode()
	u.User = url.UserPassword(c.User, c.Password)
	return u.String()
}

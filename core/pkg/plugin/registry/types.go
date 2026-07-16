package registry

import (
	"time"
)

// PluginMetadata 插件元数据
type PluginMetadata struct {
	// 基本信息
	ID          string    `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Version     string    `json:"version" db:"version"`
	Description string    `json:"description" db:"description"`
	Author      string    `json:"author" db:"author"`
	Email       string    `json:"email" db:"email"`
	URL         string    `json:"url" db:"url"`
	
	// 分类和标签
	Category string   `json:"category" db:"category"`
	Tags     []string `json:"tags" db:"tags"`
	
	// 权限声明
	Permissions []string `json:"permissions" db:"permissions"`
	
	// 依赖关系
	Dependencies []Dependency `json:"dependencies" db:"dependencies"`
	
	// 下载信息
	DownloadURL string `json:"download_url" db:"download_url"`
	Checksum    string `json:"checksum" db:"checksum"`
	Signature   string `json:"signature" db:"signature"`
	Size        int64  `json:"size" db:"size"`
	
	// 统计信息
	DownloadCount int       `json:"download_count" db:"download_count"`
	Rating        float64   `json:"rating" db:"rating"`
	RatingCount   int       `json:"rating_count" db:"rating_count"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

// Dependency 依赖项
type Dependency struct {
	ID      string `json:"id" db:"id"`
	Version string `json:"version" db:"version"`
	Optional bool  `json:"optional" db:"optional"`
}

// PluginRating 插件评分
type PluginRating struct {
	ID        string    `json:"id" db:"id"`
	PluginID  string    `json:"plugin_id" db:"plugin_id"`
	UserID    string    `json:"user_id" db:"user_id"`
	Score     int       `json:"score" db:"score"` // 1-5
	Comment   string    `json:"comment" db:"comment"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// ListPluginsRequest 列出插件请求
type ListPluginsRequest struct {
	Category   string   `json:"category" query:"category"`
	Tags       []string `json:"tags" query:"tags"`
	Author     string   `json:"author" query:"author"`
	Search     string   `json:"search" query:"search"`
	SortBy     string   `json:"sort_by" query:"sort_by"` // name, download_count, rating, created_at
	SortOrder  string   `json:"sort_order" query:"sort_order"` // asc, desc
	Page       int      `json:"page" query:"page"`
	PageSize   int      `json:"page_size" query:"page_size"`
}

// ListPluginsResponse 列出插件响应
type ListPluginsResponse struct {
	Plugins    []PluginMetadata `json:"plugins"`
	Total      int              `json:"total"`
	Page       int              `json:"page"`
	PageSize   int              `json:"page_size"`
	TotalPages int              `json:"total_pages"`
}

// RegisterPluginRequest 注册插件请求
type RegisterPluginRequest struct {
	Name         string       `json:"name" binding:"required"`
	Version      string       `json:"version" binding:"required"`
	Description  string       `json:"description"`
	Author       string       `json:"author"`
	Email        string       `json:"email"`
	URL          string       `json:"url"`
	Category     string       `json:"category"`
	Tags         []string     `json:"tags"`
	Permissions  []string     `json:"permissions"`
	Dependencies []Dependency `json:"dependencies"`
	DownloadURL  string       `json:"download_url" binding:"required"`
	Checksum     string       `json:"checksum" binding:"required"`
	Signature    string       `json:"signature"`
	Size         int64        `json:"size"`
}

// RegisterPluginResponse 注册插件响应
type RegisterPluginResponse struct {
	ID      string `json:"id"`
	Message string `json:"message"`
}

// RatePluginRequest 评分插件请求
type RatePluginRequest struct {
	Score   int    `json:"score" binding:"required,min=1,max=5"`
	Comment string `json:"comment"`
}

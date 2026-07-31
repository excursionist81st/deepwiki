package model

import "time"

type Repository struct {
	ID           uint   `gorm:"primaryKey"`
	Name         string `gorm:"size:255;not null"`
	URL          string `gorm:"size:500;not null"`
	LocalPath    string `gorm:"size:500;not null"`
	Status       string `gorm:"size:20;default:'pending'"`
	HasEmbedding bool   `gorm:"default:false"` // 标记是否已向量化
	CreatedAt    time.Time
}

type IngestTask struct {
	ID           string `gorm:"primaryKey;size:100"`
	RepoID       uint   `gorm:"index"`
	Status       string `gorm:"size:20;default:'pending'"`
	Progress     int    `gorm:"default:0"`
	ErrorMessage string `gorm:"type:text"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type CodeChunk struct {
	ID        uint   `gorm:"primaryKey"`
	RepoID    uint   `gorm:"index"`
	FilePath  string `gorm:"size:500"`
	StartLine int
	EndLine   int
	Language  string `gorm:"size:50"`

	SymbolName   string `gorm:"size:255"`  // 函数/方法名
	SymbolType   string `gorm:"size:50"`   // function/method/interface/struct
	ParentSymbol string `gorm:"size:255"`  // 父级结构体名
	ParentDef    string `gorm:"type:text"` // 父级结构体定义
	Signature    string `gorm:"type:text"` // 函数签名

	Content         string `gorm:"type:text"`
	Embedding       string `gorm:"type:text"`                 // JSON 存储向量
	EmbeddingError  string `gorm:"type:text"`                 // 向量化失败原因
	EmbeddingStatus string `gorm:"size:20;default:'pending'"` // pending/success/failed
}

type IngestRequest struct {
	RepoURL      string   `json:"repo_url" binding:"required"`
	RepoName     string   `json:"repo_name" binding:"required"`
	IncludeExts  []string `json:"include_exts,omitempty"`
	IncludeFiles []string `json:"include_files,omitempty"`
	ExcludeDirs  []string `json:"exclude_dirs,omitempty"`
	ExcludeExts  []string `json:"exclude_exts,omitempty"`
}

type IngestResponse struct {
	TaskID  string `json:"task_id"`
	Message string `json:"message"`
}

type AskRequest struct {
	RepoName       string `json:"repo_name" binding:"required"`
	Question       string `json:"question" binding:"required"`
	ApiKey         string `json:"api_key,omitempty"`
	BaseURL        string `json:"base_url,omitempty"`
	AlibabaAPIKey  string `json:"alibaba_api_key,omitempty"`
	AlibabaBaseURL string `json:"alibaba_base_url,omitempty"`
}

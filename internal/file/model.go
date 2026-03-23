package file

import (
	"time"
)

type File struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	FileName    string    `gorm:"size:255" json:"file_name"`
	FilePath    string    `gorm:"size:500" json:"file_path"`
	FileSize    int64     `json:"file_size"`
	MimeType    string    `gorm:"size:100" json:"mime_type"`
	Hash        string    `gorm:"size:64" json:"hash"`
	UploaderID  uint      `json:"uploader_id"`
	Status      int       `gorm:"default:1" json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

func (File) TableName() string { return "files" }

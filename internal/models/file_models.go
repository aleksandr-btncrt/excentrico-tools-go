package models

// FileWithPath represents a file from Google Drive with its folder path information
type FileWithPath struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	MimeType     string `json:"mimeType"`
	Size         string `json:"size"`
	CreatedTime  string `json:"createdTime"`
	ModifiedTime string `json:"modifiedTime"`
	FolderPath   string `json:"folder_path"`
	FolderName   string `json:"folder_name"`
}

// WordPressMetadata represents metadata for WordPress posts
type WordPressMetadata struct {
	PostID    int    `json:"post_id"`
	Title     string `json:"title"`
	Slug      string `json:"slug"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}
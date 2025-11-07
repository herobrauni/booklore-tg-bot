package booklore

// BookdropFile represents a file in the bookdrop folder
type BookdropFile struct {
	ID          int64  `json:"id"`
	FileName    string `json:"fileName"`
	FilePath    string `json:"filePath"`
	FileSize    int64  `json:"fileSize"`
	Status      string `json:"status"`
	DateAdded   string `json:"dateAdded"`
	DateScanned string `json:"dateScanned"`
}

// BookdropFinalizeRequest represents a request to finalize bookdrop imports
type BookdropFinalizeRequest struct {
	FileIDs []int64 `json:"fileIds"`
}

// BookdropFinalizeResult represents the result of finalizing bookdrop imports
type BookdropFinalizeResult struct {
	Success       bool    `json:"success"`
	ImportedCount int     `json:"importedCount"`
	FailedCount   int     `json:"failedCount"`
	ImportedIDs   []int64 `json:"importedIds"`
	FailedIDs     []int64 `json:"failedIds"`
	Message       string  `json:"message"`
}

// BookdropNotification represents bookdrop notification summary
type BookdropNotification struct {
	TotalFiles     int `json:"totalFiles"`
	NewFiles       int `json:"newFiles"`
	ProcessedFiles int `json:"processedFiles"`
	ImportedFiles  int `json:"importedFiles"`
	FailedFiles    int `json:"failedFiles"`
}

// APIError represents an API error response
type APIError struct {
	Message   string `json:"message"`
	Status    int    `json:"status"`
	Path      string `json:"path"`
	Timestamp string `json:"timestamp"`
	Error     string `json:"error"`
}

// PageBookdropFile represents a paginated response of bookdrop files
type PageBookdropFile struct {
	Content       []BookdropFile `json:"content"`
	TotalElements int            `json:"totalElements"`
	TotalPages    int            `json:"totalPages"`
	Size          int            `json:"size"`
	Number        int            `json:"number"`
	First         bool           `json:"first"`
	Last          bool           `json:"last"`
}

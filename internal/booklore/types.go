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

// Library represents a Booklore library
type Library struct {
	ID                  int64      `json:"id"`
	Name                string     `json:"name"`
	Icon                string     `json:"icon"`
	Watch               bool       `json:"watch"`
	ScanMode            string     `json:"scanMode"`
	DefaultBookFormat   string     `json:"fileNamingPattern"`
	Paths               []LibraryPath `json:"paths"`
}

// LibraryPath represents a path within a library
type LibraryPath struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Parent int64  `json:"parent"`
}

// UserLibraryPreference stores user's selected library and path
type UserLibraryPreference struct {
	UserID     int64  `json:"userId"`
	LibraryID  int64  `json:"libraryId"`
	PathID     int64  `json:"pathId"`
	LibraryName string `json:"libraryName"`
	PathName    string `json:"pathName"`
}

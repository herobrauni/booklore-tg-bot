package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

type Config struct {
	BotToken         string
	AllowedUserIDs   []int64
	DownloadFolder   string
	AllowedFileTypes []string
	MaxFileSizeMB    int64
	Logger           *zap.Logger
	BookloreAPI      *BookloreConfig
}

type BookloreConfig struct {
	APIURL         string
	APIToken       string
	AutoImport     bool
	Enabled        bool
	RetryAttempts  int
	RetryDelay     int // in seconds
	DefaultLibraryID string
	DefaultPathID    string
}

func Load() (*Config, error) {
	// Load .env file if it exists (for local development)
	if err := godotenv.Load(); err != nil {
		// It's okay if .env file doesn't exist in production
		fmt.Printf("Warning: Could not load .env file: %v\n", err)
	}

	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	// Get required environment variables
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN environment variable is required")
	}

	// Validate bot token format (should be like "123456789:ABCdefGHIjklMNOpqrsTUVwxyz")
	if len(botToken) < 20 || !strings.Contains(botToken, ":") {
		return nil, fmt.Errorf("invalid bot token format - token should be in format 'BOT_ID:BOT_TOKEN'")
	}

	// Parse allowed user IDs
	allowedUsersStr := os.Getenv("ALLOWED_USER_IDS")
	if allowedUsersStr == "" {
		return nil, fmt.Errorf("ALLOWED_USER_IDS environment variable is required")
	}
	allowedUserIDs, err := parseUserIDs(allowedUsersStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ALLOWED_USER_IDS: %w", err)
	}

	// Get download folder (default to "downloads")
	downloadFolder := os.Getenv("DOWNLOAD_FOLDER")
	if downloadFolder == "" {
		downloadFolder = "downloads"
	}

	// Parse allowed file types (default to common document types)
	allowedFileTypesStr := os.Getenv("ALLOWED_FILE_TYPES")
	var allowedFileTypes []string
	if allowedFileTypesStr != "" {
		allowedFileTypes = strings.Split(allowedFileTypesStr, ",")
		for i, ft := range allowedFileTypes {
			allowedFileTypes[i] = strings.TrimSpace(strings.ToLower(ft))
		}
	} else {
		// Default allowed file types
		allowedFileTypes = []string{".pdf", ".doc", ".docx", ".txt", ".jpg", ".jpeg", ".png", ".zip", ".rar"}
	}

	// Parse max file size (default to 20MB)
	maxFileSizeStr := os.Getenv("MAX_FILE_SIZE_MB")
	maxFileSizeMB := int64(20) // default
	if maxFileSizeStr != "" {
		maxFileSizeMB, err = strconv.ParseInt(maxFileSizeStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse MAX_FILE_SIZE_MB: %w", err)
		}
	}

	// Create download folder if it doesn't exist
	if err := os.MkdirAll(downloadFolder, 0755); err != nil {
		return nil, fmt.Errorf("failed to create download folder: %w", err)
	}

	// Load Booklore API configuration
	bookloreConfig := loadBookloreConfig()

	return &Config{
		BotToken:         botToken,
		AllowedUserIDs:   allowedUserIDs,
		DownloadFolder:   downloadFolder,
		AllowedFileTypes: allowedFileTypes,
		MaxFileSizeMB:    maxFileSizeMB,
		Logger:           logger,
		BookloreAPI:      bookloreConfig,
	}, nil
}

func parseUserIDs(userIDsStr string) ([]int64, error) {
	var userIDs []int64
	parts := strings.Split(userIDsStr, ",")

	for _, part := range parts {
		userID, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid user ID '%s': %w", part, err)
		}
		userIDs = append(userIDs, userID)
	}

	return userIDs, nil
}

func loadBookloreConfig() *BookloreConfig {
	// Get Booklore API configuration from environment
	apiURL := os.Getenv("BOOKLORE_API_URL")
	if apiURL == "" {
		apiURL = "https://booklore.brauni.dev"
	}

	apiToken := os.Getenv("BOOKLORE_API_TOKEN")
	autoImportStr := os.Getenv("BOOKLORE_AUTO_IMPORT")
	retryAttemptsStr := os.Getenv("BOOKLORE_RETRY_ATTEMPTS")
	retryDelayStr := os.Getenv("BOOKLORE_RETRY_DELAY")

	// Only enable Booklore integration if API token is provided
	enabled := apiToken != ""

	// Parse auto-import setting (default to true if enabled)
	autoImport := enabled
	if autoImportStr != "" {
		autoImport = strings.ToLower(autoImportStr) == "true"
	}

	// Parse retry attempts (default to 3)
	retryAttempts := 3
	if retryAttemptsStr != "" {
		if attempts, err := strconv.Atoi(retryAttemptsStr); err == nil && attempts > 0 {
			retryAttempts = attempts
		}
	}

	// Parse retry delay (default to 3 seconds)
	retryDelay := 3
	if retryDelayStr != "" {
		if delay, err := strconv.Atoi(retryDelayStr); err == nil && delay > 0 {
			retryDelay = delay
		}
	}

	// Get library configuration
	defaultLibraryID := os.Getenv("BOOKLORE_DEFAULT_LIBRARY_ID")
	defaultPathID := os.Getenv("BOOKLORE_DEFAULT_PATH_ID")

	return &BookloreConfig{
		APIURL:          strings.TrimSuffix(apiURL, "/"),
		APIToken:        apiToken,
		AutoImport:      autoImport,
		Enabled:         enabled,
		RetryAttempts:   retryAttempts,
		RetryDelay:      retryDelay,
		DefaultLibraryID: defaultLibraryID,
		DefaultPathID:   defaultPathID,
	}
}

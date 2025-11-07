package downloader

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
)

type Downloader struct {
	downloadFolder   string
	allowedFileTypes []string
	maxFileSizeMB    int64
	logger           *zap.Logger
}

func NewDownloader(downloadFolder string, allowedFileTypes []string, maxFileSizeMB int64, logger *zap.Logger) *Downloader {
	return &Downloader{
		downloadFolder:   downloadFolder,
		allowedFileTypes: allowedFileTypes,
		maxFileSizeMB:    maxFileSizeMB,
		logger:           logger,
	}
}

func (d *Downloader) IsFileTypeAllowed(filename string) bool {
	if len(d.allowedFileTypes) == 0 {
		return true // No restrictions if no types specified
	}

	ext := strings.ToLower(filepath.Ext(filename))
	for _, allowedExt := range d.allowedFileTypes {
		if ext == allowedExt {
			return true
		}
	}

	d.logger.Info("File type not allowed",
		zap.String("filename", filename),
		zap.String("extension", ext),
		zap.Strings("allowed_extensions", d.allowedFileTypes))
	return false
}

func (d *Downloader) IsFileSizeAllowed(fileSize int64) bool {
	maxSizeBytes := d.maxFileSizeMB * 1024 * 1024
	if fileSize > maxSizeBytes {
		d.logger.Info("File size exceeds limit",
			zap.Int64("file_size", fileSize),
			zap.Int64("max_size", maxSizeBytes))
		return false
	}
	return true
}

func (d *Downloader) DownloadFile(fileURL, filename string) (string, error) {
	// Validate file type
	if !d.IsFileTypeAllowed(filename) {
		return "", fmt.Errorf("file type not allowed: %s", filename)
	}

	// Download the file
	resp, err := http.Get(fileURL)
	if err != nil {
		d.logger.Error("Failed to download file",
			zap.String("url", fileURL),
			zap.Error(err))
		return "", fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	// Check file size
	if resp.ContentLength > 0 && !d.IsFileSizeAllowed(resp.ContentLength) {
		return "", fmt.Errorf("file size %d bytes exceeds maximum allowed size %d MB",
			resp.ContentLength, d.maxFileSizeMB)
	}

	// Create the file path
	filePath := filepath.Join(d.downloadFolder, filename)

	// Ensure unique filename to avoid overwrites
	uniqueFilePath := d.getUniqueFilePath(filePath)

	// Create the file
	file, err := os.Create(uniqueFilePath)
	if err != nil {
		d.logger.Error("Failed to create file",
			zap.String("path", uniqueFilePath),
			zap.Error(err))
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Copy the file content
	bytesWritten, err := io.Copy(file, resp.Body)
	if err != nil {
		d.logger.Error("Failed to save file",
			zap.String("path", uniqueFilePath),
			zap.Error(err))
		return "", fmt.Errorf("failed to save file: %w", err)
	}

	// Final size check
	if !d.IsFileSizeAllowed(bytesWritten) {
		os.Remove(uniqueFilePath)
		return "", fmt.Errorf("downloaded file size %d bytes exceeds maximum allowed size %d MB",
			bytesWritten, d.maxFileSizeMB)
	}

	d.logger.Info("File downloaded successfully",
		zap.String("filename", filename),
		zap.String("path", uniqueFilePath),
		zap.Int64("size", bytesWritten))

	return uniqueFilePath, nil
}

func (d *Downloader) getUniqueFilePath(filePath string) string {
	// If file doesn't exist, return the path as-is
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return filePath
	}

	// If file exists, append a number to make it unique
	ext := filepath.Ext(filePath)
	base := strings.TrimSuffix(filePath, ext)
	counter := 1

	for {
		newPath := fmt.Sprintf("%s_%d%s", base, counter, ext)
		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			return newPath
		}
		counter++
	}
}

func (d *Downloader) GetDownloadFolder() string {
	return d.downloadFolder
}

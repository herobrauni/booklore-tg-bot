package booklore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// Client represents the Booklore API client
type Client struct {
	baseURL    string
	apiToken   string
	httpClient *http.Client
	logger     *zap.Logger
}

// NewClient creates a new Booklore API client
func NewClient(baseURL, apiToken string, logger *zap.Logger) *Client {
	return &Client{
		baseURL:  baseURL,
		apiToken: apiToken,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// IsEnabled returns true if the client is properly configured
func (c *Client) IsEnabled() bool {
	return c.baseURL != "" && c.apiToken != ""
}

// RescanBookdrop triggers a rescan of the bookdrop folder
func (c *Client) RescanBookdrop(ctx context.Context) error {
	if !c.IsEnabled() {
		return NewAPIError(ErrInvalidToken, "Booklore API client is not configured", 0)
	}

	url := fmt.Sprintf("%s/api/v1/bookdrop/rescan", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	c.setAuthHeader(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return NewNetworkError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.handleAPIError(resp)
	}

	c.logger.Info("Bookdrop folder rescanned successfully")
	return nil
}

// FinalizeImport finalizes the import of bookdrop files
func (c *Client) FinalizeImport(ctx context.Context, fileIDs []int64, libraryID, pathID string) (*BookdropFinalizeResult, error) {
	if !c.IsEnabled() {
		return nil, NewAPIError(ErrInvalidToken, "Booklore API client is not configured", 0)
	}

	// Add query parameters for library and path
	var url string
	if libraryID != "" {
		if pathID != "" {
			url = fmt.Sprintf("%s/api/v1/bookdrop/imports/finalize?defaultLibraryId=%s&defaultPathId=%s", c.baseURL, libraryID, pathID)
		} else {
			url = fmt.Sprintf("%s/api/v1/bookdrop/imports/finalize?defaultLibraryId=%s", c.baseURL, libraryID)
		}
	} else {
		url = fmt.Sprintf("%s/api/v1/bookdrop/imports/finalize", c.baseURL)
	}

	// Log the request details
	c.logger.Info("FinalizeImport request",
		zap.String("url", url),
		zap.Int("file_ids_count", len(fileIDs)),
		zap.Any("file_ids", fileIDs))

	request := BookdropFinalizeRequest{
		FileIDs: fileIDs,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Log the JSON payload
	c.logger.Info("FinalizeImport JSON payload",
		zap.String("json", string(jsonData)))

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	c.setAuthHeader(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, NewNetworkError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleAPIError(resp)
	}

	var result BookdropFinalizeResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	c.logger.Info("Bookdrop import finalized",
		zap.Int("imported_count", result.ImportedCount),
		zap.Int("failed_count", result.FailedCount))

	return &result, nil
}

// FinalizeAllImports finalizes all available bookdrop imports
func (c *Client) FinalizeAllImports(ctx context.Context, libraryID, pathID string) (*BookdropFinalizeResult, error) {
	if !c.IsEnabled() {
		return nil, NewAPIError(ErrInvalidToken, "Booklore API client is not configured", 0)
	}

	// Get all available files first
	files, err := c.GetBookdropFiles(ctx, "NEW", 0, 1000) // Get up to 1000 new files
	if err != nil {
		return nil, fmt.Errorf("failed to get bookdrop files: %w", err)
	}

	if len(files.Content) == 0 {
		// No files to import
		return &BookdropFinalizeResult{
			Success:       true,
			ImportedCount: 0,
			FailedCount:   0,
			Message:       "No files to import",
		}, nil
	}

	// Extract file IDs
	fileIDs := make([]int64, len(files.Content))
	for i, file := range files.Content {
		fileIDs[i] = file.ID
	}

	return c.FinalizeImport(ctx, fileIDs, libraryID, pathID)
}

// GetBookdropFiles retrieves bookdrop files by status
func (c *Client) GetBookdropFiles(ctx context.Context, status string, page, size int) (*PageBookdropFile, error) {
	if !c.IsEnabled() {
		return nil, NewAPIError(ErrInvalidToken, "Booklore API client is not configured", 0)
	}

	url := fmt.Sprintf("%s/api/v1/bookdrop/files?status=%s&page=%d&size=%d",
		c.baseURL, status, page, size)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setAuthHeader(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, NewNetworkError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleAPIError(resp)
	}

	var result PageBookdropFile
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetBookdropFilesNoStatus retrieves all bookdrop files without status filter
func (c *Client) GetBookdropFilesNoStatus(ctx context.Context, page, size int) (*PageBookdropFile, error) {
	if !c.IsEnabled() {
		return nil, NewAPIError(ErrInvalidToken, "Booklore API client is not configured", 0)
	}

	url := fmt.Sprintf("%s/api/v1/bookdrop/files?page=%d&size=%d",
		c.baseURL, page, size)

	c.logger.Info("Calling Booklore API for all files",
		zap.String("url", url),
		zap.Int("page", page),
		zap.Int("size", size))

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setAuthHeader(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, NewNetworkError(err)
	}
	defer resp.Body.Close()

	c.logger.Info("Booklore API response",
		zap.String("url", url),
		zap.Int("status_code", resp.StatusCode))

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleAPIError(resp)
	}

	var result PageBookdropFile
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	c.logger.Info("Bookdrop files decoded successfully",
		zap.String("url", url),
		zap.Int("total_elements", result.TotalElements),
		zap.Int("content_length", len(result.Content)))

	return &result, nil
}

// GetBookdropNotification gets bookdrop notification summary
func (c *Client) GetBookdropNotification(ctx context.Context) (*BookdropNotification, error) {
	if !c.IsEnabled() {
		return nil, NewAPIError(ErrInvalidToken, "Booklore API client is not configured", 0)
	}

	url := fmt.Sprintf("%s/api/v1/bookdrop/notification", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setAuthHeader(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, NewNetworkError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleAPIError(resp)
	}

	var result BookdropNotification
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// setAuthHeader sets the authorization header for API requests
func (c *Client) setAuthHeader(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
}

// handleAPIError processes API error responses
func (c *Client) handleAPIError(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return NewAPIError(ErrNetworkError, "Failed to read error response", resp.StatusCode)
	}

	var apiErr APIError
	if json.Unmarshal(body, &apiErr) == nil && apiErr.Message != "" {
		// We have a structured error response
		switch resp.StatusCode {
		case http.StatusUnauthorized:
			return NewInvalidTokenError()
		case http.StatusForbidden:
			return NewAPIError(ErrForbidden, apiErr.Message, resp.StatusCode)
		case http.StatusNotFound:
			return NewAPIError(ErrNotFound, apiErr.Message, resp.StatusCode)
		case http.StatusBadRequest:
			return NewAPIError(ErrBadRequest, apiErr.Message, resp.StatusCode)
		case http.StatusInternalServerError:
			return NewAPIError(ErrInternalServer, apiErr.Message, resp.StatusCode)
		case http.StatusServiceUnavailable:
			return NewAPIError(ErrServiceUnavailable, apiErr.Message, resp.StatusCode)
		default:
			return NewAPIError(ErrBadRequest, apiErr.Message, resp.StatusCode)
		}
	}

	// Fallback to generic error
	return NewAPIError(ErrBadRequest, fmt.Sprintf("API request failed with status %d: %s", resp.StatusCode, string(body)), resp.StatusCode)
}

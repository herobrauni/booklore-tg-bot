package booklore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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

	// Try different approaches to send file IDs
	// Approach 1: JSON body with fileIds field (current approach)
	result1, err1 := c.finalizeImportWithJSON(ctx, fileIDs, libraryID, pathID, "fileIds")

	// If first approach succeeds, return result
	if err1 == nil && result1 != nil && (result1.ImportedCount > 0 || result1.FailedCount > 0 || result1.Success) {
		c.logger.Info("JSON with 'fileIds' approach succeeded")
		return result1, nil
	}

	c.logger.Info("JSON with 'fileIds' approach failed, trying alternative field names")

	// Approach 1b: Try different field name
	result1b, err1b := c.finalizeImportWithJSON(ctx, fileIDs, libraryID, pathID, "ids")
	if err1b == nil && result1b != nil && (result1b.ImportedCount > 0 || result1b.FailedCount > 0 || result1b.Success) {
		c.logger.Info("JSON with 'ids' approach succeeded")
		return result1b, nil
	}

	c.logger.Info("Alternative JSON approach failed, trying string array approach")

	// Approach 1c: Try string array instead of int64 array
	result1c, err1c := c.finalizeImportWithJSONStringArray(ctx, fileIDs, libraryID, pathID, "fileIds")
	if err1c == nil && result1c != nil && (result1c.ImportedCount > 0 || result1c.FailedCount > 0 || result1c.Success) {
		c.logger.Info("JSON string array approach succeeded")
		return result1c, nil
	}

	c.logger.Info("String array approach failed, trying query parameter approach")

	// Approach 2: Query parameters with file IDs
	result2, err2 := c.finalizeImportWithQueryParams(ctx, fileIDs, libraryID, pathID)
	if err2 == nil && result2 != nil {
		c.logger.Info("Query parameter approach succeeded")
		return result2, nil
	}

	c.logger.Info("Query parameter approach failed, trying PUT method")

	// Approach 3: Try PUT method instead of POST
	result3, err3 := c.finalizeImportWithPUT(ctx, fileIDs, libraryID, pathID)
	if err3 == nil && result3 != nil {
		c.logger.Info("PUT method approach succeeded")
		return result3, nil
	}

	// If all approaches failed, return the error from the first approach
	c.logger.Error("All approaches failed",
		zap.Error(err1), zap.Error(err1b), zap.Error(err1c), zap.Error(err2), zap.Error(err3))
	return nil, err1
}

// finalizeImportWithJSON sends file IDs in JSON body
func (c *Client) finalizeImportWithJSON(ctx context.Context, fileIDs []int64, libraryID, pathID, fieldName string) (*BookdropFinalizeResult, error) {
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

	c.logger.Info("Trying JSON approach",
		zap.String("url", url),
		zap.Int("file_ids_count", len(fileIDs)),
		zap.Any("file_ids", fileIDs))

	// Create JSON with dynamic field name
	jsonData, err := json.Marshal(map[string]interface{}{
		fieldName: fileIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	c.logger.Info("JSON payload",
		zap.String("field_name", fieldName),
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

	c.logger.Info("JSON approach result",
		zap.Int("imported_count", result.ImportedCount),
		zap.Int("failed_count", result.FailedCount),
		zap.Bool("success", result.Success))

	return &result, nil
}

// finalizeImportWithJSONStringArray sends file IDs as string array in JSON body
func (c *Client) finalizeImportWithJSONStringArray(ctx context.Context, fileIDs []int64, libraryID, pathID, fieldName string) (*BookdropFinalizeResult, error) {
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

	// Convert int64 array to string array
	stringIDs := make([]string, len(fileIDs))
	for i, id := range fileIDs {
		stringIDs[i] = fmt.Sprintf("%d", id)
	}

	c.logger.Info("Trying JSON string array approach",
		zap.String("url", url),
		zap.Int("file_ids_count", len(fileIDs)),
		zap.Strings("string_ids", stringIDs))

	// Create JSON with string array
	jsonData, err := json.Marshal(map[string]interface{}{
		fieldName: stringIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	c.logger.Info("JSON string array payload",
		zap.String("field_name", fieldName),
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

	c.logger.Info("JSON string array approach result",
		zap.Int("imported_count", result.ImportedCount),
		zap.Int("failed_count", result.FailedCount),
		zap.Bool("success", result.Success))

	return &result, nil
}

// finalizeImportWithQueryParams sends file IDs as query parameters
func (c *Client) finalizeImportWithQueryParams(ctx context.Context, fileIDs []int64, libraryID, pathID string) (*BookdropFinalizeResult, error) {
	// Build URL with all parameters including file IDs
	var urlBuilder strings.Builder
	urlBuilder.WriteString(fmt.Sprintf("%s/api/v1/bookdrop/imports/finalize", c.baseURL))

	// Add first parameter
	paramsAdded := false

	if len(fileIDs) > 0 {
		urlBuilder.WriteString("?fileIds=")
		for i, id := range fileIDs {
			if i > 0 {
				urlBuilder.WriteString(",")
			}
			urlBuilder.WriteString(fmt.Sprintf("%d", id))
		}
		paramsAdded = true
	}

	if libraryID != "" {
		if paramsAdded {
			urlBuilder.WriteString("&defaultLibraryId=")
		} else {
			urlBuilder.WriteString("?defaultLibraryId=")
			paramsAdded = true
		}
		urlBuilder.WriteString(libraryID)
	}

	if pathID != "" {
		if paramsAdded {
			urlBuilder.WriteString("&defaultPathId=")
		} else {
			urlBuilder.WriteString("?defaultPathId=")
		}
		urlBuilder.WriteString(pathID)
	}

	url := urlBuilder.String()

	c.logger.Info("Trying query parameter approach",
		zap.String("url", url),
		zap.Int("file_ids_count", len(fileIDs)),
		zap.Any("file_ids", fileIDs))

	// Send empty JSON body
	emptyData := []byte("{}")
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(emptyData))
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

	c.logger.Info("Query parameter approach result",
		zap.Int("imported_count", result.ImportedCount),
		zap.Int("failed_count", result.FailedCount),
		zap.Bool("success", result.Success))

	return &result, nil
}

// finalizeImportWithNoBody sends only query parameters (maybe the API uses context)
func (c *Client) finalizeImportWithNoBody(ctx context.Context, fileIDs []int64, libraryID, pathID string) (*BookdropFinalizeResult, error) {
	// Build URL with just library and path parameters (no file IDs)
	var urlBuilder strings.Builder
	urlBuilder.WriteString(fmt.Sprintf("%s/api/v1/bookdrop/imports/finalize", c.baseURL))

	// Add first parameter
	paramsAdded := false

	if libraryID != "" {
		urlBuilder.WriteString("?defaultLibraryId=")
		urlBuilder.WriteString(libraryID)
		paramsAdded = true
	}

	if pathID != "" {
		if paramsAdded {
			urlBuilder.WriteString("&defaultPathId=")
		} else {
			urlBuilder.WriteString("?defaultPathId=")
		}
		urlBuilder.WriteString(pathID)
	}

	url := urlBuilder.String()

	c.logger.Info("Trying no-body approach",
		zap.String("url", url),
		zap.Int("file_ids_count", len(fileIDs)),
		zap.Any("file_ids", fileIDs))

	// Send no body
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
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

	c.logger.Info("No-body approach result",
		zap.Int("imported_count", result.ImportedCount),
		zap.Int("failed_count", result.FailedCount),
		zap.Bool("success", result.Success))

	return &result, nil
}

// finalizeImportWithPUT tries using PUT method instead of POST
func (c *Client) finalizeImportWithPUT(ctx context.Context, fileIDs []int64, libraryID, pathID string) (*BookdropFinalizeResult, error) {
	// Build URL with library and path parameters
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

	c.logger.Info("Trying PUT method approach",
		zap.String("url", url),
		zap.Int("file_ids_count", len(fileIDs)),
		zap.Any("file_ids", fileIDs))

	// Create JSON payload
	jsonData, err := json.Marshal(map[string]interface{}{
		"fileIds": fileIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	c.logger.Info("PUT method payload",
		zap.String("json", string(jsonData)))

	// Use PUT instead of POST
	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBuffer(jsonData))
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

	// PUT might return 201 Created instead of 200 OK
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, c.handleAPIError(resp)
	}

	var result BookdropFinalizeResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	c.logger.Info("PUT method approach result",
		zap.Int("imported_count", result.ImportedCount),
		zap.Int("failed_count", result.FailedCount),
		zap.Bool("success", result.Success))

	return &result, nil
}

// FinalizeAllImports finalizes all available bookdrop imports
func (c *Client) FinalizeAllImports(ctx context.Context, libraryID, pathID string) (*BookdropFinalizeResult, error) {
	if !c.IsEnabled() {
		return nil, NewAPIError(ErrInvalidToken, "Booklore API client is not configured", 0)
	}

	// Get all available files first
	files, err := c.GetBookdropFilesNoStatus(ctx, 0, 1000) // Get up to 1000 files
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

// GetLibraries retrieves all libraries available to the user
func (c *Client) GetLibraries(ctx context.Context) ([]Library, error) {
	if !c.IsEnabled() {
		return nil, NewAPIError(ErrInvalidToken, "Booklore API client is not configured", 0)
	}

	url := fmt.Sprintf("%s/api/v1/libraries", c.baseURL)

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

	var libraries []Library
	if err := json.NewDecoder(resp.Body).Decode(&libraries); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return libraries, nil
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

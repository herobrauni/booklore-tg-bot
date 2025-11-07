package booklore

import (
	"fmt"
)

// APIErrorType represents different types of API errors
type APIErrorType int

const (
	ErrInvalidToken APIErrorType = iota
	ErrNetworkError
	ErrBadRequest
	ErrUnauthorized
	ErrForbidden
	ErrNotFound
	ErrInternalServer
	ErrServiceUnavailable
)

// BookloreAPIError represents a custom error for Booklore API interactions
type BookloreAPIError struct {
	Type    APIErrorType
	Message string
	Status  int
}

func (e *BookloreAPIError) Error() string {
	return e.Message
}

// NewAPIError creates a new BookloreAPIError
func NewAPIError(errorType APIErrorType, message string, status int) *BookloreAPIError {
	return &BookloreAPIError{
		Type:    errorType,
		Message: message,
		Status:  status,
	}
}

// NewNetworkError creates a new network-related error
func NewNetworkError(err error) *BookloreAPIError {
	return NewAPIError(ErrNetworkError, fmt.Sprintf("Network error: %v", err), 0)
}

// NewAuthError creates a new authentication error
func NewAuthError(message string) *BookloreAPIError {
	return NewAPIError(ErrUnauthorized, message, 401)
}

// NewInvalidTokenError creates a new invalid token error
func NewInvalidTokenError() *BookloreAPIError {
	return NewAPIError(ErrInvalidToken, "Invalid API token", 401)
}

// Package ssh provides SSH connection management, command execution,
// SFTP file transfers, and port forwarding for the ssh-mcp server.
package ssh

import (
	"errors"
	"fmt"
)

// Code is a stable error code that agents can inspect to decide retry logic.
type Code string

const (
	CodeCommandValidationFailed Code = "COMMAND_VALIDATION_FAILED"
	CodeCommandExecutionError   Code = "COMMAND_EXECUTION_ERROR"
	CodeOutputLimitExceeded     Code = "OUTPUT_LIMIT_EXCEEDED"
	CodeCommandTimeout          Code = "COMMAND_TIMEOUT"
	CodeSSHConnectionFailed     Code = "SSH_CONNECTION_FAILED"
	CodeSSHConnectionTimeout    Code = "SSH_CONNECTION_TIMEOUT"
	CodeSSHAuthMissing          Code = "SSH_AUTHENTICATION_MISSING"
	CodeLocalPathNotAllowed     Code = "LOCAL_PATH_NOT_ALLOWED"
	CodeRemotePathNotAllowed    Code = "REMOTE_PATH_NOT_ALLOWED"
	CodeLocalFileReadFailed     Code = "LOCAL_FILE_READ_FAILED"
	CodeLocalFileWriteFailed    Code = "LOCAL_FILE_WRITE_FAILED"
	CodeOperationTimeout        Code = "OPERATION_TIMEOUT"
	CodeSFTPError               Code = "SFTP_ERROR"
	CodeUnsupportedInShellMode  Code = "UNSUPPORTED_IN_SHELL_MODE"
	CodeUnknownError            Code = "UNKNOWN_ERROR"
)

// ToolError is a typed error carrying a stable code and a retriable flag.
// It mirrors the error contract of the original TypeScript implementation
// so agent-side retry logic remains the same.
type ToolError struct {
	Code      Code
	Message   string
	Retriable bool
}

// Error implements the error interface.
func (e *ToolError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap allows errors.Is and errors.As to inspect the underlying cause.
func (e *ToolError) Unwrap() error {
	return errors.New(e.Message)
}

// NewToolError creates a ToolError with the given code, message, and retriable flag.
func NewToolError(code Code, message string, retriable bool) *ToolError {
	return &ToolError{Code: code, Message: message, Retriable: retriable}
}

// AsToolError converts an arbitrary error into a ToolError.
// If the error is already a ToolError it is returned as-is.
// Otherwise it is wrapped with the provided fallback code.
func AsToolError(err error, fallback Code) *ToolError {
	if err == nil {
		return nil
	}
	var te *ToolError
	if errors.As(err, &te) {
		return te
	}
	return &ToolError{Code: fallback, Message: err.Error(), Retriable: false}
}

// CodeFromError extracts the Code from a ToolError, returning fallback if
// the error is not a ToolError.
func CodeFromError(err error, fallback Code) Code {
	var te *ToolError
	if errors.As(err, &te) {
		return te.Code
	}
	return fallback
}

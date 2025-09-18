package ebpf

import (
	"fmt"
	"log/slog"
	"runtime"
)

// Error types for better error handling
type OrchestrationError struct {
	Type    string
	Message string
	Cause   error
	Context map[string]interface{}
}

func (e *OrchestrationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

func (e *OrchestrationError) Unwrap() error {
	return e.Cause
}

// Specific error constructors
func NewRenderError(message string, cause error, context map[string]interface{}) *OrchestrationError {
	return &OrchestrationError{
		Type:    "RenderError",
		Message: message,
		Cause:   cause,
		Context: context,
	}
}

func NewPackError(message string, cause error, context map[string]interface{}) *OrchestrationError {
	return &OrchestrationError{
		Type:    "PackError",
		Message: message,
		Cause:   cause,
		Context: context,
	}
}

func NewRegistryError(message string, cause error, context map[string]interface{}) *OrchestrationError {
	return &OrchestrationError{
		Type:    "RegistryError",
		Message: message,
		Cause:   cause,
		Context: context,
	}
}

func NewValidationError(message string, cause error, context map[string]interface{}) *OrchestrationError {
	return &OrchestrationError{
		Type:    "ValidationError",
		Message: message,
		Cause:   cause,
		Context: context,
	}
}

// Error codes for programmatic error handling
const (
	ErrCodeTemplateNotFound     = "TEMPLATE_NOT_FOUND"
	ErrCodeCompilationFailed    = "COMPILATION_FAILED"
	ErrCodePackagingFailed      = "PACKAGING_FAILED"
	ErrCodeRegistryUnavailable  = "REGISTRY_UNAVAILABLE"
	ErrCodeAuthenticationFailed = "AUTHENTICATION_FAILED"
	ErrCodeValidationFailed     = "VALIDATION_FAILED"
	ErrCodeTimeout              = "TIMEOUT"
	ErrCodePermissionDenied     = "PERMISSION_DENIED"
	ErrCodeInvalidParameters    = "INVALID_PARAMETERS"
)

// GetErrorCode extracts error code from an error
func GetErrorCode(err error) string {
	if orchErr, ok := err.(*OrchestrationError); ok {
		if code, exists := orchErr.Context["error_code"]; exists {
			if codeStr, ok := code.(string); ok {
				return codeStr
			}
		}
	}
	return "UNKNOWN_ERROR"
}

// IsRetryableError checks if an error is retryable
func IsRetryableError(err error) bool {
	code := GetErrorCode(err)
	retryableCodes := map[string]bool{
		ErrCodeRegistryUnavailable:  true,
		ErrCodeTimeout:              true,
		ErrCodeAuthenticationFailed: true,
	}
	return retryableCodes[code]
}

// LogError logs an error with context and stack trace
func LogError(logger *slog.Logger, err error, context map[string]interface{}) {
	// Get stack trace
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	stackTrace := string(buf[:n])

	// Prepare log attributes
	attrs := []interface{}{
		"error", err.Error(),
		"stack_trace", stackTrace,
	}

	// Add context attributes
	for key, value := range context {
		attrs = append(attrs, key, value)
	}

	// Add error code if available
	if code := GetErrorCode(err); code != "UNKNOWN_ERROR" {
		attrs = append(attrs, "error_code", code)
	}

	// Add retry information
	attrs = append(attrs, "retryable", IsRetryableError(err))

	logger.Error("Orchestration error occurred", attrs...)
}

// Error recovery function for panic handling
func RecoverPanic(logger *slog.Logger, context map[string]interface{}) {
	if r := recover(); r != nil {
		// Get stack trace
		buf := make([]byte, 4096)
		n := runtime.Stack(buf, false)
		stackTrace := string(buf[:n])

		// Prepare log attributes
		attrs := []interface{}{
			"panic", r,
			"stack_trace", stackTrace,
		}

		// Add context attributes
		for key, value := range context {
			attrs = append(attrs, key, value)
		}

		logger.Error("Panic recovered in orchestration", attrs...)
	}
}

// WrapError wraps an error with additional context
func WrapError(err error, message string, context map[string]interface{}) error {
	if err == nil {
		return nil
	}

	if orchErr, ok := err.(*OrchestrationError); ok {
		// Merge context
		if orchErr.Context == nil {
			orchErr.Context = make(map[string]interface{})
		}
		for k, v := range context {
			orchErr.Context[k] = v
		}
		orchErr.Message = message + ": " + orchErr.Message
		return orchErr
	}

	// Create new orchestration error
	return &OrchestrationError{
		Type:    "WrappedError",
		Message: message,
		Cause:   err,
		Context: context,
	}
}

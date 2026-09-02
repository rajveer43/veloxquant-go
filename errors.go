package veloxquant

import "errors"

// Sentinel errors returned by the SDK. Use errors.Is to check for these
// after wrapping with fmt.Errorf("...: %w", err).
var (
	ErrRuntimeUnavailable  = errors.New("veloxquant runtime unavailable")
	ErrUnsupportedPlatform = errors.New("unsupported platform")
	ErrInsufficientMemory  = errors.New("insufficient memory")
	ErrModelNotFound       = errors.New("model not found")
	ErrInvalidConfig       = errors.New("invalid configuration")
)

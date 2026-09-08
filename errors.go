package veloxquant

import (
	"errors"

	"github.com/rajveer43/veloxquant-go/models"
)

// Sentinel errors returned by the SDK. Use errors.Is to check for these
// after wrapping with fmt.Errorf("...: %w", err).
var (
	ErrRuntimeUnavailable  = errors.New("veloxquant runtime unavailable")
	ErrUnsupportedPlatform = errors.New("unsupported platform")
	ErrInsufficientMemory  = errors.New("insufficient memory")
	ErrModelNotFound       = errors.New("model not found")
	ErrInvalidConfig       = errors.New("invalid configuration")

	// ErrHuggingFaceHubUnavailable re-exports models.ErrHuggingFaceHubUnavailable
	// at the top level: it's the error models.Pull/models.Delete report when
	// the configured Python interpreter can't import huggingface_hub. Defined
	// in the models package (which is what actually detects it) and aliased
	// here, alongside this SDK's other sentinel errors, so callers can
	// errors.Is against either veloxquant.ErrHuggingFaceHubUnavailable or
	// models.ErrHuggingFaceHubUnavailable interchangeably.
	ErrHuggingFaceHubUnavailable = models.ErrHuggingFaceHubUnavailable
)

package runtime

import "errors"

// ErrUnavailable indicates the VeloxQuant runtime could not be reached.
var ErrUnavailable = errors.New("veloxquant runtime unavailable")

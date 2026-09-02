// Package memory implements VeloxQuant's memory intelligence: estimating
// how much RAM a model and its KV cache will need, and how much VeloxQuant
// compression can save.
package memory

// Precision identifies the numeric format used to store model weights or
// KV-cache entries.
type Precision string

const (
	FP16 Precision = "fp16"
	FP8  Precision = "fp8"
	Int8 Precision = "int8"
	Int4 Precision = "int4"
)

// BytesPerElement returns the storage size, in bytes, of a single scalar
// stored at this precision.
func (p Precision) BytesPerElement() float64 {
	switch p {
	case FP16:
		return 2
	case FP8:
		return 1
	case Int8:
		return 1
	case Int4:
		return 0.5
	default:
		return 2
	}
}

// Valid reports whether p is a precision recognized by the SDK.
func (p Precision) Valid() bool {
	switch p {
	case FP16, FP8, Int8, Int4:
		return true
	default:
		return false
	}
}

// Architecture describes the shape of a transformer model, sufficient to
// compute weight and KV-cache memory. Future model families can populate
// this struct differently without changing the estimator's logic.
type Architecture struct {
	NumLayers      int
	NumKVHeads     int
	HeadDim        int
	HiddenSize     int
	ParameterCount int64
}

// Package optimize provides VeloxQuant optimization profile selection and
// compression recommendations for a given model and context length.
package optimize

// Profile identifies a VeloxQuant optimization strategy trading off speed,
// memory usage, and context length.
type Profile string

const (
	ProfileSpeed          Profile = "speed"
	ProfileBalanced       Profile = "balanced"
	ProfileMemory         Profile = "memory"
	ProfileMaximumContext Profile = "maximum-context"
)

// Valid reports whether p is a recognized optimization profile.
func (p Profile) Valid() bool {
	switch p {
	case ProfileSpeed, ProfileBalanced, ProfileMemory, ProfileMaximumContext:
		return true
	default:
		return false
	}
}

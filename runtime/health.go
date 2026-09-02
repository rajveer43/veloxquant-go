package runtime

// Status describes the health of a VeloxQuant runtime instance.
type Status struct {
	Healthy bool
	Version string
	Engine  string
}

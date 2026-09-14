//go:build !windows && !darwin && !linux

package sysproxy

// Read returns an empty state on unsupported platforms.
func Read() (State, error) {
	return State{Mode: "off"}, nil
}

//go:build !linux

package gpio

func NewPulseCounter(pin int, edge string) (PulseCounter, error) {
	return DisabledPulseCounter{}, nil
}


package gpio

type PulseCounter interface {
	Current() uint64
	Enabled() bool
	Close() error
}

type DisabledPulseCounter struct{}

func (DisabledPulseCounter) Current() uint64 { return 0 }
func (DisabledPulseCounter) Enabled() bool   { return false }
func (DisabledPulseCounter) Close() error    { return nil }


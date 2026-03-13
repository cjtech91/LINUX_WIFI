//go:build linux

package gpio

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

type SysfsPulseCounter struct {
	pin     int
	count   atomic.Uint64
	enabled atomic.Bool
	stop    chan struct{}
	file    *os.File
	fd      int
}

func NewPulseCounter(pin int, edge string) (PulseCounter, error) {
	pin = int(pin)
	if pin <= 0 {
		return DisabledPulseCounter{}, nil
	}
	if edge == "" {
		edge = "rising"
	}
	edge = strings.ToLower(strings.TrimSpace(edge))
	if edge != "rising" && edge != "falling" && edge != "both" {
		return nil, fmt.Errorf("unsupported edge: %q", edge)
	}

	if err := sysfsEnsureGPIO(pin); err != nil {
		return nil, err
	}
	if err := os.WriteFile(fmt.Sprintf("/sys/class/gpio/gpio%d/direction", pin), []byte("in"), 0o644); err != nil {
		return nil, err
	}
	if err := os.WriteFile(fmt.Sprintf("/sys/class/gpio/gpio%d/edge", pin), []byte(edge), 0o644); err != nil {
		return nil, err
	}

	f, err := os.OpenFile(fmt.Sprintf("/sys/class/gpio/gpio%d/value", pin), os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	fd := int(f.Fd())

	c := &SysfsPulseCounter{
		pin:  pin,
		stop: make(chan struct{}),
		file: f,
		fd:   fd,
	}
	c.enabled.Store(true)
	go c.loop(edge)
	return c, nil
}

func (c *SysfsPulseCounter) Current() uint64 { return c.count.Load() }
func (c *SysfsPulseCounter) Enabled() bool   { return c.enabled.Load() }

func (c *SysfsPulseCounter) Close() error {
	if !c.enabled.Swap(false) {
		return nil
	}
	close(c.stop)
	if c.file != nil {
		_ = c.file.Close()
	}
	return nil
}

func (c *SysfsPulseCounter) loop(edge string) {
	buf := make([]byte, 8)
	last := byte(0)
	_, _ = syscall.Seek(c.fd, 0, 0)
	n, _ := syscall.Read(c.fd, buf)
	if n > 0 {
		last = buf[0]
	}

	pfd := []syscall.PollFd{{
		Fd:     int32(c.fd),
		Events: syscall.POLLPRI,
	}}

	for {
		select {
		case <-c.stop:
			return
		default:
		}

		pfd[0].Revents = 0
		_, err := syscall.Poll(pfd, 1000)
		if err != nil {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		if (pfd[0].Revents & syscall.POLLPRI) == 0 {
			if (pfd[0].Revents & (syscall.POLLERR | syscall.POLLNVAL)) != 0 {
				time.Sleep(100 * time.Millisecond)
			}
			continue
		}

		_, _ = syscall.Seek(c.fd, 0, 0)
		n, err = syscall.Read(c.fd, buf)
		if err != nil || n == 0 {
			continue
		}
		cur := buf[0]

		switch edge {
		case "both":
			if cur != last {
				c.count.Add(1)
			}
		case "rising":
			if last == '0' && cur == '1' {
				c.count.Add(1)
			}
		case "falling":
			if last == '1' && cur == '0' {
				c.count.Add(1)
			}
		}
		last = cur
	}
}

func sysfsEnsureGPIO(pin int) error {
	gpioPath := fmt.Sprintf("/sys/class/gpio/gpio%d", pin)
	if _, err := os.Stat(gpioPath); err == nil {
		return nil
	}
	if err := os.WriteFile("/sys/class/gpio/export", []byte(strconv.Itoa(pin)), 0o644); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "busy") {
			return nil
		}
		return err
	}
	for i := 0; i < 50; i++ {
		if _, err := os.Stat(gpioPath); err == nil {
			return nil
		}
		time.Sleep(20 * time.Millisecond)
	}
	return fmt.Errorf("gpio%d did not appear", pin)
}

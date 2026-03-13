package portal

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type Allowlister interface {
	AllowIP4(ctx context.Context, ip string, until time.Time) error
}

type NoopAllowlister struct{}

func (NoopAllowlister) AllowIP4(context.Context, string, time.Time) error { return nil }

type NFTAllowlister struct {
	ExecPath    string
	Table       string
	Allowed4    string
	TimeoutSkew time.Duration
}

func (n NFTAllowlister) AllowIP4(ctx context.Context, ip string, until time.Time) error {
	if strings.TrimSpace(ip) == "" {
		return errors.New("empty ip")
	}
	if n.ExecPath == "" {
		n.ExecPath = "nft"
	}
	if n.Table == "" {
		n.Table = "inet pisowifi"
	}
	if n.Allowed4 == "" {
		n.Allowed4 = "allowed4"
	}

	now := time.Now().UTC()
	ttl := until.UTC().Sub(now)
	if ttl <= 0 {
		return nil
	}
	if n.TimeoutSkew > 0 {
		ttl += n.TimeoutSkew
	}

	ttlSeconds := int64(ttl.Seconds())
	if ttlSeconds < 1 {
		ttlSeconds = 1
	}

	cmd := exec.CommandContext(ctx, n.ExecPath, "add", "element", n.Table, n.Allowed4, fmt.Sprintf("{ %s timeout %ds }", ip, ttlSeconds))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("nft add element: %s", msg)
	}
	return nil
}


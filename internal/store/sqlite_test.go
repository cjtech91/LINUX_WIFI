package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestVoucherConsumeCreatesSessionAndExpires(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	st, err := OpenSQLite(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	_, err = st.CreateVoucher(ctx, "ABC12345", 1)
	if err != nil {
		t.Fatalf("CreateVoucher: %v", err)
	}

	now := time.Date(2026, 3, 13, 12, 0, 0, 0, time.UTC)
	res, err := st.ConsumeVoucher(ctx, ConsumeVoucherParams{
		Code: "ABC12345",
		IP:   "10.10.10.50",
		Now:  now,
	})
	if err != nil {
		t.Fatalf("ConsumeVoucher: %v", err)
	}
	if res.Session.IP != "10.10.10.50" {
		t.Fatalf("unexpected session ip: %q", res.Session.IP)
	}
	if want := now.Add(time.Minute); !res.Session.EndAt.Equal(want) {
		t.Fatalf("unexpected endAt: got %v want %v", res.Session.EndAt, want)
	}

	_, err = st.ConsumeVoucher(ctx, ConsumeVoucherParams{
		Code: "ABC12345",
		IP:   "10.10.10.51",
		Now:  now,
	})
	if err != ErrVoucherUsed {
		t.Fatalf("expected ErrVoucherUsed, got %v", err)
	}

	_, ok, err := st.GetActiveSessionByIP(ctx, "10.10.10.50", now.Add(30*time.Second))
	if err != nil {
		t.Fatalf("GetActiveSessionByIP: %v", err)
	}
	if !ok {
		t.Fatalf("expected active session")
	}

	_, ok, err = st.GetActiveSessionByIP(ctx, "10.10.10.50", now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("GetActiveSessionByIP: %v", err)
	}
	if ok {
		t.Fatalf("expected expired session")
	}
}


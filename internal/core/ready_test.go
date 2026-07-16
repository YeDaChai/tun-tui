package core

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestWaitReady_ImmediateOK(t *testing.T) {
	err := waitReady(context.Background(), func(context.Context) error {
		return nil
	}, nil, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
}

func TestWaitReady_RetriesThenOK(t *testing.T) {
	var n atomic.Int32
	err := waitReady(context.Background(), func(context.Context) error {
		if n.Add(1) < 3 {
			return errors.New("not yet")
		}
		return nil
	}, nil, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if n.Load() < 3 {
		t.Fatalf("expected retries, got %d checks", n.Load())
	}
}

func TestWaitReady_ErrTickWins(t *testing.T) {
	boom := errors.New("tun failed")
	err := waitReady(context.Background(), func(context.Context) error {
		return errors.New("not ready")
	}, func() error {
		return boom
	}, time.Millisecond)
	if !errors.Is(err, boom) {
		t.Fatalf("got %v, want tun failed", err)
	}
}

func TestWaitReady_Timeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	err := waitReady(ctx, func(context.Context) error {
		return errors.New("never")
	}, nil, 5*time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v, want deadline exceeded", err)
	}
}

func TestDefaultReadyCheck_TimeoutMessage(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := DefaultReadyCheck("127.0.0.1:1", "", nil)(ctx)
	if err == nil || !strings.Contains(err.Error(), "127.0.0.1:1") {
		t.Fatalf("got %v", err)
	}
}

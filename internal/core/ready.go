package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ReadyFunc probes whether the kernel control plane is up.
type ReadyFunc func(ctx context.Context) error

// DefaultReadyCheck polls the external-controller HTTP API until it responds.
func DefaultReadyCheck(addr, secret string) ReadyFunc {
	return func(ctx context.Context) error {
		err := waitReady(ctx, func(ctx context.Context) error {
			return pingController(ctx, addr, secret)
		}, 100*time.Millisecond)
		if err != nil && (errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)) {
			return fmt.Errorf("内核未响应控制接口 %s，启动可能失败（检查权限或端口占用）", addr)
		}
		return err
	}
}

func waitReady(ctx context.Context, check func(context.Context) error, interval time.Duration) error {
	if err := check(ctx); err == nil {
		return nil
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := check(ctx); err == nil {
				return nil
			}
		}
	}
}

func pingController(ctx context.Context, addr, secret string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+"/configs", nil)
	if err != nil {
		return err
	}
	if secret != "" {
		req.Header.Set("Authorization", "Bearer "+secret)
	}
	client := &http.Client{Timeout: 200 * time.Millisecond}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("controller status %d", resp.StatusCode)
	}
	return nil
}

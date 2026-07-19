package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// WaitReady polls the external-controller HTTP API until it responds or ctx expires.
func WaitReady(ctx context.Context, addr, secret string) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := pingController(ctx, addr, secret); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(ctx.Err(), context.Canceled) {
				return fmt.Errorf("内核未响应控制接口 %s，启动可能失败（检查权限或端口占用）", addr)
			}
			return ctx.Err()
		case <-ticker.C:
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

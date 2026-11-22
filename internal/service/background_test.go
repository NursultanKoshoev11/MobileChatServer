package service

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestGoRecoversPanicsAndWaitBackgroundWaits(t *testing.T) {
	svc := &Service{}
	var finished atomic.Bool
	svc.Go("panics", func(context.Context) { panic("boom") })
	svc.Go("slow", func(ctx context.Context) {
		if _, ok := ctx.Deadline(); !ok {
			t.Error("background context has no deadline")
		}
		time.Sleep(50 * time.Millisecond)
		finished.Store(true)
	})
	if !svc.WaitBackground(2 * time.Second) {
		t.Fatal("background tasks did not finish")
	}
	if !finished.Load() {
		t.Fatal("WaitBackground returned before the slow task finished")
	}
}

func TestWaitBackgroundIsBounded(t *testing.T) {
	svc := &Service{}
	release := make(chan struct{})
	defer close(release)
	svc.Go("blocked", func(context.Context) { <-release })
	started := time.Now()
	if svc.WaitBackground(50 * time.Millisecond) {
		t.Fatal("expected timeout while a task is still running")
	}
	if time.Since(started) > time.Second {
		t.Fatal("WaitBackground exceeded its bound")
	}
}

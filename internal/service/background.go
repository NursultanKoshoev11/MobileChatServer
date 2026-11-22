package service

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// backgroundTaskTimeout bounds one background job (for example a push fan-out
// to a large group). It is detached from the HTTP request that started it.
const backgroundTaskTimeout = 60 * time.Second

// backgroundTasks tracks fire-and-forget work started after a response so
// graceful shutdown can wait for it.
type backgroundTasks struct {
	wg sync.WaitGroup
}

// Go runs fn in a tracked goroutine with a fresh context (bounded by
// backgroundTaskTimeout and independent of any request context). A panic in fn
// is recovered and logged instead of crashing the process.
func (s *Service) Go(name string, fn func(ctx context.Context)) {
	s.background.wg.Add(1)
	go func() {
		defer s.background.wg.Done()
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Printf("background task panic recovered task=%q error=%q", name, fmt.Sprint(recovered))
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), backgroundTaskTimeout)
		defer cancel()
		fn(ctx)
	}()
}

// WaitBackground waits until every task started with Go has finished or the
// timeout elapses. It reports whether all tasks finished.
func (s *Service) WaitBackground(timeout time.Duration) bool {
	done := make(chan struct{})
	go func() {
		s.background.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}

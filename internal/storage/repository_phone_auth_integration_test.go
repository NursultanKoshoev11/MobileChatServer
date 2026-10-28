package storage

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPhoneVerificationCodeLimitsAreAtomic(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping test database: %v", err)
	}
	repo := NewRepository(pool)

	var randomBytes [16]byte
	if _, err := rand.Read(randomBytes[:]); err != nil {
		t.Fatalf("generate isolated test identifiers: %v", err)
	}
	runID := hex.EncodeToString(randomBytes[:])
	phone := fmt.Sprintf("+1999%010d", binary.BigEndian.Uint64(randomBytes[:8])%10_000_000_000)
	consumePhone := fmt.Sprintf("+1888%010d", binary.BigEndian.Uint64(randomBytes[8:])%10_000_000_000)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM phone_verification_codes WHERE phone_number = $1 OR phone_number = $2`, phone, consumePhone)
	})

	const concurrentRequests = 8
	now := time.Now().UTC()
	start := make(chan struct{})
	accepted := atomic.Int32{}
	acceptedID := make(chan string, concurrentRequests)
	errorsFound := make(chan error, concurrentRequests)
	var workers sync.WaitGroup
	for i := 0; i < concurrentRequests; i++ {
		workers.Add(1)
		go func(index int) {
			defer workers.Done()
			<-start
			id := fmt.Sprintf("PC-IT-%s-%02d", runID, index)
			allowed, err := repo.CreatePhoneCodeIfWithinLimit(
				ctx,
				id,
				phone,
				"integration-test-hash",
				now.Add(5*time.Minute),
				now.Add(-10*time.Minute),
				now.Add(-30*time.Second),
				3,
			)
			if err != nil {
				errorsFound <- err
				return
			}
			if allowed {
				accepted.Add(1)
				acceptedID <- id
			}
		}(i)
	}
	close(start)
	workers.Wait()
	close(errorsFound)
	for err := range errorsFound {
		t.Errorf("concurrent issuance failed: %v", err)
	}
	if got := accepted.Load(); got != 1 {
		t.Fatalf("expected one request to pass the per-phone minimum gap, got %d", got)
	}
	firstID := <-acceptedID

	// Bypass the minimum-gap constraint in this direct repository check so the
	// rolling request-count limit can be tested without waiting ten minutes.
	for i := 0; i < 2; i++ {
		allowed, err := repo.CreatePhoneCodeIfWithinLimit(
			ctx,
			fmt.Sprintf("PC-IT-%s-count-%d", runID, i),
			phone,
			"integration-test-hash",
			now.Add(5*time.Minute),
			now.Add(-10*time.Minute),
			now.Add(time.Hour),
			3,
		)
		if err != nil || !allowed {
			t.Fatalf("request %d should fit under the rolling count limit: allowed=%t err=%v", i+2, allowed, err)
		}
	}
	allowed, err := repo.CreatePhoneCodeIfWithinLimit(
		ctx,
		"PC-IT-"+runID+"-over-limit",
		phone,
		"integration-test-hash",
		now.Add(5*time.Minute),
		now.Add(-10*time.Minute),
		now.Add(time.Hour),
		3,
	)
	if err != nil || allowed {
		t.Fatalf("request over the rolling count limit should be denied: allowed=%t err=%v", allowed, err)
	}

	const concurrentVerifications = 8
	start = make(chan struct{})
	acceptedVerifications := atomic.Int32{}
	errorsFound = make(chan error, concurrentVerifications)
	workers = sync.WaitGroup{}
	for i := 0; i < concurrentVerifications; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			incremented, err := repo.IncrementPhoneCodeAttempts(ctx, firstID, 5)
			if err != nil {
				errorsFound <- err
				return
			}
			if incremented {
				acceptedVerifications.Add(1)
			}
		}()
	}
	close(start)
	workers.Wait()
	close(errorsFound)
	for err := range errorsFound {
		t.Errorf("concurrent verification failed: %v", err)
	}
	if got := acceptedVerifications.Load(); got != 5 {
		t.Fatalf("expected only five concurrent wrong-code attempts to consume the budget, got %d", got)
	}
	if incremented, err := repo.IncrementPhoneCodeAttempts(ctx, firstID, 5); err != nil || incremented {
		t.Fatalf("attempt beyond the limit should not increment: incremented=%t err=%v", incremented, err)
	}
	if consumed, err := repo.ConsumePhoneCode(ctx, firstID, 5); err != nil || consumed {
		t.Fatalf("exhausted code should not be consumable: consumed=%t err=%v", consumed, err)
	}

	consumeID := "PC-IT-" + runID + "-consume"
	if _, err := pool.Exec(ctx, `
		INSERT INTO phone_verification_codes (id, phone_number, code_hash, expires_at)
		VALUES ($1, $2, 'integration-test-hash', now() + interval '5 minutes')
	`, consumeID, consumePhone); err != nil {
		t.Fatalf("create single-use test code: %v", err)
	}
	start = make(chan struct{})
	acceptedConsumes := atomic.Int32{}
	errorsFound = make(chan error, concurrentVerifications)
	workers = sync.WaitGroup{}
	for i := 0; i < concurrentVerifications; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			consumed, err := repo.ConsumePhoneCode(ctx, consumeID, 5)
			if err != nil {
				errorsFound <- err
				return
			}
			if consumed {
				acceptedConsumes.Add(1)
			}
		}()
	}
	close(start)
	workers.Wait()
	close(errorsFound)
	for err := range errorsFound {
		t.Errorf("concurrent code consumption failed: %v", err)
	}
	if got := acceptedConsumes.Load(); got != 1 {
		t.Fatalf("expected one concurrent use of a valid code, got %d", got)
	}
}

package service

import (
	"context"
	"log"
	"time"
)

const (
	// phoneCodeRetention must stay longer than the longest window that counts
	// codes (the 24h per-number cap), so deleting old rows never resets a cap.
	phoneCodeRetention         = 48 * time.Hour
	phoneCodeRetentionInterval = time.Hour
)

// PurgeOldPhoneCodes deletes phone verification codes older than the retention
// period.
func (s *PhoneAuthService) PurgeOldPhoneCodes(ctx context.Context, now time.Time) (int64, error) {
	return s.repo.DeletePhoneVerificationCodesCreatedBefore(ctx, now.Add(-phoneCodeRetention))
}

// RunPhoneCodeRetention purges old phone verification codes at start-up and
// then hourly until ctx is cancelled.
func (s *PhoneAuthService) RunPhoneCodeRetention(ctx context.Context, logger *log.Logger) {
	run := func() {
		runCtx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()
		deleted, err := s.PurgeOldPhoneCodes(runCtx, time.Now().UTC())
		if err != nil {
			if ctx.Err() == nil && logger != nil {
				logger.Printf("phone code retention failed: %v", err)
			}
			return
		}
		if deleted > 0 && logger != nil {
			logger.Printf("phone code retention deleted=%d older_than=%s", deleted, phoneCodeRetention)
		}
	}
	run()
	ticker := time.NewTicker(phoneCodeRetentionInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}

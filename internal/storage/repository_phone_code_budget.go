package storage

import (
	"context"
	"fmt"
	"time"
)

// CountPhoneCodesIssued returns how many verification codes were issued to
// phoneNumber since phoneSince, and how many were issued to any number since
// globalSince. Both are used as SMS spend guards before a new code is sent.
// The counts are advisory: the exact per-phone limits are still enforced
// atomically by CreatePhoneCodeIfWithinLimit.
func (r *Repository) CountPhoneCodesIssued(ctx context.Context, phoneNumber string, phoneSince, globalSince time.Time) (phoneCount, globalCount int, err error) {
	if err := r.db.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*)::int FROM phone_verification_codes WHERE phone_number = $1 AND created_at >= $2),
			(SELECT COUNT(*)::int FROM phone_verification_codes WHERE created_at >= $3)
	`, phoneNumber, phoneSince, globalSince).Scan(&phoneCount, &globalCount); err != nil {
		return 0, 0, fmt.Errorf("count issued phone codes: %w", err)
	}
	return phoneCount, globalCount, nil
}

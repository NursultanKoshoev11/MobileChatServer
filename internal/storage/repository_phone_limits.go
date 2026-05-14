package storage

import (
	"context"
	"fmt"
	"time"
)

func (r *Repository) CountPhoneCodesSince(ctx context.Context, phoneNumber string, since time.Time) (int, error) {
	var count int
	if err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)::int
		FROM phone_verification_codes
		WHERE phone_number = $1 AND created_at >= $2`, phoneNumber, since).Scan(&count); err != nil {
		return 0, fmt.Errorf("count phone codes since: %w", err)
	}
	return count, nil
}

func (r *Repository) LatestPhoneCodeCreatedAt(ctx context.Context, phoneNumber string) (*time.Time, error) {
	var createdAt time.Time
	err := r.db.QueryRow(ctx, `
		SELECT created_at
		FROM phone_verification_codes
		WHERE phone_number = $1
		ORDER BY created_at DESC
		LIMIT 1`, phoneNumber).Scan(&createdAt)
	if err != nil {
		return nil, nil
	}
	return &createdAt, nil
}

package storage

import (
	"context"
	"fmt"
	"time"
)

// DeletePhoneVerificationCodesCreatedBefore removes verification codes created
// before cutoff (uses idx_phone_verification_codes_created) and returns how
// many rows were deleted.
func (r *Repository) DeletePhoneVerificationCodesCreatedBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	result, err := r.db.Exec(ctx, `DELETE FROM phone_verification_codes WHERE created_at < $1`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("delete old phone verification codes: %w", err)
	}
	return result.RowsAffected(), nil
}

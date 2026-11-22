package storage

import (
	"context"
	"testing"
	"time"
)

func TestDeletePhoneVerificationCodesCreatedBefore(t *testing.T) {
	repo, pool, ctx := openIntegrationRepo(t)
	phone := randomTestPhone(t)
	id := integrationID(t)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM phone_verification_codes WHERE phone_number = $1`, phone)
	})
	now := time.Now().UTC()
	for _, row := range []struct {
		id  string
		age time.Duration
	}{{"PVC-OLD-" + id, 49 * time.Hour}, {"PVC-NEW-" + id, 47 * time.Hour}} {
		if _, err := pool.Exec(ctx, `
			INSERT INTO phone_verification_codes (id, phone_number, code_hash, expires_at, created_at)
			VALUES ($1, $2, 'hash', $3, $3)`, row.id, phone, now.Add(-row.age)); err != nil {
			t.Fatalf("insert code: %v", err)
		}
	}
	if _, err := repo.DeletePhoneVerificationCodesCreatedBefore(ctx, now.Add(-48*time.Hour)); err != nil {
		t.Fatalf("delete: %v", err)
	}
	var remaining []string
	rows, err := pool.Query(ctx, `SELECT id FROM phone_verification_codes WHERE phone_number = $1`, phone)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var rowID string
		if err := rows.Scan(&rowID); err != nil {
			t.Fatal(err)
		}
		remaining = append(remaining, rowID)
	}
	if len(remaining) != 1 || remaining[0] != "PVC-NEW-"+id {
		t.Fatalf("remaining codes = %v, want only the 47h-old code", remaining)
	}
}

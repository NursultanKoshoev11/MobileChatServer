package storage

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrUploadQuotaExceeded is returned when an upload would exceed the
// uploader's rolling daily quota.
var ErrUploadQuotaExceeded = errors.New("upload quota exceeded")

type UploadedFile struct {
	ID          string
	GroupID     string
	UploaderID  string
	SizeBytes   int64
	ContentType string
	// StoragePath is relative to the public file root ("<group>/<id><ext>").
	StoragePath string
	CreatedAt   time.Time
}

// ReserveUpload records file if the uploader's uploads since `since` plus
// file.SizeBytes stay within quota (quota <= 0 disables the check). It returns
// the bytes already used in the window. Concurrent uploads by the same user are
// serialized with a transaction-scoped advisory lock so they cannot overshoot.
func (r *Repository) ReserveUpload(ctx context.Context, file UploadedFile, quota int64, since time.Time) (int64, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin upload reservation: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('upload-quota:' || $1, 0))`, file.UploaderID); err != nil {
		return 0, fmt.Errorf("lock upload quota: %w", err)
	}
	var used int64
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(size_bytes), 0)::bigint
		FROM uploaded_files
		WHERE uploader_id = $1 AND created_at >= $2`, file.UploaderID, since).Scan(&used); err != nil {
		return 0, fmt.Errorf("sum recent uploads: %w", err)
	}
	if quota > 0 && used+file.SizeBytes > quota {
		return used, ErrUploadQuotaExceeded
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO uploaded_files (id, group_id, uploader_id, size_bytes, content_type, storage_path)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		file.ID, file.GroupID, file.UploaderID, file.SizeBytes, file.ContentType, file.StoragePath); err != nil {
		return used, fmt.Errorf("record upload: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return used, fmt.Errorf("commit upload reservation: %w", err)
	}
	return used, nil
}

func (r *Repository) DeleteUploadedFile(ctx context.Context, id string) error {
	if _, err := r.db.Exec(ctx, `DELETE FROM uploaded_files WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete upload record: %w", err)
	}
	return nil
}

// ListUploadedFilesByUploader returns every recorded upload of userID.
func (r *Repository) ListUploadedFilesByUploader(ctx context.Context, userID string) ([]UploadedFile, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, group_id, uploader_id, size_bytes, content_type, storage_path, created_at
		FROM uploaded_files
		WHERE uploader_id = $1
		ORDER BY created_at`, userID)
	if err != nil {
		return nil, fmt.Errorf("list user uploads: %w", err)
	}
	defer rows.Close()
	files := make([]UploadedFile, 0)
	for rows.Next() {
		var file UploadedFile
		if err := rows.Scan(&file.ID, &file.GroupID, &file.UploaderID, &file.SizeBytes, &file.ContentType, &file.StoragePath, &file.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan user upload: %w", err)
		}
		files = append(files, file)
	}
	return files, rows.Err()
}

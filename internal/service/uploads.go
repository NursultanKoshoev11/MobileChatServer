package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/storage"
)

// uploadQuotaWindow is the rolling window of the per-user upload quota.
const uploadQuotaWindow = 24 * time.Hour

// UploadQuotaExceededError reports that an upload would exceed the uploader's
// daily quota.
type UploadQuotaExceededError struct {
	QuotaBytes int64
	UsedBytes  int64
}

func (e UploadQuotaExceededError) Error() string {
	return fmt.Sprintf("daily upload quota exceeded: %d of %d MB used in the last 24 hours", e.UsedBytes>>20, e.QuotaBytes>>20)
}

// SetUploadQuota sets the per-user upload quota in bytes per rolling 24 hours
// (0 disables it).
func (s *Service) SetUploadQuota(bytes int64) {
	if bytes < 0 {
		bytes = 0
	}
	s.uploadQuotaBytes = bytes
}

// RecordGroupUpload checks the uploader's daily quota and records the file. It
// must be called before the file is written; on a write failure the caller
// removes the record with ForgetGroupUpload.
func (s *Service) RecordGroupUpload(ctx context.Context, file storage.UploadedFile) error {
	if strings.TrimSpace(file.UploaderID) == "" {
		return ErrUnauthorized
	}
	used, err := s.repo.ReserveUpload(ctx, file, s.uploadQuotaBytes, time.Now().UTC().Add(-uploadQuotaWindow))
	if errors.Is(err, storage.ErrUploadQuotaExceeded) {
		return UploadQuotaExceededError{QuotaBytes: s.uploadQuotaBytes, UsedBytes: used}
	}
	return err
}

func (s *Service) ForgetGroupUpload(ctx context.Context, fileID string) error {
	return s.repo.DeleteUploadedFile(ctx, fileID)
}

// ListUserUploads lists the recorded group files uploaded by userID.
func (s *Service) ListUserUploads(ctx context.Context, userID string) ([]storage.UploadedFile, error) {
	return s.repo.ListUploadedFilesByUploader(ctx, strings.TrimSpace(userID))
}

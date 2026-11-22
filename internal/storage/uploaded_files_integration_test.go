package storage

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
)

func TestReserveUploadEnforcesRollingQuota(t *testing.T) {
	repo, pool, ctx := openIntegrationRepo(t)
	user := createIntegrationUser(t, ctx, repo, pool, randomTestPhone(t))
	group, err := repo.CreateGroup(ctx, domain.Group{ID: "G-IT-" + integrationID(t), OwnerID: user.ID, Title: "quota", Visibility: domain.VisibilityPrivate})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM groups WHERE id = $1`, group.ID) })

	const quota = 1000
	since := time.Now().UTC().Add(-24 * time.Hour)
	file := func(size int64) UploadedFile {
		id := "PF-IT" + integrationID(t)
		return UploadedFile{ID: id, GroupID: group.ID, UploaderID: user.ID, SizeBytes: size, ContentType: "image/png", StoragePath: group.ID + "/" + id + ".png"}
	}
	if _, err := repo.ReserveUpload(ctx, file(600), quota, since); err != nil {
		t.Fatalf("first upload: %v", err)
	}
	used, err := repo.ReserveUpload(ctx, file(500), quota, since)
	if !errors.Is(err, ErrUploadQuotaExceeded) || used != 600 {
		t.Fatalf("over-quota upload: used=%d err=%v", used, err)
	}

	// Concurrent uploads cannot overshoot the quota together.
	var wg sync.WaitGroup
	var mu sync.Mutex
	accepted := 0
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := repo.ReserveUpload(ctx, file(100), quota, since); err == nil {
				mu.Lock()
				accepted++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if accepted != 4 {
		t.Fatalf("concurrent uploads accepted = %d, want 4 (quota 1000, 600 used)", accepted)
	}

	// Uploads older than the window do not count; quota 0 disables the check.
	if _, err := pool.Exec(ctx, `UPDATE uploaded_files SET created_at = now() - interval '25 hours' WHERE uploader_id = $1`, user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ReserveUpload(ctx, file(900), quota, since); err != nil {
		t.Fatalf("upload after the window rolled: %v", err)
	}
	if _, err := repo.ReserveUpload(ctx, file(5000), 0, since); err != nil {
		t.Fatalf("quota 0 must disable the check: %v", err)
	}
	files, err := repo.ListUploadedFilesByUploader(ctx, user.ID)
	if err != nil || len(files) != 7 {
		t.Fatalf("list uploads: %d %v", len(files), err)
	}
}

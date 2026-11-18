package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

// These tests need a migrated database: TEST_DATABASE_URL=postgres://...

func openIntegrationRepo(t *testing.T) (*Repository, *pgxpool.Pool, context.Context) {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	t.Cleanup(pool.Close)
	return NewRepository(pool), pool, ctx
}

func integrationID(t *testing.T) string {
	t.Helper()
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b[:])
}

func randomTestPhone(t *testing.T) string {
	t.Helper()
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("+1777%07d", (uint32(b[0])<<24|uint32(b[1])<<16|uint32(b[2])<<8|uint32(b[3]))%10_000_000)
}

func createIntegrationUser(t *testing.T, ctx context.Context, repo *Repository, pool *pgxpool.Pool, phone string) domain.User {
	t.Helper()
	id := "U-IT-" + integrationID(t)
	user, err := repo.CreateUser(ctx, domain.User{ID: id, Email: id + "@it.invalid", DisplayName: "IT user", Phone: phone}, "")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, user.ID) })
	return user
}

func TestSyncAdminPhoneAllowlistRevokesRemovedPhones(t *testing.T) {
	repo, pool, ctx := openIntegrationRepo(t)
	keptPhone := randomTestPhone(t)
	removedPhone := randomTestPhone(t)
	kept := createIntegrationUser(t, ctx, repo, pool, keptPhone)
	removed := createIntegrationUser(t, ctx, repo, pool, removedPhone)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM admin_phone_allowlist WHERE phone = ANY($1)`, []string{keptPhone, removedPhone})
	})
	// Snapshot the allowlist so the test does not leave other entries disabled.
	rows, err := pool.Query(ctx, `SELECT phone, role, enabled FROM admin_phone_allowlist`)
	if err != nil {
		t.Fatal(err)
	}
	type entry struct {
		phone, role string
		enabled     bool
	}
	var snapshot []entry
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.phone, &e.role, &e.enabled); err != nil {
			t.Fatal(err)
		}
		snapshot = append(snapshot, e)
	}
	rows.Close()
	t.Cleanup(func() {
		for _, e := range snapshot {
			_, _ = pool.Exec(context.Background(), `UPDATE admin_phone_allowlist SET role = $2, enabled = $3 WHERE phone = $1`, e.phone, e.role, e.enabled)
		}
	})

	// Phones are written with separators to check login-equivalent normalization.
	spaced := keptPhone[:4] + " " + keptPhone[4:7] + "-" + keptPhone[7:]
	if err := repo.SyncAdminPhoneAllowlist(ctx, []string{spaced}, []string{removedPhone}); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	assertRole := func(userID string, want domain.UserRole) {
		t.Helper()
		user, err := repo.GetUserByID(ctx, userID)
		if err != nil {
			t.Fatal(err)
		}
		if user.Role != want {
			t.Fatalf("user %s role = %q, want %q", userID, user.Role, want)
		}
	}
	assertRole(kept.ID, domain.UserRoleSuperAdmin)
	assertRole(removed.ID, domain.UserRolePlatformAdmin)
	if role, _ := repo.ResolveUserRoleByPhone(ctx, keptPhone); role != domain.UserRoleSuperAdmin {
		t.Fatalf("login lookup for normalized phone = %q", role)
	}

	if err := repo.SyncAdminPhoneAllowlist(ctx, []string{keptPhone}, nil); err != nil {
		t.Fatalf("second sync: %v", err)
	}
	assertRole(kept.ID, domain.UserRoleSuperAdmin)
	assertRole(removed.ID, domain.UserRoleUser)
	if role, _ := repo.ResolveUserRoleByPhone(ctx, removedPhone); role != domain.UserRoleUser {
		t.Fatalf("removed phone still resolves to %q", role)
	}
}

type reportFixture struct {
	owner, member, outsider domain.User
	groupID, requestID      string
	commentID, messageID    string
}

func createReportFixture(t *testing.T, ctx context.Context, repo *Repository, pool *pgxpool.Pool) reportFixture {
	t.Helper()
	f := reportFixture{
		owner:    createIntegrationUser(t, ctx, repo, pool, ""),
		member:   createIntegrationUser(t, ctx, repo, pool, ""),
		outsider: createIntegrationUser(t, ctx, repo, pool, ""),
	}
	group, err := repo.CreateGroup(ctx, domain.Group{ID: "G-IT-" + integrationID(t), OwnerID: f.owner.ID, Title: "IT group", Visibility: domain.VisibilityPrivate})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	f.groupID = group.ID
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM groups WHERE id = $1`, group.ID) })
	if _, err := pool.Exec(ctx, `INSERT INTO group_members (group_id, user_id, role) VALUES ($1, $2, 'member')`, group.ID, f.member.ID); err != nil {
		t.Fatal(err)
	}
	request, err := repo.CreatePublicRequest(ctx, domain.PublicRequest{ID: "REQ-IT-" + integrationID(t), GroupID: group.ID, AuthorID: f.owner.ID, RequestType: domain.PublicRequestIdea, InteractionMode: domain.InteractionModeDiscussion, Title: "Title", Body: "Body text"})
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	f.requestID = request.ID
	comment, err := repo.CreatePublicRequestComment(ctx, domain.PublicRequestComment{ID: "COM-IT-" + integrationID(t), RequestID: request.ID, AuthorID: f.owner.ID, Body: "comment"})
	if err != nil {
		t.Fatalf("create comment: %v", err)
	}
	f.commentID = comment.ID
	message, err := repo.CreateMessage(ctx, domain.Message{ID: "M-IT-" + integrationID(t), GroupID: group.ID, SenderID: f.owner.ID, Text: "hello"})
	if err != nil {
		t.Fatalf("create message: %v", err)
	}
	f.messageID = message.ID
	return f
}

func TestContentReportsAndBlocks(t *testing.T) {
	repo, pool, ctx := openIntegrationRepo(t)
	f := createReportFixture(t, ctx, repo, pool)

	// Visibility: outsiders cannot report group content; anyone can report a user.
	for _, tt := range []domain.ReportTargetType{domain.ReportTargetPublicRequest, domain.ReportTargetComment, domain.ReportTargetMessage} {
		id := map[domain.ReportTargetType]string{domain.ReportTargetPublicRequest: f.requestID, domain.ReportTargetComment: f.commentID, domain.ReportTargetMessage: f.messageID}[tt]
		if _, err := repo.ResolveReportTarget(ctx, tt, id, f.outsider.ID); !errors.Is(err, ErrNotFound) {
			t.Fatalf("outsider resolving %s: err=%v, want ErrNotFound", tt, err)
		}
		target, err := repo.ResolveReportTarget(ctx, tt, id, f.member.ID)
		if err != nil || target.GroupID != f.groupID || target.AuthorID != f.owner.ID {
			t.Fatalf("member resolving %s: %+v err=%v", tt, target, err)
		}
	}
	if _, err := repo.ResolveReportTarget(ctx, domain.ReportTargetUser, f.owner.ID, f.outsider.ID); err != nil {
		t.Fatalf("user target: %v", err)
	}
	if _, err := repo.ResolveReportTarget(ctx, domain.ReportTargetUser, "U-missing", f.outsider.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing user target: %v", err)
	}

	// Idempotent report creation.
	report := domain.ContentReport{ID: "RPT-IT-" + integrationID(t), TargetType: domain.ReportTargetComment, TargetID: f.commentID, GroupID: f.groupID, TargetUserID: f.owner.ID, Reason: domain.ReportReasonAbuse, ReporterID: f.member.ID}
	first, created, err := repo.CreateContentReport(ctx, report)
	if err != nil || !created || first.Status != domain.ReportStatusOpen {
		t.Fatalf("first report: %+v created=%v err=%v", first, created, err)
	}
	report.ID = "RPT-IT-" + integrationID(t)
	second, created, err := repo.CreateContentReport(ctx, report)
	if err != nil || created || second.ID != first.ID {
		t.Fatalf("duplicate report: %+v created=%v err=%v", second, created, err)
	}
	open, err := repo.ListGroupContentReports(ctx, f.groupID, domain.ReportStatusOpen)
	if err != nil || len(open) != 1 || open[0].ReporterDisplayName != "IT user" {
		t.Fatalf("group reports: %+v err=%v", open, err)
	}
	removed, err := repo.RemoveReportedContent(ctx, domain.ReportTargetComment, f.commentID, f.owner.ID)
	if err != nil || removed.RequestID != f.requestID || removed.GroupID != f.groupID {
		t.Fatalf("remove comment: %+v err=%v", removed, err)
	}
	if err := repo.ResolveContentReports(ctx, first, domain.ReportResolutionContentRemoved, f.owner.ID); err != nil {
		t.Fatal(err)
	}
	resolved, err := repo.GetContentReport(ctx, first.ID)
	if err != nil || resolved.Status != domain.ReportStatusResolved || resolved.Resolution != string(domain.ReportResolutionContentRemoved) {
		t.Fatalf("resolved report: %+v err=%v", resolved, err)
	}

	// Blocking hides the blocked author's content from the blocker only.
	countVisible := func(viewer string) (int, int, int) {
		t.Helper()
		messages, err := repo.ListMessages(ctx, f.groupID, viewer, 50, time.Time{})
		if err != nil {
			t.Fatal(err)
		}
		requests, err := repo.ListPublicRequests(ctx, f.groupID, viewer, 50, time.Time{}, "", false)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := repo.CreatePublicRequestComment(ctx, domain.PublicRequestComment{ID: "COM-IT-" + integrationID(t), RequestID: f.requestID, AuthorID: f.owner.ID, Body: "again"}); err != nil {
			t.Fatal(err)
		}
		comments, err := repo.ListPublicRequestComments(ctx, f.requestID, viewer)
		if err != nil {
			t.Fatal(err)
		}
		return len(messages), len(requests), len(comments)
	}
	if m, r, c := countVisible(f.member.ID); m != 1 || r != 1 || c != 1 {
		t.Fatalf("before block: messages=%d requests=%d comments=%d", m, r, c)
	}
	if err := repo.BlockUser(ctx, f.member.ID, f.owner.ID); err != nil {
		t.Fatal(err)
	}
	if err := repo.BlockUser(ctx, f.member.ID, f.owner.ID); err != nil {
		t.Fatalf("block must be idempotent: %v", err)
	}
	if err := repo.BlockUser(ctx, f.member.ID, "U-missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("blocking a missing user: %v", err)
	}
	if m, r, c := countVisible(f.member.ID); m != 0 || r != 0 || c != 0 {
		t.Fatalf("after block: messages=%d requests=%d comments=%d", m, r, c)
	}
	blocks, err := repo.ListUserBlocks(ctx, f.member.ID)
	if err != nil || len(blocks) != 1 || blocks[0].UserID != f.owner.ID {
		t.Fatalf("blocks: %+v err=%v", blocks, err)
	}
	if err := repo.UnblockUser(ctx, f.member.ID, f.owner.ID); err != nil {
		t.Fatal(err)
	}
	if m, r, c := countVisible(f.member.ID); m != 1 || r != 1 || c != 3 {
		t.Fatalf("after unblock: messages=%d requests=%d comments=%d", m, r, c)
	}

	// Account deletion removes blocks in both directions and the user's codes.
	if err := repo.BlockUser(ctx, f.outsider.ID, f.member.ID); err != nil {
		t.Fatal(err)
	}
	phone := randomTestPhone(t)
	if _, err := pool.Exec(ctx, `UPDATE users SET phone = $2 WHERE id = $1`, f.outsider.ID, phone); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO phone_verification_codes (id, phone_number, code_hash, expires_at) VALUES ($1, $2, 'h', now())`, "PC-IT-"+integrationID(t), phone); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteAccount(ctx, f.outsider.ID, "EVT-IT-"+integrationID(t)); err != nil {
		t.Fatalf("delete account: %v", err)
	}
	var blockRows, codeRows int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM user_blocks WHERE blocker_id = $1 OR blocked_id = $1`, f.outsider.ID).Scan(&blockRows)
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM phone_verification_codes WHERE phone_number = $1`, phone).Scan(&codeRows)
	if blockRows != 0 || codeRows != 0 {
		t.Fatalf("after account deletion: blocks=%d codes=%d", blockRows, codeRows)
	}
}

func TestPushTokenMovesToNewOwner(t *testing.T) {
	repo, pool, ctx := openIntegrationRepo(t)
	first := createIntegrationUser(t, ctx, repo, pool, "")
	second := createIntegrationUser(t, ctx, repo, pool, "")
	token := "tok-IT-" + integrationID(t)
	if err := repo.UpsertPushToken(ctx, first.ID, token, "android"); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpsertPushToken(ctx, second.ID, token, "android"); err != nil {
		t.Fatal(err)
	}
	var owner string
	var count int
	if err := pool.QueryRow(ctx, `SELECT MIN(user_id), COUNT(*) FROM push_tokens WHERE token = $1`, token).Scan(&owner, &count); err != nil {
		t.Fatal(err)
	}
	if count != 1 || owner != second.ID {
		t.Fatalf("token owner=%q count=%d, want %q/1", owner, count, second.ID)
	}
}

func TestLearnedModerationRuleWildcardsAreLiteral(t *testing.T) {
	repo, pool, ctx := openIntegrationRepo(t)
	groupID := "G-IT-" + integrationID(t)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM learned_moderation_rules WHERE group_id = $1`, groupID)
	})
	if err := repo.UpsertLearnedModerationRule(ctx, LearnedModerationRule{GroupID: groupID, Pattern: "a%_b"}); err != nil {
		t.Fatal(err)
	}
	if matched, err := repo.MatchLearnedModerationRules(ctx, groupID, "axxxxb", 10); err != nil || len(matched) != 0 {
		t.Fatalf("wildcard pattern matched unrelated text: %+v err=%v", matched, err)
	}
	if matched, err := repo.MatchLearnedModerationRules(ctx, groupID, "xx a%_b yy", 10); err != nil || len(matched) != 1 {
		t.Fatalf("literal pattern not matched: %+v err=%v", matched, err)
	}
}

func TestSearchPublicGroupsTreatsWildcardsLiterally(t *testing.T) {
	repo, pool, ctx := openIntegrationRepo(t)
	owner := createIntegrationUser(t, ctx, repo, pool, "")
	marker := integrationID(t)
	for _, title := range []string{"lit" + marker + "_x", "lit" + marker + "ax"} {
		group, err := repo.CreateGroup(ctx, domain.Group{ID: "G-IT-" + integrationID(t), OwnerID: owner.ID, Title: title, Visibility: domain.VisibilityPublic})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM groups WHERE id = $1`, group.ID) })
	}
	groups, err := repo.SearchPublicGroups(ctx, marker+"_x")
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 || groups[0].Title != "lit"+marker+"_x" {
		t.Fatalf("underscore must match literally, got %+v", groups)
	}
}

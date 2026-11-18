package e2e_test

// End-to-end API tests: the real HTTP handler (httpapi.New) wired to a real,
// migrated Postgres. They are skipped unless TEST_DATABASE_URL is set, e.g.
//
//	TEST_DATABASE_URL=postgres://user:pass@localhost:5432/db?sslmode=disable go test -p 1 ./internal/e2e/...
//
// Migrations are applied by the suite itself (idempotent). Sign-in uses the
// local test-auth path (TEST_AUTH_* style config in a "test" environment) and
// one store-review (APP_REVIEW_*) account for the stored-code flow, so no SMS
// is sent. Push uses the no-op notifier; moderation is a deterministic fake
// that holds content containing heldMarker for review.

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/httpapi"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/moderation"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/service"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/storage"
)

const (
	testAuthCode  = "111111"
	appReviewCode = "246810"
	// heldMarker makes the fake moderator hold content for manual review.
	heldMarker = "E2E-HOLD"
	phonePool  = 400
)

type suiteEnv struct {
	db             *storage.Postgres
	repo           *storage.Repository
	svc            *service.Service
	phoneAuth      *service.PhoneAuthService
	phones         []string
	nextPhone      atomic.Int64
	adminPhone     string
	superPhone     string
	appReviewPhone string
}

var env *suiteEnv

// markerModerator allows everything except text containing heldMarker, which
// it sends to the moderation queue (ActionReview).
type markerModerator struct{}

func (markerModerator) Moderate(_ context.Context, input moderation.Input) (moderation.Decision, error) {
	if strings.Contains(input.Title+"\n"+input.Body, heldMarker) {
		return moderation.NewDecision(moderation.ActionReview, "e2e", "e2e_marker"), nil
	}
	return moderation.NewDecision(moderation.ActionAllow, "e2e"), nil
}

func TestMain(m *testing.M) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		// Every test calls requireEnv and skips.
		os.Exit(m.Run())
	}
	code, err := runSuite(m, databaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e setup failed: %v\n", err)
		os.Exit(1)
	}
	os.Exit(code)
}

func runSuite(m *testing.M, databaseURL string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	fileDir, err := os.MkdirTemp("", "koom-e2e-files-")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(fileDir)
	if err := os.Setenv("PUBLIC_FILE_DIR", fileDir); err != nil {
		return 0, err
	}

	db, err := storage.Open(ctx, databaseURL)
	if err != nil {
		return 0, err
	}
	defer db.Close()
	if err := db.RunMigrations(ctx, migrationsDir()); err != nil {
		return 0, fmt.Errorf("migrations: %w", err)
	}
	repo := storage.NewRepository(db.Pool)
	if err := repo.EnsureContentModerationSchema(ctx); err != nil {
		return 0, fmt.Errorf("moderation schema: %w", err)
	}

	// A per-run phone prefix keeps runs against the same database independent.
	var seed [4]byte
	if _, err := rand.Read(seed[:]); err != nil {
		return 0, err
	}
	prefix := fmt.Sprintf("+1998%03d", binary.BigEndian.Uint32(seed[:])%1000)
	phones := make([]string, phonePool)
	for i := range phones {
		phones[i] = fmt.Sprintf("%s%04d", prefix, i)
	}
	adminPhone := phones[0]
	superPhone := phones[1]
	appReviewPhone := fmt.Sprintf("+1997%03d%04d", binary.BigEndian.Uint32(seed[:])%1000, 1)

	// Grant platform_admin through the allowlist without disabling other
	// entries (SyncAdminPhoneAllowlist would).
	for phone, role := range map[string]string{adminPhone: "platform_admin", superPhone: "super_admin"} {
		if _, err := db.Pool.Exec(ctx, `
			INSERT INTO admin_phone_allowlist (phone, role, enabled)
			VALUES ($1, $2, true)
			ON CONFLICT (phone) DO UPDATE SET role = EXCLUDED.role, enabled = true, updated_at = now()`, phone, role); err != nil {
			return 0, fmt.Errorf("admin allowlist: %w", err)
		}
	}
	defer func() {
		_, _ = db.Pool.Exec(context.Background(), `DELETE FROM admin_phone_allowlist WHERE phone = ANY($1)`, []string{adminPhone, superPhone})
	}()

	jwtSecret := "e2e-jwt-secret-" + prefix + "-0123456789abcdef0123456789abcdef"
	svc := service.New(repo, service.Config{JWTSecret: jwtSecret, AccessTokenTTL: 15 * time.Minute, BCryptCost: 4}, service.NoopNotifier{})
	svc.SetContentModerator(markerModerator{})
	phoneAuth := service.NewPhoneAuth(repo, service.PhoneAuthConfig{
		JWTSecret:           jwtSecret,
		AccessTokenTTL:      15 * time.Minute,
		Environment:         "test",
		TestAuthEnabled:     true,
		TestAuthPhone:       strings.Join(phones, ","),
		TestAuthCode:        testAuthCode,
		TestAuthDisplayName: "E2E User",
		AppReviewPhone:      appReviewPhone,
		AppReviewCode:       appReviewCode,
	}, nil)

	env = &suiteEnv{db: db, repo: repo, svc: svc, phoneAuth: phoneAuth, phones: phones, adminPhone: adminPhone, superPhone: superPhone, appReviewPhone: appReviewPhone}
	env.nextPhone.Store(2) // phones[0] platform admin, phones[1] super admin
	return m.Run(), nil
}

func migrationsDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "migrations")
}

func requireEnv(t *testing.T) *suiteEnv {
	t.Helper()
	if env == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	return env
}

func (e *suiteEnv) newPhone(t *testing.T) string {
	t.Helper()
	i := e.nextPhone.Add(1) - 1
	if int(i) >= len(e.phones) {
		t.Fatalf("e2e phone pool exhausted")
	}
	return e.phones[i]
}

// api is one in-process server. Each test gets its own so per-IP rate limits
// (which key on 127.0.0.1 here) do not leak between tests.
type api struct {
	t   *testing.T
	srv *httptest.Server
}

type errorLogWriter struct {
	mu     sync.Mutex
	closed bool
	t      *testing.T
}

func (w *errorLogWriter) Write(p []byte) (int, error) {
	line := string(p)
	if strings.Contains(line, "http error") || strings.Contains(line, "panic") {
		w.mu.Lock()
		if !w.closed {
			w.t.Logf("server: %s", strings.TrimSpace(line))
		}
		w.mu.Unlock()
	}
	return len(p), nil
}

func newAPI(t *testing.T) *api {
	t.Helper()
	e := requireEnv(t)
	writer := &errorLogWriter{t: t}
	handler := httpapi.New(e.svc, e.phoneAuth, log.New(writer, "", 0), "")
	srv := httptest.NewServer(handler)
	t.Cleanup(func() {
		srv.Close()
		writer.mu.Lock()
		writer.closed = true
		writer.mu.Unlock()
	})
	return &api{t: t, srv: srv}
}

type user struct {
	api          *api
	ID           string
	Phone        string
	Name         string
	AccessToken  string
	RefreshToken string
}

type response struct {
	Status int
	Body   []byte
}

func (r response) json(t *testing.T, dst any) {
	t.Helper()
	if err := json.Unmarshal(r.Body, dst); err != nil {
		t.Fatalf("decode response %d %q: %v", r.Status, string(r.Body), err)
	}
}

func (r response) str() string {
	if len(r.Body) > 400 {
		return string(r.Body[:400]) + "..."
	}
	return string(r.Body)
}

func (a *api) raw(method, path, token, contentType string, body io.Reader) response {
	a.t.Helper()
	req, err := http.NewRequest(method, a.srv.URL+path, body)
	if err != nil {
		a.t.Fatalf("build request: %v", err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := a.srv.Client().Do(req)
	if err != nil {
		a.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	return response{Status: res.StatusCode, Body: data}
}

func (a *api) call(method, path, token string, body any) response {
	a.t.Helper()
	var reader io.Reader
	contentType := ""
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			a.t.Fatalf("encode body: %v", err)
		}
		reader = bytes.NewReader(data)
		contentType = "application/json"
	}
	return a.raw(method, path, token, contentType, reader)
}

func (u *user) do(method, path string, body any) response {
	u.api.t.Helper()
	return u.api.call(method, path, u.AccessToken, body)
}

// mustDo asserts the exact status.
func (u *user) mustDo(want int, method, path string, body any) response {
	u.api.t.Helper()
	res := u.do(method, path, body)
	if res.Status != want {
		u.api.t.Fatalf("%s %s as %s: status %d, want %d; body=%s", method, path, u.Name, res.Status, want, res.str())
	}
	return res
}

type sessionPayload struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	User         struct {
		ID          string `json:"id"`
		Mobile      string `json:"mobile"`
		DisplayName string `json:"display_name"`
		Role        string `json:"role"`
	} `json:"user"`
}

// signIn runs request-code + verify-code for a test-auth phone.
func (a *api) signIn(phone, name string) *user {
	a.t.Helper()
	res := a.call(http.MethodPost, "/api/auth/request-code", "", map[string]string{"mobile": phone})
	if res.Status != http.StatusOK {
		a.t.Fatalf("request-code %s: %d %s", phone, res.Status, res.str())
	}
	res = a.call(http.MethodPost, "/api/auth/verify-code", "", map[string]string{"mobile": phone, "code": testAuthCode, "display_name": name})
	if res.Status != http.StatusOK {
		a.t.Fatalf("verify-code %s: %d %s", phone, res.Status, res.str())
	}
	var session sessionPayload
	res.json(a.t, &session)
	if session.AccessToken == "" || session.RefreshToken == "" || session.User.ID == "" {
		a.t.Fatalf("incomplete session: %s", res.str())
	}
	return &user{api: a, ID: session.User.ID, Phone: phone, Name: name, AccessToken: session.AccessToken, RefreshToken: session.RefreshToken}
}

func (a *api) newUser(name string) *user {
	a.t.Helper()
	return a.signIn(env.newPhone(a.t), name)
}

func (a *api) platformAdmin() *user {
	a.t.Helper()
	return a.adminWithRole(env.adminPhone, "E2E Platform Admin", "platform_admin")
}

func (a *api) superAdmin() *user {
	a.t.Helper()
	return a.adminWithRole(env.superPhone, "E2E Super Admin", "super_admin")
}

func (a *api) adminWithRole(phone, name, role string) *user {
	a.t.Helper()
	admin := a.signIn(phone, name)
	res := admin.mustDo(http.StatusOK, http.MethodGet, "/api/me", nil)
	var me struct {
		Role string `json:"role"`
	}
	res.json(a.t, &me)
	if me.Role != role {
		a.t.Fatalf("%s did not get role %s: %s", name, role, res.str())
	}
	return admin
}

type groupPayload struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	OwnerID    string  `json:"owner_id"`
	Visibility string  `json:"visibility"`
	InviteCode string  `json:"invite_code"`
	MyRole     *string `json:"my_role"`
}

type creationRequestPayload struct {
	ID             string `json:"id"`
	RequesterID    string `json:"requester_id"`
	Status         string `json:"status"`
	CreatedGroupID string `json:"created_group_id"`
}

func groupCreationInput(title string) map[string]string {
	return map[string]string{
		"applicant_name":    "E2E Applicant",
		"position":          "Chair",
		"organization_name": "E2E Organization",
		"organization_type": "school",
		"region":            "Bishkek",
		"official_phone":    "+996700000000",
		"official_email":    "e2e@example.invalid",
		"group_title":       title,
		"group_description": "created by the e2e suite",
		"reason":            "end-to-end test group",
	}
}

// createOwnedGroup runs the real flow: owner files a creation request, the
// platform admin approves it, and the owner finds the new group in its list.
func createOwnedGroup(t *testing.T, owner, admin *user, title string) groupPayload {
	t.Helper()
	res := owner.mustDo(http.StatusCreated, http.MethodPost, "/api/group-creation-requests", groupCreationInput(title))
	var request creationRequestPayload
	res.json(t, &request)
	var approved creationRequestPayload
	admin.mustDo(http.StatusOK, http.MethodPost, "/api/admin/group-creation-requests/"+request.ID+"/approve", map[string]string{"admin_comment": "ok"}).json(t, &approved)
	if approved.Status != "approved" || approved.CreatedGroupID == "" {
		t.Fatalf("approve: unexpected request state %+v", approved)
	}
	var groups []groupPayload
	owner.mustDo(http.StatusOK, http.MethodGet, "/api/groups", nil).json(t, &groups)
	for _, group := range groups {
		if group.ID == approved.CreatedGroupID {
			if group.OwnerID != owner.ID {
				t.Fatalf("group %s owner %s, want %s", group.ID, group.OwnerID, owner.ID)
			}
			return group
		}
	}
	t.Fatalf("approved group %q not in owner's group list", title)
	return groupPayload{}
}

// joinWithCode gets (or creates) the invite code as owner and joins with it.
func joinWithCode(t *testing.T, owner, joiner *user, groupID string) {
	t.Helper()
	var group groupPayload
	owner.mustDo(http.StatusOK, http.MethodPost, "/api/groups/"+groupID+"/invite-code", nil).json(t, &group)
	if group.InviteCode == "" {
		t.Fatalf("invite-code response has no code")
	}
	joiner.mustDo(http.StatusOK, http.MethodPost, "/api/groups/join-by-code", map[string]string{"invite_code": group.InviteCode})
}

type idPayload struct {
	ID string `json:"id"`
}

func createRequest(t *testing.T, author *user, groupID, title, mode string) string {
	t.Helper()
	var created idPayload
	author.mustDo(http.StatusCreated, http.MethodPost, "/api/groups/"+groupID+"/requests", map[string]string{
		"request_type":     "suggestion",
		"interaction_mode": mode,
		"title":            title,
		"body":             "Body text for " + title,
	}).json(t, &created)
	return created.ID
}

func sendMessage(t *testing.T, sender *user, groupID, text string) string {
	t.Helper()
	var created idPayload
	sender.mustDo(http.StatusCreated, http.MethodPost, "/api/groups/"+groupID+"/messages", map[string]string{"text": text}).json(t, &created)
	return created.ID
}

func createComment(t *testing.T, author *user, requestID, body string) string {
	t.Helper()
	var created idPayload
	author.mustDo(http.StatusCreated, http.MethodPost, "/api/requests/"+requestID+"/comments", map[string]string{"body": body}).json(t, &created)
	return created.ID
}

// pngBytes is a minimal PNG header; the upload handler sniffs magic bytes only.
func pngBytes() []byte {
	return append([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}, bytes.Repeat([]byte{0}, 64)...)
}

func uploadFile(u *user, groupID string) response {
	u.api.t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	_ = writer.WriteField("kind", "photo")
	part, _ := writer.CreateFormFile("file", "e2e.png")
	_, _ = part.Write(pngBytes())
	_ = writer.Close()
	return u.api.raw(http.MethodPost, "/api/groups/"+groupID+"/files", u.AccessToken, writer.FormDataContentType(), &buf)
}

func uniqueSuffix() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%x", b)
}

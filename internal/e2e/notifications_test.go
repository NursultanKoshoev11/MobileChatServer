package e2e_test

// End-to-end tests for push notifications and realtime (WebSocket) delivery.
//
// They build the real HTTP handler in-process against a migrated Postgres
// database and replace FCM with a recording notifier. They are skipped unless
// TEST_DATABASE_URL points at a disposable database, for example:
//
//	docker run -d --name koom-notif-pg -e POSTGRES_PASSWORD=notif -e POSTGRES_USER=notif \
//	  -e POSTGRES_DB=notif -p 55471:5432 postgres:16-alpine
//	TEST_DATABASE_URL=postgres://notif:notif@localhost:55471/notif?sslmode=disable \
//	  go test ./internal/e2e/ -run Notif -v
//
// Migrations are applied by the tests themselves.

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/httpapi"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/moderation"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/service"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/storage"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	notifAppReviewCode = "111111"
	notifJWTSecret     = "notif-e2e-secret-0123456789abcdef0123456789abcdef"
	// notifSettle is how long negative assertions wait for stray background
	// pushes or socket events before concluding that none were sent.
	notifSettle  = 700 * time.Millisecond
	notifTimeout = 5 * time.Second
	// notifHoldMarker makes the test moderator queue content for review.
	notifHoldMarker = "NOTIF-HOLD-FOR-REVIEW"

	notifTitleNewPost       = "Новая публикация"
	notifTitleMessageEdited = "Сообщение изменено"
	notifTitlePendingReview = "Контент ожидает проверки"
)

// ---------------------------------------------------------------------------
// Recording notifier

type notifPush struct {
	Token string
	Title string
	Body  string
	Data  map[string]string
}

type notifRecorder struct {
	mu    sync.Mutex
	sends []notifPush
}

func (n *notifRecorder) SendToTokens(_ context.Context, tokens []string, message service.PushMessage) (service.PushSendResult, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	seen := map[string]bool{}
	for _, token := range tokens {
		if seen[token] {
			continue
		}
		seen[token] = true
		data := map[string]string{}
		for k, v := range message.Data {
			data[k] = v
		}
		n.sends = append(n.sends, notifPush{Token: token, Title: message.Title, Body: message.Body, Data: data})
	}
	return service.PushSendResult{}, nil
}

func (n *notifRecorder) all() []notifPush {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]notifPush(nil), n.sends...)
}

func (n *notifRecorder) to(token string, match func(notifPush) bool) []notifPush {
	out := make([]notifPush, 0)
	for _, p := range n.all() {
		if p.Token == token && (match == nil || match(p)) {
			out = append(out, p)
		}
	}
	return out
}

// waitFor waits until token received at least one push matching match.
func (n *notifRecorder) waitFor(t *testing.T, who string, token string, match func(notifPush) bool) notifPush {
	t.Helper()
	deadline := time.Now().Add(notifTimeout)
	for time.Now().Before(deadline) {
		if got := n.to(token, match); len(got) > 0 {
			return got[0]
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("%s: expected a push, none arrived; all pushes: %+v", who, n.all())
	return notifPush{}
}

func notifPushOfType(typ, key, id string) func(notifPush) bool {
	return func(p notifPush) bool {
		return p.Data["type"] == typ && (key == "" || p.Data[key] == id)
	}
}

// ---------------------------------------------------------------------------
// Test moderator: holds content containing notifHoldMarker for review.

type notifModerator struct{}

func (notifModerator) Moderate(_ context.Context, input moderation.Input) (moderation.Decision, error) {
	if strings.Contains(input.Title+" "+input.Body, notifHoldMarker) {
		return moderation.NewDecision(moderation.ActionReview, "notif-test", "held_by_test"), nil
	}
	return moderation.NewDecision(moderation.ActionAllow, "notif-test"), nil
}

// ---------------------------------------------------------------------------
// Environment

type notifEnv struct {
	t        *testing.T
	pool     *pgxpool.Pool
	svc      *service.Service
	server   *httptest.Server
	push     *notifRecorder
	phones   []string
	nextFree int
	users    []*notifUser
}

type notifUser struct {
	ID        string
	Name      string
	Phone     string
	Access    string
	PushToken string
}

var (
	notifMigrateOnce sync.Once
	notifMigrateErr  error
)

func notifRandHex(t *testing.T, n int) string {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

func notifRandomPhone(t *testing.T) string {
	t.Helper()
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatal(err)
	}
	v := (uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])) % 10_000_000
	return fmt.Sprintf("+1778%07d", v)
}

func notifMigrationsDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "migrations")
}

func notifNewEnv(t *testing.T) *notifEnv {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	notifMigrateOnce.Do(func() {
		db, err := storage.Open(ctx, databaseURL)
		if err != nil {
			notifMigrateErr = err
			return
		}
		defer db.Close()
		if err := db.RunMigrations(ctx, notifMigrationsDir()); err != nil {
			notifMigrateErr = err
			return
		}
		notifMigrateErr = storage.NewRepository(db.Pool).EnsureContentModerationSchema(ctx)
	})
	if notifMigrateErr != nil {
		t.Fatalf("migrate test database: %v", notifMigrateErr)
	}

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	t.Cleanup(pool.Close)

	phones := make([]string, 10)
	for i := range phones {
		phones[i] = notifRandomPhone(t)
	}

	repo := storage.NewRepository(pool)
	recorder := &notifRecorder{}
	svc := service.New(repo, service.Config{JWTSecret: notifJWTSecret, AccessTokenTTL: time.Hour, BCryptCost: 4}, recorder)
	svc.SetContentModerator(notifModerator{})
	phoneAuth := service.NewPhoneAuth(repo, service.PhoneAuthConfig{
		JWTSecret:          notifJWTSecret,
		AccessTokenTTL:     time.Hour,
		Environment:        "test",
		AppReviewPhone:     strings.Join(phones, ","),
		AppReviewCode:      notifAppReviewCode,
		SMSGlobalHourlyMax: 1_000_000,
	}, nil)

	logger := log.New(io.Discard, "", 0)
	if os.Getenv("NOTIF_E2E_VERBOSE") != "" {
		logger = log.New(os.Stderr, "server ", 0)
	}
	handler := httpapi.New(svc, phoneAuth, logger, "*")
	server := httptest.NewServer(handler)

	env := &notifEnv{t: t, pool: pool, svc: svc, server: server, push: recorder, phones: phones}
	t.Cleanup(func() {
		server.CloseClientConnections()
		server.Close()
		// Let background push goroutines finish before the pool closes.
		time.Sleep(100 * time.Millisecond)
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		for _, u := range env.users {
			_, _ = pool.Exec(cleanupCtx, `DELETE FROM groups WHERE owner_id = $1`, u.ID)
			_, _ = pool.Exec(cleanupCtx, `DELETE FROM users WHERE id = $1`, u.ID)
		}
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM phone_verification_codes WHERE phone_number = ANY($1)`, phones)
	})
	return env
}

// ---------------------------------------------------------------------------
// HTTP helpers

func (e *notifEnv) do(method, path, access string, body any) (int, []byte) {
	e.t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			e.t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, e.server.URL+path, reader)
	if err != nil {
		e.t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if access != "" {
		req.Header.Set("Authorization", "Bearer "+access)
	}
	res, err := e.server.Client().Do(req)
	if err != nil {
		e.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	return res.StatusCode, raw
}

func (e *notifEnv) mustDo(method, path, access string, body any, wantStatus int, out any) {
	e.t.Helper()
	status, raw := e.do(method, path, access, body)
	if status != wantStatus {
		e.t.Fatalf("%s %s: status %d, want %d; body=%s", method, path, status, wantStatus, raw)
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			e.t.Fatalf("%s %s: decode %s: %v", method, path, raw, err)
		}
	}
}

// signIn signs a new user in through phone auth with a store-review number and
// registers a distinct push token for them.
func (e *notifEnv) signIn(name string) *notifUser {
	e.t.Helper()
	if e.nextFree >= len(e.phones) {
		e.t.Fatal("notif env ran out of review phone numbers")
	}
	phone := e.phones[e.nextFree]
	e.nextFree++
	e.mustDo(http.MethodPost, "/api/auth/request-code", "", map[string]string{"mobile": phone}, http.StatusOK, nil)
	var session struct {
		AccessToken string `json:"access_token"`
		User        struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	e.mustDo(http.MethodPost, "/api/auth/verify-code", "", map[string]string{"mobile": phone, "code": notifAppReviewCode, "display_name": name}, http.StatusOK, &session)
	if session.AccessToken == "" || session.User.ID == "" {
		e.t.Fatalf("sign in %s: empty session", name)
	}
	user := &notifUser{ID: session.User.ID, Name: name, Phone: phone, Access: session.AccessToken}
	e.users = append(e.users, user)
	user.PushToken = "notif-push-" + strings.ToLower(strings.Fields(name)[0]) + "-" + notifRandHex(e.t, 8)
	e.registerToken(user, user.PushToken)
	return user
}

func (e *notifEnv) registerToken(user *notifUser, token string) {
	e.t.Helper()
	e.mustDo(http.MethodPost, "/api/push/register", user.Access, map[string]string{"token": token, "platform": "android"}, http.StatusOK, nil)
}

// createGroup creates a private group owned by owner. Group creation is limited
// to the super admin, so owner is promoted only for the creation call.
func (e *notifEnv) createGroup(owner *notifUser, title string) string {
	e.t.Helper()
	ctx := context.Background()
	if _, err := e.pool.Exec(ctx, `UPDATE users SET role = 'super_admin' WHERE id = $1`, owner.ID); err != nil {
		e.t.Fatal(err)
	}
	var group struct {
		ID string `json:"id"`
	}
	e.mustDo(http.MethodPost, "/api/groups", owner.Access, map[string]string{"title": title, "description": "notification e2e", "visibility": "private"}, http.StatusCreated, &group)
	if _, err := e.pool.Exec(ctx, `UPDATE users SET role = 'user' WHERE id = $1`, owner.ID); err != nil {
		e.t.Fatal(err)
	}
	return group.ID
}

func (e *notifEnv) join(groupID string, owner *notifUser, members ...*notifUser) {
	e.t.Helper()
	var group struct {
		InviteCode string `json:"invite_code"`
	}
	e.mustDo(http.MethodPost, "/api/groups/"+groupID+"/invite-code", owner.Access, nil, http.StatusOK, &group)
	if group.InviteCode == "" {
		e.t.Fatal("empty invite code")
	}
	for _, m := range members {
		e.mustDo(http.MethodPost, "/api/groups/join-by-code", m.Access, map[string]string{"invite_code": group.InviteCode}, http.StatusOK, nil)
	}
}

type notifID struct {
	ID string `json:"id"`
}

func (e *notifEnv) createPost(author *notifUser, groupID, title, body string) string {
	e.t.Helper()
	var out notifID
	e.mustDo(http.MethodPost, "/api/groups/"+groupID+"/requests", author.Access, map[string]string{"request_type": "announcement", "title": title, "body": body}, http.StatusCreated, &out)
	return out.ID
}

func (e *notifEnv) sendMessage(author *notifUser, groupID, text string) string {
	e.t.Helper()
	var out notifID
	e.mustDo(http.MethodPost, "/api/groups/"+groupID+"/messages", author.Access, map[string]string{"text": text}, http.StatusCreated, &out)
	return out.ID
}

func (e *notifEnv) comment(author *notifUser, requestID, body string) string {
	e.t.Helper()
	var out notifID
	e.mustDo(http.MethodPost, "/api/requests/"+requestID+"/comments", author.Access, map[string]string{"body": body}, http.StatusCreated, &out)
	return out.ID
}

// assertNoPrivateData checks every recorded push for phone numbers.
func (e *notifEnv) assertNoPrivateData() {
	e.t.Helper()
	for _, p := range e.push.all() {
		raw, _ := json.Marshal(p)
		for _, u := range e.users {
			if strings.Contains(string(raw), u.Phone) || strings.Contains(string(raw), strings.TrimPrefix(u.Phone, "+")) {
				e.t.Errorf("push to %s leaks phone number of %s: %s", p.Token, u.Name, raw)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// WebSocket helpers

type notifEvent struct {
	ID      string          `json:"id"`
	Type    string          `json:"type"`
	GroupID string          `json:"group_id"`
	Payload json.RawMessage `json:"payload"`
	raw     string
}

type notifSocket struct {
	name   string
	conn   *websocket.Conn
	events chan notifEvent
	done   chan struct{}
	mu     sync.Mutex
	seen   []notifEvent
}

func (e *notifEnv) wsToken(user *notifUser) string {
	e.t.Helper()
	var out struct {
		Token string `json:"token"`
	}
	e.mustDo(http.MethodPost, "/api/ws-token", user.Access, nil, http.StatusOK, &out)
	return out.Token
}

func (e *notifEnv) dialWS(user *notifUser, path string) (*websocket.Conn, *http.Response, error) {
	e.t.Helper()
	return e.dialWSWithToken(e.wsToken(user), path)
}

func (e *notifEnv) dialWSWithToken(wsToken, path string) (*websocket.Conn, *http.Response, error) {
	e.t.Helper()
	url := "ws" + strings.TrimPrefix(e.server.URL, "http") + path
	dialer := websocket.Dialer{Subprotocols: []string{"koom-ws", wsToken}, HandshakeTimeout: notifTimeout}
	return dialer.Dial(url, nil)
}

// connect opens a group socket (groupID != "") or the per-user socket.
func (e *notifEnv) connect(user *notifUser, groupID string) *notifSocket {
	e.t.Helper()
	path := "/api/ws"
	name := user.Name + "/user-ws"
	if groupID != "" {
		path = "/api/groups/" + groupID + "/ws"
		name = user.Name + "/group-ws"
	}
	conn, res, err := e.dialWS(user, path)
	if err != nil {
		status := 0
		if res != nil {
			status = res.StatusCode
		}
		e.t.Fatalf("%s: dial failed status=%d: %v", name, status, err)
	}
	s := &notifSocket{name: name, conn: conn, events: make(chan notifEvent, 512), done: make(chan struct{})}
	go func() {
		defer close(s.done)
		for {
			_, raw, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var ev notifEvent
			if json.Unmarshal(raw, &ev) == nil {
				ev.raw = string(raw)
				s.events <- ev
			}
		}
	}()
	e.t.Cleanup(func() { _ = conn.Close() })
	s.waitFor(e.t, "connection.ready", nil)
	return s
}

func (s *notifSocket) drain() {
	for {
		select {
		case ev := <-s.events:
			s.mu.Lock()
			s.seen = append(s.seen, ev)
			s.mu.Unlock()
		default:
			return
		}
	}
}

func (s *notifSocket) find(typ string, match func(notifEvent) bool) []notifEvent {
	s.drain()
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]notifEvent, 0)
	for _, ev := range s.seen {
		if ev.Type == typ && (match == nil || match(ev)) {
			out = append(out, ev)
		}
	}
	return out
}

func (s *notifSocket) waitFor(t *testing.T, typ string, match func(notifEvent) bool) notifEvent {
	t.Helper()
	deadline := time.Now().Add(notifTimeout)
	for time.Now().Before(deadline) {
		if got := s.find(typ, match); len(got) > 0 {
			return got[0]
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("%s: expected realtime event %q, got %v", s.name, typ, s.types())
	return notifEvent{}
}

func (s *notifSocket) assertNone(t *testing.T, typ string, match func(notifEvent) bool) {
	t.Helper()
	if got := s.find(typ, match); len(got) > 0 {
		t.Errorf("%s: unexpected realtime event %q: %s", s.name, typ, got[0].raw)
	}
}

func (s *notifSocket) types() []string {
	s.drain()
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.seen))
	for _, ev := range s.seen {
		out = append(out, ev.Type)
	}
	return out
}

func (s *notifSocket) waitClosed(t *testing.T) {
	t.Helper()
	select {
	case <-s.done:
	case <-time.After(notifTimeout):
		t.Fatalf("%s: socket was not closed by the server", s.name)
	}
}

func (s *notifSocket) assertNoPrivateData(t *testing.T, users []*notifUser) {
	t.Helper()
	s.drain()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ev := range s.seen {
		for _, u := range users {
			if strings.Contains(ev.raw, u.Phone) || strings.Contains(ev.raw, strings.TrimPrefix(u.Phone, "+")) {
				t.Errorf("%s: realtime event %q leaks phone number of %s: %s", s.name, ev.Type, u.Name, ev.raw)
			}
		}
	}
}

func notifPayloadHas(field, value string) func(notifEvent) bool {
	return func(ev notifEvent) bool {
		var payload map[string]any
		if json.Unmarshal(ev.Payload, &payload) != nil {
			return false
		}
		if v, ok := payload[field].(string); ok && v == value {
			return true
		}
		// comment events nest the comment object
		if c, ok := payload["comment"].(map[string]any); ok {
			if v, ok := c[field].(string); ok && v == value {
				return true
			}
		}
		return false
	}
}

// ---------------------------------------------------------------------------
// Tests

// TestNotifGroupScenario: group with members A (owner), B, C and outsider D.
func TestNotifGroupScenario(t *testing.T) {
	env := notifNewEnv(t)
	a := env.signIn("Alice Notif")
	b := env.signIn("Bob Notif")
	c := env.signIn("Carol Notif")
	d := env.signIn("Dave Outsider")

	groupTitle := "Notif Group " + notifRandHex(t, 3)
	groupID := env.createGroup(a, groupTitle)
	env.join(groupID, a, b, c)

	wsA := env.connect(a, groupID)
	wsB := env.connect(b, groupID)
	wsC := env.connect(c, groupID)
	userWsB := env.connect(b, "")
	userWsC := env.connect(c, "")
	userWsD := env.connect(d, "")

	// The outsider cannot subscribe to the group's socket at all.
	if conn, res, err := env.dialWS(d, "/api/groups/"+groupID+"/ws"); err == nil {
		_ = conn.Close()
		t.Fatal("outsider D connected to the group socket")
	} else if res == nil || (res.StatusCode != http.StatusForbidden && res.StatusCode != http.StatusNotFound) {
		status := 0
		if res != nil {
			status = res.StatusCode
		}
		t.Fatalf("outsider D group socket: want 403/404, got status=%d err=%v", status, err)
	}

	// 1. A creates a post.
	postTitle := "Water outage notice"
	postBody := "No water on Monday from 9 to 12"
	postID := env.createPost(a, groupID, postTitle, postBody)
	isPost := notifPushOfType("public_request.created", "request_id", postID)
	for _, u := range []*notifUser{b, c} {
		p := env.push.waitFor(t, u.Name, u.PushToken, isPost)
		if p.Title != notifTitleNewPost {
			t.Errorf("post push title to %s = %q, want %q", u.Name, p.Title, notifTitleNewPost)
		}
		if want := postTitle + ": " + postBody; p.Body != want {
			t.Errorf("post push body to %s = %q, want %q", u.Name, p.Body, want)
		}
		if p.Data["group_id"] != groupID {
			t.Errorf("post push group_id to %s = %q, want %q", u.Name, p.Data["group_id"], groupID)
		}
	}
	for _, s := range []*notifSocket{wsA, wsB, wsC, userWsB, userWsC} {
		ev := s.waitFor(t, "public_request.created", notifPayloadHas("id", postID))
		if ev.GroupID != groupID {
			t.Errorf("%s: event group_id = %q, want %q", s.name, ev.GroupID, groupID)
		}
	}

	// 2. A sends a chat message.
	messageText := "Meeting moved to Friday"
	messageID := env.sendMessage(a, groupID, messageText)
	for _, s := range []*notifSocket{wsA, wsB, wsC, userWsB, userWsC} {
		s.waitFor(t, "message.created", notifPayloadHas("id", messageID))
	}

	// 3. A comments on the post.
	commentID := env.comment(a, postID, "Thanks for reading")
	for _, s := range []*notifSocket{wsA, wsB, wsC, userWsB, userWsC} {
		s.waitFor(t, "public_request.comment_created", notifPayloadHas("id", commentID))
	}

	// 4. A edits the chat message: members are notified.
	editedText := "Meeting moved to Saturday"
	env.mustDo(http.MethodPatch, "/api/groups/"+groupID+"/messages/"+messageID, a.Access, map[string]string{"text": editedText}, http.StatusOK, nil)
	isEdit := notifPushOfType("message.updated", "message_id", messageID)
	for _, u := range []*notifUser{b, c} {
		p := env.push.waitFor(t, u.Name, u.PushToken, isEdit)
		if p.Title != notifTitleMessageEdited {
			t.Errorf("edit push title to %s = %q, want %q", u.Name, p.Title, notifTitleMessageEdited)
		}
		if want := a.Name + ": " + editedText; p.Body != want {
			t.Errorf("edit push body to %s = %q, want %q", u.Name, p.Body, want)
		}
	}
	for _, s := range []*notifSocket{wsA, wsB, wsC, userWsB, userWsC} {
		s.waitFor(t, "message.updated", notifPayloadHas("id", messageID))
	}

	time.Sleep(notifSettle)

	// Author and outsider never get a push; members get each push exactly once.
	if got := env.push.to(a.PushToken, nil); len(got) != 0 {
		t.Errorf("author A received pushes: %+v", got)
	}
	if got := env.push.to(d.PushToken, nil); len(got) != 0 {
		t.Errorf("outsider D received pushes: %+v", got)
	}
	for _, u := range []*notifUser{b, c} {
		if got := env.push.to(u.PushToken, isPost); len(got) != 1 {
			t.Errorf("%s received %d post pushes, want 1", u.Name, len(got))
		}
		if got := env.push.to(u.PushToken, isEdit); len(got) != 1 {
			t.Errorf("%s received %d edit pushes, want 1", u.Name, len(got))
		}
	}
	// Chat message creation and comments currently send no push at all (see
	// report: "needs decision"). Whatever is decided, they must never reach the
	// author or an outsider, which the checks above cover.
	for _, u := range []*notifUser{b, c} {
		for _, p := range env.push.to(u.PushToken, nil) {
			if p.Data["group_id"] != groupID {
				t.Errorf("%s got a push for another group: %+v", u.Name, p)
			}
		}
	}

	// Each realtime event is delivered once per socket.
	for _, s := range []*notifSocket{wsA, wsB, wsC, userWsB, userWsC} {
		if got := s.find("public_request.created", notifPayloadHas("id", postID)); len(got) != 1 {
			t.Errorf("%s: public_request.created delivered %d times", s.name, len(got))
		}
		if got := s.find("message.created", notifPayloadHas("id", messageID)); len(got) != 1 {
			t.Errorf("%s: message.created delivered %d times", s.name, len(got))
		}
	}
	// The outsider's own socket receives nothing about the group.
	for _, typ := range userWsD.types() {
		if typ != "connection.ready" {
			t.Errorf("outsider D user socket received %q", typ)
		}
	}

	// 5. C leaves the group: C's group socket is closed and C gets nothing more.
	env.mustDo(http.MethodDelete, "/api/groups/"+groupID+"/leave", c.Access, nil, http.StatusOK, nil)
	wsC.waitClosed(t)

	secondPostID := env.createPost(a, groupID, "Second notice", "Repairs finished, water is back")
	isSecond := notifPushOfType("public_request.created", "request_id", secondPostID)
	env.push.waitFor(t, b.Name, b.PushToken, isSecond)
	wsA.waitFor(t, "public_request.created", notifPayloadHas("id", secondPostID))
	wsB.waitFor(t, "public_request.created", notifPayloadHas("id", secondPostID))
	userWsB.waitFor(t, "public_request.created", notifPayloadHas("id", secondPostID))
	time.Sleep(notifSettle)
	if got := env.push.to(c.PushToken, isSecond); len(got) != 0 {
		t.Errorf("removed member C received a push: %+v", got)
	}
	if got := env.push.to(a.PushToken, nil); len(got) != 0 {
		t.Errorf("author A received pushes: %+v", got)
	}
	if got := env.push.to(d.PushToken, nil); len(got) != 0 {
		t.Errorf("outsider D received pushes: %+v", got)
	}
	wsC.assertNone(t, "public_request.created", notifPayloadHas("id", secondPostID))
	userWsC.assertNone(t, "public_request.created", notifPayloadHas("id", secondPostID))

	// C can no longer open the group socket.
	if conn, _, err := env.dialWS(c, "/api/groups/"+groupID+"/ws"); err == nil {
		_ = conn.Close()
		t.Error("removed member C reconnected to the group socket")
	}

	env.assertNoPrivateData()
	for _, s := range []*notifSocket{wsA, wsB, wsC, userWsB, userWsC, userWsD} {
		s.assertNoPrivateData(t, env.users)
	}
}

// TestNotifBlockedAuthorDoesNotNotifyBlocker: B blocks A; A's content must not
// notify B (push or realtime), while C is still notified.
func TestNotifBlockedAuthorDoesNotNotifyBlocker(t *testing.T) {
	env := notifNewEnv(t)
	a := env.signIn("Alice Blocked")
	b := env.signIn("Bob Blocker")
	c := env.signIn("Carol Member")
	groupID := env.createGroup(a, "Notif Block "+notifRandHex(t, 3))
	env.join(groupID, a, b, c)

	env.mustDo(http.MethodPost, "/api/users/"+a.ID+"/block", b.Access, nil, http.StatusNoContent, nil)

	wsB := env.connect(b, groupID)
	userWsB := env.connect(b, "")
	wsC := env.connect(c, groupID)

	postID := env.createPost(a, groupID, "Blocked author post", "Content the blocker hid")
	messageID := env.sendMessage(a, groupID, "Message from blocked author")
	env.mustDo(http.MethodPatch, "/api/groups/"+groupID+"/messages/"+messageID, a.Access, map[string]string{"text": "Edited by blocked author"}, http.StatusOK, nil)

	isPost := notifPushOfType("public_request.created", "request_id", postID)
	isEdit := notifPushOfType("message.updated", "message_id", messageID)
	env.push.waitFor(t, c.Name, c.PushToken, isPost)
	env.push.waitFor(t, c.Name, c.PushToken, isEdit)
	wsC.waitFor(t, "public_request.created", notifPayloadHas("id", postID))
	wsC.waitFor(t, "message.created", notifPayloadHas("id", messageID))
	time.Sleep(notifSettle)

	if got := env.push.to(b.PushToken, nil); len(got) != 0 {
		t.Errorf("blocker B received pushes about the blocked author: %+v", got)
	}
	for _, s := range []*notifSocket{wsB, userWsB} {
		s.assertNone(t, "public_request.created", notifPayloadHas("id", postID))
		s.assertNone(t, "message.created", notifPayloadHas("id", messageID))
		s.assertNone(t, "message.updated", notifPayloadHas("id", messageID))
	}

	// Unblocking restores notifications.
	env.mustDo(http.MethodDelete, "/api/users/"+a.ID+"/block", b.Access, nil, http.StatusNoContent, nil)
	afterID := env.createPost(a, groupID, "After unblock", "Visible again to Bob")
	env.push.waitFor(t, b.Name, b.PushToken, notifPushOfType("public_request.created", "request_id", afterID))
	wsB.waitFor(t, "public_request.created", notifPayloadHas("id", afterID))
}

// TestNotifModerationHeldContent: content queued for review sends no member
// push or content event until approved, then exactly once.
func TestNotifModerationHeldContent(t *testing.T) {
	env := notifNewEnv(t)
	a := env.signIn("Alice Moderator")
	b := env.signIn("Bob Author")
	c := env.signIn("Carol Reader")
	d := env.signIn("Dave Outsider")
	groupID := env.createGroup(a, "Notif Moderation "+notifRandHex(t, 3))
	env.join(groupID, a, b, c)

	wsA := env.connect(a, groupID)
	wsC := env.connect(c, groupID)
	userWsC := env.connect(c, "")

	var pending struct {
		Status string `json:"status"`
		Item   struct {
			ID string `json:"id"`
		} `json:"moderation_item"`
	}
	env.mustDo(http.MethodPost, "/api/groups/"+groupID+"/requests", b.Access, map[string]string{
		"request_type": "announcement", "title": "Held post", "body": "Please review " + notifHoldMarker,
	}, http.StatusAccepted, &pending)
	if pending.Status != "pending_review" || pending.Item.ID == "" {
		t.Fatalf("expected pending_review, got %+v", pending)
	}
	// The group's moderators are told something awaits review.
	env.push.waitFor(t, a.Name, a.PushToken, notifPushOfType("content_moderation.pending_review", "item_id", pending.Item.ID))
	time.Sleep(notifSettle)

	isAnyPost := notifPushOfType("public_request.created", "", "")
	if got := env.push.all(); len(got) > 0 {
		for _, p := range got {
			if p.Data["type"] == "public_request.created" {
				t.Errorf("held content produced a member push before approval: %+v", p)
			}
			if p.Token == b.PushToken || p.Token == c.PushToken || p.Token == d.PushToken {
				t.Errorf("non-moderator received a push while content was held: %+v", p)
			}
			if p.Data["type"] == "content_moderation.pending_review" && p.Title != notifTitlePendingReview {
				t.Errorf("pending review push title = %q", p.Title)
			}
		}
	}
	for _, s := range []*notifSocket{wsA, wsC, userWsC} {
		s.assertNone(t, "public_request.created", nil)
	}
	// The pending event the group sees carries no content.
	ev := wsC.waitFor(t, "content_moderation.pending_review", nil)
	if strings.Contains(ev.raw, notifHoldMarker) || strings.Contains(ev.raw, "Held post") {
		t.Errorf("pending_review event leaks held content: %s", ev.raw)
	}

	var approved struct {
		PublicRequest struct {
			ID string `json:"id"`
		} `json:"public_request"`
	}
	env.mustDo(http.MethodPost, "/api/moderation/items/"+pending.Item.ID+"/approve", a.Access, nil, http.StatusOK, &approved)
	requestID := approved.PublicRequest.ID
	if requestID == "" {
		t.Fatal("approval returned no public request")
	}
	isPost := notifPushOfType("public_request.created", "request_id", requestID)
	for _, u := range []*notifUser{a, c} {
		p := env.push.waitFor(t, u.Name, u.PushToken, isPost)
		if p.Title != notifTitleNewPost || p.Body != "Held post: Please review "+notifHoldMarker {
			t.Errorf("approved post push to %s = %q / %q", u.Name, p.Title, p.Body)
		}
	}
	for _, s := range []*notifSocket{wsA, wsC, userWsC} {
		s.waitFor(t, "public_request.created", notifPayloadHas("id", requestID))
	}

	// A second approval is rejected and does not re-send.
	status, _ := env.do(http.MethodPost, "/api/moderation/items/"+pending.Item.ID+"/approve", a.Access, nil)
	if status == http.StatusOK {
		t.Error("second approval succeeded")
	}
	time.Sleep(notifSettle)
	for _, u := range []*notifUser{a, c} {
		if got := env.push.to(u.PushToken, isPost); len(got) != 1 {
			t.Errorf("%s received %d pushes for the approved post, want 1", u.Name, len(got))
		}
	}
	if got := env.push.to(b.PushToken, isAnyPost); len(got) != 0 {
		t.Errorf("author B received a push for own approved post: %+v", got)
	}
	if got := env.push.to(d.PushToken, nil); len(got) != 0 {
		t.Errorf("outsider D received pushes: %+v", got)
	}
	for _, s := range []*notifSocket{wsA, wsC, userWsC} {
		if got := s.find("public_request.created", notifPayloadHas("id", requestID)); len(got) != 1 {
			t.Errorf("%s: approved post event delivered %d times", s.name, len(got))
		}
	}
	env.assertNoPrivateData()
}

// TestNotifCommentMutedMember: a member muted from commenting cannot produce
// comment events or pushes, and comments never push to the muted member.
func TestNotifCommentMutedMember(t *testing.T) {
	env := notifNewEnv(t)
	a := env.signIn("Alice Owner")
	b := env.signIn("Bob Member")
	c := env.signIn("Carol Muted")
	groupID := env.createGroup(a, "Notif Mute "+notifRandHex(t, 3))
	env.join(groupID, a, b, c)

	postID := env.createPost(a, groupID, "Discussion topic", "Share your thoughts here")
	env.push.waitFor(t, c.Name, c.PushToken, notifPushOfType("public_request.created", "request_id", postID))

	env.mustDo(http.MethodPost, "/api/groups/"+groupID+"/comment-mutes/"+c.ID, a.Access, map[string]any{"duration_minutes": 60, "reason": "test"}, http.StatusOK, nil)

	wsA := env.connect(a, groupID)
	wsB := env.connect(b, groupID)
	before := len(env.push.all())

	status, raw := env.do(http.MethodPost, "/api/requests/"+postID+"/comments", c.Access, map[string]string{"body": "Muted comment attempt"})
	if status == http.StatusCreated {
		t.Fatalf("muted member was able to comment: %s", raw)
	}
	commentID := env.comment(a, postID, "Owner comment while Carol is muted")
	wsB.waitFor(t, "public_request.comment_created", notifPayloadHas("id", commentID))
	time.Sleep(notifSettle)

	for _, s := range []*notifSocket{wsA, wsB} {
		if got := s.find("public_request.comment_created", nil); len(got) != 1 {
			t.Errorf("%s: %d comment events, want only the owner's", s.name, len(got))
		}
	}
	if got := env.push.all()[before:]; len(got) != 0 {
		t.Errorf("comments produced pushes: %+v", got)
	}
}

// TestNotifDeletedAccountGetsNothing: a deleted account's tokens and sockets
// receive nothing afterwards.
func TestNotifDeletedAccountGetsNothing(t *testing.T) {
	env := notifNewEnv(t)
	a := env.signIn("Alice Owner")
	b := env.signIn("Bob Member")
	e := env.signIn("Eve Deleted")
	groupID := env.createGroup(a, "Notif Delete "+notifRandHex(t, 3))
	env.join(groupID, a, b, e)

	wsE := env.connect(e, groupID)
	userWsE := env.connect(e, "")
	staleWSToken := env.wsToken(e) // issued before deletion, still unexpired
	env.mustDo(http.MethodDelete, "/api/me", e.Access, nil, http.StatusOK, nil)
	wsE.waitClosed(t)
	userWsE.waitClosed(t)

	var count int
	if err := env.pool.QueryRow(context.Background(), `SELECT count(*) FROM push_tokens WHERE token = $1`, e.PushToken).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("deleted account still has %d push token row(s)", count)
	}

	postID := env.createPost(a, groupID, "After deletion", "Eve should not see this")
	env.push.waitFor(t, b.Name, b.PushToken, notifPushOfType("public_request.created", "request_id", postID))
	time.Sleep(notifSettle)
	if got := env.push.to(e.PushToken, nil); len(got) != 0 {
		t.Errorf("deleted account received pushes: %+v", got)
	}
	// A deleted account's unexpired socket token can no longer open a socket.
	for _, path := range []string{"/api/ws", "/api/groups/" + groupID + "/ws"} {
		if conn, _, err := env.dialWSWithToken(staleWSToken, path); err == nil {
			_ = conn.Close()
			t.Errorf("deleted account opened %s", path)
		}
	}
}

// TestNotifDuplicateTokenGoesToCurrentOwner: a device token re-registered by
// another account only notifies the account that registered it last.
func TestNotifDuplicateTokenGoesToCurrentOwner(t *testing.T) {
	env := notifNewEnv(t)
	a := env.signIn("Alice Owner")
	b := env.signIn("Bob Member")
	d := env.signIn("Dave Outsider")
	groupID := env.createGroup(a, "Notif Token "+notifRandHex(t, 3))
	env.join(groupID, a, b)

	shared := "notif-push-shared-" + notifRandHex(t, 8)
	env.registerToken(b, shared)
	env.registerToken(d, shared) // the device moved to outsider D

	firstID := env.createPost(a, groupID, "First shared token post", "Only Bob's own token should ring")
	env.push.waitFor(t, b.Name, b.PushToken, notifPushOfType("public_request.created", "request_id", firstID))
	time.Sleep(notifSettle)
	if got := env.push.to(shared, nil); len(got) != 0 {
		t.Errorf("token now owned by outsider D received a group push: %+v", got)
	}

	env.registerToken(b, shared) // the device moved back to member B
	secondID := env.createPost(a, groupID, "Second shared token post", "Shared device belongs to Bob again")
	isSecond := notifPushOfType("public_request.created", "request_id", secondID)
	env.push.waitFor(t, "shared token", shared, isSecond)
	time.Sleep(notifSettle)
	if got := env.push.to(shared, isSecond); len(got) != 1 {
		t.Errorf("shared token received %d pushes for one post, want 1", len(got))
	}
	var owners int
	if err := env.pool.QueryRow(context.Background(), `SELECT count(*) FROM push_tokens WHERE token = $1`, shared).Scan(&owners); err != nil {
		t.Fatal(err)
	}
	if owners != 1 {
		t.Errorf("shared token has %d owner rows, want 1", owners)
	}
}

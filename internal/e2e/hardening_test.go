package e2e_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"image"
	"image/jpeg"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// dialGroupWS opens the group WebSocket as u and consumes connection.ready.
func dialGroupWS(t *testing.T, a *api, u *user, groupID string) *websocket.Conn {
	t.Helper()
	var token struct {
		Token string `json:"token"`
	}
	u.mustDo(http.StatusOK, http.MethodPost, "/api/ws-token", nil).json(t, &token)
	wsURL := "ws" + strings.TrimPrefix(a.srv.URL, "http") + "/api/groups/" + groupID + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{"Sec-WebSocket-Protocol": {"koom-ws, " + token.Token}})
	if err != nil {
		t.Fatalf("dial group websocket as %s: %v", u.Name, err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var ready struct {
		Type string `json:"type"`
	}
	if err := conn.ReadJSON(&ready); err != nil || ready.Type != "connection.ready" {
		t.Fatalf("first event %+v err=%v", ready, err)
	}
	return conn
}

// expectSocketClosed reads until the server closes the socket.
func expectSocketClosed(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			if ne, ok := err.(interface{ Timeout() bool }); ok && ne.Timeout() {
				t.Fatalf("socket was not closed by the server")
			}
			return
		}
	}
}

func groupIDs(t *testing.T, u *user) map[string]groupPayload {
	t.Helper()
	var groups []groupPayload
	u.mustDo(http.StatusOK, http.MethodGet, "/api/groups", nil).json(t, &groups)
	out := map[string]groupPayload{}
	for _, g := range groups {
		out[g.ID] = g
	}
	return out
}

func TestE2ERemoveMember(t *testing.T) {
	a := newAPI(t)
	admin := a.platformAdmin()
	owner := a.newUser("RM Owner")
	groupAdmin := a.newUser("RM Admin")
	otherAdmin := a.newUser("RM Admin 2")
	member := a.newUser("RM Member")
	member2 := a.newUser("RM Member 2")
	outsider := a.newUser("RM Outsider")
	invitee := a.newUser("RM Invitee")
	group := createOwnedGroup(t, owner, admin, "Remove group "+uniqueSuffix())
	base := "/api/groups/" + group.ID
	for _, u := range []*user{groupAdmin, otherAdmin, member, member2} {
		joinWithCode(t, owner, u, group.ID)
	}
	for _, u := range []*user{groupAdmin, otherAdmin} {
		owner.mustDo(http.StatusOK, http.MethodPost, base+"/members/"+u.ID+"/role", map[string]string{"role": "admin"})
	}

	// Permission rules.
	member.mustDo(http.StatusForbidden, http.MethodDelete, base+"/members/"+member2.ID, nil)
	outsider.mustDo(http.StatusForbidden, http.MethodDelete, base+"/members/"+member2.ID, nil)
	groupAdmin.mustDo(http.StatusForbidden, http.MethodDelete, base+"/members/"+otherAdmin.ID, nil)
	groupAdmin.mustDo(http.StatusForbidden, http.MethodDelete, base+"/members/"+owner.ID, nil)
	groupAdmin.mustDo(http.StatusForbidden, http.MethodDelete, base+"/members/"+groupAdmin.ID, nil)
	owner.mustDo(http.StatusForbidden, http.MethodDelete, base+"/members/"+owner.ID, nil)
	owner.mustDo(http.StatusNotFound, http.MethodDelete, base+"/members/"+outsider.ID, nil)
	owner.mustDo(http.StatusNotFound, http.MethodDelete, base+"/members/U-DOES-NOT-EXIST", nil)

	// An admin removes a plain member: 204, realtime socket closed, access gone.
	conn := dialGroupWS(t, a, member, group.ID)
	defer conn.Close()
	res := groupAdmin.mustDo(http.StatusNoContent, http.MethodDelete, base+"/members/"+member.ID, nil)
	if len(res.Body) != 0 {
		t.Fatalf("204 response has a body: %q", res.Body)
	}
	expectSocketClosed(t, conn)
	member.mustDo(http.StatusForbidden, http.MethodGet, base+"/messages", nil)
	if _, ok := groupIDs(t, member)[group.ID]; ok {
		t.Fatal("removed member still lists the group")
	}
	groupAdmin.mustDo(http.StatusNotFound, http.MethodDelete, base+"/members/"+member.ID, nil)

	// The owner removes an admin; the admin's pending invites for the group go.
	otherAdmin.mustDo(http.StatusCreated, http.MethodPost, base+"/invite-user", map[string]string{"target_user_id": invitee.ID})
	var pending []struct {
		GroupID string `json:"group_id"`
	}
	invitee.mustDo(http.StatusOK, http.MethodGet, "/api/invites", nil).json(t, &pending)
	if len(pending) != 1 {
		t.Fatalf("invitee should have one pending invite: %+v", pending)
	}
	owner.mustDo(http.StatusNoContent, http.MethodDelete, base+"/members/"+otherAdmin.ID, nil)
	pending = nil
	invitee.mustDo(http.StatusOK, http.MethodGet, "/api/invites", nil).json(t, &pending)
	for _, invite := range pending {
		if invite.GroupID == group.ID {
			t.Fatalf("removed admin's pending invite survived: %+v", pending)
		}
	}
	// The removed user's own pending invites to the group are cleaned too.
	owner.mustDo(http.StatusCreated, http.MethodPost, base+"/invite-user", map[string]string{"target_user_id": outsider.ID})
	joinWithCode(t, owner, outsider, group.ID)
	owner.mustDo(http.StatusNoContent, http.MethodDelete, base+"/members/"+outsider.ID, nil)
	var outsiderInvites []struct {
		GroupID string `json:"group_id"`
	}
	outsider.mustDo(http.StatusOK, http.MethodGet, "/api/invites", nil).json(t, &outsiderInvites)
	for _, invite := range outsiderInvites {
		if invite.GroupID == group.ID {
			t.Fatalf("removed user's pending invite survived: %+v", outsiderInvites)
		}
	}
}

var newInviteCodePattern = regexp.MustCompile(`^[ABCDEFGHJKMNPQRSTVWXYZ2-9]{5}-[ABCDEFGHJKMNPQRSTVWXYZ2-9]{5}$`)

func TestE2EInviteCodes(t *testing.T) {
	a := newAPI(t)
	admin := a.platformAdmin()
	owner := a.newUser("IC Owner")
	groupAdmin := a.newUser("IC Admin")
	member := a.newUser("IC Member")
	joiner := a.newUser("IC Joiner")
	late := a.newUser("IC Late")
	group := createOwnedGroup(t, owner, admin, "Codes group "+uniqueSuffix())
	base := "/api/groups/" + group.ID

	var issued groupPayload
	owner.mustDo(http.StatusOK, http.MethodPost, base+"/invite-code", nil).json(t, &issued)
	if !newInviteCodePattern.MatchString(issued.InviteCode) {
		t.Fatalf("new invite code %q is not XXXXX-XXXXX from the unambiguous alphabet", issued.InviteCode)
	}
	// Join with sloppy input: lower case, spaces, no dash.
	sloppy := " " + strings.ToLower(strings.ReplaceAll(issued.InviteCode, "-", " ")) + " "
	res := member.mustDo(http.StatusOK, http.MethodPost, "/api/groups/join-by-code", map[string]string{"invite_code": sloppy})
	if strings.Contains(res.str(), issued.InviteCode) {
		t.Fatalf("join response leaks the invite code: %s", res.str())
	}
	joinWithCode(t, owner, groupAdmin, group.ID)
	owner.mustDo(http.StatusOK, http.MethodPost, base+"/members/"+groupAdmin.ID+"/role", map[string]string{"role": "admin"})

	// Only owner/admin obtain the code.
	member.mustDo(http.StatusForbidden, http.MethodPost, base+"/invite-code", nil)
	member.mustDo(http.StatusForbidden, http.MethodPost, base+"/invite-code/rotate", nil)
	var adminView groupPayload
	groupAdmin.mustDo(http.StatusOK, http.MethodPost, base+"/invite-code", nil).json(t, &adminView)
	if adminView.InviteCode != issued.InviteCode {
		t.Fatalf("admin got %q, want %q", adminView.InviteCode, issued.InviteCode)
	}
	if got := groupIDs(t, member)[group.ID].InviteCode; got != "" {
		t.Fatalf("member's group list shows the invite code %q", got)
	}
	if got := groupIDs(t, owner)[group.ID].InviteCode; got != issued.InviteCode {
		t.Fatalf("owner's group list shows %q, want %q", got, issued.InviteCode)
	}

	// Rotation invalidates the old code.
	var rotated groupPayload
	groupAdmin.mustDo(http.StatusOK, http.MethodPost, base+"/invite-code/rotate", nil).json(t, &rotated)
	if rotated.ID != group.ID || !newInviteCodePattern.MatchString(rotated.InviteCode) || rotated.InviteCode == issued.InviteCode {
		t.Fatalf("rotate returned %+v (old %q)", rotated, issued.InviteCode)
	}
	joiner.mustDo(http.StatusNotFound, http.MethodPost, "/api/groups/join-by-code", map[string]string{"invite_code": issued.InviteCode})
	joiner.mustDo(http.StatusOK, http.MethodPost, "/api/groups/join-by-code", map[string]string{"invite_code": strings.ReplaceAll(rotated.InviteCode, "-", "")})

	// Legacy AAA-999 codes keep working, with or without the dash.
	legacy := legacyInviteCode(t)
	if _, err := env.db.Pool.Exec(context.Background(), `UPDATE groups SET invite_code = $2 WHERE id = $1`, group.ID, legacy); err != nil {
		t.Fatalf("set legacy code: %v", err)
	}
	late.mustDo(http.StatusOK, http.MethodPost, "/api/groups/join-by-code", map[string]string{"invite_code": strings.ToLower(strings.ReplaceAll(legacy, "-", ""))})
	var kept groupPayload
	owner.mustDo(http.StatusOK, http.MethodPost, base+"/invite-code", nil).json(t, &kept)
	if kept.InviteCode != legacy {
		t.Fatalf("existing code changed on read: %q, want %q", kept.InviteCode, legacy)
	}
}

// legacyInviteCode returns a random code in the old AAA-999 format.
func legacyInviteCode(t *testing.T) string {
	t.Helper()
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatal(err)
	}
	code := make([]byte, 0, 7)
	for i := 0; i < 3; i++ {
		code = append(code, 'A'+b[i]%26)
	}
	code = append(code, '-')
	for i := 3; i < 6; i++ {
		code = append(code, '0'+b[i]%10)
	}
	return string(code)
}

func TestE2ELogoutAllAndRefreshReuse(t *testing.T) {
	a := newAPI(t)
	admin := a.platformAdmin()
	u := a.newUser("Sessions User")
	second := a.signIn(u.Phone, "Ignored")
	group := createOwnedGroup(t, u, admin, "Sessions group "+uniqueSuffix())

	conn := dialGroupWS(t, a, u, group.ID)
	defer conn.Close()
	if res := a.call(http.MethodPost, "/api/auth/logout-all", "", nil); res.Status != http.StatusUnauthorized {
		t.Fatalf("logout-all without auth: %d", res.Status)
	}
	u.mustDo(http.StatusNoContent, http.MethodPost, "/api/auth/logout-all", nil)
	expectSocketClosed(t, conn)
	for _, token := range []string{u.RefreshToken, second.RefreshToken} {
		if res := a.call(http.MethodPost, "/api/auth/refresh", "", map[string]string{"refresh_token": token}); res.Status != http.StatusUnauthorized {
			t.Fatalf("refresh after logout-all: %d %s", res.Status, res.str())
		}
	}

	// Reuse detection: a rotated-out token presented later revokes everything.
	deviceA := a.signIn(u.Phone, "Ignored")
	deviceB := a.signIn(u.Phone, "Ignored")
	res := a.call(http.MethodPost, "/api/auth/refresh", "", map[string]string{"refresh_token": deviceA.RefreshToken})
	if res.Status != http.StatusOK {
		t.Fatalf("refresh: %d %s", res.Status, res.str())
	}
	var rotated sessionPayload
	res.json(t, &rotated)
	// Age the rotation past the short grace period for concurrent refreshes.
	if _, err := env.db.Pool.Exec(context.Background(), `UPDATE refresh_tokens SET revoked_at = now() - interval '1 minute' WHERE user_id = $1 AND revoked_at IS NOT NULL`, u.ID); err != nil {
		t.Fatalf("age revoked tokens: %v", err)
	}
	if res := a.call(http.MethodPost, "/api/auth/refresh", "", map[string]string{"refresh_token": deviceA.RefreshToken}); res.Status != http.StatusUnauthorized {
		t.Fatalf("reused refresh token: %d %s", res.Status, res.str())
	}
	for name, token := range map[string]string{"rotated successor": rotated.RefreshToken, "other device": deviceB.RefreshToken} {
		if res := a.call(http.MethodPost, "/api/auth/refresh", "", map[string]string{"refresh_token": token}); res.Status != http.StatusUnauthorized {
			t.Fatalf("%s still refreshes after reuse detection: %d %s", name, res.Status, res.str())
		}
	}
}

func TestE2EMessageEditRequiresMembership(t *testing.T) {
	a := newAPI(t)
	admin := a.platformAdmin()
	owner := a.newUser("Edit Owner")
	member := a.newUser("Edit Admin")
	group := createOwnedGroup(t, owner, admin, "Edit group "+uniqueSuffix())
	joinWithCode(t, owner, member, group.ID)
	// Group messages are written by the owner and admins.
	owner.mustDo(http.StatusOK, http.MethodPost, "/api/groups/"+group.ID+"/members/"+member.ID+"/role", map[string]string{"role": "admin"})
	msgID := sendMessage(t, member, group.ID, "original text")
	member.mustDo(http.StatusOK, http.MethodPatch, "/api/groups/"+group.ID+"/messages/"+msgID, map[string]string{"text": "edited while member"})
	owner.mustDo(http.StatusNoContent, http.MethodDelete, "/api/groups/"+group.ID+"/members/"+member.ID, nil)
	if res := member.do(http.MethodPatch, "/api/groups/"+group.ID+"/messages/"+msgID, map[string]string{"text": "edited after leaving"}); res.Status != http.StatusForbidden && res.Status != http.StatusNotFound {
		t.Fatalf("edit after removal: %d %s", res.Status, res.str())
	}
	var msgs []struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	}
	owner.mustDo(http.StatusOK, http.MethodGet, "/api/groups/"+group.ID+"/messages", nil).json(t, &msgs)
	for _, m := range msgs {
		if m.ID == msgID && m.Text != "edited while member" {
			t.Fatalf("message changed after the sender left: %q", m.Text)
		}
	}
}

func jpegWithExif(t *testing.T, secret string) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewGray(image.Rect(0, 0, 4, 4)), nil); err != nil {
		t.Fatal(err)
	}
	payload := append([]byte("Exif\x00\x00II\x2a\x00\x08\x00\x00\x00\x00\x00\x00\x00\x00\x00"), []byte(secret)...)
	segment := []byte{0xff, 0xe1, byte((len(payload) + 2) >> 8), byte(len(payload) + 2)}
	out := append([]byte{0xff, 0xd8}, segment...)
	out = append(out, payload...)
	return append(out, buf.Bytes()[2:]...)
}

func uploadBytes(u *user, groupID, kind, name string, data []byte) response {
	u.api.t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	_ = writer.WriteField("kind", kind)
	part, _ := writer.CreateFormFile("file", name)
	_, _ = part.Write(data)
	_ = writer.Close()
	return u.api.raw(http.MethodPost, "/api/groups/"+groupID+"/files", u.AccessToken, writer.FormDataContentType(), &buf)
}

func TestE2EUploadsStripMetadataAndCleanUp(t *testing.T) {
	a := newAPI(t)
	admin := a.platformAdmin()
	super := a.superAdmin()
	owner := a.newUser("Up Owner")
	uploader := a.newUser("Up Uploader")
	group := createOwnedGroup(t, owner, admin, "Upload group "+uniqueSuffix())
	joinWithCode(t, owner, uploader, group.ID)

	// EXIF is stripped from uploaded photos.
	res := uploadBytes(uploader, group.ID, "photo", "gps.jpg", jpegWithExif(t, "GPS-SECRET-E2E"))
	if res.Status != http.StatusCreated {
		t.Fatalf("upload: %d %s", res.Status, res.str())
	}
	var file struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}
	res.json(t, &file)
	served := owner.mustDo(http.StatusOK, http.MethodGet, file.URL, nil)
	if bytes.Contains(served.Body, []byte("GPS-SECRET-E2E")) {
		t.Fatal("served photo still carries EXIF data")
	}
	if _, err := jpeg.Decode(bytes.NewReader(served.Body)); err != nil {
		t.Fatalf("served photo does not decode: %v", err)
	}

	// HEIC (ftyp heic) is not a video.
	heic := append([]byte{0, 0, 0, 0x18, 'f', 't', 'y', 'p', 'h', 'e', 'i', 'c'}, make([]byte, 64)...)
	if res := uploadBytes(uploader, group.ID, "video", "photo.heic", heic); res.Status != http.StatusBadRequest {
		t.Fatalf("HEIC accepted as video: %d %s", res.Status, res.str())
	}
	mp4 := append([]byte{0, 0, 0, 0x18, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm'}, make([]byte, 64)...)
	res = uploadBytes(uploader, group.ID, "video", "clip.mp4", mp4)
	if res.Status != http.StatusCreated {
		t.Fatalf("mp4 upload: %d %s", res.Status, res.str())
	}
	var video struct {
		URL         string `json:"url"`
		ContentType string `json:"content_type"`
	}
	res.json(t, &video)
	if video.ContentType != "video/mp4" {
		t.Fatalf("mp4 content type %q", video.ContentType)
	}

	// Avatars: magic number checked.
	uploader.mustDo(http.StatusBadRequest, http.MethodPut, "/api/me/avatar", map[string]string{"avatar_data": "data:image/png;base64,PGh0bWw+"})

	// Account deletion removes the user's stored files.
	stored := filepath.Join(os.Getenv("PUBLIC_FILE_DIR"), group.ID, file.ID+".jpg")
	if _, err := os.Stat(stored); err != nil {
		t.Fatalf("stored file missing before deletion: %v", err)
	}
	uploader.mustDo(http.StatusOK, http.MethodDelete, "/api/me", nil)
	if _, err := os.Stat(stored); !os.IsNotExist(err) {
		t.Fatalf("uploaded file survived account deletion: %v", err)
	}

	// Group deletion removes the group's directory.
	ownerFile := uploadFile(owner, group.ID)
	if ownerFile.Status != http.StatusCreated {
		t.Fatalf("owner upload: %d %s", ownerFile.Status, ownerFile.str())
	}
	groupDir := filepath.Join(os.Getenv("PUBLIC_FILE_DIR"), group.ID)
	super.mustDo(http.StatusOK, http.MethodDelete, "/api/admin/groups/"+group.ID, nil)
	if _, err := os.Stat(groupDir); !os.IsNotExist(err) {
		t.Fatalf("group files survived group deletion: %v", err)
	}
}

func TestE2EWebSocketHealthHidesClientCount(t *testing.T) {
	a := newAPI(t)
	res := a.call(http.MethodGet, "/api/health/ws", "", nil)
	if res.Status != http.StatusOK || strings.Contains(res.str(), "clients") {
		t.Fatalf("/api/health/ws: %d %s", res.Status, res.str())
	}
}

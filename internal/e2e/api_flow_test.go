package e2e_test

import (
	"encoding/base64"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestE2EAuthSessionLifecycle(t *testing.T) {
	a := newAPI(t)

	// No / bad token.
	if res := a.call(http.MethodGet, "/api/me", "", nil); res.Status != http.StatusUnauthorized {
		t.Fatalf("GET /api/me without token: %d", res.Status)
	}
	if res := a.call(http.MethodGet, "/api/me", "not.a.jwt", nil); res.Status != http.StatusUnauthorized {
		t.Fatalf("GET /api/me with garbage token: %d", res.Status)
	}

	u := a.newUser("Session User")
	var me struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
		Role        string `json:"role"`
	}
	u.mustDo(http.StatusOK, http.MethodGet, "/api/me", nil).json(t, &me)
	if me.ID != u.ID || me.DisplayName != "Session User" || me.Role != "user" {
		t.Fatalf("unexpected /api/me: %+v", me)
	}

	// Second sign-in of the same phone keeps the account (no duplicate user).
	again := a.signIn(u.Phone, "Ignored Name")
	if again.ID != u.ID {
		t.Fatalf("re-sign-in created a different user: %s vs %s", again.ID, u.ID)
	}

	// Refresh rotates the refresh token.
	res := a.call(http.MethodPost, "/api/auth/refresh", "", map[string]string{"refresh_token": u.RefreshToken})
	if res.Status != http.StatusOK {
		t.Fatalf("refresh: %d %s", res.Status, res.str())
	}
	var refreshed sessionPayload
	res.json(t, &refreshed)
	if refreshed.AccessToken == "" || refreshed.RefreshToken == "" || refreshed.RefreshToken == u.RefreshToken || refreshed.User.ID != u.ID {
		t.Fatalf("refresh returned unexpected session: %s", res.str())
	}
	// The rotated-out refresh token is dead.
	if res := a.call(http.MethodPost, "/api/auth/refresh", "", map[string]string{"refresh_token": u.RefreshToken}); res.Status != http.StatusUnauthorized {
		t.Fatalf("reusing rotated refresh token: %d %s", res.Status, res.str())
	}
	u.AccessToken, u.RefreshToken = refreshed.AccessToken, refreshed.RefreshToken
	u.mustDo(http.StatusOK, http.MethodGet, "/api/me", nil)

	// Sign out revokes the refresh token.
	if res := a.call(http.MethodPost, "/api/auth/logout", "", map[string]string{"refresh_token": u.RefreshToken}); res.Status != http.StatusOK {
		t.Fatalf("logout: %d %s", res.Status, res.str())
	}
	if res := a.call(http.MethodPost, "/api/auth/refresh", "", map[string]string{"refresh_token": u.RefreshToken}); res.Status != http.StatusUnauthorized {
		t.Fatalf("refresh after logout: %d %s", res.Status, res.str())
	}
	// Logout is idempotent.
	if res := a.call(http.MethodPost, "/api/auth/logout", "", map[string]string{"refresh_token": u.RefreshToken}); res.Status != http.StatusOK {
		t.Fatalf("second logout: %d %s", res.Status, res.str())
	}
	// Validation.
	if res := a.call(http.MethodPost, "/api/auth/refresh", "", map[string]string{"refresh_token": ""}); res.Status != http.StatusBadRequest {
		t.Fatalf("empty refresh token: %d", res.Status)
	}
	if res := a.call(http.MethodPost, "/api/auth/verify-code", "", map[string]string{"mobile": u.Phone, "code": "000000"}); res.Status != http.StatusUnauthorized {
		t.Fatalf("wrong test code: %d %s", res.Status, res.str())
	}
	if res := a.call(http.MethodPost, "/api/auth/request-code", "", map[string]string{"mobile": "12345"}); res.Status != http.StatusBadRequest {
		t.Fatalf("invalid mobile: %d", res.Status)
	}
	if res := a.call(http.MethodPost, "/api/auth/request-code", "", map[string]any{"mobile": u.Phone, "extra": 1}); res.Status != http.StatusBadRequest {
		t.Fatalf("unknown field accepted: %d", res.Status)
	}
}

// The store-review account goes through the stored-code path (hash, attempts,
// consumption) rather than the test-auth shortcut.
func TestE2EAppReviewStoredCodeFlow(t *testing.T) {
	e := requireEnv(t)
	a := newAPI(t)
	phone := e.appReviewPhone

	res := a.call(http.MethodPost, "/api/auth/request-code", "", map[string]string{"mobile": phone})
	if res.Status != http.StatusOK {
		t.Fatalf("request-code: %d %s", res.Status, res.str())
	}
	if strings.Contains(res.str(), appReviewCode) || strings.Contains(res.str(), "dev_code") {
		t.Fatalf("request-code leaked the review code: %s", res.str())
	}
	if res := a.call(http.MethodPost, "/api/auth/verify-code", "", map[string]string{"mobile": phone, "code": "999999", "display_name": "Reviewer"}); res.Status != http.StatusUnauthorized {
		t.Fatalf("wrong code: %d %s", res.Status, res.str())
	}
	res = a.call(http.MethodPost, "/api/auth/verify-code", "", map[string]string{"mobile": phone, "code": appReviewCode, "display_name": "Reviewer"})
	if res.Status != http.StatusOK {
		t.Fatalf("verify-code: %d %s", res.Status, res.str())
	}
	// A consumed code cannot be replayed.
	if res := a.call(http.MethodPost, "/api/auth/verify-code", "", map[string]string{"mobile": phone, "code": appReviewCode, "display_name": "Reviewer"}); res.Status != http.StatusUnauthorized {
		t.Fatalf("replayed code: %d %s", res.Status, res.str())
	}
}

func TestE2EProfile(t *testing.T) {
	a := newAPI(t)
	u := a.newUser("Profile User")

	avatar := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes())
	var updated struct {
		ID         string `json:"id"`
		AvatarData string `json:"avatar_data"`
	}
	u.mustDo(http.StatusOK, http.MethodPut, "/api/me/avatar", map[string]string{"avatar_data": avatar}).json(t, &updated)
	if updated.ID != u.ID || updated.AvatarData != avatar {
		t.Fatalf("avatar not stored: %+v", updated)
	}
	var me struct {
		AvatarData string `json:"avatar_data"`
	}
	u.mustDo(http.StatusOK, http.MethodGet, "/api/me", nil).json(t, &me)
	if me.AvatarData != avatar {
		t.Fatalf("GET /api/me does not return the avatar")
	}
	u.mustDo(http.StatusBadRequest, http.MethodPut, "/api/me/avatar", map[string]string{"avatar_data": "data:image/gif;base64,R0lGODlh"})
	u.mustDo(http.StatusOK, http.MethodPut, "/api/me/avatar", map[string]string{"avatar_data": ""})
	me.AvatarData = "" // avatar_data is omitempty
	u.mustDo(http.StatusOK, http.MethodGet, "/api/me", nil).json(t, &me)
	if me.AvatarData != "" {
		t.Fatalf("avatar not cleared")
	}

	// Push token register / delete.
	u.mustDo(http.StatusOK, http.MethodPost, "/api/push/register", map[string]string{"token": "e2e-token-" + uniqueSuffix(), "platform": "android"})
	u.mustDo(http.StatusBadRequest, http.MethodPost, "/api/push/register", map[string]string{"token": "", "platform": "android"})
	var ws struct {
		Token string `json:"token"`
	}
	u.mustDo(http.StatusOK, http.MethodPost, "/api/ws-token", nil).json(t, &ws)
	// A websocket token is not an API access token.
	if res := a.call(http.MethodGet, "/api/me", ws.Token, nil); res.Status != http.StatusUnauthorized {
		t.Fatalf("ws token accepted as bearer: %d", res.Status)
	}
}

func TestE2EGroupWebSocket(t *testing.T) {
	a := newAPI(t)
	admin := a.platformAdmin()
	owner := a.newUser("WS Owner")
	outsider := a.newUser("WS Outsider")
	group := createOwnedGroup(t, owner, admin, "WS group "+uniqueSuffix())
	wsURL := "ws" + strings.TrimPrefix(a.srv.URL, "http") + "/api/groups/" + group.ID + "/ws"

	dial := func(u *user) (*websocket.Conn, int) {
		var token struct {
			Token string `json:"token"`
		}
		u.mustDo(http.StatusOK, http.MethodPost, "/api/ws-token", nil).json(t, &token)
		conn, res, err := websocket.DefaultDialer.Dial(wsURL, http.Header{"Sec-WebSocket-Protocol": {"koom-ws, " + token.Token}})
		status := 0
		if res != nil {
			status = res.StatusCode
		}
		if err != nil {
			return nil, status
		}
		return conn, status
	}
	if conn, status := dial(outsider); conn != nil || status != http.StatusForbidden {
		if conn != nil {
			conn.Close()
		}
		t.Fatalf("outsider websocket: status %d", status)
	}
	conn, status := dial(owner)
	if conn == nil {
		t.Fatalf("owner websocket: status %d", status)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var ready struct {
		Type string `json:"type"`
	}
	if err := conn.ReadJSON(&ready); err != nil || ready.Type != "connection.ready" {
		t.Fatalf("first event %+v err=%v", ready, err)
	}
	msgID := sendMessage(t, owner, group.ID, "realtime hello")
	for {
		var event struct {
			Type    string `json:"type"`
			Payload struct {
				ID string `json:"id"`
			} `json:"payload"`
		}
		if err := conn.ReadJSON(&event); err != nil {
			t.Fatalf("waiting for message.created: %v", err)
		}
		if event.Type == "message.created" && event.Payload.ID == msgID {
			break
		}
	}
}

func TestE2EGroupCreationRequestFlow(t *testing.T) {
	a := newAPI(t)
	admin := a.platformAdmin()
	requester := a.newUser("Requester")

	// Plain users cannot create groups directly or see the admin queue.
	requester.mustDo(http.StatusForbidden, http.MethodPost, "/api/groups", map[string]string{"title": "Direct group", "visibility": "public"})
	requester.mustDo(http.StatusForbidden, http.MethodGet, "/api/admin/group-creation-requests", nil)

	// Validation.
	bad := groupCreationInput("x")
	requester.mustDo(http.StatusBadRequest, http.MethodPost, "/api/group-creation-requests", bad)

	submit := func(title string) creationRequestPayload {
		var created creationRequestPayload
		requester.mustDo(http.StatusCreated, http.MethodPost, "/api/group-creation-requests", groupCreationInput(title)).json(t, &created)
		if created.Status != "pending" || created.RequesterID != requester.ID {
			t.Fatalf("unexpected new request: %+v", created)
		}
		return created
	}
	suffix := uniqueSuffix()
	toApprove := submit("Approve me " + suffix)
	toReject := submit("Reject me " + suffix)
	toAsk := submit("More info " + suffix)

	var mine []creationRequestPayload
	requester.mustDo(http.StatusOK, http.MethodGet, "/api/group-creation-requests", nil).json(t, &mine)
	if len(mine) < 3 {
		t.Fatalf("requester sees %d own requests, want >= 3", len(mine))
	}
	var queue []creationRequestPayload
	admin.mustDo(http.StatusOK, http.MethodGet, "/api/admin/group-creation-requests?status=pending", nil).json(t, &queue)
	found := 0
	for _, item := range queue {
		if item.ID == toApprove.ID || item.ID == toReject.ID || item.ID == toAsk.ID {
			found++
		}
	}
	if found != 3 {
		t.Fatalf("admin queue contains %d of the 3 new requests", found)
	}

	// The requester cannot review its own request.
	requester.mustDo(http.StatusForbidden, http.MethodPost, "/api/admin/group-creation-requests/"+toApprove.ID+"/approve", map[string]string{})

	var rejected creationRequestPayload
	admin.mustDo(http.StatusOK, http.MethodPost, "/api/admin/group-creation-requests/"+toReject.ID+"/reject", map[string]string{"admin_comment": "no"}).json(t, &rejected)
	if rejected.Status != "rejected" || rejected.CreatedGroupID != "" {
		t.Fatalf("reject: %+v", rejected)
	}
	var asked creationRequestPayload
	admin.mustDo(http.StatusOK, http.MethodPost, "/api/admin/group-creation-requests/"+toAsk.ID+"/need-more-info", map[string]string{"admin_comment": "docs?"}).json(t, &asked)
	if asked.Status != "needs_more_info" {
		t.Fatalf("need-more-info: %+v", asked)
	}
	// A rejected request cannot be approved afterwards.
	if res := admin.do(http.MethodPost, "/api/admin/group-creation-requests/"+toReject.ID+"/approve", map[string]string{}); res.Status < 400 || res.Status >= 500 {
		t.Fatalf("approving a rejected request: %d %s", res.Status, res.str())
	}
	admin.mustDo(http.StatusNotFound, http.MethodPost, "/api/admin/group-creation-requests/GCR-DOESNOTEXIST/approve", map[string]string{})

	group := createGroupFromRequest(t, requester, admin, toApprove.ID)
	if group.MyRole == nil || *group.MyRole != "owner" || group.Visibility != "public" {
		t.Fatalf("approved group: %+v", group)
	}
	// Approving twice does not create a second group.
	if res := admin.do(http.MethodPost, "/api/admin/group-creation-requests/"+toApprove.ID+"/approve", map[string]string{}); res.Status < 400 || res.Status >= 500 {
		t.Fatalf("approving twice: %d %s", res.Status, res.str())
	}
	// Direct creation and platform-wide deletion are reserved for super_admin;
	// platform_admin may only review creation requests.
	admin.mustDo(http.StatusForbidden, http.MethodPost, "/api/groups", map[string]string{"title": "Admin direct " + suffix, "visibility": "private"})
	admin.mustDo(http.StatusForbidden, http.MethodDelete, "/api/admin/groups/"+group.ID, nil)
	super := a.superAdmin()
	var direct groupPayload
	super.mustDo(http.StatusCreated, http.MethodPost, "/api/groups", map[string]string{"title": "Super direct " + suffix, "visibility": "private"}).json(t, &direct)
	if direct.ID == "" || direct.Visibility != "private" {
		t.Fatalf("direct group: %+v", direct)
	}
	super.mustDo(http.StatusOK, http.MethodDelete, "/api/admin/groups/"+direct.ID, nil)
	super.mustDo(http.StatusNotFound, http.MethodDelete, "/api/admin/groups/"+direct.ID, nil)
}

func createGroupFromRequest(t *testing.T, owner, admin *user, requestID string) groupPayload {
	t.Helper()
	var approved creationRequestPayload
	admin.mustDo(http.StatusOK, http.MethodPost, "/api/admin/group-creation-requests/"+requestID+"/approve", map[string]string{"admin_comment": "ok"}).json(t, &approved)
	var groups []groupPayload
	owner.mustDo(http.StatusOK, http.MethodGet, "/api/groups", nil).json(t, &groups)
	for _, group := range groups {
		if group.ID == approved.CreatedGroupID {
			return group
		}
	}
	t.Fatalf("approved group not visible to owner")
	return groupPayload{}
}

type memberPayload struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

func TestE2EInvitesMembersAndRoles(t *testing.T) {
	a := newAPI(t)
	admin := a.platformAdmin()
	owner := a.newUser("Owner")
	member := a.newUser("Member")
	invitee := a.newUser("Invitee")
	decliner := a.newUser("Decliner")
	group := createOwnedGroup(t, owner, admin, "Members group "+uniqueSuffix())
	base := "/api/groups/" + group.ID

	// Join by code.
	joinWithCode(t, owner, member, group.ID)
	member.mustDo(http.StatusNotFound, http.MethodPost, "/api/groups/join-by-code", map[string]string{"invite_code": "ZZZZZZZZ"})

	// Invite by phone, then accept / decline.
	var invite struct {
		ID           string `json:"id"`
		TargetUserID string `json:"target_user_id"`
	}
	owner.mustDo(http.StatusCreated, http.MethodPost, base+"/invite-user", map[string]string{"phone": invitee.Phone}).json(t, &invite)
	if invite.TargetUserID != invitee.ID {
		t.Fatalf("invite target %s, want %s", invite.TargetUserID, invitee.ID)
	}
	var pending []struct {
		ID string `json:"id"`
	}
	invitee.mustDo(http.StatusOK, http.MethodGet, "/api/invites", nil).json(t, &pending)
	if len(pending) != 1 || pending[0].ID != invite.ID {
		t.Fatalf("invitee pending invites: %+v", pending)
	}
	// Someone else cannot accept the invite.
	if res := decliner.do(http.MethodPost, "/api/invites/"+invite.ID+"/accept", nil); res.Status != http.StatusForbidden && res.Status != http.StatusNotFound {
		t.Fatalf("foreign accept: %d %s", res.Status, res.str())
	}
	invitee.mustDo(http.StatusOK, http.MethodPost, "/api/invites/"+invite.ID+"/accept", nil)

	var invite2 struct {
		ID string `json:"id"`
	}
	owner.mustDo(http.StatusCreated, http.MethodPost, base+"/invite-user", map[string]string{"target_user_id": decliner.ID}).json(t, &invite2)
	decliner.mustDo(http.StatusOK, http.MethodPost, "/api/invites/"+invite2.ID+"/decline", nil)

	var members []memberPayload
	owner.mustDo(http.StatusOK, http.MethodGet, base+"/members", nil).json(t, &members)
	roles := map[string]string{}
	for _, m := range members {
		roles[m.UserID] = m.Role
	}
	if roles[owner.ID] != "owner" || roles[member.ID] != "member" || roles[invitee.ID] != "member" {
		t.Fatalf("members after joins: %+v", roles)
	}
	if _, ok := roles[decliner.ID]; ok {
		t.Fatalf("decliner became a member")
	}

	// Role changes: plain member cannot, owner can promote/demote.
	member.mustDo(http.StatusForbidden, http.MethodPost, base+"/members/"+invitee.ID+"/role", map[string]string{"role": "admin"})
	var changed memberPayload
	owner.mustDo(http.StatusOK, http.MethodPost, base+"/members/"+member.ID+"/role", map[string]string{"role": "admin"}).json(t, &changed)
	if changed.Role != "admin" {
		t.Fatalf("promote: %+v", changed)
	}
	owner.mustDo(http.StatusBadRequest, http.MethodPost, base+"/members/"+member.ID+"/role", map[string]string{"role": "owner"})
	// The new admin cannot demote the owner.
	if res := member.do(http.MethodPost, base+"/members/"+owner.ID+"/role", map[string]string{"role": "member"}); res.Status != http.StatusForbidden {
		t.Fatalf("admin demoting owner: %d %s", res.Status, res.str())
	}
	owner.mustDo(http.StatusOK, http.MethodPost, base+"/members/role-by-phone", map[string]string{"phone": member.Phone, "role": "member"}).json(t, &changed)
	if changed.Role != "member" {
		t.Fatalf("demote by phone: %+v", changed)
	}

	// Comment mutes: owner mutes the member, member cannot comment, unmute.
	reqID := createRequest(t, owner, group.ID, "Mute check "+uniqueSuffix(), "discussion")
	owner.mustDo(http.StatusOK, http.MethodPost, base+"/comment-mutes/"+member.ID, map[string]any{"duration_minutes": 60, "reason": "e2e"})
	// Muted members get a 400 with the unmute time (service validation error).
	if res := member.do(http.MethodPost, "/api/requests/"+reqID+"/comments", map[string]string{"body": "muted comment"}); res.Status != http.StatusForbidden && res.Status != http.StatusBadRequest {
		t.Fatalf("muted member commented: %d %s", res.Status, res.str())
	}
	owner.mustDo(http.StatusOK, http.MethodPost, base+"/comment-mutes/"+member.ID+"/clear", nil)
	createComment(t, member, reqID, "unmuted comment")
	owner.mustDo(http.StatusOK, http.MethodPost, base+"/comment-mutes/by-phone", map[string]any{"phone": member.Phone, "duration_minutes": 30})
	owner.mustDo(http.StatusOK, http.MethodPost, base+"/comment-mutes/unmute-by-phone", map[string]string{"phone": member.Phone})

	// Member removal: there is no admin "remove member" route; members leave.
	invitee.mustDo(http.StatusOK, http.MethodDelete, base+"/leave", nil)
	invitee.mustDo(http.StatusForbidden, http.MethodGet, base+"/messages", nil)
	// The owner cannot leave without transferring ownership.
	if res := owner.do(http.MethodDelete, base+"/leave", nil); res.Status < 400 || res.Status >= 500 {
		t.Fatalf("owner leaving: %d %s", res.Status, res.str())
	}

	// Group avatar: owner only.
	avatar := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes())
	member.mustDo(http.StatusForbidden, http.MethodPut, base+"/avatar", map[string]string{"avatar_data": avatar})
	owner.mustDo(http.StatusOK, http.MethodPut, base+"/avatar", map[string]string{"avatar_data": avatar})

	// Messages are official posts: plain members cannot send; admins edit and
	// delete their own.
	member.mustDo(http.StatusForbidden, http.MethodPost, base+"/messages", map[string]string{"text": "plain member"})
	owner.mustDo(http.StatusOK, http.MethodPost, base+"/members/"+member.ID+"/role", map[string]string{"role": "admin"})
	msgID := sendMessage(t, member, group.ID, "hello from admin member")
	owner.mustDo(http.StatusForbidden, http.MethodPatch, base+"/messages/"+msgID, map[string]string{"text": "owner edit"})
	member.mustDo(http.StatusOK, http.MethodPatch, base+"/messages/"+msgID, map[string]string{"text": "edited"})
	var msgs []struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	}
	owner.mustDo(http.StatusOK, http.MethodGet, base+"/messages", nil).json(t, &msgs)
	if len(msgs) == 0 || msgs[0].ID != msgID || msgs[0].Text != "edited" {
		t.Fatalf("messages after edit: %+v", msgs)
	}
	member.mustDo(http.StatusOK, http.MethodDelete, base+"/messages/"+msgID, nil)
	owner.mustDo(http.StatusOK, http.MethodPost, base+"/members/"+member.ID+"/role", map[string]string{"role": "member"})

	// Files: members upload and read.
	res := uploadFile(member, group.ID)
	if res.Status != http.StatusCreated {
		t.Fatalf("upload: %d %s", res.Status, res.str())
	}
	var file struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}
	res.json(t, &file)
	if got := owner.do(http.MethodGet, file.URL, nil); got.Status != http.StatusOK {
		t.Fatalf("download: %d", got.Status)
	}

	// Statistics and search.
	owner.mustDo(http.StatusOK, http.MethodGet, base+"/statistics?period=month", nil)
	member.mustDo(http.StatusOK, http.MethodGet, "/api/groups/search?q=Members", nil)
}

func TestE2EPublicRequestsFlow(t *testing.T) {
	a := newAPI(t)
	admin := a.platformAdmin()
	owner := a.newUser("Req Owner")
	member := a.newUser("Req Member")
	other := a.newUser("Req Other Member")
	group := createOwnedGroup(t, owner, admin, "Requests group "+uniqueSuffix())
	joinWithCode(t, owner, member, group.ID)
	joinWithCode(t, owner, other, group.ID)
	base := "/api/groups/" + group.ID

	member.mustDo(http.StatusBadRequest, http.MethodPost, base+"/requests", map[string]string{"request_type": "bogus", "title": "Title", "body": "Body text"})
	reqID := createRequest(t, member, group.ID, "Fix the road", "discussion")

	type listed struct {
		ID           string  `json:"id"`
		Status       string  `json:"status"`
		SupportCount int     `json:"support_count"`
		OpposeCount  int     `json:"oppose_count"`
		CommentCount int     `json:"comment_count"`
		MyVote       *string `json:"my_vote"`
	}
	find := func(u *user) *listed {
		var items []listed
		u.mustDo(http.StatusOK, http.MethodGet, base+"/requests", nil).json(t, &items)
		for i := range items {
			if items[i].ID == reqID {
				return &items[i]
			}
		}
		return nil
	}
	if find(other) == nil {
		t.Fatalf("request not listed")
	}

	other.mustDo(http.StatusOK, http.MethodPost, "/api/requests/"+reqID+"/support", nil)
	owner.mustDo(http.StatusOK, http.MethodPost, "/api/requests/"+reqID+"/oppose", nil)
	item := find(other)
	if item.SupportCount != 1 || item.OpposeCount != 1 || item.MyVote == nil || *item.MyVote != "support" {
		t.Fatalf("after votes: %+v", item)
	}
	// Switch vote then clear.
	other.mustDo(http.StatusOK, http.MethodPost, "/api/requests/"+reqID+"/oppose", nil)
	if item = find(other); item.SupportCount != 0 || item.OpposeCount != 2 {
		t.Fatalf("after switch: %+v", item)
	}
	other.mustDo(http.StatusOK, http.MethodDelete, "/api/requests/"+reqID+"/vote", nil)
	if item = find(other); item.OpposeCount != 1 || item.MyVote != nil {
		t.Fatalf("after clear: %+v", item)
	}

	commentID := createComment(t, other, reqID, "I agree")
	var comments []struct {
		ID string `json:"id"`
	}
	member.mustDo(http.StatusOK, http.MethodGet, "/api/requests/"+reqID+"/comments", nil).json(t, &comments)
	if len(comments) != 1 || comments[0].ID != commentID {
		t.Fatalf("comments: %+v", comments)
	}
	// A plain member cannot delete someone else's comment; the owner can.
	member.mustDo(http.StatusForbidden, http.MethodDelete, "/api/requests/comments/"+commentID, nil)
	owner.mustDo(http.StatusOK, http.MethodDelete, "/api/requests/comments/"+commentID, nil)

	// Status: plain members cannot, owner can.
	member.mustDo(http.StatusForbidden, http.MethodPost, "/api/requests/"+reqID+"/status", map[string]string{"status": "accepted"})
	owner.mustDo(http.StatusBadRequest, http.MethodPost, "/api/requests/"+reqID+"/status", map[string]string{"status": "bogus"})
	owner.mustDo(http.StatusOK, http.MethodPost, "/api/requests/"+reqID+"/status", map[string]string{"status": "under_review"})
	if item = find(member); item.Status != "under_review" {
		t.Fatalf("status not updated: %+v", item)
	}

	// Vote-only requests reject comments; read-only reject votes.
	voteOnly := createRequest(t, owner, group.ID, "Vote only "+uniqueSuffix(), "vote_only")
	if res := member.do(http.MethodPost, "/api/requests/"+voteOnly+"/comments", map[string]string{"body": "nope"}); res.Status != http.StatusForbidden && res.Status != http.StatusBadRequest {
		t.Fatalf("comment on vote_only: %d %s", res.Status, res.str())
	}
	readOnly := createRequest(t, owner, group.ID, "Read only "+uniqueSuffix(), "read_only")
	if res := member.do(http.MethodPost, "/api/requests/"+readOnly+"/support", nil); res.Status != http.StatusForbidden && res.Status != http.StatusBadRequest {
		t.Fatalf("vote on read_only: %d %s", res.Status, res.str())
	}

	// Mine filter and read marker.
	var mine []listed
	member.mustDo(http.StatusOK, http.MethodGet, base+"/requests?mine=true", nil).json(t, &mine)
	if len(mine) != 1 || mine[0].ID != reqID {
		t.Fatalf("mine filter: %+v", mine)
	}
	member.mustDo(http.StatusOK, http.MethodPost, base+"/requests/read", nil)

	// Hide: plain member cannot, owner can; hidden requests leave the list.
	member.mustDo(http.StatusForbidden, http.MethodPost, "/api/requests/"+reqID+"/hide", nil)
	owner.mustDo(http.StatusOK, http.MethodPost, "/api/requests/"+reqID+"/hide", nil)
	if find(member) != nil {
		t.Fatalf("hidden request still listed")
	}
	if res := member.do(http.MethodPost, "/api/requests/"+reqID+"/support", nil); res.Status != http.StatusNotFound && res.Status != http.StatusForbidden {
		t.Fatalf("vote on hidden request: %d %s", res.Status, res.str())
	}
}

func TestE2EModerationQueue(t *testing.T) {
	a := newAPI(t)
	admin := a.platformAdmin()
	owner := a.newUser("Mod Owner")
	member := a.newUser("Mod Member")
	group := createOwnedGroup(t, owner, admin, "Moderation group "+uniqueSuffix())
	joinWithCode(t, owner, member, group.ID)
	base := "/api/groups/" + group.ID

	hold := func(title string) string {
		res := member.mustDo(http.StatusAccepted, http.MethodPost, base+"/requests", map[string]string{
			"request_type": "complaint", "interaction_mode": "discussion", "title": title, "body": "Please review " + heldMarker + " " + title,
		})
		var pending struct {
			Status string `json:"status"`
			Item   struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"moderation_item"`
		}
		res.json(t, &pending)
		if pending.Status != "pending_review" || pending.Item.ID == "" || pending.Item.Status != "pending" {
			t.Fatalf("held request: %s", res.str())
		}
		return pending.Item.ID
	}
	approveID := hold("Held approve " + uniqueSuffix())
	rejectID := hold("Held reject " + uniqueSuffix())

	// Held content is not published.
	var listed []struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	owner.mustDo(http.StatusOK, http.MethodGet, base+"/requests", nil).json(t, &listed)
	if len(listed) != 0 {
		t.Fatalf("held requests are visible: %+v", listed)
	}

	var count struct {
		Count int `json:"count"`
	}
	owner.mustDo(http.StatusOK, http.MethodGet, base+"/moderation/items/count?status=pending", nil).json(t, &count)
	if count.Count != 2 {
		t.Fatalf("pending count %d, want 2", count.Count)
	}
	var items []struct {
		ID string `json:"id"`
	}
	owner.mustDo(http.StatusOK, http.MethodGet, base+"/moderation/items?status=pending", nil).json(t, &items)
	if len(items) != 2 {
		t.Fatalf("pending items: %+v", items)
	}
	// Authors and plain members cannot review.
	member.mustDo(http.StatusForbidden, http.MethodGet, base+"/moderation/items", nil)
	member.mustDo(http.StatusForbidden, http.MethodPost, "/api/moderation/items/"+approveID+"/approve", nil)

	var result struct {
		Item struct {
			Status              string `json:"status"`
			PublishedResourceID string `json:"published_resource_id"`
		} `json:"item"`
		PublicRequest *struct {
			ID string `json:"id"`
		} `json:"public_request"`
	}
	owner.mustDo(http.StatusOK, http.MethodPost, "/api/moderation/items/"+approveID+"/approve", nil).json(t, &result)
	if result.Item.Status != "approved" || result.PublicRequest == nil || result.PublicRequest.ID == "" {
		t.Fatalf("approve result: %+v", result)
	}
	var rejected struct {
		Status string `json:"status"`
	}
	owner.mustDo(http.StatusOK, http.MethodPost, "/api/moderation/items/"+rejectID+"/reject", nil).json(t, &rejected)
	if rejected.Status != "rejected" {
		t.Fatalf("reject: %+v", rejected)
	}
	// Reviewed items cannot be reviewed again.
	if res := owner.do(http.MethodPost, "/api/moderation/items/"+rejectID+"/approve", nil); res.Status < 400 || res.Status >= 500 {
		t.Fatalf("re-review: %d %s", res.Status, res.str())
	}

	owner.mustDo(http.StatusOK, http.MethodGet, base+"/requests", nil).json(t, &listed)
	if len(listed) != 1 || listed[0].ID != result.PublicRequest.ID {
		t.Fatalf("after review: %+v", listed)
	}
	owner.mustDo(http.StatusOK, http.MethodGet, base+"/moderation/items/count?status=pending", nil).json(t, &count)
	if count.Count != 0 {
		t.Fatalf("pending count after review %d", count.Count)
	}
}

func TestE2EReportsAndBlocks(t *testing.T) {
	a := newAPI(t)
	admin := a.platformAdmin()
	owner := a.newUser("Rep Owner")
	member := a.newUser("Rep Member")
	outsider := a.newUser("Rep Outsider")
	group := createOwnedGroup(t, owner, admin, "Reports group "+uniqueSuffix())
	joinWithCode(t, owner, member, group.ID)
	base := "/api/groups/" + group.ID

	reqID := createRequest(t, owner, group.ID, "Reported request", "discussion")
	msgID := sendMessage(t, owner, group.ID, "owner message")

	var report struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	member.mustDo(http.StatusCreated, http.MethodPost, "/api/reports", map[string]string{"target_type": "public_request", "target_id": reqID, "reason": "spam", "details": "e2e"}).json(t, &report)
	if report.ID == "" || report.Status != "open" {
		t.Fatalf("report: %+v", report)
	}
	// Duplicate report is idempotent.
	member.mustDo(http.StatusOK, http.MethodPost, "/api/reports", map[string]string{"target_type": "public_request", "target_id": reqID, "reason": "spam"})
	member.mustDo(http.StatusBadRequest, http.MethodPost, "/api/reports", map[string]string{"target_type": "user", "target_id": member.ID, "reason": "spam"})
	member.mustDo(http.StatusBadRequest, http.MethodPost, "/api/reports", map[string]string{"target_type": "bogus", "target_id": reqID, "reason": "spam"})
	// Outsiders cannot report content they cannot see.
	if res := outsider.do(http.MethodPost, "/api/reports", map[string]string{"target_type": "message", "target_id": msgID, "reason": "abuse"}); res.Status != http.StatusNotFound && res.Status != http.StatusForbidden {
		t.Fatalf("outsider report: %d %s", res.Status, res.str())
	}

	var groupReports []struct {
		ID string `json:"id"`
	}
	owner.mustDo(http.StatusOK, http.MethodGet, base+"/reports?status=open", nil).json(t, &groupReports)
	if len(groupReports) != 1 || groupReports[0].ID != report.ID {
		t.Fatalf("group reports: %+v", groupReports)
	}
	member.mustDo(http.StatusForbidden, http.MethodGet, base+"/reports", nil)
	member.mustDo(http.StatusForbidden, http.MethodGet, "/api/admin/reports", nil)
	admin.mustDo(http.StatusOK, http.MethodGet, "/api/admin/reports?status=open", nil)
	member.mustDo(http.StatusForbidden, http.MethodPost, "/api/reports/"+report.ID+"/resolve", map[string]string{"action": "dismissed"})

	var resolved struct {
		Status     string `json:"status"`
		Resolution string `json:"resolution"`
	}
	owner.mustDo(http.StatusOK, http.MethodPost, "/api/reports/"+report.ID+"/resolve", map[string]string{"action": "content_removed"}).json(t, &resolved)
	if resolved.Status != "resolved" || resolved.Resolution != "content_removed" {
		t.Fatalf("resolve: %+v", resolved)
	}
	var listed []struct {
		ID string `json:"id"`
	}
	member.mustDo(http.StatusOK, http.MethodGet, base+"/requests", nil).json(t, &listed)
	for _, item := range listed {
		if item.ID == reqID {
			t.Fatalf("removed request still listed")
		}
	}

	// Blocks: member blocks owner and no longer sees owner's messages.
	member.mustDo(http.StatusBadRequest, http.MethodPost, "/api/users/"+member.ID+"/block", nil)
	member.mustDo(http.StatusNoContent, http.MethodPost, "/api/users/"+owner.ID+"/block", nil)
	var blocks []struct {
		UserID string `json:"user_id"`
	}
	member.mustDo(http.StatusOK, http.MethodGet, "/api/me/blocks", nil).json(t, &blocks)
	if len(blocks) != 1 || blocks[0].UserID != owner.ID {
		t.Fatalf("blocks: %+v", blocks)
	}
	var msgs []struct {
		ID string `json:"id"`
	}
	member.mustDo(http.StatusOK, http.MethodGet, base+"/messages", nil).json(t, &msgs)
	for _, m := range msgs {
		if m.ID == msgID {
			t.Fatalf("blocked user's message still visible")
		}
	}
	member.mustDo(http.StatusNoContent, http.MethodDelete, "/api/users/"+owner.ID+"/block", nil)
	member.mustDo(http.StatusOK, http.MethodGet, "/api/me/blocks", nil).json(t, &blocks)
	if len(blocks) != 0 {
		t.Fatalf("blocks after unblock: %+v", blocks)
	}
}

func TestE2EAccountDeletion(t *testing.T) {
	a := newAPI(t)
	admin := a.platformAdmin()
	owner := a.newUser("Del Owner")
	member := a.newUser("Del Member")
	poster := a.newUser("Del Poster")
	solo := a.newUser("Del Solo Owner")
	group := createOwnedGroup(t, owner, admin, "Deletion group "+uniqueSuffix())
	createOwnedGroup(t, solo, admin, "Solo group "+uniqueSuffix())
	joinWithCode(t, owner, member, group.ID)
	joinWithCode(t, owner, poster, group.ID)
	reqID := createRequest(t, poster, group.ID, "Poster post", "discussion")

	// The only member of a group cannot delete the account without a successor.
	solo.mustDo(http.StatusConflict, http.MethodDelete, "/api/me", nil)
	solo.mustDo(http.StatusOK, http.MethodGet, "/api/me", nil)

	poster.mustDo(http.StatusOK, http.MethodDelete, "/api/me", nil)
	if res := poster.do(http.MethodGet, "/api/me", nil); res.Status != http.StatusUnauthorized {
		t.Fatalf("token after account deletion: %d %s", res.Status, res.str())
	}
	if res := a.call(http.MethodPost, "/api/auth/refresh", "", map[string]string{"refresh_token": poster.RefreshToken}); res.Status != http.StatusUnauthorized {
		t.Fatalf("refresh after account deletion: %d %s", res.Status, res.str())
	}
	// Public content stays, anonymized.
	var listed []struct {
		ID       string `json:"id"`
		AuthorID string `json:"author_id"`
	}
	owner.mustDo(http.StatusOK, http.MethodGet, "/api/groups/"+group.ID+"/requests", nil).json(t, &listed)
	found := false
	for _, item := range listed {
		if item.ID == reqID {
			found = true
			if item.AuthorID == poster.ID {
				t.Fatalf("deleted user's request still attributed to them")
			}
		}
	}
	if !found {
		t.Fatalf("deleted user's public request disappeared")
	}

	// An owner with other members hands the group over on deletion.
	owner.mustDo(http.StatusOK, http.MethodDelete, "/api/me", nil)
	var members []memberPayload
	member.mustDo(http.StatusOK, http.MethodGet, "/api/groups/"+group.ID+"/members", nil).json(t, &members)
	if len(members) != 1 || members[0].UserID != member.ID || members[0].Role != "owner" {
		t.Fatalf("members after owner deletion: %+v", members)
	}

	// Signing in again with the same phone creates a fresh account.
	again := a.signIn(poster.Phone, "Del Poster Again")
	if again.ID == poster.ID {
		t.Fatalf("deleted account was revived")
	}
}

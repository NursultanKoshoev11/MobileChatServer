package e2e_test

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// actor names used by the matrix.
const (
	actOutsider    = "outsider"    // not a member of G; owner of its own group H
	actMember      = "member"      // plain member of G
	actGroupAdmin  = "group-admin" // admin of G (not owner)
	actGroupOwner  = "group-owner" // owner of G (for platform-only routes)
	actPlatformAdm = "platform-admin"
)

type matrixRow struct {
	// route is the chi pattern from server.go ("METHOD /path/{param}").
	route string
	// path is the concrete URL; body is the JSON body (nil for none).
	path string
	body any
	// deny lists the actors that must get 403 or 404.
	deny []string
	// multipart sends a file upload instead of JSON.
	multipart bool
}

type fixture struct {
	owner, groupAdmin, member, outsider, platformAdmin, superAdmin, inviteTarget *user

	groupID, otherGroupID, privateGroupID string
	messageID, otherGroupMessageID        string
	requestID, commentID                  string
	fileID, otherGroupFileID              string
	moderationItemID                      string
	reportID                              string
	inviteID                              string
	creationRequestID                     string
}

func setupFixture(t *testing.T, a *api) *fixture {
	t.Helper()
	f := &fixture{}
	f.platformAdmin = a.platformAdmin()
	f.superAdmin = a.superAdmin()
	f.owner = a.newUser("Matrix Owner")
	f.groupAdmin = a.newUser("Matrix Group Admin")
	f.member = a.newUser("Matrix Member")
	f.outsider = a.newUser("Matrix Outsider")
	f.inviteTarget = a.newUser("Matrix Invite Target")

	suffix := uniqueSuffix()
	g := createOwnedGroup(t, f.owner, f.platformAdmin, "Matrix G "+suffix)
	f.groupID = g.ID
	h := createOwnedGroup(t, f.outsider, f.platformAdmin, "Matrix H "+suffix)
	f.otherGroupID = h.ID
	var private groupPayload
	f.superAdmin.mustDo(http.StatusCreated, http.MethodPost, "/api/groups", map[string]string{"title": "Matrix private " + suffix, "visibility": "private"}).json(t, &private)
	f.privateGroupID = private.ID

	joinWithCode(t, f.owner, f.groupAdmin, f.groupID)
	joinWithCode(t, f.owner, f.member, f.groupID)
	f.owner.mustDo(http.StatusOK, http.MethodPost, "/api/groups/"+f.groupID+"/members/"+f.groupAdmin.ID+"/role", map[string]string{"role": "admin"})

	f.messageID = sendMessage(t, f.owner, f.groupID, "matrix official message")
	f.otherGroupMessageID = sendMessage(t, f.outsider, f.otherGroupID, "outsider's own message")
	f.requestID = createRequest(t, f.owner, f.groupID, "Matrix request", "discussion")
	f.commentID = createComment(t, f.owner, f.requestID, "matrix owner comment")

	upload := func(u *user, groupID string) string {
		res := uploadFile(u, groupID)
		if res.Status != http.StatusCreated {
			t.Fatalf("upload: %d %s", res.Status, res.str())
		}
		var file idPayload
		res.json(t, &file)
		return file.ID
	}
	f.fileID = upload(f.owner, f.groupID)
	f.otherGroupFileID = upload(f.outsider, f.otherGroupID)

	// A pending moderation item in G (authored by the plain member).
	res := f.member.mustDo(http.StatusAccepted, http.MethodPost, "/api/groups/"+f.groupID+"/requests", map[string]string{
		"request_type": "complaint", "interaction_mode": "discussion", "title": "Held matrix", "body": "Held body " + heldMarker,
	})
	var pending struct {
		Item idPayload `json:"moderation_item"`
	}
	res.json(t, &pending)
	f.moderationItemID = pending.Item.ID

	var report idPayload
	f.member.mustDo(http.StatusCreated, http.MethodPost, "/api/reports", map[string]string{"target_type": "public_request", "target_id": f.requestID, "reason": "spam"}).json(t, &report)
	f.reportID = report.ID

	var invite idPayload
	f.owner.mustDo(http.StatusCreated, http.MethodPost, "/api/groups/"+f.groupID+"/invite-user", map[string]string{"target_user_id": f.inviteTarget.ID}).json(t, &invite)
	f.inviteID = invite.ID

	var creation creationRequestPayload
	f.owner.mustDo(http.StatusCreated, http.MethodPost, "/api/group-creation-requests", groupCreationInput("Matrix pending "+suffix)).json(t, &creation)
	f.creationRequestID = creation.ID
	return f
}

func (f *fixture) actor(name string) *user {
	switch name {
	case actOutsider:
		return f.outsider
	case actMember:
		return f.member
	case actGroupAdmin:
		return f.groupAdmin
	case actGroupOwner:
		return f.owner
	case actPlatformAdm:
		return f.platformAdmin
	}
	panic("unknown actor " + name)
}

func (f *fixture) rows() []matrixRow {
	g := "/api/groups/" + f.groupID
	h := "/api/groups/" + f.otherGroupID
	avatar := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes())
	// A platform_admin is not a member of G and has no group-moderation rights.
	outsiderOnly := []string{actOutsider, actPlatformAdm}
	outsiderMember := []string{actOutsider, actPlatformAdm, actMember}
	everyoneButOwner := []string{actOutsider, actMember, actGroupAdmin}
	notPlatform := []string{actOutsider, actMember, actGroupAdmin, actGroupOwner}
	notSuper := []string{actOutsider, actMember, actGroupAdmin, actGroupOwner, actPlatformAdm}
	review := map[string]string{"admin_comment": "matrix"}

	return []matrixRow{
		// Group-scoped reads.
		{route: "GET /api/groups/{groupID}/members", path: g + "/members", deny: outsiderOnly},
		{route: "GET /api/groups/{groupID}/messages", path: g + "/messages", deny: outsiderOnly},
		{route: "GET /api/groups/{groupID}/messages", path: g + "/messages?after=2000-01-01T00:00:00Z&after_id=M-0", deny: outsiderOnly},
		{route: "GET /api/groups/{groupID}/ws", path: g + "/ws", deny: outsiderOnly},
		{route: "GET /api/groups/{groupID}/files/{fileID}", path: g + "/files/" + f.fileID, deny: outsiderOnly},
		{route: "GET /api/public-files/{groupID}/{fileID}", path: "/api/public-files/" + f.groupID + "/" + f.fileID, deny: outsiderOnly},
		// IDOR: a file of G requested through the outsider's own group H.
		{route: "GET /api/groups/{groupID}/files/{fileID}", path: h + "/files/" + f.fileID, deny: outsiderOnly},
		{route: "GET /api/public-files/{groupID}/{fileID}", path: "/api/public-files/" + f.otherGroupID + "/" + f.fileID, deny: outsiderOnly},
		{route: "GET /api/groups/{groupID}/requests", path: g + "/requests", deny: outsiderOnly},
		{route: "GET /api/groups/{groupID}/requests", path: g + "/requests?mine=true", deny: outsiderOnly},
		{route: "GET /api/groups/{groupID}/statistics", path: g + "/statistics?period=month", deny: outsiderOnly},
		{route: "GET /api/groups/{groupID}/moderation/items", path: g + "/moderation/items", deny: outsiderMember},
		{route: "GET /api/groups/{groupID}/moderation/items/count", path: g + "/moderation/items/count", deny: outsiderMember},
		{route: "GET /api/groups/{groupID}/reports", path: g + "/reports", deny: outsiderMember},
		{route: "GET /api/requests/{requestID}/comments", path: "/api/requests/" + f.requestID + "/comments", deny: outsiderOnly},

		// Group-scoped writes.
		// Only the owner and admins may see or rotate the invite code.
		{route: "POST /api/groups/{groupID}/invite-code", path: g + "/invite-code", deny: outsiderMember},
		{route: "POST /api/groups/{groupID}/invite-code/rotate", path: g + "/invite-code/rotate", deny: outsiderMember},
		// Removing the group admin: only the owner may (an admin may remove
		// plain members only, and nobody removes themselves).
		{route: "DELETE /api/groups/{groupID}/members/{userID}", path: g + "/members/" + f.groupAdmin.ID, deny: []string{actOutsider, actPlatformAdm, actMember, actGroupAdmin}},
		{route: "DELETE /api/groups/{groupID}/members/{userID}", path: g + "/members/" + f.owner.ID, deny: []string{actOutsider, actPlatformAdm, actMember, actGroupAdmin, actGroupOwner}},
		{route: "PUT /api/groups/{groupID}/avatar", path: g + "/avatar", body: map[string]string{"avatar_data": avatar}, deny: outsiderMember},
		{route: "POST /api/groups/{groupID}/members/{userID}/role", path: g + "/members/" + f.member.ID + "/role", body: map[string]string{"role": "admin"}, deny: outsiderMember},
		{route: "POST /api/groups/{groupID}/members/{userID}/role", path: g + "/members/" + f.groupAdmin.ID + "/role", body: map[string]string{"role": "member"}, deny: outsiderMember},
		{route: "POST /api/groups/{groupID}/members/role-by-phone", path: g + "/members/role-by-phone", body: map[string]string{"phone": f.member.Phone, "role": "admin"}, deny: outsiderMember},
		{route: "POST /api/groups/{groupID}/comment-mutes/{userID}", path: g + "/comment-mutes/" + f.member.ID, body: map[string]any{"duration_minutes": 60}, deny: outsiderMember},
		{route: "POST /api/groups/{groupID}/comment-mutes/{userID}/clear", path: g + "/comment-mutes/" + f.member.ID + "/clear", deny: outsiderMember},
		{route: "POST /api/groups/{groupID}/comment-mutes/by-phone", path: g + "/comment-mutes/by-phone", body: map[string]any{"phone": f.member.Phone, "duration_minutes": 60}, deny: outsiderMember},
		{route: "POST /api/groups/{groupID}/comment-mutes/unmute-by-phone", path: g + "/comment-mutes/unmute-by-phone", body: map[string]string{"phone": f.member.Phone}, deny: outsiderMember},
		{route: "DELETE /api/groups/{groupID}/comment-mutes/by-phone", path: g + "/comment-mutes/by-phone", body: map[string]string{"phone": f.member.Phone}, deny: outsiderMember},
		{route: "DELETE /api/groups/{groupID}/leave", path: g + "/leave", deny: outsiderOnly},
		{route: "POST /api/groups/{groupID}/invite-user", path: g + "/invite-user", body: map[string]string{"target_user_id": f.outsider.ID}, deny: outsiderMember},
		{route: "POST /api/groups/{groupID}/invite-user", path: g + "/invite-user", body: map[string]string{"phone": f.outsider.Phone}, deny: outsiderMember},
		{route: "POST /api/groups/{groupID}/messages", path: g + "/messages", body: map[string]string{"text": "intruder"}, deny: outsiderMember},
		{route: "PATCH /api/groups/{groupID}/messages/{messageID}", path: g + "/messages/" + f.messageID, body: map[string]string{"text": "tampered"}, deny: everyoneButOwner},
		{route: "DELETE /api/groups/{groupID}/messages/{messageID}", path: g + "/messages/" + f.messageID, deny: outsiderMember},
		// IDOR: G's message addressed through the outsider's own group H.
		{route: "PATCH /api/groups/{groupID}/messages/{messageID}", path: h + "/messages/" + f.messageID, body: map[string]string{"text": "tampered"}, deny: outsiderOnly},
		{route: "DELETE /api/groups/{groupID}/messages/{messageID}", path: h + "/messages/" + f.messageID, deny: outsiderOnly},
		{route: "POST /api/groups/{groupID}/files", path: g + "/files", multipart: true, deny: outsiderOnly},
		{route: "POST /api/groups/{groupID}/requests", path: g + "/requests", body: map[string]string{"request_type": "idea", "title": "Intruder", "body": "Intruder body"}, deny: outsiderOnly},
		{route: "POST /api/groups/{groupID}/requests/read", path: g + "/requests/read", deny: outsiderOnly},
		{route: "POST /api/groups/{groupID}/join", path: "/api/groups/" + f.privateGroupID + "/join", deny: outsiderOnly},

		// Request / comment scoped.
		{route: "POST /api/requests/{requestID}/support", path: "/api/requests/" + f.requestID + "/support", deny: outsiderOnly},
		{route: "POST /api/requests/{requestID}/oppose", path: "/api/requests/" + f.requestID + "/oppose", deny: outsiderOnly},
		{route: "DELETE /api/requests/{requestID}/vote", path: "/api/requests/" + f.requestID + "/vote", deny: outsiderOnly},
		{route: "POST /api/requests/{requestID}/comments", path: "/api/requests/" + f.requestID + "/comments", body: map[string]string{"body": "intruder comment"}, deny: outsiderOnly},
		{route: "DELETE /api/requests/comments/{commentID}", path: "/api/requests/comments/" + f.commentID, deny: outsiderMember},
		{route: "POST /api/requests/{requestID}/status", path: "/api/requests/" + f.requestID + "/status", body: map[string]string{"status": "rejected"}, deny: outsiderMember},
		{route: "POST /api/requests/{requestID}/hide", path: "/api/requests/" + f.requestID + "/hide", deny: outsiderMember},

		// Moderation items and reports.
		{route: "POST /api/moderation/items/{itemID}/approve", path: "/api/moderation/items/" + f.moderationItemID + "/approve", deny: outsiderMember},
		{route: "POST /api/moderation/items/{itemID}/reject", path: "/api/moderation/items/" + f.moderationItemID + "/reject", deny: outsiderMember},
		{route: "POST /api/reports", path: "/api/reports", body: map[string]string{"target_type": "message", "target_id": f.messageID, "reason": "abuse"}, deny: outsiderOnly},
		{route: "POST /api/reports", path: "/api/reports", body: map[string]string{"target_type": "comment", "target_id": f.commentID, "reason": "abuse"}, deny: outsiderOnly},
		{route: "POST /api/reports", path: "/api/reports", body: map[string]string{"target_type": "public_request", "target_id": f.requestID, "reason": "abuse"}, deny: outsiderOnly},
		{route: "POST /api/reports/{reportID}/resolve", path: "/api/reports/" + f.reportID + "/resolve", body: map[string]string{"action": "content_removed"}, deny: []string{actOutsider, actMember}},
		// (platform_admin may resolve any report by design: see ListAllReports / ensureCanResolveReport.)

		// Invites addressed to someone else.
		{route: "POST /api/invites/{inviteID}/accept", path: "/api/invites/" + f.inviteID + "/accept", deny: []string{actOutsider, actMember, actGroupAdmin, actGroupOwner}},
		{route: "POST /api/invites/{inviteID}/decline", path: "/api/invites/" + f.inviteID + "/decline", deny: []string{actOutsider, actMember, actGroupAdmin, actGroupOwner}},

		// Platform-level routes.
		{route: "GET /api/admin/group-creation-requests", path: "/api/admin/group-creation-requests", deny: notPlatform},
		{route: "POST /api/admin/group-creation-requests/{requestID}/approve", path: "/api/admin/group-creation-requests/" + f.creationRequestID + "/approve", body: review, deny: notPlatform},
		{route: "POST /api/admin/group-creation-requests/{requestID}/reject", path: "/api/admin/group-creation-requests/" + f.creationRequestID + "/reject", body: review, deny: notPlatform},
		{route: "POST /api/admin/group-creation-requests/{requestID}/need-more-info", path: "/api/admin/group-creation-requests/" + f.creationRequestID + "/need-more-info", body: review, deny: notPlatform},
		{route: "DELETE /api/admin/groups/{groupID}", path: "/api/admin/groups/" + f.groupID, deny: notSuper},
		{route: "GET /api/admin/reports", path: "/api/admin/reports", deny: notPlatform},
		{route: "POST /api/groups", path: "/api/groups", body: map[string]string{"title": "Matrix intruder group", "visibility": "public"}, deny: notSuper},
	}
}

// matrixExempt lists parameterised routes intentionally not in the matrix.
var matrixExempt = map[string]string{
	"POST /api/groups/{groupID}/join":  "covered with the private group; public groups are joinable by design",
	"POST /api/users/{userID}/block":   "user-scoped, any user may block any user",
	"DELETE /api/users/{userID}/block": "user-scoped, only affects the caller's own block list",
	"POST /api/groups/join-by-code":    "requires a valid code; unknown codes are 404 (TestE2EInvitesMembersAndRoles)",
}

func TestE2EAccessMatrix(t *testing.T) {
	a := newAPI(t)
	f := setupFixture(t, a)
	rows := f.rows()

	routes := map[string]bool{}
	checks := 0
	for _, row := range rows {
		routes[row.route] = true
		for _, actorName := range row.deny {
			row, actorName := row, actorName
			method := strings.SplitN(row.route, " ", 2)[0]
			t.Run(fmt.Sprintf("%s/%s/%s", actorName, row.route, row.path), func(t *testing.T) {
				u := f.actor(actorName)
				var res response
				if row.multipart {
					res = uploadFile(u, strings.TrimSuffix(strings.TrimPrefix(row.path, "/api/groups/"), "/files"))
				} else {
					res = u.api.call(method, row.path, u.AccessToken, row.body)
				}
				if res.Status != http.StatusForbidden && res.Status != http.StatusNotFound {
					t.Errorf("%s %s as %s: status %d, want 403/404; body=%s", method, row.path, actorName, res.Status, res.str())
				}
			})
			checks++
		}
	}

	// Denied calls must not have changed anything.
	t.Run("state-unchanged", func(t *testing.T) { f.assertUnchanged(t) })

	// Every parameterised / admin route in server.go must be in the matrix.
	registered := registeredRoutes(t)
	var missing []string
	scoped := 0
	for _, route := range registered {
		if !strings.Contains(route, "{") && !strings.Contains(route, "/api/admin/") && route != "POST /api/groups" && route != "POST /api/reports" {
			continue
		}
		scoped++
		if routes[route] {
			continue
		}
		if _, ok := matrixExempt[route]; ok {
			continue
		}
		missing = append(missing, route)
	}
	if len(missing) > 0 {
		t.Errorf("routes missing from the access matrix: %v", missing)
	}
	t.Logf("access matrix: %d distinct routes covered (%d scoped/admin routes registered, %d exempt), %d deny checks across %d rows",
		len(routes), scoped, len(matrixExempt), checks, len(rows))
	fmt.Printf("ACCESS MATRIX SUMMARY: routes_covered=%d scoped_routes=%d deny_checks=%d rows=%d\n", len(routes), scoped, checks, len(rows))
}

func (f *fixture) assertUnchanged(t *testing.T) {
	g := "/api/groups/" + f.groupID
	var msgs []struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	}
	f.owner.mustDo(http.StatusOK, http.MethodGet, g+"/messages", nil).json(t, &msgs)
	if len(msgs) != 1 || msgs[0].ID != f.messageID || msgs[0].Text != "matrix official message" {
		t.Errorf("messages changed: %+v", msgs)
	}
	var otherMsgs []struct {
		ID string `json:"id"`
	}
	f.outsider.mustDo(http.StatusOK, http.MethodGet, "/api/groups/"+f.otherGroupID+"/messages", nil).json(t, &otherMsgs)
	if len(otherMsgs) != 1 || otherMsgs[0].ID != f.otherGroupMessageID {
		t.Errorf("outsider's own group messages changed: %+v", otherMsgs)
	}
	var requests []struct {
		ID     string `json:"id"`
		Title  string `json:"title"`
		Status string `json:"status"`
	}
	f.owner.mustDo(http.StatusOK, http.MethodGet, g+"/requests", nil).json(t, &requests)
	if len(requests) != 1 || requests[0].ID != f.requestID || requests[0].Status != "new" {
		t.Errorf("requests changed: %+v", requests)
	}
	var comments []idPayload
	f.owner.mustDo(http.StatusOK, http.MethodGet, "/api/requests/"+f.requestID+"/comments", nil).json(t, &comments)
	if len(comments) != 1 || comments[0].ID != f.commentID {
		t.Errorf("comments changed: %+v", comments)
	}
	var count struct {
		Count int `json:"count"`
	}
	f.owner.mustDo(http.StatusOK, http.MethodGet, g+"/moderation/items/count?status=pending", nil).json(t, &count)
	if count.Count != 1 {
		t.Errorf("pending moderation items: %d, want 1", count.Count)
	}
	var reports []idPayload
	f.owner.mustDo(http.StatusOK, http.MethodGet, g+"/reports?status=open", nil).json(t, &reports)
	if len(reports) != 1 || reports[0].ID != f.reportID {
		t.Errorf("open reports changed: %+v", reports)
	}
	var members []memberPayload
	f.owner.mustDo(http.StatusOK, http.MethodGet, g+"/members", nil).json(t, &members)
	roles := map[string]string{}
	for _, m := range members {
		roles[m.UserID] = m.Role
	}
	want := map[string]string{f.owner.ID: "owner", f.groupAdmin.ID: "admin", f.member.ID: "member"}
	if len(roles) != len(want) {
		t.Errorf("members changed: %+v", roles)
	}
	for id, role := range want {
		if roles[id] != role {
			t.Errorf("member %s role %q, want %q", id, roles[id], role)
		}
	}
	var groups []struct {
		ID         string `json:"id"`
		AvatarData string `json:"avatar_data"`
	}
	f.owner.mustDo(http.StatusOK, http.MethodGet, "/api/groups", nil).json(t, &groups)
	foundGroup := false
	for _, group := range groups {
		if group.ID == f.groupID {
			foundGroup = true
			if group.AvatarData != "" {
				t.Errorf("group avatar changed")
			}
		}
	}
	if !foundGroup {
		t.Errorf("group G no longer listed for its owner")
	}
	// The plain member is not muted.
	createComment(t, f.member, f.requestID, "still allowed to comment")
	f.owner.mustDo(http.StatusOK, http.MethodDelete, "/api/requests/comments/"+lastCommentID(t, f.owner, f.requestID, f.commentID), nil)
	var invites []idPayload
	f.inviteTarget.mustDo(http.StatusOK, http.MethodGet, "/api/invites", nil).json(t, &invites)
	if len(invites) != 1 || invites[0].ID != f.inviteID {
		t.Errorf("invite changed: %+v", invites)
	}
	var mine []creationRequestPayload
	f.owner.mustDo(http.StatusOK, http.MethodGet, "/api/group-creation-requests", nil).json(t, &mine)
	for _, request := range mine {
		if request.ID == f.creationRequestID && request.Status != "pending" {
			t.Errorf("creation request reviewed by an unauthorized actor: %+v", request)
		}
	}
	// Nobody else sees the owner's creation requests.
	var outsiderRequests []creationRequestPayload
	f.outsider.mustDo(http.StatusOK, http.MethodGet, "/api/group-creation-requests", nil).json(t, &outsiderRequests)
	for _, request := range outsiderRequests {
		if request.RequesterID != f.outsider.ID {
			t.Errorf("outsider sees another user's creation request %s", request.ID)
		}
	}
	// The private group gained no members.
	var privateMembers []memberPayload
	f.superAdmin.mustDo(http.StatusOK, http.MethodGet, "/api/groups/"+f.privateGroupID+"/members", nil).json(t, &privateMembers)
	if len(privateMembers) != 1 {
		t.Errorf("private group members: %+v", privateMembers)
	}
	// Group search must not leak invite codes.
	res := f.outsider.mustDo(http.StatusOK, http.MethodGet, "/api/groups/search?q=Matrix", nil)
	if strings.Contains(res.str(), `"invite_code"`) {
		t.Errorf("group search leaks invite codes: %s", res.str())
	}
	// The outsider's group list does not include G.
	var outsiderGroups []idPayload
	f.outsider.mustDo(http.StatusOK, http.MethodGet, "/api/groups", nil).json(t, &outsiderGroups)
	for _, group := range outsiderGroups {
		if group.ID == f.groupID {
			t.Errorf("outsider became a member of G")
		}
	}
}

func lastCommentID(t *testing.T, u *user, requestID, exclude string) string {
	t.Helper()
	var comments []idPayload
	u.mustDo(http.StatusOK, http.MethodGet, "/api/requests/"+requestID+"/comments", nil).json(t, &comments)
	for _, c := range comments {
		if c.ID != exclude {
			return c.ID
		}
	}
	t.Fatalf("new comment not found")
	return ""
}

var routePattern = regexp.MustCompile(`r\.(Get|Post|Put|Patch|Delete)\("(/api/[^"]+)"`)

// registeredRoutes parses server.go so new routes cannot silently skip the matrix.
func registeredRoutes(t *testing.T) []string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	data, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "httpapi", "server.go"))
	if err != nil {
		t.Fatalf("read server.go: %v", err)
	}
	var routes []string
	for _, match := range routePattern.FindAllStringSubmatch(string(data), -1) {
		routes = append(routes, strings.ToUpper(match[1])+" "+match[2])
	}
	sort.Strings(routes)
	if len(routes) < 50 {
		t.Fatalf("parsed only %d routes from server.go", len(routes))
	}
	return routes
}

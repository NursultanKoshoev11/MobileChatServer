package e2e_test

import (
	"net/http"
	"strings"
	"testing"
)

// TestE2EEdgeCasesNo5xx sends malformed, missing and conflicting input to the
// authenticated API and requires a 4xx (never a 5xx) with the expected class.
func TestE2EEdgeCasesNo5xx(t *testing.T) {
	a := newAPI(t)
	f := setupFixture(t, a)
	g := "/api/groups/" + f.groupID
	missingGroup := "/api/groups/G-DOESNOTEXIST"
	long := strings.Repeat("x", 20000)

	type tc struct {
		who    *user
		method string
		path   string
		body   any
		want   []int
	}
	bad := []int{http.StatusBadRequest}
	notFound := []int{http.StatusNotFound}
	denied := []int{http.StatusForbidden, http.StatusNotFound}
	conflictish := []int{http.StatusBadRequest, http.StatusConflict, http.StatusNotFound, http.StatusForbidden}

	cases := []tc{
		// Unknown objects.
		{f.owner, http.MethodGet, missingGroup + "/members", nil, denied},
		{f.owner, http.MethodGet, missingGroup + "/messages", nil, denied},
		{f.owner, http.MethodGet, missingGroup + "/requests", nil, denied},
		{f.owner, http.MethodGet, missingGroup + "/statistics", nil, denied},
		{f.owner, http.MethodGet, missingGroup + "/moderation/items", nil, denied},
		{f.owner, http.MethodGet, missingGroup + "/reports", nil, denied},
		{f.owner, http.MethodPost, missingGroup + "/join", nil, notFound},
		{f.owner, http.MethodPost, missingGroup + "/invite-code", nil, denied},
		{f.owner, http.MethodPost, "/api/requests/REQ-DOESNOTEXIST/support", nil, notFound},
		{f.owner, http.MethodGet, "/api/requests/REQ-DOESNOTEXIST/comments", nil, notFound},
		{f.owner, http.MethodPost, "/api/requests/REQ-DOESNOTEXIST/comments", map[string]string{"body": "x"}, notFound},
		{f.owner, http.MethodPost, "/api/requests/REQ-DOESNOTEXIST/status", map[string]string{"status": "accepted"}, notFound},
		{f.owner, http.MethodPost, "/api/requests/REQ-DOESNOTEXIST/hide", nil, notFound},
		{f.owner, http.MethodDelete, "/api/requests/comments/COM-DOESNOTEXIST", nil, notFound},
		{f.owner, http.MethodPost, "/api/moderation/items/MOD-DOESNOTEXIST/approve", nil, notFound},
		{f.owner, http.MethodPost, "/api/moderation/items/MOD-DOESNOTEXIST/reject", nil, notFound},
		{f.owner, http.MethodPost, "/api/reports/RPT-DOESNOTEXIST/resolve", map[string]string{"action": "dismissed"}, notFound},
		{f.owner, http.MethodPost, "/api/invites/INV-DOESNOTEXIST/accept", nil, notFound},
		{f.owner, http.MethodPost, "/api/invites/INV-DOESNOTEXIST/decline", nil, notFound},
		// Unknown and foreign messages share one "not yours" answer.
		{f.owner, http.MethodPatch, g + "/messages/M-DOESNOTEXIST", map[string]string{"text": "x"}, denied},
		{f.owner, http.MethodDelete, g + "/messages/M-DOESNOTEXIST", nil, notFound},
		{f.owner, http.MethodGet, g + "/files/PF-DOESNOTEXIST0", nil, notFound},
		{f.owner, http.MethodPost, g + "/members/U-DOESNOTEXIST/role", map[string]string{"role": "admin"}, denied},
		{f.owner, http.MethodPost, g + "/comment-mutes/U-DOESNOTEXIST", map[string]any{"duration_minutes": 5}, denied},
		{f.owner, http.MethodPost, g + "/invite-user", map[string]string{"target_user_id": "U-DOESNOTEXIST"}, notFound},
		{f.owner, http.MethodPost, g + "/invite-user", map[string]string{"phone": "+19990000000"}, notFound},
		{f.owner, http.MethodPost, g + "/members/role-by-phone", map[string]string{"phone": "+19990000000", "role": "admin"}, notFound},
		{f.owner, http.MethodPost, "/api/reports", map[string]string{"target_type": "user", "target_id": "U-DOESNOTEXIST", "reason": "spam"}, notFound},
		{f.owner, http.MethodPost, "/api/users/U-DOESNOTEXIST/block", nil, notFound},
		{f.owner, http.MethodPost, "/api/admin/group-creation-requests/GCR-X/approve", map[string]string{}, denied},

		// Malformed input.
		{f.owner, http.MethodPost, g + "/messages", map[string]string{"text": ""}, bad},
		{f.owner, http.MethodPost, g + "/messages", map[string]string{"text": long}, bad},
		{f.owner, http.MethodPost, g + "/requests", map[string]string{"request_type": "idea", "title": "ab", "body": "Body text"}, bad},
		{f.owner, http.MethodPost, g + "/requests", map[string]string{"request_type": "idea", "interaction_mode": "bogus", "title": "Title", "body": "Body text"}, bad},
		{f.owner, http.MethodPost, g + "/requests", map[string]string{"request_type": "idea", "title": "Title", "body": `{"version":1,"text":"","photos":[]}`}, bad},
		{f.owner, http.MethodPost, g + "/requests", map[string]string{"request_type": "idea", "title": "Title", "body": `{"version":1,"text":"hello world","photos":[{"file_id":"PF-NOPE","url":"https://evil.example/x.png"}]}`}, bad},
		{f.owner, http.MethodPost, "/api/requests/" + f.requestID + "/comments", map[string]string{"body": ""}, bad},
		{f.owner, http.MethodPost, "/api/requests/" + f.requestID + "/comments", map[string]string{"body": long}, bad},
		{f.owner, http.MethodPost, g + "/members/" + f.member.ID + "/role", map[string]string{"role": "superuser"}, bad},
		{f.owner, http.MethodPost, g + "/comment-mutes/" + f.member.ID, map[string]any{"duration_minutes": -5}, bad},
		{f.owner, http.MethodPost, g + "/comment-mutes/by-phone", map[string]any{"phone": "not-a-phone", "duration_minutes": 5}, []int{http.StatusBadRequest, http.StatusNotFound}},
		{f.owner, http.MethodPost, "/api/reports", map[string]string{"target_type": "public_request", "target_id": f.requestID, "reason": "bogus"}, bad},
		{f.owner, http.MethodPost, "/api/reports/" + f.reportID + "/resolve", map[string]string{"action": "bogus"}, bad},
		{f.owner, http.MethodGet, g + "/reports?status=bogus", nil, bad},
		{f.owner, http.MethodGet, g + "/moderation/items?status=bogus", nil, bad},
		{f.owner, http.MethodGet, g + "/moderation/items/count?status=bogus", nil, bad},
		{f.owner, http.MethodGet, g + "/messages?before=yesterday", nil, bad},
		{f.owner, http.MethodGet, g + "/messages?after_id=M-1", nil, bad},
		{f.owner, http.MethodGet, g + "/messages?after=2020-01-01T00:00:00Z", nil, bad},
		{f.owner, http.MethodGet, g + "/requests?before=yesterday", nil, bad},
		{f.owner, http.MethodGet, g + "/requests?before_id=REQ-1", nil, bad},
		{f.owner, http.MethodGet, g + "/statistics?period=bogus", nil, []int{http.StatusOK, http.StatusBadRequest}},
		{f.owner, http.MethodGet, g + "/members?limit=-5&offset=-9", nil, []int{http.StatusOK}},
		{f.owner, http.MethodGet, g + "/messages?limit=100000", nil, []int{http.StatusOK}},
		{f.owner, http.MethodPost, "/api/groups/join-by-code", map[string]string{"invite_code": ""}, []int{http.StatusBadRequest, http.StatusNotFound}},
		{f.owner, http.MethodPost, "/api/push/register", map[string]string{"token": long, "platform": "android"}, bad},
		{f.owner, http.MethodDelete, "/api/push/token", map[string]string{"token": ""}, bad},
		{f.owner, http.MethodPost, "/api/group-creation-requests", map[string]string{}, bad},

		// Conflicting state.
		{f.owner, http.MethodDelete, g + "/leave", nil, conflictish},
		{f.owner, http.MethodPost, g + "/members/" + f.owner.ID + "/role", map[string]string{"role": "member"}, conflictish},
		{f.groupAdmin, http.MethodPost, g + "/comment-mutes/" + f.owner.ID, map[string]any{"duration_minutes": 5}, conflictish},
		{f.owner, http.MethodPost, g + "/comment-mutes/" + f.owner.ID, map[string]any{"duration_minutes": 5}, conflictish},
		{f.owner, http.MethodPost, g + "/invite-user", map[string]string{"target_user_id": f.member.ID}, conflictish},
		{f.owner, http.MethodPost, "/api/reports", map[string]string{"target_type": "user", "target_id": f.owner.ID, "reason": "spam"}, bad},
	}

	for _, c := range cases {
		res := c.who.do(c.method, c.path, c.body)
		ok := false
		for _, want := range c.want {
			if res.Status == want {
				ok = true
			}
		}
		if !ok {
			t.Errorf("%s %s as %s: status %d, want one of %v; body=%s", c.method, c.path, c.who.Name, res.Status, c.want, res.str())
		}
	}

	// Malformed JSON bodies and unknown fields.
	for _, path := range []string{g + "/messages", g + "/requests", "/api/reports", "/api/group-creation-requests"} {
		res := a.raw(http.MethodPost, path, f.owner.AccessToken, "application/json", strings.NewReader("{not json"))
		if res.Status != http.StatusBadRequest {
			t.Errorf("malformed JSON to %s: %d", path, res.Status)
		}
		res = a.raw(http.MethodPost, path, f.owner.AccessToken, "application/json", strings.NewReader(`{"unknown_field":1}`))
		if res.Status != http.StatusBadRequest {
			t.Errorf("unknown field to %s: %d", path, res.Status)
		}
	}
	// Upload with a non-media payload.
	res := a.raw(http.MethodPost, g+"/files", f.owner.AccessToken, "text/plain", strings.NewReader("hello"))
	if res.Status != http.StatusBadRequest {
		t.Errorf("non-multipart upload: %d", res.Status)
	}
}

// Hidden requests and their comments must stop being reachable for members.
func TestE2EHiddenRequestIsUnreachable(t *testing.T) {
	a := newAPI(t)
	admin := a.platformAdmin()
	owner := a.newUser("Hide Owner")
	member := a.newUser("Hide Member")
	group := createOwnedGroup(t, owner, admin, "Hidden group "+uniqueSuffix())
	joinWithCode(t, owner, member, group.ID)
	reqID := createRequest(t, member, group.ID, "To be hidden", "discussion")
	commentID := createComment(t, member, reqID, "comment on hidden")
	owner.mustDo(http.StatusOK, http.MethodPost, "/api/requests/"+reqID+"/hide", nil)

	for _, c := range []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, "/api/requests/" + reqID + "/comments", nil},
		{http.MethodPost, "/api/requests/" + reqID + "/comments", map[string]string{"body": "late comment"}},
		{http.MethodPost, "/api/requests/" + reqID + "/support", nil},
		{http.MethodPost, "/api/requests/" + reqID + "/oppose", nil},
		{http.MethodDelete, "/api/requests/" + reqID + "/vote", nil},
		{http.MethodPost, "/api/reports", map[string]string{"target_type": "public_request", "target_id": reqID, "reason": "spam"}},
		{http.MethodPost, "/api/reports", map[string]string{"target_type": "comment", "target_id": commentID, "reason": "spam"}},
	} {
		res := member.do(c.method, c.path, c.body)
		if res.Status != http.StatusNotFound && res.Status != http.StatusForbidden {
			t.Errorf("%s %s on hidden request: %d, want 403/404; body=%s", c.method, c.path, res.Status, res.str())
		}
	}
	// Hiding twice is not a 500.
	if res := owner.do(http.MethodPost, "/api/requests/"+reqID+"/hide", nil); res.Status >= 500 {
		t.Errorf("second hide: %d %s", res.Status, res.str())
	}
	if res := owner.do(http.MethodPost, "/api/requests/"+reqID+"/status", map[string]string{"status": "accepted"}); res.Status >= 500 || res.Status == http.StatusOK {
		t.Errorf("status change on hidden request: %d %s", res.Status, res.str())
	}
}

// Blocking hides the blocked user's public requests and comments too.
func TestE2EBlockHidesPublicContent(t *testing.T) {
	a := newAPI(t)
	admin := a.platformAdmin()
	owner := a.newUser("Block Owner")
	member := a.newUser("Block Member")
	pest := a.newUser("Block Pest")
	group := createOwnedGroup(t, owner, admin, "Block group "+uniqueSuffix())
	joinWithCode(t, owner, member, group.ID)
	joinWithCode(t, owner, pest, group.ID)
	ownReq := createRequest(t, member, group.ID, "Member's request", "discussion")
	pestReq := createRequest(t, pest, group.ID, "Pest's request", "discussion")
	createComment(t, pest, ownReq, "pest comment")

	member.mustDo(http.StatusNoContent, http.MethodPost, "/api/users/"+pest.ID+"/block", nil)
	var listed []idPayload
	member.mustDo(http.StatusOK, http.MethodGet, "/api/groups/"+group.ID+"/requests", nil).json(t, &listed)
	for _, item := range listed {
		if item.ID == pestReq {
			t.Errorf("blocked user's request still listed")
		}
	}
	var comments []idPayload
	member.mustDo(http.StatusOK, http.MethodGet, "/api/requests/"+ownReq+"/comments", nil).json(t, &comments)
	if len(comments) != 0 {
		t.Errorf("blocked user's comments still listed: %+v", comments)
	}
	// Blocking twice is idempotent.
	member.mustDo(http.StatusNoContent, http.MethodPost, "/api/users/"+pest.ID+"/block", nil)
}

// Repeated state transitions must be rejected cleanly, never 5xx.
func TestE2ERepeatedTransitions(t *testing.T) {
	a := newAPI(t)
	f := setupFixture(t, a)
	f.inviteTarget.mustDo(http.StatusOK, http.MethodPost, "/api/invites/"+f.inviteID+"/accept", nil)
	for _, path := range []string{"/api/invites/" + f.inviteID + "/accept", "/api/invites/" + f.inviteID + "/decline"} {
		if res := f.inviteTarget.do(http.MethodPost, path, nil); res.Status < 400 || res.Status >= 500 {
			t.Errorf("POST %s after accept: %d %s", path, res.Status, res.str())
		}
	}
	// Joining again by code is idempotent.
	joinWithCode(t, f.owner, f.member, f.groupID)

	f.owner.mustDo(http.StatusOK, http.MethodPost, "/api/reports/"+f.reportID+"/resolve", map[string]string{"action": "dismissed"})
	// Resolving again is idempotent and keeps the first resolution.
	var again struct {
		Status     string `json:"status"`
		Resolution string `json:"resolution"`
	}
	f.owner.mustDo(http.StatusOK, http.MethodPost, "/api/reports/"+f.reportID+"/resolve", map[string]string{"action": "dismissed"}).json(t, &again)
	if again.Status != "resolved" || again.Resolution != "dismissed" {
		t.Errorf("second resolve: %+v", again)
	}
	f.owner.mustDo(http.StatusOK, http.MethodDelete, "/api/requests/comments/"+f.commentID, nil)
	if res := f.owner.do(http.MethodDelete, "/api/requests/comments/"+f.commentID, nil); res.Status != http.StatusNotFound {
		t.Errorf("deleting comment twice: %d %s", res.Status, res.str())
	}
	f.owner.mustDo(http.StatusOK, http.MethodDelete, "/api/groups/"+f.groupID+"/messages/"+f.messageID, nil)
	if res := f.owner.do(http.MethodDelete, "/api/groups/"+f.groupID+"/messages/"+f.messageID, nil); res.Status != http.StatusNotFound {
		t.Errorf("deleting message twice: %d %s", res.Status, res.str())
	}
	if res := f.owner.do(http.MethodPatch, "/api/groups/"+f.groupID+"/messages/"+f.messageID, map[string]string{"text": "zombie"}); res.Status != http.StatusNotFound && res.Status != http.StatusForbidden {
		t.Errorf("editing deleted message: %d %s", res.Status, res.str())
	}
	// Reporting deleted content is a 404.
	if res := f.member.do(http.MethodPost, "/api/reports", map[string]string{"target_type": "message", "target_id": f.messageID, "reason": "spam"}); res.Status != http.StatusNotFound {
		t.Errorf("reporting deleted message: %d %s", res.Status, res.str())
	}
	// A member who left loses access to files and comments.
	f.member.mustDo(http.StatusOK, http.MethodDelete, "/api/groups/"+f.groupID+"/leave", nil)
	for _, path := range []string{
		"/api/groups/" + f.groupID + "/files/" + f.fileID,
		"/api/requests/" + f.requestID + "/comments",
		"/api/groups/" + f.groupID + "/statistics",
	} {
		if res := f.member.do(http.MethodGet, path, nil); res.Status != http.StatusForbidden && res.Status != http.StatusNotFound {
			t.Errorf("ex-member GET %s: %d", path, res.Status)
		}
	}
	// A demoted admin loses moderation rights immediately.
	f.owner.mustDo(http.StatusOK, http.MethodPost, "/api/groups/"+f.groupID+"/members/"+f.groupAdmin.ID+"/role", map[string]string{"role": "member"})
	f.groupAdmin.mustDo(http.StatusForbidden, http.MethodGet, "/api/groups/"+f.groupID+"/moderation/items", nil)
	f.groupAdmin.mustDo(http.StatusForbidden, http.MethodPost, "/api/moderation/items/"+f.moderationItemID+"/approve", nil)
}

// Regression: group_invite_requests had UNIQUE (group_id, target_user_id,
// status), so a second accepted (or declined) invite for the same user and
// group violated the constraint and the accept/decline returned 500.
func TestE2EReinviteAfterLeaveOrDecline(t *testing.T) {
	a := newAPI(t)
	admin := a.platformAdmin()
	owner := a.newUser("Reinvite Owner")
	target := a.newUser("Reinvite Target")
	group := createOwnedGroup(t, owner, admin, "Reinvite group "+uniqueSuffix())
	base := "/api/groups/" + group.ID

	invite := func() string {
		var created idPayload
		owner.mustDo(http.StatusCreated, http.MethodPost, base+"/invite-user", map[string]string{"target_user_id": target.ID}).json(t, &created)
		return created.ID
	}
	for round := 0; round < 2; round++ {
		target.mustDo(http.StatusOK, http.MethodPost, "/api/invites/"+invite()+"/accept", nil)
		target.mustDo(http.StatusOK, http.MethodGet, base+"/messages", nil)
		target.mustDo(http.StatusOK, http.MethodDelete, base+"/leave", nil)
	}
	for round := 0; round < 2; round++ {
		target.mustDo(http.StatusOK, http.MethodPost, "/api/invites/"+invite()+"/decline", nil)
	}
	// A repeated invite while one is pending reuses the pending invite.
	first, second := invite(), invite()
	if first != second {
		t.Fatalf("second pending invite %s, want reuse of %s", second, first)
	}
	var pending []idPayload
	target.mustDo(http.StatusOK, http.MethodGet, "/api/invites", nil).json(t, &pending)
	if len(pending) != 1 {
		t.Fatalf("pending invites: %+v", pending)
	}
}

package httpapi

import (
	"net/http"
	"testing"
)

func TestStrictRateLimitClassCoversAbuseProneEndpoints(t *testing.T) {
	cases := []struct {
		method, path, want string
	}{
		{http.MethodPost, "/api/groups/join-by-code", "group-join-code"},
		{http.MethodPost, "/api/groups/G-1/members/role-by-phone", "phone-lookup"},
		{http.MethodPost, "/api/groups/G-1/comment-mutes/by-phone", "phone-lookup"},
		{http.MethodDelete, "/api/groups/G-1/comment-mutes/by-phone", "phone-lookup"},
		{http.MethodPost, "/api/groups/G-1/comment-mutes/unmute-by-phone", "phone-lookup"},
		{http.MethodPost, "/api/auth/request-code", "auth-request-code"},
		{http.MethodPost, "/api/reports", "report-create"},
		{http.MethodPost, "/api/reports/RPT-1/resolve", ""},
		{http.MethodGet, "/api/groups", ""},
		{http.MethodGet, "/api/groups/G-1/members", ""},
	}
	for _, tc := range cases {
		if got := strictRateLimitClass(tc.method, tc.path); got != tc.want {
			t.Errorf("strictRateLimitClass(%s %s) = %q, want %q", tc.method, tc.path, got, tc.want)
		}
	}
}

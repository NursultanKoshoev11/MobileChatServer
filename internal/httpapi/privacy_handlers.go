package httpapi

import "net/http"

func (s *Server) privacyPolicy(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(privacyPolicyHTML))
}

const privacyPolicyHTML = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Koom Privacy Policy</title></head><body><h1>Koom Privacy Policy</h1><p>Last updated: July 8, 2026</p><p>Koom uses phone sign-in, user accounts, groups, posts, comments, votes, public requests, media uploads, notifications, and technical logs to provide and secure the service.</p><p>We do not sell personal information. We may use trusted service providers for hosting, storage, notifications, security, and support.</p><p>Users may contact the Koom support team for privacy requests.</p></body></html>`
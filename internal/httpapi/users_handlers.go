package httpapi

import (
	"net/http"
	"strconv"
)

func (s *Server) searchUsers(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	users, err := s.svc.SearchUsers(r.Context(), r.URL.Query().Get("q"), limit)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, users)
}

type updateAvatarInput struct {
	AvatarData string `json:"avatar_data"`
}

func (s *Server) updateMyAvatar(w http.ResponseWriter, r *http.Request) {
	var input updateAvatarInput
	if !readJSON(w, r, &input) {
		return
	}
	user, err := s.svc.UpdateUserAvatar(r.Context(), currentUser(r).ID, input.AvatarData)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, user)
}

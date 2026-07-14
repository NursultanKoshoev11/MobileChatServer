package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

type updateGroupAvatarInput struct {
	AvatarData string `json:"avatar_data"`
}

func (s *Server) updateGroupAvatar(w http.ResponseWriter, r *http.Request) {
	var input updateGroupAvatarInput
	if !readJSON(w, r, &input) {
		return
	}
	group, err := s.svc.UpdateGroupAvatar(
		r.Context(),
		currentUser(r).ID,
		chi.URLParam(r, "groupID"),
		input.AvatarData,
	)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, group)
}

package httpapi

import (
	"net/http"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/realtime"
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
	// Every member receives the broadcast; the invite code is for the owner and
	// admins only.
	broadcast := group
	broadcast.InviteCode = ""
	broadcast.QRPass = ""
	broadcast.MyRole = nil
	s.broadcastGroupAndUsers(r, group.ID, realtime.Event{Type: "group.avatar_updated", GroupID: group.ID, Payload: broadcast})
	writeJSON(w, http.StatusOK, group)
}

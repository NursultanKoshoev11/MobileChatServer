package httpapi

import (
	"net/http"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/storage"

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
	// Every member receives the broadcast; a private group's invite code is for
	// the owner and admins only.
	broadcast := group
	if !storage.CanSeeInviteCode(group.Visibility, domain.RoleMember) {
		broadcast.InviteCode = ""
	}
	broadcast.QRPass = ""
	broadcast.MyRole = nil
	s.broadcastGroupAndUsers(r, group.ID, realtime.Event{Type: "group.avatar_updated", GroupID: group.ID, Payload: broadcast})
	writeJSON(w, http.StatusOK, group)
}

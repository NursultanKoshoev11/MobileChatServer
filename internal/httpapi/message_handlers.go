package httpapi

import (
	"net/http"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/realtime"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/service"
	"github.com/go-chi/chi/v5"
)

func (s *Server) updateMessage(w http.ResponseWriter, r *http.Request) {
	var input service.UpdateMessageInput
	if !readJSON(w, r, &input) {
		return
	}
	groupID := chi.URLParam(r, "groupID")
	message, err := s.svc.UpdateMessage(r.Context(), currentUser(r).ID, groupID, chi.URLParam(r, "messageID"), input)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.broadcastGroupAndUsers(r, groupID, realtime.Event{Type: "message.updated", GroupID: groupID, Payload: message})
	writeJSON(w, http.StatusOK, message)
}

func (s *Server) deleteMessage(w http.ResponseWriter, r *http.Request) {
	groupID := chi.URLParam(r, "groupID")
	message, err := s.svc.DeleteMessage(r.Context(), currentUser(r).ID, groupID, chi.URLParam(r, "messageID"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.broadcastGroupAndUsers(r, groupID, realtime.Event{Type: "message.deleted", GroupID: groupID, Payload: message})
	writeJSON(w, http.StatusOK, message)
}

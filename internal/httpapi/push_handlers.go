package httpapi

import (
	"net/http"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/service"
)

func (s *Server) registerPushToken(w http.ResponseWriter, r *http.Request) {
	var input service.RegisterPushTokenInput
	if !readJSON(w, r, &input) {
		return
	}
	if err := s.svc.RegisterPushToken(r.Context(), currentUser(r).ID, input); err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "registered"})
}

func (s *Server) deletePushToken(w http.ResponseWriter, r *http.Request) {
	var input service.RegisterPushTokenInput
	if !readJSON(w, r, &input) {
		return
	}
	if err := s.svc.DeletePushToken(r.Context(), currentUser(r).ID, input); err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

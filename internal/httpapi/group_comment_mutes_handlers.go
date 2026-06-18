package httpapi

import (
	"net/http"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/service"
	"github.com/go-chi/chi/v5"
)

func (s *Server) setGroupCommentMute(w http.ResponseWriter, r *http.Request) {
	var input service.SetGroupCommentMuteInput
	if !readJSON(w, r, &input) {
		return
	}
	mute, err := s.svc.SetGroupCommentMute(r.Context(), currentUser(r).ID, chi.URLParam(r, "groupID"), chi.URLParam(r, "userID"), input)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mute)
}

func (s *Server) clearGroupCommentMute(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.ClearGroupCommentMute(r.Context(), currentUser(r).ID, chi.URLParam(r, "groupID"), chi.URLParam(r, "userID")); err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "unmuted"})
}

func (s *Server) setGroupCommentMuteByPhone(w http.ResponseWriter, r *http.Request) {
	var input service.SetGroupCommentMuteInput
	if !readJSON(w, r, &input) {
		return
	}
	mute, err := s.svc.SetGroupCommentMuteByPhone(r.Context(), currentUser(r).ID, chi.URLParam(r, "groupID"), input)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mute)
}

func (s *Server) clearGroupCommentMuteByPhone(w http.ResponseWriter, r *http.Request) {
	var input service.ClearGroupCommentMuteInput
	if !readJSON(w, r, &input) {
		return
	}
	if err := s.svc.ClearGroupCommentMuteByPhone(r.Context(), currentUser(r).ID, chi.URLParam(r, "groupID"), input); err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "unmuted"})
}

package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/service"
	"github.com/go-chi/chi/v5"
)

func (s *Server) createPublicRequest(w http.ResponseWriter, r *http.Request) {
	var input service.CreatePublicRequestInput
	if !readJSON(w, r, &input) {
		return
	}
	request, err := s.svc.CreatePublicRequest(r.Context(), currentUser(r).ID, chi.URLParam(r, "groupID"), input)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, request)
}

func (s *Server) listPublicRequests(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	var before time.Time
	if raw := r.URL.Query().Get("before"); raw != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, raw); err == nil {
			before = parsed
		}
	}
	mineOnly := r.URL.Query().Get("mine") == "true"
	requests, err := s.svc.ListPublicRequests(r.Context(), currentUser(r).ID, chi.URLParam(r, "groupID"), limit, before, mineOnly)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, requests)
}

func (s *Server) supportPublicRequest(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.VotePublicRequest(r.Context(), currentUser(r).ID, chi.URLParam(r, "requestID"), "support"); err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "supported"})
}

func (s *Server) opposePublicRequest(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.VotePublicRequest(r.Context(), currentUser(r).ID, chi.URLParam(r, "requestID"), "oppose"); err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "opposed"})
}

func (s *Server) clearPublicRequestVote(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.ClearPublicRequestVote(r.Context(), currentUser(r).ID, chi.URLParam(r, "requestID")); err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "cleared"})
}

func (s *Server) createPublicRequestComment(w http.ResponseWriter, r *http.Request) {
	var input service.CreatePublicRequestCommentInput
	if !readJSON(w, r, &input) {
		return
	}
	comment, err := s.svc.CreatePublicRequestComment(r.Context(), currentUser(r).ID, chi.URLParam(r, "requestID"), input)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, comment)
}

func (s *Server) listPublicRequestComments(w http.ResponseWriter, r *http.Request) {
	comments, err := s.svc.ListPublicRequestComments(r.Context(), currentUser(r).ID, chi.URLParam(r, "requestID"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, comments)
}

func (s *Server) deletePublicRequestComment(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.DeletePublicRequestComment(r.Context(), currentUser(r).ID, chi.URLParam(r, "commentID")); err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) updatePublicRequestStatus(w http.ResponseWriter, r *http.Request) {
	var input service.UpdatePublicRequestStatusInput
	if !readJSON(w, r, &input) {
		return
	}
	if err := s.svc.UpdatePublicRequestStatus(r.Context(), currentUser(r).ID, chi.URLParam(r, "requestID"), input.Status); err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

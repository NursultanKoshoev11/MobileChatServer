package httpapi

import (
	"net/http"
	"strconv"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/service"
	"github.com/go-chi/chi/v5"
)

func (s *Server) createGroupCreationRequest(w http.ResponseWriter, r *http.Request) {
	var input service.CreateGroupCreationRequestInput
	if !readJSON(w, r, &input) {
		return
	}
	request, err := s.svc.CreateGroupCreationRequest(r.Context(), currentUser(r), input)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, request)
}

func (s *Server) listMyGroupCreationRequests(w http.ResponseWriter, r *http.Request) {
	requests, err := s.svc.ListMyGroupCreationRequests(r.Context(), currentUser(r))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, requests)
}

func (s *Server) listGroupCreationRequestsForAdmin(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	requests, err := s.svc.ListGroupCreationRequestsForAdmin(r.Context(), currentUser(r), r.URL.Query().Get("status"), limit)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, requests)
}

func (s *Server) approveGroupCreationRequest(w http.ResponseWriter, r *http.Request) {
	var input service.ReviewGroupCreationRequestInput
	if !readJSON(w, r, &input) {
		return
	}
	request, err := s.svc.ApproveGroupCreationRequest(r.Context(), currentUser(r), chi.URLParam(r, "requestID"), input)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, request)
}

func (s *Server) rejectGroupCreationRequest(w http.ResponseWriter, r *http.Request) {
	var input service.ReviewGroupCreationRequestInput
	if !readJSON(w, r, &input) {
		return
	}
	request, err := s.svc.RejectGroupCreationRequest(r.Context(), currentUser(r), chi.URLParam(r, "requestID"), input)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, request)
}

func (s *Server) needMoreInfoForGroupCreationRequest(w http.ResponseWriter, r *http.Request) {
	var input service.ReviewGroupCreationRequestInput
	if !readJSON(w, r, &input) {
		return
	}
	request, err := s.svc.NeedMoreInfoForGroupCreationRequest(r.Context(), currentUser(r), chi.URLParam(r, "requestID"), input)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, request)
}

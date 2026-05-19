package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/realtime"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/service"
	"github.com/go-chi/chi/v5"
)

func (s *Server) createPublicRequest(w http.ResponseWriter, r *http.Request) {
	var input service.CreatePublicRequestInput
	if !readJSON(w, r, &input) {
		return
	}
	groupID := chi.URLParam(r, "groupID")
	request, err := s.svc.CreatePublicRequest(r.Context(), currentUser(r).ID, groupID, input)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.hub.BroadcastGroup(groupID, realtime.Event{Type: "public_request.created", GroupID: groupID, Payload: request})
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
	requestID := chi.URLParam(r, "requestID")
	if err := s.svc.VotePublicRequest(r.Context(), currentUser(r).ID, requestID, "support"); err != nil {
		s.writeError(w, err)
		return
	}
	s.broadcastPublicRequestRefresh(r, requestID, "public_request.voted", map[string]string{"request_id": requestID, "vote_type": "support"})
	writeJSON(w, http.StatusOK, map[string]string{"status": "supported"})
}

func (s *Server) opposePublicRequest(w http.ResponseWriter, r *http.Request) {
	requestID := chi.URLParam(r, "requestID")
	if err := s.svc.VotePublicRequest(r.Context(), currentUser(r).ID, requestID, "oppose"); err != nil {
		s.writeError(w, err)
		return
	}
	s.broadcastPublicRequestRefresh(r, requestID, "public_request.voted", map[string]string{"request_id": requestID, "vote_type": "oppose"})
	writeJSON(w, http.StatusOK, map[string]string{"status": "opposed"})
}

func (s *Server) clearPublicRequestVote(w http.ResponseWriter, r *http.Request) {
	requestID := chi.URLParam(r, "requestID")
	if err := s.svc.ClearPublicRequestVote(r.Context(), currentUser(r).ID, requestID); err != nil {
		s.writeError(w, err)
		return
	}
	s.broadcastPublicRequestRefresh(r, requestID, "public_request.vote_cleared", map[string]string{"request_id": requestID})
	writeJSON(w, http.StatusOK, map[string]string{"status": "cleared"})
}

func (s *Server) createPublicRequestComment(w http.ResponseWriter, r *http.Request) {
	var input service.CreatePublicRequestCommentInput
	if !readJSON(w, r, &input) {
		return
	}
	requestID := chi.URLParam(r, "requestID")
	comment, err := s.svc.CreatePublicRequestComment(r.Context(), currentUser(r).ID, requestID, input)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.broadcastPublicRequestRefresh(r, requestID, "public_request.comment_created", map[string]any{"request_id": requestID, "comment": comment})
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
	requestID := chi.URLParam(r, "requestID")
	if err := s.svc.UpdatePublicRequestStatus(r.Context(), currentUser(r).ID, requestID, input.Status); err != nil {
		s.writeError(w, err)
		return
	}
	s.broadcastPublicRequestRefresh(r, requestID, "public_request.status_updated", map[string]string{"request_id": requestID, "status": string(input.Status)})
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *Server) broadcastPublicRequestRefresh(r *http.Request, requestID string, eventType string, payload any) {
	ctx, err := s.svc.GetPublicRequestRealtimeContext(r.Context(), requestID)
	if err != nil || ctx.GroupID == "" {
		return
	}
	s.hub.BroadcastGroup(ctx.GroupID, realtime.Event{Type: eventType, GroupID: ctx.GroupID, Payload: payload})
}

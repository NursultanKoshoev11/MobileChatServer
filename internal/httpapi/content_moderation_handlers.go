package httpapi

import (
	"net/http"
	"strconv"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/realtime"
	"github.com/go-chi/chi/v5"
)

func (s *Server) listContentModerationItems(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	items, err := s.svc.ListContentModerationItems(r.Context(), currentUser(r).ID, chi.URLParam(r, "groupID"), r.URL.Query().Get("status"), limit)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) countContentModerationItems(w http.ResponseWriter, r *http.Request) {
	count, err := s.svc.CountContentModerationItems(r.Context(), currentUser(r).ID, chi.URLParam(r, "groupID"), r.URL.Query().Get("status"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"count": count})
}

func (s *Server) approveContentModerationItem(w http.ResponseWriter, r *http.Request) {
	result, err := s.svc.ApproveContentModerationItem(r.Context(), currentUser(r).ID, chi.URLParam(r, "itemID"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	if result.Message != nil {
		s.hub.BroadcastGroup(result.Item.GroupID, realtime.Event{Type: "message.created", GroupID: result.Item.GroupID, Payload: result.Message})
	}
	if result.PublicRequest != nil {
		s.hub.BroadcastGroup(result.Item.GroupID, realtime.Event{Type: "public_request.created", GroupID: result.Item.GroupID, Payload: result.PublicRequest})
	}
	if result.Comment != nil {
		s.broadcastPublicRequestRefresh(r, result.Comment.RequestID, "public_request.comment_created", map[string]any{"request_id": result.Comment.RequestID, "comment": result.Comment})
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) rejectContentModerationItem(w http.ResponseWriter, r *http.Request) {
	item, err := s.svc.RejectContentModerationItem(r.Context(), currentUser(r).ID, chi.URLParam(r, "itemID"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

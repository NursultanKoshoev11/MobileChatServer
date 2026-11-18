package httpapi

import (
	"net/http"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/realtime"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/service"
	"github.com/go-chi/chi/v5"
)

func (s *Server) createReport(w http.ResponseWriter, r *http.Request) {
	var input service.CreateReportInput
	if !readJSON(w, r, &input) {
		return
	}
	report, created, err := s.svc.CreateReport(r.Context(), currentUser(r).ID, input)
	if err != nil {
		s.writeError(w, err)
		return
	}
	status := http.StatusCreated
	if !created {
		status = http.StatusOK
	}
	writeJSON(w, status, map[string]any{
		"id":         report.ID,
		"status":     report.Status,
		"created_at": report.CreatedAt,
	})
}

func (s *Server) listGroupReports(w http.ResponseWriter, r *http.Request) {
	status, err := service.ParseReportStatus(r.URL.Query().Get("status"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	reports, err := s.svc.ListGroupReports(r.Context(), currentUser(r).ID, chi.URLParam(r, "groupID"), status)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, reports)
}

func (s *Server) listAdminReports(w http.ResponseWriter, r *http.Request) {
	status, err := service.ParseReportStatus(r.URL.Query().Get("status"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	reports, err := s.svc.ListAllReports(r.Context(), currentUser(r).ID, status)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, reports)
}

func (s *Server) resolveReport(w http.ResponseWriter, r *http.Request) {
	var input service.ResolveReportInput
	if !readJSON(w, r, &input) {
		return
	}
	report, removed, err := s.svc.ResolveReport(r.Context(), currentUser(r).ID, chi.URLParam(r, "reportID"), input)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.broadcastRemovedContent(r, removed)
	writeJSON(w, http.StatusOK, report)
}

// broadcastRemovedContent emits the same realtime events the regular
// moderation endpoints emit, so clients drop removed content immediately.
func (s *Server) broadcastRemovedContent(r *http.Request, removed service.RemovedContent) {
	if removed.GroupID == "" {
		return
	}
	switch removed.TargetType {
	case domain.ReportTargetMessage:
		if removed.Message != nil {
			s.broadcastGroupAndUsers(r, removed.GroupID, realtime.Event{Type: "message.deleted", GroupID: removed.GroupID, Payload: removed.Message})
		}
	case domain.ReportTargetComment:
		s.broadcastGroupAndUsers(r, removed.GroupID, realtime.Event{Type: "public_request.comment_deleted", GroupID: removed.GroupID, Payload: map[string]string{"request_id": removed.RequestID, "comment_id": removed.CommentID}})
	case domain.ReportTargetPublicRequest:
		s.broadcastGroupAndUsers(r, removed.GroupID, realtime.Event{Type: "public_request.hidden", GroupID: removed.GroupID, Payload: map[string]string{"request_id": removed.RequestID}})
	}
}

func (s *Server) blockUser(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.BlockUser(r.Context(), currentUser(r).ID, chi.URLParam(r, "userID")); err != nil {
		s.writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) unblockUser(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.UnblockUser(r.Context(), currentUser(r).ID, chi.URLParam(r, "userID")); err != nil {
		s.writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listMyBlocks(w http.ResponseWriter, r *http.Request) {
	blocks, err := s.svc.ListUserBlocks(r.Context(), currentUser(r).ID)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, blocks)
}

package httpapi

import (
	"net/http"
	"time"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/service"
	"github.com/go-chi/chi/v5"
)

func (s *Server) groupStatistics(w http.ResponseWriter, r *http.Request) {
	input := service.PublicRequestStatisticsInput{
		Period:      r.URL.Query().Get("period"),
		Granularity: r.URL.Query().Get("granularity"),
	}
	if raw := r.URL.Query().Get("from"); raw != "" {
		if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
			input.From = parsed
		}
	}
	if raw := r.URL.Query().Get("to"); raw != "" {
		if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
			input.To = parsed
		}
	}

	stats, err := s.svc.GetPublicRequestStatistics(r.Context(), currentUser(r).ID, chi.URLParam(r, "groupID"), input)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

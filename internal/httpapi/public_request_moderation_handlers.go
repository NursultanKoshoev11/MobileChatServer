package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *Server) hidePublicRequest(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.HidePublicRequest(r.Context(), currentUser(r).ID, chi.URLParam(r, "requestID")); err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "hidden"})
}

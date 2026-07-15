package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *Server) deleteGroupAsPlatformAdmin(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.DeleteGroupAsPlatformAdmin(r.Context(), currentUser(r), chi.URLParam(r, "groupID")); err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

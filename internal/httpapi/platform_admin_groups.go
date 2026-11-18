package httpapi

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

func (s *Server) deleteGroupAsPlatformAdmin(w http.ResponseWriter, r *http.Request) {
	groupID := chi.URLParam(r, "groupID")
	if err := s.svc.DeleteGroupAsPlatformAdmin(r.Context(), currentUser(r), groupID); err != nil {
		s.writeError(w, err)
		return
	}
	s.hub.CloseGroup(strings.TrimSpace(groupID))
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

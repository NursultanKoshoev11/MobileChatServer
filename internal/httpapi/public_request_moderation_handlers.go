package httpapi

import "net/http"

func (s *Server) hidePublicRequest(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.HidePublicRequest(r.Context(), currentUser(r).ID, chiURLParam(r, "requestID")); err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "hidden"})
}

func chiURLParam(r *http.Request, key string) string {
	return chi.URLParam(r, key)
}

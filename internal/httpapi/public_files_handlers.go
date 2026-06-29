package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
)

const maxPublicFileBytes = 12 << 20

var publicFileIDPattern = regexp.MustCompile(`^[A-Z0-9-]{8,80}$`)

func (s *Server) uploadPublicFile(w http.ResponseWriter, r *http.Request) {
	groupID := chi.URLParam(r, "groupID")
	if err := s.svc.EnsureGroupMember(r.Context(), currentUser(r).ID, groupID); err != nil {
		s.writeError(w, err)
		return
	}
	if err := r.ParseMultipartForm(maxPublicFileBytes); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid multipart form"})
		return
	}
	input, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file is required"})
		return
	}
	defer input.Close()

	kind := strings.TrimSpace(r.FormValue("kind"))
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if !validPublicFileKind(kind, contentType) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid file type"})
		return
	}

	id := "PF-" + randomPublicFileID()
	ext := publicFileExt(header.Filename, contentType)
	dir := filepath.Join(publicFileRoot(), groupID)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		s.writeError(w, err)
		return
	}
	path := filepath.Join(dir, id+ext)
	out, err := os.Create(path)
	if err != nil {
		s.writeError(w, err)
		return
	}
	written, copyErr := io.Copy(out, io.LimitReader(input, maxPublicFileBytes+1))
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(path)
		if copyErr != nil {
			s.writeError(w, copyErr)
		} else {
			s.writeError(w, closeErr)
		}
		return
	}
	if written > maxPublicFileBytes {
		_ = os.Remove(path)
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "file is too large"})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":           id,
		"kind":         kind,
		"file_name":    filepath.Base(header.Filename),
		"content_type": contentType,
		"size_bytes":   written,
		"url":          fmt.Sprintf("/api/public-files/%s/%s", groupID, id),
	})
}

func validPublicFileKind(kind string, contentType string) bool {
	contentType = strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	if kind == "photo" {
		switch contentType {
		case "image/jpeg", "image/png", "image/webp":
			return true
		default:
			return false
		}
	}
	if kind == "video" {
		switch contentType {
		case "video/mp4", "video/quicktime", "video/webm":
			return true
		default:
			return false
		}
	}
	return false
}

func publicFileExt(fileName string, contentType string) string {
	ext := strings.ToLower(filepath.Ext(fileName))
	if ext != "" && len(ext) <= 12 {
		return ext
	}
	exts, _ := mime.ExtensionsByType(contentType)
	if len(exts) > 0 {
		return exts[0]
	}
	return ".bin"
}

func publicFileRoot() string {
	if value := strings.TrimSpace(os.Getenv("PUBLIC_FILE_DIR")); value != "" {
		return value
	}
	return "data/public-files"
}

func randomPublicFileID() string {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return strings.ToUpper(hex.EncodeToString([]byte(fmt.Sprintf("%d", os.Getpid()))))
	}
	return strings.ToUpper(hex.EncodeToString(buf))
}

func (s *Server) servePublicFile(w http.ResponseWriter, r *http.Request) {
	groupID := chi.URLParam(r, "groupID")
	if user := currentUser(r); user.ID != "" {
		if err := s.svc.EnsureGroupMember(r.Context(), user.ID, groupID); err != nil {
			s.writeError(w, err)
			return
		}
	}
	fileID := chi.URLParam(r, "fileID")
	if !publicFileIDPattern.MatchString(fileID) {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	matches, err := filepath.Glob(filepath.Join(publicFileRoot(), groupID, fileID+".*"))
	if err != nil || len(matches) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	http.ServeFile(w, r, matches[0])
}

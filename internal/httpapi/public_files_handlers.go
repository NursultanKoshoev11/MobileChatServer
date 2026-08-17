package httpapi

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
)

const (
	maxPublicFileBytes          = 12 << 20
	maxPublicUploadRequestBytes = maxPublicFileBytes + (1 << 20)
)

var (
	publicFileIDPattern = regexp.MustCompile(`^[A-Z0-9-]{8,80}$`)
	publicGroupIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,80}$`)
)

func (s *Server) uploadPublicFile(w http.ResponseWriter, r *http.Request) {
	groupID := chi.URLParam(r, "groupID")
	if !publicGroupIDPattern.MatchString(groupID) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid group id"})
		return
	}
	if err := s.svc.EnsureGroupMember(r.Context(), currentUser(r).ID, groupID); err != nil {
		s.writeError(w, err)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxPublicUploadRequestBytes)
	if err := r.ParseMultipartForm(maxPublicFileBytes); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "upload request is too large"})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid multipart form"})
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}

	input, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file is required"})
		return
	}
	defer input.Close()

	kind := strings.TrimSpace(r.FormValue("kind"))
	prefix := make([]byte, 512)
	n, readErr := io.ReadFull(input, prefix)
	if readErr != nil && readErr != io.ErrUnexpectedEOF && readErr != io.EOF {
		s.writeError(w, readErr)
		return
	}
	prefix = prefix[:n]
	contentType, ext := detectPublicMedia(prefix)
	if !validPublicFileKind(kind, contentType) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid or unsupported file content"})
		return
	}

	id := "PF-" + randomPublicFileID()
	dir := filepath.Join(publicFileRoot(), groupID)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		s.writeError(w, err)
		return
	}
	path := filepath.Join(dir, id+ext)
	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		s.writeError(w, err)
		return
	}

	reader := io.MultiReader(bytes.NewReader(prefix), input)
	written, copyErr := io.Copy(out, io.LimitReader(reader, maxPublicFileBytes+1))
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

func validPublicFileKind(kind, contentType string) bool {
	switch kind {
	case "photo":
		return contentType == "image/jpeg" || contentType == "image/png" || contentType == "image/webp"
	case "video":
		return contentType == "video/mp4" || contentType == "video/quicktime" || contentType == "video/webm"
	default:
		return false
	}
}

func detectPublicMedia(prefix []byte) (contentType, ext string) {
	if len(prefix) >= 3 && prefix[0] == 0xff && prefix[1] == 0xd8 && prefix[2] == 0xff {
		return "image/jpeg", ".jpg"
	}
	if len(prefix) >= 8 && bytes.Equal(prefix[:8], []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}) {
		return "image/png", ".png"
	}
	if len(prefix) >= 12 && bytes.Equal(prefix[:4], []byte("RIFF")) && bytes.Equal(prefix[8:12], []byte("WEBP")) {
		return "image/webp", ".webp"
	}
	if len(prefix) >= 12 && bytes.Equal(prefix[4:8], []byte("ftyp")) {
		brand := string(prefix[8:12])
		if brand == "qt  " {
			return "video/quicktime", ".mov"
		}
		return "video/mp4", ".mp4"
	}
	if len(prefix) >= 4 && bytes.Equal(prefix[:4], []byte{0x1a, 0x45, 0xdf, 0xa3}) {
		return "video/webm", ".webm"
	}
	return "", ""
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
	if !publicGroupIDPattern.MatchString(groupID) {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if err := s.svc.EnsureGroupMember(r.Context(), currentUser(r).ID, groupID); err != nil {
		s.writeError(w, err)
		return
	}
	fileID := chi.URLParam(r, "fileID")
	if !publicFileIDPattern.MatchString(fileID) {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	matches, err := filepath.Glob(filepath.Join(publicFileRoot(), groupID, fileID+".*"))
	if err != nil || len(matches) != 1 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	switch strings.ToLower(filepath.Ext(matches[0])) {
	case ".jpg": w.Header().Set("Content-Type", "image/jpeg")
	case ".png": w.Header().Set("Content-Type", "image/png")
	case ".webp": w.Header().Set("Content-Type", "image/webp")
	case ".mp4": w.Header().Set("Content-Type", "video/mp4")
	case ".mov": w.Header().Set("Content-Type", "video/quicktime")
	case ".webm": w.Header().Set("Content-Type", "video/webm")
	default:
		w.WriteHeader(http.StatusNotFound)
		return
	}
	http.ServeFile(w, r, matches[0])
}


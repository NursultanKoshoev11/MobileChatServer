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
	"time"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/media"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/storage"
	"github.com/go-chi/chi/v5"
)

const (
	maxPublicFileBytes          = 12 << 20
	maxPublicUploadRequestBytes = maxPublicFileBytes + (1 << 20)
	// multipartMemoryBytes is how much of an upload is buffered in memory;
	// the rest spills to a temporary file (os.TempDir, a tmpfs in production).
	multipartMemoryBytes = 1 << 20
	// fileTransferDeadline replaces the server-wide read/write timeouts for
	// file uploads and downloads so a 12 MB transfer survives a slow mobile
	// link.
	fileTransferDeadline = 5 * time.Minute
)

var (
	publicFileIDPattern  = regexp.MustCompile(`^[A-Z0-9-]{8,80}$`)
	publicGroupIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,80}$`)
)

// extendTransferDeadlines lifts the server's ReadTimeout/WriteTimeout for this
// request only.
func extendTransferDeadlines(w http.ResponseWriter) {
	controller := http.NewResponseController(w)
	deadline := time.Now().Add(fileTransferDeadline)
	_ = controller.SetReadDeadline(deadline)
	_ = controller.SetWriteDeadline(deadline)
}

func (s *Server) uploadPublicFile(w http.ResponseWriter, r *http.Request) {
	extendTransferDeadlines(w)
	groupID := chi.URLParam(r, "groupID")
	if !publicGroupIDPattern.MatchString(groupID) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid group id"})
		return
	}
	userID := currentUser(r).ID
	if err := s.svc.EnsureGroupMember(r.Context(), userID, groupID); err != nil {
		s.writeError(w, err)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxPublicUploadRequestBytes)
	if err := r.ParseMultipartForm(multipartMemoryBytes); err != nil {
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

	if header.Size > maxPublicFileBytes {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "file is too large"})
		return
	}
	prefix := make([]byte, 512)
	n, readErr := io.ReadFull(input, prefix)
	if readErr != nil && readErr != io.ErrUnexpectedEOF && readErr != io.EOF {
		s.writeError(w, readErr)
		return
	}
	prefix = prefix[:n]

	kind := strings.TrimSpace(r.FormValue("kind"))
	contentType, ext := detectPublicMedia(prefix)
	if !validPublicFileKind(kind, contentType) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid or unsupported file content"})
		return
	}
	// Videos are streamed from the multipart temp file to disk. Photos are
	// read into memory once to remove EXIF (GPS, device) and text metadata
	// without re-encoding.
	var source io.Reader = io.MultiReader(bytes.NewReader(prefix), input)
	size := header.Size
	if kind == "photo" {
		data, err := io.ReadAll(io.LimitReader(source, maxPublicFileBytes+1))
		if err != nil {
			s.writeError(w, err)
			return
		}
		if len(data) > maxPublicFileBytes {
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "file is too large"})
			return
		}
		data = media.StripImageMetadata(data)
		source = bytes.NewReader(data)
		size = int64(len(data))
	}

	id := "PF-" + randomPublicFileID()
	if err := s.svc.RecordGroupUpload(r.Context(), storage.UploadedFile{
		ID:          id,
		GroupID:     groupID,
		UploaderID:  userID,
		SizeBytes:   size,
		ContentType: contentType,
		StoragePath: groupID + "/" + id + ext,
	}); err != nil {
		s.writeError(w, err)
		return
	}
	forget := func() {
		if err := s.svc.ForgetGroupUpload(r.Context(), id); err != nil {
			s.logger.Printf("upload record cleanup failed file_id=%q error=%q", id, err.Error())
		}
	}

	dir := filepath.Join(publicFileRoot(), groupID)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		forget()
		s.writeError(w, err)
		return
	}
	path := filepath.Join(dir, id+ext)
	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		forget()
		s.writeError(w, err)
		return
	}
	written, copyErr := io.Copy(out, io.LimitReader(source, maxPublicFileBytes+1))
	closeErr := out.Close()
	if copyErr == nil && written > maxPublicFileBytes {
		_ = os.Remove(path)
		forget()
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "file is too large"})
		return
	}
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(path)
		forget()
		if copyErr != nil {
			s.writeError(w, copyErr)
		} else {
			s.writeError(w, closeErr)
		}
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
		return contentType == media.TypeJPEG || contentType == media.TypePNG || contentType == media.TypeWebP
	case "video":
		return contentType == media.TypeMP4 || contentType == media.TypeQuickTime || contentType == media.TypeWebM
	default:
		return false
	}
}

func detectPublicMedia(prefix []byte) (contentType, ext string) {
	switch media.ImageType(prefix) {
	case media.TypeJPEG:
		return media.TypeJPEG, ".jpg"
	case media.TypePNG:
		return media.TypePNG, ".png"
	case media.TypeWebP:
		return media.TypeWebP, ".webp"
	}
	return media.VideoType(prefix)
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

// removeGroupFiles deletes a deleted group's stored files (best effort).
func (s *Server) removeGroupFiles(groupID string) {
	groupID = strings.TrimSpace(groupID)
	if !publicGroupIDPattern.MatchString(groupID) {
		return
	}
	if err := os.RemoveAll(filepath.Join(publicFileRoot(), groupID)); err != nil {
		s.logger.Printf("group file cleanup failed group_id=%q error=%q", groupID, err.Error())
	}
}

// removeStoredFiles deletes recorded uploads from disk (best effort).
func (s *Server) removeStoredFiles(files []storage.UploadedFile) {
	root := publicFileRoot()
	for _, file := range files {
		parts := strings.Split(file.StoragePath, "/")
		if len(parts) != 2 || !publicGroupIDPattern.MatchString(parts[0]) ||
			!publicFileIDPattern.MatchString(strings.TrimSuffix(parts[1], filepath.Ext(parts[1]))) {
			s.logger.Printf("stored file cleanup skipped: unexpected path file_id=%q", file.ID)
			continue
		}
		if err := os.Remove(filepath.Join(root, parts[0], parts[1])); err != nil && !errors.Is(err, os.ErrNotExist) {
			s.logger.Printf("stored file cleanup failed file_id=%q error=%q", file.ID, err.Error())
		}
	}
}

func (s *Server) servePublicFile(w http.ResponseWriter, r *http.Request) {
	extendTransferDeadlines(w)
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
	contentType := ""
	switch strings.ToLower(filepath.Ext(matches[0])) {
	case ".jpg":
		contentType = media.TypeJPEG
	case ".png":
		contentType = media.TypePNG
	case ".webp":
		contentType = media.TypeWebP
	case ".mp4", ".mov", ".webm":
		// Files stored before ftyp brands were checked may be, for example,
		// HEIC photos saved as .mp4, so the type comes from the content.
		contentType = storedVideoContentType(matches[0])
	default:
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Type", contentType)
	if contentType == "application/octet-stream" {
		w.Header().Set("Content-Disposition", "attachment")
	}
	http.ServeFile(w, r, matches[0])
}

func storedVideoContentType(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return "application/octet-stream"
	}
	defer file.Close()
	prefix := make([]byte, 16)
	n, _ := io.ReadFull(file, prefix)
	if contentType, _ := media.VideoType(prefix[:n]); contentType != "" {
		return contentType
	}
	return "application/octet-stream"
}

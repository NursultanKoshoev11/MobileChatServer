package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/realtime"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/service"
	"github.com/NursultanKoshoev11/MobileChatServer/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/websocket"
)

const maxJSONBodyBytes = 1 << 20

type userContextKey struct{}

type Server struct {
	svc            *service.Service
	phoneAuth      *service.PhoneAuthService
	logger         *log.Logger
	allowedOrigins map[string]bool
	hub            *realtime.Hub
	limiter        *RateLimiter
	upgrader       websocket.Upgrader
}

func New(svc *service.Service, phoneAuth *service.PhoneAuthService, logger *log.Logger, allowedOrigins string) http.Handler {
	server := &Server{
		svc:            svc,
		phoneAuth:      phoneAuth,
		logger:         logger,
		allowedOrigins: parseOrigins(allowedOrigins),
		hub:            realtime.NewHub(logger),
		limiter:        NewRateLimiter(120, time.Minute),
	}
	server.upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			return origin == "" || server.allowedOrigins["*"] || server.allowedOrigins[origin]
		},
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(server.recoverer)
	r.Use(server.requestLogger)
	r.Use(server.cors)
	r.Use(server.rateLimit)

	r.Get("/api/health", server.health)

	r.Route("/api/auth", func(r chi.Router) {
		r.Post("/request-code", server.requestPhoneCode)
		r.Post("/verify-code", server.verifyPhoneCode)
		r.Post("/refresh", server.refreshPhoneSession)
	})

	r.Group(func(r chi.Router) {
		r.Use(server.auth)
		r.Get("/api/me", server.me)
		r.Get("/api/groups", server.listGroups)
		r.Post("/api/groups", server.createGroup)
		r.Get("/api/groups/search", server.searchGroups)
		r.Post("/api/groups/join-by-code", server.joinByCode)
		r.Post("/api/groups/{groupID}/join", server.joinPublicGroup)
		r.Post("/api/groups/{groupID}/invite-user", server.inviteUser)
		r.Get("/api/groups/{groupID}/messages", server.listMessages)
		r.Post("/api/groups/{groupID}/messages", server.sendMessage)
		r.Get("/api/groups/{groupID}/ws", server.groupWebSocket)
		r.Get("/api/invites", server.listInvites)
		r.Post("/api/invites/{inviteID}/accept", server.acceptInvite)
		r.Post("/api/invites/{inviteID}/decline", server.declineInvite)
	})

	return r
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) requestPhoneCode(w http.ResponseWriter, r *http.Request) {
	var input service.RequestPhoneCodeInput
	if !readJSON(w, r, &input) {
		return
	}
	result, err := s.phoneAuth.RequestCode(r.Context(), input)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) verifyPhoneCode(w http.ResponseWriter, r *http.Request) {
	var input service.VerifyPhoneCodeInput
	if !readJSON(w, r, &input) {
		return
	}
	session, err := s.phoneAuth.VerifyCode(r.Context(), input)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (s *Server) refreshPhoneSession(w http.ResponseWriter, r *http.Request) {
	var input service.RefreshInput
	if !readJSON(w, r, &input) {
		return
	}
	session, err := s.phoneAuth.Refresh(r.Context(), input)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, currentUser(r))
}

func (s *Server) listGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := s.svc.ListUserGroups(r.Context(), currentUser(r).ID)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, groups)
}

func (s *Server) searchGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := s.svc.SearchPublicGroups(r.Context(), r.URL.Query().Get("q"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, groups)
}

func (s *Server) createGroup(w http.ResponseWriter, r *http.Request) {
	var input service.CreateGroupInput
	if !readJSON(w, r, &input) {
		return
	}
	group, err := s.svc.CreateGroup(r.Context(), currentUser(r).ID, input)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, group)
}

func (s *Server) joinPublicGroup(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.JoinPublicGroup(r.Context(), currentUser(r).ID, chi.URLParam(r, "groupID")); err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "joined"})
}

func (s *Server) joinByCode(w http.ResponseWriter, r *http.Request) {
	var input struct {
		InviteCode string `json:"invite_code"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	group, err := s.svc.JoinByInviteCode(r.Context(), currentUser(r).ID, input.InviteCode)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, group)
}

func (s *Server) inviteUser(w http.ResponseWriter, r *http.Request) {
	var input struct {
		TargetUserID string `json:"target_user_id"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	invite, err := s.svc.CreateInviteRequest(r.Context(), currentUser(r).ID, chi.URLParam(r, "groupID"), input.TargetUserID)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, invite)
}

func (s *Server) listInvites(w http.ResponseWriter, r *http.Request) {
	invites, err := s.svc.ListPendingInvites(r.Context(), currentUser(r).ID)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, invites)
}

func (s *Server) acceptInvite(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.AcceptInviteRequest(r.Context(), currentUser(r).ID, chi.URLParam(r, "inviteID")); err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "accepted"})
}

func (s *Server) declineInvite(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.DeclineInviteRequest(r.Context(), currentUser(r).ID, chi.URLParam(r, "inviteID")); err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "declined"})
}

func (s *Server) listMessages(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err == nil {
			limit = parsed
		}
	}
	var before time.Time
	if raw := r.URL.Query().Get("before"); raw != "" {
		parsed, err := time.Parse(time.RFC3339Nano, raw)
		if err == nil {
			before = parsed
		}
	}
	messages, err := s.svc.ListMessages(r.Context(), currentUser(r).ID, chi.URLParam(r, "groupID"), limit, before)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, messages)
}

func (s *Server) sendMessage(w http.ResponseWriter, r *http.Request) {
	var input service.SendMessageInput
	if !readJSON(w, r, &input) {
		return
	}
	groupID := chi.URLParam(r, "groupID")
	message, err := s.svc.SendMessage(r.Context(), currentUser(r).ID, groupID, input)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.hub.BroadcastGroup(groupID, realtime.Event{Type: "message.created", GroupID: groupID, Payload: message})
	writeJSON(w, http.StatusCreated, message)
}

func (s *Server) groupWebSocket(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	groupID := chi.URLParam(r, "groupID")
	if _, err := s.svc.ListMessages(r.Context(), user.ID, groupID, 1, time.Time{}); err != nil {
		s.writeError(w, err)
		return
	}
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Printf("websocket upgrade failed: %v", err)
		return
	}
	client := realtime.NewClient(s.hub, conn, user, groupID)
	s.hub.Register(client)
	client.Send <- realtime.Event{Type: "connection.ready", GroupID: groupID, Payload: map[string]string{"user_id": user.ID}}
	go client.WritePump()
	go client.ReadPump()
}

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerTokenFromRequest(r)
		if token == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing bearer token"})
			return
		}
		user, err := s.svc.Authenticate(r.Context(), token)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		ctx := context.WithValue(r.Context(), userContextKey{}, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func bearerTokenFromRequest(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if strings.HasPrefix(header, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	}
	if token := strings.TrimSpace(r.URL.Query().Get("token")); token != "" && strings.HasSuffix(r.URL.Path, "/ws") {
		return token
	}
	return ""
}

func (s *Server) rateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := clientIP(r)
		if user := currentUser(r); user.ID != "" {
			key = user.ID
		}
		if !s.limiter.Allow(key) {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && (s.allowedOrigins["*"] || s.allowedOrigins[origin]) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
		w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		s.logger.Printf(
			"request_id=%s method=%s path=%s status=%d bytes=%d duration_ms=%d remote=%s",
			middleware.GetReqID(r.Context()),
			r.Method,
			r.URL.Path,
			recorder.status,
			recorder.bytes,
			time.Since(started).Milliseconds(),
			r.RemoteAddr,
		)
	})
}

func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				s.logger.Printf("panic request_id=%s method=%s path=%s error=%v", middleware.GetReqID(r.Context()), r.Method, r.URL.Path, recovered)
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) writeError(w http.ResponseWriter, err error) {
	var validationErr service.ValidationError
	switch {
	case errors.As(err, &validationErr):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	case errors.Is(err, service.ErrUnauthorized):
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	case errors.Is(err, service.ErrInvalidCredentials):
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid or expired verification code"})
	case errors.Is(err, storage.ErrForbidden):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
	case errors.Is(err, storage.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	default:
		s.logger.Printf("internal error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
}

func readJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	defer r.Body.Close()
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "request body must contain only one JSON object"})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func currentUser(r *http.Request) domain.User {
	user, _ := r.Context().Value(userContextKey{}).(domain.User)
	return user
}

func parseOrigins(raw string) map[string]bool {
	result := map[string]bool{}
	for _, item := range strings.Split(raw, ",") {
		origin := strings.TrimSpace(item)
		if origin != "" {
			result[origin] = true
		}
	}
	return result
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(data []byte) (int, error) {
	written, err := r.ResponseWriter.Write(data)
	r.bytes += written
	return written, err
}

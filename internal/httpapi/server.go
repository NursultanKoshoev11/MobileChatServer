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

const maxJSONBodyBytes = 12 << 20

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
		r.Post("/api/push/register", server.registerPushToken)
		r.Delete("/api/push/token", server.deletePushToken)
		r.Get("/api/groups", server.listGroups)
		r.Post("/api/groups", server.createGroup)
		r.Get("/api/groups/search", server.searchGroups)
		r.Post("/api/groups/join-by-code", server.joinByCode)
		r.Post("/api/groups/{groupID}/join", server.joinPublicGroup)
		r.Post("/api/groups/{groupID}/invite-user", server.inviteUser)
		r.Get("/api/groups/{groupID}/messages", server.listMessages)
		r.Post("/api/groups/{groupID}/messages", server.sendMessage)
		r.Get("/api/groups/{groupID}/ws", server.groupWebSocket)
		r.Post("/api/groups/{groupID}/requests", server.createPublicRequest)
		r.Get("/api/groups/{groupID}/requests", server.listPublicRequests)
		r.Get("/api/groups/{groupID}/statistics", server.groupStatistics)
		r.Get("/api/invites", server.listInvites)
		r.Post("/api/invites/{inviteID}/accept", server.acceptInvite)
		r.Post("/api/invites/{inviteID}/decline", server.declineInvite)
		r.Post("/api/requests/{requestID}/support", server.supportPublicRequest)
		r.Post("/api/requests/{requestID}/oppose", server.opposePublicRequest)
		r.Delete("/api/requests/{requestID}/vote", server.clearPublicRequestVote)
		r.Get("/api/requests/{requestID}/comments", server.listPublicRequestComments)
		r.Post("/api/requests/{requestID}/comments", server.createPublicRequestComment)
		r.Delete("/api/requests/comments/{commentID}", server.deletePublicRequestComment)
		r.Post("/api/requests/{requestID}/status", server.updatePublicRequestStatus)
		r.Post("/api/requests/{requestID}/hide", server.hidePublicRequest)
		r.Post("/api/group-creation-requests", server.createGroupCreationRequest)
		r.Get("/api/group-creation-requests", server.listMyGroupCreationRequests)
		r.Get("/api/admin/group-creation-requests", server.listGroupCreationRequestsForAdmin)
		r.Post("/api/admin/group-creation-requests/{requestID}/approve", server.approveGroupCreationRequest)
		r.Post("/api/admin/group-creation-requests/{requestID}/reject", server.rejectGroupCreationRequest)
		r.Post("/api/admin/group-creation-requests/{requestID}/need-more-info", server.needMoreInfoForGroupCreationRequest)
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
	user := currentUser(r)
	if user.Role != domain.UserRolePlatformAdmin && user.Role != domain.UserRoleSuperAdmin {
		s.writeError(w, storage.ErrForbidden)
		return
	}
	var input service.CreateGroupInput
	if !readJSON(w, r, &input) {
		return
	}
	group, err := s.svc.CreateGroup(r.Context(), user.ID, input)
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
		Mobile       string `json:"mobile"`
		Phone        string `json:"phone"`
		TargetMobile string `json:"target_mobile"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	target := strings.TrimSpace(input.TargetUserID)
	mobile := firstNonEmpty(input.Mobile, input.Phone, input.TargetMobile)
	if mobile == "" && strings.HasPrefix(target, "+") {
		mobile = target
		target = ""
	}
	if target == "" && mobile != "" {
		user, err := s.svc.FindUserByPhone(r.Context(), mobile)
		if err != nil {
			s.writeError(w, err)
			return
		}
		target = user.ID
	}
	invite, err := s.svc.CreateInviteRequest(r.Context(), currentUser(r).ID, chi.URLParam(r, "groupID"), target)
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
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	var before time.Time
	if raw := r.URL.Query().Get("before"); raw != "" {
		before, _ = time.Parse(time.RFC3339, raw)
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
	message, err := s.svc.SendMessage(r.Context(), currentUser(r).ID, chi.URLParam(r, "groupID"), input)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.hub.Broadcast(chi.URLParam(r, "groupID"), message)
	writeJSON(w, http.StatusCreated, message)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func readJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(dst); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return false
	}
	return true
}

func currentUser(r *http.Request) domain.User {
	user, _ := r.Context().Value(userContextKey{}).(domain.User)
	return user
}

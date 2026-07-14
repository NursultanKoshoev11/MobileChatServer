package httpapi

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
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

const maxJSONBodyBytes = 16 * 1024 * 1024

type userContextKey struct{}

type Server struct {
	svc            *service.Service
	phoneAuth      *service.PhoneAuthService
	logger         *log.Logger
	allowedOrigins map[string]bool
	hub            *realtime.Hub
	limiter        *RateLimiter
	strictLimiter  *RateLimiter
	trustedProxies []net.IPNet
	router         http.Handler
	upgrader       websocket.Upgrader
}

func New(svc *service.Service, phoneAuth *service.PhoneAuthService, logger *log.Logger, allowedOrigins string, trustedProxyCIDRs ...string) http.Handler {
	server := &Server{
		svc:            svc,
		phoneAuth:      phoneAuth,
		logger:         logger,
		allowedOrigins: parseOrigins(allowedOrigins),
		hub: realtime.NewHub(logger, func(ctx context.Context, token string) (string, error) {
			user, err := svc.AuthenticateWebSocket(ctx, token)
			if err != nil {
				return "", err
			}
			return user.ID, nil
		}),
		limiter:        NewRateLimiter(600, time.Minute),
		strictLimiter:  NewRateLimiter(20, time.Minute),
		trustedProxies: parseTrustedProxyCIDRs(strings.Join(trustedProxyCIDRs, ",")),
	}
	server.upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		Subprotocols:    []string{"koom-ws"},
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			return origin == "" || server.allowedOrigins["*"] || server.allowedOrigins[origin]
		},
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(server.recoverer)
	r.Use(server.requestLogger)
	r.Use(server.cors)
	r.Use(server.rateLimit)

	r.Get("/api/health", server.health)
	r.Get("/privacy", server.privacyPolicy)
	r.Head("/privacy", server.privacyPolicy)
	r.Get("/child-safety", server.childSafetyStandards)
	r.Head("/child-safety", server.childSafetyStandards)
	r.Get("/api/health/ws", server.websocketHealth)

	r.Route("/api/auth", func(r chi.Router) {
		r.Post("/request-code", server.requestPhoneCode)
		r.Post("/verify-code", server.verifyPhoneCode)
		r.Post("/refresh", server.refreshPhoneSession)
		r.Post("/logout", server.logoutPhoneSession)
	})

	r.Group(func(r chi.Router) {
		r.Use(server.auth)
		r.Get("/api/me", server.me)
		r.Put("/api/me/avatar", server.updateMyAvatar)
		r.Post("/api/ws-token", server.issueWebSocketToken)
		r.Get("/api/ws", server.userWebSocket)
		r.Post("/api/push/register", server.registerPushToken)
		r.Delete("/api/push/token", server.deletePushToken)
		r.Get("/api/groups", server.listGroups)
		r.Post("/api/groups", server.createGroup)
		r.Get("/api/groups/search", server.searchGroups)
		r.Get("/api/users/search", server.searchUsers)
		r.Post("/api/groups/join-by-code", server.joinByCode)
		r.Post("/api/groups/{groupID}/join", server.joinPublicGroup)
		r.Post("/api/groups/{groupID}/invite-code", server.ensureGroupInviteCode)
		r.Get("/api/groups/{groupID}/members", server.listGroupMembers)
		r.Post("/api/groups/{groupID}/members/{userID}/role", server.updateGroupMemberRole)
		r.Post("/api/groups/{groupID}/members/role-by-phone", server.updateGroupMemberRoleByPhone)
		r.Post("/api/groups/{groupID}/comment-mutes/by-phone", server.setGroupCommentMuteByPhone)
		r.Post("/api/groups/{groupID}/comment-mutes/unmute-by-phone", server.clearGroupCommentMuteByPhone)
		r.Delete("/api/groups/{groupID}/comment-mutes/by-phone", server.clearGroupCommentMuteByPhone)
		r.Post("/api/groups/{groupID}/comment-mutes/{userID}", server.setGroupCommentMute)
		r.Post("/api/groups/{groupID}/comment-mutes/{userID}/clear", server.clearGroupCommentMute)
		r.Delete("/api/groups/{groupID}/leave", server.leaveGroup)
		r.Post("/api/groups/{groupID}/invite-user", server.inviteUser)
		r.Get("/api/groups/{groupID}/messages", server.listMessages)
		r.Post("/api/groups/{groupID}/messages", server.sendMessage)
		r.Patch("/api/groups/{groupID}/messages/{messageID}", server.updateMessage)
		r.Delete("/api/groups/{groupID}/messages/{messageID}", server.deleteMessage)
		r.Get("/api/groups/{groupID}/ws", server.groupWebSocket)
		r.Post("/api/groups/{groupID}/files", server.uploadPublicFile)
		r.Get("/api/groups/{groupID}/files/{fileID}", server.servePublicFile)
		r.Get("/api/public-files/{groupID}/{fileID}", server.servePublicFile)
		r.Post("/api/groups/{groupID}/requests", server.createPublicRequest)
		r.Get("/api/groups/{groupID}/requests", server.listPublicRequests)
		r.Post("/api/groups/{groupID}/requests/read", server.markPublicRequestsRead)
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
		r.Get("/api/groups/{groupID}/moderation/items/count", server.countContentModerationItems)
		r.Get("/api/groups/{groupID}/moderation/items", server.listContentModerationItems)
		r.Post("/api/moderation/items/{itemID}/approve", server.approveContentModerationItem)
		r.Post("/api/moderation/items/{itemID}/reject", server.rejectContentModerationItem)
		r.Post("/api/group-creation-requests", server.createGroupCreationRequest)
		r.Get("/api/group-creation-requests", server.listMyGroupCreationRequests)
		r.Get("/api/admin/group-creation-requests", server.listGroupCreationRequestsForAdmin)
		r.Post("/api/admin/group-creation-requests/{requestID}/approve", server.approveGroupCreationRequest)
		r.Post("/api/admin/group-creation-requests/{requestID}/reject", server.rejectGroupCreationRequest)
		r.Post("/api/admin/group-creation-requests/{requestID}/need-more-info", server.needMoreInfoForGroupCreationRequest)
	})

	server.router = r
	return server
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.svc.HealthCheck(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "error", "database": "unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "database": "ok"})
}

func (s *Server) websocketHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "clients": s.hub.ClientCount()})
}

func (s *Server) DrainWebSockets(timeout time.Duration) {
	s.hub.Drain(timeout)
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

func (s *Server) logoutPhoneSession(w http.ResponseWriter, r *http.Request) {
	var input service.RefreshInput
	if !readJSON(w, r, &input) {
		return
	}
	if err := s.phoneAuth.Logout(r.Context(), input); err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	user, err := s.svc.GetUserProfile(r.Context(), currentUser(r).ID)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) issueWebSocketToken(w http.ResponseWriter, r *http.Request) {
	token, err := s.svc.IssueWebSocketToken(currentUser(r).ID)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
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

func (s *Server) ensureGroupInviteCode(w http.ResponseWriter, r *http.Request) {
	group, err := s.svc.EnsureGroupInviteCode(r.Context(), currentUser(r).ID, chi.URLParam(r, "groupID"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, group)
}

func (s *Server) listGroupMembers(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	offset := 0
	if raw := r.URL.Query().Get("offset"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			offset = parsed
		}
	}
	members, err := s.svc.ListGroupMembersPage(r.Context(), currentUser(r).ID, chi.URLParam(r, "groupID"), limit, offset)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, members)
}

func (s *Server) updateGroupMemberRole(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Role domain.GroupRole `json:"role"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	member, err := s.svc.UpdateGroupMemberRole(r.Context(), currentUser(r).ID, chi.URLParam(r, "groupID"), chi.URLParam(r, "userID"), input.Role)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, member)
}

func (s *Server) updateGroupMemberRoleByPhone(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Phone  string           `json:"phone"`
		Mobile string           `json:"mobile"`
		Role   domain.GroupRole `json:"role"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	phone := input.Phone
	if phone == "" {
		phone = input.Mobile
	}
	member, err := s.svc.UpdateGroupMemberRoleByPhone(r.Context(), currentUser(r).ID, chi.URLParam(r, "groupID"), phone, input.Role)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, member)
}

func (s *Server) leaveGroup(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.LeaveGroup(r.Context(), currentUser(r).ID, chi.URLParam(r, "groupID")); err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "left"})
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
	s.hub.NotifyUser(target, realtime.Event{Type: "invite.created", GroupID: invite.GroupID, Payload: invite})
	go s.svc.NotifyUserAboutInvite(r.Context(), target, invite)
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
	s.hub.NotifyUser(currentUser(r).ID, realtime.Event{Type: "invite.reviewed", Payload: map[string]string{"invite_id": chi.URLParam(r, "inviteID"), "status": "accepted"}})
	writeJSON(w, http.StatusOK, map[string]string{"status": "accepted"})
}

func (s *Server) declineInvite(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.DeclineInviteRequest(r.Context(), currentUser(r).ID, chi.URLParam(r, "inviteID")); err != nil {
		s.writeError(w, err)
		return
	}
	s.hub.NotifyUser(currentUser(r).ID, realtime.Event{Type: "invite.reviewed", Payload: map[string]string{"invite_id": chi.URLParam(r, "inviteID"), "status": "declined"}})
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
		if parsed, err := time.Parse(time.RFC3339Nano, raw); err == nil {
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
	s.broadcastGroupAndUsers(r, groupID, realtime.Event{Type: "message.created", GroupID: groupID, Payload: message})
	writeJSON(w, http.StatusCreated, message)
}

func (s *Server) broadcastGroupAndUsers(r *http.Request, groupID string, event realtime.Event) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return
	}
	if event.GroupID == "" {
		event.GroupID = groupID
	}
	s.hub.BroadcastGroup(groupID, event)
	actorID := currentUser(r).ID
	memberIDs, err := s.svc.ListGroupMemberIDsExcept(r.Context(), groupID, actorID)
	if err != nil {
		s.logger.Printf("realtime user broadcast skipped group_id=%s type=%s error=%v", groupID, event.Type, err)
		return
	}
	for _, memberID := range memberIDs {
		s.hub.NotifyUser(memberID, event)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func (s *Server) groupWebSocket(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	groupID := chi.URLParam(r, "groupID")
	remoteIP := s.clientIP(r)
	if !s.strictLimiter.Allow("ws:" + remoteIP) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many websocket connection attempts"})
		return
	}
	if _, err := s.svc.ListMessages(r.Context(), user.ID, groupID, 1, time.Time{}); err != nil {
		s.writeError(w, err)
		return
	}
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Printf("websocket upgrade failed: %v", err)
		return
	}
	client := realtime.NewClient(s.hub, conn, user, groupID, remoteIP)
	if !s.hub.Register(client) {
		_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseTryAgainLater, "connection limit reached"))
		_ = conn.Close()
		return
	}
	client.Send <- realtime.Event{Type: "connection.ready", GroupID: groupID, Payload: map[string]string{"user_id": user.ID}}
	go client.WritePump()
	go client.ReadPump()
}

func (s *Server) userWebSocket(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	remoteIP := s.clientIP(r)
	if !s.strictLimiter.Allow("ws:" + remoteIP) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many websocket connection attempts"})
		return
	}
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Printf("user websocket upgrade failed: %v", err)
		return
	}
	client := realtime.NewClient(s.hub, conn, user, "", remoteIP)
	if !s.hub.Register(client) {
		_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseTryAgainLater, "connection limit reached"))
		_ = conn.Close()
		return
	}
	client.Send <- realtime.Event{Type: "connection.ready", Payload: map[string]string{"user_id": user.ID}}
	go client.WritePump()
	go client.ReadPump()
}

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, fromQuery := bearerTokenFromRequest(r)
		if token == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing bearer token"})
			return
		}

		var user domain.User
		var err error
		if fromQuery && strings.HasSuffix(r.URL.Path, "/ws") {
			user, err = s.svc.AuthenticateWebSocket(r.Context(), token)
		} else {
			user, err = s.svc.Authenticate(r.Context(), token)
		}
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		ctx := context.WithValue(r.Context(), userContextKey{}, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func bearerTokenFromRequest(r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	if strings.HasPrefix(header, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")), false
	}
	if token := bearerTokenFromWebSocketProtocol(r); token != "" && strings.HasSuffix(r.URL.Path, "/ws") {
		return token, true
	}
	if token := strings.TrimSpace(r.URL.Query().Get("token")); token != "" && strings.HasSuffix(r.URL.Path, "/ws") {
		return token, true
	}
	return "", false
}

func bearerTokenFromWebSocketProtocol(r *http.Request) string {
	for _, item := range strings.Split(r.Header.Get("Sec-WebSocket-Protocol"), ",") {
		item = strings.TrimSpace(item)
		if strings.Count(item, ".") == 2 {
			return item
		}
	}
	return ""
}

func (s *Server) rateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		key := s.clientIP(r)
		if user := currentUser(r); user.ID != "" {
			key = user.ID
		}
		limiter := s.limiter
		if isStrictRateLimitedPath(r.URL.Path) {
			limiter = s.strictLimiter
			key = r.URL.Path + ":" + key
		}
		if !limiter.Allow(key) {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isStrictRateLimitedPath(path string) bool {
	return strings.HasPrefix(path, "/api/auth/request-code") ||
		strings.HasPrefix(path, "/api/auth/verify-code") ||
		strings.HasPrefix(path, "/api/groups/") && strings.HasSuffix(path, "/messages")
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && (s.allowedOrigins["*"] || s.allowedOrigins[origin]) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
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
		s.logger.Printf("request_id=%s method=%s path=%s status=%d bytes=%d duration_ms=%d remote=%s", middleware.GetReqID(r.Context()), r.Method, r.URL.Path, recorder.status, recorder.bytes, time.Since(started).Milliseconds(), s.clientIP(r))
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

func currentUser(r *http.Request) domain.User {
	user, _ := r.Context().Value(userContextKey{}).(domain.User)
	return user
}

func readJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	defer r.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxJSONBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(payload)
}

func (s *Server) writeError(w http.ResponseWriter, err error) {
	if err == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	var validation service.ValidationError
	var unavailable service.ServiceUnavailableError
	var pending service.ContentModerationPendingError
	switch {
	case errors.As(err, &pending):
		if pending.Item.GroupID != "" {
			s.hub.BroadcastGroup(pending.Item.GroupID, realtime.Event{Type: "content_moderation.pending_review", GroupID: pending.Item.GroupID, Payload: pending.Item})
		}
		writeJSON(w, http.StatusAccepted, map[string]any{"status": "pending_review", "moderation_item": pending.Item})
	case errors.As(err, &validation):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": validation.Error()})
	case errors.As(err, &unavailable):
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": unavailable.Error()})
	case errors.Is(err, service.ErrUnauthorized):
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	case errors.Is(err, service.ErrInvalidCredentials):
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
	case errors.Is(err, storage.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	case errors.Is(err, storage.ErrForbidden):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
	default:
		s.logger.Printf("http error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
}

func parseOrigins(raw string) map[string]bool {
	result := map[string]bool{}
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			result[item] = true
		}
	}
	if len(result) == 0 {
		result["*"] = true
	}
	return result
}

func parseTrustedProxyCIDRs(raw string) []net.IPNet {
	result := make([]net.IPNet, 0)
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if ip := net.ParseIP(item); ip != nil {
			mask := net.CIDRMask(32, 32)
			if ip.To4() == nil {
				mask = net.CIDRMask(128, 128)
			}
			result = append(result, net.IPNet{IP: ip, Mask: mask})
			continue
		}
		if _, network, err := net.ParseCIDR(item); err == nil {
			result = append(result, *network)
		}
	}
	return result
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

func (r *statusRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response writer does not support hijacking")
	}
	return hijacker.Hijack()
}

func (r *statusRecorder) Push(target string, opts *http.PushOptions) error {
	pusher, ok := r.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, opts)
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(data []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(data)
	r.bytes += n
	return n, err
}

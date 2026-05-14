package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	maxJSONBodyBytes  = 1 << 20
	maxDisplayNameLen = 40
	maxGroupTitleLen  = 80
	maxDescriptionLen = 240
	maxMessageLen     = 2000
)

type User struct {
	ID          string    `json:"id"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
}

type GroupVisibility string

const (
	VisibilityPublic  GroupVisibility = "public"
	VisibilityPrivate GroupVisibility = "private"
)

type Group struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Visibility  GroupVisibility `json:"visibility"`
	OwnerID     string          `json:"owner_id"`
	InviteCode  string          `json:"invite_code,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	MemberCount int             `json:"member_count"`

	Members map[string]bool `json:"-"`
	Admins  map[string]bool `json:"-"`
}

type Message struct {
	ID         string    `json:"id"`
	GroupID    string    `json:"group_id"`
	SenderID   string    `json:"sender_id"`
	SenderName string    `json:"sender_name"`
	Text       string    `json:"text"`
	CreatedAt  time.Time `json:"created_at"`
}

type Store struct {
	mu       sync.RWMutex
	users    map[string]*User
	groups   map[string]*Group
	messages map[string][]*Message
}

func NewStore() *Store {
	store := &Store{
		users:    make(map[string]*User),
		groups:   make(map[string]*Group),
		messages: make(map[string][]*Message),
	}
	store.seedDemoData()
	return store
}

func (s *Store) seedDemoData() {
	demoUser := &User{ID: "U-DEMO01", DisplayName: "MobileChat", CreatedAt: time.Now().UTC()}
	s.users[demoUser.ID] = demoUser

	group := &Group{
		ID:          "G-WELCOME",
		Title:       "Welcome Group",
		Description: "Public demo group for the first app launch.",
		Visibility:  VisibilityPublic,
		OwnerID:     demoUser.ID,
		CreatedAt:   time.Now().UTC(),
		Members:     map[string]bool{demoUser.ID: true},
		Admins:      map[string]bool{demoUser.ID: true},
	}
	group.MemberCount = len(group.Members)
	s.groups[group.ID] = group
	s.messages[group.ID] = []*Message{
		{
			ID:         "M-WELCOME-1",
			GroupID:    group.ID,
			SenderID:   demoUser.ID,
			SenderName: demoUser.DisplayName,
			Text:       "Welcome to MobileChat. This app supports group chats only.",
			CreatedAt:  time.Now().UTC(),
		},
	}
}

func (s *Store) Login(displayName string) (*User, error) {
	displayName = strings.TrimSpace(displayName)
	if len(displayName) < 2 {
		return nil, errors.New("display_name must contain at least 2 characters")
	}
	if len(displayName) > maxDisplayNameLen {
		return nil, fmt.Errorf("display_name must be at most %d characters", maxDisplayNameLen)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	user := &User{
		ID:          "U-" + strings.ToUpper(randomHex(4)),
		DisplayName: displayName,
		CreatedAt:   time.Now().UTC(),
	}
	s.users[user.ID] = user
	return user, nil
}

func (s *Store) ListUserGroups(userID string) []*Group {
	s.mu.RLock()
	defer s.mu.RUnlock()

	groups := make([]*Group, 0)
	for _, group := range s.groups {
		if group.Members[userID] {
			groups = append(groups, cloneGroup(group))
		}
	}
	sortGroups(groups)
	return groups
}

func (s *Store) SearchPublicGroups(query string) []*Group {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query = strings.ToLower(strings.TrimSpace(query))
	groups := make([]*Group, 0)
	for _, group := range s.groups {
		if group.Visibility != VisibilityPublic {
			continue
		}
		if query == "" || strings.Contains(strings.ToLower(group.Title), query) || strings.Contains(strings.ToLower(group.Description), query) {
			groups = append(groups, cloneGroup(group))
		}
	}
	sortGroups(groups)
	return groups
}

func (s *Store) CreateGroup(ownerID, title, description string, visibility GroupVisibility) (*Group, error) {
	ownerID = strings.TrimSpace(ownerID)
	title = strings.TrimSpace(title)
	description = strings.TrimSpace(description)
	if ownerID == "" {
		return nil, errors.New("owner_id is required")
	}
	if len(title) < 3 {
		return nil, errors.New("title must contain at least 3 characters")
	}
	if len(title) > maxGroupTitleLen {
		return nil, fmt.Errorf("title must be at most %d characters", maxGroupTitleLen)
	}
	if len(description) > maxDescriptionLen {
		return nil, fmt.Errorf("description must be at most %d characters", maxDescriptionLen)
	}
	if visibility != VisibilityPublic && visibility != VisibilityPrivate {
		return nil, errors.New("visibility must be public or private")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.users[ownerID]; !ok {
		return nil, errors.New("owner user was not found")
	}

	group := &Group{
		ID:          "G-" + strings.ToUpper(randomHex(4)),
		Title:       title,
		Description: description,
		Visibility:  visibility,
		OwnerID:     ownerID,
		CreatedAt:   time.Now().UTC(),
		Members:     map[string]bool{ownerID: true},
		Admins:      map[string]bool{ownerID: true},
	}
	if visibility == VisibilityPrivate {
		group.InviteCode = strings.ToUpper(randomHex(3))
	}
	group.MemberCount = len(group.Members)
	s.groups[group.ID] = group
	s.messages[group.ID] = []*Message{}
	return cloneGroup(group), nil
}

func (s *Store) JoinPublicGroup(groupID, userID string) error {
	groupID = strings.TrimSpace(groupID)
	userID = strings.TrimSpace(userID)
	if groupID == "" || userID == "" {
		return errors.New("group_id and user_id are required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	group, ok := s.groups[groupID]
	if !ok {
		return errors.New("group was not found")
	}
	if group.Visibility != VisibilityPublic {
		return errors.New("private group requires invite code or admin invitation")
	}
	if _, ok := s.users[userID]; !ok {
		return errors.New("user was not found")
	}
	group.Members[userID] = true
	group.MemberCount = len(group.Members)
	return nil
}

func (s *Store) JoinByInviteCode(userID, inviteCode string) (*Group, error) {
	userID = strings.TrimSpace(userID)
	inviteCode = strings.ToUpper(strings.TrimSpace(inviteCode))
	if userID == "" || inviteCode == "" {
		return nil, errors.New("user_id and invite_code are required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.users[userID]; !ok {
		return nil, errors.New("user was not found")
	}
	for _, group := range s.groups {
		if group.Visibility == VisibilityPrivate && group.InviteCode == inviteCode {
			group.Members[userID] = true
			group.MemberCount = len(group.Members)
			return cloneGroup(group), nil
		}
	}
	return nil, errors.New("invite code was not found")
}

func (s *Store) InviteUserByID(groupID, adminID, targetUserID string) error {
	groupID = strings.TrimSpace(groupID)
	adminID = strings.TrimSpace(adminID)
	targetUserID = strings.TrimSpace(targetUserID)
	if groupID == "" || adminID == "" || targetUserID == "" {
		return errors.New("group_id, admin_id, and target_user_id are required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	group, ok := s.groups[groupID]
	if !ok {
		return errors.New("group was not found")
	}
	if !group.Admins[adminID] {
		return errors.New("only group admin can invite users")
	}
	if _, ok := s.users[targetUserID]; !ok {
		return errors.New("target user was not found")
	}
	group.Members[targetUserID] = true
	group.MemberCount = len(group.Members)
	return nil
}

func (s *Store) ListMessages(groupID string) ([]*Message, error) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil, errors.New("group_id is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, ok := s.groups[groupID]; !ok {
		return nil, errors.New("group was not found")
	}
	messages := s.messages[groupID]
	result := make([]*Message, 0, len(messages))
	for _, message := range messages {
		copied := *message
		result = append(result, &copied)
	}
	return result, nil
}

func (s *Store) SendMessage(groupID, senderID, text string) (*Message, error) {
	groupID = strings.TrimSpace(groupID)
	senderID = strings.TrimSpace(senderID)
	text = strings.TrimSpace(text)
	if groupID == "" || senderID == "" {
		return nil, errors.New("group_id and sender_id are required")
	}
	if text == "" {
		return nil, errors.New("text is required")
	}
	if len(text) > maxMessageLen {
		return nil, fmt.Errorf("text must be at most %d characters", maxMessageLen)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	group, ok := s.groups[groupID]
	if !ok {
		return nil, errors.New("group was not found")
	}
	if !group.Members[senderID] {
		return nil, errors.New("user is not a member of this group")
	}
	user, ok := s.users[senderID]
	if !ok {
		return nil, errors.New("sender was not found")
	}

	message := &Message{
		ID:         "M-" + strings.ToUpper(randomHex(6)),
		GroupID:    groupID,
		SenderID:   senderID,
		SenderName: user.DisplayName,
		Text:       text,
		CreatedAt:  time.Now().UTC(),
	}
	s.messages[groupID] = append(s.messages[groupID], message)
	return message, nil
}

type Server struct {
	store *Store
}

func main() {
	logger := log.New(os.Stdout, "mobilechat-server ", log.LstdFlags|log.LUTC|log.Lmicroseconds)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	app := &Server{store: NewStore()}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", app.handleHealth)
	mux.HandleFunc("/api/auth/login", app.handleLogin)
	mux.HandleFunc("/api/groups", app.handleGroups)
	mux.HandleFunc("/api/groups/search", app.handleSearchGroups)
	mux.HandleFunc("/api/groups/join-by-code", app.handleJoinByCode)
	mux.HandleFunc("/api/groups/", app.handleGroupActions)

	handler := withRecovery(logger, withRequestLogger(logger, withCORS(mux)))
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		logger.Printf("listening on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatalf("server failed: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	logger.Println("shutdown started")
	if err := server.Shutdown(ctx); err != nil {
		logger.Printf("graceful shutdown failed: %v", err)
		if closeErr := server.Close(); closeErr != nil {
			logger.Printf("forced close failed: %v", closeErr)
		}
	}
	logger.Println("shutdown completed")
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var request struct {
		DisplayName string `json:"display_name"`
	}
	if err := readJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	user, err := s.store.Login(request.DisplayName)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) handleGroups(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		userID := r.URL.Query().Get("user_id")
		writeJSON(w, http.StatusOK, s.store.ListUserGroups(userID))
	case http.MethodPost:
		var request struct {
			OwnerID     string          `json:"owner_id"`
			Title       string          `json:"title"`
			Description string          `json:"description"`
			Visibility  GroupVisibility `json:"visibility"`
		}
		if err := readJSON(w, r, &request); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		group, err := s.store.CreateGroup(request.OwnerID, request.Title, request.Description, request.Visibility)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, group)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleSearchGroups(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, http.StatusOK, s.store.SearchPublicGroups(r.URL.Query().Get("q")))
}

func (s *Server) handleJoinByCode(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var request struct {
		UserID     string `json:"user_id"`
		InviteCode string `json:"invite_code"`
	}
	if err := readJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	group, err := s.store.JoinByInviteCode(request.UserID, request.InviteCode)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, group)
}

func (s *Server) handleGroupActions(w http.ResponseWriter, r *http.Request) {
	trimmed := strings.TrimPrefix(r.URL.Path, "/api/groups/")
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) < 2 {
		writeError(w, http.StatusNotFound, "route was not found")
		return
	}

	groupID := parts[0]
	action := parts[1]

	switch action {
	case "join":
		s.handleJoinPublicGroup(w, r, groupID)
	case "invite-user":
		s.handleInviteUserByID(w, r, groupID)
	case "messages":
		s.handleMessages(w, r, groupID)
	default:
		writeError(w, http.StatusNotFound, "route was not found")
	}
}

func (s *Server) handleJoinPublicGroup(w http.ResponseWriter, r *http.Request, groupID string) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var request struct {
		UserID string `json:"user_id"`
	}
	if err := readJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.JoinPublicGroup(groupID, request.UserID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "joined"})
}

func (s *Server) handleInviteUserByID(w http.ResponseWriter, r *http.Request, groupID string) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var request struct {
		AdminID      string `json:"admin_id"`
		TargetUserID string `json:"target_user_id"`
	}
	if err := readJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.InviteUserByID(groupID, request.AdminID, request.TargetUserID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "invited"})
}

func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request, groupID string) {
	switch r.Method {
	case http.MethodGet:
		messages, err := s.store.ListMessages(groupID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, messages)
	case http.MethodPost:
		var request struct {
			SenderID string `json:"sender_id"`
			Text     string `json:"text"`
		}
		if err := readJSON(w, r, &request); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		message, err := s.store.SendMessage(groupID, request.SenderID, request.Text)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, message)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
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

func withRequestLogger(logger *log.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = "REQ-" + strings.ToUpper(randomHex(4))
		}
		w.Header().Set("X-Request-ID", requestID)

		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)

		logger.Printf(
			"request_id=%s method=%s path=%s status=%d bytes=%d duration_ms=%d remote=%s",
			requestID,
			r.Method,
			r.URL.Path,
			recorder.status,
			recorder.bytes,
			time.Since(started).Milliseconds(),
			r.RemoteAddr,
		)
	})
}

func withRecovery(logger *log.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Printf("panic method=%s path=%s error=%v", r.Method, r.URL.Path, recovered)
				writeError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
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

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	return false
}

func readJSON(w http.ResponseWriter, r *http.Request, target any) error {
	defer r.Body.Close()
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("request body must contain only one JSON object")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("failed to write json response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func randomHex(bytesCount int) string {
	buf := make([]byte, bytesCount)
	if _, err := rand.Read(buf); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format("150405.000")))
	}
	return hex.EncodeToString(buf)
}

func cloneGroup(group *Group) *Group {
	copy := *group
	copy.MemberCount = len(group.Members)
	copy.Members = nil
	copy.Admins = nil
	return &copy
}

func sortGroups(groups []*Group) {
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].CreatedAt.After(groups[j].CreatedAt)
	})
}

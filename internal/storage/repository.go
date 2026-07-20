package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserWithPassword struct {
	User         domain.User
	PasswordHash string
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Ping(ctx context.Context) error {
	return r.db.Ping(ctx)
}

func (r *Repository) CreateUser(ctx context.Context, user domain.User, passwordHash string) (domain.User, error) {
	if user.Role == "" {
		user.Role = domain.UserRoleUser
	}
	query := `INSERT INTO users (id, email, phone, display_name, password_hash, role, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, now(), now()) RETURNING created_at`
	if err := r.db.QueryRow(ctx, query, user.ID, strings.ToLower(user.Email), nullableString(user.Phone), user.DisplayName, passwordHash, user.Role).Scan(&user.CreatedAt); err != nil {
		return domain.User{}, fmt.Errorf("create user: %w", err)
	}
	user.Email = strings.ToLower(user.Email)
	return user, nil
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (UserWithPassword, error) {
	query := `SELECT id, COALESCE(email, ''), COALESCE(phone, ''), display_name, COALESCE(password_hash, ''), COALESCE(role, 'user'), COALESCE(avatar_data, ''), created_at FROM users WHERE email = $1`
	var result UserWithPassword
	err := r.db.QueryRow(ctx, query, strings.ToLower(strings.TrimSpace(email))).Scan(
		&result.User.ID,
		&result.User.Email,
		&result.User.Phone,
		&result.User.DisplayName,
		&result.PasswordHash,
		&result.User.Role,
		&result.User.AvatarData,
		&result.User.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return UserWithPassword{}, ErrNotFound
	}
	if err != nil {
		return UserWithPassword{}, fmt.Errorf("get user by email: %w", err)
	}
	return result, nil
}

func (r *Repository) GetUserByPhone(ctx context.Context, phone string) (domain.User, error) {
	phone = normalizeSearchPhone(phone)
	query := `
		SELECT id, COALESCE(email, ''), COALESCE(NULLIF(phone, ''), phone_number, '') AS phone, display_name, COALESCE(role, 'user'), COALESCE(avatar_data, ''), created_at
		FROM users
		WHERE phone = $1 OR phone_number = $1`
	var user domain.User
	err := r.db.QueryRow(ctx, query, phone).Scan(&user.ID, &user.Email, &user.Phone, &user.DisplayName, &user.Role, &user.AvatarData, &user.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, ErrNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("get user by phone: %w", err)
	}
	return user, nil
}

func (r *Repository) GetUserByID(ctx context.Context, userID string) (domain.User, error) {
	query := `SELECT id, COALESCE(email, ''), COALESCE(NULLIF(phone, ''), phone_number, '') AS phone, display_name, COALESCE(role, 'user'), COALESCE(avatar_data, ''), created_at FROM users WHERE id = $1`
	var user domain.User
	err := r.db.QueryRow(ctx, query, userID).Scan(&user.ID, &user.Email, &user.Phone, &user.DisplayName, &user.Role, &user.AvatarData, &user.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, ErrNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("get user by id: %w", err)
	}
	return user, nil
}

func (r *Repository) ListUserGroups(ctx context.Context, userID string) ([]domain.Group, error) {
	if err := r.ensurePublicRequestReadsTable(ctx); err != nil {
		return nil, err
	}
	query := `
		SELECT g.id, g.title, g.description, g.visibility, g.owner_id, COALESCE(g.avatar_data, '') AS avatar_data, COALESCE(g.invite_code, '') AS invite_code, g.created_at,
		       (SELECT COUNT(*)::int FROM group_members gm_all WHERE gm_all.group_id = g.id) AS member_count,
		       gm.role,
		       (
		           SELECT COUNT(*)::int
		           FROM public_requests pr
		           LEFT JOIN public_request_reads prr ON prr.group_id = pr.group_id AND prr.user_id = $1
		           WHERE pr.group_id = g.id
		             AND pr.author_id <> $1
		             AND pr.created_at > COALESCE(prr.last_read_at, 'epoch'::timestamptz)
		       ) AS unread_public_request_count
		FROM groups g
		JOIN group_members gm ON gm.group_id = g.id AND gm.user_id = $1
		ORDER BY g.created_at DESC`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list user groups: %w", err)
	}
	defer rows.Close()

	groups := make([]domain.Group, 0)
	for rows.Next() {
		var group domain.Group
		var role domain.GroupRole
		if err := rows.Scan(&group.ID, &group.Title, &group.Description, &group.Visibility, &group.OwnerID, &group.AvatarData, &group.InviteCode, &group.CreatedAt, &group.MemberCount, &role, &group.UnreadPublicRequestCount); err != nil {
			return nil, fmt.Errorf("scan user group: %w", err)
		}
		group.MyRole = &role
		groups = append(groups, group)
	}
	return groups, rows.Err()
}

func (r *Repository) SearchPublicGroups(ctx context.Context, queryText string) ([]domain.Group, error) {
	queryText = strings.TrimSpace(queryText)
	query := `
		SELECT g.id, g.title, g.description, g.visibility, g.owner_id, COALESCE(g.avatar_data, '') AS avatar_data, COALESCE(g.invite_code, '') AS invite_code, g.created_at,
		       COUNT(gm.user_id)::int AS member_count
		FROM groups g
		LEFT JOIN group_members gm ON gm.group_id = g.id
		WHERE g.visibility = 'public'
		  AND ($1 = '' OR lower(g.title) LIKE '%' || lower($1) || '%' OR lower(g.description) LIKE '%' || lower($1) || '%')
		GROUP BY g.id
		ORDER BY g.created_at DESC
		LIMIT 50`
	rows, err := r.db.Query(ctx, query, queryText)
	if err != nil {
		return nil, fmt.Errorf("search public groups: %w", err)
	}
	defer rows.Close()

	groups := make([]domain.Group, 0)
	for rows.Next() {
		var group domain.Group
		if err := rows.Scan(&group.ID, &group.Title, &group.Description, &group.Visibility, &group.OwnerID, &group.AvatarData, &group.InviteCode, &group.CreatedAt, &group.MemberCount); err != nil {
			return nil, fmt.Errorf("scan public group: %w", err)
		}
		groups = append(groups, group)
	}
	return groups, rows.Err()
}

func (r *Repository) CreateGroup(ctx context.Context, group domain.Group) (domain.Group, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.Group{}, fmt.Errorf("begin create group: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `INSERT INTO groups (id, title, description, visibility, owner_id, invite_code, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, now(), now()) RETURNING created_at`
	if err := tx.QueryRow(ctx, query, group.ID, group.Title, group.Description, group.Visibility, group.OwnerID, nullableInviteCode(group.InviteCode)).Scan(&group.CreatedAt); err != nil {
		return domain.Group{}, fmt.Errorf("insert group: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO group_members (group_id, user_id, role) VALUES ($1, $2, 'owner')`, group.ID, group.OwnerID); err != nil {
		return domain.Group{}, fmt.Errorf("insert group owner: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Group{}, fmt.Errorf("commit create group: %w", err)
	}
	role := domain.RoleOwner
	group.MyRole = &role
	group.MemberCount = 1
	return group, nil
}

func (r *Repository) EnsureGroupInviteCode(ctx context.Context, groupID, userID, generatedCode string) (domain.Group, error) {
	generatedCode = strings.ToUpper(strings.TrimSpace(generatedCode))
	if generatedCode == "" {
		return domain.Group{}, fmt.Errorf("generated invite code is empty")
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.Group{}, fmt.Errorf("begin ensure group invite code: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `
		SELECT g.id, g.title, g.description, g.visibility, g.owner_id, COALESCE(g.avatar_data, '') AS avatar_data, COALESCE(g.invite_code, '') AS invite_code, g.created_at,
		       gm.role,
		       (SELECT COUNT(*)::int FROM group_members WHERE group_id = g.id) AS member_count
		FROM groups g
		JOIN group_members gm ON gm.group_id = g.id AND gm.user_id = $2
		WHERE g.id = $1
		FOR UPDATE OF g`
	var group domain.Group
	var role domain.GroupRole
	if err := tx.QueryRow(ctx, query, groupID, userID).Scan(&group.ID, &group.Title, &group.Description, &group.Visibility, &group.OwnerID, &group.AvatarData, &group.InviteCode, &group.CreatedAt, &role, &group.MemberCount); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Group{}, ErrForbidden
		}
		return domain.Group{}, fmt.Errorf("load group for invite code: %w", err)
	}

	if strings.TrimSpace(group.InviteCode) == "" {
		if err := tx.QueryRow(ctx, `UPDATE groups SET invite_code=$2, updated_at=now() WHERE id=$1 RETURNING invite_code`, group.ID, generatedCode).Scan(&group.InviteCode); err != nil {
			return domain.Group{}, fmt.Errorf("set group invite code: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Group{}, fmt.Errorf("commit ensure group invite code: %w", err)
	}
	group.MyRole = &role
	return group, nil
}

func (r *Repository) JoinPublicGroup(ctx context.Context, groupID, userID string) error {
	var visibility domain.GroupVisibility
	if err := r.db.QueryRow(ctx, `SELECT visibility FROM groups WHERE id = $1`, groupID).Scan(&visibility); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("load group visibility: %w", err)
	}
	if visibility != domain.VisibilityPublic {
		return ErrForbidden
	}
	_, err := r.db.Exec(ctx, `INSERT INTO group_members (group_id, user_id, role) VALUES ($1, $2, 'member') ON CONFLICT (group_id, user_id) DO NOTHING`, groupID, userID)
	if err != nil {
		return fmt.Errorf("join public group: %w", err)
	}
	return nil
}

func (r *Repository) JoinByInviteCode(ctx context.Context, userID, inviteCode string) (domain.Group, error) {
	normalizedCode := normalizeStoredInviteCode(inviteCode)
	query := `
		SELECT id, title, description, visibility, owner_id, COALESCE(avatar_data, '') AS avatar_data, COALESCE(invite_code, '') AS invite_code, created_at
		FROM groups
		WHERE REPLACE(UPPER(COALESCE(invite_code, '')), '-', '') = $1`
	var group domain.Group
	if err := r.db.QueryRow(ctx, query, normalizedCode).Scan(&group.ID, &group.Title, &group.Description, &group.Visibility, &group.OwnerID, &group.AvatarData, &group.InviteCode, &group.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Group{}, ErrNotFound
		}
		return domain.Group{}, fmt.Errorf("find group by invite code: %w", err)
	}
	if _, err := r.db.Exec(ctx, `INSERT INTO group_members (group_id, user_id, role) VALUES ($1, $2, 'member') ON CONFLICT (group_id, user_id) DO NOTHING`, group.ID, userID); err != nil {
		return domain.Group{}, fmt.Errorf("join by invite code: %w", err)
	}
	role := domain.RoleMember
	group.MyRole = &role
	group.MemberCount, _ = r.CountGroupMembers(ctx, group.ID)
	return group, nil
}

func (r *Repository) InviteUserByID(ctx context.Context, groupID, adminID, targetUserID string) error {
	role, err := r.GetMemberRole(ctx, groupID, adminID)
	if err != nil {
		return err
	}
	if role != domain.RoleOwner && role != domain.RoleAdmin {
		return ErrForbidden
	}
	if _, err := r.GetUserByID(ctx, targetUserID); err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, `INSERT INTO group_members (group_id, user_id, role) VALUES ($1, $2, 'member') ON CONFLICT (group_id, user_id) DO NOTHING`, groupID, targetUserID)
	if err != nil {
		return fmt.Errorf("invite user by id: %w", err)
	}
	return nil
}

func (r *Repository) CreateMessage(ctx context.Context, message domain.Message) (domain.Message, error) {
	isMember, err := r.IsGroupMember(ctx, message.GroupID, message.SenderID)
	if err != nil {
		return domain.Message{}, err
	}
	if !isMember {
		return domain.Message{}, ErrForbidden
	}
	query := `
		INSERT INTO messages (id, group_id, sender_id, text, created_at)
		VALUES ($1, $2, $3, $4, now())
		RETURNING created_at`
	if err := r.db.QueryRow(ctx, query, message.ID, message.GroupID, message.SenderID, message.Text).Scan(&message.CreatedAt); err != nil {
		return domain.Message{}, fmt.Errorf("create message: %w", err)
	}
	user, err := r.GetUserByID(ctx, message.SenderID)
	if err != nil {
		return domain.Message{}, err
	}
	message.SenderName = user.DisplayName
	return message, nil
}

func (r *Repository) ListMessages(ctx context.Context, groupID, userID string, limit int, before time.Time) ([]domain.Message, error) {
	isMember, err := r.IsGroupMember(ctx, groupID, userID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, ErrForbidden
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := `
		SELECT m.id, m.group_id, m.sender_id, u.display_name, m.text, m.created_at
		FROM messages m
		JOIN users u ON u.id = m.sender_id
		WHERE m.group_id = $1 AND m.deleted_at IS NULL AND ($2::timestamptz IS NULL OR m.created_at < $2)
		ORDER BY m.created_at DESC
		LIMIT $3`
	var beforePtr *time.Time
	if !before.IsZero() {
		beforePtr = &before
	}
	rows, err := r.db.Query(ctx, query, groupID, beforePtr, limit)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	defer rows.Close()
	messages := make([]domain.Message, 0)
	for rows.Next() {
		var message domain.Message
		if err := rows.Scan(&message.ID, &message.GroupID, &message.SenderID, &message.SenderName, &message.Text, &message.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		messages = append(messages, message)
	}
	return messages, rows.Err()
}

func (r *Repository) CountGroupMembers(ctx context.Context, groupID string) (int, error) {
	var count int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*)::int FROM group_members WHERE group_id = $1`, groupID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count group members: %w", err)
	}
	return count, nil
}

func (r *Repository) IsGroupMember(ctx context.Context, groupID, userID string) (bool, error) {
	var exists bool
	if err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM group_members WHERE group_id = $1 AND user_id = $2)`, groupID, userID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check group member: %w", err)
	}
	return exists, nil
}

func (r *Repository) GetMemberRole(ctx context.Context, groupID, userID string) (domain.GroupRole, error) {
	var role domain.GroupRole
	if err := r.db.QueryRow(ctx, `SELECT role FROM group_members WHERE group_id = $1 AND user_id = $2`, groupID, userID).Scan(&role); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrForbidden
		}
		return "", fmt.Errorf("get member role: %w", err)
	}
	return role, nil
}

func nullableInviteCode(inviteCode string) any {
	value := strings.TrimSpace(inviteCode)
	if value == "" {
		return nil
	}
	return strings.ToUpper(value)
}

func normalizeStoredInviteCode(inviteCode string) string {
	value := strings.ToUpper(strings.TrimSpace(inviteCode))
	value = strings.ReplaceAll(value, "-", "")
	return value
}

func normalizeSearchPhone(phone string) string {
	phone = strings.TrimSpace(phone)
	phone = strings.ReplaceAll(phone, " ", "")
	phone = strings.ReplaceAll(phone, "-", "")
	phone = strings.ReplaceAll(phone, "(", "")
	phone = strings.ReplaceAll(phone, ")", "")
	if phone == "" {
		return ""
	}
	if strings.HasPrefix(phone, "00") && len(phone) > 2 {
		phone = "+" + strings.TrimPrefix(phone, "00")
	}
	if strings.HasPrefix(phone, "+") {
		return phone
	}
	if strings.HasPrefix(phone, "996") {
		return "+" + phone
	}
	if strings.HasPrefix(phone, "0") && len(phone) == 10 {
		return "+996" + strings.TrimPrefix(phone, "0")
	}
	if len(phone) == 9 {
		return "+996" + phone
	}
	return phone
}

func nullableString(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

var (
	ErrNotFound  = errors.New("not found")
	ErrForbidden = errors.New("forbidden")
)

func (r *Repository) ensurePublicRequestReadsTable(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS public_request_reads (
			group_id TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			last_read_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			PRIMARY KEY (group_id, user_id)
		);
		CREATE INDEX IF NOT EXISTS idx_public_request_reads_user_group ON public_request_reads (user_id, group_id);`)
	if err != nil {
		return fmt.Errorf("ensure public request reads table: %w", err)
	}
	return nil
}

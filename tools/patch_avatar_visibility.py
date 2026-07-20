from pathlib import Path


def replace_once(path: str, old: str, new: str) -> None:
    file_path = Path(path)
    text = file_path.read_text(encoding="utf-8")
    count = text.count(old)
    if count != 1:
        raise RuntimeError(f"Expected exactly one match in {path}, found {count}: {old[:120]!r}")
    file_path.write_text(text.replace(old, new, 1), encoding="utf-8")


# Publications must carry the author's current avatar in list, create and realtime payloads.
replace_once(
    "internal/domain/public_requests.go",
    '\tAuthorName      string                       `json:"author_name"`\n\tRequestType',
    '\tAuthorName      string                       `json:"author_name"`\n\tAuthorAvatarData string                       `json:"author_avatar_data,omitempty"`\n\tRequestType',
)

replace_once(
    "internal/storage/repository_public_requests.go",
    "\trequest.AuthorName = user.DisplayName\n\treturn request, nil",
    "\trequest.AuthorName = user.DisplayName\n\trequest.AuthorAvatarData = user.AvatarData\n\treturn request, nil",
)
replace_once(
    "internal/storage/repository_public_requests.go",
    "\t\tSELECT pr.id, pr.group_id, pr.author_id, u.display_name, pr.request_type, pr.interaction_mode, pr.title, pr.body, pr.status,",
    "\t\tSELECT pr.id, pr.group_id, pr.author_id, u.display_name, COALESCE(u.avatar_data, ''), pr.request_type, pr.interaction_mode, pr.title, pr.body, pr.status,",
)
replace_once(
    "internal/storage/repository_public_requests.go",
    "\t\tGROUP BY pr.id, u.display_name, myv.vote_type",
    "\t\tGROUP BY pr.id, u.display_name, u.avatar_data, myv.vote_type",
)
replace_once(
    "internal/storage/repository_public_requests.go",
    "\t\t\t&request.AuthorName,\n\t\t\t&request.RequestType,",
    "\t\t\t&request.AuthorName,\n\t\t\t&request.AuthorAvatarData,\n\t\t\t&request.RequestType,",
)

# Every common user lookup must hydrate AvatarData, otherwise immediate API/realtime responses lose it.
replace_once(
    "internal/storage/repository.go",
    "query := `SELECT id, COALESCE(email, ''), COALESCE(phone, ''), display_name, COALESCE(password_hash, ''), COALESCE(role, 'user'), created_at FROM users WHERE email = $1`",
    "query := `SELECT id, COALESCE(email, ''), COALESCE(phone, ''), display_name, COALESCE(password_hash, ''), COALESCE(role, 'user'), COALESCE(avatar_data, ''), created_at FROM users WHERE email = $1`",
)
replace_once(
    "internal/storage/repository.go",
    "\t\t&result.User.Role,\n\t\t&result.User.CreatedAt,",
    "\t\t&result.User.Role,\n\t\t&result.User.AvatarData,\n\t\t&result.User.CreatedAt,",
)
replace_once(
    "internal/storage/repository.go",
    "\t\tSELECT id, COALESCE(email, ''), COALESCE(NULLIF(phone, ''), phone_number, '') AS phone, display_name, COALESCE(role, 'user'), created_at",
    "\t\tSELECT id, COALESCE(email, ''), COALESCE(NULLIF(phone, ''), phone_number, '') AS phone, display_name, COALESCE(role, 'user'), COALESCE(avatar_data, ''), created_at",
)
replace_once(
    "internal/storage/repository.go",
    "err := r.db.QueryRow(ctx, query, phone).Scan(&user.ID, &user.Email, &user.Phone, &user.DisplayName, &user.Role, &user.CreatedAt)",
    "err := r.db.QueryRow(ctx, query, phone).Scan(&user.ID, &user.Email, &user.Phone, &user.DisplayName, &user.Role, &user.AvatarData, &user.CreatedAt)",
)
replace_once(
    "internal/storage/repository.go",
    "query := `SELECT id, COALESCE(email, ''), COALESCE(phone, ''), display_name, COALESCE(role, 'user'), created_at FROM users WHERE id = $1`",
    "query := `SELECT id, COALESCE(email, ''), COALESCE(NULLIF(phone, ''), phone_number, '') AS phone, display_name, COALESCE(role, 'user'), COALESCE(avatar_data, ''), created_at FROM users WHERE id = $1`",
)
replace_once(
    "internal/storage/repository.go",
    "err := r.db.QueryRow(ctx, query, userID).Scan(&user.ID, &user.Email, &user.Phone, &user.DisplayName, &user.Role, &user.CreatedAt)",
    "err := r.db.QueryRow(ctx, query, userID).Scan(&user.ID, &user.Email, &user.Phone, &user.DisplayName, &user.Role, &user.AvatarData, &user.CreatedAt)",
)

# Phone authentication/session responses also need the avatar.
replace_once(
    "internal/domain/phone_auth.go",
    '\tRole        UserRole  `json:"role"`\n\tCreatedAt',
    '\tRole        UserRole  `json:"role"`\n\tAvatarData  string    `json:"avatar_data,omitempty"`\n\tCreatedAt',
)
replace_once(
    "internal/storage/repository_phone_auth.go",
    "query := `SELECT id, COALESCE(phone_number, ''), display_name, created_at FROM users WHERE phone_number = $1`",
    "query := `SELECT id, COALESCE(phone_number, ''), display_name, COALESCE(role, 'user'), COALESCE(avatar_data, ''), created_at FROM users WHERE phone_number = $1`",
)
replace_once(
    "internal/storage/repository_phone_auth.go",
    "err := r.db.QueryRow(ctx, query, mobile).Scan(&user.ID, &user.Mobile, &user.DisplayName, &user.CreatedAt)",
    "err := r.db.QueryRow(ctx, query, mobile).Scan(&user.ID, &user.Mobile, &user.DisplayName, &user.Role, &user.AvatarData, &user.CreatedAt)",
)
replace_once(
    "internal/storage/repository_phone_auth.go",
    "query := `SELECT id, COALESCE(email, ''), COALESCE(phone_number, ''), display_name, created_at FROM users WHERE id = $1`",
    "query := `SELECT id, COALESCE(email, ''), COALESCE(NULLIF(phone, ''), phone_number, '') AS phone, display_name, COALESCE(role, 'user'), COALESCE(avatar_data, ''), created_at FROM users WHERE id = $1`",
)
replace_once(
    "internal/storage/repository_phone_auth.go",
    "\tvar createdAt time.Time\n\terr := r.db.QueryRow(ctx, query, userID).Scan(&id, &email, &mobile, &displayName, &createdAt)",
    "\tvar role domain.UserRole\n\tvar avatarData string\n\tvar createdAt time.Time\n\terr := r.db.QueryRow(ctx, query, userID).Scan(&id, &email, &mobile, &displayName, &role, &avatarData, &createdAt)",
)
replace_once(
    "internal/storage/repository_phone_auth.go",
    "return domain.User{ID: id, Email: email, DisplayName: displayName, CreatedAt: createdAt}, nil",
    "return domain.User{ID: id, Email: email, Phone: mobile, DisplayName: displayName, Role: role, AvatarData: avatarData, CreatedAt: createdAt}, nil",
)

# Notify all connected members immediately when a group image changes.
replace_once(
    "internal/httpapi/group_avatar_handlers.go",
    '\t"net/http"\n\n\t"github.com/go-chi/chi/v5"',
    '\t"net/http"\n\n\t"github.com/NursultanKoshoev11/MobileChatServer/internal/realtime"\n\t"github.com/go-chi/chi/v5"',
)
replace_once(
    "internal/httpapi/group_avatar_handlers.go",
    "\twriteJSON(w, http.StatusOK, group)",
    "\ts.broadcastGroupAndUsers(r, group.ID, realtime.Event{Type: \"group.avatar_updated\", GroupID: group.ID, Payload: group})\n\twriteJSON(w, http.StatusOK, group)",
)

# Remove the temporary patch machinery from the resulting branch commit.
Path(".github/workflows/apply-avatar-visibility-fix.yml").unlink(missing_ok=True)
Path(__file__).unlink(missing_ok=True)

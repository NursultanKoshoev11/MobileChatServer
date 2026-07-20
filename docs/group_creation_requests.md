# Official group creation request flow

This server flow replaces direct group creation for normal users and platform administrators.

## Roles and capabilities

Users have one of these roles:

- `user` — normal application user.
- `platform_admin` — reviews only official group creation requests.
- `super_admin` — project owner with full platform-level privileges.

Capabilities are defined centrally in `internal/domain/permissions.go` instead of repeating raw role comparisons throughout the code. This makes future privileges easier to add safely.

| Capability | user | platform_admin | super_admin |
| --- | --- | --- | --- |
| Submit own group creation request | Yes | Yes | Yes |
| Review, approve, reject, request more information | No | Yes | Yes |
| Create a group directly | No | No | Yes |
| Delete any group globally | No | No | Yes |
| Moderate any group without group membership | No | No | Yes |
| Moderate a group as its owner/admin | If assigned | If assigned | If assigned or globally |

The authenticated user role is returned from:

```http
GET /api/me
```

Phone auth sessions also return `user.role` after SMS verification.

## Admin allowlist

Admin access is controlled server-side through `admin_phone_allowlist`.

Project owner example:

```sql
INSERT INTO admin_phone_allowlist (phone, role)
VALUES ('+996000000000', 'super_admin')
ON CONFLICT (phone) DO UPDATE
SET role = EXCLUDED.role, enabled = true, updated_at = now();
```

Group request reviewer example:

```sql
INSERT INTO admin_phone_allowlist (phone, role)
VALUES ('+996700000001', 'platform_admin')
ON CONFLICT (phone) DO UPDATE
SET role = EXCLUDED.role, enabled = true, updated_at = now();
```

After a phone number passes SMS verification, the server updates the user role from the allowlist.

## User endpoints

Create a request to open an official group:

```http
POST /api/group-creation-requests
Authorization: Bearer <token>
Content-Type: application/json
```

```json
{
  "applicant_name": "Full name",
  "position": "Mayor / employee / representative",
  "organization_name": "City Hall",
  "organization_type": "city_government",
  "region": "City / region",
  "official_phone": "+996...",
  "official_email": "info@example.gov",
  "website": "https://example.gov",
  "group_title": "Official city group",
  "group_description": "Official announcements and citizen feedback",
  "reason": "Why this group is needed",
  "documents": "Official letter, staff ID, organization seal, etc."
}
```

List my requests:

```http
GET /api/group-creation-requests
Authorization: Bearer <token>
```

## Group request review endpoints

These endpoints are available to `platform_admin` and `super_admin`.

List all requests:

```http
GET /api/admin/group-creation-requests?status=pending&limit=100
Authorization: Bearer <admin-token>
```

Approve and create the group:

```http
POST /api/admin/group-creation-requests/{requestID}/approve
Authorization: Bearer <admin-token>
Content-Type: application/json
```

```json
{
  "admin_comment": "Verified by official documents."
}
```

Reject:

```http
POST /api/admin/group-creation-requests/{requestID}/reject
Authorization: Bearer <admin-token>
Content-Type: application/json
```

Need more information:

```http
POST /api/admin/group-creation-requests/{requestID}/need-more-info
Authorization: Bearer <admin-token>
Content-Type: application/json
```

## Super admin operations

`POST /api/groups` is restricted to `super_admin`. Other users, including `platform_admin`, must use `POST /api/group-creation-requests`.

`DELETE /api/admin/groups/{groupID}` is also restricted to `super_admin`, despite the legacy endpoint and method names containing `platform_admin` for backward compatibility.

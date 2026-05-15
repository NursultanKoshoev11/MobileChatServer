# Official group creation request flow

This server flow replaces direct group creation for normal users.

## Roles

Users have one of these roles:

- `user`
- `platform_admin`
- `super_admin`

The authenticated user role is returned from:

```http
GET /api/me
```

Phone auth sessions also return `user.role` after SMS verification.

## Admin allowlist

Admin access is controlled server-side through `admin_phone_allowlist`.

Example:

```sql
INSERT INTO admin_phone_allowlist (phone, role)
VALUES ('+996000000000', 'super_admin')
ON CONFLICT (phone) DO UPDATE
SET role = EXCLUDED.role, enabled = true, updated_at = now();
```

After this phone number passes SMS verification, the server updates the user role from the allowlist.

## Normal user endpoints

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

## Admin endpoints

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

## Direct group creation

`POST /api/groups` is now restricted to `platform_admin` and `super_admin`.
Normal users must use `POST /api/group-creation-requests`.

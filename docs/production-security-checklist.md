# Production Security Checklist

## Authentication

- Phone verification codes expire quickly.
- Wrong code attempts are limited.
- Code request frequency is limited by phone number and IP address.
- Refresh tokens are stored as hashes.
- Refresh tokens are rotated after use.
- Development SMS mode is disabled in production.

## Audit logging

The backend includes database and service helpers for audit events. Important flows should record events for authentication, refresh, group creation, invites, member changes, and messages.

## Group member management

Required authorization rules:

- Members can view the member list.
- Owner can promote and demote admins.
- Owner and admins can remove regular members.
- Admins cannot remove other admins or owner.
- Owner cannot be removed by regular member removal.
- Members can leave a group.

## Deployment

- Use HTTPS in production.
- Store secrets in environment variables or a secret manager.
- Keep `.env` and signing keys outside Docker images.
- Run server CI, CodeQL, dependency checks, and container scanning before release.
- Keep deployment, PostgreSQL volume, media directory, and backup paths stable;
  inspect existing host mounts before any recovery or migration.
- Verify a recent database dump and media archive, checksums, and offsite upload.

## Privacy and store release

- Group member roster APIs do not return personal phone numbers.
- The mobile app and an external web page must provide an account-deletion flow.
  The current app has no deletion endpoint or in-app control, and the current
  privacy page describes email requests instead. Define which user data is
  deleted or retained before implementing the data-removal operation.
- Publish privacy and child-safety information in the languages used by the
  target audience and keep the store data-safety declarations consistent with
  actual collection, sharing, and retention.

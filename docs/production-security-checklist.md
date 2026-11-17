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
- Run server CI (`go test`, `go vet`, and pinned `govulncheck`), CodeQL for Go, and
  Trivy image scanning before release. The production package workflow stops
  on HIGH or CRITICAL OS/library vulnerabilities. Add the CodeQL workflow check
  and production image scan as required branch/release checks after they pass
  on GitHub; CodeQL result upload requires a public repository or GitHub Code
  Security entitlement.
- Keep deployment, PostgreSQL volume, media directory, and backup paths stable;
  inspect existing host mounts before any recovery or migration.
- Verify a recent database dump and media archive, checksums, and offsite upload.

## Privacy and store release

- Group member roster APIs do not return personal phone numbers.
- The app and `/account-deletion` page provide account deletion. Public requests
  and comments are retained without author identity; private messages, votes,
  profile data, and session tokens are removed. Confirm the Play Console web
  deletion URL and test the flow after the database migration is deployed.
- Set S3 lifecycle retention for current objects and noncurrent versions. The
  backup script prunes expired current objects, while S3 versioning requires a
  bucket lifecycle rule to expire historical versions too.
- Publish privacy and child-safety information in the languages used by the
  target audience and keep the store data-safety declarations consistent with
  actual collection, sharing, and retention.

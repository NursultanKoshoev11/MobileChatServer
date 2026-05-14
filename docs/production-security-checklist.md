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

# Security Policy

## Supported Version

The `main` branch is the active development branch for Koom Server.

## Reporting a Vulnerability

Do not create a public GitHub issue for security vulnerabilities.

Send a private report to the project owner with:

- Vulnerability summary
- Affected endpoint or component
- Steps to reproduce
- Expected impact
- Suggested fix, if known

## Security Requirements

The server must keep these controls enabled before production use:

- HTTPS only in production
- Strong `JWT_SECRET` with at least 32 characters
- An implemented production SMS provider configured; `SMS_PROVIDER=dev` and `SMS_PROVIDER=disabled` are rejected in production
- Passwordless phone verification codes expire after a short TTL
- Verification code issuance and attempts are limited atomically in PostgreSQL
- `TRUSTED_PROXY_CIDRS` is restricted to the reverse proxy's private network
- Refresh tokens are stored as hashes and rotated
- CI must run tests, `go vet`, and vulnerability checks
- Secrets must not be committed to the repository

## Production Checklist

Before production deployment:

1. Set `APP_ENV=production`.
2. Use a production PostgreSQL instance with backups.
3. Integrate and configure a real SMS provider before enabling production sign-in. No production SMS provider is currently integrated.
4. Store secrets in environment variables or a secret manager.
5. Put the API behind HTTPS.
6. Enable log monitoring and alerting.
7. Review authentication and group invitation flows.

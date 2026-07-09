# Production Deployment Checklist

This checklist is for deploying the Koom backend behind HTTPS.

## 1. DNS

DuckDNS should point the domain to the server public IP:

```text
koommy.duckdns.org -> 16.171.171.191
```

## 2. Firewall / Security Group

Expose only the public web ports:

```text
80/tcp   HTTP, used by Caddy for certificate issuance and redirects
443/tcp  HTTPS, used by mobile clients
```

Do not expose the backend port directly to the internet in production. Bind the API to localhost through Docker Compose:

```yaml
ports:
  - "127.0.0.1:8080:8080"
```

## 3. HTTPS reverse proxy

Install Caddy on the server and use this Caddyfile:

```caddyfile
koommy.duckdns.org {
    reverse_proxy 127.0.0.1:8080
}
```

Then reload Caddy:

```bash
sudo systemctl reload caddy
sudo systemctl status caddy
```

The public healthcheck should be:

```bash
curl https://koommy.duckdns.org/api/health
```

Expected response:

```json
{"status":"ok"}
```

## 4. Production environment

Use `APP_ENV=production` and do not use development secrets.

Required values:

```env
APP_ENV=production
DATABASE_URL=postgres://...
JWT_SECRET=<long random secret, at least 32 chars>
ALLOWED_ORIGINS=https://koommy.duckdns.org
SMS_PROVIDER=<real provider>
RUN_MIGRATIONS_ON_START=false
```

`SMS_PROVIDER=dev` is not allowed in production.

## 5. Migrations

Run migrations before starting or updating the production API:

```bash
docker compose -f docker-compose.prod.example.yml run --rm api /app/mobilechat-migrate
```

## 6. Mobile app API URL

The Flutter app default API URL is:

```text
https://koommy.duckdns.org
```

For custom builds, override it with:

```bash
flutter build apk --dart-define=API_BASE_URL=https://koommy.duckdns.org
```

## 7. Security notes

- Keep backend port `8080` private.
- Use exact `ALLOWED_ORIGINS`, not `*`, for production.
- Store secrets outside Git.
- Use a real SMS provider before allowing public sign-in.
- Rotate `JWT_SECRET` if it was ever exposed.
- Keep Caddy, Docker, and OS packages updated.

# Koom production deployment

The production stack is defined in `docker-compose.prod.yml`. It runs the API,
PostgreSQL, Caddy, and a database/media backup worker. Caddy terminates HTTPS
and proxies requests to the API container.

## Server prerequisites

- Point `koom.com.kg` to the production server and expose TCP ports 80 and 443.
- Keep API port 8080 bound to `127.0.0.1`; do not expose PostgreSQL.
- Create the external Docker networks `stroy_default` and
  `stroy-website_default` before starting Caddy. They are shared with other
  services on the host.
- Keep the repository in a stable directory with a protected `.env.prod` file.
  The compose file uses relative `./data` and `./backups` bind mounts and a
  project-scoped PostgreSQL volume. Changing the directory or Compose project
  name can point the stack at different data. Confirm the existing volume and
  mount paths before moving the deployment.

The GitHub production workflow currently expects the checkout at
`/opt/projects/MobileChatServer`. The latest deployment run visible in the
history reviewed on 2026-11-17 reported that this directory was missing.
Confirm the actual path against the host before enabling a deployment; do not
restore an old environment file or switch the Compose working directory until
the active database volume and data paths are identified.

## Environment

Start from `.env.prod.example`, write real values to `.env.prod`, and restrict
the file to the deployment account. Never commit it. Production requires:

- `APP_ENV=production`, `DATABASE_URL`, and a random `JWT_SECRET` of at least
  32 characters;
- exact `ALLOWED_ORIGINS=https://koom.com.kg`;
- `SMS_PROVIDER=twilio` with a valid account token and sender ID or messaging
  service;
- `TRUSTED_PROXY_CIDRS` set to the actual private proxy subnet;
- `TEST_AUTH_ENABLED=false` and `MODERATION_FAIL_CLOSED=true`.

The backup health check requires a recent validated database dump and media
archive plus `offsite=uploaded`. Configure `BACKUP_S3_URI` and AWS credentials
or a working instance role before treating backups as healthy. The script can
create local-only backups, but those do not pass the production health check.
It prunes expired Koom backup objects from the configured S3 prefix according
to `BACKUP_RETENTION_DAYS`. For versioned S3 buckets, add a lifecycle rule to
expire noncurrent object versions as well; the script cannot remove those
versions through ordinary object deletion.

## First deployment

Run these commands from the stable production repository directory after
creating `.env.prod` and the external Docker networks:

```bash
sudo docker compose -f docker-compose.prod.yml --env-file .env.prod config -q
sudo docker compose -f docker-compose.prod.yml --env-file .env.prod build api migrate backup
sudo docker compose -f docker-compose.prod.yml --env-file .env.prod up -d postgres
sudo docker compose -f docker-compose.prod.yml --env-file .env.prod run --rm migrate
sudo docker compose -f docker-compose.prod.yml --env-file .env.prod up -d api caddy backup
```

## Routine backend release

The GitHub workflow verifies tests, static analysis, vulnerability checks, and
builds before deployment. On the server it builds the API and migration images,
runs migrations, then updates the API and Caddy. If backup scripts or the
backup image change, rebuild and recreate the backup service as part of the
release too.

Before a manual recovery or migration, take a fresh database and media backup.
Application rollback does not automatically reverse database migrations, so
only deploy schema changes that remain compatible with the previous API image.

## Post-deployment checks

```bash
curl --fail https://koom.com.kg/api/health
sudo docker compose -f docker-compose.prod.yml --env-file .env.prod ps
sudo docker compose -f docker-compose.prod.yml --env-file .env.prod logs --tail=100 api caddy backup
```

The health endpoint returns `{"status":"ok","database":"ok"}` when the API
can reach PostgreSQL. Confirm the backup worker is healthy and that the latest
database and media archives have matching checksum files and were uploaded
offsite.

## Mobile API URL

Android and iOS production builds should use `https://koom.com.kg`. The mobile
workflows use that value by default and require HTTPS for manual overrides.

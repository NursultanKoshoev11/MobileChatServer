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

The restricted host-side deploy helper uses
`/opt/projects/MobileChatServer`. Production deploy run 11 and the 18 November
run reached the host but stopped in preflight because SMS credentials were not
configured. They stopped before loading images or applying migrations. The
repository now supports SMSPro; the production `.env.prod` and separately
installed host-side helper must both be updated before a release. The current
workflow uses a pinned SSH host key and a key limited to preflight/deploy
commands. Verify the active database volume and data mounts before the next
release; do not change Compose paths or project names until that mapping is
confirmed.

## GitHub release controls

As of 2026-11-17, the repository requires a pull request and a current `backend`
CI check for `main`; administrators are covered, and force pushes and branch
deletion are disabled. The required approving-review count is currently zero,
so this does not provide independent human review. CodeQL and Trivy are not yet
required checks. The current GitHub `main` workflow still has a push-triggered
production deploy; this pilot branch changes it to a manually confirmed release.
After that change is merged, restrict the `production` environment to `main` and
require an approver where the GitHub plan supports it.

## Identify the live Docker project

Use read-only Docker metadata to map the running API and PostgreSQL containers
to their Compose project, working directory, config file, and mounts. Do not
read container environment variables during this check.

```bash
sudo docker ps -a --format '{{.ID}} {{.Names}} {{.Image}} {{.Status}}'
sudo docker inspect --format '{{.Name}} project={{index .Config.Labels "com.docker.compose.project"}} service={{index .Config.Labels "com.docker.compose.service"}} workdir={{index .Config.Labels "com.docker.compose.project.working_dir"}} files={{index .Config.Labels "com.docker.compose.project.config_files"}} mounts={{range .Mounts}}{{.Type}}:{{.Name}}:{{.Source}}->{{.Destination}};{{end}}' <api-container> <postgres-container>
sudo docker volume inspect <postgres-volume>
```

Confirm the database volume and uploaded-file mount against the live project
before setting `DEPLOY_DIR` or running Compose. An old code archive, image tag,
or volume name alone does not prove which database is serving production.

## Environment

Start from `.env.prod.example`, write real values to `.env.prod`, and restrict
the file to the deployment account. Never commit it. Production requires:

- `APP_ENV=production`, `DATABASE_URL`, and a random `JWT_SECRET` of at least
  32 characters;
- exact `ALLOWED_ORIGINS=https://koom.com.kg`;
- `SMS_PROVIDER=smspro` with `SMSPRO_LOGIN`, `SMSPRO_PASSWORD`, and an approved
  `SMSPRO_SENDER` set only in the protected `.env.prod` file;
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

## SMSPro integration

The API sends verification codes over HTTPS to
`https://smspro.nikita.kg/api/message` as UTF-8 XML in a `POST`. Keep the provider
login and password in the server secret file; never put them in source, logs, or
mobile builds. `SMSPRO_SENDER` must be approved in the SMSPro account and match
the provider's sender-ID format. The adapter sends one recipient per request and
does not set the optional scheduled-send time. See the
[official SMSPro XML protocol](https://smspro.nikita.kg/kg/documents/smspro.nikita.kg-XML-api.pdf).

The provider's `status=0` means the SMS was accepted into its sending queue, not
confirmed delivered. A provider processing-timeout (`status=9`) is retried once
after five seconds with the same 12-character request ID to avoid duplicate
submissions. Other provider statuses fail the request without logging the
response message, phone, verification code, or credentials. SMSPro's documented
`test=1` mode does not send a real SMS, so it is not enabled for the app's OTP
flow.

Provider status `3` indicates the production server's egress IP is not allowed
by the SMSPro account; add the server egress IP to the provider allowlist if
that control is enabled. Status `4` means the account balance is insufficient.

Configure `SMS_PROVIDER=smspro`, `SMSPRO_LOGIN`, `SMSPRO_PASSWORD`, and
`SMSPRO_SENDER` in `.env.prod`. Before deploying, update the separately
installed `/usr/local/sbin/koom-github-deploy` helper to accept and validate
these SMSPro settings; changing the repository copy alone does not update the
server helper.

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
builds before deployment. Production deployment is manual from `main`: select
`deploy` only after checking the live Compose directory, project name,
PostgreSQL volume, and latest backup. Pushes to `main` do not deploy. The
workflow builds API and migration images on a hosted runner, then transfers a
short-lived image bundle through the pinned SSH host key and restricted deploy
key. The server helper checks the commit embedded in the bundle and image
labels, runs migrations, updates the API, and checks container and public API
health. If release checks fail, it restores the previous API image; database
migrations are not reversed. If backup scripts or the backup image change,
rebuild and recreate the backup service as part of the release too.

The restricted helper installed at `/usr/local/sbin/koom-github-deploy` is
managed separately from the deployment artifact. Review and install changes to
that host-side helper through a controlled server-admin procedure; a normal
application release does not update it.

The static-site helper at `/usr/local/bin/koom-edge-site-deploy` is also
installed separately from the website package. Install reviewed helper changes
through the edge server-admin procedure; a static-site workflow does not update
the helper.

Before a manual recovery or migration, take a fresh database and media backup.
Application rollback does not automatically reverse database migrations, so
only deploy schema changes that remain compatible with the previous API image.

## Pre-deployment backup and rollback

The host-side helper (`scripts/koom-github-deploy`, installed at
`/usr/local/sbin/koom-github-deploy`) dumps the live database into
`backups/pre-deploy/` and validates it **before** switching images or running
migrations; a failed or invalid dump aborts the release with nothing changed.
It keeps the newest five dumps. If `BACKUP_AGE_RECIPIENT` is set in
`.env.prod`, the dump is age-encrypted and `age` must be installed on the host
(otherwise the deploy aborts). It checks that free disk space exceeds the
database size plus 1 GiB. If the new API fails its health checks, the helper
re-tags and starts the previous API image and reports whether the rollback is
healthy; migrations are not reversed, so restore from the pre-deploy dump
(`docs/BACKUPS.md`) only if a migration itself caused the failure.

This behaviour only exists after the reviewed helper has been installed on
the host (see docs/SECURITY-OPERATIONS.md, section 4).

## Server changes required by the 2026-11 infrastructure hardening

None of these are applied by a normal release; the running production keeps
working without them.

1. Install the updated `/usr/local/sbin/koom-github-deploy` helper (pre-deploy
   backup, fixed rollback). Ensure `backups/` has room for an extra dump.
2. Update the server checkout's `docker-compose.prod.yml` and `scripts/` to
   this revision. The new compose file adds resource limits,
   `no-new-privileges`, `cap_drop: ALL` and a tmpfs `/tmp` for the API, and
   passes `SMS_ALLOWED_COUNTRY_PREFIXES` (default `+996`) and
   `SMS_GLOBAL_HOURLY_MAX` (default `500`) to the API. Run
   `docker compose -f docker-compose.prod.yml --env-file .env.prod config -q`
   first. The API picks the new settings up on its next deploy; postgres and
   backup only when they are recreated (a short database restart, do it in a
   quiet period).
3. Rebuild the backup image (`... build backup`, now includes `age`) and
   recreate the backup service, then enable backup encryption as described in
   docs/BACKUPS.md.
4. Optional: `BACKUP_S3_SSE`, `API_READ_ONLY_ROOTFS=true`, resource limit
   overrides (docs/SECURITY-OPERATIONS.md, section 9).
5. The migration runner now adds a nullable `checksum` column to
   `schema_migrations` and backfills it on its first run; an edited, already
   applied migration file now fails the migration step (and therefore the
   deploy) instead of being ignored.

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

## Edge Caddy security headers

The edge Caddy configuration in `deployment/edge/Caddyfile` supplies HSTS,
`X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`, and
`Permissions-Policy` for the full KOOM origin, including reverse-proxied API
and policy pages. The root and `/kg` pages also have a restrictive CSP.
This file is bind-mounted by the edge Compose project and is separate from
both the backend release and the static-site asset workflow; neither workflow
updates the running Caddy configuration.

Apply a reviewed edge configuration through the server-admin process. Back up
the host file at `/opt/projects/edge/Caddyfile`, install the reviewed version,
then validate the mounted file. Reload only after validation succeeds:

```bash
sudo docker exec oracle-caddy caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile
sudo docker exec oracle-caddy caddy reload --config /etc/caddy/Caddyfile --adapter caddyfile
```

Then verify each public HTML route, including `/account-deletion` after the
backend route has been released:

```bash
for path in / /kg /privacy /child-safety; do
  headers="$(curl --fail --silent --show-error --max-time 12 -D - -o /dev/null "https://koom.com.kg$path")"
  for name in Strict-Transport-Security X-Content-Type-Options X-Frame-Options Referrer-Policy Permissions-Policy; do
    grep -Fiq "$name:" <<<"$headers" || { echo "Missing $name on $path" >&2; exit 1; }
  done
done
```

The current live edge must be rechecked after its Caddy configuration is
updated; a successful website asset deployment alone does not apply these
headers. Apply and verify the Caddy change before installing the updated
`koom-edge-site-deploy` helper: its preflight now deliberately rejects a site
deployment while any of these headers are missing.

## Mobile API URL

Android and iOS production builds should use `https://koom.com.kg`. The mobile
workflows use that value by default and require HTTPS for manual overrides.

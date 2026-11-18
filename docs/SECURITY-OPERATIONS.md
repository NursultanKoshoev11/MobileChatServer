# KOOM security operations

Operational hardening that needs a human with server, DNS or GitHub admin
access. Nothing here is applied automatically by a release. Work through the
sections in order of risk; each step is designed to be done without downtime.

Related: [DEPLOYMENT.md](DEPLOYMENT.md), [BACKUPS.md](BACKUPS.md),
[production-security-checklist.md](production-security-checklist.md).

## 1. Separate the other projects from the KOOM origin

Today `koom.com.kg` is served by Caddy configs that also route other projects:

- `Caddyfile` (repo root, used by the `caddy` profile of
  `docker-compose.prod.yml`): `/mcp-work/*`, `/mcp-agent` and
  `/whatsapp/webhook*` on the KOOM site go to `mcpforwork-hub:3000`; the same
  Caddy also serves `ailuuclo.com.kg`, `ittechlabs.xyz`, `api.stroy.com.kg`,
  `stroy.com.kg` and a plain-HTTP site on the server's public IP
  (`/stroy-site/*`, `stroy-api`). The prod compose attaches Caddy to the
  external `stroy_default` and `stroy-website_default` networks for this.
- `deployment/edge/Caddyfile` (the live edge, container `oracle-caddy`): the
  same `/mcp-work`, `/mcp-agent` and `/whatsapp` matchers on `koom.com.kg`.
  **Check:** on the edge, `/whatsapp*` is proxied to the KOOM API upstream
  (the same private `:8080` target as the API), not to `mcpforwork-hub`.
  Confirm whether that is intended.

Why it matters: every service sharing the `koom.com.kg` origin shares its
cookies, CSP/HSTS scope and browser same-origin trust, and a vulnerability in
any of them is reachable as "KOOM". The shared Docker networks also let the
Caddy container (and anything that compromises it) reach the other projects'
containers, and vice versa. The plain-HTTP IP site serves traffic without TLS.

These routes are live for other projects; **do not just delete them**. Move
them one at a time:

1. Pick dedicated hostnames, e.g. `mcp.<other-domain>` for mcpforwork-hub,
   `wa-hook.<other-domain>` for the WhatsApp webhook, and give the stroy /
   ailuu / ittechlabs sites their own Caddy (or keep them in a Caddy that does
   not serve `koom.com.kg`). Prefer a domain that is not a subdomain of
   `koom.com.kg`, so cookies and HSTS `includeSubDomains` are separate.
2. Create DNS records with a low TTL (300 s) pointing to the host that will
   serve them.
3. Add a new site block for each hostname in the target Caddy (copy the
   existing `reverse_proxy` target), validate and reload:
   `caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile`, then
   `caddy reload ...`. Wait for the certificate to be issued.
4. Switch each client to the new URL: MCP clients' server URL, the WhatsApp
   (Meta) webhook callback URL in the Meta developer console, the stroy
   website/API base URLs. Verify traffic arrives at the new hostname (Caddy
   access logs).
5. Replace the old path matchers on `koom.com.kg` with a temporary
   `redir https://<new-host>{uri} 308` for one to two weeks (webhooks follow
   308 for POST; check the provider does), then remove them.
6. Replace the `http://<public-ip>` block with an HTTPS hostname, then remove
   it.
7. When KOOM's Caddy no longer proxies to them, remove the `stroy_default`
   and `stroy_website_default` networks from the `caddy` service in
   `docker-compose.prod.yml`.
8. Long term, run the other projects on a different VM. A compromise of a
   co-hosted project then no longer reaches the KOOM database host.

## 2. TRUSTED_PROXY_CIDRS and a pinned Docker subnet

The API uses `X-Forwarded-For` only when the TCP peer is inside
`TRUSTED_PROXY_CIDRS`; it then walks the header right to left and takes the
first address that is not a trusted proxy. Per-IP rate limits (OTP, login,
invite codes) depend on this.

- **Too broad** (e.g. a whole `/16` that also contains other projects'
  containers, or a VCN range with other hosts): anything in that range can set
  `X-Forwarded-For` and pick its own rate-limit identity, bypassing per-IP
  limits.
- **Wrong or stale** (the default `172.18.0.0/16` when the network actually got
  `172.19.0.0/16`): the proxy is not trusted, every request appears to come
  from the proxy's IP and all users share one rate-limit bucket, so one
  attacker can lock everybody out of OTP login.

Set it to exactly the proxy addresses that connect to the API:

- Edge Caddy on another VM reaching the API host over the private network:
  the edge VM's private IP as `/32` (find it with `ip -4 addr` on the edge).
  Do not use the whole subnet.
- Caddy in the same Compose project: the Caddy container's address in the
  pinned compose subnet (below), or that small subnet.

Verify after a change: make a request through the public URL and check the
API access log `remote=` field shows your real client IP, not the proxy.

Docker assigns bridge subnets dynamically (`172.17`, `172.18`, ... in creation
order), so a recreated network can silently change range. Pin it in
`docker-compose.prod.yml` once you have confirmed the current range
(`docker network inspect mobilechatserver_mobilechat`):

```yaml
networks:
  mobilechat:
    driver: bridge
    ipam:
      config:
        - subnet: 172.18.0.0/24   # use the range the network has today
```

Changing the IPAM config of an existing network requires recreating it
(`docker compose down` / `up`), which restarts every container on it,
including the external edge Caddy that joins `mobilechatserver_mobilechat`.
Do it in a maintenance window, pin the range the network already has, and
update `TRUSTED_PROXY_CIDRS` in the same change. This is intentionally not
done in the compose file automatically.

## 3. Firewall: port 8080 reachable from the edge only

The API listens on 8080 without TLS. Only the edge proxy may reach it.

- OCI: in the security list / NSG of the API host's subnet, allow TCP 8080
  ingress **only** from the edge VM's private IP `/32`; no `0.0.0.0/0` rule for
  8080. Allow 80/443 only on the edge. Restrict SSH (22) to the admin IP range
  or use the OCI Bastion service; GitHub-hosted runners need SSH only to the
  deploy hosts (see section 4).
- Host firewall as a second layer (`firewalld` on Oracle Linux):

  ```bash
  sudo firewall-cmd --permanent --new-zone=koom-edge
  sudo firewall-cmd --permanent --zone=koom-edge --add-source=<edge-private-ip>/32
  sudo firewall-cmd --permanent --zone=koom-edge --add-port=8080/tcp
  sudo firewall-cmd --reload
  ```

  Docker-published ports bypass `firewalld` INPUT rules because Docker inserts
  its own NAT rules. Add the restriction to the `DOCKER-USER` chain too (and
  persist it, e.g. with a systemd unit run after docker.service):

  ```bash
  sudo iptables -I DOCKER-USER -p tcp --dport 8080 ! -s <edge-private-ip>/32 -j DROP
  ```

- PostgreSQL is not published; keep it that way.
- Verify from an outside machine: `nc -vz <public-ip> 8080` must fail, and from
  the edge `curl http://<api-private-ip>:8080/api/health` must succeed.

## 4. SSH deploy keys: one key per host, forced command

Each GitHub deployment secret should be a dedicated ed25519 key used for one
host only (`KOOM_PROD_DEPLOY_KEY` for the API host, a different key for the
edge site deploy). Generate them offline, store only the private key in the
GitHub `production` environment secrets, never reuse a personal key.

`~opc/.ssh/authorized_keys` on the API host (one line):

```text
restrict,command="/usr/local/bin/koom-ssh-gateway",from="<GitHub runner egress ranges or bastion IP>" ssh-ed25519 AAAA... koom-prod-deploy-2026
```

(`/usr/local/bin/koom-ssh-gateway` stands for wherever `scripts/koom-ssh-gateway`
is installed on the host; check the existing `authorized_keys` line.) On the
edge host, the same pattern with the installed `scripts/koom-edge-ssh-gateway`.

- `restrict` disables port/agent/X11 forwarding and PTY allocation.
- `command=` forces the gateway script; it only accepts `preflight` and
  `deploy <40-hex-sha>` (or the edge equivalents) from
  `SSH_ORIGINAL_COMMAND`, and runs the root-owned helper via `sudo -n`.
- `from=` is optional for GitHub-hosted runners (their egress IPs are wide
  and change); use it if you deploy through a bastion or fixed-IP runner.
- In `/etc/sudoers.d/koom-deploy`, allow exactly the helper with no arguments
  wildcard beyond what the gateway passes, e.g.
  `opc ALL=(root) NOPASSWD: /usr/local/sbin/koom-github-deploy`.
- The helper, the gateway and the sudoers file must be root-owned and not
  writable by the deploy user (`chmod 755`, `chown root:root`).
- Keep `.github/koom-production-known-hosts` / `koom-edge-known-hosts` in sync
  with the hosts' real host keys; a host key change must fail the deploy.

Installing an updated helper (e.g. this release's pre-deploy backup): copy the
reviewed `scripts/koom-github-deploy` to the host, compare with `diff`, then
`sudo install -o root -g root -m 755 koom-github-deploy /usr/local/sbin/`.
Run `sudo /usr/local/sbin/koom-github-deploy --preflight` afterwards.

## 5. GitHub settings the owner must change by hand

These cannot be set from the repository:

1. **Branch protection on `main`** (Settings → Branches / Rulesets): require a
   pull request with **at least one approving review from someone other than
   the author**, dismiss stale approvals on new commits, require review from
   code owners if a `CODEOWNERS` file is added, require status checks
   `backend` (Backend Test Lab CI) plus CodeQL and the `verify` job of
   "Deploy Koom production", require branches to be up to date, include
   administrators, block force pushes and deletions.
2. **`production` environment** (Settings → Environments → production):
   deployment branches = `main` only; required reviewers = at least one person
   other than the release author; enable "Prevent self-review"; keep
   `KOOM_PROD_DEPLOY_KEY` (and the edge key) as environment secrets, not
   repository secrets.
3. **Self-hosted runners** (Settings → Actions → Runners): remove all
   self-hosted runners and deregister them on the hosts
   (`./config.sh remove`), delete their work directories and any tokens.
   Workflows only use `ubuntu-latest`.
4. **Actions settings**: default `GITHUB_TOKEN` permissions = read; disable
   "Allow GitHub Actions to create and approve pull requests"; require
   approval for workflows from outside collaborators/forks.
5. **Delete stale branches** after confirming nothing unmerged is needed
   (`git log main..origin/<branch>`):
   `codex/koom-deploy-fix`, `koom-production-deploy`, `koom-hardening`,
   `investor-ready-site`, every `rt-fix-*`, `security-hardening`, every `ai/*`.
   Old branches can carry old workflows with weaker triggers; a workflow on a
   non-default branch can still run for `pull_request`/`workflow_dispatch`.
6. Enable secret scanning + push protection and Dependabot alerts/security
   updates.

## 6. Monitoring and alerting

Minimum set, all with notifications to at least two people:

| Check | How | Alert when |
| --- | --- | --- |
| Public API | External uptime monitor (e.g. UptimeRobot, Better Stack, Grafana Cloud synthetic) on `https://koom.com.kg/api/health`, every 1-5 min from 2+ regions; assert body contains `"status":"ok"` and `"database":"ok"` | 2 consecutive failures |
| Backups | Backup container health (`docker inspect -f '{{.State.Health.Status}}' mobilechat-backup`) from a cron/systemd timer that pushes to a heartbeat monitor (healthchecks.io-style dead-man switch); also an S3 check that the newest object is < 26 h old | unhealthy, or no heartbeat for 26 h |
| Backup encryption | `grep -x encryption=age backups/backup-success` once encryption is enabled | `encryption=none` |
| TLS expiry | Uptime monitor certificate check on `koom.com.kg` (and every other hostname on the edge) | < 14 days to expiry |
| Disk | Host agent or cron (`df -P / /var/lib/docker`) | > 80 % used; also watch `./backups` growth |
| Containers | `docker ps --filter health=unhealthy` and restart counts | any unhealthy / restart loop |
| SMS spend | Log alert on the API container logs for the line `ALERT sms_global_budget_exceeded` (emitted when the global hourly verification-SMS budget `SMS_GLOBAL_HOURLY_MAX` is hit; SMS sending is paused) | any occurrence: investigate SMS pumping, then raise the limit only if traffic is legitimate |
| SMS provider | SMSPro balance and delivery failure rate (provider dashboard), errors in API logs | balance low / failures spike |
| Deploys | GitHub notifications for failed `Deploy Koom production` runs; the helper prints `ROLLBACK UNHEALTHY` if a rollback fails | any failure |

For log-based alerts, ship container logs (json-file driver today) with a
small agent (Grafana Alloy / Vector / Fluent Bit) to a hosted log service and
alert on the patterns above, or as a stop-gap run a cron job:
`docker logs --since 10m mobilechat-api 2>&1 | grep -q 'ALERT sms_global_budget_exceeded' && <notify>`.

## 7. Secret rotation runbook

General rules: rotate on any suspected leak, when someone with access leaves,
and at least yearly. Never paste secrets into chats, tickets or commits. Edit
`.env.prod` on the host as root, keep a dated root-only backup of the previous
file until the rotation is verified, then delete it.

**JWT_SECRET** (signs access tokens; rotating logs every user out of their
access token, refresh flows depend on the implementation):

1. `openssl rand -base64 48` → new value in `.env.prod`.
2. `docker compose -f docker-compose.prod.yml --env-file .env.prod up -d --no-deps api`
   (a few seconds of restart).
3. Check `/api/health`, log in from a test device. Expect users to re-auth.

**SMSPro (`SMSPRO_LOGIN`/`SMSPRO_PASSWORD`) / Twilio (`TWILIO_AUTH_TOKEN`)**:

1. Create the new password / secondary auth token in the provider console
   (Twilio supports a secondary token so old and new work in parallel).
2. Update `.env.prod`, recreate the api as above.
3. Send a verification SMS to a test number; then revoke the old credential
   (Twilio: promote secondary, delete old).
4. Review provider logs for unexpected sends during the exposure window.

**FCM service account (`FCM_CLIENT_EMAIL`, `FCM_PRIVATE_KEY`)**:

1. Google Cloud console → IAM → Service accounts → the FCM account → Keys →
   Add key (JSON). Only the `client_email` and `private_key` fields go into
   `.env.prod` (keep the `\n` escapes as today).
2. Recreate the api, send a test push.
3. Delete the old key in the console. Check Cloud audit logs for its use.

**Deploy SSH keys**:

1. `ssh-keygen -t ed25519 -N '' -C koom-prod-deploy-<date> -f koom-prod-deploy`
   on an admin machine.
2. Append the new public key with the same `restrict,command=...` prefix to
   `authorized_keys` on the host (keep the old line for now).
3. Replace the GitHub `production` environment secret `KOOM_PROD_DEPLOY_KEY`;
   run a deploy (or `preflight`) to prove it works.
4. Remove the old line from `authorized_keys`; shred the local private key file.
   Repeat separately for the edge key.

**Database password** (`POSTGRES_PASSWORD` + `DATABASE_URL`): run
`ALTER ROLE <user> PASSWORD '<new>'` via `docker exec -it mobilechat-postgres psql`,
update both values in `.env.prod`, recreate `api` and `backup`, run
`migrate` once to prove access.

**Backup age key**: generate a new key pair offline, add the new public key to
`BACKUP_AGE_RECIPIENT` alongside the old one, recreate `backup`, and keep the
old identity until every backup encrypted to it has expired from local and S3
retention. Then remove the old recipient and destroy the old identity.

**AWS backup credentials**: prefer an instance role. If static keys are used,
create a new access key for the backup IAM user (scoped to the bucket prefix,
plus KMS if SSE-KMS), update `.env.prod`, recreate `backup`, trigger a run
(`docker compose ... exec backup sh /scripts/backup-postgres.sh`), then
deactivate and delete the old key.

## 8. Backup restore drill

Monthly, run `scripts/restore-drill.sh` on the production host as described in
[BACKUPS.md](BACKUPS.md#monthly-restore-drill). It restores the newest dump into
a throwaway, network-less PostgreSQL container, checks row counts for `users`,
`groups` and `schema_migrations`, and always tears down. Quarterly, also
restore a media archive into a scratch directory and a backup downloaded from
S3. Record the result (date, file, counts, duration) and treat a failed drill
as an incident.

## 9. Store-review login (Apple App Review, Google Play)

Store reviewers cannot receive an SMS on a Kyrgyz number, so production has one
dedicated review account:

- `APP_REVIEW_PHONE`: a number used only for review. Prefer a landline
  (`+996312…`) so no real person can own it. Never an admin phone (startup
  refuses that).
- `APP_REVIEW_CODE`: a fixed 6-digit code. Startup refuses trivial codes
  (repeated digits, two distinct digits, ascending or descending sequences).

For this number `request-code` sends no SMS and answers `code_sent` exactly like
a real send. It never returns the code. Verification goes through the normal
flow: the code expires after 5 minutes, allows 5 attempts and is single-use. The
per-number limits still apply (3 requests per 10 minutes, 30 s apart, at most 8
per day), so guessing is capped at 40 attempts per day. Each issued review code
is logged as `app review login code issued`.

The account is an ordinary user. Before submission, sign in once and add it to a
demo group with sample content so reviewers can see the features.

Put the phone and code in App Store Connect → App Review Information → Sign-in
required (and in Play Console → App access). Rotate the code after each review
round. To turn the feature off, clear both variables and restart the API.

## 10. Container hardening switches

`docker-compose.prod.yml` sets memory/CPU limits (override with
`API_MEM_LIMIT`, `API_CPUS`, `POSTGRES_MEM_LIMIT`, `POSTGRES_CPUS`,
`BACKUP_MEM_LIMIT`, `BACKUP_CPUS`, `MIGRATE_MEM_LIMIT`, `MIGRATE_CPUS`,
`CADDY_MEM_LIMIT`, `CADDY_CPUS`; a `*_CPUS` value above the host CPU count
prevents the container from starting), `no-new-privileges`, `cap_drop: ALL`
for api/migrate and a tmpfs `/tmp` for the API (`API_TMPFS_SIZE`, default
128m; it counts toward the API memory limit).

`read_only` root filesystem for the API is opt-in: set
`API_READ_ONLY_ROOTFS=true` in `.env.prod`. As of this change the API writes
only to `PUBLIC_FILE_DIR` (`/app/data/public-files`, the `./data` bind mount)
and to `/tmp` (multipart upload spill); the image ran read-only locally and
passed its health check. Before enabling it, re-check for new write paths:
`git grep -nE 'os\.(WriteFile|Create|CreateTemp|MkdirAll|MkdirTemp|OpenFile)' -- '*.go'`.
Then recreate the API and test a public-file upload.

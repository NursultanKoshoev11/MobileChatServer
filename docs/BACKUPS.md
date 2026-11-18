# Backups, restores and the monthly restore drill

The `backup` service in `docker-compose.prod.yml` (image built from
`Dockerfile.backup`) runs `scripts/backup-postgres.sh` once a day. It writes to
`./backups` on the host:

| File | Content |
| --- | --- |
| `<prefix>-<UTC timestamp>.dump[.age]` | `pg_dump --format=custom` of `$POSTGRES_DB` |
| `<prefix>-files-<UTC timestamp>.tar.gz[.age]` | tarball of `./data` (uploaded/public files) |
| `*.sha256` | checksum of the file *as stored* (encrypted file if encrypted) |
| `backup-success` | `completed_at`, `offsite=uploaded|missing`, `encryption=age|none` |

`<prefix>` is `BACKUP_PREFIX` (production: `mobilechat`). Each dump is checked
with `pg_restore --list` and each media archive with `tar -tz` **before** it is
encrypted. Files older than `BACKUP_RETENTION_DAYS` are pruned locally and in
S3. The container health check requires a dump, a media archive, both checksum
files (plain or `.age`) younger than 25 hours and `offsite=uploaded`.

The service runs the script from the bind-mounted `./scripts` directory of the
server checkout, so script changes take effect when that checkout is updated
and the backup container is recreated. `age` only exists in the image after it
has been rebuilt from the new `Dockerfile.backup`.

## Encrypting backups with age (opt-in, recommended)

Without a recipient the script keeps the old plaintext behaviour and prints a
loud `WARNING: backups are NOT encrypted` block on every run.

1. On an **offline/admin machine** (never on the server), create a key pair and
   store the identity (secret key) in the password manager plus one offline copy:

   ```bash
   age-keygen -o koom-backup-identity.txt   # prints "Public key: age1..."
   ```

   Optionally create a second key pair for a second custodian; age encrypts to
   every listed recipient and either identity can decrypt.
2. Rebuild the backup image so it contains `age`:

   ```bash
   sudo docker compose -f docker-compose.prod.yml --env-file .env.prod build backup
   ```

3. Add the **public** key(s) to `.env.prod`, space-separated:

   ```bash
   BACKUP_AGE_RECIPIENT=age1xxxxxxxx...
   # or a file of recipients inside a mounted directory, e.g.
   # BACKUP_AGE_RECIPIENTS_FILE=/backups/age-recipients.txt
   ```

4. Recreate the service and check the next run:

   ```bash
   sudo docker compose -f docker-compose.prod.yml --env-file .env.prod up -d --no-deps backup
   sudo docker compose -f docker-compose.prod.yml --env-file .env.prod logs --tail=50 backup
   grep encryption ./backups/backup-success   # expect encryption=age
   ```

If a recipient is configured but `age` is missing, the run fails (no plaintext
fallback) and the health check turns unhealthy. The script also refuses values
that are not `age1...` public keys and recipient files containing
`AGE-SECRET-KEY`. **The identity must never be stored on the server**; it is
only brought to a host temporarily for a restore or drill.

Older plaintext backups remain plaintext until they age out of retention.

## S3 server-side encryption (opt-in)

Set `BACKUP_S3_SSE=AES256` (SSE-S3) or `BACKUP_S3_SSE=aws:kms` (SSE-KMS, with an
optional `BACKUP_S3_SSE_KMS_KEY_ID=<key id or ARN>`). With SSE-KMS the backup
credentials/instance role also need `kms:GenerateDataKey` (upload) and
`kms:Decrypt` (download) on that key. Prefer enabling default bucket encryption
as well, plus Object Lock or versioning with a lifecycle rule for noncurrent
versions. age encryption protects the data end to end; SSE is defence in depth.

## Restoring the database (destructive)

`scripts/restore-postgres.sh` runs `pg_restore --clean --if-exists` into the
target database. It refuses to run unless `RESTORE_CONFIRM` equals the target
database name, verifies the `.sha256` file (set `RESTORE_SKIP_CHECKSUM=1` to
override when it is missing), decrypts `.age` files with
`BACKUP_AGE_IDENTITY_FILE`, and validates the archive before touching the
database.

```bash
cd /opt/projects/MobileChatServer
# 1. Take a fresh backup of the current state first (even if it is broken).
sudo docker compose -f docker-compose.prod.yml --env-file .env.prod exec backup sh /scripts/backup-postgres.sh
# 2. Stop writers.
sudo docker compose -f docker-compose.prod.yml --env-file .env.prod stop api
# 3. Restore (copy the identity to the host only for the duration of the restore).
sudo docker compose -f docker-compose.prod.yml --env-file .env.prod run --rm --no-deps \
  -e RESTORE_CONFIRM=<value of POSTGRES_DB> \
  -e BACKUP_AGE_IDENTITY_FILE=/run/koom-age-identity \
  -v /root/koom-backup-identity.txt:/run/koom-age-identity:ro \
  backup /scripts/restore-postgres.sh /backups/mobilechat-YYYYMMDDTHHMMSSZ.dump.age
# 4. Start the API and check health; then shred the identity copy.
sudo docker compose -f docker-compose.prod.yml --env-file .env.prod up -d --no-deps api
curl --fail https://koom.com.kg/api/health
sudo shred -u /root/koom-backup-identity.txt
```

`RESTORE_DB` and `RESTORE_PGHOST` select another database/host (for example a
scratch database for inspection).

## Restoring uploaded media

`scripts/restore-media.sh` extracts a media archive into `RESTORE_MEDIA_TARGET`.
It requires `RESTORE_CONFIRM` to equal the target path, rejects archives with
absolute or `..` paths, and moves the current contents to
`<target>/.pre-restore-<timestamp>/` instead of deleting them. The backup
service mounts `./data` read-only, so mount it read-write for the restore:

```bash
sudo docker compose -f docker-compose.prod.yml --env-file .env.prod stop api
sudo docker compose -f docker-compose.prod.yml --env-file .env.prod run --rm --no-deps \
  -v ./data:/restore-target \
  -e RESTORE_MEDIA_TARGET=/restore-target -e RESTORE_CONFIRM=/restore-target \
  -e BACKUP_AGE_IDENTITY_FILE=/run/koom-age-identity \
  -v /root/koom-backup-identity.txt:/run/koom-age-identity:ro \
  backup /scripts/restore-media.sh /backups/mobilechat-files-YYYYMMDDTHHMMSSZ.tar.gz.age
sudo docker compose -f docker-compose.prod.yml --env-file .env.prod up -d --no-deps api
```

Check that restored files are owned by uid 10001 (the API user), then delete the
`.pre-restore-*` directory once the restore is verified (it is otherwise
included in later media backups).

## Pre-deployment database backups

`scripts/koom-github-deploy` (the host-side deploy helper) takes a
`pg_dump` into `/opt/projects/MobileChatServer/backups/pre-deploy/` before it
runs migrations and aborts the release if the dump fails or does not validate.
It keeps the newest `KOOM_PREDEPLOY_KEEP` dumps (default 5). If
`BACKUP_AGE_RECIPIENT` is set in `.env.prod` and `age` is installed on the host,
the dump is encrypted; if a recipient is set but `age` is missing on the host,
the deploy aborts. Install `age` on the host (`sudo dnf install age` or the
release binary) before setting a recipient.

## Monthly restore drill

Once a month (and after any change to backup scripts, PostgreSQL major version
or migrations that drop data), prove that the newest backup restores:

```bash
cd /opt/projects/MobileChatServer
sudo BACKUP_AGE_IDENTITY_FILE=/root/koom-backup-identity.txt scripts/restore-drill.sh
sudo shred -u /root/koom-backup-identity.txt
```

`scripts/restore-drill.sh`:

1. picks the newest `<prefix>-*.dump[.age]` in `./backups` (or
   `DRILL_BACKUP_FILE`) and fails if it is older than `DRILL_MAX_AGE_HOURS`
   (default 36);
2. verifies its sha256 and decrypts it into a private temp directory (host
   `age`, or the `mobilechat-backup:prod` image when `age` is not installed);
3. starts a throwaway `postgres:16-alpine` container with `--network none`, a
   random password and a unique name, restores with `--exit-on-error`;
4. runs sanity queries (row counts of `users`, `groups`, `schema_migrations`,
   latest migration) and fails if `users` has fewer than `DRILL_MIN_USERS`
   (default 1) rows;
5. removes the container and temp files on every exit path.

It never connects to the live database. Record the date, backup file name,
row counts and restore duration in the operations log. Every quarter also
restore one media archive into a scratch directory and one backup fetched from
S3 (proves the off-site copy and the age identity custody, not just the local
copy).

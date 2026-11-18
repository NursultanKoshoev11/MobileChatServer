# Docker quickstart (local development only)

`docker-compose.yml` and the `Makefile` targets start a **local development**
stack: weak default database credentials, a development JWT secret,
`SMS_PROVIDER=dev` and `ALLOWED_ORIGINS=*`. The API port is bound to
`127.0.0.1` only. **Do not use this file on a real server, even with a
customised `.env`.** Production runs `docker-compose.prod.yml` with
`.env.prod`, behind Caddy, and is released by the GitHub workflow; see
[docs/DEPLOYMENT.md](docs/DEPLOYMENT.md), [docs/BACKUPS.md](docs/BACKUPS.md)
and [docs/SECURITY-OPERATIONS.md](docs/SECURITY-OPERATIONS.md).

Optional: create `.env` from the example before starting:

```bash
cp .env.example .env
```

Start everything:

```bash
docker compose up -d --build
```

Check containers:

```bash
docker compose ps
```

Check API:

```bash
curl http://127.0.0.1:8080/api/health
```

View logs:

```bash
docker compose logs -f api
```

Stop:

```bash
docker compose down
```

Allow-list an admin phone in the local database after containers are running:

```bash
sh scripts/docker-create-admin.sh +996000000000 super_admin
```

The script targets the local stack by default. To use it against another
Compose project, set `COMPOSE_FILE`/`--env-file` through the environment and
`POSTGRES_USER`/`POSTGRES_DB` to match that project's database.

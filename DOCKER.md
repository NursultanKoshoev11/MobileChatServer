# Docker quickstart

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
curl http://localhost:8080/api/health
```

View logs:

```bash
docker compose logs -f api
```

Stop:

```bash
docker compose down
```

Optional: create `.env` from the example before starting:

```bash
cp .env.example .env
```

For a real server, change `POSTGRES_PASSWORD`, `DATABASE_URL`, `JWT_SECRET`, and `ALLOWED_ORIGINS` in `.env`.

Create an admin phone after containers are running:

```bash
sh scripts/docker-create-admin.sh +996000000000 super_admin
```

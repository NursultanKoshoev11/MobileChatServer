.PHONY: up down restart logs ps health admin rebuild

up:
	docker compose up -d --build

rebuild:
	docker compose build --no-cache
	docker compose up -d

down:
	docker compose down

restart:
	docker compose restart

logs:
	docker compose logs -f api

ps:
	docker compose ps

health:
	curl http://localhost:$${API_PORT:-8080}/api/health

admin:
	@if [ -z "$(PHONE)" ]; then echo "Usage: make admin PHONE=+996000000000 ROLE=super_admin"; exit 1; fi
	sh scripts/docker-create-admin.sh "$(PHONE)" "$(ROLE)"

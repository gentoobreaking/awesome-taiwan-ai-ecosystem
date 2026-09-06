.PHONY: build build-crawler build-migrator build-exporter build-api test test-acceptance test-fp vet fmt lint clean docker docker-build docker-compose-up docker-compose-down docker-compose-search search stats export run discover classify verify scan score migrate api web web-build web-dev

# ============================================================
# Build
# ============================================================

build: build-crawler build-migrator build-exporter build-api

build-crawler:
	go build -o bin/crawler ./cmd/crawler

build-migrator:
	go build -o bin/migrator ./cmd/migrate

build-exporter:
	go build -o bin/exporter ./cmd/export

build-api:
	go build -o bin/api ./cmd/api

# ============================================================
# Test
# ============================================================

test:
	go test ./... -count=1 -timeout=120s

test-acceptance:
	go test ./internal/engines/... -run TestAcceptance -v -count=1

test-fp:
	go test ./internal/engines/... -run TestFPRate -v -count=1

test-race:
	go test -race ./internal/... -count=1 -timeout=120s

# ============================================================
# Lint
# ============================================================

vet:
	go vet ./...

fmt:
	gofmt -s -w .

lint: vet fmt
	golangci-lint run 2>/dev/null || echo "golangci-lint not installed, skipping"

# ============================================================
# Clean
# ============================================================

clean:
	rm -rf bin/ data/registry.db registry/

# ============================================================
# Docker
# ============================================================

docker-build:
	docker build -t awesome-taiwan-ai-ecosystem .

docker-build-multi:
	docker buildx build \
		--target runtime-crawler -t awesome-taiwan-ai-ecosystem:crawler \
		--target runtime-api -t awesome-taiwan-ai-ecosystem:api \
		--target web -t awesome-taiwan-ai-ecosystem:web \
		.

docker-compose-up:
	docker compose up

docker-compose-down:
	docker compose down

# Search via docker-compose (requires ./data:/data/db mount)
docker-compose-search:
	docker compose run --rm crawler search $(filter-out $@,$(MAKECMDGOALS)) --db /data/db/registry.db

# ============================================================
# CLI shortcuts (require build first)
# ============================================================

run: build-crawler
	./bin/crawler run

discover: build-crawler
	./bin/crawler discover

classify: build-crawler
	./bin/crawler classify

verify: build-crawler
	./bin/crawler verify

scan: build-crawler
	./bin/crawler scan

score: build-crawler
	./bin/crawler score

migrate: build-crawler
	./bin/crawler migrate

export: build-crawler
	./bin/crawler export

search: build-crawler
	./bin/crawler search $(filter-out $@,$(MAKECMDGOALS))

stats: build-crawler
	./bin/crawler stats

# API server
api: build-api
	./bin/api --port 8080 --db ./data/registry.db

# Web UI
web: web-build

web-build:
	cd web && pnpm install && pnpm run build

web-dev:
	cd web && pnpm install && pnpm run dev

# Allow passing extra args to search (e.g., make search "taiwan" --level T3)
%:
	@:

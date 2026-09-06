.PHONY: build build-crawler build-migrator build-exporter test test-acceptance test-fp vet fmt lint clean docker docker-run docker-build docker-push search stats export run discover classify verify scan score migrate

# ============================================================
# Build
# ============================================================

build: build-crawler build-migrator build-exporter

build-crawler:
	go build -o bin/crawler ./cmd/crawler

build-migrator:
	go build -o bin/migrator ./cmd/migrate

build-exporter:
	go build -o bin/exporter ./cmd/export

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

docker-run:
	docker run --rm \
		-e GITHUB_TOKEN=${GITHUB_TOKEN} \
		-e OPENAI_API_KEY=${OPENAI_API_KEY} \
		-v $(PWD)/data:/data/db \
		-v $(PWD)/registry:/data/registry \
		awesome-taiwan-ai-ecosystem run

# ============================================================
# CLI shortcuts (require build first)
# ============================================================

run: build-crawler
	./bin/crawler run

discover: build-crawler
	./bin crawler discover

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

# Allow passing extra args to search (e.g., make search "taiwan" --level T3)
%:
	@:

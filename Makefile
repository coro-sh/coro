PROTO_FILES := $(wildcard proto/*)
BUF_VERSION := 1.54.0

.PHONY: unit-test
unit-test:
	go test ./... -race

.PHONY: integration-test
integration-test:
	go test -tags=integration ./... -race -count=1


# SQLC

.PHONY: sqlc-gen
sqlc-gen:
	go generate postgres/gen.go
	go generate sqlite/gen.go

# Postgres

.PHONY: start-postgres
start-postgres:
	docker run --name coro-postgres -e POSTGRES_PASSWORD=postgres -p 5432:5432 -d postgres:17
	go run cmd/pgtool/main.go --user postgres --password postgres init

.PHONY: stop-postgres
stop-postgres:
	@if docker ps -a --format '{{.Names}}' | grep -q '^coro-postgres$$'; then \
		docker stop coro-postgres >/dev/null 2>&1 || true; \
		docker rm -f coro-postgres >/dev/null 2>&1 || true; \
	fi

.PHONY: restart-postgres
restart-postgres: stop-postgres start-postgres

# Buf

.PHONY: buf-format
buf-format: $(PROTO_FILES)
	docker run -v $$(pwd):/srv -w /srv bufbuild/buf:$(BUF_VERSION) format -w

.PHONY: buf-lint
buf-lint: $(PROTO_FILES)
	docker run -v $$(pwd):/srv -w /srv bufbuild/buf:$(BUF_VERSION) lint

.PHONY: buf-gen
buf-gen: $(PROTO_FILES) buf-format buf-lint
	rm -rf **/gen/
	docker run -v $$(pwd):/srv -w /srv bufbuild/buf:$(BUF_VERSION) generate

# OpenAPI

.PHONY: oapi-client-gen
oapi-client-gen:
	go generate -run "oapi-codegen" ./client/oapicodegen/gen.go

# Dev Server

.PHONY: dev-server
# Usage: make dev-server CORS_ORIGINS="http://localhost:8080 http://localhost:5173"
dev-server:
	go run ./cmd/devserver -ui $(foreach origin,$(CORS_ORIGINS),-cors-origin $(origin))

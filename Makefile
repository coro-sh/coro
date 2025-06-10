PROTO_FILES := $(wildcard proto/*)
BUF_VERSION := 1.54.0

.PHONY:run
run:
	go run main.go --service all --config local/config_all.yaml

.PHONY: test
unit:
	go test ./... -race -count=1

# SQLC

.PHONY: sqlc-gen
sqlc-gen:
	go generate postgres/gen.go


# Postgres

.PHONY: start-postgres
start-postgres:
	docker run --name coro-postgres -e POSTGRES_PASSWORD=postgres -p 5432:5432 -d postgres:17
	go run cmd/pgtool/main.go --user postgres --password postgres init

.PHONY: stop-postgres
stop-postgres:
	docker stop coro-postgres && docker rm -f coro-postgres

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

.PHONY: client-gen
client-gen:
	go generate -run "oapi-codegen" ./client/oapicodegen/gen.go

# Dev Server

.PHONY: dev-server
# Usage: make dev-server CORS_ORIGINS="http://localhost:8080 http://localhost:5173"
dev-server:
	go run ./cmd/devserver $(foreach origin,$(CORS_ORIGINS),-cors-origin $(origin))

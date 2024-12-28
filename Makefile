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

start-postgres:
	docker run --name coro-postgres -e POSTGRES_PASSWORD=postgres -p 5432:5432 -d postgres:17
	go run cmd/pgtool/main.go --user postgres --password postgres init

stop-postgres:
	docker stop coro-postgres && docker rm -f coro-postgres
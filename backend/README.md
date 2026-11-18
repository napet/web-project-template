# Backend

Go API: Gin, Postgres via pgx, migrations with goose (embedded in the binary, applied on startup). Configuration and endpoints are in the [root README](../README.md).

## Run

```sh
cp .env.example .env   # fill in DATABASE_URL and ALLOWED_ORIGINS
go run .
```

```sh
curl localhost:8080/api/v1/health
curl -X POST -H "Origin: http://localhost:5173" localhost:8080/api/v1/visitors
curl -N localhost:8080/api/v1/visitors/stream
```

## Test

```sh
go vet ./...
go test ./...
```

The `counter` tests need a real Postgres: testcontainers starts one (Docker required), or set `TEST_DATABASE_URL` to an existing database. Everything else runs without external services.

## Migrations

Add `db/migrations/<YYYYMMDDHHMMSS>_<name>.sql` with goose's `-- +goose Up` and `-- +goose Down` sections. It is applied the next time the service starts.

## Container

```sh
docker build -t app-backend .
docker run --rm -p 8080:8080 -e DATABASE_URL=... -e ALLOWED_ORIGINS=... app-backend
```

The image is built from `scratch`: the static binary, the CA bundle (for `sslmode=verify-full`), `GIN_MODE=release`, and an unprivileged user.

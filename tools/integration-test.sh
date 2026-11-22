#!/usr/bin/env bash
#
# Runs the database integration tests (the TestIntegration* tests of each
# service's Prisma repository package) against a real Postgres.
#
# Usage:
#   tools/integration-test.sh                 # the four Prisma services
#   tools/integration-test.sh user-data auth  # some of them
#
#   services: auth | user-data | championship | race-data
#
# Postgres:
#   INTEGRATION_POSTGRES_URL set (CI): the server to use, without a database
#   name, e.g. postgresql://postgres:secret@localhost:5432.
#   Unset (local): a throwaway postgres:16.16-alpine container is started on a
#   random localhost port and removed at the end. Docker is required; the
#   Compose stack and its data are never touched.
#
# Each service gets its own database overdrive_<service>_test, reset by
# `db push --force-reset` before its tests. The tests refuse any database
# whose name does not end with _test.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ALL_SERVICES=(auth user-data championship race-data)
POSTGRES_IMAGE="postgres:16.16-alpine"

usage() {
	echo "usage: $0 [auth|user-data|championship|race-data ...]" >&2
	exit 2
}

services=("$@")
[[ ${#services[@]} -gt 0 ]] || services=("${ALL_SERVICES[@]}")
for service in "${services[@]}"; do
	[[ " ${ALL_SERVICES[*]} " == *" $service "* ]] || usage
done

container=""
cleanup() {
	if [[ -n "$container" ]]; then
		docker rm -f "$container" >/dev/null 2>&1 || true
	fi
}
trap cleanup EXIT

server_url="${INTEGRATION_POSTGRES_URL:-}"
if [[ -z "$server_url" ]]; then
	command -v docker >/dev/null || { echo "docker is required when INTEGRATION_POSTGRES_URL is not set" >&2; exit 1; }
	password="$(openssl rand -hex 16)"
	echo "Starting a throwaway $POSTGRES_IMAGE container..."
	container="$(docker run -d --rm -e POSTGRES_PASSWORD="$password" -p 127.0.0.1::5432 "$POSTGRES_IMAGE")"
	# The image first runs a temporary server on the Unix socket only: waiting on
	# TCP waits for the final server.
	for _ in $(seq 1 60); do
		if docker exec "$container" pg_isready -h 127.0.0.1 -U postgres >/dev/null 2>&1; then
			break
		fi
		sleep 1
	done
	docker exec "$container" pg_isready -h 127.0.0.1 -U postgres >/dev/null || { echo "Postgres did not start" >&2; exit 1; }
	port="$(docker port "$container" 5432/tcp | head -n 1 | sed 's/.*://')"
	server_url="postgresql://postgres:${password}@127.0.0.1:${port}"
fi
server_url="${server_url%/}"
# The script appends the database name: a server URL with a path or a query
# string would produce a wrong one.
if [[ "$server_url" == *\?* || "${server_url#*://*/}" != "$server_url" ]]; then
	echo "INTEGRATION_POSTGRES_URL must be scheme://user:password@host:port, without database nor query" >&2
	exit 2
fi

for service in "${services[@]}"; do
	dir="$ROOT_DIR/services/${service}-service"
	database="overdrive_${service//-/_}_test"
	url_var="$(printf '%s' "$service" | tr 'a-z-' 'A-Z_')_DATABASE_URL"
	database_url="${server_url}/${database}"

	echo "== $service ($database)"
	cd "$dir"
	if [[ ! -f resources/db/db_gen.go ]]; then
		go run github.com/steebchen/prisma-client-go generate --schema resources/schema.prisma
	fi
	env "$url_var=$database_url" go run github.com/steebchen/prisma-client-go db push \
		--schema resources/schema.prisma --skip-generate --force-reset
	output="$(mktemp)"
	status=0
	INTEGRATION_DATABASE_URL="$database_url" go test -race -count=1 -v -run '^TestIntegration' \
		./src/adapters/repository/prisma/ >"$output" 2>&1 || status=$?
	if [[ "$status" -ne 0 ]]; then
		cat "$output"
		rm -f "$output"
		exit "$status"
	fi
	grep -E '^(--- |ok)' "$output" || true
	# A renamed test or a missing URL must not pass silently.
	if grep -q -- '--- SKIP' "$output" || ! grep -q -- '--- PASS: TestIntegration' "$output"; then
		echo "$service: integration tests were skipped or none ran" >&2
		rm -f "$output"
		exit 1
	fi
	rm -f "$output"
done

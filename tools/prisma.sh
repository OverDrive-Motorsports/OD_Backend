#!/usr/bin/env bash
#
# Runs a Prisma CLI command on one service schema with the CLI embedded in
# prisma-client-go, so that every command uses the same Prisma version as the
# *-dbsync jobs that push the schemas.
#
# Studio is the exception: the embedded CLI is a single packaged binary that
# cannot start the generator Studio needs ("Cannot find module
# .../--max-old-space-size=8096"). Studio therefore runs the npm Prisma CLI
# through npx (Node.js required), pinned to the embedded CLI's version.
#
# Usage:
#   tools/prisma.sh <service> <command> [args...]
#   tools/prisma.sh all validate
#
#   <service>  auth | user-data | championship | race-data
#   <command>  any Prisma CLI command, e.g. validate, studio, format
#
# The database URL is <SERVICE>_DATABASE_URL when it is set (environment first,
# then the service .env). Otherwise it is built from POSTGRES_USER and
# POSTGRES_PASSWORD in the root .env for a Postgres reachable on localhost:5432.
# Without credentials a URL with no user is used: commands that never connect
# (validate, format) keep working, and Studio fails with Postgres' own error.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SERVICES=(auth user-data championship race-data)

usage() {
	echo "usage: $0 <auth|user-data|championship|race-data|all> <prisma command> [args...]" >&2
	exit 2
}

# read_env_value FILE KEY prints the value of KEY in a .env file, without quotes.
# The file is parsed, never sourced.
read_env_value() {
	local file="$1" key="$2" line value
	[[ -f "$file" ]] || return 0
	line="$(grep -E "^[[:space:]]*${key}[[:space:]]*=" "$file" | head -n 1 || true)"
	[[ -n "$line" ]] || return 0
	value="$(printf '%s\n' "${line#*=}" | sed -E 's/^[[:space:]]+//; s/[[:space:]]+$//')"
	case "$value" in
	\"*\") value="${value:1:${#value}-2}" ;;
	\'*\') value="${value:1:${#value}-2}" ;;
	*) value="$(printf '%s\n' "$value" | sed -E 's/[[:space:]]+#.*$//')" ;; # inline comment, as Compose
	esac
	printf '%s' "$value"
}

# urlencode STRING percent-encodes everything but unreserved URL characters.
urlencode() {
	local LC_ALL=C input="$1" output="" char i
	for ((i = 0; i < ${#input}; i++)); do
		char="${input:i:1}"
		case "$char" in
		[a-zA-Z0-9.~_-]) output+="$char" ;;
		*) output+="$(printf '%%%02X' "'$char")" ;;
		esac
	done
	printf '%s' "$output"
}

# run_studio runs Prisma Studio with the npm CLI at the embedded CLI's version.
run_studio() {
	local version
	if ! command -v npx >/dev/null 2>&1; then
		echo "Prisma Studio needs Node.js (npx), which is not installed." >&2
		exit 1
	fi
	version="$(go run github.com/steebchen/prisma-client-go version 2>/dev/null |
		sed -n -E 's/^prisma[[:space:]]+:[[:space:]]+([0-9][0-9A-Za-z.-]*).*/\1/p')"
	if [[ -z "$version" ]]; then
		echo "Unable to read the Prisma CLI version embedded in prisma-client-go." >&2
		exit 1
	fi
	npx --yes "prisma@${version}" "$@" --schema resources/schema.prisma
}

run_for_service() {
	local service="$1"
	shift
	local service_dir="$ROOT_DIR/services/${service}-service"
	local env_name database url user password
	env_name="$(echo "$service" | tr 'a-z-' 'A-Z_')_DATABASE_URL"
	database="overdrive_$(echo "$service" | tr '-' '_')"

	url="${!env_name:-}"
	if [[ -z "$url" ]]; then
		url="$(read_env_value "$service_dir/.env" "$env_name")"
	fi
	if [[ -z "$url" ]]; then
		user="${POSTGRES_USER:-$(read_env_value "$ROOT_DIR/.env" POSTGRES_USER)}"
		password="${POSTGRES_PASSWORD:-$(read_env_value "$ROOT_DIR/.env" POSTGRES_PASSWORD)}"
		if [[ -n "$user" && -n "$password" ]]; then
			url="postgresql://$(urlencode "$user"):$(urlencode "$password")@localhost:5432/${database}?schema=public"
		else
			url="postgresql://localhost:5432/${database}?schema=public"
		fi
	fi

	(
		cd "$service_dir"
		export "$env_name=$url"
		if [[ "$1" == "studio" ]]; then
			run_studio "$@"
		else
			go run github.com/steebchen/prisma-client-go "$@" --schema resources/schema.prisma
		fi
	)
}

[[ $# -ge 2 ]] || usage
target="$1"
shift

if [[ "$target" == "all" ]]; then
	for service in "${SERVICES[@]}"; do
		echo "== ${service}"
		run_for_service "$service" "$@"
	done
	exit 0
fi

for service in "${SERVICES[@]}"; do
	if [[ "$service" == "$target" ]]; then
		run_for_service "$service" "$@"
		exit 0
	fi
done
usage

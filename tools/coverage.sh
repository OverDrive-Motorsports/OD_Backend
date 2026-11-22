#!/usr/bin/env bash
#
# Runs the unit tests of Go modules with the race detector and coverage, and
# fails when a module's total coverage is below its minimum in
# tools/coverage-thresholds.txt.
#
# Usage:
#   tools/coverage.sh                      # every module of the thresholds file
#   tools/coverage.sh gateway shared/apierror
#
# Coverage counts every package of the module (-coverpkg), so tests living in
# another package of the module (shared/apierror/test) are measured too. The
# generated Prisma clients (resources/db) are excluded. The Go toolchain is the
# one named in the thresholds file (downloaded by the go command if needed),
# because totals differ between patch releases. In GitHub Actions the result is
# also added to the job summary.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
THRESHOLDS="$ROOT_DIR/tools/coverage-thresholds.txt"

# The minimums are only meaningful with the toolchain they were measured with.
toolchain="$(awk '$1 == "toolchain" { print $2 }' "$THRESHOLDS")"
if [[ -z "$toolchain" ]]; then
	echo "::error::tools/coverage-thresholds.txt has no toolchain line"
	exit 1
fi
export GOTOOLCHAIN="$toolchain"

# threshold MODULE prints the minimum coverage of MODULE, or nothing.
threshold() {
	awk -v module="$1" '$1 == module { print $2 }' "$THRESHOLDS"
}

modules=("$@")
if [[ ${#modules[@]} -eq 0 ]]; then
	mapfile -t modules < <(awk '!/^[[:space:]]*(#|$)/ && $1 != "toolchain" { print $1 }' "$THRESHOLDS")
fi

profile_dir="$(mktemp -d)"
trap 'rm -rf "$profile_dir"' EXIT

if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
	printf '| Module | Coverage | Minimum | Status |\n| --- | --- | --- | --- |\n' >>"$GITHUB_STEP_SUMMARY"
fi

failed=0
for module in "${modules[@]}"; do
	minimum="$(threshold "$module")"
	if [[ -z "$minimum" ]]; then
		echo "::error::$module has no minimum in tools/coverage-thresholds.txt"
		failed=1
		continue
	fi

	profile="$profile_dir/$(printf '%s' "$module" | tr '/' '_').out"
	cd "$ROOT_DIR/$module"
	packages="$(go list ./... | grep -v '/resources/db$' | paste -sd, -)"
	if ! go test ./... -race -coverpkg="$packages" -coverprofile="$profile"; then
		echo "::error::$module tests failed"
		failed=1
		continue
	fi
	total="$(go tool cover -func="$profile" | awk '/^total:/ { sub("%", "", $NF); print $NF }')"

	if awk -v total="$total" -v minimum="$minimum" 'BEGIN { exit !(total + 0 >= minimum + 0) }'; then
		status="ok"
	else
		status="below minimum"
		echo "::error::$module coverage ${total}% is below its minimum of ${minimum}%"
		failed=1
	fi
	echo "coverage $module: ${total}% (minimum ${minimum}%) $status"
	if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
		echo "| \`$module\` | ${total}% | ${minimum}% | $status |" >>"$GITHUB_STEP_SUMMARY"
	fi
done

exit "$failed"

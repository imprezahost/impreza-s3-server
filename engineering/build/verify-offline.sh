#!/usr/bin/env bash
# Offline verification. No provisioning, listener/service setup or credential acquisition.
# Usage: verify-offline.sh <checkout> <NEW-output> <go> <module-cache> <govulncheck> <local-vulndb-dir> <built-impreza-s3-server>
# Every step records its exit status; failure is reported after all checks, including the binary scan.
set -euo pipefail
SRC=${1:?}
OUT=${2:?}
GO=${3:?}
MODCACHE=${4:?}
VULN=${5:?}
DB=${6:?}
BIN=${7:?built impreza-s3-server binary}
[ -f "$BIN" ] && [ ! -L "$BIN" ] || {
	echo 'built binary required (regular file)' >&2
	exit 2
}
BIN=$(cd "$(dirname "$BIN")" && pwd -P)/$(basename "$BIN")
[ "${OUT:0:1}" = / ] && [ ! -e "$OUT" ] || {
	echo 'new absolute output required' >&2
	exit 2
}
SRC=$(cd "$SRC" && pwd -P)
MODCACHE=$(cd "$MODCACHE" && pwd -P)
DB=$(cd "$DB" && pwd -P)
GO=$(cd "$(dirname "$GO")" && pwd -P)/$(basename "$GO")
VULN=$(cd "$(dirname "$VULN")" && pwd -P)/$(basename "$VULN")
mkdir "$OUT"
OUT=$(cd "$OUT" && pwd -P)
export GOTOOLCHAIN=local GOENV=off GOWORK=off GOVCS='*:off' GOPROXY=off
export GOSUMDB=sum.golang.org GONOSUMDB= GOPRIVATE= GOINSECURE=
export GOMODCACHE="$MODCACHE" GOCACHE="$OUT/gocache" GOPATH="$OUT/gopath" GOFLAGS='-mod=readonly'
# Windows (Git Bash): Go needs mixed-style paths. Exported here, after the POSIX values, so it takes effect (inside the
# $(...) that resolved SRC it was lost with the subshell).
if command -v cygpath >/dev/null 2>&1; then export GOMODCACHE="$(cygpath -m "$MODCACHE")" GOCACHE="$(cygpath -m "$OUT/gocache")" GOPATH="$(cygpath -m "$OUT/gopath")"; fi
# govulncheck runs `go list` through PATH; keep the selected compiler first.
export PATH="$(dirname "$GO"):$PATH"
cd "$SRC"
step() {
	local name=$1
	shift
	set +e
	"$@" >"$OUT/$name.txt" 2>&1
	local rc=$?
	set -e
	echo "$name exit=$rc" >>"$OUT/steps.txt"
}
step mod-verify "$GO" mod verify
# This script does not acquire dependencies. Some tests may create local listeners;
# Use an isolated environment and record explicit skips separately from passing checks.
step internal-tests "$GO" test -short -count=1 -timeout=10m ./internal/...
step vet "$GO" vet ./...
step govulncheck-source "$VULN" -db "file://$DB" ./...
step govulncheck-binary "$VULN" -db "file://$DB" -mode=binary "$BIN"
(cd "$(dirname "$BIN")" && sha256sum "$(basename "$BIN")") >"$OUT/scanned-binary.sha256"
# Full cmd suite and live deployment checks require separate isolated runs; skipped checks are not passes.
cat "$OUT/steps.txt"
! grep -qv ' exit=0$' "$OUT/steps.txt"

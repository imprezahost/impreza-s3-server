#!/usr/bin/env bash
# Reproducible fork build with a source-bound identity.
# Usage: build.sh <own-checkout> <NEW-output-dir> <canonical-go> <own-module-cache>
set -euo pipefail
SRC=${1:?source}; OUT=${2:?new-output}; GO=${3:?canonical-go}; MODCACHE=${4:?own-module-cache}
[ ! -L "$SRC" ] && [ ! -L "$MODCACHE" ] || { echo 'source/cache link refused' >&2; exit 2; }
SRC=$(cd "$SRC" && pwd -P); MODCACHE=$(cd "$MODCACHE" && pwd -P)
GO=$(cd "$(dirname "$GO")" && pwd -P)/$(basename "$GO")
[ "${OUT:0:1}" = / ] && [ ! -e "$OUT" ] || { echo 'output must be absolute and absent' >&2; exit 2; }
[ -z "$(find -P "$SRC" -type l -print -quit)" ] || { echo 'source links refused' >&2; exit 2; }
ROOT=$(cd "$(git -C "$SRC" rev-parse --show-toplevel)" && pwd -P)
[ "$SRC" = "$ROOT" ] || { echo 'unexpected source directory' >&2; exit 2; }
git -C "$SRC" ls-files --error-unmatch go.mod go.sum >/dev/null
[ -z "$(git -C "$ROOT" status --porcelain --untracked-files=all -- .)" ] || { echo 'uncommitted source/build inputs refused' >&2; exit 2; }
(cd "$SRC" && sha256sum --check "$ROOT/engineering/build/dependency-lock.sha256")
COMMIT=$(git -C "$ROOT" rev-parse HEAD)
TOOLCHAIN=go1.26.10
export GOTOOLCHAIN=local GOENV=off GOWORK=off GOVCS='*:off'
export CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOAMD64=v1
export GOFLAGS='-mod=readonly -trimpath -buildvcs=false'
# All module acquisition/verification precedes this offline build.
export GOPROXY=off GOSUMDB=sum.golang.org GONOSUMDB= GOPRIVATE= GOINSECURE=
export GOMODCACHE="$MODCACHE"
if command -v cygpath >/dev/null 2>&1; then export GOMODCACHE="$(cygpath -m "$MODCACHE")"; fi
[ "$("$GO" env GOVERSION | tr -d '\r')" = "$TOOLCHAIN" ] || { echo 'canonical toolchain mismatch' >&2; exit 2; }
mkdir "$OUT"; OUT=$(cd "$OUT" && pwd -P)
export GOCACHE="$OUT/gocache" GOPATH="$OUT/gopath"
if command -v cygpath >/dev/null 2>&1; then export GOCACHE="$(cygpath -m "$OUT/gocache")" GOPATH="$(cygpath -m "$OUT/gopath")"; fi
LDFLAGS="-s -w -buildid="
# Provisional fork identity, derived from the exact source commit rather than wall clock.
FORK_VERSION="impreza-s3-server.${COMMIT:0:12}"
LDFLAGS+=" -X github.com/minio/minio/cmd.Version=$FORK_VERSION"
LDFLAGS+=" -X github.com/minio/minio/cmd.CopyrightYear=2025"
LDFLAGS+=" -X github.com/minio/minio/cmd.ReleaseTag=DEVELOPMENT"
LDFLAGS+=" -X github.com/minio/minio/cmd.CommitID=$COMMIT"
LDFLAGS+=" -X github.com/minio/minio/cmd.ShortCommitID=${COMMIT:0:12}"
LDFLAGS+=" -X github.com/minio/minio/cmd.GOPATH= -X github.com/minio/minio/cmd.GOROOT="
(cd "$SRC" && "$GO" mod verify >"$OUT/go-mod-verify.txt" && "$GO" build -tags kqueue -ldflags "$LDFLAGS" -o "$OUT/minio" .)
(cd "$OUT" && "$GO" version -m minio) >"$OUT/minio.buildinfo.txt"
(cd "$OUT" && sha256sum minio >minio.sha256)
{
 echo "source_commit=$COMMIT"
 echo 'upstream_commit=9e49d5e7a648f00e26f2246f4dc28e6b07f8c84a'
 echo "toolchain=$TOOLCHAIN"
 echo "go.mod.sha256=$(sha256sum "$SRC/go.mod" | cut -d' ' -f1)"
 echo "go.sum.sha256=$(sha256sum "$SRC/go.sum" | cut -d' ' -f1)"
 echo "ldflags=$LDFLAGS"
} >"$OUT/build-params.txt"
cat "$OUT/minio.sha256"

#!/usr/bin/env bash
set -euo pipefail
A=${1:?first-output}; B=${2:?second-output}
for item in minio minio.buildinfo.txt build-params.txt; do cmp -- "$A/$item" "$B/$item"; done
(cd "$A" && sha256sum --check minio.sha256)
(cd "$B" && sha256sum --check minio.sha256)
echo 'PASS two independent builds: identical bytes/buildinfo/parameters'

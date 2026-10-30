#!/usr/bin/env bash
set -euo pipefail
A=${1:?first-output}
B=${2:?second-output}
for item in impreza-s3-server impreza-s3-server.buildinfo.txt build-params.txt; do cmp -- "$A/$item" "$B/$item"; done
(cd "$A" && sha256sum --check impreza-s3-server.sha256)
(cd "$B" && sha256sum --check impreza-s3-server.sha256)
echo 'PASS two independent builds: identical bytes/buildinfo/parameters'

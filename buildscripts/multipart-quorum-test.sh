#!/usr/bin/env bash
# DISABLED: upstream test artifacts are not admitted in this fork.
# Coverage lost: multipart quorum test. See engineering/ci/README.md.
printf '%s\n' 'DISABLED: multipart quorum test; fork-owned fixtures required' >&2
exit 1

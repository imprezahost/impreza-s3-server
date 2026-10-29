#!/usr/bin/env bash
# DISABLED: upstream test artifacts are not admitted in this fork.
# Coverage lost: versioning tests. See engineering/ci/README.md.
printf '%s\n' 'DISABLED: versioning tests; fork-owned fixtures required' >&2
exit 1

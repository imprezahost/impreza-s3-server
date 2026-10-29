#!/usr/bin/env bash
# DISABLED: upstream test artifacts are not admitted in this fork.
# Coverage lost: pbac tests. See engineering/ci/README.md.
printf '%s\n' 'DISABLED: pbac tests; fork-owned fixtures required' >&2
exit 1

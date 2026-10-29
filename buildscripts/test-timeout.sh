#!/usr/bin/env bash
# DISABLED: upstream test artifacts are not admitted in this fork.
# Coverage lost: test timeout. See engineering/ci/README.md.
printf '%s\n' 'DISABLED: test timeout; fork-owned fixtures required' >&2
exit 1

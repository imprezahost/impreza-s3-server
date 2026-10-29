#!/usr/bin/env bash
# DISABLED: upstream test artifacts are not admitted in this fork.
# Coverage lost: disable root. See engineering/ci/README.md.
printf '%s\n' 'DISABLED: disable root; fork-owned fixtures required' >&2
exit 1

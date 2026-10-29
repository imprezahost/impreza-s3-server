#!/usr/bin/env bash
# DISABLED: upstream test artifacts are not admitted in this fork.
# Coverage lost: delete replication. See engineering/ci/README.md.
printf '%s\n' 'DISABLED: delete replication; fork-owned fixtures required' >&2
exit 1

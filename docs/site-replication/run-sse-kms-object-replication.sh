#!/usr/bin/env bash
# DISABLED: upstream test artifacts are not admitted in this fork.
# Coverage lost: run sse kms object replication. See engineering/ci/README.md.
printf '%s\n' 'DISABLED: run sse kms object replication; fork-owned fixtures required' >&2
exit 1

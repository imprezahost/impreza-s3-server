#!/usr/bin/env bash
# DISABLED: upstream test artifacts are not admitted in this fork.
# Coverage lost: decom encrypted sse s3. See engineering/ci/README.md.
printf '%s\n' 'DISABLED: decom encrypted sse s3; fork-owned fixtures required' >&2
exit 1

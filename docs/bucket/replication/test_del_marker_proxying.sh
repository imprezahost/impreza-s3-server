#!/usr/bin/env bash
# DISABLED: upstream test artifacts are not admitted in this fork.
# Coverage lost: test del marker proxying. See engineering/ci/README.md.
printf '%s\n' 'DISABLED: test del marker proxying; fork-owned fixtures required' >&2
exit 1

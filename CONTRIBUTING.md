# Contributing to the Impreza S3 server

This repository is a maintained fork: it tracks upstream and carries our
security, build and maintenance changes. Contributions in that scope are
welcome.

**Scope.** Security fixes and hardening, reproducible-build corrections
(toolchain, dependency lock, verification steps) and maintenance (CI, docs
that ship with a release). Feature requests and new functionality are out of
scope and will be closed without review — upstream is where features belong.

**Security reports never go through issues or pull requests.** Follow
[SECURITY.md](SECURITY.md) and report privately through the portal. A fix
for a vulnerability that is not yet public is a security report: send it
privately the same way, not as a pull request.

## Sending a change

1. Open an issue first describing the problem (unless it is a security
   report — see above).
2. Branch from `next`, keep the change minimal and described in the issue.
3. Send a pull request against `next` referencing that issue.

## Build and verification

Builds use the toolchain pinned in `go.mod` (Go 1.26.10) and the tracked
dependency lock; `make verify` must pass. A security change must add or
extend the test that fails without it, and reachability claims belong in the
issue, not in the commit message.

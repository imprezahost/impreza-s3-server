# Building Impreza S3 Server

Use Bash, Git, checksum tools, Go 1.26.10 and an existing verified module cache.
Prepare dependencies separately with the official module proxy and checksum database.
`build.sh` verifies the tracked lock and modules, then builds offline from a clean
checkout. It does not acquire a compiler or start the executable. The target is
Linux/amd64, GOAMD64=v1, CGO disabled, with kqueue tags, trimpath and empty build ID.

Run build.sh twice with separate, absent output directories and compare.sh on the
results, as shown in the root README. Keep the exact compiler, source commit and
module lock. Source commit IDs are embedded in the executable, so changing the
commit changes its identity and generally its checksum even for equivalent trees.
Build parameters and buildinfo are retained next to the executable, `impreza-s3-server`.

The `Version` string is `impreza-s3-server.<12-character source commit>` and
`ReleaseTag` is `DEVELOPMENT`, for `rN` releases too: the release tag is recorded
in the release, not in the executable. This intentionally identifies a source build:
the inherited update code classifies a non-RFC3339 Version as a source build and
uses executable modification time for release-time comparisons. Do not interpret
upstream update responses as authorization to replace this fork.

To isolate the effect of Version on bytes, `identity-proof.py` builds the same
clean commit three times: one control using the historical upstream Version,
and two fork builds. All other ldflags, including CommitID and ReleaseTag, must
be identical. It checks that the two fork executables match and that the control
executable differs. This controls a single metadata input; it does not compare
against a deployed binary or prove that different commits have identical code.
The control executable is an experiment and must not be distributed.

```sh
python3 -I -B engineering/build/identity-proof.py \
  --source /absolute/clean-checkout --go /absolute/go/bin/go \
  --module-cache /absolute/verified-cache --output /absolute/new-proof-directory
```

The proof does not execute any compiled binary. Run it in an isolated development
environment without credentials. Build outputs and logs remain local, outside the
checkout, and are not source distribution files. The scripts configure offline
module use; they are not a network or operating system sandbox.

Use verify-offline.sh only in an environment where the tests' local listeners are
allowed, with an admitted local scanner and database. Preserve failures and record
explicit skips; no scanner exit or local hash alone establishes security. Separate
build environments are needed to investigate independent reproduction.

CI selects Go from `go.mod`, whose `toolchain` directive pins Go 1.26.10.
See [CI coverage and verification](../ci/README.md) for the checks and gaps.

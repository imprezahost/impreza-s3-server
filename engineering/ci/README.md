# CI coverage and verification

All workflow files except `release.yml` accept pushes to `main` and pull requests targeting `main`; `release.yml`
runs only for an `rN` tag (below).
The Go workflows use `actions/setup-go` with `go-version-file: go.mod`; the
`toolchain go1.26.10` directive is the compiler source of truth. The `go` directive
remains the module language requirement, not a second CI version matrix.

## Checks to observe after publication

These are expected checks, not results of a GitHub run. Publishing and enabling
GitHub Actions are separate maintainer actions. Jobs execute only on disposable
GitHub-hosted Ubuntu runners, with a read-only contents token; checkout does not
persist that token. No signing key or repository secret is required. No workflow
publishes a binary, container, package or release: `release.yml` only leaves an
unsigned draft release, which the maintainer signs and publishes by hand.

| Workflow | Expected work / result |
| --- | --- |
| `go.yml` | Unit packages (`go test -v -tags kqueue,dev ./...`) remain enabled. Upstream-client functional verification and timeout integration are disabled; see losses below |
| `go-lint.yml` | golangci-lint v2.9.0 with the tracked v2 config, generation/lock check, build, unit tests and package-by-package race tests pass |
| `vulncheck.yml` | Source `govulncheck -show verbose ./...`; also runs daily at 05:23 UTC and by manual dispatch. The prior baseline scan (govulncheck v1.9.0, database of 2026-10-21) reported four reachable advisories in indirect Prometheus v0.303.0 and one `golang.org/x/crypto` module advisory not called. This tree selects Prometheus v0.311.3 and removes the legacy Console graph; a fresh source and executable scan is still required. The earlier report is historical evidence, not the result for this updated tree. Classify any red run as a dependency finding or CI failure using its actual diagnostics |
| `go-cross.yml` | Existing cross-compilation matrix passes; this does not promise release artifacts for every target |
| `go-healing.yml` | DISABLED before runner allocation: healing/inconsistent-version/root-disk/rewrite integration needs fork-owned client/fixture inputs |
| `go-resiliency.yml` | Resiliency tests pass |
| `replication.yaml` | DISABLED before runner allocation: decommission/ILM/PBAC/config/object and site replication/versioning/multipart integration needs fork-owned client/fixture inputs |
| `root-disable.yml` | DISABLED before runner allocation: root-lockdown integration needs fork-owned client/fixture inputs |
| `upgrade-ci-cd.yaml` | DISABLED before runner allocation: historical-format upgrade integration needs fork-owned client/fixture inputs |
| `shfmt.yml`, `typos.yml` | Existing shell formatting and spelling checks pass; ShellCheck remains disabled as in the inherited workflow |
| `depsreview.yaml` | Dependency review passes on PRs; skipped on pushes because the action needs a PR base/head comparison |
| `issues.yaml`, `lock.yml` | Explicitly skipped: inherited issue automation removed; no project assignment or automatic thread locking |

The binaries-only cleanup retires upstream executable/image acquisition from inherited integration rigs.
Affected entrypoints fail explicitly before running anything; their CI consumers are
commented out or disabled before runner allocation. The removed, unmaintained rigs
and every disabled entrypoint are listed below. A skipped rig is missing coverage,
not a PASS. Remaining unit/lint/build jobs are not certified green by this edit.
The inherited resiliency workflow is unchanged and has not been executed here.
The source scanner installs the exact official govulncheck v1.9.0 through
https://proxy.golang.org with sum.golang.org verification. It queries the live
https://vuln.go.dev database at execution time (`-db=https://vuln.go.dev`). The
database is deliberately not frozen: its contents, availability and findings can
change independently of this commit. Preserve scanner/Go versions, database
timestamp and findings for each run; a pinned scanner does not freeze the result.

## Release

`release.yml` runs only when an annotated `r<number>` tag is pushed.
- **Before anything builds:** it checks, through the API, that the tag points at a commit on `main`, that its message has
  one `Upstream-Base: <40-hex>` line, and which toolchain `go.mod` pins.
- **Then** it reuses `reproducible-build.yml` (two builds per architecture, byte comparison) and `vulncheck.yml`.
- **The draft:** a GitHub Release draft with `impreza-s3-server-linux-amd64`, `impreza-s3-server-linux-arm64`, `SHA256SUMS` and `BUILDINFO.txt`,
  and notes naming the source commit, the upstream base and the toolchain.

Its safety properties:
- it uses no repository secret, and the workflow grants no permission at the top;
- every job is read-only except the draft job (contents write, actions read), and its direct jobs run no action; the reused build/scan workflows pin their actions by commit;
- the draft job uses only the runner's `gh`, so no third-party or first-party action ever holds the write token;
- no `${{ }}` expression is placed inside a shell script.

Signing and publishing are the maintainer's, outside CI: `engineering/release/SIGNING.md`. While `vulncheck.yml` is red
(above), a tag stops before the draft.

`verify.cjs` checks all of this with one rule per property and a real negative control per rule (the rule run on a
mutated copy of the workflow).

## Explicit gaps

- The mixed IAM matrix (which pulled upstream `openldap`/`dex`/`etcd`
  images) was **removed with its workflow**: the fork builds and
  ships binaries only, so the container-based rig had no path back. The two
  IAM import jobs that required upstream `minio/minio-iam-testing` data were
  removed with it. Ordinary in-tree unit tests remain enabled.
- Mint depended on an upstream self-hosted runner, Docker images and host-wide
  pruning. The skipped entry, its scripts and the docker-compose
  scenario assets were **removed** with the Docker/Helm distribution: they
  pulled upstream `minio` images from a public registry. Multipart migration,
  compression/encryption, multiple pools and erasure Mint scenarios are
  uncovered until fork-owned, binary-only fixtures exist. Binary releases
  remain the current distribution scope.
- `.github/stale.yml` is removed. Issue triage/locking is left to humans. The
  upstream MinIO project and its personal access token are not used.
- The committed `root.key`/`root.cert` pair originated in the upstream test
  workflow in 2021, then pointed at the public `play.min.io:7373` KES endpoint.
  No consumer of these files remains in this tree. Both are removed, without
  rewriting history or printing their contents. Future TLS test consumers must
  generate a short-lived, job-local certificate and key; do not restore the
  static pair. No current test consumer needs certificate generation.

No S3 request handling, deployment default, operator flag or executable behavior
changes here, so no entry is added to SECURITY's operator-visible changes list.

## Local verification

Use an already admitted Node runtime and **js-yaml 4.x**. No install is performed
by the checker. If the dependency is cached outside this checkout, select it
with `NODE_PATH` using your platform's normal environment-variable syntax.

```sh
node engineering/ci/verify.cjs BASE_COMMIT
```

The checker parses all workflow/dependabot YAML (including duplicate-key
rejection), checks the used runner/step/permissions/matrix/setup-go shapes,
exercises main/master event dispatch against the base, and requires negative
controls for branch, push, Go version, runner, credentials and duplicate keys.
It also requires `engineering/build/dependency-lock.sha256` to match `go.mod` and
`go.sum`, because `engineering/build/build.sh` refuses to build otherwise; any edit
to `go.mod` (including its `toolchain` line) must update that lock in the same commit.
It does not evaluate the complete Actions expression language, lint embedded
shell or contact GitHub. Run an admitted `actionlint` on Linux as an additional
independent check when available.

Use an isolated Linux development environment, Go from `go.mod` and existing
verified module/tool caches for the product checks. Run and record each command
separately so a failure is retained, not converted into a pass:

```sh
MINIO_API_REQUESTS_MAX=10000 CGO_ENABLED=0 go test -v -tags kqueue,dev ./...
golangci-lint run --build-tags kqueue --timeout 10m --config .golangci.yml
govulncheck -show verbose ./...
```

Missing Linux, tools, modules, database or listener permissions is an environment
block, not a product assertion failure or a passing check. Windows Go results
cannot certify Linux packages or race/heavy integration tests. Local equivalents
do not certify hosted CI; observe the complete workflow list above after the
maintainer's curated publication.

## Dependency updates

Dependabot opens weekly grouped routine version PRs for Go modules and GitHub
Actions. Security updates are advisory-triggered independently of that schedule;
there is no cooldown, custom target branch or automatic merge. Only routine
version updates are grouped. This configuration requires the maintainer to enable
Dependabot alerts and security updates in the GitHub repository settings, with
`main` as the default branch. Configuration alone is not evidence of activation.
See [GitHub's security update documentation](https://docs.github.com/en/code-security/concepts/supply-chain-security/dependabot-security-updates)
and [Dependabot options](https://docs.github.com/en/code-security/reference/supply-chain-security/dependabot-options-reference).
The selected lint version introduced [Go 1.26 support](https://github.com/golangci/golangci-lint/blob/main/CHANGELOG.md#v290).

## Coverage lost and retired inputs

The inventory is `retired-upstream-tests.json`. Each row records what is lost; no removed test is counted as passing. Removed rigs have no maintained workflow consumer. Their old Make targets refuse explicitly. Disabled scripts retain a fail-closed entrypoint, with no fetch fallback.

The disabled functional matrix loses upstream-mc S3/API/configuration scenarios and timeout checks. The healing workflow also loses root-disk checks that depended on the shared client environment; it is disabled as a whole rather than pretending the remaining calls have that fixture. Replication loses multi-site/ILM/tiering/delete-marker/SSE-C/SSE-KMS/compression/IDP/versioning/multipart scenarios. Standalone LDAP/OIDC, IAM-import, LDAP-container and checksum-header labs are removed. Ordinary Go unit tests, lint, cross-compilation and reproducible-build jobs remain.

| Retired input | Disposition | Coverage lost |
| --- | --- | --- |
| `buildscripts/disable-root.sh` | disabled | disable root |
| `buildscripts/heal-inconsistent-versions.sh` | disabled | heal inconsistent versions |
| `buildscripts/minio-upgrade.sh` | disabled | minio upgrade |
| `buildscripts/multipart-quorum-test.sh` | disabled | multipart quorum test |
| `buildscripts/resolve-right-versions.sh` | disabled | resolve right versions |
| `buildscripts/rewrite-old-new.sh` | disabled | rewrite old new |
| `buildscripts/test-timeout.sh` | disabled | test timeout |
| `buildscripts/verify-build.sh` | disabled | verify build |
| `buildscripts/verify-healing-empty-erasure-set.sh` | disabled | verify healing empty erasure set |
| `buildscripts/verify-healing.sh` | disabled | verify healing |
| `docs/bucket/lifecycle/setup_ilm_transition.sh` | disabled | setup ilm transition |
| `docs/bucket/replication/delete-replication.sh` | disabled | delete replication |
| `docs/bucket/replication/setup_2site_existing_replication.sh` | disabled | setup 2site existing replication |
| `docs/bucket/replication/setup_3site_replication.sh` | disabled | setup 3site replication |
| `docs/bucket/replication/setup_ilm_expiry_replication.sh` | disabled | setup ilm expiry replication |
| `docs/bucket/replication/test_del_marker_proxying.sh` | disabled | test del marker proxying |
| `docs/bucket/versioning/versioning-tests.sh` | disabled | versioning tests |
| `docs/distributed/decom-compressed-sse-s3.sh` | disabled | decom compressed sse s3 |
| `docs/distributed/decom-encrypted-kes.sh` | disabled | decom encrypted kes |
| `docs/distributed/decom-encrypted-sse-s3.sh` | disabled | decom encrypted sse s3 |
| `docs/distributed/decom-encrypted.sh` | disabled | decom encrypted |
| `docs/distributed/decom.sh` | disabled | decom |
| `docs/distributed/distributed-from-config-file.sh` | disabled | distributed from config file |
| `docs/distributed/iam-import-with-missing-entities.sh` | removed | iam import with missing entities |
| `docs/distributed/iam-import-with-openid.sh` | removed | iam import with openid |
| `docs/iam/policies/pbac-tests.sh` | disabled | pbac tests |
| `docs/site-replication/ldap.yaml` | removed | ldap |
| `docs/site-replication/run-multi-site-ldap.sh` | removed | run multi site ldap |
| `docs/site-replication/run-multi-site-minio-idp.sh` | disabled | run multi site minio idp |
| `docs/site-replication/run-multi-site-oidc.sh` | removed | run multi site oidc |
| `docs/site-replication/run-replication-with-checksum-header.sh` | removed | run replication with checksum header |
| `docs/site-replication/run-sse-kms-object-replication.sh` | disabled | run sse kms object replication |
| `docs/site-replication/run-ssec-object-replication-with-compression.sh` | disabled | run ssec object replication with compression |
| `docs/site-replication/run-ssec-object-replication.sh` | disabled | run ssec object replication |

Manual setup changes: the tuning guide uses its tracked local configuration; the TLS guide requires a separately admitted local certificate generator. The public KES quick-start (shared upstream key/certificate download and public test service) is removed. Coverage lost: that public-KES experiment; no replacement service or credentials are provisioned. Historical attribution/reference links and static upstream illustrations are not executable acquisition paths. The disabled updater display string in `cmd/update.go` and its unit test remain source history; a separate change owns removal, and startup/in-place update paths are already disabled.

The checker now applies the same asset, Makefile, action-pin and executable-input rules to production and real mutated files. A returned Helm directory, a Makefile download target, a floating third-party action and the alternate download host must be refused. Mutating a shared rule into a no-op must make its negative control fail. No fake filesystem or literal `check(false)` stands in for those three controls. Temporary control trees are removed in `finally`.

Follow-up second-session checks also follow the two local reusable workflows: checkout, setup-go and artifact actions use
full commit pins with version comments. The pins were checked against official tag refs (and peeled commits for
annotated tags). The release validator refuses inherited job secrets and extra read scopes, and real mutated reusable
workflow files exercise floating-action refusal. This validates source structure, not execution of hosted jobs.
## Tool installation pins

`vulncheck.yml` uses `golang.org/x/vuln/cmd/govulncheck@v1.9.0`; the release reuses
that same workflow, so its scanner has the same pin and live database source.
The Go module proxy confirms v1.9.0 was published on 2026-10-11. The dormant/local
Makefile `getdeps` path no longer pipes a remote installer to a shell: it installs
`github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.9.0` from the official
Go proxy/checksum database, matching the version already selected by go-lint.yml.
No installer was executed for this edit.

The server go.mod has a `tool` section for msgp and stringer. Adding the scanner
there was considered but would couple a CI-only tool to the server dependency
graph. Explicit versioned installs keep that graph and its dependency lock
unchanged while Go's checksum database verifies the tool's module acquisition.
These tool modules are not added to the server go.sum; their install verification
is separate from the server's two-file lock.

`verify.cjs` applies `rules.toolInstalls` to every parsed workflow, including
release and both reusable children. It rejects latest anywhere in a workflow,
unversioned Go/npm/pip installation arguments, version ranges and a fetched
installer piped to sh/bash (including continued lines). Real mutations of the scan
workflow call that very rule; exact-version syntax controls are never installed.
`makeToolPins` also checks the resolved Makefile linter pin and install command.
This is a structural policy for explicit run scripts, not a full shell interpreter:
opaque helper programs, runner-image binaries and action internals require their
own supply-chain review. The inherited gh/jq binaries still come from the hosted
runner; this change does not claim an immutable runner image or alter draft-token
gates. Hosted workflows and actual installation remain NOT_RUN locally.

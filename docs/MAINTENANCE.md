# Security maintenance

Impreza S3 Server, a fork of MinIO (AGPLv3), accepts security, build and maintenance changes within its documented
scope. Source changes, local proof, independent review, signed release and
production rollout are separate states. A source commit is not evidence that a
running service has been updated. Private reports use the channel in
[SECURITY.md](../SECURITY.md).

From advisory to signed release, counted from an upstream security advisory:
- **critical or high severity:** a signed release within **14 days**. When no
  fix is possible within that window, SECURITY.md and the README carry an
  advisory with the mitigation for **every** configuration;
- **medium or low severity:** in the next release, within **45 days**.

Every release depends on the maintainer's offline signature; the clock above
is the policy this fork commits to, not a promise that a signature can be
produced while its holder is unavailable.

## Release procedure (rN tags)

The release workflow runs only for an **annotated** `r<number>` tag whose
message carries an `Upstream-Base: <40-hex>` line; the workflow checks that
the tag points at a commit on `main` and which toolchain `go.mod` pins. The
lightweight tags of the repository's early history do not satisfy it and are
kept as-is; the first real release is the next annotated tag. To release:

1. create the annotated tag on `main`, with the upstream base as its own
   message line: `git tag -a rN -m "rN" -m "Upstream-Base: <40-hex upstream commit>"`;
2. push the tag: the workflow builds twice per architecture, byte-compares,
   runs govulncheck and leaves an **unsigned draft** release;
3. rebuild the same commit locally with the recorded recipe and compare the
   bytes against the draft;
4. the maintainer signs offline (the signature never passes through CI);
5. publish the release with SHA256SUMS, and publish the signing key's
   fingerprint through the documentation and the onion mirror.

## Disabled CI coverage

The workflows disabled before runner allocation — healing, replication,
root-lockdown, historical-format upgrade and the resiliency rig — need
fork-owned client/fixture inputs that the upstream rigs (removed with the
container distribution) used to provide. A skipped workflow is missing
coverage, not a pass; each names what is lost in its own file.

## Sources and triage

Before a release, and when an advisory is reported, the maintainer checks:

- [minio/minio advisories](https://github.com/minio/minio/security/advisories)
  and the advisory pages of every `github.com/minio/*` dependency in the selected
  module graph; an archived repository's silence does not establish safety.
- [NVD](https://nvd.nist.gov/vuln/search) and the
  [CVE records](https://github.com/CVEProject/cvelistV5), reconciling identifiers,
  affected versions, withdrawn records and the original report.
- [Go vulnerability reports](https://pkg.go.dev/vuln/) and current
  [govulncheck](https://go.dev/security/vuln/) source and built-binary scans.
- [Go release notes](https://go.dev/doc/devel/release) and security fixes in the
  compiler/standard library used to build the release.

Record the exact source/module/toolchain version, advisory URL and revision,
scanner and database timestamp, selected build tags, affected entry point,
reachability, required attacker permissions, data impact and a synthetic
reproduction. Keep a module-only finding separate from a reachable call trace.
Uncertainty stays open; a version comparison alone does not dismiss a finding.
Do not put credentials, production endpoints, customer data or exploit secrets
in public issues or commits. Recheck a reachability dismissal when tags,
configuration, imports or dependencies change.

A backport is reviewed line by line against its original source. Its commit
names the upstream repository, full source commit and advisory, explains any
adaptation, and carries a behavioral regression test with a failing control on
the prior version. Preserve attribution and licensing. An independent reviewer
checks the real caller, permissions, untrusted input, network destination,
replay/concurrency and storage failures as applicable. Do not copy a whole
history or accept a helper test as proof of its integration.

## Console boundary and current dependency work

The legacy Console is removed entirely, including its opt-in source variant.
`MINIO_BROWSER=on` cannot enable a browser server. Neither ordinary builds nor
an old `minio_console` tag can restore the removed implementation.
The [published CVE-2026-113466 CNA record](https://github.com/CVEProject/cvelistV5/blob/main/cves/2026/113xxx/CVE-2026-113466.json)
exists and describes the embedded Console session authentication/key derivation.
Disabling a setting in an old binary is different from removing code from a
newly built binary. See [SECURITY.md](../SECURITY.md).

The prior dependency baseline had Prometheus v0.303.0. The four Go advisories
[GO-2026-5710](https://pkg.go.dev/vuln/GO-2026-5710),
[GO-2026-5662](https://pkg.go.dev/vuln/GO-2026-5662),
[GO-2026-5381](https://pkg.go.dev/vuln/GO-2026-5381) and
[GO-2026-5264](https://pkg.go.dev/vuln/GO-2026-5264) are covered by the selected v0.311.3 update. Source and executable scans still
need to judge the complete selected graph; a version change is not a clean-scan
claim. A future scan may identify additional affected modules.

The self-updater implementation is removed from the server. Legacy protocol
entry points refuse downloading, reading/staging and committing executable
bytes, even if called without the higher-level disabled-update flag. The module graph must also be checked after `go mod tidy` for a transitive
re-entry. The legacy Console/mc import chain is removed rather than retaining
the updater indirectly in an opt-in variant. Never restore runtime update checks or
substitution of the executable as part of dependency maintenance.

## MinIO dependency inventory

Public repository pages were inspected on 2026-10-28. The observed archive
banner is an observation of repository state, not a support contract or a
complete security review. No archive banner observed does **not** mean maintained,
safe or current. Recheck the exact selected graph at each release. Nested major
versions use the same repository page.

| Repository / modules | Observed state | Handling within this maintenance scope |
| --- | --- | --- |
| [minio/minio](https://github.com/minio/minio) (source baseline) | Archived 2026-04-25 | Maintain this fork with reviewed security backports and independent entry-point tests; do not rely on upstream releases |
| [minio/mc](https://github.com/minio/mc) | Archived 2026-07-14 | Remove the legacy Console/mc path and verify that this module is absent after tidy |
| [minio/console](https://github.com/minio/console) | Page returned unavailable (404); archive status not independently established | Remove the component and opt-in source variant; do not restore it through a build tag |
| [minio/selfupdate](https://github.com/minio/selfupdate) | No archive banner observed | Remove the updater implementation and Console/mc path; verify no selected module or import remains after tidy |
| [minio/cli](https://github.com/minio/cli) | No archive banner observed | Retain CLI compatibility; review command parsing and negative input tests before a version change |
| [minio/csvparser](https://github.com/minio/csvparser) | No archive banner observed | Retain reviewed parser; keep record-size and malformed-input regression tests |
| [minio/dnscache](https://github.com/minio/dnscache) | No archive banner observed | Retain with DNS/timeout/destination review on updates |
| [minio/dperf](https://github.com/minio/dperf) | No archive banner observed | Audit diagnostic-only reachability and storage effects before updates |
| [minio/highwayhash](https://github.com/minio/highwayhash) | No archive banner observed | Preserve architecture vectors and hash compatibility when updating |
| [minio/kms-go](https://github.com/minio/kms-go) (`kes`, `kms`) | No archive banner observed | Review authentication/TLS/key handling and KMS compatibility; no credential acquisition in dependency tests |
| [minio/madmin-go/v3](https://github.com/minio/madmin-go) | No archive banner observed | Review admin authorization, protocol and transitive dependencies before version changes |
| [minio/minio-go/v7](https://github.com/minio/minio-go) | No archive banner observed | Preserve S3 signing, TLS and request compatibility tests |
| [minio/mux](https://github.com/minio/mux) | No archive banner observed | Review route matching and negative authorization paths |
| [minio/pkg/v3](https://github.com/minio/pkg) | No archive banner observed | Review affected shared utilities and real call sites, not just helper tests |
| [minio/simdjson-go](https://github.com/minio/simdjson-go) | No archive banner observed | Retain parser limits, malformed-input and architecture tests |
| [minio/sio](https://github.com/minio/sio) | No archive banner observed | Retain encrypted-format compatibility and corruption tests; require specialist review for cryptographic changes |
| [minio/xxml](https://github.com/minio/xxml) | No archive banner observed | Retain XML size/malformed-input boundary tests |
| [minio/zipindex](https://github.com/minio/zipindex) | No archive banner observed | Retain archive/path/allocation boundary tests |
| [minio/colorjson](https://github.com/minio/colorjson) | No archive banner observed | Audit whether complete Console removal drops it from the graph; review any retained parser consumers |
| [minio/crc64nvme](https://github.com/minio/crc64nvme) | No archive banner observed | Preserve checksum vectors and architecture tests |
| [minio/filepath](https://github.com/minio/filepath) | No archive banner observed | Audit selected reachability and path traversal semantics |
| [minio/md5-simd](https://github.com/minio/md5-simd) | No archive banner observed | Preserve signing/checksum compatibility; do not treat MD5 as a security signature |
| [minio/websocket](https://github.com/minio/websocket) | Page unavailable to this read surface; state unverified | Verify whether removal of the legacy Console drops it from the graph; review any retained message/auth consumers |

This inventory is a component maintenance policy, not a second priority backlog.
Record newly discovered defects in the project's existing plan and keep evidence
with the applicable maintenance item.

## Validation and release gates

After any module change, run `go mod tidy` with an admitted, verified module
source/cache. Update `engineering/build/dependency-lock.sha256` for **both**
`go.mod` and `go.sum` in the same commit. Do not invent checksums or accept a
partial network/tidy result as a finished graph. Run the CI checker in
[engineering/ci](../engineering/ci/README.md).

Run the affected `cmd` tests, including `TestConsoleDefaultOff` and
`TestInPlaceUpdateMechanicsDisabled`. Confirm that even an obsolete
`minio_console` tag cannot enable the removed component. Run the independent source-selection and
isolated implementation check with an admitted Go 1.26.10 runtime:

```sh
go run engineering/maintenance/verify.go
```

That offline check executes the actual extracted updater functions and the
real Go build-tag selector. It is **not** a full server build,
a binary-symbol proof, a transitive-graph proof or an integration test. The
full package tests remain necessary.

Build the same clean commit twice using `engineering/build/build.sh` and compare
with `compare.sh`. Release evidence requires matching SHA-256 on two independent
hosts, not just repeated runs against one cache. Inspect default `go version -m`
and unstripped `go tool nm` output for absence of `github.com/minio/console`,
`github.com/minio/selfupdate` and their reachable implementations. The canonical
stripped binary alone does not provide a symbol table; retain the matching
unstripped diagnostic build separately and never distribute it as the release.
Scan source and the actual standard executable; classify every remaining finding. Preserve errors, environment blocks and
explicit skips. No scanner result alone proves absence of vulnerabilities.

Final independent review, maintainer approval, curated public publication,
out-of-CI signature and production rollout remain separate gates. Rollback is
replacement with the previously verified binary and its matching configuration;
no automatic fleet update or runtime download is authorized by this policy.
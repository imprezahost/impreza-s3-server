# Security policy

This policy covers Impreza S3 Server, a fork of MinIO (AGPLv3).

## Report a vulnerability privately

**Reporting channel: email <support@imprezahost.com>** — preferably
[encrypted](#encrypt-a-report-optional) to the support key below. A support
ticket at https://portal.imprezahost.com/submitticket.php (also reachable
over its onion address) is an alternative; mark it as a security report.

Do not post exploit details or sensitive information in public issues.

A useful report includes the source commit, build toolchain, affected configuration,
expected behavior and a minimal reproduction with synthetic data. Omit credentials,
access keys, customer data, private endpoints and identifying infrastructure details.
The maintainer will arrange any sensitive evidence through the selected channel.

## Encrypt a report (optional)

Reports to <support@imprezahost.com> can be encrypted to the Impreza
support key — an OpenPGP key generated offline by the maintainer and kept
apart from service and release-signing keys.

- User ID: `Impreza Support <support@imprezahost.com>`
- Primary key: Ed25519 (certifies and signs); encryption subkey: Cv25519
- Fingerprint: `FD4A F833 BBA6 D9B8 B890  B1AE 8BF2 4F8D E8F9 50F9`
- Expires: 2029-10-29
- Public key block: [SECURITY-KEY.asc](SECURITY-KEY.asc) in this repository

**Verify the fingerprint through two independent channels before
trusting it:** compare the fingerprint above with the copy served at
`https://docs.imprezahost.com/.well-known/security-key.asc`. Only encrypt
when both match.

Encrypt with GnuPG and send the ASCII-armored result by mail:

```sh
gpg --import SECURITY-KEY.asc
gpg --armor --encrypt --recipient FD4AF833BBA6D9B8B890B1AE8BF24F8DE8F950F9 report.txt
```

The key is not published to third-party keyservers.

## Handling and updates

The maintainer will assess impact, reproduce reports in isolation and coordinate
fixes and disclosure with reporters. Response and release expectations follow the
[maintenance policy](docs/MAINTENANCE.md): a signed release within 14 days for
critical or high severity (or a published advisory with the mitigation), and
within 45 days for medium or low. Every release depends on the maintainer's
offline signature. Availability of source or a reproducible build does not mean
a report has been resolved.

Use the `rN` releases, checksums and signatures announced by this fork's maintainer. Upstream update
feeds do not identify this fork. The build recipe requires Go 1.26.10 and the
tracked dependency lock. Check both source and executable with a current,
independently obtained vulnerability database, recording its timestamp.
Retain all findings and assess reachability; no clean-scanner claim is made here.

## Legacy Console removed

The legacy browser Console and its source integration are removed. The standard
release contains no Console code, and there is no legacy opt-in variant.
`MINIO_BROWSER=on` logs a warning and leaves the browser disabled; S3 API access
remains available. Publication and rollout of this source change are separate
maintainer actions.

[CVE-2026-113466](https://www.cve.org/CVERecord?id=CVE-2026-113466) exists: its
[published CNA record](https://github.com/CVEProject/cvelistV5/blob/main/cves/2026/113xxx/CVE-2026-113466.json)
identifies the embedded Console's session authentication/key derivation.
Removing the component is not a cryptographic repair of the old implementation.
It does not invalidate sessions or alter an already-running old binary.

See [maintenance and dependency policy](docs/MAINTENANCE.md) for triage and release
gates. Do not publish a variant as fixed based only on a module version or scan.

## Server vulnerabilities fixed in this fork

These fixes are backported from [pgsty/silo](https://github.com/pgsty/silo), a
maintained AGPL-3.0 fork of the same upstream, and reviewed line by line before
inclusion. Each one is a separate commit that names the silo commit it came from.

| Advisory | Component | Status in this fork |
| --- | --- | --- |
| [CVE-2026-39414](https://github.com/minio/minio/security/advisories/GHSA-h749-fxx7-pwpg) | S3 Select record size | Fixed: CSV and JSON Lines records over 1 MiB are refused with `OverMaxRecordSize` |
| [CVE-2026-41145](https://github.com/minio/minio/security/advisories/GHSA-hv4r-mvr4-25vw) | Unsigned-trailer PUT and multipart | Fixed: query-string credentials are refused for this upload mode; header signatures are verified before the body is read |
| [CVE-2026-40344](https://github.com/minio/minio/security/advisories/GHSA-9c4q-hq6p-c237) | Snowball (`PutObjectExtract`) | Fixed: unsigned-trailer uploads are verified before extraction |
| [CVE-2026-42600](https://github.com/minio/minio/security/advisories/GHSA-xh8f-g2qw-gcm7) | Internode `ReadMultiple` | Fixed: the endpoint is removed, and the remaining internode storage-REST and Grid payloads are validated (path traversal, malformed erasure metadata, allocations sized from declared values, unusable erasure metadata refused at every sink; silo SN-2026-002). Distributed deployments should still keep internode traffic on a private network |
| [CVE-2026-97731](https://nvd.nist.gov/vuln/detail/CVE-2026-97731) | SigV4 signed headers / `CopyObject` | Fixed: any unsigned `x-amz-*` request header is refused, and a repeated `x-amz-copy-source` is refused |
| [CVE-2026-33322](https://github.com/minio/minio/security/advisories/GHSA-5cx5-wh4m-82fh) | OIDC STS | Fixed: HMAC-signed tokens are no longer accepted |
| [CVE-2026-33419](https://github.com/minio/minio/security/advisories/GHSA-jv87-32hw-hh99) | LDAP STS | Fixed: unknown users and wrong passwords get the same answer; logins are throttled per source address |
| [CVE-2026-34204](https://github.com/minio/minio/security/advisories/GHSA-3rh2-v3gr-35p9) | Replication metadata headers | Fixed: internal replication headers (source mtime, ETag, delete marker, retention, legal hold and tagging timestamps, SSE-C replication keys) are honoured only after the request is authorized for `s3:ReplicateObject` or `s3:ReplicateDelete`, across GET, HEAD, `GetObjectAttributes`, PUT, copy, Snowball, multipart and DELETE; otherwise they are stripped before options, metadata and event parameters are built (silo SN-2026-008) |
| [CVE-2026-113466](https://nvd.nist.gov/vuln/detail/CVE-2026-113466) | Console | **Legacy component removed** from this source; no opt-in variant (see above) |

### Other security fixes in this fork

These have no CVE identifier. They are fixed here and listed so operators can check their policies.

| Area | What changed |
| --- | --- |
| Object Lock default retention | A PUT, copy or multipart upload that carries `x-amz-object-lock-legal-hold` (`ON` or `OFF`) now inherits the bucket's default retention. Before, any legal-hold header suppressed it, so a caller allowed `s3:PutObjectLegalHold` could store an object outside the bucket's mandatory retention (from silo `9a303f5096c572c74efaca605f50d3c511e9885b`) |
| `PutObjectRetention` authorization | `s3:PutObjectRetention` is required and is evaluated from the requested retention before any object state is read; `s3:BypassGovernanceRetention` alone no longer sets retention, and is required in addition (with the bypass header) to shorten, remove or change unexpired GOVERNANCE retention. Clearing retention counts as 0 remaining days for `s3:object-lock-remaining-retention-days` (it was the distance to year 0001) (from silo `3196480da93ebe7583677e64848cc5afa4dada18`, SN-2026-015 part) |
| Writes that name a version | A version whose lock state cannot be read is no longer treated as unlocked: the write fails with the read error. An unexpired GOVERNANCE version now blocks the write like a COMPLIANCE one |
| Update check | The server no longer contacts `dl.min.io` at startup, and in-place update (`mc admin update`) is removed. Both revealed the server address, version, OS and architecture to a third party |
| Source address | `X-Forwarded-For`, `X-Real-IP` and `Forwarded` are ignored by default (see below) |

### Changes you may notice after upgrading

- **No official container image or Helm chart**: this fork builds and
  distributes signed **binaries only**; the upstream `Dockerfile`s, image
  publication and the Helm chart were removed. Run it from the binary in the
  signed release, or containerize it yourself with your own image review.
- **Browser Console**: the legacy implementation is removed, including its
  opt-in variant. `MINIO_BROWSER=on` cannot enable it.

- **OIDC**: identity providers that sign STS tokens with `HS256`, `HS384` or `HS512`
  must move to JWKS-backed RSA or ECDSA keys.
- **SigV4**: requests must sign every `x-amz-*` header they send (all AWS SDKs,
  `minio-go` and `mc` already do). An unsigned one is refused with `AccessDenied`.
- **Unsigned-trailer uploads** (`STREAMING-UNSIGNED-PAYLOAD-TRAILER`) no longer
  accept presigned query-string credentials.
- **LDAP STS**: about 10 failed logins per minute per source address, in memory on
  each node; `429 ThrottlingException` when exceeded. Behind a reverse proxy, set
  `MINIO_IDENTITY_LDAP_STS_TRUSTED_PROXIES` to the proxy addresses; see
  `docs/sts/ldap.md`.
- **S3 Select**: records over 1 MiB fail with `OverMaxRecordSize`.
- **Replication headers**: a request that sends `X-Amz-Replication-Status: REPLICA`
  without the replication permission is refused with `AccessDenied`; other internal
  replication headers from such callers are ignored. `GetObjectAttributes` on an
  SSE-C object now requires the customer key, as `HeadObject` does.
- **Policy conditions**: request headers and query parameters can no longer
  supply condition keys the server derives itself (identity, LDAP and JWT
  claims, time, transport, signature age, object lock, existing object tags).
  Header-only keys such as `s3:x-amz-content-sha256` ignore query parameters, and
  `ExistingObjectTag` conditions read the stored tags only. Policies that relied
  on a client-supplied value for one of these keys stop matching.
- **Source address (`aws:SourceIp`, audit and logs)**: `X-Forwarded-For`,
  `X-Real-IP` and `Forwarded` are **ignored by default**; the address is the
  connection's. **Incompatible behind a reverse proxy or load balancer**: there
  every request would come from the proxy's address, so `IpAddress` conditions,
  audit records and logs would see the proxy. To use the client address
  again, set `_MINIO_API_XFF_HEADER=on` **only if every request reaches the
  server through a proxy that overwrites all three headers**; there is no
  trusted-proxy list, and with it on any client that reaches the server directly
  chooses its address. Do not turn it on for an onion service: Tor clients
  arrive over loopback and could set any address. LDAP STS throttling keeps its
  own `MINIO_IDENTITY_LDAP_STS_TRUSTED_PROXIES` list. SFTP requests are now
  recorded with the server's own address instead of the SFTP client's.
- **Updates**: `MINIO_UPDATE` is ignored and `mc admin update` answers
  `MethodNotAllowed`. Update by replacing the binary.
- **Object Lock**: a PUT with a legal-hold header now gets the bucket's default
  retention; `s3:BypassGovernanceRetention` without `s3:PutObjectRetention` no
  longer sets retention; a write that names a version fails if that version's
  lock state cannot be read or it is under unexpired GOVERNANCE retention.

This list is not a claim that the fork is free of vulnerabilities. Scan your own
build as described above.

## Scope

This policy covers the maintained fork and its build tooling. Deployment security
also depends on authentication, authorization, TLS, encryption configuration,
network access and backups chosen by the operator. Do not use a production system
or another person's data to demonstrate a vulnerability.

License and attribution notices are preserved in LICENSE, NOTICE and the source.

# Security policy

## Report a vulnerability privately

**Reporting channel: open a support ticket at https://portal.imprezahost.com/submitticket.php**
(the portal is also reachable over its onion address). Mark it as a security report.

Do not post exploit details or sensitive information in public issues.

A useful report includes the source commit, build toolchain, affected configuration,
expected behavior and a minimal reproduction with synthetic data. Omit credentials,
access keys, customer data, private endpoints and identifying infrastructure details.
The maintainer will arrange any sensitive evidence through the selected channel.

## Handling and updates

The maintainer will assess impact, reproduce reports in isolation and coordinate
fixes and disclosure with reporters. No response deadline or support window is
promised before the public maintenance policy is announced. Availability of source
or a reproducible build does not mean a report has been resolved.

Use releases and checksums announced by this fork's maintainer. Upstream update
feeds do not identify this fork. The build recipe requires Go 1.26.10 and the
tracked dependency lock. Check both source and executable with a current,
independently obtained vulnerability database, recording its timestamp.
Retain all findings and assess reachability; no clean-scanner claim is made here.

## Console disabled by default

The browser Console is disabled by default as a mitigation for
[CVE-2026-113466](https://nvd.nist.gov/vuln/detail/CVE-2026-113466).
S3 API access remains available. An explicit `MINIO_BROWSER=on` enables the
Console and prints a security warning; it restores exposure to the unresolved
vulnerability. Existing deployments that explicitly enable the Console must
remove that setting or set `MINIO_BROWSER=off` to use this mitigation.

This is a default-off mitigation, not a repair of the Console session encryption.
If you opt in, restrict Console network access and use strong, unique root
credentials. Previously issued Console sessions are not invalidated by this
change. Deployment and public release remain separate operator actions.

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
| [CVE-2026-113466](https://nvd.nist.gov/vuln/detail/CVE-2026-113466) | Console | **Mitigated, not fixed**: see "Console disabled by default" above |

### Changes you may notice after upgrading

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
- **`aws:SourceIp`** is computed from `X-Forwarded-For`, `X-Real-IP` and
  `Forwarded` when present, and those headers are trusted from any peer. On a
  directly reachable server, an `IpAddress` condition is not enforceable: put the
  server behind a proxy that overwrites these headers, or set
  `_MINIO_API_XFF_HEADER=off` and strip the other two at the edge.

This list is not a claim that the fork is free of vulnerabilities. Scan your own
build as described above.

## Scope

This policy covers the maintained fork and its build tooling. Deployment security
also depends on authentication, authorization, TLS, encryption configuration,
network access and backups chosen by the operator. Do not use a production system
or another person's data to demonstrate a vulnerability.

License and attribution notices are preserved in LICENSE, NOTICE and the source.

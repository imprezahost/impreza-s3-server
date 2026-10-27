# impreza-s3-server

A security maintenance fork for operators who need self-hosted, S3-compatible
object storage with a pinned Go toolchain and reproducible build instructions.
The project name is provisional.

[![License: AGPLv3](https://img.shields.io/badge/license-AGPLv3-blue)](LICENSE)
[![Documentation](https://img.shields.io/badge/docs-Impreza%20Host-blue)](https://docs.imprezahost.com/)

## What it does

- Provides an S3-compatible storage server for infrastructure you control.
- Maintains dependency locks and a supported Go toolchain for security updates.
- Provides offline build and byte comparison scripts.
- Preserves upstream source attribution and third-party license notices.

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

## Security fixes

This source includes backported fixes for CVE-2026-39414 (S3 Select),
CVE-2026-41145 (unsigned-trailer uploads), CVE-2026-40344 (Snowball),
CVE-2026-42600 (internode storage), CVE-2026-97731 (SigV4 unsigned headers),
CVE-2026-33322 (OIDC), CVE-2026-33419 (LDAP) and CVE-2026-34204 (replication
metadata). CVE-2026-113466 (Console) is mitigated by the default-off Console,
not fixed. It also fixes Object Lock default retention and retention
authorization, never checks for updates, and ignores `X-Forwarded-For`,
`X-Real-IP` and `Forwarded` by default (behind a proxy that overwrites them, set
`_MINIO_API_XFF_HEADER=on`). Some changes are visible to clients and
identity providers; read [SECURITY.md](SECURITY.md) before upgrading.

## Quick start: build and compare

Install Go 1.26.10 and prepare
an isolated development environment first. These commands compile locally;
they do not install or start a storage service.

```sh
git clone https://github.com/imprezahost/impreza-s3-server.git impreza-s3-server
cd impreza-s3-server
S3_GO="$(command -v go)"
S3_CACHE="$PWD/../s3-module-cache"
GOTOOLCHAIN=local GOMODCACHE="$S3_CACHE" GOPROXY=https://proxy.golang.org GOSUMDB=sum.golang.org "$S3_GO" mod download
bash engineering/build/build.sh "$PWD" "$PWD/../s3-build-a" "$S3_GO" "$S3_CACHE"
bash engineering/build/build.sh "$PWD" "$PWD/../s3-build-b" "$S3_GO" "$S3_CACHE"
bash engineering/build/compare.sh "$PWD/../s3-build-a" "$PWD/../s3-build-b"
(cd ../s3-build-a && sha256sum --check minio.sha256)
```

Each output directory must be new and outside the checkout. Builds use the
same clean source commit, dependency lock and toolchain, with separate build
caches. The executable retains the historical filename `minio` for compatibility;
its build metadata identifies this fork. See [build instructions](engineering/build/BUILDING.md).

A locally generated checksum detects changes relative to that build. To verify
a downloaded release, obtain the expected checksum and source commit independently
from the maintainer; a checksum accompanying an untrusted file does not authenticate it.

## How it works and what it protects

Storage, identity, encryption and network access remain under the operator's
configuration. Dependency maintenance and reproducible builds do not establish
that a deployment is secure or free of vulnerabilities. Review [security policy](SECURITY.md)
and the [security and privacy documentation](https://docs.imprezahost.com/)
before deployment. This fork is not an upstream release; use only update artifacts
and instructions supplied by this project's maintainer.

## Documentation

[Impreza Host documentation](https://docs.imprezahost.com/) is the canonical entry
point. The source tree also includes inherited technical documentation under `docs/`.

## Support and security

The maintainer will select the public support and private security-reporting
channels before publication. Until then, no reporting address is advertised.
Do not put credentials, access keys, customer data or exploit details in public issues.
See [SECURITY.md](SECURITY.md).

## License and attribution

GNU Affero General Public License, version 3; see [LICENSE](LICENSE) and [NOTICE](NOTICE).
This fork derives from MinIO at commit
[`9e49d5e7a648f00e26f2246f4dc28e6b07f8c84a`](https://github.com/minio/minio/commit/9e49d5e7a648f00e26f2246f4dc28e6b07f8c84a).
MinIO is named here for attribution; this fork is independently maintained and
is not endorsed by the upstream project. Upstream and dependency copyright and
license notices remain in place. Corresponding source and build instructions
must accompany this project's distributions.

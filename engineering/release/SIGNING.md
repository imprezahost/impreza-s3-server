# Signing a release (maintainer, outside CI)

The `rN` tag workflow (`.github/workflows/release.yml`) leaves an **unsigned draft** GitHub Release with
`impreza-s3-server-linux-amd64`, `impreza-s3-server-linux-arm64`, `SHA256SUMS` and `BUILDINFO.txt`. CI never signs and never sees the
private key. This is the maintainer's part: rebuild, compare, sign, attach, publish. Replace `<owner>/<repo>`, `rN`
and the paths below; nothing here contains a key.

## 0. Tag the release
The tag must be annotated, point at a commit on `main`, and carry the upstream base the fork is built on, on its own
line. The workflow refuses any other tag.

```sh
git tag -a rN -m "rN" -m "Upstream-Base: <40-hex upstream commit>" -m "<references: advisories, backports>"
git push origin rN
```

## 1. Download the draft
```sh
gh release download rN --repo <owner>/<repo> --dir rN \
  --pattern 'impreza-s3-server-linux-*' --pattern SHA256SUMS --pattern BUILDINFO.txt
(cd rN && sha256sum --check SHA256SUMS)
```
This only proves the files match the sums next to them. The next step is what makes them trustworthy.

## 2. Rebuild locally and compare
In a clean checkout of the tag, with Go from `go.mod` (`toolchain`) and a module cache prepared as in the root
README, build twice per architecture and compare, then check that your sums are the draft's:

```sh
RELEASE_WORKDIR="$PWD" # the directory containing the downloaded rN directory
git clone https://github.com/<owner>/<repo> src && cd src && git checkout rN
S3_GO="$(command -v go)"; S3_CACHE="$PWD/../s3-module-cache"
GOTOOLCHAIN=local GOMODCACHE="$S3_CACHE" GOPROXY=https://proxy.golang.org GOSUMDB=sum.golang.org "$S3_GO" mod download
: > ../local-SHA256SUMS
for arch in amd64 arm64; do
  GOARCH=$arch bash engineering/build/build.sh "$PWD" "$PWD/../build-a-$arch" "$S3_GO" "$S3_CACHE"
  GOARCH=$arch bash engineering/build/build.sh "$PWD" "$PWD/../build-b-$arch" "$S3_GO" "$S3_CACHE"
  bash engineering/build/compare.sh "$PWD/../build-a-$arch" "$PWD/../build-b-$arch"
  (cd "../build-a-$arch" && sha256sum impreza-s3-server | sed "s/  impreza-s3-server$/  impreza-s3-server-linux-$arch/") >> ../local-SHA256SUMS
done
cd "$RELEASE_WORKDIR" && diff local-SHA256SUMS rN/SHA256SUMS
```
Stop if `diff` prints anything: the CI build is not the one this commit produces here.

## 3. Sign (offline key)
The secret key lives outside any repository and any CI, on a machine or medium only the maintainer uses, protected
by the minisign password.

```sh
cd rN
minisign -S -s /path/to/offline/impreza-s3-server.key -m SHA256SUMS -t "impreza-s3-server rN SHA256SUMS"
minisign -S -s /path/to/offline/impreza-s3-server.key -m impreza-s3-server-linux-amd64 -t "impreza-s3-server rN linux/amd64"
minisign -S -s /path/to/offline/impreza-s3-server.key -m impreza-s3-server-linux-arm64 -t "impreza-s3-server rN linux/arm64"
for f in SHA256SUMS impreza-s3-server-linux-amd64 impreza-s3-server-linux-arm64; do minisign -V -p /path/to/impreza-s3-server.pub -m "$f"; done
```

## 4. Attach and publish
```sh
gh release upload rN --repo <owner>/<repo> SHA256SUMS.minisig impreza-s3-server-linux-amd64.minisig impreza-s3-server-linux-arm64.minisig
```
Publish the public key and this release's `SHA256SUMS` **outside GitHub** first (the documentation site and its onion
service), so verifying a download never depends only on the same page that served it. Then publish the draft:

```sh
gh release edit rN --repo <owner>/<repo> --draft=false
```

## Key generation, storage and rotation
- **Generate** once, offline: `minisign -G -p impreza-s3-server.pub -s /path/to/offline/impreza-s3-server.key`, with a
  strong password. Publish only the `.pub` (documentation site and onion service, and a pointer in README/SECURITY).
- **Store** the secret key encrypted (minisign's password) on offline media, with one offline backup kept apart. Never
  in a repository, a CI secret, a cloud drive or a chat.
- **Rotate** when the key or its password may be exposed, or on a schedule the maintainer sets: generate a new pair,
  sign a short statement naming the new public key with the **old** key (`minisign -S -s /path/to/offline/old-impreza-s3-server.key -m new-key-statement.txt`), publish
  both and the statement outside GitHub, and sign every later release with the new key only. If the old key is
  compromised, say so where the public key is published and do not sign the statement with it.

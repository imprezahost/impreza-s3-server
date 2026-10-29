'use strict';
// The positive checks and their filesystem/workflow mutation controls share
// these rules. No fake filesystem or canned negative result is involved.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const removed = ['Dockerfile', 'Dockerfile.cicd', 'Dockerfile.hotfix', 'Dockerfile.release', 'Dockerfile.release.old_cpu', 'Dockerfile.scratch',
  'docker-buildx.sh', 'helm-reindex.sh', 'index.yaml', 'dockerscripts', 'helm', 'helm-releases', 'docs/resiliency',
  '.github/workflows/mint', '.github/workflows/multipart', '.github/workflows/run-mint.sh',
  'buildscripts/minio-iam-ldap-upgrade-import-test.sh'];
const actions = [['go-lint.yml', 'golangci/golangci-lint-action'], ['typos.yml', 'crate-ci/typos'], ['shfmt.yml', 'luizm/action-sh-checker']];
function assets(tree, check = assert.ok) {
  for (const name of removed) check(!fs.existsSync(path.join(tree, name)), `${name}: removed (binaries-only fork)`);
}
function makefile(tree, check = assert.ok) {
  check(!/dl\.min(?:io)?\.io|quay\.io\/minio/.test(fs.readFileSync(path.join(tree, 'Makefile'), 'utf8')), 'Makefile: no upstream artifact target');
}
function pins(tree, check = assert.ok) {
  for (const [file, action] of actions) {
    const text = fs.readFileSync(path.join(tree, '.github/workflows', file), 'utf8');
    const line = text.split('\n').find(l => l.includes(`uses: ${action}@`));
    check(line && /@([0-9a-f]{40}) # v[0-9]/.test(line), `${file}: ${action} pinned by SHA with version comment`);
    check(line && !/[v\^]?(master|latest|v\d+)\s*$/.test(line.replace(/ #.*$/, '').split('@').pop()), `${file}: ${action} not a floating tag`);
  }
}
function upstream(tree, check = assert.ok) {
  // Cover both download-host spellings, registry images, git clone/raw/release
  // downloads and go-install of upstream CLI tools. Go module imports, historic
  // source comments and static Markdown attribution are not executable inputs.
  const forbidden = /dl\.min(?:io)?\.io|quay\.io\/minio|https?:\/\/(?:raw\.githubusercontent\.com\/minio\/|github\.com\/minio\/)|go\s+install[^\n]*github\.com\/minio\//;
  function walk(dir) {
    if (!fs.existsSync(dir)) return;
    for (const entry of fs.readdirSync(dir, {withFileTypes: true})) {
      if (entry.name === '.git') continue;
      const full = path.join(dir, entry.name);
      check(!entry.isSymbolicLink(), `${full}: executable input link refused`);
      if (entry.isDirectory()) walk(full);
      else if (/\.(?:sh|ya?ml)$/.test(entry.name)) check(!forbidden.test(fs.readFileSync(full, 'utf8')), `${path.relative(tree, full)}: no upstream test artifact acquisition`);
    }
  }
  walk(tree);
  makefile(tree, check);
}
// The release reuses these workflows from the same source commit. Its transitive actions must be immutable too.
function releasePins(tree, check = assert.ok) {
  for (const file of ['reproducible-build.yml', 'vulncheck.yml']) {
    const text=fs.readFileSync(path.join(tree,'.github/workflows',file),'utf8');
    for (const line of text.split('\n').filter(l=>/^\s*(?:-\s*)?uses:/.test(l))) {
      check(/uses:\s+[\w.-]+\/[\w./-]+@[0-9a-f]{40}\s+#\s+v\d/.test(line), `${file}: transitive release action pinned by commit with version comment`);
    }
  }
}
// Installation commands are inspected, never executed. This is a bounded
// workflow policy, not a complete shell interpreter or network sandbox.
function installScript(script, name, check = assert.ok) {
  const text = script.replace(/\\\r?\n/g, ' ');
  check(!/@latest\b/i.test(text), `${name}: no floating latest tool`);
  check(!/\b(?:curl|wget)\b[^\n;]*\|\s*(?:sudo\s+(?:-\S+\s+)*)?(?:\/(?:usr\/)?bin\/)?(?:ba)?sh\b/.test(text), `${name}: no fetched installer piped to a shell`);
  for (const raw of text.split(/[\n;&|]+/)) {
    const line = raw.replace(/\s+#.*$/, '').trim();
    if (!line || line.startsWith('#')) continue;
    for (const [command, exact] of [
      [/\bgo\s+install\b(.*)/, /^[\w.~+/-]+@v\d+\.\d+\.\d+(?:-[\w.-]+)?(?:\+[\w.-]+)?$/],
      [/\bnpm\s+(?:(?:-g|--global|--location=global)\s+)*(?:install|i)\b(.*)/, /^(?:@[\w.-]+\/)?[\w.-]+@\d+\.\d+\.\d+(?:-[\w.-]+)?$/],
      [/\bpip\d*\s+install\b(.*)/, /^[\w.-]+(?:\[[\w,.-]+\])?==\d+(?:\.\d+)+(?:[\w.+-]*)$/],
    ]) {
      const match = line.match(command);
      if (!match) continue;
      const tokens = (match[1].match(/'[^']*'|"[^"]*"|[^\s]+/g) || []).map(t => t.replace(/^(['"])(.*)\1$/, '$2'));
      const packages = tokens.filter(t => !t.startsWith('-'));
      check(packages.length > 0 && packages.every(p => exact.test(p)), `${name}: installed tools need exact versions`);
    }
  }
}
function toolInstalls(doc, text, name, check = assert.ok) {
  check(!/@latest\b/i.test(text), `${name}: no latest anywhere in a workflow`);
  for (const [job, value] of Object.entries(doc.jobs || {})) {
    for (const [i, step] of (value.steps || []).entries()) {
      if (typeof step.run === 'string') installScript(step.run, `${name}/${job}/${i}`, check);
    }
  }
}
function makeToolPins(tree, check = assert.ok) {
  const text = fs.readFileSync(path.join(tree, 'Makefile'), 'utf8');
  const pin = text.match(/^GOLANGCI_VERSION\s*:=\s*(v\d+\.\d+\.\d+)\s*$/m);
  check(pin, 'Makefile: exact golangci-lint version');
  check(text.includes('go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VERSION)'), 'Makefile: install the pinned linter module');
  installScript(text.replaceAll('$(GOLANGCI_VERSION)', pin[1]), 'Makefile', check);
}
// Published files never carry internal identifiers: the fork's internal
// change IDs (S3M<n>), three-digit internal ticket numbers of the X/F
// namespace and single-letter review-session names in documentation and
// configuration text. Upstream technical tokens must not match: X86-64 and
// X25519 (not exactly three digits) and the erasure benchmarks' X000
// (zero-leading) stay allowed by construction, and generated dependency data
// (go.mod/go.sum) plus third-party texts without a checked extension
// (LICENSE, NOTICE, CREDITS) are out of this rule's scope.
const INTERNAL_CHANGE = /\bS3M[0-9][a-z]?\b/i;
// X509 is the certificate standard, not a ticket (it appears in Go comments).
const INTERNAL_TICKET = /\b[XF](?!509\b)[1-9][0-9]{2}\b/;
const INTERNAL_SESSION = /\b(?:C|R|M)(?:1[0-9]|[1-9])\b/;
// F-numbered sessions and features, with an optional dotted increment.
const INTERNAL_FEATURE = /\bF[1-9][0-9]?(?:\.[0-9]+)?\b/;
// The coordinating role's name, lowercase only: upstream test data has a capitalized person name.
const INTERNAL_ROLE = new RegExp('\\b' + 'mae' + 'stro' + '\\b');
const ID_TEXT = /\.(?:md|markdown|mjs|cjs|js|py|sh|ya?ml|json|txt|html|tpl|toml|cfg|conf|ini|go|mk)$/i;
// Build files without an extension carry comments too (the Makefile had internal IDs).
const ID_NAMES = /^(?:Makefile|Dockerfile(?:\..+)?)$/;
function internalIds(tree, check = assert.ok) {
  function walk(dir) {
    for (const entry of fs.readdirSync(dir, {withFileTypes: true})) {
      if (entry.name === '.git' || entry.name === 'node_modules') continue;
      const full = path.join(dir, entry.name);
      if (entry.isDirectory()) { walk(full); continue; }
      if (!entry.isFile() || !(ID_TEXT.test(entry.name) || ID_NAMES.test(entry.name))) continue;
      if (/^(?:go\.mod|go\.sum)$/.test(entry.name)) continue;
      const text = fs.readFileSync(full, 'utf8');
      const rel = path.relative(tree, full);
      check(!INTERNAL_CHANGE.test(text), `${rel}: no internal change ID`);
      check(!INTERNAL_TICKET.test(text), `${rel}: no internal ticket number`);
      check(!INTERNAL_SESSION.test(text), `${rel}: no internal session name`);
      check(!INTERNAL_FEATURE.test(text), `${rel}: no internal feature or F-session ID`);
      check(!INTERNAL_ROLE.test(text), `${rel}: no internal role name`);
    }
  }
  walk(tree);
}
module.exports = {assets, makefile, pins, releasePins, upstream, actions, installScript, toolInstalls, makeToolPins, internalIds};

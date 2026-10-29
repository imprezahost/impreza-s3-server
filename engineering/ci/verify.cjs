'use strict';
// Offline structural/dispatch regression checks. Requires an existing js-yaml 4.x.
// No dependency installation, workflow execution, secret reads or network access.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const cp = require('node:child_process');
const os = require('node:os');
const rules = require('./rules.cjs');
const yaml = require('js-yaml');
const root = path.resolve(__dirname, '../..');
const base = process.argv[2];
let checks = 0;
function check(value, message) { assert.ok(value, message); checks++; }
function load(text) { return yaml.load(text); } // Duplicate mapping keys are errors.
function dispatch(doc, event, branch) {
  const events = doc.on;
  if (Array.isArray(events)) return events.includes(event);
  if (!events || !(event in events)) return false;
  const cfg = events[event];
  return !cfg?.branches || cfg.branches.includes(branch);
}
function validate(doc, text, name) {
  check(doc && typeof doc.name === 'string', `${name}: name`);
  for (const event of ['push', 'pull_request']) {
    check(dispatch(doc, event, 'main'), `${name}: ${event} must dispatch on main`);
    check(!dispatch(doc, event, 'master'), `${name}: ${event} must not dispatch on master`);
  }
  check(doc.permissions?.contents === 'read', `${name}: read-only contents token`);
  check(!Object.values(doc.permissions).includes('write'), `${name}: no token write permissions`);
  check(!/secrets\./.test(text), `${name}: no repository secrets`);
  check(doc.jobs && Object.keys(doc.jobs).length, `${name}: jobs`);
  if (doc.concurrency) check(doc.concurrency.group.includes('github.ref'), `${name}: push/PR concurrency identity`);
  for (const [id, job] of Object.entries(doc.jobs)) {
    check(typeof job['runs-on'] === 'string', `${name}/${id}: runner`);
    check(job['runs-on'] === 'ubuntu-latest' || job['runs-on'] === '${{ matrix.os }}', `${name}/${id}: hosted runner only`);
    if (job.strategy) check(Object.keys(job.strategy.matrix ?? {}).length > 0, `${name}/${id}: nonempty matrix`);
    check(Array.isArray(job.steps) && job.steps.length, `${name}/${id}: steps`);
    if (job.if === '${{ false }}') check(text.includes('# DISABLED:'), `${name}/${id}: explain disabled coverage`);
    for (const step of job.steps) {
      check(Boolean(step.run) !== Boolean(step.uses), `${name}/${id}: exactly one run or uses`);
      if (step.uses?.startsWith('actions/setup-go@')) {
        // One Go source: either the module file directly, or the exact
        // toolchain DERIVED from go.mod by a step of this job (setup-go
        // ignores the toolchain line; build.sh pins GOTOOLCHAIN=local).
        const derived = job.steps.some(s => /sed -n 's\/\^toolchain go\/\/p' go\.mod/.test(String(s.run ?? '')));
        const okVersion = step.with?.['go-version-file'] === 'go.mod'
          || (step.with?.['go-version'] === '${{ env.GO_TOOLCHAIN }}' && derived);
        check(okVersion, `${name}/${id}: one Go source (go.mod, directly or derived)`);
        check(!(step.with?.['go-version-file'] !== undefined && 'go-version' in step.with) && !('cached' in step.with), `${name}/${id}: valid setup-go inputs`);
      }
      if (step.uses?.startsWith('actions/checkout@')) check(step.with?.['persist-credentials'] === false, `${name}/${id}: do not persist token`);
      for (const value of Object.values(step.env ?? {})) check(['string', 'number'].includes(typeof value), `${name}/${id}: env scalar`);
      const expr = JSON.stringify(step) + (job.name ?? '');
      for (const m of expr.matchAll(/matrix\.([\w-]+)/g)) check(m[1] in (job.strategy?.matrix ?? {}), `${name}/${id}: defined matrix.${m[1]}`);
    }
  }
}
// The rN release workflow. Only an r<number> tag push dispatches it; it uses no repository secret; it grants
// nothing at the top and write only to the draft job's contents; no job runs an action (so nothing needs a pin and no
// action ever holds the write token) and every `uses:` is a local reusable workflow; no `${{ }}` expression reaches a
// shell script; the tag is checked before anything builds; and the result is a draft nothing here signs or publishes.
function validateRelease(doc, text, name) {
  check(doc && typeof doc.name === 'string', `${name}: name`);
  const on = doc.on ?? {};
  check(Object.keys(on).length === 1 && on.push && Object.keys(on.push).length === 1, `${name}: only a tag push dispatches`);
  check(JSON.stringify(on.push?.tags) === JSON.stringify(['r[0-9]*']), `${name}: only rN tags`);
  check(!/secrets\./.test(text), `${name}: no repository secrets`);
  check(doc.permissions && typeof doc.permissions === 'object' && Object.keys(doc.permissions).length === 0, `${name}: no permission at the top`);
  const jobs = doc.jobs ?? {};
  check(Object.keys(jobs).sort().join(',') === 'build,check,draft,vulncheck', `${name}: check, build, vulncheck, draft`);
  for (const [id, job] of Object.entries(jobs)) {
    const perms = job.permissions;
    check(perms && typeof perms === 'object', `${name}/${id}: explicit job permissions`);
    check(!('secrets' in job), name + '/' + id + ': no inherited or explicit secrets');
    const granted = Object.entries(perms).filter(([, v]) => v !== 'none');
    if (id === 'draft') {
      check(perms.contents === 'write' && granted.every(([k, v]) => (k === 'contents' && v === 'write') || (k === 'actions' && v === 'read')), `${name}/draft: write only to contents`);
    } else {
      check(perms.contents === 'read' && granted.length === 1, `${name}/${id}: read-only`);
    }
    if (job.uses !== undefined) {
      check(/^\.\/\.github\/workflows\/[\w.-]+\.ya?ml$/.test(job.uses) && !job.steps, `${name}/${id}: only a local reusable workflow`);
    } else {
      check(job['runs-on'] === 'ubuntu-latest', `${name}/${id}: hosted runner`);
      check(Array.isArray(job.steps) && job.steps.length, `${name}/${id}: steps`);
    }
    for (const step of job.steps ?? []) {
      check(typeof step.run === 'string' && !step.uses, `${name}/${id}: no action, only scripts`);
      check(!/\$\{\{/.test(step.run), `${name}/${id}: no expression inside a shell script`);
      for (const value of Object.values(step.env ?? {})) check(['string', 'number'].includes(typeof value), `${name}/${id}: env scalar`);
    }
  }
  check(jobs.build?.uses === './.github/workflows/reproducible-build.yml', `${name}: build reuses the reproducible build workflow`);
  check(jobs.vulncheck?.uses === './.github/workflows/vulncheck.yml', `${name}: govulncheck runs`);
  for (const j of ['build', 'vulncheck']) check(JSON.stringify(jobs[j]?.needs) === '["check"]', `${name}/${j}: after the tag check`);
  check(['check', 'build', 'vulncheck'].every(n => (jobs.draft?.needs ?? []).includes(n)), `${name}/draft: after the check, the builds and govulncheck`);
  const runs = j => (jobs[j]?.steps ?? []).map(s => s.run ?? '').join('\n');
  for (const [re, what] of [[/\^r\[0-9\]\{1,6\}\$/, 'r<number> tag'], [/\.object\.type <<<"\$ref"\)" = tag/, 'annotated tag'],
                            [/Upstream-Base: /, 'upstream base line'], [/compare\/\$SOURCE\.\.\.main/, 'commit on main'],
                            [/\^toolchain /, 'toolchain from go.mod']]) {
    check(re.test(runs('check')), `${name}/check: ${what}`);
  }
  check(/gh release create "\$TAG" --draft --verify-tag/.test(runs('draft')), `${name}/draft: creates a draft only`);
  check(/SHA256SUMS-\$arch/.test(runs('draft')) && /sha256sum --check SHA256SUMS/.test(runs('draft')), `${name}/draft: only the compared binaries`);
  const all = Object.keys(jobs).map(runs).join('\n');
  check(!/^\s*(minisign|gpg|cosign|signify)\b/m.test(all) && !/--draft=false|gh release (edit|upload)/.test(all), `${name}: nothing signs or publishes in CI`);
}
const files = fs.readdirSync(path.join(root, '.github/workflows')).filter(n => /\.ya?ml$/.test(n)).sort();
const docs = new Map();
for (const name of files) {
  const text = fs.readFileSync(path.join(root, '.github/workflows', name), 'utf8');
  const doc = load(text); rules.toolInstalls(doc, text, name, check);
  (name === 'release.yml' ? validateRelease : validate)(doc, text, name); docs.set(name, doc);
}
for (const name of ['go.yml', 'go-lint.yml', 'vulncheck.yml', 'go-cross.yml']) check(docs.has(name), `primary ${name}`);
// The release exists, and the two workflows it reuses can be called.
check(docs.has('release.yml'), 'release.yml exists');
for (const name of ['reproducible-build.yml', 'vulncheck.yml']) check('workflow_call' in (docs.get(name).on ?? {}), `${name}: callable by the release workflow`);
const vuln = docs.get('vulncheck.yml');
check(vuln.on.schedule?.length === 1 && /^\d+ \d+ \* \* \*$/.test(vuln.on.schedule[0].cron), 'vulncheck daily schedule');
check(docs.get('depsreview.yaml').jobs['dependency-review'].if === "github.event_name == 'pull_request'", 'dependency-review requires PR base/head');
for (const name of ['issues.yaml', 'lock.yml']) {
  check(Object.values(docs.get(name).jobs).every(j => j.if === '${{ false }}'), `${name}: inherited resources disabled before runner allocation`);
}
// Binaries only. The container/image distribution and every rig that
// pulled upstream images left the tree; what remains disabled must not cost
// a runner.
for (const name of ['iam-integrations.yaml', 'mint.yml', 'helm-lint.yml']) check(!docs.has(name), `${name}: removed with the container distribution`);
rules.assets(root, check);
rules.makefile(root, check);
rules.upstream(root, check);
// Nothing published carries an internal change ID, ticket number or
// review-session name; upstream technical tokens (X86-64, X25519, the
// benchmarks' X000) must keep passing.
rules.internalIds(root, check);
// The Go toolchain comes from go.mod only: build.sh reads it, and no second
// pinned copy exists anywhere in the build scripts .
const buildSh = fs.readFileSync(path.join(root, 'engineering/build/build.sh'), 'utf8');
check(buildSh.includes('go.mod has no toolchain line') && buildSh.includes('^toolchain'), 'build.sh reads the toolchain from go.mod');
check(!/TOOLCHAIN=go[0-9]/.test(buildSh), 'build.sh has no second pinned toolchain constant');
check(/GOARCH=\$\{GOARCH:-amd64\}/.test(buildSh), 'build.sh accepts GOARCH for the CI matrix');
// Third-party actions are pinned by commit SHA with a version comment.
rules.pins(root, check); rules.releasePins(root, check); rules.makeToolPins(root, check);
// The reproducible-build job: two independent builds per arch, byte compared,
// SHA256SUMS artifact, no secret and no signing step.
const rb = docs.get('reproducible-build.yml');
check(rb, 'reproducible-build.yml exists');
check(['build-a', 'build-b', 'compare'].every(j => rb.jobs[j]), 'reproducible-build: build-a/build-b/compare');
check(JSON.stringify(rb.jobs).includes('amd64') && JSON.stringify(rb.jobs).includes('arm64'), 'reproducible-build: linux/amd64 and linux/arm64 matrix');
check(rb.jobs.compare.needs.includes('build-a') && rb.jobs.compare.needs.includes('build-b'), 'reproducible-build: compare needs both builds');
check(/compare\.sh/.test(JSON.stringify(rb.jobs.compare)), 'reproducible-build: compare runs compare.sh');
check(JSON.stringify(rb.jobs).includes('SHA256SUMS'), 'reproducible-build: SHA256SUMS artifact');
check(!/secrets\.|minisign|gpg/i.test(JSON.stringify(rb)), 'reproducible-build: no secret, no signing in CI');
// build.sh is offline (GOPROXY=off); each build job must fetch and verify the modules before it, or a fresh runner fails.
const modulesBeforeBuild = (doc, job) => {
  const runs = (doc.jobs[job].steps || []).map(st => st.run || '');
  const dl = runs.findIndex(r => /go mod download/.test(r) && /go mod verify/.test(r));
  const build = runs.findIndex(r => /engineering\/build\/build\.sh/.test(r));
  return dl >= 0 && build > dl;
};
for (const job of ['build-a', 'build-b']) check(modulesBeforeBuild(rb, job), `reproducible-build/${job}: modules downloaded and verified before the offline build`);
const mod = fs.readFileSync(path.join(root, 'go.mod'), 'utf8');
check(/^toolchain go1\.26\.10$/m.test(mod), 'toolchain pinned in go.mod');
// build.sh refuses to build when go.mod/go.sum drift from this lock, so a go.mod edit must update it.
const lockText = fs.readFileSync(path.join(root, 'engineering/build/dependency-lock.sha256'), 'utf8');
function lockMatches(text) {
  const lines = text.trim().split('\n').map(l => l.split(/\s+\*?/));
  return lines.length === 2 && ['go.mod', 'go.sum'].every(f => lines.some(([sum, name]) => name === f &&
    sum === require('node:crypto').createHash('sha256').update(fs.readFileSync(path.join(root, f))).digest('hex')));
}
check(lockMatches(lockText), 'dependency-lock.sha256 matches go.mod/go.sum');
for (const name of ['README.md', 'SECURITY.md', 'engineering/build/BUILDING.md']) check(fs.readFileSync(path.join(root, name), 'utf8').includes('Go 1.26.10'), `${name}: documented toolchain`);
for (const name of ['.github/stale.yml', '.github/workflows/root.key', '.github/workflows/root.cert']) check(!fs.existsSync(path.join(root, name)), `${name}: removed`);
const dep = load(fs.readFileSync(path.join(root, '.github/dependabot.yml'), 'utf8'));
check(dep.version === 2 && dep.updates.length === 2, 'Dependabot schema/ecosystems');
for (const ecosystem of ['gomod', 'github-actions']) {
  const rule = dep.updates.find(u => u['package-ecosystem'] === ecosystem);
  check(rule?.directory === '/' && rule.schedule.interval === 'weekly', `${ecosystem}: weekly root updates`);
  check(!rule['target-branch'] && !rule.cooldown, `${ecosystem}: do not restrict/delay security PRs`);
  check(Object.values(rule.groups).every(g => g['applies-to'] === 'version-updates' && g.patterns.includes('*')), `${ecosystem}: group routine updates only`);
}
// Exercise dispatch behaviour, rather than treating textual grep as proof.
if (base) {
  for (const name of ['go.yml', 'go-lint.yml', 'vulncheck.yml', 'go-cross.yml']) {
    const old = load(cp.execFileSync('git', ['show', `${base}:.github/workflows/${name}`], {cwd: root, encoding: 'utf8'}));
    check(!dispatch(old, 'pull_request', 'main'), `${name}: BASE main PR does not dispatch`);
    check(dispatch(old, 'pull_request', 'master'), `${name}: BASE master PR dispatches`);
    for (const event of ['push', 'pull_request']) check(dispatch(docs.get(name), event, 'main'), `${name}: candidate main ${event} dispatches`);
  }
}
// Negative controls of the actual validator; all must fail by assertion/parser error.
const mainText = fs.readFileSync(path.join(root, '.github/workflows/go.yml'), 'utf8');
for (const [label, mutate] of [
  ['wrong branch', d => { d.on.pull_request.branches = ['master']; }],
  ['missing push', d => { delete d.on.push; }],
  ['version bypass', d => { const s=d.jobs.build.steps.find(s=>s.uses?.startsWith('actions/setup-go@'));s.with={'go-version':'1.24.x'}; }],
  ['self-hosted runner', d => { d.jobs.build['runs-on']='self-hosted'; }],
  ['credential persistence', d => { d.jobs.build.steps[0].with['persist-credentials']=true; }],
]) {
  const doc=load(mainText); mutate(doc);
  assert.throws(()=>validate(doc, mainText, label), assert.AssertionError, label);
  console.log(`NEGATIVE PASS: ${label}`);
}
assert.throws(()=>load('on:\n  push: {}\n  push: {}\n'), /duplicated mapping key/);
assert.throws(()=>check(lockMatches(lockText.replace(/^[0-9a-f]/, c => c === '0' ? '1' : '0')), 'stale lock'), assert.AssertionError);
console.log('NEGATIVE PASS: stale dependency lock');
// Negative controls of the new rules; each must fail by assertion.
{
  const tree = fs.mkdtempSync(path.join(os.tmpdir(), 'impreza-verify-controls-'));
  try {
    fs.mkdirSync(path.join(tree, '.github/workflows'), {recursive: true});
    for (const [file] of rules.actions) fs.copyFileSync(path.join(root, '.github/workflows', file), path.join(tree, '.github/workflows', file));
    fs.copyFileSync(path.join(root, 'Makefile'), path.join(tree, 'Makefile'));
    // Positive controls prove these very same rules accept the real inputs.
    rules.assets(tree); rules.makefile(tree); rules.pins(tree);
    fs.mkdirSync(path.join(tree, 'helm'));
    assert.throws(()=>rules.assets(tree), assert.AssertionError, 'removed asset reappears');
    fs.rmdirSync(path.join(tree, 'helm'));
    console.log('NEGATIVE PASS: removed asset reappears (real tree, production rule)');
    fs.appendFileSync(path.join(tree, 'Makefile'), '\nhotfix-push:\n\t@echo https://dl.min.io/server/minio/\n');
    assert.throws(()=>rules.makefile(tree), assert.AssertionError, 'minio host target returns');
    fs.copyFileSync(path.join(root, 'Makefile'), path.join(tree, 'Makefile'));
    console.log('NEGATIVE PASS: minio host target returns (real Makefile, production rule)');
    const file = path.join(tree, '.github/workflows/typos.yml');
    const pinned = fs.readFileSync(file, 'utf8');
    const floating = pinned.replace(/(uses: crate-ci\/typos)@[0-9a-f]{40}/, '$1@master');
    check(floating !== pinned, 'floating-tag mutation applied');
    fs.writeFileSync(file, floating);
    assert.throws(()=>rules.pins(tree), assert.AssertionError, 'floating action tag');
    console.log('NEGATIVE PASS: floating action tag (real workflow, production rule)');
    fs.writeFileSync(file, pinned);
    fs.mkdirSync(path.join(tree, 'docs/site-replication'), {recursive:true});
    fs.writeFileSync(path.join(tree, 'docs/site-replication/bad.sh'), '#!/bin/sh\nwget https://dl.minio.io/client/mc/release/linux-amd64/mc\n');
    assert.throws(()=>rules.upstream(tree), assert.AssertionError, 'alternate upstream host returns');
    console.log('NEGATIVE PASS: alternate upstream host returns (real tree)');
  } finally { fs.rmSync(tree, {recursive:true, force:true}); }
  assert.throws(()=>check(/TOOLCHAIN=go[0-9]/.test(buildSh), 'build.sh reads go.mod'), assert.AssertionError, 'second toolchain constant');
  console.log('NEGATIVE PASS: second toolchain constant');
  const bad = load(fs.readFileSync(path.join(root, '.github/workflows/reproducible-build.yml'), 'utf8'));
  delete bad.jobs['build-b'];
  assert.throws(()=>check(bad.jobs['build-b'], 'build-b present'), assert.AssertionError, 'single-build job');
  console.log('NEGATIVE PASS: single-build job');
  const nodl = load(fs.readFileSync(path.join(root, '.github/workflows/reproducible-build.yml'), 'utf8'));
  nodl.jobs['build-a'].steps = nodl.jobs['build-a'].steps.filter(st => !/go mod download/.test(st.run || ''));
  assert.throws(()=>check(modulesBeforeBuild(nodl, 'build-a'), 'modules before build'), assert.AssertionError, 'offline build without module download');
  console.log('NEGATIVE PASS: offline build without module download');
}
// Internal identifiers: each reintroduction must fail the production rule,
// while upstream technical tokens (X86-64, X25519, the benchmarks' X000)
// keep passing it.
{
  const tree = fs.mkdtempSync(path.join(os.tmpdir(), 'impreza-id-controls-'));
  try {
    rules.internalIds(tree);
    // The refused tokens are built by concatenation so this very file keeps passing the rule.
    const change = 'S3M' + '2', ticket = 'X' + '123', session = 'R' + '8';
    fs.writeFileSync(path.join(tree, 'README.md'), `Coverage note (${change}) kept for history.\n`);
    assert.throws(()=>rules.internalIds(tree), assert.AssertionError, 'internal change ID reintroduced');
    fs.writeFileSync(path.join(tree, 'README.md'), `Fixed as ${ticket}.\n`);
    assert.throws(()=>rules.internalIds(tree), assert.AssertionError, 'internal ticket number reintroduced');
    fs.writeFileSync(path.join(tree, 'notes.sh'), `# reviewed in ${session}\n`);
    assert.throws(()=>rules.internalIds(tree), assert.AssertionError, 'internal session name reintroduced');
    fs.rmSync(path.join(tree, 'notes.sh'));
    // Two-digit sessions, F-numbered sessions/features, the role name, and Go/Makefile scope.
    for (const [file, body, label] of [
      ['README.md', `Authored by ${'C' + '11'}.\n`, 'two-digit session name reintroduced'],
      ['README.md', `Authored by ${'F' + '7'}.\n`, 'F-numbered session reintroduced'],
      ['README.md', `Pilot of ${'F' + '43.7'}.\n`, 'internal feature ID reintroduced'],
      ['README.md', `Approved by the ${'mae' + 'stro'}.\n`, 'internal role name reintroduced'],
      ['guard.go', `// ${'X' + '826'}: keep the guard\npackage cmd\n`, 'ticket in a Go comment reintroduced'],
      ['Makefile', `# ${'S3M' + '2'}: binaries only\n`, 'change ID in the Makefile reintroduced'],
    ]) {
      fs.rmSync(path.join(tree, 'README.md'), {force: true});
      fs.writeFileSync(path.join(tree, file), body);
      assert.throws(()=>rules.internalIds(tree), assert.AssertionError, label);
      fs.rmSync(path.join(tree, file));
    }
    fs.writeFileSync(path.join(tree, 'certs.go'), '// LoadX509KeyPair - load an X509 key pair\npackage config\n');
    rules.internalIds(tree);
    fs.rmSync(path.join(tree, 'certs.go'));
    // A full OpenPGP fingerprint passes; a ticket on a line that merely mentions PGP does not.
    fs.writeFileSync(path.join(tree, 'README.md'), 'Fingerprint: `FD4A F833 BBA6 D9B8 B890  B1AE 8BF2 4F8D E8F9 50F9`\n');
    rules.internalIds(tree);
    fs.writeFileSync(path.join(tree, 'README.md'), `The PGP key was rotated in ${'X' + '826'}.\n`);
    assert.throws(()=>rules.internalIds(tree), assert.AssertionError, 'ticket on a PGP line');
    fs.rmSync(path.join(tree, 'README.md'), {force: true});
    // Portuguese in public texts is refused (accented and unaccented);
    // inherited Go unicode test data stays out of the language rule.
    fs.writeFileSync(path.join(tree, 'README.md'), 'Prazo: a de' + 'finir pe' + 'lo mantened' + 'or.\n');
    assert.throws(()=>rules.internalIds(tree), assert.AssertionError, 'Portuguese text reintroduced');
    fs.writeFileSync(path.join(tree, 'README.md'), 'O prazo n\u00e3o foi de' + 'finido.\n');
    assert.throws(()=>rules.internalIds(tree), assert.AssertionError, 'accented Portuguese reintroduced');
    fs.rmSync(path.join(tree, 'README.md'), {force: true});
    fs.writeFileSync(path.join(tree, 'testdata.go'), 'valid := []string{"t\u00e1bl\u00eb"}\npackage target\n');
    rules.internalIds(tree);
    fs.rmSync(path.join(tree, 'testdata.go'));
    fs.writeFileSync(path.join(tree, 'README.md'), 'X86-64 CPUs, X25519 curves and the X000 benchmark labels are technical tokens.\n');
    rules.internalIds(tree);
    console.log('NEGATIVE PASS: internal change ID, ticket, session, feature, role (text, Go, Makefile) refused; Portuguese refused in texts; technical tokens pass');
  } finally { fs.rmSync(tree, {recursive:true, force:true}); }
}
// The published SECURITY-KEY.asc is the maintainer's exact export: it parses
// as an OpenPGP key and its primary fingerprint is the one SECURITY.md
// names. gpg reads the key (ubuntu runners and dev boxes ship it); a missing
// gpg fails closed instead of skipping the check.
{
  const ascPath = path.join(root, 'SECURITY-KEY.asc');
  check(fs.existsSync(ascPath), 'SECURITY-KEY.asc: the public key file ships in the repository');
  const asc = fs.readFileSync(ascPath, 'utf8');
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 's3-key-'));
  fs.mkdirSync(path.join(tmp, 'gnupg'), {mode: 0o700});
  try {
    const readFingerprint = text => {
      fs.writeFileSync(path.join(tmp, 'k.asc'), text);
      // A private, empty home: no keyring, options or trustdb of the runner or
      // developer is read, and nothing is written outside the temporary dir.
      const r = cp.spawnSync('gpg', ['--homedir', path.join(tmp, 'gnupg'), '--batch', '--no-options', '--show-keys', '--with-colons', path.join(tmp, 'k.asc')], {encoding: 'utf8'});
      if (r.status !== 0) return null;
      return (r.stdout.match(/^fpr:::::::::([0-9A-F]{40})/m) || [])[1] ?? null;
    };
    const fingerprint = readFingerprint(asc);
    check(fingerprint !== null, 'SECURITY-KEY.asc parses as an OpenPGP public key (gpg present)');
    const sec = fs.readFileSync(path.join(root, 'SECURITY.md'), 'utf8');
    const doc = sec.match(/Fingerprint: `([0-9A-F ]+)`/);
    check(doc !== null, 'SECURITY.md publishes the fingerprint in the expected form');
    if (fingerprint !== null && doc !== null) {
      check(doc[1].replace(/ /g, '') === fingerprint, 'SECURITY-KEY.asc carries exactly the fingerprint SECURITY.md names');
      // Negative: flip one base64 character inside the key body — the file
      // must no longer match the published fingerprint (or not parse at all).
      const lines = asc.split('\n');
      const bodyIdx = lines.findIndex((l, i) => i > 0 && l.length > 10 && /^[A-Za-z0-9+/=]+$/.test(l));
      lines[bodyIdx] = lines[bodyIdx].slice(0, 3) + (lines[bodyIdx][3] === 'A' ? 'B' : 'A') + lines[bodyIdx].slice(4);
      check(readFingerprint(lines.join('\n')) !== fingerprint, 'a mutated key file no longer matches the published fingerprint');
      console.log('NEGATIVE PASS: SECURITY-KEY.asc fingerprint matches SECURITY.md; a mutated key does not');
    }
  } finally { fs.rmSync(tmp, {recursive:true, force:true}); }
}
// Negative controls: each mutation of the real release workflow must fail validateRelease by assertion. The text
// rules read yaml.dump of the mutated document, so a mutation reaches them too.
let releaseNegatives = 0;
{
  const tree=fs.mkdtempSync(path.join(os.tmpdir(),'impreza-release-pins-controls-'));
  try {
    fs.mkdirSync(path.join(tree,'.github/workflows'),{recursive:true});
    for (const file of ['reproducible-build.yml','vulncheck.yml']) fs.copyFileSync(path.join(root,'.github/workflows',file),path.join(tree,'.github/workflows',file));
    rules.releasePins(tree);
    for (const file of ['reproducible-build.yml','vulncheck.yml']) {
      const target=path.join(tree,'.github/workflows',file), good=fs.readFileSync(target,'utf8');
      fs.writeFileSync(target,good.replace(/actions\/checkout@[0-9a-f]{40}/,'actions/checkout@v4'));
      assert.throws(()=>rules.releasePins(tree),assert.AssertionError,'floating release child action');
      fs.writeFileSync(target,good); releaseNegatives++;
      console.log('NEGATIVE PASS: release transitive pin '+file);
    }
  } finally { fs.rmSync(tree,{recursive:true,force:true}); }
}
{
  const relText = fs.readFileSync(path.join(root, '.github/workflows/release.yml'), 'utf8');
  for (const [label, mutate] of [
    ['branch push dispatches', d => { d.on.push.branches = ['main']; }],
    ['pull request dispatches', d => { d.on.pull_request = {}; }],
    ['manual dispatch', d => { d.on.workflow_dispatch = null; }],
    ['any tag', d => { d.on.push.tags = ['*']; }],
    ['inherited secrets', d => { d.jobs.build.secrets = 'inherit'; }],
    ['unneeded read permission', d => { d.jobs.check.permissions.checks = 'read'; }],
    ['repository secret', d => { d.jobs.draft.env.TOKEN = '${{ secrets.RELEASE_TOKEN }}'; }],
    ['top-level permission', d => { d.permissions = { contents: 'write' }; }],
    ['write token in a build job', d => { d.jobs.build.permissions = { contents: 'write' }; }],
    ['draft writes more', d => { d.jobs.draft.permissions['id-token'] = 'write'; }],
    ['action with the write token', d => { d.jobs.draft.steps.unshift({ uses: 'actions/checkout@v4' }); }],
    ['external reusable workflow', d => { d.jobs.vulncheck.uses = 'someone/workflows/.github/workflows/vulncheck.yml@main'; }],
    ['expression inside a script', d => { d.jobs.draft.steps[1].run += '\necho "${{ github.ref_name }}"\n'; }],
    ['published, not a draft', d => { d.jobs.draft.steps[1].run = d.jobs.draft.steps[1].run.replace('--draft --verify-tag', '--verify-tag'); }],
    ['signing in CI', d => { d.jobs.draft.steps.push({ name: 'sign', run: 'minisign -S -s key -m SHA256SUMS\n' }); }],
    ['no upstream base check', d => { d.jobs.check.steps[0].run = d.jobs.check.steps[0].run.replace(/Upstream-Base: /g, 'Base: '); }],
    ['draft before govulncheck', d => { d.jobs.draft.needs = ['check', 'build']; }],
    ['build without the tag check', d => { delete d.jobs.build.needs; }],
  ]) {
    const doc = load(relText);
    mutate(doc);
    assert.throws(() => validateRelease(doc, yaml.dump(doc), label), assert.AssertionError, label);
    console.log(`NEGATIVE PASS: release ${label}`);
    releaseNegatives++;
  }
}
// Mutate the actual reusable scan workflow and call its production rule.
let toolNegatives = 0;
{
  const good = fs.readFileSync(path.join(root, '.github/workflows/vulncheck.yml'), 'utf8');
  const scan = load(good);
  const index = scan.jobs.vulncheck.steps.findIndex(s => /go install/.test(s.run || ''));
  check(index >= 0, 'tool-install mutation has the real installer step');
  rules.toolInstalls(scan, good, 'positive scan workflow');
  for (const [label, run] of [
    ['floating govulncheck', 'go install golang.org/x/vuln/cmd/govulncheck@latest'],
    ['unversioned Go tool', 'go install golang.org/x/vuln/cmd/govulncheck'],
    ['unversioned npm global', 'npm install -g fixture'],
    ['floating npm global', 'npm install --global fixture@latest'],
    ['unversioned pip tool', 'python3 -m pip install fixture'],
    ['pip range', 'pip install fixture>=1.2.3'],
    ['fetched shell installer', 'curl -fsSL https://example.invalid/install.sh | bash'],
    // Construct an actual backslash + LF, not the literal characters backslash-n.
    ['multiline fetched installer', 'wget -qO- https://example.invalid/install.sh | ' + String.fromCharCode(92, 10) + 'sh'],
    ['unversioned npm global before install', 'npm -g install fixture'],
    ['absolute shell installer', 'curl -fsSL https://example.invalid/install.sh | /bin/sh'],
  ]) {
    const doc = load(good); doc.jobs.vulncheck.steps[index].run = run;
    assert.throws(() => rules.toolInstalls(doc, yaml.dump(doc), label), assert.AssertionError, label);
    toolNegatives++; console.log('NEGATIVE PASS: tool install ' + label);
  }
  // The Makefile linter pin rule negative: mutate a real copy of the Makefile and call makeToolPins on it.
  const mk = fs.mkdtempSync(path.join(os.tmpdir(), 'impreza-make-controls-'));
  try {
    const makefileText = fs.readFileSync(path.join(root, 'Makefile'), 'utf8');
    rules.makeToolPins((fs.writeFileSync(path.join(mk, 'Makefile'), makefileText), mk));
    for (const [label, from, to] of [
      ['Makefile linter pin floats', /^GOLANGCI_VERSION\s*:=.*$/m, 'GOLANGCI_VERSION := latest'],
      ['Makefile installer piped to sh', /^\t@GOTOOLCHAIN=local .*go install .*$/m, '\t@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(GOLANGCI_DIR)'],
    ]) {
      const mutated = makefileText.replace(from, to);
      check(mutated !== makefileText, label + ' mutation applied');
      fs.writeFileSync(path.join(mk, 'Makefile'), mutated);
      assert.throws(() => rules.makeToolPins(mk), assert.AssertionError, label);
      toolNegatives++; console.log('NEGATIVE PASS: ' + label);
    }
  } finally { fs.rmSync(mk, {recursive: true, force: true}); }
  for (const run of ['go install example.invalid/fixture@v1.2.3', 'npm install -g @fixture/tool@1.2.3', 'python3 -m pip install fixture==1.2.3']) {
    const doc = load(good); doc.jobs.vulncheck.steps[index].run = run;
    rules.toolInstalls(doc, yaml.dump(doc), 'exact version syntax control');
  }
}
console.log(`PASS: ${files.length} workflow YAMLs; ${checks} assertions; BASE=${base ?? 'not requested'}; ${14 + releaseNegatives + toolNegatives + 4} negative controls`);
console.log('LIMIT: structural/dispatch checks, not actionlint expression/shell lint or execution of GitHub-hosted jobs.');

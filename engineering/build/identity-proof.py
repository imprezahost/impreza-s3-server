#!/usr/bin/env python3
"""Finite offline build experiment. Does not run the executable or publish artifacts."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import signal
import subprocess

UPSTREAM_VERSION = '2025-10-15T17:29:55Z'
ASSIGNMENT = 'FORK_VERSION="impreza-s3-server.${COMMIT:0:12}"'


def digest(path):
    with path.open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def run(argv, cwd, log):
    with log.open('xb') as stream:
        process = subprocess.Popen([str(x) for x in argv], cwd=cwd, stdout=stream,
                                   stderr=subprocess.STDOUT, start_new_session=True)
        try:
            code = process.wait(timeout=3600)
        except (subprocess.TimeoutExpired, KeyboardInterrupt):
            os.killpg(process.pid, signal.SIGKILL)
            process.wait()
            raise RuntimeError('build interrupted; outputs retained') from None
    if code:
        raise RuntimeError('build failed; output and log retained')


def main():
    p = argparse.ArgumentParser()
    for name in ('source', 'go', 'module-cache', 'output'):
        p.add_argument('--' + name, type=Path, required=True)
    args = p.parse_args()
    src, go, cache, out = (v.resolve() for v in (args.source, args.go, args.module_cache, args.output))
    if out.exists() or out.is_relative_to(src) or not go.is_file() or not cache.is_dir():
        raise ValueError('fresh external output and existing compiler/cache required')
    source_commit = subprocess.check_output(['git', '--no-replace-objects', '-C', str(src), 'rev-parse', 'HEAD']).decode().strip()
    script = (src / 'engineering/build/build.sh').read_text()
    if script.splitlines().count(ASSIGNMENT) != 1:
        raise ValueError('unrecognized fork identity recipe')
    # Exact single-line replacement; no source commit, ReleaseTag or other flags change.
    control_script = script.replace(ASSIGNMENT, 'FORK_VERSION="' + UPSTREAM_VERSION + '"')
    out.mkdir(mode=0o700)
    control = out / 'control-build.sh'
    control.write_text(control_script)
    control.chmod(0o700)
    report = {'source_commit': source_commit, 'control_version': UPSTREAM_VERSION,
              'fork_version': 'impreza-s3-server.' + source_commit[:12],
              'status': 'INCOMPLETE', 'runtime': 'NOT_RUN', 'release': False}
    def save():
        (out / 'identity-proof.json').write_text(json.dumps(report, sort_keys=True, indent=2) + '\n')
    save()
    for name, recipe in [('control', control), ('fork-a', src / 'engineering/build/build.sh'),
                         ('fork-b', src / 'engineering/build/build.sh')]:
        run(['bash', recipe, src, out / name, go, cache], src, out / (name + '.log'))
    # The existing comparator checks bytes, buildinfo, build parameters and their seals.
    run(['bash', src / 'engineering/build/compare.sh', out / 'fork-a', out / 'fork-b'], src, out / 'compare.log')
    old = (out / 'control/build-params.txt').read_text()
    new = (out / 'fork-a/build-params.txt').read_text()
    if old.count('cmd.Version=' + UPSTREAM_VERSION) != 1:
        raise ValueError('control version missing or ambiguous')
    normalized = old.replace('cmd.Version=' + UPSTREAM_VERSION, 'cmd.Version=' + report['fork_version'])
    if normalized != new:
        raise ValueError('inputs differ beyond Version')
    hashes = {name: digest(out / name / 'impreza-s3-server') for name in ('control', 'fork-a', 'fork-b')}
    if hashes['control'] == hashes['fork-a'] or hashes['fork-a'] != hashes['fork-b']:
        raise ValueError('identity effect or reproducibility not established')
    report.update(status='VERSION_ONLY_INPUT_CHANGE_PROVEN', executable_sha256=hashes,
                  build_recipe_sha256=digest(src / 'engineering/build/build.sh'),
                  control_recipe_sha256=digest(control))
    save()
    print(json.dumps(report, sort_keys=True))


if __name__ == '__main__':
    main()

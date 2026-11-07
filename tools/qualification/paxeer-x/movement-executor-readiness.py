#!/usr/bin/env python3
"""Movement readiness reports ready only with an authenticated, usable executor.

--build compiles the named targets once and records them in a private build
manifest. Without arguments the gate checks its prerequisites and that
manifest, then runs the named readiness tests from those prebuilt executables
against a real Human KMS process, two TLS chain origins and the retained KMS
executor, retaining every log and fixture under a private evidence directory.
It never compiles.
"""
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile

ROOT = Path(__file__).resolve().parents[3]
SCHEMA = 'paxeer-x.movement-executor-readiness.v1'
MANIFEST = Path(os.environ.get('PAXEER_X_MOVEMENT_READINESS_MANIFEST',
                               ROOT / 'services/human/target/paxeer-x/movement-executor-readiness.json'))
SOURCES = ['services/human/Cargo.toml', 'services/human/Cargo.lock', 'services/human/crates', 'services/agent/crates',
           'services/platform/Cargo.toml', 'services/platform/Cargo.lock', 'services/platform/hosted/runtime-clock',
           'services/interop', 'programs', 'tools/runtime/run-with-clock.sh',
           'tools/qualification/paxeer-x/movement-executor-readiness.py']
CASES = [
    ('probe', 'proof_human_kms'),
    ('kms_cases', 'actual_client_lifecycle_and_mutual_tls'),
    ('deadline_cases', 'fragmented_handshake_and_frame_release_all_permits'),
    ('deadline_cases', 'store_admission_has_the_same_deadline'),
    ('kms_cases', 'atomic_rotation_lost_response_restart_and_tombstones'),
    ('kms_cases', 'executor_certificate_cannot_authorize_or_manage_keys'),
    ('kms_cases', 'external_signature_verification_and_executor_journal_recovery'),
]
ARTIFACTS = frozenset({'probe', 'kms_cases', 'deadline_cases', 'kms', 'clock'})

PASSED = re.compile(r'^test result: ok\. 1 passed; 0 failed; 0 ignored; 0 measured; \d+ filtered out;', re.M)


def refuse(reason):
    raise RuntimeError(reason)


def digest(path):
    h = hashlib.sha256()
    with Path(path).open('rb') as stream:
        for block in iter(lambda: stream.read(1 << 20), b''):
            h.update(block)
    return h.hexdigest()


def git(*args):
    return subprocess.check_output(['git', '-C', str(ROOT), *args], text=True).strip()


def source():
    if git('status', '--porcelain', '--untracked-files=all', '--', *SOURCES):
        refuse('the qualified sources have uncommitted changes')
    return git('rev-parse', 'HEAD')


def artifacts(argv, wanted):
    result = subprocess.run(argv, cwd=ROOT, text=True, stdout=subprocess.PIPE, stdin=subprocess.DEVNULL)
    if result.returncode:
        refuse('build failed (%d): %s' % (result.returncode, ' '.join(argv)))
    found = {}
    for line in result.stdout.splitlines():
        try:
            item = json.loads(line)
        except ValueError:
            continue
        if item.get('reason') != 'compiler-artifact' or not item.get('executable'):
            continue
        key = (item['target']['name'], tuple(item['target']['kind']), bool(item['profile']['test']))
        if key in wanted:
            found[wanted[key]] = {'path': str(Path(item['executable']).resolve()),
                                  'sha256': digest(item['executable'])}
    if set(found) != set(wanted.values()):
        refuse('build did not produce: ' + ', '.join(sorted(set(wanted.values()) - set(found))))
    return found


def build():
    revision = source()
    human = ['--locked', '--manifest-path', 'services/human/Cargo.toml', '--message-format=json']
    built = {}
    built.update(artifacts(['cargo', 'test', '--no-run', '-p', 'layerx-human-kms',
                            '--test', 'proof_human_kms', '--test', 'provider',
                            '--test', 'repair_22_33_25', *human], {
        ('proof_human_kms', ('test',), True): 'probe',
        ('provider', ('test',), True): 'kms_cases',
        ('repair_22_33_25', ('test',), True): 'deadline_cases',
    }))
    built.update(artifacts(['cargo', 'build', '-p', 'layerx-human-kms', '--bin', 'layerx-human-kms', *human], {
        ('layerx-human-kms', ('bin',), False): 'kms',
    }))
    built.update(artifacts(['cargo', 'build', '--locked', '--manifest-path', 'services/platform/Cargo.toml',
                            '-p', 'layerx-runtime-clock', '--bin', 'layerx-runtime-clock',
                            '--message-format=json'], {
        ('layerx-runtime-clock', ('bin',), False): 'clock',
    }))
    if source() != revision:
        refuse('sources changed during the build')
    MANIFEST.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    temporary = MANIFEST.with_suffix('.next')
    fd = os.open(temporary, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    with os.fdopen(fd, 'w') as stream:
        json.dump({'schema': SCHEMA, 'revision': revision, 'artifacts': built}, stream, indent=2)
        stream.write('\n')
    os.replace(temporary, MANIFEST)
    print('PAXEER_X_BUILD manifest=%s revision=%s' % (MANIFEST, revision))


def prerequisites():
    for tool in ['openssl', 'socat', 'git']:
        if not shutil.which(tool):
            refuse('prerequisite missing: ' + tool)
    if not (Path('/root/.foundry/bin/anvil').is_file() or shutil.which('anvil')):
        refuse('prerequisite missing: anvil')
    if not os.access('/usr/bin/setpriv', os.X_OK):
        refuse('prerequisite missing: /usr/bin/setpriv for the runtime clock supervisor')
    if not MANIFEST.is_file():
        refuse('no build manifest at %s; run this gate with --build first' % MANIFEST)
    info = MANIFEST.stat()
    if info.st_uid != os.geteuid() or info.st_mode & 0o077:
        refuse('the build manifest must be private and owned by the gate user')
    data = json.loads(MANIFEST.read_text())
    if data.get('schema') != SCHEMA or data.get('revision') != source():
        refuse('the prebuilt executables were not built from this revision; run --build')
    built = admit_artifacts(data, source())
    return built


def admit_artifacts(data, revision):
    if data.get('schema') != SCHEMA or data.get('revision') != revision:
        refuse('the retained KMS executables were not built from this revision')
    built = data['artifacts']
    if set(built) != ARTIFACTS:
        refuse('exact retained KMS executor artifact inventory required')
    for name, row in built.items():
        path = Path(row['path'])
        if not (path.is_absolute() and path.is_file() and not path.is_symlink() and os.access(path, os.X_OK)):
            refuse('prebuilt executable missing: ' + name)
        if digest(path) != row['sha256']:
            refuse('prebuilt executable changed since the build: ' + name)
    return built


def run(built):
    evidence = os.environ.get('PAXEER_X_EVIDENCE_DIR')
    if evidence:
        evidence = Path(evidence)
        evidence.mkdir(mode=0o700, parents=True, exist_ok=False)
    else:
        evidence = Path(tempfile.mkdtemp(prefix='movement-executor-readiness-'))
    evidence.chmod(0o700)
    for name in ['tmp', 'clock']:
        (evidence / name).mkdir(mode=0o700)
    env = {key: value for key, value in os.environ.items()
           if not key.startswith(('LAYERX_', 'PAXEER_X_', 'CARGO_', 'RUST'))}
    env.update(TMPDIR=str(evidence / 'tmp'), PAXEER_X_RETAIN_STATE='1',
               LAYERX_RUNTIME_CLOCK_BIN=built['clock']['path'],
               LAYERX_RUNTIME_CLOCK_DIRECTORY=str(evidence / 'clock'))
    print('PAXEER_X_EVIDENCE dir=%s' % evidence, flush=True)
    passed = 0
    for target, test in CASES:
        log = evidence / (test.replace('::', '-') + '.log')
        try:
            result = subprocess.run(['sh', str(ROOT / 'tools/runtime/run-with-clock.sh'), built[target]['path'],
                                     test, '--exact', '--nocapture', '--test-threads=1'],
                                    cwd=ROOT, env=env, text=True, stdout=subprocess.PIPE,
                                    stderr=subprocess.STDOUT, stdin=subprocess.DEVNULL, timeout=300)
            output, code = result.stdout, result.returncode
        except subprocess.TimeoutExpired as error:
            output = error.stdout.decode() if isinstance(error.stdout, bytes) else (error.stdout or '')
            code = 'timeout'
        log.write_text(output)
        if code != 0 or not PASSED.search(output):
            refuse('%s failed (%s); log %s' % (test, code, log))
        passed += 1
        print('PAXEER_X_CASE %s ok' % test, flush=True)
    print('PAXEER_X_GATE tests=%d skipped=0 evidence=%s' % (passed, evidence))


if __name__ == '__main__':
    try:
        if sys.argv[1:] == ['--build']:
            build()
        elif sys.argv[1:]:
            refuse('expected no arguments or --build')
        else:
            run(prerequisites())
    except (OSError, ValueError, KeyError, RuntimeError, subprocess.SubprocessError) as error:
        print('movement executor readiness refused:', error, file=sys.stderr)
        sys.exit(1)

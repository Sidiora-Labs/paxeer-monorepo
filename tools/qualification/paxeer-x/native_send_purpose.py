#!/usr/bin/env python3
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import time

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / 'tools/paxeer-x'))
from candidate import SERVICE_CATALOGUE, catalogue, load_private, validate

spec = importlib.util.spec_from_file_location('human_boundary', ROOT / 'tools/qualification/paxeer-x/human-api-boundary.py')
B = importlib.util.module_from_spec(spec)
spec.loader.exec_module(B)
B.DEADLINE = time.monotonic() + 1180
if os.environ.get('PAXEER_X_TASK_DEADLINE_UNIX'):
    B.DEADLINE = min(B.DEADLINE, time.monotonic() + float(os.environ['PAXEER_X_TASK_DEADLINE_UNIX']) - time.time())


def inventory(root=None):
    root = ROOT if root is None else Path(root)
    roots = tuple(root / name for name in (
        'services/human', 'services/agent', 'services/wallet',
        'services/platform', 'services/interop', 'programs',
        'sdk', 'core', 'layerxproof', 'modules', 'precompiles',
        'admin', 'api', 'consensus', 'custodyproof', 'daemon', 'include',
        'interchain', 'node', 'rpc', 'src', 'storage', 'utils', 'wasm', 'wasm-runtime'))
    paths = {root / 'tools/qualification/paxeer-x/native_send_purpose.py',
             root / 'apps/web/e2e/software-authenticator.ts'}
    paths.add(root / 'services/agent/schema/agent-api/v1.kvx')
    paths.update(root / name for name in ('go.mod', 'go.sum', 'go.work',
                                        'go.work.sum', 'rust-toolchain.toml')
                 if (root / name).is_file())
    for source_root in roots:
        for directory, children, files in os.walk(source_root):
            children[:] = [name for name in children if name not in (
                'target', 'node_modules', '.git', '.next', '__pycache__', '.pytest_cache', '.venv')]
            # Rust include_bytes!/include_str! and Go embed accept arbitrary suffixes.
            for name in files:
                paths.add(Path(directory) / name)
    return {str(path.relative_to(root)): B.digest(path) for path in sorted(paths)}


def source_digest(files):
    value = hashlib.sha256()
    for name, digest in sorted(files.items()):
        value.update(name.encode() + b'\0' + digest.encode() + b'\n')
    return value.hexdigest()


def run(command, log, cwd, environment):
    with log.open('xb') as output:
        os.chmod(log, 0o600)
        result = subprocess.run(command, cwd=cwd, env=environment, stdin=subprocess.DEVNULL,
                                stdout=output, stderr=subprocess.STDOUT, timeout=B.remaining())
    print('command=' + json.dumps(command) + ' exit=' + str(result.returncode) + ' log=' + str(log), flush=True)
    B.require(result.returncode == 0, 'bounded native custody command failed')


def admit_build(material, candidate_revision, revision=None, root=None):
    B.require(material['schema'] == 'layerx-human-native-send-purpose-build.v1', 'closed protected real-process build authority')
    files = inventory(root)
    if revision is None:
        revision = B.git('rev-parse', 'HEAD').decode().strip()
    source = source_digest(files)
    B.require(material['source_files'] == files and material['source_digest'] == source
              and material['source_revision'] == revision and candidate_revision == revision,
              'actual frozen candidate source binding')
    artifacts = {name: B.executable(row, revision, source) for name, row in material['artifacts'].items()}
    B.require(set(artifacts) == {'attestor', 'go_corpus', 'rust_corpus'}, 'actual compiled production daemon and focused real consumers')
    return artifacts


def main():
    parser = argparse.ArgumentParser()
    mode = parser.add_mutually_exclusive_group()
    mode.add_argument('--build', action='store_true')
    mode.add_argument('--candidate-manifest')
    args = parser.parse_args()
    B.require(os.environ.get('PAXEER_X_TASK_DEADLINE_UNIX'), 'original task deadline required')
    if not args.build and not args.candidate_manifest:
        args.candidate_manifest = os.environ.get('PAXEER_X_CANDIDATE_MANIFEST')
        B.require(args.candidate_manifest, 'actual published candidate manifest required')
    destination = Path(os.environ.get('PAXEER_X_NATIVE_SEND_ARTIFACT_DIRECTORY',
                                      '/root/lx-ops/paxeer-x-integration-2026-10-03/native-send-purpose-artifacts'))
    if args.build:
        destination.mkdir(mode=0o700, parents=True, exist_ok=False)
        B.private(str(destination), True)
        B.require(not B.git('status', '--porcelain', '--untracked-files=all').strip(), 'clean root-intaken immutable candidate required')
        files = inventory()
        revision = B.git('rev-parse', 'HEAD').decode().strip()
        source = source_digest(files)
        B.require(os.environ.get('PAXEER_X_MAINLINE') == revision, 'exact published mainline source pin required')
        environment = dict(os.environ, CARGO_BUILD_JOBS='4', CARGO_TARGET_DIR='/root/lx-target/human',
                           GOMAXPROCS='4', PATH='/root/lx-toolchains/node24/bin:/usr/local/go/bin:/root/.cargo/bin:' + os.environ.get('PATH', ''))
        run(['/usr/local/go/bin/go', 'build', '-mod=readonly', '-p', '4', '-o', str(destination / 'attestor'), './cmd/attestor'],
            destination / 'go-daemon-build.log', ROOT / 'services/wallet/attestor', environment)
        run(['/usr/local/go/bin/go', 'test', '-mod=readonly', '-p', '4', '-c', '-o', str(destination / 'native-server.test'), './internal/server'],
            destination / 'go-corpus-build.log', ROOT / 'services/wallet/attestor', environment)
        command = ['/root/.cargo/bin/cargo', 'test', '--locked', '--manifest-path', 'services/human/Cargo.toml',
                   '-p', 'layerx-human-service', '--test', 'native_send_purpose', '--no-run', '--message-format=json']
        run(command, destination / 'rust-corpus-build.log', ROOT, environment)
        executables = []
        for line in (destination / 'rust-corpus-build.log').read_text().splitlines():
            try:
                row = json.loads(line)
            except ValueError:
                continue
            if row.get('reason') == 'compiler-artifact' and row.get('target', {}).get('name') == 'native_send_purpose' and row.get('executable'):
                executables.append(Path(row['executable']))
        B.require(len(executables) == 1, 'one real focused Rust executable')
        B.require(files == inventory(), 'source remained frozen through the bounded build')
        artifacts = {}
        for role, path in [('attestor', destination / 'attestor'), ('go_corpus', destination / 'native-server.test'), ('rust_corpus', executables[0])]:
            artifacts[role] = dict(path=str(path), sha256=B.digest(path), source_revision=revision, source_digest=source, build_exit=0)
        document = dict(schema='layerx-human-native-send-purpose-build.v1', source_files=files,
                        source_revision=revision, source_digest=source, artifacts=artifacts)
        manifest = destination / 'build.json'
        with manifest.open('x') as output:
            os.chmod(manifest, 0o600)
            json.dump(document, output)
        print('native-send-purpose build manifest=' + str(manifest))
        return
    candidate = load_private(args.candidate_manifest)
    validate(candidate, catalogue(ROOT / SERVICE_CATALOGUE))
    material = B.load(os.environ.get('PAXEER_X_NATIVE_SEND_BUILD_MANIFEST', str(destination / 'build.json')))
    artifacts = admit_build(material, candidate['source']['revision'])
    environment = dict(os.environ, PAXEER_X_NATIVE_SEND_RUST_TEST=str(artifacts['rust_corpus']),
                       PATH='/root/lx-toolchains/node24/bin:/usr/local/go/bin:/root/.cargo/bin:' + os.environ.get('PATH', ''))
    log = destination / 'native-send-purpose-verify.log'
    run([str(artifacts['go_corpus']), '-test.run=^TestNativeSendPurposeRealProfiles$', '-test.count=1', '-test.v',
         '-test.timeout=' + str(max(1, int(B.remaining()))) + 's'], log, ROOT / 'services/wallet/attestor', environment)
    observed = log.read_text()
    B.require('--- PASS: TestNativeSendPurposeRealProfiles' in observed and 'native_send_provider_and_sdk_real_cluster ... ok' in observed
              and 'native custody genuine passkey issuance and grant consent verified' in observed
              and 'native custody principal digest forgery ceremony-replay and consumed-evidence refusals verified' in observed
              and '\nPASS\n' in observed and 'SKIP' not in observed,
              'both real Go corpus and actual Rust custody/provider consumer ran without skips')
    B.require(material['source_files'] == inventory()
              and material['source_revision'] == B.git('rev-parse', 'HEAD').decode().strip(),
              'source remained frozen through native qualification')
    print('human-native-send-purpose: PASS revision=' + material['source_revision'])


if __name__ == '__main__':
    try:
        main()
    except (OSError, RuntimeError, ValueError, subprocess.TimeoutExpired) as error:
        print('human-native-send-purpose: FAIL ' + str(error), file=sys.stderr)
        sys.exit(1)

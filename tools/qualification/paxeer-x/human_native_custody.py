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
sys.path.insert(0, str(Path(__file__).resolve().parent))
import candidate_build as BUILD

sys.path.insert(0, str(ROOT / 'tools/paxeer-x'))
from candidate import SERVICE_CATALOGUE, catalogue, load_private, validate

spec = importlib.util.spec_from_file_location('human_boundary', ROOT / 'tools/qualification/paxeer-x/human-api-boundary.py')
B = importlib.util.module_from_spec(spec)
spec.loader.exec_module(B)
B.DEADLINE = time.monotonic() + 1180
if os.environ.get('PAXEER_X_TASK_DEADLINE_UNIX'):
    B.DEADLINE = min(B.DEADLINE, time.monotonic() + float(os.environ['PAXEER_X_TASK_DEADLINE_UNIX']) - time.time())


def inventory(root=ROOT):
    root = Path(root)
    roots = tuple(root / name for name in (
        'services/human', 'services/agent', 'services/wallet', 'services/platform',
        'services/interop', 'programs', 'api', 'app', 'chain', 'cmd', 'core',
        'modules', 'node', 'rpc', 'pkg', 'proto', 'layerxproof', 'tests', '.cargo'))
    paths = {root / 'tools/qualification/paxeer-x/human_native_custody.py',
             root / 'apps/web/e2e/software-authenticator.ts'}
    paths.update(root / name for name in ('go.mod', 'go.sum', 'go.work', 'go.work.sum')
                 if (root / name).exists())
    for name in ('services/human', 'services/agent', 'services/wallet'):
        B.require((root / name).is_dir(), 'production source root missing: ' + name)
    for source_root in roots:
        for directory, children, files in os.walk(source_root):
            children[:] = [name for name in children if name not in (
                'target', 'node_modules', '.git', '.next', '__pycache__', '.venv')]
            B.require(not any((Path(directory) / name).is_symlink() for name in children),
                      'source inventory cannot omit symlinked directories')
            for name in files:
                paths.add(Path(directory) / name)
    for path in paths:
        B.require(path.is_file() and not path.is_symlink(), 'source inventory requires regular files: ' + str(path))
    return {str(path.relative_to(root)): B.digest(path) for path in sorted(paths)}


def cargo_test_artifacts(output, targets):
    found = {target: [] for target in targets}
    for line in output.splitlines():
        try:
            row = json.loads(line)
        except ValueError:
            continue
        target = row.get('target', {})
        name = target.get('name')
        if (row.get('reason') == 'compiler-artifact' and name in found
                and target.get('kind') == ['test'] and row.get('profile', {}).get('test') is True
                and row.get('executable')):
            found[name].append(row['executable'])
    B.require(all(len(paths) == 1 for paths in found.values()),
              'one compiler-emitted executable required for every test target')
    return {name: paths[0] for name, paths in found.items()}


def publish_test_manifest(path, before, after, command, code, log, artifacts, root=ROOT):
    path = Path(path).absolute()
    B.require(before == after, 'source changed during test build')
    BUILD.verify_files(root, before)
    for executable in artifacts.values():
        executable = Path(executable)
        B.require(executable.is_absolute() and not executable.is_symlink()
                  and executable.is_file() and os.access(executable, os.X_OK),
                  'compiler-emitted executable required')
    sidecar = path.with_name(path.name + '.build.json')
    reference = BUILD.publish(sidecar, before, after, command, code, log, artifacts.values())
    revision = before['candidate']['revision']
    source = BUILD.source_digest(before)
    value = {'schema': 'paxeer-x.test-build.v1', 'source': before, 'artifacts': {
        name: {'path': str(Path(executable).resolve(strict=True)),
               'sha256': BUILD.file_digest(executable), 'source_revision': revision,
               'source_digest': source, 'build_exit': code, 'build_sidecar': reference}
        for name, executable in artifacts.items()}}
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(fd, 'w') as stream:
        json.dump(value, stream, sort_keys=True)
        stream.write('\n')
        stream.flush()
        os.fsync(stream.fileno())
    return path


def admit_test_manifest(path, source, targets, root=ROOT):
    value = BUILD.private_json(Path(path).absolute())
    B.require(value.get('schema') == 'paxeer-x.test-build.v1' and value.get('source') == source,
              'test build manifest does not bind the complete current revision and source')
    BUILD.verify_files(root, source)
    B.require(set(value.get('artifacts', {})) == set(targets), 'exact compiled test target inventory required')
    revision, digest = source['candidate']['revision'], BUILD.source_digest(source)
    artifacts = {name: BUILD.admit(row, revision, digest, root)
                 for name, row in value['artifacts'].items()}
    B.require(all(os.access(path, os.X_OK) for path in artifacts.values()),
              'admitted test artifact is not executable')
    return artifacts


def build_test_targets(path, targets, target_directory):
    path = Path(path).absolute()
    path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    before = BUILD.source_identity(ROOT)
    command = ['cargo', 'test', '--locked', '--manifest-path', str(ROOT / 'services/agent/Cargo.toml'),
               '-p', 'layerx-agentd', '--target-dir', str(target_directory), '--no-run', '--message-format=json']
    for target in targets:
        command.extend(['--test', target])
    log = path.with_name(path.name + '.log')
    fd = os.open(log, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(fd, 'w') as stream:
        result = subprocess.run(command, cwd=ROOT, stdin=subprocess.DEVNULL, stdout=stream,
                                stderr=subprocess.STDOUT, timeout=1140, check=False)
    B.require(result.returncode == 0, 'test build failed; log ' + str(log))
    artifacts = cargo_test_artifacts(log.read_text(), targets)
    return publish_test_manifest(path, before, BUILD.source_identity(ROOT), command,
                                 result.returncode, log, artifacts)


def source_digest(files):
    value = hashlib.sha256()
    for name, digest in sorted(files.items()):
        value.update(name.encode() + b'\0' + digest.encode() + b'\n')
    return value.hexdigest()


def custody_source_record(files, revision):
    return {'source_files': files, 'source_revision': revision, 'source_digest': source_digest(files)}


def admit_custody_source(material, files, revision, candidate_revision):
    expected = custody_source_record(files, revision)
    B.require(all(material.get(name) == value for name, value in expected.items())
              and candidate_revision == revision, 'actual frozen candidate source binding')
    return expected['source_digest']


def run(command, log, cwd, environment):
    with log.open('xb') as output:
        os.chmod(log, 0o600)
        result = subprocess.run(command, cwd=cwd, env=environment, stdin=subprocess.DEVNULL,
                                stdout=output, stderr=subprocess.STDOUT, timeout=B.remaining())
    print('command=' + json.dumps(command) + ' exit=' + str(result.returncode) + ' log=' + str(log), flush=True)
    B.require(result.returncode == 0, 'bounded native custody command failed')


def main():
    parser = argparse.ArgumentParser()
    mode = parser.add_mutually_exclusive_group(required=True)
    mode.add_argument('--build', action='store_true')
    mode.add_argument('--candidate-manifest')
    args = parser.parse_args()
    destination = Path(os.environ.get('PAXEER_X_NATIVE_CUSTODY_ARTIFACT_DIRECTORY',
                                      '/root/lx-ops/paxeer-x-integration-2026-10-03/native-custody-artifacts'))
    if args.build:
        destination.mkdir(mode=0o700, parents=True, exist_ok=False)
        B.private(str(destination), True)
        files = inventory()
        revision = B.git('rev-parse', 'HEAD').decode().strip()
        source = source_digest(files)
        environment = dict(os.environ, CARGO_BUILD_JOBS='4', CARGO_TARGET_DIR='/root/lx-target/human')
        run(['go', 'build', '-mod=readonly', '-o', str(destination / 'attestor'), './cmd/attestor'],
            destination / 'go-daemon-build.log', ROOT / 'services/wallet/attestor', environment)
        run(['go', 'test', '-mod=readonly', '-c', '-o', str(destination / 'native-server.test'), './internal/server'],
            destination / 'go-corpus-build.log', ROOT / 'services/wallet/attestor', environment)
        command = ['/root/.cargo/bin/cargo', 'test', '--locked', '--manifest-path', 'services/human/Cargo.toml',
                   '-p', 'layerx-human-kms', '--test', 'attestor_native_profile', '--no-run', '--message-format=json']
        run(command, destination / 'rust-corpus-build.log', ROOT, environment)
        executables = []
        for line in (destination / 'rust-corpus-build.log').read_text().splitlines():
            try:
                row = json.loads(line)
            except ValueError:
                continue
            if row.get('reason') == 'compiler-artifact' and row.get('target', {}).get('name') == 'attestor_native_profile' and row.get('executable'):
                executables.append(Path(row['executable']))
        B.require(len(executables) == 1, 'one real focused Rust executable')
        B.require(files == inventory(), 'source remained frozen through the bounded build')
        artifacts = {}
        for role, path in [('attestor', destination / 'attestor'), ('go_corpus', destination / 'native-server.test'), ('rust_corpus', executables[0])]:
            artifacts[role] = dict(path=str(path), sha256=B.digest(path), source_revision=revision, source_digest=source, build_exit=0)
        document = dict(schema='layerx-human-native-custody-build.v1',
                        **custody_source_record(files, revision), artifacts=artifacts)
        manifest = destination / 'build.json'
        with manifest.open('x') as output:
            os.chmod(manifest, 0o600)
            json.dump(document, output)
        print('native-custody build manifest=' + str(manifest))
        return
    candidate = load_private(args.candidate_manifest)
    validate(candidate, catalogue(ROOT / SERVICE_CATALOGUE))
    material = B.load(os.environ.get('PAXEER_X_NATIVE_CUSTODY_BUILD_MANIFEST', str(destination / 'build.json')))
    B.require(material['schema'] == 'layerx-human-native-custody-build.v1', 'closed protected real-process build authority')
    files = inventory()
    revision = B.git('rev-parse', 'HEAD').decode().strip()
    source = admit_custody_source(material, files, revision, candidate['source']['revision'])
    artifacts = {name: B.executable(row, revision, source) for name, row in material['artifacts'].items()}
    B.require(set(artifacts) == {'attestor', 'go_corpus', 'rust_corpus'}, 'actual compiled production daemon and focused real consumers')
    environment = dict(os.environ, PAXEER_X_NATIVE_CUSTODY_RUST_TEST=str(artifacts['rust_corpus']))
    log = destination / 'native-custody-verify.log'
    run([str(artifacts['go_corpus']), '-test.run=^TestNativeCustody', '-test.count=1', '-test.v',
         '-test.timeout=' + str(max(1, int(B.remaining()))) + 's'], log, ROOT / 'services/wallet/attestor', environment)
    observed = log.read_text()
    B.require('--- PASS: TestNativeCustodyRealProfiles' in observed and 'native_provider_and_sdk_real_cluster ... ok' in observed
              and 'native custody genuine passkey issuance and grant consent verified' in observed
              and 'native custody principal digest forgery ceremony-replay and consumed-evidence refusals verified' in observed
              and '\nPASS\n' in observed and 'SKIP' not in observed,
              'both real Go corpus and actual Rust custody/provider consumer ran without skips')
    print('human-native-custody: PASS revision=' + revision)


if __name__ == '__main__':
    try:
        main()
    except (OSError, RuntimeError, ValueError, subprocess.TimeoutExpired) as error:
        print('human-native-custody: FAIL ' + str(error), file=sys.stderr)
        sys.exit(1)

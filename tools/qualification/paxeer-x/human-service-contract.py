#!/usr/bin/env python3
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import sys
import tempfile

sys.dont_write_bytecode = True
ROOT = Path(__file__).resolve().parents[3]
RECORD_NAME = 'task-104.13.24-artifacts.json'
CLOCK_RECORD_NAME = 'task-104.13.25-artifacts.json'
V1_GATE_DIGEST = 'c13cff140997c8cbde2a6602221ce92df850ab8736c47fbc9576f92678673549'
SOURCES = (
    'services/human/crates/layerx-human-service/src/server/agent_runtime.rs',
    'services/human/crates/layerx-human-service/src/server/production_rotation.rs',
    'services/human/crates/layerx-human-service/src/server/production_components.rs',
    'services/human/crates/layerx-human-service/src/server/http.rs',
    'services/human/crates/layerx-human-service/tests/server_browser_boundary.rs',
    'services/human/crates/layerx-human-service/Cargo.toml',
    'services/human/Cargo.toml', 'services/human/Cargo.lock',
    'tools/qualification/paxeer-x/human-service-contract.py',
)
CLOCK_SOURCES = (
    'services/platform/hosted/runtime-clock/src/main.rs',
    'services/platform/hosted/runtime-clock/src/server.rs',
    'services/platform/hosted/runtime-clock/Cargo.toml',
    'services/platform/Cargo.toml', 'services/platform/Cargo.lock',
    'services/agent/crates/layerx-types/src/clock.rs',
    'services/agent/crates/layerx-types/src/clock_protocol.rs',
    'services/agent/crates/layerx-client/src/runtime_clock.rs',
    'tools/runtime/run-with-clock.sh',
)


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def digest(path):
    with path.open('rb') as source:
        return hashlib.file_digest(source, 'sha256').hexdigest()


def source_hashes():
    return {source: digest(ROOT / source) for source in SOURCES}


def clock_source_hashes():
    return {source: digest(ROOT / source) for source in (*SOURCES, *CLOCK_SOURCES)}


def protected_record(path):
    info = path.lstat()
    require(stat.S_ISREG(info.st_mode) and 0 < info.st_size <= 65536
            and info.st_uid == os.geteuid() and info.st_mode & 0o077 == 0,
            'protected compiler artifact record required')
    return json.loads(path.read_text())


def check_artifacts(artifacts, names):
    require(set(artifacts) == names, 'closed artifact profile required')
    for artifact in artifacts.values():
        path = Path(artifact['path'])
        require(path.is_absolute() and path.is_file() and os.access(path, os.X_OK)
                and digest(path) == artifact['sha256'], 'compiler artifact was changed or unavailable')


def save_record(path, record):
    path.parent.mkdir(parents=True, exist_ok=True)
    descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_TRUNC | os.O_NOFOLLOW, 0o600)
    with os.fdopen(descriptor, 'w') as output:
        os.fchmod(output.fileno(), 0o600)
        json.dump(record, output, indent=2)
        output.write('\n')


def record_paths(directory, create=False):
    directory = directory.expanduser().absolute()
    if create:
        directory.mkdir(mode=0o700, parents=True, exist_ok=True)
    info = directory.lstat()
    require(stat.S_ISDIR(info.st_mode) and info.st_uid == os.geteuid()
            and info.st_mode & 0o077 == 0, 'private owned evidence directory required')
    return directory / RECORD_NAME, directory / CLOCK_RECORD_NAME


def build_environment(target_dir=None):
    env = dict(os.environ)
    if target_dir is not None:
        env['CARGO_TARGET_DIR'] = str(target_dir)
    env.setdefault('CARGO_BUILD_JOBS', '4')
    return env


def compiler_command(cargo, clock=False):
    workspace = 'platform' if clock else 'human'
    package = 'layerx-runtime-clock' if clock else 'layerx-human-service'
    command = [cargo, 'build', '--locked', '--manifest-path',
               str(ROOT / 'services' / workspace / 'Cargo.toml'),
               '-p', package, '--bin', package]
    if not clock:
        command.extend(['--test', 'server_browser_boundary'])
    return command + ['--message-format=json-render-diagnostics']


def build(record_path, cargo, target_dir=None):
    hashes = source_hashes()
    command = compiler_command(cargo)
    env = build_environment(target_dir)
    result = subprocess.run(command, cwd=ROOT, env=env, capture_output=True,
                            timeout=1100, check=False)
    sys.stderr.buffer.write(result.stderr)
    artifacts = {}
    for line in result.stdout.splitlines():
        value = json.loads(line)
        if value.get('reason') == 'compiler-message':
            rendered = value.get('message', {}).get('rendered')
            if rendered:
                print(rendered, file=sys.stderr, end='')
        if value.get('reason') != 'compiler-artifact' or not value.get('executable'):
            continue
        target = value['target']
        if target['name'] == 'layerx-human-service' and 'bin' in target['kind']:
            artifacts['service'] = value['executable']
        elif target['name'] == 'server_browser_boundary' and 'test' in target['kind']:
            artifacts['boundary'] = value['executable']
    if result.returncode:
        return result.returncode
    require(hashes == source_hashes(), 'source changed during the declared build')
    require(set(artifacts) == {'service', 'boundary'}, 'actual service and boundary artifacts required')
    for path in artifacts.values():
        require(Path(path).is_file() and os.access(path, os.X_OK), 'compiler artifact unavailable')
    publish_service_record(record_path, hashes, artifacts)
    print('compiled genuine service and focused browser-boundary target')
    return 0


def publish_service_record(record_path, hashes, artifacts):
    record = {'schema': 'paxeer-x.human-service-contract.v1', 'sources': hashes,
              'artifacts': {name: {'path': path, 'sha256': digest(Path(path))}
                            for name, path in artifacts.items()}}
    save_record(record_path, record)


def load_service_record(record_path):
    inherited = protected_record(record_path)
    require(inherited.get('schema') == 'paxeer-x.human-service-contract.v1',
            'genuine prior service compiler record required')
    expected = source_hashes()
    if inherited.get('sources', {}).get('tools/qualification/paxeer-x/human-service-contract.py') == V1_GATE_DIGEST:
        expected['tools/qualification/paxeer-x/human-service-contract.py'] = V1_GATE_DIGEST
    require(inherited.get('sources') == expected,
            'prior service and boundary sources changed; clock-only build cannot replace them')
    check_artifacts(inherited.get('artifacts', {}), {'service', 'boundary'})
    return inherited


def build_clock(record_path, clock_record_path, cargo, target_dir=None):
    inherited = load_service_record(record_path)
    predecessor = digest(record_path)
    hashes = clock_source_hashes()
    env = build_environment(target_dir)
    command = compiler_command(cargo, clock=True)
    result = subprocess.run(command, cwd=ROOT, env=env, capture_output=True, timeout=540, check=False)
    sys.stderr.buffer.write(result.stderr)
    clock = None
    for line in result.stdout.splitlines():
        value = json.loads(line)
        if value.get('reason') == 'compiler-message':
            rendered = value.get('message', {}).get('rendered')
            if rendered:
                print(rendered, file=sys.stderr, end='')
        if value.get('reason') == 'compiler-artifact' and value.get('executable'):
            target = value['target']
            if target['name'] == 'layerx-runtime-clock' and 'bin' in target['kind']:
                require(clock is None, 'one genuine clock compiler artifact required')
                clock = value['executable']
    if result.returncode:
        return result.returncode
    require(hashes == clock_source_hashes() and predecessor == digest(record_path),
            'source or predecessor record changed during clock build')
    check_artifacts(inherited['artifacts'], {'service', 'boundary'})
    require(clock and Path(clock).is_file() and os.access(clock, os.X_OK),
            'actual clock compiler artifact required')
    publish_clock_record(clock_record_path, hashes, inherited['artifacts'], clock, predecessor)
    print('compiled genuine runtime clock; unchanged service and boundary artifacts retained')
    return 0


def publish_clock_record(clock_record_path, hashes, service_artifacts, clock, predecessor):
    artifacts = dict(service_artifacts)
    artifacts['clock'] = {'path': clock, 'sha256': digest(Path(clock))}
    save_record(clock_record_path, {'schema': 'paxeer-x.human-service-contract.v2',
                'sources': hashes, 'artifacts': artifacts,
                'predecessor_digest': predecessor})


def load_clock_record(record_path, clock_record_path):
    record = protected_record(clock_record_path)
    require(record.get('schema') == 'paxeer-x.human-service-contract.v2'
            and record.get('sources') == clock_source_hashes(),
            'clock-supervised artifact record does not bind current sources')
    require(record.get('predecessor_digest') == digest(record_path), 'service predecessor record changed')
    check_artifacts(record.get('artifacts', {}), {'service', 'boundary', 'clock'})
    return record


def verify(record_path, clock_record_path):
    record = load_clock_record(record_path, clock_record_path)
    env = {key: value for key, value in os.environ.items()
           if not key.startswith('LAYERX_RUNTIME_CLOCK_')}
    env['LAYERX_RUNTIME_CLOCK_BIN'] = record['artifacts']['clock']['path']
    with tempfile.TemporaryDirectory(prefix='lx-human-contract-', dir='/tmp') as private:
        os.chmod(private, 0o700)
        env['LAYERX_RUNTIME_CLOCK_DIRECTORY'] = private
        result = subprocess.run(['sh', str(ROOT / 'tools/runtime/run-with-clock.sh'),
                                 record['artifacts']['boundary']['path'], '--test-threads=1'],
                                cwd=ROOT, env=env, capture_output=True, timeout=480, check=False)
    sys.stdout.buffer.write(result.stdout)
    sys.stderr.buffer.write(result.stderr)
    require(result.returncode == 0 and re.search(
        rb'test result: ok\. 6 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out',
        result.stdout), 'all six existing boundary cases must execute and pass')
    require(record['sources'] == clock_source_hashes(), 'source changed during the focused verification')
    check_artifacts(record['artifacts'], {'service', 'boundary', 'clock'})
    return 0


def parse_args(argv=None):
    parser = argparse.ArgumentParser(
        description='Build and verify source-bound human-service compiler evidence.')
    parser.add_argument('--evidence-dir', type=Path, required=True,
                        help='private owned directory containing the service and clock build records')
    parser.add_argument('--cargo', default=os.environ.get('CARGO', 'cargo'),
                        help='Cargo executable (default: CARGO environment binding, then cargo on PATH)')
    parser.add_argument('--target-dir', type=Path,
                        help='Cargo output directory (default: CARGO_TARGET_DIR or Cargo configuration)')
    modes = parser.add_mutually_exclusive_group()
    modes.add_argument('--build', action='store_true')
    modes.add_argument('--build-clock', action='store_true')
    return parser.parse_args(argv)


def main(argv=None):
    args = parse_args(argv)
    record_path, clock_record_path = record_paths(
        args.evidence_dir, create=args.build or args.build_clock)
    if args.build:
        return build(record_path, args.cargo, args.target_dir)
    if args.build_clock:
        return build_clock(record_path, clock_record_path, args.cargo, args.target_dir)
    return verify(record_path, clock_record_path)


if __name__ == '__main__':
    try:
        sys.exit(main())
    except (RuntimeError, OSError, ValueError, subprocess.TimeoutExpired) as error:
        print('human service contract refused: ' + str(error), file=sys.stderr)
        sys.exit(1)

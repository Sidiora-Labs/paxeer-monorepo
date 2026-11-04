#!/usr/bin/env python3
import argparse
import importlib.util
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile

sys.dont_write_bytecode = True
definition = importlib.util.spec_from_file_location(
    'interface_producer', Path(__file__).with_name('program-interface-producer.py'))
producer = importlib.util.module_from_spec(definition)
definition.loader.exec_module(producer)
require = producer.require


def run(command, log, environment):
    fd = os.open(log, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(fd, 'wb') as stream:
        result = subprocess.run(command, cwd=producer.ROOT, env=environment,
                                stdin=subprocess.DEVNULL, stdout=stream,
                                stderr=subprocess.STDOUT, timeout=900)
    output = log.read_text(errors='replace')
    print(output, end='', flush=True)
    require(result.returncode == 0, f'command exited {result.returncode}; log={log}')
    return output


def rust_count(output, required):
    passed = re.findall(r'^test ([A-Za-z0-9_:]+) \.\.\. ok$', output, re.M)
    require(len(passed) == len(set(passed)) and required <= set(passed),
            'required Rust cases did not pass: ' + ', '.join(sorted(required - set(passed))))
    counts = re.findall(
        r'^test result: ok\. (\d+) passed; (\d+) failed; (\d+) ignored;', output, re.M)
    require(len(counts) == 1, 'expected exactly one Rust test summary')
    count, failed, ignored = map(int, counts[0])
    require(count == len(passed) and count >= len(required) and failed == 0 and ignored == 0,
            'Rust execution was empty, incomplete or skipped')
    return count


def admit_manifest(manifest, source):
    path = Path(manifest).absolute()
    producer.private_directory(path.parent)
    value = producer.load_private(path)
    require(value.get('schema') == producer.SCHEMA, 'unsupported interface artifact schema')
    require(value.get('source') == source, 'artifacts do not bind this clean candidate')
    require(value.get('required_protocol_cases') == sorted(producer.PROTOCOL_CASES)
            and value.get('required_legacy_cases') == sorted(producer.LEGACY_CASES),
            'incomplete required case inventory')
    require(value.get('profiles') == {'dev_debug': 0, 'test_debug': 0, 'incremental': False}
            and value.get('features') == {'sandbox': ['host-ffi'], 'registry-tests': []},
            'unexpected build profiles or features')
    artifacts = value['artifacts']
    require(set(artifacts) == producer.ARTIFACTS, 'incomplete artifact inventory')
    require(len(value['logs']) == producer.BUILD_LOG_COUNT, 'missing explicit build command logs')
    for saved in list(artifacts.values()) + value['logs']:
        require(producer.artifact(saved['path']) == saved,
                'artifact differs from producer: ' + saved['path'])
    return value


def native_fixtures(path, source, destination):
    require(path, 'retained native interface evidence manifest is required')
    path = Path(path).absolute()
    producer.private_directory(path.parent)
    value = producer.load_private(path)
    require(value.get('schema') == producer.NATIVE_EVIDENCE_SCHEMA
            and value.get('source') == source, 'native evidence does not bind this candidate')
    expected = {case + '/' + name for case in producer.NATIVE_FIXTURE_CASES
                for name in ('module.wasm', 'interface.bin', 'evidence.kvx')}
    require(set(value.get('artifacts', {})) == expected, 'native lifecycle evidence inventory')
    for relative, saved in value['artifacts'].items():
        require(producer.artifact(saved['path']) == saved, 'native evidence changed: ' + relative)
        target = destination / relative
        target.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
        if target.exists():
            require(target.read_bytes() == Path(saved['path']).read_bytes(),
                    'native evidence differs from emitted interface input: ' + relative)
        else:
            shutil.copyfile(saved['path'], target)
            target.chmod(0o600)


def interface_suites(artifacts):
    return [
        ('protocol', [artifacts['protocol-tests']['path'], '--skip', 'emit_native_inputs',
                      '--test-threads=1'], producer.PROTOCOL_CASES),
        ('legacy', [artifacts['legacy-tests']['path'], 'interface::conformance_vectors::',
                    '--test-threads=1'], producer.LEGACY_CASES),
    ]


def verify(manifest, native_manifest=None):
    path = Path(manifest).absolute()
    directory = producer.private_directory(path.parent)
    source = producer.identity()
    value = admit_manifest(path, source)
    artifacts = value['artifacts']
    for name in ('protocol-tests', 'legacy-tests'):
        require(os.access(artifacts[name]['path'], os.X_OK), 'artifact is not executable: ' + name)
    run_dir = Path(tempfile.mkdtemp(prefix='gate-', dir=directory))
    fixtures = run_dir / 'fixtures'
    fixtures.mkdir(mode=0o700)
    environment = dict(os.environ, PYTHONDONTWRITEBYTECODE='1',
                       PAXEER_X_INTERFACE_INPUTS=str(fixtures),
                       PAXEER_X_INTERFACE_FIXTURES=str(fixtures))
    binary = artifacts['protocol-tests']['path']
    count = rust_count(run([binary, '--exact', 'emit_native_inputs', '--test-threads=1'],
                          run_dir / 'inputs.log', environment), {'emit_native_inputs'})
    native_fixtures(native_manifest, source, fixtures)
    native_binding = producer.artifact(Path(native_manifest))
    for name, command, required in interface_suites(artifacts):
        count += rust_count(run(command, run_dir / (name + '.log'), environment), required)
    require(producer.identity() == source, 'source changed during gate')
    for saved in artifacts.values():
        require(producer.artifact(saved['path']) == saved, 'artifact changed during gate')
    require(producer.artifact(Path(native_manifest)) == native_binding,
            'native evidence manifest changed during gate')
    producer.write_private(run_dir / 'result.json', {
        'schema': 'paxeer-x.program-interface-result.v1', 'source': source,
        'manifest': producer.artifact(path), 'tests': count, 'skipped': 0,
        'native_cases': list(producer.NATIVE_FIXTURE_CASES),
        'native_manifest': native_binding,
        'logs': [producer.artifact(log) for log in sorted(run_dir.glob('*.log'))],
        'fixtures': [producer.artifact(file) for file in sorted(fixtures.rglob('*'))
                     if file.is_file()],
    })
    print(f'PAXEER_X_GATE tests={count} skipped=0')


def main():
    os.umask(0o077)
    parser = argparse.ArgumentParser()
    parser.add_argument('--manifest', required=True)
    parser.add_argument('--native-fixtures', required=True)
    args = parser.parse_args()
    try:
        verify(args.manifest, args.native_fixtures)
    except (OSError, ValueError, KeyError, TypeError, subprocess.SubprocessError) as error:
        print(f'interface gate refused: {error}', file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__':
    sys.exit(main())

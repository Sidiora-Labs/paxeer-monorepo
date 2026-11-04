#!/usr/bin/env python3
import argparse
import importlib.util
import hashlib
import json
import os
from pathlib import Path
import re
import signal
import subprocess
import sys
import tempfile
import urllib.request

sys.path.insert(0, str(Path(__file__).parent))
import merged_qualification as merged
from programs_language_abi import admitted_abis, unallocated_abi

sys.dont_write_bytecode = True
spec = importlib.util.spec_from_file_location(
    'interface_artifacts', Path(__file__).with_name('program-interface-producer.py'))
producer = importlib.util.module_from_spec(spec)
spec.loader.exec_module(producer)
require = producer.require
COMMAND = 'timeout 30m python3 tools/qualification/paxeer-x/programs_native_interfaces.py'
TEST = 'native_interfaces::canonical_native_interfaces_across_supported_abis'
SCHEMA = 'paxeer-x.native-interface-artifacts.v2'
SUFFIXES = {
    'deploy', 'registry', 'call', 'breaking-refused', 'refusal-preserves-state',
    'compatible-upgrade', 'upgrade-registry', 'upgraded-call',
    'capability-widening-refused', 'explicit-breaking-upgrade', 'breaking-call',
    'before-restart', 'hash-mismatch', 'abi-mismatch', 'unknown-interface-abi',
    'wrong-domain', 'trailing-interface', 'invalid-schema', 'invalid-result-schema',
    'missing-entrypoint', 'overdeclared-capability',
    'undeclared-capability', 'unknown-native-abi-0', f'unknown-native-abi-{unallocated_abi()}',
    'unknown-native-abi-65535', 'call-abi-mismatch', 'call-unknown-abi',
    'after-restart', 'restart-call',
}
REQUIRED = {f'abi{abi}-{case}' for abi in admitted_abis() for case in SUFFIXES}
REQUIRED |= {f'abi{abi}-{phase}-bindings-{variant}' for abi in admitted_abis()
             for phase in ('upgrade',) for variant in ('valid', 'bad-digest', 'bad-code')}
REQUIRED |= {f'abi{abi}-bindings-{variant}' for abi in admitted_abis()
             for variant in ('valid', 'bad-digest', 'bad-code')}
REQUIRED |= {f'abi{abi}-unsupported-import' for abi in admitted_abis()}
REQUIRED |= {f'abi{abi}-dynamic-{case}' for abi in [version for version in admitted_abis() if version >= 2] for case in (
    'deploy', 'registry', 'bindings-valid', 'bindings-bad-digest', 'bindings-bad-code',
    'zero-ceiling', 'offset-overflow', 'amount-offset-overflow')}
REQUIRED |= {'monotonic-deploy', 'breaking-cannot-downgrade-abi', 'immutable-deploy',
             'breaking-cannot-bypass-authority', 'immutable-refusal-preserves-state'}
REQUIRED |= {f'{label}-{case}' for label in tuple(f'oracle-v{abi}' for abi in admitted_abis() if abi >= 3)
             + tuple(f'web-v{abi}' for abi in admitted_abis() if abi >= 4) + ('market-v5',)
             for case in ('deploy', 'registry', 'bindings-valid', 'bindings-bad-digest',
                          'bindings-bad-code', 'bindings-registry-process')}
REQUIRED |= {f'monotonic-{operation}-{abi}' for abi in [version for version in admitted_abis() if version >= 2]
             for operation in ('upgrade', 'call')}
REQUIRED |= {f'abi{abi}-{phase}registry-process' for abi in admitted_abis()
             for phase in ('bindings-', 'upgrade-bindings-', 'breaking-', 'restart-')}
REQUIRED |= {f'abi{abi}-dynamic-bindings-registry-process' for abi in [version for version in admitted_abis() if version >= 2]}
ARTIFACTS = {'registry', 'builder-isolation', 'builder-supervisor', 'real-node-tests',
             'boundary', 'paxd', 'cli', 'bank-funding'}


def manifest_contract(value, source):
    require(set(value) == {'schema', 'source', 'test', 'required_cases', 'artifacts',
                           'build_logs', 'bank_artifacts', 'registry_configuration',
                           'registry_cgroup_parent'}, 'unexpected manifest fields')
    require(value['schema'] == SCHEMA and value['test'] == TEST, 'wrong native interface manifest')
    require(value['source'] == source, 'artifacts do not bind this clean candidate')
    require(value['required_cases'] == sorted(REQUIRED), 'incomplete or changed case inventory')
    require(set(value['artifacts']) == ARTIFACTS, 'required merged candidate process artifacts missing')
    require(value['build_logs'] and value['bank_artifacts'], 'build provenance or native bank funding absent')
    require(Path(value['artifacts']['paxd']['path']).name == 'paxd', 'merged paxd executable required')


def registry_interface(node, registry_url, program, abi, code_hash, interface, version, now, headers=None):
    """Bind the registry's current interface to the node and generated SDK consumer."""
    from layerx_sdk.programs import _interface
    require(abi in admitted_abis(), 'unallocated native ABI')
    domains = (b'LayerX/program-interface/v1\0', b'LayerX/program-interface/v2\0') if abi <= 2 else (
        f'LayerX/program-interface/v{abi}\0'.encode(),)
    require(any(interface.startswith(domain + code_hash + abi.to_bytes(2, 'big')) for domain in domains),
            'interface code hash or ABI substitution')
    record = merged.decode_abi(['uint8', 'bytes32', 'bytes32', 'uint16', 'uint32', 'bool'],
                              node.program_read('getProgram', [program]))
    require(record[2:] == [code_hash, abi, version, True], 'node deployment identity differs')
    observed = merged.decode_abi(['uint32', 'bytes32', 'bytes'], node.program_read('getInterface', [program]))
    require(observed == [version, hashlib.sha256(interface).digest(), interface],
            'node interface identity differs')
    merged.CHAIN.endpoint(registry_url)
    request = urllib.request.Request(registry_url.rstrip('/') + '/v1/programs/registry/' + program.hex() + '/interface',
                                     headers=headers or {})
    with urllib.request.urlopen(request, timeout=8) as response:
        raw = response.read(1048577)
        require(response.status == 200 and len(raw) <= 1048576, 'bounded registry interface required')
    value = json.loads(raw)
    parsed = _interface(value, program.hex(), now, protocol_version=3)
    require(value['version'] == version and value['abi_version'] == abi
            and value['code_hash'] == code_hash.hex() and value['interface'] == interface.hex()
            and value['interface_digest'] == observed[1].hex(), 'registry and node interface differ')
    return parsed


def case_output(output, fixtures):
    cases = re.findall(r'^NATIVE_INTERFACE_CASE ([A-Za-z0-9_-]+)$', output, re.M)
    require(len(cases) == len(set(cases)) and set(cases) == REQUIRED,
            'native case inventory incomplete, duplicated or unexpected')
    summaries = re.findall(r'^test result: ok\. (\d+) passed; (\d+) failed; (\d+) ignored;', output, re.M)
    require(summaries == [('1', '0', '0')], 'fixture absent, failed or skipped')
    for abi in admitted_abis():
        before = Path(fixtures) / f'abi{abi}-before-restart.state'
        after = Path(fixtures) / f'abi{abi}-after-restart.state'
        for path in (before, after, Path(fixtures) / f'abi{abi}-bindings.deployment',
                     Path(fixtures) / f'abi{abi}-bindings.interface'):
            producer.artifact(path)
        require(before.read_bytes() == after.read_bytes(), 'registry state changed across restart')
    return cases


def launch(command, environment, log):
    fd = os.open(log, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(fd, 'wb') as stream:
        process = subprocess.Popen(command, cwd=producer.ROOT, env=environment,
            stdin=subprocess.DEVNULL, stdout=stream, stderr=subprocess.STDOUT,
            start_new_session=True)
        try:
            return process.wait(timeout=1500)
        except subprocess.TimeoutExpired:
            return 124
        finally:
            try:
                os.killpg(process.pid, signal.SIGTERM)
            except ProcessLookupError:
                pass
            try:
                process.wait(timeout=10)
            except subprocess.TimeoutExpired:
                os.killpg(process.pid, signal.SIGKILL)
                process.wait(timeout=10)


def verify(manifest):
    manifest = Path(manifest).absolute()
    directory = producer.private_directory(manifest.parent)
    value = producer.load_private(manifest)
    source = producer.identity()
    manifest_contract(value, source)
    artifacts = value['artifacts']
    for saved in list(artifacts.values()) + value['build_logs'] + value['bank_artifacts']:
        require(producer.artifact(saved['path']) == saved, 'candidate artifact changed: ' + saved['path'])
    for name, saved in artifacts.items():
        require(os.access(saved['path'], os.X_OK), 'artifact is not executable: ' + name)
    configuration = producer.load_private(value['registry_configuration'])
    for key, name, digest_key in (
        ('LAYERX_REGISTRY_BUILDER_ISOLATION_RUNTIME', 'builder-isolation', 'LAYERX_REGISTRY_BUILDER_ISOLATION_RUNTIME_DIGEST'),
        ('LAYERX_REGISTRY_BUILDER_JOB_SUPERVISOR', 'builder-supervisor', 'LAYERX_REGISTRY_BUILDER_JOB_SUPERVISOR_DIGEST')):
        require(configuration[key] == artifacts[name]['path'] and configuration[digest_key] == artifacts[name]['sha256'],
                'registry builder process artifact mismatch')
    parent = Path(value['registry_cgroup_parent'])
    require(parent.is_absolute() and parent.resolve(strict=True) == parent
            and parent.stat().st_uid == 0 and (parent / 'cgroup.controllers').is_file(),
            'root-owned delegated cgroup parent required')
    require(os.geteuid() == 0, 'real process fixture requires root for protected identities')
    run = Path(tempfile.mkdtemp(prefix='native-interfaces-', dir=directory))
    fixtures = run / 'evidence'
    fixtures.mkdir(mode=0o700)
    environment = dict(os.environ, PYTHONDONTWRITEBYTECODE='1',
        PAXEER_X_PAXD=artifacts['paxd']['path'], PAXEER_X_PROGRAM_CLI=artifacts['cli']['path'],
        PAXEER_X_BANK_FUNDING=artifacts['bank-funding']['path'],
        PAXEER_X_NATIVE_INTERFACE_EVIDENCE=str(fixtures),
        PAXEER_X_NATIVE_INTERFACE_BOUNDARY=artifacts['boundary']['path'],
        PAXEER_X_REGISTRY_BINARY=artifacts['registry']['path'],
        PAXEER_X_REGISTRY_CONFIGURATION=value['registry_configuration'],
        PAXEER_X_REGISTRY_CGROUP_PARENT=str(parent))
    command = [artifacts['real-node-tests']['path'], '--exact', TEST,
               '--nocapture', '--test-threads=1']
    log = run / 'real-node.log'
    record = {'schema': 'paxeer-x.native-interface-result.v1', 'source': source,
              'command': COMMAND, 'process_command': command,
              'manifest': producer.artifact(manifest), 'exit_code': None,
              'cases': [], 'skipped': None, 'evidence': [], 'logs': []}
    try:
        code = launch(command, environment, log)
        record['exit_code'] = code
        require(code == 0, f'real native interface process exited {code}; log={log}')
        output = log.read_text(errors='strict')
        cases = case_output(output, fixtures)
        record['cases'] = cases
        require(producer.identity() == source, 'source changed during qualification')
        for saved in artifacts.values():
            require(producer.artifact(saved['path']) == saved, 'process artifact changed during qualification')
        receipts = sorted(fixtures.glob('*.receipt'))
        require(receipts and all(path.stat().st_size > 0 for path in receipts), 'native receipts absent')
        record['skipped'] = 0
        record['evidence'] = [producer.artifact(path) for path in sorted(fixtures.rglob('*')) if path.is_file()]
    finally:
        if log.exists():
            record['logs'] = [{'path': str(log), 'sha256': producer.digest(log),
                               'bytes': log.stat().st_size}]
        producer.write_private(run / 'result.json', record)
        print(f'EVIDENCE {run / "result.json"}', flush=True)
    print(f'cases={len(REQUIRED)} passed={len(REQUIRED)} skipped=0', flush=True)


def main():
    os.umask(0o077)
    parser = argparse.ArgumentParser()
    parser.add_argument('--manifest', default=os.environ.get('PAXEER_X_NATIVE_INTERFACES_MANIFEST'))
    args = parser.parse_args()
    try:
        require(args.manifest, 'PAXEER_X_NATIVE_INTERFACES_MANIFEST or --manifest is required')
        verify(args.manifest)
    except (OSError, ValueError, KeyError, TypeError, subprocess.SubprocessError) as error:
        print(f'native interface qualification refused: {error}', file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__':
    sys.exit(main())

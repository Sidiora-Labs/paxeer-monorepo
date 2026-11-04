#!/usr/bin/env python3
"""Qualify every typed guest ABI through the merged Programs precompile."""
import argparse
import hashlib
import importlib.util
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
sys.path.insert(0, str(Path(__file__).parent))
import merged_qualification as merged

COMMAND = 'timeout 30m python3 tools/qualification/paxeer-x/programs_typed_interfaces.py'
OUTPUTS = ('client.rs', 'client.ts', 'guest.rs', 'client.go', 'ProgramBindings.java',
           'client.kt', 'client.py', 'client.swift', 'Client.cs')
ROLES = {'node': 'paxd'}
ABI_CASES = ('wrong-hash', 'unknown-abi', 'wrong-schema', 'wrong-capability', 'deploy',
    'interface-read', 'bindings', 'generated-CODE_HASH_MISMATCH', 'generated-STALE_INTERFACE',
    'generated-call', 'simulation', 'incompatible-upgrade', 'refusal-preserves-interface',
    'compatible-upgrade', 'upgraded-bindings', 'restart-interface', 'restart-bindings',
    'restart-generated-call', 'restart-simulation')
LANGUAGES = {'rust': 'client.rs', 'typescript': 'client.ts', 'go': 'client.go',
    'java': 'ProgramBindings.java', 'kotlin': 'client.kt', 'python': 'client.py',
    'swift': 'client.swift', 'csharp': 'Client.cs'}
CONSUMER_CASES = {'roundtrip_call', 'typed_failure', 'stale_digest', 'wrong_code_hash', 'malformed_call'}


def require(value, message):
    if not value:
        raise ValueError(message)


def protected(path):
    path = Path(path)
    require(path.is_absolute() and path.resolve(strict=True) == path and not path.is_symlink(),
            'canonical absolute configuration path required')
    info = path.stat()
    require(stat.S_ISREG(info.st_mode) and info.st_uid == os.geteuid() and info.st_nlink == 1
            and not info.st_mode & 0o077 and 0 < info.st_size <= 1048576,
            'private bounded caller-owned regular configuration required')
    return path


def load(path):
    return json.loads(protected(path).read_text())


def digest(path):
    with Path(path).open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def artifact(path):
    path = Path(path).resolve(strict=True)
    require(path.is_file() and path.stat().st_size > 0, 'nonempty artifact required')
    return {'path': str(path), 'sha256': digest(path), 'bytes': path.stat().st_size}


def save(path, value):
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(fd, 'w') as stream:
        json.dump(value, stream, sort_keys=True, indent=2)
        stream.write('\n')


def identity():
    def git(*args):
        return subprocess.check_output(['git', *args], cwd=ROOT, text=True, timeout=30).strip()
    require(not git('status', '--porcelain=v1', '--untracked-files=normal'), 'clean candidate required')
    return {'revision': git('rev-parse', 'HEAD^{commit}'), 'tree': git('rev-parse', 'HEAD^{tree}')}


def case_inventory(rows):
    required = {f'abi{abi}-{case}' for abi in range(1, 5) for case in ABI_CASES}
    required |= {'untyped-deploy', 'untyped-interface-refused'}
    require(isinstance(rows, list) and len(rows) == len(required)
            and {row['id'] for row in rows} == required, 'complete typed ABI/refusal/restart inventory required')
    return rows


class TypedPrograms:
    def __init__(self, node, restart, caller):
        require(isinstance(caller, str) and re.fullmatch('0x[0-9a-fA-F]{40}', caller), 'typed EVM caller required')
        self.node, self.restart, self.caller = node, restart, caller

    def discover(self, program, abi, code_hash, interface, version):
        record = merged.decode_abi(['uint8', 'bytes32', 'bytes32', 'uint16', 'uint32', 'bool'],
            self.node.program_read('getProgram', [program]))
        require(record[2] == code_hash and record[3] == abi and record[4] == version and record[5],
                'CODE_HASH_MISMATCH or stale program ABI/version')
        observed = merged.decode_abi(['uint32', 'bytes32', 'bytes'], self.node.program_read('getInterface', [program]))
        require(observed == [version, hashlib.sha256(interface).digest(), interface], 'STALE_INTERFACE or noncanonical interface')
        prefix = b'LayerX/program-interface/v1\x00'
        require(interface.startswith(prefix + code_hash + abi.to_bytes(2, 'big')), 'interface code hash or ABI substitution')
        return record, observed

    def state(self, program):
        return self.node.program_read('getStateHead', [program])

    def snapshot(self, program):
        result = []
        for method in ('getProgram', 'getInterface', 'getStateHead'):
            try:
                result.append(self.node.program_read(method, [program]))
            except merged.CHAIN.RpcError as error:
                result.append({'refusal': error.error})
        return result

    def recover(self, program, receipts=()):
        retained = self.snapshot(program)
        committed = [self.node.committed(row['transaction_hash']) for row in receipts]
        require(committed == list(receipts), 'receipt changed before typed restart')
        self.restart()
        require(self.snapshot(program) == retained, 'restart changed typed program state or interface')
        require([self.node.committed(row['transaction_hash']) for row in receipts] == committed,
                'restart changed typed committed receipt')

    def action(self, row):
        require(set(row) == {'method', 'arguments', 'raw_transaction', 'refusal', 'expected'}, 'closed typed action required')
        method = row['method']
        require(method in {'deploy', 'upgrade', 'call', 'simulateCall', 'getProgram', 'getInterface', 'getStateHead'},
                'undeclared typed Programs action')
        arguments = self.node.json_arguments(method, row['arguments'])
        if row['raw_transaction'] is not None:
            require(method in {'deploy', 'upgrade', 'call'} and row['refusal'] is None,
                    'typed write requires committed execution')
            transaction, signature = merged.evm.decode_signed(bytes.fromhex(row['raw_transaction'].removeprefix('0x')))
            require(transaction.recover_signer(signature).lower() == self.caller.lower(), 'typed custody payer substitution')
            result = self.node.program_send(row['raw_transaction'], method, arguments)
            require(result['payer'].lower() == self.caller.lower(), 'typed custody payer substitution')
            if method == 'call':
                actual = self.node.call('pax_sequencerReceipt', [result['transaction_hash']])['returnData']
                require(isinstance(row['expected'], str) and actual == row['expected'], 'typed execution response differs')
                status, kind, _ = merged.CHAIN._EXECUTION.program_result(bytes.fromhex(actual.removeprefix('0x')))
                require(status == 0 and kind == 1, 'typed guest execution failed')
            else:
                require(row['expected'] is None, 'lifecycle write cannot claim an unverified return value')
            return result
        try:
            require(self.node.call('eth_chainId', []) == '0x7d', 'typed programs requires chain 125')
            value = self.node.call('eth_call', [{'from': self.caller, 'to': merged.CHAIN.PROGRAMS,
                'data': '0x' + self.node.program_call(method, arguments).hex()}, 'latest'])
        except merged.CHAIN.RpcError as error:
            require(isinstance(row['refusal'], dict) and error.error == row['refusal'], 'wrong typed ABI refusal')
            return {'refusal': error.error}
        require(row['refusal'] is None and value == row['expected'], 'typed call/simulation result differs or refusal missing')
        return value

    def case(self, row):
        require(set(row) == {'id', 'program', 'abi', 'code_hash', 'interface', 'version', 'actions', 'restart'},
                'closed typed qualification case required')
        program = bytes.fromhex(row['program'])
        require(len(program) == 32 and type(row['abi']) is int and row['abi'] in range(1, 5), 'typed program/ABI required')
        require(type(row['restart']) is bool and isinstance(row['actions'], list) and row['actions'], 'typed case actions required')
        name = row['id']
        require(name in {f'abi{abi}-{case}' for abi in range(1, 5) for case in ABI_CASES}
                | {'untyped-deploy', 'untyped-interface-refused'}, 'undeclared typed case')
        require(not name.startswith('abi') or name.startswith(f'abi{row["abi"]}-'), 'case ABI substitution')
        methods = {action['method'] for action in row['actions']}
        for suffix, method in [('deploy', 'deploy'), ('generated-call', 'call'), ('simulation', 'simulateCall'),
                               ('compatible-upgrade', 'upgrade'), ('interface-read', 'getInterface'),
                               ('restart-interface', 'getInterface'), ('untyped-interface-refused', 'getInterface')]:
            if name.endswith(suffix):
                require(method in methods, 'required typed action missing: ' + method)
        negative = any(name.endswith(suffix) for suffix in
            ('wrong-hash', 'unknown-abi', 'wrong-schema', 'wrong-capability', 'incompatible-upgrade', 'untyped-interface-refused'))
        before = self.snapshot(program) if negative else None
        if row['restart']:
            self.recover(program)
        for action in row['actions']:
            arguments = self.node.json_arguments(action['method'], action['arguments'])
            require(arguments[0] == program, 'typed action program substitution')
            if not negative and action['method'] in {'deploy', 'upgrade', 'call', 'simulateCall'}:
                require(arguments[1] == row['abi'], 'typed action ABI substitution')
            if not negative and action['method'] == 'deploy':
                require(arguments[4].hex() == row['code_hash'] and arguments[5].hex() == (row['interface'] or ''),
                        'typed deploy target metadata differs')
            if not negative and action['method'] == 'upgrade':
                require(arguments[4].hex() == row['code_hash'] and arguments[6].hex() == row['interface'],
                        'typed upgrade target metadata differs')
        simulation_state = self.state(program) if 'simulation' in name else None
        results = [self.action(action) for action in row['actions']]
        if simulation_state is not None:
            require(self.state(program) == simulation_state, 'simulation mutated program state')
        if not negative and any(method in methods for method in ('deploy', 'upgrade', 'call')):
            require(any(isinstance(value, dict) and 'transaction_hash' in value for value in results),
                    'positive typed transition requires a committed receipt')
        if negative:
            require(any(isinstance(value, dict) and 'refusal' in value for value in results), 'typed refusal case admitted')
            if before is not None:
                require(self.snapshot(program) == before, 'refused registration or upgrade changed state or interface')
        if row['interface'] is not None:
            discovered = self.discover(program, row['abi'], bytes.fromhex(row['code_hash']),
                bytes.fromhex(row['interface']), row['version'])
        else:
            require(negative or name.startswith('untyped-'), 'typed interface omitted')
            discovered = None
        if 'restart-' in name:
            require(row['restart'], 'typed restart omitted')
        if row['restart']:
            self.recover(program, [result for result in results if isinstance(result, dict) and 'transaction_hash' in result])
            if discovered is not None:
                require(self.discover(program, row['abi'], bytes.fromhex(row['code_hash']),
                    bytes.fromhex(row['interface']), row['version']) == discovered, 'restart changed typed discovery')
            for result in results:
                if isinstance(result, dict) and 'transaction_hash' in result:
                    require(self.node.committed(result['transaction_hash']) == result, 'restart changed typed committed receipt')
        if 'simulation' in name:
            # Simulation must be side-effect free at the actual EVM boundary.
            state = self.state(program)
            for action in row['actions']:
                if action['method'] == 'simulateCall':
                    require(self.action(action) in results, 'simulation is nondeterministic')
            require(self.state(program) == state, 'simulation mutated program state')
        return results


def validate_generated(client, abi):
    require(client.INTERFACE_ABI_VERSION == abi, 'generated ABI metadata differs')
    call = client.Client(client.CODE_HASH, client.INTERFACE_DIGEST).entry_call_01020304(
        client.Entry_01020304Input(b'typed-interface'))
    require(call.as_bytes() == bytes.fromhex('0102030401200000000f') + b'typed-interface', 'noncanonical generated calldata')
    require(call.decode_failure(7, bytes.fromhex('011009')).detail == 9, 'typed failure decoding changed')
    try:
        call.decode_output(bytes.fromhex('ff20'))
    except client.BindingRefusal:
        pass
    else:
        raise ValueError('generated decoder admitted an invalid response schema')
    for code, code_hash, interface_digest in [('CODE_HASH_MISMATCH', bytes(32), client.INTERFACE_DIGEST),
                                              ('STALE_INTERFACE', client.CODE_HASH, bytes(32))]:
        try:
            client.Client(code_hash, interface_digest)
        except client.BindingRefusal as error:
            require(error.code == code, 'wrong generated target refusal')
        else:
            raise ValueError('generated client accepted a tampered target')
    return call


def consumers(rows, source, directory, session):
    required = {(abi, phase, language) for abi in range(1, 5) for phase in ('initial', 'upgrade') for language in LANGUAGES}
    require(len(rows) == 64 and {(row['abi'], row['phase'], row['language']) for row in rows} == required,
            'all ABI/version/language consumers are required')
    results = []
    for row in rows:
        require(row['source_revision'] == source['revision'] and row['compile_exit_code'] == 0
                and row['compile_command'], 'source-bound successful consumer compilation required')
        for saved in [row['compile_log'], row['compiler'], row['runtime'], *row['sources'].values(), *row['outputs']]:
            require(artifact(saved['path']) == saved, 'consumer source, compiler or artifact changed')
        binding = row['sources'][LANGUAGES[row['language']]]
        require(binding['sha256'] == row['binding_sha256'], 'consumer does not use emitted binding')
        program = bytes.fromhex(row['program'])
        session.discover(program, row['abi'], bytes.fromhex(row['code_hash']), bytes.fromhex(row['interface']), row['version'])
        if row['language'] == 'python':
            spec = importlib.util.spec_from_file_location('typed_consumer_' + str(len(results)), binding['path'])
            client = importlib.util.module_from_spec(spec)
            sys.modules[spec.name] = client
            spec.loader.exec_module(client)
            call = validate_generated(client, row['abi'])
            require(bytes(client.CODE_HASH) == bytes.fromhex(row['code_hash'])
                    and bytes(client.INTERFACE_DIGEST) == hashlib.sha256(bytes.fromhex(row['interface'])).digest(),
                    'generated client differs from deployed target')
            actions = row['actions']
            require({action['method'] for action in actions} == {'call', 'simulateCall'}, 'generated consumer call and simulation required')
            for action in actions:
                args = session.node.json_arguments(action['method'], action['arguments'])
                require(args[0] == program and args[1] == row['abi'] and args[3] == call.as_bytes(), 'generated call binding substitution')
                result = session.action(action)
                if isinstance(result, dict):
                    encoded = session.node.call('pax_sequencerReceipt', [result['transaction_hash']])['returnData']
                else:
                    encoded = result
                status, kind, response = merged.CHAIN._EXECUTION.program_result(bytes.fromhex(encoded.removeprefix('0x')))
                require(status == 0 and kind == 1, 'generated consumer execution failed')
                if row['abi'] >= 2:
                    require(call.decode_output(response) == b'typed-interface', 'typed response differs from real guest echo')
        command = row['command']
        require(command and command[0] == row['runtime']['path'], 'unbound consumer runtime')
        outputs = {value['path'] for value in row['outputs']}
        runtime = Path(command[0]).name
        if command[0] not in outputs:
            require(runtime in {'node', 'python3', 'java', 'dotnet', 'mono'} and '-c' not in command and '-m' not in command,
                    'compiler or inline interpreter execution forbidden in verify')
            require(runtime != 'java' or not any(arg.endswith('.java') for arg in command), 'Java source compilation refused')
            require(runtime != 'dotnet' or len(command) > 1 and command[1].endswith('.dll'), 'compiled .NET consumer required')
        cwd = Path(row['cwd']).resolve(strict=True)
        closure = {saved['path'] for saved in row['sources'].values()} | outputs
        require({str(path.resolve()) for path in cwd.rglob('*') if path.is_file()} == closure, 'consumer source closure incomplete')
        result = subprocess.run(command, cwd=cwd, capture_output=True, timeout=90,
            env=dict(os.environ, CHAIN_RPC_URL=session.node.url, PYTHONDONTWRITEBYTECODE='1'))
        save(directory / f'consumer-{len(results)}.json', {'command': command, 'exit_code': result.returncode,
            'stdout': result.stdout.decode(), 'stderr': result.stderr.decode()})
        cases = re.findall(r'^BINDING_CASE ([a-z_]+)$', result.stdout.decode(), re.M)
        require(result.returncode == 0 and len(cases) == len(CONSUMER_CASES) and set(cases) == CONSUMER_CASES,
                'consumer omitted canonical calldata/result/error/refusal cases')
        results.append({'abi': row['abi'], 'phase': row['phase'], 'language': row['language'], 'cases': cases})
    return results


def qualify(manifest_path):
    manifest = load(manifest_path)
    require(manifest['schema'] == 'paxeer-x.typed-interface-artifacts.v2', 'merged typed manifest v2 required')
    source = identity()
    require(manifest['source'] == source, 'artifacts do not bind exact clean candidate')
    merged.source_artifacts(manifest, source['revision'])
    profile = manifest['paxd']
    require(profile['source_revision'] == source['revision'], 'paxd seed revision mismatch')
    rows = case_inventory(manifest['cases'])
    parent = protected(manifest['evidence_anchor']).parent
    require(parent != ROOT and ROOT not in parent.parents, 'private evidence outside repository required')
    directory = Path(tempfile.mkdtemp(prefix='typed-interfaces-', dir=parent))
    fixture = merged.OWNED.PaxdFixture(profile, manifest['artifacts']['paxd']['path'], directory)
    record = {'task': '7.4', 'source': source, 'command': COMMAND, 'exit_code': 1, 'cases': [], 'skipped': 0}
    try:
        fixture.start()
        session = TypedPrograms(fixture.node, fixture.restart, manifest['actor_address'])
        for row in rows:
            record['cases'].append({'id': row['id'], 'results': session.case(row)})
        record['consumers'] = consumers(manifest['consumers'], source, directory, session)
        require(identity() == source, 'source changed during qualification')
        merged.source_artifacts(manifest, source['revision'])
        record['exit_code'] = 0
    finally:
        fixture.stop()
        save(directory / 'result.json', record)
        print('EVIDENCE ' + str(directory / 'result.json'))
    return record['exit_code']


def main():
    os.umask(0o077)
    parser = argparse.ArgumentParser()
    parser.add_argument('--manifest', default=os.environ.get('PAXEER_X_TYPED_INTERFACES_MANIFEST'))
    args = parser.parse_args()
    require(args.manifest, 'source-bound merged typed manifest required')
    return qualify(args.manifest)


if __name__ == '__main__':
    try:
        sys.exit(main())
    except (OSError, ValueError, KeyError, TypeError, RuntimeError, subprocess.SubprocessError) as error:
        print('typed interface qualification refused: ' + str(error), file=sys.stderr)
        sys.exit(1)

#!/usr/bin/env python3
"""Qualify account upgrades, winddown and recovery on an owned merged paxd.

The fixture supplies source-bound signed EVM transactions, exact ABI observations
and a private seeded paxd home. Every historical case remains mandatory. The
runner owns the process and reopens that same home for recovery cases.
"""
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import shutil
import stat
import subprocess
import sys
import time

ROOT = Path(__file__).resolve().parents[3]
_spec = importlib.util.spec_from_file_location('winddown_merged_chain', Path(__file__).with_name('fixtures') / 'merged_chain.py')
CHAIN = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(CHAIN)
SCHEMA = 'paxeer-x.merged-accounts-artifacts.v2'
ACCOUNTS_SCHEMA = SCHEMA
COMMAND = 'timeout 20m python3 tools/qualification/paxeer-x/programs_accounts_winddown.py --task 6.6'
ACCOUNTS_COMMAND = 'timeout 30m python3 tools/qualification/paxeer-x/programs_accounts_winddown.py'
REQUIRED = {'deploy-abi2', 'owner-opt-in-profile2', 'fund-abi2', 'upgrade-abi3',
            'additional-account-abi3', 'fund-abi3', 'upgrade-abi4',
            'additional-account-abi4', 'fund-abi4', 'missing-transfer-capability',
            'native-restart', 'route', 'deprecate', 'exit',
            'route-abi3', 'route-abi4', 'exit-abi3', 'exit-abi4', 'tombstone',
            'proof-corruption-refused', 'stale-head-refused'}
ACCOUNTS_REQUIRED = {'retained-abi2-3-4-proof-transfer-winddown',
    'unsupported-abi-refused', 'unauthorized-route-refused', 'wrong-asset-refused',
    'wrong-principal-refused', 'invalid-proof-refused', 'restart-exit-eligibility',
    'conservation'}
SOURCE_PATHS = ('Makefile', 'chain.mk', 'go.mod', 'go.sum', 'daemon/paxd', 'node', 'rpc', 'precompiles',
                'core', 'programs', 'modules', 'src', 'include',
                'tools/qualification/paxeer-x/programs_accounts_winddown.py',
                'tools/qualification/paxeer-x/fixtures/merged_chain.py')


def require(condition, message):
    if not condition:
        raise ValueError(message)


def digest(path):
    with Path(path).open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def private_json(path):
    path = Path(path).absolute()
    require(path.resolve() == path, 'private manifest symlink refused')
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    with os.fdopen(fd) as stream:
        info = os.fstat(stream.fileno())
        require(stat.S_ISREG(info.st_mode) and info.st_uid == os.geteuid()
                and not info.st_mode & 0o077 and info.st_nlink == 1 and info.st_size <= 16_777_216,
                'bounded private caller-owned manifest required')
        return json.load(stream)


def write_private(path, value):
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(fd, 'w') as stream:
        json.dump(value, stream, sort_keys=True, indent=2)
        stream.write('\n')
        stream.flush()
        os.fsync(stream.fileno())


def artifact(path):
    path = Path(path).absolute()
    require(path.is_file() and path.resolve() == path and os.access(path, os.X_OK), 'genuine paxd artifact missing')
    with path.open('rb') as stream:
        require(stream.read(4) == b'\x7fELF', 'native paxd executable required')
    return {'path': str(path), 'sha256': digest(path), 'bytes': path.stat().st_size}


def sources():
    require(not subprocess.check_output(['git', 'status', '--porcelain', '--untracked-files=no'], cwd=ROOT),
            'clean immutable candidate required')
    revision = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip()
    output = subprocess.check_output(['git', 'ls-files', '-z', '--', *SOURCE_PATHS], cwd=ROOT)
    files = sorted(os.fsdecode(name) for name in output.split(b'\0') if name)
    require('daemon/paxd/main.go' in files and 'precompiles/layerxprograms/abi.json' in files,
            'merged paxd and Programs sources required')
    bound = {}
    for name in files:
        require(not any(part.startswith('.env') for part in Path(name).parts), 'credential source refused')
        committed = subprocess.check_output(['git', 'show', revision + ':' + name], cwd=ROOT)
        require(hashlib.sha256(committed).hexdigest() == digest(ROOT / name), 'candidate source differs from revision')
        bound[name] = digest(ROOT / name)
    return {'revision': revision, 'files': bound}


class WinddownSession:
    def __init__(self, node, restart):
        self.node, self.restart = node, restart

    def observations(self, reads):
        require(isinstance(reads, list) and reads, 'account invariant observations required')
        results = []
        for row in reads:
            require(set(row) == {'method', 'arguments', 'expected'}, 'closed account observation required')
            require(row['method'] in {'getProgram', 'getAccountProfile', 'getProgramAccountById',
                'getValueAccount', 'getExitRoute', 'getWindDown', 'getStateHead'}, 'undeclared account invariant read')
            arguments = self.node.json_arguments(row['method'], row['arguments'])
            actual = self.node.program_read(row['method'], arguments)
            require(actual == row['expected'], 'account state/proof/route invariant differs: ' + row['method'])
            require(isinstance(actual, str) and re.fullmatch('0x(?:[0-9a-f]{64})+', actual),
                    'canonical Programs ABI observation required')
            results.append(actual)
        return results

    @staticmethod
    def total(reads, results):
        total = 0
        balances = 0
        for row, value in zip(reads, results, strict=True):
            if row['method'] == 'getValueAccount':
                raw = bytes.fromhex(value[2:])
                require(len(raw) == 64 and int.from_bytes(raw[:32], 'big') < 2**128
                        and int.from_bytes(raw[32:], 'big') in (0, 1), 'invalid account balance/frozen ABI')
                total += int.from_bytes(raw[:32], 'big')
                balances += 1
        require(balances > 0, 'conservation requires real account balances')
        return total

    def case(self, row):
        require(isinstance(row, dict) and set(row) == {
            'id', 'before', 'steps', 'after', 'restart', 'expected_total_delta'}, 'closed winddown case required')
        name = row['id']
        require(name in REQUIRED | ACCOUNTS_REQUIRED, 'undeclared winddown case')
        require(type(row['restart']) is bool and type(row['expected_total_delta']) is int,
                'exact recovery and conservation policy required')
        before = self.observations(row['before'])
        require([(item['method'], item['arguments']) for item in row['before']] ==
                [(item['method'], item['arguments']) for item in row['after']],
                'conservation and recovery must read the identical accounts and routes')
        receipts, refusals = [], 0
        methods = []
        require(isinstance(row['steps'], list), 'winddown steps must be a list')
        for step in row['steps']:
            require(set(step) == {'method', 'arguments', 'raw_transaction', 'refusal'}, 'closed Programs step required')
            method = step['method']
            arguments = self.node.json_arguments(method, step['arguments'])
            methods.append((method, arguments))
            if step['raw_transaction'] is not None:
                require(step['refusal'] is None, 'transaction outcomes need committed evidence')
                receipts.append(self.node.program_send(step['raw_transaction'], method, arguments))
            else:
                require(isinstance(step['refusal'], dict) and set(step['refusal']) == {'code', 'message', 'data'},
                        'exact typed Programs refusal required')
                try:
                    self.node.program_read(method, arguments)
                except CHAIN.RpcError as error:
                    require(error.error == step['refusal'], 'Programs refused for a different reason')
                    refusals += 1
                else:
                    require(False, 'invalid Programs action was admitted')
        negative = name.endswith('refused') or name == 'missing-transfer-capability'
        if negative:
            require(refusals > 0 and not receipts and row['expected_total_delta'] == 0,
                    'negative account cases require an actual refusal and no value movement')
        elif 'restart' not in name and name != 'conservation':
            require(receipts, 'account transition requires a committed Programs transaction')
        mandatory = {'deploy-abi2': 'deploy', 'owner-opt-in-profile2': 'registerAccount',
            'upgrade-abi3': 'upgrade', 'upgrade-abi4': 'upgrade', 'deprecate': 'deprecate', 'tombstone': 'tombstone'}
        wanted = mandatory.get(name)
        if name.startswith('additional-account-'):
            wanted = 'registerAccount'
        if name.startswith('route'):
            wanted = 'setExitRoute'
        if name.startswith('exit'):
            require(any(method in ('exitValue', 'exitValueBounded') for method, _ in methods), 'exit action missing')
        if name.startswith('fund-'):
            wanted = 'transfer'
        if wanted:
            require(any(method == wanted for method, _ in methods), 'required Programs transition missing: ' + wanted)
        if name in ('deploy-abi2', 'upgrade-abi3', 'upgrade-abi4'):
            abi = int(name[-1])
            require(any(method == wanted and arguments[1] == abi for method, arguments in methods),
                    'retained ABI upgrade case changed its ABI')
        if name == 'owner-opt-in-profile2':
            require(any(method == 'registerAccount' and arguments[1] == 2 for method, arguments in methods),
                    'owner opt-in must retain profile 2')
        if name == 'retained-abi2-3-4-proof-transfer-winddown':
            require({arguments[1] for method, arguments in methods if method in ('deploy', 'upgrade')} >= {2, 3, 4}
                    and {method for method, _ in methods} >= {'transfer', 'setExitRoute', 'deprecate', 'exitValue', 'tombstone'},
                    'retained ABI/proof/transfer/winddown sequence missing')
        after = self.observations(row['after'])
        require(self.total(row['after'], after) - self.total(row['before'], before) == row['expected_total_delta'],
                'account value conservation failed')
        if negative:
            require(after == before, 'refused action changed retained state or proof')
        if 'restart' in name:
            require(row['restart'], 'required account restart was omitted')
        if row['restart']:
            self.restart()
            require(self.observations(row['after']) == after, 'restart changed account eligibility, proof or exit route')
            for receipt in receipts:
                require(self.node.committed(receipt['transaction_hash']) == receipt,
                        'restart changed the committed receipt or transaction identity')
        return {'id': name, 'receipts': receipts, 'before': before, 'after': after, 'refusals': refusals}


class PaxdFixture:
    def __init__(self, document, binary, directory):
        require(set(document) == {'schema', 'source_revision', 'home', 'home_sha256', 'endpoint', 'receipt_keys', 'cases'},
                'closed merged winddown fixture required')
        require(document['schema'] == 'paxeer-x.merged-winddown-fixture.v2', 'merged fixture schema required')
        url = CHAIN.endpoint(document['endpoint'])
        require(url.hostname in ('localhost', '127.0.0.1', '::1'), 'owned loopback paxd required')
        seed = Path(document['home'])
        require(seed.is_absolute() and seed.resolve() == seed and seed.is_dir(), 'owned seeded paxd home required')
        files = {}
        for path in [seed, *seed.rglob('*')]:
            info = path.lstat()
            require(not path.is_symlink() and info.st_uid == os.geteuid() and not info.st_mode & 0o077
                    and (stat.S_ISDIR(info.st_mode) or stat.S_ISREG(info.st_mode)), 'unsafe paxd seed')
            if path.is_file():
                files[str(path.relative_to(seed))] = digest(path)
        require(files and files == document['home_sha256'], 'paxd seed provenance mismatch')
        self.home = directory / 'paxd-home'
        shutil.copytree(seed, self.home)
        self.binary, self.directory = binary, directory
        self.node = CHAIN.Node(document['endpoint'], document['receipt_keys'])
        self.process = None
        self.starts = 0

    def start(self):
        require(self.process is None or self.process.poll() is not None, 'owned paxd already running')
        self.starts += 1
        with (self.directory / ('paxd-' + str(self.starts) + '.log')).open('xb') as log:
            self.process = subprocess.Popen([self.binary, 'start', '--home', str(self.home)],
                cwd=ROOT, stdin=subprocess.DEVNULL, stdout=log, stderr=subprocess.STDOUT)
        deadline = time.monotonic() + 180
        while time.monotonic() < deadline:
            require(self.process.poll() is None, 'merged paxd exited before readiness')
            try:
                if self.node.call('eth_chainId', []) == '0x7d':
                    return
            except (OSError, ValueError):
                pass
            time.sleep(0.1)
        require(False, 'merged paxd readiness deadline')

    def stop(self):
        if self.process is not None and self.process.poll() is None:
            self.process.terminate()
            try:
                self.process.wait(timeout=10)
            except subprocess.TimeoutExpired:
                self.process.kill()
                self.process.wait(timeout=5)
                require(False, 'merged paxd shutdown deadline')

    def restart(self):
        self.stop()
        self.start()


def build(arguments, evidence):
    before = sources()
    binary = ROOT / 'build/paxd'
    command = ['make', 'paxd-build']
    with (evidence / 'build.log').open('xb') as log:
        result = subprocess.run(command, cwd=ROOT, stdin=subprocess.DEVNULL, stdout=log, stderr=subprocess.STDOUT, timeout=1100)
    require(result.returncode == 0, 'merged paxd build failed; log=' + str(evidence / 'build.log'))
    require(sources() == before, 'source changed during merged build')
    manifest = evidence / 'artifacts.json'
    write_private(manifest, {'schema': SCHEMA, 'source': before, 'artifacts': {'paxd': artifact(binary)}})
    print('PAXEER_X_ACCOUNTS_WINDDOWN_MANIFEST=' + str(manifest), flush=True)


def case_inventory(task, rows):
    required = REQUIRED if task == '6.6' else REQUIRED | ACCOUNTS_REQUIRED
    require(isinstance(rows, list) and len(rows) == len(required)
            and all(isinstance(row, dict) and isinstance(row.get('id'), str) for row in rows)
            and {row['id'] for row in rows} == required,
            'missing, duplicate or changed retained winddown case inventory')
    return rows


def verify(arguments, evidence):
    raw = arguments.manifest or os.environ.get('PAXEER_X_ACCOUNTS_WINDDOWN_MANIFEST')
    require(raw, 'merged candidate artifact manifest required')
    manifest = private_json(raw)
    require(set(manifest) == {'schema', 'source', 'artifacts'} and manifest['schema'] == SCHEMA
            and manifest['source'] == sources() and set(manifest['artifacts']) == {'paxd'}, 'complete merged source binding required')
    saved = manifest['artifacts']['paxd']
    require(artifact(saved['path']) == saved, 'merged paxd artifact changed')
    profile_path = os.environ.get('PAXEER_X_PROGRAMS_WINDDOWN_FIXTURE')
    require(profile_path, 'source-bound merged Programs case producer required')
    profile = private_json(profile_path)
    require(profile['source_revision'] == manifest['source']['revision'], 'Programs producer revision mismatch')
    rows = case_inventory(arguments.task, profile['cases'].get(arguments.task))
    fixture = PaxdFixture(profile, saved['path'], evidence)
    results = []
    try:
        fixture.start()
        session = WinddownSession(fixture.node, fixture.restart)
        for row in rows:
            result = session.case(row)
            write_private(evidence / (row['id'] + '.json'), result)
            results.append(result)
        require(sources() == manifest['source'] and artifact(saved['path']) == saved, 'candidate changed during verification')
        return results
    finally:
        fixture.stop()


def main():
    os.umask(0o077)
    parser = argparse.ArgumentParser()
    parser.add_argument('--task', choices=('6.4', '6.6'), default='6.4')
    parser.add_argument('--build', action='store_true')
    parser.add_argument('--manifest')
    arguments = parser.parse_args()
    raw = os.environ.get('PAXEER_X_ACCOUNTS_WINDDOWN_EVIDENCE')
    require(raw, 'private merged winddown evidence directory required')
    base = Path(raw).absolute()
    require(base != ROOT and ROOT not in base.parents and base.resolve() == base, 'private evidence outside candidate required')
    base.mkdir(mode=0o700, parents=True, exist_ok=True)
    require(base.stat().st_uid == os.geteuid() and not base.stat().st_mode & 0o077, 'private evidence ownership/mode required')
    evidence = base / ('build-' if arguments.build else 'verify-')
    evidence.mkdir(mode=0o700)
    result = {'command': ACCOUNTS_COMMAND if arguments.task == '6.4' else COMMAND, 'exit_code': 1}
    try:
        result['revision'] = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip()
        if arguments.build:
            build(arguments, evidence)
        else:
            result['cases'] = verify(arguments, evidence)
        result['exit_code'] = 0
    except (OSError, ValueError, KeyError, TypeError, subprocess.SubprocessError) as error:
        result['error'] = str(error)
    finally:
        write_private(evidence / 'result.json', result)
        print('EVIDENCE ' + str(evidence / 'result.json'), flush=True)
    return result['exit_code']


if __name__ == '__main__':
    sys.exit(main())

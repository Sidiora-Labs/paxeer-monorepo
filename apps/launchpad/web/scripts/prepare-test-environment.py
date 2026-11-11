#!/usr/bin/env python3
"""Prepare the ordered, source-bound producers before starting the full suite."""
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import signal
import socket
import subprocess
import sys
import tempfile
import time
import urllib.request
import urllib.error
import urllib.parse

WEB = Path(__file__).resolve().parents[1]
ROOT = WEB.parents[2]
FIXTURES = WEB / 'src/core/wallet-sdk/test-fixtures'
CREATE = WEB / 'src/widgets/create'
TRADE = WEB / 'src/widgets/trading/transaction-controls'


def digest(path):
    path = Path(path)
    if not path.is_file() or path.is_symlink():
        raise ValueError('Regular bound input required: ' + str(path))
    return hashlib.sha256(path.read_bytes()).hexdigest()


def module(path):
    spec = importlib.util.spec_from_file_location(path.stem.replace('-', '_'), path)
    value = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(value)
    return value


class Environment:
    def __init__(self, output, env):
        self.output, self.env = output, env
        self.children, self.logs = [], []
        self.processes, self.readiness_failures = {}, []

    def readiness_error(self, child, reason):
        role, log = self.processes[child.pid]
        failure = {'role': role, 'exit_status': child.poll(), 'reason': reason,
                   'log': str(log), 'last_log_lines': log.read_text(errors='replace').splitlines()[-40:]}
        self.readiness_failures.append(failure)
        return RuntimeError(f"{role}: exit status {failure['exit_status']}; {reason}; {log}\n"
                            + '\n'.join(failure['last_log_lines']))

    def run(self, argv, name, cwd=ROOT, input=None):
        log = self.output / (name + '.log')
        with log.open('w') as stream:
            result = subprocess.run(argv, cwd=cwd, env=self.env, input=input,
                                    text=True, stdout=stream, stderr=subprocess.STDOUT,
                                    timeout=600)
        if result.returncode:
            raise RuntimeError(f'{name}: exit {result.returncode}; {log}')

    def stage(self, name, function):
        print('Preparing ' + name, flush=True)
        function()
        (self.output / 'environment.json').write_text(json.dumps({key: value for key, value in self.env.items()
            if key.startswith(('L25_', 'L26_', 'L27_', 'L28_', 'L38_', 'WALLET_TEST_', 'DISPATCH_TEST_', 'JOINED_'))}, indent=2) + '\n')

    def start(self, argv, name, cwd=ROOT, env=None):
        log = (self.output / (name + '.log')).open('w')
        self.logs.append(log)
        child = subprocess.Popen(argv, cwd=cwd, env=self.env if env is None else env,
                                 stdout=log, stderr=subprocess.STDOUT, start_new_session=True)
        self.children.append(child)
        self.processes[child.pid] = (name, Path(log.name))
        return child

    def wait_file(self, path, child, timeout=60):
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            if child.poll() is not None:
                raise self.readiness_error(child, f'exited before writing {path}')
            if path.is_file() and path.stat().st_size:
                json.loads(path.read_text())
                return
            time.sleep(.05)
        raise self.readiness_error(child, 'readiness timed out: ' + str(path))

    def tls(self):
        key, cert = self.output / 'loopback.key', self.output / 'loopback.crt'
        self.run(['openssl', 'req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-days', '1',
                  '-subj', '/CN=localhost', '-addext', 'subjectAltName=DNS:localhost,IP:127.0.0.1',
                  '-keyout', str(key), '-out', str(cert)], 'loopback-certificate')
        os.chmod(key, 0o600)
        self.env.update(LAUNCHPAD_TLS_KEY=str(key), LAUNCHPAD_TLS_CERT=str(cert), NODE_EXTRA_CA_CERTS=str(cert))

    def anvil(self, port, chain_id=125):
        # Never reuse, reset or terminate another process's endpoint.
        with socket.socket() as probe:
            probe.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
            probe.bind(('127.0.0.1', port))
        log = (self.output / f'anvil-{port}.log').open('w')
        self.logs.append(log)
        child = subprocess.Popen([self.env.get('ANVIL_BIN', 'anvil'), '--host', '127.0.0.1',
                                  '--port', str(port), '--chain-id', str(chain_id), '--silent'],
                                 env=self.env, stdout=log, stderr=subprocess.STDOUT)
        self.children.append(child)
        self.processes[child.pid] = (f'anvil-{port}', Path(log.name))
        url = f'http://127.0.0.1:{port}'
        deadline = time.monotonic() + 15
        while time.monotonic() < deadline:
            if child.poll() is not None:
                raise self.readiness_error(child, 'exited before RPC readiness')
            try:
                request = urllib.request.Request(url, data=b'{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}', headers={'Content-Type': 'application/json'})
                with urllib.request.urlopen(request, timeout=1) as response:
                    if json.load(response).get('result') != hex(chain_id):
                        raise ValueError('Owned Anvil chain identity mismatch')
                return child, url
            except OSError:
                time.sleep(.05)
        raise self.readiness_error(child, 'RPC readiness timed out')

    def wallet(self):
        module(FIXTURES / 'run-l25-check.py').prepare_fixture(self)

    def creation(self):
        module(CREATE / 'run-creation-chain.py').prepare_fixture(self)

    def advanced(self):
        ready = self.output / 'advanced-environment.json'
        child = self.start(['node', str(WEB / 'src/widgets/meta-ag/prepare-owned-deployment.mjs'),
                            self.env['L27_PHASE2_FIXTURE'], str(self.output / 'advanced-deployment'),
                            str(ROOT / 'contracts/launchpad'), str(ready)], 'advanced-deploy')
        self.wait_file(ready, child, timeout=300)
        self.env.update(json.loads(ready.read_text()))
        fixture = self.env['L28_CHAIN_FIXTURE_JSON']
        supplied = self.env['L28_CHAIN_FIXTURE_SHA256']
        if digest(fixture) != supplied:
            raise ValueError('Advanced fixture input checksum mismatch')
        log = self.output / 'advanced-plan.json'
        with log.open('w') as stream:
            subprocess.run(['node', str(WEB / 'src/widgets/meta-ag/connected-chain-inputs.mjs')],
                           input=json.dumps({'fixtureFile': fixture, 'fixtureSha256': supplied,
                                             'output': str(self.output / 'advanced')}),
                           text=True, stdout=stream, check=True, env=self.env, timeout=30)
        self.env.update(json.loads(log.read_text())['environment'])

    def trading_phase2(self):
        deployment = json.loads(Path(self.env['L27_PHASE2_FIXTURE']).read_text())
        core = deployment['phase2']
        fixture = Path(self.env['L26_FIXTURE_JSON'])
        artifact = Path(deployment['consumedArtifacts'][0]['path'])
        debug = json.loads(artifact.with_suffix('.dbg.json').read_text())
        build = json.loads((artifact.parent / debug['buildInfo']).read_text())
        cache = Path(self.env.get('XDG_CACHE_HOME', str(Path.home() / '.cache'))) / 'hardhat-nodejs/compilers-v2/linux-amd64'
        compilers = json.loads((cache / 'list.json').read_text())
        matches = [entry for entry in compilers['builds'] if entry['longVersion'] == build['solcLongVersion']]
        if len(matches) != 1:
            raise ValueError('Owned creation compiler identity is ambiguous')
        compiler = cache / matches[0]['path']
        prepared = module(TRADE / 'run-phase2.py').prepare_phase2_environment(
            str(fixture), digest(fixture), core['rpcUrl'],
            str(compiler), digest(compiler), self.output / 'trading-phase2')
        self.env.update(prepared)

    def joined(self):
        module(WEB / 'src/widgets/trading/run-joined-services.py').prepare_fixture(self)

    def required(self, name):
        value = self.env.get(name)
        if not value:
            raise ValueError('Missing producer input ' + name + '; fixture evidence: ' + str(self.output))
        return value

    def close(self):
        for child in reversed(self.children):
            if child.poll() is None:
                if child.args[0] == self.env.get('ANVIL_BIN', 'anvil'):
                    child.terminate()
                else:
                    os.killpg(child.pid, signal.SIGTERM)
                try:
                    child.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    if child.args[0] == self.env.get('ANVIL_BIN', 'anvil'):
                        child.kill()
                    else:
                        os.killpg(child.pid, signal.SIGKILL)
                    child.wait()
        for log in self.logs:
            log.close()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output-root', type=Path, default=WEB / 'build/launchpad-tests')
    parser.add_argument('--prepare-only', action='store_true')
    parser.add_argument('command', nargs=argparse.REMAINDER)
    args = parser.parse_args()
    command = args.command[1:] if args.command[:1] == ['--'] else args.command
    if not command and not args.prepare_only:
        parser.error('A child command is required after --')
    if command and args.prepare_only:
        parser.error('--prepare-only cannot run a child command')
    args.output_root.mkdir(parents=True, exist_ok=True)
    output = Path(tempfile.mkdtemp(prefix='run-', dir=args.output_root.resolve()))
    os.chmod(output, 0o700)
    env = dict(os.environ)
    env['L38_ARTIFACT_DIR'] = str(output / 'safety-voice')
    Path(env['L38_ARTIFACT_DIR']).mkdir(mode=0o700)
    setup = Environment(output, env)
    print('Private launchpad fixture evidence: ' + str(output), flush=True)
    def interrupted(number, frame):
        raise KeyboardInterrupt
    signal.signal(signal.SIGTERM, interrupted)
    try:
        setup.stage('loopback TLS', setup.tls)
        setup.stage('wallet and dispatch', setup.wallet)
        setup.stage('creation deployment', setup.creation)
        setup.stage('advanced inputs', setup.advanced)
        setup.stage('trading phase two', setup.trading_phase2)
        setup.stage('joined services', setup.joined)
        if args.prepare_only:
            # Observe the candle interface separately from mandatory trade readiness.
            # Never manufacture candle documents or change a gate based on this probe.
            joined = json.loads(Path(setup.env['JOINED_CONFIG_FILE']).read_text())
            chain = json.loads(Path(setup.env['JOINED_CHAIN_FILE']).read_text())
            now = int(time.time())
            candle_query = urllib.parse.urlencode({'interval': '1d',
                'from': time.strftime('%Y-%m-%dT%H:%M:%S.000Z', time.gmtime(now - 3 * 86400)),
                'to': time.strftime('%Y-%m-%dT%H:%M:%S.000Z', time.gmtime(now + 60)), 'limit': '50'})
            probes = []
            for path in ['/v2/snapshots?' + urllib.parse.urlencode({
                    'topics': 'market.candles.' + chain['pool'].lower() + '.1d'}),
                    '/v2/pools/' + chain['pool'].lower() + '/candles?' + candle_query]:
                request = urllib.request.Request(joined['api_origin'] + path, headers={
                    'Origin': joined['stream_application_origin'], 'X-Schema-Version': '2'})
                try:
                    response = urllib.request.urlopen(request, timeout=10)
                except urllib.error.HTTPError as error:
                    response = error
                with response:
                    probes.append({'url': request.full_url, 'status': response.status,
                        'body': response.read().decode()})
            (output / 'joined-candle-interface.json').write_text(json.dumps(probes, indent=2) + '\n')
            degraded = [line for line in (output / 'api.log').read_text().splitlines()
                        if 'development stream projection degraded' in line
                        or 'development stream disabled' in line]
            (output / 'result.json').write_text(json.dumps({
                'command': [sys.executable, str(Path(__file__).resolve()), '--prepare-only'],
                'exit': 0, 'api_degraded_lines': degraded,
                'children': [p.pid for p in setup.children],
                'readinessFailures': setup.readiness_failures}, indent=2) + '\n')
            print('All roles and public snapshot ready; API degraded messages: '
                  + json.dumps(degraded), flush=True)
            return 0
        report = output / 'vitest.json'
        command = [*command, '--reporter=default', '--reporter=json', '--outputFile=' + str(report)]
        runner = setup.start(command, 'vitest', cwd=WEB)
        result = runner.wait()
        sys.stdout.write((output / 'vitest.log').read_text())
        code = result
        if report.is_file():
            results = json.loads(report.read_text())
            failures = [{'target': target['name'], 'name': case['fullName'],
                         'messages': case.get('failureMessages', [])}
                        for target in results.get('testResults', [])
                        for case in target.get('assertionResults', []) if case['status'] == 'failed']
            (output / 'failing-cases.json').write_text(json.dumps(failures, indent=2) + '\n')
            print('Failing case inventory: ' + str(output / 'failing-cases.json'), flush=True)
        (output / 'result.json').write_text(json.dumps({'command': command, 'exit': code,
            'children': [p.pid for p in setup.children], 'readinessFailures': setup.readiness_failures}, indent=2) + '\n')
        return code
    except (OSError, ValueError, RuntimeError, ImportError, subprocess.SubprocessError) as error:
        (output / 'result.json').write_text(json.dumps({'command': command, 'exit': 1,
            'preparationError': str(error), 'readinessFailures': setup.readiness_failures}, indent=2) + '\n')
        print('Fixture preparation failed before Vitest: ' + str(error), file=sys.stderr, flush=True)
        return 1
    finally:
        setup.close()


if __name__ == '__main__':
    sys.exit(main())

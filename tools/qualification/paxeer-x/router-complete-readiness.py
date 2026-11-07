#!/usr/bin/env python3
"""Merged node, consensus and hosted gateway readiness qualification."""
import argparse
import hashlib
import http.client
import json
import os
from pathlib import Path
import re
import ssl
import stat
import subprocess
import sys
import tempfile
import time
from urllib.parse import urlsplit

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(Path(__file__).parent))
import merged_qualification as merged

ROSTER = {'durable_store', 'paxeer_chain', 'consensus'}
EXECUTABLES = {'node': 'paxd', 'gateway': 'layerx-gateway'}
CASES = {'healthy-configured-roster-and-real-read-session-paths',
    'real-authenticated-contracts-and-network-shape-refusals', 'only-node-unavailable',
    'node-restart-retained-state-recovery', 'missing-consensus-configuration-refused',
    'consensus-configuration-restored-with-same-retained-state', 'wrong-network-refused',
    'gateway-restart-same-roster-and-path-recovery', 'configured-roster-omission-refused',
    'configured-roster-restored', 'chain-only-startup-chain-method-and-retired-method-refusal'}


def require(condition, reason):
    if not condition:
        raise RuntimeError(reason)


def private(path, directory=False):
    path = Path(path)
    info = path.lstat()
    require(
        path.is_absolute()
        and path.resolve() == path
        and info.st_uid == os.geteuid()
        and not info.st_mode & 0o077
        and (
            stat.S_ISDIR(info.st_mode)
            if directory
            else stat.S_ISREG(info.st_mode) and info.st_nlink == 1
        ),
        "protected input ownership, path or permissions refused",
    )
    return path


def document(variable):
    require(os.environ.get(variable), variable + " is required")
    path = private(os.environ[variable])
    require(path.stat().st_size <= 1048576, "input document exceeds bound")
    return json.loads(path.read_text())


def git(*arguments):
    return subprocess.check_output(["git", *arguments], cwd=ROOT, text=True).strip()


def artifact(row, revision, name=None):
    path = Path(row["path"])
    require(
        path.is_absolute()
        and path.resolve() == path
        and path.is_file()
        and os.access(path, os.X_OK),
        "prebuilt executable path refused",
    )
    require(
        not path.stat().st_mode & 0o022 and path.stat().st_uid == os.geteuid(),
        "prebuilt executable ownership refused",
    )
    if revision is not None:
        require(row["source_revision"] == revision, "prebuilt source revision differs")
    else:
        require(
            row.get("kind") == "system-package" and row.get("package_version"),
            "system package provenance is required",
        )
    if name is not None:
        require(path.name == name, "unexpected production executable")
    with path.open("rb") as stream:
        require(stream.read(4) == b"\x7fELF", "prebuilt native executable required")
        stream.seek(0)
        require(
            hashlib.file_digest(stream, "sha256").hexdigest() == row["sha256"],
            "prebuilt executable changed",
        )
    return path


def readiness_document(status, body, unavailable=frozenset(), unconfigured=frozenset(), event_producer=False,
                       protocol_network_id=None):
    roster = ROSTER | ({'event_producer'} if event_producer else set())
    require(isinstance(body, dict) and set(body['backends']) == roster,
            'configured dependency roster omitted or added a backend')
    require(not (set(unavailable) | set(unconfigured)) - roster, 'unknown readiness backend')
    for name in roster:
        expected = ({'state': 'unavailable', 'reason': 'not_configured'} if name in unconfigured else
                    {'state': 'unavailable', 'reason': 'unreachable'} if name in unavailable else
                    {'state': 'ready', 'reason': 'ready'})
        require(body['backends'][name] == expected, 'backend state mismatch: ' + name)
    require(status == (503 if unavailable else 200) and body['status'] == ('degraded' if unavailable else 'ready'),
            'gateway readiness status mismatch')
    require(body['readiness_version'] == 1 and body['service'] == 'layerx-gateway'
            and type(body['observed_at_ms']) is int and type(body['valid_until_ms']) is int
            and body['valid_until_ms'] - body['observed_at_ms'] == 30000, 'readiness freshness invalid')
    require(body['components'] == {'durable_store': 'unavailable' if 'durable_store' in unavailable else 'ready'},
            'merged readiness components mismatch')
    chain = body['backends']['paxeer_chain']
    require(body['capabilities']['chain_serving'] == {**chain, 'chain_id': 125}
            and body['capabilities']['layerx_activity'] == {'state': 'not_configured', 'reason': 'not_configured'},
            'merged readiness capability mismatch')
    if protocol_network_id is not None:
        require(type(body['protocol_network_id']) is int and body['protocol_network_id'] == protocol_network_id,
                'wrong protocol network readiness')
    return body


class Run:
    def __init__(self, node, gateway_url, transaction_hash, protocol_network_id, headers=None, context=None, event_producer=False):
        self.client = merged.Client(node, gateway_url, headers, context)
        self.node, self.gateway_url = node, gateway_url
        self.transaction_hash, self.protocol_network_id = transaction_hash, protocol_network_id
        self.headers, self.context, self.event_producer = headers or {}, context, event_producer
        self.cases = []

    def request(self, path):
        parsed = merged.CHAIN.endpoint(self.gateway_url)
        connection = (http.client.HTTPSConnection(parsed.hostname, parsed.port, timeout=20, context=self.context)
                      if parsed.scheme == 'https' else http.client.HTTPConnection(parsed.hostname, parsed.port, timeout=20))
        try:
            connection.request('GET', path, headers={'Accept': 'application/json', **self.headers})
            response = connection.getresponse()
            require(response.getheader('Content-Type') == 'application/json', 'exact response media type required')
            raw = response.read(1_048_577)
            require(len(raw) <= 1_048_576 and not 300 <= response.status < 400, 'bounded nonredirected readiness required')
            return response.status, json.loads(raw)
        finally:
            connection.close()

    def readiness(self, unavailable=frozenset(), unconfigured=frozenset()):
        status, body = self.request('/readyz/core')
        result = readiness_document(status, body, unavailable, unconfigured, self.event_producer, self.protocol_network_id)
        product_status, product = self.request('/readyz')
        require(product_status == status and product['backends'] == body['backends'],
                'public readiness disagrees with configured dependency vector')
        return result

    def usable(self):
        require(self.node.call('eth_chainId', []) == '0x7d', 'merged chain id differs')
        capabilities = self.client.capabilities()
        require(capabilities['programs'] and capabilities['ledger'] and capabilities['pay'] and capabilities['auth'],
                'required merged surfaces are unavailable')
        receipt = self.node.committed(self.transaction_hash)
        # The hosted compatibility receipt must be precisely the node's committed projection.
        status, answer = merged.CHAIN.post(self.gateway_url,
            {'jsonrpc': '2.0', 'id': 'readiness-receipt', 'method': 'pax_sequencerReceipt', 'params': [self.transaction_hash]},
            self.headers, self.context)
        require(status == 200 and answer.get('jsonrpc') == '2.0' and answer.get('id') == 'readiness-receipt'
                and 'error' not in answer and answer.get('result') == self.node.call('pax_sequencerReceipt', [self.transaction_hash]),
                'hosted receipt differs from committed node receipt')
        return receipt

    def retired_refusals(self):
        for method in ('lx_getWalletCaps', 'lx_getBalance', 'lx_getProgram'):
            status, value = merged.CHAIN.post(self.gateway_url,
                {'jsonrpc': '2.0', 'id': method, 'method': method, 'params': []}, self.headers, self.context)
            require(status == 200 and value.get('jsonrpc') == '2.0' and value.get('id') == method
                    and value.get('error', {}).get('code') == -32601 and 'result' not in value,
                    'retired namespace did not produce exact method refusal')

    def record(self, name):
        require(name in CASES and name not in self.cases, 'unexpected or duplicate readiness case')
        self.cases.append(name)


class Gateway:
    def __init__(self, binary, fixture, directory, revision):
        self.binary, self.fixture, self.directory, self.revision = binary, fixture, directory, revision
        self.environment = json.loads(private(fixture['environment_file']).read_text())
        require(isinstance(self.environment, dict) and all(isinstance(k, str) and isinstance(v, str)
                for k, v in self.environment.items()), 'explicit production gateway environment required')
        require(not any(key.startswith('LAYERX_GATEWAY_SEQUENCER_') for key in self.environment),
                'independent sequencer lifecycle is retired')
        require(self.environment['LAYERX_GATEWAY_CONSENSUS_RPC_URL'] == fixture['consensus_url'], 'owned paxd consensus binding required')
        urls = json.loads(self.environment['LAYERX_GATEWAY_PAXEER_RPC_URLS'])
        require(urls == [fixture['node_url']], 'gateway must consume the owned merged node root')
        merged.CHAIN.endpoint(fixture['node_url'])
        require(merged.CHAIN.endpoint(fixture['endpoint']).scheme == 'https', 'real gateway TLS listener required')
        self.process, self.starts = None, 0

    def stop(self):
        if self.process is not None and self.process.poll() is None:
            self.process.terminate()
            try:
                self.process.wait(timeout=10)
            except subprocess.TimeoutExpired:
                self.process.kill()
                self.process.wait(timeout=5)
                require(False, 'gateway shutdown deadline')

    def start(self, changes=None, remove=()):
        self.stop()
        self.starts += 1
        environment = {**self.environment, **(changes or {})}
        for key in remove:
            environment.pop(key, None)
        with (self.directory / f'gateway-{self.starts}.log').open('xb') as stream:
            self.process = subprocess.Popen([str(self.binary)], cwd=ROOT, env=environment,
                stdin=subprocess.DEVNULL, stdout=stream, stderr=subprocess.STDOUT)
        parsed = merged.CHAIN.endpoint(self.fixture['endpoint'])
        deadline = time.monotonic() + 45
        import socket
        while time.monotonic() < deadline:
            require(self.process.poll() is None, 'gateway exited before readiness; inspect private log')
            try:
                with socket.create_connection((parsed.hostname, parsed.port), timeout=.2):
                    return
            except OSError:
                time.sleep(.1)
        require(False, 'gateway listener deadline')


def wait(predicate):
    deadline = time.monotonic() + 45
    while True:
        try:
            return predicate()
        except (OSError, ValueError, RuntimeError, http.client.HTTPException):
            require(time.monotonic() < deadline, 'bounded real readiness transition timed out')
            time.sleep(.1)


def qualify(manifest, fixture, directory, revision):
    merged.source_artifacts(manifest, revision)
    require(fixture['source_revision'] == revision and fixture['version'] == 2, 'source-bound merged fixture v2 required')
    node = merged.OWNED.PaxdFixture(fixture['paxd'], manifest['artifacts']['paxd']['path'], directory)
    gateway = Gateway(artifact(manifest['artifacts']['gateway'], revision, 'layerx-gateway'), fixture['gateway'], directory, revision)
    context = ssl.create_default_context(cafile=str(private(fixture['ca_pem'])))
    context.load_cert_chain(str(private(fixture['client_certificate_pem'])), str(private(fixture['client_key_pem'])))
    run = Run(node.node, fixture['gateway']['endpoint'], fixture['transaction_hash'], fixture['protocol_network_id'],
              context=context, event_producer=fixture['event_producer'])
    try:
        node.start()
        gateway.start()
        baseline = wait(run.readiness)
        committed = run.usable()
        run.record('healthy-configured-roster-and-real-read-session-paths')
        run.retired_refusals()
        run.record('real-authenticated-contracts-and-network-shape-refusals')
        node.stop()
        wait(lambda: run.readiness({'paxeer_chain', 'consensus'}))
        run.record('only-node-unavailable')
        node.start()
        wait(run.readiness)
        require(run.usable() == committed, 'node restart changed committed execution')
        run.record('node-restart-retained-state-recovery')
        gateway.start(remove=('LAYERX_GATEWAY_CONSENSUS_RPC_URL',))
        wait(lambda: run.readiness(unconfigured={'consensus'}))
        try:
            run.readiness()
        except RuntimeError:
            pass
        else:
            require(False, 'missing configured consensus escaped qualification')
        run.record('missing-consensus-configuration-refused')
        gateway.start()
        wait(run.readiness)
        require(run.usable() == committed, 'configuration restore changed committed execution')
        run.record('consensus-configuration-restored-with-same-retained-state')
        wrong = fixture['protocol_network_id'] % (2**32 - 1) + 1
        gateway.start({'LAYERX_GATEWAY_PROTOCOL_NETWORK_ID': str(wrong)})
        status, body = run.request('/readyz/core')
        try:
            readiness_document(status, body, event_producer=fixture['event_producer'], protocol_network_id=fixture['protocol_network_id'])
        except RuntimeError:
            pass
        else:
            require(False, 'wrong network escaped readiness qualification')
        run.record('wrong-network-refused')
        gateway.start()
        wait(run.readiness)
        require(run.readiness()['backends'] == baseline['backends'] and run.usable() == committed,
                'gateway restart changed roster or committed receipt')
        run.record('gateway-restart-same-roster-and-path-recovery')
        for omitted in ROSTER:
            observed = dict(baseline, backends=dict(baseline['backends']))
            del observed['backends'][omitted]
            try:
                readiness_document(200, observed, event_producer=fixture['event_producer'])
            except RuntimeError:
                pass
            else:
                require(False, 'omitted configured backend escaped gate roster requirement')
        run.record('configured-roster-omission-refused')
        wait(run.readiness)
        run.usable()
        run.record('configured-roster-restored')
        require(not any(key in gateway.environment for key in ('LAYERX_GATEWAY_COMPONENT_URL', 'LAYERX_GATEWAY_PUBLIC_CORE_URL')),
                'readiness must not require an independent execution service')
        run.retired_refusals()
        run.usable()
        run.record('chain-only-startup-chain-method-and-retired-method-refusal')
        require(set(run.cases) == CASES, 'incomplete merged readiness scenarios')
        return run.cases
    finally:
        gateway.stop()
        node.stop()


def main():
    parser = argparse.ArgumentParser()
    parser.parse_args()
    os.umask(0o077)
    require(not git('status', '--porcelain', '--untracked-files=no'), 'committed source required')
    revision = git('rev-parse', 'HEAD')
    manifest = document('PAXEER_X_ROUTER_READINESS_BUILD_MANIFEST')
    fixture = document('PAXEER_X_ROUTER_READINESS_FIXTURE')
    require(manifest['source_tree'] == git('rev-parse', 'HEAD^{tree}') and manifest['build_exit'] == 0,
            'source-bound successful scoped build required')
    parent = private(os.environ['PAXEER_X_EVIDENCE_DIR'], directory=True)
    directory = Path(tempfile.mkdtemp(prefix='merged-readiness-', dir=parent))
    record = {'revision': revision, 'exit_code': 1, 'cases': [], 'skipped': 0}
    try:
        record['cases'] = qualify(manifest, fixture, directory, revision)
        require(git('rev-parse', 'HEAD') == revision and not git('status', '--porcelain', '--untracked-files=no'),
                'source changed during qualification')
        merged.source_artifacts(manifest, revision)
        record['exit_code'] = 0
    finally:
        (directory / 'result.json').write_text(json.dumps(record, indent=2))
    return record['exit_code']


if __name__ == '__main__':
    try:
        sys.exit(main())
    except (OSError, ValueError, RuntimeError, KeyError, TypeError, subprocess.SubprocessError) as error:
        print('router-complete-readiness: refused: ' + str(error), file=sys.stderr)
        sys.exit(1)

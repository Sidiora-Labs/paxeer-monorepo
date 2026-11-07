#!/usr/bin/env python3
import hashlib
import http.server
import json
import os
from pathlib import Path
import pwd
import shutil
import signal
import socket
import subprocess
import tempfile
import threading
import time
import urllib.error
import urllib.parse
import urllib.request

ROOT = Path(__file__).resolve().parents[3]
STATE = None
PROCESSES = []


def require(value, message):
    if not value:
        raise RuntimeError(message)


def local_url(value):
    url = urllib.parse.urlparse(value)
    require(url.hostname in ('localhost', '127.0.0.1', '::1'), 'fixture endpoint must be loopback-only')
    return value


def protected(path):
    path = Path(path)
    stat = path.lstat()
    require(path.is_file() and not path.is_symlink() and stat.st_uid == os.getuid()
            and stat.st_nlink == 1 and stat.st_mode & 0o077 == 0, 'protected fixture file required')
    return path


def stop(process):
    if process.poll() is None:
        os.killpg(process.pid, signal.SIGKILL)
        process.wait(timeout=10)


def start(argv, environment, name, identity=None):
    with open(STATE / (name + '.log'), 'ab', buffering=0) as output:
        process = subprocess.Popen(argv, cwd=STATE if identity else ROOT, env=environment,
                                   stdout=output, stderr=output, start_new_session=True, **(identity or {}))
    PROCESSES.append(process)
    return process


NATIVE_FLOW_PATHS = (
    'apps/wallet/src/account/AccountView.tsx',
    'apps/wallet/src/account/KernelPlan.tsx',
    'apps/wallet/src/account/AccountProvider.tsx',
    'apps/wallet/src/account/hooks.ts',
    'apps/wallet/src/widgets/send/SendWidget.tsx',
    'apps/wallet/src/widgets/send/useSendForm.ts',
)

DRIVER = r'''
import assert from 'node:assert/strict';
import fs from 'node:fs';
import {resolve, relative, isAbsolute, join} from 'node:path';
import {pathToFileURL} from 'node:url';
const runtime = JSON.parse(fs.readFileSync(process.env.WALLET_SIGNING_ADAPTERS_RUNTIME, 'utf8'));
const sdk = await import(pathToFileURL(runtime.sdk_entry).href);
function secret(name) {
  const path = resolve(runtime.fixture_dir, name);
  const inside = relative(runtime.fixture_dir, path);
  assert(inside && !inside.startsWith('..') && !isAbsolute(inside));
  const info = fs.lstatSync(path);
  assert(info.isFile() && !info.isSymbolicLink() && info.nlink === 1
    && info.uid === process.getuid() && (info.mode & 0o077) === 0);
  return fs.readFileSync(path, 'utf8').trim();
}
const token = secret(runtime.owner_token_file);
const foreign = secret(runtime.foreign_owner_token_file);
const cases = [];
let lastSend;
const observedFetch = async (input, init) => {
  const response = await fetch(input, init);
  if (String(input).endsWith('/v1/wallet/send')) {
    lastSend = {status: response.status, request: JSON.parse(init.body), body: await response.clone().json()};
  }
  return response;
};
function provider() {
  return new sdk.PaxeerProvider({gatewayUrl: runtime.gateway_url, rpcUrl: runtime.rpc_url,
    chainId: runtime.chain_id, token: () => token, fetch: observedFetch,
    confirm: async request => {
      assert(request.method && Array.isArray(request.params));
      return true;
    }});
}
async function control(action) {
  const response = await fetch(runtime.control_url + '/control/' + action,
    {method: 'POST', headers: {'Content-Type': 'application/json'}, body: '{}'});
  assert.equal(response.status, 200);
  return response.json();
}
async function rpc(method, params) {
  const response = await fetch(runtime.real_rpc_url, {method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({jsonrpc: '2.0', id: 1, method, params})});
  assert.equal(response.status, 200);
  const answer = await response.json();
  assert.equal(answer.jsonrpc, '2.0'); assert.equal(answer.id, 1); assert(!answer.error);
  return answer.result;
}
const original = provider();
const wallet = new sdk.WalletInterface(original);
try {
  assert.equal(runtime.chain_id, 125);
  assert.equal(runtime.attestors.length, 5);
  const [account] = await wallet.accounts();
  assert.equal(account.toLowerCase(), runtime.account.toLowerCase());
  const tx = {...runtime.native_transaction, value: BigInt(runtime.native_transaction.value), chainId: 125, data: '0x'};
  assert(/^0x[0-9a-fA-F]{40}$/.test(tx.to) && tx.value > 0n);
  await assert.rejects(wallet.sendTransaction({...tx, chainId: 126}), error => error.code === 4901);
  const typed = runtime.typed_data;
  assert.equal(Number(typed.domain.chainId), 125);
  const wrong = structuredClone(typed); wrong.domain.chainId = 126;
  await assert.rejects(wallet.signTypedData(wrong), error => error.code === 4901);
  const signature = await wallet.signTypedData(typed);
  assert(/^0x[0-9a-fA-F]{130}$/.test(signature));
  const unauthorized = await fetch(runtime.gateway_url + '/v1/wallet/send', {method: 'POST',
    headers: {'Content-Type': 'application/json', Authorization: 'Bearer ' + foreign},
    body: JSON.stringify({tx: {chainId: 125, from: account, to: tx.to, value: String(tx.value), data: '0x'}})});
  assert([401,403,404].includes(unauthorized.status));
  const bare = await fetch(runtime.gateway_url + '/v1/wallet/sign-digest', {method: 'POST',
    headers: {'Content-Type': 'application/json', Authorization: 'Bearer ' + token},
    body: JSON.stringify({digest: '0x' + '01'.repeat(32)})});
  assert.equal(bare.status, 400);
  cases.push('native-owner-and-chain-refusal', 'typed-signing', 'bare-digest-refusal');

  assert.equal((await control('stop-two')).stopped, 2);
  const hash = await wallet.sendTransaction(tx);
  assert(/^0x[0-9a-fA-F]{64}$/.test(hash));
  const transaction = await rpc('eth_getTransactionByHash', [hash]);
  assert.equal(transaction.from.toLowerCase(), account.toLowerCase());
  assert.equal(transaction.to.toLowerCase(), tx.to.toLowerCase());
  assert.equal(BigInt(transaction.value), tx.value);
  assert.equal(BigInt(transaction.chainId), 125n);
  cases.push('three-of-five-native-signing');

  await control('arm-custody');
  await assert.rejects(wallet.sendTransaction(tx), error => error.code === -32602);
  assert.equal(lastSend.status, 202);
  const pending = lastSend.body;
  assert.equal(pending.error, 'outcome_unknown');
  assert.equal(pending.retry, 'poll_receipt');
  assert.equal(pending.poll, 'eth_getTransactionReceipt');
  assert.equal(pending.must_not_resubmit, true);
  assert(/^0x[0-9a-fA-F]{64}$/.test(pending.tx_hash));
  assert.equal((await control('state')).dropped_reply, true);
  const retained = join(runtime.evidence_dir, 'native-pending.json');
  fs.writeFileSync(retained, JSON.stringify(lastSend), {mode: 0o600, flag: 'wx'});
  original.disconnect();
  await control('restart');
  await control('receipts');
  const reopened = provider();
  try {
    assert.equal((await new sdk.WalletInterface(reopened).accounts())[0].toLowerCase(), account.toLowerCase());
    const saved = JSON.parse(fs.readFileSync(retained, 'utf8'));
    assert.deepEqual(saved, lastSend);
    const answer = await sdk.waitForSequencerReceipt(
      (method, params) => reopened.request({method, params}), saved.body.tx_hash, {rung: 'committed'});
    assert.equal(answer.txHash, saved.body.tx_hash);
    assert.equal(answer.rung, 'committed'); assert.equal(answer.status, '0x1');
    const recovered = await rpc('eth_getTransactionByHash', [saved.body.tx_hash]);
    assert.equal(BigInt(recovered.nonce), BigInt(saved.body.nonce));
    assert.equal(recovered.from.toLowerCase(), account.toLowerCase());
    assert.equal(recovered.to.toLowerCase(), tx.to.toLowerCase());
    assert.equal(BigInt(recovered.value), tx.value);
    assert((await control('state')).send_counts.every(count => count === 1));
    cases.push('ambiguous-native-broadcast-reopen-without-resubmit');
    await control('lose-quorum');
    await assert.rejects(new sdk.WalletInterface(reopened).sendTransaction(tx), error => error.code === -32603 && error.status === 503);
    assert.equal(lastSend.status, 503); assert(!lastSend.body.signature && !lastSend.body.tx_hash);
    cases.push('two-signers-refused-without-local-fallback');
  } finally {reopened.disconnect();}
  fs.writeFileSync(join(runtime.evidence_dir, 'cases-result.json'),
    JSON.stringify({complete: true, cases}), {mode: 0o600});
} finally {wallet.dispose(); original.disconnect();}
'''


def manifest_paths(root=ROOT):
    expected = set()
    for relative in ('services/wallet/gateway/src', 'services/wallet/gateway/dist', 'services/wallet/gateway/migrations',
                     'sdk/wallet/src', 'sdk/wallet/dist',
                     'services/agent/sdk/typescript/src', 'services/agent/sdk/typescript/dist',
                     'services/interop/crates/layerx-gas-station/src'):
        expected.update(str(path.relative_to(root)) for path in (root / relative).rglob('*') if path.is_file())
    expected.update(str(path.relative_to(root)) for path in (root / 'services/wallet/attestor').rglob('*.go'))
    expected.update(('services/wallet/attestor/go.mod', 'services/wallet/attestor/go.sum',
                     *NATIVE_FLOW_PATHS,
                     'tools/qualification/paxeer-x/wallet-signing-adapters.py',
                     'services/wallet/gateway/package.json', 'sdk/wallet/package.json',
                     'services/agent/sdk/typescript/package.json', 'services/interop/crates/layerx-gas-station/Cargo.toml'))
    return expected


def validate_manifest(manifest, root=ROOT):
    require(set(manifest) == manifest_paths(root),
            'complete current source and prebuilt artifact manifest required')
    for relative, digest in manifest.items():
        require(not any(part == '.env' or part.startswith('.env.') for part in Path(relative).parts),
                'credential files are not source artifacts')
        path = root / relative
        require(path.is_file() and not path.is_symlink(), 'ordinary source/artifact required')
        require(hashlib.sha256(path.read_bytes()).hexdigest() == digest,
                'source/prebuilt artifact mismatch: ' + relative)


def emit_driver(directory):
    path = Path(directory) / 'native-signing-adapters.mjs'
    descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(descriptor, 'w') as output:
        output.write(DRIVER)
    return path


def main():
    global STATE
    supplied = os.environ.get('WALLET_SIGNING_ADAPTERS_FIXTURE_BUNDLE')
    require(supplied, 'prerequisite: WALLET_SIGNING_ADAPTERS_FIXTURE_BUNDLE with approved real isolated services')
    bundle = Path(supplied).resolve()
    require(bundle.is_dir() and bundle.stat().st_mode & 0o077 == 0, 'private fixture directory required')
    require(not any(path.is_symlink() for path in bundle.rglob('*')), 'fixture symlinks refused')
    require(not any(path.name == '.env' or path.name.startswith('.env.') for path in bundle.rglob('*')),
            'fixture bundle must not include .env files')
    config = json.loads(protected(bundle / 'wallet-signing-adapters-fixture.json').read_text())
    require(config.get('version') == 1 and config.get('isolated') is True, 'isolated version 1 fixture required')
    manifest_path = (bundle / config['source_artifact_manifest']).resolve()
    require(manifest_path.is_relative_to(bundle), 'source manifest must remain inside protected bundle')
    manifest = json.loads(protected(manifest_path).read_text())
    validate_manifest(manifest)
    require('driver' not in config['artifacts'], 'native driver is emitted from sealed qualifier source')
    for name, artifact in config['artifacts'].items():
        path = Path(artifact['path'])
        require(path.is_file() and os.access(path, os.X_OK), 'prebuilt artifact missing: ' + name)
        require(hashlib.sha256(path.read_bytes()).hexdigest() == artifact['sha256'], 'executable digest mismatch: ' + name)
        require(isinstance(artifact['source_revision'], str) and len(artifact['source_revision']) == 40,
                'executable source provenance missing: ' + name)
    STATE = Path(tempfile.mkdtemp(prefix='wallet-signing-adapters-', dir=os.environ.get('WALLET_SIGNING_ADAPTERS_EVIDENCE_ROOT')))
    STATE.chmod(0o700)
    driver_artifact = emit_driver(STATE)
    copied = STATE / 'fixture'
    shutil.copytree(bundle, copied)
    def expand(value):
        if isinstance(value, str):
            return value.replace('{fixture}', str(copied)).replace('{state}', str(STATE))
        if isinstance(value, list):
            return [expand(item) for item in value]
        if isinstance(value, dict):
            return {key: expand(item) for key, item in value.items()}
        return value
    config = expand(config)
    environment = {key: os.environ[key] for key in ('PATH', 'LANG', 'LC_ALL', 'TZ') if key in os.environ}
    services = config['services']
    for kind, count in (('attestor', 5), ('postgres', 1), ('supabase', 1), ('chain', 1), ('station', 1)):
        require(sum(item['kind'] == kind for item in services) == count, 'required real service cardinality: ' + kind)
    require(all(item['kind'] in ('attestor', 'postgres', 'supabase', 'chain', 'station') for item in services), 'unknown fixture service')
    database = Path(config['postgres_data_dir']).resolve()
    baseline = Path(config['postgres_baseline']).resolve()
    require(database.parent == STATE and baseline.is_relative_to(copied), 'independent retained database state required')
    shutil.copytree(baseline, database)
    database_identity = None
    if os.getuid() == 0:
        owner = pwd.getpwnam('postgres')
        database_identity = {'user': owner.pw_uid, 'group': owner.pw_gid}
        os.chown(STATE, owner.pw_uid, owner.pw_gid)
        for path in [database, *database.rglob('*')]:
            require(not path.is_symlink(), 'database snapshot symlinks refused')
            os.chown(path, owner.pw_uid, owner.pw_gid)
    running = []
    def launch_service(item, index):
        env = dict(environment)
        env.update(item['env'])
        for key, value in item['env'].items():
            if key.endswith('_URL') and urllib.parse.urlparse(value).hostname:
                local_url(value)
        return start([config['artifacts'][item['artifact']]['path'], *item['argv']], env,
                     item['kind'] + '-' + str(index), database_identity if item['kind'] == 'postgres' else None)
    def service_ready(item, process):
        endpoint = urllib.parse.urlparse(local_url(item['ready_tcp']))
        require(endpoint.port is not None, 'explicit actual service readiness port required')
        deadline = time.monotonic() + 25
        while time.monotonic() < deadline:
            require(process.poll() is None, 'fixture service exited before readiness: ' + item['kind'])
            try:
                with socket.create_connection((endpoint.hostname, endpoint.port), timeout=1):
                    return
            except OSError:
                time.sleep(0.1)
        raise RuntimeError('actual fixture readiness deadline: ' + item['kind'])
    for index, item in enumerate(services):
        running.append(launch_service(item, index))
        service_ready(item, running[-1])
    upstream = local_url(config['chain_rpc_url'])
    block_receipts = threading.Event()
    drop_send_reply = threading.Event()
    dropped_reply = threading.Event()
    send_counts = {}
    count_lock = threading.Lock()
    gateway = None
    gateway_env = dict(environment)
    gateway_env.update(config['gateway_env'])
    require(gateway_env.get('ATTESTOR_QUORUM') == '3', 'custody threshold must remain three')
    endpoints = gateway_env['ATTESTOR_ENDPOINTS'].split(',')
    require(len(set(endpoints)) == 5, 'exactly five approved custody endpoints required')
    for endpoint in endpoints:
        local_url(endpoint)
    gateway_url = local_url(config['gateway_url'])
    def launch_gateway(name):
        return start([config['artifacts']['node']['path'], str(ROOT / 'services/wallet/gateway/dist/index.js')], gateway_env, name)
    def ready():
        deadline = time.monotonic() + 90
        while time.monotonic() < deadline:
            require(gateway.poll() is None, 'gateway exited before readiness; inspect retained log')
            try:
                with urllib.request.urlopen(gateway_url + '/readyz', timeout=2) as response:
                    if response.status == 200:
                        return
            except (OSError, urllib.error.URLError):
                pass
            time.sleep(0.25)
        raise RuntimeError('actual gateway readiness deadline exceeded')
    class Proxy(http.server.BaseHTTPRequestHandler):
        def log_message(self, *_):
            pass
        def do_POST(self):
            nonlocal gateway
            data = self.rfile.read(int(self.headers.get('content-length', '0')))
            if self.path == '/control/arm-custody':
                block_receipts.set(); drop_send_reply.set(); dropped_reply.clear()
                return self.answer({'armed': True})
            if self.path == '/control/receipts':
                block_receipts.clear()
                return self.answer({'available': True})
            if self.path == '/control/restart':
                stop(gateway)
                station_index = next(index for index, item in enumerate(services) if item['kind'] == 'station')
                stop(running[station_index])
                running[station_index] = launch_service(services[station_index], station_index)
                service_ready(services[station_index], running[station_index])
                gateway = launch_gateway('gateway-restarted')
                ready()
                return self.answer({'restarted': True})
            if self.path == '/control/stop-two':
                for index in [index for index, item in enumerate(services) if item['kind'] == 'attestor'][:2]:
                    stop(running[index])
                return self.answer({'stopped': 2})
            if self.path == '/control/lose-quorum':
                for index in [index for index, item in enumerate(services) if item['kind'] == 'attestor'][:3]:
                    stop(running[index])
                return self.answer({'stopped': 3})
            if self.path == '/control/state':
                with count_lock:
                    counts = list(send_counts.values())
                return self.answer({'dropped_reply': dropped_reply.is_set(), 'send_counts': counts})
            request = json.loads(data)
            if request.get('method') in ('eth_getTransactionReceipt', 'pax_sequencerReceipt') and block_receipts.is_set():
                self.close_connection = True
                return
            if request.get('method') == 'eth_sendRawTransaction':
                with count_lock:
                    raw = request['params'][0]
                    send_counts[raw] = send_counts.get(raw, 0) + 1
            try:
                forwarded = urllib.request.Request(upstream, data=data, headers={'Content-Type': 'application/json'})
                with urllib.request.urlopen(forwarded, timeout=120) as response:
                    body = response.read()
                if request.get('method') == 'eth_sendRawTransaction' and drop_send_reply.is_set():
                    drop_send_reply.clear(); dropped_reply.set()
                    self.close_connection = True
                    return
                self.send_response(200); self.send_header('Content-Type', 'application/json')
                self.send_header('Content-Length', str(len(body))); self.end_headers(); self.wfile.write(body)
            except (OSError, urllib.error.URLError):
                self.close_connection = True
        def answer(self, body):
            encoded = json.dumps(body).encode()
            self.send_response(200); self.send_header('Content-Type', 'application/json')
            self.send_header('Content-Length', str(len(encoded))); self.end_headers(); self.wfile.write(encoded)
    proxy = http.server.ThreadingHTTPServer(('127.0.0.1', 0), Proxy)
    proxy.daemon_threads = True
    threading.Thread(target=proxy.serve_forever, daemon=True).start()
    proxy_url = 'http://127.0.0.1:' + str(proxy.server_port)
    gateway_env.update(HYPERPAXEER_RPC_URL=proxy_url, RPC_URLS=proxy_url)
    local_url(gateway_env['DATABASE_URL'])
    local_url(gateway_env['SUPABASE_URL'])
    local_url(gateway_env['WALLET_GAS_STATION_URL'])
    gateway = launch_gateway('gateway')
    ready()
    runtime = dict(config['cases'])
    runtime.update(gateway_url=gateway_url, rpc_url=proxy_url, real_rpc_url=upstream,
                   control_url=proxy_url, sdk_entry=str(ROOT / 'sdk/wallet/dist/index.js'),
                   evidence_dir=str(STATE), fixture_dir=str(copied), database_url=gateway_env['DATABASE_URL'])
    runtime_path = STATE / 'runtime.json'
    runtime_path.write_text(json.dumps(runtime)); runtime_path.chmod(0o600)
    driver_env = dict(environment)
    driver_env['WALLET_SIGNING_ADAPTERS_RUNTIME'] = str(runtime_path)
    driver = start([config['artifacts']['node']['path'], str(driver_artifact)], driver_env, 'cases')
    require(driver.wait(timeout=780) == 0, 'actual SDK adapter cases failed; inspect retained cases.log')
    report = json.loads((STATE / 'cases-result.json').read_text())
    require(report.get('complete') is True, 'actual SDK driver did not complete')
    result = {'status': 'passed', 'exit_code': 0, 'cases': report['cases']}
    (STATE / 'result.json').write_text(json.dumps(result, indent=2))
    print(json.dumps({'status': 'passed', 'evidence': str(STATE)}))
    proxy.shutdown()


if __name__ == '__main__':
    signal.signal(signal.SIGTERM, lambda *_: (_ for _ in ()).throw(KeyboardInterrupt()))
    try:
        main()
    except (Exception, KeyboardInterrupt) as error:
        reason = str(error) if isinstance(error, RuntimeError) else type(error).__name__
        result = {'status': 'failed', 'exit_code': 2, 'reason': reason, 'evidence': str(STATE) if STATE else None}
        if STATE:
            (STATE / 'result.json').write_text(json.dumps(result, indent=2))
        print(json.dumps(result))
        raise SystemExit(2)
    finally:
        for process in reversed(PROCESSES):
            stop(process)

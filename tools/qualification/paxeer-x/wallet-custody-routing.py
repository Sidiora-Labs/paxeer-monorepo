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
import stat
import subprocess
import tempfile
import threading
import time
import urllib.error
import urllib.parse
import urllib.request

ROOT = Path(__file__).resolve().parents[3]
GATEWAY = ROOT / 'services/wallet/gateway'
PROCESSES = []
STATE = None
PG_STOP = None


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def private_info(info, directory=False):
    require((stat.S_ISDIR(info.st_mode) if directory else stat.S_ISREG(info.st_mode))
            and info.st_uid == os.getuid() and info.st_mode & 0o077 == 0
            and (directory or info.st_nlink == 1), 'private caller-owned ordinary bundle entry required')


def bundle_path(bundle, relative):
    require(isinstance(relative, str) and relative and not Path(relative).is_absolute()
            and all(part not in ('', '.', '..') for part in relative.split('/')),
            'bundle reference must be a contained relative path')
    return bundle / relative


def bundle_open(bundle, relative=None, directory=False):
    """Open each component relative to an admitted directory, never following links."""
    bundle = Path(bundle).absolute()
    for parent in (*reversed(bundle.parents), bundle):
        require(not parent.is_symlink(), 'bundle ancestor symlinks refused')
    descriptor = os.open(bundle, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    try:
        private_info(os.fstat(descriptor), True)
        if relative is not None:
            bundle_path(bundle, relative)
            parts = relative.split('/')
            for index, part in enumerate(parts):
                is_directory = index < len(parts) - 1 or directory
                flags = os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK
                if is_directory:
                    flags |= os.O_DIRECTORY
                child = os.open(part, flags, dir_fd=descriptor)
                os.close(descriptor)
                descriptor = child
                private_info(os.fstat(descriptor), is_directory)
        return descriptor
    except BaseException:
        os.close(descriptor)
        raise


def bundle_read(bundle, relative):
    with os.fdopen(bundle_open(bundle, relative), 'rb') as stream:
        return stream.read()


def bundle_entries(bundle, relative=None):
    descriptor = bundle_open(bundle, relative, directory=True)
    try:
        with os.scandir(descriptor) as entries:
            for entry in sorted(entries, key=lambda item: item.name):
                require(entry.name != '.env' and not entry.name.startswith('.env.'),
                        'fixture bundles must not contain .env files')
                name = entry.name if relative is None else relative + '/' + entry.name
                info = entry.stat(follow_symlinks=False)
                is_directory = stat.S_ISDIR(info.st_mode)
                private_info(info, is_directory)
                if is_directory:
                    yield name, True
                    yield from bundle_entries(bundle, name)
                else:
                    yield name, False
    finally:
        os.close(descriptor)


def load_bundle(supplied):
    bundle = Path(supplied).absolute()
    # Admit the entire credential-bearing tree before reading even its manifest.
    list(bundle_entries(bundle))
    config = json.loads(bundle_read(bundle, 'wallet-custody-fixture.json'))
    require(config.get('version') == 1 and config.get('isolated') is True,
            'isolated fixture version 1 required')
    for key in ('artifact_manifest', 'attestor_source_manifest', 'postgres_dump'):
        with os.fdopen(bundle_open(bundle, config[key]), 'rb'):
            pass
    bundle_path(bundle, config['node_binary'])
    for binary in ('initdb', 'pg_ctl', 'pg_restore'):
        relative = str(Path(config['postgres_bin_dir']) / binary)
        bundle_path(bundle, relative)
    for service in config['services']:
        bundle_path(bundle, service['executable'])
    def references(value, key=''):
        if isinstance(value, dict):
            for name, item in value.items():
                references(item, name)
        elif isinstance(value, list):
            for item in value:
                references(item, key)
        elif isinstance(value, str):
            if '{fixture}' in value:
                require(value.startswith('{fixture}/') and value.count('{fixture}') == 1,
                        'fixture path must remain in bundle')
                descriptor = bundle_open(bundle, value[len('{fixture}/'):], directory=key.endswith('_DIR'))
                os.close(descriptor)
            elif key.endswith(('_FILE', '_DIR')):
                require(value.startswith('{state}/') and '..' not in Path(value).parts,
                        'credential paths must remain in bundle or private generated state')
    references(config.get('gateway_env', {}))
    references(config['services'])
    return bundle, config


def copy_bundle(bundle, destination):
    entries = list(bundle_entries(bundle))
    destination.mkdir(mode=0o700)
    for relative, directory in entries:
        target = destination / relative
        if directory:
            target.mkdir(mode=0o700)
        else:
            descriptor = bundle_open(bundle, relative)
            with os.fdopen(descriptor, 'rb') as source:
                mode = stat.S_IMODE(os.fstat(source.fileno()).st_mode)
                with target.open('xb') as output:
                    os.fchmod(output.fileno(), mode)
                    shutil.copyfileobj(source, output)


def local_url(value):
    parsed = urllib.parse.urlparse(value)
    require(parsed.hostname in ('localhost', '127.0.0.1', '::1'), 'fixture service must be loopback-only')
    return value


def free_port():
    with socket.socket() as listener:
        listener.bind(('127.0.0.1', 0))
        return listener.getsockname()[1]


def run(argv, **kwargs):
    return subprocess.run(argv, check=True, timeout=60, stdout=subprocess.DEVNULL,
                          stderr=subprocess.DEVNULL, **kwargs)


def start(argv, env, name):
    log = open(STATE / (name + '.log'), 'ab', buffering=0)
    process = subprocess.Popen(argv, cwd=GATEWAY, env=env, stdout=log, stderr=subprocess.STDOUT,
                               start_new_session=True)
    log.close()
    PROCESSES.append(process)
    return process


def stop(process):
    if process.poll() is None:
        os.killpg(process.pid, signal.SIGKILL)
        process.wait(timeout=10)


def main():
    global STATE, PG_STOP
    bundle_arg = os.environ.get('WALLET_CUSTODY_FIXTURE_BUNDLE')
    require(bundle_arg, 'prerequisite: WALLET_CUSTODY_FIXTURE_BUNDLE with approved isolated real-service fixtures')
    bundle, config = load_bundle(bundle_arg)
    artifacts = json.loads(bundle_read(bundle, config['artifact_manifest']))
    for directory in ('src', 'dist'):
        actual = {str(p.relative_to(GATEWAY)) for p in (GATEWAY / directory).rglob('*') if p.is_file()}
        expected = {p for p in artifacts if p.startswith(directory + '/')}
        require(actual == expected and actual, 'prerequisite: complete source and prebuilt artifact manifest')
    for relative, digest in artifacts.items():
        path = (GATEWAY / relative).resolve()
        require(path.is_relative_to(GATEWAY) and '.env' not in path.name, 'invalid artifact manifest path')
        require(hashlib.sha256(path.read_bytes()).hexdigest() == digest, 'prebuilt artifact/source digest mismatch: ' + relative)
    require((GATEWAY / 'node_modules/tsx').is_dir(), 'prerequisite: installed tsx runtime; no install/build allowed')
    attestor_sources = json.loads(bundle_read(bundle, config['attestor_source_manifest']))
    source_files = {str(p.relative_to(ROOT)) for p in (ROOT / 'services/wallet/attestor').rglob('*.go')}
    source_files.update({'services/wallet/attestor/go.mod', 'services/wallet/attestor/go.sum'})
    require(set(attestor_sources) == source_files, 'complete attestor source manifest required')
    for relative, digest in attestor_sources.items():
        require(hashlib.sha256((ROOT / relative).read_bytes()).hexdigest() == digest, 'attestor source digest mismatch: ' + relative)
    require(hashlib.sha256(bundle_read(bundle, config['node_binary'])).hexdigest() == config['node_sha256'], 'node artifact digest mismatch')
    services = config['services']
    require(sum(s['kind'] == 'attestor' for s in services) == 5, 'exactly five real attestor processes required')
    require(sum(s['kind'] == 'chain' for s in services) == 1, 'one real chain process required')
    require(sum(s['kind'] == 'supabase' for s in services) == 1, 'one real Supabase auth process required')
    require(all(service['kind'] in ('attestor', 'chain', 'supabase', 'supabase_proxy') for service in services), 'unknown fixture service')
    require(sum(service['kind'] == 'supabase_proxy' for service in services) <= 1, 'at most one real Supabase ingress process')
    STATE = Path(tempfile.mkdtemp(prefix='wallet-custody-routing-', dir=os.environ.get('WALLET_CUSTODY_EVIDENCE_ROOT')))
    STATE.chmod(0o700)
    copied = STATE / 'fixture'
    copy_bundle(bundle, copied)
    config['node_binary'] = str(bundle_path(copied, config['node_binary']))
    config['postgres_bin_dir'] = str(bundle_path(copied, config['postgres_bin_dir']))
    for service in services:
        service['executable'] = str(bundle_path(copied, service['executable']))
    def expand(value):
        return value.replace('{state}', str(STATE)).replace('{fixture}', str(copied)).replace('{database_url}', env.get('DATABASE_URL', ''))
    env = {key: value for key, value in os.environ.items() if not key.startswith(('DATABASE_', 'ATTESTOR_', 'WALLET_', 'SUPABASE_', 'HYPERPAXEER_', 'RPC_', 'LAYER'))}
    env.update({key: expand(str(value)) for key, value in config['gateway_env'].items()})
    for key in ('SUPABASE_URL', 'HYPERPAXEER_RPC_URL'):
        local_url(env[key])
    require(env.get('ATTESTOR_QUORUM') == '3', 'existing threshold 3 must be preserved')
    endpoints = env['ATTESTOR_ENDPOINTS'].split(',')
    require(len(set(endpoints)) == 5, 'five distinct attestor endpoints required')
    for endpoint in endpoints:
        local_url(endpoint)
    pg_bin = Path(config['postgres_bin_dir'])
    for binary in ('initdb', 'pg_ctl', 'pg_restore'):
        require((pg_bin / binary).is_file(), 'prerequisite: prebuilt PostgreSQL ' + binary)
        require(hashlib.sha256((pg_bin / binary).read_bytes()).hexdigest() == config['postgres_sha256'][binary], 'PostgreSQL artifact digest mismatch')
    pg_dir = STATE / 'postgres'
    pg_dir.mkdir(mode=0o700)
    owner_prefix = []
    if os.getuid() == 0:
        owner = pwd.getpwnam('postgres')
        os.chown(STATE, owner.pw_uid, owner.pw_gid)
        os.chown(pg_dir, owner.pw_uid, owner.pw_gid)
        owner_prefix = ['runuser', '-u', 'postgres', '--']
    pg_port = free_port()
    run(owner_prefix + [str(pg_bin / 'initdb'), '-D', str(pg_dir / 'data'), '-A', 'trust', '-U', 'postgres'])
    run(owner_prefix + [str(pg_bin / 'pg_ctl'), '-D', str(pg_dir / 'data'), '-l', str(pg_dir / 'server.log'),
                       '-o', f'-p {pg_port} -h 127.0.0.1 -k {pg_dir}', '-w', 'start'])
    PG_STOP = owner_prefix + [str(pg_bin / 'pg_ctl'), '-D', str(pg_dir / 'data'), '-m', 'fast', '-w', 'stop']
    env['DATABASE_URL'] = f'postgres://postgres@127.0.0.1:{pg_port}/postgres'
    run([str(pg_bin / 'pg_restore'), '--exit-on-error', '--no-owner', '--dbname', env['DATABASE_URL'],
         str(copied / config['postgres_dump'])])
    inventory_files = {env['WALLET_CUSTODY_INVENTORY_FILE']}
    attestor_processes = []
    for index, service in enumerate(services):
        executable = Path(service['executable']).resolve()
        require(executable.is_file() and os.access(executable, os.X_OK), 'prerequisite: executable service artifact')
        require(hashlib.sha256(executable.read_bytes()).hexdigest() == service['sha256'], 'service artifact digest mismatch')
        require(isinstance(service['source_revision'], str) and len(service['source_revision']) == 40, 'service source revision provenance required')
        service_env = dict(env)
        service_env.update({key: expand(str(value)) for key, value in service.get('env', {}).items()})
        if service['kind'] == 'attestor':
            inventory_files.add(service_env['ATTESTOR_INVENTORY_FILE'])
        process = start([str(executable), *[expand(value) for value in service['argv']]], service_env,
                        f'{service["kind"]}-{index}')
        if service['kind'] == 'attestor':
            attestor_processes.append(process)
    upstream = env['HYPERPAXEER_RPC_URL']
    armed = threading.Event()
    intercepted = threading.Event()
    release = threading.Event()
    gateway = None
    gateway_port = free_port()
    gateway_base = f'http://127.0.0.1:{gateway_port}'
    rpc_counts = {}
    counter_lock = threading.Lock()
    def launch_gateway(overrides=None, name='gateway'):
        process_env = dict(env)
        process_env.update(overrides or {})
        return start([config['node_binary'], str(GATEWAY / 'dist/index.js')], process_env, name)
    def ready():
        deadline = time.monotonic() + 90
        while time.monotonic() < deadline:
            require(gateway.poll() is None, 'gateway exited before readiness; retained log')
            try:
                with urllib.request.urlopen(gateway_base + '/healthz', timeout=2) as response:
                    if response.status == 200:
                        return
            except (OSError, urllib.error.URLError):
                pass
            time.sleep(0.25)
        raise RuntimeError('gateway readiness deadline exceeded')
    class Proxy(http.server.BaseHTTPRequestHandler):
        def log_message(self, *_):
            pass
        def do_POST(self):
            nonlocal gateway
            data = self.rfile.read(int(self.headers.get('content-length', '0')))
            if self.path == '/control/stop-two':
                for process in attestor_processes[:2]:
                    stop(process)
                return self.answer({'stopped': 2})
            if self.path == '/control/stop-quorum':
                for process in attestor_processes[:3]:
                    stop(process)
                return self.answer({'stopped': 3})
            if self.path == '/control/arm':
                armed.set(); intercepted.clear(); release.clear()
                return self.answer({'armed': True})
            if self.path == '/control/restart':
                require(intercepted.is_set(), 'restart must follow the real prebroadcast checkpoint')
                stop(gateway)
                armed.clear(); release.set()
                gateway = launch_gateway(name='gateway-restarted')
                ready()
                return self.answer({'restarted': True})
            request = json.loads(data)
            if request.get('method') == 'eth_sendRawTransaction':
                raw = request['params'][0]
                if armed.is_set():
                    armed.clear(); intercepted.set()
                    release.wait(timeout=60)
                    self.close_connection = True
                    return
                with counter_lock:
                    rpc_counts[raw] = rpc_counts.get(raw, 0) + 1
            try:
                req = urllib.request.Request(upstream, data=data, headers={'Content-Type': 'application/json'})
                with urllib.request.urlopen(req, timeout=60) as response:
                    body = response.read()
                self.send_response(200); self.send_header('Content-Type', 'application/json'); self.end_headers(); self.wfile.write(body)
            except (OSError, urllib.error.URLError):
                self.send_error(502)
        def do_GET(self):
            if self.path == '/control/checkpoint':
                return self.answer({'intercepted': intercepted.is_set()})
            if self.path == '/control/counts':
                return self.answer({'counts': list(rpc_counts.values())})
            self.send_error(404)
        def answer(self, body):
            data = json.dumps(body).encode()
            self.send_response(200); self.send_header('Content-Type', 'application/json'); self.send_header('Content-Length', str(len(data))); self.end_headers(); self.wfile.write(data)
    proxy = http.server.ThreadingHTTPServer(('127.0.0.1', 0), Proxy)
    proxy.daemon_threads = True
    threading.Thread(target=proxy.serve_forever, daemon=True).start()
    proxy_base = f'http://127.0.0.1:{proxy.server_port}'
    env.update(HYPERPAXEER_RPC_URL=proxy_base, RPC_URLS=proxy_base, PORT=str(gateway_port),
               NODE_ENV='test', API_WORKERS='1', LOG_LEVEL='warn')
    gateway = launch_gateway()
    ready()
    mismatch_port = free_port()
    mismatch = launch_gateway({'PORT': str(mismatch_port), 'ATTESTOR_ENDPOINTS': ','.join(endpoints[:4])}, 'gateway-membership-mismatch')
    require(mismatch.wait(timeout=15) == 1, 'four-member configuration must refuse startup')
    require('wallet custody requires exactly five endpoints and threshold three' in (STATE / 'gateway-membership-mismatch.log').read_text(),
            'membership refusal must name the production inventory constraint')
    runtime = dict(config['cases'])
    runtime.update(fixture_dir=str(copied), database_url=env['DATABASE_URL'], gateway_url=gateway_base,
                   mismatch_url=f'http://127.0.0.1:{mismatch_port}', control_url=proxy_base,
                   real_rpc_url=upstream, evidence_dir=str(STATE), membership_mismatch_verified=True,
                   inventory_files=sorted(inventory_files), attestor_endpoints=endpoints,
                   inventory_public_key_file=env['WALLET_CUSTODY_INVENTORY_PUBLIC_KEY_FILE'],
                   attestor_ca_file=env['ATTESTOR_CA_FILE'])
    runtime_path = STATE / 'runtime.json'
    runtime_path.write_text(json.dumps(runtime)); runtime_path.chmod(0o600)
    driver_env = dict(env)
    driver_env['WALLET_CUSTODY_RUNTIME'] = str(runtime_path)
    driver = start([config['node_binary'], '--import', 'tsx', str(GATEWAY / 'test/e2e/wallet-custody-production.ts')], driver_env, 'cases')
    code = driver.wait(timeout=780)
    require(code == 0, f'production custody cases failed with exit {code}; inspect retained cases.log')
    report = json.loads((STATE / 'cases-result.json').read_text())
    require(report.get('complete') is True, 'case driver did not complete')
    require(all(p.poll() is None for p in PROCESSES if p not in (driver, mismatch) and p is gateway), 'gateway died during cases')
    (STATE / 'result.json').write_text(json.dumps({'status': 'passed', 'cases': report['cases'], 'exit_code': 0}, indent=2))
    print(json.dumps({'status': 'passed', 'evidence': str(STATE)}))
    proxy.shutdown()
    run(PG_STOP)
    PG_STOP = None


if __name__ == '__main__':
    def interrupted(*_):
        raise KeyboardInterrupt()
    signal.signal(signal.SIGTERM, interrupted)
    try:
        main()
    except (Exception, KeyboardInterrupt) as error:
        reason = str(error) if isinstance(error, RuntimeError) else type(error).__name__
        result = {'status': 'failed', 'reason': reason, 'evidence': str(STATE) if STATE else None, 'exit_code': 2}
        if STATE:
            (STATE / 'result.json').write_text(json.dumps(result, indent=2))
        print(json.dumps(result))
        raise SystemExit(2)
    finally:
        for process in reversed(PROCESSES):
            stop(process)
        if PG_STOP:
            try:
                run(PG_STOP)
            except Exception:
                pass

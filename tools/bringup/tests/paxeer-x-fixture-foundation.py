#!/usr/bin/env python3
import base64
import copy
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import sys
import time
import uuid

from foundation_provenance import qualify_foundation

ROOT = Path(__file__).resolve().parents[3]
sys.dont_write_bytecode = True
spec = importlib.util.spec_from_file_location('foundation_artifacts', Path(__file__).with_name('foundation-artifacts.py'))
ART = importlib.util.module_from_spec(spec)
spec.loader.exec_module(ART)
SOURCES = ('docker/kernel', 'services/platform/hosted/node',
           'services/wallet/attestor', 'daemon/paxd', 'modules', 'layerxproof',
           'tools/bringup')
NETWORK_ID = 125


class Refused(Exception):
    pass


def require(ok, message):
    if not ok:
        raise Refused(message)


def run(args, *, data=None, timeout=90, check=True):
    result = subprocess.run([str(x) for x in args], input=data, stdout=subprocess.PIPE,
                            stderr=subprocess.PIPE, timeout=timeout)
    if check and result.returncode:
        raise RuntimeError('producer failed: ' + str(args[0]) + ' exit=' + str(result.returncode))
    return result


def private(path, directory=False):
    path = Path(path)
    require(path.is_absolute() and '..' not in path.parts, 'absolute normalized path required')
    require(not any(p.is_symlink() for p in (path, *path.parents)), 'symlink path refused')
    require(not any(p.name.startswith('.env') or '.env.' in p.name or p.name.endswith('.env') for p in (path, *path.parents)), '.env input refused')
    info = path.lstat()
    require(info.st_uid == os.geteuid() and stat.S_IMODE(info.st_mode) == (0o700 if directory else 0o600), 'private owner/mode mismatch')
    require(stat.S_ISDIR(info.st_mode) if directory else stat.S_ISREG(info.st_mode) and info.st_nlink == 1, 'private file type mismatch')
    for parent in path.parents:
        info = parent.stat()
        require(not info.st_mode & 0o022, 'writable parent directory refused')
    return path


def write(path, value):
    raw = value if isinstance(value, bytes) else (json.dumps(value, sort_keys=True, separators=(',', ':')) + '\n').encode()
    descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(descriptor, 'wb') as stream:
        stream.write(raw)
        stream.flush()
        os.fsync(stream.fileno())


def atomic(path, value):
    temporary = path.with_name(path.name + '.' + uuid.uuid4().hex)
    write(temporary, value)
    os.replace(temporary, path)
    descriptor = os.open(path.parent, os.O_RDONLY | os.O_DIRECTORY)
    try:
        os.fsync(descriptor)
    finally:
        os.close(descriptor)


def source_revision():
    require(not ART.git('status', '--porcelain', '--untracked-files=normal'), 'published clean source required')
    return ART.git('rev-parse', 'HEAD')


def prerequisites(revision, environment=None):
    environment = environment or os.environ
    raw = environment.get('PAXEER_X_FOUNDATION_ARTIFACT_MANIFEST')
    require(bool(raw), 'missing PAXEER_X_FOUNDATION_ARTIFACT_MANIFEST; owner task24.13')
    path = private(Path(raw))
    manifest = json.loads(path.read_text())
    ART.validate(manifest, path.parent, revision)
    image = environment.get('PAXEER_X_RUNTIME_IMAGE', '')
    require(re.fullmatch(r'sha256:[0-9a-f]{64}', image) is not None, 'missing source-bound prebuilt PAXEER_X_RUNTIME_IMAGE')
    inspected = json.loads(run(['docker', 'image', 'inspect', image]).stdout)[0]
    require(inspected['Id'] == image, 'runtime image digest mismatch')
    image_revision = (inspected['Config'].get('Labels') or {}).get('org.opencontainers.image.revision', '')
    require(re.fullmatch(r'[0-9a-f]{40}', image_revision) is not None, 'runtime image source identity missing')
    require(ART.source_binding(image_revision, SOURCES) == ART.source_binding(revision, SOURCES), 'runtime image producer source mismatch')
    return path, manifest, image


def jwt(key, audience, claims, producer):
    encode = lambda raw: base64.urlsafe_b64encode(raw).rstrip(b'=')
    header = encode(json.dumps({'alg': 'ES256', 'typ': audience + '+jwt'}, separators=(',', ':')).encode())
    body = encode(json.dumps(claims, separators=(',', ':')).encode())
    message = header + b'.' + body
    signature = producer(['openssl', 'dgst', '-sha256', '-sign', key], data=message)
    require(signature[0] == 0x30 and signature[1] == len(signature) - 2, 'ECDSA signature DER refused')
    position, numbers = 2, []
    for _ in range(2):
        require(signature[position] == 2, 'ECDSA integer tag refused')
        size = signature[position + 1]
        raw = signature[position + 2:position + 2 + size]
        require(size > 0 and not raw[0] & 0x80, 'ECDSA positive integer required')
        number = int.from_bytes(raw, 'big')
        require(0 < number < 2**256, 'ECDSA integer bounds refused')
        numbers.append(number.to_bytes(32, 'big'))
        position += 2 + size
    require(position == len(signature), 'ECDSA trailing bytes refused')
    return message + b'.' + encode(b''.join(numbers))


class Foundation:
    def __init__(self, output, revision, artifacts, image):
        self.output, self.revision, self.artifacts, self.image = output, revision, artifacts, image
        self.name = 'paxeer-x-fixture-' + uuid.uuid4().hex
        self.network = self.name + '-network'
        self.producers = []
        self.created = False
        self.network_created = False
        self.manifest = None
        self.authority_sequence = 0

    def producer(self, args, **options):
        result = run(args, **options)
        self.producers.append({'argv': [str(x) for x in args], 'exit_code': result.returncode,
                               'stdout_sha256': hashlib.sha256(result.stdout).hexdigest(),
                               'stderr_sha256': hashlib.sha256(result.stderr).hexdigest()})
        return result.stdout

    def execute(self, args, data=None, timeout=90):
        return self.producer(['docker', 'exec', '-i', self.name, *args], data=data, timeout=timeout)

    def shell(self, code, timeout=90):
        return self.execute(['bash', '-euo', 'pipefail', '-c', code], timeout=timeout)

    def ca(self, name):
        directory = self.output / 'trust' / name
        directory.mkdir(mode=0o700)
        self.producer(['openssl', 'genpkey', '-algorithm', 'EC', '-pkeyopt', 'ec_paramgen_curve:P-256', '-out', directory / 'key.pem'])
        self.producer(['openssl', 'req', '-x509', '-new', '-key', directory / 'key.pem', '-days', '2', '-sha256',
                       '-subj', '/O=LayerX disposable foundation/CN=' + name,
                       '-addext', 'basicConstraints=critical,CA:TRUE,pathlen:0',
                       '-addext', 'keyUsage=critical,keyCertSign,cRLSign', '-out', directory / 'ca.pem'])
        self.producer(['openssl', 'x509', '-in', directory / 'ca.pem', '-outform', 'DER', '-out', directory / 'ca.der'])
        return directory

    def identity(self, name, ca, usages, ipv6):
        directory = self.output / 'trust' / name
        directory.mkdir(mode=0o700)
        self.producer(['openssl', 'genpkey', '-algorithm', 'EC', '-pkeyopt', 'ec_paramgen_curve:P-256', '-out', directory / 'key.pem'])
        self.producer(['openssl', 'req', '-new', '-key', directory / 'key.pem', '-subj', '/O=LayerX disposable foundation/CN=' + name, '-out', directory / 'csr.pem'])
        write(directory / 'extensions.cnf', ('basicConstraints=CA:FALSE\nkeyUsage=digitalSignature,keyEncipherment\nextendedKeyUsage=' + usages +
              '\nsubjectAltName=DNS:' + name + ',DNS:' + self.name + ',IP:127.0.0.1,IP:::1,IP:' + ipv6 + '\n').encode())
        self.producer(['openssl', 'x509', '-req', '-in', directory / 'csr.pem', '-CA', ca / 'ca.pem', '-CAkey', ca / 'key.pem',
                       '-CAcreateserial', '-days', '2', '-sha256', '-extfile', directory / 'extensions.cnf', '-out', directory / 'cert.pem'])
        self.producer(['openssl', 'x509', '-in', directory / 'cert.pem', '-outform', 'DER', '-out', directory / 'cert.der'])
        self.producer(['openssl', 'pkcs8', '-topk8', '-nocrypt', '-in', directory / 'key.pem', '-outform', 'DER', '-out', directory / 'key.der'])
        write(directory / 'password', os.urandom(24).hex().encode())
        self.producer(['openssl', 'pkcs12', '-export', '-inkey', directory / 'key.pem', '-in', directory / 'cert.pem',
                       '-certfile', ca / 'ca.pem', '-name', name, '-passout', 'file:' + str(directory / 'password'), '-out', directory / 'identity.p12'])
        public = self.producer(['openssl', 'x509', '-in', directory / 'cert.pem', '-pubkey', '-noout'])
        der = self.producer(['openssl', 'pkey', '-pubin', '-outform', 'DER'], data=public)
        return hashlib.sha256(der).hexdigest()

    def rpc(self, method):
        code = "import json,urllib.request; req=urllib.request.Request('http://127.0.0.1:8545',data=json.dumps({'jsonrpc':'2.0','id':1,'method':" + repr(method) + ",'params':[]}).encode(),headers={'Content-Type':'application/json'}); print(urllib.request.urlopen(req,timeout=5).read().decode())"
        return json.loads(self.execute(['python3', '-c', code]))

    def node_start(self):
        self.producer(['docker', 'exec', '-d', self.name, 'bash', '-c',
                       'exec /artifacts/paxd start --home /fixture/chain '
                       '--rpc.laddr tcp://127.0.0.1:26657 --evm.http_enabled true '
                       '--evm.http_address 127.0.0.1 --evm.http_port 8545 '
                       '--sequencer.enable true --sequencer.wal-dir /fixture/chain/data/sequencer-wal '
                       '--sequencer.receipt-key-file /fixture/chain/config/receipt.key '
                       '>> /fixture/chain-process.log 2>&1'])

    def wait_chain(self):
        deadline = time.monotonic() + 90
        while time.monotonic() < deadline:
            try:
                # The inspector verifies the actual canonical Comet signature,
                # native genesis binding and the same-height EVM block. Its
                # retained checkpoint is reopened on every observation.
                raw = self.execute(['/artifacts/bringup-inspect', 'foundation-observe',
                                    '/fixture/run/committed', '/fixture/chain/config/genesis.json',
                                    '/fixture/chain/config/receipt.key',
                                    'http://127.0.0.1:8545', 'http://127.0.0.1:26657'])
                self.committed = json.loads(raw)
                require(self.committed['height'] >= 1, 'actual signed commit required')
                return
            except (RuntimeError, KeyError, ValueError):
                time.sleep(0.5)
        raise RuntimeError('actual integrated Paxeer chain did not commit')

    def attest(self, path='/health', body=None):
        script = """import json,ssl,urllib.request,sys
ctx=ssl.create_default_context(cafile='/fixture/trust/nodes/ca.pem')
ctx.minimum_version=ssl.TLSVersion.TLSv1_3
ctx.load_cert_chain('/fixture/trust/gateway/cert.pem','/fixture/trust/gateway/key.pem')
for node in range(1,6):
 request=urllib.request.Request('https://node-'+str(node)+':'+str(4429+node)+sys.argv[1], data=sys.stdin.buffer.read() if len(sys.argv)>2 else None,headers={'Content-Type':'application/json'})
 response=urllib.request.urlopen(request,context=ctx,timeout=5)
 if response.status!=200: raise RuntimeError('attestor protocol refused')
 print(json.dumps({'node_id':'node-'+str(node),'status':response.status,'response':response.read().decode()}))
"""
        if body is not None:
            script = script.replace('sys.stdin.buffer.read() if len(sys.argv)>2 else None', 'payload')
            script = script.replace('for node in range(1,6):', 'payload=sys.stdin.buffer.read()\nfor node in range(1,6):')
        return self.execute(['python3', '-c', script, path], data=body)

    def wait_attestors(self):
        deadline = time.monotonic() + 90
        while time.monotonic() < deadline:
            try:
                rows = [json.loads(row) for row in self.attest().splitlines()]
                require(len(rows) == 5 and len({row['node_id'] for row in rows}) == 5, 'actual five attestors required')
                return rows
            except RuntimeError:
                time.sleep(0.5)
        raise RuntimeError('actual isolated attestors did not satisfy their health protocol')

    def refresh_authority(self):
        self.authority_sequence += 1
        now = int(time.time())
        claims = {'version': 1, 'iss': 'disposable-foundation', 'aud': 'wallet-custody-authority',
                  'tenant': self.name, 'sequence': str(self.authority_sequence), 'iat': now - 1,
                  'exp': now + 59, 'principals': []}
        token = jwt(self.output / 'authority' / 'key.pem', 'wallet-custody-authority', claims, self.producer)
        atomic(self.output / 'authority' / 'snapshot.jwt', token)
        response = self.attest('/v1/authority', json.dumps({'token': token.decode()}).encode())
        rows = [json.loads(line) for line in response.splitlines()]
        require(len(rows) == 5, 'authority admission requires all five real nodes')
        for row in rows:
            admitted = json.loads(row['response'])
            require(admitted['node_id'] == row['node_id'] and admitted['sequence'] == str(self.authority_sequence), 'actual authority admission identity/sequence mismatch')
        self.manifest['authority']['sequence'] = str(self.authority_sequence)
        self.manifest['authority']['snapshot'] = 'authority/snapshot.jwt'
        self.manifest['authority']['expires_at'] = claims['exp']

    def launch_attestors(self):
        nodes = self.manifest['attestors']
        for node in nodes:
            identity = node['id']
            values = {'ATTESTOR_NODE_ID': identity, 'ATTESTOR_REGION': 'disposable-foundation',
                      'ATTESTOR_LISTEN_ADDR': '[::]:' + str(node['port']), 'ATTESTOR_PEER_LISTEN_ADDR': '[::]:' + str(node['peer_port']),
                      'ATTESTOR_PEERS': ','.join(n['id'] + '=' + n['id'] + ':' + str(n['peer_port']) for n in nodes if n != node),
                      'ATTESTOR_PEER_PINS': ','.join(n['id'] + '=' + n['spki_sha256'] for n in nodes if n != node),
                      'ATTESTOR_NODE_KEY_FILE': '/fixture/attestors/' + identity + '/node.key',
                      'ATTESTOR_DATA_DIR': '/fixture/attestors/' + identity + '/data',
                      'ATTESTOR_CEREMONY': 'true', 'ATTESTOR_CHAIN_ID': '125',
                      'ATTESTOR_TLS_CERT_FILE': '/fixture/trust/' + identity + '/cert.pem',
                      'ATTESTOR_TLS_KEY_FILE': '/fixture/trust/' + identity + '/key.pem',
                      'ATTESTOR_TLS_CA_FILE': '/fixture/trust/peer-gateway-ca.pem',
                      'ATTESTOR_OPERATOR_CA_FILE': '/fixture/trust/operators/ca.pem',
                      'ATTESTOR_AUTHORITY_PUBLIC_KEY_FILE': '/fixture/authority/public.pem',
                      'ATTESTOR_AUTHORITY_ISSUER': 'disposable-foundation', 'ATTESTOR_AUTHORITY_TENANT': self.name,
                      'ATTESTOR_INVENTORY_PUBLIC_KEY_FILE': '/fixture/authority/public.pem',
                      'ATTESTOR_INVENTORY_FILE': '/fixture/authority/inventory.jwt',
                      'ATTESTOR_RPC_URL': 'http://127.0.0.1:8545'}
            arguments = ['docker', 'exec', '-d']
            for name, value in values.items():
                arguments += ['-e', name + '=' + value]
            arguments += [self.name, 'bash', '-c', 'exec /artifacts/attestor > /fixture/attestors/' + identity + '/process.log 2>&1']
            self.producer(arguments)

    def start(self):
        self.output.mkdir(mode=0o700)
        for child in ('chain', 'keys', 'genesis', 'node', 'run', 'trust', 'authority', 'attestors'):
            (self.output / child).mkdir(mode=0o700)
        subnet = 'fd' + os.urandom(5).hex()[:2] + ':' + os.urandom(2).hex() + ':' + os.urandom(2).hex() + '::/64'
        self.producer(['docker', 'network', 'create', '--internal', '--ipv6', '--subnet', subnet, self.network])
        self.network_created = True
        aliases = []
        for node in range(1, 6):
            aliases += ['--network-alias', 'node-' + str(node)]
        self.producer(['docker', 'run', '-d', '--name', self.name, '--network', self.network, *aliases,
                       '--read-only', '--tmpfs', '/tmp:rw,nosuid,nodev,mode=1777', '--cap-drop', 'ALL',
                       '--security-opt', 'no-new-privileges', '--mount', 'type=bind,src=' + str(ROOT) + ',dst=/source,readonly',
                       '--mount', 'type=bind,src=' + str(self.artifacts.parent) + ',dst=/artifacts,readonly',
                       '--mount', 'type=bind,src=' + str(self.output) + ',dst=/fixture',
                       '-e', 'LD_LIBRARY_PATH=/artifacts', '--entrypoint', 'sleep', self.image, 'infinity'])
        self.created = True
        inspected = json.loads(self.producer(['docker', 'inspect', self.name]))[0]
        ipv6 = inspected['NetworkSettings']['Networks'][self.network]['GlobalIPv6Address']
        require(bool(ipv6), 'actual isolated IPv6 address missing')
        native_raw = os.environ.get('PAXEER_X_NATIVE_INPUTS')
        require(bool(native_raw), 'private disposable paxd initialization inputs required; owner domain 5')
        native = private(Path(native_raw), directory=True)
        # paxd owns initialization, all application validation and the only
        # integrated kernel process. Import only explicit native inputs.
        self.execute(['/artifacts/paxd', 'init', 'disposable-foundation', '--chain-id',
                      'hyperpax_125-1', '--mode', 'validator', '--home', '/fixture/chain'])
        for name in ('genesis.json', 'config.toml', 'app.toml', 'priv_validator_key.json', 'receipt.key'):
            origin = private(native / name)
            self.producer(['install', '-m', '0600', origin, self.output / 'chain' / 'config' / name])
        report = self.execute(['/artifacts/bringup-inspect', 'native-genesis-check',
                               '/fixture/chain/config/genesis.json', '/fixture/chain/config/receipt.key'])
        initialization = json.loads(report)
        require(initialization['chain_id'] == 'hyperpax_125-1', 'real genesis source-network mismatch')
        require(set(initialization['modules']) == {'bank', 'sequencer', 'lxphost'}, 'merged initialization modules differ')
        write(self.output / 'genesis' / 'initialization.json', initialization)
        self.execute(['/artifacts/paxd', 'validate-genesis', '--home', '/fixture/chain'])
        public = initialization['receipt_public_key']
        seq_id = hashlib.sha256(bytes.fromhex(public)).hexdigest()
        self.node_start()
        self.wait_chain()
        node_ca, gateway_ca, operator_ca = self.ca('nodes'), self.ca('gateways'), self.ca('operators')
        self.identity('gateway', gateway_ca, 'clientAuth', ipv6)
        self.identity('operator', operator_ca, 'clientAuth', ipv6)
        write(self.output / 'trust' / 'peer-gateway-ca.pem', (node_ca / 'ca.pem').read_bytes() + (gateway_ca / 'ca.pem').read_bytes())
        members = []
        for number in range(1, 6):
            identity = 'node-' + str(number)
            pin = self.identity(identity, node_ca, 'serverAuth,clientAuth', ipv6)
            directory = self.output / 'attestors' / identity
            directory.mkdir(mode=0o700)
            (directory / 'data').mkdir(mode=0o700)
            write(directory / 'node.key', os.urandom(32).hex().encode())
            members.append({'id': identity, 'spki_sha256': pin, 'port': 4429 + number, 'peer_port': 4529 + number,
                            'url': 'https://' + identity + ':' + str(4429 + number), 'ipv6_url': 'https://[' + ipv6 + ']:' + str(4429 + number)})
        key = self.output / 'authority' / 'key.pem'
        self.producer(['openssl', 'genpkey', '-algorithm', 'EC', '-pkeyopt', 'ec_paramgen_curve:P-256', '-out', key])
        self.producer(['openssl', 'pkey', '-in', key, '-pubout', '-out', self.output / 'authority' / 'public.pem'])
        now = int(time.time())
        claims = {'version': 1, 'iss': 'disposable-foundation', 'tenant': self.name, 'sequence': '1', 'iat': now - 1,
                  'exp': now + 86400, 'aud': 'wallet-custody-inventory', 'protocol': 'wallet', 'threshold': 3,
                  'members': [{k: m[k] for k in ('id', 'spki_sha256')} for m in members], 'keys': []}
        write(self.output / 'authority' / 'inventory.jwt', jwt(key, 'wallet-custody-inventory', claims, self.producer))
        environment = {'LAYERX_NODE_NETWORK_ID': str(NETWORK_ID),
                       'LAYERX_NODE_CHAIN_ID': 'hyperpax_125-1', 'LAYERX_NODE_EVM_CHAIN_ID': '125',
                       'LAYERX_NODE_SEQUENCER_ID': seq_id, 'LAYERX_NODE_SEQUENCER_PUBLIC_KEY': public,
                       'LAYERX_NODE_RPC_URL': 'http://127.0.0.1:8545',
                       'LAYERX_NODE_CONSENSUS_RPC_URL': 'http://127.0.0.1:26657'}
        require(all(re.fullmatch(r'LAYERX_[A-Z0-9_]+', name) and isinstance(value, str) and 0 < len(value) <= 4096
                    and not any(ord(character) < 32 for character in value) for name, value in environment.items()), 'bounded fixture environment refused')
        write(self.output / 'genesis' / 'environment.json', environment)
        endpoint_policy = {'version': 1, 'network': self.network, 'internal': True, 'host_ports': [],
                           'chain': {'evm_chain_id': 125, 'rpc': 'http://127.0.0.1:8545', 'comet': 'http://127.0.0.1:26657',
                                     'scope': 'shared isolated container network namespace'},
                           'attestors': [{'id': member['id'], 'url': member['url'], 'ipv6_url': member['ipv6_url'],
                                         'spki_sha256': member['spki_sha256']} for member in members],
                           'tls_minimum': '1.3', 'downstream_human_started': False, 'downstream_registry_started': False}
        write(self.output / 'genesis' / 'endpoint-policy.json', endpoint_policy)
        self.manifest = {'version': 2, 'stage': 'dependency-foundation', 'purpose': 'disposable-test-only',
                         'source_revision': self.revision, 'source_binding': ART.source_binding(self.revision, SOURCES),
                         'artifact_manifest': str(self.artifacts), 'image': self.image, 'network': self.network, 'container': self.name,
                         'network_id': NETWORK_ID, 'chain_id': 125, 'cosmos_chain_id': 'hyperpax_125-1', 'ipv6': ipv6,
                         'chain_rpc': 'http://127.0.0.1:8545', 'comet_rpc': 'http://127.0.0.1:26657',
                         'endpoint_scope': 'isolated container network namespace; no published host ports',
                         'committed': self.committed, 'sequencer_id': seq_id,
                         'sequencer_public_key': public,
                         'attestors': members, 'authority': {'issuer': 'disposable-foundation', 'tenant': self.name,
                         'public_key': 'authority/public.pem', 'signing_key': 'authority/key.pem', 'inventory': 'authority/inventory.jwt'},
                         'inputs': {'initialization': 'genesis/initialization.json',
                         'environment': 'genesis/environment.json', 'endpoint_policy': 'genesis/endpoint-policy.json',
                         'native_genesis': 'chain/config/genesis.json',
                         'receipt_key': 'chain/config/receipt.key', 'committed': 'run/committed/committed.json'},
                         'tls': {'node_ca': 'trust/nodes/ca.pem', 'gateway_ca': 'trust/gateways/ca.pem', 'operator_ca': 'trust/operators/ca.pem',
                         'gateway_identity': 'trust/gateway', 'operator_identity': 'trust/operator'},
                         'human_readiness': False, 'registry_readiness': False, 'producers': self.producers}
        self.launch_attestors()
        rows = self.wait_attestors()
        self.manifest['attestor_health'] = rows
        self.refresh_authority()
        self.manifest['authority']['approved_keys'] = 0
        self.manifest['authority']['approved_principals'] = 0
        self.manifest['generated_artifacts'] = {}
        for name in ('chain/config/genesis.json', *self.manifest['inputs'].values(), 'authority/public.pem', 'authority/inventory.jwt',
                     'trust/nodes/ca.der', 'trust/gateways/ca.der', 'trust/operators/ca.der'):
            self.manifest['generated_artifacts'][name] = {'sha256': ART.sha256(self.output / name), 'size': (self.output / name).stat().st_size}
        self.manifest['artifact_manifest_sha256'] = ART.sha256(self.artifacts)
        self.manifest['source_paths'] = list(SOURCES)
        qualify_foundation(self.manifest, self.output, self.revision, 'foundation')
        atomic(self.output / 'manifest.json', self.manifest)

    def validate_retained(self, manifest):
        qualify_foundation(manifest, self.output, self.revision, 'foundation')
        require(manifest.get('version') == 2 and manifest['purpose'] == 'disposable-test-only' and manifest['stage'] == 'dependency-foundation', 'wrong foundation stage')
        require(manifest['source_binding'] == ART.source_binding(self.revision, SOURCES), 'retained source mismatch')
        require(manifest['chain_id'] == 125 and manifest['network_id'] == NETWORK_ID, 'wrong source-network identity')
        network = json.loads(run(['docker', 'network', 'inspect', self.network]).stdout)[0]
        require(network['Internal'] is True and network['EnableIPv6'] is True, 'dependency network is not internal IPv6')
        container = json.loads(run(['docker', 'inspect', self.name]).stdout)[0]
        require(container['Image'] == self.image and not container['HostConfig']['PortBindings'], 'wrong runtime image or external host ports')
        require(set(container['NetworkSettings']['Networks']) == {self.network}, 'extra dependency network refused')
        for name, record in manifest.get('generated_artifacts', {}).items():
            require(not any(part.startswith('.env') or '.env.' in part or part.endswith('.env') for part in Path(name).parts), '.env artifact refused')
            require(not Path(name).is_absolute() and '..' not in Path(name).parts, 'generated artifact path refused')
            require(ART.sha256(self.output / name) == record['sha256'] and (self.output / name).stat().st_size == record['size'], 'retained generated artifact mismatch')
        self.wait_chain()
        self.wait_attestors()

    def restart(self):
        protected = ['chain/config/genesis.json', 'chain/config/priv_validator_key.json',
                     'chain/config/node_key.json', 'chain/config/receipt.key',
                     'genesis/initialization.json', 'authority/key.pem', 'authority/inventory.jwt']
        previous_height = self.committed['height']
        before = {name: ART.sha256(self.output / name) for name in protected}
        self.producer(['docker', 'restart', self.name])
        self.validate_retained(self.manifest)
        self.node_start()
        self.wait_chain()
        self.launch_attestors()
        self.refresh_authority()
        require(before == {name: ART.sha256(self.output / name) for name in protected}, 'restart changed durable foundation material')
        require(self.committed['height'] >= previous_height, 'restart rolled back committed state')
        self.manifest['committed'] = self.committed
        checkpoint = self.output / 'run/committed/committed.json'
        self.manifest['generated_artifacts']['run/committed/committed.json'] = {'sha256': ART.sha256(checkpoint), 'size': checkpoint.stat().st_size}
        self.manifest['restart_preserved'] = before
        self.manifest['producers'] = self.producers
        qualify_foundation(self.manifest, self.output, self.revision, 'foundation')
        atomic(self.output / 'manifest.json', self.manifest)

    def cleanup(self):
        if self.created:
            run(['docker', 'rm', '-f', self.name], check=False)
        if self.network_created:
            run(['docker', 'network', 'rm', self.network], check=False)


def main():
    os.umask(0o077)
    foundation, output, evidence = None, None, {'task': '24.11', 'tests': 0, 'skipped': 0, 'cases': [], 'exit_code': 1}
    try:
        revision = source_revision()
        raw = os.environ.get('PAXEER_X_FOUNDATION_OUTPUT')
        require(bool(raw), 'missing PAXEER_X_FOUNDATION_OUTPUT')
        output = Path(raw)
        private(output.parent, directory=True)
        require(output.is_absolute() and '..' not in output.parts and not output.exists() and not output.is_symlink(), 'fresh private foundation output required')
        require(ROOT not in output.parents and output != ROOT, 'repository output refused')
        path, artifacts, image = prerequisites(revision)
        foundation = Foundation(output, revision, path, image)
        foundation.start()
        evidence['cases'].append('actual-chain-genesis-native-producers-and-five-attestors')
        wrong_artifacts = copy.deepcopy(artifacts)
        wrong_artifacts['artifacts']['paxd']['sha256'] = '0' * 64
        for name, action in (
            ('missing-prebuilt-artifact-refused', lambda: prerequisites(revision, {'PAXEER_X_RUNTIME_IMAGE': image})),
            ('mismatched-prebuilt-artifact-refused', lambda: ART.validate(wrong_artifacts, path.parent, revision)),
            ('wrong-source-network-refused', lambda: foundation.validate_retained(dict(foundation.manifest, chain_id=126))),
            ('wrong-source-revision-refused', lambda: foundation.validate_retained(dict(foundation.manifest, source_binding='0' * 64))),
        ):
            try:
                action()
            except (Refused, ART.Refused):
                evidence['cases'].append(name)
            else:
                raise RuntimeError(name + ' unexpectedly accepted')
        failure = run(['docker', 'exec', foundation.name, '/artifacts/paxd', 'validate-genesis', '--home', '/fixture/absent-output'], check=False)
        require(failure.returncode != 0, 'failed actual producer unexpectedly accepted')
        require(not (output / 'absent-output').exists(), 'failed real producer published output')
        evidence['cases'].append('failed-real-native-producer-refused')
        foreign = run(['docker', 'exec', '--user', '4021:4021', foundation.name, 'test', '-r', '/fixture/chain/config/receipt.key'], check=False)
        require(foreign.returncode != 0, 'foreign runtime UID can read fixture signing key')
        evidence['cases'].append('foreign-runtime-uid-refused')
        foundation.restart()
        evidence['cases'].append('retained-foundation-restart')
        evidence.update(source_revision=revision, manifest=str(output / 'manifest.json'), tests=len(evidence['cases']), exit_code=0)
        atomic(output / 'qualification.json', evidence)
        print('PAXEER_X_FOUNDATION_MANIFEST=' + str(output / 'manifest.json'), flush=True)
        print('PAXEER_X_GATE tests=' + str(evidence['tests']) + ' skipped=0', flush=True)
        return 0
    except (Refused, ART.Refused, FileNotFoundError) as error:
        code = 78
        evidence['observed'] = str(error)
    except Exception as error:
        code = 1
        evidence['observed'] = str(error)
    evidence['exit_code'] = code
    if foundation is not None:
        foundation.cleanup()
    if output is not None and output.is_dir():
        atomic(output / 'qualification.json', evidence)
    print('PAXEER_X_FOUNDATION_REFUSED ' + evidence['observed'], file=sys.stderr, flush=True)
    return code


if __name__ == '__main__':
    raise SystemExit(main())

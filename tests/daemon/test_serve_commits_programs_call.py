import hashlib
import importlib.util
import os
from pathlib import Path
import shutil
import socket
import struct
import subprocess
import tempfile
import time

from cryptography.hazmat.primitives.asymmetric import ec, ed25519
from cryptography.hazmat.primitives.serialization import Encoding, PublicFormat


ROOT = Path(__file__).resolve().parents[2]


def pack(fmt, *values):
    return struct.pack('>' + fmt, *values)


def blob(value):
    return pack('I', len(value)) + value


def digest(domain, value):
    return hashlib.sha256(b'LXP/v1/' + domain + b'\0' + value).digest()


def signed_call(key):
    public = key.public_key().public_bytes(Encoding.Raw, PublicFormat.Raw)
    did = b'did:layerx:' + public.hex().encode()
    entry = b'layerx_call'
    access = b'LayerX/programs/access-declaration/v1\0\0'
    payload = (os.urandom(32) + pack('HHIHII', 1, len(entry), 0, 2, len(access), 16)
               + pack('7Q', 1_000_000, 16_777_216, 1_048_576, 1_048_576,
                      64, 1_048_576, 4096) + entry + b'\0\0' + access)
    now = time.time_ns() // 1_000_000
    fields = [pack('H', 3), pack('I', 77), pack('I', 0x00090003), blob(did),
              blob(public), pack('Q', 1), pack('QQ', now - 30_000, now + 120_000),
              blob(os.urandom(32)), bytes(16), blob(digest(b'payload-hash', payload)),
              blob(payload)]
    body = b''.join(bytes([i]) + field for i, field in enumerate(fields, 1))
    unsigned = pack('HHB', 3, 0x1001, 11) + body
    signature = key.sign(digest(b'signature-preimage', unsigned))
    return did, public, pack('HHB', 3, 0x1001, 12) + body + b'\x0c' + blob(signature)


def receive(stream, size):
    result = bytearray()
    while len(result) < size:
        chunk = stream.recv(size - len(result))
        assert chunk, 'LNI connection closed'
        result.extend(chunk)
    return bytes(result)


def exchange(stream, tag, correlation, payload=b''):
    message = pack('HHHQ', 1, 4, tag, correlation) + blob(payload) + blob(b'')
    stream.sendall(blob(message))
    size, = struct.unpack('>I', receive(stream, 4))
    assert 22 <= size <= 1_212_416
    response = receive(stream, size)
    major, minor, response_tag, response_id, length = struct.unpack_from('>HHHQI', response)
    assert major == 1 and response_id == correlation
    proof_size, = struct.unpack_from('>I', response, 18 + length)
    assert len(response) == 22 + length + proof_size
    return response_tag, response[18:18 + length], response[22 + length:]


def build_genesis(directory):
    spec = importlib.util.spec_from_file_location('lxgb_metadata', ROOT / 'tests/support/lxgb_metadata.py')
    metadata = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(metadata)
    asset = os.urandom(32)
    guarantor = ec.generate_private_key(ec.SECP256K1()).public_key().public_bytes(
        Encoding.X962, PublicFormat.CompressedPoint)
    now = time.time_ns() // 1_000_000
    request = (b'LXGB\x02' + pack('HIQHH', 3, 77, now, 1, 7)
               + b'parameter-version'.ljust(32, b'\0') + bytes(31) + b'\x01'
               + pack('H', 1) + b'\x01' + bytes(31) + guarantor + bytes(16) + asset
               + pack('I', 1) + pack('9Q', 1, 1, 1, 1, 1, 8, 8, 64, 8)
               + pack('QBI', 1, 1, 1) + pack('7Q', 1, 1, 2, 4, 1, 1, 100)
               + pack('6Q', 100, 1, 1, 10, 1, 1000)
               + metadata.metadata(asset, os.urandom(32), os.urandom(32)))
    request_path = directory / 'request.lxgb'
    request_path.write_bytes(request)
    signer = directory / 'signer.key'
    signer.write_bytes(os.urandom(32))
    signer.chmod(0o600)
    # Link the production genesis producer against the libraries from hf-build.
    builder = directory / 'layerx-genesis-build'
    result = subprocess.run(
        ['cc', '-std=c17', '-Iinclude', '-Ibuild/generated',
         'cmd/layerx-genesis/main.c', 'cmd/layerx-genesis/lxp_genesis_build_cli.c',
         'cmd/layerx-genesis/lxp_genesis_builder.c', 'build/liblayerx.a',
         'programs/target/release/liblayerx_programs_sandbox.a', 'build/liblayerx.a',
         '-lssl', '-lcrypto', '-pthread', '-ldl', '-lm', '-o', str(builder)],
        cwd=ROOT, capture_output=True, text=True, timeout=60)
    assert result.returncode == 0, result.stdout + result.stderr
    artifacts = directory / 'genesis'
    result = subprocess.run([str(builder), str(request_path), str(signer), str(artifacts)],
                            capture_output=True, text=True, timeout=30)
    assert result.returncode == 0, result.stdout + result.stderr
    registration = (artifacts / 'paxeer-registration-request.lxrr').read_bytes()
    assert len(registration) == 73 and registration[:5] == b'LXRR\x01'
    return artifacts, registration[41:]


def test_serve_commits_an_admitted_programs_call():
    assert os.geteuid() == 0, 'distinct daemon/client LNI credentials require root'
    binary = ROOT / 'build/bin/layerxd'
    assert binary.is_file(), 'hf-build must build layerxd before verification'
    directory = Path(tempfile.mkdtemp(prefix='layerx-serve-programs-'))
    directory.chmod(0o755)
    processes = []
    reservations = []
    stream = None
    try:
        artifacts, state_root = build_genesis(directory)
        executable = directory / 'layerxd'
        shutil.copyfile(binary, executable)
        executable.chmod(0o755)
        migrations = directory / 'migrations'
        shutil.copytree(ROOT / 'migrations', migrations)
        node = directory / 'node'
        replica = directory / 'replica'
        run = directory / 'run'
        for path in (node, replica, run, node / 'logs', node / 'checkpoints'):
            path.mkdir(mode=0o700)
        run.chmod(0o750)
        key = ed25519.Ed25519PrivateKey.generate()
        did, public, signed = signed_call(key)
        (node / 'identities.txt').write_text(f'{did.hex()}:{public.hex()}:1\n')
        (node / 'registration.lxgr').write_bytes(
            b'LXGR\x01' + pack('IQ', 77, 0) + state_root * 2 + b'\x01')
        for path, role in ((node, 'sequencer'), (replica, 'replica')):
            (path / 'config.txt').write_text(
                f'config_version=2\nrole={role}\nnetwork_id=77\nstart_sequence=0\n'
                'verify_workers=0\nserial_execution=true\n')
        for tree in (node, replica, run, artifacts):
            for path in (tree, *tree.rglob('*')):
                os.chown(path, 65534, 0)
                if path.is_file():
                    path.chmod(0o600)
        for _ in range(2):
            reservation = socket.socket()
            reservation.bind(('127.0.0.1', 0))
            reservations.append(reservation)
        replica_port, program_port = [s.getsockname()[1] for s in reservations]
        seed = os.urandom(32)
        sequencer = ed25519.Ed25519PrivateKey.from_private_bytes(seed).public_key().public_bytes(
            Encoding.Raw, PublicFormat.Raw).hex()
        replica_id, replica_token = os.urandom(32).hex(), os.urandom(32).hex()

        def start(mode, config, environment, stderr):
            with (directory / stderr).open('wb') as log:
                process = subprocess.Popen(
                    [str(executable), mode, str(config)], env={'PATH': '/usr/bin:/bin', **environment},
                    user=65534, group=0, stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=log)
            processes.append(process)
            return process

        def diagnostics():
            return f'evidence: {directory}\n' + '\n'.join(f'{name}:\n{(directory / name).read_text()}'
                             for name in ('replica.stderr', 'sequencer.stderr')
                             if (directory / name).exists())

        replica_env = {
            'LAYERX_AUTHORITY_REPLICA_LOG': str(replica / 'replica.log'),
            'LAYERX_AUTHORITY_REPLICA_ID': replica_id,
            'LAYERX_AUTHORITY_SEQUENCER_ID': sequencer,
            'LAYERX_AUTHORITY_SEQUENCER_PUBLIC_KEY': sequencer,
            'LAYERX_AUTHORITY_FIRST_BATCH': '1', 'LAYERX_AUTHORITY_LAST_BATCH': '2',
            'LAYERX_AUTHORITY_BEARER_TOKEN': replica_token,
            'LAYERX_AUTHORITY_ADDRESS': '127.0.0.1', 'LAYERX_AUTHORITY_PORT': str(replica_port),
        }
        reservations[0].close()
        authority = start('--authority-replica', replica / 'config.txt', replica_env, 'replica.stderr')
        deadline = time.monotonic() + 10
        while True:
            assert authority.poll() is None, diagnostics()
            try:
                with socket.create_connection(('127.0.0.1', replica_port), timeout=0.1):
                    break
            except OSError:
                assert time.monotonic() < deadline, diagnostics()
                time.sleep(0.05)
        paths = {
            'SNAPSHOT': artifacts / '00000000000000000000.lxs',
            'GENESIS_MANIFEST': artifacts / 'genesis.manifest',
            'GENESIS_REGISTRATION': node / 'registration.lxgr',
            'IDENTITIES': node / 'identities.txt', 'CHECKPOINT_DIRECTORY': node / 'checkpoints',
            'PROGRAM_FEED_LOG': node / 'logs/feed.log', 'CANONICAL_LOG': node / 'logs/canonical.log',
            'RECEIPT_AUTHORITY_LOG': node / 'logs/receipt-authority.log',
            'BATCH_LOG': node / 'logs/batch.log', 'EVIDENCE_LOG': node / 'logs/evidence.log',
            'HISTORY_DATABASE': node / 'history.db',
            'HISTORY_MIGRATIONS': migrations / '0007_history_index.sql',
            'LNI_SOCKET': run / 'layerxd.sock',
        }
        env = {'LAYERX_NODE_' + name: str(value) for name, value in paths.items()}
        env.update({'LAYERX_NODE_' + name: value for name, value in {
            'SEQUENCER_ID': sequencer, 'SEQUENCER_PUBLIC_KEY': sequencer,
            'SEQUENCER_PRIVATE_KEY': seed.hex(), 'FIRST_BATCH': '1', 'LAST_BATCH': '2',
            'AUTHORITY_REPLICA_ADDRESS': '127.0.0.1', 'AUTHORITY_REPLICA_PORT': str(replica_port),
            'AUTHORITY_REPLICA_ID': replica_id, 'AUTHORITY_REPLICA_BEARER_TOKEN': replica_token,
            'PROGRAM_BEARER_TOKEN': os.urandom(32).hex(), 'PROGRAM_ADDRESS': '127.0.0.1',
            'PROGRAM_PORT': str(program_port), 'LNI_ALLOWED_UID': '0', 'LNI_ALLOWED_GID': '0',
            'LNI_FRAME_BYTES': '1212416', 'LNI_DEADLINE_MS': '10000',
        }.items()})
        reservations[1].close()
        daemon = start('--serve', node / 'config.txt', env, 'sequencer.stderr')
        deadline = time.monotonic() + 10
        while True:
            assert daemon.poll() is None, diagnostics()
            stream = socket.socket(socket.AF_UNIX)
            stream.settimeout(1)
            try:
                stream.connect(str(paths['LNI_SOCKET']))
                break
            except OSError:
                stream.close()
                assert time.monotonic() < deadline, diagnostics()
                time.sleep(0.05)
        tag, info, proof = exchange(stream, 1, 0)
        assert tag == 2 and not proof and info[:10] == pack('HHHI', 1, 8, 3, 77), diagnostics()
        activity_id = digest(b'activity-id', signed)
        started = time.monotonic()
        tag, retained, proof = exchange(stream, 3, 1, signed)
        assert (tag, retained, proof) == (4, signed, activity_id), diagnostics()
        deadline = started + 10
        attempt = 0
        while time.monotonic() < deadline:
            stream.settimeout(max(0.001, deadline - time.monotonic()))
            tag, receipt, proof = exchange(stream, 5, 1000 + attempt, b'\x01' + activity_id)
            assert tag == 6 and not proof, diagnostics()
            assert 'execution failed' not in (directory / 'sequencer.stderr').read_text(), diagnostics()
            if receipt:
                assert time.monotonic() - started < 10, diagnostics()
                assert daemon.poll() is None and authority.poll() is None, diagnostics()
                break
            attempt += 1
            time.sleep(0.05)
        else:
            raise AssertionError('receipt did not commit within 10 s\n' + diagnostics())
        assert receipt, diagnostics()
        assert 'execution failed' not in (directory / 'sequencer.stderr').read_text(), diagnostics()
    finally:
        if stream is not None:
            stream.close()
        for reservation in reservations:
            reservation.close()
        for process in reversed(processes):
            if process.poll() is None:
                process.terminate()
            try:
                process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait()
        # Keep private daemon stderr and genesis inputs for qualification evidence.
        directory.chmod(0o700)

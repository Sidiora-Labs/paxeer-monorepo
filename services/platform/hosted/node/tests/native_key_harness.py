"""Shared native bootstrap inputs; recorded peers are selected explicitly by the gate."""
import contextlib
import hashlib
import json
import os
from pathlib import Path
import shutil
import socket
import ssl
import sys
import threading

NODE = Path(__file__).resolve().parents[1]
ROOT = NODE.parents[3]
sys.path.insert(0, str(NODE))
sys.path.insert(0, str(NODE / 'signer'))

import generation_client as generation
import generation_transport as transport
import native_runtime as native
import reset_state as reset
import signer as signer_service
from client import SignerClient
from provider import FileProvider

FIXTURES = Path(__file__).with_name('fixtures') / 'repair_22_31_355'


def write_seed(path, seed):
    descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(descriptor, 'wb') as handle:
        handle.write(seed)


def public_key_of(seed):
    from cryptography.hazmat.primitives import serialization
    from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey

    return Ed25519PrivateKey.from_private_bytes(seed).public_key().public_bytes(
        serialization.Encoding.Raw, serialization.PublicFormat.Raw)


class NativeKeyHarness:
    def __init__(self, root, recorded=False):
        self.root = Path(root)
        self.resources = contextlib.ExitStack()
        self.peer = None
        config = json.loads((FIXTURES / 'native_configuration.json').read_text())
        self.config = config
        self.data, self.state, self.run = (self.root / name for name in ('data', 'state', 'run'))
        for directory in (self.data, self.state, self.run):
            directory.mkdir(mode=0o700)
        inputs = self.root / 'native-inputs'
        inputs.mkdir(mode=0o700)
        for name in ('genesis.json', 'config.toml', 'app.toml'):
            native.write_file(inputs / name, (FIXTURES / name).read_bytes())
        native.write_file(inputs / 'priv_validator_key.json', reset.canonical(config['validator_key']))
        certificate = ssl.PEM_cert_to_DER_cert((FIXTURES / 'tls-certificate.pem.txt').read_text())
        native.write_file(inputs / 'rpc-ca.der', certificate)
        self.receipt_seed = bytes.fromhex(config['receipt_seed'])
        self.receipt_key = self.root / 'receipt.key'
        write_seed(self.receipt_key, self.receipt_seed.hex().encode() + b'\n')
        self.treasury_seed = os.urandom(32)
        self.treasury_key = self.root / 'treasury.key'
        write_seed(self.treasury_key, self.treasury_seed)
        native.write_file(self.root / 'funding.key', config['funding_scalar'].encode())
        config.update(native_inputs=str(inputs), receipt_key_file=str(self.receipt_key),
                      funding_sender_key_file=str(self.root / 'funding.key'),
                      gateway_endpoint='https://localhost:9443',
                      chain_rpc_url='http://127.0.0.1:8545',
                      consensus_rpc_url='http://127.0.0.1:26657')
        if recorded:
            from test_repair_22_31_355 import RecordedNativePeer

            self.peer = RecordedNativePeer(config, self.root)
            self.resources.callback(self.peer.close)
            config.update(paxd='paxd', peer=self.peer,
                          chain_rpc_url='http://127.0.0.1:' + str(self.peer.chain.server_port),
                          consensus_rpc_url='http://127.0.0.1:' + str(self.peer.consensus.server_port),
                          gateway_endpoint='https://localhost:' + str(self.peer.gateway.server_port))
        else:
            binary = ROOT / 'build/bin/paxd'
            executable = str(binary) if binary.is_file() else shutil.which('paxd')
            if executable is None or not os.access(executable, os.X_OK):
                raise AssertionError('native bootstrap requires an executable paxd in build/bin or PATH')
            config.update(paxd=executable, peer=None)

    def close(self):
        self.resources.close()

    def arguments(self, action='bootstrap', receipt_key=None, treasury_socket=None):
        values = dict(self.config)
        values['receipt_key_file'] = str(receipt_key if receipt_key is not None else self.receipt_key)
        values['treasury_signer_socket'] = str(treasury_socket if treasury_socket is not None
                                              else self.run / 'treasury.sock')
        command = [action, '--data-dir', str(self.data), '--state-dir', str(self.state),
                   '--run-dir', str(self.run), '--consumer-uid', str(os.geteuid()),
                   '--consumer-gid', str(os.getegid())]
        for name in ('paxd', 'native_inputs', 'receipt_key_file', 'funding_sender_key_file',
                     'network_id', 'chain_id', 'evm_chain_id', 'asset', 'gateway_endpoint',
                     'chain_rpc_url', 'consensus_rpc_url', 'treasury_signer_socket'):
            command.extend(['--' + name.replace('_', '-'), str(values[name])])
        return command

    def bootstrap(self, receipt_key=None, treasury_socket=None):
        args = native.parser().parse_args(self.arguments(receipt_key=receipt_key,
                                                         treasury_socket=treasury_socket))
        return native.bootstrap(args, commands=self.peer)

    def signer(self, name='treasury.sock', key=None, policy=None):
        return self.resources.enter_context(self.serving_signer(name, key, policy))

    @contextlib.contextmanager
    def serving_signer(self, name='treasury.sock', key=None, policy=None):
        path = self.run / name
        backend = FileProvider(key if key is not None else self.treasury_key)
        signer = signer_service.Signer(backend, [os.geteuid()], policy)
        try:
            server = signer_service.Server(str(path), signer_service.Handler)
        except BaseException:
            signer.close()
            raise
        server.signer = signer
        server.request_timeout = 5
        path.chmod(0o600)
        worker = threading.Thread(target=server.serve_forever, kwargs={'poll_interval': 0.01},
                                  daemon=True)
        worker.start()

        try:
            yield path
        finally:
            server.shutdown()
            worker.join(timeout=5)
            server.server_close()
            signer.close()
            path.unlink()
            assert not worker.is_alive(), 'native signer did not stop'


    def signer_client(self, path, public_key=None):
        return SignerClient(str(path), expected_peer_uid=os.geteuid(),
                            expected_peer_gid=os.getegid(),
                            expected_public_key=public_key if public_key is not None
                            else public_key_of(self.treasury_seed))

    @contextlib.contextmanager
    def consume(self):
        if not (self.state / 'state.json').exists():
            reset.Store(self.state, self.data, self.run, self.config['network_id'],
                        {'owner': 'native-key-harness'}, os.getegid()).initialize()
        broker = transport.Broker(self.data, self.state, self.run, os.geteuid(), os.getegid())
        path = self.run / 'key-handoff.sock'
        listener = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        listener.bind(str(path))
        path.chmod(0o660)
        listener.listen(1)
        listener.settimeout(5)
        errors = []

        def serve():
            try:
                connection, _ = listener.accept()
                connection.settimeout(5)
                broker.handle(connection)
            except BaseException as error:
                errors.append(error)

        worker = threading.Thread(target=serve, daemon=True)
        worker.start()
        descriptors = []
        try:
            result, descriptors = generation.receive(str(path), 'core', 1,
                                                     os.geteuid(), os.getegid())
            adopted = generation.child_environment(result, descriptors, 'core')
            view, adopted = generation.materialize(result, descriptors, 'core', adopted)
            try:
                yield result, adopted, Path(view.name)
            finally:
                view.cleanup()
        finally:
            generation.close_descriptors(descriptors)
            listener.close()
            worker.join(timeout=6)
            path.unlink()
            assert not worker.is_alive(), 'native generation handoff did not stop'
            assert not errors, errors

    def assert_no_copy(self, testcase, seed, source):
        for entry in self.root.rglob('*'):
            if entry.is_file() and entry != source:
                content = entry.read_bytes()
                for forbidden in (seed, seed.hex().encode(), seed.hex().upper().encode()):
                    testcase.assertNotIn(forbidden, content, str(entry))
        testcase.assertEqual(source.stat().st_mode & 0o777, 0o600)
        testcase.assertEqual(source.stat().st_nlink, 1)

    def assert_public_handoff(self, testcase):
        public = public_key_of(self.receipt_seed)
        with self.consume() as (result, adopted, view):
            testcase.assertEqual(result['public']['LAYERX_NODE_SEQUENCER_PUBLIC_KEY'], public.hex())
            testcase.assertEqual(adopted['LAYERX_CORE_SEQUENCER_ID'], hashlib.sha256(public).hexdigest())
            testcase.assertEqual(adopted['LAYERX_GENERATION_NUMBER'], '1')
            testcase.assertNotIn('LAYERX_NODE_SEQUENCER_PRIVATE_KEY', adopted)
            testcase.assertNotIn('LAYERX_NODE_SEQUENCER_KEY_FILE', adopted)
            for entry in view.iterdir():
                testcase.assertEqual(entry.stat().st_mode & 0o777, 0o600)
                testcase.assertNotIn(self.receipt_seed, entry.read_bytes())
                testcase.assertNotIn(self.receipt_seed.hex().encode(), entry.read_bytes())

"""Real command-provider and Unix client/server proofs for audit 22.30.83."""
from concurrent.futures import ThreadPoolExecutor
from contextlib import contextmanager
import json
import os
from pathlib import Path
import shlex
import socket
import sys
import threading
import time

import pytest

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
from client import SignerClient, SignerError
from provider import (CommandProvider, FileProvider, ProviderError, ProviderTimeout,
                      bounded_process, memory_file, operation_deadline, verify,
                      verify_binding)
from signer import BindingPolicy, Handler, Server, Signer

DIGEST = bytes([9]) * 32
PEER = HERE / 'fixture_provider_22_33_1454.py'


def wait_until(condition, timeout=3):
    deadline = time.monotonic() + timeout
    while not condition():
        assert time.monotonic() < deadline, 'condition did not become true within its bound'
        time.sleep(0.005)


def configure(state, **values):
    temporary = state.with_suffix('.new')
    temporary.write_text(json.dumps(values))
    temporary.replace(state)


def calls(state):
    path = Path(str(state) + '.calls')
    return [line.split() for line in path.read_text().splitlines()] if path.exists() else []


def assert_reaped(state):
    for pid, _ in calls(state):
        with pytest.raises(ChildProcessError):
            os.waitpid(int(pid), os.WNOHANG)
        with pytest.raises(ProcessLookupError):
            os.kill(int(pid), 0)


@contextmanager
def command_backend(tmp_path, timeout=0.8):
    state = tmp_path / 'provider.json'
    configure(state)
    command = shlex.join([sys.executable, '-u', str(PEER), str(state)])
    backend = CommandProvider(command, timeout)
    try:
        yield backend, state
    finally:
        backend.close()


@contextmanager
def serving(tmp_path, backend, timeout=3, maximum=3, policy=None, clock=time.monotonic):
    signer = Signer(backend, [os.geteuid()], policy)
    path = str(tmp_path / 'signer.sock')
    server = Server(path, Handler, max_active_requests=maximum, request_timeout=timeout, clock=clock)
    server.signer = signer
    thread = threading.Thread(target=server.serve_forever, kwargs={'poll_interval': 0.01})
    thread.start()
    client = SignerClient(path, timeout=4, expected_peer_uid=os.geteuid(),
                          expected_peer_gid=os.getegid(), expected_public_key=signer.public_key)
    try:
        yield server, client
    finally:
        server.shutdown()
        thread.join(timeout=3)
        assert not thread.is_alive()
        wait_until(lambda: not server.deadlines)
        server.server_close()
        signer.close()


@pytest.fixture
def file_backend(tmp_path):
    path = tmp_path / 'key'
    path.write_bytes(bytes([7]) * 32)
    path.chmod(0o600)
    backend = FileProvider(str(path))
    yield backend
    backend.close()


@pytest.mark.parametrize('mode', ['hang', 'stdout', 'stderr', 'closed-pipes', 'descendant-pipes'])
def test_provider_process_bounds_client_recovery(tmp_path, mode):
    with command_backend(tmp_path) as (backend, state):
        with serving(tmp_path, backend) as (server, client):
            configure(state, sign=mode)
            started = time.monotonic()
            code = 'busy' if mode in ('hang', 'closed-pipes', 'descendant-pipes') else 'signature_refused'
            with pytest.raises(SignerError, match=code):
                client.sign(DIGEST)
            assert time.monotonic() - started < 2.2
            wait_until(lambda: not server.deadlines)
            assert_reaped(state)
            configure(state)
            signature = client.sign(DIGEST)
            assert verify(client.public_key(), DIGEST, signature)
            wait_until(lambda: not server.deadlines)
            assert_reaped(state)


@pytest.mark.parametrize('mode', ['hang', 'stdout', 'stderr'])
def test_provider_process_bounds_public_key(tmp_path, mode):
    with command_backend(tmp_path) as (backend, state):
        configure(state, **{'public-key': mode})
        started = time.monotonic()
        with pytest.raises(ProviderError):
            Signer(backend, [os.geteuid()])
        assert time.monotonic() - started < 2.2
        assert_reaped(state)


def test_provider_signature_and_identity_checks_remain(tmp_path):
    with command_backend(tmp_path) as (backend, state):
        with serving(tmp_path, backend) as (server, client):
            configure(state, sign='bad-signature')
            with pytest.raises(SignerError, match='signature_refused'):
                client.sign(DIGEST)
            configure(state, **{'public-key': 'changed-key'})
            with pytest.raises(SignerError, match='signature_refused'):
                client.sign(DIGEST)
            configure(state)
            assert verify(client.public_key(), DIGEST, client.sign(DIGEST))
            wait_until(lambda: not server.deadlines)
            assert_reaped(state)


def test_provider_bounded_pipes_stdin_descriptors_and_deadline(tmp_path):
    state = tmp_path / 'pipes.json'
    configure(state)
    descriptor = memory_file('pipe-fixture', b'descriptor')
    command = [sys.executable, '-u', str(PEER), str(state), 'pipe', str(descriptor)]
    try:
        assert bounded_process(command, 2, (descriptor,), b'input:', 32) == b'input:descriptor\n'
        configure(state, pipe='hang')
        started = time.monotonic()
        with operation_deadline(started + 0.4):
            with pytest.raises(ProviderTimeout):
                bounded_process(command, 10, (descriptor,), b'x' * 262144, 32)
        assert time.monotonic() - started < 1.2
        assert_reaped(state)
    finally:
        os.close(descriptor)


def test_signer_concurrency_bound_busy_and_overload_recovery(tmp_path, file_backend):
    with serving(tmp_path, file_backend, timeout=0.7, maximum=3) as (server, client):
        server.signer._lock.acquire()
        try:
            with ThreadPoolExecutor(max_workers=3) as workers:
                started = time.monotonic()
                active = [workers.submit(client.sign, DIGEST) for _ in range(3)]
                wait_until(lambda: len(server.deadlines) == 3)
                with ThreadPoolExecutor(max_workers=8) as overload:
                    refused = [overload.submit(client.public_key) for _ in range(16)]
                    for result in refused:
                        with pytest.raises(SignerError, match='busy'):
                            result.result(timeout=1)
                        assert len(server.deadlines) <= 3
                # The refusal is explicit JSON, including its retry disposition.
                with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as connection:
                    connection.settimeout(1)
                    connection.connect(client._path)
                    connection.sendall(b'public-key\n')
                    reply = json.loads(connection.makefile('rb').readline())
                    assert reply == {'error': {'code': 'busy', 'retry': 'later'}}
                for result in active:
                    with pytest.raises(SignerError, match='busy'):
                        result.result(timeout=1.5)
                assert time.monotonic() - started < 1.5
        finally:
            server.signer._lock.release()
        wait_until(lambda: not server.deadlines)
        assert verify(client.public_key(), DIGEST, client.sign(DIGEST))


def test_signer_deadline_covers_read_and_lock_wait(tmp_path, file_backend):
    with serving(tmp_path, file_backend, timeout=0.7) as (server, client):
        server.signer._lock.acquire()
        try:
            with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as connection:
                connection.settimeout(2)
                connection.connect(client._path)
                started = time.monotonic()
                connection.sendall(b'sign ')
                wait_until(lambda: len(server.deadlines) == 1)
                time.sleep(0.4)
                connection.sendall(DIGEST.hex().encode() + b'\n')
                reply = json.loads(connection.makefile('rb').readline())
                assert reply == {'error': {'code': 'busy', 'retry': 'later'}}
                assert time.monotonic() - started < 0.95
        finally:
            server.signer._lock.release()
        wait_until(lambda: not server.deadlines)
        assert verify(client.public_key(), DIGEST, client.sign(DIGEST))


def test_signer_deadline_spans_provider_operations(tmp_path):
    clock_path = tmp_path / 'operation-clock'
    clock_path.write_text('100.0')

    def clock():
        return float(clock_path.read_text())

    with command_backend(tmp_path, timeout=3) as (backend, state):
        with serving(tmp_path, backend, timeout=0.9, clock=clock) as (server, client):
            previous = calls(state)
            configure(state, clock=str(clock_path), **{'public-key_delay': 0.45, 'sign_delay': 0.6})
            started = clock()
            with pytest.raises(SignerError, match='busy'):
                client.sign(DIGEST)
            assert clock() - started == pytest.approx(1.05)
            assert 0.9 < clock() - started < 1.2
            assert [operation for _, operation in calls(state)[len(previous):]] == ['public-key', 'sign']
            assert any(operation == 'sign' for _, operation in calls(state))
            wait_until(lambda: not server.deadlines)
            assert_reaped(state)
            configure(state, clock=str(clock_path))
            assert verify(client.public_key(), DIGEST, client.sign(DIGEST))
            assert clock() - started == pytest.approx(1.05)


def test_signer_binding_deadline_and_signature_checks(tmp_path):
    clock_path = tmp_path / 'operation-clock'
    clock_path.write_text('100.0')

    def clock():
        return float(clock_path.read_text())

    policy_path = tmp_path / 'policy.json'
    policy_path.write_text(json.dumps({'version': 1, 'network_id': 125,
                                       'asset_id': '11' * 32, 'recipient': '22' * 20}))
    policy_path.chmod(0o600)
    policy = BindingPolicy(str(policy_path))
    with command_backend(tmp_path) as (backend, state):
        with serving(tmp_path, backend, timeout=0.7, policy=policy, clock=clock) as (server, client):
            signer = server.signer
            arguments = (125, signer._binding_account, bytes.fromhex('11' * 32),
                         bytes.fromhex('22' * 20), bytes([3]) * 32)
            signer._lock.acquire()
            try:
                with pytest.raises(SignerError, match='busy'):
                    client.bind(*arguments)
            finally:
                signer._lock.release()
            wait_until(lambda: not server.deadlines)
            configure(state, clock=str(clock_path), bind='bad-signature')
            with pytest.raises(SignerError, match='binding_refused'):
                client.bind(*arguments)
            configure(state, clock=str(clock_path))
            signature = client.bind(*arguments)
            binding = arguments[0].to_bytes(4, 'big') + b''.join(arguments[1:])
            assert verify_binding(signer.public_key, binding, signature)
            with pytest.raises(SignerError, match='binding_refused'):
                client.bind(126, *arguments[1:])
            assert clock() == 100.0


def test_signer_slow_reader_releases_capacity(tmp_path, file_backend):
    with serving(tmp_path, file_backend, timeout=0.35, maximum=1) as (server, client):
        with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as connection:
            connection.settimeout(1)
            connection.connect(client._path)
            connection.sendall(b'p')
            wait_until(lambda: len(server.deadlines) == 1)
            with pytest.raises(SignerError, match='busy'):
                client.public_key()
            time.sleep(0.2)
            connection.sendall(b'u')
            started = time.monotonic()
            assert connection.recv(4096) == b''
            assert time.monotonic() - started < 0.3
        wait_until(lambda: not server.deadlines)
        assert verify(client.public_key(), DIGEST, client.sign(DIGEST))

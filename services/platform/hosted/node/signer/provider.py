#!/usr/bin/env python3
# Signing providers for the LayerX treasury signer.
#
# A provider owns one Ed25519 identity and answers two questions: the public
# key it signs under, and the 64-byte signature over a 32-byte digest. The
# file provider keeps the seed in anonymous memory for the life of the
# process and never writes it back to disk. The command provider delegates
# both answers to an external program, which is how a KMS or an HSM is placed
# behind the signer without changing the socket contract:
#
#   PROGRAM public-key        -> 64 lowercase hex characters on standard output
#   PROGRAM sign <64 hex>     -> 128 lowercase hex characters on standard output
#   PROGRAM bind <240 hex>    -> signature over the recipient domain and typed payload
#
# The signer verifies every signature a provider returns against the public
# key the provider advertised before it answers a client.
from contextlib import contextmanager
from contextvars import ContextVar
import math
import os
import selectors
import shlex
import signal
import stat
import subprocess
import time

from cryptography.exceptions import InvalidSignature
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey, Ed25519PublicKey

PKCS8_ED25519_PREFIX = bytes.fromhex('302e020100300506032b657004220420')
SPKI_ED25519_PREFIX = bytes.fromhex('302a300506032b6570032100')
MAXIMUM_MATERIAL_BYTES = 4096
MAXIMUM_COMMAND_OUTPUT_BYTES = 4096
DEFAULT_OPERATION_TIMEOUT = 10.0
OPERATION_DEADLINE = ContextVar('treasury_operation_deadline', default=None)
OPERATION_CLOCK = ContextVar('treasury_operation_clock', default=time.monotonic)
RECIPIENT_BINDING_DOMAIN = b'LX:SETTLE:RECIPIENT:v1\0'
RECIPIENT_BINDING_BYTES = 120


class ProviderError(Exception):
    pass


class ProviderTimeout(ProviderError):
    pass


@contextmanager
def operation_deadline(deadline, clock=None):
    clock_token = OPERATION_CLOCK.set(OPERATION_CLOCK.get() if clock is None else clock)
    previous = OPERATION_DEADLINE.get()
    token = OPERATION_DEADLINE.set(deadline if previous is None else min(previous, deadline))
    try:
        yield
    finally:
        OPERATION_DEADLINE.reset(token)
        OPERATION_CLOCK.reset(clock_token)


def monotonic():
    return OPERATION_CLOCK.get()()


def remaining_time(deadline=None):
    if deadline is None:
        deadline = OPERATION_DEADLINE.get()
    if deadline is None:
        return DEFAULT_OPERATION_TIMEOUT
    remaining = deadline - monotonic()
    if remaining <= 0:
        raise ProviderTimeout('the treasury operation deadline expired')
    return remaining


def bounded_process(command, timeout, descriptors=(), stdin=None,
                    maximum=MAXIMUM_COMMAND_OUTPUT_BYTES):
    if not math.isfinite(timeout) or timeout <= 0 or maximum < 0:
        raise ProviderError('provider process bounds must be finite and positive')
    deadline = monotonic() + timeout
    request_deadline = OPERATION_DEADLINE.get()
    if request_deadline is not None:
        deadline = min(deadline, request_deadline)
    process = None
    try:
        remaining_time(deadline)
        process = subprocess.Popen(command, stdin=subprocess.PIPE if stdin is not None
                                   else subprocess.DEVNULL, stdout=subprocess.PIPE,
                                   stderr=subprocess.PIPE, pass_fds=tuple(descriptors),
                                   start_new_session=True)
        output = bytearray()
        counts = {'stdout': 0, 'stderr': 0}
        pending = memoryview(stdin) if stdin is not None else None
        with selectors.DefaultSelector() as selector:
            for pipe, name in ((process.stdout, 'stdout'), (process.stderr, 'stderr')):
                os.set_blocking(pipe.fileno(), False)
                selector.register(pipe, selectors.EVENT_READ, name)
            if pending:
                os.set_blocking(process.stdin.fileno(), False)
                selector.register(process.stdin, selectors.EVENT_WRITE, 'stdin')
            elif process.stdin is not None:
                process.stdin.close()
            while selector.get_map():
                for key, _ in selector.select(remaining_time(deadline)):
                    remaining_time(deadline)
                    if key.data == 'stdin':
                        try:
                            written = os.write(key.fd, pending[:4096])
                            pending = pending[written:]
                        except BrokenPipeError:
                            pending = pending[:0]
                        if not pending:
                            selector.unregister(key.fileobj)
                            key.fileobj.close()
                        continue
                    limit = maximum if key.data == 'stdout' else MAXIMUM_COMMAND_OUTPUT_BYTES
                    chunk = os.read(key.fd, min(4096, limit - counts[key.data] + 1))
                    if not chunk:
                        selector.unregister(key.fileobj)
                        key.fileobj.close()
                        continue
                    counts[key.data] += len(chunk)
                    if counts[key.data] > limit:
                        raise ProviderError('the provider process output exceeds its bound')
                    if key.data == 'stdout':
                        output.extend(chunk)
            process.wait(timeout=remaining_time(deadline))
        remaining_time(deadline)
        if process.returncode != 0:
            raise ProviderError('the provider process refused the request')
        return bytes(output)
    except subprocess.TimeoutExpired as error:
        raise ProviderTimeout('the treasury operation deadline expired') from error
    except (OSError, subprocess.SubprocessError) as error:
        raise ProviderError('the provider process did not answer') from error
    finally:
        if process is not None:
            # Kill the session as well: descendants may still hold a pipe open.
            try:
                os.killpg(process.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            process.wait()
            for pipe in (process.stdin, process.stdout, process.stderr):
                if pipe is not None:
                    pipe.close()


def openssl(arguments, descriptors=(), stdin=None, maximum=MAXIMUM_COMMAND_OUTPUT_BYTES):
    return bounded_process(['openssl'] + arguments, DEFAULT_OPERATION_TIMEOUT,
                           descriptors, stdin, maximum)


def memory_file(name, value):
    descriptor = os.memfd_create(name, 0)
    try:
        os.write(descriptor, value)
        os.lseek(descriptor, 0, os.SEEK_SET)
    except BaseException:
        os.close(descriptor)
        raise
    return descriptor


def parse_hex(text, length):
    value = text.strip()
    if len(value) != length * 2 or value != value.lower():
        raise ProviderError('provider output is not lowercase hexadecimal of the required length')
    try:
        decoded = bytes.fromhex(value)
    except ValueError as error:
        raise ProviderError('provider output is not hexadecimal') from error
    return decoded


def verify(public_key, digest, signature):
    if len(public_key) != 32 or len(digest) != 32 or len(signature) != 64:
        return False
    return _verify_message(public_key, digest, signature)


def verify_binding(public_key, binding, signature):
    if len(public_key) != 32 or len(binding) != RECIPIENT_BINDING_BYTES \
            or len(signature) != 64:
        return False
    return _verify_message(public_key, RECIPIENT_BINDING_DOMAIN + binding, signature)


def _verify_message(public_key, digest, signature):
    try:
        Ed25519PublicKey.from_public_bytes(public_key).verify(signature, digest)
        return True
    except (InvalidSignature, ValueError):
        return False


class Provider:
    name = 'provider'

    def public_key(self):
        raise NotImplementedError

    def sign(self, digest):
        raise NotImplementedError

    def bind(self, binding):
        raise ProviderError('the provider has no recipient binding capability')

    def close(self):
        return None


class FileProvider(Provider):
    name = 'file'

    def __init__(self, path):
        self._key = None
        seed = self._load(path)
        try:
            self._key = memory_file('treasury-signer-key', PKCS8_ED25519_PREFIX + bytes(seed))
        finally:
            for index in range(len(seed)):
                seed[index] = 0

    @staticmethod
    def _load(path):
        descriptor = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
        try:
            info = os.fstat(descriptor)
            mode = stat.S_IMODE(info.st_mode)
            if not stat.S_ISREG(info.st_mode) or info.st_nlink != 1:
                raise ProviderError('treasury key material must be a regular file with one link')
            if mode & 0o007 or mode & 0o020:
                raise ProviderError('treasury key material must not be readable by others '
                                    'or writable by its group')
            if info.st_uid not in (os.geteuid(), 0):
                raise ProviderError('treasury key material must be owned by the signer or by root')
            if info.st_size > MAXIMUM_MATERIAL_BYTES:
                raise ProviderError('treasury key material exceeds its bound')
            raw = bytearray(os.read(descriptor, MAXIMUM_MATERIAL_BYTES + 1))
        finally:
            os.close(descriptor)
        if len(raw) == 32:
            return raw
        text = bytes(raw).decode('ascii', 'replace').strip()
        for index in range(len(raw)):
            raw[index] = 0
        if len(text) != 64 or text != text.lower():
            raise ProviderError('treasury key material must hold 32 raw bytes or 64 hex characters')
        try:
            return bytearray(bytes.fromhex(text))
        except ValueError as error:
            raise ProviderError('treasury key material is not hexadecimal') from error

    def public_key(self):
        return self._private_key().public_key().public_bytes(
            serialization.Encoding.Raw, serialization.PublicFormat.Raw)

    def _private_key(self):
        if self._key is None:
            raise ProviderError('treasury provider is closed')
        encoded = os.pread(self._key, len(PKCS8_ED25519_PREFIX) + 32, 0)
        if len(encoded) != len(PKCS8_ED25519_PREFIX) + 32 or not encoded.startswith(PKCS8_ED25519_PREFIX):
            raise ProviderError('treasury key material is not an Ed25519 identity')
        return Ed25519PrivateKey.from_private_bytes(encoded[len(PKCS8_ED25519_PREFIX):])

    def sign(self, digest):
        if len(digest) != 32:
            raise ProviderError('a treasury signature covers a 32-byte digest')
        return self._sign_message(digest)

    def bind(self, binding):
        if len(binding) != RECIPIENT_BINDING_BYTES:
            raise ProviderError('recipient binding has the wrong length')
        return self._sign_message(RECIPIENT_BINDING_DOMAIN + binding)

    def _sign_message(self, digest):
        signature = self._private_key().sign(digest)
        if len(signature) != 64:
            raise ProviderError('the treasury signature has an unexpected length')
        return signature

    def close(self):
        if self._key is not None:
            os.close(self._key)
            self._key = None


class CommandProvider(Provider):
    name = 'command'

    def __init__(self, command, timeout):
        self._command = shlex.split(command)
        if not self._command:
            raise ProviderError('the provider command is empty')
        if not math.isfinite(timeout) or timeout <= 0:
            raise ProviderError('the provider timeout must be positive')
        self._timeout = timeout

    def _run(self, arguments):
        return bounded_process(self._command + arguments, self._timeout).decode('ascii', 'replace')

    def public_key(self):
        return parse_hex(self._run(['public-key']), 32)

    def sign(self, digest):
        if len(digest) != 32:
            raise ProviderError('a treasury signature covers a 32-byte digest')
        return parse_hex(self._run(['sign', digest.hex()]), 64)

    def bind(self, binding):
        if len(binding) != RECIPIENT_BINDING_BYTES:
            raise ProviderError('recipient binding has the wrong length')
        return parse_hex(self._run(['bind', binding.hex()]), 64)


def load(name, key_file, command, timeout):
    if name == 'file':
        if not key_file or command:
            raise ProviderError('the file provider takes a key file and no provider command')
        return FileProvider(key_file)
    if name == 'command':
        if not command or key_file:
            raise ProviderError('the command provider takes a provider command and no key file')
        return CommandProvider(command, timeout)
    raise ProviderError('the provider must be file or command')

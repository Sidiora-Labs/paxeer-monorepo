"""Local Ed25519 command peer with controlled process failure modes."""
import json
import os
from pathlib import Path
import signal
import sys
import time

from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey


def run():
    state = Path(sys.argv[1])
    operation = sys.argv[2]
    config = json.loads(state.read_text())
    descriptor = os.open(str(state) + '.calls', os.O_WRONLY | os.O_CREAT | os.O_APPEND, 0o600)
    try:
        os.write(descriptor, f'{os.getpid()} {operation}\n'.encode())
    finally:
        os.close(descriptor)
    mode = config.get(operation, 'valid')
    if mode == 'hang':
        signal.signal(signal.SIGTERM, signal.SIG_IGN)
        while True:
            signal.pause()
    if mode in ('stdout', 'stderr'):
        destination = 1 if mode == 'stdout' else 2
        while True:
            os.write(destination, b'x' * 4096)
    if mode == 'closed-pipes':
        os.close(1)
        os.close(2)
        while True:
            signal.pause()
    if mode == 'descendant-pipes':
        child = os.fork()
        if child == 0:
            while True:
                signal.pause()
        return
    delay = config.get(operation + '_delay', 0)
    if 'clock' in config:
        clock = Path(config['clock'])
        temporary = clock.with_suffix('.next')
        temporary.write_text(str(float(clock.read_text()) + delay))
        temporary.replace(clock)
    else:
        time.sleep(delay)
    seed = bytes([8 if mode == 'changed-key' else 7]) * 32
    key = Ed25519PrivateKey.from_private_bytes(seed)
    if operation == 'public-key':
        output = key.public_key().public_bytes(serialization.Encoding.Raw,
                                               serialization.PublicFormat.Raw).hex()
    elif operation in ('sign', 'bind'):
        message = bytes.fromhex(sys.argv[3])
        if operation == 'bind':
            message = b'LX:SETTLE:RECIPIENT:v1\0' + message
        if mode == 'bad-signature':
            message = b'different message'
        output = key.sign(message).hex()
    elif operation == 'pipe':
        output = (sys.stdin.buffer.read() + os.read(int(sys.argv[3]), 4096)).decode('ascii')
    else:
        raise ValueError('unknown fixture operation')
    sys.stdout.write(output + '\n')
    sys.stdout.flush()


if __name__ == '__main__':
    run()

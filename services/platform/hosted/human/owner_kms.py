import json
import os
from pathlib import Path
import subprocess
import selectors
import signal
import time

from provision import fields, protected_bytes, protected_json, require, write_json
from owner_native import protected_write, digest, span

PREPARE_DEADLINE_SECONDS = 10
PREPARE_OUTPUT_BYTES = 65536


def prepare_child(executable, call):
    """Bound both pipes and the lifetime of a preparation process group."""
    deadline = time.monotonic() + PREPARE_DEADLINE_SECONDS
    request = json.dumps(call, allow_nan=False).encode()
    require(len(request) <= 16384, 'KMS prepare', 'bounded preparation request')
    try:
        child = subprocess.Popen([executable, 'prepare'], stdin=subprocess.PIPE,
            stdout=subprocess.PIPE, stderr=subprocess.PIPE, start_new_session=True)
    except OSError as error:
        raise ValueError('KMS prepare refused: executable unavailable; retain custody and retry the same request') from error
    output = bytearray()
    total = 0
    offset = 0
    try:
        with selectors.DefaultSelector() as selector:
            for pipe in (child.stdin, child.stdout, child.stderr):
                os.set_blocking(pipe.fileno(), False)
            selector.register(child.stdin, selectors.EVENT_WRITE)
            selector.register(child.stdout, selectors.EVENT_READ)
            selector.register(child.stderr, selectors.EVENT_READ)
            while selector.get_map():
                remaining = deadline - time.monotonic()
                require(remaining > 0, 'KMS prepare',
                        'preparation deadline exceeded; retain custody and retry the same request')
                for key, _ in selector.select(remaining):
                    pipe = key.fileobj
                    if pipe is child.stdin:
                        try:
                            offset += os.write(pipe.fileno(), request[offset:])
                        except BrokenPipeError:
                            offset = len(request)
                        if offset == len(request):
                            selector.unregister(pipe)
                            pipe.close()
                    else:
                        data = os.read(pipe.fileno(), min(8192, PREPARE_OUTPUT_BYTES - total + 1))
                        if not data:
                            selector.unregister(pipe)
                            pipe.close()
                            continue
                        total += len(data)
                        require(total <= PREPARE_OUTPUT_BYTES, 'KMS prepare',
                                'preparation output exceeds bound; retain custody and retry the same request')
                        if pipe is child.stdout:
                            output.extend(data)
            remaining = deadline - time.monotonic()
            require(remaining > 0, 'KMS prepare', 'preparation deadline exceeded; retain custody')
            try:
                status = child.wait(timeout=remaining)
            except subprocess.TimeoutExpired as error:
                raise ValueError('KMS prepare refused: preparation deadline exceeded; retain custody') from error
            require(status == 0, 'KMS prepare', 'real KMS sponsor preparation; retain custody')
        return bytes(output)
    finally:
        # Descendants may retain a pipe or survive their immediate parent.
        try:
            os.killpg(child.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
        child.wait()
        for pipe in (child.stdin, child.stdout, child.stderr):
            if not pipe.closed:
                pipe.close()


def retain_json(path, value):
    if path.exists():
        require(protected_json(path) == value, path, 'identical retained KMS result')
    else:
        write_json(path, value)


def prepare(work_dir, executable, config_directory):
    root = Path(work_dir) / 'human-evidence-input'
    config_directory = Path(config_directory)
    require(config_directory.is_absolute() and config_directory.resolve() == config_directory,
            config_directory, 'canonical onboarding configuration directory')
    for suffix in ('TENANCY_DIGEST', 'AUTH_INDEX_KEY', 'STREAM_CURSOR_KEY'):
        name = 'LAYERX_HUMAN_' + suffix
        value = protected_bytes(config_directory / name, 128).decode()
        require(os.environ.get(name) == value, config_directory, 'same protected production configuration')
    funding = int(os.environ['LAYERX_HUMAN_ONBOARDING_INITIAL_FUNDING'])
    require(0 < funding < 2**128, config_directory, 'finite onboarding funding')
    request = protected_json(root / 'owner-request.json')
    fields(request, 'email display_name idempotency_key now', root, 'owner provisioning request')
    call = {key: request[key] for key in ('email', 'display_name', 'idempotency_key')}
    from provision import strict_pairs
    owner = json.loads(prepare_child(executable, call), object_pairs_hook=strict_pairs)
    fields(owner, 'principal did public_key pending_key recovery_root recovery_threshold recovery_delay_seconds registration_action recovery_action recipient', root, 'KMS sponsor export')
    for name in ('public_key', 'pending_key', 'recovery_root', 'registration_action', 'recovery_action'):
        require(type(owner[name]) is list and len(owner[name]) == 32
                and all(type(value) is int and 0 <= value <= 255 for value in owner[name])
                and any(owner[name]), root, name)
    require(owner['public_key'] != owner['pending_key']
            and owner['did'] == 'did:layerx:' + owner['principal'], root, 'actual provider and KMS identity binding')
    retain_json(root / 'owner-kms.json', owner)
    retain_json(root / 'onboarding-configuration.json', dict(directory=str(config_directory),
        sponsor_principal=owner['principal'], initial_funding=funding))
    retain_json(Path(work_dir) / 'human-owner-result.json', {key: owner[key] for key in
        ('principal', 'did', 'recovery_root', 'recovery_threshold', 'recovery_delay_seconds')})
    public = bytes(owner['public_key'])
    import hashlib
    account = hashlib.sha256(b'LX:ACCOUNT:v1' + span(('agent:' + owner['did'] + ':main').encode())).hexdigest()
    retain_json(root / 'owner-admission.json', dict(did=owner['did'], public_key=public.hex(), owner_account=account))
    from onboarding_material import write
    write(root / 'owner-admission.txt', owner['did'].encode().hex().encode() + b':' + public.hex().encode() + b':0\n')


def sign(config, owner, label, payload, sequence, not_before, not_after, action):
    require(label in ('credit', 'identity', 'rotation', 'recovery'), config, 'bootstrap operation')
    request = dict(principal=owner['principal'], operation=label, sequence=sequence,
        not_before_ms=not_before, not_after_ms=not_after, action_key=list(action), fee_limit=config['fee_limit'],
        credit=list(payload) if label == 'credit' else None, effective_sequence=None,
        policy_start_ms=None, policy_end_ms=None)
    if label == 'rotation':
        require(len(payload) == 92 and payload[:4] == b'\x71\2\0\4', config, 'canonical rotation policy')
        request.update(policy_start_ms=int.from_bytes(payload[68:76], 'big'),
            policy_end_ms=int.from_bytes(payload[76:84], 'big'), effective_sequence=int.from_bytes(payload[84:92], 'big'))
    from onboarding_socket import request as signed_request
    result = signed_request(config['kms_signer'], 'sign', request)
    fields(result, 'activity activity_id', config, 'signed bootstrap response')
    signed = bytes.fromhex(result['activity'])
    verify_signed(signed, config, owner, payload, sequence, not_before, not_after, action,
                  8 if label == 'credit' else 7, 1 if label in ('credit', 'identity') else 2 if label == 'rotation' else 3)
    require(digest(b'activity-id', signed).hex() == result['activity_id'], config, 'signed activity identity')
    return signed


def verify_signed(signed, config, owner, payload, sequence, not_before, not_after, action, module, ordinal):
    from owner_native import Reader
    from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PublicKey
    r = Reader(signed, 'KMS signed activity')
    require(len(signed) <= 1048576 and r.take(5) == b'\0\3\x10\1\14', config, 'signed native header')
    def tag(value):
        require(r.number(1) == value, config, 'ordered canonical activity field')
    tag(1); require(r.number(2) == 3, config, 'inner protocol')
    tag(2); require(r.number(4) == config['network_id'], config, 'network')
    tag(3); require(r.number(4) == module << 16 | ordinal, config, 'operation')
    tag(4); require(r.span(255) == owner['did'].encode(), config, 'actor')
    tag(5); require(r.span(32) == bytes(owner['public_key']), config, 'owner')
    tag(6); require(r.number(8) == sequence, config, 'sequence')
    tag(7); require(r.number(8) == not_before and r.number(8) == not_after, config, 'time bounds')
    tag(8); require(r.span(32) == action, config, 'action key')
    tag(9); require(r.number(16) == config['fee_limit'], config, 'fee limit')
    tag(10); require(r.span(32) == digest(b'payload-hash', payload), config, 'payload hash')
    tag(11); require(r.span(524288) == payload, config, 'canonical payload')
    unsigned = signed[:4] + b'\13' + signed[5:r.offset]
    tag(12); signature = r.span(64); r.finish()
    require(len(signature) == 64, config, 'signature length')
    Ed25519PublicKey.from_public_bytes(bytes(owner['public_key'])).verify(signature, digest(b'signature-preimage', unsigned))


if __name__ == '__main__':
    import argparse
    parser = argparse.ArgumentParser()
    parser.add_argument('--work-dir', required=True)
    parser.add_argument('--executable', required=True)
    parser.add_argument('--config-directory', required=True)
    args = parser.parse_args()
    prepare(args.work_dir, args.executable, args.config_directory)

import hashlib
import json
import os
from pathlib import Path
import re
import ssl
import stat
import struct
import subprocess
import time
import urllib.request
import urllib.error
import urllib.parse

from provision import Refused, fields, h32, protected_bytes, protected_json, require, uint, write_json

GUARDIAN_ROLES = ('guarantor-1', 'guarantor-2', 'sequencer')
GUARDIAN_EPOCH_MAXIMUM = 64


def digest(domain, data):
    return hashlib.sha256(b'LXP/v1/' + domain + b'\0' + data).digest()


def span(data):
    return struct.pack('>I', len(data)) + data


class Reader:
    def __init__(self, data, path):
        self.data, self.offset, self.path = data, 0, path

    def take(self, count):
        require(0 <= count <= len(self.data) - self.offset, self.path, 'truncated native encoding')
        result = self.data[self.offset:self.offset + count]
        self.offset += count
        return result

    def number(self, count):
        return int.from_bytes(self.take(count), 'big')

    def span(self, maximum):
        length = self.number(4)
        require(length <= maximum, self.path, 'native field bound')
        return self.take(length)

    def finish(self):
        require(self.offset == len(self.data), self.path, 'native trailing bytes')


def protected_write(path, data):
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(fd, 'wb') as output:
        output.write(data)
        output.flush()
        os.fsync(output.fileno())


def authority_document(request, context, path):
    deadline = time.monotonic() + 30
    while True:
        try:
            with urllib.request.urlopen(request, context=context,
                    timeout=max(0.001, deadline - time.monotonic())) as response:
                body = response.read(1048577)
                require(len(body) <= 1048576, path, 'authority document bound')
                return json.loads(body)
        except urllib.error.HTTPError as error:
            if error.code not in (404, 503):
                raise
            try:
                document = json.loads(error.read(4097))
                refusal = document['error']
                code = refusal['code']
                delay = refusal.get('retry_after_seconds', 0.1)
            except (ValueError, KeyError, TypeError):
                raise error
            if code not in ('unknown_activity', 'replica_evidence_unavailable',
                            'replica_unavailable', 'receipt_source_unavailable'):
                raise
            if not isinstance(delay, (int, float)) or isinstance(delay, bool) or not 0 < delay <= 5:
                raise
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                raise
            time.sleep(min(delay, remaining))

def receipt_fields(data, public_key, path):
    from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PublicKey
    r = Reader(data, path)
    require(r.take(6) == b'\0\3\x52\1\0\3', path, 'protocol 3 receipt')
    activity = r.span(32)
    sequence = r.number(8)
    previous, resulting, activity_root = r.span(32), r.span(32), r.span(32)
    require(all(len(v) == 32 for v in (activity, previous, resulting, activity_root)), path, 'receipt hashes')
    require(r.number(4) == 0, path, 'successful native execution')
    count = r.number(4)
    require(count <= 512, path, 'effect count')
    effects = []
    for ordinal in range(count):
        module, index, event = r.number(2), r.number(2), r.number(2)
        kind, monetary, root, body = r.number(1), r.number(1), r.span(32), r.span(256)
        require(index == ordinal and kind <= 3 and monetary <= 1 and len(root) == 32, path, 'effect encoding')
        effects.append((module, event, kind, monetary, body))
    r.take(16)
    batch = r.span(32)
    module, version = r.number(2), r.number(4)
    r.take(4)
    operation = r.number(1)
    asset = r.span(32)
    amount = r.number(16)
    source = r.span(32)
    r.take(40)
    target = r.span(32)
    r.take(32)
    for _ in range(3):
        require(len(r.span(32)) == 32, path, 'receipt commitment')
    timestamp = r.number(8)
    unsigned = data[:r.offset] + b'\0'
    require(r.number(1) == 1, path, 'sequencer signature')
    signature = r.span(64)
    r.finish()
    receipt_digest = digest(b'receipt', unsigned)
    try:
        Ed25519PublicKey.from_public_bytes(public_key).verify(signature, receipt_digest)
    except Exception as error:
        raise Refused(f'{path}: native receipt signature refused') from error
    if module == 8:
        credits = [body for emitted_module, event, kind, monetary, body in effects
                   if emitted_module == 8 and event == 1 and kind == 3 and not monetary]
        balances = [body for emitted_module, event, kind, monetary, body in effects
                    if emitted_module == 8 and event == 2 and kind == 3 and not monetary]
        require(operation == 0 and amount == 0 and target == bytes(32) and asset == bytes(32)
                and len(credits) == 1 and len(credits[0]) == 208
                and len(balances) == 1 and len(balances[0]) == 112, path, 'canonical custody credit events')
        event, balance = credits[0], balances[0]
        asset, target, amount = event[32:64], event[64:96], int.from_bytes(event[96:112], 'big')
        require(amount > 0 and int.from_bytes(event[176:192], 'big') + amount == int.from_bytes(event[192:208], 'big')
                and balance[32:48] == balance[48:64]
                and int.from_bytes(balance[64:80], 'big') + amount == int.from_bytes(balance[80:96], 'big'),
                path, 'committed credit supply and balance conservation')
    return dict(activity_id=activity.hex(), receipt_digest=receipt_digest.hex(), sequence=sequence,
                batch=batch.hex(), module=module, version=version, operation=operation,
                asset=asset.hex(), amount=amount, source=source.hex(), target=target.hex(),
                timestamp=timestamp, effects=effects)


def guardian_binding_message(role, identity, epoch, public_key):
    return (b'LX:HUMAN:GUARDIAN:BINDING:v1\0' + struct.pack('>HH', epoch, len(role))
            + role.encode('ascii') + bytes.fromhex(identity) + public_key)


def guardian_rotation_message(role, identity, epoch, predecessor, successor):
    return (b'LX:HUMAN:GUARDIAN:ROTATION:v1\0' + struct.pack('>HH', epoch, len(role))
            + role.encode('ascii') + bytes.fromhex(identity) + predecessor + successor)


def guardian_commitment(threshold, keys):
    return hashlib.sha256(b'LX:HUMAN:RECOVERY:v1\0' + struct.pack('>HH', threshold, len(keys)) + b''.join(keys)).digest()


def guardian_signature(value, path, field):
    require(type(value) is str and re.fullmatch('[0-9a-f]{128}', value) is not None
            and int(value, 16) != 0, path, field)
    return bytes.fromhex(value)


def guardian_verified(public_key, signature, message, path, field):
    from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PublicKey
    try:
        Ed25519PublicKey.from_public_bytes(public_key).verify(signature, message)
    except Exception as error:
        raise Refused(f'{path}: invalid {field}') from error


def guardian_enrollment_key(value, path):
    fields(value, 'role identity public_key epoch custody signature', path, 'guardian enrollment')
    require(value['role'] in GUARDIAN_ROLES, path, 'guardian role')
    h32(value['identity'], path, 'guardian operator identity')
    h32(value['public_key'], path, 'guardian public key')
    uint(value['epoch'], 16, path, 'guardian epoch', 1)
    require(value['epoch'] <= GUARDIAN_EPOCH_MAXIMUM, path, 'guardian epoch bound')
    require(type(value['custody']) is str and value['custody'].startswith('/'), path, 'guardian custody directory')
    custody = Path(value['custody'])
    require(custody.name == value['role'] + '-e' + str(value['epoch'])
            and custody.parent.name == 'human-guardians', path, 'guardian custody directory')
    public = bytes.fromhex(value['public_key'])
    guardian_verified(public, guardian_signature(value['signature'], path, 'guardian binding signature'),
                      guardian_binding_message(value['role'], value['identity'], value['epoch'], public),
                      path, 'guardian binding signature')
    return public


def guardian_member_key(member, path):
    fields(member, 'role identity public_key epoch custody signature rotation', path, 'guardian binding')
    public = guardian_enrollment_key({name: member[name] for name in member if name != 'rotation'}, path)
    rotation = member['rotation']
    if member['epoch'] == 1:
        require(rotation is None, path, 'genesis guardian rotation')
        return public
    fields(rotation, 'predecessor signature', path, 'guardian rotation')
    predecessor = rotation['predecessor']
    previous = guardian_member_key(predecessor, path)
    require(predecessor['role'] == member['role'] and predecessor['identity'] == member['identity']
            and predecessor['epoch'] == member['epoch'] - 1 and previous != public
            and predecessor['custody'] != member['custody'], path, 'guardian rotation chain')
    guardian_verified(previous, guardian_signature(rotation['signature'], path, 'guardian rotation signature'),
                      guardian_rotation_message(member['role'], member['identity'], member['epoch'], previous, public),
                      path, 'guardian rotation signature')
    return public


def guardian_set(document, path, threshold):
    fields(document, 'version threshold public_keys members', path, 'guardian set')
    uint(document['version'], 16, path, 'guardian set version', 1)
    require(document['threshold'] == threshold, path, 'guardian threshold binding')
    members, published = document['members'], document['public_keys']
    require(type(members) is list and type(published) is list and len(published) == len(members)
            and 0 < threshold <= len(members) <= len(GUARDIAN_ROLES), path, 'guardian threshold')
    keys, roles, identities, custody = [], set(), set(), set()
    for member in members:
        keys.append(guardian_member_key(member, path))
        roles.add(member['role'])
        identities.add(member['identity'])
        custody.add(member['custody'])
        require(member['epoch'] <= document['version'], path, 'guardian set version')
    require(len(roles) == len(identities) == len(custody) == len(set(keys)) == len(members),
            path, 'separately enrolled guardians')
    require(document['version'] == max(member['epoch'] for member in members), path, 'guardian set version')
    for key in published:
        h32(key, path, 'guardian public key')
    keys = sorted(keys)
    require(sorted(bytes.fromhex(key) for key in published) == keys, path, 'guardian public key manifest')
    return keys


def _produce(work_dir):
    from merged_owner import produce as merged_produce
    return merged_produce(work_dir)


def produce(work_dir):
    try:
        _produce(work_dir)
    except Refused:
        raise
    except (OSError, ValueError, KeyError, TypeError, OverflowError) as error:
        raise Refused(f'{Path(work_dir) / "human-evidence-input/owner-native.json"}: native provisioning refused; preserve owner-native-run for reconciliation') from error


def prepare_admission(work_dir, secrets_dir):
    from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
    from cryptography.hazmat.primitives.serialization import Encoding, PrivateFormat, PublicFormat, NoEncryption
    from provision import owner_result
    root = Path(work_dir) / 'human-evidence-input'
    owner = owner_result(work_dir, Path(work_dir) / 'human-owner-result.json')
    did = owner['did'].encode('ascii')
    require(0 < len(did) <= 255 and did.startswith(b'did:layerx:act_'), root, 'exact LXIP DID')
    custody = Path(secrets_dir) / 'human-owner'
    custody.mkdir(mode=0o700)
    public = None
    for name in ('owner', 'pending'):
        key = Ed25519PrivateKey.generate()
        protected_write(custody / (name + '.seed'), key.private_bytes(Encoding.Raw, PrivateFormat.Raw, NoEncryption()))
        if name == 'owner':
            public = key.public_key().public_bytes(Encoding.Raw, PublicFormat.Raw)
    account = hashlib.sha256(b'LX:ACCOUNT:v1' + span(b'agent:' + did + b':main')).hexdigest()
    write_json(root / 'owner-admission.json', dict(did=did.decode(), public_key=public.hex(), owner_account=account))
    protected_write(root / 'owner-admission.txt', did.hex().encode() + b':' + public.hex().encode() + b':0\n')

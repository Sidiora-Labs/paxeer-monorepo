#!/usr/bin/env python3
"""Qualify native genesis, receipt authority and protected Human policy material."""
import argparse
import base64
import hashlib
import importlib.util
import json
import os
import re
from pathlib import Path
import stat
import sys

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / 'services/platform/hosted/human'))
sys.path.insert(0, str(ROOT / 'services/platform/hosted/node/signer'))
import material as policy_material
from provider import FileProvider

C = sys.modules.get('human_committed_projections')
if C is None:
    spec = importlib.util.spec_from_file_location('human_committed_projections', Path(__file__).with_name('human-canonical-projections.py'))
    C = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = C
    spec.loader.exec_module(C)


class SecurityPrerequisites:
    def __init__(self, genesis, receipt_key, policy_bundle, network_id, chain_id, uid):
        self.genesis, self.receipt_key, self.bundle = Path(genesis), Path(receipt_key), Path(policy_bundle)
        self.network_id, self.chain_id, self.uid = network_id, chain_id, uid

    def binding(self):
        C.require(type(self.uid) is int and self.uid >= 0 and type(self.network_id) is int and self.network_id > 0,
                  'security provider UID and network required')
        genesis_raw = C.private_bytes(self.genesis, uid=self.uid)
        genesis = C.strict_json(genesis_raw)
        C.require(genesis['chain_id'] == self.chain_id and genesis['validators']
                  and isinstance(genesis['app_state']['bank'], dict)
                  and isinstance(genesis['app_state']['lxphost'], dict), 'native chain genesis required')
        validators = []
        for validator in genesis['validators']:
            public = base64.b64decode(validator['pub_key']['value'], validate=True)
            C.require(validator['pub_key']['type'] == 'tendermint/PubKeyEd25519' and len(public) == 32
                      and hashlib.sha256(public).digest()[:20].hex() == validator['address'].lower()
                      and int(validator['power']) > 0, 'native validator authority differs')
            validators.append({'public_key': public.hex(), 'address': validator['address'].lower(), 'power': int(validator['power'])})
        C.require(len({row['address'] for row in validators}) == len(validators), 'duplicate native validator')
        seed = C.private_bytes(self.receipt_key, 4096, self.uid).strip()
        C.require(len(seed) == 64 and re.fullmatch(rb'[0-9a-f]{64}', seed),
                  'paxd hexadecimal receipt seed contract required')
        provider = FileProvider(self.receipt_key)
        try:
            public = provider.public_key()
        finally:
            provider.close()
        key_id = hashlib.sha256(public).hexdigest()
        pins = genesis['app_state']['sequencer']['receipt_keys']
        matched = [pin for pin in pins if base64.b64decode(pin['public_key'], validate=True) == public
                   and base64.b64decode(pin['key_id'], validate=True).hex() == key_id
                   and int(pin['first_height']) == 1 and int(pin['last_height']) >= 1]
        C.require(len(matched) == 1, 'native receipt signer is not uniquely pinned in genesis')
        for path in [self.bundle, *self.bundle.rglob('*')]:
            info = path.lstat()
            C.require(not path.is_symlink() and info.st_uid == self.uid
                      and ((stat.S_ISDIR(info.st_mode) and stat.S_IMODE(info.st_mode) == 0o700)
                           or (stat.S_ISREG(info.st_mode) and info.st_nlink == 1 and stat.S_IMODE(info.st_mode) == 0o600)),
                      'protected owner policy prerequisite required')
        policy_binding = policy_material.verify_bundle(self.bundle, self.network_id, 125)
        C.require(C.private_bytes(self.genesis, uid=self.uid) == genesis_raw
                  and C.private_bytes(self.receipt_key, 4096, self.uid).strip() == seed,
                  'native security prerequisite changed during admission')
        return {'schema': 'layerx.human.native-security-binding.v2', 'network_id': self.network_id,
                'chain_id': self.chain_id, 'evm_chain_id': 125, 'uid': self.uid,
                'genesis_sha256': hashlib.sha256(genesis_raw).hexdigest(), 'receipt_key_id': key_id,
                'receipt_public_key': public.hex(), 'validators': validators, 'policy': policy_binding}

    def project(self, destination):
        destination = Path(destination)
        info = destination.lstat()
        C.require(stat.S_ISDIR(info.st_mode) and not destination.is_symlink()
                  and info.st_uid == self.uid and stat.S_IMODE(info.st_mode) == 0o700,
                  'private security runtime directory required')
        binding = self.binding()
        path = destination / 'native-binding.json'
        raw = json.dumps(binding, sort_keys=True, separators=(',', ':')).encode()
        if path.exists() or path.is_symlink():
            C.require(C.private_bytes(path, uid=self.uid) == raw, 'retained native security binding changed; reconcile explicitly')
        else:
            fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
            with os.fdopen(fd, 'wb') as stream:
                stream.write(raw)
                stream.flush()
                os.fsync(stream.fileno())
        C.require(C.strict_json(C.private_bytes(path, uid=self.uid)) == binding, 'private native binding projection differs')
        return binding


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--fixture', required=True)
    args = parser.parse_args()
    value = C.strict_json(C.private_bytes(args.fixture))
    C.require(set(value) == {'genesis', 'receipt_key', 'policy_bundle', 'network_id', 'chain_id', 'uid', 'runtime'},
              'closed native security fixture required')
    prerequisites = SecurityPrerequisites(**{key: value[key] for key in value if key != 'runtime'})
    print(json.dumps(prerequisites.project(value['runtime']), sort_keys=True))
    return 0


if __name__ == '__main__':
    try:
        sys.exit(main())
    except (ValueError, OSError, KeyError, TypeError) as error:
        print('human-security-prerequisites: refused: ' + str(error), file=sys.stderr)
        sys.exit(1)

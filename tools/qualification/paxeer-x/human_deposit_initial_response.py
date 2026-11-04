#!/usr/bin/env python3
"""Native funding/transfer submission, exact retry and acknowledgement recovery."""
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import stat
import sys

C = sys.modules.get('human_committed_projections')
if C is None:
    spec = importlib.util.spec_from_file_location('human_committed_projections', Path(__file__).with_name('human-canonical-projections.py'))
    C = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = C
    spec.loader.exec_module(C)


class FundingResponseContract:
    """Retain the exact signed intent before sending across the Human boundary."""
    def __init__(self, client, foreign, chain, principal, state):
        self.client, self.foreign, self.chain = client, foreign, chain
        self.principal, self.state = principal, Path(state)
        info = self.state.parent.lstat()
        C.require(self.state.is_absolute() and self.state.parent.resolve() == self.state.parent
                  and stat.S_ISDIR(info.st_mode) and info.st_uid == os.geteuid()
                  and stat.S_IMODE(info.st_mode) == 0o700, 'private native retry directory required')
        if self.state.exists() or self.state.is_symlink():
            self.retained()

    def retained(self):
        value = C.strict_json(C.private_bytes(self.state))
        C.require(set(value) == {'schema', 'principal', 'chain_id', 'key', 'body', 'request_digest', 'response'}
                  and value['schema'] == 'layerx.human.native-response.v2' and value['principal'] == self.principal
                  and value['chain_id'] == 125 and value['request_digest'] == self.digest(value['body'])
                  and re.fullmatch('[0-9a-f]{64}', value['body']['plan_digest'])
                  and re.fullmatch('[0-9a-f]{64}', value['body']['signed_digest']), 'exact private native retry record required')
        if value['response'] is not None:
            self.response(value['response'], value['body'])
        return value

    @staticmethod
    def digest(body):
        return hashlib.sha256(json.dumps(body, sort_keys=True, separators=(',', ':')).encode()).hexdigest()

    def save(self, value, initial=False):
        raw = json.dumps(value, sort_keys=True, separators=(',', ':')).encode()
        temporary = self.state.with_name(self.state.name + '.pending')
        fd = os.open(temporary, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
        try:
            with os.fdopen(fd, 'wb') as stream:
                stream.write(raw)
                stream.flush()
                os.fsync(stream.fileno())
            if initial:
                os.link(temporary, self.state)
                temporary.unlink()
            else:
                C.private_bytes(self.state)
                os.replace(temporary, self.state)
            directory = os.open(self.state.parent, os.O_RDONLY | os.O_DIRECTORY)
            try:
                os.fsync(directory)
            finally:
                os.close(directory)
        finally:
            if temporary.exists():
                temporary.unlink()

    @staticmethod
    def response(value, body):
        C.require(set(value) in ({'journey_id', 'plan_digest', 'state', 'state_copy_key'},
                                {'journey_id', 'plan_digest', 'state', 'state_copy_key', 'tx_hash'})
                  and re.fullmatch('jrn_[a-z0-9]+', value['journey_id'])
                  and value['plan_digest'] == body['plan_digest'] and value['state'] == 'processing'
                  and isinstance(value['state_copy_key'], str) and value['state_copy_key'],
                  'native submission must report submitted progress')
        C.require('tx_hash' not in value or re.fullmatch('0x[0-9a-f]{64}', value['tx_hash']), 'actual native transaction identity required')
        return value

    def submit(self, body, key):
        C.require(isinstance(key, str) and 0 < len(key) <= 256 and key.isascii()
                  and not any(c.isspace() for c in key), 'bounded exact retry key required')
        C.require(self.chain.node.call('eth_chainId', []) == '0x7d', 'native funding chain differs')
        C.require(set(body) in ({'plan_digest', 'signed_digest', 'bindings'},
                               {'plan_digest', 'signed_digest', 'bindings', 'native_access_id'})
                  and re.fullmatch('[0-9a-f]{64}', body['plan_digest'])
                  and re.fullmatch('[0-9a-f]{64}', body['signed_digest'])
                  and isinstance(body['bindings'], list) and body['bindings'], 'current signed native intent required')
        if self.state.exists() or self.state.is_symlink():
            retained = self.retained()
            C.require(retained['body'] == body and retained['key'] == key, 'changed native retry refused before transport')
        else:
            retained = {'schema': 'layerx.human.native-response.v2', 'principal': self.principal, 'chain_id': 125,
                        'key': key, 'body': body, 'request_digest': self.digest(body), 'response': None}
            self.save(retained, initial=True)
        value = self.response(self.client.call('POST', '/v1/intents/submit', body, key), body)
        C.require(retained['response'] is None or retained['response'] == value, 'native exact retry changed original response')
        if retained['response'] is None:
            retained['response'] = value
            self.save(retained)
        return value

    def recover(self):
        retained = self.retained()
        return self.submit(retained['body'], retained['key'])

    def owner_isolation(self):
        retained = self.retained()
        C.require(retained['response'] is not None, 'native response required before owner isolation')
        C.denied(lambda: self.foreign.call('POST', '/v1/intents/submit', retained['body'], retained['key']))
        C.denied(lambda: self.foreign.call('GET', '/v1/journeys/' + retained['response']['journey_id']))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--fixture', required=True)
    args = parser.parse_args()
    fixture = C.strict_json(C.private_bytes(args.fixture))
    owner, foreign, chain = C.current_clients(fixture)
    contract = FundingResponseContract(owner, foreign, chain, fixture['principal'], fixture['response_state'])
    result = contract.submit(fixture['submission'], fixture['idempotency_key'])
    C.require(contract.recover() == result, 'native response exact retry differs')
    contract.owner_isolation()
    print(json.dumps(result, sort_keys=True))
    return 0


if __name__ == '__main__':
    try:
        sys.exit(main())
    except (ValueError, OSError, KeyError, TypeError) as error:
        print('human-native-funding-response: refused: ' + str(error), file=sys.stderr)
        sys.exit(1)

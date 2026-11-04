#!/usr/bin/env python3
"""Current native transaction projections and recovered owner history."""
import argparse
from datetime import datetime
import importlib.util
import json
from pathlib import Path
import re
import sys

C = sys.modules.get('human_committed_projections')
if C is None:
    spec = importlib.util.spec_from_file_location('human_committed_projections', Path(__file__).with_name('human-canonical-projections.py'))
    C = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = C
    spec.loader.exec_module(C)


class JourneyProjections:
    def __init__(self, client, foreign, chain, owner):
        self.client, self.foreign, self.chain, self.owner = client, foreign, chain, owner

    def get(self, identity):
        C.require(re.fullmatch('jrn_[a-z0-9]+', identity), 'canonical journey identity required')
        value = self.client.call('GET', '/v1/journeys/' + identity)
        C.require(set(value) == {'journey_id', 'kind', 'state', 'state_copy_key', 'stages', 'evidence', 'started_at', 'updated_at'}
                  and value['journey_id'] == identity and value['kind'] == 'move'
                  and value['state'] in ('processing', 'still-checking', 'done', 'refused')
                  and value['state_copy_key'] == 'journey.state.' + value['state'],
                  'current native journey projection required')
        start = datetime.fromisoformat(value['started_at'].replace('Z', '+00:00'))
        updated = datetime.fromisoformat(value['updated_at'].replace('Z', '+00:00'))
        C.require(start.tzinfo is not None and updated.tzinfo is not None and start <= updated, 'authoritative journey timestamps required')
        C.require(value['stages'] and len(value['stages']) <= 64
                  and len({stage['stage_id'] for stage in value['stages']}) == len(value['stages']), 'bounded distinct transaction stages required')
        verified = []
        for stage in value['stages']:
            C.require(set(stage) == {'stage_id', 'copy_key', 'state', 'evidence', 'tx_hash', 'nonce'},
                      'closed native transaction stage required')
            C.require(stage['copy_key'] in ('journey.stage.submitted', 'journey.stage.confirmed', 'journey.stage.refused')
                      and re.fullmatch('0x[0-9a-f]{64}', stage['tx_hash'])
                      and type(stage['nonce']) is int and stage['nonce'] >= 0, 'native transaction stage required')
            if stage['state'] == 'done':
                C.require(stage['copy_key'] == 'journey.stage.confirmed' and stage['evidence'], 'committed native stage evidence required')
                for reference in stage['evidence']:
                    actual = C.committed_evidence(self.client, self.foreign, self.chain.node,
                                                  reference, stage['tx_hash'], self.owner)
                    transaction, _ = C.merged.evm.decode_signed(bytes.fromhex(actual['raw_transaction'][2:]))
                    C.require(transaction.nonce == stage['nonce'], 'committed journey nonce differs')
                    verified.append(actual)
            elif stage['state'] == 'processing':
                C.require(stage['copy_key'] == 'journey.stage.submitted'
                          and all(ref['verification'] == 'unverified' and ref['class'] == 'submission-record'
                                  for ref in stage['evidence']), 'submission cannot claim committed authority')
            else:
                C.require(stage['state'] == 'refused' and stage['copy_key'] == 'journey.stage.refused'
                          and value['state'] == 'refused', 'typed failed transaction projection required')
        C.require(all(ref in value['evidence'] for stage in value['stages'] for ref in stage['evidence']),
                  'common journey retains every transaction evidence reference')
        if value['state'] == 'done':
            C.require(all(stage['state'] == 'done' for stage in value['stages']) and verified,
                      'completion requires every native transaction committed')
        else:
            C.require(not all(stage['state'] == 'done' for stage in value['stages']), 'pending journey cannot conceal completion')
        C.denied(lambda: self.foreign.call('GET', '/v1/journeys/' + identity))
        return value

    def inventory(self):
        found, seen, pages = {}, set(), []
        cursor = 'cur_start'
        for _ in range(64):
            C.require(cursor not in seen, 'journey pagination must progress')
            seen.add(cursor)
            page = self.client.call('GET', '/v1/journeys/page/' + cursor)
            C.require(set(page) == {'journeys', 'next_cursor'} and len(page['journeys']) <= 50, 'bounded current history page required')
            pages.append(page)
            for row in page['journeys']:
                identity = row['journey_id']
                C.require(identity not in found and self.get(identity) == row, 'list and detail preserve one native journey')
                found[identity] = row
            cursor = page['next_cursor']
            C.require(re.fullmatch('cur_(?:end|jrn1_[a-f0-9]{64}_[1-9][0-9]*)', cursor), 'closed owner history cursor required')
            if cursor == 'cur_end':
                C.require(self.client.call('GET', '/v1/journeys') == pages[0], 'default history is the first canonical page')
                return found, pages
            C.denied(lambda: self.foreign.call('GET', '/v1/journeys/page/' + cursor), ('invalid-request',), (400,))
        raise ValueError('bounded native history did not terminate')

    def recovered(self, previous):
        current = self.inventory()
        C.require(current == previous, 'reopened native journey history or evidence differs')
        return current


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--fixture', required=True)
    args = parser.parse_args()
    fixture = C.strict_json(C.private_bytes(args.fixture))
    client, foreign, chain = C.current_clients(fixture)
    contract = JourneyProjections(client, foreign, chain, fixture['owner_address'])
    found, pages = contract.inventory()
    C.require(found and {'processing', 'still-checking', 'done'} <= {row['state'] for row in found.values()},
              'submitted ambiguous and committed native cases required')
    print(json.dumps({'journeys': len(found), 'pages': len(pages)}, sort_keys=True))
    return 0


if __name__ == '__main__':
    try:
        sys.exit(main())
    except (ValueError, OSError, KeyError, TypeError) as error:
        print('human-journey-projections: refused: ' + str(error), file=sys.stderr)
        sys.exit(1)

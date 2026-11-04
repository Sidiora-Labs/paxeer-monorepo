"""Task 22.33.860: current producers -> HTTP/RPC -> qualification consumers.

The peers replay the committed Go receipt corpus and Rust budget-proof export.
Human projection templates are the production serializer's committed golden files;
the loopback producer keeps its own durable submission/history store. Neither the
clients nor their receipt, transaction, commit, ABI or policy codecs are replaced.
"""
from contextlib import contextmanager
from copy import deepcopy
from http.client import RemoteDisconnected
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import base64
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import socket
import sys
import threading

import pytest

QUAL = Path(__file__).resolve().parents[1]
ROOT = QUAL.parents[2]
GOLD = ROOT / 'services/human/schema/human-api/golden'
VECTORS = ROOT / 'tests/vectors/budget-proof-v2'

TASK_RECORD = {
    'task': '22.33.860',
    'base_revision': 'aa5e0820b3652890f3812a220f4b91f4f8a45387',
    'findings': {
        'human-projection-finality': 'test_human_projection_finality',
        'human-security-node-material': 'test_human_security_node_material',
        'human-attestor-dual-finality': 'test_human_attestor_single_chain_intent',
        'human-deposit-hop': 'test_native_response_lost_acknowledgement_recovery',
        'human-journey-custody': 'test_native_journey_history_recovery',
        'approval-budget-checkpoint': 'test_approval_committed_budget_and_held_material',
        'funded-operator-destination': 'test_funded_operator_committed_destination',
        'migration-connected-destination': 'test_connected_migration_destination',
    },
    'baseline': 'Not run: explicit hf-build/hf-done ordering forbids a separate unchanged-tree verification; the declared test did not exist on the base revision.',
    'wiring_requests': [
        'Domain 16: tools/qualification/paxeer-x/ramp-source-settlement.py must adopt the current --transaction-hash receipt-verifier interface for its standalone gate; these two consumers override it here.',
        'Domain 16: tools/paxeer-x fixture producers must emit current genesis/receipt-key paths and chain/validator pins consumed by these qualification clients.',
        'Domain 10: services/agent/sdk/python/layerx_sdk/intent.py plan decoder must accept current EVM execution fields; this gate uses its current production submit encoder.',
    ],
}


def load(name, path):
    if name in sys.modules:
        return sys.modules[name]
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    sys.modules[name] = module
    spec.loader.exec_module(module)
    return module


C = load('human_committed_projections', QUAL / 'human-canonical-projections.py')
S = load('repair860_security', QUAL / 'human-security-prerequisites.py')
A = load('repair860_attestor', QUAL / 'human_attestor_intents.py')
D = load('repair860_response', QUAL / 'human_deposit_initial_response.py')
J = load('repair860_journey', QUAL / 'human_journey_projections.py')
P = load('repair860_approvals', QUAL / 'human_program_approvals.py')
F = load('committed_funded_operator', QUAL / 'migration-funded-operator.py')
M = load('repair860_migration', QUAL / 'migration-v2.py')
R = load('repair860_recorded_peer', QUAL / 'tests/test_repair_22_33_779.py')
from layerx_sdk.intent import IntentLegBinding, IntentMoney, SubmitPlanRequest, encode_submit_plan_request
from layerx_sdk.verifier import SequencerReceiptVerificationError

OWNER = 'did:layerx:' + '0a' * 32
NOW = 1_700_000_000_000


def golden(name):
    return json.loads((GOLD / (name + '.json')).read_text())['body']['result']


def put(path, raw):
    path.write_bytes(raw)
    path.chmod(0o600)
    return path


def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':')).encode()


def submission():
    plan = golden('intent.plan.response')
    requirement, leg = plan['signing_requirements'][0], plan['legs'][0]
    binding = IntentLegBinding(0, requirement['action_key'], 'did:layerx:alice', requirement['authority'],
                               'owner', leg['nonce'], 1_700_000_000, 1_900_000_000,
                               IntentMoney(int(leg['fee']['amount']), leg['fee']['currency']))
    # The real SDK encoder supplies the exact current JSON types on the wire.
    return encode_submit_plan_request(SubmitPlanRequest(plan['plan_digest'], '4b' * 32, (binding,)))


class RecordedNode(R.Peer):
    def __init__(self, path):
        super().__init__(path)
        corpus = json.loads((ROOT / 'sdk/platform/conformance/fixtures/sequencer-receipt-v1.json').read_text())
        self.transfer = next(row for row in corpus['cases'] if row['id'] == 'V1-pax-transfer')
        reference = self.transfer['receiptJson']['txHash']
        self.records[reference] = self.transfer
        row = self.execution(reference)
        self.data['blocks'][row['blockNumber']] = {'number': row['blockNumber'], 'hash': row['blockHash'],
            'stateRoot': '0x' + 'ab' * 32, 'transactions': [reference]}
        self.persist()


class BudgetNode(R.Peer):
    def __init__(self, path):
        self.answers = json.loads((VECTORS / 'node-answers.json').read_text())
        self.head = 8
        super().__init__(path)

    def dispatch(self, role, path, document, headers):
        assert headers.get('Authorization') == R.AUTH
        assert headers.get_content_type() == 'application/json'
        assert set(document) == {'jsonrpc', 'id', 'method', 'params'} and document['jsonrpc'] == '2.0'
        method, params = document['method'], document['params']
        if method == 'eth_blockNumber':
            result = hex(self.head)
        else:
            rows = [row for row in self.answers if row['method'] == method and row['params'] == params]
            if not rows:
                return super().dispatch(role, path, document, headers)
            assert len(rows) == 1
            assert role == ('gateway' if method.startswith('px_') else 'node')
            result = deepcopy(rows[0]['result'])
        answer = {'jsonrpc': '2.0', 'id': document['id'], 'result': result}
        if self.response_damage:
            self.response_damage(role, method, answer)
        return answer

    def client(self):
        client = super().client()
        validators = next(row['result']['validators'] for row in self.answers if row['method'] == 'px_getValidators')
        client.validator_pins = [{'address': row['address'],
            'public_key': base64.b64decode(row['pubKey']['value'], validate=True).hex(),
            'voting_power': int(row['votingPower'])} for row in validators]
        return client


class HumanProducer:
    """Loopback Human peer with real envelopes and a reopened durable owner store."""
    def __init__(self, path, node):
        self.path, self.node, self.port = path, node, 0
        self.failures, self.requests = [], []
        self.lose_ack, self.damage = False, None
        self.data = {'evidence': {}, 'journeys': {}, 'submissions': {}, 'operations': 0, 'approvals': {}}
        self.plan = golden('intent.plan.response')
        self.body = submission()
        self.persist()
        self.start()

    @property
    def url(self):
        return 'http://127.0.0.1:' + str(self.port)

    def clients(self):
        return tuple(C.HumanClient(self.url, self.url, {'assertion': who, 'cookies': {
            '__Host-layerx_access': who, '__Host-layerx_csrf': 'csrf-' + who}}) for who in ('owner', 'foreign'))

    def persist(self):
        put(self.path, canonical(self.data))

    def export(self, raw, kind, verification='unverified', identity=None, **extra):
        identity = identity or 'evd_' + hashlib.sha256(raw).hexdigest()
        ref = {'evidence_id': identity, 'class': kind, 'verification': verification}
        self.data['evidence'][identity] = {**ref, 'content_type': 'application/json',
            'bytes_base64': base64.b64encode(raw).decode(), **extra}
        self.persist()
        return ref

    def committed_receipt(self):
        material = golden('evidence.get.response')
        # The signed envelope is the real producer recording, unchanged. The
        # peer attaches block inclusion returned by the node's current producer.
        material['verification'], material['rung'] = 'checkpoint-finalised', 'committed'
        self.data['evidence'][material['evidence_id']] = material
        self.persist()
        return {key: material[key] for key in ('evidence_id', 'class', 'verification')}

    def journey(self, identity, state, reference):
        record = self.node.transfer['receiptJson']
        transaction, _ = C.merged.evm.decode_signed(bytes.fromhex(self.node.transfer['rawTransaction'][2:]))
        value = golden('journey.get.response')
        value.update(journey_id=identity, state=state, state_copy_key='journey.state.' + state)
        stage = value['stages'][0]
        done = state == 'done'
        stage.update(stage_id='stg_' + identity + '_0', state='done' if done else 'processing',
                     copy_key='journey.stage.confirmed' if done else 'journey.stage.submitted',
                     tx_hash=record['txHash'], nonce=transaction.nonce, evidence=[reference])
        value['evidence'] = [reference]
        self.data['journeys'][identity] = value
        self.persist()
        return value

    def budget_approval(self):
        raw = (VECTORS / 'export.json').read_bytes()
        proof = json.loads(raw)
        reference = self.export(raw, 'checkpoint-proof', 'checkpoint-finalised')
        held = canonical({'held': 'immutable Programs call', 'budget': proof['budget_state']['budget_id']})
        carrier = self.export(held, 'approval-hold')
        unsigned = self.export(canonical({'canonical': 'unsigned current Programs operation'}), 'approval-hold')
        reservation = self.export(canonical({'reservation': proof['budget_state']['budget_id'],
                                           'held_digest': carrier['evidence_id'][4:]}), 'local-journey-state')
        identity = 'apr_' + '23' * 32
        budget = {'approval_id': identity, 'held_digest': carrier['evidence_id'][4:],
            'budget_id': proof['budget_state']['budget_id'], 'source_account': proof['accounts']['source'],
            'asset_id': proof['budget_state']['asset'], 'remaining': proof['budget_state']['remaining'],
            'observed_head_sequence': 8, 'age_sequences': 0, 'maximum_age_sequences': 2,
            'checkpoint_digest': proof['head']['block_hash'], 'verification': 'checkpoint-finalised',
            'within_bound': True, 'proof_digest': reference['evidence_id'][4:], 'evidence': [reference]}
        detail = golden('approval.program.get.response')
        detail.update(approval_id=identity, held_digest=carrier['evidence_id'][4:], agent_id='agt_' + '25' * 32, budget=budget)
        material = golden('approval.program.material.response')
        material.update(held_digest=detail['held_digest'], provenance='local-owned',
                        canonical_unsigned=unsigned, immutable_carrier=carrier, budget_reservation=reservation)
        self.data['approvals'][identity] = {'detail': detail, 'material': material, 'budget': budget}
        self.persist()
        trust = {'held_digest': detail['held_digest'], 'agent_id': detail['agent_id'], 'tenant': 'tenant',
                 'principal': 'operator', 'owner': OWNER, 'now_ms': NOW}
        return identity, trust, reference

    def dispatch(self, method, path, body, headers):
        who = headers.get('Authorization', '').removeprefix('Bearer ')
        assert who in ('owner', 'foreign')
        assert headers.get('Origin') == self.url
        assert headers.get('X-LayerX-CSRF') == 'csrf-' + who
        assert headers.get('Cookie') == '__Host-layerx_access=' + who + '; __Host-layerx_csrf=csrf-' + who
        assert headers.get('Accept') == 'application/json'
        if method == 'POST':
            assert headers.get_content_type() == 'application/json'
        trace = headers['X-LayerX-Trace']
        assert len(trace) == 36 and trace.startswith('trc_')
        self.requests.append((who, method, path, deepcopy(body), headers.get('Idempotency-Key')))

        def refuse(code='forbidden', status=403):
            return status, {'ok': False, 'error': {'code': code, 'copy_key': 'error.' + code, 'retriability': 'final'}, 'trace': trace}

        if who != 'owner':
            return refuse('invalid-request', 400) if '/page/cur_jrn1_' in path else refuse()
        if method == 'POST' and path == '/v1/intents/plan':
            assert body == json.loads((GOLD / 'intent.plan.request.json').read_text())['body']
            result = self.plan
        elif method == 'POST' and path == '/v1/intents/submit':
            key = headers.get('Idempotency-Key')
            assert key
            if body != self.body:
                return refuse('conflict', 409)
            if key in self.data['submissions']:
                retained = self.data['submissions'][key]
                if retained['body'] != body:
                    return refuse('conflict', 409)
                result = retained['result']
            else:
                result = {'journey_id': 'jrn_native1', 'plan_digest': body['plan_digest'],
                          'state': 'processing', 'state_copy_key': 'journey.state.processing'}
                self.data['submissions'][key] = {'body': body, 'result': result}
                self.data['operations'] += 1
                self.persist()
                if self.lose_ack:
                    self.lose_ack = False
                    return None
        elif method == 'GET' and path.startswith('/v1/evidence/'):
            result = self.data['evidence'].get(path.rsplit('/', 1)[1])
        elif method == 'GET' and path.startswith('/v1/program-approvals/'):
            parts = path.split('/')
            item = self.data['approvals'].get(parts[3], {})
            result = item.get(parts[4] if len(parts) > 4 else 'detail')
        elif method == 'GET' and (path == '/v1/journeys' or path.startswith('/v1/journeys/page/')):
            cursor = 'cur_start' if path == '/v1/journeys' else path.rsplit('/', 1)[1]
            offset = 0 if cursor == 'cur_start' else int(cursor.rsplit('_', 1)[1])
            rows = [self.data['journeys'][identity] for identity in sorted(self.data['journeys'])]
            result = {'journeys': rows[offset:offset + 50],
                      'next_cursor': 'cur_end' if offset + 50 >= len(rows) else 'cur_jrn1_' + 'a4' * 32 + '_' + str(offset + 50)}
        elif method == 'GET' and path.startswith('/v1/journeys/'):
            result = self.data['journeys'].get(path.rsplit('/', 1)[1])
            # The durable initial response can be inspected without inventing a
            # completion record while an acknowledgement is being recovered.
            if result is None and path.endswith('/jrn_native1'):
                result = next(iter(self.data['submissions'].values()))['result']
        else:
            raise AssertionError('unexpected Human operation: ' + path)
        if result is None:
            return refuse('not-found', 404)
        envelope = {'ok': True, 'result': deepcopy(result), 'trace': trace}
        if self.damage:
            self.damage(path, envelope)
        return 200, envelope

    def start(self):
        self.data = json.loads(self.path.read_text())
        peer = self

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *_):
                pass

            def handle_request(self):
                try:
                    body = None
                    if self.command == 'POST':
                        body = C.strict_json(self.rfile.read(int(self.headers['Content-Length'])))
                    answer = peer.dispatch(self.command, self.path, body, self.headers)
                    if answer is None:
                        self.connection.shutdown(socket.SHUT_RDWR)
                        self.connection.close()
                        return
                    status, envelope = answer
                    raw = canonical(envelope)
                    self.send_response(status)
                    self.send_header('Content-Type', 'application/json')
                    self.send_header('Cache-Control', 'no-store')
                    self.send_header('X-LayerX-Trace', self.headers['X-LayerX-Trace'])
                    self.send_header('Content-Length', str(len(raw)))
                    self.end_headers()
                    self.wfile.write(raw)
                except BaseException as error:
                    peer.failures.append(error)
                    self.send_error(500, str(error))

            do_GET = handle_request
            do_POST = handle_request

        self.server = ThreadingHTTPServer(('127.0.0.1', self.port), Handler)
        self.server.daemon_threads = True
        self.port = self.server.server_port
        self.thread = threading.Thread(target=self.server.serve_forever, kwargs={'poll_interval': .01}, daemon=True)
        self.thread.start()

    def stop(self):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(2)

    def restart(self):
        self.persist()
        self.stop()
        self.data = None
        self.start()


@contextmanager
def peers(tmp_path, budget=False):
    node = (BudgetNode if budget else RecordedNode)(tmp_path / 'node.json')
    human = HumanProducer(tmp_path / 'human.json', node)
    try:
        yield node, human
        assert node.failures == []
        assert human.failures == []
    finally:
        human.stop()
        node.stop()


def test_human_projection_finality(tmp_path):
    with peers(tmp_path) as (node, human):
        owner, foreign = human.clients()
        reference = human.committed_receipt()
        transaction = node.transfer['receiptJson']
        actual = C.committed_evidence(owner, foreign, node.node(), reference, transaction['txHash'], transaction['from'])
        assert actual['receipt_digest'] == node.transfer['receiptDigest'][2:]
        assert actual['block_number'] == transaction['height']
        human.restart()
        node.restart()
        assert C.committed_evidence(*human.clients(), node.node(), reference, transaction['txHash'], transaction['from']) == actual
        with pytest.raises(ValueError, match='another tenant'):
            C.committed_evidence(*human.clients(), node.node(), reference, transaction['txHash'], '0x' + '18' * 20)
        human.data['evidence'][reference['evidence_id']]['rung'] = 'receipted'
        with pytest.raises(ValueError, match='committed receipt'):
            C.committed_evidence(*human.clients(), node.node(), reference, transaction['txHash'], transaction['from'])
        human.data['evidence'][reference['evidence_id']]['rung'] = 'committed'
        original = human.data['evidence'][reference['evidence_id']]['bytes_base64']
        changed = json.loads(base64.b64decode(original))
        changed['receiptDigest'] = '0x' + '91' * 32
        human.data['evidence'][reference['evidence_id']]['bytes_base64'] = base64.b64encode(canonical(changed)).decode()
        with pytest.raises(ValueError, match='portable digest'):
            C.committed_evidence(*human.clients(), node.node(), reference, transaction['txHash'], transaction['from'])
        human.data['evidence'][reference['evidence_id']]['bytes_base64'] = original

        def corrupt_block(role, method, answer):
            if method == 'eth_getBlockByNumber':
                answer['result']['hash'] = '0x' + '21' * 32
        node.response_damage = corrupt_block
        with pytest.raises(ValueError, match='committed node block'):
            C.committed_evidence(*human.clients(), node.node(), reference, transaction['txHash'], transaction['from'])


def test_human_attestor_single_chain_intent(tmp_path):
    with peers(tmp_path) as (node, human):
        request = json.loads((GOLD / 'intent.plan.request.json').read_text())['body']
        case = {'request': request, 'expected_plan': deepcopy(human.plan), 'submission': human.body,
                'expected_submission': deepcopy(human.body), 'idempotency_key': 'signed-native'}
        result = A.qualify_intent(*human.clients(), node.client(), case)
        assert result['plan_digest'] == human.plan['plan_digest']
        assert human.data['operations'] == 1
        human.restart()
        node.restart()
        assert A.qualify_intent(*human.clients(), node.client(), case) == result
        assert human.data['operations'] == 1
        for field, changed in (('chain_id', 713714), ('mechanism', 'custody-deposit'), ('calldata', '0x00000000')):
            plan = deepcopy(human.plan)
            plan['legs'][0][field] = changed
            with pytest.raises(ValueError):
                A.qualify_plan(node.client(), plan, plan)
        altered = deepcopy(case)
        altered['submission']['bindings'][0]['account_sequence'] += 1
        altered['expected_submission'] = deepcopy(altered['submission'])
        with pytest.raises(ValueError, match='leg binding'):
            A.qualify_intent(*human.clients(), node.client(), altered)
        assert any(row[0] == 'foreign' and row[2] == '/v1/intents/submit' for row in human.requests)
        assert any(row[2] and row[2]['method'] == 'px_getCapabilities' for row in node.requests)


def test_native_response_lost_acknowledgement_recovery(tmp_path):
    with peers(tmp_path) as (node, human):
        owner, foreign = human.clients()
        state = tmp_path / 'retry.json'
        contract = D.FundingResponseContract(owner, foreign, node.client(), 'owner', state)
        human.lose_ack = True
        with pytest.raises(RemoteDisconnected):
            contract.submit(human.body, 'lost-ack')
        retained = contract.retained()
        assert retained['body'] == human.body and retained['response'] is None
        assert human.data['operations'] == 1
        del contract
        human.restart()
        node.restart()
        reopened = D.FundingResponseContract(*human.clients(), node.client(), 'owner', state)
        result = reopened.recover()
        assert result['state'] == 'processing' and result['journey_id'] == 'jrn_native1'
        assert reopened.recover() == result and human.data['operations'] == 1
        reopened.owner_isolation()
        changed = deepcopy(human.body)
        changed['signed_digest'] = '5c' * 32
        requests = len(human.requests)
        with pytest.raises(ValueError, match='changed native retry'):
            reopened.submit(changed, 'lost-ack')
        assert len(human.requests) == requests
        with pytest.raises(ValueError, match='private native retry'):
            D.FundingResponseContract(*human.clients(), node.client(), 'foreign', state)
        state.chmod(0o644)
        with pytest.raises(ValueError, match='private regular owner'):
            reopened.recover()
        state.chmod(0o600)
        alias = tmp_path / 'alias'
        alias.symlink_to(state)
        with pytest.raises(OSError):
            D.FundingResponseContract(*human.clients(), node.client(), 'owner', alias)


def test_native_journey_history_recovery(tmp_path):
    with peers(tmp_path) as (node, human):
        receipt = human.committed_receipt()
        pending = human.export(canonical({'transaction_hash': node.transfer['receiptJson']['txHash'],
                                          'state': 'submitted'}), 'submission-record')
        for number in range(51):
            state = 'done' if number == 0 else 'still-checking' if number == 1 else 'processing'
            human.journey('jrn_history' + str(number), state, receipt if state == 'done' else pending)
        contract = J.JourneyProjections(*human.clients(), node.client(), node.transfer['receiptJson']['from'])
        before = contract.inventory()
        assert len(before[0]) == 51 and len(before[1]) == 2
        assert {row['state'] for row in before[0].values()} == {'done', 'still-checking', 'processing'}
        human.restart()
        node.restart()
        del contract
        reopened = J.JourneyProjections(*human.clients(), node.client(), node.transfer['receiptJson']['from'])
        assert reopened.recovered(before) == before
        human.data['journeys']['jrn_history1']['stages'][0]['copy_key'] = 'deposit.stage.wallet'
        with pytest.raises(ValueError, match='native transaction stage'):
            reopened.get('jrn_history1')
        human.data['journeys']['jrn_history1'] = deepcopy(before[0]['jrn_history1'])
        human.data['journeys']['jrn_history1']['state'] = 'done'
        human.data['journeys']['jrn_history1']['state_copy_key'] = 'journey.state.done'
        with pytest.raises(ValueError, match='every native transaction committed'):
            reopened.get('jrn_history1')


def test_approval_committed_budget_and_held_material(tmp_path):
    with peers(tmp_path, budget=True) as (node, human):
        identity, trust, reference = human.budget_approval()
        actual = P.qualify_budget(*human.clients(), node.client(), identity, trust)
        assert actual['remaining'] == 77 and actual['height'] == 8 and actual['age_blocks'] == 0
        assert actual['proof_digest'] == reference['evidence_id'][4:]
        human.restart()
        node.restart()
        assert P.qualify_budget(*human.clients(), node.client(), identity, trust) == actual
        node.head = 11
        with pytest.raises(ValueError, match='freshness'):
            P.qualify_budget(*human.clients(), node.client(), identity, trust)
        node.head = 8
        altered = dict(trust, now_ms=NOW + 60_001)
        with pytest.raises(ValueError, match='freshness'):
            P.qualify_budget(*human.clients(), node.client(), identity, altered)
        altered = dict(trust, owner='did:layerx:' + '15' * 32)
        with pytest.raises(ValueError, match='owner account join'):
            P.qualify_budget(*human.clients(), node.client(), identity, altered)
        altered = dict(trust, held_digest='16' * 32)
        with pytest.raises(ValueError, match='held digest'):
            P.qualify_budget(*human.clients(), node.client(), identity, altered)
        row = human.data['approvals'][identity]
        row['material']['budget_reservation']['verification'] = 'checkpoint-finalised'
        with pytest.raises(ValueError, match='held material'):
            P.qualify_budget(*human.clients(), node.client(), identity, trust)
        row['material']['budget_reservation']['verification'] = 'unverified'
        row['budget']['remaining'] = '78'
        row['detail']['budget']['remaining'] = '78'
        with pytest.raises(ValueError, match='amount or block-based freshness'):
            P.qualify_budget(*human.clients(), node.client(), identity, trust)


def test_budget_export_replays_real_bank_abi_and_commit(tmp_path):
    with peers(tmp_path, budget=True) as (node, human):
        identity, trust, reference = human.budget_approval()
        material, raw = C.evidence(*human.clients(), reference)
        actual = C.committed_budget(raw, node.client(), 'tenant', 'operator', OWNER, NOW)
        assert actual['remaining'] == 77
        assert material['verification'] == 'checkpoint-finalised'
        for section, field, changed in (('budget_state', 'remaining', '78'), ('budget_state', 'observed_head_sequence', 9),
                                         ('finality', 'app_hash', '27' * 32)):
            proof = json.loads(raw)
            proof[section][field] = changed
            with pytest.raises(ValueError):
                C.committed_budget(canonical(proof), node.client(), 'tenant', 'operator', OWNER, NOW)
        proof = json.loads(raw)
        proof['proofs']['remaining_call']['data'] = '00000000' + proof['proofs']['remaining_call']['data'][8:]
        with pytest.raises(ValueError, match='selector'):
            C.committed_budget(canonical(proof), node.client(), 'tenant', 'operator', OWNER, NOW)

        def corrupt_commit(role, method, answer):
            if method == 'px_getCommit':
                answer['result']['signedHeader']['commit']['signatures'][0]['signature'] = base64.b64encode(bytes(64)).decode()
        node.response_damage = corrupt_commit
        with pytest.raises(ValueError):
            C.committed_budget(raw, node.client(), 'tenant', 'operator', OWNER, NOW)


def test_human_security_node_material(tmp_path):
    producer = load('repair860_policy_producer', ROOT / 'services/platform/hosted/human/test_repair_22_31_348.py')
    private = tmp_path / 'policy-producer'
    private.mkdir(mode=0o700)
    with producer.merged_environment(private) as environment:
        fixtures = ROOT / 'services/platform/hosted/node/tests/fixtures/repair_22_31_355'
        configuration = json.loads((fixtures / 'native_configuration.json').read_text())
        genesis = put(tmp_path / 'genesis.json', (fixtures / 'genesis.json').read_bytes())
        seed = put(tmp_path / 'receipt.seed', configuration['receipt_seed'].encode())
        runtime = tmp_path / 'runtime'
        runtime.mkdir(mode=0o700)
        args = (genesis, seed, environment.bundle, 125, configuration['chain_id'], os.geteuid())
        contract = S.SecurityPrerequisites(*args)
        actual = contract.project(runtime)
        assert actual['receipt_public_key'] == configuration['receipt_public_key']
        assert actual['policy']['records'] == 2 and actual['validators']
        del contract
        assert S.SecurityPrerequisites(*args).project(runtime) == actual
        with pytest.raises(ValueError, match='private regular owner'):
            S.SecurityPrerequisites(*args[:-1], os.geteuid() + 1).binding()
        seed.chmod(0o644)
        with pytest.raises(ValueError, match='private regular owner'):
            S.SecurityPrerequisites(*args).binding()
        seed.chmod(0o600)
        link = tmp_path / 'receipt-link'
        link.symlink_to(seed)
        with pytest.raises(OSError):
            S.SecurityPrerequisites(genesis, link, *args[2:]).binding()
        hardlink = tmp_path / 'receipt-hardlink'
        os.link(seed, hardlink)
        with pytest.raises(ValueError, match='private regular owner'):
            S.SecurityPrerequisites(*args).binding()
        hardlink.unlink()
        put(seed, b'25' * 32)
        with pytest.raises(ValueError, match='uniquely pinned'):
            S.SecurityPrerequisites(*args).binding()
        put(seed, configuration['receipt_seed'].encode())
        original = genesis.read_bytes()
        changed = json.loads(original)
        changed['chain_id'] = 'different-chain'
        put(genesis, canonical(changed))
        with pytest.raises(ValueError, match='native chain genesis'):
            S.SecurityPrerequisites(*args).binding()
        put(genesis, original)
        policy = environment.bundle / 'policy.json'
        old_policy = policy.read_bytes()
        put(policy, b'{}')
        with pytest.raises((ValueError, KeyError)):
            S.SecurityPrerequisites(*args).binding()
        put(policy, old_policy)
        retained = runtime / 'native-binding.json'
        altered = json.loads(retained.read_bytes())
        altered['receipt_key_id'] = '40' * 32
        put(retained, canonical(altered))
        with pytest.raises(ValueError, match='retained native security binding'):
            S.SecurityPrerequisites(*args).project(runtime)


def payment_binding(node):
    record = node.payment_record
    transaction, _ = C.merged.evm.decode_signed(bytes.fromhex(record['rawTransaction'][2:]))
    fields = C.merged.decode_abi(['bytes32', 'bytes32', 'bytes32', 'uint128', 'uint64', 'bytes32'],
                                '0x' + transaction.data[4:].hex())
    return {'operator_address': record['receiptJson']['from'], 'from_account': fields[0].hex(),
            'to_account': fields[1].hex(), 'asset': fields[2].hex(), 'amount': str(fields[3]),
            'expires_at_ms': fields[4], 'order_digest': fields[5].hex()}


def source_claim(binding, state='done', proof=None):
    proof = ownership_capture('solana:' + '68' * 32)[0] if proof is None else proof
    return {'version': F.ramp.VERSION, 'order_digest': list(bytes.fromhex(binding['order_digest'])),
            'source_evidence_digest': list(hashlib.sha256(proof).digest()),
            'source_claim_id': None if state == 'source_pending' else list(hashlib.sha256(b'loopback-source-claim\0' + proof).digest()),
            'state': state}


def test_funded_operator_committed_destination(tmp_path):
    with peers(tmp_path) as (node, human):
        consumer = F.DestinationReceiptConsumer(node.node())
        binding = payment_binding(node)
        reference = node.payment_record['receiptJson']['txHash']
        source = source_claim(binding)
        pending = consumer.observe(source_claim(binding, 'source_pending'), reference, binding)
        assert pending['destination'] is None and pending['source']['source_claim_id'] is None
        result = consumer.observe(source, reference, binding)
        assert result['source'] == source and result['destination']['rung'] == 'committed'
        assert result['destination']['transaction_hash'] == reference
        assert result['destination']['receipt_digest'] == node.payment_record['receiptDigest'][2:]
        # Both overridden process adapters consume this same current output shape.
        assert F.verify_destination_record(result['destination'], binding['order_digest'], reference) == result['destination']
        assert F.FundedOperatorContract.receipt is not F.ramp.Contract.receipt
        assert M.MigrationContract.receipt is not M.ramp.Contract.receipt
        node.restart()
        del consumer
        reopened = F.DestinationReceiptConsumer(node.node())
        assert reopened.observe(source, reference, binding) == result
        assert reopened.observe(source, reference, binding) == result
        for name, value in (('to_account', '44' * 32), ('order_digest', '45' * 32), ('amount', '1'),
                            ('operator_address', '0x' + '46' * 20)):
            with pytest.raises(ValueError):
                reopened.verify(reference, dict(binding, **{name: value}))
        with pytest.raises(F.ramp.Refusal):
            reopened.observe(dict(source, source_claim_id=None), reference, binding)
        forged = dict(result['destination'], rung='receipted', maintained_batch=True)
        with pytest.raises(ValueError, match='committed native destination'):
            F.verify_destination_record(forged, binding['order_digest'], reference)


def ownership_capture(source_network):
    """Wallet-signed peer capture in the source verifier's fixed binary format."""
    from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
    from cryptography.hazmat.primitives.serialization import Encoding, PublicFormat
    identity = bytes.fromhex('65' * 32)
    lifetime = (1_700_000_000).to_bytes(8, 'big') + (1_700_000_600).to_bytes(8, 'big')
    nonce = bytes.fromhex('69' * 32)
    if source_network.startswith('ethereum:'):
        wallet = load('repair860_source_wallet', ROOT / 'services/platform/hosted/paxeer/evm.py')
        address = wallet.address_of(17)
        fields = (1).to_bytes(8, 'big') + address + identity + lifetime + nonce
        message = b'LayerX Ethereum migration ownership v1\n' + fields
        digest = C.merged.CHAIN.keccak256(b'\x19Ethereum Signed Message:\n' + str(len(message)).encode() + message)
        parity, r, scalar = wallet.sign_digest(17, digest)
        signature = r.to_bytes(32, 'big') + scalar.to_bytes(32, 'big') + bytes([27 + parity])
        assert C.merged.evm._recover_signer(digest, signature).lower() == '0x' + address.hex()
        return b'LXM/ETH/OWNERSHIP/1\0' + fields + signature, address.hex()
    signer = Ed25519PrivateKey.from_private_bytes(bytes.fromhex('70' * 32))
    address = signer.public_key().public_bytes(Encoding.Raw, PublicFormat.Raw)
    fields = bytes.fromhex(source_network.split(':', 1)[1]) + address + identity + lifetime + nonce
    message = b'LayerX Solana migration ownership v1\n' + fields
    signature = signer.sign(message)
    signer.public_key().verify(signature, message)
    return b'LXM/SOL/OWNERSHIP/1\0' + fields + signature, address.hex()


class MappingProducer(HumanProducer):
    """Authenticated ownership peer with a durable mapping journal."""
    def __init__(self, path, node, proof, mapping, journal):
        self.proof, self.mapping, self.journal = proof, mapping, journal
        super().__init__(path, node)

    def dispatch(self, method, path, body, headers):
        assert method == 'POST' and path == '/v2/migration/accounts'
        assert headers.get_content_type() == 'application/json' and headers.get('Idempotency-Key')
        if headers.get('Authorization') != 'Bearer owner':
            return 403, {'ok': False, 'error': {'code': 'forbidden'}, 'trace': 'trc_mapping'}
        if (set(body) != {'chain', 'source_evidence'}
                or body['chain'] != ('ethereum' if self.mapping['source_network'].startswith('ethereum:') else 'solana')
                or body['source_evidence'] != base64.b64encode(self.proof).decode()):
            return 422, {'ok': False, 'error': {'code': 'invalid-request'}, 'trace': 'trc_mapping'}
        if 'mapping' not in self.data:
            self.data['mapping'] = self.mapping
            put(self.journal / 'ownership.record', canonical(self.mapping))
            self.persist()
        assert self.data['mapping'] == self.mapping
        return 200, {'ok': True, 'result': self.data['mapping'], 'trace': 'trc_mapping'}


def mapping_client(peer, journal):
    # Only the artifact-launching constructor is omitted: every mapping method,
    # private-file check and HTTP transport below is the actual contract's code.
    client = M.MigrationContract.__new__(M.MigrationContract)
    client.auth, client.context, client.interop_base = {'customer': 'owner', 'other_customer': 'foreign'}, None, peer.url
    client.fixture = {'expected_did': '65' * 32}
    client.profile = {'ethereum': {'chain_id': 1}, 'solana': {'genesis_hash': [0x68] * 32}}
    client.mapping_path, client.count = journal, 0
    return client


@pytest.mark.parametrize('source_network,address_size', [('ethereum:1', 20), ('solana:' + '68' * 32, 32)])
def test_connected_migration_destination(tmp_path, source_network, address_size):
    with peers(tmp_path) as (node, human):
        binding = payment_binding(node)
        proof, source_address = ownership_capture(source_network)
        mapping = {'version': 'account-mapping-v2', 'principal': 'migration-owner',
            'source_network': source_network, 'source_address': source_address,
            'layerx_identity': list(bytes.fromhex('65' * 32)), 'evm_address': list(bytes.fromhex('66' * 20)),
            'paxeer_block_number': 999, 'paxeer_block_hash': [67] * 32,
            'evidence_digest': list(hashlib.sha256(proof).digest()), 'provenance': 'paxeer-binding', 'layerx_receipt': False}
        journal = tmp_path / 'mapping-journal'
        journal.mkdir(mode=0o700)
        peer = MappingProducer(tmp_path / 'mapping-peer.json', node, proof, mapping, journal)
        try:
            contract = mapping_client(peer, journal)
            case = {'chain': 'ethereum' if source_network.startswith('ethereum:') else 'solana',
                    'evidence_file': str(put(tmp_path / 'ownership.bin', proof)),
                    'expected_source_address': source_address, 'expected_evm_address': '66' * 20}
            request, retained = contract.mapping_request(case)
            assert retained == proof and base64.b64decode(request['source_evidence'], validate=True) == proof
            contract.mapping_refusal('foreign-owner', request, 'other_customer')
            status, response = contract.mapping_post(request, key='mapping-original')
            assert status == 200 and response['ok'] is True
            contract.assert_mapping(response['result'], case, retained)
            snapshot = contract.snapshot()
            assert snapshot and 'ownership.record' in snapshot
            peer.restart()
            del contract
            recovered_contract = mapping_client(peer, journal)
            status, replay = recovered_contract.mapping_post(request, key='mapping-original')
            assert status == 200 and replay == response
            recovered_contract.assert_mapping(replay['result'], case, retained)
            assert recovered_contract.snapshot() == snapshot
            recovered_contract.mapping_refusal('wrong-source-proof', dict(request, source_evidence=base64.b64encode(proof[:-1]).decode()))
            assert recovered_contract.snapshot() == snapshot
            assert peer.failures == []
        finally:
            peer.stop()
        binding.update({name: mapping[name] for name in ('principal', 'source_network', 'source_address',
                                                         'layerx_identity', 'evm_address')})
        source = source_claim(binding, proof=proof)
        reference = node.payment_record['receiptJson']['txHash']
        consumer = M.ConnectedDestinationConsumer(node.node())
        actual = consumer.connected(mapping, source, reference, binding)
        assert actual['source']['source_claim_id'] == source['source_claim_id']
        assert actual['destination']['chain_id'] == 125 and actual['destination']['rung'] == 'committed'
        node.restart()
        del consumer
        reopened = M.ConnectedDestinationConsumer(node.node())
        assert reopened.connected(mapping, source, reference, binding) == actual
        for name, changed in (('principal', 'another-owner'), ('source_address', '77' * address_size),
                               ('source_network', 'ethereum:9'), ('evm_address', [78] * 20),
                               ('layerx_identity', [79] * 32), ('layerx_receipt', True)):
            with pytest.raises(ValueError, match='connected source ownership'):
                reopened.connected(dict(mapping, **{name: changed}), source, reference, binding)
        pending = reopened.connected(mapping, source_claim(binding, 'layerx_pending'), reference, binding)
        assert pending['destination'] is None

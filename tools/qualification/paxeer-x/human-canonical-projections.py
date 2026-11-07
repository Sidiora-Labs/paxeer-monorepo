#!/usr/bin/env python3
import argparse
import importlib.util
import os
from pathlib import Path
import ssl
import subprocess
import sys
import time

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / 'tools/paxeer-x'))
from candidate import SERVICE_CATALOGUE

spec = importlib.util.spec_from_file_location('human_boundary', ROOT / 'tools/qualification/paxeer-x/human-api-boundary.py')
B = importlib.util.module_from_spec(spec)
spec.loader.exec_module(B)

# The Human wire vocabulary retains checkpoint-finalised for committed blocks.
# Native evidence, rather than that label, establishes the achieved authority.
import base64
from copy import deepcopy
from datetime import datetime
import hashlib
import http.client
import json
import re
import secrets
import stat
from urllib.parse import urlsplit

sys.path.insert(0, str(Path(__file__).parent))
import merged_qualification as merged
from cryptography.exceptions import InvalidSignature
from layerx_sdk.committed import verify_commit

MAXIMUM = 4_194_304
PAY = '0x' + '00' * 18 + '101b'


def require(value, reason):
    if not value:
        raise ValueError(reason)


def strict_json(raw):
    def pairs(items):
        result = {}
        for key, value in items:
            require(key not in result, 'duplicate JSON member')
            result[key] = value
        return result
    return json.loads(raw, object_pairs_hook=pairs)


def private_bytes(path, maximum=MAXIMUM, uid=None):
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    with os.fdopen(fd, 'rb') as stream:
        info = os.fstat(stream.fileno())
        require(stat.S_ISREG(info.st_mode) and info.st_nlink == 1
                and info.st_uid == (os.geteuid() if uid is None else uid)
                and stat.S_IMODE(info.st_mode) == 0o600 and 0 < info.st_size <= maximum,
                'private regular owner file required')
        raw = stream.read(maximum + 1)
        after = os.fstat(stream.fileno())
        require(len(raw) == info.st_size and info.st_mtime_ns == after.st_mtime_ns
                and info.st_ctime_ns == after.st_ctime_ns, 'private file changed during read')
        return raw


class HumanRefusal(ValueError):
    def __init__(self, status, detail):
        self.status, self.detail = status, detail
        super().__init__('Human request refused: ' + str(detail))


class HumanClient:
    """The real bounded Human HTTP boundary, also usable with recorded peers."""
    def __init__(self, url, origin, session, context=None):
        self.url = merged.CHAIN.endpoint(url)
        merged.CHAIN.endpoint(origin)
        self.origin, self.context = origin, context
        self.session = deepcopy(session)
        require(self.session.get('cookies', {}).get('__Host-layerx_csrf'), 'CSRF authority required')

    def call(self, method, path, body=None, idempotency=None):
        require(method in ('GET', 'POST') and re.fullmatch(r'/v1/[A-Za-z0-9_./-]+', path)
                and '..' not in path and (body is None or method == 'POST'), 'canonical Human request required')
        trace = 'trc_' + secrets.token_hex(16)
        headers = {'Origin': self.origin, 'X-LayerX-Trace': trace,
                   'X-LayerX-CSRF': self.session['cookies']['__Host-layerx_csrf'],
                   'Cookie': '; '.join(key + '=' + value for key, value in self.session['cookies'].items()),
                   'Accept': 'application/json'}
        if self.session.get('assertion'):
            headers['Authorization'] = 'Bearer ' + self.session['assertion']
        if idempotency is not None:
            require(isinstance(idempotency, str) and 0 < len(idempotency) <= 256
                    and idempotency.isascii() and not any(c.isspace() for c in idempotency),
                    'bounded idempotency key required')
            headers['Idempotency-Key'] = idempotency
        raw = None if body is None else json.dumps(body, separators=(',', ':')).encode()
        require(raw is None or len(raw) <= 1_048_576, 'bounded Human request')
        if raw is not None:
            headers['Content-Type'] = 'application/json'
        connection = (http.client.HTTPSConnection(self.url.hostname, self.url.port, context=self.context, timeout=20)
                      if self.url.scheme == 'https' else http.client.HTTPConnection(self.url.hostname, self.url.port, timeout=20))
        try:
            connection.request(method, path, raw, headers)
            response = connection.getresponse()
            data = response.read(MAXIMUM + 1)
            require(len(data) <= MAXIMUM and response.getheader('Content-Type', '').split(';')[0] == 'application/json'
                    and response.getheader('Cache-Control') == 'no-store'
                    and response.getheader('X-LayerX-Trace') == trace, 'private typed Human transport required')
            value = strict_json(data)
            require(isinstance(value, dict) and value.get('trace') == trace and type(value.get('ok')) is bool,
                    'Human response envelope identity')
            for key, cookie in response.getheaders():
                if key.lower() != 'set-cookie':
                    continue
                parts = [part.strip() for part in cookie.split(';')]
                name, separator, content = parts[0].partition('=')
                require(separator and name in ('__Host-layerx_access', '__Host-layerx_refresh', '__Host-layerx_csrf')
                        and {'Secure', 'Path=/', 'SameSite=Strict'} <= set(parts)
                        and (name == '__Host-layerx_csrf' or 'HttpOnly' in parts), 'protected session cookie required')
                self.session['cookies'][name] = content
            if not value['ok']:
                require(response.status in (400, 401, 403, 404, 409, 422, 503)
                        and set(value) == {'ok', 'error', 'trace'} and isinstance(value['error'], dict)
                        and isinstance(value['error'].get('code'), str), 'typed Human refusal required')
                raise HumanRefusal(response.status, value['error'])
            require(response.status == 200 and set(value) == {'ok', 'result', 'trace'}, 'closed Human success required')
            return value['result']
        finally:
            connection.close()


def denied(operation, codes=('not-found', 'forbidden'), statuses=(403, 404)):
    try:
        operation()
    except HumanRefusal as error:
        require(error.status in statuses and error.detail['code'] in codes, 'wrong authenticated refusal')
        return
    raise ValueError('foreign or altered request was admitted')


def evidence(client, foreign, reference, committed=False):
    identity = reference['evidence_id']
    require(re.fullmatch('evd_[a-f0-9]{64}', identity), 'canonical evidence identity')
    material = client.call('GET', '/v1/evidence/' + identity)
    raw = base64.b64decode(material['bytes_base64'], validate=True)
    require(raw and base64.b64encode(raw).decode() == material['bytes_base64']
            and material['evidence_id'] == identity and material['class'] == reference['class']
            and (material['verification'] == reference['verification']
                 or committed and reference['verification'] == 'receipt-verified'
                    and material['verification'] == 'checkpoint-finalised'), 'exact owned evidence material required')
    # Receipt references identify signed receipt bytes; other exports identify
    # their raw material. A portable receipt envelope is deliberately distinct.
    digest_material = raw
    if material['class'] == 'layerx-receipt':
        require(material['content_type'] == 'application/vnd.layerx.receipt-proof-v2+json', 'current receipt export required')
        portable = strict_json(raw)
        require(portable.get('format') == 'layerx-receipt-proof-v2', 'current portable receipt codec required')
        digest_material = bytes.fromhex(portable['signedReceipt'].removeprefix('0x'))
    matches = 'evd_' + hashlib.sha256(digest_material).hexdigest() == identity
    if material['class'] == 'layerx-receipt' and 'rawTransaction' in portable:
        transaction = bytes.fromhex(portable['rawTransaction'].removeprefix('0x'))
        matches = matches or 'evd_' + merged.CHAIN.keccak256(transaction).hex() == identity
    require(matches, 'actual evidence digest differs')
    denied(lambda: foreign.call('GET', '/v1/evidence/' + identity))
    return material, raw


def committed_evidence(client, foreign, node, reference, transaction_hash, owner):
    material, raw = evidence(client, foreign, reference, committed=True)
    require(reference['class'] == 'layerx-receipt' and reference['verification'] in ('receipt-verified', 'checkpoint-finalised')
            and material['verification'] == 'checkpoint-finalised' and material.get('rung') == 'committed', 'committed receipt evidence required')
    portable = strict_json(raw)
    require(portable['verificationLevel'] == 'receipted', 'portable signed execution proof required')
    canonical = portable['signedReceipt'].removeprefix('0x')
    actual = node.committed(transaction_hash, owner=owner, canonical=canonical)
    require(portable['receiptDigest'] == '0x' + actual['receipt_digest']
            and ('rawTransaction' not in portable or portable['rawTransaction'] == actual['raw_transaction']),
            'portable digest or signed transaction differs')
    require(material['inclusion_height'] == int(actual['block_number'], 16)
            and int(portable['inclusionHeight'], 16) == material['inclusion_height'], 'receipt inclusion height differs')
    pin = portable['receiptKey']
    require(any(pin['keyId'].removeprefix('0x') == key['key_id']
                and pin['publicKey'].removeprefix('0x') == key['public_key']
                and int(pin['firstHeight'], 16) == int(str(key['first_height']), 0)
                and int(pin['lastHeight'], 16) == int(str(key['last_height']), 0)
                for key in node.receipt_keys), 'portable receipt authority differs from installed pins')
    return actual


def committed_budget(raw, chain, tenant, principal, owner, now_ms):
    """Replay the authority's v2 export at its signed committed bank state."""
    value = strict_json(raw)
    require(value['schema'] == 'layerx.human.budget-proof.v2' and value['tenant'] == tenant
            and value['principal'] == principal and value['network_id'] == 125 and value['protocol_version'] == 3,
            'budget proof owner or native network differs')
    height = value['head']['committed_height']
    require(type(height) is int and height > 0, 'committed budget height required')
    require(chain.node.call('eth_chainId', []) == '0x7d', 'budget chain differs')
    block = chain.node.call('eth_getBlockByNumber', [hex(height), False])
    finality = value['finality']
    require(block['number'] == hex(height) and block['hash'] == '0x' + value['head']['block_hash']
            and finality == {'height': height, 'block_hash': block['hash'][2:], 'app_hash': block['stateRoot'][2:]},
            'budget does not bind the committed bank block')
    signed = chain.hosted('px_getCommit', [str(height)])['signedHeader']
    commit = deepcopy(signed['commit'])
    commit['height'] = int(commit['height'])
    for signature in commit['signatures']:
        if signature['block_id_flag'] == 2:
            signature['signature'] = base64.b64decode(signature['signature'], validate=True).hex()
    try:
        block_hash = verify_commit(signed['header'], commit, trusted_validators=tuple(chain.validator_pins), chain_id='125')
    except InvalidSignature as error:
        raise ValueError('budget commit signature refused') from error
    require(block_hash.hex() == value['head']['block_hash']
            and signed['header']['app_hash'] == finality['app_hash'], 'budget commit signature or app hash differs')
    state = value['budget_state']
    current = int(chain.node.call('eth_blockNumber', []), 16)
    age = current - height
    timestamp = int(datetime.fromisoformat(signed['header']['time'].replace('Z', '+00:00')).timestamp() * 1000)
    require(type(state['maximum_age_sequences']) is int and state['maximum_age_sequences'] > 0
            and 0 <= age <= state['maximum_age_sequences'] and state['age_sequences'] == 0
            and 0 < value['maximum_age_seconds'] <= 60
            and 0 <= now_ms - timestamp <= value['maximum_age_seconds'] * 1000,
            'budget committed block freshness refused')
    budget_id = bytes.fromhex(state['budget_id'])
    require(len(budget_id) == 32 and any(budget_id), 'budget identity required')
    outputs = []
    for key, name in (('budget_call', 'getBudget'), ('remaining_call', 'budgetRemaining')):
        call = value['proofs'][key]
        method = merged.abi_method('layerxpay', name)
        data = merged.evm.encode_call(name, ['bytes32'], [budget_id])
        require(call['height'] == height and call['to'] == PAY[2:] and call['data'] == data.hex(),
                'budget selector or height differs')
        actual = chain.node.call('eth_call', [{'to': PAY, 'data': '0x' + data.hex()}, hex(height)])
        require(actual == '0x' + call['result'], 'budget result differs from committed bank')
        outputs.append(merged.decode_abi([merged.CHAIN.Node.abi_type(row) for row in method['outputs']], actual))
    record, remaining = outputs[0][0], outputs[1][0]
    source = record[17] if any(record[17]) and record[17] != record[0] else record[0]
    require(value['accounts'] == {'owner': record[0].hex(), 'budget': record[1].hex(), 'source': source.hex()}
            and state['asset'] == record[2].hex() and state['canonical_core_bytes'] == value['proofs']['budget_call']['result']
            and state['remaining'] == str(remaining) and remaining <= max(0, record[4] + record[8] - record[7])
            and state['closed'] == record[14] and state['revoked'] == record[15]
            and (not (record[14] or record[15] or timestamp < record[10] or timestamp >= record[11]) or remaining == 0)
            and state['verification'] == 4 and state['observed_head_sequence'] == height
            and state['checkpoint_digest'] == block_hash.hex()
            and state['receipt_digest'] == hashlib.sha256(bytes.fromhex(value['proofs']['budget_call']['result'])).hexdigest(),
            'budget accounting or committed projection differs')
    require(type(state['revocation_sequence']) is int and 0 < state['revocation_sequence'] <= height
            and state['revocation_sequence'] == record[12], 'budget revocation sequence differs')
    join_raw = json.dumps(value['proofs']['account_join'], separators=(',', ':'), sort_keys=True).encode()
    binding = hashlib.sha256(b'layerx-human/rpc-budget-evidence/v2\0' + block_hash)
    for data in (bytes.fromhex(value['proofs']['budget_call']['result']),
                 bytes.fromhex(value['proofs']['remaining_call']['result']), join_raw):
        binding.update(len(data).to_bytes(8, 'big') + data)
    require(binding.hexdigest() == state['evidence_digest'], 'budget producer evidence digest differs')
    identity = b'LXP/v1/did-id\0' + len(owner.encode()).to_bytes(2, 'big') + owner.encode()
    join = value['proofs']['account_join']
    require(value['identity_key'] == hashlib.sha256(identity).hexdigest()
            and join['account']['layerx_did'] == owner
            and sum(row['bound'] is True and row['layerx_account'].removeprefix('0x') == record[0].hex()
                    and re.fullmatch('0x[0-9a-f]{40}', row['evm_address']) is not None
                    and int(row['evm_address'], 16) != 0 for row in join['layerx']['accounts']) == 1,
            'budget owner account join differs')
    return {'remaining': remaining, 'source_account': source.hex(), 'asset_id': record[2].hex(),
            'budget_id': state['budget_id'], 'block_hash': block_hash.hex(), 'height': height, 'age_blocks': age,
            'maximum_age_blocks': state['maximum_age_sequences'], 'proof_digest': hashlib.sha256(raw).hexdigest()}


def current_clients(fixture):
    context = ssl.create_default_context(cafile=fixture['ca_pem']) if fixture.get('ca_pem') else None
    clients = {name: HumanClient(fixture['url'], fixture['origin'], session, context)
               for name, session in fixture['sessions'].items()}
    require(fixture['principal'] != fixture['other_principal'], 'distinct authenticated principals required')
    config = fixture['chain']
    node = merged.CHAIN.Node(config['rpc_url'], config['receipt_keys'], config.get('headers'), context)
    chain = merged.Client(node, config['gateway_url'], config.get('headers'), context)
    chain.validator_pins = config['validator_pins']
    return clients[fixture['principal']], clients[fixture['other_principal']], chain


def qualify_projections(client, foreign, chain, fixture):
    snapshots = {}
    for row in fixture['committed_journeys']:
        value = client.call('GET', '/v1/journeys/' + row['journey_id'])
        require(value['state'] == 'done' and value['kind'] == 'move', 'native completed journey required')
        references = value['evidence'] + [ref for stage in value['stages'] for ref in stage['evidence']]
        require(references, 'actual committed journey evidence required')
        for ref in references:
            committed_evidence(client, foreign, chain.node, ref, row['transaction_hash'], row['owner_address'])
        denied(lambda: foreign.call('GET', '/v1/journeys/' + row['journey_id']))
        snapshots[row['journey_id']] = value
    for row in fixture['budget_exports']:
        material, raw = evidence(client, foreign, row['reference'])
        require(material['verification'] == 'checkpoint-finalised', 'committed bank budget label required')
        verified = committed_budget(raw, chain, row['tenant'], row['principal'], row['owner'], fixture['now_ms'])
        require(verified['remaining'] == int(row['remaining']), 'post-reservation native budget differs')
        snapshots[row['reference']['evidence_id']] = verified
    require(fixture['committed_journeys'] and fixture['budget_exports'], 'receipt and bank budget producers required')
    return snapshots


CLIENT = r'''
import {readFile, writeFile} from 'node:fs/promises';
import {pathToFileURL} from 'node:url';
import {createHash, randomBytes} from 'node:crypto';
const [root, manifestPath, outputPath] = process.argv.slice(2);
const fixture = JSON.parse(await readFile(manifestPath, 'utf8'));
const {createHumanApiClient, HumanApiError} = await import(pathToFileURL(root + '/apps/web/src/api/generated/index.ts').href);
const {conformance} = await import(pathToFileURL(root + '/apps/web/src/api/generated/conformance.ts').href);
const {copyEntry} = await import(pathToFileURL(root + '/apps/web/copy/runtime.ts').href);
const {copyEntries} = await import(pathToFileURL(root + '/apps/web/copy/catalog.ts').href);
const {runtimeMessages} = await import(pathToFileURL(root + '/apps/web/copy/messages.generated.ts').href);
let tests = 0;
const stable = (value) => JSON.stringify(value, (_, item) => typeof item === 'bigint' ? item.toString() : item);
function check(value, message) { if (!value) throw new Error(message); tests += 1; }
check(stable(runtimeMessages) === stable(copyEntries.map(({key,message}) => [key,message])), 'runtime copy is generated from the actual authoritative catalog');
function copy(key) {
  check(typeof key === 'string' && copyEntry(key).message.length > 0, 'actual service projection copy resolves through the production catalog');
}
const clients = new Map();
for (const [name, session] of Object.entries(fixture.sessions)) {
  const jar = new Map(Object.entries(session.cookies));
  clients.set(name, createHumanApiClient({baseUrl:fixture.url, csrfToken:() => jar.get('__Host-layerx_csrf'), trace:() => 'trc_' + randomBytes(16).toString('hex'),
    fetch:async (url, init) => {
      const headers = new Headers(init.headers);
      headers.set('Origin', fixture.origin);
      headers.set('Cookie', [...jar].map(([key,value]) => key + '=' + value).join('; '));
      const response = await fetch(url, {...init, headers, redirect:'error', signal:init.signal ?? AbortSignal.timeout(20000)});
      for (const cookie of response.headers.getSetCookie()) {
        const pair = cookie.split(';')[0], index = pair.indexOf('=');
        jar.set(pair.slice(0,index), pair.slice(index+1));
      }
      return response;
    }}));
}
const client = clients.get(fixture.principal), foreign = clients.get(fixture.other_principal);
check(client !== undefined && foreign !== undefined, 'real distinct authenticated sessions');
async function denied(operation, codes) {
  let error; try { await operation(); } catch (caught) { error = caught; }
  check(error instanceof HumanApiError && codes.includes(error.detail.code), 'typed authenticated foreign-owner refusal');
}
function timestamp(value) {
  check(typeof value === 'string' && /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{3})?Z$/.test(value) && Number.isFinite(Date.parse(value)), 'real UTC timestamp preserves native precision');
}
const summaryFields = ['approval_id','agent_id','agent_name','counterparty','amount','reason_copy_key','expires_at','state','budget_remaining_after'];
const detailFields = ['approval_id','agent_id','agent_name','state','state_copy_key','reason_copy_key','facts','budget_remaining_after','created_at','evidence'];
function fields(value, expected) { check(stable(Object.keys(value).sort()) === stable([...expected].sort()), 'distinct closed canonical response fields'); }
function summary(value) {
  fields(value, summaryFields);
  check(/^apr_[a-f0-9]{64}$/.test(value.approval_id) && /^agt_[a-f0-9]{64}$/.test(value.agent_id), 'strict approval and actual managed child IDs');
  timestamp(value.expires_at);
  check(value.reason_copy_key === 'approval.reason.policy-required', 'actual policy reason preserved');
  copy(value.reason_copy_key);
  check(['receipt-verified','checkpoint-finalised'].includes(value.budget_remaining_after.verification), 'native verified post-reservation budget label');
  check(value.budget_remaining_after.money.currency === value.amount.currency, 'remaining budget retains the actual held asset currency');
}
async function exportEvidence(ref, supplied = client) {
  check(/^evd_[a-f0-9]{64}$/.test(ref.evidence_id), 'strict genuine evidence ID');
  const material = await supplied.evidenceGet(ref.evidence_id);
  check(material.evidence_id === ref.evidence_id, 'actual evidence response retains the requested owned identifier');
  const bytes = Buffer.from(material.bytes_base64, 'base64');
  check(bytes.toString('base64') === material.bytes_base64, 'export uses canonical complete base64 bytes');
  let digestBytes = bytes;
  if (material.class === 'layerx-receipt') {
    const portable = JSON.parse(bytes.toString('utf8'));
    check(portable.format === 'layerx-receipt-proof-v2' && /^0x(?:[a-f0-9]{2})+$/.test(portable.signedReceipt), 'current portable signed receipt');
    digestBytes = Buffer.from(portable.signedReceipt.slice(2), 'hex');
  }
  const committedReference = material.class === 'layerx-receipt' && material.rung === 'committed' && fixture.committed_journeys.some(row => 'evd_' + row.transaction_hash.slice(2) === ref.evidence_id);
  check(bytes.length > 0 && ('evd_' + createHash('sha256').update(digestBytes).digest('hex') === ref.evidence_id || committedReference), 'actual signed receipt or independently committed transaction identity');
  check((material.verification === ref.verification || material.class === 'layerx-receipt' && ref.verification === 'receipt-verified' && material.verification === 'checkpoint-finalised' && material.rung === 'committed') && material.class === ref.class, 'export retains actual achieved verification and class');
  await denied(() => foreign.evidenceGet(ref.evidence_id), ['not-found','forbidden']);
}
const opened = await client.streamOpen();
const agents = (await client.agentList()).agents;
check(agents.length > 0, 'actual managed agent producer required');
for (const value of agents) {
  check(/^agt_[a-f0-9]{64}$/.test(value.agent_id), 'actual managed ID');
  copy(value.state_copy_key);
  copy(value.limit.enforcement_copy_key);
  for (const key of ['created_at','updated_at']) timestamp(value[key]);
  timestamp(value.spend.period_start); timestamp(value.spend.period_end);
  check(value.spend.verification === 'receipt-verified' || value.spend.verification === 'checkpoint-finalised', 'native managed verification mapping');
  const point = await client.agentGet(value.agent_id);
  check(stable(point) === stable(value), 'shared managed list and get projection');
  check(value.evidence.length > 0, 'actual managed receipt evidence present');
  for (const ref of value.evidence) await exportEvidence(ref);
  await denied(() => foreign.agentGet(value.agent_id), ['not-found','forbidden']);
}
const inventory = await client.approvalList();
check(inventory.approvals.length > 0 && inventory.approvals.length <= 100, 'actual bounded approval producer inventory');
const seen = new Map();
for (const value of inventory.approvals) {
  summary(value); seen.set(value.approval_id, value);
  const detail = await client.approvalGet(value.approval_id);
  fields(detail, detailFields);
  copy(detail.state_copy_key);
  copy(detail.reason_copy_key);
  check(detail.agent_id === value.agent_id && detail.agent_name === value.agent_name && detail.state === value.state, 'shared list and detail actual association');
  check(detail.facts.amount.amount === value.amount.amount && detail.facts.amount.currency === value.amount.currency && detail.facts.counterparty === value.counterparty && detail.facts.expires_at === value.expires_at, 'same held disclosure facts across distinct projections');
  check(stable(detail.budget_remaining_after) === stable(value.budget_remaining_after), 'shared detail and summary use the same genuine post-reservation budget');
  timestamp(detail.created_at);
  check(detail.evidence.length > 0, 'real hold or budget evidence required');
  for (const ref of detail.evidence) await exportEvidence(ref);
  await denied(() => foreign.approvalGet(value.approval_id), ['not-found','forbidden']);
}
check(Array.isArray(fixture.observations) && fixture.observations.length >= 2, 'genuine legacy and native producer observations required');
const profiles = new Set();
for (const expected of fixture.observations) {
  profiles.add(expected.profile);
  const value = seen.get(expected.approval_id);
  check(value !== undefined && value.agent_id === expected.agent_id && value.counterparty === expected.counterparty && value.amount.amount === BigInt(expected.amount) && value.amount.currency === expected.currency && value.budget_remaining_after.money.amount === BigInt(expected.remaining_after), 'actual authenticated producer observation matches canonical facts and reserved budget');
  check(Date.parse(value.expires_at) === expected.activity_expires_at_unix_milliseconds, 'activity expiry preserves genuine wall-time units');
  const detail = await client.approvalGet(value.approval_id);
  check(Date.parse(detail.created_at) === expected.created_at_unix_seconds * 1000, 'creation time comes from genuine persisted Unix metadata');
  if (expected.profile === 3) {
    const local = detail.evidence.filter((reference) => reference.class === 'approval-hold');
    check(local.length === 2 && local.every((reference) => reference.verification === 'unverified'), 'actual held canonical bytes and local carrier never promoted to protocol verification');
    const allocation = detail.evidence.filter((reference) => reference.class === 'local-journey-state');
    check(allocation.length === 1 && allocation[0].verification === 'unverified', 'allocation report remains honestly local owned');
    const proof = detail.evidence.filter((reference) => reference.class === 'checkpoint-proof');
    check(proof.length >= 1 && proof.every((reference) => ['checkpoint-finalised'].includes(reference.verification)), 'real independently verified raw budget package remains separate from local materials');
    check(local.some((reference) => reference.evidence_id === 'evd_' + expected.held_digest), 'held carrier export bound to genuine immutable owner digest');
    check(proof.some((reference) => reference.evidence_id === 'evd_' + expected.budget_proof_digest), 'raw budget export bound to actual independently verified package');
    await denied(() => conformance['approval.approve']({client:foreign,params:{approval_id:expected.approval_id},body:expected.foreign_approve_body,idempotencyKey:expected.foreign_approve_key}), ['not-found','forbidden','step-up-required']);
  }

}
check(profiles.has(2) && profiles.has(3), 'real legacy and generic native producers both executed');
const home = await client.homeSummary();
check(stable(home.agents) === stable(agents) && stable(home.approvals) === stable(inventory.approvals), 'home uses identical canonical projections');
const page = await client.streamNext(opened.cursor);
let delivered = 0;
for (const event of page.events) {
  timestamp(event.observed_at);
  if (event.approval) { summary(event.approval); check(seen.has(event.approval.approval_id), 'durable approval stream uses canonical summary'); check(stable(event.approval) === stable(seen.get(event.approval.approval_id)), 'durable push retains the exact canonical approval facts'); delivered += 1; }
}
check(delivered > 0, 'real durable approval producer append observed');
check(Array.isArray(fixture.decisions) && fixture.decisions.length > 0, 'actual winning and repeated decision cases required');
for (const row of fixture.decisions) {
  check(['approval.approve','approval.reject'].includes(row.operation), 'schema-owned exact decision operation');
  const input = {client, params:{approval_id:row.approval_id}, body:row.body, idempotencyKey:row.idempotency_key};
  const decision = await conformance[row.operation](input);
  fields(decision,['approval_id','state','state_copy_key','money_moved','moved_copy_key','evidence']);
  check(decision.approval_id === row.approval_id && decision.state === row.state && decision.money_moved === false, 'actual decision never claims money moved');
  copy(decision.state_copy_key);
  copy(decision.moved_copy_key);
  check(decision.moved_copy_key === 'approval.decision.no-money-moved', 'decision wording states authorization without claiming execution');
  check(stable(await conformance[row.operation](input)) === stable(decision), 'same decision replay returns exact original outcome');
  const terminal = await client.approvalGet(row.approval_id);
  check(terminal.state === decision.state, 'terminal owner record remains readable through canonical detail');
  fields(terminal, detailFields);
  copy(terminal.state_copy_key);
  check(['receipt-verified','checkpoint-finalised'].includes(terminal.budget_remaining_after.verification), 'terminal budget retains its actual achieved verification level');
  for (const reference of terminal.evidence) await exportEvidence(reference);
  const terminalInventory = await client.approvalList();
  const terminalSummary = terminalInventory.approvals.find((value) => value.approval_id === row.approval_id);
  check(terminalSummary !== undefined, 'actual terminal approval remains present in canonical inventory');
  summary(terminalSummary);
  check(terminalSummary.state === terminal.state && stable(terminalSummary.budget_remaining_after) === stable(terminal.budget_remaining_after), 'terminal list and detail share genuine state and budget');
  const terminalHome = await client.homeSummary();
  check(stable(terminalHome.approvals) === stable(terminalInventory.approvals), 'terminal home retains canonical owner inventory');
  await denied(() => foreign.approvalGet(row.approval_id), ['not-found','forbidden']);
}
await writeFile(outputPath, stable({tests}), {mode:0o600});
'''


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--candidate-manifest', required=True)
    args = parser.parse_args()
    sys.path.insert(0, str(ROOT / 'tools/paxeer-x'))
    from candidate import catalogue, load_private, validate
    candidate = load_private(args.candidate_manifest)
    validate(candidate, catalogue(ROOT / SERVICE_CATALOGUE))
    revision, source = B.identity()
    B.require(candidate['source']['revision'] == revision and not candidate['source']['dirty'], 'genuine clean candidate source')
    fixture = os.environ.get('PAXEER_X_HUMAN_CANONICAL_MANIFEST')
    B.require(fixture, 'genuine private canonical projection fixture required')
    material = B.load(fixture)
    B.require(material['schema'] == 'layerx-human-canonical-projections.v1', 'closed real-process fixture profile')
    directory = B.private(material['evidence_directory'], True)
    B.require(not any(directory.iterdir()), 'fresh private evidence directory')
    artifacts = {name:B.executable(row,revision,source) for name,row in material['artifacts'].items()}
    roles = ('service','components','agent','paxd')
    B.require(all(role in artifacts for role in roles), 'actual production artifacts required')
    node = Path(material['node'])
    B.require(node.is_absolute() and node.is_file() and os.access(node,os.X_OK) and B.digest(node) == material['node_sha256'], 'pinned actual Node runtime')
    ca = B.private(material['ca_pem'])
    context = ssl.create_default_context(cafile=str(ca))
    url = B.endpoint(material['url'])
    B.require(material['principal'] != material['other_principal'], 'genuine distinct principal ownership')
    B.require(5 <= len(material['processes']) <= 32, 'bounded actual production processes')
    by_role = {row['artifact']:row for row in material['processes']}
    B.require(all(role in by_role for role in roles), 'actual process roles')
    service = by_role['service']['environment']
    B.require(service['LAYERX_HUMAN_LISTENER'] == 'tls' and service['LAYERX_HUMAN_BIND'] == '127.0.0.1:' + str(url.port) and service['LAYERX_HUMAN_WEB_ORIGIN'] == material['origin'], 'isolated real HTTPS boundary')
    B.private(service['LAYERX_HUMAN_TLS_CERT_DER']); B.private(service['LAYERX_HUMAN_TLS_KEY_DER'])
    processes = B.Processes(directory,artifacts)
    try:
        for row in material['processes']: processes.start(row)
        until = time.monotonic() + min(90,B.remaining())
        while True:
            B.require(all(child.poll() is None for child,_,_ in processes.children.values()), 'actual process remains alive')
            try:
                status,ready = B.request(url,context,'GET','/readyz')
                if status == 200 and ready.get('result',{}).get('ready') is True: break
            except (OSError,B.http.client.HTTPException): pass
            B.require(time.monotonic() < until, 'real readiness deadline')
            time.sleep(0.1)
        owner, foreign, chain = current_clients(material)
        before = qualify_projections(owner, foreign, chain, material)
        stopped = processes.stop(by_role['service']['name'])
        processes.start(dict(stopped, name=stopped['name'] + '-reopened'))
        until = time.monotonic() + min(90, B.remaining())
        while True:
            try:
                status, ready = B.request(url, context, 'GET', '/readyz')
                if status == 200 and ready.get('result', {}).get('ready') is True: break
            except (OSError, B.http.client.HTTPException): pass
            B.require(time.monotonic() < until, 'reopened Human readiness deadline')
            time.sleep(0.1)
        B.require(qualify_projections(owner, foreign, chain, material) == before, 'committed projections survive restart')
        runner = directory / 'canonical-projections.mts'; runner.write_text(CLIENT); runner.chmod(0o600)
        output = directory / 'canonical-result.json'
        with (directory / 'canonical-client.log').open('xb') as log:
            os.chmod(log.name,0o600)
            result = subprocess.run([str(node),'--experimental-strip-types',str(runner),str(ROOT),fixture,str(output)],cwd=ROOT,env=dict(os.environ,NODE_EXTRA_CA_CERTS=str(ca),NODE_TLS_REJECT_UNAUTHORIZED='1',NODE_OPTIONS=''),stdout=log,stderr=log,timeout=min(900,B.remaining()))
        B.require(result.returncode == 0,'real canonical production boundary case failed')
        observed = B.load(output)
        B.require(type(observed['tests']) is int and observed['tests'] >= 30,'real bounded positive and negative cases executed without skips')
        print('human-canonical-projections: PASS tests=' + str(observed['tests']) + ' revision=' + revision)
    finally:
        processes.close()


if __name__ == '__main__':
    try: main()
    except Exception as error:
        print('human-canonical-projections: FAIL: ' + str(error),file=sys.stderr)
        raise SystemExit(1)

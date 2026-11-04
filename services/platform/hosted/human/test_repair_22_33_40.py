"""Keep the operator guide bound to real merged owner receipts and projections."""
import copy
import hashlib
import json
from pathlib import Path
import sys
import time

import pytest
import yaml

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[3]
sys.path.insert(0, str(HERE))

import material
import merged_owner
import provision
from merged_owner import LEDGER, PAYMENT, evm
from test_repair_22_31_348 import environment


def guide_sections():
    guide = (HERE / 'README.md').read_text()
    current, barriers = guide.split('## Remaining cluster migration barriers\n', 1)
    return current, barriers


def test_hosted_human_first_batch_committed_observations(environment):
    e = environment
    current, _ = guide_sections()
    for term in ('eth_chainId', 'eth_getTransactionCount', '`pending`',
                 'eth_sendRawTransaction', 'eth_getTransactionReceipt',
                 'eth_getBlockByNumber', 'getLayerXDid(address)',
                 'accountId(bytes32,bytes32)', 'balanceOf(bytes32,bytes32)',
                 'committed identity/recovery', 'zero consumed limits'):
        assert term in current
    assert 'A transaction hash alone does not establish completion.' in current
    assert 'Committed blocks are final on this network.' in current
    assert 'validate-account-head' not in current
    assert 'batch-number-one' not in current

    evidence = provision.protected_json(e.inputs / 'merged-owner-evidence.json')
    assert set(evidence['transactions']) == {'binding', 'identity', 'recovery', 'account', 'funding'}
    assert merged_owner.validate_evidence(evidence, e.context, e.server.ca.read_bytes(),
                                         e.registration, 125, 125) == evidence
    for record in evidence['transactions'].values():
        receipt = record['receipt']
        assert receipt['status'] == '0x1'
        assert e.client.rpc.request('eth_getBlockByNumber', [receipt['blockNumber'], False]) == {
            'number': receipt['blockNumber'], 'hash': receipt['blockHash'],
            'transactions': [receipt['transactionHash']],
        }
    identity = e.registration['identity']
    recovery = evidence['transactions']['recovery']['receipt']
    assert identity['evidence'] == identity['recovery']['evidence'] == merged_owner.reference(recovery)
    installed = provision.protected_json(e.installed / 'authority/principal-policy.json')
    assert installed['principals'][0]['identities'] == [identity]

    # Spend after bootstrap: current account state changes, while the original
    # committed funding and recovery evidence remain valid for bundle consumers.
    owner = e.registration['owner_account']
    balance_call = ('0x' + owner, '0x' + e.asset)
    before, = e.client.rpc.eth_call(LEDGER, 'balanceOf(bytes32,bytes32)(uint128)', *balance_call)
    reference = hashlib.sha256(b'documented-current-observation').hexdigest()
    paid = e.client.pay(e.policy, 'beta-protocol', e.destination, 7, reference,
                        time.time_ns() // 1000000 + 300000)
    after, = e.client.rpc.eth_call(LEDGER, 'balanceOf(bytes32,bytes32)(uint128)', *balance_call)
    assert after == before - 7
    assert paid['receipt']['to'] == LEDGER
    assert material.verify_bundle(e.bundle, 125, 125)['records'] == 2
    assert {'eth_chainId', 'eth_getTransactionCount', 'eth_sendRawTransaction',
            'eth_getTransactionReceipt', 'eth_getBlockByNumber', 'eth_call'} <= {
                request['method'] for request in e.server.requests}
    assert all(request['jsonrpc'] == '2.0' and set(request) == {'jsonrpc', 'id', 'method', 'params'}
               for request in e.server.requests)


def test_hosted_human_first_batch_receipt_refusal(environment):
    e = environment
    current, _ = guide_sections()
    assert 'An unavailable\nRPC or changed retained receipt refuses assembly.' in current
    evidence = provision.protected_json(e.inputs / 'merged-owner-evidence.json')
    receipt = evidence['transactions']['funding']['receipt']
    transaction = receipt['transactionHash']
    original = e.server.receipts[transaction]
    altered = copy.deepcopy(original)
    altered['status'] = '0x0'
    try:
        e.server.receipts[transaction] = altered
        with pytest.raises(ValueError, match='successful transaction receipt'):
            material.verify_bundle(e.bundle, 125, 125)
    finally:
        e.server.receipts[transaction] = original


def test_hosted_human_first_batch_bank_contract():
    current, _ = guide_sections()
    signature = 'send(address,address,string,uint256)'
    selector = '0x' + evm.selector(signature).hex()
    bank = '0x0000000000000000000000000000000000001001'
    abi = json.loads((ROOT / 'precompiles/bank/abi.json').read_text())
    send, = [row for row in abi if row['type'] == 'function' and row['name'] == 'send'
             and [field['type'] for field in row['inputs']] == ['address', 'address', 'string', 'uint256']]
    types = [field['type'] for field in send['inputs']]
    assert 'send(' + ','.join(types) + ')' == signature
    arguments = ['0x' + '11' * 20, '0x' + '22' * 20, 'apax', 7]
    calldata = evm.calldata(signature, *arguments)
    assert evm.decode(types, evm.unhex(calldata)[4:]) == arguments
    assert bank in current and signature in current and selector in current
    contract = (ROOT / 'services/human/schema/human-api/intent.kvx').read_text()
    assert bank in contract and signature in contract and selector in contract
    for term in ('denom:<denomination>', 'submission re-plans', 'signed digest',
                 'GET /v1/journeys/{journey_id}', '`submitted`', '`receipted`',
                 '`committed`', '`checkpoint-finalised`',
                 'waits for a verified committed receipt for each leg'):
        assert term in current
    assert 'These bank sends are distinct from the ledger' in current
    assert LEDGER in current and PAYMENT in current


def test_hosted_human_first_batch_migration_barriers(environment):
    e = environment
    current, barriers = guide_sections()
    job = yaml.safe_load((HERE / 'provision-owner-job.yaml').read_text())
    containers = job['spec']['template']['spec']['containers']
    assert any(container['name'] == 'validate-account-head' for container in containers)
    lni = e.installed / 'agent-config/LAYERX_AGENT_HUMAN_NODE_LNI'
    assert lni.read_text() == '/run/layerx/node/layerxd.lni.sock'
    movement = e.installed / 'movement-config'
    assert (movement / 'LAYERX_HUMAN_MOVEMENT_PROVIDER_PAXEER_LEDGER_CONTRACT').read_text() == LEDGER
    assert 'LAYERX_HUMAN_MOVEMENT_PROVIDER_' not in current
    assert 'HUMAN_NODE_LNI' not in current
    assert 'not a provisioning recipe for merged accounts' in barriers
    assert 'Never synthesize the result from a balance' in barriers
    for name in ('validate-account-head', 'batch-number-one', 'consumed=0',
                 'LAYERX_HUMAN_MOVEMENT_PROVIDER_', 'human-movement',
                 '/run/layerx/human/movement.sock', 'HUMAN_NODE_LNI'):
        assert name in barriers
    record = (HERE / 'repair_22_33_40.kvx').read_text()
    assert '[finding.hosted-human-first-batch]' in record
    for name in ('services/human/crates/layerx-human-identity-provider',
                 'services/agent', 'services/platform/hosted/node/deployment.yaml'):
        assert name in record

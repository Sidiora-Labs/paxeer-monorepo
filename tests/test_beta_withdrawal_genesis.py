"""Task 22.33.891 repair record (kept in scope with its Python gate).

Finding -> proving case:
authority-selector-ignores-run -> authority_cli_selection (actual suite exec).
beta-test-retired-bootstrap -> test_public_bootstrap_and_real_builder_bind_v3_and_native_authority
    (production bootstrap/broker/consumer with committed paxd CLI/RPC transcripts).
deposit-tests-retired-credit -> test_native_subaccount_funding_committed_proof,
    test_historical_deposit_refusals (signed ledger SEND into the derived asset
    account with committed balance witnesses; Cosmos bank wiring requested below).
withdraw-tests-retired-settlement -> test_native_ledger_withdrawal_reopen
    (signed asset-account-to-main-account SEND, committed balances, persisted
    producer records, two fresh consumers and duplicate refusal), plus
    bridge_withdraw_regression (historical codec/refusal recovery only).
fee-replay-private-wire -> test_kernel_fee_log_reopen (signed kernel records).
negative-fuzz-count -> test_smoke_iteration_bounds, test_smoke_invalid_cli,
    test_smoke_positive_workload (default suite runs all three; requested
    positive workloads of 1, 17 and 100000 are counted exactly).
manifest-bootstrap-anchor -> check_version (merged v3, NULL registration).
module-table-bootstrap-anchor -> check_kernel (merged signed genesis/snapshot),
    check_allowance_policy (occupancy downgrade rejects zero guarantors as
    noncanonical; its historical-guarantor mirror reaches version refusal).

The narrowed verify_cmd executes all seven C binaries and this Python file.
Wiring request, domains 1/15, modules/layerxledger/keeper and sdk/x/bank/keeper:
expose the production bank-send-to-derived-subaccount and withdrawal path to
this C-only gate; its kernel ledger proofs do not execute Go bank handlers.
Wiring request, domain 6, precompiles/layerxbridge: expose the production
bridgeOut bank debit, emitted nonce and restart duplicate rejection to this gate.
Wiring request, domain 1, src/modules/bridge/lxp_bridge_deposit.c: remove the
unreachable post-finality compatibility guards with their retired producer;
the checkpoint registry refuses construction, so malformed legacy inputs here
prove only refusal with no balance, staged-write or receipt mutation.
Wiring request, domain 16, daemon/paxd/cmd: qualify genesis against the real node;
this gate forbids native artifacts/toolchains and uses committed node transcripts.
Baseline: the supplied attempt-1 evidence records six failures in the aggregate
Python suite (gotchas 3043/3045), outside this task's paths, and no C executions.
No baseline rerun: the ordered bounded gates require implementation first.
Continuation baseline: supplied attempt-3 evidence at revision
6629cd4ff26ac002ecf7421046e2a4f97d83933f reports 25 of 26 C cases passing;
genesis.module-table.allowance-policy alone failed on the zero-guarantor
occupancy downgrade (gotcha 3066). Both exact refusal codes are now asserted
in that case; the native bank and bridgeOut wiring requests above stay open.
Qualification repair: candidate d59d9b22e105b7f82de367fc5144c0be17ab5612,
cg spec done 22.33.891 -f layerx-merge --root /work/paxeer-monorepo,
exit 1, /out/logs/verify-1.log: the funding fixture supplied a 2 KiB arena
to the production activity encoder, which reserves LXP_MAX_ACTIVITY_BYTES;
activity, receipt and fuzz scratch arenas now satisfy the production codec
contract, while canonical output and persisted receipt limits remain checked.
Setup finding: revision 5ddcda8bb07ffee04c4d14185497a2e17c66c7e5,
cg spec done 22.33.891 -f layerx-merge --root /work/paxeer-monorepo,
exit 1, /out/logs/verify-1.log: pytest was absent before collection; installed
the runner dependency without changing a repository manifest.
No gate result is claimed by this record; the runner retains revision and logs.
"""

import base64
import hashlib
import importlib.util
import json
import os
import sys
import tempfile
from pathlib import Path
import unittest

ROOT = Path(__file__).resolve().parents[1]


def load(name, path):
    spec = importlib.util.spec_from_file_location(name, ROOT / path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


FEES = load('genesis_fees', 'services/platform/hosted/node/genesis_fees.py')
METADATA = load('lxgb_metadata', 'tests/support/lxgb_metadata.py')


class WithdrawalGenesisTest(unittest.TestCase):
    def test_explicit_price_preserves_records_and_existing_fee_components(self):
        legacy = METADATA.check()
        expanded = FEES.withdrawal_metadata(legacy, 17)
        self.assertEqual(expanded, METADATA.metadata_withdrawal(
            METADATA.VECTOR_ASSET, METADATA.VECTOR_ISSUER_PUBLIC, METADATA.VECTOR_SALT, 17))
        self.assertEqual(expanded[-255:], (ROOT / 'tests/fixtures/fee-params-v3.bin').read_bytes())
        self.assertEqual(expanded[:-257], legacy[:-249])
        self.assertEqual(expanded[-253:-169], legacy[-245:-161])
        self.assertEqual(expanded[-168:-8], legacy[-160:])
        self.assertEqual(FEES.withdrawal_metadata(expanded, 17), expanded)
        for price in (-1, 2**64, True, '17'):
            with self.subTest(price=price), self.assertRaises(ValueError):
                FEES.withdrawal_metadata(legacy, price)
        for malformed in (b'', legacy[:-1], legacy + b'\0', b'\0\0' + legacy[2:],
                          legacy[:-247] + b'\0\x01' + legacy[-245:]):
            with self.subTest(length=len(malformed)), self.assertRaises(ValueError):
                FEES.withdrawal_metadata(malformed, 17)
        with self.assertRaises(ValueError):
            FEES.withdrawal_metadata(expanded, 0)

    def test_public_bootstrap_and_real_builder_bind_v3_and_native_authority(self):
        # The fee metadata boundary remains a codec contract. Native startup
        # consumes paxd genesis.json, not an LXGB request or registration anchor.
        for price in (0, 17):
            original = METADATA.check()
            expanded = FEES.withdrawal_metadata(original, price)
            self.assertEqual(int.from_bytes(expanded[-8:], 'big'), price)
            self.assertEqual(expanded[:-257], original[:-249])
            self.assertEqual(FEES.withdrawal_metadata(expanded, price), expanded)
        with tempfile.TemporaryDirectory(prefix='paxd-genesis-') as directory:
            paths = sys.path[:]
            sys.path.insert(0, str(ROOT / 'services/platform/hosted/node/tests'))
            try:
                from native_key_harness import NativeKeyHarness, native, reset

                harness = NativeKeyHarness(Path(directory), recorded=True)
                process = None
                try:
                    binding = harness.bootstrap()
                    genesis_bytes = (harness.data / 'config/genesis.json').read_bytes()
                    genesis = json.loads(genesis_bytes)
                    self.assertEqual(genesis['chain_id'], 'hyperpax_125-1')
                    self.assertTrue(genesis['validators'])
                    self.assertTrue(genesis['app_state']['bank']['balances'])
                    self.assertEqual(genesis['app_state']['lxphost']['last_sequence'], 0)
                    self.assertEqual(binding['config/genesis.json'], hashlib.sha256(genesis_bytes).hexdigest())
                    public = bytes.fromhex(harness.config['receipt_public_key'])
                    pins = genesis['app_state']['sequencer']['receipt_keys']
                    self.assertTrue(any(
                        base64.b64decode(pin['public_key'], validate=True) == public
                        and base64.b64decode(pin['key_id'], validate=True) == hashlib.sha256(public).digest()
                        and pin['first_height'] == 1 and pin['last_height'] >= 1
                        for pin in pins))
                    commands = [json.loads(line) for line in harness.peer.journal.read_text().splitlines()]
                    self.assertEqual([row['command'][1] for row in commands], ['init', 'validate-genesis'])
                    self.assertTrue(all(row['exit'] == 0 for row in commands))
                    # Production broker and consumer open the producer's sealed
                    # generation twice, releasing every descriptor between reads.
                    for _ in range(2):
                        with harness.consume() as (result, adopted, view):
                            self.assertEqual((view / 'genesis.json').read_bytes(), genesis_bytes)
                            self.assertEqual(result['generation'], 1)
                            self.assertEqual(adopted['LAYERX_CORE_SEQUENCER_ID'], hashlib.sha256(public).hexdigest())
                            self.assertEqual(result['public']['LAYERX_NODE_SEQUENCER_PUBLIC_KEY'], public.hex())
                        self.assertFalse(view.exists())
                    args = native.parser().parse_args(harness.arguments('supervise'))
                    store = reset.Store(harness.state, harness.data, harness.run,
                                        harness.config['network_id'], {'owner': 'native-key-harness'}, os.getegid())
                    process = native.start_native(args, store, commands=harness.peer)
                    node = {'LAYERX_NODE_CONSENSUS_RPC_URL': harness.config['consensus_rpc_url'],
                            'LAYERX_NODE_RPC_URL': harness.config['chain_rpc_url'],
                            'LAYERX_NODE_CHAIN_ID': genesis['chain_id'],
                            'LAYERX_NODE_EVM_CHAIN_ID': '125'}
                    committed = native.committed_health(node)
                    self.assertGreater(committed['height'], 0)
                    native.stop_process(process)
                    process = native.start_native(args, store, commands=harness.peer)
                    self.assertEqual(native.committed_health(node), committed)
                    self.assertEqual(native.generation_binding(harness.data), binding)
                    # A reopened consumer must reject changed genesis bytes,
                    # even when the producer's retained generation is intact.
                    proof = harness.data / '.generation-proof/config/genesis.json'
                    retained = proof.read_bytes()
                    native.write_file(proof, retained + b'\n')
                    with self.assertRaises(reset.StoreError):
                        with harness.consume():
                            self.fail('tampered genesis reached the native consumer')
                    self.assertEqual(native.generation_binding(harness.data), binding)
                    native.write_file(proof, retained)
                    with harness.consume() as (_, _, view):
                        self.assertEqual((view / 'genesis.json').read_bytes(), genesis_bytes)
                finally:
                    native.stop_process(process)
                    harness.close()
            finally:
                sys.path[:] = paths


if __name__ == '__main__':
    unittest.main()

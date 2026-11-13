"""Task 22.33.863 repair record (acceptance 22998.1).

Finding proof-cli-positive-genesis is proven by
test_proof_cli_positive_genesis_success_and_reopen and
test_proof_cli_positive_genesis_refusals_and_recovery below. Both execute the
production native bootstrap, broker and client with the committed paxd transcript;
the historical ELF/LXGB refusal remains explicit in test_proof_qual_cli.py.

Baseline: no tests ran before implementation; the bounded gate order forbids a
manual verify run and this planned test module did not exist on the base tree.
Verified revision, command, exit and private log are supplied by hf-done's ledger.
Wiring request: native-chain owner, daemon/paxd/cmd/init.go and
daemon/paxd/cmd/root.go: qualify native executable validation and image wiring in
the owning gate; this gate uses the committed command/file and genesis transcript.
"""

import test_proof_qual_cli as proof


def test_proof_cli_positive_genesis_success_and_reopen(tmp_path):
    proof.test_paxd_genesis_producer_consumer_success_and_reopen(tmp_path)


def test_proof_cli_positive_genesis_refusals_and_recovery(tmp_path):
    proof.test_paxd_genesis_duplicate_and_tampered_reopen_refuse(tmp_path)

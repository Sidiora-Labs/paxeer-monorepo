#!/usr/bin/env python3
import base64
import hashlib
import importlib.util
import os
from pathlib import Path
import re
import ssl
import subprocess
import sys

sys.dont_write_bytecode = True
ROOT = Path(__file__).resolve().parents[3]
COMPLETE = "MIGRATION_FUNDED_OPERATOR_COMPLETE ethereum=1 solana=1 funded=2 pending=1 skipped=0"
spec = importlib.util.spec_from_file_location(
    "funded_operator_real_ramp", ROOT / "tools/qualification/paxeer-x/ramp-source-settlement.py")
ramp = importlib.util.module_from_spec(spec)
spec.loader.exec_module(ramp)


C = sys.modules.get('human_committed_projections')
if C is None:
    spec = importlib.util.spec_from_file_location('human_committed_projections', Path(__file__).with_name('human-canonical-projections.py'))
    C = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = C
    spec.loader.exec_module(C)
LEDGER = '0x' + '00' * 18 + '101a'


def verify_destination_record(value, order_digest, transaction_hash):
    C.require(value['verified'] is True and value['rung'] == 'committed' and value['chain_id'] == 125
              and value['order_digest'] == order_digest and value['transaction_hash'] == transaction_hash
              and re.fullmatch('[0-9a-f]{64}', value['receipt_digest'])
              and type(value['height']) is int and value['height'] > 0,
              'committed native destination receipt binding differs')
    return value


class DestinationReceiptConsumer:
    """Destination authority is the signed native payment and its committed block."""
    def __init__(self, node):
        self.node = node

    def verify(self, transaction_hash, binding):
        actual = self.node.committed(transaction_hash, owner=binding['operator_address'])
        transaction, _ = C.merged.evm.decode_signed(bytes.fromhex(actual['raw_transaction'][2:]))
        arguments = [bytes.fromhex(binding[name]) for name in ('from_account', 'to_account', 'asset')]
        arguments += [int(binding['amount']), binding['expires_at_ms'], bytes.fromhex(binding['order_digest'])]
        data = C.merged.evm.encode_call('payExact', ['bytes32', 'bytes32', 'bytes32', 'uint128', 'uint64', 'bytes32'], arguments)
        C.require(transaction.chain_id == 125 and transaction.to.hex() == LEDGER[2:]
                  and transaction.value == 0 and transaction.data == data,
                  'committed native funding amount account asset or order differs')
        receipt = self.node.call('eth_getTransactionReceipt', [transaction_hash])
        topic = '0x' + C.merged.CHAIN.keccak256(b'PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)').hex()
        logs = [log for log in receipt['logs'] if log['address'] == LEDGER and log['topics'][0] == topic]
        C.require(len(logs) == 1 and logs[0]['topics'] == [topic, '0x' + binding['order_digest'],
                  '0x' + binding['from_account'], '0x' + binding['to_account']]
                  and C.merged.decode_abi(['bytes32', 'uint128'], logs[0]['data'])
                  == [bytes.fromhex(binding['asset']), int(binding['amount'])],
                  'committed native funding log differs from bound order')
        value = {'verified': True, 'rung': 'committed', 'chain_id': 125,
                 'order_digest': binding['order_digest'], 'transaction_hash': transaction_hash,
                 'receipt_digest': actual['receipt_digest'], 'height': int(actual['block_number'], 16),
                 'block_hash': actual['block_hash']}
        return verify_destination_record(value, binding['order_digest'], transaction_hash)

    def observe(self, source, transaction_hash, binding):
        C.require(source['version'] == ramp.VERSION and source['order_digest'] == list(bytes.fromhex(binding['order_digest']))
                  and len(ramp.byte32(source['source_evidence_digest'])) == 32
                  and source['state'] in ('source_pending', 'source_settled', 'layerx_pending', 'layerx_refused', 'done'),
                  'independent external source claim binding required')
        if source['state'] == 'source_pending':
            C.require(source['source_claim_id'] is None, 'unfinalized source cannot consume a claim')
        else:
            ramp.byte32(source['source_claim_id'])
        if source['state'] != 'done':
            return {'source': source, 'destination': None}
        C.require(source['source_claim_id'] is not None, 'funding requires a separate finalized source claim')
        return {'source': source, 'destination': self.verify(transaction_hash, binding)}


def current_destination_receipt(contract, case, order, activity, output, expected_success=True):
    transaction_hash = '0x' + bytes(activity).hex()
    args = [contract.binaries['receipt_verifier'], '--config-file', contract.fixture['receipt_config_file'],
            '--order-file', case['order_file'], '--transaction-hash', transaction_hash, '--output-file', str(output)]
    result = subprocess.run(args, cwd=ROOT, capture_output=True, timeout=120)
    if not expected_success:
        ramp.require(result.returncode != 0 and not output.exists(), 'forged native receipt authority accepted')
        return
    ramp.require(result.returncode == 0, 'independent committed native funding verification refused')
    return verify_destination_record(ramp.document(output), bytes(order['order_digest']).hex(), transaction_hash)



class FundedOperatorContract(ramp.Contract):
    def receipt(self, case, order, activity, output, expected_success=True):
        return current_destination_receipt(self, case, order, activity, output, expected_success)

    def __init__(self, fixture):
        ramp.closed(fixture, ("version", "execution_domain", "ramp_fixture_file", "migrate_cli",
                              "gateway_config_file", "other_gateway_config_file"), "funded operator fixture")
        ramp.require(fixture["version"] == "layerx-migration-funded-operator-fixture-v2"
                     and fixture["execution_domain"] == "isolated-real-process",
                     "isolated owner funded operator fixture required")
        super().__init__(ramp.document(fixture["ramp_fixture_file"]))
        self.cli = ramp.artifact(fixture["migrate_cli"], self.fixture["source_revision"])
        launched_cli = os.environ.get("LAYERX_MIGRATE_BIN")
        ramp.require(launched_cli and Path(launched_cli).resolve() == Path(self.cli),
                     "declared artifact must be the actual Rust-built operator")
        self.gateway_configs = {"customer": fixture["gateway_config_file"],
                                "other_customer": fixture["other_gateway_config_file"]}
        for authority, path in self.gateway_configs.items():
            config = ramp.document(path)
            ramp.closed(config, ("endpoint", "ca_certificate_der", "customer_authorization_file",
                                 "connect_timeout_ms", "request_timeout_ms", "maximum_response_bytes"),
                        "operator gateway configuration")
            ramp.require(config["endpoint"].rstrip("/") == self.interop_base.rstrip("/"),
                         "operator must connect to the actual isolated consumer")
            ramp.require(ramp.credential(config["customer_authorization_file"]) == "Bearer " + self.auth[authority],
                         "operator authority must match the genuine customer")
            certificate = ramp.private(config["ca_certificate_der"], 256 * 1024)
            pem = ramp.private(self.fixture["tls_ca_file"], 256 * 1024).decode("ascii")
            ramp.require(certificate == ssl.PEM_cert_to_DER_cert(pem), "actual consumer TLS authority mismatch")
            for field, lower, upper in (("connect_timeout_ms", 100, 30000),
                                        ("request_timeout_ms", 100, 120000),
                                        ("maximum_response_bytes", 1024, 2 * 1024 * 1024)):
                ramp.require(type(config[field]) is int and lower <= config[field] <= upper,
                             "bounded operator transport required")
        self.cli_initial = set()
        self.cli_terminal = set()
        self.cli_pending = False

    def invoke(self, case, digest, idempotency, authority="customer", refused=False):
        result = subprocess.run([self.cli, "migrate-asset", "--chain", case["chain"],
                                 "--evidence", case["evidence_file"], "--gateway-config",
                                 self.gateway_configs[authority], "--order-digest", bytes(digest).hex(),
                                 "--idempotency", idempotency], cwd=ROOT, capture_output=True, timeout=130)
        ramp.require(not result.stderr and len(result.stdout) <= ramp.MAX_FILE,
                     "bounded typed operator output required")
        import json
        document = json.loads(result.stdout)
        if refused:
            ramp.require(result.returncode == 1 and document.get("ok") is False
                         and document.get("error", {}).get("code") == "plane_refused",
                         "actual owner refusal must survive the operator boundary")
            return document
        ramp.closed(document, ("ok", "result", "trace"), "operator success")
        ramp.require(result.returncode == 0 and document["ok"] is True
                     and isinstance(document["trace"], str) and document["trace"],
                     "actual migration operator refused")
        observed = document["result"]
        ramp.closed(observed, ("state", "producer_state", "order_digest", "operation", "source_evidence_digest",
                               "source_claim_id", "provenance", "layerx_receipt", "layerx_credit_verified"),
                    "operator observation")
        proof = ramp.private(case["evidence_file"], 1024 * 1024)
        ramp.require(observed["state"] == "producer-observed" and observed["order_digest"] == digest
                     and observed["producer_state"] in ("source_pending", "source_settled", "layerx_pending",
                                                         "layerx_refused", "done")
                     and observed["source_evidence_digest"] == list(hashlib.sha256(proof).digest())
                     and observed["provenance"] == "external-custody"
                     and observed["layerx_receipt"] is False and observed["layerx_credit_verified"] is False,
                     "operator source binding or receipt-authority claim refused")
        operation = observed["operation"]
        ramp.require(isinstance(operation, str) and operation.isascii() and 0 < len(operation) <= 256
                     and "/" not in operation, "real bounded operation required")
        if observed["producer_state"] == "source_pending":
            ramp.require(observed["source_claim_id"] is None, "unfinalized operator source claimed custody")
        else:
            ramp.byte32(observed["source_claim_id"])
        status, response = super().http("GET", "/v1/operations/" + operation,
                                        authority=authority, base=self.interop_base)
        ramp.require(status in (200, 202) and response.get("ok") is True and response.get("operation") == operation,
                     "actual operator operation cannot be resumed")
        payload = response["result"]
        source = payload["source_settlement"]
        ramp.require(source["version"] == ramp.VERSION and source["order_digest"] == digest
                     and source["state"] == observed["producer_state"]
                     and source["source_evidence_digest"] == observed["source_evidence_digest"]
                     and source["source_claim_id"] == observed["source_claim_id"]
                     and payload["provenance"] == "external-custody" and payload["layerx_receipt"] is False,
                     "operator differs from genuine producer observation")
        return observed, status, response

    def http(self, method, path, body=None, authority=None, customer=None, base=None, idempotency=None):
        if method == "POST" and path == "/v2/migration/assets" and base == self.interop_base:
            ramp.require(authority == "customer" and customer is None and idempotency,
                         "baseline operator boundary requires genuine customer and replay key")
            matches = [case for case in self.fixture["accepted"] if case["chain"] == body["chain"]
                       and base64.b64encode(ramp.private(case["evidence_file"], 1024 * 1024)).decode("ascii")
                       == body["source_evidence"]]
            ramp.require(len(matches) == 1, "genuine immutable accepted source capture required")
            observed, status, response = self.invoke(matches[0], body["order_digest"], idempotency)
            ramp.require(observed["producer_state"] == "source_settled",
                         "initial operator source must precede funded LayerX transition")
            self.cli_initial.add(matches[0]["chain"])
            self.passed(matches[0]["chain"] + "-cli-authenticated-source-before-funding")
            return status, response
        return super().http(method, path, body, authority, customer, base, idempotency)

    def accepted_case(self, case):
        super().accepted_case(case)
        digest = ramp.document(case["order_file"])["order_digest"]
        before = self.journal()
        key = "operator-terminal-" + bytes(digest).hex()
        original = None
        for replay_key in (key, key, key + "-fresh"):
            observed, _, _ = self.invoke(case, digest, replay_key)
            ramp.require(observed["producer_state"] == "done" and self.journal() == before,
                         "funded operator replay caused a second debit or lost terminal observation")
            if original is None:
                original = observed
            if replay_key == key:
                ramp.require(observed == original, "identical operator replay changed its observation")
        self.invoke(case, digest, key + "-other-owner", "other_customer", refused=True)
        ramp.require(self.journal() == before, "other customer mutated funded operator settlement")
        status, refused = super().http("GET", "/v1/operations/" + original["operation"],
                                      authority="other_customer", base=self.interop_base)
        ramp.require(status in (403, 404) and refused.get("ok") is False,
                     "other customer read the operator operation")
        self.cli_terminal.add(case["chain"])
        self.passed(case["chain"] + "-cli-funded-replay-and-owner-isolation")

    def pending_case(self, case):
        super().pending_case(case)
        digest = ramp.document(case["order_file"])["order_digest"]
        before = self.journal()
        for key in ("operator-pending", "operator-pending", "operator-pending-fresh"):
            observed, _, _ = self.invoke(case, digest, key)
            ramp.require(observed["producer_state"] == "source_pending"
                         and observed["source_claim_id"] is None and self.journal() == before,
                         "unfinalized operator replay consumed a claim or appended liability")
        self.cli_pending = True
        self.passed("cli-unfinalized-source-no-credit-and-replay")

    def run(self):
        super().run()
        ramp.require(self.cli_initial == {"ethereum", "solana"}
                     and self.cli_terminal == {"ethereum", "solana"} and self.cli_pending,
                     "complete actual operator source/funded/pending corpus required")


def corpus():
    contract = None
    try:
        ramp.require(os.environ.get("LAYERX_MIGRATION_FUNDED_OPERATOR_RUST_LAUNCH") == "1",
                     "funded corpus must be launched by the declared Rust test")
        path = os.environ.get("LAYERX_MIGRATION_FUNDED_OPERATOR_FIXTURE")
        ramp.require(path, "genuine funded operator owner fixture required")
        contract = FundedOperatorContract(ramp.document(path))
        contract.run()
        print(COMPLETE)
        print(f"PAXEER_X_GATE tests={contract.count} skipped=0")
        return 0
    except (ramp.Refusal, OSError, ValueError, KeyError, TypeError, subprocess.SubprocessError):
        print("migration-funded-operator: genuine funded qualification refused", file=sys.stderr)
        print(f"PAXEER_X_GATE tests={contract.count if contract else 0} skipped=0")
        return 1
    finally:
        if contract is not None:
            contract.close()


def gate():
    try:
        ramp.require(len(sys.argv) == 1, "declared funded operator gate takes no arguments")
        explicit = os.environ.get("LAYERX_MIGRATION_FUNDED_OPERATOR_TEST_BIN")
        target = Path(os.environ.get("CARGO_TARGET_DIR", "/root/lx-target/interop"))
        binaries = ([Path(explicit)] if explicit else
                    [path for path in (target / "debug/deps").glob("funded_operator-*")
                     if path.is_file() and os.access(path, os.X_OK)])
        ramp.require(len(binaries) == 1 and binaries[0].is_absolute()
                     and binaries[0].is_file() and os.access(binaries[0], os.X_OK),
                     "one actual declared Rust funded operator artifact required")
        result = subprocess.run([str(binaries[0]), "--nocapture", "--test-threads=1"],
                                cwd=ROOT, capture_output=True, text=True, timeout=550)
        print(result.stdout, end="")
        ramp.require(result.returncode == 0 and "6 passed; 0 failed; 0 ignored" in result.stdout
                     and result.stdout.splitlines().count(COMPLETE) == 1
                     and re.search(r"^MIGRATION_FUNDED_OPERATOR_CORPUS tests=[1-9][0-9]* skipped=0$",
                                   result.stdout, re.MULTILINE),
                     "all declared real operator tests and funded corpus must pass without skips")
        print("PAXEER_X_GATE tests=6 skipped=0")
        return 0
    except (ramp.Refusal, OSError, ValueError, subprocess.SubprocessError):
        print("migration-funded-operator: declared real Rust gate refused", file=sys.stderr)
        print("PAXEER_X_GATE tests=0 skipped=0")
        return 1


if __name__ == "__main__":
    sys.exit(corpus() if sys.argv[1:] == ["--corpus"] else gate())

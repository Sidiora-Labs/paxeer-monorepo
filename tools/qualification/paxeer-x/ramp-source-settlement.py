#!/usr/bin/env python3
import base64
import hashlib
import json
import os
from pathlib import Path
import ssl
import stat
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request

sys.dont_write_bytecode = True
sys.path.insert(0, str(Path(__file__).parent))
import merged_qualification as merged

ROOT = Path(__file__).resolve().parents[3]
VERSION = "layerx-migration-source-v2"
FIXTURE_VERSION = "layerx-ramp-source-settlement-fixture-v2"
MAX_FILE = 2 * 1024 * 1024


class Refusal(Exception):
    pass


def require(condition, message):
    if not condition:
        raise Refusal(message)


def closed(value, fields, name):
    require(isinstance(value, dict) and set(value) == set(fields), name + " schema refused")
    return value


def private(path, maximum=MAX_FILE):
    path = Path(path)
    require(path.is_absolute() and path.resolve() == path, "absolute canonical input required")
    flags = os.O_RDONLY | os.O_NOFOLLOW
    descriptor = os.open(path, flags)
    try:
        info = os.fstat(descriptor)
        require(stat.S_ISREG(info.st_mode) and info.st_uid == os.geteuid()
                and info.st_nlink == 1 and not info.st_mode & 0o077
                and 0 < info.st_size <= maximum, "protected input refused")
        with os.fdopen(descriptor, "rb", closefd=False) as handle:
            data = handle.read(maximum + 1)
        require(0 < len(data) <= maximum, "input bound refused")
        return data
    finally:
        os.close(descriptor)


def document(path):
    return json.loads(private(path))


def credential(path):
    value = private(path, 8192).decode("utf-8").strip()
    require(value and all(ord(character) >= 32 and character not in "\r\n"
                          for character in value), "credential refused")
    return value


def artifact(record, revision):
    closed(record, ("path", "sha256", "source_revision"), "artifact")
    path = Path(record["path"])
    require(path.is_absolute() and path.resolve() == path and path.is_file()
            and os.access(path, os.X_OK), "real executable required")
    require(record["source_revision"] == revision, "executable revision mismatch")
    with path.open("rb") as handle:
        digest = hashlib.file_digest(handle, "sha256").hexdigest()
    require(digest == record["sha256"], "executable digest mismatch")
    return str(path)


def byte32(value):
    require(isinstance(value, list) and len(value) == 32
            and all(type(part) is int and 0 <= part <= 255 for part in value)
            and any(value), "nonzero digest required")
    return value


def write_private(path, value):
    data = json.dumps(value, separators=(",", ":")).encode() + b"\n"
    descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(descriptor, "wb") as handle:
        handle.write(data)
        handle.flush()
        os.fsync(handle.fileno())


def order_binding(order):
    """Recompute the maintained RampOrder digest before trusting quote context."""
    quote = order['quote']
    require(quote['direction'] == 'on_ramp' and order['payer_grant'] is None
            and order.get('payer_grant_purpose') is None,
            'operator-funded source settlement order required')
    require(order['context'] == quote['context'], 'order and quote context differ')
    def text(value):
        require(isinstance(value, str) and value, 'nonempty canonical order field required')
        return value.encode('utf-8')
    def integer(value, size):
        require(type(value) is int and 0 <= value < 1 << (size * 8), 'canonical order integer required')
        return value.to_bytes(size, 'big')
    values = [text(quote['direction']), text(order['order_id']), text(quote['quote_id']),
              text(order['customer']['principal_id']), text(order['customer']['account']),
              text(order['operator']['principal_id']), text(order['operator']['account']),
              text(order['operator']['signer_key_handle']), bytes(byte32(quote['layerx_asset'])),
              integer(quote['layerx_amount'], 16), text(quote['external_currency']),
              integer(quote['external_amount_minor'], 16), integer(quote['rate_numerator'], 16),
              integer(quote['rate_denominator'], 16), integer(quote['fee_minor'], 16),
              integer(quote['maximum_slippage_bps'], 2), bytes(byte32(order['context'])),
              text(quote['provider_token']), text(quote['payout_token']), integer(quote['expires_at'], 8), b'', b'']
    encoded = b'LXP/market-maker-ramp/order/v1\0' + b''.join(
        len(value).to_bytes(16, 'big') + value for value in values)
    require(hashlib.sha256(encoded).digest() == bytes(byte32(order['order_digest'])),
            'order digest does not bind its context and principals')
    return {'order_digest': bytes(order['order_digest']), 'context': bytes(order['context']),
            'from': account_id(order['operator']['account']), 'to': account_id(order['customer']['account']),
            'asset': bytes(quote['layerx_asset']), 'amount': quote['layerx_amount'],
            'expires_at': quote['expires_at'] * 1000}


def committed_destination(node, transaction_hash, expected):
    closed(expected, ('order_digest', 'context', 'from', 'to', 'asset', 'amount', 'expires_at'),
           'committed destination binding')
    for name in ('order_digest', 'context', 'from', 'to', 'asset'):
        require(isinstance(expected[name], bytes) and len(expected[name]) == 32 and any(expected[name]),
                'nonzero destination binding required')
    require(type(expected['amount']) is int and 0 < expected['amount'] < 1 << 128
            and type(expected['expires_at']) is int and 0 < expected['expires_at'] < 1 << 64,
            'destination amount or expiry refused')
    proof = node.committed(transaction_hash)
    transaction, _ = merged.evm.decode_signed(bytes.fromhex(proof['raw_transaction'][2:]))
    address = '0x' + '00' * 18 + '101a'
    method = merged.abi_method('layerxledger', 'payExact')
    types = [node.abi_type(row) for row in method['inputs']]
    arguments = [expected[name] for name in ('from', 'to', 'asset', 'amount', 'expires_at', 'order_digest')]
    require(transaction.chain_id == 125 and transaction.value == 0
            and transaction.to.hex() == address[2:]
            and transaction.data == merged.evm.encode_call('payExact', types, arguments),
            'committed destination transaction does not bind the order')
    row = node.call('pax_sequencerReceipt', [transaction_hash])
    # Authenticate this exact observation too; do not trust a second, mutable JSON log view.
    merged.CHAIN._EXECUTION.verified_execution(row, bytes.fromhex(proof['raw_transaction'][2:]),
                                             {'receipt_keys': node.receipt_keys})
    topic = '0x' + merged.CHAIN.keccak256(b'PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)').hex()
    payments = [log for log in row['logs'] if log['address'] == address and log['topics']
                and log['topics'][0] == topic]
    require(len(payments) == 1 and payments[0]['topics'] == [topic, '0x' + expected['order_digest'].hex(),
            '0x' + expected['from'].hex(), '0x' + expected['to'].hex()]
            and merged.decode_abi(['bytes32', 'uint128'], payments[0]['data'])
            == [expected['asset'], expected['amount']], 'committed destination transfer event mismatch')
    return {'verified': True, 'rung': 'committed', 'chain_id': 125,
            'order_digest': expected['order_digest'].hex(), 'context': expected['context'].hex(),
            'from': expected['from'].hex(), 'to': expected['to'].hex(),
            'asset': expected['asset'].hex(), 'amount': str(expected['amount']), **proof}


def destination_receipt(node, transaction_hash, expected, output):
    evidence = committed_destination(node, transaction_hash, expected)
    output = Path(output)
    try:
        write_private(output, evidence)
    except FileExistsError:
        require(document(output) == evidence, 'recovered destination proof changed')
    return evidence


def account_id(name):
    require(isinstance(name, str) and name.startswith('agent:') and name.endswith(':main')
            and all(character in 'abcdefghijklmnopqrstuvwxyz0123456789._-:' for character in name),
            'canonical native main account required')
    raw = name.encode('ascii')
    return hashlib.sha256(b'LX:ACCOUNT:v1' + len(raw).to_bytes(4, 'big') + raw).digest()


class Contract:
    def __init__(self, fixture):
        closed(fixture, ("version", "execution_domain", "authorize_funded_layerx",
                         "source_revision", "artifacts", "ramp_config_file", "tls_ca_file",
                         "migration_authorization_file", "operator_authorization_file",
                         "customer_authorization_file", "other_customer_authorization_file",
                         "receipt_config_file", "accepted", "pending", "refused", "expected_did",
                         "interop_url", "interop_environment_file"), "fixture")
        require(fixture["version"] == FIXTURE_VERSION
                and fixture["execution_domain"] == "isolated-real-process"
                and fixture["authorize_funded_layerx"] is True,
                "owner isolated funded fixture authorization required")
        revision = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT,
                                           text=True).strip()
        require(fixture["source_revision"] == revision, "fixture candidate revision mismatch")
        closed(fixture["artifacts"], ("reference_ramp", "interop_service"), "artifacts")
        self.binaries = {name: artifact(record, revision)
                         for name, record in fixture["artifacts"].items()}
        self.fixture = fixture
        self.config = document(fixture["ramp_config_file"])
        self.auth = {"migration": credential(fixture["migration_authorization_file"]),
                     "operator": credential(fixture["operator_authorization_file"]),
                     "customer": credential(fixture["customer_authorization_file"]),
                     "other_customer": credential(fixture["other_customer_authorization_file"])}
        require(self.auth["customer"] != self.auth["other_customer"], "different customer required")
        require(self.auth["migration"] != self.auth["operator"], "dedicated migration authority required")
        self.work = Path(tempfile.mkdtemp(prefix="layerx-source-settlement-"))
        self.work.chmod(0o700)
        self.child = None
        self.interop_child = None
        self.interop_log = None
        self.log = None
        self.count = 0
        self.saw_uncertain = False
        self.context = ssl.create_default_context(cafile=fixture["tls_ca_file"])
        address = self.config["listen"].split(":")
        require(len(address) == 2 and address[0] == "127.0.0.1" and 0 < int(address[1]) < 65536,
                "isolated fixed loopback producer listener required")
        self.port = int(address[1])
        self.base = "https://localhost:" + str(self.port)
        self.interop_base = fixture["interop_url"]
        parsed = urllib.parse.urlsplit(self.interop_base)
        require(parsed.scheme == "https" and parsed.hostname in ("localhost", "127.0.0.1")
                and parsed.port is not None and not parsed.username and not parsed.password
                and parsed.path in ("", "/") and not parsed.query and not parsed.fragment,
                "isolated actual interop HTTPS listener required")
        self.interop_env = document(fixture["interop_environment_file"])
        require(isinstance(self.interop_env, dict) and self.interop_env
                and all(key.startswith("LAYERX_INTEROP_") and isinstance(value, str)
                        for key, value in self.interop_env.items()), "interop configuration environment refused")
        require(self.interop_env.get("LAYERX_INTEROP_LISTEN") == "127.0.0.1:" + str(parsed.port),
                "interop configured listener mismatch")
        profile = document(self.interop_env["LAYERX_INTEROP_MIGRATION_V2_CONFIG"])
        require(profile["ramp_intake"]["endpoint"].rstrip("/") == self.base,
                "actual interop consumer is not connected to this producer")
        require(isinstance(fixture["expected_did"], str) and len(fixture["expected_did"]) == 64
                and bytes.fromhex(fixture["expected_did"]) != bytes(32), "authentic expected signer DID required")
        self.config["listener"] = "tls"
        self.config["journal_path"] = str(self.work / "journal.jsonl")
        self.config["reconcile_seconds"] = 3600
        self.config_path = self.work / "ramp.json"
        write_private(self.config_path, self.config)

    def passed(self, name):
        self.count += 1
        print("PASS " + name, flush=True)

    def start(self):
        self.log = (self.work / ("producer-" + str(time.time_ns()) + ".log")).open("wb")
        self.child = subprocess.Popen([self.binaries["reference_ramp"], str(self.config_path)],
                                      cwd=ROOT, stdout=self.log, stderr=subprocess.STDOUT)
        deadline = time.monotonic() + 30
        while time.monotonic() < deadline:
            require(self.child.poll() is None, "real producer startup refused; retained log " + str(self.log.name))
            try:
                status, body = self.http("GET", "/readyz")
                if status in (200, 503) and isinstance(body, dict) and "ready" in body:
                    return
            except (OSError, urllib.error.URLError):
                pass
            time.sleep(0.1)
        raise Refusal("real producer readiness timeout")

    def stop(self):
        if self.child is not None:
            self.child.terminate()
            try:
                self.child.wait(timeout=10)
            except subprocess.TimeoutExpired:
                self.child.kill()
                self.child.wait(timeout=10)
            self.child = None
        if self.log is not None:
            self.log.close()
            self.log = None

    def start_interop(self):
        self.interop_log = (self.work / "interop-service.log").open("wb")
        environment = {key: value for key, value in os.environ.items()
                       if not key.startswith("LAYERX_INTEROP_")}
        environment.update(self.interop_env)
        self.interop_child = subprocess.Popen([self.binaries["interop_service"]], cwd=ROOT,
                                             env=environment, stdout=self.interop_log,
                                             stderr=subprocess.STDOUT)
        deadline = time.monotonic() + 30
        while time.monotonic() < deadline:
            require(self.interop_child.poll() is None,
                    "actual interop startup refused; retained log " + str(self.interop_log.name))
            try:
                status, _ = self.http("GET", "/livez", base=self.interop_base)
                if status == 200:
                    return
            except (OSError, urllib.error.URLError):
                pass
            time.sleep(0.1)
        raise Refusal("actual interop startup timeout")

    def close(self):
        self.stop()
        if self.interop_child is not None:
            self.interop_child.terminate()
            try:
                self.interop_child.wait(timeout=10)
            except subprocess.TimeoutExpired:
                self.interop_child.kill()
                self.interop_child.wait(timeout=10)
        if self.interop_log is not None:
            self.interop_log.close()

    def http(self, method, path, body=None, authority=None, customer=None, base=None, idempotency=None):
        headers = {"Content-Type": "application/json"}
        if authority is not None:
            headers["Authorization"] = "Bearer " + self.auth[authority]
        if customer is not None:
            headers["X-LayerX-Customer-Authorization"] = "Bearer " + self.auth[customer]
            headers["X-LayerX-Expected-Did"] = self.fixture["expected_did"]
        if idempotency is not None:
            headers["Idempotency-Key"] = idempotency
        payload = None if body is None else json.dumps(body, separators=(",", ":")).encode()
        request = urllib.request.Request((base or self.base).rstrip("/") + path, data=payload, headers=headers, method=method)
        try:
            response = urllib.request.urlopen(request, context=self.context, timeout=20)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            raw = response.read(MAX_FILE + 1)
            require(len(raw) <= MAX_FILE, "producer response bound refused")
            return response.status, json.loads(raw)

    def work_order(self, order, action, sequence=None):
        status, response = self.http("POST", "/internal/v1/work", {
            "order_digest": order["order_digest"], "action": action,
            "account_sequence": sequence, "canonical_receive_payload": None}, "operator")
        require(status == 200, "actual worker transition refused: " + action)
        return response

    def order(self, digest):
        status, response = self.http("GET", "/v1/orders/" + bytes(digest).hex(), authority="customer")
        require(status == 200, "bound order read refused")
        return response

    def journal(self):
        return Path(self.config["journal_path"]).read_bytes()

    def settle_request(self, case, order):
        evidence = private(case["evidence_file"], 1024 * 1024)
        return {"version": VERSION, "order_digest": order["order_digest"],
                "chain": case["chain"], "source_evidence": base64.b64encode(evidence).decode("ascii")}

    def setup_order(self, case):
        order = document(case["order_file"])
        byte32(order["order_digest"])
        status, response = self.http("POST", "/v1/orders", {
            "order_id": order["order_id"], "quote_id": order["quote"]["quote_id"],
            "payer_grant": order["payer_grant"]}, "customer")
        require(status == 201 and response["order_digest"] == order["order_digest"],
                "actual authenticated order creation mismatch")
        require(self.order(order["order_digest"])["order"] == order, "immutable bound order mismatch")
        self.work_order(order, "compliance")
        require(self.order(order["order_digest"])["stage"] == "awaiting_external_credit",
                "actual compliance approval required")
        return order

    def refusal(self, name, request, customer="customer", authority="migration"):
        before = self.journal()
        status, _ = self.http("POST", "/internal/v2/source-settlements", request, authority, customer)
        require(status in (400, 401, 403, 404, 409, 422), name + " was not refused")
        require(self.journal() == before, name + " mutated durable settlement")
        self.passed(name)

    def receipt(self, case, order, transaction_hash, output, expected_success=True):
        configuration = document(self.fixture['receipt_config_file'])
        closed(configuration, ('node_url', 'receipt_keys'), 'destination receipt verifier configuration')
        node = merged.CHAIN.Node(configuration['node_url'], configuration['receipt_keys'], context=self.context)
        expected = order_binding(order)
        try:
            evidence = (destination_receipt(node, transaction_hash, expected, output) if expected_success
                        else committed_destination(node, transaction_hash, expected))
        except (ValueError, Refusal):
            require(not expected_success and not output.exists(), 'genuine committed destination proof refused')
            return
        require(expected_success and evidence['rung'] == 'committed', 'forged receipt authority accepted')

    def accepted_case(self, case):
        closed(case, ("chain", "order_file", "evidence_file", "account_sequence"), "accepted case")
        require(case["chain"] in ("ethereum", "solana") and type(case["account_sequence"]) is int
                and case["account_sequence"] >= 0, "accepted case bounds refused")
        order = self.setup_order(case)
        request = self.settle_request(case, order)
        self.refusal(case["chain"] + "-migration-auth", request, authority="operator")
        self.refusal(case["chain"] + "-customer-auth", request, customer="other_customer")
        forged = dict(request, receipt_digest=[9] * 32)
        self.refusal(case["chain"] + "-forged-receipt-intake", forged)
        changed = dict(request, order_digest=[255] * 32)
        self.refusal(case["chain"] + "-wrong-order", changed)
        changed = dict(request, chain="solana" if case["chain"] == "ethereum" else "ethereum")
        self.refusal(case["chain"] + "-wrong-network", changed)
        public_request = {key: value for key, value in request.items() if key != "version"}
        status, result_document = self.http("POST", "/v2/migration/assets", public_request,
                                           "customer", base=self.interop_base,
                                           idempotency="source-" + bytes(order["order_digest"]).hex())
        require(status in (200, 202) and result_document["ok"] is True,
                "actual authenticated migration consumer refused finalized source")
        result_payload = result_document["result"]
        require(result_payload["provenance"] == "external-custody"
                and result_payload["layerx_receipt"] is False
                and result_payload["source_settlement"]["state"] == "source_settled",
                "consumer claimed wrong settlement stage or receipt authority")
        settled = self.order(order["order_digest"])
        require(settled["stage"] == "source_settled_v2"
                and settled["presentation"]["status"] != "done", "source finality claimed funded completion")
        before = self.journal()
        status, response = self.http("POST", "/internal/v2/source-settlements", request, "migration", "customer")
        require(status == 200 and response["state"] == "source_settled"
                and self.journal() == before, "source claim replay appended or changed state")
        self.passed(case["chain"] + "-finality-and-idempotent-source-claim")
        first_transfer = self.work_order(order, "submit_layerx", case["account_sequence"])
        if first_transfer["stage"] in ("layerx_submitted_unknown", "layerx_pending"):
            self.saw_uncertain = True
            require(first_transfer["presentation"]["status"] != "done", "uncertain funded transfer completed")
        deadline = time.monotonic() + 120
        while time.monotonic() < deadline:
            current = self.order(order["order_digest"])
            if current["stage"] == "done":
                break
            require(current["stage"] in ("layerx_submitted_unknown", "layerx_pending",
                                          "layerx_submission_planned", "layerx_verified"),
                    "funded transfer reached a refusal")
            require(current["presentation"]["status"] != "done", "uncertain transfer reported completion")
            if current["stage"] != "layerx_verified":
                self.work_order(order, "resolve_layerx")
            time.sleep(0.2)
        else:
            raise Refusal("real funded receipt remained uncertain")
        transaction_hash = current["presentation"]["transaction_hash"]
        self.receipt(case, order, transaction_hash, self.work / (case["chain"] + "-receipt.json"))
        invalid = "0x" + ("0" if transaction_hash[2] != "0" else "1") + transaction_hash[3:]
        self.receipt(case, order, invalid, self.work / (case["chain"] + "-forged.json"), False)
        before = self.journal()
        status, response = self.http("POST", "/internal/v2/source-settlements", request, "migration", "customer")
        require(status == 200 and response["state"] == "done" and self.journal() == before,
                "terminal replay caused a second debit or append")
        operation = result_document["operation"]
        require(isinstance(operation, str) and operation and "/" not in operation,
                "actual resumable operation required")
        status, resumed = self.http("GET", "/v1/operations/" + operation,
                                    authority="customer", base=self.interop_base)
        require(status == 200 and resumed["ok"] is True
                and resumed["result"]["source_settlement"]["state"] == "done"
                and resumed["result"]["layerx_receipt"] is False,
                "actual interop continuation did not observe independently funded completion")
        self.passed(case["chain"] + "-funded-independent-receipt-and-terminal-replay")
        self.stop()
        self.start()
        status, health = self.http("GET", "/readyz")
        require(status == 200 and health["ready"] is True, "automatic independent source and receipt recovery refused")
        require(self.order(order["order_digest"])["stage"] == "done", "durable V2 settlement lost on restart")
        self.receipt(case, order, transaction_hash, self.work / (case["chain"] + "-receipt.json"))
        before = self.journal()
        status, response = self.http("POST", "/internal/v2/source-settlements", request, "migration", "customer")
        require(status == 200 and response["state"] == "done" and self.journal() == before,
                "recovered claim replay caused a second debit")
        self.passed(case["chain"] + "-restart-independent-recovery")

    def pending_case(self, case):
        closed(case, ("chain", "order_file", "evidence_file"), "pending case")
        order = self.setup_order(case)
        request = self.settle_request(case, order)
        status, response = self.http("POST", "/internal/v2/source-settlements", request, "migration", "customer")
        require(status == 202 and response["state"] == "source_pending"
                and response["source_claim_id"] is None, "unfinalized source consumed a claim")
        require(self.order(order["order_digest"])["presentation"]["status"] != "done",
                "unfinalized liability reported completion")
        before = self.journal()
        self.http("POST", "/internal/v2/source-settlements", request, "migration", "customer")
        require(self.journal() == before, "identical pending replay duplicated liability")
        self.passed("unfinalized-source-retained-and-idempotent")

    def run(self):
        require(len(self.fixture["accepted"]) == 2
                and {case["chain"] for case in self.fixture["accepted"]} == {"ethereum", "solana"},
                "genuine finalized Ethereum and Solana cases required")
        self.start()
        self.start_interop()
        for case in self.fixture["refused"]:
            closed(case, ("name", "chain", "order_file", "evidence_file"), "refused case")
            order = self.setup_order(case)
            self.refusal(case["name"], self.settle_request(case, order))
        require({case["name"] for case in self.fixture["refused"]}
                == {"wrong-asset", "wrong-amount", "wrong-recipient", "wrong-custody"},
                "complete genuine source binding refusal cases required")
        for case in self.fixture["accepted"]:
            self.accepted_case(case)
        self.pending_case(self.fixture["pending"])
        require(self.saw_uncertain, "real pending or unknown funded transition was not exercised")


def main():
    contract = None
    try:
        path = os.environ.get("LAYERX_RAMP_SOURCE_SETTLEMENT_FIXTURE")
        require(path, "LAYERX_RAMP_SOURCE_SETTLEMENT_FIXTURE required; genuine owner source and funded fixtures absent")
        contract = Contract(document(path))
        contract.run()
        print(f"PAXEER_X_GATE tests={contract.count} skipped=0")
        return 0
    except (Refusal, OSError, ValueError, KeyError, TypeError, subprocess.SubprocessError) as error:
        print("ramp-source-settlement: refusal: " + str(error), file=sys.stderr)
        print(f"PAXEER_X_GATE tests={contract.count if contract else 0} skipped=0")
        return 1
    finally:
        if contract is not None:
            contract.close()


if __name__ == "__main__":
    sys.exit(main())

#!/usr/bin/env python3
import sys

sys.dont_write_bytecode = True

import argparse
import hashlib
import http.client
import http.server
import json
import os
from pathlib import Path
import shutil
import socket
import ssl
import subprocess
import tempfile
import threading
import time
import urllib.error
import urllib.parse
import urllib.request

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / "tools/paxeer-x"))
from candidate import SERVICE_CATALOGUE

sys.path.insert(0, str(Path(__file__).parent))
import merged_qualification as merged
import native_evidence as evidence

CASES = ("ramp-journal-readiness", "mirror-checkpoint-acquisition",
         "interop-settlement-readiness")


class Failure(Exception):
    pass


def require(condition, message):
    if not condition:
        raise Failure(message)


def now_ms():
    return int(time.time() * 1000)


def unused_port():
    with socket.socket() as connection:
        connection.bind(("127.0.0.1", 0))
        return connection.getsockname()[1]


def revision():
    result = subprocess.run(["git", "--no-optional-locks", "-C", str(ROOT), "rev-parse", "HEAD"],
                            capture_output=True, text=True, timeout=30)
    return result.stdout.strip() if result.returncode == 0 else "unknown"


class RampJournalContract:
    def __init__(self, manifest):
        import stat
        sys.path.insert(0, str(ROOT / "tools/paxeer-x"))
        from candidate import catalogue, load_private, validate
        self.private = load_private
        candidate = load_private(manifest)
        validate(candidate, catalogue(ROOT / SERVICE_CATALOGUE))
        self.revision = candidate["source"]["revision"]
        require(self.revision == revision(), "ramp candidate source revision does not match checkout")
        ramp = next((service for service in candidate["services"] if service["id"] == "ramp"), None)
        require(ramp is not None, "candidate ramp service absent")
        require(ramp["bindings"]["source_revision"] == self.revision, "ramp source binding absent")
        self.inputs = load_private(os.environ.get("PAXEER_X_RAMP_READINESS_INPUTS", ""))
        self.binary = Path(self.inputs["binary"])
        require(self.binary.is_file() and os.access(self.binary, os.X_OK), "ramp binary absent")
        require(hashlib.sha256(self.binary.read_bytes()).hexdigest() == self.inputs["binary_sha256"],
                "ramp binary digest mismatch")
        self.config = load_private(self.inputs["config"])
        config_digest = "sha256:" + hashlib.sha256(Path(self.inputs["config"]).read_bytes()).hexdigest()
        require(config_digest == ramp["bindings"]["config_digest"], "ramp configuration binding mismatch")
        require(self.config["paxeer"]["rpc_chain_id"] == candidate["foundation"]["chain_id"],
                "ramp chain pin differs from candidate")
        require(self.config["layerx"]["network_id"] == self.inputs["network_id"]
                and self.config["layerx"]["protocol_version"] == self.inputs["wire_version"],
                "explicit LayerX network or wire pin missing")
        self.tokens = {}
        for actor, key in (("customer", "customer_token_file"), ("operator", "operator_control_token_file")):
            path = Path(self.inputs[key] if actor == "customer" else self.config[key])
            info = path.lstat()
            require(stat.S_ISREG(info.st_mode) and info.st_uid == os.geteuid()
                    and info.st_nlink == 1 and info.st_mode & 0o077 == 0,
                    "ramp credential input is not private")
            self.tokens[actor] = path.read_text().strip()
            require(bool(self.tokens[actor]), "ramp credential empty")
        self.work = Path(tempfile.mkdtemp(prefix="ramp-journal-", dir=self.inputs["evidence_directory"]))
        self.work.chmod(0o700)
        self.processes = []
        self.cases = []
        self.port = unused_port()
        self.config.update(listen=f"127.0.0.1:{self.port}", listener="plain",
                           server_identity_pkcs12=None, server_identity_password_file=None)
        self.url = f"http://127.0.0.1:{self.port}"
        self.journal = self.work / "journal.jsonl"
        self.config["journal_path"] = str(self.journal)
        self.config_path = self.work / "config.json"
        self.save(self.config_path, self.config)
        self.process = None

    def save(self, path, value):
        with path.open("x") as stream:
            json.dump(value, stream)
        path.chmod(0o600)

    def request(self, path, body=None, actor="operator"):
        data = None if body is None else json.dumps(body).encode()
        request = urllib.request.Request(self.url + path, data=data,
                    headers={"Authorization": "Bearer " + self.tokens[actor], "Content-Type": "application/json"})
        try:
            with urllib.request.urlopen(request, timeout=30) as response:
                return response.status, json.load(response)
        except urllib.error.HTTPError as error:
            return error.code, json.loads(error.read())

    def start(self, limit=None):
        import resource
        import signal
        def restricted():
            resource.setrlimit(resource.RLIMIT_CORE, (0, 0))
            if limit is not None:
                signal.signal(signal.SIGXFSZ, signal.SIG_IGN)
                resource.setrlimit(resource.RLIMIT_FSIZE, (limit, limit))
        process = subprocess.Popen([str(self.binary), str(self.config_path)], stdin=subprocess.DEVNULL,
                     stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, preexec_fn=restricted)
        self.processes.append(process)
        self.process = process
        return process

    def stop(self):
        if self.process and self.process.poll() is None:
            self.process.terminate()
            try:
                self.process.wait(timeout=10)
            except subprocess.TimeoutExpired:
                self.process.kill()
                self.process.wait(timeout=10)

    def ready(self, expected):
        deadline = time.monotonic() + 90
        while time.monotonic() < deadline:
            require(self.process.poll() is None, "ramp exited before readiness observation")
            try:
                code, body = self.request("/readyz")
                if code == expected:
                    require(body["ready"] == (expected == 200), "readiness body disagrees with status")
                    return body
            except (OSError, ValueError):
                pass
            time.sleep(0.1)
        raise Failure("ramp readiness deadline")

    def passed(self, name, assertions):
        self.cases.append({"name": name, "assertions": assertions})

    def provider_callback_recovery(self):
        import stat
        callback = self.private(self.inputs["provider_callback"])
        source = Path(self.inputs["callback_journal"])
        info = source.lstat()
        require(stat.S_ISREG(info.st_mode) and info.st_uid == os.geteuid()
                and info.st_nlink == 1 and info.st_mode & 0o077 == 0,
                "callback journal input is not private")
        saved_config = dict(self.config)
        saved_path = self.config_path
        journal = self.work / "callback-recovery.jsonl"
        journal.write_bytes(source.read_bytes())
        journal.chmod(0o600)
        self.config = dict(self.config, journal_path=str(journal), reconcile_seconds=3600)
        self.config_path = self.work / "callback-recovery-config.json"
        self.save(self.config_path, self.config)
        digest = bytes(callback["result"]["order_digest"]).hex()
        self.start()
        self.ready(200)
        code, before = self.request("/v1/orders/" + digest, actor="customer")
        require(code == 200 and before["stage"] == "awaiting_external_credit",
                "real callback recovery order is not awaiting external credit")
        original = journal.read_bytes()
        for index in range(1, 9):
            if len(original) > len(json.dumps(callback).encode()) + 1024:
                break
            seed = dict(self.inputs["create_order"])
            seed["order_id"] += "-callback-prefix-" + str(index)
            code, _ = self.request("/v1/orders", seed, "customer")
            require(code == 201, "real callback journal prefix creation refused")
            original = journal.read_bytes()
        require(len(original) > len(json.dumps(callback).encode()) + 1024,
                "callback journal prefix too short for real OS interruption")
        self.stop()
        self.start(len(original) + 64)
        self.ready(200)
        code, _ = self.request("/v1/provider-callbacks", callback)
        require(code == 503, "signed provider callback append did not encounter OS refusal")
        self.ready(503)
        damaged = journal.read_bytes()
        require(damaged.startswith(original) and len(damaged) == len(original) + 64
                and not damaged.endswith(b"\n"), "signed callback append did not retain its exact torn prefix")
        intent_path = Path(str(journal) + ".append-intent")
        raw = intent_path.read_bytes()
        domain = b"LXP/market-maker-ramp/append-intent/v1\0"
        header = len(domain) + 12
        require(raw.startswith(domain) and len(raw) > header + 32
                and hashlib.sha256(raw[:-32]).digest() == raw[-32:], "provider pending intent invalid")
        length = int.from_bytes(raw[len(domain) + 8:header], "big")
        require(len(raw) == header + length + 32
                and int.from_bytes(raw[len(domain):len(domain) + 8], "big") == len(original),
                "provider pending intent window differs")
        pending = raw[header:-32]
        record = json.loads(pending)
        event = record["event"]["provider_callback_applied"]
        require(pending.startswith(damaged[len(original):])
                and event["callback_id"] == callback["callback_id"]
                and event["order_digest"] == callback["result"]["order_digest"]
                and event["provider_sequence"] == callback["provider_sequence"]
                and event["evidence"]["provider_operation_id"] == callback["result"]["operation_id"]
                and event["evidence"]["provider_evidence_digest"] == callback["result"]["evidence_digest"],
                "retained pending callback lost authenticated provider identity")
        code, _ = self.request("/v1/provider-callbacks", callback)
        require(code == 503 and journal.read_bytes() == damaged,
                "provider callback retry bypassed persistent halt")
        self.stop()
        self.start()
        self.ready(200)
        expected = original + pending
        require(journal.read_bytes() == expected and expected.startswith(damaged)
                and not intent_path.exists(), "provider recovery rewrote history or duplicated the event")
        code, recovered = self.request("/v1/orders/" + digest, actor="customer")
        require(code == 200 and recovered["stage"] == "provider_settled"
                and recovered["presentation"]["provider_evidence_digest"] == callback["result"]["evidence_digest"]
                and recovered["presentation"]["status"] == "pending",
                "recovery did not bind real provider settlement or falsely completed the LayerX leg")
        code, response = self.request("/v1/provider-callbacks", callback)
        require(code == 200 and response["accepted"] is True and journal.read_bytes() == expected,
                "recovered provider callback retry duplicated its event")
        callbacks = [json.loads(line)["event"].get("provider_callback_applied", {})
                     for line in journal.read_bytes().splitlines()]
        require(sum(event.get("callback_id") == callback["callback_id"] for event in callbacks) == 1,
                "recovered provider callback identity is not unique")
        self.passed("interrupted_provider_callback", ["real signed callback and OS append refusal",
                    "awaiting_external_credit retains exact callback request/provider identity",
                    "restart validates actual provider settlement before completing the missing bytes",
                    "one callback record and duplicate retry no-op", "LayerX leg remains pending"])
        self.stop()
        self.config = saved_config
        self.config_path = saved_path

    def run(self):
        self.start()
        self.ready(200)
        code, _ = self.request("/v1/orders", self.inputs["create_order"], "customer")
        require(code == 201, "real authenticated order creation refused")
        original = self.journal.read_bytes()
        require(original.endswith(b"\n") and bool(original), "real journal record absent")
        require(len(self.inputs["create_order"]["order_id"]) <= 80, "journal case order identity too long")
        for index in range(1, 9):
            if len(original) > len(original.splitlines(keepends=True)[-1]) + 1024:
                break
            seed = dict(self.inputs["create_order"])
            seed["order_id"] += "-journal-prefix-" + str(index)
            code, _ = self.request("/v1/orders", seed, "customer")
            require(code == 201, "real journal prefix order creation refused")
            original = self.journal.read_bytes()
        require(len(original) > len(original.splitlines(keepends=True)[-1]) + 1024,
                "durable journal prefix too short for the real append interruption")
        conflict = subprocess.run([str(self.binary), str(self.config_path)], stdin=subprocess.DEVNULL,
                    stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=90)
        require(conflict.returncode != 0 and self.journal.read_bytes() == original,
                "exclusive writer conflict failed to refuse or changed history")
        self.passed("exclusive_writer", ["second process nonzero", "history unchanged"])
        self.stop()
        self.start()
        self.ready(200)
        require(self.journal.read_bytes() == original, "verified restart changed journal history")
        self.passed("verified_restart", ["readyz=200", "history unchanged after replay and external checks"])
        code, _ = self.request("/internal/v1/journal/recover", {})
        require(code == 200, "explicit verified recovery refused")
        self.passed("explicit_recovery", ["authenticated recovery=200", "external settlement verified"])
        self.stop()
        self.start(len(original) + 64)
        self.ready(200)
        interrupted = dict(self.inputs["create_order"])
        interrupted["order_id"] += "-interrupted"
        code, _ = self.request("/v1/orders", interrupted, "customer")
        require(code == 503, "OS interrupted append did not return durable refusal")
        state = self.ready(503)
        damaged = self.journal.read_bytes()
        require(damaged.startswith(original) and len(damaged) > len(original)
                and not damaged.endswith(b"\n"), "OS interrupted append did not retain a torn tail")
        intent_path = Path(str(self.journal) + ".append-intent")
        raw = intent_path.read_bytes()
        domain = b"LXP/market-maker-ramp/append-intent/v1\0"
        header = len(domain) + 12
        require(raw.startswith(domain) and len(raw) > header + 32, "durable exact append intent absent")
        offset = int.from_bytes(raw[len(domain):len(domain) + 8], "big")
        length = int.from_bytes(raw[len(domain) + 8:header], "big")
        require(len(raw) == header + length + 32 and hashlib.sha256(raw[:-32]).digest() == raw[-32:],
                "durable pending record framing or digest invalid")
        pending = raw[header:-32]
        require(offset == len(original) and pending.startswith(damaged[len(original):])
                and pending.endswith(b"\n"), "torn bytes do not match exact retained pending record")
        require(state["journal"]["halted"] and state["journal"]["recovery_required"],
                "interrupted append lost its halt")
        for _ in range(3):
            code, health = self.request("/internal/v1/journal")
            require(code == 200 and health["halted"] and not health["ready"], "status read cleared halt")
        code, _ = self.request("/v1/orders", interrupted, "customer")
        require(code == 503, "halted mutation was admitted")
        code, _ = self.request("/internal/v1/journal/recover", {})
        require(code == 503 and self.journal.read_bytes() == damaged and intent_path.read_bytes() == raw,
                "recovery under the same OS write refusal changed retained history or intent")
        self.stop()
        original_config_path = self.config_path
        mismatch_journal = self.work / "mismatched-tail.jsonl"
        mismatch_bytes = bytearray(damaged)
        mismatch_bytes[len(original)] ^= 1
        mismatch_journal.write_bytes(mismatch_bytes)
        mismatch_journal.chmod(0o600)
        mismatch_intent = Path(str(mismatch_journal) + ".append-intent")
        mismatch_intent.write_bytes(raw)
        mismatch_intent.chmod(0o600)
        self.config_path = self.work / "mismatched-tail-config.json"
        self.save(self.config_path, dict(self.config, journal_path=str(mismatch_journal)))
        self.start()
        self.ready(503)
        code, _ = self.request("/internal/v1/journal/recover", {})
        require(code == 503 and mismatch_journal.read_bytes() == mismatch_bytes
                and mismatch_intent.read_bytes() == raw, "unexplained tail was rewritten or admitted")
        self.passed("unexplained_tail", ["nonmatching retained prefix refuses recovery=503",
                    "journal and pending intent unchanged"])
        self.stop()
        self.config_path = original_config_path
        self.start()
        self.ready(200)
        recovered = original + pending
        require(self.journal.read_bytes() == recovered and recovered.startswith(damaged)
                and not intent_path.exists(), "restart did not complete exactly the retained record")
        code, _ = self.request("/v1/orders", interrupted, "customer")
        require(code == 201 and self.journal.read_bytes() == recovered,
                "recovered retry duplicated the order or its durable event")
        self.passed("interrupted_append", ["real RLIMIT_FSIZE interruption", "halted readyz=503",
                    "status reads retain halt", "mutation=503", "bounded recovery=503 preserves bytes",
                    "restart completes exact retained record without truncation", "retry adds no event"])
        self.stop()
        invalid = self.work / "invalid.jsonl"
        invalid.write_bytes(original.replace(b'"record_hash":', b'"invalid_hash":', 1))
        invalid.chmod(0o600)
        invalid_config = dict(self.config, journal_path=str(invalid))
        invalid_path = self.work / "invalid-config.json"
        self.save(invalid_path, invalid_config)
        refusal = subprocess.run([str(self.binary), str(invalid_path)], stdin=subprocess.DEVNULL,
                    stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=90)
        require(refusal.returncode != 0, "invalid journal process did not refuse")
        self.passed("invalid_journal", ["corrupt record process exits nonzero", "no history repair"])
        self.provider_callback_recovery()
        source = Path(self.inputs["provider_journal"])
        import stat
        info = source.lstat()
        require(stat.S_ISREG(info.st_mode) and info.st_uid == os.geteuid()
                and info.st_nlink == 1 and info.st_mode & 0o077 == 0,
                "provider journal input is not private")
        provider_journal = self.work / "provider.jsonl"
        provider_journal.write_bytes(source.read_bytes())
        provider_journal.chmod(0o600)
        self.config["journal_path"] = str(provider_journal)
        self.config_path = self.work / "provider-config.json"
        self.save(self.config_path, self.config)
        self.start()
        self.ready(200)
        required = {"pending", "reversal", "chargeback"}
        require(set(self.inputs["provider_cases"]) == required, "required real provider cases missing")
        for name, expected in self.inputs["provider_cases"].items():
            code, order = self.request("/v1/orders/" + expected["order_digest"], actor="customer")
            require(code == 200 and order["stage"] == expected["stage"], "provider typed stage changed")
            presentation = order["presentation"]
            require(presentation["external_custody_label"] and presentation["status"] == expected["status"],
                    "provider custody or aggregate state lost")
            require(expected["status"] == ("pending" if name == "pending" else "reversed"),
                    "provider expected state weakens required case")
            if name == "chargeback":
                require(presentation["refusal_code"] == "chargeback", "chargeback reason lost")
            self.passed(name, ["real provider recovery succeeded", "declared typed status retained"])
        self.stop()
        self.start()
        self.ready(200)
        environment = dict(os.environ)
        environment.update(self.private(self.inputs["journey_environment"]))
        verifier = self.work / "receipt-verifier.json"
        self.save(verifier, {"client_tls": self.config["client_tls"], "layerx": self.config["layerx"]})
        environment.update(LAYERX_RAMP_URL=self.url, LAYERX_RAMP_OPERATOR_URL=self.url,
                           LAYERX_RAMP_CUSTOMER_TOKEN=self.tokens["customer"],
                           LAYERX_RAMP_OPERATOR_TOKEN=self.tokens["operator"],
                           LAYERX_RAMP_REFERENCE_BIN=str(self.binary),
                           LAYERX_RAMP_RECEIPT_VERIFIER_CONFIG=str(verifier))
        journey = subprocess.run([str(ROOT / "services/platform/ramps/sandbox-journey.sh")], env=environment,
                    stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=600)
        require(journey.returncode == 0, "recovered real provider journey or receipt verification failed")
        self.passed("recovered_journey", ["real sandbox on/off ramp done", "independent LayerX receipt verified",
                    "external custody declared"])
        require(len(self.cases) == 11, "required ramp execution cases absent")

    def close(self):
        for process in self.processes:
            if process.poll() is None:
                process.kill()
                process.wait(timeout=10)


def ramp_journal_main(manifest):
    import http.client
    contract = None
    try:
        contract = RampJournalContract(manifest)
        contract.run()
        contract.save(contract.work / "result.json", {"status": "passed", "case": "ramp-journal-readiness",
                      "revision": contract.revision, "cases": contract.cases})
        print("RESULT passed case=ramp-journal-readiness cases=11")
        return 0
    except (Failure, OSError, ValueError, KeyError, TypeError, subprocess.TimeoutExpired) as error:
        print("FAIL ramp-journal-readiness " + type(error).__name__)
        if contract is not None:
            contract.save(contract.work / "result.json", {"status": "failed", "cases": contract.cases,
                          "failure_type": type(error).__name__})
        return 1
    finally:
        if contract is not None:
            contract.close()
            print("Evidence: " + str(contract.work))


class EthereumArchiveReader:
    """Independent finalized mirror readback using the deployed manifest/chunk ABI."""
    def __init__(self, node, config):
        self.node, self.config = node, config

    def call(self, signature, types, arguments, outputs, block):
        data = merged.evm.encode_call(signature.split('(')[0], types, arguments)
        result = self.node.call('eth_call', [{'to': self.config['contract'], 'data': '0x' + data.hex()}, block])
        return merged.decode_abi(outputs, result)

    def retrieve(self, commitment):
        config = self.config
        require(self.node.call('eth_chainId', []) == hex(config['chain_id']), 'archive chain identity')
        genesis = self.node.call('eth_getBlockByNumber', ['0x0', False])
        require(genesis['hash'] == config['genesis_hash'], 'archive genesis identity')
        head = self.node.call('eth_getBlockByNumber', ['finalized', False])
        block = head['number']
        require(int(block, 16) > 0 and int(head['hash'], 16) != 0, 'archive finalized coordinate')
        code = bytes.fromhex(self.node.call('eth_getCode', [config['contract'], block])[2:])
        require(code and merged.evm.keccak256(code).hex() == config['code_hash'], 'archive contract identity')
        publisher = self.call('publisher()', [], [], ['address'], block)[0]
        require(publisher.hex() == config['publisher'].removeprefix('0x'), 'archive publisher identity')
        length, count, digest, finalized = self.call('manifest(bytes32)', ['bytes32'], [bytes.fromhex(commitment)],
                                                   ['uint64', 'uint32', 'bytes32', 'bool'], block)
        require(finalized and 0 < length <= 67_108_864 and 0 < count <= 65_536, 'archive manifest bounds/finality')
        data = bytearray()
        for index in range(count):
            chunk = self.call('chunk(bytes32,uint32)', ['bytes32', 'uint32'], [bytes.fromhex(commitment), index], ['bytes'], block)[0]
            require(0 < len(chunk) <= 24_576 and len(data) + len(chunk) <= length, 'archive chunk bound')
            data.extend(chunk)
        data = bytes(data)
        require(len(data) == length and hashlib.sha256(data).digest() == digest
                and archive_commitment(data) == commitment, 'archive corruption or commitment mismatch')
        require(self.node.call('eth_getBlockByNumber', [block, False])['hash'] == head['hash'], 'archive publication reorged')
        return data


def archive_commitment(data):
    require(data.startswith(b'LXMIRROR\0\4'), 'committed archive v4 required')
    return hashlib.sha256(b'LXP/mirror/archive/v4\0' + data).hexdigest()


class CommittedArchiveContract:
    """Authenticate complete native block history before accepting any mirror bytes."""
    def __init__(self, nodes, genesis_hash, directory):
        require(len(nodes) >= 2 and len({node.url for node in nodes}) == len(nodes), 'independent native acquisition required')
        self.nodes, self.genesis_hash, self.directory = nodes, genesis_hash, Path(directory)

    def acquire(self, height):
        require(type(height) is int and 0 < height < 2**64, 'committed archive height')
        observations = []
        for node in self.nodes:
            require(node.call('eth_chainId', []) == '0x7d', 'archive requires merged chain 125')
            require(node.call('eth_getBlockByNumber', ['0x0', False])['hash'] == self.genesis_hash, 'archive native genesis mismatch')
            head = node.call('eth_getBlockByNumber', ['finalized', False])
            require(int(head['number'], 16) >= height, 'archive head behind requested block')
            block = node.call('eth_getBlockByNumber', [hex(height), False])
            require(block['number'] == hex(height) and int(block['hash'], 16) != 0, 'archive block identity')
            transactions = block['transactions']
            require(isinstance(transactions, list) and len(transactions) <= 1_048_576
                    and len(set(transactions)) == len(transactions), 'complete distinct block transactions')
            receipts = []
            for reference in transactions:
                committed = node.committed(reference)
                require(committed['block_hash'] == block['hash'] and committed['block_number'] == block['number'],
                        'archive receipt belongs to another committed block')
                receipts.append(bytes.fromhex(committed['canonical_bytes']))
            data = bytearray(b'LXMIRROR\0\4' + (125).to_bytes(4, 'big') + height.to_bytes(8, 'big'))
            for name in ('hash', 'parentHash', 'stateRoot'):
                value = bytes.fromhex(block[name].removeprefix('0x'))
                require(len(value) == 32, 'archive header hash width')
                data.extend(value)
            timestamp = int(block['timestamp'], 16)
            require(0 <= timestamp < 2**64, 'archive timestamp')
            data.extend(timestamp.to_bytes(8, 'big') + len(receipts).to_bytes(4, 'big'))
            for receipt in receipts:
                require(0 < len(receipt) <= 33_554_432, 'archive receipt bound')
                data.extend(len(receipt).to_bytes(4, 'big') + receipt)
            require(len(data) <= 67_108_864, 'archive byte bound')
            observations.append(bytes(data))
        require(all(value == observations[0] for value in observations), 'independent native archive divergence')
        data = observations[0]
        commitment = archive_commitment(data)
        evidence.retain(self.directory / (commitment + '.archive'), data)
        return commitment, data

    def readback(self, height, readers):
        require(readers and len({reader.node.url for reader in readers}) == len(readers), 'independent archive consumers required')
        commitment, expected = self.acquire(height)
        for reader in readers:
            require(reader.retrieve(commitment) == expected, 'mirror differs from committed native block history')
        evidence.retain(self.directory / (str(height) + '.json'), evidence.canonical({'height': height, 'commitment': commitment}))
        return commitment, expected


class MirrorCheckpointContract:
    """Historical gate name, current paxd/v4 publisher and consumer contract."""
    def __init__(self, manifest):
        from candidate import catalogue, load_private, validate
        candidate = load_private(manifest)
        validate(candidate, catalogue(ROOT / SERVICE_CATALOGUE))
        self.revision = candidate['source']['revision']
        require(self.revision == revision(), 'mirror source revision differs from checkout')
        self.inputs = load_private(os.environ.get('PAXEER_X_MIRROR_CHECKPOINT_INPUTS', ''))
        require(self.inputs['schema'] == 'paxeer-x.committed-archive-inputs.v2', 'merged archive runtime inputs required')
        require(self.inputs['source_revision'] == self.revision, 'archive source revision')
        merged.source_artifacts(self.inputs, self.revision)
        self.work = Path(tempfile.mkdtemp(prefix='committed-archive-', dir=self.inputs['evidence_directory']))
        self.work.chmod(0o700)
        self.children, self.logs, self.cases = [], [], []
        self.binaries = self.inputs['artifacts']
        self.config = load_private(self.inputs['publisher_config'])
        mirror = next(service for service in candidate['services'] if service['id'] == 'mirror')
        require(mirror['bindings']['config_digest'] == 'sha256:' + hashlib.sha256(Path(self.inputs['publisher_config']).read_bytes()).hexdigest(),
                'publisher candidate config binding')
        require(self.config['paxeer']['chain_id'] == candidate['foundation']['chain_id'] == 125,
                'native committed archive chain pin')
        self.fixture = merged.OWNED.PaxdFixture(self.inputs['native'], self.binaries['paxd']['path'], self.work)
        self.config['state_directory'] = str(self.work / 'publisher-state')

    @staticmethod
    def save(path, value):
        evidence.retain(path, evidence.canonical(value))

    def launch_publisher(self):
        log = (self.work / ('publisher-' + str(len(self.children)) + '.log')).open('xb')
        process = subprocess.Popen([self.binaries['layerx-mirror-publisher']['path'], str(self.work / 'publisher.json')],
                                   stdin=subprocess.DEVNULL, stdout=log, stderr=log)
        self.children.append(process)
        self.logs.append(log)
        return process

    @staticmethod
    def stop(process):
        if process is not None and process.poll() is None:
            process.terminate()
            process.wait(timeout=10)

    def run(self):
        self.fixture.start()
        self.save(self.work / 'publisher.json', self.config)
        publisher = self.launch_publisher()
        nodes = [self.fixture.node] + [merged.CHAIN.Node(row['endpoint'], self.fixture.node.receipt_keys)
                                      for row in self.inputs['independent_nodes']]
        contract = CommittedArchiveContract(nodes, '0x' + self.config['paxeer']['genesis_hash_hex'].removeprefix('0x'), self.work)
        readers = [EthereumArchiveReader(merged.CHAIN.Node(row['endpoint'], self.fixture.node.receipt_keys), row)
                   for row in self.inputs['readback_sources']]
        deadline = time.monotonic() + 150
        while True:
            require(publisher.poll() is None, 'current archive publisher exited')
            try:
                baseline = contract.readback(self.config['first_height'], readers)
                break
            except (Failure, ValueError, OSError):
                require(time.monotonic() < deadline, 'committed publication/readback deadline')
                time.sleep(0.2)
        self.cases.append('independent-committed-v4-acquisition-and-readback')
        self.stop(publisher)
        retained = {str(path.relative_to(self.work / 'publisher-state')): path.read_bytes()
                    for path in (self.work / 'publisher-state').rglob('*') if path.is_file()}
        require(retained, 'durable archive publication records absent')
        self.fixture.restart()
        self.launch_publisher()
        require(contract.readback(self.config['first_height'], readers) == baseline, 'restart changed committed archive')
        for name, data in retained.items():
            require((self.work / 'publisher-state' / name).read_bytes().startswith(data), 'restart replaced durable publication history')
        self.cases.append('paxd-publisher-and-consumer-restart')
        self.fixture.stop()
        verifier = self.inputs['verifier_config']
        verifier_config = json.loads(evidence.protected(verifier))
        require([row['kind'] for row in verifier_config['sources']] == ['ethereum', 'solana'],
                'both independent Ethereum and Solana archive consumers required')
        original = bytes.fromhex(self.inputs['receipt_canonical'])
        require(original and original in baseline[1], 'offline receipt must be in the acquired native archive')
        corrupt = original[:-1] + bytes([original[-1] ^ 1])
        for source in range(len(verifier_config['sources'])):
            for canonical, expected in ((original.hex(), True), (corrupt.hex(), False)):
                request = {'batch_number': str(self.config['first_height']), 'evidence': {'kind': 'receipt', 'canonical_hex': canonical},
                           'policy': {'kind': 'exact', 'candidate': {'source': source, 'commitment_hex': baseline[0]}}}
                result = subprocess.run([self.binaries['layerx-mirror-verify']['path'], verifier], input=json.dumps(request),
                                        text=True, capture_output=True, timeout=90)
                require(result.returncode == 0 and json.loads(result.stdout)['ok'] is expected, 'offline archive evidence verdict')
        self.cases.append('offline-current-verifier-valid-and-corrupted-receipt')

    def close(self):
        for process in reversed(self.children):
            self.stop(process)
        self.fixture.stop()
        for log in self.logs:
            log.close()


def mirror_checkpoint_main(manifest):
    contract = None
    try:
        contract = MirrorCheckpointContract(manifest)
        contract.run()
        contract.save(contract.work / 'result.json', {'status': 'passed', 'revision': contract.revision, 'cases': contract.cases})
        print('RESULT passed case=mirror-checkpoint-acquisition')
        return 0
    except (Failure, OSError, ValueError, KeyError, TypeError, subprocess.TimeoutExpired) as error:
        print('FAIL mirror-checkpoint-acquisition ' + str(error))
        return 1
    finally:
        if contract is not None:
            contract.close()
            print('Evidence: ' + str(contract.work))


class ReadinessInterposer:
    def __init__(self, contract):
        self.contract = contract
        self.mode = "pass"
        self.seen = {}
        outer = self

        class Handler(http.server.BaseHTTPRequestHandler):
            def log_message(self, format, *args):
                del format, args

            def do_POST(self):
                self.close_connection = True

            def do_GET(self):
                if outer.mode == "offline":
                    outer.seen[outer.mode] = outer.seen.get(outer.mode, 0) + 1
                    self.close_connection = True
                    return
                try:
                    headers = {"Authorization": self.headers.get("Authorization", "")}
                    code, body = outer.contract.http(outer.contract.inputs["gateway"]["url"],
                                                    self.path, headers=headers)
                    if self.path == "/readyz/core":
                        require(body.get("service") == "layerx-gateway", "actual gateway readiness absent")
                        outer.seen[outer.mode] = outer.seen.get(outer.mode, 0) + 1
                        if outer.mode != "pass":
                            require(code == 200 and body["status"] == "ready",
                                    "hostile readiness requires an actual ready upstream")
                            code = 200
                            outer.mutate(body)
                    raw = json.dumps(body, separators=(",", ":")).encode()
                    self.send_response(code)
                    self.send_header("Content-Type", "application/json")
                    self.send_header("Content-Length", str(len(raw)))
                    self.send_header("Connection", "close")
                    self.end_headers()
                    self.wfile.write(raw)
                except (OSError, Failure, ValueError, KeyError, TypeError):
                    self.close_connection = True

        from urllib.parse import urlsplit
        endpoint = urlsplit(contract.inputs["proxy_tls"]["url"])
        require(endpoint.scheme == "https" and endpoint.hostname in ("127.0.0.1", "localhost")
                and endpoint.port is not None and endpoint.port > 1024 and not endpoint.path,
                "owned proxy must be an explicit loopback TLS endpoint")
        self.server = http.server.ThreadingHTTPServer((endpoint.hostname, endpoint.port), Handler)
        tls = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        tls.minimum_version = ssl.TLSVersion.TLSv1_2
        tls.load_cert_chain(contract.inputs["proxy_tls"]["certificate"],
                            contract.inputs["proxy_tls"]["key"])
        tls.load_verify_locations(cafile=contract.inputs["proxy_tls"]["client_ca"])
        tls.verify_mode = ssl.CERT_REQUIRED
        self.server.socket = tls.wrap_socket(self.server.socket, server_side=True)
        self.server.daemon_threads = True
        threading.Thread(target=self.server.serve_forever, daemon=True).start()

    def mutate(self, body):
        mode = self.mode
        if mode == "degraded":
            body["status"] = "degraded"
        elif mode == "absent":
            del body["backends"]["core_agent_boundary"]
        elif mode == "unconfigured":
            body["backends"]["core_agent_boundary"] = {"state": "unavailable", "reason": "not_configured"}
        elif mode == "unknown-field":
            body["unknown_required_contract"] = True
        elif mode == "unknown-component":
            body["components"]["unknown_required_contract"] = "ready"
        elif mode == "malformed":
            body["protocol_network_id"] = str(body["protocol_network_id"])
        elif mode == "network":
            body["network_id"] += "-wrong"
        elif mode == "protocol-network":
            body["protocol_network_id"] += 1
        elif mode == "wire":
            body["lxp_wire_version"] += "0"
        elif mode == "chain":
            body["capabilities"]["chain_serving"]["chain_id"] = 126
        elif mode == "activity-capability":
            body["capabilities"]["layerx_activity"]["state"] = "unavailable"
        elif mode == "unknown-capability":
            body["capabilities"]["unknown"] = {"state": "ready", "reason": "ready"}
        elif mode == "scope":
            body["scope"] = "product"
        elif mode == "expiry":
            body["observed_at_ms"] = now_ms() - 30_001
            body["valid_until_ms"] = now_ms() - 1
        elif mode == "future":
            body["observed_at_ms"] = now_ms() + 60_000
            body["valid_until_ms"] = body["observed_at_ms"] + 30_000
        elif mode == "version":
            body["readiness_version"] += 1
        else:
            raise Failure("unknown hostile readiness mode")

    def close(self):
        self.server.shutdown()
        self.server.server_close()


class InteropReadinessContract:
    ADAPTERS = {"x402", "ap2", "ucp", "visa-tap", "fiat"}
    TRANSPORTS = {"http", "mcp", "a2a"}

    def __init__(self, manifest):
        import stat
        from urllib.parse import urlsplit
        sys.path.insert(0, str(ROOT / "tools/paxeer-x"))
        from candidate import catalogue, load_private, validate
        self.private = load_private
        candidate = load_private(manifest)
        validate(candidate, catalogue(ROOT / SERVICE_CATALOGUE))
        self.revision = candidate["source"]["revision"]
        require(self.revision == revision(), "interop candidate revision differs from checkout")
        services = {item["id"]: item for item in candidate["services"]}
        path = os.environ.get("PAXEER_X_INTEROP_READINESS_INPUTS", "")
        require(path and services["interop"]["bindings"]["providers_ref"] == "private:" + str(Path(path).resolve()),
                "candidate lacks the exact protected interop qualification input reference")
        self.inputs = load_private(path)
        self.fields(self.inputs, {"schema", "source_revision", "pins", "evidence_directory", "interop",
                    "gateway", "dependencies", "proxy_tls", "http_tls", "credential_file", "exchanges",
                    "refusals", "states", "configuration_refusals"}, "qualification inputs")
        require(self.inputs["schema"] == "paxeer-x.interop-readiness-inputs.v1"
                and self.inputs["source_revision"] == self.revision, "input schema or source mismatch")
        self.fields(self.inputs["pins"], {"chain_id", "network_id", "protocol_network_id", "wire_version"}, "pins")
        require(self.inputs["pins"]["chain_id"] == candidate["foundation"]["chain_id"] == 125,
                "chain pin differs from the selected candidate")
        evidence = Path(self.inputs["evidence_directory"])
        info = evidence.lstat()
        require(stat.S_ISDIR(info.st_mode) and info.st_uid == os.geteuid() and info.st_mode & 0o077 == 0,
                "evidence parent is not a private owned directory")
        self.work = Path(tempfile.mkdtemp(prefix="interop-readiness-", dir=evidence))
        self.work.chmod(0o700)
        self.children = {}
        self.logs = []
        self.cases = []
        self.proxy = None
        self.environment = {}
        self.specs = {}
        for name in ("interop", "gateway"):
            spec = self.inputs[name]
            self.fields(spec, {"binary", "binary_sha256", "environment", "url", "state_paths"}, name)
            self.binary(spec)
            env = load_private(spec["environment"])
            require(all(isinstance(key, str) and isinstance(value, str) for key, value in env.items()),
                    "environment must contain string values")
            prefix = "LAYERX_INTEROP_" if name == "interop" else "LAYERX_GATEWAY_"
            require(all(key.startswith(prefix) for key in env), "unexpected production environment variable")
            binding = services[name]["bindings"]
            require(binding["source_revision"] == self.revision
                    and binding["config_ref"] == "private:" + str(Path(spec["environment"]).resolve())
                    and binding["config_digest"] == "sha256:" + self.digest(Path(spec["environment"])),
                    "candidate environment/source binding mismatch")
            endpoint = urlsplit(spec["url"])
            require(endpoint.scheme in ("http", "https") and endpoint.hostname in ("127.0.0.1", "localhost")
                    and endpoint.port is not None and endpoint.port > 1024 and not endpoint.path,
                    "served process must use an owned loopback endpoint")
            require(env[prefix + "LISTEN"] == endpoint.hostname + ":" + str(endpoint.port),
                    "process listen endpoint differs from qualification endpoint")
            self.environment[name] = env
            self.specs[name] = dict(spec, argv=[])
        pins = self.inputs["pins"]
        for name, prefix, wire in (("interop", "LAYERX_INTEROP_", "WIRE_VERSION"),
                                    ("gateway", "LAYERX_GATEWAY_", "LXP_WIRE_VERSION")):
            env = self.environment[name]
            require(env[prefix + "NETWORK_ID"] == pins["network_id"]
                    and env[prefix + wire] == pins["wire_version"]
                    and env[prefix + "PROTOCOL_NETWORK_ID"] == str(pins["protocol_network_id"]),
                    "production pins differ from qualification pins")
        self.fields(self.inputs["proxy_tls"], {"url", "certificate", "key", "client_ca"}, "proxy TLS")
        self.fields(self.inputs["http_tls"], {"ca", "certificate", "key"}, "HTTP TLS")
        require(self.environment["interop"]["LAYERX_INTEROP_HOSTED_GATEWAY_URL"] == self.inputs["proxy_tls"]["url"],
                "actual interop consumer is not wired to the hostile forwarding boundary")
        for key in ("key",):
            self.private_file(self.inputs["proxy_tls"][key])
            self.private_file(self.inputs["http_tls"][key])
        self.context = ssl.create_default_context(cafile=self.inputs["http_tls"]["ca"])
        self.context.load_cert_chain(self.inputs["http_tls"]["certificate"], self.inputs["http_tls"]["key"])
        self.private_file(self.inputs["credential_file"])
        self.credential = Path(self.inputs["credential_file"]).read_text().strip()
        require(self.credential and "\n" not in self.credential, "authentication input absent")
        dependencies = self.inputs["dependencies"]
        require(isinstance(dependencies, list) and {item["id"] for item in dependencies}
                == {"core_agent_boundary", "redis"} and len(dependencies) == 2,
                "owned real core and Redis processes are required")
        for spec in dependencies:
            self.fields(spec, {"id", "binary", "binary_sha256", "source_revision", "argv", "environment",
                              "state_paths"}, "dependency")
            require(spec["source_revision"] == self.revision and isinstance(spec["argv"], list)
                    and all(isinstance(arg, str) for arg in spec["argv"]), "dependency source/argv mismatch")
            self.binary(spec)
            self.specs[spec["id"]] = spec
            self.environment[spec["id"]] = load_private(spec["environment"])
        for spec in self.specs.values():
            require(isinstance(spec["state_paths"], list) and spec["state_paths"], "real persistence paths absent")
            for path in spec["state_paths"]:
                directory = Path(path)
                info = directory.lstat()
                require(directory.resolve().is_relative_to(evidence.resolve())
                        and stat.S_ISDIR(info.st_mode) and info.st_uid == os.geteuid()
                        and info.st_mode & 0o077 == 0, "process persistence is not an owned private fixture")
        runtime = load_private(self.environment["interop"]["LAYERX_INTEROP_CONFIG"])
        require({item["id"] for item in runtime["adapters"]} == self.ADAPTERS
                and len(runtime["adapters"]) == 5 and {item["id"] for item in runtime["transports"]} == self.TRANSPORTS
                and len(runtime["transports"]) == 3, "exact configured adapter/transport membership absent")
        self.runtime = runtime

    @staticmethod
    def fields(value, expected, label):
        require(isinstance(value, dict) and set(value) == expected, label + " has missing or unknown fields")

    @staticmethod
    def digest(path):
        digest = hashlib.sha256()
        with path.open("rb") as stream:
            for block in iter(lambda: stream.read(1024 * 1024), b""):
                digest.update(block)
        return digest.hexdigest()

    @staticmethod
    def private_file(path):
        import stat
        require(not any(part == ".env" or part.startswith(".env.") for part in Path(path).parts),
                "environment credential files are forbidden")
        info = Path(path).lstat()
        require(stat.S_ISREG(info.st_mode) and info.st_uid == os.geteuid()
                and info.st_nlink == 1 and info.st_mode & 0o077 == 0, "input must be a private owned regular file")

    def binary(self, spec):
        path = Path(spec["binary"])
        require(path.is_file() and os.access(path, os.X_OK) and self.digest(path) == spec["binary_sha256"],
                "exact production executable absent or mismatched")
        with path.open("rb") as stream:
            require(stream.read(4) == b"\x7fELF", "qualification requires an actual built production executable")

    def save(self, path, value):
        fd = os.open(path, os.O_CREAT | os.O_EXCL | os.O_WRONLY | os.O_NOFOLLOW, 0o600)
        with os.fdopen(fd, "w") as stream:
            json.dump(value, stream, sort_keys=True)
            stream.write("\n")

    def start(self, name, environment=None):
        spec = self.specs[name]
        log = (self.work / (name + ".log")).open("ab")
        os.chmod(log.name, 0o600)
        self.logs.append(log)
        env = dict(PATH=os.environ.get("PATH", "/usr/bin:/bin"),
                   **(self.environment[name] if environment is None else environment))
        process = subprocess.Popen([spec["binary"], *spec["argv"]], cwd=ROOT, env=env,
                                   stdin=subprocess.DEVNULL, stdout=log, stderr=subprocess.STDOUT)
        self.children[name] = process
        return process

    def stop(self, name):
        process = self.children.get(name)
        if process is not None and process.poll() is None:
            process.terminate()
            try:
                process.wait(timeout=10)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait(timeout=10)

    def http(self, url, path, body=None, headers=None):
        request = urllib.request.Request(url + path, data=None if body is None else json.dumps(body).encode(),
                    headers=dict({"Content-Type": "application/json"}, **(headers or {})))
        try:
            response = urllib.request.urlopen(request, context=self.context, timeout=8)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            require(response.headers.get_content_type() == "application/json", "served response is not JSON")
            raw = response.read(512 * 1024 + 1)
            require(len(raw) <= 512 * 1024, "served response exceeds bound")
            from candidate import duplicate_free
            return response.status, json.loads(raw, object_pairs_hook=duplicate_free)

    def request(self, path, body=None, identity=None):
        headers = {"Authorization": "Bearer " + self.credential}
        if identity is not None:
            headers["Idempotency-Key"] = identity
        return self.http(self.inputs["interop"]["url"], path, body, headers)

    def until(self, predicate, label, seconds=60):
        deadline = time.monotonic() + seconds
        while time.monotonic() < deadline:
            require(self.children["interop"].poll() is None, "served interop process exited")
            try:
                result = predicate()
                if result:
                    return result
            except (OSError, ValueError, KeyError, TypeError, Failure):
                pass
            time.sleep(0.1)
        raise Failure("readiness deadline: " + label)

    def agreement(self, expected):
        code, ready = self.request("/readyz")
        metadata_code, metadata = self.request("/v1/adapters")
        require(code == (200 if expected else 503) and metadata_code == 200,
                "readiness HTTP/metadata availability disagreement")
        require(ready["status"] == ("ready" if expected else "degraded"), "readiness status disagreement")
        require(ready["capabilities"] == metadata["capabilities"], "ready/metadata capability disagreement")
        caps = ready["capabilities"]
        require(caps["chain_serving"]["chain_id"] == 125
                and caps["layerx_settlement"]["state"] == ("ready" if expected else "unavailable"),
                "chain pin or settlement capability incorrect")
        require({item["id"] for item in metadata["adapters"]} == self.ADAPTERS
                and len(metadata["adapters"]) == 5
                and {item["id"] for item in metadata["transports"]} == self.TRANSPORTS
                and len(metadata["transports"]) == 3, "configured manifest membership changed")
        pins = self.inputs["pins"]
        require(ready["network_id"] == pins["network_id"]
                and ready["protocol_network_id"] == pins["protocol_network_id"]
                and ready["lxp_wire_version"] == pins["wire_version"], "served identity pins differ")
        for adapter in metadata["adapters"]:
            require(adapter["readiness"]["settlement"] == ("ready" if expected else "unavailable"),
                    "adapter settlement differs from shared readiness")
            configured = next(item for item in self.runtime["adapters"] if item["id"] == adapter["id"])
            for field in ("specification", "version", "specification_sha256", "conformance_suite",
                          "conformance_vectors", "conformance_sha256", "evidence_policy"):
                require(adapter[field] == configured[field], "adapter trust manifest pin changed")
        return ready

    def passed(self, name, **evidence):
        self.cases.append(dict(case=name, **evidence))
        print("PASS " + name, flush=True)

    def receipt(self, path):
        evidence = self.private(path)
        self.fields(evidence, {"activity_hex", "receipt_hex", "activity_id", "batch_id", "expected_signer"},
                    "independent receipt evidence")
        binary = self.inputs["interop"]["binary"]
        env = dict(PATH=os.environ.get("PATH", "/usr/bin:/bin"), **self.environment["interop"])
        def verify(file, valid):
            result = subprocess.run([binary, "--verify-readiness-receipt", str(file)], cwd=ROOT, env=env,
                                    stdin=subprocess.DEVNULL, capture_output=True, timeout=60)
            require((result.returncode == 0) == valid, "independent production receipt verifier result differs")
            if not valid:
                return
            value = json.loads(result.stdout)
            self.fields(value, {"verified", "activity_id", "batch_id", "receipt_sha256", "receipt_digest", "network_id",
                                "protocol_network_id", "wire_version"}, "receipt verifier output")
            require(value["verified"] is True and value["activity_id"] == evidence["activity_id"]
                    and value["batch_id"] == evidence["batch_id"]
                    and value["receipt_sha256"] == hashlib.sha256(bytes.fromhex(evidence["receipt_hex"])).hexdigest(),
                    "independent verifier does not bind exact activity, receipt and batch")
            pins = self.inputs["pins"]
            require(all(value[key] == pins[key] for key in ("network_id", "protocol_network_id", "wire_version")),
                    "independent receipt network identity differs")
            return value
        verified = verify(path, True)
        for field in ("activity_hex", "receipt_hex", "activity_id", "batch_id", "expected_signer"):
            hostile = dict(evidence)
            raw = bytearray(bytes.fromhex(hostile[field]))
            require(raw, "empty actual receipt evidence")
            raw[-1] ^= 1
            hostile[field] = raw.hex()
            file = self.work / ("receipt-refusal-" + field + "-" + str(len(self.cases)) + ".json")
            self.save(file, hostile)
            verify(file, False)
        self.passed("independent-exact-activity-receipt-batch", activity_id=evidence["activity_id"],
                    batch_id=evidence["batch_id"], refusals=5)
        return dict(evidence, receipt_digest=verified["receipt_digest"])

    def run(self):
        for name in ("redis", "core_agent_boundary", "gateway"):
            self.start(name)
        self.proxy = ReadinessInterposer(self)
        self.start("interop")
        baseline = self.until(lambda: self.agreement(True), "actual complete core readiness")
        self.passed("actual-served-ready-metadata-agreement", capabilities=baseline["capabilities"])
        for mode in ("degraded", "absent", "unconfigured", "unknown-field", "unknown-component", "malformed",
                     "network", "protocol-network", "wire", "chain", "activity-capability", "unknown-capability",
                     "scope", "expiry", "future", "version"):
            self.proxy.mode = mode
            state = self.until(lambda: self.proxy.seen.get(mode, 0) > 0 and self.agreement(False), mode)
            require(state["capabilities"]["chain_serving"] == baseline["capabilities"]["chain_serving"],
                    "LayerX boundary refusal changed independent chain capability")
            self.passed("actual-readiness-" + mode + "-refused", http_status=503,
                        real_mutated_responses=self.proxy.seen[mode])
            self.proxy.mode = "pass"
            self.until(lambda: self.agreement(True), mode + " recovery")
        self.fields(self.inputs["exchanges"], self.TRANSPORTS, "real exact exchanges")
        for transport, path in self.inputs["exchanges"].items():
            exchange = self.private(path)
            self.fields(exchange, {"payment_required", "scheme_payload", "idempotency_prefix", "completed_operation",
                                   "receipt_evidence"}, "exact exchange")
            offers = exchange["payment_required"]["accepts"]
            require(offers and all(offer["scheme"] == "exact" for offer in offers), "real exact scheme offer absent")
            prefix = "/v1/" + transport + "/x402/"
            code, offered = self.request(prefix + "seller/offer", exchange["payment_required"],
                                          exchange["idempotency_prefix"] + "-seller")
            require(code == 200 and offered["ok"] is True and offered["result"]["status"] == 402
                    and offered["result"]["payment_required"] == exchange["payment_required"],
                    "actual seller exact payment-required exchange refused")
            code, built = self.request(prefix + "buyer/build", {"payment_required": offered["result"]["payment_required"],
                                     "scheme_payload": exchange["scheme_payload"]}, exchange["idempotency_prefix"] + "-buyer")
            require(code == 200 and built["ok"] is True, "real buyer payment exchange refused")
            payment = built["result"]["payment_payload"]
            require(payment["accepted"]["scheme"] == "exact" and payment["payload"] == exchange["scheme_payload"],
                    "buyer changed exact signed payment payload")
            code, verified = self.request(prefix + "verify", {"x402Version": 2, "paymentPayload": payment,
                                          "paymentRequirements": payment["accepted"]},
                                          exchange["idempotency_prefix"] + "-verify")
            require(code == 200 and verified["ok"] is True and verified["result"]["isValid"] is True,
                    "actual exact signed scheme verification refused")
            code, settled = self.operation(exchange["completed_operation"])
            require(code == 200 and settled["ok"] is True and settled["result"]["success"] is True,
                    "actual durable completed exact exchange is absent")
            evidence = self.receipt(exchange["receipt_evidence"])
            require(payment["payload"]["layerxActivity"] == evidence["activity_hex"],
                    "exchange signed activity differs from independently verified settlement")
            self.passed("real-exact-scheme-" + transport, operation=exchange["completed_operation"])
        self.fields(self.inputs["refusals"], self.ADAPTERS, "adapter constrained refusal cases")
        allowed = {"x402": {"invalid_x402_request", "unsupported_x402_offer"},
                   "ap2": {"mandate_verification_refused", "asset_binding_ambiguous"},
                   "ucp": {"ucp_profile_refused", "ucp_capability_refused", "ucp_order_invalid"},
                   "visa-tap": {"visa_tap_refused"},
                   "fiat": {"provider_callback_refused"}}
        routes = {"x402": "/v1/http/x402/verify", "ap2": "/v1/http/ap2/mandates/verify",
                  "ucp": "/v1/http/ucp/checkouts/complete", "visa-tap": "/v1/http/visa-tap/intents/verify",
                  "fiat": "/v1/http/fiat/card/callbacks"}
        for adapter, path in self.inputs["refusals"].items():
            refusal = self.private(path)
            self.fields(refusal, {"body", "idempotency_key", "error_code"}, "constrained refusal")
            require(refusal["error_code"] in allowed[adapter], "refusal fixture weakens adapter constraint")
            code, body = self.request(routes[adapter], refusal["body"], refusal["idempotency_key"])
            require(code == 400 and body["ok"] is False and body["error"]["code"] == refusal["error_code"],
                    "real configured adapter did not refuse its trust constraint")
            self.passed("configured-" + adapter + "-constrained-refusal", http_status=code,
                        error_code=body["error"]["code"])
        self.fields(self.inputs["states"], {"pending", "reversed", "chargeback"}, "external state cases")
        retained = {}
        for name in ("reversed", "chargeback"):
            state = self.private(self.inputs["states"][name])
            self.fields(state, {"completed_operation", "receipt_evidence"}, "retained external state")
            code, body = self.operation(state["completed_operation"])
            label = "reversed" if name == "reversed" else "charged-back"
            require(code == 200 and body["ok"] is True and body["result"]["state"] == label,
                    "actual external reversal/chargeback semantics changed")
            evidence = self.receipt(state["receipt_evidence"])
            require(body["result"]["receipt_digest"] == evidence["receipt_digest"],
                    "external state lost its independently verified exact receipt")
            retained[state["completed_operation"]] = body["result"]
            self.passed("real-" + name + "-state", operation=state["completed_operation"])
        self.proxy.mode = "offline"
        self.until(lambda: self.agreement(False), "gateway loss")
        pending = self.private(self.inputs["states"]["pending"])
        self.fields(pending, {"body", "idempotency_key"}, "pending real provider input")
        require(pending["body"]["evidence"]["facts"]["class"] == "settled"
                and pending["body"]["activity"], "pending requires real signed provider and activity input")
        code, body = self.request("/v1/http/fiat/card/callbacks", pending["body"], pending["idempotency_key"])
        require(code == 503 and body["ok"] is False and body["error"]["code"] == "settlement_unavailable",
                "dependency loss silently completed or refused a pending settlement")
        require("receipt_digest" not in body and "receipt" not in body, "pending advertised receipt success")
        self.passed("real-pending-during-dependency-loss", http_status=code, funded_dispatches=0)
        self.proxy.mode = "pass"
        self.until(lambda: self.agreement(True), "actual gateway restoration")
        for name in ("gateway", "core_agent_boundary", "redis"):
            self.stop(name)
            self.until(lambda: self.agreement(False), name + " loss")
            self.stop("interop")
            self.start("interop")
            self.until(lambda: self.agreement(False), "interop restart during " + name + " loss")
            self.start(name)
            self.until(lambda: self.agreement(True), name + " restoration")
            self.passed("real-" + name + "-loss-restart-restoration", http_status=200)
        self.stop("interop")
        self.start("interop")
        self.until(lambda: self.agreement(True), "retained interop restart")
        for operation, result in retained.items():
            code, body = self.operation(operation)
            require(code == 200 and body["result"] == result, "restart lost retained exact external state")
        self.passed("real-persistence-retains-distinct-external-outcomes")
        self.configuration_refusals()
        require(len(self.cases) == 42, "required interop execution cases absent")

    def operation(self, operation):
        require(isinstance(operation, str) and len(operation) == 64
                and bytes.fromhex(operation).hex() == operation, "completed operation identity invalid")
        return self.request("/v1/operations/" + operation)

    def configuration_refusals(self):
        required = {"ap2-assets", "ap2-roots", "visa-identities", "fiat-roots", "redis"}
        self.fields(self.inputs["configuration_refusals"], required, "protected configuration refusals")
        self.stop("interop")
        for name, path in self.inputs["configuration_refusals"].items():
            environment = self.private(path)
            base = self.environment["interop"]
            changed = {key for key in set(environment) | set(base) if environment.get(key) != base.get(key)}
            expected = {"LAYERX_INTEROP_REDIS_PASSWORD_FILE"} if name == "redis" else {"LAYERX_INTEROP_CONFIG"}
            require(changed == expected, "refusal environment changes unrelated authority")
            if expected == {"LAYERX_INTEROP_CONFIG"}:
                document = self.private(environment["LAYERX_INTEROP_CONFIG"])
                field = {"ap2-assets": "ap2_assets", "ap2-roots": "ap2_keys", "visa-identities": "visa_agents",
                         "fiat-roots": "fiat_providers"}[name]
                require(document[field] == [] and {key for key in set(document) | set(self.runtime)
                        if document.get(key) != self.runtime.get(key)} == {field}, "trust refusal removed extra constraints")
            else:
                require(not Path(environment[next(iter(expected))]).exists(), "missing protected authority fixture is present")
            result = subprocess.run([self.inputs["interop"]["binary"]],
                                    env=dict(PATH=os.environ.get("PATH", "/usr/bin:/bin"), **environment),
                                    cwd=ROOT, stdin=subprocess.DEVNULL, capture_output=True, timeout=30)
            require(result.returncode != 0, "missing required adapter/authority material silently disabled a service")
            self.passed("missing-" + name + "-startup-refused", exit_code=result.returncode)

    def close(self):
        if self.proxy is not None:
            self.proxy.close()
        for name in reversed(list(self.children)):
            self.stop(name)
        for log in self.logs:
            log.close()


def interop_readiness_main(manifest):
    contract = None
    try:
        contract = InteropReadinessContract(manifest)
        contract.run()
        contract.save(contract.work / "result.json", {"status": "passed", "revision": contract.revision,
                      "case": "interop-settlement-readiness", "cases": contract.cases})
        print("RESULT passed case=interop-settlement-readiness")
        return 0
    except (Failure, OSError, ValueError, KeyError, TypeError, subprocess.TimeoutExpired) as error:
        print("FAIL interop-settlement-readiness " + type(error).__name__)
        if contract is not None:
            contract.save(contract.work / "result.json", {"status": "failed", "revision": contract.revision,
                          "cases": contract.cases, "failure_type": type(error).__name__})
        return 1
    finally:
        if contract is not None:
            contract.close()
            print("Evidence: " + str(contract.work))


def main():
    parser = argparse.ArgumentParser(prog="interop-archive-contract")
    parser.add_argument("--case", required=True, choices=CASES)
    parser.add_argument("--candidate-manifest", required=True)
    arguments = parser.parse_args()
    if arguments.case == "interop-settlement-readiness":
        return interop_readiness_main(arguments.candidate_manifest)
    if arguments.case == "mirror-checkpoint-acquisition":
        return mirror_checkpoint_main(arguments.candidate_manifest)
    if arguments.case == "ramp-journal-readiness":
        return ramp_journal_main(arguments.candidate_manifest)


if __name__ == "__main__":
    raise SystemExit(main())

import os
from pathlib import Path
import re
import shutil
import subprocess
import sys


ROOT = Path(__file__).resolve().parents[2]
PREPARE_TARGET = "agent-prepare-codec-corpus"


def test_retained_eip1559_producer_consumer_conformance():
    import json
    from http.server import BaseHTTPRequestHandler, HTTPServer
    from threading import Thread

    from cryptography.hazmat.primitives import hashes
    from cryptography.hazmat.primitives.asymmetric import ec, utils

    sys.path.insert(0, str(ROOT / "services/agent/sdk/python"))
    from layerx_sdk.account_derivation import _N, keccak256
    from layerx_sdk.evm_transaction import (
        Type2Transaction,
        activity_id,
        decode_signed,
        encode_call,
    )
    from layerx_sdk.x402_rpc import PaymentRpc

    sender = "0x7E5F4552091A69125d5DfCb7b8C2659029395Bdf"
    recipient = bytes.fromhex("11" * 20)
    calldata = encode_call("transfer", ["address", "uint256"], [recipient, 17])
    assert calldata == (
        bytes.fromhex("a9059cbb") + recipient.rjust(32, b"\0")
        + (17).to_bytes(32, "big")
    )
    transaction = Type2Transaction(
        chain_id=125,
        nonce=9,
        max_priority_fee_per_gas=1_000_000_000,
        max_fee_per_gas=3_000_000_000,
        gas_limit=100_000,
        to=bytes.fromhex("22" * 20),
        value=0,
        data=calldata,
    )
    unsigned = transaction.encode()
    assert unsigned[0] == 2
    assert Type2Transaction.decode(unsigned) == transaction
    digest = transaction.signing_digest()
    assert digest == keccak256(unsigned)
    key = ec.derive_private_key(1, ec.SECP256K1())
    r, s = utils.decode_dss_signature(
        key.sign(digest, ec.ECDSA(utils.Prehashed(hashes.SHA256())))
    )
    s = min(s, _N - s)
    signatures = [
        r.to_bytes(32, "big") + s.to_bytes(32, "big") + bytes([parity])
        for parity in (27, 28)
    ]
    signature, = [
        value for value in signatures if transaction.recover_signer(value) == sender
    ]
    raw = transaction.attach_signature(signature, sender)
    received = []
    failures = []

    class TransactionServer(BaseHTTPRequestHandler):
        def log_message(self, *args):
            pass

        def do_POST(self):
            try:
                assert self.path == "/"
                assert self.headers["Content-Type"] == "application/json"
                request = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
                assert set(request) == {"jsonrpc", "id", "method", "params"}
                assert request["jsonrpc"] == "2.0"
                assert request["method"] == "eth_sendRawTransaction"
                assert request["params"] == ["0x" + raw.hex()]
                transported = bytes.fromhex(request["params"][0][2:])
                decoded, recovered_signature = decode_signed(transported)
                assert decoded == transaction
                assert decoded.data == calldata
                assert decoded.recover_signer(recovered_signature) == sender
                assert decoded.attach_signature(recovered_signature, sender) == raw
                received.append((decoded, recovered_signature, transported))
                response = {
                    "jsonrpc": "2.0", "id": request["id"],
                    "result": "0x" + activity_id(transported).hex(),
                }
                body = json.dumps(response).encode()
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)
            except Exception as error:
                failures.append(error)
                self.send_error(500)

    with HTTPServer(("127.0.0.1", 0), TransactionServer) as server:
        worker = Thread(target=server.serve_forever, daemon=True)
        worker.start()
        try:
            client = PaymentRpc(f"http://127.0.0.1:{server.server_port}/")
            result = client.send("0x" + raw.hex())
            assert result == "0x" + keccak256(raw).hex()
            assert not failures, failures
            assert received == [(transaction, signature, raw)]
        finally:
            server.shutdown()
            worker.join(timeout=5)
            assert not worker.is_alive()


def test_agent_vectors_prepare_codec_corpus_from_clean_checkout(tmp_path):
    repository = tmp_path / "repository"
    repository.mkdir()
    with subprocess.Popen(
        ["git", "archive", "--format=tar", "HEAD"], cwd=ROOT, stdout=subprocess.PIPE
    ) as archive:
        extracted = subprocess.run(
            ["tar", "-xf", "-", "-C", str(repository)],
            stdin=archive.stdout,
            capture_output=True,
            text=True,
            check=False,
        )
        archive.stdout.close()
        assert archive.wait() == 0
        assert extracted.returncode == 0, extracted.stdout + extracted.stderr
    shutil.copy2(ROOT / "Makefile", repository / "Makefile")
    corpus = repository / "tests/vectors/codec/replay_corpus.lxb"
    assert not corpus.exists()

    environment = os.environ.copy()
    for name in ("MAKEFLAGS", "MFLAGS", "MAKELEVEL", "LAYERX_RUNTIME_CLOCK_SOCKET"):
        environment.pop(name, None)
    environment["CARGO_TARGET_DIR"] = str(tmp_path / "cargo-target")
    environment["CARGO_BUILD_JOBS"] = "4"
    environment["CARGO_TERM_COLOR"] = "never"

    def run(*command):
        result = subprocess.run(
            command,
            cwd=repository,
            env=environment,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            check=False,
        )
        print(result.stdout)
        return result

    vectors = run("make", "agent-test-vectors")
    assert vectors.returncode == 0, vectors.stdout
    assert "running 3 tests" in vectors.stdout, vectors.stdout
    for name in (
        "repository_corpora_are_loaded_and_fully_accounted",
        "unsupported_corpus_version_is_build_breaking",
        "unknown_vector_class_names_the_missing_capability",
    ):
        assert f"test {name} ... ok" in vectors.stdout, vectors.stdout
    assert "test result: ok. 3 passed; 0 failed;" in vectors.stdout, vectors.stdout
    generated = "retired budget pull excluded"
    assert generated in vectors.stdout, vectors.stdout
    assert vectors.stdout.index(generated) < vectors.stdout.index(
        "tools/runtime/run-with-clock.sh"
    ), vectors.stdout
    assert corpus.is_file()
    expected = corpus.read_bytes()
    assert expected

    (repository / PREPARE_TARGET).touch()
    regenerated = run("make", PREPARE_TARGET)
    assert regenerated.returncode == 0, regenerated.stdout
    assert generated in regenerated.stdout, regenerated.stdout
    assert corpus.read_bytes() == expected

    test_retained_eip1559_producer_consumer_conformance()
    assert corpus.read_bytes() == expected

    header = repository / "include/layerx/lx_budget.h"
    declarations, removed = re.subn(
        r"^\s*LX_BUDGET_DEFUND\s*=\s*0x00030008,\s*$",
        "",
        header.read_text(),
        flags=re.MULTILINE,
    )
    assert removed == 1
    header.write_text(declarations)
    refused = run("make", "agent-test-vectors")
    assert refused.returncode != 0, refused.stdout
    diagnostic = re.search(
        r"unexpected undeclared replay activities: \[(\d+)\]", refused.stdout
    )
    assert diagnostic is not None, refused.stdout
    assert int(diagnostic.group(1)) == 0x00030008, refused.stdout
    assert "tools/runtime/run-with-clock.sh" not in refused.stdout, refused.stdout
    assert corpus.read_bytes() == expected

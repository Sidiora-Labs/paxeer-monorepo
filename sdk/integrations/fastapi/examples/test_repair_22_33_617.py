from __future__ import annotations

import asyncio
import json
import os
import runpy
import signal
import sqlite3
import sys
import time
from base64 import b64encode
from concurrent.futures import ThreadPoolExecutor
from dataclasses import replace
from pathlib import Path

import httpx
import pytest
from fastapi import FastAPI

ROOT = Path(__file__).resolve().parents[4]
sys.path[:0] = [
    str(ROOT / "sdk/integrations/fastapi"),
    str(ROOT / "services/agent/sdk/python"),
]

from layerx_sdk.account_derivation import keccak256
from layerx_fastapi import LayerXResource
from layerx_fastapi.integration import LayerXSellerRuntime, layerx_payment_gate
from layerx_fastapi.protocol import (
    LayerXReceiptEvidence,
    PaymentPayload,
    decode_payment_required_header,
    decode_settlement_header,
    encode_payment_payload_header,
    verify_payment_receipt,
)
from layerx_fastapi.seller import (
    ReceiptPayloadAuthority,
    SellerMiddleware,
    idempotency_key,
    layerx_evidence_wire,
)
from layerx_fastapi.signatures import LayerXSignatureVerifier

BODY = b'{"access":"paid-resource"}'


@pytest.fixture
def example(tmp_path, monkeypatch):
    document = json.loads(
        (ROOT / "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json").read_text()
    )
    assert document["provenance"]["generator"] == "rpc/sequencer_receipt_vectors_test.go"
    vector = next(row for row in document["cases"] if row["id"] == "V1-layerxledger-pay-exact")
    log = vector["receiptJson"]["logs"][0]
    resource_file = tmp_path / "resource.json"
    resource_file.write_bytes(BODY)
    environment = {
        "LAYERX_PRINCIPAL": "merchant",
        "LAYERX_PROTECTED_PATH": "/paid",
        "LAYERX_RESOURCE_URL": "https://merchant.example/paid",
        "LAYERX_X402_SCHEME": "exact",
        "LAYERX_X402_NETWORK": "eip155:125",
        "LAYERX_PRICE": str(int(log["data"][66:], 16)),
        "LAYERX_ASSET": log["data"][:66],
        "LAYERX_PAY_TO": log["topics"][3],
        "LAYERX_PAYMENT_TIMEOUT_SECONDS": "120",
        "LAYERX_RECEIPT_KEYS_JSON": json.dumps({"keys": [vector["pinnedKey"]]}),
        "LAYERX_WEBHOOK_PATH": "/webhook",
        "LAYERX_WEBHOOK_PUBLIC_KEYS_JSON": json.dumps(
            {"receipt": b64encode(bytes.fromhex(vector["pinnedKey"]["publicKey"][2:])).decode()}
        ),
        "LAYERX_RESOURCE_FILE": str(resource_file),
        "LAYERX_FULFILLMENT_DIR": str(tmp_path / "fulfillments"),
    }
    for name, value in environment.items():
        monkeypatch.setenv(name, value)
    loaded = runpy.run_path(str(Path(__file__).with_name("paid_api.py")))
    config = loaded["mount"].config
    requirements = replace(
        config.requirements,
        extra={"layerx": {
            "commitment": "receipted",
            "account": "agent:did:layerx:payee:main",
            "currency": "LXP",
            "purposeHash": log["topics"][1][2:],
            "payer": log["topics"][2][2:],
        }},
        has_extra=True,
    )
    raw = bytes.fromhex(vector["rawTransaction"][2:])
    signed = bytes.fromhex(vector["signedReceipt"][2:])
    verified = verify_payment_receipt(signed, raw, config.receipt_keys, requirements)
    loaded.update(
        requirements=requirements,
        required=replace(config.payment_required, accepts=(requirements,)),
        directory=tmp_path / "fulfillments",
        effects=tmp_path / "resource.sqlite3",
        key=idempotency_key(config.principal, verified),
        evidence=layerx_evidence_wire(LayerXReceiptEvidence(
            raw_transaction=vector["rawTransaction"],
            signed_receipt=vector["signedReceipt"],
            receipt_digest="0x" + keccak256(signed).hex(),
            receipt_key=config.receipt_keys[0],
        )),
    )
    with sqlite3.connect(loaded["effects"]) as connection:
        connection.execute("CREATE TABLE effects (key TEXT PRIMARY KEY, body BLOB NOT NULL)")
        connection.execute("CREATE TABLE attempts (key TEXT NOT NULL)")
    return loaded


def application(example, body=BODY):
    base = example["StaticResourceHandler"]
    effects = example["effects"]

    class DurableResource(base):
        async def release_once(self, key):
            resource = await super().release_once(key)
            with sqlite3.connect(effects) as connection:
                connection.execute("INSERT INTO attempts VALUES (?)", (key,))
                connection.commit()
                connection.execute("BEGIN IMMEDIATE")
                connection.execute("INSERT OR IGNORE INTO effects VALUES (?, ?)", (key, resource.body))
                effect = connection.execute("SELECT body FROM effects WHERE key = ?", (key,)).fetchone()
                connection.commit()
            await asyncio.sleep(0.02)
            return LayerXResource(content_type=resource.content_type, body=effect[0])

    seller = SellerMiddleware(
        example["required"],
        ReceiptPayloadAuthority(example["mount"].config.receipt_keys),
        example["FileFulfillmentRepository"](example["directory"]),
        LayerXSignatureVerifier(),
    )
    runtime = LayerXSellerRuntime(seller, example["requirements"], "merchant", DurableResource(body))
    app = FastAPI()
    app.add_api_route("/paid", layerx_payment_gate(runtime), methods=["GET"], response_model=None)
    return app


async def request(app, header=None):
    async with httpx.AsyncClient(
        transport=httpx.ASGITransport(app=app), base_url="https://merchant.example"
    ) as client:
        return await client.get("/paid", headers={} if header is None else {"PAYMENT-SIGNATURE": header})


def payment_header(example):
    challenge = asyncio.run(request(application(example)))
    assert challenge.status_code == 402
    required = decode_payment_required_header(challenge.headers["PAYMENT-REQUIRED"])
    return encode_payment_payload_header(PaymentPayload(
        accepted=required.accepts[0], resource=required.resource, payload=example["evidence"]
    ))


def assert_paid(response, example, body=BODY):
    assert response.status_code == 200, response.text
    assert response.content == body
    settlement = decode_settlement_header(response.headers["PAYMENT-RESPONSE"])
    assert settlement.success
    assert settlement.extensions["layerx"] == example["evidence"]
    assert settlement.network == "eip155:125"
    assert settlement.amount == example["requirements"].amount
    assert response.headers["layerx-receipt-digest"] == example["evidence"]["receiptDigest"][2:]
    assert response.headers["layerx-transaction"] == settlement.transaction
    return dict(response.headers)


def counts(example):
    with sqlite3.connect(example["effects"]) as connection:
        effects = connection.execute("SELECT key, body FROM effects").fetchall()
        attempts = connection.execute("SELECT key FROM attempts").fetchall()
    assert all(row[0] == example["key"] for row in attempts)
    return effects, attempts


def test_018_concurrent_workers_and_reopen(example):
    header = payment_header(example)
    apps = [application(example) for _ in range(6)]
    with ThreadPoolExecutor(max_workers=len(apps)) as workers:
        responses = list(workers.map(lambda app: asyncio.run(request(app, header)), apps))
    expected_headers = assert_paid(responses[0], example)
    for response in responses[1:]:
        assert assert_paid(response, example) == expected_headers
    assert counts(example) == ([(example["key"], BODY)], [(example["key"],)])
    record = example["directory"] / (example["key"] + ".json")
    original = record.read_bytes()
    reopened = asyncio.run(request(application(example, b"changed after reopen"), header))
    assert assert_paid(reopened, example) == expected_headers
    assert record.read_bytes() == original
    assert counts(example) == ([(example["key"], BODY)], [(example["key"],)])
    with pytest.raises(LookupError):
        example["fulfillment_idempotency_key"].get()


PERSISTENCE_BOUNDARIES = [
    (phase, boundary)
    for phase in ("admission", "completion")
    for boundary in ("write", "flush", "fsync", "replace", "directory", "durable")
] + [("effect", "uncommitted"), ("effect", "committed"),
     ("directory", "create"), ("directory", "created")]


def crash_worker(example, header, phase, boundary):
    filename = example["FileFulfillmentRepository"].fulfill.__code__.co_filename
    lines = Path(filename).read_text().splitlines()
    test_lines = Path(__file__).read_text().splitlines()
    statements = {
        "write": "handle.write(", "flush": "handle.flush()", "fsync": "fsync(handle.fileno())",
        "replace": "replace(temporary, path)", "directory": "cls.sync_directory(path.parent)",
        "create": "parent.mkdir(", "created": "cls.sync_directory(parent.parent)",
    }

    def trace(frame, event, arg):
        if phase == "effect":
            if frame.f_code.co_name == "release_once" and frame.f_code.co_filename == __file__:
                if event == "line":
                    text = test_lines[frame.f_lineno - 1].strip()
                    if boundary == "uncommitted" and text == "connection.commit()" and "effect" in frame.f_locals:
                        os._exit(73)
                    if boundary == "committed" and text == "await asyncio.sleep(0.02)":
                        os._exit(73)
        elif frame.f_code.co_filename == filename:
            if phase == "directory" and frame.f_code.co_name == "ensure_directory":
                if event == "line" and lines[frame.f_lineno - 1].strip().startswith(statements[boundary]):
                    os._exit(73)
            elif frame.f_code.co_name == "write_atomic":
                path = frame.f_locals.get("path")
                suffix = ".admitted" if phase == "admission" else ".json"
                if path is not None and path.suffix == suffix:
                    if boundary == "durable" and event == "return":
                        os._exit(73)
                    if boundary != "durable" and event == "line":
                        if lines[frame.f_lineno - 1].strip().startswith(statements[boundary]):
                            os._exit(73)
        return trace

    child = os.fork()
    if child == 0:
        try:
            sys.settrace(trace)
            asyncio.run(request(application(example), header))
        finally:
            os._exit(74)
    deadline = time.monotonic() + 10
    while time.monotonic() < deadline:
        waited, status = os.waitpid(child, os.WNOHANG)
        if waited:
            assert os.waitstatus_to_exitcode(status) == 73, (phase, boundary, status)
            return
        time.sleep(0.01)
    os.kill(child, signal.SIGKILL)
    os.waitpid(child, 0)
    pytest.fail(f"worker did not reach {phase}/{boundary}")


@pytest.mark.parametrize(("phase", "boundary"), PERSISTENCE_BOUNDARIES)
def test_018_crash_reopen_each_persistence_boundary(example, phase, boundary):
    header = payment_header(example)
    crash_worker(example, header, phase, boundary)
    effects, attempts = counts(example)
    committed = phase == "completion" or (phase, boundary) == ("effect", "committed")
    assert effects == ([(example["key"], BODY)] if committed else [])
    assert len(attempts) == (1 if phase in ("effect", "completion") else 0)
    record = example["directory"] / (example["key"] + ".json")
    complete = phase == "completion" and boundary in ("directory", "durable")
    assert record.exists() == complete
    recovered = asyncio.run(request(application(example), header))
    expected_headers = assert_paid(recovered, example)
    assert counts(example)[0] == [(example["key"], BODY)]
    expected_attempts = len(attempts) + (0 if complete else 1)
    assert len(counts(example)[1]) == expected_attempts
    original = record.read_bytes()
    again = asyncio.run(request(application(example, b"new resource version"), header))
    assert assert_paid(again, example) == expected_headers
    assert record.read_bytes() == original
    assert len(counts(example)[1]) == expected_attempts


def test_018_admission_and_completion_refuse_receipt_reuse(example):
    header = payment_header(example)
    crash_worker(example, header, "admission", "durable")
    payload = PaymentPayload(
        accepted=example["requirements"],
        resource=replace(example["required"].resource, description="another request"),
        payload=example["evidence"],
    )
    changed = encode_payment_payload_header(payload)
    assert changed != header
    for complete in (False, True):
        if complete:
            assert_paid(asyncio.run(request(application(example), header)), example)
        before = counts(example)
        response = asyncio.run(request(application(example), changed))
        assert response.status_code == 409
        assert response.json() == {"detail": "settled_receipt_reuse"}
        assert counts(example) == before


def test_018_existing_complete_record_is_reused(example):
    header = payment_header(example)
    assert_paid(asyncio.run(request(application(example), header)), example)
    record = example["directory"] / (example["key"] + ".json")
    original = record.read_bytes()
    admission = record.with_suffix(".admitted")
    admission.unlink()
    assert_paid(asyncio.run(request(application(example, b"changed"), header)), example)
    assert record.read_bytes() == original
    assert not admission.exists()
    assert counts(example) == ([(example["key"], BODY)], [(example["key"],)])

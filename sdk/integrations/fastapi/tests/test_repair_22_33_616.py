"""Audit 22.30.163 findings 017 and 060: real receipt and mounted HTTP proofs."""
from pathlib import Path

import pytest
from fastapi.testclient import TestClient

from layerx_fastapi.config import IntegrationError, IntegrationErrorCode, offer_purpose_hash
from layerx_fastapi.protocol import (
    PAYMENT_REQUIRED_HEADER,
    PAYMENT_RESPONSE_HEADER,
    PAYMENT_SIGNATURE_HEADER,
    decode_payment_required_header,
    encode_payment_payload_header,
    verify_payment_receipt,
)

import test_package_surface as package_surface
import test_seller_receipts as seller_receipts


def test_finding_017_package_exports() -> None:
    package_surface.test_package_exports_no_authorized_batch_names()


def test_finding_017_mounted_paid_response_and_restart(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    package_surface.test_create_integration_uses_receipt_keys(tmp_path, monkeypatch)


def test_finding_060_native_receipt_and_tamper_refusal(tmp_path: Path) -> None:
    seller_receipts.test_seller_settles_signed_sequencer_receipt(tmp_path)


def test_finding_060_receipt_reuse_refusal(tmp_path: Path) -> None:
    seller_receipts.test_seller_refuses_a_settled_receipt_for_a_new_request(tmp_path)


def test_finding_060_durable_receipt_reuse_refusal(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    seller_receipts.test_example_repository_refuses_settled_receipt_reuse(tmp_path, monkeypatch)


def test_finding_060_strict_key_and_payment_refusal(tmp_path: Path) -> None:
    seller_receipts.test_verify_payment_receipt_refuses_unpinned_key(tmp_path)


def test_finding_060_mounted_current_evidence_and_restart(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    seller_receipts.test_mounted_seller_accepts_current_producer_evidence_and_replays(
        tmp_path, monkeypatch
    )


@pytest.mark.parametrize("purpose_hash", [None, "2a" * 32])
def test_mounted_receipt_for_another_purpose_is_refused(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, purpose_hash: str | None
) -> None:
    raw, signed, key, digest, config = seller_receipts._fixture()
    environment = seller_receipts.mounted_environment(tmp_path, monkeypatch, config, key)
    example = package_surface.mount_example_offer(
        seller_receipts.ROOT / "sdk/integrations/fastapi/examples/paid_api.py",
        environment,
        purpose_hash,
    )
    mount = example["mount"]
    expected_purpose = (
        offer_purpose_hash(config.payment_required.resource.url)
        if purpose_hash is None else purpose_hash
    )
    assert expected_purpose != config.requirements.extra["layerx"]["purposeHash"]
    assert verify_payment_receipt(
        signed, raw, config.receipt_keys, config.requirements
    ).receipt_digest.hex() == digest
    payload = seller_receipts._payload(mount.config, raw, signed, key)
    with TestClient(example["app"]) as client:
        challenge = client.get(config.protected_path)
        assert challenge.status_code == 402
        advertised = decode_payment_required_header(challenge.headers[PAYMENT_REQUIRED_HEADER])
        assert advertised.to_wire() == challenge.json()
        assert advertised.accepts == (mount.config.requirements,)
        assert advertised.accepts[0].extra["layerx"]["purposeHash"] == expected_purpose
        refused = client.get(config.protected_path, headers={
            PAYMENT_SIGNATURE_HEADER: encode_payment_payload_header(payload)
        })
        assert refused.status_code == 402
        assert refused.json() == {"error": "verification-failure"}
        assert PAYMENT_RESPONSE_HEADER not in refused.headers
    assert not Path(environment["LAYERX_FULFILLMENT_DIR"]).exists()


@pytest.mark.parametrize("purpose_hash", ["", "ab" * 31, "ab" * 33, "zz" * 32])
def test_mount_refuses_invalid_offer_purpose_hash(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, purpose_hash: str
) -> None:
    _, _, key, _, config = seller_receipts._fixture()
    environment = seller_receipts.mounted_environment(tmp_path, monkeypatch, config, key)
    with pytest.raises(IntegrationError) as raised:
        package_surface.mount_example_offer(
            seller_receipts.ROOT / "sdk/integrations/fastapi/examples/paid_api.py",
            environment,
            purpose_hash,
        )
    assert raised.value.code is IntegrationErrorCode.INVALID_DECLARED_KEY
    assert not Path(environment["LAYERX_FULFILLMENT_DIR"]).exists()

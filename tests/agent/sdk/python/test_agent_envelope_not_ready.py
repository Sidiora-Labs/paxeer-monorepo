import json
import sys
from pathlib import Path

import pytest

_ROOT = Path(__file__).resolve().parents[4]
sys.path.insert(0, str(_ROOT / "services/agent/sdk/python"))

from layerx_sdk.agent_http import (  # noqa: E402
    AgentEnvelopeSuccess,
    _envelope_reply,
    _valid_verification_status,
)
from layerx_sdk.production import PlatformSdkError, SdkErrorCode  # noqa: E402

_GOLDENS = _ROOT / "services/agent/schema/agent-api/golden"


@pytest.mark.parametrize("state", ["pending", "absent"])
def test_agent_envelope_accepts_not_ready_verification_status(state: str) -> None:
    encoded = (_GOLDENS / f"agent-http-success-{state}.json").read_bytes()
    expected = json.loads(encoded)

    response = _envelope_reply(200, "application/json", encoded, "7", False)

    assert isinstance(response, AgentEnvelopeSuccess)
    assert response.request_id == "7"
    assert response.value == expected["value"]
    assert response.value["found"] is False
    assert response.verification_status == expected["verification_status"]
    assert response.verification_status["state"] == state
    assert response.verification_status["state"] != "achieved"
    assert response.verification_status["requested"] == "receipted"
    if state == "pending":
        assert response.verification_status["achieved"] == "submitted"
        assert response.value["submitted"] == {
            "nonce": "7", "tx_hash": "0x" + "ab" * 32,
        }
    else:
        assert "achieved" not in response.verification_status


@pytest.mark.parametrize("state", ["pending", "absent"])
def test_agent_envelope_refuses_inconsistent_not_ready_goldens(state: str) -> None:
    envelope = json.loads((_GOLDENS / f"agent-http-success-{state}.json").read_bytes())
    status = envelope["verification_status"]
    status["achieved"] = status["requested"]

    with pytest.raises(PlatformSdkError) as raised:
        _envelope_reply(200, "application/json", json.dumps(envelope).encode(), "7", False)

    assert raised.value.code is SdkErrorCode.VERIFICATION_FAILURE
    assert raised.value.request_id == "7"
    assert raised.value.retry == "never"


@pytest.mark.parametrize("status", [
    {"state": "pending", "requested": "receipted", "achieved": "submitted"},
    {"state": "absent", "requested": "receipted"},
])
def test_not_ready_status_accepts_known_levels(status: dict[str, object]) -> None:
    assert _valid_verification_status(status)


@pytest.mark.parametrize("achieved", ["receipted", "committed"])
def test_pending_status_refuses_achieved_at_or_above_requested(achieved: str) -> None:
    assert not _valid_verification_status({
        "state": "pending", "requested": "receipted", "achieved": achieved,
    })


def test_absent_status_refuses_achieved() -> None:
    assert not _valid_verification_status({
        "state": "absent", "requested": "receipted", "achieved": "submitted",
    })


@pytest.mark.parametrize("status", [
    {"state": "pending", "requested": "receipted", "achieved": "submitted"},
    {"state": "absent", "requested": "receipted"},
])
def test_not_ready_status_refuses_unknown_fields(status: dict[str, object]) -> None:
    assert not _valid_verification_status({**status, "unexpected": True})


@pytest.mark.parametrize("status", [
    None,
    [],
    {"state": "unknown", "requested": "receipted"},
    {"state": "pending", "requested": "receipted"},
    {"state": "pending", "achieved": "submitted"},
    {"state": "pending", "requested": "unknown", "achieved": "submitted"},
    {"state": "pending", "requested": "receipted", "achieved": "unknown"},
    {"state": "pending", "requested": [], "achieved": "submitted"},
    {"state": "pending", "requested": "receipted", "achieved": []},
    {"state": "pending", "requested": "receipted", "achieved": "submitted", "reason": "pending"},
    {"state": "absent"},
    {"state": "absent", "requested": "unknown"},
    {"state": "absent", "requested": []},
])
def test_not_ready_status_refuses_malformed_shapes(status: object) -> None:
    assert not _valid_verification_status(status)


@pytest.mark.parametrize("rung", ["submitted", "receipted", "committed"])
def test_agent_envelope_accepts_achieved_rung(rung: str) -> None:
    envelope = json.loads((_GOLDENS / "agent-http-success.json").read_bytes())
    envelope["verification_status"]["rung"] = rung

    response = _envelope_reply(200, "application/json", json.dumps(envelope).encode(), "7", False)

    assert response.verification_status == {"state": "achieved", "rung": rung}
    assert response.value == envelope["value"]


@pytest.mark.parametrize("keep_rung", [False, True])
def test_agent_envelope_refuses_old_level_key(keep_rung: bool) -> None:
    envelope = json.loads((_GOLDENS / "agent-http-success.json").read_bytes())
    status = envelope["verification_status"]
    status["level"] = status["rung"]
    if not keep_rung:
        del status["rung"]

    with pytest.raises(PlatformSdkError) as raised:
        _envelope_reply(200, "application/json", json.dumps(envelope).encode(), "7", False)

    assert raised.value.code is SdkErrorCode.VERIFICATION_FAILURE
    assert raised.value.request_id == "7"
    assert raised.value.retry == "never"


@pytest.mark.parametrize("achieved, accepted", [
    ("submitted", True),
    ("receipted", False),
    ("committed", False),
])
def test_unverified_status_requires_achieved_below_requested(achieved: str, accepted: bool) -> None:
    assert _valid_verification_status({
        "state": "unverified", "requested": "receipted", "achieved": achieved,
        "reason": "receipt_pending",
    }) is accepted


@pytest.mark.parametrize("rung", [
    "Unverified", "SequencerSigned", "BatchIncluded", "StateProven",
    "CheckpointFinalised", "SettlementAnchored", "unknown", None, [],
])
def test_achieved_status_refuses_old_or_unknown_rungs(rung: object) -> None:
    assert not _valid_verification_status({"state": "achieved", "rung": rung})

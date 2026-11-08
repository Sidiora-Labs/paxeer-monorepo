from pathlib import Path

import pytest


def test_guest_within_deployment_bound():
    guest = (
        Path(__file__).resolve().parents[2]
        / "build/paxai-host-boundary/ai_market.wasm"
    )
    if not guest.is_file():
        pytest.skip("Build the guest with make -s paxai-host-boundary-build")
    size = len(guest.read_bytes())
    assert size <= 504184, f"ai-market guest is {size} bytes; maximum is 504184"

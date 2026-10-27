import sys
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

import layerx_sdk
from layerx_sdk import px


def test_network_head_has_no_anchor():
    document = {
        "network_id": "paxeer",
        "paxeer": {"chain_id": 125, "latest_block": 8},
        "layerx": None,
        "latest_committed_height": 7,
    }
    head = px.decode_px_network_head(document)
    assert head.network_id == "paxeer"
    assert head.paxeer.chain_id == 125
    assert head.paxeer.latest_block == 8
    assert head.latest_committed_height == 7
    assert head.layerx is None
    assert not hasattr(head, "anchor")
    assert not hasattr(px, "PxAnchorHead")
    assert not hasattr(px, "decode_px_anchor_head")
    assert not hasattr(layerx_sdk, "PxAnchorHead")
    assert not hasattr(layerx_sdk, "decode_px_anchor_head")
    assert "PxAnchorHead" not in layerx_sdk.__all__
    assert "decode_px_anchor_head" not in layerx_sdk.__all__
    assert layerx_sdk.decode_px_network_head is px.decode_px_network_head

    for anchor in (None, {}, {"latest_finalized_batch": 9}):
        with pytest.raises(ValueError, match="^invalid-px-finality$"):
            px.decode_px_network_head({**document, "anchor": anchor})


def test_history_cursor_is_block_position():
    account = "0x" + "11" * 20
    cursor = px.history_cursor(7, 2, 1)
    assert layerx_sdk.history_cursor is px.history_cursor
    assert "history_cursor" in layerx_sdk.__all__
    assert cursor == "00000000000000070000000200000001"
    assert px.history_params(account, cursor=cursor) == [account, cursor, None, None]
    assert px.history_params(account) == [account, None, None, None]

    for position, expected in (
        ((0, 0, 0), "0" * 32),
        (
            (0x0123456789ABCDEF, 0x12345678, 0x9ABCDEF0),
            "0123456789abcdef123456789abcdef0",
        ),
        (((1 << 64) - 1, (1 << 32) - 1, (1 << 32) - 1), "f" * 32),
    ):
        encoded = px.history_cursor(*position)
        assert encoded == expected
        assert px.history_params(account, cursor=encoded, limit=50, kind="transfer") == [
            account,
            encoded,
            50,
            "transfer",
        ]

    for index, maximum in enumerate(
        ((1 << 64) - 1, (1 << 32) - 1, (1 << 32) - 1)
    ):
        for invalid in (-1, maximum + 1, True, False, 1.0, "1", None):
            position = [7, 2, 1]
            position[index] = invalid
            with pytest.raises(ValueError, match="^invalid-history-position$"):
                px.history_cursor(*position)

    for invalid in (
        "",
        cursor[:-1],
        cursor + "0",
        "0x" + cursor,
        "F" * 32,
        "g" * 32,
        " " + cursor,
        cursor + "\n",
        "\uff10" * 32,
        7,
        True,
        cursor.encode("ascii"),
    ):
        with pytest.raises(ValueError, match="^invalid-history-cursor$"):
            px.history_params(account, cursor=invalid)

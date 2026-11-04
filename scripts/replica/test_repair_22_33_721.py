"""Exercise the fixture producer and transported codec bytes without native tools."""

from functools import partial
import hashlib
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
import importlib.util
from pathlib import Path
import re
import struct
from threading import Thread
from urllib.request import ProxyHandler, build_opener

import pytest


ROOT = Path(__file__).resolve().parents[2]
PRODUCER = ROOT / "scripts/replica/lxp_replay_fixture.py"
HISTORICAL = ROOT / "tests/vectors/replay_corpus.lxb"
RETIRED_BUDGET_PULL = 0x0003000B


def load_producer(path):
    spec = importlib.util.spec_from_file_location("fixture_producer", path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


@pytest.fixture
def producer(tmp_path):
    # Relocate the actual CLI into an isolated checkout layout so its default
    # output path is exercised without changing out-of-scope repository vectors.
    path = tmp_path / "scripts/replica/lxp_replay_fixture.py"
    path.parent.mkdir(parents=True)
    path.write_bytes(PRODUCER.read_bytes())
    return load_producer(path)


class Reader:
    def __init__(self, data):
        self.data = data
        self.offset = 0

    def take(self, count):
        assert count >= 0
        end = self.offset + count
        assert end <= len(self.data), "truncated codec bytes"
        value = self.data[self.offset:end]
        self.offset = end
        return value

    def integer(self, width):
        return int.from_bytes(self.take(width), "big")

    def sized(self):
        return self.take(self.integer(4))

    def finished(self):
        assert self.offset == len(self.data), "trailing codec bytes"


def digest(tag, payload):
    return hashlib.sha256(tag + b"\0" + payload).digest()


def sized(payload):
    return len(payload).to_bytes(4, "big") + payload


def consume_corpus(payload, *, compatibility=False):
    """Independently check the real wire layout, bindings and complete chains."""
    reader = Reader(payload)
    assert reader.take(8) == b"LXPRP001"
    assert reader.integer(4) == 1
    count = reader.integer(4)
    assert 0 < count <= 256
    final_state = reader.take(32)
    final_digest = reader.take(32)
    previous_state = previous_batch = previous_digest = bytes(32)
    types = []
    boundaries = []
    for sequence in range(1, count + 1):
        assert reader.integer(8) == sequence
        boundary = reader.integer(1)
        expected_boundary = (
            sequence % 7 == 0
            or sequence == count
            or (compatibility and sequence == 53)
        )
        assert boundary == int(expected_boundary)
        if boundary:
            boundaries.append(sequence)
        encoded = reader.sized()
        envelope = Reader(encoded)
        assert envelope.take(5) == b"\x00\x01\x10\x01\x0c"
        fields = {}
        fixed_widths = {1: 2, 2: 4, 3: 4, 6: 8, 7: 16, 9: 16}
        for tag in range(1, 13):
            assert envelope.integer(1) == tag
            fields[tag] = (
                envelope.take(fixed_widths[tag])
                if tag in fixed_widths
                else envelope.sized()
            )
        envelope.finished()
        activity_type = int.from_bytes(fields[3], "big")
        types.append(activity_type)
        assert fields[1] == b"\x00\x01"
        assert fields[2] == (77).to_bytes(4, "big")
        assert fields[4] == b"did:lx:crossarch"
        assert fields[5] == b"\xa1\x01"
        assert fields[6] == sequence.to_bytes(8, "big")
        assert fields[7] == struct.pack(">QQ", 1_700_000_000_000, 1_700_000_100_000)
        assert fields[8] == hashlib.sha256(
            b"LayerX replay idempotency" + fields[3]
        ).digest()
        assert fields[9] == (1).to_bytes(16, "big")
        assert fields[11] == fields[3] + fields[6]
        assert fields[10] == digest(b"LXP/v1/payload-hash", fields[11])
        signature_seed = hashlib.sha256(
            b"LayerX replay signature" + fields[3]
        ).digest()
        assert fields[12] == signature_seed * 2
        activity_id = digest(b"LXP/v1/activity-id", encoded)
        state = reader.take(32)
        assert state == digest(
            b"LXP/v1/state-root-chain",
            previous_state + sequence.to_bytes(8, "big") + activity_id,
        )
        receipt = reader.sized()
        assert receipt == (
            struct.pack(">HQ", 1, sequence) + activity_id + previous_state + state
        )
        event = reader.sized()
        assert event == fields[3] + activity_id
        batch = reader.take(32)
        assert batch == (
            digest(
                b"LXP/v1/batch-header",
                previous_batch + state + sequence.to_bytes(8, "big"),
            )
            if boundary
            else bytes(32)
        )
        previous_digest = digest(
            b"LXP/v1/state-root-chain",
            previous_digest
            + sized(receipt)
            + sized(event)
            + bytes((boundary,))
            + (batch if boundary else b""),
        )
        previous_state = state
        if boundary:
            previous_batch = batch
    reader.finished()
    assert previous_state == final_state
    assert previous_digest == final_digest
    assert boundaries[-1] == count
    assert len(set(types)) == count
    return tuple(types), boundaries


def declared_codec_types():
    # Use the kernel declarations as the independent inventory for the six
    # original codec modules; the oracle does not import the producer's table.
    values = {
        int(value, 16)
        for header in (ROOT / "include/layerx").glob("*.h")
        for value in re.findall(
            r"\bLX_\w+\s*=\s*(0x[0-9a-fA-F]{8})\b", header.read_text()
        )
        if 1 <= int(value, 16) >> 16 <= 6
    }
    assert len(values) == 60
    return tuple(sorted(values))


def test_replica_fixture_current_declared_activity_corpus():
    producer = load_producer(PRODUCER)
    expected = declared_codec_types()
    types, boundaries = consume_corpus(producer.build())
    assert types == expected
    assert producer.ACTIVITY_TYPES == producer.REPLAY_ACTIVITY_TYPES == expected
    assert RETIRED_BUDGET_PULL not in types
    assert {0x00010009, 0x0001000A, 0x0001000B} <= set(types)
    assert boundaries == [7, 14, 21, 28, 35, 42, 49, 56, 60]
    assert producer.build() == producer.build()


def test_replica_fixture_canonical_cli_producer_transport_consumer(
    producer, tmp_path, capsys
):
    producer.main([])
    output = tmp_path / "tests/vectors/codec/replay_corpus.lxb"
    assert producer.OUTPUT == output
    assert not (tmp_path / "tests/vectors/replay_corpus.lxb").exists()
    generated = output.read_bytes()
    assert generated == producer.build()
    assert capsys.readouterr().out == (
        f"tests/vectors/codec/replay_corpus.lxb 60 {len(generated)} "
        "(declared activities; codec fixture only)\n"
    )

    handler = partial(SimpleHTTPRequestHandler, directory=str(tmp_path))
    with ThreadingHTTPServer(("127.0.0.1", 0), handler) as server:
        worker = Thread(target=server.serve_forever, daemon=True)
        worker.start()
        try:
            # A real HTTP client reads the file emitted by the production CLI.
            # Disable ambient proxy settings for this loopback-only exchange.
            client = build_opener(ProxyHandler({}))
            url = (
                f"http://127.0.0.1:{server.server_port}"
                "/tests/vectors/codec/replay_corpus.lxb"
            )
            with client.open(url, timeout=5) as response:
                assert response.status == 200
                assert response.headers.get_content_type() == "application/octet-stream"
                assert int(response.headers["Content-Length"]) == len(generated)
                transported = response.read()
            assert transported == generated
            types, _ = consume_corpus(transported)
            assert types == declared_codec_types()
            assert RETIRED_BUDGET_PULL not in types
        finally:
            server.shutdown()
            worker.join(timeout=5)
            assert not worker.is_alive()


def test_replica_fixture_explicit_compatibility_preserves_recorded_bytes(
    producer, tmp_path, capsys
):
    producer.main(["--compatibility"])
    output = tmp_path / "tests/vectors/replay_corpus.lxb"
    assert producer.COMPATIBILITY_OUTPUT == output
    assert not producer.OUTPUT.exists()
    recorded = HISTORICAL.read_bytes()
    assert hashlib.sha256(recorded).hexdigest() == (
        "ce0c6277c0be72d8774ab1aece20a2bf6dd712549daeee88f99f593c2a9be60d"
    )
    assert output.read_bytes() == producer.build(compatibility=True) == recorded
    types, boundaries = consume_corpus(recorded, compatibility=True)
    assert len(types) == 58
    assert types == producer.COMPATIBILITY_ACTIVITY_TYPES
    assert types[:53] == producer.ORIGINAL_ACTIVITY_TYPES
    assert types[-5:] == (0x00030008, 0x00030009, 0x0003000A, 0x0003000B, 0x00040008)
    assert boundaries == [7, 14, 21, 28, 35, 42, 49, 53, 56, 58]
    assert capsys.readouterr().out == (
        f"tests/vectors/replay_corpus.lxb 58 {len(recorded)} "
        "(historical compatibility; codec fixture only)\n"
    )
    # A compatibility call must not switch subsequent default API calls.
    current, _ = consume_corpus(producer.build())
    assert current == declared_codec_types()


@pytest.mark.parametrize("compatibility", [False, True])
def test_replica_fixture_cli_explicit_output(producer, tmp_path, compatibility):
    output = tmp_path / "custom/nested/corpus.lxb"
    args = ["--output", str(output)]
    if compatibility:
        args.append("--compatibility")
    producer.main(args)
    assert output.read_bytes() == producer.build(compatibility=compatibility)
    assert not producer.OUTPUT.exists()
    assert not producer.COMPATIBILITY_OUTPUT.exists()


def test_replica_fixture_cli_identifies_archival_bytes(producer, capsys):
    with pytest.raises(SystemExit) as result:
        producer.main(["--help"])
    assert result.value.code == 0
    help_text = " ".join(capsys.readouterr().out.split())
    assert "these bytes do not deploy the retired replica" in help_text
    assert "--compatibility" in help_text
    assert "including retired budget pull" in help_text
    assert "codec/replay_corpus.lxb" in help_text


def test_replica_fixture_cli_rejects_implicit_legacy_mode(producer):
    with pytest.raises(SystemExit) as result:
        producer.main(["--compatibility=false"])
    assert result.value.code == 2
    assert not producer.OUTPUT.exists()
    assert not producer.COMPATIBILITY_OUTPUT.exists()

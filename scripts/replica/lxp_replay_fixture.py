#!/usr/bin/env python3
"""Generate codec fixtures; these bytes do not deploy the retired replica.

The default corpus covers the currently declared activities in the six original
codec modules. Compatibility mode preserves the historical ordering, retired
budget-pull entry and batch boundaries for byte-level archival comparisons.
Neither corpus is an executable activity workload or a running batch service.
"""

import argparse
import hashlib
import pathlib
import struct

ROOT = pathlib.Path(__file__).resolve().parents[2]
OUTPUT = ROOT / "tests" / "vectors" / "codec" / "replay_corpus.lxb"
COMPATIBILITY_OUTPUT = ROOT / "tests" / "vectors" / "replay_corpus.lxb"

ACTIVITY_TYPES = tuple(
    (module << 16) | ordinal
    for module, maximum in ((1, 11), (2, 7), (3, 10), (4, 8), (5, 13), (6, 11))
    for ordinal in range(1, maximum + 1)
)
ORIGINAL_ACTIVITY_TYPES = tuple(
    (module << 16) | ordinal
    for module, maximum in ((1, 8), (2, 7), (3, 7), (4, 7), (5, 13), (6, 11))
    for ordinal in range(1, maximum + 1)
)
REPLAY_ACTIVITY_TYPES = ACTIVITY_TYPES
COMPATIBILITY_ACTIVITY_TYPES = ORIGINAL_ACTIVITY_TYPES + tuple(
    (module << 16) | ordinal
    for module, maximum in ((1, 8), (2, 7), (3, 11), (4, 8), (5, 13), (6, 11))
    for ordinal in range(1, maximum + 1)
    if ((module << 16) | ordinal) not in ORIGINAL_ACTIVITY_TYPES
)


def domain(tag: bytes, payload: bytes) -> bytes:
    return hashlib.sha256(tag + b"\0" + payload).digest()


def sized(payload: bytes) -> bytes:
    return struct.pack(">I", len(payload)) + payload


def activity(activity_type: int, sequence: int) -> bytes:
    actor = b"did:lx:crossarch"
    authority = b"\xa1\x01"
    payload = struct.pack(">IQ", activity_type, sequence)
    idempotency = hashlib.sha256(
        b"LayerX replay idempotency" + struct.pack(">I", activity_type)
    ).digest()
    signature_seed = hashlib.sha256(
        b"LayerX replay signature" + struct.pack(">I", activity_type)
    ).digest()
    parts = [struct.pack(">HHB", 1, 0x1001, 12)]
    parts += [b"\x01" + struct.pack(">H", 1)]
    parts += [b"\x02" + struct.pack(">I", 77)]
    parts += [b"\x03" + struct.pack(">I", activity_type)]
    parts += [b"\x04" + sized(actor)]
    parts += [b"\x05" + sized(authority)]
    parts += [b"\x06" + struct.pack(">Q", sequence)]
    parts += [b"\x07" + struct.pack(">QQ", 1_700_000_000_000, 1_700_000_100_000)]
    parts += [b"\x08" + sized(idempotency)]
    parts += [b"\x09" + (1).to_bytes(16, "big")]
    parts += [b"\x0a" + sized(domain(b"LXP/v1/payload-hash", payload))]
    parts += [b"\x0b" + sized(payload)]
    parts += [b"\x0c" + sized(signature_seed + signature_seed)]
    return b"".join(parts)


def build(*, compatibility: bool = False) -> bytes:
    """Encode current fixtures, or explicitly request the historical bytes."""
    activity_types = (
        COMPATIBILITY_ACTIVITY_TYPES if compatibility else REPLAY_ACTIVITY_TYPES
    )
    records = []
    previous_state = bytes(32)
    previous_batch = bytes(32)
    previous_digest = bytes(32)
    for sequence, activity_type in enumerate(activity_types, 1):
        encoded = activity(activity_type, sequence)
        activity_id = domain(b"LXP/v1/activity-id", encoded)
        state = domain(
            b"LXP/v1/state-root-chain",
            previous_state + struct.pack(">Q", sequence) + activity_id,
        )
        receipt = struct.pack(">HQ", 1, sequence) + activity_id + previous_state + state
        event = struct.pack(">I", activity_type) + activity_id
        boundary = (
            sequence % 7 == 0
            or sequence == len(activity_types)
            or (compatibility and sequence == len(ORIGINAL_ACTIVITY_TYPES))
        )
        batch = bytes(32)
        if boundary:
            batch = domain(
                b"LXP/v1/batch-header",
                previous_batch + state + struct.pack(">Q", sequence),
            )
        digest_input = (
            previous_digest
            + sized(receipt)
            + sized(event)
            + bytes((int(boundary),))
            + (batch if boundary else b"")
        )
        digest = domain(b"LXP/v1/state-root-chain", digest_input)
        record = (
            struct.pack(">QB", sequence, int(boundary))
            + sized(encoded)
            + state
            + sized(receipt)
            + sized(event)
            + batch
        )
        records.append(record)
        previous_state = state
        if boundary:
            previous_batch = batch
        previous_digest = digest
    header = (
        b"LXPRP001"
        + struct.pack(">II", 1, len(records))
        + previous_state
        + previous_digest
    )
    return header + b"".join(records)


def main(argv: list[str] | None = None) -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--compatibility",
        action="store_true",
        help="emit historical codec bytes, including retired budget pull",
    )
    parser.add_argument(
        "--output",
        type=pathlib.Path,
        help="output file (default: codec/replay_corpus.lxb, or the historical "
        "vectors/replay_corpus.lxb in compatibility mode)",
    )
    args = parser.parse_args(argv)
    payload = build(compatibility=args.compatibility)
    output = (
        args.output
        if args.output is not None
        else COMPATIBILITY_OUTPUT if args.compatibility else OUTPUT
    ).resolve()
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_bytes(payload)
    display = output.relative_to(ROOT) if output.is_relative_to(ROOT) else output
    count = len(
        COMPATIBILITY_ACTIVITY_TYPES if args.compatibility else REPLAY_ACTIVITY_TYPES
    )
    mode = "historical compatibility" if args.compatibility else "declared activities"
    print(f"{display} {count} {len(payload)} ({mode}; codec fixture only)")


if __name__ == "__main__":
    main()

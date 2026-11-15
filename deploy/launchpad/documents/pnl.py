"""Compose the PnL inventory runtime document for chain 125."""

from __future__ import annotations

import json
import re
from pathlib import Path
from typing import Any

from common import DocumentError


IDENTIFIER = re.compile(r"[A-Za-z0-9_-]{1,128}")
REGION = re.compile(r"[a-z][a-z0-9-]{1,62}")


def _object(value: Any, keys: set[str], key: str) -> dict[str, Any]:
    if not isinstance(value, dict):
        raise DocumentError(f"pnl: invalid value {key}")
    for name in sorted(keys - value.keys()):
        raise DocumentError(f"pnl: missing value {key}.{name}")
    for name in sorted(value.keys() - keys):
        raise DocumentError(f"pnl: unknown value {key}.{name}")
    return value


def _name(value: Any, key: str, pattern: re.Pattern[str] = IDENTIFIER) -> str:
    if not isinstance(value, str) or pattern.fullmatch(value) is None:
        raise DocumentError(f"pnl: invalid value {key}")
    return value


def _integer(value: Any, key: str, minimum: int, maximum: int) -> int:
    if type(value) is not int or not minimum <= value <= maximum:
        raise DocumentError(f"pnl: invalid value {key}")
    return value


def documents(service: str, values: str | Path) -> dict[str, Any]:
    if service != "pnl":
        raise DocumentError("pnl: invalid value service")
    directory = Path(values)
    path = next(
        (directory / name for name in ("pnl.json", "pnl.example.json") if (directory / name).is_file()),
        None,
    )
    if path is None:
        raise DocumentError("pnl: missing values file pnl.json")
    try:
        source = json.loads(path.read_bytes())
    except (OSError, ValueError) as exc:
        raise DocumentError(f"pnl: invalid values file {path.name}") from exc
    source = _object(source, {
        "sourceStream", "consumerId", "generation", "owner",
        "projectionVersion", "epoch", "region", "consumer",
    }, "pnl")
    consumer = _object(source["consumer"], {
        "stream", "durable", "batch", "maxAckPending", "maxWaiting",
        "maxDeliver", "fetchWaitMS", "ackWaitMS",
    }, "consumer")
    projection = source["projectionVersion"]
    try:
        valid_projection = isinstance(projection, str) and 0 < len(projection.encode("utf-8")) <= 128
    except UnicodeEncodeError:
        valid_projection = False
    if not valid_projection:
        raise DocumentError("pnl: invalid value projectionVersion")
    batch = _integer(consumer["batch"], "consumer.batch", 1, 64)
    document = {
        "schema": "pnl",
        "sourceStream": _name(source["sourceStream"], "sourceStream"),
        "consumerId": _name(source["consumerId"], "consumerId"),
        "generation": _name(source["generation"], "generation"),
        "owner": _name(source["owner"], "owner"),
        "projectionVersion": projection,
        "epoch": _integer(source["epoch"], "epoch", 1, (1 << 63) - 1),
        "region": _name(source["region"], "region", REGION),
        "consumer": {
            "Stream": _name(consumer["stream"], "consumer.stream"),
            "Durable": _name(consumer["durable"], "consumer.durable"),
            "Subject": "accounting.inventory.v1",
            "Batch": batch,
            "MaxAckPending": _integer(consumer["maxAckPending"], "consumer.maxAckPending", batch, 10000),
            "MaxWaiting": _integer(consumer["maxWaiting"], "consumer.maxWaiting", 1, 32),
            "MaxDeliver": _integer(consumer["maxDeliver"], "consumer.maxDeliver", 1, 100),
            "FetchWait": _integer(consumer["fetchWaitMS"], "consumer.fetchWaitMS", 10, 1000) * 1_000_000,
            "AckWait": _integer(consumer["ackWaitMS"], "consumer.ackWaitMS", 1000, 3_600_000) * 1_000_000,
        },
    }
    return {"DELIVERY_CONFIG_DOCUMENT": document}

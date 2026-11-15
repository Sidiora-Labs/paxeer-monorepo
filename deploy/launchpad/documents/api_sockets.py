"""Bounded stream topic assignments for the sockets API."""

from __future__ import annotations

from pathlib import Path
from typing import Any

from api import integer_value, object_keys, read_values, string_value
from common import DocumentError


def documents(service: str, values: str | Path) -> dict[str, Any]:
    if service != "api-sockets":
        raise DocumentError(f"{service}: expected api-sockets")
    assignments = read_values(service, values)
    if not isinstance(assignments, list) or not 1 <= len(assignments) <= 128:
        raise DocumentError("STREAM_CONSUMER_CONFIG_DOCUMENT: expected 1..128 assignments")
    keys = (
        "Stream", "Durable", "Subject", "Batch", "MaxAckPending", "MaxWaiting",
        "MaxDeliver", "FetchWaitMS", "AckWaitMS",
    )
    document = []
    seen = set()
    for index, assignment in enumerate(assignments):
        prefix = f"STREAM_CONSUMER_CONFIG_DOCUMENT[{index}]"
        assignment = object_keys(assignment, keys, prefix)
        for key in ("Stream", "Durable"):
            string_value(assignment[key], f"{prefix}.{key}", 128, r"[A-Za-z0-9_-]{1,128}")
        subject = string_value(
            assignment["Subject"], f"{prefix}.Subject", 200,
            r"[A-Za-z0-9_-]+(?:\.[A-Za-z0-9_-]+)*",
        )
        if subject in seen:
            raise DocumentError(f"{prefix}.Subject: duplicate assignment")
        seen.add(subject)
        batch = integer_value(assignment["Batch"], f"{prefix}.Batch", 1, 64)
        integer_value(assignment["MaxAckPending"], f"{prefix}.MaxAckPending", batch, 10000)
        integer_value(assignment["MaxWaiting"], f"{prefix}.MaxWaiting", 1, 32)
        integer_value(assignment["MaxDeliver"], f"{prefix}.MaxDeliver", 1, 100)
        fetch_ms = integer_value(assignment["FetchWaitMS"], f"{prefix}.FetchWaitMS", 10, 1000)
        ack_ms = integer_value(assignment["AckWaitMS"], f"{prefix}.AckWaitMS", 1000, 3600000)
        consumer = {key: assignment[key] for key in keys if key not in ("FetchWaitMS", "AckWaitMS")}
        consumer["FetchWait"] = fetch_ms * 1000000
        consumer["AckWait"] = ack_ms * 1000000
        document.append(consumer)
    return {"STREAM_CONSUMER_CONFIG_DOCUMENT": document}

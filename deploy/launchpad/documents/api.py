"""Accounting read-model startup document for the REST API."""

from __future__ import annotations

import json
import re
from pathlib import Path
from typing import Any

from common import DocumentError


def read_values(service: str, values: str | Path) -> Any:
    for name in (f"{service}.json", f"{service}.example.json"):
        path = Path(values) / name
        if path.is_file():
            try:
                return json.loads(path.read_bytes(), object_pairs_hook=unique_keys)
            except (OSError, UnicodeError, json.JSONDecodeError) as exc:
                raise DocumentError(f"{service}: invalid values file {name}") from exc
    raise DocumentError(f"{service}: missing values file {service}.json")


def unique_keys(pairs: list[tuple[str, Any]]) -> dict[str, Any]:
    result = {}
    for key, value in pairs:
        if key in result:
            raise DocumentError(f"{key}: duplicate value")
        result[key] = value
    return result


def object_keys(value: Any, keys: tuple[str, ...], key: str) -> dict[str, Any]:
    if not isinstance(value, dict):
        raise DocumentError(f"{key}: expected an object")
    for field in keys:
        if field not in value:
            raise DocumentError(f"{key}.{field}: missing value")
    for field in value:
        if field not in keys:
            raise DocumentError(f"{key}.{field}: unknown value")
    return value


def string_value(value: Any, key: str, maximum: int, pattern: str | None = None) -> str:
    try:
        length = len(value.encode("utf-8")) if isinstance(value, str) else 0
    except UnicodeError as exc:
        raise DocumentError(f"{key}: invalid UTF-8 string") from exc
    if (
        not isinstance(value, str)
        or not 1 <= length <= maximum
        or (pattern is not None and re.fullmatch(pattern, value) is None)
    ):
        raise DocumentError(f"{key}: invalid string")
    return value


def integer_value(value: Any, key: str, minimum: int, maximum: int) -> int:
    if type(value) is not int or not minimum <= value <= maximum:
        raise DocumentError(f"{key}: expected an integer in {minimum}..{maximum}")
    return value


def documents(service: str, values: str | Path) -> dict[str, Any]:
    if service != "api":
        raise DocumentError(f"{service}: expected api")
    schemas = (
        "ChartsSchema", "StatsSchema", "ChartsDeliverySchema", "StatsDeliverySchema",
        "StreamSchema", "SourcePartitionSchema",
    )
    tokens = ("ProjectionGeneration", "ChartsConsumerID", "StatsConsumerID")
    keys = schemas + tokens + ("ProjectionVersion", "SourceStream")
    document = object_keys(read_values(service, values), keys, "ACCOUNTING_READ_CONFIG_DOCUMENT")
    for key in schemas:
        string_value(document[key], key, 63, r"[a-z][a-z0-9_]{0,62}")
    for key in tokens:
        string_value(document[key], key, 128, r"[a-zA-Z0-9_-]{1,128}")
    string_value(document["ProjectionVersion"], "ProjectionVersion", 64)
    string_value(document["SourceStream"], "SourceStream", 128)
    if document["ChartsConsumerID"] == document["StatsConsumerID"]:
        raise DocumentError("StatsConsumerID: must differ from ChartsConsumerID")
    return {"ACCOUNTING_READ_CONFIG_DOCUMENT": document}

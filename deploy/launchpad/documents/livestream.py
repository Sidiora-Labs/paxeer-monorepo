"""Validate deployment values for the livestream service.

Livestream has no service-specific startup document.  Its startup contract is
the shared database budget and expected-schema document supplied by
``common.py``; this module keeps the deployment values that describe the
provider and the small mainnet-beta runtime checked before composition.
"""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any
from urllib.parse import urlparse


class LivestreamValuesError(ValueError):
    """The livestream deployment values are missing or malformed."""


def _read_values(values: Path) -> dict[str, Any]:
    for name in ("livestream.json", "livestream.example.json"):
        path = values / name
        if path.is_file():
            try:
                value = json.loads(path.read_bytes())
            except (OSError, json.JSONDecodeError) as exc:
                raise LivestreamValuesError(f"livestream: invalid {name}") from exc
            if not isinstance(value, dict):
                raise LivestreamValuesError(f"livestream: {name} must be an object")
            return value
    raise LivestreamValuesError("livestream: missing values file livestream.json")


def _required(value: dict[str, Any], key: str) -> Any:
    if key not in value:
        raise LivestreamValuesError(f"livestream: missing value {key}")
    return value[key]


def _string(value: dict[str, Any], key: str) -> str:
    candidate = _required(value, key)
    if not isinstance(candidate, str) or not candidate.strip():
        raise LivestreamValuesError(f"livestream: invalid value {key}")
    return candidate.strip()


def _positive_int(value: dict[str, Any], key: str) -> int:
    candidate = _required(value, key)
    if isinstance(candidate, bool) or not isinstance(candidate, int) or candidate <= 0:
        raise LivestreamValuesError(f"livestream: invalid value {key}")
    return candidate


def _https_url(value: dict[str, Any], key: str, *, path: str | None = None) -> None:
    raw = _string(value, key)
    parsed = urlparse(raw)
    if (
        parsed.scheme != "https"
        or not parsed.netloc
        or parsed.username is not None
        or parsed.query
        or parsed.fragment
        or (path is not None and parsed.path != path)
    ):
        raise LivestreamValuesError(f"livestream: invalid value {key}")


def _validate(value: dict[str, Any]) -> None:
    network = _required(value, "network")
    if not isinstance(network, dict):
        raise LivestreamValuesError("livestream: invalid value network")
    if network.get("name") != "paxeer-mainnet-beta":
        raise LivestreamValuesError("livestream: invalid value network.name")
    chain_id = network.get("evm_chain_id")
    if isinstance(chain_id, bool) or chain_id != 125:
        raise LivestreamValuesError("livestream: invalid value network.evm_chain_id")
    rpc_url = network.get("rpc_url")
    if not isinstance(rpc_url, str) or not rpc_url.strip():
        raise LivestreamValuesError("livestream: invalid value network.rpc_url")
    parsed_rpc = urlparse(rpc_url)
    if parsed_rpc.scheme != "https" or not parsed_rpc.netloc or parsed_rpc.path not in ("", "/") or parsed_rpc.query or parsed_rpc.fragment:
        raise LivestreamValuesError("livestream: invalid value network.rpc_url")

    endpoints = _required(value, "endpoints")
    if not isinstance(endpoints, dict):
        raise LivestreamValuesError("livestream: invalid value endpoints")
    _https_url(endpoints, "livepeer_api_url", path="/api")
    _https_url(endpoints, "public_url")

    addresses = _required(value, "addresses")
    if not isinstance(addresses, dict):
        raise LivestreamValuesError("livestream: invalid value addresses")
    bind = _string(addresses, "bind")
    if bind not in {"0.0.0.0", "::"}:
        raise LivestreamValuesError("livestream: invalid value addresses.bind")
    port = _positive_int(addresses, "port")
    if port > 65535:
        raise LivestreamValuesError("livestream: invalid value addresses.port")

    names = _required(value, "names")
    if not isinstance(names, dict):
        raise LivestreamValuesError("livestream: invalid value names")
    for key in ("service", "runtime", "zone"):
        _string(names, key)

    limits = _required(value, "limits")
    if not isinstance(limits, dict):
        raise LivestreamValuesError("livestream: invalid value limits")
    wallet = _positive_int(limits, "max_streams_per_wallet")
    global_limit = _positive_int(limits, "max_global_streams")
    if wallet > global_limit:
        raise LivestreamValuesError("livestream: invalid value limits.max_streams_per_wallet")

    periods = _required(value, "periods")
    if not isinstance(periods, dict):
        raise LivestreamValuesError("livestream: invalid value periods")
    _positive_int(periods, "secrets_refresh_seconds")
    timestamp_unit = _string(periods, "webhook_timestamp_unit")
    if timestamp_unit not in {"seconds", "milliseconds"}:
        raise LivestreamValuesError("livestream: invalid value periods.webhook_timestamp_unit")

    credentials = _required(value, "credentials")
    if not isinstance(credentials, dict):
        raise LivestreamValuesError("livestream: invalid value credentials")
    _string(credentials, "livepeer_api_key")
    if credentials.get("is_test") is not True:
        raise LivestreamValuesError("livestream: invalid value credentials.is_test")


def documents(service: str, values: str | Path) -> dict[str, Any]:
    """Validate livestream values and return no service-specific documents."""

    if service != "livestream":
        raise LivestreamValuesError(f"{service}: unsupported document service")
    value = _read_values(Path(values))
    _validate(value)
    return {}


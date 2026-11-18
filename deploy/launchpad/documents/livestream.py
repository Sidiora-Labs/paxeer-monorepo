"""Compose deployment values for the livestream service.

The shared database budget and expected-schema documents come from
``common.py``. Connected broadcasting also carries its bound budget policy;
this module emits the settings for each runtime profile.
"""

from __future__ import annotations

import ipaddress
import json
import re
from decimal import Decimal
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
    environment = _required(value, "environment")
    if not isinstance(environment, dict) or any(not isinstance(v, str) for v in environment.values()):
        raise LivestreamValuesError("livestream: invalid value environment")
    platform = environment.get("RUNTIME_PROFILE") == "platform"
    if environment.get("RUNTIME_PROFILE") not in {"container", "platform"}:
        raise LivestreamValuesError("livestream: RUNTIME_PROFILE must be container or platform")
    if not platform:
        _https_url(endpoints, "livepeer_api_url", path="/api")
        if endpoints["livepeer_api_url"] != "https://livepeer.studio/api":
            raise LivestreamValuesError("livestream: connected_livestream_provider_endpoint_invalid")
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
    if wallet > global_limit or global_limit > 100000:
        raise LivestreamValuesError("livestream: invalid value limits.max_streams_per_wallet")

    periods = _required(value, "periods")
    if not isinstance(periods, dict):
        raise LivestreamValuesError("livestream: invalid value periods")
    if _positive_int(periods, "secrets_refresh_seconds") > 60:
        raise LivestreamValuesError("livestream: invalid value periods.secrets_refresh_seconds")
    timestamp_unit = _string(periods, "webhook_timestamp_unit")
    if timestamp_unit not in {"seconds", "milliseconds"}:
        raise LivestreamValuesError("livestream: invalid value periods.webhook_timestamp_unit")

    web = _required(value, "web")
    if not isinstance(web, dict):
        raise LivestreamValuesError("livestream: invalid value web")
    for name in ("NEXT_PUBLIC_GATEWAY_URL", "NEXT_PUBLIC_LIVESTREAM_API_URL"):
        _https_url(web, name)
    if platform:
        _validate_platform(value)
        return
    if environment.get("LIVEPEER_PROVIDER_MODE") != "available" or environment.get("LIVESTREAM_CONNECTED_ENABLED") != "true":
        raise LivestreamValuesError("livestream: LIVEPEER_PROVIDER_MODE=available requires LIVESTREAM_CONNECTED_ENABLED=true")
    credentials = _required(value, "credentials")
    if not isinstance(credentials, dict):
        raise LivestreamValuesError("livestream: invalid value credentials")
    _string(credentials, "livepeer_api_key")
    if credentials.get("is_test") is not True:
        raise LivestreamValuesError("livestream: invalid value credentials.is_test")
    for name in ("LIVEPEER_WEBHOOK_SECRET", "ACTION_AUDIT_KEY", "LIVESTREAM_VIEWER_TOKEN_KEY", "LIVEPEER_PLAYBACK_WEBHOOK_SECRET", "LIVESTREAM_BUDGET_ALERT_KEY"):
        if len(_string(environment, name).encode()) < 32:
            raise LivestreamValuesError(f"livestream: {name} must be at least 32 bytes")
    if len(_string(environment, "LIVESTREAM_VAULT_TRANSIT_TOKEN")) < 16:
        raise LivestreamValuesError("livestream: LIVESTREAM_VAULT_TRANSIT_TOKEN must be at least 16 bytes")
    for name in ("ACTION_ENVIRONMENT", "LIVESTREAM_VAULT_TRANSIT_KEY", "LIVEPEER_PLAYBACK_WEBHOOK_ID", "IDENTITY_ASSERTION_SIGNING_KEY", "IDENTITY_MTLS_CA_FILE", "IDENTITY_MTLS_CLIENT_CERT_FILE", "IDENTITY_MTLS_CLIENT_KEY_FILE", "IDENTITY_MTLS_PEER_CERT_SHA256"):
        _string(environment, name)
    for name in ("LIVESTREAM_VAULT_TRANSIT_URL", "IDENTITY_SERVICE_URL"):
        _https_url(environment, name, path="")
    for name in ("LIVEPEER_PLAYBACK_WEBHOOK_URL", "LIVESTREAM_BUDGET_ALERT_URL"):
        _https_url(environment, name)
    ingest = urlparse(_string(environment, "LIVESTREAM_INGEST_ENDPOINT"))
    if ingest.scheme not in {"https", "rtmps"} or not ingest.netloc or ingest.username is not None or ingest.query or ingest.fragment:
        raise LivestreamValuesError("livestream: invalid value LIVESTREAM_INGEST_ENDPOINT")
    _budget_policy(value)


def _budget_policy(value: dict[str, Any]) -> dict[str, Any]:
    policy = _required(value, "budget_policy")
    if not isinstance(policy, dict):
        raise LivestreamValuesError("livestream: invalid value budget_policy")
    for name in ("vendorUserId", "currency", "approvalReference"):
        if not re.fullmatch(r"[A-Za-z0-9_-]{1,128}", _string(policy, name)):
            raise LivestreamValuesError(f"livestream: invalid value budget_policy.{name}")
    for name in ("monthlyUnits", "broadcastReserveUnits", "deliveryPricePerTB", "deliveryPricePerMinute", "transcodePricePerMinute", "storagePricePerMinute"):
        number = _string(policy, name)
        if not re.fullmatch(r"(0|[1-9][0-9]{0,77})(\.[0-9]{1,18})?", number) or (name in {"monthlyUnits", "broadcastReserveUnits", "deliveryPricePerTB"} and Decimal(number) <= 0):
            raise LivestreamValuesError(f"livestream: invalid value budget_policy.{name}")
    if _positive_int(policy, "peakDurationSeconds") > 2**64 - 1:
        raise LivestreamValuesError("livestream: invalid value budget_policy.peakDurationSeconds")
    return policy


def private_port(address: str) -> int:
    """Read the IP listener port used by the platform configuration."""
    try:
        host, port = address.rsplit(":", 1)
        ipaddress.ip_address(host.removeprefix("[").removesuffix("]"))
        if ":" in host and not (host.startswith("[") and host.endswith("]")):
            raise ValueError
        number = int(port)
        if not 1 <= number <= 65535:
            raise ValueError
        return number
    except (ValueError, AttributeError) as exc:
        raise LivestreamValuesError("livestream: LIVESTREAM_PRIVATE_LISTEN_ADDRESS must be an IP:port distinct from PORT") from exc


def _validate_platform(value: dict[str, Any]) -> None:
    env = value["environment"]
    if env.get("LIVEPEER_PROVIDER_MODE") != "unavailable" or env.get("LIVESTREAM_CONNECTED_ENABLED") != "false":
        raise LivestreamValuesError("livestream: RUNTIME_PROFILE=platform requires LIVEPEER_PROVIDER_MODE=unavailable and LIVESTREAM_CONNECTED_ENABLED=false")
    if private_port(_string(env, "LIVESTREAM_PRIVATE_LISTEN_ADDRESS")) == value["addresses"]["port"]:
        raise LivestreamValuesError("livestream: LIVESTREAM_PRIVATE_LISTEN_ADDRESS must be distinct from PORT")
    for part in ("CERT", "KEY", "CLIENT_CA"):
        file, pem = f"LIVESTREAM_TLS_{part}_FILE", f"LIVESTREAM_TLS_{part}_PEM"
        if bool(env.get(file)) == bool(env.get(pem)):
            raise LivestreamValuesError(f"livestream: exactly one of {file} and {pem} is required")
    if not re.fullmatch(r"[0-9a-f]{64}", _string(env, "GATEWAY_PEER_CERT_SHA256")):
        raise LivestreamValuesError("livestream: GATEWAY_PEER_CERT_SHA256 must be lowercase hex sha256")
    if len(_string(env, "LIVESTREAM_VIEWER_TOKEN_KEY").encode()) < 32:
        raise LivestreamValuesError("livestream: LIVESTREAM_VIEWER_TOKEN_KEY must be at least 32 bytes")
    try:
        connections = int(_string(env, "DB_MAX_CONNS"))
    except ValueError as exc:
        raise LivestreamValuesError("livestream: DB_MAX_CONNS must be in 1..200") from exc
    if not 1 <= connections <= 200:
        raise LivestreamValuesError("livestream: DB_MAX_CONNS must be in 1..200")
    origins = [origin.strip() for origin in _string(env, "CORS_ALLOWED_ORIGINS").split(",") if origin.strip()]
    if not origins:
        raise LivestreamValuesError("livestream: CORS_ALLOWED_ORIGINS must list the web origin(s)")
    for origin in origins:
        _https_url({"origin": origin}, "origin")
        if urlparse(origin).path not in {"", "/"}:
            raise LivestreamValuesError("livestream: CORS_ALLOWED_ORIGINS entries must be https origins")
    if value["web"]["NEXT_PUBLIC_LIVESTREAM_API_URL"] != value["web"]["NEXT_PUBLIC_GATEWAY_URL"]:
        raise LivestreamValuesError("livestream: NEXT_PUBLIC_LIVESTREAM_API_URL must equal NEXT_PUBLIC_GATEWAY_URL for RUNTIME_PROFILE=platform")


def documents(service: str, values: str | Path) -> dict[str, Any]:
    """Return the connected budget policy when broadcasting is provisioned."""

    if service != "livestream":
        raise LivestreamValuesError(f"{service}: unsupported document service")
    value = _read_values(Path(values))
    _validate(value)
    if value["environment"]["RUNTIME_PROFILE"] == "platform":
        return {}
    return {"LIVESTREAM_BUDGET_POLICY_DOCUMENT": _budget_policy(value)}


def environment(service: str, values: str | Path) -> dict[str, str]:
    """Write the runtime settings supplied by the deployment values."""

    documents(service, values)
    value = _read_values(Path(values))
    result = {
        "BIND_ADDRESS": value["addresses"]["bind"],
        "PORT": str(value["addresses"]["port"]),
        "RPC_URL": value["network"]["rpc_url"],
        "ZONE": value["names"]["zone"],
        "MAX_STREAMS_PER_WALLET": str(value["limits"]["max_streams_per_wallet"]),
        "MAX_GLOBAL_STREAMS": str(value["limits"]["max_global_streams"]),
        "SECRETS_REFRESH_SECONDS": str(value["periods"]["secrets_refresh_seconds"]),
        "LIVEPEER_WEBHOOK_TIMESTAMP_UNIT": value["periods"]["webhook_timestamp_unit"],
    }
    result.update(value["environment"])
    if value["environment"]["RUNTIME_PROFILE"] != "platform":
        result["LIVEPEER_API_URL"] = value["endpoints"]["livepeer_api_url"]
        result["LIVEPEER_API_KEY"] = value["credentials"]["livepeer_api_key"]
    else:
        result.pop("LIVEPEER_WEBHOOK_TIMESTAMP_UNIT")
    return result


def selection(values: str | Path) -> dict[str, Any]:
    """Return validated values for the selected livestream deployment form."""
    value = _read_values(Path(values))
    _validate(value)
    return value


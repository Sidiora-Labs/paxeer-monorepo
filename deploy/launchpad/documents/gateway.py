"""Compose the launchpad media gateway runtime and identity documents."""

from __future__ import annotations

import base64
import json
import re
from pathlib import Path
from typing import Any
from urllib.parse import urlparse

from common import DocumentError


_HEX64 = re.compile(r"^[0-9a-fA-F]{64}$")


def _values_file(values: Path) -> Path:
    for name in ("gateway.json", "gateway.example.json"):
        path = values / name
        if path.is_file():
            return path
    raise DocumentError("gateway: missing values file gateway.json")


def _read_values(values: Path) -> dict[str, Any]:
    path = _values_file(values)
    try:
        value = json.loads(path.read_bytes())
    except (OSError, json.JSONDecodeError) as exc:
        raise DocumentError(f"gateway: invalid values file {path.name}") from exc
    if not isinstance(value, dict):
        raise DocumentError("gateway: values must be an object")
    for key in ("runtime", "identity", "credentials"):
        if not isinstance(value.get(key), dict):
            raise DocumentError(f"gateway: {key} must be an object")
    return value


def _field(obj: dict[str, Any], key: str) -> Any:
    if key not in obj:
        raise DocumentError(f"gateway: missing value {key}")
    return obj[key]


def _string(obj: dict[str, Any], key: str) -> str:
    value = _field(obj, key)
    if not isinstance(value, str) or not value.strip():
        raise DocumentError(f"gateway: invalid value {key}")
    return value


def _integer(obj: dict[str, Any], key: str, *, minimum: int = 1, maximum: int | None = None) -> int:
    value = _field(obj, key)
    if isinstance(value, bool) or not isinstance(value, int) or value < minimum or (maximum is not None and value > maximum):
        raise DocumentError(f"gateway: invalid value {key}")
    return value


def _url(obj: dict[str, Any], key: str, schemes: set[str], *, allow_path: bool = False) -> str:
    value = _string(obj, key)
    parsed = urlparse(value)
    if parsed.scheme not in schemes or not parsed.netloc or parsed.username or (not allow_path and parsed.path not in ("", "/")) or parsed.query or parsed.fragment:
        raise DocumentError(f"gateway: invalid value {key}")
    return value


def _digest(obj: dict[str, Any], key: str) -> str:
    value = _string(obj, key)
    if not _HEX64.fullmatch(value):
        raise DocumentError(f"gateway: invalid value {key}")
    return value.lower()


def _credential(credentials: dict[str, Any], key: str) -> str:
    entry = _field(credentials, key)
    if isinstance(entry, str):
        reference = entry
    elif isinstance(entry, dict):
        reference = _string(entry, "reference")
        marker = _field(entry, "test")
        if not isinstance(marker, bool):
            raise DocumentError(f"gateway: invalid value credentials.{key}.test")
    else:
        raise DocumentError(f"gateway: invalid value credentials.{key}")
    if not re.fullmatch(r"[A-Z][A-Z0-9_]{0,127}", reference):
        raise DocumentError(f"gateway: invalid value credentials.{key}.reference")
    return reference


def _route(route: Any, key: str) -> dict[str, Any]:
    if not isinstance(route, dict):
        raise DocumentError(f"gateway: invalid value {key}")
    method = _string(route, "method")
    template = _string(route, "template")
    if not method.isupper() or not template.startswith("/"):
        raise DocumentError(f"gateway: invalid value {key}")
    result: dict[str, Any] = {"Method": method, "Template": template}
    for flag in ("authenticate", "optional", "upload"):
        value = route.get(flag, False)
        if not isinstance(value, bool):
            raise DocumentError(f"gateway: invalid value {key}.{flag}")
        result[flag.capitalize() if flag != "upload" else "Upload"] = value
    # ConnectedRoute is encoded with Go's field names because it has no json tags.
    result["Authenticate"] = result.pop("Authenticate")
    result["Optional"] = result.pop("Optional")
    result["Upload"] = result.pop("Upload")
    return result


def _runtime(values: dict[str, Any]) -> dict[str, Any]:
    runtime = values["runtime"]
    credentials = values["credentials"]
    pools = _field(runtime, "pools")
    if not isinstance(pools, dict):
        raise DocumentError("gateway: invalid value pools")
    pool_doc = {
        "identityPoolId": _string(pools, "identityPoolId"),
        "identityMaxConnections": _integer(pools, "identityMaxConnections"),
        "controlPoolId": _string(pools, "controlPoolId"),
        "controlMaxConnections": _integer(pools, "controlMaxConnections"),
        "controlCredentialReference": _credential(credentials, _string(pools, "controlCredential")),
    }
    if pool_doc["identityPoolId"] == pool_doc["controlPoolId"]:
        raise DocumentError("gateway: invalid value pools.controlPoolId")

    control = _field(runtime, "control")
    if not isinstance(control, dict):
        raise DocumentError("gateway: invalid value control")
    control_doc = {
        "schema": _string(control, "schema"),
        "authoritySchema": _string(control, "authoritySchema"),
        "incidentCredentialReference": _credential(credentials, _string(control, "incidentCredential")),
        "revocationEndpoint": _url(control, "revocationEndpoint", {"https"}, allow_path=True),
        "revocationTokenReference": _credential(credentials, _string(control, "revocationCredential")),
    }

    workers = _field(runtime, "objectWorkers")
    if not isinstance(workers, list) or not 1 <= len(workers) <= 4:
        raise DocumentError("gateway: invalid value objectWorkers")
    worker_docs = []
    kinds: set[str] = set()
    prefixes = {"variant": "approved/", "original": "quarantine/", "temporary": "temporary/", "recording": "private/"}
    for index, worker in enumerate(workers):
        key = f"objectWorkers[{index}]"
        if not isinstance(worker, dict):
            raise DocumentError(f"gateway: invalid value {key}")
        kind = _string(worker, "kind")
        if kind not in prefixes or kind in kinds or _string(worker, "prefix") != prefixes[kind]:
            raise DocumentError(f"gateway: invalid value {key}.kind")
        kinds.add(kind)
        worker_docs.append(
            {
                "kind": kind,
                "bucket": _string(worker, "bucket"),
                "prefix": _string(worker, "prefix"),
                "deleteAccessKeyReference": _credential(credentials, _string(worker, "deleteAccessKeyCredential")),
                "deleteSecretKeyReference": _credential(credentials, _string(worker, "deleteSecretKeyCredential")),
                "inventoryAccessKeyReference": _credential(credentials, _string(worker, "inventoryAccessKeyCredential")),
                "inventorySecretKeyReference": _credential(credentials, _string(worker, "inventorySecretKeyCredential")),
                "inventoryLimit": _integer(worker, "inventoryLimit", maximum=100000),
            }
        )
    if not {"variant", "original", "temporary"}.issubset(kinds):
        raise DocumentError("gateway: objectWorkers must include variant, original and temporary")

    trusted = _field(runtime, "trustedProxies")
    if not isinstance(trusted, list) or len(trusted) > 16 or any(not isinstance(item, str) or "/" not in item for item in trusted):
        raise DocumentError("gateway: invalid value trustedProxies")
    for item in trusted:
        address, bits = item.rsplit("/", 1)
        try:
            import ipaddress

            network = ipaddress.ip_network(item, strict=True)
            if network.prefixlen == 0 or str(network) != item:
                raise ValueError
        except ValueError:
            raise DocumentError("gateway: invalid value trustedProxies") from None

    identity_peers = _field(runtime, "identityPeers")
    if not isinstance(identity_peers, list) or not 1 <= len(identity_peers) <= 16:
        raise DocumentError("gateway: invalid value identityPeers")
    peer_docs = []
    seen_issuers: set[str] = set()
    for index, peer in enumerate(identity_peers):
        key = f"identityPeers[{index}]"
        if not isinstance(peer, dict):
            raise DocumentError(f"gateway: invalid value {key}")
        issuer = _string(peer, "issuer")
        if issuer == "gateway" or issuer in seen_issuers:
            raise DocumentError(f"gateway: invalid value {key}.issuer")
        seen_issuers.add(issuer)
        pin = _digest(peer, "peerCertSha256")
        public_key = _string(peer, "publicKey")
        try:
            decoded = base64.urlsafe_b64decode(public_key + "=" * (-len(public_key) % 4))
        except ValueError:
            decoded = b""
        if len(decoded) != 32 or "+" in public_key or "/" in public_key or "=" in public_key:
            raise DocumentError(f"gateway: invalid value {key}.publicKey")
        peer_docs.append({"issuer": issuer, "peerCertSHA256": pin, "publicKey": public_key})

    upstreams = _field(runtime, "upstreams")
    if not isinstance(upstreams, list) or len(upstreams) != 4:
        raise DocumentError("gateway: invalid value upstreams")
    upstream_docs = []
    seen_upstreams: set[str] = set()
    for index, upstream in enumerate(upstreams):
        key = f"upstreams[{index}]"
        if not isinstance(upstream, dict):
            raise DocumentError(f"gateway: invalid value {key}")
        name = _string(upstream, "name")
        if name not in {"user", "metadata", "social", "livestream"} or name in seen_upstreams:
            raise DocumentError(f"gateway: invalid value {key}.name")
        seen_upstreams.add(name)
        routes = _field(upstream, "routes")
        if not isinstance(routes, list) or not routes:
            raise DocumentError(f"gateway: invalid value {key}.routes")
        upstream_docs.append({"name": name, "url": _url(upstream, "url", {"https"}), "routes": [_route(route, f"{key}.routes[{i}]") for i, route in enumerate(routes)]})

    audit = _field(runtime, "audit")
    if not isinstance(audit, dict):
        raise DocumentError("gateway: invalid value audit")
    audit_doc = {
        "keyReference": _credential(credentials, _string(audit, "keyCredential")),
        "deliverySchema": _string(audit, "deliverySchema"),
        "sinkDeliverySchema": _string(audit, "sinkDeliverySchema"),
        "sinkSchema": _string(audit, "sinkSchema"),
        "subject": _string(audit, "subject"),
        "eventType": _string(audit, "eventType"),
        "stream": _string(audit, "stream"),
        "durable": _string(audit, "durable"),
    }

    purge_mode = _string(runtime, "purgeMode")
    if purge_mode not in {"http", "origin_cache"}:
        raise DocumentError("gateway: invalid value purgeMode")
    purge_endpoint = _url(runtime, "purgeEndpoint", {"https"}) if purge_mode == "http" else ""
    purge_reference = _credential(credentials, _string(runtime, "purgeCredential")) if purge_mode == "http" else ""

    return {
        "pools": pool_doc,
        "control": control_doc,
        "objectWorkers": worker_docs,
        "trustedProxies": trusted,
        "identityEndpoint": _url(runtime, "identityEndpoint", {"https"}),
        "identityServerPeerCertSHA256": _digest(runtime, "identityServerPeerCertSha256"),
        "identityPeers": peer_docs,
        "identityListenAddress": _string(runtime, "identityListenAddress"),
        "caFile": _string(runtime, "caFile"),
        "caSHA256": _digest(runtime, "caSha256"),
        "clientCertFile": _string(runtime, "clientCertFile"),
        "clientCertSHA256": _digest(runtime, "clientCertSha256"),
        "clientKeyReference": _credential(credentials, _string(runtime, "clientKeyCredential")),
        "serverCertFile": _string(runtime, "serverCertFile"),
        "serverCertSHA256": _digest(runtime, "serverCertSha256"),
        "serverKeyReference": _credential(credentials, _string(runtime, "serverKeyCredential")),
        "assertionKeyReference": _credential(credentials, _string(runtime, "assertionKeyCredential")),
        "natsURL": _url(runtime, "natsUrl", {"tls"}),
        "natsCredentialsReference": _credential(credentials, _string(runtime, "natsCredential")),
        "stream": _string(runtime, "stream"),
        "durable": _string(runtime, "durable"),
        "consumerGeneration": _string(runtime, "consumerGeneration"),
        "recoverySchema": _string(runtime, "recoverySchema"),
        "recoveryEpoch": _integer(runtime, "recoveryEpoch"),
        "recoveryRegion": _string(runtime, "recoveryRegion"),
        "mediaSchema": _string(runtime, "mediaSchema"),
        "metadataSchema": _string(runtime, "metadataSchema"),
        "safetySchema": _string(runtime, "safetySchema"),
        "approvedBucket": _string(runtime, "approvedBucket"),
        "uploadNamespace": _string(runtime, "uploadNamespace"),
        "uploadConcurrent": _integer(runtime, "uploadConcurrent"),
        "uploadBytesPerSecond": _integer(runtime, "uploadBytesPerSecond"),
        "purgeMode": purge_mode,
        "purgeEndpoint": purge_endpoint,
        "purgeTokenReference": purge_reference,
        "audit": audit_doc,
        "upstreams": upstream_docs,
    }


def _identity(values: dict[str, Any]) -> dict[str, Any]:
    identity = values["identity"]
    credentials = values["credentials"]
    origins = _field(identity, "allowedOrigins")
    if not isinstance(origins, list) or not origins:
        raise DocumentError("gateway: invalid value allowedOrigins")
    for index, origin in enumerate(origins):
        if not isinstance(origin, str):
            raise DocumentError(f"gateway: invalid value allowedOrigins[{index}]")
        parsed = urlparse(origin)
        if parsed.scheme != "https" or not parsed.netloc or parsed.username or parsed.path not in ("", "/") or parsed.query or parsed.fragment or "*" in origin:
            raise DocumentError(f"gateway: invalid value allowedOrigins[{index}]")
    keys = _field(identity, "keys")
    if not isinstance(keys, list) or not 1 <= len(keys) <= 8:
        raise DocumentError("gateway: invalid value keys")
    key_docs = []
    ids: set[str] = set()
    for index, key in enumerate(keys):
        name = f"keys[{index}]"
        if not isinstance(key, dict):
            raise DocumentError(f"gateway: invalid value {name}")
        key_id = _string(key, "id")
        if key_id in ids or len(key_id) > 64:
            raise DocumentError(f"gateway: invalid value {name}.id")
        ids.add(key_id)
        active = _string(key, "activeAt")
        retire = _string(key, "retireAt")
        if retire <= active:
            raise DocumentError(f"gateway: invalid value {name}.retireAt")
        key_docs.append({"id": key_id, "secretReference": _credential(credentials, _string(key, "credential")), "activeAt": active, "retireAt": retire})
    active_key = _string(identity, "activeKey")
    if active_key not in ids:
        raise DocumentError("gateway: invalid value activeKey")
    return {
        "issuer": _string(identity, "issuer"),
        "audience": _string(identity, "audience"),
        "activeKey": active_key,
        "keys": key_docs,
        "domain": _string(identity, "domain"),
        "uri": _url(identity, "uri", {"https"}, allow_path=True),
        "allowedOrigins": origins,
    }


def _json_bytes(value: Any) -> bytes:
    try:
        return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode("utf-8")
    except (TypeError, ValueError) as exc:
        raise DocumentError("gateway: values document is not JSON serializable") from exc


def documents(service: str, values: str | Path) -> dict[str, bytes]:
    """Return the digest-bound gateway runtime and identity documents."""
    if service != "gateway":
        raise DocumentError(f"gateway: unexpected service {service}")
    source = _read_values(Path(values))
    return {
        "GATEWAY_RUNTIME_DOCUMENT": _json_bytes(_runtime(source)),
        "IDENTITY_CONFIG_DOCUMENT": _json_bytes(_identity(source)),
    }

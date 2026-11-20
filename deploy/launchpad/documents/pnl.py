"""Compose the PnL inventory runtime document for chain 125."""

from __future__ import annotations

import json
import posixpath
import re
from pathlib import Path
from typing import Any
from urllib.parse import urlsplit

from common import DocumentError


IDENTIFIER = re.compile(r"[A-Za-z0-9_-]{1,128}")
REGION = re.compile(r"[a-z][a-z0-9-]{1,62}")
DIGEST = re.compile(r"[0-9a-f]{64}")
CHAIN_HASH = re.compile(r"0x[0-9a-f]{64}")
ADDRESS = re.compile(r"0x[0-9a-f]{40}")
PROGRAM_IDENTIFIER = re.compile(r"[A-Za-z0-9_.:-]{1,128}")
AMOUNT = re.compile(r"0|[1-9][0-9]{0,77}")
FONT_REGULAR_SHA256 = "40d692fce188e4471e2b3cba937be967878f631ad3ebbbdcd587687c7ebe0c82"
FONT_BOLD_SHA256 = "288316099b1e0a47a4716d159098005eef7c0066921f34e3200393dbdb01947f"
FONT_LICENSE_SHA256 = "262481e844521b326f5ecd053e59b98c8b2da78c8ee1bdbb6e8174305e54935a"
REWARD_MANIFEST_DIGEST = "6ec466f0cade69ad3da3ea55485afe6d13ec47722bba46864df156ea0900ee6b"
REWARD_NFT = "0xe8dcc844a8006dbf64f5bb851bcad90292a6409b"
REWARD_ACCUMULATOR = "0x7b14808b72f61664cdb79d25e6d1587655571a66"
REWARD_FEES_ROUTER = "0x05681da6f611c2dffd3e92611f6de2d247647927"
REWARD_USDL = "0x85fcd13735f4309833a503ee804ea32395851479"
JOURNEY_KEYS = {
    "realm", "identityEndpoint", "identityPeerCertSha256",
    "identityCaFile", "identityCaSha256", "identityClientCertFile",
    "identityClientCertSha256", "identityClientKeyFile", "identityClientKeySha256",
    "fontDirectory", "liveBatch", "releaseDigest", "interfaceDigest", "nodeIds",
    "exportsDirectory", "brokerStream", "cdnOrigin", "cdnZone",
    "rewardRuntimeHashes", "programs", "approvedCommunityWallets",
}
PROGRAM_KEYS = {
    "ID", "Version", "DeploymentID", "Asset", "ReferralRaw", "CashbackStartHeight",
    "CashbackEndHeight", "CashbackBPS", "AwardCapRaw", "Treasury", "Custody",
    "Adapter", "AdapterCodeHash", "DailyCapRaw", "BeneficiaryCapRaw",
}


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


def _read_values(service: str, values: str | Path) -> dict[str, Any]:
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
    required = {
        "sourceStream", "consumerId", "generation", "owner",
        "projectionVersion", "epoch", "region", "consumer",
    }
    if isinstance(source, dict) and "journeys" in source:
        required = required | {"journeys"}
    return _object(source, required, "pnl")


def _path(value: Any, key: str) -> str:
    if (not isinstance(value, str) or not value.startswith("/") or value == "/"
            or "\x00" in value or "//" in value or posixpath.normpath(value) != value):
        raise DocumentError(f"pnl: invalid value {key}")
    try:
        value.encode("utf-8")
    except UnicodeEncodeError as exc:
        raise DocumentError(f"pnl: invalid value {key}") from exc
    return value


def _origin(value: Any, key: str) -> str:
    if not isinstance(value, str) or not value.startswith("https://") or any(
        char.isspace() or ord(char) < 32 or ord(char) == 127 for char in value
    ):
        raise DocumentError(f"pnl: invalid value {key}")
    try:
        value.encode("utf-8")
        parsed = urlsplit(value)
        valid = (bool(parsed.hostname) and parsed.username is None and parsed.password is None
                 and not parsed.path and "?" not in value and "#" not in value
                 and "\\" not in parsed.netloc)
        port = parsed.port
        valid = valid and (port is None or 0 < port <= 65535)
    except (ValueError, UnicodeEncodeError):
        valid = False
    if not valid:
        raise DocumentError(f"pnl: invalid value {key}")
    return value


def _wallet(value: Any, key: str, *, canonical: bool = False) -> str:
    if (not isinstance(value, str) or ADDRESS.fullmatch(value.lower()) is None
            or value.lower() == "0x" + "0" * 40 or (canonical and value != value.lower())):
        raise DocumentError(f"pnl: invalid value {key}")
    return value


def _program(value: Any, key: str) -> dict[str, Any]:
    program = dict(_object(value, PROGRAM_KEYS, key))
    for name in ("ID", "Version", "DeploymentID"):
        _name(program[name], f"{key}.{name}", PROGRAM_IDENTIFIER)
    if program["Asset"] != REWARD_USDL:
        raise DocumentError(f"pnl: invalid value {key}.Asset")
    _integer(program["CashbackBPS"], f"{key}.CashbackBPS", 0, 10000)
    for name in ("ReferralRaw", "AwardCapRaw", "DailyCapRaw", "BeneficiaryCapRaw"):
        raw = _name(program[name], f"{key}.{name}", AMOUNT)
        if int(raw) >= 1 << 256:
            raise DocumentError(f"pnl: invalid value {key}.{name}")
    start = program["CashbackStartHeight"]
    heights = {}
    for name in ("CashbackStartHeight", "CashbackEndHeight"):
        height = program[name]
        if not isinstance(height, str):
            raise DocumentError(f"pnl: invalid value {key}.{name}")
        if height != "":
            _name(height, f"{key}.{name}", re.compile(r"[0-9]+"))
            significant = height.lstrip("0")
            if len(significant) > 20 or not 1 <= int(significant or "0") <= (1 << 64) - 1:
                raise DocumentError(f"pnl: invalid value {key}.{name}")
            heights[name] = int(significant)
    end = program["CashbackEndHeight"]
    if end != "" and (start == "" or heights["CashbackEndHeight"] < heights["CashbackStartHeight"]):
        raise DocumentError(f"pnl: invalid value {key}.CashbackEndHeight")
    for name in ("Treasury", "Custody"):
        if program[name] != "":
            _wallet(program[name], f"{key}.{name}", canonical=True)
    for name, pattern in (("Adapter", PROGRAM_IDENTIFIER), ("AdapterCodeHash", CHAIN_HASH)):
        if program[name] != "":
            _name(program[name], f"{key}.{name}", pattern)
    return program


def _journeys(value: Any) -> dict[str, Any]:
    key_members = {"identityClientKeyFile", "identityClientKeySha256"}
    keys = JOURNEY_KEYS
    if isinstance(value, dict) and not key_members.intersection(value):
        keys = JOURNEY_KEYS - key_members
    document = dict(_object(value, keys, "journeys"))
    file_key = bool(key_members.intersection(document))
    if file_key:
        _path(document["identityClientKeyFile"], "journeys.identityClientKeyFile")
        _name(document["identityClientKeySha256"], "journeys.identityClientKeySha256", DIGEST)
    else:
        document.update(identityClientKeyFile="", identityClientKeySha256="")
    if document["realm"] not in ("production", "private_fixture"):
        raise DocumentError("pnl: invalid value journeys.realm")
    for name in ("identityEndpoint", "cdnOrigin"):
        _origin(document[name], f"journeys.{name}")
    for name in ("identityPeerCertSha256", "identityCaSha256", "identityClientCertSha256",
                 "releaseDigest", "interfaceDigest"):
        _name(document[name], f"journeys.{name}", DIGEST)
    for name in ("identityCaFile", "identityClientCertFile",
                 "fontDirectory", "exportsDirectory"):
        _path(document[name], f"journeys.{name}")
    _integer(document["liveBatch"], "journeys.liveBatch", 1, 32)
    nodes = document["nodeIds"]
    if not isinstance(nodes, list) or len(nodes) != 2:
        raise DocumentError("pnl: invalid value journeys.nodeIds")
    for index, node in enumerate(nodes):
        try:
            valid = isinstance(node, str) and 1 <= len(node.encode("utf-8")) <= 256
        except UnicodeEncodeError:
            valid = False
        if not valid:
            raise DocumentError(f"pnl: invalid value journeys.nodeIds[{index}]")
    if nodes[0] == nodes[1]:
        raise DocumentError("pnl: invalid value journeys.nodeIds")
    _name(document["brokerStream"], "journeys.brokerStream")
    if document["cdnZone"] != "":
        _name(document["cdnZone"], "journeys.cdnZone", re.compile(r"[0-9a-f]{32}"))
    hashes = document["rewardRuntimeHashes"]
    if not isinstance(hashes, dict):
        raise DocumentError("pnl: invalid value journeys.rewardRuntimeHashes")
    for address, digest in hashes.items():
        _wallet(address, "journeys.rewardRuntimeHashes", canonical=True)
        _name(digest, "journeys.rewardRuntimeHashes", CHAIN_HASH)
    for address in (REWARD_NFT, REWARD_ACCUMULATOR, REWARD_FEES_ROUTER, REWARD_USDL):
        if address not in hashes:
            raise DocumentError("pnl: missing value journeys.rewardRuntimeHashes")
    programs = document["programs"]
    if not isinstance(programs, list):
        raise DocumentError("pnl: invalid value journeys.programs")
    seen = set()
    for index, program in enumerate(programs):
        program = _program(program, f"journeys.programs[{index}]")
        identity = (program["ID"], program["Version"])
        if identity in seen:
            raise DocumentError(f"pnl: invalid value journeys.programs[{index}]")
        seen.add(identity)
    wallets = document["approvedCommunityWallets"]
    if not isinstance(wallets, list) or len(wallets) > 64:
        raise DocumentError("pnl: invalid value journeys.approvedCommunityWallets")
    approved = set()
    for index, wallet in enumerate(wallets):
        _wallet(wallet, f"journeys.approvedCommunityWallets[{index}]")
        if wallet.lower() in approved:
            raise DocumentError(f"pnl: invalid value journeys.approvedCommunityWallets[{index}]")
        approved.add(wallet.lower())
    return document | {
        "version": 3, "identityIssuer": "pnld", "identityAudience": "identity",
        "fontRegularSha256": FONT_REGULAR_SHA256, "fontBoldSha256": FONT_BOLD_SHA256,
        "fontLicenseSha256": FONT_LICENSE_SHA256, "rewardManifestDigest": REWARD_MANIFEST_DIGEST,
    }


def environment(service: str, values: str | Path) -> dict[str, str]:
    source = _read_values(service, values)
    if "journeys" not in source:
        return {}
    document = _journeys(source["journeys"])
    return {"IDENTITY_SERVICE_URL": document["identityEndpoint"], "PNL_FONT_DIR": document["fontDirectory"]}


def documents(service: str, values: str | Path) -> dict[str, Any]:
    source = _read_values(service, values)
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
    result = {"DELIVERY_CONFIG_DOCUMENT": document}
    if "journeys" in source:
        result["PNL_JOURNEY_CONFIG_DOCUMENT"] = _journeys(source["journeys"])
    return result

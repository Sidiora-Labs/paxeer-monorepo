#!/usr/bin/env python3

import argparse
import base64
import binascii
import json
import os
from pathlib import Path
import re
import sys
import tempfile

MAX_HEIGHT = (1 << 63) - 1
MAX_UINT64 = (1 << 64) - 1
RELEASE_URL = "https://github.com/Sidiora-Labs/paxeer-monorepo/releases/download"
REGISTERED_PLAN_NAMES = {
    "MergeUpgrade": "v7.0",
    "AnchorStoreDeleteUpgrade": "v7.0-anchor-store-delete",
}


class ArgumentParser(argparse.ArgumentParser):
    def error(self, message):
        raise ValueError(message)


def release_identity():
    identity = json.loads(Path(__file__).with_name("version.json").read_text())
    if not isinstance(identity, dict):
        raise ValueError("invalid release identity")
    for key in ("version", "upgrade"):
        value = identity.get(key)
        if not isinstance(value, str) or not (
            re.fullmatch(r"v[0-9]+\.[0-9]+\.[0-9]+", value)
            or (key == "upgrade" and value in REGISTERED_PLAN_NAMES.values())
        ):
            raise ValueError(f"invalid release {key}")
    return identity


def validate_name(name, identity):
    if name != identity["upgrade"]:
        raise ValueError(
            f"name {name} is not the release upgrade {identity['upgrade']}"
        )
    return name


def validate_checksum(checksum):
    if not isinstance(checksum, str) or not re.fullmatch(r"[0-9a-f]{64}", checksum):
        raise ValueError("checksum must be 64 lowercase hex digits")
    return checksum


def asset_url(identity):
    version = identity["version"]
    return f"{RELEASE_URL}/paxeer-network/{version}/paxd-{version}-linux-amd64"


def write_json(path, document, mode):
    encoded = json.dumps(document, indent=2) + "\n"
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = None
    try:
        with tempfile.NamedTemporaryFile(
            mode="w",
            encoding="utf-8",
            dir=path.parent,
            prefix=f".{path.name}.",
            delete=False,
        ) as output:
            temporary = Path(output.name)
            os.chmod(temporary, mode)
            output.write(encoded)
            output.flush()
            os.fsync(output.fileno())
        os.replace(temporary, path)
    finally:
        if temporary is not None:
            temporary.unlink(missing_ok=True)


def uint64(value, label):
    if type(value) is not int or not 0 <= value <= MAX_UINT64:
        raise ValueError(f"{label} must be a uint64 integer")
    return value


def decode_bytes(value, label):
    if not isinstance(value, str):
        raise ValueError(f"{label} must be a base64 string")
    try:
        decoded = base64.b64decode(value, validate=True)
    except (ValueError, binascii.Error) as error:
        raise ValueError(f"{label} must be a base64 string") from error
    if len(decoded) != 32:
        raise ValueError(f"{label} must be 32 bytes")
    return decoded


def byte_argument(value):
    if re.fullmatch(r"(?:0x)?[0-9a-fA-F]{64}", value):
        value = base64.b64encode(bytes.fromhex(value.removeprefix("0x"))).decode()
    decode_bytes(value, "key or hash")
    return value


def validate_plan_info(info, plan_height=None):
    if not isinstance(info, dict):
        raise ValueError("plan info must be an object")
    leaders = info.get("leaders")
    if not isinstance(leaders, list) or not leaders:
        raise ValueError("leader list must be non-empty")
    seen = set()
    for leader in leaders:
        key = decode_bytes(leader, "leader")
        if not any(key) or key in seen:
            raise ValueError("leaders must be non-zero and distinct")
        seen.add(key)
    keys = info.get("receipt_keys")
    if not isinstance(keys, list) or not keys:
        raise ValueError("receipt key list must be non-empty")
    seen = set()
    for entry in keys:
        if not isinstance(entry, dict):
            raise ValueError("receipt key entry must be an object")
        key_id = decode_bytes(entry.get("key_id"), "key id")
        if not any(key_id) or key_id in seen:
            raise ValueError("receipt key ids must be non-zero and distinct")
        seen.add(key_id)
        public_key = decode_bytes(entry.get("public_key"), "receipt public key")
        # Match layerxproof/verify.PublicKeyIsCanonical, used by ReceiptKeyEntry.Validate.
        y = int.from_bytes(public_key, "little") & ((1 << 255) - 1)
        if y in (0, 1) or y >= (1 << 255) - 19:
            raise ValueError("receipt public key is not a canonical ed25519 key")
        first = uint64(entry.get("first_height"), "first height")
        last = uint64(entry.get("last_height"), "last height")
        if first == 0 or first > last:
            raise ValueError("receipt key range must be positive and ordered")
    last_seq = uint64(info.get("last_seq", 0), "last sequence")
    last_hash = (
        decode_bytes(info["last_seq_hash"], "last sequence hash")
        if "last_seq_hash" in info else bytes(32)
    )
    if last_seq != 0 or any(last_hash):
        raise ValueError("merge plan requires last sequence 0 and the zero hash")
    activation = uint64(info.get("activation_height", 0), "activation height")
    if activation > MAX_HEIGHT:
        raise ValueError("activation height exceeds int64")
    recorded_height = info.get("plan_height")
    if recorded_height is not None:
        validate_height(recorded_height, "plan height")
        if plan_height is not None and plan_height != recorded_height:
            raise ValueError("plan height differs from recorded plan height")
        plan_height = recorded_height
    heights = [activation] if activation else []
    if plan_height is not None:
        first_height = validate_height(plan_height, "plan height") + 1
        heights.append(validate_height(first_height, "first sequenced height"))
    for height in heights:
        if not any(
            key["first_height"] <= height <= key["last_height"] for key in keys
        ):
            raise ValueError(f"no receipt key covers sequenced height {height}")
    if "binaries" in info and (
        not isinstance(info["binaries"], dict)
        or not info["binaries"]
        or any(
            not isinstance(k, str) or not isinstance(v, str) or not v
            for k, v in info["binaries"].items()
        )
    ):
        raise ValueError("binaries must be a non-empty map of URLs")
    return info


def read_plan_info(path):
    document = json.loads(path.read_text(encoding="utf-8"))
    if isinstance(document, dict) and "info" in document:
        if not isinstance(document["info"], str):
            raise ValueError("document info must be a JSON string")
        return json.loads(document["info"])
    return document


def merge_plan_info(info, path, plan_height=None):
    plan = validate_plan_info(read_plan_info(path), plan_height)
    for field in info.keys() & plan.keys():
        if info[field] != plan[field]:
            raise ValueError(f"plan info {field} differs from release document")
    return {**plan, **info}


def plan_info(args, identity):
    if args.check is not None:
        if (
            args.leader or args.receipt_key or args.release_doc or args.out
            or args.last_seq is not None or args.last_seq_hash is not None
            or args.activation_height is not None
        ):
            raise ValueError("--check accepts only --plan-height with its input file")
        validate_plan_info(read_plan_info(args.check), args.plan_height)
        return
    if not args.leader or not args.receipt_key:
        raise ValueError("--leader and --receipt-key are required")
    info = {
        "leaders": [byte_argument(key) for key in args.leader],
        "receipt_keys": [json.loads(entry) for entry in args.receipt_key],
        "last_seq": 0 if args.last_seq is None else args.last_seq,
        "last_seq_hash": (
            byte_argument(args.last_seq_hash) if args.last_seq_hash is not None
            else base64.b64encode(bytes(32)).decode()
        ),
        "activation_height": 0 if args.activation_height is None else args.activation_height,
    }
    if args.plan_height is not None:
        info["plan_height"] = args.plan_height
    if args.release_doc is not None:
        document = json.loads(args.release_doc.read_text(encoding="utf-8"))
        if not isinstance(document, dict) or not isinstance(document.get("info"), str):
            raise ValueError("release document must contain name and info")
        validate_name(document.get("name"), identity)
        release_info = json.loads(document["info"])
        if not isinstance(release_info, dict) or not isinstance(
            release_info.get("binaries"), dict
        ):
            raise ValueError("release document must contain binaries")
        info["binaries"] = release_info["binaries"]
    validate_plan_info(info, args.plan_height)
    if args.out is None:
        print(json.dumps(info, indent=2))
    else:
        write_json(args.out, info, 0o644)


def release(args, identity):
    name = validate_name(
        identity["upgrade"] if args.name is None else args.name, identity
    )
    expected_tag = f"paxeer-network/{identity['version']}"
    if args.tag is not None and args.tag != expected_tag:
        raise ValueError(f"tag {args.tag} is not the release tag {expected_tag}")
    asset = f"paxd-{identity['version']}-linux-amd64"
    checksums = []
    for line in args.sums.read_text(encoding="utf-8").splitlines():
        match = re.fullmatch(r"(\S+) [ *](.+)", line)
        if match and match[2] == asset:
            checksums.append(validate_checksum(match[1]))
    if not checksums:
        raise ValueError(f"no {asset} line in SHA256SUMS")
    if len(checksums) != 1:
        raise ValueError(f"multiple {asset} lines in SHA256SUMS")
    info = {
        "binaries": {
            "linux/amd64": f"{asset_url(identity)}?checksum=sha256:{checksums[0]}"
        }
    }
    if getattr(args, "plan_info", None) is not None:
        info = merge_plan_info(info, args.plan_info)
    document = {"name": name, "info": json.dumps(info, separators=(",", ":"))}
    if args.out is None:
        print(json.dumps(document, indent=2))
    else:
        write_json(args.out, document, 0o644)


def release_document_name(path, checksum, identity):
    validate_checksum(checksum)
    document = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(document, dict) or set(document) != {"name", "info"}:
        raise ValueError("release document must contain name and info")
    name = validate_name(document["name"], identity)
    if not isinstance(document["info"], str):
        raise ValueError("release document info must be a JSON string")
    info = json.loads(document["info"])
    if not isinstance(info, dict) or not isinstance(info.get("binaries"), dict):
        raise ValueError("release document must contain binaries")
    url = info["binaries"].get("linux/amd64")
    prefix = f"{asset_url(identity)}?checksum=sha256:"
    if not isinstance(url, str) or not url.startswith(prefix):
        raise ValueError("release document must name the release paxd asset")
    actual_checksum = validate_checksum(url[len(prefix) :])
    if actual_checksum != checksum:
        raise ValueError("release document checksum differs")
    return name


def validate_height(height, label):
    if type(height) is not int:
        raise ValueError(f"{label} must be an integer")
    if height < 1:
        raise ValueError(f"{label} must be positive")
    if height > MAX_HEIGHT:
        raise ValueError(f"{label} exceeds int64")
    return height


def node(args, identity):
    name = validate_name(
        identity["upgrade"] if args.name is None else args.name, identity
    )
    if (args.release_doc is None) != (args.sha256 is None):
        raise ValueError("--release-doc and --sha256 must be supplied together")
    if args.release_doc is not None:
        name = release_document_name(args.release_doc, args.sha256, identity)
    height = args.height
    if height is not None:
        validate_height(height, "height")
    if args.halt_height is not None:
        halt = validate_height(args.halt_height, "halt height")
        if height is not None and height != halt + 1:
            raise ValueError("height must be halt height + 1")
        height = validate_height(halt + 1, "height")
    if height is None:
        raise ValueError("--height or --halt-height is required")
    info = None
    if args.release_doc is not None:
        info = read_plan_info(args.release_doc)
    if args.plan_info is not None:
        info = merge_plan_info({} if info is None else info, args.plan_info, height)
    if name == REGISTERED_PLAN_NAMES["MergeUpgrade"]:
        validate_plan_info(info, height)
    document = {"name": name, "height": height}
    if info is not None:
        document["info"] = json.dumps(info, separators=(",", ":"))
    write_json(
        args.home / "data" / "upgrade-info.json",
        document,
        0o600,
    )


def main():
    parser = ArgumentParser(prog="upgrade_info")
    commands = parser.add_subparsers(dest="command", required=True)
    release_parser = commands.add_parser("release")
    release_parser.add_argument("--sums", type=Path, required=True)
    release_parser.add_argument("--tag")
    release_parser.add_argument("--name")
    release_parser.add_argument("--out", type=Path)
    release_parser.add_argument("--plan-info", type=Path, help="merge validated sequencer info into the release document")
    release_parser.set_defaults(run=release)
    plan_parser = commands.add_parser("plan-info", help="generate merge plan info; JSON byte fields use base64")
    plan_parser.add_argument("--leader", action="append", help="leader public key as 64 hex digits or base64; repeat in primary-first order")
    plan_parser.add_argument("--receipt-key", action="append", help='repeat a JSON entry: {"key_id":"BASE64","public_key":"BASE64","first_height":1,"last_height":18446744073709551615}')
    plan_parser.add_argument("--last-seq", type=int)
    plan_parser.add_argument("--last-seq-hash", help="32-byte hash as hex or base64; the merge requires zero")
    plan_parser.add_argument("--activation-height", type=int)
    plan_parser.add_argument("--plan-height", type=int, help="check receipt coverage at this height plus one; record it in generated info")
    plan_parser.add_argument("--release-doc", type=Path, help="include the binaries map from a release document")
    plan_parser.add_argument("--out", type=Path)
    plan_parser.add_argument("--check", type=Path, help="validate raw plan info or a release document without writing")
    plan_parser.set_defaults(run=plan_info)
    node_parser = commands.add_parser("node")
    node_parser.add_argument("--home", type=Path, required=True)
    node_parser.add_argument("--name")
    node_parser.add_argument("--height", type=int)
    node_parser.add_argument("--halt-height", type=int)
    node_parser.add_argument("--release-doc", type=Path)
    node_parser.add_argument("--sha256")
    node_parser.add_argument("--plan-info", type=Path, help="sequencer info matching the proposal; required for merge unless release-doc already carries it")
    node_parser.set_defaults(run=node)
    try:
        args = parser.parse_args()
        args.run(args, release_identity())
    except (OSError, ValueError, UnicodeError) as error:
        print(f"upgrade_info: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())

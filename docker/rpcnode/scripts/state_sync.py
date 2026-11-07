"""Validate operator-pinned state sync before publishing node configuration."""

import argparse
import ipaddress
import json
import os
from pathlib import Path
import re
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request


MAX_RESPONSE_BYTES = 1024 * 1024
IO_TIMEOUT_SECONDS = 3
REQUEST_DEADLINE_SECONDS = 10
MAX_PEERS = 256


class NoRedirects(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        raise ValueError("state-sync providers must not redirect")


def provider_urls(value):
    providers = value.split(",")
    if len(providers) != 2:
        raise ValueError("configure exactly two independent STATE_SYNC_RPC_SERVERS")
    urls = []
    hosts = set()
    for provider in providers:
        url = urllib.parse.urlsplit(provider.strip())
        if (
            url.scheme not in {"https", "http"}
            or not url.hostname
            or url.username is not None
            or url.password is not None
            or url.query
            or url.fragment
            or url.path not in {"", "/"}
        ):
            raise ValueError("providers must be HTTP(S) origins without credentials")
        host = url.hostname.lower().rstrip(".")
        try:
            address = ipaddress.ip_address(host)
            host = str(address.ipv4_mapped or address) if address.version == 6 else str(address)
        except ValueError:
            if not re.fullmatch(r"[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?", host):
                raise ValueError("invalid provider hostname") from None
        if host in hosts:
            raise ValueError("state-sync providers must use distinct hosts")
        hosts.add(host)
        port = url.port
        if port is not None and not 1 <= port <= 65535:
            raise ValueError("invalid provider port")
        authority = f"[{host}]" if ":" in host else host
        if port is not None:
            authority += f":{port}"
        urls.append(f"{url.scheme}://{authority}")
    return urls


def fetch_result(origin, route):
    opener = urllib.request.build_opener(
        urllib.request.ProxyHandler({}), NoRedirects()
    )
    deadline = time.monotonic() + REQUEST_DEADLINE_SECONDS
    request = urllib.request.Request(origin + route, headers={"Accept": "application/json"})
    with opener.open(request, timeout=IO_TIMEOUT_SECONDS) as response:
        if response.status != 200:
            raise ValueError("state-sync provider did not return HTTP 200")
        if response.headers.get_content_type() != "application/json":
            raise ValueError("state-sync provider did not return JSON")
        length = response.headers.get("Content-Length")
        if length is not None and (
            not re.fullmatch(r"[0-9]+", length) or int(length) > MAX_RESPONSE_BYTES
        ):
            raise ValueError("state-sync response exceeds byte limit")
        data = bytearray()
        while True:
            if time.monotonic() >= deadline:
                raise ValueError("state-sync response exceeded deadline")
            chunk = response.read1(min(65536, MAX_RESPONSE_BYTES + 1 - len(data)))
            if time.monotonic() >= deadline:
                raise ValueError("state-sync response exceeded deadline")
            if not chunk:
                break
            data.extend(chunk)
            if len(data) > MAX_RESPONSE_BYTES:
                raise ValueError("state-sync response exceeds byte limit")
        if length is not None and len(data) != int(length):
            raise ValueError("truncated state-sync response")
    envelope = json.loads(data)
    if (
        not isinstance(envelope, dict)
        or envelope.get("error") is not None
        or not isinstance(envelope.get("result"), dict)
    ):
        raise ValueError("invalid state-sync RPC result")
    return envelope["result"]


def checkpoint_identity(result, chain_id, height, block_hash):
    try:
        header = result["block"]["header"]
        actual_hash = result["block_id"]["hash"]
        if (
            header["chain_id"] != chain_id
            or header["height"] != height
            or not isinstance(actual_hash, str)
            or not re.fullmatch(r"[0-9a-fA-F]{64}", actual_hash)
            or actual_hash.upper() != block_hash
        ):
            raise ValueError("provider checkpoint identity does not match operator pin")
    except (KeyError, TypeError):
        raise ValueError("missing checkpoint identity") from None


def peer_addresses(result):
    peers = result.get("peers")
    if not isinstance(peers, list) or not 1 <= len(peers) <= MAX_PEERS:
        raise ValueError("provider must return a bounded, nonempty peer list")
    addresses = set()
    for peer in peers:
        if not isinstance(peer, dict) or not isinstance(peer.get("url"), str):
            raise ValueError("invalid discovered peer")
        address = peer["url"]
        if not address.startswith("mconn://"):
            raise ValueError("peer must use the node's mconn transport")
        parsed = urllib.parse.urlsplit(address)
        if (
            not re.fullmatch(r"[0-9a-fA-F]{40}", parsed.username or "")
            or peer.get("node_id") != parsed.username
            or parsed.password is not None
            or not parsed.hostname
            or parsed.path
            or parsed.query
            or parsed.fragment
            or parsed.port is None
            or not 1 <= parsed.port <= 65535
        ):
            raise ValueError("invalid peer identity or address")
        # Reject unspecified/multicast addresses and characters that could enter TOML.
        host = parsed.hostname
        try:
            ip = ipaddress.ip_address(host)
        except ValueError:
            if not re.fullmatch(r"[a-zA-Z0-9](?:[a-zA-Z0-9.-]*[a-zA-Z0-9])?", host):
                raise ValueError("invalid peer hostname") from None
        else:
            if ip.is_unspecified or ip.is_multicast:
                raise ValueError("peer address is not dialable")
        host = f"[{host}]" if ":" in host else host
        addresses.add(f"{parsed.username.lower()}@{host}:{parsed.port}")
    return addresses


def replace_keys(template, replacements):
    section = ""
    seen = set()
    output = []
    for line in template.splitlines(keepends=True):
        heading = re.fullmatch(r"\s*\[([^\[\]]+)\]\s*(?:#.*)?", line.strip())
        if heading:
            section = heading[1]
        assignment = re.match(r"\s*([a-z-]+)\s*=", line)
        key = (section, assignment[1]) if assignment else None
        if key in replacements:
            if key in seen:
                raise ValueError("duplicate state-sync configuration key")
            seen.add(key)
            line = f"{key[1]} = {replacements[key]}\n"
        output.append(line)
    if seen != set(replacements):
        raise ValueError("missing state-sync configuration key")
    return "".join(output)


def atomic_write(path, content):
    path = Path(path)
    descriptor, temporary = tempfile.mkstemp(prefix=f".{path.name}.", dir=path.parent)
    try:
        with os.fdopen(descriptor, "w", encoding="utf-8") as stream:
            stream.write(content)
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(temporary, path)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def configure(config, genesis, output, peers, environment):
    providers = provider_urls(environment.get("STATE_SYNC_RPC_SERVERS", ""))
    height = environment.get("STATE_SYNC_TRUST_HEIGHT", "")
    block_hash = environment.get("STATE_SYNC_TRUST_HASH", "")
    if not re.fullmatch(r"[1-9][0-9]{0,18}", height) or int(height) > 2**63 - 1:
        raise ValueError("STATE_SYNC_TRUST_HEIGHT must be a positive int64 checkpoint")
    if not re.fullmatch(r"[0-9a-fA-F]{64}", block_hash):
        raise ValueError("STATE_SYNC_TRUST_HASH must be a 32-byte hexadecimal checkpoint")
    block_hash = block_hash.upper()
    chain_id = json.loads(Path(genesis).read_text(encoding="utf-8"))["chain_id"]
    if chain_id != "pax":
        raise ValueError("genesis chain identity must match paxd --chain-id pax")
    for provider in providers:
        checkpoint_identity(
            fetch_result(provider, f"/block?height={height}"), chain_id, height, block_hash
        )
    discovered = set()
    for provider in providers:
        discovered.update(peer_addresses(fetch_result(provider, "/net_info")))
    peer_list = sorted(discovered)
    template = Path(config).read_text(encoding="utf-8")
    updated = replace_keys(
        template,
        {
            ("statesync", "enable"): "true",
            ("statesync", "rpc-servers"): json.dumps(",".join(providers)),
            ("statesync", "trust-height"): height,
            ("statesync", "trust-hash"): json.dumps(block_hash),
            ("p2p", "persistent-peers"): json.dumps(",".join(peer_list)),
        },
    )
    # All network, identity and source validation precedes either publication.
    atomic_write(peers, "\n".join(peer_list) + "\n")
    atomic_write(output, updated)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("config", "genesis", "output", "peers"):
        parser.add_argument(f"--{name}", required=True, type=Path)
    args = parser.parse_args()
    try:
        configure(args.config, args.genesis, args.output, args.peers, os.environ)
    except (OSError, ValueError, KeyError, TypeError, urllib.error.URLError) as error:
        parser.exit(1, f"State-sync validation failed: {error}\n")


if __name__ == "__main__":
    main()

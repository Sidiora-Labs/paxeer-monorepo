"""Validate native bank-send funding and the public root RPC documentation contract."""

import argparse
import json
from pathlib import Path
import re
from urllib.parse import urlsplit

INVENTORY = "docs/site/docs/reference/public-rpc.md"
BANK_PRODUCER = "services/platform/hosted/core/src/chain_fund.rs"
BANK_ABI = "precompiles/bank/abi.json"


def bank_send_contract(root):
    abi = json.loads((root / BANK_ABI).read_text())
    methods = [row for row in abi if row.get("type") == "function" and row.get("name") == "send"]
    signatures = {tuple(item["type"] for item in row["inputs"]) for row in methods}
    if signatures != {("address", "address", "string", "uint256"),
                      ("address", "address", "string", "uint256", "bytes32")}:
        raise ValueError("retained native bank-send ABI required")
    return signatures


def public_roots(text):
    urls = re.findall(r"`((?:https|wss)://[^`]+)`", text)
    expected_hosts = {"api-mainnet-beta.paxeer.network"} | {
        "api" + str(number) + ".mainnet-beta.paxeer.network" for number in range(1, 15)
    }
    roots = set()
    for value in urls:
        url = urlsplit(value)
        if (url.hostname not in expected_hosts or url.path not in {"", "/"}
                or url.port is not None or url.username or url.password or url.query or url.fragment):
            raise ValueError("public JSON-RPC must use a served root endpoint")
        roots.add(url.scheme + "://" + url.hostname)
    expected = {scheme + "://" + host for host in expected_hosts for scheme in ("https", "wss")}
    if roots != expected or "125" not in text or "eth_chainId" not in text:
        raise ValueError("complete public root RPC roster and chain identity required")
    return roots


def native_funding(text, roots):
    if re.search(r"\bFAUCET_URL\b", text):
        raise ValueError("unserved faucet configuration forbidden")
    endpoints = re.findall(r"https://(?:api[^\s`\"'<>)]*)", text)
    if not endpoints or any(endpoint.rstrip(".,;") not in roots for endpoint in endpoints):
        raise ValueError("funding documentation requires public root RPC endpoints")
    # Retired-flow warnings may remain; executable custody credit instructions may not.
    commands = "\n".join(re.findall(r"```[^\n]*\n(.*?)```", text, re.S))
    if re.search(r"custody_credit\.py|sign-credit|lx_sendActivity|0x0{36}1013", commands, re.I):
        raise ValueError("retired funding producer or transport in onboarding")
    if (not re.search(r"native|bank[ -]send|\bPAX\b", text, re.I)
            or not re.search(r"transfer|funded account|funded EVM|bank[ -]send", text, re.I)
            or not re.search(r"eth_getBalance|eth_chainId|chain.*125", text, re.I)):
        raise ValueError("native bank-send funding and chain/balance verification required")
    return True


def main():
    parser = argparse.ArgumentParser(description=__doc__, allow_abbrev=False)
    parser.add_argument("page", type=Path)
    args = parser.parse_args()
    root = Path(__file__).resolve().parents[3]
    bank_send_contract(root)
    roots = public_roots((root / INVENTORY).read_text())
    native_funding(args.page.read_text(), roots)


if __name__ == "__main__":
    main()

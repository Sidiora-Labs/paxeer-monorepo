#!/usr/bin/env python3
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import sys
import urllib.parse
import urllib.request

ROOT = Path(__file__).resolve().parents[3]
SCHEMA_TEST = "rpc::tests::schema_lists_every_public_method"
HOSTED_METHODS = frozenset({
    "px_register", "px_getCommit", "px_getValidators", "px_getReceiptKeys",
    "px_getSequenceBatch", "px_requestFunds", "pax_sequencerReceipt", "px_resolveAccount",
    "px_getAccount", "px_getBalances", "px_listAssets", "px_getNetwork", "px_getHistory",
    "px_getUnifiedHistory", "px_listHolderRecords", "px_getCapabilities", "px_getRouteCatalogue",
})
INFERENCE_METHODS = frozenset({
    "px_getInferenceRooms", "px_getInferenceRoom", "px_getInferenceDeal",
    "px_getInferencePeer", "px_getInferenceReceipts",
})
NODE_METHODS = ("eth_chainId", "eth_call", "eth_sendRawTransaction", "pax_sequencerReceipt")


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def git(*args):
    return subprocess.check_output(["git", *args], cwd=ROOT, text=True).strip()


def protected(path, executable=False):
    info = path.lstat()
    require(stat.S_ISREG(info.st_mode) and not info.st_mode & 0o077,
            "artifact must be a protected regular file")
    require(not executable or os.access(path, os.X_OK), "artifact must be executable")


def document(variable):
    value = os.environ.get(variable)
    require(value, variable + " is required")
    path = Path(value)
    protected(path)
    return json.loads(path.read_text())


def artifact(record, revision):
    require(record["source_revision"] == revision, "artifact revision mismatch")
    path = Path(record["path"])
    require(path.is_absolute(), "absolute artifact path required")
    protected(path, True)
    with path.open("rb") as stream:
        require(hashlib.file_digest(stream, "sha256").hexdigest() == record["sha256"],
                "artifact digest mismatch")
    return str(path)


def rpc(endpoint, method, params):
    url = urllib.parse.urlsplit(endpoint)
    require(url.scheme in {"http", "https"} and url.path in {"", "/"}
            and not url.query and not url.fragment and not url.username and not url.password,
            "node JSON-RPC root endpoint required")
    request = urllib.request.Request(endpoint, method="POST",
        headers={"Content-Type": "application/json", "Accept": "application/json"},
        data=json.dumps({"jsonrpc": "2.0", "id": 1, "method": method, "params": params}).encode())
    with urllib.request.urlopen(request, timeout=20) as response:
        require(response.headers.get_content_type() == "application/json", "JSON-RPC content type required")
        raw = response.read(4 * 1024 * 1024 + 1)
    require(len(raw) <= 4 * 1024 * 1024, "bounded JSON-RPC response required")
    value = json.loads(raw)
    require(isinstance(value, dict) and value.get("jsonrpc") == "2.0" and type(value.get("id")) is int and value["id"] == 1
            and ("result" in value) != ("error" in value), "invalid JSON-RPC envelope")
    return value


def hosted_schema(schema):
    methods = schema["methods"]
    require(isinstance(methods, list) and all(isinstance(item, dict) and isinstance(item.get("name"), str)
                                            for item in methods), "hosted method schema fields differ")
    names = [item["name"] for item in methods]
    require(len(names) == len(set(names)), "duplicate hosted schema method")
    require(HOSTED_METHODS | INFERENCE_METHODS == set(names),
            "retained hosted and node inference contract inventory differs")
    require(all(name.startswith("px_") or name == "pax_sequencerReceipt" for name in names),
            "node methods must be qualified separately")
    for item in methods:
        require(isinstance(item.get("params"), list) and isinstance(item.get("result"), dict),
                "hosted parameter and result schemas required")
    return len(names)


def node_schema(endpoint):
    modules = rpc(endpoint, "rpc_modules", [])
    require(isinstance(modules.get("result"), dict) and {"eth", "pax"} <= set(modules["result"]),
            "node eth and pax namespaces required")
    require(rpc(endpoint, "eth_chainId", []).get("result") == "0x7d", "Paxeer chain 125 required")
    call = rpc(endpoint, "eth_call", [{"to": "0x" + "00" * 20, "data": "0x"}, "latest"])
    require(call.get("result") == "0x", "node eth_call schema probe failed")
    # Invalid transaction bytes exercise dispatch and the parameter decoder without submitting a transaction.
    sent = rpc(endpoint, "eth_sendRawTransaction", ["0x"])
    require(sent.get("error", {}).get("code") in {-32602, -32000},
            "node raw-transaction decoder refusal required")
    receipt = rpc(endpoint, "pax_sequencerReceipt", ["0x" + "00" * 32])
    require(("result" in receipt and receipt["result"] is None)
            or receipt.get("error", {}).get("code") == -32000,
            "node receipt method schema probe failed")
    return len(NODE_METHODS)


def main():
    count = 0
    try:
        require(not git("status", "--porcelain", "--untracked-files=no"), "clean source required")
        revision = git("rev-parse", "HEAD")
        build = document("PAXEER_X_SCHEMA_BUILD_MANIFEST")
        require(build["source_revision"] == revision and build["source_tree"] == git("rev-parse", "HEAD^{tree}")
                and build["build_exit"] == 0, "source-bound successful scoped build required")
        executable = artifact(build["artifacts"]["gateway_tests"], revision)
        result = subprocess.run([executable, SCHEMA_TEST, "--exact", "--nocapture", "--test-threads=1"],
                                cwd=ROOT, stdin=subprocess.DEVNULL, stdout=subprocess.PIPE,
                                stderr=subprocess.STDOUT, text=True, timeout=840)
        print(result.stdout, end="")
        markers = re.findall(r"^test result: ok\. (\d+) passed; 0 failed; 0 ignored; 0 measured; \d+ filtered out;",
                             result.stdout, re.M)
        require(result.returncode == 0 and markers == ["1"] and f"test {SCHEMA_TEST} ... ok" in result.stdout,
                "exactly one hosted schema contract must execute")
        count = 1
        hosted_schema(json.loads((ROOT / "services/platform/hosted/gateway/openrpc.json").read_text()))
        count += node_schema(os.environ["PAXEER_X_NODE_RPC_URL"])
        require(not git("status", "--porcelain", "--untracked-files=no") and git("rev-parse", "HEAD") == revision,
                "schema source changed during verification")
        return 0
    except (OSError, ValueError, KeyError, RuntimeError, subprocess.SubprocessError) as error:
        print("hosted-node-schema: refusal: " + str(error), file=sys.stderr)
        return 1
    finally:
        print(f"PAXEER_X_GATE tests={count} skipped=0")


if __name__ == "__main__":
    sys.exit(main())

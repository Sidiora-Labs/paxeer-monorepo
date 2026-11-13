#!/usr/bin/env bash
set -euo pipefail

if ! command -v python3 >/dev/null 2>&1; then
  echo 'postflight: python3 is required' >&2
  exit 1
fi

exec python3 - "$@" <<'PY'
import argparse
import base64
import json
import math
import re
import sys
import time
import urllib.error
import urllib.parse
import urllib.request


class Options(argparse.ArgumentParser):
    def error(self, message):
        self.print_usage(sys.stderr)
        self.exit(1, f"postflight: {message}\n")


def height_option(value):
    if not re.fullmatch(r"[0-9]+", value) or int(value) < 1:
        raise argparse.ArgumentTypeError("upgrade height must be a positive integer")
    return int(value)


def seconds_option(value):
    try:
        seconds = float(value)
    except ValueError:
        raise argparse.ArgumentTypeError("seconds must be a finite positive number")
    if not math.isfinite(seconds) or seconds <= 0:
        raise argparse.ArgumentTypeError("seconds must be a finite positive number")
    return seconds


def endpoint(value):
    parsed = urllib.parse.urlsplit(value)
    if parsed.scheme not in ("http", "https") or not parsed.hostname:
        raise argparse.ArgumentTypeError("RPC address must be an HTTP or HTTPS URL")
    if parsed.query or parsed.fragment or parsed.username or parsed.password:
        raise argparse.ArgumentTypeError("RPC address must not contain credentials, query or fragment")
    return value.rstrip("/")


parser = Options(
    prog="postflight.sh",
    description="Read-only checks of a Paxeer 7.0.0 node after its upgrade height (requires Python 3).",
    epilog=("Ledger/auth/chains parameters are read from committed store keys; pay/agent and "
            "sequencer use Cosmos queries. Programs and lxphost operating schedules are read "
            "through the Programs precompile. Every served namespace must answer a read; "
            "denied legacy methods are reported as failures. No transactions are sent."),
    formatter_class=argparse.ArgumentDefaultsHelpFormatter,
)
parser.add_argument("--upgrade-height", required=True, type=height_option, help="passed proposal height")
# Ports: consensus/config/config.go, rpc/config/config.go, sdk/server/config/config.go.
parser.add_argument("--comet-rpc", type=endpoint, default="http://127.0.0.1:26657", help="CometBFT HTTP RPC")
parser.add_argument("--evm-rpc", type=endpoint, default="http://127.0.0.1:8545", help="root EVM JSON-RPC")
parser.add_argument("--rest-rpc", type=endpoint, default="http://127.0.0.1:1317", help="Cosmos REST API")
parser.add_argument("--interval", type=seconds_option, default=5.0, help="seconds between status reads")
parser.add_argument("--timeout", type=seconds_option, default=10.0, help="timeout in seconds per request")
options = parser.parse_args()


class CheckError(Exception):
    pass


def require(condition, reason):
    if not condition:
        raise CheckError(reason)


def request(url, payload=None):
    data = None if payload is None else json.dumps(payload).encode()
    req = urllib.request.Request(url, data=data, headers={"Content-Type": "application/json"})
    try:
        with urllib.request.urlopen(req, timeout=options.timeout) as response:
            body = response.read(4 * 1024 * 1024 + 1)
        require(len(body) <= 4 * 1024 * 1024, "response exceeds 4 MiB")
        result = json.loads(body)
    except urllib.error.HTTPError as error:
        raise CheckError(f"HTTP {error.code}") from None
    except (urllib.error.URLError, TimeoutError, OSError):
        raise CheckError("RPC request failed or timed out") from None
    except (ValueError, UnicodeError):
        raise CheckError("invalid JSON response") from None
    require(isinstance(result, dict), "expected a JSON object")
    if result.get("error") is not None:
        error = result["error"]
        if isinstance(error, dict):
            raise CheckError(f"RPC error {error.get('code')}: {error.get('message')}")
        raise CheckError("RPC error response")
    return result


def rpc(method, params):
    result = request(options.evm_rpc, {"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
    require(result.get("jsonrpc") == "2.0" and type(result.get("id")) is int and result["id"] == 1,
            "invalid JSON-RPC envelope")
    require("result" in result and result["result"] is not None, "missing JSON-RPC result")
    return result["result"]


failed = False


def check(label, action):
    global failed
    try:
        value = action()
    except (CheckError, KeyError, TypeError, ValueError, AttributeError) as error:
        failed = True
        reason = str(error) if isinstance(error, CheckError) else "malformed response"
        reason = " ".join(reason.split()) or "check failed"
        print(f"{label}: {reason}", flush=True)
        return None
    print(f"{label}: ok", flush=True)
    return value


def version():
    info = request(options.rest_rpc + "/cosmos/base/tendermint/v1beta1/node_info")
    actual = info["application_version"]["version"]
    require(actual in ("7.0.0", "v7.0.0"), f"expected 7.0.0, got {actual}")


check("version", version)

# Both status responses participate in the height and catch-up checks.
statuses = []
status_error = None
for sample in range(2):
    if sample:
        time.sleep(options.interval)
    try:
        statuses.append(request(options.comet_rpc + "/status")["result"]["sync_info"])
    except (CheckError, KeyError, TypeError) as error:
        status_error = str(error) if isinstance(error, CheckError) else "malformed status response"


def heights():
    require(status_error is None, status_error)
    values = [status["latest_block_height"] for status in statuses]
    require(all(isinstance(value, str) and re.fullmatch(r"[0-9]+", value) for value in values),
            "invalid latest height")
    first, second = map(int, values)
    require(first > options.upgrade_height and second > options.upgrade_height,
            f"heights {first}, {second} must be above upgrade height {options.upgrade_height}")
    require(second > first, f"height did not rise ({first} -> {second})")


def synced():
    require(status_error is None, status_error)
    require(all(status["catching_up"] is False for status in statuses), "node is catching up or flag is invalid")


def chain_id():
    value = rpc("eth_chainId", [])
    require(isinstance(value, str) and re.fullmatch(r"0x[0-9a-fA-F]+", value), "invalid chain identifier")
    require(int(value, 16) == 125, f"expected 125 (0x7d), got {value}")


check("height", heights)
check("catching_up", synced)
check("chain_id", chain_id)


def store_params(module):
    # ParamsKey is 0x01 in modules/{layerxledger,layerxauth,layerxchains}/types/keys.go.
    query = urllib.parse.urlencode({"path": json.dumps(f"/store/{module}/key"), "data": "0x01", "prove": "false"})
    response = request(options.comet_rpc + "/abci_query?" + query)["result"]["response"]
    require(type(response["code"]) is int and response["code"] == 0, "parameter store query failed")
    require(base64.b64decode(response["key"], validate=True) == b"\x01", "parameter store key differs")
    data = base64.b64decode(response["value"], validate=True)
    if module == "layerxchains":
        require(isinstance(json.loads(data), dict), "invalid chains parameters")
    else:
        # Protobuf's empty message is the valid unpaused default. Both Params
        # messages contain only varints (paused and auth's retention counter).
        pos = 0
        while pos < len(data):
            tag = data[pos]
            pos += 1
            require(tag in ((8, 16) if module == "layerxauth" else (8,)), "invalid parameter field")
            value = 0
            shift = 0
            while True:
                require(pos < len(data) and shift < 70, "invalid parameter encoding")
                byte = data[pos]
                pos += 1
                value |= (byte & 127) << shift
                shift += 7
                if byte < 128:
                    break
            require(value < 2**64 and (tag != 8 or value in (0, 1)), "invalid parameter value")


def rest_params(module):
    params = request(options.rest_rpc + f"/paxprotocol/paxchain/{module}/params")["params"]
    require(isinstance(params, dict) and type(params["paused"]) is bool, "invalid module parameters")


def sequencer_params():
    for key in ("Leaders", "ReceiptKeys"):
        query = urllib.parse.urlencode({"subspace": "sequencer", "key": key})
        param = request(options.rest_rpc + "/cosmos/params/v1beta1/params?" + query)["param"]
        require(param["subspace"] == "sequencer" and param["key"] == key, "sequencer parameter key differs")
        require(isinstance(json.loads(param["value"]), list), "invalid sequencer parameter value")


def words(*values):
    return "".join(f"{value:064x}" for value in values)


def address(suffix):
    return "0x" + suffix.lower().zfill(40)


def call(suffix, calldata, count, dynamic=False):
    value = rpc("eth_call", [{"to": address(suffix), "data": "0x" + calldata}, "latest"])
    require(isinstance(value, str) and re.fullmatch(r"0x[0-9a-fA-F]+", value), "empty or invalid ABI result")
    require((len(value) - 2) % 64 == 0, "unaligned ABI result")
    size = (len(value) - 2) // 64
    require(size >= count if dynamic else size == count, "unexpected ABI result length")
    return value


check("module sequencer parameters", sequencer_params)
for module in ("layerxledger", "layerxauth", "layerxchains"):
    check(f"module {module} parameters", lambda module=module: store_params(module))
for module in ("layerxpay", "layerxagent"):
    check(f"module {module} parameters", lambda module=module: rest_params(module))
# Programs and the genesis-only host expose their schedules through the Programs ABI.
check("module layerxprograms fee schedule", lambda: call("101d", "bbe2e42d", 9))
check("module lxphost metering schedule", lambda: call("101d", "543c7a2f", 10))

# docs/system/precompiles.md and the linked ABIs: one view/pure selector per
# merge-gated address. Inputs use absent records or self-contained arithmetic.
precompiles = (
    ("101a", "accountId", "3f050cbc" + words(0, 0), 1, False),
    ("101b", "askStreamOf", "929c1012" + words(0), 1, False),
    ("101c", "isLive", "d4a03b3d" + words(0, 0, 0), 4, True),
    ("101d", "getCurrentFeeSchedule", "bbe2e42d", 9, False),
    ("101e", "chainCount", "e02e1bfd", 1, False),
    ("0901", "resolveOffset", "454ad3a1" + words(100, 0), 1, False),
    ("0902", "computeClearing", "8397e9f8" + words(100, 96, 128, 0, 0), 3, False),
    ("0903", "aggregate", "81097a95" + words(32, 1, 100, 1, 1), 2, False),
    ("0904", "scoreFill", "e087a6ba" + words(100, 100), 1, False),
    ("0905", "pending", "5eebea20" + words(0), 2, True),
    ("0906", "accrued", "f52981f4" + words(0), 1, False),
    ("0907", "rootCount", "26f02502" + words(0), 1, False),
    ("0908", "hashTypedData", "d61c3c34" + words(0, 0), 1, False),
)
for suffix, method, calldata, count, dynamic in precompiles:
    check(f"precompile {address(suffix)} {method}",
          lambda suffix=suffix, calldata=calldata, count=count, dynamic=dynamic: call(suffix, calldata, count, dynamic))

# rpc/server.go and docs/system/rpc.md. The test namespace has no read method.
# Block reads in pax/pax2 require the operator's enabled_legacy_pax_apis policy.
namespace_reads = {
    "debug": ("debug_traceCall", [{"to": address("0"), "gas": "0x5208"}, "latest", {"tracer": "callTracer"}]),
    "echo": ("echo_echo", ["postflight"]),
    "eth": ("eth_blockNumber", []),
    "net": ("net_version", []),
    "pax": ("pax_getBlockByNumber", ["latest", False]),
    "pax2": ("pax2_getBlockByNumber", ["latest", False]),
    "px": ("px_getNetwork", []),
    "txpool": ("txpool_status", []),
    "web3": ("web3_clientVersion", []),
}


def namespaces():
    modules = rpc("rpc_modules", [])
    require(isinstance(modules, dict) and modules, "invalid namespace inventory")
    require(all(isinstance(name, str) and isinstance(version, str) for name, version in modules.items()),
            "invalid namespace inventory")
    return modules


served = check("namespace rpc rpc_modules", namespaces)
for namespace, (method, params) in namespace_reads.items():
    def namespace_read(namespace=namespace, method=method, params=params):
        require(served is not None, "namespace inventory unavailable")
        require(namespace in served, "namespace is not served")
        return rpc(method, params)
    check(f"namespace {namespace} {method}", namespace_read)
if served is not None:
    for namespace in sorted(set(served) - set(namespace_reads) - {"rpc"}):
        check(f"namespace {namespace}", lambda: require(False, "no read method declared for served namespace"))

sys.exit(1 if failed else 0)
PY

#!/usr/bin/env bash
set -euo pipefail

# The host map, rpc_domain and repo_root are the probe's.
# shellcheck source=tools/bringup/check-live.sh
. "$(dirname "${BASH_SOURCE[0]}")/check-live.sh"

usage() {
	cat <<'EOF'
usage: tools/bringup/xweb-config.sh prices | render serve|attestor <directory> | inventory | register

The x-websearch configs of the bring-up. Rendering uses the space-separated
XWEB_SERVE_NAMES when set; otherwise it reads the serving RPC names from
BRINGUP_HOSTS_FILE through search-front.sh names. Never prints the host map
or a key and writes no key: each key is generated on its own host.

prices    prints one "<symbol> <asset id> <price>" line for SID, PAX, USDC and
          USDL, the price in the asset's base units for one request at the
          approved 0.001 US dollars and 13.44 US dollars per PAX: PAX exactly
          1/13440, SID that PAX amount times XWEB_SID_PER_PAX, USDC and USDL
          1/1000 of their unit, each rounded up to a whole base unit of the
          asset's decimals. Every asset id and its decimals must match the
          matching record the gateway's px_listAssets reports: that symbol,
          those decimals, not paused, an authenticated committed snapshot.

inventory prints, one line each, the read-backs the paid search and web
          attestation settle through, and one "input <NAME> absent" line for
          every input it lacks; never invents an amount or an authority:
            receiver <serving name> payTo=<account>  the receiver each serving
                     RPC name's unpaid /search challenge offers, priced at the
                     approved prices
            payer    XWEB_PAYER_ACCOUNT's balance of XWEB_PAYER_ASSET against
                     XWEB_PAYER_MIN_BALANCE
            kernel-did attestor-<N>  XWEB_ATTESTOR_<N>_RECEIVER_DID registered
                     (px_getAccount), the submitter and payout DID
            signer attestor-<N>  XWEB_ATTESTOR_<N>_SIGNER a member of the
                     precompile's getAttestors, the set holding exactly the
                     four, the threshold XWEB_ATTESTOR_THRESHOLD and a majority
            submitter attestor-<N>  XWEB_ATTESTOR_<N>_SUBMITTER's balance on
                     XWEB_INVENTORY_EVM_ENDPOINT against XWEB_SUBMITTER_MIN_WEI
          Exits 1 on any failed read-back or absent input.

register  registers the Ed25519 key of XWEB_REGISTER_KEY_FILE (32 hex bytes
          as the sidecar reads it, a regular file readable by its owner only,
          never printed) with the gateway's px_register, signing the beta
          tenant binding, and checks the answer names exactly that key, the
          beta tenant and the subject the key derives; registering a key
          again answers the same principal. Prints the public key and subject.

render serve <directory>
          writes <directory>/<apiN name>.json for every serving RPC name that
          XWEB_SERVE_NAMES or search-front.sh names lists: listen 127.0.0.1:8482,
          gateway.endpoint the exact configured hosted gateway URL with the
          node's complete receipt trust history, the four assets with their prices, evm the node's
          loopback JSON-RPC on chain 125, kernel_network_id 125, peers the
          https names of the other serving RPC names, the crawl seeds of
          XWEB_SEEDS and no kernel block.

render attestor <directory>
          writes <directory>/attestor-<N>.json for the four Fly attestor apps
          of services/interop/deploy/x-websearch/attestor-<N>.toml: listen [::]:8480,
          data_dir /data/state, gateway.endpoint the configured hosted
          gateway URL with the node's complete receipt trust history, the four assets with their
          prices, evm the https name of a serving RPC name (attestor N takes
          the (N-1)th of the list, wrapping) on chain 125, kernel_network_id
          125, peers the loopback hops http://127.0.0.1:849<M> of the other
          three, the crawl seeds of XWEB_SEEDS and only supported config fields.
          Program relay authority is configured by the runtime environment;
          no root kernel object is written.

Inputs (all required, none printed but the ids and prices):
  XWEB_ASSET_ID_SID, XWEB_ASSET_ID_PAX, XWEB_ASSET_ID_USDC, XWEB_ASSET_ID_USDL
                          the 64-hex asset ids the kernel genesis registered
  XWEB_DECIMALS_SID, XWEB_DECIMALS_PAX, XWEB_DECIMALS_USDC, XWEB_DECIMALS_USDL
                          the decimals of those genesis asset records
  XWEB_RECEIPT_KEYS_ENDPOINT
                          node JSON-RPC publishing px_getReceiptKeys, defaults to
                          XWEB_EVM_ENDPOINT; every historical range is retained
  XWEB_SEEDS              the space-separated https URLs the sidecar crawls from
  XWEB_SID_PER_PAX        SID per PAX, default the owner-set 3.114
  XWEB_GATEWAY_ENDPOINT   required operator-deployed hosted gateway JSON-RPC
                          endpoint for px_listAssets, px_register and px_getAccount;
                          use the deployed gateway URL verbatim (including its
                          declared path). No synthesized public router /rpc
                          default; it must be separate from every node root
  XWEB_EVM_ENDPOINT       node root JSON-RPC for eth_* methods, default
                          http://127.0.0.1:8545 (root only, never append /rpc)
  XWEB_PAYER_ADDRESS      EVM payer address bound through addr 0x1004, used
                          by inventory to request each search payment challenge
  XWEB_PAX_USD_PRICE      when set, must be the approved 13.44

Exits 1 when a destination does not answer, 2 on a usage error or a missing
or malformed input, or an input the registered asset records contradict.
EOF
}

xweb_symbols=(SID PAX USDC USDL)
xweb_gateway="${XWEB_GATEWAY_ENDPOINT:-}"
xweb_evm="${XWEB_EVM_ENDPOINT:-http://127.0.0.1:8545}"
xweb_sid_per_pax="${XWEB_SID_PER_PAX:-3.114}"

xweb_need() {
	if [ -z "${!1:-}" ]; then
		echo "xweb-config: $1 is unset" >&2
		exit 2
	fi
}

# xweb_rpc_py: the JSON-RPC read every Python step below shares, a POST of
# one call to an endpoint bounded by XWEB_TIMEOUT seconds; status 1 when the
# destination does not answer, a reply's error is returned as an error.
xweb_rpc_py='
import json
import os
import urllib.error
import urllib.request

def unique_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError("duplicate RPC field")
        result[key] = value
    return result

def rpc(endpoint, method, params):
    body = json.dumps({"jsonrpc": "2.0", "id": 1, "method": method, "params": params}).encode()
    request = urllib.request.Request(endpoint, body, {"content-type": "application/json"})
    try:
        with urllib.request.urlopen(request, timeout=int(os.environ.get("XWEB_TIMEOUT", "30"))) as reply:
            answer = json.loads(reply.read(1 << 20), object_pairs_hook=unique_object)
    except (OSError, ValueError, urllib.error.URLError):
        return None, "unreachable"
    if not isinstance(answer, dict) or answer.get("jsonrpc") != "2.0" or type(answer.get("id")) is not int or answer["id"] != 1:
        return None, "malformed"
    if ("result" in answer) == ("error" in answer):
        return None, "malformed"
    if "error" in answer:
        error = answer["error"]
        if not isinstance(error, dict) or type(error.get("code")) is not int or not isinstance(error.get("message"), str):
            return None, "malformed"
        return None, "error=%s" % error["code"]
    return answer.get("result"), None
'

# Pricing, registration and config production share the installed Go transport.
xweb_prices() {
 xweb_need XWEB_GATEWAY_ENDPOINT
 "${BRINGUP_INSPECT_BIN:-bringup-inspect}" xweb-config prices
}

xweb_render() {
 local kind="$1" directory="$2" listing
 local -a names=()
 xweb_need XWEB_GATEWAY_ENDPOINT
 xweb_need XWEB_SEEDS
 if [ "${XWEB_SERVE_NAMES+x}" = x ]; then
  read -r -a names <<<"$XWEB_SERVE_NAMES"
 else
  listing="$("$(dirname "${BASH_SOURCE[0]}")/search-front.sh" names)" || return 1
  mapfile -t names < <(sed -n 's/^serve //p' <<<"$listing")
 fi
 "${BRINGUP_INSPECT_BIN:-bringup-inspect}" xweb-config render "$kind" "$directory" "${names[@]}"
}
xweb_render_serve() { xweb_render serve "$1"; }
xweb_render_attestor() { xweb_render attestor "$1"; }

# xweb_inventory: the read-backs of the receivers, the payer, the kernel
# submitter and payout DIDs, the web signers and the EVM submitters, one line
# each, and an "input <NAME> absent" line per input it lacks.
xweb_inventory() {
	if [ -z "$xweb_gateway" ]; then
		echo "xweb-config: hosted gateway endpoint is not configured (XWEB_GATEWAY_ENDPOINT)" >&2
		return 2
	fi
	local listing prices="" sym id decimals ready=1
	local -a names=()
	for sym in "${xweb_symbols[@]}"; do
		id="XWEB_ASSET_ID_$sym"
		decimals="XWEB_DECIMALS_$sym"
		[ -n "${!id:-}" ] && [ -n "${!decimals:-}" ] || ready=0
	done
	if [ "$ready" -eq 1 ]; then
		prices="$(xweb_prices)"
	fi
	listing="$("$(dirname "${BASH_SOURCE[0]}")/search-front.sh" names)" || return 1
	mapfile -t names < <(sed -n 's/^serve //p' <<<"$listing")
	python3 -c "$xweb_rpc_py"'
import base64
import json
import os
import re
import sys
import urllib.error
import urllib.request

gateway, node, prices, *names = sys.argv[1:]
env = os.environ.get
failures = 0
def line(ok, text):
    global failures
    failures += 0 if ok else 1
    print(("pass " if ok else "fail ") + text)
def need(name, pattern):
    global failures
    value = env(name, "")
    if not value:
        failures += 1
        print("input " + name + " absent")
        return None
    if not re.fullmatch(pattern, value):
        print("xweb-config: " + name + " is malformed", file=sys.stderr)
        sys.exit(2)
    return value
priced = {}
for row in prices.splitlines():
    sym, asset, price = row.split()
    priced[asset] = (sym, price)
if not priced:
    for sym in ("SID", "PAX", "USDC", "USDL"):
        need("XWEB_ASSET_ID_" + sym, r"(0x)?[0-9a-fA-F]{64}")
        need("XWEB_DECIMALS_" + sym, r"[0-9]{1,2}")
payer_address = need("XWEB_PAYER_ADDRESS", r"0x[0-9a-fA-F]{40}")
if not names:
    line(False, "receiver serving=0")
for name in names:
    if not payer_address:
        break
    request = urllib.request.Request("https://" + name + "/search?q=paxeer", headers={"LAYERX-PAYER-ADDRESS": payer_address})
    status, header = None, None
    try:
        with urllib.request.urlopen(request, timeout=int(env("XWEB_TIMEOUT", "30"))) as reply:
            status = reply.status
    except urllib.error.HTTPError as error:
        status, header = error.code, error.headers.get("payment-required")
    except OSError:
        pass
    try:
        offers = json.loads(base64.b64decode(header or "", validate=True))["accepts"]
        receivers = {offer["payTo"] for offer in offers}
        amounts = {(offer["asset"], offer["amount"]) for offer in offers}
    except (ValueError, KeyError, TypeError):
        line(False, "receiver https://%s/search http=%s offers=none" % (name, status or "none"))
        continue
    approved = {(asset, price) for asset, (_, price) in priced.items()}
    ok = status == 402 and len(receivers) == 1 and len(offers) == 8 and (not priced or amounts == approved)
    line(ok, "receiver https://%s/search http=402 payTo=%s offers=%d priced=%s" % (name, ",".join(sorted(receivers)), len(offers),
         "unchecked" if not priced else "approved" if amounts == approved else "differs"))
account = need("XWEB_PAYER_ACCOUNT", r"[0-9a-f]{64}")
payer_sym = need("XWEB_PAYER_ASSET", r"SID|PAX|USDC|USDL")
minimum = need("XWEB_PAYER_MIN_BALANCE", r"[1-9][0-9]{0,37}")
def balance_line(label, account, sym, minimum):
    want = next((asset for asset, (name, _) in priced.items() if name == sym), None)
    if want is None:
        line(False, "%s account=%s balance=asset-unavailable" % (label, account))
        return
    result, error = rpc(node, "eth_call", [{
        "to": "0x" + "00" * 18 + "101a",
        "data": "0xb1ffc03e" + account + want,
    }, "pending"])
    if error or not isinstance(result, str) or not re.fullmatch(r"0x[0-9a-fA-F]{64}", result) or int(result, 16) >= 1 << 128:
        line(False, "%s account=%s balance=%s" % (label, account, error or "malformed"))
        return
    balance = str(int(result, 16))
    ok = minimum is not None and int(balance) >= int(minimum)
    line(ok, "%s account=%s asset=%s balance=%s minimum=%s" % (label, account, sym, balance or "none", minimum or "absent"))
if account and payer_sym and minimum:
    balance_line("payer", account, payer_sym, minimum)
fee_sym = need("XWEB_KERNEL_FEE_ASSET", r"SID|PAX|USDC|USDL")
for n in range(1, 5):
    did = need("XWEB_ATTESTOR_%d_RECEIVER_DID" % n, r"did:[a-z0-9._:-]+")
    if did:
        result, error = rpc(gateway, "px_getAccount", [did])
        registered = result.get("account") if isinstance(result, dict) else None
        ok = error is None and isinstance(registered, dict) and registered.get("layerx_did") == did and registered.get("bound") is True
        line(ok, "kernel-did attestor-%d did=%s registered=%s" % (n, did, "yes" if ok else error or "no"))
    payout = need("XWEB_ATTESTOR_%d_PAYOUT_ACCOUNT" % n, r"[0-9a-f]{64}")
    if payout and fee_sym:
        balance_line("payout attestor-%d" % n, payout, fee_sym, "0")
evm = need("XWEB_INVENTORY_EVM_ENDPOINT", r"https?://[^ ]+")
threshold = need("XWEB_ATTESTOR_THRESHOLD", r"[1-9]")
signers = [need("XWEB_ATTESTOR_%d_SIGNER" % n, r"0x[0-9a-fA-F]{40}") for n in range(1, 5)]
if evm:
    try:
        from Crypto.Hash import keccak
    except ImportError:
        print("xweb-config: pycryptodome keccak is required", file=sys.stderr)
        sys.exit(2)
    selector = keccak.new(digest_bits=256, data=b"getAttestors()").hexdigest()[:8]
    result, error = rpc(evm, "eth_call", [{"to": "0x" + "00" * 18 + "1019", "data": "0x" + selector}, "latest"])
    registered, onchain = None, None
    try:
        raw = bytes.fromhex(result[2:])
        word = lambda at: int.from_bytes(raw[at:at + 32], "big")
        onchain = word(32)
        array = word(0)
        count = word(array)
        registered = []
        for index in range(count):
            tuple_at = array + 32 + word(array + 32 + 32 * index)
            registered.append("0x" + raw[tuple_at + 12:tuple_at + 32].hex())
    except (TypeError, ValueError, IndexError, AttributeError):
        registered = None
    if registered is None:
        line(False, "signers getAttestors=%s" % (error or "malformed"))
    else:
        named = [s.lower() for s in signers if s]
        for n, signer in enumerate(signers, 1):
            if signer:
                line(signer.lower() in registered, "signer attestor-%d address=%s member=%s" % (n, signer.lower(), "yes" if signer.lower() in registered else "no"))
        exact = len(named) == 4 and len(set(named)) == 4 and sorted(named) == sorted(registered)
        majority = onchain is not None and len(registered) // 2 < onchain <= len(registered)
        line(exact and majority and threshold is not None and onchain == int(threshold),
             "signers registered=%d named=%d exact=%s threshold=%s approved=%s" % (len(registered), len(set(named)),
             "yes" if exact else "no", onchain, threshold or "absent"))
    minimum_wei = need("XWEB_SUBMITTER_MIN_WEI", r"[1-9][0-9]{0,77}")
    for n in range(1, 5):
        submitter = need("XWEB_ATTESTOR_%d_SUBMITTER" % n, r"0x[0-9a-fA-F]{40}")
        if not submitter:
            continue
        if signers[n - 1] and submitter.lower() == signers[n - 1].lower():
            line(False, "submitter attestor-%d address=%s role=shared-with-signer" % (n, submitter.lower()))
            continue
        result, error = rpc(evm, "eth_getBalance", [submitter, "latest"])
        try:
            wei = int(result, 16)
        except (TypeError, ValueError):
            line(False, "submitter attestor-%d address=%s balance=%s" % (n, submitter.lower(), error or "malformed"))
            continue
        line(minimum_wei is not None and wei >= int(minimum_wei),
             "submitter attestor-%d address=%s balance=%d minimum=%s" % (n, submitter.lower(), wei, minimum_wei or "absent"))
sys.exit(1 if failures else 0)
' "$xweb_gateway" "$xweb_evm" "$prices" ${names[@]+"${names[@]}"}
}

# px_register signs the hosted beta binding; no node root receives identity calls.
xweb_register() {
 xweb_need XWEB_GATEWAY_ENDPOINT
 xweb_need XWEB_REGISTER_KEY_FILE
 "${BRINGUP_INSPECT_BIN:-bringup-inspect}" xweb-config register
}

mode="${1:-}"
case "$mode" in
-h | --help)
	usage
	exit 0
	;;
prices)
	[ "$#" -eq 1 ] || {
		usage >&2
		exit 2
	}
	xweb_prices
	;;
inventory)
	[ "$#" -eq 1 ] || {
		usage >&2
		exit 2
	}
	load_hosts
	xweb_inventory
	;;
register)
	[ "$#" -eq 1 ] || {
		usage >&2
		exit 2
	}
	xweb_register
	;;
render)
	# One renderer per sidecar kind: serve on the RPC nodes, attestor on
	# the four Fly attestor apps.
	case "${2:-}" in
	attestor)
		[ "$#" -eq 3 ] || {
			usage >&2
			exit 2
		}
		if [ "${XWEB_SERVE_NAMES+x}" != x ]; then
			load_hosts
		fi
		xweb_render_attestor "$3"
		;;
	serve)
		[ "$#" -eq 3 ] || {
			usage >&2
			exit 2
		}
		if [ "${XWEB_SERVE_NAMES+x}" != x ]; then
			load_hosts
		fi
		xweb_render_serve "$3"
		;;
	*)
		usage >&2
		exit 2
		;;
	esac
	;;
*)
	usage >&2
	exit 2
	;;
esac

#!/bin/bash
# Runs as root inside the kernel app's machine (services/wallet/deploy/human.toml);
# tools/bringup/check-live.sh kernel-value-loop pipes it to bash -s through
# flyctl ssh console. It proves the value loop of the kernel:
#
#   value-loop.sh [ASSET=PAX|SID|USDC|USDL]
#
# Requires provisioned genesis, an authenticated node RPC and its trust anchor.
# Sender and recipient seeds and the signed transaction journal survive restart.
# Inclusion is read from the canonical block and the node's governed signed
# receipt; only committed evidence is reported as a successful value loop.
set -euo pipefail
umask 077

layerx=${LAYERX_KERNEL_DATA:-/data/layerx}
layerx=${layerx%/}
run=${LAYERX_KERNEL_RUN:-/run/layerx}
run=${run%/}
tls=${LAYERX_KERNEL_TLS:-/data/tls}
tls=${tls%/}
keys=$layerx/keys
genesis=$layerx/genesis
state=$layerx/value-loop
rpc=${LAYERX_NODE_RPC:-}
trust_anchor=${LAYERX_NODE_TRUST_ANCHOR_DER:-}
network_id=${LAYERX_NODE_NETWORK_ID:-125}

precondition() {
	echo "precondition $*"
	exit 3
}

failed() {
	echo "$*"
	exit 1
}

# as_node <command...>: runs the probe as its unprivileged client identity.
as_node() { setpriv --reuid 4021 --regid 4020 --clear-groups -- "$@"; }

seed_public() {
	od -An -tx1 "$1" | tr -d ' \n' | python3 -c '
import sys
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from cryptography.hazmat.primitives.serialization import Encoding, PublicFormat
key = Ed25519PrivateKey.from_private_bytes(bytes.fromhex(sys.stdin.read().strip()))
print(key.public_key().public_bytes(Encoding.Raw, PublicFormat.Raw).hex())'
}

# main_id <did>: sha256("LX:ACCOUNT:v1" || u32 length || "agent:<did>:main").
main_id() {
	python3 -c '
import hashlib, sys
name = ("agent:" + sys.argv[1] + ":main").encode()
print(hashlib.sha256(b"LX:ACCOUNT:v1" + len(name).to_bytes(4, "big") + name).hexdigest())' "$1"
}

handshake_committed() {
	as_node layerx-node-probe handshake --rpc "$rpc" --trust-anchor "$trust_anchor" --network-id "$network_id" 2>/dev/null |
		jq -er '.latest_committed_height'
}

balance() {
	as_node layerx-node-probe balance --rpc "$rpc" --trust-anchor "$trust_anchor" --network-id "$network_id" \
		--account "agent:$1:main" --asset "$asset" 2>/dev/null
}

# Kept inline because check-live.sh transports this script alone to the machine.
# send.json is the authoritative journal; send.activity-id is a compatibility view.
send_journal() {
	python3 - "$state" "$network_id" "$rpc" "$trust_anchor" "$@" <<'PY'
import json
import os
from pathlib import Path
import ssl
import stat
import sys
import tempfile
import urllib.request

root, network, url, anchor, action, *arguments = sys.argv[1:]
root = Path(root)


def keccak256(data):
    # Keccak-f[1600], with Ethereum's 0x01 domain separator (not SHA3-256).
    rotations = (0, 1, 62, 28, 27, 36, 44, 6, 55, 20, 3, 10, 43,
                 25, 39, 41, 45, 15, 21, 8, 18, 2, 61, 56, 14)
    constants = (0x1, 0x8082, 0x800000000000808A, 0x8000000080008000,
                 0x808B, 0x80000001, 0x8000000080008081, 0x8000000000008009,
                 0x8A, 0x88, 0x80008009, 0x8000000A, 0x8000808B,
                 0x800000000000008B, 0x8000000000008089, 0x8000000000008003,
                 0x8000000000008002, 0x8000000000000080, 0x800A,
                 0x800000008000000A, 0x8000000080008081,
                 0x8000000000008080, 0x80000001, 0x8000000080008008)
    mask = (1 << 64) - 1

    def rotate(value, count):
        return ((value << count) | (value >> ((64 - count) % 64))) & mask

    padded = bytearray(data)
    padded.append(1)
    padded.extend(bytes((-len(padded)) % 136))
    padded[-1] |= 0x80
    lanes = [0] * 25
    for start in range(0, len(padded), 136):
        for i in range(17):
            lanes[i] ^= int.from_bytes(padded[start + i * 8:start + i * 8 + 8], "little")
        for constant in constants:
            columns = [lanes[x] ^ lanes[x + 5] ^ lanes[x + 10] ^
                       lanes[x + 15] ^ lanes[x + 20] for x in range(5)]
            for x in range(5):
                delta = columns[(x - 1) % 5] ^ rotate(columns[(x + 1) % 5], 1)
                for y in range(5):
                    lanes[x + 5 * y] ^= delta
            moved = [0] * 25
            for x in range(5):
                for y in range(5):
                    moved[y + 5 * ((2 * x + 3 * y) % 5)] = rotate(
                        lanes[x + 5 * y], rotations[x + 5 * y])
            for x in range(5):
                for y in range(5):
                    lanes[x + 5 * y] = moved[x + 5 * y] ^ (
                        ~moved[(x + 1) % 5 + 5 * y] & moved[(x + 2) % 5 + 5 * y])
            lanes[0] ^= constant
    return b"".join(lane.to_bytes(8, "little") for lane in lanes)[:32].hex()


def rlp(data, offset=0, depth=0):
    if offset >= len(data) or depth > 5:
        raise ValueError("invalid transaction RLP")
    prefix = data[offset]
    if prefix < 128:
        return data[offset:offset + 1], offset + 1
    listed = prefix >= 192
    base = 192 if listed else 128
    size = prefix - base
    start = offset + 1
    if size > 55:
        width = size - 55
        size_bytes = data[start:start + width]
        if len(size_bytes) != width or size_bytes[0] == 0:
            raise ValueError("invalid RLP length")
        size = int.from_bytes(size_bytes, "big")
        start += width
        if size < 56:
            raise ValueError("noncanonical RLP length")
    end = start + size
    if end > len(data):
        raise ValueError("truncated signed transaction")
    if not listed:
        value = data[start:end]
        if size == 1 and value[0] < 128:
            raise ValueError("noncanonical RLP string")
        return value, end
    values = []
    while start < end:
        value, start = rlp(data, start, depth + 1)
        values.append(value)
    if start != end:
        raise ValueError("invalid RLP list")
    return values, end


def integer(value, bits):
    if not isinstance(value, bytes) or len(value) > (bits + 7) // 8 or value[:1] == b"\0":
        raise ValueError("invalid transaction integer")
    number = int.from_bytes(value, "big")
    if number >= 1 << bits:
        raise ValueError("transaction integer out of range")
    return number


def sync_directory(path):
    descriptor = os.open(path, os.O_RDONLY | os.O_DIRECTORY)
    try:
        os.fsync(descriptor)
    finally:
        os.close(descriptor)


def atomic(name, data):
    descriptor, temporary = tempfile.mkstemp(prefix=".send-", dir=root)
    try:
        with os.fdopen(descriptor, "wb") as output:
            output.write(data)
            output.flush()
            os.fsync(output.fileno())
        os.replace(temporary, root / name)
        sync_directory(root)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def load():
    descriptor = os.open(root / "send.bin", os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    with os.fdopen(descriptor, "rb") as source:
        metadata = os.fstat(source.fileno())
        if not stat.S_ISREG(metadata.st_mode) or not 0 < metadata.st_size <= 1048576:
            raise ValueError("signed transaction must be a bounded regular file")
        raw = source.read(1048577)
        if len(raw) != metadata.st_size or raw[0] != 2:
            raise ValueError("expected immutable signed EIP-1559 transaction")
        os.fsync(source.fileno())
    fields, end = rlp(raw, 1)
    if end != len(raw) or not isinstance(fields, list) or len(fields) != 12:
        raise ValueError("invalid signed EIP-1559 fields")
    chain = integer(fields[0], 256)
    nonce = integer(fields[1], 64)
    for index in (2, 3, 6, 10, 11):
        integer(fields[index], 256)
    integer(fields[4], 64)
    if (chain != int(network) or integer(fields[9], 1) > 1 or
            not fields[10] or not fields[11] or
            not isinstance(fields[5], bytes) or len(fields[5]) not in (0, 20) or
            not isinstance(fields[7], bytes) or not isinstance(fields[8], list)):
        raise ValueError("signed transaction chain or shape mismatch")
    digest = keccak256(raw)
    identity = dict(transaction="0x" + raw.hex(), hash="0x" + digest,
                    nonce=str(nonce), chain_id=str(chain))
    journal = root / "send.json"
    if journal.exists():
        saved = json.loads(journal.read_bytes())
        if (any(saved.get(key) != value for key, value in identity.items()) or
                saved.get("status") not in ("prepared", "submitted")):
            raise ValueError("saved transaction identity or status changed")
    else:
        saved = dict(identity, status="prepared")
    legacy = root / "send.activity-id"
    if legacy.exists() and legacy.read_text().strip().removeprefix("0x") != digest:
        raise ValueError("retained activity identity differs from transaction hash")
    sync_directory(root)
    sync_directory(root.parent)
    return saved


def save(saved):
    atomic("send.json", (json.dumps(saved, sort_keys=True) + "\n").encode())
    atomic("send.activity-id", (saved["hash"][2:] + "\n").encode())


def rpc(method, params):
    context = None
    if url.startswith("https://"):
        context = ssl.create_default_context(cadata=ssl.DER_cert_to_PEM_cert(Path(anchor).read_bytes()))
    request = urllib.request.Request(url, data=json.dumps(dict(
        jsonrpc="2.0", id=1, method=method, params=params)).encode(),
        headers={"Content-Type": "application/json"})
    with urllib.request.urlopen(request, context=context, timeout=15) as response:
        encoded = response.read(1048577)
    if len(encoded) > 1048576:
        raise ValueError("RPC response exceeds bound")
    answer = json.loads(encoded)
    if answer.get("id") != 1 or answer.get("jsonrpc") != "2.0" or "error" in answer or "result" not in answer:
        code = answer.get("error", {}).get("code")
        raise ValueError("transaction reconciliation RPC failed: method=" + method + " code=" + str(code))
    return answer["result"]


try:
    saved = load()
    if action == "prepare":
        if arguments and arguments[0].removeprefix("0x") != saved["hash"][2:]:
            raise ValueError("producer identity differs from signed transaction hash")
        save(saved)
    elif action == "reconcile":
        # Save even when recovering a producer output whose stdout was lost.
        save(saved)
        if int(rpc("eth_chainId", []), 16) != int(network):
            raise ValueError("reconciliation RPC chain mismatch")
        existing = rpc("eth_getTransactionByHash", [saved["hash"]])
        if existing is not None:
            if (existing.get("hash") != saved["hash"] or
                    int(existing["nonce"], 16) != int(saved["nonce"])):
                raise ValueError("reconciliation transaction identity mismatch")
        else:
            # A lost acknowledgement can only replay these exact signed bytes.
            result = rpc("eth_sendRawTransaction", [saved["transaction"]])
            if result != saved["hash"]:
                raise ValueError("submission transaction hash mismatch")
        saved["status"] = "submitted"
        save(saved)
    else:
        raise ValueError("unknown journal operation")
    print(saved["hash"][2:])
except Exception as error:
    print("value-loop journal: " + str(error), file=sys.stderr)
    sys.exit(1)
PY
}

send_transfer() {
	local activity="" send_command=write-send
	local value_loop_lock
	exec {value_loop_lock}>"$state/send.lock"
	flock -n "$value_loop_lock" || failed "activity another value loop holds the journal lock"
	# Never regenerate a retained or partially written transaction. A partial or
	# contradictory journal requires operator reconciliation, not a new payment.
	if [ ! -e "$state/send.bin" ]; then
		[ ! -e "$state/send.json" ] && [ ! -e "$state/send.activity-id" ] ||
			failed "activity retained identity has no signed transaction; reconcile manually"
		if [ "$explicit_asset" = 1 ]; then
			send_command=write-asset-send
		fi
		activity=$(as_node layerx-node-probe "$send_command" --rpc "$rpc" --trust-anchor "$trust_anchor" --network-id "$network_id" \
			--seed-file "$keys/value-loop/sender.key" --destination-did "${dids[recipient]}" \
			--asset "$asset" --output "$state/send.bin" 2>/dev/null) || failed "activity write-send refused"
		send_journal prepare "$activity" >/dev/null || failed "activity journal prepare failed"
	else
		send_journal prepare >/dev/null || failed "activity journal recovery failed"
	fi
	send_journal reconcile || failed "activity reconciliation failed; retained transaction must be retried"
}

main() {
	local argument file role seed public did id
	local answer activity committed_height batch deadline inclusion_height
	local symbol=PAX explicit_asset=0
	for argument in "$@"; do
		case "$argument" in
		ASSET=PAX|ASSET=SID|ASSET=USDC|ASSET=USDL) symbol=${argument#ASSET=}; explicit_asset=1 ;;
		*) failed "usage value-loop.sh [ASSET=PAX|SID|USDC|USDL]" ;;
		esac
	done
	[ "$(id -u)" = 0 ] || failed "usage run as root inside the kernel machine"

	for file in "$genesis/asset-id"; do
		[ -s "$file" ] || precondition genesis-ids "$file absent; run kernel-genesis.sh genesis (step C) first"
	done
	if [ "$explicit_asset" = 1 ]; then
		state="$layerx/value-loop/assets/$symbol"
	fi

	[ -n "$rpc" ] || precondition custody-rpc "LAYERX_NODE_RPC must be set"
	[ -n "$trust_anchor" ] || precondition custody-trust-anchor "LAYERX_NODE_TRUST_ANCHOR_DER must be set"
	[ -s "$trust_anchor" ] || precondition custody-trust-anchor "trust anchor is absent"
	for tool in layerx-node-probe "${BRINGUP_INSPECT_BIN:-bringup-inspect}"; do
		command -v "$tool" >/dev/null || precondition image "$tool is not in the kernel image"
	done
	# Both the probe and inspector must authenticate this same RPC origin.
	SSL_CERT_FILE=$(mktemp)
	openssl x509 -inform DER -in "$trust_anchor" -out "$SSL_CERT_FILE" || failed "trust anchor is invalid"
	trap 'rm -f "$SSL_CERT_FILE"' EXIT
	handshake_committed >/dev/null || precondition kernel-node "committed RPC handshake refused"

	asset=$(printf 'layerx-asset:125:%s' "$symbol" | sha256sum | cut -d' ' -f1)
	if [ "$symbol" = PAX ]; then
		[ "$(tr -d ' \r\n' <"$genesis/asset-id")" = "$asset" ] || failed "asset PAX genesis id differs from $asset"
	fi
	echo "asset $symbol id=$asset"

	install -d -o 0 -g 4020 -m 0750 "$keys/value-loop"
	if [ "$explicit_asset" = 1 ]; then
		install -d -o 4021 -g 4020 -m 0700 "$layerx/value-loop" "$layerx/value-loop/assets"
	fi
	install -d -o 4021 -g 4020 -m 0700 "$state"
	declare -gA dids=() mains=()
	for role in sender recipient; do
		seed=$keys/value-loop/$role.key
		if [ ! -s "$seed" ]; then
			install -o 4021 -g 4020 -m 0400 /dev/null "$seed.new"
			openssl rand 32 >"$seed.new"
			mv "$seed.new" "$seed"
		fi
		public=$(seed_public "$seed")
		did=did:layerx:$public
		id=$(main_id "$did")
		dids[$role]=$did
		mains[$role]=$id
		echo "account $role did=$did main=$id"
	done
	answer=$(balance "${dids[sender]}") || failed "balance sender read failed"
	if jq -e '.refused.result == -208' <<<"$answer" >/dev/null 2>&1; then
		precondition sender-balance "the sender account ${mains[sender]} must be funded before running the value loop"
	fi

	activity=$(send_transfer) || failed "activity transfer recovery failed"
	echo "activity id=$activity"
	deadline=$(($(date +%s) + 120))
	while :; do
		answer=$("${BRINGUP_INSPECT_BIN:-bringup-inspect}" committed-transaction "$rpc" "0x$activity" "$SSL_CERT_FILE" 2>/dev/null) && break
		[ "$(date +%s)" -lt "$deadline" ] || failed "activity id=$activity committed=never"
		sleep 1
	done
	batch=$(jq -er '.block' <<<"$answer") || failed "committed block unreadable"
	inclusion_height=$(jq -er '.height' <<<"$answer") || failed "committed height unreadable"

	for role in sender recipient; do
		answer=$(balance "${dids[$role]}") || failed "balance $role committed read failed"
		jq -e --arg account "agent:${dids[$role]}:main" --arg asset "$asset" --argjson minimum "$((inclusion_height))" \
			'.account == $account and .asset == $asset and .achieved == "committed" and .global_sequence >= $minimum and (.balance | type == "string") and (.balance | test("^(0|[1-9][0-9]*)$"))' \
			<<<"$answer" >/dev/null || failed "balance $role committed evidence refused"
		echo "balance $role $(jq -c '{balance, achieved, global_sequence}' <<<"$answer")"
	done

	committed_height=$(handshake_committed) || failed "batch id=$batch committed_height=unreadable"
	[ "$committed_height" -ge "$((inclusion_height))" ] || failed "committed handshake precedes inclusion"
	echo "batch id=$batch committed_height=$committed_height"
}

if [ "${BASH_SOURCE[0]:-$0}" = "$0" ]; then
	main "$@" </dev/null
	exit
fi

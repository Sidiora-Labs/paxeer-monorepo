#!/bin/bash
# Runs as root inside the kernel app's machine (services/wallet/deploy/human.toml)
# through flyctl ssh console, after docker/kernel/init.sh has made the volume
# layout and the treasury key. The kernel genesis is made in two steps around
# the governance step on chain 125:
#
#   kernel-genesis.sh keys [rotate]
#       step A: generates the sequencer key, the deposit authority key and the
#       publication recipient key under
#       /data/layerx/keys and prints their public values. Submit a layerxcustody
#       governance proposal through the chain's governance module carrying
#       exactly the sequencer authorization, the deposit root authority (the
#       deposit authority public key) and the custody asset map. The proposal
#       must pass before step C. The publication recipient key is a kernel
#       input, not a proposal input.
#       rotate first runs the identity preflight of kernel-init
#       --identity-generation and retires the kernel keys, the genesis outputs,
#       the registry material and the node data into
#       /data/layerx/identity/generations/<generation>; an interrupted rotate
#       resumes when run again.
#   kernel-genesis.sh plan
#       the compatibility plan of the persisted trust history, receipt-authority
#       replica bindings and Human role material against the current genesis,
#       and its plan_sha256.
#   kernel-genesis.sh migrate PLAN_SHA256
#       the migration that plan authorizes: the old bindings are retained beside
#       the retired generation, the role material is rebound to the new genesis
#       and the new identity generation is recorded; durable Human state is
#       never touched.
#   kernel-genesis.sh genesis SID=POINTER USDC=POINTER USDL=POINTER [SYMBOL=POINTER...]
#       step C, once governance has set the custody asset map and the deposit
#       root authority: reads PAX from nativeAssetId() and every pointer from
#       assetByPointer() through the pod's paxeer relay, writes the LXGB v2
#       genesis metadata with one canonical Asset record per mapped asset, the
#       treasury recipient-binding policy, and the kernel asset id (the PAX
#       record) and replica id the init reads. The init then runs bootstrap.sh,
#       whose layerx-genesis-build signs the manifest under /data/layerx/node; this
#       step waits for it and prints the digests and the asset ids.
#       Precondition: every registered asset id is the derivation
#       sha256("layerx-asset:125:<SYMBOL>"); the step B proposer chooses the id
#       and this step refuses a registered id that differs from it.
#   kernel-genesis.sh render ENV_FILE OUTPUT_DIR
#       renders the beta kernel genesis from the deploy inputs of ENV_FILE and
#       runs layerx-genesis-build on it into OUTPUT_DIR/genesis; runs anywhere,
#       not only inside the kernel machine. ENV_FILE holds KEY=VALUE lines:
#         LAYERX_GENESIS_NETWORK_ID           decimal kernel network id
#         LAYERX_GENESIS_SEQUENCER_PUBLIC_KEY 64 hex, the ed25519 sequencer key
#         LAYERX_GENESIS_SEQUENCER_KEY_FILE   its seed (64 hex or 32 bytes), signs the genesis
#         LAYERX_GENESIS_TREASURY_PUBLIC_KEY  64 hex, the issuer of the Asset records
#         LAYERX_GENESIS_ASSETS               SYMBOL:ID:DECIMALS ..., PAX first (the kernel asset),
#                                             ID = sha256("layerx-asset:125:<SYMBOL>")
#         LAYERX_GENESIS_MODULES              escrow budget stream service perps spot web, or a subset
#         LAYERX_GENESIS_TIMESTAMP_MS         optional, default now
#       The binary is LAYERX_GENESIS_BUILD, else build/bin/layerx-genesis-build
#       of this repository, else /usr/local/bin/layerx-genesis-build.
#
# Nothing secret is printed. rotate retires the kernel keys, the genesis
# outputs and the node data; nothing is deleted.
set -euo pipefail
umask 077

layerx=/data/layerx
node_data=$layerx/node
keys=$layerx/keys
genesis=$layerx/genesis
[ "${1:-}" = render ] || network_id=${LAYERX_NODE_NETWORK_ID:?the kernel network id comes from the app env}
rpc=http://127.0.0.1:${LAYERX_NODE_PAXEER_RELAY_PORT:-18545}
custody=0x0000000000000000000000000000000000001013
deposit_key=$keys/checkpoint-submitter/deposit-authority.pem
key_files=("$keys/sequencer.key" "$deposit_key" "$keys/publication/recipient.key")
genesis_files=("$keys/publication/binding-policy.json"
	"$genesis/metadata.lxgb" "$genesis/custody-assets.json" "$genesis/asset-id" "$genesis/replica-id")
kernel_init=/usr/local/bin/kernel-init
retire_files=("${key_files[@]}" "$keys/checkpoint-authority/key.pem" "$keys/checkpoint-authority/key.pem.lock" "${genesis_files[@]}"
	"$layerx/registry-material")
rotation=$layerx/identity/rotation.json

fail() {
	printf 'kernel-genesis: %s\n' "$*" >&2
	exit 1
}

if [ "${1:-}" != render ]; then
	[ "$(id -u)" = 0 ] || fail "run as root inside the kernel machine"
	[ -s "$keys/treasury.key" ] || fail "the init has not made $keys/treasury.key yet"
fi

hex_public() {
	python3 -c 'import sys; sys.stdout.buffer.write(bytes.fromhex("302e020100300506032b657004220420" + sys.argv[1]))' "$1" |
		openssl pkey -inform DER -pubout -outform DER | tail -c 32 | od -An -tx1 | tr -d ' \n'
}
pem_public() { openssl pkey -in "$1" -pubout -outform DER | tail -c 32 | od -An -tx1 | tr -d ' \n'; }
recipient_address() { python3 /opt/layerx/paxeer/evm.py address "$keys/publication/recipient.key" | tr 'A-F' 'a-f'; }

identity() { "$kernel_init" --identity-generation "$@"; }
rotation_pending() {
	[ ! -e "$rotation" ] || fail "an identity rotation is pending at $rotation; run kernel-genesis.sh keys rotate to resume it"
}

[ "${1:-}" = render ] || treasury_public=$(hex_public "$(tr -d ' \r\n' <"$keys/treasury.key")")

# public_values: the step A outputs, read back from the key files.
public_values() {
	sequencer_public=$(hex_public "$(cat "$keys/sequencer.key")")
	replica_id=$(printf 'layerx-authority-replica:%s' "$sequencer_public" | sha256sum | cut -d' ' -f1)
	recipient=$(recipient_address)
	recipient=${recipient#0x}
}

keys_step() {
	local file key
	if [ "${1:-}" = rotate ]; then
		identity retire "${retire_files[@]}" || fail "identity rotation refused; see kernel-genesis.sh plan"
	elif [ -n "${1:-}" ]; then
		fail "usage: kernel-genesis.sh keys [rotate]"
	else
		rotation_pending
		for file in "${key_files[@]}"; do
			[ ! -e "$file" ] || fail "$file exists; pass rotate to replace the kernel keys"
		done
	fi
	# The sequencer seed, 64 hex characters as bootstrap.sh reads it, owned by the node user.
	install -o 4020 -g 4020 -m 0600 /dev/null "$keys/sequencer.key"
	openssl rand -hex 32 | tr -d '\n' >"$keys/sequencer.key"
	install -o 4021 -g 4020 -m 0600 /dev/null "$deposit_key"
	openssl genpkey -algorithm ED25519 >"$deposit_key"
	# The publication recipient: an EVM key of the pod's evm.py.
	while :; do
		key="0x$(openssl rand -hex 32)"
		python3 /opt/layerx/paxeer/evm.py address /dev/stdin <<<"$key" >/dev/null 2>&1 && break
	done
	printf '%s' "$key" >"$keys/publication/recipient.key"
	unset key
	identity rotated "${key_files[@]}" || fail "identity rotation record refused"
	public_values
	echo "network_id=$network_id"
	echo "sequencer_public_key=$sequencer_public"
	echo "sequencer_id=$(printf 'layerx-sequencer:%s' "$sequencer_public" | sha256sum | cut -d' ' -f1)"
	echo "replica_id=$replica_id"
	echo "treasury_public_key=$treasury_public"
	echo "deposit_authority_public_key=$(pem_public "$deposit_key")"
	echo "publication_recipient=0x$recipient"
	echo "custody keys=$keys next=submit a layerxcustody governance proposal through the chain's governance module carrying exactly the sequencer authorization, the deposit root authority (the deposit authority public key) and the custody asset map; after the proposal passes and governance is enacted run kernel-genesis.sh genesis (step C)"
}

# call <data>: the eth_call answer of the custody precompile, or of $to.
call() {
	curl -fsS -m 15 "$rpc" -H 'content-type: application/json' \
		-d "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"eth_call\",\"params\":[{\"to\":\"${to:-$custody}\",\"data\":\"$1\"},\"latest\"]}" |
		jq -er '.result'
}

word() { printf '%064s' "${1#0x}" | tr ' ' 0; }
# erc20 <pointer> <selector>: the decoded symbol (95d89b41) or decimals (313ce567).
erc20() {
	to=$1 call "0x$2" | python3 -c '
import re
import sys
encoded = sys.stdin.read().strip()
data = bytes.fromhex(encoded[2:])
if sys.argv[1] == "313ce567":
    print(int.from_bytes(data[:32], "big"))
else:
    if re.fullmatch(r"0x(?:[0-9a-fA-F]{2})+", encoded) is None:
        raise SystemExit("symbol() returned invalid hex data")
    if len(data) == 32:
        symbol = data.rstrip(b"\0")
    else:
        if len(data) < 96 or int.from_bytes(data[:32], "big") != 32:
            raise SystemExit("symbol() returned an invalid ABI offset")
        size = int.from_bytes(data[32:64], "big")
        padded_size = ((size + 31) // 32) * 32
        if len(data) != 64 + padded_size or any(data[64 + size:]):
            raise SystemExit("symbol() returned invalid ABI length or padding")
        symbol = data[64:64 + size]
    if not 0 < len(symbol) <= 16 or any(byte < 33 or byte > 126 for byte in symbol):
        raise SystemExit("symbol() must contain 1 to 16 printable ASCII bytes")
    print(symbol.decode("ascii"))
' "$2"
}

# asset_id <SYMBOL>: the asset id step B registers for the symbol,
# sha256("layerx-asset:125:<SYMBOL>").
asset_id() {
	printf 'layerx-asset:125:%s' "$1" | sha256sum | cut -d' ' -f1
}

# genesis_metadata <output> <issuer-public> <asset:symbol:decimals>...: the LXGB
# v2 metadata, the record layout bootstrap.sh writes for one asset, one record
# per mapped asset, issued by the treasury identity, then the zero fee schedule
# that bootstrap's --withdrawal-fee and --module-fees complete.
genesis_metadata() {
	python3 - "$@" <<'PY'
import hashlib
import os
import sys

output, issuer_public = sys.argv[1], bytes.fromhex(sys.argv[2])
did = ('did:layerx:' + issuer_public.hex()).encode()
issuer = hashlib.sha256(b'LXP/v1/did-id\0' + len(did).to_bytes(2, 'big') + did).digest()
body = b''
for entry in sys.argv[3:]:
    asset, symbol, decimals = entry.split(':')
    asset, symbol, decimals = bytes.fromhex(asset), symbol.encode('ascii'), int(decimals)
    if not 0 < len(symbol) <= 16 or not 0 <= decimals <= 38:
        raise SystemExit('asset symbol must be 1 to 16 ASCII bytes and decimals 0..38')
    reference = bytes(12) + asset[12:]
    record = (b'\0\x03' + asset + len(symbol).to_bytes(1, 'big') + symbol + decimals.to_bytes(1, 'big')
              + b'\x02' + len(reference).to_bytes(2, 'big') + reference
              + b'\0\x0dCustody token' + bytes(16) + issuer + b'\x02' + bytes(16) + os.urandom(32))
    body += len(record).to_bytes(2, 'big') + record
schedule = b'\0\x02' + bytes(80) + (10000).to_bytes(4, 'big') + b'\x0a' + bytes(160)
metadata = (len(sys.argv) - 3).to_bytes(2, 'big') + body + len(schedule).to_bytes(2, 'big') + schedule
with os.fdopen(os.open(output, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600), 'wb') as handle:
    handle.write(metadata)
    handle.flush()
    os.fsync(handle.fileno())
PY
}

# genesis_ids <asset-id> <replica-id>: the ids the init hands layerxd; written
# last, so the init starts on a complete set.
genesis_ids() {
	printf '%s\n' "$1" >"$genesis/asset-id"
	printf '%s\n' "$2" >"$genesis/replica-id"
	chown 0:0 "$genesis/asset-id" "$genesis/replica-id"
	chmod 0444 "$genesis/asset-id" "$genesis/replica-id"
}

genesis_step() {
	local argument file symbol pointer id onchain decimals pax manifest record
	local -A pointers=()
	local -a records=() approved_records=()
	rotation_pending
	for argument in "$@"; do
		[[ $argument =~ ^([A-Z0-9]{1,16})=(0x[0-9a-fA-F]{40})$ ]] || fail "not SYMBOL=POINTER: $argument"
		symbol=${BASH_REMATCH[1]}
		[ "$symbol" != PAX ] || fail "PAX uses the native custody mapping, not a token pointer"
		[ -z "${pointers[$symbol]:-}" ] || fail "duplicate asset pointer input for $symbol"
		pointers[$symbol]=${BASH_REMATCH[2],,}
	done
	for symbol in SID USDC USDL; do
		[ -n "${pointers[$symbol]:-}" ] || fail "the pointer of $symbol is required"
	done
	for file in "${key_files[@]}"; do
		[ -s "$file" ] || fail "$file is absent; run kernel-genesis.sh keys first"
	done
	for file in "${genesis_files[@]}"; do
		[ ! -e "$file" ] || fail "$file exists; run kernel-genesis.sh keys rotate to replace the kernel genesis"
	done
	[ "$(curl -fsS -m 15 "$rpc" -H 'content-type: application/json' \
		-d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}' | jq -r .result)" = 0x7d ] ||
		fail "the paxeer relay at $rpc does not answer chain 125"
	id=$(call 0xb4768600) || fail "eth_call depositRootAuthority() failed"
	[ "${id#0x}" = "$(pem_public "$deposit_key")" ] ||
		fail "the deposit root authority on chain 125 is not this deposit authority: eth_call $custody depositRootAuthority() answered $id"

	pax=$(call 0xaafcde84) || fail "eth_call nativeAssetId() failed"
	pax=${pax#0x}
	if ! [[ $pax =~ ^[0-9a-f]{64}$ ]] || [ "$pax" = "$(word 0)" ]; then
		fail "custody asset map has no PAX record: eth_call $custody nativeAssetId() answered 0x$pax"
	fi
	[ "$pax" = "$(asset_id PAX)" ] ||
		fail "the PAX asset id 0x$pax is not sha256(\"layerx-asset:125:PAX\") 0x$(asset_id PAX)"
	records+=("$pax:PAX:6")
	approved_records+=("$pax:PAX:6:0x0000000000000000000000000000000000000000")

	for symbol in "${!pointers[@]}"; do
		pointer=${pointers[$symbol]}
		id=$(call "0xca65021c$(word "$pointer")") || fail "eth_call assetByPointer($pointer) failed"
		id=${id#0x}
		[ "$id" != "$(word 0)" ] ||
			fail "custody asset map has no $symbol record: eth_call $custody assetByPointer($pointer) answered 0x$id"
		[ "$id" = "$(asset_id "$symbol")" ] ||
			fail "the $symbol asset id 0x$id is not sha256(\"layerx-asset:125:$symbol\") 0x$(asset_id "$symbol")"
		onchain=$(erc20 "$pointer" 95d89b41) || fail "symbol() of $pointer failed"
		[ "$onchain" = "$symbol" ] || fail "$pointer is $onchain, not $symbol"
		decimals=$(erc20 "$pointer" 313ce567) || fail "decimals() of $pointer failed"
		records+=("$id:$symbol:$decimals")
		case "$symbol" in
		SID | USDC | USDL) approved_records+=("$id:$symbol:$decimals:$pointer") ;;
		esac
	done
	python3 - "$genesis/custody-assets.json" "${approved_records[@]}" <<'PYMETA'
import json
import os
import sys

output = sys.argv[1]
entries = {}
for record in sys.argv[2:]:
    asset, symbol, decimals, pointer = record.split(':')
    if symbol in entries:
        raise SystemExit('duplicate approved custody asset')
    entries[symbol] = dict(symbol=symbol, asset_id=asset, token_pointer=pointer, decimals=int(decimals))
if set(entries) != {'PAX', 'SID', 'USDC', 'USDL'}:
    raise SystemExit('all four approved custody assets are required')
with os.fdopen(os.open(output, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600), 'w') as handle:
    json.dump(dict(assets=[entries[symbol] for symbol in ('PAX', 'SID', 'USDC', 'USDL')]), handle)
    handle.write('\n')
    handle.flush()
    os.fsync(handle.fileno())
PYMETA
	chown 4020:4020 "$genesis/custody-assets.json"
	chmod 0444 "$genesis/custody-assets.json"
	public_values

	# The treasury signer's recipient-binding policy.
	printf '{"asset_id": "%s", "network_id": %s, "recipient": "%s", "version": 1}\n' "$pax" "$network_id" "$recipient" \
		>"$keys/publication/binding-policy.json"
	genesis_metadata "$genesis/metadata.lxgb" "$treasury_public" "${records[@]}"
	chown 4020:4020 "$genesis/metadata.lxgb"
	genesis_ids "$pax" "$replica_id"

	manifest=$node_data/genesis/genesis.manifest
	for _ in $(seq 120); do
		[ -s "$manifest" ] && break
		sleep 5
	done
	[ -s "$manifest" ] || fail "layerxd bootstrap wrote no $manifest within ten minutes; see /run/layerx/init/layerxd"

	echo "network_id=$network_id"
	echo "genesis_sha256=$(sha256sum "$manifest" | cut -d' ' -f1)"
	echo "metadata_sha256=$(sha256sum "$genesis/metadata.lxgb" | cut -d' ' -f1)"
	echo "sequencer_public_key=$sequencer_public"
	echo "replica_id=$replica_id"
	echo "publication_recipient=0x$recipient"
	for record in "${records[@]}"; do
		IFS=: read -r id symbol decimals <<<"$record"
		echo "asset $symbol id=$id decimals=$decimals"
	done
	echo "custody node=$node_data keys=$keys genesis=$genesis/metadata.lxgb digests=genesis_sha256,metadata_sha256"
}

render_step() {
	[ "$#" = 2 ] || fail "usage: kernel-genesis.sh render ENV_FILE OUTPUT_DIR"
	local env_file=$1 output=$2 line key value module entry id public symbol decimals seed build rc
	local network sequencer_public treasury timestamp manifest
	local -A input=() seen=()
	local -a modules=() records=()
	[ -f "$env_file" ] && [ ! -L "$env_file" ] && [ -r "$env_file" ] || fail "the genesis env file must be a readable regular file: $env_file"
	while IFS= read -r line || [ -n "$line" ]; do
		[[ -z $line || $line = '#'* ]] && continue
		[[ $line =~ ^(LAYERX_GENESIS_[A-Z_]+)=(.*)$ ]] || fail "genesis env line is not KEY=VALUE: ${line%%=*}"
		key=${BASH_REMATCH[1]} value=${BASH_REMATCH[2]}
		case "$key" in
		LAYERX_GENESIS_NETWORK_ID | LAYERX_GENESIS_SEQUENCER_PUBLIC_KEY | LAYERX_GENESIS_SEQUENCER_KEY_FILE | \
			LAYERX_GENESIS_TREASURY_PUBLIC_KEY | LAYERX_GENESIS_ASSETS | \
			LAYERX_GENESIS_MODULES | LAYERX_GENESIS_TIMESTAMP_MS) ;;
		*) fail "the genesis env file carries an unexpected key $key" ;;
		esac
		[ -z "${input[$key]+set}" ] || fail "$key repeated"
		value=${value#\"} value=${value%\"}
		input[$key]=$value
	done <"$env_file"
	for key in NETWORK_ID SEQUENCER_PUBLIC_KEY SEQUENCER_KEY_FILE TREASURY_PUBLIC_KEY ASSETS MODULES; do
		[ -n "${input[LAYERX_GENESIS_$key]:-}" ] || fail "LAYERX_GENESIS_$key is required"
	done

	network=${input[LAYERX_GENESIS_NETWORK_ID]}
	[[ $network =~ ^[1-9][0-9]{0,9}$ ]] && [ "$network" -le 4294967295 ] || fail "LAYERX_GENESIS_NETWORK_ID must be decimal 1..4294967295"
	timestamp=${input[LAYERX_GENESIS_TIMESTAMP_MS]:-$(date +%s%3N)}
	[[ $timestamp =~ ^[1-9][0-9]{0,17}$ ]] || fail "LAYERX_GENESIS_TIMESTAMP_MS must be decimal milliseconds"

	sequencer_public=${input[LAYERX_GENESIS_SEQUENCER_PUBLIC_KEY],,}
	[[ $sequencer_public =~ ^[0-9a-f]{64}$ ]] || fail "LAYERX_GENESIS_SEQUENCER_PUBLIC_KEY must be 64 hex characters"
	key=${input[LAYERX_GENESIS_SEQUENCER_KEY_FILE]}
	[ -f "$key" ] && [ -r "$key" ] || fail "LAYERX_GENESIS_SEQUENCER_KEY_FILE must name a readable regular file"
	if [ "$(stat -c %s "$key")" = 32 ]; then
		seed=$(od -An -v -tx1 "$key" | tr -d ' \n')
	else
		seed=$(tr -d ' \r\n' <"$key")
		seed=${seed,,}
	fi
	[[ $seed =~ ^[0-9a-f]{64}$ ]] || fail "the sequencer key file is neither 32 raw bytes nor 64 hex characters"
	[ "$(hex_public "$seed")" = "$sequencer_public" ] ||
		fail "LAYERX_GENESIS_SEQUENCER_KEY_FILE does not hold the key of LAYERX_GENESIS_SEQUENCER_PUBLIC_KEY"

	treasury=${input[LAYERX_GENESIS_TREASURY_PUBLIC_KEY],,}
	[[ $treasury =~ ^[0-9a-f]{64}$ ]] || fail "LAYERX_GENESIS_TREASURY_PUBLIC_KEY must be 64 hex characters"
	[ "$treasury" != "$sequencer_public" ] || fail "the sequencer and treasury keys must differ"

	seen=()
	for entry in ${input[LAYERX_GENESIS_ASSETS]}; do
		[[ $entry =~ ^([A-Z0-9]{1,16}):([0-9a-fA-F]{64}):([0-9]{1,2})$ ]] || fail "not an asset SYMBOL:ID:DECIMALS: $entry"
		symbol=${BASH_REMATCH[1]} id=${BASH_REMATCH[2],,} decimals=${BASH_REMATCH[3]}
		[ -z "${seen[$symbol]:-}" ] || fail "asset $symbol repeated"
		seen[$symbol]=1
		[ "$id" = "$(asset_id "$symbol")" ] ||
			fail "the $symbol asset id $id is not sha256(\"layerx-asset:125:$symbol\") $(asset_id "$symbol")"
		[ "${#records[@]}" -gt 0 ] || [ "$symbol" = PAX ] || fail "LAYERX_GENESIS_ASSETS must list PAX first, the kernel asset"
		records+=("$id:$symbol:$decimals")
	done
	[ "${#records[@]}" -gt 0 ] || fail "LAYERX_GENESIS_ASSETS names no asset"

	seen=()
	for module in ${input[LAYERX_GENESIS_MODULES]}; do
		case "$module" in
		escrow | budget | stream | service | perps | spot | web) ;;
		*) fail "LAYERX_GENESIS_MODULES admits escrow, budget, stream, service, perps, spot and web, not $module" ;;
		esac
		[ -z "${seen[$module]:-}" ] || fail "module $module repeated"
		seen[$module]=1
		modules+=("$module")
	done
	[ "${#modules[@]}" -gt 0 ] || fail "LAYERX_GENESIS_MODULES names no module"
	mapfile -t modules < <(printf '%s\n' "${modules[@]}" | LC_ALL=C sort)

	build=${LAYERX_GENESIS_BUILD:-}
	if [ -z "$build" ]; then
		for build in "$(cd "$(dirname "$0")/../.." && pwd)/build/bin/layerx-genesis-build" /usr/local/bin/layerx-genesis-build; do
			[ ! -x "$build" ] || break
		done
	fi
	[ -x "$build" ] || fail "layerx-genesis-build not found; build it with make layerx-genesis-build or set LAYERX_GENESIS_BUILD"

	[ ! -e "$output" ] || fail "$output exists; the genesis is rendered into a new directory"
	mkdir -m 0700 "$output"
	genesis_metadata "$output/metadata.lxgb" "$treasury" "${records[@]}"
	python3 - "$output/genesis-request.lxgb" "$network" "$timestamp" "${records[0]%%:*}" "$output/metadata.lxgb" \
		"${#modules[@]}" "${modules[@]}" <<'PY'
import os
import sys

output, network, timestamp, asset, metadata_path, count = sys.argv[1:7]
modules = sys.argv[7:7 + int(count)]
with open(metadata_path, 'rb') as handle:
    metadata = handle.read()


def u(value, size):
    return int(value).to_bytes(size, 'big')


def parameter(key, value):
    return u(7, 2) + key.encode().ljust(32, b'\0') + u(value, 32)


# The request layout services/platform/hosted/node/bootstrap.sh writes: the module
# enable rows, the fee authority and parameter version rows,
# the kernel asset and the beta fee and demand constants, then the metadata.
request = b'LXGB' + b'\x02' + u(3, 2) + u(network, 4) + u(timestamp, 8) + u(2 + len(modules), 2)
request += b''.join(parameter('module-enable:' + module, 1) for module in modules)
request += parameter('native-fee-authority-version', 2) + parameter('parameter-version', 1)
request += u(0, 2)
request += bytes.fromhex(asset) + u(1, 4)
request += b''.join(u(value, 8) for value in (1, 1, 1, 1, 1, 8, 8, 64, 8)) + u(1, 8) + b'\x01' + u(1, 4)
request += b''.join(u(value, 8) for value in (1, 1, 2, 4, 1, 1, 100))
request += b''.join(u(value, 8) for value in (100, 1, 1, 10, 1, 1000))
request += metadata
if len(request) != 380 + 66 * len(modules) + len(metadata):
    raise SystemExit('genesis request has an unexpected length')
with os.fdopen(os.open(output, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600), 'wb') as handle:
    handle.write(request)
PY
	install -m 0600 /dev/null "$output/signer.key"
	python3 -c 'import sys; sys.stdout.buffer.write(bytes.fromhex(sys.stdin.read()))' <<<"$seed" >"$output/signer.key"
	unset seed
	rc=0
	"$build" "$output/genesis-request.lxgb" "$output/signer.key" "$output/genesis" || rc=$?
	rm -f "$output/signer.key"
	[ "$rc" = 0 ] || fail "layerx-genesis-build refused the rendered genesis ($rc)"
	manifest=$output/genesis/genesis.manifest
	for entry in "$manifest" "$output/genesis/00000000000000000000.lxs" "$output/genesis/paxeer-registration-request.lxrr" \
		"$output/genesis/paxeer-deployment-descriptor.lxgd"; do
		[ -s "$entry" ] || fail "genesis artifact missing: $entry"
	done

	echo "network_id=$network"
	echo "genesis_sha256=$(sha256sum "$manifest" | cut -d' ' -f1)"
	echo "request_sha256=$(sha256sum "$output/genesis-request.lxgb" | cut -d' ' -f1)"
	echo "sequencer_public_key=$sequencer_public"
	echo "modules=${modules[*]}"
	for entry in "${records[@]}"; do
		IFS=: read -r id symbol decimals <<<"$entry"
		echo "asset $symbol id=$id decimals=$decimals"
	done
	echo "genesis dir=$output/genesis"
}

case "${1:-}" in
render) render_step "${@:2}" ;;
keys) keys_step "${@:2}" ;;
genesis) genesis_step "${@:2}" ;;
plan)
	[ "$#" = 1 ] || fail "usage: kernel-genesis.sh plan"
	identity plan "${retire_files[@]}"
	;;
migrate)
	[ "$#" = 2 ] && [[ $2 =~ ^[0-9a-f]{64}$ ]] || fail "usage: kernel-genesis.sh migrate PLAN_SHA256"
	identity migrate "$2" "${retire_files[@]}" || fail "identity migration refused"
	;;
*) fail "usage: kernel-genesis.sh keys [rotate] | render ENV_FILE OUTPUT_DIR | genesis SID=POINTER USDC=POINTER USDL=POINTER [SYMBOL=POINTER...] | plan | migrate PLAN_SHA256" ;;
esac

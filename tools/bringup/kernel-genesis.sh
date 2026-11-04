#!/usr/bin/env bash
# Native chain initialization and retained paxd identity. Historical LXGB
# rendering is isolated in explicit offline commands and never starts a node.
set -euo pipefail
umask 077
fail() { printf 'kernel-genesis: %s\n' "$*" >&2; exit 1; }
inspector="${BRINGUP_INSPECT_BIN:-bringup-inspect}"

hex_public() {
	python3 -c 'import sys; sys.stdout.buffer.write(bytes.fromhex("302e020100300506032b657004220420" + sys.argv[1]))' "$1" |
		openssl pkey -inform DER -pubout -outform DER | tail -c 32 | od -An -tx1 | tr -d ' \n'
}

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

regenerate_fixture() {
	[ "$#" = 1 ] || fail "usage: kernel-genesis.sh offline-regenerate-fixture FIXTURE_DIR"
	local fixture=$1 build
	build=${LAYERX_GENESIS_BUILD:-$(cd "$(dirname "$0")/../.." && pwd)/build/bin/layerx-genesis-build}
	[ -x "$build" ] || fail "layerx-genesis-build not found: $build"
	python3 - "$fixture" "$build" <<'PYFIXTURE'
import hashlib
import json
import pathlib
import subprocess
import sys
import tempfile

fixture = pathlib.Path(sys.argv[1]).resolve()
expected = json.loads((fixture / 'expected.json').read_text())
# These are public test identities, derived reproducibly without deployment keys.
seed = hashlib.sha256(('layerx-public-fixture:' + fixture.name).encode()).digest()
with tempfile.TemporaryDirectory(prefix='.regenerate-', dir=fixture) as scratch:
    scratch = pathlib.Path(scratch)
    key = scratch / 'signer.key'
    key.write_bytes(seed)
    key.chmod(0o600)
    public = subprocess.run(
        ['openssl', 'pkey', '-inform', 'DER', '-pubout', '-outform', 'DER'],
        input=bytes.fromhex('302e020100300506032b657004220420') + seed,
        stdout=subprocess.PIPE, check=True).stdout[-32:]
    output = scratch / 'genesis'
    subprocess.run([sys.argv[2], str(fixture / 'genesis-request.lxgb'),
                    str(key), str(output)], check=True)
    registration = (output / 'paxeer-registration-request.lxrr').read_bytes()
    if len(registration) != 73 or registration[:5] != b'LXRR\x01':
        raise SystemExit('unexpected genesis registration format')
    if int.from_bytes(registration[5:9], 'big') != expected['network_id']:
        raise SystemExit('genesis network differs from fixture')
    root, receipt = registration[9:41].hex(), registration[41:73].hex()
    if root != expected['canonical_state_root']:
        expected['previous_canonical_state_root'] = expected['canonical_state_root']
        expected['previous_receipt_state_root'] = expected['receipt_state_root']
    expected['canonical_state_root'] = root
    expected['receipt_state_root'] = receipt
    expected['sequencer_public_key'] = public.hex()
    for name in expected['sha256']:
        data = (output / name).read_bytes()
        (fixture / name).write_bytes(data)
        expected['sha256'][name] = hashlib.sha256(data).hexdigest()
    (fixture / 'sequencer.public').write_bytes(public)
    (fixture / 'expected.json').write_text(json.dumps(expected, indent=2, sort_keys=True) + '\n')
PYFIXTURE
}

render_step() {
	[ "$#" = 2 ] || fail "usage: kernel-genesis.sh offline-render ENV_FILE OUTPUT_DIR"
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
		[ "$((10#$decimals))" -le 38 ] || fail "asset $symbol decimals must be 0..38"
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
init)
	[ "$#" = 8 ] || fail "usage: kernel-genesis.sh init HOME MONIKER CHAIN_ID MODE GENESIS_JSON SHA256 STOPPED_UNIT"
	home=$2 moniker=$3 chain=$4 mode=$5 approved=$6 digest=$7 unit=$8
	# The operator names the exclusive service owner; never overwrite a live home.
	[[ "$unit" =~ ^[a-zA-Z0-9_.@-]+\.service$ ]] || fail "invalid paxd unit"
	exec 8>"${home}.bringup.lock"
	flock -x 8
	[ "$(systemctl show "$unit" -p MainPID --value)" = 0 ] || fail "paxd unit still has a running process"
	state="$(systemctl show "$unit" -p ActiveState --value)"
	[[ "$state" = inactive || "$state" = failed ]] || fail "paxd unit is not stopped"
	"$inspector" paxd-init-check "$home" "$moniker" "$chain" "$mode"
	"$inspector" paxd-genesis-check "$approved" "$chain" "$digest"
	"${PAXD:-paxd}" init "$moniker" --home "$home" --chain-id "$chain" --mode "$mode"
	install -m 0600 "$approved" "$home/config/genesis.json"
	"$inspector" paxd-identity adopt "$home" "$chain"
	;;
adopt | identity)
	[ "$#" = 3 ] || fail "usage: kernel-genesis.sh adopt|identity HOME CHAIN_ID"
	action=check; [ "$1" != adopt ] || action=adopt
	"$inspector" paxd-identity "$action" "$2" "$3"
	;;
offline-render) render_step "${@:2}" ;;
offline-regenerate-fixture) regenerate_fixture "${@:2}" ;;
keys | genesis | plan | migrate | render | regenerate-fixture)
	fail "retired live workflow; use init/adopt/identity for paxd, or explicit offline-render/offline-regenerate-fixture for historical artifacts"
	;;
*) fail "usage: kernel-genesis.sh init|adopt|identity|offline-render|offline-regenerate-fixture (see tools/bringup/paxd-initialization.md)" ;;
esac

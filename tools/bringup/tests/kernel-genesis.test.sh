#!/usr/bin/env bash
# Qualify native paxd initialization and its signed committed output.
set -euo pipefail
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)
fixture=$root/services/platform/hosted/node/tests/fixtures/repair_22_31_355
paxd=${PAXD:-$root/build/paxd}
inspect=${BRINGUP_INSPECT_BIN:-bringup-inspect}
work=$(mktemp -d)
child=
cleanup() {
	if [ -n "$child" ]; then kill "$child" 2>/dev/null || true; wait "$child" 2>/dev/null || true; fi
	rm -rf "$work"
}
fail() { printf 'kernel-genesis.test: %s\n' "$*" >&2; exit 1; }
# Historical render checks remain available only under their explicit offline mode.
if [ "${1:-}" = offline ]; then
    trap cleanup EXIT
    script=$root/tools/bringup/kernel-genesis.sh
    export LAYERX_GENESIS_BUILD=${LAYERX_GENESIS_BUILD:-$root/build/bin/layerx-genesis-build}
hex() { od -An -v -tx1 | tr -d ' \n'; }
ed25519_public() {
	python3 -c 'import sys; sys.stdout.buffer.write(bytes.fromhex("302e020100300506032b657004220420" + sys.argv[1]))' "$1" |
		openssl pkey -inform DER -pubout -outform DER | tail -c 32 | hex
}
sha() { printf '%s' "$1" | sha256sum | cut -d' ' -f1; }

[ -x "$LAYERX_GENESIS_BUILD" ] || fail "layerx-genesis-build is not built at $LAYERX_GENESIS_BUILD"

openssl rand -hex 32 | tr -d '\n' >"$work/sequencer.key"
sequencer_public=$(ed25519_public "$(cat "$work/sequencer.key")")
treasury_public=$(ed25519_public "$(openssl rand -hex 32)")
guarantor_public=$(openssl ecparam -name secp256k1 -genkey -noout |
	openssl ec -pubout -conv_form compressed -outform DER 2>/dev/null | tail -c 33 | hex)
guarantor_id=$(sha "layerx-beta-guarantor:$guarantor_public")
modules=(escrow budget stream service perps spot web)

fixture() {
	cat <<ENV
# kernel genesis fixture
LAYERX_GENESIS_NETWORK_ID=77
LAYERX_GENESIS_SEQUENCER_PUBLIC_KEY=$sequencer_public
LAYERX_GENESIS_SEQUENCER_KEY_FILE=$work/sequencer.key
LAYERX_GENESIS_TREASURY_PUBLIC_KEY=$treasury_public
LAYERX_GENESIS_GUARANTORS=$guarantor_id:$guarantor_public
LAYERX_GENESIS_ASSETS="PAX:$(sha layerx-asset:125:PAX):6 SID:$(sha layerx-asset:125:SID):18"
LAYERX_GENESIS_TIMESTAMP_MS=1790000000000
ENV
}
{ fixture; echo "LAYERX_GENESIS_MODULES=${modules[*]}"; } >"$work/genesis.env"

bash "$script" offline-render "$work/genesis.env" "$work/out" >"$work/render.out" ||
	fail "render refused the fixture: $(cat "$work/render.out")"
manifest=$work/out/genesis/genesis.manifest
[ -s "$manifest" ] || fail "no genesis manifest"
grep -qx "sequencer_public_key=$sequencer_public" "$work/render.out" || fail "render did not report the fixture sequencer"
grep -qx "modules=budget escrow perps service spot stream web" "$work/render.out" || fail "render did not report the seven modules"

python3 - "$manifest" "$sequencer_public" "$guarantor_id" "$guarantor_public" "${modules[@]}" <<'PY'
import sys

manifest = open(sys.argv[1], 'rb').read()
sequencer, guarantor, guarantor_public = (bytes.fromhex(value) for value in sys.argv[2:5])
modules = sys.argv[5:]


def field(value):
    return len(value).to_bytes(4, 'big') + value


# The manifest ends with the signer public key and the signature, each a
# length-prefixed field (src/protocol/lxp_genesis.c lxp_genesis_encode).
if manifest[-104:-68] != field(sequencer) or manifest[-68:-64] != (64).to_bytes(4, 'big'):
    raise SystemExit('the manifest is not signed by the fixture sequencer')
for module in modules:
    row = field(('module-enable:' + module).encode().ljust(32, b'\0')) + field(bytes(31) + b'\x01')
    if manifest.count(row) != 1:
        raise SystemExit('the manifest does not enable ' + module)
if manifest.count(b'module-enable:') != len(modules):
    raise SystemExit('the manifest enables modules beyond the fixture list')
if manifest.count(field(guarantor) + field(guarantor_public)) != 1:
    raise SystemExit('the manifest does not carry the fixture guarantor')
PY

# Refusals: an unknown module, a sequencer key that is not the declared one,
# and bootstrap.sh's beta profile admits web but not beside explicit modules.
{ fixture; echo "LAYERX_GENESIS_MODULES=${modules[*]} asset"; } >"$work/unknown.env"
! bash "$script" offline-render "$work/unknown.env" "$work/unknown" 2>"$work/unknown.err" || fail "render accepted an unknown module"
grep -q 'not asset' "$work/unknown.err" || fail "unknown module refusal: $(cat "$work/unknown.err")"
[ ! -e "$work/unknown" ] || fail "a refused render left output"
openssl rand -hex 32 | tr -d '\n' >"$work/sequencer.key"
! bash "$script" offline-render "$work/genesis.env" "$work/mismatch" 2>"$work/mismatch.err" || fail "render accepted a foreign sequencer key"
grep -q 'does not hold the key' "$work/mismatch.err" || fail "sequencer refusal: $(cat "$work/mismatch.err")"
! bash "$root/services/platform/hosted/node/bootstrap.sh" --module-profile beta --enable-module web 2>"$work/profile.err" ||
	fail "bootstrap accepted --module-profile beside --enable-module"
grep -q -- '--module-profile excludes --enable-module' "$work/profile.err" || fail "bootstrap profile refusal: $(cat "$work/profile.err")"

echo "kernel-genesis.test: ok genesis_sha256=$(sha256sum "$manifest" | cut -d' ' -f1)"
	exit 0
fi
trap cleanup EXIT
fail() { printf 'kernel-genesis.test: %s\n' "$*" >&2; exit 1; }
[ -x "$paxd" ] || fail "the merged paxd artifact is required"
"$paxd" init bringup-genesis-test --chain-id hyperpax_125-1 --mode validator --home "$work/node" >"$work/init.log" 2>&1
for name in genesis.json config.toml app.toml; do install -m 0600 "$fixture/$name" "$work/node/config/$name"; done
python3 - "$fixture/native_configuration.json" "$work/node/config" <<'PY'
import json, pathlib, sys
configuration = json.loads(pathlib.Path(sys.argv[1]).read_text())
output = pathlib.Path(sys.argv[2])
(output / 'receipt.key').write_text(configuration['receipt_seed'])
(output / 'priv_validator_key.json').write_text(json.dumps(configuration['validator_key']))
for name in ('receipt.key', 'priv_validator_key.json'):
    (output / name).chmod(0o600)
PY
"$inspect" native-genesis-check "$work/node/config/genesis.json" "$work/node/config/receipt.key" >"$work/initialization.json"
"$paxd" validate-genesis --home "$work/node" >"$work/validate.log" 2>&1 || fail "paxd refused native application genesis"
python3 - "$work/node/config/genesis.json" "$work/initialization.json" <<'PY'
import json, sys
actual, report = (json.load(open(path)) for path in sys.argv[1:])
assert actual['chain_id'] == 'hyperpax_125-1'
assert set(report['modules']) == {'bank', 'sequencer', 'lxphost'}
assert actual['app_state']['lxphost'] == {'last_sequence': 0, 'records': None}
assert actual['app_state']['sequencer']['receipt_keys']
assert all(coin['denom'] == 'uhpx' and int(coin['amount']) > 0
           for balance in actual['app_state']['bank']['balances'] for coin in balance['coins'])
assert not {'layerxcustody', 'layerxanchor', 'guarantor'} & set(actual['app_state'])
PY
# Malformed module/asset inputs and a foreign signer are refused before launch.
python3 - "$work/node/config/genesis.json" "$work" <<'PY'
import copy, json, pathlib, sys
original = json.load(open(sys.argv[1]))
for name in ('retired-module', 'bad-asset', 'bad-range'):
    altered = copy.deepcopy(original)
    if name == 'retired-module':
        altered['app_state']['layerxcustody'] = {}
    elif name == 'bad-asset':
        altered['app_state']['bank']['balances'][0]['coins'][0]['amount'] = '-1'
    else:
        altered['app_state']['sequencer']['receipt_keys'][0]['first_height'] = 0
    (pathlib.Path(sys.argv[2]) / (name + '.json')).write_text(json.dumps(altered))
PY
for name in retired-module bad-asset bad-range; do
	if "$inspect" native-genesis-check "$work/$name.json" "$work/node/config/receipt.key" >/dev/null 2>&1; then fail "accepted $name"; fi
done
openssl rand -hex 32 >"$work/foreign.key"
if "$inspect" native-genesis-check "$work/node/config/genesis.json" "$work/foreign.key" >/dev/null 2>&1; then fail "accepted a foreign receipt signer"; fi
if "$root/services/platform/hosted/node/bootstrap.sh" --module-profile beta --enable-module web >/dev/null 2>&1; then fail "accepted the retired native bootstrap schema"; fi
read -r rpc_port comet_port < <(python3 - <<'PY'
import socket
with socket.socket() as rpc, socket.socket() as comet:
    rpc.bind(('127.0.0.1', 0)); comet.bind(('127.0.0.1', 0))
    print(rpc.getsockname()[1], comet.getsockname()[1])
PY
)
"$paxd" start --home "$work/node" --chain-id hyperpax_125-1 --p2p.laddr tcp://127.0.0.1:0 \
	--rpc.laddr "tcp://127.0.0.1:$comet_port" --evm.http_enabled true --evm.http_address 127.0.0.1 \
	--evm.http_port "$rpc_port" --evm.ws_enabled false --sequencer.enable true \
	--sequencer.receipt-key-file "$work/node/config/receipt.key" --sequencer.wal-dir "$work/node/data/sequencer-wal" >"$work/process.log" 2>&1 &
child=$!
for attempt in $(seq 90); do
	if "$inspect" foundation-observe "$work/committed" "$work/node/config/genesis.json" "$work/node/config/receipt.key" \
		"http://127.0.0.1:$rpc_port" "http://127.0.0.1:$comet_port" >"$work/commit.json" 2>"$work/commit.err"; then
		break
	fi
	kill -0 "$child" || fail "paxd exited before a signed commit"
	sleep 1
done
[ -s "$work/committed/committed.json" ] || fail "native signed commit did not qualify"
echo 'kernel-genesis.test: ok modules=bank,sequencer,lxphost assets=uhpx signature=canonical-native-commit'

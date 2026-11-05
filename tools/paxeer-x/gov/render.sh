#!/usr/bin/env bash
# Render the first post-upgrade governance window into an output directory and
# dry-run every proposal through paxd (--generate-only --offline), failing on
# any rejection.
set -euo pipefail

usage() {
  cat <<'USAGE'
usage: render.sh --out DIR --height H --release-url URL --release-sha256 HEX
                 --deposit COINS --pax-usd PRICE
                 --kernel-network-id N [--rate-update-height H]
                 [--bridge-config FILE]... [--bridge-manifest FILE]
                 [--paxd BIN] [--from BECH32] [--chain-id ID]
USAGE
  exit 2
}

ROOT=$(cd "$(dirname "$0")/../../.." && pwd)
TEMPLATES="$ROOT/tools/paxeer-x/gov/templates"
OUTPUT_GUARD="$ROOT/tools/paxeer-x/gov/output.py"
GOV_AUTHORITY=pax10d07y265gmmuvt4z0w9aw880jnsr700jxdwa9m
CHAIN_ID=hyperpax_125-1
FROM=$GOV_AUTHORITY
PAXD=${PAXD:-paxd}
BRIDGE_MANIFEST="$ROOT/services/bridge/deploy/attestors.json"
BRIDGE_CONFIGS=()
OUT="" HEIGHT="" RELEASE_URL="" RELEASE_SHA256="" DEPOSIT="" PAX_USD="" RATE_UPDATE_HEIGHT=""
KERNEL_NETWORK_ID=""

while [ $# -gt 0 ]; do
  [ $# -ge 2 ] || usage
  case "$1" in
    --out) OUT=$2 ;;
    --height) HEIGHT=$2 ;;
    --release-url) RELEASE_URL=$2 ;;
    --release-sha256) RELEASE_SHA256=$2 ;;
    --deposit) DEPOSIT=$2 ;;
    --pax-usd) PAX_USD=$2 ;;
    --rate-update-height) RATE_UPDATE_HEIGHT=$2 ;;
    --kernel-network-id) KERNEL_NETWORK_ID=$2 ;;
    --bridge-config) BRIDGE_CONFIGS+=("$2") ;;
    --bridge-manifest) BRIDGE_MANIFEST=$2 ;;
    --paxd) PAXD=$2 ;;
    --from) FROM=$2 ;;
    --chain-id) CHAIN_ID=$2 ;;
    *) usage ;;
  esac
  shift 2
done

die() { echo "render.sh: $*" >&2; exit 1; }
for name in OUT HEIGHT RELEASE_URL RELEASE_SHA256 DEPOSIT PAX_USD KERNEL_NETWORK_ID; do
  [ -n "${!name}" ] || die "--$(echo "$name" | tr 'A-Z_' 'a-z-') is required"
done
RATE_UPDATE_HEIGHT=${RATE_UPDATE_HEIGHT:-$HEIGHT}
[[ $HEIGHT =~ ^[1-9][0-9]*$ ]] || die "--height $HEIGHT is not a positive integer"
[[ $RATE_UPDATE_HEIGHT =~ ^[1-9][0-9]*$ ]] || die "--rate-update-height $RATE_UPDATE_HEIGHT is not a positive integer"
[[ $KERNEL_NETWORK_ID =~ ^[1-9][0-9]*$ && ${#KERNEL_NETWORK_ID} -le 10 && $KERNEL_NETWORK_ID -le 4294967295 ]] \
  || die "--kernel-network-id $KERNEL_NETWORK_ID is not a positive uint32"
[[ $RELEASE_SHA256 =~ ^[0-9a-f]{64}$ ]] || die "--release-sha256 is not 64 lowercase hex characters"
OUT=$(realpath -m "$OUT")
BRIDGE_MANIFEST=$(realpath "$BRIDGE_MANIFEST")
for i in "${!BRIDGE_CONFIGS[@]}"; do BRIDGE_CONFIGS[$i]=$(realpath "${BRIDGE_CONFIGS[$i]}"); done
command -v "$PAXD" >/dev/null || die "paxd binary $PAXD not found (pass --paxd or PAXD)"

# 0.001 USD in uhpx (1 PAX = 10^6 uhpx), rounded half up.
XWEB_FEE=$(python3 - "$PAX_USD" <<'PY'
import sys
from decimal import Decimal, ROUND_HALF_UP, InvalidOperation
try:
    price = Decimal(sys.argv[1])
except InvalidOperation:
    sys.exit("--pax-usd %r is not a decimal" % sys.argv[1])
if not price.is_finite() or price <= 0:
    sys.exit("--pax-usd must be positive")
fee = (Decimal("0.001") / price * 1000000).quantize(Decimal(1), rounding=ROUND_HALF_UP)
if fee < 1:
    sys.exit("--pax-usd %s prices 0.001 USD below one uhpx" % price)
print(fee)
PY
)

UPGRADE_INFO=$(jq -cn --arg url "$RELEASE_URL" --arg sha "$RELEASE_SHA256" \
  '{binaries: {"linux/amd64": ($url + "?checksum=sha256:" + $sha)}}')

VARS=$(jq -n \
  --arg HEIGHT "$HEIGHT" --arg UPGRADE_INFO "$UPGRADE_INFO" --arg DEPOSIT "$DEPOSIT" \
  --arg FEE_TOKEN_RATE "3114000" --arg RATE_UPDATE_HEIGHT "$RATE_UPDATE_HEIGHT" \
  --arg MAX_FEE_TOKEN_RATE_AGE "3400000" --arg GOV_AUTHORITY "$GOV_AUTHORITY" \
  --arg XWEB_FEE "$XWEB_FEE" --argjson XWEB_MAX_PAYLOAD_BYTES 8192 \
  --arg XWEB_MAX_CALLBACK_GAS "500000" --arg XWEB_TIMEOUT_BLOCKS "3600" \
  --argjson XWEB_KERNEL_NETWORK_ID "$KERNEL_NETWORK_ID" \
  --argjson XWEB_PROGRAM_FEE_DENOMS '["uhpx","usid","uusdc","uusdl"]' \
  '$ARGS.named')

OUT=$(python3 "$OUTPUT_GUARD" prepare "$OUT")
HOME_DIR=$(mktemp -d)
trap 'rm -rf "$HOME_DIR"' EXIT

for template in "$TEMPLATES"/*.json; do
  jq --argjson v "$VARS" '
    walk(if type == "string" and test("^@[A-Z0-9_]+@$")
         then (.[1:-1]) as $k | if ($v | has($k)) then $v[$k] else error("unset placeholder \($k)") end
         else . end)' "$template" >"$OUT/$(basename "$template")"
done

dry_run() {
  local file=$1; shift
  if ! "$PAXD" "$@" --generate-only --offline --chain-id "$CHAIN_ID" --from "$FROM" \
      --home "$HOME_DIR" --output json >"$file.tx.json" 2>"$file.err"; then
    cat "$file.err" >&2
    die "paxd refused $(basename "$file")"
  fi
  jq -e '.body.messages | length == 1' "$file.tx.json" >/dev/null || die "no transaction for $(basename "$file")"
  rm -f "$file.err"
  echo "validated $(basename "$file")"
}

up="$OUT/01-software-upgrade.json"
dry_run "$up" tx gov submit-proposal software-upgrade "$(jq -r .name "$up")" \
  --upgrade-height "$(jq -r .height "$up")" --upgrade-info "$(jq -r .info "$up")" \
  --title "$(jq -r .title "$up")" --description "$(jq -r .description "$up")" --deposit "$(jq -r .deposit "$up")"
dry_run "$OUT/02-fee-token-params.json" tx gov submit-proposal param-change "$OUT/02-fee-token-params.json"
dry_run "$OUT/03-xweb-fee.json" tx gov submit-proposal layerx-proposal "$OUT/03-xweb-fee.json" --deposit "$DEPOSIT"
for config in "${BRIDGE_CONFIGS[@]}"; do
  dir=$(python3 "$OUTPUT_GUARD" bridge "$OUT" "$config")
  stage=$(mktemp -d "$OUT/.bridge-render.XXXXXXXX")
  (cd "$ROOT" && go run ./services/bridge/deploy/proposals/cmd/paxeer-bridge-proposals -manifest "$BRIDGE_MANIFEST" \
    -proposals "$stage/proposals" "$config" "$stage/bodies") >/dev/null
  python3 "$OUTPUT_GUARD" publish "$OUT" "$dir" "$stage"
  rmdir -- "$stage/bodies" "$stage/proposals" "$stage"
  for file in "$dir"/proposals/04-proposal-open-chain.json "$dir"/proposals/05-proposal-sidiora-cap.json; do
    [ -f "$file" ] || continue
    dry_run "$file" tx gov submit-proposal layerxbridge-proposal "$file" --deposit "$DEPOSIT"
  done
done

#!/usr/bin/env bash
# Render the governance window kit with fixed inputs and assert every proposal
# exists and passes the paxd offline dry run. No network.
set -euo pipefail
ROOT=$(cd "$(dirname "$0")/../../.." && pwd)
command -v go >/dev/null || PATH=$PATH:/usr/local/go/bin
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

fail() { echo "test.sh: $*" >&2; exit 1; }

plan_named_v6120() {
  jq -e '.name == "v6.12.0"' "$OUT/01-software-upgrade.json" >/dev/null \
    || fail "plan_named_v6120"
  echo "plan_named_v6120: ok"
}

sid_docs_name_v6120() {
  local file
  for file in tools/paxeer-x/sid-snapshot/README.md RELEASE.md; do
    grep -Fq 'v6.12.0 migration' "$ROOT/$file" || fail "sid_docs_name_v6120: $file"
    grep -Fq 'SID holder migration and SID mint-authority move' "$ROOT/$file" \
      || fail "sid_docs_name_v6120: SID hooks missing in $file"
    grep -Fq 'node/upgrades_v6120.go' "$ROOT/$file" \
      || fail "sid_docs_name_v6120: handler missing in $file"
    if grep -Eq 'v6[.]11\b' "$ROOT/$file"; then
      fail "sid_docs_name_v6120: retired name in $file"
    fi
  done
  echo "sid_docs_name_v6120: ok"
}

sid_docs_name_v6120
for file in tools/paxeer-x/gov/templates/01-software-upgrade.json \
  tools/paxeer-x/gov/README.md tools/paxeer-x/gov/test.sh \
  tools/paxeer-x/sid-snapshot/README.md tools/bringup/sync-fleet.test.sh \
  tools/hpx/publish.sh RELEASE.md contracts/src/SidioraNativeERC20.sol; do
  grep -Fq 'v6.12.0' "$ROOT/$file" || fail "release name missing in $file"
  if grep -Eq 'v6[.]11\b' "$ROOT/$file"; then
    fail "retired release name in $file"
  fi
done
echo "release_files_name_v6120: ok"

PAXD=${PAXD:-}
if [ -z "$PAXD" ]; then
  (cd "$ROOT" && go build -tags netgo -o "$WORK/paxd" ./daemon/paxd)
  PAXD=$WORK/paxd
fi

OUT=${GOV_TEST_OUT:-$WORK/out}
RENDER_ARGS=(
  --paxd "$PAXD"
  --height 27000000
  --release-url https://example.invalid/paxd-v6.12.0-linux-amd64
  --release-sha256 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
  --deposit 10000000uhpx --pax-usd 0.02
  --kernel-network-id 125
  --bridge-manifest "$ROOT/services/bridge/deploy/proposals/testdata/attestors.json"
)
"$ROOT/tools/paxeer-x/gov/render.sh" --out "$OUT" "${RENDER_ARGS[@]}" \
  --bridge-config "$ROOT/services/bridge/deploy/proposals/testdata/solana.json"

bridge_names_refuse_traversal() {
  local name config="$WORK/invalid-bridge.json" output="$WORK/refused"
  mkdir "$WORK/escape"
  printf 'operator data must survive\n' >"$WORK/escape/sentinel"
  for name in '../escape' '../../escape' 'solana/../../escape' \
    'solana/child' 'solana\child' "$WORK/escape" '.' '..' ''; do
    jq --arg name "$name" '.name = $name' \
      "$ROOT/services/bridge/deploy/proposals/testdata/solana.json" >"$config"
    if "$ROOT/tools/paxeer-x/gov/render.sh" --out "$output" "${RENDER_ARGS[@]}" \
        --bridge-config "$config" >"$WORK/refused.log" 2>&1; then
      fail "bridge_names_refuse_traversal: accepted $name"
    fi
    grep -Fq 'bridge name must be' "$WORK/refused.log" \
      || fail "bridge_names_refuse_traversal: wrong refusal for $name"
    [ ! -e "$output/bridge" ] || fail "bridge_names_refuse_traversal: created bridge directory"
    [ "$(cat "$WORK/escape/sentinel")" = 'operator data must survive' ] \
      || fail "bridge_names_refuse_traversal: changed outside sentinel"
    [ "$(ls -A "$WORK/escape")" = sentinel ] \
      || fail "bridge_names_refuse_traversal: wrote outside output"
  done
  echo "bridge_names_refuse_traversal: ok"
}

bridge_names_refuse_traversal

for f in 01-software-upgrade 02-fee-token-params 03-xweb-fee \
  bridge/solana/proposals/04-proposal-open-chain bridge/solana/proposals/05-proposal-sidiora-cap; do
  [ -s "$OUT/$f.json" ] || fail "$f.json missing"
  [ -s "$OUT/$f.json.tx.json" ] || fail "$f.json was not dry-run"
  jq -e '.body.messages[0]["@type"] == "/cosmos.gov.v1beta1.MsgSubmitProposal"' "$OUT/$f.json.tx.json" >/dev/null \
    || fail "$f.json.tx.json does not carry a MsgSubmitProposal"
done

plan_named_v6120
jq -e '.name == "v6.12.0" and .height == "27000000" and (.info | fromjson | .binaries["linux/amd64"] | endswith("?checksum=sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"))' \
  "$OUT/01-software-upgrade.json" >/dev/null || fail "upgrade plan"
jq -e '.changes == [
  {subspace: "evm", key: "KeyFeeTokenEnabled", value: true},
  {subspace: "evm", key: "KeyAllowedFeeDenoms", value: [{denom: "usid", rate: "3114000", rate_update_height: "27000000"}]},
  {subspace: "evm", key: "KeyMaxFeeTokenRateAge", value: "3400000"}]' "$OUT/02-fee-token-params.json" >/dev/null || fail "fee-token params"
jq -e '.messages[0].fee == "50000"' "$OUT/03-xweb-fee.json" >/dev/null || fail "xweb fee is not 0.001 USD at 0.02 USD/PAX"
jq -e '.messages[0].kernel_network_id == 125 and .messages[0].program_fee_denoms == ["uhpx","usid","uusdc","uusdl"]' \
  "$OUT/03-xweb-fee.json" >/dev/null || fail "xweb program settings"
echo "governance window kit: ok"

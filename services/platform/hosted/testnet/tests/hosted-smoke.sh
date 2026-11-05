#!/bin/sh
set -eu
umask 077
: "${LAYERX_TESTNET_URL:?LAYERX_TESTNET_URL is required}"
: "${LAYERX_GATEWAY_URL:?LAYERX_GATEWAY_URL is required}"
: "${LAYERX_FAUCET_URL:?LAYERX_FAUCET_URL is required}"
: "${LAYERX_TEST_AUTH_TOKEN_FILE:?LAYERX_TEST_AUTH_TOKEN_FILE is required}"
: "${LAYERX_TEST_CA_FILE:?LAYERX_TEST_CA_FILE is required}"
: "${LAYERX_TEST_SOURCE_DID:?LAYERX_TEST_SOURCE_DID is required}"
: "${LAYERX_TEST_SOURCE_PUBLIC_KEY:?LAYERX_TEST_SOURCE_PUBLIC_KEY is required}"
: "${LAYERX_TEST_DESTINATION_DID:?LAYERX_TEST_DESTINATION_DID is required}"
: "${LAYERX_TEST_DESTINATION_PUBLIC_KEY:?LAYERX_TEST_DESTINATION_PUBLIC_KEY is required}"
: "${LAYERX_TEST_DESTINATION_AUTH_TOKEN_FILE:?LAYERX_TEST_DESTINATION_AUTH_TOKEN_FILE is required}"
: "${LAYERX_TEST_SEQUENCER_PUBLIC_KEY:?An independent sequencer public key is required}"
: "${LAYERX_TEST_ASSET:?LAYERX_TEST_ASSET is required}"
: "${LAYERX_TEST_AMOUNT:?LAYERX_TEST_AMOUNT is required}"
: "${LAYERX_TEST_ACCOUNTS_FILE:?The explicit DID/EVM smoke account bindings are required}"
: "${LAYERX_TEST_SOURCE_EVM_KEY_FILE:?LAYERX_TEST_SOURCE_EVM_KEY_FILE is required}"
: "${LAYERX_TEST_NODE_URL:?The HTTPS root node RPC endpoint is required}"
: "${LAYERX_TEST_RECEIPT_KEYS_FILE:?Independent node receipt keys are required}"
test -r "$LAYERX_TEST_AUTH_TOKEN_FILE"
test -r "$LAYERX_TEST_CA_FILE"
command -v jq >/dev/null
command -v openssl >/dev/null
test -r "$LAYERX_TEST_RECEIPT_KEYS_FILE"
work=$(mktemp -d "${TMPDIR:-/tmp}/layerx-hosted-smoke.XXXXXX")
cleanup() {
  printf '%s\n' "retained hosted smoke evidence at $work"
}
trap cleanup EXIT HUP INT TERM
chmod 0700 "$work"
python3 "$(dirname "$0")/journey-credentials.py" --gateway "$LAYERX_GATEWAY_URL" \
  --ca "$LAYERX_TEST_CA_FILE" --session "$LAYERX_TEST_AUTH_TOKEN_FILE" \
  --destination-session "$LAYERX_TEST_DESTINATION_AUTH_TOKEN_FILE" \
  --public-key "$LAYERX_TEST_SOURCE_PUBLIC_KEY" --output "$work/credentials"
auth_config="$work/credentials/gateway.curl"
session_config="$work/credentials/source.curl"
journey="scheduled-$(date -u +%Y%m%dT%H%M%SZ)-$$"

fetch_document() {
  fetch_name=$1
  fetch_url=$2
  curl --silent --show-error --max-time 15 --cacert "$LAYERX_TEST_CA_FILE" \
    --output "$work/$fetch_name.json" --write-out '%{http_code}' "$fetch_url" \
    > "$work/$fetch_name.status"
  jq -e 'type == "object"' "$work/$fetch_name.json" >/dev/null
}

admit_journey() {
  admit_name=$1
  fetch_document "journey-$admit_name" "$LAYERX_TESTNET_URL/v1/journeys/$admit_name"
  if test "$(cat "$work/journey-$admit_name.status")" != 200 \
    || ! jq -e '.admitted == true and .ready == true and (.failing | length) == 0' \
      "$work/journey-$admit_name.json" >/dev/null; then
    printf '%s\n' "journey $admit_name is not admitted:" >&2
    jq -r '"  failing: \(.failing // [] | join(", "))", (.dependencies[]? | select(.ready | not) | "  dependency \(.name): \(.detail)")' \
      "$work/journey-$admit_name.json" >&2
    exit 1
  fi
  printf '%s\n' "journey $admit_name admitted: $(jq -r '[.dependencies[].name] | join(", ")' "$work/journey-$admit_name.json")"
}

fetch_document testnet-readyz "$LAYERX_TESTNET_URL/readyz"
if test "$(cat "$work/testnet-readyz.status")" != 200 \
  || ! jq -e '.state == "ready"' "$work/testnet-readyz.json" >/dev/null; then
  printf '%s\n' "testnet-control is not ready:" >&2
  jq -r '(.release? | select(. != null and .state != "ready") | "  release: \(.detail)"), (.journeys[]? | select(.ready | not) | "  journey \(.journey) degraded: \(.failing | join(", "))"), (.dependencies[]? | select(.ready | not) | "  dependency \(.name): \(.detail)")' \
    "$work/testnet-readyz.json" >&2
  exit 1
fi
jq -e '.service == "layerx-hosted-testnet" and .release.state == "ready"
  and ([.dependencies[].name] | sort) == ["core","core_admin","faucet","gateway","identity","paxeer","receipt_authority","redis"]
  and all(.dependencies[]; .ready == true)
  and ([.journeys[].journey] | sort) == ["funding","payment","receipt_inspection"]
  and all(.journeys[]; .ready == true and (.failing | length) == 0)' \
  "$work/testnet-readyz.json" >/dev/null
jq -r '.dependencies[] | "edge testnet-control -> \(.name): \(.detail)"' "$work/testnet-readyz.json"

fetch_document gateway-readyz "$LAYERX_GATEWAY_URL/readyz"
if test "$(cat "$work/gateway-readyz.status")" != 200 \
  || ! jq -e '.status == "ready" and .service == "layerx-gateway"
      and .components.durable_store == "ready" and .components.core_agent_boundary == "ready"
      and .components.independent_receipt_authority == "ready" and .components.program_registry == "ready"' \
    "$work/gateway-readyz.json" >/dev/null; then
  printf '%s\n' "gateway is not ready:" >&2
  jq -r '.components // {} | to_entries[] | "  component \(.key): \(.value)"' "$work/gateway-readyz.json" >&2
  exit 1
fi
jq -r '.components | to_entries[] | "edge gateway -> \(.key): \(.value)"' "$work/gateway-readyz.json"

fetch_document faucet-readyz "$LAYERX_FAUCET_URL/readyz"
if test "$(cat "$work/faucet-readyz.status")" != 200 \
  || ! jq -e '.status == "ready" and .service == "faucet"' "$work/faucet-readyz.json" >/dev/null; then
  printf '%s\n' "faucet is not ready:" >&2
  cat "$work/faucet-readyz.json" >&2
  exit 1
fi
printf '%s\n' "edge faucet -> redis: ready"

fetch_document parameters "$LAYERX_TESTNET_URL/v1/parameters"
test "$(cat "$work/parameters.status")" = 200
jq -e '.network == "layerx-testnet" and (.network_id | type) == "number" and .evm_chain_id == 125
  and (.package_semver | type) == "string" and (.lxp_wire_protocol_version | type) == "number"' \
  "$work/parameters.json" >/dev/null
jq -e --slurpfile ready "$work/testnet-readyz.json" \
  '.network_id == $ready[0].network_id and .package_semver == $ready[0].package_semver
  and .lxp_wire_protocol_version == $ready[0].lxp_wire_protocol_version' \
  "$work/parameters.json" >/dev/null
ca_fingerprint=$(openssl x509 -in "$LAYERX_TEST_CA_FILE" -noout -fingerprint -sha256 | tr -d '\r\n')
cluster_identity="cluster identity: $(jq -r '"network=\(.network) network_id=\(.network_id) package_semver=\(.package_semver) lxp_wire_protocol_version=\(.lxp_wire_protocol_version)"' "$work/parameters.json") gateway_network_id=$(jq -r '.network_id' "$work/gateway-readyz.json") gateway_package_semver=$(jq -r '.package_semver' "$work/gateway-readyz.json") testnet=$LAYERX_TESTNET_URL gateway=$LAYERX_GATEWAY_URL faucet=$LAYERX_FAUCET_URL ca=$ca_fingerprint"
printf '%s\n' "$cluster_identity"

admit_journey funding
jq -n --arg did "$LAYERX_TEST_SOURCE_DID" --arg public_key "$LAYERX_TEST_SOURCE_PUBLIC_KEY" \
  '{did:$did, public_key:$public_key}' > "$work/faucet-request.json"
curl --fail --silent --show-error --max-time 30 --cacert "$LAYERX_TEST_CA_FILE" \
  --config "$session_config" \
  --request POST "$LAYERX_FAUCET_URL/v1/faucet/claims" \
  --header "Idempotency-Key: faucet-$journey" \
  --header 'Content-Type: application/json' --data-binary "@$work/faucet-request.json" \
  > "$work/faucet-response.json"
jq -e '.funded == true and .funding_id != null' "$work/faucet-response.json" >/dev/null
printf '%s\n' "funding journey: claim faucet-$journey funded"

jq -n --arg did "$LAYERX_TEST_DESTINATION_DID" --arg public_key "$LAYERX_TEST_DESTINATION_PUBLIC_KEY" \
  '{did:$did, public_key:$public_key}' > "$work/destination-faucet-request.json"
curl --fail --silent --show-error --max-time 30 --cacert "$LAYERX_TEST_CA_FILE" \
  --config "$work/credentials/destination.curl" \
  --request POST "$LAYERX_FAUCET_URL/v1/faucet/claims" \
  --header "Idempotency-Key: destination-faucet-$journey" \
  --header 'Content-Type: application/json' --data-binary "@$work/destination-faucet-request.json" \
  > "$work/destination-faucet-response.json"
jq -e '.funded == true and .funding_id != null' "$work/destination-faucet-response.json" >/dev/null

admit_journey payment
python3 "$(dirname "$0")/payment-journey.py" --gateway "$LAYERX_GATEWAY_URL" \
  --node "$LAYERX_TEST_NODE_URL" \
  --ca "$LAYERX_TEST_CA_FILE" --auth-config "$auth_config" --signer "$LAYERX_TEST_SOURCE_EVM_KEY_FILE" \
  --accounts "$LAYERX_TEST_ACCOUNTS_FILE" \
  --did "$LAYERX_TEST_SOURCE_DID" --destination "$LAYERX_TEST_DESTINATION_DID" \
  --asset "$LAYERX_TEST_ASSET" --amount "$LAYERX_TEST_AMOUNT" \
  --evm-chain-id "$(jq -er '.evm_chain_id' "$work/parameters.json")" \
  --sequencer-key "$LAYERX_TEST_SEQUENCER_PUBLIC_KEY" \
  --output "$work/payment"
receipt_id=$(jq -er '.activity_id' "$work/payment/result.json")
printf '%s\n' "payment journey: signed transfer $receipt_id executed and replayed without additional credit"

admit_journey receipt-inspection
python3 "$(dirname "$0")/receipt_inspection.py" --node "$LAYERX_TEST_NODE_URL" \
  --ca "$LAYERX_TEST_CA_FILE" --receipt-keys "$LAYERX_TEST_RECEIPT_KEYS_FILE" \
  --payment "$work/payment" --output "$work/receipt-inspection"
jq -e --arg activity "$receipt_id" '.verified == true and .activity_id == $activity
  and .result_code == 0 and .rung == "committed" and .verification_level == "sequencer-signed"
  and .replayed == true and .corruption_refused == true' \
  "$work/receipt-inspection/verification.json" >/dev/null
printf '%s\n' "receipt inspection journey: committed payment $receipt_id independently signature-verified"

printf '%s\n' "hosted payment $receipt_id was independently receipt-verified"
printf '%s\n' "$cluster_identity"

#!/usr/bin/env bash
set -euo pipefail

: "${WEBHOOKS_URL:?HTTPS webhook service URL is required}"
: "${WEBHOOKS_INGRESS_URL:?HTTPS private producer ingress URL is required}"
: "${WEBHOOKS_INGRESS_CA_FILE:?internal CA certificate of the ingress is required}"
: "${WEBHOOK_PRODUCER_CERT_FILE:?producer-role client certificate is required}"
: "${WEBHOOK_PRODUCER_KEY_FILE:?producer-role client key is required}"
: "${WEBHOOK_ENDPOINT_ID:?registered real endpoint is required}"
: "${WEBHOOK_SESSION_COOKIE_FILE:?protected developer session cookie file is required}"
: "${WEBHOOK_CSRF_FILE:?anti-forgery token file is required}"
: "${WEBHOOK_SOURCE_TRIGGER_TOKEN_FILE:?source trigger token file is required}"
: "${WEBHOOK_SOURCE_EVENT_FIRST:?first canonical source event id is required}"
: "${WEBHOOK_SOURCE_EVENT_SECOND:?second canonical source event id is required}"
: "${WEBHOOK_SOURCE_EVENT_STALE:?canonical event older than the second event is required}"
: "${WEBHOOK_SOURCE_KIND:?journey, payment, approval or program is required}"
: "${WEBHOOK_RECEIVER_OBSERVATIONS_URL:?real receiver observation API is required}"
: "${WEBHOOK_RECEIVER_TOKEN_FILE:?real receiver observation token is required}"
: "${WEBHOOK_RESTART_POD:?exact webhook participant pod to restart is required}"
: "${WEBHOOK_RESTART_NAMESPACE:?namespace of the webhook participant is required}"

restart_timeout="${WEBHOOK_RESTART_TIMEOUT_SECONDS:-180}"
if ! [[ "${restart_timeout}" =~ ^[1-9][0-9]{0,3}$ ]] || (( restart_timeout > 3600 )); then
  printf 'restart timeout must be an integer from 1 to 3600 seconds\n' >&2
  exit 1
fi

kube() {
  local remaining=5
  if [[ -n "${restart_deadline:-}" ]]; then
    remaining="$(request_budget "${restart_deadline}")" || return 1
  fi
  kubectl --namespace "${WEBHOOK_RESTART_NAMESPACE}" --request-timeout="${remaining}s" "$@"
}

request_budget() {
  local remaining=$(( $1 - SECONDS ))
  (( remaining > 0 )) || return 1
  if (( remaining > 5 )); then
    remaining=5
  fi
  printf '%s' "${remaining}"
}

source_trigger() {
  local event_id="$1" response status remaining
  local trigger_deadline=$((SECONDS + restart_timeout))
  while (( SECONDS < trigger_deadline )); do
    remaining="$(request_budget "${trigger_deadline}")" || break
    if response="$(curl --silent --show-error --proto '=https' --tlsv1.2 \
      --connect-timeout 3 --max-time "${remaining}" \
      -w $'\n%{http_code}' -X POST -H "Authorization: Bearer ${source_token}" "${producer[@]}" \
      "${WEBHOOKS_INGRESS_URL}/internal/v1/events/${WEBHOOK_SOURCE_KIND}/${event_id}")"; then
      status="${response##*$'\n'}"
      case "${status}" in
        2??) printf '%s' "${response%$'\n'*}"; return 0 ;;
        408|425|429|500|502|503|504) ;;
        *) printf 'source trigger refused with HTTP %s\n' "${status}" >&2; return 1 ;;
      esac
    fi
    sleep 1
  done
  printf 'source trigger deadline exceeded for %s\n' "${event_id}" >&2
  return 1
}

wait_readiness() {
  local readiness_deadline=$((SECONDS + restart_timeout)) public private remaining
  while (( SECONDS < readiness_deadline )); do
    remaining="$(request_budget "${readiness_deadline}")" || break
    if public="$(curl --fail --silent --show-error --proto '=https' --tlsv1.2 \
        --connect-timeout 3 --max-time "${remaining}" "${WEBHOOKS_URL}/healthz")" &&
      jq -e '.ready == true and .role == "public"' <<<"${public}" >/dev/null &&
      remaining="$(request_budget "${readiness_deadline}")" &&
      private="$(curl --fail --silent --show-error --proto '=https' --tlsv1.2 \
        --connect-timeout 3 --max-time "${remaining}" "${producer[@]}" "${WEBHOOKS_INGRESS_URL}/healthz")" &&
      jq -e '.ready == true and .role == "ingress"' <<<"${private}" >/dev/null; then
      return 0
    fi
    sleep 1
  done
  printf 'private/public webhook readiness deadline exceeded\n' >&2
  return 1
}

session_cookie="$(<"${WEBHOOK_SESSION_COOKIE_FILE}")"
csrf="$(<"${WEBHOOK_CSRF_FILE}")"
source_token="$(<"${WEBHOOK_SOURCE_TRIGGER_TOKEN_FILE}")"
receiver_token="$(<"${WEBHOOK_RECEIVER_TOKEN_FILE}")"
producer=(--cacert "${WEBHOOKS_INGRESS_CA_FILE}" --cert "${WEBHOOK_PRODUCER_CERT_FILE}" --key "${WEBHOOK_PRODUCER_KEY_FILE}")

public_internal="$(curl --silent --show-error --proto '=https' --tlsv1.2 \
  -o /dev/null -w '%{http_code}' \
  -X POST -H "Authorization: Bearer ${source_token}" \
  "${WEBHOOKS_URL}/internal/v1/events/${WEBHOOK_SOURCE_KIND}/${WEBHOOK_SOURCE_EVENT_FIRST}")"
[[ "${public_internal}" == "404" ]]

cursor="$(curl --fail --silent --show-error --proto '=https' --tlsv1.2 \
  -H "Cookie: __Host-layerx-session=${session_cookie}" \
  "${WEBHOOKS_URL}/v1/webhooks/endpoints/${WEBHOOK_ENDPOINT_ID}/events?limit=1" | jq -er .next_cursor)"

participant="$(kube get pod "${WEBHOOK_RESTART_POD}" -o json)"
jq -e '
  .metadata.labels.app == "layerx-webhooks" and
  (.metadata.labels.role == "public" or .metadata.labels.role == "ingress") and
  .metadata.deletionTimestamp == null and
  any(.status.conditions[]; .type == "Ready" and .status == "True") and
  any(.metadata.ownerReferences[]; .kind == "ReplicaSet" and .controller == true)
' <<<"${participant}" >/dev/null
participant_uid="$(jq -er .metadata.uid <<<"${participant}")"
participant_role="$(jq -er .metadata.labels.role <<<"${participant}")"
participant_owner="$(jq -er '.metadata.ownerReferences[] | select(.kind == "ReplicaSet" and .controller == true) | .uid' <<<"${participant}")"
existing_uids="$(kube get pods -l "app=layerx-webhooks,role=${participant_role}" -o json | jq -c '[.items[].metadata.uid]')"

source_trigger "${WEBHOOK_SOURCE_EVENT_FIRST}" >/dev/null

restart_deadline=$((SECONDS + restart_timeout))
kube delete pod "${WEBHOOK_RESTART_POD}" --wait=false
restarted=false
while (( SECONDS < restart_deadline )); do
  if participants="$(kube get pods -l "app=layerx-webhooks,role=${participant_role}" -o json)" &&
    jq -e --arg previous "${participant_uid}" --arg owner "${participant_owner}" --argjson existing "${existing_uids}" '
      all(.items[]; .metadata.uid != $previous) and
      any(.items[];
        .metadata.uid as $uid |
        ($existing | index($uid)) == null and
        .metadata.deletionTimestamp == null and
        any(.metadata.ownerReferences[]; .uid == $owner and .controller == true) and
        any(.status.conditions[]; .type == "Ready" and .status == "True") and
        any(.status.containerStatuses[]; .name == "webhooks" and .ready == true and .state.running != null)
      )
    ' <<<"${participants}" >/dev/null; then
    restarted=true
    break
  fi
  sleep 1
done
[[ "${restarted}" == true ]]

wait_readiness
source_trigger "${WEBHOOK_SOURCE_EVENT_SECOND}" >/dev/null

duplicate="$(source_trigger "${WEBHOOK_SOURCE_EVENT_FIRST}")"
jq -e '.duplicate == true' <<<"${duplicate}" >/dev/null

stale_status="$(curl --silent --show-error --proto '=https' --tlsv1.2 \
  -o /dev/null -w '%{http_code}' \
  -X POST -H "Authorization: Bearer ${source_token}" "${producer[@]}" \
  "${WEBHOOKS_INGRESS_URL}/internal/v1/events/${WEBHOOK_SOURCE_KIND}/${WEBHOOK_SOURCE_EVENT_STALE}")"
[[ "${stale_status}" == "409" ]]

deadline="$((SECONDS + 180))"
while (( SECONDS < deadline )); do
  observations="$(curl --fail --silent --show-error --proto '=https' --tlsv1.2 \
    -H "Authorization: Bearer ${receiver_token}" \
    "${WEBHOOK_RECEIVER_OBSERVATIONS_URL}?endpoint=${WEBHOOK_ENDPOINT_ID}")"
  if jq -e --arg first "${WEBHOOK_SOURCE_EVENT_FIRST}" --arg second "${WEBHOOK_SOURCE_EVENT_SECOND}" \
    '([.deliveries[] | select(.source_event_id == $first)] | length) > 0 and ([.deliveries[] | select(.source_event_id == $second)] | length) > 0' \
    <<<"${observations}" >/dev/null; then
    break
  fi
  sleep 2
done

jq -e --arg first "${WEBHOOK_SOURCE_EVENT_FIRST}" --arg second "${WEBHOOK_SOURCE_EVENT_SECOND}" '
  [.deliveries[] | select(.source_event_id == $first or .source_event_id == $second)] as $selected |
  ($selected | length) >= 2 and
  ([range(0; ($selected | length)) | select($selected[.].source_event_id == $first)] | min) <
    ([range(0; ($selected | length)) | select($selected[.].source_event_id == $second)] | min) and
  ([$selected[] | {event_id, body_digest}] | group_by(.event_id) | all(map(.body_digest) | unique | length == 1))
' <<<"${observations}" >/dev/null

curl --fail --silent --show-error --proto '=https' --tlsv1.2 \
  -X POST \
  -H "Cookie: __Host-layerx-session=${session_cookie}" \
  -H "X-LayerX-CSRF: ${csrf}" \
  -H "Idempotency-Key: fault-redelivery-${WEBHOOK_SOURCE_EVENT_FIRST}" \
  "${WEBHOOKS_URL}/v1/webhooks/endpoints/${WEBHOOK_ENDPOINT_ID}/redeliveries?cursor=${cursor}&limit=200" >/dev/null

deadline="$((SECONDS + 180))"
while (( SECONDS < deadline )); do
  observations="$(curl --fail --silent --show-error --proto '=https' --tlsv1.2 \
    -H "Authorization: Bearer ${receiver_token}" \
    "${WEBHOOK_RECEIVER_OBSERVATIONS_URL}?endpoint=${WEBHOOK_ENDPOINT_ID}")"
  if jq -e --arg first "${WEBHOOK_SOURCE_EVENT_FIRST}" \
    '([.deliveries[] | select(.source_event_id == $first)] | length) >= 2' \
    <<<"${observations}" >/dev/null; then
    break
  fi
  sleep 2
done

jq -e --arg first "${WEBHOOK_SOURCE_EVENT_FIRST}" '
  [.deliveries[] | select(.source_event_id == $first)] as $duplicates |
  ($duplicates | length) >= 2 and
  ([$duplicates[].body_digest] | unique | length) == 1
' <<<"${observations}" >/dev/null

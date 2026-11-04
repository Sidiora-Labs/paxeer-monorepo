#!/bin/bash

if [[ $# -lt 4 || $# -gt 5 ]]; then
  echo "Usage: $0 ITER SLEEP NUMBLOCKS NODEADDR [CONTAINER_NAME_FILTER]" >&2
  exit 2
fi

ITER=$1
SLEEP=$2
NUMBLOCKS=$3
NODEADDR=$4
CONTAINER_FILTER=${5-'^pax-node-[0-9]+$|simd'}
CNT=0

# Canonical decimal inputs also keep Bash arithmetic within signed 64-bit range.
if [[ ! "$ITER" =~ ^[1-9][0-9]{0,5}$ ||
      ! "$SLEEP" =~ ^(0|[1-9][0-9]{0,5})$ ||
      ! "$NUMBLOCKS" =~ ^(0|[1-9][0-9]{0,17})$ ]]; then
  echo "Configuration error: ITER must be 1..999999, SLEEP 0..999999 seconds, NUMBLOCKS 0..999999999999999999 (decimal, no leading zeros)." >&2
  exit 2
fi
if [[ -z "$NODEADDR" || -z "$CONTAINER_FILTER" ]]; then
  echo "Configuration error: provide a node address and a nonempty Docker container name filter." >&2
  exit 2
fi

if ! container_output=$(timeout --kill-after=1 5 docker ps -q \
  --filter "name=$CONTAINER_FILTER" --format='{{.Names}}'); then
  echo "Configuration error: Docker target discovery failed or timed out; check the Docker daemon and container name filter." >&2
  exit 2
fi
if [[ -z "$container_output" ]]; then
  echo "Configuration error: no running containers match '$CONTAINER_FILTER'; start the localnet (make -f chain.mk docker-cluster-start) or pass CONTAINER_NAME_FILTER as argument 5." >&2
  exit 2
fi
mapfile -t docker_containers <<< "$container_output"

while (( CNT < ITER )); do
  # HTTP and JSON failures consume an iteration, just like a low block height.
  if status_response=$(curl --fail --silent --show-error \
    --connect-timeout 2 --max-time 5 --max-filesize 1048576 \
    --proto '=http,https' --write-out '\n%{http_code}' \
    -- "$NODEADDR:26657/status"); then
    http_code=${status_response##*$'\n'}
    status_body=${status_response%$'\n'*}
    if [[ "$http_code" == 200 ]] && curr_block=$(printf '%s' "$status_body" | jq -ers '
      if length == 1 then .[0] else error("expected one status response") end
      | if type == "object" and .error == null then .result.sync_info.latest_block_height
        else error("invalid status response") end
      | if type == "string" and test("^(0|[1-9][0-9]{0,17})\\z") then .
        else error("invalid block height") end
    '); then
      echo "Number of Blocks: ${curr_block}"
      if (( curr_block > NUMBLOCKS )); then
        echo "Number of blocks reached. Success!"
        exit 0
      fi
    else
      echo "Invalid HTTP/JSON status response; counting failed observation." >&2
    fi
  else
    echo "Status request failed or timed out; counting failed observation." >&2
  fi

  # Emulate network chaos every ten observations, including the first.
  if (( CNT % 10 == 0 )); then
    rand_container=${docker_containers[RANDOM % ${#docker_containers[@]}]}
    echo "Restarting random docker container ${rand_container}"
    if ! timeout --kill-after=1 5 docker restart -- "$rand_container" >/dev/null; then
      echo "Container restart failed or timed out: $rand_container" >&2
    fi
  fi
  CNT=$((CNT + 1))
  if (( CNT < ITER )); then
    sleep "$SLEEP" || exit 1
  fi
done
echo "Timeout reached. Failure!"
exit 1

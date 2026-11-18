#!/usr/bin/env bash
# Checks preset selection and deployment values through the image entrypoint.
set -euo pipefail

repo="$(cd "$(dirname "$0")/../../../.." && pwd)"
entrypoint="$repo/apps/explorer/deploy/scripts/entrypoint.sh"
dockerfile="$repo/docker/explorer-frontend/Dockerfile"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fail=0
status=0

check() {
  local description="$1"
  shift
  if ! "$@"; then
    echo "FAIL $description"
    fail=1
  fi
}

exported_value_is() {
  local entry
  while IFS= read -r -d '' entry; do
    if [[ $entry == "$1="* ]]; then
      [[ ${entry#*=} == "$2" ]]
      return
    fi
  done < "$tmp/stdout"
  return 1
}

not_exported() {
  local entry
  while IFS= read -r -d '' entry; do
    if [[ $entry == "$1="* ]]; then
      return 1
    fi
  done < "$tmp/stdout"
}

run_preset() {
  local directory="$1" preset="$2"
  shift 2
  status=0
  (cd "$directory" && env -i PATH="$PATH" ENVS_PRESET="$preset" "$@" bash "$tmp/load.sh") \
    > "$tmp/stdout" 2> "$tmp/stderr" || status=$?
}

check 'entrypoint marks the external asset download' grep -qFx '# Download external assets' "$entrypoint"
awk '/^# Download external assets/ { exit } { print }' "$entrypoint" > "$tmp/load.sh"
printf '\nenv -0\n' >> "$tmp/load.sh"

deployment_names=(
  NEXT_PUBLIC_APP_PROTOCOL
  NEXT_PUBLIC_APP_HOST
  NEXT_PUBLIC_APP_PORT
  NEXT_PUBLIC_APP_ENV
  NEXT_PUBLIC_API_WEBSOCKET_PROTOCOL
  NEXT_PUBLIC_API_HOST
  NEXT_PUBLIC_ASSISTANT_ENABLED
)

mkdir -p "$tmp/sample/configs/envs" "$tmp/paxeer/configs/envs"
sample="$tmp/sample/configs/envs/sample-1.test_name.env"
for name in "${deployment_names[@]}"; do
  printf '%s=preset-value\n' "$name" >> "$sample"
done
cat >> "$sample" <<'PRESET'
# Public preset values
; Preset comments are ignored
NEXT_PUBLIC_NETWORK_NAME=preset-network
NEXT_PUBLIC_APP_HOST_EXTRA=preset-extra
NEXT_PUBLIC_APP=preset-short-name
NEXT_PUBLIC_EQUALS=left=middle=right
NEXT_PUBLIC_QUOTED='quoted value'
PRIVATE_PRESET_VALUE=private-value
PRESET
printf '  NEXT_PUBLIC_INDENTED=  value with spaces  \n' >> "$sample"

run_preset "$tmp/sample" sample-1.test_name \
  NEXT_PUBLIC_NETWORK_NAME=process-network NEXT_PUBLIC_APP_HOST_EXTRA=process-extra NEXT_PUBLIC_APP=process-short-name
check 'tracked-form preset loads successfully' test "$status" -eq 0
check 'tracked-form preset has no error' test ! -s "$tmp/stderr"
for name in "${deployment_names[@]}"; do
  check "$name takes its preset value when absent from the process" exported_value_is "$name" preset-value
done
check 'other public values replace process values' exported_value_is NEXT_PUBLIC_NETWORK_NAME preset-network
check 'names containing a deployment name are replaced' exported_value_is NEXT_PUBLIC_APP_HOST_EXTRA preset-extra
check 'partial deployment names are replaced' exported_value_is NEXT_PUBLIC_APP preset-short-name
check 'values retain additional equals signs' exported_value_is NEXT_PUBLIC_EQUALS left=middle=right
check 'leading name whitespace is trimmed and value whitespace retained' exported_value_is NEXT_PUBLIC_INDENTED '  value with spaces  '
check 'value quotes are retained' exported_value_is NEXT_PUBLIC_QUOTED "'quoted value'"
check 'private preset names are not exported' not_exported PRIVATE_PRESET_VALUE

process_values=()
empty_values=()
for name in "${deployment_names[@]}"; do
  process_values+=("$name=process-value")
  empty_values+=("$name=")
done
run_preset "$tmp/sample" sample-1.test_name "${process_values[@]}"
check 'preset loads with deployment values' test "$status" -eq 0
for name in "${deployment_names[@]}"; do
  check "$name keeps a process value" exported_value_is "$name" process-value
done
run_preset "$tmp/sample" sample-1.test_name "${empty_values[@]}"
check 'preset loads with empty deployment values' test "$status" -eq 0
for name in "${deployment_names[@]}"; do
  check "$name keeps an empty process value" exported_value_is "$name" ''
done

printf 'NEXT_PUBLIC_NETWORK_NAME=dot-network\n' > "$tmp/sample/configs/envs/.env.sample-1.test_name"
run_preset "$tmp/sample" sample-1.test_name
check 'preset loads when both filename forms exist' test "$status" -eq 0
check 'dot-form preset takes precedence' exported_value_is NEXT_PUBLIC_NETWORK_NAME dot-network
check 'tracked-form values are not merged into the dot form' not_exported NEXT_PUBLIC_ASSISTANT_ENABLED

printf 'NEXT_PUBLIC_NETWORK_NAME=empty-name-network\n' > "$tmp/sample/configs/envs/.env."
printf 'NEXT_PUBLIC_NETWORK_NAME=empty-name-network\n' > "$tmp/sample/configs/envs/.env"
printf 'NEXT_PUBLIC_NETWORK_NAME=none-network\n' > "$tmp/sample/configs/envs/.env.none"
printf 'NEXT_PUBLIC_NETWORK_NAME=none-network\n' > "$tmp/sample/configs/envs/none.env"
for preset in '' none; do
  run_preset "$tmp/sample" "$preset" NEXT_PUBLIC_NETWORK_NAME=process-network
  check "preset '$preset' exits zero" test "$status" -eq 0
  check "preset '$preset' reads nothing" exported_value_is NEXT_PUBLIC_NETWORK_NAME process-network
  check "preset '$preset' has no error" test ! -s "$tmp/stderr"
done

# Even an existing file cannot select a preset through a slash in its name.
mkdir -p "$tmp/sample/configs/envs/.env.nested"
printf 'NEXT_PUBLIC_NETWORK_NAME=nested-network\n' > "$tmp/sample/configs/envs/.env.nested/name"
for preset in missing nested/name; do
  run_preset "$tmp/sample" "$preset"
  check "preset '$preset' exits non-zero" test "$status" -ne 0
  check "preset '$preset' reports its name" grep -qF "$preset" "$tmp/stderr"
  check "preset '$preset' reports the dot-form path" grep -qF "./configs/envs/.env.$preset" "$tmp/stderr"
  check "preset '$preset' reports the tracked-form path" grep -qF "./configs/envs/$preset.env" "$tmp/stderr"
  check "preset '$preset' reports exactly one error line" test "$(wc -l < "$tmp/stderr")" -eq 1
  check "preset '$preset' exits before subsequent commands" test ! -s "$tmp/stdout"
done

check 'Paxeer X production preset names no application port' \
  test "$(grep -c '^NEXT_PUBLIC_APP_PORT' "$repo/apps/explorer/configs/envs/paxeer-x.env")" -eq 0
check 'Paxeer X development preset names application port 3000' \
  grep -qFx 'NEXT_PUBLIC_APP_PORT=3000' "$repo/apps/explorer/configs/envs/paxeer-x-dev.env"

for preset in paxeer-x paxeer-x-dev; do
  check "$preset preset names no default theme" \
    test "$(grep -c '^NEXT_PUBLIC_COLOR_THEME_DEFAULT' "$repo/apps/explorer/configs/envs/$preset.env")" -eq 0
done

cp "$repo/apps/explorer/configs/envs/paxeer-x.env" "$tmp/paxeer/configs/envs/paxeer-x.env"
run_preset "$tmp/paxeer" paxeer-x
check 'tracked Paxeer X preset loads successfully' test "$status" -eq 0
check 'Paxeer X exports no default theme without a process value' not_exported NEXT_PUBLIC_COLOR_THEME_DEFAULT
check 'Paxeer X exports no application port without a process value' not_exported NEXT_PUBLIC_APP_PORT
check 'Paxeer X network ID is 125' exported_value_is NEXT_PUBLIC_NETWORK_ID 125
check 'Paxeer X assistant defaults to false' exported_value_is NEXT_PUBLIC_ASSISTANT_ENABLED false
check 'Paxeer X private assistant settings are not exported' not_exported EXPLORER_ASSISTANT_MODEL
run_preset "$tmp/paxeer" paxeer-x NEXT_PUBLIC_COLOR_THEME_DEFAULT=dark
check 'Paxeer X preset loads with a deployment default theme' test "$status" -eq 0
check 'Paxeer X default theme keeps the deployment value' exported_value_is NEXT_PUBLIC_COLOR_THEME_DEFAULT dark
run_preset "$tmp/paxeer" paxeer-x NEXT_PUBLIC_APP_PORT=8443
check 'Paxeer X preset loads with a deployment application port' test "$status" -eq 0
check 'Paxeer X application port keeps the deployment value' exported_value_is NEXT_PUBLIC_APP_PORT 8443
run_preset "$tmp/paxeer" paxeer-x NEXT_PUBLIC_ASSISTANT_ENABLED=true NEXT_PUBLIC_API_HOST=localhost
check 'Paxeer X preset loads with deployment overrides' test "$status" -eq 0
check 'Paxeer X assistant keeps a deployment opt-in' exported_value_is NEXT_PUBLIC_ASSISTANT_ENABLED true
check 'Paxeer X API host keeps the deployment value' exported_value_is NEXT_PUBLIC_API_HOST localhost
check 'Paxeer X network ID remains 125 with deployment overrides' exported_value_is NEXT_PUBLIC_NETWORK_ID 125

check 'image copies tracked presets to configs/envs' \
  grep -qE '^COPY[[:space:]]+apps/explorer/configs/envs[[:space:]]+\./configs/envs[[:space:]]*$' "$dockerfile"
check 'image copies the entrypoint' \
  grep -qE '^COPY[[:space:]]+(--chmod=[0-7]+[[:space:]]+)?apps/explorer/deploy/scripts/entrypoint\.sh[[:space:]]+\.[[:space:]]*$' "$dockerfile"

if [ "$fail" -eq 0 ]; then
  echo 'explorer preset check: ok'
else
  echo 'explorer preset check: failed'
fi
exit "$fail"

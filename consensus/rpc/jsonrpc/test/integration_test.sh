#!/usr/bin/env bash
set -euo pipefail

# Get the directory of where this script is.
SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ]; do SOURCE="$(readlink "$SOURCE")"; done
DIR="$(cd -P "$(dirname "$SOURCE")" && pwd)"
cd "$DIR"

WORK_DIR=$(mktemp -d)
: >"$WORK_DIR/server.log"
PID=""
cleanup() {
    status=$?
    trap - EXIT
    if [[ -n "$PID" ]]; then
        if kill -0 "$PID" 2>/dev/null; then
            kill -TERM "$PID" || true
        fi
        wait "$PID" || true
    fi
    if (( status != 0 )); then
        cat "$WORK_DIR/server.log" >&2
    fi
    rm -rf "$WORK_DIR"
    exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

# Compare parsed JSON objects, independently of whitespace and key order.
assert_json() {
    jq -es --argjson expected "$2" 'length == 1 and .[0] == $expected' <<<"$1" >/dev/null
}
assert_invalid_params() {
    jq -es 'length == 1 and (.[0] | keys == ["code","data","message"] and
        .code == -32602 and .message == "Invalid params" and
        (.data | type == "string" and length > 0))' <<<"$1" >/dev/null
}
get_uri() {
    curl --silent --show-error --fail --max-time 5 --get \
        --data-urlencode "name=$1" --data-urlencode "num=$2" \
        "$BASE_URL/hello_world"
}

echo "==> Building the server"
go build -o "$WORK_DIR/rpcserver" main.go

echo "==> Starting the server"
"$WORK_DIR/rpcserver" -listen tcp://127.0.0.1:0 >"$WORK_DIR/server.log" 2>&1 &
PID=$!
ready=false
for ((attempt=0; attempt<50; attempt++)); do
    if ! kill -0 "$PID" 2>/dev/null; then
        echo "server exited during startup" >&2
        exit 1
    fi
    BASE_URL=$(sed -n 's/^RPC listening on //p' "$WORK_DIR/server.log")
    if [[ -n "$BASE_URL" ]]; then
        ready=true
        break
    fi
    sleep 0.1
done
[[ "$ready" == true ]]
kill -0 "$PID"

echo "==> simple request: URI result and JSON-RPC envelope"
assert_json "$(get_uri '"my_world"' 5)" '{"Result":"hi my_world 5"}'
assert_json "$(curl --silent --show-error --fail --max-time 5 \
    -H 'Content-Type: application/json' --data @data.json "$BASE_URL/")" \
    '{"jsonrpc":"2.0","id":"","result":{"Result":"hi my_world 5"}}'

echo "==> request with 0x-prefixed hex string arg"
assert_json "$(get_uri 0x41424344 123)" '{"Result":"hi ABCD 123"}'

echo "==> request with missing params"
assert_json "$(curl --silent --show-error --fail --max-time 5 \
    "$BASE_URL/hello_world")" '{"Result":"hi  0"}'

echo "==> request with unquoted string arg"
assert_json "$(get_uri abcd 123)" '{"Result":"hi abcd 123"}'

echo "==> request with string type when expecting number arg"
assert_invalid_params "$(get_uri '"abcd"' 0xabcd)"
RPC_ERROR=$(curl --silent --show-error --fail --max-time 5 \
    -H 'Content-Type: application/json' \
    --data '{"jsonrpc":"2.0","id":7,"method":"hello_world","params":{"name":"abcd","num":"0xabcd"}}' \
    "$BASE_URL/")
jq -e 'keys == ["error","id","jsonrpc"] and .jsonrpc == "2.0" and .id == 7' <<<"$RPC_ERROR" >/dev/null
assert_invalid_params "$(jq '.error' <<<"$RPC_ERROR")"

echo "OK"

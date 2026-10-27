#!/usr/bin/env bash
set -Eeuo pipefail
umask 077

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)
BASE=$(realpath -m "${TMPDIR:-/tmp}/lx-localnode")
ENV_FILE=$BASE/localnode.env
STATE=
STOP=0
SUCCESS=0

fail() {
    echo "perps-gateway: $*" >&2
    exit 1
}

stop_gateway() {
    local dir=$1 role pid command attempt
    dir=$(realpath -e "$dir")
    [[ "$dir" == "$BASE"/gateway.* ]] && [ -f "$dir/.local-perps-gateway" ] \
        || fail 'not a disposable gateway home'
    for role in gateway nats-1 nats-2 nats-3; do
        [ -f "$dir/$role.pid" ] || continue
        pid=$(cat "$dir/$role.pid")
        [[ "$pid" =~ ^[1-9][0-9]*$ ]] || fail "invalid $role pid"
        if kill -0 "$pid" 2>/dev/null; then
            [ "$(awk '{print $3}' "/proc/$pid/stat" 2>/dev/null)" != Z ] || continue
            command=$(tr '\0' ' ' < "/proc/$pid/cmdline")
            [[ "$command" == *"$dir/"* ]] || fail "$role pid no longer belongs to this gateway"
            [ "$(ps -o pgid= -p "$pid" | tr -d ' ')" = "$pid" ] || fail "invalid $role process group"
            kill -TERM -- "-$pid"
            for ((attempt = 0; attempt < 100; attempt++)); do
                kill -0 -- "-$pid" 2>/dev/null || break
                sleep 0.1
            done
            if kill -0 -- "-$pid" 2>/dev/null; then
                # A go-run parent can exit before its compiled child finishes shutting down.
                kill -KILL -- "-$pid" 2>/dev/null || true
            fi
        fi
    done
}

cleanup() {
    local status=$?
    if [ "$SUCCESS" = 0 ] && [ -n "$STATE" ]; then
        echo "perps-gateway: startup failed; private evidence: $STATE" >&2
        (stop_gateway "$STATE") || true
    fi
    return "$status"
}
trap cleanup EXIT

while [ "$#" -gt 0 ]; do
    case "$1" in
        --env) [ "$#" -ge 2 ] || fail '--env requires a file'; ENV_FILE=$2; shift 2 ;;
        --stop)
            STOP=1
            shift
            if [ "$#" -gt 0 ] && [[ "$1" != --* ]]; then STATE=$1; shift; fi
            ;;
        *) fail "unknown argument: $1" ;;
    esac
done
ENV_FILE=$(realpath -e "$ENV_FILE")
[[ "$ENV_FILE" == "$BASE"/* ]] || fail "env file must be beneath $BASE"
[[ "$BASE/" != "$ROOT/"* ]] || fail 'gateway state must be outside the repository'
if [ "$STOP" = 1 ] && [ -n "$STATE" ]; then
    stop_gateway "$STATE"
    SUCCESS=1
    exit
fi
set -a
. "$ENV_FILE"
set +a
if [ "$STOP" = 1 ]; then
    [ -n "${LOCAL_PERPS_GATEWAY_HOME:-}" ] || fail 'env file has no gateway home'
    stop_gateway "$LOCAL_PERPS_GATEWAY_HOME"
    SUCCESS=1
    exit
fi
for tool in go python3 curl jq openssl nats-server setsid timeout; do
    command -v "$tool" >/dev/null || fail "$tool is required"
done
[ -n "${PERPS_DATA_TEST_RPC:-}" ] || fail 'node env must set PERPS_DATA_TEST_RPC'
[ -n "${PERPS_DATA_TEST_DATABASE_URL:-}" ] || fail 'PERPS_DATA_TEST_DATABASE_URL must name a local disposable Postgres database'
python3 - <<'PY'
import os
from urllib.parse import urlsplit
for key, schemes in [('PERPS_DATA_TEST_RPC', ('http', 'https')),
                     ('PERPS_DATA_TEST_DATABASE_URL', ('postgres', 'postgresql'))]:
    u = urlsplit(os.environ[key])
    if u.scheme not in schemes or u.hostname not in ('127.0.0.1', 'localhost', '::1'):
        raise SystemExit(f'{key} must use a loopback host and a supported scheme')
PY
if [ -n "${LOCAL_PERPS_GATEWAY_HOME:-}" ]; then
    stop_gateway "$LOCAL_PERPS_GATEWAY_HOME"
fi
mkdir -p "$BASE"
STATE=$(mktemp -d "$BASE/gateway.XXXXXXXX")
touch "$STATE/.local-perps-gateway"
read -r PORT NATS_PORT NATS_PORT_2 NATS_PORT_3 MONITOR_PORT MONITOR_PORT_2 MONITOR_PORT_3 CLUSTER_PORT CLUSTER_PORT_2 CLUSTER_PORT_3 < <(python3 - <<'PORTS'
import socket
listeners = [socket.socket() for _ in range(10)]
for listener in listeners:
    listener.bind(('127.0.0.1', 0))
print(*(listener.getsockname()[1] for listener in listeners))
for listener in listeners:
    listener.close()
PORTS
)
export PORT PERPS_CHAIN_ID=125 PERPS_RPC_URLS="$PERPS_DATA_TEST_RPC"
export DATABASE_URL="$PERPS_DATA_TEST_DATABASE_URL" NATS_URL="nats://127.0.0.1:$NATS_PORT"
export PERPS_AUTH_SECRET="$(openssl rand -hex 32)"
NATS_PORTS=("$NATS_PORT" "$NATS_PORT_2" "$NATS_PORT_3")
MONITOR_PORTS=("$MONITOR_PORT" "$MONITOR_PORT_2" "$MONITOR_PORT_3")
CLUSTER_PORTS=("$CLUSTER_PORT" "$CLUSTER_PORT_2" "$CLUSTER_PORT_3")
# PERPS_V1 requires three replicas; all three peers stay on loopback.
for index in 0 1 2; do
    role=nats-$((index + 1))
    cat > "$STATE/$role.conf" <<EOF
server_name: "$(basename "$STATE")-$role"
host: 127.0.0.1
port: ${NATS_PORTS[$index]}
http: 127.0.0.1:${MONITOR_PORTS[$index]}
jetstream { store_dir: "$STATE/$role/jetstream" }
cluster {
    name: "$(basename "$STATE")"
    listen: "127.0.0.1:${CLUSTER_PORTS[$index]}"
    routes: [
EOF
    for peer in 0 1 2; do
        [ "$peer" = "$index" ] || printf '        "nats://127.0.0.1:%s",\n' "${CLUSTER_PORTS[$peer]}" >> "$STATE/$role.conf"
    done
    printf '    ]\n}\n' >> "$STATE/$role.conf"
    nohup setsid nats-server -c "$STATE/$role.conf" > "$STATE/$role.log" 2>&1 < /dev/null &
    echo "$!" > "$STATE/$role.pid"
done
deadline=$((SECONDS + 60))
for index in 0 1 2; do
    role=nats-$((index + 1))
    while :; do
        for peer in 1 2 3; do
            kill -0 "$(cat "$STATE/nats-$peer.pid")" 2>/dev/null || fail "NATS peer exited; see $STATE/nats-$peer.log"
        done
        if curl -fsS --max-time 2 "http://127.0.0.1:${MONITOR_PORTS[$index]}/healthz" > /dev/null 2>&1 \
            && curl -fsS --max-time 2 "http://127.0.0.1:${MONITOR_PORTS[$index]}/jsz" > "$STATE/$role-jsz.json" 2>/dev/null \
            && jq -e '(.meta_cluster.leader // "" | length) > 0' "$STATE/$role-jsz.json" > /dev/null; then
            break
        fi
        (( SECONDS < deadline )) || fail "NATS cluster readiness timed out; see $STATE/$role.log"
        sleep 0.2
    done
done
cat > "$STATE/bootstrap.go" <<'GO'
package main

import (
    "context"
    "fmt"
    "log"
    "math/big"
    "os"
    "time"

    "github.com/ethereum/go-ethereum/ethclient"
    "github.com/nats-io/nats.go"
    "github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/markets"
    "github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/publish"
    "github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/store"
)

func main() {
    if err := bootstrap(); err != nil { log.Fatal(err) }
}

func bootstrap() error {
    ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
    defer cancel()
    client, err := ethclient.DialContext(ctx, os.Getenv("PERPS_RPC_URLS"))
    if err != nil { return err }
    defer client.Close()
    chain, err := client.ChainID(ctx)
    if err != nil { return err }
    if chain.Cmp(big.NewInt(125)) != 0 { return fmt.Errorf("chain ID = %s, want 125", chain) }
    db, err := store.Open(os.Getenv("DATABASE_URL"))
    if err != nil { return err }
    defer db.Close()
    if err := store.Migrate(ctx, db); err != nil { return err }
    if err := markets.Refresh(ctx, db, client); err != nil { return err }
    nc, err := nats.Connect(os.Getenv("NATS_URL"), nats.Timeout(5*time.Second))
    if err != nil { return err }
    defer nc.Close()
    return publish.EnsureStream(ctx, nc, 125)
}
GO
# The command has no bind-host flag. Overlays keep both the bootstrap and
# the loopback listener outside the checkout while using the real packages.
python3 - "$ROOT" "$STATE" <<'PY'
import json
from pathlib import Path
import sys
root, state = map(Path, sys.argv[1:])
main = root / 'services/platform/perps-data/cmd/perps-gateway/main.go'
source = main.read_text()
old = 'net.Listen("tcp", ":"+strconv.FormatUint(uint64(cfg.Port), 10))'
new = 'net.Listen("tcp", "127.0.0.1:"+strconv.FormatUint(uint64(cfg.Port), 10))'
if source.count(old) != 1:
    raise SystemExit('gateway listener changed: update the explicit loopback overlay')
(state / 'gateway.go').write_text(source.replace(old, new))
for mode in ('bootstrap', 'gateway'):
    (state / f'{mode}.json').write_text(json.dumps({'Replace': {str(main): str(state / f'{mode}.go')}}))
PY
cd "$ROOT"
timeout 180s go run -overlay "$STATE/bootstrap.json" ./services/platform/perps-data/cmd/perps-gateway \
    > "$STATE/bootstrap.log" 2>&1 || fail "bootstrap failed; see $STATE/bootstrap.log"
nohup setsid go run -overlay "$STATE/gateway.json" ./services/platform/perps-data/cmd/perps-gateway \
    > "$STATE/gateway.log" 2>&1 < /dev/null &
echo "$!" > "$STATE/gateway.pid"
URL=http://127.0.0.1:$PORT
deadline=$((SECONDS + 120))
while :; do
    kill -0 "$(cat "$STATE/gateway.pid")" 2>/dev/null || fail "gateway exited; see $STATE/gateway.log"
    for peer in 1 2 3; do
        kill -0 "$(cat "$STATE/nats-$peer.pid")" 2>/dev/null || fail "NATS peer exited; see $STATE/nats-$peer.log"
    done
    if curl -fsS --max-time 2 "$URL/healthz/ready" > "$STATE/health.json" 2>/dev/null \
        && curl -fsS --max-time 2 -H 'Content-Type: application/json' \
            --data '{"query":"query LocalGatewayHealth { markets { marketId } }"}' \
            "$URL/graphql" > "$STATE/graphql-health.json" 2>/dev/null \
        && jq -e '(.errors // [] | length) == 0 and (.data.markets | length) == 18' \
            "$STATE/graphql-health.json" > /dev/null; then
        break
    fi
    (( SECONDS < deadline )) || fail "gateway readiness timed out; see $STATE/gateway.log"
    sleep 0.2
done
{
    printf '\nLOCAL_PERPS_GATEWAY_HOME=%q\n' "$STATE"
    printf 'NEXT_PUBLIC_PERPS_GATEWAY_URL=%q\n' "$URL"
} >> "$ENV_FILE"
SUCCESS=1
printf 'perps-gateway: ready; url=%s private evidence=%s\n' "$URL" "$STATE" >&2

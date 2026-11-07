#!/usr/bin/env bash
# =============================================================================
# fleet-keygen.sh — install hpx, collect operator identities and verify the fleet.
#
# By default, copy the local hpx to /usr/local/bin/hpx on each selected host,
# then run `hpx validator keygen`. The operator key stays in the node's keyring;
# collection returns public pax1 and validator operator addresses only.
#
# State: HPX_WORKDIR, otherwise .fleet beside this script. Preserve state.json
# and operators.tsv (moniker \t ip \t pax1 \t valoper) between runs. Unreachable
# or changed identities remain unresolved and block funding and staking.
#
# Usage:
#   ./fleet-keygen.sh                      # servers from registry /api/nodes
#   SERVERS='203.0.113.10 203.0.113.11' ./fleet-keygen.sh
#   SERVERS_FILE=ips.txt ./fleet-keygen.sh
#   HPX_PUSH=0 ./fleet-keygen.sh           # skip push (servers already updated)
# =============================================================================
set -uo pipefail

readonly SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly HPX_LOCAL="${HPX_LOCAL:-${SELF_DIR}/hpx}"
readonly WORKDIR="${HPX_WORKDIR:-${SELF_DIR}/.fleet}"
readonly OPERATORS_FILE="${WORKDIR}/operators.tsv"
readonly REMOTE_HPX="/usr/local/bin/hpx"
readonly MIRROR="${HPX_MIRROR:-https://node.hyperpaxeer.com}"
readonly SSH_USER="${HPX_SSH_USER:-root}"
readonly PUSH="${HPX_PUSH:-1}"
readonly SSH="ssh -o StrictHostKeyChecking=accept-new -o ConnectTimeout=12 ${HPX_SSH_OPTS:-}"
readonly SCP="scp -o StrictHostKeyChecking=accept-new -o ConnectTimeout=12 ${HPX_SCP_OPTS:-}"

C_RST=$'\033[0m'; C_B=$'\033[1m'; C_D=$'\033[2m'
C_R=$'\033[0;31m'; C_G=$'\033[0;32m'; C_Y=$'\033[0;33m'; C_C=$'\033[0;36m'
info(){ printf "%b\n" "${C_C}[*]${C_RST} $*"; }
ok(){   printf "%b\n" "${C_G}[+]${C_RST} $*"; }
warn(){ printf "%b\n" "${C_Y}[!]${C_RST} $*"; }
err(){  printf "%b\n" "${C_R}[-]${C_RST} $*" >&2; }
die(){  err "$@"; exit 1; }
hr(){   printf "%b\n" "${C_D}----------------------------------------------------------------------${C_RST}"; }

command -v python3 >/dev/null 2>&1 || die "missing python3"
command -v flock >/dev/null 2>&1 || die "missing flock"
command -v jq   >/dev/null 2>&1 || die "missing jq"
command -v curl >/dev/null 2>&1 || die "missing curl"
command -v ssh  >/dev/null 2>&1 || die "missing ssh"
[ -f "$HPX_LOCAL" ] || die "local hpx not found at $HPX_LOCAL (set HPX_LOCAL=)"
bash -n "$HPX_LOCAL" || die "local hpx has a syntax error — fix before pushing to the fleet"
mkdir -p "$WORKDIR"; chmod 700 "$WORKDIR"

discover_servers(){
    if [ -n "${SERVERS:-}" ]; then printf '%s\n' $SERVERS; return; fi
    if [ -n "${SERVERS_FILE:-}" ] && [ -f "$SERVERS_FILE" ]; then
        grep -oE '([0-9]{1,3}\.){3}[0-9]{1,3}' "$SERVERS_FILE"; return; fi
    curl -fsS --max-time 12 "${MIRROR}/api/nodes" 2>/dev/null | jq -r '.nodes[].ip' 2>/dev/null
}

exec 9>"${WORKDIR}/workflow.lock"
flock -n 9 || die "another fleet workflow owns $WORKDIR"
discovered=$(discover_servers) || die "server discovery failed"
mapfile -t SERVERS_ARR < <(printf '%s\n' "$discovered" | sed '/^$/d' | sort -u)
[ "${#SERVERS_ARR[@]}" -gt 0 ] || die "no servers (set SERVERS='ip ip ...' | SERVERS_FILE=<file> | ensure registry reachable)"

hr; info "${C_B}Fleet keygen${C_RST}  targets=${#SERVERS_ARR[@]}  push=${PUSH}"
info "${SERVERS_ARR[*]}"; hr

roster=$(python3 "${SELF_DIR}/fleet_state.py" "$WORKDIR" begin "${SERVERS_ARR[@]}") || die "cannot persist intended roster"
mapfile -t SERVERS_ARR <<<"$roster"
declare -i okc=0 failc=0
for ip in "${SERVERS_ARR[@]}"; do
    printf "  %-16s " "$ip"
    # 1) push updated hpx (unless disabled)
    if [ "$PUSH" = 1 ]; then
        if ! $SCP "$HPX_LOCAL" "${SSH_USER}@${ip}:${REMOTE_HPX}.new" >/dev/null 2>&1; then
            printf "%b\n" "${C_R}scp failed (unreachable?)${C_RST}"; failc+=1; continue
        fi
        $SSH "${SSH_USER}@${ip}" "chmod +x ${REMOTE_HPX}.new && mv -f ${REMOTE_HPX}.new ${REMOTE_HPX}" >/dev/null 2>&1 \
            || { printf "%b\n" "${C_R}install failed${C_RST}"; failc+=1; continue; }
    fi
    # 2) run keygen, capture the machine-readable OPERATOR line
    line=$($SSH "${SSH_USER}@${ip}" "bash -o pipefail -c 'hpx validator keygen 2>/dev/null | grep \"^OPERATOR \"'" 2>/dev/null)
    if [ "$?" -ne 0 ] || ! python3 "${SELF_DIR}/fleet_state.py" "$WORKDIR" record "$ip" "$line"; then
        printf "%b\n" "${C_R}unresolved; known identity retained${C_RST}"; failc+=1; continue
    fi
    printf "%b\n" "${C_G}verified${C_RST}"; okc+=1
done

python3 "${SELF_DIR}/fleet_state.py" "$WORKDIR" complete || die "incomplete roster; retry unresolved hosts before funding"
hr
ok "keygen OK on ${okc} host(s); ${failc} failed"
ok "operators -> ${OPERATORS_FILE}  (n=$(wc -l < "$OPERATORS_FILE"))"
[ "$failc" -eq 0 ] || warn "re-run to retry failed hosts (idempotent), or fix connectivity"
echo
echo "Next:  ./stake-fleet.sh fund --yes   then   ./stake-fleet.sh stake-cmds"

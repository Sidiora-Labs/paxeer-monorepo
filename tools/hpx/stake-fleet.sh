#!/usr/bin/env bash
# =============================================================================
# stake-fleet.sh — fund operator wallets and bootstrap Paxeer fleet validators.
#
# Commands:
#   fund-wallet : prepare funding wallet W and transfer the founder balance,
#                 retaining the configured fee buffer.
#   keygen      : collect and verify every intended host's operator identity.
#   fund        : distribute W's balance using a persistent allocation journal.
#   stake-cmds  : print per-host validator restart and staking commands.
#   stake       : drive the ratio-aware staking ramp across the fleet.
#   status      : show wallet balances and on-chain validator state.
#
# Association makes the founder's EVM-native funds spendable from its canonical
# pax1 address. `paxd tx evm associate-address --from` uses the protected keyring
# and pays gas from the EVM balance; subsequent funding uses Cosmos bank sends.
#
# Set HPX_RPC and HPX_EVM_RPC to the intended network before invoking a command.
# Money phases default to dry-run; --yes or CONFIRM=YES enables broadcasting.
# Key collection contacts the fleet over SSH. Preserve HPX_WORKDIR (default:
# .fleet beside this script), including its roster, keyring and payment journal,
# across retries. Resolve incomplete identities before funding or staking.
# =============================================================================
set -uo pipefail

# ── Paths / binary ────────────────────────────────────────────────────────────
readonly SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly REPO_ROOT="$(cd "${SELF_DIR}/../.." && pwd)"
readonly ENV_FILE="${HPX_ENV_FILE:-${REPO_ROOT}/.env}"
# The selected trusted shell configuration is loaded first. Explicit shell
# variables take precedence over file values, which take precedence over defaults.
declare -A shell_overrides=()
for name in ${!HPX_@} PAXD SERVERS SERVERS_FILE CONFIRM MNEMONIC_PHRASE; do
    if [[ -v "$name" ]]; then shell_overrides["$name"]="${!name}"; fi
done
if [ -f "$ENV_FILE" ]; then
    . "$ENV_FILE" || { echo "cannot load selected env file: $ENV_FILE" >&2; exit 1; }
elif [ -n "${HPX_ENV_FILE:-}" ]; then
    echo "selected env file not found: $ENV_FILE" >&2; exit 1
fi
for name in "${!shell_overrides[@]}"; do
    printf -v "$name" '%s' "${shell_overrides[$name]}"
done
unset shell_overrides name
if [[ -v MNEMONIC_PHRASE ]]; then export -n MNEMONIC_PHRASE; fi
readonly STATE="${SELF_DIR}/fleet_state.py"
readonly PAXD="${PAXD:-/usr/local/bin/paxd}"
readonly WORKDIR="${HPX_WORKDIR:-${SELF_DIR}/.fleet}"
readonly KDIR="${HPX_KEYRING_DIR:-${WORKDIR}/keyring}"
readonly OPERATORS_FILE="${WORKDIR}/operators.tsv"   # moniker \t ip \t pax1 \t valoper
readonly W_MNEMONIC_FILE="${WORKDIR}/funding-wallet.mnemonic"
readonly W_ADDR_FILE="${WORKDIR}/funding-wallet.addr"

# ── Chain / endpoints (override via env) ──────────────────────────────────────
readonly CHAIN_ID="${HPX_CHAIN_ID:-hyperpax_125-1}"
readonly RPC="${HPX_RPC:?HPX_RPC is required}"
readonly EVM_RPC="${HPX_EVM_RPC:?HPX_EVM_RPC is required}"
readonly DENOM="uhpx"
readonly UHPX_PER_PAX=1000000                 # 1 PAX = 10^6 uhpx (6-dec cosmos base)
readonly GAS_PRICES="${HPX_GAS_PRICES:-0.1uhpx}"
readonly KEYRING_BACKEND="${HPX_KEYRING_BACKEND:-file}"
readonly KB=(--keyring-backend "$KEYRING_BACKEND" --keyring-dir "$KDIR")

# ── Key names inside the local temp keyring ───────────────────────────────────
readonly SRC_KEY="founder-src"                # founder wallet (81M), coin-type 60 (EVM)
readonly W_KEY="funding-w"                     # new funding wallet, coin-type 118 (cosmos)

# ── Distribution knobs ────────────────────────────────────────────────────────
readonly BUFFER_PAX="${HPX_BUFFER_PAX:-1000}"       # PAX kept in W as a fee buffer
readonly SSH_USER="${HPX_SSH_USER:-root}"
readonly SSH="ssh -o StrictHostKeyChecking=accept-new -o ConnectTimeout=12 ${HPX_SSH_OPTS:-}"
readonly MIRROR="${HPX_MIRROR:-https://node.hyperpaxeer.com}"

# ── UI ────────────────────────────────────────────────────────────────────────
C_RST=$'\033[0m'; C_B=$'\033[1m'; C_D=$'\033[2m'
C_R=$'\033[0;31m'; C_G=$'\033[0;32m'; C_Y=$'\033[0;33m'; C_C=$'\033[0;36m'
info(){ printf "%b\n" "${C_C}[*]${C_RST} $*"; }
ok(){   printf "%b\n" "${C_G}[+]${C_RST} $*"; }
warn(){ printf "%b\n" "${C_Y}[!]${C_RST} $*"; }
err(){  printf "%b\n" "${C_R}[-]${C_RST} $*" >&2; }
die(){  err "$@"; exit 1; }
hr(){   printf "%b\n" "${C_D}----------------------------------------------------------------------${C_RST}"; }

# ── Global flags ──────────────────────────────────────────────────────────────
YES=0; [ "${CONFIRM:-}" = "YES" ] && YES=1
for a in "$@"; do [ "$a" = "--yes" ] && YES=1; done
broadcasting(){ [ "$YES" = 1 ]; }

need(){ command -v "$1" >/dev/null 2>&1 || die "missing dependency: $1"; }
preflight(){
    need jq; need curl; need ssh; need python3; need flock
    [ -x "$PAXD" ] || die "paxd not found at $PAXD (set PAXD=)"
    mkdir -p "$WORKDIR" "$KDIR"; chmod 700 "$WORKDIR"
    exec 9>"${WORKDIR}/workflow.lock"
    flock -n 9 || die "another fleet workflow owns $WORKDIR"
}

# ── Helpers ───────────────────────────────────────────────────────────────────
pax_to_uhpx(){ echo $(( $1 * UHPX_PER_PAX )); }

bank_uhpx(){   # $1 = pax1 address -> integer uhpx (0 if none)
    "$PAXD" q bank balances "$1" --node "$RPC" -o json 2>/dev/null \
        | jq -r --arg d "$DENOM" '(.balances[]? | select(.denom==$d) | .amount) // "0"' 2>/dev/null | head -n1
}
key_addr(){ "$PAXD" keys show "$1" -a "${KB[@]}" 2>/dev/null; }
key_exists(){ "$PAXD" keys show "$1" -a "${KB[@]}" >/dev/null 2>&1; }

# ─────────────────────────────────────────────────────────────────────────────
# PHASE 1: fund-wallet — new seed, associate founder, move 81M into W
# ─────────────────────────────────────────────────────────────────────────────
phase_fund_wallet(){
    preflight
    hr; info "${C_B}PHASE 1 — funding wallet setup${C_RST}"; hr

    # 1a. import founder (81M) as coin-type 60 (matches Paxport m/44'/60'/0'/0/0)
    if ! broadcasting; then
        local public_src
        public_src=$(key_addr "$SRC_KEY") || die "import founder into the protected keyring before preview"
        info "[dry-run] founder ${public_src}; cosmos balance $(bank_uhpx "$public_src") ${DENOM}"
        if key_exists "$W_KEY"; then
            local public_w; public_w=$(key_addr "$W_KEY")
            info "[dry-run] funding wallet ${public_w}; balance $(bank_uhpx "$public_w") ${DENOM}"
        fi
        return 0
    fi
    if key_exists "$SRC_KEY"; then ok "founder key already imported"; else
        [ -n "${MNEMONIC_PHRASE:-}" ] || die "MNEMONIC_PHRASE missing for initial founder import"
        info "importing founder wallet (coin-type 60)"
        printf '%s\n' "$MNEMONIC_PHRASE" | "$PAXD" keys add "$SRC_KEY" --recover --coin-type 60 "${KB[@]}" >/dev/null 2>&1 \
            || die "founder import failed"
        ok "founder imported"
    fi
    local src; src=$(key_addr "$SRC_KEY")
    info "founder pax1 (cosmos): ${C_B}${src}${C_RST}"

    # 1c. associate founder so the 81M becomes cosmos-spendable
    local before; before=$(bank_uhpx "$src")
    if [ "${before:-0}" -gt 0 ] 2>/dev/null; then
        ok "founder already associated (cosmos balance ${before} ${DENOM})"
    else
        warn "founder cosmos balance is 0 — needs association (EVM-native -> cosmos)"
        if broadcasting; then
            info "broadcasting associate-address (EVM tx, gas from founder EVM balance)"
            "$PAXD" tx evm associate-address --from "$SRC_KEY" "${KB[@]}" --evm-rpc "$EVM_RPC" 2>&1 | tail -3 \
                || die "associate-address failed"
            info "waiting for association to settle..."; sleep 8
        else
            warn "[dry-run] would run: paxd tx evm associate-address --from $SRC_KEY --evm-rpc $EVM_RPC"
        fi
    fi

    # 1d. GATE: verify the 81M is now cosmos-visible under founder pax1
    local sbal; sbal=$(bank_uhpx "$src"); sbal=${sbal:-0}
    hr; info "founder cosmos balance: ${C_B}${sbal} ${DENOM}${C_RST} ($(( sbal / UHPX_PER_PAX )) PAX)"
    if [ "$sbal" -le 0 ] 2>/dev/null; then
        if broadcasting; then die "association did not surface a cosmos balance — STOP. Do not proceed; inspect manually."
        else warn "[dry-run] cannot verify until association is broadcast (re-run with --yes)"; return 0; fi
    fi

    # 1e. generate the NEW funding wallet W (clean cosmos-native seed)
    if key_exists "$W_KEY"; then
        ok "funding wallet W already exists"
    else
        info "generating new funding-wallet seed (W)"
        local mn; mn=$("$PAXD" keys mnemonic 2>/dev/null)
        [ -n "$mn" ] || die "mnemonic generation failed"
        printf '%s\n' "$mn" | "$PAXD" keys add "$W_KEY" --recover --coin-type 118 "${KB[@]}" >/dev/null 2>&1 \
            || die "W import failed"
        umask 077; printf '%s\n' "$mn" > "$W_MNEMONIC_FILE"; chmod 600 "$W_MNEMONIC_FILE"
        ok "W seed saved (chmod 600): ${W_MNEMONIC_FILE}"
    fi
    local w; w=$(key_addr "$W_KEY"); echo "$w" > "$W_ADDR_FILE"
    info "funding wallet W: ${C_B}${w}${C_RST}"

    # 1f. move 81M (minus buffer) founder -> W
    local buf_u send_u wbal
    buf_u=$(pax_to_uhpx "$BUFFER_PAX")
    send_u=$(( sbal - buf_u ))
    [ "$send_u" -gt 0 ] 2>/dev/null || die "nothing to move after buffer"
    wbal=$(bank_uhpx "$w"); wbal=${wbal:-0}
    if [ "$wbal" -ge "$send_u" ] 2>/dev/null; then
        ok "W already funded (${wbal} ${DENOM})"
    elif broadcasting; then
        info "sending ${send_u} ${DENOM} founder -> W"
        "$PAXD" tx bank send "$SRC_KEY" "$w" "${send_u}${DENOM}" \
            --from "$SRC_KEY" --node "$RPC" --chain-id "$CHAIN_ID" \
            --gas auto --gas-adjustment 1.5 --gas-prices "$GAS_PRICES" "${KB[@]}" -y 2>&1 | tail -3 \
            || die "founder->W transfer failed"
        sleep 6; wbal=$(bank_uhpx "$w")
        ok "W balance now: ${wbal} ${DENOM} ($(( ${wbal:-0} / UHPX_PER_PAX )) PAX)"
    else
        warn "[dry-run] would send ${send_u} ${DENOM} founder -> W ($(( send_u / UHPX_PER_PAX )) PAX)"
    fi
    hr; ok "PHASE 1 done. W address: ${w}"
}

# ─────────────────────────────────────────────────────────────────────────────
# PHASE 2: keygen — generate operator accounts on every server, collect addrs
# ─────────────────────────────────────────────────────────────────────────────
discover_servers(){
    # priority: $SERVERS env (space list) > $SERVERS_FILE > registry API
    if [ -n "${SERVERS:-}" ]; then printf '%s\n' $SERVERS; return; fi
    if [ -n "${SERVERS_FILE:-}" ] && [ -f "$SERVERS_FILE" ]; then
        grep -oE '([0-9]{1,3}\.){3}[0-9]{1,3}' "$SERVERS_FILE"; return; fi
    curl -fsS --max-time 12 "${MIRROR}/api/nodes" 2>/dev/null | jq -r '.nodes[].ip' 2>/dev/null
}

phase_keygen(){
    preflight
    hr; info "${C_B}PHASE 2 — operator keygen across fleet${C_RST}"; hr
    local -a servers
    local discovered ip line
    discovered=$(discover_servers) || die "server discovery failed"
    mapfile -t servers < <(printf '%s\n' "$discovered" | sed '/^$/d' | sort -u)
    [ "${#servers[@]}" -gt 0 ] || die "no discovered servers"
    discovered=$(python3 "$STATE" "$WORKDIR" begin "${servers[@]}") || die "cannot persist intended roster"
    mapfile -t servers <<<"$discovered"
    for ip in "${servers[@]}"; do
        line=$($SSH "${SSH_USER}@${ip}" "bash -o pipefail -c 'hpx validator keygen 2>/dev/null | grep \"^OPERATOR \"'" 2>/dev/null)
        if [ "$?" -ne 0 ] || ! python3 "$STATE" "$WORKDIR" record "$ip" "$line"; then
            warn "$ip unresolved; established identity retained"
        fi
    done
    python3 "$STATE" "$WORKDIR" complete || die "incomplete roster; funding and staking blocked"
    ok "full intended roster verified -> $OPERATORS_FILE"
}

# ─────────────────────────────────────────────────────────────────────────────
# PHASE 3: fund — distribute W balance across the collected operators
# ─────────────────────────────────────────────────────────────────────────────
phase_fund(){
    preflight
    hr; info "${C_B}PHASE 3 — fund operator wallets from W${C_RST}"; hr
    python3 "$STATE" "$WORKDIR" complete || die "incomplete roster; run keygen"
    key_exists "$W_KEY" || die "no funding wallet W — run 'fund-wallet' first"
    local w; w=$(key_addr "$W_KEY") || die "cannot read funding identity"
    python3 "$STATE" "$WORKDIR" fund "$PAXD" "$RPC" "$CHAIN_ID" "$W_KEY" "$KDIR" \
        "$GAS_PRICES" "$KEYRING_BACKEND" "$w" "$(pax_to_uhpx "$BUFFER_PAX")" "$YES" \
        || die "funding incomplete; retained plan and signed payment journal for recovery"
    ok "funding phase finished"
    broadcasting || warn "dry-run only — re-run with --yes to broadcast"
}

# ─────────────────────────────────────────────────────────────────────────────
# PHASE 4: stake-cmds — print the operator commands to run per server
# ─────────────────────────────────────────────────────────────────────────────
phase_stake_cmds(){
    preflight
    python3 "$STATE" "$WORKDIR" complete || die "incomplete intended roster; run keygen"
    hr; info "${C_B}PHASE 4 — run these per server (restarts node as validator + stakes)${C_RST}"; hr
    echo "# Each command: switches the node to validator mode, restarts it, then"
    echo "# broadcasts create-validator self-delegating its funded balance (minus 1 PAX fee reserve)."
    echo "# Pre-req: the node must be fully synced (hpx status -> catching_up=false)."
    echo
    local mon ip acc val
    while IFS=$'\t' read -r mon ip acc val; do
        [ -n "$ip" ] || continue
        echo "# ${mon}  (${acc})"
        echo "${SSH% *} ${SSH_USER}@${ip} 'HPX_YES=1 hpx validator stake'"
    done < "$OPERATORS_FILE"
    hr
    echo "# Verify afterwards (per server):  ssh ${SSH_USER}@<ip> 'hpx validator status'"
    echo "# Or globally:  ${PAXD} q staking validators --node ${RPC} -o json | jq '.validators|length'"
}

# ─────────────────────────────────────────────────────────────────────────────
# PHASE 5: stake — drive the ratio-aware ramp across the fleet, round by round
# ─────────────────────────────────────────────────────────────────────────────
phase_stake(){
    preflight
    python3 "$STATE" "$WORKDIR" complete || die "incomplete intended roster; run keygen"
    local rounds="${HPX_STAKE_ROUNDS:-10}" settle="${HPX_STAKE_SETTLE:-8}"
    local doneline=$(( 2 * UHPX_PER_PAX ))   # a validator is "done" when <=2 PAX left to stake
    hr; info "${C_B}PHASE 5 — staged validator ramp (max_voting_power_ratio)${C_RST}"; hr
    broadcasting || warn "dry-run — shows what would run; add --yes to drive the ramp"
    local r mon ip acc val bal done_all
    for r in $(seq 1 "$rounds"); do
        info "── round ${r}/${rounds} ──"
        done_all=1
        while IFS=$'\t' read -r mon ip acc val; do
            [ -n "$ip" ] || continue
            bal=$(bank_uhpx "$acc"); bal=${bal:-0}
            if [ "$bal" -le "$doneline" ] 2>/dev/null; then
                printf "  %-18s %-16s %b\n" "$mon" "$ip" "${C_D}done (bal $(( bal/UHPX_PER_PAX )) PAX)${C_RST}"; continue
            fi
            done_all=0
            if broadcasting; then
                local out rc
                out=$($SSH -n "${SSH_USER}@${ip}" "HPX_YES=1 hpx validator stake 2>&1"); rc=$?
                if [ "$rc" -eq 0 ]; then
                    printf "  %-18s %-16s %b\n" "$mon" "$ip" "${C_G}stepped${C_RST}  ${C_D}$(echo "$out" | grep -E 'ramp step done|broadcasting|txhash' | tail -n1)${C_RST}"
                else
                    printf "  %-18s %-16s %b\n" "$mon" "$ip" "${C_R}step failed${C_RST}  ${C_D}$(echo "$out" | grep -E '\[-\]|Error|error|failed|catching' | tail -n1)${C_RST}"
                fi
                sleep 1
            else
                printf "  %-18s %-16s %b\n" "$mon" "$ip" "${C_Y}[dry-run] would step${C_RST}"
            fi
        done < "$OPERATORS_FILE"
        if [ "$done_all" = 1 ]; then ok "all validators reached target — ramp complete"; break; fi
        broadcasting || break
        info "settling ${settle}s for pool to update before next round..."; sleep "$settle"
    done
    hr; phase_status
}

# ─────────────────────────────────────────────────────────────────────────────
phase_status(){
    preflight
    hr; info "${C_B}Fleet staking status${C_RST}"; hr
    if key_exists "$W_KEY"; then
        local w wbal; w=$(key_addr "$W_KEY"); wbal=$(bank_uhpx "$w")
        info "W ${w}  balance ${wbal:-0} ${DENOM} ($(( ${wbal:-0} / UHPX_PER_PAX )) PAX)"
    else warn "no funding wallet yet"; fi
    if [ -f "$OPERATORS_FILE" ]; then
        local mon ip acc val bal
        while IFS=$'\t' read -r mon ip acc val; do
            [ -n "$acc" ] || continue
            bal=$(bank_uhpx "$acc")
            local on="no"; "$PAXD" q staking validator "$val" --node "$RPC" -o json >/dev/null 2>&1 && on="YES"
            printf "  %-18s bal=%-16s validator=%s\n" "$mon" "${bal:-0}" "$on"
        done < "$OPERATORS_FILE"
    else warn "no operators collected yet"; fi
    hr
}

usage(){
    cat <<EOF
${C_B}stake-fleet.sh${C_RST} — Paxeer fleet validator bootstrap

  fund-wallet    Phase 1: new seed W, associate founder, move 81M founder -> W
  keygen         Phase 2: run 'hpx validator keygen' on every server, collect addrs
  fund           Phase 3: distribute W balance across all operator wallets
  stake          Phase 5: drive the ratio-aware staking ramp across the fleet (rounds)
  stake-cmds     Phase 4: print the exact per-server stake commands to run
  status         Show W + operator balances and on-chain validator state

Money phases are DRY-RUN unless you pass ${C_B}--yes${C_RST} (or CONFIRM=YES).

Config (env): HPX_RPC=$RPC  HPX_EVM_RPC=$EVM_RPC  HPX_CHAIN_ID=$CHAIN_ID
  SERVERS='ip ip ...' | SERVERS_FILE=<file> | (default: registry ${MIRROR}/api/nodes)
  HPX_BUFFER_PAX=$BUFFER_PAX  HPX_SSH_USER=$SSH_USER  HPX_GAS_PRICES=$GAS_PRICES

Typical run:
  ./stake-fleet.sh fund-wallet --yes      # move the 81M into W
  ./stake-fleet.sh keygen                 # generate operator accounts fleet-wide
  ./stake-fleet.sh fund --yes             # fund every operator
  ./stake-fleet.sh stake-cmds             # copy/paste the per-server stake commands
EOF
}

main(){
    case "${1:-}" in
        fund-wallet)  phase_fund_wallet ;;
        keygen)       phase_keygen ;;
        fund)         phase_fund ;;
        stake)        phase_stake ;;
        stake-cmds)   phase_stake_cmds ;;
        status)       phase_status ;;
        ""|help|-h|--help) usage ;;
        *) err "unknown command: $1"; usage; exit 1 ;;
    esac
}
main "$@"

#!/usr/bin/env bash
set -euo pipefail

# The host map and rpc_unit are the probe's.
# shellcheck source=tools/bringup/check-live.sh
check_live_path="$(dirname "${BASH_SOURCE[0]}")/check-live.sh"
if [ "$(grep -Fxc 'mode="${1:-}"' "$check_live_path")" != 1 ]; then
	echo "search-front: check-live helper boundary is unavailable" >&2
	exit 2
fi
# shellcheck disable=SC1090
. <(sed '/^mode="\${1:-}"$/,$d' "$check_live_path")
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

usage() {
	cat <<'EOF'
usage: tools/bringup/search-front.sh names | render | config | qualify | deploy <service-host>

The search front of search.paxeer.network: the Railway service configured by
services/interop/deploy/search-front/railway.env.example and built from
docker/search-front/Dockerfile. It keeps each client address on one serving
RPC name; names reads the host map from BRINGUP_HOSTS_FILE without printing it.

names     asks every RPC_HOSTS destination over ssh which apiN site its nginx
          serves (rpc_unit of tools/bringup/check-live.sh) and prints one line
          per destination, ordered by N: "serve <apiN name>" for a serving
          RPC name, "validator <apiN name>" for a destination that
          VALIDATOR_HOSTS also names. Exits 1 naming RPC_HOSTS[k] when a
          destination does not answer or serves no apiN site.

render    prints the upstream list that nginx includes at http level: a
          comment line, the split_clients block on $xweb_client (the rightmost
          X-Forwarded-For hop outside LAYERX_TRUSTED_PROXIES) that pins
          $xweb_node to one serving name for every client, and the map that
          admits it. The pin is
          SEARCH_FRONT_PRIMARY when its bounded typed /readyz contract is
          valid, else SEARCH_FRONT_BACKUP when its contract is valid, else
          SEARCH_FRONT_PRIMARY unadmitted, so readiness and search paths
          answer 503. One pinned name keeps the receiver between a payment
          challenge and its retry; nginx checks current readiness on that
          backend before every paid request.

qualify   validates mounted render output from stdin against discovered serving
          names, primary/backup eligibility and fresh typed readiness.

config    prints the serving sidecar configuration of
          services/interop/crates/x-websearch/config.example.json with every "${NAME}"
          string replaced by the environment variable NAME and the note
          dropped. Exits 1 naming every unset or empty variable, never a
          value.

deploy <service-host>
          prints two lines: SEARCH_FRONT_UPSTREAMS=<base64 of render output>
          to set on the search-front service, then
          tools/bringup/edge.sh register search.paxeer.network <service-host>
          to register that service with the edge.

Environment:
  BRINGUP_HOSTS_FILE   the private host map, as tools/bringup/check-live.sh
  CHECK_LIVE_TIMEOUT   seconds per ssh call, default 30
  SEARCH_FRONT_PRIMARY the pinned serving name, default the first discovered serving RPC node
  SEARCH_FRONT_BACKUP  the serving name pinned while the primary is not
                       ready, default the next distinct discovered serving RPC node
  SEARCH_FRONT_READINESS_TIMEOUT seconds per HTTPS readiness probe, default 5

Exits 1 when a destination or configuration step fails; 2 on a usage
error or a host map lacking a role.
EOF
}

search_front_host="search.paxeer.network"
search_front_primary="${SEARCH_FRONT_PRIMARY:-}"
search_front_backup="${SEARCH_FRONT_BACKUP:-}"
search_front_config_template="services/interop/crates/x-websearch/config.example.json"

search_front_names() {
	load_rpc_inventory || return 1
	local -a dests validators lines=()
	local k v reply name role status
	read -r -a dests <<<"$RPC_HOSTS"
	read -r -a validators <<<"$VALIDATOR_HOSTS"
	for k in "${!dests[@]}"; do
		status=0
		reply="$(rpc_unit "${dests[$k]}")" || status=$?
		if [ "$status" -ne 0 ]; then
			echo "search-front: RPC_HOSTS[$k] ssh=$status" >&2
			return 1
		fi
		read -r name _ <<<"$reply"
		case " ${rpc_names[*]} " in
		*" $name.$rpc_domain "*) ;;
		*)
			echo "search-front: RPC_HOSTS[$k] site=none" >&2
			return 1
			;;
		esac
		role=serve
		for v in "${validators[@]}"; do
			[ "$v" != "${dests[$k]}" ] || role=validator
		done
		lines+=("$role $name.$rpc_domain")
	done
	printf '%s\n' "${lines[@]}" | sort -k2,2V
}

search_front_select() {
	local listing="$1" selection
	load_rpc_inventory || return 1
	selection="$(printf '%s\n' "$listing" | "${BRINGUP_INSPECT_BIN:-bringup-inspect}" search-selection "$rpc_inventory" "$search_front_primary" "$search_front_backup")" || return 1
	read -r search_front_primary search_front_backup <<<"$selection"
}

search_front_candidate() {
	"${BRINGUP_INSPECT_BIN:-bringup-inspect}" search-ready "$1" "${SEARCH_FRONT_READINESS_TIMEOUT:-5}"
}

search_front_render() {
	load_hosts
	local listing
	listing="$(search_front_names)" || return 1
	search_front_select "$listing" || return 1
	printf '%s\n' "$listing" | "${BRINGUP_INSPECT_BIN:-bringup-inspect}" search-render "$rpc_inventory" "$search_front_primary" "$search_front_backup" "${SEARCH_FRONT_READINESS_TIMEOUT:-5}"
}

# qualify reads the mounted renderer output on stdin, checks the single pin
# and exact eligibility map against discovery, then probes that serving node
# through the same bounded typed readiness consumer used by render.
search_front_qualify() {
	local mounted listing pin
	mounted="$(cat)"
	listing="$1"
	pin="$(python3 -c '
import re
import sys

mounted, listing, primary, backup = sys.argv[1:]
try:
    serving, validators = set(), set()
    for line in listing.splitlines():
        role, name = line.split()
        (serving if role == "serve" else validators).add(name)
    if primary == backup or any(name not in serving or name in validators for name in (primary, backup)):
        raise ValueError("primary/backup must be distinct serving nodes")
    text = re.sub(r"(?m)^\s*#.*$", "", mounted).strip()
    contract = re.fullmatch(r"""split_clients\s+"\$\{xweb_client\}"\s+\$xweb_node\s*\{\s*\*\s+([a-z0-9.-]+);\s*\}\s*map\s+\$xweb_node\s+\$xweb_eligible\s*\{\s*default\s+0;\s*([a-z0-9.-]+)\s+([01]);\s*\}""", text)
    if not contract:
        raise ValueError("single pin and eligibility map required")
    pin, eligible, admitted = contract.groups()
    if pin not in (primary, backup) or eligible != pin or admitted != "1":
        raise ValueError("pin is unknown, ineligible or unready")
    print(pin)
except ValueError as error:
    sys.exit("search-front: " + str(error))
' "$mounted" "$listing" "$search_front_primary" "$search_front_backup")" || return 1
	search_front_candidate "$pin" || { echo "search-front: pinned backend readiness refused" >&2; return 1; }
	printf '%s\n' "$pin"
}

search_front_config() {
	python3 -I - "$repo_root/$search_front_config_template" <<'PY'
import json
import os
import re
import sys

with open(sys.argv[1], encoding="utf-8") as handle:
    template = json.load(handle)
template.pop("note", None)
missing = []

def fill(value):
    if isinstance(value, dict):
        return {key: fill(item) for key, item in value.items()}
    if isinstance(value, list):
        return [fill(item) for item in value]
    if isinstance(value, str) and "${" in value:
        match = re.fullmatch(r"\$\{([A-Z][A-Z0-9_]*)\}", value)
        if not match:
            sys.exit("search-front: malformed reference in " + sys.argv[1])
        item = os.environ.get(match[1], "")
        if not item and match[1] not in missing:
            missing.append(match[1])
        return item
    return value

config = fill(template)
if missing:
    sys.exit("search-front: unset " + " ".join(missing))
print(json.dumps(config, indent=2))
PY
}


# Helpers may be sourced by qualification without executing the CLI.
[ "${BASH_SOURCE[0]}" = "$0" ] || return 0

mode="${1:-}"
case "$mode" in
-h | --help)
	usage
	exit 0
	;;
names | render | config | qualify | deploy) ;;
*)
	usage >&2
	exit 2
	;;
esac
expected_args=1
[ "$mode" != deploy ] || expected_args=2
if [ "$#" -ne "$expected_args" ]; then
	usage >&2
	exit 2
fi

tools=(timeout)
case "$mode" in names | render | qualify | deploy) tools+=("${BRINGUP_INSPECT_BIN:-bringup-inspect}") ;; esac
[ "$mode" != names ] || tools+=(ssh sort)
[ "$mode" = names ] || tools+=(python3)
[ "$mode" != deploy ] || tools+=(base64)
for tool in "${tools[@]}"; do
	if ! command -v "$tool" >/dev/null 2>&1; then
		echo "search-front: $tool is required" >&2
		exit 2
	fi
done

case "$mode" in
names)
	load_hosts
	search_front_names
	;;
render) search_front_render ;;
qualify)
	load_hosts
	listing="$(search_front_names)" || exit 1
	search_front_select "$listing" || exit 1
	search_front_qualify "$listing"
	;;
config) search_front_config ;;
deploy)
	upstreams="$(search_front_render | base64 -w0)"
	printf 'SEARCH_FRONT_UPSTREAMS=%s\n' "$upstreams"
	printf 'tools/bringup/edge.sh register %s %s\n' "$search_front_host" "$2"
	;;
esac

#!/usr/bin/env bash
set -euo pipefail

usage() {
	cat <<'EOF'
usage: tools/bringup/check-live.sh hosts|rpc-nodes|archive-node|ca|hpx|explorer

Checks one system of the Paxeer X Network bring-up against its live answers.
Every subcommand reads the operator's private host map from the file named
by BRINGUP_HOSTS_FILE and never prints a value from it; explorer needs no
host map. A check of a Fly app reads the app's name from the app line of
its toml and runs its command inside a machine of the app through flyctl
ssh console, bounded by CHECK_LIVE_TIMEOUT.

hosts     runs ssh true against every destination of every role of the host
          map and prints one line per role:
  <ROLE>  "pass <ROLE> reachable=<n>/<m>" when all m destinations answered,
          "fail <ROLE> reachable=<n>/<m> ssh=<exit>" otherwise, with the exit
          code of the first destination that did not answer (255 when ssh
          could not connect, 124 when CHECK_LIVE_TIMEOUT elapsed)
          Exits 0 only when every role passes.

rpc-nodes asks each of the declared public RPC names numbered nodes of the
          network domain for eth_blockNumber over https, and asks every
          RPC_HOSTS destination over ssh which public name its nginx site
          serves, the ActiveState of its paxd.service unit and how many
          seconds that unit has been active. Prints one line per name:
  apiN    "pass apiN head=<height> lag=<blocks> unit=active active=<s>s"
          when the name answered, its lag behind the highest answer is at
          most ten blocks, a destination serves it, and its unit is active
          and has not restarted in the last hour (3600 seconds);
          "fail apiN head=<height|none> lag=<blocks|none>
          unit=<state|unmapped> active=<s>s|none" otherwise.
          A destination that did not answer prints "fail RPC_HOSTS[k]
          ssh=<exit>" (k is its index in RPC_HOSTS) and one whose nginx
          serves no apiN site prints "fail RPC_HOSTS[k] site=none".
          Exits 0 only when every name passes.
archive-node  reads the archive host's retention keys, paxd unit state and
          public RPC name over ssh, then asks that name and the declared
          public RPC names over HTTPS, printing three lines:
  retention  "pass ARCHIVE_HOST retention min-retain-blocks=0 ss-keep-recent=0 paxd=active"
             or the fail line with the values read (ssh=<exit> when the host
             did not answer)
  head       "pass ARCHIVE_HOST eth_blockNumber archive=<n> head=<m> gap=<d>"
             where head is the highest answer of the declared public names
             and the gap is at most ten blocks
  history    "pass ARCHIVE_HOST eth_getBlockByNumber height=<h> hash=<hash>"
             for h = archive head minus 200000, twice the 100000 blocks the
             pruned public nodes retain
          Exits 0 only when all three pass.

ca        reads the internal CA under LAYERX_CA_DIR on this host and the
          certificate of every service that tools/bringup/ca.sh services
          lists, from the row's target: for box:<VARIABLE>, over ssh to the
          box the hosts file assigns to the variable,
          LAYERX_TLS_DIR/<service>/cert.pem; for railway:<service>[,...],
          the base64 <PREFIX>_CERT variable of every Railway service of the
          row in environment LAYERX_RAILWAY_ENVIRONMENT, which must agree.
          A row of ca.sh attestor_services is verified against the
          attestors' gateway CA under LAYERX_ATTESTOR_CA_DIR instead of the
          internal CA. One line each:
  ca      "pass ca ca expires_in=<days>d" when the CA certificate is readable
          and more than thirty days from expiry
  <service>
          "pass <service> target=<target> chain=ok san=<m>/<m>
          expires_in=<days>d" when the certificate chains to the CA, carries
          every SAN the service list declares, and is more than thirty days
          from expiry; "fail <service> target=<target> host=absent" when the
          hosts file assigns no box to the variable; "fail <service>
          target=<target> cert=absent" when the box or a Railway service
          holds none; "fail <service> target=<target> cert=differs" when the
          Railway services of the row hold different certificates;
          otherwise "fail" with chain=untrusted, san=<n>/<m> missing=<names>
          or the expiry as observed; "fail <service> target=<target>
          attestor_ca=absent LAYERX_ATTESTOR_CA_DIR=<dir or unset>" for an
          attestor_services row when that directory holds no ca.pem.
          Exits 0 only when the CA and every certificate pass.

hpx       reads the hpx registry at CHECK_LIVE_HPX_ORIGIN, by default
          https://node.hyperpaxeer.com, and at CHECK_LIVE_HPX_APP_ORIGIN, by
          default https://<app>.fly.dev for the app of tools/hpx/hosting/fly.toml,
          and prints one line per check; the app's lines carry the app- prefix:
  healthz    GET <origin>/healthz answers 200 with ok true, chain_id
             hyperpax_125-1 and a forty-hex source_revision
  checksums  GET <origin>/checksums.txt lists "<sha256>  <path>" lines and
             every listed path, fetched from the origin, hashes to its line;
             "pass checksums verified=<n>/<n>" or "fail checksums
             verified=<k>/<n> first=<path>" naming the first mismatch
  api-nodes  GET <origin>/api/nodes answers 200 with chain_id hyperpax_125-1
             and a nodes list whose length is count
  landing    GET <origin>/ answers 200 with the landing page of
             tools/hpx/hosting/index.html
  served-by  the public origin's /healthz carries the Fly edge's
             fly-request-id header and equals the app's /healthz, so the name
             is served by the app and not by the registry on the edge host;
             "fail served-by app=<app> fly-request-id=<present|absent>
             healthz=<match|differ>" otherwise
          Exits 0 only when all eight checks pass.

explorer  reads the deployed explorer at CHECK_LIVE_EXPLORER_ORIGIN, by default
          https://paxscan.io, its envs.js, its backend and its databases, and
          asks the declared public RPC names for their head and the lowest
          block they serve, one line per check:
  frontend   the origin answers 200 and the NEXT_PUBLIC_NETWORK_RPC_URL of its
             /assets/envs.js is https://<one of the declared public names>
  archive    that name serves a lower first block than every other name
             within ten blocks of the head (the pruned floor)
  backend    GET /api/health and /api/v2/stats at the NEXT_PUBLIC_API_HOST of
             envs.js answer 200, one line each, and GET
             /api/v2/blocks/<the archive name's first block> answers 200
  history    over EXPLORER_DATABASE_URL, EXPLORER_LEGACY_DATABASE_URL and
             PAXSCAN_DATABASE_PUBLIC_URL: the consensus blocks the explorer
             database holds from its first block up to the archive name's
             first block equal the legacy database's from that block up to
             its ceiling plus the paxscan database's above the ceiling
  missing-ranges
             every missing_block_ranges row of the explorer database at or
             above its first block lies inside the union of the two sources'
             missing_block_ranges
          Exits 0 only when every check passes.

bridge    runs services/bridge/deploy/checklist.sh for every chain under contracts/bridge/evm/chains
          and contracts/bridge/solana/chains against PAXEER_BRIDGE_RECORDS_DIR/<chain>.json,
          then reads a machine of the app of each of
          services/interop/deploy/bridge-relayer/relayer-1.toml through relayer-5.toml
          through flyctl ssh console, one line per check:
  checklist  "pass checklist chain=<chain> exit=0", or "fail checklist
             chain=<chain> record=absent" or "fail checklist chain=<chain>
             exit=<n> <last output line, URLs as <url>>"
  processes  "pass processes app=<app> relayer=running signer=running", one
             line per relayer app, when layerx-bridge-relayer and
             layerx-mirror-signer both run
  bridge-in  "pass bridge-in app=<app> item=in:<chain id>:<tx>:<log>
             outcome=included" when the journal on the volume of one relayer
             records a deposit as observed and as completed by that relayer's
             own included bridgeIn transaction; "fail bridge-in relayers=5
             journal=absent|journal=present included=none" otherwise
          Exits 0 only when every check passes; 2 when
          PAXEER_BRIDGE_RECORDS_DIR is unset. The checklist's own inputs
          (each chain's RPC variable, PAXEER_BRIDGE_PAXEER_RPC_URL,
          PAXEER_BRIDGE_GOVERNANCE_AUTHORITY) pass through the environment.

wallet    reads the public wallet endpoint name from the wallet_endpoint
          value of [decision.public_names] in
          spec/layerx-merge/spec.kvx, then runs the wallet gates of
          scripts/wallet/check-live.sh and prints their check lines:
  endpoint   "pass endpoint name=<host> source=spec", or "fail endpoint
             source=spec name=unset" when the value is unset or not a host
             name, in which case cutover is skipped
  cutover    the served_by and readiness lines of its cutover mode with
             CHECK_LIVE_CUTOVER_HOST set to that name
  gateway    the readiness and me lines of its gateway mode with
             CHECK_LIVE_GATEWAY_BASE https://<app>.fly.dev for the app of
             services/wallet/deploy/gateway.toml
  machines   "pass machines started=<n> regions=<list> app=<app>" when the
             app runs at least two started machines in at least two regions
          Exits 0 only when every check passes; a failing gate counts once.

kernel-value-loop pipes tools/bringup/value-loop.sh into a machine of the
          app of services/wallet/deploy/human.toml and prints one line per step:
  precondition "fail precondition <name> <detail>" when a genesis output
             (genesis-ids, custody-profile, publication-policy), the
             sequencer's LNI socket (kernel-node), the receipt authority, a
             kernel image binary (image) or the opening deposit (deposit-tx)
             is absent; no other line follows
  asset      "pass asset PAX id=<hex>" when the genesis asset id equals the
             custody precompile's nativeAssetId()
  account    "pass account sender|recipient did=<did> main=<id>"
  credit     "pass credit deposit=<hash> amount=<n>" when the sender was
             opened by the first-credit path from DEPOSIT_TX
  activity   "pass activity id=<hex>" for the SEND sealed by the sequencer
  balance    "pass balance sender|recipient <json>"
  batch      "pass batch id=<hex> sealed=<n>" when the receipt authority's
             /v1/authorized-batches/by-activity answers for the SEND
  checkpoint "pass checkpoint batch=<n> status=submitted|final" when the
             anchor precompile's statusOf answers 1 or 2 for a sealed batch at
             or after it; a failing step prints its fail line and stops
          CHECK_LIVE_VALUE_LOOP_DEPOSIT_TX names the owner's deposit
          transaction for the sender's opening credit and
          CHECK_LIVE_VALUE_LOOP_CHECKPOINT_SECONDS bounds the checkpoint wait,
          default 600; the whole run is bounded by
          CHECK_LIVE_VALUE_LOOP_TIMEOUT, default 840.

registry-plan  prints the registry/hosted-gateway bring-up order from this checkout's
          tomls, no network, no host map, exactly four lines in order:
  stage   "stage <n> <name> requires=<previous stage|-> needs=<prerequisite>,...
          producers=<prerequisite>:<producer>,..." for material,
          registry-bootstrap, router-activation and routed-proof, a producer
          being ca.sh:<service>, fly-secret:<NAME>, init.sh:--prepare-material,
          deploy:<step> or stage:<name>; "fail registry-plan toml=absent
          <path>" and exit 1 when a toml line it reads is absent.

registry-bootstrap  the machines, private-ingress and mTLS healthz checks of
          registry, which ask no hosted gateway; registry adds program-events and
          router-readyz read through the configured hosted gateway.

router    requires protected material and registry-bootstrap records under
          CHECK_LIVE_STAGE_DIR, bound to CHECK_LIVE_CANDIDATE_REVISION,
          CHECK_LIVE_CANDIDATE_IMAGE and CHECK_LIVE_MATERIAL_GENERATION.
          Unless their ordered outcomes pass with the same pins, prints "fail router-activation
          missing=registry-bootstrap producer=stage:registry-bootstrap" and
          exits 1 before any request; its readyz line also fails with
          missing=program_registry when the hosted gateway's program_registry backend
          is absent or not configured.

relay     submits CHECK_LIVE_RELAY_TRANSACTION to the explicit archive node
          CHECK_LIVE_RELAY_ORIGIN and reads back its successful EVM receipt,
          canonical block inclusion and authenticated native committed receipt.
          CHECK_LIVE_RELAY_CA optionally supplies the archive's PEM trust root.
          The transaction stays pre-signed under operator custody.

Environment:
  BRINGUP_HOSTS_FILE   private env file assigning EDGE_HOST, ARCHIVE_HOST,
                       VALIDATOR_HOSTS, RPC_HOSTS and OLD_WALLET_HOST;
                       each value is one ssh destination or,
                       for the plural roles, a space-separated list of them;
                       optionally RETAINED_ON_VALIDATOR, the public RPC names
                       retained on a validator host by owner ruling
  CHECK_LIVE_HPX_ORIGIN  origin of the hpx registry, default
                       https://node.hyperpaxeer.com
  CHECK_LIVE_HPX_APP_ORIGIN  origin of the hpx registry's Fly app, default
                       https://<app>.fly.dev
  CHECK_LIVE_EXPLORER_ORIGIN  origin of the explorer frontend, default
                       https://paxscan.io
  EXPLORER_DATABASE_URL, EXPLORER_LEGACY_DATABASE_URL,
  PAXSCAN_DATABASE_PUBLIC_URL  read-only connection strings of the explorer's
                       production database, its legacy database and the
                       paxscan copy source; never printed
  CHECK_LIVE_GATEWAY_TOKEN  access token of a provisioned test identity of
                       the wallet gateway, required by wallet; never printed
  CHECK_LIVE_STAGE_DIR the operator's stage records, <stage>.json each,
                       required by router
  CHECK_LIVE_PAXD_HOME  actual mounted paxd --home path, required by kernel checks
  CHECK_LIVE_KERNEL_RPC, CHECK_LIVE_KERNEL_COMET_RPC
                       explicit node and consensus RPC endpoints for kernel-node
  CHECK_LIVE_KERNEL_LOCAL  1 inspects local paxd; otherwise uses KERNEL_HOST
  CHECK_LIVE_RELAY_ORIGIN  explicit archive node JSON-RPC origin, required by relay
  CHECK_LIVE_RELAY_CA  optional PEM CA file for the archive origin
  CHECK_LIVE_RELAY_TRANSACTION  file of signed chain bytes (or 0x hex)
  CHECK_LIVE_INTEROP_RPC, CHECK_LIVE_INTEROP_NODE_RPC
                       explicit hosted account and node receipt endpoints
  CHECK_LIVE_INTEROP_TRANSACTION  pre-signed chain-125 payExact transaction file
  CHECK_LIVE_INTEROP_API_KEY_FILE  facilitator API key file, never printed
  CHECK_LIVE_INDEXER_RPC, CHECK_LIVE_GATEWAY_RPC  explicit hosted gateway URLs
  CHECK_LIVE_SEARCH_PAYER_ADDRESS, CHECK_LIVE_SEARCH_GRANT_ID
                       EVM payer and existing authorized metered ledger grant
  CHECK_LIVE_DEVELOPERS_SOURCE_URL  authorized Human /v1/intents/submit URL
  CHECK_LIVE_DEVELOPERS_SOURCE_REQUEST, CHECK_LIVE_DEVELOPERS_SOURCE_TOKEN_FILE
                       signed current intent request and source session token
  CHECK_LIVE_EDGE_NGINX_DIR  edge nginx root, default /etc/nginx
  CHECK_LIVE_EDGE_BOX_CA     edge box upstream trust, default /etc/layerx/ca.crt
  CHECK_LIVE_EDGE_READINESS_PATH  HTTP readiness path, default /readyz
  CHECK_LIVE_INTERNAL_CONNECT_<GROUP>  private host:port reachable from KERNEL_HOST;
                       TLS retains the deployed service name
  CHECK_LIVE_TIMEOUT   seconds per request, ssh or flyctl call, default 30
  LAYERX_CA_DIR        the internal CA directory on this host, default
                       /etc/layerx/ca
  LAYERX_ATTESTOR_CA_DIR
                       the attestors' gateway CA directory on this host,
                       required by ca for the rows of ca.sh attestor_services
  LAYERX_TLS_DIR       the certificate directory root on a box, default
                       /data/tls
  LAYERX_RAILWAY_ENVIRONMENT
                       the Railway environment ca reads, default beta
  LAYERX_RAILWAY_BIN   the railway CLI, default ~/.railway/bin/railway
  CHECK_LIVE_GAS_ACCOUNT_KEYSTORE, CHECK_LIVE_GAS_ACCOUNT_PASSWORD_FILE
                       the keystore and password file of the gas check's
                       account, already delegated to the paymaster and
                       holding SID; the sponsored batch spends SID from it;
                       never printed
  CHECK_LIVE_GAS_MAX_TOKEN_AMOUNT  the most SID base units the gas check's
                       quote may charge
  CHECK_LIVE_GAS_ORIGIN  origin of the gas station, default
                       https://chain.paxeer.network
  CHECK_LIVE_ROUTER_URL  operator-deployed hosted gateway origin, required by
                       router, search and registry (except registry-bootstrap);
                       registry uses its explicit origin; hosted JSON-RPC uses CHECK_LIVE_GATEWAY_RPC
  CHECK_LIVE_INDEXER_RPC  hosted gateway JSON-RPC endpoint for px_* history;
                       required by indexer, with no public node default
  CHECK_LIVE_GAS_RPC   node JSON-RPC URL at the root, chain id 125,
                       default https://api-mainnet-beta.paxeer.network;
                       api1 through api14.mainnet-beta.paxeer.network also
                       serve JSON-RPC at the root, with no router or /rpc
  CHECK_LIVE_GAS_RECEIPT_ATTEMPTS  receipt polls five seconds apart, default 36

Exits 1 when any check fails, 2 on a usage error, an unset BRINGUP_HOSTS_FILE
or a host map lacking a role.
EOF
}

timeout="${CHECK_LIVE_TIMEOUT:-30}"
ca_dir="${LAYERX_CA_DIR:-/etc/layerx/ca}"
tls_dir="${LAYERX_TLS_DIR:-/data/tls}"
railway_env="${LAYERX_RAILWAY_ENVIRONMENT:-beta}"
railway_bin="${LAYERX_RAILWAY_BIN:-$HOME/.railway/bin/railway}"
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

roles=(EDGE_HOST ARCHIVE_HOST VALIDATOR_HOSTS RPC_HOSTS OLD_WALLET_HOST)

# Shared production validator, installed from tools/bringup/cmd/bringup-inspect.
bringup_inspect() {
	"${BRINGUP_INSPECT_BIN:-bringup-inspect}" "$@"
}

rpc_inventory="$repo_root/docs/site/docs/reference/public-rpc.md"
rpc_names=()
node_endpoints=()
rpc_public_names=()
load_rpc_inventory() {
	local names endpoints
	names="$(bringup_inspect rpc-names "$rpc_inventory")" || return 1
	endpoints="$(bringup_inspect node-endpoints "$rpc_inventory")" || return 1
	mapfile -t rpc_names <<<"$names"
	mapfile -t node_endpoints <<<"$endpoints"
	mapfile -t rpc_public_names < <(printf '%s\n' "${node_endpoints[@]#https://}")
	rpc_name_count="${#rpc_names[@]}"
	rpc_domain="${rpc_names[0]#*.}"
}


# load_hosts: sources BRINGUP_HOSTS_FILE and exits 2 naming the first role it
# lacks. Nothing read from the file is ever printed.
load_hosts() {
	local role
	if [ -z "${BRINGUP_HOSTS_FILE:-}" ]; then
		echo "check-live: BRINGUP_HOSTS_FILE is unset" >&2
		exit 2
	fi
	if [ ! -r "$BRINGUP_HOSTS_FILE" ]; then
		echo "check-live: BRINGUP_HOSTS_FILE does not name a readable file" >&2
		exit 2
	fi
	# shellcheck disable=SC1090
	. "$BRINGUP_HOSTS_FILE"
	for role in "${roles[@]}"; do
		if [ -z "${!role:-}" ]; then
			echo "check-live: BRINGUP_HOSTS_FILE lacks $role" >&2
			exit 2
		fi
	done
}

# finish <failures>: prints the summary line and exits 1 on any failure.
finish() {
	if [ "$1" -ne 0 ]; then
		echo "check-live: $1 check(s) failed"
		exit 1
	fi
	echo "check-live: all checks passed"
	exit 0
}

# ssh_true <destination>: runs true on the destination without prompting,
# bounded by CHECK_LIVE_TIMEOUT, printing nothing; returns the ssh exit code
# or 124 when the bound elapsed.
ssh_true() {
	timeout "$timeout" ssh -n -o BatchMode=yes -- "$1" true >/dev/null 2>&1
}

# ssh_read <destination> <command>: runs a read-only command on the
# destination without prompting or a terminal, bounded by CHECK_LIVE_TIMEOUT;
# its stdout is the result and its stderr is dropped.
ssh_read() {
	timeout "$timeout" ssh -n -o BatchMode=yes -- "$1" "$2" 2>/dev/null
}

# fly_app <toml>: prints the Fly app name from the app line of the toml, a
# path relative to the repository root; status 1 when the file or its app
# line is missing.
fly_app() {
	local app
	app="$(sed -n 's/^app[[:space:]]*=[[:space:]]*"\([a-z0-9-]*\)"[[:space:]]*$/\1/p' "$repo_root/$1" 2>/dev/null | head -n 1)"
	[ -n "$app" ] && printf '%s' "$app"
}

# railway_service <rollout row>: resolve the deployed service, failing when
# the row has no Railway service instead of consulting a retired Fly toml.
railway_service() {
	python3 -c '
import sys, tomllib
with open(sys.argv[1], "rb") as source:
    row = tomllib.load(source).get("service", {}).get(sys.argv[2], {})
service = row.get("railway_service")
if not service:
    sys.exit(1)
print(service, end="")
' "$repo_root/deploy/rollout.kvx" "$1" 2>/dev/null
}

# Railway ssh accepts one command line and does not forward stdin. Carry the
# input in that command, just as the relay probe does.
railway_ssh() {
	local input command
	local -a instance=()
	[ -z "${3:-}" ] || instance=(--deployment-instance "$3")
	input="$(base64 -w0)"
	command="echo $input | base64 -d | $2"
	command="${command//\'/\'\\\'\'}"
	timeout "$timeout" "$railway_bin" ssh --service "$1" --environment "$railway_env" "${instance[@]}" -- "sh -c '$command'" 2>/dev/null
}

railway_instance() {
	timeout "$timeout" "$railway_bin" status --service "$1" --environment "$railway_env" --json </dev/null 2>/dev/null |
		python3 -c '
import json, sys
def nodes(connection):
    return [edge["node"] for edge in connection["edges"]]
doc = json.load(sys.stdin)
service, environment = sys.argv[1:]
if "environments" in doc:
    env = next(e for e in nodes(doc["environments"]) if e["name"] == environment or e["id"] == environment)
    if "serviceInstances" in env:
        instance = next(i for i in nodes(env["serviceInstances"]) if i["serviceName"] == service or i["serviceId"] == service)
    else:
        svc = next(s for s in nodes(doc["services"]) if s["name"] == service or s["id"] == service)
        instance = next(i for i in nodes(svc["serviceInstances"]) if i["environmentId"] == env["id"])
else:
    instance = doc
print(json.dumps(instance))
' "$1" "$railway_env" 2>/dev/null
}

railway_domains() {
	railway_instance "$1" | python3 -c '
import json, sys
domains = json.load(sys.stdin)["domains"]
print(len(domains["serviceDomains"]) + len(domains["customDomains"]))
' 2>/dev/null
}

railway_origin() {
	railway_instance "$1" | python3 -c '
import json, sys
domains = json.load(sys.stdin)["domains"]
print("https://" + (domains["customDomains"] + domains["serviceDomains"])[0]["domain"])
' 2>/dev/null
}

railway_deployment() {
	local answer
	answer="$(railway_instance "$1" | python3 -c '
import json, sys
i = json.load(sys.stdin)
d = i.get("latestDeployment") or {}
config = (d.get("meta") or {}).get("serviceManifest", {}).get("deploy", {})
regions = config.get("multiRegionConfig") or {}
replicas = sum(r["numReplicas"] for r in regions.values()) if regions else config.get("numReplicas")
volume = sys.argv[1]
volumes = (d.get("meta") or {}).get("volumeMounts") or {}
if isinstance(volumes, dict):
    volumes = [e["node"] for e in volumes.get("edges", [])]
mounted = any(v.get("mountPath") == volume for v in volumes)
ok = d.get("status") == "SUCCESS" and isinstance(replicas, int)
if volume != "-":
    ok = ok and replicas == 1 and len(volumes) == 1 and mounted
else:
    ok = ok and replicas >= 2 and len(regions) >= 2
print(("pass" if ok else "fail") + " deployment status=%s replicas=%s regions=%s volume=%s" % (d.get("status", "none"), replicas, ",".join(sorted(regions)) or "none", volume if mounted else "none"))
' "${2:--}" 2>/dev/null)" || answer="fail deployment status=unreadable"
	echo "${answer%% *} deployment service=$1 ${answer#* deployment }"
	[ "${answer%% *}" = pass ]
}

# fly_ssh <app> <process group or -> <command>: runs the command under sh
# inside a machine of the app, of the process group unless it is -, through
# flyctl ssh console, bounded by CHECK_LIVE_TIMEOUT. stdin passes through and
# stdout is the result; stderr is dropped because flyctl names the machine's
# private address on it. The command carries no single quote.
fly_ssh() {
	local group=()
	[ "$2" = - ] || group=(--process-group "$2")
	timeout "$timeout" flyctl ssh console --quiet --app "$1" ${group[@]+"${group[@]}"} --command "sh -c '$3'" 2>/dev/null
}

# railway_cert <service> <prefix>: prints the PEM of the base64 <PREFIX>_CERT
# variable of the Railway service, or nothing when it is unset; status 1 when
# the variables cannot be read. No other value is printed.
railway_cert() {
	timeout "$timeout" "$railway_bin" variable list --service "$1" --environment "$railway_env" --json </dev/null 2>/dev/null |
		python3 -c '
import base64, json, sys
doc = json.load(sys.stdin)
if isinstance(doc, list):
    doc = {v.get("name"): v.get("value") for v in doc}
cert = doc.get(sys.argv[1] + "_CERT") or ""
if cert:
    sys.stdout.write(base64.b64decode(cert, validate=True).decode("ascii"))
' "$2" 2>/dev/null
}

# days_left: whole days from now until the notAfter of the PEM certificate on
# stdin.
days_left() {
	local end
	end="$(openssl x509 -noout -enddate | cut -d= -f2)"
	echo $((($(date -d "$end" +%s) - $(date +%s)) / 86400))
}

# fly_regions <app>: prints "pass machines started=<n> regions=<list>
# app=<app>" when the app runs at least two started machines in at least two
# regions, the fail line with the observed values (or machines=unreadable)
# and status 1 otherwise.
fly_regions() {
	local machines
	machines="$(timeout "$timeout" flyctl machines list --app "$1" --json 2>/dev/null | python3 -c '
import json
import sys

try:
    doc = json.load(sys.stdin)
except ValueError:
    print("fail machines=unreadable")
    sys.exit(0)
started = [m for m in doc if m.get("state") == "started"]
regions = sorted({m.get("region", "") for m in started})
ok = len(started) >= 2 and len(regions) >= 2
print(("pass" if ok else "fail") + " machines started=%d regions=%s" % (len(started), ",".join(regions) or "none"))
')" || machines="fail machines=unreadable"
	echo "${machines%% *} ${machines#* } app=$1"
	[ "${machines%% *}" = pass ]
}

rpc_domain="mainnet-beta.paxeer.network"
rpc_name_count=0
rpc_max_lag=10
rpc_min_active=3600
# shellcheck disable=SC2016
rpc_unit_cmd='n=$(ls /etc/nginx/sites-enabled 2>/dev/null | sed -n "s/^\(api[0-9]*\)\..*\.conf$/\1/p" | head -n 1); s=$(systemctl show paxd.service -p ActiveState --value 2>/dev/null); e=$(systemctl show paxd.service -p ActiveEnterTimestampMonotonic --value 2>/dev/null); u=$(awk "{print int(\$1)}" /proc/uptime); echo "${n:-none} ${s:-none} $((u - ${e:-0} / 1000000))"'

# rpc_head <name>: prints the decimal eth_blockNumber the public name answers
# over https within CHECK_LIVE_TIMEOUT, or nothing (status 1) when it does not.
rpc_head() {
	local hex
	hex="$(curl -s -m "$timeout" -H 'content-type: application/json' \
		-d '{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}' \
		"https://$1.$rpc_domain" 2>/dev/null | sed -n 's/.*"result":"0x\([0-9a-fA-F]*\)".*/\1/p')"
	[ -n "$hex" ] && printf '%d\n' "0x$hex"
}

# rpc_unit <destination>: prints "<apiN|none> <ActiveState> <seconds active>"
# for the paxd.service unit behind the destination's nginx site; returns the
# ssh exit code or 124 when CHECK_LIVE_TIMEOUT elapsed.
rpc_unit() {
	timeout "$timeout" ssh -n -o BatchMode=yes -- "$1" "$rpc_unit_cmd" 2>/dev/null
}

check_rpc_nodes() {
	load_rpc_inventory || { echo "fail rpc inventory=unreadable"; finish 1; }
	local -a dests heads
	local -A units=()
	local k reply status name state secs n head lag ok polls top=0 failures=0
	read -r -a dests <<<"$RPC_HOSTS"
	for k in "${!dests[@]}"; do
		status=0
		reply="$(rpc_unit "${dests[$k]}")" || status=$?
		if [ "$status" -ne 0 ]; then
			echo "fail RPC_HOSTS[$k] ssh=$status"
			failures=$((failures + 1))
			continue
		fi
		read -r name state secs <<<"$reply"
		case " ${rpc_names[*]} " in
		*" $name.$rpc_domain "*) units[$name]="$state $secs" ;;
		*)
			echo "fail RPC_HOSTS[$k] site=none"
			failures=$((failures + 1))
			;;
		esac
	done
	# The declared names are asked at once so that the ten-block window is not
	# eaten by the chain advancing between one request and the next.
	polls="$(mktemp -d)"
	trap 'rm -rf "$polls"' EXIT
	for n in $(seq 1 "$rpc_name_count"); do
		rpc_head "api$n" >"$polls/$n" 2>/dev/null &
	done
	wait
	for n in $(seq 1 "$rpc_name_count"); do
		heads[n]="$(cat "$polls/$n")"
		if [ -n "${heads[n]}" ] && [ "${heads[n]}" -gt "$top" ]; then
			top="${heads[n]}"
		fi
	done
	for n in $(seq 1 "$rpc_name_count"); do
		name="api$n"
		head="${heads[n]}"
		lag=none
		state=unmapped
		secs=none
		ok=1
		if [ -n "$head" ]; then
			lag=$((top - head))
			[ "$lag" -le "$rpc_max_lag" ] || ok=0
		else
			head=none
			ok=0
		fi
		if [ -n "${units[$name]:-}" ]; then
			read -r state secs <<<"${units[$name]}"
			secs="${secs}s"
			[ "$state" = active ] && [ "${secs%s}" -ge "$rpc_min_active" ] || ok=0
		else
			ok=0
		fi
		if [ "$ok" -eq 1 ]; then
			echo "pass $name head=$head lag=$lag unit=$state active=$secs"
		else
			echo "fail $name head=$head lag=$lag unit=$state active=$secs"
			failures=$((failures + 1))
		fi
	done
	local canonical_chain canonical_head
	canonical_chain="$(bringup_inspect rpc "${node_endpoints[0]}" eth_chainId '[]')" || canonical_chain=""
	canonical_head="$(bringup_inspect rpc "${node_endpoints[0]}" eth_blockNumber '[]')" || canonical_head=""
	canonical_head="$(hex_to_dec "$canonical_head")"
	if [ "$canonical_chain" = 0x7d ] && [ -n "$canonical_head" ] && [ "$top" -gt 0 ] &&
		[ $((top - canonical_head)) -le "$rpc_max_lag" ] && [ $((canonical_head - top)) -le "$rpc_max_lag" ]; then
		echo "pass canonical chain_id=125 head=$canonical_head"
	else
		echo "fail canonical chain-or-head=invalid"
		failures=$((failures + 1))
	fi
	finish "$failures"
}

# The read-only inspection rpc-placement runs on a validator host: the
# ActiveState of the full-node unit paxd.service (never a validator unit), the
# HTTP and HTTPS ports it listens on beyond loopback and the apiN names of its
# enabled nginx sites (other for a site that is no apiN name), one line, no
# address.
# shellcheck disable=SC2016
placement_cmd='s=$(systemctl is-active paxd.service 2>/dev/null); p=$(ss -Hltn 2>/dev/null | awk "{print \$4}" | grep -Ev "^(127\.|\[::1\]:|::1:)" | sed -n "s/.*:\(80\|443\)$/\1/p" | sort -u | paste -sd, -); n=$(ls /etc/nginx/sites-enabled 2>/dev/null | sed -e "s/^\(api[0-9]*\)\..*/\1/" -e "/^api[0-9]*$/!s/.*/other/" | sort -u | paste -sd, -); echo "${s:-inactive} ${p:-none} ${n:-none}"'

# rpc_addrs <apiN>: the addresses the public name resolves to, one per line.
rpc_addrs() {
	getent ahosts "$1.$rpc_domain" 2>/dev/null | awk '{print $1}' | sort -u
}

# check_rpc_placement: each public RPC name resolves to an RPC_HOSTS
# destination outside VALIDATOR_HOSTS and answers within ten blocks of the
# highest answer, and no validator host runs the full-node unit or listens on
# a public HTTP or HTTPS port. A name RETAINED_ON_VALIDATOR lists (full public
# names or apiN) may resolve to an RPC_HOSTS destination that is a validator
# host by owner ruling: it is reported as retained, and that validator host
# may run the full-node unit and listen on HTTP and HTTPS for the retained
# names' nginx sites only. Destinations are named by role and index only.
check_rpc_placement() {
	load_rpc_inventory || { echo "fail rpc inventory=unreadable"; finish 1; }
	local -a rpcs validators heads addrs
	local -A retained=() kept=()
	local k j n r name head lag at validator ok polls status reply state ports sites site top=0 failures=0
	read -r -a rpcs <<<"$RPC_HOSTS"
	read -r -a validators <<<"$VALIDATOR_HOSTS"
	for r in ${RETAINED_ON_VALIDATOR:-}; do
		retained[${r%%.*}]=1
	done
	polls="$(mktemp -d)"
	trap 'rm -rf "$polls"' EXIT
	for n in $(seq 1 "$rpc_name_count"); do
		rpc_head "api$n" >"$polls/$n" 2>/dev/null &
	done
	wait
	for n in $(seq 1 "$rpc_name_count"); do
		heads[n]="$(cat "$polls/$n")"
		if [ -n "${heads[n]}" ] && [ "${heads[n]}" -gt "$top" ]; then
			top="${heads[n]}"
		fi
	done
	for n in $(seq 1 "$rpc_name_count"); do
		name="api$n"
		mapfile -t addrs < <(rpc_addrs "$name")
		at=none
		validator=no
		for k in "${!rpcs[@]}"; do
			if printf '%s\n' "${addrs[@]}" | grep -qxF -- "${rpcs[$k]#*@}"; then
				at="RPC_HOSTS[$k]"
				break
			fi
		done
		for j in "${!validators[@]}"; do
			if printf '%s\n' "${addrs[@]}" | grep -qxF -- "${validators[$j]#*@}"; then
				validator=yes
				[ "$at" != none ] || at="VALIDATOR_HOSTS[$j]"
				break
			fi
		done
		head="${heads[n]}"
		ok=1
		if [ "$validator" = yes ] && [ -n "${retained[$name]:-}" ] && [ "${at%%\[*}" = RPC_HOSTS ]; then
			echo "retained $name: on a validator host by owner ruling"
			kept[$j]="${kept[$j]:-}${kept[$j]:+,}$name"
			validator=retained
		fi
		if [ -n "$head" ]; then
			lag=$((top - head))
			[ "$lag" -le "$rpc_max_lag" ] || ok=0
		else
			head=none
			lag=none
			ok=0
		fi
		[ "$at" != none ] && [ "$validator" != yes ] && [ "${at%%\[*}" = RPC_HOSTS ] || ok=0
		if [ "$ok" -eq 1 ]; then
			echo "pass $name host=$at validator=$validator head=$head lag=$lag"
		else
			echo "fail $name host=$at validator=$validator head=$head lag=$lag"
			failures=$((failures + 1))
		fi
	done
	for j in "${!validators[@]}"; do
		status=0
		reply="$(ssh_read "${validators[$j]}" "$placement_cmd")" || status=$?
		if [ "$status" -ne 0 ] || [ -z "$reply" ]; then
			echo "fail VALIDATOR_HOSTS[$j] ssh=$status"
			failures=$((failures + 1))
			continue
		fi
		read -r state ports sites <<<"$reply"
		if [ "$state" = inactive ] && [ "$ports" = none ]; then
			echo "pass VALIDATOR_HOSTS[$j] paxd=$state listen=$ports"
			continue
		fi
		ok=0
		if [ -n "${kept[$j]:-}" ]; then
			ok=1
			for site in ${sites//,/ }; do
				[ "$site:$ports" != none:none ] || continue
				[[ ",${kept[$j]}," == *",$site,"* ]] || ok=0
			done
		fi
		if [ "$ok" -eq 1 ]; then
			echo "retained VALIDATOR_HOSTS[$j] paxd=$state listen=$ports sites=$sites for ${kept[$j]} by owner ruling"
		else
			echo "fail VALIDATOR_HOSTS[$j] paxd=$state listen=$ports sites=${sites:-none}"
			failures=$((failures + 1))
		fi
	done
	finish "$failures"
}

check_hosts() {
	local role dest dests total answered first status failures=0
	for role in "${roles[@]}"; do
		read -r -a dests <<<"${!role}"
		total="${#dests[@]}"
		answered=0
		first=0
		for dest in "${dests[@]}"; do
			status=0
			ssh_true "$dest" || status=$?
			if [ "$status" -eq 0 ]; then
				answered=$((answered + 1))
			elif [ "$first" -eq 0 ]; then
				first=$status
			fi
		done
		if [ "$answered" -eq "$total" ]; then
			echo "pass $role reachable=$answered/$total"
		else
			echo "fail $role reachable=$answered/$total ssh=$first"
			failures=$((failures + 1))
		fi
	done
	finish "$failures"
}

# rpc <name> <method> <params>: posts one JSON-RPC request to https://<name>
# bounded by CHECK_LIVE_TIMEOUT and prints "result <value>" (a string result
# as is, a block as its number and hash) or "error <reason>".
rpc() {
	local body status=0
	body="$(curl -sS --max-time "$timeout" -H 'content-type: application/json' \
		--data "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"$2\",\"params\":$3}" \
		"https://$1" 2>&1)" || status=$?
	if [ "$status" -ne 0 ]; then
		echo "error transport $(printf '%s' "$body" | tr '\n' ' ' | cut -c1-120)"
		return 0
	fi
	printf '%s' "$body" | python3 -c '
import json
import sys

try:
    doc = json.loads(sys.stdin.read())
except ValueError:
    print("error non-json")
    sys.exit(0)
result = doc.get("result") if isinstance(doc, dict) else None
error = doc.get("error") if isinstance(doc, dict) else None
if isinstance(error, dict):
    print("error " + " ".join(str(error.get("message", error)).split())[:120])
elif isinstance(result, str):
    print("result " + result)
elif isinstance(result, dict):
    print("result " + str(result.get("number")) + " " + str(result.get("hash")))
else:
    print("error no-result")
'
}

# hex_to_dec <quantity>: prints the decimal value of a 0x-prefixed hex
# quantity and nothing for anything else.
hex_to_dec() {
	if [[ "$1" =~ ^0x[0-9a-fA-F]+$ ]]; then
		printf '%d' "$((16#${1#0x}))"
	fi
}

# The read-only inspection the archive-node check runs on the archive host:
# the public RPC name its nginx serves, the two retention keys of the hpx
# node home and the paxd unit state, one line, none of it a host address.
# shellcheck disable=SC2016
archive_facts='name=$(sed -n "s/^[[:space:]]*server_name[[:space:]]*\([a-z0-9-]*\.mainnet-beta\.paxeer\.network\);.*/\1/p" /etc/nginx/sites-enabled/*.conf 2>/dev/null | head -n 1)
retain=$(sed -n "s/^min-retain-blocks = //p" /root/.paxeer/config/app.toml 2>/dev/null)
keep=$(sed -n "s/^ss-keep-recent = //p" /root/.paxeer/config/app.toml 2>/dev/null)
unit=$(systemctl is-active paxd 2>/dev/null)
printf "%s %s %s %s\n" "${name:-none}" "${retain:-none}" "${keep:-none}" "${unit:-none}"'

check_archive_node() {
	load_rpc_inventory || { echo "fail rpc inventory=unreadable"; finish 1; }
	local failures=0 status=0 facts name retain keep unit dir i verdict value n head=0 archive="" deep hash
	facts="$(timeout "$timeout" ssh -n -o BatchMode=yes -- "$ARCHIVE_HOST" "$archive_facts" 2>/dev/null)" || status=$?
	read -r name retain keep unit <<<"$facts" || true
	if [ "$status" -ne 0 ]; then
		echo "fail ARCHIVE_HOST retention ssh=$status"
		failures=$((failures + 1))
		name=none
	elif [ "$retain" = 0 ] && [ "$keep" = 0 ] && [ "$unit" = active ]; then
		echo "pass ARCHIVE_HOST retention min-retain-blocks=0 ss-keep-recent=0 paxd=active"
	else
		echo "fail ARCHIVE_HOST retention min-retain-blocks=$retain ss-keep-recent=$keep paxd=$unit"
		failures=$((failures + 1))
	fi

	dir="$(mktemp -d)"
	trap 'rm -rf "$dir"' EXIT
	for i in "${!rpc_public_names[@]}"; do
		rpc "${rpc_public_names[$i]}" eth_blockNumber '[]' >"$dir/$i" &
	done
	wait
	for i in "${!rpc_public_names[@]}"; do
		read -r verdict value <"$dir/$i" || true
		n="$(hex_to_dec "${value:-}")"
		if [ "$verdict" = result ] && [ -n "$n" ] && [ "$n" -gt "$head" ]; then
			head=$n
		fi
	done

	verdict=""
	value=""
	if [ "$name" != none ]; then
		read -r verdict value <<<"$(rpc "$name" eth_blockNumber '[]')" || true
		if [ "$verdict" = result ]; then
			archive="$(hex_to_dec "$value")"
		fi
	fi
	if [ "$name" = none ]; then
		echo "fail ARCHIVE_HOST eth_blockNumber public-name=none"
		failures=$((failures + 1))
	elif [ -z "$archive" ]; then
		echo "fail ARCHIVE_HOST eth_blockNumber $verdict $value"
		failures=$((failures + 1))
	elif [ "$head" -gt 0 ] && [ $((head - archive)) -le 10 ]; then
		echo "pass ARCHIVE_HOST eth_blockNumber archive=$archive head=$head gap=$((head - archive))"
	else
		echo "fail ARCHIVE_HOST eth_blockNumber archive=$archive head=$head gap=$((head - archive))"
		failures=$((failures + 1))
	fi

	if [ -z "$archive" ]; then
		echo "fail ARCHIVE_HOST eth_getBlockByNumber no-archive-head"
		failures=$((failures + 1))
	else
		deep=$((archive - 200000))
		hash=""
		read -r verdict value hash <<<"$(rpc "$name" eth_getBlockByNumber "[\"$(printf '0x%x' "$deep")\",false]")" || true
		if [ "$verdict" = result ] && [ "$(hex_to_dec "$value")" = "$deep" ] && [[ "$hash" =~ ^0x[0-9a-f]{64}$ ]]; then
			echo "pass ARCHIVE_HOST eth_getBlockByNumber height=$deep hash=$hash"
		else
			echo "fail ARCHIVE_HOST eth_getBlockByNumber height=$deep $verdict $value $hash"
			failures=$((failures + 1))
		fi
	fi
	finish "$failures"
}

check_ca() {
	local table service target custody eku sans var host svc cert one chain
	local got san missing n m days line failures=0 attestors trust
	if [ ! -r "$ca_dir/ca.pem" ]; then
		echo "fail ca ca cert=absent"
		finish 1
	fi
	days="$(days_left <"$ca_dir/ca.pem")"
	if [ "$days" -gt 30 ]; then
		echo "pass ca ca expires_in=${days}d"
	else
		echo "fail ca ca expires_in=${days}d"
		failures=$((failures + 1))
	fi
	table="$("$(dirname "${BASH_SOURCE[0]}")/ca.sh" services)"
	attestors="$(sed -n 's/^attestor_services="\(.*\)"$/\1/p' "$(dirname "${BASH_SOURCE[0]}")/ca.sh")"
	while read -r service target custody _ eku sans; do
		trust="$ca_dir/ca.pem"
		if [[ " $attestors " == *" $service "* ]]; then
			trust="${LAYERX_ATTESTOR_CA_DIR:-}/ca.pem"
			if [ -z "${LAYERX_ATTESTOR_CA_DIR:-}" ] || [ ! -r "$trust" ]; then
				echo "fail $service target=$target attestor_ca=absent LAYERX_ATTESTOR_CA_DIR=${LAYERX_ATTESTOR_CA_DIR:-unset}"
				failures=$((failures + 1))
				continue
			fi
		fi
		cert=""
		case "$target" in
		box:*)
			var="${target#box:}"
			host="${!var:-}"
			if [ -z "$host" ]; then
				echo "fail $service target=$target host=absent"
				failures=$((failures + 1))
				continue
			fi
			cert="$(ssh_read "$host" "cat $tls_dir/$service/cert.pem")" || cert=""
			;;
		railway:*)
			for svc in $(tr , ' ' <<<"${target#railway:}"); do
				one="$(railway_cert "$svc" "$custody")" || one=""
				if [ -z "$one" ]; then
					cert=""
					break
				fi
				if [ -n "$cert" ] && [ "$one" != "$cert" ]; then
					cert=differs
					break
				fi
				cert="$one"
			done
			;;
		esac
		if [ -z "$cert" ]; then
			echo "fail $service target=$target cert=absent"
			failures=$((failures + 1))
			continue
		fi
		if [ "$cert" = differs ]; then
			echo "fail $service target=$target cert=differs"
			failures=$((failures + 1))
			continue
		fi
		chain=ok
		openssl verify -CAfile "$trust" <<<"$cert" >/dev/null 2>&1 || chain=untrusted
		[ "$sans" != - ] || sans=""
		got="$(openssl x509 -noout -ext subjectAltName <<<"$cert" 2>/dev/null | tail -n +2 | sed 's/IP Address:/IP:/g; s/, /,/g; s/^ *//')"
		missing=""
		n=0
		m=0
		while read -r -d, san; do
			[ -n "$san" ] || continue
			m=$((m + 1))
			if [[ ",$got," == *",$san,"* ]]; then
				n=$((n + 1))
			else
				missing="${missing:+$missing,}$san"
			fi
		done <<<"${sans:+$sans,}"
		days="$(days_left <<<"$cert")"
		line="$service target=$target chain=$chain san=$n/$m${missing:+ missing=$missing} expires_in=${days}d"
		if [ "$chain" = ok ] && [ -z "$missing" ] && [ "$days" -gt 30 ]; then
			echo "pass $line"
		else
			echo "fail $line"
			failures=$((failures + 1))
		fi
	done <<<"$table"
	finish "$failures"
}

# check_hpx: reads the hpx registry at its public origin and at the fly.dev
# name of the Fly app of tools/hpx/hosting/fly.toml: at both, /healthz with the chain
# id and the source revision, checksums.txt verified against every served
# artifact it lists and /api/nodes answering; the landing page at the public
# origin; and the public origin served by the app: its /healthz carries the
# Fly edge's fly-request-id header and matches the app's /healthz.
check_hpx() {
	local origin="${CHECK_LIVE_HPX_ORIGIN:-https://node.hyperpaxeer.com}" failures=0
	local status body verdict manifest line sum path served total verified first
	local entry='^([0-9a-f]{64})  ([A-Za-z0-9._-]+(/[A-Za-z0-9._-]+)*)$'
	local app app_origin base prefix code name_health app_health fly_id
	origin="${origin%/}"
	app="$(fly_app tools/hpx/hosting/fly.toml)" || app=absent
	app_origin="${CHECK_LIVE_HPX_APP_ORIGIN:-https://$app.fly.dev}"
	app_origin="${app_origin%/}"

	for prefix in "" app-; do
		base="$origin"
		[ -z "$prefix" ] || base="$app_origin"

		status=0
		body="$(curl -sS --max-time "$timeout" -w '\n%{http_code}' "$base/healthz" 2>&1)" || status=$?
		if [ "$status" -ne 0 ]; then
			verdict="fail transport $(printf '%s' "$body" | tr '\n' ' ' | cut -c1-200)"
		else
			verdict="$(printf '%s' "$body" | python3 -c '
import json
import re
import sys

raw = sys.stdin.read()
text, _, code = raw.rpartition("\n")
try:
    doc = json.loads(text)
except ValueError:
    print("fail http=" + code + " non-json " + " ".join(text.split())[:200])
    sys.exit(0)
if not isinstance(doc, dict):
    print("fail http=" + code + " not-health " + json.dumps(doc)[:200])
    sys.exit(0)
chain = doc.get("chain_id")
rev = doc.get("source_revision")
ok = (
    code == "200"
    and doc.get("ok") is True
    and chain == "hyperpax_125-1"
    and isinstance(rev, str)
    and re.fullmatch(r"[0-9a-f]{40}", rev) is not None
)
print(("pass " if ok else "fail ") + "http=" + code + " ok=" + str(doc.get("ok") is True).lower()
      + " chain_id=" + str(chain) + " source_revision=" + str(rev))
')"
		fi
		case "$verdict" in
		pass\ *) echo "pass ${prefix}healthz ${verdict#pass }" ;;
		*)
			echo "fail ${prefix}healthz ${verdict#fail }"
			failures=$((failures + 1))
			;;
		esac

		total=0
		verified=0
		first=""
		status=0
		manifest="$(curl -sS --max-time "$timeout" "$base/checksums.txt" 2>&1)" || status=$?
		if [ "$status" -ne 0 ]; then
			echo "fail ${prefix}checksums transport $(printf '%s' "$manifest" | tr '\n' ' ' | cut -c1-200)"
			failures=$((failures + 1))
		else
			while IFS= read -r line; do
				[ -n "$line" ] || continue
				total=$((total + 1))
				if [[ "$line" =~ $entry ]]; then
					sum="${BASH_REMATCH[1]}"
					path="${BASH_REMATCH[2]}"
					status=0
					served="$(curl -sS --max-time "$timeout" "$base/$path" 2>/dev/null | sha256sum | cut -d' ' -f1)" || status=$?
					if [ "$status" -eq 0 ] && [ "$served" = "$sum" ]; then
						verified=$((verified + 1))
					else
						first="${first:-$path}"
					fi
				else
					first="${first:-malformed:$(printf '%s' "$line" | cut -c1-80)}"
				fi
			done <<<"$manifest"
			if [ "$total" -gt 0 ] && [ "$verified" -eq "$total" ]; then
				echo "pass ${prefix}checksums verified=$verified/$total"
			else
				echo "fail ${prefix}checksums verified=$verified/$total first=${first:-empty-manifest}"
				failures=$((failures + 1))
			fi
		fi

		status=0
		body="$(curl -sS --max-time "$timeout" -w '\n%{http_code}' "$base/api/nodes" 2>&1)" || status=$?
		if [ "$status" -ne 0 ]; then
			verdict="fail transport $(printf '%s' "$body" | tr '\n' ' ' | cut -c1-200)"
		else
			verdict="$(printf '%s' "$body" | python3 -c '
import json
import sys

raw = sys.stdin.read()
text, _, code = raw.rpartition("\n")
try:
    doc = json.loads(text)
except ValueError:
    print("fail http=" + code + " non-json " + " ".join(text.split())[:200])
    sys.exit(0)
if not isinstance(doc, dict):
    print("fail http=" + code + " not-nodes " + json.dumps(doc)[:200])
    sys.exit(0)
chain = doc.get("chain_id")
count = doc.get("count")
nodes = doc.get("nodes")
ok = (
    code == "200"
    and chain == "hyperpax_125-1"
    and isinstance(count, int)
    and isinstance(nodes, list)
    and len(nodes) == count
)
print(("pass " if ok else "fail ") + "http=" + code + " chain_id=" + str(chain) + " count=" + str(count))
')"
		fi
		case "$verdict" in
		pass\ *) echo "pass ${prefix}api-nodes ${verdict#pass }" ;;
		*)
			echo "fail ${prefix}api-nodes ${verdict#fail }"
			failures=$((failures + 1))
			;;
		esac
	done

	status=0
	body="$(curl -sS --max-time "$timeout" -w '\n%{http_code}' "$origin/" 2>&1)" || status=$?
	code="${body##*$'\n'}"
	if [ "$status" -ne 0 ]; then
		echo "fail landing transport $(printf '%s' "$body" | tr '\n' ' ' | cut -c1-200)"
		failures=$((failures + 1))
	elif [ "$code" = 200 ] && grep -qF '<title>HPX — HyperPax Node Network</title>' <<<"$body"; then
		echo "pass landing http=200"
	else
		echo "fail landing http=$code"
		failures=$((failures + 1))
	fi

	# The public name is served by the app when its answer passed the Fly edge
	# (fly-request-id) and equals the app's own answer; the registry on the
	# edge host answers without that header.
	name_health="$(curl -sS --max-time "$timeout" -D - "$origin/healthz" 2>/dev/null)" || name_health=""
	fly_id=absent
	grep -qiE '^fly-request-id: *[^[:space:]]' <<<"$name_health" && fly_id=present
	name_health="${name_health#*$'\r\n\r\n'}"
	app_health="$(curl -sS --max-time "$timeout" "$app_origin/healthz" 2>/dev/null)" || app_health=""
	if [ "$fly_id" = present ] && [ -n "$app_health" ] && [ "$name_health" = "$app_health" ]; then
		echo "pass served-by app=$app fly-request-id=present healthz=match"
	else
		code=differ
		if [ -n "$app_health" ] && [ "$name_health" = "$app_health" ]; then
			code=match
		fi
		echo "fail served-by app=$app fly-request-id=$fly_id healthz=$code"
		failures=$((failures + 1))
	fi
	finish "$failures"
}

# earliest <name>: asks https://<name> for block 1 and prints the lowest
# height the node still serves: 1 when block 1 answers, the height its pruning
# error names otherwise, nothing when neither can be read.
earliest() {
	curl -sS --max-time "$timeout" -H 'content-type: application/json' \
		--data '{"jsonrpc":"2.0","id":1,"method":"eth_getBlockByNumber","params":["0x1",false]}' \
		"https://$1" 2>/dev/null | python3 -c '
import json
import re
import sys

try:
    doc = json.loads(sys.stdin.read())
except ValueError:
    sys.exit(0)
if not isinstance(doc, dict):
    sys.exit(0)
if isinstance(doc.get("result"), dict):
    print(1)
    sys.exit(0)
error = doc.get("error")
match = re.search(r"earliest available height (\d+)", str(error.get("message", "")) if isinstance(error, dict) else "")
if match:
    print(match.group(1))
'
}

# http_code <url>: prints the HTTP status of one GET bounded by
# CHECK_LIVE_TIMEOUT, or "transport" when no answer arrived.
http_code() {
	curl -sS --max-time "$timeout" -o /dev/null -w '%{http_code}' "$1" 2>/dev/null || echo transport
}

# sql <variable> <query>: runs one read-only query against the database whose
# connection string the named variable holds and prints the unaligned rows;
# never prints the connection string or psql's diagnostics.
sql() {
	PGCONNECT_TIMEOUT="$timeout" psql -X -At -F ' ' -v ON_ERROR_STOP=1 -d "${!1}" -c "$2" 2>/dev/null
}

# ranges_inside: reads "dst" and "src" lines of "<kind> <a> <b>" block ranges
# on stdin and prints "inside" when every dst range lies inside the union of
# the src ranges, or the first dst range that does not.
ranges_inside='
import sys

dst, src = [], []
for line in sys.stdin:
    parts = line.split()
    if parts and parts[0] == "error":
        print("error " + " ".join(parts[1:]))
        sys.exit(0)
    if len(parts) != 3:
        continue
    lo, hi = sorted((int(parts[1]), int(parts[2])))
    (dst if parts[0] == "dst" else src).append((lo, hi))
merged = []
for lo, hi in sorted(src):
    if merged and lo <= merged[-1][1] + 1:
        merged[-1] = (merged[-1][0], max(merged[-1][1], hi))
    else:
        merged.append((lo, hi))
for lo, hi in sorted(dst):
    if not any(a <= lo and hi <= b for a, b in merged):
        print("outside %d-%d" % (lo, hi))
        sys.exit(0)
print("inside")
'

check_explorer() {
	load_rpc_inventory || { echo "fail rpc inventory=unreadable"; finish 1; }
	local origin="${CHECK_LIVE_EXPLORER_ORIGIN:-https://paxscan.io}" failures=0
	local dir i verdict value n head=0 floor="" code envs api rpc_url name="" archive_low="" var missing=""
	local first ceiling dst legacy paxscan answer
	origin="${origin%/}"

	dir="$(mktemp -d)"
	trap 'rm -rf "$dir"' EXIT
	for i in "${!rpc_public_names[@]}"; do
		rpc "${rpc_public_names[$i]}" eth_blockNumber '[]' >"$dir/head.$i" &
		earliest "${rpc_public_names[$i]}" >"$dir/low.$i" &
	done
	wait
	for i in "${!rpc_public_names[@]}"; do
		read -r verdict value <"$dir/head.$i" || true
		n="$(hex_to_dec "${value:-}")"
		if [ "$verdict" = result ] && [ -n "$n" ] && [ "$n" -gt "$head" ]; then
			head=$n
		fi
		echo "$n" >"$dir/n.$i"
	done

	code="$(http_code "$origin")"
	envs="$(curl -sS --max-time "$timeout" "$origin/assets/envs.js" 2>/dev/null)" || envs=""
	read -r api rpc_url <<<"$(printf '%s' "$envs" | python3 -c '
import re
import sys

text = sys.stdin.read()
def get(key):
    match = re.search(key + r"\s*:\s*\"([^\"]*)\"", text)
    return match.group(1) if match else ""
host = get("NEXT_PUBLIC_API_HOST")
proto = get("NEXT_PUBLIC_API_PROTOCOL") or "https"
print((proto + "://" + host if host else "none") + " " + (get("NEXT_PUBLIC_NETWORK_RPC_URL") or "none"))
')" || true
	for i in "${!rpc_public_names[@]}"; do
		if [ "${rpc_url%/}" = "https://${rpc_public_names[$i]}" ]; then
			name="${rpc_public_names[$i]}"
			archive_low="$(cat "$dir/low.$i")"
		fi
	done
	if [ "$code" = 200 ] && [ -n "$name" ]; then
		echo "pass frontend $origin http=200 rpc=$name"
	else
		echo "fail frontend $origin http=$code rpc=${rpc_url:-none}"
		failures=$((failures + 1))
	fi

	# The pruned floor: the lowest height any other name at the head serves.
	for i in "${!rpc_public_names[@]}"; do
		n="$(cat "$dir/n.$i")"
		value="$(cat "$dir/low.$i")"
		if [ "${rpc_public_names[$i]}" != "$name" ] && [ -n "$n" ] && [ -n "$value" ] && [ $((head - n)) -le 10 ]; then
			if [ -z "$floor" ] || [ "$value" -lt "$floor" ]; then
				floor=$value
			fi
		fi
	done
	if [ -n "$archive_low" ] && [ -n "$floor" ] && [ "$archive_low" -lt "$floor" ]; then
		echo "pass archive $name first_retained=$archive_low pruned_floor=$floor"
	else
		echo "fail archive ${name:-none} first_retained=${archive_low:-none} pruned_floor=${floor:-none}"
		failures=$((failures + 1))
	fi

	for value in /api/health /api/v2/stats; do
		code="none"
		[ "$api" = none ] || code="$(http_code "$api$value")"
		if [ "$code" = 200 ]; then
			echo "pass backend $value http=200"
		else
			echo "fail backend $value http=$code"
			failures=$((failures + 1))
		fi
	done

	code="none"
	if [ "$api" != none ] && [ -n "$archive_low" ]; then
		code="$(http_code "$api/api/v2/blocks/$archive_low")"
	fi
	if [ "$code" = 200 ]; then
		echo "pass backend block=$archive_low below pruned_floor=$floor"
	else
		echo "fail backend block=${archive_low:-none} http=$code"
		failures=$((failures + 1))
	fi

	for var in EXPLORER_DATABASE_URL EXPLORER_LEGACY_DATABASE_URL PAXSCAN_DATABASE_PUBLIC_URL; do
		[ -n "${!var:-}" ] || missing="$missing${missing:+,}$var"
	done
	if [ -n "$missing" ] || [ -z "$archive_low" ]; then
		echo "fail history unset=${missing:-none} first_retained=${archive_low:-none}"
		finish $((failures + 2))
	fi

	first="$(sql EXPLORER_DATABASE_URL 'SELECT min(number) FROM blocks WHERE consensus')" || first=""
	ceiling="$(sql EXPLORER_LEGACY_DATABASE_URL 'SELECT max(number) FROM blocks WHERE consensus')" || ceiling=""
	if ! [[ "$first" =~ ^[0-9]+$ && "$ceiling" =~ ^[0-9]+$ ]]; then
		echo "fail history first=${first:-none} legacy_ceiling=${ceiling:-none}"
		finish $((failures + 2))
	fi
	dst="$(sql EXPLORER_DATABASE_URL "SELECT count(*) FROM blocks WHERE consensus AND number BETWEEN $first AND $((archive_low - 1))")" || dst=""
	legacy="$(sql EXPLORER_LEGACY_DATABASE_URL "SELECT count(*) FROM blocks WHERE consensus AND number BETWEEN $first AND $ceiling")" || legacy=""
	paxscan="$(sql PAXSCAN_DATABASE_PUBLIC_URL "SELECT count(*) FROM blocks WHERE consensus AND number > $ceiling AND number < $archive_low")" || paxscan=""
	if [[ "$dst" =~ ^[0-9]+$ && "$legacy" =~ ^[0-9]+$ && "$paxscan" =~ ^[0-9]+$ ]] && [ "$dst" -eq $((legacy + paxscan)) ]; then
		echo "pass history blocks=$dst from=$first legacy=$legacy paxscan=$paxscan ceiling=$ceiling"
	else
		echo "fail history blocks=${dst:-none} from=$first legacy=${legacy:-none} paxscan=${paxscan:-none} ceiling=$ceiling"
		failures=$((failures + 1))
	fi

	answer="$(
		{
			sql EXPLORER_DATABASE_URL "SELECT 'dst', from_number, to_number FROM missing_block_ranges WHERE greatest(from_number, to_number) >= $first" || echo "error explorer-database"
			sql EXPLORER_LEGACY_DATABASE_URL "SELECT 'src', from_number, to_number FROM missing_block_ranges" || echo "error legacy-database"
			sql PAXSCAN_DATABASE_PUBLIC_URL "SELECT 'src', from_number, to_number FROM missing_block_ranges" || echo "error paxscan-database"
		} | python3 -c "$ranges_inside"
	)"
	if [ "$answer" = inside ]; then
		echo "pass missing-ranges inside-source-lost from=$first"
	else
		echo "fail missing-ranges $answer from=$first"
		failures=$((failures + 1))
	fi
	finish "$failures"
}

# check_validators: from this host the attestor ports 8480 and 8481 of every
# VALIDATOR_HOSTS destination refuse the connection (a tcp reset, not a
# timeout); over ssh each attestor answers /health with 200 on loopback, and
# each validator host reaches every other one's attestor ports with 200. One
# line per check, naming hosts by their index only:
#   "pass VALIDATOR_HOSTS[k] remote-<port>=refused" or "=open|timeout|unreachable"
#   "pass VALIDATOR_HOSTS[k] loopback-<port> health=200"
#   "pass VALIDATOR_HOSTS[k] peer-VALIDATOR_HOSTS[j]-<port> health=200"
# with fail and the observed code (ssh=<exit> when the host did not answer).
# With XWEB_ATTESTORS_ON_FLY=yes in the host map the loopback and peer checks
# give way to "pass VALIDATOR_HOSTS[k] listeners-8480-8481=0": nothing listens
# on the attestor ports.
check_validators() {
	local -a dests addrs
	local k j port err status code line target failures=0
	read -r -a dests <<<"$VALIDATOR_HOSTS"
	for k in "${!dests[@]}"; do
		addrs[k]="$(ssh -G -- "${dests[$k]}" 2>/dev/null | awk '$1 == "hostname" { print $2; exit }')"
	done
	for k in "${!dests[@]}"; do
		for port in 8480 8481; do
			status=0
			# A bare connect through bash's /dev/tcp, because only its error
			# text (the system's ECONNREFUSED message) tells a tcp reset from
			# an unreachable host; curl reports both as exit 7.
			# shellcheck disable=SC2016
			err="$(timeout "$timeout" bash -c 'exec 3<>"/dev/tcp/$1/$2"' - "${addrs[k]}" "$port" 2>&1)" || status=$?
			case "$status:$err" in
			0:*) line="fail VALIDATOR_HOSTS[$k] remote-$port=open" ;;
			124:*) line="fail VALIDATOR_HOSTS[$k] remote-$port=timeout" ;;
			*"Connection refused"*) line="pass VALIDATOR_HOSTS[$k] remote-$port=refused" ;;
			*) line="fail VALIDATOR_HOSTS[$k] remote-$port=unreachable" ;;
			esac
			echo "$line"
			[ "${line%% *}" = pass ] || failures=$((failures + 1))
		done
		# Once the four attestors run as Fly apps (XWEB_ATTESTORS_ON_FLY=yes
		# in the host map), nothing listens on the attestor ports here.
		if [ "${XWEB_ATTESTORS_ON_FLY:-}" = yes ]; then
			status=0
			code="$(ssh_read "${dests[$k]}" 'ss -Hltn "( sport = :8480 or sport = :8481 )" | wc -l')" || status=$?
			if [ "$status" -ne 0 ]; then
				echo "fail VALIDATOR_HOSTS[$k] listeners-8480-8481 ssh=$status"
				failures=$((failures + 1))
			elif [ "$code" = 0 ]; then
				echo "pass VALIDATOR_HOSTS[$k] listeners-8480-8481=0"
			else
				echo "fail VALIDATOR_HOSTS[$k] listeners-8480-8481=${code:-none}"
				failures=$((failures + 1))
			fi
			continue
		fi
		for port in 8480 8481; do
			for j in "${!dests[@]}"; do
				target="${addrs[j]}"
				line="peer-VALIDATOR_HOSTS[$j]-$port"
				if [ "$j" -eq "$k" ]; then
					target=127.0.0.1
					line="loopback-$port"
				fi
				status=0
				code="$(ssh_read "${dests[$k]}" "curl -s -o /dev/null -w %{http_code} -m $timeout http://$target:$port/health")" || status=$?
				if [ "$code" = 200 ]; then
					echo "pass VALIDATOR_HOSTS[$k] $line health=200"
				elif [ "$status" -eq 255 ] || [ "$status" -eq 124 ]; then
					echo "fail VALIDATOR_HOSTS[$k] $line ssh=$status"
					failures=$((failures + 1))
				else
					echo "fail VALIDATOR_HOSTS[$k] $line health=${code:-none}"
					failures=$((failures + 1))
				fi
			done
		done
	done
	finish "$failures"
}

# check_identity: the rollout's identity service runs one replica with a
# volume and no public domain. Its Railway private name answers readiness
# over TLS under the internal CA, sent to the service through Railway ssh.
check_identity() {
	local app from answer url code body attempt failures=0
	if ! app="$(railway_service identity)"; then
		echo "fail identity rollout=absent"
		finish 1
	fi
	railway_deployment "$app" /data || failures=$((failures + 1))
	answer="$(railway_domains "$app")" || answer=none
	if [ "$answer" = 0 ]; then
		echo "pass public-domains service=$app count=0"
	else
		echo "fail public-domains service=$app count=${answer:-none}"
		failures=$((failures + 1))
	fi
	url="https://$app.railway.internal:9443/readyz"
	from="$app"
	if [ ! -r "$ca_dir/ca.pem" ]; then
		echo "fail readiness app=$app ca=absent"
		finish $((failures + 1))
	fi
	answer=""
	for attempt in 1 2; do
		answer="$(railway_ssh "$from" "cat >/tmp/identity-ca.pem && curl -sS -m $timeout --cacert /tmp/identity-ca.pem -w \" %{http_code}\" $url; rm -f /tmp/identity-ca.pem" <"$ca_dir/ca.pem" | tail -n 1)" || true
		[ "${answer##* }" != 200 ] || break
		[ "$attempt" = 2 ] || sleep 10
	done
	code="${answer##* }"
	body="${answer% *}"
	if [ "$code" = 200 ] && [[ "$body" == *'"status":"ready"'* ]]; then
		echo "pass readiness app=$app from=$from url=$url http=200 status=ready"
	else
		echo "fail readiness app=$app from=$from url=$url http=${code:-none}"
		failures=$((failures + 1))
	fi
	finish "$failures"
}

# check_search_front: search.paxeer.network answers /healthz 200 over a
# certificate valid for the name, through the edge host that proxies it to the
# app of services/interop/deploy/search-front/fly.toml; the app runs
# at least two started machines in two regions; the upstream list a machine
# mounts equals the serving RPC names of tools/bringup/search-front.sh names
# and holds no validator host's apiN name; two requests from this address get
# the same X-Search-Node, one of those names. One line per check.
check_search_front() {
	local host=search.paxeer.network
	local app status headers code listing mounted expected validators listed="" first second failures=0
	if ! command -v "$railway_bin" >/dev/null 2>&1; then
		echo "check-live: $railway_bin is required" >&2
		exit 2
	fi
	if ! app="$(railway_service search-front)"; then
		echo "fail search-front rollout=absent"
		finish 1
	fi

	status=0
	headers="$(curl -sS --max-time "$timeout" -D - -o /dev/null "https://$host/healthz" 2>&1)" || status=$?
	code="$(sed -n '1s/^HTTP\/[0-9.]* \([0-9]*\).*/\1/p' <<<"$headers")"
	if [ "$status" -ne 0 ]; then
		echo "fail healthz https://$host/healthz transport $(printf '%s' "$headers" | tr '\n' ' ' | cut -c1-160)"
		failures=$((failures + 1))
	elif [ "$code" = 200 ]; then
		echo "pass healthz https://$host/healthz http=200 tls=verified"
	else
		echo "fail healthz https://$host/healthz http=${code:-none} tls=verified"
		failures=$((failures + 1))
	fi

	railway_deployment "$app" || failures=$((failures + 1))

	if ! listing="$("$(dirname "${BASH_SOURCE[0]}")/search-front.sh" names 2>&1)"; then
		echo "fail upstreams names=unreadable $(printf '%s' "$listing" | tr '\n' ' ' | cut -c1-160)"
		failures=$((failures + 1))
		expected=""
	else
		expected="$(sed -n 's/^serve //p' <<<"$listing" | sort)"
		validators="$(sed -n 's/^validator //p' <<<"$listing" | sort)"
		mounted="$(railway_ssh "$app" "cat /etc/nginx/search/upstreams.conf" </dev/null)" || mounted=""
		if listed="$("$repo_root/tools/bringup/search-front.sh" qualify <<<"$mounted")"; then
			echo "pass upstreams pinned=$listed eligible=1 validator=0 readiness=typed"
		else
			echo "fail upstreams pin-or-readiness=invalid"
			failures=$((failures + 1))
			listed=""
		fi
	fi

	first="$(curl -sS --max-time "$timeout" -D - -o /dev/null "https://$host/xweb/health" 2>/dev/null | sed -n 's/^x-search-node:[[:space:]]*//Ip' | tr -d '\r')" || first=""
	second="$(curl -sS --max-time "$timeout" -D - -o /dev/null "https://$host/xweb/health" 2>/dev/null | sed -n 's/^x-search-node:[[:space:]]*//Ip' | tr -d '\r')" || second=""
	if [ -n "$first" ] && [ "$first" = "$second" ] && [ "$first" = "$listed" ]; then
		echo "pass sticky node=$first requests=2"
	else
		echo "fail sticky first=${first:-none} second=${second:-none}"
		failures=$((failures + 1))
	fi
	finish "$failures"
}

# The registered edge contract is shared by edge and ramp. Decode every row;
# a malformed row fails qualification instead of silently disappearing.
# shellcheck disable=SC2016
edge_cmd='cat "$nginx_dir/edge/manifest" 2>/dev/null; echo @@sites; for f in "$nginx_dir"/sites-available/* "$nginx_dir"/edge/stream/*.conf; do head -n 1 "$f" 2>/dev/null | grep -q "^# rendered by tools/bringup/edge.sh" && cat "$f"; done; true'

edge_read() {
	local command
	printf -v command 'nginx_dir=%q; %s' "${CHECK_LIVE_EDGE_NGINX_DIR:-/etc/nginx}" "$edge_cmd"
	ssh_read "$EDGE_HOST" "$command"
}

edge_manifest() {
	bringup_inspect edge-contract
}

# edge_probe <six manifest fields>: verify the actual public and upstream
# responses. Upstream requests originate on the edge, where private box routes
# and the box CA exist. TLS URLs retain SNI while --connect-to selects the
# manifest's actual host:port. No provider request-id or Fly metadata is needed.
edge_probe() {
	local name="$1" mode="$2" upstream="$3" port="$4" kind="$5" sni="$6"
	local addrs edge_addrs first fp_edge fp_up public direct command identity scheme host ca="" route
	local members member payload='' failures=0
	edge_addrs="$(getent ahosts "${EDGE_HOST#*@}" 2>/dev/null | awk '{print $1}' | sort -u)"
	addrs="$(getent ahosts "$name" 2>/dev/null | awk '{print $1}' | sort -u)"
	if [ -z "$addrs" ] || [ -z "$edge_addrs" ] || [ -n "$(comm -23 <(printf '%s\n' "$addrs") <(printf '%s\n' "$edge_addrs"))" ]; then
		echo "fail $name mode=$mode edge=no"
		return 1
	fi
	if [ "$mode" = stream ]; then
		first="$(head -n 1 <<<"$addrs")"
		fp_edge="$(timeout "$timeout" openssl s_client -connect "$first:$port" -servername "$name" </dev/null 2>/dev/null | openssl x509 -noout -fingerprint -sha256 2>/dev/null)" || fp_edge=""
		printf -v command 'timeout %q openssl s_client -connect %q -servername %q </dev/null 2>/dev/null | openssl x509 -noout -fingerprint -sha256 2>/dev/null' "$timeout" "$upstream" "$name"
		fp_up="$(ssh_read "$EDGE_HOST" "$command")" || fp_up=""
		if [ -n "$fp_edge" ] && [ "$fp_edge" = "$fp_up" ]; then
			echo "pass $name mode=stream edge=yes certificate=match"
			return 0
		fi
		echo "fail $name mode=stream certificate=differ"
		return 1
	fi
	route="${CHECK_LIVE_EDGE_READINESS_PATH:-/readyz}"
	if [ "$mode" = rpc ]; then
		route=/
		payload='{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
	fi
	local -a request=()
	[ -z "$payload" ] || request=(-H 'Content-Type: application/json' --data "$payload")
	public="$(curl -fsS --max-time "$timeout" "${request[@]}" "https://$name$route")" || public=""
	if [ -z "$public" ]; then
		echo "fail $name mode=$mode public=unready"
		return 1
	fi
	if [ "$mode" = rpc ]; then
		if ! python3 -c 'import json,sys; v=json.load(sys.stdin); assert v.get("jsonrpc") == "2.0" and type(v.get("id")) is int and v["id"] == 1 and v.get("result") == "0x7d" and "error" not in v' <<<"$public"; then
			echo "fail $name mode=rpc chain-id=invalid"
			return 1
		fi
		members="$(bringup_inspect edge-contract "$name" "$port" <<<"$edge_reply")" || return 1
		while read -r member; do
			printf -v command 'curl -fsS --max-time %q -H %q --data %q %q' "$timeout" 'Content-Type: application/json' "$payload" "https://$member/"
			direct="$(ssh_read "$EDGE_HOST" "$command")" || direct=""
			if ! python3 -c 'import json,sys; a,b=map(json.loads,sys.argv[1:]); assert a == b' "$public" "$direct"; then
				failures=$((failures + 1))
			fi
		done <<<"$members"
	else
		scheme=http
		identity="${upstream%:*}"
		[ "$sni" = - ] || identity="$sni"
		host="$name"
		if [ "$kind" = railway ]; then scheme=https; host="$identity"; fi
		if [ "$kind" = box ] && [ "$sni" != - ]; then scheme=https; printf -v ca ' --cacert %q' "${CHECK_LIVE_EDGE_BOX_CA:-/etc/layerx/ca.crt}"; fi
		printf -v command 'curl -fsS --max-time %q%s --connect-to %q -H %q %q' "$timeout" "$ca" "$identity:${upstream##*:}:$upstream" "Host: $host" "$scheme://$identity:${upstream##*:}$route"
		direct="$(ssh_read "$EDGE_HOST" "$command")" || direct=""
		[ "$public" = "$direct" ] || failures=1
	fi
	if [ "$failures" = 0 ]; then
		echo "pass $name mode=$mode kind=$kind edge=yes tls=verified body=match"
		return 0
	fi
	echo "fail $name mode=$mode kind=$kind upstream=differ"
	return 1
}

# The ramp consumer uses the same rendered-route and upstream contract as edge.
check_ramp_edge() {
	local ramp_name="$1" edge_reply manifest line name mode upstream port kind sni
	edge_reply="$(edge_read)" || { echo "fail edge $ramp_name manifest=unreadable"; return 1; }
	manifest="$(edge_manifest <<<"$edge_reply")" || { echo "fail edge $ramp_name manifest=invalid"; return 1; }
	line="$(bringup_inspect edge-ramp "$ramp_name" <<<"$edge_reply")" || line=""
	if [ -z "$line" ] || [ "$(wc -l <<<"$line")" != 1 ]; then
		echo "fail edge $ramp_name route=absent-or-ambiguous"
		return 1
	else
		read -r name mode upstream port kind sni <<<"$line"
		CHECK_LIVE_EDGE_READINESS_PATH=/readyz edge_probe "$name" "$mode" "$upstream" "$port" "$kind" "$sni" || return 1
	fi

}

check_edge() {
	local edge_reply manifest sites v named=0 name mode upstream port kind sni failures=0
	edge_reply="$(edge_read)" || { echo 'fail EDGE_HOST manifest=unreadable'; finish 1; }
	manifest="$(edge_manifest <<<"$edge_reply")" || { echo 'fail manifest contract=invalid'; finish 1; }
	echo "pass manifest names=$(grep -c . <<<"$manifest")"
	sites="${edge_reply#*@@sites}"
	for v in $VALIDATOR_HOSTS; do
		if grep -qF -- "${v#*@}" <<<"$sites"; then named=$((named + 1)); fi
	done
	if [ "$named" = 0 ]; then echo 'pass sites validator=0'; else echo "fail sites validator=$named"; failures=$((failures + 1)); fi
	while read -r name mode upstream port kind sni; do
		edge_probe "$name" "$mode" "$upstream" "$port" "$kind" "$sni" || failures=$((failures + 1))
	done <<<"$manifest"
	finish "$failures"
}

# check_ci: the CI controller app of tools/flyci/controller/fly.toml runs one
# started machine, the repository variable CI_LINUX_RUNNER is the runner label
# the controller serves, and runner-canary.yml dispatched on the default
# branch completes with conclusion success and a successful job on a runner
# whose name starts with fly-. A run still unfinished after the wait is
# cancelled. One line per check.
ci_dispatch_id() {
	python3 -c '
import json, sys
try:
    value = json.load(sys.stdin)
    run = value["workflow_run_id"]
    if type(run) is not int or run <= 0:
        raise ValueError("dispatch run identity")
    print(run)
except (ValueError, KeyError, TypeError):
    sys.exit(1)
'
}

ci_canary_evidence() {
	python3 -c '
import json, re, sys
try:
    expected_run, branch, candidate, workflow = sys.argv[1:]
    if not re.fullmatch(r"[1-9][0-9]*", expected_run) or not re.fullmatch(r"[0-9a-f]{40}", candidate):
        raise ValueError("expected identity")
    decoder = json.JSONDecoder()
    raw = sys.stdin.read().strip()
    run, offset = decoder.raw_decode(raw)
    pages = json.loads(raw[offset:].strip())
    if type(run.get("id")) is not int or run["id"] != int(expected_run):
        raise ValueError("run identity")
    if run.get("event") != "workflow_dispatch" or run.get("head_branch") != branch or run.get("head_sha") != candidate:
        raise ValueError("source identity")
    if run.get("path", "").split("@")[0] != ".github/workflows/" + workflow:
        raise ValueError("workflow identity")
    if run.get("status") != "completed" or run.get("conclusion") != "success":
        raise ValueError("run incomplete or failed")
    if not isinstance(pages, list) or not pages:
        raise ValueError("job pages")
    jobs, seen = [], set()
    total = pages[0].get("total_count")
    if type(total) is not int or total <= 0:
        raise ValueError("job count")
    for page in pages:
        if page.get("total_count") != total or not isinstance(page.get("jobs"), list):
            raise ValueError("job inventory")
        for job in page["jobs"]:
            ident = job.get("id")
            if type(ident) is not int or ident <= 0 or ident in seen or type(job.get("run_id")) is not int or job["run_id"] != int(expected_run):
                raise ValueError("job identity")
            seen.add(ident)
            if job.get("status") != "completed" or job.get("conclusion") != "success":
                raise ValueError("job incomplete or failed")
            jobs.append(job)
    if len(jobs) != total:
        raise ValueError("incomplete job pagination")
    runners = [job["runner_name"] for job in jobs if type(job.get("runner_id")) is int and job["runner_id"] > 0 and isinstance(job.get("runner_name"), str) and re.fullmatch(r"fly-[1-9][0-9]*", job["runner_name"])]
    if not runners:
        raise ValueError("runner identity")
    print("completed success", candidate, runners[0], len(jobs))
except (ValueError, KeyError, TypeError, AttributeError):
    sys.exit(1)
' "$@"
}

check_ci() {
	local toml=tools/flyci/controller/fly.toml workflow=runner-canary.yml wait=1500
	local app label answer n_machines n_started variable branch run status conclusion head_sha runner jobs candidate run_json jobs_json failures=0
	if ! command -v gh >/dev/null 2>&1 || ! command -v flyctl >/dev/null 2>&1; then
		echo "check-live: gh and flyctl are required" >&2
		exit 2
	fi
	if ! app="$(fly_app "$toml")"; then
		echo "fail ci toml=absent"
		finish 1
	fi
	label="$(sed -n 's/^[[:space:]]*RUNNER_LABELS[[:space:]]*=[[:space:]]*"\([^",]*\).*"[[:space:]]*$/\1/p' "$repo_root/$toml" | head -n 1)"
	answer="$(timeout "$timeout" flyctl machines list --app "$app" --json 2>/dev/null | python3 -c '
import json, sys
ms = json.load(sys.stdin)
print(len(ms), len([m for m in ms if m.get("state") == "started"]))
' 2>/dev/null)" || answer=""
	read -r n_machines n_started <<<"${answer:-none none}"
	if [ "$n_started" = 1 ]; then
		echo "pass controller app=$app machines=$n_machines started=1"
	else
		echo "fail controller app=$app machines=$n_machines started=$n_started"
		failures=$((failures + 1))
	fi
	variable="$(cd "$repo_root" && timeout "$timeout" gh variable get CI_LINUX_RUNNER 2>/dev/null)" || variable=""
	if [ -n "$label" ] && [ "$variable" = "$label" ]; then
		echo "pass variable CI_LINUX_RUNNER=$variable"
	else
		echo "fail variable CI_LINUX_RUNNER=${variable:-unset} want=${label:-none}"
		failures=$((failures + 1))
	fi
	branch="$(cd "$repo_root" && timeout "$timeout" gh repo view --json defaultBranchRef --jq .defaultBranchRef.name 2>/dev/null)" || branch=""
	if [ -z "$branch" ]; then
		echo "fail canary workflow=$workflow branch=none"
		finish $((failures + 1))
	fi
	candidate="$(git -C "$repo_root" rev-parse --verify HEAD)" || candidate=""
	if [[ ! "$candidate" =~ ^[0-9a-f]{40}$ ]]; then
		echo "fail canary workflow=$workflow candidate=absent"
		finish $((failures + 1))
	fi
	if ! answer="$(cd "$repo_root" && timeout "$timeout" gh api --method POST "repos/{owner}/{repo}/actions/workflows/$workflow/dispatches" -H 'X-GitHub-Api-Version: 2022-11-28' -f ref="$branch" -F return_run_details=true 2>/dev/null)"; then
		echo "fail canary workflow=$workflow branch=$branch dispatch=unacknowledged"
		finish $((failures + 1))
	fi
	if ! run="$(printf '%s' "$answer" | ci_dispatch_id)"; then
		echo "fail canary workflow=$workflow branch=$branch run=unbound"
		finish $((failures + 1))
	fi
	(cd "$repo_root" && timeout "$wait" gh run watch "$run" --interval 15 >/dev/null 2>&1) || true
	run_json="$(cd "$repo_root" && timeout "$timeout" gh api "repos/{owner}/{repo}/actions/runs/$run" 2>/dev/null)" || run_json=""
	jobs_json="$(cd "$repo_root" && timeout "$timeout" gh api "repos/{owner}/{repo}/actions/runs/$run/jobs?per_page=100&filter=latest" --paginate --slurp 2>/dev/null)" || jobs_json=""
	answer="$(printf '%s\n%s\n' "$run_json" "$jobs_json" | ci_canary_evidence "$run" "$branch" "$candidate" "$workflow")" || answer=""
	read -r status conclusion head_sha runner jobs <<<"${answer:-none none none none none}"
	if [ "$status" != completed ]; then
		(cd "$repo_root" && timeout "$timeout" gh run cancel "$run" >/dev/null 2>&1) || true
	fi
	if [ "$status" = completed ] && [ "$conclusion" = success ] && [ "$runner" != none ]; then
		echo "pass canary workflow=$workflow branch=$branch run=$run head_sha=$head_sha conclusion=success runner=$runner"
	else
		echo "fail canary workflow=$workflow branch=$branch run=$run head_sha=$head_sha status=$status conclusion=$conclusion runners=${jobs:-none}"
		failures=$((failures + 1))
	fi
	finish "$failures"
}

# check_human_session: the wallet check-live harness tests/wallet/check-live.test.sh
# passes, then the wallet feature's human-session gate plans the golden intent
# at https://api-hull.paxeer.network from the wallet origin
# https://paxportwallet.com under the wallet identity assertion of
# CHECK_LIVE_HUMAN_ASSERTION, with CHECK_LIVE_HUMAN_ASSET and
# CHECK_LIVE_HUMAN_DESTINATION passed through; its check lines are printed as
# they come and any failure of either fails the subcommand.
check_human_session() {
	local output status=0 failures=0
	output="$("$repo_root/tests/wallet/check-live.test.sh" 2>&1)" || status=$?
	if [ "$status" -eq 0 ]; then
		echo "pass harness tests/wallet/check-live.test.sh"
	else
		echo "fail harness tests/wallet/check-live.test.sh exit=$status first=$(grep -m 1 '^FAIL ' <<<"$output" | cut -c1-160 || echo none)"
		failures=$((failures + 1))
	fi
	status=0
	CHECK_LIVE_HUMAN_BASE=https://api-hull.paxeer.network CHECK_LIVE_HUMAN_ORIGIN=https://paxportwallet.com \
		"$repo_root/scripts/wallet/check-live.sh" human-session || status=$?
	[ "$status" -eq 0 ] || failures=$((failures + 1))
	finish "$failures"
}

# check_paxeer_boundary: inside the machine of the kernel app of
# services/wallet/deploy/human.toml, the layerx-paxeer-boundary processes are
# found with their listen and node ports, the loopback socat hops to port 443
# with the RPC name each dials, and every boundary is asked for eth_chainId and
# eth_blockNumber over TLS verified against this host's internal CA for the
# name localhost, at once with the declared public RPC names, so the ten-block
# window is not eaten between the answers. Two boundaries must answer 0x7d
# within ten blocks of the highest public answer, each through its own hop,
# and the two hops must name two different serving RPC names of
# tools/bringup/search-front.sh names, neither a validator host's. One line
# per check, the boundaries numbered by listen port.
check_paxeer_boundary() {
	load_rpc_inventory || { echo "fail rpc inventory=unreadable"; finish 1; }
	# shellcheck disable=SC2016
	local script='d=$(mktemp -d)
ca="$d/ca.pem"
printf "%s\n" "$CA" >"$ca"
ask() {
	curl -sS -m "$T" --cacert "$ca" -H "content-type: application/json" -d "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"$2\",\"params\":[]}" "https://localhost:$1/" >"$d/$3.body" 2>/dev/null
	echo "$?" >"$d/$3.exit"
}
pub() {
	curl -sS -m "$T" -H "content-type: application/json" -d "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"eth_blockNumber\",\"params\":[]}" "https://api$1.$RPC_DOMAIN/" >"$d/p$1.body" 2>/dev/null
	echo "$?" >"$d/p$1.exit"
}
result() {
	sed -n "s/.*\"result\":\"\(0x[0-9a-fA-F]*\)\".*/\1/p" "$d/$1.body" 2>/dev/null | head -n 1
}
for p in /proc/[0-9]*; do
	c=$(tr "\000" " " <"$p/cmdline" 2>/dev/null) || continue
	case "$c" in
	*/layerx-paxeer-boundary*)
		e=$(tr "\000" "\n" <"$p/environ" 2>/dev/null) || continue
		l=$(printf "%s\n" "$e" | sed -n "s/^LAYERX_PAXEER_BOUNDARY_LISTEN=.*:\([0-9]*\)$/\1/p")
		u=$(printf "%s\n" "$e" | sed -n "s/^LAYERX_PAXEER_NODE_URL=http:\/\/[^/]*:\([0-9]*\).*$/\1/p")
		echo "boundary ${l:-none} ${u:-none}"
		;;
	socat\ *TCP4-LISTEN:*OPENSSL:*:443,*)
		l=$(printf "%s\n" "$c" | sed -n "s/.*TCP4-LISTEN:\([0-9]*\),.*/\1/p")
		n=$(printf "%s\n" "$c" | sed -n "s/.*OPENSSL:\([a-z0-9.-]*\):443,.*/\1/p")
		echo "hop ${l:-none} ${n:-none}"
		;;
	esac
done | sort -u >"$d/found"
cat "$d/found"
for l in $(sed -n "s/^boundary \([0-9][0-9]*\) .*/\1/p" "$d/found"); do
	ask "$l" eth_chainId "c$l" &
	ask "$l" eth_blockNumber "h$l" &
done
n=1
while [ "$n" -le "$RPC_COUNT" ]; do
	pub "$n" &
	n=$((n + 1))
done
wait
for l in $(sed -n "s/^boundary \([0-9][0-9]*\) .*/\1/p" "$d/found"); do
	echo "chain $l $(result "c$l") $(cat "$d/c$l.exit")"
	echo "head $l $(result "h$l")"
done
n=1
while [ "$n" -le "$RPC_COUNT" ]; do
	echo "public $n $(result "p$n")"
	n=$((n + 1))
done
rm -rf "$d"'
	local toml=services/wallet/deploy/human.toml
	local app reply listing serving validators top=0 n value k port node hop chain code head lag tls ok
	local -a ports=() nodes=() hops=()
	local failures=0
	if ! command -v flyctl >/dev/null 2>&1; then
		echo "check-live: flyctl is required" >&2
		exit 2
	fi
	if ! app="$(fly_app "$toml")"; then
		echo "fail paxeer-boundary toml=absent"
		finish 1
	fi
	if [ ! -r "$ca_dir/ca.pem" ]; then
		echo "fail paxeer-boundary app=$app ca=absent"
		finish 1
	fi
	reply="$(fly_ssh "$app" - "sh -s" < <(printf "T=%s\nRPC_COUNT=%s\nRPC_DOMAIN=%s\nCA='%s'\n%s\n" "$(((timeout + 1) / 2))" "$rpc_name_count" "$rpc_domain" "$(cat "$ca_dir/ca.pem")" "$script"))" || reply=""
	if [ -z "$reply" ]; then
		echo "fail paxeer-boundary app=$app machine=unreadable"
		finish 1
	fi
	while read -r _ n value; do
		value="$(hex_to_dec "${value:-}")"
		if [ -n "$value" ] && [ "$value" -gt "$top" ]; then
			top="$value"
		fi
	done < <(grep '^public ' <<<"$reply")
	mapfile -t ports < <(sed -n 's/^boundary \([0-9][0-9]*\) .*/\1/p' <<<"$reply" | sort -n)
	if [ "${#ports[@]}" -eq 2 ]; then
		echo "pass boundaries app=$app count=2"
	else
		echo "fail boundaries app=$app count=${#ports[@]}"
		failures=$((failures + 1))
	fi
	for k in "${!ports[@]}"; do
		port="${ports[$k]}"
		node="$(sed -n "s/^boundary $port \([0-9a-z]*\)$/\1/p" <<<"$reply" | head -n 1)"
		hop="$(sed -n "s/^hop $node \([a-z0-9.-]*\)$/\1/p" <<<"$reply" | head -n 1)"
		read -r _ _ chain code <<<"$(grep "^chain $port " <<<"$reply" | head -n 1)" || true
		read -r _ _ head <<<"$(grep "^head $port " <<<"$reply" | head -n 1)" || true
		if [ "${code:-none}" = 0 ]; then
			tls=verified
		else
			tls="curl-${code:-none}"
		fi
		head="$(hex_to_dec "${head:-}")"
		lag=none
		ok=1
		if [ -n "$head" ] && [ "$top" -gt 0 ]; then
			lag=$((top - head))
			[ "$lag" -le "$rpc_max_lag" ] || ok=0
		else
			ok=0
		fi
		[ "$tls" = verified ] && [ "${chain:-}" = 0x7d ] && [ -n "$hop" ] || ok=0
		nodes[k]="$node"
		hops[k]="${hop:-none}"
		value="boundary-$((k + 1)) port=$port tls=$tls chain_id=${chain:-none} node_port=$node hop=${hop:-none} head=${head:-none} top=$top lag=$lag"
		if [ "$ok" -eq 1 ]; then
			echo "pass $value"
		else
			echo "fail $value"
			failures=$((failures + 1))
		fi
	done
	if ! listing="$("$(dirname "${BASH_SOURCE[0]}")/search-front.sh" names 2>&1)"; then
		echo "fail hops names=unreadable $(printf '%s' "$listing" | tr '\n' ' ' | cut -c1-160)"
		finish $((failures + 1))
	fi
	serving=0
	validators=0
	for k in "${!hops[@]}"; do
		grep -qxF "serve ${hops[$k]}" <<<"$listing" && serving=$((serving + 1))
		grep -qxF "validator ${hops[$k]}" <<<"$listing" && validators=$((validators + 1))
	done
	value="hops first=${hops[0]:-none} second=${hops[1]:-none} serving=$serving/2 validator=$validators"
	if [ "${#hops[@]}" -eq 2 ] && [ "${hops[0]}" != "${hops[1]}" ] && [ "${nodes[0]}" != "${nodes[1]}" ] && [ "$serving" -eq 2 ] && [ "$validators" -eq 0 ]; then
		echo "pass $value distinct=yes"
	else
		echo "fail $value distinct=$([ "${#hops[@]}" -eq 2 ] && [ "${hops[0]}" != "${hops[1]}" ] && [ "${nodes[0]}" != "${nodes[1]}" ] && echo yes || echo no)"
		failures=$((failures + 1))
	fi
	finish "$failures"
}

# check_agent_public: the agentd mTLS surface of the kernel app of
# services/wallet/deploy/human.toml at https://machine.paxeer.network:9454, the
# name the edge host passes through unchanged to the app's dedicated IPv4.
# The app holds a dedicated IPv4; inside a machine of the app, the running
# layerx-agentd's program bearer and probe program are read from its own
# environment, and services/platform/hosted/agentd/probe.sh runs against the public
# name with the agentd-client identity tools/bringup/ca.sh left on the
# volume and the internal CA root beside it; then one program.discover read
# envelope posted to /rpc must answer 200 with request_id, value and
# verification_status. The identity and the bearer never leave the machine.
# One line per check.
# shellcheck disable=SC2016
agent_public_head='umask 077
w=$(mktemp -d) || exit 1
trap "rm -rf $w" EXIT
cat >"$w/probe.sh" <<"LXPROBE"'
# shellcheck disable=SC2016
agent_public_tail='pid=
for d in /proc/[0-9]*; do
	case "$(readlink "$d/exe" 2>/dev/null)" in
	*/layerx-agentd)
		pid=$d
		break
		;;
	esac
done
if [ -z "$pid" ]; then
	echo "@@agentd none"
	exit 0
fi
echo "@@agentd found"
env_of() { tr "\000" "\n" <"$pid/environ" | sed -n "s/^$1=//p" | head -n 1; }
env_of LAYERX_AGENT_PROGRAM_BEARER_TOKEN >"$w/bearer"
program=$(env_of LAYERX_AGENT_PROGRAM_PROBE_ID)
status=0
sh "$w/probe.sh" --url "$url" --ca "$tls/ca.pem" --client-cert "$tls/cert.pem" --client-key "$tls/key.pem" --bearer-file "$w/bearer" >"$w/probe.out" 2>&1 </dev/null || status=$?
echo "@@probe $status"
head -n 3 "$w/probe.out"
printf "header = \"Authorization: Bearer %s\"\n" "$(cat "$w/bearer")" >"$w/bearer.conf"
printf "{\"operation\":\"program.discover\",\"request\":{\"program_id\":\"%s\",\"requested_verification_level\":\"sequencer-signed\"}}" "$program" >"$w/rpc.json"
status=0
code=$(curl --silent --show-error --max-time "$limit" --output "$w/rpc.out" --write-out "%{http_code}" --cacert "$tls/ca.pem" --cert "$tls/cert.pem" --key "$tls/key.pem" --config "$w/bearer.conf" -H "Content-Type: application/json" --data-binary "@$w/rpc.json" "$url/rpc" 2>"$w/rpc.err" </dev/null) || status=$?
echo "@@rpc $status ${code:-none}"
if [ "$status" -eq 0 ]; then head -c 65536 "$w/rpc.out"; else head -n 1 "$w/rpc.err"; fi
echo'
agent_public_py='
import json
import sys

try:
    doc = json.loads(sys.stdin.read())
except ValueError:
    doc = None
if not isinstance(doc, dict):
    print("body=unreadable")
    sys.exit(1)
request_id = doc.get("request_id")
status = doc.get("verification_status")
state = status.get("state") if isinstance(status, dict) else None
ok = isinstance(request_id, str) and request_id != "" and doc.get("value") is not None and isinstance(state, str) and state != ""
print("request_id=%s value=%s verification_status=%s" % (
    "present" if isinstance(request_id, str) and request_id else "absent",
    "present" if doc.get("value") is not None else "absent",
    state if isinstance(state, str) and state else "absent"))
sys.exit(0 if ok else 1)
'
check_agent_public() {
	local toml=services/wallet/deploy/human.toml url=https://machine.paxeer.network:9454
	local app answer reply agentd probe_status probe_out rpc_line rpc_status rpc_code rpc_body shape status failures=0
	if ! command -v flyctl >/dev/null 2>&1; then
		echo "check-live: flyctl is required" >&2
		exit 2
	fi
	if ! app="$(fly_app "$toml")"; then
		echo "fail agent-public toml=absent"
		finish 1
	fi
	answer="$(timeout "$timeout" flyctl ips list --app "$app" --json 2>/dev/null | python3 -c 'import json, sys; print(len([i for i in json.load(sys.stdin) or [] if i.get("Type") == "v4"]))' 2>/dev/null)" || answer=""
	if [ -n "$answer" ] && [ "$answer" -ge 1 ]; then
		echo "pass ipv4 app=$app dedicated=$answer"
	else
		echo "fail ipv4 app=$app dedicated=${answer:-unreadable}"
		failures=$((failures + 1))
	fi
	reply="$({
		printf '%s\n' "$agent_public_head"
		cat "$repo_root/services/platform/hosted/agentd/probe.sh"
		printf '%s\n' LXPROBE "$agent_public_tail"
	} | fly_ssh "$app" - "tls=$tls_dir/agentd-client url=$url limit=$timeout sh -s")" || reply=""
	agentd="$(sed -n 's/^@@agentd //p' <<<"$reply" | head -n 1)"
	if [ "$agentd" != found ]; then
		echo "fail agentd app=$app process=${agentd:-unreachable}"
		finish $((failures + 1))
	fi
	echo "pass agentd app=$app process=found"
	probe_status="$(sed -n 's/^@@probe //p' <<<"$reply" | head -n 1)"
	probe_out="$(sed -n '/^@@probe /,/^@@rpc /{/^@@/d;p}' <<<"$reply" | tr '\n' ' ' | cut -c1-200)"
	if [ "$probe_status" = 0 ]; then
		echo "pass probe $url ready=true bearer=enforced client-cert=enforced"
	else
		echo "fail probe $url exit=${probe_status:-none} ${probe_out% }"
		failures=$((failures + 1))
	fi
	rpc_line="$(sed -n 's/^@@rpc //p' <<<"$reply" | head -n 1)"
	read -r rpc_status rpc_code <<<"${rpc_line:-none none}"
	rpc_body="$(sed -n '/^@@rpc /,$p' <<<"$reply" | sed '1d')"
	if [ "$rpc_status" != 0 ]; then
		echo "fail rpc $url/rpc operation=program.discover transport=curl-$rpc_status $(printf '%s' "$rpc_body" | tr '\n' ' ' | cut -c1-160)"
		finish $((failures + 1))
	fi
	shape="$(python3 -c "$agent_public_py" <<<"$rpc_body")" && status=0 || status=$?
	if [ "$status" -eq 0 ] && [ "$rpc_code" = 200 ]; then
		echo "pass rpc $url/rpc operation=program.discover http=200 $shape"
	else
		echo "fail rpc $url/rpc operation=program.discover http=$rpc_code $shape"
		failures=$((failures + 1))
	fi
	finish "$failures"
}

# check_gas: the gas station app of services/interop/deploy/gas-station/fly.toml runs
# one started machine with one volume and the configuration its init rendered
# there names chain 125, the SID token, the primary node first among three
# endpoints and the paymaster; the account of CHECK_LIVE_GAS_ACCOUNT_KEYSTORE
# is delegated to that paymaster; POST /quote at chain.paxeer.network answers
# 200 with a quote for one no-op call whose SID amount lies within the
# contract's spread of the paymaster's currentRate; the paymaster's
# rateUpdatedAt is no older than the station's max_rate_age at the latest
# block, which the machine's rate publisher keeps true; an info line reports
# the publisher's worst-case daily wei (rate_max_fee_per_gas times
# rate_gas_budget_per_day of its rate.json) and the wei its journal rate.jsonl
# records as spent on the latest block's chain day, none for what the machine
# does not answer; the account signs the
# batch and its EIP-7702 authorization with cast, POST /submit answers 200
# with a transaction hash, and its receipt succeeds with the SID Transfer of
# the quoted amount from the account to the sponsor. The keystore password
# never leaves cast. One line per check.
gas_sid=0x21f7b20a555199fa73A238B1a91FD0f549068fEe
gas_transfer_topic=0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef
gas_eth_prefix=0x19457468657265756d205369676e6564204d6573736167653a0a3332

# gas_rpc <method> <params json>: the result of one JSON-RPC call to the gas
# check's RPC URL, a string as is and anything else as compact JSON; status 1
# when there is no result.
gas_rpc() {
	bringup_inspect rpc "$gas_rpc_url" "$1" "$2"
}

# gas_post <route> <request file> <response file>: POSTs the file to the
# station's route and prints the HTTP status.
gas_post() {
	curl -sS --max-time "$timeout" --output "$3" --write-out '%{http_code}' \
		-H 'content-type: application/json' --data-binary "@$2" "$gas_origin$1" 2>/dev/null || true
}

# gas_signed <32-byte hash>: the EIP-191 hash of the 32 bytes, as
# MessageHashUtils.toEthSignedMessageHash computes it.
gas_signed() {
	cast keccak "$gas_eth_prefix${1#0x}"
}

gas_sign() {
	cast wallet sign --no-hash "$1" --keystore "$CHECK_LIVE_GAS_ACCOUNT_KEYSTORE" \
		--password-file "$CHECK_LIVE_GAS_ACCOUNT_PASSWORD_FILE" 2>/dev/null
}

check_gas() {
	load_rpc_inventory || { echo "fail rpc inventory=unreadable"; finish 1; }
	local name
	local app answer config chain token endpoints first paymaster gas_limit priority
	local account code rate price nonce auth_nonce gas_cost w status body quote sponsor amount maximum deadline qnonce
	local qd calls_hash bd account_sig auth_sig auth_rlp call_data tx receipt attempt verdict failures=0
	local max_age updated head age updated_at rate_config journal worst spent
	for name in CHECK_LIVE_GAS_ACCOUNT_KEYSTORE CHECK_LIVE_GAS_ACCOUNT_PASSWORD_FILE CHECK_LIVE_GAS_MAX_TOKEN_AMOUNT; do
		if [ -z "${!name:-}" ]; then
			echo "check-live: $name is unset" >&2
			exit 2
		fi
	done
	if ! [[ "$CHECK_LIVE_GAS_MAX_TOKEN_AMOUNT" =~ ^[1-9][0-9]*$ ]]; then
		echo "check-live: CHECK_LIVE_GAS_MAX_TOKEN_AMOUNT is not a positive integer" >&2
		exit 2
	fi
	gas_origin="${CHECK_LIVE_GAS_ORIGIN:-https://chain.paxeer.network}"
	gas_rpc_url="${CHECK_LIVE_GAS_RPC:-${node_endpoints[0]}}"
	if ! printf '%s\n' "${node_endpoints[@]}" | grep -qxF -- "$gas_rpc_url"; then
		echo "fail gas rpc=undeclared-node-endpoint"
		finish 1
	fi
	if ! app="$(railway_service gas-station)"; then
		echo "fail gas rollout=absent"
		finish 1
	fi
	railway_deployment "$app" /data || failures=$((failures + 1))

	local station
	station="$(railway_ssh "$app" "cat /data/gas-station/station.json" </dev/null)" || station=""
	if ! bringup_inspect gas-config "$rpc_inventory" <<<"$station" >/dev/null; then
		echo "fail config app=$app endpoints=undeclared-or-invalid"
		finish $((failures + 1))
	fi
	config="$(python3 -c '
import json, sys
c = json.load(sys.stdin)
print(c["chain_id"], c["token"], len(c["endpoints"]), c["endpoints"][0], c["paymaster"], c["gas_limit"], c["max_priority_fee_per_gas"], c["max_rate_age"])
' <<<"$station" 2>/dev/null)" || config=""
	read -r chain token endpoints first paymaster gas_limit priority max_age <<<"${config:-none none none none none none none none}"
	if [ -z "$config" ]; then
		echo "fail config app=$app station.json=unreadable"
		finish $((failures + 1))
	fi
	if [ "$chain" = 125 ] && [ "${token,,}" = "${gas_sid,,}" ] && [ "$endpoints" -gt 0 ] && [ "$first" = "${node_endpoints[0]}" ]; then
		echo "pass config app=$app chain_id=125 token=SID endpoints=$endpoints first=node paymaster=$paymaster"
	else
		echo "fail config app=$app chain_id=$chain token=$token endpoints=$endpoints first=$first"
		finish $((failures + 1))
	fi

	updated="$(gas_rpc eth_call "[{\"to\":\"$paymaster\",\"data\":\"$(cast sig 'rateUpdatedAt()')\"},\"latest\"]")" || updated=""
	head="$(gas_rpc eth_getBlockByNumber '["latest",false]')" || head=""
	read -r age updated_at <<<"$(python3 -c '
import json, sys
updated, head = int(sys.argv[1], 16), json.loads(sys.argv[2])
print(int(head["timestamp"], 16) - updated, updated)
' "$updated" "$head" 2>/dev/null || echo none none)"
	if [[ "$age" =~ ^[0-9]+$ && "$max_age" =~ ^[0-9]+$ ]] && [ "$updated_at" -gt 0 ] && [ "$age" -le "$max_age" ]; then
		echo "pass rate-age paymaster=$paymaster updated_at=$updated_at age=$age max_rate_age=$max_age"
	else
		echo "fail rate-age paymaster=$paymaster updated_at=${updated_at:-none} age=${age:-none} max_rate_age=$max_age"
		failures=$((failures + 1))
	fi
	rate_config="$(railway_ssh "$app" "cat /data/gas-station/rate.json" </dev/null)" || rate_config=""
	worst="$(python3 -c '
import json, sys
c = json.loads(sys.argv[1])
print(int(c["rate_max_fee_per_gas"]) * int(c["rate_gas_budget_per_day"]))
' "$rate_config" 2>/dev/null)" || worst=none
	spent=none
	if journal="$(railway_ssh "$app" "cat /data/gas-station/rate.jsonl" </dev/null)"; then
		spent="$(python3 -c '
import json, sys
day = int(json.loads(sys.argv[1])["timestamp"], 16) // 86400
signed, spent = {}, 0
for line in sys.stdin:
    if not line.strip():
        continue
    e = json.loads(line)
    if e["kind"] == "rate_published":
        signed[tuple(e["publication"]["hash"])] = e["publication"]["signed_at"] // 86400
    elif e["kind"] == "rate_settled" and signed.get(tuple(e["hash"])) == day:
        spent += int(e["settlement"]["cost_wei"])
print(spent)
' "$head" <<<"$journal" 2>/dev/null)" || spent=none
	fi
	echo "info rate-spend app=$app daily_wei_max=$worst spent_wei=$spent"

	account="$(cast wallet address --keystore "$CHECK_LIVE_GAS_ACCOUNT_KEYSTORE" --password-file "$CHECK_LIVE_GAS_ACCOUNT_PASSWORD_FILE" 2>/dev/null)" || account=""
	if [ -z "$account" ]; then
		echo "fail account keystore=unreadable"
		finish $((failures + 1))
	fi
	code="$(gas_rpc eth_getCode "[\"$account\",\"latest\"]")" || code=""
	if [ "${code,,}" = "0xef0100$(tr '[:upper:]' '[:lower:]' <<<"${paymaster#0x}")" ]; then
		echo "pass delegation account=$account delegate=$paymaster"
	else
		echo "fail delegation account=$account code=${code:-none} want=0xef0100${paymaster#0x}"
		finish $((failures + 1))
	fi

	rate="$(gas_rpc eth_call "[{\"to\":\"$paymaster\",\"data\":\"$(cast sig 'currentRate()')\"},\"latest\"]")" || rate=""
	price="$(gas_rpc eth_gasPrice '[]')" || price=""
	nonce="$(gas_rpc eth_call "[{\"to\":\"$account\",\"data\":\"$(cast sig 'nonce()')\"},\"pending\"]")" || nonce=""
	auth_nonce="$(gas_rpc eth_getTransactionCount "[\"$account\",\"pending\"]")" || auth_nonce=""
	if ! [[ "$rate" =~ ^0x[0-9a-fA-F]+$ && "$price" =~ ^0x[0-9a-fA-F]+$ && "$nonce" =~ ^0x[0-9a-fA-F]+$ && "$auth_nonce" =~ ^0x[0-9a-fA-F]+$ ]] || [ "$((rate))" -eq 0 ]; then
		echo "fail chain-state rate=${rate:-none} gas_price=${price:-none} batch_nonce=${nonce:-none} account_nonce=${auth_nonce:-none}"
		finish $((failures + 1))
	fi
	read -r rate nonce auth_nonce gas_cost <<<"$(python3 -c '
import sys
rate, price, nonce, auth, limit, priority = (int(v, 0) for v in sys.argv[1:])
fee = 2 * price + priority
print(rate, nonce, auth, limit * fee)
' "$rate" "$price" "$nonce" "$auth_nonce" "$gas_limit" "$priority")"

	w="$(mktemp -d)"
	printf '{"account":"%s","nonce":"%s","calls":[{"to":"%s","value":"0","data":"0x"}],"maxTokenAmount":"%s","gasCost":"%s","chainId":"125","token":"%s","decimals":6}' \
		"$account" "$nonce" "$account" "$CHECK_LIVE_GAS_MAX_TOKEN_AMOUNT" "$gas_cost" "$gas_sid" >"$w/quote.json"
	status="$(gas_post /quote "$w/quote.json" "$w/quote.out")"
	quote="$(python3 -c '
import json, sys
account, sid, maximum, gas_cost, rate = sys.argv[1].lower(), sys.argv[2].lower(), int(sys.argv[3]), int(sys.argv[4]), int(sys.argv[5])
try:
    doc = json.load(open(sys.argv[6]))
    q = doc["quote"]
    fields = [q["sponsor"], q["token"], int(q["maxTokenAmount"]), int(q["tokenAmount"]), int(q["deadline"]), int(q["quoteNonce"]), int(q["gasCost"]), q["decimals"], doc["relayerSignature"]]
except Exception:
    print("|body=unreadable")
    sys.exit(1)
sponsor, token, qmax, amount, deadline, qnonce, qcost, decimals, signature = fields
expected = -(-gas_cost * rate // 10**18)
lower, upper = -(-expected * 9500 // 10000), expected * 10500 // 10000
ok = (token.lower() == sid and decimals == 6 and qcost == gas_cost and qmax <= maximum and 0 < amount <= qmax
      and lower <= amount <= upper and sponsor.lower() not in ("", account) and len(signature) == 132)
print(sponsor, qmax, amount, deadline, qnonce, signature, "|rate=%d token_amount=%d expected=%d max=%d sponsor=%s" % (rate, amount, expected, qmax, sponsor))
sys.exit(0 if ok else 1)
' "$account" "$gas_sid" "$CHECK_LIVE_GAS_MAX_TOKEN_AMOUNT" "$gas_cost" "$rate" "$w/quote.out" 2>/dev/null)" && verdict=pass || verdict=fail
	[ "$status" = 200 ] || verdict=fail
	if [ "$verdict" != pass ]; then
		echo "fail quote $gas_origin/quote http=${status:-none} ${quote#*|}"
		rm -rf "$w"
		finish $((failures + 1))
	fi
	read -r sponsor maximum amount deadline qnonce body <<<"${quote%%|*}"
	echo "pass quote $gas_origin/quote http=200 ${quote#*|}"

	qd="$(gas_signed "$(cast keccak "$(cast abi-encode 'f(bytes32,uint256,address,address,address,uint256,uint256,uint256,uint256,uint256)' \
		"$(cast keccak 'Quote(uint256 chainId,address account,address sponsor,address token,uint256 maxTokenAmount,uint256 tokenAmount,uint256 deadline,uint256 quoteNonce,uint256 gasCost)')" \
		125 "$account" "$sponsor" "$gas_sid" "$maximum" "$amount" "$deadline" "$qnonce" "$gas_cost")")")"
	calls_hash="$(cast keccak "$(cast abi-encode 'f((address,uint256,bytes)[])' "[($account,0,0x)]")")"
	bd="$(gas_signed "$(cast keccak "$(cast abi-encode 'f(bytes32,uint256,bytes32,bytes32)' \
		"$(cast keccak 'SponsoredBatch(uint256 nonce,bytes32 callsHash,bytes32 quoteDigest)')" "$nonce" "$calls_hash" "$qd")")")"
	auth_rlp="$(cast to-rlp "[\"0x7d\",\"$paymaster\",\"$(python3 -c 'import sys; n = int(sys.argv[1]); print("0x" + (("%x" % n).rjust(len("%x" % n) + len("%x" % n) % 2, "0") if n else ""))' "$auth_nonce")\"]")"
	account_sig="$(gas_sign "$bd")" || account_sig=""
	auth_sig="$(gas_sign "$(cast keccak "0x05${auth_rlp#0x}")")" || auth_sig=""
	if [ "${#account_sig}" -ne 132 ] || [ "${#auth_sig}" -ne 132 ]; then
		echo "fail sign account=$account keystore=refused"
		rm -rf "$w"
		finish $((failures + 1))
	fi
	call_data="$(cast calldata 'executeSponsored((address,uint256,bytes)[],(address,address,uint256,uint256,uint256,uint256,uint256),bytes,bytes)' \
		"[($account,0,0x)]" "($sponsor,$gas_sid,$maximum,$amount,$deadline,$qnonce,$gas_cost)" "$account_sig" "$body")"
	printf '{"call":{"to":"%s","value":"0","data":"%s"},"authorization":{"chainId":"125","address":"%s","nonce":"%s","yParity":%d,"r":"0x%s","s":"0x%s"},"batch":{"chainId":"125","account":"%s","nonce":"%s","calls":[{"to":"%s","value":"0","data":"0x"}],"quote":{"sponsor":"%s","token":"%s","maxTokenAmount":"%s","tokenAmount":"%s","deadline":"%s","quoteNonce":"%s","gasCost":"%s","decimals":6}},"accountSignature":"%s","relayerSignature":"%s"}' \
		"$account" "$call_data" "$paymaster" "$auth_nonce" "$((16#${auth_sig:130:2} - 27))" "${auth_sig:2:64}" "${auth_sig:66:64}" \
		"$account" "$nonce" "$account" "$sponsor" "$gas_sid" "$maximum" "$amount" "$deadline" "$qnonce" "$gas_cost" "$account_sig" "$body" >"$w/submit.json"
	status="$(gas_post /submit "$w/submit.json" "$w/submit.out")"
	tx="$(python3 -c 'import json, sys; print(json.load(open(sys.argv[1]))["transactionHash"])' "$w/submit.out" 2>/dev/null)" || tx=""
	rm -rf "$w"
	if [ "$status" != 200 ] || ! [[ "$tx" =~ ^0x[0-9a-fA-F]{64}$ ]]; then
		echo "fail submit $gas_origin/submit http=${status:-none} tx=${tx:-none}"
		finish $((failures + 1))
	fi
	receipt=""
	for attempt in $(seq "${CHECK_LIVE_GAS_RECEIPT_ATTEMPTS:-36}"); do
		receipt="$(gas_rpc eth_getTransactionReceipt "[\"$tx\"]")" || receipt=""
		[ -z "$receipt" ] || [ "$receipt" = null ] || break
		[ "$attempt" = "${CHECK_LIVE_GAS_RECEIPT_ATTEMPTS:-36}" ] || sleep 5
	done
	answer="$(python3 -c '
import json, sys
sid, topic, account, sponsor, amount = sys.argv[1].lower(), sys.argv[2], sys.argv[3].lower(), sys.argv[4].lower(), int(sys.argv[5])
try:
    r = json.loads(sys.argv[6])
    status = int(r["status"], 16)
    logs = r["logs"]
except Exception:
    print("receipt=none")
    sys.exit(1)
pad = lambda a: "0x" + a[2:].rjust(64, "0")
moved = [l for l in logs if l.get("address", "").lower() == sid and [t.lower() for t in l.get("topics", [])] == [topic, pad(account), pad(sponsor)]]
paid = moved and int(moved[0].get("data", "0x0"), 16) or 0
print("status=%d sid_transfer=%d want=%d to=sponsor" % (status, paid, amount))
sys.exit(0 if status == 1 and paid == amount else 1)
' "$gas_sid" "$gas_transfer_topic" "$account" "$sponsor" "$amount" "${receipt:-null}")" && verdict=pass || verdict=fail
	echo "$verdict submit $gas_origin/submit http=200 tx=$tx $answer"
	[ "$verdict" = pass ] || failures=$((failures + 1))
	finish "$failures"
}

# check_internal resolves the Railway roles and their private domains, verifies
# the payments/programs node-root pins, then probes every role from KERNEL_HOST
# with the Human client retained there by ca.sh. Explicit private connect
# targets preserve each deployed service's TLS name.
# shellcheck disable=SC2016
internal_pin_script='pid=
for d in /proc/[0-9]*; do
	case "$(readlink "$d/exe" 2>/dev/null)" in
	*/layerx-event-source)
		if tr "\000" "\n" <"$d/environ" 2>/dev/null | grep -qx "LAYERX_EVENTS_KIND=$kind"; then
			pid=$d
			break
		fi
		;;
	esac
done
if [ -z "$pid" ]; then
	echo "@@process none"
	exit 0
fi
env_of() { tr "\000" "\n" <"$pid/environ" | sed -n "s/^$1=//p" | head -n 1; }
echo "@@url $(env_of LAYERX_EVENTS_UPSTREAM_URL)"
echo "@@pin $(base64 <"$(env_of LAYERX_EVENTS_UPSTREAM_CA_DER)" 2>/dev/null | tr -d "\n")"'
# The kernel box must have the private route and DNS of the deployed service.
# An explicit private connect target can override routing while preserving the
# service URL's TLS identity. Never move the Human key to the target role.
internal_probe() {
	local app="$1" group="$2" command reply status=0 variable connect
	variable="CHECK_LIVE_INTERNAL_CONNECT_${group^^}"
	connect="${!variable:-}"
	printf -v command '%q internal-readiness %q %q %q' "${BRINGUP_INSPECT_BIN:-bringup-inspect}" "https://$app.railway.internal:9443/readyz" "$tls_dir/human-event-client" "$connect"
	reply="$(ssh_read "$KERNEL_HOST" "$command")" || status=$?
	if [ "$status" -eq 0 ] && [ "$reply" = 'pass readiness http=200 ready=true identity=human-event-client' ]; then
		echo "@@$group 0 200 {\"ready\":true}"
	else
		echo "@@$group $((status == 0 ? 1 : status)) 000 refused"
	fi
}

check_internal() {
	local node=api-mainnet-beta.paxeer.network
	local connect="${CHECK_LIVE_ROUTER_CONNECT:-api-mainnet-beta.paxeer.network:443}"
	local app kernel name="" answer w root="" group reply url pin subject line status code body failures=0
	if ! command -v "$railway_bin" >/dev/null 2>&1; then
		echo "check-live: $railway_bin is required" >&2
		exit 2
	fi
	for group in kms payments programs journeys approvals redis; do
		name="internal-$group"
		[ "$group" != redis ] || name=redis-internal
		if ! app="$(railway_service "$name")"; then
			echo "fail internal service=$name rollout=absent"
			finish 1
		fi
		answer="$(railway_domains "$app")" || answer=none
		if [ "$answer" = 0 ]; then
			echo "pass public-domains service=$app count=0"
		else
			echo "fail public-domains service=$app count=${answer:-none}"
			failures=$((failures + 1))
		fi
	done

	name=""
	w="$(mktemp -d)"
	# shellcheck disable=SC2064
	trap "rm -rf '$w'" EXIT
	timeout "$timeout" openssl s_client -connect "$connect" -servername "$node" -showcerts </dev/null 2>/dev/null |
		awk -v dir="$w" '/-BEGIN CERTIFICATE-/ { n++; in_cert = 1 } in_cert { print >(dir "/served-" n ".pem") } /-END CERTIFICATE-/ { in_cert = 0; close(dir "/served-" n ".pem") }' || true
	if [ -s "$w/served-1.pem" ]; then
		cat "$w"/served-[2-9].pem >"$w/untrusted.pem" 2>/dev/null || : >"$w/untrusted.pem"
		# shellcheck disable=SC2012
		name="$(openssl x509 -in "$(ls "$w"/served-*.pem | sort -V | tail -n 1)" -noout -issuer -nameopt sep_multiline,utf8 | sed -n 's/^ *CN=//p')"
		case "$name" in
		"ISRG Root X1") root=X1 ;;
		"ISRG Root X2") root=X2 ;;
		esac
	fi
	if [ -n "$root" ]; then
		echo "pass node-root host=$node root=$root"
	else
		[ -n "$name" ] && name=other || name=unreadable
		echo "fail node-root host=$node root=$name"
		failures=$((failures + 1))
	fi

	for group in payments programs; do
		app="$(railway_service "internal-$group")"
		reply="$(printf '%s\n' "$internal_pin_script" | railway_ssh "$app" "kind=$group sh -s")" || reply=""
		url="$(sed -n 's/^@@url //p' <<<"$reply" | head -n 1)"
		pin="$(sed -n 's/^@@pin //p' <<<"$reply" | head -n 1)"
		if [ -z "$url" ]; then
			echo "fail upstream-ca group=$group process=$(sed -n 's/^@@process //p' <<<"$reply" | head -n 1 | grep . || echo unreachable)"
			failures=$((failures + 1))
			continue
		fi
		subject=""
		status=1
		if [ -n "$pin" ] && base64 -d <<<"$pin" 2>/dev/null | openssl x509 -inform DER -out "$w/pin-$group.pem" 2>/dev/null; then
			subject="$(openssl x509 -in "$w/pin-$group.pem" -noout -subject -nameopt sep_multiline,utf8 | sed -n 's/^ *CN=//p')"
			if [ -n "$root" ] && [ "$subject" = "ISRG Root $root" ] &&
				[ "$subject" = "$(openssl x509 -in "$w/pin-$group.pem" -noout -issuer -nameopt sep_multiline,utf8 | sed -n 's/^ *CN=//p')" ] &&
				openssl verify -CAfile "$w/pin-$group.pem" -untrusted "$w/untrusted.pem" -verify_hostname "$node" "$w/served-1.pem" >/dev/null 2>&1; then
				status=0
			fi
		fi
		if [ "$url" = "https://$node" ] && [ "$status" -eq 0 ]; then
			echo "pass upstream-ca group=$group url=$url root=$root verifies=yes"
		else
			subject="${subject:-unreadable}"
			echo "fail upstream-ca group=$group url=$url pin=${subject// /-} served=${root:-unreadable}"
			failures=$((failures + 1))
		fi
	done

	for group in kms journeys payments approvals programs; do
		app="$(railway_service "internal-$group")"
		kernel=KERNEL_HOST
		url="https://$app.railway.internal:9443/readyz"
		reply="$(internal_probe "$app" "$group")" || reply=""
		line="$(sed -n "s/^@@$group //p" <<<"$reply" | head -n 1)"
		read -r status code body <<<"${line:-none none}"
		if [ "$status" = 0 ] && [ "$code" = 200 ] && [[ "$body" == *'"ready":true'* ]]; then
			echo "pass readiness group=$group url=$url from=$kernel http=200 ready=true"
		else
			if [ "$status" = 0 ] && [ "$code" = 503 ] && [[ "$body" == *'"state":"waiting-principals"'* ]]; then
				echo "fail readiness group=$group url=$url from=$kernel http=503 state=waiting-principals"
			else
			echo "fail readiness group=$group url=$url from=$kernel curl=${status:-none} http=${code:-none}"
			fi
			failures=$((failures + 1))
		fi
	done
	finish "$failures"
}

# bridge_machine_script: runs inside the bridge relayer machine under sh -s
# with journal set to the relayer's journal path. Prints "@@relayer" and
# "@@signer" with running or none, from the executables of /proc, then
# "@@journal absent" or "@@journal present" and, for the first inbound item
# the journal records as observed and as completed by this relayer's included
# bridgeIn transaction, "@@bridge-in <item>".
# shellcheck disable=SC2016
bridge_machine_script='relayer=none
signer=none
for d in /proc/[0-9]*; do
	case "$(readlink "$d/exe" 2>/dev/null)" in
	*/layerx-bridge-relayer) relayer=running ;;
	*/layerx-mirror-signer) signer=running ;;
	esac
done
echo "@@relayer $relayer"
echo "@@signer $signer"
if [ ! -r "$journal" ]; then
	echo "@@journal absent"
	exit 0
fi
echo "@@journal present"
sed -n "s/^{\"kind\":\"completed\",\"item\":\"\(in:[^\"]*\)\",\"completion\":{\"outcome\":\"included\".*/\1/p" "$journal" | while read -r item; do
	if grep -qF "{\"kind\":\"observed\",\"item\":\"$item\"," "$journal"; then
		echo "@@bridge-in $item"
		break
	fi
done'

# check_bridge: services/bridge/deploy/checklist.sh exits 0 for every chain under
# contracts/bridge/evm/chains and contracts/bridge/solana/chains against the deployment record
# PAXEER_BRIDGE_RECORDS_DIR/<chain>.json, each run bounded by ten times
# CHECK_LIVE_TIMEOUT; a machine of the app of each of
# services/interop/deploy/bridge-relayer/relayer-1.toml through relayer-5.toml runs both
# the relayer and its bridge signer; and the journal on the volume of one of
# them records one observed deposit completed by that relayer's own included
# bridgeIn transaction. One line per check; a
# failing checklist line carries its last output line with every URL replaced
# by <url>.
check_bridge() {
	local journal=/data/relayer/journal.jsonl
	local n toml app dir chain record out status reply relayer signer item="" item_app="" journals=0 failures=0
	local -a chains=()
	if [ -z "${PAXEER_BRIDGE_RECORDS_DIR:-}" ]; then
		echo "check-live: PAXEER_BRIDGE_RECORDS_DIR is unset" >&2
		exit 2
	fi
	if ! command -v flyctl >/dev/null 2>&1; then
		echo "check-live: flyctl is required" >&2
		exit 2
	fi
	for dir in "$repo_root"/contracts/bridge/evm/chains/*/ "$repo_root"/contracts/bridge/solana/chains/*/; do
		[ -d "$dir" ] && chains+=("$(basename "$dir")")
	done
	if [ "${#chains[@]}" -eq 0 ]; then
		echo "fail checklist chains=none"
		failures=$((failures + 1))
	fi
	for chain in ${chains[@]+"${chains[@]}"}; do
		record="$PAXEER_BRIDGE_RECORDS_DIR/$chain.json"
		if [ ! -r "$record" ]; then
			echo "fail checklist chain=$chain record=absent"
			failures=$((failures + 1))
			continue
		fi
		status=0
		out="$(PAXEER_BRIDGE_DEPLOYMENT_RECORD="$record" timeout "$((timeout * 10))" "$repo_root/services/bridge/deploy/checklist.sh" "$chain" 2>&1 </dev/null)" || status=$?
		if [ "$status" -eq 0 ]; then
			echo "pass checklist chain=$chain exit=0"
		else
			echo "fail checklist chain=$chain exit=$status $(printf '%s\n' "$out" | tail -n 1 | sed -E 's#[A-Za-z][A-Za-z0-9+.-]*://[^[:space:]]*#<url>#g' | cut -c1-160)"
			failures=$((failures + 1))
		fi
	done

	for n in 1 2 3 4 5; do
		toml="services/interop/deploy/bridge-relayer/relayer-$n.toml"
		if ! app="$(fly_app "$toml")"; then
			echo "fail bridge toml=absent relayer=$n"
			failures=$((failures + 1))
			continue
		fi
		reply="$(fly_ssh "$app" - "journal=$journal sh -s" <<<"$bridge_machine_script")" || reply=""
		relayer="$(sed -n 's/^@@relayer //p' <<<"$reply" | head -n 1)"
		signer="$(sed -n 's/^@@signer //p' <<<"$reply" | head -n 1)"
		if [ -z "$relayer" ]; then
			echo "fail processes app=$app machine=unreachable"
			failures=$((failures + 1))
			continue
		elif [ "$relayer" = running ] && [ "$signer" = running ]; then
			echo "pass processes app=$app relayer=running signer=running"
		else
			echo "fail processes app=$app relayer=$relayer signer=${signer:-none}"
			failures=$((failures + 1))
		fi
		if grep -qx '@@journal present' <<<"$reply"; then
			journals=$((journals + 1))
		fi
		if [ -z "$item" ]; then
			item="$(sed -n 's/^@@bridge-in //p' <<<"$reply" | head -n 1)"
			item_app="$app"
		fi
	done
	if [ -n "$item" ]; then
		echo "pass bridge-in app=$item_app item=$item outcome=included"
	elif [ "$journals" -gt 0 ]; then
		echo "fail bridge-in relayers=5 journal=present included=none"
		failures=$((failures + 1))
	else
		echo "fail bridge-in relayers=5 journal=absent"
		failures=$((failures + 1))
	fi
	finish "$failures"
}

# check_wallet: the public wallet endpoint name is the wallet_endpoint of
# [contract_context.108.decision.public_names] in the spec, which must be a host name. The
# wallet feature's cutover gate reads that name, its gateway gate reads the
# wallet gateway app of services/wallet/deploy/gateway.toml at its fly.dev name
# with CHECK_LIVE_GATEWAY_TOKEN passed through, and the app runs started
# machines in two regions. The gates' check lines are printed as they come
# without their summary lines, and a failing gate counts as one failure.
check_wallet() {
	local spec=spec/layerx-merge/spec.kvx
	local app name output status answer failures=0
	local host_re='^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+(:[0-9]{1,5})?$'
	if [ -z "${CHECK_LIVE_GATEWAY_TOKEN:-}" ]; then
		echo "check-live: CHECK_LIVE_GATEWAY_TOKEN is required; it is the access token of a provisioned test identity of the wallet gateway" >&2
		exit 2
	fi
	if ! command -v "$railway_bin" >/dev/null 2>&1; then
		echo "check-live: $railway_bin is required" >&2
		exit 2
	fi
	if ! app="$(railway_service wallet-gateway)"; then
		echo "fail wallet rollout=absent"
		finish 1
	fi

	name="$(bringup_inspect wallet-name "$repo_root/$spec")" || name=""
	if grep -Eq "$host_re" <<<"$name"; then
		echo "pass endpoint name=$name source=spec"
		status=0
		output="$(CHECK_LIVE_CUTOVER_HOST="$name" "$repo_root/scripts/wallet/check-live.sh" cutover 2>&1)" || status=$?
		grep -Ev '^check-live: (all checks passed|[0-9]+ check\(s\) failed)$' <<<"$output" || true
		[ "$status" -eq 0 ] || failures=$((failures + 1))
	else
		echo "fail endpoint source=spec name=unset"
		failures=$((failures + 1))
	fi

	status=0
	if ! answer="$(railway_origin "$app")"; then
		echo "fail wallet service=$app public-domain=absent"
		finish $((failures + 1))
	fi
	output="$(CHECK_LIVE_GATEWAY_BASE="$answer" "$repo_root/scripts/wallet/check-live.sh" gateway 2>&1)" || status=$?
	grep -Ev '^check-live: (all checks passed|[0-9]+ check\(s\) failed)$' <<<"$output" || true
	[ "$status" -eq 0 ] || failures=$((failures + 1))

	railway_deployment "$app" || failures=$((failures + 1))
	finish "$failures"
}

# Probe every running replica selected by the deployment record. Identity and
# region come from that selected process, never from the region loop's label.
railway_gateway_regions() {
	local app="$1" work selections deployment instance reply probe failed=0
	work="$(mktemp -d)" || return 1
	if ! railway_instance "$app" >"$work/deployment.json" ||
		! selections="$(bringup_inspect regional-instances <"$work/deployment.json")"; then
		echo "fail regional-readiness service=$app deployment=unreadable-or-incomplete"
		rm -rf "$work"
		return 1
	fi
	probe='import json,os,urllib.request
with urllib.request.urlopen("http://127.0.0.1:8080/readyz",timeout=10) as r:
    print(json.dumps({"instance":os.environ["RAILWAY_REPLICA_ID"],"deployment":os.environ["RAILWAY_DEPLOYMENT_ID"],"region":os.environ["RAILWAY_REPLICA_REGION"],"body":json.load(r)}))'
	: >"$work/responses.jsonl"
	while read -r deployment instance; do
		if reply="$(printf '%s' "$probe" | railway_ssh "$app" "python3 -" "$instance")"; then
			printf '%s\n' "$reply" >>"$work/responses.jsonl"
		else
			echo "fail readyz service=$app deployment=$deployment instance=$instance response=absent"
			failed=1
		fi
	done <<<"$selections"
	if [ "$failed" -eq 0 ] && python3 -c 'import json,sys; print(json.dumps([json.loads(line) for line in sys.stdin]))' <"$work/responses.jsonl" >"$work/responses.json" &&
		bringup_inspect regional-readiness "$work/deployment.json" <"$work/responses.json"; then
		rm -rf "$work"
		return 0
	fi
	echo "fail regional-readiness service=$app coverage=unproven"
	rm -rf "$work"
	return 1
}

# Node root methods and hosted OpenRPC reads have separate owners and routes.
check_router() {
	router_prerequisite
	load_rpc_inventory || { echo "fail rpc inventory=unreadable"; finish 1; }
	local gateway="${CHECK_LIVE_GATEWAY_RPC:-}" account="${CHECK_LIVE_ROUTER_ACCOUNT:-}"
	local app wallet body answer failures=0
	if [ -z "$gateway" ]; then
		echo "check-live: CHECK_LIVE_GATEWAY_RPC must name the hosted gateway JSON-RPC URL" >&2
		exit 2
	fi
	if ! app="$(railway_service gateway)"; then
		echo "fail gateway rollout=absent"
		finish 1
	fi
	bringup_inspect route-check "$rpc_inventory" "$gateway" "$account" || failures=$((failures + 1))
	railway_deployment "$app" || failures=$((failures + 1))
	railway_gateway_regions "$app" || failures=$((failures + 1))
	if ! wallet="$(railway_service wallet-gateway)"; then
		echo "fail wallet-gateway rollout=absent"
		finish $((failures + 1))
	fi
	body="$(railway_ssh "$wallet" 'node -e "fetch(\"http://127.0.0.1:8080/readyz\").then(r => r.text()).then(t => process.stdout.write(t))"' </dev/null | tail -n 1)" || body=""
	if answer="$(bringup_inspect pool-check "$rpc_inventory" <<<"$body")"; then
		echo "$answer app=$wallet"
	else
		echo "fail wallet-gateway app=$wallet rpc_pool=invalid-node-endpoints"
		failures=$((failures + 1))
	fi
	finish "$failures"
}

# check_kernel_boundaries: inside the machine of the kernel app of
# services/wallet/deploy/human.toml, the core boundary (9443), its admin plane
# (9444), the receipt authority (9445) and the agent boundary (9446) each
# answer /readyz 200 with "ready":true over TLS verified under the internal CA
# at the app's .internal name, presenting the agentd-client identity that
# tools/bringup/ca.sh issued to the app's volume; and no service of any
# machine of the app exposes one of those ports. One line per check.
check_kernel_boundaries() {
	local app answer n_machines ports exposed port name line code body attempt failures=0
	local -A names=([9443]=core [9444]=core-admin [9445]=receipt-authority [9446]=agent-boundary)
	if ! app="$(fly_app services/wallet/deploy/human.toml)"; then
		echo "fail kernel-boundaries toml=absent"
		finish 1
	fi
	answer="$(timeout "$timeout" flyctl machines list --app "$app" --json 2>/dev/null | python3 -c '
import json, sys
ms = json.load(sys.stdin)
ports = sorted({s.get("internal_port") for m in ms for s in (m.get("config") or {}).get("services") or [] if s.get("internal_port")})
print(len(ms), ",".join(str(p) for p in ports) or "none")
' 2>/dev/null)" || answer=""
	read -r n_machines ports <<<"${answer:-none none}"
	exposed=""
	for port in 9443 9444 9445 9446; do
		[[ ",$ports," != *",$port,"* ]] || exposed="${exposed:+$exposed,}$port"
	done
	if [ "$n_machines" != none ] && [ "$n_machines" -gt 0 ] && [ -z "$exposed" ]; then
		echo "pass public-services app=$app machines=$n_machines ports=$ports boundaries=none"
	else
		echo "fail public-services app=$app machines=$n_machines ports=$ports boundaries=${exposed:-none}"
		failures=$((failures + 1))
	fi
	answer=""
	for attempt in 1 2; do
		answer="$(fly_ssh "$app" - "d=$tls_dir/agentd-client; for p in 9443 9444 9445 9446; do printf \"%s \" \$p; curl -sS -m $timeout --cacert \$d/ca.pem --cert \$d/cert.pem --key \$d/key.pem -w \" %{http_code}\" https://$app.internal:\$p/readyz 2>/dev/null | tr -d \"\\n\"; echo; done")" || true
		[ "$(grep -c ' 200$' <<<"$answer")" != 4 ] || break
		[ "$attempt" = 2 ] || sleep 10
	done
	for port in 9443 9444 9445 9446; do
		name="${names[$port]}"
		line="$(grep -m 1 "^$port " <<<"$answer")" || line="$port "
		line="${line#"$port "}"
		code="${line##* }"
		body="${line% *}"
		[[ "$code" =~ ^[0-9]{3}$ ]] || code=none
		if [ "$code" = 200 ] && [[ "$body" == *'"ready":true'* ]]; then
			echo "pass $name app=$app url=https://$app.internal:$port/readyz identity=agentd-client http=200 ready=true"
		else
			echo "fail $name app=$app url=https://$app.internal:$port/readyz identity=agentd-client http=$code"
			failures=$((failures + 1))
		fi
	done
	finish "$failures"
}

# check_human: the human service of the kernel app answers at
# CHECK_LIVE_HUMAN_BASE, by default https://api-hull.paxeer.network, as the
# wallet origin CHECK_LIVE_HUMAN_ORIGIN, by default https://paxportwallet.com:
# the live, preflight and plan checks of scripts/wallet/check-live.sh human,
# /readyz 200 with ready true, and the passkey registration options of the
# probe account CHECK_LIVE_HUMAN_PROBE_EMAIL, created under an idempotency key
# derived from that address so every run converges on one account, naming the
# origin's host as rp.id. One line per check.
check_human() {
	local base="${CHECK_LIVE_HUMAN_BASE:-https://api-hull.paxeer.network}"
	local origin="${CHECK_LIVE_HUMAN_ORIGIN:-https://paxportwallet.com}"
	local email="${CHECK_LIVE_HUMAN_PROBE_EMAIL:-}" rp="${origin#https://}"
	local failures=0 answer code body account key child_output child_status=0
	base="${base%/}"
	child_output="$(CHECK_LIVE_HUMAN_BASE="$base" CHECK_LIVE_HUMAN_ORIGIN="$origin" CHECK_LIVE_TIMEOUT="$timeout" "$repo_root/scripts/wallet/check-live.sh" human 2>&1)" || child_status=$?
	bringup_inspect human-child "$child_status" <<<"$child_output" || failures=$((failures + 1))
	answer="$(curl -sS --max-time "$timeout" -H "origin: $origin" -w ' %{http_code}' "$base/readyz" 2>/dev/null)" || answer=""
	code="${answer##* }"
	body="${answer% *}"
	if [ "$code" = 200 ] && printf '%s' "$body" | python3 -c 'import json, sys; sys.exit(0 if json.load(sys.stdin)["result"]["ready"] is True else 1)' 2>/dev/null; then
		echo "pass readyz http=200 ready=true"
	else
		echo "fail readyz http=${code:-none} $(printf '%s' "$body" | tr '\n' ' ' | cut -c1-200)"
		failures=$((failures + 1))
	fi
	if [ -z "$email" ]; then
		echo "fail rp-id email=unset"
		finish $((failures + 1))
	fi
	key="$(printf 'check-live human %s' "$email" | sha256sum | cut -c1-32)"
	answer="$(python3 -c 'import json, sys; print(json.dumps({"email": sys.argv[1], "display_name": "check-live human"}))' "$email" |
		curl -sS --max-time "$timeout" -X POST -H "origin: $origin" -H 'content-type: application/json' -H "idempotency-key: $key" --data-binary @- -w ' %{http_code}' "$base/v1/accounts" 2>/dev/null)" || answer=""
	code="${answer##* }"
	account="$(printf '%s' "${answer% *}" | python3 -c 'import json, sys; print(json.load(sys.stdin)["result"]["account_id"])' 2>/dev/null)" || account=""
	if [ -z "$account" ]; then
		echo "fail rp-id account=none http=${code:-none}"
		finish $((failures + 1))
	fi
	answer="$(python3 -c 'import json, sys; print(json.dumps({"account_id": sys.argv[1]}))' "$account" |
		curl -sS --max-time "$timeout" -X POST -H "origin: $origin" -H 'content-type: application/json' --data-binary @- -w ' %{http_code}' "$base/v1/passkeys/registrations" 2>/dev/null)" || answer=""
	code="${answer##* }"
	answer="$(printf '%s' "${answer% *}" | python3 -c '
import base64, json, sys
ceremony = json.load(sys.stdin)["result"]["ceremony"]
print(json.loads(base64.urlsafe_b64decode(ceremony + "=" * (-len(ceremony) % 4)))["rp"]["id"])
' 2>/dev/null)" || answer=""
	if [ "$answer" = "$rp" ]; then
		echo "pass rp-id http=$code rp.id=$answer"
	else
		echo "fail rp-id http=${code:-none} rp.id=${answer:-none} want=$rp"
		failures=$((failures + 1))
	fi
	finish "$failures"
}

# kernel_roster: the expected service roster of the kernel app's launch
# contract, the full profile docker/kernel/init.sh runs from the final stage of
# docker/kernel/Dockerfile on the kernel box. One row
# per role: name, uid, "genesis" when the role's waits name a genesis output
# (so waiting on the genesis is its permitted bootstrap state) or "-", and the
# role's material boundary as comma-separated path=uid:gid[:mode] entries, or
# "-" for a role that holds no private material.
kernel_roster() {
	if ! [[ "${CHECK_LIVE_PAXD_HOME:-}" =~ ^/[a-zA-Z0-9_./-]+$ ]]; then
		echo "check-live: CHECK_LIVE_PAXD_HOME must name the mounted paxd home" >&2
		return 1
	fi
	printf 'paxd 4020 - %s=4020:4020:700\n' "$CHECK_LIVE_PAXD_HOME"
	cat <<'ROSTER'
treasury-signer 4020 genesis /data/layerx/keys/treasury.key=4020:4020:400
paxeer-hop-1 4020 - -
paxeer-boundary-loopback 4020 - /data/tls/paxeer-boundary-loopback=4020:4020
paxeer-hop-2 4020 - -
paxeer-boundary-public 4020 - /data/tls/paxeer-boundary-public=4020:4020
paxeer-relay 4020 - -
core-boundary 4021 genesis /data/layerx/core=4021:4020:2700,/data/tls/pending-core=4021:4020,/data/tls/pending-core-admin=4021:4020
agent-boundary 4021 genesis /data/layerx/agent-boundary=4021:4020:2700,/data/tls/agent-boundary=4021:4020
human-kms 4026 genesis /run/human-private/kms=4026:4020:700,/data/human-state/kms=4026:4020:700,/run/layerx/human-material/human-kms=4026:4020:2500
human-components 4020 genesis /run/human-private/components=4020:4020:700,/data/human-state/components=4020:4020:700,/run/layerx/human-material/human-components=4020:4020:2500
human-identity 4020 genesis /run/human-private/identity=4020:4020:700,/data/human-state/identity=4020:4020:700,/run/layerx/human-material/human-identity=4020:4020:2500
human-security 4020 genesis /run/human-private/security=4020:4020:700,/data/human-state/security=4020:4020:700,/run/layerx/human-material/human-security=4020:4020:2500
human-movement 4020 genesis /run/human-private/movement=4020:4020:700,/data/human-state/movement=4020:4020:700,/run/layerx/human-material/human-movement=4020:4020:2500
human-owner 4021 genesis /run/human-private/agent=4021:4020:700,/data/human-state/agent=4021:4020:700,/run/layerx/human-material/human-owner=4021:4020:2500
human 4020 - /run/human-private/service=4020:4020:700
human-tls 4020 - /run/human-private/service=4020:4020:700,/data/tls/human=4020:4020
mirror-signer 4021 genesis /run/mirror-signer=4021:4020:700
mirror-publisher 4021 genesis /run/mirror-publisher=4021:4020:700
ROSTER
}

# kernel_roster_paths: every material path of the roster once, space separated.
kernel_roster_paths() {
	kernel_roster | awk '$4 != "-" { n = split($4, items, ","); for (i = 1; i <= n; i++) { sub(/=.*/, "", items[i]); if (!seen[items[i]]++) printf "%s%s", (out++ ? " " : ""), items[i] } }'
}

# kernel_app_probe: run as root inside the kernel machine with the init status
# directory and the roster's material paths as arguments, prints one JSON
# object: the init pid, every status record, every process descending from the
# init with its uids, gids, groups, no_new_privs flag and parent, and the
# owner, group, mode and type of each material path (not followed). A zombie
# or dead process is reported with its state and never counts as running.
# shellcheck disable=SC2016 # Python source
kernel_app_probe='import json, os, stat, sys
status, paths = sys.argv[1], sys.argv[2:]
out = {"init": None, "status": {}, "procs": {}, "material": {}}
def proc(pid):
    try:
        fields = {}
        with open("/proc/%d/status" % pid) as handle:
            for line in handle:
                key, _, value = line.partition(":")
                fields[key] = value.strip()
        with open("/proc/%d/cmdline" % pid, "rb") as handle:
            cmdline = handle.read(4096).replace(b"\0", b" ").decode("utf-8", "replace").strip()
        return {"ppid": int(fields["PPid"]), "uids": [int(x) for x in fields["Uid"].split()],
                "gids": [int(x) for x in fields["Gid"].split()], "groups": [int(x) for x in fields.get("Groups", "").split()],
                "nnp": int(fields.get("NoNewPrivs", "0")), "comm": fields.get("Name", ""), "state": fields.get("State", "?")[:1],
                "cmdline": cmdline[:240]}
    except (OSError, ValueError, KeyError):
        return None
try:
    with open(os.path.join(status, "pid")) as handle:
        out["init"] = int(handle.read(64).strip())
except (OSError, ValueError):
    pass
try:
    names = sorted(os.listdir(status))
except OSError:
    names = []
for name in names:
    if name == "pid":
        continue
    path = os.path.join(status, name)
    try:
        if not stat.S_ISREG(os.lstat(path).st_mode):
            out["status"][name] = "!not-a-file"
            continue
        with open(path) as handle:
            out["status"][name] = handle.read(4096).split("\n", 1)[0]
    except OSError:
        out["status"][name] = "!unreadable"
table = {}
for entry in os.listdir("/proc"):
    if entry.isdigit():
        info = proc(int(entry))
        if info is not None:
            table[int(entry)] = info
if out["init"] in table:
    keep, frontier = {out["init"]}, [out["init"]]
    while frontier:
        parent = frontier.pop()
        for pid, info in table.items():
            if info["ppid"] == parent and pid not in keep:
                keep.add(pid)
                frontier.append(pid)
    out["procs"] = {str(pid): table[pid] for pid in sorted(keep)}
for path in paths:
    try:
        info = os.lstat(path)
    except OSError:
        out["material"][path] = None
        continue
    kind = "dir" if stat.S_ISDIR(info.st_mode) else "file" if stat.S_ISREG(info.st_mode) else "link" if stat.S_ISLNK(info.st_mode) else "other"
    out["material"][path] = [kind, info.st_uid, info.st_gid, format(stat.S_IMODE(info.st_mode), "o")]
print(json.dumps(out, sort_keys=True))
'

# kernel_app_verdict <app> <candidate>: reads the kernel_app_probe JSON on
# stdin and compares it to kernel_roster. Prints one line per check, the
# candidate identity and the role-specific reason on every role line, then
# "verdict <failures> <waiting-genesis>". A role passes when its status names
# its roster uid, its recorded process is alive under a service supervisor of
# the init, every process of its tree runs under exactly that uid with gid 4020,
# no supplementary group and no_new_privs (the pid-namespace unshare wrapper of
# the recorded pid alone excepted), and every material path of its row has
# the row's type, owner, group and mode. A missing, unexpected or wrong-UID
# role, a live role process no status records (a duplicate launch), a
# non-genesis wait, and a genesis wait of a role whose row does not permit it
# each fail; a permitted genesis wait is counted, never passed as running.
kernel_app_verdict() {
	python3 -c '
import json, sys
app, candidate, roster_text = sys.argv[1], sys.argv[2], sys.argv[3]
lines, failures, waiting, running = [], 0, 0, 0
def fail(text):
    global failures
    failures += 1
    lines.append("fail " + text + " candidate=" + candidate)
roster = {}
for row in roster_text.splitlines():
    fields = row.split()
    if len(fields) != 4 or not fields[1].isdigit() or fields[2] not in ("genesis", "-"):
        fail("roster row=%s reason=malformed-row" % (fields[0] if fields else "empty"))
        continue
    if fields[0] in roster:
        fail("roster row=%s reason=duplicate-row" % fields[0])
        continue
    material = []
    if fields[3] != "-":
        for item in fields[3].split(","):
            path, _, spec = item.partition("=")
            want = spec.split(":")
            if not path.startswith("/") or len(want) not in (2, 3) or not all(x.isdigit() for x in want):
                fail("roster row=%s reason=malformed-material item=%s" % (fields[0], item))
                continue
            material.append((path, want))
    roster[fields[0]] = (int(fields[1]), fields[2] == "genesis", material)
try:
    probe = json.loads(sys.stdin.read())
    init = probe["init"]
    procs = {int(pid): info for pid, info in probe["procs"].items()}
    records = probe["status"]
    found = probe["material"]
except (ValueError, KeyError, TypeError, AttributeError):
    probe = None
if probe is None or init not in procs:
    fail("init app=%s status=absent" % app)
    print("\n".join(lines))
    print("verdict %d 0" % failures)
    sys.exit(0)
info = procs[init]
if info["uids"][1] == 0 and any(word.endswith("kernel-init") for word in info["cmdline"].split(" ")[:2]):
    lines.append("pass init app=%s uid=0 entrypoint=kernel-init pid=%d" % (app, init))
else:
    fail("init app=%s uid=%d entrypoint=%s" % (app, info["uids"][1], info["cmdline"].split(" ", 1)[0] or "none"))
children = {}
for pid, value in procs.items():
    children.setdefault(value["ppid"], []).append(pid)
def tree(root):
    seen, frontier = [root], [root]
    while frontier:
        for child in children.get(frontier.pop(), []):
            seen.append(child)
            frontier.append(child)
    return seen
recorded = {}
for name in sorted(set(records) - set(roster)):
    fields = records[name].split(" ")
    if len(fields) == 3 and fields[1] == "running" and fields[2].isdigit():
        recorded.setdefault(int(fields[2]), []).append(name)
    fail("role %s state=unexpected status=%s reason=not-in-roster" % (name, records[name].replace(" ", "_") or "empty"))
for name, (want, genesis, material) in roster.items():
    head = "role %s uid=%d" % (name, want)
    if name not in records:
        fail(head + " state=missing reason=no-status")
        continue
    fields = records[name].split(" ", 2)
    if len(fields) < 3 or not fields[0].isdigit():
        fail(head + " state=malformed status=%s reason=malformed-status" % (records[name].replace(" ", "_") or "empty"))
        continue
    uid, state, detail = int(fields[0]), fields[1], fields[2]
    if uid != want:
        if state == "running" and detail.isdigit():
            recorded.setdefault(int(detail), []).append(name)
        fail("role %s uid=%d want=%d state=%s reason=wrong-uid" % (name, uid, want, state))
        continue
    if state == "waiting":
        if detail == "genesis" and genesis:
            waiting += 1
            lines.append("wait " + head + " state=waiting-genesis candidate=" + candidate)
        elif detail == "genesis":
            fail(head + " state=waiting on=genesis reason=genesis-wait-not-permitted")
        else:
            fail(head + " state=waiting on=%s reason=waiting" % detail.replace(" ", "_"))
        continue
    if state != "running" or not detail.isdigit():
        fail(head + " state=%s reason=malformed-status" % state)
        continue
    pid = int(detail)
    recorded.setdefault(pid, []).append(name)
    if pid not in procs or procs[pid]["state"] in ("Z", "X"):
        fail(head + " state=running pid=%d reason=process-absent" % pid)
        continue
    supervisor = procs[pid]["ppid"]
    if supervisor not in procs or procs[supervisor]["ppid"] != init:
        fail(head + " state=running pid=%d reason=unsupervised" % pid)
        continue
    problems = []
    members = tree(pid)
    role = [p for p in members if not (p == pid and procs[p]["comm"] == "unshare" and set(procs[p]["uids"]) == {0})]
    if not role:
        problems.append("reason=no-role-process")
    for p in role:
        value = procs[p]
        if set(value["uids"]) != {want}:
            problems.append("reason=process-uid pid=%d uids=%s comm=%s" % (p, ",".join(map(str, value["uids"])), value["comm"]))
        elif set(value["gids"]) != {4020} or value["groups"] or value["nnp"] != 1:
            problems.append("reason=process-privileges pid=%d gids=%s groups=%s no_new_privs=%d" % (
                p, ",".join(map(str, value["gids"])), ",".join(map(str, value["groups"])) or "none", value["nnp"]))
    for path, spec in material:
        seen = found.get(path)
        if seen is None:
            problems.append("reason=material-absent path=%s" % path)
            continue
        observed = [str(seen[1]), str(seen[2]), seen[3]][:len(spec)]
        if seen[0] not in ("dir", "file") or observed != spec:
            problems.append("reason=material path=%s want=%s observed=%s:%s" % (path, ":".join(spec), seen[0], ":".join(map(str, seen[1:]))))
    if problems:
        for problem in problems:
            fail(head + " state=running pid=%d " % pid + problem)
        continue
    running += 1
    lines.append("pass " + head + " state=running pid=%d processes=%d material=%d candidate=%s" % (pid, len(role), len(material), candidate))
for pid, names in sorted(recorded.items()):
    if len(names) > 1:
        fail("role %s state=running pid=%d reason=duplicate-process shared=%s" % (names[0], pid, ",".join(names)))
for pid, value in sorted(procs.items()):
    parent = procs.get(value["ppid"])
    if (pid not in recorded and pid != init and parent is not None and parent["ppid"] == init
            and (set(value["uids"]) != {0} or value["comm"] in ("unshare", "setpriv"))):
        fail("role - state=running pid=%d uids=%s comm=%s reason=unrecorded-role-process" % (
            pid, ",".join(map(str, value["uids"])), value["comm"]))
if failures == 0 and waiting:
    lines.append("bootstrap app=%s candidate=%s state=pre-genesis roles=%d running=%d waiting-genesis=%d operational=no" % (
        app, candidate, len(roster), running, waiting))
elif failures == 0:
    lines.append("pass roster app=%s candidate=%s roles=%d running=%d operational=yes" % (app, candidate, len(roster), running))
print("\n".join(lines))
print("verdict %d %d" % (failures, waiting))
' "$1" "$2" "$(kernel_roster)"
}

# Sourced by tools/bringup/ca.sh for the Fly helpers and the CA settings: the
# probe's own dispatch below runs only when this file is executed.
[ "${BASH_SOURCE[0]}" = "$0" ] || return 0

# check_kernel_app: the kernel app of services/wallet/deploy/human.toml runs
# exactly two machines, both started, each with its own volume mounted at
# /data, from one candidate image whose identity every line records; the init of
# docker/kernel/init.sh runs as root as the entrypoint; and the roles the init
# records under /run/layerx/init are exactly the kernel_roster, each with its
# process and material boundary (kernel_app_verdict). One line per check.
# Exits 0 when every role runs, 3 when the only roles not running are roster
# roles permitted to wait on the genesis (the pre-genesis bootstrap state,
# never reported as operational), and 1 on any failure.
check_kernel_app() {
	local app answer n_machines n_started n_data candidate verdict failures waiting
	if ! command -v flyctl >/dev/null 2>&1; then
		echo "check-live: flyctl is required" >&2
		exit 2
	fi
	if ! app="$(fly_app services/wallet/deploy/human.toml)"; then
		echo "fail kernel-app toml=absent"
		finish 1
	fi
	answer="$(timeout "$timeout" flyctl machines list --app "$app" --json 2>/dev/null | python3 -c '
import json, sys
ms = json.load(sys.stdin)
started = [m for m in ms if m.get("state") == "started"]
data = [x for m in started for x in (m.get("config") or {}).get("mounts") or [] if x.get("path") == "/data" and x.get("volume")]
def identity(m):
    ref = m.get("image_ref") or {}
    if ref.get("digest"):
        return "%s@%s" % (ref.get("repository") or "image", ref["digest"])
    return (m.get("config") or {}).get("image") or "none"
ids = sorted({identity(m) for m in started})
print(len(ms), len(started), len(data), ids[0] if len(ids) == 1 and " " not in ids[0] else "none")
' 2>/dev/null)" || answer=""
	read -r n_machines n_started n_data candidate <<<"${answer:-none none none none}"
	if [ "$n_machines" = 2 ] && [ "$n_started" = 2 ] && [ "$n_data" = 2 ] && [ "$candidate" != none ]; then
		echo "pass machines app=$app machines=2 started=2 volume=/data candidate=$candidate"
	else
		echo "fail machines app=$app machines=$n_machines started=$n_started volume-at-data=$n_data candidate=$candidate"
		finish 1
	fi
	answer="$(printf '%s' "$kernel_app_probe" | fly_ssh "$app" - "python3 - /run/layerx/init $(kernel_roster_paths)")" || answer=""
	verdict="$(kernel_app_verdict "$app" "$candidate" <<<"$answer")" || verdict=""
	read -r _ failures waiting <<<"$(grep '^verdict ' <<<"$verdict" || true)" || true
	if ! [[ "${failures:-}" =~ ^[0-9]+$ && "${waiting:-}" =~ ^[0-9]+$ ]]; then
		echo "fail roster app=$app candidate=$candidate verdict=unreadable"
		finish 1
	fi
	sed '/^verdict /d' <<<"$verdict"
	if [ "$failures" -eq 0 ] && [ "$waiting" -gt 0 ]; then
		echo "check-live: pre-genesis bootstrap, $waiting role(s) waiting on the genesis; not operational"
		exit 3
	fi
	finish "$failures"
}

# check_mirrors: inside the kernel machine of services/wallet/deploy/human.toml
# the mirror publisher's status listener, CHECK_LIVE_MIRRORS_STATUS
# (host:port, default 127.0.0.1:9456, the status_listen docker/kernel/init.sh
# runs it on), answers GET /readyz 200 ready; the verifier config
# CHECK_LIVE_MIRRORS_VERIFY_CONFIG names the Ethereum and the Solana mirror
# as sources, with their RPC endpoints, CA and bearer files; and
# layerx-mirror-verify (CHECK_LIVE_MIRRORS_VERIFY_BIN, default the one on
# PATH) verifies the receipt of CHECK_LIVE_MIRRORS_REQUEST from the mirrors
# alone, with LAYERX_NODE_URL, LAYERX_GATEWAY_URL and
# LAYERX_EXPLORER_API_ORIGIN removed from its environment, bounded by
# CHECK_LIVE_MIRRORS_VERIFY_TIMEOUT seconds (default 300). One line per check.
check_mirrors() {
	local status="${CHECK_LIVE_MIRRORS_STATUS:-127.0.0.1:9456}"
	local verify="${CHECK_LIVE_MIRRORS_VERIFY_BIN:-layerx-mirror-verify}"
	local limit="${CHECK_LIVE_MIRRORS_VERIFY_TIMEOUT:-300}"
	local name app reply rc code body answer eth sol failures=0
	# shellcheck disable=SC2016 # the script expands on the machine
	local script='rc=0
body=$(curl -sS -m "$limit" -w "\n%{http_code}" "http://$status/readyz" 2>/dev/null) || rc=$?
echo "@@readyz $rc $(printf "%s" "$body" | tail -n 1) $(printf "%s" "$body" | head -n 1 | tr -d " " | cut -c1-120)"'
	for name in CHECK_LIVE_MIRRORS_VERIFY_CONFIG CHECK_LIVE_MIRRORS_REQUEST; do
		if [ -z "${!name:-}" ]; then
			echo "check-live: $name is unset" >&2
			exit 2
		elif [ ! -r "${!name}" ]; then
			echo "check-live: $name does not name a readable file" >&2
			exit 2
		fi
	done
	for name in flyctl "$verify"; do
		if ! command -v "$name" >/dev/null 2>&1; then
			echo "check-live: $name is required" >&2
			exit 2
		fi
	done
	if ! app="$(fly_app services/wallet/deploy/human.toml)"; then
		echo "fail mirrors toml=absent"
		finish 1
	fi

	reply="$(printf '%s\n' "$script" | fly_ssh "$app" - "status=$status limit=$timeout sh -s")" || reply=""
	read -r rc code body <<<"$(sed -n 's/^@@readyz //p' <<<"$reply" | head -n 1)"
	if [ "${rc:-}" = 0 ] && [ "${code:-}" = 200 ] && [[ "${body:-}" == *'"ready":true'* ]]; then
		echo "pass mirror-readyz app=$app listen=$status http=200 ready=true"
	else
		echo "fail mirror-readyz app=$app listen=$status curl=${rc:-none} http=${code:-none}"
		failures=$((failures + 1))
	fi

	answer="$(python3 -c '
import json, sys
kinds = [s.get("kind") for s in json.load(open(sys.argv[1])).get("sources", [])]
print(kinds.count("ethereum"), kinds.count("solana"))
' "$CHECK_LIVE_MIRRORS_VERIFY_CONFIG" 2>/dev/null)" || answer=""
	read -r eth sol <<<"${answer:-none none}"
	if [ "$eth" != none ] && [ "$eth" -ge 1 ] && [ "$sol" -ge 1 ]; then
		echo "pass mirror-sources ethereum=$eth solana=$sol"
	else
		echo "fail mirror-sources ethereum=$eth solana=$sol"
		failures=$((failures + 1))
	fi

	answer="$(env -u LAYERX_NODE_URL -u LAYERX_GATEWAY_URL -u LAYERX_EXPLORER_API_ORIGIN \
		timeout "$limit" "$verify" "$CHECK_LIVE_MIRRORS_VERIFY_CONFIG" <"$CHECK_LIVE_MIRRORS_REQUEST" 2>/dev/null)" || true
	answer="$(python3 -c '
import json, sys
try:
    r = json.loads(sys.stdin.read())
except ValueError:
    print("fail error=unreadable")
    sys.exit(0)
v = r.get("verification") or {}
if r.get("ok") is True and v.get("provenance") == "Canonical" and v.get("sourceId"):
    print("pass source=%s batch=%s provenance=Canonical level=%s" % (v["sourceId"], v.get("batchNumber"), v.get("level")))
else:
    print("fail error=%s provenance=%s" % (r.get("error", "none"), v.get("provenance", "none")))
' <<<"$answer")"
	echo "${answer%% *} mirror-verify ${answer#* }"
	[ "${answer%% *}" = pass ] || failures=$((failures + 1))
	finish "$failures"
}

# The explicit archive node accepts signed chain bytes and serves their
# successful EVM receipt and native committed receipt from the same block.
check_relay() {
	local origin="${CHECK_LIVE_RELAY_ORIGIN:-}" transaction="${CHECK_LIVE_RELAY_TRANSACTION:-}"
	if [ -z "$origin" ] || [ -z "$transaction" ] || [ ! -s "$transaction" ]; then
		echo "check-live: CHECK_LIVE_RELAY_ORIGIN and readable CHECK_LIVE_RELAY_TRANSACTION are required" >&2
		exit 2
	fi
	if timeout "${CHECK_LIVE_RELAY_DELIVERY_TIMEOUT:-180}" "${BRINGUP_INSPECT_BIN:-bringup-inspect}" archive-delivery "$origin" "$transaction" "${CHECK_LIVE_RELAY_CA:--}"; then
		finish 0
	fi
	echo "fail archive-delivery committed-read-back=unproven"
	finish 1
}

# A pre-signed payExact transaction preserves operator custody. The real
# account/nonce/balance, seller, buyer, settlement and committed read-back
# paths all have to agree on its decoded transfer facts.
check_interop() {
	local origin="${CHECK_LIVE_INTEROP_ORIGIN:-https://interchain.paxeer.network}"
	local gateway="${CHECK_LIVE_INTEROP_RPC:-}" node="${CHECK_LIVE_INTEROP_NODE_RPC:-}"
	local transaction="${CHECK_LIVE_INTEROP_TRANSACTION:-}" key="${CHECK_LIVE_INTEROP_API_KEY_FILE:-}"
	local app answer failures=0
	if [ -z "$gateway" ] || [ -z "$node" ] || [ ! -s "$transaction" ] || [ ! -s "$key" ]; then
		echo "check-live: explicit CHECK_LIVE_INTEROP_RPC, CHECK_LIVE_INTEROP_NODE_RPC, signed CHECK_LIVE_INTEROP_TRANSACTION and CHECK_LIVE_INTEROP_API_KEY_FILE are required" >&2
		exit 2
	fi
	if ! app="$(railway_service interop)"; then
		echo "fail interop rollout=absent"
		finish 1
	fi
	railway_deployment "$app" || failures=$((failures + 1))
	answer="$(curl -fsS --max-time "$timeout" "$origin/readyz" 2>/dev/null)" || answer=""
	if python3 -c 'import json,sys; d=json.load(sys.stdin); c=d["components"]; sys.exit(0 if d["status"]=="ready" and c and all(v=="ready" for v in c.values()) else 1)' <<<"$answer" 2>/dev/null; then
		echo "pass interop readyz=ready"
	else
		echo "fail interop readyz=unavailable"
		failures=$((failures + 1))
	fi
	timeout "${CHECK_LIVE_INTEROP_DELIVERY_TIMEOUT:-180}" "${BRINGUP_INSPECT_BIN:-bringup-inspect}" interop-payment "$origin" "$gateway" "$node" "$transaction" "$key" || failures=$((failures + 1))
	finish "$failures"
}

# check_ramp: the reference ramp app of services/platform/ramps/fly.toml runs one
# started machine with one volume; ramp.paxeer.network is registered on the
# edge host as an http site of that app, resolves to the edge host and
# answers /readyz 200 under a publicly verified certificate and the
# matching upstream readiness body; and
# services/platform/ramps/sandbox-journey.sh against the name records done for one
# on-ramp and one off-ramp order. The journey's inputs are its own variables
# (LAYERX_RAMP_CUSTOMER_TOKEN, LAYERX_RAMP_OPERATOR_URL,
# LAYERX_RAMP_OPERATOR_TOKEN, LAYERX_RAMP_ON_QUOTE_ID,
# LAYERX_RAMP_OFF_QUOTE_ID, LAYERX_RAMP_OFF_GRANT_JSON,
# LAYERX_RAMP_ON_ACCOUNT_SEQUENCE, LAYERX_RAMP_OFF_RECEIVER_SEQUENCE);
# LAYERX_RAMP_CA_PEM defaults to the system CA file. One line per check,
# never printing a token or an address.
check_ramp() {
	local ramp_name="${CHECK_LIVE_RAMP_NAME:-ramp.paxeer.network}" name app answer
	local reply status=0 manifest line edge_addrs addrs at out headers body code railway_id app_body v failures=0
	for name in LAYERX_RAMP_CUSTOMER_TOKEN LAYERX_RAMP_OPERATOR_URL LAYERX_RAMP_OPERATOR_TOKEN LAYERX_RAMP_ON_QUOTE_ID \
		LAYERX_RAMP_OFF_QUOTE_ID LAYERX_RAMP_OFF_GRANT_JSON LAYERX_RAMP_ON_ACCOUNT_SEQUENCE LAYERX_RAMP_OFF_RECEIVER_SEQUENCE; do
		if [ -z "${!name:-}" ]; then
			echo "check-live: $name is unset" >&2
			exit 2
		fi
	done
	for name in "$railway_bin" jq; do
		if ! command -v "$name" >/dev/null 2>&1; then
			echo "check-live: $name is required" >&2
			exit 2
		fi
	done
	if ! app="$(railway_service ramp)"; then
		echo "fail ramp rollout=absent"
		finish 1
	fi
	railway_deployment "$app" /data || failures=$((failures + 1))

	check_ramp_edge "$ramp_name" || failures=$((failures + 1))

	status=0
	LAYERX_RAMP_URL="https://$ramp_name" LAYERX_RAMP_CA_PEM="${LAYERX_RAMP_CA_PEM:-/etc/ssl/certs/ca-certificates.crt}" \
		timeout 10m sh "$repo_root/services/platform/ramps/sandbox-journey.sh" >/dev/null 2>&1 || status=$?
	if [ "$status" -eq 0 ]; then
		echo "pass journey https://$ramp_name on-ramp=done off-ramp=done"
	else
		echo "fail journey https://$ramp_name exit=$status"
		failures=$((failures + 1))
	fi
	finish "$failures"
}

# The deployed process supplies EVM/Comet URLs and their pins. Its real SQLite
# cursor snapshot must agree with the immutable start/cutover files and the
# completed-backfill marker; both hosted history methods must return items.
check_indexer() {
	local gateway_url="${CHECK_LIVE_INDEXER_RPC:-}"
	local account="${CHECK_LIVE_INDEXER_ACCOUNT:-}" app answer unit w reply line side want url pin host
	local subject status code earliest start cutover backfilled live listen method params failures=0
	local script
	if ! [[ "$account" =~ ^0x[0-9a-fA-F]{40}$ ]]; then
		echo "check-live: CHECK_LIVE_INDEXER_ACCOUNT must be a 0x EVM address with indexed history" >&2
		exit 2
	fi
	if [ -z "$gateway_url" ]; then
		echo "check-live: hosted gateway endpoint is not configured (CHECK_LIVE_INDEXER_RPC)" >&2
		exit 2
	fi
	if ! command -v "$railway_bin" >/dev/null 2>&1; then
		echo "check-live: $railway_bin is required" >&2
		exit 2
	fi
	if ! app="$(railway_service indexer)"; then
		echo "fail indexer rollout=absent"
		finish 1
	fi
	answer="$(railway_domains "$app")" || answer=none
	if [ "$answer" = 0 ]; then
		echo "pass public-domains service=$app count=0"
	else
		echo "fail public-domains service=$app count=${answer:-none}"
		failures=$((failures + 1))
	fi
	railway_deployment "$app" /data || failures=$((failures + 1))

	# shellcheck disable=SC2016 # the script expands on the machine
	script='pid=
for d in /proc/[0-9]*; do
	case "$(readlink "$d/exe" 2>/dev/null)" in
	*/layerx-indexer)
		pid=$d
		break
		;;
	esac
done
if [ -z "$pid" ]; then
	echo "@@process none"
	exit 0
fi
env_of() { tr "\000" "\n" <"$pid/environ" | sed -n "s/^$1=//p" | head -n 1; }
for side in EVM COMET; do
	echo "@@$side $(env_of "LAYERX_INDEXER_${side}_URL") $(base64 <"$(env_of "LAYERX_INDEXER_${side}_CA_DER")" 2>/dev/null | tr -d "\n")"
done
db=$(env_of LAYERX_INDEXER_DB)
listen=$(env_of LAYERX_INDEXER_LISTEN)
echo "@@start $(env_of LAYERX_INDEXER_START_BLOCK)"
echo "@@listen $listen"
status=0
answer=$(curl -sS -m "$limit" --cacert "$tls/ca.pem" -w "\n%{http_code}" "https://$app.railway.internal:${listen##*:}/healthz" 2>&1) || status=$?
echo "@@health $status $(printf "%s" "$answer" | tail -n 1) $(printf "%s" "$answer" | head -n 1 | tr -d " " | cut -c1-120)"
echo "@@cutover $(cat "$(dirname "$db")/cutover-height" 2>/dev/null)"
echo "@@retained-start $(cat "$(dirname "$db")/start-block" 2>/dev/null)"
[ "$(cat "$(dirname "$db")/backfill-done" 2>/dev/null)" != "$(cat "$(dirname "$db")/cutover-height" 2>/dev/null)" ] || echo "@@done true"
snapshot=$(mktemp)
if cursors=$(sqlite3 -readonly -separator " " "$db" "ATTACH DATABASE '\''$snapshot'\'' AS evidence; BEGIN; CREATE TABLE evidence.cursors AS SELECT chain, position, hash FROM main.cursors WHERE chain = '\''paxeer'\''; CREATE TABLE evidence.backfill_cursors AS SELECT chain, position, hash FROM main.backfill_cursors WHERE chain = '\''paxeer'\''; SELECT (SELECT position FROM main.backfill_cursors WHERE chain = '\''paxeer'\''), (SELECT position FROM main.cursors WHERE chain = '\''paxeer'\''), (SELECT hash FROM main.backfill_cursors WHERE chain = '\''paxeer'\''), (SELECT hash FROM main.cursors WHERE chain = '\''paxeer'\''); COMMIT;" 2>/dev/null); then
    echo "@@cursors $cursors"
    echo "@@database $(base64 <"$snapshot" | tr -d "\n")"
fi
rm -f "$snapshot"'
	reply="$(printf '%s\n' "$script" | railway_ssh "$app" "tls=$tls_dir/indexer app=$app limit=$timeout sh -s")" || reply=""
	if [ -z "$reply" ] || grep -q '^@@process none' <<<"$reply"; then
		[ -n "$reply" ] && answer=none || answer=unreachable
		echo "fail indexer app=$app process=$answer"
		finish $((failures + 1))
	fi

	listen="$(sed -n 's/^@@listen //p' <<<"$reply" | head -n 1)"
	read -r status code answer <<<"$(sed -n 's/^@@health //p' <<<"$reply" | head -n 1)"
	if [[ "$listen" == "[::]:"* ]] && [ "$status" = 0 ] && [ "$code" = 200 ] && [[ "$answer" == *'"status":"ok"'* ]]; then
		echo "pass listener url=https://$app.railway.internal:${listen##*:}/healthz listen=$listen tls=internal-ca http=200"
	else
		echo "fail listener app=$app listen=${listen:-none} curl=${status:-none} http=${code:-none}"
		failures=$((failures + 1))
	fi

	w="$(mktemp -d)"
	# shellcheck disable=SC2064
	trap "rm -rf '$w'" EXIT
	for side in EVM COMET; do
		read -r url pin <<<"$(sed -n "s/^@@$side //p" <<<"$reply" | head -n 1)"
		host="${url#https://}"
		host="${host%%/*}"
		if [ -n "$host" ] && [ ! -e "$w/$host-1.pem" ]; then
			timeout "$timeout" openssl s_client -connect "${CHECK_LIVE_INDEXER_CONNECT:-$host:443}" -servername "$host" -showcerts </dev/null 2>/dev/null |
				awk -v out="$w/$host" '/-BEGIN CERTIFICATE-/ { n++; in_cert = 1 } in_cert { print >(out "-" n ".pem") } /-END CERTIFICATE-/ { in_cert = 0; close(out "-" n ".pem") }' || true
			cat "$w/$host"-[2-9].pem >"$w/$host-untrusted.pem" 2>/dev/null || : >"$w/$host-untrusted.pem"
		fi
		subject=unreadable
		status=1
		if [ -n "$pin" ] && base64 -d <<<"$pin" 2>/dev/null | openssl x509 -inform DER -out "$w/pin-$side.pem" 2>/dev/null; then
			subject="$(openssl x509 -in "$w/pin-$side.pem" -noout -subject -nameopt sep_multiline,utf8 | sed -n 's/^ *CN=//p')"
			if [ -s "$w/$host-1.pem" ] &&
				openssl verify -CAfile "$w/pin-$side.pem" -untrusted "$w/$host-untrusted.pem" -verify_hostname "$host" "$w/$host-1.pem" >/dev/null 2>&1; then
				status=0
			fi
		fi
		if [[ "$url" == https://* ]] && [ "$status" -eq 0 ]; then
			echo "pass upstream-ca side=${side,,} url=$url root=configured verifies=yes"
		else
			echo "fail upstream-ca side=${side,,} url=${url:-none} pin=${subject// /-} verifies=no"
			failures=$((failures + 1))
		fi
	done

	earliest=""
	if [ -s "$w/pin-COMET.pem" ]; then
		code="$(curl -sS -m "$timeout" --cacert "$w/pin-COMET.pem" -o "$w/comet.json" -w '%{http_code}' -H 'content-type: application/json' \
			--data '{"jsonrpc":"2.0","id":1,"method":"status","params":{}}' "$(sed -n 's/^@@COMET //p' <<<"$reply" | cut -d' ' -f1)" 2>/dev/null)" || code=none
		earliest="$(python3 -c '
import json, sys
doc = json.load(open(sys.argv[1]))
value = str(((doc.get("result") or doc).get("sync_info") or {}).get("earliest_block_height", ""))
print(value if value.isdigit() else "")
' "$w/comet.json" 2>/dev/null)" || earliest=""
		status="$(curl -sS -m "$timeout" --cacert "$w/pin-COMET.pem" -o /dev/null -w '%{http_code}' "$(sed -n 's/^@@COMET //p' <<<"$reply" | cut -d' ' -f1)" 2>/dev/null)" || status=none
		if [ "$code" = 200 ] && [ -n "$earliest" ] && [ "$status" = 403 ]; then
			echo "pass comet-location upstream=configured post=200 get=403 earliest=$earliest"
		else
			echo "fail comet-location upstream=configured post=$code get=$status earliest=${earliest:-none}"
			failures=$((failures + 1))
		fi
	fi
	start="$(sed -n 's/^@@start //p' <<<"$reply" | head -n 1)"
	if [ -n "$earliest" ] && [ "$start" = "$earliest" ]; then
		echo "pass start-block block=$start earliest=$earliest"
	else
		echo "fail start-block block=${start:-none} earliest=${earliest:-none}"
		failures=$((failures + 1))
	fi

	if ! python3 -c '
import json,sys
rows=dict(line[2:].split(" ",1) for line in sys.stdin.read().splitlines() if line.startswith("@@") and " " in line)
b,l,bh,lh=rows["cursors"].split()
print(json.dumps(dict(start=int(rows["start"]),retained_start=int(rows["retained-start"]),earliest=int(sys.argv[1]),cutover=int(rows["cutover"]),done=rows.get("done")=="true",backfill=int(b),live=int(l),backfill_hash=bh,live_hash=lh,database=rows["database"])))
' "$earliest" <<<"$reply" >"$w/state.json" || ! bringup_inspect indexer-state <"$w/state.json"; then
		echo "fail backfill chain=paxeer database=inconsistent-or-incomplete"
		failures=$((failures + 1))
	fi

	for method in px_getUnifiedHistory px_getHistory; do
		params="[\"$account\"]"
		code="$(curl -sS -m "$timeout" -o "$w/history.json" -w '%{http_code}' -H 'content-type: application/json' \
			-H @<(printf 'Authorization: Bearer %s\n' "${CHECK_LIVE_GATEWAY_TOKEN:-}") --data "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"$method\",\"params\":$params}" "$gateway_url" 2>/dev/null)" || code=none
		read -r answer line <<<"$(python3 -c '
import json, sys
doc = json.load(open(sys.argv[1]))
result = doc.get("result")
if not isinstance(result, dict):
    error = doc.get("error") or {}
    print("error", ((error.get("data") or {}).get("code") or error.get("message") or "no-result").replace(" ", "-"))
    sys.exit(0)
print(len(result.get("items") or []))
' "$w/history.json" 2>/dev/null)"
		if [ "$code" = 200 ] && [[ "${answer:-}" =~ ^[0-9]+$ ]] && [ "$answer" -gt 0 ]; then
			echo "pass history method=$method account=${params:2:-2} items=$answer"
		else
			echo "fail history method=$method account=${params:2:-2} http=$code answer=${answer:-none}${line:+ $line}"
			failures=$((failures + 1))
		fi
	done
	finish "$failures"
}

# check_developers: current rollout deployments are ready; hooks, api-dev
# and dev answer ready over the edge; a subscription for CHECK_LIVE_DEVELOPERS_RECEIVER
# registers under the session of CHECK_LIVE_DEVELOPERS_TOKEN_FILE and lists
# back; and one producer event is delivered to it within
# CHECK_LIVE_DEVELOPERS_DELIVERY_ATTEMPTS reads five seconds apart. The token
# is read from its configured file and is never printed. One line per check.
check_developers() {
	local hooks=https://hooks.paxeer.network api=https://api-dev.paxeer.network web=https://dev.paxeer.network
	local token_file="${CHECK_LIVE_DEVELOPERS_TOKEN_FILE:-}" receiver="${CHECK_LIVE_DEVELOPERS_RECEIVER:-}"
	local attempts="${CHECK_LIVE_DEVELOPERS_DELIVERY_ATTEMPTS:-36}"
	local name app url want code body w endpoint="" n failures=0
	if ! command -v "$railway_bin" >/dev/null 2>&1; then
		echo "check-live: $railway_bin is required" >&2
		exit 2
	fi
	if [ -z "$token_file" ] || [ ! -r "$token_file" ]; then
		echo "check-live: CHECK_LIVE_DEVELOPERS_TOKEN_FILE does not name a readable file" >&2
		exit 2
	fi
	if [[ "$receiver" != https://?* ]]; then
		echo "check-live: CHECK_LIVE_DEVELOPERS_RECEIVER is not an https URL" >&2
		exit 2
	fi
	if ! [[ "$attempts" =~ ^[1-9][0-9]{0,2}$ ]]; then
		echo "check-live: CHECK_LIVE_DEVELOPERS_DELIVERY_ATTEMPTS is not a count from 1 to 999" >&2
		exit 2
	fi

	for name in webhooks-public webhooks-ingress dashboard dashboard-web; do
		if ! app="$(railway_service "$name")"; then
			echo "fail deployment service=$name rollout=absent"
			failures=$((failures + 1))
			continue
		fi
		railway_deployment "$app" || failures=$((failures + 1))
	done

	w="$(mktemp -d)"
	# shellcheck disable=SC2064
	trap "rm -rf '$w'" EXIT
	for url in "$hooks/healthz" "$api/healthz" "$web/"; do
		code="$(curl -sS --max-time "$timeout" --output "$w/body" --write-out '%{http_code}' "$url" 2>/dev/null)" || code="${code:-none}"
		body="$(cat "$w/body" 2>/dev/null || true)"
		want=200
		if [ "$code" = "$want" ] && { [ "$url" = "$web/" ] || [[ "$body" == *'"ready":true'* ]]; }; then
			echo "pass readiness url=$url http=200"
		else
			echo "fail readiness url=$url http=$code"
			failures=$((failures + 1))
		fi
	done

	local source="${CHECK_LIVE_DEVELOPERS_SOURCE_URL:-}" request="${CHECK_LIVE_DEVELOPERS_SOURCE_REQUEST:-}"
	local source_token="${CHECK_LIVE_DEVELOPERS_SOURCE_TOKEN_FILE:-}"
	if [[ "$source" != https://*/v1/intents/submit ]] || [ ! -s "$request" ] || [ ! -s "$source_token" ]; then
		echo "check-live: authorized CHECK_LIVE_DEVELOPERS_SOURCE_URL (/v1/intents/submit), CHECK_LIVE_DEVELOPERS_SOURCE_REQUEST and CHECK_LIVE_DEVELOPERS_SOURCE_TOKEN_FILE are required" >&2
		exit 2
	fi
	timeout "$((attempts * (timeout * 2 + 5) + timeout * 3))" "${BRINGUP_INSPECT_BIN:-bringup-inspect}" developers-delivery "$hooks" "$receiver" "$source" "$request" "$token_file" "$source_token" "$attempts" || failures=$((failures + 1))

	finish "$failures"
}

# Paid search uses the standing ledger grant and EVM payer address accepted
# by the current sidecar. Metadata is read from the explicit hosted gateway.
check_search() {
	local gateway="${CHECK_LIVE_GATEWAY_RPC:-}" payer="${CHECK_LIVE_SEARCH_PAYER_ADDRESS:-}"
	local grant="${CHECK_LIVE_SEARCH_GRANT_ID:-}" listing name first="" second="" body failures=0
	local -a names=() paths=()
	if [ -z "$gateway" ] || [ -z "$payer" ] || [ -z "$grant" ]; then
		echo "check-live: CHECK_LIVE_GATEWAY_RPC, CHECK_LIVE_SEARCH_PAYER_ADDRESS and CHECK_LIVE_SEARCH_GRANT_ID are required" >&2
		exit 2
	fi
	if ! listing="$("$(dirname "${BASH_SOURCE[0]}")/search-front.sh" names 2>&1)"; then
		echo "fail names serving=unreadable"
		finish 1
	fi
	mapfile -t names < <(sed -n 's/^serve //p' <<<"$listing")
	[ "${#names[@]}" -gt 0 ] || { echo "fail names serving=0"; finish 1; }
	for name in "${names[@]}"; do
		if [ -z "$first" ]; then first="$name"; elif [ -z "$second" ]; then second="$name"; fi
		body="$(curl -fsS --max-time "$timeout" "https://$name/xweb/health" 2>/dev/null)" || body=""
		if python3 -c 'import json,sys; sys.exit(0 if json.load(sys.stdin)["status"]=="ok" else 1)' <<<"$body" 2>/dev/null; then
			echo "pass health https://$name/xweb/health status=ok"
		else
			echo "fail health https://$name/xweb/health"
			failures=$((failures + 1))
		fi
	done
	paths=("$first")
	[ -z "$second" ] || paths+=("$second")
	paths+=(search.paxeer.network)
	for name in "${paths[@]}"; do
		bringup_inspect search-payment "https://$name/search?q=paxeer" "$gateway" "$payer" "$grant" || failures=$((failures + 1))
	done
	finish "$failures"
}

# check_registry: the app of services/platform/hosted/registry/fly.toml runs one
# started machine with its volume and holds a dedicated IPv4 for the edge's
# passthrough; from the kernel machine of services/wallet/deploy/human.toml,
# https://index.paxeer.network/healthz refuses a request without a client
# certificate and answers 200 ready to one with the kernel's internal-CA
# client identity; the hosted gateway's lx_getProgramEvents returns, on the reference
# escrow's event topic, an event of the program CHECK_LIVE_REGISTRY_PROGRAM_ID
# names (the deploy step records it); and the hosted gateway's /readyz reports
# program_registry ready. CHECK_LIVE_ROUTER_URL must name the hosted gateway
# origin for the full check. One line per check.
check_registry() {
	local row target service host_key unit health host command reply status=0
	local program="${CHECK_LIVE_REGISTRY_PROGRAM_ID:-}"
	row="$(bringup_inspect rollout-service "$repo_root/deploy/rollout.kvx" program-registry)" || { echo 'fail registry rollout=absent'; finish 1; }
	read -r target service host_key unit health <<<"$row"
	[ "$target" = box ] || { echo 'fail registry rollout=not-box'; finish 1; }
	host="${!host_key:-}"
	[ -n "$host" ] || { echo "fail registry host=$host_key absent"; finish 1; }
	printf -v command 'systemctl is-active --quiet %q && %q registry-health %q' "$unit" "${BRINGUP_INSPECT_BIN:-bringup-inspect}" "$tls_dir/registry-event-client"
	if [ "${registry_stage:-full}" != bootstrap ]; then
		[[ "$program" =~ ^[0-9a-f]{64}$ ]] || { echo 'fail program-events program=unset want=CHECK_LIVE_REGISTRY_PROGRAM_ID'; finish 1; }
		printf -v command 'systemctl is-active --quiet %q && %q registry-observe %q %q %q' "$unit" "${BRINGUP_INSPECT_BIN:-bringup-inspect}" "$tls_dir/registry-event-client" "$program" "${CHECK_LIVE_REGISTRY_CURSOR_DIR:-/var/lib/layerx/bringup}/registry-$program.cursor"
	fi
	reply="$(ssh_read "$host" "$command")" || status=$?
	[ -z "$reply" ] || printf '%s\n' "$reply"
	if [ "$status" -ne 0 ] || ! grep -q '^pass registry ' <<<"$reply"; then echo 'fail registry observation=refused'; finish 1; fi
	finish 0
}

check_registry_bootstrap() {
	registry_stage=bootstrap
	check_registry
}

# check_registry_plan: prints the four stages of the registry/hosted-gateway bring-up
# in order, each with the stage it requires, its prerequisites and the
# producer of each: material is issued once (ca.sh rows and Fly secrets) and
# reused by every later stage; the registry bootstrap needs only material and
# no hosted gateway gate; gateway activation needs the registry bootstrap;
# the final proof reads a signed registry receipt through that hosted gateway.
# Reads only the repository's tomls; no network, no flyctl.
check_registry_plan() {
	local registry=services/platform/hosted/registry/fly.toml endpoint=services/wallet/deploy/endpoint.toml path
	for path in "$registry" "$endpoint"; do
		if ! grep -qE '^app = "[a-z0-9-]+"$' "$repo_root/$path" 2>/dev/null; then
			echo "fail registry-plan toml=absent $path"
			exit 1
		fi
	done
	if ! grep -q '^  LAYERX_REGISTRY_REQUEST_TOKEN_FILE = ' "$repo_root/$registry" ||
		! grep -q '^  LAYERX_REGISTRY_PUBLICATION_TOKEN_FILE = ' "$repo_root/$registry"; then
		echo "fail registry-plan toml=absent $registry"
		exit 1
	fi
	if ! grep -q '^  LAYERX_GATEWAY_PROGRAM_REGISTRY_URL = ' "$repo_root/$endpoint" ||
		! grep -q '^  LAYERX_GATEWAY_PROGRAM_REGISTRY_TOKEN_FILE = ' "$repo_root/$endpoint"; then
		echo "fail registry-plan toml=absent $endpoint"
		exit 1
	fi
	echo "stage 1 material requires=- needs=request-token,publication-token,gateway-client,registry,registry-event-client,REGISTRY_IDENTITY_TOKEN,REGISTRY_PROGRAM_EVENTS_TOKEN,REGISTRY_WEBHOOKS_EVENTS_TOKEN,LAYERX_REGISTRY_NODE_AUTHORIZATION,LAYERX_REGISTRY_RECEIPT_AUTHORITY_AUTHORIZATION,builder-rootfs,environment-tree-digest,replica-id,trust-history producers=request-token:init.sh:--prepare-material,publication-token:init.sh:--prepare-material,gateway-client:ca.sh:gateway-client,registry:ca.sh:registry,registry-event-client:ca.sh:registry-event-client,REGISTRY_IDENTITY_TOKEN:fly-secret:REGISTRY_IDENTITY_TOKEN,REGISTRY_PROGRAM_EVENTS_TOKEN:fly-secret:REGISTRY_PROGRAM_EVENTS_TOKEN,REGISTRY_WEBHOOKS_EVENTS_TOKEN:fly-secret:REGISTRY_WEBHOOKS_EVENTS_TOKEN,LAYERX_REGISTRY_NODE_AUTHORIZATION:fly-secret:LAYERX_REGISTRY_NODE_AUTHORIZATION,LAYERX_REGISTRY_RECEIPT_AUTHORITY_AUTHORIZATION:fly-secret:LAYERX_REGISTRY_RECEIPT_AUTHORITY_AUTHORIZATION,builder-rootfs:deploy:builder-environment,environment-tree-digest:deploy:builder-environment,replica-id:deploy:kernel-material,trust-history:deploy:kernel-material"
	echo "stage 2 registry-bootstrap requires=material needs=material-record,request-token,publication-token producers=material-record:stage:material,request-token:init.sh:--prepare-material,publication-token:init.sh:--prepare-material"
	echo "stage 3 router-activation requires=registry-bootstrap needs=registry-bootstrap,program-registry-token,client-identity,client-password producers=registry-bootstrap:stage:registry-bootstrap,program-registry-token:fly-secret:ENDPOINT_PROGRAM_REGISTRY_TOKEN,client-identity:fly-secret:ENDPOINT_CLIENT_P12,client-password:fly-secret:ENDPOINT_CLIENT_PASSWORD"
	echo "stage 4 routed-proof requires=router-activation needs=router-activation,registry-receipt producers=router-activation:stage:router-activation,registry-receipt:deploy:routed-proof"
	exit 0
}

# router_prerequisite: router activation requires a passed registry bootstrap,
# protected material and bootstrap records bound to the selected revision,
# image and material generation; refusal precedes every router request.
router_prerequisite() {
	if [ -z "${CHECK_LIVE_STAGE_DIR:-}" ]; then
		echo "check-live: CHECK_LIVE_STAGE_DIR is unset" >&2
		exit 2
	fi
	if ! python3 "$repo_root/tools/qualification/paxeer-x/registry-router-bootstrap.py" \
		--check-stage registry-bootstrap --stage-dir "$CHECK_LIVE_STAGE_DIR" 2>/dev/null; then
		echo "fail router-activation missing=registry-bootstrap producer=stage:registry-bootstrap"
		exit 1
	fi
}

# check_interop_adapters: three conformance legs of the AP2, Visa TAP and fiat
# adapters of the interop gateway app of services/platform/hosted/interop/fly.toml,
# each the make target of the tree followed by the adapter's entry of
# GET <CHECK_LIVE_INTEROP_ORIGIN>/v1/adapters (default
# https://interchain.paxeer.network), whose conformance suite, vector count
# and digest equal the suite services/interop/deploy/gateway/render.py derives from this
# checkout and whose four readiness fields are ready: mandates
# (interop-test-mandates, ap2), visa-tap (interop-test-visa-tap, visa-tap) and
# ramps-sandbox (interop-test-ramps-sandbox, fiat). The sandbox journey reads
# the LAYERX_RAMP_* inputs of services/platform/ramps/sandbox-journey.sh from the
# environment and its two bearer tokens from the files named by
# CHECK_LIVE_INTEROP_ADAPTERS_CUSTOMER_TOKEN_FILE and
# CHECK_LIVE_INTEROP_ADAPTERS_OPERATOR_TOKEN_FILE; no value is printed. One
# line per leg.
check_interop_adapters() {
	local toml=services/platform/hosted/interop/fly.toml name app origin w status http target exits=()
	for name in LAYERX_RAMP_URL LAYERX_RAMP_CA_PEM LAYERX_RAMP_OPERATOR_URL LAYERX_RAMP_ON_QUOTE_ID \
		LAYERX_RAMP_OFF_QUOTE_ID LAYERX_RAMP_OFF_GRANT_JSON LAYERX_RAMP_ON_ACCOUNT_SEQUENCE \
		LAYERX_RAMP_OFF_RECEIVER_SEQUENCE CHECK_LIVE_INTEROP_ADAPTERS_CUSTOMER_TOKEN_FILE \
		CHECK_LIVE_INTEROP_ADAPTERS_OPERATOR_TOKEN_FILE; do
		if [ -z "${!name:-}" ]; then
			echo "check-live: $name is unset" >&2
			exit 2
		fi
	done
	origin="${CHECK_LIVE_INTEROP_ORIGIN:-https://interchain.paxeer.network}"
	if ! app="$(fly_app "$toml")"; then
		echo "fail interop-adapters toml=absent"
		finish 1
	fi
	w="$(mktemp -d)"
	# shellcheck disable=SC2064 # the path expands now, the locals are gone at exit
	trap "rm -rf '$w'" EXIT

	for target in interop-test-mandates interop-test-visa-tap interop-test-ramps-sandbox; do
		status=0
		(
			if [ "$target" = interop-test-ramps-sandbox ]; then
				LAYERX_RAMP_CUSTOMER_TOKEN="$(<"$CHECK_LIVE_INTEROP_ADAPTERS_CUSTOMER_TOKEN_FILE")"
				LAYERX_RAMP_OPERATOR_TOKEN="$(<"$CHECK_LIVE_INTEROP_ADAPTERS_OPERATOR_TOKEN_FILE")"
				export LAYERX_RAMP_CUSTOMER_TOKEN LAYERX_RAMP_OPERATOR_TOKEN
			fi
			make -C "$repo_root" --no-print-directory "$target"
		) >"$w/$target.log" 2>&1 </dev/null || status=$?
		exits+=("$status")
	done
	http="$(curl -sS -m "$timeout" -o "$w/adapters" -w '%{http_code}' "$origin/v1/adapters" 2>/dev/null)" || http=""
	status=0
	python3 - "$repo_root" "$app" "$w/adapters" "${http:-none}" "${exits[@]}" <<'PY' || status=$?
import importlib.util, json, pathlib, sys

root, app, path, http = sys.argv[1:5]
exits = sys.argv[5:]
spec = importlib.util.spec_from_file_location("render", root + "/services/interop/deploy/gateway/render.py")
render = importlib.util.module_from_spec(spec)
spec.loader.exec_module(render)
try:
    served = {entry["id"]: entry for entry in json.load(open(path))["adapters"]}
except Exception:
    served = {}
failed = 0
for (leg, target, adapter), code in zip((("mandates", "interop-test-mandates", "ap2"),
                                         ("visa-tap", "interop-test-visa-tap", "visa-tap"),
                                         ("ramps-sandbox", "interop-test-ramps-sandbox", "fiat")), exits):
    entry = served.get(adapter)
    suite = render.first_party_suite(pathlib.Path(root), adapter)
    if entry is None:
        pins, ready = "absent", 0
    else:
        pins = "match" if suite is not None and (entry.get("conformance_suite"), entry.get("conformance_vectors"),
                                                 entry.get("conformance_sha256")) == suite else "differ"
        ready = sum(1 for value in (entry.get("readiness") or {}).values() if value == "ready")
    ok = code == "0" and http == "200" and pins == "match" and ready == 4
    failed += not ok
    print("%s %s app=%s make=%s exit=%s adapter=%s http=%s pins=%s readiness=%d/4" % (
        "pass" if ok else "fail", leg, app, target, code, adapter, http, pins, ready))
sys.exit(failed)
PY
	finish "$status"
}

check_kernel_node() {
	local home="${CHECK_LIVE_PAXD_HOME:-}" node="${CHECK_LIVE_KERNEL_RPC:-}" comet="${CHECK_LIVE_KERNEL_COMET_RPC:-}"
	local probe answer
	if ! [[ "$home" =~ ^/[a-zA-Z0-9_./-]+$ ]] || [ -z "$node" ] || [ -z "$comet" ]; then
		echo "check-live: CHECK_LIVE_PAXD_HOME, CHECK_LIVE_KERNEL_RPC and CHECK_LIVE_KERNEL_COMET_RPC are required" >&2
		exit 2
	fi
	probe=$(cat <<'PYCODE'
import json,os,stat,sys
from pathlib import Path
home=Path(sys.argv[1]).resolve(strict=True)
found=[]
for process in Path("/proc").iterdir():
    if not process.name.isdecimal(): continue
    try:
        if Path(os.readlink(process/"exe")).name!="paxd": continue
        args=(process/"cmdline").read_bytes().decode().split("\0")
        explicit=args[args.index("--home")+1] if "--home" in args else next((a.split("=",1)[1] for a in args if a.startswith("--home=")),None)
        if explicit is None or Path(explicit).resolve()!=home: continue
        fields=dict(line.split(":",1) for line in (process/"status").read_text().splitlines() if ":" in line)
        if fields["State"].strip().startswith(("Z","X")): continue
        uid=int(fields["Uid"].split()[1])
        mounts=[Path(line.split()[4]) for line in Path("/proc/self/mountinfo").read_text().splitlines()]
        mounted=any(m!=Path("/") and (m==home or m in home.parents) for m in mounts)
        info=home.stat()
        private=stat.S_ISDIR(info.st_mode) and info.st_uid==uid and info.st_mode&0o077==0
        state=home/"data"
        genesis=home/"config/genesis.json"
        state_info=state.lstat(); genesis_info=genesis.lstat()
        found.append(dict(executable="paxd",pid=int(process.name),uid=uid,owner=info.st_uid,home=str(home),mounted=mounted,
            genesis=stat.S_ISREG(genesis_info.st_mode) and genesis_info.st_uid==uid and genesis_info.st_size>0,
            state=private and stat.S_ISDIR(state_info.st_mode) and state_info.st_uid==uid,
            running=True,start_ticks=int((process/"stat").read_text().rsplit(")",1)[1].split()[19]),
            boot_id=Path("/proc/sys/kernel/random/boot_id").read_text().strip()))
    except (OSError,ValueError,KeyError,UnicodeError): continue
if len(found)!=1: raise SystemExit(1)
print(json.dumps(found[0]))
PYCODE
)
	if [ "${CHECK_LIVE_KERNEL_LOCAL:-0}" = 1 ]; then
		answer="$(python3 -I - "$home" <<<"$probe" 2>/dev/null)" || answer=""
	else
		[ -n "${KERNEL_HOST:-}" ] || { echo "fail kernel-node KERNEL_HOST=unset"; finish 1; }
		answer="$(ssh_read "$KERNEL_HOST" "echo $(printf '%s' "$probe" | base64 -w0) | base64 -d | python3 -I - '$home'")" || answer=""
	fi
	if bringup_inspect kernel-node "$node" "$comet" <<<"$answer"; then finish 0; fi
	echo "fail kernel-node process-or-committed-state=unproven"
	finish 1
}

# check_xweb_attestors: each of the four apps of
# services/interop/deploy/x-websearch/attestor-<N>.toml runs one started machine with
# its volume and holds no public IP; inside it, through flyctl ssh console,
# the attestor answers /health on 8480 and every other attestor answers
# /health through its loopback hop 849<M>, and its volume holds the three
# key roles of the toml (/data/keys/attestor.key the web signer,
# submitter.key the EVM submitter, receiver.key the kernel receiver and
# submitter DID) as regular files readable by the owner only, no two of the
# same digest (compared on the machine, never printed); no VALIDATOR_HOSTS
# destination runs an x-websearch unit. One line per check:
#   "pass machines app=<app> machines=1 started=1 volumes=1"
#   "pass public-ips app=<app> count=0"
#   "pass health app=<app> http=200"
#   "pass hop app=<app> peer=<peer app> port=849<M> http=200"
#   "pass keys app=<app> roles=attestor,submitter,receiver private=yes distinct=yes"
#   "pass VALIDATOR_HOSTS[k] x-websearch-units=0"
# with fail and the observed values (ssh=<exit> when a host did not answer).
check_xweb_attestors() {
	local -a apps dests
	local n m answer n_machines n_started n_mounts code status failures=0
	for n in 1 2 3 4; do
		apps[n]="$(fly_app "services/interop/deploy/x-websearch/attestor-$n.toml")" || apps[n]=""
	done
	for n in 1 2 3 4; do
		if [ -z "${apps[n]}" ]; then
			echo "fail attestor-$n toml=absent"
			failures=$((failures + 1))
			continue
		fi
		answer="$(timeout "$timeout" flyctl machines list --app "${apps[n]}" --json 2>/dev/null | python3 -c '
import json, sys
ms = json.load(sys.stdin)
started = [m for m in ms if m.get("state") == "started"]
mounts = [x.get("volume", "") for m in ms for x in (m.get("config") or {}).get("mounts") or []]
print(len(ms), len(started), len(mounts))
' 2>/dev/null)" || answer=""
		read -r n_machines n_started n_mounts <<<"${answer:-none none none}"
		if [ "$n_machines" = 1 ] && [ "$n_started" = 1 ] && [ "$n_mounts" = 1 ]; then
			echo "pass machines app=${apps[n]} machines=1 started=1 volumes=1"
		else
			echo "fail machines app=${apps[n]} machines=$n_machines started=$n_started volumes=$n_mounts"
			failures=$((failures + 1))
		fi
		answer="$(timeout "$timeout" flyctl ips list --app "${apps[n]}" --json 2>/dev/null | python3 -c 'import json, sys; print(len(json.load(sys.stdin) or []))' 2>/dev/null)" || answer=none
		if [ "$answer" = 0 ]; then
			echo "pass public-ips app=${apps[n]} count=0"
		else
			echo "fail public-ips app=${apps[n]} count=${answer:-none}"
			failures=$((failures + 1))
		fi
		# The image carries socat, not curl: one HTTP/1.0 request per port,
		# the status code of the answer's first line.
		answer="$(fly_ssh "${apps[n]}" - "for p in 8480 8491 8492 8493 8494; do printf \"%s \" \$p; printf \"GET /health HTTP/1.0\\r\\nHost: 127.0.0.1\\r\\n\\r\\n\" | timeout $timeout socat -t $timeout - TCP:127.0.0.1:\$p 2>/dev/null | head -n 1 | cut -d\" \" -f2; echo; done")" || answer=""
		code="$(awk '$1 == 8480 { print $2 }' <<<"$answer")"
		if [ "$code" = 200 ]; then
			echo "pass health app=${apps[n]} http=200"
		else
			echo "fail health app=${apps[n]} http=${code:-none}"
			failures=$((failures + 1))
		fi
		for m in 1 2 3 4; do
			[ "$m" -ne "$n" ] || continue
			code="$(awk -v p=$((8490 + m)) '$1 == p { print $2 }' <<<"$answer")"
			if [ "$code" = 200 ]; then
				echo "pass hop app=${apps[n]} peer=${apps[m]:-attestor-$m} port=$((8490 + m)) http=200"
			else
				echo "fail hop app=${apps[n]} peer=${apps[m]:-attestor-$m} port=$((8490 + m)) http=${code:-none}"
				failures=$((failures + 1))
			fi
		done
		answer="$(fly_ssh "${apps[n]}" - "for r in attestor submitter receiver; do f=/data/keys/\$r.key; if [ -f \$f ] && [ ! -L \$f ]; then echo \$r \$(stat -c %a \$f) \$(sha256sum <\$f | cut -c1-64); else echo \$r absent; fi; done")" || answer=""
		answer="$(python3 -c '
import sys
rows = {}
for line in sys.argv[1].splitlines():
    parts = line.split()
    if parts and parts[0] in ("attestor", "submitter", "receiver"):
        rows[parts[0]] = parts[1:]
roles = ("attestor", "submitter", "receiver")
missing = [r for r in roles if len(rows.get(r, [])) != 2]
if missing:
    print("fail roles=%s absent=%s" % (",".join(roles), ",".join(missing)))
    sys.exit()
open_ = [r for r in roles if rows[r][0] not in ("600", "400")]
distinct = len({rows[r][1] for r in roles}) == 3
ok = not open_ and distinct
print("%s roles=%s private=%s distinct=%s" % ("pass" if ok else "fail", ",".join(roles),
      "yes" if not open_ else "no:" + ",".join(open_), "yes" if distinct else "no"))
' "$answer")"
		echo "${answer%% *} keys app=${apps[n]} ${answer#* }"
		[ "${answer%% *}" = pass ] || failures=$((failures + 1))
	done
	read -r -a dests <<<"$VALIDATOR_HOSTS"
	for n in "${!dests[@]}"; do
		status=0
		answer="$(ssh_read "${dests[$n]}" 'systemctl list-units --no-legend --plain --state=active,activating,reloading "x-websearch*" | wc -l')" || status=$?
		if [ "$status" -ne 0 ]; then
			echo "fail VALIDATOR_HOSTS[$n] x-websearch-units ssh=$status"
			failures=$((failures + 1))
		elif [ "$answer" = 0 ]; then
			echo "pass VALIDATOR_HOSTS[$n] x-websearch-units=0"
		else
			echo "fail VALIDATOR_HOSTS[$n] x-websearch-units=${answer:-none}"
			failures=$((failures + 1))
		fi
	done
	finish "$failures"
}

check_kernel_value_loop() {
	local app key value answer status=0 failures=0
	local -a arguments=()
	if ! command -v flyctl >/dev/null 2>&1; then
		echo "check-live: flyctl is required" >&2
		exit 2
	fi
	if ! app="$(fly_app services/wallet/deploy/human.toml)"; then
		echo "fail kernel-value-loop toml=absent"
		finish 1
	fi
	[ -z "${CHECK_LIVE_VALUE_LOOP_DEPOSIT_TX:-}" ] || arguments+=("DEPOSIT_TX=$CHECK_LIVE_VALUE_LOOP_DEPOSIT_TX")
	[ -z "${CHECK_LIVE_VALUE_LOOP_CHECKPOINT_SECONDS:-}" ] || arguments+=("CHECKPOINT_SECONDS=$CHECK_LIVE_VALUE_LOOP_CHECKPOINT_SECONDS")
	if [ -n "${CHECK_LIVE_VALUE_LOOP_ASSET:-}" ]; then
		case "$CHECK_LIVE_VALUE_LOOP_ASSET" in
		PAX|SID|USDC|USDL) arguments+=("ASSET=$CHECK_LIVE_VALUE_LOOP_ASSET") ;;
		*) echo "fail kernel-value-loop asset=invalid"; finish 1 ;;
		esac
	fi
	answer="$(timeout "${CHECK_LIVE_VALUE_LOOP_TIMEOUT:-840}" flyctl ssh console --quiet --app "$app" \
		--command "sh -c 'LAYERX_KERNEL_DATA=/data/layerx/ LAYERX_KERNEL_RUN=/run/layerx/ LAYERX_KERNEL_TLS=/data/tls/ bash -s -- ${arguments[*]}'" \
		<"$repo_root/tools/bringup/value-loop.sh" 2>/dev/null)" || status=$?
	while read -r key value; do
		case "$key" in
		precondition)
			echo "fail precondition $value"
			finish 1
			;;
		asset | account | credit | activity | balance | batch | checkpoint)
			if [ "$status" -ne 1 ] || [ "$key $value" != "$(tail -n 1 <<<"$answer")" ]; then
				echo "pass $key $value"
			else
				echo "fail $key $value"
				failures=$((failures + 1))
			fi
			;;
		esac
	done <<<"$answer"
	if [ "$status" -ne 0 ] && [ "$failures" -eq 0 ]; then
		echo "fail kernel-value-loop app=$app exit=$status"
		failures=1
	fi
	if [ "$status" -eq 0 ] && ! grep -q '^checkpoint ' <<<"$answer"; then
		echo "fail checkpoint status=absent"
		failures=1
	fi
	finish "$failures"
}

# Event source roles and the ingress trigger follow deploy/rollout.kvx.
# Producer credentials stay on their box or selected running role replica.
# Select every running replica of a declared role; a missing or stale
# deployment never turns into an unselected SSH probe.
role_instances() {
	railway_instance "$1" | bringup_inspect role-instances
}

check_events_upstream() {
	local app instances instance kind executable row target service host_key unit health host command reply status failures=0
	for row in 'journey KERNEL_HOST layerx-human' 'approval KERNEL_HOST layerx-human' 'program program-registry layerx-program-registry' 'payment router layerx-gateway'; do
		read -r kind service executable <<<"$row"
		printf -v command '%q event-handoff %q %q' "${BRINGUP_INSPECT_BIN:-bringup-inspect}" "$kind" "$executable"
		if [ "$service" = KERNEL_HOST ]; then
			host="${KERNEL_HOST:-}"
			[ -n "$host" ] || { echo "fail producer-handoff kind=$kind host=absent"; failures=$((failures + 1)); continue; }
			status=0
			reply="$(ssh_read "$host" "$command")" || status=$?
		else
			row="$(bringup_inspect rollout-service "$repo_root/deploy/rollout.kvx" "$service")" || { echo "fail producer-handoff kind=$kind rollout=absent"; failures=$((failures + 1)); continue; }
			read -r target app host_key unit health <<<"$row"
			if [ "$target" = box ]; then
				host="${!host_key:-}"
				[ -n "$host" ] || { echo "fail producer-handoff kind=$kind host=$host_key absent"; failures=$((failures + 1)); continue; }
				status=0
				reply="$(ssh_read "$host" "$command")" || status=$?
			else
				instances="$(role_instances "$app")" || { echo "fail producer-handoff kind=$kind deployment=unreadable"; failures=$((failures + 1)); continue; }
				while read -r instance; do
					status=0
					reply="$(railway_ssh "$app" "$command" "$instance" </dev/null)" || status=$?
					if [ "$status" -eq 0 ] && grep -q "^pass producer-handoff kind=$kind " <<<"$reply"; then printf '%s instance=%s\n' "$reply" "$instance"; else echo "fail producer-handoff kind=$kind instance=$instance"; failures=$((failures + 1)); fi
				done <<<"$instances"
				continue
			fi
		fi
		if [ "$status" -eq 0 ] && grep -q "^pass producer-handoff kind=$kind " <<<"$reply"; then printf '%s\n' "$reply"; else echo "fail producer-handoff kind=$kind"; failures=$((failures + 1)); fi
	done
	for row in 'journey internal-journeys' 'approval internal-approvals' 'payment internal-payments' 'program internal-programs' 'trigger webhooks-ingress'; do
		read -r kind service <<<"$row"
		app="$(railway_service "$service")" || { echo "fail event-role service=$service rollout=absent"; failures=$((failures + 1)); continue; }
		instances="$(role_instances "$app")" || { echo "fail event-role service=$service deployment=unreadable"; failures=$((failures + 1)); continue; }
		if [ "$kind" = trigger ]; then printf -v command '%q event-trigger' "${BRINGUP_INSPECT_BIN:-bringup-inspect}"; else printf -v command '%q event-role %q' "${BRINGUP_INSPECT_BIN:-bringup-inspect}" "$kind"; fi
		while read -r instance; do
			status=0
			reply="$(railway_ssh "$app" "$command" "$instance" </dev/null)" || status=$?
			if [ "$status" -eq 0 ] && grep -q '^pass event-' <<<"$reply"; then printf '%s service=%s instance=%s\n' "$reply" "$app" "$instance"; else echo "fail event-role service=$app instance=$instance"; failures=$((failures + 1)); fi
		done <<<"$instances"
	done
	finish "$failures"
}

mode="${1:-}"
case "$mode" in
-h | --help)
	usage
	exit 0
	;;
hosts | rpc-nodes | archive-node | ca | hpx | explorer) ;;
rpc-placement) ;;
validators) ;;
identity) ;;
kernel-app) ;;
kernel-node) ;;
kernel-value-loop) ;;
search-front) ;;
edge) ;;
ci) ;;
human-session) ;;
paxeer-boundary) ;;
agent-public) ;;
gas) ;;
internal) ;;
bridge) ;;
wallet) ;;
mirrors) ;;
relay) ;;
interop) ;;
ramp) ;;
indexer) ;;
developers) ;;
router) ;;
search) ;;
registry) ;;
registry-bootstrap) ;;
registry-plan) ;;
interop-adapters) ;;
xweb-attestors) ;;
kernel-boundaries) ;;
events-upstream) ;;
human) ;;
*)
	usage >&2
	exit 2
	;;
esac

if [ "$#" -ne 1 ]; then
	usage >&2
	exit 2
fi

tools=(ssh timeout curl python3 openssl sha256sum)
case "$mode" in
rpc-nodes | rpc-placement | archive-node | explorer | paxeer-boundary | gas | wallet | router | ca | human | kernel-node | relay | interop | indexer | developers | search | edge | ramp | internal | registry | registry-bootstrap | events-upstream) tools+=("${BRINGUP_INSPECT_BIN:-bringup-inspect}") ;;
esac
[ "$mode" != explorer ] || tools=(curl python3 psql)
case "$mode" in
ca | identity | search-front | internal | wallet | router | ramp | indexer | developers | interop | events-upstream) tools+=("$railway_bin") ;;
esac
[ "$mode" != gas ] || tools+=("$railway_bin" cast)
[ "$mode" != relay ] || tools=(timeout "${BRINGUP_INSPECT_BIN:-bringup-inspect}")
[ "$mode" != interop-adapters ] || tools+=(make)
[ "$mode" != registry-plan ] || tools=(grep)
if [ "$mode" = kernel-node ] && [ "${CHECK_LIVE_KERNEL_LOCAL:-0}" = 1 ]; then tools=(timeout python3 "${BRINGUP_INSPECT_BIN:-bringup-inspect}"); fi
for tool in "${tools[@]}"; do
	if ! command -v "$tool" >/dev/null 2>&1; then
		echo "check-live: $tool is required" >&2
		exit 2
	fi
done

case "$mode" in
explorer | registry-plan) ;;
relay) ;;
kernel-node) [ "${CHECK_LIVE_KERNEL_LOCAL:-0}" = 1 ] || load_hosts ;;
*) load_hosts ;;
esac
"check_${mode//-/_}"

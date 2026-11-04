#!/usr/bin/env bash
set -euo pipefail

usage() {
	cat <<'EOF'
usage: tools/bringup/install-archive.sh <public-rpc-name> <acme-email>

Runs as root on the archive host and brings it to the archive state without
touching what is already right: paxd installed through hpx as a full node at
the release the mirror publishes, min-retain-blocks and ss-keep-recent at 0 so
the node keeps every block, receipt and state version from its first synced
block on, and nginx serving the node's JSON-RPC and WebSocket at the public
RPC name over a Let's Encrypt certificate, like the other published RPC
hosts. Every step is skipped when its result is already in place; only a
changed app.toml restarts paxd, only a new vhost reloads nginx, and only a
paxd that differs from the published release is updated. A fresh node syncs
from the snapshot the mirror's trust anchor admits; the explorer's history
copy covers the blocks before it.

From the operator host, with the host map loaded:
  ssh -- "$ARCHIVE_HOST" BRINGUP_RPC_INVENTORY=/usr/local/share/paxeer/public-rpc.md bash -s -- <public-rpc-name> <acme-email> \
    <tools/bringup/install-archive.sh

Arguments:
  public-rpc-name   a numbered node from the published public RPC inventory
  acme-email        the Let's Encrypt account address, used only when the
                    name has no certificate yet

Environment:
  BRINGUP_RPC_INVENTORY  installed public-rpc.md inventory (required for stdin SSH installs)
  HPX_MIRROR   the hpx mirror, default https://node.hyperpaxeer.com
  HPX_HOME     the node home, default /root/.paxeer

Exits 1 when a step fails, 2 on a usage error.
EOF
}

case "${1:-}" in
-h | --help)
	usage
	exit 0
	;;
esac

if [ "$#" -ne 2 ]; then
	usage >&2
	exit 2
fi

name="$1"
email="$2"

inspector="${BRINGUP_INSPECT_BIN:-bringup-inspect}"
inventory="${BRINGUP_RPC_INVENTORY:-$(dirname "${BASH_SOURCE[0]:-$0}")/../../docs/site/docs/reference/public-rpc.md}"
if ! "$inspector" archive-name "$inventory" "$name" || [[ "$email" != *@* ]]; then
	usage >&2
	exit 2
fi

if [ "$(id -u)" -ne 0 ]; then
	echo "install-archive: must run as root" >&2
	exit 1
fi

mirror="${HPX_MIRROR:-https://node.hyperpaxeer.com}"
home="${HPX_HOME:-/root/.paxeer}"
app="$home/config/app.toml"
webroot=/var/www/certbot

say() {
	echo "install-archive: $*"
}

# packages: curl and jq for hpx, nginx and certbot for the public name.
missing=()
for tool in curl jq nginx certbot; do
	command -v "$tool" >/dev/null 2>&1 || missing+=("$tool")
done
if [ "${#missing[@]}" -ne 0 ]; then
	DEBIAN_FRONTEND=noninteractive apt-get update -qq
	DEBIAN_FRONTEND=noninteractive apt-get install -y -qq "${missing[@]}"
	say "installed ${missing[*]}"
fi

# hpx: the CLI verified against the mirror's manifest, as get-hpx.sh does.
if ! command -v hpx >/dev/null 2>&1; then
	want="$(curl -fsSL --retry 5 --max-time 60 "$mirror/checksums.txt" | awk '$2 == "hpx" { print $1; exit }')"
	curl -fsSL --retry 5 --max-time 60 "$mirror/hpx" -o /usr/local/bin/hpx.new
	got="$(sha256sum /usr/local/bin/hpx.new | awk '{print $1}')"
	if [ -z "$want" ] || [ "$got" != "$want" ]; then
		rm -f /usr/local/bin/hpx.new
		echo "install-archive: hpx sha256 mismatch against $mirror/checksums.txt" >&2
		exit 1
	fi
	chmod +x /usr/local/bin/hpx.new
	mv -f /usr/local/bin/hpx.new /usr/local/bin/hpx
	say "installed hpx"
fi

# node: a full node through hpx when none is here; its prompts take their
# defaults from an empty stdin.
if [ ! -f "$home/config/config.toml" ] || [ ! -f /etc/systemd/system/paxd.service ]; then
	HPX_TYPE=fullnode HPX_HOME="$home" HPX_MIRROR="$mirror" hpx setup </dev/null
	say "installed paxd full node"
fi

# retention: both keys at 0 keep every block, receipt and state version.
restart=0
for key in min-retain-blocks ss-keep-recent; do
	current="$(sed -n "s/^$key = //p" "$app")"
	case "$current" in
	0) ;;
	"")
		echo "install-archive: $app has no $key line" >&2
		exit 1
		;;
	*)
		sed -i "s/^$key = .*/$key = 0/" "$app"
		say "$key $current -> 0"
		restart=1
		;;
	esac
done
if [ "$restart" -eq 1 ]; then
	systemctl restart paxd
	say "restarted paxd"
fi
systemctl is-enabled --quiet paxd || systemctl enable paxd
systemctl is-active --quiet paxd || systemctl start paxd

# Progress is reconciled from the certificate and rendered state on every run.
# The lock stays outside the replaceable vhost and spans issuance and reload.
exec 8>/etc/nginx/.archive-operation.lock
flock -x 8
mkdir -p "$webroot"
archive_reconcile() {
	"$inspector" archive-reconcile "$inventory" /etc/nginx /etc/letsencrypt/live "$webroot" "$name" v1
}
archive_reload() {
	nginx -t
	systemctl enable --now nginx
	systemctl reload nginx
}
state="$(archive_reconcile)"
case "$state" in *' changed') archive_reload ;; esac
if [[ "$state" = certificate* ]]; then
	certbot certonly --webroot -w "$webroot" -d "$name" -m "$email" --agree-tos --non-interactive --keep-until-expiring
	state="$(archive_reconcile)"
	[[ "$state" = tls* ]] || { echo 'install-archive: ACME did not install a valid certificate/key' >&2; exit 1; }
fi
case "$state" in *' changed') archive_reload ;; esac
if ! "$inspector" archive-health "https://$name"; then
	archive_reload
	"$inspector" archive-health "https://$name"
fi
say "serving $name over HTTPS"

# release: paxd at the sha256 the mirror publishes, updated through hpx.
published="$(curl -fsS --max-time 15 "$mirror/chain-info.json" | jq -r '.paxd_sha256 // empty')"
if [ -z "$published" ]; then
	echo "install-archive: $mirror/chain-info.json names no paxd_sha256" >&2
	exit 1
fi
if [ "$published" != "$(sha256sum /usr/local/bin/paxd | awk '{print $1}')" ]; then
	HPX_HOME="$home" HPX_MIRROR="$mirror" hpx update
	say "updated paxd to the published release"
fi

say "paxd=$(systemctl is-active paxd) nginx=$(systemctl is-active nginx) health=$(curl -fsS --max-time 10 "https://$name/health")"

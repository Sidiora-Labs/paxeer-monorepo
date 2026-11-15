#!/bin/sh
set -eu
umask 077

[ "$#" -eq 1 ] || { echo "usage: indexer-init backfill | serve" >&2; exit 2; }
case "$1" in
backfill)
	: "${LAYERX_INDEXER_PAXSCAN_CERT_SHA256:?Paxscan TLS certificate pin is required}"
	exec /usr/local/bin/indexer-env-init backfill
	;;
serve) ;;
*) echo "usage: indexer-init backfill | serve" >&2; exit 2 ;;
esac

mkdir -p /run/indexer-health
/usr/local/bin/indexer-health-config >/run/indexer-health/nginx.conf
openssl x509 -inform DER -in "${LAYERX_INDEXER_HEALTH_CA_DER:?Indexer TLS CA is required}" \
	-out /run/indexer-health/ca.pem
chown -R 4020:4020 /run/indexer-health

indexer_pid=
health_pid=
cleanup() {
	[ -z "$indexer_pid" ] || kill "$indexer_pid" 2>/dev/null || true
	[ -z "$health_pid" ] || kill "$health_pid" 2>/dev/null || true
	[ -z "$indexer_pid" ] || wait "$indexer_pid" 2>/dev/null || true
	[ -z "$health_pid" ] || wait "$health_pid" 2>/dev/null || true
}
trap cleanup EXIT
trap 'exit 143' TERM
trap 'exit 130' INT
trap 'exit 129' HUP

state_dir=$(dirname "$LAYERX_INDEXER_DB")
if [ ! -e "$state_dir/backfill-done" ]; then
	: "${LAYERX_INDEXER_PAXSCAN_CERT_SHA256:?Paxscan TLS certificate pin is required}"
	/usr/local/bin/indexer-env-init backfill &
	indexer_pid=$!
	wait "$indexer_pid"
	indexer_pid=
fi

/usr/local/bin/indexer-env-init serve &
indexer_pid=$!
setpriv --reuid=4020 --regid=4020 --clear-groups --no-new-privs \
	nginx -p /run/indexer-health/ -c /run/indexer-health/nginx.conf -g 'daemon off;' &
health_pid=$!
wait -n "$indexer_pid" "$health_pid"

#!/bin/sh
set -eu

fail() {
	echo "layerx-redis: $*" >&2
	exit 1
}

case "${LAYERX_ROLE:-}" in
redis-router | redis-internal) ;;
*) fail "LAYERX_ROLE must be redis-router or redis-internal, got '${LAYERX_ROLE:-}'" ;;
esac

maxmemory=${REDIS_MAXMEMORY:-0}
case "$maxmemory" in
'' | *[!0-9a-zA-Z]* | [!0-9]*) fail "REDIS_MAXMEMORY '$maxmemory' is not a Redis memory size" ;;
esac

render() {
	cat <<CONF
include /run/secrets/redis.conf
bind * -::*
port 0
tls-port 6379
tls-cert-file /run/secrets/redis.crt
tls-key-file /run/secrets/redis.key
tls-ca-cert-file /run/secrets/ca.crt
tls-auth-clients no
aclfile /run/layerx/users.acl
appendonly yes
appendfsync always
dir /data
protected-mode yes
maxmemory $maxmemory
maxmemory-policy noeviction
CONF
}

render_acl() {
	awk '
		BEGIN { target = "user default off ~* &* +@all" }
		$1 == "user" && $2 == "default" {
			has_default = 1
			for (i = 3; i <= NF; i++) {
				if ($i == "off") {
					$0 = target
					break
				}
			}
		}
		{ lines[NR] = $0 }
		END {
			if (!has_default) print target
			for (n = 1; n <= NR; n++) print lines[n]
		}
	'
}

if [ "${1:-}" = render ]; then
	render
	exit 0
fi

if [ "${1:-}" = render-acl ]; then
	render_acl
	exit 0
fi

conf=/run/layerx/redis.conf
mkdir -p /run/layerx
acl_tmp=$(mktemp /run/layerx/.users.acl.XXXXXX)
trap 'rm -f "$acl_tmp"' 0
chmod 0600 "$acl_tmp"
chown "$(stat -c '%u:%g' /run/secrets/users.acl)" "$acl_tmp"
render_acl </run/secrets/users.acl >"$acl_tmp"
mv -f "$acl_tmp" /run/layerx/users.acl
trap - 0
render >"$conf.tmp"
chmod 0644 "$conf.tmp"
mv -f "$conf.tmp" "$conf"
exec docker-entrypoint.sh redis-server "$conf"

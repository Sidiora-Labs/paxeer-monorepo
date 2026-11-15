#!/bin/sh
set -eu

if [ "$#" -ne 0 ]; then
	[ "$#" -eq 1 ] && [ "$1" = backfill ] || {
		echo "usage: indexer-health-config [backfill]" >&2
		exit 2
	}
fi

: "${PORT:?HTTP health port is required}"

port() {
	case "$1" in
	'' | *[!0-9]* | 0*) return 1 ;;
	esac
	[ "${#1}" -le 5 ] && [ "$1" -le 65535 ]
}

port "$PORT" || { echo "indexer: invalid HTTP health port" >&2; exit 2; }

if [ "$#" -eq 1 ]; then
	cat <<EOF
pid /run/indexer-health/nginx.pid;
error_log /dev/stderr warn;
events { worker_connections 32; }
http {
	access_log off;
	client_body_temp_path /run/indexer-health/client;
	proxy_temp_path /run/indexer-health/proxy;
	fastcgi_temp_path /run/indexer-health/fastcgi;
	uwsgi_temp_path /run/indexer-health/uwsgi;
	scgi_temp_path /run/indexer-health/scgi;
	server {
		listen [::]:${PORT} ipv6only=off;
		location = /healthz {
			default_type text/plain;
			return 200 "backfill";
		}
		location / { return 404; }
	}
}
EOF
	exit 0
fi

: "${LAYERX_INDEXER_LISTEN:?TLS listener is required}"
: "${LAYERX_INDEXER_HEALTH_SERVER_NAME:?TLS health server name is required}"

case "$LAYERX_INDEXER_LISTEN" in
\[::\]:*) tls_port=${LAYERX_INDEXER_LISTEN##*:} ;;
*) echo "indexer: TLS listener must use all IPv6 interfaces" >&2; exit 2 ;;
esac
port "$tls_port" || { echo "indexer: invalid TLS port" >&2; exit 2; }
[ "$PORT" != "$tls_port" ] || { echo "indexer: HTTP and TLS ports must differ" >&2; exit 2; }
case "$LAYERX_INDEXER_HEALTH_SERVER_NAME" in
*[!a-zA-Z0-9.-]* | .* | *.) echo "indexer: invalid TLS health server name" >&2; exit 2 ;;
esac

cat <<EOF
pid /run/indexer-health/nginx.pid;
error_log /dev/stderr warn;
events { worker_connections 32; }
http {
	access_log off;
	client_body_temp_path /run/indexer-health/client;
	proxy_temp_path /run/indexer-health/proxy;
	fastcgi_temp_path /run/indexer-health/fastcgi;
	uwsgi_temp_path /run/indexer-health/uwsgi;
	scgi_temp_path /run/indexer-health/scgi;
	server {
		listen [::]:${PORT} ipv6only=off;
		location = /healthz {
			proxy_pass https://127.0.0.1:${tls_port};
			proxy_http_version 1.1;
			proxy_ssl_trusted_certificate /run/indexer-health/ca.pem;
			proxy_ssl_verify on;
			proxy_ssl_server_name on;
			proxy_ssl_name ${LAYERX_INDEXER_HEALTH_SERVER_NAME};
			proxy_connect_timeout 5s;
			proxy_read_timeout 5s;
		}
		location / { return 404; }
	}
}
EOF

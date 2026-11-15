#!/bin/sh
set -eu
umask 077

export LAYERX_AGENT_BOUNDARY_TLS_CERT_DER=/data/tls/agent-boundary/cert.der
export LAYERX_AGENT_BOUNDARY_TLS_KEY_DER=/data/tls/agent-boundary/key.der
export LAYERX_AGENT_BOUNDARY_GATEWAY_TOKEN_FILE=/run/layerx/agent-boundary/gateway-token
export LAYERX_AGENT_BOUNDARY_REGISTRY_TOKEN_FILE=/run/layerx/agent-boundary/registry-token
export LAYERX_AGENT_BOUNDARY_WEBHOOK_TOKEN_FILE=/run/layerx/agent-boundary/webhook-token
export LAYERX_AGENT_BOUNDARY_RECEIPT_KEYS_FILE=/data/agent-boundary/receipt-keys.json

port=${PORT:-9446}
case "$port" in
	*[!0-9]* | '' | 0) echo "agent-boundary-env-init: PORT must be a positive decimal port" >&2; exit 1 ;;
esac
if [ "$port" -gt 65535 ]; then
	echo "agent-boundary-env-init: PORT is outside the TCP port range" >&2
	exit 1
fi
export LAYERX_AGENT_BOUNDARY_LISTEN="0.0.0.0:$port"

for file in \
	"$LAYERX_AGENT_BOUNDARY_TLS_CERT_DER" \
	"$LAYERX_AGENT_BOUNDARY_TLS_KEY_DER" \
	"$LAYERX_AGENT_BOUNDARY_GATEWAY_TOKEN_FILE" \
	"$LAYERX_AGENT_BOUNDARY_REGISTRY_TOKEN_FILE" \
	"$LAYERX_AGENT_BOUNDARY_WEBHOOK_TOKEN_FILE" \
	"$LAYERX_AGENT_BOUNDARY_RECEIPT_KEYS_FILE"; do
	[ -s "$file" ] || { echo "agent-boundary-env-init: required file is missing or empty: $file" >&2; exit 1; }
done

chown -R 4020:4020 /data/tls/agent-boundary /data/agent-boundary /run/layerx/agent-boundary
chmod 0711 /data/tls /data
exec setpriv --reuid=4020 --regid=4020 --clear-groups --no-new-privs /usr/local/bin/layerx-agent-boundary

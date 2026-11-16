#!/bin/sh
# Init of one webhooks service on Railway (services/platform/hosted/webhooks/railway.env.example),
# run as root after layerx-env-files has written the role's files under /run/layerx.
# LAYERX_ROLE selects the role:
# public serves only the developer API, either with private TLS on [::]:9444
# behind the unified endpoint or as plain HTTP on the front's listener, and
# reads only the developer identity token of the role credentials.
# ingress serves only the internal producer and operator routes, TLS on
# LAYERX_WEBHOOKS_LISTEN (default [::]:9443) with the certificate of the developer
# row of tools/bringup/ca.sh, requiring an internal-CA client leaf with a webhook
# role URI SAN, and reads the trigger, operator, source, receipt and sequencer
# credentials. The TLS form answers GET /healthz on LAYERX_HEALTH_ADDR, else on
# [::]:$PORT when PORT is set, else on [::]:9442 (public) or [::]:8080 (ingress);
# the plain public form answers it on the application listener. Each replica is
# its own delivery instance, named by RAILWAY_REPLICA_ID or the hostname, and
# the service runs as uid 65532.
set -eu
[ "$#" -eq 0 ] || {
	echo "usage: LAYERX_ROLE=public|ingress webhooks-env-init" >&2
	exit 2
}
LAYERX_WEBHOOKS_INSTANCE_ID=${RAILWAY_REPLICA_ID:-$(hostname)}
[ -n "$LAYERX_WEBHOOKS_INSTANCE_ID" ] || {
	echo "webhooks-env-init: neither RAILWAY_REPLICA_ID nor the hostname names this instance" >&2
	exit 2
}
export LAYERX_WEBHOOKS_INSTANCE_ID
health() {
	if [ -n "${LAYERX_HEALTH_ADDR:-}" ]; then
		echo "$LAYERX_HEALTH_ADDR"
	elif [ -n "${PORT:-}" ]; then
		echo "[::]:$PORT"
	else
		echo "$1"
	fi
}
case "${LAYERX_ROLE:-}" in
public)
	case "${LAYERX_WEBHOOKS_LISTENER:-tls}" in
	tls)
		export LAYERX_WEBHOOKS_ROLE=public LAYERX_WEBHOOKS_LISTENER=tls LAYERX_WEBHOOKS_LISTEN="${LAYERX_WEBHOOKS_LISTEN:-[::]:9444}" \
			LAYERX_WEBHOOKS_HEALTH_LISTEN="$(health "[::]:9442")" \
			LAYERX_WEBHOOKS_TLS_CERT_DER=/run/layerx/tls/server.der \
			LAYERX_WEBHOOKS_TLS_KEY_DER=/run/layerx/tls/server-key.der \
			LAYERX_WEBHOOKS_IDENTITY_TOKEN_FILE=/run/layerx/tokens/identity
		;;
	plain)
		if [ -n "${LAYERX_WEBHOOKS_LISTEN:-}" ]; then
			listen="$LAYERX_WEBHOOKS_LISTEN"
		elif [ -n "${PORT:-}" ]; then
			listen="[::]:$PORT"
		else
			listen="[::]:9444"
		fi
		unset LAYERX_HEALTH_ADDR LAYERX_WEBHOOKS_HEALTH_LISTEN LAYERX_WEBHOOKS_TLS_CERT_DER LAYERX_WEBHOOKS_TLS_KEY_DER
		export LAYERX_WEBHOOKS_ROLE=public LAYERX_WEBHOOKS_LISTENER=plain LAYERX_WEBHOOKS_LISTEN="$listen" \
			LAYERX_WEBHOOKS_IDENTITY_TOKEN_FILE=/run/layerx/tokens/identity
		;;
	*)
		echo "webhooks-env-init: LAYERX_WEBHOOKS_LISTENER must be tls or plain" >&2
		exit 2
		;;
	esac
	;;
ingress)
	export LAYERX_WEBHOOKS_ROLE=ingress LAYERX_WEBHOOKS_LISTENER=tls LAYERX_WEBHOOKS_LISTEN="${LAYERX_WEBHOOKS_LISTEN:-[::]:9443}" \
		LAYERX_WEBHOOKS_HEALTH_LISTEN="$(health "[::]:8080")" \
		LAYERX_WEBHOOKS_TLS_CERT_DER=/run/layerx/tls/server.der \
		LAYERX_WEBHOOKS_TLS_KEY_DER=/run/layerx/tls/server-key.der \
		LAYERX_WEBHOOKS_INGRESS_CLIENT_CA_DER=/run/layerx/ca/internal.der \
		LAYERX_WEBHOOKS_COMPONENT_TOKEN_FILE=/run/layerx/tokens/component \
		LAYERX_WEBHOOKS_AUTHORITY_TOKEN_FILE=/run/layerx/tokens/authority \
		LAYERX_WEBHOOKS_JOURNEY_SOURCE_TOKEN_FILE=/run/layerx/tokens/journey \
		LAYERX_WEBHOOKS_PAYMENT_SOURCE_TOKEN_FILE=/run/layerx/tokens/payment \
		LAYERX_WEBHOOKS_APPROVAL_SOURCE_TOKEN_FILE=/run/layerx/tokens/approval \
		LAYERX_WEBHOOKS_PROGRAM_SOURCE_TOKEN_FILE=/run/layerx/tokens/program \
		LAYERX_WEBHOOKS_SOURCE_TRIGGER_TOKEN_FILE=/run/layerx/tokens/source-trigger \
		LAYERX_WEBHOOKS_OPERATOR_TOKEN_FILE=/run/layerx/tokens/operator \
		LAYERX_WEBHOOKS_SEQUENCER_PUBLIC_KEY_FILE=/run/layerx/keys/sequencer \
		LAYERX_WEBHOOKS_SEQUENCER_ID_FILE=/run/layerx/keys/sequencer-id \
		LAYERX_WEBHOOKS_SEQUENCER_FIRST_BATCH_FILE=/run/layerx/keys/sequencer-first-batch \
		LAYERX_WEBHOOKS_SEQUENCER_LAST_BATCH_FILE=/run/layerx/keys/sequencer-last-batch
	;;
*)
	echo "webhooks-env-init: LAYERX_ROLE must be public or ingress" >&2
	exit 2
	;;
esac
exec setpriv --reuid=65532 --regid=65532 --clear-groups --no-new-privs /usr/local/bin/layerx-webhooks

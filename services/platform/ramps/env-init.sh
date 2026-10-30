#!/bin/sh
# Init of the Railway service ramp (services/platform/ramps/railway.env.example). Runs
# as root on the service's one replica, as the materialize-private-files init
# container of services/platform/ramps/deployment.yaml does: layerx-env-files has
# written the files.tsv secrets to /run/ramp-source; this copies them to
# /run/secrets as uid 4020 with mode 0600, renders the ramp config from the
# service's variables and those files to /run/layerx-ramp/config/config.json,
# hands the journal directory on the volume to uid 4020 and starts the ramp
# under that uid on the plain listener. No secret value is printed.
set -eu
umask 077
source_dir=/run/ramp-source
secrets=/run/secrets
config_dir=/run/layerx-ramp/config
journal_dir=/data/ramp

need() {
	value=""
	eval "value=\${$1:-}"
	if ! printf '%s' "$value" | grep -Eqx "$2"; then
		echo "ramp-env-init: $1 is unset or malformed" >&2
		exit 2
	fi
}
uint='[0-9]+'
hex32='[0-9a-fA-F]{64}'
url='https://[^ "\\]+'
text='[^"\\]+'
need RAMP_LISTEN '\[::\]:[0-9]+'
need RAMP_WORKER_ID "$text"
need RAMP_LEASE_SECONDS "$uint"
need RAMP_RECONCILE_SECONDS "$uint"
need RAMP_CLIENT_TIMEOUT_SECONDS "$uint"
need RAMP_IDENTITY_ENDPOINT "$url"
need RAMP_IDENTITY_AUDIENCE "$text"
need RAMP_SEQUENCER_RPC_ENDPOINT "$url"
need RAMP_RPC_CHAIN_ID '[1-9][0-9]*'
need RAMP_OPERATOR_ADDRESS '0x[0-9a-fA-F]{40}'
need RAMP_MAX_FEE_PER_GAS "$uint"
need RAMP_MAX_PRIORITY_FEE_PER_GAS "$uint"
need RAMP_GAS_LIMIT_CAP '[1-9][0-9]*'
need RAMP_POLL_CADENCE_SECONDS "$uint"
need RAMP_OPERATOR_PRINCIPAL_ID "$text"
need RAMP_ACTOR_DID 'did:layerx:[^ "\\:]+'
need RAMP_LAYERX_SIGNER_KEY_HANDLE "$text"
need RAMP_PAXEER_SIGNER_KEY_HANDLE "$text"
need RAMP_COMPLIANCE_ENDPOINT "$url"
need RAMP_COMPLIANCE_PUBLIC_KEY "$hex32"
need RAMP_PROVIDER_ENDPOINT "$url"
need RAMP_PROVIDER_CALLBACK_PUBLIC_KEY "$hex32"
need RAMP_CUSTODY_ENDPOINT "$url"
need RAMP_WALLET_ADDRESS "$text"
need RAMP_VAULT_ID "$text"
need RAMP_SIGNER_ENDPOINT "$url"
if ! printf '%s' "${RAMP_RECEIPT_KEYS_JSON:-}" | jq -e \
	'type == "array" and length > 0 and all(.[]; type == "object")' >/dev/null 2>&1; then
	echo "ramp-env-init: RAMP_RECEIPT_KEYS_JSON is unset or malformed" >&2
	exit 2
fi

for f in quotes.json outbound-ca.pem outbound-identity.p12 outbound-identity-password paxeer-rpc-ca.der \
	identity-token compliance-token provider-token kms-token \
	paxeer-custody-token operator-control-token; do
	if [ ! -s "$source_dir/$f" ]; then
		echo "ramp-env-init: $source_dir/$f is missing or empty" >&2
		exit 2
	fi
done
mkdir -p "$secrets" "$config_dir" "$journal_dir"
for f in outbound-ca.pem outbound-identity.p12 outbound-identity-password paxeer-rpc-ca.der identity-token \
	compliance-token provider-token kms-token paxeer-custody-token \
	operator-control-token; do
	cp "$source_dir/$f" "$secrets/$f"
done

jq -n --slurpfile quotes "$source_dir/quotes.json" --arg s "$secrets" \
	--arg journal "$journal_dir/journal.jsonl" '{
	listen: env.RAMP_LISTEN,
	listener: "plain",
	journal_path: $journal,
	worker_id: env.RAMP_WORKER_ID,
	lease_seconds: (env.RAMP_LEASE_SECONDS | tonumber),
	reconcile_seconds: (env.RAMP_RECONCILE_SECONDS | tonumber),
	operator: {
		principal_id: env.RAMP_OPERATOR_PRINCIPAL_ID,
		account: ("agent:" + env.RAMP_ACTOR_DID + ":main"),
		signer_key_handle: env.RAMP_LAYERX_SIGNER_KEY_HANDLE
	},
	quotes: $quotes[0],
	client_tls: {
		ca_pem: ($s + "/outbound-ca.pem"),
		identity_pkcs12: ($s + "/outbound-identity.p12"),
		identity_password_file: ($s + "/outbound-identity-password"),
		timeout_seconds: (env.RAMP_CLIENT_TIMEOUT_SECONDS | tonumber)
	},
	identity: {endpoint: env.RAMP_IDENTITY_ENDPOINT, service_token_file: ($s + "/identity-token"), audience: env.RAMP_IDENTITY_AUDIENCE},
	compliance: {endpoint: env.RAMP_COMPLIANCE_ENDPOINT, service_token_file: ($s + "/compliance-token"), public_key: env.RAMP_COMPLIANCE_PUBLIC_KEY},
	provider: {
		endpoint: env.RAMP_PROVIDER_ENDPOINT,
		credential_file: ($s + "/provider-token"),
		settlement_path: "/layerx-ramp-v1/settlements",
		status_path: "/layerx-ramp-v1/settlements"
	},
	layerx: {
		rpc_endpoint: env.RAMP_SEQUENCER_RPC_ENDPOINT,
		rpc_trust_anchor_der: ($s + "/paxeer-rpc-ca.der"),
		chain_id: (env.RAMP_RPC_CHAIN_ID | tonumber),
		operator_address: env.RAMP_OPERATOR_ADDRESS,
		receipt_keys: (env.RAMP_RECEIPT_KEYS_JSON | fromjson),
		max_fee_per_gas: (env.RAMP_MAX_FEE_PER_GAS | tonumber),
		max_priority_fee_per_gas: (env.RAMP_MAX_PRIORITY_FEE_PER_GAS | tonumber),
		gas_limit_cap: (env.RAMP_GAS_LIMIT_CAP | tonumber),
		signer_endpoint: env.RAMP_SIGNER_ENDPOINT,
		signer_token_file: ($s + "/kms-token"),
		actor_did: env.RAMP_ACTOR_DID
	},
	paxeer: {
		custody_endpoint: env.RAMP_CUSTODY_ENDPOINT,
		custody_credential_file: ($s + "/paxeer-custody-token"),
		broadcast_path: "/layerx-paxeer-v1/rebalances",
		status_path: "/layerx-paxeer-v1/rebalances",
		operator_account: ("agent:" + env.RAMP_ACTOR_DID + ":main"),
		wallet_address: env.RAMP_WALLET_ADDRESS,
		vault_id: env.RAMP_VAULT_ID,
		signer_key_handle: env.RAMP_PAXEER_SIGNER_KEY_HANDLE,
		poll_cadence_seconds: (env.RAMP_POLL_CADENCE_SECONDS | tonumber)
	},
	provider_callback_public_key: env.RAMP_PROVIDER_CALLBACK_PUBLIC_KEY,
	operator_control_token_file: ($s + "/operator-control-token")
}' >"$config_dir/config.json.new"
mv "$config_dir/config.json.new" "$config_dir/config.json"
chown -R 4020:4020 "$secrets" "$config_dir" "$journal_dir"
chmod 0600 "$secrets"/* "$config_dir/config.json"
chmod 0700 "$secrets" "$config_dir" "$journal_dir"
chmod 0711 /run/layerx-ramp
exec setpriv --reuid=4020 --regid=4020 --clear-groups --no-new-privs \
	/usr/local/bin/layerx-reference-ramp "$config_dir/config.json"

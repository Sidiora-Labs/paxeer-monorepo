#!/usr/bin/env bash
set -euo pipefail

usage() {
	cat >&2 <<'EOF'
usage: tools/bringup/mint-secrets.sh migrate-retired OLD_DIR NEW_DIR
       ATTESTOR_CERT_DIR=<dir> MODULE_REGISTRY_FILE=<file> tools/bringup/mint-secrets.sh <out-dir>

Mints the beta secrets of the wallet, gateway, human service, webhooks,
internal, identity, registry, dashboard and ramp services on this
host, each token once, and writes it to every consumer that needs it, under
<out-dir> (created with mode 0700, every file 0600):

  <service>.railway.env  KEY=value lines of one Railway service
  <service>.railway.sh   sets each of them with railway variable set
                         --stdin --skip-deploys (run with sh; the CLI is
                         LAYERX_RAILWAY_BIN, default ~/.railway/bin/railway,
                         the environment LAYERX_RAILWAY_ENVIRONMENT, default
                         beta)
  <app>.box.env          KEY=value lines of one box app, its EnvironmentFile

Values of file secrets are base64, as the image's env-files shim decodes
them; values a service reads straight from its environment (the identity
binding tenant, the kernel bearers, the registry tokens and bearers) are
plain. The public and ingress webhook roles receive only their own capabilities.
Explicit migrate-retired writes a new protected directory, reuses surviving tokens,
and preserves the old mint intact for operator revocation of retired credentials.
Nothing secret is
printed; stdout lists the written paths.

ATTESTOR_CERT_DIR     attestor-1.crt .. attestor-5.crt, the deployed attestor
                      TLS certificates (PEM); their SPKI SHA-256 pins are the
                      custody inventory members.
MODULE_REGISTRY_FILE  the kernel deployment's module-registry.json.

The issuer and tenant are read from services/wallet/deploy/attestor-*.env.example
and must agree across the five attestors. <out-dir>/keys keeps the operator
custody inventory signing key, the only copy; the inventory expires 30 days
after minting and is re-signed with that key.
EOF
	exit 2
}

if [ "${1:-}" = migrate-retired ]; then
	[ "$#" -eq 3 ] || usage
	root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
	exec "${BRINGUP_INSPECT_BIN:-bringup-inspect}" mint-migrate "$root/deploy/rollout.kvx" "$root/services/wallet/deploy/human.toml" "$2" "$3"
fi
[ "$#" -eq 1 ] || usage
out=$1
: "${ATTESTOR_CERT_DIR:?ATTESTOR_CERT_DIR is required}"
: "${MODULE_REGISTRY_FILE:?MODULE_REGISTRY_FILE is required}"
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
deploy=$root/services/wallet/deploy

dotenv() { sed -n "s/^$2=//p" "$1"; }
issuer=$(dotenv "$deploy/attestor-1.env.example" ATTESTOR_AUTHORITY_ISSUER)
tenant=$(dotenv "$deploy/attestor-1.env.example" ATTESTOR_AUTHORITY_TENANT)
[ -n "$issuer" ] && [ -n "$tenant" ] || { echo "mint-secrets: attestor-1.env.example lacks the authority issuer or tenant" >&2; exit 1; }
case "$issuer" in https://*/auth/v1) ;; *) echo "mint-secrets: issuer must be <supabase url>/auth/v1" >&2; exit 1 ;; esac
for i in 2 3 4 5; do
	[ "$(dotenv "$deploy/attestor-$i.env.example" ATTESTOR_AUTHORITY_ISSUER)" = "$issuer" ] &&
		[ "$(dotenv "$deploy/attestor-$i.env.example" ATTESTOR_AUTHORITY_TENANT)" = "$tenant" ] ||
		{ echo "mint-secrets: attestor-$i.env.example disagrees with attestor-1.env.example on issuer or tenant" >&2; exit 1; }
done
python3 -c 'import json,sys; json.load(open(sys.argv[1]))' "$MODULE_REGISTRY_FILE" ||
	{ echo "mint-secrets: $MODULE_REGISTRY_FILE is not JSON" >&2; exit 1; }

if [ -e "$out" ] && [ -n "$(ls -A "$out")" ]; then
	echo "mint-secrets: $out is not empty; refusing to replace minted secrets" >&2
	exit 1
fi
umask 077
mkdir -p "$out/keys"
chmod 0700 "$out" "$out/keys"
keys=$out/keys

pins=()
for i in 1 2 3 4 5; do
	pin=$(openssl x509 -in "$ATTESTOR_CERT_DIR/attestor-$i.crt" -pubkey -noout | openssl pkey -pubin -outform DER | sha256sum | cut -d' ' -f1)
	[ "${#pin}" -eq 64 ] || { echo "mint-secrets: cannot pin attestor-$i.crt" >&2; exit 1; }
	pins+=("$pin")
done

openssl ecparam -name prime256v1 -genkey -noout | openssl pkcs8 -topk8 -nocrypt -out "$keys/wallet_identity_binding_key.pem"
openssl pkey -in "$keys/wallet_identity_binding_key.pem" -pubout -out "$keys/wallet_identity_binding_public.pem"
openssl ecparam -name prime256v1 -genkey -noout | openssl pkcs8 -topk8 -nocrypt -out "$keys/wallet_custody_inventory_signer.pem"
openssl pkey -in "$keys/wallet_custody_inventory_signer.pem" -pubout -out "$keys/wallet_custody_inventory_authority.pem"

# The inventory token has the shape the gateway (src/agent/authority.ts) and
# the attestors (internal/server/authority.go) verify: ES256 over
# header.payload, P1363 signature, members pinned to the attestor SPKIs.
node - "$keys/wallet_custody_inventory_signer.pem" "$issuer" "$tenant" "${pins[@]}" >"$keys/wallet_custody_inventory.jwt" <<'EOF'
const { createPrivateKey, sign } = require('node:crypto');
const { readFileSync } = require('node:fs');
const [key, iss, tenant, ...pins] = process.argv.slice(2);
const iat = Math.floor(Date.now() / 1000) - 60;
const payload = { version: 1, iss, aud: 'wallet-custody-inventory', tenant, sequence: '1', iat, exp: iat + 2_592_000,
  protocol: 'wallet', threshold: 3, members: pins.map((spki_sha256, i) => ({ id: String(i + 1), spki_sha256 })), keys: [] };
const enc = (v) => Buffer.from(JSON.stringify(v)).toString('base64url');
const input = `${enc({ alg: 'ES256', typ: 'wallet-custody-inventory+jwt' })}.${enc(payload)}`;
const signature = sign('sha256', Buffer.from(input, 'ascii'), { key: createPrivateKey(readFileSync(key)), dsaEncoding: 'ieee-p1363' });
process.stdout.write(`${input}.${signature.toString('base64url')}\n`);
EOF

b64() { base64 -w0; }
b64v() { printf '%s' "$1" | b64; }
b64f() { b64 <"$1"; }

# envfile <file> NAME=value...: one KEY=value line per pair, 0600.
envfile() {
	local file=$1
	shift
	printf '%s\n' "$@" >"$file"
	chmod 0600 "$file"
	echo "$file"
}

# railway_service <service> NAME=value...: the service's dotenv and the
# script that sets each variable from it on standard input.
railway_service() {
	local service=$1
	shift
	envfile "$out/$service.railway.env" "$@"
	cat >"$out/$service.railway.sh" <<EOF
#!/bin/sh
set -eu
railway=\${LAYERX_RAILWAY_BIN:-\$HOME/.railway/bin/railway}
while IFS= read -r pair; do
	printf '%s' "\${pair#*=}" | "\$railway" variable set "\${pair%%=*}" --stdin --service $service --environment "\${LAYERX_RAILWAY_ENVIRONMENT:-beta}" --skip-deploys >/dev/null
done <"\$(dirname "\$0")/$service.railway.env"
EOF
	chmod 0600 "$out/$service.railway.sh"
	echo "$out/$service.railway.sh"
}

# box_app <app> NAME=value...
box_app() {
	local app=$1
	shift
	envfile "$out/$app.box.env" "$@"
}

railway_service wallet-gateway \
	"WALLET_IDENTITY_BINDING_TENANT=$tenant" \
	"WALLET_IDENTITY_BINDING_KEY=$(b64f "$keys/wallet_identity_binding_key.pem")" \
	"WALLET_CUSTODY_INVENTORY=$(b64f "$keys/wallet_custody_inventory.jwt")" \
	"WALLET_CUSTODY_INVENTORY_PUBLIC_KEY=$(b64f "$keys/wallet_custody_inventory_authority.pem")"

for i in 1 2 3 4 5; do
	box_app "paxeer-attestor-$i" \
		"WALLET_IDENTITY_BINDING_PUBLIC_KEY=$(b64f "$keys/wallet_identity_binding_public.pem")" \
		"WALLET_CUSTODY_INVENTORY_PUBLIC_KEY=$(b64f "$keys/wallet_custody_inventory_authority.pem")" \
		"WALLET_CUSTODY_INVENTORY=$(b64f "$keys/wallet_custody_inventory.jwt")"
done

# Token roles and their current delivery targets are resolved from rollout and
# the Human box consumer by the shared producer. No retired authority or
# replica bearer, router-named output, or interop authority is generated.
"${BRINGUP_INSPECT_BIN:-bringup-inspect}" mint-token-plan "$root/deploy/rollout.kvx" "$root/services/wallet/deploy/human.toml" "$out" "$MODULE_REGISTRY_FILE"

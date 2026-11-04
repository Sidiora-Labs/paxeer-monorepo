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
binding tenant, identity and registry tokens) are
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
: "${ATTESTOR_CERT_DIR:?ATTESTOR_CERT_DIR is required}"
: "${MODULE_REGISTRY_FILE:?MODULE_REGISTRY_FILE is required}"
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
exec "${BRINGUP_INSPECT_BIN:-bringup-inspect}" mint "$1" "$ATTESTOR_CERT_DIR" "$MODULE_REGISTRY_FILE" "$root/services/wallet/deploy"

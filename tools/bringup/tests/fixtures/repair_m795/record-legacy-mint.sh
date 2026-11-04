#!/usr/bin/env bash
# Records public disposable migration inputs using the unchanged old producer.
set -euo pipefail
root=$(git rev-parse --show-toplevel)
cd "$root"
fixture=tools/bringup/tests/fixtures/repair_m795
producer=tools/bringup/.record-legacy-m795.sh
certs=$fixture/.certificates
trap 'rm -rf "$producer" "$certs"' EXIT
mkdir -p "$certs"
for i in 1 2 3 4 5; do
 openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:prime256v1 -nodes -days 1 -subj "/CN=public-fixture-attestor-$i" -keyout "$certs/attestor-$i.key" -out "$certs/attestor-$i.crt" 2>/dev/null
done
git show 5f6b5f0aad35a415da93671f70183d5e5688bda0:tools/bringup/mint-secrets.sh >"$producer"
ATTESTOR_CERT_DIR="$root/$certs" MODULE_REGISTRY_FILE="$root/services/interop/deploy/gateway/module-registry.example.json" bash "$producer" "$root/$fixture/legacy-mint" >/dev/null

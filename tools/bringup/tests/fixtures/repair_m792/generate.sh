#!/usr/bin/env bash
set -euo pipefail
# Disposable, committed peer recordings. Never use these keys for deployment.
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../../.." && pwd)
fixture="$root/tools/bringup/tests/fixtures/repair_m792"
. "$root/tools/bringup/ca.sh"
issuer="$fixture/issuer"
mkdir -m 0700 "$issuer"
ca_dir="$issuer"
ca_init >/dev/null
key="$fixture/key.pem"
openssl genpkey -algorithm EC -pkeyopt ec_paramgen_curve:P-256 -out "$key" 2>/dev/null
for generation in expired renewed; do
	work="$fixture/$generation"
	mkdir -m 0700 "$work"
	cp "$key" "$work/key.pem"
	openssl req -new -key "$work/key.pem" -subj '/O=Paxeer X Network/CN=layerx-agentd' -out "$work/csr.pem"
	cert_days=1
	[ "$generation" != renewed ] || cert_days=397
	sign agentd serverAuth DNS:layerx-agentd,DNS:localhost,IP:127.0.0.1
	cp "$issuer/ca.pem" "$work/ca.pem"
	(cd "$work" && sh -c "$(derive_cmd layerx-agentd)")
	chmod 0600 "$work"/*
	rm "$work/csr.pem" "$work/ext.cnf"
done
rm "$key" "$issuer/ca.key" "$issuer/ca.srl" "$issuer/ca.der"

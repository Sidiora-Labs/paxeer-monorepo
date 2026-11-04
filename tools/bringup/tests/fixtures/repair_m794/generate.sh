#!/usr/bin/env bash
set -euo pipefail
# Disposable conformance recordings from the actual ca.sh signing and encoding
# producer. No issuer private key is retained, and these keys are never deployed.
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../../.." && pwd)
fixture="$root/tools/bringup/tests/fixtures/repair_m794"
. "$root/tools/bringup/ca.sh"
issuer=$(mktemp -d)
trap 'rm -rf "$issuer"' EXIT
ca_dir="$issuer"
ca_init >/dev/null
mkdir "$issuer/attestor"
ca_dir="$issuer/attestor"
ca_init >/dev/null
ca_rows >"$fixture/approved-roster.tsv"
while read -r service target custody cn eku sans; do
	ca_dir="$issuer"
	[ "$service" != human-attestor-client ] || ca_dir="$issuer/attestor"
	work="$fixture/$service"
	mkdir -p "$work"
	chmod 0700 "$work"
	openssl genpkey -algorithm EC -pkeyopt ec_paramgen_curve:P-256 -out "$work/key.pem" 2>/dev/null
	openssl req -new -key "$work/key.pem" -subj "/O=Paxeer X Network/CN=$cn" -out "$work/csr.pem"
	[ "$sans" != - ] || sans=""
	cert_days=397
	sign "$service" "$eku" "$sans"
	cp "$ca_dir/ca.pem" "$work/ca.pem"
	(cd "$work" && sh -c "$(derive_cmd "$cn")")
	chmod 0600 "$work"/*
	rm "$work/csr.pem" "$work/ext.cnf"
done <"$fixture/approved-roster.tsv"

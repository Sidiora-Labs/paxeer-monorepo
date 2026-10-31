#!/bin/bash
# Runs as root inside the kernel app's machine (services/wallet/deploy/human.toml);
# tools/bringup/check-live.sh kernel-value-loop pipes it to bash -s through
# flyctl ssh console. It proves the value loop of the kernel:
#
#   value-loop.sh [ASSET=PAX|SID|USDC|USDL]
#
# Requires the genesis asset id, the sequencer's LNI socket and the receipt
# authority answering /readyz on 9445. The sender must already be funded.
# Sender and recipient seeds are retained under /data/layerx/keys/value-loop.
# Sends one unit, then reports both balances and the sealed batch from the
# receipt authority. A rerun reuses the retained SEND activity.
# Output lines: account, asset, activity, balance, batch; exits 0 on success,
# 1 on failure, or 3 when a precondition is absent.
# Nothing secret is printed.
set -euo pipefail
umask 077

layerx=${LAYERX_KERNEL_DATA:-/data/layerx}
layerx=${layerx%/}
run=${LAYERX_KERNEL_RUN:-/run/layerx}
run=${run%/}
tls=${LAYERX_KERNEL_TLS:-/data/tls}
tls=${tls%/}
keys=$layerx/keys
genesis=$layerx/genesis
state=$layerx/value-loop
lni=$run/node/layerxd.lni.sock
network_id=${LAYERX_NODE_NETWORK_ID:-125}
authority=https://127.0.0.1:9445

precondition() {
	echo "precondition $*"
	exit 3
}

failed() {
	echo "$*"
	exit 1
}

# as_lni <command...>: runs the command as the LNI client uid of the pod.
as_lni() { setpriv --reuid 4021 --regid 4020 --clear-groups -- "$@"; }

seed_public() {
	od -An -tx1 "$1" | tr -d ' \n' | python3 -c '
import sys
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from cryptography.hazmat.primitives.serialization import Encoding, PublicFormat
key = Ed25519PrivateKey.from_private_bytes(bytes.fromhex(sys.stdin.read().strip()))
print(key.public_key().public_bytes(Encoding.Raw, PublicFormat.Raw).hex())'
}

# main_id <did>: sha256("LX:ACCOUNT:v1" || u32 length || "agent:<did>:main").
main_id() {
	python3 -c '
import hashlib, sys
name = ("agent:" + sys.argv[1] + ":main").encode()
print(hashlib.sha256(b"LX:ACCOUNT:v1" + len(name).to_bytes(4, "big") + name).hexdigest())' "$1"
}

handshake_sealed() {
	as_lni layerx-node-probe handshake --socket "$lni" --network-id "$network_id" 2>/dev/null |
		jq -er '.latest_sealed_batch'
}

balance() {
	as_lni layerx-node-probe balance --socket "$lni" --network-id "$network_id" \
		--account "agent:$1:main" --asset "$asset" 2>/dev/null
}

main() {
	local argument file pid bearer="" role seed public did id
	local answer activity sealed batch deadline token_files
	local symbol=PAX explicit_asset=0
	for argument in "$@"; do
		case "$argument" in
		ASSET=PAX|ASSET=SID|ASSET=USDC|ASSET=USDL) symbol=${argument#ASSET=}; explicit_asset=1 ;;
		*) failed "usage value-loop.sh [ASSET=PAX|SID|USDC|USDL]" ;;
		esac
	done
	[ "$(id -u)" = 0 ] || failed "usage run as root inside the kernel machine"

	for file in "$genesis/asset-id"; do
		[ -s "$file" ] || precondition genesis-ids "$file absent; run kernel-genesis.sh genesis (step C) first"
	done
	if [ "$explicit_asset" = 1 ]; then
		state="$layerx/value-loop/assets/$symbol"
	fi
	[ -S "$lni" ] || precondition kernel-node "$lni absent; the sequencer has not bootstrapped the genesis"
	for pid in /proc/[0-9]*; do
		[ "$(basename "$(readlink "$pid/exe" 2>/dev/null)")" = layerx-receipt-authority ] || continue
		token_files=$(tr '\000' '\n' <"$pid/environ" 2>/dev/null | sed -n 's/^LAYERX_AUTHORITY_TOKEN_FILES=//p' | head -n 1) || true
		[ -n "$token_files" ] && bearer=$(tr -d '\r\n' <"${token_files%%:*}") && break
	done
	[ -n "$bearer" ] || precondition receipt-authority "no layerx-receipt-authority process with a token file; task 2.1 runs it on 9445"
	[ -s "$tls/receipt-authority/ca.pem" ] ||
		precondition receipt-authority "$tls/receipt-authority/ca.pem absent; task 2.1 issues its certificate"
	answer=$(curl -sS -m 10 --cacert "$tls/receipt-authority/ca.pem" "$authority/readyz" 2>/dev/null) || answer=""
	[ "$(jq -r '.ready' <<<"$answer" 2>/dev/null)" = true ] ||
		precondition receipt-authority "not answering ready on $authority/readyz: ${answer:-no answer}"
	for tool in layerx-node-probe layerxctl; do
		command -v "$tool" >/dev/null || precondition image "$tool is not in the kernel image"
	done

	asset=$(printf 'layerx-asset:125:%s' "$symbol" | sha256sum | cut -d' ' -f1)
	if [ "$symbol" = PAX ]; then
		[ "$(tr -d ' \r\n' <"$genesis/asset-id")" = "$asset" ] || failed "asset PAX genesis id differs from $asset"
	fi
	echo "asset $symbol id=$asset"

	install -d -o 0 -g 4020 -m 0750 "$keys/value-loop"
	if [ "$explicit_asset" = 1 ]; then
		install -d -o 4021 -g 4020 -m 0700 "$layerx/value-loop" "$layerx/value-loop/assets"
	fi
	install -d -o 4021 -g 4020 -m 0700 "$state"
	declare -gA dids=() mains=()
	for role in sender recipient; do
		seed=$keys/value-loop/$role.key
		if [ ! -s "$seed" ]; then
			install -o 4021 -g 4020 -m 0400 /dev/null "$seed.new"
			openssl rand 32 >"$seed.new"
			mv "$seed.new" "$seed"
		fi
		public=$(seed_public "$seed")
		did=did:layerx:$public
		id=$(main_id "$did")
		dids[$role]=$did
		mains[$role]=$id
		echo "account $role did=$did main=$id"
	done
	answer=$(balance "${dids[sender]}") || failed "balance sender read failed"
	if jq -e '.refused.result == -208' <<<"$answer" >/dev/null 2>&1; then
		precondition sender-balance "the sender account ${mains[sender]} must be funded before running the value loop"
	fi

	if [ ! -s "$state/send.activity-id" ]; then
		rm -f "$state/send.bin"
		local send_command=write-send
		if [ "$explicit_asset" = 1 ]; then send_command=write-asset-send; fi
		activity=$(as_lni layerx-node-probe "$send_command" --socket "$lni" --network-id "$network_id" \
			--seed-file "$keys/value-loop/sender.key" --destination-did "${dids[recipient]}" \
			--asset "$asset" --output "$state/send.bin" 2>/dev/null) || failed "activity write-send refused"
		answer=$(as_lni layerxctl submit --socket "$lni" --network-id "$network_id" --protocol-version 3 \
			--actor "${dids[sender]}" --public-key "${dids[sender]#did:layerx:}" --activity "$state/send.bin" 2>&1) || answer=""
		jq -e --arg a "$activity" '.state == "acknowledged" and .activity_id == $a' <<<"$answer" >/dev/null 2>&1 ||
			failed "activity id=$activity submit=${answer:-none}"
		printf '%s\n' "$activity" >"$state/send.activity-id"
	fi
	activity=$(cat "$state/send.activity-id")
	echo "activity id=$activity"
	deadline=$(($(date +%s) + 120))
	until as_lni layerxctl read-state --socket "$lni" --network-id "$network_id" --protocol-version 3 \
		--actor "${dids[sender]}" >/dev/null 2>&1; do
		[ "$(date +%s)" -lt "$deadline" ] || failed "activity id=$activity sealed=never"
		sleep 1
	done

	for role in sender recipient; do
		answer=$(balance "${dids[$role]}") || answer=""
		echo "balance $role $(jq -c '{balance, refused}' <<<"$answer" 2>/dev/null || echo none)"
	done

	deadline=$(($(date +%s) + 120))
	while :; do
		answer=$(curl -sS -m 15 --cacert "$tls/receipt-authority/ca.pem" -H "Authorization: Bearer $bearer" \
			"$authority/v1/authorized-batches/by-activity/$activity" 2>/dev/null) || answer=""
		batch=$(jq -er --arg a "$activity" 'select(.activity_id == $a) | .batch_id' <<<"$answer" 2>/dev/null) && break
		[ "$(date +%s)" -lt "$deadline" ] || failed "batch activity=$activity authority=${answer:-none}"
		sleep 2
	done
	sealed=$(handshake_sealed) || failed "batch id=$batch sealed=unreadable"
	echo "batch id=$batch sealed=$sealed"
}

main "$@" </dev/null
exit

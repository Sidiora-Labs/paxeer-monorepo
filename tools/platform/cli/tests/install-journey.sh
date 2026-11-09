#!/usr/bin/env bash
set -euo pipefail

# Also usable by the focused subprocess proof without live payment credentials.
check_cli_output() {
  jq -se --arg kind "$1" '
    length == 1 and .[0].ok == true and .[0].kind == $kind
    and (.[0].message | type) == "string" and (.[0] | has("data"))'
}
if [ "${1:-}" = "--check-cli-output" ]; then
  check_cli_output "${2:?expected command kind}"
  exit
fi

: "${LAYERX_BIN:?set LAYERX_BIN to the built layerx executable}"
: "${LAYERX_GATEWAY_URL:?set the hosted gateway URL}"
: "${LAYERX_NODE_RPC_URL:?set the Ethereum node root RPC URL}"
: "${LAYERX_EVM_KEY_FILE:?set the private wallet derive export for the funded source identity}"
: "${LAYERX_NETWORK_ID:?set the hosted protocol network id}"
: "${LAYERX_IDENTITY_TOKEN:?set a short-lived identity session}"
: "${LAYERX_SIGNING_SEED:?set the funded source Ed25519 seed}"
: "${LAYERX_SOURCE_ACCOUNT:?set the funded 64-hex source account}"
: "${LAYERX_PAYMENT_ASSET:?set the 64-hex payment asset}"
: "${LAYERX_BASE_PAYMENT_DENOM:?set the independently expected native base payment denomination}"
: "${LAYERX_CONSENSUS_CHAIN_ID:?set the independently expected consensus chain id}"
: "${LAYERX_PAYMENT_DESTINATION:?set the 64-hex destination account}"
: "${LAYERX_A2A_SEQUENCE:?set the next source sequence for the A2A payment}"
: "${LAYERX_SEQUENCER_PUBLIC_KEY:?set the independently pinned 64-hex sequencer public key}"
: "${LAYERX_MCP_DAEMON_BINDING:?set the absolute binding document written by genuine full-mode agent-daemon enrolment}"
: "${LAYERX_MCP_PREPARE_REQUEST_FILE:?set the private activity.prepare request issued for the enrolled daemon session}"
: "${LAYERX_MCP_SIGNER_SEED_FILE:?set the private 32-byte Ed25519 seed of the enrolled activity signer}"

hex32='^[0-9a-f]{64}$'
for value in "$LAYERX_SOURCE_ACCOUNT" "$LAYERX_PAYMENT_ASSET" "$LAYERX_PAYMENT_DESTINATION" "$LAYERX_SEQUENCER_PUBLIC_KEY"; do
  [[ "$value" =~ $hex32 ]] || { echo "journey identities must be lowercase 64-hex values" >&2; exit 1; }
done
case "$LAYERX_MCP_DAEMON_BINDING" in
  /*) ;;
  *) echo "LAYERX_MCP_DAEMON_BINDING must be absolute" >&2; exit 1 ;;
esac
for private_input in "$LAYERX_MCP_DAEMON_BINDING" "$LAYERX_MCP_PREPARE_REQUEST_FILE" "$LAYERX_MCP_SIGNER_SEED_FILE" "$LAYERX_EVM_KEY_FILE"; do
  if [ ! -f "$private_input" ] || [ -L "$private_input" ] \
    || [ "$(stat -c '%u' "$private_input")" != "$(id -u)" ] \
    || [ $((8#$(stat -c '%a' "$private_input") & 8#077)) -ne 0 ]; then
    echo "journey input $private_input must be a private regular file owned by the caller" >&2
    exit 1
  fi
done
test "$(stat -c '%s' "$LAYERX_MCP_SIGNER_SEED_FILE")" -eq 32

journey_root=$(mktemp -d)
cleanup() {
  LAYERX_CONFIG="$journey_root/config.json" LAYERX_INSTALL_ROOT="$journey_root" \
    "$LAYERX_BIN" a2a stop >/dev/null 2>&1 || true
  rm -rf "$journey_root"
}
trap cleanup EXIT INT TERM
umask 077

export LAYERX_CONFIG="$journey_root/config.json"
export LAYERX_INSTALL_ROOT="$journey_root"

"$LAYERX_BIN" --json environment use beta \
  --endpoint "$LAYERX_GATEWAY_URL" --network-id "$LAYERX_NETWORK_ID" >/dev/null
printf '%s\n' "$LAYERX_SIGNING_SEED" | \
  "$LAYERX_BIN" --json key import agent-runtime >/dev/null

mcp_refusal="$journey_root/mcp-install.json"
if "$LAYERX_BIN" --json install mcp --host layerx >"$mcp_refusal" 2>&1; then
  echo "install mcp registered a server without an agent-daemon binding" >&2
  exit 1
fi
grep -q 'binding.json' "$mcp_refusal"
grep -q 'agent-daemon enrolment' "$mcp_refusal"
test ! -e "$journey_root/mcp.json"

mcp_install="$journey_root/mcp-install-bound.json"
"$LAYERX_BIN" --json install mcp --host layerx --daemon-binding "$LAYERX_MCP_DAEMON_BINDING" >"$mcp_install"
jq -e --arg binding "$LAYERX_MCP_DAEMON_BINDING" '
  .data.component == "mcp" and .data.transport == "stdio"
  and .data.authorization == "agent-daemon" and .data.deployment_mode == "full"
  and .data.daemon_binding.path == $binding and .data.daemon_binding.declared_mode == "full"
  and .data.server.args == ["mcp", "serve", "--daemon-binding", $binding]
  and .data.server.env == {}
  and ([.data.tools[].name] | contains(["activity.prepare", "activity.disclose", "activity.sign", "activity.submit", "activity.track"]))
  and (.data.registrations | length) == 1 and .data.registrations[0].host == "layerx"
  and .data.changed == true' "$mcp_install" >/dev/null
jq -e --arg binding "$LAYERX_MCP_DAEMON_BINDING" '
  .mcpServers.layerx.args == ["mcp", "serve", "--daemon-binding", $binding]' "$journey_root/mcp.json" >/dev/null
"$LAYERX_BIN" --json install mcp --host layerx --daemon-binding "$LAYERX_MCP_DAEMON_BINDING" \
  | jq -e '.data.changed == false and .data.idempotent == true' >/dev/null
for secret in "$LAYERX_IDENTITY_TOKEN" "$LAYERX_SIGNING_SEED"; do
  if grep -qF -- "$secret" "$journey_root/mcp.json" "$mcp_install"; then
    echo "MCP installation published a credential outside credential storage" >&2
    exit 1
  fi
done

mcp_payment="$journey_root/mcp-payment.json"
python3 - "$journey_root/mcp.json" "$LAYERX_MCP_PREPARE_REQUEST_FILE" "$LAYERX_MCP_SIGNER_SEED_FILE" \
  >"$mcp_payment" <<'MCP_JOURNEY'
import json
import os
import re
import select
import subprocess
import sys
import time
from pathlib import Path

from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from cryptography.hazmat.primitives.serialization import Encoding, PublicFormat

LEVELS = ('StateProven', 'CheckpointFinalised', 'SettlementAnchored')
PENDING = ('Prepared', 'Signed', 'Queued', 'Submitted', 'Acknowledged', 'Unknown')
PAYMENT = {'activity.prepare', 'activity.disclose', 'activity.sign', 'activity.submit', 'activity.track'}


def require(condition, reason):
    if not condition:
        raise SystemExit('mcp journey: ' + reason)


entry = json.loads(Path(sys.argv[1]).read_text())['mcpServers']['layerx']
request = json.loads(Path(sys.argv[2]).read_text())
signer = Ed25519PrivateKey.from_private_bytes(Path(sys.argv[3]).read_bytes())
public = signer.public_key().public_bytes(Encoding.Raw, PublicFormat.Raw).hex()
key = request.get('idempotency_key', '') if isinstance(request, dict) else ''
require(re.fullmatch('[0-9a-f]{64}', key) is not None, 'prepare request carries no idempotency key')
environment = {name: value for name, value in os.environ.items() if not name.startswith('LAYERX_')}
environment.update(entry.get('env', {}))
server = subprocess.Popen([entry['command'], *entry['args']], stdin=subprocess.PIPE,
                          stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, env=environment)
sequence = 0


def message(method, params):
    global sequence
    sequence += 1
    line = json.dumps({'jsonrpc': '2.0', 'id': sequence, 'method': method, 'params': params},
                      separators=(',', ':'))
    server.stdin.write(line.encode() + b'\n')
    server.stdin.flush()
    require(bool(select.select([server.stdout], [], [], 60)[0]), method + ' response deadline')
    raw = server.stdout.readline(1_048_577)
    require(raw.endswith(b'\n') and len(raw) <= 1_048_576, 'unbounded MCP response line')
    response = json.loads(raw)
    require(response.get('jsonrpc') == '2.0' and response.get('id') == sequence,
            'MCP response lost its invocation identity')
    require('error' not in response, method + ' refused: ' + json.dumps(response.get('error')))
    return response['result']


def call(tool, arguments):
    result = message('tools/call', {'name': tool, 'arguments': arguments,
                                    '_meta': {'layerx/idempotency_key': key}})
    require(isinstance(result, dict) and len(result.get('content', [])) == 1
            and result['content'][0].get('type') == 'text', tool + ' result framing')
    value = json.loads(result['content'][0]['text'])
    require(result.get('structuredContent') == value, tool + ' structured result differs')
    require(result.get('isError') is False and value.get('tool') == tool and 'result' in value,
            tool + ' refused: ' + json.dumps(value.get('refusal')))
    return value['result']


try:
    message('initialize', {})
    listed = {tool['name'] for tool in message('tools/list', {})['tools']}
    require(PAYMENT <= listed, 'installed server does not serve the payment journey')
    prepared = call('activity.prepare', request)
    require(re.fullmatch('[0-9a-f]{64}', prepared.get('preparation_id', '')) is not None
            and re.fullmatch('[0-9a-f]{64}', prepared.get('signing_preimage', '')) is not None
            and prepared.get('approval_required') is False, 'incomplete native preparation')
    disclosed = call('activity.disclose', {'canonical_bytes': prepared['canonical_bytes']})
    require(disclosed.get('preparation_id') == prepared['preparation_id']
            and disclosed.get('canonical_bytes') == prepared['canonical_bytes'],
            'disclosure differs from the immutable preparation')
    signature = signer.sign(bytes.fromhex(prepared['signing_preimage'])).hex()
    call('activity.sign', {'variant': 'external_signature_v1',
                           'preparation_ref': prepared['preparation_id'], 'signature': signature})
    observed = call('activity.submit', {'preparation_ref': prepared['preparation_id'],
                                        'signature': signature, 'signer_public_key': public})
    reference = observed['submission']['submission_ref']
    activity = observed['activity_id']
    deadline = time.monotonic() + 180
    while observed['submission'].get('state') != 'Executed':
        require(observed['submission'].get('state') in PENDING and observed.get('receipt') is None,
                'payment reached ' + str(observed['submission'].get('state')) + ' without a receipt')
        require(time.monotonic() < deadline, 'payment did not execute within the bounded wait')
        time.sleep(1)
        observed = call('activity.track', {'submission_ref': reference})
        require(observed['activity_id'] == activity
                and observed['submission']['submission_ref'] == reference,
                'tracking changed the submitted payment identity')
    receipt = observed.get('receipt') or {}
    require(observed['submission'].get('verification_level') in LEVELS
            and receipt.get('verification_level') in LEVELS,
            'executed payment carries no verified receipt')
    canonical = bytes.fromhex(receipt.get('canonical_bytes', ''))
    require(len(canonical) > 166 and canonical[6:10] == b'\0\0\0\x20'
            and canonical[10:42].hex() == activity, 'receipt belongs to another activity')
    print(json.dumps({'activity_id': activity, 'submission_ref': reference,
                      'verification_level': receipt['verification_level']}))
finally:
    server.stdin.close()
    server.terminate()
    server.wait(timeout=10)
MCP_JOURNEY
jq -e '.activity_id | test("^[0-9a-f]{64}$")' "$mcp_payment" >/dev/null

a2a_port="${LAYERX_A2A_PORT:-19433}"
printf '%s\n' "$LAYERX_IDENTITY_TOKEN" | \
  "$LAYERX_BIN" --json install a2a --environment beta --key agent-runtime \
    --token-stdin --source-account "$LAYERX_SOURCE_ACCOUNT" \
    --asset "$LAYERX_PAYMENT_ASSET" --listen "127.0.0.1:$a2a_port" \
    >"$journey_root/a2a-install.json"
check_cli_output install.a2a <"$journey_root/a2a-install.json" >/dev/null
jq -e '.data.lifecycle.state == "running"' "$journey_root/a2a-install.json" >/dev/null
a2a_authorization_file=$(jq -er '.data.authorization.credential_file' "$journey_root/a2a-install.json")
test "$(stat -c '%a' "$a2a_authorization_file")" = 600
for secret in "$LAYERX_IDENTITY_TOKEN" "$LAYERX_SIGNING_SEED"; do
  if grep -qF -- "$secret" "$journey_root/a2a/runtime.json" "$journey_root/a2a/agent-card.json" \
    "$journey_root/a2a-install.json"; then
    echo "A2A installation published a credential outside credential storage" >&2
    exit 1
  fi
done
a2a_authorization=$(tr -d '\r\n' <"$a2a_authorization_file")
"$LAYERX_BIN" --json a2a stop | jq -e '.data.state == "stopped"' >/dev/null
"$LAYERX_BIN" --json a2a start | jq -e '.data.state == "running"' >/dev/null

ready=false
for _ in $(seq 1 50); do
  if curl --silent --fail "http://127.0.0.1:$a2a_port/.well-known/agent-card.json" \
    | jq -e '.name == "LayerX"' >/dev/null; then
    ready=true
    break
  fi
  sleep 0.1
done
test "$ready" = true

a2a_payment="$journey_root/a2a-payment.json"
python3 - "$LAYERX_EVM_KEY_FILE" >"$a2a_payment" <<'A2A_SIGN'
import hashlib
import json
import os
import secrets
import sys
import time
import urllib.parse
import urllib.request
from pathlib import Path

from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from cryptography.hazmat.primitives.serialization import Encoding, PublicFormat
from eth_account import Account


def require(condition, reason):
    if not condition:
        raise SystemExit('a2a journey: ' + reason)


def integer(value, width):
    return int(value).to_bytes(width, 'big')


node = os.environ['LAYERX_NODE_RPC_URL']
parsed = urllib.parse.urlsplit(node)
require(parsed.path in ('', '/') and not parsed.query and not parsed.fragment,
        'the node RPC must be a root endpoint')


def rpc(method, params):
    body = json.dumps({'jsonrpc': '2.0', 'id': 1, 'method': method, 'params': params}).encode()
    request = urllib.request.Request(node, body, {'Content-Type': 'application/json'})
    with urllib.request.urlopen(request, timeout=15) as response:
        value = json.load(response)
    require(value.get('jsonrpc') == '2.0' and value.get('id') == 1
            and 'error' not in value and 'result' in value, method + ' refused')
    return value['result']


export = json.loads(Path(sys.argv[1]).read_text())
signer = Ed25519PrivateKey.from_private_bytes(bytes.fromhex(os.environ['LAYERX_SIGNING_SEED']))
public = signer.public_key().public_bytes(Encoding.Raw, PublicFormat.Raw)
require(export['layerx_seed'] == os.environ['LAYERX_SIGNING_SEED']
        and export['did'] == 'did:layerx:' + public.hex(), 'EVM export belongs to another identity')
account = Account.from_key(bytes.fromhex(export['evm_private_key']))
require(account.address.lower() == export['evm_address'].lower(), 'EVM export address mismatch')
require(int(rpc('eth_chainId', []), 16) == 125, 'expected chain 125')
source = bytes.fromhex(os.environ['LAYERX_SOURCE_ACCOUNT'])
destination = bytes.fromhex(os.environ['LAYERX_PAYMENT_DESTINATION'])
asset = bytes.fromhex(os.environ['LAYERX_PAYMENT_ASSET'])
require(source != destination, 'source and destination must differ')
ledger = '0x000000000000000000000000000000000000101a'


def read(selector, arguments):
    result = rpc('eth_call', [{'to': ledger, 'data': '0x' + selector + arguments.hex()}, 'pending'])
    decoded = bytes.fromhex(result.removeprefix('0x'))
    require(len(decoded) == 32, 'ledger read returned a non-word result')
    return decoded


require(read('37955ab2', source) == public, 'source is not controlled by the installed signer')
source_sequence = int.from_bytes(read('ce2f921f', source + asset), 'big')
require(source_sequence == int(os.environ['LAYERX_A2A_SEQUENCE']), 'source sequence changed')
require(int.from_bytes(read('b1ffc03e', source + asset), 'big') >= 1, 'source account is not funded')
nonce = int(rpc('eth_getTransactionCount', [account.address, 'pending']), 16)
expiry = int(time.time() * 1000) + 120000
idempotency = secrets.token_bytes(32)
context = hashlib.sha256(b'LXP/v1/context-hash\0' + source + destination + asset
                         + integer(1, 16) + idempotency).digest()
common = (source + destination + asset + integer(1, 16) + integer(source_sequence, 8)
          + idempotency + integer(expiry, 8) + context + b'\0')
network = int(os.environ['LAYERX_NETWORK_ID'])
authority = b'\1' + source
scope = context + integer(network, 4) + integer(3, 2)
authorization = integer(0x5301, 2) + common + authority + scope
digest = hashlib.sha256(b'LXP/v1/signature-preimage\0' + authorization).digest()
signature = signer.sign(digest)
signer.public_key().verify(signature, digest)
payload = integer(0x5301, 2) + integer(10, 2) + common + authority + public + signature + scope
calldata = (bytes.fromhex('2bb1ae7c') + integer(32, 32) + integer(len(payload), 32)
            + payload + bytes((-len(payload)) % 32))
gas_price = int(rpc('eth_gasPrice', []), 16)
priority = int(rpc('eth_maxPriorityFeePerGas', []), 16)
require(gas_price > 0, 'zero gas price')
max_fee = gas_price * 2 + priority
transaction = {'type': 2, 'chainId': 125, 'nonce': nonce, 'to': ledger, 'value': 0,
               'data': '0x' + calldata.hex(), 'gas': 300000,
               'maxPriorityFeePerGas': priority, 'maxFeePerGas': max_fee, 'accessList': []}
estimate = dict(transaction)
for field in ('type', 'chainId', 'nonce', 'value', 'gas', 'maxPriorityFeePerGas', 'maxFeePerGas'):
    estimate[field] = hex(estimate[field])
estimate['from'] = account.address
estimated = int(rpc('eth_estimateGas', [estimate, 'pending']), 16)
require(0 < estimated <= transaction['gas'], 'gas estimate exceeds the disclosed bound')
transaction['gas'] = estimated
require(int(rpc('eth_getBalance', [account.address, 'pending']), 16) >= estimated * max_fee,
        'EVM sender is not funded for the maximum gas charge')
disclosure = {'from': account.address, 'to': ledger, 'source': source.hex(),
              'destination': destination.hex(), 'asset': asset.hex(), 'amount': '1',
              'source_account_sequence': str(source_sequence), 'evm_nonce': str(nonce),
              'expires_at_ms': str(expiry), 'idempotency_key': idempotency.hex(),
              'chain_id': 125, 'gas': estimated, 'max_fee_per_gas': str(max_fee),
              'max_priority_fee_per_gas': str(priority), 'data': transaction['data']}
print(json.dumps({'a2a_disclosure': disclosure}), file=sys.stderr)
signed = Account.sign_transaction(transaction, account.key)
require(Account.recover_transaction(signed.raw_transaction).lower() == account.address.lower(),
        'signed transaction sender differs from the disclosure')
print(json.dumps({'raw_transaction': '0x' + signed.raw_transaction.hex(),
                  'txHash': '0x' + signed.hash.hex(), 'disclosure': disclosure}))
A2A_SIGN

a2a_request=$(jq -nc --slurpfile payment "$a2a_payment" \
  '{jsonrpc:"2.0",id:1,method:"message/send",params:{message:{kind:"message",role:"user",messageId:"install-journey",parts:[{kind:"data",data:{skill:"activity.submit",arguments:{raw_transaction:$payment[0].raw_transaction}}}]}}}')
unauthorized=$(curl --silent --show-error --output "$journey_root/a2a-unauthorized.json" \
  --write-out '%{http_code}' -H 'Content-Type: application/json' \
  --data "$a2a_request" "http://127.0.0.1:$a2a_port/")
test "$unauthorized" = 401
a2a_response=$(curl --fail-with-body --silent --show-error \
  -H 'Content-Type: application/json' -H "Authorization: Bearer $a2a_authorization" \
  --data "$a2a_request" "http://127.0.0.1:$a2a_port/")
a2a_transaction=$(printf '%s' "$a2a_response" | \
  jq -er --slurpfile payment "$a2a_payment" '
  select(.result.status.state == "submitted") | .result.artifacts[0].parts[0].data.result
  | select(.rung == "submitted" and .txHash == $payment[0].txHash) | .txHash')
[[ "$a2a_transaction" =~ ^0x[0-9a-f]{64}$ ]]

receipt_request=$(jq -nc --arg transaction "$a2a_transaction" \
  '{jsonrpc:"2.0",id:2,method:"message/send",params:{message:{kind:"message",role:"user",messageId:"install-journey-receipt",parts:[{kind:"data",data:{skill:"receipt.get",arguments:{activity_id:$transaction}}}]}}}')
receipt_document=
for _ in $(seq 1 90); do
  receipt_response=$(curl --fail-with-body --silent --show-error \
    -H 'Content-Type: application/json' -H "Authorization: Bearer $a2a_authorization" \
    --data "$receipt_request" "http://127.0.0.1:$a2a_port/")
  if printf '%s' "$receipt_response" | jq -e --arg transaction "$a2a_transaction" '
    .result.status.state == "completed" and
    (.result.artifacts[0].parts[0].data.result |
      .txHash == $transaction and (.rung == "receipted" or .rung == "committed") and .status == "0x1")' >/dev/null; then
    receipt_document=$(printf '%s' "$receipt_response" | jq -ec '.result.artifacts[0].parts[0].data.result')
    break
  fi
  sleep 2
done
test -n "$receipt_document"
printf '%s\n' "$receipt_document" >"$journey_root/a2a-served-receipt.json"
python3 - "$a2a_payment" "$journey_root/a2a-served-receipt.json" "$journey_root" <<'A2A_RECEIPT'
import hashlib
import json
import os
import sys
import time
import urllib.request
from pathlib import Path

def require(condition, reason):
    if not condition:
        raise SystemExit('a2a receipt reconciliation: ' + reason)


def hex_bytes(value, size=None):
    result = bytes.fromhex(value.removeprefix('0x'))
    require(size is None or len(result) == size, 'invalid evidence width')
    return result


def sequence_authority(height, first, last, previous, resulting, tx_root,
                       activities, transaction, denomination):
    require(height > 0 and first > 0 and last >= first
            and len(activities) == last - first + 1, 'batch range differs from block')
    ids = [hex_bytes(activity, 32) for activity in activities]
    require(len(set(ids)) == len(ids) and activities.count(transaction) == 1,
            'submitted transaction is absent or ambiguous in block')
    require(hashlib.sha256(b''.join(ids)).digest() == tx_root, 'batch transaction root differs')
    require(len(previous) == len(resulting) == 32, 'invalid sequence hash width')
    require(denomination and len(denomination.encode()) <= 64, 'invalid base payment denomination')
    current = previous
    authority = None
    for offset, activity in enumerate(ids):
        sequence = first + offset
        following = hashlib.sha256(b'PAXEER/v1/sequence\0' + current
                                   + sequence.to_bytes(8, 'big') + activity).digest()
        if activities[offset] == transaction:
            authority = {'batch_id': height.to_bytes(32, 'big').hex(),
                         'asset': hashlib.sha256(denomination.encode()).hexdigest(),
                         'previous_state_root': current.hex(),
                         'resulting_state_root': following.hex(), 'sequence': sequence}
        current = following
    require(current == resulting, 'batch resulting sequence hash differs')
    return authority


def batch_evidence(batch, height, data_hash):
    wire = hex_bytes(batch['tx'])
    domain = b'PAXEER/v1/sequence-batch\0'
    require(len(wire) == len(domain) + 216 and wire.startswith(domain), 'invalid sequence batch')
    fields = wire[len(domain):]
    positions = [int.from_bytes(fields[i:i + 8], 'big') for i in (0, 8, 16)]
    require(int(batch['height']) == height == positions[0], 'batch height differs from block')
    proof = batch['proof']
    require(proof['index'] == 0 and proof['total'] > 0, 'batch is not block transaction zero')
    leaf = hashlib.sha256(b'\0' + hashlib.sha256(wire).digest()).digest()
    require(leaf == hex_bytes(proof['leafHash'], 32), 'batch proof leaf differs')
    total = proof['total']
    depth = 0
    while total > 1:
        total = 1 << ((total - 1).bit_length() - 1)
        depth += 1
    require(len(proof['aunts']) == depth, 'batch proof has an inconsistent path')
    for aunt in proof['aunts']:
        leaf = hashlib.sha256(b'\1' + leaf + hex_bytes(aunt, 32)).digest()
    require(leaf == hex_bytes(data_hash, 32), 'batch proof differs from committed block')
    return wire, positions, fields[24:56], fields[56:88], fields[88:120]


def batch_digest(wire, chain_id):
    domain_size = len(b'PAXEER/v1/sequence-batch\0')
    return hashlib.sha256(wire[:domain_size] + chain_id.encode()
                          + wire[domain_size:domain_size + 120]).digest()


def reconcile_transaction(payment, served, actual, public_key, denomination):
    transaction = payment['txHash']
    disclosure = payment['disclosure']
    require(actual['txHash'] == transaction and actual['receipt'] == served['receipt']
            and actual['status'] == '0x1', 'A2A receipt differs from the actual node receipt')
    canonical = hex_bytes(actual['receipt'])
    require(canonical[:6] == b'\0\2R\1\0\2' and canonical[6:10] == b'\0\0\0\x20'
            and canonical[10:42].hex() == transaction[2:]
            and actual['from'].lower() == disclosure['from'].lower()
            and actual['to'].lower() == disclosure['to'].lower()
            and int(actual['nonce'], 16) == int(disclosure['evm_nonce']),
            'receipt does not bind the disclosed signed transaction')
    require(actual['signingKey'].removeprefix('0x') == public_key,
            'receipt does not use the independently pinned sequencer key')
    require(actual['paymentDenom'] == denomination, 'unexpected native payment denomination')
    return canonical


def reconcile_ledger(disclosure, logs):
    payment_exact = '0xc4da5d6f757c251cd70947b752917e5671b677517e3d953f362a1df6648ef5b1'
    # Transfer(bytes32,bytes32,bytes32,uint128,uint8)
    transfer = '0x3c136bc6b05877456edd6c1baed12a9d44c364ad7301478c948e7d70dd4b1ca0'
    outcomes = []
    for log in logs:
        if log['address'].lower() != disclosure['to'].lower():
            continue
        topics = log['topics']
        if not topics or topics[0] not in (payment_exact, transfer):
            continue
        require(len(topics) == 4, 'invalid ledger payment topics')
        data = hex_bytes(log['data'], 64)
        if topics[0] == payment_exact:
            source, destination, asset = topics[2], topics[3], data[:32].hex()
            amount = int.from_bytes(data[32:], 'big')
        else:
            source, destination, asset = topics[1], topics[2], topics[3].removeprefix('0x')
            amount = int.from_bytes(data[:32], 'big')
        outcomes.append((source.removeprefix('0x'), destination.removeprefix('0x'), asset, amount))
    require(outcomes == [(disclosure['source'], disclosure['destination'], disclosure['asset'],
                          int(disclosure['amount']))], 'ledger payment differs from signed disclosure')


def rpc(method, params):
    request = urllib.request.Request(os.environ['LAYERX_NODE_RPC_URL'],
        json.dumps({'jsonrpc': '2.0', 'id': 1, 'method': method, 'params': params}).encode(),
        {'Content-Type': 'application/json'})
    with urllib.request.urlopen(request, timeout=15) as response:
        result = json.load(response)
    if result.get('jsonrpc') != '2.0' or result.get('id') != 1 or 'error' in result:
        raise SystemExit('node receipt reconciliation refused: ' + method)
    return result['result']


payment = json.loads(Path(sys.argv[1]).read_text())
served = json.loads(Path(sys.argv[2]).read_text())
root = Path(sys.argv[3])
transaction = payment['txHash']
disclosure = payment['disclosure']
expected_denom = os.environ['LAYERX_BASE_PAYMENT_DENOM']
public_key = os.environ['LAYERX_SEQUENCER_PUBLIC_KEY']
actual = rpc('pax_sequencerReceipt', [transaction])
canonical = reconcile_transaction(payment, served, actual, public_key, expected_denom)
deadline = time.monotonic() + 180
while True:
    landed = rpc('eth_getTransactionReceipt', [transaction])
    if landed is not None:
        if landed['transactionHash'] != transaction or landed['status'] != '0x1':
            raise SystemExit('node transaction receipt reports another transaction or a revert')
        break
    if time.monotonic() >= deadline:
        raise SystemExit('transaction did not land before the receipt deadline')
    time.sleep(2)
height = int(landed['blockNumber'], 16)
block = rpc('eth_getBlockByNumber', [hex(height), False])
require(block is not None and int(block['number'], 16) == height
        and block['hash'] == landed['blockHash']
        and block['transactions'][int(landed['transactionIndex'], 16)] == transaction,
        'transaction receipt differs from independently read block')
commit = rpc('px_getCommit', [str(height)])
header = commit['signedHeader']['header']
require(int(commit['height']) == int(header['height']) == height
        and header['chain_id'] == os.environ['LAYERX_CONSENSUS_CHAIN_ID']
        and hex_bytes(commit['signedHeader']['commit']['block_id']['hash'], 32)
            == hex_bytes(block['hash'], 32), 'consensus evidence differs from transaction block')
batch = rpc('px_getSequenceBatch', [str(height)])
wire, positions, previous, resulting, tx_root = batch_evidence(batch, height, header['data_hash'])
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PublicKey
digest = batch_digest(wire, header['chain_id'])
Ed25519PublicKey.from_public_bytes(hex_bytes(public_key, 32)).verify(wire[-64:], digest)
require(wire[-96:-64] == hex_bytes(actual['keyId'], 32), 'batch and receipt key identifiers differ')
authority = sequence_authority(height, positions[1], positions[2], previous, resulting, tx_root,
                               block['transactions'], transaction, expected_denom)
require(int(actual['seq'], 16) == int.from_bytes(canonical[42:50], 'big') == authority['sequence'],
        'receipt sequence differs from independent batch position')
reconcile_ledger(disclosure, landed['logs'])
source = bytes.fromhex(disclosure['source'])
asset = bytes.fromhex(disclosure['asset'])
sequence = rpc('eth_call', [{'to': disclosure['to'],
    'data': '0xce2f921f' + (source + asset).hex()}, 'latest'])
if int(sequence, 16) != int(disclosure['source_account_sequence']) + 1:
    raise SystemExit('source account sequence did not advance independently')
nonce = rpc('eth_getTransactionCount', [disclosure['from'], 'latest'])
if int(nonce, 16) <= int(disclosure['evm_nonce']):
    raise SystemExit('EVM sender nonce did not advance')
(root / 'a2a-receipt.hex').write_text(canonical.hex())
(root / 'a2a-receipt-authority.json').write_text(json.dumps(authority))
A2A_RECEIPT
"$LAYERX_BIN" --json receipt verify --receipt "$journey_root/a2a-receipt.hex" \
  --batch-id "$(jq -er '.batch_id' "$journey_root/a2a-receipt-authority.json")" \
  --asset "$(jq -er '.asset' "$journey_root/a2a-receipt-authority.json")" \
  --previous-state-root "$(jq -er '.previous_state_root' "$journey_root/a2a-receipt-authority.json")" \
  --resulting-state-root "$(jq -er '.resulting_state_root' "$journey_root/a2a-receipt-authority.json")" \
  --sequencer-public-key "$LAYERX_SEQUENCER_PUBLIC_KEY" \
  | jq -e --arg activity "${a2a_transaction#0x}" \
    '.data.verified == true and .data.activity_id == $activity and .data.result_code == 0' >/dev/null

"$LAYERX_BIN" --json a2a status | jq -e '.data.state == "running"' >/dev/null
"$LAYERX_BIN" --json a2a stop | jq -e '.data.state == "stopped"' >/dev/null

jq -nrc --slurpfile mcp "$mcp_payment" --arg a2a "$a2a_transaction" \
  '"INSTALL_JOURNEY_RESULT " + ({tests: 7, skipped: 0, mcp_activity: $mcp[0].activity_id, a2a_transaction: $a2a} | tojson)'

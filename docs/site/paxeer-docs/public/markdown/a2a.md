<!-- Source: https://docs.paxeer.app/a2a/ -->

# Agent-to-Agent Payments

Discover a payment agent, submit structured tasks and reconcile each outcome with Paxeer X protocol evidence.

## A payment agent with explicit boundaries

The A2A transport exposes a JSON-RPC payment agent bound to one hosted environment. Its agent card declares served skills and enforced gateway scopes. The runtime never signs: a payment is a caller-signed EIP-1559 transaction that the runtime forwards, and verified receipts establish the monetary outcome.

## Install the runtime

```
layerx environment use beta
layerx auth set --environment beta
layerx install a2a --environment beta --key operator
layerx a2a start
layerx a2a status
```

Use an already configured beta profile and an existing key named operator. The identity session is read from stdin by `auth set`. Installation provisions a gateway key, stores its secret in credential storage, and starts the managed runtime. Payment mode grants `activity:write` and `receipt:read`. The optional `--source-account` and `--asset` flags must be supplied together as 32-byte hex identifiers. The submitted transaction determines the payment contents. The listener is restricted to loopback and defaults to port `9433`.

### Read-only and credential rotation

```
layerx install a2a --environment beta --read-only
layerx a2a stop
# Rotate the installed credentials.
layerx install a2a --environment beta --key operator --rotate
```

Read-only installation grants `receipt:read`. A separate installation-local Bearer protects incoming task requests. It lives in the owner-only authorization file; the hosted gateway credential remains in secure credential storage. Agent-card discovery is public on the loopback listener.

## Discover the agent card

```
GET /.well-known/agent-card.json
# Compatibility discovery path:
GET /.well-known/agent.json
```

The card uses A2A protocol version `0.3.0` and preferred transport `JSONRPC`. It includes URL, software version, JSON input/output modes, skill descriptors, the `layerxLocalBearer` security scheme and an environment extension containing environment, deployment mode and scopes. Read this card before choosing a skill.

| Skill | Inputs | Authority and evidence |
| --- | --- | --- |
| `receipt.get` | `activity_id`, a `0x`-prefixed 64-hex transaction hash. | Requires receipt read scope; fetched via `pax_sequencerReceipt`. Verification requires independently pinned receipt keys; the gateway does not verify it for you. |
| `activity.submit` | `raw_transaction` only: a caller-signed EIP-1559 transaction as `0x02…` hex. | Requires activity write scope; submitted unchanged via `eth_sendRawTransaction`. The runtime constructs and signs nothing. |
| `faucet.request` | DID and public key. | Requires an explicit hosted faucet endpoint through --faucet-endpoint on a2a serve and a stored identity session for the active environment. |

## Send a structured task

```
curl --http1.1 "$A2A_ENDPOINT" \
  -H "Authorization: Bearer $A2A_AUTHORIZATION" \
  -H "Content-Type: application/json" \
  --data-binary ' {"jsonrpc":"2.0","id":1,"method":"message/send","params":{
  "message":{
    "kind":"message","role":"user","messageId":"receipt-check-01",
    "contextId":"treasury-reconciliation",
    "parts":[{"kind":"data","data":{
      "skill":"receipt.get","arguments":{"activity_id":"0x<64-hex-transaction-hash>"}
    }}]
  }
}}'
```

Set the example shell variables to the installed listener URL and its local authorization credential. Curl supplies the Host and Content-Length headers. HTTP requests carry an explicit Content-Length and one JSON body. A data part contains `skill` and `arguments`; a text part may contain the same object serialized as JSON. The server admits only skills in its card. It generates task and artifact IDs and retains the submitted message in task history.

### Payment arguments

```
{"skill":"activity.submit","arguments":{
  "raw_transaction":"0x02<caller-signed-eip1559-transaction-hex>"
}}
```

`raw_transaction` is the sole argument. There are no destination, amount, account-sequence, validity-window, fee-limit, or idempotency-key arguments: destination, amount, nonce, and fees are whatever the caller signed into the transaction. The returned hash must match the raw transaction, and the caller retains it for receipt lookup.

## Track task state and protocol state

```
{"jsonrpc":"2.0","id":2,"method":"tasks/get","params":{"id":"<task-id>"}}
{"jsonrpc":"2.0","id":3,"method":"tasks/cancel","params":{"id":"<task-id>"}}
```

| Task state | Meaning | Next step |
| --- | --- | --- |
| `completed` | The skill returned its result. | Inspect the artifact and verify the monetary outcome evidence. |
| `submitted` | Gateway state is acknowledged or pending. | Retain the existing transaction hash and reconcile its receipt. |
| `unknown` | The gateway outcome is uncertain. | Look up the original transaction before submitting another write. |
| `rejected` | The gateway refused the operation or skill execution failed. | Read the task status message and resolve its cause. |

The task artifact carries the skill result as a data part. A completed transport task does not independently establish final settlement. Submitted protocol activities cannot be canceled by this transport; terminal tasks also cannot be canceled. The runtime persists a bounded task ledger and retains unresolved transaction identities across restarts. When started with a matching receipt policy through --receipt-policy, it reconciles pending tasks against verified receipts at startup. Preserve transaction hashes and receipt evidence for application records.

## Capabilities, errors and policy

The card declares streaming, push notifications and transition-history capabilities as false for this JSON-RPC runtime. Use `tasks/get` and receipt reads. Unsupported streaming, resubscription, and push-configuration requests return `-32004`, as does `tasks/cancel` on a submitted or unknown task; canceling a terminal task returns `-32002`. Unknown methods or unserved skills return `-32601`; invalid arguments return `-32602`; missing tasks return `-32001`. Malformed JSON returns `-32700`, a request without a method returns `-32600`, and internal failures return `-32603`. Task-ledger admission refusals return `-32005`. Missing or incorrect incoming Bearer credentials return HTTP 401.

Use owner policy, capability scope and [budget and approval controls](https://docs.paxeer.app/agent-budgets) to bound agent spending. MCP routes tool calls through an enrolled daemon binding; this A2A runtime forwards caller-signed transactions under its installed gateway credential. See [API authority and commitment levels](https://docs.paxeer.app/platform-api), [identity](https://docs.paxeer.app/identity) and [CLI workflows](https://docs.paxeer.app/cli).

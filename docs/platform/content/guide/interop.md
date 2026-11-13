# Interop

The interop plane translates external protocols into typed requests and verification results. Its payload authorities construct LayerX payload bytes; adapters retain responsibility for their protocol's cryptography and validation.

## Adapters

| Adapter | Boundary |
| --- | --- |
| `layerx-x402` | x402 v2 buyer, seller and facilitator |
| `layerx-ap2` | AP2 mandate verification and signed evidence |
| `layerx-ucp` | UCP checkout and order translation |
| `layerx-visa-tap` | Visa Trusted Agent Protocol HTTP message signatures |
| `layerx-fiat` | Fiat provider tokens |
| `layerx-migrate` | Ethereum and Solana migration |
| `layerx-mirror` | Ethereum and Solana batch-archive publication |

An `AdapterDescriptor` identifies the adapter, pinned specification and declared conformance suite. A descriptor alone does not establish conformance; consult each adapter's compatibility documentation for supported coverage.

## Receipt export and evidence import

`PortableReceiptV2` carries exact signed receipt bytes, a receipt digest, key metadata, a height and optional raw transaction bytes. Verification uses independently pinned, height-ranged receipt keys. It can run offline. The exported level is `receipted`, including when the producer supplied a committed answer; proving BFT commit requires additional consensus evidence.

`verify_external_evidence` binds an adapter's typed verification result to the exact presentation bytes and pinned specification. The caller selects and trusts the adapter and supplies its verification context. The binding digest alone does not authenticate a report from another process.

See the [portable verification guide](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/docs/interop/PORTABILITY.md) for the format and public API.

## The gateway

The interop workspace contains `layerx-interop-gateway` and `layerx-interop-service`. Its declared core boundary is `layerx-agent-api`. The gateway carries principals, traces and translation requests, classifies translation status and redacts sensitive fields.

## x402

The HTTP headers are `PAYMENT-REQUIRED` on a `402` response, `PAYMENT-SIGNATURE` on the retry and `PAYMENT-RESPONSE` on the settlement response. The Rust crate exposes `Buyer`, `Seller` and `Facilitator` roles. x402 carries offers and payment evidence; the underlying network transaction performs settlement. Consumers verify the receipt and match its payment facts to the offer.

## MCP

MCP uses an authenticated agent-daemon binding and serves the daemon's tool catalogue over standard input and output:

```sh
layerx install mcp --host claude-code --daemon-binding ./daemon-binding.json
layerx mcp serve --daemon-binding ./daemon-binding.json
```

The supported host names are `layerx`, `claude-code`, `claude-desktop`, `cursor` and `vscode`. The daemon holds the credentials. `--read-only` filters write tools from the catalogue; a read-only binding also enforces that restriction. The MCP installation does not provision a hosted gateway key.

## A2A

A2A publishes an agent card and task interface through a managed local runtime. It uses a configured hosted environment and key:

```sh
layerx install a2a --environment beta --key agent-runtime \
  --source-account <64-hex-funded-account> --asset <64-hex-asset>
layerx a2a status
```

Installation accepts `--token-stdin` for hosted key provisioning and `--rotate` for replacement. The runtime uses an operating-system credential-store alias. `layerx a2a start` and `layerx a2a stop` control the installed runtime.

| Tool | Input and behavior |
| --- | --- |
| `activity.submit` | Only `raw_transaction`: signed EIP-1559 transaction hex. Validates the signature, sender and chain, then calls `eth_sendRawTransaction`. |
| `receipt.get` | Only `activity_id`: the transaction hash. Retrieves `pax_sequencerReceipt` material for independent verification. |
| `faucet.request` | Available only with an explicitly configured faucet endpoint. |

`--read-only` removes write tools. A2A does not construct or sign a payment from model-supplied destination and amount fields. Submitted, pending and unknown outcomes do not establish a completed payment; receipt and commitment verification remain separate checks.

## Enforced by

| Capability | Layer | What that means here |
| --- | --- | --- |
| Interop adapters hold no protocol authority | `protocol` | Payload authorities construct LayerX payload bytes; adapters translate at the boundary. |
| Offline receipt verification | `protocol` | Signed receipt evidence is checked against independently pinned receipt keys. |
| Atomic settlement | `protocol` | A network payment applies completely or not at all. |
| Replay refusal | `protocol` | An applied signed transaction cannot apply again with the same sender nonce. |
| Quote then commit | `service` | The x402 buyer accepts an offer before paying it. |
| Idempotent money moves | `service` | Request identity supports reconciliation with the original payment. |
| MCP binding and catalogue access | `agent-layer` | MCP uses an authenticated daemon binding and filters write tools in read-only mode. |
| A2A transaction validation and local bearer checks | `service` | The A2A runtime validates signed transaction inputs and authenticates local requests. |

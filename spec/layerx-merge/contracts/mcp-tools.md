# MCP tools contract

The [MCP server](../../../services/agent/crates/layerx-mcp/src/server.rs) exposes tools described by its [catalogue](../../../services/agent/crates/layerx-mcp/src/catalogue.rs). Tool scope checks and session binding precede daemon operations. Prepared calls, signed submission and verified receipts are separate stages. Paxeer mainnet-beta uses chain ID `125` (`0x7d`); finality follows block commitment.

## activity-sign

`activity.sign` with `external_signature_v1` requires a 65-byte signature. A 64-byte signature is refused with `mcp.external_signature`. The [binding decoder](../../../services/agent/crates/layerx-mcp/src/binding.rs) validates the external signature before forwarding the request.

## activity-submit

The catalogue declares separate prepared and raw-transaction submission shapes. Use the schema for the selected variant. Native external signing uses a 65-byte signature and a 20-byte signer address. A prepared submission and a raw signed EVM transaction are not interchangeable inputs.

## approval-get

`approval.get` returns the approval's `tx_hash` and `rung`. These values can be null before signing or receipt availability.

## balance-get

`balance.get` reports ordinary-read freshness through chain-head and value-height fields. An ordinary balance read has no transaction rung.

## binding-document

Session bindings use `core_height`. `core_sequence` is refused with `mcp.binding_retired_key`. A web binding supplies `receipt_key_index`, whose selected key must cover the receipt height; `sequencer_public_key` is refused. See [binding.rs](../../../services/agent/crates/layerx-mcp/src/binding.rs).

## faucet-request

`faucet.request` requires `write:faucet:claim` and calls `px_requestFunds` for the session address. It returns the funding transaction hash and maps to no ordinary agent operation. Method availability follows the configured endpoint; hosted methods are declared in [the gateway contract](../../../services/platform/hosted/gateway/openrpc.json).

## grant-draw

`native_alias_target` maps `grant.draw` to Asset module ordinal `6` at `0x101A`. This is an alias mapping, not a payment authorization guarantee: the ledger's retained draw selector refuses execution. The customer budget payment path uses `budgetSpend` at `0x101B`; see [pay](pay.md).

## receipt-get

`receipt.get` reports the receipt's achieved rung and block height. A signed execution receipt establishes `receipted`; block commitment establishes finality. Receipt verification selects the published key by its identifier and inclusive height range.

## subscription-acknowledge

`subscription.acknowledge` takes a cursor with `block_height`, `tx_index` and `log_index`. A single integer cursor is refused.

## wallet-send

The catalogue maps native aliases as follows. Call preparation still passes through the daemon's authority and execution checks.

| Tool | Asset ordinal | Target |
|---|---|---|
| `wallet.send`, `token.transfer` | 5 | `0x101A` |
| `token.create` | 1 | `0x101A` |
| `token.mint` | 10 | `0x101A` |
| `grant.issue` | 7 | `0x101A` |
| `grant.draw` | 6 | `0x101A` |

Retained grant aliases do not make retired ledger grant activities callable. The [ledger contract](ledger-auth.md) describes the served methods and refusal behavior.

## web-fetch

[ExactTerms::prepare_pay_exact](../../../services/agent/crates/layerx-mcp/src/tools/web.rs) constructs `payExact(bytes32,bytes32,bytes32,uint128,uint64,bytes32)` with `PrecompileTarget::Ledger`, at `0x101A`. It binds payer account, recipient, asset, amount, expiry in milliseconds and payment reference. [The production payer](../../../services/agent/crates/layerx-agentd-host/src/payer/implementation.rs) signs the EVM transaction and verifies receipt signatures and logs against its configured receipt-key set before reporting payment.

# SDK quickstarts

One page per shipped SDK. Each shows the real register, faucet, send,
and verify-receipt surface of that package — or states that a step
is not on that surface.

Keep these “register” / “fund” / “send” planes distinct:

| Plane | What it is | Where it lives |
| --- | --- | --- |
| Hosted public operations | `px_register`, `px_requestFunds` (Bearer session) | configured hosted gateway; [Public JSON-RPC](PublicRpc.md) |
| Native node JSON-RPC | `eth_sendRawTransaction`, `pax_sequencerReceipt`, `eth_getTransactionReceipt`; `px_getAccount`, `px_getBalances`, `px_getNetwork` | node root (`/`) |
| Agent operations | `agent.register`, `prepare` / `submit` | generated agent catalogs |
| Human plane | `move.quote` / `move.commit` | [Human journeys](HumanJourneys.md) |

`agent.register` registers an agent through the agent plane. Public
self-service principal registration uses hosted `px_register`.

| Language | Package | Register | Faucet | Send | Verify receipt |
| --- | --- | --- | --- | --- | --- |
| [Rust](SdkQuickstartRust.md) | `layerx-sdk` (`services/agent/crates/layerx-sdk`) | `RpcClient::register` | `RpcClient::request_funds` (see auth note) | `RpcClient::send_raw_transaction` | `production::verify_receipt` |
| [TypeScript](SdkQuickstartTypeScript.md) | `@sidiora/layerx-sdk` | not on `JsonRpcClient` | not on `JsonRpcClient` | `JsonRpcClient.sendRawTransaction` or `ProductionClient.human` | `verifySequencerReceipt`, then `verifyCommittedReceipt` |
| [Python](SdkQuickstartPython.md) | `layerx-sdk` | `HostedGatewayRpc.call("px_register", …)` | `HostedGatewayRpc.call("px_requestFunds", …)` | `PaymentRpc.send` (signed transaction) | `verify_sequencer_receipt` |
| [Go](SdkQuickstartGo.md) | `sdk/platform/go` | catalog only | HTTP `px_requestFunds` | HTTP `eth_sendRawTransaction` | `VerifySequencerReceipt` |
| [JVM](SdkQuickstartJvm.md) | `com.sidiora.layerx:layerx-sdk` | catalog only | HTTP `px_requestFunds` | HTTP `eth_sendRawTransaction` | `LocalVerifier.verifySequencerReceipt` |
| [Swift](SdkQuickstartSwift.md) | `LayerXSDK` | catalog only | HTTP `px_requestFunds` | HTTP `eth_sendRawTransaction` | `LocalVerifier.verifySequencerReceipt` |
| [.NET](SdkQuickstartDotNet.md) | `LayerX.Sdk` | catalog only | HTTP `px_requestFunds` | `HumanMoveQuoteAsync` / `HumanMoveCommitAsync` | `LocalVerifier.VerifySequencerReceipt` |

Rust uses `get_balances` / `get_node_info` for `px_getBalances` /
`px_getNetwork`. Python's `PxClient` exposes `get_account`, `get_balances`
and `get_network` for node reads; `PaymentRpc.send`, `receipt` and `status`
use the native transaction and receipt methods above. Go, JVM and Swift
use direct HTTP for these node methods because their platform packages
have no public JSON-RPC client. A submission hash is not a verified receipt.

## Fund from faucet

Send `px_requestFunds` with params `[did, signer_public_key]` to the
configured hosted gateway, with `Authorization: Bearer <identity session token>`.
The signer public key is 32 bytes encoded as lowercase hex. Rust's
`RpcClient::request_funds` names the operation but cannot construct that
Bearer session header; use the HTTP example on its page. Python uses
`HostedGatewayRpc.call("px_requestFunds", …)` from `layerx_sdk.x402_rpc`
with session headers and the configured hosted gateway endpoint. Go, JVM and
Swift show the HTTP request directly.
See [Hosted faucet](HostedFaucet.md) for the hosted service setup.

There is no Rust tree under `sdk/platform`. There is no TypeScript or
Python tree under `sdk/platform`. Go, JVM, Swift, and .NET ship only
under `sdk/platform`.

TypeScript and Python's `AgentHttpTransport` route Programs operations.
Swift's `AgentHTTPTransport` refuses calls with `unavailable-capability`;
`AgentEnvelopeTransport` provides its agent-envelope transport. Go provides
`NewAgentEnvelopeHTTPTransport` for hosted agent operations. Select a transport
that implements the required plane and the deployment's authentication contract.

[Home](Home.md)

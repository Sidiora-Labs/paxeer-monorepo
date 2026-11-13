<!-- Source: https://docs.paxeer.app/xdk/ -->

# XDK developer toolkit

Build Paxeer X applications in seven languages with native activities, human journeys, agent tools, exchange integrations, and local proof verification.

## Choose your integration plane

XDK is the Paxeer X developer toolkit across native activities, payments, Programs, exchange operations, identity, and verifiable settlement. Package and import names retain `layerx`; use the exact language identifiers below in your application.

| Plane | Use it for | Entry points |
| --- | --- | --- |
| Public JSON-RPC | Principal registration, raw-transaction submission, balances, receipts, proofs, and subscriptions. The retired `lx_*` names return JSON-RPC `-32601` Method not found. | `px_register`, `px_requestFunds`, `eth_sendRawTransaction` |
| Human API | User journeys with a quote, explicit approval, and an idempotent commit. | `move.quote` / `move.commit` |
| Agent API | Daemon registration, policy-bound preparation, external signing, submission, and Programs. | `agent.register`, `prepare`, `submit`, `program.*` |
| MCP | Tool calls from assistants through the same agent daemon gates. | [22 catalogue tools, plus 11 session tools in Full mode (33 total)](https://docs.paxeer.app/mcp) |
| Offline verification | Independent receipt, outcome, terminal, and transfer-root checks. | Language-specific local verifiers |

Platform SDKs verify sequencer receipts: pass the signed receipt, the raw transaction, and trusted receipt keys; the result reaches the `receipted` or `committed` rung. The retired kernel `AuthorizedBatch` verifiers and the `executed`/`batched`/`finalised` rungs no longer exist in the platform SDKs. Only the Rust `layerx-sdk` still exposes `production::verify_receipt` for that model.

## Seven language SDKs

| Language | Package | Payment entry point | Receipt verifier |
| --- | --- | --- | --- |
| Rust | `layerx-sdk` | `RpcClient::send_raw_transaction` | `production::verify_receipt` |
| TypeScript | `@sidiora/layerx-sdk` | `JsonRpcClient.sendRawTransaction`; `ProductionClient.human` | `verifySequencerReceipt` |
| Python | `layerx-sdk` | `PaymentRpc.send`; `ProductionClient.human` | `verify_sequencer_receipt` |
| Go | `github.com/Sidiora-Labs/LayerX-Network/sdk/platform/go` | `Client.Human` | `VerifySequencerReceipt` |
| JVM | `com.sidiora.layerx:layerx-sdk` | `ProductionClient.human` | `LocalVerifier.verifySequencerReceipt` |
| Swift | `LayerXSDK` | `humanMoveQuote` / `humanMoveCommit` | `LocalVerifier.verifySequencerReceipt` |
| .NET | `LayerX.Sdk` | `HumanMoveQuoteAsync` / `HumanMoveCommitAsync` | `LocalVerifier.VerifySequencerReceipt` |

## Native and application API families

| Family | Developer surface |
| --- | --- |
| Native activities | Canonical encoders, intent construction, account derivation, fee estimation, sequencing, and signed activity submission. |
| Programs | Program lifecycle, native program calls, transfer authority, call graphs, runtime terminals, and occupancy settlement. |
| Spot and perpetuals | `encodeSpotActivity`, `decodeSpotActivity`, `encodePerpsActivity`, and `decodePerpsActivity` construct and inspect native trading payloads. |
| Exchange | ABI helpers and event decoders for exchange operations. Confirm the exact helper names in the SDK source for your language before use. |
| Bridge | `bridgeInCall`, `bridgeOutCall`, and `decodeBridgeEvent` bind cross-network transfer requests to the bridge ABI. |
| Launchpad | Launchpad market operations. Confirm the exact operations exposed by your language SDK in its source before use. |
| Streams and mirrors | Streaming and mirror surfaces; see [streams](https://docs.paxeer.app/streams) and [mirrors](https://docs.paxeer.app/mirrors) for the documented behavior. |
| XWeb and agent services | XWeb API, web search, native capabilities, gas-station support, and policy-aware transport interfaces. |

## Credentials, amounts, and retries

Use Bearer identity sessions for authenticated human journeys and faucet claims. Public activity submission uses a `LayerX-Key` credential with `activity:write`. These credentials authorize different operations. Keep secret material in `SecretBytes` or the language credential wrapper, and keep signing outside application logs.

Use protocol integer amounts or decimal strings rather than floating point values. Attach a stable idempotency key to a business action and preserve it across retries. A submission result can be pending; poll or subscribe until the requested commitment is reached, then verify the receipt.

## Transport selection

The generated operation catalog defines names and schemas; the transport determines the reachable plane. TypeScript, Python, and Swift agent HTTP transports route Programs operations. Go provides a human HTTP transport. JVM transport accepts separate human and agent origins. Supply an agent-capable transport for daemon operations and use public JSON-RPC for self-service principal registration.

Start with [SDK quickstarts](https://docs.paxeer.app/sdk-quickstarts), then continue to [MCP tools](https://docs.paxeer.app/mcp), [Programs](https://docs.paxeer.app/programs), and [public JSON-RPC](https://docs.paxeer.app/public-rpc).

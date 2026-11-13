<!-- Source: https://docs.paxeer.app/xdk/ -->

# XDK developer toolkit

Build Paxeer X applications in seven languages with native activities, human journeys, agent tools, exchange integrations, and local proof verification.

## Choose your integration plane

XDK is the Paxeer X developer toolkit across native activities, payments, Programs, exchange operations, identity, and verifiable settlement. Package and import names use `layerx`; use the exact language identifiers below in your application.

| Plane | Use it for | Entry points |
| --- | --- | --- |
| Node JSON-RPC | Signed EVM transaction submission, balances and receipt reads at the node root path. The `eth_*` namespace provides Ethereum methods; the EVM chain ID is `125`. | `eth_sendRawTransaction`, `eth_getBalance`, `pax_sequencerReceipt` |
| Human API | User journeys with a quote, explicit approval, and an idempotent commit. | `move.quote` / `move.commit` |
| Agent API | Daemon registration, policy-bound preparation, external signing, submission, and Programs. | `agent.register`, `prepare`, `submit`, `program.*` |
| MCP | Tool calls from assistants through the same agent daemon gates. | [22 catalogue tools, plus 11 session tools in Full mode (33 total)](https://docs.paxeer.app/mcp) |
| Offline verification | Independent signed-receipt and block-commitment evidence checks. | Language-specific local verifiers |

The hosted gateway in `services/platform/hosted/gateway` serves `px_register` and `px_requestFunds`. Platform SDK receipt verifiers bind signed receipt bytes to the raw transaction and independently trusted receipt keys. That signature check reaches `receipted`; `committed` verification also requires authenticated block and inclusion evidence. Paxeer is one BFT network, and a receipt is final when its block is committed. Rust `layerx-sdk` also exposes `production::verify_receipt` with an independently supplied `AuthorizedBatch`.

## Seven language SDKs

| Language | Package | Payment entry point | Receipt verifier |
| --- | --- | --- | --- |
| Rust | `layerx-sdk` in services/agent/crates/layerx-sdk | `RpcClient::send_raw_transaction` | `production::verify_receipt` |
| TypeScript | `@sidiora/layerx-sdk` in services/agent/sdk/typescript | `JsonRpcClient.sendRawTransaction`; `ProductionClient.human` | `verifySequencerReceipt` |
| Python | `layerx-sdk` in services/agent/sdk/python | `PaymentRpc.send`; `ProductionClient.human` | `verify_sequencer_receipt` |
| Go | `sdk/platform/go` | `Client.Human` | `VerifySequencerReceipt` |
| JVM | `com.sidiora.layerx:layerx-sdk` in sdk/platform/jvm | `ProductionClient.human` | `LocalVerifier.verifySequencerReceipt` |
| Swift | `LayerXSDK` in sdk/platform/swift | `humanMoveQuote` / `humanMoveCommit` | `LocalVerifier.verifySequencerReceipt` |
| .NET | `LayerX.Sdk` in sdk/platform/dotnet | `HumanMoveQuoteAsync` / `HumanMoveCommitAsync` | `LocalVerifier.VerifySequencerReceipt` |

## Native and application API families

| Family | Developer surface |
| --- | --- |
| Native activities | Canonical payload encoders, account derivation, fee estimation and signed EVM transaction submission. |
| Programs | Program lifecycle, native program calls, transfer authority, call graphs, runtime terminals, and occupancy settlement. |
| Spot and perpetuals | `encodeSpotActivity`, `decodeSpotActivity`, `encodePerpsActivity`, and `decodePerpsActivity` construct and inspect TypeScript native trading payloads. |
| Exchange | ABI helpers and event decoders for exchange operations. Confirm the exact helper names in the SDK source for your language before use. |
| Bridge | `bridgeInCall`, `bridgeOutCall`, and `decodeBridgeEvent` construct TypeScript bridge calls and decode bridge events. |
| Launchpad | Launchpad market operations. Confirm the exact operations exposed by your language SDK in its source before use. |
| Streams and mirrors | Streaming and mirror surfaces; see [streams](https://docs.paxeer.app/streams) and [mirrors](https://docs.paxeer.app/mirrors) for the documented behavior. |
| XWeb and agent services | XWeb API, web search, native capabilities, gas-station support, and policy-aware transport interfaces. |

## Credentials, amounts, and retries

Use Bearer identity sessions for authenticated human journeys. Hosted gateway activity submission uses a `LayerX-Key` credential with `activity:write`. Node JSON-RPC submits signed EVM transactions. Choose authentication for the service handling the operation. Keep secret material in `SecretBytes` or the language credential wrapper, and keep signing outside application logs.

Use protocol integer amounts or decimal strings rather than floating point values. Attach a stable idempotency key to a business action and preserve it across retries. A submission result can be pending; poll or subscribe until the requested commitment is reached, then verify the receipt.

## Transport selection

The generated operation catalog defines names and schemas; the transport determines the reachable plane. Human operations use their HTTP routes, and agent transports use the daemon envelope. Programs clients submit mutations as signed EVM transactions and read program state through node JSON-RPC. Go provides human and agent envelope HTTP transports; JVM transport accepts separate human and agent origins. Use the hosted gateway for principal registration and funding requests.

Start with [SDK quickstarts](https://docs.paxeer.app/sdk-quickstarts), then continue to [MCP tools](https://docs.paxeer.app/mcp), [Programs](https://docs.paxeer.app/programs), and [public JSON-RPC](https://docs.paxeer.app/public-rpc).

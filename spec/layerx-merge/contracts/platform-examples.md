# Platform examples contract

This page describes the [platform examples](../../../services/platform/examples) and their configuration, transaction submission and receipt verification.

The examples use one Paxeer network: `paxd` is the BFT node, its integrated kernel proposes blocks, and the EVM chain ID is `125` (`0x7d`, payment network `eip155:125`). A receipt is final when its block commits. Sub-chains are tenant domains in the database, not additional execution or settlement infrastructure.

Node JSON-RPC uses the configured root URL. Hosted `px_` methods have their own [gateway OpenRPC contract](../../../services/platform/hosted/gateway/openrpc.json); availability depends on the selected endpoint.

The explorer's [verifier HTTP contract](verifier-http.md) describes source compilation and bytecode comparison. It does not replace the signed transaction receipt verification described here.

## examples-config

Each application has a `layerx.example.json` with `version: 1`, its `application` name, and `environments.emulator` and `environments.beta`. [loadApplicationConfig](../../../services/platform/examples/support/runtime.mjs) checks the version and application, then selects the environment from `--environment emulator` or `--environment beta`. Optional `--action` accepts `deploy`, `list` or `buy`; otherwise the action is `run`.

| Application config | Receipt-key variable for emulator | Receipt-key variable for beta |
|---|---|---|
| [buyer-agent](../../../services/platform/examples/buyer-agent/layerx.example.json) | `LAYERX_EMULATOR_RECEIPT_KEYS_JSON` | `LAYERX_TESTNET_RECEIPT_KEYS_JSON` |
| [marketplace](../../../services/platform/examples/marketplace/layerx.example.json) | `LAYERX_EMULATOR_RECEIPT_KEYS_JSON` | `LAYERX_TESTNET_RECEIPT_KEYS_JSON` |
| [merchant-checkout](../../../services/platform/examples/merchant-checkout/layerx.example.json) | `LAYERX_EMULATOR_RECEIPT_KEYS_JSON` | `LAYERX_TESTNET_RECEIPT_KEYS_JSON` |
| [merchant-shop](../../../services/platform/examples/merchant-shop/layerx.example.json) | `LAYERX_EMULATOR_RECEIPT_KEYS_JSON` | `LAYERX_TESTNET_RECEIPT_KEYS_JSON` |
| [paid-api](../../../services/platform/examples/paid-api/layerx.example.json) | `LAYERX_RECEIPT_KEYS_JSON` | `LAYERX_RECEIPT_KEYS_JSON` |

All five configs name `network: "eip155:125"` and supply `receiptKeysEnvironment`. Merchant environments require the network, RPC URL and pinned-key binding. `receiptAuthorityUrl` and `protocolVersion` are not configuration fields.

Select a configuration pointing to the local node root. `rpcUrl` identifies the node listener for the selected environment; a configured URL does not establish that a matching listener is running.

`rpcUrl` must support the methods used by the application. `SequencerReceiptClient` calls `pax_sequencerReceipt`; transaction submission elsewhere uses `eth_sendRawTransaction`, and the marketplace and merchant hash adapters need `eth_getRawTransactionByHash`. A node's chain ID alone does not prove those receipt methods are available.

The buyer's `humanUrl` is a hosted payment API origin, distinct from `rpcUrl`. [buyerHumanUrl(config)](../../../services/platform/examples/buyer-agent/index.mjs) requires HTTPS at the root path for beta and refuses public node hostnames or the node's origin. Set the configured `humanUrlEnvironment` (`LAYERX_TESTNET_HUMAN_URL`) for the deployed hosted API. Merchant `settlementUrl` is an application payment endpoint, not another chain or a later consensus-finality step; merchant-shop beta uses `settlementUrlEnvironment` (`LAYERX_TESTNET_SETTLEMENT_URL`). A checked-in URL does not prove endpoint deployment.

## examples-environment

The shared [runtime](../../../services/platform/examples/support/runtime.mjs) reads these bindings:

| Binding | Behavior |
|---|---|
| `LAYERX_EXAMPLE_CONFIG` | Optional config filename, resolved relative to the application directory. |
| `LAYERX_EXAMPLE_ENDPOINT` | Optional beta endpoint base. `applyEndpointOverride(environment, config)` rebases existing `humanUrl`, `rpcUrl`, `settlementUrl` and `endpoint` fields while preserving their paths. Emulator configs are unchanged. |
| `LAYERX_EXAMPLE_STATE_ROOT` | Optional root for application state, partitioned by `config.application`. Otherwise state resolves under the application directory. |
| The name in `receiptKeysEnvironment` | Required JSON array of pinned receipt keys, decoded by `receiptKeysFromEnvironment(name)`. |

An endpoint override selects an origin without discovering hosted services or rewriting variable-supplied endpoints. For the buyer, an override without a dedicated hosted human URL is refused. Use a complete config for the deployed topology.

`requiredEnvironment(name)` rejects invalid binding names and missing or empty values; `optionalEnvironment(name)` returns `undefined` for absent or empty values. Names must match `^[A-Z][A-Z0-9_]{0,127}$`. Receipt key entries contain 32-byte hex `keyId` and `publicKey`, plus inclusive `firstHeight` and `lastHeight`. Heights accept nonnegative safe integer numbers, canonical decimal strings or canonical lowercase hex strings, bounded by unsigned 64-bit range. Inverted ranges fail with `invalid_receipt_key_range`. The decoder produces `Uint8Array` keys and bigint `first_height`/`last_height` values for the SDK.

`secureUrl(value)` rejects credentials, query strings and fragments. It accepts HTTPS and permits HTTP for the implementation's loopback allowlist; use a loopback origin for local examples. `applicationStatePath(config, relative)` rejects empty, absolute and parent-traversing relative paths. `diagnosticText(value, secrets, limit)` defaults to 1024 characters and redacts supplied secrets of at least eight characters.

`LAYERX_PROTOCOL_VERSION`, `applicationProtocolVersion`, `SELECTABLE_PROTOCOL_VERSIONS` and the old `ReceiptAuthorityClient` are retired example interfaces. The shared runtime uses pinned receipt keys and signed raw transactions rather than a protocol-version selector or receipt-authority URL.

## examples-js-exports

| Module | Implemented exports and purpose |
|---|---|
| [support/runtime.mjs](../../../services/platform/examples/support/runtime.mjs) | `LayerXApplicationStateError(state, detail)`; `requiredEnvironment`, `optionalEnvironment`, `loadApplicationConfig`, `applicationStatePath`, `applyEndpointOverride`; `SequencerReceiptClient`, `receiptKeysFromEnvironment`; `secureUrl`, `secureBaseUrl`, `exactObject`, `hex32`, `equalBytes`, `diagnosticText`, `commandPath`. |
| [buyer-agent/index.mjs](../../../services/platform/examples/buyer-agent/index.mjs) | `platform_ref_buyer()`, `buyerHumanUrl(config)`, `runBuyerAgent(config)`. |
| [paid-api/index.mjs](../../../services/platform/examples/paid-api/index.mjs) | `platform_ref_seller()`, `FileFulfillmentRepository`, `createPaidApiServer(middleware, resourceBody)`, `runPaidApiApplication()`. |
| [support/merchant-app.mjs](../../../services/platform/examples/support/merchant-app.mjs) | `platform_ref_merchant()`, `MerchantCheckout`, `DeclaredCatalog`, `FileOrders`, `FileFulfillments`, `createMerchantApplicationServer(config)`, `runMerchantApplication(moduleUrl, application)`. |
| [marketplace/index.mjs](../../../services/platform/examples/marketplace/index.mjs) | `platform_ref_marketplace()`; the module also loads config and runs the selected application action. |
| [marketplace/receipt-outcome.mjs](../../../services/platform/examples/marketplace/receipt-outcome.mjs) | `verifyProgramOutcome(output, client)`, `programReceiptOutcome(output, verified, rung)`. |
| [marketplace/deploy-payload.mjs](../../../services/platform/examples/marketplace/deploy-payload.mjs) | `canonicalNativeProgramDeploy({ programId, abiVersion, codeHash, wasm })`, `accountNameForDid(did)`, `readLifecycleAnchor({...})`, `marketplaceCallRequest({...})`, `u128(value)`. Constants: `IMMUTABLE_POLICY = 0`, `LIFECYCLE_WINDOW_MS = 300_000n`, `IDENTITY_SEQUENCE_SELECTOR = "pending"`, `LIST_OPERATION = 1`, `BUY_OPERATION = 2`. |
| [agent-spend/services.mjs](../../../services/platform/examples/agent-spend/services.mjs) | `createAgentServices(environment)`, `ownerWalletHandoff()`, `preparedTransactionReference(prepared)`. The last binds the preparation reference to serialized unsigned EIP-1559 bytes. |
| [support/artifacts.mjs](../../../services/platform/examples/support/artifacts.mjs) | `referenceArtifacts(path, root)` validates the private artifact manifest, source revision and artifact hashes. |

### Raw transaction receipt contract

The implemented signature is `new SequencerReceiptClient(rpcUrl, receiptKeys, transportLimits = {}).resolve(rawTransaction)`. Signed transaction bytes are required to verify the receipt's binding. Input is a nonempty `Uint8Array` or a lowercase `0x`-prefixed, even-length hex string; the string decoder allows at most 1,048,576 decoded bytes.

Receipt lookups use `boundedJsonRequest` with a default `timeoutMs` of **10,000 milliseconds** and `maxResponseBytes` of **4,194,304 bytes**. Either transport limit can be reduced to a positive safe integer, up to its default ceiling. Requests reject redirects, oversized responses and invalid UTF-8 or JSON.

The client computes `keccak256(rawTransaction)`, posts `pax_sequencerReceipt` with that hash as its sole parameter, checks the JSON-RPC version, ID and matching `result.txHash`, then calls `verifySequencerReceipt` with the original bytes and pinned keys. Keys are copied on construction. A transaction hash alone cannot substitute for signed bytes.

| Receipt response or failure | Observable result |
|---|---|
| `rung: "submitted"` | State `pending`, detail `sequencer_receipt_submitted`. |
| `rung: "receipted"` or `"committed"`, valid signature and raw binding | Proof with `format: "layerx-receipt-proof-v2"` and `verificationLevel: "receipted"`. |
| JSON-RPC error | State `refused`, detail `sequencer_receipt_refused`. |
| No pinned key verifies | State `refused`, detail `sequencer_receipt_verification_failed`. |
| Fetch or JSON decode failure | State `unknown`, detail `sequencer_receipt_unreachable`. |
| Transaction hash mismatch | State `unknown`, detail `sequencer_receipt_mismatch`. |
| Other rung | State `unknown`, detail `invalid_sequencer_receipt_rung`. |

Failure states are carried by `LayerXApplicationStateError`. A proof also contains `rawTransaction`, `signedReceipt`, `receiptDigest` (Keccak-256 of signed receipt bytes), and the verifying `receiptKey` with hex `keyId`, `publicKey`, `firstHeight` and `lastHeight`. Even for a server response of `committed`, this helper reports `receipted`; it does not independently prove block commitment.

For a lookup using an actual signed transaction and independently pinned keys, run an ES module from the repository root after installing/building the example dependencies. Supply `PAXEER_RPC_URL` with the node root URL, `LAYERX_SIGNED_TRANSACTION` with the exact signed transaction hex and `LAYERX_TESTNET_RECEIPT_KEYS_JSON` with the trusted key array. This example only reads a receipt:

```js
import assert from "node:assert/strict";
import { verifySequencerReceipt } from "@sidiora/layerx-sdk";
import {
  SequencerReceiptClient,
  receiptKeysFromEnvironment,
  requiredEnvironment,
} from "./services/platform/examples/support/runtime.mjs";

const keys = receiptKeysFromEnvironment("LAYERX_TESTNET_RECEIPT_KEYS_JSON");
const raw = requiredEnvironment("LAYERX_SIGNED_TRANSACTION");
const client = new SequencerReceiptClient(
  requiredEnvironment("PAXEER_RPC_URL"),
  keys,
);
const proof = await client.resolve(raw);
const bytes = (hex) => Uint8Array.from(Buffer.from(hex.slice(2), "hex"));
await verifySequencerReceipt(bytes(proof.signedReceipt), bytes(raw), keys);
assert.equal(proof.rawTransaction, raw);
assert.equal(proof.verificationLevel, "receipted");
```

### Marketplace receipt and lifecycle adapters

`verifyProgramOutcome(output, client)` finds a transaction hash in CLI output, retrieves raw bytes with `eth_getRawTransactionByHash`, checks their Keccak-256 hash, and calls `client.resolve(raw)`. It verifies the proof again before interpreting `resultCode`: `0` becomes `completed`, `1` becomes `refused`, and other codes fail as `program_receipt_invalid_status`. Without a hash it preserves explicit `pending`, `unknown` or `refused` output, otherwise it raises `program_response_omitted_transaction_hash` with state `unknown`.

`canonicalNativeProgramDeploy` accepts guest ABI versions `1`, `2` and `3`, uses immutable policy and zero authority, and round-trips through the SDK native deployment encoder/decoder. `readLifecycleAnchor` reads the latest block's `stateRoot` and timestamp and `eth_getTransactionCount(evmAddress, "pending")`, converts seconds to milliseconds and adds the 300,000 ms lifecycle window. These are reads of the same chain. `marketplaceCallRequest` encodes list/buy operations and their required capabilities; `u128` requires a positive decimal price within unsigned 128-bit range.

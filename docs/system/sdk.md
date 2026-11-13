# SDK surfaces

Paxeer is one BFT network, run by `paxd`, with the kernel participating in block proposal. Mainnet-beta uses EVM chain id **125** (`0x7d`). A transaction is final when its block is committed. Sub-chains are tenant domains in the database, launched without separate infrastructure.

Public node JSON-RPC uses the root URL; select a node from the [public RPC reference](../site/docs/reference/public-rpc.md). The hpx library mirror distributes build libraries. Hosted gateway APIs have their own configured URL; their `px_` contract is [openrpc.json](../../services/platform/hosted/gateway/openrpc.json).

This reference covers the wallet package, the agent TypeScript and Python SDKs, and the Rust program guest SDK. Export availability is an inventory of source declarations; it does not establish that a backend is configured or that a transport exchange has been verified. See also [EVM JSON-RPC methods](rpc.md), [precompile ABIs](precompiles.md), and the [system index](INDEX.md).

## Wallet (`sdk/wallet`)

Package [`@paxeer/wallet`](../../sdk/wallet/package.json) has three public entry points:

| Import | Source | Surface |
|---|---|---|
| `@paxeer/wallet` | [`src/index.ts`](../../sdk/wallet/src/index.ts) | `PaxeerWallet`, EVM review types, and re-exports of the modules below |
| `@paxeer/wallet/provider` | [`src/provider.ts`](../../sdk/wallet/src/provider.ts) | `PaxeerProvider`, `PaxeerProviderConfig`, `TokenSupplier`, `ConfirmRequest`, `BindingRefusedError`, `PAXEER_CHAIN_ID`, `LEDGER_PRECOMPILE_LOG_FLOOR`, `ledgerLogFloor` |
| `@paxeer/wallet/react` | [`src/react.ts`](../../sdk/wallet/src/react.ts) | Optional React hooks and their result/options interfaces; React 18 or later is a peer dependency |

### Wallet authentication and signing

`PaxeerWallet` requires `apiUrl`, `supabaseUrl`, and `supabaseAnonKey`; `fetch` is optional. `signInWithEmail` starts the magic-link flow, `signInWithOAuth` starts the OAuth flow, and `getSession`, `getUser`, `onAuthStateChange`, and `signOut` expose the authentication lifecycle. Authenticated wallet API calls carry the session's bearer token.

`getStandardSelf()` reads `GET /v1/wallet/me` and returns `null` on HTTP 404. `provisionStandardWallet()` explicitly calls `POST /v1/wallet/provision`. `getWallet()` composes those operations and provisions when needed. `signTransaction`, `sendTransaction`, and `signMessage` use the configured wallet API. `sendTransaction` validates the response and authenticates an included receipt, or reads one from the returned chain's `rpc_url`.

`TxRequest` accepts decimal strings or `bigint` for value and gas quantities; value is in wei. `SendTxResponse` includes `tx_hash`, `address`, `chain_id`, and an optional `receipt`. A transaction hash or an absent receipt does not establish commitment. API failures use `PaxeerWalletError` with `code`, `status`, and `detail`; a missing session raises `NO_SESSION` with status 401.

EVM review uses `EvmReviewWalletConfig` with `evmReview.storage` implementing `EvmReviewStorage.getItem` and `setItem`. `reviewEvmTransaction` requires an unsigned EIP-1559 transaction with a destination and checks the returned review against its transaction and principal. `approveEvmTransaction` requires the retained original review; `evmApprovalStatus` reconciles the server artifact. The exported `EvmTransactionReview` and `EvmApproval` types describe these artifacts, including `signing_unknown` recovery state.

Funded accounts are retired. The retained funded methods reject with `FundedUnsupportedError` (`FUNDED_UNSUPPORTED`, HTTP status 410); funded types and React hooks remain compatibility exports.

### Provider and wallet discovery

`PaxeerProvider` implements `Eip1193Provider`. Configure `gatewayUrl`, `rpcUrl`, and a `token` supplier; optional keys are `chainId` (default 125), `confirm`, and `fetch`. `gatewayUrl` is the wallet API base, while `rpcUrl` is the node URL. `eth_requestAccounts` obtains or provisions the embedded account. Signing requests use the authenticated gateway and recheck account/session state around confirmation. Read requests are proxied to the node unless the provider handles the method locally.

The provider supports `eth_sendTransaction`, `personal_sign`, `eth_signTypedData_v4`, and the wallet's receipted/sponsored operations. `eth_sign` accepts the supported digest constructions rather than arbitrary signing input. `eth_signTransaction`, older typed-data variants, `wallet_addEthereumChain`, and the retired `paxeer_*Custody` methods raise `UnsupportedMethodError`.

Provider events include account, chain, connection, and disconnection changes. `ProviderRpcError` subclasses preserve machine-readable codes: user rejection 4001, unauthorized 4100, unsupported method 4200, disconnected 4900, chain disconnected 4901, invalid parameters -32602, and internal error -32603. `GatewayRefusalError` also retains the gateway reason, HTTP status, and response body.

`announceProvider`, `discoverProviders`, `validateProviderInfo`, and `install` provide EIP-6963 discovery. `install` announces the provider and sets `window.paxeer` when a browser window exists. `WalletInterface` and `walletInterfaces` expose embedded and injected wallets through common account, chain, transaction, message, and typed-data methods.

### Reads, receipts, and application helpers

The following are selected exports from the root entry point; each linked source contains its associated option, response, and error types.

| Source | Exports and behavior |
|---|---|
| [`rpc.ts`](../../sdk/wallet/src/rpc.ts) | `JsonRpcClient`, `JsonRpcError`, `JsonRpcTransportError`, `decodeResponse`, `parseReceiptKeyResponse`; JSON-RPC calls and batches |
| [`endpoint.ts`](../../sdk/wallet/src/endpoint.ts) | `EndpointClient`, `DEFAULT_ENDPOINT_URL`, `EndpointClientOptions`, account/balance/history decoders, `balanceOfCalldata` |
| [`kernel.ts`](../../sdk/wallet/src/kernel.ts) | `KernelClient`, `KernelAvailability`, `kernelUnavailableFrom`, `KERNEL_UNAVAILABLE_CODE`, `DEFAULT_KERNEL_AVAILABILITY_TTL_MS`, and budget/grant snapshot types |
| [`human.ts`](../../sdk/wallet/src/human.ts) | `HumanClient`, `HumanServiceError`, `HumanDecodeError`, journey and intent decoders; plan, submit, and journey reads |
| [`receipt.ts`](../../sdk/wallet/src/receipt.ts) | `readReceiptKeyPins`, `decodeSequencerReceipt`, `authenticateSequencerReceipt`, `readSequencerReceipt`, `waitForSequencerReceipt`, `ReceiptTimeoutError` |
| [`ladder.ts`](../../sdk/wallet/src/ladder.ts) | `LADDER_RUNGS`, `statusLadder`, `decodeExplorerStatus`, `readExplorerStatus`, `submittedTransfer`, `recoverTransferObservation`, `observeTransfer` |
| [`agent.ts`](../../sdk/wallet/src/agent.ts) | `agentPublicKey`, `agentDid`, `agentRequestMethod`, `agentRequestDigest`, `signAgentRequest`, `bindingMessage`, `signBinding`, `agentClaimDigest`, `buildClaim`, `claimAgent` |
| [`modules/index.ts`](../../sdk/wallet/src/modules/index.ts) | `ModuleProvider`, transaction/event helpers, `listHolderRecords`, and the module factories below |
| [`types.ts`](../../sdk/wallet/src/types.ts) | Wallet configuration, transaction, account, asset, history, intent, receipt, EIP-1193 and availability types; `assertSendTxResponse`, `PaxeerWalletError`, `FundedUnsupportedError` |
| [`errors.ts`](../../sdk/wallet/src/errors.ts) | `PROVIDER_ERROR_CODES`, `ProviderRpcError` subclasses, `GATEWAY_REFUSAL_REASONS`, `gatewayRefusal` |

`EndpointClient` accepts `nodeUrl` (or `url`) and optional `hostedUrl`, plus `fetch` and `headers`. An omitted node URL uses `DEFAULT_ENDPOINT_URL`. `px_` reads go to the node root; only `px_register` and `px_requestFunds` require `hostedUrl`. Account, asset, balance, capabilities, network and history methods, including `resolveAccount`, `getChainAccount`, `listBankAssets`, `getCapabilities`, `getNetwork`, `getBankBalances`, `getChainHistory`, `historyPages`, and `history`, use the node.

`EndpointClient.batch()` follows the same split: `px_register` and `px_requestFunds` go to the configured hosted gateway; all other methods, including `px_getReceiptKeys`, go to the node. Results retain the original call order. Without `hostedUrl`, a batch containing either hosted operation throws `JsonRpcTransportError`. Its legacy error message refers to hosted reads, but read-only batches do not require `hostedUrl`.

`KernelAvailability` checks the network consensus state through the node and caches successful availability observations for 5,000 ms by default. `KernelClient` returns `KernelRead<T>` with either `available: true` and a result, or `available: false` and a reason. Structured JSON-RPC error -32010 with `data.code = "kernel_unavailable"` can carry `not_configured`, `unreachable`, or `consensus_unavailable`. Other errors propagate. An unavailable read must not be presented as an empty balance or a successful transaction.

`HumanClient` takes `url`, `kernel`, optional `fetch`, and an optional `authorization` supplier. It plans at `POST /v1/intents/plan`, submits at `POST /v1/intents/submit` with an idempotency key, and reads `GET /v1/journeys/{id}`. Submission checks kernel availability first.

`pax_sequencerReceipt` and the `SequencerReceipt*` identifiers are retained API names, not a separate execution network. `readSequencerReceipt` authenticates returned receipt data with governed keys obtained through `px_getReceiptKeys`. `waitForSequencerReceipt` takes a target `rung`, bounded `attempts`, `intervalMs`, and optional abort `signal`; exhaustion raises `ReceiptTimeoutError`. The status ladder is `submitted` → `receipted` → `committed`. Journey `done` maps to `receipted`, while `done-finalised` maps to `committed`; callers must distinguish progress from commitment and inspect execution status for success.

Module factories exported by the wallet are [`exchange`](../../sdk/wallet/src/modules/exchange.ts), [`bridge`](../../sdk/wallet/src/modules/bridge.ts), [`launchpad`](../../sdk/wallet/src/modules/launchpad.ts), [`feeToken`](../../sdk/wallet/src/modules/fee-token.ts), [`webData`](../../sdk/wallet/src/modules/web-data.ts), [`gasStation`](../../sdk/wallet/src/modules/gas-station.ts), and [`feeChoice`](../../sdk/wallet/src/modules/fee-choice.ts). They provide module-specific calls, encoding, and response/event decoding. Use the named address constants and ABI definitions in those sources when constructing calls. `sendModuleTransaction` returns a transaction hash; `sendModuleTransactionReceipted` additionally requests receipt evidence.

The React entry point exports `useSession`, `useWallet`, `usePaxeerWallet`, and `useStandardAccount`. `useWallet` uses the auto-provisioning `getWallet`; `useStandardAccount` performs a passive read and exposes `refresh`. The compatibility hooks `useFundedTiers` and `useFundedAccount` surface the retired funded API's errors.

## TypeScript (`services/agent/sdk/typescript`)

Package [`@sidiora/layerx-sdk`](../../services/agent/sdk/typescript/package.json), entry point [`src/index.ts`](../../services/agent/sdk/typescript/src/index.ts). The table follows that entry point's declarations and re-exports. The `aiMarket` namespace re-exports `ai_market.ts`. Agent transport, receipt verification, native program calls, intent/history, account derivation, and precompile helpers are distinct surfaces within this package; exporting an operation does not demonstrate live backend wiring.

The generated Cosmos bindings in [`sdk/js`](../../sdk/js/package.json) are the separate `@pax-js/cosmos` package, with `types/*`, `rest`, and `encoding` entry points.

| Export | Kind | Defined in |
|---|---|---|
| `ACCOUNT_DERIVATION_DOMAIN_NAME` | const | `services/agent/sdk/typescript/src/account-derivation.ts` |
| `ACCOUNT_DERIVATION_DOMAIN_VERSION` | const | `services/agent/sdk/typescript/src/account-derivation.ts` |
| `ACCOUNT_DERIVATION_HKDF_INFO_PREFIX` | const | `services/agent/sdk/typescript/src/account-derivation.ts` |
| `ACCOUNT_DERIVATION_HKDF_SALT` | const | `services/agent/sdk/typescript/src/account-derivation.ts` |
| `ACCOUNT_DERIVATION_ORIGIN` | const | `services/agent/sdk/typescript/src/account-derivation.ts` |
| `ACCOUNT_DERIVATION_PURPOSE` | const | `services/agent/sdk/typescript/src/account-derivation.ts` |
| `ACCOUNT_DERIVATION_VERSION` | const | `services/agent/sdk/typescript/src/account-derivation.ts` |
| `ADDR_PRECOMPILE_ADDRESS` | const | `services/agent/sdk/typescript/src/account-derivation.ts` |
| `AccountDerivationError` | class | `services/agent/sdk/typescript/src/account-derivation.ts` |
| `AccountDerivationErrorCode` | type | `services/agent/sdk/typescript/src/account-derivation.ts` |
| `BindCall` | interface | `services/agent/sdk/typescript/src/account-derivation.ts` |
| `BindFees` | interface | `services/agent/sdk/typescript/src/account-derivation.ts` |
| `BindPlan` | type | `services/agent/sdk/typescript/src/account-derivation.ts` |
| `DerivedAccount` | interface | `services/agent/sdk/typescript/src/account-derivation.ts` |
| `Eip1193Requester` | interface | `services/agent/sdk/typescript/src/account-derivation.ts` |
| `KeyDerivationParameters` | interface | `services/agent/sdk/typescript/src/account-derivation.ts` |
| `KeyDerivationTypedData` | interface | `services/agent/sdk/typescript/src/account-derivation.ts` |
| `LAYERX_COIN_TYPE` | const | `services/agent/sdk/typescript/src/account-derivation.ts` |
| `LayerXBindState` | interface | `services/agent/sdk/typescript/src/account-derivation.ts` |
| `WalletRequest` | interface | `services/agent/sdk/typescript/src/account-derivation.ts` |
| `autoBindLayerX` | function | `services/agent/sdk/typescript/src/account-derivation.ts` |
| `bindNonceCall` | function | `services/agent/sdk/typescript/src/account-derivation.ts` |
| `bindTransaction` | function | `services/agent/sdk/typescript/src/account-derivation.ts` |
| `bindTransactionRequest` | function | `services/agent/sdk/typescript/src/account-derivation.ts` |
| `boundDidCall` | function | `services/agent/sdk/typescript/src/account-derivation.ts` |
| `checksumAddress` | function | `services/agent/sdk/typescript/src/account-derivation.ts` |
| `decodeBindNonce` | function | `services/agent/sdk/typescript/src/account-derivation.ts` |
| `decodeBoundDid` | function | `services/agent/sdk/typescript/src/account-derivation.ts` |
| `decodeLayerXDid` | function | `services/agent/sdk/typescript/src/account-derivation.ts` |
| `deriveFromBrowserWallet` | function | `services/agent/sdk/typescript/src/account-derivation.ts` |
| `deriveFromMnemonic` | function | `services/agent/sdk/typescript/src/account-derivation.ts` |
| `deriveFromSeed` | function | `services/agent/sdk/typescript/src/account-derivation.ts` |
| `deriveFromWalletSignature` | function | `services/agent/sdk/typescript/src/account-derivation.ts` |
| `keyDerivationHash` | function | `services/agent/sdk/typescript/src/account-derivation.ts` |
| `keyDerivationRequest` | function | `services/agent/sdk/typescript/src/account-derivation.ts` |
| `keyDerivationTypedData` | function | `services/agent/sdk/typescript/src/account-derivation.ts` |
| `layerXBindMessage` | function | `services/agent/sdk/typescript/src/account-derivation.ts` |
| `layerXDidCall` | function | `services/agent/sdk/typescript/src/account-derivation.ts` |
| `normalizeWalletSignature` | function | `services/agent/sdk/typescript/src/account-derivation.ts` |
| `planLayerXBind` | function | `services/agent/sdk/typescript/src/account-derivation.ts` |
| `readLayerXBindState` | function | `services/agent/sdk/typescript/src/account-derivation.ts` |
| `requireLayerXBinding` | function | `services/agent/sdk/typescript/src/account-derivation.ts` |
| `sendBoundLayerXCall` | function | `services/agent/sdk/typescript/src/account-derivation.ts` |
| `signBindTransaction` | function | `services/agent/sdk/typescript/src/account-derivation.ts` |
| `slip10Ed25519` | function | `services/agent/sdk/typescript/src/account-derivation.ts` |
| `AGENT_DAEMON_RPC_PATH` | const | `services/agent/sdk/typescript/src/agent-http.ts` |
| `AGENT_ENVELOPE_OPERATION_NAMES` | const | `services/agent/sdk/typescript/src/agent-http.ts` |
| `AGENT_ENVELOPE_PATH` | const | `services/agent/sdk/typescript/src/agent-http.ts` |
| `AGENT_ENVELOPE_VERSION` | const | `services/agent/sdk/typescript/src/agent-http.ts` |
| `AgentEnvelopeSuccess` | interface | `services/agent/sdk/typescript/src/agent-http.ts` |
| `AgentEnvelopeSurface` | type | `services/agent/sdk/typescript/src/agent-http.ts` |
| `AgentEnvelopeTransport` | class | `services/agent/sdk/typescript/src/agent-http.ts` |
| `AgentEnvelopeTransportOptions` | interface | `services/agent/sdk/typescript/src/agent-http.ts` |
| `AgentHttpTransport` | class | `services/agent/sdk/typescript/src/agent-http.ts` |
| `AgentHttpTransportOptions` | interface | `services/agent/sdk/typescript/src/agent-http.ts` |
| `AgentSessionCredential` | class | `services/agent/sdk/typescript/src/agent-http.ts` |
| `LayerXKeyCredential` | class | `services/agent/sdk/typescript/src/agent-http.ts` |
| `ProgramBoundaryError` | class | `services/agent/sdk/typescript/src/agent-http.ts` |
| `ProofBundleRead` | interface | `services/agent/sdk/typescript/src/agent-http.ts` |
| `ProofBundleRecord` | type | `services/agent/sdk/typescript/src/agent-http.ts` |
| `ProofBundleRequest` | interface | `services/agent/sdk/typescript/src/agent-http.ts` |
| `ProofBundleVerificationLevel` | type | `services/agent/sdk/typescript/src/agent-http.ts` |
| `RetiredDaemonTransportOptions` | interface | `services/agent/sdk/typescript/src/agent-http.ts` |
| `RetiredOperationError` | class | `services/agent/sdk/typescript/src/agent-http.ts` |
| `TenantReadinessV1` | interface | `services/agent/sdk/typescript/src/agent-http.ts` |
| `TenantRecoveryReasonV1` | type | `services/agent/sdk/typescript/src/agent-http.ts` |
| `checkProofBundleResponse` | function | `services/agent/sdk/typescript/src/agent-http.ts` |
| `checkTenantReadinessResponse` | function | `services/agent/sdk/typescript/src/agent-http.ts` |
| `decodeAgentEnvelopeResponse` | function | `services/agent/sdk/typescript/src/agent-http.ts` |
| `decodeEnvelope` | function | `services/agent/sdk/typescript/src/agent-http.ts` |
| `decodeProgramBoundaryError` | function | `services/agent/sdk/typescript/src/agent-http.ts` |
| `decodeProofBundleRecord` | function | `services/agent/sdk/typescript/src/agent-http.ts` |
| `decodeTenantReadiness` | function | `services/agent/sdk/typescript/src/agent-http.ts` |
| `encodeAgentEnvelope` | function | `services/agent/sdk/typescript/src/agent-http.ts` |
| `encodeProgramMutationBody` | function | `services/agent/sdk/typescript/src/agent-http.ts` |
| `encodeProofBundleRequest` | function | `services/agent/sdk/typescript/src/agent-http.ts` |
| `BRIDGE_EVENTS` | const | `services/agent/sdk/typescript/src/bridge.ts` |
| `BridgeInAttestation` | interface | `services/agent/sdk/typescript/src/bridge.ts` |
| `LAYERX_BRIDGE_PRECOMPILE` | const | `services/agent/sdk/typescript/src/bridge.ts` |
| `bridgeInCall` | function | `services/agent/sdk/typescript/src/bridge.ts` |
| `bridgeOutCall` | function | `services/agent/sdk/typescript/src/bridge.ts` |
| `decodeBridgeEvent` | function | `services/agent/sdk/typescript/src/bridge.ts` |
| `sendBridgeIn` | const | `services/agent/sdk/typescript/src/bridge.ts` |
| `sendBridgeOut` | const | `services/agent/sdk/typescript/src/bridge.ts` |
| `CHAINS_EVENTS` | const | `services/agent/sdk/typescript/src/chains.ts` |
| `CHAINS_METHODS` | const | `services/agent/sdk/typescript/src/chains.ts` |
| `ChainInfo` | interface | `services/agent/sdk/typescript/src/chains.ts` |
| `ChainsArguments` | interface | `services/agent/sdk/typescript/src/chains.ts` |
| `ChainsClient` | class | `services/agent/sdk/typescript/src/chains.ts` |
| `ChainsParams` | interface | `services/agent/sdk/typescript/src/chains.ts` |
| `CreateChainOptions` | interface | `services/agent/sdk/typescript/src/chains.ts` |
| `DecodedChainsEvent` | interface | `services/agent/sdk/typescript/src/chains.ts` |
| `LAYERX_CHAINS_PRECOMPILE` | const | `services/agent/sdk/typescript/src/chains.ts` |
| `chainRpcUrl` | function | `services/agent/sdk/typescript/src/chains.ts` |
| `createChainCall` | function | `services/agent/sdk/typescript/src/chains.ts` |
| `decodeChainsEvent` | function | `services/agent/sdk/typescript/src/chains.ts` |
| `decodeChainsParams` | function | `services/agent/sdk/typescript/src/chains.ts` |
| `decodeGetChain` | function | `services/agent/sdk/typescript/src/chains.ts` |
| `encodeChainsCall` | function | `services/agent/sdk/typescript/src/chains.ts` |
| `virtualChainId` | function | `services/agent/sdk/typescript/src/chains.ts` |
| `CometProof` | interface | `services/agent/sdk/typescript/src/committed-proof.ts` |
| `CommittedReceiptInclusion` | interface | `services/agent/sdk/typescript/src/committed-proof.ts` |
| `cometCommitHash` | function | `services/agent/sdk/typescript/src/committed-proof.ts` |
| `cometValidatorsHash` | function | `services/agent/sdk/typescript/src/committed-proof.ts` |
| `unwrapCometEvmTransaction` | function | `services/agent/sdk/typescript/src/committed-proof.ts` |
| `verifyCometProof` | function | `services/agent/sdk/typescript/src/committed-proof.ts` |
| `verifyReceiptInclusion` | function | `services/agent/sdk/typescript/src/committed-proof.ts` |
| `Eip1559AccessListEntry` | interface | `services/agent/sdk/typescript/src/evm-transaction.ts` |
| `Eip1559Transaction` | interface | `services/agent/sdk/typescript/src/evm-transaction.ts` |
| `SignedEip1559Transaction` | interface | `services/agent/sdk/typescript/src/evm-transaction.ts` |
| `decodeRawEip1559` | function | `services/agent/sdk/typescript/src/evm-transaction.ts` |
| `eip1559SigningHash` | function | `services/agent/sdk/typescript/src/evm-transaction.ts` |
| `encodeUnsignedEip1559` | function | `services/agent/sdk/typescript/src/evm-transaction.ts` |
| `keccak256` | function | `services/agent/sdk/typescript/src/evm-transaction.ts` |
| `recoverEip1559Sender` | function | `services/agent/sdk/typescript/src/evm-transaction.ts` |
| `AbiArgument` | type | `services/agent/sdk/typescript/src/exchange.ts` |
| `DecodedPrecompileEvent` | interface | `services/agent/sdk/typescript/src/exchange.ts` |
| `EXCHANGE_EVENTS` | const | `services/agent/sdk/typescript/src/exchange.ts` |
| `ExchangePlaceOrder` | type | `services/agent/sdk/typescript/src/exchange.ts` |
| `LAYERX_EXCHANGE_PRECOMPILE` | const | `services/agent/sdk/typescript/src/exchange.ts` |
| `PrecompileAbiError` | class | `services/agent/sdk/typescript/src/exchange.ts` |
| `PrecompileAbiErrorCode` | type | `services/agent/sdk/typescript/src/exchange.ts` |
| `PrecompileAbiType` | type | `services/agent/sdk/typescript/src/exchange.ts` |
| `PrecompileAbiValue` | type | `services/agent/sdk/typescript/src/exchange.ts` |
| `PrecompileCall` | interface | `services/agent/sdk/typescript/src/exchange.ts` |
| `PrecompileEventFields` | type | `services/agent/sdk/typescript/src/exchange.ts` |
| `PrecompileEventInput` | interface | `services/agent/sdk/typescript/src/exchange.ts` |
| `PrecompileEventSpec` | interface | `services/agent/sdk/typescript/src/exchange.ts` |
| `PrecompileLog` | interface | `services/agent/sdk/typescript/src/exchange.ts` |
| `PrecompileWrite` | re-export | `services/agent/sdk/typescript/src/exchange.ts` |
| `abiEventTopic` | function | `services/agent/sdk/typescript/src/exchange.ts` |
| `abiSelector` | function | `services/agent/sdk/typescript/src/exchange.ts` |
| `abiSignature` | function | `services/agent/sdk/typescript/src/exchange.ts` |
| `decodeEventFrom` | function | `services/agent/sdk/typescript/src/exchange.ts` |
| `decodeEventWithSpec` | function | `services/agent/sdk/typescript/src/exchange.ts` |
| `decodeExchangeEvent` | function | `services/agent/sdk/typescript/src/exchange.ts` |
| `encodeAbiCall` | function | `services/agent/sdk/typescript/src/exchange.ts` |
| `exchangeCancelOrderCall` | function | `services/agent/sdk/typescript/src/exchange.ts` |
| `exchangeDepositMarginCall` | function | `services/agent/sdk/typescript/src/exchange.ts` |
| `exchangeDepositMarginTokenCall` | function | `services/agent/sdk/typescript/src/exchange.ts` |
| `exchangePlaceOrderCall` | function | `services/agent/sdk/typescript/src/exchange.ts` |
| `exchangeRequestSettlementCall` | function | `services/agent/sdk/typescript/src/exchange.ts` |
| `exchangeWithdrawMarginCall` | function | `services/agent/sdk/typescript/src/exchange.ts` |
| `precompileEventSignature` | function | `services/agent/sdk/typescript/src/exchange.ts` |
| `precompileTransactionRequest` | function | `services/agent/sdk/typescript/src/exchange.ts` |
| `precompileWriteCall` | function | `services/agent/sdk/typescript/src/exchange.ts` |
| `sendExchangeCancelOrder` | const | `services/agent/sdk/typescript/src/exchange.ts` |
| `sendExchangeDepositMargin` | const | `services/agent/sdk/typescript/src/exchange.ts` |
| `sendExchangeDepositMarginToken` | const | `services/agent/sdk/typescript/src/exchange.ts` |
| `sendExchangePlaceOrder` | const | `services/agent/sdk/typescript/src/exchange.ts` |
| `sendExchangeRequestSettlement` | const | `services/agent/sdk/typescript/src/exchange.ts` |
| `sendExchangeWithdrawMargin` | const | `services/agent/sdk/typescript/src/exchange.ts` |
| `sendPrecompileCall` | function | `services/agent/sdk/typescript/src/exchange.ts` |
| `Eip7702Authorization` | interface | `services/agent/sdk/typescript/src/gas-station.ts` |
| `GAS_RESPONSE_LIMITS` | const | `services/agent/sdk/typescript/src/gas-station.ts` |
| `GAS_STATION_QUOTE_URL` | const | `services/agent/sdk/typescript/src/gas-station.ts` |
| `GasQuote` | interface | `services/agent/sdk/typescript/src/gas-station.ts` |
| `GasQuoteRequest` | interface | `services/agent/sdk/typescript/src/gas-station.ts` |
| `GasRefusal` | interface | `services/agent/sdk/typescript/src/gas-station.ts` |
| `GasRefusalCode` | type | `services/agent/sdk/typescript/src/gas-station.ts` |
| `GasResult` | type | `services/agent/sdk/typescript/src/gas-station.ts` |
| `GasStationConfig` | interface | `services/agent/sdk/typescript/src/gas-station.ts` |
| `GasSubmissionIdentity` | interface | `services/agent/sdk/typescript/src/gas-station.ts` |
| `GasSubmissionOutcome` | type | `services/agent/sdk/typescript/src/gas-station.ts` |
| `GasSubmissionStatus` | interface | `services/agent/sdk/typescript/src/gas-station.ts` |
| `GasSubmissionTransaction` | interface | `services/agent/sdk/typescript/src/gas-station.ts` |
| `NativeFeePreference` | type | `services/agent/sdk/typescript/src/gas-station.ts` |
| `SIDIORA_DECIMALS` | const | `services/agent/sdk/typescript/src/gas-station.ts` |
| `SIDIORA_TOKEN` | const | `services/agent/sdk/typescript/src/gas-station.ts` |
| `SignedEip7702Authorization` | interface | `services/agent/sdk/typescript/src/gas-station.ts` |
| `SignedGasQuote` | interface | `services/agent/sdk/typescript/src/gas-station.ts` |
| `SponsoredBatch` | interface | `services/agent/sdk/typescript/src/gas-station.ts` |
| `SponsoredConsent` | interface | `services/agent/sdk/typescript/src/gas-station.ts` |
| `SponsoredGasBatchStatus` | type | `services/agent/sdk/typescript/src/gas-station.ts` |
| `SponsoredGasSubmission` | interface | `services/agent/sdk/typescript/src/gas-station.ts` |
| `assembleEip7702Authorization` | function | `services/agent/sdk/typescript/src/gas-station.ts` |
| `discoverSponsoredNonce` | function | `services/agent/sdk/typescript/src/gas-station.ts` |
| `eip7702AuthorizationDigest` | function | `services/agent/sdk/typescript/src/gas-station.ts` |
| `gasQuoteDigest` | function | `services/agent/sdk/typescript/src/gas-station.ts` |
| `gasReceiptDenominations` | function | `services/agent/sdk/typescript/src/gas-station.ts` |
| `gasStationQuoteUrl` | function | `services/agent/sdk/typescript/src/gas-station.ts` |
| `readNativeFeePreference` | function | `services/agent/sdk/typescript/src/gas-station.ts` |
| `requestGasQuote` | function | `services/agent/sdk/typescript/src/gas-station.ts` |
| `requestGasSubmissionStatus` | function | `services/agent/sdk/typescript/src/gas-station.ts` |
| `requestSponsoredGasBatchStatus` | function | `services/agent/sdk/typescript/src/gas-station.ts` |
| `retryGasSubmission` | function | `services/agent/sdk/typescript/src/gas-station.ts` |
| `sponsoredBatchCall` | function | `services/agent/sdk/typescript/src/gas-station.ts` |
| `sponsoredBatchDigest` | function | `services/agent/sdk/typescript/src/gas-station.ts` |
| `sponsoredConsent` | function | `services/agent/sdk/typescript/src/gas-station.ts` |
| `submitSponsoredGasBatch` | function | `services/agent/sdk/typescript/src/gas-station.ts` |
| `APPROVAL_CONTRACT_INTRODUCED` | const | `services/agent/sdk/typescript/src/generated/client.ts` |
| `APPROVAL_DECISION_OUTCOMES` | const | `services/agent/sdk/typescript/src/generated/client.ts` |
| `APPROVAL_ENFORCEMENT_NOTICE` | const | `services/agent/sdk/typescript/src/generated/client.ts` |
| `APPROVAL_EVENT_KINDS` | const | `services/agent/sdk/typescript/src/generated/client.ts` |
| `APPROVAL_STATES` | const | `services/agent/sdk/typescript/src/generated/client.ts` |
| `AddressBytes` | type | `services/agent/sdk/typescript/src/generated/client.ts` |
| `Amount` | type | `services/agent/sdk/typescript/src/generated/client.ts` |
| `ApiError` | interface | `services/agent/sdk/typescript/src/generated/client.ts` |
| `ApprovalApproveRequest` | interface | `services/agent/sdk/typescript/src/generated/client.ts` |
| `ApprovalDecision` | interface | `services/agent/sdk/typescript/src/generated/client.ts` |
| `ApprovalDecisionOutcome` | type | `services/agent/sdk/typescript/src/generated/client.ts` |
| `ApprovalEventKind` | type | `services/agent/sdk/typescript/src/generated/client.ts` |
| `ApprovalGetRequest` | interface | `services/agent/sdk/typescript/src/generated/client.ts` |
| `ApprovalLifecycleEvent` | interface | `services/agent/sdk/typescript/src/generated/client.ts` |
| `ApprovalListRequest` | interface | `services/agent/sdk/typescript/src/generated/client.ts` |
| `ApprovalPage` | interface | `services/agent/sdk/typescript/src/generated/client.ts` |
| `ApprovalRecord` | interface | `services/agent/sdk/typescript/src/generated/client.ts` |
| `ApprovalRejectRequest` | interface | `services/agent/sdk/typescript/src/generated/client.ts` |
| `ApprovalState` | type | `services/agent/sdk/typescript/src/generated/client.ts` |
| `BudgetLimit` | type | `services/agent/sdk/typescript/src/generated/client.ts` |
| `Client` | class | `services/agent/sdk/typescript/src/generated/client.ts` |
| `ErrorClass` | type | `services/agent/sdk/typescript/src/generated/client.ts` |
| `HoldReason` | interface | `services/agent/sdk/typescript/src/generated/client.ts` |
| `IdempotentMutation` | interface | `services/agent/sdk/typescript/src/generated/client.ts` |
| `NativeActivityV1` | interface | `services/agent/sdk/typescript/src/generated/client.ts` |
| `NativeApprovalDecisionV1` | interface | `services/agent/sdk/typescript/src/generated/client.ts` |
| `NativeApprovalListResultV1` | interface | `services/agent/sdk/typescript/src/generated/client.ts` |
| `NativeApprovalResultV1` | interface | `services/agent/sdk/typescript/src/generated/client.ts` |
| `NativeLocalGrantV1` | interface | `services/agent/sdk/typescript/src/generated/client.ts` |
| `NativePrepareRequestV1` | interface | `services/agent/sdk/typescript/src/generated/client.ts` |
| `NativePrepareResultV1` | interface | `services/agent/sdk/typescript/src/generated/client.ts` |
| `NativePurposeV1` | interface | `services/agent/sdk/typescript/src/generated/client.ts` |
| `Operation` | type | `services/agent/sdk/typescript/src/generated/client.ts` |
| `PROGRAM_LIFECYCLE_METADATA_JSON` | const | `services/agent/sdk/typescript/src/generated/client.ts` |
| `PROGRAM_LIFECYCLE_REQUEST_JSON` | const | `services/agent/sdk/typescript/src/generated/client.ts` |
| `PROGRAM_LIFECYCLE_TRANSPORT` | const | `services/agent/sdk/typescript/src/generated/client.ts` |
| `ProgramLifecycleOperation` | type | `services/agent/sdk/typescript/src/generated/client.ts` |
| `Rung` | enum | `services/agent/sdk/typescript/src/generated/client.ts` |
| `Sequence` | type | `services/agent/sdk/typescript/src/generated/client.ts` |
| `SignRequest` | interface | `services/agent/sdk/typescript/src/generated/client.ts` |
| `SignatureBytes` | type | `services/agent/sdk/typescript/src/generated/client.ts` |
| `StructuredActivityDisclosure` | interface | `services/agent/sdk/typescript/src/generated/client.ts` |
| `SubmissionState` | type | `services/agent/sdk/typescript/src/generated/client.ts` |
| `SubmitRequest` | interface | `services/agent/sdk/typescript/src/generated/client.ts` |
| `TimestampSeconds` | type | `services/agent/sdk/typescript/src/generated/client.ts` |
| `Transport` | interface | `services/agent/sdk/typescript/src/generated/client.ts` |
| `VerifiedRead` | interface | `services/agent/sdk/typescript/src/generated/client.ts` |
| `addressBytes` | function | `services/agent/sdk/typescript/src/generated/client.ts` |
| `decodeNativeApprovalListResult` | function | `services/agent/sdk/typescript/src/generated/client.ts` |
| `decodeNativeApprovalResult` | function | `services/agent/sdk/typescript/src/generated/client.ts` |
| `decodeNativePrepareResult` | function | `services/agent/sdk/typescript/src/generated/client.ts` |
| `encodeNativeApprovalDecision` | function | `services/agent/sdk/typescript/src/generated/client.ts` |
| `encodeNativeApprovalGet` | function | `services/agent/sdk/typescript/src/generated/client.ts` |
| `encodeNativePrepareRequest` | function | `services/agent/sdk/typescript/src/generated/client.ts` |
| `layerx_sdk_ts_package` | function | `services/agent/sdk/typescript/src/generated/client.ts` |
| `parseAmount` | function | `services/agent/sdk/typescript/src/generated/client.ts` |
| `parseBudgetLimit` | function | `services/agent/sdk/typescript/src/generated/client.ts` |
| `parseSequence` | function | `services/agent/sdk/typescript/src/generated/client.ts` |
| `parseTimestampSeconds` | function | `services/agent/sdk/typescript/src/generated/client.ts` |
| `programLifecycleRequest` | function | `services/agent/sdk/typescript/src/generated/client.ts` |
| `requireVerified` | function | `services/agent/sdk/typescript/src/generated/client.ts` |
| `signatureBytes` | function | `services/agent/sdk/typescript/src/generated/client.ts` |
| `MIRROR_ARCHIVE_MAGIC` | const | `services/agent/sdk/typescript/src/generated/mirror.ts` |
| `MIRROR_MAX_ARCHIVE_BYTES` | const | `services/agent/sdk/typescript/src/generated/mirror.ts` |
| `MIRROR_MAX_JSON_DEPTH` | const | `services/agent/sdk/typescript/src/generated/mirror.ts` |
| `MIRROR_MAX_SOURCES` | const | `services/agent/sdk/typescript/src/generated/mirror.ts` |
| `MIRROR_SCHEMA_VERSION` | const | `services/agent/sdk/typescript/src/generated/mirror.ts` |
| `MirrorErrorCode` | type | `services/agent/sdk/typescript/src/generated/mirror.ts` |
| `MirrorPolicyKind` | type | `services/agent/sdk/typescript/src/generated/mirror.ts` |
| `PROGRAM_ABI_V1` | const | `services/agent/sdk/typescript/src/generated/receipt.ts` |
| `PROGRAM_ABI_V2` | const | `services/agent/sdk/typescript/src/generated/receipt.ts` |
| `PROGRAM_ABI_V3` | const | `services/agent/sdk/typescript/src/generated/receipt.ts` |
| `PROGRAM_ABI_V4` | const | `services/agent/sdk/typescript/src/generated/receipt.ts` |
| `PROGRAM_ABI_V5` | const | `services/agent/sdk/typescript/src/generated/receipt.ts` |
| `RECEIPT_DOMAIN` | const | `services/agent/sdk/typescript/src/generated/receipt.ts` |
| `RECEIPT_VERSION_V2` | const | `services/agent/sdk/typescript/src/generated/receipt.ts` |
| `RECEIPT_VERSION_V3` | const | `services/agent/sdk/typescript/src/generated/receipt.ts` |
| `ReceiptFailureCode` | enum | `services/agent/sdk/typescript/src/generated/receipt.ts` |
| `SEQUENCE_DOMAIN` | const | `services/agent/sdk/typescript/src/generated/receipt.ts` |
| `supportsProgramGuestAbi` | function | `services/agent/sdk/typescript/src/generated/receipt.ts` |
| `HISTORY_DEFAULT_LIMIT` | const | `services/agent/sdk/typescript/src/history.ts` |
| `HISTORY_MAXIMUM_ACCOUNTS` | const | `services/agent/sdk/typescript/src/history.ts` |
| `HISTORY_MAXIMUM_LIMIT` | const | `services/agent/sdk/typescript/src/history.ts` |
| `HistoryAssetMetadata` | interface | `services/agent/sdk/typescript/src/history.ts` |
| `HistoryChain` | type | `services/agent/sdk/typescript/src/history.ts` |
| `HistoryDirection` | type | `services/agent/sdk/typescript/src/history.ts` |
| `HistoryOptions` | interface | `services/agent/sdk/typescript/src/history.ts` |
| `HistoryPage` | interface | `services/agent/sdk/typescript/src/history.ts` |
| `HistoryRow` | interface | `services/agent/sdk/typescript/src/history.ts` |
| `HistorySide` | interface | `services/agent/sdk/typescript/src/history.ts` |
| `UnifiedHistoryPage` | interface | `services/agent/sdk/typescript/src/history.ts` |
| `decodeHistoryAssetMetadata` | function | `services/agent/sdk/typescript/src/history.ts` |
| `decodeHistoryPage` | function | `services/agent/sdk/typescript/src/history.ts` |
| `decodeHistoryRow` | function | `services/agent/sdk/typescript/src/history.ts` |
| `decodePaxeerHistoryPage` | function | `services/agent/sdk/typescript/src/history.ts` |
| `decodeUnifiedHistoryPage` | function | `services/agent/sdk/typescript/src/history.ts` |
| `historyAccount` | function | `services/agent/sdk/typescript/src/history.ts` |
| `historyCursor` | function | `services/agent/sdk/typescript/src/history.ts` |
| `historyParams` | function | `services/agent/sdk/typescript/src/history.ts` |
| `aiMarket` | namespace re-export | `services/agent/sdk/typescript/src/index.ts` |
| `CanaryLogits` | type | `services/agent/sdk/typescript/src/inference_market.ts` |
| `ChallengeLapParams` | interface | `services/agent/sdk/typescript/src/inference_market.ts` |
| `FundAskParams` | interface | `services/agent/sdk/typescript/src/inference_market.ts` |
| `INFERENCE_LIMITS` | const | `services/agent/sdk/typescript/src/inference_market.ts` |
| `INFERENCE_OPERATIONS` | const | `services/agent/sdk/typescript/src/inference_market.ts` |
| `InferenceAskStream` | interface | `services/agent/sdk/typescript/src/inference_market.ts` |
| `InferenceAskStreamRecord` | interface | `services/agent/sdk/typescript/src/inference_market.ts` |
| `InferenceAttestationBinding` | interface | `services/agent/sdk/typescript/src/inference_market.ts` |
| `InferenceBookView` | interface | `services/agent/sdk/typescript/src/inference_market.ts` |
| `InferenceDealView` | interface | `services/agent/sdk/typescript/src/inference_market.ts` |
| `InferenceLayerSplit` | interface | `services/agent/sdk/typescript/src/inference_market.ts` |
| `InferenceMarketError` | class | `services/agent/sdk/typescript/src/inference_market.ts` |
| `InferencePeer` | interface | `services/agent/sdk/typescript/src/inference_market.ts` |
| `InferencePeerCapabilities` | interface | `services/agent/sdk/typescript/src/inference_market.ts` |
| `InferencePeerView` | interface | `services/agent/sdk/typescript/src/inference_market.ts` |
| `InferencePlacement` | interface | `services/agent/sdk/typescript/src/inference_market.ts` |
| `InferenceReceiptBatch` | interface | `services/agent/sdk/typescript/src/inference_market.ts` |
| `InferenceRoom` | interface | `services/agent/sdk/typescript/src/inference_market.ts` |
| `InferenceRoomProfile` | interface | `services/agent/sdk/typescript/src/inference_market.ts` |
| `InferenceRoomView` | interface | `services/agent/sdk/typescript/src/inference_market.ts` |
| `LockBondParams` | interface | `services/agent/sdk/typescript/src/inference_market.ts` |
| `OpenRoomParams` | interface | `services/agent/sdk/typescript/src/inference_market.ts` |
| `PostAskParams` | interface | `services/agent/sdk/typescript/src/inference_market.ts` |
| `PostBidParams` | interface | `services/agent/sdk/typescript/src/inference_market.ts` |
| `RecordDealParams` | interface | `services/agent/sdk/typescript/src/inference_market.ts` |
| `RefundBondParams` | interface | `services/agent/sdk/typescript/src/inference_market.ts` |
| `ResolveChallengeParams` | interface | `services/agent/sdk/typescript/src/inference_market.ts` |
| `SettleLapsParams` | interface | `services/agent/sdk/typescript/src/inference_market.ts` |
| `SlashBondParams` | interface | `services/agent/sdk/typescript/src/inference_market.ts` |
| `SplitPayoutParams` | interface | `services/agent/sdk/typescript/src/inference_market.ts` |
| `challengeLap` | function | `services/agent/sdk/typescript/src/inference_market.ts` |
| `closeRoom` | function | `services/agent/sdk/typescript/src/inference_market.ts` |
| `decodeBookView` | function | `services/agent/sdk/typescript/src/inference_market.ts` |
| `decodeDealView` | function | `services/agent/sdk/typescript/src/inference_market.ts` |
| `decodeInferenceDeal` | function | `services/agent/sdk/typescript/src/inference_market.ts` |
| `decodeInferencePeer` | function | `services/agent/sdk/typescript/src/inference_market.ts` |
| `decodeInferenceReceipt` | function | `services/agent/sdk/typescript/src/inference_market.ts` |
| `decodeInferenceReceipts` | function | `services/agent/sdk/typescript/src/inference_market.ts` |
| `decodeInferenceRoom` | function | `services/agent/sdk/typescript/src/inference_market.ts` |
| `decodeInferenceRooms` | function | `services/agent/sdk/typescript/src/inference_market.ts` |
| `decodePeerView` | function | `services/agent/sdk/typescript/src/inference_market.ts` |
| `decodeReceiptBatchView` | function | `services/agent/sdk/typescript/src/inference_market.ts` |
| `decodeRoomView` | function | `services/agent/sdk/typescript/src/inference_market.ts` |
| `fundAsk` | function | `services/agent/sdk/typescript/src/inference_market.ts` |
| `getInferenceDeal` | function | `services/agent/sdk/typescript/src/inference_market.ts` |
| `getInferencePeer` | function | `services/agent/sdk/typescript/src/inference_market.ts` |
| `getInferenceReceipts` | function | `services/agent/sdk/typescript/src/inference_market.ts` |
| `getInferenceRoom` | function | `services/agent/sdk/typescript/src/inference_market.ts` |
| `getInferenceRooms` | function | `services/agent/sdk/typescript/src/inference_market.ts` |
| `inferenceId` | function | `services/agent/sdk/typescript/src/inference_market.ts` |
| `lockBond` | function | `services/agent/sdk/typescript/src/inference_market.ts` |
| `openRoom` | function | `services/agent/sdk/typescript/src/inference_market.ts` |
| `postAsk` | function | `services/agent/sdk/typescript/src/inference_market.ts` |
| `postBid` | function | `services/agent/sdk/typescript/src/inference_market.ts` |
| `recordDeal` | function | `services/agent/sdk/typescript/src/inference_market.ts` |
| `refundBond` | function | `services/agent/sdk/typescript/src/inference_market.ts` |
| `resolveChallenge` | function | `services/agent/sdk/typescript/src/inference_market.ts` |
| `settleLaps` | function | `services/agent/sdk/typescript/src/inference_market.ts` |
| `slashBond` | function | `services/agent/sdk/typescript/src/inference_market.ts` |
| `splitPayout` | function | `services/agent/sdk/typescript/src/inference_market.ts` |
| `HUMAN_ERROR_CODES` | const | `services/agent/sdk/typescript/src/intent.ts` |
| `HUMAN_RETRIABILITIES` | const | `services/agent/sdk/typescript/src/intent.ts` |
| `HumanErrorCode` | type | `services/agent/sdk/typescript/src/intent.ts` |
| `HumanIntentClient` | class | `services/agent/sdk/typescript/src/intent.ts` |
| `HumanIntentClientOptions` | interface | `services/agent/sdk/typescript/src/intent.ts` |
| `HumanIntentError` | class | `services/agent/sdk/typescript/src/intent.ts` |
| `HumanIntentErrorDetails` | interface | `services/agent/sdk/typescript/src/intent.ts` |
| `HumanRetriability` | type | `services/agent/sdk/typescript/src/intent.ts` |
| `INTENT_DOMAINS` | const | `services/agent/sdk/typescript/src/intent.ts` |
| `INTENT_ENDPOINT_KINDS` | const | `services/agent/sdk/typescript/src/intent.ts` |
| `IntentConstraints` | interface | `services/agent/sdk/typescript/src/intent.ts` |
| `IntentDomain` | type | `services/agent/sdk/typescript/src/intent.ts` |
| `IntentEndpointKind` | type | `services/agent/sdk/typescript/src/intent.ts` |
| `IntentEndpointRef` | interface | `services/agent/sdk/typescript/src/intent.ts` |
| `IntentLeg` | interface | `services/agent/sdk/typescript/src/intent.ts` |
| `IntentLegBinding` | interface | `services/agent/sdk/typescript/src/intent.ts` |
| `IntentMoney` | interface | `services/agent/sdk/typescript/src/intent.ts` |
| `IntentOperation` | type | `services/agent/sdk/typescript/src/intent.ts` |
| `IntentPlan` | interface | `services/agent/sdk/typescript/src/intent.ts` |
| `IntentSigningRequirement` | interface | `services/agent/sdk/typescript/src/intent.ts` |
| `IntentSubmission` | interface | `services/agent/sdk/typescript/src/intent.ts` |
| `PlanIntentRequest` | interface | `services/agent/sdk/typescript/src/intent.ts` |
| `SubmitPlanRequest` | interface | `services/agent/sdk/typescript/src/intent.ts` |
| `decodeHumanEnvelope` | function | `services/agent/sdk/typescript/src/intent.ts` |
| `decodeHumanError` | function | `services/agent/sdk/typescript/src/intent.ts` |
| `decodeIntentPlan` | function | `services/agent/sdk/typescript/src/intent.ts` |
| `decodeIntentSubmission` | function | `services/agent/sdk/typescript/src/intent.ts` |
| `encodePlanIntentRequest` | function | `services/agent/sdk/typescript/src/intent.ts` |
| `encodeSubmitPlanRequest` | function | `services/agent/sdk/typescript/src/intent.ts` |
| `LAUNCHPAD_EVENTS` | re-export | `services/agent/sdk/typescript/src/launchpad.ts` |
| `LAUNCHPAD_PRECOMPILE` | re-export | `services/agent/sdk/typescript/src/launchpad.ts` |
| `LAUNCHPAD_WRITES` | re-export | `services/agent/sdk/typescript/src/launchpad.ts` |
| `LaunchpadNftWrite` | type | `services/agent/sdk/typescript/src/launchpad.ts` |
| `LaunchpadSpend` | type | `services/agent/sdk/typescript/src/launchpad.ts` |
| `LaunchpadSwapOrder` | interface | `services/agent/sdk/typescript/src/launchpad.ts` |
| `LaunchpadWrite` | type | `services/agent/sdk/typescript/src/launchpad.ts` |
| `decodeLaunchpadEvent` | function | `services/agent/sdk/typescript/src/launchpad.ts` |
| `launchpadApproval` | function | `services/agent/sdk/typescript/src/launchpad.ts` |
| `launchpadApproveCall` | function | `services/agent/sdk/typescript/src/launchpad.ts` |
| `launchpadBuyCall` | function | `services/agent/sdk/typescript/src/launchpad.ts` |
| `launchpadCall` | function | `services/agent/sdk/typescript/src/launchpad.ts` |
| `launchpadClaimFeesCall` | function | `services/agent/sdk/typescript/src/launchpad.ts` |
| `launchpadCreateMarketCall` | function | `services/agent/sdk/typescript/src/launchpad.ts` |
| `launchpadNftCall` | function | `services/agent/sdk/typescript/src/launchpad.ts` |
| `launchpadSellCall` | function | `services/agent/sdk/typescript/src/launchpad.ts` |
| `launchpadSetFeeStrategyCall` | function | `services/agent/sdk/typescript/src/launchpad.ts` |
| `sendLaunchpadBuy` | const | `services/agent/sdk/typescript/src/launchpad.ts` |
| `sendLaunchpadClaimFees` | const | `services/agent/sdk/typescript/src/launchpad.ts` |
| `sendLaunchpadCreateMarket` | const | `services/agent/sdk/typescript/src/launchpad.ts` |
| `sendLaunchpadNftWrite` | const | `services/agent/sdk/typescript/src/launchpad.ts` |
| `sendLaunchpadSell` | const | `services/agent/sdk/typescript/src/launchpad.ts` |
| `sendLaunchpadSetFeeStrategy` | const | `services/agent/sdk/typescript/src/launchpad.ts` |
| `sendLaunchpadWithApproval` | function | `services/agent/sdk/typescript/src/launchpad.ts` |
| `waitLaunchpadApproval` | function | `services/agent/sdk/typescript/src/launchpad.ts` |
| `MirrorCandidate` | interface | `services/agent/sdk/typescript/src/mirror.ts` |
| `MirrorLag` | type | `services/agent/sdk/typescript/src/mirror.ts` |
| `MirrorObservation` | interface | `services/agent/sdk/typescript/src/mirror.ts` |
| `MirrorProvenance` | type | `services/agent/sdk/typescript/src/mirror.ts` |
| `MirrorReadPolicy` | type | `services/agent/sdk/typescript/src/mirror.ts` |
| `MirrorTarget` | interface | `services/agent/sdk/typescript/src/mirror.ts` |
| `NameResolution` | interface | `services/agent/sdk/typescript/src/names.ts` |
| `labelhash` | function | `services/agent/sdk/typescript/src/names.ts` |
| `namehash` | function | `services/agent/sdk/typescript/src/names.ts` |
| `resolveName` | function | `services/agent/sdk/typescript/src/names.ts` |
| `MAX_NATIVE_BALANCE_VIEWS` | const | `services/agent/sdk/typescript/src/native-capabilities.ts` |
| `MAX_NATIVE_CAPABILITIES` | const | `services/agent/sdk/typescript/src/native-capabilities.ts` |
| `MAX_NATIVE_CAPABILITY_BYTES` | const | `services/agent/sdk/typescript/src/native-capabilities.ts` |
| `NativeCapability` | type | `services/agent/sdk/typescript/src/native-capabilities.ts` |
| `decodeNativeCapabilitySet` | function | `services/agent/sdk/typescript/src/native-capabilities.ts` |
| `deriveNativeProgramAccount` | function | `services/agent/sdk/typescript/src/native-capabilities.ts` |
| `encodeNativeCapabilitySet` | function | `services/agent/sdk/typescript/src/native-capabilities.ts` |
| `narrowNativeCapabilitySet` | function | `services/agent/sdk/typescript/src/native-capabilities.ts` |
| `NativeEffectPrepareRequestV1` | interface | `services/agent/sdk/typescript/src/native-effect.ts` |
| `encodeNativeEffectPrepareRequest` | function | `services/agent/sdk/typescript/src/native-effect.ts` |
| `nativeEffectPrepareRequestDigest` | function | `services/agent/sdk/typescript/src/native-effect.ts` |
| `NativeProgramCall` | interface | `services/agent/sdk/typescript/src/native-program-call.ts` |
| `NativeProgramCallV1` | type | `services/agent/sdk/typescript/src/native-program-call.ts` |
| `decodeNativeProgramCall` | function | `services/agent/sdk/typescript/src/native-program-call.ts` |
| `decodeNativeProgramCallV1` | function | `services/agent/sdk/typescript/src/native-program-call.ts` |
| `encodeNativeProgramCall` | function | `services/agent/sdk/typescript/src/native-program-call.ts` |
| `encodeNativeProgramCallV1` | function | `services/agent/sdk/typescript/src/native-program-call.ts` |
| `ExecutableMirrorError` | class | `services/agent/sdk/typescript/src/node-mirror.ts` |
| `ExecutableMirrorVerification` | interface | `services/agent/sdk/typescript/src/node-mirror.ts` |
| `LocalMirrorExecutableVerifier` | class | `services/agent/sdk/typescript/src/node-mirror.ts` |
| `decodeMirrorProcessResult` | function | `services/agent/sdk/typescript/src/node-mirror.ts` |
| `PERPS_ACTIVITY_TYPES` | const | `services/agent/sdk/typescript/src/perps.ts` |
| `PERPS_EVENTS` | re-export | `services/agent/sdk/typescript/src/perps.ts` |
| `PERPS_MODULE_ID` | const | `services/agent/sdk/typescript/src/perps.ts` |
| `PERPS_PRECOMPILE` | re-export | `services/agent/sdk/typescript/src/perps.ts` |
| `PERPS_WRITES` | re-export | `services/agent/sdk/typescript/src/perps.ts` |
| `PerpsActivity` | type | `services/agent/sdk/typescript/src/perps.ts` |
| `PerpsActivityName` | type | `services/agent/sdk/typescript/src/perps.ts` |
| `PerpsMarket` | interface | `services/agent/sdk/typescript/src/perps.ts` |
| `PerpsOpenPosition` | interface | `services/agent/sdk/typescript/src/perps.ts` |
| `PerpsWrite` | type | `services/agent/sdk/typescript/src/perps.ts` |
| `TradeSide` | type | `services/agent/sdk/typescript/src/perps.ts` |
| `TradingId` | type | `services/agent/sdk/typescript/src/perps.ts` |
| `TradingPayloadError` | class | `services/agent/sdk/typescript/src/perps.ts` |
| `TradingPayloadErrorCode` | type | `services/agent/sdk/typescript/src/perps.ts` |
| `TradingReader` | class | `services/agent/sdk/typescript/src/perps.ts` |
| `TradingWriter` | class | `services/agent/sdk/typescript/src/perps.ts` |
| `decodePerpsActivity` | function | `services/agent/sdk/typescript/src/perps.ts` |
| `decodePerpsEvent` | function | `services/agent/sdk/typescript/src/perps.ts` |
| `encodePerpsActivity` | function | `services/agent/sdk/typescript/src/perps.ts` |
| `perpsActivityType` | function | `services/agent/sdk/typescript/src/perps.ts` |
| `perpsCall` | function | `services/agent/sdk/typescript/src/perps.ts` |
| `perpsCancelOrderCall` | function | `services/agent/sdk/typescript/src/perps.ts` |
| `perpsClosePositionCall` | function | `services/agent/sdk/typescript/src/perps.ts` |
| `perpsDepositCall` | function | `services/agent/sdk/typescript/src/perps.ts` |
| `perpsOpenPositionCall` | function | `services/agent/sdk/typescript/src/perps.ts` |
| `perpsWithdrawCall` | function | `services/agent/sdk/typescript/src/perps.ts` |
| `AGENT_OPERATIONS` | const | `services/agent/sdk/typescript/src/production.ts` |
| `CallOptions` | interface | `services/agent/sdk/typescript/src/production.ts` |
| `HUMAN_OPERATIONS` | const | `services/agent/sdk/typescript/src/production.ts` |
| `HumanOperation` | type | `services/agent/sdk/typescript/src/production.ts` |
| `IdempotencyKey` | type | `services/agent/sdk/typescript/src/production.ts` |
| `PlatformPlane` | type | `services/agent/sdk/typescript/src/production.ts` |
| `PlatformSdkError` | class | `services/agent/sdk/typescript/src/production.ts` |
| `ProductionClient` | class | `services/agent/sdk/typescript/src/production.ts` |
| `ProductionTransport` | interface | `services/agent/sdk/typescript/src/production.ts` |
| `ProtocolAmount` | type | `services/agent/sdk/typescript/src/production.ts` |
| `RETIRED_OPERATION_NAMES` | const | `services/agent/sdk/typescript/src/production.ts` |
| `RetryClass` | type | `services/agent/sdk/typescript/src/production.ts` |
| `SDK_ERROR_CODES` | const | `services/agent/sdk/typescript/src/production.ts` |
| `SafeErrorDetails` | interface | `services/agent/sdk/typescript/src/production.ts` |
| `SdkErrorCode` | type | `services/agent/sdk/typescript/src/production.ts` |
| `SdkTelemetry` | type | `services/agent/sdk/typescript/src/production.ts` |
| `SdkTelemetryDiagnostic` | type | `services/agent/sdk/typescript/src/production.ts` |
| `SdkTelemetryEvent` | interface | `services/agent/sdk/typescript/src/production.ts` |
| `SecretBytes` | class | `services/agent/sdk/typescript/src/production.ts` |
| `TransportCall` | interface | `services/agent/sdk/typescript/src/production.ts` |
| `idempotencyKey` | function | `services/agent/sdk/typescript/src/production.ts` |
| `platform_sdk_typescript` | function | `services/agent/sdk/typescript/src/production.ts` |
| `protocolAmount` | function | `services/agent/sdk/typescript/src/production.ts` |
| `NATIVE_PROGRAM_REGISTRATION_MAX_BYTES` | const | `services/agent/sdk/typescript/src/program-lifecycle.ts` |
| `NativeProgramDeploy` | interface | `services/agent/sdk/typescript/src/program-lifecycle.ts` |
| `NativeProgramGuestAbi` | type | `services/agent/sdk/typescript/src/program-lifecycle.ts` |
| `NativeProgramLifecycleRequest` | class | `services/agent/sdk/typescript/src/program-lifecycle.ts` |
| `NativeProgramUpgrade` | interface | `services/agent/sdk/typescript/src/program-lifecycle.ts` |
| `NativeProgramWindDown` | type | `services/agent/sdk/typescript/src/program-lifecycle.ts` |
| `ProgramLifecycleSubmission` | type | `services/agent/sdk/typescript/src/program-lifecycle.ts` |
| `admitNativeProgramUpgrade` | function | `services/agent/sdk/typescript/src/program-lifecycle.ts` |
| `decodeNativeProgramDeploy` | function | `services/agent/sdk/typescript/src/program-lifecycle.ts` |
| `decodeNativeProgramUpgrade` | function | `services/agent/sdk/typescript/src/program-lifecycle.ts` |
| `decodeNativeProgramWindDown` | function | `services/agent/sdk/typescript/src/program-lifecycle.ts` |
| `encodeNativeProgramDeploy` | function | `services/agent/sdk/typescript/src/program-lifecycle.ts` |
| `encodeNativeProgramUpgrade` | function | `services/agent/sdk/typescript/src/program-lifecycle.ts` |
| `encodeNativeProgramWindDown` | function | `services/agent/sdk/typescript/src/program-lifecycle.ts` |
| `GatewayProgramDiscovery` | interface | `services/agent/sdk/typescript/src/programs.ts` |
| `NativeProgramRequest` | class | `services/agent/sdk/typescript/src/programs.ts` |
| `NativeProgramRequestV1` | class | `services/agent/sdk/typescript/src/programs.ts` |
| `OccupancyPayer` | re-export | `services/agent/sdk/typescript/src/programs.ts` |
| `ProgramAuthorityDocument` | interface | `services/agent/sdk/typescript/src/programs.ts` |
| `ProgramBudget` | interface | `services/agent/sdk/typescript/src/programs.ts` |
| `ProgramCall` | interface | `services/agent/sdk/typescript/src/programs.ts` |
| `ProgramCapability` | type | `services/agent/sdk/typescript/src/programs.ts` |
| `ProgramDiscovery` | interface | `services/agent/sdk/typescript/src/programs.ts` |
| `ProgramExecutionDocument` | interface | `services/agent/sdk/typescript/src/programs.ts` |
| `ProgramFailure` | type | `services/agent/sdk/typescript/src/programs.ts` |
| `ProgramInterface` | interface | `services/agent/sdk/typescript/src/programs.ts` |
| `ProgramOperations` | class | `services/agent/sdk/typescript/src/programs.ts` |
| `ProgramOutcome` | type | `services/agent/sdk/typescript/src/programs.ts` |
| `ProgramSimulation` | interface | `services/agent/sdk/typescript/src/programs.ts` |
| `ProgramSimulationEvidence` | interface | `services/agent/sdk/typescript/src/programs.ts` |
| `ProgramSource` | type | `services/agent/sdk/typescript/src/programs.ts` |
| `ProgramSubmission` | type | `services/agent/sdk/typescript/src/programs.ts` |
| `ProgramTrustContext` | class | `services/agent/sdk/typescript/src/programs.ts` |
| `ProgramUnknownSubmission` | interface | `services/agent/sdk/typescript/src/programs.ts` |
| `ProgramUsage` | interface | `services/agent/sdk/typescript/src/programs.ts` |
| `parseProgramExecutionDocument` | function | `services/agent/sdk/typescript/src/programs.ts` |
| `parseProgramExecutionDocumentV5` | function | `services/agent/sdk/typescript/src/programs.ts` |
| `platform_sdk_programs` | function | `services/agent/sdk/typescript/src/programs.ts` |
| `resolveLifecycleFailure` | function | `services/agent/sdk/typescript/src/programs.ts` |
| `verifyGatewayProgramDiscovery` | function | `services/agent/sdk/typescript/src/programs.ts` |
| `verifyGatewayProgramInterface` | function | `services/agent/sdk/typescript/src/programs.ts` |
| `PX_MAXIMUM_JOINED_ASSETS` | const | `services/agent/sdk/typescript/src/px.ts` |
| `PX_METHODS` | const | `services/agent/sdk/typescript/src/px.ts` |
| `PxAccountBalances` | interface | `services/agent/sdk/typescript/src/px.ts` |
| `PxAccountJoin` | interface | `services/agent/sdk/typescript/src/px.ts` |
| `PxAssetBalance` | interface | `services/agent/sdk/typescript/src/px.ts` |
| `PxAssetEntry` | interface | `services/agent/sdk/typescript/src/px.ts` |
| `PxAssetTable` | interface | `services/agent/sdk/typescript/src/px.ts` |
| `PxClient` | class | `services/agent/sdk/typescript/src/px.ts` |
| `PxClientOptions` | interface | `services/agent/sdk/typescript/src/px.ts` |
| `PxCustodyAsset` | interface | `services/agent/sdk/typescript/src/px.ts` |
| `PxMethod` | type | `services/agent/sdk/typescript/src/px.ts` |
| `PxNetworkHead` | interface | `services/agent/sdk/typescript/src/px.ts` |
| `PxPaxeerAccount` | interface | `services/agent/sdk/typescript/src/px.ts` |
| `PxPaxeerAssetBalance` | interface | `services/agent/sdk/typescript/src/px.ts` |
| `PxPaxeerHead` | interface | `services/agent/sdk/typescript/src/px.ts` |
| `PxReceiptKeyEntry` | interface | `services/agent/sdk/typescript/src/px.ts` |
| `PxResolvedIdentities` | interface | `services/agent/sdk/typescript/src/px.ts` |
| `decodePxAccountBalances` | function | `services/agent/sdk/typescript/src/px.ts` |
| `decodePxAccountJoin` | function | `services/agent/sdk/typescript/src/px.ts` |
| `decodePxAssetBalance` | function | `services/agent/sdk/typescript/src/px.ts` |
| `decodePxAssetTable` | function | `services/agent/sdk/typescript/src/px.ts` |
| `decodePxCustodyAsset` | function | `services/agent/sdk/typescript/src/px.ts` |
| `decodePxNetworkHead` | function | `services/agent/sdk/typescript/src/px.ts` |
| `decodePxReceiptKeys` | function | `services/agent/sdk/typescript/src/px.ts` |
| `decodePxResolvedIdentities` | function | `services/agent/sdk/typescript/src/px.ts` |
| `pxAccountKey` | function | `services/agent/sdk/typescript/src/px.ts` |
| `pxOptionalQuantity` | function | `services/agent/sdk/typescript/src/px.ts` |
| `pxQuantity` | function | `services/agent/sdk/typescript/src/px.ts` |
| `LXP_RETRYABLE` | const | `services/agent/sdk/typescript/src/revert.ts` |
| `RevertReasonError` | class | `services/agent/sdk/typescript/src/revert.ts` |
| `decodeRevertReason` | function | `services/agent/sdk/typescript/src/revert.ts` |
| `AssetListSnapshot` | interface | `services/agent/sdk/typescript/src/rpc.ts` |
| `AssetMetadata` | interface | `services/agent/sdk/typescript/src/rpc.ts` |
| `AssetSnapshot` | interface | `services/agent/sdk/typescript/src/rpc.ts` |
| `Commitment` | type | `services/agent/sdk/typescript/src/rpc.ts` |
| `IdentitySequenceSnapshot` | interface | `services/agent/sdk/typescript/src/rpc.ts` |
| `JsonRpcClient` | class | `services/agent/sdk/typescript/src/rpc.ts` |
| `JsonRpcError` | class | `services/agent/sdk/typescript/src/rpc.ts` |
| `LIST_ASSETS_DEFAULT_PAGE` | const | `services/agent/sdk/typescript/src/rpc.ts` |
| `LIST_ASSETS_MAX_PAGE` | const | `services/agent/sdk/typescript/src/rpc.ts` |
| `RpcParam` | type | `services/agent/sdk/typescript/src/rpc.ts` |
| `SubscriptionEvent` | re-export | `services/agent/sdk/typescript/src/rpc.ts` |
| `SubscriptionTopic` | re-export | `services/agent/sdk/typescript/src/rpc.ts` |
| `decodeAssetListSnapshot` | function | `services/agent/sdk/typescript/src/rpc.ts` |
| `decodeAssetSnapshot` | function | `services/agent/sdk/typescript/src/rpc.ts` |
| `decodeIdentitySequenceSnapshot` | function | `services/agent/sdk/typescript/src/rpc.ts` |
| `decodeJsonRpcResponse` | function | `services/agent/sdk/typescript/src/rpc.ts` |
| `listAssetsParams` | function | `services/agent/sdk/typescript/src/rpc.ts` |
| `walletAccount` | function | `services/agent/sdk/typescript/src/rpc.ts` |
| `SUBSCRIPTION_MAX_BACKLOG` | const | `services/agent/sdk/typescript/src/rpc-subscription.ts` |
| `SUBSCRIPTION_MAX_BACKLOG_BYTES` | const | `services/agent/sdk/typescript/src/rpc-subscription.ts` |
| `SubscriptionReconciliationRequiredError` | class | `services/agent/sdk/typescript/src/rpc-subscription.ts` |
| `encodeSequencerReceipt` | function | `services/agent/sdk/typescript/src/sequencer-receipt-codec.ts` |
| `SPOT_ACTIVITY_TYPES` | const | `services/agent/sdk/typescript/src/spot.ts` |
| `SPOT_MODULE_ID` | const | `services/agent/sdk/typescript/src/spot.ts` |
| `SPOT_PRECOMPILES` | re-export | `services/agent/sdk/typescript/src/spot.ts` |
| `SPOT_WRITES` | re-export | `services/agent/sdk/typescript/src/spot.ts` |
| `SpotActivity` | type | `services/agent/sdk/typescript/src/spot.ts` |
| `SpotContract` | type | `services/agent/sdk/typescript/src/spot.ts` |
| `SpotOrderKind` | type | `services/agent/sdk/typescript/src/spot.ts` |
| `SpotTimeInForce` | type | `services/agent/sdk/typescript/src/spot.ts` |
| `SpotWrite` | type | `services/agent/sdk/typescript/src/spot.ts` |
| `decodeSpotActivity` | function | `services/agent/sdk/typescript/src/spot.ts` |
| `encodeSpotActivity` | function | `services/agent/sdk/typescript/src/spot.ts` |
| `spotActivityType` | function | `services/agent/sdk/typescript/src/spot.ts` |
| `spotCall` | function | `services/agent/sdk/typescript/src/spot.ts` |
| `ResumableStream` | class | `services/agent/sdk/typescript/src/stream.ts` |
| `STREAM_EVENT_ID_MAX_LENGTH` | const | `services/agent/sdk/typescript/src/stream.ts` |
| `STREAM_REPLAY_WINDOW` | const | `services/agent/sdk/typescript/src/stream.ts` |
| `StreamCursor` | type | `services/agent/sdk/typescript/src/stream.ts` |
| `StreamEvent` | interface | `services/agent/sdk/typescript/src/stream.ts` |
| `StreamPage` | interface | `services/agent/sdk/typescript/src/stream.ts` |
| `StreamPageSource` | type | `services/agent/sdk/typescript/src/stream.ts` |
| `streamCursor` | function | `services/agent/sdk/typescript/src/stream.ts` |
| `CometBlockId` | interface | `services/agent/sdk/typescript/src/verifier.ts` |
| `CometCommitSignature` | interface | `services/agent/sdk/typescript/src/verifier.ts` |
| `CometHeader` | interface | `services/agent/sdk/typescript/src/verifier.ts` |
| `CometValidator` | interface | `services/agent/sdk/typescript/src/verifier.ts` |
| `CommittedReceiptEvidence` | interface | `services/agent/sdk/typescript/src/verifier.ts` |
| `CommittedReceiptVerification` | interface | `services/agent/sdk/typescript/src/verifier.ts` |
| `DEFAULT_PROTOCOL_VERSION` | const | `services/agent/sdk/typescript/src/verifier.ts` |
| `LEGACY_PROTOCOL_VERSION` | const | `services/agent/sdk/typescript/src/verifier.ts` |
| `NodeCommittedReceiptEvidence` | interface | `services/agent/sdk/typescript/src/verifier.ts` |
| `NodeReceiptInclusion` | interface | `services/agent/sdk/typescript/src/verifier.ts` |
| `ProtocolReceipt` | interface | `services/agent/sdk/typescript/src/verifier.ts` |
| `ProtocolSelection` | interface | `services/agent/sdk/typescript/src/verifier.ts` |
| `ReceiptEffect` | interface | `services/agent/sdk/typescript/src/verifier.ts` |
| `ReceiptVerification` | interface | `services/agent/sdk/typescript/src/verifier.ts` |
| `ReceiptVerificationError` | class | `services/agent/sdk/typescript/src/verifier.ts` |
| `STATE_COMMITMENT_PROTOCOL_VERSION` | const | `services/agent/sdk/typescript/src/verifier.ts` |
| `SelectableProtocolVersion` | type | `services/agent/sdk/typescript/src/verifier.ts` |
| `SequencerAuthorization` | interface | `services/agent/sdk/typescript/src/verifier.ts` |
| `SequencerReceipt` | interface | `services/agent/sdk/typescript/src/verifier.ts` |
| `SequencerReceiptKey` | interface | `services/agent/sdk/typescript/src/verifier.ts` |
| `SequencerReceiptRefusal` | type | `services/agent/sdk/typescript/src/verifier.ts` |
| `SequencerReceiptVerification` | interface | `services/agent/sdk/typescript/src/verifier.ts` |
| `SequencerReceiptVerificationError` | class | `services/agent/sdk/typescript/src/verifier.ts` |
| `cometHeaderHash` | function | `services/agent/sdk/typescript/src/verifier.ts` |
| `cometValidatorSetHash` | function | `services/agent/sdk/typescript/src/verifier.ts` |
| `isSelectableProtocolVersion` | function | `services/agent/sdk/typescript/src/verifier.ts` |
| `protocolVersionUsesOccupancy` | function | `services/agent/sdk/typescript/src/verifier.ts` |
| `selectedProtocolVersion` | function | `services/agent/sdk/typescript/src/verifier.ts` |
| `supportedProgramGuestAbi` | function | `services/agent/sdk/typescript/src/verifier.ts` |
| `verifyCometCommit` | function | `services/agent/sdk/typescript/src/verifier.ts` |
| `verifyCommittedReceipt` | function | `services/agent/sdk/typescript/src/verifier.ts` |
| `verifySequencerReceipt` | function | `services/agent/sdk/typescript/src/verifier.ts` |
| `NativeTokenRegistration` | interface | `services/agent/sdk/typescript/src/wallet.ts` |
| `VerifiedWalletReceipt` | interface | `services/agent/sdk/typescript/src/wallet.ts` |
| `WalletExecutionOptions` | interface | `services/agent/sdk/typescript/src/wallet.ts` |
| `WalletPendingError` | class | `services/agent/sdk/typescript/src/wallet.ts` |
| `WalletRpc` | class | `services/agent/sdk/typescript/src/wallet.ts` |
| `WalletRpcConfiguration` | interface | `services/agent/sdk/typescript/src/wallet.ts` |
| `encodeNativeRegistration` | function | `services/agent/sdk/typescript/src/wallet.ts` |
| `WEB_CONTENT_DOMAIN` | const | `services/agent/sdk/typescript/src/web-search.ts` |
| `WEB_CONTENT_FETCH` | const | `services/agent/sdk/typescript/src/web-search.ts` |
| `WEB_CONTENT_SEARCH` | const | `services/agent/sdk/typescript/src/web-search.ts` |
| `WEB_SEARCH_CURRENCIES` | const | `services/agent/sdk/typescript/src/web-search.ts` |
| `WEB_SEARCH_MAX_RESULTS` | const | `services/agent/sdk/typescript/src/web-search.ts` |
| `WEB_SEARCH_PAYER_HEADER` | const | `services/agent/sdk/typescript/src/web-search.ts` |
| `WebContent` | interface | `services/agent/sdk/typescript/src/web-search.ts` |
| `WebContentKind` | type | `services/agent/sdk/typescript/src/web-search.ts` |
| `WebContentResponse` | interface | `services/agent/sdk/typescript/src/web-search.ts` |
| `WebFetchResponse` | interface | `services/agent/sdk/typescript/src/web-search.ts` |
| `WebSearchAssetTerms` | interface | `services/agent/sdk/typescript/src/web-search.ts` |
| `WebSearchClient` | class | `services/agent/sdk/typescript/src/web-search.ts` |
| `WebSearchClientOptions` | interface | `services/agent/sdk/typescript/src/web-search.ts` |
| `WebSearchCurrency` | type | `services/agent/sdk/typescript/src/web-search.ts` |
| `WebSearchError` | class | `services/agent/sdk/typescript/src/web-search.ts` |
| `WebSearchExactLogPayment` | interface | `services/agent/sdk/typescript/src/web-search.ts` |
| `WebSearchExactPayment` | interface | `services/agent/sdk/typescript/src/web-search.ts` |
| `WebSearchMeteredPayment` | interface | `services/agent/sdk/typescript/src/web-search.ts` |
| `WebSearchOffer` | interface | `services/agent/sdk/typescript/src/web-search.ts` |
| `WebSearchPayer` | interface | `services/agent/sdk/typescript/src/web-search.ts` |
| `WebSearchPaymentVerification` | interface | `services/agent/sdk/typescript/src/web-search.ts` |
| `WebSearchPreference` | interface | `services/agent/sdk/typescript/src/web-search.ts` |
| `WebSearchResponse` | interface | `services/agent/sdk/typescript/src/web-search.ts` |
| `WebSearchResult` | interface | `services/agent/sdk/typescript/src/web-search.ts` |
| `WebSearchScheme` | type | `services/agent/sdk/typescript/src/web-search.ts` |
| `WebSettlement` | interface | `services/agent/sdk/typescript/src/web-search.ts` |
| `bindLedgerPayment` | function | `services/agent/sdk/typescript/src/web-search.ts` |
| `contentDigest` | function | `services/agent/sdk/typescript/src/web-search.ts` |
| `decodePayerGrant` | function | `services/agent/sdk/typescript/src/web-search.ts` |
| `decodeWebContent` | function | `services/agent/sdk/typescript/src/web-search.ts` |
| `resolveWebLedgerAccount` | function | `services/agent/sdk/typescript/src/web-search.ts` |
| `verifyWebSearchPayment` | function | `services/agent/sdk/typescript/src/web-search.ts` |
| `webContentBytes` | function | `services/agent/sdk/typescript/src/web-search.ts` |
| `webSearchPayerOwner` | function | `services/agent/sdk/typescript/src/web-search.ts` |
| `ExpectedDrawTransaction` | interface | `services/agent/sdk/typescript/src/x402/activity.ts` |
| `bindDrawTransaction` | function | `services/agent/sdk/typescript/src/x402/activity.ts` |
| `GrantDrawRpc` | interface | `services/agent/sdk/typescript/src/x402/draw.ts` |
| `PreparedGrantDraws` | class | `services/agent/sdk/typescript/src/x402/draw.ts` |
| `GrantDraw` | interface | `services/agent/sdk/typescript/src/x402/grant.ts` |
| `GrantOffer` | interface | `services/agent/sdk/typescript/src/x402/grant.ts` |
| `decodeGrantDraw` | function | `services/agent/sdk/typescript/src/x402/grant.ts` |
| `validateGrantDraw` | function | `services/agent/sdk/typescript/src/x402/grant.ts` |
| `AssetRegistration` | interface | `services/agent/sdk/typescript/src/x402/receive.ts` |
| `LAYERX_LEDGER_PRECOMPILE` | const | `services/agent/sdk/typescript/src/x402/receive.ts` |
| `PayerGrant` | interface | `services/agent/sdk/typescript/src/x402/receive.ts` |
| `Receive` | interface | `services/agent/sdk/typescript/src/x402/receive.ts` |
| `ReceiverAuthorization` | interface | `services/agent/sdk/typescript/src/x402/receive.ts` |
| `decodeReceive` | function | `services/agent/sdk/typescript/src/x402/receive.ts` |
| `deriveNativeAssetId` | function | `services/agent/sdk/typescript/src/x402/receive.ts` |
| `encodeAccountOpen` | function | `services/agent/sdk/typescript/src/x402/receive.ts` |
| `encodeAssetRegister` | function | `services/agent/sdk/typescript/src/x402/receive.ts` |
| `encodeAssetSupply` | function | `services/agent/sdk/typescript/src/x402/receive.ts` |
| `encodeGrant` | function | `services/agent/sdk/typescript/src/x402/receive.ts` |
| `encodeGrantRevoke` | function | `services/agent/sdk/typescript/src/x402/receive.ts` |
| `encodeReceive` | function | `services/agent/sdk/typescript/src/x402/receive.ts` |
| `grantAuthorizationMessage` | function | `services/agent/sdk/typescript/src/x402/receive.ts` |
| `issueGrantCall` | function | `services/agent/sdk/typescript/src/x402/receive.ts` |
| `openAccountCall` | function | `services/agent/sdk/typescript/src/x402/receive.ts` |
| `receiveAuthorizationMessage` | function | `services/agent/sdk/typescript/src/x402/receive.ts` |
| `registerAssetCall` | function | `services/agent/sdk/typescript/src/x402/receive.ts` |
| `XWEB_API_LIMITS` | const | `services/agent/sdk/typescript/src/xweb-api.ts` |
| `XWEB_API_VERSION` | const | `services/agent/sdk/typescript/src/xweb-api.ts` |
| `XWEB_DEFAULT_MAX_PAYLOAD_BYTES` | const | `services/agent/sdk/typescript/src/xweb-api.ts` |
| `XWEB_ENVELOPE_INFO` | const | `services/agent/sdk/typescript/src/xweb-api.ts` |
| `XWEB_ENVELOPE_KEY_LENGTH` | const | `services/agent/sdk/typescript/src/xweb-api.ts` |
| `XWEB_ENVELOPE_NONCE_LENGTH` | const | `services/agent/sdk/typescript/src/xweb-api.ts` |
| `XWEB_ENVELOPE_OVERHEAD` | const | `services/agent/sdk/typescript/src/xweb-api.ts` |
| `XWEB_ENVELOPE_TAG_LENGTH` | const | `services/agent/sdk/typescript/src/xweb-api.ts` |
| `XWEB_KIND_API` | const | `services/agent/sdk/typescript/src/xweb-api.ts` |
| `XWEB_LEVEL_MAJORITY` | const | `services/agent/sdk/typescript/src/xweb-api.ts` |
| `XWEB_LEVEL_SINGLE` | const | `services/agent/sdk/typescript/src/xweb-api.ts` |
| `XWEB_MAX_ENVELOPE_BYTES` | const | `services/agent/sdk/typescript/src/xweb-api.ts` |
| `XWEB_METHOD_GET` | const | `services/agent/sdk/typescript/src/xweb-api.ts` |
| `XWEB_METHOD_POST` | const | `services/agent/sdk/typescript/src/xweb-api.ts` |
| `XWEB_PRECOMPILE` | const | `services/agent/sdk/typescript/src/xweb-api.ts` |
| `XWebApiBuildOptions` | interface | `services/agent/sdk/typescript/src/xweb-api.ts` |
| `XWebApiCall` | interface | `services/agent/sdk/typescript/src/xweb-api.ts` |
| `XWebApiError` | class | `services/agent/sdk/typescript/src/xweb-api.ts` |
| `XWebApiErrorCode` | type | `services/agent/sdk/typescript/src/xweb-api.ts` |
| `XWebApiHeader` | interface | `services/agent/sdk/typescript/src/xweb-api.ts` |
| `XWebApiMethod` | type | `services/agent/sdk/typescript/src/xweb-api.ts` |
| `XWebApiPayload` | interface | `services/agent/sdk/typescript/src/xweb-api.ts` |
| `XWebApiRequest` | interface | `services/agent/sdk/typescript/src/xweb-api.ts` |
| `XWebAttestor` | interface | `services/agent/sdk/typescript/src/xweb-api.ts` |
| `XWebAttestorSet` | interface | `services/agent/sdk/typescript/src/xweb-api.ts` |
| `XWebEnvelopeRandomness` | interface | `services/agent/sdk/typescript/src/xweb-api.ts` |
| `XWebSealedEnvelope` | interface | `services/agent/sdk/typescript/src/xweb-api.ts` |
| `buildXWebApiRequest` | function | `services/agent/sdk/typescript/src/xweb-api.ts` |
| `decodeXWebApiPayload` | function | `services/agent/sdk/typescript/src/xweb-api.ts` |
| `decodeXWebAttestors` | function | `services/agent/sdk/typescript/src/xweb-api.ts` |
| `encodeXWebApiPayload` | function | `services/agent/sdk/typescript/src/xweb-api.ts` |
| `encodeXWebCredential` | function | `services/agent/sdk/typescript/src/xweb-api.ts` |
| `sealXWebEnvelope` | function | `services/agent/sdk/typescript/src/xweb-api.ts` |
| `xwebApiOrigin` | function | `services/agent/sdk/typescript/src/xweb-api.ts` |
| `xwebApiRequestCall` | function | `services/agent/sdk/typescript/src/xweb-api.ts` |
| `xwebAttestorAddress` | function | `services/agent/sdk/typescript/src/xweb-api.ts` |
| `xwebGetAttestorsCallData` | function | `services/agent/sdk/typescript/src/xweb-api.ts` |
| `xwebMethodCode` | function | `services/agent/sdk/typescript/src/xweb-api.ts` |

## Python (`services/agent/sdk/python`)

Package `layerx_sdk`, entry point [`__init__.py`](../../services/agent/sdk/python/layerx_sdk/__init__.py). This list follows the package's explicit `__all__`, including the tenant-chain helpers. As with TypeScript, the exported transport and verifier interfaces do not establish deployment availability or transport agreement.

- `AGENT_OPERATIONS`
- `APPROVAL_CONTRACT_INTRODUCED`
- `APPROVAL_DECISION_OUTCOMES`
- `APPROVAL_ENFORCEMENT_NOTICE`
- `APPROVAL_EVENT_KINDS`
- `APPROVAL_STATES`
- `AgentGrantMiddleware`
- `AgentHttpTransport`
- `Amount`
- `ApiError`
- `ApprovalApproveRequest`
- `ApprovalDecision`
- `ApprovalDecisionOutcome`
- `ApprovalEventKind`
- `ApprovalGetRequest`
- `ApprovalLifecycleEvent`
- `ApprovalListRequest`
- `ApprovalPage`
- `ApprovalRecord`
- `ApprovalRejectRequest`
- `ApprovalState`
- `BudgetLimit`
- `BuyerMiddleware`
- `CHAINS_EVENT_SIGNATURES`
- `CHAINS_EVENT_TOPICS`
- `CHAINS_PRECOMPILE`
- `CHAINS_SELECTORS`
- `CHAINS_SIGNATURES`
- `ChainsClient`
- `Client`
- `ConfiguredReceiptAuthority`
- `ErrorClass`
- `FulfillmentStore`
- `HISTORY_DEFAULT_LIMIT`
- `HISTORY_MAXIMUM_ACCOUNTS`
- `HISTORY_MAXIMUM_LIMIT`
- `HUMAN_ERROR_CODES`
- `HUMAN_OPERATIONS`
- `HUMAN_RETRIABILITIES`
- `HistoryAssetMetadata`
- `HistoryPage`
- `HistoryRow`
- `HistorySide`
- `HoldReason`
- `HumanErrorCode`
- `HumanIntentClient`
- `HumanIntentError`
- `HumanRetriability`
- `INTENT_DOMAINS`
- `INTENT_ENDPOINT_KINDS`
- `IdempotencyKey`
- `IdempotentMutation`
- `IntentConstraints`
- `IntentDomain`
- `IntentEndpointKind`
- `IntentEndpointRef`
- `IntentLeg`
- `IntentLegBinding`
- `IntentMoney`
- `IntentOperation`
- `IntentPlan`
- `IntentSigningRequirement`
- `IntentSubmission`
- `LayerXKeyCredential`
- `LocalSignatureVerifier`
- `MirrorCandidate`
- `MirrorPolicy`
- `MirrorVerification`
- `MirrorVerificationError`
- `MirrorVerifier`
- `NativeBalanceView`
- `NativeCall`
- `NativeCapability`
- `NativeEffectPrepareRequestV1`
- `NativeEmitEvent`
- `NativeProgramCall`
- `NativeProgramDeploy`
- `NativeProgramLifecycleRequest`
- `NativeProgramRequest`
- `NativeProgramSpend`
- `NativeProgramUpgrade`
- `NativeProgramWindDown`
- `NativeReceiptRead`
- `NativeSharedStorageRead`
- `NativeSharedStorageWrite`
- `NativeStorageRead`
- `NativeStorageWrite`
- `NativeTransfer402`
- `OccupancyPayer`
- `Operation`
- `PX_MAXIMUM_JOINED_ASSETS`
- `PX_METHODS`
- `PaymentBudget`
- `PaymentCommitment`
- `PaymentEvidence`
- `PaymentRpc`
- `PaymentRpcError`
- `PlanIntentRequest`
- `PlatformSdkError`
- `PreparedGrantDraws`
- `ProductionClient`
- `ProductionTransport`
- `ProgramBoundaryError`
- `ProgramCall`
- `ProgramDiscovery`
- `ProgramInterface`
- `ProgramOperations`
- `ProgramSource`
- `ProgramTrustContext`
- `ProtocolAmount`
- `PxAccountBalances`
- `PxAccountJoin`
- `PxAssetBalance`
- `PxAssetEntry`
- `PxAssetTable`
- `PxClient`
- `PxCustodyAsset`
- `PxMethod`
- `PxNetworkHead`
- `PxPaxeerAccount`
- `PxPaxeerAssetBalance`
- `PxPaxeerHead`
- `PxResolvedIdentities`
- `PxRpcError`
- `ReceiptFailureCode`
- `ReceiptKey`
- `ReceiptVerification`
- `ReceiptVerificationError`
- `ResumableStream`
- `RetryClass`
- `Rung`
- `SdkErrorCode`
- `SdkTelemetry`
- `SecretBytes`
- `SellerMiddleware`
- `Sequence`
- `SequencerReceiptVerification`
- `SequencerReceiptVerificationError`
- `StreamCursor`
- `StreamEvent`
- `StreamPage`
- `StructuredActivityDisclosure`
- `SubmissionCommitted`
- `SubmissionReceipted`
- `SubmissionReverted`
- `SubmissionState`
- `SubmissionSubmitted`
- `SubmitPlanRequest`
- `TimestampSeconds`
- `Transport`
- `UnifiedHistoryPage`
- `VerifiedProgramReceipt`
- `VerifiedRead`
- `WEB_CONTENT_DOMAIN`
- `WEB_CONTENT_FETCH`
- `WEB_CONTENT_SEARCH`
- `WEB_SEARCH_CURRENCIES`
- `WEB_SEARCH_MAX_RESULTS`
- `WEB_SEARCH_PAYER_HEADER`
- `WebContent`
- `WebContentResponse`
- `WebFetchResponse`
- `WebSearchAssetTerms`
- `WebSearchClient`
- `WebSearchError`
- `WebSearchExactPayment`
- `WebSearchMeteredPayment`
- `WebSearchOffer`
- `WebSearchPayer`
- `WebSearchPreference`
- `WebSearchResponse`
- `WebSearchResult`
- `WebSettlement`
- `XWEB_API_VERSION`
- `XWEB_DEFAULT_MAX_PAYLOAD_BYTES`
- `XWEB_ENVELOPE_INFO`
- `XWEB_KIND_API`
- `XWEB_LEVEL_MAJORITY`
- `XWEB_LEVEL_SINGLE`
- `XWEB_METHOD_GET`
- `XWEB_METHOD_POST`
- `XWEB_PRECOMPILE`
- `XWebApiCall`
- `XWebApiError`
- `XWebApiHeader`
- `XWebApiPayload`
- `XWebApiRequest`
- `XWebAttestor`
- `XWebAttestorSet`
- `XWebEnvelopeRandomness`
- `XWebPrecompileCall`
- `XWebSealedEnvelope`
- `ai_market`
- `bind_receive_activity`
- `build_xweb_api_request`
- `chain_rpc_url`
- `chains_call`
- `content_digest`
- `decode_chains_call`
- `decode_chains_event`
- `decode_chains_params`
- `decode_get_chain`
- `decode_header`
- `decode_history_asset_metadata`
- `decode_history_row`
- `decode_human_envelope`
- `decode_human_error`
- `decode_intent_plan`
- `decode_intent_submission`
- `decode_native_capability_set`
- `decode_native_program_call`
- `decode_paxeer_history_page`
- `decode_payer_grant`
- `decode_px_account_balances`
- `decode_px_account_join`
- `decode_px_asset_balance`
- `decode_px_asset_table`
- `decode_px_custody_asset`
- `decode_px_network_head`
- `decode_px_resolved_identities`
- `decode_receive`
- `decode_unified_history_page`
- `decode_web_content`
- `decode_xweb_api_payload`
- `decode_xweb_attestors`
- `derive_native_asset_id`
- `derive_native_program_account`
- `encode_account_open`
- `encode_asset_register`
- `encode_asset_supply`
- `encode_grant`
- `encode_grant_revoke`
- `encode_header`
- `encode_native_capability_set`
- `encode_native_effect_prepare_request`
- `encode_native_program_call`
- `encode_plan_intent_request`
- `encode_receive`
- `encode_submit_plan_request`
- `encode_xweb_api_payload`
- `encode_xweb_credential`
- `grant_authorization_message`
- `grant_payment_header`
- `history_account`
- `history_cursor`
- `history_params`
- `layerx_sdk_py_package`
- `narrow_native_capability_set`
- `parse_amount`
- `parse_budget_limit`
- `parse_sequence`
- `parse_timestamp_seconds`
- `payment_commitment`
- `payment_payer`
- `payment_purpose`
- `platform_sdk_programs`
- `platform_sdk_python`
- `px_account_key`
- `px_optional_quantity`
- `px_quantity`
- `receive_authorization_message`
- `require_verified`
- `rpc_committed_evidence`
- `rpc_hex`
- `rpc_payment_evidence`
- `seal_xweb_envelope`
- `validate_grant_draw`
- `validate_payload`
- `validate_required`
- `validate_requirements`
- `verify_payment_receipt`
- `verify_rpc_payment`
- `verify_sequencer_receipt`
- `virtual_chain_id`
- `web_content_bytes`
- `xweb_api_origin`
- `xweb_api_request_call`
- `xweb_attestor_address`
- `xweb_get_attestors_call_data`
- `xweb_method_code`

## Rust (`programs/sdk/rust`)

Crate [`layerx-program-sdk`](../../programs/sdk/rust/Cargo.toml) (`layerx_program_sdk`), root [`src/lib.rs`](../../programs/sdk/rust/src/lib.rs). This is the `no_std` program guest SDK. Host-call bindings and `entry` are available on `wasm32`; `bindgen` and its re-exported generator types are available on non-`wasm32` targets. Other targets can use the pure types and codecs without a running host.

### Public modules

| Module | Availability |
|---|---|
| [`abi`](../../programs/sdk/rust/src/abi.rs) | all targets (host-call functions may be target-gated) |
| [`abi_policy`](../../programs/sdk/rust/src/abi_policy.rs) | all targets (host-call functions may be target-gated) |
| [`access`](../../programs/sdk/rust/src/access.rs) | all targets (host-call functions may be target-gated) |
| [`amount`](../../programs/sdk/rust/src/amount.rs) | all targets (host-call functions may be target-gated) |
| [`arbiter`](../../programs/sdk/rust/src/arbiter.rs) | all targets (host-call functions may be target-gated) |
| [`balance`](../../programs/sdk/rust/src/balance.rs) | all targets (host-call functions may be target-gated) |
| [`bindgen`](../../programs/sdk/rust/src/bindgen.rs) | non-`wasm32` only |
| [`buffer`](../../programs/sdk/rust/src/buffer.rs) | all targets (host-call functions may be target-gated) |
| [`call`](../../programs/sdk/rust/src/call.rs) | all targets (host-call functions may be target-gated) |
| [`capability`](../../programs/sdk/rust/src/capability.rs) | all targets (host-call functions may be target-gated) |
| [`context`](../../programs/sdk/rust/src/context.rs) | all targets (host-call functions may be target-gated) |
| [`crypto`](../../programs/sdk/rust/src/crypto.rs) | all targets (host-call functions may be target-gated) |
| [`error`](../../programs/sdk/rust/src/error.rs) | all targets (host-call functions may be target-gated) |
| [`event`](../../programs/sdk/rust/src/event.rs) | all targets (host-call functions may be target-gated) |
| [`ids`](../../programs/sdk/rust/src/ids.rs) | all targets (host-call functions may be target-gated) |
| [`lxt20`](../../programs/sdk/rust/src/lxt20.rs) | all targets (host-call functions may be target-gated) |
| [`lxt721`](../../programs/sdk/rust/src/lxt721.rs) | all targets (host-call functions may be target-gated) |
| [`naming`](../../programs/sdk/rust/src/naming.rs) | all targets (host-call functions may be target-gated) |
| [`oracle`](../../programs/sdk/rust/src/oracle.rs) | all targets (host-call functions may be target-gated) |
| [`payments`](../../programs/sdk/rust/src/payments.rs) | all targets (host-call functions may be target-gated) |
| [`proposal`](../../programs/sdk/rust/src/proposal.rs) | all targets (host-call functions may be target-gated) |
| [`receipt`](../../programs/sdk/rust/src/receipt.rs) | all targets (host-call functions may be target-gated) |
| [`storage`](../../programs/sdk/rust/src/storage.rs) | all targets (host-call functions may be target-gated) |
| [`transfer`](../../programs/sdk/rust/src/transfer.rs) | all targets (host-call functions may be target-gated) |
| [`web`](../../programs/sdk/rust/src/web.rs) | all targets (host-call functions may be target-gated) |
| [`entry`](../../programs/sdk/rust/src/entry.rs) | `wasm32` only |

### Crate-root re-exports

Grouped `pub use` declarations expose the individual names below at the crate root. Their kinds come from their declarations in the defining module.

| Item | Kind | Defined in | Availability |
|---|---|---|---|
| `HostFunction` | struct | `abi` | all targets |
| `ABI_MANIFEST` | const | `abi` | all targets |
| `ABI_MODULE` | const | `abi` | all targets |
| `ABI_VERSION` | const | `abi` | all targets |
| `CALL_ENTRY_EXPORT` | const | `abi` | all targets |
| `CALL_RESERVE_EXPORT` | const | `abi` | all targets |
| `CANDIDATE_ABI_MANIFEST` | const | `abi` | all targets |
| `CANDIDATE_ABI_MODULE` | const | `abi` | all targets |
| `CANDIDATE_HOST_FUNCTIONS` | const | `abi` | all targets |
| `CANDIDATE_REFUSAL_SENTINEL` | const | `abi` | all targets |
| `ENTRYPOINT` | const | `abi` | all targets |
| `HOST_FUNCTIONS` | const | `abi` | all targets |
| `MAX_CALL_INPUT_BYTES` | const | `abi` | all targets |
| `MAX_CALL_RESPONSE_BYTES` | const | `abi` | all targets |
| `MAX_CANONICAL_CAPABILITY_SET_BYTES` | const | `abi` | all targets |
| `MAX_CAPABILITIES` | const | `abi` | all targets |
| `MAX_CAPABILITY_ENCODING_BYTES` | const | `abi` | all targets |
| `MAX_CAPABILITY_ENCODING_GRANT_BYTES` | const | `abi` | all targets |
| `MAX_CAPABILITY_ENCODING_HEADER_BYTES` | const | `abi` | all targets |
| `MAX_EVENTS_PER_ACTIVITY` | const | `abi` | all targets |
| `MAX_EVENT_DATA_BYTES` | const | `abi` | all targets |
| `MAX_EVENT_TOPIC_BYTES` | const | `abi` | all targets |
| `MAX_PROGRAM_ACCOUNT_SEED_BYTES` | const | `abi` | all targets |
| `MAX_REFUSAL_REASON_BYTES` | const | `abi` | all targets |
| `MAX_STORAGE_KEY_BYTES` | const | `abi` | all targets |
| `MAX_STORAGE_VALUE_BYTES` | const | `abi` | all targets |
| `MEMORY_EXPORT` | const | `abi` | all targets |
| `RECEIPT_ENCODING_BYTES` | const | `abi` | all targets |
| `AccessEntry` | enum | `access` | all targets |
| `AccessMode` | enum | `access` | all targets |
| `AccessRecipe` | struct | `access` | all targets |
| `AccessScope` | enum | `access` | all targets |
| `ActivityAccess` | enum | `access` | all targets |
| `KeyAccess` | enum | `access` | all targets |
| `Amount` | struct | `amount` | all targets |
| `ProtocolInteger` | trait | `amount` | all targets |
| `BindgenError` | enum | `bindgen` | non-`wasm32` only |
| `BindingGenerator` | struct | `bindgen` | non-`wasm32` only |
| `GeneratedBindings` | struct | `bindgen` | non-`wasm32` only |
| `Bytes` | struct | `buffer` | all targets |
| `CallInput` | struct | `call` | all targets |
| `CallResponse` | struct | `call` | all targets |
| `CallResult` | struct | `call` | all targets |
| `GrantedCapabilities` | struct | `call` | all targets |
| `Capability` | enum | `capability` | all targets |
| `CapabilitySet` | struct | `capability` | all targets |
| `Context` | struct | `context` | all targets |
| `Principal` | struct | `context` | all targets |
| `EntryResponse` | struct | `entry` | `wasm32` only |
| `Field` | enum | `error` | all targets |
| `HostRefusal` | enum | `error` | all targets |
| `ProgramError` | enum | `error` | all targets |
| `ProgramRefusal` | struct | `error` | all targets |
| `Reason` | enum | `error` | all targets |
| `RefusalClass` | enum | `error` | all targets |
| `RefusalReason` | struct | `error` | all targets |
| `ValueError` | struct | `error` | all targets |
| `REFUSAL_CLASS_MANIFEST` | const | `error` | all targets |
| `STATUS_BOUNDS` | const | `error` | all targets |
| `STATUS_BUFFER_TOO_SMALL` | const | `error` | all targets |
| `STATUS_CAPABILITY_BYTES` | const | `error` | all targets |
| `STATUS_CAPABILITY_LIMIT` | const | `error` | all targets |
| `STATUS_DATA_TOO_LARGE` | const | `error` | all targets |
| `STATUS_DENIED` | const | `error` | all targets |
| `STATUS_DUPLICATE_CAPABILITY` | const | `error` | all targets |
| `STATUS_EMPTY_KEY` | const | `error` | all targets |
| `STATUS_EMPTY_TOPIC` | const | `error` | all targets |
| `STATUS_EVIDENCE` | const | `error` | all targets |
| `STATUS_INPUT_TOO_LARGE` | const | `error` | all targets |
| `STATUS_INVALID` | const | `error` | all targets |
| `STATUS_KEY_TOO_LARGE` | const | `error` | all targets |
| `STATUS_METER` | const | `error` | all targets |
| `STATUS_NULL_ARGUMENT` | const | `error` | all targets |
| `STATUS_OVERFLOW` | const | `error` | all targets |
| `STATUS_RECEIPT_ENCODING` | const | `error` | all targets |
| `STATUS_RESERVED_IDENTIFIER` | const | `error` | all targets |
| `STATUS_TOPIC_TOO_LARGE` | const | `error` | all targets |
| `STATUS_UNDERFLOW` | const | `error` | all targets |
| `STATUS_VALUE_TOO_LARGE` | const | `error` | all targets |
| `STATUS_VERIFY_FAILED` | const | `error` | all targets |
| `STATUS_ZERO_AMOUNT` | const | `error` | all targets |
| `EventData` | struct | `event` | all targets |
| `EventTopic` | struct | `event` | all targets |
| `AccountId` | struct | `ids` | all targets |
| `AssetId` | struct | `ids` | all targets |
| `ProgramId` | struct | `ids` | all targets |
| `ReceiptDigest` | struct | `ids` | all targets |
| `Observation` | struct | `oracle` | all targets |
| `OBSERVATION_BYTES` | const | `oracle` | all targets |
| `Receipt` | struct | `receipt` | all targets |
| `StorageKey` | struct | `storage` | all targets |
| `StorageValue` | struct | `storage` | all targets |
| `Payment` | struct | `transfer` | all targets |
| `ProgramAccountPayment` | struct | `transfer` | all targets |
| `ProgramAccountSeed` | struct | `transfer` | all targets |
| `ProgramDeposit` | struct | `transfer` | all targets |
| `Answer` | struct | `web` | all targets |
| `ANSWER_HEADER_BYTES` | const | `web` | all targets |
| `MAX_RESPONSE_BYTES` | const | `web` | all targets |
| `RECORD_BYTES` | const | `web` | all targets |

### Reference functions and macros

The crate-root reference helpers are **const functions**, callable on all targets:

| Function | Signature | Returned repository path |
|---|---|---|
| `programs_escrow_reference` | `pub const fn programs_escrow_reference() -> &'static str` | [`programs/sdk/rust/examples/escrow`](../../programs/sdk/rust/examples/escrow) |
| `programs_vault_reference` | `pub const fn programs_vault_reference() -> &'static str` | [`programs/sdk/rust/examples/vault`](../../programs/sdk/rust/examples/vault) |

[`macros.rs`](../../programs/sdk/rust/src/macros.rs) exports `program!`, `entrypoint!`, `callable!`, `response_entrypoint!`, `failure_entrypoint!`, and `trap_on_panic!`. They generate program entry/panic code for the guest; `callable!` aliases `entrypoint!`.

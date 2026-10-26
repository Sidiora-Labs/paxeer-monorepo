# clients.md: client contract (revision 2a1033aa2)

| Item | Value |
|---|---|
| Contract | how every first-party client (SDKs, apps, CLIs, middleware) sends, reads, identifies itself and verifies receipts after the merge |
| Version | 2.0.0 |
| Change control | changes only by amendment; each amendment raises the version |
| Conformance | method names, selectors, events and error codes belong to the construct contracts; RPC shapes to contracts/rpc.md; receipt fields and key pins to contracts/sequencer.md; this file takes theirs |
| Citations | every `path:line` is relative to the repository root at revision 2a1033aa2 and was read by the generator; `(planned)` marks a file that does not exist at HEAD |

## (a) Operations

Every client operation is one of the following kinds.

| Code | Operation | Rule | Source at HEAD |
|---|---|---|---|
| E | user write | EIP-1559 (type 2) transaction signed with the user's secp256k1 key, sent with `eth_sendRawTransaction`; the sender address is bound once to the DID at addr 0x1004 | `Eip1559Call` `services/agent/crates/layerx-crypto/src/evm_transaction.rs:13`, `sign` `services/agent/crates/layerx-crypto/src/evm_transaction.rs:207` |
| S | sponsored write | the gas station quotes and submits a sponsored batch; the user still signs the authorisation | `/quote`, `/submit` `services/interop/crates/layerx-gas-station/src/service.rs:289-290` |
| R | receipt read | `pax_sequencerReceipt(txHash)` returns the rung and, from rung receipted on, the signed receipt | `rpc/sequencer_receipt.go:26` |
| Q | state read | `eth_call`, `eth_getBalance`, `eth_getTransactionCount` at tag `pending` or a committed height; joined reads through the router `px_*` methods | `px_resolveAccount` `services/platform/hosted/gateway/src/rpc.rs:1140`, `px_getBalances` `services/platform/hosted/gateway/src/rpc.rs:356`, `px_listAssets` `services/platform/hosted/gateway/src/rpc.rs:357`, `px_getAccount` `services/platform/hosted/gateway/src/evm.rs:832`, `px_getNetwork` `services/platform/hosted/gateway/src/evm.rs:824`, `px_getHistory` `services/platform/hosted/gateway/src/rpc.rs:1145` |
| B | binding | the DID key signs the bind message once; the EVM sender calls addr `bindLayerX` | domain `LX:PAXEER-BIND:v1` `modules/evm/types/layerx_binding.go:18`; `bindLayerX` `precompiles/addr/addr.go:39` |
| W | subscription | `eth_subscribe` `logs`, `newHeads`, `newPendingTransactions`; no lx_ subscription | section (d) rows L20-L22 |

Rungs reported by `pax_sequencerReceipt`:

| Rung | Meaning | Source |
|---|---|---|
| submitted | the sequencer holds the transaction without a receipt | `rpc/sequencer_receipt.go:32` |
| receipted | a signed receipt exists; its batch is not committed | `rpc/sequencer_receipt.go:34` |
| committed | the receipt is covered by the MsgSequenceBatch of a committed block | `rpc/sequencer_receipt.go:36` |

A client reports a rung only from a verified receipt (section (c)). A submitted transaction is pending, never paid or done. A client never replaces a submitted transaction with another of the same nonce.

## (b) Identity and keys

| Item | Rule | Source at HEAD |
|---|---|---|
| DID binding | one EVM address per DID binding, written by `bindLayerX` and removed by `unbindLayerX`; the actor DID of a transaction is `getLayerXDid(msg.sender)` | `precompiles/addr/addr.go:39-41` |
| bind message | "LX:PAXEER-BIND:v1" followed by the chain id (uint256 big-endian), the EVM address and the nonce, signed by the ed25519 DID key | `modules/evm/types/layerx_binding.go:18`, `modules/evm/types/layerx_binding.go:41` |
| EVM key | secp256k1; signs every user write (code E) | `services/agent/crates/layerx-crypto/src/secp256k1.rs` |
| DID key | ed25519; signs bind messages and off-chain authorisations, never a transaction | `services/agent/crates/layerx-crypto/src/ed25519.rs` |
| registration | `px_register` admits the tenant and key at the identity service; the binding digest domain is `layerx-register-binding-v1` | `services/platform/hosted/gateway/src/rpc_register.rs:404`, `services/platform/hosted/gateway/src/rpc_register.rs:12` |
| embedded wallet | the user key exists only as attestor shares, threshold three of five | `services/wallet/gateway/src/attestor/client.ts:533` |

## (c) Receipt verification

A client that acts on a receipt (releases goods, reports paid, advances a plan) verifies it first:

1. Decode the canonical receipt bytes returned by `pax_sequencerReceipt`.
2. Select the signing key from the ReceiptKeys pins by key id and height; a key whose height range does not cover the receipt height is refused (`ReceiptKeys.KeyForHeight` `modules/sequencer/keeper/receipt_keys.go:96`).
3. Verify the signature and the receipt fields (`ReceiptKeys.VerifyReceipt` `modules/sequencer/keeper/receipt_keys.go:112`); field layout is contracts/sequencer.md.
4. Match the transaction hash, the payment log (address, topic, amount) and the rung the client requires; a mismatch is a verification failure, not a retry.

Verifiers at HEAD and their direction:

| Verifier | At HEAD | Direction |
|---|---|---|
| Go chain-side | `ReceiptKeys.VerifyReceipt` over the chain-state pins `modules/sequencer/keeper/receipt_keys.go:112` | stable; the reference for every other verifier |
| Go proof library | `verify.ReceiptSignature` against one sequencer public key `layerxproof/verify/receipt.go:49` | single-key baseline |
| Go platform SDK | `AuthorizedBatch` verification `sdk/platform/go/verify.go:37` | kernel batch form; moves to ReceiptKeys |
| Rust agent SDK | offline verification `services/agent/crates/layerx-sdk/src/lib.rs:413` | takes ReceiptKeys instead of an authorised batch (decision.925) |
| Receipt authority | one `sequencer_public_key` `services/platform/hosted/authority/src/lib.rs:184` | planned receipt keys file (contracts/services.md#platform-receipt-authority) |
| x402 seller (Rust) | `layerx_proof::receipt::verify` of a kernel receipt `services/interop/crates/layerx-x402/src/seller.rs:247` | sequencer receipt with ReceiptKeys (task 11.4.2) |
| Seller middleware (JS) | conformance service reads planned `LAYERX_RECEIPT_KEYS_JSON` (planned) | criteria under [k3r-receipt-checks](#k3r-receipt-checks) |

<a id="lx-switch-off"></a>

## (d) Switch-off

No `lx_*` method is registered by the node: it answers -32601 for every `lx_*` name on HTTP and WebSocket (decision.701). The hosted gateway still routes them at HEAD; task 8.3.1 makes every `lx_` name answer -32601 on HTTP and WebSocket and deletes the OpenRPC entries in the same change (decision.803). The relay archive names L23-L26 retire with the relay archive.

Server lines are the gateway and relay archive files; callers are every other file (gateway tests and OpenRPC included) under `services/`, `sdk/`, `apps/`, `tools/`, `programs/` (directories .git, .next, dist, node_modules, target, vendor skipped), matched as whole words at HEAD. Up to six caller files are listed per name, each with up to four lines.

| # | name | server at HEAD | caller files | callers | replacement | tasks |
|---|---|---|---|---|---|---|
| L1 | `lx_sendActivity` | `services/platform/hosted/gateway/src/rpc.rs:341,400,1129`; `services/platform/relay_archive/README.md:210`; `services/platform/relay_archive/forward.py:259` | 23 | `services/agent/crates/layerx-sdk/src/rpc.rs:937`; `services/agent/sdk/typescript/src/rpc.ts:117`; `services/platform/middleware/seller/src/rpc.ts:56`; `services/platform/hosted/testnet/tests/payment-journey.py:198,249`; `services/platform/hosted/testnet/tests/program-journey.py:258`; `services/platform/hosted/gateway/README.md:32` | `eth_sendRawTransaction(rawTx)` then `pax_sequencerReceipt(txHash)`; sponsored sends go to the gas station `POST /submit` | 8.3.1, 8.32.1, 11.30.1, 11.34.5, 11.36.1 |
| L2 | `lx_getSequence` | `services/platform/hosted/gateway/src/rpc.rs:237,286,318,345` | 31 | `services/agent/crates/layerx-sdk/src/rpc.rs:747,794`; `services/agent/sdk/typescript/src/rpc.ts:76,79`; `services/platform/hosted/testnet/tests/payment-journey.py:161,233,258`; `services/platform/hosted/testnet/tests/program-journey.py:172`; `services/platform/hosted/gateway/openrpc.json:687`; `services/platform/hosted/gateway/tests/unified_service_paths.rs:429` | `eth_getTransactionCount(addr, "pending")`; sponsored: `eth_call` of BatchCallAndSponsor `nonce()` at the agent address, tag pending | 8.3.1, 8.48.2, 11.36.1, 12.24.5, 12.25.2, 12.25.3, 12.25.4 |
| L3 | `lx_getAccount` | `services/platform/hosted/gateway/src/rpc.rs:286,342,1123,1238` | 19 | `services/agent/crates/layerx-sdk/src/rpc_verification.rs:100,401`; `services/agent/crates/layerx-sdk/src/rpc.rs:746`; `services/agent/crates/layerx-sdk/tests/account_reads.rs:584,586`; `services/agent/sdk/typescript/src/rpc.ts:73`; `services/platform/hosted/testnet/tests/payment-journey.py:155,232,255`; `services/platform/hosted/gateway/openrpc.json:167,250` | `px_getAccount` | 8.3.1, 12.24.5, 12.25.2, 12.25.3, 12.25.4 |
| L4 | `lx_getBalance` | `services/platform/hosted/gateway/src/rpc.rs:286,318,343,1124` | 13 | `services/agent/crates/layerx-sdk/src/rpc_verification.rs:100,416`; `services/agent/crates/layerx-sdk/src/rpc.rs:746`; `services/agent/crates/layerx-sdk/tests/account_reads.rs:585`; `services/agent/sdk/typescript/src/rpc.ts:74`; `services/platform/hosted/testnet/tests/payment-journey.py:154,230,252`; `services/platform/hosted/testnet/tests/program-journey.py:262,337` | `eth_getBalance(addr, "pending")`; layerxledger `balanceOf(bytes32,bytes32)` by `eth_call` | 8.3.1 |
| L5 | `lx_getBalances` | `services/platform/hosted/gateway/src/rpc.rs:287,306,319,344` | 18 | `services/agent/crates/layerx-sdk/src/rpc_verification.rs:212,431`; `services/agent/crates/layerx-sdk/src/rpc.rs:805`; `services/agent/crates/layerx-sdk/tests/account_reads.rs:837,838`; `services/agent/sdk/typescript/src/rpc.ts:75`; `services/platform/hosted/testnet/tests/payment-journey.py:133`; `services/platform/hosted/gateway/openrpc.json:299` | `px_getBalances` | 8.3.1, 8.48.3, 12.24.5, 12.25.2, 12.25.3, 12.25.4 |
| L6 | `lx_getReceipt` | `services/platform/hosted/gateway/src/rpc.rs:288,348,1130,1241`; `services/platform/relay_archive/runtime.py:1227` | 21 | `services/agent/crates/layerx-sdk/src/rpc_verification.rs:605`; `services/agent/crates/layerx-sdk/src/rpc.rs:748`; `services/agent/sdk/typescript/src/rpc.ts:85`; `services/platform/middleware/seller/src/rpc.ts:58`; `services/platform/hosted/testnet/tests/payment-journey.py:208,260`; `services/platform/hosted/gateway/openrpc.json:365,448` | `pax_sequencerReceipt(txHash)`; `eth_getTransactionReceipt(txHash)` for committed transactions | 8.3.1, 8.32.1, 8.48.3, 11.34.5, 11.36.1, 12.24.5, 12.25.2, 12.25.3 (+1) |
| L7 | `lx_getActivityStatus` | `services/platform/hosted/gateway/src/rpc.rs:288,349,1131,1242`; `services/platform/relay_archive/runtime.py:1221` | 16 | `services/agent/crates/layerx-sdk/src/rpc.rs:748`; `services/agent/sdk/typescript/src/rpc.ts:86`; `services/platform/middleware/seller/src/rpc.ts:59`; `services/platform/hosted/gateway/openrpc.json:431`; `services/platform/hosted/gateway/tests/unified_service_paths.rs:434`; `services/platform/hosted/gateway/tests/local/lifecycle.rs:1608` | the `rung` field of `pax_sequencerReceipt` | 8.3.1, 8.32.1, 11.34.5, 11.36.1, 12.24.5, 12.25.2, 12.25.3, 12.25.4 |
| L8 | `lx_getBatchHeader` | `services/platform/hosted/gateway/src/rpc.rs:289,297,350,1132`; `services/platform/relay_archive/runtime.py:1232` | 7 | `services/agent/crates/layerx-sdk/src/rpc.rs:749`; `services/agent/sdk/typescript/src/rpc.ts:87`; `services/platform/hosted/gateway/openrpc.json:497`; `services/platform/hosted/gateway/tests/unified_service_paths.rs:435`; `tools/platform/cli/src/rpc.rs:108,110,491,704`; `tools/platform/cli/tests/fixtures/openrpc.json:497` | `eth_getBlockByNumber(H, false)` | 8.3.1, 8.32.1 |
| L9 | `lx_getCheckpoint` | `services/platform/hosted/gateway/src/rpc.rs:290,351,1133,1243` | 8 | `services/agent/crates/layerx-sdk/src/rpc_verification.rs:583`; `services/agent/crates/layerx-sdk/src/rpc.rs:749`; `services/agent/sdk/typescript/src/rpc.ts:88`; `services/platform/hosted/gateway/openrpc.json:563`; `services/platform/hosted/gateway/tests/unified_service_paths.rs:436`; `tools/platform/cli/src/rpc.rs:98,492,641` | the consensus commit at H read from an operator full node | 8.3.1 |
| L10 | `lx_getProof` | `services/platform/hosted/gateway/src/rpc.rs:250,352,1134,1277` | 9 | `services/agent/crates/layerx-sdk/src/rpc_verification.rs:661`; `services/agent/crates/layerx-sdk/src/rpc.rs:922`; `services/agent/sdk/typescript/src/rpc.ts:113`; `services/platform/hosted/gateway/openrpc.json:764`; `services/platform/hosted/gateway/tests/unified_service_paths.rs:437`; `services/platform/hosted/gateway/tests/local/lifecycle.rs:1294` | `eth_getProof` at H; below the prune horizon from the archive node | 8.3.1 |
| L11 | `lx_getAsset` | `services/platform/hosted/gateway/src/rpc.rs:291,354,1055,1065` | 13 | `services/agent/crates/layerx-sdk/src/rpc.rs:891`; `services/agent/sdk/typescript/src/rpc.ts:101`; `services/platform/hosted/gateway/openrpc.json:1094`; `services/platform/hosted/gateway/tests/unified_service_paths.rs:439`; `services/platform/hosted/gateway/tests/local/lifecycle.rs:1118,2349,2424,2428`; `tools/platform/cli/README.md:194` | `px_listAssets` plus bank `name`, `symbol`, `decimals`, `supply` | 8.3.1, 8.48.3 |
| L12 | `lx_listAssets` | `services/platform/hosted/gateway/src/rpc.rs:274,353,409,1048` | 11 | `services/agent/crates/layerx-sdk/src/rpc.rs:7,9,401,862`; `services/agent/sdk/typescript/test/rpc.test.ts:83`; `services/agent/sdk/typescript/src/rpc.ts:93`; `services/platform/hosted/gateway/openrpc.json:1015`; `services/platform/hosted/gateway/tests/unified_service_paths.rs:438`; `services/platform/hosted/gateway/tests/local/lifecycle.rs:1106,2343` | `px_listAssets` | 8.3.1, 8.48.3 |
| L13 | `lx_estimateFee` | `services/platform/hosted/gateway/src/rpc.rs:346,417,1127` | 8 | `services/agent/crates/layerx-sdk/src/rpc.rs:905`; `services/agent/sdk/typescript/src/rpc.ts:109`; `services/platform/hosted/gateway/README.md:29`; `services/platform/hosted/gateway/openrpc.json:1161`; `services/platform/hosted/gateway/tests/local/lifecycle.rs:1114,2390,2417,2432`; `tools/platform/cli/src/rpc.rs:146,148,501,571` | `eth_estimateGas(tx, "pending")` multiplied by the gas price | 8.3.1, 8.48.1 |
| L14 | `lx_getProgramEvents` | `services/platform/hosted/gateway/src/rpc.rs:347,413,802,846` | 14 | `services/platform/hosted/gateway/openrpc.json:1230`; `services/platform/hosted/gateway/tests/local/events.rs:565,694`; `services/platform/hosted/gateway/tests/fixtures/program-events.json:4,54,78,116`; `services/interop/crates/x-websearch/README.md:11`; `services/interop/crates/x-websearch/src/kernel.rs:55`; `services/interop/crates/x-websearch/tests/binary.rs:204` | `eth_getLogs` on layerxprograms 0x101D | 8.3.1, 11.36.1 |
| L15 | `lx_getNodeInfo` | `services/platform/hosted/gateway/src/evm.rs:823`; `services/platform/hosted/gateway/src/rpc.rs:278,355,1137,1213`; `services/platform/relay_archive/runtime.py:1217` | 10 | `services/agent/crates/layerx-sdk/src/rpc_verification.rs:567`; `services/agent/crates/layerx-sdk/src/rpc.rs:853`; `services/agent/sdk/typescript/src/rpc.ts:89`; `services/platform/hosted/gateway/openrpc.json:629`; `services/platform/hosted/gateway/tests/unified_service_paths.rs:440`; `services/platform/hosted/gateway/tests/local/lifecycle.rs:1018` | `px_getNetwork` and `eth_chainId` | 8.3.1 |
| L16 | `lx_getWalletCaps` | `services/platform/hosted/gateway/src/rpc.rs:368` | 6 | `services/platform/hosted/gateway/openrpc.json:2493`; `sdk/wallet/test/modules.test.ts:611,684,687,692`; `apps/wallet/src/surfaces/WebDataView.test.tsx:296,315`; `tools/qualification/paxeer-x/wallet-402-caps.py:147,229,733,734`; `tools/paxeer-x/retired_callers.json:541,543`; `tools/paxeer-x/route-catalogue.json:44656,44659` | layerxpay `getBudget(bytes32)`, layerxledger `getPayerGrant(bytes32)`, layerxauth `getGrant(bytes32)` by `eth_call` | 8.3.1 |
| L17 | `lx_getHistory` | `services/platform/hosted/gateway/src/history.rs:1966` | 7 | `services/platform/hosted/gateway/tests/unified_service_paths.rs:416,427,428`; `apps/web/e2e/gateway-history.test.ts:236`; `tools/platform/cli/src/rpc.rs:508`; `tools/platform/cli/tests/fixtures/openrpc.json:1606`; `tools/bringup/check-live.sh:3716,3881,3883,3884`; `tools/bringup/check-live.test.sh:2399,2449,2472` | `px_getHistory` | none |
| L18 | `lx_register` | `services/platform/hosted/gateway/src/rpc_register.rs:402,426` | 7 | `services/agent/crates/layerx-sdk/src/rpc.rs:1062`; `services/platform/hosted/gateway/tests/unified_service_paths.rs:396,415`; `apps/web/e2e/gateway-history.test.ts:236`; `tools/platform/cli/src/register.rs:211`; `tools/platform/cli/src/rpc.rs:484`; `tools/platform/cli/tests/fixtures/openrpc.json:10` | `px_register`, then the user-signed addr 0x1004 `bindLayerX` | none |
| L19 | `lx_requestFunds` | `services/platform/hosted/gateway/src/rpc.rs:340,1122`; `services/platform/hosted/gateway/src/rpc_faucet.rs:373` | 9 | `services/agent/crates/layerx-sdk/src/rpc.rs:785`; `services/platform/hosted/gateway/openrpc.json:87`; `services/platform/hosted/gateway/tests/unified_service_paths.rs:441`; `tools/platform/cli/README.md:83`; `tools/platform/cli/src/faucet.rs:18,143`; `tools/platform/cli/src/rpc.rs:58,60,90,302` | the faucet claim path behind the router; the service block names `px_requestFunds`, which no gateway file serves at HEAD | 8.3.1, 8.50.2 |
| L20 | `lx_subscribe` | `services/platform/hosted/gateway/src/rpc.rs:404,1138`; `services/platform/hosted/gateway/src/ws.rs:593,942` | 10 | `services/agent/crates/layerx-sdk/src/rpc_subscription.rs:189`; `services/agent/sdk/typescript/src/rpc-subscription.ts:103`; `services/platform/hosted/gateway/README.md:26,27`; `services/platform/hosted/gateway/openrpc.json:937,990`; `services/platform/hosted/gateway/tests/local/lifecycle.rs:1124,2254,2267,2288`; `tools/platform/cli/src/rpc_subscription.rs:19,29` | `eth_subscribe` with `logs`, `newHeads` or `newPendingTransactions` | 8.3.1, 8.50.1 |
| L21 | `lx_unsubscribe` | `services/platform/hosted/gateway/src/rpc.rs:404,1139`; `services/platform/hosted/gateway/src/ws.rs:593,942` | 8 | `services/agent/crates/layerx-sdk/src/rpc_subscription.rs:68`; `services/agent/sdk/typescript/src/rpc-subscription.ts:120`; `services/platform/hosted/gateway/README.md:27`; `services/platform/hosted/gateway/openrpc.json:980`; `tools/platform/cli/src/rpc.rs:52,153,155,359`; `tools/platform/cli/tests/rpc_identity.rs:119` | `eth_unsubscribe` | 8.3.1 |
| L22 | `lx_subscription` | `services/platform/hosted/gateway/src/ws.rs:593,942,1188` | 7 | `services/agent/crates/layerx-sdk/src/rpc_subscription.rs:75,390,919,958`; `services/agent/sdk/typescript/test/rpc.test.ts:115,118,120`; `services/agent/sdk/typescript/src/rpc-subscription.ts:46`; `services/platform/hosted/gateway/openrpc.json:977`; `services/platform/hosted/gateway/tests/local/lifecycle.rs:2272`; `tools/platform/cli/src/rpc_subscription.rs:128,148` | `eth_subscription` notifications | 8.3.1 |
| L23 | `lx_getArchiveNetwork` | `services/platform/relay_archive/runtime.py:1217` | 0 | none | none; the relay archive retires and history is served by `eth_*` on the archive node | none |
| L24 | `lx_getArchiveHead` | `services/platform/relay_archive/runtime.py:1219` | 0 | none | none; the relay archive retires | 8.32.1 |
| L25 | `lx_listActivities` | `services/platform/relay_archive/runtime.py:1239` | 0 | none | none; the relay archive retires | 8.32.1 |
| L26 | `lx_listBatches` | `services/platform/relay_archive/runtime.py:1259` | 0 | none | none; the relay archive retires | 8.32.1 |
| total | 26 names | | 275 | | | |

Replacement methods served at HEAD:

| method | served at |
|---|---|
| `pax_sequencerReceipt` | `services/platform/hosted/gateway/src/main.rs:4523` |
| `px_getAccount` | `services/platform/hosted/gateway/src/evm.rs:832` |
| `px_getBalances` | `services/platform/hosted/gateway/src/rpc.rs:356` |
| `px_listAssets` | `services/platform/hosted/gateway/src/rpc.rs:357` |
| `px_getNetwork` | `services/platform/hosted/gateway/src/evm.rs:824` |
| `px_getHistory` | `services/platform/hosted/gateway/src/history.rs:546` |
| `px_register` | `services/platform/hosted/gateway/src/rpc_register.rs:404` |
| `px_requestFunds` | not served at HEAD |

Decisions that rule the switch-off:

- decision.701: lx_* at HEAD. No lx_* method is registered in rpc/; the node answers -32601 for every lx_* name on HTTP and WS. The gateway still routes lx_* and its switch-off belongs to domain 8.
- decision.803: lx_ switch-off is one change across HTTP, WebSocket and OpenRPC. Every lx_ name answers -32601 from dispatch before backend admission, the WebSocket session sends lx_ frames to the same dispatcher, and the OpenRPC lx_ entries are deleted in the same task, because the schema test and the unified service test read all three.
- decision.925: layerx-sdk reads through eth_* and px_*, writes EIP-1559 layerxledger calls and verifies receipts against pinned receipt keys. RpcClient replaces every lx_ method by the clients.md section (d) replacement, Wallet builds layerxledger calls signed by the secp256k1 key, verify_receipt takes ReceiptKeys instead of an AuthorizedBatch, and the SDK stops re-exporting SettlementDomain; the kernel activity decoders in agent_envelope, programs, program_lifecycle and program_http decode raw EIP-1559 transactions instead.
- decision.1201: lx_send_authorization is evaluated under its own policy kind. Add policy kind lx_send_authorization and retire lx_activity as both a sign kind and a policy kind; at HEAD a send authorization is evaluated under policy kind lx_activity.
- decision.1202: EVM review is a new route family beside the narrowed lx routes. Add attestor POST /v1/evm/review and gateway /v1/wallet/evm/review, /approve, /approvals/:id and /sign; the /v1/wallet/lx/* routes keep serving lx_send_authorization only instead of being re-pointed in place.
- decision.1204: Activity types narrow to the asset send 0x10005. ATTESTOR_ACTIVITY_TYPES admits only 0x10005 after lx_activity retires.
- decision.1421: Keep the lx_* tables and stop writing the retired ones. No drop migration; ingestion of 0x1013 and 0x1014 stops and the custody, deposit-root and guarantor schemas and runners are deleted.

## (e) Conformance

Each criterion below has exactly one test; the test runs through the client's real code path.

### k3r-receipt-checks

Receipt checks a client runs on a sequencer receipt before acting on it.

Acceptance criteria (37):

| criterion | scenario | test |
|---|---|---|
| 8243.1 | Given a raw EIP-1559 transaction, its signed receipt with one PaymentExact log for 25 units and the signing key pinned with range [1, 1000] covering height 7, when verifyPaymentReceipt runs, then it returns rung receipted and txHash equal to keccak256 of the raw transaction. | `services/platform/middleware/seller/test/receipt-verification.test.mjs::verifyPaymentReceiptAcceptsSignedSequencerReceipt (planned)` |
| 8243.2 | Given the same receipt and the key pinned with range [100, 200], when verifyPaymentReceipt runs, then it throws MiddlewareError verification-failure. | `services/platform/middleware/seller/test/receipt-verification.test.mjs::verifyPaymentReceiptRefusesKeyOutsideHeightRange (planned)` |
| 8243.3 | Given a valid signed receipt whose PaymentExact amount is 24 and requirements for 25, when verifyPaymentReceipt runs, then it throws MiddlewareError verification-failure. | `services/platform/middleware/seller/test/receipt-verification.test.mjs::verifyPaymentReceiptRefusesAmountMismatch (planned)` |
| 8243.4 | Given a payment payload whose receiptDigest has one flipped byte, when ReceiptPayloadAuthority.settle runs, then it throws MiddlewareError verification-failure and resolves nothing. | `services/platform/middleware/seller/test/receipt-verification.test.mjs::receiptPayloadAuthorityRefusesDigestMismatch (planned)` |
| 8244.1 | Given a receipted verification at height 7, landing block 7 and a commit signed by 3 of 4 equal-power validators, when verifyPaymentCommitmentEvidence runs with rung committed, then it resolves without error. | `services/platform/middleware/seller/test/commitment-rungs.test.mjs::committedRungAcceptsTwoThirdsCommit (planned)` |
| 8244.2 | Given the same evidence with a commit signed by 2 of 3 equal-power validators, when verifyPaymentCommitmentEvidence runs with rung committed, then it throws code verification-failure. | `services/platform/middleware/seller/test/commitment-rungs.test.mjs::committedRungRefusesTwoThirdsExactly (planned)` |
| 8244.3 | Given a receipted verification at height 7 and landing block 6 with a valid commit, when verifyPaymentCommitmentEvidence runs with rung committed, then it throws code verification-failure. | `services/platform/middleware/seller/test/commitment-rungs.test.mjs::committedRungRefusesEarlierLandingBlock (planned)` |
| 8254.1 | Given a spend request for 25 units and a signed receipt with a matching PaymentExact log, when verifyAgentPayment runs, then it returns rung receipted. | `services/platform/middleware/agent/test/agent-receipt.test.mjs::verifyAgentPaymentMatchesLedgerLog (planned)` |
| 8254.2 | Given a committed reservation recording digest D and evidence whose signed receipt hashes to another digest, when verifyCommittedAgentPayment runs, then it throws AgentMiddlewareError budget-conflict. | `services/platform/middleware/agent/test/agent-receipt.test.mjs::verifyCommittedAgentPaymentRefusesOtherDigest (planned)` |
| 8258.1 | Given a ConformanceSequencer and facts for 10 units, when buildSignedReceipt runs and verifyPaymentReceipt checks the result, then verification returns rung receipted. | `services/platform/middleware/conformance/test/receipts.test.mjs::conformanceReceiptVerifiesUnderSellerVerifier (planned)` |
| 8258.2 | Given a built receipt whose signedReceipt log amount byte is flipped, when verifyPaymentReceipt runs, then it throws verification-failure. | `services/platform/middleware/conformance/test/receipts.test.mjs::conformanceReceiptRefusesTamperedLog (planned)` |
| 8258.3 | Given any payee, asset and amount, when offerFixture runs, then requirements.network is eip155:125 and extra.layerx.commitment is receipted. | `services/platform/middleware/conformance/test/receipts.test.mjs::offerFixtureUsesEip155Network (planned)` |
| 8262.1 | Given a validator set of four equal-power keys and a commit signed by three of them, when verifyPaymentCommitmentEvidence checks the committed receipt, then it returns rung committed. | `services/platform/middleware/conformance/pay6/committed-rung.test.mjs::committedFixtureAcceptsThreeOfFourValidators (planned)` |
| 8262.2 | Given the same commit and a validator set with one key replaced, when verifyPaymentCommitmentEvidence runs, then it throws verification-failure. | `services/platform/middleware/conformance/pay6/committed-rung.test.mjs::committedFixtureRefusesValidatorSetMismatch (planned)` |
| 8267.1 | Given a bound on-ramp order, when compile_operator_send runs, then call.to is LEDGER_ADDRESS and the sixth calldata word equals order.order_digest. | `services/platform/ramps/toolkit/tests/evm_order.rs::on_ramp_compiles_pay_exact_with_order_reference (planned)` |
| 8267.2 | Given a bound off-ramp order with payer grant G, when compile_payer_grant_draw runs, then the draw from word is the customer account id and the grantId word is G. | `services/platform/ramps/toolkit/tests/evm_order.rs::off_ramp_compiles_draw_against_customer_grant (planned)` |
| 8267.3 | Given an on-ramp order and a receipt signed by a pinned key covering its height with a matching PaymentExact log, when verify_order_receipt runs, then it returns the leg with rung receipted. | `services/platform/ramps/toolkit/tests/evm_order.rs::verify_order_receipt_accepts_matching_payment_log (planned)` |
| 8267.4 | Given the same receipt and a key entry whose lastHeight is the receipt height minus one, when verify_order_receipt runs, then it returns RampError::Receipt. | `services/platform/ramps/toolkit/tests/evm_order.rs::verify_order_receipt_refuses_key_outside_height_range (planned)` |
| 8268.1 | Given an otherwise valid LayerxConfig with chain_id 0, when build runs, then it returns Err containing chain_id. | `services/platform/ramps/toolkit/tests/evm_config.rs::layerx_config_refuses_zero_chain_id (planned)` |
| 8268.2 | Given a receipt key entry with firstHeight 10 and lastHeight 9, when build runs, then it returns Err naming receipt_keys. | `services/platform/ramps/toolkit/tests/evm_config.rs::layerx_config_refuses_inverted_receipt_key_range (planned)` |
| 8268.3 | Given a JSON LayerxConfig with every new field plus gateway_endpoint, when serde_json parses it, then parsing fails with unknown field gateway_endpoint. | `services/platform/ramps/toolkit/tests/evm_config.rs::layerx_config_rejects_retired_gateway_field (planned)` |
| 8269.1 | Given a prepared on-ramp leg and a signature over its signing hash by a key other than the operator, when the leg is assembled and submitted, then the result is an error and the listener saw no eth_sendRawTransaction. | `services/platform/ramps/toolkit/tests/evm_submit.rs::signature_from_other_key_is_refused_before_submission (planned)` |
| 8269.2 | Given a correctly signed leg and a listener answering the transaction hash, when submit_prepared runs, then the listener saw one eth_sendRawTransaction with the raw bytes and the submission names the hash. | `services/platform/ramps/toolkit/tests/evm_submit.rs::submit_prepared_sends_raw_transaction_once (planned)` |
| 8269.3 | Given a listener answering pax_sequencerReceipt with status submitted, when resolve runs, then it returns Pending with the transaction hash. | `services/platform/ramps/toolkit/tests/evm_submit.rs::resolve_reports_submitted_receipt_as_pending (planned)` |
| 8270.1 | Given the four flag pairs with --activity-id in third position, when the binary runs, then it exits with status 2. | `services/platform/ramps/toolkit/tests/verify_receipt_cli.rs::verify_receipt_cli_refuses_activity_id_flag (planned)` |
| 8270.2 | Given --transaction-hash 0x1234 and valid protected config and order files, when the binary runs, then it exits with status 2 and writes no output file. | `services/platform/ramps/toolkit/tests/verify_receipt_cli.rs::verify_receipt_cli_refuses_short_transaction_hash (planned)` |
| 8271.1 | Given a journal in which a verified leg with rung committed was recorded, when the journal is reopened, then the order snapshot evidence holds the same tx_hash, receipt_digest and rung. | `services/platform/ramps/toolkit/tests/engine_legs.rs::journal_records_transaction_hash_and_rung (planned)` |
| 8271.2 | Given a journal file holding v1 records for one settled order, when it is opened, then health is ready and the order stage is unchanged. | `services/platform/ramps/toolkit/tests/engine_legs.rs::journal_replays_version_one_records (planned)` |
| 8275.1 | Given protected config and order files and a bare 64-hex argument, when --verify-receipt runs, then it exits non-zero and stderr names TRANSACTION_HASH. | `services/platform/ramps/reference/tests/work_routes.rs::verify_receipt_mode_refuses_activity_hex (planned)` |
| 8275.2 | Given a running reference ramp and an operator token, when a work request with a payload field is posted, then the answer is 400 work_invalid. | `services/platform/ramps/reference/tests/work_routes.rs::work_route_refuses_native_payload_field (planned)` |
| 8278.1 | Given the journey script, when it is run with --help, then the output omits LAYERX_RAMP_ON_ACCOUNT_SEQUENCE and contains LAYERX_RAMP_OFF_RECEIVER_SEQUENCE. | `services/platform/tests/test_ramp_sandbox_journey.py::test_journey_help_names_no_on_account_sequence (planned)` |
| 8278.2 | Given the journey script text, when every --verify-receipt invocation is read, then each passes a value taken from tx_hash. | `services/platform/tests/test_ramp_sandbox_journey.py::test_journey_verifies_by_transaction_hash (planned)` |
| 8284.1 | Given a listener answering pax_sequencerReceipt with a receipt signed by key K at height 12 and K pinned for heights 1 to 100, when resolve runs for the transaction hash, then it returns the receipt with rung receipted. | `services/platform/examples/test/runtime.test.mjs::sequencerReceiptClientReturnsVerifiedReceipt (planned)` |
| 8284.2 | Given a listener answering pax_sequencerReceipt with status submitted, when resolve runs, then it throws a LayerXApplicationStateError whose state is pending. | `services/platform/examples/test/runtime.test.mjs::sequencerReceiptClientReportsSubmittedAsPending (planned)` |
| 8284.3 | Given a variable holding one entry with firstHeight 9 and lastHeight 8, when receiptKeysFromEnvironment reads it, then it throws. | `services/platform/examples/test/runtime.test.mjs::receiptKeysFromEnvironmentRefusesInvertedRange (planned)` |
| 8286.1 | Given CLI output holding transaction hash H and a listener answering a signed receipt for H with status 1, when verifyProgramOutcome runs, then it returns state completed, rung receipted and the receipt digest. | `services/platform/examples/marketplace/test/receipt-outcome.test.mjs::programOutcomeCompletesOnSuccessfulReceipt (planned)` |
| 8286.2 | Given a receipt for H signed by a key absent from the pinned entries, when verifyProgramOutcome runs, then it throws with state refused. | `services/platform/examples/marketplace/test/receipt-outcome.test.mjs::programOutcomeRefusesReceiptUnderUnpinnedKey (planned)` |

### c10-x402-conformance

x402 conformance of the seller middleware: rung names, network identifiers and payment terms.

Acceptance criteria (26):

| criterion | scenario | test |
|---|---|---|
| 8242.1 | Given requirements extras {}, {layerx:{commitment:"receipted"}} and {layerx:{commitment:"committed"}}, when paymentCommitment is called on each, then it returns receipted, receipted and committed. | `services/platform/middleware/seller/test/evm-terms.test.mjs::paymentCommitmentParsesRungNames (planned)` |
| 8242.2 | Given extras naming commitment executed, batched and finalised, when paymentCommitment is called on each, then each call throws code verification-failure. | `services/platform/middleware/seller/test/evm-terms.test.mjs::paymentCommitmentRefusesKernelLevels (planned)` |
| 8242.3 | Given two otherwise identical seller configurations with networks layerx:beta and eip155:125, when each SellerMiddleware is constructed, then the first throws MiddlewareError unsupported-payment and the second constructs. | `services/platform/middleware/seller/test/evm-terms.test.mjs::sellerRefusesLayerxNetworkIdentifiers (planned)` |
| 8249.1 | Given a buyer supporting exact on eip155:125 without commitments, when parseOffer reads a receipted offer and then a committed offer, then the first returns the offer and the second throws unsupported-payment. | `services/platform/middleware/buyer/test/evm-terms.test.mjs::buyerAcceptsReceiptedOfferWithoutResolver (planned)` |
| 8249.2 | Given a buyer configuration listing {scheme exact, network layerx:beta}, when BuyerMiddleware is constructed, then it throws unsupported-payment. | `services/platform/middleware/buyer/test/evm-terms.test.mjs::buyerRefusesLayerxSupportedKind (planned)` |
| 8252.1 | Given an agent middleware without commitments, when spend validation reads commitment {eip155:125, receipted}, then it raises no invalid-request. | `services/platform/middleware/agent/test/evm-terms.test.mjs::agentSpendAcceptsEip155ReceiptedCommitment (planned)` |
| 8252.2 | Given requests with commitment {layerx:beta, receipted} and {eip155:125, finalised}, when spend validation runs on each, then each throws invalid-request. | `services/platform/middleware/agent/test/evm-terms.test.mjs::agentSpendRefusesKernelCommitmentTerms (planned)` |
| 8256.1 | Given a signed settlement webhook body with verification committed, when consume parses it, then parsing raises no invalid-webhook. | `services/platform/middleware/merchant/test/evm-terms.test.mjs::settlementWebhookAcceptsRungNames (planned)` |
| 8256.2 | Given signed webhook bodies with verification sequencer-signed and settlement-anchored, when consume parses each, then each is refused with invalid-webhook. | `services/platform/middleware/merchant/test/evm-terms.test.mjs::settlementWebhookRefusesKernelLevels (planned)` |
| 8259.1 | Given the built middleware packages, when runScenarios runs, then the suite holds at least one result and every result is ok. | `services/platform/middleware/conformance/test/scenarios.test.mjs::runScenariosPassesEveryCheck (planned)` |
| 8259.2 | Given the suite returned by runScenarios, when its results are searched for the committed rung check, then exactly one such result exists and it is ok. | `services/platform/middleware/conformance/test/scenarios.test.mjs::runScenariosChecksCommittedRungIsNotDowngraded (planned)` |
| 8260.1 | Given a fresh Suite, when runServiceScenarios runs it, then every recorded CheckResult is ok. | `services/platform/middleware/conformance/test/service-scenarios.test.mjs::runServiceScenariosPassesAgainstExampleApps (planned)` |
| 8260.2 | Given the example config file the service scenarios write, when it is read back, then receiptKeysEnvironment is LAYERX_RECEIPT_KEYS_JSON and neither retired key is present. | `services/platform/middleware/conformance/test/service-scenarios.test.mjs::serviceConfigCarriesReceiptKeysEnvironment (planned)` |
| 8263.1 | Given the committed evm-receipt.json, when generate_evm_receipt.mjs writes a new copy to a temporary path, then both files are byte-identical. | `services/platform/middleware/conformance/pay6/evm-receipt.test.mjs::evmReceiptFixtureMatchesGenerator (planned)` |
| 8263.2 | Given evm-receipt.json, when verifyPaymentReceipt runs under its key and under a copy whose lastHeight is one below the receipt height, then the first returns receipted and the second throws verification-failure. | `services/platform/middleware/conformance/pay6/evm-receipt.test.mjs::evmReceiptFixtureVerifiesUnderPinnedKey (planned)` |
| 8287.1 | Given a config copy whose emulator network is layerx:beta and a listener counting requests, when the buyer agent runs, then it prints state refused, exits 4 and the listener saw no request. | `services/platform/examples/test/buyer-agent.test.mjs::buyerAgentRefusesLayerxNetworkConfig (planned)` |
| 8287.2 | Given buyer-agent/layerx.example.json, when both environment objects are read, then each has rpcUrl and receiptKeysEnvironment and lacks both retired keys. | `services/platform/examples/test/buyer-agent.test.mjs::buyerAgentExampleConfigHasNoReceiptAuthority (planned)` |
| 8290.1 | Given the reference applications after tasks 8.33.1 to 8.34.1, when the runner is run with --check, then it exits 0 and prints checked names with environments emulator and beta. | `services/platform/examples/test/run-reference-apps.test.mjs::referenceAppsCheckPassesWithoutRetiredSurfaces (planned)` |
| 8291.1 | Given an outcome set that is otherwise complete but whose buyer row carries verification sequencer-signed, when the qualifier assertions run, then they fail naming the buyer payment. | `services/platform/examples/test/qualify-reference-apps.test.mjs::qualifierRefusesSequencerSignedLabel (planned)` |
| 8292.1 | Given an environment holding only PATH and HOME, when agent-spend/index.mjs runs, then it exits non-zero and stderr does not contain missing_layerx_protocol_version. | `services/platform/examples/test/agent-spend.test.mjs::agentSpendStartsWithoutProtocolVersion (planned)` |
| 8293.1 | Given an unsigned EIP-1559 transaction T and preparation_ref keccak256(T), when preparedTransactionReference runs, then it returns that reference. | `services/platform/examples/test/agent-spend-services.test.mjs::preparedReferenceMatchesKeccakOfUnsignedTransaction (planned)` |
| 8293.2 | Given T and preparation_ref sha256(T), when preparedTransactionReference runs, then it throws prepared_bytes_mismatch. | `services/platform/examples/test/agent-spend-services.test.mjs::preparedReferenceRefusesSha256Reference (planned)` |
| 8294.1 | Given the served offer for the conformance payee on eip155:125, when the buyer parses its header, then accepted.network is eip155:125 and extra.layerx.commitment is receipted. | `services/platform/middleware/conformance/test/service-account.test.mjs::serviceOfferParsesOnEip155ReceiptedRung (planned)` |
| 8294.2 | Given the served offer with network layerx:beta, when the buyer parses its header, then it throws MiddlewareError unsupported-payment. | `services/platform/middleware/conformance/test/service-account.test.mjs::serviceOfferOnLayerxNetworkIsRefused (planned)` |
| 8295.1 | Given a JSON-RPC listener that answers eth_call to 0x000000000000000000000000000000000000101d with a refused simulateCall outcome, when walkthrough.sh runs against it, then it prints status typed-refusal and the listener saw no eth_sendRawTransaction. | `services/platform/examples/test/agent-program-call.test.mjs::walkthroughSimulatesThroughProgramsPrecompile (planned)` |
| 8295.2 | Given a JSON-RPC listener that answers simulateCall completed, eth_sendRawTransaction with a transaction hash and pax_sequencerReceipt with a receipted receipt, when walkthrough.sh runs against it, then its JSON output has status called, that tx_hash, rung receipted and no activity_id. | `services/platform/examples/test/agent-program-call.test.mjs::walkthroughReportsTransactionHashAndRung (planned)` |

### o52-x402-builders

Builders of the layerxledger draw calldata and the PaymentExact log decoder.

Acceptance criteria (13):

| criterion | scenario | test |
|---|---|---|
| 8241.1 | Given a draw with amount 1000000 and fixed grant, asset and context ids, when encodeLedgerDraw is called, then the result is 228 bytes, starts with the draw selector and word three ends with 0x0f4240. | `services/platform/middleware/seller/test/ledger.test.mjs::encodesLedgerDrawCalldataWithDrawSelector (planned)` |
| 8241.2 | Given a log from 0x...101a with the PaymentExact topic0, three indexed topics and data holding assetId and amount 25, when decodePaymentExactLog is called, then it returns those five fields with amount 25n. | `services/platform/middleware/seller/test/ledger.test.mjs::decodesPaymentExactLogFromLedgerPrecompile (planned)` |
| 8241.3 | Given the PaymentExact log of the previous test re-addressed to 0x...101b, when each decoder is called, then each throws MiddlewareError with code verification-failure. | `services/platform/middleware/seller/test/ledger.test.mjs::refusesLedgerLogFromAnotherAddress (planned)` |
| 8247.1 | Given a metered requirement for 40 units and a raw transaction calling draw on 0x101A for 40 units, when settle runs, then execute receives that rawTransaction and the idempotency key. | `services/platform/middleware/seller/test/grant-draw.test.mjs::grantAuthorityForwardsLedgerDrawTransaction (planned)` |
| 8247.2 | Given the same transaction addressed to 0x...101b, when settle runs, then it throws MiddlewareError invalid-payment-payload and execute is not called. | `services/platform/middleware/seller/test/grant-draw.test.mjs::grantAuthorityRefusesDrawToAnotherAddress (planned)` |
| 8247.3 | Given a metered payload carrying a 1466-hex receive, when settle runs, then it throws MiddlewareError invalid-payment-payload. | `services/platform/middleware/seller/test/grant-draw.test.mjs::grantAuthorityRefusesKernelReceivePayload (planned)` |
| 8251.1 | Given a metered offer for 40 units and a raw draw transaction for 40 units, when grantPaymentHeader runs, then the decoded payload rawTransaction equals the input and idempotencyKey equals the calldata key. | `services/platform/middleware/buyer/test/grant-draw-header.test.mjs::grantHeaderCarriesRawDrawTransaction (planned)` |
| 8251.2 | Given the same offer and a draw for 39 units, when grantPaymentHeader runs, then it throws MiddlewareError requirements-mismatch. | `services/platform/middleware/buyer/test/grant-draw-header.test.mjs::grantHeaderRefusesAmountMismatch (planned)` |
| 8251.3 | Given a grant payment header and a response naming another transaction hash, when captureGrantSettlement runs, then it throws verification-failure. | `services/platform/middleware/buyer/test/grant-draw-header.test.mjs::captureGrantSettlementRequiresDrawHash (planned)` |
| 8255.1 | Given a reserved budget of 40 units and a draw execution answering a receipt with PaymentDrawn amount 40, when execute runs, then it returns kind settled and retainVerified receives the 0x transaction hash. | `services/platform/middleware/agent/test/agent-grant.test.mjs::agentGrantSettlesDrawWithPaymentDrawnLog (planned)` |
| 8255.2 | Given a reserved budget of 40 units and a draw for 41 units, when execute runs, then it throws budget-conflict and the draw execution count is 0. | `services/platform/middleware/agent/test/agent-grant.test.mjs::agentGrantRefusesDrawAboveReservation (planned)` |
| 8261.1 | Given grant terms for 40 units and a fixed signing key, when the draw transaction is built and decoded, then every decoded word equals the input and the transaction hash equals keccak256 of the raw transaction. | `services/platform/middleware/conformance/pay6/ledger-draw.test.mjs::ledgerDrawTransactionBindsGrantTerms (planned)` |
| 8261.2 | Given a JSON-RPC listener recording calls, when PaymentRpc submits the draw, then the listener saw one eth_sendRawTransaction with the raw bytes followed by pax_sequencerReceipt with the same hash. | `services/platform/middleware/conformance/pay6/ledger-draw.test.mjs::paymentRpcSubmitsLedgerDrawOnce (planned)` |

### o53-x402-verification

x402 payment verification against the sequencer receipt.

Acceptance criteria (12):

| criterion | scenario | test |
|---|---|---|
| 8246.1 | Given a seller for network eip155:125 and a valid receipted payment, when handle runs, then the decoded PAYMENT-RESPONSE has success true, transaction 0x<txHash> and network eip155:125. | `services/platform/middleware/seller/test/settlement-response.test.mjs::settlementResponseNamesTransactionHash (planned)` |
| 8246.2 | Given the same paid request, when handle runs, then extensions.layerx.format is layerx-receipt-proof-v2, verificationLevel is receipted and rawTransaction equals the submitted raw transaction. | `services/platform/middleware/seller/test/settlement-response.test.mjs::settlementResponseCarriesPortableReceipt (planned)` |
| 8246.3 | Given a fulfilled request, when the same request is handled again, then the authority settle count stays 1 and the second PAYMENT-RESPONSE names the same transaction hash. | `services/platform/middleware/seller/test/settlement-response.test.mjs::replayedSettlementAnswersStoredTransaction (planned)` |
| 8250.1 | Given a done journey with one receipted evidence item whose PortableReceiptV2 pays the offer, when prepare runs, then it returns kind ready and the payload rawTransaction equals the evidence rawTransaction. | `services/platform/middleware/buyer/test/portable-receipts.test.mjs::prepareSendsPortableReceiptFromEvidence (planned)` |
| 8250.2 | Given a prepared payment and two responses differing only in transaction (lxp:<digest> and 0x<txHash>), when captureSettlement runs on each, then the first throws verification-failure and the second returns the settlement. | `services/platform/middleware/buyer/test/portable-receipts.test.mjs::captureSettlementRequiresTransactionHash (planned)` |
| 8250.3 | Given a done journey whose only evidence is a receipt-verified kernel receipt, when prepare runs, then evidence.get is not called and prepare throws verification-failure. | `services/platform/middleware/buyer/test/portable-receipts.test.mjs::findMatchingReceiptIgnoresKernelEvidence (planned)` |
| 8257.1 | Given an awaiting-payment order and a webhook naming transaction 0x<txHash> whose receipt pays the quote, when consume runs, then the order state is paid-verified with that transaction. | `services/platform/middleware/merchant/test/settlement-webhook.test.mjs::webhookMarksOrderPaidWithTransactionHash (planned)` |
| 8257.2 | Given the same order and a webhook naming another transaction hash, when consume runs, then it is refused with invalid-webhook and the order stays awaiting-payment. | `services/platform/middleware/merchant/test/settlement-webhook.test.mjs::webhookRefusesTransactionOtherThanReceiptHash (planned)` |
| 8285.1 | Given the paid API started on the emulator environment with receipt keys in LAYERX_RECEIPT_KEYS_JSON, when /paid is fetched without payment, then the answer is 402 and its single accepts entry names eip155:125 and receipted. | `services/platform/examples/test/paid-api.test.mjs::paidApiAdvertisesEip155ReceiptedOffer (planned)` |
| 8285.2 | Given the emulator environment without LAYERX_RECEIPT_KEYS_JSON, when the paid API starts, then it exits non-zero and stderr names missing_layerx_receipt_keys_json. | `services/platform/examples/test/paid-api.test.mjs::paidApiRefusesStartWithoutReceiptKeys (planned)` |
| 8288.1 | Given merchant-checkout started with receipt keys and webhook keys in its environment, when a checkout is posted, then the payment-required offer names eip155:125 and receipted. | `services/platform/examples/test/merchant-app.test.mjs::merchantCheckoutOfferUsesReceiptedRung (planned)` |
| 8288.2 | Given an open order and a correctly signed settlement webhook whose transaction names hash A while the receipt resolves for hash B, when the webhook is posted, then GET of the order still reports it unpaid. | `services/platform/examples/test/merchant-app.test.mjs::merchantWebhookRefusesTransactionMismatch (planned)` |

### o54-seller-rpc

Seller RPC: send, receipt lookup and pending reporting.

Acceptance criteria (7):

| criterion | scenario | test |
|---|---|---|
| 8245.1 | Given a loopback JSON-RPC endpoint at /rpc recording requests, when send is called with a raw transaction, then the recorded method is eth_sendRawTransaction with params [0x<raw>] and the returned hash equals the endpoint answer. | `services/platform/middleware/seller/test/payment-rpc.test.mjs::paymentRpcSendsRawTransaction (planned)` |
| 8245.2 | Given the same endpoint, when receipt is called with a 32-byte hash, then the recorded method is pax_sequencerReceipt with params [0x<hash>]. | `services/platform/middleware/seller/test/payment-rpc.test.mjs::paymentRpcReadsSequencerReceipt (planned)` |
| 8245.3 | Given a submitted answer for the expected hash and a receipted answer for another hash, when verifyRpcPayment runs on each, then the first returns kind pending and the second throws verification-failure. | `services/platform/middleware/seller/test/payment-rpc.test.mjs::verifyRpcPaymentReportsSubmittedAsPending (planned)` |
| 8264.1 | Given a listener answering eth_sendRawTransaction and pax_sequencerReceipt for a receipt paying the offer, when the example runs, then it exits 0 and prints the 0x transaction hash. | `services/platform/middleware/examples/test/public-rpc.test.mjs::nodeExampleVerifiesReceiptedPayment (planned)` |
| 8264.2 | Given a receipt keys file whose single entry lacks lastHeight, when the example runs, then it exits non-zero and the listener saw no eth_sendRawTransaction. | `services/platform/middleware/examples/test/public-rpc.test.mjs::nodeExampleRefusesReceiptKeyWithoutRange (planned)` |
| 8265.1 | Given a listener answering eth_sendRawTransaction and pax_sequencerReceipt for a receipt paying the offer, when the example runs, then it exits 0. | `services/platform/middleware/examples/test_public_rpc.py::test_python_example_verifies_receipted_payment (planned)` |
| 8265.2 | Given the example invoked with --authority FILE, when argparse parses it, then the process exits 2 naming the unrecognised argument. | `services/platform/middleware/examples/test_public_rpc.py::test_python_example_has_no_authority_option (planned)` |

### wiring-integration

Cross-domain integration criteria filed against this contract by the wiring job.

Acceptance criteria (10):

| criterion | test |
|---|---|
| 8961.1 | `tools/platform/cli/tests/no_retired_client_modules.rs::cli_names_no_retired_client_module (planned)` |
| 8962.1 | `services/platform/hosted/gateway/tests/no_bridge_withdraw_kind.rs::gateway_tests_name_no_bridge_withdraw_request (planned)` |
| 8963.1 | `services/platform/hosted/indexer/tests/finality_evidence_v2.rs::finality_evidence_v2_matches_vector (planned)` |
| 8964.1 | `services/platform/hosted/tests/test_plain_listener_targets.py::test_make_targets_name_no_relay_archive (planned)` |
| 8965.1 | `services/platform/hosted/gateway/tests/explorer_capabilities_keys.rs::capabilities_answer_has_no_custody_or_anchor (planned)` |
| 8966.1 | `services/platform/hosted/registry/tests/program_state_eth_call.rs::registry_reads_program_state_through_eth_call (planned)` |
| 8967.1 | `services/platform/hosted/registry/tests/journal_chain_trust.rs::journal_verifies_deployments_from_chain_trust (planned)` |
| 8968.1 | `services/platform/hosted/core/tests/no_kernel_disclosure_codec.rs::platform_names_no_kernel_disclosure_or_send_codec (planned)` |
| 8969.1 | `tools/platform/emulator/tests/gateway.rs::lifecycle_executes_without_the_native_state_feed (planned)` |
| 12962.1 | `services/wallet/attestor/internal/policy/lx/testdata/kernelvectors/tests/no_kernel_codec.rs::kernelvectors_use_no_kernel_codec (planned)` |

## (f) Client packages

| package | code root | interfaces | non-stable | acceptance |
|---|---|---|---|---|
| [launchpad-sdk-shared-go](#launchpad-sdk-shared-go) | `sdk/launchpad/shared` | 3428 | 1 | 2 |
| [wallet-app](#wallet-app) | `apps/wallet` | 1289 | 49 | 23 |
| [launchpad-sdk-sdk](#launchpad-sdk-sdk) | `sdk/launchpad/packages/sdk` | 786 | 58 | 11 |
| [launchpad-sdk-sdk-v2](#launchpad-sdk-sdk-v2) | `sdk/launchpad/packages/sdk-v2` | 693 | 0 | 0 |
| [launchpad-sdk-production-contracts-v2](#launchpad-sdk-production-contracts-v2) | `sdk/launchpad/packages/production-contracts-v2` | 693 | 0 | 0 |
| [wallet-sdk](#wallet-sdk) | `sdk/wallet` | 603 | 141 | 43 |
| [launchpad-sdk-production-contracts](#launchpad-sdk-production-contracts) | `sdk/launchpad/packages/production-contracts` | 509 | 0 | 0 |
| [launchpad-sdk-protocol-go](#launchpad-sdk-protocol-go) | `sdk/launchpad/protocol` | 424 | 9 | 16 |
| [launchpad-web-app](#launchpad-web-app) | `apps/launchpad/web` | 141 | 30 | 26 |
| [wallet-demo](#wallet-demo) | `services/wallet/demo` | 29 | 2 | 3 |
| [launchpad-sdk-candles-sdk](#launchpad-sdk-candles-sdk) | `sdk/launchpad/packages/candles-sdk` | 24 | 0 | 0 |
| [launchpad-sdk-users-sdk](#launchpad-sdk-users-sdk) | `sdk/launchpad/packages/users-sdk` | 21 | 0 | 0 |
| [launchpad-mobile-app](#launchpad-mobile-app) | `apps/launchpad/mobile` | 17 | 12 | 15 |
| [launchpad-sdk-metadata-sdk](#launchpad-sdk-metadata-sdk) | `sdk/launchpad/packages/metadata-sdk` | 15 | 0 | 0 |
| [launchpad-sdk-ranking-algo-sdk](#launchpad-sdk-ranking-algo-sdk) | `sdk/launchpad/packages/ranking-algo-sdk` | 14 | 0 | 0 |
| [launchpad-sdk-sidiora-launchpad-v3](#launchpad-sdk-sidiora-launchpad-v3) | `sdk/launchpad/packages/sidiora-launchpad-v3` | 14 | 1 | 0 |
| [launchpad-sdk-stats-sdk](#launchpad-sdk-stats-sdk) | `sdk/launchpad/packages/stats-sdk` | 13 | 0 | 0 |
| [launchpad-sdk-indexer-sdk](#launchpad-sdk-indexer-sdk) | `sdk/launchpad/packages/indexer-sdk` | 10 | 0 | 0 |
| [explorer-smart-contract-verifier-metrics](#explorer-smart-contract-verifier-metrics) | `services/explorer/services/smart-contract-verifier` | 7 | 0 | 0 |
| [explorer-recaptcha-verifier-errors](#explorer-recaptcha-verifier-errors) | `services/explorer/services/libs` | 6 | 0 | 0 |
| [explorer-service-launcher-metrics](#explorer-service-launcher-metrics) | `services/explorer/services/libs` | 2 | 0 | 0 |
| [explorer-smart-contract-verifier-server-errors](#explorer-smart-contract-verifier-server-errors) | `services/explorer/services/smart-contract-verifier` | 2 | 0 | 0 |
| [launchpad-sdk-sidiora-v3](#launchpad-sdk-sidiora-v3) | `sdk/launchpad/packages/sidiora-launchpad-v3` | 1 | 1 | 3 |
| [explorer-smart-contract-verifier-server-metrics](#explorer-smart-contract-verifier-server-metrics) | `services/explorer/services/smart-contract-verifier` | 1 | 0 | 0 |

### launchpad-sdk-shared-go

Shared Go packages of the launchpad services. Code root: `sdk/launchpad/shared`.

| kind | stable | change | new | retire | total |
|---|---|---|---|---|---|
| env_var | 27 | 0 | 0 | 0 | 27 |
| go_exported | 3397 | 1 | 0 | 0 | 3398 |
| http_route | 3 | 0 | 0 | 0 | 3 |
| all | 3427 | 1 | 0 | 0 | 3428 |

Interfaces that change, are new or retire (1):

| interface | kind | status | signature | defined at |
|---|---|---|---|---|
| 4.go-shared-config-capabilities-preflight-engine-reconcile-creation-fee | go_exported | change | `func (e *PreflightEngine) ReconcileCreationFee(ctx context.Context, hash common.Hash, treasuryIdenti` | `sdk/launchpad/shared/config/capabilities/preflight.go:988` |

Covering tasks (1):

| task | wave | title | interfaces named (by status) |
|---|---|---|---|
| 4.55.1 | 1 | Accept a surface Treasury in ReconcileCreationFee | change 1 |

Acceptance criteria (2):

| criterion | test |
|---|---|
| 4535.1 | `sdk/launchpad/shared/config/capabilities/preflight_chain_test.go::TestReconcileCreationFeeAcceptsSurfaceTreasury (planned)` |
| 4535.2 | `sdk/launchpad/shared/config/capabilities/preflight_chain_test.go::TestReconcileCreationFeeRejectsSurfaceTreasuryWithImplementation (planned)` |

### wallet-app

Wallet app. Code root: `apps/wallet`.

| kind | stable | change | new | retire | total |
|---|---|---|---|---|---|
| env_var | 30 | 0 | 0 | 0 | 30 |
| error_code | 4 | 0 | 0 | 0 | 4 |
| event | 12 | 0 | 0 | 0 | 12 |
| http_route | 11 | 3 | 0 | 1 | 15 |
| storage_key | 18 | 0 | 0 | 1 | 19 |
| ts_export | 1165 | 34 | 0 | 10 | 1209 |
| all | 1240 | 37 | 0 | 12 | 1289 |

Interfaces that change, are new or retire (49):

| interface | kind | status | signature | defined at |
|---|---|---|---|---|
| 12.http-app-page-account | http_route | change | `GET /account (Next.js page) export default function AccountPage()` | `apps/wallet/src/app/account/page.tsx:14` |
| 12.http-app-page-account-deposit | http_route | retire | `GET /account/deposit (Next.js page) export default function AccountDepositPage()` | `apps/wallet/src/app/account/deposit/page.tsx:5` |
| 12.http-app-page-account-history | http_route | change | `GET /account/history (Next.js page) export default function AccountHistoryPage()` | `apps/wallet/src/app/account/history/page.tsx:6` |
| 12.http-app-page-account-plan | http_route | change | `GET /account/plan (Next.js page) export default function AccountPlanPage()` | `apps/wallet/src/app/account/plan/page.tsx:6` |
| 12.key-app-paxeer-custody-v2 | storage_key | retire | `localStorage paxeer:custody:v2:<identity id>:<chainId>:<address> -> retained custody deposit record` | `apps/wallet/src/account/DepositFlow.tsx:65` |
| 12.ts-app-account-deposit-page | ts_export | retire | `export default function AccountDepositPage()` | `apps/wallet/src/app/account/deposit/page.tsx:5` |
| 12.ts-app-account-history-page | ts_export | change | `export default function AccountHistoryPage()` | `apps/wallet/src/app/account/history/page.tsx:6` |
| 12.ts-app-account-hooks-use-kernel-state | ts_export | change | `export function useKernelState(): AsyncState<KernelAvailabilityState> & { readonly reload: () => voi` | `apps/wallet/src/account/hooks.ts:40` |
| 12.ts-app-account-page | ts_export | change | `export default function AccountPage()` | `apps/wallet/src/app/account/page.tsx:14` |
| 12.ts-app-account-plan-page | ts_export | change | `export default function AccountPlanPage()` | `apps/wallet/src/app/account/plan/page.tsx:6` |
| 12.ts-app-account-view | ts_export | change | `export function AccountView({ account }: { account: string })` | `apps/wallet/src/account/AccountView.tsx:18` |
| 12.ts-app-asset-list | ts_export | change | `export function AssetList({ account, assets }: { account: string; assets?: readonly CompletionAsset[` | `apps/wallet/src/account/AssetList.tsx:88` |
| 12.ts-app-beneficiary-of | ts_export | retire | `export function beneficiaryOf(layerxAccount: string): Hex` | `apps/wallet/src/account/custody.ts:22` |
| 12.ts-app-completion-assets | ts_export | change | `export function completionAssets(): CompletionAsset[]` | `apps/wallet/src/account/format.ts:10` |
| 12.ts-app-custody-abi | ts_export | retire | `export const CUSTODY_ABI = [` | `apps/wallet/src/account/custody.ts:7` |
| 12.ts-app-custody-call | ts_export | retire | `export interface CustodyCall` | `apps/wallet/src/account/custody.ts:36` |
| 12.ts-app-custody-deposit-error | ts_export | retire | `export class CustodyDepositError extends Error` | `apps/wallet/src/account/custody.ts:15` |
| 12.ts-app-custody-handoff | ts_export | retire | `export function custodyHandoff(call: CustodyCall): Hex` | `apps/wallet/src/account/custody.ts:43` |
| 12.ts-app-custody-handoff-domain | ts_export | retire | `export const CUSTODY_HANDOFF_DOMAIN = 'LX:CUSTODY:v1';` | `apps/wallet/src/account/custody.ts:5` |
| 12.ts-app-custody-precompile | ts_export | retire | `export const CUSTODY_PRECOMPILE = '0x0000000000000000000000000000000000001013' as const;` | `apps/wallet/src/account/custody.ts:4` |
| 12.ts-app-deposit-flow | ts_export | retire | `export function DepositFlow()` | `apps/wallet/src/account/DepositFlow.tsx:293` |
| 12.ts-app-deposit-token-calldata | ts_export | retire | `export function depositTokenCalldata(pointer: string, amount: bigint, beneficiary: Hex): Hex` | `apps/wallet/src/account/custody.ts:30` |
| 12.ts-app-fee-selection | ts_export | change | `export interface FeeSelection` | `apps/wallet/src/surfaces/useSurface.ts:185` |
| 12.ts-app-history-view | ts_export | change | `export function HistoryView({ account, pageSize = 20 }: { account: string; pageSize?: number })` | `apps/wallet/src/account/HistoryView.tsx:27` |
| 12.ts-app-kernel-gate | ts_export | change | `export type KernelGate =` | `apps/wallet/src/account/hooks.ts:50` |
| (24 more) | | | | |

Covering tasks (16):

| task | wave | title | interfaces named (by status) |
|---|---|---|---|
| 12.33.1 | 1 | Read grant caps observed at a block on a rung in the web data surface | change 1 |
| 12.34.1 | 1 | Label the account status ladder submitted, receipted and committed | change 1 |
| 12.35.1 | 1 | Complete a transfer at the committed rung in the transfer result | change 1 |
| 12.33.2 | 3 | Send module transactions through the receipted send in the surfaces | change 6 |
| 12.33.3 | 4 | Apply the fee token preference through the receipted send | change 2 |
| 12.33.4 | 5 | Read the created market from the receipt logs in the launchpad view | change 1 |
| 12.33.5 | 5 | Await the swap receipt and refuse a reverted swap in the swap executor | change 1 |
| 12.34.3 | 5 | Bind kernel plan legs with the pending nonce of the wallet address | change 5 |
| 12.34.4 | 5 | Read kernel state and the chain account document in the account hooks | change 6 |
| 12.34.5 | 5 | Show the chain account in the account view and notice | change 2 |
| 12.34.6 | 5 | Page the chain history in the history view | change 3 |
| 12.34.7 | 5 | List bank balances per denom in the asset list | change 2 |
| 12.33.6 | 6 | Delete the receipt polling helpers from the surfaces | change 3 |
| 12.34.2 | 6 | Read the transaction ladder from the sequencer receipt before the explorer | change 1 |
| 12.34.8 | 6 | Remove the account deposit route and its link | change 2, retire 2 |
| 12.34.9 | 6 | Delete the custody deposit flow and the custody precompile helpers | retire 10 |

Acceptance criteria (23):

| criterion | test |
|---|---|
| 12274.1 | `apps/wallet/src/account/test/endpoint.test.ts::account_test_endpoint_answers_the_merged_reads (planned)` |
| 12275.1 | `apps/wallet/src/surfaces/test/chain.test.tsx::surfaces_test_chain_answers_the_sequencer_receipt_of_a_sent_transaction (planned)` |
| 12276.1 | `apps/wallet/src/app/api/health/route.test.ts::health_route_checks_the_gateway_healthz (planned)` |
| 12277.1 | `apps/wallet/src/surfaces/WebDataView.test.tsx::web_data_view_shows_caps_observed_at_the_committed_block (planned)` |
| 12278.1 | `apps/wallet/src/surfaces/BridgeView.test.tsx::bridge_view_shows_the_receipted_rung_and_events_from_the_sequencer_receipt (planned)` |
| 12279.1 | `apps/wallet/src/surfaces/FeeChoice.test.tsx::fee_choice_applies_the_preference_once_the_change_is_receipted (planned)` |
| 12280.1 | `apps/wallet/src/surfaces/LaunchpadView.test.tsx::launchpad_view_shows_the_created_market_from_the_receipt_logs (planned)` |
| 12281.1 | `apps/wallet/src/lib/swap/executor.test.ts::execute_swap_returns_after_the_receipt (planned)` |
| 12281.2 | `apps/wallet/src/lib/swap/executor.test.ts::execute_swap_refuses_a_reverted_swap (planned)` |
| 12282.1 | `apps/wallet/src/surfaces/useSurface.test.ts::use_surface_exports_no_receipt_polling (planned)` |
| 12283.1 | `apps/wallet/src/account/StatusLadder.test.tsx::status_ladder_labels_submitted_receipted_and_committed (planned)` |
| 12284.1 | `apps/wallet/src/account/StatusLadder.test.tsx::transaction_ladder_reaches_receipted_from_the_sequencer_before_the_explorer (planned)` |
| 12285.1 | `apps/wallet/src/account/KernelPlan.test.tsx::kernel_plan_binds_legs_with_the_pending_nonce_of_the_wallet_address (planned)` |
| 12286.1 | `apps/wallet/src/account/hooks.test.ts::account_hooks_report_the_chain_node_and_its_consensus_reason (planned)` |
| 12287.1 | `apps/wallet/src/account/AccountView.test.tsx::account_view_shows_the_chain_account_and_node_notice (planned)` |
| 12288.1 | `apps/wallet/src/account/HistoryView.test.tsx::history_view_pages_the_chain_history_without_a_side_label (planned)` |
| 12289.1 | `apps/wallet/src/account/AssetList.test.tsx::asset_list_shows_one_bank_balance_per_denom (planned)` |
| 12290.1 | `apps/wallet/src/app/account/page.test.tsx::account_page_offers_no_deposit_link (planned)` |
| 12291.1 | `apps/wallet/src/account/custody-retired.test.ts::account_module_has_no_custody_deposit (planned)` |
| 12292.1 | `apps/wallet/src/account/test/endpoint.test.ts::account_test_endpoint_refuses_lx_and_custody_methods (planned)` |
| 12293.1 | `apps/wallet/src/components/TransferSuccess.test.tsx::transfer_success_completes_only_at_the_committed_rung (planned)` |
| 12294.1 | `apps/wallet/e2e/send.spec.ts::send_shows_receipted_from_the_sequencer_receipt_and_keeps_it_on_reload (planned)` |
| 12295.1 | `apps/wallet/e2e/send.spec.ts::send_reaches_committed_only_with_a_matching_explorer_answer (planned)` |

### launchpad-sdk-sdk

Launchpad TypeScript SDK. Code root: `sdk/launchpad/packages/sdk`.

| kind | stable | change | new | retire | total |
|---|---|---|---|---|---|
| cli_command | 7 | 2 | 0 | 3 | 12 |
| config_key | 0 | 0 | 0 | 1 | 1 |
| ts_export | 721 | 17 | 2 | 33 | 773 |
| all | 728 | 19 | 2 | 37 | 786 |

Interfaces that change, are new or retire (58):

| interface | kind | status | signature | defined at |
|---|---|---|---|---|
| 4.cfg-sdk-v2-evidence-manifest-j30 | config_key | retire | `"v2EvidenceManifest": string` | `sdk/launchpad/packages/sdk/package.json:52` |
| 4.cli-sdk-build-phase4 | cli_command | retire | `pnpm run build:phase4 (node test/build-phase4.mjs)` | `sdk/launchpad/packages/sdk/package.json:27` |
| 4.cli-sdk-script-examples | cli_command | change | `python3 scripts/examples.py` | `sdk/launchpad/packages/sdk/scripts/examples.py` |
| 4.cli-sdk-script-generate | cli_command | change | `python3 scripts/generate.py` | `sdk/launchpad/packages/sdk/scripts/generate.py` |
| 4.cli-sdk-test-phase2 | cli_command | retire | `pnpm run test:phase2 (python3 test/run-phase2.py)` | `sdk/launchpad/packages/sdk/package.json:25` |
| 4.cli-sdk-test-phase4 | cli_command | retire | `pnpm run test:phase4 (python3 test/run-phase4.py)` | `sdk/launchpad/packages/sdk/package.json:26` |
| 4.ts-sdk-browser-stream-browser-stream-transport | ts_export | retire | `export function browserStreamTransport(client: SdkClient, authenticated = false): StreamTransport {` | `sdk/launchpad/packages/sdk/src/browser-stream.ts:6` |
| 4.ts-sdk-op-if-account-delete-j30 | ts_export | retire | `operationDefinitions.IF_ACCOUNT_DELETE: POST /v2/me/deletion-requests (api)` | `sdk/launchpad/packages/sdk/src/operations.gen.ts` |
| 4.ts-sdk-op-if-account-delete-status-j30 | ts_export | retire | `operationDefinitions.IF_ACCOUNT_DELETE_STATUS: GET /v2/me/deletion-requests/{jobId} (api)` | `sdk/launchpad/packages/sdk/src/operations.gen.ts` |
| 4.ts-sdk-op-if-account-export-j30 | ts_export | retire | `operationDefinitions.IF_ACCOUNT_EXPORT: POST /v2/me/exports (api)` | `sdk/launchpad/packages/sdk/src/operations.gen.ts` |
| 4.ts-sdk-op-if-account-export-status-j30 | ts_export | retire | `operationDefinitions.IF_ACCOUNT_EXPORT_STATUS: GET /v2/me/exports/{jobId} (api)` | `sdk/launchpad/packages/sdk/src/operations.gen.ts` |
| 4.ts-sdk-op-if-alert-create-j30 | ts_export | change | `operationDefinitions.IF_ALERT_CREATE: POST /v2/me/alerts (api)` | `sdk/launchpad/packages/sdk/src/operations.gen.ts` |
| 4.ts-sdk-op-if-alert-delete-j30 | ts_export | change | `operationDefinitions.IF_ALERT_DELETE: DELETE /v2/me/alerts/{alertId} (api)` | `sdk/launchpad/packages/sdk/src/operations.gen.ts:325` |
| 4.ts-sdk-op-if-automation-action-j30 | ts_export | retire | `operationDefinitions.IF_AUTOMATION_ACTION: POST /v2/me/automation-campaigns/{campaignId}/actions (ap` | `sdk/launchpad/packages/sdk/src/operations.gen.ts` |
| 4.ts-sdk-op-if-automation-create-j30 | ts_export | retire | `operationDefinitions.IF_AUTOMATION_CREATE: POST /v2/me/automation-campaigns (api)` | `sdk/launchpad/packages/sdk/src/operations.gen.ts` |
| 4.ts-sdk-op-if-automation-public-disclosure-j30 | ts_export | retire | `operationDefinitions.IF_AUTOMATION_PUBLIC_DISCLOSURE: GET /v2/automation-campaigns/{campaignId}/disc` | `sdk/launchpad/packages/sdk/src/operations.gen.ts` |
| 4.ts-sdk-op-if-automation-status-j30 | ts_export | retire | `operationDefinitions.IF_AUTOMATION_STATUS: GET /v2/me/automation-campaigns/{campaignId} (api)` | `sdk/launchpad/packages/sdk/src/operations.gen.ts` |
| 4.ts-sdk-op-if-cup-current-j30 | ts_export | change | `operationDefinitions.IF_CUP_CURRENT: GET /v2/cups/current (api)` | `sdk/launchpad/packages/sdk/src/operations.gen.ts` |
| 4.ts-sdk-op-if-cup-get-j30 | ts_export | change | `operationDefinitions.IF_CUP_GET: GET /v2/cups/{cupId} (api)` | `sdk/launchpad/packages/sdk/src/operations.gen.ts` |
| 4.ts-sdk-op-if-cup-results-j30 | ts_export | change | `operationDefinitions.IF_CUP_RESULTS: GET /v2/cups/{cupId}/results (api)` | `sdk/launchpad/packages/sdk/src/operations.gen.ts` |
| 4.ts-sdk-op-if-cup-standings-j30 | ts_export | change | `operationDefinitions.IF_CUP_STANDINGS: GET /v2/cups/{cupId}/standings (api)` | `sdk/launchpad/packages/sdk/src/operations.gen.ts` |
| 4.ts-sdk-op-if-deposit-intent-j30 | ts_export | retire | `operationDefinitions.IF_DEPOSIT_INTENT: POST /v2/me/funding/deposit-intents (api)` | `sdk/launchpad/packages/sdk/src/operations.gen.ts` |
| 4.ts-sdk-op-if-device-register-j30 | ts_export | retire | `operationDefinitions.IF_DEVICE_REGISTER: PUT /v2/me/devices/{deviceId} (api)` | `sdk/launchpad/packages/sdk/src/operations.gen.ts` |
| 4.ts-sdk-op-if-device-revoke-j30 | ts_export | retire | `operationDefinitions.IF_DEVICE_REVOKE: DELETE /v2/me/devices/{deviceId} (api)` | `sdk/launchpad/packages/sdk/src/operations.gen.ts` |
| 4.ts-sdk-op-if-dm-create-j30 | ts_export | retire | `operationDefinitions.IF_DM_CREATE: POST /v2/me/conversations (api)` | `sdk/launchpad/packages/sdk/src/operations.gen.ts` |
| (33 more) | | | | |

Covering tasks (8):

| task | wave | title | interfaces named (by status) |
|---|---|---|---|
| 4.48.1 | 0 | Add the served-route registry for SDK operations | new 1 |
| 4.48.5 | 0 | Retire the SDK phase scripts and the evidence manifest key | retire 4 |
| 4.48.2 | 1 | Refuse unrouted SDK operations before any network request | change 1, retire 32 |
| 4.49.1 | 1 | Repoint the SDK generator at the monorepo layout | change 1 |
| 4.49.2 | 1 | Repoint the SDK examples generator at the monorepo layout | change 1 |
| 4.48.3 | 2 | Refuse development-only SDK operations in production clients | change 15 |
| 4.48.4 | 3 | Correct the IF_REWARD_CLAIM path to the served rewards claim route | change 1, new 1 |
| 4.50.1 | 5 | Retire the v1 browser stream transport from the SDK | retire 1 |

Acceptance criteria (11):

| criterion | test |
|---|---|
| 4511.1 | `sdk/launchpad/packages/sdk/test/served-routes.test.mjs::routeStatusCoversEveryOperation (planned)` |
| 4512.1 | `sdk/launchpad/packages/sdk/test/unrouted-refusal.test.mjs::unroutedOperationRefusedBeforeNetwork (planned)` |
| 4512.2 | `sdk/launchpad/packages/sdk/test/unrouted-refusal.test.mjs::servedOperationStillReachesNetwork (planned)` |
| 4513.1 | `sdk/launchpad/packages/sdk/test/development-gating.test.mjs::developmentOperationRefusedInProduction (planned)` |
| 4513.2 | `sdk/launchpad/packages/sdk/test/development-gating.test.mjs::developmentOperationAllowedOutsideProduction (planned)` |
| 4514.1 | `sdk/launchpad/packages/sdk/test/route-corrections.test.mjs::rewardClaimUsesServedRoute (planned)` |
| 4515.1 | `sdk/launchpad/packages/sdk/test/package-scripts.test.mjs::packageScriptsReferenceExistingFiles (planned)` |
| 4517.1 | `sdk/launchpad/packages/sdk/test/test_generate_layout.py::test_repository_root_is_monorepo (planned)` |
| 4517.2 | `sdk/launchpad/packages/sdk/test/test_generate_layout.py::test_default_contracts_use_launchpad_spec_prefix (planned)` |
| 4518.1 | `sdk/launchpad/packages/sdk/test/test_examples_layout.py::test_examples_paths_use_monorepo_layout (planned)` |
| 4519.1 | `sdk/launchpad/packages/sdk/test/browser-stream-retirement.test.mjs::browserStreamV1TransportRetired (planned)` |

### launchpad-sdk-sdk-v2

Launchpad TypeScript SDK, v2 routes. Code root: `sdk/launchpad/packages/sdk-v2`.

| kind | stable | change | new | retire | total |
|---|---|---|---|---|---|
| cli_command | 1 | 0 | 0 | 0 | 1 |
| ts_export | 692 | 0 | 0 | 0 | 692 |
| all | 693 | 0 | 0 | 0 | 693 |

Covering tasks: none (every interface here is stable).

### launchpad-sdk-production-contracts-v2

Generated launchpad contract types, v2. Code root: `sdk/launchpad/packages/production-contracts-v2`.

| kind | stable | change | new | retire | total |
|---|---|---|---|---|---|
| cli_command | 1 | 0 | 0 | 0 | 1 |
| ts_export | 692 | 0 | 0 | 0 | 692 |
| all | 693 | 0 | 0 | 0 | 693 |

Covering tasks: none (every interface here is stable).

### wallet-sdk

Wallet SDK used by the wallet app and third-party pages. Code root: `sdk/wallet`.

| kind | stable | change | new | retire | total |
|---|---|---|---|---|---|
| config_key | 27 | 3 | 0 | 0 | 30 |
| error_code | 31 | 1 | 0 | 1 | 33 |
| event | 10 | 0 | 0 | 0 | 10 |
| http_route | 3 | 0 | 1 | 0 | 4 |
| jsonrpc_method | 35 | 7 | 3 | 10 | 55 |
| storage_key | 2 | 1 | 1 | 0 | 4 |
| ts_export | 354 | 80 | 23 | 10 | 467 |
| all | 462 | 92 | 28 | 21 | 603 |

Interfaces that change, are new or retire (141):

| interface | kind | status | signature | defined at |
|---|---|---|---|---|
| 12.cfg-sdk-lx-activity-options-confirm | config_key | change | `LxActivityOptions.confirm: (review: LxActivityReview) => boolean \| Promise<boolean>` | `sdk/wallet/src/index.ts:105` |
| 12.cfg-sdk-lx-activity-options-storage | config_key | change | `LxActivityOptions.storage: LxActivityStorage` | `sdk/wallet/src/index.ts:104` |
| 12.cfg-sdk-lx-activity-wallet-config-lx-activity | config_key | change | `LxActivityWalletConfig.lxActivity?: LxActivityOptions` | `sdk/wallet/src/index.ts:109` |
| 12.err-sdk-kernel-unavailable | error_code | change | `export const KERNEL_UNAVAILABLE_CODE = -32010;` | `sdk/wallet/src/kernel.ts:12` |
| 12.err-sdk-unauthorized-custody-not-supported | error_code | retire | `UnauthorizedReason 'custody_not_supported'` | `sdk/wallet/src/errors.ts:39` |
| 12.http-sdk-calls-post-v1-wallet-sponsored-quote-j50 | http_route | new | `outbound POST /v1/wallet/sponsored/quote issued by GasStationModule.quote` | `sdk/wallet/src/modules/gas-station.ts` (new, not at HEAD) |
| 12.key-sdk-evm-review-v1-j50 | storage_key | new | `paxeer:evm-review:v1:<principal>` | `sdk/wallet/src/index.ts` (new, not at HEAD) |
| 12.key-sdk-lx-activity-v1 | storage_key | change | `storage paxeer:lx-activity:v1:<encoded apiUrl>:<principal>:<review id> -> retained lx activity recor` | `sdk/wallet/src/index.ts:433` |
| 12.rpc-sdk-calls-eth-get-transaction-count-pending-j50 | jsonrpc_method | new | `outbound JSON-RPC call eth_getTransactionCount [address, 'pending'] issued by KernelClient.getSequen` | `sdk/wallet/src/kernel.ts` (new, not at HEAD) |
| 12.rpc-sdk-calls-lx-get-account | jsonrpc_method | retire | `outbound JSON-RPC call lx_getAccount issued by the SDK` | `sdk/wallet/src/kernel.ts:95` |
| 12.rpc-sdk-calls-lx-get-activity-status | jsonrpc_method | retire | `outbound JSON-RPC call lx_getActivityStatus issued by the SDK` | `sdk/wallet/src/kernel.ts:112` |
| 12.rpc-sdk-calls-lx-get-balances | jsonrpc_method | retire | `outbound JSON-RPC call lx_getBalances issued by the SDK` | `sdk/wallet/src/kernel.ts:100` |
| 12.rpc-sdk-calls-lx-get-receipt | jsonrpc_method | retire | `outbound JSON-RPC call lx_getReceipt issued by the SDK` | `sdk/wallet/src/kernel.ts:108` |
| 12.rpc-sdk-calls-lx-get-sequence | jsonrpc_method | retire | `outbound JSON-RPC call lx_getSequence issued by the SDK` | `sdk/wallet/src/kernel.ts:104` |
| 12.rpc-sdk-calls-pax-sequencer-receipt-j50 | jsonrpc_method | new | `outbound JSON-RPC call pax_sequencerReceipt [txHash] issued by the SDK` | `sdk/wallet/src/receipt.ts` (planned) |
| 12.rpc-sdk-calls-px-get-account | jsonrpc_method | change | `outbound JSON-RPC call px_getAccount issued by the SDK` | `sdk/wallet/src/endpoint.ts:55` |
| 12.rpc-sdk-calls-px-get-balances | jsonrpc_method | change | `outbound JSON-RPC call px_getBalances issued by the SDK` | `sdk/wallet/src/endpoint.ts:76` |
| 12.rpc-sdk-calls-px-get-network | jsonrpc_method | change | `outbound JSON-RPC call px_getNetwork issued by the SDK` | `sdk/wallet/src/endpoint.ts:71` |
| 12.rpc-sdk-calls-px-get-unified-history | jsonrpc_method | change | `outbound JSON-RPC call px_getUnifiedHistory issued by the SDK` | `sdk/wallet/src/endpoint.ts:99` |
| 12.rpc-sdk-calls-px-list-assets | jsonrpc_method | change | `outbound JSON-RPC call px_listAssets issued by the SDK` | `sdk/wallet/src/endpoint.ts:59` |
| 12.rpc-sdk-provider-eth-send-transaction | jsonrpc_method | change | `PaxeerProvider.request({ method: 'eth_sendTransaction' })` | `sdk/wallet/src/provider.ts:190` |
| 12.rpc-sdk-provider-paxeer-custody-status | jsonrpc_method | retire | `PaxeerProvider.request({ method: 'paxeer_custodyStatus' })` | `sdk/wallet/src/provider.ts:200` |
| 12.rpc-sdk-provider-paxeer-prepare-custody | jsonrpc_method | retire | `PaxeerProvider.request({ method: 'paxeer_prepareCustody' })` | `sdk/wallet/src/provider.ts:198` |
| 12.rpc-sdk-provider-paxeer-recover-custody | jsonrpc_method | retire | `PaxeerProvider.request({ method: 'paxeer_recoverCustody' })` | `sdk/wallet/src/provider.ts:204` |
| 12.rpc-sdk-provider-paxeer-restore-custody | jsonrpc_method | retire | `PaxeerProvider.request({ method: 'paxeer_restoreCustody' })` | `sdk/wallet/src/provider.ts:202` |
| (116 more) | | | | |

Covering tasks (31):

| task | wave | title | interfaces named (by status) |
|---|---|---|---|
| 12.21.1 | 0 | Add the merged-chain read and receipt types to the wallet SDK beside the joined ones | change 1, new 9 |
| 12.22.1 | 1 | Move the SDK status ladder to submitted, receipted and committed with no anchor table | change 5 |
| 12.23.1 | 1 | Read pax_sequencerReceipt from the node in a new SDK receipt module | new 3 |
| 12.24.1 | 1 | Read chain history without LayerX rows or the side label | change 3, new 1 |
| 12.25.1 | 1 | Type wallet caps observations by block height and the receipted and committed rungs | change 5 |
| 12.22.2 | 2 | Decode the explorer rung answer rung, block_number and committed_height | change 4 |
| 12.23.2 | 2 | Wait for a target sequencer rung with bounded attempts | new 1 |
| 12.23.3 | 2 | Return the sequencer receipt from PaxeerWallet.sendTransaction | change 1 |
| 12.23.4 | 2 | Add paxeer_sendTransactionReceipted to the SDK provider | change 1, new 1 |
| 12.24.2 | 2 | Read bank balances through px_getBalances in the endpoint client | change 1, new 1 |
| 12.22.3 | 3 | Raise an observed transfer to receipted from the sequencer receipt | change 3 |
| 12.23.5 | 3 | Add sendModuleTransactionReceipted to the SDK modules | new 1 |
| 12.24.3 | 3 | Read the chain account through px_getAccount in the endpoint client | change 1, new 1 |
| 12.25.2 | 3 | Move KernelClient account and balance reads off lx_getAccount and lx_getBalances | change 2, retire 2 |
| 12.29.1 | 3 | Carry the optional leg tx_hash in SDK intent bindings | change 1 |
| 12.22.4 | 4 | Add statusLadder.fromSequencerReceipt to the SDK ladder | new 1 |
| 12.24.4 | 4 | List bank assets through px_listAssets in the endpoint client | change 1, new 1 |
| 12.25.3 | 4 | Read the pending nonce and sequencer receipt in KernelClient and retire the lx reads | change 4, new 1, retire 3 |
| 12.26.1 | 4 | Quote sponsored gas through the gateway sponsored quote route | change 3, new 1 |
| 12.26.2 | 4 | Answer the sequencer rung from paxeer_sponsoredStatus in the provider | change 1 |
| 12.22.5 | 5 | Recover stored transfer observations written with the legacy rung names | change 1 |
| 12.24.5 | 5 | Decode the px_getNetwork consensus head in place of anchor and kernel | change 7 |
| 12.25.4 | 5 | Derive kernel availability from the px_getNetwork consensus head | change 5 |
| 12.26.3 | 5 | Carry the sequencer receipt in sponsored submission evidence | change 2 |
| 12.27.1 | 5 | Add EVM transaction review, approval and status to PaxeerWallet | new 6 |
| 12.21.3 | 6 | Delete the joined, anchor and custody types from the wallet SDK | change 12, retire 2 |
| 12.24.6 | 6 | Retire the joined endpoint reads and their decoders | change 9, retire 1 |
| 12.27.2 | 6 | Retire the lx_activity review, approval and signing from PaxeerWallet | change 19 |
| 12.28.1 | 6 | Retire the custody methods from the SDK provider | retire 11 |
| 12.28.2 | 6 | Retire WalletInterface.signCustody from the SDK wallet interface | retire 1 |
| 12.28.3 | 6 | Retire the custody_not_supported unauthorized reason | retire 1 |

Acceptance criteria (43):

| criterion | test |
|---|---|
| 12241.1 | `sdk/wallet/test/merged-types.test.ts::merged_types_type_every_merged_read_and_the_sequencer_receipt (planned)` |
| 12242.1 | `sdk/wallet/test/fixtures.test.ts::sdk_fixtures_carry_one_answer_per_sequencer_and_explorer_rung (planned)` |
| 12243.1 | `sdk/wallet/test/merged-types.test.ts::merged_types_export_no_joined_anchor_or_custody_type (planned)` |
| 12244.1 | `sdk/wallet/test/ladder.test.ts::ladder_rungs_are_submitted_receipted_committed_in_order (planned)` |
| 12244.2 | `sdk/wallet/test/ladder.test.ts::ladder_journey_states_map_onto_the_three_rungs_without_an_anchor_source (planned)` |
| 12245.1 | `sdk/wallet/test/ladder.test.ts::ladder_explorer_status_decodes_rung_block_number_and_committed_height (planned)` |
| 12245.2 | `sdk/wallet/test/ladder.test.ts::ladder_explorer_status_refuses_a_committed_answer_below_its_block (planned)` |
| 12246.1 | `sdk/wallet/test/ladder.test.ts::observe_transfer_reaches_receipted_from_the_sequencer_receipt (planned)` |
| 12246.2 | `sdk/wallet/test/ladder.test.ts::observe_transfer_reaches_committed_only_with_a_matching_explorer_block (planned)` |
| 12247.1 | `sdk/wallet/test/ladder.test.ts::status_ladder_from_sequencer_receipt_steps_up_to_the_node_rung (planned)` |
| 12248.1 | `sdk/wallet/test/ladder.test.ts::recover_transfer_observation_maps_legacy_rungs_onto_the_merged_ladder (planned)` |
| 12249.1 | `sdk/wallet/test/receipt.test.ts::read_sequencer_receipt_calls_pax_sequencer_receipt_and_decodes_each_rung (planned)` |
| 12249.2 | `sdk/wallet/test/receipt.test.ts::decode_sequencer_receipt_refuses_an_unknown_rung_or_a_foreign_hash (planned)` |
| 12250.1 | `sdk/wallet/test/receipt.test.ts::wait_for_sequencer_receipt_returns_once_the_requested_rung_is_reached (planned)` |
| 12250.2 | `sdk/wallet/test/receipt.test.ts::wait_for_sequencer_receipt_times_out_with_the_last_answer (planned)` |
| 12251.1 | `sdk/wallet/test/client.test.ts::paxeer_wallet_send_transaction_returns_the_sequencer_receipt (planned)` |
| 12251.2 | `sdk/wallet/test/client.test.ts::paxeer_wallet_send_transaction_refuses_a_receipt_for_another_hash (planned)` |
| 12252.1 | `sdk/wallet/test/provider.test.ts::provider_send_transaction_receipted_answers_the_receipted_sequencer_receipt (planned)` |
| 12252.2 | `sdk/wallet/test/provider.test.ts::provider_eth_send_transaction_still_answers_only_the_hash (planned)` |
| 12253.1 | `sdk/wallet/test/modules.test.ts::send_module_transaction_receipted_answers_the_receipt_with_logs (planned)` |
| 12253.2 | `sdk/wallet/test/modules.test.ts::send_module_transaction_receipted_falls_back_for_an_injected_provider (planned)` |
| 12254.1 | `sdk/wallet/test/history.test.ts::chain_history_keeps_paxeer_rows_and_drops_the_side_label (planned)` |
| 12255.1 | `sdk/wallet/test/balances.test.ts::endpoint_bank_balances_decode_one_row_per_denom (planned)` |
| 12256.1 | `sdk/wallet/test/account-reads.test.ts::endpoint_chain_account_decodes_the_paxeer_half_only (planned)` |
| 12257.1 | `sdk/wallet/test/assets.test.ts::endpoint_bank_assets_decode_the_bank_asset_page (planned)` |
| 12258.1 | `sdk/wallet/test/network.test.ts::endpoint_network_decodes_the_consensus_head_or_its_absence (planned)` |
| 12259.1 | `sdk/wallet/test/endpoint.test.ts::endpoint_client_exposes_no_joined_read (planned)` |
| 12260.1 | `sdk/wallet/test/caps.test.ts::wallet_caps_snapshot_is_observed_at_a_block_on_a_rung (planned)` |
| 12261.1 | `sdk/wallet/test/kernel-reads.test.ts::kernel_client_reads_account_and_balances_through_px_methods (planned)` |
| 12262.1 | `sdk/wallet/test/kernel-reads.test.ts::kernel_client_reads_the_pending_nonce_and_sequencer_receipt (planned)` |
| 12263.1 | `sdk/wallet/test/availability.test.ts::kernel_availability_follows_the_consensus_head (planned)` |
| 12264.1 | `sdk/wallet/test/gas-station.test.ts::gas_station_quote_posts_the_budget_to_the_sponsored_quote_route (planned)` |
| 12264.2 | `sdk/wallet/test/gas-station.test.ts::gas_station_quote_reports_an_exhausted_daily_budget (planned)` |
| 12265.1 | `sdk/wallet/test/provider.test.ts::provider_sponsored_status_answers_the_sequencer_rung (planned)` |
| 12266.1 | `sdk/wallet/test/gas-station.test.ts::gas_station_status_evidence_carries_the_matching_sequencer_receipt (planned)` |
| 12267.1 | `sdk/wallet/test/evm-review.test.ts::evm_review_answers_to_value_data_and_signing_hash_of_the_transaction (planned)` |
| 12267.2 | `sdk/wallet/test/evm-review.test.ts::evm_review_approves_and_reads_the_approval_status (planned)` |
| 12268.1 | `sdk/wallet/test/lx-retired.test.ts::paxeer_wallet_has_no_lx_activity_methods (planned)` |
| 12269.1 | `sdk/wallet/test/provider.test.ts::provider_refuses_the_retired_custody_methods (planned)` |
| 12270.1 | `sdk/wallet/test/custody-retired.test.ts::wallet_interface_has_no_sign_custody (planned)` |
| 12271.1 | `sdk/wallet/test/errors.test.ts::unauthorized_reasons_exclude_custody_not_supported (planned)` |
| 12272.1 | `sdk/wallet/test/human-binding.test.ts::intent_binding_carries_the_pending_nonce_and_optional_tx_hash (planned)` |
| 12273.1 | `sdk/wallet/test/human-binding.test.ts::journey_evidence_carries_an_optional_transaction_hash (planned)` |

### launchpad-sdk-production-contracts

Generated launchpad contract types. Code root: `sdk/launchpad/packages/production-contracts`.

| kind | stable | change | new | retire | total |
|---|---|---|---|---|---|
| cli_command | 3 | 0 | 0 | 0 | 3 |
| ts_export | 506 | 0 | 0 | 0 | 506 |
| all | 509 | 0 | 0 | 0 | 509 |

Covering tasks: none (every interface here is stable).

### launchpad-sdk-protocol-go

Go bindings and address catalogue of the launchpad protocol. Code root: `sdk/launchpad/protocol`.

| kind | stable | change | new | retire | total |
|---|---|---|---|---|---|
| cli_command | 0 | 1 | 0 | 0 | 1 |
| go_exported | 415 | 8 | 0 | 0 | 423 |
| all | 415 | 9 | 0 | 0 | 424 |

Interfaces that change, are new or retire (9):

| interface | kind | status | signature | defined at |
|---|---|---|---|---|
| 4.cli-protocol-abi-baseline-j30 | cli_command | change | `python3 bootstrap/abi_baseline.py --contracts <root> [--abi-root <dir>]` | `sdk/launchpad/protocol/bootstrap/abi_baseline.py:12` |
| 4.go-protocol-bindings-fee-accumulator-meta-data | go_exported | change | `var FeeAccumulatorMetaData = &bind.MetaData` | `sdk/launchpad/protocol/bindings/feeaccumulator.go:33` |
| 4.go-protocol-bindings-fees-router-meta-data | go_exported | change | `var FeesRouterMetaData = &bind.MetaData` | `sdk/launchpad/protocol/bindings/feesrouter.go:33` |
| 4.go-protocol-bindings-pool-registry-meta-data | go_exported | change | `var PoolRegistryMetaData = &bind.MetaData` | `sdk/launchpad/protocol/bindings/poolregistry.go:43` |
| 4.go-protocol-bindings-protocol-config-meta-data | go_exported | change | `var ProtocolConfigMetaData = &bind.MetaData` | `sdk/launchpad/protocol/bindings/protocolconfig.go:33` |
| 4.go-protocol-bindings-quoter-meta-data | go_exported | change | `var QuoterMetaData = &bind.MetaData` | `sdk/launchpad/protocol/bindings/quoter.go:52` |
| 4.go-protocol-bindings-router-meta-data | go_exported | change | `var RouterMetaData = &bind.MetaData` | `sdk/launchpad/protocol/bindings/router.go:33` |
| 4.go-protocol-bindings-sidiora-factory-meta-data | go_exported | change | `var SidioraFactoryMetaData = &bind.MetaData` | `sdk/launchpad/protocol/bindings/sidiorafactory.go:33` |
| 4.go-protocol-bindings-sidiora-nftmeta-data | go_exported | change | `var SidioraNFTMetaData = &bind.MetaData` | `sdk/launchpad/protocol/bindings/sidioranft.go:33` |

Covering tasks (9):

| task | wave | title | interfaces named (by status) |
|---|---|---|---|
| 4.52.1 | 0 | Regenerate the FeeAccumulator Go binding from the precompile role ABI | change 1 |
| 4.52.2 | 0 | Regenerate the FeesRouter Go binding from the precompile role ABI | change 1 |
| 4.52.3 | 0 | Regenerate the PoolRegistry Go binding from the precompile role ABI | change 1 |
| 4.52.4 | 0 | Regenerate the ProtocolConfig Go binding from the precompile role ABI | change 1 |
| 4.52.5 | 0 | Regenerate the Quoter Go binding from the precompile role ABI | change 1 |
| 4.52.6 | 0 | Regenerate the Router Go binding from the precompile role ABI | change 1 |
| 4.52.7 | 0 | Regenerate the SidioraFactory Go binding from the precompile role ABI | change 1 |
| 4.52.8 | 0 | Regenerate the SidioraNFT Go binding from the precompile role ABI | change 1 |
| 4.53.2 | 1 | Compare protocol bindings against the precompile role ABIs | change 1 |

Acceptance criteria (16):

| criterion | test |
|---|---|
| 4523.1 | `sdk/launchpad/protocol/bindings/feeaccumulator_abi_test.go::TestFeeAccumulatorBindingMatchesRoleABI (planned)` |
| 4524.1 | `sdk/launchpad/protocol/bindings/feesrouter_abi_test.go::TestFeesRouterBindingMatchesRoleABI (planned)` |
| 4525.1 | `sdk/launchpad/protocol/bindings/poolregistry_abi_test.go::TestPoolRegistryBindingMatchesRoleABI (planned)` |
| 4526.1 | `sdk/launchpad/protocol/bindings/protocolconfig_abi_test.go::TestProtocolConfigBindingMatchesRoleABI (planned)` |
| 4527.1 | `sdk/launchpad/protocol/bindings/quoter_abi_test.go::TestQuoterBindingMatchesRoleABI (planned)` |
| 4528.1 | `sdk/launchpad/protocol/bindings/router_abi_test.go::TestRouterBindingMatchesRoleABI (planned)` |
| 4529.1 | `sdk/launchpad/protocol/bindings/sidiorafactory_abi_test.go::TestSidioraFactoryBindingMatchesRoleABI (planned)` |
| 4530.1 | `sdk/launchpad/protocol/bindings/sidioranft_abi_test.go::TestSidioraNFTBindingMatchesRoleABI (planned)` |
| 4531.1 | `sdk/launchpad/protocol/role_events_test.go::TestRoleEventsRegistered (planned)` |
| 4532.1 | `sdk/launchpad/protocol/bootstrap/test_abi_baseline.py::test_role_binding_compared_with_precompile_abi (planned)` |
| 4532.2 | `sdk/launchpad/protocol/bootstrap/test_abi_baseline.py::test_non_role_binding_uses_artifact (planned)` |
| 4533.1 | `sdk/launchpad/protocol/catalog/registry_input_test.go::TestOpticalRegistryReviewUsesRealCompiledArtifactAndExplicitAttestation (existing)` |
| 4534.1 | `sdk/launchpad/protocol/generator-inputs/test_export.py::test_real_frozen_artifact_determinism_and_drift (existing)` |
| 4534.2 | `sdk/launchpad/protocol/generator-inputs/test_export.py::test_selector_inputs_and_identity_validation (existing)` |
| 4534.3 | `sdk/launchpad/protocol/generator-inputs/test_export.py::test_versioned_inputs_are_deterministic_and_require_authentic_history (existing)` |
| 4534.4 | `sdk/launchpad/protocol/generator-inputs/test_export.py::test_real_optical_registry_review_preserves_compiler_and_source_identity (existing)` |

### launchpad-web-app

Launchpad web app. Code root: `apps/launchpad/web`.

| kind | stable | change | new | retire | total |
|---|---|---|---|---|---|
| cli_command | 11 | 0 | 0 | 2 | 13 |
| config_key | 5 | 5 | 0 | 0 | 10 |
| env_var | 81 | 8 | 2 | 1 | 92 |
| file_format | 0 | 3 | 0 | 1 | 4 |
| http_route | 3 | 0 | 0 | 0 | 3 |
| ts_export | 11 | 5 | 2 | 1 | 19 |
| all | 111 | 21 | 4 | 5 | 141 |

Interfaces that change, are new or retire (30):

| interface | kind | status | signature | defined at |
|---|---|---|---|---|
| 4.cfg-web-community-lane-include-j30 | config_key | change | `test.include: string[]` | `apps/launchpad/web/src/widgets/community-interactions/vitest.config.mts` |
| 4.cfg-web-next-allowed-dev-origins-j30 | config_key | change | `allowedDevOrigins: string[]` | `apps/launchpad/web/next.config.ts:113` |
| 4.cfg-web-next-rewrites-j30 | config_key | change | `rewrites: () => Promise<{beforeFiles, afterFiles, fallback}>` | `apps/launchpad/web/next.config.ts:108` |
| 4.cfg-web-next-transpile-packages-j30 | config_key | change | `transpilePackages: string[]` | `apps/launchpad/web/next.config.ts:166` |
| 4.cfg-web-vitest-test-env-j30 | config_key | change | `test.env: Record<string,string>` | `apps/launchpad/web/vitest.config.ts` |
| 4.cli-launchpad-build-bot-j30 | cli_command | retire | `pnpm --dir apps/launchpad run build:bot` | `apps/launchpad/package.json:8` |
| 4.cli-launchpad-dev-bot-j30 | cli_command | retire | `pnpm --dir apps/launchpad run dev:bot` | `apps/launchpad/package.json:6` |
| 4.env-web-anvil-bin-j30 | env_var | new | `process.env.ANVIL_BIN` | `apps/launchpad/web/src/core/test-anvil.ts` (planned) |
| 4.env-web-l25-vitest-cache-dir-j30 | env_var | change | `process.env.L25_VITEST_CACHE_DIR` | `apps/launchpad/web/src/core/wallet-sdk/vitest.l25.config.ts:6` |
| 4.env-web-l26-cache-dir-j30 | env_var | change | `process.env.L26_CACHE_DIR` | `apps/launchpad/web/src/widgets/trading/transaction-controls/vitest.phase2.config.ts:4` |
| 4.env-web-l28-private-cache-directory-j30 | env_var | change | `process.env.L28_PRIVATE_CACHE_DIRECTORY` | `apps/launchpad/web/src/widgets/meta-ag/vitest.l28.config.ts:5` |
| 4.env-web-next-dev-allowed-origins-j30 | env_var | new | `process.env.NEXT_DEV_ALLOWED_ORIGINS` | `apps/launchpad/web/src/core/next-build-config.ts` (planned) |
| 4.env-web-next-public-private-qualification-authority-profile-j30 | env_var | change | `process.env.NEXT_PUBLIC_PRIVATE_QUALIFICATION_AUTHORITY_PROFILE` | `apps/launchpad/web/src/app/layout.tsx:14` |
| 4.env-web-next-public-user-api-url-j30 | env_var | retire | `process.env.NEXT_PUBLIC_USER_API_URL` | `apps/launchpad/web/src/widgets/home/home-panel-state.test.tsx:51` |
| 4.env-web-web-api-upstream-j30 | env_var | change | `process.env.WEB_API_UPSTREAM` | `apps/launchpad/web/next.config.ts:74` |
| 4.env-web-web-charts-upstream-j30 | env_var | change | `process.env.WEB_CHARTS_UPSTREAM` | `apps/launchpad/web/next.config.ts:78` |
| 4.env-web-web-gateway-upstream-j30 | env_var | change | `process.env.WEB_GATEWAY_UPSTREAM` | `apps/launchpad/web/next.config.ts:74` |
| 4.env-web-web-sockets-upstream-j30 | env_var | change | `process.env.WEB_SOCKETS_UPSTREAM` | `apps/launchpad/web/next.config.ts:77` |
| 4.fmt-web-abi-feesrouter-j30 | file_format | change | `FeesRouter.json ABI JSON array` | `apps/launchpad/web/src/core/network/abis/FeesRouter.json` |
| 4.fmt-web-abi-quoter-j30 | file_format | change | `Quoter.json ABI JSON array` | `apps/launchpad/web/src/core/network/abis/Quoter.json` |
| 4.fmt-web-abi-router-j30 | file_format | change | `Router.json ABI JSON array` | `apps/launchpad/web/src/core/network/abis/Router.json` |
| 4.fmt-web-ecosystem-config-j30 | file_format | retire | `pm2 ecosystem module exporting apps[{name:'my-next-app', cwd}]` | `apps/launchpad/web/ecosystem.config.mjs:2` |
| 4.ts-web-chain-trade-provider-j30 | ts_export | change | `export class ChainTradeProvider` | `apps/launchpad/web/src/widgets/trading/transaction-controls/providers.ts:51` |
| 4.ts-web-core-deployment-j30 | ts_export | change | `export const CORE_DEPLOYMENT` | `apps/launchpad/web/src/core/network/deployment.ts:3` |
| 4.ts-web-next-build-config-transpile-packages-j30 | ts_export | new | `export const TRANSPILE_PACKAGES: readonly string[]; export function allowedDevOrigins(env); export a` | `apps/launchpad/web/src/core/next-build-config.ts` (planned) |
| (5 more) | | | | |

Covering tasks (18):

| task | wave | title | interfaces named (by status) |
|---|---|---|---|
| 4.41.1 | 0 | Drop undeclared transpilePackages entries from the web build | change 1, new 1 |
| 4.41.4 | 0 | Retire the telegram-bot scripts and the pm2 ecosystem template | retire 3 |
| 4.42.1 | 0 | Make the web vitest environment hermetic with loopback backend origins | change 1 |
| 4.42.2 | 0 | Point the home widget suites at NEXT_PUBLIC_GATEWAY_URL | retire 1 |
| 4.42.3 | 0 | Resolve the anvil binary from ANVIL_BIN or PATH in the web chain suites | new 2 |
| 4.42.4 | 0 | Resolve the anvil binary from ANVIL_BIN or PATH in the creation chain runner | new 1 |
| 4.42.5 | 0 | Keep lane vitest cache directories inside the web workspace | change 3 |
| 4.42.6 | 0 | Fix the community-interactions lane include so it matches its tests | change 1 |
| 4.43.1 | 0 | Sync the web Router ABI with the launchpad precompile role ABI | change 1 |
| 4.43.2 | 0 | Sync the web Quoter ABI with the launchpad precompile role ABI | change 1 |
| 4.43.3 | 0 | Sync the web FeesRouter ABI with the launchpad precompile role ABI | change 1 |
| 4.44.2 | 0 | Retire the deprecated mediaApiUrl alias | retire 1 |
| 4.41.2 | 1 | Read Next allowedDevOrigins from NEXT_DEV_ALLOWED_ORIGINS instead of a literal host | change 1, new 1 |
| 4.44.1 | 1 | Refuse the private fixture authority profile in production builds | change 2 |
| 4.45.2 | 1 | Emit role code identities from the deployment manifest into the web helper | change 1 |
| 4.41.3 | 2 | Require explicit backend upstreams for production rewrites | change 5 |
| 4.43.4 | 5 | Pin Router, Quoter and PoolRegistry to the launchpad surface runtime | change 2 |
| 4.43.5 | 6 | Decode UnexecutableQuote into a typed optical rejection | change 1 |

Acceptance criteria (26):

| criterion | test |
|---|---|
| 4481.1 | `apps/launchpad/web/src/core/next-build-config.test.ts::transpilePackagesAreDeclaredWorkspaceDependencies (planned)` |
| 4482.1 | `apps/launchpad/web/src/core/next-build-config.test.ts::allowedDevOriginsDefaultToLoopback (planned)` |
| 4482.2 | `apps/launchpad/web/src/core/next-build-config.test.ts::allowedDevOriginsRejectSchemeOrPath (planned)` |
| 4483.1 | `apps/launchpad/web/src/core/next-build-config.test.ts::backendRewritesRefuseMissingProductionUpstream (planned)` |
| 4483.2 | `apps/launchpad/web/src/core/next-build-config.test.ts::backendRewritesEmptyInDevelopment (planned)` |
| 4483.3 | `apps/launchpad/web/src/core/next-build-config.test.ts::backendRewritesKeepRouteTemplates (planned)` |
| 4484.1 | `apps/launchpad/web/src/core/workspace-scripts.test.ts::workspaceScriptFiltersResolve (planned)` |
| 4485.1 | `apps/launchpad/web/src/core/vitest-env.test.ts::vitestEnvOriginsAreLoopback (planned)` |
| 4486.1 | `apps/launchpad/web/src/widgets/home/home-panel-state.test.tsx::homePanelProfilesUseGatewayOrigin (planned)` |
| 4486.2 | `apps/launchpad/web/src/widgets/home/home-empty-states.test.tsx::homeEmptyStatesProfilesUseGatewayOrigin (planned)` |
| 4487.1 | `apps/launchpad/web/src/core/test-anvil.test.ts::resolveAnvilBinaryPrefersAnvilBin (planned)` |
| 4487.2 | `apps/launchpad/web/src/core/test-anvil.test.ts::resolveAnvilBinaryFailsWithoutCandidate (planned)` |
| 4488.1 | `apps/launchpad/web/src/widgets/create/test_run_creation_chain.py::test_anvil_binary_prefers_env (planned)` |
| 4488.2 | `apps/launchpad/web/src/widgets/create/test_run_creation_chain.py::test_anvil_binary_fails_without_candidate (planned)` |
| 4489.1 | `apps/launchpad/web/src/core/lane-cache-dirs.test.ts::laneCacheDirsStayInWorkspace (planned)` |
| 4490.1 | `apps/launchpad/web/src/core/community-lane-config.test.ts::communityLaneIncludeMatchesSuites (planned)` |
| 4491.1 | `apps/launchpad/web/src/core/network/abis/router-abi.test.ts::webRouterAbiMatchesPrecompileRole (planned)` |
| 4492.1 | `apps/launchpad/web/src/core/network/abis/quoter-abi.test.ts::webQuoterAbiMatchesPrecompileRole (planned)` |
| 4493.1 | `apps/launchpad/web/src/core/network/abis/feesrouter-abi.test.ts::webFeesRouterAbiMatchesPrecompileRole (planned)` |
| 4494.1 | `apps/launchpad/web/src/widgets/trading/transaction-controls/surface-authority.test.ts::chainTradeProviderAcceptsSurfaceRuntime (planned)` |
| 4494.2 | `apps/launchpad/web/src/widgets/trading/transaction-controls/surface-authority.test.ts::chainTradeProviderRejectsProxyCodeAtSurfaceRole (planned)` |
| 4495.1 | `apps/launchpad/web/src/widgets/trading/transaction-controls/optical-errors.test.ts::unexecutableQuoteMapsToOpticalRejection (planned)` |
| 4496.1 | `apps/launchpad/web/src/core/runtime-authority-production.test.ts::privateFixtureProfileRefusedInProduction (planned)` |
| 4496.2 | `apps/launchpad/web/src/core/runtime-authority-production.test.ts::privateFixtureProfileParsedOutsideProduction (planned)` |
| 4497.1 | `apps/launchpad/web/src/core/sdk-config-exports.test.ts::sdkConfigExportsNoMediaApiUrl (planned)` |
| 4516.1 | `apps/launchpad/web/src/core/market/development-refusal.test.ts::developmentOperationsRenderUnsupportedInProduction (planned)` |

### wallet-demo

Embedded wallet demo page. Code root: `services/wallet/demo`.

| kind | stable | change | new | retire | total |
|---|---|---|---|---|---|
| cli_command | 5 | 0 | 0 | 0 | 5 |
| env_var | 5 | 0 | 0 | 0 | 5 |
| http_route | 2 | 0 | 0 | 0 | 2 |
| storage_key | 0 | 0 | 0 | 1 | 1 |
| ts_export | 15 | 1 | 0 | 0 | 16 |
| all | 27 | 1 | 0 | 1 | 29 |

Interfaces that change, are new or retire (2):

| interface | kind | status | signature | defined at |
|---|---|---|---|---|
| 12.key-demo-retained-custody | storage_key | retire | `localStorage paxeer-demo:custody:125:<account lowercase> -> retained layerxCustody consent bytes (he` | `services/wallet/demo/src/components/FundedAccountPanel.tsx:54` |
| 12.ts-demo-supported-wallet-panel | ts_export | change | `export function SupportedWalletPanel({ paxeer, origin, injected }:` | `services/wallet/demo/src/components/FundedAccountPanel.tsx:23` |

Covering tasks (1):

| task | wave | title | interfaces named (by status) |
|---|---|---|---|
| 12.0.4 | 5 | Demo supported-wallet panel drops the custody handoff | change 1, retire 1 |

Acceptance criteria (3):

| criterion | test |
|---|---|
| 12035.1 | `services/wallet/demo/src/components/FundedAccountPanel.test.tsx::supportedWalletPanelRendersNoCustodyHandoff (planned)` |
| 12035.2 | `services/wallet/demo/src/components/FundedAccountPanel.test.tsx::supportedWalletPanelClearsRetainedCustodyKey (planned)` |
| 12999.1 | `services/wallet/demo/src/components/Completion.test.tsx::walletCompletionRetiredSurfacesAbsent (planned)` |

### launchpad-sdk-candles-sdk

Candles SDK. Code root: `sdk/launchpad/packages/candles-sdk`.

| kind | stable | change | new | retire | total |
|---|---|---|---|---|---|
| cli_command | 3 | 0 | 0 | 0 | 3 |
| ts_export | 21 | 0 | 0 | 0 | 21 |
| all | 24 | 0 | 0 | 0 | 24 |

Covering tasks: none (every interface here is stable).

### launchpad-sdk-users-sdk

Users SDK. Code root: `sdk/launchpad/packages/users-sdk`.

| kind | stable | change | new | retire | total |
|---|---|---|---|---|---|
| cli_command | 3 | 0 | 0 | 0 | 3 |
| ts_export | 18 | 0 | 0 | 0 | 18 |
| all | 21 | 0 | 0 | 0 | 21 |

Covering tasks: none (every interface here is stable).

### launchpad-mobile-app

Launchpad mobile app. Code root: `apps/launchpad/mobile`.

| kind | stable | change | new | retire | total |
|---|---|---|---|---|---|
| cli_command | 3 | 0 | 1 | 0 | 4 |
| config_key | 1 | 0 | 0 | 0 | 1 |
| ts_export | 1 | 4 | 7 | 0 | 12 |
| all | 5 | 4 | 8 | 0 | 17 |

Interfaces that change, are new or retire (12):

| interface | kind | status | signature | defined at |
|---|---|---|---|---|
| 4.cli-mobile-test-vitest-j30 | cli_command | new | `pnpm --dir apps/launchpad/mobile run test:vitest` | `apps/launchpad/mobile/vitest.config.ts` (planned) |
| 4.ts-mobile-create-native-quote-transaction-builder-j30 | ts_export | change | `export function createNativeQuoteTransactionBuilder(...)` | `apps/launchpad/mobile/src/runtime/quote-transaction.ts:27` |
| 4.ts-mobile-market-session-j30 | ts_export | change | `export class MarketSession` | `apps/launchpad/mobile/src/runtime/market-session.ts:4` |
| 4.ts-mobile-mobile-services-j30 | ts_export | change | `export class MobileServices` | `apps/launchpad/mobile/src/runtime/service-client.ts:20` |
| 4.ts-mobile-native-approval-controller-j30 | ts_export | change | `export class NativeApprovalController` | `apps/launchpad/mobile/src/runtime/quote-transaction.ts:44` |
| 4.ts-mobile-native-authority-j30 | ts_export | new | `export async function verifyRolePins(reader, pins, block): Promise<void>` | `apps/launchpad/mobile/src/runtime/native-authority.ts` (planned) |
| 4.ts-mobile-onchain-quote-j30 | ts_export | new | `export async function onchainQuote(client, quoter, request, signal): Promise<Quote>` | `apps/launchpad/mobile/src/runtime/onchain-quote.ts` (planned) |
| 4.ts-mobile-routes-account-j30 | ts_export | new | `export const accountRoutes: route functions taking (sdk: SdkClient, ..., signal)` | `apps/launchpad/mobile/src/runtime/routes/account.ts` (planned) |
| 4.ts-mobile-routes-activity-j30 | ts_export | new | `export const activityRoutes: route functions taking (sdk: SdkClient, ..., signal)` | `apps/launchpad/mobile/src/runtime/routes/activity.ts` (planned) |
| 4.ts-mobile-routes-chat-j30 | ts_export | new | `export const chatRoutes: route functions taking (sdk: SdkClient, ..., signal)` | `apps/launchpad/mobile/src/runtime/routes/chat.ts` (planned) |
| 4.ts-mobile-routes-market-j30 | ts_export | new | `export const marketRoutes: route functions taking (sdk: SdkClient, ..., signal)` | `apps/launchpad/mobile/src/runtime/routes/market.ts` (planned) |
| 4.ts-mobile-routes-notifications-j30 | ts_export | new | `export const notificationRoutes: route functions taking (sdk: SdkClient, ..., signal)` | `apps/launchpad/mobile/src/runtime/routes/notifications.ts` (planned) |

Covering tasks (11):

| task | wave | title | interfaces named (by status) |
|---|---|---|---|
| 4.46.1 | 0 | Add a vitest harness and tsconfig to the mobile app | new 1 |
| 4.46.2 | 1 | Extract mobile service reads into per-concern route modules | change 1, new 5 |
| 4.46.3 | 5 | Repoint mobile market reads to the served v2 pool and search routes | new 1 |
| 4.46.4 | 5 | Repoint the mobile portfolio read to the served v2 portfolio route | new 1 |
| 4.46.5 | 5 | Repoint mobile notifications to the discovery notification routes | new 1 |
| 4.46.6 | 5 | Repoint mobile chat to the served v2 conversation and room message routes | new 1 |
| 4.46.7 | 5 | Repoint mobile stream list and receipt reads to served v2 routes | new 1 |
| 4.46.8 | 5 | Move the mobile market session to the v2 stream transport | change 1 |
| 4.47.1 | 5 | Pin mobile native trading to the launchpad surface runtime | change 2, new 1 |
| 4.47.2 | 5 | Quote mobile trades on chain through Quoter.quoteExactInput | new 1 |
| 4.47.3 | 6 | Route the mobile quote through the on-chain Quoter and retire /v1/quotes | change 1, new 1 |

Acceptance criteria (15):

| criterion | test |
|---|---|
| 4500.1 | `apps/launchpad/mobile/test/vitest/harness.test.ts::mobileHarnessLoadsSdkSource (planned)` |
| 4501.1 | `apps/launchpad/mobile/test/vitest/routes-scaffold.test.ts::routeModulesPreserveHeadPaths (planned)` |
| 4502.1 | `apps/launchpad/mobile/test/vitest/routes-market.test.ts::marketReadsUseV2PoolRoutes (planned)` |
| 4503.1 | `apps/launchpad/mobile/test/vitest/routes-account.test.ts::portfolioReadUsesV2Route (planned)` |
| 4504.1 | `apps/launchpad/mobile/test/vitest/routes-notifications.test.ts::notificationsUseDiscoveryRoutes (planned)` |
| 4505.1 | `apps/launchpad/mobile/test/vitest/routes-chat.test.ts::chatReadsUseServedRoutes (planned)` |
| 4505.2 | `apps/launchpad/mobile/test/vitest/routes-chat.test.ts::chatSendUsesRoomMessagesRoute (planned)` |
| 4506.1 | `apps/launchpad/mobile/test/vitest/routes-activity.test.ts::streamListUsesV2Route (planned)` |
| 4506.2 | `apps/launchpad/mobile/test/vitest/routes-activity.test.ts::receiptUsesV2TransactionStatus (planned)` |
| 4507.1 | `apps/launchpad/mobile/test/vitest/market-session.test.ts::marketSessionUsesV2Snapshots (planned)` |
| 4508.1 | `apps/launchpad/mobile/test/vitest/native-authority.test.ts::surfacePinAcceptsSurfaceRuntime (planned)` |
| 4508.2 | `apps/launchpad/mobile/test/vitest/native-authority.test.ts::surfacePinRejectsProxyRuntime (planned)` |
| 4509.1 | `apps/launchpad/mobile/test/vitest/onchain-quote.test.ts::onchainQuoteReadsQuoterAtPinnedBlock (planned)` |
| 4509.2 | `apps/launchpad/mobile/test/vitest/onchain-quote.test.ts::onchainQuoteRejectsInvalidSlippage (planned)` |
| 4510.1 | `apps/launchpad/mobile/test/vitest/routes-activity-quote.test.ts::mobileQuoteIssuesNoHttpRequest (planned)` |

### launchpad-sdk-metadata-sdk

Metadata SDK. Code root: `sdk/launchpad/packages/metadata-sdk`.

| kind | stable | change | new | retire | total |
|---|---|---|---|---|---|
| cli_command | 3 | 0 | 0 | 0 | 3 |
| ts_export | 12 | 0 | 0 | 0 | 12 |
| all | 15 | 0 | 0 | 0 | 15 |

Covering tasks: none (every interface here is stable).

### launchpad-sdk-ranking-algo-sdk

Ranking SDK. Code root: `sdk/launchpad/packages/ranking-algo-sdk`.

| kind | stable | change | new | retire | total |
|---|---|---|---|---|---|
| cli_command | 3 | 0 | 0 | 0 | 3 |
| ts_export | 11 | 0 | 0 | 0 | 11 |
| all | 14 | 0 | 0 | 0 | 14 |

Covering tasks: none (every interface here is stable).

### launchpad-sdk-sidiora-launchpad-v3

Launchpad v3 contract package. Code root: `sdk/launchpad/packages/sidiora-launchpad-v3`.

| kind | stable | change | new | retire | total |
|---|---|---|---|---|---|
| cli_command | 3 | 0 | 0 | 0 | 3 |
| ts_export | 10 | 1 | 0 | 0 | 11 |
| all | 13 | 1 | 0 | 0 | 14 |

Interfaces that change, are new or retire (1):

| interface | kind | status | signature | defined at |
|---|---|---|---|---|
| 4.ts-sidiora-launchpad-v3-abis-erc20-abi | ts_export | change | `export const erc20Abi = [` | `sdk/launchpad/packages/sidiora-launchpad-v3/src/abis.ts:3089` |

Covering tasks (1):

| task | wave | title | interfaces named (by status) |
|---|---|---|---|
| 4.51.1 | 0 | Sync the sidiora-launchpad-v3 erc20Abi with the SidioraERC20 role ABI | change 1 |

### launchpad-sdk-stats-sdk

Stats SDK. Code root: `sdk/launchpad/packages/stats-sdk`.

| kind | stable | change | new | retire | total |
|---|---|---|---|---|---|
| cli_command | 3 | 0 | 0 | 0 | 3 |
| ts_export | 10 | 0 | 0 | 0 | 10 |
| all | 13 | 0 | 0 | 0 | 13 |

Covering tasks: none (every interface here is stable).

### launchpad-sdk-indexer-sdk

Indexer SDK. Code root: `sdk/launchpad/packages/indexer-sdk`.

| kind | stable | change | new | retire | total |
|---|---|---|---|---|---|
| cli_command | 3 | 0 | 0 | 0 | 3 |
| ts_export | 7 | 0 | 0 | 0 | 7 |
| all | 10 | 0 | 0 | 0 | 10 |

Covering tasks: none (every interface here is stable).

### explorer-smart-contract-verifier-metrics

Metrics of the contract verifier used by explorer clients. Code root: `services/explorer/services/smart-contract-verifier`.

| kind | stable | change | new | retire | total |
|---|---|---|---|---|---|
| metric | 7 | 0 | 0 | 0 | 7 |
| all | 7 | 0 | 0 | 0 | 7 |

Covering tasks: none (every interface here is stable).

### explorer-recaptcha-verifier-errors

Error codes of the explorer recaptcha verifier. Code root: `services/explorer/services/libs`.

| kind | stable | change | new | retire | total |
|---|---|---|---|---|---|
| error_code | 6 | 0 | 0 | 0 | 6 |
| all | 6 | 0 | 0 | 0 | 6 |

Covering tasks: none (every interface here is stable).

### explorer-service-launcher-metrics

Metrics of the explorer service launcher library. Code root: `services/explorer/services/libs`.

| kind | stable | change | new | retire | total |
|---|---|---|---|---|---|
| metric | 2 | 0 | 0 | 0 | 2 |
| all | 2 | 0 | 0 | 0 | 2 |

Covering tasks: none (every interface here is stable).

### explorer-smart-contract-verifier-server-errors

Server error codes of the contract verifier. Code root: `services/explorer/services/smart-contract-verifier`.

| kind | stable | change | new | retire | total |
|---|---|---|---|---|---|
| error_code | 2 | 0 | 0 | 0 | 2 |
| all | 2 | 0 | 0 | 0 | 2 |

Covering tasks: none (every interface here is stable).

### launchpad-sdk-sidiora-v3

Launchpad v3 contract package, the single changed entry. Code root: `sdk/launchpad/packages/sidiora-launchpad-v3`.

| kind | stable | change | new | retire | total |
|---|---|---|---|---|---|
| file_format | 0 | 1 | 0 | 0 | 1 |
| all | 0 | 1 | 0 | 0 | 1 |

Interfaces that change, are new or retire (1):

| interface | kind | status | signature | defined at |
|---|---|---|---|---|
| 4.fmt-sidiora-launchpad-v3-provenance-j30 | file_format | change | `provenance.json {abi_sources:[{artifact, artifact_sha256, ...}]}` | `sdk/launchpad/packages/sidiora-launchpad-v3/provenance.json:2` |

Covering tasks (1):

| task | wave | title | interfaces named (by status) |
|---|---|---|---|
| 4.51.2 | 0 | Repoint sidiora-launchpad-v3 provenance at repository artifacts | change 1 |

Acceptance criteria (3):

| criterion | test |
|---|---|
| 4520.1 | `sdk/launchpad/packages/sidiora-launchpad-v3/test/sdk.test.mjs::erc20AbiMatchesSidioraErc20 (planned)` |
| 4521.1 | `sdk/launchpad/packages/sidiora-launchpad-v3/test/provenance.test.mjs::provenanceArtifactsResolveInRepository (planned)` |
| 4522.1 | `sdk/launchpad/packages/sidiora-launchpad-v3/test/abi-conformance.test.mjs::sidioraAbisMatchPrecompileRoleAbis (planned)` |

### explorer-smart-contract-verifier-server-metrics

Server metrics of the contract verifier. Code root: `services/explorer/services/smart-contract-verifier`.

| kind | stable | change | new | retire | total |
|---|---|---|---|---|---|
| metric | 1 | 0 | 0 | 0 | 1 |
| all | 1 | 0 | 0 | 0 | 1 |

Covering tasks: none (every interface here is stable).

## Open items

- `px_requestFunds` is named by the faucet service block but is not served by any gateway file at HEAD; the L19 replacement is the faucet claim path.
- The stale switch-off table cited pre-move paths; every server and caller line above is recomputed at HEAD.


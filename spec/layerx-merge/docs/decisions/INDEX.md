# Decision index

Programme carry-forward index for decisions 1–94 and 1701–1720. Each decision has one row, in numeric order. Carry statuses come from [the programme index](../../programme/data/decisions.kvx): `kept`, `rewritten` or `superseded`. New venue and programme decisions are indexed as `kept`; their source blocks use `carry = "new"`.

Titles, affected surfaces and resolution counts follow [the installed specification](../../spec.kvx). Each linked record contains the decision value, basis and bindings.

| Id | Title | Carry | Affects | Resolutions |
|---|---|---|---|---|
| [D1](D1.md) | Kernel C modules and the Rust program runtime inside paxd | rewritten | modules/layerxledger, modules/layerxpay, modules/layerxauth, modules/layerxprograms, modules/layerxexchange, 0x101A, 0x101B, 0x101C, 0x101D, 0x1015 | 1 |
| [D2](D2.md) | Kernel account and margin model | rewritten | 0x1001 bank, 0x1015 layerxexchange, 0x101A layerxledger, explorer balance views | 1 |
| [D3](D3.md) | Home of session keys and delegated authorities | kept | 0x101C layerxauth, sdk/x/authz, 0x101A, 0x101B, 0x101D | 1 |
| [D4](D4.md) | Custody claims and guarantor bonds at the activation height (superseded by decision.642) | superseded | 0x1013 layerxCustody, 0x1014 layerxAnchor, activation handler | 1 |
| [D5](D5.md) | Kernel balances without an addr binding at activation | kept | 0x1004 addr, activation handler | 1 |
| [D6](D6.md) | Precompile error encoding | rewritten | 0x1001, 0x1004, 0x1015, 0x1017, 0x1019, 0x101A, 0x101B, 0x101C, 0x101D, 0x0901-0x0908, precompiles/common | 1 |
| [D7](D7.md) | Time semantics under the sequencer cadence | kept | node/sequencer, modules/evm params, 0x1018 feeToken, 0x101D AI and market guests, 0x1019 xweb TimeoutBlocks | 1 |
| [D8](D8.md) | Trading semantics where kernel and exchange differ | rewritten | 0x1015 layerxexchange, 0x1017 launchpad, agent SDK order API | 1 |
| [D9](D9.md) | Program fee versus gas | kept | 0x101D layerxprograms | 1 |
| [D10](D10.md) | Paid search settlement and lxpay placement | kept | 0x101A layerxledger, modules/evm params, x-websearch sidecar, x402 seller | 1 |
| [D11](D11.md) | Program-origin web request terms | rewritten | 0x1019 xweb, 0x101D layerxprograms | 1 |
| [D12](D12.md) | Program-origin observation admission | kept | 0x1019 xweb, modules/xweb params, x-websearch sidecar | 1 |
| [D13](D13.md) | Reference guest deployment | kept | 0x101D layerxprograms, activation runbook | 1 |
| [D14](D14.md) | Sponsorship path and per-identity budget home | kept | gas station, BatchCallAndSponsor, 0x1004 addr, agent SDK nonce handling | 1 |
| [D15](D15.md) | Sequencer receipt signing key | kept | node/sequencer, modules/sequencer ReceiptKeys, authority service, SDK verifiers | 1 |
| [D16](D16.md) | DID key rotation and the embedded-wallet ed25519 share | kept | 0x1004 addr, embedded wallet | 1 |
| [D17](D17.md) | Client-facing contract | kept | agent SDKs, gateway, CLI, x402 seller, seller middleware, explorer status UI, wallet status UI | 1 |
| [D18](D18.md) | Sequencer consensus scope | kept | modules/sequencer, node/sequencer, consensus Leader(), node/legacyabci BeginBlock, activation height | 1 |
| [D19](D19.md) | Sequencer execution mode | kept | node/sequencer executor | 1 |
| [D20](D20.md) | Legacy surfaces | rewritten | cmd/layerxctl, program registry service, 0x0901-0x0908 | 1 |
| [D21](D21.md) | layerxAnchor reads after the fold | kept | 0x1014 layerxAnchor | 1 |
| [D22](D22.md) | Agent functions 0x0901-0x0904 | rewritten | 0x0901, 0x0902, 0x0903, 0x0904 | 1 |
| [D23](D23.md) | Revert reasons for addr and bank | rewritten | 0x1001 bank, 0x1004 addr, precompiles/common | 1 |
| [D24](D24.md) | Service dispute resolution | kept | 0x101B layerxpay service and escrow | 1 |
| [D25](D25.md) | Exchange withdrawMargin | rewritten | 0x1015 layerxexchange | 1 |
| [D26](D26.md) | Signer of the agent-host web payer | kept | agentd-host, embedded wallet attestors, 0x101A payer grants | 1 |
| [D27](D27.md) | Typed exact payment at 0x101A | kept | 0x101A layerxledger, x402 seller | 1 |
| [D28](D28.md) | xweb error naming | kept | 0x1019 xweb | 1 |
| [D29](D29.md) | One stream state | kept | 0x0906, 0x101B layerxpay | 1 |
| [D30](D30.md) | Gas rule across the new precompiles | rewritten | 0x101A, 0x101B, 0x101C, 0x101D, 0x0901-0x0908, 0x1015, 0x1019 | 1 |
| [D31](D31.md) | computeClearing and scoreBatch ABI form | kept | 0x0902, 0x0904 | 1 |
| [D32](D32.md) | submitPrice signer rule | rewritten | 0x0903, 0x1015 oracle store | 1 |
| [D33](D33.md) | TEE attestor roots | kept | 0x0907, 0x1019 xweb attestor set | 1 |
| [D34](D34.md) | TEE signer threshold and key type | kept | 0x0907, embedded wallet attestors | 1 |
| [D35](D35.md) | Result-code retryability source of truth | kept | stale-spec 10.1.1, result.rs, SDK result decoders | 1 |
| [D36](D36.md) | Receipt key range overlap | kept | stale-spec acceptance.6.72.2, stale-spec 6.5.32 | 1 |
| [D37](D37.md) | Stream rate update ordinal 8 | rewritten | stale-spec 2.2.2 | 1 |
| [D38](D38.md) | Blob store key layout | kept | stale-spec 13.2.2 | 1 |
| [D39](D39.md) | Payments and exchange governance surfaces | rewritten | stale-spec 13.3.14 | 1 |
| [D40](D40.md) | Kernel result codes -737 to -739 | kept | stale-spec 10.0.4 | 1 |
| [D41](D41.md) | Subscription lag signalling | kept | stale-spec 0.21, stale-spec 13.3.2 | 1 |
| [D42](D42.md) | Cross-module grant state stage | kept | stale-spec 2.4.1 | 1 |
| [D43](D43.md) | Activity proof verifier | kept | stale-spec 4.2.14 | 1 |
| [D44](D44.md) | Agent signing and client interfaces | rewritten | stale-spec 6.5.1, stale-spec 6.5.3, stale-spec 6.5.41, stale-spec 6.5.42 | 1 |
| [D45](D45.md) | Committed proof codecs | rewritten | stale-spec 6.5.19, stale-spec 6.5.27, stale-spec 6.5.29, stale-spec 6.5.30, stale-spec 6.5.36, stale-spec 8.5.31 | 1 |
| [D46](D46.md) | Sequencer handover encoding | rewritten | stale-spec 9.2.6, stale-spec 9.2.8, stale-spec 9.2.9 | 1 |
| [D47](D47.md) | .NET and Swift gates | rewritten | stale-spec 6.5.28 | 1 |
| [D48](D48.md) | PXVE v1 dev attestation envelope | kept | stale-spec 6.2.6, stale-spec 13.3.11 | 1 |
| [D49](D49.md) | Pure agent helpers gate on the agent module | kept | stale-spec 10.0.1, stale-spec 10.6.1 | 1 |
| [D50](D50.md) | AI reputation history retention prune runs at epoch opening | rewritten | stale-spec 4.2.12 | 1 |
| [D51](D51.md) | Merge plan name constant in the precompiles package | kept | stale-spec 0.1, stale-spec 10.0.1, stale-spec 10.6.1 | 1 |
| [D52](D52.md) | Gas station: a finalized earlier publication never consumes the renewal tick | kept | stale-spec 0.110, stale-spec 7.0.6 | 1 |
| [D53](D53.md) | Autobahn receiptsRoot is the same-block results root | kept | stale-spec 0.51, stale-spec 0.112 | 1 |
| [D54](D54.md) | KMS secp256k1 key kind for EVM sender handles | kept | stale-spec 7.0.8 | 1 |
| [D55](D55.md) | Wallet SDK formatting configuration | kept | stale-spec 5.2.5, stale-spec 0.116 | 1 |
| [D56](D56.md) | addr 0x1004 revert reasons use the registry LXP_ADDR codes | kept | stale-spec 1.1.5, stale-spec 10.2.1 | 1 |
| [D57](D57.md) | CosmWasm transaction counter is the block position from the merge plan height | kept | stale-spec 9.1.7, stale-spec 0.118 | 1 |
| [D58](D58.md) | Stream rate change is activity ordinal 8 end to end | kept | stale-spec 2.3.2 | 1 |
| [D59](D59.md) | Checkpoint settlement header without the retired policy field | kept | stale-spec 0.122, stale-spec 11.7.19 | 1 |
| [D60](D60.md) | One serde_json pin | kept | stale-spec 0.123, stale-spec 0.114 | 1 |
| [D61](D61.md) | FinalizeBlock response retention | kept | stale-spec 0.119 | 1 |
| [D62](D62.md) | Fatal kernel codes in the exchange code table | rewritten | stale-spec 3.2.8, stale-spec 3.5.1 | 1 |
| [D63](D63.md) | Intent plan vector holder | rewritten | stale-spec 6.5.36, stale-spec 12.5.102 | 1 |
| [D64](D64.md) | transactionsRoot verifier | kept | stale-spec 6.6.5 | 1 |
| [D65](D65.md) | History finality API removal order | rewritten | stale-spec 6.6.11 | 1 |
| [D66](D66.md) | Explorer paxeer-x page retirement | rewritten | stale-spec 11.5.69, stale-spec 11.5.67, stale-spec 11.5.68 | 1 |
| [D67](D67.md) | Guarantor header trim order | rewritten | stale-spec 11.5.115, stale-spec 11.7.19, stale-spec 11.7.23 | 1 |
| [D68](D68.md) | xweb program pair and the applied v6.10 state | kept | stale-spec 5.2.1 | 1 |
| [D69](D69.md) | newHeads receiptsRoot | kept | stale-spec 0.111 | 1 |
| [D70](D70.md) | lxphost raw-key scan scope | kept | stale-spec 1.1.13 | 1 |
| [D71](D71.md) | layerxagent params | kept | stale-spec 6.2.8, stale-spec 6.2.5, stale-spec 10.6.1, stale-spec 0.129 | 1 |
| [D72](D72.md) | Pay contract record views and stream rate event | kept | stale-spec 2.4.9, stale-spec 0.130 | 1 |
| [D73](D73.md) | Retained exchange intent selectors | rewritten | stale-spec 11.5.190 | 1 |
| [D74](D74.md) | Committed-proof verifier per language | kept | stale-spec 11.5.168, stale-spec 11.5.169, stale-spec 11.5.170, stale-spec 11.5.171, stale-spec 11.5.172, stale-spec 11.5.173, stale-spec 11.5.174, stale-spec 11.5.175, stale-spec 11.5.176, stale-spec 11.5.177, stale-spec 11.5.178, stale-spec 11.5.183, stale-spec 11.5.205, stale-spec 11.5.232, stale-spec 11.5.138, stale-spec 11.5.139 | 1 |
| [D75](D75.md) | Sequencer log ordinal | kept | stale-spec 0.25 | 1 |
| [D76](D76.md) | Indexer position migration | rewritten | stale-spec 0.27 | 1 |
| [D77](D77.md) | Prepared EIP-1559 fields order | rewritten | stale-spec 6.5.1, stale-spec 6.5.2 | 1 |
| [D78](D78.md) | native_terminal_outcome evidence | rewritten | stale-spec 0.113 | 1 |
| [D79](D79.md) | Explorer receipts heights | rewritten | stale-spec 0.131, stale-spec 8.7.20 | 1 |
| [D80](D80.md) | 8.73 acceptance vector | rewritten | stale-spec 8.5.36 | 1 |
| [D81](D81.md) | Gateway position cursor ownership | rewritten | stale-spec 0.27, stale-spec 7.5.6 | 1 |
| [D82](D82.md) | Wallet SDK curve library version | kept | stale-spec 0.125, stale-spec 0.116 | 1 |
| [D83](D83.md) | Oracle feeder transaction path | kept | stale-spec 7.3.2, stale-spec 7.1.2, stale-spec 0.140, stale-spec 0.141 | 1 |
| [D84](D84.md) | Model runner engine binding | kept | spec/layerx-merge/spec.kvx | 1 |
| [D85](D85.md) | Giga block fee-token charge destination | kept | spec/layerx-merge/spec.kvx | 1 |
| [D86](D86.md) | Curve library resolution in the wallet app | kept | spec/layerx-merge/spec.kvx | 1 |
| [D87](D87.md) | Vendor-native quotes in the exchange interface | kept | spec/layerx-merge/spec.kvx | 1 |
| [D88](D88.md) | Malformed snapshot ring refusal in launchpad price history | kept | spec/layerx-merge/spec.kvx | 1 |
| [D89](D89.md) | Relay journal restore admits the local signer beyond the observation cap | kept | spec/layerx-merge/spec.kvx | 1 |
| [D90](D90.md) | eth_subscribe relays over the WebSocket entry; HTTP answers WebSocket required | kept | stale-spec 13.5.1, stale-spec 7.5.2, stale-spec 6.5.48, stale-spec 7.5.3 | 1 |
| [D91](D91.md) | Explorer signature route vectors | kept | stale-spec 8.5.37 | 1 |
| [D92](D92.md) | Router sequence batch read | kept | stale-spec 0.165, stale-spec 0.166, stale-spec 0.163 | 1 |
| [D93](D93.md) | Hosted registry retirement is one compile unit | kept | stale-spec 11.5.163, stale-spec 11.5.164, stale-spec 11.5.165, stale-spec 7.5.13 | 1 |
| [D94](D94.md) | AI market guest size is met by a binaryen post-pass | kept | stale-spec 0.160, stale-spec 0.161, stale-spec 4.2.11, stale-spec 4.2.12 | 1 |
| [D1701](D1701.md) | 0x1015 is the perps venue | kept | 0x1015, modules/perps, precompiles/perps, modules/layerxexchange | 1 |
| [D1702](D1702.md) | pricefeed 0x1020 is the price source | kept | 0x1020, modules/pricefeed, precompiles/pricefeed | 1 |
| [D1703](D1703.md) | spot runs on PECOR at 0x1021-0x1025 | kept | 0x1021, 0x1022, 0x1023, 0x1024, 0x1025, modules/spot | 1 |
| [D1704](D1704.md) | launchpad is the v6.12.0 native rewrite | kept | 0x1017, modules/launchpad, precompiles/launchpad | 1 |
| [D1705](D1705.md) | xweb fee destinations by origin | kept | modules/xweb, 0x1019 | 1 |
| [D1706](D1706.md) | address-table reality: production serves none of the twelve merge addresses | kept | precompiles/setup.go, 0x101A, 0x101B, 0x101C, 0x101D, 0x0901, 0x0908 | 1 |
| [D1707](D1707.md) | saturn.wasm and venus.wasm are retired fixtures | kept | tests/integration/contracts, tools/benchmark/loadtest/contracts | 1 |
| [D1708](D1708.md) | launchpad API bundles and SQL tree are external gaps | kept | services/launchpad, apps/launchpad | 1 |
| [D1709](D1709.md) | go-cgo tasks prebuild the kernel archives | kept | modules/lxphost, precompiles | 1 |
| [D1710](D1710.md) | one programme gate at one revision | kept | spec/layerx-merge/programme | 1 |
| [D1711](D1711.md) | qualification ledger sections are unique | kept | spec/layerx-merge/qualification.kvx | 1 |
| [D1712](D1712.md) | stale task-ownership rulings are not carried | kept | spec/layerx-merge/spec.kvx | 1 |
| [D1713](D1713.md) | Eight requires cycles are broken by earlier fulfilment or by dropping same-wave requires | kept | 5.97.1, 5.53.1, 5.54.2, 5.54.10, 2.95.2, 6.97.2, 6.60.5, 5.55.5, 5.0.9, 17.93.1, 8.93.1, 16.93.1, 10.96.1, 16.97.5 | 0 |
| [D1714](D1714.md) | Merge-table interfaces not served at HEAD are new, and address values are hex only | kept | MergeAddressTable, interface status, override status | 0 |
| [D1715](D1715.md) | Wave gates are fulfilled by each wave's own transition task and the final gate replays the evidence | kept | 16.97.4, 16.1.1, 16.1.7, 17.5.1, 17.5.7, 17.5.8, 17.0.9, req.17061 | 0 |
| [D1716](D1716.md) | Contract and service facts follow the code at HEAD | kept | contract_context.3.pricefeed-0x1020, contract_context.3.oracle-0x1008, contract_context.2.layerxpay-0x101b, decision.401, service.human-kms, service.platform-runtime-clock, service.human-network-gateway, service.human-explorer-index | 0 |
| [D1717](D1717.md) | Service dependencies name existing service blocks | kept | service.platform-faucet, service dependencies | 0 |
| [D1718](D1718.md) | Idempotent receipt consumers conflict with the replay retirement: unresolved, flagged for domains 1, 2 and 13 | kept | decision.101, 1.2.1, 1.2.3, 1.25.1, 1.25.2, 2.7.4, 2.22.4, 2.40.1, 13.22.4, 13.30.3, req.13294 | 0 |
| [D1719](D1719.md) | Domain 13 places ReceiptAuthorities on the activities it submits | kept | 13.98.1, 1.26.1, wiring.J98.1, decision.192 | 0 |
| [D1720](D1720.md) | The J44 caller migrations are split by crate and wave | kept | wiring.J44.20, wiring.J44.21, wiring.J44.25, 9.24.3, 9.35.1, 10.20.3, 10.25.2, 10.0.3 | 0 |

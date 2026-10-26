# Decision index

Every `[decision.*]` block of spec.kvx at revision `2a1033aa2`: 615 decisions, 599 of them named by at least one `[resolution.*]` block. Ids 1-94 are carried-forward decisions; ids from 100 up are `<domain>*100+<k>` decisions written by the domain jobs. Each id links to its page with value, basis, affects and resolutions.

| Id | Title | Affects | Resolutions |
|---|---|---|---|
| [D1](D1.md) | Kernel C modules and the Rust program runtime inside paxd | modules/layerxledger, modules/layerxpay, modules/layerxauth, modules/layerxprograms, modules/layerxexchange, 0x101A, 0x101B, 0x101C, 0x101D, 0x1015 | 1 |
| [D2](D2.md) | Kernel account and margin model | 0x1001 bank, 0x1015 layerxexchange, 0x101A layerxledger, explorer balance views | 1 |
| [D3](D3.md) | Home of session keys and delegated authorities | 0x101C layerxauth, sdk/x/authz, 0x101A, 0x101B, 0x101D | 1 |
| [D4](D4.md) | Custody claims and guarantor bonds at the activation height (superseded by decision.642) | 0x1013 layerxCustody, 0x1014 layerxAnchor, activation handler | 1 |
| [D5](D5.md) | Kernel balances without an addr binding at activation | 0x1004 addr, activation handler | 1 |
| [D6](D6.md) | Precompile error encoding | 0x1001, 0x1004, 0x1015, 0x1017, 0x1019, 0x101A, 0x101B, 0x101C, 0x101D, 0x0901-0x0908, precompiles/common | 1 |
| [D7](D7.md) | Time semantics under the sequencer cadence | node/sequencer, modules/evm params, 0x1018 feeToken, 0x101D AI and market guests, 0x1019 xweb TimeoutBlocks | 1 |
| [D8](D8.md) | Trading semantics where kernel and exchange differ | 0x1015 layerxexchange, 0x1017 launchpad, agent SDK order API | 1 |
| [D9](D9.md) | Program fee versus gas | 0x101D layerxprograms | 1 |
| [D10](D10.md) | Paid search settlement and lxpay placement | 0x101A layerxledger, modules/evm params, x-websearch sidecar, x402 seller | 1 |
| [D11](D11.md) | Program-origin web request terms | 0x1019 xweb, 0x101D layerxprograms | 1 |
| [D12](D12.md) | Program-origin observation admission | 0x1019 xweb, modules/xweb params, x-websearch sidecar | 1 |
| [D13](D13.md) | Reference guest deployment | 0x101D layerxprograms, activation runbook | 1 |
| [D14](D14.md) | Sponsorship path and per-identity budget home | gas station, BatchCallAndSponsor, 0x1004 addr, agent SDK nonce handling | 1 |
| [D15](D15.md) | Sequencer receipt signing key | node/sequencer, modules/sequencer ReceiptKeys, authority service, SDK verifiers | 1 |
| [D16](D16.md) | DID key rotation and the embedded-wallet ed25519 share | 0x1004 addr, embedded wallet | 1 |
| [D17](D17.md) | Client-facing contract | agent SDKs, gateway, CLI, x402 seller, seller middleware, explorer status UI, wallet status UI | 1 |
| [D18](D18.md) | Sequencer consensus scope | modules/sequencer, node/sequencer, consensus Leader(), node/legacyabci BeginBlock, activation height | 1 |
| [D19](D19.md) | Sequencer execution mode | node/sequencer executor | 1 |
| [D20](D20.md) | Legacy surfaces | cmd/layerxctl, program registry service, 0x0901-0x0908 | 1 |
| [D21](D21.md) | layerxAnchor reads after the fold | 0x1014 layerxAnchor | 1 |
| [D22](D22.md) | Agent functions 0x0901-0x0904 | 0x0901, 0x0902, 0x0903, 0x0904 | 1 |
| [D23](D23.md) | Revert reasons for addr and bank | 0x1001 bank, 0x1004 addr, precompiles/common | 1 |
| [D24](D24.md) | Service dispute resolution | 0x101B layerxpay service and escrow | 1 |
| [D25](D25.md) | Exchange withdrawMargin | 0x1015 layerxexchange | 1 |
| [D26](D26.md) | Signer of the agent-host web payer | agentd-host, embedded wallet attestors, 0x101A payer grants | 1 |
| [D27](D27.md) | Typed exact payment at 0x101A | 0x101A layerxledger, x402 seller | 1 |
| [D28](D28.md) | xweb error naming | 0x1019 xweb | 1 |
| [D29](D29.md) | One stream state | 0x0906, 0x101B layerxpay | 1 |
| [D30](D30.md) | Gas rule across the new precompiles | 0x101A, 0x101B, 0x101C, 0x101D, 0x0901-0x0908, 0x1015, 0x1019 | 1 |
| [D31](D31.md) | computeClearing and scoreBatch ABI form | 0x0902, 0x0904 | 1 |
| [D32](D32.md) | submitPrice signer rule | 0x0903, 0x1015 oracle store | 1 |
| [D33](D33.md) | TEE attestor roots | 0x0907, 0x1019 xweb attestor set | 1 |
| [D34](D34.md) | TEE signer threshold and key type | 0x0907, embedded wallet attestors | 1 |
| [D35](D35.md) | Result-code retryability source of truth | stale-spec 10.1.1, result.rs, SDK result decoders | 1 |
| [D36](D36.md) | Receipt key range overlap | stale-spec acceptance.6.72.2, stale-spec 6.5.32 | 1 |
| [D37](D37.md) | Stream rate update ordinal 8 | stale-spec 2.2.2 | 1 |
| [D38](D38.md) | Blob store key layout | stale-spec 13.2.2 | 1 |
| [D39](D39.md) | Payments and exchange governance surfaces | stale-spec 13.3.14 | 1 |
| [D40](D40.md) | Kernel result codes -737 to -739 | stale-spec 10.0.4 | 1 |
| [D41](D41.md) | Subscription lag signalling | stale-spec 0.21, stale-spec 13.3.2 | 1 |
| [D42](D42.md) | Cross-module grant state stage | stale-spec 2.4.1 | 1 |
| [D43](D43.md) | Activity proof verifier | stale-spec 4.2.14 | 1 |
| [D44](D44.md) | Agent signing and client interfaces | stale-spec 6.5.1, stale-spec 6.5.3, stale-spec 6.5.41, stale-spec 6.5.42 | 1 |
| [D45](D45.md) | Committed proof codecs | stale-spec 6.5.19, stale-spec 6.5.27, stale-spec 6.5.29, stale-spec 6.5.30, stale-spec 6.5.36, stale-spec 8.5.31 | 1 |
| [D46](D46.md) | Sequencer handover encoding | stale-spec 9.2.6, stale-spec 9.2.8, stale-spec 9.2.9 | 1 |
| [D47](D47.md) | .NET and Swift gates | stale-spec 6.5.28 | 1 |
| [D48](D48.md) | PXVE v1 dev attestation envelope | stale-spec 6.2.6, stale-spec 13.3.11 | 1 |
| [D49](D49.md) | Pure agent helpers gate on the agent module | stale-spec 10.0.1, stale-spec 10.6.1 | 1 |
| [D50](D50.md) | AI reputation history retention prune runs at epoch opening | stale-spec 4.2.12 | 1 |
| [D51](D51.md) | Merge plan name constant in the precompiles package | stale-spec 0.1, stale-spec 10.0.1, stale-spec 10.6.1 | 1 |
| [D52](D52.md) | Gas station: a finalized earlier publication never consumes the renewal tick | stale-spec 0.110, stale-spec 7.0.6 | 1 |
| [D53](D53.md) | Autobahn receiptsRoot is the same-block results root | stale-spec 0.51, stale-spec 0.112 | 1 |
| [D54](D54.md) | KMS secp256k1 key kind for EVM sender handles | stale-spec 7.0.8 | 1 |
| [D55](D55.md) | Wallet SDK formatting configuration | stale-spec 5.2.5, stale-spec 0.116 | 1 |
| [D56](D56.md) | addr 0x1004 revert reasons use the registry LXP_ADDR codes | stale-spec 1.1.5, stale-spec 10.2.1 | 1 |
| [D57](D57.md) | CosmWasm transaction counter is the block position from the merge plan height | stale-spec 9.1.7, stale-spec 0.118 | 1 |
| [D58](D58.md) | Stream rate change is activity ordinal 8 end to end | stale-spec 2.3.2 | 1 |
| [D59](D59.md) | Checkpoint settlement header without the retired policy field | stale-spec 0.122, stale-spec 11.7.19 | 1 |
| [D60](D60.md) | One serde_json pin | stale-spec 0.123, stale-spec 0.114 | 1 |
| [D61](D61.md) | FinalizeBlock response retention | stale-spec 0.119 | 1 |
| [D62](D62.md) | Fatal kernel codes in the exchange code table | stale-spec 3.2.8, stale-spec 3.5.1 | 1 |
| [D63](D63.md) | Intent plan vector holder | stale-spec 6.5.36, stale-spec 12.5.102 | 1 |
| [D64](D64.md) | transactionsRoot verifier | stale-spec 6.6.5 | 1 |
| [D65](D65.md) | History finality API removal order | stale-spec 6.6.11 | 1 |
| [D66](D66.md) | Explorer paxeer-x page retirement | stale-spec 11.5.69, stale-spec 11.5.67, stale-spec 11.5.68 | 1 |
| [D67](D67.md) | Guarantor header trim order | stale-spec 11.5.115, stale-spec 11.7.19, stale-spec 11.7.23 | 1 |
| [D68](D68.md) | xweb program pair and the applied v6.10 state | stale-spec 5.2.1 | 1 |
| [D69](D69.md) | newHeads receiptsRoot | stale-spec 0.111 | 1 |
| [D70](D70.md) | lxphost raw-key scan scope | stale-spec 1.1.13 | 1 |
| [D71](D71.md) | layerxagent params | stale-spec 6.2.8, stale-spec 6.2.5, stale-spec 10.6.1, stale-spec 0.129 | 1 |
| [D72](D72.md) | Pay contract record views and stream rate event | stale-spec 2.4.9, stale-spec 0.130 | 1 |
| [D73](D73.md) | Retained exchange intent selectors | stale-spec 11.5.190 | 1 |
| [D74](D74.md) | Committed-proof verifier per language | stale-spec 11.5.168, stale-spec 11.5.169, stale-spec 11.5.170, stale-spec 11.5.171, stale-spec 11.5.172, stale-spec 11.5.173, stale-spec 11.5.174, stale-spec 11.5.175, stale-spec 11.5.176, stale-spec 11.5.177, stale-spec 11.5.178, stale-spec 11.5.183, stale-spec 11.5.205, stale-spec 11.5.232, stale-spec 11.5.138, stale-spec 11.5.139 | 1 |
| [D75](D75.md) | Sequencer log ordinal | stale-spec 0.25 | 1 |
| [D76](D76.md) | Indexer position migration | stale-spec 0.27 | 1 |
| [D77](D77.md) | Prepared EIP-1559 fields order | stale-spec 6.5.1, stale-spec 6.5.2 | 1 |
| [D78](D78.md) | native_terminal_outcome evidence | stale-spec 0.113 | 1 |
| [D79](D79.md) | Explorer receipts heights | stale-spec 0.131, stale-spec 8.7.20 | 1 |
| [D80](D80.md) | 8.73 acceptance vector | stale-spec 8.5.36 | 1 |
| [D81](D81.md) | Gateway position cursor ownership | stale-spec 0.27, stale-spec 7.5.6 | 1 |
| [D82](D82.md) | Wallet SDK curve library version | stale-spec 0.125, stale-spec 0.116 | 1 |
| [D83](D83.md) | Oracle feeder transaction path | stale-spec 7.3.2, stale-spec 7.1.2, stale-spec 0.140, stale-spec 0.141 | 1 |
| [D84](D84.md) | Model runner engine binding | spec/layerx-merge/spec.kvx | 1 |
| [D85](D85.md) | Giga block fee-token charge destination | spec/layerx-merge/spec.kvx | 1 |
| [D86](D86.md) | Curve library resolution in the wallet app | spec/layerx-merge/spec.kvx | 1 |
| [D87](D87.md) | Vendor-native quotes in the exchange interface | spec/layerx-merge/spec.kvx | 1 |
| [D88](D88.md) | Malformed snapshot ring refusal in launchpad price history | spec/layerx-merge/spec.kvx | 1 |
| [D89](D89.md) | Relay journal restore admits the local signer beyond the observation cap | spec/layerx-merge/spec.kvx | 1 |
| [D90](D90.md) | eth_subscribe relays over the WebSocket entry; HTTP answers WebSocket required | stale-spec 13.5.1, stale-spec 7.5.2, stale-spec 6.5.48, stale-spec 7.5.3 | 1 |
| [D91](D91.md) | Explorer signature route vectors | stale-spec 8.5.37 | 1 |
| [D92](D92.md) | Router sequence batch read | stale-spec 0.165, stale-spec 0.166, stale-spec 0.163 | 1 |
| [D93](D93.md) | Hosted registry retirement is one compile unit | stale-spec 11.5.163, stale-spec 11.5.164, stale-spec 11.5.165, stale-spec 7.5.13 | 1 |
| [D94](D94.md) | AI market guest size is met by a binaryen post-pass | stale-spec 0.160, stale-spec 0.161, stale-spec 4.2.11, stale-spec 4.2.12 | 1 |
| [D101](D101.md) | Idempotency replay retires; the actor sequence is the replay guard | task.1.2.1, task.1.2.2, task.1.2.3 | 1 |
| [D102](D102.md) | Capacity ledger retires whole | task.1.2.4 | 1 |
| [D103](D103.md) | Snapshot keeps the idempotency count word at zero | task.1.2.2, task.1.6.1 | 1 |
| [D104](D104.md) | Bridge and web module ids stay reserved | task.1.18.1, task.1.18.2, task.1.0.2 | 1 |
| [D105](D105.md) | Bridge reserve credit leaves the kernel | task.1.2.4, task.1.2.5, task.1.19.3 | 1 |
| [D106](D106.md) | Fee relay entries retire; fee governance staging stays | task.1.14.3, task.1.15.2 | 1 |
| [D107](D107.md) | Governance keeps its genesis key after handover leaves | task.1.17.1, task.1.17.2, task.1.17.3 | 1 |
| [D108](D108.md) | Guarantor includes leave kernel headers before anchor 1.0.3 | task.1.19.1, task.1.16.2, task.1.0.3 | 1 |
| [D109](D109.md) | Payer grant host read is coordinated with domain 1 host work | task.1.12.1 | 1 |
| [D110](D110.md) | Qualification table drops to 49 entries | task.1.12.3 | 1 |
| [D111](D111.md) | Module KV and staged write bounds are 4096 and 512 | task.1.16.3 | 1 |
| [D112](D112.md) | Asset binding authority replaces per-activity signature checks for bound hosts | task.1.11.1 | 1 |
| [D113](D113.md) | Module ids are frozen at the wave-2 anchor | task.1.0.2 | 1 |
| [D114](D114.md) | Daemon header deletion lands at wave 6 | task.1.18.3 | 1 |
| [D115](D115.md) | Authority grant codecs stay | task.1.12.1 | 1 |
| [D116](D116.md) | Native receipt authority is a 151-byte V2 record keyed by block height | task.1.17.4 | 1 |
| [D121](D121.md) | Settlement-chain client retired whole | task.1.32.1, task.1.32.2, task.1.32.3, task.1.32.4, task.1.32.5, task.1.38.1 | 1 |
| [D122](D122.md) | Oracle observation codec split from the adapter | task.1.29.1, task.1.29.2, interface.1.c-lx-oracle-observation-encode, interface.1.c-lx-oracle-observation-sign | 1 |
| [D123](D123.md) | Kernel admission kept, sequencer queue retired | task.1.28.1, task.1.28.2, task.1.28.3 | 1 |
| [D124](D124.md) | Retired log kinds refused on append and readable on read | task.1.27.1, interface.1.fmt-kernel-log-segment | 1 |
| [D125](D125.md) | Host loads only the modules an activity can read | task.1.22.1, task.1.22.2, task.1.39.1 | 1 |
| [D126](D126.md) | Proof codec primitives stay stable | task.1.36.1, task.1.36.2, interface.1.go-layerxproof-verify-withdrawal-effect, interface.1.go-layerxproof-codec-decode-merkle-proof | 1 |
| [D127](D127.md) | Legacy idempotency rows are ignored, not migrated | task.1.25.1, task.1.39.1, interface.1.key-lxphost-idempotency-j22 | 1 |
| [D128](D128.md) | Host node pool ceiling | task.1.26.1, task.1.39.2, interface.1.go-lxphost-config-j22 | 1 |
| [D129](D129.md) | Kernel headers are deleted after their definitions | task.1.38.1, task.1.38.2 | 1 |
| [D130](D130.md) | Batch header format kept | task.1.35.1, interface.1.fmt-batch-header, interface.1.c-lxp-batch-verify-signature | 1 |
| [D131](D131.md) | Signature validity is computed, binding authority is explicit | task.1.23.1, task.1.23.2, task.1.26.3 | 1 |
| [D132](D132.md) | Host executes on a node pool | task.1.26.1, task.1.39.2, interface.1.go-lxphost-host-j22 | 1 |
| [D133](D133.md) | Daemon header retires with the daemon | task.1.38.1 | 1 |
| [D134](D134.md) | Batch body records retire although handover reads them | task.1.35.2, task.1.27.1, interface.1.fmt-batch-body-archive | 1 |
| [D135](D135.md) | Batch signature verification moves to the header codec | task.1.35.1 | 1 |
| [D136](D136.md) | Kernel fee exposed in the host receipt | task.1.21.3, interface.1.go-lxphost-receipt-j22 | 1 |
| [D137](D137.md) | Retirement is proven on build products | task.1.21.1, task.1.21.2 | 1 |
| [D190](D190.md) | The golden replay release gate is kept and ported | 1.4.5, 16.98.1 | 1 |
| [D191](D191.md) | Wiring requests with no owner after merges are dropped | 2.7.4, 13.32.1 | 1 |
| [D192](D192.md) | The wave-6 host pool task fills the height authority index | 1.26.1, 1.17.4 | 1 |
| [D201](D201.md) | One shared kernel host over its own host store key; module stores hold params only | 2.3.1, 2.6.1, 2.14.4, 2.14.5 | 1 |
| [D202](D202.md) | Idempotency is kernel-owned | 2.1.6, 2.6.4, 2.14.4 | 1 |
| [D203](D203.md) | Pause routing goes through the layerxgov pause scope with revokeGrant as the exit | 2.1.7, 2.3.3, 2.6.3, 2.12.3 | 1 |
| [D204](D204.md) | Bank mirror under a native layerx denom | 2.1.5, 2.3.5, 2.0.3 | 1 |
| [D205](D205.md) | Global sequence comes from a host export | 2.3.4, 2.6.4 | 1 |
| [D206](D206.md) | The EVM DID binding replaces the envelope signature | 2.3.2, 2.6.2 | 1 |
| [D207](D207.md) | Receipts carry events and transfer legs | 2.3.5, 2.3.6, 2.6.5 | 1 |
| [D208](D208.md) | Precompile packages self-register as merge constructors | 2.10.1, 2.11.1, 2.2.5, 2.2.6 | 1 |
| [D209](D209.md) | No gRPC Query service; views are precompile-only and genesis carries params only | 2.1.1, 2.1.2, 2.5.3, 2.9.3 | 1 |
| [D210](D210.md) | Capability grants are mirrored as authz grants and a decorator supplies revocation sequences | 2.7.2, 2.7.3, 2.8.2, 2.12.4 | 1 |
| [D211](D211.md) | LayerXScopeAuthorization gains the kernel scope fields | 2.1.8, 2.8.1 | 1 |
| [D212](D212.md) | Go payload encoders live in the types packages; issueGrant is 346 bytes | 2.1.3, 2.1.4, 2.1.9 | 1 |
| [D213](D213.md) | Bank 0x1001 context-hash send and addr 0x1004 binding are baseline | 2.3.2, 2.13.1, 2.0.4 | 1 |
| [D221](D221.md) | The pay keeper runs on the shared kernel host; the layerxpay store keeps params only | 2.22.1, 2.22.5, 2.38.6 | 1 |
| [D222](D222.md) | escrowPartialCapture and escrowTimeout reuse the capture and release payloads | 2.21.4, 2.23.1, 2.23.2 | 1 |
| [D223](D223.md) | Idempotency keys and 0x0906 stream ids are derived from the transaction | 2.21.3, 2.24.2, 2.24.3, 2.25.5 | 1 |
| [D224](D224.md) | layerxpay Params.Validate stays a no-op; family flags are checked against kernel enable records | 2.22.3, 2.22.5, 2.27.1 | 1 |
| [D225](D225.md) | Pause lives in module Params and the layerxgov pause scope | 2.22.3, 2.32.3, 2.32.4, 2.38.5 | 1 |
| [D226](D226.md) | Scheduled jobs run in (block, id) order | 2.32.4, 2.40.6 | 1 |
| [D227](D227.md) | Agent end-block gas is capped by MaxBlockGas and counted in block gas | 2.32.1, 2.32.4, 2.38.3, 2.40.6 | 1 |
| [D228](D228.md) | Scheduled calldata is capped at MaxCallDataBytes 8192 | 2.32.1, 2.32.3, 2.39.1 | 1 |
| [D229](D229.md) | 0x0901 refusal codes follow the code at HEAD | 2.37.1 | 1 |
| [D230](D230.md) | Merge constructors pass explicit provisional caps | 2.36.2, 2.37.1, 2.37.2, 2.37.3, 2.37.4 | 1 |
| [D231](D231.md) | Agent gas is charged on the full call input | 2.34.1, 2.35.1, 2.36.1 | 1 |
| [D232](D232.md) | 0x0907 trusts the xweb attestor set and does not check the envelope nonce | 2.38.1 | 1 |
| [D233](D233.md) | AttestationVerified stays in the frozen ABI and is never emitted | 2.34.2, 2.38.1 | 1 |
| [D234](D234.md) | 0x101B logs are packed from call arguments and post-call kernel records | 2.28.3, 2.28.4, 2.28.5, 2.28.6, 2.28.7 | 1 |
| [D235](D235.md) | 0x101B charges the gas table through WithGasTable | 2.28.1 | 1 |
| [D236](D236.md) | Kernel authority comes from the caller's 0x1004 binding, with no signature | 2.22.2, 2.21.2 | 1 |
| [D237](D237.md) | The rate change is a kernel activity; StreamRateUpdated is logged at both addresses | 2.25.3, 2.28.6, 2.35.1 | 1 |
| [D238](D238.md) | 0x0906 accepts the kernel stream authority | 2.35.1, 2.40.4 | 1 |
| [D239](D239.md) | The 0x0903 oracle write and committed read belong to the exchange keeper | 2.36.1 | 1 |
| [D240](D240.md) | The service dispute arbiter check runs in the pay keeper until the kernel carries it | 2.26.4, 2.40.3 | 1 |
| [D301](D301.md) | 0x1015 is served by the pure-Go perps module and precompile | 3.0.1, 3.0.2, 3.0.3, 3.0.4 | 1 |
| [D302](D302.md) | 0x1015 revert data stays the parameterless custom-error selector | 3.4.1, 3.4.2 | 1 |
| [D303](D303.md) | Perps margin is USDL held as ERC20 store accounts | 3.6.3 | 1 |
| [D304](D304.md) | Pricefeed is the price source of the venue | 3.2.1, 3.2.2, 3.2.3 | 1 |
| [D305](D305.md) | Order ids come from the keeper sequence and order caps from params | 3.6.12 | 1 |
| [D306](D306.md) | The fixed 0x1015 gas table is baseline | 3.3.1 | 1 |
| [D307](D307.md) | Oracle 0x1008 is retired at latest and v6.4.0 | 3.18.1 | 1 |
| [D308](D308.md) | Oracle snapshots stay frozen at genesis | 3.18.1 | 1 |
| [D309](D309.md) | Oracle EventTypeVote is retired | 3.18.1 | 1 |
| [D310](D310.md) | Perps governance goes through Msgs in api/perps/tx.proto and a LayerX proposal | 3.0.1, 3.0.2, 3.0.3, 3.1.1, 3.1.4 | 1 |
| [D311](D311.md) | The perps pause stays local and withdraw is blocked while paused | 3.1.5 | 1 |
| [D312](D312.md) | Pricefeed MsgSetParams joins the LayerX proposal allow-list | 3.14.1, 3.14.2 | 1 |
| [D313](D313.md) | Perps and pricefeed sdk errors get LXP registry rows without changing revert data | 3.4.2, 3.13.1, 3.13.2 | 1 |
| [D314](D314.md) | PerpsAddress becomes a literal | 3.3.1 | 1 |
| [D315](D315.md) | getValidatorByFeedAccount keeps the zero-address sentinel | 3.15.1 | 1 |
| [D316](D316.md) | Pricefeed gets a read-only Query service and CLI; perps gets none | 3.0.4, 3.16.1, 3.16.2, 3.16.3, 3.16.4, 3.16.5, 3.16.6, 3.16.7, 3.16.8 | 1 |
| [D317](D317.md) | CollateralToken and listed-market identity are immutable through governance | 3.1.1, 3.1.2 | 1 |
| [D318](D318.md) | The withdraw floor and the ADL bound read params | 3.6.1, 3.6.2 | 1 |
| [D319](D319.md) | Lowering MaxOpenOrdersPerAccount below an open count is refused | 3.6.12 | 1 |
| [D320](D320.md) | Perps and pricefeed ConsensusVersion stay 1 | 3.0.2, 3.0.4 | 1 |
| [D321](D321.md) | Spot ABI custom errors are frozen | interface.3.pc-0x1021-err-zeroamount, interface.3.pc-0x1025-err-wethonly, contract_context.3.spot-served-set-j26 | 1 |
| [D322](D322.md) | Production serves the dynamic-gas spot wrapper | task.3.21.1, task.3.21.2, task.3.21.4, wiring.J26.1 | 1 |
| [D323](D323.md) | layerxexchange is replay-only after v6.12.0 | task.3.27.2, task.3.27.3, interface.3.msg-layerxexchange-setmarket | 1 |
| [D324](D324.md) | Retirement refusal semantics | task.3.27.1, task.3.27.2, task.3.27.3 | 1 |
| [D325](D325.md) | Governance keeps the exchange codec | task.3.27.4, wiring.J26.3 | 1 |
| [D326](D326.md) | Spot parameter bounds | task.3.23.1 | 1 |
| [D327](D327.md) | Governance listing registers the token book | task.3.23.2, task.3.23.4 | 1 |
| [D328](D328.md) | Active index position keys | task.3.22.1, task.3.22.2, task.3.22.5 | 1 |
| [D329](D329.md) | Epoch hooks re-emit their events | task.3.25.1, task.3.26.4 | 1 |
| [D330](D330.md) | Mint release amount fits int64 | task.3.26.1, task.3.26.2, task.3.26.3 | 1 |
| [D331](D331.md) | Retire the legacy mint REST routes | task.3.26.5 | 1 |
| [D332](D332.md) | Resting execution skips unpriced tokens | task.3.24.1 | 1 |
| [D333](D333.md) | layerxexchange revert data stays Error(string) | interface.3.go-pclxex-reverterror-j26, task.3.27.1 | 1 |
| [D334](D334.md) | Spot runs on PECOR at 0x1021-0x1025 | task.3.21.1 | 1 |
| [D341](D341.md) | 0x1015 is served by the Go perps precompile and the app ABI equals its JSON | 3.47.1, 3.47.2 | 1 |
| [D342](D342.md) | App tests run on node:test with tsx; service tests on go test | 3.48.1, 3.42.1 | 1 |
| [D343](D343.md) | Market and explorer streams and candles stay on the external market-data feed | 3.48.6 | 1 |
| [D344](D344.md) | The app market catalog comes from gateway GraphQL; order limits come from chain views | 3.48.5, 3.48.7, 3.55.1 | 1 |
| [D345](D345.md) | The perps-data market set follows perps.markets | 3.41.1, 3.41.3 | 1 |
| [D346](D346.md) | Retire the eight phantom spot GraphQL roots | 3.43.1, 3.43.3 | 1 |
| [D347](D347.md) | Minimum and maximum order value are enforced by mapping the chain reverts | 3.47.3 | 1 |
| [D348](D348.md) | Spot order fills are valued gross | 3.46.2 | 1 |
| [D349](D349.md) | NativeSwap stays excluded from PnL | 3.46.2 | 1 |
| [D350](D350.md) | Spot venue topics stay fixed to PAX and SID | 3.48.9 | 1 |
| [D351](D351.md) | Markets are refreshed periodically | 3.42.3, 3.42.5 | 1 |
| [D361](D361.md) | The unwired oracle v6.5 snapshot is recorded, not tasked | interface.3.go-oracle-getversioned-j25 | 1 |
| [D362](D362.md) | Perps and spot genesis do not cross-check pricefeed listings | 3.1.5, 3.23.4 | 1 |
| [D363](D363.md) | The spot user order index is append-only | 3.22.2 | 1 |
| [D364](D364.md) | The chain is the authority for perps-data market configuration | 3.42.2, 3.42.6 | 1 |
| [D365](D365.md) | Duplicate telemetry, the per-block epoch log and dead helpers are recorded, not tasked | interface.3.cli-epoch-tx-root | 1 |
| [D401](D401.md) | Launchpad at HEAD is the v6.12.0 native rewrite; v6.10 is a frozen replay copy | task.3.2.10, task.3.3.9, task.12.3.28, task.12.3.29, task.12.3.30, task.13.4.2, req.3020, req.3021 | 1 |
| [D402](D402.md) | Surface reverts use Launch-x custom-error selectors; keeper-only errors use Error(string) | decision.6, req.10006 | 1 |
| [D403](D403.md) | No per-market protocol fee counter; the protocol cut is credited to the Treasury ledger | decision.8, req.3021, task.3.2.11 | 1 |
| [D404](D404.md) | FeeAccumulator.getProtocolFeesPending stays a constant zero | interface.4.pc-fee-accumulator-get-protocol-fees-pending | 1 |
| [D405](D405.md) | Add a Cosmos Query service for operators, governance and indexers | req.3022, task.3.3.9 | 1 |
| [D406](D406.md) | PriceHistory reads the eight-slot snapshot ring with the swap refusals | decision.88, req.3022 | 1 |
| [D407](D407.md) | Launchpad error codes without a registry row get one in the shared LXP registry | interface.4.err-sdk-launchpad-2-invalid-params, interface.4.err-sdk-launchpad-24-invalid-genesis, interface.4.err-sdk-launchpad-30-invalid-range | 1 |
| [D408](D408.md) | Generation 2 role addresses stay compiled into the keeper | interface.4.go-launchpad-keeper-keeper-generation2 | 1 |
| [D409](D409.md) | Retire the generation 1 native airdrop, holding-history, balance hook and protocol-fee state | interface.4.go-launchpad-keeper-keeper-add-protocol-fees, task.13.4.2 | 1 |
| [D410](D410.md) | The precompile versions file lists the served versions v6.10 and v6.12.0 | interface.4.go-lppc-get-versioned | 1 |
| [D411](D411.md) | Contract operation scripts recognise native surfaces and refuse admin writes there | interface.4.cli-lp-contracts-script-lib-production-health, interface.4.cli-lp-contracts-script-lib-upgrade-preflight, interface.4.cli-lp-contracts-script-set-creation-fee | 1 |
| [D412](D412.md) | Stale launchpad tasks already landed at HEAD carry no new task | task.3.2.10, task.3.2.11, task.3.2.13, task.3.4.8 | 1 |
| [D421](D421.md) | The in-repo frozen catalog and schema documents are the launchpad contract authority | task.4.30.1, task.4.33.1, task.4.34.1, task.4.34.2, task.4.22.1 | 1 |
| [D422](D422.md) | Unbacked frozen api operations answer 503 not configured | task.4.24.4, task.4.24.5 | 1 |
| [D423](D423.md) | The user service serves the frozen v2 owner operations behind the gateway | task.4.22.4, task.4.23.3, task.4.23.4 | 1 |
| [D424](D424.md) | Ranking produces gainers and losers from the signed 24h change | task.4.31.1, task.4.31.2, interface.4.err-ranking-algo-invalid-category-j11 | 1 |
| [D425](D425.md) | Social serves the frozen v2 voice templates beside the connected v3 routes | task.4.32.2, task.4.32.3, task.4.22.5 | 1 |
| [D426](D426.md) | Asset upload and room message templates are not served at the gateway | task.4.22.2, task.4.22.3 | 1 |
| [D427](D427.md) | Social comments, callouts, bounties and reports stay platform-profile surfaces | task.4.32.4 | 1 |
| [D428](D428.md) | Service chain reads match the launchpad precompile ABIs | task.4.28.1, task.4.28.2, task.4.24.7, task.4.35.1 | 1 |
| [D429](D429.md) | Frozen notification ids are a generated UUID over the inbox key | task.4.27.2, interface.4.key-notifications-inbox-v2-id-j29 | 1 |
| [D430](D430.md) | Discovery serves the frozen /v2/squads operations | task.4.26.2, task.4.26.4, task.4.24.6 | 1 |
| [D431](D431.md) | Discovery serves the frozen /v2/me/alerts operations with a wallet resolver | task.4.36.3, task.4.24.8 | 1 |
| [D441](D441.md) | Service route registrations at HEAD are the route authority for clients | 4.48.1, 4.48.2, 4.46.3, 4.46.4, 4.46.5, 4.46.6, 4.46.7 | 1 |
| [D442](D442.md) | Unrouted SDK operations are refused locally with CapabilityUnavailableError | 4.48.1, 4.48.2 | 1 |
| [D443](D443.md) | Development-only operations are refused in production clients | 4.48.3, 4.48.6 | 1 |
| [D444](D444.md) | IF_REWARD_CLAIM is corrected and the duplicate voice operations are retired | 4.48.2, 4.48.4 | 1 |
| [D445](D445.md) | Clients pin surface roles by runtime hash only | 4.43.4, 4.45.2, 4.47.1, 4.55.1 | 1 |
| [D446](D446.md) | The tools/launchpad/production self-qualification harness gets no tasks | 4.49.1 | 1 |
| [D447](D447.md) | Embedded wallet calls stay with the wallet domain | 4.42.5 | 1 |
| [D448](D448.md) | Evidence-bound lane fixtures are blocked, not rewritten | 4.42.5 | 1 |
| [D449](D449.md) | The private fixture authority profile never applies in production | 4.44.1 | 1 |
| [D501](D501.md) | The CosmWasm transaction counter stays unset on the sequencer path | task.5.4.1, task.5.4.7 | 1 |
| [D502](D502.md) | A bad sequence batch halts the node; ProcessProposal rejects it first | task.5.0.2, task.5.2.2, task.5.3.1 | 1 |
| [D503](D503.md) | The merge plan pins a key for the first sequenced height | task.5.6.2 | 1 |
| [D504](D504.md) | The Layers seal path is retired | task.5.17.2 | 1 |
| [D505](D505.md) | The executor serves upgrade heights | task.5.4.2, task.5.17.2 | 1 |
| [D506](D506.md) | The wasm capability list is kept | task.5.13.1 | 1 |
| [D507](D507.md) | Embedded foreign genesis documents are retired | task.5.11.2 | 1 |
| [D508](D508.md) | An absent snapshot writer limit keeps the default | task.5.12.1, task.5.12.3 | 1 |
| [D509](D509.md) | The anchor store-delete step is its own plan | task.5.7.1, task.5.7.2 | 1 |
| [D510](D510.md) | MergeUpgrade has one source | task.5.6.1 | 1 |
| [D511](D511.md) | The node role is a config key | task.5.5.1, task.5.5.9 | 1 |
| [D512](D512.md) | The replication link is plain TCP bounded by the replication timeout | task.5.5.5 | 1 |
| [D513](D513.md) | The accepter fails stop after a replication failure | task.5.5.4, task.5.5.17 | 1 |
| [D514](D514.md) | A chain born merged follows the keeper activation height | task.5.0.2, task.5.11.4 | 1 |
| [D515](D515.md) | The application checks proposer membership only | task.5.1.2, task.5.6.4, task.5.18.1 | 1 |
| [D516](D516.md) | Sequenced reads the keeper activation height | task.5.0.2 | 1 |
| [D517](D517.md) | Sequenced blocks distribute from stored votes | task.5.9.1, task.5.9.2, task.5.0.3 | 1 |
| [D518](D518.md) | seqHash and the tx-0 codec live in the module types | task.5.17.1 | 1 |
| [D519](D519.md) | The sequencer config reaches the App through the daemon | task.5.0.1, task.5.5.12, task.5.5.15 | 1 |
| [D520](D520.md) | A superseded primary is fenced in the executor | task.5.15.3, task.5.15.4 | 1 |
| [D521](D521.md) | The first sequenced height is keeper state | 5.21.1, 5.24.2, 5.24.3, 5.25.2, 5.25.4 | 1 |
| [D522](D522.md) | The leader list is a governed param synced to LeaderListKey | 5.21.2, 5.25.1, 5.25.3, 5.25.4, 5.25.5 | 1 |
| [D523](D523.md) | No round number crosses ABCI | 5.31.1, 5.31.2 | 1 |
| [D524](D524.md) | Consensus reads the schedule through an in-process LeaderSource | 5.31.1, 5.31.2, 5.34.1 | 1 |
| [D525](D525.md) | The leader rotation runs over the full list | 5.31.1 | 1 |
| [D526](D526.md) | sequencer.enable only selects timeouts and excludes Autobahn | 5.32.1, 5.32.2, 5.33.1, 5.34.1 | 1 |
| [D527](D527.md) | Pinned heights never wait for mempool transactions | 5.33.2 | 1 |
| [D528](D528.md) | seqHash, txRoot and the tx-0 codec are canonical in modules/sequencer/types | 5.22.1, 5.22.2, 5.22.3, 5.22.4 | 1 |
| [D529](D529.md) | Decided votes are stored per height and two heights are kept | 5.21.1, 5.23.1, 5.23.2 | 1 |
| [D530](D530.md) | Fencing travels in the replication ack | 5.27.2, 5.27.3, 5.27.4 | 1 |
| [D531](D531.md) | Recovery order after both hosts stop | 5.27.1, 5.27.3 | 1 |
| [D532](D532.md) | Executor fencing lives in the executor | 5.27.4 | 1 |
| [D533](D533.md) | No store migration for the sequencer module | 5.24.2, 5.25.2 | 1 |
| [D541](D541.md) | occ_tests is absent; OCC tests live under tests/occ | occ_tests | 1 |
| [D542](D542.md) | tests/daemon retires with the C daemons | task.5.54.1, task.5.54.11 | 1 |
| [D543](D543.md) | layerxd retires entirely | task.5.50.1, task.5.50.7 | 1 |
| [D544](D544.md) | The agent store key is a storage constant | task.5.41.1, task.5.41.2 | 1 |
| [D545](D545.md) | The exact store-key golden list is extended, not loosened | task.5.41.2 | 1 |
| [D546](D546.md) | mvcc metric names are kept | task.5.44.1 | 1 |
| [D547](D547.md) | layerxctl reports submitted and receipted only | task.5.47.2, task.5.47.3 | 1 |
| [D548](D548.md) | The genesis builder is kept and its file formats stay stable | task.5.49.1, task.5.49.2 | 1 |
| [D549](D549.md) | Module registry read-node retires with the LNI | task.5.48.1 | 1 |
| [D550](D550.md) | The wasm runtime link is baseline | wasm-runtime | 1 |
| [D551](D551.md) | memory_cache_size replaces lru_size | task.5.46.1 | 1 |
| [D552](D552.md) | Receipt vectors are recorded under daemon/committed-fixture/vectors | task.5.55.1, task.5.55.5 | 1 |
| [D553](D553.md) | admin, modules/store and wasm-runtime are baseline | admin, modules/store, wasm-runtime | 1 |
| [D554](D554.md) | go.mod changes arrive by the tasks that need them | go.mod, go.sum | 1 |
| [D555](D555.md) | The sequencer config is still discarded by paxd | task.5.46.7 | 1 |
| [D556](D556.md) | Landed baseline is not re-tasked | task.5.47.1 | 1 |
| [D557](D557.md) | The host store key is mounted with the merge keys | task.5.41.2 | 1 |
| [D558](D558.md) | Merge fail-fast addresses live in their own list | task.5.45.1 | 1 |
| [D581](D581.md) | Merge precompile packages are linked by blank import | task.5.19.4, task.5.19.1 | 1 |
| [D582](D582.md) | Consensus receives the sequencer source from the App by value | task.5.0.1, task.5.5.12 | 1 |
| [D601](D601.md) | Precompile retirement is enforced per executor | 6.1.3, 6.1.4 | 1 |
| [D602](D602.md) | Upgrade reader comes from the EVM keeper UpgradeKeeper method | 6.1.1, 6.1.4, 6.3.1 | 1 |
| [D603](D603.md) | Keeper accessors are declared in the consuming package | 6.6.3 | 1 |
| [D604](D604.md) | Bank site codes follow the addr site rows | 6.2.1, 6.3.3, 6.3.5 | 1 |
| [D605](D605.md) | Constructor GetName values win over stale contract names | 6.12.1, 6.12.5, 6.12.11 | 1 |
| [D606](D606.md) | Merge constructor registration happens in node | 6.6.5, 6.12.11 | 1 |
| [D607](D607.md) | GetCustomPrecompilesE is added and the panicking form is kept | 6.6.2 | 1 |
| [D608](D608.md) | Perps errors stay outside the LXP registry | 6.2.1 | 1 |
| [D609](D609.md) | evm_pending_nonce counts added and rejected CheckTx nonces | 6.8.2 | 1 |
| [D610](D610.md) | Fee-denom preferences are genesis state | 6.7.6, 6.7.7, 6.7.9 | 1 |
| [D611](D611.md) | Unwired legacy v6.5 snapshots are recorded, not tasked | 6.0.1 | 1 |
| [D612](D612.md) | Stale binding call-form task is superseded | 6.7.6 | 1 |
| [D613](D613.md) | 0x1015 carries both the layerxexchange and perps names | 6.6.3, 6.8.4 | 1 |
| [D621](D621.md) | Denom pause gates user transfer paths, not keeper-level module flows | 6.25.1, 6.25.2 | 1 |
| [D622](D622.md) | GigaBankKeeper consults the tokenfactory send-enabled checker | 6.25.1, wiring.J35.1, wiring.J35.2 | 1 |
| [D623](D623.md) | Denom pause authority is issuer-bound and not governance-routed | 6.22.3, 6.22.6, 6.27.2, 6.28.6 | 1 |
| [D624](D624.md) | LXP_PAUSED_SCOPE text in two codespaces is kept | 6.27.2, 6.28.6 | 1 |
| [D625](D625.md) | Retirement is by refusal; types and codecs stay for replay | 6.31.4, 6.31.5, 6.28.5 | 1 |
| [D626](D626.md) | tokenfactory consensus version and historical migrations stay | 6.21.1, 6.21.4 | 1 |
| [D627](D627.md) | The tokenfactory gov proposal route stays registered and refusing | modules/tokenfactory/handler.go | 1 |
| [D628](D628.md) | CustodyAnchor stays until custody retires; the keeper AnchorReader goes | 6.34.1 | 1 |
| [D629](D629.md) | The anchor store-delete step is named v7.0-anchor-store-delete | 6.31.2, 6.32.2, wiring.J35.9 | 1 |
| [D630](D630.md) | No layerxgov-level retired-message list | 6.28.5 | 1 |
| [D631](D631.md) | Bonds are returned at the merge plan; reads continue until store delete | 6.33.1, 6.33.2, 6.32.2 | 1 |
| [D632](D632.md) | 0x1014 Solidity interfaces are retired by executor refusal, not deletion | 6.32.1, 6.32.2, 6.32.3 | 1 |
| [D633](D633.md) | An issuer binding closes the bech32 admin pause path | 6.22.6 | 1 |
| [D641](D641.md) | Custody retires at the v7.0 merge plan with no drain window | task.6.42.1, task.6.43.1, task.6.44.1, task.6.45.1 | 1 |
| [D642](D642.md) | Funds stranded in custody are released at the plan to the custody authority | task.6.46.1, task.6.47.1 | 1 |
| [D643](D643.md) | The 0x1013 retire gate runs first in Execute and returns error text | task.6.45.1 | 1 |
| [D644](D644.md) | Retired custody calls keep their RequiredGas | task.6.45.1 | 1 |
| [D645](D645.md) | The bridge precompile real upgrade-keeper path has landed |  | 1 |
| [D646](D646.md) | Forge gates in contracts/bridge/evm run bootstrap-libs first | task.6.50.1 | 1 |
| [D647](D647.md) | Re-targeting the bridge and custody precompile tests is unnecessary |  | 1 |
| [D648](D648.md) | The Sidiora proxy timelock and its four libraries are retained | task.6.55.1, task.6.55.2 | 1 |
| [D649](D649.md) | Kernel Solidity retirement is proven by tracked-file and live-reference checks | task.6.51.1 | 1 |
| [D650](D650.md) | tests/bridge/files.h and tests/bridge/requirements.txt stay | task.6.60.2 | 1 |
| [D651](D651.md) | The custody transaction CLI is removed in wave 6 | task.6.48.1 | 1 |
| [D652](D652.md) | The exchange custody margin hop fails with ErrModuleRetired after the plan | task.6.43.1 | 1 |
| [D653](D653.md) | Retire for custody protos and events means refused or not emitted | task.6.42.1, task.6.44.1 | 1 |
| [D654](D654.md) | ErrModuleRetired takes custody error code 29 | task.6.41.1 | 1 |
| [D655](D655.md) | Origin-2 fees stay in the per-asset web fee accounts | 6.63.2, 6.63.3 | 1 |
| [D656](D656.md) | The xweb keeper reads the merge plan through the upgrade keeper the EVM keeper carries | 6.61.1, 6.61.2, 6.61.3, 6.65.3 | 1 |
| [D657](D657.md) | Origin-2 methods of 0x1019 are height-gated in the order activation, pause, decode | 6.61.2, 6.61.3, 6.65.2, 6.65.3 | 1 |
| [D658](D658.md) | The fee denom of each asset is recorded at request time | 6.63.1, 6.63.2, 6.63.3 | 1 |
| [D659](D659.md) | Origin-2 state is carried in genesis with the kernel capacities | 6.64.1, 6.64.2, 6.64.3, 6.64.4, 6.75.1 | 1 |
| [D660](D660.md) | The attestor set cap stays at 64 entries | 6.66.1 | 1 |
| [D661](D661.md) | Activation params arrive by governance MsgSetParams | 6.67.1 | 1 |
| [D662](D662.md) | Interchain accounts stay unrouted | 6.73.1 | 1 |
| [D663](D663.md) | The CosmWasm stargate querier stays disabled | 6.72.1 | 1 |
| [D664](D664.md) | A CosmWasm gov message without a plain vote is refused | 6.72.1 | 1 |
| [D665](D665.md) | No precompile version bump for 0x1019 | 6.65.2, 6.65.3 | 1 |
| [D666](D666.md) | The committed answer capacity of 64 is permanent | 6.64.1, 6.69.3 | 1 |
| [D667](D667.md) | IBC core params become genesis field 4 | 6.73.1, 6.73.2, 6.73.3 | 1 |
| [D668](D668.md) | The gas table for 0x1019 origin-2 methods is the web.md D2 formula | 6.66.1, 6.65.2 | 1 |
| [D681](D681.md) | contracts/security and contracts/storage are owned by domain 6 and retire with the kernel contracts, except ReentrancyLock | task.6.80.1, task.6.80.2, task.6.80.3, task.6.80.4, task.6.80.5, interface.6.err-reentrancylock-reentrancy-ab143c06 | 1 |
| [D682](D682.md) | Hardhat aligns to the Foundry solc pin for contracts/src | task.6.81.1, interface.6.cfg-contracts-hardhat-solidity-version | 1 |
| [D683](D683.md) | IBank drops the two methods 0x1001 never served | task.6.82.1, interface.6.sol-ibank-sendfromcaller-0b5765aa, interface.6.sol-ibank-sendfromorigin-9fdfa201 | 1 |
| [D684](D684.md) | Legacy precompile snapshots are replay copies; the unwired v65 snapshots stay untasked | 6.0.1, interface.6.cfg-serve-0x1001, interface.6.go-pc-ibc-getversioned | 1 |
| [D685](D685.md) | Retired 0x1012, 0x1013 and 0x1014 calls decode their arguments before the retire gate | task.6.71.1, task.6.45.1, task.6.32.1 | 1 |
| [D701](D701.md) | lx_* at HEAD | 7.16.1, 7.0.2 | 1 |
| [D702](D702.md) | eth_sendRawTransactionSync semantics | 7.8.1, 7.8.2, 7.13.5 | 1 |
| [D703](D703.md) | debug raw header, block and receipts | 7.18.1 | 1 |
| [D704](D704.md) | Methods not served | 7.0.4 | 1 |
| [D705](D705.md) | newHeads stateRoot | 7.10.3 | 1 |
| [D706](D706.md) | newHeads field set | 7.10.1, 7.10.2 | 1 |
| [D707](D707.md) | Pending block field set | 7.11.1, 7.11.2, 7.11.3 | 1 |
| [D708](D708.md) | WebSocket service set | 7.9.2, 7.0.4 | 1 |
| [D709](D709.md) | rpc/AGENTS.md |  | 1 |
| [D710](D710.md) | web3_clientVersion | 7.4.1 | 1 |
| [D711](D711.md) | Rate limit key and answer | 7.13.3, 7.13.4 | 1 |
| [D712](D712.md) | checktx_timeout | 7.13.2 | 1 |
| [D713](D713.md) | Sequencer options in the node | 7.12.1, 7.12.2, 7.0.3 | 1 |
| [D714](D714.md) | txpool scope | 7.7.1 | 1 |
| [D715](D715.md) | Pending receipts | 7.8.1, 7.0.3 | 1 |
| [D716](D716.md) | Accidental exports | 7.2.1, 7.2.2, 7.2.3 | 1 |
| [D717](D717.md) | Pax registered set | 7.3.1, 7.3.2, 7.1.1 | 1 |
| [D718](D718.md) | Legacy metrics | 7.14.3, 7.14.4 | 1 |
| [D719](D719.md) | Unknown blockHash in eth_getLogs | 7.17.2 | 1 |
| [D720](D720.md) | Batch limit defaults | 7.13.5, 7.13.4 | 1 |
| [D721](D721.md) | The api/ test package belongs to rpc-api | 7.21.1, 7.29.1, 7.29.2 | 1 |
| [D722](D722.md) | The root buf v2 workspace is the single buf authority | 7.22.3, interface.7.fmt-api-buf-yaml, interface.7.fmt-api-buf-gen-yaml | 1 |
| [D723](D723.md) | Frozen camelCase proto fields keep their names | 7.22.1 | 1 |
| [D724](D724.md) | Breaking changes are checked with WIRE and WIRE_JSON against a committed image | 7.22.2 | 1 |
| [D725](D725.md) | Proto parity is checked at descriptor shape level | 7.21.2, 7.24.4, 7.26.2, 7.29.2 | 1 |
| [D726](D726.md) | Denom-valued REST routes gain query-string bindings | 7.26.1, 7.26.2, 7.26.3, 7.26.4 | 1 |
| [D727](D727.md) | REST path prefixes stay as declared | 7.29.1 | 1 |
| [D728](D728.md) | Reflection services stay gRPC-only | 7.29.1 | 1 |
| [D729](D729.md) | Removed field numbers are reserved | 7.27.1, 7.27.2 | 1 |
| [D730](D730.md) | One registration per proto file name | 7.28.1, 7.28.2 | 1 |
| [D731](D731.md) | layerxprograms protos are owned by the programs domain | 7.28.3 | 1 |
| [D732](D732.md) | Codegen toolchain pins | 7.29.2, interface.7.fmt-buf-gen-yaml-j39 | 1 |
| [D733](D733.md) | Logs subscription lifetime | 7.10.4, 7.10.5 | 1 |
| [D801](D801.md) | px_getReceiptKeys answers the gateway's pinned receipt key list | 8.1.1, 8.1.2 | 1 |
| [D802](D802.md) | Faucet funding stays on the faucet, testnet-control and core boundary chain | 8.2.1, 8.10.1 | 1 |
| [D803](D803.md) | lx_ switch-off is one change across HTTP, WebSocket and OpenRPC | 8.3.1 | 1 |
| [D804](D804.md) | px_getAccount and px_getBalances drop the kernel half | 8.5.1 | 1 |
| [D805](D805.md) | px_getNetwork reports the consensus head, not an anchor checkpoint | 8.5.3, 8.4.4 | 1 |
| [D806](D806.md) | Capability probes cover every merged construct with one served view each | 8.4.2, 8.4.3 | 1 |
| [D807](D807.md) | Agent-boundary receipt identifiers are EVM transaction hashes | 8.8.1, 8.8.2 | 1 |
| [D808](D808.md) | Kernel program surfaces of the hosted services retire rather than translate | 8.7.2, 8.7.3, 8.8.2, 8.8.3 | 1 |
| [D809](D809.md) | One task per wave edits the platform lock file | 8.9.1, 8.9.2, 8.20.3, 8.28.1 | 1 |
| [D810](D810.md) | Testnet funding pays the EVM address bound to the funded DID | 8.10.1, 8.10.2 | 1 |
| [D811](D811.md) | The core boundary keeps its kernel routes in this card | 8.10.1, 8.10.2, 8.20.1 | 1 |
| [D812](D812.md) | Hosted readers pin receipt keys from a file, not from checkpoints | 8.11.1, 8.11.2, 8.8.1 | 1 |
| [D813](D813.md) | Webhook finality on the merged chain is the committed block | 8.13.1, 8.13.2 | 1 |
| [D814](D814.md) | Testnet journeys keep their scripts and change their methods | 8.14.3, 8.14.4 | 1 |
| [D815](D815.md) | Router anchors are integration tests over the real gateway binary | 8.0.3, 8.0.4 | 1 |
| [D816](D816.md) | Indexer finality on the merged chain is the committed block | 8.16.2, 8.16.7 | 1 |
| [D817](D817.md) | Kernel settlement and relay ingestion retire from the indexer | 8.16.4, 8.16.7 | 1 |
| [D818](D818.md) | The storage keeper and the hosted guarantors retire with the kernel checkpoint path | 8.17.1, 8.17.2, 8.18.1, 8.19.1 | 1 |
| [D819](D819.md) | The hosted node launcher stays until a merged-chain pod replaces it | 8.19.1, 8.19.2, 8.19.3 | 1 |
| [D820](D820.md) | Human provisioning funds accounts by chain transfer, not custody deposit | 8.19.4, 8.19.10 | 1 |
| [D821](D821.md) | x402 commitment terms are the receipted and committed rungs | task.8.21.2, task.8.23.1, task.8.24.1, task.8.25.1, task.8.33.2, task.8.34.1, task.8.34.4 | 1 |
| [D822](D822.md) | x402 networks are CAIP-2 eip155 identifiers | task.8.21.2, task.8.23.1, task.8.24.1, task.8.33.2, task.8.34.2, task.8.26.2 | 1 |
| [D823](D823.md) | Payment receipt evidence is the PortableReceiptV2 document | task.8.22.1, task.8.22.2, task.8.26.1 | 1 |
| [D824](D824.md) | Receipt protocol version selection retires; receipt keys are pinned | task.8.22.1, task.8.23.2, task.8.24.3, task.8.24.4, task.8.25.2, task.8.26.1, task.8.26.3, task.8.33.1, task.8.33.2, task.8.33.4, task.8.34.1, task.8.34.2, task.8.34.3 | 1 |
| [D825](D825.md) | Agent daemon verification levels are Submitted, Receipted and Committed | task.8.24.2 | 1 |
| [D826](D826.md) | Kernel wire fixtures of the conformance suite are replaced by EVM fixtures | task.8.26.4, task.8.26.5, task.8.26.6, task.8.27.2 | 1 |
| [D827](D827.md) | Ramp LayerX legs are ledger calls to 0x101A | task.8.28.2, task.8.28.3, task.8.29.1, task.8.30.3, task.8.31.2 | 1 |
| [D828](D828.md) | The ramp signer signs secp256k1 transaction signing hashes | task.8.28.4 | 1 |
| [D829](D829.md) | Ramp finality is the committed rung read from the sequencer endpoint | task.8.29.2, task.8.29.3, task.8.30.1, task.8.30.3, task.8.30.4, task.8.31.2 | 1 |
| [D830](D830.md) | Ramp LayerxConfig names the sequencer endpoint, chain id and fee bounds | task.8.28.3, task.8.30.4 | 1 |
| [D831](D831.md) | The relay archive retires with the kernel | task.8.32.1, task.8.32.2, task.8.32.3, task.8.32.4 | 1 |
| [D834](D834.md) | Ramp journals move to a v2 domain and replay v1 read-only | task.8.29.1 | 1 |
| [D841](D841.md) | Sequencer receipt vectors V1-V3 are already recorded | task.8.41.1, task.8.42.1, task.8.43.1, task.8.44.1, task.8.47.7 | 1 |
| [D842](D842.md) | Kernel receipt verifier entry points stay until wave 6 | task.8.41.5, task.8.42.5, task.8.43.5, task.8.44.5, task.8.47.4, task.8.47.5, task.8.47.6 | 1 |
| [D843](D843.md) | Payload codecs stay inside layerxprograms calldata | task.8.47.1, task.8.47.2, task.8.47.9 | 1 |
| [D844](D844.md) | The pipeline lock is resealed once in wave 6 | task.8.45.4 | 1 |
| [D845](D845.md) | Already landed CLI register and derive bind are not re-specified | task.8.50.4 | 1 |
| [D846](D846.md) | The platform CLI already uses no retired layerx_client module | 9.24.3 | 1 |
| [D890](D890.md) | Task 8.28.1 is the wave-0 lock-file editor | 8.28.1, 8.31.2, 8.9.2 | 1 |
| [D891](D891.md) | Retirement of the gateway receipt views wins over the explorer receipt consumer | 8.6.1, 14.23.3, 14.32.3, 14.32.4 | 1 |
| [D901](D901.md) | The agent RPC envelope version stays 1 across the merge | 9.0.2 | 1 |
| [D902](D902.md) | The agent API schema is already at contract major 2 with rungs | 9.1.3, 9.1.4, 9.18.4 | 1 |
| [D903](D903.md) | The outbox adopts the nine schema submission states | 9.3.1, 9.3.2, 9.3.3 | 1 |
| [D904](D904.md) | Faucet funding is an MCP tool over px_requestFunds only | 9.1.5, 9.15.1 | 1 |
| [D905](D905.md) | Managed agent owner rotation is refused after the merge | 9.2.4 | 1 |
| [D906](D906.md) | The human peer frame LXHAGT01 is kept with its codec moved into agentd | 9.13.1, 9.13.2, 9.13.3 | 1 |
| [D907](D907.md) | Kernel verification levels map onto the three rungs | 9.0.1, 9.4.1, 9.1.4 | 1 |
| [D908](D908.md) | Idempotency windows are swept by block height | 9.3.2 | 1 |
| [D909](D909.md) | Subscription cursors become a block, transaction and log index triple | 9.11.1, 9.11.2, 9.11.3 | 1 |
| [D910](D910.md) | The layerx-wire crate stays as the agent's canonical encoding library | 9.18.3, 9.19.1 | 1 |
| [D921](D921.md) | The layerx-wire crate stays a workspace member; only its kernel activity envelope retires | 9.29.1 | 1 |
| [D922](D922.md) | The layerx-paxeer-verifier crate retires and leaves the agent workspace | 9.28.1, 9.25.2 | 1 |
| [D923](D923.md) | layerx-client keeps the node JSON-RPC client and retires the LNI kernel client modules | 9.21.6, 9.21.7, 9.24.1, 9.24.2, 9.24.3, 9.24.4, 9.24.6, 9.24.7, 9.25.1, 9.25.2 | 1 |
| [D924](D924.md) | layerx-crypto discloses and signs EIP-1559 calls and retires the kernel activity disclosure | 9.31.1, 9.31.2, 9.31.3, 9.35.1, 9.35.3 | 1 |
| [D925](D925.md) | layerx-sdk reads through eth_* and px_*, writes EIP-1559 layerxledger calls and verifies receipts against pinned receipt keys | 9.32.1, 9.32.2, 9.32.3, 9.32.4, 9.35.2, 9.35.4, 9.35.5 | 1 |
| [D926](D926.md) | The TypeScript and Python SDKs verify sequencer receipts at the receipted and committed rungs and drop anchor and checkpoint verification | 9.34.2, 9.33.5, 9.36.1 | 1 |
| [D927](D927.md) | VerificationLevel stays through wave 6 and gains a rung mapping | 9.26.1 | 1 |
| [D928](D928.md) | Guarantor checkpoint certificates and the checkpoint settlement document retire in wave 6 | 9.27.3, 9.27.4, 9.26.3 | 1 |
| [D929](D929.md) | Proof verifiers that bind a signed kernel activity retire with the activity envelope | 9.27.3, 9.27.5, 9.29.1 | 1 |
| [D930](D930.md) | Revert classification of registry-only rows is a layerx-types change; the header mirror needs no Rust edit | 9.26.2 | 1 |
| [D990](D990.md) | J44 agent caller migrations for wiring.J44.20 and J44.21 go to J98 | 9.24.3, 9.24.7, 9.25.1, 9.25.2 | 1 |
| [D991](D991.md) | J44 caller migration for wiring.J44.22 is owned by task 11.25.5 | 9.24.3, 11.25.5 | 1 |
| [D992](D992.md) | J44 caller migration for wiring.J44.25 goes to J98 | 9.35.1 | 1 |
| [D1001](D1001.md) | The human-api stays at major 1 and retires operations by refusal | 10.15.2, 10.0.2, 10.15.5, 10.15.7 | 1 |
| [D1002](D1002.md) | Human code uses layerx_types rungs and keeps the human-api VerificationLevel strings | 10.2.1, 10.2.3, 10.15.3, 10.14.1 | 1 |
| [D1003](D1003.md) | The account_sequence field name is kept and carries the pending-tag nonce | 10.7.1, 10.13.4, 10.13.7, 10.5.2 | 1 |
| [D1004](D1004.md) | The human service carries its own frame transport and keeps the LXHAGT peer codec | 10.0.1, 10.4.1, 10.4.4, 10.13.3 | 1 |
| [D1005](D1005.md) | KMS EVM keys live in the attestor secp256k1 quorum | 10.11.1, 10.11.2, 10.11.3, 10.11.5 | 1 |
| [D1006](D1006.md) | Onboarding is sponsored by the gas station with bindLayerX first | 10.12.1, 10.12.2 | 1 |
| [D1007](D1007.md) | The settlement recipient is retired end to end | 10.11.4, 10.20.3, 10.12.2 | 1 |
| [D1008](D1008.md) | Retired workspace members: superseded by decision.1081 | 10.1.2, 10.10.1 | 1 |
| [D1009](D1009.md) | The human service holds no direct chain endpoint | 10.20.3 | 1 |
| [D1010](D1010.md) | Compatibility rows are backfilled for schema 1.8 and 1.9 | 10.15.2, 10.16.3 | 1 |
| [D1011](D1011.md) | Rung imports and decode_rung are already landed | 10.2.1, 10.13.3 | 1 |
| [D1021](D1021.md) | Paxeer client and explorer index leave the human workspace in wave 5 | 10.26.1, 10.27.1, 10.1.2 | 1 |
| [D1022](D1022.md) | The explorer index crate is deleted, not re-targeted | 10.27.1, 10.27.3, 10.27.5, 10.27.7, 10.27.11 | 1 |
| [D1023](D1023.md) | The owner activity envelope retires instead of moving to EIP-1559 | 10.21.4 | 1 |
| [D1024](D1024.md) | LxpSend keeps account_sequence unchanged | override.J46.1, override.J46.2, override.J46.3 | 1 |
| [D1025](D1025.md) | Retired intent tags are refused as a retired shape and never reused | 10.21.3 | 1 |
| [D1026](D1026.md) | Disclosure of 0x1015 v2 methods is not added to layerx-intents | 10.21.1, 10.21.2 | 1 |
| [D1027](D1027.md) | The gateway client follows the chain-only px_ answers | 10.24.1, 10.24.2, 10.24.3 | 1 |
| [D1081](D1081.md) | All three retired crates leave the human workspace members in wave 5; task 10.1.2 trims the manifest in wave 6 | 10.1.2, 10.10.1, 10.10.2, 10.26.1, 10.27.1, 10.96.1 | 1 |
| [D1101](D1101.md) | Stale finding 5.0.8 is not reproduced at HEAD | 11.15.1, 11.15.2 | 1 |
| [D1102](D1102.md) | The service relays payer-signed transactions instead of posting hosted activities | 11.11.2, 11.16.2, 11.16.3, 11.16.4, 11.18.1 | 1 |
| [D1103](D1103.md) | One canonical receipt digest | 11.0.1, 11.1.1, 11.9.2, 11.17.2 | 1 |
| [D1104](D1104.md) | Committed is reported only with a verified committed proof | 11.4.2, 11.5.2 | 1 |
| [D1105](D1105.md) | A pending x402 settlement names the transaction hash | 11.16.2, 11.4.1 | 1 |
| [D1106](D1106.md) | AP2 keeps the lxp: digest reference | 11.7.1, 11.6.1 | 1 |
| [D1107](D1107.md) | A pinned receipt keys file replaces the batch-window variables | 11.12.1, 11.12.4, 11.12.5, 11.18.1 | 1 |
| [D1108](D1108.md) | J47 owns the layerx-committed-proof crate | 11.15.1, 11.2.2, 11.14.3 | 1 |
| [D1121](D1121.md) | The xweb precompile address is 0x1019 | interface.11.rs-xws-xweb-precompile, task.11.32.1, task.11.32.3 | 1 |
| [D1122](D1122.md) | layerx-migrate mapping v2 already reads at one finalized height | contract_context.11.bridge-relayer | 1 |
| [D1123](D1123.md) | x-websearch settles paid routes on the 0x101A ledger precompile with receiver-signed EVM transactions | task.11.34.1, task.11.34.2, task.11.34.3, task.11.34.5, task.11.34.6 | 1 |
| [D1124](D1124.md) | gateway.endpoint is kept as the settlement endpoint field | interface.11.cfg-xws-gateway-endpoint, task.11.34.4 | 1 |
| [D1125](D1125.md) | layerx-mirror-signer has no task |  | 1 |
| [D1126](D1126.md) | The x-websearch conformance suite is re-recorded once after the settlement fixtures settle | task.11.36.3, task.11.34.2, task.11.34.5, task.11.34.6 | 1 |
| [D1127](D1127.md) | The ingester removal check lives in layerx-portable | task.11.30.1 | 1 |
| [D1190](D1190.md) | wiring.J48.3 is fulfilled by task 16.3.1, not 11.30.1 | 11.30.1, 16.3.1, 11.22.2 | 1 |
| [D1201](D1201.md) | lx_send_authorization is evaluated under its own policy kind | 12.3.1, 12.3.3, 12.11.3 | 1 |
| [D1202](D1202.md) | EVM review is a new route family beside the narrowed lx routes | 12.0.2, 12.10.1, 12.10.3, 12.10.4, 12.11.2 | 1 |
| [D1203](D1203.md) | keys.generate is gated on the gateway certificate | 12.1.1 | 1 |
| [D1204](D1204.md) | Activity types narrow to the asset send 0x10005 | 12.12.1, 12.3.4, 12.19.3 | 1 |
| [D1205](D1205.md) | Attestor fee sponsors, trusted spenders and fee caps are landed baseline | 12.8.1 | 1 |
| [D1206](D1206.md) | The gateway sponsor key read is already retired | 12.6.3, 12.16.1 | 1 |
| [D1207](D1207.md) | layerxledger and layerxauth ABIs come from domain 2 | 12.2.1, 12.2.2 | 1 |
| [D1208](D1208.md) | Gateway writes go to SEQUENCER_RPC_URL and read the signed receipt | 12.0.1, 12.5.1, 12.5.2, 12.5.3, 12.6.2, 12.5.5, 12.5.6, 12.9.3, 12.10.4 | 1 |
| [D1209](D1209.md) | Retired tables are dropped by a forward migration | 12.14.4, 12.11.1 | 1 |
| [D1210](D1210.md) | Bind availability reads addr 0x1004 | 12.6.1 | 1 |
| [D1211](D1211.md) | Attestor metrics are served at /metrics to the operator | 12.4.1, 12.4.2, 12.4.3 | 1 |
| [D1221](D1221.md) | The wallet ladder rungs are submitted, receipted and committed with no anchor rung | 12.22.1, 12.22.2, 12.34.1, 12.35.1 | 1 |
| [D1222](D1222.md) | The wallet SDK reads pax_sequencerReceipt from the node and trusts the reported rung | 12.23.1, 12.23.3, 12.23.4 | 1 |
| [D1223](D1223.md) | Wallet history stays on px_getUnifiedHistory and drops LayerX rows client side | 12.24.1, 12.34.6 | 1 |
| [D1224](D1224.md) | The endpoint client adds chain reads beside the joined reads and retires the joined reads in wave 6 | 12.24.2, 12.24.3, 12.24.4, 12.24.6 | 1 |
| [D1225](D1225.md) | KernelClient and KernelAvailability keep their names over the merged reads | 12.25.2, 12.25.3, 12.25.4 | 1 |
| [D1226](D1226.md) | Wallet caps are observed at a block height on a rung | 12.25.1, 12.33.1 | 1 |
| [D1227](D1227.md) | The SDK sponsored quote is served by the gateway | 12.26.1 | 1 |
| [D1228](D1228.md) | The custody deposit flow is removed from the wallet app and SDK | 12.28.1, 12.28.2, 12.28.3, 12.34.8, 12.34.9 | 1 |
| [D1229](D1229.md) | EVM transaction review replaces lx activity review in the wallet SDK | 12.27.1, 12.27.2 | 1 |
| [D1230](D1230.md) | The exchange module on 0x1015 is the landed baseline | 12.33.2 | 1 |
| [D1231](D1231.md) | The app health check uses the gateway /healthz | 12.32.1 | 1 |
| [D1301](D1301.md) | Chain proofs are verified by an in-crate verifier without new crates | 13.0.1, 13.1.1, 13.1.2, 13.1.3, 13.1.4, 13.8.2 | 1 |
| [D1302](D1302.md) | Programs state keys use the host module KV layout | 13.0.2, 13.1.4, 13.2.5, 13.3.10 | 1 |
| [D1303](D1303.md) | Maintenance heads, trust history and batch fields retire | 13.3.3, 13.3.6, 13.4.6 | 1 |
| [D1304](D1304.md) | Sandbox expiry sweep bound becomes 128 per block | 13.6.1, 13.0.3 | 1 |
| [D1305](D1305.md) | Native receipt authority is keyed by block height | 13.7.2, 13.7.3, 13.7.5 | 1 |
| [D1306](D1306.md) | Program events become 0x101D logs | 13.7.1, 13.7.4, 13.0.3 | 1 |
| [D1307](D1307.md) | The frozen ABI checksum file is baseline |  | 1 |
| [D1308](D1308.md) | The workspace release profile is baseline |  | 1 |
| [D1309](D1309.md) | layerx_programs_call_begin keeps its signature | 13.7.4 | 1 |
| [D1310](D1310.md) | Sandbox evidence is state-based | 13.3.10, 13.6.2, 13.6.3, 13.6.4 | 1 |
| [D1311](D1311.md) | The protocol adapter crate is deleted, not retargeted | 13.8.2 | 1 |
| [D1312](D1312.md) | Validator signatures follow ZIP-215 | 13.1.14, 13.2.8 | 1 |
| [D1321](D1321.md) | deploy keeps seven inputs without feeLimit | 13.21.1, 13.23.1 | 1 |
| [D1325](D1325.md) | Sandbox expiry and retirement events are logged only inside 0x101D calls | 13.22.7 | 1 |
| [D1326](D1326.md) | Activity idempotency key from transaction hash and call ordinal | 13.21.8, 13.34.4 | 1 |
| [D1329](D1329.md) | 0x101D principal is the bound DID's kernel main account | 13.22.3 | 1 |
| [D1330](D1330.md) | Static calls serve views and simulateCall only | 13.22.2, 13.24.2 | 1 |
| [D1331](D1331.md) | quoteCallFee uses the occupancy formula of the current fee schedule | 13.24.3 | 1 |
| [D1332](D1332.md) | AI market snapshot binding v2 uses block-commit finality | 13.29.1, 13.29.2, 13.29.3 | 1 |
| [D1333](D1333.md) | Arbiter verifies ICS23 IAVL proofs in-crate without a new dependency | 13.30.1, 13.30.3, 13.30.4, 13.30.5 | 1 |
| [D1334](D1334.md) | AI market chain domain is derived from the merged chain id | 13.31.1 | 1 |
| [D1341](D1341.md) | layerxprograms Msg and Query services stay descriptor-only | 13.95.1, 7.28.3 | 1 |
| [D1342](D1342.md) | The arbiter crate keeps its agent-client, proof and wire dependencies | 13.8.2, 13.30.2, 13.30.3, 13.30.4, 13.30.5 | 1 |
| [D1401](D1401.md) | sdk/js becomes a first-party package aligned to HEAD by hand | 14.0.1, 14.1.1, 14.1.2, 14.1.3, 14.3.1, 14.3.2, 14.3.3, 14.3.4, 14.3.5, 14.3.6, 14.3.7, 14.3.8, 14.3.9, 14.3.10, 14.1.4 | 1 |
| [D1402](D1402.md) | Perps custom-error decoding is baseline |  | 1 |
| [D1403](D1403.md) | Send receipts are read with eth_getTransactionReceipt | 14.0.2, 14.4.1, 14.4.2, 14.4.3, 14.0.4 | 1 |
| [D1404](D1404.md) | One explorer origin | 14.5.3, 14.5.4 | 1 |
| [D1405](D1405.md) | Three verification rungs in the web kit | 14.9.1, 14.5.2, 14.5.5, 14.10.3, 14.9.2, 14.9.3, 14.9.4, 14.9.5, 14.9.6 | 1 |
| [D1406](D1406.md) | Custody journeys and wallet binding leave the web app | 14.7.1, 14.7.2, 14.7.3, 14.7.4, 14.7.5, 14.8.1, 14.8.2, 14.10.2, 14.11.1, 14.0.3 | 1 |
| [D1407](D1407.md) | sdk/js REST path segments are escaped | 14.3.6 | 1 |
| [D1421](D1421.md) | Keep the lx_* tables and stop writing the retired ones | task.14.25.1, task.14.25.3, task.14.25.4 | 1 |
| [D1422](D1422.md) | Serve kernel anchors and receipts read-only under /api/v2/paxeer-x/archive | task.14.23.2, task.14.23.3, task.14.23.6 | 1 |
| [D1423](D1423.md) | Keep the kernel frontend routes and repoint them at the archive | task.14.32.1, task.14.32.2, task.14.32.3, task.14.32.4, task.14.32.5 | 1 |
| [D1424](D1424.md) | Remove the unread finality settings instead of implementing a cache | task.14.26.2 | 1 |
| [D1425](D1425.md) | The contract verifier stays at its baseline | task.14.30.2 | 1 |
| [D1426](D1426.md) | A committed sequencer verification places an unindexed hash on the committed rung | task.14.22.2, task.14.22.3, task.14.36.1 | 1 |
| [D1427](D1427.md) | The unified account schemas follow the controller answer | task.14.27.1, task.14.27.2, task.14.33.1 | 1 |
| [D1428](D1428.md) | The sig-provider answers precompile selectors from the ABI files in the repository | task.14.30.1, task.14.30.2, task.14.30.3, task.14.30.4 | 1 |
| [D1429](D1429.md) | newHeads headers are compared on parentHash and receiptsRoot as well | task.14.28.1, task.14.28.2 | 1 |
| [D1441](D1441.md) | The services/bridge/deploy test suite needs no re-targeting task | task.14.53.4, task.14.58.1 | 1 |
| [D1442](D1442.md) | The bridge app invents no Paxeer explorer | task.14.57.1 | 1 |
| [D1443](D1443.md) | The copied telemetry hook is retired, not registered | task.14.56.1 | 1 |
| [D1444](D1444.md) | Submitted transfers are keyed by source hash and direction | task.14.45.1, task.14.47.2, task.14.48.3 | 1 |
| [D1445](D1445.md) | The app's vault variables come from deployment records | task.14.53.1, task.14.53.2, task.14.53.3, task.14.58.1 | 1 |
| [D1501](D1501.md) | Legacy go-metrics emissions are removed only after a test proves the OTel replacement records | 15.2.1, 15.2.2, 15.2.3, 15.2.4, 15.2.5, 15.2.6, 15.2.7, 15.2.8, 15.2.9, 15.2.10, 15.2.11, 15.2.12, 15.2.13, 15.2.14, 15.2.15, 15.2.16, 15.3.1, 15.3.2, 15.3.3, 15.3.4, 15.3.5, 15.3.6, 15.3.7, 15.3.8, 15.3.9, 15.3.10, 15.3.11, 15.3.12, 15.3.13, 15.3.14, 15.3.15, 15.3.16 | 1 |
| [D1502](D1502.md) | Bank refusal of paused tokenfactory denoms is landed through the send-enabled checker | sdk/x/bank/keeper/send.go | 1 |
| [D1503](D1503.md) | Integration repoints are gated by behaviour tests, not caller scans | task.15.0.4, task.15.4.1, task.15.4.2, task.15.4.3, task.15.4.4, task.15.4.5, task.15.4.6, task.15.4.7, task.15.4.8, task.15.4.9, task.15.4.10, task.15.4.11, task.15.4.12 | 1 |
| [D1504](D1504.md) | The sealed-state setter takes the sealed context as well as the multistore | task.15.0.3, wiring.J31.1 | 1 |
| [D1505](D1505.md) | The OTel upgrade plan gauges drop the info label | 15.3.4, 15.2.4 | 1 |
| [D1521](D1521.md) | Commit under Autobahn answers a named sentinel | 5.64.1, 5.64.2, 5.68.1 | 1 |
| [D1522](D1522.md) | consensus/networks/remote stays out of the gated surface | 5.62.5 | 1 |
| [D1523](D1523.md) | Version stamping targets the fork module path | 5.61.1, 5.61.2 | 1 |
| [D1524](D1524.md) | IBC light-client verification of this chain under Autobahn is out of scope | 5.68.1 | 1 |
| [D1525](D1525.md) | lag_status under Autobahn measures execution behind consensus | 5.64.7 | 1 |
| [D1526](D1526.md) | consensus_params under Autobahn answers the genesis parameters | 5.64.8 | 1 |
| [D1527](D1527.md) | Router event publication never fails block execution | 5.67.1 | 1 |
| [D1601](D1601.md) | Paxeer-x gate registrations move to a committed registry | task.16.9.1 | 1 |
| [D1602](D1602.md) | Tooling reads the layerx-merge spec and ledger | task.16.9.2, task.16.5.1, task.16.10.1, task.16.10.2, task.16.10.3, task.16.7.1 | 1 |
| [D1603](D1603.md) | Retired daemons leave the container images | task.16.3.1, task.16.3.2, task.16.3.3, task.16.3.4 | 1 |
| [D1604](D1604.md) | The beta deployment script retires whole | task.16.6.1 | 1 |
| [D1605](D1605.md) | The root Foundry profile stays for the kept contracts | task.16.12.1 | 1 |
| [D1606](D1606.md) | Makefile retirements run dependents first | task.16.2.2, task.16.0.2, task.16.2.3, task.16.0.3, task.16.0.4, task.16.2.4 | 1 |
| [D1607](D1607.md) | Wave gates compose existing aggregates | task.16.0.1, task.16.1.1 | 1 |
| [D1608](D1608.md) | Human service tooling stays except the movement role | task.16.3.6 | 1 |
| [D1609](D1609.md) | Generated, relative and non-path names are not defects |  | 1 |
| [D1610](D1610.md) | Rules whose source names a retired symbol keep their rule |  | 1 |
| [D1611](D1611.md) | The replay golden release gate retires with its target | task.16.2.2, task.16.5.6 | 1 |
| [D1621](D1621.md) | C suites are selected by Makefile target, and named cases by --run | 16.21.1, 16.23.1, 16.23.2 | 1 |
| [D1622](D1622.md) | No kernel-to-chain parity harness under tests/parity | 16.31.2, 16.33.4 | 1 |
| [D1623](D1623.md) | tests/solidity retires as a whole | 16.30.1, 16.30.2 | 1 |
| [D1624](D1624.md) | The paxload report is JSON | 16.24.2, 16.24.3 | 1 |
| [D1625](D1625.md) | Sourceless benchmark scenarios retire | 16.26.1, 16.26.2, 16.26.3 | 1 |
| [D1626](D1626.md) | Daemon-driven test suites retire with the daemon | 16.34.1, 16.34.2, 16.34.3, 16.35.1 | 1 |
| [D1627](D1627.md) | Benchmark regression threshold | 16.25.1, 16.25.2 | 1 |
| [D1628](D1628.md) | OCC harness fixtures follow the node capability list | 16.38.1 | 1 |
| [D1701](D1701.md) | 0x1015 is the perps venue | 0x1015, modules/perps, precompiles/perps, modules/layerxexchange | 1 |
| [D1702](D1702.md) | pricefeed 0x1020 is the price source | 0x1020, modules/pricefeed, precompiles/pricefeed | 1 |
| [D1703](D1703.md) | spot runs on PECOR at 0x1021-0x1025 | 0x1021, 0x1022, 0x1023, 0x1024, 0x1025, modules/spot | 1 |
| [D1704](D1704.md) | launchpad is the v6.12.0 native rewrite | 0x1017, modules/launchpad, precompiles/launchpad | 1 |
| [D1705](D1705.md) | xweb fee destinations by origin | modules/xweb, 0x1019 | 1 |
| [D1706](D1706.md) | address-table reality: production serves none of the twelve merge addresses | precompiles/setup.go, 0x101A, 0x101B, 0x101C, 0x101D, 0x0901, 0x0908 | 1 |
| [D1707](D1707.md) | saturn.wasm and venus.wasm are retired fixtures | tests/integration/contracts, tools/benchmark/loadtest/contracts | 1 |
| [D1708](D1708.md) | launchpad API bundles and SQL tree are external gaps | services/launchpad, apps/launchpad | 1 |
| [D1709](D1709.md) | go-cgo tasks prebuild the kernel archives | modules/lxphost, precompiles | 1 |
| [D1710](D1710.md) | one programme gate at one revision | spec/layerx-merge/programme | 1 |
| [D1711](D1711.md) | qualification ledger sections are unique | spec/layerx-merge/qualification.kvx | 1 |
| [D1712](D1712.md) | stale task-ownership rulings are not carried | spec/layerx-merge/spec.kvx | 1 |
| [D1713](D1713.md) | Eight requires cycles are broken by earlier fulfilment or by dropping same-wave requires | 5.97.1, 5.53.1, 5.54.2, 5.54.10, 2.95.2, 6.97.2, 6.60.5, 5.55.5, 5.0.9, 17.93.1, 8.93.1, 16.93.1, 10.96.1, 16.97.5 | 0 |
| [D1714](D1714.md) | Merge-table interfaces not served at HEAD are new, and address values are hex only | MergeAddressTable, interface status, override status | 0 |
| [D1715](D1715.md) | Wave gates are fulfilled by each wave's own transition task and the final gate replays the evidence | 16.97.4, 16.1.1, 16.1.7, 17.5.1, 17.5.7, 17.5.8, 17.0.9, req.17061 | 0 |
| [D1716](D1716.md) | Contract and service facts follow the code at HEAD | contract_context.3.pricefeed-0x1020, contract_context.3.oracle-0x1008, contract_context.2.layerxpay-0x101b, decision.401, service.human-kms, service.platform-runtime-clock, service.human-network-gateway, service.human-explorer-index | 0 |
| [D1717](D1717.md) | Service dependencies name existing service blocks | service.platform-faucet, service dependencies | 0 |
| [D1718](D1718.md) | Idempotent receipt consumers conflict with the replay retirement: unresolved, flagged for domains 1, 2 and 13 | decision.101, 1.2.1, 1.2.3, 1.25.1, 1.25.2, 2.7.4, 2.22.4, 2.40.1, 13.22.4, 13.30.3, req.13294 | 0 |
| [D1719](D1719.md) | Domain 13 places ReceiptAuthorities on the activities it submits | 13.98.1, 1.26.1, wiring.J98.1, decision.192 | 0 |
| [D1720](D1720.md) | The J44 caller migrations are split by crate and wave | wiring.J44.20, wiring.J44.21, wiring.J44.25, 9.24.3, 9.35.1, 10.20.3, 10.25.2, 10.0.3 | 0 |
| [D1721](D1721.md) | endOfBlock stays and KeyID is the pinned sequencer id | 5.17.2, 7.94.1, req.7961 | 0 |
| [D1722](D1722.md) | Interface notes follow the code at HEAD | interface.4.go-launchpad-keeper-keeper-add-protocol-fees, interface.7.rpc-pax-sequencerreceipt, override.J55.17 | 0 |
| [D1723](D1723.md) | The J16 launchpad Solidity inventory is canonical | interface.4.sol-lp-*, interface.4.sol-* | 0 |
| [D1724](D1724.md) | Verify commands match their toolchain and select every named test | verify_cmd, sandbox_profile, 9.27.6, 5.94.3 | 0 |
| [D1725](D1725.md) | The J60 service generator is stale and refuses to run | fragments/J60/_gen/gen_services.py, service blocks | 0 |
| [D1726](D1726.md) | 8.9.2 also removes the ramp toolkit lock entries | 8.9.2, req.8082, acceptance.8.82.1 | 0 |
| [D1727](D1727.md) | Handoff findings recorded without an edit | 2.28.1, 2.40.5, 13.22.1, 5.5.5, 5.5.8, 5.19.3, 3.93.1, 9.33.10, 8.41.2, applyNamedUpgrade | 0 |
| [D1728](D1728.md) | J95 programme findings follow the code at HEAD and the domain decisions | 6.46.1, 17.1.1, 17.1.3, 17.1.8, 17.1.9, 5.6.9, 17.2.3, 16.9.4, req.6485, req.17002, req.5063 | 0 |
| [D1790](D1790.md) | Waves 0-6 are balanced by moving free tasks | 1.0.4, 16.0.4 | 1 |
| [D1791](D1791.md) | J57 consensus and interchain tasks belong to domains 5 and 6 | 6.13.1, 5.61.1 | 1 |

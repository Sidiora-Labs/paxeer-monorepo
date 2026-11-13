# Precompile address and error registry

The registered precompiles execute within the merged Paxeer network. The source of address registration is [setup.go](../../../precompiles/setup.go), with application keepers supplied by [node wiring](../../../node/precompiles.go). Construct behavior is described by [ledger and auth](ledger-auth.md), [pay](pay.md), [markets](markets.md), [launchpad](launchpad.md), [programs](programs.md), [web](web.md), [agent functions](agent-functions.md), and [virtual chains](virtual-chains.md).

## (a) Address table

The existing/new column distinguishes base addresses from entries in `MergeAddressTable`. Registration, upgrade versions and module activation determine availability. The table uses full 20-byte EVM addresses.

| Address | Name (GetName) | Package path (address constant; registration) | Existing or new | Status after merge | Activation gate | Construct (merged) |
| --- | --- | --- | --- | --- | --- | --- |
| 0x0000000000000000000000000000000000001001 | bank | precompiles/bank | existing | served when its activation conditions hold | upgrade versions | bank |
| 0x0000000000000000000000000000000000001002 | wasmd | precompiles/wasmd | existing | served when its activation conditions hold | upgrade versions | wasmd |
| 0x0000000000000000000000000000000000001003 | json | precompiles/json | existing | served when its activation conditions hold | upgrade versions | json |
| 0x0000000000000000000000000000000000001004 | addr | precompiles/addr | existing | served when its activation conditions hold | upgrade versions | addr |
| 0x0000000000000000000000000000000000001005 | staking | precompiles/staking | existing | served when its activation conditions hold | upgrade versions | staking |
| 0x0000000000000000000000000000000000001006 | gov | precompiles/gov | existing | served when its activation conditions hold | upgrade versions | gov |
| 0x0000000000000000000000000000000000001007 | distribution | precompiles/distribution | existing | served when its activation conditions hold | upgrade versions | distribution |
| 0x0000000000000000000000000000000000001008 | oracle | precompiles/oracle | existing | served when its activation conditions hold | upgrade versions | oracle |
| 0x0000000000000000000000000000000000001009 | ibc | precompiles/ibc | existing | served when its activation conditions hold | upgrade versions | ibc |
| 0x000000000000000000000000000000000000100A | pointerview | precompiles/pointerview | existing | served when its activation conditions hold | upgrade versions | pointerview |
| 0x000000000000000000000000000000000000100B | pointer | precompiles/pointer | existing | served when its activation conditions hold | upgrade versions | pointer |
| 0x000000000000000000000000000000000000100C | solo | precompiles/solo | existing | served when its activation conditions hold | upgrade versions | solo |
| 0x0000000000000000000000000000000000001011 | p256Verify | precompiles/p256 | existing | served when its activation conditions hold | upgrade versions | p256Verify |
| 0x0000000000000000000000000000000000001012 | layerxVerify | precompiles/layerxverify | existing | registered; methods retired at the merge plan height | upgrade versions and module activation; see (c) | layerxVerify |
| 0x0000000000000000000000000000000000001013 | layerxCustody | precompiles/layerxcustody | existing | registered; methods retired at the merge plan height | upgrade versions and module activation; see (c) | layerxCustody |
| 0x0000000000000000000000000000000000001014 | layerxAnchor | precompiles/layerxanchor | existing | writes retired at the merge plan height; reads retired at the anchor store-delete plan height | upgrade versions and module activation; see (c) | layerxAnchor |
| 0x0000000000000000000000000000000000001015 | perps | precompiles/perps | existing | perps ABI from v6.12.0 | v6.12.0 contract switch; see (c) | perps |
| 0x0000000000000000000000000000000000001016 | layerxBridge | precompiles/layerxbridge | existing | served when its activation conditions hold | upgrade versions and module activation; see (c) | layerxBridge |
| 0x0000000000000000000000000000000000001017 | launchpad | precompiles/launchpad | existing | launchpad ABI from v6.12.0 | upgrade versions and module activation; see (c) | launchpad |
| 0x0000000000000000000000000000000000001018 | feeToken | precompiles/feetoken | existing | served when its activation conditions hold | upgrade versions and module activation; see (c) | feeToken |
| 0x0000000000000000000000000000000000001019 | xweb | precompiles/xweb | existing | served when its activation conditions hold | upgrade versions and module activation; see (c) | xweb |
| 0x000000000000000000000000000000000000101A | layerxledger | precompiles/layerxledger | new | served when its activation conditions hold | merge plan | layerxledger |
| 0x000000000000000000000000000000000000101B | layerxpay | precompiles/layerxpay | new | served when its activation conditions hold | merge plan | layerxpay |
| 0x000000000000000000000000000000000000101C | layerxauth | precompiles/layerxauth | new | served when its activation conditions hold | merge plan | layerxauth |
| 0x000000000000000000000000000000000000101D | layerxprograms | precompiles/layerxprograms | new | served when its activation conditions hold | merge plan | layerxprograms |
| 0x0000000000000000000000000000000000000901 | orob | precompiles/layerxagent/orob | new | served when its activation conditions hold | merge plan | orob |
| 0x0000000000000000000000000000000000000902 | clearing | precompiles/layerxagent/clearing | new | served when its activation conditions hold | merge plan | clearing |
| 0x0000000000000000000000000000000000000903 | oracle aggregator | precompiles/layerxagent/oracle | new | served when its activation conditions hold | merge plan | oracle aggregator |
| 0x0000000000000000000000000000000000000904 | pofq | precompiles/layerxagent/pofq | new | served when its activation conditions hold | merge plan | pofq |
| 0x0000000000000000000000000000000000000905 | scheduler | precompiles/layerxagent/scheduler | new | served when its activation conditions hold | merge plan | scheduler |
| 0x0000000000000000000000000000000000000906 | streams | precompiles/layerxagent/streams | new | served when its activation conditions hold | merge plan | streams |
| 0x0000000000000000000000000000000000000907 | tee attestor | precompiles/layerxagent/teeattestor | new | served when its activation conditions hold | merge plan | tee attestor |
| 0x0000000000000000000000000000000000000908 | eip712 | precompiles/layerxagent/eip712 | new | served when its activation conditions hold | merge plan | eip712 |
| 0x000000000000000000000000000000000000101E | layerxchains | precompiles/layerxchains | new | served when its activation conditions hold | merge plan | layerxchains |
| 0x0000000000000000000000000000000000001020 | pricefeed | precompiles/pricefeed | existing | served when its activation conditions hold | v6.12.0 | pricefeed |
| 0x0000000000000000000000000000000000001021 | spot PECOR | precompiles/spot | existing | served when its activation conditions hold | v6.12.0 | spot PECOR |
| 0x0000000000000000000000000000000000001022 | spot orders | precompiles/spot | existing | served when its activation conditions hold | v6.12.0 | spot orders |
| 0x0000000000000000000000000000000000001023 | spot stop orders | precompiles/spot | existing | served when its activation conditions hold | v6.12.0 | spot stop orders |
| 0x0000000000000000000000000000000000001024 | spot quoter | precompiles/spot | existing | served when its activation conditions hold | v6.12.0 | spot quoter |
| 0x0000000000000000000000000000000000001025 | spot vault | precompiles/spot | existing | served when its activation conditions hold | v6.12.0 | spot vault |

### (a2) Method selectors of changed and new addresses

Selected current signatures are listed below. A method selector is the first four bytes of Keccak-256 of its canonical signature; an event topic is its full hash. The complete ABIs are in each precompile package. At `0x1015`, use [the perps ABI](../../../precompiles/perps/abi/abi.json); at `0x1017`, use [the launchpad ABIs](../../../precompiles/launchpad/abi). Spot addresses and ABI files are defined in [spot keys](../../../modules/spot/types/keys.go) and [spot ABI files](../../../precompiles/spot/abi).

| Address | Canonical signature | Selector | Source (row) |
| --- | --- | --- | --- |
| 0x1001 | send(address,address,string,uint256,bytes32) | 0x2e4c85ac | `precompiles/bank/abi.json` |
| 0x1001 | log LayerXReceipt(address,address,string,uint256,bytes32) | topic0 0x4b11729a809ec729868d9ba54f3192aa4179fe63668e9f742aa9b25de21de65f | `precompiles/bank/abi.json` |
| 0x1019 | fulfilProgram(bytes) | 0x2d4bf7ad | `precompiles/xweb/abi.json` |
| 0x1019 | getProgramResult(bytes32,uint64) | 0x2b2e13fc | `precompiles/xweb/abi.json` |
| 0x1019 | getKernelNetworkId() | 0xfc0d26f4 | `precompiles/xweb/abi.json` |
| 0x101A | register(bytes32,bytes32,string,string,uint8,uint128,uint8,bytes) | 0xfd03012a | `precompiles/layerxledger/abi.json` |
| 0x101A | pause(bytes32) | 0xed56531a | `precompiles/layerxledger/abi.json` |
| 0x101A | unpause(bytes32) | 0x2f4dae9f | `precompiles/layerxledger/abi.json` |
| 0x101A | openAccount(bytes32) | 0xbdf52150 | `precompiles/layerxledger/abi.json` |
| 0x101A | send(bytes) | 0x2bb1ae7c | `precompiles/layerxledger/abi.json` |
| 0x101A | draw(bytes32,bytes32,uint128,bytes32,uint64,bytes32,bytes32) | 0x2e89490f | `precompiles/layerxledger/abi.json` |
| 0x101A | payExact(bytes32,bytes32,bytes32,uint128,uint64,bytes32) | 0x78ea11db | `precompiles/layerxledger/abi.json` |
| 0x101A | issueGrant(bytes) | 0xd0d53480 | `precompiles/layerxledger/abi.json` |
| 0x101A | revokeGrant(bytes32,uint64) | 0x58d183a8 | `precompiles/layerxledger/abi.json` |
| 0x101A | mint(bytes32,bytes32,uint128) | 0xf04db938 | `precompiles/layerxledger/abi.json` |
| 0x101A | burn(bytes32,bytes32,uint128) | 0xda3c63f3 | `precompiles/layerxledger/abi.json` |
| 0x101A | getAsset(bytes32) | 0x2cc3ce80 | `precompiles/layerxledger/abi.json` |
| 0x101A | balanceOf(bytes32,bytes32) | 0xb1ffc03e | `precompiles/layerxledger/abi.json` |
| 0x101A | totalUnits(bytes32) | 0x53a6332b | `precompiles/layerxledger/abi.json` |
| 0x101A | accountId(bytes32,bytes32) | 0x3f050cbc | `precompiles/layerxledger/abi.json` |
| 0x101A | accountOwner(bytes32) | 0x37955ab2 | `precompiles/layerxledger/abi.json` |
| 0x101A | getPayerGrant(bytes32) | 0x423fe4b2 | `precompiles/layerxledger/abi.json` |
| 0x101A | nextSequence(bytes32,bytes32) | 0xce2f921f | `precompiles/layerxledger/abi.json` |
| 0x101B | budgetPull(bytes32,bytes32,uint128,bytes) | 0xbc2548e7 | `precompiles/layerxpay/abi.json` |
| 0x101B | budgetDelegateSpend(bytes32,bytes32,uint128) | 0xf9708fc1 | `precompiles/layerxpay/abi.json` |
| 0x101C | grantSession(bytes,uint64,bytes32) | 0x1107b49e | `precompiles/layerxauth/abi.json` |
| 0x101C | grantSessionSuccessor(bytes,uint64,bytes32,bytes32,bytes32) | 0x0c49a046 | `precompiles/layerxauth/abi.json` |
| 0x101C | grantCapability(bytes) | 0x0cb291d1 | `precompiles/layerxauth/abi.json` |
| 0x101C | revokeGrant(bytes32,uint8,uint64) | 0x859615ab | `precompiles/layerxauth/abi.json` |
| 0x101C | getGrant(bytes32) | 0xe483ac04 | `precompiles/layerxauth/abi.json` |
| 0x101C | isLive(bytes32,uint64,uint64) | 0xd4a03b3d | `precompiles/layerxauth/abi.json` |
| 0x101C | sessionKey(bytes32) | 0x55eb131c | `precompiles/layerxauth/abi.json` |
| 0x101C | capabilityScope(bytes32) | 0xd05b2513 | `precompiles/layerxauth/abi.json` |
| 0x101C | checkActivity(bytes32,uint32,bytes32,uint128) | 0xd825990c | `precompiles/layerxauth/abi.json` |
| 0x101D | deploy(bytes32,uint16,uint8,bytes32,bytes32,bytes,bytes) | 0xbde3ff66 | `precompiles/layerxprograms/abi.json` |
| 0x101D | upgrade(bytes32,uint16,uint8,bytes32,bytes32,bytes,bytes,bytes,uint128) | 0xa860929f | `precompiles/layerxprograms/abi.json` |
| 0x101D | call(bytes32,uint16,string,bytes,bytes,bytes,uint32,uint64[7],uint128) | 0x54bd2b48 | `precompiles/layerxprograms/abi.json` |
| 0x101D | registryRead(bytes32,bytes) | 0xfb28c4b1 | `precompiles/layerxprograms/abi.json` |
| 0x101D | transfer(bytes32,(bytes32,bytes32,bytes32,uint128)[]) | 0x59849fe7 | `precompiles/layerxprograms/abi.json` |
| 0x101D | registerAccount(bytes32,uint8,bytes32,bytes) | 0xfc786007 | `precompiles/layerxprograms/abi.json` |
| 0x101D | setExitRoute(bytes32,bytes32,bytes32,bytes32,bytes) | 0x23330dd5 | `precompiles/layerxprograms/abi.json` |
| 0x101D | deprecate(bytes32,bytes32,uint64) | 0x86ba1a99 | `precompiles/layerxprograms/abi.json` |
| 0x101D | tombstone(bytes32) | 0xa512f1a9 | `precompiles/layerxprograms/abi.json` |
| 0x101D | exitValue(bytes32,bytes32) | 0x4dc50730 | `precompiles/layerxprograms/abi.json` |
| 0x101D | exitValueBounded(bytes32,bytes32,uint128) | 0xade5d9e6 | `precompiles/layerxprograms/abi.json` |
| 0x101D | sandboxExecute(bytes32,uint64,bytes32,bytes32,bytes32,bytes32,uint32,uint64[7],bytes,uint128) | 0xff04aa78 | `precompiles/layerxprograms/abi.json` |
| 0x101D | sandboxFund(bytes32,bytes32,bytes32,bytes32,bytes32,uint128,uint64,uint32,uint64[7],bytes,bytes32,(bytes32,bytes32,bytes32,uint128)[]) | 0xf11e5752 | `precompiles/layerxprograms/abi.json` |
| 0x101D | sandboxActivate(bytes32,bytes32,uint64) | 0x64e1674e | `precompiles/layerxprograms/abi.json` |
| 0x101D | retireStorage(bytes) | 0x1a5fe5ff | `precompiles/layerxprograms/abi.json` |
| 0x101D | simulateCall(bytes32,uint16,string,bytes,bytes,bytes,uint32,uint64[7],uint128) | 0x3044bd56 | `precompiles/layerxprograms/abi.json` |
| 0x101D | quoteCallFee(uint64[7]) | 0xd67f9ffa | `precompiles/layerxprograms/abi.json` |
| 0x0901 | resolveOffset(int256,int16) | 0x454ad3a1 | `precompiles/layerxagent/orob/orob.go` |
| 0x0901 | resolveOffsetBatch(int256,int16[]) | 0x3df597e3 | `precompiles/layerxagent/orob/orob.go` |
| 0x0901 | toOffset(int256,int256) | 0x3f779aef | `precompiles/layerxagent/orob/orob.go` |
| 0x0902 | computeClearing(int256,(int16,uint128)[],(int16,uint128)[]) | 0x8397e9f8 | `precompiles/layerxagent/clearing/clearing.go` |
| 0x0903 | aggregate((int256,uint256,uint256)[]) | 0x81097a95 | `precompiles/layerxagent/oracle/oracle.go` |
| 0x0903 | submitPrice(bytes32,int256,uint256) | 0xdc9c2875 | `precompiles/layerxagent/oracle/oracle.go` |
| 0x0903 | getValidatorPrice(bytes32) | 0x99812615 | `precompiles/layerxagent/oracle/oracle.go` |
| 0x0904 | scoreFill(int256,int256) | 0xe087a6ba | `precompiles/layerxagent/pofq/pofq.go` |
| 0x0904 | scoreBatch((int256,int256,uint128)[]) | 0x1b1fba48 | `precompiles/layerxagent/pofq/pofq.go` |
| 0x0904 | updateRollingScore(uint256,uint256,uint256,uint256,uint16) | 0xa8f32b0e | `precompiles/layerxagent/pofq/pofq.go` |
| 0x0905 | schedule(address,bytes,uint64,uint64) | 0x849ba0a3 | `precompiles/layerxagent/scheduler/scheduler.go` |
| 0x0905 | cancel(uint256) | 0x40e58ee5 | `precompiles/layerxagent/scheduler/scheduler.go` |
| 0x0905 | reschedule(uint256,uint64) | 0x08e6f770 | `precompiles/layerxagent/scheduler/scheduler.go` |
| 0x0905 | getJob(uint256) | 0xbf22c457 | `precompiles/layerxagent/scheduler/scheduler.go` |
| 0x0905 | pending(address) | 0x5eebea20 | `precompiles/layerxagent/scheduler/scheduler.go` |
| 0x0906 | open(address,address,uint256,uint64,uint64,uint256) | 0x93e5b08b | `precompiles/layerxagent/streams/streams.go` |
| 0x0906 | settle(uint256) | 0x8df82800 | `precompiles/layerxagent/streams/streams.go` |
| 0x0906 | close(uint256) | 0x0aebeb4e | `precompiles/layerxagent/streams/streams.go` |
| 0x0906 | updateRate(uint256,uint256) | 0x405abb41 | `precompiles/layerxagent/streams/streams.go` |
| 0x0906 | accrued(uint256) | 0xf52981f4 | `precompiles/layerxagent/streams/streams.go` |
| 0x0906 | getStream(uint256) | 0x894e9a0d | `precompiles/layerxagent/streams/streams.go` |
| 0x0907 | verify(uint8,bytes) | 0x5fa55ca7 | `precompiles/layerxagent/teeattestor/teeattestor.go` |
| 0x0907 | verifyAndExpect(uint8,bytes,bytes32) | 0x71ded4b3 | `precompiles/layerxagent/teeattestor/teeattestor.go` |
| 0x0907 | rootOf(uint8,uint256) | 0x01ec24b8 | `precompiles/layerxagent/teeattestor/teeattestor.go` |
| 0x0907 | rootCount(uint8) | 0x26f02502 | `precompiles/layerxagent/teeattestor/teeattestor.go` |
| 0x0908 | hashTypedData(bytes32,bytes32) | 0xd61c3c34 | `precompiles/layerxagent/eip712/eip712.go` |
| 0x0908 | domainSeparator(string,string,uint256,address) | 0xe6d341cd | `precompiles/layerxagent/eip712/eip712.go` |
| 0x0908 | recoverTypedSigner(bytes32,bytes32,bytes) | 0xac580b5c | `precompiles/layerxagent/eip712/eip712.go` |

### 0x101A resolution (D10 lxpay versus layerxledger)

`0x101A` serves `layerxledger`, including `issueGrant`, `revokeGrant`, `draw` and `payExact`. `0x101B` serves the pay module's escrow, budget, stream and service operations. The `ILxPay` subset in the [ledger contract](ledger-auth.md) gives x402 clients the ledger payment ABI at `0x101A`.

### Route-map entries that are not chain addresses

The [wallet gateway route map](../../../services/wallet/gateway/src/precompiles.ts) maps `staking` to `0x1005`, `bech32` to addr at `0x1004`, and `p256` to `0x1011`. `bech32` uses `getPaxAddr` and `getEvmAddr` from the addr ABI. Route keys select registered addresses.

### Code-owned ABIs (no construct contract)

Addr, bank, fee-token and bridge ABIs are published in their respective precompile directories. [BatchCallAndSponsor](../../../contracts/src/BatchCallAndSponsor.sol) is an EIP-7702 delegation target, with no fixed precompile address. The [gas station](../../../services/interop/crates/layerx-gas-station/src/tx.rs) requires the authorization delegate to match its configured paymaster.

## (b) Error code registry (D6)
SDK registry snapshot: repair-22.39.297-v1 — 395 codes, 54 retryable; SHA-256 of the registry rows including trailing LF: `6a9f742816f66fc1707c20ded000821c47bf9ac4715387e825e2da8e2b63615f`. 

Kernel numeric codes are defined in [lxp_result.h](../../../include/layerx/lxp_result.h). Go codes come from module errors and precompile error sites. The registry retains each code, message, origin metadata and retryable flag. Origin strings are metadata stored in this registry; the linked construct implementations define execution behavior.

| Rule | Value | Source |
| --- | --- | --- |
| Wire form | ABI `Error(string)`, selector `0x08c379a0`, with an `LXP_` code and message | [Revert reasons](../../../precompiles/common/precompiles.go) |
| Dynamic-gas reasons | Addr and bank enable reason encoding from the merge plan's height | [DynamicGasPrecompile](../../../precompiles/common/precompiles.go) |
| Kernel names | `lxp_result_name` supplies kernel result names; unknown values return `LXP_ERR_UNKNOWN` | [Result names](../../../src/protocol/lxp_result.c) |
| Retryability | The Retryable column supplies the Rust, TypeScript and Python decoder mappings | [Rust mapping](../../../services/agent/crates/layerx-types/build.rs), [SDK mapping](../../../scripts/ci/result-codes-gen.py) |

| Code | Message | Emitting construct | Kernel or Go origin (file:line) | Retryable |
|---|---|---|---|---|
| LXP_TRUNCATED | truncated | layerxledger 0x101A; layerxpay 0x101B; layerxprograms 0x101D; shared cgo host (all kernel-backed precompiles) | LXP_ERR_TRUNCATED = -1, include/layerx/lxp_result.h:31; emitted src/modules/programs/registration.c:160 | no |
| LXP_TRAILING_BYTES | trailing bytes | layerxexchange 0x1015; layerxpay 0x101B; layerxprograms 0x101D; shared cgo host (all kernel-backed precompiles) | LXP_ERR_TRAILING_BYTES = -2, include/layerx/lxp_result.h:32; emitted src/modules/stream/lx_stream_record.c:159 | no |
| LXP_NON_CANONICAL | non canonical | addr 0x1004 (DID identity); layerxauth 0x101C; layerxexchange 0x1015; kernel governance checks inside every kernel-backed precompile; layerxledger 0x101A; layerxpay 0x101B; layerxprograms 0x101D; shared cgo host (all kernel-backed precompiles); xweb 0x1019 (origin 2) | LXP_ERR_NON_CANONICAL = -3, include/layerx/lxp_result.h:33; emitted src/modules/asset/lx_asset_transfer.c:11 | no |
| LXP_UNSORTED_SEQUENCE | unsorted sequence | layerxexchange 0x1015; kernel governance checks inside every kernel-backed precompile; layerxprograms 0x101D; shared cgo host (all kernel-backed precompiles); xweb 0x1019 (origin 2) | LXP_ERR_UNSORTED_SEQUENCE = -4, include/layerx/lxp_result.h:34; emitted src/protocol/lxp_genesis.c:435 | no |
| LXP_LENGTH_LIMIT | length limit | layerxexchange 0x1015; kernel governance checks inside every kernel-backed precompile; layerxledger 0x101A; layerxpay 0x101B; layerxprograms 0x101D; shared cgo host (all kernel-backed precompiles); xweb 0x1019 (origin 2) | LXP_ERR_LENGTH_LIMIT = -5, include/layerx/lxp_result.h:35; emitted src/modules/asset/lx_asset_reserve.c:99 | no |
| LXP_INVALID_TAG | invalid tag | layerxprograms 0x101D; shared cgo host (all kernel-backed precompiles) | LXP_ERR_INVALID_TAG = -6, include/layerx/lxp_result.h:36; emitted src/modules/programs/occupancy_evidence.c:617 | no |
| LXP_UNKNOWN_FIELD | unknown field | layerxauth 0x101C; layerxexchange 0x1015; kernel governance checks inside every kernel-backed precompile; layerxledger 0x101A; layerxpay 0x101B; layerxprograms 0x101D; shared cgo host (all kernel-backed precompiles); xweb 0x1019 (origin 2) | LXP_ERR_UNKNOWN_FIELD = -7, include/layerx/lxp_result.h:37; emitted src/modules/asset/lx_asset_custody.c:232 | no |
| LXP_DUPLICATE_ENTRY | duplicate entry | layerxpay 0x101B; layerxprograms 0x101D; shared cgo host (all kernel-backed precompiles); xweb 0x1019 (origin 2) | LXP_ERR_DUPLICATE_ENTRY = -8, include/layerx/lxp_result.h:38; emitted src/modules/programs/blob_lifecycle.c:300 | no |
| LXP_WRONG_NETWORK | wrong network | layerxprograms 0x101D; shared cgo host (all kernel-backed precompiles); xweb 0x1019 (origin 2) | LXP_ERR_WRONG_NETWORK = -100, include/layerx/lxp_result.h:39; emitted src/modules/web/lx_web_intake.c:533 | no |
| LXP_VERSION_UNSUPPORTED | version unsupported | layerxauth 0x101C; layerxexchange 0x1015; kernel governance checks inside every kernel-backed precompile; layerxpay 0x101B; layerxprograms 0x101D; shared cgo host (all kernel-backed precompiles) | LXP_ERR_VERSION_UNSUPPORTED = -101, include/layerx/lxp_result.h:40; emitted src/modules/programs/occupancy_evidence.c:222 | no |
| LXP_UNKNOWN_MODULE | unknown module | layerxauth 0x101C; shared cgo host (all kernel-backed precompiles) | LXP_ERR_UNKNOWN_MODULE = -102, include/layerx/lxp_result.h:41; emitted src/protocol/lxp_genesis.c:90 | no |
| LXP_MODULE_DISABLED | module disabled | retired executors at 0x1012, 0x1013, 0x1014 and retired 0x1015 methods (section c); layerxauth 0x101C; layerxpay 0x101B; layerxprograms 0x101D; shared cgo host (all kernel-backed precompiles); xweb 0x1019 (origin 2) | LXP_ERR_MODULE_DISABLED = -103, include/layerx/lxp_result.h:42; emitted src/modules/escrow/lx_escrow_timeout.c:312 | no |
| LXP_PAYLOAD_HASH_MISMATCH | payload hash mismatch | layerxprograms 0x101D; shared cgo host (all kernel-backed precompiles) | LXP_ERR_PAYLOAD_HASH_MISMATCH = -104, include/layerx/lxp_result.h:43; emitted src/modules/programs/deploy.c:226 | no |
| LXP_MALFORMED_ENVELOPE | malformed envelope | layerxauth 0x101C; shared cgo host (all kernel-backed precompiles) | LXP_ERR_MALFORMED_ENVELOPE = -105, include/layerx/lxp_result.h:44; emitted src/state/lxp_authority.c:558 | no |
| LXP_UNKNOWN_ACTIVITY | unknown activity | layerxexchange 0x1015; layerxledger 0x101A; layerxpay 0x101B; layerxprograms 0x101D; shared cgo host (all kernel-backed precompiles); xweb 0x1019 (origin 2) | LXP_ERR_UNKNOWN_ACTIVITY = -106, include/layerx/lxp_result.h:45; emitted src/modules/asset/lx_asset_registry.c:345 | no |
| LXP_MALFORMED_SEND | malformed send | layerxledger 0x101A | LXP_ERR_MALFORMED_SEND = -107, include/layerx/lxp_result.h:46; emitted src/ledger/lxp_send.c:189 | no |
| LXP_MALFORMED_RECEIVE | malformed receive | layerxledger 0x101A | LXP_ERR_MALFORMED_RECEIVE = -108, include/layerx/lxp_result.h:47; emitted src/ledger/lxp_receive.c:193 | no |
| LXP_UNKNOWN_DID | unknown did | addr 0x1004 (DID identity); layerxauth 0x101C; shared cgo host (all kernel-backed precompiles) | LXP_ERR_UNKNOWN_DID = -200, include/layerx/lxp_result.h:48; emitted src/state/lxp_authority.c:1151 | no |
| LXP_BAD_SIGNATURE | bad signature | addr 0x1004 (DID identity); layerxauth 0x101C; kernel governance checks inside every kernel-backed precompile; layerxledger 0x101A; shared cgo host (all kernel-backed precompiles); xweb 0x1019 (origin 2); layerxpay 0x101B (pay.md) | LXP_ERR_BAD_SIGNATURE = -201, include/layerx/lxp_result.h:49; emitted src/modules/governance/lxp_governance.c:565 | no |
| LXP_AUTH_EXPIRED | auth expired | layerxauth 0x101C | LXP_ERR_AUTH_EXPIRED = -202, include/layerx/lxp_result.h:50; emitted src/state/lxp_authority.c:1129 | no |
| LXP_AUTH_REVOKED | auth revoked | addr 0x1004 (DID identity); layerxauth 0x101C; kernel governance checks inside every kernel-backed precompile | LXP_ERR_AUTH_REVOKED = -203, include/layerx/lxp_result.h:51; emitted src/modules/governance/lxp_governance.c:397 | no |
| LXP_AUTH_SCOPE | auth scope | addr 0x1004 (DID identity); layerxauth 0x101C; kernel governance checks inside every kernel-backed precompile; layerxledger 0x101A; layerxprograms 0x101D; shared cgo host (all kernel-backed precompiles) | LXP_ERR_AUTH_SCOPE = -204, include/layerx/lxp_result.h:52; emitted src/modules/programs/fees.c:372 | no |
| LXP_AUTH_ALLOWANCE | auth allowance | layerxauth 0x101C; layerxledger 0x101A; shared cgo host (all kernel-backed precompiles); xweb 0x1019 (origin 2) | LXP_ERR_AUTH_ALLOWANCE = -205, include/layerx/lxp_result.h:53; emitted src/state/lxp_authority.c:929 | no |
| LXP_IDENTITY_FROZEN | identity frozen | addr 0x1004 (DID identity) | LXP_ERR_IDENTITY_FROZEN = -206, include/layerx/lxp_result.h:54; emitted src/state/lxp_identity.c:106 | yes |
| LXP_UNAUTHORIZED_DEBIT | unauthorized debit | layerxauth 0x101C; layerxexchange 0x1015; layerxledger 0x101A; layerxpay 0x101B; layerxprograms 0x101D; shared cgo host (all kernel-backed precompiles); xweb 0x1019 (origin 2) | LXP_ERR_UNAUTHORIZED_DEBIT = -207, include/layerx/lxp_result.h:55; emitted src/modules/budget/lx_budget_close.c:14 | no |
| LXP_UNKNOWN_ACCOUNT_NAMESPACE | unknown account namespace | addr 0x1004 (DID identity); layerxexchange 0x1015; kernel governance checks inside every kernel-backed precompile; layerxledger 0x101A; layerxpay 0x101B; layerxprograms 0x101D; shared cgo host (all kernel-backed precompiles); xweb 0x1019 (origin 2) | LXP_ERR_UNKNOWN_ACCOUNT_NAMESPACE = -208, include/layerx/lxp_result.h:56; emitted src/modules/asset/lx_asset_state.c:45 | no |
| LXP_MALFORMED_GRANT | malformed grant | layerxauth 0x101C; layerxledger 0x101A; layerxpay 0x101B (pay.md) | LXP_ERR_MALFORMED_GRANT = -209, include/layerx/lxp_result.h:57; emitted src/ledger/lxp_grant_store.c:37 | no |
| LXP_UNKNOWN_AUTHORITY_KIND | unknown authority kind | layerxauth 0x101C; kernel governance checks inside every kernel-backed precompile; layerxledger 0x101A; shared cgo host (all kernel-backed precompiles) | LXP_ERR_UNKNOWN_AUTHORITY_KIND = -210, include/layerx/lxp_result.h:58; emitted src/modules/governance/lxp_governance.c:396 | no |
| LXP_GRANT_EXHAUSTED | grant exhausted | layerxauth 0x101C; layerxledger 0x101A; shared cgo host (all kernel-backed precompiles); xweb 0x1019 (origin 2) | LXP_ERR_GRANT_EXHAUSTED = -211, include/layerx/lxp_result.h:59; emitted src/state/lxp_authority.c:818 | no |
| LXP_STALE_REVOCATION | stale revocation | layerxauth 0x101C; layerxledger 0x101A; layerxpay 0x101B | LXP_ERR_STALE_REVOCATION = -212, include/layerx/lxp_result.h:60; emitted src/modules/budget/lx_budget_module.c:515 | no |
| LXP_CONTEXT_MISMATCH | context mismatch | layerxauth 0x101C; kernel governance checks inside every kernel-backed precompile; layerxledger 0x101A; layerxpay 0x101B; layerxprograms 0x101D; shared cgo host (all kernel-backed precompiles); xweb 0x1019 (origin 2) | LXP_ERR_CONTEXT_MISMATCH = -213, include/layerx/lxp_result.h:61; emitted src/modules/escrow/lx_escrow_dispute.c:76 | no |
| LXP_NO_PAYER_GRANT | no payer grant | layerxledger 0x101A | LXP_ERR_NO_PAYER_GRANT = -214, include/layerx/lxp_result.h:62; emitted src/modules/asset/lx_asset_transfer.c:67 | no |
| LXP_GRANT_SCOPE_VIOLATION | grant scope violation | layerxledger 0x101A; layerxpay 0x101B; shared cgo host (all kernel-backed precompiles) | LXP_ERR_GRANT_SCOPE_VIOLATION = -215, include/layerx/lxp_result.h:63; emitted src/modules/asset/lx_asset_transfer.c:75 | no |
| LXP_PURPOSE_MISMATCH | purpose mismatch | layerxledger 0x101A | LXP_ERR_PURPOSE_MISMATCH = -216, include/layerx/lxp_result.h:64; emitted src/ledger/lxp_receive.c:379 | no |
| LXP_INVOICE_ALREADY_SETTLED | invoice already settled | layerxledger 0x101A | LXP_ERR_INVOICE_ALREADY_SETTLED = -217, include/layerx/lxp_result.h:65; emitted src/ledger/lxp_receive.c:375 | no |
| LXP_GRANT_EXPIRED | grant expired | layerxledger 0x101A | LXP_ERR_GRANT_EXPIRED = -218, include/layerx/lxp_result.h:66; emitted src/ledger/lxp_receive.c:422 | no |
| LXP_GRANT_REVOKED | grant revoked | layerxledger 0x101A | LXP_ERR_GRANT_REVOKED = -219, include/layerx/lxp_result.h:67; emitted src/ledger/lxp_receive.c:420 | no |
| LXP_AUTH_THRESHOLD_UNMET | auth threshold unmet | layerxauth 0x101C | LXP_ERR_AUTH_THRESHOLD_UNMET = -220, include/layerx/lxp_result.h:68; emitted src/state/lxp_authority.c:391 | no |
| LXP_AUTH_DUPLICATE_SIGNER | auth duplicate signer | layerxauth 0x101C; xweb 0x1019 (origin 2) | LXP_ERR_AUTH_DUPLICATE_SIGNER = -221, include/layerx/lxp_result.h:69; emitted src/modules/web/lx_web_intake.c:234 | no |
| LXP_AUTH_NOT_MATURE | auth not mature | layerxauth 0x101C | LXP_ERR_AUTH_NOT_MATURE = -222, include/layerx/lxp_result.h:70; emitted src/state/lxp_authority.c:1136 | yes |
| LXP_SEQUENCE_GAP | sequence gap | addr 0x1004 (DID identity); layerxledger 0x101A; layerxpay 0x101B; layerxprograms 0x101D; shared cgo host (all kernel-backed precompiles); xweb 0x1019 (origin 2) | LXP_ERR_SEQUENCE_GAP = -300, include/layerx/lxp_result.h:71; emitted src/modules/budget/lx_budget_module.c:292 | no |
| LXP_SEQUENCE_REUSED | sequence reused | addr 0x1004 (DID identity); layerxauth 0x101C; layerxexchange 0x1015; kernel governance checks inside every kernel-backed precompile; layerxledger 0x101A; layerxpay 0x101B; layerxprograms 0x101D; shared cgo host (all kernel-backed precompiles); xweb 0x1019 (origin 2) | LXP_ERR_SEQUENCE_REUSED = -301, include/layerx/lxp_result.h:72; emitted src/modules/asset/lx_asset_custody.c:102 | no |
| LXP_IDEMPOTENT_REPLAY | idempotent replay | layerxledger 0x101A; layerxpay 0x101B; layerxprograms 0x101D; shared cgo host (all kernel-backed precompiles); xweb 0x1019 (origin 2) | LXP_ERR_IDEMPOTENT_REPLAY = -302, include/layerx/lxp_result.h:73; emitted src/modules/escrow/lx_escrow_state.c:483 | no |
| LXP_EXPIRED | expired | kernel governance checks inside every kernel-backed precompile; layerxledger 0x101A; layerxpay 0x101B; xweb 0x1019 (origin 2) | LXP_ERR_EXPIRED = -303, include/layerx/lxp_result.h:74; emitted src/modules/budget/lx_budget_spend.c:63 | no |
| LXP_NOT_YET_VALID | not yet valid | addr 0x1004 (DID identity); layerxauth 0x101C; layerxexchange 0x1015; kernel governance checks inside every kernel-backed precompile; layerxpay 0x101B; layerxprograms 0x101D | LXP_ERR_NOT_YET_VALID = -304, include/layerx/lxp_result.h:75; emitted src/modules/programs/blob_lifecycle.c:465 | yes |
| LXP_SEQUENCE_MISMATCH | sequence mismatch | kernel governance checks inside every kernel-backed precompile; layerxledger 0x101A; layerxprograms 0x101D; shared cgo host (all kernel-backed precompiles) | LXP_ERR_SEQUENCE_MISMATCH = -305, include/layerx/lxp_result.h:76; emitted src/state/lxp_snapshot.c:496 | no |
| LXP_CONDITION_UNMET | condition unmet | layerxledger 0x101A; layerxprograms 0x101D; shared cgo host (all kernel-backed precompiles) | LXP_ERR_CONDITION_UNMET = -306, include/layerx/lxp_result.h:77; emitted src/ledger/lxp_send.c:271 | no |
| LXP_SEQUENCE_EXHAUSTED | sequence exhausted | layerxledger 0x101A; shared cgo host (all kernel-backed precompiles); xweb 0x1019 (origin 2) | LXP_ERR_SEQUENCE_EXHAUSTED = -307, include/layerx/lxp_result.h:78; emitted src/modules/asset/lx_asset_registry.c:637 | no |
| LXP_INSUFFICIENT_BALANCE | insufficient balance | layerxexchange 0x1015; layerxledger 0x101A; layerxprograms 0x101D; shared cgo host (all kernel-backed precompiles); xweb 0x1019 (origin 2) | LXP_ERR_INSUFFICIENT_BALANCE = -400, include/layerx/lxp_result.h:79; emitted src/modules/perps/lx_perps_liquidate.c:122 | yes |
| LXP_ZERO_AMOUNT | zero amount | layerxexchange 0x1015; layerxledger 0x101A; layerxpay 0x101B; layerxprograms 0x101D; shared cgo host (all kernel-backed precompiles); xweb 0x1019 (origin 2) | LXP_ERR_ZERO_AMOUNT = -401, include/layerx/lxp_result.h:80; emitted src/modules/perps/lx_perps_liquidate.c:157 | no |
| LXP_ASSET_MISMATCH | asset mismatch | layerxauth 0x101C; layerxexchange 0x1015; layerxledger 0x101A; layerxpay 0x101B; layerxprograms 0x101D; shared cgo host (all kernel-backed precompiles); xweb 0x1019 (origin 2) | LXP_ERR_ASSET_MISMATCH = -402, include/layerx/lxp_result.h:81; emitted src/modules/asset/lx_asset_transfer.c:23 | no |
| LXP_ASSET_PAUSED | asset paused | layerxexchange 0x1015; layerxledger 0x101A; shared cgo host (all kernel-backed precompiles); xweb 0x1019 (origin 2) | LXP_ERR_ASSET_PAUSED = -403, include/layerx/lxp_result.h:82; emitted src/modules/asset/lx_asset_transfer.c:19 | yes |
| LXP_ACCOUNT_FROZEN | account frozen | layerxledger 0x101A; layerxprograms 0x101D; xweb 0x1019 (origin 2) | LXP_ERR_ACCOUNT_FROZEN = -404, include/layerx/lxp_result.h:83; emitted src/modules/asset/lx_asset_transfer.c:25 | yes |
| LXP_CONSERVATION | conservation | layerxexchange 0x1015; layerxledger 0x101A; layerxpay 0x101B; xweb 0x1019 (origin 2) | LXP_ERR_CONSERVATION = -405, include/layerx/lxp_result.h:84; emitted src/ledger/lxp_apply.c:412 | no |
| LXP_TOO_MANY_LEGS | too many legs | layerxexchange 0x1015; layerxledger 0x101A; layerxprograms 0x101D; shared cgo host (all kernel-backed precompiles); xweb 0x1019 (origin 2) | LXP_ERR_TOO_MANY_LEGS = -406, include/layerx/lxp_result.h:85; emitted src/modules/perps/lx_perps_insurance.c:160 | no |
| LXP_ACCOUNT_NOT_EMPTY | account not empty | addr 0x1004 (DID identity); layerxexchange 0x1015; layerxledger 0x101A | LXP_ERR_ACCOUNT_NOT_EMPTY = -407, include/layerx/lxp_result.h:86; emitted src/modules/perps/lx_perps_dispatch.c:1359 | no |
| LXP_BALANCE_BYPASS | balance bypass | layerxledger 0x101A; shared cgo host (all kernel-backed precompiles); xweb 0x1019 (origin 2) | LXP_ERR_BALANCE_BYPASS = -408, include/layerx/lxp_result.h:87; emitted src/modules/asset/lx_asset_transfer.c:12 | no |
| LXP_ACCOUNT_ID_MISMATCH | account id mismatch | layerxledger 0x101A; layerxprograms 0x101D; shared cgo host (all kernel-backed precompiles) | LXP_ERR_ACCOUNT_ID_MISMATCH = -409, include/layerx/lxp_result.h:88; emitted src/ledger/lx_account_registry.c:564 | no |
| LXP_CLIENT_SUPPLIED_BALANCE | client supplied balance | layerxledger 0x101A; xweb 0x1019 (origin 2) | LXP_ERR_CLIENT_SUPPLIED_BALANCE = -410, include/layerx/lxp_result.h:89; emitted src/ledger/lxp_apply.c:265 | no |
| LXP_ASSET_ALREADY_REGISTERED | asset already registered | layerxledger 0x101A; shared cgo host (all kernel-backed precompiles) | LXP_ERR_ASSET_ALREADY_REGISTERED = -411, include/layerx/lxp_result.h:90; emitted src/modules/asset/lx_asset_registry.c:644 | no |
| LXP_INVALID_AMOUNT | invalid amount | layerxledger 0x101A; layerxpay 0x101B | LXP_ERR_INVALID_AMOUNT = -412, include/layerx/lxp_result.h:91; emitted src/modules/asset/lx_asset_decode.c:102 | no |
| LXP_DEPOSIT_PROOF_NOT_FINAL | deposit proof not final | layerxledger 0x101A; shared cgo host (all kernel-backed precompiles) | LXP_ERR_DEPOSIT_PROOF_NOT_FINAL = -413, include/layerx/lxp_result.h:92; emitted src/modules/asset/lx_asset_custody.c:98 | no |
| LXP_DEPOSIT_ALREADY_CREDITED | deposit already credited | layerxledger 0x101A; shared cgo host (all kernel-backed precompiles) | LXP_ERR_DEPOSIT_ALREADY_CREDITED = -414, include/layerx/lxp_result.h:93; emitted src/modules/asset/lx_asset_custody.c:231 | no |
| LXP_WITHDRAWAL_ALREADY_SETTLED | withdrawal already settled | not emitted after the merge: kernel WITHDRAW 0x00010009 is not registered and 0x101A has no withdraw (ledger-auth.md 1 A); only lx_asset_withdraw_request (src/modules/asset/lx_asset_custody.c:330) and lx_asset_withdraw_settle (:387) emit it | LXP_ERR_WITHDRAWAL_ALREADY_SETTLED = -415, include/layerx/lxp_result.h:94; emitted src/modules/asset/lx_asset_custody.c:351 | no |
| LXP_CHALLENGE_WINDOW_OPEN | challenge window open | not emitted after the merge: only retired kernel paths (bridge credit, network, replica, DA, guarantor) use it | LXP_ERR_CHALLENGE_WINDOW_OPEN = -416, include/layerx/lxp_result.h:95 | no |
| LXP_WITHDRAWAL_CANCELLED | withdrawal cancelled | not emitted after the merge: only retired kernel paths (bridge credit, network, replica, DA, guarantor) use it | LXP_ERR_WITHDRAWAL_CANCELLED = -417, include/layerx/lxp_result.h:96 | no |
| LXP_WITHDRAWAL_ASSET_MISMATCH | withdrawal asset mismatch | not emitted after the merge: kernel WITHDRAW 0x00010009 is not registered and 0x101A has no withdraw (ledger-auth.md 1 A); only lx_asset_withdraw_request (src/modules/asset/lx_asset_custody.c:330) and lx_asset_withdraw_settle (:387) emit it | LXP_ERR_WITHDRAWAL_ASSET_MISMATCH = -418, include/layerx/lxp_result.h:97; emitted src/modules/asset/lx_asset_custody.c:416 | no |
| LXP_OVERFLOW | overflow | addr 0x1004 (DID identity); layerxexchange 0x1015; kernel governance checks inside every kernel-backed precompile; layerxledger 0x101A; layerxpay 0x101B; layerxprograms 0x101D; shared cgo host (all kernel-backed precompiles); xweb 0x1019 (origin 2) | LXP_ERR_OVERFLOW = -500, include/layerx/lxp_result.h:98; emitted src/modules/asset/lx_asset_transfer.c:32 | no |
| LXP_UNDERFLOW | underflow | layerxledger 0x101A; shared cgo host (all kernel-backed precompiles); xweb 0x1019 (origin 2) | LXP_ERR_UNDERFLOW = -501, include/layerx/lxp_result.h:99; emitted src/modules/asset/lx_asset_transfer.c:29 | no |
| LXP_DIV_ZERO | div zero | shared cgo host (all kernel-backed precompiles) | LXP_ERR_DIV_ZERO = -502, include/layerx/lxp_result.h:100; emitted src/protocol/lxp_u256.c:72 | no |
| LXP_PRECISION | precision | not emitted after the merge: only retired kernel paths (bridge credit, network, replica, DA, guarantor) use it; the xweb wei remainder refusal is LXP_PRECOMPILE_WEI_REMAINDER | LXP_ERR_PRECISION = -503, include/layerx/lxp_result.h:101 | no |
| LXP_FEE_LIMIT | fee limit | layerxprograms 0x101D; shared cgo host (all kernel-backed precompiles) | LXP_ERR_FEE_LIMIT = -600, include/layerx/lxp_result.h:102; emitted src/protocol/lxp_fee_policy.c:53 | no |
| LXP_GAS_EXHAUSTED | gas exhausted | layerxprograms 0x101D; shared cgo host (all kernel-backed precompiles) | LXP_ERR_GAS_EXHAUSTED = -601, include/layerx/lxp_result.h:103; emitted src/protocol/lxp_meter.c:37 | yes |
| LXP_FEE_UNPAYABLE | fee unpayable | layerxprograms 0x101D; shared cgo host (all kernel-backed precompiles) | LXP_ERR_FEE_UNPAYABLE = -602, include/layerx/lxp_result.h:104; emitted src/modules/programs/call.c:1903 | yes |
| LXP_ESCROW_STATE | escrow state | layerxpay 0x101B | LXP_ERR_ESCROW_STATE = -700, include/layerx/lxp_result.h:105; emitted src/modules/escrow/lx_escrow_capture.c:14 | no |
| LXP_BUDGET_PERIOD_CAP | budget period cap | layerxpay 0x101B | LXP_ERR_BUDGET_PERIOD_CAP = -701, include/layerx/lxp_result.h:106; emitted src/modules/budget/lx_budget_module.c:418 | yes |
| LXP_STREAM_UNDERFUNDED | stream underfunded | not emitted after the merge: only retired kernel paths (bridge credit, network, replica, DA, guarantor) use it | LXP_ERR_STREAM_UNDERFUNDED = -702, include/layerx/lxp_result.h:107 | no |
| LXP_MARKET_HALTED | market halted | layerxexchange 0x1015; shared cgo host (all kernel-backed precompiles) | LXP_ERR_MARKET_HALTED = -703, include/layerx/lxp_result.h:108; emitted src/modules/perps/lx_oracle_halt.c:24 | yes |
| LXP_ORACLE_STALE | oracle stale | layerxexchange 0x1015 | LXP_ERR_ORACLE_STALE = -704, include/layerx/lxp_result.h:109; emitted src/modules/perps/lx_oracle_intake.c:158 | yes |
| LXP_MARGIN_INSUFFICIENT | margin insufficient | layerxexchange 0x1015 | LXP_ERR_MARGIN_INSUFFICIENT = -705, include/layerx/lxp_result.h:110; emitted src/modules/perps/lx_perps_position.c:270 | yes |
| LXP_AGREEMENT_STATE | agreement state | layerxexchange 0x1015; layerxpay 0x101B | LXP_ERR_AGREEMENT_STATE = -706, include/layerx/lxp_result.h:111; emitted src/modules/service/lx_service_agreement.c:79 | no |
| LXP_CAPTURE_EXCEEDS_HOLD | capture exceeds hold | layerxpay 0x101B | LXP_ERR_CAPTURE_EXCEEDS_HOLD = -707, include/layerx/lxp_result.h:112; emitted src/modules/escrow/lx_escrow_capture.c:88 | no |
| LXP_UNAUTHORIZED_CAPTURE | unauthorized capture | layerxpay 0x101B | LXP_ERR_UNAUTHORIZED_CAPTURE = -708, include/layerx/lxp_result.h:113; emitted src/modules/escrow/lx_escrow_capture.c:78 | no |
| LXP_HOLD_EXPIRED | hold expired | layerxpay 0x101B | LXP_ERR_HOLD_EXPIRED = -709, include/layerx/lxp_result.h:114; emitted src/modules/escrow/lx_escrow_capture.c:67 | no |
| LXP_UNAUTHORIZED_ESCROW_SPEND | unauthorized escrow spend | layerxpay 0x101B | LXP_ERR_UNAUTHORIZED_ESCROW_SPEND = -710, include/layerx/lxp_result.h:115; emitted src/modules/escrow/lx_escrow_invariants.c:20 | no |
| LXP_DISPUTE_WINDOW_CLOSED | dispute window closed | layerxpay 0x101B | LXP_ERR_DISPUTE_WINDOW_CLOSED = -711, include/layerx/lxp_result.h:116; emitted src/modules/service/lx_service_dispute.c:41 | no |
| LXP_HOLD_DISPUTED | hold disputed | layerxpay 0x101B | LXP_ERR_HOLD_DISPUTED = -712, include/layerx/lxp_result.h:117; emitted src/modules/escrow/lx_escrow_timeout.c:47 | no |
| LXP_BUDGET_ALLOWANCE_EXCEEDED | budget allowance exceeded | layerxpay 0x101B | LXP_ERR_BUDGET_ALLOWANCE_EXCEEDED = -713, include/layerx/lxp_result.h:118; emitted src/modules/budget/lx_budget_spend.c:28 | no |
| LXP_INSUFFICIENT_BUDGET_FUNDS | insufficient budget funds | layerxpay 0x101B | LXP_ERR_INSUFFICIENT_BUDGET_FUNDS = -714, include/layerx/lxp_result.h:119; emitted src/modules/budget/lx_budget_close.c:72 | yes |
| LXP_UNAUTHORIZED_DELEGATE | unauthorized delegate | layerxpay 0x101B | LXP_ERR_UNAUTHORIZED_DELEGATE = -715, include/layerx/lxp_result.h:120; emitted src/modules/budget/lx_budget_delegate.c:61 | no |
| LXP_BUDGET_REVOKED | budget revoked | layerxpay 0x101B | LXP_ERR_BUDGET_REVOKED = -716, include/layerx/lxp_result.h:121; emitted src/modules/budget/lx_budget_close.c:93 | no |
| LXP_NON_MONOTONIC_TIME | non monotonic time | kernel governance checks inside every kernel-backed precompile; layerxpay 0x101B | LXP_ERR_NON_MONOTONIC_TIME = -717, include/layerx/lxp_result.h:122; emitted src/modules/stream/lx_stream_open.c:92 | no |
| LXP_ACCRUAL_OVERFLOW | accrual overflow | layerxpay 0x101B | LXP_ERR_ACCRUAL_OVERFLOW = -718, include/layerx/lxp_result.h:123; emitted src/modules/stream/lx_stream_accrue.c:58 | no |
| LXP_UNAUTHORIZED_METER | unauthorized meter | layerxpay 0x101B | LXP_ERR_UNAUTHORIZED_METER = -719, include/layerx/lxp_result.h:124; emitted src/modules/stream/lx_stream_meter.c:39 | no |
| LXP_METER_REGRESSION | meter regression | layerxpay 0x101B | LXP_ERR_METER_REGRESSION = -720, include/layerx/lxp_result.h:125; emitted src/modules/service/lx_service_commit.c:143 | no |
| LXP_STREAM_CLOSED | stream closed | layerxpay 0x101B | LXP_ERR_STREAM_CLOSED = -721, include/layerx/lxp_result.h:126; emitted src/modules/stream/lx_stream_open.c:135 | no |
| LXP_MODULE_MAY_NOT_WRITE_BALANCE | module may not write balance | layerxexchange 0x1015; layerxpay 0x101B | LXP_ERR_MODULE_MAY_NOT_WRITE_BALANCE = -722, include/layerx/lxp_result.h:127; emitted src/modules/service/lx_service_acceptance.c:17 | no |
| LXP_OFFER_UNAVAILABLE | offer unavailable | layerxpay 0x101B | LXP_ERR_OFFER_UNAVAILABLE = -723, include/layerx/lxp_result.h:128; emitted src/modules/service/lx_service_offer.c:69 | no |
| LXP_TERMS_MISMATCH | terms mismatch | layerxpay 0x101B | LXP_ERR_TERMS_MISMATCH = -724, include/layerx/lxp_result.h:129; emitted src/modules/service/lx_service_agreement.c:34 | no |
| LXP_INVALID_ATTESTATION | invalid attestation | layerxpay 0x101B; xweb 0x1019 (origin 2) | LXP_ERR_INVALID_ATTESTATION = -725, include/layerx/lxp_result.h:130; emitted src/modules/service/lx_service_attest.c:127 | no |
| LXP_DELIVERABLE_MISMATCH | deliverable mismatch | layerxpay 0x101B | LXP_ERR_DELIVERABLE_MISMATCH = -726, include/layerx/lxp_result.h:131; emitted src/modules/service/lx_service_deliver.c:25 | no |
| LXP_DELIVERY_DEADLINE_PASSED | delivery deadline passed | layerxpay 0x101B | LXP_ERR_DELIVERY_DEADLINE_PASSED = -727, include/layerx/lxp_result.h:132; emitted src/modules/service/lx_service_deliver.c:87 | no |
| LXP_UNAUTHORIZED_DISPUTANT | unauthorized disputant | layerxpay 0x101B | LXP_ERR_UNAUTHORIZED_DISPUTANT = -728, include/layerx/lxp_result.h:133; emitted src/modules/service/lx_service_dispute.c:39 | no |
| LXP_UNAUTHORIZED_ORACLE | unauthorized oracle | layerxexchange 0x1015 | LXP_ERR_UNAUTHORIZED_ORACLE = -729, include/layerx/lxp_result.h:134; emitted src/modules/perps/lx_oracle_intake.c:83 | no |
| LXP_ORACLE_SEQUENCE | oracle sequence | layerxexchange 0x1015 | LXP_ERR_ORACLE_SEQUENCE = -730, include/layerx/lxp_result.h:135; emitted src/modules/perps/lx_perps_dispatch.c:530 | no |
| LXP_ORACLE_BOUNDS | oracle bounds | layerxexchange 0x1015 | LXP_ERR_ORACLE_BOUNDS = -731, include/layerx/lxp_result.h:136; emitted src/modules/perps/lx_oracle_checks.c:25 | no |
| LXP_ORACLE_DEVIATION | oracle deviation | layerxexchange 0x1015 | LXP_ERR_ORACLE_DEVIATION = -732, include/layerx/lxp_result.h:137; emitted src/modules/perps/lx_oracle_checks.c:48 | no |
| LXP_MARKET_ALREADY_EXISTS | market already exists | layerxexchange 0x1015 | LXP_ERR_MARKET_ALREADY_EXISTS = -733, include/layerx/lxp_result.h:138; emitted src/modules/perps/lx_perps_dispatch.c:606 | no |
| LXP_PARAMETER_BOUNDS | parameter bounds | layerxexchange 0x1015; kernel governance checks inside every kernel-backed precompile; layerxpay 0x101B; layerxprograms 0x101D; shared cgo host (all kernel-backed precompiles); xweb 0x1019 (origin 2) | LXP_ERR_PARAMETER_BOUNDS = -734, include/layerx/lxp_result.h:139; emitted src/modules/service/lx_service_payload.c:159 | no |
| LXP_PAUSED_SCOPE | paused scope | every kernel-backed precompile through lxp_pause_scope_check (src/modules/governance/lxp_emergency.c:139-160); layerxpay 0x101B writes outside the exit set (pay.md F); layerxexchange 0x1015 Go Params.Paused, exit paths cancelOrder, cancelSpotOrder, closePosition, withdrawMargin exempt (exchange-launchpad.md F) | LXP_ERR_PAUSED_SCOPE = -735, include/layerx/lxp_result.h:140; emitted src/modules/governance/lxp_emergency.c:98 | yes |
| LXP_PROGRAM_REFUSED | program refused | layerxprograms 0x101D; shared cgo host (all kernel-backed precompiles) | LXP_ERR_PROGRAM_REFUSED = -736, include/layerx/lxp_result.h:141; emitted src/modules/programs/call.c:648 | no |
| LXP_ARBITER_REQUIRED | arbiter required | layerxpay 0x101B serviceDisputeOpen, serviceDisputeResolve (linked escrow has no arbiter) | LXP_ERR_ARBITER_REQUIRED = -737, include/layerx/lxp_result.h:142; pay.md A.6 | no |
| LXP_UNAUTHORIZED_ARBITER | unauthorized arbiter | layerxpay 0x101B serviceDisputeResolve (principal is not the linked escrow arbiter) | LXP_ERR_UNAUTHORIZED_ARBITER = -738, include/layerx/lxp_result.h:143; pay.md A.6 | no |
| LXP_ESCROW_LINK_MISMATCH | escrow link mismatch | layerxpay 0x101B serviceDisputeOpen, serviceDisputeResolve (escrow `owner`, beneficiary or asset differ from the agreement) | LXP_ERR_ESCROW_LINK_MISMATCH = -739, include/layerx/lxp_result.h:144; pay.md A.6 | no |
| LXP_REPLAY_BOUND | replay bound | layerxprograms 0x101D profiled CALL capture | LXP_ERR_REPLAY_BOUND = -740, include/layerx/lxp_result.h:150; programs/crates/layerx-programs-runtime/src/native_result.rs | no |
| LXP_ENGINE_FAULT | engine fault | layerxprograms 0x101D CALL executor engine fault | LXP_ERR_ENGINE_FAULT = -741, include/layerx/lxp_result.h:151; programs/crates/layerx-programs-runtime/src/native_result.rs | no |
| LXP_PAY_UNBOUND_CALLER | unbound caller | layerxpay 0x101B | PayRevertCode(ErrUnboundCaller), modules/layerxpay/types/errors.go:17-18; precompiles/layerxpay/errors.go:20-21 | no |
| LXP_PAY_KERNEL_GATE_MISMATCH | kernel gate mismatch | layerxpay 0x101B | PayRevertCode(ErrKernelGateMismatch), modules/layerxpay/types/errors.go:21-22; precompiles/layerxpay/errors.go:20-21 | no |
| LXP_BATCH_GAP | batch gap | layerxprograms 0x101D; shared cgo host (all kernel-backed precompiles) | LXP_ERR_BATCH_GAP = -800, include/layerx/lxp_result.h:145; emitted src/modules/programs/occupancy_evidence.c:673 | no |
| LXP_ROOT_MISMATCH | root mismatch | layerxprograms 0x101D; shared cgo host (all kernel-backed precompiles) | LXP_ERR_ROOT_MISMATCH = -801, include/layerx/lxp_result.h:146; emitted src/modules/programs/fees.c:606 | no |
| LXP_TIMESTAMP_REGRESSION | timestamp regression | layerxexchange 0x1015; layerxpay 0x101B; shared cgo host (all kernel-backed precompiles) | LXP_ERR_TIMESTAMP_REGRESSION = -802, include/layerx/lxp_result.h:147; emitted src/modules/perps/lx_oracle_intake.c:162 | no |
| LXP_ATTESTATION_THRESHOLD | attestation threshold | layerxprograms 0x101D; xweb 0x1019 (origin 2) | LXP_ERR_ATTESTATION_THRESHOLD = -803, include/layerx/lxp_result.h:148; emitted src/modules/programs/blob_lifecycle.c:486 | no |
| LXP_DA_MISSING | da missing | layerxpay 0x101B | layerxpay service delivery (precompiles/common/lxp_errors.go:164); the kernel value LXP_ERR_DA_MISSING was retired from the Go registry by 6.97.3 | no |
| LXP_EQUIVOCATION | equivocation | not emitted after the merge: only retired kernel paths (bridge credit, network, replica, DA, guarantor) use it | LXP_ERR_EQUIVOCATION = -805, include/layerx/lxp_result.h:150 | no |
| LXP_LOG_CORRUPT | log corrupt | layerxprograms 0x101D; shared cgo host (all kernel-backed precompiles) | LXP_ERR_LOG_CORRUPT = -900, include/layerx/lxp_result.h:151; emitted src/state/lxp_journal.c:1197 | no |
| LXP_LOG_TRUNCATED | log truncated | shared cgo host (all kernel-backed precompiles) | LXP_ERR_LOG_TRUNCATED = -901, include/layerx/lxp_result.h:152; emitted src/protocol/lxp_kernel.c:8671 | no |
| LXP_SNAPSHOT_MISMATCH | snapshot mismatch | shared cgo host (all kernel-backed precompiles) | LXP_ERR_SNAPSHOT_MISMATCH = -902, include/layerx/lxp_result.h:153; emitted src/state/lxp_snapshot.c:154 | no |
| LXP_PROJECTION_STALE | projection stale | layerxprograms 0x101D; shared cgo host (all kernel-backed precompiles) | LXP_ERR_PROJECTION_STALE = -903, include/layerx/lxp_result.h:154; emitted src/protocol/lxp_kernel.c:8085 | yes |
| LXP_IO | io | layerxauth 0x101C; layerxledger 0x101A; layerxprograms 0x101D; shared cgo host (all kernel-backed precompiles) | LXP_ERR_IO = -904, include/layerx/lxp_result.h:155; emitted src/ledger/lxp_journal.c:248 | yes |
| LXP_ARENA_EXHAUSTED | arena exhausted | addr 0x1004 (DID identity); layerxexchange 0x1015; kernel governance checks inside every kernel-backed precompile; layerxledger 0x101A; layerxpay 0x101B; layerxprograms 0x101D; shared cgo host (all kernel-backed precompiles); xweb 0x1019 (origin 2) | LXP_ERR_ARENA_EXHAUSTED = -905, include/layerx/lxp_result.h:156; emitted src/modules/perps/lx_perps_insurance.c:83 | no |
| LXP_SNAPSHOT_BLOBS_MISSING | snapshot blobs missing | shared cgo host (all kernel-backed precompiles) | LXP_ERR_SNAPSHOT_BLOBS_MISSING = -906, include/layerx/lxp_result.h:157; emitted src/state/lxp_snapshot.c:951 | no |
| LXP_FATAL_INVARIANT | fatal invariant | not a revert: fatal, the node halts (src/protocol/lxp_result.c:55 lxp_result_is_fatal) | LXP_FATAL_INVARIANT = -1001, include/layerx/lxp_result.h:158; emitted src/modules/budget/lx_budget_close.c:77 | no |
| LXP_FATAL_REPLAY_DIVERGENCE | fatal replay divergence | not a revert: fatal, the node halts (src/protocol/lxp_result.c:55 lxp_result_is_fatal) | LXP_FATAL_REPLAY_DIVERGENCE = -1002, include/layerx/lxp_result.h:159; emitted src/modules/programs/fees.c:625 | no |
| LXP_FATAL_SUPPLY_MISMATCH | fatal supply mismatch | not a revert: fatal, the node halts (src/protocol/lxp_result.c:55 lxp_result_is_fatal) | LXP_FATAL_SUPPLY_MISMATCH = -1003, include/layerx/lxp_result.h:160; emitted src/modules/asset/lx_asset_reserve.c:10 | no |
| LXP_UNKNOWN | unknown | shared cgo host (any value outside the enum) | lxp_result_name default branch, src/protocol/lxp_result.c:10 | no |
| LXP_XWEB_UNAUTHORIZED | unauthorized | xweb 0x1019 | ErrUnauthorized (code 2), modules/xweb/types/errors.go:6 | no |
| LXP_XWEB_INVALID_GENESIS | invalid genesis | xweb 0x1019 | ErrInvalidGenesis (code 3), modules/xweb/types/errors.go:7 | no |
| LXP_XWEB_INVALID_PARAMS | invalid params | xweb 0x1019 | ErrInvalidParams (code 4), modules/xweb/types/errors.go:8 | no |
| LXP_XWEB_INVALID_ATTESTORS | invalid attestor set | xweb 0x1019 | ErrInvalidAttestors (code 5), modules/xweb/types/errors.go:9 | no |
| LXP_XWEB_INVALID_THRESHOLD | threshold is not a majority of the attestor set | xweb 0x1019 | ErrInvalidThreshold (code 6), modules/xweb/types/errors.go:10 | no |
| LXP_XWEB_PAUSED | xweb paused | xweb 0x1019 | ErrPaused (code 7), modules/xweb/types/errors.go:11 | yes |
| LXP_XWEB_UNKNOWN_KIND | unknown request kind | xweb 0x1019 | ErrUnknownKind (code 8), modules/xweb/types/errors.go:12 | no |
| LXP_XWEB_PAYLOAD_SIZE | payload empty or over the payload cap | xweb 0x1019 | ErrPayloadSize (code 9), modules/xweb/types/errors.go:13 | no |
| LXP_XWEB_CALLBACK_GAS | callback gas zero or over the callback cap | xweb 0x1019 | ErrCallbackGas (code 10), modules/xweb/types/errors.go:14 | no |
| LXP_XWEB_WRONG_FEE | paid amount is not the current fee | xweb 0x1019 | ErrWrongFee (code 11), modules/xweb/types/errors.go:15 | no |
| LXP_XWEB_UNKNOWN_REQUEST | request not found | xweb 0x1019; `refund` of an origin-2 id, never keyed in the refund store (web.md A3, F7) | ErrUnknownRequest (code 12), modules/xweb/types/errors.go:16 | no |
| LXP_XWEB_ALREADY_FULFILLED | request already fulfilled | xweb 0x1019 | ErrAlreadyFulfilled (code 13), modules/xweb/types/errors.go:17 | no |
| LXP_XWEB_REFUNDED | request already refunded | xweb 0x1019 | ErrRefunded (code 14), modules/xweb/types/errors.go:18 | no |
| LXP_XWEB_RESPONSE_TOO_LARGE | response over the stored response bound | xweb 0x1019 | ErrResponseTooLarge (code 15), modules/xweb/types/errors.go:19 | no |
| LXP_XWEB_INVALID_LENGTH | full length shorter than the response | xweb 0x1019 | ErrInvalidLength (code 16), modules/xweb/types/errors.go:20 | no |
| LXP_XWEB_BAD_SIGNATURE | invalid attestor signature | xweb 0x1019 | ErrBadSignature (code 17), modules/xweb/types/errors.go:21 | no |
| LXP_XWEB_BELOW_THRESHOLD | attestor signatures below threshold | xweb 0x1019 | ErrBelowThreshold (code 18), modules/xweb/types/errors.go:22 | no |
| LXP_XWEB_NOT_EXPIRED | request not yet past its timeout | xweb 0x1019 | ErrNotExpired (code 19), modules/xweb/types/errors.go:23 | yes |
| LXP_XWEB_UNKNOWN_ATTESTOR | attestor not registered | xweb 0x1019 | ErrUnknownAttestor (code 20), modules/xweb/types/errors.go:24 | no |
| LXP_XWEB_CALLBACK_RECORDED | callback outcome already recorded | xweb 0x1019 | ErrCallbackRecorded (code 21), modules/xweb/types/errors.go:25 | no |
| LXP_XWEB_INVALID_REQUEST | invalid request | xweb 0x1019 | ErrInvalidRequest (code 22), modules/xweb/types/errors.go:26 | no |
| LXP_XWEB_INVALID_API | invalid api payload | xweb 0x1019 | ErrInvalidApi (code 23), modules/xweb/types/errors.go:27 | no |
| LXP_XWEB_INVALID_ENVELOPE | invalid credential envelope | xweb 0x1019 | ErrInvalidEnvelope (code 24), modules/xweb/types/errors.go:28 | no |
| LXP_XWEB_INVALID_LEVEL | invalid attestation level | xweb 0x1019 | ErrInvalidLevel (code 25), modules/xweb/types/errors.go:29 | no |
| LXP_XWEB_INVALID_PAYLOAD | invalid storage receipt payload | xweb 0x1019 | ErrInvalidPayload (code 26), modules/xweb/types/errors.go:91 | no |
| LXP_XWEB_DELEGATECALL | cannot delegatecall xweb | xweb 0x1019 | precompile site precompiles/xweb/xweb.go:290 | no |
| LXP_XWEB_STATICCALL | cannot call xweb <method> from staticcall | xweb 0x1019 | precompile site precompiles/xweb/xweb.go:293 | no |
| LXP_XWEB_KEEPER_MISSING | xweb: the xweb keeper is not wired into this application | xweb 0x1019 | precompile site precompiles/xweb/xweb.go:296 | no |
| LXP_XWEB_FULFIL_EVM_ONLY | xweb: fulfil runs only through the EVM, which meters its callback | xweb 0x1019 | precompile site precompiles/xweb/xweb.go:243 | no |
| LXP_XWEB_CALLBACK_BUDGET | xweb: fulfil of request <id> needs <n> gas for its callback and its record, <m> left | xweb 0x1019 | precompile site precompiles/xweb/xweb.go:416 (text :417-418) | no |
| LXP_BANK_INSUFFICIENT_FUNDS | insufficient funds | xweb 0x1019 `request`, `fulfil`, `refund` bank legs; any precompile moving coins through bank SendCoins | ErrInsufficientFunds (root codespace "sdk", code 5), sdk/types/errors/errors.go:32, surfaced by sdk/x/bank/keeper/send.go:238; xweb legs modules/xweb/keeper/request.go:53, fulfil.go:148, refund.go:35 (web.md RQ, FU, RF) | yes |
| LXP_BANK_STATICCALL | cannot call <method> from staticcall | bank 0x1001: send, sendNative | precompile site precompiles/bank/bank.go:218,262,338 | no |
| LXP_BANK_DELEGATECALL | cannot delegatecall <method> | bank 0x1001: sendNative | precompile site precompiles/bank/bank.go:341 | no |
| LXP_BANK_ZERO_VALUE | set `value` field to non-zero to send | bank 0x1001: sendNative | precompile site precompiles/bank/bank.go:347 | no |
| LXP_BRIDGE_UNAUTHORIZED | unauthorized | layerxbridge 0x1016 | ErrUnauthorized (code 2), modules/layerxbridge/types/errors.go:6 | no |
| LXP_BRIDGE_UNKNOWN_CHAIN | chain not registered | layerxbridge 0x1016 | ErrUnknownChain (code 5), modules/layerxbridge/types/errors.go:9 | no |
| LXP_BRIDGE_CHAIN_DISABLED | chain disabled | layerxbridge 0x1016 | ErrChainDisabled (code 6), modules/layerxbridge/types/errors.go:10 | yes |
| LXP_BRIDGE_UNKNOWN_ASSET | asset not registered | layerxbridge 0x1016 | ErrUnknownAsset (code 9), modules/layerxbridge/types/errors.go:13 | no |
| LXP_BRIDGE_PAUSED | bridge paused | layerxbridge 0x1016 | ErrPaused (code 10), modules/layerxbridge/types/errors.go:14 | yes |
| LXP_BRIDGE_NULLIFIED | remote event already bridged | layerxbridge 0x1016 | ErrNullified (code 11), modules/layerxbridge/types/errors.go:15 | no |
| LXP_BRIDGE_CAP_EXCEEDED | amount exceeds the asset cap | layerxbridge 0x1016 | ErrCapExceeded (code 12), modules/layerxbridge/types/errors.go:16 | yes |
| LXP_BRIDGE_BAD_SIGNATURE | invalid attestor signature | layerxbridge 0x1016 | ErrBadSignature (code 13), modules/layerxbridge/types/errors.go:17 | no |
| LXP_BRIDGE_BELOW_THRESHOLD | attestor signatures below threshold | layerxbridge 0x1016 | ErrBelowThreshold (code 14), modules/layerxbridge/types/errors.go:18 | no |
| LXP_BRIDGE_VAULT_MISMATCH | vault is not the registered vault | layerxbridge 0x1016 | ErrVaultMismatch (code 15), modules/layerxbridge/types/errors.go:19 | no |
| LXP_BRIDGE_INVALID_REQUEST | invalid bridge request | layerxbridge 0x1016 | ErrInvalidRequest (code 16), modules/layerxbridge/types/errors.go:20 | no |
| LXP_LAUNCHPAD_OVERFLOW | uint256 overflow | launchpad 0x1017 | ErrOverflow (code 3), modules/launchpad/types/errors.go:9 | no |
| LXP_LAUNCHPAD_DIVISION_BY_ZERO | division by zero | launchpad 0x1017 | ErrDivisionByZero (code 4), modules/launchpad/types/errors.go:10 | no |
| LXP_LAUNCHPAD_INSUFFICIENT_INPUT | insufficient input | launchpad 0x1017 | ErrInsufficientInput (code 5), modules/launchpad/types/errors.go:11 | no |
| LXP_LAUNCHPAD_INSUFFICIENT_LIQUIDITY | insufficient liquidity | launchpad 0x1017 | ErrInsufficientLiquidity (code 6), modules/launchpad/types/errors.go:12 | yes |
| LXP_LAUNCHPAD_VIRTUAL_FLOOR_BREACHED | virtual floor breached | launchpad 0x1017 | ErrVirtualFloorBreached (code 7), modules/launchpad/types/errors.go:13 | no |
| LXP_LAUNCHPAD_SLIPPAGE_EXCEEDED | slippage exceeded | launchpad 0x1017 | ErrSlippageExceeded (code 8), modules/launchpad/types/errors.go:14 | yes |
| LXP_LAUNCHPAD_DEADLINE_EXPIRED | deadline expired | launchpad 0x1017 | ErrDeadlineExpired (code 9), modules/launchpad/types/errors.go:15 | no |
| LXP_LAUNCHPAD_ZERO_ADDRESS | zero address | launchpad 0x1017 | ErrZeroAddress (code 10), modules/launchpad/types/errors.go:16 | no |
| LXP_LAUNCHPAD_ZERO_AMOUNT | zero amount | launchpad 0x1017 | ErrZeroAmount (code 11), modules/launchpad/types/errors.go:17 | no |
| LXP_LAUNCHPAD_UNKNOWN_MARKET | unknown market | launchpad 0x1017 | ErrUnknownMarket (code 12), modules/launchpad/types/errors.go:18 | no |
| LXP_LAUNCHPAD_PAUSED | market is paused | launchpad 0x1017 | ErrPaused (code 13), modules/launchpad/types/errors.go:19 | yes |
| LXP_LAUNCHPAD_NOT_PAUSED | market is not paused | launchpad 0x1017 | ErrNotPaused (code 14), modules/launchpad/types/errors.go:20 | yes |
| LXP_LAUNCHPAD_NOT_GUARDIAN | caller is not the market guardian | launchpad 0x1017 | ErrNotGuardian (code 15), modules/launchpad/types/errors.go:21 | no |
| LXP_LAUNCHPAD_NOT_FEE_RIGHTS_HOLDER | caller does not hold the market's fee rights | launchpad 0x1017 | ErrNotFeeRightsHolder (code 16), modules/launchpad/types/errors.go:22 | no |
| LXP_LAUNCHPAD_WRONG_STRATEGY | wrong fee strategy | launchpad 0x1017 | ErrWrongStrategy (code 17), modules/launchpad/types/errors.go:23 | no |
| LXP_LAUNCHPAD_INVALID_STRATEGY | invalid fee strategy | launchpad 0x1017 | ErrInvalidStrategy (code 18), modules/launchpad/types/errors.go:24 | no |
| LXP_LAUNCHPAD_NO_FEES_ACCUMULATED | no fees accumulated | launchpad 0x1017 | ErrNoFeesAccumulated (code 19), modules/launchpad/types/errors.go:25 | yes |
| LXP_LAUNCHPAD_AIRDROP_NOT_TRIGGERED | airdrop not triggered | launchpad 0x1017 | ErrAirdropNotTriggered (code 20), modules/launchpad/types/errors.go:26 | yes |
| LXP_LAUNCHPAD_ALREADY_CLAIMED | airdrop already claimed | launchpad 0x1017 | ErrAlreadyClaimed (code 21), modules/launchpad/types/errors.go:27 | no |
| LXP_LAUNCHPAD_INVALID_MARKET | invalid market | launchpad 0x1017 | ErrInvalidMarket (code 22), modules/launchpad/types/errors.go:28 | no |
| LXP_LAUNCHPAD_UNAUTHORIZED | not the launchpad params authority | launchpad 0x1017 | ErrUnauthorized (code 23), modules/launchpad/types/errors.go:29 | no |
| LXP_LAUNCHPAD_HOLDING_HISTORY | invalid holding history | launchpad 0x1017 | ErrHoldingHistory (code 25), modules/launchpad/types/errors.go:31 | no |
| LXP_LAUNCHPAD_UNSUPPORTED_HOLDING_EPOCH | holding history does not cover epoch | launchpad 0x1017 | ErrUnsupportedHoldingEpoch (code 26), modules/launchpad/types/errors.go:32 | no |
| LXP_LAUNCHPAD_INVALID_AIRDROP_EPOCH | airdrop epoch not opened | launchpad 0x1017 | ErrInvalidAirdropEpoch (code 27), modules/launchpad/types/errors.go:33 | yes |
| LXP_LAUNCHPAD_LEGACY_AIRDROP_EPOCH | airdrop epoch has no entitlement basis | launchpad 0x1017 | ErrLegacyAirdropEpoch (code 28), modules/launchpad/types/errors.go:34 | no |
| LXP_LAUNCHPAD_AIRDROP_BASIS | invalid airdrop entitlement basis | launchpad 0x1017 | ErrAirdropBasis (code 29), modules/launchpad/types/errors.go:35 | no |
| LXP_LAUNCHPAD_INVALID_PARAMS | invalid params | launchpad 0x1017 | ErrInvalidParams (code 2), modules/launchpad/types/errors.go:18 | no |
| LXP_LAUNCHPAD_INVALID_GENESIS | invalid genesis | launchpad 0x1017 | ErrInvalidGenesis (code 24), modules/launchpad/types/errors.go:40 | no |
| LXP_LAUNCHPAD_INVALID_RANGE | invalid range | launchpad 0x1017 | ErrInvalidRange (code 30), modules/launchpad/types/errors.go:46 | no |
| LXP_LAUNCHPAD_ALREADY_INITIALIZED | already initialized | launchpad 0x1017 | ErrAlreadyInitialized (code 31), modules/launchpad/types/errors.go:48 | no |
| LXP_LAUNCHPAD_ALREADY_MINTED | already minted | launchpad 0x1017 | ErrAlreadyMinted (code 32), modules/launchpad/types/errors.go:49 | no |
| LXP_LAUNCHPAD_APPROVE_FAILED | approve failed | launchpad 0x1017 | ErrApproveFailed (code 33), modules/launchpad/types/errors.go:50 | no |
| LXP_LAUNCHPAD_DUPLICATE_POOL | duplicate pool | launchpad 0x1017 | ErrDuplicatePool (code 34), modules/launchpad/types/errors.go:51 | no |
| LXP_LAUNCHPAD_DUPLICATE_TOKEN | duplicate token | launchpad 0x1017 | ErrDuplicateToken (code 35), modules/launchpad/types/errors.go:52 | no |
| LXP_LAUNCHPAD_FEE_OUT_OF_RANGE | fee out of range | launchpad 0x1017 | ErrFeeOutOfRange (code 36), modules/launchpad/types/errors.go:53 | no |
| LXP_LAUNCHPAD_INDEX_OUT_OF_BOUNDS | index out of bounds | launchpad 0x1017 | ErrIndexOutOfBounds (code 37), modules/launchpad/types/errors.go:54 | no |
| LXP_LAUNCHPAD_INSUFFICIENT_BALANCE | insufficient balance | launchpad 0x1017 | ErrInsufficientBalance (code 38), modules/launchpad/types/errors.go:55 | yes |
| LXP_LAUNCHPAD_INSUFFICIENT_CREATION_FEE | insufficient creation fee | launchpad 0x1017 | ErrInsufficientCreationFee (code 39), modules/launchpad/types/errors.go:56 | yes |
| LXP_LAUNCHPAD_INSUFFICIENT_SURPLUS | insufficient surplus | launchpad 0x1017 | ErrInsufficientSurplus (code 40), modules/launchpad/types/errors.go:57 | yes |
| LXP_LAUNCHPAD_INVALID_BALANCE | invalid balance | launchpad 0x1017 | ErrInvalidBalance (code 41), modules/launchpad/types/errors.go:58 | no |
| LXP_LAUNCHPAD_INVALID_CONFIGURATION | invalid configuration | launchpad 0x1017 | ErrInvalidConfiguration (code 42), modules/launchpad/types/errors.go:59 | no |
| LXP_LAUNCHPAD_INVALID_HOOK_RESPONSE | invalid hook response | launchpad 0x1017 | ErrInvalidHookResponse (code 43), modules/launchpad/types/errors.go:60 | no |
| LXP_LAUNCHPAD_INVALID_IMPLEMENTATION | invalid implementation | launchpad 0x1017 | ErrInvalidImplementation (code 44), modules/launchpad/types/errors.go:61 | no |
| LXP_LAUNCHPAD_INVALID_ROLE_ACCOUNT | invalid role account | launchpad 0x1017 | ErrInvalidRoleAccount (code 45), modules/launchpad/types/errors.go:62 | no |
| LXP_LAUNCHPAD_LAST_DEFAULT_ADMIN | last default admin | launchpad 0x1017 | ErrLastDefaultAdmin (code 46), modules/launchpad/types/errors.go:63 | no |
| LXP_LAUNCHPAD_MISSING_ROLE | missing role | launchpad 0x1017 | ErrMissingRole (code 47), modules/launchpad/types/errors.go:64 | no |
| LXP_LAUNCHPAD_MULTICALL_FAILED | multicall failed | launchpad 0x1017 | ErrMulticallFailed (code 48), modules/launchpad/types/errors.go:65 | no |
| LXP_LAUNCHPAD_NON_ERC721_RECEIVER | non erc721 receiver | launchpad 0x1017 | ErrNonERC721Receiver (code 49), modules/launchpad/types/errors.go:66 | no |
| LXP_LAUNCHPAD_NON_PAYABLE_UPGRADE | non payable upgrade | launchpad 0x1017 | ErrNonPayableUpgrade (code 50), modules/launchpad/types/errors.go:67 | no |
| LXP_LAUNCHPAD_NOT_APPROVED | not approved | launchpad 0x1017 | ErrNotApproved (code 51), modules/launchpad/types/errors.go:68 | no |
| LXP_LAUNCHPAD_NOT_INITIALIZING | not initializing | launchpad 0x1017 | ErrNotInitializing (code 52), modules/launchpad/types/errors.go:69 | no |
| LXP_LAUNCHPAD_NOT_TOKEN_OWNER | not token owner | launchpad 0x1017 | ErrNotTokenOwner (code 53), modules/launchpad/types/errors.go:70 | no |
| LXP_LAUNCHPAD_OPTICAL_QUOTE_UNSUPPORTED | optical quote unsupported | launchpad 0x1017 | ErrOpticalQuoteUnsupported (code 54), modules/launchpad/types/errors.go:71 | no |
| LXP_LAUNCHPAD_POOL_INDEX_MIGRATION_REQUIRED | pool index migration required | launchpad 0x1017 | ErrPoolIndexMigrationRequired (code 55), modules/launchpad/types/errors.go:72 | no |
| LXP_LAUNCHPAD_REENTRANCY_GUARD_REENTRANT_CALL | reentrancy guard reentrant call | launchpad 0x1017 | ErrReentrancyGuardReentrantCall (code 56), modules/launchpad/types/errors.go:73 | no |
| LXP_LAUNCHPAD_SAME_TOKEN | same token | launchpad 0x1017 | ErrSameToken (code 57), modules/launchpad/types/errors.go:74 | no |
| LXP_LAUNCHPAD_TOKEN_NOT_FOUND | token not found | launchpad 0x1017 | ErrTokenNotFound (code 58), modules/launchpad/types/errors.go:75 | no |
| LXP_LAUNCHPAD_TRANSFER_FAILED | transfer failed | launchpad 0x1017 | ErrTransferFailed (code 59), modules/launchpad/types/errors.go:76 | no |
| LXP_LAUNCHPAD_TRANSFER_FROM_FAILED | transfer from failed | launchpad 0x1017 | ErrTransferFromFailed (code 60), modules/launchpad/types/errors.go:77 | no |
| LXP_LAUNCHPAD_UUPS_NOT_THROUGH_ACTIVE_PROXY | uups not through active proxy | launchpad 0x1017 | ErrUUPSNotThroughActiveProxy (code 61), modules/launchpad/types/errors.go:78 | no |
| LXP_LAUNCHPAD_UUPS_NOT_THROUGH_PROXY | uups not through proxy | launchpad 0x1017 | ErrUUPSNotThroughProxy (code 62), modules/launchpad/types/errors.go:79 | no |
| LXP_LAUNCHPAD_UNAUTHORIZED_UPGRADE | unauthorized upgrade | launchpad 0x1017 | ErrUnauthorizedUpgrade (code 63), modules/launchpad/types/errors.go:80 | no |
| LXP_LAUNCHPAD_UNEXECUTABLE_QUOTE | unexecutable quote | launchpad 0x1017 | ErrUnexecutableQuote (code 64), modules/launchpad/types/errors.go:81 | no |
| LXP_LAUNCHPAD_UNSUPPORTED_SNAPSHOT_TOKEN | unsupported snapshot token | launchpad 0x1017 | ErrUnsupportedSnapshotToken (code 65), modules/launchpad/types/errors.go:82 | no |
| LXP_LAUNCHPAD_UPGRADE_INITIALIZATION_FAILED | upgrade initialization failed | launchpad 0x1017 | ErrUpgradeInitializationFailed (code 66), modules/launchpad/types/errors.go:83 | no |
| LXP_LAUNCHPAD_INVALID_TOKEN | invalid token | launchpad 0x1017 | ErrInvalidToken (code 67), modules/launchpad/types/errors.go:84 | no |
| LXP_LAUNCHPAD_REENTRANT_CALL | reentrant call | launchpad 0x1017 | ErrReentrantCall (code 68), modules/launchpad/types/errors.go:85 | no |
| LXP_LAUNCHPAD_ACCESS_CONTROL_UNAUTHORIZED_ACCOUNT | access control unauthorized account | launchpad 0x1017 | ErrAccessControlUnauthorizedAccount (code 69), modules/launchpad/types/errors.go:86 | no |
| LXP_LAUNCHPAD_RETIRED | retired | launchpad 0x1017 | ErrRetired (code 70), modules/launchpad/types/errors.go:87 | no |
| LXP_LAUNCHPAD_FUTURE_LOOKUP | future lookup | launchpad 0x1017 | ErrFutureLookup (code 71), modules/launchpad/types/errors.go:88 | yes |
| LXP_LAUNCHPAD_INSUFFICIENT_ALLOWANCE | insufficient allowance | launchpad 0x1017 | ErrInsufficientAllowance (code 72), modules/launchpad/types/errors.go:89 | yes |
| LXP_LAUNCHPAD_INVALID_PERMIT | invalid permit | launchpad 0x1017 | ErrInvalidPermit (code 73), modules/launchpad/types/errors.go:90 | no |
| LXP_LAUNCHPAD_INVALID_POOL_BINDING | invalid pool binding | launchpad 0x1017 | ErrInvalidPoolBinding (code 74), modules/launchpad/types/errors.go:91 | no |
| LXP_LAUNCHPAD_PERMIT_EXPIRED | permit expired | launchpad 0x1017 | ErrPermitExpired (code 75), modules/launchpad/types/errors.go:92 | no |
| LXP_ANCHOR_CHECKPOINT_UNKNOWN | checkpoint unknown | layerxanchor 0x1014 read methods until the store-delete step (writes retired) | ErrCheckpointUnknown (code 8), modules/layerxanchor/types/errors.go:12 | no |
| LXP_ANCHOR_GUARANTOR_UNKNOWN | guarantor unknown | layerxanchor 0x1014 read methods until the store-delete step (writes retired) | ErrGuarantorUnknown (code 10), modules/layerxanchor/types/errors.go:14 | no |
| LXP_TOKENFACTORY_DENOM_EXISTS | attempting to create a denom that already exists (has bank metadata) | layerxledger 0x101A register/mint/burn through the tokenfactory keeper | ErrDenomExists (code 2), modules/tokenfactory/types/errors.go:13 | no |
| LXP_TOKENFACTORY_UNAUTHORIZED | unauthorized account | layerxledger 0x101A register/mint/burn through the tokenfactory keeper | ErrUnauthorized (code 3), modules/tokenfactory/types/errors.go:14 | no |
| LXP_TOKENFACTORY_INVALID_DENOM | invalid denom | layerxledger 0x101A register/mint/burn through the tokenfactory keeper | ErrInvalidDenom (code 4), modules/tokenfactory/types/errors.go:15 | no |
| LXP_TOKENFACTORY_SUBDENOM_TOO_LONG | subdenom too long, max length is MaxSubdenomLength bytes | layerxledger 0x101A register/mint/burn through the tokenfactory keeper | ErrSubdenomTooLong (code 8), modules/tokenfactory/types/errors.go:19 | no |
| LXP_TOKENFACTORY_DENOM_DOES_NOT_EXIST | denom does not exist | layerxledger 0x101A register/mint/burn through the tokenfactory keeper | ErrDenomDoesNotExist (code 10), modules/tokenfactory/types/errors.go:21 | no |
| LXP_EXCHANGE_INVALID_MARKET | invalid market | layerxexchange 0x1015 | ErrInvalidMarket (code 3), modules/layerxexchange/types/errors.go:9 | no |
| LXP_EXCHANGE_UNKNOWN_MARKET | market is not listed or is disabled | layerxexchange 0x1015 | ErrUnknownMarket (code 4), modules/layerxexchange/types/errors.go:10 | no |
| LXP_EXCHANGE_NOT_MARGIN_ASSET | asset is not the margin asset of an enabled market | layerxexchange 0x1015 | ErrNotMarginAsset (code 5), modules/layerxexchange/types/errors.go:11 | no |
| LXP_EXCHANGE_KEEPER_MISSING | layerxexchange keeper is not wired | layerxexchange 0x1015 | ErrKeeperMissing (code 12), modules/layerxexchange/types/errors.go:18 | no |
| LXP_ADDR_PANIC | execution reverted: <recovered value> | addr 0x1004 | precompiles/addr/addr.go:138 | no |
| LXP_ADDR_STATICCALL | cannot call <method> from staticcall | addr 0x1004 | precompiles/addr/addr.go:148,153,158,166 | no |
| LXP_ADDR_DELEGATECALL | cannot delegatecall <method> | addr 0x1004 | precompiles/addr/addr.go:161,169 | no |
| LXP_ADDR_EVM_NOT_ASSOCIATED | EVM address is not associated | addr 0x1004 | precompiles/addr/addr.go:196 | yes |
| LXP_ADDR_PAX_NOT_ASSOCIATED | pax address is not associated | addr 0x1004 | precompiles/addr/addr.go:219 | yes |
| LXP_ADDR_ALREADY_ASSOCIATED | address is already associated with evm address | addr 0x1004 | precompiles/addr/addr.go:308 | no |
| LXP_ADDR_EVM_NOT_BOUND | EVM address is not bound to a LayerX DID | addr 0x1004; principal resolution of 0x101A-0x101D (sender without a DID) | precompiles/addr/addr.go:401 | yes |
| LXP_ADDR_DID_NOT_BOUND | LayerX DID is not bound to an EVM address | addr 0x1004 | precompiles/addr/addr.go:417 | yes |
| LXP_ADDR_LAYERX_CHAIN_ID | layerx binding: chain id does not fit 32 bytes | addr 0x1004 (binding keeper) | ErrLayerXChainID, modules/evm/types/layerx_binding.go:28 | no |
| LXP_ADDR_LAYERX_SIGNATURE | layerx binding: signature refused | addr 0x1004 (binding keeper) | ErrLayerXSignature, modules/evm/types/layerx_binding.go:29 | no |
| LXP_ADDR_LAYERX_ADDRESS_BOUND | layerx binding: evm address is already bound to a did | addr 0x1004 (binding keeper) | ErrLayerXAddressBound, modules/evm/types/layerx_binding.go:30 | no |
| LXP_ADDR_LAYERX_DID_BOUND | layerx binding: did is already bound to an evm address | addr 0x1004 (binding keeper) | ErrLayerXDidBound, modules/evm/types/layerx_binding.go:31 | no |
| LXP_ADDR_LAYERX_NOT_BOUND | layerx binding: evm address is not bound to a did | addr 0x1004 (binding keeper) | ErrLayerXNotBound, modules/evm/types/layerx_binding.go:32 | yes |
| LXP_ADDR_LAYERX_NONCE_EXHAUSTED | layerx binding: nonce exhausted | addr 0x1004 (binding keeper) | ErrLayerXNonceExhausted, modules/evm/types/layerx_binding.go:33 | no |
| LXP_ADDR_LAYERX_NON_CANONICAL_KEY | layerx binding: did public key is not canonical | addr 0x1004 (binding keeper) | ErrLayerXNonCanonicalKey, modules/evm/types/layerx_binding.go:34 | no |
| LXP_ADDR_LAYERX_SIGNATURE_LENGTH | layerx binding: signature must be 64 bytes | addr 0x1004 (binding keeper) | ErrLayerXSignatureLength, modules/evm/types/layerx_binding.go:35 | no |
| LXP_FEETOKEN_KEEPER_MISSING | feetoken: fee-token EVM keeper is unavailable | feetoken 0x1018 | precompiles/feetoken/feetoken.go:45,85 | no |
| LXP_FEETOKEN_DELEGATECALL | cannot delegatecall feeToken | feetoken 0x1018 | precompiles/feetoken/feetoken.go:76 | no |
| LXP_FEETOKEN_STATICCALL | cannot call a feeToken state change from staticcall | feetoken 0x1018 | precompiles/feetoken/feetoken.go:82 | no |
| LXP_FEETOKEN_UNKNOWN_METHOD | feetoken: unknown method | feetoken 0x1018 | precompiles/feetoken/feetoken.go:108 | no |
| LXP_FEETOKEN_SWITCH_OFF | fee token preference: fee-token switch is off | feetoken 0x1018 | modules/evm/keeper/feetoken.go:27 | yes |
| LXP_FEETOKEN_DENOM_NOT_ALLOWED | fee token preference: denom is not allowed | feetoken 0x1018 | modules/evm/keeper/feetoken.go:30 (ErrFeeTokenDenomNotAllowed :43) | no |
| LXP_PRICEFEED_DELEGATECALL | cannot delegatecall pricefeed | pricefeed 0x1020 | precompile site precompiles/pricefeed/pricefeed.go:157 | no |
| LXP_PRICEFEED_STATICCALL | cannot call pricefeed <method> from staticcall | pricefeed 0x1020: state changes | precompile site precompiles/pricefeed/pricefeed.go:160 | no |
| LXP_PRICEFEED_KEEPER_MISSING | pricefeed: the pricefeed keeper is not wired into this application | pricefeed 0x1020 | precompile site precompiles/pricefeed/pricefeed.go:163 | no |
| LXP_PRICEFEED_BATCH_TOO_LARGE | getPrices takes at most 64 market ids | pricefeed 0x1020: getPrices | precompile site precompiles/pricefeed/views.go:89 | no |
| LXP_PRECOMPILE_NO_CONTEXT | cannot get context from EVM | every precompile (framework) | precompiles/common/precompiles.go:125 | no |
| LXP_PRECOMPILE_UNKNOWN_METHOD | no method with id: <selector> | every precompile (framework) | precompiles/common/precompiles.go:131 (abi MethodById); text from pax-geth v1.15.7-pax.17 `no method with id: %#x` (go.mod:280 replaces go-ethereum) | no |
| LXP_PRECOMPILE_INPUT_TOO_SHORT | input too short to extract method ID | every precompile (framework) | precompiles/common/precompiles.go:281 | no |
| LXP_PRECOMPILE_ARG_COUNT | expected N arguments but got M | every precompile (framework) | precompiles/common/precompiles.go:213 | no |
| LXP_PRECOMPILE_NON_PAYABLE | sending funds to a non-payable function | every precompile (framework) | precompiles/common/precompiles.go:221 | no |
| LXP_PRECOMPILE_WEI_REMAINDER | selected precompile function does not allow payment with non-zero wei remainder | every payable precompile (framework); xweb 0x1019 `request` and `fulfil` (web.md A1; precompiles/xweb/xweb_test.go:296-298) | precompiles/common/precompiles.go:230 | no |
| LXP_PRECOMPILE_INVALID_ADDR | invalid addr | every precompile taking an address argument (framework) | precompiles/common/precompiles.go:318 | no |
| LXP_AGENT_NOT_ACTIVE | agent function not active | agent functions 0x0901-0x0908: every agent function | ErrNotActive, precompiles/layerxagent/common/common.go:66; HeightGate :99,103 | no |
| LXP_AGENT_ABI_DECODE | argument decode failed | agent functions 0x0901-0x0908: every agent function | agent-functions.md A.0; new under precompiles/layerxagent | no |
| LXP_AGENT_READ_ONLY | write under staticcall | agent functions 0x0901-0x0908: submitPrice, schedule, cancel, reschedule, open, settle, close, updateRate | agent-functions.md A.0; new under precompiles/layerxagent | no |
| LXP_AGENT_DELEGATECALL | cannot delegatecall an agent function | agent functions 0x0901-0x0908: every write method | agent-functions.md A.0; new under precompiles/layerxagent | no |
| LXP_AGENT_OVERFLOW | result out of range | agent functions 0x0901-0x0908: orob 0x0901, clearing 0x0902, pofq 0x0904, streams 0x0906 | agent-functions.md A.0; new under precompiles/layerxagent | no |
| LXP_AGENT_BATCH_TOO_LARGE | batch over the cap (cap set at activation rehearsal) | agent functions 0x0901-0x0908: resolveOffsetBatch, computeClearing, scoreBatch, aggregate | agent-functions.md A.0; new under precompiles/layerxagent | no |
| LXP_AGENT_OROB_ORACLE_PRICE | oraclePrice cannot be zero | agent functions 0x0901-0x0908: orob 0x0901, computeClearing | agent-functions.md A.0; new under precompiles/layerxagent | no |
| LXP_AGENT_OROB_OFFSET_RANGE | offset out of range | agent functions 0x0901-0x0908: orob 0x0901, computeClearing | agent-functions.md A.0; new under precompiles/layerxagent | no |
| LXP_AGENT_CLEARING_LENGTH_MISMATCH | buy or sell arrays length mismatch | agent functions 0x0901-0x0908: computeClearing | agent-functions.md A.0; new under precompiles/layerxagent | no |
| LXP_AGENT_ORACLE_NO_FEEDS | no feeds provided | agent functions 0x0901-0x0908: aggregate | agent-functions.md A.0; new under precompiles/layerxagent | no |
| LXP_AGENT_ORACLE_NO_CONFIDENT_FEEDS | no feeds with non-zero confidence | agent functions 0x0901-0x0908: aggregate | agent-functions.md A.0; new under precompiles/layerxagent | no |
| LXP_AGENT_ORACLE_NOT_SIGNER | caller is not a validator oracle signer | agent functions 0x0901-0x0908: submitPrice | agent-functions.md A.0; new under precompiles/layerxagent | no |
| LXP_AGENT_ORACLE_PRICE | invalid price | agent functions 0x0901-0x0908: submitPrice, aggregate | agent-functions.md A.0; new under precompiles/layerxagent | no |
| LXP_AGENT_ORACLE_CONFIDENCE | invalid confidence | agent functions 0x0901-0x0908: aggregate, submitPrice | agent-functions.md A.0; new under precompiles/layerxagent | no |
| LXP_AGENT_POFQ_LENGTH_MISMATCH | array length mismatch | agent functions 0x0901-0x0908: scoreBatch | agent-functions.md A.0; new under precompiles/layerxagent | no |
| LXP_AGENT_POFQ_DECAY_RANGE | decayBps must be < 10000 | agent functions 0x0901-0x0908: updateRollingScore | agent-functions.md A.0; new under precompiles/layerxagent | no |
| LXP_AGENT_SCHEDULER_JOB_NOT_FOUND | job not found | agent functions 0x0901-0x0908: cancel, reschedule | agent-functions.md A.0; new under precompiles/layerxagent | no |
| LXP_AGENT_SCHEDULER_UNAUTHORIZED | unauthorized | agent functions 0x0901-0x0908: cancel, reschedule | agent-functions.md A.0; new under precompiles/layerxagent | no |
| LXP_AGENT_SCHEDULER_HORIZON_EXCEEDED | horizon exceeded | agent functions 0x0901-0x0908: schedule, reschedule | agent-functions.md A.0; new under precompiles/layerxagent | no |
| LXP_AGENT_SCHEDULER_PAST_BLOCK | target block is in the past | agent functions 0x0901-0x0908: schedule, reschedule | agent-functions.md A.0; new under precompiles/layerxagent | no |
| LXP_AGENT_SCHEDULER_MAX_JOBS | max jobs exceeded | agent functions 0x0901-0x0908: schedule | agent-functions.md A.0; new under precompiles/layerxagent | yes |
| LXP_AGENT_SCHEDULER_DEPOSIT_TOO_LOW | deposit too low | agent functions 0x0901-0x0908: schedule | agent-functions.md A.0; new under precompiles/layerxagent | no |
| LXP_AGENT_SCHEDULER_INVALID_GAS_LIMIT | invalid gas limit | agent functions 0x0901-0x0908: schedule | agent-functions.md A.0; new under precompiles/layerxagent | no |
| LXP_AGENT_SCHEDULER_INVALID_TARGET | invalid target | agent functions 0x0901-0x0908: schedule | agent-functions.md A.0; new under precompiles/layerxagent | no |
| LXP_AGENT_SCHEDULER_JOB_INACTIVE | job inactive | agent functions 0x0901-0x0908: cancel, reschedule | agent-functions.md A.0; new under precompiles/layerxagent | no |
| LXP_AGENT_SCHEDULER_ESCROW | escrow deposit failed | agent functions 0x0901-0x0908: schedule | agent-functions.md A.0; new under precompiles/layerxagent | no |
| LXP_AGENT_STREAMS_INVALID_PAYEE | invalid payee | agent functions 0x0901-0x0908: open | agent-functions.md A.0; new under precompiles/layerxagent | no |
| LXP_AGENT_STREAMS_SELF_PAYMENT | self payment | agent functions 0x0901-0x0908: open | agent-functions.md A.0; new under precompiles/layerxagent | no |
| LXP_AGENT_STREAMS_UNKNOWN_TOKEN | token has no assetId mapping | agent functions 0x0901-0x0908: open | agent-functions.md A.0; new under precompiles/layerxagent | no |
| LXP_AGENT_TEE_UNKNOWN_FAMILY | unknown family | agent functions 0x0901-0x0908: verify, verifyAndExpect | agent-functions.md A.0; new under precompiles/layerxagent | no |
| LXP_AGENT_TEE_NO_ROOTS | no roots loaded | agent functions 0x0901-0x0908: verify, verifyAndExpect | agent-functions.md A.0; new under precompiles/layerxagent | yes |
| LXP_AGENT_TEE_INVALID_QUOTE | invalid quote | agent functions 0x0901-0x0908: verify, verifyAndExpect | agent-functions.md A.0; new under precompiles/layerxagent | no |
| LXP_AGENT_TEE_UNTRUSTED_CHAIN | untrusted chain | agent functions 0x0901-0x0908: verify, verifyAndExpect | agent-functions.md A.0; new under precompiles/layerxagent | no |
| LXP_AGENT_TEE_STALE | attestation stale | agent functions 0x0901-0x0908: verify, verifyAndExpect | agent-functions.md A.0; new under precompiles/layerxagent | no |
| LXP_AGENT_TEE_DEBUG_REFUSED | debug enclave refused | agent functions 0x0901-0x0908: verify, verifyAndExpect | agent-functions.md A.0; new under precompiles/layerxagent | no |
| LXP_AGENT_TEE_REPORT_DATA_MISMATCH | report data mismatch | agent functions 0x0901-0x0908: verifyAndExpect | agent-functions.md A.0; new under precompiles/layerxagent | no |
| LXP_AGENT_TEE_UNAUTHORIZED_UPDATE | unauthorized root update | agent functions 0x0901-0x0908: tee root update (governance) | agent-functions.md A.0; new under precompiles/layerxagent | no |
| LXP_AGENT_SCHEDULER_CALLDATA_TOO_LARGE | scheduler calldata too large | agent scheduler 0x0905: schedule | ErrSchedulerCallDataTooLarge, precompiles/layerxagent/common/common.go:54 | no |
| LXP_PERPS_INVALID_PARAMS | invalid params | perps 0x1015 | ErrInvalidParams (code 2), modules/perps/types/errors.go:14 | no |
| LXP_PERPS_UNKNOWN_MARKET | unknown market | perps 0x1015 | ErrUnknownMarket (code 3), modules/perps/types/errors.go:15 | no |
| LXP_PERPS_MARKET_DISABLED | market disabled | perps 0x1015 | ErrMarketDisabled (code 4), modules/perps/types/errors.go:16 | yes |
| LXP_PERPS_PRICE_STALE | price stale | perps 0x1015 | ErrPriceStale (code 5), modules/perps/types/errors.go:17 | yes |
| LXP_PERPS_ZERO_AMOUNT | zero amount | perps 0x1015 | ErrZeroAmount (code 6), modules/perps/types/errors.go:18 | no |
| LXP_PERPS_UNSUPPORTED_COLLATERAL | unsupported collateral | perps 0x1015 | ErrUnsupportedCollateral (code 7), modules/perps/types/errors.go:19 | no |
| LXP_PERPS_INSUFFICIENT_COLLATERAL | insufficient collateral | perps 0x1015 | ErrInsufficientCollateral (code 8), modules/perps/types/errors.go:20 | yes |
| LXP_PERPS_INSUFFICIENT_MARGIN | insufficient margin | perps 0x1015 | ErrInsufficientMargin (code 9), modules/perps/types/errors.go:21 | yes |
| LXP_PERPS_LEVERAGE_TOO_HIGH | leverage too high | perps 0x1015 | ErrLeverageTooHigh (code 10), modules/perps/types/errors.go:22 | no |
| LXP_PERPS_TICK_SIZE | tick size | perps 0x1015 | ErrTickSize (code 11), modules/perps/types/errors.go:23 | no |
| LXP_PERPS_LOT_SIZE | lot size | perps 0x1015 | ErrLotSize (code 12), modules/perps/types/errors.go:24 | no |
| LXP_PERPS_MIN_ORDER_VALUE | min order value | perps 0x1015 | ErrMinOrderValue (code 13), modules/perps/types/errors.go:25 | no |
| LXP_PERPS_MAX_ORDER_VALUE | max order value | perps 0x1015 | ErrMaxOrderValue (code 14), modules/perps/types/errors.go:26 | no |
| LXP_PERPS_OPEN_INTEREST_CAP | open interest cap | perps 0x1015 | ErrOpenInterestCap (code 15), modules/perps/types/errors.go:27 | yes |
| LXP_PERPS_INVALID_TIME_IN_FORCE | invalid time in force | perps 0x1015 | ErrInvalidTimeInForce (code 16), modules/perps/types/errors.go:28 | no |
| LXP_PERPS_INVALID_ORDER_TYPE | invalid order type | perps 0x1015 | ErrInvalidOrderType (code 17), modules/perps/types/errors.go:29 | no |
| LXP_PERPS_POST_ONLY_WOULD_CROSS | post only would cross | perps 0x1015 | ErrPostOnlyWouldCross (code 18), modules/perps/types/errors.go:30 | no |
| LXP_PERPS_FOK_NOT_FILLED | fok not filled | perps 0x1015 | ErrFokNotFilled (code 19), modules/perps/types/errors.go:31 | yes |
| LXP_PERPS_NO_LIQUIDITY | no liquidity | perps 0x1015 | ErrNoLiquidity (code 20), modules/perps/types/errors.go:32 | yes |
| LXP_PERPS_SLIPPAGE_EXCEEDED | slippage exceeded | perps 0x1015 | ErrSlippageExceeded (code 21), modules/perps/types/errors.go:33 | yes |
| LXP_PERPS_DEADLINE_EXPIRED | deadline expired | perps 0x1015 | ErrDeadlineExpired (code 22), modules/perps/types/errors.go:34 | no |
| LXP_PERPS_REDUCE_ONLY_VIOLATION | reduce only violation | perps 0x1015 | ErrReduceOnlyViolation (code 23), modules/perps/types/errors.go:35 | no |
| LXP_PERPS_DUPLICATE_CLOID | duplicate cloid | perps 0x1015 | ErrDuplicateCloid (code 24), modules/perps/types/errors.go:36 | no |
| LXP_PERPS_UNKNOWN_ORDER | unknown order | perps 0x1015 | ErrUnknownOrder (code 25), modules/perps/types/errors.go:37 | no |
| LXP_PERPS_NOT_ORDER_HOLDER | not order holder | perps 0x1015 | ErrNotOrderHolder (code 26), modules/perps/types/errors.go:38 | no |
| LXP_PERPS_UNKNOWN_POSITION | unknown position | perps 0x1015 | ErrUnknownPosition (code 27), modules/perps/types/errors.go:39 | no |
| LXP_PERPS_NOT_POSITION_HOLDER | not position holder | perps 0x1015 | ErrNotPositionHolder (code 28), modules/perps/types/errors.go:40 | no |
| LXP_PERPS_INVALID_TRIGGER_PRICE | invalid trigger price | perps 0x1015 | ErrInvalidTriggerPrice (code 29), modules/perps/types/errors.go:41 | no |
| LXP_PERPS_TRIGGER_NOT_REACHED | trigger not reached | perps 0x1015 | ErrTriggerNotReached (code 30), modules/perps/types/errors.go:42 | no |
| LXP_PERPS_TOO_MANY_OPEN_ORDERS | too many open orders | perps 0x1015 | ErrTooManyOpenOrders (code 31), modules/perps/types/errors.go:43 | yes |
| LXP_PERPS_CROSS_MARGIN_ONLY | cross margin only | perps 0x1015 | ErrCrossMarginOnly (code 32), modules/perps/types/errors.go:44 | no |
| LXP_PERPS_NOT_LIQUIDATABLE | not liquidatable | perps 0x1015 | ErrNotLiquidatable (code 33), modules/perps/types/errors.go:45 | no |
| LXP_PERPS_LIQUIDATION_COOLDOWN | liquidation cooldown | perps 0x1015 | ErrLiquidationCooldown (code 34), modules/perps/types/errors.go:46 | yes |
| LXP_PERPS_UNAUTHORIZED | unauthorized | perps 0x1015 | ErrUnauthorized (code 35), modules/perps/types/errors.go:47 | no |
| LXP_PERPS_STATIC_CALL | static call | perps 0x1015 | ErrStaticCall (code 36), modules/perps/types/errors.go:48 | no |
| LXP_PERPS_DELEGATECALL | delegatecall | perps 0x1015 | ErrDelegatecall (code 37), modules/perps/types/errors.go:49 | no |
| LXP_PERPS_OVERFLOW | overflow | perps 0x1015 | ErrOverflow (code 38), modules/perps/types/errors.go:50 | no |
| LXP_PERPS_INVALID_GENESIS | invalid genesis | perps 0x1015 | ErrInvalidGenesis (code 39), modules/perps/types/errors.go:51 | no |
| LXP_PERPS_CLOSE_SIZE_EXCEEDS_POSITION | close size exceeds position | perps 0x1015 | ErrCloseSizeExceedsPosition (code 40), modules/perps/types/errors.go:52 | no |
| LXP_PERPS_SELF_TRANSFER | self transfer | perps 0x1015 | ErrSelfTransfer (code 41), modules/perps/types/errors.go:53 | no |
| LXP_PERPS_ZERO_ADDRESS | zero address | perps 0x1015 | ErrZeroAddress (code 42), modules/perps/types/errors.go:54 | no |
| LXP_PERPS_NO_ACCOUNT | no account | perps 0x1015 | ErrNoAccount (code 43), modules/perps/types/errors.go:55 | no |
| LXP_PERPS_TOKEN_TRANSFER_FAILED | token transfer failed | perps 0x1015 | ErrTokenTransferFailed (code 44), modules/perps/types/errors.go:56 | no |
| LXP_PERPS_PAUSED | paused | perps 0x1015 | ErrPaused (code 45), modules/perps/types/errors.go:57 | yes |
| LXP_PRICEFEED_UNAUTHORIZED | unauthorized | pricefeed 0x1020 | ErrUnauthorized (code 2), modules/pricefeed/types/errors.go:12 | no |
| LXP_PRICEFEED_INVALID_PARAMS | invalid params | pricefeed 0x1020 | ErrInvalidParams (code 3), modules/pricefeed/types/errors.go:13 | no |
| LXP_PRICEFEED_INVALID_GENESIS | invalid genesis | pricefeed 0x1020 | ErrInvalidGenesis (code 4), modules/pricefeed/types/errors.go:14 | no |
| LXP_PRICEFEED_NOT_VALIDATOR | not a bonded validator | pricefeed 0x1020 | ErrNotValidator (code 5), modules/pricefeed/types/errors.go:15 | no |
| LXP_PRICEFEED_INVALID_KEY | feed key must be a 33-byte compressed secp256k1 public key | pricefeed 0x1020 | ErrInvalidKey (code 6), modules/pricefeed/types/errors.go:16 | no |
| LXP_PRICEFEED_KEY_IN_USE | feed account is registered to another validator | pricefeed 0x1020 | ErrKeyInUse (code 7), modules/pricefeed/types/errors.go:17 | no |
| LXP_PRICEFEED_NO_KEY | validator has no feed key | pricefeed 0x1020 | ErrNoKey (code 8), modules/pricefeed/types/errors.go:18 | no |
| LXP_PRICEFEED_UNKNOWN_MARKET | market is not listed | pricefeed 0x1020 | ErrUnknownMarket (code 9), modules/pricefeed/types/errors.go:19 | no |
| LXP_PRICEFEED_UNREGISTERED_FEEDER | feeder is not a registered feed account | pricefeed 0x1020 | ErrUnregisteredFeeder (code 10), modules/pricefeed/types/errors.go:20 | no |
| LXP_PRICEFEED_RATE_LIMITED | the validator has already submitted prices at this height | pricefeed 0x1020 | ErrRateLimited (code 11), modules/pricefeed/types/errors.go:21 | yes |
| LXP_PRICEFEED_TOO_MANY_OBSERVATIONS | too many observations in one submission | pricefeed 0x1020 | ErrTooManyObservations (code 12), modules/pricefeed/types/errors.go:22 | no |
| LXP_PRICEFEED_SUBMISSION_TOO_LARGE | submission is over the byte bound | pricefeed 0x1020 | ErrSubmissionTooLarge (code 13), modules/pricefeed/types/errors.go:23 | no |
| LXP_PRICEFEED_DUPLICATE_MARKET | a submission carries one observation per market | pricefeed 0x1020 | ErrDuplicateMarket (code 14), modules/pricefeed/types/errors.go:24 | no |
| LXP_PRICEFEED_SUBMISSION_ALONE | price submissions cannot be in the same tx as other messages | pricefeed 0x1020 | ErrSubmissionAlone (code 15), modules/pricefeed/types/errors.go:25 | no |
| LXP_PRICEFEED_INVALID_OBSERVATION | invalid observation | pricefeed 0x1020 | ErrInvalidObservation (code 16), modules/pricefeed/types/errors.go:26 | no |

A guest trap in a profiled CALL is a retained failure record with replay evidence.
LXP_ENGINE_FAULT describes an executor engine fault of a CALL.

## (c) Activation

`MergeUpgrade` is `v7.0`, defined by [the common precompile code](../../../precompiles/common/precompiles.go) and used by [node upgrades](../../../node/upgrades.go). The node registers `mergeUpgradePlan` as its handler. Activation checks read `IsUpgradeActiveAtHeight`; the EVM keeper stores a list of [CustomPrecompileActivation](../../../modules/evm/keeper/keeper.go) entries.

| Item | Value | Source |
| --- | --- | --- |
| Base fork activation | `v6.9`, with the module dependencies of `activationPrecompileGate` | [node/upgrades.go](../../../node/upgrades.go) |
| Perps, launchpad and spot | `v6.12.0`, with versioned contract switches and spot module gates | [node/precompiles_v6120.go](../../../node/precompiles_v6120.go) |
| Merge construct addresses | `0x101A` through `0x101E`, requiring their respective ledger, pay, auth, programs and chains modules | [mergePrecompileGate](../../../node/upgrades.go) |
| Agent function addresses | `0x0901` through `0x0908`; all require `layerxagent` except streams, which requires `layerxpay`; oracle also requires `layerxexchange` | [mergePrecompileGate](../../../node/upgrades.go) |
| Constructor validation | All entries in `MergeAddressTable` require registered constructors with matching names and addresses | [precompiles/setup.go](../../../precompiles/setup.go), [node/upgrades.go](../../../node/upgrades.go) |
| Retired addresses | Verify and custody methods refuse calls after merge activation; anchor writes are retired, and anchor reads are retired after `v7.0-anchor-store-delete` | [verify](../../../precompiles/layerxverify), [custody](../../../precompiles/layerxcustody), [anchor](../../../precompiles/layerxanchor), [upgrade handler](../../../node/upgrades.go) |

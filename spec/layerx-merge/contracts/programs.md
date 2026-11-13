# Programs contract: layerxprograms 0x101D

The program construct at `0x000000000000000000000000000000000000101D` is implemented by [precompiles/layerxprograms](../../../precompiles/layerxprograms). Its `NewMergePrecompile` consumes the shared host, EVM keeper and xweb keeper. The node imports its constructor and includes the address in the `v7.0` merge gate.

Program execution belongs to the one Paxeer state machine. Mainnet-beta uses EVM chain ID `125` (`0x7d`); a receipt is final when its block commits. Program state is owned by the shared host rather than a separate programs chain module.

## layerxprograms-0x101d

The current [ABI](../../../precompiles/layerxprograms/abi.json) declares these methods. [Dispatch](../../../precompiles/layerxprograms/submit.go) and method handlers define authority and validation; ABI declarations alone do not establish successful execution for every input.

| Method | Mutability | Returns | Selector |
|---|---|---|---|
| `call(bytes32,uint16,string,bytes,bytes,bytes,uint32,uint64[7],uint128)` | nonpayable | `int32,uint8,bytes` | `0x54bd2b48` |
| `callWithReceipts(bytes32,uint16,string,bytes,bytes,bytes,uint32,uint64[7],uint128,(bytes,uint64)[])` | nonpayable | `int32,uint8,bytes` | `0xabc313ef` |
| `deploy(bytes32,uint16,uint8,bytes32,bytes32,bytes,bytes)` | nonpayable | `none` | `0xbde3ff66` |
| `deprecate(bytes32,bytes32,uint64)` | nonpayable | `none` | `0x86ba1a99` |
| `exitValue(bytes32,bytes32)` | nonpayable | `none` | `0x4dc50730` |
| `exitValueBounded(bytes32,bytes32,uint128)` | nonpayable | `none` | `0xade5d9e6` |
| `getAccountProfile(bytes32)` | view | `uint8,bytes32` | `0x0f2d0496` |
| `getCurrentFeeSchedule()` | view | `uint32,uint64[7],bytes32` | `0xbbe2e42d` |
| `getCurrentMeteringSchedule()` | view | `uint32,uint64[9]` | `0x543c7a2f` |
| `getExitRoute(bytes32,bytes32)` | view | `bytes32,bytes32,bytes` | `0x3ee6f18d` |
| `getFeeSchedule(uint32)` | view | `uint64[7],bytes32` | `0x8d8bfb15` |
| `getInterface(bytes32)` | view | `uint32,bytes32,bytes` | `0x8efc7240` |
| `getMeteringSchedule(uint32)` | view | `uint64[9],uint8` | `0xa842e85b` |
| `getPendingFeeSchedule()` | view | `uint64[7],bytes32,uint64[6],uint64` | `0xe8b5cd95` |
| `getProgram(bytes32)` | view | `uint8,bytes32,bytes32,uint16,uint32,bool` | `0xb1ffb3d4` |
| `getProgramAccount(bytes32,bytes32,bytes)` | view | `bytes32,bytes32,uint64` | `0x9677b10c` |
| `getProgramAccountById(bytes32)` | view | `bytes32,bytes32,bytes` | `0x7a599bd0` |
| `getProgramOwner(bytes32)` | view | `bytes32` | `0x5d6be786` |
| `getRetirementControl()` | view | `bytes` | `0xdcee2e41` |
| `getSandbox(bytes32,uint8)` | view | `bytes` | `0xb8b4f8bc` |
| `getStateHead(bytes32)` | view | `bytes` | `0x0833650b` |
| `getValueAccount(bytes32,bytes32)` | view | `uint128,bool` | `0x24460e23` |
| `getWebRequest(bytes32,uint64)` | view | `bytes` | `0x61bc2ca4` |
| `getWindDown(bytes32)` | view | `uint8,bytes32,uint64,uint64,uint16,uint16` | `0x98e4e41d` |
| `quoteCallFee(uint64[7])` | view | `uint32,uint128,bytes32` | `0xd67f9ffa` |
| `registerAccount(bytes32,uint8,bytes32,bytes)` | nonpayable | `none` | `0xfc786007` |
| `registryRead(bytes32,bytes)` | nonpayable | `none` | `0xfb28c4b1` |
| `retireStorage(bytes)` | nonpayable | `none` | `0x1a5fe5ff` |
| `sandboxActivate(bytes32,bytes32,uint64)` | nonpayable | `none` | `0x64e1674e` |
| `sandboxExecute(bytes32,uint64,bytes32,bytes32,bytes32,bytes32,uint32,uint64[7],bytes,uint128)` | nonpayable | `int32,uint8,bytes` | `0xff04aa78` |
| `sandboxFund(bytes32,bytes32,bytes32,bytes32,bytes32,uint128,uint64,uint32,uint64[7],bytes,bytes32,(bytes32,bytes32,bytes32,uint128)[])` | nonpayable | `none` | `0xf11e5752` |
| `setExitRoute(bytes32,bytes32,bytes32,bytes32,bytes)` | nonpayable | `none` | `0x23330dd5` |
| `simulateCall(bytes32,uint16,string,bytes,bytes,bytes,uint32,uint64[7],uint128)` | view | `int32,uint8,bytes` | `0x3044bd56` |
| `tombstone(bytes32)` | nonpayable | `none` | `0xa512f1a9` |
| `transfer(bytes32,(bytes32,bytes32,bytes32,uint128)[])` | nonpayable | `none` | `0x59849fe7` |
| `upgrade(bytes32,uint16,uint8,bytes32,bytes32,bytes,bytes,bytes,uint128)` | nonpayable | `none` | `0xa860929f` |

## layerxprograms-0x101d-events

[Event encoding](../../../precompiles/layerxprograms/events.go) translates accepted kernel outcomes into EVM logs. Guest events and transfer logs are decoded from their corresponding receipt records.

| Event | Indexed fields | Topic |
|---|---|---|
| `ProgramDeployed(bytes32,bytes32)` | `programId` | `0x2765c1020c93cab47e389edaa5b5c68834bdd76f7a5d384adb460fc86e688709` |
| `ProgramUpgraded(bytes32,bytes32,bytes32)` | `programId` | `0x36b27ad7ada15e27c12e726d56c3dbd2aaa084a9250e5282c7cd38eb78a1a8ab` |
| `RegistryRead(bytes32)` | `programId` | `0x74f03c32873ceda1e22e1bff01e678b798a6e7c3f6b64ea0547a965b6c33a900` |
| `ProgramTransferred(bytes32,bytes32)` | `programId` | `0x079c617c449e4a58644f3328aa041f2e65a1b6f281b13b8719ea21c3449aac6a` |
| `ProgramGuestEvent(bytes32,bytes32,bytes32,uint64,uint8,uint32,uint16,bytes32,uint32,bytes32)` | `programId` | `0x23a84d3a8c8e7d232703d9715c64dba47a94a94c87c46fe61532e5a609e761d1` |
| `ProgramCallOutcome(bytes32,bytes32,bytes32,uint64,uint8,uint16,uint16,uint32,uint32,int32,bytes32,bytes32,bytes32,bytes32)` | `programId` | `0xcfe5297f03a2951065e463e9e0c2e3686bc0204f727eeec1a52357b5dd4a27a0` |
| `ProgramAccountRegistered(bytes32,bytes32,bytes32,bytes32,uint16,uint64)` | `programId` | `0x25814f9b8786afbe9705da19656abcc9aa1f0afc96a17e02b7a0f5a85de27b5a` |
| `ExitRouteSet(bytes32,bytes32,bytes32,bytes32)` | `programId` | `0x6b9aceca11d63f8d54de57f0d38646082c9375dc119465a34e7099ec6f92ac57` |
| `ProgramDeprecated(bytes32,uint8,uint8,bytes32,bytes32,uint64,uint64,uint16,uint16,bytes32)` | `programId` | `0x769e024ff1115e1c26a069148f332c81f2578f4ea67dde7d5076a9e9fd5e1eb8` |
| `ProgramTombstoned(bytes32,uint8,uint8,bytes32,bytes32,uint64,uint64,uint16,uint16,bytes32)` | `programId` | `0x47fc53b89ce902506c8f3a0424a87f764a54dcb5ccb9c985639991a3a55b410c` |
| `ProgramValueExited(bytes32,bytes32,bytes32)` | `programId` | `0xf07e690dd4cedc62197bec125b1418cfee68758e9c183deac3b50902e52f59ca` |
| `SandboxUsageReceipt(bytes32,uint64,uint16,uint16,uint32,bytes)` | `activityId` | `0x5c8386df0628270b0285d3f16695d6147ce4a66e048cbef0d572dae8ac4d2ba7` |
| `SandboxDestroyed(bytes32,uint8,bytes)` | `leaseId` | `0x88612a15e0cf0a2b8bfa1e89a7ad1a7284828c20680de0bb004210052506dc10` |
| `StorageRetired(bytes32,uint8,bytes)` | `activityId` | `0xb8fb07925225ab6c059c761581d51adeb4b7017ea3b0523efded11df61bed65b` |

## host-interface

The [host](../../../modules/lxphost/host.go) binds the caller's verified authority and executes kernel program activities. [The dispatcher](../../../precompiles/layerxprograms/layerxprograms.go) obtains this shared host through `HostKeepers.LayerXHost()`. Program operations use the caller's bound principal; static calls cannot perform writes.

[gasTable](../../../precompiles/layerxprograms/gas.go) supplies a KV calldata charge plus method surcharges. Deployment and upgrade use the lifecycle surcharge, execution methods use the execution surcharge, and retirement includes signature and manifest work.

## Runtime and guest ABI

The [programs workspace](../../../programs/Cargo.toml) contains the runtime, interpreter, sandbox, registry, arbiter, market and AI market crates. The [runtime FFI](../../../programs/crates/layerx-programs-runtime/src) links native execution into the host. The runtime and each registration path must use the same guest ABI version.

The AI market guest has a package-specific release profile with `opt-level = "z"`. Its deployment payload is limited to `524288` bytes including the `104`-byte envelope. The build-time chain domain is declared in [the AI market crate](../../../programs/crates/layerx-programs-ai-market).

## State and evidence

The [registry](../../../programs/crates/layerx-programs-registry/src) verifies program evidence against committed-chain proofs. The [sandbox](../../../programs/crates/layerx-programs-sandbox/src) and [arbiter](../../../programs/crates/layerx-programs-arbiter/src) consume authenticated program and transition records. A signed execution receipt and committed-block proof establish different stages of the same transaction.

The [xweb integration](../../../precompiles/layerxprograms/xweb.go) forwards guest web-request events to the xweb keeper in the program call's cached state. Payment legs and program records belong to the same transaction branch.

## Guest SDKs

Guest SDKs are provided for [Rust](../../../programs/sdk/rust), [C](../../../programs/sdk/c) and [AssemblyScript](../../../programs/sdk/assemblyscript). Their host imports, payload encodings and ABI versions must match the selected runtime. The workspace manifest identifies the included crates and separately excluded example packages.

<a id="programs-contract-layerxprograms-0x101d-the-programs-runtime-and-the-guest-sdks"></a>

<a id="contents"></a>

<a id="layerxprograms-0x101d-views"></a>

<a id="sandbox-evidence"></a>

<a id="registry-evidence"></a>

<a id="chain-proof"></a>

<a id="protocol-adapter"></a>

<a id="guest-sdk-rust"></a>

<a id="guest-sdk-c"></a>

<a id="guest-sdk-assemblyscript"></a>

<a id="wiring-integration"></a>

<a id="decisions"></a>

## frozen-abi

| Kind | Declaration | Source |
|---|---|---|
| `file_format` | `1 09fcad46aeea9659d7d555a4a09ec151bd36d60b85bfd052d1f7a43cf71d58a3` | [programs/abi-frozen.sha256](../../../programs/abi-frozen.sha256) |
| `file_format` | `2 8827869bf1324c3e82c60baf7360b1c3e2baa84d8591e49eb6b73c2cea6c8b69` | [programs/abi-frozen.sha256](../../../programs/abi-frozen.sha256) |
| `file_format` | `3 0d1c2505bb16c52cbac38f966359fafd4e930949f968484250ec96520a7de0b2` | [programs/abi-frozen.sha256](../../../programs/abi-frozen.sha256) |
| `file_format` | `4 733f27cbf903eaceab532424d13e321f6dd8939afb1f85511c7a5a12c0289aef` | [programs/abi-frozen.sha256](../../../programs/abi-frozen.sha256) |
| `file_format` | `5 07d573becee2e920b7bacb3f7d712ff017070681d46337f62e9b6987c58a2e87` | [programs/abi-frozen.sha256](../../../programs/abi-frozen.sha256) |
| source | — | [programs/sdk/abi-manifest.tsv](../../../programs/sdk/abi-manifest.tsv) |
| source | — | [programs/tests/vectors/abi-v1.hex](../../../programs/tests/vectors/abi-v1.hex) |
| source | — | [programs/tests/vectors/abi-v2.hex](../../../programs/tests/vectors/abi-v2.hex) |
| source | — | [programs/tests/vectors/abi-v3.hex](../../../programs/tests/vectors/abi-v3.hex) |
| source | — | [programs/tests/vectors/abi-v4.hex](../../../programs/tests/vectors/abi-v4.hex) |
| source | — | [programs/tests/vectors/abi-v5.hex](../../../programs/tests/vectors/abi-v5.hex) |

## fmt-abi-frozen

| Kind | Declaration | Source |
|---|---|---|
| source | — | [programs/abi-frozen.sha256](../../../programs/abi-frozen.sha256) |

## runtime-ffi

| Kind | Declaration | Source |
|---|---|---|
| `rust_pub_item` | `pub unsafe extern "C" fn layerx_programs_call_arbiter_authority_check(` | [programs/crates/layerx-programs-runtime/src/ffi_call.rs](../../../programs/crates/layerx-programs-runtime/src/ffi_call.rs) |

## runtime-crate

| Kind | Declaration | Source |
|---|---|---|
| `rust_pub_item` | `pub fn canonical_program_event_envelope(&self) -> Result<Vec<u8>, AbiError>` | [programs/crates/layerx-programs-runtime/src/abi/mod.rs](../../../programs/crates/layerx-programs-runtime/src/abi/mod.rs) |
| `rust_pub_item` | `pub const fn canonical_bytes(&self) -> &[u8; RAW_NATIVE_RECEIPT_AUTHORITY_V2_BYTES] {` | [programs/crates/layerx-programs-runtime/src/ffi_call.rs](../../../programs/crates/layerx-programs-runtime/src/ffi_call.rs) |
| `rust_pub_item` | `pub unsafe fn read_native_receipt_authority(` | [programs/crates/layerx-programs-runtime/src/ffi_call.rs](../../../programs/crates/layerx-programs-runtime/src/ffi_call.rs) |

## interpreter-guest

| Kind | Declaration | Source |
|---|---|---|
| `rust_pub_item` | `pub const MAGIC: [u8; 4] = *b"LXSI"` | [programs/crates/layerx-programs-interpreter/src/lib.rs](../../../programs/crates/layerx-programs-interpreter/src/lib.rs) |
| `rust_pub_item` | `pub const VERSION: u8 = 1` | [programs/crates/layerx-programs-interpreter/src/lib.rs](../../../programs/crates/layerx-programs-interpreter/src/lib.rs) |
| `rust_pub_item` | `pub const MAX_SCRIPT_BYTES: usize = 4_096` | [programs/crates/layerx-programs-interpreter/src/lib.rs](../../../programs/crates/layerx-programs-interpreter/src/lib.rs) |
| `rust_pub_item` | `pub const MAX_REGISTERS: usize = 16` | [programs/crates/layerx-programs-interpreter/src/lib.rs](../../../programs/crates/layerx-programs-interpreter/src/lib.rs) |
| `rust_pub_item` | `pub const MAX_STEPS: u32 = 4_096` | [programs/crates/layerx-programs-interpreter/src/lib.rs](../../../programs/crates/layerx-programs-interpreter/src/lib.rs) |
| `rust_pub_item` | `pub const MAX_CONTROL_DEPTH: u8 = 4` | [programs/crates/layerx-programs-interpreter/src/lib.rs](../../../programs/crates/layerx-programs-interpreter/src/lib.rs) |
| `rust_pub_item` | `pub const HEADER_BYTES: usize = 10` | [programs/crates/layerx-programs-interpreter/src/lib.rs](../../../programs/crates/layerx-programs-interpreter/src/lib.rs) |
| `rust_pub_item` | `pub enum Instruction<'a>` | [programs/crates/layerx-programs-interpreter/src/lib.rs](../../../programs/crates/layerx-programs-interpreter/src/lib.rs) |
| `rust_pub_item` | `pub struct Interpreter<'a>` | [programs/crates/layerx-programs-interpreter/src/lib.rs](../../../programs/crates/layerx-programs-interpreter/src/lib.rs) |
| `rust_pub_item` | `pub fn validate(script: &'a [u8]) -> Result<Self, ProgramError>` | [programs/crates/layerx-programs-interpreter/src/lib.rs](../../../programs/crates/layerx-programs-interpreter/src/lib.rs) |
| `rust_pub_item` | `pub const fn maximum_steps(self) -> u32` | [programs/crates/layerx-programs-interpreter/src/lib.rs](../../../programs/crates/layerx-programs-interpreter/src/lib.rs) |
| `rust_pub_item` | `pub const fn register_count(self) -> u8` | [programs/crates/layerx-programs-interpreter/src/lib.rs](../../../programs/crates/layerx-programs-interpreter/src/lib.rs) |

## sandbox-crate

| Kind | Declaration | Source |
|---|---|---|
| `rust_pub_item` | `pub struct SandboxSubmissionReceipt` | [programs/crates/layerx-programs-sandbox/src/activity.rs](../../../programs/crates/layerx-programs-sandbox/src/activity.rs) |
| `rust_pub_item` | `pub struct SandboxReceiptEvidence` | [programs/crates/layerx-programs-sandbox/src/activity.rs](../../../programs/crates/layerx-programs-sandbox/src/activity.rs) |
| `rust_pub_item` | `pub fn verify<E: Into<crate::invocation::SandboxReceiptProof>>(` | [programs/crates/layerx-programs-sandbox/src/activity.rs](../../../programs/crates/layerx-programs-sandbox/src/activity.rs) |
| `rust_pub_item` | `pub const fn activity_id(&self) -> [u8; 32]` | [programs/crates/layerx-programs-sandbox/src/activity.rs](../../../programs/crates/layerx-programs-sandbox/src/activity.rs) |
| `rust_pub_item` | `pub fn authenticated_usage_receipt(` | [programs/crates/layerx-programs-sandbox/src/activity.rs](../../../programs/crates/layerx-programs-sandbox/src/activity.rs) |
| `rust_pub_item` | `pub const MAX_SWEEP_LEASES_PER_BATCH: u32 = 128;` | [programs/crates/layerx-programs-sandbox/src/expiry.rs](../../../programs/crates/layerx-programs-sandbox/src/expiry.rs) |
| `rust_pub_item` | `pub struct TerminalReceiptEvidence` | [programs/crates/layerx-programs-sandbox/src/expiry.rs](../../../programs/crates/layerx-programs-sandbox/src/expiry.rs) |
| `rust_pub_item` | `pub fn verify(` | [programs/crates/layerx-programs-sandbox/src/expiry.rs](../../../programs/crates/layerx-programs-sandbox/src/expiry.rs) |
| `rust_pub_item` | `pub struct LeaseTransition` | [programs/crates/layerx-programs-sandbox/src/lease.rs](../../../programs/crates/layerx-programs-sandbox/src/lease.rs) |
| `rust_pub_item` | `pub struct TransitionEvidence` | [programs/crates/layerx-programs-sandbox/src/lease.rs](../../../programs/crates/layerx-programs-sandbox/src/lease.rs) |
| `rust_pub_item` | `pub fn verify_call(` | [programs/crates/layerx-programs-sandbox/src/lease.rs](../../../programs/crates/layerx-programs-sandbox/src/lease.rs) |

## registry-crate

| Kind | Declaration | Source |
|---|---|---|
| `rust_pub_item` | `pub struct StateLeafWitness` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub struct ProgramLifecycleProof(pub ChainLifecycleProof);` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub struct ProgramStateProof` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub struct DeploymentProof` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub struct ProtocolHeadProof<'a>` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub enum ProtocolEvidenceError` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub struct VerifiedDeploymentEvidence` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub const fn program(&self) -> ProgramId` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub const fn version(&self) -> u32` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub const fn code_hash(&self) -> [u8; 32]` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub const fn abi_version(&self) -> u16` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub const fn interface_present(&self) -> bool` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub const fn interface(&self) -> Option<&crate::ProgramInterface>` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub const fn receipt_digest(&self) -> [u8; 32]` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub const fn activity_id(&self) -> [u8; 32]` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub const fn state_root(&self) -> [u8; 32]` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub const fn programs_root(&self) -> [u8; 32]` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub const fn freshness(&self) -> ReadFreshness` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub const fn lifecycle(&self) -> ProgramLifecycle` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub fn module(&self) -> &[u8]` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub const fn record(&self) -> &DeploymentRecord` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub const fn proof(&self) -> &DeploymentProof` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub struct VerifiedProgramHead` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub const fn policy(&self) -> UpgradePolicy` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub const fn valid_until_ms(&self) -> u64` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub struct VerifiedProtocolHead` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub enum ProgramHeadKind` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub const fn wire_byte(self) -> u8` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub const fn from_wire_byte(byte: u8) -> Option<Self>` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub enum ReceiptEvidenceDigest` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub const fn bytes(self) -> [u8; 32]` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub const fn head_kind(self) -> ProgramHeadKind` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub struct VerifiedChainHead` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub const fn head_kind(&self) -> ProgramHeadKind` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub const fn receipt_evidence_digest(&self) -> ReceiptEvidenceDigest` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub struct ProgramStateBundle` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub struct VerifiedProgramBundle` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub const fn program_head(&self) -> &VerifiedProgramHead` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub const fn chain_head(&self) -> &VerifiedChainHead` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub const fn interface(&self) -> &VerifiedInterfaceRead` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub enum ProgramBundleError` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub struct ProtocolDeploymentVerifier` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub const fn staleness_limit_ms(&self) -> u64` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub fn verify_deployment(` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub fn verify_historical_deployment(` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub fn verify_current_program(` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub fn verify_current_protocol_head(` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub fn verify_historical_protocol_head(` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub fn verify_current_protocol_head_proof(` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub fn verify_historical_protocol_head_proof(` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub fn verify_current_chain_head(` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub fn verify_current_program_bundle(` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub fn canonical_encoding(&self) -> Result<Vec<u8>, ProgramBundleError> {` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub fn decode(bytes: &[u8]) -> Result<Self, ProtocolEvidenceError>` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |
| `rust_pub_item` | `pub fn claimed_receipt_digest(&self) -> Result<[u8; 32], ProtocolEvidenceError>` | [programs/crates/layerx-programs-registry/src/protocol_evidence.rs](../../../programs/crates/layerx-programs-registry/src/protocol_evidence.rs) |

## arbiter-crate

| Kind | Declaration | Source |
|---|---|---|
| `rust_pub_item` | `pub struct VerifiedReplayAuthority` | [programs/crates/layerx-programs-arbiter/src/authority.rs](../../../programs/crates/layerx-programs-arbiter/src/authority.rs) |
| `rust_pub_item` | `pub fn derive(` | [programs/crates/layerx-programs-arbiter/src/authority.rs](../../../programs/crates/layerx-programs-arbiter/src/authority.rs) |
| `rust_pub_item` | `pub fn derive_program_call(` | [programs/crates/layerx-programs-arbiter/src/authority.rs](../../../programs/crates/layerx-programs-arbiter/src/authority.rs) |
| `rust_pub_item` | `pub fn verify_native(&self, bytes: &[u8]) -> Result<(), ReplayError>` | [programs/crates/layerx-programs-arbiter/src/authority.rs](../../../programs/crates/layerx-programs-arbiter/src/authority.rs) |
| `rust_pub_item` | `pub fn canonical_bytes(&self) -> &[u8]` | [programs/crates/layerx-programs-arbiter/src/authority.rs](../../../programs/crates/layerx-programs-arbiter/src/authority.rs) |
| `rust_pub_item` | `pub const fn principal(&self) -> [u8; 32]` | [programs/crates/layerx-programs-arbiter/src/authority.rs](../../../programs/crates/layerx-programs-arbiter/src/authority.rs) |
| `rust_pub_item` | `pub const fn payer(&self) -> [u8; 32]` | [programs/crates/layerx-programs-arbiter/src/authority.rs](../../../programs/crates/layerx-programs-arbiter/src/authority.rs) |
| `rust_pub_item` | `pub const fn activity_binding(&self) -> [u8; 32]` | [programs/crates/layerx-programs-arbiter/src/authority.rs](../../../programs/crates/layerx-programs-arbiter/src/authority.rs) |
| `rust_pub_item` | `pub fn capabilities(&self) -> &CapabilitySet` | [programs/crates/layerx-programs-arbiter/src/authority.rs](../../../programs/crates/layerx-programs-arbiter/src/authority.rs) |
| `rust_pub_item` | `pub struct TraceCommitment` | [programs/crates/layerx-programs-arbiter/src/bisect.rs](../../../programs/crates/layerx-programs-arbiter/src/bisect.rs) |
| `rust_pub_item` | `pub fn open_leaf(&self, proof: &BoundaryProof) -> Option<PortableBoundary>` | [programs/crates/layerx-programs-arbiter/src/bisect.rs](../../../programs/crates/layerx-programs-arbiter/src/bisect.rs) |
| `rust_pub_item` | `pub struct CommittedTrace` | [programs/crates/layerx-programs-arbiter/src/bisect.rs](../../../programs/crates/layerx-programs-arbiter/src/bisect.rs) |
| `rust_pub_item` | `pub fn new(leaves: Vec<Vec<u8>>) -> Result<Self, GameError>` | [programs/crates/layerx-programs-arbiter/src/bisect.rs](../../../programs/crates/layerx-programs-arbiter/src/bisect.rs) |
| `rust_pub_item` | `pub fn root(&self) -> [u8; 32]` | [programs/crates/layerx-programs-arbiter/src/bisect.rs](../../../programs/crates/layerx-programs-arbiter/src/bisect.rs) |
| `rust_pub_item` | `pub fn boundary_count(&self) -> u32` | [programs/crates/layerx-programs-arbiter/src/bisect.rs](../../../programs/crates/layerx-programs-arbiter/src/bisect.rs) |
| `rust_pub_item` | `pub fn proof(&self, index: u32) -> Option<BoundaryProof>` | [programs/crates/layerx-programs-arbiter/src/bisect.rs](../../../programs/crates/layerx-programs-arbiter/src/bisect.rs) |
| `rust_pub_item` | `pub trait StepJudge` | [programs/crates/layerx-programs-arbiter/src/bisect.rs](../../../programs/crates/layerx-programs-arbiter/src/bisect.rs) |
| `rust_pub_item` | `fn trace(&self) -> TraceCommitment` | [programs/crates/layerx-programs-arbiter/src/bisect.rs](../../../programs/crates/layerx-programs-arbiter/src/bisect.rs) |
| `rust_pub_item` | `fn step_holds(&self, pre: &BoundaryProof, post: &BoundaryProof) -> bool` | [programs/crates/layerx-programs-arbiter/src/bisect.rs](../../../programs/crates/layerx-programs-arbiter/src/bisect.rs) |
| `rust_pub_item` | `pub struct SandboxStepJudge<'a>` | [programs/crates/layerx-programs-arbiter/src/bisect.rs](../../../programs/crates/layerx-programs-arbiter/src/bisect.rs) |
| `rust_pub_item` | `pub fn new(leaves: Vec<Vec<u8>>) -> Result<Self, GameError> {` | [programs/crates/layerx-programs-arbiter/src/bisect.rs](../../../programs/crates/layerx-programs-arbiter/src/bisect.rs) |
| `rust_pub_item` | `pub fn same_state(claimed: &PortableBoundary, truth: &PortableBoundary, maximum: u32) -> bool` | [programs/crates/layerx-programs-arbiter/src/bisect.rs](../../../programs/crates/layerx-programs-arbiter/src/bisect.rs) |
| `rust_pub_item` | `pub const fn max_rounds(boundary_count: u32) -> u32` | [programs/crates/layerx-programs-arbiter/src/bisect.rs](../../../programs/crates/layerx-programs-arbiter/src/bisect.rs) |
| `rust_pub_item` | `pub enum GameError` | [programs/crates/layerx-programs-arbiter/src/bisect.rs](../../../programs/crates/layerx-programs-arbiter/src/bisect.rs) |
| `rust_pub_item` | `pub enum Party` | [programs/crates/layerx-programs-arbiter/src/bisect.rs](../../../programs/crates/layerx-programs-arbiter/src/bisect.rs) |
| `rust_pub_item` | `pub enum Awaiting` | [programs/crates/layerx-programs-arbiter/src/bisect.rs](../../../programs/crates/layerx-programs-arbiter/src/bisect.rs) |
| `rust_pub_item` | `pub enum MoveKind` | [programs/crates/layerx-programs-arbiter/src/bisect.rs](../../../programs/crates/layerx-programs-arbiter/src/bisect.rs) |
| `rust_pub_item` | `pub struct Move` | [programs/crates/layerx-programs-arbiter/src/bisect.rs](../../../programs/crates/layerx-programs-arbiter/src/bisect.rs) |
| `rust_pub_item` | `pub enum Reason` | [programs/crates/layerx-programs-arbiter/src/bisect.rs](../../../programs/crates/layerx-programs-arbiter/src/bisect.rs) |
| `rust_pub_item` | `pub struct Stakes` | [programs/crates/layerx-programs-arbiter/src/bisect.rs](../../../programs/crates/layerx-programs-arbiter/src/bisect.rs) |
| `rust_pub_item` | `pub struct Settlement` | [programs/crates/layerx-programs-arbiter/src/bisect.rs](../../../programs/crates/layerx-programs-arbiter/src/bisect.rs) |
| `rust_pub_item` | `pub struct BisectionGame<J>` | [programs/crates/layerx-programs-arbiter/src/bisect.rs](../../../programs/crates/layerx-programs-arbiter/src/bisect.rs) |
| `rust_pub_item` | `pub fn open(judge: J, stakes: Stakes, window: u64, height: u64) -> Result<Self, GameError>` | [programs/crates/layerx-programs-arbiter/src/bisect.rs](../../../programs/crates/layerx-programs-arbiter/src/bisect.rs) |
| `rust_pub_item` | `pub fn trace(&self) -> TraceCommitment` | [programs/crates/layerx-programs-arbiter/src/bisect.rs](../../../programs/crates/layerx-programs-arbiter/src/bisect.rs) |
| `rust_pub_item` | `pub fn stakes(&self) -> Stakes` | [programs/crates/layerx-programs-arbiter/src/bisect.rs](../../../programs/crates/layerx-programs-arbiter/src/bisect.rs) |
| `rust_pub_item` | `pub fn awaiting(&self) -> Option<Awaiting>` | [programs/crates/layerx-programs-arbiter/src/bisect.rs](../../../programs/crates/layerx-programs-arbiter/src/bisect.rs) |
| `rust_pub_item` | `pub fn deadline(&self) -> u64` | [programs/crates/layerx-programs-arbiter/src/bisect.rs](../../../programs/crates/layerx-programs-arbiter/src/bisect.rs) |
| `rust_pub_item` | `pub fn interval(&self) -> (u32, u32)` | [programs/crates/layerx-programs-arbiter/src/bisect.rs](../../../programs/crates/layerx-programs-arbiter/src/bisect.rs) |
| `rust_pub_item` | `pub fn rounds(&self) -> u32` | [programs/crates/layerx-programs-arbiter/src/bisect.rs](../../../programs/crates/layerx-programs-arbiter/src/bisect.rs) |
| `rust_pub_item` | `pub fn record(&self) -> &[Move]` | [programs/crates/layerx-programs-arbiter/src/bisect.rs](../../../programs/crates/layerx-programs-arbiter/src/bisect.rs) |
| `rust_pub_item` | `pub fn settlement(&self) -> Option<&Settlement>` | [programs/crates/layerx-programs-arbiter/src/bisect.rs](../../../programs/crates/layerx-programs-arbiter/src/bisect.rs) |
| `rust_pub_item` | `pub fn advance(&mut self, height: u64) -> Result<Option<&Settlement>, GameError>` | [programs/crates/layerx-programs-arbiter/src/bisect.rs](../../../programs/crates/layerx-programs-arbiter/src/bisect.rs) |
| `rust_pub_item` | `pub fn reveal(&mut self, height: u64, proof: BoundaryProof) -> Result<(), GameError>` | [programs/crates/layerx-programs-arbiter/src/bisect.rs](../../../programs/crates/layerx-programs-arbiter/src/bisect.rs) |
| `rust_pub_item` | `pub fn respond(&mut self, height: u64, agree: bool) -> Result<(), GameError>` | [programs/crates/layerx-programs-arbiter/src/bisect.rs](../../../programs/crates/layerx-programs-arbiter/src/bisect.rs) |
| `rust_pub_item` | `pub enum MarketEvidenceError` | [programs/crates/layerx-programs-arbiter/src/market.rs](../../../programs/crates/layerx-programs-arbiter/src/market.rs) |
| `rust_pub_item` | `pub enum MarketStepVerdict` | [programs/crates/layerx-programs-arbiter/src/market.rs](../../../programs/crates/layerx-programs-arbiter/src/market.rs) |
| `rust_pub_item` | `pub struct NativeNamespaceProof<'a>` | [programs/crates/layerx-programs-arbiter/src/market.rs](../../../programs/crates/layerx-programs-arbiter/src/market.rs) |
| `rust_pub_item` | `pub struct MarketReplayEvidence<'a>` | [programs/crates/layerx-programs-arbiter/src/market.rs](../../../programs/crates/layerx-programs-arbiter/src/market.rs) |
| `rust_pub_item` | `pub struct VerifiedMarketSandbox` | [programs/crates/layerx-programs-arbiter/src/market.rs](../../../programs/crates/layerx-programs-arbiter/src/market.rs) |
| `rust_pub_item` | `pub fn verify(evidence: MarketReplayEvidence<'_>) -> Result<Self, MarketEvidenceError>` | [programs/crates/layerx-programs-arbiter/src/market.rs](../../../programs/crates/layerx-programs-arbiter/src/market.rs) |
| `rust_pub_item` | `pub fn profile(&self) -> &MarketSandboxProfile` | [programs/crates/layerx-programs-arbiter/src/market.rs](../../../programs/crates/layerx-programs-arbiter/src/market.rs) |
| `rust_pub_item` | `pub fn billing(&self) -> &MarketBillingCommitment` | [programs/crates/layerx-programs-arbiter/src/market.rs](../../../programs/crates/layerx-programs-arbiter/src/market.rs) |
| `rust_pub_item` | `pub fn verify_step(&self, pre: &BoundaryProof, post: &BoundaryProof) -> MarketStepVerdict` | [programs/crates/layerx-programs-arbiter/src/market.rs](../../../programs/crates/layerx-programs-arbiter/src/market.rs) |
| `rust_pub_item` | `pub enum ReplayError` | [programs/crates/layerx-programs-arbiter/src/step.rs](../../../programs/crates/layerx-programs-arbiter/src/step.rs) |
| `rust_pub_item` | `pub struct ReplayMetadata` | [programs/crates/layerx-programs-arbiter/src/step.rs](../../../programs/crates/layerx-programs-arbiter/src/step.rs) |
| `rust_pub_item` | `pub fn decode_untrusted(bytes: &[u8]) -> Result<Self, ReplayError>` | [programs/crates/layerx-programs-arbiter/src/step.rs](../../../programs/crates/layerx-programs-arbiter/src/step.rs) |
| `rust_pub_item` | `pub struct AuthenticatedCatalogue` | [programs/crates/layerx-programs-arbiter/src/step.rs](../../../programs/crates/layerx-programs-arbiter/src/step.rs) |
| `rust_pub_item` | `pub fn verify(` | [programs/crates/layerx-programs-arbiter/src/step.rs](../../../programs/crates/layerx-programs-arbiter/src/step.rs) |
| `rust_pub_item` | `pub fn verify_with_module(` | [programs/crates/layerx-programs-arbiter/src/step.rs](../../../programs/crates/layerx-programs-arbiter/src/step.rs) |
| `rust_pub_item` | `pub struct BoundaryProof` | [programs/crates/layerx-programs-arbiter/src/step.rs](../../../programs/crates/layerx-programs-arbiter/src/step.rs) |
| `rust_pub_item` | `pub struct VerifiedReplay` | [programs/crates/layerx-programs-arbiter/src/step.rs](../../../programs/crates/layerx-programs-arbiter/src/step.rs) |
| `rust_pub_item` | `pub struct VerifiedStep` | [programs/crates/layerx-programs-arbiter/src/step.rs](../../../programs/crates/layerx-programs-arbiter/src/step.rs) |
| `rust_pub_item` | `pub const fn activity_id(self) -> [u8; 32]` | [programs/crates/layerx-programs-arbiter/src/step.rs](../../../programs/crates/layerx-programs-arbiter/src/step.rs) |
| `rust_pub_item` | `pub const fn pre_index(self) -> u32` | [programs/crates/layerx-programs-arbiter/src/step.rs](../../../programs/crates/layerx-programs-arbiter/src/step.rs) |
| `rust_pub_item` | `pub const fn post_index(self) -> u32` | [programs/crates/layerx-programs-arbiter/src/step.rs](../../../programs/crates/layerx-programs-arbiter/src/step.rs) |
| `rust_pub_item` | `pub fn verify_with_batch_context(` | [programs/crates/layerx-programs-arbiter/src/step.rs](../../../programs/crates/layerx-programs-arbiter/src/step.rs) |
| `rust_pub_item` | `pub fn metadata(&self) -> &ReplayMetadata` | [programs/crates/layerx-programs-arbiter/src/step.rs](../../../programs/crates/layerx-programs-arbiter/src/step.rs) |
| `rust_pub_item` | `pub fn verify_step(` | [programs/crates/layerx-programs-arbiter/src/step.rs](../../../programs/crates/layerx-programs-arbiter/src/step.rs) |

## market-guest

| Kind | Declaration | Source |
|---|---|---|
| `rust_pub_item` | `pub const AUTHORIZE_SANDBOX_PROFILE: u8 = 13` | [programs/crates/layerx-programs-market/src/arbitration.rs](../../../programs/crates/layerx-programs-market/src/arbitration.rs) |
| `rust_pub_item` | `pub const COMMIT_SANDBOX_BILLING: u8 = 14` | [programs/crates/layerx-programs-market/src/arbitration.rs](../../../programs/crates/layerx-programs-market/src/arbitration.rs) |
| `rust_pub_item` | `pub const PROFILE_PREFIX: &[u8] = b"lx.market.arbitration-profile/"` | [programs/crates/layerx-programs-market/src/arbitration.rs](../../../programs/crates/layerx-programs-market/src/arbitration.rs) |
| `rust_pub_item` | `pub const BILLING_PREFIX: &[u8] = b"lx.market.arbitration-billing/"` | [programs/crates/layerx-programs-market/src/arbitration.rs](../../../programs/crates/layerx-programs-market/src/arbitration.rs) |
| `rust_pub_item` | `pub const TOPIC_PROFILE: &[u8] = b"lx.market.arbitration-profile"` | [programs/crates/layerx-programs-market/src/arbitration.rs](../../../programs/crates/layerx-programs-market/src/arbitration.rs) |
| `rust_pub_item` | `pub const TOPIC_BILLING: &[u8] = b"lx.market.arbitration-billing"` | [programs/crates/layerx-programs-market/src/arbitration.rs](../../../programs/crates/layerx-programs-market/src/arbitration.rs) |
| `rust_pub_item` | `pub const NAMESPACE_DOMAIN: &[u8] = b"LayerX/programs/sandbox/namespace/v1\0";` | [programs/crates/layerx-programs-market/src/arbitration.rs](../../../programs/crates/layerx-programs-market/src/arbitration.rs) |
| `rust_pub_item` | `pub fn authorize_profile(` | [programs/crates/layerx-programs-market/src/arbitration.rs](../../../programs/crates/layerx-programs-market/src/arbitration.rs) |
| `rust_pub_item` | `pub fn billing_claim(` | [programs/crates/layerx-programs-market/src/arbitration.rs](../../../programs/crates/layerx-programs-market/src/arbitration.rs) |
| `rust_pub_item` | `pub fn matches_claim(` | [programs/crates/layerx-programs-market/src/arbitration.rs](../../../programs/crates/layerx-programs-market/src/arbitration.rs) |
| `rust_pub_item` | `pub const MAX_ATTESTERS: usize = 8` | [programs/crates/layerx-programs-market/src/attest.rs](../../../programs/crates/layerx-programs-market/src/attest.rs) |
| `rust_pub_item` | `pub const MAX_ATTESTER_NAME_BYTES: usize = 32` | [programs/crates/layerx-programs-market/src/attest.rs](../../../programs/crates/layerx-programs-market/src/attest.rs) |
| `rust_pub_item` | `pub const MAX_INPUTS_PER_LEASE: u32 = 1_024` | [programs/crates/layerx-programs-market/src/attest.rs](../../../programs/crates/layerx-programs-market/src/attest.rs) |
| `rust_pub_item` | `pub const ATTESTER_SET_CAPACITY: usize = 252 + MAX_ATTESTERS * (33 + MAX_ATTESTER_NAME_BYTES)` | [programs/crates/layerx-programs-market/src/attest.rs](../../../programs/crates/layerx-programs-market/src/attest.rs) |
| `rust_pub_item` | `pub const ATTESTED_INPUT_CAPACITY: usize = 244` | [programs/crates/layerx-programs-market/src/attest.rs](../../../programs/crates/layerx-programs-market/src/attest.rs) |
| `rust_pub_item` | `pub const ATTESTATION_STATEMENT_CAPACITY: usize = 256` | [programs/crates/layerx-programs-market/src/attest.rs](../../../programs/crates/layerx-programs-market/src/attest.rs) |
| `rust_pub_item` | `pub const ATTESTATION_REQUEST_CAPACITY: usize = 242` | [programs/crates/layerx-programs-market/src/attest.rs](../../../programs/crates/layerx-programs-market/src/attest.rs) |
| `rust_pub_item` | `pub const ATTESTATION_SIGNATURE_BYTES: usize = 64` | [programs/crates/layerx-programs-market/src/attest.rs](../../../programs/crates/layerx-programs-market/src/attest.rs) |
| `rust_pub_item` | `pub type LeaseId = [u8; 32]` | [programs/crates/layerx-programs-market/src/attest.rs](../../../programs/crates/layerx-programs-market/src/attest.rs) |
| `rust_pub_item` | `pub enum ExternalInputSource` | [programs/crates/layerx-programs-market/src/attest.rs](../../../programs/crates/layerx-programs-market/src/attest.rs) |
| `rust_pub_item` | `pub enum EvidenceClass` | [programs/crates/layerx-programs-market/src/attest.rs](../../../programs/crates/layerx-programs-market/src/attest.rs) |
| `rust_pub_item` | `pub enum InputStatus` | [programs/crates/layerx-programs-market/src/attest.rs](../../../programs/crates/layerx-programs-market/src/attest.rs) |
| `rust_pub_item` | `pub struct Attester<'a>` | [programs/crates/layerx-programs-market/src/attest.rs](../../../programs/crates/layerx-programs-market/src/attest.rs) |
| `rust_pub_item` | `pub struct AttesterSet<'a>` | [programs/crates/layerx-programs-market/src/attest.rs](../../../programs/crates/layerx-programs-market/src/attest.rs) |
| `rust_pub_item` | `pub fn new(` | [programs/crates/layerx-programs-market/src/attest.rs](../../../programs/crates/layerx-programs-market/src/attest.rs) |
| `rust_pub_item` | `pub fn named_key(&self, name: &[u8]) -> Result<[u8; 32], ProgramError>` | [programs/crates/layerx-programs-market/src/attest.rs](../../../programs/crates/layerx-programs-market/src/attest.rs) |
| `rust_pub_item` | `pub fn ready_for_settlement(&self) -> bool` | [programs/crates/layerx-programs-market/src/attest.rs](../../../programs/crates/layerx-programs-market/src/attest.rs) |
| `rust_pub_item` | `pub fn settlement_input_commitment(&self) -> Result<[u8; 32], ProgramError>` | [programs/crates/layerx-programs-market/src/attest.rs](../../../programs/crates/layerx-programs-market/src/attest.rs) |
| `rust_pub_item` | `pub struct InputCommitment` | [programs/crates/layerx-programs-market/src/attest.rs](../../../programs/crates/layerx-programs-market/src/attest.rs) |
| `rust_pub_item` | `pub struct Attestation<'a>` | [programs/crates/layerx-programs-market/src/attest.rs](../../../programs/crates/layerx-programs-market/src/attest.rs) |
| `rust_pub_item` | `pub struct AttestedInput` | [programs/crates/layerx-programs-market/src/attest.rs](../../../programs/crates/layerx-programs-market/src/attest.rs) |
| `rust_pub_item` | `pub fn encode_attestation(` | [programs/crates/layerx-programs-market/src/attest.rs](../../../programs/crates/layerx-programs-market/src/attest.rs) |
| `rust_pub_item` | `pub fn decode_attestation(input: &[u8]) -> Result<Attestation<'_>, ProgramError>` | [programs/crates/layerx-programs-market/src/attest.rs](../../../programs/crates/layerx-programs-market/src/attest.rs) |
| `rust_pub_item` | `pub fn commit_input(` | [programs/crates/layerx-programs-market/src/attest.rs](../../../programs/crates/layerx-programs-market/src/attest.rs) |
| `rust_pub_item` | `pub fn seal_inputs(` | [programs/crates/layerx-programs-market/src/attest.rs](../../../programs/crates/layerx-programs-market/src/attest.rs) |
| `rust_pub_item` | `pub fn encode_policy(policy: &AttesterSet<'_>, output: &mut [u8]) -> Result<usize, ProgramError>` | [programs/crates/layerx-programs-market/src/attest.rs](../../../programs/crates/layerx-programs-market/src/attest.rs) |
| `rust_pub_item` | `pub fn decode_policy(input: &[u8]) -> Result<AttesterSet<'_>, ProgramError>` | [programs/crates/layerx-programs-market/src/attest.rs](../../../programs/crates/layerx-programs-market/src/attest.rs) |
| `rust_pub_item` | `pub fn encode_input(input: AttestedInput, output: &mut [u8]) -> Result<usize, ProgramError>` | [programs/crates/layerx-programs-market/src/attest.rs](../../../programs/crates/layerx-programs-market/src/attest.rs) |
| `rust_pub_item` | `pub fn decode_input(input: &[u8]) -> Result<AttestedInput, ProgramError>` | [programs/crates/layerx-programs-market/src/attest.rs](../../../programs/crates/layerx-programs-market/src/attest.rs) |
| `rust_pub_item` | `pub fn statement_bytes(` | [programs/crates/layerx-programs-market/src/attest.rs](../../../programs/crates/layerx-programs-market/src/attest.rs) |
| `rust_pub_item` | `pub fn encode_policy_commitment(` | [programs/crates/layerx-programs-market/src/attest.rs](../../../programs/crates/layerx-programs-market/src/attest.rs) |
| `rust_pub_item` | `pub fn configure(` | [programs/crates/layerx-programs-market/src/attest.rs](../../../programs/crates/layerx-programs-market/src/attest.rs) |
| `rust_pub_item` | `pub fn commit(commitment: InputCommitment, principal: AccountId) -> Result<(), ProgramError>` | [programs/crates/layerx-programs-market/src/attest.rs](../../../programs/crates/layerx-programs-market/src/attest.rs) |
| `rust_pub_item` | `pub fn seal(lease_id: LeaseId, principal: AccountId, height: u64) -> Result<(), ProgramError>` | [programs/crates/layerx-programs-market/src/attest.rs](../../../programs/crates/layerx-programs-market/src/attest.rs) |
| `rust_pub_item` | `pub fn admit(attestation: Attestation<'_>, height: u64) -> Result<(), ProgramError>` | [programs/crates/layerx-programs-market/src/attest.rs](../../../programs/crates/layerx-programs-market/src/attest.rs) |
| `rust_pub_item` | `pub fn require_ready_commitment(lease_id: LeaseId) -> Result<[u8; 32], ProgramError>` | [programs/crates/layerx-programs-market/src/attest.rs](../../../programs/crates/layerx-programs-market/src/attest.rs) |
| `rust_pub_item` | `pub const REVEAL_BOUNDARY: u8 = 15` | [programs/crates/layerx-programs-market/src/dispute.rs](../../../programs/crates/layerx-programs-market/src/dispute.rs) |
| `rust_pub_item` | `pub const RESPOND_BOUNDARY: u8 = 16` | [programs/crates/layerx-programs-market/src/dispute.rs](../../../programs/crates/layerx-programs-market/src/dispute.rs) |
| `rust_pub_item` | `pub const RESOLVE_DISPUTE: u8 = 17` | [programs/crates/layerx-programs-market/src/dispute.rs](../../../programs/crates/layerx-programs-market/src/dispute.rs) |
| `rust_pub_item` | `pub const DISPUTE_PREFIX: &[u8] = b"lx.market.dispute/"` | [programs/crates/layerx-programs-market/src/dispute.rs](../../../programs/crates/layerx-programs-market/src/dispute.rs) |
| `rust_pub_item` | `pub const TOPIC_DISPUTE: &[u8] = b"lx.market.dispute"` | [programs/crates/layerx-programs-market/src/dispute.rs](../../../programs/crates/layerx-programs-market/src/dispute.rs) |
| `rust_pub_item` | `pub const TRANSCRIPT_DOMAIN: &[u8] = b"LXP/market-dispute-transcript/v1\0";` | [programs/crates/layerx-programs-market/src/dispute.rs](../../../programs/crates/layerx-programs-market/src/dispute.rs) |
| `rust_pub_item` | `pub const DISPUTE_CAPACITY: usize = 80` | [programs/crates/layerx-programs-market/src/dispute.rs](../../../programs/crates/layerx-programs-market/src/dispute.rs) |
| `rust_pub_item` | `pub const MOVE_BYTES: usize = 46` | [programs/crates/layerx-programs-market/src/dispute.rs](../../../programs/crates/layerx-programs-market/src/dispute.rs) |
| `rust_pub_item` | `pub const TRANSCRIPT_PREIMAGE_BYTES: usize = TRANSCRIPT_DOMAIN.len() + 32 + MOVE_BYTES` | [programs/crates/layerx-programs-market/src/dispute.rs](../../../programs/crates/layerx-programs-market/src/dispute.rs) |
| `rust_pub_item` | `pub const MAX_BOUNDARIES: u32 = 4096` | [programs/crates/layerx-programs-market/src/dispute.rs](../../../programs/crates/layerx-programs-market/src/dispute.rs) |
| `rust_pub_item` | `pub enum Party` | [programs/crates/layerx-programs-market/src/dispute.rs](../../../programs/crates/layerx-programs-market/src/dispute.rs) |
| `rust_pub_item` | `pub enum MoveKind` | [programs/crates/layerx-programs-market/src/dispute.rs](../../../programs/crates/layerx-programs-market/src/dispute.rs) |
| `rust_pub_item` | `pub enum Resolution` | [programs/crates/layerx-programs-market/src/dispute.rs](../../../programs/crates/layerx-programs-market/src/dispute.rs) |
| `rust_pub_item` | `pub enum Awaiting` | [programs/crates/layerx-programs-market/src/dispute.rs](../../../programs/crates/layerx-programs-market/src/dispute.rs) |
| `rust_pub_item` | `pub struct Move` | [programs/crates/layerx-programs-market/src/dispute.rs](../../../programs/crates/layerx-programs-market/src/dispute.rs) |
| `rust_pub_item` | `pub fn encode(&self) -> [u8; MOVE_BYTES]` | [programs/crates/layerx-programs-market/src/dispute.rs](../../../programs/crates/layerx-programs-market/src/dispute.rs) |
| `rust_pub_item` | `pub struct Dispute` | [programs/crates/layerx-programs-market/src/dispute.rs](../../../programs/crates/layerx-programs-market/src/dispute.rs) |
| `rust_pub_item` | `pub fn open(` | [programs/crates/layerx-programs-market/src/dispute.rs](../../../programs/crates/layerx-programs-market/src/dispute.rs) |
| `rust_pub_item` | `pub fn transcript_preimage(&self, next: &Move) -> [u8; TRANSCRIPT_PREIMAGE_BYTES]` | [programs/crates/layerx-programs-market/src/dispute.rs](../../../programs/crates/layerx-programs-market/src/dispute.rs) |
| `rust_pub_item` | `pub const fn verdict(&self) -> Option<ArbiterVerdict>` | [programs/crates/layerx-programs-market/src/dispute.rs](../../../programs/crates/layerx-programs-market/src/dispute.rs) |
| `rust_pub_item` | `pub fn reveal(` | [programs/crates/layerx-programs-market/src/dispute.rs](../../../programs/crates/layerx-programs-market/src/dispute.rs) |
| `rust_pub_item` | `pub fn respond(mut self, height: u64, agree: bool) -> Result<(Self, Move), ProgramError>` | [programs/crates/layerx-programs-market/src/dispute.rs](../../../programs/crates/layerx-programs-market/src/dispute.rs) |
| `rust_pub_item` | `pub fn adjudication(&self) -> Result<(u8, u32), ProgramError>` | [programs/crates/layerx-programs-market/src/dispute.rs](../../../programs/crates/layerx-programs-market/src/dispute.rs) |
| `rust_pub_item` | `pub fn adjudicate(` | [programs/crates/layerx-programs-market/src/dispute.rs](../../../programs/crates/layerx-programs-market/src/dispute.rs) |
| `rust_pub_item` | `pub fn timeout(mut self, height: u64) -> Result<(Self, Move), ProgramError>` | [programs/crates/layerx-programs-market/src/dispute.rs](../../../programs/crates/layerx-programs-market/src/dispute.rs) |
| `rust_pub_item` | `pub fn encode_dispute(dispute: &Dispute, output: &mut [u8]) -> Result<usize, ProgramError>` | [programs/crates/layerx-programs-market/src/dispute.rs](../../../programs/crates/layerx-programs-market/src/dispute.rs) |
| `rust_pub_item` | `pub fn decode_dispute(input: &[u8]) -> Result<Dispute, ProgramError>` | [programs/crates/layerx-programs-market/src/dispute.rs](../../../programs/crates/layerx-programs-market/src/dispute.rs) |
| `rust_pub_item` | `pub const OFFER_PREFIX: &[u8] = b"lx.market.offer/"` | [programs/crates/layerx-programs-market/src/lib.rs](../../../programs/crates/layerx-programs-market/src/lib.rs) |
| `rust_pub_item` | `pub const LEASE_PREFIX: &[u8] = b"lx.market.lease/"` | [programs/crates/layerx-programs-market/src/lib.rs](../../../programs/crates/layerx-programs-market/src/lib.rs) |
| `rust_pub_item` | `pub const CLAIM_PREFIX: &[u8] = b"lx.market.claim/"` | [programs/crates/layerx-programs-market/src/lib.rs](../../../programs/crates/layerx-programs-market/src/lib.rs) |
| `rust_pub_item` | `pub const SETTLEMENT_PREFIX: &[u8] = b"lx.market.settlement/"` | [programs/crates/layerx-programs-market/src/lib.rs](../../../programs/crates/layerx-programs-market/src/lib.rs) |
| `rust_pub_item` | `pub const SETTLEMENT_CAPACITY: usize = 282` | [programs/crates/layerx-programs-market/src/lib.rs](../../../programs/crates/layerx-programs-market/src/lib.rs) |
| `rust_pub_item` | `pub enum VerificationModel` | [programs/crates/layerx-programs-market/src/lib.rs](../../../programs/crates/layerx-programs-market/src/lib.rs) |
| `rust_pub_item` | `pub enum OfferStatus` | [programs/crates/layerx-programs-market/src/lib.rs](../../../programs/crates/layerx-programs-market/src/lib.rs) |
| `rust_pub_item` | `pub enum LeaseStatus` | [programs/crates/layerx-programs-market/src/lib.rs](../../../programs/crates/layerx-programs-market/src/lib.rs) |
| `rust_pub_item` | `pub struct Offer<'a>` | [programs/crates/layerx-programs-market/src/lib.rs](../../../programs/crates/layerx-programs-market/src/lib.rs) |
| `rust_pub_item` | `pub struct ComputeLease<'a>` | [programs/crates/layerx-programs-market/src/lib.rs](../../../programs/crates/layerx-programs-market/src/lib.rs) |
| `rust_pub_item` | `pub struct SettlementRecord` | [programs/crates/layerx-programs-market/src/lib.rs](../../../programs/crates/layerx-programs-market/src/lib.rs) |
| `rust_pub_item` | `pub fn new(` | [programs/crates/layerx-programs-market/src/lib.rs](../../../programs/crates/layerx-programs-market/src/lib.rs) |
| `rust_pub_item` | `pub fn encode_settlement(` | [programs/crates/layerx-programs-market/src/lib.rs](../../../programs/crates/layerx-programs-market/src/lib.rs) |
| `rust_pub_item` | `pub fn decode_settlement(bytes: &[u8]) -> Result<SettlementRecord, ProgramError>` | [programs/crates/layerx-programs-market/src/lib.rs](../../../programs/crates/layerx-programs-market/src/lib.rs) |
| `rust_pub_item` | `pub struct RegisterOffer<'a>` | [programs/crates/layerx-programs-market/src/lib.rs](../../../programs/crates/layerx-programs-market/src/lib.rs) |
| `rust_pub_item` | `pub struct OpenLease<'a>` | [programs/crates/layerx-programs-market/src/lib.rs](../../../programs/crates/layerx-programs-market/src/lib.rs) |
| `rust_pub_item` | `pub fn register(` | [programs/crates/layerx-programs-market/src/lib.rs](../../../programs/crates/layerx-programs-market/src/lib.rs) |
| `rust_pub_item` | `pub fn open<'a>(` | [programs/crates/layerx-programs-market/src/lib.rs](../../../programs/crates/layerx-programs-market/src/lib.rs) |
| `rust_pub_item` | `pub fn expire<'a>(` | [programs/crates/layerx-programs-market/src/lib.rs](../../../programs/crates/layerx-programs-market/src/lib.rs) |
| `rust_pub_item` | `pub fn close(mut offer: Offer<'_>, principal: AccountId) -> Result<Offer<'_>, ProgramError>` | [programs/crates/layerx-programs-market/src/lib.rs](../../../programs/crates/layerx-programs-market/src/lib.rs) |
| `rust_pub_item` | `pub fn encode_offer(offer: Offer<'_>, output: &mut [u8]) -> Result<usize, ProgramError>` | [programs/crates/layerx-programs-market/src/lib.rs](../../../programs/crates/layerx-programs-market/src/lib.rs) |
| `rust_pub_item` | `pub fn decode_offer(input: &[u8]) -> Result<Offer<'_>, ProgramError>` | [programs/crates/layerx-programs-market/src/lib.rs](../../../programs/crates/layerx-programs-market/src/lib.rs) |
| `rust_pub_item` | `pub fn encode_lease(lease: &ComputeLease<'_>, output: &mut [u8]) -> Result<usize, ProgramError>` | [programs/crates/layerx-programs-market/src/lib.rs](../../../programs/crates/layerx-programs-market/src/lib.rs) |
| `rust_pub_item` | `pub fn decode_lease(input: &[u8]) -> Result<ComputeLease<'_>, ProgramError>` | [programs/crates/layerx-programs-market/src/lib.rs](../../../programs/crates/layerx-programs-market/src/lib.rs) |
| `rust_pub_item` | `pub const MAX_CHALLENGE_WINDOW_BATCHES: u64 = 100_000` | [programs/crates/layerx-programs-market/src/settle.rs](../../../programs/crates/layerx-programs-market/src/settle.rs) |
| `rust_pub_item` | `pub const MAX_USAGE_UNITS_PER_CLAIM: u64 = 1_000_000_000_000` | [programs/crates/layerx-programs-market/src/settle.rs](../../../programs/crates/layerx-programs-market/src/settle.rs) |
| `rust_pub_item` | `pub const CLAIM_CAPACITY: usize = 290` | [programs/crates/layerx-programs-market/src/settle.rs](../../../programs/crates/layerx-programs-market/src/settle.rs) |
| `rust_pub_item` | `pub const CHALLENGE_CAPACITY: usize = 427 + layerx_program_sdk::MAX_PROGRAM_ACCOUNT_SEED_BYTES` | [programs/crates/layerx-programs-market/src/settle.rs](../../../programs/crates/layerx-programs-market/src/settle.rs) |
| `rust_pub_item` | `pub struct MeteredUsageClaim` | [programs/crates/layerx-programs-market/src/settle.rs](../../../programs/crates/layerx-programs-market/src/settle.rs) |
| `rust_pub_item` | `pub enum ClaimStatus` | [programs/crates/layerx-programs-market/src/settle.rs](../../../programs/crates/layerx-programs-market/src/settle.rs) |
| `rust_pub_item` | `pub struct UsageClaim` | [programs/crates/layerx-programs-market/src/settle.rs](../../../programs/crates/layerx-programs-market/src/settle.rs) |
| `rust_pub_item` | `pub struct ChallengeWindow` | [programs/crates/layerx-programs-market/src/settle.rs](../../../programs/crates/layerx-programs-market/src/settle.rs) |
| `rust_pub_item` | `pub fn contains(self, height: u64) -> bool` | [programs/crates/layerx-programs-market/src/settle.rs](../../../programs/crates/layerx-programs-market/src/settle.rs) |
| `rust_pub_item` | `pub fn elapsed(self, height: u64) -> bool` | [programs/crates/layerx-programs-market/src/settle.rs](../../../programs/crates/layerx-programs-market/src/settle.rs) |
| `rust_pub_item` | `pub struct ProviderCommitment` | [programs/crates/layerx-programs-market/src/settle.rs](../../../programs/crates/layerx-programs-market/src/settle.rs) |
| `rust_pub_item` | `pub struct ContradictingCommitment` | [programs/crates/layerx-programs-market/src/settle.rs](../../../programs/crates/layerx-programs-market/src/settle.rs) |
| `rust_pub_item` | `pub struct UsageChallenge<'a>` | [programs/crates/layerx-programs-market/src/settle.rs](../../../programs/crates/layerx-programs-market/src/settle.rs) |
| `rust_pub_item` | `pub enum ArbiterVerdict` | [programs/crates/layerx-programs-market/src/settle.rs](../../../programs/crates/layerx-programs-market/src/settle.rs) |
| `rust_pub_item` | `pub struct ArbiterResolution` | [programs/crates/layerx-programs-market/src/settle.rs](../../../programs/crates/layerx-programs-market/src/settle.rs) |
| `rust_pub_item` | `pub struct SettlementPlan` | [programs/crates/layerx-programs-market/src/settle.rs](../../../programs/crates/layerx-programs-market/src/settle.rs) |
| `rust_pub_item` | `pub fn total(self) -> Result<Amount, ProgramError>` | [programs/crates/layerx-programs-market/src/settle.rs](../../../programs/crates/layerx-programs-market/src/settle.rs) |
| `rust_pub_item` | `pub fn conserves(self, escrow: Amount, stake: Amount) -> Result<(), ProgramError>` | [programs/crates/layerx-programs-market/src/settle.rs](../../../programs/crates/layerx-programs-market/src/settle.rs) |
| `rust_pub_item` | `pub fn decode_claim(input: &[u8]) -> Result<UsageClaim, ProgramError>` | [programs/crates/layerx-programs-market/src/settle.rs](../../../programs/crates/layerx-programs-market/src/settle.rs) |
| `rust_pub_item` | `pub fn commit_usage(` | [programs/crates/layerx-programs-market/src/settle.rs](../../../programs/crates/layerx-programs-market/src/settle.rs) |
| `rust_pub_item` | `pub fn commit_direct_usage(` | [programs/crates/layerx-programs-market/src/settle.rs](../../../programs/crates/layerx-programs-market/src/settle.rs) |
| `rust_pub_item` | `pub struct ChallengeRequest<'a>` | [programs/crates/layerx-programs-market/src/settle.rs](../../../programs/crates/layerx-programs-market/src/settle.rs) |
| `rust_pub_item` | `let challenge = UsageChallenge {` | [programs/crates/layerx-programs-market/src/settle.rs](../../../programs/crates/layerx-programs-market/src/settle.rs) |
| `rust_pub_item` | `pub fn finalize_unchallenged<'a>(` | [programs/crates/layerx-programs-market/src/settle.rs](../../../programs/crates/layerx-programs-market/src/settle.rs) |
| `rust_pub_item` | `pub fn fund_challenge(challenge: UsageChallenge<'_>, asset: AssetId) -> Result<(), ProgramError>` | [programs/crates/layerx-programs-market/src/settle.rs](../../../programs/crates/layerx-programs-market/src/settle.rs) |
| `rust_pub_item` | `pub fn execute_settlement(` | [programs/crates/layerx-programs-market/src/settle.rs](../../../programs/crates/layerx-programs-market/src/settle.rs) |
| `rust_pub_item` | `pub const STAKE_PREFIX: &[u8] = b"lx.market.stake/"` | [programs/crates/layerx-programs-market/src/stake.rs](../../../programs/crates/layerx-programs-market/src/stake.rs) |
| `rust_pub_item` | `pub const STAKE_CAPACITY: usize = 177` | [programs/crates/layerx-programs-market/src/stake.rs](../../../programs/crates/layerx-programs-market/src/stake.rs) |
| `rust_pub_item` | `pub const MAX_TRANSFERS: usize = 3` | [programs/crates/layerx-programs-market/src/stake.rs](../../../programs/crates/layerx-programs-market/src/stake.rs) |
| `rust_pub_item` | `pub struct Stake` | [programs/crates/layerx-programs-market/src/stake.rs](../../../programs/crates/layerx-programs-market/src/stake.rs) |
| `rust_pub_item` | `pub struct Transfer<'a>` | [programs/crates/layerx-programs-market/src/stake.rs](../../../programs/crates/layerx-programs-market/src/stake.rs) |
| `rust_pub_item` | `pub struct TransferSet<'a>` | [programs/crates/layerx-programs-market/src/stake.rs](../../../programs/crates/layerx-programs-market/src/stake.rs) |
| `rust_pub_item` | `pub fn iter(&self) -> impl Iterator<Item = &Transfer<'a>>` | [programs/crates/layerx-programs-market/src/stake.rs](../../../programs/crates/layerx-programs-market/src/stake.rs) |
| `rust_pub_item` | `pub fn debited(&self, account: AccountId) -> Result<Amount, ProgramError>` | [programs/crates/layerx-programs-market/src/stake.rs](../../../programs/crates/layerx-programs-market/src/stake.rs) |
| `rust_pub_item` | `pub fn credited(&self, account: AccountId) -> Result<Amount, ProgramError>` | [programs/crates/layerx-programs-market/src/stake.rs](../../../programs/crates/layerx-programs-market/src/stake.rs) |
| `rust_pub_item` | `pub fn total(&self) -> Result<Amount, ProgramError>` | [programs/crates/layerx-programs-market/src/stake.rs](../../../programs/crates/layerx-programs-market/src/stake.rs) |
| `rust_pub_item` | `pub const fn value_at_risk(lease: &ComputeLease<'_>) -> Amount` | [programs/crates/layerx-programs-market/src/stake.rs](../../../programs/crates/layerx-programs-market/src/stake.rs) |
| `rust_pub_item` | `pub fn post(offer: &Offer<'_>) -> Result<Self, ProgramError>` | [programs/crates/layerx-programs-market/src/stake.rs](../../../programs/crates/layerx-programs-market/src/stake.rs) |
| `rust_pub_item` | `pub fn unlocked(self) -> Result<Amount, ProgramError>` | [programs/crates/layerx-programs-market/src/stake.rs](../../../programs/crates/layerx-programs-market/src/stake.rs) |
| `rust_pub_item` | `pub fn lock(` | [programs/crates/layerx-programs-market/src/stake.rs](../../../programs/crates/layerx-programs-market/src/stake.rs) |
| `rust_pub_item` | `pub fn release(` | [programs/crates/layerx-programs-market/src/stake.rs](../../../programs/crates/layerx-programs-market/src/stake.rs) |
| `rust_pub_item` | `pub fn withdraw<'a>(` | [programs/crates/layerx-programs-market/src/stake.rs](../../../programs/crates/layerx-programs-market/src/stake.rs) |
| `rust_pub_item` | `pub fn slash<'a>(` | [programs/crates/layerx-programs-market/src/stake.rs](../../../programs/crates/layerx-programs-market/src/stake.rs) |
| `rust_pub_item` | `pub fn execute(set: &TransferSet<'_>) -> Result<(), ProgramError>` | [programs/crates/layerx-programs-market/src/stake.rs](../../../programs/crates/layerx-programs-market/src/stake.rs) |
| `rust_pub_item` | `pub fn encode_stake(stake: &Stake, output: &mut [u8]) -> Result<usize, ProgramError>` | [programs/crates/layerx-programs-market/src/stake.rs](../../../programs/crates/layerx-programs-market/src/stake.rs) |
| `rust_pub_item` | `pub fn decode_stake(input: &[u8]) -> Result<Stake, ProgramError>` | [programs/crates/layerx-programs-market/src/stake.rs](../../../programs/crates/layerx-programs-market/src/stake.rs) |

## ai-market-guest

| Kind | Declaration | Source |
|---|---|---|
| `rust_pub_item` | `pub const FINALIZED_RANK: u8 = 4` | [programs/crates/layerx-programs-ai-market/src/queries.rs](../../../programs/crates/layerx-programs-ai-market/src/queries.rs) |
| `rust_pub_item` | `pub const BINDING_MAX_BYTES: usize = BINDING_PREFIX_MAX_BYTES + 8 + 32 + 32 + 32 + 1 + 8 + 8;` | [programs/crates/layerx-programs-ai-market/src/queries.rs](../../../programs/crates/layerx-programs-ai-market/src/queries.rs) |
| `rust_pub_item` | `pub struct ReadProof` | [programs/crates/layerx-programs-ai-market/src/queries.rs](../../../programs/crates/layerx-programs-ai-market/src/queries.rs) |
| `rust_pub_item` | `pub struct FinalityEvidence` | [programs/crates/layerx-programs-ai-market/src/queries.rs](../../../programs/crates/layerx-programs-ai-market/src/queries.rs) |
| `rust_pub_item` | `pub struct SnapshotBinding` | [programs/crates/layerx-programs-ai-market/src/queries.rs](../../../programs/crates/layerx-programs-ai-market/src/queries.rs) |
| `rust_pub_item` | `pub fn bind_snapshot(` | [programs/crates/layerx-programs-ai-market/src/queries.rs](../../../programs/crates/layerx-programs-ai-market/src/queries.rs) |
| `rust_pub_item` | `pub fn encode(&self, out: &mut [u8]) -> CodecResult<usize>` | [programs/crates/layerx-programs-ai-market/src/queries.rs](../../../programs/crates/layerx-programs-ai-market/src/queries.rs) |
| `rust_pub_item` | `pub fn require_finalized(&self) -> QueryResult<()>` | [programs/crates/layerx-programs-ai-market/src/queries.rs](../../../programs/crates/layerx-programs-ai-market/src/queries.rs) |

## sdk-examples

| Kind | Declaration | Source |
|---|---|---|
| `wasm_binding` | `export function configure(` | [programs/sdk/assemblyscript/examples/paid-counter/assembly/index.ts](../../../programs/sdk/assemblyscript/examples/paid-counter/assembly/index.ts) |
| `wasm_binding` | `export function count(): i64` | [programs/sdk/assemblyscript/examples/paid-counter/assembly/index.ts](../../../programs/sdk/assemblyscript/examples/paid-counter/assembly/index.ts) |
| `wasm_binding` | `export function settle(` | [programs/sdk/assemblyscript/examples/paid-counter/assembly/index.ts](../../../programs/sdk/assemblyscript/examples/paid-counter/assembly/index.ts) |
| `wasm_binding` | `export function forward(` | [programs/sdk/assemblyscript/examples/paid-counter/assembly/index.ts](../../../programs/sdk/assemblyscript/examples/paid-counter/assembly/index.ts) |
| `wasm_binding` | `export function reset(): i32` | [programs/sdk/assemblyscript/examples/paid-counter/assembly/index.ts](../../../programs/sdk/assemblyscript/examples/paid-counter/assembly/index.ts) |
| `wasm_binding` | `export function layerx_main(selector: i64): i64` | [programs/sdk/assemblyscript/examples/paid-counter/assembly/index.ts](../../../programs/sdk/assemblyscript/examples/paid-counter/assembly/index.ts) |
| `wasm_binding` | `export function layerx_reserve(length: i32): i32` | [programs/sdk/assemblyscript/examples/paid-counter/assembly/index.ts](../../../programs/sdk/assemblyscript/examples/paid-counter/assembly/index.ts) |
| `wasm_binding` | `export function layerx_call(inputPointer: i32, inputLength: i32): i32` | [programs/sdk/assemblyscript/examples/paid-counter/assembly/index.ts](../../../programs/sdk/assemblyscript/examples/paid-counter/assembly/index.ts) |
| `wasm_binding` | `LXP_PROGRAM_EXPORT("configure") int32_t configure(int64_t fee_high, int64_t fee_low, int64_t asset_word0,` | [programs/sdk/c/examples/paid-counter/src/program.c](../../../programs/sdk/c/examples/paid-counter/src/program.c) |
| `wasm_binding` | `LXP_PROGRAM_EXPORT("count") int64_t count(void)` | [programs/sdk/c/examples/paid-counter/src/program.c](../../../programs/sdk/c/examples/paid-counter/src/program.c) |
| `wasm_binding` | `LXP_PROGRAM_EXPORT("settle") int64_t settle(int64_t digest_word0, int64_t digest_word1,` | [programs/sdk/c/examples/paid-counter/src/program.c](../../../programs/sdk/c/examples/paid-counter/src/program.c) |
| `wasm_binding` | `LXP_PROGRAM_EXPORT("forward") int32_t forward(int64_t callee_word0, int64_t callee_word1,` | [programs/sdk/c/examples/paid-counter/src/program.c](../../../programs/sdk/c/examples/paid-counter/src/program.c) |
| `wasm_binding` | `LXP_PROGRAM_EXPORT("reset") int32_t reset(void)` | [programs/sdk/c/examples/paid-counter/src/program.c](../../../programs/sdk/c/examples/paid-counter/src/program.c) |
| `wasm_binding` | `LXP_PROGRAM_EXPORT(LXP_PROGRAM_ENTRYPOINT) int64_t layerx_main(int64_t selector)` | [programs/sdk/c/examples/paid-counter/src/program.c](../../../programs/sdk/c/examples/paid-counter/src/program.c) |
| `wasm_binding` | `LXP_PROGRAM_EXPORT(LXP_PROGRAM_CALL_RESERVE_EXPORT) int32_t layerx_reserve(int32_t length)` | [programs/sdk/c/examples/paid-counter/src/program.c](../../../programs/sdk/c/examples/paid-counter/src/program.c) |
| `wasm_binding` | `LXP_PROGRAM_EXPORT(LXP_PROGRAM_CALL_ENTRY_EXPORT) int32_t layerx_call(int32_t input_pointer, int32_t input_length)` | [programs/sdk/c/examples/paid-counter/src/program.c](../../../programs/sdk/c/examples/paid-counter/src/program.c) |
| source | — | [programs/sdk/c/examples/paid-counter/build.sh](../../../programs/sdk/c/examples/paid-counter/build.sh) |
| source | — | [programs/sdk/rust/examples/escrow/build.sh](../../../programs/sdk/rust/examples/escrow/build.sh) |
| source | — | [programs/sdk/rust/examples/naming/build.sh](../../../programs/sdk/rust/examples/naming/build.sh) |
| source | — | [programs/sdk/rust/examples/nft-lxt721/build.sh](../../../programs/sdk/rust/examples/nft-lxt721/build.sh) |
| source | — | [programs/sdk/rust/examples/vault/build.sh](../../../programs/sdk/rust/examples/vault/build.sh) |
| source | — | [programs/sdk/rust/examples/web-reader/build.sh](../../../programs/sdk/rust/examples/web-reader/build.sh) |
| source | — | [programs/sdk/rust/quickstart/build.sh](../../../programs/sdk/rust/quickstart/build.sh) |
| source | — | [programs/sdk/rust/response-fixture/build.sh](../../../programs/sdk/rust/response-fixture/build.sh) |
| source | — | [programs/sdk/assemblyscript/examples/paid-counter/layerx-program.json](../../../programs/sdk/assemblyscript/examples/paid-counter/layerx-program.json) |
| source | — | [programs/sdk/c/examples/paid-counter/layerx-program.json](../../../programs/sdk/c/examples/paid-counter/layerx-program.json) |
| source | — | [programs/sdk/rust/examples/escrow/LayerX.toml](../../../programs/sdk/rust/examples/escrow/LayerX.toml) |
| source | — | [programs/sdk/rust/examples/escrow/interface.kvx](../../../programs/sdk/rust/examples/escrow/interface.kvx) |
| source | — | [programs/sdk/rust/examples/escrow/layerx-program.json](../../../programs/sdk/rust/examples/escrow/layerx-program.json) |
| source | — | [programs/sdk/rust/examples/naming/LayerX.toml](../../../programs/sdk/rust/examples/naming/LayerX.toml) |
| source | — | [programs/sdk/rust/examples/naming/interface.kvx](../../../programs/sdk/rust/examples/naming/interface.kvx) |
| source | — | [programs/sdk/rust/examples/naming/layerx-program.json](../../../programs/sdk/rust/examples/naming/layerx-program.json) |
| source | — | [programs/sdk/rust/examples/nft-lxt721/LayerX.toml](../../../programs/sdk/rust/examples/nft-lxt721/LayerX.toml) |
| source | — | [programs/sdk/rust/examples/nft-lxt721/interface.kvx](../../../programs/sdk/rust/examples/nft-lxt721/interface.kvx) |
| source | — | [programs/sdk/rust/examples/nft-lxt721/layerx-program.json](../../../programs/sdk/rust/examples/nft-lxt721/layerx-program.json) |
| source | — | [programs/sdk/rust/examples/vault/LayerX.toml](../../../programs/sdk/rust/examples/vault/LayerX.toml) |
| source | — | [programs/sdk/rust/examples/vault/interface.kvx](../../../programs/sdk/rust/examples/vault/interface.kvx) |
| source | — | [programs/sdk/rust/examples/vault/layerx-program.json](../../../programs/sdk/rust/examples/vault/layerx-program.json) |
| source | — | [programs/sdk/rust/examples/web-reader/LayerX.toml](../../../programs/sdk/rust/examples/web-reader/LayerX.toml) |
| source | — | [programs/sdk/rust/examples/web-reader/interface.kvx](../../../programs/sdk/rust/examples/web-reader/interface.kvx) |
| source | — | [programs/sdk/rust/examples/web-reader/layerx-program.json](../../../programs/sdk/rust/examples/web-reader/layerx-program.json) |
| source | — | [programs/sdk/rust/quickstart/LayerX.toml](../../../programs/sdk/rust/quickstart/LayerX.toml) |
| source | — | [programs/sdk/rust/quickstart/layerx-program.json](../../../programs/sdk/rust/quickstart/layerx-program.json) |
| source | — | [programs/sdk/rust/response-fixture/layerx-program.json](../../../programs/sdk/rust/response-fixture/layerx-program.json) |
| source | — | [programs/sdk/rust/examples/escrow/src/lib.rs](../../../programs/sdk/rust/examples/escrow/src/lib.rs) |
| source | — | [programs/sdk/rust/examples/naming/src/lib.rs](../../../programs/sdk/rust/examples/naming/src/lib.rs) |
| source | — | [programs/sdk/rust/examples/nft-lxt721/src/lib.rs](../../../programs/sdk/rust/examples/nft-lxt721/src/lib.rs) |
| source | — | [programs/sdk/rust/examples/payments-merchant/src/lib.rs](../../../programs/sdk/rust/examples/payments-merchant/src/lib.rs) |
| source | — | [programs/sdk/rust/examples/swap-cpmm/src/lib.rs](../../../programs/sdk/rust/examples/swap-cpmm/src/lib.rs) |
| source | — | [programs/sdk/rust/examples/token-lxt20/src/lib.rs](../../../programs/sdk/rust/examples/token-lxt20/src/lib.rs) |
| source | — | [programs/sdk/rust/examples/vault/src/lib.rs](../../../programs/sdk/rust/examples/vault/src/lib.rs) |
| source | — | [programs/sdk/rust/examples/web-reader/src/lib.rs](../../../programs/sdk/rust/examples/web-reader/src/lib.rs) |
| source | — | [programs/sdk/rust/quickstart/src/lib.rs](../../../programs/sdk/rust/quickstart/src/lib.rs) |
| source | — | [programs/sdk/rust/response-fixture/src/lib.rs](../../../programs/sdk/rust/response-fixture/src/lib.rs) |

## sdk-lint

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `determinism-lint [--abi-version 1..4] [--abi-manifest path] <module.wasm\|source.c>...` | [programs/sdk/c/tools/determinism_lint.c](../../../programs/sdk/c/tools/determinism_lint.c) |
| `rust_pub_item` | `pub enum DeterminismViolation` | [programs/sdk/rust/lint/src/lib.rs](../../../programs/sdk/rust/lint/src/lib.rs) |
| `rust_pub_item` | `pub fn name(&self) -> &'static str` | [programs/sdk/rust/lint/src/lib.rs](../../../programs/sdk/rust/lint/src/lib.rs) |
| `rust_pub_item` | `pub fn abi_surface_violations() -> Vec<DeterminismViolation>` | [programs/sdk/rust/lint/src/lib.rs](../../../programs/sdk/rust/lint/src/lib.rs) |
| `rust_pub_item` | `pub fn lint_artifact(wasm: &[u8]) -> Vec<DeterminismViolation>` | [programs/sdk/rust/lint/src/lib.rs](../../../programs/sdk/rust/lint/src/lib.rs) |
| `rust_pub_item` | `pub fn lint_candidate_artifact(wasm: &[u8]) -> Vec<DeterminismViolation>` | [programs/sdk/rust/lint/src/lib.rs](../../../programs/sdk/rust/lint/src/lib.rs) |
| `rust_pub_item` | `pub fn lint_artifact_for_abi(wasm: &[u8], abi_version: u16) -> Vec<DeterminismViolation>` | [programs/sdk/rust/lint/src/lib.rs](../../../programs/sdk/rust/lint/src/lib.rs) |
| `rust_pub_item` | `pub fn lint_sources(project: &Path) -> Vec<DeterminismViolation>` | [programs/sdk/rust/lint/src/lib.rs](../../../programs/sdk/rust/lint/src/lib.rs) |
| `rust_pub_item` | `pub fn lint_project(project: &Path, artifact: Option<&Path>) -> Vec<DeterminismViolation>` | [programs/sdk/rust/lint/src/lib.rs](../../../programs/sdk/rust/lint/src/lib.rs) |
| `rust_pub_item` | `pub fn lint_project_for_abi(` | [programs/sdk/rust/lint/src/lib.rs](../../../programs/sdk/rust/lint/src/lib.rs) |
| `rust_pub_item` | `pub fn discover_artifact(project: &Path) -> Result<PathBuf, DeterminismViolation>` | [programs/sdk/rust/lint/src/lib.rs](../../../programs/sdk/rust/lint/src/lib.rs) |
| source | — | [programs/sdk/assemblyscript/tools/check-capability-fixture.mjs](../../../programs/sdk/assemblyscript/tools/check-capability-fixture.mjs) |
| source | — | [programs/sdk/assemblyscript/tools/determinism-lint.mjs](../../../programs/sdk/assemblyscript/tools/determinism-lint.mjs) |
| source | — | [programs/sdk/rust/lint/src/main.rs](../../../programs/sdk/rust/lint/src/main.rs) |

## error-codes

| Kind | Declaration | Source |
|---|---|---|
| `error_code` | `pub const BAD_VERSION: ApplicationError = application_error(0x0001);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const NON_CANONICAL: ApplicationError = application_error(0x0002);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const WRONG_DOMAIN: ApplicationError = application_error(0x0003);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const WRONG_PROGRAM: ApplicationError = application_error(0x0004);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const WRONG_MARKET: ApplicationError = application_error(0x0005);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const UNAUTHORIZED: ApplicationError = application_error(0x0006);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const ROLE_CONFLICT: ApplicationError = application_error(0x0007);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const REVOKED: ApplicationError = application_error(0x0008);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const KEY_MISMATCH: ApplicationError = application_error(0x0009);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const BAD_SIGNATURE: ApplicationError = application_error(0x000a);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const EXPIRED: ApplicationError = application_error(0x000b);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const WRONG_EPOCH: ApplicationError = application_error(0x000c);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const WRONG_CONFIG: ApplicationError = application_error(0x000d);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const WRONG_ROSTER: ApplicationError = application_error(0x000e);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const WRONG_PHASE: ApplicationError = application_error(0x000f);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const REPLAY_CONFLICT: ApplicationError = application_error(0x0010);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const SEQUENCE_CONSUMED: ApplicationError = application_error(0x0011);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const SEQUENCE_GAP: ApplicationError = application_error(0x0012);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const STALE_CURSOR: ApplicationError = application_error(0x0013);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const CONFLICT: ApplicationError = application_error(0x0014);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const NOT_FOUND: ApplicationError = application_error(0x0015);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const CAPACITY: ApplicationError = application_error(0x0016);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const RETENTION_FULL: ApplicationError = application_error(0x0017);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const INSUFFICIENT_FREE: ApplicationError = application_error(0x0018);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const ACCOUNT_BINDING: ApplicationError = application_error(0x0019);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const HOST_CAPABILITY: ApplicationError = application_error(0x001a);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const HOST_TRANSFER: ApplicationError = application_error(0x001b);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const ARITHMETIC: ApplicationError = application_error(0x001c);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const EVIDENCE_BINDING: ApplicationError = application_error(0x001d);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const READINESS_BLOCKED: ApplicationError = application_error(0x001e);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const UNKNOWN_OPERATION: ApplicationError = application_error(0x001f);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F03_NO_GRANT: ApplicationError = application_error(0x0301);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F03_EVALUATOR_CAPACITY: ApplicationError = application_error(0x0302);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F03_BAD_ACTIVATION: ApplicationError = application_error(0x0303);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F03_KEY_REUSED: ApplicationError = application_error(0x0304);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F03_KEY_VERSION_CONFLICT: ApplicationError = application_error(0x0305);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F03_GRANT_VERSION_CONFLICT: ApplicationError = application_error(0x0306);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F03_REPORT_ALREADY_FINAL: ApplicationError = application_error(0x0307);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F03_CHALLENGE_CAPACITY: ApplicationError = application_error(0x0308);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F03_UNKNOWN_WORKER: ApplicationError = application_error(0x0309);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F03_NO_SCORES: ApplicationError = application_error(0x030a);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F03_SCORE_RANGE: ApplicationError = application_error(0x030b);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F03_NONCANONICAL_VECTOR: ApplicationError = application_error(0x030c);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F03_EVIDENCE_NOT_SEALED: ApplicationError = application_error(0x030d);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F03_EVIDENCE_ROOT_MISMATCH: ApplicationError = application_error(0x030e);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F06_FUNDING_POLICY_MISMATCH: ApplicationError = application_error(0x0601);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F06_REFUND_RECIPIENT_MISMATCH: ApplicationError = application_error(0x0602);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F06_WRONG_ASSET: ApplicationError = application_error(0x0603);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F06_INVALID_AMOUNT: ApplicationError = application_error(0x0604);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F06_EPOCH_ALREADY_RESERVED: ApplicationError = application_error(0x0605);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F06_EPOCH_NOT_RESERVED: ApplicationError = application_error(0x0606);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F06_AGGREGATION_MISMATCH: ApplicationError = application_error(0x0607);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F06_EPOCH_TERMINAL: ApplicationError = application_error(0x0608);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F06_CLAIM_NOT_READY: ApplicationError = application_error(0x0609);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F06_CLAIM_EXPIRED: ApplicationError = application_error(0x060a);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F06_UNKNOWN_WORKER_ENTITLEMENT: ApplicationError = application_error(0x060b);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F06_WRONG_CLAIM_RECIPIENT: ApplicationError = application_error(0x060c);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F06_WRONG_CLAIM_AMOUNT: ApplicationError = application_error(0x060d);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F06_NOTHING_TO_CLAIM: ApplicationError = application_error(0x060e);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F06_LEDGER_INVARIANT_VIOLATION: ApplicationError = application_error(0x060f);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F06_CONTRIBUTION_CONSENT_REQUIRED: ApplicationError = application_error(0x0610);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F01_INVALID_POLICY: ApplicationError = application_error(0x0101);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F01_PRINCIPAL_MISMATCH: ApplicationError = application_error(0x0102);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F01_STALE_REVISION: ApplicationError = application_error(0x0103);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F01_ALREADY_CREATED: ApplicationError = application_error(0x0104);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F01_ACCOUNT_BINDING_MISSING: ApplicationError = application_error(0x0105);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F01_VERSION_MISMATCH: ApplicationError = application_error(0x0106);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F01_PENDING_POLICY_EXISTS: ApplicationError = application_error(0x0107);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F01_NO_PENDING_POLICY: ApplicationError = application_error(0x0108);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F01_ACTIVATION_TOO_EARLY: ApplicationError = application_error(0x0109);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F01_ACTIVATION_NOT_READY: ApplicationError = application_error(0x010a);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F01_ALREADY_ACTIVATED: ApplicationError = application_error(0x010b);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F01_WRONG_LIFECYCLE: ApplicationError = application_error(0x010c);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F01_LIFECYCLE_CLOSED: ApplicationError = application_error(0x010d);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F01_INVALID_REASON: ApplicationError = application_error(0x010e);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F01_GRANT_ALREADY_REVOKED: ApplicationError = application_error(0x010f);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F01_ALREADY_CLOSING: ApplicationError = application_error(0x0110);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F01_OBLIGATIONS_OUTSTANDING: ApplicationError = application_error(0x0111);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F01_CAPACITY_UNAVAILABLE: ApplicationError = application_error(0x0112);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F01_NO_TASK_CAPACITY: ApplicationError = application_error(0x0113);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F01_UNKNOWN_WORKER: ApplicationError = application_error(0x0114);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F01_TASK_CONFLICT: ApplicationError = application_error(0x0115);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F01_TASK_EXPIRED: ApplicationError = application_error(0x0116);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F01_POLICY_MISMATCH: ApplicationError = application_error(0x0117);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F01_TASK_NOT_FOUND: ApplicationError = application_error(0x0118);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F01_WRONG_WORKER: ApplicationError = application_error(0x0119);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F01_TASK_ALREADY_ACCEPTED: ApplicationError = application_error(0x011a);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F02_OWNER_REQUIRED: ApplicationError = application_error(0x0201);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F02_DELEGATE_CONSENT_REQUIRED: ApplicationError = application_error(0x0202);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F02_DELEGATE_REVOKED: ApplicationError = application_error(0x0203);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F02_IDENTITY_FROZEN: ApplicationError = application_error(0x0204);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F02_STALE_AUTHORITY: ApplicationError = application_error(0x0205);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F02_WRONG_GENERATION: ApplicationError = application_error(0x0206);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F02_WRONG_REVISION: ApplicationError = application_error(0x0207);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F02_METADATA_EXPIRED: ApplicationError = application_error(0x0208);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F02_METADATA_UNAVAILABLE: ApplicationError = application_error(0x0209);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F02_METADATA_INTEGRITY_FAILURE: ApplicationError = application_error(0x020a);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F02_CAPABILITY_MISMATCH: ApplicationError = application_error(0x020b);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F02_ADMISSION_NOT_EFFECTIVE: ApplicationError = application_error(0x020c);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F02_MARKET_PAUSED: ApplicationError = application_error(0x020d);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F02_RATE_LIMITED: ApplicationError = application_error(0x020e);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F02_INPUT_TOO_LARGE: ApplicationError = application_error(0x020f);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F02_OUTPUT_TOO_LARGE: ApplicationError = application_error(0x0210);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F02_DEADLINE_INVALID: ApplicationError = application_error(0x0211);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F02_ACCESS_DENIED: ApplicationError = application_error(0x0212);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F02_UNKNOWN_EXECUTION: ApplicationError = application_error(0x0213);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F04_NO_SCORES: ApplicationError = application_error(0x0401);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F04_SALT_INVALID: ApplicationError = application_error(0x0402);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F04_COMMIT_MISMATCH: ApplicationError = application_error(0x0403);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F05_REPORT_INVARIANT: ApplicationError = application_error(0x0501);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F07_IDENTITY_FROZEN: ApplicationError = application_error(0x0701);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F07_SEGMENT_MISMATCH: ApplicationError = application_error(0x0702);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F07_GENERATION_MISMATCH: ApplicationError = application_error(0x0703);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F07_UNKNOWN_WORKER: ApplicationError = application_error(0x0704);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F07_EPOCH_NOT_SEALED: ApplicationError = application_error(0x0705);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F07_BINDING_MISMATCH: ApplicationError = application_error(0x0706);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F07_RESOURCE_LIMIT: ApplicationError = application_error(0x0707);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F07_FINALITY_UNAVAILABLE: ApplicationError = application_error(0x0708);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F08_OWNER_REQUIRED: ApplicationError = application_error(0x0801);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F08_ADMINISTRATOR_APPROVAL_REQUIRED: ApplicationError = application_error(0x0802);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F08_BAD_CONSENT: ApplicationError = application_error(0x0803);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F08_PERMIT_EXPIRED: ApplicationError = application_error(0x0804);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F08_PERMIT_CONSUMED: ApplicationError = application_error(0x0805);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F08_IDENTITY_FROZEN: ApplicationError = application_error(0x0806);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F08_DELEGATE_REVOKED: ApplicationError = application_error(0x0807);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F08_WRONG_GENERATION: ApplicationError = application_error(0x0808);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F08_MARKET_PAUSED: ApplicationError = application_error(0x0809);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F08_DUPLICATE_IDENTITY: ApplicationError = application_error(0x080a);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F08_OWNER_CAPACITY_EXCEEDED: ApplicationError = application_error(0x080b);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F08_CAPACITY_EXCEEDED: ApplicationError = application_error(0x080c);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F08_ADMISSION_WINDOW_FULL: ApplicationError = application_error(0x080d);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F08_RATE_LIMITED: ApplicationError = application_error(0x080e);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F08_NO_PRUNABLE_MEMBER: ApplicationError = application_error(0x080f);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F08_CANDIDATE_CHANGED: ApplicationError = application_error(0x0810);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F08_STALE_STATE: ApplicationError = application_error(0x0811);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F08_RETENTION_BLOCKED: ApplicationError = application_error(0x0812);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F08_QUORUM_UNAVAILABLE: ApplicationError = application_error(0x0813);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F08_IDEMPOTENCY_CONFLICT: ApplicationError = application_error(0x0814);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F08_RESOURCE_EXHAUSTED: ApplicationError = application_error(0x0815);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F09_EVIDENCE_TASK_SET_UNSEALED: ApplicationError = application_error(0x0901);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F09_EVIDENCE_SEAL_CONFLICT: ApplicationError = application_error(0x0902);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `pub const F07_IDEMPOTENCY_CONFLICT: ApplicationError = application_error(0x0709);` | [programs/crates/layerx-programs-ai-market/src/errors.rs](../../../programs/crates/layerx-programs-ai-market/src/errors.rs) |
| `error_code` | `export const OK: i32 = 0;` | [programs/sdk/assemblyscript/assembly/error.ts](../../../programs/sdk/assemblyscript/assembly/error.ts) |
| `error_code` | `export const ERR_DENIED: i32 = -1;` | [programs/sdk/assemblyscript/assembly/error.ts](../../../programs/sdk/assemblyscript/assembly/error.ts) |
| `error_code` | `export const ERR_INVALID: i32 = -2;` | [programs/sdk/assemblyscript/assembly/error.ts](../../../programs/sdk/assemblyscript/assembly/error.ts) |
| `error_code` | `export const ERR_BOUNDS: i32 = -3;` | [programs/sdk/assemblyscript/assembly/error.ts](../../../programs/sdk/assemblyscript/assembly/error.ts) |
| `error_code` | `export const ERR_METER: i32 = -4;` | [programs/sdk/assemblyscript/assembly/error.ts](../../../programs/sdk/assemblyscript/assembly/error.ts) |
| `error_code` | `export const ERR_EVIDENCE: i32 = -5;` | [programs/sdk/assemblyscript/assembly/error.ts](../../../programs/sdk/assemblyscript/assembly/error.ts) |
| `error_code` | `export const ERR_NULL_ARGUMENT: i32 = -16;` | [programs/sdk/assemblyscript/assembly/error.ts](../../../programs/sdk/assemblyscript/assembly/error.ts) |
| `error_code` | `export const ERR_EMPTY_KEY: i32 = -17;` | [programs/sdk/assemblyscript/assembly/error.ts](../../../programs/sdk/assemblyscript/assembly/error.ts) |
| `error_code` | `export const ERR_KEY_TOO_LARGE: i32 = -18;` | [programs/sdk/assemblyscript/assembly/error.ts](../../../programs/sdk/assemblyscript/assembly/error.ts) |
| `error_code` | `export const ERR_VALUE_TOO_LARGE: i32 = -19;` | [programs/sdk/assemblyscript/assembly/error.ts](../../../programs/sdk/assemblyscript/assembly/error.ts) |
| `error_code` | `export const ERR_EMPTY_TOPIC: i32 = -20;` | [programs/sdk/assemblyscript/assembly/error.ts](../../../programs/sdk/assemblyscript/assembly/error.ts) |
| `error_code` | `export const ERR_TOPIC_TOO_LARGE: i32 = -21;` | [programs/sdk/assemblyscript/assembly/error.ts](../../../programs/sdk/assemblyscript/assembly/error.ts) |
| `error_code` | `export const ERR_DATA_TOO_LARGE: i32 = -22;` | [programs/sdk/assemblyscript/assembly/error.ts](../../../programs/sdk/assemblyscript/assembly/error.ts) |
| `error_code` | `export const ERR_INPUT_TOO_LARGE: i32 = -23;` | [programs/sdk/assemblyscript/assembly/error.ts](../../../programs/sdk/assemblyscript/assembly/error.ts) |
| `error_code` | `export const ERR_ZERO_AMOUNT: i32 = -24;` | [programs/sdk/assemblyscript/assembly/error.ts](../../../programs/sdk/assemblyscript/assembly/error.ts) |
| `error_code` | `export const ERR_RESERVED_IDENTIFIER: i32 = -25;` | [programs/sdk/assemblyscript/assembly/error.ts](../../../programs/sdk/assemblyscript/assembly/error.ts) |
| `error_code` | `export const ERR_DUPLICATE_CAPABILITY: i32 = -26;` | [programs/sdk/assemblyscript/assembly/error.ts](../../../programs/sdk/assemblyscript/assembly/error.ts) |
| `error_code` | `export const ERR_CAPABILITY_LIMIT: i32 = -27;` | [programs/sdk/assemblyscript/assembly/error.ts](../../../programs/sdk/assemblyscript/assembly/error.ts) |
| `error_code` | `export const ERR_CAPABILITY_BYTES: i32 = -28;` | [programs/sdk/assemblyscript/assembly/error.ts](../../../programs/sdk/assemblyscript/assembly/error.ts) |
| `error_code` | `export const ERR_BUFFER_TOO_SMALL: i32 = -29;` | [programs/sdk/assemblyscript/assembly/error.ts](../../../programs/sdk/assemblyscript/assembly/error.ts) |
| `error_code` | `export const ERR_RECEIPT_ENCODING: i32 = -30;` | [programs/sdk/assemblyscript/assembly/error.ts](../../../programs/sdk/assemblyscript/assembly/error.ts) |
| `error_code` | `export const ERR_OVERFLOW: i32 = -31;` | [programs/sdk/assemblyscript/assembly/error.ts](../../../programs/sdk/assemblyscript/assembly/error.ts) |
| `error_code` | `export const ERR_UNDERFLOW: i32 = -32;` | [programs/sdk/assemblyscript/assembly/error.ts](../../../programs/sdk/assemblyscript/assembly/error.ts) |
| `error_code` | `pub const STATUS_DENIED: i32 = -1` | [programs/sdk/rust/src/error.rs](../../../programs/sdk/rust/src/error.rs) |
| `error_code` | `pub const STATUS_INVALID: i32 = -2` | [programs/sdk/rust/src/error.rs](../../../programs/sdk/rust/src/error.rs) |
| `error_code` | `pub const STATUS_BOUNDS: i32 = -3` | [programs/sdk/rust/src/error.rs](../../../programs/sdk/rust/src/error.rs) |
| `error_code` | `pub const STATUS_METER: i32 = -4` | [programs/sdk/rust/src/error.rs](../../../programs/sdk/rust/src/error.rs) |
| `error_code` | `pub const STATUS_EVIDENCE: i32 = -5` | [programs/sdk/rust/src/error.rs](../../../programs/sdk/rust/src/error.rs) |
| `error_code` | `pub const STATUS_VERIFY_FAILED: i32 = -6` | [programs/sdk/rust/src/error.rs](../../../programs/sdk/rust/src/error.rs) |
| `error_code` | `pub const STATUS_NULL_ARGUMENT: i32 = -16` | [programs/sdk/rust/src/error.rs](../../../programs/sdk/rust/src/error.rs) |
| `error_code` | `pub const STATUS_EMPTY_KEY: i32 = -17` | [programs/sdk/rust/src/error.rs](../../../programs/sdk/rust/src/error.rs) |
| `error_code` | `pub const STATUS_KEY_TOO_LARGE: i32 = -18` | [programs/sdk/rust/src/error.rs](../../../programs/sdk/rust/src/error.rs) |
| `error_code` | `pub const STATUS_VALUE_TOO_LARGE: i32 = -19` | [programs/sdk/rust/src/error.rs](../../../programs/sdk/rust/src/error.rs) |
| `error_code` | `pub const STATUS_EMPTY_TOPIC: i32 = -20` | [programs/sdk/rust/src/error.rs](../../../programs/sdk/rust/src/error.rs) |
| `error_code` | `pub const STATUS_TOPIC_TOO_LARGE: i32 = -21` | [programs/sdk/rust/src/error.rs](../../../programs/sdk/rust/src/error.rs) |
| `error_code` | `pub const STATUS_DATA_TOO_LARGE: i32 = -22` | [programs/sdk/rust/src/error.rs](../../../programs/sdk/rust/src/error.rs) |
| `error_code` | `pub const STATUS_INPUT_TOO_LARGE: i32 = -23` | [programs/sdk/rust/src/error.rs](../../../programs/sdk/rust/src/error.rs) |
| `error_code` | `pub const STATUS_ZERO_AMOUNT: i32 = -24` | [programs/sdk/rust/src/error.rs](../../../programs/sdk/rust/src/error.rs) |
| `error_code` | `pub const STATUS_RESERVED_IDENTIFIER: i32 = -25` | [programs/sdk/rust/src/error.rs](../../../programs/sdk/rust/src/error.rs) |
| `error_code` | `pub const STATUS_DUPLICATE_CAPABILITY: i32 = -26` | [programs/sdk/rust/src/error.rs](../../../programs/sdk/rust/src/error.rs) |
| `error_code` | `pub const STATUS_CAPABILITY_LIMIT: i32 = -27` | [programs/sdk/rust/src/error.rs](../../../programs/sdk/rust/src/error.rs) |
| `error_code` | `pub const STATUS_CAPABILITY_BYTES: i32 = -28` | [programs/sdk/rust/src/error.rs](../../../programs/sdk/rust/src/error.rs) |
| `error_code` | `pub const STATUS_BUFFER_TOO_SMALL: i32 = -29` | [programs/sdk/rust/src/error.rs](../../../programs/sdk/rust/src/error.rs) |
| `error_code` | `pub const STATUS_RECEIPT_ENCODING: i32 = -30` | [programs/sdk/rust/src/error.rs](../../../programs/sdk/rust/src/error.rs) |
| `error_code` | `pub const STATUS_OVERFLOW: i32 = -31` | [programs/sdk/rust/src/error.rs](../../../programs/sdk/rust/src/error.rs) |
| `error_code` | `pub const STATUS_UNDERFLOW: i32 = -32` | [programs/sdk/rust/src/error.rs](../../../programs/sdk/rust/src/error.rs) |
| source | — | [programs/sdk/c/include/layerx/program.h](../../../programs/sdk/c/include/layerx/program.h) |

## config-keys

| Kind | Declaration | Source |
|---|---|---|
| `config_key` | `[profile.release] opt-level = 3` | [programs/Cargo.toml](../../../programs/Cargo.toml) |
| `config_key` | `[workspace.dependencies] wasmi = { path = "vendor/wasmi-0.31.2", version = "=0.31.2" }` | [programs/Cargo.toml](../../../programs/Cargo.toml) |
| `config_key` | `[workspace.lints.rust] unsafe_code = "deny"` | [programs/Cargo.toml](../../../programs/Cargo.toml) |
| `config_key` | `[workspace.lints.clippy] all = { level = "deny", priority = -1 }` | [programs/Cargo.toml](../../../programs/Cargo.toml) |
| `config_key` | `[lib] crate-type = ["cdylib"]` | [programs/benches/compiled-equivalent/Cargo.toml](../../../programs/benches/compiled-equivalent/Cargo.toml) |
| `config_key` | `[lib] crate-type = ["rlib", "staticlib"]` | [programs/crates/layerx-programs-runtime/Cargo.toml](../../../programs/crates/layerx-programs-runtime/Cargo.toml) |
| `config_key` | `[features] host-ffi = []` | [programs/crates/layerx-programs-runtime/Cargo.toml](../../../programs/crates/layerx-programs-runtime/Cargo.toml) |
| `config_key` | `[lib] crate-type = ["rlib", "staticlib"]` | [programs/crates/layerx-programs-sandbox/Cargo.toml](../../../programs/crates/layerx-programs-sandbox/Cargo.toml) |
| `config_key` | `[features] host-ffi = ["layerx-programs-runtime/host-ffi"]` | [programs/crates/layerx-programs-sandbox/Cargo.toml](../../../programs/crates/layerx-programs-sandbox/Cargo.toml) |
| `config_key` | `[lib] crate-type = ["cdylib"]` | [programs/sdk/rust/examples/escrow/Cargo.toml](../../../programs/sdk/rust/examples/escrow/Cargo.toml) |
| `config_key` | `[profile.release] ` | [programs/sdk/rust/examples/escrow/Cargo.toml](../../../programs/sdk/rust/examples/escrow/Cargo.toml) |
| `config_key` | `[profile.release] opt-level = "z"` | [programs/sdk/rust/examples/escrow/Cargo.toml](../../../programs/sdk/rust/examples/escrow/Cargo.toml) |
| `config_key` | `[lib] crate-type = ["cdylib"]` | [programs/sdk/rust/examples/naming/Cargo.toml](../../../programs/sdk/rust/examples/naming/Cargo.toml) |
| `config_key` | `[profile.release] ` | [programs/sdk/rust/examples/naming/Cargo.toml](../../../programs/sdk/rust/examples/naming/Cargo.toml) |
| `config_key` | `[profile.release] opt-level = "z"` | [programs/sdk/rust/examples/naming/Cargo.toml](../../../programs/sdk/rust/examples/naming/Cargo.toml) |
| `config_key` | `[lib] crate-type = ["cdylib"]` | [programs/sdk/rust/examples/nft-lxt721/Cargo.toml](../../../programs/sdk/rust/examples/nft-lxt721/Cargo.toml) |
| `config_key` | `[profile.release] ` | [programs/sdk/rust/examples/nft-lxt721/Cargo.toml](../../../programs/sdk/rust/examples/nft-lxt721/Cargo.toml) |
| `config_key` | `[profile.release] opt-level = "z"` | [programs/sdk/rust/examples/nft-lxt721/Cargo.toml](../../../programs/sdk/rust/examples/nft-lxt721/Cargo.toml) |
| `config_key` | `[lib] crate-type = ["cdylib"]` | [programs/sdk/rust/examples/payments-merchant/Cargo.toml](../../../programs/sdk/rust/examples/payments-merchant/Cargo.toml) |
| `config_key` | `[profile.release] ` | [programs/sdk/rust/examples/payments-merchant/Cargo.toml](../../../programs/sdk/rust/examples/payments-merchant/Cargo.toml) |
| `config_key` | `[profile.release] opt-level = "z"` | [programs/sdk/rust/examples/payments-merchant/Cargo.toml](../../../programs/sdk/rust/examples/payments-merchant/Cargo.toml) |
| `config_key` | `[lib] crate-type = ["cdylib"]` | [programs/sdk/rust/examples/swap-cpmm/Cargo.toml](../../../programs/sdk/rust/examples/swap-cpmm/Cargo.toml) |
| `config_key` | `[profile.release] ` | [programs/sdk/rust/examples/swap-cpmm/Cargo.toml](../../../programs/sdk/rust/examples/swap-cpmm/Cargo.toml) |
| `config_key` | `[profile.release] opt-level = "z"` | [programs/sdk/rust/examples/swap-cpmm/Cargo.toml](../../../programs/sdk/rust/examples/swap-cpmm/Cargo.toml) |
| `config_key` | `[lib] crate-type = ["cdylib"]` | [programs/sdk/rust/examples/token-lxt20/Cargo.toml](../../../programs/sdk/rust/examples/token-lxt20/Cargo.toml) |
| `config_key` | `[profile.release] ` | [programs/sdk/rust/examples/token-lxt20/Cargo.toml](../../../programs/sdk/rust/examples/token-lxt20/Cargo.toml) |
| `config_key` | `[profile.release] opt-level = "z"` | [programs/sdk/rust/examples/token-lxt20/Cargo.toml](../../../programs/sdk/rust/examples/token-lxt20/Cargo.toml) |
| `config_key` | `[lib] crate-type = ["cdylib"]` | [programs/sdk/rust/examples/vault/Cargo.toml](../../../programs/sdk/rust/examples/vault/Cargo.toml) |
| `config_key` | `[profile.release] ` | [programs/sdk/rust/examples/vault/Cargo.toml](../../../programs/sdk/rust/examples/vault/Cargo.toml) |
| `config_key` | `[profile.release] opt-level = "z"` | [programs/sdk/rust/examples/vault/Cargo.toml](../../../programs/sdk/rust/examples/vault/Cargo.toml) |
| `config_key` | `[lib] crate-type = ["cdylib"]` | [programs/sdk/rust/examples/web-reader/Cargo.toml](../../../programs/sdk/rust/examples/web-reader/Cargo.toml) |
| `config_key` | `[profile.release] ` | [programs/sdk/rust/examples/web-reader/Cargo.toml](../../../programs/sdk/rust/examples/web-reader/Cargo.toml) |
| `config_key` | `[profile.release] opt-level = "z"` | [programs/sdk/rust/examples/web-reader/Cargo.toml](../../../programs/sdk/rust/examples/web-reader/Cargo.toml) |
| `config_key` | `target = "wasm32-unknown-unknown"` | [programs/sdk/rust/quickstart/.cargo/config.toml](../../../programs/sdk/rust/quickstart/.cargo/config.toml) |
| `config_key` | `[lib] crate-type = ["cdylib"]` | [programs/sdk/rust/quickstart/Cargo.toml](../../../programs/sdk/rust/quickstart/Cargo.toml) |
| `config_key` | `[profile.release] ` | [programs/sdk/rust/quickstart/Cargo.toml](../../../programs/sdk/rust/quickstart/Cargo.toml) |
| `config_key` | `[profile.release] opt-level = "z"` | [programs/sdk/rust/quickstart/Cargo.toml](../../../programs/sdk/rust/quickstart/Cargo.toml) |
| `config_key` | `[lib] crate-type = ["cdylib"]` | [programs/sdk/rust/response-fixture/Cargo.toml](../../../programs/sdk/rust/response-fixture/Cargo.toml) |
| `config_key` | `[profile.release] ` | [programs/sdk/rust/response-fixture/Cargo.toml](../../../programs/sdk/rust/response-fixture/Cargo.toml) |
| `config_key` | `[profile.release] opt-level = "z"` | [programs/sdk/rust/response-fixture/Cargo.toml](../../../programs/sdk/rust/response-fixture/Cargo.toml) |
| `env_var` | `LAYERX_INTERPRETER_BENCH_SAMPLES` | [programs/benches/interpreter.rs](../../../programs/benches/interpreter.rs) |
| `env_var` | `LAYERX_INTERPRETER_WASM` | [programs/benches/interpreter.rs](../../../programs/benches/interpreter.rs) |
| `env_var` | `LAYERX_COMPILED_EQUIVALENT_WASM` | [programs/benches/interpreter.rs](../../../programs/benches/interpreter.rs) |
| `env_var` | `PAXAI_CHAIN_DOMAIN` | [programs/crates/layerx-programs-ai-market/src/lib.rs](../../../programs/crates/layerx-programs-ai-market/src/lib.rs) |
| `env_var` | `LAYERX_MARKET_WASM` | [programs/crates/layerx-programs-runtime/src/market_tests.rs](../../../programs/crates/layerx-programs-runtime/src/market_tests.rs) |
| `env_var` | `LAYERX_INTERFACE_PATH` | [programs/sdk/rust/build.rs](../../../programs/sdk/rust/build.rs) |
| `env_var` | `LAYERX_INTERFACE_DIGEST` | [programs/sdk/rust/build.rs](../../../programs/sdk/rust/build.rs) |
| `env_var` | `LAYERX_PROGRAM_CODE_HASH` | [programs/sdk/rust/build.rs](../../../programs/sdk/rust/build.rs) |
| `env_var` | `LAYERX_BINDINGS_DIR` | [programs/sdk/rust/build.rs](../../../programs/sdk/rust/build.rs) |
| `env_var` | `PAXEER_X_TYPED_INTERFACE_INPUTS` | [programs/sdk/rust/src/bindgen.rs](../../../programs/sdk/rust/src/bindgen.rs) |
| `env_var` | `PAXEER_X_TYPED_CONSUMER_OUTPUT` | [programs/sdk/rust/src/bindgen.rs](../../../programs/sdk/rust/src/bindgen.rs) |
| `env_var` | `PAXEER_X_BINDINGS_EMIT_DIR` | [programs/sdk/rust/tests/bindgen_conformance.rs](../../../programs/sdk/rust/tests/bindgen_conformance.rs) |
| `env_var` | `PAXEER_X_BINDINGS_ARTIFACTS` | [programs/sdk/rust/tests/bindgen_conformance.rs](../../../programs/sdk/rust/tests/bindgen_conformance.rs) |
| `env_var` | `PAXEER_X_BINDINGS_FIXTURES` | [programs/sdk/rust/tests/bindgen_conformance.rs](../../../programs/sdk/rust/tests/bindgen_conformance.rs) |
| source | — | [programs/sdk/assemblyscript/examples/paid-counter/package.json](../../../programs/sdk/assemblyscript/examples/paid-counter/package.json) |
| source | — | [programs/sdk/assemblyscript/package.json](../../../programs/sdk/assemblyscript/package.json) |
| source | — | [programs/sdk/assemblyscript/asconfig.json](../../../programs/sdk/assemblyscript/asconfig.json) |
| source | — | [programs/sdk/assemblyscript/examples/paid-counter/asconfig.json](../../../programs/sdk/assemblyscript/examples/paid-counter/asconfig.json) |
| source | — | [programs/sdk/c/examples/paid-counter/build.sh](../../../programs/sdk/c/examples/paid-counter/build.sh) |
| source | — | [programs/sdk/rust/examples/escrow/build.sh](../../../programs/sdk/rust/examples/escrow/build.sh) |
| source | — | [programs/sdk/rust/examples/naming/build.sh](../../../programs/sdk/rust/examples/naming/build.sh) |
| source | — | [programs/sdk/rust/examples/nft-lxt721/build.sh](../../../programs/sdk/rust/examples/nft-lxt721/build.sh) |
| source | — | [programs/sdk/rust/examples/vault/build.sh](../../../programs/sdk/rust/examples/vault/build.sh) |
| source | — | [programs/sdk/rust/examples/web-reader/build.sh](../../../programs/sdk/rust/examples/web-reader/build.sh) |
| source | — | [programs/sdk/rust/quickstart/build.sh](../../../programs/sdk/rust/quickstart/build.sh) |
| source | — | [programs/sdk/rust/response-fixture/build.sh](../../../programs/sdk/rust/response-fixture/build.sh) |
| source | — | [programs/tests/ai-market/run-aggregation-runtime.sh](../../../programs/tests/ai-market/run-aggregation-runtime.sh) |
| source | — | [programs/tests/conservation/run.sh](../../../programs/tests/conservation/run.sh) |
| source | — | [programs/sdk/assemblyscript/toolchain.json](../../../programs/sdk/assemblyscript/toolchain.json) |
| source | — | [programs/sdk/c/toolchain.json](../../../programs/sdk/c/toolchain.json) |
| source | — | [programs/sdk/rust/toolchain.json](../../../programs/sdk/rust/toolchain.json) |

## fixtures

| Kind | Declaration | Source |
|---|---|---|
| source | — | [programs/fixtures/pay5/account-authorization-vectors.json](../../../programs/fixtures/pay5/account-authorization-vectors.json) |

## test-vectors

| Kind | Declaration | Source |
|---|---|---|
| source | — | [programs/tests/ai-market/run-aggregation-runtime.sh](../../../programs/tests/ai-market/run-aggregation-runtime.sh) |
| source | — | [programs/tests/conservation/run.sh](../../../programs/tests/conservation/run.sh) |
| source | — | [programs/tests/differential/parallel.c](../../../programs/tests/differential/parallel.c) |
| source | — | [programs/sdk/rust/vectors/lxt20-requests.txt](../../../programs/sdk/rust/vectors/lxt20-requests.txt) |
| source | — | [programs/sdk/vectors/capability-boundary.kvx](../../../programs/sdk/vectors/capability-boundary.kvx) |
| source | — | [programs/tests/gauntlet/attack-inventory.tsv](../../../programs/tests/gauntlet/attack-inventory.tsv) |
| source | — | [programs/tests/vectors/calldata/boundaries/depth_and_size.json](../../../programs/tests/vectors/calldata/boundaries/depth_and_size.json) |
| source | — | [programs/tests/vectors/calldata/evm/head_only_layout.json](../../../programs/tests/vectors/calldata/evm/head_only_layout.json) |
| source | — | [programs/tests/vectors/calldata/invalid/malformed.json](../../../programs/tests/vectors/calldata/invalid/malformed.json) |
| source | — | [programs/tests/vectors/calldata/valid/primitives.json](../../../programs/tests/vectors/calldata/valid/primitives.json) |
| source | — | [programs/tests/vectors/terminal-v4-outcome.hex](../../../programs/tests/vectors/terminal-v4-outcome.hex) |
| source | — | [programs/tests/vectors/terminal-v4-schema.json](../../../programs/tests/vectors/terminal-v4-schema.json) |

## fuzz-harness

| Kind | Declaration | Source |
|---|---|---|
| source | — | [programs/fuzz/src/main.rs](../../../programs/fuzz/src/main.rs) |
| source | — | [programs/fuzz/corpus/execution/add.hex](../../../programs/fuzz/corpus/execution/add.hex) |
| source | — | [programs/fuzz/corpus/execution/noadd.hex](../../../programs/fuzz/corpus/execution/noadd.hex) |
| source | — | [programs/fuzz/corpus/instantiation/add.hex](../../../programs/fuzz/corpus/instantiation/add.hex) |
| source | — | [programs/fuzz/corpus/instantiation/memory.hex](../../../programs/fuzz/corpus/instantiation/memory.hex) |
| source | — | [programs/fuzz/corpus/validation/add.hex](../../../programs/fuzz/corpus/validation/add.hex) |
| source | — | [programs/fuzz/corpus/validation/float.hex](../../../programs/fuzz/corpus/validation/float.hex) |
| source | — | [programs/fuzz/corpus/validation/forbidden_import.hex](../../../programs/fuzz/corpus/validation/forbidden_import.hex) |
| source | — | [programs/fuzz/corpus/validation/malformed.hex](../../../programs/fuzz/corpus/validation/malformed.hex) |

## msg-layerxprograms-stageretirementprofile

| Kind | Declaration | Source |
|---|---|---|
| source | — | [api/layerxprograms/tx.proto](../../../api/layerxprograms/tx.proto) |

## msg-layerxprograms-updatefeeschedule

| Kind | Declaration | Source |
|---|---|---|
| source | — | [api/layerxprograms/tx.proto](../../../api/layerxprograms/tx.proto) |

## msg-layerxprograms-updatemeteringschedule

| Kind | Declaration | Source |
|---|---|---|
| source | — | [api/layerxprograms/tx.proto](../../../api/layerxprograms/tx.proto) |

## msg-layerxprograms-updateparams

| Kind | Declaration | Source |
|---|---|---|
| source | — | [api/layerxprograms/tx.proto](../../../api/layerxprograms/tx.proto) |

## proto-layerxprograms-feeschedule

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message FeeSchedule {` | [api/layerxprograms/tx.proto](../../../api/layerxprograms/tx.proto) |

## proto-layerxprograms-genesisstate

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message GenesisState {` | [api/layerxprograms/genesis.proto](../../../api/layerxprograms/genesis.proto) |

## proto-layerxprograms-meteringauthoritykind

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `enum MeteringAuthorityKind {` | [api/layerxprograms/tx.proto](../../../api/layerxprograms/tx.proto) |

## proto-layerxprograms-meteringschedule

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message MeteringSchedule {` | [api/layerxprograms/tx.proto](../../../api/layerxprograms/tx.proto) |

## proto-layerxprograms-migrationprofile

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message MigrationProfile {` | [api/layerxprograms/tx.proto](../../../api/layerxprograms/tx.proto) |

## proto-layerxprograms-msgstageretirementprofile

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message MsgStageRetirementProfile {` | [api/layerxprograms/tx.proto](../../../api/layerxprograms/tx.proto) |

## proto-layerxprograms-msgstageretirementprofileresponse

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `MsgStageRetirementProfileResponse` | [api/layerxprograms/tx.proto](../../../api/layerxprograms/tx.proto) |

## proto-layerxprograms-msgupdatefeeschedule

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message MsgUpdateFeeSchedule {` | [api/layerxprograms/tx.proto](../../../api/layerxprograms/tx.proto) |

## proto-layerxprograms-msgupdatefeescheduleresponse

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `MsgUpdateFeeScheduleResponse` | [api/layerxprograms/tx.proto](../../../api/layerxprograms/tx.proto) |

## proto-layerxprograms-msgupdatemeteringschedule

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message MsgUpdateMeteringSchedule {` | [api/layerxprograms/tx.proto](../../../api/layerxprograms/tx.proto) |

## proto-layerxprograms-msgupdatemeteringscheduleresponse

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `MsgUpdateMeteringScheduleResponse` | [api/layerxprograms/tx.proto](../../../api/layerxprograms/tx.proto) |

## proto-layerxprograms-msgupdateparams

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message MsgUpdateParams {` | [api/layerxprograms/tx.proto](../../../api/layerxprograms/tx.proto) |

## proto-layerxprograms-msgupdateparamsresponse

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `MsgUpdateParamsResponse` | [api/layerxprograms/tx.proto](../../../api/layerxprograms/tx.proto) |

## proto-layerxprograms-params

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message Params {` | [api/layerxprograms/tx.proto](../../../api/layerxprograms/tx.proto) |

## proto-layerxprograms-programbalance

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message ProgramBalance {` | [api/layerxprograms/query.proto](../../../api/layerxprograms/query.proto) |

## proto-layerxprograms-programblob

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message ProgramBlob {` | [api/layerxprograms/genesis.proto](../../../api/layerxprograms/genesis.proto) |

## proto-layerxprograms-programevent

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message ProgramEvent {` | [api/layerxprograms/query.proto](../../../api/layerxprograms/query.proto) |

## proto-layerxprograms-programstoreentry

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message ProgramStoreEntry {` | [api/layerxprograms/genesis.proto](../../../api/layerxprograms/genesis.proto) |

## proto-layerxprograms-queryprogrambalancesrequest

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message QueryProgramBalancesRequest {` | [api/layerxprograms/query.proto](../../../api/layerxprograms/query.proto) |

## proto-layerxprograms-queryprogrambalancesresponse

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message QueryProgramBalancesResponse {` | [api/layerxprograms/query.proto](../../../api/layerxprograms/query.proto) |

## proto-layerxprograms-queryprogrameventsrequest

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message QueryProgramEventsRequest {` | [api/layerxprograms/query.proto](../../../api/layerxprograms/query.proto) |

## proto-layerxprograms-queryprogrameventsresponse

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message QueryProgramEventsResponse {` | [api/layerxprograms/query.proto](../../../api/layerxprograms/query.proto) |

## proto-layerxprograms-queryprogramstaterequest

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message QueryProgramStateRequest {` | [api/layerxprograms/query.proto](../../../api/layerxprograms/query.proto) |

## proto-layerxprograms-queryprogramstateresponse

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message QueryProgramStateResponse {` | [api/layerxprograms/query.proto](../../../api/layerxprograms/query.proto) |

## proto-layerxprograms-retirementprofile

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message RetirementProfile {` | [api/layerxprograms/tx.proto](../../../api/layerxprograms/tx.proto) |

See the implementation sources above.

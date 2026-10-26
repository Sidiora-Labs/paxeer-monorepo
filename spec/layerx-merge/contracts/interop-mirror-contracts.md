# Interop mirror contracts contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## cfg-mirror-archive-foundry

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.cfg-mirror-archive-foundry</code> | <code>config_key</code> | <code>stable</code> | <code>services/interop/contracts/ethereum-mirror/foundry.toml:1</code> | <code>[profile.default] src ., test test, solc_version 0.8.30, optimizer true, optimizer_runs 100000, via_ir true, bytecode_hash none, cbor_metadata false, out and cache under build/</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.cfg-mirror-archive-foundry</code> | notes | compiler settings must match the runtime_code_keccak256 post-deploy record |

## err-mirror-archive-archive-digest-mismatch

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-mirror-archive-archive-digest-mismatch</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol:74</code> | <code>error ArchiveDigestMismatch()</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-mirror-archive-archive-digest-mismatch</code> | selector | 0xdaf64659 |
| <code>interface.11.err-mirror-archive-archive-digest-mismatch</code> | notes | custom error selector 0xdaf64659 returned as revert data Address: per deployment: constructor publisher_ argument and contract_address are post-deploy records of services/interop/deploy/mirror/deploy-ethereum-mirror.sh. |

## err-mirror-archive-chunk-conflict

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-mirror-archive-chunk-conflict</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol:71</code> | <code>error ChunkConflict()</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-mirror-archive-chunk-conflict</code> | selector | 0x98c2cf7f |
| <code>interface.11.err-mirror-archive-chunk-conflict</code> | notes | custom error selector 0x98c2cf7f returned as revert data Address: per deployment: constructor publisher_ argument and contract_address are post-deploy records of services/interop/deploy/mirror/deploy-ethereum-mirror.sh. |

## err-mirror-archive-chunk-order

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-mirror-archive-chunk-order</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol:70</code> | <code>error ChunkOrder()</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-mirror-archive-chunk-order</code> | selector | 0xfe14b004 |
| <code>interface.11.err-mirror-archive-chunk-order</code> | notes | custom error selector 0xfe14b004 returned as revert data Address: per deployment: constructor publisher_ argument and contract_address are post-deploy records of services/interop/deploy/mirror/deploy-ethereum-mirror.sh. |

## err-mirror-archive-incomplete-archive

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-mirror-archive-incomplete-archive</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol:72</code> | <code>error IncompleteArchive()</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-mirror-archive-incomplete-archive</code> | selector | 0xa17b3a47 |
| <code>interface.11.err-mirror-archive-incomplete-archive</code> | notes | custom error selector 0xa17b3a47 returned as revert data Address: per deployment: constructor publisher_ argument and contract_address are post-deploy records of services/interop/deploy/mirror/deploy-ethereum-mirror.sh. |

## err-mirror-archive-invalid-manifest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-mirror-archive-invalid-manifest</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol:68</code> | <code>error InvalidManifest()</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-mirror-archive-invalid-manifest</code> | selector | 0xe18d5d12 |
| <code>interface.11.err-mirror-archive-invalid-manifest</code> | notes | custom error selector 0xe18d5d12 returned as revert data Address: per deployment: constructor publisher_ argument and contract_address are post-deploy records of services/interop/deploy/mirror/deploy-ethereum-mirror.sh. |

## err-mirror-archive-invalid-publisher

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-mirror-archive-invalid-publisher</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol:73</code> | <code>error InvalidPublisher()</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-mirror-archive-invalid-publisher</code> | selector | 0x28eaff96 |
| <code>interface.11.err-mirror-archive-invalid-publisher</code> | notes | custom error selector 0x28eaff96 returned as revert data Address: per deployment: constructor publisher_ argument and contract_address are post-deploy records of services/interop/deploy/mirror/deploy-ethereum-mirror.sh. |

## err-mirror-archive-manifest-conflict

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-mirror-archive-manifest-conflict</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol:69</code> | <code>error ManifestConflict()</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-mirror-archive-manifest-conflict</code> | selector | 0xd360c955 |
| <code>interface.11.err-mirror-archive-manifest-conflict</code> | notes | custom error selector 0xd360c955 returned as revert data Address: per deployment: constructor publisher_ argument and contract_address are post-deploy records of services/interop/deploy/mirror/deploy-ethereum-mirror.sh. |

## err-solana-mirror-archive-digest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-solana-mirror-archive-digest</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/contracts/solana-mirror/src/lib.rs:38</code> | <code>ProgramError::Custom(8) MirrorError::ArchiveDigest</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-solana-mirror-archive-digest</code> | notes | streamed SHA-256 differs from archive_digest; surfaced to the mirror Solana publisher as a custom program error in the transaction status |

## err-solana-mirror-authority

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-solana-mirror-authority</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/contracts/solana-mirror/src/lib.rs:32</code> | <code>ProgramError::Custom(2) MirrorError::Authority</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-solana-mirror-authority</code> | notes | payer not signer, wrong system program or non-writable account; surfaced to the mirror Solana publisher as a custom program error in the transaction status |

## err-solana-mirror-bounds

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-solana-mirror-bounds</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/contracts/solana-mirror/src/lib.rs:36</code> | <code>ProgramError::Custom(6) MirrorError::Bounds</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-solana-mirror-bounds</code> | notes | zero or out-of-bound field, digest mismatch on append; surfaced to the mirror Solana publisher as a custom program error in the transaction status |

## err-solana-mirror-conflict

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-solana-mirror-conflict</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/contracts/solana-mirror/src/lib.rs:34</code> | <code>ProgramError::Custom(4) MirrorError::Conflict</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-solana-mirror-conflict</code> | notes | re-initialize or re-append with different content, or append after finalize; surfaced to the mirror Solana publisher as a custom program error in the transaction status |

## err-solana-mirror-incomplete

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-solana-mirror-incomplete</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/contracts/solana-mirror/src/lib.rs:37</code> | <code>ProgramError::Custom(7) MirrorError::Incomplete</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-solana-mirror-incomplete</code> | notes | finalize before all chunks and bytes arrived or chunk chain mismatch; surfaced to the mirror Solana publisher as a custom program error in the transaction status |

## err-solana-mirror-instruction

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-solana-mirror-instruction</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/contracts/solana-mirror/src/lib.rs:31</code> | <code>ProgramError::Custom(1) MirrorError::Instruction</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-solana-mirror-instruction</code> | notes | bad magic, version, op or trailing bytes; surfaced to the mirror Solana publisher as a custom program error in the transaction status |

## err-solana-mirror-order

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-solana-mirror-order</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/contracts/solana-mirror/src/lib.rs:35</code> | <code>ProgramError::Custom(5) MirrorError::Order</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-solana-mirror-order</code> | notes | chunk index is not next_chunk; surfaced to the mirror Solana publisher as a custom program error in the transaction status |

## err-solana-mirror-pda

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.err-solana-mirror-pda</code> | <code>error_code</code> | <code>stable</code> | <code>services/interop/contracts/solana-mirror/src/lib.rs:33</code> | <code>ProgramError::Custom(3) MirrorError::Pda</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.err-solana-mirror-pda</code> | notes | manifest or chunk address is not the derived PDA; surfaced to the mirror Solana publisher as a custom program error in the transaction status |

## fmt-mirror-archive-chunk-carrier

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-mirror-archive-chunk-carrier</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol:6</code> | <code>ArchiveChunk runtime code = 0x00 &#124;&#124; chunk bytes; a full 24576-byte chunk is split into a 24575-byte first carrier and a 1-byte last carrier; chunk() reads them back with extcodecopy from offset 1</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-mirror-archive-chunk-carrier</code> | notes | internal storage layout of chunk bytes, observable through eth_getCode on the carrier addresses |

## fmt-mirror-archive-chunk-chain

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-mirror-archive-chunk-chain</code> | <code>file_format</code> | <code>stable</code> | <code>services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol:147</code> | <code>observedChunkChain_i = keccak256(abi.encodePacked(bytes32 previous, uint32 index, bytes32 sha256(chunk), uint32 chunkBytes)), starting from bytes32(0); finalize requires it to equal expectedChunkChain</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-mirror-archive-chunk-chain</code> | notes | the Solana program folds the same chain with SHA-256; archive digest is the streamed SHA-256 of all chunk bytes (ArchiveSha256 library) |

## fmt-solana-mirror-chunk-account

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-solana-mirror-chunk-account</code> | <code>file_format</code> | <code>change</code> | <code>services/interop/contracts/solana-mirror/src/lib.rs:22</code> | <code>chunk PDA ["chunk", publisher, commitment, index u32be]: 80-byte header "LXMCHK03" &#124; commitment &#124; index u32be &#124; sha256(value) &#124; length u32be, followed by value (1..720 bytes)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-solana-mirror-chunk-account</code> | notes | limits: MAX_ARCHIVE_BYTES 64 MiB (services/interop/contracts/solana-mirror/src/lib.rs:25), MAX_CHUNK_BYTES 720 (services/interop/contracts/solana-mirror/src/lib.rs:26), MAX_CHUNKS 524288 (services/interop/contracts/solana-mirror/src/lib.rs:27) |
| <code>interface.11.fmt-solana-mirror-chunk-account</code> | overridden_by | override.J48.27 |

Contract context <code>contract_context.11.mirror-contracts</code>:

| Field | Frozen rule |
|---|---|
| interface | LayerXMirrorArchive on Ethereum and the Solana mirror program accept v4 chunks. |
| storage | Chunk accounts and archive records keyed by publisher, commitment and index. |
| errors | Reverts and program errors on version or size mismatch. |
| events | Archive chunk events on Ethereum. |
| gas | Per-chunk storage cost. |
| activation | Instruction version 4. |
| invariants | A commitment is finalised once. |
| pause | none |

## fmt-solana-mirror-instruction

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-solana-mirror-instruction</code> | <code>file_format</code> | <code>change</code> | <code>services/interop/contracts/solana-mirror/src/lib.rs:19</code> | <code>instruction = "LXMA" &#124; version:u16be=3 &#124; op:u8; op 1 initialize: commitment:bytes32 &#124; network_id:u32be &#124; batch_number:u64be &#124; checkpoint_id:bytes32 &#124; total_bytes:u64be &#124; total_chunks:u32be &#124; archive_digest:bytes32 &#124; expected_chain:bytes32, accounts [payer signer, manifest PDA writable, system program]; op 2 append: commitment:bytes32 &#124; index:u32be &#124; length:u16be &#124; value[length] &#124; sha256(value):bytes32, accounts [payer, manifest, chunk PDA, system]; op 3 finalize: commitment:bytes32, accounts [payer, manifest]; trailing bytes refused</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-solana-mirror-instruction</code> | notes | encoded by the mirror Solana publisher (services/interop/crates/layerx-mirror/src/solana.rs:24); checkpoint_id field: stale task 7.7.5 do_3 changes the archive payload to the committed header and app hash (the Ethereum archive) and do_5 updates services/interop/contracts/ethereum-mirror/test/LayerXMirrorArchive.t.sol:82; at HEAD the manifest still carries checkpointId and the Solana payload mirrors it |

Contract context <code>contract_context.11.mirror-contracts</code>:

| Field | Frozen rule |
|---|---|
| interface | LayerXMirrorArchive on Ethereum and the Solana mirror program accept v4 chunks. |
| storage | Chunk accounts and archive records keyed by publisher, commitment and index. |
| errors | Reverts and program errors on version or size mismatch. |
| events | Archive chunk events on Ethereum. |
| gas | Per-chunk storage cost. |
| activation | Instruction version 4. |
| invariants | A commitment is finalised once. |
| pause | none |

## fmt-solana-mirror-manifest-account

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-solana-mirror-manifest-account</code> | <code>file_format</code> | <code>change</code> | <code>services/interop/contracts/solana-mirror/src/lib.rs:21</code> | <code>manifest PDA ["manifest", publisher, commitment], 334 bytes: [0..8] "LXMMAN03" &#124; [8..40] commitment &#124; [40..72] publisher &#124; [72..76] network_id u32be &#124; [76..84] batch_number u64be &#124; [84..116] checkpoint_id &#124; [116..124] total_bytes u64be &#124; [124..128] total_chunks u32be &#124; [128..160] archive_digest &#124; [160..192] expected_chain &#124; [192..224] observed chain &#124; [224..232] received_bytes u64be &#124; [232..236] next_chunk u32be &#124; [236] finalized 0/1 &#124; [237..334] streamed SHA-256 state</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-solana-mirror-manifest-account</code> | notes | read by the mirror Solana publisher and verifier through getAccountInfo; checkpoint_id at [84..116] follows 7.7.5 |

Contract context <code>contract_context.11.mirror-contracts</code>:

| Field | Frozen rule |
|---|---|
| interface | LayerXMirrorArchive on Ethereum and the Solana mirror program accept v4 chunks. |
| storage | Chunk accounts and archive records keyed by publisher, commitment and index. |
| errors | Reverts and program errors on version or size mismatch. |
| events | Archive chunk events on Ethereum. |
| gas | Per-chunk storage cost. |
| activation | Instruction version 4. |
| invariants | A commitment is finalised once. |
| pause | none |

## rs-solana-mirror-process-instruction

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-solana-mirror-process-instruction</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/contracts/solana-mirror/src/lib.rs:47</code> | <code>pub fn process_instruction(program_id: &amp;Pubkey, accounts: &amp;[AccountInfo&lt;'_&gt;], instruction: &amp;[u8]) -&gt; ProgramResult</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-solana-mirror-process-instruction</code> | notes | crate layerx-solana-mirror-program (cdylib + lib); program entrypoint; dispatches op 1 initialize, 2 append, 3 finalize |

## sevt-mirror-archive-archive-finalized

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.sevt-mirror-archive-archive-finalized</code> | <code>sol_event</code> | <code>stable</code> | <code>services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol:66</code> | <code>event ArchiveFinalized(bytes32 indexed commitment, bytes32 archiveDigest)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.sevt-mirror-archive-archive-finalized</code> | selector | 0x7591caef980424e48fc1150ff38be523904603a408938644ace6e1114863896d |
| <code>interface.11.sevt-mirror-archive-archive-finalized</code> | notes | ArchiveFinalized(bytes32,bytes32); emitted by finalize once Address: per deployment: constructor publisher_ argument and contract_address are post-deploy records of services/interop/deploy/mirror/deploy-ethereum-mirror.sh. |

## sevt-mirror-archive-chunk-stored

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.sevt-mirror-archive-chunk-stored</code> | <code>sol_event</code> | <code>stable</code> | <code>services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol:60</code> | <code>event ChunkStored(bytes32 indexed commitment, uint32 index, bytes32 chunkDigest, uint32 chunkBytes)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.sevt-mirror-archive-chunk-stored</code> | selector | 0x55d830d363f6491a3deb7936a7565fe229395bd4c1de693b791dd69ef1a05ae8 |
| <code>interface.11.sevt-mirror-archive-chunk-stored</code> | notes | ChunkStored(bytes32,uint32,bytes32,uint32); emitted by append for each newly stored chunk Address: per deployment: constructor publisher_ argument and contract_address are post-deploy records of services/interop/deploy/mirror/deploy-ethereum-mirror.sh. |

## sevt-mirror-archive-manifest-opened

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.sevt-mirror-archive-manifest-opened</code> | <code>sol_event</code> | <code>change</code> | <code>services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol:52</code> | <code>event ManifestOpened(bytes32 indexed commitment, uint32 networkId, uint64 batchNumber, uint64 totalBytes, uint32 totalChunks, bytes32 archiveDigest)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.sevt-mirror-archive-manifest-opened</code> | selector | 0x74328fcffd640a655d187b38eda3d1860b658de352a1a30a1c481319c87ba376 |
| <code>interface.11.sevt-mirror-archive-manifest-opened</code> | notes | ManifestOpened(bytes32,uint32,uint64,uint64,uint32,bytes32); emitted by begin on first open; stale task 7.7.5 do_3 changes the archive payload to the committed header and app hash (services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol:81, :126) and do_5 updates services/interop/contracts/ethereum-mirror/test/LayerXMirrorArchive.t.sol:82; at HEAD the manifest still carries checkpointId (the event fields themselves omit checkpointId) Address: per deployment: constructor publisher_ argument and contract_address are post-deploy records of services/interop/deploy/mirror/deploy-ethereum-mirror.sh. |

Contract context <code>contract_context.11.mirror-contracts</code>:

| Field | Frozen rule |
|---|---|
| interface | LayerXMirrorArchive on Ethereum and the Solana mirror program accept v4 chunks. |
| storage | Chunk accounts and archive records keyed by publisher, commitment and index. |
| errors | Reverts and program errors on version or size mismatch. |
| events | Archive chunk events on Ethereum. |
| gas | Per-chunk storage cost. |
| activation | Instruction version 4. |
| invariants | A commitment is finalised once. |
| pause | none |

## sol-mirror-archive-append

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.sol-mirror-archive-append</code> | <code>sol_function</code> | <code>change</code> | <code>services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol:126</code> | <code>function append(bytes32 commitment, uint32 index, bytes calldata value) external onlyPublisher</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.sol-mirror-archive-append</code> | selector | 0xf6a828db |
| <code>interface.11.sol-mirror-archive-append</code> | notes | append(bytes32,uint32,bytes); stale task 7.7.5 do_3 changes the archive payload to the committed header and app hash (services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol:81, :126) and do_5 updates services/interop/contracts/ethereum-mirror/test/LayerXMirrorArchive.t.sol:82; at HEAD the manifest still carries checkpointId; stores one chunk of at most MAX_CHUNK_BYTES in one or two code-carrier contracts, folds observedChunkChain = keccak256(prev, index, sha256(value), uint32 len); reverts InvalidManifest, ChunkOrder, ChunkConflict; emits ChunkStored Address: per deployment: constructor publisher_ argument and contract_address are post-deploy records of services/interop/deploy/mirror/deploy-ethereum-mirror.sh. |

Contract context <code>contract_context.11.mirror-contracts</code>:

| Field | Frozen rule |
|---|---|
| interface | LayerXMirrorArchive on Ethereum and the Solana mirror program accept v4 chunks. |
| storage | Chunk accounts and archive records keyed by publisher, commitment and index. |
| errors | Reverts and program errors on version or size mismatch. |
| events | Archive chunk events on Ethereum. |
| gas | Per-chunk storage cost. |
| activation | Instruction version 4. |
| invariants | A commitment is finalised once. |
| pause | none |

## sol-mirror-archive-begin

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.sol-mirror-archive-begin</code> | <code>sol_function</code> | <code>change</code> | <code>services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol:81</code> | <code>function begin(bytes32 commitment, uint32 networkId, uint64 batchNumber, bytes32 checkpointId, uint64 totalBytes, uint32 totalChunks, bytes32 archiveDigest, bytes32 expectedChunkChain) external onlyPublisher</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.sol-mirror-archive-begin</code> | selector | 0x77afdf6c |
| <code>interface.11.sol-mirror-archive-begin</code> | notes | begin(bytes32,uint32,uint64,bytes32,uint64,uint32,bytes32,bytes32); stale task 7.7.5 do_3 changes the archive payload to the committed header and app hash (services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol:81, :126) and do_5 updates services/interop/contracts/ethereum-mirror/test/LayerXMirrorArchive.t.sol:82; at HEAD the manifest still carries checkpointId; opens or idempotently re-checks a manifest; reverts InvalidManifest on zero or out-of-bound fields, ManifestConflict on a differing re-open; emits ManifestOpened Address: per deployment: constructor publisher_ argument and contract_address are post-deploy records of services/interop/deploy/mirror/deploy-ethereum-mirror.sh. |

Contract context <code>contract_context.11.mirror-contracts</code>:

| Field | Frozen rule |
|---|---|
| interface | LayerXMirrorArchive on Ethereum and the Solana mirror program accept v4 chunks. |
| storage | Chunk accounts and archive records keyed by publisher, commitment and index. |
| errors | Reverts and program errors on version or size mismatch. |
| events | Archive chunk events on Ethereum. |
| gas | Per-chunk storage cost. |
| activation | Instruction version 4. |
| invariants | A commitment is finalised once. |
| pause | none |

## sol-mirror-archive-chunk

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.sol-mirror-archive-chunk</code> | <code>sol_function</code> | <code>stable</code> | <code>services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol:181</code> | <code>function chunk(bytes32 commitment, uint32 index) external view returns (bytes)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.sol-mirror-archive-chunk</code> | selector | 0x9ad9ea40 |
| <code>interface.11.sol-mirror-archive-chunk</code> | notes | chunk(bytes32,uint32); view; returns empty bytes for an absent chunk; pinned as chunk_getter in sdk/platform/schema/mirror-v2.kvx:35 Address: per deployment: constructor publisher_ argument and contract_address are post-deploy records of services/interop/deploy/mirror/deploy-ethereum-mirror.sh. |

## sol-mirror-archive-constructor

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.sol-mirror-archive-constructor</code> | <code>sol_function</code> | <code>stable</code> | <code>services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol:24</code> | <code>constructor(address publisher_)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.sol-mirror-archive-constructor</code> | notes | reverts InvalidPublisher for the zero address; deployed by services/interop/deploy/mirror/deploy-ethereum-mirror.sh with forge create; no payable, custody or token surface Address: per deployment: constructor publisher_ argument and contract_address are post-deploy records of services/interop/deploy/mirror/deploy-ethereum-mirror.sh. |

## sol-mirror-archive-finalize

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.sol-mirror-archive-finalize</code> | <code>sol_function</code> | <code>stable</code> | <code>services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol:155</code> | <code>function finalize(bytes32 commitment) external onlyPublisher</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.sol-mirror-archive-finalize</code> | selector | 0x92584d80 |
| <code>interface.11.sol-mirror-archive-finalize</code> | notes | finalize(bytes32); idempotent; reverts IncompleteArchive when chunks, bytes or chunk chain disagree, ArchiveDigestMismatch when the streamed SHA-256 differs; emits ArchiveFinalized Address: per deployment: constructor publisher_ argument and contract_address are post-deploy records of services/interop/deploy/mirror/deploy-ethereum-mirror.sh. |

## sol-mirror-archive-manifest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.sol-mirror-archive-manifest</code> | <code>sol_function</code> | <code>stable</code> | <code>services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol:170</code> | <code>function manifest(bytes32 commitment) external view returns (uint64 totalBytes, uint32 totalChunks, bytes32 archiveDigest, bool finalized)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.sol-mirror-archive-manifest</code> | selector | 0x95b4410e |
| <code>interface.11.sol-mirror-archive-manifest</code> | notes | manifest(bytes32); view read by the mirror Ethereum publisher and verifier; also pinned as manifest_getter in sdk/platform/schema/mirror-v2.kvx:34 Address: per deployment: constructor publisher_ argument and contract_address are post-deploy records of services/interop/deploy/mirror/deploy-ethereum-mirror.sh. |

## sol-mirror-archive-max-archive-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.sol-mirror-archive-max-archive-bytes</code> | <code>sol_function</code> | <code>stable</code> | <code>services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol:19</code> | <code>uint256 public constant MAX_ARCHIVE_BYTES = 64 * 1024 * 1024</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.sol-mirror-archive-max-archive-bytes</code> | selector | 0x8a45a4ea |
| <code>interface.11.sol-mirror-archive-max-archive-bytes</code> | notes | MAX_ARCHIVE_BYTES(); public constant getter Address: per deployment: constructor publisher_ argument and contract_address are post-deploy records of services/interop/deploy/mirror/deploy-ethereum-mirror.sh. |

## sol-mirror-archive-max-chunk-bytes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.sol-mirror-archive-max-chunk-bytes</code> | <code>sol_function</code> | <code>stable</code> | <code>services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol:20</code> | <code>uint256 public constant MAX_CHUNK_BYTES = 24 * 1024</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.sol-mirror-archive-max-chunk-bytes</code> | selector | 0xb632ab57 |
| <code>interface.11.sol-mirror-archive-max-chunk-bytes</code> | notes | MAX_CHUNK_BYTES(); public constant getter Address: per deployment: constructor publisher_ argument and contract_address are post-deploy records of services/interop/deploy/mirror/deploy-ethereum-mirror.sh. |

## sol-mirror-archive-max-chunks

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.sol-mirror-archive-max-chunks</code> | <code>sol_function</code> | <code>stable</code> | <code>services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol:21</code> | <code>uint256 public constant MAX_CHUNKS = 65_536</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.sol-mirror-archive-max-chunks</code> | selector | 0x5ed25346 |
| <code>interface.11.sol-mirror-archive-max-chunks</code> | notes | MAX_CHUNKS(); public constant getter Address: per deployment: constructor publisher_ argument and contract_address are post-deploy records of services/interop/deploy/mirror/deploy-ethereum-mirror.sh. |

## sol-mirror-archive-publisher

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.sol-mirror-archive-publisher</code> | <code>sol_function</code> | <code>stable</code> | <code>services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol:22</code> | <code>address public immutable publisher</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.sol-mirror-archive-publisher</code> | selector | 0x8c72c54e |
| <code>interface.11.sol-mirror-archive-publisher</code> | notes | publisher(); public immutable getter; the only account allowed to call begin, append and finalize (onlyPublisher) Address: per deployment: constructor publisher_ argument and contract_address are post-deploy records of services/interop/deploy/mirror/deploy-ethereum-mirror.sh. |

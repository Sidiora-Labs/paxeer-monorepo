# Interop mirror contracts contract

The [Ethereum archive contract](../../../services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol) and [Solana archive program](../../../services/interop/contracts/solana-mirror) store chunked mirror archives under publisher authority.

## cfg-mirror-archive-foundry

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.cfg-mirror-archive-foundry</code> | <code>config_key</code> | [services/interop/contracts/ethereum-mirror/foundry.toml](../../../services/interop/contracts/ethereum-mirror/foundry.toml) | <code>[profile.default] src ., test test, solc_version 0.8.30, optimizer true, optimizer_runs 100000, via_ir true, bytecode_hash none, cbor_metadata false, out and cache under build/</code> |

## err-mirror-archive-archive-digest-mismatch

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-mirror-archive-archive-digest-mismatch</code> | <code>error_code</code> | [services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol](../../../services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol) | <code>error ArchiveDigestMismatch()</code> |

## err-mirror-archive-chunk-conflict

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-mirror-archive-chunk-conflict</code> | <code>error_code</code> | [services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol](../../../services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol) | <code>error ChunkConflict()</code> |

## err-mirror-archive-chunk-order

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-mirror-archive-chunk-order</code> | <code>error_code</code> | [services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol](../../../services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol) | <code>error ChunkOrder()</code> |

## err-mirror-archive-incomplete-archive

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-mirror-archive-incomplete-archive</code> | <code>error_code</code> | [services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol](../../../services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol) | <code>error IncompleteArchive()</code> |

## err-mirror-archive-invalid-manifest

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-mirror-archive-invalid-manifest</code> | <code>error_code</code> | [services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol](../../../services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol) | <code>error InvalidManifest()</code> |

## err-mirror-archive-invalid-publisher

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-mirror-archive-invalid-publisher</code> | <code>error_code</code> | [services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol](../../../services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol) | <code>error InvalidPublisher()</code> |

## err-mirror-archive-manifest-conflict

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-mirror-archive-manifest-conflict</code> | <code>error_code</code> | [services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol](../../../services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol) | <code>error ManifestConflict()</code> |

## err-solana-mirror-archive-digest

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-solana-mirror-archive-digest</code> | <code>error_code</code> | [services/interop/contracts/solana-mirror/src/lib.rs](../../../services/interop/contracts/solana-mirror/src/lib.rs) | <code>ProgramError::Custom(8) MirrorError::ArchiveDigest</code> |

## err-solana-mirror-authority

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-solana-mirror-authority</code> | <code>error_code</code> | [services/interop/contracts/solana-mirror/src/lib.rs](../../../services/interop/contracts/solana-mirror/src/lib.rs) | <code>ProgramError::Custom(2) MirrorError::Authority</code> |

## err-solana-mirror-bounds

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-solana-mirror-bounds</code> | <code>error_code</code> | [services/interop/contracts/solana-mirror/src/lib.rs](../../../services/interop/contracts/solana-mirror/src/lib.rs) | <code>ProgramError::Custom(6) MirrorError::Bounds</code> |

## err-solana-mirror-conflict

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-solana-mirror-conflict</code> | <code>error_code</code> | [services/interop/contracts/solana-mirror/src/lib.rs](../../../services/interop/contracts/solana-mirror/src/lib.rs) | <code>ProgramError::Custom(4) MirrorError::Conflict</code> |

## err-solana-mirror-incomplete

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-solana-mirror-incomplete</code> | <code>error_code</code> | [services/interop/contracts/solana-mirror/src/lib.rs](../../../services/interop/contracts/solana-mirror/src/lib.rs) | <code>ProgramError::Custom(7) MirrorError::Incomplete</code> |

## err-solana-mirror-instruction

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-solana-mirror-instruction</code> | <code>error_code</code> | [services/interop/contracts/solana-mirror/src/lib.rs](../../../services/interop/contracts/solana-mirror/src/lib.rs) | <code>ProgramError::Custom(1) MirrorError::Instruction</code> |

## err-solana-mirror-order

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-solana-mirror-order</code> | <code>error_code</code> | [services/interop/contracts/solana-mirror/src/lib.rs](../../../services/interop/contracts/solana-mirror/src/lib.rs) | <code>ProgramError::Custom(5) MirrorError::Order</code> |

## err-solana-mirror-pda

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.err-solana-mirror-pda</code> | <code>error_code</code> | [services/interop/contracts/solana-mirror/src/lib.rs](../../../services/interop/contracts/solana-mirror/src/lib.rs) | <code>ProgramError::Custom(3) MirrorError::Pda</code> |

## fmt-mirror-archive-chunk-carrier

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-mirror-archive-chunk-carrier</code> | <code>file_format</code> | [services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol](../../../services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol) | <code>ArchiveChunk runtime code = 0x00 &#124;&#124; chunk bytes; a full 24576-byte chunk is split into a 24575-byte first carrier and a 1-byte last carrier; chunk() reads them back with extcodecopy from offset 1</code> |

## fmt-mirror-archive-chunk-chain

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-mirror-archive-chunk-chain</code> | <code>file_format</code> | [services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol](../../../services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol) | <code>observedChunkChain_i = keccak256(abi.encodePacked(bytes32 previous, uint32 index, bytes32 sha256(chunk), uint32 chunkBytes)), starting from bytes32(0); finalize requires it to equal expectedChunkChain</code> |

## fmt-solana-mirror-chunk-account

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-solana-mirror-chunk-account</code> | <code>file_format</code> | [services/interop/contracts/solana-mirror/src/lib.rs](../../../services/interop/contracts/solana-mirror/src/lib.rs) | <code>chunk PDA ["chunk", publisher, commitment, index u32be]: 80-byte header "LXMCHK04" &#124; commitment &#124; index u32be &#124; sha256(value) &#124; length u32be, followed by value (1..720 bytes)</code> |

| Field | Rule |
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

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-solana-mirror-instruction</code> | <code>file_format</code> | [services/interop/contracts/solana-mirror/src/lib.rs](../../../services/interop/contracts/solana-mirror/src/lib.rs) | <code>instruction = "LXMA" &#124; version:u16be=4 &#124; op:u8; op 1 initialize: commitment:bytes32 &#124; network_id:u32be &#124; height:u64be &#124; block_hash:bytes32 &#124; total_bytes:u64be &#124; total_chunks:u32be &#124; archive_digest:bytes32 &#124; expected_chain:bytes32, accounts [payer signer, manifest PDA writable, system program]; op 2 append: commitment:bytes32 &#124; index:u32be &#124; length:u16be &#124; value[length] &#124; sha256(value):bytes32, accounts [payer, manifest, chunk PDA, system]; op 3 finalize: commitment:bytes32, accounts [payer, manifest]; trailing bytes refused</code> |

| Field | Rule |
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

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-solana-mirror-manifest-account</code> | <code>file_format</code> | [services/interop/contracts/solana-mirror/src/lib.rs](../../../services/interop/contracts/solana-mirror/src/lib.rs) | <code>manifest PDA ["manifest", publisher, commitment], 334 bytes: [0..8] "LXMMAN04" &#124; [8..40] commitment &#124; [40..72] publisher &#124; [72..76] network_id u32be &#124; [76..84] height u64be &#124; [84..116] block_hash &#124; [116..124] total_bytes u64be &#124; [124..128] total_chunks u32be &#124; [128..160] archive_digest &#124; [160..192] expected_chain &#124; [192..224] observed chain &#124; [224..232] received_bytes u64be &#124; [232..236] next_chunk u32be &#124; [236] finalized 0/1 &#124; [237..334] streamed SHA-256 state</code> |

| Field | Rule |
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

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-solana-mirror-process-instruction</code> | <code>rust_pub_item</code> | [services/interop/contracts/solana-mirror/src/lib.rs](../../../services/interop/contracts/solana-mirror/src/lib.rs) | <code>pub fn process_instruction(program_id: &amp;Pubkey, accounts: &amp;[AccountInfo&lt;'_&gt;], instruction: &amp;[u8]) -&gt; ProgramResult</code> |

## sevt-mirror-archive-archive-finalized

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.sevt-mirror-archive-archive-finalized</code> | <code>sol_event</code> | [services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol](../../../services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol) | <code>event ArchiveFinalized(bytes32 indexed commitment, bytes32 archiveDigest)</code> |

## sevt-mirror-archive-chunk-stored

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.sevt-mirror-archive-chunk-stored</code> | <code>sol_event</code> | [services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol](../../../services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol) | <code>event ChunkStored(bytes32 indexed commitment, uint32 index, bytes32 chunkDigest, uint32 chunkBytes)</code> |

## sevt-mirror-archive-manifest-opened

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.sevt-mirror-archive-manifest-opened</code> | <code>sol_event</code> | [services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol](../../../services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol) | <code>event ManifestOpened(bytes32 indexed commitment, uint32 networkId, uint64 batchNumber, uint64 totalBytes, uint32 totalChunks, bytes32 archiveDigest)</code> |

| Field | Rule |
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

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.sol-mirror-archive-append</code> | <code>sol_function</code> | [services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol](../../../services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol) | <code>function append(bytes32 commitment, uint32 index, bytes calldata value) external onlyPublisher</code> |

| Field | Rule |
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

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.sol-mirror-archive-begin</code> | <code>sol_function</code> | [services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol](../../../services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol) | <code>function begin(bytes32 commitment, uint32 networkId, uint64 batchNumber, bytes32 checkpointId, uint64 totalBytes, uint32 totalChunks, bytes32 archiveDigest, bytes32 expectedChunkChain) external onlyPublisher</code> |

| Field | Rule |
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

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.sol-mirror-archive-chunk</code> | <code>sol_function</code> | [services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol](../../../services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol) | <code>function chunk(bytes32 commitment, uint32 index) external view returns (bytes)</code> |

## sol-mirror-archive-constructor

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.sol-mirror-archive-constructor</code> | <code>sol_function</code> | [services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol](../../../services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol) | <code>constructor(address publisher_)</code> |

## sol-mirror-archive-finalize

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.sol-mirror-archive-finalize</code> | <code>sol_function</code> | [services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol](../../../services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol) | <code>function finalize(bytes32 commitment) external onlyPublisher</code> |

## sol-mirror-archive-manifest

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.sol-mirror-archive-manifest</code> | <code>sol_function</code> | [services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol](../../../services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol) | <code>function manifest(bytes32 commitment) external view returns (uint64 totalBytes, uint32 totalChunks, bytes32 archiveDigest, bool finalized)</code> |

## sol-mirror-archive-max-archive-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.sol-mirror-archive-max-archive-bytes</code> | <code>sol_function</code> | [services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol](../../../services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol) | <code>uint256 public constant MAX_ARCHIVE_BYTES = 64 * 1024 * 1024</code> |

## sol-mirror-archive-max-chunk-bytes

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.sol-mirror-archive-max-chunk-bytes</code> | <code>sol_function</code> | [services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol](../../../services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol) | <code>uint256 public constant MAX_CHUNK_BYTES = 24 * 1024</code> |

## sol-mirror-archive-max-chunks

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.sol-mirror-archive-max-chunks</code> | <code>sol_function</code> | [services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol](../../../services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol) | <code>uint256 public constant MAX_CHUNKS = 65_536</code> |

## sol-mirror-archive-publisher

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.sol-mirror-archive-publisher</code> | <code>sol_function</code> | [services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol](../../../services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol) | <code>address public immutable publisher</code> |

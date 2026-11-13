# Arbitrum data availability

These modules decode batch data availability (DA) metadata for the Arbitrum
indexer. They describe the configured Arbitrum rollup and its parent chain.
Paxeer's native receipt finality comes from committed BFT blocks.

## Decoding and import

Batch [discovery](../workers/batches/discovery.ex) obtains calldata through
[Utils.Rpc](../utils/rpc.ex) and calls
`Indexer.Fetcher.Arbitrum.DA.Common.examine_batch_accompanying_data/2`.
[Common](common.ex) identifies the container from the accompanying data:

| Data | Container |
| --- | --- |
| No accompanying data (`nil`) | EIP-4844 blob |
| Header flag `0` | Transaction calldata |
| Header flag `99` | Celestia |
| Header flag `128` or `136` | AnyTrust |
| Header flag `237` | EigenDA |

Unsupported flags, including `32`, and malformed payloads produce error
results. Each DA-specific module implements `parse_batch_accompanying_data/2`.
`Common.required_import?/1` selects Celestia, AnyTrust and EigenDA metadata
for separate storage.

`Common.prepare_for_import/2` prepares DA records and batch-to-blob links.
It returns `{records, links}` or
`{:error, {:anytrust_keyset, hash, reason}}` when an AnyTrust keyset cannot
be retrieved or decoded. Celestia and EigenDA implement `prepare_for_import/2`;
AnyTrust implements `prepare_for_import/4`, including the parent-chain
connection and a keyset cache. Each module implements `resolve_conflict/2`
to compare candidates with stored records. Preparation deduplicates data
keys within the import chunk as well as against the database.

## Stored metadata

| Module | Record data and key |
| --- | --- |
| [Celestia](celestia.ex) | Height, transaction commitment and raw descriptor; SHA-256 of the unsigned encoded height followed by the commitment |
| [Anytrust](anytrust.ex) | Certificate data hash as the key; timeout, signers mask, keyset hash and BLS signature in the certificate |
| [Eigenda](eigenda.ex) | Hex-encoded blob header and verification proof; Keccak-256 of the blob header as the key |

Primary metadata has `data_type: 0`. AnyTrust keysets have `data_type: 1`
and use the keyset hash as their key. Unknown keysets are fetched from the
SequencerInbox contract's parent-chain data, including `SetValidKeyset` logs.
The cache avoids duplicate keyset retrieval within preparation. For duplicate
AnyTrust certificates, the higher timeout wins; Celestia and EigenDA retain
an existing database record with the same key.

The [DA record schema](../../../../../../explorer/lib/explorer/chain/arbitrum/da_multi_purpose_record.ex)
uses `arbitrum_da_multi_purpose`. Its `Helper` module supplies
`calculate_celestia_data_key/2` and `calculate_eigenda_data_key/1`.
The [batch-to-blob schema](../../../../../../explorer/lib/explorer/chain/arbitrum/batch_to_da_blob.ex)
uses `arbitrum_batches_to_da_blobs` to associate batches with DA keys.
The [batch schema](../../../../../../explorer/lib/explorer/chain/arbitrum/l1_batch.ex)
stores the container in `batch_container`.

## API lookup

The Arbitrum API registers these GET routes under `/api/v2/arbitrum/batches/da`:

| Suffix | Lookup |
| --- | --- |
| `/anytrust/:data_hash` | AnyTrust certificate data hash |
| `/eigenda/:data_hash` | EigenDA blob-header hash |
| `/celestia/:height/:transaction_commitment` | Celestia height and commitment |

AnyTrust and EigenDA return the latest associated batch by default;
`type=all` selects a paginated batch list. Celestia returns one associated
batch. These routes are defined by the Arbitrum chain scope in
[ApiRouter](../../../../../../block_scout_web/lib/block_scout_web/routers/api_router.ex).
The controller and view read stored DA metadata to render batch information.

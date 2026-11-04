# Ethereum and Solana mirrors

`layerx-mirror` archives committed Paxeer blocks, publishes retrievable archive
bytes to Ethereum or Solana, and verifies receipts retrieved from configured
mirror sources. Paxeer is one BFT network with EVM chain id `125`; its kernel
proposes blocks, and a receipt is final once its block is committed. External
publication provides another evidence source and has its own confirmation and
freshness policy.

The binaries are `layerx-mirror-publisher` and `layerx-mirror-verify`.
See the [crate exports](../../services/interop/crates/layerx-mirror/src/lib.rs),
[Interop](Interop.md), and [x402 receipt binding](X402Transport.md#receipt-binding).

---

## Public types

Public modules are `committed`, `ethereum`, `rpc`, `runtime`, `signer`, `solana`,
`source`, and `store`. Publisher and verifier types are re-exported at the crate
root.

| Purpose | Names |
| --- | --- |
| Committed source acquisition (`committed`) | `CommittedBlockSource`, `CommittedSourceConfig`, `CommittedBlock`, `CommittedHeader`, `CommittedCoordinate`, `ReceiptKeyConfig` |
| Archive and publication (crate root) | `Archive`, `ArchiveData`, `ArchivedCommittedBlock`, `ArchiveCommitment`, `Publisher`, `GenericPublisher`, `PublicationReport`, `MirrorCursor`, `MirrorFreshness` |
| Verification (crate root) | `MirrorVerifier`, `MirrorVerification`, `MirrorVerificationFreshness`, `MirrorEvidenceLevel`, `MirrorVerifyError` |
| Source selection (`source`) | `MirrorSource`, `MirrorSources`, `MirrorReadPolicy`, `MirrorLocator`, `ObservedArchive`, `MirrorObservation`, `MirrorLag` |
| External publication | `ethereum::EthereumArchiveClient`, `solana::SolanaArchiveClient`, `signer::RemoteChainSigner`, `store::PublicationJournal` |

The contracts are
[LayerXMirrorArchive.sol](../../services/interop/contracts/ethereum-mirror/LayerXMirrorArchive.sol)
and the [Solana mirror program](../../services/interop/contracts/solana-mirror/src/lib.rs).
Contract addresses, publisher identities, program identities and code hashes are
operator configuration, not universal constants.

## Committed archives

[CommittedBlockSource](../../services/interop/crates/layerx-mirror/src/committed.rs)
checks chain id and genesis against configuration, reads blocks through root
node JSON-RPC, and obtains `pax_sequencerReceipt` evidence for each transaction.
It requires the `committed` rung, matching block number/hash and transaction
identity, and receipt signatures under configured height-scoped keys. Sequential
acquisition checks the parent hash; malformed or unavailable evidence is refused.
The RPC method name denotes the node's signed-receipt API.

[Archive::from_committed](../../services/interop/crates/layerx-mirror/src/publisher.rs)
produces version-4 archives containing network id, block height, block and parent
hashes, app hash, timestamp and signed receipt bytes. The wire field
`batch_number` carries the committed height. Decoding enforces version, framing,
size bounds and nonempty receipt records. This format carries committed-block
metadata and receipts; it does not carry a signed batch-header inclusion proof.

## Operator binaries

```text
layerx-mirror-publisher <config.json>
layerx-mirror-verify <config.json> < request.json
```

The [publisher runtime](../../services/interop/crates/layerx-mirror/src/runtime.rs)
reads `state_directory`, `first_height`, `poll_interval_ms`,
`freshness_budget_blocks`, `status_listen`, `paxeer`, `ethereum`, and optional
`solana`. The `paxeer` section supplies `rpc`, `chain_id`, `genesis_hash_hex`
and `receipt_keys`. It spools committed archives in height order and retains
publication progress for restart. Ethereum/Solana clients and remote signing
clients are implemented; operation requires configured RPC, target identities,
signing authority, storage and external transaction funding.

The [verify binary](../../services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs)
reads `layerx_network_id`, `receipt_keys` and pinned `sources` from configuration.
Its stdin request contains a decimal-string `batch_number`, `evidence` and
`policy`. Evidence is `{ "kind": "receipt", "index": 0 }` or
`{ "kind": "committed" }`. Policies select an exact source, ordered preference,
or a minimum agreement among configured sources and archive commitments.
Public requests select source indices; they cannot inject RPC endpoints or keys.

`MirrorSources::read` retains one source's archive and observation together and
refuses conflicting commitments. `MirrorVerifier::from_committed_source` checks
the archive commitment and height. `committed_receipt(index, keys)` verifies the
selected receipt's signature under independently configured receipt authority.
Its result is `MirrorEvidenceLevel::Committed` with the source observation and
freshness. This path authenticates the configured mirror's archive and signed
receipt; it does not independently verify a BFT commit certificate or payment
terms. A payment consumer still binds the receipt to its expected operation.

An in-memory archive can be verified without contacting Paxeer or the hosted
gateway; the binary itself retrieves it through external-chain RPC. Source
observations retain target identity, canonical position, commitment, failover
count and agreeing-source count. `MirrorSources::recheck` updates provenance
after a source-chain reorganization. Treat unknown lag as unknown: a valid old
receipt does not demonstrate that the mirror is current.

## Qualification boundary

The source clients, publisher runtime, durable journals, remote signer client
and committed receipt verifier are implemented. Tests and fixtures under the
[mirror crate](../../services/interop/crates/layerx-mirror/tests) exercise local
archive, acquisition, publication and verification paths. They do not attest a
funded live publisher, current external-chain deployment or support in every SDK.
Check the selected SDK's actual mirror API and configure its trust explicitly.

[Interop](Interop.md) · [Portable receipt verifier](PortableVerifier.md) · [Home](Home.md)

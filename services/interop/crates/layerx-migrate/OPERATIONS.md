# Migration source verifier operations

`EthereumVerifier` and `SolanaVerifier` are the source-verification boundaries used by `MigrationAdapter` and the operator binary. Construct them from `EthereumConfig` or `SolanaConfig`; both configuration types deserialize from JSON. The verifier must remain alive for verification and the subsequent history-cursor commit.

## Network and source configuration

Paxeer mainnet-beta is one BFT network with EVM chain ID 125 (`0x7d`); the
kernel participates in the network and proposes blocks. Block commit establishes
network finality. Source-chain verification establishes external provenance;
it does not by itself execute a payment or create a network balance.

Public node JSON-RPC uses root HTTPS URLs such as
`https://api1.mainnet-beta.paxeer.network/`, with hosts `api1` through `api14` and
`api-mainnet-beta.paxeer.network`. These public endpoints are not a migration
rollback-authority deployment. The protected quorum client additionally requires
operator-supplied TLS and bearer credentials.

Each source and rollback-anchor quorum requires two to eight independent HTTPS JSON-RPC endpoints and a strict majority. Endpoint URLs must use DNS names and HTTPS. Every endpoint declaration requires an `independent_backend` label for the operator-audited backend identity. Both `(host, port)` and normalized backend identities must be unique, so aliases, alternate request paths, and duplicate labels cannot manufacture quorum membership. The DER trust anchor and bearer credential paths must be absolute. Bearer credentials must be regular files with no group or other permissions; invalid paths or permissions refuse construction before any network request.

Protected Ethereum, Solana, and rollback-anchor endpoint declarations use this closed shape; omitting `independent_backend` is a configuration error:

```json
{
  "url": "https://source-a.example/",
  "ca_certificate_der": "${LAYERX_MIGRATION_SECRET_DIR}/source-ca.der",
  "bearer_token_file": "${LAYERX_MIGRATION_SECRET_DIR}/source-a.token",
  "independent_backend": "operator-a"
}
```

Ethereum configuration pins the chain ID, genesis block, custody contract, immutable runtime or explicit proxy implementation, runtime hashes at the source block, ABI selector, event topic, and the location of every custody field. Solana configuration pins the genesis hash, custody program, immutable ProgramData account and code hash, loader, custody and token authorities, account owners, instruction discriminator, account indices, byte offsets, and integer encoding. No default custody schema or deployment identifier exists.

For Solana, `custody_program_data_hash` is SHA-256 of **every byte after the
fixed 45-byte ProgramData metadata region**, including allocated trailing
padding, as implemented by `immutable_program_code_hash` in
[src/solana.rs](src/solana.rs). The loader variant must be ProgramData, the
upgrade authority must be absent, and deployment must not postdate the source
transaction. Do not hash only the ELF bytes or shorten the metadata offset
when the authority is absent.

## Journal and rollback authority

The journal directory is canonical absolute local durable storage protected by an HMAC key file. The key path must be canonical and absolute with no symlinked component, resolve directly to a regular file, and have no group or other permissions. The namespace directory must likewise be canonical with no symlinked component, have the same owner as the key, and have no group or other permissions; newly created directories and record/seal files use modes `0700` and `0600`. Every existing record and seal must be a direct regular file owned by that identity with no group or other permissions before replay reads it. Unsafe, case-normalized, relative, symlinked, foreign-owned, or accessible storage is refused.

The local head is reconciled on journal load and append through
`JournalConfig.rollback_anchor_rpc` and `rollback_anchor_id`. The client calls
these methods on a strict-majority HTTPS authority:

- `layerx_getMigrationJournalHead([anchor_id])`, returning `{sequence,digest}`.
- `layerx_advanceMigrationJournalHead([{anchor_id,expected_sequence,expected_digest,sequence,digest}])`, performing an authenticated compare-and-swap and returning the committed `{sequence,digest}`.

These are the client's required external interface, not methods supplied by the
public node. The current tree contains the client and test peers, but no
production server for these methods; an operator must identify and qualify the
actual producer before relying on rollback protection. The methods require
linearizable durable compare-and-swap, authorization and retention independent
of the verifier host. A local journal alone cannot provide that authority.

`sequence` is a JSON unsigned integer and `digest` is a 32-byte lowercase hex
string without a prefix. The initial head is sequence zero with a zero digest.
A remote head ahead of the local journal or a mismatched digest is refused; a
remote head behind a valid local chain is advanced one entry at a time using
compare-and-swap. Unavailable or divergent responses fail closed. Keep journal
keys, RPC tokens and authority credentials readable only by the service identity.

## Mapping and custody receipt checks

For `MigrationAdapter::map_account`, a wallet-signed bounded ownership claim is
checked alongside a deployment-specific `BindingReceiptPolicy`. For
`MigrationAdapter::migrate_asset`, exact source custody evidence is checked
alongside a `CustodyReceiptPolicy`. The policies pin the trusted signing public
key, authority and module/operation coordinates. They require a successful
receipt with a nonzero authorization hash and independently match recipient,
asset, amount, and exact debit/credit deltas using checked arithmetic.

The v2 receipt's signed `contextHash` (`context_hash` in Rust) is the producer's
`keyId`. Pin a `SequencerAuthorization` with that key identifier, its trusted
public key and its inclusive block-height range; both policies also require the
authorized public key to equal their independently configured signing key.
Never substitute an external-claim digest for `keyId`. The policies'
`context_hash(...)` helpers compute claim hashes, but their receipt-policy checks
do not compare those hashes to the signed context field.

Claim matching is independent of signing context: source evidence and canonical
source-derived idempotency keys identify the claim, while receipt-policy checks
match the configured operation, authority, recipient, asset, amount and exact
balance deltas. The gateway verifies the signed log commitments and requires
exactly one payment matching the idempotency key. Native `PaymentExact` topics
are `[event signature, paymentReference, from, to]`; the ABI data words are
`[assetId, amount]`. Match the reference in topic 1, not either account topic.
The `MigrationPlane` receives adapter-created typed requests; receipt verification
must still use the independently configured signing authority.

The binary's `confirm-mapping` follows the separate V2 path:
`AccountMappingStoreV2` verifies ownership, matches the supplied identity and
uses `PaxeerBindingVerifierV2` to read both directions of the binding at a
finalized Paxeer block. It pins chain ID 125 and the genesis hash and calls the
address precompile at `0x0000000000000000000000000000000000001004` using
the fixed selectors `0xc413bad3` (forward lookup) and `0xd3600b24` (reverse lookup),
as encoded in [mapping_v2.rs](src/mapping_v2.rs). The stored result is
`account-mapping-v2`, provenance `paxeer-binding`, with `layerx_receipt: false`;
it confirms an existing binding rather than executing a binding transaction.

The binary's `migrate-asset` sends `SourceSettlementRequestV2` through the
authenticated `GatewayClient`. Its version is `layerx-migration-source-v2` and
its response states are `source_pending`, `source_settled`, `layerx_pending`,
`layerx_refused` or `done`. A response is checked against the order digest and
exact source evidence. This observation is not a portable proof of block commit;
even a producer state of `done` is reported as `producer-observed` with
`provenance: external-custody`, `layerx_receipt: false` and
`layerx_credit_verified: false`. External custody does not establish network
credit; see [receipt verification](../../../../docs/interop/PORTABILITY.md).

## External history

History pages are external provenance. They are prepared in the authenticated journal, stored through `ExternalHistorySink`, then committed through the same verifier. A sink must durably deduplicate by chain, native transaction identifier, address, asset, and kind before returning success. It must never translate an imported record into a LayerX activity or receipt.

`history::DurableExternalHistory` implements that sink using the existing authenticated journal and rollback-anchor protocol. Construct it with a dedicated `JournalConfig`; each stored page is one atomic `external-history` update containing canonical records with the `LXP/ExternalHistory/v1` domain. Admission validates the complete page before applying it, treats identical records as replay, and refuses conflicting facts for the same source identity. Principal-scoped reads return only `ExternalHistoryRecord` values, including their closed Ethereum/Solana provenance label, and accept page limits of 1–256. The continuation cursor orders record identities within the current durable state; it is not a frozen snapshot or a source-chain evidence token. Reopen and every read reconcile the existing HMAC/hash-chain journal with its independent rollback anchor. Retain the history journal, its authentication key and anchor identity together; deleting local data is not a reset of the durable anchor.

## Live qualification inputs

The live history cases additionally require `LAYERX_ETHEREUM_HISTORY_STORE_CONFIG` and `LAYERX_SOLANA_HISTORY_STORE_CONFIG`, each naming a protected `JournalConfig` JSON file. Each must name a fresh, dedicated qualification namespace and rollback-anchor id, distinct from its source verifier's journal. Supply genuine nonempty source history evidence. The fixture stores and replays those verified pages, reopens the real sink, checks principal isolation and bounded pagination, and corrupts private copies of the resulting journal to exercise integrity, permissions and rollback refusal. It does not modify the original journal for those refusal cases or synthesize history records. The workflow receives these configurations from `ETHEREUM_HISTORY_STORE_CONFIG_B64` and `SOLANA_HISTORY_STORE_CONFIG_B64`; they use the same protected-path substitution as the existing verifier configurations. Source verification and external history storage do not establish that account binding or custody credit executed on LayerX.

The protected `migration-testnets` GitHub environment supplies exact deployed configuration, evidence envelopes, credentials, journal keys, and trust anchors to `.github/workflows/interop-migration-testnets.yml`. The workflow is manually dispatched because it reads live test networks and durable operator state. It exercises the production verifiers through `make interop-test-migration-testnets`.

See the [crate README](README.md) for connected fixture requirements and the
[workspace index](../../README.md) for the interop boundary. None of the local
source or history checks establishes that a custody credit executed on Paxeer;
that claim requires independently matched network evidence.

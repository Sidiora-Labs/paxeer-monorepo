# layerx-mirror

Publishes retrievable archives of committed Paxeer blocks to Ethereum and,
optionally, Solana. An archive contains block coordinates and signed network
receipts; it holds no custody and creates no separate settlement domain.
Paxeer mainnet-beta is one BFT network (chain ID 125), with the kernel part of
the network and participating in block proposal. Its block commit is finality;
external publication records archive availability under each destination's
confirmation policy.

See the [workspace index](../../README.md) and
[portable receipt guide](../../PORTABILITY.md).

| Binary | Invocation from the monorepo root |
| --- | --- |
| `layerx-mirror-publisher` | `cargo run --locked --manifest-path services/interop/Cargo.toml -p layerx-mirror --bin layerx-mirror-publisher -- <config.json>` |
| `layerx-mirror-verify` | `cargo run --locked --manifest-path services/interop/Cargo.toml -p layerx-mirror --bin layerx-mirror-verify -- <config.json> < <request.json>` |

## Publisher configuration and acquisition

The publisher's closed JSON schema is
[`RuntimeConfig`](src/runtime.rs): `state_directory`, `first_height`,
`poll_interval_ms`, `freshness_budget_blocks`, `status_listen`, `paxeer`,
`ethereum`, and optional `solana`. The `paxeer` object contains `rpc`, `chain_id`,
`genesis_hash_hex` and `receipt_keys`. Each key entry has `sequencer_id_hex`,
`public_key_hex`, `first_height` and `last_height`; obtain these pins independently.
The source network ID written to archives is derived from `paxeer.chain_id`.

[`CommittedBlockSource`](src/committed.rs) checks chain ID and genesis, reads
`eth_getBlockByNumber` with `finalized`, and acquires consecutive blocks with
parent-hash checks. For every transaction it requires a `pax_sequencerReceipt`
answer at rung `committed`, matching the block number/hash and transaction hash,
and verifies the receipt signature under pinned height-ranged keys.

Use root HTTPS node URLs for Paxeer, such as
`https://api1.mainnet-beta.paxeer.network/`; the public hosts are `api1` through
`api14.mainnet-beta.paxeer.network` and `api-mainnet-beta.paxeer.network`.
The quorum transport requires two to eight independent HTTPS endpoints, a strict
majority, explicit time/response bounds, and file-backed TLS and bearer material.
Distinct DNS aliases do not establish independent backends: provide unique,
operator-audited `independent_backend` identities as well as unique host/port pairs.

The [publisher configuration template](../../deploy/mirror/config.example.json)
is a field-layout reference, not a deployable configuration. Replace its source
URLs with root node URLs, fixture keys and deployment placeholders with trusted
pins, and credentials and signer handles with protected operator values.
Use the current schema in `src/runtime.rs` when preparing configuration.
Publication uses a local archive spool and independent destination workers;
preserve `state_directory` for recovery. Ethereum is required, while omission
of the Solana section selects Ethereum-only operation.

## Verifier configuration and input

The verifier reads its configuration from the argument path and one JSON request
from standard input. Its current closed schema is declared in
[`layerx-mirror-verify.rs`](src/bin/layerx-mirror-verify.rs), independently of the
publisher schema. Configure `layerx_network_id` (125 for Paxeer), `receipt_keys`
in the four-field shape above, and `sources`.

Each source declares `kind`, `id`, `rpc`, and `publisher_hex`, plus these pins:

| `kind` | Additional fields |
| --- | --- |
| `ethereum` | `chain_id`, `genesis_hash_hex`, `archive_contract_hex`, `archive_code_hash_hex` |
| `solana` | `genesis_hash_hex`, `archive_program_hex`, `upgradeable_loader_hex`, `program_data_account_hex`, `program_code_hash_hex` |

Provide real independently verified deployment values. The verifier uses hex
pins, including Solana identity fields; publisher Solana configuration uses
base58 for several corresponding fields. Do not copy publisher configuration
into the verifier or use an old verifier example as a complete operational contract.

A request contains exactly `batch_number`, `evidence` and `policy`.
`batch_number` is a canonical decimal string naming the archived block height;
the retained field name does not imply a separate execution batch.
`evidence` is `{"kind":"committed"}` for archive coordinates or
`{"kind":"receipt","index":0}` for one receipt, with a zero-based index.
`policy` selects `exact` with a `candidate`, `ordered-preference` with
`candidates`, or `agreement` with `candidates` and `minimum`. Every candidate
has a zero-based `source` index into configured sources and a 32-byte
`commitment_hex` obtained from the publication record.

The tool prints JSON with `ok: true` and `verification`, or `ok: false` and
`error`. Read this field: refusal is rendered as JSON, not necessarily a nonzero
process exit. Errors include `configuration`, `malformed`, `bounds`, `missing`,
`unavailable`, `rate-limited`, `divergent`, `insufficient-agreement` and `proof`.
The receipt mode checks the archived receipt against the independently pinned
keys. Archive coordinates and a destination-chain observation do not replace
an independently trusted Paxeer consensus commit proof. For offline signature
proofs and their limits, see [FORMAT.md](../layerx-portable/FORMAT.md).

## Contracts, signer and development targets

On-chain archives: [Ethereum](../../contracts/ethereum-mirror/) and
[Solana](../../contracts/solana-mirror/). Remote signer framing remains defined
in [signer-protocol.md](../../deploy/mirror/signer-protocol.md).

From the monorepo root, development targets are `make interop-build`,
`make interop-test` and `make interop-test-mirrors` (the crate plus both archive
contracts). `make mirror-live` and `make mirror-verify-live` require genuine
operator configuration and live infrastructure. Listing these targets does not
claim a live qualification result.

# layerx-mirror

Publishes retrievable archives of committed Paxeer blocks to Ethereum and,
optionally, Solana. An archive contains block coordinates and signed network
receipts in the pure v4 archive format; it holds no custody and creates no
separate settlement domain.
Paxeer mainnet-beta is one BFT network (chain ID 125), with the kernel part of
the network and participating in block proposal. Its block commit is finality;
external publication records archive availability under each destination's
confirmation policy.

See the [workspace index](../../README.md) and
[portable receipt guide](../../../../docs/interop/PORTABILITY.md).

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

The [verifier configuration example](examples/verifier.json) is accepted by this
closed schema. It includes Ethereum and optional Solana source layouts, fixture
receipt keys and placeholder destination pins. Copy it to an operator-owned path,
replace all endpoint, credential-file and deployment values, and independently
verify the receipt keys before use. Source `chain_id` identifies the destination
Ethereum chain; `layerx_network_id` identifies Paxeer chain 125. Remove the Solana
source if unused. Never obtain trusted keys from the request or archive itself.

The key windows are inclusive at both ends and authorize the height encoded in
the signed receipt, which can precede the archived block height. Configure a
nonempty key list, with `first_height <= last_height`, and retain independently
verified historical keys during rotation. A matching key ID outside its window
fails receipt verification; knowing only a publisher address is insufficient.

Save either complete request below as `request.json` and use the invocation above.
Both examples select block 1000 and source 0. Replace `commitment_hex` with the
actual 32-byte archive commitment from that source's publication record; the
repeated `01` bytes are a placeholder, not a discovered archive commitment.

Receipt request (index 0 is the first archived receipt):

```json
{
  "batch_number": "1000",
  "evidence": {"kind": "receipt", "index": 0},
  "policy": {
    "kind": "exact",
    "candidate": {
      "source": 0,
      "commitment_hex": "0101010101010101010101010101010101010101010101010101010101010101"
    }
  }
}
```

Committed-coordinate request:

```json
{
  "batch_number": "1000",
  "evidence": {"kind": "committed"},
  "policy": {
    "kind": "exact",
    "candidate": {
      "source": 0,
      "commitment_hex": "0101010101010101010101010101010101010101010101010101010101010101"
    }
  }
}
```

Committed mode reports the archived height/hash and destination observation; it
does not authenticate a receipt signature. Receipt mode additionally authenticates
the selected receipt under the configured key windows and emits `evidenceDigest`.
Neither selector proves arbitrary state, transaction inclusion under a consensus
root, nor a Paxeer consensus certificate. Both report the level `Committed` as an
archive coordinate. `latestBatch` can be null and `batchLag` can be `Unknown`:
a read of one archive does not prove the source's latest published head. Assess
freshness separately against an independently observed committed Paxeer head,
in blocks and per source.

The receipt mode checks the archived receipt against the independently pinned
keys. Archive coordinates and a destination-chain observation do not replace
an independently trusted Paxeer consensus commit proof. For offline signature
proofs and their limits, see [FORMAT.md](../layerx-portable/FORMAT.md).

## Durable state and upgrade ownership

The publisher operator owns `state_directory`, backups and cutover reconciliation.
The current store uses v3 journals (`LXMJNL03`), sealed heads (`LXMHED03`) and spool
objects (`LXMSPL03`); these are separate from the v4 on-chain archive format.
There is no automatic v2-to-v3 migration. Do not delete, relabel or rewrite old
state to bypass a version refusal, and never share a state directory between
publisher versions.

Stop writers and preserve a consistent durable copy of all lane journals, sealed
heads, spool objects, configuration and the matching old executable. Use that
version to verify retained history and reconcile every prepared, signed, pending,
unknown or reorged transaction with canonical destination-chain history; absence
alone does not prove failure. Unresolved transactions block cutover. Retrieve and
compare completed archives with their retained bytes/commitments, verify finality
positions and establish the contiguous committed height/hash cursor. Preserve the
old state and reconciliation inventory for historical reads.

After reconciliation, initialize a distinct empty directory with `first_height`
set to the operator-verified next committed block. Reopen and verify the new state,
retrieve a new publication and authenticate its receipts before cutover. A rollback
also requires reconciliation of any newly signed or broadcast transactions.
Domain 11 (interop) owns any future migration tooling and durable-format contract
updates; this crate does not supply a migration command. The
[retained-state repair record](src/repair_22_33_101.kvx) describes the existing
recovery evidence; its historical configuration spelling must not replace the
current `RuntimeConfig.first_height` field.

## Contracts, signer and development targets

On-chain archives: [Ethereum](../../contracts/ethereum-mirror/) and
[Solana](../../contracts/solana-mirror/). Remote signer framing remains defined
in [signer-protocol.md](../../deploy/mirror/signer-protocol.md).

From the monorepo root, development targets are `make interop-build`,
`make interop-test` and `make interop-test-mirrors` (the crate plus both archive
contracts). `make mirror-live` and `make mirror-verify-live` require genuine
operator configuration and live infrastructure. Listing these targets does not
claim a live qualification result.

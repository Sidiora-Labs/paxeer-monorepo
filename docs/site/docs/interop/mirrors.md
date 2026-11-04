# Batch mirrors

Mirrors publish committed Paxeer X Network block archives to Ethereum and
Solana. The archives carry commitments and retrievable data. They create no
vault, custody or withdrawal authority on the destination chain. Network
balances and finality belong to the same Paxeer network, EVM chain ID `125`.

The current archive format is v4. It retains the committed block header and
canonical signed receipts. Historical names such as `batch_number` remain
in the API, but the current source coordinate is a committed block height
and hash. Separate checkpoint finality is retired.

## Publisher and verifier

Both binaries belong to `services/interop/crates/layerx-mirror` in the
`services/interop` Cargo workspace. From the monorepo root:

```sh
cargo run --manifest-path services/interop/Cargo.toml -p layerx-mirror --bin layerx-mirror-publisher -- /path/to/publisher-config.json
cargo run --manifest-path services/interop/Cargo.toml -p layerx-mirror --bin layerx-mirror-verify -- /path/to/verifier-config.json < /path/to/request.json
```

The publisher's `CommittedBlockSource` pins chain ID, genesis hash and
receipt key windows. It reads committed headers and `pax_sequencerReceipt`
answers, requires the committed height and hash to agree, and checks receipt
signatures and transaction identities before archiving. Sequential
acquisition also checks parent continuity.

The verifier reads configured Ethereum or Solana archive sources. Its
configuration pins `layerx_network_id`, `receipt_keys` and `sources`,
including destination network, code and publisher identities. It does not
need a live Paxeer endpoint to read a published archive, but the CLI does
need the configured destination-chain sources.

On-chain archive programs live in
`services/interop/contracts/ethereum-mirror/` and
`services/interop/contracts/solana-mirror/`. Remote signer framing is
documented in `services/interop/deploy/mirror/signer-protocol.md`.

## What the CLI verifies

The JSON request has `batch_number` (a positive decimal string), `evidence`
and `policy`. The supported evidence requests are:

| `evidence` value | Check |
| --- | --- |
| `{"kind":"committed"}` | Admits the archive against the observed source commitment and height, then reports its committed coordinate. |
| `{"kind":"receipt","index":0}` | Additionally verifies the indexed canonical receipt against independently configured receipt keys. |

Read policies are `exact`, `ordered-preference` and `agreement`. Candidates
name a source index and `commitment_hex`; agreement also requires a minimum
source count. Source selection and canonical destination-chain observations
are part of the evidence boundary. The CLI does not expose a general
account-state or arbitrary balance-proof operation, and a reported archive
coordinate is not an independent replay of Paxeer consensus.

Successful JSON has `ok: true` and a `verification` object with
`level: "Committed"`, `committedHeight`, `committedBlockHash`, source and
target identity, canonical destination position and provenance. Receipt
mode adds `evidenceDigest`; committed-coordinate mode returns it as `null`.
Failures have `ok: false` and an error such as `missing`, `divergent`,
`insufficient-agreement`, `unavailable` or `proof`. Consumers must inspect
that JSON: `main` prints the refusal without setting a failing process exit.

## Freshness

Keep the archived committed height and hash visible alongside source
provenance. The CLI also reports `latestBatch`, `batchLag`, `failoverCount`
and `agreeingSources`. An unknown lag is not zero lag. Destination-chain
position describes where the archive was observed; it does not replace the
archived Paxeer block coordinate.

The library's `MirrorVerificationFreshness` can carry
`latest_committed_mirrored` and committed lag information when supplied.
An offline archive alone cannot establish the current Paxeer head. State
the known coordinate and any unknown freshness rather than calling old data
current.

## Status

The publisher, verifier, destination-chain contracts and tests are in this
tree. Their source is not evidence that a particular public mirror is live.
For an individual receipt carried without an archive, use
[Portable receipts](portable-receipts.md); its proof level remains receipted.
See [Receipt concepts](../concepts/receipts.md) for network finality and
[Interoperability](index.md) for the other adapter surfaces.

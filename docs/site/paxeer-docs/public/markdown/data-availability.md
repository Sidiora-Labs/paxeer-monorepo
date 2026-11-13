<!-- Source: https://docs.paxeer.app/data-availability/ -->

# Data Availability

Retrieve authenticated batch data, verify it against the committed availability root, and preserve independent evidence through archives and mirrors.

## Availability makes replay possible

Paxeer X commits batch data in five availability classes. A verifier fetches these bytes in chunks, authenticates every chunk against the committed data-availability root, and can then independently replay the transition. An availability proof establishes committed data; replay establishes the computed result.

| Class | Contents |
| --- | --- |
| Activities (1) | Canonical accepted activities |
| Receipts (2) | Canonical activity receipts |
| Oracle (3) | Canonical signed oracle inputs |
| StateDiff (4) | Canonical state-diff material |
| Recovery (5) | Canonical recovery metadata |

Chunks carry at most 65,536 bytes. Each chunk states its batch number, flattened bundle index, class, byte offset inside the class section, exact bytes and claimed digest. A chunk is accepted only when its metadata digest recomputes correctly and its Merkle inclusion path verifies under the data-availability root. Verified chunks must appear in strict flattened-index order.

**Source:** `services/agent/crates/layerx-proof/src/availability.rs` — `AvailabilityClass`, `Chunk`, `VerifiedChunk`.

## Fetch and verify workflow

1. Obtain the committed data-availability root from evidence you already trust.
2. Fetch chunks for the batch from a provider.
3. For each chunk, recompute its digest and verify its inclusion path against the root. Refuse chunks from another batch, out-of-order chunks and digest mismatches.
4. Require complete coverage of the classes your decision depends on.
5. Replay activities and oracle inputs; compare state roots and the recomputed account diff.
6. Attest only after replay reaches the exact committed result.

## Agent availability API

The agent runtime exposes an `availability_fetch` operation. A request carries a selector string, a requested rung, and finite retrieval bounds: `maximum_bytes`, `maximum_chunks` and a `deadline`. An empty selector or a zero bound is refused.

The report states whether retrieval completed from one provider or is partial. It carries a per-class report (completeness, verified chunk and byte counts, and any failure) and a per-provider report. Treat a partial report as missing data, not as an empty batch.

**Source:** `services/agent/crates/layerx-agent-api/src/read.rs` — `AvailabilityRequest`, `AvailabilityReport`; `services/agent/crates/layerx-agentd/src/human_runtime.rs` — `availability_fetch`.

## Retired kernel bundle API

The earlier kernel bundle symbols (`lxp_batch_availability_root` and the `lxp_da_*` family) are retired. The chain test suite asserts these symbols are absent from the kernel archive. Do not build integrations against them; use the chunk verification and agent API surfaces above.

**Source:** `modules/lxphost/retired_da_bundle_test.go`.

## Refusals and recovery

Keep storage and proof failures as typed refusals. Chunk verification distinguishes batch mismatch, index mismatch, digest mismatch, inclusion failure and ordering failure. Retry a retained selection or restore authenticated archive data; do not treat missing chunks as an empty batch or replace a verified root with an untrusted newer one.

## Ethereum and Solana mirrors

Mirror archives extend independent access to batch history. `layerx-mirror-publisher` publishes archives to Ethereum or Solana, while `layerx-mirror-verify` checks a receipt obtained from an untrusted mirror archive without a hosted gateway.

Verification validates the archive, binds its batch header to operator-configured trust, proves receipt inclusion and returns freshness. Archive provenance and freshness are separate from the receipt's signed outcome. Preserve the archive commitment, trusted header context, inclusion proof and checkpoint references for later audit.

See [Receipts & Proofs](https://docs.paxeer.app/proofs) for portable offline verification and native state witnesses, or [Finality & Commitment](https://docs.paxeer.app/finality) for how data availability supports settlement.

<!-- Source: https://docs.paxeer.app/data-availability/ -->

# Data Availability

Read block availability through the agent runtime and verify supplied batch archives with the proof library.

## Availability makes replay possible

The proof library defines five batch availability classes. Given an authenticated availability root and provider chunks, a verifier authenticates the data before replay. This format describes supplied batch evidence; the agent runtime reads merged-node blocks through a separate availability operation.

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
4. Require complete bundle coverage and contiguous class offsets before complete-bundle verification.
5. Replay activities and oracle inputs; compare state roots and the recomputed account diff.
6. Attest only after replay reaches the exact committed result.

## Agent availability API

The agent runtime exposes an `availability_fetch` operation. Its selector is a decimal block height. The request also carries a requested rung, `maximum_bytes`, `maximum_chunks` and a `deadline` in Unix seconds. Empty selectors, zero bounds and expired deadlines are refused. The implementation bounds transaction bytes and transaction count for the selected block.

For an available block, the runtime returns a complete `transactions` class with transaction count and byte count, plus the height, transaction root and receipt root. A pruned block produces a partial report with a provider failure of `pruned`. Missing data does not represent an empty block.

**Source:** `services/agent/crates/layerx-agent-api/src/read.rs` — `AvailabilityRequest`, `AvailabilityReport`; `services/agent/crates/layerx-agentd/src/human_runtime.rs` — `availability_fetch`.

## Block availability reads

The merged-node reader checks the requested height against the node head and fetches that block. It returns the block's transaction root, receipt root and transaction count, or a pruned result when the block is absent. The runtime returns this block report separately from the proof library's five-class batch format.

**Source:** `services/agent/crates/layerx-agentd/src/read/availability.rs`.

## Refusals and recovery

Keep storage and proof failures as typed refusals. Chunk verification distinguishes batch mismatch, index mismatch, digest mismatch, inclusion failure, ordering failure and incomplete bundle coverage. Retry a retained selection or restore authenticated archive data; do not treat missing chunks as an empty batch or replace a verified root with an untrusted newer one.

## Ethereum and Solana mirrors

Mirror archives extend independent access to batch history. `layerx-mirror-publisher` publishes archives to Ethereum or Solana, while `layerx-mirror-verify` checks a receipt obtained from an untrusted mirror archive without a hosted gateway.

Verification validates the archive, binds its batch header to operator-configured trust, proves receipt inclusion and returns freshness. Archive provenance and freshness are separate from the receipt's signed outcome. Preserve the archive commitment, trusted header context, inclusion proof and checkpoint references for later audit.

See [Receipts & Proofs](https://docs.paxeer.app/proofs) for portable offline verification and native state witnesses, or [Finality & Commitment](https://docs.paxeer.app/finality) for how data availability supports settlement.

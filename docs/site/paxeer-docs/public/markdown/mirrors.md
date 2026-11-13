<!-- Source: https://docs.paxeer.app/mirrors/ -->

# Ethereum & Solana Mirrors

Publish Paxeer X batch archives to external chains and verify committed receipt evidence independently of a hosted gateway.

## External access to canonical evidence

The mirror system builds committed batch archives, publishes them to Ethereum or Solana, and retrieves them through independently configured chain sources. A consumer can then verify an archived receipt under its own sequencer key configuration without contacting the Paxeer X gateway.

Mirror publication distributes evidence. Paxeer receipts are final once their blocks are committed by the BFT network. External-chain publication confirms archive availability under that chain's rules; it does not authorize a Paxeer transfer.

## Publisher and verifier roles

| Component | Responsibility |
| --- | --- |
| `layerx-mirror-publisher` | Acquire committed node data, construct archives, publish to configured targets and track durable progress |
| `layerx-mirror-verify` | Read configured mirror sources and verify a committed archive or one archived receipt |
| `MirrorVerifier` | Admit a committed archive, return its committed coordinate, and verify a receipt by index under configured sequencer keys |
| `PublicationJournal` and archive spool | Preserve publication phases and exact archive material through restart |

## Trust configuration

The verifier configuration carries `layerx_network_id`, a `receipt_keys` list, and a `sources` list. Each receipt key pins `sequencer_id_hex`, `public_key_hex`, and the `first_height`/`last_height` window in which that key authorizes receipts. This trust material is configured independently of archive payloads; an archive cannot authorize its own signing key or extend its height window. A configuration with no receipt keys is refused.

| Source | Identity pins |
| --- | --- |
| Ethereum | RPC quorum, chain ID, genesis hash, archive contract, archive code hash and publisher identity |
| Solana | RPC quorum, genesis hash, archive program, upgradeable loader, program-data account, program code hash and publisher identity |

Retain target-chain identity and canonical position with each observation. On Ethereum this includes the observed block and reference head; on Solana it includes the rooted slot and block hash.

## Publish an archive

```
layerx-mirror-publisher <publisher-config.json>
layerx-mirror-publisher --probe <configured-status-listener>
```

The second form probes the publisher's status listener for readiness instead of starting the service. Any other argument shape exits with a usage error.

1. Read committed blocks from the configured Paxeer source, starting at the configured first height.
2. Build and validate the archive, preserving its exact canonical bytes and content commitment.
3. Persist publication intent, obtain target-chain signatures and broadcast.
4. Observe configured chain confirmation or rooting, retrieve the published archive and verify the bytes.
5. Advance the durable publication cursor only through the recorded publication workflow.

Publisher configuration includes the state directory, first height, polling interval, committed-block freshness budget, status listener address, and the Paxeer committed-source section. The Ethereum target is configured explicitly; the Solana section is optional and validated when present.

### Recover publication safely

The publication journal distinguishes prepared, pre-broadcast failure, signed, broadcast-unknown, pending, finalized, and retrieved-verified states, alongside permanent refusal, reorganization, and expired broadcast. When broadcast status is unknown, resolve chain history before issuing a replacement nonce or blockhash.

Readiness includes committed-block freshness against the configured block budget. A healthy process and a completed historical publication do not by themselves establish that every target is current.

## Read policies

| Policy | Behavior | Use |
| --- | --- | --- |
| `exact` | Read one source locator and commitment | Audit a specific publication |
| `ordered-preference` | Use an explicit ordered candidate set | Controlled source failover |
| `agreement` | Require a configured minimum agreement among candidates | Cross-source corroboration |

A candidate identifies the configured source index and exact archive commitment. The command reports source or RPC divergence as divergent, alongside insufficient-agreement, missing, rate-limited and unavailable outcomes.

## Verify from a mirror

Prepare a verifier configuration with your independent network and source pins, then save the JSON request below as receipt-request.json and supply it on standard input. A receipt request names the archived receipt by its index within the batch. Replace the commitment placeholder with the 32-byte archive commitment:

```
{
  "batch_number": "12",
  "evidence": {
    "kind": "receipt",
    "index": 0
  },
  "policy": {
    "kind": "exact",
    "candidate": {
      "source": 0,
      "commitment_hex": "REPLACE_WITH_32_BYTE_ARCHIVE_COMMITMENT_HEX"
    }
  }
}
```

```
layerx-mirror-verify <verifier-config.json> < receipt-request.json
```

The verifier fetches the archive from the pinned source, checks its commitment and batch, and verifies the sequencer signature over the archived receipt bytes against the receipt key whose height window covers the batch. It does not take receipt bytes from the caller: a request carrying fields such as `canonical_hex` is refused, because unknown request members are denied. There is no state-proof request in this command; the only other evidence kind is `committed`, which verifies the archive's committed coordinate without selecting a receipt.

Batch numbers are nonzero canonical decimal strings without leading zeros. Receipt indices at or above 1,048,576 are out of bounds, and the request body is capped at 40 MiB.

## Interpret the result

On success the verifier prints `{ "ok": true, "verification": { ... } }`. The verification object contains `level`, `evidenceDigest`, `sourceId`, `target`, `canonicalPosition`, `provenance`, `latestBatch`, `batchLag`, `failoverCount`, `agreeingSources`, `committedHeight`, and `committedBlockHash`. Failures print `ok: false` with an error code.

The only evidence level is `Committed`: the archive was admitted under the pinned source commitment and, for a receipt request, the receipt's sequencer signature verified. The report identifies the admitted committed height and block hash. Freshness is reported separately through `latestBatch` and `batchLag` and can be unknown. Store the observation and trust configuration with the evidence so later readers can distinguish historical validity from current coverage.

## Library verifier

In Rust, `MirrorVerifier::from_committed_source` or `admit_committed` admits archive bytes against an observation, refusing a mismatched source commitment or batch. `committed()` returns the committed height and block hash retained in the archive. `committed_receipt(index, keys)` verifies one archived receipt under the supplied sequencer height-range keys. Admission checks the archive against its independently observed source commitment. To verify BFT consensus or application state independently, use native block-commit evidence and the relevant state proofs in addition to the mirror report.

## Archive retention and audit

Keep the archive commitment, canonical archive bytes, trusted sequencer key windows, target identity, canonical chain position and evidence verification result together. For independent settlement verification, retain native block-commit evidence and the trusted validator configuration as well.

Continue with [Relay & Archive Nodes](https://docs.paxeer.app/relay-archive) for public history endpoints, [Receipts & Proofs](https://docs.paxeer.app/proofs) for portable verification, and [Data Availability](https://docs.paxeer.app/data-availability) for authenticated replay bundles.

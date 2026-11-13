# Inference market

The inference market coordinates paid model inference across a host and peers.
An asker buys tokens in a room for a particular model and runtime; the host
assigns model layers to peers, collects signed lap receipts and settles payment.
The chain verifies records and pays rewards. Validators and nodes never run the
model or join a room.

## Availability

The daemon accepts full, human-owner and inference-market.
The [mode parser](../../../services/agent/crates/layerx-agentd/src/human_owner_mode.rs)
and [profile configuration](../../../services/agent/crates/layerx-agentd/src/config.rs)
are implemented, as are the attestor's peer-binding methods at `0x0907`.
These guides cover the F11 operations, daemon profile, peer runtime, SDK
additions and wallet/explorer surfaces; implementation does not establish
network deployment or end-to-end readiness.
Check F11 availability on the target network before attempting the flows.

## Three planes

| Plane | Runs where | Responsibility |
| --- | --- | --- |
| Protocol and settlement | Node execution of the ai-market program F11, pay at `0x101B`, attestor at `0x0907` | Enrolment records, rooms, asks, bids, deals, sealed receipts, challenges, bonds, metered streams and reward credits |
| Orchestration | Operator's `layerx-agentd` inference-market profile | Capability enrolment, placement, signalling, signed writes, lap collection, canary checks, recovery and completion routes |
| Inference | Host plus browser or native peers; see the [peer guide](../../../apps/inference-peer/README.md) | Execute the assigned model layers over WebRTC data channels and countersign contributed laps; the host keeps embedding and head |

The operator runs the host daemon and provides its node connection, signing
identity, model/runtime digests and room policy. Peer operators provide compute,
their own signing identities and bonds. The asker funds one metered ask stream
and calls the host completion listener. SDKs and the wallet prepare writes through
the existing signing path; the explorer reads chain records through its server.
No participant gains protocol authority from an HTTP success or a local receipt.

## Guides

- [Protocol](protocol.md): lifecycle, selectors and payload fields, lap evidence,
  canaries, payouts, bonds and the stream meter invariant.
- [Operator guide](operator.md): configuration, enrolment, opening rooms,
  funding, settlement, claims and recovery.
- [Client guide](client.md): SDK names, node reads, completion routes, wallet
  and explorer entry points.
- [Threat statement](threats.md): development attestation, incorrect answers,
  liveness, collusion and privacy limits.

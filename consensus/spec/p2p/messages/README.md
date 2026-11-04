---
order: 1
parent:
  title: Messages
  order: 1
---

# Messages

Paxeer's embedded consensus engine supports compatibility reactor channels and
Autobahn streaming RPCs. The selected mode and authenticated connection type
determine which protocol applies. These protocols run in the single `paxd`
network; they do not define additional execution or settlement domains.

## Mode Applicability

A non-empty top-level `autobahn-config-file` enables Autobahn. In
[`node.go`](../../../node/node.go), the classic mode starts consensus, mempool,
state-sync and block-sync reactors. Autobahn owns its consensus, data fetch and
execution paths, while a block-sync reactor without a sync controller remains
available to serve compatibility block and status requests. PEX and evidence
reactors are constructed independently of that mode branch; actual delivery
still depends on the peer's negotiated transport and advertised channels.

The compatibility connection exchanges channel support in `NodeInfo` and carries
protobuf reactor messages over `MConnection`. See
[Connection](../connection.md#mconnection) for packet framing and channel APIs,
and [Config](../config.md) for peer settings. These inherited protocol pages
cover the compatibility messages:

- [Block Sync](./block-sync.md): block requests and retained-height status.
- [Mempool](./mempool.md): classic transaction gossip.
- [Evidence](./evidence.md): evidence exchange.
- [State Sync](./state-sync.md): classic snapshot and state synchronization.
- [Pex](./pex.md): peer address exchange.
- [Consensus](./consensus.md): classic consensus gossip, separate from Autobahn certificates.

## Autobahn Streaming RPCs

Autobahn uses authenticated connections with `pax_giga_connection` enabled and
the stream multiplexer under `internal/p2p/mux`. Stream-kind numbers below are
registered by [`giga/api.go`](../../../internal/p2p/giga/api.go); they are not
compatibility channel IDs. Wire request/response messages are declared in
[`giga/api.proto`](../../../internal/p2p/giga/api.proto) and
[`autobahn.proto`](../../../internal/autobahn/autobahn.proto).

| Kind | Registration | Request | Response |
| --- | --- | --- | --- |
| 0 | `Ping` | `PingReq` | `PingResp` |
| 1 | `StreamLaneProposals` | `StreamLaneProposalsReq` (`first_block_number`) | `LaneProposal` carrying a signed lane proposal |
| 2 | `StreamLaneVotes` | `StreamLaneVotesReq` | `LaneVote` carrying a signed lane vote |
| 3 | `StreamCommitQCs` | `StreamCommitQCsReq` | `autobahn.CommitQC` |
| 4 | `StreamAppVotes` | `StreamAppVotesReq` | `AppVote` carrying a signed application vote |
| 5 | `StreamAppQCs` | `StreamAppQCsReq` | `StreamAppQCsResp` pairing `AppQC` with `CommitQC` |
| 6 | `Consensus` | `autobahn.ConsensusReq` | `ConsensusResp` |
| 7 | `StreamFullCommitQCs` | `StreamFullCommitQCsReq` (`next_block`) | `autobahn.FullCommitQC` |
| 8 | `GetBlock` | `GetBlockReq` (`global_number`) | `GetBlockResp` with an optional Autobahn block |

`ConsensusReq` carries a full proposal, prepare vote, commit vote, full timeout
vote or timeout QC. Consensus updates stream from client to server. The other
streams provide lane availability, application certification and block data;
[`giga/service.go`](../../../internal/p2p/giga/service.go) starts their actual
client and server handlers.

The registrations bound message sizes, windows, call rates and concurrent
streams. In particular, `Consensus` and `CommitQC` messages are limited to 10 KiB,
`FullCommitQC` to 100 KiB, and block-bearing responses and lane proposals to
`MaxBlockProtoSize + 10 KiB`. These are per-message limits, not block-size
settings for the compatibility multiplexer.

## Certificate Consumption

[`giga/data.go`](../../../internal/p2p/giga/data.go) resumes full commit-QC
streaming at the next required global block number. It decodes and verifies the
certificate against the committee, rejects a range that does not cover and
advance that number, then inserts it into the data state. `GetBlock` fetches the
payload separately, and data-state insertion checks it against certified headers.
A missing block is an absent optional field in `GetBlockResp`.

The [classic light-client draft](../../light-client/verification/verification_002_draft.md)
describes trusting-period and validator-overlap reasoning. Its
[signature-verification companion](../../light-client/verification/verification_003_draft.md)
explains the classic early-exit verifier. Neither establishes equivalence to
Autobahn's committee-bound certificates or durable recovery.

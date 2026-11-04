## Abstract

The epoch module manages a fixed-duration schedule inside `paxd` on the single
Paxeer network. The default duration is one minute, measured from the epoch
genesis time. In [`BeginBlock`](../keeper/abci.go), a transition occurs only when
consensus block time is strictly after the current epoch's scheduled end:
`block time - current epoch start time > epoch duration`. Equality does not
trigger a transition.

Each transition calls `AfterEpochEnd` with the ending epoch, stores the next
epoch, calls `BeforeEpochStart` with that new epoch, and emits `new_epoch`, all in
the same block. The epoch number increases by one, its scheduled start advances
by exactly one duration, and its height becomes the current block height. A hook
panic propagates and aborts block processing.

After downtime, the module catches up at most one elapsed period per block. It
preserves the scheduled start times instead of resetting the schedule to the
current block time. For example, a block five minutes and one second after the
start of a one-minute epoch advances the schedule by just one minute; later
blocks process the remaining elapsed periods. See
[`TestEpochCatchupPreservesElapsedPeriods`](../keeper/abci_catchup_test.go) and the
[hook timing and catch-up example](../README.md#hooks).

## Contents

The module has no transaction messages and its parameter set is empty. Its
[state and query](../README.md#state), [hooks](../README.md#hooks),
[events](../README.md#events), and [package verification history](../README.md#package-verification)
are described in the module README.

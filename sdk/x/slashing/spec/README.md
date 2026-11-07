<!--
order: 20
title: Slashing Overview
parent:
  title: "slashing"
-->

# `x/slashing`

## Abstract

This section specifies the slashing module of the Cosmos SDK, which implements functionality
first outlined in the Cosmos whitepaper.

The slashing module enables Cosmos SDK-based blockchains to disincentivize any attributable action
by a protocol-recognized actor with value at stake by penalizing them ("slashing").

Penalties may include, but are not limited to:

- Burning some amount of their stake
- Removing their ability to vote on future blocks for a period of time.

In this repository the module is wired into the Paxeer X chain application in [`node/app.go`](../../../../node/app.go). Implementation notes for this fork are in [`../README.md`](../README.md).

## Contents

1. **[Concepts](01_concepts.md)**
   - [States](01_concepts.md#states)
   - [Tombstone Caps](01_concepts.md#tombstone-caps)
   - [Infraction Timelines](01_concepts.md#infraction-timelines)
2. **[State](02_state.md)**
   - [Signing Info](02_state.md#signing-info-liveness)
3. **[Messages](03_messages.md)**
   - [Unjail](03_messages.md#unjail)
4. **[Begin-Block](04_begin_block.md)**
   - [Liveness Tracking](04_begin_block.md#liveness-tracking)
5. **[05_hooks.md](05_hooks.md)**
   - [Hooks](05_hooks.md#hooks)
6. **[Events](06_events.md)**
   - [BeginBlocker](06_events.md#beginblocker-handlevalidatorsignature)
   - [MsgServer](06_events.md#msgserver)
7. **[Staking Tombstone](07_tombstone.md)**
   - [Abstract](07_tombstone.md#abstract)
8. **[Parameters](08_params.md)**
9. **[Client](09_client.md)**
    - [CLI](09_client.md#cli)
    - [gRPC](09_client.md#grpc)
    - [REST](09_client.md#rest)

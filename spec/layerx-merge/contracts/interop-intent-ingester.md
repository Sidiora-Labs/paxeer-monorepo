# Interop event ingestion contract

Interop services observe events and receipts from the Paxeer node. Paxeer mainnet-beta has EVM chain ID `125` (`0x7d`); receipts are final once their blocks are committed.

| Consumer | Source and behavior |
|---|---|
| Bridge relayer | [Relayer](../../../services/interop/crates/layerx-bridge-relayer/src/relayer.rs) observes bridge events and journals attestations and submissions; see the [bridge relayer contract](interop-bridge-relayer.md). |
| Mirror | [Committed block source](../../../services/interop/crates/layerx-mirror/src/committed.rs) acquires blocks and authenticated receipts by committed coordinate; see the [mirror contract](interop-mirror.md). |
| Web search | [Request watcher](../../../services/interop/crates/x-websearch/src/watch.rs) reads xweb request logs and persists its cursor and pending requests; see the [web-search contract](interop-websearch.md). |

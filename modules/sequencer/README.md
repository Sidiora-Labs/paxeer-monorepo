# Sequencer module

This directory holds the chain module for sequencing state in Paxeer's BFT network: the leader list, pinned receipt keys, activation height, committed sequence position and hash, and decided votes. The proposer injects `MsgSequenceBatch` at transaction index zero after activation; the keeper validates its height, continuity and signature before recording the sequence state. Sequencing runs inside the node through [`node/sequencer`](../../node/sequencer/). A receipt becomes final when its covering block commits.

## Layout

- [`module.go`](module.go): module registration, genesis validation and import/export, and governance handlers that synchronize leader and receipt-key parameters with keeper state.
- [`module_test.go`](module_test.go): genesis round trips and governed leader changes.
- [`keeper/`](keeper/): persistent sequence state, activation and leader scheduling, receipt-key validation and rotation, decided votes, and store and proof tests.
- [`types/`](types/): genesis and batch types, canonical batch encoding, signature verification, sequence hashing, transaction roots, stored-vote encoding, and their tests.

## Build and test

The module belongs to the root [`go.mod`](../../go.mod) and has no separate build manifest or Makefile. From the repository root, use the chain targets declared in the root [`Makefile`](../../Makefile):

```sh
make paxeer-build
make paxeer-test
```

These targets delegate to [`chain.mk`](../../chain.mk). The build produces `build/paxd` with this module and its native dependencies. The test target builds the native dependencies and runs the split Go package suite, including this directory's module, keeper and types tests. Both targets use locally available Go modules.

## Configuration

The module receives genesis JSON from the application and reads the `sequencer` parameter subspace and module store. Its genesis fields are `leaders`, `receipt_keys`, `last_seq`, `last_seq_hash` and `activation_height`; receipt-key entries use `key_id`, `public_key`, `first_height` and `last_height`. Governance updates the `Leaders` and `ReceiptKeys` parameters. The module reads no standalone configuration file or runtime environment variable.

The keeper's proof-vector tests read `LXP_WRITE_VECTORS` to control writing vector files under [`programs/tests/vectors`](../../programs/tests/vectors/). Node role configuration belongs to [`node/sequencer/config.go`](../../node/sequencer/config.go).

## Documentation

- [Sequencing and block finality](../../docs/wiki/Sequencing.md)
- [Chain module reference](../../docs/system/chain-modules.md#sequencer)
- [Documentation site: Sequencing & Finality](../../docs/site/paxeer-docs/public/markdown/sequencing.md)
- [Documentation site page source](../../docs/site/paxeer-docs/src/app/sequencing/page.tsx)

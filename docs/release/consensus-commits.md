# Consensus-changing commits for v6.12.0

The next release is **v6.12.0**, as declared in
[`tools/release/version.json`](../../tools/release/version.json). This table
tracks consensus-changing commits included in that release by their full Git
hash, state-machine change, non-test consensus paths and coordinated upgrade.

Under decision.1906, a binary containing a listed commit is **never rolled out
node by node**. Follow the [governance upgrade procedure](../../RELEASE.md):
every validator and full node halts at the agreed plan height and restarts on
the same release binary. The v6.12.0 plan is `V6120Upgrade` in
[`node/upgrades_v6120.go`](../../node/upgrades_v6120.go). The governance plan
supplies the height; this ledger does not invent a halt height.

| Full commit hash | State-machine change | Non-test consensus paths | Coordinated upgrade |
| --- | --- | --- | --- |
| `0b48316e2eea28256461f1a1a99c753b2666f98c` | Whole-position closes pass the exact position base size to `sizeOrder` and simulate the same sizing in quotes, preventing an unaligned mark from leaving one lot open. | `modules/perps/keeper/orders.go`, `modules/perps/keeper/quotes.go`, `precompiles/perps/positions.go` | v6.12.0 through `V6120Upgrade`, which first adds the perps store and initializes its state. |
| `12fbbeb80389f9021d791db00d3f32101ea51668` | Read the sequenced flag from chain state and the applied merge-plan height, so node-local configuration cannot change the accepted block format. | `node/app.go`, `node/sequencer/consensus_source.go` | Ship the binary through the coordinated v6.12.0 `V6120Upgrade`; sequenced blocks activate through `MergeUpgrade` (`v7.0`) in `node/upgrades.go`, as recorded in chain state. |
| `cdd97253c4219a77363a62377c36d212cda5a7ba` | Refuse a plain usage commit for a fraud-provable compute lease; its claim must use sandbox billing with challengeable profile and billing rows. | `programs/crates/layerx-programs-market/src/lib.rs`, `programs/crates/layerx-programs-market/src/settle.rs` | v6.12.0 coordinated binary upgrade through `V6120Upgrade`; all nodes use the same compute-market rules after restart. |
| `97d256444123c41a8db5c9b3d0c4ef2d05df4e0f` | Anchor forced exits to the first live exit checkpoint and refuse ordinary withdrawals proved in later batches, including after the emergency clears. | `modules/layerxcustody/keeper/claims.go`, `modules/layerxcustody/types/errors.go` | v6.12.0 coordinated binary upgrade through `V6120Upgrade`; all nodes switch custody claim and withdrawal rules together. |
| `68c8887a9520542d2878da3d6a44920ddd71b4b1` | Persist the exit checkpoint under its own key, rebuild it on cancellation and genesis, and refuse deposits while a forced exit is live. | `modules/layerxcustody/keeper/claims.go`, `modules/layerxcustody/keeper/deposit.go`, `modules/layerxcustody/keeper/genesis.go`, `modules/layerxcustody/types/errors.go`, `modules/layerxcustody/types/keys.go` | v6.12.0 coordinated binary upgrade through `V6120Upgrade`; all nodes switch checkpoint storage and deposit refusal rules together. |

For each additional consensus-changing commit, add a row in a subsequent commit
on the same pull request, including its upgrade plan or halt height. A commit
cannot contain its own final hash. The release workflow checks every non-merge
commit in the pull request's base-to-head range against the table **at the head
revision**, even when the table update comes later than the consensus change.

Run the same check locally from the repository root with
`python3 tools/ci/consensus_commit_gate.py <base> <head>`.
The gate covers the prefixes in `CONSENSUS_PATHS` in that script, excludes
`_test.go` files, and prints every unlisted commit and its consensus paths.

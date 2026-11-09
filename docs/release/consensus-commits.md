# Consensus-changing commits for v7.0.0

The next release is **v7.0.0**, carrying plan **v7.0** (`MergeUpgrade`).
The release identity belongs in
[`tools/release/version.json`](../../tools/release/version.json). This ledger
tracks consensus-changing commits by their full Git hash, state-machine change,
non-test consensus paths and coordinated upgrade, retaining the historical ledger.

A binary containing a listed commit is **never rolled out
node by node**. Follow the [governance upgrade procedure](../../RELEASE.md):
every validator and full node halts at the agreed plan height and restarts on
the same release binary. The governance plan supplies the height; this ledger
does not invent a halt height.

## v7.0.0 ledger

The node registers both plans explicitly in
[`node/upgrades.go`](../../node/upgrades.go). Apply the merge first; retain
the anchor store until the separately coordinated deletion height.

| Plan | Registered handler | State-machine change |
| --- | --- | --- |
| `v7.0` (`MergeUpgrade`) | `mergeUpgradePlan` | Add the sequencer and merged stores, validate sequencer genesis and receipt keys covering the first sequenced height, retire anchor writes, return guarantor bonds and release retired custody balances. |
| `v7.0-anchor-store-delete` (`AnchorStoreDeleteUpgrade`) | Inline handler calling `app.mm.RunMigrations` | The store loader deletes `layerxanchor`; all anchor methods then refuse. This is a later plan with its own height and evidence. |

The inventory starts after the latest historical v6.12.0 ledger commit,
`68c8887a9520542d2878da3d6a44920ddd71b4b1` (exclusive base). Rows below were generated
with `python3 tools/ci/consensus_ledger_rows.py 68c8887a9520542d2878da3d6a44920ddd71b4b1 8c5a7acd059a88796c193d07d122b8805fee29bb`,
which reuses the consensus gate's unlisted-commit discovery and emits rows
oldest first in Git topological order; the existing row is retained.

| Full commit hash | State-machine change | Non-test consensus paths | Coordinated upgrade |
| --- | --- | --- | --- |
| `12fbbeb80389f9021d791db00d3f32101ea51668` | Read the sequenced flag from chain state and the applied merge-plan height, so node-local configuration cannot change the accepted block format. | `node/app.go`, `node/sequencer/consensus_source.go` | v7.0.0 coordinated binary upgrade through `MergeUpgrade` (`v7.0`); sequenced blocks activate as recorded in chain state. |
| `c3520a88056e6fb7b65daf706ae05f4b8fabbd07` | px26-worker px26-fm1588 2026-10-22 run1020/IS-712-GATE a83e3ee0f72c20d868e8fa2e961a6df9b10a3206 "Install the merge precompile gate and cover address withholding with a passing node build and focused activation tests [spec:layerx-merge]" | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `fbaac34c7ce0a56679d6ad2d3617b62a4dda36ef` | Add EventRetiredRelease to the custody events proto and regenerate it [spec:layerx-merge/6.41.3] | `modules/layerxcustody/types/events.pb.go` | `v7.0` (`MergeUpgrade`) |
| `134f076b3457af8a90f22e08e96dfd65fc0cbc5e` | Add the custody merge plan name and the ErrModuleRetired code 29 [spec:layerx-merge/6.41.1] | `modules/layerxcustody/types/errors.go`, `modules/layerxcustody/types/retire.go` | `v7.0` (`MergeUpgrade`) |
| `813a8b2de31f54702151e1f59893b3743bb9156d` | Add Keeper.Retired reading the v7.0 plan from the bound upgrade reader [spec:layerx-merge/6.41.2] | `modules/layerxcustody/keeper/retire.go` | `v7.0` (`MergeUpgrade`) |
| `9ccb46b0d0ae9f60ad1dee2cbde49bd4931c4d3b` | Refuse withdrawals, forced exits and claim cancellation from the merge plan height [spec:layerx-merge/6.42.1] | `modules/layerxcustody/keeper/claims.go` | `v7.0` (`MergeUpgrade`) |
| `0b3872e6ea0f8e57e73baec07890a71f6cbb39e8` | Refuse custody parameter, asset and emergency changes from the merge plan height [spec:layerx-merge/6.43.4] | `modules/layerxcustody/keeper/keeper.go` | `v7.0` (`MergeUpgrade`) |
| `7f82452585611af50988d65a45c9a0186168ab80` | Add ReleaseRetired paying stranded custody balances to the authority once [spec:layerx-merge/6.46.1] | `modules/layerxcustody/keeper/release.go` | `v7.0` (`MergeUpgrade`) |
| `2ee6f997b4477c0950bcda814a064470c452ac9a` | Refuse authority checkpoint registration from the merge plan height [spec:layerx-merge/6.43.3] | `modules/layerxcustody/keeper/anchor.go` | `v7.0` (`MergeUpgrade`) |
| `07894ad30176786beafd1e17954434aaf34639d0` | Refuse deposit-root registration from the merge plan height [spec:layerx-merge/6.43.2] | `modules/layerxcustody/keeper/deposit_root.go` | `v7.0` (`MergeUpgrade`) |
| `04080421eb4b1e7ef21e09c86035176823cd392a` | Declare PerpsAddress as the 0x1015 literal [spec:layerx-merge/3.3.1] | `precompiles/perps/perps.go` | `v7.0` (`MergeUpgrade`) |
| `3fa3a5193a8e9628786d530077b926845fe027a9` | Add the keeper SetPaused path under the gov authority [spec:layerx-merge/3.1.3] | `modules/perps/keeper/pause.go` | `v7.0` (`MergeUpgrade`) |
| `fbb6ad6fd272a5a47d83f1e946c5dd8ce2e3a304` | Export the perps keeper genesis validation that InitGenesis runs [spec:layerx-merge/3.5.1] | `modules/perps/keeper/genesis.go` | `v7.0` (`MergeUpgrade`) |
| `3b328307e82e2c227cf325b5fd6ad2a097996564` | Bound ADL per block by Params.AdlPositionsPerBlock [spec:layerx-merge/3.6.2] | `modules/perps/keeper/adl.go` | `v7.0` (`MergeUpgrade`) |
| `f14a08f183026c8af9765f278eddf9b4b1d9f2e5` | Serve MsgUpdateParams through the perps msg server with a params event [spec:layerx-merge/3.1.4] | `modules/perps/keeper/msg_server.go` | `v7.0` (`MergeUpgrade`) |
| `5e26b7a5d97cf7bd61c89a846d0314d5e7ffa947` | Make Withdrawable read Params.WithdrawFloorPpm instead of a fixed divisor [spec:layerx-merge/3.6.1] | `modules/perps/keeper/account.go` | `v7.0` (`MergeUpgrade`) |
| `032849a17260c377c8da4101f2250384d394e11e` | Serve MsgSetPaused and emit the pause event [spec:layerx-merge/3.1.6] | `modules/perps/keeper/msg_server_pause.go` | `v7.0` (`MergeUpgrade`) |
| `36c90b2b694ff7920e284c6406857943d3e6e10d` | Add App seams for the sequencer source, the sequencer RPC options and a commit listener [spec:layerx-merge/5.0.1] | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `f504f41db25d0c209dcb92db392fd07279852ba5` | Refuse all nine custody Msgs from the merge plan height before decoding [spec:layerx-merge/6.44.1] | `modules/layerxcustody/keeper/msg_server.go` | `v7.0` (`MergeUpgrade`) |
| `23452f776a13e46566df85ce7691f4149f6c64ee` | Refuse lowering MaxOpenOrdersPerAccount below an account's open orders [spec:layerx-merge/3.6.12] | `modules/perps/keeper/msg_server.go` | `v7.0` (`MergeUpgrade`) |
| `0c76247257c1bca8b4c7c6c79a04af103c0716bd` | Report the fencing lease from WAL replay [spec:layerx-merge/5.26.1] | `node/sequencer/wal/wal.go` | `v7.0` (`MergeUpgrade`) |
| `4aa2bd0da5779eef74ff67dd2d580cfc71921489` | Refresh the precompile set inside executor Open [spec:layerx-merge/5.4.2] | `node/sequencer/executor.go` | `v7.0` (`MergeUpgrade`) |
| `e4ba17a6241fed8e39fbcbcd652b8d3e5eb582ed` | Keep the sealed batch when ConsensusSource.Seal fails after the executor sealed [spec:layerx-merge/5.4.4] | `node/sequencer/consensus_source.go` | `v7.0` (`MergeUpgrade`) |
| `e645edd46e17d1e81f99326a860e9c8240eee1d9` | Add the receipt forward URL and the standby role keys to the sequencer config [spec:layerx-merge/5.5.1] | `node/sequencer/config.go` | `v7.0` (`MergeUpgrade`) |
| `7ebe6f9c0d1beafe6654a97ae599ff197f46111a` | Report the standby replication WAL sync time to the sequencer driver [spec:layerx-merge/5.92.1] | `node/sequencer/standby.go` | `v7.0` (`MergeUpgrade`) |
| `36bd959682910d301e717d6209ae1b3cadd2bdd0` | Place each accepted transaction with SetTxPosition [spec:layerx-merge/5.4.1] | `node/sequencer/accept.go` | `v7.0` (`MergeUpgrade`) |
| `04b2d6a2f0927ab477bc5cd2af69f5ef89aca79e` | Add the sequencer runtime metrics instruments [spec:layerx-merge/5.5.2] | `node/sequencer/metrics.go` | `v7.0` (`MergeUpgrade`) |
| `c74f5407e47d8576ed76a05d63bd8285c2f3cbfe` | Restore the standby lease epoch on restart [spec:layerx-merge/5.26.2] | `node/sequencer/standby.go` | `v7.0` (`MergeUpgrade`) |
| `261382cd62fce82f40e480023fb8e86f1feea26d` | Merge task 5.26.2 into feature/node-runtime [spec:layerx-merge/5.26.2] | `node/sequencer/standby.go` | `v7.0` (`MergeUpgrade`) |
| `d2a8adc582e4888cad9fd9c10c69dd80e60e2d58` | Report the last logged ACCEPT from WAL replay [spec:layerx-merge/5.27.1] | `node/sequencer/wal/wal.go` | `v7.0` (`MergeUpgrade`) |
| `88781fdd74200371387559f1485a91f42b7b4c4c` | Add WAL retention, nonce queue bounds, lease duration and the start flags to the sequencer config [spec:layerx-merge/5.5.15] | `node/sequencer/config.go` | `v7.0` (`MergeUpgrade`) |
| `b6978b427618a8e7f4796efcaf982b7e4e4ab7f0` | Add the bounded TCP dialer for the standby replication link [spec:layerx-merge/5.5.5] | `node/sequencer/dial.go` | `v7.0` (`MergeUpgrade`) |
| `f3ef8c257053361e5b73fba93f7a16140e47d426` | Refuse a transaction that would overflow the block gas or byte limits at Accept [spec:layerx-merge/5.4.3] | `node/sequencer/accept.go`, `node/sequencer/limits.go` | `v7.0` (`MergeUpgrade`) |
| `545f44fa65d258a8d650fe959a136cc3fbfc5269` | Place micro-batch transactions with SetTxPosition [spec:layerx-merge/5.4.7] | `node/sequencer/microbatch.go` | `v7.0` (`MergeUpgrade`) |
| `5df1a45f6361902f24585eade8c3d604479a8b16` | Refresh the receipt key pins from the keeper at each Open [spec:layerx-merge/5.5.16] | `node/sequencer/receiptkey.go` | `v7.0` (`MergeUpgrade`) |
| `5596ceb585229c5dc2415f1969b757940665cde1` | Add the receipt source that serves accepted sequencer receipts by hash [spec:layerx-merge/5.5.3] | `node/sequencer/receipt_source.go` | `v7.0` (`MergeUpgrade`) |
| `b5ad82a45a3f07be7341253c72304847f342b0b4` | Wire the order and proposer checks into ProcessProposal and halt on a failed batch [spec:layerx-merge/5.0.2] | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `2e843a8ebb3dbe6bb3d2b292704eb4fcc5b410cc` | Use the module seqHash in the executor [spec:layerx-merge/5.17.1] | `node/sequencer/executor.go` | `v7.0` (`MergeUpgrade`) |
| `010afd7ff1d9c1fdd14b7ae52f7c18f86a4f424c` | Fence the old primary through the standby lease and the ack epoch [spec:layerx-merge/5.27.2] | `node/sequencer/standby.go` | `v7.0` (`MergeUpgrade`) |
| `d9d69e1ea660abf375176484d1f963f462d7584f` | Admit future-nonce transactions through the nonce queue [spec:layerx-merge/5.5.11] | `node/sequencer/nonce_admission.go` | `v7.0` (`MergeUpgrade`) |
| `6d66eaf9f2e3c7ad65ec978ed91ec97a048c6ba6` | Forward sequencer receipt reads from a non-sequencer node [spec:layerx-merge/5.5.13] | `node/sequencer/follower.go` | `v7.0` (`MergeUpgrade`) |
| `93ef54d28c5a44479eab554973f19aa246a60b21` | Record the seal of each block in the WAL before the sealed block is returned [spec:layerx-merge/5.5.14] | `node/sequencer/consensus_source.go` | `v7.0` (`MergeUpgrade`) |
| `91b92c6d584701322bc1b86b305a61cc011aa266` | Add the accepter that receipts a send only after WAL write and standby ack [spec:layerx-merge/5.5.4] | `node/sequencer/accepter.go` | `v7.0` (`MergeUpgrade`) |
| `17af9fe35dfc40fe614d82696253638c35922bac` | Publish receipted logs to the RPC receipt log feed [spec:layerx-merge/5.5.10] | `node/sequencer/log_feed.go` | `v7.0` (`MergeUpgrade`) |
| `9b55e2c937093469fa649a658f35bd3c0ee31a26` | Take a fencing lease when the primary sequencer starts [spec:layerx-merge/5.15.1] | `node/sequencer/lease.go` | `v7.0` (`MergeUpgrade`) |
| `d3031ca2faabe404564fd373e6bec975c0e064c9` | Add the block driver that commits, prunes and opens the next block [spec:layerx-merge/5.5.7] | `node/sequencer/driver.go` | `v7.0` (`MergeUpgrade`) |
| `c1b44508da3fd690c0e32f10cbd927b8855a39d1` | Add the production WAL replayer that re-executes accepted transactions [spec:layerx-merge/5.5.8] | `node/sequencer/replay.go` | `v7.0` (`MergeUpgrade`) |
| `b3de702c1d3d7e45a63a534148f643acf49e9128` | Renew the sequencer lease before it expires [spec:layerx-merge/5.15.2] | `node/sequencer/lease_renew.go` | `v7.0` (`MergeUpgrade`) |
| `898e74278becbe6f1b69f6760f76f952832c02ec` | Refuse custody deposits from the merge plan height [spec:layerx-merge/6.43.1] | `modules/layerxcustody/keeper/deposit.go` | `v7.0` (`MergeUpgrade`) |
| `8fe024ea01787076e158732ca52043598cb51cfa` | Serve MsgSetMarket with pricefeed listing and identity checks [spec:layerx-merge/3.1.5] | `modules/perps/keeper/msg_server_market.go` | `v7.0` (`MergeUpgrade`) |
| `1ffb1822344dc9435faff82c99e58a6ff3fbeb70` | Start a standby from the sequencer config role [spec:layerx-merge/5.5.9] | `node/sequencer/standby_role.go` | `v7.0` (`MergeUpgrade`) |
| `554fc97e404901cbc79d26fb426f70fe1cb16f15` | Fail over to the standby after an unplanned primary stop [spec:layerx-merge/5.27.3] | `node/sequencer/failover.go` | `v7.0` (`MergeUpgrade`) |
| `90e07859ca34ae2c2b1d658f5b6ac897ae9cdef3` | Build the custom precompiles through GetCustomPrecompilesE [spec:layerx-merge/5.19.1] | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `24fb4183889ee96c7090a2b911d3b8d91a0a2e18` | Merge task 5.19.1 into feature/node-runtime [spec:layerx-merge/5.19.1] | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `238dc2034f609037cb74bf5586e8223ba8610aff` | Carry the committed app hash and last block id in the executor Open header [spec:layerx-merge/5.4.5] | `node/app.go`, `node/sequencer/executor.go` | `v7.0` (`MergeUpgrade`) |
| `e856ff1c9cd17a51b7908dae0cf5e3e6cadcb025` | Retire the Layers seal path in favour of SealBatch [spec:layerx-merge/5.17.2] | `node/sequencer/layers.go`, `node/sequencer/seal.go` | `v7.0` (`MergeUpgrade`) |
| `0dc739534641cc9a8450e2ac0bbf71ce2644b79d` | Fence the executor so a superseded primary writes nothing [spec:layerx-merge/5.15.3] | `node/sequencer/accept.go`, `node/sequencer/executor.go`, `node/sequencer/fee.go`, `node/sequencer/microbatch.go` | `v7.0` (`MergeUpgrade`) |
| `33c2f9a35d72a18955f753ce13c4475f5b3f5dd4` | Fence the primary on ErrFenced and build the promoted standby's consensus source [spec:layerx-merge/5.15.4] | `node/sequencer/consensus_source.go` | `v7.0` (`MergeUpgrade`) |
| `3e99b79efc04a703820a84d23cefe5e55407330d` | Hand over the open block between hosts with a lease [spec:layerx-merge/5.27.4] | `node/sequencer/handover.go` | `v7.0` (`MergeUpgrade`) |
| `bd41925be1f2d09194bb32e90eccffad4a65189a` | Assemble the sequencer runtime from config [spec:layerx-merge/5.5.12] | `node/sequencer/node.go` | `v7.0` (`MergeUpgrade`) |
| `2c65d2c3c6901b1a016fb9e3679cb65d9a186b59` | Start the sequencer node: load the key, replay the WAL and install it on the App [spec:layerx-merge/5.5.19] | `node/sequencer/node_start.go` | `v7.0` (`MergeUpgrade`) |
| `43cfe1647abe81017447f7043ce416983a915f4a` | Stop the sequencer node cleanly [spec:layerx-merge/5.5.17] | `node/sequencer/node_stop.go` | `v7.0` (`MergeUpgrade`) |
| `1e3c9243f68df5ba3f6598b55bb52aef4178ee67` | Register the ledger, auth and agent modules and the shared host in the App [spec:layerx-merge/5.19.2] | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `9c04cbe1a794258f4ebca3c8e242a35130c565ee` | Freeze the 0x1015 order and market view outputs in a golden [spec:layerx-merge/3.7.2] | `precompiles/perps/testdata/views_market_golden.json` | `v7.0` (`MergeUpgrade`) |
| `9b8968f3025f187529497f7b5759ed4d91d155ee` | Freeze the 0x1015 quote outputs in a golden [spec:layerx-merge/3.7.4] | `precompiles/perps/testdata/views_quotes_golden.json` | `v7.0` (`MergeUpgrade`) |
| `eed67ac9c010eb1758bb1e4852275df88297b642` | Freeze the 0x1015 account and position view outputs in a golden [spec:layerx-merge/3.7.1] | `precompiles/perps/testdata/views_account_golden.json` | `v7.0` (`MergeUpgrade`) |
| `9468751909c5b2219f11860190805ba27ca430c0` | Freeze the 0x1015 insurance and ADL view outputs in a golden [spec:layerx-merge/3.7.5] | `precompiles/perps/testdata/views_insurance_golden.json` | `v7.0` (`MergeUpgrade`) |
| `c1902ec34ef5e6e2551793d981bc14db264962ab` | Freeze the 0x1015 funding and fee view outputs in a golden [spec:layerx-merge/3.7.3] | `precompiles/perps/testdata/views_funding_golden.json` | `v7.0` (`MergeUpgrade`) |
| `eb832eb4d53709d42d9c43ef03845f6f29ccdb30` | Run the layerxauth revocation decorator in the ante chain [spec:layerx-merge/5.14.3] | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `0fb1aa46ccae0857d0204861ea0836d1d6c1262d` | Pass the sequencer RPC options and wire the merged keepers, the served spot set [spec:layerx-merge/5.0.3] | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `2888be80def09b4b83a63d930bd0035df7673ab4` | Build the layerxanchor module with the upgrade reader and route the gated msg server [spec:layerx-merge/6.31.5] | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `0ab6b99623d5dadcef4d72a397ed6167cfff9a9d` | Adopt the sealed block in FinalizeBlock and store the decided votes [spec:layerx-merge/5.19.3] | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `27f40329f2eb8a4a0fe32dfd764e014f56c0dde5` | Merge the qualified node-runtime work into main | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `4b596e6ea8314343f8bb33876e333f90bdbbb0b7` | Drop the unused distrKeeper from the tokenfactory keeper [spec:layerx-merge/6.26.2] | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `0b66c4e6012786e16a978a5e4980e69c37a73b4d` | Merge task 6.26.2 into feature/chain-modules [spec:layerx-merge/6.26.2] | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `f288cd755ab108e205f0daff1dc47f4593dee316` | Merge the qualified chain-modules work into main | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `d34d5ea8db5c87f8f4335db03fcbd0f94ee647f5` | Merge task 6.28.5 into feature/chain-modules [spec:layerx-merge/6.28.5] | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `ae2beed6383a968df05f308a45f11d01d654258e` | Delete the anchor store and module at a second plan after the merge [spec:layerx-merge/5.7.1] | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `39c8bb7152e9675fdd1ed5723e87700ab2f3ca35` | Keep the anchor store deletion upgrade compiling after the store key constant retires [spec:layerx-merge/5.41.4] | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `5b2d955eee0b63b4c0919b509fb30a8a6bc1f2e2` | Keep retired anchor names only at deletion boundaries | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `2db1f1e3fe2868eae8cef51a3f14d380d26ca09d` | Preserve the accepted sequence frontier across retention pruning | `node/sequencer/handover.go`, `node/sequencer/standby.go` | `v7.0` (`MergeUpgrade`) |
| `85b415acd9943d7208cf8ae1a3505fd781e40dcd` | Let the standby cross commits and serve the sequencer base fee [spec:layerx-merge/5.55.6] | `node/app.go`, `node/sequencer/replay.go`, `node/sequencer/standby.go`, `node/sequencer/standby_role.go` | `v7.0` (`MergeUpgrade`) |
| `e4b4233271dfba196ef152892ea25524297ab7d2` | Merge the qualified node-runtime work into main | `node/sequencer/standby.go` | `v7.0` (`MergeUpgrade`) |
| `5885f13e244a95dcb0e3333dc83f1042412a2f86` | Issue #906: Seed the custody forced-exit checkpoint key on in-place upgrade [spec:layerx-merge/18.906.1] | `modules/layerxcustody/keeper/migrations.go` | `v7.0` (`MergeUpgrade`) |
| `65516073c40d6274ea32b29f14fa14c2902e7cbc` | Issue #715: Check the sequence batch range, txRoot and lastSeqHash against the block's transactions in FinalizeBlock [spec:layerx-merge/18.715.1] | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `e28cb9af076e1920c7331fdca40c0dfeeb65d3c4` | Issue #811: Refuse Bonded and Attested market offers and leases until they can be challenged [spec:layerx-merge/18.811.1] | `programs/crates/layerx-programs-market/src/lib.rs`, `programs/crates/layerx-programs-market/src/settle.rs` | `v7.0` (`MergeUpgrade`) |
| `58263ea9f0f554052061bef8e194f7985e0b54c3` | Issue #904: Size every perps close in base so a partial close fills the quoted lot [spec:layerx-merge/18.904.1] | `modules/perps/keeper/quotes.go`, `precompiles/perps/positions.go` | `v7.0` (`MergeUpgrade`) |
| `43af3141f0f51068f8ccb3272dd44935f287c068` | Issue #915: Document the forced-exit checkpoint and freeze in the custody docs [spec:layerx-merge/18.915.1] | `modules/layerxcustody/keeper/claims.go` | `v7.0` (`MergeUpgrade`) |
| `327ec63c0d8eb0bf60e8ec0973b8731d1fa7eacf` | Issue #830: Keep a keyed forced-exit total per account and asset instead of scanning every claim [spec:layerx-merge/18.830.1] | `modules/layerxcustody/keeper/claims.go`, `modules/layerxcustody/types/keys.go` | `v7.0` (`MergeUpgrade`) |
| `59417423e377ff9b625e524b5e6b0dd3e48cff5e` | Issue #1074: Make sealed-block adoption in FinalizeBlock commit the state follower re-execution computes [spec:layerx-merge/18.1074.1] | `node/app.go`, `node/sequencer/seal.go` | `v7.0` (`MergeUpgrade`) |
| `11f153ccffd44c540d40a577d1929eb6f0769517` | Issue #1109: Issue #1109: Make the standby acknowledge every replicated group of the committed-fixture recording through openAccount [spec:layerx-merge/18.1109.1] | `node/app.go`, `node/sequencer/seal.go`, `node/sequencer/standby.go` | `v7.0` (`MergeUpgrade`) |
| `f93d74e1caab5ed82115aabdd0ba313ca503cab9` | Issue #1120: Surface the standby's refusal reason and make it acknowledge the recorder's openAccount groups [spec:layerx-merge/18.1120.1] | `node/sequencer/standby.go` | `v7.0` (`MergeUpgrade`) |
| `07d11e56dd86d28a7c90ac37acbb8f5492f43823` | Issue #1126: Standby replays a ledger openAccount after register with the primary's receipt [spec:layerx-merge/18.1126.1] | `node/sequencer/standby_role.go` | `v7.0` (`MergeUpgrade`) |
| `d6c2e0ee24f32185f8d7dc13bc02558a9d1c96fd` | Issue #1060: Answer a commit-pending OPEN well inside the primary's replication deadline and let the primary retry it [spec:layerx-merge/18.1060.1] | `node/sequencer/standby.go` | `v7.0` (`MergeUpgrade`) |
| `c6973b8da83841e75d3471740b5ecfc149b2409d` | Issue #1061: Wait for the consensus commit without holding the standby mutex that guards lease takeover [spec:layerx-merge/18.1061.1] | `node/sequencer/standby.go` | `v7.0` (`MergeUpgrade`) |
| `3ed1f47aa7d69b1234eea438b1fcff63be4dfedc` | Issue #1144: Issue #1144: Commit the standby's replayed layer through the sealed-layer path so prior-block kernel reads rebuild the primary's receipt [spec:layerx-merge/18.1144.1] | `node/app.go`, `node/sequencer/standby_role.go` | `v7.0` (`MergeUpgrade`) |
| `4f6627f11677c5ccfddf435f3d1cdc04bf6df154` | Overlap the standby sync with re-execution and the primary sync with the exchange [spec:layerx-merge/5.16.7] | `node/sequencer/standby.go` | `v7.0` (`MergeUpgrade`) |
| `71835ba46447cf789c68c84db720c85997e7bcc3` | Issue #1299: Let the sequencer replication log carry the seal of an empty layer [spec:layerx-merge/18.1299.1] | `node/sequencer/wal/wal.go` | `v7.0` (`MergeUpgrade`) |
| `536f72a9469316d757941e46c5c88af35018ab91` | Export and import the shared host store in genesis before every host-writing module [spec:layerx-merge/2.41.1] | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `c7fdb284e57582e8b2dc7323bcb51211488a18ba` | Issue #1307: A failing commit listener fails Commit instead of being recovered and logged [spec:layerx-merge/18.1307.1] | `node/app.go`, `node/sequencer/driver.go`, `node/sequencer/node_start.go` | `v7.0` (`MergeUpgrade`) |
| `ab66e8cc9862b34b10f840c51b41e49fae5af751` | Merge the qualified node-runtime work into main | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `1503627359a03044ac5e3ef39e1080bff94043e8` | Check buf generate reproduces the checked-in Go for api and sdk/proto [spec:layerx-merge/7.29.2] | `modules/layerxcustody/types/genesis.pb.go`, `modules/layerxcustody/types/params.pb.go`, `modules/layerxcustody/types/proposal.pb.go`, `modules/layerxcustody/types/state.pb.go`, `modules/layerxcustody/types/tx.pb.go` | `v7.0` (`MergeUpgrade`) |
| `6805e86162937254115f0a8dcacc10e69bc8f852` | Release the next OPEN without waiting inside ABCI Commit for the standby's commit of the same height [spec:layerx-merge/12.35.5] | `node/sequencer/driver.go`, `node/sequencer/node_start.go`, `node/sequencer/standby.go` | `v7.0` (`MergeUpgrade`) |
| `450177d9e3847c10e24a2a9bdc3969d3ff8b6fe7` | Send the EVM RPC start signal when FinalizeBlocker adopts a sealed block [spec:layerx-merge/12.35.6] | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `13684186846693a1f94fde12f6e51baf1df62cc5` | Keep the anchor keeper and its store mounted in the binary that applies the v7.0 plan [spec:layerx-merge/5.7.5] | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `8fe2607dd5e923948e4cd79031d63c8a09e28108` | Retire the 0x1012 receipt verifiers and the custody inclusion and exit-balance paths [spec:layerx-merge/6.96.2] | `modules/layerxcustody/keeper/claims.go`, `modules/layerxcustody/types/ids.go` | `v7.0` (`MergeUpgrade`) |
| `30b042b4f681cb74f2cd4437a8e84e79ecfba2a0` | Carry the transient receipt through the accept critical section so a seal does not halt replication [spec:layerx-merge/12.35.8] | `node/sequencer/accept.go`, `node/sequencer/accepter.go`, `node/sequencer/executor.go` | `v7.0` (`MergeUpgrade`) |
| `93d2d11b1a4d65951cc2af24958fb03da0561139` | Acknowledge replicated groups on WAL durability so a pending commit cannot time the primary out [spec:layerx-merge/12.35.10] | `node/sequencer/standby.go` | `v7.0` (`MergeUpgrade`) |
| `a97ab62aafde875f883819dcfbff65e826947afa` | Run the sequencer primary without a standby when no standby address is configured [spec:layerx-merge/12.35.9] | `node/sequencer/node.go`, `node/sequencer/standby.go` | `v7.0` (`MergeUpgrade`) |
| `ca6a3aff298a8821e91f45fbef3d0f3681c1d7b9` | Merge task 12.35.9 into feature/node-runtime [spec:layerx-merge/12.35.9] | `node/sequencer/standby.go` | `v7.0` (`MergeUpgrade`) |
| `92ea1d44f61a7f14eea2fa0af105c839b5de69ea` | Log recovered RPC handler panics with their stack and prove the standalone primary receipts a signed transfer through the send path [spec:layerx-merge/12.35.11] | `node/sequencer/accepter.go`, `node/sequencer/node_start.go`, `node/sequencer/standby.go` | `v7.0` (`MergeUpgrade`) |
| `99bef8286afb048a84b967e1592c31457c6d9842` | Wait for the next open block on the RPC send path instead of panicking [spec:layerx-merge/5.98.5] | `node/app.go`, `node/sequencer/executor.go` | `v7.0` (`MergeUpgrade`) |
| `a5b872224c9d1dea51a2078283862521c1a649a4` | Record the receipt's logs at the receipt's height when the layer seals after Accept [spec:layerx-merge/5.98.7] | `node/sequencer/accepter.go` | `v7.0` (`MergeUpgrade`) |
| `7c6f9653c8ba38c7102ccf23792523d706dd9974` | Retain committed sends in the receipt source so pax_sequencerReceipt answers the committed rung [spec:layerx-merge/5.5.22] | `node/sequencer/receipt_source.go` | `v7.0` (`MergeUpgrade`) |
| `88e4603c6866aff847cd4780b81e9628f0ca1a24` | Admit the consensus mempool into the sequencer's open layer at each proposal [spec:layerx-merge/5.98.8] | `node/sequencer/consensus_source.go`, `node/sequencer/node.go` | `v7.0` (`MergeUpgrade`) |
| `752175585cfaf8526c322f9e0700ed1dad2e4bcf` | Issue #1320: Read the receipt context captured inside the accept critical section in Receipter.after so a seal does not halt replication [spec:layerx-merge/18.1320.1] | `node/sequencer/receipt.go` | `v7.0` (`MergeUpgrade`) |
| `78f814d6aee203a327dd989cb50ebd994e6b7522` | Issue #1328: Carry the receipt context on every micro-batch accept [spec:layerx-merge/18.1328.1] | `node/sequencer/microbatch.go` | `v7.0` (`MergeUpgrade`) |
| `819e3bd4999a7d11517da4f7b77a27c8e968cd67` | Issue #813: Accept a deposit-root registration on the deposit root authority's signature, not on the checkpoint submitter [spec:layerx-merge/18.813.1] | `modules/layerxcustody/keeper/deposit_root.go` | `v7.0` (`MergeUpgrade`) |
| `290ac0b03df7c03199a972dd29b5f6907f273ad5` | Wire the chain registry module into the application and its merge store upgrade [spec:layerx-merge/20.1.2] | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `dd59c88fc6a75eec061b13d51c794ba4140fa34e` | Partition block admission by chain allowance [spec:layerx-merge/20.3.2] | `node/app.go`, `node/sequencer/executor.go`, `node/sequencer/limits.go` | `v7.0` (`MergeUpgrade`) |
| `e576fb4452bca51b3db98441a05545bf9bd5ac88` | Index chain transactions, adjust each chain's base fee and pay the operator share at end of block [spec:layerx-merge/20.3.3] | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `6bd5fa59da94925203fabb708ec410dbd50e4564` | Merge the qualified chain-modules work into main | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `e4631bb1816d5e485ab8b6dcbaf34d4566eb1a6f` | Serve the chain registry precompile and gate it on the module [spec:layerx-merge/20.2.2] | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `9acf8949148a3c64d250496bd8f6a6d14975229a` | Prove the sequencer config path end to end: producer, transport, consumer [spec:layerx-merge/21.32.1] | `node/sequencer/config.go`, `node/sequencer/receiptkey.go` | `v7.0` (`MergeUpgrade`) |
| `e684aac095c61dfb9de16c4dab0b32ecc1642ca0` | Prove the perps errors path end to end: producer, transport, consumer [spec:layerx-merge/21.7.1] | `modules/perps/keeper/account.go`, `modules/perps/keeper/adl.go`, `modules/perps/keeper/book.go`, `modules/perps/keeper/matching.go`, `modules/perps/keeper/orders.go`, `modules/perps/keeper/triggers.go` | `v7.0` (`MergeUpgrade`) |
| `b4db752c64066e1c9e8f361115686155aab7032f` | Prove the sequencer wal path end to end: producer, transport, consumer [spec:layerx-merge/21.27.1] | `node/sequencer/dial.go`, `node/sequencer/failover.go`, `node/sequencer/handover.go`, `node/sequencer/standby.go` | `v7.0` (`MergeUpgrade`) |
| `992919ea83fc616cfc149511fff4d32d2263f3b9` | Prove the perps 0x1015 path end to end: producer, transport, consumer [spec:layerx-merge/21.73.1] | `modules/perps/keeper/keeper.go` | `v7.0` (`MergeUpgrade`) |
| `76d1d6c21a2f500ab162abd5679a643d5c812167` | Prove the node genesis path end to end: producer, transport, consumer [spec:layerx-merge/21.74.1] | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `401182b1f7b405c32dff76c0c2b91aa8a8e89769` | Prove the sequencer module path end to end: producer, transport, consumer [spec:layerx-merge/21.10.1] | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `65d7f4bb9443272d1b49c29427a8f9500772e8d7` | Prove the perps governance path end to end: producer, transport, consumer [spec:layerx-merge/21.25.1] | `modules/perps/keeper/msg_server.go`, `modules/perps/keeper/pause.go` | `v7.0` (`MergeUpgrade`) |
| `143889db56bf587948feec0b3cdf968deaa45f71` | Prove the retirement path end to end: producer, transport, consumer [spec:layerx-merge/21.76.1] | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `e3d6107cb9aeb4ac9c10cbaddd49f880ef44a477` | Prove the 0x1013 layerxcustody path end to end: producer, transport, consumer [spec:layerx-merge/21.89.1] | `modules/layerxcustody/keeper/msg_server.go`, `modules/layerxcustody/keeper/release.go`, `modules/layerxcustody/keeper/retire.go` | `v7.0` (`MergeUpgrade`) |
| `a72bf127bc3fcdc476036e3edb4cd177a90c51b4` | Merge the qualified chain-modules work into main | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `4b58d257c94647b8543df7c1fd97803b792c95dc` | Prove the module keeper path end to end: producer, transport, consumer [spec:layerx-merge/21.58.1] | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `fccf43f01490242f1ad22717a1f737830424ad8b` | Merge the qualified launchpad work into main | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `b77dcc007a9955f56949e9393bd075163a7ae505` | Prove the config path end to end: producer, transport, consumer [spec:layerx-merge/21.61.1] | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `9097a7fa584972ed140444cbb604dd61f4784622` | Merge the qualified rpc-api work into main | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `01611e2eca6c1b5f988b6ceb3010688bf152d07c` | Prove the go exported modules spot keeper path end to end: producer, transport, consumer [spec:layerx-merge/21.56.1] | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `2c85937758de927de3156f949e5b7835fbfe22f2` | Prove the node go api path end to end: producer, transport, consumer [spec:layerx-merge/21.48.1] | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `3d01fcb29f13fc9c46749e4a48b5a125ade80a56` | Prove the node abci path end to end: producer, transport, consumer [spec:layerx-merge/21.47.1] | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `62108d89d8ac6f9911878af0f6eaaa765c9ce738` | Prove the sequencer executor path end to end: producer, transport, consumer [spec:layerx-merge/21.19.1] | `node/sequencer/config.go`, `node/sequencer/executor.go`, `node/sequencer/fee.go`, `node/sequencer/receipt.go`, `node/sequencer/receiptkey.go`, `node/sequencer/standby.go` | `v7.0` (`MergeUpgrade`) |
| `ec287c0eac5484249c31bf5b7cd7699a4110d870` | Resolve chain operators through the EVM association for bond, fund and refund [spec:layerx-merge/20.3.5] | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `098f561a06d7ba51ff2557437ebfc38129ba4954` | Reconcile the perps ADL refusal, legacy margin vector and spot error fixtures [spec:layerx-merge/22.3.3] | `precompiles/perps/testdata/v69_vectors.json` | `v7.0` (`MergeUpgrade`) |
| `1208b0754634b208295a6731854f7fc22b9b397c` | Return the committed block's own id from LastBlockID [spec:layerx-merge/22.5.4] | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `2f080287b7c12433126fc3ddb02085724d55c50b` | Prove the sequencer receipt path end to end: producer, transport, consumer [spec:layerx-merge/21.87.1] | `node/sequencer/accepter.go`, `node/sequencer/follower.go`, `node/sequencer/log_feed.go`, `node/sequencer/receipt_source.go` | `v7.0` (`MergeUpgrade`) |
| `280826868f8aac6cf2378e88376f4987d636eec5` | Merge the qualified node-runtime work into main | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `202fc37075d5fbff509cd6ca49d807a8d82e550a` | Merge task 22.5.8 into feature/node-runtime [spec:layerx-merge/22.5.8] | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `75a1d648ba66ef5a4687997b128bc65ffaa4eb3e` | Merge commit '382e39faa2c6b0276514542cfb89f12844702e54' into task/5.0.9 | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `839aba41da4cd87aaeb320da9e88be2005d10bc8` | Read an unset Block.MaxGasWanted as unbounded at Accept and state the bound in the RPC fixtures [spec:layerx-merge/22.5.9] | `node/sequencer/limits.go` | `v7.0` (`MergeUpgrade`) |
| `50deffff99af2bad3fd9db811562fc963e2c7628` | Prove one block of root and chain transactions end to end, including restart recovery [spec:layerx-merge/20.3.4] | `node/app.go`, `node/sequencer/accept.go` | `v7.0` (`MergeUpgrade`) |
| `8f81a8be091bfb1e8338d827950dc8bca0653b44` | Cancel the attached take-profit and stop-loss pair whenever a perps position is removed [spec:layerx-merge/22.23.4] | `modules/perps/keeper/positions.go`, `modules/perps/keeper/triggers.go` | `v7.0` (`MergeUpgrade`) |
| `75256e3897053527589ab3fc26ee00cee9eec532` | Name the payment asset in the sequencer receipt and carry the fee denomination separately [spec:layerx-merge/22.23.8] | `node/sequencer/receipt.go`, `node/sequencer/standby_role.go` | `v7.0` (`MergeUpgrade`) |
| `740605d0ffa5f6e7a2896fc90399b822c80fb831` | Make the newHeads subscription tests pass against the test node after the state-root rule [spec:layerx-merge/7.10.6] | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `aab8c3d2a744c5f0d38b1ebfb18c6f1f6543305f` | Issue #1539: Return the recorded hash for identical WebSocket transaction resubmissions [spec:layerx-merge/18.1539.1] | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `4d4bfd9a7bc9638e7416305e7a2f1138d0f95104` | Rebuild the sequencer receipt source from the write-ahead log replay on node start [spec:layerx-merge/22.95.2] | `node/sequencer/accepter.go`, `node/sequencer/node_start.go`, `node/sequencer/receipt_source.go`, `node/sequencer/replay.go` | `v7.0` (`MergeUpgrade`) |
| `6ea5ee7e8d9ec4aea789da41df5c25d464fa4bf2` | Issue #1567: Populate Autobahn committed header roots before publishing newHeads [spec:layerx-merge/18.1567.1] | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `d429fa28bab336c64e4c83c2ea638a44473a3e41` | Serve receipts of sends committed before a restart from the rebuilt receipt source [spec:layerx-merge/22.95.13] | `node/sequencer/accepter.go`, `node/sequencer/node_start.go`, `node/sequencer/receipt_source.go`, `node/sequencer/wal/wal.go` | `v7.0` (`MergeUpgrade`) |
| `607875f3576776cb8615a562d7a1a20e747a85ac` | Repair the inventory sweep target node/sequencer [spec:layerx-merge/22.99.144] | `node/sequencer/limits.go` | `v7.0` (`MergeUpgrade`) |
| `165efc528ae0bf76b4aa544a6ba7b7ade592eb06` | Repair audit record 22.30.59 findings in modules/layerxcustody/keeper: interface-mismatch, restart-recovery [spec:layerx-merge/22.31.338] | `modules/layerxcustody/keeper/genesis.go`, `modules/layerxcustody/keeper/keeper.go`, `modules/layerxcustody/keeper/release.go`, `modules/layerxcustody/keeper/repair_338.kvx`, `modules/layerxcustody/keeper/retire.go`, `modules/layerxcustody/keeper/retirement_genesis.go` | `v7.0` (`MergeUpgrade`) |
| `7aa2cc6857031b6a9a77667a34a3d35a22ec38f1` | Repair audit record 22.30.14 findings in modules/perps/keeper: interface-mismatch, restart-recovery, runtime-risk [spec:layerx-merge/22.31.14] | `modules/perps/keeper/account.go`, `modules/perps/keeper/adl.go`, `modules/perps/keeper/adl_recovery.go`, `modules/perps/keeper/fees.go`, `modules/perps/keeper/funding.go`, `modules/perps/keeper/genesis.go`, `modules/perps/keeper/insurance.go`, `modules/perps/keeper/keeper.go`, `modules/perps/keeper/liquidation.go`, `modules/perps/keeper/margin.go`, `modules/perps/keeper/mark.go`, `modules/perps/keeper/matching.go`, `modules/perps/keeper/msg_server.go`, `modules/perps/keeper/msg_server_market.go`, `modules/perps/keeper/openinterest.go`, `modules/perps/keeper/orders.go`, `modules/perps/keeper/positions.go`, `modules/perps/keeper/quotes.go`, `modules/perps/keeper/repair_14.kvx`, `modules/perps/keeper/triggers.go`, `precompiles/perps/positions.go` | `v7.0` (`MergeUpgrade`) |
| `881d9c90ebe6817e56b41a9c27d39719b13b6f38` | Repair audit record 22.30.46 findings in consensus/internal/state: restart-recovery [spec:layerx-merge/22.31.89] | `node/sequencer/consensus_source.go` | `v7.0` (`MergeUpgrade`) |
| `b408a0bf7297f2ddbefb8a495b131cb254ba822c` | Switch the hosted kernel to programs guest ABI 5 at the merge upgrade and keep the choice across restarts [spec:layerx-merge/5.99.1] | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `83a611ecefe405d19fb6c23e10c35c5bac941406` | Repair audit record 22.30.126 findings in programs/crates/layerx-programs-market/src: interface-mismatch, security [spec:layerx-merge/22.31.260] | `programs/crates/layerx-programs-market/src/arbitration.rs`, `programs/crates/layerx-programs-market/src/lib.rs`, `programs/crates/layerx-programs-market/src/repair_22_31_260.kvx`, `programs/crates/layerx-programs-market/src/settle.rs` | `v7.0` (`MergeUpgrade`) |
| `1ee93282cbe5d59b98b34bebd8a33f7ffdecf5b6` | Repair audit record 22.30.50 findings in node: interface-mismatch, restart-recovery, runtime-risk, security [spec:layerx-merge/22.31.102] | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `26d76c12362b7507da8e1635453268de152cfd6a` | Repair audit record 22.30.51 findings in node/sequencer: interface-mismatch, restart-recovery, runtime-risk, security [spec:layerx-merge/22.31.103] | `node/sequencer/config.go`, `node/sequencer/consensus_source.go`, `node/sequencer/dial.go`, `node/sequencer/handover.go`, `node/sequencer/microbatch.go`, `node/sequencer/node.go`, `node/sequencer/node_start.go`, `node/sequencer/receipt_source.go`, `node/sequencer/repair_103.kvx`, `node/sequencer/replay.go`, `node/sequencer/standby.go`, `node/sequencer/standby_role.go` | `v7.0` (`MergeUpgrade`) |
| `e1b27a05d11ac6890ccf5f3f14614ec1be58a0d8` | Repair audit record 22.30.14 medium findings in modules/perps/keeper (part 1 of 1): config-drift, security, test-gap [spec:layerx-merge/22.33.363] | `modules/perps/keeper/endblock.go`, `modules/perps/keeper/msg_server_pause.go`, `modules/perps/keeper/repair_m363.kvx` | `v7.0` (`MergeUpgrade`) |
| `52c5b590658c80dfc7116c9e89a47dc6fe2f2672` | Repair audit record 22.30.162 medium findings in sdk/x/evidence/keeper (part 1 of 1): runtime-risk, wiring-gap [spec:layerx-merge/22.33.603] | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `c5b5060fdb3b4e0d6e22bce073b443b6715f6c35` | Repair audit record 22.30.126 findings in programs/crates/layerx-programs-runtime/src: interface-mismatch, restart-recovery [spec:layerx-merge/22.31.259] | `programs/crates/layerx-programs-market/src/dispute.rs` | `v7.0` (`MergeUpgrade`) |
| `fe12e73e857d44b1a98c3cf16c02b3a692f858e7` | Repair audit record 22.30.51 medium findings in node/sequencer (part 1 of 2): runtime-risk, test-gap, wiring-gap [spec:layerx-merge/22.33.1144] | `node/sequencer/accept.go`, `node/sequencer/accepter.go`, `node/sequencer/driver.go`, `node/sequencer/executor.go`, `node/sequencer/lease.go`, `node/sequencer/lease_renew.go`, `node/sequencer/microbatch.go`, `node/sequencer/node.go`, `node/sequencer/node_start.go`, `node/sequencer/node_stop.go`, `node/sequencer/nonce_admission.go`, `node/sequencer/nonce_queue.go`, `node/sequencer/receipt.go`, `node/sequencer/receipt_source.go`, `node/sequencer/receiptkey.go`, `node/sequencer/repair_m1144.kvx`, `node/sequencer/replay.go`, `node/sequencer/standby.go`, `node/sequencer/standby_role.go`, `node/sequencer/wal/wal.go` | `v7.0` (`MergeUpgrade`) |
| `f56cd1339ecbdfc0e34b1a5ebf4dc614940ea614` | Repair audit record 22.30.57 medium findings in interchain/modules/apps/27-interchain-accounts (part 1 of 1): restart-recovery, wiring-gap [spec:layerx-merge/22.33.1202] | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `311c57868f6517011f3c06222dea95784cf9951a` | Repair audit record 22.30.51 medium findings in node/sequencer (part 2 of 2): runtime-risk [spec:layerx-merge/22.33.1145] | `node/sequencer/receiptkey.go`, `node/sequencer/repair_m1145.kvx` | `v7.0` (`MergeUpgrade`) |
| `90f4829d366a509c270bc0d18085fcece25b5fa8` | Repair audit record 22.30.59 medium findings in modules/layerxcustody/keeper (part 1 of 1): runtime-risk, test-gap [spec:layerx-merge/22.33.1228] | `modules/layerxcustody/keeper/keeper.go`, `modules/layerxcustody/keeper/repair_m1228.kvx` | `v7.0` (`MergeUpgrade`) |
| `b8dd8df90baa744439fe410a9243051b61784e5c` | Repair audit record 22.30.66 medium findings in wasm/x/wasm (part 1 of 1): config-drift, doc-drift, runtime-risk [spec:layerx-merge/22.33.1289] | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `da61cc5f62a00ab375fcb87768288e82a4bd4ba2` | Repair audit record 22.30.8 medium findings in modules/layerxledger/keeper (part 1 of 1): restart-recovery, test-gap, wiring-gap [spec:layerx-merge/22.33.1423] | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `9ae50fb7f181a446e992fb85e6fc1c3b6102c27c` | Merge the qualified ledger-auth-pay work into main | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `99a76e154286fefe6da0c854c0a7f236d162f0cd` | Reconcile the consensus AppHash with the application commit hash across a node restart [spec:layerx-merge/22.31.475] | `node/sequencer/driver.go`, `node/sequencer/executor.go`, `node/sequencer/node_start.go` | `v7.0` (`MergeUpgrade`) |
| `a929232089889e2357e5b5b336c2598d75ec464c` | Merge task 22.33.1425 into feature/ledger-auth-pay [spec:layerx-merge/22.33.1425] | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `2b5d674413f8e816349b6f383c03dbe4d87e9c72` | Repair audit record 22.30.50 medium findings in node (part 1 of 4): runtime-risk, test-gap [spec:layerx-merge/22.33.1140] | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `2c3d306db894d0a7f3e7e6d9154a730efe43e455` | Repair the accepter receipt tests of the sequencer that time out in the node-build check [spec:layerx-merge/22.31.597] | `node/sequencer/receipt.go`, `node/sequencer/repair_597.kvx` | `v7.0` (`MergeUpgrade`) |
| `a2bf776a9fbdd7265c4a1dfcc89cb2fe8e755d13` | Lock every intentional copy of a generated or shared file with a sync test and remove the copies that are not intentional [spec:layerx-merge/22.39.62] | `precompiles/perps/abi/abi.json` | `v7.0` (`MergeUpgrade`) |
| `bc89ead834ccbc9670b2b23aaa16894524057843` | Make the node package tests pass again by repairing the four upgrade activation tests and the standby commit release hang [spec:layerx-merge/22.39.40] | `node/sequencer/driver.go` | `v7.0` (`MergeUpgrade`) |
| `045cbd6efe2440c316e3bca8ef136c2ddd2411f5` | Repair the six node sequencer test failures the standby commit hang masked [spec:layerx-merge/22.39.86] | `node/sequencer/config.go`, `node/sequencer/driver.go`, `node/sequencer/lease_renew.go`, `node/sequencer/metrics.go`, `node/sequencer/repair_22_39_86.kvx` | `v7.0` (`MergeUpgrade`) |

## Historical v6.12.0 ledger

The earlier plan is `V6120Upgrade` in
[`node/upgrades_v6120.go`](../../node/upgrades_v6120.go). These rows preserve
its coordinated upgrade record.

| Full commit hash | State-machine change | Non-test consensus paths | Coordinated upgrade |
| --- | --- | --- | --- |
| `0b48316e2eea28256461f1a1a99c753b2666f98c` | Whole-position closes pass the exact position base size to `sizeOrder` and simulate the same sizing in quotes, preventing an unaligned mark from leaving one lot open. | `modules/perps/keeper/orders.go`, `modules/perps/keeper/quotes.go`, `precompiles/perps/positions.go` | v6.12.0 through `V6120Upgrade`, which first adds the perps store and initializes its state. |
| `cdd97253c4219a77363a62377c36d212cda5a7ba` | Refuse a plain usage commit for a fraud-provable compute lease; its claim must use sandbox billing with challengeable profile and billing rows. | `programs/crates/layerx-programs-market/src/lib.rs`, `programs/crates/layerx-programs-market/src/settle.rs` | v6.12.0 coordinated binary upgrade through `V6120Upgrade`; all nodes use the same compute-market rules after restart. |
| `97d256444123c41a8db5c9b3d0c4ef2d05df4e0f` | Anchor forced exits to the first live exit checkpoint and refuse ordinary withdrawals proved in later batches, including after the emergency clears. | `modules/layerxcustody/keeper/claims.go`, `modules/layerxcustody/types/errors.go` | v6.12.0 coordinated binary upgrade through `V6120Upgrade`; all nodes switch custody claim and withdrawal rules together. |
| `68c8887a9520542d2878da3d6a44920ddd71b4b1` | Persist the exit checkpoint under its own key, rebuild it on cancellation and genesis, and refuse deposits while a forced exit is live. | `modules/layerxcustody/keeper/claims.go`, `modules/layerxcustody/keeper/deposit.go`, `modules/layerxcustody/keeper/genesis.go`, `modules/layerxcustody/types/errors.go`, `modules/layerxcustody/types/keys.go` | v6.12.0 coordinated binary upgrade through `V6120Upgrade`; all nodes switch checkpoint storage and deposit refusal rules together. |

For each additional consensus-changing commit, add a row to the v7.0.0 ledger
in a subsequent commit on the same pull request, including its upgrade plan or halt height. A commit
cannot contain its own final hash. The release workflow checks every non-merge
commit in the pull request's base-to-head range against the table **at the head
revision**, even when the table update comes later than the consensus change.

Run the same check locally from the repository root with
`python3 tools/ci/consensus_commit_gate.py <base> <head>`.
The gate covers the prefixes in `CONSENSUS_PATHS` in that script, excludes
`_test.go` files, and prints every unlisted commit and its consensus paths.

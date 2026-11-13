# Consensus-changing commits for v7.0.0

The next release is **v7.0.0**, carrying plan **v7.0** (`MergeUpgrade`).
The release identity belongs in
[`tools/release/version.json`](../../tools/release/version.json). This ledger
tracks consensus-changing commits by their full Git hash, state-machine change,
non-test consensus paths and coordinated upgrade.

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
| `v7.0-anchor-store-delete` (`AnchorStoreDeleteUpgrade`) | `app.runAnchorStoreDeleteUpgrade` | Requires a positive applied merge height strictly below the current height, then runs module migrations. The store loader deletes `layerxanchor`; all anchor methods then refuse. |

The v7.0.0 inventory uses
`68c8887a9520542d2878da3d6a44920ddd71b4b1` (exclusive base). Consensus paths are identified by `tools/ci/consensus_ledger_rows.py`.

| Full commit hash | State-machine change | Non-test consensus paths | Coordinated upgrade |
| --- | --- | --- | --- |
| `12fbbeb80389f9021d791db00d3f32101ea51668` | Read the sequenced flag from chain state and the applied merge-plan height, so node-local configuration cannot change the accepted block format. | `node/app.go`, `node/sequencer/consensus_source.go` | v7.0.0 coordinated binary upgrade through `MergeUpgrade` (`v7.0`); sequenced blocks activate as recorded in chain state. |
| `c3520a88056e6fb7b65daf706ae05f4b8fabbd07` | Withhold merge precompile addresses until the governed activation plan applies. | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `fbaac34c7ce0a56679d6ad2d3617b62a4dda36ef` | Add EventRetiredRelease to the custody events proto and regenerate it | `modules/layerxcustody/types/events.pb.go` | `v7.0` (`MergeUpgrade`) |
| `134f076b3457af8a90f22e08e96dfd65fc0cbc5e` | Add the custody merge plan name and the ErrModuleRetired code 29 | `modules/layerxcustody/types/errors.go`, `modules/layerxcustody/types/retire.go` | `v7.0` (`MergeUpgrade`) |
| `813a8b2de31f54702151e1f59893b3743bb9156d` | Add Keeper.Retired reading the v7.0 plan from the bound upgrade reader | `modules/layerxcustody/keeper/retire.go` | `v7.0` (`MergeUpgrade`) |
| `9ccb46b0d0ae9f60ad1dee2cbde49bd4931c4d3b` | Refuse withdrawals, forced exits and claim cancellation from the merge plan height | `modules/layerxcustody/keeper/claims.go` | `v7.0` (`MergeUpgrade`) |
| `0b3872e6ea0f8e57e73baec07890a71f6cbb39e8` | Refuse custody parameter, asset and emergency changes from the merge plan height | `modules/layerxcustody/keeper/keeper.go` | `v7.0` (`MergeUpgrade`) |
| `7f82452585611af50988d65a45c9a0186168ab80` | Add ReleaseRetired paying stranded custody balances to the authority once | `modules/layerxcustody/keeper/release.go` | `v7.0` (`MergeUpgrade`) |
| `2ee6f997b4477c0950bcda814a064470c452ac9a` | Refuse authority checkpoint registration from the merge plan height | `modules/layerxcustody/keeper/anchor.go` | `v7.0` (`MergeUpgrade`) |
| `07894ad30176786beafd1e17954434aaf34639d0` | Refuse deposit-root registration from the merge plan height | `modules/layerxcustody/keeper/deposit_root.go` | `v7.0` (`MergeUpgrade`) |
| `04080421eb4b1e7ef21e09c86035176823cd392a` | Declare PerpsAddress as the 0x1015 literal | `precompiles/perps/perps.go` | `v7.0` (`MergeUpgrade`) |
| `3fa3a5193a8e9628786d530077b926845fe027a9` | Add the keeper SetPaused path under the gov authority | `modules/perps/keeper/pause.go` | `v7.0` (`MergeUpgrade`) |
| `fbb6ad6fd272a5a47d83f1e946c5dd8ce2e3a304` | Export the perps keeper genesis validation that InitGenesis runs | `modules/perps/keeper/genesis.go` | `v7.0` (`MergeUpgrade`) |
| `3b328307e82e2c227cf325b5fd6ad2a097996564` | Bound ADL per block by Params.AdlPositionsPerBlock | `modules/perps/keeper/adl.go` | `v7.0` (`MergeUpgrade`) |
| `f14a08f183026c8af9765f278eddf9b4b1d9f2e5` | Serve MsgUpdateParams through the perps msg server with a params event | `modules/perps/keeper/msg_server.go` | `v7.0` (`MergeUpgrade`) |
| `5e26b7a5d97cf7bd61c89a846d0314d5e7ffa947` | Make Withdrawable read Params.WithdrawFloorPpm instead of a fixed divisor | `modules/perps/keeper/account.go` | `v7.0` (`MergeUpgrade`) |
| `032849a17260c377c8da4101f2250384d394e11e` | Serve MsgSetPaused and emit the pause event | `modules/perps/keeper/msg_server_pause.go` | `v7.0` (`MergeUpgrade`) |
| `36c90b2b694ff7920e284c6406857943d3e6e10d` | Add App seams for the sequencer source, the sequencer RPC options and a commit listener | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `f504f41db25d0c209dcb92db392fd07279852ba5` | Refuse all nine custody Msgs from the merge plan height before decoding | `modules/layerxcustody/keeper/msg_server.go` | `v7.0` (`MergeUpgrade`) |
| `23452f776a13e46566df85ce7691f4149f6c64ee` | Refuse lowering MaxOpenOrdersPerAccount below an account's open orders | `modules/perps/keeper/msg_server.go` | `v7.0` (`MergeUpgrade`) |
| `0c76247257c1bca8b4c7c6c79a04af103c0716bd` | Report the fencing lease from WAL replay | `node/sequencer/wal/wal.go` | `v7.0` (`MergeUpgrade`) |
| `4aa2bd0da5779eef74ff67dd2d580cfc71921489` | Refresh the precompile set inside executor Open | `node/sequencer/executor.go` | `v7.0` (`MergeUpgrade`) |
| `e4ba17a6241fed8e39fbcbcd652b8d3e5eb582ed` | Keep the sealed batch when ConsensusSource.Seal fails after the executor sealed | `node/sequencer/consensus_source.go` | `v7.0` (`MergeUpgrade`) |
| `e645edd46e17d1e81f99326a860e9c8240eee1d9` | Add the receipt forward URL and the standby role keys to the sequencer config | `node/sequencer/config.go` | `v7.0` (`MergeUpgrade`) |
| `7ebe6f9c0d1beafe6654a97ae599ff197f46111a` | Report the standby replication WAL sync time to the sequencer driver | `node/sequencer/standby.go` | `v7.0` (`MergeUpgrade`) |
| `36bd959682910d301e717d6209ae1b3cadd2bdd0` | Place each accepted transaction with SetTxPosition | `node/sequencer/accept.go` | `v7.0` (`MergeUpgrade`) |
| `04b2d6a2f0927ab477bc5cd2af69f5ef89aca79e` | Add the sequencer runtime metrics instruments | `node/sequencer/metrics.go` | `v7.0` (`MergeUpgrade`) |
| `c74f5407e47d8576ed76a05d63bd8285c2f3cbfe` | Restore the standby lease epoch on restart | `node/sequencer/standby.go` | `v7.0` (`MergeUpgrade`) |
| `261382cd62fce82f40e480023fb8e86f1feea26d` | Node integration of native execution, module state and sequencing. | `node/sequencer/standby.go` | `v7.0` (`MergeUpgrade`) |
| `d2a8adc582e4888cad9fd9c10c69dd80e60e2d58` | Report the last logged ACCEPT from WAL replay | `node/sequencer/wal/wal.go` | `v7.0` (`MergeUpgrade`) |
| `88781fdd74200371387559f1485a91f42b7b4c4c` | Add WAL retention, nonce queue bounds, lease duration and the start flags to the sequencer config | `node/sequencer/config.go` | `v7.0` (`MergeUpgrade`) |
| `b6978b427618a8e7f4796efcaf982b7e4e4ab7f0` | Add the bounded TCP dialer for the standby replication link | `node/sequencer/dial.go` | `v7.0` (`MergeUpgrade`) |
| `f3ef8c257053361e5b73fba93f7a16140e47d426` | Refuse a transaction that would overflow the block gas or byte limits at Accept | `node/sequencer/accept.go`, `node/sequencer/limits.go` | `v7.0` (`MergeUpgrade`) |
| `545f44fa65d258a8d650fe959a136cc3fbfc5269` | Place micro-batch transactions with SetTxPosition | `node/sequencer/microbatch.go` | `v7.0` (`MergeUpgrade`) |
| `5df1a45f6361902f24585eade8c3d604479a8b16` | Refresh the receipt key pins from the keeper at each Open | `node/sequencer/receiptkey.go` | `v7.0` (`MergeUpgrade`) |
| `5596ceb585229c5dc2415f1969b757940665cde1` | Add the receipt source that serves accepted sequencer receipts by hash | `node/sequencer/receipt_source.go` | `v7.0` (`MergeUpgrade`) |
| `b5ad82a45a3f07be7341253c72304847f342b0b4` | Wire the order and proposer checks into ProcessProposal and halt on a failed batch | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `2e843a8ebb3dbe6bb3d2b292704eb4fcc5b410cc` | Use the module seqHash in the executor | `node/sequencer/executor.go` | `v7.0` (`MergeUpgrade`) |
| `010afd7ff1d9c1fdd14b7ae52f7c18f86a4f424c` | Fence the old primary through the standby lease and the ack epoch | `node/sequencer/standby.go` | `v7.0` (`MergeUpgrade`) |
| `d9d69e1ea660abf375176484d1f963f462d7584f` | Admit future-nonce transactions through the nonce queue | `node/sequencer/nonce_admission.go` | `v7.0` (`MergeUpgrade`) |
| `6d66eaf9f2e3c7ad65ec978ed91ec97a048c6ba6` | Forward sequencer receipt reads from a non-sequencer node | `node/sequencer/follower.go` | `v7.0` (`MergeUpgrade`) |
| `93ef54d28c5a44479eab554973f19aa246a60b21` | Record the seal of each block in the WAL before the sealed block is returned | `node/sequencer/consensus_source.go` | `v7.0` (`MergeUpgrade`) |
| `91b92c6d584701322bc1b86b305a61cc011aa266` | Add the accepter that receipts a send only after WAL write and standby ack | `node/sequencer/accepter.go` | `v7.0` (`MergeUpgrade`) |
| `17af9fe35dfc40fe614d82696253638c35922bac` | Publish receipted logs to the RPC receipt log feed | `node/sequencer/log_feed.go` | `v7.0` (`MergeUpgrade`) |
| `9b55e2c937093469fa649a658f35bd3c0ee31a26` | Take a fencing lease when the primary sequencer starts | `node/sequencer/lease.go` | `v7.0` (`MergeUpgrade`) |
| `d3031ca2faabe404564fd373e6bec975c0e064c9` | Add the block driver that commits, prunes and opens the next block | `node/sequencer/driver.go` | `v7.0` (`MergeUpgrade`) |
| `c1b44508da3fd690c0e32f10cbd927b8855a39d1` | Add the production WAL replayer that re-executes accepted transactions | `node/sequencer/replay.go` | `v7.0` (`MergeUpgrade`) |
| `b3de702c1d3d7e45a63a534148f643acf49e9128` | Renew the sequencer lease before it expires | `node/sequencer/lease_renew.go` | `v7.0` (`MergeUpgrade`) |
| `898e74278becbe6f1b69f6760f76f952832c02ec` | Refuse custody deposits from the merge plan height | `modules/layerxcustody/keeper/deposit.go` | `v7.0` (`MergeUpgrade`) |
| `8fe024ea01787076e158732ca52043598cb51cfa` | Serve MsgSetMarket with pricefeed listing and identity checks | `modules/perps/keeper/msg_server_market.go` | `v7.0` (`MergeUpgrade`) |
| `1ffb1822344dc9435faff82c99e58a6ff3fbeb70` | Start a standby from the sequencer config role | `node/sequencer/standby_role.go` | `v7.0` (`MergeUpgrade`) |
| `554fc97e404901cbc79d26fb426f70fe1cb16f15` | Fail over to the standby after an unplanned primary stop | `node/sequencer/failover.go` | `v7.0` (`MergeUpgrade`) |
| `90e07859ca34ae2c2b1d658f5b6ac897ae9cdef3` | Build the custom precompiles through GetCustomPrecompilesE | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `24fb4183889ee96c7090a2b911d3b8d91a0a2e18` | Node integration of native execution, module state and sequencing. | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `238dc2034f609037cb74bf5586e8223ba8610aff` | Carry the committed app hash and last block id in the executor Open header | `node/app.go`, `node/sequencer/executor.go` | `v7.0` (`MergeUpgrade`) |
| `e856ff1c9cd17a51b7908dae0cf5e3e6cadcb025` | Retire the Layers seal path in favour of SealBatch | `node/sequencer/layers.go`, `node/sequencer/seal.go` | `v7.0` (`MergeUpgrade`) |
| `0dc739534641cc9a8450e2ac0bbf71ce2644b79d` | Fence the executor so a superseded primary writes nothing | `node/sequencer/accept.go`, `node/sequencer/executor.go`, `node/sequencer/fee.go`, `node/sequencer/microbatch.go` | `v7.0` (`MergeUpgrade`) |
| `33c2f9a35d72a18955f753ce13c4475f5b3f5dd4` | Fence the primary on ErrFenced and build the promoted standby's consensus source | `node/sequencer/consensus_source.go` | `v7.0` (`MergeUpgrade`) |
| `3e99b79efc04a703820a84d23cefe5e55407330d` | Hand over the open block between hosts with a lease | `node/sequencer/handover.go` | `v7.0` (`MergeUpgrade`) |
| `bd41925be1f2d09194bb32e90eccffad4a65189a` | Assemble the sequencer runtime from config | `node/sequencer/node.go` | `v7.0` (`MergeUpgrade`) |
| `2c65d2c3c6901b1a016fb9e3679cb65d9a186b59` | Start the sequencer node: load the key, replay the WAL and install it on the App | `node/sequencer/node_start.go` | `v7.0` (`MergeUpgrade`) |
| `43cfe1647abe81017447f7043ce416983a915f4a` | Stop the sequencer node cleanly | `node/sequencer/node_stop.go` | `v7.0` (`MergeUpgrade`) |
| `1e3c9243f68df5ba3f6598b55bb52aef4178ee67` | Register the ledger, auth and agent modules and the shared host in the App | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `9c04cbe1a794258f4ebca3c8e242a35130c565ee` | Freeze the 0x1015 order and market view outputs in a golden | `precompiles/perps/testdata/views_market_golden.json` | `v7.0` (`MergeUpgrade`) |
| `9b8968f3025f187529497f7b5759ed4d91d155ee` | Freeze the 0x1015 quote outputs in a golden | `precompiles/perps/testdata/views_quotes_golden.json` | `v7.0` (`MergeUpgrade`) |
| `eed67ac9c010eb1758bb1e4852275df88297b642` | Freeze the 0x1015 account and position view outputs in a golden | `precompiles/perps/testdata/views_account_golden.json` | `v7.0` (`MergeUpgrade`) |
| `9468751909c5b2219f11860190805ba27ca430c0` | Freeze the 0x1015 insurance and ADL view outputs in a golden | `precompiles/perps/testdata/views_insurance_golden.json` | `v7.0` (`MergeUpgrade`) |
| `c1902ec34ef5e6e2551793d981bc14db264962ab` | Freeze the 0x1015 funding and fee view outputs in a golden | `precompiles/perps/testdata/views_funding_golden.json` | `v7.0` (`MergeUpgrade`) |
| `eb832eb4d53709d42d9c43ef03845f6f29ccdb30` | Run the layerxauth revocation decorator in the ante chain | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `0fb1aa46ccae0857d0204861ea0836d1d6c1262d` | Pass the sequencer RPC options and wire the merged keepers, the served spot set | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `2888be80def09b4b83a63d930bd0035df7673ab4` | Build the layerxanchor module with the upgrade reader and route the gated msg server | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `0ab6b99623d5dadcef4d72a397ed6167cfff9a9d` | Adopt the sealed block in FinalizeBlock and store the decided votes | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `27f40329f2eb8a4a0fe32dfd764e014f56c0dde5` | Node integration of native execution, module state and sequencing. | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `4b596e6ea8314343f8bb33876e333f90bdbbb0b7` | Drop the unused distrKeeper from the tokenfactory keeper | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `0b66c4e6012786e16a978a5e4980e69c37a73b4d` | Node integration of native execution, module state and sequencing. | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `f288cd755ab108e205f0daff1dc47f4593dee316` | Node integration of native execution, module state and sequencing. | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `d34d5ea8db5c87f8f4335db03fcbd0f94ee647f5` | Node integration of native execution, module state and sequencing. | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `ae2beed6383a968df05f308a45f11d01d654258e` | Delete the anchor store and module at a second plan after the merge | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `39c8bb7152e9675fdd1ed5723e87700ab2f3ca35` | Keep the anchor store deletion upgrade compiling after the store key constant retires | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `5b2d955eee0b63b4c0919b509fb30a8a6bc1f2e2` | Keep retired anchor names only at deletion boundaries | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `2db1f1e3fe2868eae8cef51a3f14d380d26ca09d` | Preserve the accepted sequence frontier across retention pruning | `node/sequencer/handover.go`, `node/sequencer/standby.go` | `v7.0` (`MergeUpgrade`) |
| `85b415acd9943d7208cf8ae1a3505fd781e40dcd` | Let the standby cross commits and serve the sequencer base fee | `node/app.go`, `node/sequencer/replay.go`, `node/sequencer/standby.go`, `node/sequencer/standby_role.go` | `v7.0` (`MergeUpgrade`) |
| `e4b4233271dfba196ef152892ea25524297ab7d2` | Node integration of native execution, module state and sequencing. | `node/sequencer/standby.go` | `v7.0` (`MergeUpgrade`) |
| `5885f13e244a95dcb0e3333dc83f1042412a2f86` | Seed the custody forced-exit checkpoint key on in-place upgrade | `modules/layerxcustody/keeper/migrations.go` | `v7.0` (`MergeUpgrade`) |
| `65516073c40d6274ea32b29f14fa14c2902e7cbc` | Check the sequence batch range, txRoot and lastSeqHash against the block's transactions in FinalizeBlock | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `e28cb9af076e1920c7331fdca40c0dfeeb65d3c4` | Refuse Bonded and Attested market offers and leases until they can be challenged | `programs/crates/layerx-programs-market/src/lib.rs`, `programs/crates/layerx-programs-market/src/settle.rs` | `v7.0` (`MergeUpgrade`) |
| `58263ea9f0f554052061bef8e194f7985e0b54c3` | Size every perps close in base so a partial close fills the quoted lot | `modules/perps/keeper/quotes.go`, `precompiles/perps/positions.go` | `v7.0` (`MergeUpgrade`) |
| `43af3141f0f51068f8ccb3272dd44935f287c068` | Document the forced-exit checkpoint and freeze in the custody docs | `modules/layerxcustody/keeper/claims.go` | `v7.0` (`MergeUpgrade`) |
| `327ec63c0d8eb0bf60e8ec0973b8731d1fa7eacf` | Keep a keyed forced-exit total per account and asset instead of scanning every claim | `modules/layerxcustody/keeper/claims.go`, `modules/layerxcustody/types/keys.go` | `v7.0` (`MergeUpgrade`) |
| `59417423e377ff9b625e524b5e6b0dd3e48cff5e` | Make sealed-block adoption in FinalizeBlock commit the state follower re-execution computes | `node/app.go`, `node/sequencer/seal.go` | `v7.0` (`MergeUpgrade`) |
| `11f153ccffd44c540d40a577d1929eb6f0769517` | Make the standby acknowledge every replicated group of the committed-fixture recording through openAccount | `node/app.go`, `node/sequencer/seal.go`, `node/sequencer/standby.go` | `v7.0` (`MergeUpgrade`) |
| `f93d74e1caab5ed82115aabdd0ba313ca503cab9` | Surface the standby's refusal reason and make it acknowledge the recorder's openAccount groups | `node/sequencer/standby.go` | `v7.0` (`MergeUpgrade`) |
| `07d11e56dd86d28a7c90ac37acbb8f5492f43823` | Standby replays a ledger openAccount after register with the primary's receipt | `node/sequencer/standby_role.go` | `v7.0` (`MergeUpgrade`) |
| `d6c2e0ee24f32185f8d7dc13bc02558a9d1c96fd` | Answer a commit-pending OPEN well inside the primary's replication deadline and let the primary retry it | `node/sequencer/standby.go` | `v7.0` (`MergeUpgrade`) |
| `c6973b8da83841e75d3471740b5ecfc149b2409d` | Wait for the consensus commit without holding the standby mutex that guards lease takeover | `node/sequencer/standby.go` | `v7.0` (`MergeUpgrade`) |
| `3ed1f47aa7d69b1234eea438b1fcff63be4dfedc` | Commit the standby's replayed layer through the sealed-layer path so prior-block kernel reads rebuild the primary's receipt | `node/app.go`, `node/sequencer/standby_role.go` | `v7.0` (`MergeUpgrade`) |
| `4f6627f11677c5ccfddf435f3d1cdc04bf6df154` | Overlap the standby sync with re-execution and the primary sync with the exchange | `node/sequencer/standby.go` | `v7.0` (`MergeUpgrade`) |
| `71835ba46447cf789c68c84db720c85997e7bcc3` | Let the sequencer replication log carry the seal of an empty layer | `node/sequencer/wal/wal.go` | `v7.0` (`MergeUpgrade`) |
| `536f72a9469316d757941e46c5c88af35018ab91` | Export and import the shared host store in genesis before every host-writing module | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `c7fdb284e57582e8b2dc7323bcb51211488a18ba` | A failing commit listener fails Commit instead of being recovered and logged | `node/app.go`, `node/sequencer/driver.go`, `node/sequencer/node_start.go` | `v7.0` (`MergeUpgrade`) |
| `ab66e8cc9862b34b10f840c51b41e49fae5af751` | Node integration of native execution, module state and sequencing. | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `1503627359a03044ac5e3ef39e1080bff94043e8` | Check buf generate reproduces the checked-in Go for api and sdk/proto | `modules/layerxcustody/types/genesis.pb.go`, `modules/layerxcustody/types/params.pb.go`, `modules/layerxcustody/types/proposal.pb.go`, `modules/layerxcustody/types/state.pb.go`, `modules/layerxcustody/types/tx.pb.go` | `v7.0` (`MergeUpgrade`) |
| `6805e86162937254115f0a8dcacc10e69bc8f852` | Release the next OPEN without waiting inside ABCI Commit for the standby's commit of the same height | `node/sequencer/driver.go`, `node/sequencer/node_start.go`, `node/sequencer/standby.go` | `v7.0` (`MergeUpgrade`) |
| `450177d9e3847c10e24a2a9bdc3969d3ff8b6fe7` | Send the EVM RPC start signal when FinalizeBlocker adopts a sealed block | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `13684186846693a1f94fde12f6e51baf1df62cc5` | Keep the anchor keeper and its store mounted in the binary that applies the v7.0 plan | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `8fe2607dd5e923948e4cd79031d63c8a09e28108` | Retire the 0x1012 receipt verifiers and the custody inclusion and exit-balance paths | `modules/layerxcustody/keeper/claims.go`, `modules/layerxcustody/types/ids.go` | `v7.0` (`MergeUpgrade`) |
| `30b042b4f681cb74f2cd4437a8e84e79ecfba2a0` | Carry the transient receipt through the accept critical section so a seal does not halt replication | `node/sequencer/accept.go`, `node/sequencer/accepter.go`, `node/sequencer/executor.go` | `v7.0` (`MergeUpgrade`) |
| `93d2d11b1a4d65951cc2af24958fb03da0561139` | Acknowledge replicated groups on WAL durability so a pending commit cannot time the primary out | `node/sequencer/standby.go` | `v7.0` (`MergeUpgrade`) |
| `a97ab62aafde875f883819dcfbff65e826947afa` | Run the sequencer primary without a standby when no standby address is configured | `node/sequencer/node.go`, `node/sequencer/standby.go` | `v7.0` (`MergeUpgrade`) |
| `ca6a3aff298a8821e91f45fbef3d0f3681c1d7b9` | Node integration of native execution, module state and sequencing. | `node/sequencer/standby.go` | `v7.0` (`MergeUpgrade`) |
| `92ea1d44f61a7f14eea2fa0af105c839b5de69ea` | Log recovered RPC handler panics with their stack and prove the standalone primary receipts a signed transfer through the send path | `node/sequencer/accepter.go`, `node/sequencer/node_start.go`, `node/sequencer/standby.go` | `v7.0` (`MergeUpgrade`) |
| `99bef8286afb048a84b967e1592c31457c6d9842` | Wait for the next open block on the RPC send path instead of panicking | `node/app.go`, `node/sequencer/executor.go` | `v7.0` (`MergeUpgrade`) |
| `a5b872224c9d1dea51a2078283862521c1a649a4` | Record the receipt's logs at the receipt's height when the layer seals after Accept | `node/sequencer/accepter.go` | `v7.0` (`MergeUpgrade`) |
| `7c6f9653c8ba38c7102ccf23792523d706dd9974` | Retain committed sends in the receipt source so pax_sequencerReceipt answers the committed rung | `node/sequencer/receipt_source.go` | `v7.0` (`MergeUpgrade`) |
| `88e4603c6866aff847cd4780b81e9628f0ca1a24` | Admit the consensus mempool into the sequencer's open layer at each proposal | `node/sequencer/consensus_source.go`, `node/sequencer/node.go` | `v7.0` (`MergeUpgrade`) |
| `752175585cfaf8526c322f9e0700ed1dad2e4bcf` | Read the receipt context captured inside the accept critical section in Receipter.after so a seal does not halt replication | `node/sequencer/receipt.go` | `v7.0` (`MergeUpgrade`) |
| `78f814d6aee203a327dd989cb50ebd994e6b7522` | Carry the receipt context on every micro-batch accept | `node/sequencer/microbatch.go` | `v7.0` (`MergeUpgrade`) |
| `819e3bd4999a7d11517da4f7b77a27c8e968cd67` | Accept a deposit-root registration on the deposit root authority's signature, not on the checkpoint submitter | `modules/layerxcustody/keeper/deposit_root.go` | `v7.0` (`MergeUpgrade`) |
| `290ac0b03df7c03199a972dd29b5f6907f273ad5` | Wire the chain registry module into the application and its merge store upgrade | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `dd59c88fc6a75eec061b13d51c794ba4140fa34e` | Partition block admission by chain allowance | `node/app.go`, `node/sequencer/executor.go`, `node/sequencer/limits.go` | `v7.0` (`MergeUpgrade`) |
| `e576fb4452bca51b3db98441a05545bf9bd5ac88` | Index chain transactions, adjust each chain's base fee and pay the operator share at end of block | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `6bd5fa59da94925203fabb708ec410dbd50e4564` | Node integration of native execution, module state and sequencing. | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `e4631bb1816d5e485ab8b6dcbaf34d4566eb1a6f` | Serve the chain registry precompile and gate it on the module | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `9acf8949148a3c64d250496bd8f6a6d14975229a` | Integrate sequencer config | `node/sequencer/config.go`, `node/sequencer/receiptkey.go` | `v7.0` (`MergeUpgrade`) |
| `e684aac095c61dfb9de16c4dab0b32ecc1642ca0` | Integrate perps errors | `modules/perps/keeper/account.go`, `modules/perps/keeper/adl.go`, `modules/perps/keeper/book.go`, `modules/perps/keeper/matching.go`, `modules/perps/keeper/orders.go`, `modules/perps/keeper/triggers.go` | `v7.0` (`MergeUpgrade`) |
| `b4db752c64066e1c9e8f361115686155aab7032f` | Integrate sequencer WAL | `node/sequencer/dial.go`, `node/sequencer/failover.go`, `node/sequencer/handover.go`, `node/sequencer/standby.go` | `v7.0` (`MergeUpgrade`) |
| `992919ea83fc616cfc149511fff4d32d2263f3b9` | Integrate perps 0x1015 | `modules/perps/keeper/keeper.go` | `v7.0` (`MergeUpgrade`) |
| `76d1d6c21a2f500ab162abd5679a643d5c812167` | Integrate node genesis | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `401182b1f7b405c32dff76c0c2b91aa8a8e89769` | Integrate sequencer module | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `65d7f4bb9443272d1b49c29427a8f9500772e8d7` | Integrate perps governance | `modules/perps/keeper/msg_server.go`, `modules/perps/keeper/pause.go` | `v7.0` (`MergeUpgrade`) |
| `143889db56bf587948feec0b3cdf968deaa45f71` | Integrate retirement | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `e3d6107cb9aeb4ac9c10cbaddd49f880ef44a477` | Integrate 0x1013 layerxcustody | `modules/layerxcustody/keeper/msg_server.go`, `modules/layerxcustody/keeper/release.go`, `modules/layerxcustody/keeper/retire.go` | `v7.0` (`MergeUpgrade`) |
| `a72bf127bc3fcdc476036e3edb4cd177a90c51b4` | Node integration of native execution, module state and sequencing. | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `4b58d257c94647b8543df7c1fd97803b792c95dc` | Integrate module keeper | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `fccf43f01490242f1ad22717a1f737830424ad8b` | Node integration of native execution, module state and sequencing. | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `b77dcc007a9955f56949e9393bd075163a7ae505` | Integrate config | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `9097a7fa584972ed140444cbb604dd61f4784622` | Node integration of native execution, module state and sequencing. | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `01611e2eca6c1b5f988b6ceb3010688bf152d07c` | Integrate go exported modules spot keeper | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `2c85937758de927de3156f949e5b7835fbfe22f2` | Integrate node go API | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `3d01fcb29f13fc9c46749e4a48b5a125ade80a56` | Integrate node ABCI | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `62108d89d8ac6f9911878af0f6eaaa765c9ce738` | Integrate sequencer executor | `node/sequencer/config.go`, `node/sequencer/executor.go`, `node/sequencer/fee.go`, `node/sequencer/receipt.go`, `node/sequencer/receiptkey.go`, `node/sequencer/standby.go` | `v7.0` (`MergeUpgrade`) |
| `ec287c0eac5484249c31bf5b7cd7699a4110d870` | Resolve chain operators through the EVM association for bond, fund and refund | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `098f561a06d7ba51ff2557437ebfc38129ba4954` | Perps ADL refusals, margin encoding and spot errors. | `precompiles/perps/testdata/v69_vectors.json` | `v7.0` (`MergeUpgrade`) |
| `1208b0754634b208295a6731854f7fc22b9b397c` | Return the committed block's own id from LastBlockID | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `2f080287b7c12433126fc3ddb02085724d55c50b` | Integrate sequencer receipt | `node/sequencer/accepter.go`, `node/sequencer/follower.go`, `node/sequencer/log_feed.go`, `node/sequencer/receipt_source.go` | `v7.0` (`MergeUpgrade`) |
| `280826868f8aac6cf2378e88376f4987d636eec5` | Node integration of native execution, module state and sequencing. | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `202fc37075d5fbff509cd6ca49d807a8d82e550a` | Node integration of native execution, module state and sequencing. | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `75a1d648ba66ef5a4687997b128bc65ffaa4eb3e` | Node integration of native execution, module state and sequencing. | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `839aba41da4cd87aaeb320da9e88be2005d10bc8` | Treat an unset Block.MaxGasWanted as unbounded at admission. | `node/sequencer/limits.go` | `v7.0` (`MergeUpgrade`) |
| `50deffff99af2bad3fd9db811562fc963e2c7628` | one block of root and chain transactions end to end, including restart recovery. | `node/app.go`, `node/sequencer/accept.go` | `v7.0` (`MergeUpgrade`) |
| `8f81a8be091bfb1e8338d827950dc8bca0653b44` | Cancel the attached take-profit and stop-loss pair whenever a perps position is removed | `modules/perps/keeper/positions.go`, `modules/perps/keeper/triggers.go` | `v7.0` (`MergeUpgrade`) |
| `75256e3897053527589ab3fc26ee00cee9eec532` | Name the payment asset in the sequencer receipt and carry the fee denomination separately | `node/sequencer/receipt.go`, `node/sequencer/standby_role.go` | `v7.0` (`MergeUpgrade`) |
| `740605d0ffa5f6e7a2896fc90399b822c80fb831` | Publish committed state roots in new-head subscriptions. | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `aab8c3d2a744c5f0d38b1ebfb18c6f1f6543305f` | Return the recorded hash for identical WebSocket transaction resubmissions | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `4d4bfd9a7bc9638e7416305e7a2f1138d0f95104` | Rebuild the sequencer receipt source from the write-ahead log replay on node start | `node/sequencer/accepter.go`, `node/sequencer/node_start.go`, `node/sequencer/receipt_source.go`, `node/sequencer/replay.go` | `v7.0` (`MergeUpgrade`) |
| `6ea5ee7e8d9ec4aea789da41df5c25d464fa4bf2` | Populate Autobahn committed header roots before publishing newHeads | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `d429fa28bab336c64e4c83c2ea638a44473a3e41` | Serve receipts of sends committed before a restart from the rebuilt receipt source | `node/sequencer/accepter.go`, `node/sequencer/node_start.go`, `node/sequencer/receipt_source.go`, `node/sequencer/wal/wal.go` | `v7.0` (`MergeUpgrade`) |
| `607875f3576776cb8615a562d7a1a20e747a85ac` | Sequencer transaction and block limits. | `node/sequencer/limits.go` | `v7.0` (`MergeUpgrade`) |
| `165efc528ae0bf76b4aa544a6ba7b7ade592eb06` | Preserve custody retirement state through genesis export and import. | `modules/layerxcustody/keeper/genesis.go`, `modules/layerxcustody/keeper/keeper.go`, `modules/layerxcustody/keeper/release.go`, `modules/layerxcustody/keeper/retire.go`, `modules/layerxcustody/keeper/retirement_genesis.go` | `v7.0` (`MergeUpgrade`) |
| `7aa2cc6857031b6a9a77667a34a3d35a22ec38f1` | Perps account, margin, funding, fee and position accounting. | `modules/perps/keeper/account.go`, `modules/perps/keeper/adl.go`, `modules/perps/keeper/adl_recovery.go`, `modules/perps/keeper/fees.go`, `modules/perps/keeper/funding.go`, `modules/perps/keeper/genesis.go`, `modules/perps/keeper/insurance.go`, `modules/perps/keeper/keeper.go`, `modules/perps/keeper/liquidation.go`, `modules/perps/keeper/margin.go`, `modules/perps/keeper/mark.go`, `modules/perps/keeper/matching.go`, `modules/perps/keeper/msg_server.go`, `modules/perps/keeper/msg_server_market.go`, `modules/perps/keeper/openinterest.go`, `modules/perps/keeper/orders.go`, `modules/perps/keeper/positions.go`, `modules/perps/keeper/quotes.go`, `modules/perps/keeper/triggers.go`, `precompiles/perps/positions.go` | `v7.0` (`MergeUpgrade`) |
| `881d9c90ebe6817e56b41a9c27d39719b13b6f38` | Recover sequencing consensus state after restart. | `node/sequencer/consensus_source.go` | `v7.0` (`MergeUpgrade`) |
| `b408a0bf7297f2ddbefb8a495b131cb254ba822c` | Switch the hosted kernel to programs guest ABI 5 at the merge upgrade and keep the choice across restarts | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `83a611ecefe405d19fb6c23e10c35c5bac941406` | Validate compute-market settlement and arbitration records. | `programs/crates/layerx-programs-market/src/arbitration.rs`, `programs/crates/layerx-programs-market/src/lib.rs`, `programs/crates/layerx-programs-market/src/settle.rs` | `v7.0` (`MergeUpgrade`) |
| `1ee93282cbe5d59b98b34bebd8a33f7ffdecf5b6` | Native execution and committed-state validation in block processing. | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `26d76c12362b7507da8e1635453268de152cfd6a` | Persistent sequencing receipts and primary/standby recovery. | `node/sequencer/config.go`, `node/sequencer/consensus_source.go`, `node/sequencer/dial.go`, `node/sequencer/handover.go`, `node/sequencer/microbatch.go`, `node/sequencer/node.go`, `node/sequencer/node_start.go`, `node/sequencer/receipt_source.go`, `node/sequencer/replay.go`, `node/sequencer/standby.go`, `node/sequencer/standby_role.go` | `v7.0` (`MergeUpgrade`) |
| `e1b27a05d11ac6890ccf5f3f14614ec1be58a0d8` | Perps pause authority and end-block processing. | `modules/perps/keeper/endblock.go`, `modules/perps/keeper/msg_server_pause.go` | `v7.0` (`MergeUpgrade`) |
| `52c5b590658c80dfc7116c9e89a47dc6fe2f2672` | Evidence keeper integration in the node. | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `c5b5060fdb3b4e0d6e22bce073b443b6715f6c35` | Compute-market dispute state and runtime recovery. | `programs/crates/layerx-programs-market/src/dispute.rs` | `v7.0` (`MergeUpgrade`) |
| `fe12e73e857d44b1a98c3cf16c02b3a692f858e7` | Bound sequencing admission, receipt handling, leases and WAL replay. | `node/sequencer/accept.go`, `node/sequencer/accepter.go`, `node/sequencer/driver.go`, `node/sequencer/executor.go`, `node/sequencer/lease.go`, `node/sequencer/lease_renew.go`, `node/sequencer/microbatch.go`, `node/sequencer/node.go`, `node/sequencer/node_start.go`, `node/sequencer/node_stop.go`, `node/sequencer/nonce_admission.go`, `node/sequencer/nonce_queue.go`, `node/sequencer/receipt.go`, `node/sequencer/receipt_source.go`, `node/sequencer/receiptkey.go`, `node/sequencer/replay.go`, `node/sequencer/standby.go`, `node/sequencer/standby_role.go`, `node/sequencer/wal/wal.go` | `v7.0` (`MergeUpgrade`) |
| `f56cd1339ecbdfc0e34b1a5ebf4dc614940ea614` | Interchain account lifecycle integration in the node. | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `311c57868f6517011f3c06222dea95784cf9951a` | Validate receipt-signing key material. | `node/sequencer/receiptkey.go` | `v7.0` (`MergeUpgrade`) |
| `90f4829d366a509c270bc0d18085fcece25b5fa8` | Governed custody-bank transfer authority. | `modules/layerxcustody/keeper/keeper.go` | `v7.0` (`MergeUpgrade`) |
| `b8dd8df90baa744439fe410a9243051b61784e5c` | CosmWasm configuration in node setup. | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `da61cc5f62a00ab375fcb87768288e82a4bd4ba2` | Native ledger keeper integration in the node. | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `9ae50fb7f181a446e992fb85e6fc1c3b6102c27c` | Node integration of native execution, module state and sequencing. | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `99a76e154286fefe6da0c854c0a7f236d162f0cd` | Reconcile the consensus AppHash with the application commit hash across a node restart | `node/sequencer/driver.go`, `node/sequencer/executor.go`, `node/sequencer/node_start.go` | `v7.0` (`MergeUpgrade`) |
| `a929232089889e2357e5b5b336c2598d75ec464c` | Node integration of native execution, module state and sequencing. | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `2b5d674413f8e816349b6f383c03dbe4d87e9c72` | Sequencer lifecycle integration in the node application. | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `2c3d306db894d0a7f3e7e6d9154a730efe43e455` | Sequencer receipt resolution across block commitment. | `node/sequencer/receipt.go` | `v7.0` (`MergeUpgrade`) |
| `a2bf776a9fbdd7265c4a1dfcc89cb2fe8e755d13` | Lock every intentional copy of a generated or shared file with a sync test and remove the copies that are not intentional | `precompiles/perps/abi/abi.json` | `v7.0` (`MergeUpgrade`) |
| `bc89ead834ccbc9670b2b23aaa16894524057843` | Release primary replication after block commitment. | `node/sequencer/driver.go` | `v7.0` (`MergeUpgrade`) |
| `045cbd6efe2440c316e3bca8ef136c2ddd2411f5` | Bound sequencing leases and commit replication. | `node/sequencer/config.go`, `node/sequencer/driver.go`, `node/sequencer/lease_renew.go`, `node/sequencer/metrics.go` | `v7.0` (`MergeUpgrade`) |
| `e8f9aa1c5bd69a10261c23c71d18fad59758152c` | Freeze the tokenfactory store key layout and drop unused prefixes | `modules/tokenfactory/types/keys.go` | `v7.0` (`MergeUpgrade`) |
| `58a034c9e0f8e0eb44fd095ba621890c03e5e84f` | Trim the tokenfactory BankKeeper expected interface to the methods it calls | `modules/tokenfactory/types/expected_keepers.go` | `v7.0` (`MergeUpgrade`) |
| `477965c4fad6caee007ade75e67a43709839cbd6` | Add paused_denoms field 3 to the tokenfactory GenesisState proto | `modules/tokenfactory/types/genesis.pb.go` | `v7.0` (`MergeUpgrade`) |
| `4d1f8830d5b66ec32b5d01f37d752350aaa0c0bf` | Declare the xweb Query service in api/xweb/query.proto and generate its Go code | `modules/xweb/types/query.pb.go`, `modules/xweb/types/query.pb.gw.go` | `v7.0` (`MergeUpgrade`) |
| `47a98e9f4a38efc174499e9495876f7a6dfa71e1` | Declare the launchpad Query service in api/launchpad/query.proto and generate it | `modules/launchpad/types/query.pb.go`, `modules/launchpad/types/query.pb.gw.go` | `v7.0` (`MergeUpgrade`) |
| `460b14417a92ba3c2bd89672d910e5a9e83e6566` | Derive per-call idempotency keys and 0x0906 stream ids deterministically | `modules/layerxpay/types/idempotency.go` | `v7.0` (`MergeUpgrade`) |
| `da532eb561e32482f2f45f59416c0527dd226cd3` | Decode kernel stream records and stream results for the 0x101B views | `modules/layerxpay/types/records_stream.go` | `v7.0` (`MergeUpgrade`) |
| `4456a7d70a0bddc6d253824129a58307b2b423ab` | Decode kernel service offer, agreement, commitment, delivery, progress, dispute records | `modules/layerxpay/types/records_service.go` | `v7.0` (`MergeUpgrade`) |
| `79a5a0d96074615681916211db57f30e2ab03095` | Add the layerxledger genesis proto and GenesisState | `modules/layerxledger/types/genesis.go`, `modules/layerxledger/types/genesis.pb.go` | `v7.0` (`MergeUpgrade`) |
| `be41d8c9da39f13cfde1dd0353f44706bd3f226d` | Encode the fixed-layout asset payloads in layerxledger types | `modules/layerxledger/types/payload.go` | `v7.0` (`MergeUpgrade`) |
| `0991c9602aaf014db6894383679211943a1dced2` | Encode send and receive payloads and validate payer grants in layerxledger types | `modules/layerxledger/types/payload_transfer.go` | `v7.0` (`MergeUpgrade`) |
| `7c2aa8e3176016da3bf77433074f7645dfdb0f8c` | Extend LayerXScopeAuthorization with kind, period caps, purpose and validity | `modules/layerxauth/types/authorization.go`, `modules/layerxauth/types/tx.pb.go` | `v7.0` (`MergeUpgrade`) |
| `10edc28de88ca4ff67fcd05531b2bda228e2f017` | Decode kernel escrow and budget records for the 0x101B views | `modules/layerxpay/types/records_budget.go`, `modules/layerxpay/types/records_escrow.go` | `v7.0` (`MergeUpgrade`) |
| `071d3ae78b5c9c6f4f075a6000535f42a108081f` | Encode the governance session, capability and revoke payloads in layerxauth types | `modules/layerxauth/types/payload.go` | `v7.0` (`MergeUpgrade`) |
| `d375cc52b88dacaafb89b8173ddd2778287223c6` | Add the layerxauth genesis proto and GenesisState | `modules/layerxauth/types/genesis.go`, `modules/layerxauth/types/genesis.pb.go` | `v7.0` (`MergeUpgrade`) |
| `8a2bd6117b8f3a4114163dcd3610e2e365ddce60` | Freeze the pricefeed read-only Query proto | `modules/pricefeed/types/query.pb.go`, `modules/pricefeed/types/query.pb.gw.go` | `v7.0` (`MergeUpgrade`) |
| `4b10078da32c50006dd270aa6dc292e586648135` | Reserve the removed oracle Params and vote fields in the oracle protos | `modules/oracle/types/oracle.pb.go`, `modules/oracle/types/tx.pb.go` | `v7.0` (`MergeUpgrade`) |
| `fe95952994442040bf715947ac4f555ea59d0420` | Add ValidateLeaders as the single leader-list rule | `modules/sequencer/types/leaders.go` | `v7.0` (`MergeUpgrade`) |
| `6f6e7f877906cd69382b7d5e8672125528790e1e` | Add the stored-votes binary codec | `modules/sequencer/types/votes.go` | `v7.0` (`MergeUpgrade`) |
| `6cc60a93f5adbcd624975cd04ca5fb85b42ac3f0` | Export the bank v6.12.0 plan name and pin it to the node plan | `precompiles/bank/bank.go` | `v7.0` (`MergeUpgrade`) |
| `4300a05da72ec499085e4e133d9c09539d52aeab` | Add the web/feedenom key family and its accessor | `modules/xweb/keeper/program_denom.go` | `v7.0` (`MergeUpgrade`) |
| `1c6a7abd65ca3173c80bf8eb29f36e0935fe16cc` | Carry origin-2 records, answers, fee denoms and counters in GenesisState and validate them | `modules/xweb/keeper/program.go`, `modules/xweb/keeper/program_fulfil.go`, `modules/xweb/types/genesis.go` | `v7.0` (`MergeUpgrade`) |
| `a40ca2a4534d1f6af3c81a8a719d6ec5a263b614` | Run DenomAuthorityMetadata.Validate inside tokenfactory GenesisState.Validate | `modules/tokenfactory/types/genesis.go` | `v7.0` (`MergeUpgrade`) |
| `c6faa058ca00a565291da2bf86378245418fdf02` | Register tokenfactory error codes 28 to 30 for issuer and wasm pause paths | `modules/tokenfactory/types/errors.go` | `v7.0` (`MergeUpgrade`) |
| `527feda1a889f25c03b16f902598d7dadf9b92fe` | Add AttributePaused and drop the unused tokenfactory transfer attributes | `modules/tokenfactory/keeper/msg_server.go`, `modules/tokenfactory/types/events.go` | `v7.0` (`MergeUpgrade`) |
| `f2cefb1ad4f0f9780912b71fdd53b8b86e937ca8` | Register layerxanchor ErrModuleRetired code 19 carrying LXP_MODULE_DISABLED | `modules/layerxanchor/types/errors.go` | `v7.0` (`MergeUpgrade`) |
| `181bcf27d313c42479e77b119967f7b362598ad1` | Freeze the layerxanchor retirement step names and retired event | `modules/layerxanchor/types/events.go`, `modules/layerxanchor/types/retire.go` | `v7.0` (`MergeUpgrade`) |
| `0760515c4481e3de5009919f46434d5d0b5d3b33` | Record the fee denom of each asset when RequestFromProgram accepts a paid request | `modules/xweb/keeper/program.go` | `v7.0` (`MergeUpgrade`) |
| `fe3c578a5fd6439b69bdb9d491b6220b35c1f80b` | Update consensus state and behavior in `modules/xweb/keeper/program.go` | `modules/xweb/keeper/program.go` | `v7.0` (`MergeUpgrade`) |
| `fa6ea2fb0664e10f44c2991a923c1ceabdba0e17` | Iterate origin-2 records, answers and fee denoms in key order | `modules/xweb/keeper/keeper.go` | `v7.0` (`MergeUpgrade`) |
| `de0c5001535767b4618f9b2a4aab4e479cf294cd` | Bind the upgrade reader to the xweb keeper and expose ProgramOriginActive | `modules/xweb/keeper/activation.go`, `modules/xweb/keeper/keeper.go`, `modules/xweb/types/expected_keepers.go` | `v7.0` (`MergeUpgrade`) |
| `b4178cb3fb3b6cd42870cc80f6248b32b6d2df75` | Update consensus state and behavior in `modules/xweb/keeper/keeper.go` | `modules/xweb/keeper/keeper.go` | `v7.0` (`MergeUpgrade`) |
| `d69fe9cbd0d8029d6f237ec443dde33b8af3004e` | Export and import origin-2 state through InitGenesis and ExportGenesis | `modules/xweb/keeper/genesis.go` | `v7.0` (`MergeUpgrade`) |
| `a2ab6d9f280f113ed5904e3ebeed59a5a7162993` | Split the origin-2 fee in the recorded denom and remove balance probing | `modules/xweb/keeper/program_fulfil.go` | `v7.0` (`MergeUpgrade`) |
| `5e826262faf7f2498cb90e9328a6eba562622c2e` | Count bank-created accounts through OTel and drop the legacy new-account counter | `sdk/x/bank/keeper/metrics.go`, `sdk/x/bank/keeper/send.go` | `v7.0` (`MergeUpgrade`) |
| `d9c13350a47ad697fe06282d0ca330ae8b071d3a` | Convert a launchpad Market into the MarketInfo query view | `modules/launchpad/types/query_market.go` | `v7.0` (`MergeUpgrade`) |
| `dde6ac34ab7eb8edee6aacc81d9f41d60cab7f22` | Convert fee accounting and parse quote ledger kinds for queries | `modules/launchpad/types/query_ledger.go` | `v7.0` (`MergeUpgrade`) |
| `1aa92029df8f0e44929c3ea67ee976e06b13e92f` | Add the launchpad Querier that serves the Query service | `modules/launchpad/keeper/grpc_query.go` | `v7.0` (`MergeUpgrade`) |
| `4cb36c201aa1a7217d9c6c507a792777c56d7acc` | Validate query paging and hex address arguments | `modules/launchpad/types/query_validate.go` | `v7.0` (`MergeUpgrade`) |
| `7b9353d01ac5e5af2b2eb431027a00da3b70eb5e` | Convert fee rights, airdrop epochs and retired addresses into query views | `modules/launchpad/types/query_registry.go` | `v7.0` (`MergeUpgrade`) |
| `44cafed89f6c42220e66619527e8b9c40cf7e409` | Read a market's price snapshot ring as height-ordered price points | `modules/launchpad/types/price.go` | `v7.0` (`MergeUpgrade`) |
| `4e8eb39c783cbf697bab73dd5b8d5e55cd112daa` | Refuse generation 1 genesis state and stop writing or exporting it | `modules/launchpad/keeper/genesis.go`, `modules/launchpad/types/genesis.go` | `v7.0` (`MergeUpgrade`) |
| `0bd553fa64c41bade5eda441e5527a8d8b4b4642` | Serve a market's price history from the keeper | `modules/launchpad/keeper/price_history.go` | `v7.0` (`MergeUpgrade`) |
| `ad6e509c6169d55c51caa93add25b64113e643f8` | Export the surface runtime from the keeper and alias it in the precompile | `modules/launchpad/keeper/market.go`, `modules/launchpad/keeper/migrate_v6120.go`, `modules/launchpad/keeper/surfaces.go` | `v7.0` (`MergeUpgrade`) |
| `265868e304c74c899153d6b1db1c1ef13e96d3f6` | Delete the uncalled generation 1 native airdrop functions | `modules/launchpad/keeper/fees.go` | `v7.0` (`MergeUpgrade`) |
| `37bfcddc4e0ce00bd102f4be7ff1af749ed58af9` | Delete the generation 1 keeper accessors and the per-market protocol counter | `modules/launchpad/keeper/keeper.go` | `v7.0` (`MergeUpgrade`) |
| `8952e9957217a0006ccef720f87ea5fde6364fa4` | Freeze the kernel result-code mapping of the layerxledger and layerxauth keepers | `modules/layerxauth/keeper/kernel_codes.go`, `modules/layerxledger/keeper/kernel_codes.go` | `v7.0` (`MergeUpgrade`) |
| `3353406052d2ae0e669cfd5bede44a4068773eb1` | Retire the keeper-level idempotency keys of layerxauth | `modules/layerxauth/types/keys.go` | `v7.0` (`MergeUpgrade`) |
| `dbec138bd01a0f1dfa6f76a1cdc846adf053d7d4` | Generate layerxpay Params and MsgUpdateParams from api/layerxpay/tx.proto | `modules/layerxpay/types/codec.go`, `modules/layerxpay/types/msgs.go`, `modules/layerxpay/types/params.go`, `modules/layerxpay/types/tx.pb.go` | `v7.0` (`MergeUpgrade`) |
| `db012c6d9cdfc8488fe125788714bffa08ae63ed` | Register layerxpay codespace errors for unbound callers, disabled families, gate drift | `modules/layerxpay/types/errors.go` | `v7.0` (`MergeUpgrade`) |
| `e90664c162ce7ddf211a815ddf91f894a2095de3` | Freeze the 0x101B method to kernel activity type table | `modules/layerxpay/types/activity.go` | `v7.0` (`MergeUpgrade`) |
| `0a5d4de2305092fd6807dea09ef552fa7ad3962e` | Retire layerxauth error code 11 in favour of the layerxgov 735 pause refusal | `modules/layerxauth/types/errors.go` | `v7.0` (`MergeUpgrade`) |
| `4c970bd48ab43e1c4cf0d70d2ea8d80db9f2d383` | Generate layerxagent Params and MsgUpdateParams from api/layerxagent/tx.proto | `modules/layerxagent/types/codec.go`, `modules/layerxagent/types/keys.go`, `modules/layerxagent/types/msgs.go`, `modules/layerxagent/types/params.go`, `modules/layerxagent/types/tx.pb.go` | `v7.0` (`MergeUpgrade`) |
| `b8bf3a7e24fd1891a01163ea3ba709c662e82593` | Define the layerx bank mirror denom for kernel assets | `modules/layerxledger/types/denom.go` | `v7.0` (`MergeUpgrade`) |
| `812e69362afd39c590173801ae8b9d313d8295ee` | Add the layerxpay Params query proto | `modules/layerxpay/types/query.pb.go`, `modules/layerxpay/types/query.pb.gw.go` | `v7.0` (`MergeUpgrade`) |
| `215b90bd1b9c7e9a8aca66c79e95ff4f68979b9b` | Construct the layerxledger keeper over the shared kernel host and retire KernelKVPrefix | `modules/layerxledger/keeper/keeper.go`, `modules/layerxledger/types/expected_keepers.go`, `modules/layerxledger/types/keys.go` | `v7.0` (`MergeUpgrade`) |
| `601ea7931ec58c9bfaf8087f3d7161abf5057744` | Generate layerxagent GenesisState, Job and the Query service | `modules/layerxagent/types/genesis.go`, `modules/layerxagent/types/genesis.pb.go`, `modules/layerxagent/types/query.pb.go`, `modules/layerxagent/types/query.pb.gw.go` | `v7.0` (`MergeUpgrade`) |
| `1baec2b9e7e0a315fe6387b3b70e0da3ca068253` | Add the layerxpay keeper over the shared lxphost host with a real-host test harness | `modules/layerxpay/keeper/keeper.go`, `modules/layerxpay/keeper/params.go` | `v7.0` (`MergeUpgrade`) |
| `7fa436c3953b7ad2d7f4c0c42058c1b5940c11e3` | Gate every pay write on module pause, family enable flags and kernel enable records | `modules/layerxpay/keeper/gate.go` | `v7.0` (`MergeUpgrade`) |
| `c33fd9c0100ca320df0c22a6678d5ddb46336666` | Init and export the params-only layerxledger genesis | `modules/layerxledger/keeper/genesis.go` | `v7.0` (`MergeUpgrade`) |
| `7f9341203ddddc618823015aa0120d52afbc7710` | Resolve the kernel authority of an EVM caller from its 0x1004 DID binding | `modules/layerxpay/keeper/authority.go` | `v7.0` (`MergeUpgrade`) |
| `bca9d33a47bd9b37168b87bb9de212613aeb090c` | Serve ledger views from the kernel host read exports | `modules/layerxledger/keeper/views.go` | `v7.0` (`MergeUpgrade`) |
| `2e00c59d7b9c5eb22c2815f3b7123cb5fc539317` | Serve getStream, getStreamResult and streamAccrued from the kernel | `modules/layerxpay/keeper/stream_views.go` | `v7.0` (`MergeUpgrade`) |
| `83ba31c1306158903457d4ca6ff7e08f6134b138` | Serve escrow views from kernel records in the host store | `modules/layerxpay/keeper/escrow_views.go` | `v7.0` (`MergeUpgrade`) |
| `68be9e65aaf4984b05be1c057e98928f5af567bc` | Serve the nine service views from kernel records | `modules/layerxpay/keeper/service_views.go` | `v7.0` (`MergeUpgrade`) |
| `68f42f38a12b0abe1242f392a3e2108d6ff4240a` | Resolve the EVM caller to its bound LayerX DID as the ledger actor | `modules/layerxledger/keeper/authority.go` | `v7.0` (`MergeUpgrade`) |
| `9243969f5ae5c3be06b2f0b3f097e9492493fe37` | Freeze the perps Msg proto and the Params and Market customtype codec | `modules/perps/types/msg_codec.go`, `modules/perps/types/tx.pb.go` | `v7.0` (`MergeUpgrade`) |
| `0f25f269d01dc9cd6acd1202169319ac443a57ce` | Add spot active position and length key prefixes | `modules/spot/types/keys.go` | `v7.0` (`MergeUpgrade`) |
| `1d7c2431bf1b81134e2b9f7e4880781843c7058d` | Register layerxexchange ErrRetired code 14 | `modules/layerxexchange/types/errors.go`, `modules/layerxexchange/types/keys.go` | `v7.0` (`MergeUpgrade`) |
| `e2b0d92c19c350a1bbb57bd95b92d2ca68e758a3` | Validate the three perps governance Msgs and declare their signers | `modules/perps/types/msgs.go` | `v7.0` (`MergeUpgrade`) |
| `5ac23575f4136cd72f5659a08cf766b88e78a6e6` | Expose MarketSymbol on the pricefeed keeper | `modules/pricefeed/keeper/price.go` | `v7.0` (`MergeUpgrade`) |
| `0366a30f88342c6d37c844b5108902517dac93b1` | Make spot active list operations constant-cost | `modules/spot/keeper/orders.go` | `v7.0` (`MergeUpgrade`) |
| `328192be98ea97a5ff84ef42729c1cb8a4cc1d3e` | Register the perps Msgs on the interface registry and legacy amino | `modules/perps/types/codec.go` | `v7.0` (`MergeUpgrade`) |
| `63b9566d2c1e9d480168ea81cf33a59ab5801314` | Register a spot active index invariant | `modules/spot/keeper/invariants.go` | `v7.0` (`MergeUpgrade`) |
| `7641d7c20e716fd44db59a8c0e56f01401bb23a5` | Drive pricefeed LXP names from one table and list the codes | `modules/pricefeed/types/errors.go` | `v7.0` (`MergeUpgrade`) |
| `b4a69825b4bd5002773c47f5c291c76f1197c1a2` | Write spot genesis active lists through activeAdd | `modules/spot/keeper/genesis.go` | `v7.0` (`MergeUpgrade`) |
| `67bd4bd3e89ec9cc96901c7c51b6190dae6b88d7` | Bound spot EndBlock budgets, token count and price markets | `modules/spot/types/params.go` | `v7.0` (`MergeUpgrade`) |
| `1af6aa38fc0d77729e7c7ca71488aba2fe4d96ef` | Add spot ListTokens registering new and guarding delisted tokens | `modules/spot/keeper/vault.go` | `v7.0` (`MergeUpgrade`) |
| `5a1adbd9c17d892e200796cb167034d5a2387f67` | Give the layerxexchange keeper an upgrade keeper and Retired | `modules/layerxexchange/keeper/keeper.go`, `modules/layerxexchange/types/expected_keepers.go` | `v7.0` (`MergeUpgrade`) |
| `cec4bc7d54490735bee3aa13740f757912a81f36` | List every registered perps error in registry order | `modules/perps/types/errors.go` | `v7.0` (`MergeUpgrade`) |
| `f6cc80fbb9d1fa43f87baa78369e07d082a151ab` | Add spot ValidatePriceMarket against the price feed | `modules/spot/keeper/pricing.go` | `v7.0` (`MergeUpgrade`) |
| `3d467bdf723dc890eabaf918cd6fe3481924416d` | Add MarketSymbol to the perps PricefeedKeeper contract | `modules/perps/types/expected_keepers.go` | `v7.0` (`MergeUpgrade`) |
| `b8a9a135e91e30b83aac974eb26f2aed733f7109` | Re-emit epoch hook events into the block context | `modules/epoch/types/hooks.go` | `v7.0` (`MergeUpgrade`) |
| `7a5c473f89258ba0787195001efc3746ae3bdc8d` | Refuse mint token_release_amount above int64 | `modules/mint/types/params.go` | `v7.0` (`MergeUpgrade`) |
| `87bb449c9271ec9616aad51b55d0f508dfbdca53` | Convert mint amounts with NewIntFromUint64 | `modules/mint/types/minter.go` | `v7.0` (`MergeUpgrade`) |
| `cf4f75b34709f6a8d6e6fb53cfb8f3f44664b533` | Guard epoch keeper hooks against an unset hook set | `modules/epoch/keeper/hooks.go` | `v7.0` (`MergeUpgrade`) |
| `2ca20ab12baa95d21afd773f99a5a99b0b8624b8` | Serve the pricefeed Params and Price queries | `modules/pricefeed/keeper/grpc_query.go` | `v7.0` (`MergeUpgrade`) |
| `924150509439e6d0af6a535627ac31e2a838f78b` | Add spot Migrate1to2 building the active index | `modules/spot/keeper/migrations.go` | `v7.0` (`MergeUpgrade`) |
| `56a8274757e094e2dedd21808e9089bf7e074b8c` | Refuse layerxexchange writes after v6.12.0 | `modules/layerxexchange/keeper/intents.go`, `modules/layerxexchange/keeper/msg_server.go` | `v7.0` (`MergeUpgrade`) |
| `3cdd4a058567bb76d68af2d2a52ca7dea1a49ff9` | Retire the never-emitted oracle vote event type | `modules/oracle/types/events.go` | `v7.0` (`MergeUpgrade`) |
| `36e33e2fa9a7cf65a17e19b629253fdaeccef800` | Route spot MsgUpdateParams through ListTokens and market checks | `modules/spot/keeper/msg_server.go` | `v7.0` (`MergeUpgrade`) |
| `e4b10d2e9965f01925421acf464722c87dcc7b19` | Skip unpriced tokens in spot resting execution | `modules/spot/keeper/execute.go` | `v7.0` (`MergeUpgrade`) |
| `f39ea47fcc935ae923a3cc06af6a7b676cb6eace` | Serve the pricefeed Prices batch query with the 64-id bound | `modules/pricefeed/keeper/grpc_query_prices.go` | `v7.0` (`MergeUpgrade`) |
| `b7944b98d532a6f2f452c10d0df2430a5384bd60` | Serve the pricefeed feed-key, validator and observation queries | `modules/pricefeed/keeper/grpc_query_feeds.go` | `v7.0` (`MergeUpgrade`) |
| `f29008b2c3d26b12b3119e359fd217babde22036` | Add ActivationHeightKey 0x07 and per-height stored-votes keys | `modules/sequencer/types/keys.go` | `v7.0` (`MergeUpgrade`) |
| `8f65baeb6c7235521e5adca8e14e25504de045c0` | Make seqHash canonical in modules/sequencer/types | `modules/sequencer/types/seqhash.go` | `v7.0` (`MergeUpgrade`) |
| `f064261a30e79cb005da45f9be7a28c48977e8ca` | Make txRoot canonical in modules/sequencer/types | `modules/sequencer/types/txroot.go` | `v7.0` (`MergeUpgrade`) |
| `de24c7777263eed777806386e4c9b8868bd54a3c` | Move the tx-0 MsgSequenceBatch codec into modules/sequencer/types | `modules/sequencer/types/batch_tx.go` | `v7.0` (`MergeUpgrade`) |
| `5abda45e22d1d3e163711fdd43059b718ffbffa3` | Add VerifyBatchTxs binding a batch to its sequenced transactions | `modules/sequencer/types/batch_verify.go` | `v7.0` (`MergeUpgrade`) |
| `30edb7a5551e2c64a9203ac1b46e303eb32437a4` | Store the first sequenced height in the keeper | `modules/sequencer/keeper/activation.go` | `v7.0` (`MergeUpgrade`) |
| `ba0fc03bdab101c02be8814bfccdb63fdc63cfe3` | Store decided votes per height in the keeper and keep two heights | `modules/sequencer/keeper/votes.go` | `v7.0` (`MergeUpgrade`) |
| `5c4a8716d6eb1fd5e54f11d4ffe20b97f9a44043` | Carry the activation height in GenesisState and validate its key coverage | `modules/sequencer/types/genesis.go` | `v7.0` (`MergeUpgrade`) |
| `03b4d03ed189cfb1131f51d0cd055d707171875e` | Add SetLeaders and LeaderSchedule to the keeper | `modules/sequencer/keeper/leaders.go` | `v7.0` (`MergeUpgrade`) |
| `ce5c31a559bafeb74f486bc44f35e67033b33b42` | Refuse a sequence batch below the activation height | `modules/sequencer/keeper/keeper.go` | `v7.0` (`MergeUpgrade`) |
| `249e5fe9a64e65ccc270a10afa86f4ec758fb21c` | Register the governed Leaders param in the sequencer subspace | `modules/sequencer/keeper/leaders_param.go`, `modules/sequencer/keeper/receipt_keys.go` | `v7.0` (`MergeUpgrade`) |
| `b7799a269447da66741a715e8ba8e079cc752ff4` | Give the ledger, auth and agent governance pause scopes their exit methods | `modules/layerxgov/types/proposal.go` | `v7.0` (`MergeUpgrade`) |
| `96d5991376c740b84fd926fd8dc4cc816efce84d` | Refuse RequestFromProgram with LXP_MODULE_DISABLED below the merge plan | `modules/xweb/keeper/program.go` | `v7.0` (`MergeUpgrade`) |
| `3b7f561d3f5ba02f917710408528459271557e97` | Register an invariant that the origin-2 counters match their records | `modules/xweb/keeper/invariants.go` | `v7.0` (`MergeUpgrade`) |
| `ea9995ca1486094abb02da813a6a0a9ccf2c959d` | Add the DenomPaused query rpc and REST route to tokenfactory query.proto | `modules/tokenfactory/keeper/grpc_query.go`, `modules/tokenfactory/types/query.pb.go`, `modules/tokenfactory/types/query.pb.gw.go` | `v7.0` (`MergeUpgrade`) |
| `0f3ada2e3baa9a629fa649af6f4f239ec579208b` | Refuse FulfilProgram with LXP_MODULE_DISABLED below the merge plan | `modules/xweb/keeper/program_fulfil.go` | `v7.0` (`MergeUpgrade`) |
| `f233e9ad028c39a6e5c0aaa9f6c77db482e6a939` | Add the tokenfactory metadata and paused REST bindings without path denoms | `modules/tokenfactory/types/query.pb.go`, `modules/tokenfactory/types/query.pb.gw.go` | `v7.0` (`MergeUpgrade`) |
| `ebc54cdaa6b66f1941f04a2dd41481d179e7acea` | Validate paused_denoms in tokenfactory GenesisState.Validate | `modules/tokenfactory/types/genesis.go` | `v7.0` (`MergeUpgrade`) |
| `bfac2a33d29230533f0a5760a5625bbbd163ad01` | Add GetPausedDenoms iterating the tokenfactory paused&#124; prefix | `modules/tokenfactory/keeper/pause.go` | `v7.0` (`MergeUpgrade`) |
| `89bd84e8e4cca6a2f2675c88b1e8b0d95819781e` | Emit a set_issuer_did event from SetIssuerDid | `modules/tokenfactory/keeper/admins.go`, `modules/tokenfactory/types/events.go` | `v7.0` (`MergeUpgrade`) |
| `4ddb425f08cbb836403c43e97821ebe0f3cd0a87` | Route both bank plan gates through the nil-safe PlanActive helper | `precompiles/bank/bank.go` | `v7.0` (`MergeUpgrade`) |
| `41926e1dde26c3743e2fbc2cae063c691d26918a` | Read a committed origin-2 answer with the kernel committed-read rules | `modules/xweb/keeper/program_result.go` | `v7.0` (`MergeUpgrade`) |
| `93029a000bd67dd668d4dbd83a332e1f0e6141fc` | Refuse SetDenomPaused for a missing or issuer-bound denom | `modules/tokenfactory/keeper/msg_server.go` | `v7.0` (`MergeUpgrade`) |
| `5bc9f8193d85a041d9be6375d75abd534836247c` | Add the MergeRetired gate reader to the layerxanchor keeper package | `modules/layerxanchor/keeper/retire.go` | `v7.0` (`MergeUpgrade`) |
| `2b1f320d917535e3d0c1bb16c1a2c2ee11f927d5` | Add RetireAtMerge returning every guarantor bond and emitting layerx_anchor_retired | `modules/layerxanchor/keeper/retire_merge.go` | `v7.0` (`MergeUpgrade`) |
| `dfb73538cfb662f8ff12a7e9f865912b56c944ea` | Report activation addresses that have no custom precompile | `modules/evm/keeper/keeper.go` | `v7.0` (`MergeUpgrade`) |
| `1352fb25ba427ec90ea5f10497cc1ea9f4e7c3b2` | Validate 0x23 fee-denom genesis entries | `modules/evm/types/fee_denom_genesis.go` | `v7.0` (`MergeUpgrade`) |
| `464fd4cc943930043b9342b3405746c25c57a30a` | Reserve the removed evm Params field numbers and names | `modules/evm/types/params.pb.go` | `v7.0` (`MergeUpgrade`) |
| `5b9c0e9976f38dd370afca14c57269f873d79cb8` | Regenerate the bank query gateway with the by_denom and query-string routes | `sdk/x/bank/types/query.pb.go`, `sdk/x/bank/types/query.pb.gw.go` | `v7.0` (`MergeUpgrade`) |
| `c88d1120fe730b0ac70abc165ddb0cb07eba78e0` | Emit launchpad_market_created when the keeper creates a market | `modules/launchpad/keeper/market.go` | `v7.0` (`MergeUpgrade`) |
| `2fcc434f4929525bbd2163200b9d5469d57dfad4` | Serve Query/Params from the launchpad keeper | `modules/launchpad/keeper/grpc_query.go`, `modules/launchpad/keeper/grpc_query_params.go` | `v7.0` (`MergeUpgrade`) |
| `0f249dbcfd02dce500f2329b7eeae79f13ee7031` | Convert stored scheduler jobs to and from the genesis Job message | `modules/layerxagent/keeper/job_proto.go` | `v7.0` (`MergeUpgrade`) |
| `665104991b08a45a72e55c7ab4d1bb892c63ad25` | Import and export layerxpay genesis with kernel records in the host store | `modules/layerxpay/keeper/genesis.go` | `v7.0` (`MergeUpgrade`) |
| `7fd82d76171c136bac98f43e6ff0773bf2155946` | Serve the layerxpay Params gRPC query | `modules/layerxpay/keeper/grpc_query.go` | `v7.0` (`MergeUpgrade`) |
| `7e4416e6194d9755f6248231f68b9574f76c2740` | Check scoped activities and spend totals in LayerXScopeAuthorization.Accept | `modules/layerxauth/types/authorization.go` | `v7.0` (`MergeUpgrade`) |
| `b1b4fcf05a9477e478e9d074649aa9ef99248fa9` | Bound scheduled calldata and honour the governance pause in the agent keeper | `modules/layerxagent/keeper/keeper.go` | `v7.0` (`MergeUpgrade`) |
| `82a3dc306d4ba59417c872a804ecc318f133a297` | Cap end-block job gas with MaxBlockGas and report the gas through RunDue | `modules/layerxagent/keeper/scheduler.go` | `v7.0` (`MergeUpgrade`) |
| `1bde1e56aeef17a10f4866526ac94c7b54fbdb5f` | Import and export layerxagent genesis with escrow coverage | `modules/layerxagent/keeper/genesis.go` | `v7.0` (`MergeUpgrade`) |
| `3bcb01ac49e365297f89f4f298b8b580d84bb3c1` | Construct the layerxauth keeper over the shared host, authz and EVM keepers | `modules/layerxauth/keeper/keeper.go`, `modules/layerxauth/types/expected_keepers.go` | `v7.0` (`MergeUpgrade`) |
| `b76044b03e3253a3055fcb8bb1fde9842b241ef7` | Route ledger writes through the governance pause scope | `modules/layerxledger/keeper/pause.go` | `v7.0` (`MergeUpgrade`) |
| `6704b360dbf07a6cde3381813c69975c88627783` | Resolve the EVM caller to its bound DID as the auth actor | `modules/layerxauth/keeper/authority.go` | `v7.0` (`MergeUpgrade`) |
| `1132a60e10ffc58c849e6a7dd15e9996d5c409fb` | Route auth grants through the governance pause scope | `modules/layerxauth/keeper/pause.go` | `v7.0` (`MergeUpgrade`) |
| `808b567842555ed16ce7efa156e398a3bb8915cc` | Serve authority views from the kernel host read exports | `modules/layerxauth/keeper/views.go` | `v7.0` (`MergeUpgrade`) |
| `63b91010b8d97a037a0638680525e8eaf40e0583` | Init and export the params-only layerxauth genesis | `modules/layerxauth/keeper/genesis.go` | `v7.0` (`MergeUpgrade`) |
| `1e18de6965ded0c04cc7b2a7b2f933fef8b7c855` | Serve layerxledger MsgUpdateParams from the module authority | `modules/layerxledger/keeper/msg_server.go` | `v7.0` (`MergeUpgrade`) |
| `8a5b0e92dc38734d195c0b982b861766c8b7c35d` | Serve layerxauth MsgUpdateParams from the module authority | `modules/layerxauth/keeper/msg_server.go` | `v7.0` (`MergeUpgrade`) |
| `3ef4c8bcc21bef364c75cc00786a29a6cb741f8d` | Update consensus state and behavior in `modules/spot/types/events.go` | `modules/spot/types/events.go` | `v7.0` (`MergeUpgrade`) |
| `28639ecb2481ec5880597d3a5ab6e0f0b0200cb6` | Retire the custody margin hop of exchange deposit intents | `modules/layerxexchange/keeper/intents.go` | `v7.0` (`MergeUpgrade`) |
| `be067520ed51b30213f962db17bc8eb6a0950f11` | Expose the pinned receipt keys as sequencer authorizations | `modules/sequencer/keeper/pins.go` | `v7.0` (`MergeUpgrade`) |
| `ee0f4b7978d68054a72a22a52c8ecf1054cdace1` | Refuse a governed receipt key entry with firstHeight 0 | `modules/sequencer/keeper/receipt_keys.go` | `v7.0` (`MergeUpgrade`) |
| `4b58c2fa29e47434e283e263fa7986c17a3731ac` | Update consensus state and behavior in `modules/sequencer/keeper/receipt_keys.go` | `modules/sequencer/keeper/receipt_keys.go` | `v7.0` (`MergeUpgrade`) |
| `e52b78a25c59bf4ce49973b27a7aa320aef98c77` | Init and export the activation height and validate the stored leader list | `modules/sequencer/keeper/keeper.go` | `v7.0` (`MergeUpgrade`) |
| `2abaeb529accc9a01337d4ba73e1ecd54af7cfac` | Add issuer-bound SetDenomPausedByIssuer to the tokenfactory keeper | `modules/tokenfactory/keeper/issuer.go` | `v7.0` (`MergeUpgrade`) |
| `5e2c594dda9e9f9b1b62e57caadca5b08ee4353f` | Import and export paused denoms in tokenfactory genesis | `modules/tokenfactory/keeper/genesis.go` | `v7.0` (`MergeUpgrade`) |
| `2965b4a0cc63646927ba47a45bc59b7ede28d850` | Revert a paused-denom bank send with LXP_ASSET_PAUSED | `precompiles/bank/bank.go` | `v7.0` (`MergeUpgrade`) |
| `17d3f96b4c5cb2a0f8051f41cbc7f828fba346a3` | Refuse layerxanchor MsgUpdateParams after the merge plan | `modules/layerxanchor/keeper/msg_server.go` | `v7.0` (`MergeUpgrade`) |
| `5041572c657075b8b82e213180fb288863b17062` | Serve Query/Market from the launchpad keeper | `modules/launchpad/keeper/grpc_query.go`, `modules/launchpad/keeper/grpc_query_market.go` | `v7.0` (`MergeUpgrade`) |
| `2d01fa67ce3bc5917cefeb0b84510d45292966ac` | Add MintByIssuer and BurnByIssuer keeper entry points | `modules/tokenfactory/keeper/issuer_supply.go` | `v7.0` (`MergeUpgrade`) |
| `b15ecf54f76abbb96c24af5df262297ad195cadd` | Register the tokenfactory PausedDenomsInvariant | `modules/tokenfactory/keeper/invariants.go` | `v7.0` (`MergeUpgrade`) |
| `d9207f1a0a34e4e9a5977268e3c7fb0dda217874` | Freeze the IssuerKeeper interface that layerxledger consumes | `modules/tokenfactory/types/expected_issuer.go` | `v7.0` (`MergeUpgrade`) |
| `a8f87bdaf832a86c00e6b619125136d67b129475` | Serve layerxagent MsgUpdateParams and the Params, Job and Pending queries | `modules/layerxagent/keeper/grpc_query.go`, `modules/layerxagent/keeper/msg_server.go` | `v7.0` (`MergeUpgrade`) |
| `6aec577cc26ec867e5bd0c4589ad7728af8d7e1b` | Serve the DenomPaused gRPC query from the tokenfactory keeper | `modules/tokenfactory/keeper/grpc_query.go` | `v7.0` (`MergeUpgrade`) |
| `7f65d97eba2b4015fd2269adb3df80ba023fe3f7` | Serve the Params, Attestors, Request, Result, ProgramRequest, ProgramResult and ProgramCounters queries | `modules/xweb/keeper/grpc_query.go`, `modules/xweb/keeper/grpc_query_program.go`, `modules/xweb/keeper/grpc_query_requests.go` | `v7.0` (`MergeUpgrade`) |
| `0d179df2e7f6f7878b93530ebf0c2a86fb3da6e9` | Serve layerxpay MsgUpdateParams with the governance authority | `modules/layerxpay/keeper/msg_server.go` | `v7.0` (`MergeUpgrade`) |
| `a3997f7c9fb1fe146239f4b3a2a0d226795623ec` | Serve Query/MarketByAddress from the launchpad keeper | `modules/launchpad/keeper/grpc_query.go`, `modules/launchpad/keeper/grpc_query_market_by_address.go` | `v7.0` (`MergeUpgrade`) |
| `75ee11a0746c45f1f8c8d2c5d7e161c506f601b3` | Submit governance authority activities to the kernel host | `modules/layerxauth/keeper/execute.go` | `v7.0` (`MergeUpgrade`) |
| `d36d6f8b401831923ae94573df80792cae3a6273` | Decode authority receipts into the 0x101C event set | `modules/layerxauth/keeper/events.go` | `v7.0` (`MergeUpgrade`) |
| `cd7756d81c272e4a2305577ebb728abc0c850a15` | Submit ledger activities to the kernel host with global sequencing and idempotency | `modules/layerxledger/keeper/execute.go` | `v7.0` (`MergeUpgrade`) |
| `1672f3bdd17da11e8281587bd070925a3913c494` | Revoke grants through governance sub-op 6 and advance the grantor revocation sequence | `modules/layerxauth/keeper/revoke.go` | `v7.0` (`MergeUpgrade`) |
| `63495dc4cea2ead8c744162cbd5a4eb4ef09e170` | Serve Query/Markets from the launchpad keeper | `modules/launchpad/keeper/grpc_query.go`, `modules/launchpad/keeper/grpc_query_markets.go` | `v7.0` (`MergeUpgrade`) |
| `3855070fb660b33b3c7606f95b22e1f9d8f65b03` | Serve Query/MarketsByCreator from the launchpad keeper | `modules/launchpad/keeper/grpc_query.go`, `modules/launchpad/keeper/grpc_query_markets_by_creator.go` | `v7.0` (`MergeUpgrade`) |
| `090ccfed8b139b2fb820e6c86f6fa5aafc72e414` | Grant session keys and successor sessions through governance sub-op 5 | `modules/layerxauth/keeper/sessions.go` | `v7.0` (`MergeUpgrade`) |
| `d3fe72f93dab78e612a4819c6d03dd2506b89a4a` | Grant delegated capabilities through governance sub-op 8 and mirror them as authz grants | `modules/layerxauth/keeper/capability.go` | `v7.0` (`MergeUpgrade`) |
| `cd2946045b4e412232381404d72a3943653930fb` | Mirror kernel asset supply and transfers into bank under the layerx denom | `modules/layerxledger/keeper/mirror.go` | `v7.0` (`MergeUpgrade`) |
| `3eb413bdcaf10847cd4067cc9fe2327e799c1972` | Register, pause and unpause kernel assets from the ledger keeper | `modules/layerxledger/keeper/assets.go` | `v7.0` (`MergeUpgrade`) |
| `5e468268cfbb26ffaa0c3425e9718bbf8c8f5773` | Decode ledger receipts into the 0x101A event set | `modules/layerxledger/keeper/events.go`, `modules/layerxledger/keeper/execute.go` | `v7.0` (`MergeUpgrade`) |
| `cae1a29337cfe9bcc52d1da0f7846828df49d7c8` | Serve Query/PriceHistory from the launchpad keeper | `modules/launchpad/keeper/grpc_query.go`, `modules/launchpad/keeper/grpc_query_price_history.go`, `modules/launchpad/keeper/price_history.go` | `v7.0` (`MergeUpgrade`) |
| `056cf0e3a4ad054a42551527884010a0ef001cb9` | Serve Query/Quote from the launchpad keeper | `modules/launchpad/keeper/grpc_query.go`, `modules/launchpad/keeper/grpc_query_quote.go` | `v7.0` (`MergeUpgrade`) |
| `526e27e77b7a05ecc2f0f0e8703840150780194c` | Serve Query/Treasury from the launchpad keeper | `modules/launchpad/keeper/grpc_query.go`, `modules/launchpad/keeper/grpc_query_treasury.go`, `modules/launchpad/types/query.pb.go` | `v7.0` (`MergeUpgrade`) |
| `c72fcb164ca421f822ba1dd6d2beed6ad587bbb6` | Take the ledger receipt from the kernel call instead of the retired idempotency table | `modules/layerxledger/keeper/execute.go` | `v7.0` (`MergeUpgrade`) |
| `1c5b88c47e2bb102a91e6ee84ec9454d0bf501eb` | Open per-asset kernel accounts and derive their ids | `modules/layerxledger/keeper/accounts.go` | `v7.0` (`MergeUpgrade`) |
| `2a18c62778e56663ee0c3af5c14f918a0565b116` | Issue and revoke payer grants from the ledger keeper | `modules/layerxledger/keeper/grants.go` | `v7.0` (`MergeUpgrade`) |
| `0235f9d56a8c975570bfb6177f915b7dae27d4b8` | Draw against a payer grant with the typed receive form | `modules/layerxledger/keeper/draw.go` | `v7.0` (`MergeUpgrade`) |
| `a66a19cb2cf6214555b09e8073a41e9349f38bb2` | Serve Query/Surface from the launchpad keeper | `modules/launchpad/keeper/grpc_query.go`, `modules/launchpad/keeper/grpc_query_surface.go` | `v7.0` (`MergeUpgrade`) |
| `bccc11e3445afe44d68fbb1caaa3d49186344b4f` | Serve Query/Retired from the launchpad keeper | `modules/launchpad/keeper/grpc_query.go`, `modules/launchpad/keeper/grpc_query_retired.go` | `v7.0` (`MergeUpgrade`) |
| `c0121992c6a63f9e75aab6f610bdf2e46470283f` | Keep the launchpad unknown-market code through mustMarket | `modules/launchpad/keeper/keeper.go` | `v7.0` (`MergeUpgrade`) |
| `43f3d9a1f670c34446af37f426bd2ed8553487a4` | Mint and burn kernel asset units from the ledger keeper | `modules/layerxledger/keeper/supply.go` | `v7.0` (`MergeUpgrade`) |
| `d163405ab30565c6ad47cba9a61b55570637b745` | Send kernel asset units between DID accounts | `modules/layerxledger/keeper/send.go` | `v7.0` (`MergeUpgrade`) |
| `2c48362b3d9cf343da107ca4ef8b032e9e28ac02` | Submit one kernel activity per pay write through HostExecute | `modules/layerxpay/keeper/execute.go` | `v7.0` (`MergeUpgrade`) |
| `1d8ccad987c195983c308451cb65997b1ccc49db` | Serve getBudget and budgetRemaining from kernel records | `modules/layerxpay/keeper/budget_views.go` | `v7.0` (`MergeUpgrade`) |
| `49c795b6810e15bc49c78f3cf228f190ea43c8f4` | Pay an exact amount bound to a derived context hash | `modules/layerxledger/keeper/payexact.go` | `v7.0` (`MergeUpgrade`) |
| `a6ad6456fc063f30626d4c68ca75cd0f4b52166d` | Serve Query/FeeRight from the launchpad keeper | `modules/launchpad/keeper/grpc_query.go`, `modules/launchpad/keeper/grpc_query_fee_right.go` | `v7.0` (`MergeUpgrade`) |
| `9052706882f1da81807db2b36d19d4afa713844b` | Implement escrow open, capture and partial capture in the pay keeper | `modules/layerxpay/keeper/escrow_open.go` | `v7.0` (`MergeUpgrade`) |
| `2229523c95974fa3d619b556408df1e28ec65b14` | Implement escrow release, timeout, dispute open and dispute resolve | `modules/layerxpay/keeper/escrow_settle.go` | `v7.0` (`MergeUpgrade`) |
| `cb9e9e542d59515b21a876bf1e789fa456cfe2c2` | Serve Query/AirdropEpoch from the launchpad keeper | `modules/launchpad/keeper/grpc_query.go`, `modules/launchpad/keeper/grpc_query_airdrop_epoch.go` | `v7.0` (`MergeUpgrade`) |
| `b634665802d566fe53475a7f215b5b824541834c` | Serve Query/AirdropClaimed from the launchpad keeper | `modules/launchpad/keeper/grpc_query.go`, `modules/launchpad/keeper/grpc_query_airdrop_claimed.go` | `v7.0` (`MergeUpgrade`) |
| `21bcb4a1e845296111095441483b9a8629a5f1d0` | Serve Query/HasRole from the launchpad keeper | `modules/launchpad/keeper/grpc_query.go`, `modules/launchpad/keeper/grpc_query_has_role.go` | `v7.0` (`MergeUpgrade`) |
| `7f5939965d3dccc1960969887c45c2dd5c3722bb` | Serve Query/QuoteLedger from the launchpad keeper | `modules/launchpad/keeper/grpc_query.go`, `modules/launchpad/keeper/grpc_query_quote_ledger.go`, `modules/launchpad/types/query.pb.go` | `v7.0` (`MergeUpgrade`) |
| `d15df3bc5ae3c1d9d971bfae3fb356f96a8d4a4b` | Serve Query/FeeAccounting from the launchpad keeper | `modules/launchpad/keeper/grpc_query.go`, `modules/launchpad/keeper/grpc_query_fee_accounting.go` | `v7.0` (`MergeUpgrade`) |
| `6b1f13b477a68cd0509116ebe25017c502618b43` | Serve Query/PoolMetadata from the launchpad keeper | `modules/launchpad/keeper/grpc_query.go`, `modules/launchpad/keeper/grpc_query_pool_metadata.go` | `v7.0` (`MergeUpgrade`) |
| `774754c3b7eee811dc7f02a655a23b78a0ffd0f6` | Implement budget spend, delegate add, delegate remove and delegate spend | `modules/layerxpay/keeper/budget_spend.go` | `v7.0` (`MergeUpgrade`) |
| `1a4559d2d52b54513762fb6f866527fc254fa938` | Implement budget close, defund and revoke | `modules/layerxpay/keeper/budget_close.go` | `v7.0` (`MergeUpgrade`) |
| `8e28788f6f0d1214a802325f836f246a42011a15` | Implement budget create, create native, fund, fund native and amend | `modules/layerxpay/keeper/budget_create.go` | `v7.0` (`MergeUpgrade`) |
| `860148685e13e4cdacf481027e6e72434b4dc9c1` | Route the layerxpay MsgUpdateParams pause through LayerXProposal | `modules/layerxgov/types/proposal.go` | `v7.0` (`MergeUpgrade`) |
| `108ec8345c142434b2881039e92105d48c6a2e0a` | Implement budget pull against a payer grant | `modules/layerxpay/keeper/budget_pull.go` | `v7.0` (`MergeUpgrade`) |
| `a9b0775ee6d75925f48a5092b0e2077334a9cda1` | Route the pay, agent, perps, pricefeed and spot authority messages through layerxgov | `modules/layerxgov/types/proposal.go` | `v7.0` (`MergeUpgrade`) |
| `191413b1190b412298f9e3a0f8e75f0767bce749` | Bind each registered ledger asset to a tokenfactory denom and mirror its pause state | `modules/layerxledger/keeper/issuer_binding.go`, `modules/layerxledger/types/expected_keepers.go` | `v7.0` (`MergeUpgrade`) |
| `c83339b445828f0b324445e6a576d0c9ea3cbd85` | Code bank staticcall and delegatecall refusals | `precompiles/bank/bank.go` | `v7.0` (`MergeUpgrade`) |
| `1782d2d2946ca36285bbbc793aab563b505ebe7b` | Code sendNative zero-value and malformed-receiver refusals | `precompiles/bank/bank.go` | `v7.0` (`MergeUpgrade`) |
| `27b1faa0663e5b6604c39909e63fbd332837c90c` | Retire the launchpad bank balance-change hook and holding history | `modules/launchpad/keeper/` | `v7.0` (`MergeUpgrade`) |
| `ebfccf28bbd098d9dfd6b92316673f32c4db830c` | a layerxledger-registered denom carries its issuer DID and pause. | `modules/layerxledger/keeper/issuer_binding.go` | `v7.0` (`MergeUpgrade`) |
| `94bae91dc5dd8a64a5f7ed8558a93576ebf35869` | Delete the generation 1 key builders and keep their prefixes reserved | `modules/launchpad/types/keys.go` | `v7.0` (`MergeUpgrade`) |
| `5a618ee5ffe0124de05f97c0c598a73fdea991d4` | Preserve registered error codes across AirdropEpoch queries | `modules/launchpad/keeper/grpc_query_airdrop_epoch.go`, `modules/launchpad/types/query_validate.go` | `v7.0` (`MergeUpgrade`) |
| `88337b7fea18868fe8aea0b33406dea977af9471` | Keep a checkpoint's challenge window when the same checkpoint is resubmitted | `modules/layerxanchor/keeper/checkpoints.go` | `v7.0` (`MergeUpgrade`) |
| `6aeef2232251c104911574ae848d528ae37a71ce` | the launchpad registry REST routes against a live network. | `modules/launchpad/keeper/grpc_query.go` | `v7.0` (`MergeUpgrade`) |
| `da64731074fa8042c9203f681838f00c6e790950` | Implement stream open and top up in the pay keeper | `modules/layerxpay/keeper/stream_open.go` | `v7.0` (`MergeUpgrade`) |
| `6c26c977ceeb3323acc2f3c9b46dab509ee2b165` | Implement stream meter, settle and close in the pay keeper | `modules/layerxpay/keeper/stream_settle.go` | `v7.0` (`MergeUpgrade`) |
| `cfdeaa54d239d3ed6c3dcdc19fbf4d5365930745` | Implement stream pause, resume and the kernel rate change | `modules/layerxpay/keeper/stream_lifecycle.go` | `v7.0` (`MergeUpgrade`) |
| `0209504adfba108a8d96a33d66adede2aad0c2e0` | Implement service offer publish and withdraw in the pay keeper | `modules/layerxpay/keeper/service_offer.go` | `v7.0` (`MergeUpgrade`) |
| `b34a90dfa454e00491ffc0d4bcc989c31a116c66` | Implement service agreement propose, accept, commit and abandon | `modules/layerxpay/keeper/service_agreement.go` | `v7.0` (`MergeUpgrade`) |
| `661eaee3c2a751dc00a71ec8e1895e0d72eb3ba4` | Implement service tool attest, progress, deliver, accept and reject | `modules/layerxpay/keeper/service_delivery.go` | `v7.0` (`MergeUpgrade`) |
| `d2518c0a056be13d92fd31f69411f39a8e30941c` | Expose the 0x0906 agent stream API on the pay keeper | `modules/layerxpay/keeper/stream_agent.go` | `v7.0` (`MergeUpgrade`) |
| `90bda704ad9383f8b3a17e81c8b27b319a9d4b81` | Retire Keeper.ResultByIdempotency and the resultByIdempotency precompile view with the idempotency read | `modules/layerxauth/keeper/views.go` | `v7.0` (`MergeUpgrade`) |
| `451c6bee38c8281dc691a2f15382f85e6cafb824` | Move the xweb state-type adapters out of query.pb.go and make the pinned generator's output compile | `modules/xweb/types/codec.go`, `modules/xweb/types/query.pb.go` | `v7.0` (`MergeUpgrade`) |
| `893c100a2d095d1e0cf28bd4c0196cd7412c653a` | Export the oracle observation submit and committed price read on the exchange keeper | `modules/layerxexchange/keeper/oracle.go` | `v7.0` (`MergeUpgrade`) |
| `bee874530d1ec2cec3e6c04ac4539f7ae1291b7e` | Serve bank SupplyOf by_denom first through the generated gateway | `sdk/x/bank/types/query.pb.go`, `sdk/x/bank/types/query.pb.gw.go` | `v7.0` (`MergeUpgrade`) |
| `0264f08036e949b8ce8b1bffb89d64389c889bd9` | Implement service dispute open and resolve with the escrow arbiter link | `modules/layerxpay/keeper/service_dispute.go` | `v7.0` (`MergeUpgrade`) |
| `0ff0f04f134e3edfff3d933d65f6f9072b4202c1` | Apply lapsed service default outcomes on the hosted kernel | `modules/layerxpay/keeper/service_delivery.go` | `v7.0` (`MergeUpgrade`) |
| `afcc3485cd54ec0343286be0502c6799bf5e46d7` | Retire the guarantor checkpoint path of the layerxanchor keeper and precompile | `modules/layerxanchor/keeper/checkpoints.go`, `modules/layerxanchor/keeper/slashing.go` | `v7.0` (`MergeUpgrade`) |
| `deeff8f13fd8a9bd1ff74c2f1b3b079f83884128` | Return LXP_MODULE_DISABLED from an unwired layerxAnchor and drop the keeper AnchorReader | `modules/layerxanchor/keeper/keeper.go` | `v7.0` (`MergeUpgrade`) |
| `984636931a0601069f318fec743edcf3d8e2d38a` | Remove the host payer grant read that the retired kernel grant store backed | `modules/layerxledger/keeper/draw.go`, `modules/layerxledger/keeper/views.go` | `v7.0` (`MergeUpgrade`) |
| `3e43ee21402d5c67755fa22ac5464c7372fa940f` | Add recordLifecycle to the layerxauth precompile so DID and recovery registrations execute through the kernel | `modules/layerxauth/keeper/lifecycle.go` | `v7.0` (`MergeUpgrade`) |
| `f05477cf9f41dd8c954eef0ffce7a591254e6084` | Add the xweb storage-receipt request kind with its digest-and-bucket payload | `modules/xweb/keeper/request.go`, `modules/xweb/types/genesis.go`, `modules/xweb/types/observation.go`, `modules/xweb/types/state.go`, `modules/xweb/types/storage_receipt.go` | `v7.0` (`MergeUpgrade`) |
| `ea92b886a074699c5421592d377d37909a26a396` | Refuse launchpad markets whose optical registry has not approved | `modules/launchpad/keeper/market.go`, `modules/launchpad/types/errors.go` | `v7.0` (`MergeUpgrade`) |
| `1c81ed369131899bd638a564ba09016d21b1d1a4` | Retire budgetPull of the pay precompile with the payer grants | `modules/layerxpay/keeper/budget_pull.go`, `modules/layerxpay/types/errors.go` | `v7.0` (`MergeUpgrade`) |
| `e70ead0fcc86531b22c326857d209a4d70ec220c` | Carry agent stream address metadata through pay genesis export and import | `modules/layerxpay/keeper/genesis.go`, `modules/layerxpay/keeper/stream_agent.go`, `modules/layerxpay/types/keys.go`, `modules/layerxpay/types/params.go` | `v7.0` (`MergeUpgrade`) |
| `cc6f6aa76c30bab774ff41eeeb90905636f689ca` | Drop the cosmos telemetry validator slashed counter from the oracle slash path | `modules/oracle/keeper/slash.go` | `v7.0` (`MergeUpgrade`) |
| `520414c8709f6d98f24b54f7cdd09b15393fee25` | Re-home the availability class count in the layerxanchor keeper after the codec retirement | `modules/layerxanchor/keeper/checkpoints.go` | `v7.0` (`MergeUpgrade`) |
| `ee3dad5f6fa6c323537325037bf2d0b97fc81bc7` | Delete the legacy telemetry calls of bank Send | `sdk/x/bank/keeper/msg_server.go` | `v7.0` (`MergeUpgrade`) |
| `d87b4b63cf8afab070a4c8e6069b266ff25f5c01` | Write the chain registry module: a virtual chain is one record with a lifecycle | `modules/layerxchains/keeper/accounting.go`, `modules/layerxchains/keeper/genesis.go`, `modules/layerxchains/keeper/keeper.go`, `modules/layerxchains/types/domain.go`, `modules/layerxchains/types/errors.go`, `modules/layerxchains/types/genesis.go`, `modules/layerxchains/types/keys.go`, `modules/layerxchains/types/params.go` | `v7.0` (`MergeUpgrade`) |
| `1d41dbfba8f423a960ddfa71d32fe31d6c7285bb` | Map a transaction's chain id to its chain in the ante handler and charge the chain's base fee | `modules/layerxchains/types/context.go` | `v7.0` (`MergeUpgrade`) |
| `40dc3f063452450258755fefe8695e7c8f4ad987` | Integrate wasm | `modules/tokenfactory/types/errors.go` | `v7.0` (`MergeUpgrade`) |
| `7284041939378ef3f3ab79b0ccd29d650d3460cc` | Integrate events | `modules/tokenfactory/keeper/msg_server.go`, `modules/tokenfactory/types/events.go`, `modules/tokenfactory/types/msgs.go` | `v7.0` (`MergeUpgrade`) |
| `749bf68508a6d49d6600aa8cb435b031488457f9` | Integrate sequencer msg | `modules/sequencer/types/batch_tx.go`, `modules/sequencer/types/batch_verify.go`, `modules/sequencer/types/seqhash.go`, `modules/sequencer/types/txroot.go` | `v7.0` (`MergeUpgrade`) |
| `0ef90cf6d600f26df8fdef01f5cc9c1cc741c4da` | Integrate 0x1001 bank | `modules/layerxledger/types/errors.go`, `modules/tokenfactory/types/errors.go`, `precompiles/bank/bank.go` | `v7.0` (`MergeUpgrade`) |
| `f7c73426c258cf8495504f06f945de0d9c26264f` | Update consensus state and behavior in `modules/tokenfactory/types/errors.go` | `modules/tokenfactory/types/errors.go` | `v7.0` (`MergeUpgrade`) |
| `cf984b1b3aab40d35c9adb8818d443cfba3ec795` | Integrate launchpad errors | `modules/launchpad/keeper/migrate_v6120_read.go`, `modules/launchpad/keeper/roles.go`, `modules/launchpad/keeper/surfaces.go`, `modules/launchpad/types/errors.go`, `modules/launchpad/types/math.go`, `modules/launchpad/types/msgs.go`, `modules/launchpad/types/params.go` | `v7.0` (`MergeUpgrade`) |
| `d46de347454e01abc05632be20b41eaefe99ec9f` | Integrate pricefeed errors | `modules/pricefeed/keeper/genesis.go`, `modules/pricefeed/keeper/params.go`, `modules/pricefeed/types/errors.go`, `modules/pricefeed/types/params.go` | `v7.0` (`MergeUpgrade`) |
| `61fe77da09c3cde3d28ae1ede48bee48e52a2009` | Integrate spot params | `modules/spot/types/params.go` | `v7.0` (`MergeUpgrade`) |
| `c56bafbe147d18ccb2844e7389775021273edaf7` | Integrate layerxexchange retirement | `modules/layerxexchange/types/errors.go`, `modules/layerxexchange/types/expected_keepers.go` | `v7.0` (`MergeUpgrade`) |
| `3bdf0d9b55c6824e4db673662777ef7c3953a889` | Integrate issuer DID | `modules/tokenfactory/keeper/admins.go`, `modules/tokenfactory/keeper/issuer.go`, `modules/tokenfactory/keeper/issuer_supply.go` | `v7.0` (`MergeUpgrade`) |
| `aa456c218cfb8cba6edeb4d0b3abf65447b279ba` | Integrate layerxauth genesis | `modules/layerxauth/types/genesis.go` | `v7.0` (`MergeUpgrade`) |
| `3086432776de2bb52ee87ab01fd8dd9d3e31c6e0` | Integrate layerxledger genesis | `modules/layerxledger/types/genesis.go` | `v7.0` (`MergeUpgrade`) |
| `df1bf4458827d38325e6b89c0f8537f89c68a3d9` | Integrate did binding | `modules/layerxauth/keeper/authority.go` | `v7.0` (`MergeUpgrade`) |
| `b4a1f7a4d56a840efc761d0a660100c5c839754a` | Integrate pay genesis | `modules/layerxpay/keeper/genesis.go` | `v7.0` (`MergeUpgrade`) |
| `00d16e86f4e92ab1c3ac33406a06e5ba432fe66e` | Integrate layerxagent module | `modules/layerxagent/keeper/genesis.go`, `modules/layerxagent/keeper/keeper.go`, `modules/layerxagent/keeper/msg_server.go`, `modules/layerxagent/keeper/scheduler.go` | `v7.0` (`MergeUpgrade`) |
| `7f52a980bbcf10ca7c5398947c7b7dfc73728103` | Integrate layerxauth keeper | `modules/layerxauth/keeper/capability.go`, `modules/layerxauth/keeper/events.go`, `modules/layerxauth/keeper/genesis.go`, `modules/layerxauth/keeper/keeper.go`, `modules/layerxauth/keeper/sessions.go`, `modules/layerxauth/keeper/views.go` | `v7.0` (`MergeUpgrade`) |
| `7db3792c61ffd0ede5fe0aff26806d3cd9a2cecc` | Integrate spot storage | `modules/spot/keeper/invariants.go`, `modules/spot/keeper/migrations.go` | `v7.0` (`MergeUpgrade`) |
| `f8d1bdf40825dced9ffcb963ac6bf22871842f12` | Integrate genesis | `modules/tokenfactory/keeper/genesis.go` | `v7.0` (`MergeUpgrade`) |
| `7493c0337c336bfe966053ac269b9722b96d760f` | Integrate go API | `modules/xweb/keeper/activation.go`, `modules/xweb/keeper/genesis.go`, `modules/xweb/keeper/grpc_query.go`, `modules/xweb/keeper/invariants.go`, `modules/xweb/keeper/keeper.go`, `modules/xweb/keeper/program_fulfil.go`, `modules/xweb/keeper/program_result.go` | `v7.0` (`MergeUpgrade`) |
| `fca33d0692a447bb110d22ba59604ebb6ff8ec82` | Integrate msg layerxpay updateparams | `modules/layerxpay/types/codec.go`, `modules/layerxpay/types/msgs.go` | `v7.0` (`MergeUpgrade`) |
| `9796b60cb295d5d4ff1e7d0bc0a05a55730df643` | Integrate pay keeper | `modules/layerxpay/keeper/execute.go`, `modules/layerxpay/keeper/keeper.go` | `v7.0` (`MergeUpgrade`) |
| `3d0a6d7a111bdd5a64d8239b387c070bc88ae2e7` | Integrate layerxledger keeper | `modules/layerxledger/keeper/accounts.go`, `modules/layerxledger/keeper/draw.go`, `modules/layerxledger/keeper/genesis.go`, `modules/layerxledger/keeper/keeper.go`, `modules/layerxledger/keeper/supply.go`, `modules/layerxledger/keeper/views.go` | `v7.0` (`MergeUpgrade`) |
| `5d357e7c5625bcb4508613af6ab1beef437e8e24` | Integrate payload codec | `modules/layerxledger/types/payload.go`, `modules/layerxledger/types/payload_transfer.go` | `v7.0` (`MergeUpgrade`) |
| `2ee5c3ed4813a2192ce8bb4b95490c1d5f2d9522` | Integrate views | `modules/layerxpay/types/records_budget.go`, `modules/layerxpay/types/records_escrow.go`, `modules/layerxpay/types/records_service.go` | `v7.0` (`MergeUpgrade`) |
| `e8556430addd1575b390f676535c9301270966a8` | Express the default chain bond and creation fee in bank base units | `modules/layerxchains/types/params.go` | `v7.0` (`MergeUpgrade`) |
| `ad4a30dfba9d21e541f3f107f251880059ff94f4` | Refuse an exec under a revoked grant with ErrAuthRevoked | `modules/layerxauth/keeper/revoke.go` | `v7.0` (`MergeUpgrade`) |
| `84ed832c374956a3c10b8fe5ce840e7203f3a0e3` | Launchpad module, CLI and airdrop integration. | `modules/launchpad/keeper/grpc_query.go`, `modules/launchpad/keeper/testdata/v6120/holders.json` | `v7.0` (`MergeUpgrade`) |
| `8fc4a07ffd2fe80466fa3d537e571dc3fbac5ae5` | Integrate layerxledger 0x101a | `modules/layerxledger/keeper/grants.go`, `modules/layerxledger/types/errors.go`, `modules/layerxledger/types/payload_transfer.go` | `v7.0` (`MergeUpgrade`) |
| `ccd3b0d9d2af7c9b24f51389fa3343cc13d2fa25` | Retired custody, exchange, verify and anchor methods refuse calls. | `modules/layerxexchange/keeper/intents.go` | `v7.0` (`MergeUpgrade`) |
| `bcc0bd391b71ab444b392955a5d8847ebd32b9f5` | Measure the price aggregation quorum in bonded power instead of operator count | `modules/pricefeed/keeper/aggregate.go` | `v7.0` (`MergeUpgrade`) |
| `ea039363eee9b135bcc504e4c82802cd930c0c8a` | Align the capability replay order, the retired payer-grant reader and the lxphost authority payload version | `modules/layerxauth/keeper/capability.go`, `modules/layerxauth/keeper/revoke.go` | `v7.0` (`MergeUpgrade`) |
| `41642833f92d2e1db4289d7d1589ec246321a8a7` | Update consensus state and behavior in `modules/layerxauth/keeper/execute.go` | `modules/layerxauth/keeper/execute.go` | `v7.0` (`MergeUpgrade`) |
| `9c5024439144bc970f2597336d2bae1ca8a2d521` | Update consensus state and behavior in `modules/layerxexchange/keeper/intents.go` | `modules/layerxexchange/keeper/intents.go`, `modules/layerxexchange/keeper/proofs.go` | `v7.0` (`MergeUpgrade`) |
| `bef9763709488387c696248b64433b28dfd52d33` | Update consensus state and behavior in `modules/xweb/keeper/testdata/kernel-program-vectors.json` | `modules/xweb/keeper/testdata/kernel-program-vectors.json` | `v7.0` (`MergeUpgrade`) |
| `a295064a5f2859ca146e63844b9bb335c3819b71` | Update consensus state and behavior in `modules/oracle/types/ballot.go` | `modules/oracle/types/ballot.go` | `v7.0` (`MergeUpgrade`) |
| `c34e862ea3217d0702c5e8a38249915c56245c10` | Update consensus state and behavior in `modules/epoch/types/epoch.go` | `modules/epoch/types/epoch.go` | `v7.0` (`MergeUpgrade`) |
| `7044ea7c7d5adf431c2c754e20d28564dab0daab` | Update consensus state and behavior in `modules/oracle/keeper/migrations.go` | `modules/oracle/keeper/migrations.go`, `modules/oracle/keeper/testutils/test_utils.go` | `v7.0` (`MergeUpgrade`) |
| `11e410c862cb7735666b3fdab0b09b1530b1138c` | Update consensus state and behavior in `modules/perps/types/genesis.go` | `modules/perps/types/genesis.go`, `modules/perps/types/math.go`, `modules/perps/types/params.go` | `v7.0` (`MergeUpgrade`) |
| `79e6fc4b5d06901e36376d52ff8cadadb4e50c7e` | Update consensus state and behavior in `modules/mint/keeper/keeper.go` | `modules/mint/keeper/keeper.go` | `v7.0` (`MergeUpgrade`) |
| `74b08abddb101496da616c7337d2436500dd77d2` | Update consensus state and behavior in `modules/launchpad/keeper/genesis.go` | `modules/launchpad/keeper/genesis.go`, `modules/launchpad/keeper/keeper.go`, `modules/launchpad/keeper/treasury.go`, `modules/launchpad/types/genesis.go` | `v7.0` (`MergeUpgrade`) |
| `de325b7cc1e392a68052b03b8314d24326f62162` | Update consensus state and behavior in `modules/layerxagent/keeper/genesis.go` | `modules/layerxagent/keeper/genesis.go`, `modules/layerxagent/keeper/scheduler.go`, `modules/layerxagent/keeper/settlement.go` | `v7.0` (`MergeUpgrade`) |
| `cbe9a00f0021605969867b3cd526b52c3ac24c1c` | Update consensus state and behavior in `modules/layerxchains/keeper/genesis.go` | `modules/layerxchains/keeper/genesis.go`, `modules/layerxchains/keeper/genesis_history.go` | `v7.0` (`MergeUpgrade`) |
| `b07c35a4a0954aa58452c550a3f93c547d35cde3` | Update consensus state and behavior in `modules/oracle/keeper/keeper.go` | `modules/oracle/keeper/keeper.go`, `modules/oracle/types/genesis.go`, `modules/oracle/types/genesis.pb.go` | `v7.0` (`MergeUpgrade`) |
| `ca53491da0a001a9824ef8fbd5cda62cada3b0a3` | Update consensus state and behavior in `modules/layerxauth/keeper/capability.go` | `modules/layerxauth/keeper/capability.go` | `v7.0` (`MergeUpgrade`) |
| `76d6a76d08d4e02f5830b7a9b880e90ceec89f4f` | Update consensus state and behavior in `modules/launchpad/keeper/market.go` | `modules/launchpad/keeper/market.go` | `v7.0` (`MergeUpgrade`) |
| `f4054bf811a903c4b80dbe306ea0f1f3e75283bb` | Update consensus state and behavior in `modules/xweb/keeper/program.go` | `modules/xweb/keeper/program.go`, `modules/xweb/types/genesis.go`, `modules/xweb/types/state.go` | `v7.0` (`MergeUpgrade`) |
| `83b5bed9ba339ff5106caf245e22561c5b00b57a` | Update consensus state and behavior in `modules/layerxledger/keeper/send.go` | `modules/layerxledger/keeper/send.go`, `modules/layerxledger/keeper/views.go` | `v7.0` (`MergeUpgrade`) |
| `286991a009f8e8242703ef6ec02510ad1c391ed9` | Update consensus state and behavior in `sdk/x/bank/keeper/genesis.go` | `sdk/x/bank/keeper/genesis.go`, `sdk/x/bank/keeper/genesis_allowlists.go`, `sdk/x/bank/keeper/keeper.go` | `v7.0` (`MergeUpgrade`) |
| `783bc2a27bafff6f2f26b206a548d60ff58ffce0` | Update consensus state and behavior in `modules/layerxauth/types/authorization.go` | `modules/layerxauth/types/authorization.go` | `v7.0` (`MergeUpgrade`) |
| `fee0ca4e4bcc1da73945c6f85c50c3f40d86c351` | Update consensus state and behavior in `modules/layerxbridge/keeper/msg_server.go` | `modules/layerxbridge/keeper/msg_server.go` | `v7.0` (`MergeUpgrade`) |
| `a98dcea91f25eba2fd8cf300d69120cfe239db4a` | Update consensus state and behavior in `modules/layerxpay/keeper/budget_close.go` | `modules/layerxpay/keeper/budget_close.go` | `v7.0` (`MergeUpgrade`) |
| `3c7a854b934f572df180fde7763057f52cdb84e5` | Update consensus state and behavior in `modules/evm/keeper/abci.go` | `modules/evm/keeper/abci.go` | `v7.0` (`MergeUpgrade`) |
| `a17cb9ba4d6e239be488fd347afb3dbb86acea7f` | Update consensus state and behavior in `modules/xweb/keeper/genesis.go` | `modules/xweb/keeper/genesis.go`, `modules/xweb/keeper/invariants.go`, `modules/xweb/keeper/program.go`, `modules/xweb/keeper/program_fulfil.go`, `modules/xweb/keeper/program_result.go` | `v7.0` (`MergeUpgrade`) |
| `1bd0c1f8cb14794c7fef3e94a62c346cb2467a99` | Update consensus state and behavior in `modules/layerxauth/types/authorization.go` | `modules/layerxauth/types/authorization.go`, `modules/layerxauth/types/tx.pb.go` | `v7.0` (`MergeUpgrade`) |
| `a57e4c1e28bfabaf91c9c1ccefcf20cf93c06c1e` | Update consensus state and behavior in `modules/spot/keeper/execute.go` | `modules/spot/keeper/execute.go`, `modules/spot/keeper/genesis.go`, `modules/spot/keeper/msg_server.go`, `modules/spot/keeper/vault.go` | `v7.0` (`MergeUpgrade`) |
| `382416a4291a49fdc2f6d60308a0dec5dda86a2a` | Update consensus state and behavior in `modules/layerxledger/keeper/payexact.go` | `modules/layerxledger/keeper/payexact.go`, `modules/layerxledger/keeper/send.go` | `v7.0` (`MergeUpgrade`) |
| `3c759cc91560ab0c6886087fbbb63a789217e6ff` | Update consensus state and behavior in `modules/spot/keeper/shares.go` | `modules/spot/keeper/shares.go` | `v7.0` (`MergeUpgrade`) |
| `e71fbafd7d70b3245ac18df6a96a96cb41168946` | Update consensus state and behavior in `modules/layerxpay/keeper/budget_views.go` | `modules/layerxpay/keeper/budget_views.go`, `modules/layerxpay/keeper/escrow_receipt.go`, `modules/layerxpay/keeper/execute.go`, `modules/layerxpay/keeper/genesis.go`, `modules/layerxpay/keeper/stream_agent.go`, `modules/layerxpay/types/params.go` | `v7.0` (`MergeUpgrade`) |
| `7f433f63be422eaa7642b89a62283cceed8b82db` | Update consensus state and behavior in `modules/oracle/keeper/` | `modules/oracle/keeper/` | `v7.0` (`MergeUpgrade`) |
| `08d449fa5eb77ea2c98339a9b3f41a3cd90388d2` | Update consensus state and behavior in `modules/oracle/types/ballot.go` | `modules/oracle/types/ballot.go`, `modules/oracle/types/genesis.go` | `v7.0` (`MergeUpgrade`) |
| `8ac5cb3d3629713465b2d96eb227c6e2fdd6a567` | Update consensus state and behavior in `modules/perps/types/` | `modules/perps/types/` | `v7.0` (`MergeUpgrade`) |
| `9912cdac9722fab03b15a7440fc6daec715b894f` | Update consensus state and behavior in `modules/spot/types/genesis.go` | `modules/spot/types/genesis.go` | `v7.0` (`MergeUpgrade`) |
| `19f13759812b6d3c0d81216cbecff3a945ed39f2` | Update consensus state and behavior in `modules/spot/keeper/` | `modules/spot/keeper/` | `v7.0` (`MergeUpgrade`) |
| `ac301c2afc55ac8eab465a2e5e1875350b0095ae` | Update consensus state and behavior in `modules/mint/types/gov.go` | `modules/mint/types/gov.go` | `v7.0` (`MergeUpgrade`) |
| `88aa9fa9d367351fcff871c7552ef2d80f07a275` | Update consensus state and behavior in `modules/layerxexchange/keeper/intents.go` | `modules/layerxexchange/keeper/intents.go`, `modules/layerxexchange/types/expected_keepers.go` | `v7.0` (`MergeUpgrade`) |
| `3e99c03a3003993e85d86272a0384a5b5e2c647b` | Update consensus state and behavior in `sdk/x/bank/types/genesis.go` | `sdk/x/bank/types/genesis.go` | `v7.0` (`MergeUpgrade`) |
| `f8905c4dc728efd52e4d3dba07a373cadc690937` | Update consensus state and behavior in `modules/mint/types/gov.go` | `modules/mint/types/gov.go` | `v7.0` (`MergeUpgrade`) |
| `bd96e1609feea73e92613f9224dcf32a5f7cc1d5` | Update consensus state and behavior in `sdk/x/bank/keeper/invariants.go` | `sdk/x/bank/keeper/invariants.go` | `v7.0` (`MergeUpgrade`) |
| `6bdfb361b640e63d170c899af7c535db8b84ec7a` | Update consensus state and behavior in `modules/tokenfactory/types/msgs.go` | `modules/tokenfactory/types/msgs.go` | `v7.0` (`MergeUpgrade`) |
| `c39e059f0d7a4902971f3bfe25fc79ca2cecf786` | Update consensus state and behavior in `modules/evm/types/codec.go` | `modules/evm/types/codec.go`, `modules/evm/types/genesis.go`, `modules/evm/types/message_send.go` | `v7.0` (`MergeUpgrade`) |
| `722c3c2afa03bc26c1aee8c97d27e7d238a7773d` | Update consensus state and behavior in `modules/xweb/keeper/` | `modules/xweb/keeper/request.go`, `modules/xweb/types/state.go` | `v7.0` (`MergeUpgrade`) |
| `b51172e754760da46359ae3b68f5ca4f8b82f312` | Update consensus state and behavior in `modules/layerxbridge/keeper/bridge.go` | `modules/layerxbridge/keeper/bridge.go`, `modules/layerxbridge/keeper/genesis.go`, `modules/layerxbridge/keeper/sidiora.go` | `v7.0` (`MergeUpgrade`) |
| `3d70f7a0ca12060e2363fa8691c7ceb9e7acee35` | Update consensus state and behavior in `modules/layerxledger/keeper/` | `modules/layerxledger/keeper/` | `v7.0` (`MergeUpgrade`) |
| `9b206de728f588e4b88a6e5a1a7ac8b3132bb52a` | Update consensus state and behavior in `modules/layerxpay/keeper/execute.go` | `modules/layerxpay/keeper/execute.go`, `modules/layerxpay/keeper/genesis.go`, `modules/layerxpay/keeper/service_dispute.go`, `modules/layerxpay/keeper/service_indexes.go`, `modules/layerxpay/keeper/service_views.go` | `v7.0` (`MergeUpgrade`) |
| `82672da364eb2dec17a7b430b81ab05ef5f7cf3d` | Update consensus state and behavior in `modules/launchpad/keeper/airdrop.go` | `modules/launchpad/keeper/airdrop.go`, `modules/launchpad/keeper/custody.go`, `modules/launchpad/keeper/fee_accounting_counts.go`, `modules/launchpad/keeper/feerights.go`, `modules/launchpad/keeper/genesis.go`, `modules/launchpad/keeper/grpc_query_fee_accounting.go`, `modules/launchpad/keeper/migrate_v6120.go`, `modules/launchpad/keeper/surfaces.go` | `v7.0` (`MergeUpgrade`) |
| `662487514516033e2852e8bbc484b7a79942f2e6` | Update consensus state and behavior in `modules/layerxchains/keeper/` | `modules/layerxchains/keeper/` | `v7.0` (`MergeUpgrade`) |
| `09fb44bfa9251241d65178af208c4088cd07a93f` | Update consensus state and behavior in `modules/layerxauth/keeper/` | `modules/layerxauth/keeper/` | `v7.0` (`MergeUpgrade`) |
| `f32bbc1d2a395ffb99a4630c4b92ce0e8deb6639` | Update consensus state and behavior in `modules/evm/types/ethtx/blob_tx.go` | `modules/evm/types/ethtx/blob_tx.go`, `modules/evm/types/ethtx/set_code_tx.go`, `modules/evm/types/ethtx/uint256_validation.go` | `v7.0` (`MergeUpgrade`) |
| `e3428a4176d32f6d778c7b67645e66cbea6ad0de` | Update consensus state and behavior in `modules/layerxagent/keeper/keeper.go` | `modules/layerxagent/keeper/keeper.go`, `modules/layerxagent/keeper/scheduler.go` | `v7.0` (`MergeUpgrade`) |
| `751e6e95c7cc8e82dadbe5f9e78cf003bf15f48b` | Update consensus state and behavior in `modules/layerxauth/types/` | `modules/layerxauth/types/` | `v7.0` (`MergeUpgrade`) |
| `27776cc90141e0b724da1cd2b53509c6d5b761c8` | Update consensus state and behavior in `modules/tokenfactory/keeper/issuer_supply.go` | `modules/tokenfactory/keeper/issuer_supply.go` | `v7.0` (`MergeUpgrade`) |
| `faa0f795f17d9f5e7e27d32ab16dbb67d5bbf7c6` | Serve the account-bearing placeOrder overload of the exchange precompile that the ABI and the agent SDKs already define | `modules/layerxexchange/keeper/intents.go`, `modules/layerxexchange/keeper/order_accounts.go` | `v7.0` (`MergeUpgrade`) |
| `a8f61f1233fca9d7ac8055bdc89883161980b823` | Update consensus state and behavior in `modules/xweb/types/errors.go` | `modules/xweb/types/errors.go`, `modules/xweb/types/genesis.go` | `v7.0` (`MergeUpgrade`) |
| `dcc500a07bba813952aaaca374b1be512f62fc26` | Update consensus state and behavior in `modules/layerxpay/keeper/budget_views.go` | `modules/layerxpay/keeper/budget_views.go`, `modules/layerxpay/keeper/escrow_views.go`, `modules/layerxpay/keeper/genesis.go`, `modules/layerxpay/keeper/service_dispute.go` | `v7.0` (`MergeUpgrade`) |
| `a874cef595c32c0c3c0703580d764a2903395076` | Update consensus state and behavior in `modules/evm/keeper/genesis.go` | `modules/evm/keeper/genesis.go`, `modules/evm/keeper/receipt.go`, `modules/evm/keeper/view.go` | `v7.0` (`MergeUpgrade`) |
| `3c39be6047a5ab1407a546feb04eee442f04cc2e` | Update consensus state and behavior in `modules/launchpad/keeper/custody.go` | `modules/launchpad/keeper/custody.go`, `modules/launchpad/keeper/fees.go`, `modules/launchpad/types/genesis.go` | `v7.0` (`MergeUpgrade`) |
| `65d57338b5f4b367e38701f8ee6bba2d7cd31ee5` | Update consensus state and behavior in `modules/layerxpay/types/params.go` | `modules/layerxpay/types/params.go` | `v7.0` (`MergeUpgrade`) |
| `47e3f12fba6f8015d788bd5f63e0c6da1b296452` | Admit the compiler's key-rotation payload in the disclosure binder and the auth module | `modules/layerxauth/keeper/lifecycle.go` | `v7.0` (`MergeUpgrade`) |
| `d7e7f8e8394e31e972d3cd3eb05bd3a2b1ff45a9` | Name the replay bound and the guest fault with distinct kernel status codes | `modules/xweb/types/` | `v7.0` (`MergeUpgrade`) |
| `41d32cc012181c9d27889efabefa64e2170c3882` | Bind ask funding to a metered stream whose meter authority is the room host | `modules/layerxpay/keeper/stream_open.go` | `v7.0` (`MergeUpgrade`) |
| `a3e2fab170a326ac62cfc8270adab4b0c78ff80d` | Register the fee-token rate refusals of the EVM keeper so the ABCI response carries their codespace, code and message | `modules/evm/keeper/params.go`, `modules/evm/types/errors.go` | `v7.0` (`MergeUpgrade`) |
| `854460656d1dd300c7274020c09b19ef1634fd6f` | Wrap the fee-token rate errors with sdkerrors so their ABCI code survives and assert the code through the producer function | `modules/evm/keeper/params.go` | `v7.0` (`MergeUpgrade`) |
| `9b8c6465b5c1d251ebe8960fc376b44f5930195b` | Update consensus state and behavior in `modules/evm/keeper/msg_server.go` | `modules/evm/keeper/msg_server.go`, `modules/evm/keeper/receipt.go` | `v7.0` (`MergeUpgrade`) |
| `fabde5e945017f87d4ce1fa9249c1e13e041e60e` | Update consensus state and behavior in `modules/layerxauth/keeper/` | `modules/layerxauth/keeper/` | `v7.0` (`MergeUpgrade`) |
| `ecb298987c82684da8542a69b0e27e46736fbd66` | Preserve canonical escrow receipts across both genesis recovery paths | `modules/layerxpay/keeper/genesis.go`, `modules/layerxpay/types/params.go` | `v7.0` (`MergeUpgrade`) |
| `de7511549a3be441303123c03f8d1d24fe93e6e8` | Update consensus state and behavior in `modules/layerxpay/keeper/` | `modules/layerxpay/keeper/service_dispute.go` | `v7.0` (`MergeUpgrade`) |
| `84504358384d1557650571104862462fd9b8f0b4` | Update consensus state and behavior in `modules/layerxanchor/keeper/custody_reader.go` | `modules/layerxanchor/keeper/custody_reader.go` | `v7.0` (`MergeUpgrade`) |
| `60410908dc7b4b8165fec3c805a7f39fe231c433` | Update consensus state and behavior in `modules/layerxexchange/types/state.go` | `modules/layerxexchange/types/state.go` | `v7.0` (`MergeUpgrade`) |
| `0ca0b687fb62b0f1c7767b807e086da83e291bb7` | Update consensus state and behavior in `modules/layerxpay/keeper/genesis.go` | `modules/layerxpay/keeper/genesis.go`, `modules/layerxpay/types/params.go` | `v7.0` (`MergeUpgrade`) |
| `23b34f1426b8aa95fe8c7a9e8d829a4a46d65d41` | Give the pay module its stream ask activity entry and a canonical escrow genesis fixture | `modules/layerxpay/types/activity.go` | `v7.0` (`MergeUpgrade`) |
| `68964425e461281ea0b42fde1aebb34b908495a8` | Update consensus state and behavior in `modules/launchpad/keeper/migrate_v6120.go` | `modules/launchpad/keeper/migrate_v6120.go` | `v7.0` (`MergeUpgrade`) |
| `b9624c0e1cecfadfce61626bc5277fae6e6fa082` | Restore the launchpad query validation text on the client paths | `modules/launchpad/keeper/grpc_query.go` | `v7.0` (`MergeUpgrade`) |
| `839d7e24961f2c06aa1cad130ceadd0779969d41` | Update consensus state and behavior in `modules/evm/keeper/keeper.go` | `modules/evm/keeper/keeper.go` | `v7.0` (`MergeUpgrade`) |
| `cb32073812242e9589573491a8f70836d31b3012` | Update consensus state and behavior in `modules/launchpad/types/` | `modules/launchpad/types/` | `v7.0` (`MergeUpgrade`) |
| `3b5ff586a4e018a6ec7d24f057cc1d149e791e89` | Update consensus state and behavior in `modules/xweb/keeper/program.go` | `modules/xweb/keeper/program.go` | `v7.0` (`MergeUpgrade`) |
| `40a8dd4d275db5ce6d322e03c142b85a820ca4ef` | End the node process after its close sequence when the termination signal arrives in block sync | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `a3b79971b83cebc10392f83747a97c1d2ac68628` | Run the block-end occupancy transition of Programs in the block flow of the node | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `733d92802e9d4b014c38705ffa07e75e0bdb4f57` | Add the records of the chain launch to the launchpad module | `modules/launchpad/keeper/chainlaunch_store.go`, `modules/launchpad/keeper/genesis.go`, `modules/launchpad/types/chainlaunch.go`, `modules/launchpad/types/errors.go`, `modules/launchpad/types/events.go`, `modules/launchpad/types/genesis.go`, `modules/launchpad/types/keys.go` | `v7.0` (`MergeUpgrade`) |
| `362f167eb8aefd26804025a3860c202e2179f222` | Launch a chain and its market in one state transition of the launchpad keeper | `modules/launchpad/keeper/chainlaunch.go`, `modules/launchpad/keeper/keeper.go`, `modules/launchpad/keeper/market.go`, `modules/launchpad/types/expected_keepers.go`, `modules/layerxchains/keeper/keeper.go`, `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `aadf020a00b7d348e8395e429acf7eab775ee663` | Pay the operator share of a launched chain to the holders of its token by epoch | `modules/launchpad/keeper/airdrop.go`, `modules/launchpad/keeper/chainrevenue.go`, `modules/launchpad/keeper/custody.go` | `v7.0` (`MergeUpgrade`) |
| `2b2ee13ac30038f75f0f7dba81a3872e886b1ea6` | Let the holder of the fee right pause, resume and retire the launched chain | `modules/launchpad/keeper/chaincontrol.go`, `modules/launchpad/keeper/feerights.go`, `modules/launchpad/types/expected_keepers.go`, `modules/layerxchains/keeper/keeper.go` | `v7.0` (`MergeUpgrade`) |
| `7659fd0a0d2bac04ae4d648840d05efa5dff1727` | Bind the trades of a market to its home chain | `modules/launchpad/keeper/homechain.go`, `modules/launchpad/keeper/market.go`, `modules/launchpad/keeper/swap.go`, `modules/layerxchains/types/context.go` | `v7.0` (`MergeUpgrade`) |
| `a88bcef98913a2d938d6ac88e90ef5d9e9f55ea5` | Check the occupancy debit of a program call at the call | `node/app.go` | `v7.0` (`MergeUpgrade`) |
| `1454eb8efd887404ce280227ac20d5c4c09d8444` | Update consensus state and behavior in `programs/crates/layerx-programs-market/src/arbitration.rs` | `programs/crates/layerx-programs-market/src/arbitration.rs`, `programs/crates/layerx-programs-market/src/attest.rs`, `programs/crates/layerx-programs-market/src/lib.rs`, `programs/crates/layerx-programs-market/src/settle.rs`, `programs/crates/layerx-programs-market/src/stake.rs` | `v7.0` (`MergeUpgrade`) |

## v6.12.0 ledger

The earlier plan is `V6120Upgrade` in
[`node/upgrades_v6120.go`](../../node/upgrades_v6120.go). These rows preserve
its coordinated upgrade record.

| Full commit hash | State-machine change | Non-test consensus paths | Coordinated upgrade |
| --- | --- | --- | --- |
| `0b48316e2eea28256461f1a1a99c753b2666f98c` | Whole-position closes pass the exact position base size to `sizeOrder` and simulate the same sizing in quotes, preventing an unaligned mark from leaving one lot open. | `modules/perps/keeper/orders.go`, `modules/perps/keeper/quotes.go`, `precompiles/perps/positions.go` | v6.12.0 through `V6120Upgrade`, which first adds the perps store and initializes its state. |
| `cdd97253c4219a77363a62377c36d212cda5a7ba` | Refuse a plain usage commit for a fraud-provable compute lease; its claim must use sandbox billing with challengeable profile and billing rows. | `programs/crates/layerx-programs-market/src/lib.rs`, `programs/crates/layerx-programs-market/src/settle.rs` | v6.12.0 coordinated binary upgrade through `V6120Upgrade`; all nodes use the same compute-market rules after restart. |
| `97d256444123c41a8db5c9b3d0c4ef2d05df4e0f` | Anchor forced exits to the first live exit checkpoint and refuse ordinary withdrawals proved in later batches, including after the emergency clears. | `modules/layerxcustody/keeper/claims.go`, `modules/layerxcustody/types/errors.go` | v6.12.0 coordinated binary upgrade through `V6120Upgrade`; all nodes switch custody claim and withdrawal rules together. |
| `68c8887a9520542d2878da3d6a44920ddd71b4b1` | Persist the exit checkpoint under its own key, rebuild it on cancellation and genesis, and refuse deposits while a forced exit is live. | `modules/layerxcustody/keeper/claims.go`, `modules/layerxcustody/keeper/deposit.go`, `modules/layerxcustody/keeper/genesis.go`, `modules/layerxcustody/types/errors.go`, `modules/layerxcustody/types/keys.go` | v6.12.0 coordinated binary upgrade through `V6120Upgrade`; all nodes switch checkpoint storage and deposit refusal rules together. |

## Coverage check

`python3 tools/ci/consensus_commit_gate.py <base> <head>` checks consensus-changing commits in the base-to-head range against the table at the head revision. It covers `CONSENSUS_PATHS` and native module keeper/type paths, excludes `_test.go` files, and reports unlisted commits with their paths.

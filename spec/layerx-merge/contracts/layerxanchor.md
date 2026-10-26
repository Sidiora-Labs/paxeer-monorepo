# Layerxanchor contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## bond-return

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.6.evt-layerxanchor-anchor-retired-j35</code> | <code>event</code> | <code>new</code> | <code>modules/layerxanchor/types/events.go</code> (planned) | <code>layerx_anchor_retired { returned_bond }</code> |
| <code>interface.6.go-layerxanchor-retireatmerge-j35</code> | <code>go_exported</code> | <code>new</code> | <code>modules/layerxanchor/keeper/retire_merge.go</code> (planned) | <code>func (k Keeper) RetireAtMerge(ctx sdk.Context) (sdk.Int, error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.6.evt-layerxanchor-anchor-retired-j35</code> | notes | emitted by RetireAtMerge |
| <code>interface.6.go-layerxanchor-retireatmerge-j35</code> | notes | called by the merge handler after wiring |

Contract context <code>contract_context.6.layerxanchor-retirement-j35</code>:

| Field | Frozen rule |
|---|---|
| interface | layerxAnchor precompile 0x1014 (precompiles/layerxanchor/abi.json: 12 writes, 9 reads, 12 events), layerxanchor Msg UpdateParams (api/layerxanchor/tx.proto:5), keeper RetireAtMerge and MergeRetired |
| storage | layerxanchor store prefixes 0x01..0x0e (modules/layerxanchor/types/keys.go:13-26), served read-only between the merge plan and StoreDeleteUpgrade, then deleted |
| errors | Precompile: Error(string) LXP_MODULE_DISABLED "layerxAnchor retired" (precompiles/common/retire.go:34); module: planned ErrModuleRetired 19 LXP_MODULE_DISABLED |
| events | No 0x1014 log after the merge plan; one layerx_anchor_retired { returned_bond } at the merge height |
| gas | Retirement gate reads the plan without charging gas (precompiles/common/retire.go:28), so gas below the plan height equals the Gas formula at precompiles/layerxanchor/layerxanchor.go:162 |
| activation | Writes and MsgUpdateParams refused from the MergeUpgrade v7.0 done height; reads refused from the v7.0-anchor-store-delete done height (decision.629); address stays registered (precompiles/setup.go:180) |
| invariants | BalanceInvariant (modules/layerxanchor/keeper/genesis.go:94) holds at every height; zero escrow after RetireAtMerge; no anchor state changes after the merge height |
| pause | No pause scope: retirement replaces pause for this module |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.6.285.1</code> | req | 6285.1 |
| <code>acceptance.6.285.1</code> | scenario | Given an anchor keeper with three bonded guarantors and one open challenge, when RetireAtMerge runs, then every operator and the challenger are refunded, Escrowed returns zero, BalanceInvariant holds and exactly one layerx_anchor_retired event carries the total; a second RetireAtMerge returns zero and emits a second event with returned_bond 0 |
| <code>acceptance.6.285.1</code> | test | modules/layerxanchor/keeper/retire_merge_test.go::TestRetireAtMergeReturnsBondsAndEmits (planned) |
| <code>acceptance.6.285.1</code> | qualification | UNRUN |
| <code>req.6285</code> | ac_1 | RetireAtMerge(ctx) in modules/layerxanchor/keeper/retire_merge.go (planned) calls ReturnAllBonds (modules/layerxanchor/keeper/bond_return.go:8), leaves ModuleBalance (modules/layerxanchor/keeper/genesis.go:28) at zero escrow and emits one layerx_anchor_retired event whose returned_bond attribute is the refunded total. |
| <code>acceptance.6.286.1</code> | req | 6286.1 |
| <code>acceptance.6.286.1</code> | scenario | Given the integrated app with bonded guarantors and a valid merge plan info, when the v7.0 upgrade handler runs, then every guarantor bond is back with its operator, Escrowed is zero and one layerx_anchor_retired event is emitted |
| <code>acceptance.6.286.1</code> | test | modules/layerxanchor/keeper/merge_handler_test.go::TestMergeHandlerRetiresAnchor (planned) |
| <code>acceptance.6.286.1</code> | qualification | UNRUN |
| <code>req.6286</code> | ac_1 | The merge upgrade handler mergeUpgradePlan (node/upgrades.go:432) calls RetireAtMerge after RunMigrations (node/upgrades.go:443), so applying the v7.0 plan refunds every bond and an error from RetireAtMerge stops the plan. |
| <code>capability.6.33.1</code> | title | Add RetireAtMerge returning every guarantor bond and emitting layerx_anchor_retired |
| <code>capability.6.33.1</code> | reqs | 6285.1 |
| <code>capability.6.33.2</code> | title | Prove the merge upgrade handler returns every anchor bond |
| <code>capability.6.33.2</code> | reqs | 6286.1 |

## errors

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.6.err-sdk-layerxanchor-19-module-retired-j35</code> | <code>error_code</code> | <code>new</code> | <code>modules/layerxanchor/types/errors.go</code> (planned) | <code>ErrModuleRetired = Register("layerxanchor", 19, "LXP_MODULE_DISABLED: layerxanchor retired")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.6.err-sdk-layerxanchor-19-module-retired-j35</code> | notes | planned |

Contract context <code>contract_context.6.layerxanchor-retirement-j35</code>:

| Field | Frozen rule |
|---|---|
| interface | layerxAnchor precompile 0x1014 (precompiles/layerxanchor/abi.json: 12 writes, 9 reads, 12 events), layerxanchor Msg UpdateParams (api/layerxanchor/tx.proto:5), keeper RetireAtMerge and MergeRetired |
| storage | layerxanchor store prefixes 0x01..0x0e (modules/layerxanchor/types/keys.go:13-26), served read-only between the merge plan and StoreDeleteUpgrade, then deleted |
| errors | Precompile: Error(string) LXP_MODULE_DISABLED "layerxAnchor retired" (precompiles/common/retire.go:34); module: planned ErrModuleRetired 19 LXP_MODULE_DISABLED |
| events | No 0x1014 log after the merge plan; one layerx_anchor_retired { returned_bond } at the merge height |
| gas | Retirement gate reads the plan without charging gas (precompiles/common/retire.go:28), so gas below the plan height equals the Gas formula at precompiles/layerxanchor/layerxanchor.go:162 |
| activation | Writes and MsgUpdateParams refused from the MergeUpgrade v7.0 done height; reads refused from the v7.0-anchor-store-delete done height (decision.629); address stays registered (precompiles/setup.go:180) |
| invariants | BalanceInvariant (modules/layerxanchor/keeper/genesis.go:94) holds at every height; zero escrow after RetireAtMerge; no anchor state changes after the merge height |
| pause | No pause scope: retirement replaces pause for this module |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.6.277.1</code> | req | 6277.1 |
| <code>acceptance.6.277.1</code> | scenario | Given the layerxanchor types package, when ErrModuleRetired is inspected, then its codespace is layerxanchor, its ABCI code is 19 and its text begins LXP_MODULE_DISABLED |
| <code>acceptance.6.277.1</code> | test | modules/layerxanchor/types/errors_retired_test.go::TestErrModuleRetiredCode19 (planned) |
| <code>acceptance.6.277.1</code> | qualification | UNRUN |
| <code>req.6277</code> | ac_1 | layerxanchor registers ErrModuleRetired = sdkerrors.Register(ModuleName, 19, "LXP_MODULE_DISABLED: layerxanchor retired") after ErrNotFinalizable code 18 (modules/layerxanchor/types/errors.go:22). |
| <code>capability.6.31.1</code> | title | Register layerxanchor ErrModuleRetired code 19 carrying LXP_MODULE_DISABLED |
| <code>capability.6.31.1</code> | reqs | 6277.1 |

## go-api

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.6.go-layerxanchor-keeper-balanceinvariant-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/genesis.go:94</code> | <code>func BalanceInvariant(k Keeper) sdk.Invariant</code> |
| <code>interface.6.go-layerxanchor-keeper-custodyanchor-finalizedcheckpoint-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/custody_reader.go:56</code> | <code>func (a CustodyAnchor) FinalizedCheckpoint(ctx sdk.Context, checkpointID [32]byte) (custodytypes.FinalizedCheckpoint, bool)</code> |
| <code>interface.6.go-layerxanchor-keeper-custodyanchor-finalizedreceiptroot-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/custody_reader.go:50</code> | <code>func (a CustodyAnchor) FinalizedReceiptRoot(ctx sdk.Context, batchNumber uint64) ([32]byte, bool)</code> |
| <code>interface.6.go-layerxanchor-keeper-custodyanchor-finalizedstateroot-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/custody_reader.go:46</code> | <code>func (a CustodyAnchor) FinalizedStateRoot(ctx sdk.Context, batchNumber uint64) ([32]byte, bool)</code> |
| <code>interface.6.go-layerxanchor-keeper-custodyanchor-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/custody_reader.go:38</code> | <code>type CustodyAnchor struct{ k Keeper }</code> |
| <code>interface.6.go-layerxanchor-keeper-custodyanchor-latestfinalizedbatch-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/custody_reader.go:71</code> | <code>func (a CustodyAnchor) LatestFinalizedBatch(ctx sdk.Context) (uint64, int64, bool)</code> |
| <code>interface.6.go-layerxanchor-keeper-custodyanchor-sequencerauthorization-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/custody_reader.go:42</code> | <code>func (a CustodyAnchor) SequencerAuthorization(ctx sdk.Context, batchNumber uint64) (verify.SequencerAuthorization, bool)</code> |
| <code>interface.6.go-layerxanchor-keeper-keeper-activateguarantor-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/guarantors.go:105</code> | <code>func (k Keeper) ActivateGuarantor(ctx sdk.Context, authority sdk.AccAddress, id [32]byte) error</code> |
| <code>interface.6.go-layerxanchor-keeper-keeper-beginunbond-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/guarantors.go:156</code> | <code>func (k Keeper) BeginUnbond(ctx sdk.Context, operator sdk.AccAddress, id [32]byte, amount sdk.Int) (types.UnbondingEntry, error)</code> |
| <code>interface.6.go-layerxanchor-keeper-keeper-checkpointbyid-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/checkpoints.go:33</code> | <code>func (k Keeper) CheckpointByID(ctx sdk.Context, checkpointID [32]byte) (types.Checkpoint, bool)</code> |
| <code>interface.6.go-layerxanchor-keeper-keeper-completeunbond-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/guarantors.go:183</code> | <code>func (k Keeper) CompleteUnbond(ctx sdk.Context, operator sdk.AccAddress, id [32]byte) (sdk.Int, error)</code> |
| <code>interface.6.go-layerxanchor-keeper-keeper-eligiblesigner-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/guarantors.go:53</code> | <code>func (k Keeper) EligibleSigner(ctx sdk.Context, params types.Params) func([32]byte, uint64) ([20]byte, bool)</code> |
| <code>interface.6.go-layerxanchor-keeper-keeper-escrowed-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/genesis.go:12</code> | <code>func (k Keeper) Escrowed(ctx sdk.Context) sdk.Int</code> |
| <code>interface.6.go-layerxanchor-keeper-keeper-exportgenesis-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/genesis.go:75</code> | <code>func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState</code> |
| <code>interface.6.go-layerxanchor-keeper-keeper-finalize-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/checkpoints.go:221</code> | <code>func (k Keeper) Finalize(ctx sdk.Context, batchNumber uint64) (types.Checkpoint, error)</code> |
| <code>interface.6.go-layerxanchor-keeper-keeper-finalizedreceiptroot-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/keeper.go:204</code> | <code>func (k Keeper) FinalizedReceiptRoot(ctx sdk.Context, batchNumber uint64) ([32]byte, bool)</code> |
| <code>interface.6.go-layerxanchor-keeper-keeper-finalizedstateroot-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/keeper.go:196</code> | <code>func (k Keeper) FinalizedStateRoot(ctx sdk.Context, batchNumber uint64) ([32]byte, bool)</code> |
| <code>interface.6.go-layerxanchor-keeper-keeper-getanchor-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/keeper.go:98</code> | <code>func (k Keeper) GetAnchor(ctx sdk.Context) types.Anchor</code> |
| <code>interface.6.go-layerxanchor-keeper-keeper-getavailability-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/checkpoints.go:234</code> | <code>func (k Keeper) GetAvailability(ctx sdk.Context) []types.AvailabilityAttestation</code> |
| <code>interface.6.go-layerxanchor-keeper-keeper-getchallenge-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/challenges.go:12</code> | <code>func (k Keeper) GetChallenge(ctx sdk.Context, id uint64) (types.Challenge, bool)</code> |
| <code>interface.6.go-layerxanchor-keeper-keeper-getchallenges-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/challenges.go:18</code> | <code>func (k Keeper) GetChallenges(ctx sdk.Context) []types.Challenge</code> |
| <code>interface.6.go-layerxanchor-keeper-keeper-getcheckpoint-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/checkpoints.go:14</code> | <code>func (k Keeper) GetCheckpoint(ctx sdk.Context, batchNumber uint64) (types.Checkpoint, bool)</code> |
| <code>interface.6.go-layerxanchor-keeper-keeper-getcheckpoints-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/checkpoints.go:45</code> | <code>func (k Keeper) GetCheckpoints(ctx sdk.Context) []types.Checkpoint</code> |
| <code>interface.6.go-layerxanchor-keeper-keeper-getguarantor-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/guarantors.go:11</code> | <code>func (k Keeper) GetGuarantor(ctx sdk.Context, id [32]byte) (types.Guarantor, bool)</code> |
| <code>interface.6.go-layerxanchor-keeper-keeper-getguarantors-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/guarantors.go:21</code> | <code>func (k Keeper) GetGuarantors(ctx sdk.Context) []types.Guarantor</code> |
| <code>interface.6.go-layerxanchor-keeper-keeper-getparams-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/keeper.go:63</code> | <code>func (k Keeper) GetParams(ctx sdk.Context) types.Params</code> |
| <code>interface.6.go-layerxanchor-keeper-keeper-getsequencerauthorizations-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/keeper.go:123</code> | <code>func (k Keeper) GetSequencerAuthorizations(ctx sdk.Context) []types.SequencerAuthorization</code> |
| <code>interface.6.go-layerxanchor-keeper-keeper-getslashrecords-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/slashing.go:13</code> | <code>func (k Keeper) GetSlashRecords(ctx sdk.Context) []types.SlashRecord</code> |
| <code>interface.6.go-layerxanchor-keeper-keeper-getunbondings-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/guarantors.go:34</code> | <code>func (k Keeper) GetUnbondings(ctx sdk.Context) []types.UnbondingEntry</code> |
| <code>interface.6.go-layerxanchor-keeper-keeper-increasebond-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/guarantors.go:134</code> | <code>func (k Keeper) IncreaseBond(ctx sdk.Context, operator sdk.AccAddress, id [32]byte, amount sdk.Int) (types.Guarantor, error)</code> |
| <code>interface.6.go-layerxanchor-keeper-keeper-initgenesis-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/genesis.go:32</code> | <code>func (k Keeper) InitGenesis(ctx sdk.Context, genesis types.GenesisState)</code> |
| <code>interface.6.go-layerxanchor-keeper-keeper-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/keeper.go:17</code> | <code>type Keeper struct</code> |
| <code>interface.6.go-layerxanchor-keeper-keeper-latestfinalizedbatch-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/keeper.go:212</code> | <code>func (k Keeper) LatestFinalizedBatch(ctx sdk.Context) (uint64, bool)</code> |
| <code>interface.6.go-layerxanchor-keeper-keeper-moduleaddress-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/keeper.go:30</code> | <code>func (k Keeper) ModuleAddress() sdk.AccAddress</code> |
| <code>interface.6.go-layerxanchor-keeper-keeper-modulebalance-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/genesis.go:28</code> | <code>func (k Keeper) ModuleBalance(ctx sdk.Context) sdk.Int</code> |
| <code>interface.6.go-layerxanchor-keeper-keeper-openchallenge-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/challenges.go:33</code> | <code>func (k Keeper) OpenChallenge(ctx sdk.Context, challenger sdk.AccAddress, batchNumber uint64, kind uint8,</code> |
| <code>interface.6.go-layerxanchor-keeper-keeper-registerguarantor-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/guarantors.go:70</code> | <code>func (k Keeper) RegisterGuarantor(ctx sdk.Context, operator sdk.AccAddress, id [32]byte, signer [20]byte, amount sdk.Int) (types.Guarantor, error)</code> |
| <code>interface.6.go-layerxanchor-keeper-keeper-resolvechallenge-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/challenges.go:70</code> | <code>func (k Keeper) ResolveChallenge(ctx sdk.Context, authority sdk.AccAddress, id uint64, upheld bool) (types.Challenge, []types.SlashRecord, error)</code> |
| <code>interface.6.go-layerxanchor-keeper-keeper-returnallbonds-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/bond_return.go:8</code> | <code>func (k Keeper) ReturnAllBonds(ctx sdk.Context) (sdk.Int, error)</code> |
| <code>interface.6.go-layerxanchor-keeper-keeper-sequencerauthorization-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/keeper.go:162</code> | <code>func (k Keeper) SequencerAuthorization(ctx sdk.Context, sequencerID [32]byte, batchNumber uint64) (verify.SequencerAuthorization, bool)</code> |
| <code>interface.6.go-layerxanchor-keeper-keeper-sequencerauthorizationforbatch-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/custody_reader.go:15</code> | <code>func (k Keeper) SequencerAuthorizationForBatch(ctx sdk.Context, batchNumber uint64) (verify.SequencerAuthorization, bool)</code> |
| <code>interface.6.go-layerxanchor-keeper-keeper-setparams-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/keeper.go:71</code> | <code>func (k Keeper) SetParams(ctx sdk.Context, params types.Params) error</code> |
| <code>interface.6.go-layerxanchor-keeper-keeper-setsequencerauthorization-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/keeper.go:142</code> | <code>func (k Keeper) SetSequencerAuthorization(ctx sdk.Context, authority sdk.AccAddress, authorization types.SequencerAuthorization) error</code> |
| <code>interface.6.go-layerxanchor-keeper-keeper-statusof-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/keeper.go:221</code> | <code>func (k Keeper) StatusOf(ctx sdk.Context, batchNumber uint64) uint8</code> |
| <code>interface.6.go-layerxanchor-keeper-keeper-submitavailabilityattestation-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/checkpoints.go:287</code> | <code>func (k Keeper) SubmitAvailabilityAttestation(ctx sdk.Context, encoded []byte) (types.Checkpoint, types.AvailabilityAttestation, error)</code> |
| <code>interface.6.go-layerxanchor-keeper-keeper-submitcheckpoint-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/checkpoints.go:99</code> | <code>func (k Keeper) SubmitCheckpoint(ctx sdk.Context, submitter sdk.AccAddress, header []byte, headerSignature [64]byte, certificateBytes []byte) (types.Checkpoint, error)</code> |
| <code>interface.6.go-layerxanchor-keeper-keeper-submitequivocation-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/slashing.go:104</code> | <code>func (k Keeper) SubmitEquivocation(ctx sdk.Context, reporter sdk.AccAddress, evidenceA, evidenceB []byte) (types.SlashRecord, error)</code> |
| <code>interface.6.go-layerxanchor-keeper-keeper-updateparams-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/keeper.go:87</code> | <code>func (k Keeper) UpdateParams(ctx sdk.Context, authority sdk.AccAddress, params types.Params) error</code> |
| <code>interface.6.go-layerxanchor-keeper-newcustodyanchor-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/custody_reader.go:40</code> | <code>func NewCustodyAnchor(k Keeper) CustodyAnchor { return CustodyAnchor{k: k} }</code> |
| <code>interface.6.go-layerxanchor-keeper-newkeeper-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/keeper.go:23</code> | <code>func NewKeeper(storeKey sdk.StoreKey, accountKeeper types.AccountKeeper, bankKeeper types.BankKeeper) Keeper</code> |
| <code>interface.6.go-layerxanchor-keeper-newmsgserverimpl-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/msg_server.go:13</code> | <code>func NewMsgServerImpl(k Keeper) types.MsgServer { return msgServer{keeper: k} }</code> |
| <code>interface.6.go-layerxanchor-keeper-registerinvariants-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/keeper/genesis.go:103</code> | <code>func RegisterInvariants(registry sdk.InvariantRegistry, k Keeper)</code> |
| <code>interface.6.go-layerxanchor-module-appmodule-exportgenesis-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/module.go:103</code> | <code>func (am AppModule) ExportGenesis(ctx sdk.Context, _ codec.JSONCodec) json.RawMessage</code> |
| <code>interface.6.go-layerxanchor-module-appmodule-exportgenesisstream-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/module.go:111</code> | <code>func (am AppModule) ExportGenesisStream(ctx sdk.Context, cdc codec.JSONCodec) &lt;-chan json.RawMessage</code> |
| <code>interface.6.go-layerxanchor-module-appmodule-initgenesis-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/module.go:94</code> | <code>func (am AppModule) InitGenesis(ctx sdk.Context, _ codec.JSONCodec, gs json.RawMessage) []abci.ValidatorUpdate</code> |
| <code>interface.6.go-layerxanchor-module-appmodule-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/module.go:73</code> | <code>type AppModule struct</code> |
| <code>interface.6.go-layerxanchor-module-appmodule-registerinvariants-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/module.go:90</code> | <code>func (am AppModule) RegisterInvariants(registry sdk.InvariantRegistry)</code> |
| <code>interface.6.go-layerxanchor-module-appmodulebasic-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/module.go:30</code> | <code>type AppModuleBasic struct{}</code> |
| <code>interface.6.go-layerxanchor-module-appmodulebasic-validategenesisstream-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/module.go:56</code> | <code>func (am AppModuleBasic) ValidateGenesisStream(cdc codec.JSONCodec, config client.TxEncodingConfig, genesisCh &lt;-chan json.RawMessage) error</code> |
| <code>interface.6.go-layerxanchor-module-basic-defaultgenesis-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/module.go:40</code> | <code>func (AppModuleBasic) DefaultGenesis(_ codec.JSONCodec) json.RawMessage</code> |
| <code>interface.6.go-layerxanchor-module-basic-getquerycmd-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/module.go:71</code> | <code>func (AppModuleBasic) GetQueryCmd() *cobra.Command { return nil }</code> |
| <code>interface.6.go-layerxanchor-module-basic-gettxcmd-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/module.go:69</code> | <code>func (AppModuleBasic) GetTxCmd() *cobra.Command { return nil }</code> |
| <code>interface.6.go-layerxanchor-module-basic-name-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/module.go:32</code> | <code>func (AppModuleBasic) Name() string { return types.ModuleName }</code> |
| <code>interface.6.go-layerxanchor-module-basic-registergrpcgatewayroutes-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/module.go:67</code> | <code>func (AppModuleBasic) RegisterGRPCGatewayRoutes(_ client.Context, _ *runtime.ServeMux) {}</code> |
| <code>interface.6.go-layerxanchor-module-basic-registerinterfaces-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/module.go:36</code> | <code>func (AppModuleBasic) RegisterInterfaces(registry cdctypes.InterfaceRegistry)</code> |
| <code>interface.6.go-layerxanchor-module-basic-registerlegacyaminocodec-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/module.go:34</code> | <code>func (AppModuleBasic) RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) { types.RegisterCodec(cdc) }</code> |
| <code>interface.6.go-layerxanchor-module-basic-registerrestroutes-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/module.go:65</code> | <code>func (AppModuleBasic) RegisterRESTRoutes(_ client.Context, _ *mux.Router) {}</code> |
| <code>interface.6.go-layerxanchor-module-basic-validategenesis-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/module.go:48</code> | <code>func (AppModuleBasic) ValidateGenesis(_ codec.JSONCodec, _ client.TxEncodingConfig, bz json.RawMessage) error</code> |
| <code>interface.6.go-layerxanchor-module-module-consensusversion-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/module.go:120</code> | <code>func (AppModule) ConsensusVersion() uint64 { return 1 }</code> |
| <code>interface.6.go-layerxanchor-module-module-legacyquerierhandler-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/module.go:84</code> | <code>func (AppModule) LegacyQuerierHandler(_ *codec.LegacyAmino) sdk.Querier { return nil }</code> |
| <code>interface.6.go-layerxanchor-module-module-querierroute-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/module.go:82</code> | <code>func (AppModule) QuerierRoute() string { return types.QuerierRoute }</code> |
| <code>interface.6.go-layerxanchor-module-module-route-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/module.go:80</code> | <code>func (AppModule) Route() sdk.Route { return sdk.Route{} }</code> |
| <code>interface.6.go-layerxanchor-types-accountkeeper-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/expected_keepers.go:8</code> | <code>type AccountKeeper interface</code> |
| <code>interface.6.go-layerxanchor-types-address20-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/state.go:19</code> | <code>type Address20 [20]byte</code> |
| <code>interface.6.go-layerxanchor-types-address20-marshal-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/codec.go:59</code> | <code>func (a Address20) Marshal() ([]byte, error) { return append([]byte(nil), a[:]...), nil }</code> |
| <code>interface.6.go-layerxanchor-types-address20-marshaljson-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/state.go:21</code> | <code>func (a Address20) MarshalJSON() ([]byte, error) { return json.Marshal(hex.EncodeToString(a[:])) }</code> |
| <code>interface.6.go-layerxanchor-types-address20-marshalto-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/codec.go:60</code> | <code>func (a Address20) MarshalTo(data []byte) (int, error)</code> |
| <code>interface.6.go-layerxanchor-types-address20-size-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/codec.go:58</code> | <code>func (a Address20) Size() int                { return len(a) }</code> |
| <code>interface.6.go-layerxanchor-types-address20-unmarshal-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/codec.go:66</code> | <code>func (a *Address20) Unmarshal(data []byte) error</code> |
| <code>interface.6.go-layerxanchor-types-address20-unmarshaljson-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/state.go:23</code> | <code>func (a *Address20) UnmarshalJSON(raw []byte) error { return unmarshalHex(raw, a[:]) }</code> |
| <code>interface.6.go-layerxanchor-types-anchor-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/state.go:165</code> | <code>type Anchor struct</code> |
| <code>interface.6.go-layerxanchor-types-availabilityattestation-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/state.go:112</code> | <code>type AvailabilityAttestation struct</code> |
| <code>interface.6.go-layerxanchor-types-availabilitybatchprefix-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/keys.go:63</code> | <code>func AvailabilityBatchPrefix(batchNumber uint64) []byte</code> |
| <code>interface.6.go-layerxanchor-types-availabilitykey-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/keys.go:59</code> | <code>func AvailabilityKey(batchNumber uint64, guarantorID [32]byte) []byte</code> |
| <code>interface.6.go-layerxanchor-types-bankkeeper-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/expected_keepers.go:13</code> | <code>type BankKeeper interface</code> |
| <code>interface.6.go-layerxanchor-types-challenge-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/state.go:132</code> | <code>type Challenge struct</code> |
| <code>interface.6.go-layerxanchor-types-challengekey-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/keys.go:67</code> | <code>func ChallengeKey(id uint64) []byte { return join(ChallengePrefix, u64(id)) }</code> |
| <code>interface.6.go-layerxanchor-types-checkpoint-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/state.go:83</code> | <code>type Checkpoint struct</code> |
| <code>interface.6.go-layerxanchor-types-checkpointidkey-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/keys.go:57</code> | <code>func CheckpointIDKey(checkpointID [32]byte) []byte { return join(CheckpointIDPrefix, checkpointID[:]) }</code> |
| <code>interface.6.go-layerxanchor-types-checkpointkey-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/keys.go:54</code> | <code>func CheckpointKey(batchNumber uint64) []byte { return join(CheckpointPrefix, u64(batchNumber)) }</code> |
| <code>interface.6.go-layerxanchor-types-dec-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/codec.go:75</code> | <code>type Dec = sdk.Dec</code> |
| <code>interface.6.go-layerxanchor-types-defaultauthority-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/genesis.go:28</code> | <code>func DefaultAuthority() string</code> |
| <code>interface.6.go-layerxanchor-types-defaultgenesis-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/genesis.go:32</code> | <code>func DefaultGenesis() *GenesisState</code> |
| <code>interface.6.go-layerxanchor-types-defaultparams-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/params.go:36</code> | <code>func DefaultParams(authority string) Params</code> |
| <code>interface.6.go-layerxanchor-types-genesisstate-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/genesis.go:11</code> | <code>type GenesisState struct</code> |
| <code>interface.6.go-layerxanchor-types-genesisstate-validate-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/genesis.go:52</code> | <code>func (g GenesisState) Validate() error</code> |
| <code>interface.6.go-layerxanchor-types-guarantor-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/state.go:57</code> | <code>type Guarantor struct</code> |
| <code>interface.6.go-layerxanchor-types-guarantorkey-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/keys.go:50</code> | <code>func GuarantorKey(guarantorID [32]byte) []byte { return join(GuarantorPrefix, guarantorID[:]) }</code> |
| <code>interface.6.go-layerxanchor-types-hash32-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/state.go:12</code> | <code>type Hash32 [32]byte</code> |
| <code>interface.6.go-layerxanchor-types-hash32-marshaljson-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/state.go:14</code> | <code>func (h Hash32) MarshalJSON() ([]byte, error) { return json.Marshal(hex.EncodeToString(h[:])) }</code> |
| <code>interface.6.go-layerxanchor-types-hash32-unmarshaljson-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/state.go:16</code> | <code>func (h *Hash32) UnmarshalJSON(raw []byte) error { return unmarshalHex(raw, h[:]) }</code> |
| <code>interface.6.go-layerxanchor-types-int-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/codec.go:74</code> | <code>type Int = sdk.Int</code> |
| <code>interface.6.go-layerxanchor-types-msgupdateparams-getsignbytes-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/msgs.go:18</code> | <code>func (m MsgUpdateParams) GetSignBytes() []byte</code> |
| <code>interface.6.go-layerxanchor-types-msgupdateparams-getsigners-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/msgs.go:11</code> | <code>func (m MsgUpdateParams) GetSigners() []sdk.AccAddress</code> |
| <code>interface.6.go-layerxanchor-types-msgupdateparams-route-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/msgs.go:9</code> | <code>func (m MsgUpdateParams) Route() string { return RouterKey }</code> |
| <code>interface.6.go-layerxanchor-types-msgupdateparams-string-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/msgs.go:21</code> | <code>func (m MsgUpdateParams) String() string { return jsonString(m) }</code> |
| <code>interface.6.go-layerxanchor-types-msgupdateparams-type-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/msgs.go:10</code> | <code>func (m MsgUpdateParams) Type() string  { return TypeMsgUpdateParams }</code> |
| <code>interface.6.go-layerxanchor-types-msgupdateparams-validatebasic-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/msgs.go:22</code> | <code>func (m MsgUpdateParams) ValidateBasic() error</code> |
| <code>interface.6.go-layerxanchor-types-params-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/params.go:10</code> | <code>type Params struct</code> |
| <code>interface.6.go-layerxanchor-types-params-marshaljsonpb-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/codec.go:30</code> | <code>func (p Params) MarshalJSONPB(*jsonpb.Marshaler) ([]byte, error) { return json.Marshal(p) }</code> |
| <code>interface.6.go-layerxanchor-types-params-string-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/codec.go:29</code> | <code>func (p Params) String() string                                  { return jsonString(p) }</code> |
| <code>interface.6.go-layerxanchor-types-params-unmarshaljsonpb-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/codec.go:31</code> | <code>func (p *Params) UnmarshalJSONPB(u *jsonpb.Unmarshaler, raw []byte) error</code> |
| <code>interface.6.go-layerxanchor-types-params-validate-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/params.go:59</code> | <code>func (p Params) Validate() error</code> |
| <code>interface.6.go-layerxanchor-types-registercodec-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/codec.go:15</code> | <code>func RegisterCodec(cdc *codec.LegacyAmino)</code> |
| <code>interface.6.go-layerxanchor-types-registerinterfaces-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/codec.go:18</code> | <code>func RegisterInterfaces(registry cdctypes.InterfaceRegistry)</code> |
| <code>interface.6.go-layerxanchor-types-sequencerauthorization-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/state.go:42</code> | <code>type SequencerAuthorization struct</code> |
| <code>interface.6.go-layerxanchor-types-sequenceridprefix-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/keys.go:48</code> | <code>func SequencerIDPrefix(sequencerID [32]byte) []byte { return join(SequencerPrefix, sequencerID[:]) }</code> |
| <code>interface.6.go-layerxanchor-types-sequencerkey-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/keys.go:44</code> | <code>func SequencerKey(sequencerID [32]byte, firstBatch uint64) []byte</code> |
| <code>interface.6.go-layerxanchor-types-slashrecord-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/state.go:152</code> | <code>type SlashRecord struct</code> |
| <code>interface.6.go-layerxanchor-types-slashrecordkey-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/keys.go:71</code> | <code>func SlashRecordKey(guarantorID [32]byte, reason uint8, batchNumber uint64) []byte</code> |
| <code>interface.6.go-layerxanchor-types-unbondingentry-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/state.go:68</code> | <code>type UnbondingEntry struct</code> |
| <code>interface.6.go-layerxanchor-types-unbondingkey-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/keys.go:52</code> | <code>func UnbondingKey(id uint64) []byte { return join(UnbondingPrefix, u64(id)) }</code> |
| <code>interface.6.go-layerxanchor-types-validatesequencers-j35</code> | <code>go_exported</code> | <code>stable</code> | <code>modules/layerxanchor/types/genesis.go:38</code> | <code>func ValidateSequencers(list []SequencerAuthorization) error</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.6.go-layerxanchor-keeper-balanceinvariant-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-layerxanchor-keeper-custodyanchor-finalizedcheckpoint-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxanchor-keeper-custodyanchor-finalizedreceiptroot-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxanchor-keeper-custodyanchor-finalizedstateroot-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-layerxanchor-keeper-custodyanchor-j35</code> | notes | exported type; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxanchor-keeper-custodyanchor-latestfinalizedbatch-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-layerxanchor-keeper-custodyanchor-sequencerauthorization-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-layerxanchor-keeper-keeper-activateguarantor-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-layerxanchor-keeper-keeper-beginunbond-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxanchor-keeper-keeper-checkpointbyid-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxanchor-keeper-keeper-completeunbond-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxanchor-keeper-keeper-eligiblesigner-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxanchor-keeper-keeper-escrowed-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxanchor-keeper-keeper-exportgenesis-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-layerxanchor-keeper-keeper-finalize-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-layerxanchor-keeper-keeper-finalizedreceiptroot-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxanchor-keeper-keeper-finalizedstateroot-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-layerxanchor-keeper-keeper-getanchor-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxanchor-keeper-keeper-getavailability-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxanchor-keeper-keeper-getchallenge-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxanchor-keeper-keeper-getchallenges-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxanchor-keeper-keeper-getcheckpoint-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-layerxanchor-keeper-keeper-getcheckpoints-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-layerxanchor-keeper-keeper-getguarantor-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-layerxanchor-keeper-keeper-getguarantors-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxanchor-keeper-keeper-getparams-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-layerxanchor-keeper-keeper-getsequencerauthorizations-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxanchor-keeper-keeper-getslashrecords-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-layerxanchor-keeper-keeper-getunbondings-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxanchor-keeper-keeper-increasebond-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxanchor-keeper-keeper-initgenesis-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-layerxanchor-keeper-keeper-j35</code> | notes | exported type; referenced across packages |
| <code>interface.6.go-layerxanchor-keeper-keeper-latestfinalizedbatch-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-layerxanchor-keeper-keeper-moduleaddress-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxanchor-keeper-keeper-modulebalance-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxanchor-keeper-keeper-openchallenge-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxanchor-keeper-keeper-registerguarantor-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-layerxanchor-keeper-keeper-resolvechallenge-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxanchor-keeper-keeper-returnallbonds-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxanchor-keeper-keeper-sequencerauthorization-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-layerxanchor-keeper-keeper-sequencerauthorizationforbatch-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxanchor-keeper-keeper-setparams-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-layerxanchor-keeper-keeper-setsequencerauthorization-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-layerxanchor-keeper-keeper-statusof-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxanchor-keeper-keeper-submitavailabilityattestation-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxanchor-keeper-keeper-submitcheckpoint-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-layerxanchor-keeper-keeper-submitequivocation-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxanchor-keeper-keeper-updateparams-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxanchor-keeper-newcustodyanchor-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-layerxanchor-keeper-newkeeper-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-layerxanchor-keeper-newmsgserverimpl-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-layerxanchor-keeper-registerinvariants-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-layerxanchor-module-appmodule-exportgenesis-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-layerxanchor-module-appmodule-exportgenesisstream-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxanchor-module-appmodule-initgenesis-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-layerxanchor-module-appmodule-j35</code> | notes | exported type; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxanchor-module-appmodule-registerinvariants-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxanchor-module-appmodulebasic-j35</code> | notes | exported type; referenced across packages |
| <code>interface.6.go-layerxanchor-module-appmodulebasic-validategenesisstream-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxanchor-module-basic-defaultgenesis-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-layerxanchor-module-basic-getquerycmd-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxanchor-module-basic-gettxcmd-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxanchor-module-basic-name-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-layerxanchor-module-basic-registergrpcgatewayroutes-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-layerxanchor-module-basic-registerinterfaces-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-layerxanchor-module-basic-registerlegacyaminocodec-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-layerxanchor-module-basic-registerrestroutes-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-layerxanchor-module-basic-validategenesis-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxanchor-module-module-consensusversion-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxanchor-module-module-legacyquerierhandler-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxanchor-module-module-querierroute-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxanchor-module-module-route-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-layerxanchor-types-accountkeeper-j35</code> | notes | exported type; referenced across packages |
| <code>interface.6.go-layerxanchor-types-address20-j35</code> | notes | exported type; referenced across packages |
| <code>interface.6.go-layerxanchor-types-address20-marshal-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-layerxanchor-types-address20-marshaljson-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxanchor-types-address20-marshalto-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-layerxanchor-types-address20-size-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxanchor-types-address20-unmarshal-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-layerxanchor-types-address20-unmarshaljson-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxanchor-types-anchor-j35</code> | notes | exported type; referenced across packages |
| <code>interface.6.go-layerxanchor-types-availabilityattestation-j35</code> | notes | exported type; referenced across packages |
| <code>interface.6.go-layerxanchor-types-availabilitybatchprefix-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-layerxanchor-types-availabilitykey-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-layerxanchor-types-bankkeeper-j35</code> | notes | exported type; referenced across packages |
| <code>interface.6.go-layerxanchor-types-challenge-j35</code> | notes | exported type; referenced across packages |
| <code>interface.6.go-layerxanchor-types-challengekey-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-layerxanchor-types-checkpoint-j35</code> | notes | exported type; referenced across packages |
| <code>interface.6.go-layerxanchor-types-checkpointidkey-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-layerxanchor-types-checkpointkey-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-layerxanchor-types-dec-j35</code> | notes | exported type; referenced across packages |
| <code>interface.6.go-layerxanchor-types-defaultauthority-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-layerxanchor-types-defaultgenesis-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-layerxanchor-types-defaultparams-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-layerxanchor-types-genesisstate-j35</code> | notes | exported type; referenced across packages |
| <code>interface.6.go-layerxanchor-types-genesisstate-validate-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-layerxanchor-types-guarantor-j35</code> | notes | exported type; referenced across packages |
| <code>interface.6.go-layerxanchor-types-guarantorkey-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-layerxanchor-types-hash32-j35</code> | notes | exported type; referenced across packages |
| <code>interface.6.go-layerxanchor-types-hash32-marshaljson-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxanchor-types-hash32-unmarshaljson-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxanchor-types-int-j35</code> | notes | exported type; referenced across packages |
| <code>interface.6.go-layerxanchor-types-msgupdateparams-getsignbytes-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-layerxanchor-types-msgupdateparams-getsigners-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-layerxanchor-types-msgupdateparams-route-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-layerxanchor-types-msgupdateparams-string-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-layerxanchor-types-msgupdateparams-type-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-layerxanchor-types-msgupdateparams-validatebasic-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-layerxanchor-types-params-j35</code> | notes | exported type; referenced across packages |
| <code>interface.6.go-layerxanchor-types-params-marshaljsonpb-j35</code> | notes | exported method; no cross-package reference at HEAD; reached through its package or an interface |
| <code>interface.6.go-layerxanchor-types-params-string-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-layerxanchor-types-params-unmarshaljsonpb-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-layerxanchor-types-params-validate-j35</code> | notes | exported method; referenced across packages |
| <code>interface.6.go-layerxanchor-types-registercodec-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-layerxanchor-types-registerinterfaces-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-layerxanchor-types-sequencerauthorization-j35</code> | notes | exported type; referenced across packages |
| <code>interface.6.go-layerxanchor-types-sequenceridprefix-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-layerxanchor-types-sequencerkey-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-layerxanchor-types-slashrecord-j35</code> | notes | exported type; referenced across packages |
| <code>interface.6.go-layerxanchor-types-slashrecordkey-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-layerxanchor-types-unbondingentry-j35</code> | notes | exported type; referenced across packages |
| <code>interface.6.go-layerxanchor-types-unbondingkey-j35</code> | notes | exported func; referenced across packages |
| <code>interface.6.go-layerxanchor-types-validatesequencers-j35</code> | notes | exported func; referenced across packages |

## precompile-retirement

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.6.go-layerxanchor-anchorreader-j35</code> | <code>go_exported</code> | <code>retire</code> | <code>modules/layerxanchor/keeper/keeper.go:187</code> | <code>type AnchorReader interface { FinalizedStateRoot; FinalizedReceiptRoot; LatestFinalizedBatch; SequencerAuthorization }</code> |
| <code>interface.6.go-layerxanchor-precompile-execute-j35</code> | <code>go_exported</code> | <code>change</code> | <code>precompiles/layerxanchor/layerxanchor.go:189</code> | <code>func (p PrecompileExecutor) Execute(ctx sdk.Context, method *abi.Method, caller common.Address, _ common.Address, args []interface{}, value *big.Int, readOnly bool, evm *vm.EVM, hooks *tracing.Hooks) (ret []byte, err error)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.6.go-layerxanchor-anchorreader-j35</code> | notes | only the compile-time assertion at modules/layerxanchor/keeper/keeper.go:194 refers to it |
| <code>interface.6.go-layerxanchor-precompile-execute-j35</code> | notes | gains the merge write gate, the store-delete read gate and the unwired retired revert |

Contract context <code>contract_context.6.precompiles-layerxanchor-j97</code>:

| Field | Frozen rule |
|---|---|
| interface | 1 surfaces under precompiles/layerxanchor (1 go_exported; 1 change) frozen at the anchors contracts/layerxanchor.md#precompile-retirement: func (p PrecompileExecutor) Execute(ctx sdk.Context, method *abi.Method, caller common.Add (precompiles/layerxanchor/layerxanchor.go:189) |
| storage | no storage_key item in this context; the precompiles/layerxanchor items read or write state only through the keepers and stores frozen by the other domain-6 contexts |
| errors | no error_code item in this context; failures surface through the error registries frozen by the other domain-6 contexts |
| events | no event, precompile_event or sol_event item is named by this context; events of the handlers behind these items are frozen by the other domain-6 contexts |
| gas | not metered: no item in this context is a precompile method |
| activation | lands with tasks 6.32.1, 6.32.2, 6.34.1 in waves 2, 4, 6 |
| invariants | each item keeps the kind, path and signature of its interface block once its landing task passes; a later change of any item needs a new decision of domain 6 |
| pause | no pause parameter governs these items; they are disabled only by reverting the touches of their landing tasks |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.6.282.1</code> | req | 6282.1 |
| <code>acceptance.6.282.1</code> | scenario | Given the merge plan done at height 20, when each of the twelve write methods of precompiles/layerxanchor/abi.json is called at height 20, then every call reverts with Error(string) LXP_MODULE_DISABLED layerxAnchor retired, emits no log and leaves anchor state unchanged |
| <code>acceptance.6.282.1</code> | test | precompiles/layerxanchor/retire_gate_test.go::TestRetiredWritesRevertAfterMerge (planned) |
| <code>acceptance.6.282.1</code> | qualification | UNRUN |
| <code>req.6282</code> | ac_1 | Execute (precompiles/layerxanchor/layerxanchor.go:189) runs pcommon.RetiredAt (precompiles/common/retire.go:24) with plan MergeUpgrade before the payable switch (precompiles/layerxanchor/layerxanchor.go:205-211) and returns pcommon.RetiredRevert("layerxAnchor") with vm.ErrExecutionReverted for each of the twelve write methods. |
| <code>acceptance.6.282.2</code> | req | 6282.2 |
| <code>acceptance.6.282.2</code> | scenario | Given the merge plan done at height 20, when registerGuarantor and finalize are called at height 19, then they return the HEAD outputs and the gas used equals the Gas formula at precompiles/layerxanchor/layerxanchor.go:162 |
| <code>acceptance.6.282.2</code> | test | precompiles/layerxanchor/retire_gate_test.go::TestWritesServedBeforeMerge (planned) |
| <code>acceptance.6.282.2</code> | qualification | UNRUN |
| <code>req.6282</code> | ac_2 | Below the merge plan height Execute serves writes unchanged, with the same gas and return bytes as at HEAD, because RetiredAt reads the plan without charging gas (precompiles/common/retire.go:28). |
| <code>acceptance.6.282.3</code> | req | 6282.3 |
| <code>acceptance.6.282.3</code> | scenario | Given the merge plan done and a caller holding 1000 units, when registerGuarantor is called with value 100, then the call reverts LXP_MODULE_DISABLED and the caller balance stays 1000 |
| <code>acceptance.6.282.3</code> | test | precompiles/layerxanchor/retire_gate_test.go::TestRetiredPayableWriteKeepsValue (planned) |
| <code>acceptance.6.282.3</code> | qualification | UNRUN |
| <code>req.6282</code> | ac_3 | A retired payable write (registerGuarantor, increaseBond, openChallenge at precompiles/layerxanchor/layerxanchor.go:205-206) reverts before any value transfer, so the caller keeps the attached value. |
| <code>acceptance.6.283.1</code> | req | 6283.1 |
| <code>acceptance.6.283.1</code> | scenario | Given the merge plan done at height 20 and the store-delete step not applied, when latestFinalized, checkpoint and threshold are called at height 25, then they return the stored values |
| <code>acceptance.6.283.1</code> | test | precompiles/layerxanchor/retire_read_gate_test.go::TestReadsServedUntilStoreDelete (planned) |
| <code>acceptance.6.283.1</code> | qualification | UNRUN |
| <code>req.6283</code> | ac_1 | After the merge plan and before StoreDeleteUpgrade the nine reads (precompiles/layerxanchor/layerxanchor.go:260-287) keep answering from the anchor store. |
| <code>acceptance.6.283.2</code> | req | 6283.2 |
| <code>acceptance.6.283.2</code> | scenario | Given the store-delete step done at height 30, when each of the nine view methods is called at height 30, then every call reverts with Error(string) LXP_MODULE_DISABLED layerxAnchor retired |
| <code>acceptance.6.283.2</code> | test | precompiles/layerxanchor/retire_read_gate_test.go::TestReadsRevertFromStoreDelete (planned) |
| <code>acceptance.6.283.2</code> | qualification | UNRUN |
| <code>req.6283</code> | ac_2 | Execute returns RetiredRevert("layerxAnchor") for each of the nine reads once RetiredAt reports StoreDeleteUpgrade active, the second step named at precompiles/common/retire.go:19-20. |
| <code>acceptance.6.284.1</code> | req | 6284.1 |
| <code>acceptance.6.284.1</code> | scenario | Given the integrated app with the anchor store deleted and no anchor keeper wired, when each abi.json selector is called through the EVM at 0x1014, then each reverts with LXP_MODULE_DISABLED layerxAnchor retired and the receipt carries no log |
| <code>acceptance.6.284.1</code> | test | precompiles/layerxanchor/retired_surface_test.go::TestEveryAnchorSelectorRetired (planned) |
| <code>acceptance.6.284.1</code> | qualification | UNRUN |
| <code>req.6284</code> | ac_1 | After the store-delete step every one of the 21 methods in precompiles/layerxanchor/abi.json reverts LXP_MODULE_DISABLED, the address stays registered (precompiles/setup.go:180) and no 0x1014 event of the 12 in the ABI is emitted. |
| <code>acceptance.6.287.1</code> | req | 6287.1 |
| <code>acceptance.6.287.1</code> | scenario | Given a PrecompileExecutor built from utils.Keepers whose AnchorK returns nil (precompiles/utils/expected_keepers.go:98), when threshold and finalize are called, then both revert with Error(string) LXP_MODULE_DISABLED layerxAnchor retired |
| <code>acceptance.6.287.1</code> | test | precompiles/layerxanchor/unwired_test.go::TestUnwiredAnchorRevertsModuleDisabled (planned) |
| <code>acceptance.6.287.1</code> | qualification | UNRUN |
| <code>req.6287</code> | ac_1 | With no anchor keeper wired, Execute returns RetiredRevert("layerxAnchor") instead of the plain error "layerxanchor: anchor module is not wired" (precompiles/layerxanchor/layerxanchor.go:199-201), and the unused keeper AnchorReader interface (modules/layerxanchor/keeper/keeper.go:186-194) is removed. |
| <code>capability.6.32.1</code> | title | Refuse every layerxAnchor 0x1014 write with LXP_MODULE_DISABLED after the merge plan |
| <code>capability.6.32.1</code> | reqs | 6282.1<br>6282.2<br>6282.3 |
| <code>capability.6.32.2</code> | title | Refuse layerxAnchor 0x1014 reads from the anchor store-delete step |
| <code>capability.6.32.2</code> | reqs | 6283.1<br>6283.2 |
| <code>capability.6.32.3</code> | title | Check every 0x1014 selector returns LXP_MODULE_DISABLED after anchor store delete |
| <code>capability.6.32.3</code> | reqs | 6284.1 |
| <code>capability.6.34.1</code> | title | Return LXP_MODULE_DISABLED from an unwired layerxAnchor and drop the keeper AnchorReader |
| <code>capability.6.34.1</code> | reqs | 6287.1 |
| <code>resolution.17.724</code> | title | decision.628 binds: CustodyAnchor stays until custody retires; the keeper AnchorReader goes |
| <code>resolution.17.724</code> | applies_to | decision.628<br>task.6.34.1 |
| <code>resolution.17.724</code> | source | decision.628 in fragments/J35/decision.kvx; tasks from affects |
| <code>resolution.17.727</code> | title | decision.631 binds: Bonds are returned at the merge plan; reads continue until store delete |
| <code>resolution.17.727</code> | applies_to | decision.631<br>task.6.32.2<br>task.6.33.1<br>task.6.33.2 |
| <code>resolution.17.727</code> | source | decision.631 in fragments/J35/decision.kvx; tasks from affects |
| <code>resolution.17.728</code> | title | decision.632 binds: 0x1014 Solidity interfaces are retired by executor refusal, not deletion |
| <code>resolution.17.728</code> | applies_to | decision.632<br>task.6.32.1<br>task.6.32.2<br>task.6.32.3 |
| <code>resolution.17.728</code> | source | decision.632 in fragments/J35/decision.kvx; tasks from affects |
| <code>resolution.17.762</code> | title | decision.685 binds: Retired 0x1012, 0x1013 and 0x1014 calls decode their arguments before the retire gate |
| <code>resolution.17.762</code> | applies_to | decision.685<br>task.6.32.1<br>task.6.45.1<br>task.6.71.1 |
| <code>resolution.17.762</code> | source | decision.685 in fragments/J67/decision.kvx; tasks from affects |

## retirement

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.6.go-layerxanchor-appmodule-registerservices-j35</code> | <code>go_exported</code> | <code>change</code> | <code>modules/layerxanchor/module.go:86</code> | <code>func (am AppModule) RegisterServices(cfg module.Configurator)</code> |
| <code>interface.6.go-layerxanchor-mergeretired-j35</code> | <code>go_exported</code> | <code>new</code> | <code>modules/layerxanchor/keeper/retire.go</code> (planned) | <code>func MergeRetired(ctx sdk.Context, upgrades UpgradeReader) bool</code> |
| <code>interface.6.go-layerxanchor-mergeupgrade-j35</code> | <code>go_exported</code> | <code>new</code> | <code>modules/layerxanchor/types/retire.go</code> (planned) | <code>const MergeUpgrade = "v7.0"</code> |
| <code>interface.6.go-layerxanchor-newappmodule-j35</code> | <code>go_exported</code> | <code>change</code> | <code>modules/layerxanchor/module.go:78</code> | <code>func NewAppModule(k keeper.Keeper) AppModule</code> |
| <code>interface.6.go-layerxanchor-newmsgserverimplwithupgrades-j35</code> | <code>go_exported</code> | <code>new</code> | <code>modules/layerxanchor/keeper/msg_server.go</code> (planned) | <code>func NewMsgServerImplWithUpgrades(k Keeper, upgrades UpgradeReader) types.MsgServer</code> |
| <code>interface.6.go-layerxanchor-storedeleteupgrade-j35</code> | <code>go_exported</code> | <code>new</code> | <code>modules/layerxanchor/types/retire.go</code> (planned) | <code>const StoreDeleteUpgrade = "v7.0-anchor-store-delete"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.6.go-layerxanchor-appmodule-registerservices-j35</code> | notes | registers the gated msg server |
| <code>interface.6.go-layerxanchor-mergeretired-j35</code> | notes | planned |
| <code>interface.6.go-layerxanchor-mergeupgrade-j35</code> | notes | equal to node/upgrades.go:112 |
| <code>interface.6.go-layerxanchor-newappmodule-j35</code> | notes | gains the upgrade reader argument |
| <code>interface.6.go-layerxanchor-newmsgserverimplwithupgrades-j35</code> | notes | planned |
| <code>interface.6.go-layerxanchor-storedeleteupgrade-j35</code> | notes | second retirement step, decision.629 |

Contract context <code>contract_context.6.layerxanchor-retirement-j35</code>:

| Field | Frozen rule |
|---|---|
| interface | layerxAnchor precompile 0x1014 (precompiles/layerxanchor/abi.json: 12 writes, 9 reads, 12 events), layerxanchor Msg UpdateParams (api/layerxanchor/tx.proto:5), keeper RetireAtMerge and MergeRetired |
| storage | layerxanchor store prefixes 0x01..0x0e (modules/layerxanchor/types/keys.go:13-26), served read-only between the merge plan and StoreDeleteUpgrade, then deleted |
| errors | Precompile: Error(string) LXP_MODULE_DISABLED "layerxAnchor retired" (precompiles/common/retire.go:34); module: planned ErrModuleRetired 19 LXP_MODULE_DISABLED |
| events | No 0x1014 log after the merge plan; one layerx_anchor_retired { returned_bond } at the merge height |
| gas | Retirement gate reads the plan without charging gas (precompiles/common/retire.go:28), so gas below the plan height equals the Gas formula at precompiles/layerxanchor/layerxanchor.go:162 |
| activation | Writes and MsgUpdateParams refused from the MergeUpgrade v7.0 done height; reads refused from the v7.0-anchor-store-delete done height (decision.629); address stays registered (precompiles/setup.go:180) |
| invariants | BalanceInvariant (modules/layerxanchor/keeper/genesis.go:94) holds at every height; zero escrow after RetireAtMerge; no anchor state changes after the merge height |
| pause | No pause scope: retirement replaces pause for this module |

Contract context <code>contract_context.6.modules-layerxanchor-j97</code>:

| Field | Frozen rule |
|---|---|
| interface | 1 surfaces under modules/layerxanchor (1 go_exported; 1 new) frozen at the anchors contracts/layerxanchor.md#retirement: func NewMsgServerImplWithUpgrades(k Keeper, upgrades UpgradeReader) types.MsgServer (modules/layerxanchor/keeper/msg_server.go (planned)) |
| storage | no storage_key item in this context; the modules/layerxanchor items read or write state only through the keepers and stores frozen by the other domain-6 contexts |
| errors | no error_code item in this context; failures surface through the error registries frozen by the other domain-6 contexts |
| events | no event, precompile_event or sol_event item is named by this context; events of the handlers behind these items are frozen by the other domain-6 contexts |
| gas | not metered: no item in this context is a precompile method |
| activation | lands with tasks 6.31.4 in wave 2 |
| invariants | each item keeps the kind, path and signature of its interface block once its landing task passes; a later change of any item needs a new decision of domain 6 |
| pause | no pause parameter governs these items; they are disabled only by reverting the touches of their landing tasks |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.6.275.1</code> | req | 6275.1 |
| <code>acceptance.6.275.1</code> | scenario | Given the integrated app with the anchor msg server built with the upgrade reader, when the gov route handles a LayerXProposal updating the anchor Threshold below and at the merge plan done height, then the first call changes Threshold and the second returns ErrModuleRetired and leaves Threshold unchanged |
| <code>acceptance.6.275.1</code> | test | modules/layerxgov/handler_anchor_retired_test.go::TestGovRouteRefusesRetiredAnchorParams (planned) |
| <code>acceptance.6.275.1</code> | qualification | UNRUN |
| <code>req.6275</code> | ac_1 | A LayerXProposal carrying layerxanchor MsgUpdateParams (modules/layerxgov/types/proposal.go:184-185) succeeds through the gov route (node/app.go:969) before the merge plan and returns ErrModuleRetired code 19 after it, with params unchanged. |
| <code>acceptance.6.278.1</code> | req | 6278.1 |
| <code>acceptance.6.278.1</code> | scenario | Given the layerxanchor types package, when the step names and event names are read, then MergeUpgrade is v7.0, StoreDeleteUpgrade is v7.0-anchor-store-delete, the two differ and EventAnchorRetired is layerx_anchor_retired |
| <code>acceptance.6.278.1</code> | test | modules/layerxanchor/types/retire_names_test.go::TestRetirementStepNames (planned) |
| <code>acceptance.6.278.1</code> | qualification | UNRUN |
| <code>req.6278</code> | ac_1 | modules/layerxanchor/types/retire.go (planned) defines MergeUpgrade = "v7.0" equal to node/upgrades.go:112 and precompiles/common/precompiles.go:30, StoreDeleteUpgrade = "v7.0-anchor-store-delete", and events.go gains EventAnchorRetired = "layerx_anchor_retired" with AttributeReturned = "returned_bond". |
| <code>acceptance.6.279.1</code> | req | 6279.1 |
| <code>acceptance.6.279.1</code> | scenario | Given an app whose merge plan is done at height 20, when MergeRetired is evaluated at heights 19, 20 and 21 with the app UpgradeKeeper, then it returns false, true and true and the context gas meter is unchanged |
| <code>acceptance.6.279.1</code> | test | modules/layerxanchor/keeper/retire_gate_test.go::TestMergeRetiredFollowsPlanHeight (planned) |
| <code>acceptance.6.279.1</code> | qualification | UNRUN |
| <code>req.6279</code> | ac_1 | MergeRetired(ctx, upgrades) in modules/layerxanchor/keeper/retire.go (planned) returns true exactly when the upgrade reader reports MergeUpgrade active at ctx.BlockHeight() (sdk/x/upgrade/keeper/keeper.go:296), read without charging gas as RetiredAt does (precompiles/common/retire.go:24-29). |
| <code>acceptance.6.279.2</code> | req | 6279.2 |
| <code>acceptance.6.279.2</code> | scenario | Given an app whose merge plan is done at height 20, when MergeRetired is evaluated at height 25 with a nil reader, then it returns false |
| <code>acceptance.6.279.2</code> | test | modules/layerxanchor/keeper/retire_gate_test.go::TestMergeRetiredWithoutReader (planned) |
| <code>acceptance.6.279.2</code> | qualification | UNRUN |
| <code>req.6279</code> | ac_2 | MergeRetired with a nil upgrade reader returns false, the rule of RetiredAt at precompiles/common/retire.go:25-27, so a keeper built without the reader keeps serving. |
| <code>acceptance.6.280.1</code> | req | 6280.1 |
| <code>acceptance.6.280.1</code> | scenario | Given the merge plan done at height 20 and the governance authority, when UpdateParams is sent at height 19 and at height 20 through NewMsgServerImplWithUpgrades, then the first changes Threshold and the second returns ErrModuleRetired with params unchanged and no layerx_anchor_params_updated event |
| <code>acceptance.6.280.1</code> | test | modules/layerxanchor/keeper/msg_server_retired_test.go::TestMsgUpdateParamsRefusedAfterMerge (planned) |
| <code>acceptance.6.280.1</code> | qualification | UNRUN |
| <code>req.6280</code> | ac_1 | NewMsgServerImplWithUpgrades(k, upgrades) returns a msg server whose UpdateParams (modules/layerxanchor/keeper/msg_server.go:14-27) returns ErrModuleRetired before decoding the authority once MergeRetired is true; NewMsgServerImpl (modules/layerxanchor/keeper/msg_server.go:13) keeps serving for replay. |
| <code>acceptance.6.281.1</code> | req | 6281.1 |
| <code>acceptance.6.281.1</code> | scenario | Given the integrated app built with NewAppModule(app.LayerXAnchorKeeper, &amp;app.UpgradeKeeper), when MsgUpdateParams is dispatched through app.MsgServiceRouter() below and at the merge plan done height, then the first succeeds and the second returns ErrModuleRetired |
| <code>acceptance.6.281.1</code> | test | modules/layerxanchor/module_retired_route_test.go::TestAppRoutesAnchorUpdateParamsRetired (planned) |
| <code>acceptance.6.281.1</code> | qualification | UNRUN |
| <code>req.6281</code> | ac_1 | NewAppModule(k, upgrades) (modules/layerxanchor/module.go:78) registers NewMsgServerImplWithUpgrades in RegisterServices (modules/layerxanchor/module.go:86-88), so the app MsgServiceRouter refuses layerxanchor MsgUpdateParams after the merge plan. |
| <code>acceptance.6.288.1</code> | req | 6288.1 |
| <code>acceptance.6.288.1</code> | scenario | Given the integrated app with bonded guarantors, a finalized checkpoint and both plans scheduled, when blocks run across the merge height and the store-delete height, then each height shows the write, read, balance and invariant outcomes listed for it |
| <code>acceptance.6.288.1</code> | test | modules/layerxanchor/keeper/retirement_rehearsal_test.go::TestAnchorRetirementRehearsal (planned) |
| <code>acceptance.6.288.1</code> | qualification | UNRUN |
| <code>req.6288</code> | ac_1 | Across the v7.0 plan and the v7.0-anchor-store-delete step: writes revert from the merge height, every bond is back with its operator at the merge height, reads answer until the store-delete height and revert from it, the BalanceInvariant (modules/layerxanchor/keeper/genesis.go:94) holds at every height and the module account holds zero escrow. |
| <code>acceptance.6.288.2</code> | req | 6288.2 |
| <code>acceptance.6.288.2</code> | scenario | Given an app after RetireAtMerge, when the anchor genesis is exported and validated, then Validate returns nil and every guarantor carries zero bond and zero unbonding |
| <code>acceptance.6.288.2</code> | test | modules/layerxanchor/keeper/retirement_rehearsal_test.go::TestExportAfterRetireValidates (planned) |
| <code>acceptance.6.288.2</code> | qualification | UNRUN |
| <code>req.6288</code> | ac_2 | ExportGenesis (modules/layerxanchor/keeper/genesis.go:75) after RetireAtMerge produces a GenesisState that passes Validate (modules/layerxanchor/types/genesis.go:52) with no bonded or unbonding guarantor balance. |
| <code>capability.6.28.5</code> | title | Prove the gov route refuses layerxanchor MsgUpdateParams after the merge plan |
| <code>capability.6.28.5</code> | reqs | 6275.1 |
| <code>capability.6.31.2</code> | title | Freeze the layerxanchor retirement step names and retired event |
| <code>capability.6.31.2</code> | reqs | 6278.1 |
| <code>capability.6.31.3</code> | title | Add the MergeRetired gate reader to the layerxanchor keeper package |
| <code>capability.6.31.3</code> | reqs | 6279.1<br>6279.2 |
| <code>capability.6.31.4</code> | title | Refuse layerxanchor MsgUpdateParams after the merge plan |
| <code>capability.6.31.4</code> | reqs | 6280.1 |
| <code>capability.6.31.5</code> | title | Build the layerxanchor module with the upgrade reader and route the gated msg server |
| <code>capability.6.31.5</code> | reqs | 6281.1 |
| <code>capability.6.35.1</code> | title | Rehearse the anchor retirement from bonds posted to store delete |
| <code>capability.6.35.1</code> | reqs | 6288.1<br>6288.2 |
| <code>resolution.17.721</code> | title | decision.625 binds: Retirement is by refusal; types and codecs stay for replay |
| <code>resolution.17.721</code> | applies_to | decision.625<br>task.6.28.5<br>task.6.31.4<br>task.6.31.5 |
| <code>resolution.17.721</code> | source | decision.625 in fragments/J35/decision.kvx; tasks from affects |
| <code>resolution.17.725</code> | title | decision.629 binds: The anchor store-delete step is named v7.0-anchor-store-delete |
| <code>resolution.17.725</code> | applies_to | decision.629<br>task.6.31.2<br>task.6.32.2<br>task.5.7.1<br>task.6.34.1 |
| <code>resolution.17.725</code> | source | decision.629 in fragments/J35/decision.kvx; tasks from affects, tasks naming the affected blocks |
| <code>resolution.17.726</code> | title | decision.630 binds: No layerxgov-level retired-message list |
| <code>resolution.17.726</code> | applies_to | decision.630<br>task.6.28.5 |
| <code>resolution.17.726</code> | source | decision.630 in fragments/J35/decision.kvx; tasks from affects |

## storage

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.6.gen-layerxanchor-genesisstate-j35</code> | <code>genesis_param</code> | <code>stable</code> | <code>modules/layerxanchor/types/genesis.go:11</code> | <code>type GenesisState struct (JSON genesis of layerxanchor, decoded by json.Unmarshal in AppModule.InitGenesis)</code> |
| <code>interface.6.key-layerxanchor-anchorkey-j35</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/layerxanchor/types/keys.go:24</code> | <code>AnchorKey = []byte{0x0c}</code> |
| <code>interface.6.key-layerxanchor-availabilityprefix-j35</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/layerxanchor/types/keys.go:19</code> | <code>AvailabilityPrefix = []byte{0x07}</code> |
| <code>interface.6.key-layerxanchor-challengeprefix-j35</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/layerxanchor/types/keys.go:20</code> | <code>ChallengePrefix = []byte{0x08}</code> |
| <code>interface.6.key-layerxanchor-checkpointattesterprefix-j35</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/layerxanchor/types/keys.go:25</code> | <code>CheckpointAttesterPrefix = []byte{0x0d}</code> |
| <code>interface.6.key-layerxanchor-checkpointidprefix-j35</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/layerxanchor/types/keys.go:26</code> | <code>CheckpointIDPrefix = []byte{0x0e}</code> |
| <code>interface.6.key-layerxanchor-checkpointprefix-j35</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/layerxanchor/types/keys.go:17</code> | <code>CheckpointPrefix = []byte{0x05}</code> |
| <code>interface.6.key-layerxanchor-guarantorprefix-j35</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/layerxanchor/types/keys.go:15</code> | <code>GuarantorPrefix = []byte{0x03}</code> |
| <code>interface.6.key-layerxanchor-latestfinalizedkey-j35</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/layerxanchor/types/keys.go:18</code> | <code>LatestFinalizedKey = []byte{0x06}</code> |
| <code>interface.6.key-layerxanchor-nextchallengeidkey-j35</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/layerxanchor/types/keys.go:21</code> | <code>NextChallengeIDKey = []byte{0x09}</code> |
| <code>interface.6.key-layerxanchor-nextunbondingidkey-j35</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/layerxanchor/types/keys.go:23</code> | <code>NextUnbondingIDKey = []byte{0x0b}</code> |
| <code>interface.6.key-layerxanchor-paramskey-j35</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/layerxanchor/types/keys.go:13</code> | <code>ParamsKey = []byte{0x01}</code> |
| <code>interface.6.key-layerxanchor-sequencerprefix-j35</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/layerxanchor/types/keys.go:14</code> | <code>SequencerPrefix = []byte{0x02}</code> |
| <code>interface.6.key-layerxanchor-slashrecordprefix-j35</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/layerxanchor/types/keys.go:22</code> | <code>SlashRecordPrefix = []byte{0x0a}</code> |
| <code>interface.6.key-layerxanchor-unbondingprefix-j35</code> | <code>storage_key</code> | <code>stable</code> | <code>modules/layerxanchor/types/keys.go:16</code> | <code>UnbondingPrefix = []byte{0x04}</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.6.gen-layerxanchor-genesisstate-j35</code> | notes | JSON, not proto; validated by GenesisState.Validate at modules/layerxanchor/types/genesis.go:52 |
| <code>interface.6.key-layerxanchor-anchorkey-j35</code> | notes | layerxanchor store prefix; deleted with the store at the store-delete step |
| <code>interface.6.key-layerxanchor-availabilityprefix-j35</code> | notes | layerxanchor store prefix; deleted with the store at the store-delete step |
| <code>interface.6.key-layerxanchor-challengeprefix-j35</code> | notes | layerxanchor store prefix; deleted with the store at the store-delete step |
| <code>interface.6.key-layerxanchor-checkpointattesterprefix-j35</code> | notes | layerxanchor store prefix; deleted with the store at the store-delete step |
| <code>interface.6.key-layerxanchor-checkpointidprefix-j35</code> | notes | layerxanchor store prefix; deleted with the store at the store-delete step |
| <code>interface.6.key-layerxanchor-checkpointprefix-j35</code> | notes | layerxanchor store prefix; deleted with the store at the store-delete step |
| <code>interface.6.key-layerxanchor-guarantorprefix-j35</code> | notes | layerxanchor store prefix; deleted with the store at the store-delete step |
| <code>interface.6.key-layerxanchor-latestfinalizedkey-j35</code> | notes | layerxanchor store prefix; deleted with the store at the store-delete step |
| <code>interface.6.key-layerxanchor-nextchallengeidkey-j35</code> | notes | layerxanchor store prefix; deleted with the store at the store-delete step |
| <code>interface.6.key-layerxanchor-nextunbondingidkey-j35</code> | notes | layerxanchor store prefix; deleted with the store at the store-delete step |
| <code>interface.6.key-layerxanchor-paramskey-j35</code> | notes | layerxanchor store prefix; deleted with the store at the store-delete step |
| <code>interface.6.key-layerxanchor-sequencerprefix-j35</code> | notes | layerxanchor store prefix; deleted with the store at the store-delete step |
| <code>interface.6.key-layerxanchor-slashrecordprefix-j35</code> | notes | layerxanchor store prefix; deleted with the store at the store-delete step |
| <code>interface.6.key-layerxanchor-unbondingprefix-j35</code> | notes | layerxanchor store prefix; deleted with the store at the store-delete step |

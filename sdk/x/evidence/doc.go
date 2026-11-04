/*
Package evidence implements a Cosmos SDK module, per ADR 009, that allows for the
submission and handling of arbitrary evidence of misbehavior.

Concrete evidence implements exported.Evidence. The keeper routes submissions to
registered handlers, which may perform state transitions such as slashing,
jailing and tombstoning, then stores accepted evidence by hash. Consensus
misbehavior uses BeginBlocker and HandleEquivocationEvidence directly.

An embedding application registers evidence.AppModuleBasic{} with its basic
manager and interface registry, mounts evidencetypes.StoreKey, and constructs the
keeper after staking and slashing. The setup below accepts an application-owned
handler for the equivocation route. SetRouter seals the routes before the keeper
is copied into the module; it may only be called once. An application that does
not accept transaction submissions can explicitly install an empty router, as
node/app.go does, which rejects submissions with ErrNoEvidenceHandlerExists.

After construction, call mm.RegisterServices with
module.NewConfigurator(appCodec, app.MsgServiceRouter(), app.GRPCQueryRouter())
to register the SDK Msg and Query services. Include evidencetypes.ModuleName in
the manager's genesis initialization order. The following setup is compiled and
exercised by TestRepairM_605_SetupExample:

	import (
	    "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	    sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	    "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
	    "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/evidence"
	    evidencekeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/evidence/keeper"
	    evidencetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/evidence/types"
	)

	func configureEvidence(
	    appCodec codec.Codec, storeKey sdk.StoreKey,
	    stakingKeeper evidencetypes.StakingKeeper, slashingKeeper evidencetypes.SlashingKeeper,
	    evidenceHandler evidencetypes.Handler,
	) (*evidencekeeper.Keeper, *module.Manager) {
	    evidenceKeeper := evidencekeeper.NewKeeper(appCodec, storeKey, stakingKeeper, slashingKeeper)
	    evidenceRouter := evidencetypes.NewRouter().
	        AddRoute(evidencetypes.RouteEquivocation, evidenceHandler)
	    evidenceKeeper.SetRouter(evidenceRouter)
	    mm := module.NewManager(evidence.NewAppModule(*evidenceKeeper))
	    return evidenceKeeper, mm
	}
*/
package evidence

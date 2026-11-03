// Package layerxanchor is the native LayerX anchor: checkpoint registry,
// finality authority, availability record and guarantor bonds. State and
// genesis are plain JSON; every state change is reached through the keeper,
// which the layerxAnchor precompile exposes to the EVM.
package layerxanchor

import (
	"encoding/json"
	"fmt"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
	"github.com/gorilla/mux"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/spf13/cobra"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor/types"
)

var (
	_ module.AppModule      = AppModule{}
	_ module.AppModuleBasic = AppModuleBasic{}
)

type AppModuleBasic struct{}

func (AppModuleBasic) Name() string { return types.ModuleName }

func (AppModuleBasic) RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) { types.RegisterCodec(cdc) }

func (AppModuleBasic) RegisterInterfaces(registry cdctypes.InterfaceRegistry) {
	types.RegisterInterfaces(registry)
}

func (AppModuleBasic) DefaultGenesis(_ codec.JSONCodec) json.RawMessage {
	encoded, err := json.Marshal(types.DefaultGenesis())
	if err != nil {
		panic(err)
	}
	return encoded
}

func (AppModuleBasic) ValidateGenesis(_ codec.JSONCodec, _ client.TxEncodingConfig, bz json.RawMessage) error {
	// node/app.go retains only the basic-manager registration for anchor, so
	// module-manager exports omit its genesis state.
	if len(bz) == 0 {
		return nil
	}
	var genesis types.GenesisState
	if err := json.Unmarshal(bz, &genesis); err != nil {
		return fmt.Errorf("failed to unmarshal %s genesis state: %w", types.ModuleName, err)
	}
	return genesis.Validate()
}

func (am AppModuleBasic) ValidateGenesisStream(cdc codec.JSONCodec, config client.TxEncodingConfig, genesisCh <-chan json.RawMessage) error {
	for genesis := range genesisCh {
		if err := am.ValidateGenesis(cdc, config, genesis); err != nil {
			return err
		}
	}
	return nil
}

func (AppModuleBasic) RegisterRESTRoutes(_ client.Context, _ *mux.Router) {}

func (AppModuleBasic) RegisterGRPCGatewayRoutes(_ client.Context, _ *runtime.ServeMux) {}

func (AppModuleBasic) GetTxCmd() *cobra.Command { return nil }

func (AppModuleBasic) GetQueryCmd() *cobra.Command { return nil }

type AppModule struct {
	AppModuleBasic
	keeper   keeper.Keeper
	upgrades keeper.UpgradeReader
}

// NewAppModule binds the anchor keeper to the application's live upgrade reader.
// Retirement is checked on every message, rather than captured at construction.
func NewAppModule(k keeper.Keeper, upgrades keeper.UpgradeReader) AppModule {
	return AppModule{keeper: k, upgrades: upgrades}
}

func (AppModule) Route() sdk.Route { return sdk.Route{} }

func (AppModule) QuerierRoute() string { return types.QuerierRoute }

func (AppModule) LegacyQuerierHandler(_ *codec.LegacyAmino) sdk.Querier { return nil }

// RegisterServices also supports registration outside the module manager: the
// retired anchor must retain its refusal route after its store is deleted.
func (am AppModule) RegisterServices(cfg module.Configurator) {
	types.RegisterMsgServer(cfg.MsgServer(), keeper.NewMsgServerImplWithUpgrades(am.keeper, am.upgrades))
}

func (am AppModule) RegisterInvariants(registry sdk.InvariantRegistry) {
	keeper.RegisterInvariants(registry, am.keeper)
}

func (am AppModule) InitGenesis(ctx sdk.Context, _ codec.JSONCodec, gs json.RawMessage) []abci.ValidatorUpdate {
	var genesis types.GenesisState
	if err := json.Unmarshal(gs, &genesis); err != nil {
		panic(err)
	}
	am.keeper.InitGenesis(ctx, genesis)
	return []abci.ValidatorUpdate{}
}

func (am AppModule) ExportGenesis(ctx sdk.Context, _ codec.JSONCodec) json.RawMessage {
	encoded, err := json.Marshal(am.keeper.ExportGenesis(ctx))
	if err != nil {
		panic(err)
	}
	return encoded
}

func (am AppModule) ExportGenesisStream(ctx sdk.Context, cdc codec.JSONCodec) <-chan json.RawMessage {
	// Capture the state before returning; callers may advance or close ctx
	// before they drain the stream.
	ch := make(chan json.RawMessage, 1)
	ch <- am.ExportGenesis(ctx, cdc)
	close(ch)
	return ch
}

func (AppModule) ConsensusVersion() uint64 { return 1 }

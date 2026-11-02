// Package pricefeed carries validator-signed index and mark prices on chain:
// each bonded validator's feed account submits observations, and the
// mid-block hook aggregates the fresh observations of the bonded set into
// one price per enabled market. State and genesis are plain JSON.
package pricefeed

import (
	"context"
	"encoding/json"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
	"github.com/gorilla/mux"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/spf13/cobra"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/client/cli"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
)

var (
	_ module.AppModule         = AppModule{}
	_ module.AppModuleBasic    = AppModuleBasic{}
	_ module.MidBlockAppModule = AppModule{}
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

// ValidateGenesis checks the decoded state with the keeper's genesis checks.
func (AppModuleBasic) ValidateGenesis(_ codec.JSONCodec, _ client.TxEncodingConfig, bz json.RawMessage) error {
	genesis, err := decodeGenesis(bz)
	if err != nil {
		return err
	}
	return keeper.ValidateGenesis(genesis)
}

// decodeGenesis keeps the validation and import boundaries on the same JSON
// decoder and registered error, before either path reaches the keeper.
func decodeGenesis(bz json.RawMessage) (types.GenesisState, error) {
	var genesis types.GenesisState
	if err := json.Unmarshal(bz, &genesis); err != nil {
		return types.GenesisState{}, types.ErrInvalidGenesis.Wrapf("failed to unmarshal %s genesis state: %v", types.ModuleName, err)
	}
	return genesis, nil
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

func (AppModuleBasic) RegisterGRPCGatewayRoutes(clientCtx client.Context, mux *runtime.ServeMux) {
	if err := types.RegisterQueryHandlerClient(context.Background(), mux, types.NewQueryClient(clientCtx)); err != nil {
		panic(err)
	}
}

func (AppModuleBasic) GetTxCmd() *cobra.Command { return nil }

func (AppModuleBasic) GetQueryCmd() *cobra.Command { return cli.GetQueryCmd() }

type AppModule struct {
	AppModuleBasic
	keeper keeper.Keeper
}

func NewAppModule(k keeper.Keeper) AppModule { return AppModule{keeper: k} }

func (AppModule) Route() sdk.Route { return sdk.Route{} }

func (AppModule) QuerierRoute() string { return types.ModuleName }

func (AppModule) LegacyQuerierHandler(_ *codec.LegacyAmino) sdk.Querier { return nil }

// RegisterServices routes messages and queries to the keeper.
func (am AppModule) RegisterServices(cfg module.Configurator) {
	types.RegisterMsgServer(cfg.MsgServer(), keeper.NewMsgServerImpl(am.keeper))
	types.RegisterQueryServer(cfg.QueryServer(), keeper.NewQueryServerImpl(am.keeper))
}

func (AppModule) RegisterInvariants(_ sdk.InvariantRegistry) {}

func (am AppModule) InitGenesis(ctx sdk.Context, _ codec.JSONCodec, gs json.RawMessage) []abci.ValidatorUpdate {
	genesis, err := decodeGenesis(gs)
	if err != nil {
		panic(err)
	}
	if err := am.keeper.InitGenesis(ctx, genesis); err != nil {
		panic(err)
	}
	return []abci.ValidatorUpdate{}
}

func (am AppModule) ExportGenesis(ctx sdk.Context, _ codec.JSONCodec) json.RawMessage {
	genesis, err := am.keeper.ExportGenesis(ctx)
	if err != nil {
		panic(err)
	}
	encoded, err := json.Marshal(genesis)
	if err != nil {
		panic(err)
	}
	return encoded
}

func (am AppModule) ExportGenesisStream(ctx sdk.Context, cdc codec.JSONCodec) <-chan json.RawMessage {
	ch := make(chan json.RawMessage)
	go func() {
		ch <- am.ExportGenesis(ctx, cdc)
		close(ch)
	}()
	return ch
}

func (AppModule) ConsensusVersion() uint64 { return 1 }

// MidBlock aggregates observations once per block after the block's
// transactions and before the end block.
func (am AppModule) MidBlock(ctx sdk.Context, _ int64) {
	MidBlocker(ctx, am.keeper)
}

package layerxpay

import (
	"context"
	"encoding/json"
	"fmt"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/client/cli"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
	"github.com/gorilla/mux"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/spf13/cobra"
)

var (
	_ module.AppModuleBasic = AppModuleBasic{}
	_ module.AppModule      = AppModule{}
)

type AppModuleBasic struct{}

func NewAppModuleBasic() AppModuleBasic { return AppModuleBasic{} }

func (AppModuleBasic) Name() string { return types.ModuleName }

func (AppModuleBasic) RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	types.RegisterCodec(cdc)
}

func (AppModuleBasic) RegisterInterfaces(reg cdctypes.InterfaceRegistry) {
	types.RegisterInterfaces(reg)
}

func (AppModuleBasic) DefaultGenesis(_ codec.JSONCodec) json.RawMessage {
	bz, err := json.Marshal(types.DefaultGenesis())
	if err != nil {
		panic(fmt.Errorf("layerxpay: encode default genesis: %w", err))
	}
	return bz
}

func (AppModuleBasic) ValidateGenesis(_ codec.JSONCodec, _ client.TxEncodingConfig, bz json.RawMessage) error {
	var genState types.GenesisState
	if err := json.Unmarshal(bz, &genState); err != nil {
		return fmt.Errorf("failed to unmarshal %s genesis state: %w", types.ModuleName, err)
	}
	return genState.Validate()
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

func NewAppModule(k keeper.Keeper) AppModule {
	return AppModule{AppModuleBasic: NewAppModuleBasic(), keeper: k}
}

func (am AppModule) Name() string { return am.AppModuleBasic.Name() }

func (AppModule) Route() sdk.Route { return sdk.Route{} }

func (AppModule) QuerierRoute() string { return types.QuerierRoute }

func (AppModule) LegacyQuerierHandler(_ *codec.LegacyAmino) sdk.Querier { return nil }

func (AppModule) RegisterInvariants(_ sdk.InvariantRegistry) {}

func (am AppModule) RegisterServices(cfg module.Configurator) {
	types.RegisterMsgServer(cfg.MsgServer(), keeper.NewMsgServerImpl(am.keeper))
	types.RegisterQueryServer(cfg.QueryServer(), keeper.Querier{Keeper: am.keeper})
}

func (am AppModule) InitGenesis(ctx sdk.Context, _ codec.JSONCodec, gs json.RawMessage) []abci.ValidatorUpdate {
	var genState types.GenesisState
	if err := json.Unmarshal(gs, &genState); err != nil {
		panic(fmt.Errorf("failed to unmarshal %s genesis state: %w", types.ModuleName, err))
	}
	if err := am.keeper.InitGenesis(ctx, genState); err != nil {
		panic(fmt.Errorf("layerxpay: initialize genesis: %w", err))
	}
	return []abci.ValidatorUpdate{}
}

func (am AppModule) ExportGenesis(ctx sdk.Context, _ codec.JSONCodec) json.RawMessage {
	bz, err := json.Marshal(am.keeper.ExportGenesis(ctx))
	if err != nil {
		panic(fmt.Errorf("layerxpay: encode genesis: %w", err))
	}
	return bz
}

func (am AppModule) ExportGenesisStream(ctx sdk.Context, cdc codec.JSONCodec) <-chan json.RawMessage {
	ch := make(chan json.RawMessage, 1)
	ch <- am.ExportGenesis(ctx, cdc)
	close(ch)
	return ch
}

func (AppModule) ConsensusVersion() uint64 { return 1 }

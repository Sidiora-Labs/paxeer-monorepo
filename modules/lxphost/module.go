//go:build cgo

package lxphost

import (
	"encoding/json"
	"fmt"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
	storekeys "github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/keys"
	"github.com/gorilla/mux"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/spf13/cobra"
)

var (
	_ module.AppModuleBasic   = StoreAppModuleBasic{}
	_ module.AppModuleGenesis = StoreAppModule{}
)

type StoreAppModuleBasic struct{}

func (StoreAppModuleBasic) Name() string                                                { return storekeys.LayerXHostStoreKey }
func (StoreAppModuleBasic) RegisterLegacyAminoCodec(*codec.LegacyAmino)                 {}
func (StoreAppModuleBasic) RegisterInterfaces(cdctypes.InterfaceRegistry)               {}
func (StoreAppModuleBasic) RegisterRESTRoutes(client.Context, *mux.Router)              {}
func (StoreAppModuleBasic) RegisterGRPCGatewayRoutes(client.Context, *runtime.ServeMux) {}
func (StoreAppModuleBasic) GetTxCmd() *cobra.Command                                    { return nil }
func (StoreAppModuleBasic) GetQueryCmd() *cobra.Command                                 { return nil }

func (StoreAppModuleBasic) DefaultGenesis(codec.JSONCodec) json.RawMessage {
	bz, err := json.Marshal(DefaultStoreGenesis())
	if err != nil {
		panic(fmt.Errorf("lxphost: encode default genesis: %w", err))
	}
	return bz
}

func (StoreAppModuleBasic) ValidateGenesis(_ codec.JSONCodec, _ client.TxEncodingConfig, bz json.RawMessage) error {
	var gs StoreGenesis
	if err := json.Unmarshal(bz, &gs); err != nil {
		return fmt.Errorf("lxphost: decode genesis: %w", err)
	}
	return gs.Validate()
}

func (am StoreAppModuleBasic) ValidateGenesisStream(cdc codec.JSONCodec, config client.TxEncodingConfig, ch <-chan json.RawMessage) error {
	for gs := range ch {
		if err := am.ValidateGenesis(cdc, config, gs); err != nil {
			return err
		}
	}
	return nil
}

type StoreAppModule struct {
	StoreAppModuleBasic
	key sdk.StoreKey
}

func NewStoreAppModule(key sdk.StoreKey) module.AppModule {
	return module.NewGenesisOnlyAppModule(StoreAppModule{key: key})
}

func (am StoreAppModule) InitGenesis(ctx sdk.Context, _ codec.JSONCodec, bz json.RawMessage) []abci.ValidatorUpdate {
	var gs StoreGenesis
	if err := json.Unmarshal(bz, &gs); err != nil {
		panic(fmt.Errorf("lxphost: decode genesis: %w", err))
	}
	if err := InitStoreGenesis(ctx, am.key, gs); err != nil {
		panic(fmt.Errorf("lxphost: initialize genesis: %w", err))
	}
	return []abci.ValidatorUpdate{}
}

func (am StoreAppModule) ExportGenesis(ctx sdk.Context, _ codec.JSONCodec) json.RawMessage {
	gs, err := ExportStoreGenesis(ctx, am.key)
	if err != nil {
		panic(fmt.Errorf("lxphost: export genesis: %w", err))
	}
	bz, err := json.Marshal(gs)
	if err != nil {
		panic(fmt.Errorf("lxphost: encode genesis: %w", err))
	}
	return bz
}

func (am StoreAppModule) ExportGenesisStream(ctx sdk.Context, cdc codec.JSONCodec) <-chan json.RawMessage {
	ch := make(chan json.RawMessage, 1)
	ch <- am.ExportGenesis(ctx, cdc)
	close(ch)
	return ch
}

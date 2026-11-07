package keeper_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	ledgertypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/stretchr/testify/require"
)

func TestLedgerAuthGenesisExportImport(t *testing.T) {
	for _, tc := range []struct {
		name   string
		ledger ledgertypes.Params
		auth   authtypes.Params
	}{
		{
			name:   "defaults",
			ledger: ledgertypes.DefaultParams(),
			auth:   authtypes.DefaultParams(),
		},
		{
			name:   "paused_with_retention",
			ledger: ledgertypes.Params{Paused: true},
			auth:   authtypes.Params{Paused: true, IdempotencyRetention: 37},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := app.Setup(t, false, false, false)
			t.Cleanup(func() { require.NoError(t, source.Close()) })
			at := time.Unix(1_800_000_000, 0).UTC()
			header := tmproto.Header{ChainID: "pax-test", Time: at}
			ctx := source.NewContext(false, header)
			require.True(t, ctx.KVStore(source.GetKey(ledgertypes.StoreKey)).Has(ledgertypes.ParamsKey))
			require.True(t, ctx.KVStore(source.GetKey(authtypes.StoreKey)).Has(authtypes.ParamsKey))
			require.Equal(t, ledgertypes.DefaultParams(), source.LayerXLedgerKeeper.GetParams(ctx))
			require.Equal(t, authtypes.DefaultParams(), source.LayerXAuthKeeper.GetParams(ctx))
			require.NoError(t, source.LayerXLedgerKeeper.SetParams(ctx, tc.ledger))
			require.NoError(t, source.LayerXAuthKeeper.SetParams(ctx, tc.auth))
			wantLedger := source.LayerXLedgerKeeper.GetParams(ctx)
			wantAuth := source.LayerXAuthKeeper.GetParams(ctx)
			require.Equal(t, tc.ledger, wantLedger)
			require.Equal(t, tc.auth, wantAuth)

			_, err := source.Commit(context.Background())
			require.NoError(t, err)
			exported, err := source.ExportAppStateAndValidators(false, nil)
			require.NoError(t, err)
			var state app.GenesisState
			require.NoError(t, json.Unmarshal(exported.AppState, &state))
			require.Contains(t, state, ledgertypes.ModuleName)
			require.Contains(t, state, authtypes.ModuleName)

			var ledgerGenesis ledgertypes.GenesisState
			require.NoError(t, ledgertypes.ModuleCdc.UnmarshalAsJSON(state[ledgertypes.ModuleName], &ledgerGenesis))
			require.NoError(t, ledgerGenesis.Validate())
			require.Equal(t, wantLedger, ledgerGenesis.Params)
			var authGenesis authtypes.GenesisState
			require.NoError(t, authtypes.ModuleCdc.UnmarshalAsJSON(state[authtypes.ModuleName], &authGenesis))
			require.NoError(t, authGenesis.Validate())
			require.Equal(t, wantAuth, authGenesis.Params)

			restored := app.Setup(t, true, false, false)
			t.Cleanup(func() { require.NoError(t, restored.Close()) })
			_, err = restored.InitChain(context.Background(), &abci.RequestInitChain{
				Time:            at,
				ChainId:         header.ChainID,
				InitialHeight:   exported.Height,
				ConsensusParams: exported.ConsensusParams,
				AppStateBytes:   exported.AppState,
			})
			require.NoError(t, err)
			restoredCtx := restored.NewContext(false, header)
			require.True(t, restoredCtx.KVStore(restored.GetKey(ledgertypes.StoreKey)).Has(ledgertypes.ParamsKey))
			require.True(t, restoredCtx.KVStore(restored.GetKey(authtypes.StoreKey)).Has(authtypes.ParamsKey))
			require.Equal(t, wantLedger, restored.LayerXLedgerKeeper.GetParams(restoredCtx))
			require.Equal(t, wantAuth, restored.LayerXAuthKeeper.GetParams(restoredCtx))
		})
	}
}

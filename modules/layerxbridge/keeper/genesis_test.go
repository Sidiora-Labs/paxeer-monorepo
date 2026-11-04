package keeper_test

import (
	"context"
	"encoding/json"
	"math/big"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxbridge/keeper"
	bridgetestutil "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxbridge/testutil"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxbridge/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func TestGenesisRoundTripsTheRegisteredSidioraPair(t *testing.T) {
	s := newSuite(t, true)
	require.NoError(t, s.k.RegisterChain(s.ctx, types.MsgRegisterChain{Authority: authority, Chain: solanaChain}))
	denom, err := s.k.RegisterSidioraPair(s.ctx, types.MsgRegisterSidioraPair{Authority: authority, ChainID: types.SidioraHomeChainID})
	require.NoError(t, err)
	require.Equal(t, types.SidioraDenom(), denom)
	require.NoError(t, s.k.SetCap(s.ctx, *sidioraCap(authority)))
	in := sidioraDeposit(1, 700)
	in.ChainID = types.SidioraHomeChainID
	_, err = s.k.BridgeIn(s.ctx, in, s.signed(in, s.attestors[0], s.attestors[1]))
	require.NoError(t, err)

	pair := types.BridgedAsset{ChainID: types.SidioraHomeChainID, Asset: sidioraAsset(), Denom: denom}
	limit := types.Cap{Denom: denom, MaxInFlight: sdk.NewInt(1500), MaxPerTx: sdk.NewInt(1000)}
	flying := types.InFlight{Denom: denom, Amount: sdk.NewInt(700)}
	exported := s.k.ExportGenesis(s.ctx)
	require.NoError(t, exported.Validate())
	require.Contains(t, exported.Assets, pair)
	require.Contains(t, exported.Caps, limit)
	require.Contains(t, exported.InFlight, flying)

	fresh, ctx := bridgetestutil.NewKeeper(s.app, s.ctx)
	fresh.InitGenesis(ctx, exported)
	record, found := fresh.GetAsset(ctx, types.SidioraHomeChainID, sidioraAsset())
	require.True(t, found)
	require.Equal(t, pair, record)
	reverse, found := fresh.GetAssetByDenom(ctx, denom)
	require.True(t, found)
	require.Equal(t, pair, reverse)
	imported, found := fresh.GetCap(ctx, denom)
	require.True(t, found)
	require.Equal(t, limit, imported)
	require.Equal(t, flying.Amount, fresh.InFlight(ctx, denom))
	require.Equal(t, exported, fresh.ExportGenesis(ctx))
}

// persistentBridgeApp owns a complete application and a disk-backed database.
// Reopening uses the same home and store, after releasing the database lock.
type persistentBridgeApp struct {
	s     *suite
	open  func() (*app.App, func())
	close func()
}

func newPersistentBridgeApp(t *testing.T, state app.GenesisState, height int64) *persistentBridgeApp {
	t.Helper()
	home, dir := t.TempDir(), t.TempDir()
	open := func() (*app.App, func()) {
		database, err := dbm.NewGoLevelDB("application", dir)
		require.NoError(t, err)
		a := app.New(database, nil, true, map[int64]bool{}, home, 1, false, nil,
			app.MakeEncodingConfig(), wasm.EnableAllProposals, app.TestAppOpts{}, app.EmptyWasmOpts, nil)
		closed := false
		closeApp := func() {
			if !closed {
				require.NoError(t, a.Close())
				closed = true
			}
		}
		t.Cleanup(closeApp)
		return a, closeApp
	}
	a, closeApp := open()
	if state == nil {
		state = app.NewDefaultGenesisState(a.AppCodec())
	}
	encoded, err := json.Marshal(state)
	require.NoError(t, err)
	at := time.Unix(1_800_000_000, 0).UTC()
	_, err = a.InitChain(context.Background(), &abci.RequestInitChain{
		Time: at, ChainId: "pax-test", InitialHeight: height,
		ConsensusParams: app.DefaultConsensusParams, AppStateBytes: encoded,
	})
	require.NoError(t, err)
	return &persistentBridgeApp{s: &suite{t: t, app: a, k: a.LayerXBridgeKeeper,
		ctx:       a.NewContext(false, tmproto.Header{ChainID: "pax-test", Height: height, Time: at}),
		attestors: bridgetestutil.Attestors(3)}, open: open, close: closeApp}
}

func (p *persistentBridgeApp) recover() []*suite {
	s := p.s
	t := s.t
	t.Helper()
	wantBridge := s.k.ExportGenesis(s.ctx)
	require.NoError(t, wantBridge.Validate())
	wantBank := s.app.AppCodec().MustMarshalJSON(s.app.BankKeeper.ExportGenesis(s.ctx))
	wantFactory := s.app.AppCodec().MustMarshalJSON(s.app.TokenFactoryKeeper.ExportGenesis(s.ctx))
	require.NoError(t, s.app.SetSealedStateToCommit(s.ctx, s.ctx.MultiStore().(sdk.CacheMultiStore)))
	_, err := s.app.Commit(context.Background())
	require.NoError(t, err)
	height := s.app.LastBlockHeight()
	require.Equal(t, s.ctx.BlockHeight(), height)
	exported, err := s.app.ExportAppStateAndValidators(false, nil)
	require.NoError(t, err)
	var state app.GenesisState
	require.NoError(t, json.Unmarshal(exported.AppState, &state))
	var carried types.GenesisState
	require.NoError(t, json.Unmarshal(state[types.ModuleName], &carried))
	require.NoError(t, carried.Validate())
	require.Equal(t, wantBridge, carried)
	p.close()
	reopened, closeApp := p.open()
	p.close = closeApp
	require.Equal(t, height, reopened.LastBlockHeight())
	ctx := reopened.NewContext(true, tmproto.Header{ChainID: "pax-test", Height: height + 1, Time: s.ctx.BlockTime()})
	recovered := &suite{t: t, app: reopened, k: reopened.LayerXBridgeKeeper, ctx: ctx,
		attestors: s.attestors, denom: s.denom}
	p.s = recovered
	imported := newPersistentBridgeApp(t, state, exported.Height).s
	imported.attestors, imported.denom = s.attestors, s.denom
	for _, fresh := range []*suite{recovered, imported} {
		require.Equal(t, wantBridge, fresh.k.ExportGenesis(fresh.ctx))
		require.JSONEq(t, string(wantBank), string(fresh.app.AppCodec().MustMarshalJSON(fresh.app.BankKeeper.ExportGenesis(fresh.ctx))))
		require.JSONEq(t, string(wantFactory), string(fresh.app.AppCodec().MustMarshalJSON(fresh.app.TokenFactoryKeeper.ExportGenesis(fresh.ctx))))
	}
	return []*suite{recovered, imported}
}

func testSidioraPersistentRecovery(t *testing.T) {
	t.Helper()
	p := newPersistentBridgeApp(t, nil, 1)
	s := p.s
	s.denom = types.SidioraDenom()
	router := s.router()
	require.NoError(t, s.route(router, &types.MsgRegisterChain{Authority: authority, Chain: solanaChain}))
	require.NoError(t, s.route(router, &types.MsgSetAttestors{Authority: authority, Set: bridgetestutil.Set(s.attestors, 1_000_000, 2)}))
	require.NoError(t, s.route(router, &types.MsgRegisterSidioraPair{Authority: authority, ChainID: types.SidioraHomeChainID}))
	require.NoError(t, s.route(router, sidioraCap(authority)))
	in := sidioraDeposit(71, 900)
	signatures := s.signed(in, s.attestors[0], s.attestors[1])
	_, err := s.k.BridgeIn(s.ctx, in, signatures)
	require.NoError(t, err)
	out, err := s.k.BridgeOut(s.ctx, recipient, in.ChainID, in.Asset, big.NewInt(400), types.Address20{0x33})
	require.NoError(t, err)
	require.Equal(t, uint64(1), out.Nonce)
	for index, fresh := range p.recover() {
		t.Run([]string{"reopened", "imported"}[index], func(t *testing.T) {
			metadata, found := fresh.app.BankKeeper.GetDenomMetaData(fresh.ctx, fresh.denom)
			require.True(t, found)
			require.Equal(t, keeper.SidioraMetadata(), metadata)
			require.NoError(t, metadata.Validate())
			admin, err := fresh.app.TokenFactoryKeeper.GetAuthorityMetadata(fresh.ctx, fresh.denom)
			require.NoError(t, err)
			require.Equal(t, fresh.k.ModuleAddress().String(), admin.Admin)
			pair, found := fresh.k.GetAsset(fresh.ctx, types.SidioraHomeChainID, sidioraAsset())
			require.True(t, found)
			require.Equal(t, types.BridgedAsset{ChainID: types.SidioraHomeChainID, Asset: sidioraAsset(), Denom: fresh.denom}, pair)
			reverse, found := fresh.k.GetAssetByDenom(fresh.ctx, fresh.denom)
			require.True(t, found)
			require.Equal(t, pair, reverse)
			assertRecoveredBridgeRelease(t, fresh, in, signatures, 500, 100)
		})
	}
}

func TestGenesisSidioraSurvivesPersistentReopenAndImport(t *testing.T) {
	testSidioraPersistentRecovery(t)
}

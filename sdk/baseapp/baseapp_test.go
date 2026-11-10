package baseapp

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	store "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	storev2rootmulti "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/storev2/rootmulti"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/legacy/legacytx"
	storageconfig "github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
)

var (
	capKey1 = sdk.NewKVStoreKey("bank")
	capKey2 = sdk.NewKVStoreKey("staking")
)

func newBaseApp(name string, options ...func(*BaseApp)) *BaseApp {
	db := dbm.NewMemDB()
	codec := codec.NewLegacyAmino()
	registerTestCodec(codec)
	return NewBaseApp(name, db, testTxDecoder(codec), nil, &testutil.TestAppOpts{}, options...)
}

func registerTestCodec(cdc *codec.LegacyAmino) {
	// register Tx, Msg
	sdk.RegisterLegacyAminoCodec(cdc)

	// register test types
	cdc.RegisterConcrete(&txTest{}, "cosmos-sdk/baseapp/txTest", nil)
	cdc.RegisterConcrete(&msgCounter{}, "cosmos-sdk/baseapp/msgCounter", nil)
	cdc.RegisterConcrete(&msgCounter2{}, "cosmos-sdk/baseapp/msgCounter2", nil)
	cdc.RegisterConcrete(&msgKeyValue{}, "cosmos-sdk/baseapp/msgKeyValue", nil)
	cdc.RegisterConcrete(&msgNoRoute{}, "cosmos-sdk/baseapp/msgNoRoute", nil)
}

// aminoTxEncoder creates a amino TxEncoder for testing purposes.
func aminoTxEncoder() sdk.TxEncoder {
	cdc := codec.NewLegacyAmino()
	registerTestCodec(cdc)

	return legacytx.StdTxConfig{Cdc: cdc}.TxEncoder()
}

// simple one store baseapp
func setupBaseApp(t *testing.T, options ...func(*BaseApp)) *BaseApp {
	app := newBaseApp(t.Name(), options...)
	require.Equal(t, t.Name(), app.Name())

	app.MountStores(capKey1, capKey2)
	app.SetParamStore(&paramStore{db: dbm.NewMemDB()})

	// stores are mounted
	err := app.LoadLatestVersion()
	require.Nil(t, err)
	return app
}

func newVersionedTestStore(t *testing.T, homeDir string) *storev2rootmulti.Store {
	t.Helper()
	scConfig := storageconfig.DefaultStateCommitConfig()
	scConfig.MemIAVLConfig.SnapshotInterval = 1
	scConfig.MemIAVLConfig.SnapshotMinTimeInterval = 0
	scConfig.MemIAVLConfig.AsyncCommitBuffer = 0
	ssConfig := storageconfig.DefaultStateStoreConfig()
	ssConfig.AsyncWriteBuffer = 0
	return storev2rootmulti.NewStore(homeDir, scConfig, ssConfig, nil)
}

func setupBaseAppWithVersionedState(t *testing.T, options ...func(*BaseApp)) *BaseApp {
	t.Helper()
	cms := newVersionedTestStore(t, t.TempDir())
	options = append([]func(*BaseApp){func(app *BaseApp) { app.SetCMS(cms) }}, options...)
	app := setupBaseApp(t, options...)
	t.Cleanup(func() { require.NoError(t, app.Close()) })
	return app
}

func requireVersionedTestValue(t *testing.T, app *BaseApp, height int64, storeKey sdk.StoreKey, key, value []byte) {
	t.Helper()
	cms := app.cms.(*storev2rootmulti.Store)
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		got, err := cms.GetStateStore().Get(storeKey.Name(), height, key)
		require.NoError(c, err)
		require.Equal(c, value, got)
	}, 5*time.Second, time.Millisecond, "historical state writer must publish the committed value")
}

func TestLoadVersionPruning(t *testing.T) {
	pruningOptions := store.PruningOptions{
		KeepRecent: 2,
		KeepEvery:  3,
		Interval:   1,
	}
	pruningOpt := SetPruning(pruningOptions)
	db := dbm.NewMemDB()
	name := t.Name()
	homeDir := t.TempDir()
	cmsOpt := func(app *BaseApp) { app.SetCMS(newVersionedTestStore(t, homeDir)) }
	app := NewBaseApp(name, db, nil, nil, &testutil.TestAppOpts{}, cmsOpt, pruningOpt)
	t.Cleanup(func() { require.NoError(t, app.Close()) })

	// make a cap key and mount the store
	capKey := sdk.NewKVStoreKey("key1")
	app.MountStores(capKey)

	err := app.LoadLatestVersion() // needed to make stores non-nil
	require.Nil(t, err)

	emptyCommitID := sdk.CommitID{}

	// fresh store has zero/empty last commit
	lastHeight := app.LastBlockHeight()
	lastID := app.LastCommitID()
	require.Equal(t, int64(0), lastHeight)
	require.Equal(t, emptyCommitID, lastID)

	var lastCommitID sdk.CommitID

	// Commit seven blocks, of which 7 (latest) is kept in addition to 6, 5
	// (keep recent) and 3 (keep every).
	for i := int64(1); i <= 7; i++ {
		app.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{Header: &tmproto.Header{ChainID: app.ChainID, Height: i}})
		app.SetDeliverStateToCommit()
		app.Commit(context.Background())
		lastCommitID = sdk.CommitID{Version: i}
	}

	for _, v := range []int64{1, 2, 4} {
		_, err = app.cms.CacheMultiStoreWithVersion(v)
		require.NoError(t, err)
	}

	for _, v := range []int64{3, 5, 6, 7} {
		_, err = app.cms.CacheMultiStoreWithVersion(v)
		require.NoError(t, err)
	}

	// reload with LoadLatestVersion, check it loads last version
	require.NoError(t, app.Close())
	app = NewBaseApp(name, dbm.NewMemDB(), nil, nil, &testutil.TestAppOpts{}, cmsOpt, pruningOpt)
	app.MountStores(capKey)

	err = app.LoadLatestVersion()
	require.Nil(t, err)
	testLoadVersionHelper(t, app, int64(7), lastCommitID)
}

func testLoadVersionHelper(t *testing.T, app *BaseApp, expectedHeight int64, expectedID sdk.CommitID) {
	lastHeight := app.LastBlockHeight()
	lastID := app.LastCommitID()
	require.Equal(t, expectedHeight, lastHeight)
	require.Equal(t, expectedID.Version, lastID.Version)
}

func TestSetMinGasPrices(t *testing.T) {
	minGasPrices := sdk.DecCoins{sdk.NewInt64DecCoin("uhpx", 5000)}
	app := newBaseApp(t.Name(), SetMinGasPrices(minGasPrices.String()))
	require.Equal(t, minGasPrices, app.minGasPrices)
}

func TestSetOccEnabled(t *testing.T) {
	app := newBaseApp(t.Name(), SetOccEnabled(true))
	require.True(t, app.OccEnabled())
}

func TestListSnapshots(t *testing.T) {
	app := setupBaseAppWithSnapshots(t, 2, 5)

	expected := abci.ResponseListSnapshots{Snapshots: []*abci.Snapshot{
		{Height: 2, Format: 1, Chunks: 2},
	}}

	resp, _ := app.ListSnapshots(context.Background(), &abci.RequestListSnapshots{})
	queryResponse, _ := app.Query(context.Background(), &abci.RequestQuery{
		Path: "/app/snapshots",
	})

	queryListSnapshotsResp := abci.ResponseListSnapshots{}
	err := json.Unmarshal(queryResponse.Value, &queryListSnapshotsResp)
	require.NoError(t, err)

	for i, s := range resp.Snapshots {
		querySnapshot := queryListSnapshotsResp.Snapshots[i]
		assert.Equal(t, *s, *querySnapshot)
		assert.NotEmpty(t, s.Hash)
		assert.NotEmpty(t, s.Metadata)
		s.Hash = nil
		s.Metadata = nil
	}
	assert.Equal(t, expected, *resp)
}

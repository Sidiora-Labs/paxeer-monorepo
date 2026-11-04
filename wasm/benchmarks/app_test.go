package benchmarks

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	dbm "github.com/tendermint/tm-db"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"

	paxapp "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm"
	wasmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/types"
)

func setup(b testing.TB, db benchmarkDatabase, withGenesis bool, opts ...wasm.Option) (*paxapp.App, paxapp.GenesisState) {
	b.Helper()
	wasmApp := paxapp.New(dbm.NewMemDB(), nil, true, map[int64]bool{}, b.TempDir(), 0,
		false, config.TestConfig(), paxapp.MakeEncodingConfig(), wasm.EnableAllProposals,
		benchmarkAppOptions{database: db}, opts, nil)
	b.Cleanup(func() { require.NoError(b, wasmApp.Close()) })
	if withGenesis {
		return wasmApp, paxapp.NewDefaultGenesisState(paxapp.MakeEncodingConfig().Marshaler)
	}
	return wasmApp, paxapp.GenesisState{}
}

// SetupWithGenesisAccounts initializes a new WasmApp with the provided genesis
// accounts and possible balances.
func SetupWithGenesisAccounts(b testing.TB, db benchmarkDatabase, genAccs []authtypes.GenesisAccount, balances ...banktypes.Balance) *paxapp.App {
	return setupWithGenesisAccounts(b, db, genAccs, nil, balances...)
}

func setupWithGenesisAccounts(b testing.TB, db benchmarkDatabase, genAccs []authtypes.GenesisAccount, opts []wasm.Option, balances ...banktypes.Balance) *paxapp.App {
	b.Helper()
	wasmApp, genesisState := setup(b, db, true, opts...)
	authGenesis := authtypes.NewGenesisState(authtypes.DefaultParams(), genAccs)
	appCodec := paxapp.MakeEncodingConfig().Marshaler

	genesisState[authtypes.ModuleName] = appCodec.MustMarshalJSON(authGenesis)

	totalSupply := sdk.NewCoins()
	for _, b := range balances {
		totalSupply = totalSupply.Add(b.Coins...)
	}

	bankGenesis := banktypes.NewGenesisState(banktypes.DefaultGenesisState().Params, balances, totalSupply, []banktypes.Metadata{}, banktypes.DefaultGenesisState().WeiBalances)
	genesisState[banktypes.ModuleName] = appCodec.MustMarshalJSON(bankGenesis)

	stateBytes, err := json.MarshalIndent(genesisState, "", " ")
	if err != nil {
		panic(err)
	}

	_, err = wasmApp.InitChain(
		context.Background(),
		&abci.RequestInitChain{
			ConsensusParams: paxapp.DefaultConsensusParams,
			ChainId:         "pax-test",
			Time:            time.Unix(1, 0).UTC(),
			AppStateBytes:   stateBytes,
		},
	)

	require.NoError(b, err)
	wasmApp.SetDeliverStateToCommit()
	_, err = wasmApp.Commit(context.Background())
	require.NoError(b, err)
	require.EqualValues(b, 1, wasmApp.LastBlockHeight())

	return wasmApp
}

type AppInfo struct {
	App          *paxapp.App
	MinterKey    *secp256k1.PrivKey
	MinterAddr   sdk.AccAddress
	ContractAddr string
	Denom        string
	AccNum       uint64
	SeqNum       uint64
	TxConfig     client.TxConfig
}

func InitializeWasmApp(b testing.TB, db benchmarkDatabase, numAccounts int, opts ...wasm.Option) AppInfo {
	b.Helper()
	// constants
	minter := secp256k1.GenPrivKey()
	addr := sdk.AccAddress(minter.PubKey().Address())
	denom := "uatom"

	// genesis setup (with a bunch of random accounts)
	genAccs := make([]authtypes.GenesisAccount, numAccounts+1)
	bals := make([]banktypes.Balance, numAccounts+1)
	genAccs[0] = &authtypes.BaseAccount{
		Address: addr.String(),
	}
	bals[0] = banktypes.Balance{
		Address: addr.String(),
		Coins:   sdk.NewCoins(sdk.NewInt64Coin(denom, 100000000000)),
	}
	for i := 0; i <= numAccounts; i++ {
		acct := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()
		if i == 0 {
			acct = addr.String()
		}
		genAccs[i] = &authtypes.BaseAccount{
			Address: acct,
		}
		bals[i] = banktypes.Balance{
			Address: acct,
			Coins: sdk.NewCoins(
				sdk.NewInt64Coin(denom, 100000000000),
				sdk.NewInt64Coin(sdk.DefaultBondDenom, 100000000000),
			),
		}
	}
	wasmApp := setupWithGenesisAccounts(b, db, genAccs, opts, bals...)

	// add wasm contract
	txGen := paxapp.MakeEncodingConfig().TxConfig

	// upload the code
	cw20Code, err := os.ReadFile("./testdata/cw20_base.wasm")
	require.NoError(b, err)
	storeMsg := wasmtypes.MsgStoreCode{
		Sender:       addr.String(),
		WASMByteCode: cw20Code,
	}
	storeTx, err := paxapp.GenTx(txGen, []sdk.Msg{&storeMsg}, benchmarkFees(b, wasmApp, 55123123), 55123123, wasmApp.ChainID, []uint64{0}, []uint64{0}, minter)
	require.NoError(b, err)
	storeResults := commitBenchmarkBlock(b, wasmApp, txGen, []sdk.Tx{storeTx})
	var codeID uint64
	for _, event := range storeResults[0].Events {
		if event.Type == wasmtypes.EventTypeStoreCode {
			for _, attr := range event.Attributes {
				if string(attr.Key) == wasmtypes.AttributeKeyCodeID {
					codeID, err = strconv.ParseUint(string(attr.Value), 10, 64)
					require.NoError(b, err)
				}
			}
		}
	}
	require.Positive(b, codeID)

	// instantiate the contract
	initialBalances := make([]balance, numAccounts+1)
	for i := 0; i <= numAccounts; i++ {
		acct := sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()
		if i == 0 {
			acct = addr.String()
		}
		initialBalances[i] = balance{
			Address: acct,
			Amount:  1000000000,
		}
	}
	init := cw20InitMsg{
		Name:            "Cash Money",
		Symbol:          "CASH",
		Decimals:        2,
		InitialBalances: initialBalances,
	}
	initBz, err := json.Marshal(init)
	require.NoError(b, err)
	initMsg := wasmtypes.MsgInstantiateContract{
		Sender: addr.String(),
		Admin:  addr.String(),
		CodeID: codeID,
		Label:  "Demo contract",
		Msg:    initBz,
	}
	gasWanted := 500000 + 10000*uint64(numAccounts)
	initTx, err := paxapp.GenTx(txGen, []sdk.Msg{&initMsg}, benchmarkFees(b, wasmApp, gasWanted), gasWanted, wasmApp.ChainID, []uint64{0}, []uint64{1}, minter)
	require.NoError(b, err)
	results := commitBenchmarkBlock(b, wasmApp, txGen, []sdk.Tx{initTx})
	var contractAddr string
	for _, event := range results[0].Events {
		if event.Type == wasmtypes.EventTypeInstantiate {
			for _, attr := range event.Attributes {
				if string(attr.Key) == wasmtypes.AttributeKeyContractAddr {
					contractAddr = string(attr.Value)
				}
			}
		}
	}
	require.NotEmpty(b, contractAddr)
	contract, err := sdk.AccAddressFromBech32(contractAddr)
	require.NoError(b, err)
	require.NotNil(b, wasmApp.WasmKeeper.GetContractInfo(wasmApp.GetCheckCtx(), contract))

	return AppInfo{
		App:          wasmApp,
		MinterKey:    minter,
		MinterAddr:   addr,
		ContractAddr: contractAddr,
		Denom:        denom,
		AccNum:       0,
		SeqNum:       2,
		TxConfig:     paxapp.MakeEncodingConfig().TxConfig,
	}
}

func GenSequenceOfTxs(b testing.TB, info *AppInfo, msgGen func(*AppInfo) ([]sdk.Msg, error), numToGenerate int) []sdk.Tx {
	fees := benchmarkFees(b, info.App, 1234567)
	txs := make([]sdk.Tx, numToGenerate)

	for i := 0; i < numToGenerate; i++ {
		msgs, err := msgGen(info)
		require.NoError(b, err)
		txs[i], err = paxapp.GenTx(
			info.TxConfig,
			msgs,
			fees,
			1234567,
			info.App.ChainID,
			[]uint64{info.AccNum},
			[]uint64{info.SeqNum},
			info.MinterKey,
		)
		require.NoError(b, err)
		info.SeqNum += 1
	}

	return txs
}

func benchmarkFees(b testing.TB, app *paxapp.App, gas uint64) sdk.Coins {
	b.Helper()
	ctx := app.GetCheckCtx()
	params := app.ParamsKeeper.GetFeesParams(ctx)
	prices := params.GetGlobalMinimumGasPrices().UnionMax(ctx.MinGasPrices())
	price := prices.AmountOf(sdk.DefaultBondDenom)
	require.True(b, price.IsPositive(), "benchmark transactions must pay the chain's minimum fee")
	fee := price.Mul(sdk.NewDecFromInt(sdk.NewIntFromUint64(gas))).Ceil().RoundInt()
	return sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, fee))
}

func commitBenchmarkBlock(b testing.TB, app *paxapp.App, txConfig client.TxConfig, txs []sdk.Tx) []*abci.ExecTxResult {
	b.Helper()
	height := app.LastBlockHeight() + 1
	raw := make([][]byte, len(txs))
	for i, tx := range txs {
		var err error
		raw[i], err = txConfig.TxEncoder()(tx)
		require.NoError(b, err)
	}
	// FinalizeBlock runs the node's BeginBlock, delivery, deferred writes and EndBlock.
	res, err := app.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{
		Header: &tmproto.Header{Height: height, ChainID: app.ChainID, Time: time.Unix(height, 0).UTC()},
		Txs:    raw,
	})
	require.NoError(b, err)
	require.NotNil(b, res)
	require.Len(b, res.TxResults, len(txs))
	for i, result := range res.TxResults {
		require.NotNil(b, result)
		require.Zero(b, result.Code, "transaction %d at height %d: %s", i, height, result.Log)
	}
	_, err = app.Commit(context.Background())
	require.NoError(b, err)
	require.Equal(b, height, app.LastBlockHeight())
	return res.TxResults
}

package simulation

import (
	"bytes"
	"context"
	"math"
	"math/rand"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	cryptocodec "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	simtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/simulation"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx/signing"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/ante"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authsign "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/signing"
	authtx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/tx"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func TestRepairM_633_TransitionWeightValidation(t *testing.T) {
	cases := []struct {
		name    string
		weights [][]int
		reason  string
	}{
		{"nil", nil, "empty state set"},
		{"empty", [][]int{}, "empty state set"},
		{"short row", [][]int{{1, 1}, {1}}, "non-square"},
		{"long row", [][]int{{1, 1, 1}, {1, 1}}, "non-square"},
		{"negative", [][]int{{1, -1}, {1, 2}}, "negative weight at row 0 column 1"},
		{"zero column", [][]int{{1, 0}, {2, 0}}, "zero total in column 1"},
		{"overflow", [][]int{{math.MaxInt, 1}, {1, 1}}, "total overflows in column 0"},
		{"overflow wraps positive", [][]int{{math.MaxInt, 1, 1}, {math.MaxInt, 1, 1}, {3, 1, 1}}, "total overflows"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := CreateTransitionMatrix(tc.weights)
			require.ErrorContains(t, err, tc.reason)
		})
	}
	matrix, err := CreateTransitionMatrix([][]int{{0, math.MaxInt}, {math.MaxInt, 0}})
	require.NoError(t, err)
	r := rand.New(rand.NewSource(633))
	for i := 0; i < 100; i++ {
		require.Equal(t, 1, matrix.NextState(r, 0))
		require.Equal(t, 0, matrix.NextState(r, 1))
	}
	require.PanicsWithValue(t, "transition matrix: state -1 outside [0,2)", func() { matrix.NextState(r, -1) })
	require.PanicsWithValue(t, "transition matrix: state 2 outside [0,2)", func() { matrix.NextState(r, 2) })
	require.PanicsWithValue(t, "transition matrix: nil random source", func() { matrix.NextState(nil, 0) })
	require.PanicsWithValue(t, "transition matrix: state 0 outside [0,0)", func() { (TransitionMatrix{}).NextState(r, 0) })
	weighted, err := CreateTransitionMatrix([][]int{{0, 0, 0}, {2, 2, 2}, {3, 3, 3}})
	require.NoError(t, err)
	draws := rand.New(rand.NewSource(31))
	actual := rand.New(rand.NewSource(31))
	seen := map[int]bool{}
	for i := 0; i < 100; i++ {
		expected := 2
		if draws.Intn(5) < 2 {
			expected = 1
		}
		got := weighted.NextState(actual, i%3)
		require.Equal(t, expected, got)
		seen[got] = true
	}
	require.Len(t, seen, 2)
}

func TestRepairM_633_InitialStateValidation(t *testing.T) {
	r := rand.New(rand.NewSource(633))
	cases := []struct {
		weights []int
		reason  string
	}{
		{nil, "initial state: empty state set"},
		{[]int{}, "initial state: empty state set"},
		{[]int{0, 0}, "initial state: zero total"},
		{[]int{1, -1}, "initial state: negative weight at state 1"},
		{[]int{math.MaxInt, 1}, "initial state: total overflows"},
	}
	for _, tc := range cases {
		require.PanicsWithValue(t, tc.reason, func() { GetMemberOfInitialState(r, tc.weights) })
	}
	require.PanicsWithValue(t, "initial state: nil random source", func() { GetMemberOfInitialState(nil, []int{1}) })
	for i := 0; i < 100; i++ {
		require.Equal(t, 1, GetMemberOfInitialState(r, []int{0, math.MaxInt, 0}))
	}
}

func TestRepairM_633_TransitionWeightAlias(t *testing.T) {
	weights := [][]int{{1, 0}, {0, 1}}
	matrix, err := CreateTransitionMatrix(weights)
	require.NoError(t, err)
	weights[0][0] = 0
	weights[1][0] = math.MaxInt
	weights[1][1] = -1
	weights[0] = nil
	weights = append(weights, []int{-1})
	require.Len(t, weights, 3)
	r := rand.New(rand.NewSource(633))
	for i := 0; i < 100; i++ {
		require.Equal(t, 0, matrix.NextState(r, 0))
		require.Equal(t, 1, matrix.NextState(r, 1))
	}
}

func repair633Codec() (*codec.ProtoCodec, client.TxConfig) {
	registry := codectypes.NewInterfaceRegistry()
	sdk.RegisterInterfaces(registry)
	cryptocodec.RegisterInterfaces(registry)
	authtypes.RegisterInterfaces(registry)
	banktypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	return cdc, authtx.NewTxConfig(cdc, authtx.DefaultSignModes)
}

type repair633Fixture struct {
	input     OperationInput
	bank      bankkeeper.BaseKeeper
	account   authkeeper.AccountKeeper
	recipient sdk.AccAddress
	delivered *[][]byte
}

func repair633App(t *testing.T, seed int64) repair633Fixture {
	t.Helper()
	cdc, gen := repair633Codec()
	app := baseapp.NewBaseApp("repair633", dbm.NewMemDB(), gen.TxDecoder(), nil, &testutil.TestAppOpts{})
	t.Cleanup(func() { require.NoError(t, app.Close()) })
	authKey := sdk.NewKVStoreKey(authtypes.StoreKey)
	bankKey := sdk.NewKVStoreKey(banktypes.StoreKey)
	paramsKey := sdk.NewKVStoreKey(paramtypes.StoreKey)
	transientKey := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	app.MountStores(authKey, bankKey, paramsKey, transientKey)
	amino := codec.NewLegacyAmino()
	authParams := paramtypes.NewSubspace(cdc, amino, paramsKey, transientKey, authtypes.ModuleName)
	bankParams := paramtypes.NewSubspace(cdc, amino, paramsKey, transientKey, banktypes.ModuleName)
	ak := authkeeper.NewAccountKeeper(cdc, authKey, authParams, authtypes.ProtoBaseAccount, nil)
	bk := bankkeeper.NewBaseKeeper(cdc, bankKey, ak, bankParams, nil)
	key := secp256k1.GenPrivKeyFromSecret([]byte("repair633 sender"))
	sender := sdk.AccAddress(key.PubKey().Address())
	recipient := sdk.AccAddress(secp256k1.GenPrivKeyFromSecret([]byte("repair633 recipient")).PubKey().Address())
	app.SetInterfaceRegistry(cdc.InterfaceRegistry())
	banktypes.RegisterMsgServer(app.MsgServiceRouter(), bankkeeper.NewMsgServerImpl(bk))
	app.SetAnteHandler(sdk.ChainAnteDecorators(
		ante.NewDefaultSetUpContextDecorator(),
		ante.NewSigVerificationDecorator(ak, gen.SignModeHandler()),
		ante.NewIncrementSequenceDecorator(ak),
	))
	delivered := [][]byte{}
	app.RegisterDeliverTxHook(func(ctx sdk.Context, tx sdk.Tx, checksum [32]byte, result sdk.DeliverTxHookInput) error {
		delivered = append(delivered, bytes.Clone(ctx.TxBytes()))
		encoded, err := gen.TxEncoder()(tx)
		require.NoError(t, err)
		require.Equal(t, encoded, ctx.TxBytes(), "the consumer sees exactly the producer's encoded transaction")
		return nil
	})
	app.SetInitChainer(func(ctx sdk.Context, req abci.RequestInitChain) abci.ResponseInitChain {
		ak.SetParams(ctx, authtypes.DefaultParams())
		acc := ak.NewAccountWithAddress(ctx, sender)
		require.NoError(t, acc.SetPubKey(key.PubKey()))
		ak.SetAccount(ctx, acc)
		genesis := banktypes.DefaultGenesisState()
		genesis.Balances = []banktypes.Balance{{Address: sender.String(), Coins: sdk.NewCoins(sdk.NewInt64Coin("stake", 1000000))}}
		bk.InitGenesis(ctx, genesis)
		return abci.ResponseInitChain{}
	})
	require.NoError(t, app.LoadLatestVersion())
	_, err := app.InitChain(context.Background(), &abci.RequestInitChain{ChainId: "repair633", Time: time.Unix(1700000000, 0).UTC()})
	require.NoError(t, err)
	ctx := *app.DeliverContext()
	coins := sdk.NewCoins(sdk.NewInt64Coin("stake", 7))
	return repair633Fixture{
		input: OperationInput{R: rand.New(rand.NewSource(seed)), App: app, TxGen: gen, Cdc: cdc,
			Msg: banktypes.NewMsgSend(sender, recipient, coins), MsgType: "send", ModuleName: banktypes.ModuleName,
			CoinsSpentInMsg: coins, Context: ctx, SimAccount: simtypes.Account{Address: sender, PrivKey: key}, AccountKeeper: ak, Bankkeeper: bk},
		bank: bk, account: ak, recipient: recipient, delivered: &delivered,
	}
}

func TestRepairM_633_MissingSimulationAccount(t *testing.T) {
	for _, randomFees := range []bool{false, true} {
		t.Run(map[bool]string{false: "explicit fees", true: "random fees"}[randomFees], func(t *testing.T) {
			fixture := repair633App(t, 633)
			in := fixture.input
			fixture.account.RemoveAccount(in.Context, fixture.account.GetAccount(in.Context, in.SimAccount.Address))
			require.Nil(t, fixture.account.GetAccount(in.Context, in.SimAccount.Address))
			before := fixture.bank.GetAllBalances(in.Context, in.SimAccount.Address)
			var msg simtypes.OperationMsg
			var future []simtypes.FutureOperation
			var err error
			if randomFees {
				msg, future, err = GenAndDeliverTxWithRandFees(in)
			} else {
				msg, future, err = GenAndDeliverTx(in, nil)
			}
			require.NoError(t, err)
			require.False(t, msg.OK)
			require.Equal(t, banktypes.ModuleName, msg.Route)
			require.Equal(t, "send", msg.Name)
			require.Contains(t, msg.Comment, in.SimAccount.Address.String())
			require.Contains(t, msg.Comment, "does not exist")
			require.Nil(t, future)
			require.Empty(t, *fixture.delivered)
			require.Equal(t, before, fixture.bank.GetAllBalances(in.Context, in.SimAccount.Address))
			require.Nil(t, fixture.account.GetAccount(in.Context, in.SimAccount.Address))
		})
	}
}

func TestRepairM_633_SeededTransactionReplay(t *testing.T) {
	for _, randomFees := range []bool{false, true} {
		t.Run(map[bool]string{false: "explicit fees", true: "random fees"}[randomFees], func(t *testing.T) {
			replay := func(seed int64) ([][]byte, []byte, []byte) {
				fixture := repair633App(t, seed)
				in := fixture.input
				expectedR := rand.New(rand.NewSource(seed))
				for i := 0; i < 6; i++ {
					var op simtypes.OperationMsg
					var err error
					if randomFees {
						spendable := fixture.bank.SpendableCoins(in.Context, in.SimAccount.Address).Sub(in.CoinsSpentInMsg)
						_, err = simtypes.RandomFees(expectedR, in.Context, spendable)
						require.NoError(t, err)
						op, _, err = GenAndDeliverTxWithRandFees(in)
					} else {
						op, _, err = GenAndDeliverTx(in, sdk.NewCoins(sdk.NewInt64Coin("stake", 3)))
					}
					require.NoError(t, err)
					require.True(t, op.OK, op.Comment)
					require.Len(t, *fixture.delivered, i+1)
					decoded, err := in.TxGen.TxDecoder()((*fixture.delivered)[i])
					require.NoError(t, err)
					expectedMemo := simtypes.RandStringOfLength(expectedR, simtypes.RandIntBetween(expectedR, 0, 100))
					require.Equal(t, expectedMemo, decoded.(interface{ GetMemo() string }).GetMemo())
					require.Equal(t, uint64(i+1), fixture.account.GetAccount(in.Context, in.SimAccount.Address).GetSequence())
				}
				require.Equal(t, sdk.NewInt(999958), fixture.bank.GetBalance(in.Context, in.SimAccount.Address, "stake").Amount)
				require.Equal(t, sdk.NewInt(42), fixture.bank.GetBalance(in.Context, fixture.recipient, "stake").Amount)
				bankState, err := in.Cdc.MarshalAsJSON(fixture.bank.ExportGenesis(in.Context))
				require.NoError(t, err)
				accountState, err := in.Cdc.MarshalInterface(fixture.account.GetAccount(in.Context, in.SimAccount.Address))
				require.NoError(t, err)
				return *fixture.delivered, bankState, accountState
			}
			txs1, bank1, account1 := replay(633)
			txs2, bank2, account2 := replay(633)
			require.Equal(t, txs1, txs2)
			require.Equal(t, bank1, bank2)
			require.Equal(t, account1, account2)
			txs3, _, _ := replay(634)
			require.NotEqual(t, txs1, txs3, "the operation's random source must control the encoded memo")
		})
	}
}

func TestRepairM_633_TransactionSigningErrors(t *testing.T) {
	cdc, gen := repair633Codec()
	key := secp256k1.GenPrivKeyFromSecret([]byte("repair633 signer"))
	address := sdk.AccAddress(key.PubKey().Address())
	msgs := []sdk.Msg{banktypes.NewMsgSend(address, address, sdk.NewCoins(sdk.NewInt64Coin("stake", 1)))}
	var typedNil *secp256k1.PrivKey
	cases := []struct {
		name       string
		nums, seqs []uint64
		keys       []cryptotypes.PrivKey
		reason     string
	}{
		{"no signers", nil, nil, nil, "metadata length mismatch"},
		{"missing account number", nil, []uint64{0}, []cryptotypes.PrivKey{key}, "metadata length mismatch"},
		{"missing sequence", []uint64{0}, nil, []cryptotypes.PrivKey{key}, "metadata length mismatch"},
		{"extra account number", []uint64{0, 1}, []uint64{0}, []cryptotypes.PrivKey{key}, "metadata length mismatch"},
		{"extra sequence", []uint64{0}, []uint64{0, 1}, []cryptotypes.PrivKey{key}, "metadata length mismatch"},
		{"second signer missing sequence", []uint64{0, 1}, []uint64{0}, []cryptotypes.PrivKey{key, key}, "metadata length mismatch"},
		{"nil key", []uint64{0}, []uint64{0}, []cryptotypes.PrivKey{nil}, "nil private key for signer 0"},
		{"typed nil key", []uint64{0}, []uint64{0}, []cryptotypes.PrivKey{typedNil}, "nil private key for signer 0"},
		{"second nil key", []uint64{0, 1}, []uint64{0, 1}, []cryptotypes.PrivKey{key, nil}, "nil private key for signer 1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := GenTx(gen, msgs, nil, DefaultGenTxGas, "repair633", tc.nums, tc.seqs, tc.keys...)
			require.ErrorContains(t, err, tc.reason)
			require.Nil(t, tx)
		})
	}
	// The production handler map admits an unsupported default and returns its real signing error.
	unsupported := authsign.NewSignModeHandlerMap(signing.SignMode_SIGN_MODE_TEXTUAL, nil)
	badGen := authtx.NewTxConfigWithHandler(cdc, unsupported)
	tx, err := GenTx(badGen, msgs, nil, DefaultGenTxGas, "repair633", []uint64{0}, []uint64{0}, key)
	require.ErrorContains(t, err, "sign bytes for signer 0")
	require.ErrorContains(t, err, "can't verify sign mode SIGN_MODE_TEXTUAL")
	require.Nil(t, tx)
	for _, mode := range authtx.DefaultSignModes {
		t.Run(mode.String(), func(t *testing.T) {
			gen := authtx.NewTxConfig(cdc, []signing.SignMode{mode})
			tx, err := GenTxWithMemo(gen, msgs, nil, DefaultGenTxGas, "repair633", "explicit memo", []uint64{5}, []uint64{8}, key)
			require.NoError(t, err)
			bz, err := gen.TxEncoder()(tx)
			require.NoError(t, err)
			decoded, err := gen.TxDecoder()(bz)
			require.NoError(t, err)
			sigs, err := decoded.(authsign.SigVerifiableTx).GetSignaturesV2()
			require.NoError(t, err)
			require.Len(t, sigs, 1)
			require.Equal(t, uint64(8), sigs[0].Sequence)
			require.NoError(t, authsign.VerifySignature(key.PubKey(), authsign.SignerData{ChainID: "repair633", AccountNumber: 5, Sequence: 8}, sigs[0].Data, gen.SignModeHandler(), decoded))
			require.Equal(t, "explicit memo", decoded.(interface{ GetMemo() string }).GetMemo())
		})
	}
	first, err := GenTx(gen, msgs, nil, DefaultGenTxGas, "repair633", []uint64{5}, []uint64{8}, key)
	require.NoError(t, err)
	second, err := GenTx(gen, msgs, nil, DefaultGenTxGas, "repair633", []uint64{5}, []uint64{8}, key)
	require.NoError(t, err)
	firstBytes, err := gen.TxEncoder()(first)
	require.NoError(t, err)
	secondBytes, err := gen.TxEncoder()(second)
	require.NoError(t, err)
	require.Equal(t, firstBytes, secondBytes)
	require.Empty(t, first.(interface{ GetMemo() string }).GetMemo())
}

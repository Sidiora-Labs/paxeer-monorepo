package ante_test

import (
	"testing"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	clienttx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/tx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	cryptocodec "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/codec"
	kmultisig "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/multisig"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types/multisig"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx/signing"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/ante"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authsigning "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/signing"
	authtx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/tx"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	paramskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/keeper"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func repairM655Keepers(t *testing.T) (sdk.Context, authkeeper.AccountKeeper, bankkeeper.BaseKeeper, paramskeeper.Keeper, client.TxConfig) {
	t.Helper()
	registry := codectypes.NewInterfaceRegistry()
	sdk.RegisterInterfaces(registry)
	cryptocodec.RegisterInterfaces(registry)
	authtypes.RegisterInterfaces(registry)
	banktypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	amino := codec.NewLegacyAmino()
	sdk.RegisterLegacyAminoCodec(amino)
	authtypes.RegisterLegacyAminoCodec(amino)
	cryptocodec.RegisterCrypto(amino)

	db := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	cms := store.NewCommitMultiStore(db)
	authKey := sdk.NewKVStoreKey(authtypes.StoreKey)
	bankKey := sdk.NewKVStoreKey(banktypes.StoreKey)
	paramsKey := sdk.NewKVStoreKey(paramtypes.StoreKey)
	deferredKey := sdk.NewKVStoreKey("repair-m655-deferred")
	transientKey := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	for _, key := range []sdk.StoreKey{authKey, bankKey, paramsKey, deferredKey} {
		cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	}
	cms.MountStoreWithDB(transientKey, sdk.StoreTypeTransient, nil)
	require.NoError(t, cms.LoadLatestVersion())
	ctx := sdk.NewContext(cms, tmproto.Header{Height: 1, ChainID: "repair-m655"}, true)
	pk := paramskeeper.NewKeeper(cdc, amino, paramsKey, transientKey)
	ak := authkeeper.NewAccountKeeper(cdc, authKey, pk.Subspace(authtypes.ModuleName), authtypes.ProtoBaseAccount,
		map[string][]string{authtypes.FeeCollectorName: nil})
	ak.SetParams(ctx, authtypes.DefaultParams())
	ak.SetModuleAccount(ctx, authtypes.NewEmptyModuleAccount(authtypes.FeeCollectorName))
	bk := bankkeeper.NewBaseKeeperWithDeferredCache(cdc, bankKey, ak, pk.Subspace(banktypes.ModuleName), nil, deferredKey)
	bk.SetParams(ctx, banktypes.DefaultParams())
	return ctx, ak, bk, pk, authtx.NewTxConfig(cdc, authtx.DefaultSignModes)
}

func TestRepairM_655_AnteParamsConstruction(t *testing.T) {
	ctx, ak, bk, pk, config := repairM655Keepers(t)
	options := ante.HandlerOptions{AccountKeeper: ak, BankKeeper: bk, ParamsKeeper: pk, SignModeHandler: config.SignModeHandler()}
	var keeperPointer *paramskeeper.Keeper
	for _, tc := range []struct {
		name   string
		keeper ante.ParamsKeeper
	}{
		{"missing", nil},
		{"real keeper pointer", &pk},
		{"nil keeper pointer", keeperPointer},
	} {
		t.Run(tc.name, func(t *testing.T) {
			unsupported := options
			unsupported.ParamsKeeper = tc.keeper
			var handler sdk.AnteHandler
			var err error
			require.NotPanics(t, func() { handler, err = ante.NewAnteHandler(unsupported) })
			require.Nil(t, handler)
			require.ErrorIs(t, err, sdkerrors.ErrLogic)
			if tc.keeper != nil {
				require.Contains(t, err.Error(), "requires paramskeeper.Keeper for fee checking")
			}
		})
	}

	handler, err := ante.NewAnteHandler(options)
	require.NoError(t, err)
	require.NotNil(t, handler)
	priv := secp256k1.GenPrivKeyFromSecret([]byte("repair-m655-fee-payer"))
	addr := sdk.AccAddress(priv.PubKey().Address())
	account := ak.NewAccountWithAddress(ctx, addr)
	ak.SetAccount(ctx, account)
	genesis := banktypes.DefaultGenesisState()
	genesis.Balances = []banktypes.Balance{{Address: addr.String(), Coins: sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 100000))}}
	bk.InitGenesis(ctx, genesis)
	pk.SetFeesParams(ctx, paramtypes.NewFeesParams(sdk.NewDecCoins(sdk.NewDecCoinFromDec(sdk.DefaultBondDenom, sdk.NewDecWithPrec(1, 2)))))

	makeTx := func(fee int64) sdk.Tx {
		builder := config.NewTxBuilder()
		require.NoError(t, builder.SetMsgs(&banktypes.MsgSend{FromAddress: addr.String(), ToAddress: addr.String(), Amount: sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 1))}))
		builder.SetGasLimit(200000)
		builder.SetFeeAmount(sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, fee)))
		require.NoError(t, builder.SetSignatures(signing.SignatureV2{PubKey: priv.PubKey(), Data: &signing.SingleSignatureData{SignMode: signing.SignMode_SIGN_MODE_DIRECT}}))
		sig, err := clienttx.SignWithPrivKey(signing.SignMode_SIGN_MODE_DIRECT, authsigning.SignerData{
			ChainID: ctx.ChainID(), AccountNumber: account.GetAccountNumber(), Sequence: account.GetSequence(),
		}, builder, priv, config, account.GetSequence())
		require.NoError(t, err)
		require.NoError(t, builder.SetSignatures(sig))
		wire, err := config.TxEncoder()(builder.GetTx())
		require.NoError(t, err)
		decoded, err := config.TxDecoder()(wire)
		require.NoError(t, err)
		return decoded
	}
	_, err = handler(ctx, makeTx(1999), false)
	require.ErrorIs(t, err, sdkerrors.ErrInsufficientFee)
	require.Equal(t, int64(100000), bk.GetBalance(ctx, addr, sdk.DefaultBondDenom).Amount.Int64())
	newCtx, err := handler(ctx, makeTx(2000), false)
	require.NoError(t, err)
	require.Equal(t, uint64(1), ak.GetAccount(newCtx, addr).GetSequence())
	require.Equal(t, int64(98000), bk.GetBalance(newCtx, addr, sdk.DefaultBondDenom).Amount.Int64())
	bk.WriteDeferredBalances(newCtx)
	require.Equal(t, int64(2000), bk.GetBalance(newCtx, ak.GetModuleAddress(authtypes.FeeCollectorName), sdk.DefaultBondDenom).Amount.Int64())
}

func repairM655Multisig(t *testing.T, nested bool, count int) (*kmultisig.LegacyAminoPubKey, *signing.MultiSignatureData) {
	t.Helper()
	keys := make([]cryptotypes.PubKey, count)
	sigs := make([]signing.SignatureData, count)
	for i := range keys {
		priv := secp256k1.GenPrivKeyFromSecret(append([]byte("repair-m655-signer"), byte(i)))
		keys[i] = priv.PubKey()
		bz, err := priv.Sign([]byte("repair-m655-multisig"))
		require.NoError(t, err)
		sigs[i] = &signing.SingleSignatureData{SignMode: signing.SignMode_SIGN_MODE_DIRECT, Signature: bz}
	}
	if nested {
		keys[0], sigs[0] = repairM655Multisig(t, false, 3)
	}
	key := kmultisig.NewLegacyAminoPubKey(1, keys)
	sig := multisig.NewMultisig(len(keys))
	for _, i := range []int{0, len(keys) - 1} {
		require.NoError(t, multisig.AddSignatureV2(sig, signing.SignatureV2{PubKey: keys[i], Data: sigs[i]}, keys))
	}
	return key, sig
}

func TestRepairM_655_MultisigGasShapes(t *testing.T) {
	params := authtypes.DefaultParams()
	cases := []struct {
		name   string
		nested bool
		mutate func(*signing.MultiSignatureData) *signing.MultiSignatureData
	}{
		{"nil multisignature", false, func(*signing.MultiSignatureData) *signing.MultiSignatureData { return nil }},
		{"nil bitarray", false, func(s *signing.MultiSignatureData) *signing.MultiSignatureData { s.BitArray = nil; return s }},
		{"short signatures", false, func(s *signing.MultiSignatureData) *signing.MultiSignatureData {
			s.Signatures = s.Signatures[:1]
			return s
		}},
		{"extra signatures", false, func(s *signing.MultiSignatureData) *signing.MultiSignatureData {
			s.Signatures = append(s.Signatures, s.Signatures[0])
			return s
		}},
		{"oversized bitarray", false, func(s *signing.MultiSignatureData) *signing.MultiSignatureData {
			s.BitArray = cryptotypes.NewCompactBitArray(4)
			s.BitArray.SetIndex(0, true)
			s.BitArray.SetIndex(3, true)
			return s
		}},
		{"oversized unset bitarray", false, func(s *signing.MultiSignatureData) *signing.MultiSignatureData {
			s.BitArray = cryptotypes.NewCompactBitArray(16)
			return s
		}},
		{"undersized bitarray", false, func(s *signing.MultiSignatureData) *signing.MultiSignatureData {
			s.BitArray = cryptotypes.NewCompactBitArray(2)
			s.BitArray.SetIndex(0, true)
			s.Signatures = s.Signatures[:1]
			return s
		}},
		{"invalid extra bits", false, func(s *signing.MultiSignatureData) *signing.MultiSignatureData {
			s.BitArray.ExtraBitsStored = 9
			return s
		}},
		{"missing bit storage", false, func(s *signing.MultiSignatureData) *signing.MultiSignatureData { s.BitArray.Elems = nil; return s }},
		{"empty bitarray", false, func(s *signing.MultiSignatureData) *signing.MultiSignatureData {
			s.BitArray = &cryptotypes.CompactBitArray{}
			return s
		}},
		{"nonzero padding", false, func(s *signing.MultiSignatureData) *signing.MultiSignatureData { s.BitArray.Elems[0] |= 1; return s }},
		{"nested short signatures", true, func(s *signing.MultiSignatureData) *signing.MultiSignatureData {
			inner := s.Signatures[0].(*signing.MultiSignatureData)
			inner.Signatures = inner.Signatures[:1]
			return s
		}},
		{"nested oversized bitarray", true, func(s *signing.MultiSignatureData) *signing.MultiSignatureData {
			inner := s.Signatures[0].(*signing.MultiSignatureData)
			inner.BitArray = cryptotypes.NewCompactBitArray(4)
			inner.BitArray.SetIndex(0, true)
			inner.BitArray.SetIndex(3, true)
			return s
		}},
		{"nested nil multisignature", true, func(s *signing.MultiSignatureData) *signing.MultiSignatureData {
			s.Signatures[0] = (*signing.MultiSignatureData)(nil)
			return s
		}},
		{"nested nil bitarray", true, func(s *signing.MultiSignatureData) *signing.MultiSignatureData {
			s.Signatures[0].(*signing.MultiSignatureData).BitArray = nil
			return s
		}},
		{"nested wrong data type", true, func(s *signing.MultiSignatureData) *signing.MultiSignatureData {
			s.Signatures[0] = s.Signatures[1]
			return s
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			key, data := repairM655Multisig(t, tc.nested, 3)
			data = tc.mutate(data)
			var firstError string
			for attempt := 0; attempt < 2; attempt++ {
				for _, direct := range []bool{true, false} {
					meter := sdk.NewInfiniteGasMeter(1, 1)
					var err error
					require.NotPanics(t, func() {
						if direct {
							err = ante.ConsumeMultisignatureVerificationGas(meter, data, key, params, 7)
						} else {
							err = ante.DefaultSigVerificationGasConsumer(meter, signing.SignatureV2{PubKey: key, Data: data, Sequence: 7}, params)
						}
					})
					require.ErrorIs(t, err, sdkerrors.ErrInvalidRequest)
					require.Zero(t, meter.GasConsumed())
					if firstError == "" {
						firstError = err.Error()
					}
					require.EqualError(t, err, firstError)
				}
			}
		})
	}
}

func TestRepairM_655_MultisigGasDecorator(t *testing.T) {
	ctx, ak, _, _, config := repairM655Keepers(t)
	for _, nested := range []bool{false, true} {
		key, data := repairM655Multisig(t, nested, 3)
		addr := sdk.AccAddress(key.Address())
		account := ak.NewAccountWithAddress(ctx, addr)
		require.NoError(t, account.SetPubKey(key))
		ak.SetAccount(ctx, account)
		builder := config.NewTxBuilder()
		require.NoError(t, builder.SetMsgs(&banktypes.MsgSend{FromAddress: addr.String(), ToAddress: addr.String(), Amount: sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 1))}))
		for _, malformed := range []bool{false, true} {
			if malformed {
				if nested {
					data.Signatures[0].(*signing.MultiSignatureData).Signatures = nil
				} else {
					data.Signatures = data.Signatures[:1]
				}
			}
			require.NoError(t, builder.SetSignatures(signing.SignatureV2{PubKey: key, Data: data}))
			wire, err := config.TxEncoder()(builder.GetTx())
			require.NoError(t, err)
			decoded, err := config.TxDecoder()(wire)
			require.NoError(t, err)
			meter := sdk.NewInfiniteGasMeter(1, 1)
			nextCalled := false
			var gasAtNext uint64
			decorator := ante.NewSigGasConsumeDecorator(ak, nil)
			require.NotPanics(t, func() {
				_, err = decorator.AnteHandle(ctx.WithGasMeter(meter), decoded, false, func(nextCtx sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) {
					nextCalled = true
					gasAtNext = nextCtx.GasMeter().GasConsumed()
					return nextCtx, nil
				})
			})
			if malformed {
				require.ErrorIs(t, err, sdkerrors.ErrInvalidRequest)
				require.False(t, nextCalled)
			} else {
				require.NoError(t, err)
				require.True(t, nextCalled)
				// Store reads also consume gas; isolate signature gas with the consumer below.
				require.GreaterOrEqual(t, gasAtNext, uint64(2)*authtypes.DefaultParams().SigVerifyCostSecp256k1)
			}
		}
	}
}

func TestRepairM_655_ValidMultisigGas(t *testing.T) {
	params := authtypes.DefaultParams()
	for _, count := range []int{3, 8, 9} {
		for _, nested := range []bool{false, true} {
			key, data := repairM655Multisig(t, nested, count)
			meter := sdk.NewInfiniteGasMeter(1, 1)
			require.NoError(t, ante.DefaultSigVerificationGasConsumer(meter, signing.SignatureV2{PubKey: key, Data: data}, params))
			leaves := uint64(2)
			if nested {
				leaves = 3
			}
			require.Equal(t, leaves*params.SigVerifyCostSecp256k1, meter.GasConsumed())
			meter = sdk.NewInfiniteGasMeter(1, 1)
			require.NoError(t, ante.ConsumeMultisignatureVerificationGas(meter, multisig.NewMultisig(count), key, params, 0))
			require.Zero(t, meter.GasConsumed())
		}
	}
}

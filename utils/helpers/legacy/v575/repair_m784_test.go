package v575_test

import (
	"bytes"
	"math/big"
	"strconv"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	transferkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/transfer/keeper"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	cryptocodec "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/utils/helpers"
	legacy "github.com/Sidiora-Labs/Paxeer-X-Network/utils/helpers/legacy/v575"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

type repair784Fixture struct {
	ctx  sdk.Context
	auth authkeeper.AccountKeeper
	bank bankkeeper.BaseKeeper
	evm  *evmkeeper.Keeper
	keys []*sdk.KVStoreKey
}

func repair784Keepers(t *testing.T) repair784Fixture {
	t.Helper()
	db := dbm.NewMemDB()
	cms := store.NewCommitMultiStore(db)
	authKey := sdk.NewKVStoreKey(authtypes.StoreKey)
	bankKey := sdk.NewKVStoreKey(banktypes.StoreKey)
	evmKey := sdk.NewKVStoreKey(evmtypes.StoreKey)
	paramsKey := sdk.NewKVStoreKey(paramtypes.StoreKey)
	paramsTransient := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	evmTransient := sdk.NewTransientStoreKey(evmtypes.TransientStoreKey)
	keys := []*sdk.KVStoreKey{authKey, bankKey, evmKey, paramsKey}
	for _, key := range keys {
		cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	}
	cms.MountStoreWithDB(paramsTransient, sdk.StoreTypeTransient, nil)
	cms.MountStoreWithDB(evmTransient, sdk.StoreTypeTransient, nil)
	require.NoError(t, cms.LoadLatestVersion())
	t.Cleanup(func() { require.NoError(t, cms.Close()) })
	registry := codectypes.NewInterfaceRegistry()
	authtypes.RegisterInterfaces(registry)
	cryptocodec.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	amino := codec.NewLegacyAmino()
	auth := authkeeper.NewAccountKeeper(cdc, authKey,
		paramtypes.NewSubspace(cdc, amino, paramsKey, paramsTransient, authtypes.ModuleName),
		authtypes.ProtoBaseAccount, map[string][]string{"repair784": {authtypes.Minter}})
	bank := bankkeeper.NewBaseKeeper(cdc, bankKey, auth,
		paramtypes.NewSubspace(cdc, amino, paramsKey, paramsTransient, banktypes.ModuleName), map[string]bool{})
	evm := evmkeeper.NewKeeper(evmKey, evmTransient,
		paramtypes.NewSubspace(cdc, amino, paramsKey, paramsTransient, evmtypes.ModuleName),
		nil, bank, &auth, nil, transferkeeper.Keeper{}, nil, nil, nil)
	ctx := sdk.NewContext(cms, tmproto.Header{Height: 1, Time: time.Unix(1700000000, 0)}, false)
	auth.SetParams(ctx, authtypes.DefaultParams())
	bank.SetParams(ctx, banktypes.DefaultParams())
	return repair784Fixture{ctx: ctx, auth: auth, bank: bank, evm: evm, keys: keys}
}

// Recover an identity from a real signed transaction after its wire round trip.
func repair784Identity(t *testing.T, ctx sdk.Context) (common.Address, sdk.AccAddress, cryptotypes.PubKey) {
	t.Helper()
	key, err := crypto.HexToECDSA("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	require.NoError(t, err)
	to := crypto.PubkeyToAddress(key.PublicKey)
	signer := ethtypes.HomesteadSigner{}
	tx, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.LegacyTx{
		Nonce: 7, To: &to, Gas: 21000, GasPrice: big.NewInt(1), Value: big.NewInt(0),
	}), signer, key)
	require.NoError(t, err)
	wire, err := tx.MarshalBinary()
	require.NoError(t, err)
	var decoded ethtypes.Transaction
	require.NoError(t, decoded.UnmarshalBinary(wire))
	v, r, s := decoded.RawSignatureValues()
	evm, pax, pub, err := legacy.GetAddressesForContext(ctx, v, r, s, signer.Hash(&decoded))
	require.NoError(t, err)
	require.Equal(t, to, evm)
	expected := &secp256k1.PubKey{Key: crypto.CompressPubkey(&key.PublicKey)}
	require.Equal(t, expected.Bytes(), pub.Bytes())
	require.Equal(t, sdk.AccAddress(expected.Address()), pax)
	return evm, pax, pub
}

func repair784Fund(t *testing.T, f repair784Fixture, address sdk.AccAddress) sdk.Coins {
	t.Helper()
	coins := sdk.NewCoins(sdk.NewInt64Coin("uhpx", 100), sdk.NewInt64Coin("replaytoken", 23))
	require.NoError(t, f.bank.MintCoins(f.ctx, "repair784", coins))
	require.NoError(t, f.bank.SendCoinsFromModuleToAccount(f.ctx, "repair784", address, coins))
	require.NoError(t, f.bank.AddWei(f.ctx, address, sdk.NewInt(19)))
	return coins
}

func repair784Snapshot(t *testing.T, f repair784Fixture) map[string]map[string][]byte {
	t.Helper()
	out := map[string]map[string][]byte{}
	for _, key := range f.keys {
		values := map[string][]byte{}
		iterator := f.ctx.KVStore(key).Iterator(nil, nil)
		for ; iterator.Valid(); iterator.Next() {
			values[string(iterator.Key())] = bytes.Clone(iterator.Value())
		}
		require.NoError(t, iterator.Close())
		out[key.Name()] = values
	}
	return out
}

func TestRepairM_784_KeyRoutingAndReplay(t *testing.T) {
	for _, tc := range []struct {
		name       string
		upgrade    string
		tracing    bool
		historical bool
	}{
		{"unselected", "", false, false},
		{"missing upgrade", "", true, false},
		{"invalid upgrade", "unknown", true, false},
		{"before range", "v5.6.2", true, false},
		{"range start", "v5.7.5", true, true},
		{"inside range", "v5.8.0", true, true},
		{"last patch", "v5.99.99", true, true},
		{"range end", "v6.0.0", true, false},
		{"current replay", "v6.12.0", true, false},
		{"current execution at old height", "v5.7.5", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := sdk.Context{}.WithIsTracing(tc.tracing).WithClosestUpgradeName(tc.upgrade)
			require.Equal(t, tc.historical, legacy.IsHistoricalReplay(ctx))
			repair784Identity(t, ctx)
			if tc.historical {
				// The old unchecked parse is a deliberate replay result, confined to this range.
				require.Panics(t, func() { _, _, _, _ = legacy.GetAddressesFromPubkeyBytes(ctx, []byte{4}) })
			} else {
				require.NotPanics(t, func() {
					evm, pax, pub, err := legacy.GetAddressesFromPubkeyBytes(ctx, []byte{4})
					require.ErrorIs(t, err, helpers.ErrInvalidPubkey)
					require.Equal(t, common.Address{}, evm)
					require.Empty(t, pax)
					require.Nil(t, pub)
				})
			}
		})
	}
}

func TestRepairM_784_InvalidNewKeys(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	valid := crypto.FromECDSAPub(&key.PublicKey)
	for _, input := range [][]byte{nil, {}, {4}, {1, 2, 3}, append([]byte{4}, make([]byte, 64)...),
		valid[:64], append(bytes.Clone(valid), 0), append([]byte{4}, bytes.Repeat([]byte{255}, 64)...)} {
		require.NotPanics(t, func() {
			evm, err := legacy.PubkeyToEVMAddress(input)
			require.ErrorIs(t, err, helpers.ErrInvalidPubkey)
			require.Equal(t, common.Address{}, evm)
			pub, err := legacy.PubkeyBytesToPaxPubKey(input)
			require.ErrorIs(t, err, helpers.ErrInvalidPubkey)
			require.Empty(t, pub.Key)
			_, pax, recovered, err := legacy.GetAddressesFromPubkeyBytes(sdk.Context{}, input)
			require.ErrorIs(t, err, helpers.ErrInvalidPubkey)
			require.Empty(t, pax)
			require.Nil(t, recovered)
		})
	}
	for _, input := range [][]byte{valid, crypto.CompressPubkey(&key.PublicKey)} {
		pub, err := legacy.PubkeyBytesToPaxPubKey(input)
		require.NoError(t, err)
		require.Equal(t, crypto.CompressPubkey(&key.PublicKey), pub.Bytes())
	}
	for _, values := range [][3]*big.Int{
		{nil, big.NewInt(1), big.NewInt(1)}, {big.NewInt(27), nil, big.NewInt(1)},
		{big.NewInt(27), big.NewInt(1), nil}, {big.NewInt(-27), big.NewInt(1), big.NewInt(1)},
		{big.NewInt(283), big.NewInt(1), big.NewInt(1)}, {big.NewInt(27), big.NewInt(0), big.NewInt(1)},
		{big.NewInt(29), big.NewInt(1), big.NewInt(1)},
	} {
		require.NotPanics(t, func() {
			_, pax, pub, err := legacy.GetAddresses(values[0], values[1], values[2], common.Hash{})
			require.ErrorIs(t, err, ethtypes.ErrInvalidSig)
			require.Empty(t, pax)
			require.Nil(t, pub)
		})
	}
}

func TestRepairM_784_AssociationCurrentRouting(t *testing.T) {
	for _, upgrade := range []string{"", "unknown", "v5.6.2", "v5.7.5", "v6.0.0", "v6.12.0"} {
		for _, tracing := range []bool{false, true} {
			ctx := sdk.Context{}.WithClosestUpgradeName(upgrade).WithIsTracing(tracing)
			if legacy.IsHistoricalReplay(ctx) {
				continue // The historical range has its own explicit replay test below.
			}
			t.Run(upgrade+"/tracing="+strconv.FormatBool(tracing), func(t *testing.T) {
				for _, identical := range []bool{false, true} {
					routed, direct := repair784Keepers(t), repair784Keepers(t)
					routed.ctx = routed.ctx.WithClosestUpgradeName(upgrade).WithIsTracing(tracing)
					direct.ctx = direct.ctx.WithClosestUpgradeName(upgrade).WithIsTracing(tracing)
					evm, pax, pub := repair784Identity(t, routed.ctx)
					cast := sdk.AccAddress(evm.Bytes())
					if identical {
						pax = cast
					}
					for _, f := range []repair784Fixture{routed, direct} {
						f.auth.SetAccount(f.ctx, authtypes.NewBaseAccount(cast, nil, 42, 7))
						repair784Fund(t, f, cast)
					}
					if !identical {
						require.Nil(t, routed.auth.GetAccount(routed.ctx, pax))
					}
					require.NoError(t, legacy.NewAssociationHelper(routed.evm, routed.bank, routed.auth).
						AssociateAddresses(routed.ctx, pax, evm, pub))
					require.NoError(t, helpers.NewAssociationHelper(direct.evm, direct.bank, direct.auth).
						AssociateAddresses(direct.ctx, pax, evm, pub, false))
					require.Equal(t, repair784Snapshot(t, direct), repair784Snapshot(t, routed))
					require.Equal(t, direct.ctx.EventManager().Events(), routed.ctx.EventManager().Events())
					acc := routed.auth.GetAccount(routed.ctx, pax)
					require.NotNil(t, acc)
					require.Equal(t, uint64(42), acc.GetAccountNumber())
					require.Equal(t, uint64(7), acc.GetSequence())
					require.Equal(t, pub.Bytes(), acc.GetPubKey().Bytes())
					require.Equal(t, sdk.NewInt(19), routed.bank.GetWeiBalance(routed.ctx, pax))
					require.Equal(t, sdk.NewInt(100), routed.bank.GetBalance(routed.ctx, pax, "uhpx").Amount)
					require.Equal(t, sdk.NewInt(23), routed.bank.GetBalance(routed.ctx, pax, "replaytoken").Amount)
					if !identical {
						require.Nil(t, routed.auth.GetAccount(routed.ctx, cast))
					}
					mapped, found := routed.evm.GetPaxAddress(routed.ctx, evm)
					require.True(t, found)
					require.Equal(t, pax, mapped)
					mappedEVM, found := routed.evm.GetEVMAddress(routed.ctx, pax)
					require.True(t, found)
					require.Equal(t, evm, mappedEVM)
					// Direct calls to the migration entry point must use the same current guard.
					before := repair784Snapshot(t, routed)
					require.NoError(t, legacy.NewAssociationHelper(routed.evm, routed.bank, routed.auth).
						MigrateBalance(routed.ctx, evm, sdk.AccAddress(evm.Bytes())))
					require.Equal(t, before, repair784Snapshot(t, routed))
				}
			})
		}
	}
}

func TestRepairM_784_AssociationHistoricalReplay(t *testing.T) {
	for _, identical := range []bool{false, true} {
		f := repair784Keepers(t)
		f.ctx = f.ctx.WithIsTracing(true).WithClosestUpgradeName("v5.7.5")
		evm, pax, pub := repair784Identity(t, f.ctx)
		cast := sdk.AccAddress(evm.Bytes())
		if identical {
			pax = cast
		}
		f.auth.SetAccount(f.ctx, authtypes.NewBaseAccount(cast, nil, 42, 7))
		coins := repair784Fund(t, f, cast)
		if !identical {
			require.Nil(t, f.auth.GetAccount(f.ctx, pax))
		}
		require.NoError(t, legacy.NewAssociationHelper(f.evm, f.bank, f.auth).AssociateAddresses(f.ctx, pax, evm, pub))
		require.Equal(t, coins, f.bank.GetAllBalances(f.ctx, pax))
		require.Equal(t, sdk.NewInt(19), f.bank.GetWeiBalance(f.ctx, pax))
		if identical {
			// Historical self-migration removes the account; current execution retains it.
			require.Nil(t, f.auth.GetAccount(f.ctx, pax))
		} else {
			// SetAddressMapping creates a missing target on the real EVM keeper path.
			acc := f.auth.GetAccount(f.ctx, pax)
			require.NotNil(t, acc)
			require.NotEqual(t, uint64(42), acc.GetAccountNumber())
			require.Zero(t, acc.GetSequence())
			require.Equal(t, pub.Bytes(), acc.GetPubKey().Bytes())
			require.Nil(t, f.auth.GetAccount(f.ctx, cast))
			require.True(t, f.bank.GetAllBalances(f.ctx, cast).IsZero())
			require.True(t, f.bank.GetWeiBalance(f.ctx, cast).IsZero())
		}
		mapped, found := f.evm.GetPaxAddress(f.ctx, evm)
		require.True(t, found)
		require.Equal(t, pax, mapped)
	}
}

func TestRepairM_784_AssociationRejectsNewInvalidKey(t *testing.T) {
	for _, pub := range []cryptotypes.PubKey{nil, (*secp256k1.PubKey)(nil), &secp256k1.PubKey{}, &secp256k1.PubKey{Key: []byte{4}}} {
		for _, routed := range []bool{false, true} {
			f := repair784Keepers(t)
			evm, pax, _ := repair784Identity(t, f.ctx)
			before := repair784Snapshot(t, f)
			events := f.ctx.EventManager().Events()
			require.NotPanics(t, func() {
				var err error
				if routed {
					err = legacy.NewAssociationHelper(f.evm, f.bank, f.auth).AssociateAddresses(f.ctx, pax, evm, pub)
				} else {
					err = helpers.NewAssociationHelper(f.evm, f.bank, f.auth).AssociateAddresses(f.ctx, pax, evm, pub, false)
				}
				require.ErrorIs(t, err, helpers.ErrInvalidPubkey)
			})
			require.Equal(t, before, repair784Snapshot(t, f))
			require.Equal(t, events, f.ctx.EventManager().Events())
		}
	}
}

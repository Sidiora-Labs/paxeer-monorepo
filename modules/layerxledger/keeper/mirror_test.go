package keeper_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	ledgerkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/stretchr/testify/require"
)

type mirrorFixture struct {
	*harness
	bank     bankkeeper.BaseKeeper
	actor    ledgerViewActor
	asset    [32]byte
	issuance [32]byte
	global   uint64
}

func newMirrorFixture(t *testing.T, blocked map[string]bool) *mirrorFixture {
	t.Helper()
	h := newHarness(t)
	app := testkeeper.EVMTestApp
	auth := authkeeper.NewAccountKeeper(app.AppCodec(), app.GetKey(authtypes.StoreKey),
		app.GetSubspace(authtypes.ModuleName), authtypes.ProtoBaseAccount,
		map[string][]string{types.ModuleName: {authtypes.Minter}})
	bank := bankkeeper.NewBaseKeeper(app.AppCodec(), app.GetKey(banktypes.StoreKey), auth,
		app.GetSubspace(banktypes.ModuleName), blocked)
	h.keeper = ledgerkeeper.NewKeeper(app.AppCodec(), h.storeKey, h.host, bank, &app.EvmKeeper, h.authority)
	f := &mirrorFixture{harness: h, bank: bank, actor: newLedgerViewActor(t, 101)}
	salt := bytes.Repeat([]byte{0x41}, 32)
	material := append(append([]byte("LX:ASSET:v1"), f.actor.id[:]...), salt...)
	f.asset = sha256.Sum256(material)
	var err error
	_, f.issuance, err = lxphost.IssuanceAccountID(f.asset)
	require.NoError(t, err)
	return f
}

func (f *mirrorFixture) execute(t *testing.T, actor ledgerViewActor, activityType uint32, sequence uint64, payload []byte) lxphost.Receipt {
	t.Helper()
	f.global++
	now := uint64(f.ctx.BlockTime().UnixMilli())
	receipt, err := f.host.Execute(lxphost.WithSequence(f.ctx, f.global), lxphost.Activity{
		ProtocolVersion: 3, Type: activityType, ActorDID: actor.did, AuthorityKey: actor.key,
		AccountSequence: sequence, NotBefore: now, NotAfter: now,
		IdempotencyKey: sha256.Sum256(binary.BigEndian.AppendUint64(nil, f.global)), Payload: payload,
	}, lxphost.KernelAuthority{Bound: true, Actor: actor.id, Principal: actor.id, VerifiedKey: actor.key})
	require.NoError(t, err)
	require.Equal(t, lxphost.Code(0), receipt.ResultCode)
	return receipt
}

func (f *mirrorFixture) register(t *testing.T, cap sdk.Int) lxphost.Receipt {
	t.Helper()
	payload, err := types.EncodeRegisterPayload(types.RegisterArgs{
		AssetID: f.asset, Salt: [32]byte(bytes.Repeat([]byte{0x41}, 32)), Symbol: "TKN", Name: "Token",
		Decimals: 6, SupplyCap: cap, IssuerKind: 1,
	})
	require.NoError(t, err)
	return f.execute(t, f.actor, 0x00010001, 0, payload)
}

func TestMirrorRegisterMintsSupply(t *testing.T) {
	maximum := sdk.NewIntFromBigInt(new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1)))
	for _, tc := range []struct {
		name      string
		cap, want sdk.Int
	}{
		{"bounded", sdk.NewInt(1000), sdk.NewInt(1000)},
		{"unbounded", sdk.ZeroInt(), maximum},
		{"maximum", maximum, maximum},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newMirrorFixture(t, nil)
			receipt := f.register(t, tc.cap)
			require.NoError(t, f.keeper.Mirror(f.ctx, 0x00010001, receipt))
			denom := types.MirrorDenom(f.asset)
			require.Equal(t, tc.want, f.bank.GetSupply(f.ctx, denom).Amount)
			require.Equal(t, tc.want, f.bank.GetBalance(f.ctx, lxphost.BankAddress(f.issuance), denom).Amount)
			require.True(t, f.bank.GetBalance(f.ctx, authtypes.NewModuleAddress(types.ModuleName), denom).IsZero())
			require.Equal(t, denom, ledgerkeeper.MirrorDenom(f.asset))
			require.Equal(t, lxphost.BankAddress(f.issuance), ledgerkeeper.BankAddress(f.issuance))
			_, issuance, err := ledgerkeeper.IssuanceAccountID(f.asset)
			require.NoError(t, err)
			require.Equal(t, f.issuance, issuance)
		})
	}
	t.Run("bank_errors", func(t *testing.T) {
		blocked := map[string]bool{}
		f := newMirrorFixture(t, blocked)
		receipt := f.register(t, sdk.NewInt(1000))
		blocked[lxphost.BankAddress(f.issuance).String()] = true
		ctx, _ := f.ctx.CacheContext()
		require.ErrorIs(t, f.keeper.Mirror(ctx, 0x00010001, receipt), types.ErrFatalSupplyMismatch)
		require.Equal(t, sdk.NewInt(1000), f.bank.GetBalance(ctx, authtypes.NewModuleAddress(types.ModuleName), types.MirrorDenom(f.asset)).Amount)
		require.True(t, f.bank.GetSupply(f.ctx, types.MirrorDenom(f.asset)).IsZero())
	})
}

func TestMirrorTransferLegs(t *testing.T) {
	f := newMirrorFixture(t, nil)
	registered := f.register(t, sdk.NewInt(1000))
	require.NoError(t, f.keeper.Mirror(f.ctx, 0x00010001, registered))
	_, from, err := lxphost.AssetAccountID(f.actor.key, f.asset)
	require.NoError(t, err)
	recipient := newLedgerViewActor(t, 102)
	_, to, err := lxphost.AssetAccountID(recipient.key, f.asset)
	require.NoError(t, err)
	f.execute(t, f.actor, 0x00010004, 1, types.EncodeOpenAccountPayload(f.asset))
	f.execute(t, recipient, 0x00010004, 0, types.EncodeOpenAccountPayload(f.asset))
	denom := types.MirrorDenom(f.asset)
	assertBalance := func(account [32]byte, want int64) {
		t.Helper()
		balance, err := f.host.ReadBalance(f.ctx, account, f.asset)
		require.NoError(t, err)
		require.Equal(t, sdk.NewInt(want), balance)
		require.Equal(t, balance, f.bank.GetBalance(f.ctx, lxphost.BankAddress(account), denom).Amount)
	}
	mint, err := types.EncodeSupplyPayload(f.asset, from, sdk.NewInt(25))
	require.NoError(t, err)
	minted := f.execute(t, f.actor, 0x0001000a, 2, mint)
	require.Len(t, minted.Transfers, 1)
	require.NoError(t, f.keeper.Mirror(f.ctx, 0x0001000a, minted))
	assertBalance(from, 25)
	assertBalance(f.issuance, 975)

	key := sha256.Sum256(binary.BigEndian.AppendUint64(nil, f.global+1))
	material := append(append(append([]byte(nil), from[:]...), to[:]...), f.asset[:]...)
	material = binary.BigEndian.AppendUint64(material, 0)
	material = binary.BigEndian.AppendUint64(material, 5)
	contextHash, err := codec.DomainHash(codec.DomainContextHash, append(material, key[:]...))
	require.NoError(t, err)
	send, err := types.EncodeSendPayload(types.SendArgs{
		From: from, To: to, Asset: f.asset, Amount: sdk.NewInt(5), IdempotencyKey: key,
		ExpiresAt: uint64(f.ctx.BlockTime().UnixMilli()) + 1000, ContextHash: contextHash,
		PublicKey: f.actor.key, NetworkID: 125, ProtocolVersion: 3,
	})
	require.NoError(t, err)
	authorization := len(send) - 167
	message := append(append([]byte(nil), send[:2]...), send[4:authorization]...)
	message = append(message, send[authorization:authorization+33]...)
	message = append(message, send[authorization+129:]...)
	preimage, err := codec.DomainHash(codec.DomainSignaturePreimage, message)
	require.NoError(t, err)
	copy(send[authorization+65:authorization+129], ed25519.Sign(f.actor.private, preimage[:]))
	sent := f.execute(t, f.actor, 0x00010005, 3, send)
	require.Len(t, sent.Transfers, 1)
	require.NoError(t, f.keeper.Mirror(f.ctx, 0x00010005, sent))
	assertBalance(from, 20)
	assertBalance(to, 5)

	burn, err := types.EncodeSupplyPayload(f.asset, from, sdk.NewInt(3))
	require.NoError(t, err)
	burned := f.execute(t, f.actor, 0x0001000b, 4, burn)
	require.Len(t, burned.Transfers, 1)
	require.NoError(t, f.keeper.Mirror(f.ctx, 0x0001000b, burned))
	assertBalance(from, 17)
	assertBalance(f.issuance, 978)
	require.Equal(t, sdk.NewInt(1000), f.bank.GetSupply(f.ctx, denom).Amount)

	t.Run("multiple_legs_receive", func(t *testing.T) {
		ctx, _ := f.ctx.CacheContext()
		receipt := lxphost.Receipt{Transfers: []lxphost.TransferLeg{
			{Asset: f.asset, From: from, To: to, Amount: sdk.NewInt(5)},
			{Asset: f.asset, From: to, To: f.issuance, Amount: sdk.NewInt(2)},
		}}
		require.NoError(t, f.keeper.Mirror(ctx, 0x00010006, receipt))
		require.Equal(t, sdk.NewInt(12), f.bank.GetBalance(ctx, lxphost.BankAddress(from), denom).Amount)
		require.Equal(t, sdk.NewInt(8), f.bank.GetBalance(ctx, lxphost.BankAddress(to), denom).Amount)
		require.Equal(t, sdk.NewInt(980), f.bank.GetBalance(ctx, lxphost.BankAddress(f.issuance), denom).Amount)
	})
	t.Run("bank_failure", func(t *testing.T) {
		for _, activityType := range []uint32{0x00010005, 0x00010006, 0x0001000a, 0x0001000b} {
			ctx, _ := f.ctx.CacheContext()
			receipt := lxphost.Receipt{Transfers: []lxphost.TransferLeg{
				{Asset: f.asset, From: from, To: to, Amount: sdk.NewInt(18)},
			}}
			require.ErrorIs(t, f.keeper.Mirror(ctx, activityType, receipt), types.ErrFatalSupplyMismatch)
		}
		assertBalance(from, 17)
		assertBalance(to, 5)
	})
}

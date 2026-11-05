package keeper_test

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	ledgerkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

type sendFixture struct {
	*mirrorFixture
	actor     ledgerkeeper.Actor
	recipient ledgerkeeper.Actor
	from      [32]byte
	to        [32]byte
}

func newSendFixture(t *testing.T) *sendFixture {
	t.Helper()
	return newFundedSendFixture(t, 250)
}

func newFundedSendFixture(t *testing.T, funding int64) *sendFixture {
	t.Helper()
	f := &sendFixture{mirrorFixture: newMirrorFixture(t, nil)}
	f.ctx = f.ctx.WithChainID("hyperpax_125-1")
	bind := func(identity ledgerViewActor, caller common.Address) ledgerkeeper.Actor {
		t.Helper()
		app := testkeeper.EVMTestApp
		message, err := evmtypes.LayerXBindMessage(app.EvmKeeper.ChainID(f.ctx), caller, 0)
		require.NoError(t, err)
		var signature [ed25519.SignatureSize]byte
		copy(signature[:], ed25519.Sign(identity.private, message))
		_, err = app.EvmKeeper.BindLayerX(f.ctx, caller, identity.key, signature)
		require.NoError(t, err)
		actor, err := f.keeper.Actor(f.ctx, caller)
		require.NoError(t, err)
		return actor
	}
	f.actor = bind(f.mirrorFixture.actor, common.Address{0x24, 0x44, 1})
	f.recipient = bind(newLedgerViewActor(t, 103), common.Address{0x24, 0x44, 2})
	var err error
	f.from, err = ledgerkeeper.AccountID(f.actor.DIDKey, f.asset)
	require.NoError(t, err)
	f.to, err = ledgerkeeper.AccountID(f.recipient.DIDKey, f.asset)
	require.NoError(t, err)
	_, err = f.keeper.RegisterAsset(f.ctx, f.actor, types.RegisterArgs{
		AssetID: f.asset, Salt: [32]byte(bytes.Repeat([]byte{0x41}, 32)), Symbol: "TKN", Name: "Token",
		Decimals: 6, SupplyCap: sdk.NewInt(1000), IssuerKind: types.AssetIssuerNative,
	}, [32]byte{1})
	require.NoError(t, err)
	_, err = f.keeper.OpenAccount(f.ctx, f.actor, f.asset, [32]byte{2})
	require.NoError(t, err)
	_, err = f.keeper.OpenAccount(f.ctx, f.recipient, f.asset, [32]byte{3})
	require.NoError(t, err)
	_, err = f.keeper.Mint(f.ctx, f.actor, f.asset, f.from, sdk.NewInt(funding), [32]byte{4})
	require.NoError(t, err)
	f.global = 4
	return f
}

func (f *sendFixture) envelope(t *testing.T, from, to [32]byte, amount sdk.Int, sequence uint64, key [32]byte, expires uint64) []byte {
	t.Helper()
	material := append(append(append([]byte(nil), from[:]...), to[:]...), f.asset[:]...)
	var encoded [16]byte
	amount.BigInt().FillBytes(encoded[:])
	material = append(material, encoded[:]...)
	contextHash, err := codec.DomainHash(codec.DomainContextHash, append(material, key[:]...))
	require.NoError(t, err)
	payload, err := types.EncodeSendPayload(types.SendArgs{
		From: from, To: to, Asset: f.asset, Amount: amount, Sequence: sequence,
		IdempotencyKey: key, ExpiresAt: expires, ContextHash: contextHash,
		PublicKey: f.actor.DIDKey, NetworkID: 125, ProtocolVersion: 3,
	})
	require.NoError(t, err)
	require.NoError(t, types.ValidateSendEnvelope(payload))
	return payload
}

func (f *sendFixture) assertBalance(t *testing.T, account [32]byte, want int64) {
	t.Helper()
	balance, err := f.host.ReadBalance(f.ctx, account, f.asset)
	require.NoError(t, err)
	require.Equal(t, sdk.NewInt(want), balance)
	require.Equal(t, balance, f.bank.GetBalance(f.ctx, lxphost.BankAddress(account), types.MirrorDenom(f.asset)).Amount)
}

func TestSendMovesBalance(t *testing.T) {
	f := newSendFixture(t)
	f.assertBalance(t, f.from, 250)
	f.assertBalance(t, f.to, 0)
	f.assertBalance(t, f.issuance, 750)
	key := [32]byte{0x45}
	payload := f.envelope(t, f.from, f.to, sdk.NewInt(40), 0, key, uint64(f.ctx.BlockTime().UnixMilli())+1000)
	result, err := f.keeper.Send(f.ctx, f.actor, payload, key)
	require.NoError(t, err)
	require.Equal(t, lxphost.Code(0), result.Receipt.ResultCode)
	require.Equal(t, f.global+1, result.Receipt.GlobalSequence)
	require.NotEmpty(t, result.Receipt.Encoded)
	require.Empty(t, result.Events)
	require.Equal(t, result.Receipt.Events, result.Events)
	events, err := ledgerkeeper.LedgerEvents(0x00010005, result.Receipt, ledgerkeeper.EventArgs{})
	require.NoError(t, err)
	require.Equal(t, []ledgerkeeper.Event{{Name: "Transfer", Fields: map[string]any{
		"from": f.from, "to": f.to, "assetId": f.asset,
		"amount": sdk.NewInt(40).BigInt(), "kind": uint8(5),
	}}}, events)
	require.Equal(t, []lxphost.TransferLeg{{Asset: f.asset, From: f.from, To: f.to, Amount: sdk.NewInt(40)}}, result.Receipt.Transfers)
	f.assertBalance(t, f.from, 210)
	f.assertBalance(t, f.to, 40)
	f.assertBalance(t, f.issuance, 750)
	total, err := f.host.ReadTotalUnits(f.ctx, f.asset)
	require.NoError(t, err)
	require.Equal(t, sdk.NewInt(250), total)
	require.Equal(t, sdk.NewInt(1000), f.bank.GetSupply(f.ctx, types.MirrorDenom(f.asset)).Amount)
	name, _, err := lxphost.AssetAccountID(f.actor.DIDKey, f.asset)
	require.NoError(t, err)
	account, err := f.host.ReadAccount(f.ctx, name, f.from)
	require.NoError(t, err)
	require.Equal(t, uint64(1), account.NextSequence)
	nextPayload := f.envelope(t, f.from, f.to, sdk.NewInt(40), 1, key, uint64(f.ctx.BlockTime().UnixMilli())+1000)
	hostBefore := ledgerViewStore(t, f.ctx, f.hostKey)
	bankKey := testkeeper.EVMTestApp.GetKey(banktypes.StoreKey)
	bankBefore := ledgerViewStore(t, f.ctx, bankKey)
	eventsBefore := len(f.ctx.EventManager().Events())
	duplicate, err := f.keeper.Send(f.ctx, f.actor, nextPayload, key)
	require.ErrorIs(t, err, types.ErrIdempotentReplay)
	require.Equal(t, ledgerkeeper.Result{}, duplicate)
	require.Equal(t, hostBefore, ledgerViewStore(t, f.ctx, f.hostKey))
	require.Equal(t, bankBefore, ledgerViewStore(t, f.ctx, bankKey))
	require.Len(t, f.ctx.EventManager().Events(), eventsBefore)
	f.assertBalance(t, f.from, 210)
	f.assertBalance(t, f.to, 40)

	nextKey := [32]byte{0x46}
	nextPayload = f.envelope(t, f.from, f.to, sdk.NewInt(40), 1, nextKey, uint64(f.ctx.BlockTime().UnixMilli())+1000)
	next, err := f.keeper.Send(f.ctx, f.actor, nextPayload, nextKey)
	require.NoError(t, err)
	require.Equal(t, lxphost.Code(0), next.Receipt.ResultCode)
	require.Equal(t, f.global+2, next.Receipt.GlobalSequence)
	require.NotEqual(t, result.Receipt.Encoded, next.Receipt.Encoded)
	require.Empty(t, next.Events)
	require.Equal(t, next.Receipt.Events, next.Events)
	require.Equal(t, result.Receipt.Transfers, next.Receipt.Transfers)
	f.assertBalance(t, f.from, 170)
	f.assertBalance(t, f.to, 80)
	account, err = f.host.ReadAccount(f.ctx, name, f.from)
	require.NoError(t, err)
	require.Equal(t, uint64(2), account.NextSequence)
	hostBefore = ledgerViewStore(t, f.ctx, f.hostKey)
	bankBefore = ledgerViewStore(t, f.ctx, bankKey)
	eventsBefore = len(f.ctx.EventManager().Events())
	duplicate, err = f.keeper.Send(f.ctx, f.actor, payload, key)
	require.ErrorIs(t, err, types.ErrIdempotentReplay)
	require.Equal(t, ledgerkeeper.Result{}, duplicate)
	require.Equal(t, hostBefore, ledgerViewStore(t, f.ctx, f.hostKey))
	require.Equal(t, bankBefore, ledgerViewStore(t, f.ctx, bankKey))
	require.Len(t, f.ctx.EventManager().Events(), eventsBefore)
	staleKey := [32]byte{0x47}
	stalePayload := f.envelope(t, f.from, f.to, sdk.NewInt(40), 0, staleKey, uint64(f.ctx.BlockTime().UnixMilli())+1000)
	replayed, err := f.keeper.Send(f.ctx, f.actor, stalePayload, staleKey)
	require.ErrorIs(t, err, types.ErrSequenceMismatch)
	require.Equal(t, lxphost.Code(-305), replayed.Receipt.ResultCode)
	require.NotEmpty(t, replayed.Receipt.Encoded)
	f.assertBalance(t, f.from, 170)
	f.assertBalance(t, f.to, 80)
	f.assertBalance(t, f.issuance, 750)
	total, err = f.host.ReadTotalUnits(f.ctx, f.asset)
	require.NoError(t, err)
	require.Equal(t, sdk.NewInt(250), total)
	require.Equal(t, sdk.NewInt(1000), f.bank.GetSupply(f.ctx, types.MirrorDenom(f.asset)).Amount)
}

func TestSendRefusals(t *testing.T) {
	f := newSendFixture(t)
	now := uint64(f.ctx.BlockTime().UnixMilli())
	for _, tc := range []struct {
		name   string
		mutate func([]byte) []byte
		paused bool
		want   error
	}{
		{"foreign_source", func(p []byte) []byte { copy(p[4:36], f.to[:]); return p }, false, types.ErrUnauthorizedDebit},
		{"asset_subaccount_wrong_asset", func(p []byte) []byte {
			_, holding, err := lxphost.AssetAccountID(f.actor.DIDKey, f.asset)
			require.NoError(t, err)
			copy(p[4:36], holding[:])
			copy(p[68:100], make([]byte, 32))
			return p
		}, false, types.ErrAssetMismatch},
		{"expired", func(p []byte) []byte { binary.BigEndian.PutUint64(p[156:164], now-1); return p }, false, types.ErrExpired},
		{"zero_amount", func(p []byte) []byte { clear(p[100:116]); return p }, false, types.ErrZeroAmount},
		{"malformed", func(p []byte) []byte { return p[:3] }, false, types.ErrMalformedSend},
		{"trailing_bytes", func(p []byte) []byte { return append(p, 0) }, false, types.ErrMalformedSend},
		{"paused_first", func(p []byte) []byte { return nil }, true, ledgerkeeper.ErrPausedScope},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := f.ctx.CacheContext()
			require.NoError(t, f.keeper.SetParams(ctx, types.Params{Paused: tc.paused}))
			before := ledgerViewStore(t, ctx, f.hostKey)
			key := [32]byte{0x46}
			payload := tc.mutate(f.envelope(t, f.from, f.to, sdk.NewInt(40), 0, key, now+1000))
			_, err := f.keeper.Send(ctx, f.actor, payload, key)
			require.ErrorIs(t, err, tc.want)
			require.Equal(t, before, ledgerViewStore(t, ctx, f.hostKey))
			for _, account := range []struct {
				id   [32]byte
				want int64
			}{{f.from, 250}, {f.to, 0}, {f.issuance, 750}} {
				balance, err := f.host.ReadBalance(ctx, account.id, f.asset)
				require.NoError(t, err)
				require.Equal(t, sdk.NewInt(account.want), balance)
				require.Equal(t, balance, f.bank.GetBalance(ctx, lxphost.BankAddress(account.id), types.MirrorDenom(f.asset)).Amount)
			}
		})
	}
	f.assertBalance(t, f.from, 250)
	f.assertBalance(t, f.to, 0)
}

func TestSendNextSequenceKeyReuse(t *testing.T) {
	f := newSendFixture(t)
	key := [32]byte{0x14, 0x23}
	expires := uint64(f.ctx.BlockTime().UnixMilli()) + 1000
	first, err := f.keeper.Send(f.ctx, f.actor, f.envelope(t, f.from, f.to, sdk.NewInt(40), 0, key, expires), key)
	require.NoError(t, err)
	f.assertBalance(t, f.from, 210)
	f.assertBalance(t, f.to, 40)
	hostBefore := ledgerViewStore(t, f.ctx, f.hostKey)
	bankKey := testkeeper.EVMTestApp.GetKey(banktypes.StoreKey)
	bankBefore := ledgerViewStore(t, f.ctx, bankKey)
	eventsBefore := len(f.ctx.EventManager().Events())
	second, err := f.keeper.Send(f.ctx, f.actor, f.envelope(t, f.from, f.to, sdk.NewInt(40), 1, key, expires), key)
	require.ErrorIs(t, err, types.ErrIdempotentReplay)
	require.Equal(t, ledgerkeeper.Result{}, second)
	require.Equal(t, hostBefore, ledgerViewStore(t, f.ctx, f.hostKey))
	require.Equal(t, bankBefore, ledgerViewStore(t, f.ctx, bankKey))
	require.Len(t, f.ctx.EventManager().Events(), eventsBefore)
	f.assertBalance(t, f.from, 210)
	f.assertBalance(t, f.to, 40)
	sequence, err := f.keeper.NextSequenceByAccountID(f.ctx, f.from, f.asset)
	require.NoError(t, err)
	require.Equal(t, uint64(1), sequence)
	nextKey := [32]byte{0x14, 0x24}
	second, err = f.keeper.Send(f.ctx, f.actor, f.envelope(t, f.from, f.to, sdk.NewInt(40), sequence, nextKey, expires), nextKey)
	require.NoError(t, err)
	require.Equal(t, first.Receipt.GlobalSequence+1, second.Receipt.GlobalSequence)
	f.assertBalance(t, f.from, 170)
	f.assertBalance(t, f.to, 80)
	sequence, err = f.keeper.NextSequenceByAccountID(f.ctx, f.from, f.asset)
	require.NoError(t, err)
	require.Equal(t, uint64(2), sequence)
}

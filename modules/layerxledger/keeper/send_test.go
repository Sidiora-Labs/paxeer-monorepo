package keeper_test

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	ledgerkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
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

func sendMainAccount(t *testing.T, h *harness, actor ledgerkeeper.Actor, asset [32]byte) [32]byte {
	t.Helper()
	name, id, err := lxphost.MainAccountID(actor.DIDKey)
	require.NoError(t, err)
	value := binary.BigEndian.AppendUint32(nil, 32)
	value = append(value, id[:]...)
	value = binary.BigEndian.AppendUint32(value, uint32(len(name)))
	value = append(value, name...)
	value = append(value, 1)
	value = append(value, make([]byte, 16)...)
	value = binary.BigEndian.AppendUint32(value, 32)
	value = append(value, asset[:]...)
	value = append(value, 1)
	value = binary.BigEndian.AppendUint64(value, 0)
	value = binary.BigEndian.AppendUint64(value, 0)
	value = append(value, 0, 0)
	value = binary.BigEndian.AppendUint32(value, 32)
	value = append(value, actor.DIDKey[:]...)
	value = append(value, 1)
	h.ctx.KVStore(h.hostKey).Set(append([]byte{0x06}, id[:]...), value)
	account, err := h.host.ReadAccount(h.ctx, name, id)
	require.NoError(t, err)
	require.Equal(t, uint8(1), account.Kind)
	require.Equal(t, asset, account.AssetID)
	require.Equal(t, actor.DIDKey, account.AuthorityKey)
	return id
}

func newSendFixture(t *testing.T) *sendFixture {
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
	f.from = sendMainAccount(t, f.harness, f.actor, f.asset)
	f.to = sendMainAccount(t, f.harness, f.recipient, f.asset)
	registered := f.register(t, sdk.NewInt(1000))
	require.NoError(t, f.keeper.Mirror(f.ctx, 0x00010001, registered))
	f.execute(t, f.mirrorFixture.actor, 0x00010004, 1, types.EncodeOpenAccountPayload(f.asset))
	_, holding, err := lxphost.AssetAccountID(f.actor.DIDKey, f.asset)
	require.NoError(t, err)
	mint, err := types.EncodeSupplyPayload(f.asset, holding, sdk.NewInt(250))
	require.NoError(t, err)
	minted := f.execute(t, f.mirrorFixture.actor, 0x0001000a, 2, mint)
	require.NoError(t, f.keeper.Mirror(f.ctx, 0x0001000a, minted))
	key := sha256.Sum256(binary.BigEndian.AppendUint64(nil, f.global+1))
	payload := f.envelope(t, holding, f.from, sdk.NewInt(250), 0, key,
		uint64(f.ctx.BlockTime().UnixMilli())+1000)
	funded := f.execute(t, f.mirrorFixture.actor, 0x00010005, 3, payload)
	require.NoError(t, f.keeper.Mirror(f.ctx, 0x00010005, funded))
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
	name, _, err := lxphost.MainAccountID(f.actor.DIDKey)
	require.NoError(t, err)
	account, err := f.host.ReadAccount(f.ctx, name, f.from)
	require.NoError(t, err)
	require.Equal(t, uint64(1), account.NextSequence)
	nextPayload := f.envelope(t, f.from, f.to, sdk.NewInt(40), 1, key, uint64(f.ctx.BlockTime().UnixMilli())+1000)
	next, err := f.keeper.Send(f.ctx, f.actor, nextPayload, key)
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
	replayed, err := f.keeper.Send(f.ctx, f.actor, payload, key)
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
		{"asset_subaccount", func(p []byte) []byte {
			_, holding, err := lxphost.AssetAccountID(f.actor.DIDKey, f.asset)
			require.NoError(t, err)
			copy(p[4:36], holding[:])
			return p
		}, false, types.ErrUnauthorizedDebit},
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

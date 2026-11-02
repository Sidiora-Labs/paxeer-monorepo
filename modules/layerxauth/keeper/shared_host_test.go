package keeper_test

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	ledgertypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestSharedHostIdempotencyAcrossModules(t *testing.T) {
	a := app.Setup(t, false, true, false)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	ctx := a.GetContextForDeliverTx(nil).WithChainID("pax-test").WithBlockHeight(100).
		WithBlockTime(time.Unix(1_800_000_000, 0).UTC())
	hostKey := a.GetKey(app.LayerXHostStoreKey)
	now := uint64(ctx.BlockTime().UnixMilli())
	require.NotNil(t, a.LayerXHost)
	require.NotNil(t, hostKey)
	require.Same(t, a.LayerXHost, a.LayerXAuthKeeper.TestHost())

	bind := func(caller common.Address, seedByte byte) (ed25519.PrivateKey, authkeeper.Actor, uint64) {
		t.Helper()
		var seed [ed25519.SeedSize]byte
		seed[0] = seedByte
		private := ed25519.NewKeyFromSeed(seed[:])
		var public [32]byte
		copy(public[:], private.Public().(ed25519.PublicKey))
		message, err := evmtypes.LayerXBindMessage(a.EvmKeeper.ChainID(ctx), caller, 0)
		require.NoError(t, err)
		var signature [ed25519.SignatureSize]byte
		copy(signature[:], ed25519.Sign(private, message))
		_, err = a.EvmKeeper.BindLayerX(ctx, caller, public, signature)
		require.NoError(t, err)
		actor, err := a.LayerXAuthKeeper.Actor(ctx, caller)
		require.NoError(t, err)
		ledgerActor, err := a.LayerXLedgerKeeper.Actor(ctx, caller)
		require.NoError(t, err)
		require.Equal(t, actor.DID, ledgerActor.DID)
		require.Equal(t, actor.Authority, ledgerActor.Authority)
		sequence, err := lxphost.NextSequence(ctx, hostKey)
		require.NoError(t, err)
		payload := append([]byte{0x71, 1, 0, 2}, actor.Authority.Actor[:]...)
		payload = append(payload, public[:]...)
		activity := lxphost.Activity{
			ProtocolVersion: 3, Type: 0x00070001, ActorDID: actor.DID, AuthorityKey: public,
			NotBefore: now, NotAfter: now, IdempotencyKey: [32]byte{0x10, seedByte}, Payload: payload,
		}
		digest, err := a.LayerXHost.SigningHash(activity)
		require.NoError(t, err)
		activity.Signature = ed25519.Sign(private, digest[:])
		receipt, err := a.LayerXHost.Execute(lxphost.WithSequence(ctx, sequence), activity, actor.Authority)
		require.NoError(t, err)
		require.Equal(t, lxphost.Code(0), receipt.ResultCode)
		return private, actor, receipt.GlobalSequence
	}
	payerCaller, otherCaller := common.Address{0x27, 1}, common.Address{0x27, 2}
	private, payer, payerEpoch := bind(payerCaller, 1)
	_, other, otherEpoch := bind(otherCaller, 2)
	require.NotEqual(t, payer.DID, other.DID)
	ledgerActor, err := a.LayerXLedgerKeeper.Actor(ctx, payerCaller)
	require.NoError(t, err)
	otherLedgerActor, err := a.LayerXLedgerKeeper.Actor(ctx, otherCaller)
	require.NoError(t, err)

	salt := [32]byte{0x31}
	material := append([]byte("LX:ASSET:v1"), payer.Authority.Actor[:]...)
	asset := sha256.Sum256(append(material, salt[:]...))
	_, err = a.LayerXLedgerKeeper.RegisterAsset(ctx, ledgerActor, ledgertypes.RegisterArgs{
		AssetID: asset, Salt: salt, Symbol: "SHR", Name: "Shared host", Decimals: 6,
		SupplyCap: sdk.NewInt(1000), IssuerKind: ledgertypes.AssetIssuerNative,
	}, [32]byte{0x32})
	require.NoError(t, err)
	_, err = a.LayerXLedgerKeeper.OpenAccount(ctx, ledgerActor, asset, [32]byte{0x33})
	require.NoError(t, err)
	_, err = a.LayerXLedgerKeeper.OpenAccount(ctx, otherLedgerActor, asset, [32]byte{0x34})
	require.NoError(t, err)
	_, holding, err := lxphost.AssetAccountID(payer.DIDKey, asset)
	require.NoError(t, err)
	_, to, err := lxphost.AssetAccountID(other.DIDKey, asset)
	require.NoError(t, err)
	mainName, from, err := lxphost.MainAccountID(payer.DIDKey)
	require.NoError(t, err)
	row := binary.BigEndian.AppendUint32(nil, 32)
	row = append(row, from[:]...)
	row = binary.BigEndian.AppendUint32(row, uint32(len(mainName)))
	row = append(row, mainName...)
	row = append(row, 1)
	row = append(row, make([]byte, 16)...)
	row = binary.BigEndian.AppendUint32(row, 32)
	row = append(row, asset[:]...)
	row = append(row, 1)
	row = binary.BigEndian.AppendUint64(row, 0)
	row = binary.BigEndian.AppendUint64(row, 0)
	row = append(row, 0, 0)
	row = binary.BigEndian.AppendUint32(row, 32)
	row = append(row, payer.DIDKey[:]...)
	row = append(row, 1)
	ctx.KVStore(hostKey).Set(append([]byte{0x06}, from[:]...), row)
	mainAccount, err := a.LayerXHost.ReadAccount(ctx, mainName, from)
	require.NoError(t, err)
	require.Equal(t, sdk.ZeroInt(), mainAccount.Balance)
	require.Equal(t, payer.DIDKey, mainAccount.AuthorityKey)
	_, err = a.LayerXLedgerKeeper.Mint(ctx, ledgerActor, asset, holding, sdk.NewInt(100), [32]byte{0x35})
	require.NoError(t, err)
	_, err = a.LayerXLedgerKeeper.PayExact(ctx, ledgerActor, ledgertypes.PayExactArgs{
		From: holding, To: from, Asset: asset, Amount: sdk.NewInt(100),
		ExpiresAt: now + 1000, PaymentReference: [32]byte{0x36},
	})
	require.NoError(t, err)

	key := [32]byte{0x41}
	material = append(append(append([]byte(nil), from[:]...), to[:]...), asset[:]...)
	var amount [16]byte
	sdk.NewInt(40).BigInt().FillBytes(amount[:])
	material = append(material, amount[:]...)
	contextHash, err := codec.DomainHash(codec.DomainContextHash, append(material, key[:]...))
	require.NoError(t, err)
	mainAccount, err = a.LayerXHost.ReadAccount(ctx, mainName, from)
	require.NoError(t, err)
	envelope, err := ledgertypes.EncodeSendPayload(ledgertypes.SendArgs{
		From: from, To: to, Asset: asset, Amount: sdk.NewInt(40), Sequence: mainAccount.NextSequence,
		IdempotencyKey: key, ExpiresAt: now + 1000, ContextHash: contextHash, PublicKey: payer.DIDKey,
		NetworkID: uint32(a.EvmKeeper.ChainID(ctx).Uint64()), ProtocolVersion: codec.StateCommitmentProtocolVersion,
	})
	require.NoError(t, err)
	authorization := len(envelope) - 167
	message := append(append([]byte(nil), envelope[:2]...), envelope[4:authorization]...)
	message = append(message, envelope[authorization:authorization+33]...)
	message = append(message, envelope[authorization+129:]...)
	digest, err := codec.DomainHash(codec.DomainSignaturePreimage, message)
	require.NoError(t, err)
	copy(envelope[authorization+65:authorization+129], ed25519.Sign(private, digest[:]))
	sent, err := a.LayerXLedgerKeeper.Send(ctx, ledgerActor, envelope, key)
	require.NoError(t, err)
	require.Equal(t, lxphost.Code(0), sent.Receipt.ResultCode)
	for account, want := range map[[32]byte]int64{from: 60, to: 40} {
		balance, err := a.LayerXHost.ReadBalance(ctx, account, asset)
		require.NoError(t, err)
		require.Equal(t, sdk.NewInt(want), balance)
		require.Equal(t, balance, a.BankKeeper.GetBalance(ctx, lxphost.BankAddress(account), ledgertypes.MirrorDenom(asset)).Amount)
	}

	grantBody := func(actor authkeeper.Actor, epoch uint64) ([]byte, [32]byte) {
		t.Helper()
		body := []byte{0, 1, 0x20, 1, 1}
		field := func(value []byte) {
			body = binary.BigEndian.AppendUint32(body, uint32(len(value)))
			body = append(body, value...)
		}
		field(actor.Authority.Actor[:])
		field(actor.Authority.Actor[:])
		body = append(body, 2)
		delegate := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize)).Public().(ed25519.PublicKey)
		field(delegate)
		body = binary.BigEndian.AppendUint64(body, 1<<9)
		body = binary.BigEndian.AppendUint16(body, 4)
		body = binary.BigEndian.AppendUint16(body, 5)
		field(make([]byte, 32))
		body = append(body, make([]byte, 96)...)
		field(make([]byte, 32))
		body = binary.BigEndian.AppendUint64(body, now-1000)
		body = binary.BigEndian.AppendUint64(body, now+1000)
		body = binary.BigEndian.AppendUint64(body, epoch)
		body = append(body, 0)
		body = binary.BigEndian.AppendUint64(body, 0)
		field(make([]byte, ed25519.SignatureSize))
		id, err := codec.DomainHash(codec.DomainAuthorityHash, body)
		require.NoError(t, err)
		return body, id
	}
	snapshot := func() map[string][]byte {
		t.Helper()
		rows := map[string][]byte{}
		iterator := ctx.KVStore(hostKey).Iterator(nil, nil)
		defer iterator.Close()
		for ; iterator.Valid(); iterator.Next() {
			rows[string(iterator.Key())] = append([]byte(nil), iterator.Value()...)
		}
		return rows
	}
	assertNoIdempotency := func() {
		t.Helper()
		iterator := sdk.KVStorePrefixIterator(ctx.KVStore(hostKey), []byte{0x04})
		defer iterator.Close()
		require.False(t, iterator.Valid(), "shared host must not persist idempotency entries")
	}
	assertNoIdempotency()

	body, grantID := grantBody(payer, payerEpoch)
	consumedSequence, err := a.LayerXHost.ReadIdentitySequence(ctx, payer.DID)
	require.NoError(t, err)
	granted, err := a.LayerXAuthKeeper.GrantSession(ctx, payer, body, 1000, [32]byte{0x51}, key)
	require.NoError(t, err)
	require.Equal(t, lxphost.Code(0), granted.ResultCode)
	require.Equal(t, sent.Receipt.GlobalSequence+1, granted.GlobalSequence)
	require.NotEmpty(t, granted.Encoded)
	stored, err := a.LayerXAuthKeeper.GetGrant(ctx, grantID)
	require.NoError(t, err)
	require.Equal(t, body, stored)
	nextAccountSequence, err := a.LayerXHost.ReadIdentitySequence(ctx, payer.DID)
	require.NoError(t, err)
	require.Equal(t, consumedSequence+1, nextAccountSequence)
	assertNoIdempotency()

	payload, err := authtypes.EncodeSessionPayload(body, 1000, [32]byte{0x51}, nil)
	require.NoError(t, err)
	replay := lxphost.Activity{
		ProtocolVersion: 3, Type: 0x00070005, ActorDID: payer.DID, AuthorityKey: payer.DIDKey,
		AccountSequence: consumedSequence, NotBefore: now, NotAfter: now,
		IdempotencyKey: key, Payload: payload,
	}
	sequence, err := lxphost.NextSequence(ctx, hostKey)
	require.NoError(t, err)
	require.Equal(t, granted.GlobalSequence+1, sequence)
	before := snapshot()
	_, err = a.LayerXHost.Execute(lxphost.WithSequence(ctx, sequence), replay, payer.Authority)
	var code lxphost.Code
	require.ErrorAs(t, err, &code)
	require.ErrorIs(t, authkeeper.FromKernel(code), authtypes.ErrSequenceReused)
	require.Equal(t, before, snapshot())
	assertNoIdempotency()

	otherBody, otherGrantID := grantBody(other, otherEpoch)
	otherGranted, err := a.LayerXAuthKeeper.GrantSession(ctx, other, otherBody, 1000, [32]byte{0x53}, key)
	require.NoError(t, err)
	require.Equal(t, lxphost.Code(0), otherGranted.ResultCode)
	require.Equal(t, sequence, otherGranted.GlobalSequence)
	require.NotEmpty(t, otherGranted.Encoded)
	stored, err = a.LayerXAuthKeeper.GetGrant(ctx, otherGrantID)
	require.NoError(t, err)
	require.Equal(t, otherBody, stored)
	stored, err = a.LayerXAuthKeeper.GetGrant(ctx, grantID)
	require.NoError(t, err)
	require.Equal(t, body, stored)
	next, err := lxphost.NextSequence(ctx, hostKey)
	require.NoError(t, err)
	require.Equal(t, otherGranted.GlobalSequence+1, next)
	for account, want := range map[[32]byte]int64{from: 60, to: 40} {
		balance, err := a.LayerXHost.ReadBalance(ctx, account, asset)
		require.NoError(t, err)
		require.Equal(t, sdk.NewInt(want), balance)
		require.Equal(t, balance, a.BankKeeper.GetBalance(ctx, lxphost.BankAddress(account), ledgertypes.MirrorDenom(asset)).Amount)
	}
	assertNoIdempotency()
}

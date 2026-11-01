package keeper_test

import (
	"crypto/ed25519"
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

func bindPayerGrantActor(t *testing.T, f *ledgerViewFixture, signer ledgerViewActor, caller common.Address) ledgerkeeper.Actor {
	t.Helper()
	f.ctx = f.ctx.WithChainID("hyperpax_125-1")
	evm := &testkeeper.EVMTestApp.EvmKeeper
	message, err := evmtypes.LayerXBindMessage(evm.ChainID(f.ctx), caller, 0)
	require.NoError(t, err)
	var signature [ed25519.SignatureSize]byte
	copy(signature[:], ed25519.Sign(signer.private, message))
	_, err = evm.BindLayerX(f.ctx, caller, signer.key, signature)
	require.NoError(t, err)
	actor, err := f.keeper.Actor(f.ctx, caller)
	require.NoError(t, err)
	require.True(t, actor.Authority.Bound)
	require.Equal(t, signer.id, actor.Authority.Actor)
	return actor
}

func drawPayerGrant(t *testing.T, f *ledgerViewFixture, actor ledgerkeeper.Actor, grant []byte, grantID, idempotencyKey [32]byte) (lxphost.Receipt, error) {
	t.Helper()
	sequence, err := f.keeper.NextSequence(f.ctx, actor.DIDKey, f.asset)
	require.NoError(t, err)
	contextHash, err := codec.DomainHash(codec.DomainContextHash, grant[177:209])
	require.NoError(t, err)
	payload, err := types.EncodeReceivePayload(types.ReceiveArgs{
		From: f.account, To: f.to, Asset: f.asset, Amount: sdk.NewInt(10),
		GrantID: grantID, ReceiverSequence: sequence, IdempotencyKey: idempotencyKey,
		ContextHash: contextHash, PublicKey: actor.DIDKey,
		NetworkID: 125, ProtocolVersion: 3, PayerGrant: grant,
	})
	require.NoError(t, err)
	const authorizationOffset = 220
	message := append([]byte("LXP:RECEIVE:v1"), payload[4:authorizationOffset]...)
	message = append(message, payload[authorizationOffset:authorizationOffset+33]...)
	message = append(message, payload[authorizationOffset+129:authorizationOffset+167]...)
	hash, err := codec.DomainHash(codec.DomainSignaturePreimage, message)
	require.NoError(t, err)
	copy(payload[authorizationOffset+65:authorizationOffset+129], ed25519.Sign(f.recipient.private, hash[:]))
	accountSequence, err := f.host.ReadIdentitySequence(f.ctx, actor.DID)
	require.NoError(t, err)
	ctx, err := ledgerkeeper.WithSequence(f.ctx)
	require.NoError(t, err)
	now := uint64(ctx.BlockTime().UnixMilli())
	receipt, err := f.host.Execute(ctx, lxphost.Activity{
		ProtocolVersion: 3, Type: 0x00010006, ActorDID: actor.DID,
		AuthorityKey: actor.Authority.VerifiedKey, AccountSequence: accountSequence,
		NotBefore: now, NotAfter: now, IdempotencyKey: idempotencyKey, Payload: payload,
	}, actor.Authority)
	require.NoError(t, err)
	return receipt, ledgerkeeper.FromKernel(receipt.ResultCode)
}

func TestIssuePayerGrant(t *testing.T) {
	f := newLedgerViewFixture(t)
	actor := bindPayerGrantActor(t, f, f.actor, common.Address{0x24, 0x06, 1})
	grant, grantID := f.payerGrant(t)
	require.Len(t, grant, 346)
	require.NoError(t, types.ValidateIssueGrantPayload(grant))
	badRecurring := append([]byte(nil), grant...)
	badRecurring[160] = 2
	badReference := append([]byte(nil), grant...)
	badReference[209] = 2
	for _, tc := range []struct {
		name    string
		payload []byte
	}{
		{"short", grant[:345]},
		{"long", append(append([]byte(nil), grant...), 0)},
		{"recurring", badRecurring},
		{"reference", badReference},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := ledgerViewStore(t, f.ctx, f.hostKey)
			result, err := f.keeper.IssueGrant(f.ctx, actor, tc.payload, [32]byte{0x61})
			require.ErrorIs(t, err, types.ErrNonCanonical)
			require.Equal(t, ledgerkeeper.Result{}, result)
			require.Equal(t, before, ledgerViewStore(t, f.ctx, f.hostKey))
		})
	}
	require.NoError(t, f.keeper.SetParams(f.ctx, types.Params{Paused: true}))
	before := ledgerViewStore(t, f.ctx, f.hostKey)
	result, err := f.keeper.IssueGrant(f.ctx, actor, nil, [32]byte{0x61})
	require.ErrorIs(t, err, ledgerkeeper.ErrPausedScope)
	require.Equal(t, ledgerkeeper.Result{}, result)
	require.Equal(t, before, ledgerViewStore(t, f.ctx, f.hostKey))
	require.NoError(t, f.keeper.SetParams(f.ctx, types.Params{}))

	sequence, err := lxphost.NextSequence(f.ctx, f.hostKey)
	require.NoError(t, err)
	result, err = f.keeper.IssueGrant(f.ctx, actor, grant, [32]byte{0x61})
	require.NoError(t, err)
	require.Zero(t, result.Receipt.ResultCode)
	require.Equal(t, sequence, result.Receipt.GlobalSequence)
	require.NotEmpty(t, result.Receipt.Encoded)
	require.Contains(t, result.Receipt.Events, lxphost.ReceiptEvent{Code: 7, Body: grantID[:]})
	require.NotEmpty(t, result.Events)
	require.Equal(t, result.Receipt.Events, result.Events)
	events, err := ledgerkeeper.LedgerEvents(0x00010007, result.Receipt, ledgerkeeper.EventArgs{})
	require.NoError(t, err)
	require.Equal(t, []ledgerkeeper.Event{{Name: "PayerGrantIssued", Fields: map[string]any{"grantId": grantID}}}, events)
	state, err := f.host.ReadPayerGrant(f.ctx, grantID)
	require.NoError(t, err)
	require.Equal(t, lxphost.PayerGrantState{
		Grant: grant, DrawnTotal: sdk.ZeroInt(), DrawnThisPeriod: sdk.ZeroInt(),
	}, state)
	accountSequence, err := f.host.ReadIdentitySequence(f.ctx, actor.DID)
	require.NoError(t, err)
	secondGrant := append([]byte(nil), grant...)
	secondGrant[177] ^= 1
	secondID, err := codec.DomainHash(codec.DomainAuthorityHash, append([]byte("LXP:GRANT:v1"), secondGrant[32:282]...))
	require.NoError(t, err)
	copy(secondGrant[:32], secondID[:])
	copy(secondGrant[282:], ed25519.Sign(f.actor.private, secondID[:]))
	second, err := f.keeper.IssueGrant(f.ctx, actor, secondGrant, [32]byte{0x61})
	require.NoError(t, err)
	require.Zero(t, second.Receipt.ResultCode)
	require.Equal(t, sequence+1, second.Receipt.GlobalSequence)
	require.NotEqual(t, result.Receipt.Encoded, second.Receipt.Encoded)
	require.Contains(t, second.Events, lxphost.ReceiptEvent{Code: 7, Body: secondID[:]})
	nextAccountSequence, err := f.host.ReadIdentitySequence(f.ctx, actor.DID)
	require.NoError(t, err)
	require.Equal(t, accountSequence+1, nextAccountSequence)
	secondState, err := f.host.ReadPayerGrant(f.ctx, secondID)
	require.NoError(t, err)
	require.Equal(t, lxphost.PayerGrantState{
		Grant: secondGrant, DrawnTotal: sdk.ZeroInt(), DrawnThisPeriod: sdk.ZeroInt(),
	}, secondState)
	firstState, err := f.host.ReadPayerGrant(f.ctx, grantID)
	require.NoError(t, err)
	require.Equal(t, state, firstState)
}

func TestRevokePayerGrant(t *testing.T) {
	f := newLedgerViewFixture(t)
	payer := bindPayerGrantActor(t, f, f.actor, common.Address{0x24, 0x06, 1})
	payee := bindPayerGrantActor(t, f, f.recipient, common.Address{0x24, 0x06, 2})
	grant, grantID := f.payerGrant(t)
	_, err := f.keeper.IssueGrant(f.ctx, payer, grant, [32]byte{0x71})
	require.NoError(t, err)
	receipt, err := drawPayerGrant(t, f, payee, grant, grantID, [32]byte{0x72})
	require.NoError(t, err)
	require.Zero(t, receipt.ResultCode)
	state, err := f.host.ReadPayerGrant(f.ctx, grantID)
	require.NoError(t, err)
	require.Equal(t, sdk.NewInt(10), state.DrawnTotal)
	require.False(t, state.Revoked)

	payeeSequence, err := f.host.ReadIdentitySequence(f.ctx, payee.DID)
	require.NoError(t, err)
	refused, err := f.keeper.RevokeGrant(f.ctx, payee, grantID, payeeSequence, [32]byte{0x73})
	require.ErrorIs(t, err, types.ErrUnauthorizedDebit)
	require.Equal(t, lxphost.Code(-207), refused.Receipt.ResultCode)
	unchanged, err := f.host.ReadPayerGrant(f.ctx, grantID)
	require.NoError(t, err)
	require.Equal(t, state, unchanged)

	require.NoError(t, f.keeper.SetParams(f.ctx, types.Params{Paused: true}))
	revocationSequence, err := f.host.ReadIdentitySequence(f.ctx, payer.DID)
	require.NoError(t, err)
	sequence, err := lxphost.NextSequence(f.ctx, f.hostKey)
	require.NoError(t, err)
	result, err := f.keeper.RevokeGrant(f.ctx, payer, grantID, revocationSequence, [32]byte{0x74})
	require.NoError(t, err)
	require.Zero(t, result.Receipt.ResultCode)
	require.Equal(t, sequence, result.Receipt.GlobalSequence)
	require.Contains(t, result.Receipt.Events, lxphost.ReceiptEvent{Code: 8, Body: grantID[:]})
	require.NotEmpty(t, result.Events)
	require.Equal(t, result.Receipt.Events, result.Events)
	events, err := ledgerkeeper.LedgerEvents(0x00010008, result.Receipt, ledgerkeeper.EventArgs{})
	require.NoError(t, err)
	require.Equal(t, []ledgerkeeper.Event{{Name: "PayerGrantRevoked", Fields: map[string]any{"grantId": grantID}}}, events)
	state.Revoked = true
	state.RevokedAt = sequence
	revoked, err := f.host.ReadPayerGrant(f.ctx, grantID)
	require.NoError(t, err)
	require.Equal(t, state, revoked)
	require.True(t, f.keeper.GetParams(f.ctx).Paused)
	require.NoError(t, f.keeper.SetParams(f.ctx, types.Params{}))

	payerBalance, err := f.host.ReadBalance(f.ctx, f.account, f.asset)
	require.NoError(t, err)
	payeeBalance, err := f.host.ReadBalance(f.ctx, f.to, f.asset)
	require.NoError(t, err)
	receipt, err = drawPayerGrant(t, f, payee, grant, grantID, [32]byte{0x75})
	require.Equal(t, lxphost.Code(-219), receipt.ResultCode)
	require.ErrorIs(t, err, lxphost.Code(-219))
	require.Equal(t, lxphost.Code(-219), ledgerkeeper.FromKernel(receipt.ResultCode))
	after, readErr := f.host.ReadPayerGrant(f.ctx, grantID)
	require.NoError(t, readErr)
	require.Equal(t, revoked, after)
	afterPayer, readErr := f.host.ReadBalance(f.ctx, f.account, f.asset)
	require.NoError(t, readErr)
	require.Equal(t, payerBalance, afterPayer)
	afterPayee, readErr := f.host.ReadBalance(f.ctx, f.to, f.asset)
	require.NoError(t, readErr)
	require.Equal(t, payeeBalance, afterPayee)
}

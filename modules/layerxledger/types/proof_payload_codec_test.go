// Proves: types.EncodeRegisterPayload, EncodeOpenAccountPayload, EncodeSupplyPayload, EncodeSendPayload, EncodePausePayload, EncodeReceivePayload, EncodeRevokeGrantPayload and ValidateIssueGrantPayload -> exported []byte payloads in lxphost.Activity -> lxphost.Host.Execute and Host.ReadAsset/ReadBalance.
//
// The test is the first consumer in the payload-codec interface inventory. It
// submits real encoder output to the kernel, signs both send and activity bytes,
// and closes the host and committed LevelDB store before reopening them. Repeated
// production is byte-identical; the recovered kernel refuses duplicate assets.
//
// Current behavior outside this task's scope: src/modules/asset/lx_asset_registry.c,
// activity_types/module_decode, refuses ordinals 6, 7 and 8 with
// LXP_ERR_UNKNOWN_ACTIVITY. Thus
// legacy receive, issue-grant and revoke-grant codecs still encode bytes, but
// cannot move funds or create grants. The proof asserts that refusal explicitly.
// No live payer-grant producer remains: a zero-filled structural grant below is
// deliberately unauthenticated input to the shape validator, never seeded state
// or a substitute for a successful kernel grant. ValidateIssueGrantPayload must
// accept its canonical shape and reject malformed boolean/length encodings.
package types_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math/big"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

const proofPayloadTime = int64(1700000000000)

type proofPayloadActor struct {
	private ed25519.PrivateKey
	key     [32]byte
	id      [32]byte
	did     []byte
}

func proofPayloadIdentity(t *testing.T, seed byte) proofPayloadActor {
	t.Helper()
	a := proofPayloadActor{private: ed25519.NewKeyFromSeed(bytes.Repeat([]byte{seed}, 32))}
	copy(a.key[:], a.private.Public().(ed25519.PublicKey))
	a.did = []byte("did:layerx:" + hex.EncodeToString(a.key[:]))
	var err error
	a.id, err = lxphost.DIDID(a.did)
	require.NoError(t, err)
	return a
}

type proofPayloadNode struct {
	host *lxphost.Host
	cms  sdk.CommitMultiStore
	key  *sdk.KVStoreKey
	ctx  sdk.Context
}

func proofPayloadOpen(t *testing.T, directory string) *proofPayloadNode {
	t.Helper()
	db, err := dbm.NewDB("payload-proof", dbm.GoLevelDBBackend, directory)
	require.NoError(t, err, "open real payload proof store")
	key := sdk.NewKVStoreKey("lxphost")
	cms := store.NewCommitMultiStore(db)
	cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	require.NoError(t, cms.LoadLatestVersion())
	host, err := lxphost.NewHost(lxphost.Config{
		StoreKey: key, NetworkID: 125, ParameterVersion: 1,
		TimestampWindowMs: 60000, GasLimit: 10000000,
	})
	require.NoError(t, err, "modules/lxphost/host.go: NewHost must provide the real kernel producer/consumer")
	ctx := sdk.NewContext(cms, tmproto.Header{ChainID: "payload-proof", Height: 1, Time: time.UnixMilli(proofPayloadTime)}, false)
	node := &proofPayloadNode{host: host, cms: cms, key: key, ctx: ctx}
	t.Cleanup(func() {
		if node.host != nil {
			node.close(t)
		}
	})
	return node
}

func (n *proofPayloadNode) close(t *testing.T) {
	t.Helper()
	n.host.Close()
	require.NoError(t, n.cms.Close())
	n.host, n.cms = nil, nil
	n.ctx = sdk.Context{}
}

func (n *proofPayloadNode) execute(t *testing.T, actor proofPayloadActor, activityType uint32, payload []byte, key [32]byte) lxphost.Receipt {
	t.Helper()
	receipt, err := n.submit(t, actor, activityType, payload, key)
	require.NoError(t, err)
	require.NotEmpty(t, receipt.Encoded)
	decoded, err := codec.DecodeReceipt(receipt.Encoded)
	require.NoError(t, err)
	require.Equal(t, int32(receipt.ResultCode), decoded.ResultCode)
	require.Equal(t, receipt.GlobalSequence, decoded.GlobalSequence)
	return receipt
}

func (n *proofPayloadNode) submit(t *testing.T, actor proofPayloadActor, activityType uint32, payload []byte, key [32]byte) (lxphost.Receipt, error) {
	t.Helper()
	global, err := lxphost.NextSequence(n.ctx, n.key)
	require.NoError(t, err)
	sequence, err := n.host.ReadIdentitySequence(n.ctx, actor.did)
	require.NoError(t, err)
	activity := lxphost.Activity{
		ProtocolVersion: 3, Type: activityType, ActorDID: actor.did,
		AuthorityKey: actor.key, AccountSequence: sequence,
		NotBefore: uint64(proofPayloadTime), NotAfter: uint64(proofPayloadTime),
		IdempotencyKey: key, Payload: payload,
	}
	hash, err := n.host.SigningHash(activity)
	require.NoError(t, err)
	activity.Signature = ed25519.Sign(actor.private, hash[:])
	receipt, err := n.host.Execute(lxphost.WithSequence(n.ctx, global), activity,
		lxphost.KernelAuthority{Actor: actor.id, Principal: actor.id, VerifiedKey: actor.key})
	if err == nil {
		require.Equal(t, global, receipt.GlobalSequence)
	}
	return receipt, err
}

func proofPayloadSend(t *testing.T, args types.SendArgs, signer proofPayloadActor) []byte {
	t.Helper()
	payload, err := types.EncodeSendPayload(args)
	require.NoError(t, err)
	require.NoError(t, types.ValidateSendEnvelope(payload))
	require.Len(t, payload, 364)
	require.Equal(t, make([]byte, 64), payload[262:326], "producer leaves only the signature slot unsigned")
	// lxp_send_authorization_message (src/ledger/lxp_send.c): tag, common
	// fields, authority kind/controller, signed context, network and version.
	message := append([]byte(nil), payload[:2]...)
	message = append(message, payload[4:230]...)
	message = append(message, payload[326:364]...)
	hash, err := codec.DomainHash(codec.DomainSignaturePreimage, message)
	require.NoError(t, err)
	copy(payload[262:326], ed25519.Sign(signer.private, hash[:]))
	return payload
}

func TestProofPayloadCodec(t *testing.T) {
	issuer, receiver := proofPayloadIdentity(t, 21), proofPayloadIdentity(t, 22)
	salt := [32]byte{0x31}
	material := append([]byte("LX:ASSET:v1"), issuer.id[:]...)
	asset := sha256.Sum256(append(material, salt[:]...))
	args := types.RegisterArgs{AssetID: asset, Salt: salt, Symbol: "PAX", Name: "Paxéer",
		Decimals: 18, SupplyCap: sdk.NewInt(1000), IssuerKind: types.AssetIssuerNative}
	register, err := types.EncodeRegisterPayload(args)
	require.NoError(t, err)
	require.NoError(t, types.ValidateRegisterPayload(register))
	directory := t.TempDir()
	n := proofPayloadOpen(t, directory)
	require.Zero(t, n.execute(t, issuer, 0x00010001, register, [32]byte{1}).ResultCode)
	for i, actor := range []proofPayloadActor{issuer, receiver} {
		open := types.EncodeOpenAccountPayload(asset)
		require.NoError(t, types.ValidatePausePayload(open))
		require.Zero(t, n.execute(t, actor, 0x00010004, open, [32]byte{2, byte(i)}).ResultCode)
	}
	name, from, err := lxphost.AssetAccountID(issuer.key, asset)
	require.NoError(t, err)
	_, to, err := lxphost.AssetAccountID(receiver.key, asset)
	require.NoError(t, err)
	mint, err := types.EncodeSupplyPayload(asset, from, sdk.NewInt(250))
	require.NoError(t, err)
	require.Zero(t, n.execute(t, issuer, 0x0001000a, mint, [32]byte{3}).ResultCode)
	key := [32]byte{4}
	contextMaterial := append(append(append([]byte(nil), from[:]...), to[:]...), asset[:]...)
	var amount [16]byte
	big.NewInt(40).FillBytes(amount[:])
	contextMaterial = append(contextMaterial, amount[:]...)
	contextHash, err := codec.DomainHash(codec.DomainContextHash, append(contextMaterial, key[:]...))
	require.NoError(t, err)
	sendArgs := types.SendArgs{From: from, To: to, Asset: asset, Amount: sdk.NewInt(40),
		Sequence: 0, IdempotencyKey: key, ExpiresAt: uint64(proofPayloadTime + 1000),
		ContextHash: contextHash, PublicKey: issuer.key, NetworkID: 125, ProtocolVersion: 3}
	send := proofPayloadSend(t, sendArgs, issuer)
	receipt := n.execute(t, issuer, 0x00010005, send, key)
	require.Zero(t, receipt.ResultCode)
	require.Equal(t, []lxphost.TransferLeg{{Asset: asset, From: from, To: to, Amount: sdk.NewInt(40)}}, receipt.Transfers)
	pause := types.EncodePausePayload(asset)
	require.NoError(t, types.ValidatePausePayload(pause))
	require.Zero(t, n.execute(t, issuer, 0x00010002, pause, [32]byte{5}).ResultCode)
	before, err := n.host.ReadAsset(n.ctx, asset)
	require.NoError(t, err)
	require.Equal(t, args.AssetID, before.AssetID)
	require.Equal(t, args.Salt, before.Salt)
	require.Equal(t, args.Symbol, before.Symbol)
	require.Equal(t, args.Name, before.Name)
	require.Equal(t, args.Decimals, before.Decimals)
	require.Equal(t, args.SupplyCap, before.SupplyCap)
	require.Equal(t, issuer.id, before.IssuerDID)
	require.True(t, before.Paused)
	n.cms.Commit(true)
	n.close(t) // Neither the host nor its store survives into the consumer read.
	n = proofPayloadOpen(t, directory)
	recovered, err := n.host.ReadAsset(n.ctx, asset)
	require.NoError(t, err)
	require.Equal(t, before, recovered)
	assertBalances := func() {
		for _, row := range []struct {
			account [32]byte
			amount  int64
		}{{from, 210}, {to, 40}} {
			balance, err := n.host.ReadBalance(n.ctx, row.account, asset)
			require.NoError(t, err)
			require.Equal(t, sdk.NewInt(row.amount), balance)
		}
		total, err := n.host.ReadTotalUnits(n.ctx, asset)
		require.NoError(t, err)
		require.Equal(t, sdk.NewInt(250), total)
	}
	assertBalances()
	account, err := n.host.ReadAccount(n.ctx, name, from)
	require.NoError(t, err)
	require.Equal(t, uint64(1), account.NextSequence)
	again, err := types.EncodeRegisterPayload(args)
	require.NoError(t, err)
	require.Equal(t, register, again)
	require.Equal(t, lxphost.Code(-411), n.execute(t, issuer, 0x00010001, again, [32]byte{6}).ResultCode)
	globalBefore, err := lxphost.NextSequence(n.ctx, n.key)
	require.NoError(t, err)
	identityBefore, err := n.host.ReadIdentitySequence(n.ctx, issuer.did)
	require.NoError(t, err)
	duplicate, err := n.submit(t, issuer, 0x00010005, send, key)
	require.ErrorIs(t, err, lxphost.Code(-302), "decision 2337 preserves successful payment references across restart")
	require.Empty(t, duplicate.Encoded)
	globalAfter, err := lxphost.NextSequence(n.ctx, n.key)
	require.NoError(t, err)
	require.Equal(t, globalBefore, globalAfter)
	identityAfter, err := n.host.ReadIdentitySequence(n.ctx, issuer.did)
	require.NoError(t, err)
	require.Equal(t, identityBefore, identityAfter)
	assertBalances()
	pausedArgs := sendArgs
	pausedArgs.Sequence = account.NextSequence
	pausedArgs.IdempotencyKey = [32]byte{10}
	pausedArgs.ContextHash, err = codec.DomainHash(codec.DomainContextHash, append(contextMaterial, pausedArgs.IdempotencyKey[:]...))
	require.NoError(t, err)
	pausedSend := proofPayloadSend(t, pausedArgs, issuer)
	require.Equal(t, lxphost.Code(-403), n.execute(t, issuer, 0x00010005, pausedSend, pausedArgs.IdempotencyKey).ResultCode)
	assertBalances()
	// Repeat each active producer after restart; output must remain independent
	// of any cached host state and of previous payload buffers.
	require.Equal(t, pause, types.EncodePausePayload(asset))
	require.Equal(t, types.EncodeOpenAccountPayload(asset), pause)
	repeatedMint, err := types.EncodeSupplyPayload(asset, from, sdk.NewInt(250))
	require.NoError(t, err)
	require.Equal(t, mint, repeatedMint)
	require.Equal(t, send, proofPayloadSend(t, sendArgs, issuer))
	malformed := append(append([]byte(nil), register...), 0)
	require.ErrorIs(t, types.ValidateRegisterPayload(malformed), types.ErrNonCanonical)
	require.Equal(t, lxphost.Code(-3), n.execute(t, issuer, 0x00010001, malformed, [32]byte{7}).ResultCode)
	proofPayloadLegacy(t, n, issuer, receiver, asset, from, to)
	assertBalances()
}

func proofPayloadLegacy(t *testing.T, n *proofPayloadNode, issuer, receiver proofPayloadActor, asset, from, to [32]byte) {
	t.Helper()
	// The shape validator deliberately has no signature/state authority.
	grant := make([]byte, 346)
	require.NoError(t, types.ValidateIssueGrantPayload(grant))
	for _, offset := range []int{160, 209} {
		bad := append([]byte(nil), grant...)
		bad[offset] = 2
		require.ErrorIs(t, types.ValidateIssueGrantPayload(bad), types.ErrNonCanonical)
	}
	require.ErrorIs(t, types.ValidateIssueGrantPayload(grant[:345]), types.ErrNonCanonical)
	require.ErrorIs(t, types.ValidateIssueGrantPayload(append(append([]byte(nil), grant...), 0)), types.ErrNonCanonical)
	args := types.ReceiveArgs{From: from, To: to, Asset: asset, Amount: sdk.NewInt(7),
		GrantID: [32]byte{0x41}, ReceiverSequence: 3, IdempotencyKey: [32]byte{8},
		PublicKey: receiver.key, NetworkID: 125, ProtocolVersion: 3, PayerGrant: grant}
	receive, err := types.EncodeReceivePayload(args)
	require.NoError(t, err)
	require.Len(t, receive, 733)
	require.Equal(t, uint16(0x5201), binary.BigEndian.Uint16(receive[:2]))
	require.Equal(t, from[:], receive[4:36])
	require.Equal(t, to[:], receive[36:68])
	require.Equal(t, asset[:], receive[68:100])
	require.Equal(t, big.NewInt(7), new(big.Int).SetBytes(receive[100:116]))
	require.Equal(t, args.GrantID[:], receive[116:148])
	require.Equal(t, args.ReceiverSequence, binary.BigEndian.Uint64(receive[148:156]))
	require.NoError(t, types.ValidateIssueGrantPayload(receive[387:]))
	repeated, err := types.EncodeReceivePayload(args)
	require.NoError(t, err)
	require.Equal(t, receive, repeated)
	revoke := types.EncodeRevokeGrantPayload(args.GrantID, 0x0102030405060708)
	require.Len(t, revoke, 42)
	require.Equal(t, uint16(1), binary.BigEndian.Uint16(revoke[:2]))
	require.Equal(t, args.GrantID[:], revoke[2:34])
	require.Equal(t, uint64(0x0102030405060708), binary.BigEndian.Uint64(revoke[34:]))
	require.Equal(t, revoke, types.EncodeRevokeGrantPayload(args.GrantID, 0x0102030405060708))
	for _, row := range []struct {
		kind    uint32
		payload []byte
	}{
		{0x00010006, receive}, {0x00010007, grant}, {0x00010008, revoke},
	} {
		before, err := lxphost.NextSequence(n.ctx, n.key)
		require.NoError(t, err)
		receipt, err := n.submit(t, issuer, row.kind, row.payload, [32]byte{9, byte(row.kind)})
		require.ErrorIs(t, err, lxphost.Code(-106),
			"src/modules/asset/lx_asset_registry.c: activity_types must refuse retired activity %#x", row.kind)
		require.Empty(t, receipt.Encoded, "an unregistered route must not produce a receipt")
		after, err := lxphost.NextSequence(n.ctx, n.key)
		require.NoError(t, err)
		require.Equal(t, before, after, "unregistered routes must not consume a sequence")
	}
	grant[160] = 2
	require.NoError(t, types.ValidateIssueGrantPayload(receive[387:]), "receive must own its grant copy")
	args.PayerGrant = grant
	invalid, err := types.EncodeReceivePayload(args)
	require.ErrorIs(t, err, types.ErrNonCanonical)
	require.Nil(t, invalid)
}

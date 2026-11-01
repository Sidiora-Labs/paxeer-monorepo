package keeper_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxexchange/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxexchange/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

type oracleFixture struct {
	keeper    *keeper.Keeper
	ctx       sdk.Context
	host      *lxphost.Host
	key       *sdk.KVStoreKey
	validator sdk.ValAddress
	private   ed25519.PrivateKey
	public    [32]byte
	did       []byte
	authority lxphost.KernelAuthority
	feed      [32]byte
}

func newOracleFixture(t *testing.T) *oracleFixture {
	t.Helper()
	k, ctx := newMarginHopKeeper(t)
	key := sdk.NewKVStoreKey("exchange-oracle-host")
	ctx = withStore(t, ctx, key)
	host, err := lxphost.NewHost(lxphost.Config{StoreKey: key, NetworkID: 7,
		ParameterVersion: 1, TimestampWindowMs: 60000, GasLimit: 1000000})
	require.NoError(t, err)
	t.Cleanup(host.Close)
	f := &oracleFixture{keeper: k, ctx: ctx, host: host, key: key, feed: [32]byte{0x93, 1},
		private: ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x39}, ed25519.SeedSize))}
	copy(f.public[:], f.private.Public().(ed25519.PublicKey))
	validatorHash := sha256.Sum256(f.public[:])
	f.validator = sdk.ValAddress(validatorHash[:20])
	f.did = []byte("did:layerx:" + hex.EncodeToString(f.public[:]))
	id, err := lxphost.DIDID(f.did)
	require.NoError(t, err)
	f.authority = lxphost.KernelAuthority{Actor: id, Principal: id, VerifiedKey: f.public}
	registration := append([]byte{0x71, 1, 0, 2}, id[:]...)
	registration = append(registration, f.public[:]...)
	receipt, err := host.Execute(lxphost.WithSequence(ctx, 1),
		f.sign(t, 0x00070001, 0, registration), f.authority)
	require.NoError(t, err)
	require.Equal(t, lxphost.Code(0), receipt.ResultCode)

	// Genesis market configuration; observations are committed only by Execute.
	market := make([]byte, 622)
	copy(market[:32], f.feed[:])
	copy(market[64:96], id[:])
	for i, offset := range []int{32, 96, 128, 160, 192} {
		market[offset] = byte(i + 1)
	}
	for _, offset := range []int{224, 240, 256, 272} {
		market[offset+15] = 1
	}
	for i, value := range []uint32{1000, 500, 100, 5000, 100, 10000} {
		binary.BigEndian.PutUint32(market[288+i*4:], value)
	}
	binary.BigEndian.PutUint64(market[312:], 60000)
	binary.BigEndian.PutUint64(market[320:], 60000)
	market[343] = 1
	copy(market[344:360], bytes.Repeat([]byte{0xff}, 16))
	market[360] = 1
	copy(market[361:393], f.public[:])
	binary.BigEndian.PutUint32(market[617:], 1)
	ctx.KVStore(key).Set(append([]byte{1, 0, 6, 'm', 'a', 'r', 'k', 'e', 't', ':'}, f.feed[:]...), market)
	return f
}

func (f *oracleFixture) sign(t *testing.T, kind uint32, sequence uint64, payload []byte) lxphost.Activity {
	t.Helper()
	now := uint64(f.ctx.BlockTime().UnixMilli())
	activity := lxphost.Activity{ProtocolVersion: 3, Type: kind, ActorDID: f.did,
		AuthorityKey: f.public, AccountSequence: sequence, NotBefore: now - 1000, NotAfter: now + 1000,
		IdempotencyKey: sha256.Sum256(append(binary.BigEndian.AppendUint64(nil, sequence), payload...)),
		Payload:        payload}
	digest, err := f.host.SigningHash(activity)
	require.NoError(t, err)
	activity.Signature = ed25519.Sign(f.private, digest[:])
	return activity
}

func (f *oracleFixture) observation(t *testing.T, sequence uint64, price sdk.Int) lxphost.Activity {
	t.Helper()
	payload := append([]byte(nil), f.feed[:]...)
	payload = binary.BigEndian.AppendUint64(payload, sequence)
	amount := make([]byte, 16)
	price.BigInt().FillBytes(amount)
	payload = append(payload, amount...)
	payload = binary.BigEndian.AppendUint64(payload, uint64(f.ctx.BlockTime().UnixMilli()))
	payload = binary.BigEndian.AppendUint64(payload, 1)
	digest, err := codec.DomainHash(codec.DomainSignaturePreimage,
		append([]byte("LXP:ORACLE:OBSERVATION:v1"), payload...))
	require.NoError(t, err)
	transport := append([]byte{1}, payload...)
	transport = append(transport, ed25519.Sign(f.private, digest[:])...)
	return f.sign(t, 0x00060003, sequence, transport)
}

func (f *oracleFixture) replay(activity lxphost.Activity, globalSequence uint64) sdk.Context {
	return keeper.WithOracleReplay(lxphost.WithSequence(f.ctx, globalSequence),
		f.host, f.key, f.validator, activity, f.authority)
}

func TestSubmitThenCommittedOraclePrice(t *testing.T) {
	f := newOracleFixture(t)
	price, ok := sdk.NewIntFromString("1208925819614629174718521")
	require.True(t, ok)
	activity := f.observation(t, 1, price)
	ctx := f.replay(activity, 2)
	before, found, err := f.keeper.CommittedOraclePrice(ctx, f.validator, f.feed)
	require.NoError(t, err)
	require.False(t, found)
	require.Equal(t, keeper.OraclePrice{}, before)

	require.NoError(t, f.keeper.SubmitOracleObservation(ctx, f.validator, f.feed, price, 9750))
	committed, found, err := f.keeper.CommittedOraclePrice(ctx, f.validator, f.feed)
	require.NoError(t, err)
	require.True(t, found)
	require.True(t, price.Equal(committed.Price), "%s != %s", price, committed.Price)
	require.Equal(t, uint64(ctx.BlockTime().UnixMilli()), committed.ObservedAt)
	require.Equal(t, uint64(1), committed.ObservationSequence)
	require.NotEqual(t, [32]byte{}, committed.SourceSetDigest)

	var exchange sdk.KVStore
	for _, key := range ctx.MultiStore().StoreKeys() {
		if key.Name() == types.StoreKey {
			exchange = ctx.KVStore(key)
		}
	}
	require.NotNil(t, exchange)
	var recorded struct {
		Validator  sdk.ValAddress
		Confidence uint64
		Activity   lxphost.Activity
		Receipt    lxphost.Receipt
	}
	require.NoError(t, json.Unmarshal(exchange.Get(append([]byte("oracle/activity/"), activity.IdempotencyKey[:]...)), &recorded))
	require.Equal(t, f.validator, recorded.Validator)
	require.Equal(t, uint64(9750), recorded.Confidence)
	require.Equal(t, activity, recorded.Activity)
	require.Equal(t, lxphost.Code(0), recorded.Receipt.ResultCode)
	require.Equal(t, uint64(2), recorded.Receipt.GlobalSequence)
	require.NotEmpty(t, recorded.Receipt.Encoded)

	fresh := keeper.NewKeeper(nil, nil, nil, nil)
	reread, found, err := fresh.CommittedOraclePrice(ctx, f.validator, f.feed)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, committed, reread)

	nextPrice := price.AddRaw(7)
	next := f.observation(t, 2, nextPrice)
	nextCtx := f.replay(next, 3)
	require.Error(t, f.keeper.SubmitOracleObservation(nextCtx, f.validator, f.feed, price, 9750))
	require.NoError(t, f.keeper.SubmitOracleObservation(nextCtx, f.validator, f.feed, nextPrice, 9750))
	latest, found, err := f.keeper.CommittedOraclePrice(nextCtx, f.validator, f.feed)
	require.NoError(t, err)
	require.True(t, found)
	require.True(t, nextPrice.Equal(latest.Price))
	require.Equal(t, uint64(2), latest.ObservationSequence)
}

func TestCommittedOraclePriceUnknownFeed(t *testing.T) {
	f := newOracleFixture(t)
	ctx := f.replay(lxphost.Activity{}, 2)
	for _, feed := range [][32]byte{{0xff}, f.feed} {
		committed, found, err := f.keeper.CommittedOraclePrice(ctx, f.validator, feed)
		require.NoError(t, err)
		require.False(t, found)
		require.Equal(t, keeper.OraclePrice{}, committed)
	}
	_, found, err := f.keeper.CommittedOraclePrice(f.ctx, f.validator, f.feed)
	require.ErrorIs(t, err, types.ErrKeeperMissing)
	require.False(t, found)
	key := append([]byte{1, 0, 6, 'o', 'r', 'a', 'c', 'l', 'e', ':'}, f.feed[:]...)
	ctx.KVStore(f.key).Set(key, []byte{1})
	_, found, err = f.keeper.CommittedOraclePrice(ctx, f.validator, f.feed)
	require.Error(t, err)
	require.False(t, found)
}

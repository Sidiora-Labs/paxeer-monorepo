package oracle_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"testing"
	"time"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	exchangekeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxexchange/keeper"
	exchangetypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxexchange/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	agentcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/oracle"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/cachemulti"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/dbadapter"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	upgradekeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/keeper"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

type oracleFixture struct {
	p           *agentcommon.Precompile
	ctx         sdk.Context
	exchange    *exchangekeeper.Keeper
	host        *lxphost.Host
	hostKey     *sdk.KVStoreKey
	exchangeKey *sdk.KVStoreKey
	feed        [32]byte
	private     [2]ed25519.PrivateKey
	public      [2][32]byte
	did         [2][]byte
	authority   [2]lxphost.KernelAuthority
	caller      [2]common.Address
}

func mounted(parent sdk.MultiStore, key sdk.StoreKey) (store sdk.KVStore, ok bool) {
	defer func() {
		if recover() != nil {
			store, ok = nil, false
		}
	}()
	return parent.GetKVStore(key), true
}

func withStores(t *testing.T, ctx sdk.Context, added ...*sdk.KVStoreKey) sdk.Context {
	t.Helper()
	stores := map[sdk.StoreKey]sdk.CacheWrapper{}
	keys := map[string]sdk.StoreKey{}
	for _, key := range ctx.MultiStore().StoreKeys() {
		if store, ok := mounted(ctx.MultiStore(), key); ok {
			stores[key], keys[key.Name()] = store, key
		}
	}
	db := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	for _, key := range added {
		stores[key], keys[key.Name()] = dbadapter.Store{DB: dbm.NewPrefixDB(db, []byte(key.Name()))}, key
	}
	return ctx.WithMultiStore(cachemulti.NewStore(db, stores, keys, nil, nil, nil, 0))
}

func fixture(t *testing.T) *oracleFixture {
	t.Helper()
	app := testkeeper.EVMTestApp
	ctx, _ := app.NewContext(false, tmtypes.Header{}).WithBlockHeight(11).WithBlockTime(time.Unix(1800000000, 987000000).UTC()).CacheContext()
	exchangeKey := sdk.NewKVStoreKey(exchangetypes.StoreKey)
	hostKey := sdk.NewKVStoreKey("oracle-precompile-host")
	upgradeKey := sdk.NewKVStoreKey("oracle-precompile-upgrade")
	retirementKey := sdk.NewKVStoreKey("oracle-exchange-retirement")
	ctx = withStores(t, ctx, exchangeKey, hostKey, upgradeKey, retirementKey)
	upgrade := upgradekeeper.NewKeeper(nil, upgradeKey, app.AppCodec(), t.TempDir(), nil)
	upgrade.SetDone(ctx, "oracle-merge")
	exchange := exchangekeeper.NewKeeper(app.AppCodec(), exchangeKey, nil, &app.EvmKeeper)
	exchange.SetUpgradeKeeper(upgradekeeper.NewKeeper(nil, retirementKey, app.AppCodec(), t.TempDir(), nil))
	exchange.InitGenesis(ctx, *exchangetypes.DefaultGenesis())
	p, err := oracle.NewPrecompile(exchange, &app.EvmKeeper, &upgrade, "oracle-merge", 4, big.NewInt(10000))
	require.NoError(t, err)
	host, err := lxphost.NewHost(lxphost.Config{StoreKey: hostKey, NetworkID: 7, ParameterVersion: 1, TimestampWindowMs: 60000, GasLimit: 1000000})
	require.NoError(t, err)
	t.Cleanup(host.Close)
	f := &oracleFixture{p: p, ctx: ctx, exchange: exchange, host: host, hostKey: hostKey, exchangeKey: exchangeKey, feed: [32]byte{0x93, 1}}
	for i := range f.private {
		f.private[i] = ed25519.NewKeyFromSeed(bytes.Repeat([]byte{byte(0x39 + i)}, ed25519.SeedSize))
		copy(f.public[i][:], f.private[i].Public().(ed25519.PublicKey))
		f.did[i] = []byte("did:layerx:" + hex.EncodeToString(f.public[i][:]))
		id, err := lxphost.DIDID(f.did[i])
		require.NoError(t, err)
		f.authority[i] = lxphost.KernelAuthority{Actor: id, Principal: id, VerifiedKey: f.public[i]}
		registration := append([]byte{0x71, 1, 0, 2}, id[:]...)
		registration = append(registration, f.public[i][:]...)
		receipt, err := host.Execute(lxphost.WithSequence(ctx, uint64(i+1)), f.sign(t, i, 0x00070001, 0, registration), f.authority[i])
		require.NoError(t, err)
		require.Equal(t, lxphost.Code(0), receipt.ResultCode)
		f.caller[i] = common.Address{19: byte(i + 1)}
		message, err := evmtypes.LayerXBindMessage(app.EvmKeeper.ChainID(ctx), f.caller[i], app.EvmKeeper.GetLayerXBindNonce(ctx, f.caller[i]))
		require.NoError(t, err)
		var signature [64]byte
		copy(signature[:], ed25519.Sign(f.private[i], message))
		_, err = app.EvmKeeper.BindLayerX(ctx, f.caller[i], f.public[i], signature)
		require.NoError(t, err)
	}
	market := make([]byte, 622)
	copy(market[:32], f.feed[:])
	copy(market[64:96], f.authority[0].Actor[:])
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
	copy(market[361:393], f.public[0][:])
	binary.BigEndian.PutUint32(market[617:], 1)
	ctx.KVStore(hostKey).Set(append([]byte{1, 0, 6, 'm', 'a', 'r', 'k', 'e', 't', ':'}, f.feed[:]...), market)
	return f
}

func (f *oracleFixture) sign(t *testing.T, signer int, kind uint32, sequence uint64, payload []byte) lxphost.Activity {
	t.Helper()
	now := uint64(f.ctx.BlockTime().UnixMilli())
	activity := lxphost.Activity{ProtocolVersion: 3, Type: kind, ActorDID: f.did[signer], AuthorityKey: f.public[signer], AccountSequence: sequence, NotBefore: now - 1000, NotAfter: now + 1000, IdempotencyKey: sha256.Sum256(append(binary.BigEndian.AppendUint64(nil, sequence), payload...)), Payload: payload}
	digest, err := f.host.SigningHash(activity)
	require.NoError(t, err)
	activity.Signature = ed25519.Sign(f.private[signer], digest[:])
	return activity
}

func (f *oracleFixture) observation(t *testing.T, signer int, price *big.Int) (sdk.Context, lxphost.Activity) {
	t.Helper()
	payload := append([]byte(nil), f.feed[:]...)
	payload = binary.BigEndian.AppendUint64(payload, 1)
	amount := make([]byte, 16)
	price.FillBytes(amount)
	payload = append(payload, amount...)
	payload = binary.BigEndian.AppendUint64(payload, uint64(f.ctx.BlockTime().UnixMilli()))
	payload = binary.BigEndian.AppendUint64(payload, 1)
	digest, err := codec.DomainHash(codec.DomainSignaturePreimage, append([]byte("LXP:ORACLE:OBSERVATION:v1"), payload...))
	require.NoError(t, err)
	transport := append([]byte{1}, payload...)
	transport = append(transport, ed25519.Sign(f.private[signer], digest[:])...)
	activity := f.sign(t, signer, 0x00060003, 1, transport)
	hash := sha256.Sum256(f.public[signer][:])
	ctx := oracle.WithOracleReplay(lxphost.WithSequence(f.ctx, 3), f.host, f.hostKey, sdk.ValAddress(hash[:20]), activity, f.authority[signer])
	return ctx, activity
}

func call(t *testing.T, f *oracleFixture, ctx sdk.Context, caller common.Address, method string, args ...interface{}) ([]byte, error) {
	t.Helper()
	input, err := f.p.GetABI().Pack(method, args...)
	require.NoError(t, err)
	db := state.NewDBImpl(ctx, &testkeeper.EVMTestApp.EvmKeeper, false)
	data, callErr := f.p.Run(&vm.EVM{StateDB: db}, caller, f.p.Address(), input, nil, method != oracle.SubmitPriceMethod, false, nil)
	require.Empty(t, db.Logs())
	if callErr == nil && method == oracle.SubmitPriceMethod {
		_, err := db.Finalize()
		require.NoError(t, err)
	}
	return data, callErr
}

func revertText(t *testing.T, data []byte) string {
	t.Helper()
	text, err := abi.UnpackRevert(data)
	require.NoError(t, err)
	return text
}
func feed(price, confidence int64) oracle.Feed {
	return oracle.Feed{Price: big.NewInt(price), Confidence: big.NewInt(confidence), Timestamp: big.NewInt(123)}
}

func TestOracleAggregateWeightedMedian(t *testing.T) {
	f := fixture(t)
	require.Equal(t, oracle.OracleAddress, f.p.Address().Hex())
	for name, selector := range map[string]string{oracle.AggregateMethod: "81097a95", oracle.SubmitPriceMethod: "dc9c2875", oracle.GetValidatorPriceMethod: "99812615"} {
		require.Equal(t, selector, hex.EncodeToString(f.p.GetABI().Methods[name].ID))
	}
	for _, tc := range []struct {
		name              string
		feeds             []oracle.Feed
		price, confidence int64
	}{
		{"single", []oracle.Feed{feed(44, 7)}, 44, 7},
		{"discard zero", []oracle.Feed{feed(10, 0), feed(44, 7)}, 44, 7},
		{"weighted", []oracle.Feed{feed(100, 1), feed(30, 6), feed(10, 2)}, 30, 1},
		{"even lower tie", []oracle.Feed{feed(30, 2), feed(10, 2)}, 10, 2},
		{"odd floor", []oracle.Feed{feed(30, 3), feed(10, 2)}, 10, 2},
		{"minimum across all kept", []oracle.Feed{feed(30, 8), feed(20, 1), feed(10, 2), feed(40, 0)}, 30, 1},
		{"equal prices", []oracle.Feed{feed(20, 4), feed(20, 1), feed(10, 1)}, 20, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before, err := json.Marshal(tc.feeds)
			require.NoError(t, err)
			price, confidence, err := oracle.Aggregate(tc.feeds, 4)
			require.NoError(t, err)
			require.Equal(t, big.NewInt(tc.price), price)
			require.Equal(t, big.NewInt(tc.confidence), confidence)
			data, err := call(t, f, f.ctx, common.Address{}, oracle.AggregateMethod, tc.feeds)
			require.NoError(t, err)
			values, err := f.p.GetABI().Unpack(oracle.AggregateMethod, data)
			require.NoError(t, err)
			require.Equal(t, []interface{}{big.NewInt(tc.price), big.NewInt(tc.confidence)}, values)
			price.SetInt64(999)
			confidence.SetInt64(999)
			after, err := json.Marshal(tc.feeds)
			require.NoError(t, err)
			require.Equal(t, before, after)
		})
	}
}

func TestOracleAggregateRefusals(t *testing.T) {
	f := fixture(t)
	for _, tc := range []struct {
		name   string
		feeds  []oracle.Feed
		code   agentcommon.ErrorCode
		reason string
	}{
		{"empty", []oracle.Feed{}, agentcommon.ErrOracleNoFeeds, "LXP_AGENT_ORACLE_NO_FEEDS: no feeds provided"},
		{"no confidence", []oracle.Feed{feed(10, 0), feed(20, 0)}, agentcommon.ErrOracleNoConfidentFeeds, "LXP_AGENT_ORACLE_NO_CONFIDENT_FEEDS: no feeds with non-zero confidence"},
		{"zero price", []oracle.Feed{feed(0, 1)}, agentcommon.ErrOraclePrice, "LXP_AGENT_ORACLE_PRICE: invalid price"},
		{"negative price", []oracle.Feed{feed(-1, 1)}, agentcommon.ErrOraclePrice, "LXP_AGENT_ORACLE_PRICE: invalid price"},
		{"zero confidence invalid price", []oracle.Feed{feed(0, 0), feed(20, 1)}, agentcommon.ErrOraclePrice, "LXP_AGENT_ORACLE_PRICE: invalid price"},
		{"above confidence cap", []oracle.Feed{feed(10, 10001)}, agentcommon.ErrOracleConfidence, "LXP_AGENT_ORACLE_CONFIDENCE: invalid confidence"},
		{"above batch cap", []oracle.Feed{feed(1, 1), feed(2, 1), feed(3, 1), feed(4, 1), feed(5, 1)}, agentcommon.ErrBatchTooLarge, "LXP_AGENT_BATCH_TOO_LARGE: batch over the cap (cap set at activation rehearsal)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.code != agentcommon.ErrOracleConfidence {
				_, _, err := oracle.Aggregate(tc.feeds, 4)
				require.ErrorIs(t, err, tc.code)
			}
			data, err := call(t, f, f.ctx, common.Address{}, oracle.AggregateMethod, tc.feeds)
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			require.Equal(t, tc.reason, revertText(t, data))
		})
	}
}

func TestOracleSubmitPriceRequiresPermittedKey(t *testing.T) {
	f := fixture(t)
	price := big.NewInt(1000)
	deniedCtx, _ := f.observation(t, 1, price)
	data, err := call(t, f, deniedCtx, f.caller[1], oracle.SubmitPriceMethod, f.feed, price, big.NewInt(5))
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	require.Equal(t, "LXP_AGENT_ORACLE_NOT_SIGNER: caller is not a validator oracle signer", revertText(t, data))
	ctx, activity := f.observation(t, 0, price)
	hash := sha256.Sum256(f.public[0][:])
	validator := sdk.ValAddress(hash[:20])
	_, found, err := f.exchange.CommittedOraclePrice(ctx, validator, f.feed)
	require.NoError(t, err)
	require.False(t, found)
	for _, invalid := range []*big.Int{big.NewInt(0), big.NewInt(-1), new(big.Int).Lsh(big.NewInt(1), 128)} {
		data, err := call(t, f, ctx, f.caller[0], oracle.SubmitPriceMethod, f.feed, invalid, big.NewInt(5))
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		require.Equal(t, "LXP_AGENT_ORACLE_PRICE: invalid price", revertText(t, data))
	}
	for _, confidence := range []*big.Int{big.NewInt(10001), new(big.Int).Lsh(big.NewInt(1), 64)} {
		data, err := call(t, f, ctx, f.caller[0], oracle.SubmitPriceMethod, f.feed, price, confidence)
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		require.Equal(t, "LXP_AGENT_ORACLE_CONFIDENCE: invalid confidence", revertText(t, data))
	}
	input, err := f.p.GetABI().Pack(oracle.SubmitPriceMethod, f.feed, price, big.NewInt(5))
	require.NoError(t, err)
	for _, guard := range []struct {
		name               string
		readOnly, delegate bool
		value              *big.Int
		reason             string
	}{
		{"static", true, false, nil, "LXP_AGENT_READ_ONLY: write under staticcall"},
		{"delegate", false, true, nil, "LXP_AGENT_DELEGATECALL: cannot delegatecall an agent function"},
		{"nonpayable", false, false, big.NewInt(1), "LXP_PRECOMPILE_NON_PAYABLE: sending funds to a non-payable function"},
	} {
		t.Run(guard.name, func(t *testing.T) {
			db := new(state.DBImpl)
			db.WithCtx(ctx)
			data, err := f.p.Run(&vm.EVM{StateDB: db}, f.caller[0], f.p.Address(), input, guard.value, guard.readOnly, guard.delegate, nil)
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			require.Equal(t, guard.reason, revertText(t, data))
		})
	}
	data, err = call(t, f, ctx, common.Address{19: 99}, oracle.SubmitPriceMethod, f.feed, price, big.NewInt(5))
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	require.Equal(t, "LXP_AGENT_ORACLE_NOT_SIGNER: caller is not a validator oracle signer", revertText(t, data))
	data, err = call(t, f, ctx, f.caller[0], oracle.SubmitPriceMethod, f.feed, price, big.NewInt(5))
	require.NoError(t, err)
	require.Empty(t, data)
	committed, found, err := f.exchange.CommittedOraclePrice(ctx, validator, f.feed)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, price, committed.Price.BigInt())
	require.Equal(t, uint64(ctx.BlockTime().UnixMilli()), committed.ObservedAt)
	require.Equal(t, uint64(1), committed.ObservationSequence)
	var record struct {
		Validator  sdk.ValAddress
		Confidence uint64
		Activity   lxphost.Activity
		Receipt    lxphost.Receipt
	}
	require.NoError(t, json.Unmarshal(ctx.KVStore(f.exchangeKey).Get(append([]byte("oracle/activity/"), activity.IdempotencyKey[:]...)), &record))
	require.Equal(t, validator, record.Validator)
	require.Equal(t, uint64(5), record.Confidence)
	require.Equal(t, activity, record.Activity)
	require.Equal(t, lxphost.Code(0), record.Receipt.ResultCode)
	iterator := sdk.KVStorePrefixIterator(ctx.KVStore(f.exchangeKey), []byte("oracle/activity/"))
	count := 0
	for ; iterator.Valid(); iterator.Next() {
		count++
	}
	require.NoError(t, iterator.Close())
	require.Equal(t, 1, count)
	for _, caller := range []common.Address{f.caller[0], f.caller[1], {19: 99}} {
		data, err := call(t, f, ctx, caller, oracle.GetValidatorPriceMethod, f.feed)
		require.NoError(t, err, "committed reads must answer any caller")
		values, err := f.p.GetABI().Unpack(oracle.GetValidatorPriceMethod, data)
		require.NoError(t, err)
		require.Equal(t, []interface{}{price, big.NewInt(1), big.NewInt(ctx.BlockTime().Unix())}, values)
	}
}

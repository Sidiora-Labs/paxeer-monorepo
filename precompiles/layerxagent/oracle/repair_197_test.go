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

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	exchangetypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxexchange/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/addr"
	agentcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/oracle"
	storev2 "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/storev2/rootmulti"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

type repair197Node struct {
	a             *app.App
	ctx           sdk.Context
	p             precompiles.IPrecompile
	signer, other utils.TestAcct
	private       ed25519.PrivateKey
	public        [32]byte
	did           []byte
	feed          [32]byte
	txIndex       uint64
}

func newRepair197Node(t *testing.T) *repair197Node {
	t.Helper()
	a := app.Setup(t, false, true, false)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	ctx := a.GetContextForDeliverTx(nil).WithChainID("pax-test").WithBlockHeight(10).WithBlockTime(time.Unix(1_800_000_000, 0).UTC())
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx))
	a.UpgradeKeeper.SetDone(ctx, exchangetypes.RetiredFromUpgrade)
	ctx = ctx.WithBlockHeight(11)
	a.UpgradeKeeper.SetDone(ctx, precompiles.MergeUpgrade)
	a.RefreshPrecompiles(ctx)
	p, ok := a.EvmKeeper.CustomPrecompiles(ctx)[common.HexToAddress(oracle.OracleAddress)].(precompiles.IPrecompile)
	require.True(t, ok)
	h := &repair197Node{a: a, ctx: ctx, p: p, signer: utils.NewSigner(), other: utils.NewSigner(), feed: [32]byte{0x19, 0x7}}
	h.private = ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x79}, ed25519.SeedSize))
	copy(h.public[:], h.private.Public().(ed25519.PublicKey))
	h.did = []byte("did:layerx:" + hex.EncodeToString(h.public[:]))
	for _, account := range []utils.TestAcct{h.signer, h.other} {
		a.EvmKeeper.SetAddressMapping(ctx, account.AccountAddress, account.EvmAddress)
		funds := sdk.NewCoins(sdk.NewInt64Coin("uhpx", 10_000_000_000))
		require.NoError(t, a.BankKeeper.MintCoins(ctx, "mint", funds))
		require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, "mint", account.AccountAddress, funds))
	}
	h.bind(t, h.signer, h.private)
	h.register(t, h.private)
	// Genesis market configuration only. All oracle observations and audit
	// records below are produced by SignObservation and real node transactions.
	market := make([]byte, 622)
	copy(market[:32], h.feed[:])
	id, err := lxphost.DIDID(h.did)
	require.NoError(t, err)
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
	copy(market[361:393], h.public[:])
	binary.BigEndian.PutUint32(market[617:], 1)
	ctx.KVStore(a.GetKey(app.LayerXHostStoreKey)).Set(append([]byte{1, 0, 6, 'm', 'a', 'r', 'k', 'e', 't', ':'}, h.feed[:]...), market)
	return h
}

func (h *repair197Node) deliver(t *testing.T, account utils.TestAcct, to common.Address, input []byte, success bool) *evmtypes.Receipt {
	t.Helper()
	signer := ethtypes.LatestSignerForChainID(h.a.EvmKeeper.ChainID(h.ctx))
	signed, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.LegacyTx{
		Nonce: h.a.EvmKeeper.GetNonce(h.ctx, account.EvmAddress), GasPrice: big.NewInt(500_000_000_000),
		Gas: 2_000_000, To: &to, Value: big.NewInt(0), Data: input,
	}), signer, account.EvmPrivateKey)
	require.NoError(t, err)
	recovered, err := ethtypes.Sender(signer, signed)
	require.NoError(t, err)
	require.Equal(t, account.EvmAddress, recovered)
	data, err := ethtx.NewLegacyTx(signed)
	require.NoError(t, err)
	msg, err := evmtypes.NewMsgEVMTransaction(data)
	require.NoError(t, err)
	builder := h.a.GetTxConfig().NewTxBuilder()
	require.NoError(t, builder.SetMsgs(msg))
	builder.SetGasLimit(signed.Gas())
	encoded, err := h.a.GetTxConfig().TxEncoder()(builder.GetTx())
	require.NoError(t, err)
	decoded, err := h.a.GetTxConfig().TxDecoder()(encoded)
	require.NoError(t, err)
	ctx := h.ctx.WithTxIndex(int(h.txIndex)).WithEventManager(sdk.NewEventManager())
	result := h.a.DeliverTx(ctx, abci.RequestDeliverTxV2{Tx: encoded}, decoded, sha256.Sum256(encoded))
	require.NotNil(t, result.EvmTxInfo, result.Log)
	if success {
		require.Zero(t, result.Code, result.Log)
		require.Empty(t, result.EvmTxInfo.VmError)
	} else {
		require.NotZero(t, result.Code, result.Log)
		require.Contains(t, result.EvmTxInfo.VmError, vm.ErrExecutionReverted.Error())
	}
	receipt, err := h.a.EvmKeeper.GetTransientReceipt(h.ctx, signed.Hash(), h.txIndex)
	require.NoError(t, err)
	if success {
		require.Equal(t, uint32(ethtypes.ReceiptStatusSuccessful), receipt.Status)
	} else {
		require.Equal(t, uint32(ethtypes.ReceiptStatusFailed), receipt.Status)
	}
	require.Equal(t, signed.Hash().Hex(), receipt.TxHashHex)
	require.Equal(t, account.EvmAddress.Hex(), receipt.From)
	require.Equal(t, to.Hex(), receipt.To)
	require.Positive(t, receipt.GasUsed)
	h.txIndex++
	return receipt
}

func (h *repair197Node) bind(t *testing.T, account utils.TestAcct, private ed25519.PrivateKey) {
	t.Helper()
	var public [32]byte
	copy(public[:], private.Public().(ed25519.PublicKey))
	message, err := evmtypes.LayerXBindMessage(h.a.EvmKeeper.ChainID(h.ctx), account.EvmAddress, h.a.EvmKeeper.GetLayerXBindNonce(h.ctx, account.EvmAddress))
	require.NoError(t, err)
	p, err := addr.NewPrecompile(h.a.GetPrecompileKeepers())
	require.NoError(t, err)
	input, err := p.GetABI().Pack(addr.BindLayerXMethod, public, ed25519.Sign(private, message))
	require.NoError(t, err)
	h.deliver(t, account, p.Address(), input, true)
	bound, found := h.a.EvmKeeper.GetLayerXDid(h.ctx, account.EvmAddress)
	require.True(t, found)
	require.Equal(t, public, bound)
}

func (h *repair197Node) register(t *testing.T, private ed25519.PrivateKey) {
	t.Helper()
	var public [32]byte
	copy(public[:], private.Public().(ed25519.PublicKey))
	did := []byte("did:layerx:" + hex.EncodeToString(public[:]))
	id, err := lxphost.DIDID(did)
	require.NoError(t, err)
	payload := append([]byte{0x71, 1, 0, 2}, id[:]...)
	payload = append(payload, public[:]...)
	now := uint64(h.ctx.BlockTime().UnixMilli())
	a := lxphost.Activity{ProtocolVersion: 3, Type: 0x00070001, ActorDID: did, AuthorityKey: public, NotBefore: now, NotAfter: now, IdempotencyKey: sha256.Sum256(payload), Payload: payload}
	digest, err := h.a.LayerXHost.SigningHash(a)
	require.NoError(t, err)
	a.Signature = ed25519.Sign(private, digest[:])
	sequence, err := lxphost.NextSequence(h.ctx, h.a.GetKey(app.LayerXHostStoreKey))
	require.NoError(t, err)
	receipt, err := h.a.LayerXHost.Execute(lxphost.WithSequence(h.ctx, sequence), a, lxphost.KernelAuthority{Actor: id, Principal: id, VerifiedKey: public})
	require.NoError(t, err)
	require.Zero(t, receipt.ResultCode, receipt.ResultCode.Error())
}

func (h *repair197Node) observation(t *testing.T, sequence uint64, price int64) (lxphost.Activity, []byte) {
	t.Helper()
	identitySequence, err := h.a.LayerXHost.ReadIdentitySequence(h.ctx, h.did)
	require.NoError(t, err)
	now := uint64(h.ctx.BlockTime().UnixMilli())
	a, err := oracle.SignObservation(h.a.LayerXHost, h.private, oracle.Observation{
		FeedID: h.feed, Price: big.NewInt(price), Sequence: sequence, ObservedAt: now, SourceIdentifier: 1,
	}, identitySequence, now-1000, now+1000)
	require.NoError(t, err)
	input, err := oracle.PackSubmitPrice(h.feed, big.NewInt(price), big.NewInt(5), a)
	require.NoError(t, err)
	return a, input
}

func (h *repair197Node) read(t *testing.T, ctx sdk.Context, caller sdk.AccAddress, price int64) {
	t.Helper()
	input, err := h.p.GetABI().Pack(oracle.GetValidatorPriceMethod, h.feed)
	require.NoError(t, err)
	to := h.p.Address()
	output, err := h.a.EvmKeeper.StaticCallEVM(ctx, caller, &to, input)
	require.NoError(t, err)
	values, err := h.p.GetABI().Unpack(oracle.GetValidatorPriceMethod, output)
	require.NoError(t, err)
	require.Equal(t, []interface{}{big.NewInt(price), big.NewInt(1), big.NewInt(ctx.BlockTime().Unix())}, values)
}

func (h *repair197Node) auditCount(t *testing.T) int {
	t.Helper()
	it := sdk.KVStorePrefixIterator(h.ctx.KVStore(h.a.GetKey(exchangetypes.StoreKey)), []byte("oracle/activity/"))
	defer func() { require.NoError(t, it.Close()) }()
	count := 0
	for ; it.Valid(); it.Next() {
		count++
	}
	return count
}

func TestRepair_197_SignedNodeTransaction(t *testing.T) {
	h := newRepair197Node(t)
	a, input := h.observation(t, 1, 1000)
	decoded, err := h.p.GetABI().Methods[oracle.SubmitPriceMethod].Inputs.Unpack(input[4:])
	require.NoError(t, err)
	require.Equal(t, []interface{}{h.feed, big.NewInt(1000), big.NewInt(5)}, decoded)
	require.Equal(t, uint64(30000+30*len(input)), h.p.RequiredGas(input))
	h.deliver(t, h.signer, h.p.Address(), input, true)
	h.read(t, h.ctx, h.other.AccountAddress, 1000)
	require.Equal(t, 1, h.auditCount(t))
	var recorded struct {
		Activity   lxphost.Activity
		Receipt    lxphost.Receipt
		Confidence uint64
	}
	raw := h.ctx.KVStore(h.a.GetKey(exchangetypes.StoreKey)).Get(append([]byte("oracle/activity/"), a.IdempotencyKey[:]...))
	require.NoError(t, json.Unmarshal(raw, &recorded))
	require.Equal(t, a, recorded.Activity)
	require.Zero(t, recorded.Receipt.ResultCode)
	require.NotEmpty(t, recorded.Receipt.Encoded)
	require.Equal(t, uint64(5), recorded.Confidence)
	before, err := h.a.LayerXHost.ReadIdentitySequence(h.ctx, h.did)
	require.NoError(t, err)
	_, next := h.observation(t, 2, 1010)
	badOuter := bytes.Clone(next)
	badOuter[len(badOuter)-1] ^= 1
	badPrice := bytes.Clone(next)
	badPrice[67] ^= 1
	badMagic := bytes.Clone(next)
	badMagic[100] ^= 1
	for _, row := range []struct {
		name  string
		input []byte
	}{
		{"missing transport", next[:100]}, {"truncated transport", next[:len(next)-1]},
		{"extra bytes", append(bytes.Clone(next), 0)}, {"outer signature", badOuter},
		{"price binding", badPrice}, {"version", badMagic}, {"replay", input},
	} {
		t.Run(row.name, func(t *testing.T) {
			h.deliver(t, h.signer, h.p.Address(), row.input, false)
			h.read(t, h.ctx, h.other.AccountAddress, 1000)
			require.Equal(t, 1, h.auditCount(t))
			current, err := h.a.LayerXHost.ReadIdentitySequence(h.ctx, h.did)
			require.NoError(t, err)
			require.Equal(t, before, current)
		})
	}
	badInner, _ := h.observation(t, 2, 1010)
	badInner.Payload[73] ^= 1
	digest, err := h.a.LayerXHost.SigningHash(badInner)
	require.NoError(t, err)
	badInner.Signature = ed25519.Sign(h.private, digest[:])
	badInnerInput, err := oracle.PackSubmitPrice(h.feed, big.NewInt(1010), big.NewInt(5), badInner)
	require.NoError(t, err)
	h.deliver(t, h.signer, h.p.Address(), badInnerInput, false)
	h.deliver(t, h.other, h.p.Address(), next, false)
	otherKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x7a}, ed25519.SeedSize))
	h.bind(t, h.other, otherKey)
	h.deliver(t, h.other, h.p.Address(), next, false)
	for _, row := range []struct {
		static, delegate bool
		code             agentcommon.ErrorCode
	}{
		{true, false, agentcommon.ErrReadOnly}, {false, true, agentcommon.ErrDelegatecall},
	} {
		db := state.NewDBImpl(h.ctx, &h.a.EvmKeeper, false)
		output, err := h.p.Run(&vm.EVM{StateDB: db}, h.signer.EvmAddress, h.p.Address(), next, nil, row.static, row.delegate, nil)
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		require.ErrorIs(t, db.GetPrecompileError(), row.code)
		require.Contains(t, revertText(t, output), row.code.Error())
	}
	require.Equal(t, 1, h.auditCount(t))
	h.read(t, h.ctx, h.other.AccountAddress, 1000)
	h.deliver(t, h.signer, h.p.Address(), next, true)
	h.read(t, h.ctx, h.other.AccountAddress, 1010)
	require.Equal(t, 2, h.auditCount(t))
}

func TestRepair_197_RetiredExchange(t *testing.T) {
	h := newRepair197Node(t)
	retired, err := h.a.LayerXExchangeKeeper.Retired(h.ctx)
	require.NoError(t, err)
	require.True(t, retired)
	require.True(t, h.a.UpgradeKeeper.IsUpgradeActiveAtHeight(h.ctx, precompiles.MergeUpgrade, h.ctx.BlockHeight()))
	hash := sha256.Sum256(h.public[:])
	err = h.a.LayerXExchangeKeeper.SubmitOracleObservation(h.ctx, sdk.ValAddress(hash[:20]), h.feed, sdk.NewInt(1000), 5)
	require.Error(t, err)
	require.Contains(t, err.Error(), "retired")
	_, input := h.observation(t, 1, 1000)
	h.deliver(t, h.signer, h.p.Address(), input, true)
	live, err := oracle.NewKeeper(h.a.LayerXHost, app.LayerXHostStoreKey, exchangetypes.StoreKey)
	require.NoError(t, err)
	price, found, err := live.CommittedOraclePrice(h.ctx, h.feed)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, big.NewInt(1000), price.Price.BigInt())
	require.Equal(t, uint64(1), price.ObservationSequence)
	require.NotEqual(t, [32]byte{}, price.SourceSetDigest)
	h.read(t, h.ctx, h.other.AccountAddress, 1000)
	retired, err = h.a.LayerXExchangeKeeper.Retired(h.ctx)
	require.NoError(t, err)
	require.True(t, retired)
}

func TestRepair_197_ReadWithoutReplay(t *testing.T) {
	h := newRepair197Node(t)
	dir, keys := t.TempDir(), h.ctx.MultiStore().StoreKeys()
	open := func() sdk.CommitMultiStore {
		cfg := config.DefaultStateCommitConfig()
		cfg.AsyncCommitBuffer = 0
		cfg.MemIAVLConfig.AsyncCommitBuffer = 0
		cfg.MemIAVLConfig.SnapshotInterval = 1
		cfg.MemIAVLConfig.SnapshotMinTimeInterval = 0
		stores := storev2.NewStore(dir, cfg, config.StateStoreConfig{}, nil)
		for _, key := range keys {
			kind := sdk.StoreTypeIAVL
			switch key.(type) {
			case *sdk.TransientStoreKey:
				kind = sdk.StoreTypeTransient
			case *sdk.MemoryStoreKey:
				kind = sdk.StoreTypeMemory
			}
			stores.MountStoreWithDB(key, kind, nil)
		}
		require.NoError(t, stores.LoadLatestVersion())
		return stores
	}
	stores := open()
	for _, key := range keys {
		from, to := h.ctx.KVStore(key), stores.GetKVStore(key)
		it := from.Iterator(nil, nil)
		for ; it.Valid(); it.Next() {
			to.Set(bytes.Clone(it.Key()), bytes.Clone(it.Value()))
		}
		require.NoError(t, it.Close())
	}
	stores.Commit(true)
	h.ctx = h.ctx.WithMultiStore(stores.CacheMultiStore())
	_, input := h.observation(t, 1, 1000)
	h.deliver(t, h.signer, h.p.Address(), input, true)
	h.read(t, h.ctx, h.other.AccountAddress, 1000)
	h.read(t, h.ctx.WithIsCheckTx(true), sdk.AccAddress(bytes.Repeat([]byte{0x42}, 20)), 1000)
	h.ctx.MultiStore().(sdk.CacheMultiStore).Write()
	committed := stores.Commit(true)
	require.NotEmpty(t, committed.Hash)
	h.a.LayerXHost.Close()
	require.NoError(t, stores.Close())
	previous := h.p
	h.p = nil
	stores = open()
	t.Cleanup(func() { require.NoError(t, stores.Close()) })
	require.Equal(t, committed, stores.LastCommitID())
	h.ctx = h.ctx.WithMultiStore(stores.CacheMultiStore()).WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(h.ctx))
	var err error
	h.a.LayerXHost, err = app.NewHost(h.a.GetKey(app.LayerXHostStoreKey), uint32(h.a.EvmKeeper.ChainID(h.ctx).Uint64()))
	require.NoError(t, err)
	set, err := precompiles.GetCustomPrecompilesE(app.LatestUpgrade, h.a.GetPrecompileKeepers())
	require.NoError(t, err)
	h.a.EvmKeeper.SetCustomPrecompiles(set, app.LatestUpgrade)
	var ok bool
	h.p, ok = h.a.EvmKeeper.CustomPrecompiles(h.ctx)[common.HexToAddress(oracle.OracleAddress)].(precompiles.IPrecompile)
	require.True(t, ok)
	require.NotSame(t, previous, h.p)
	h.read(t, h.ctx, h.other.AccountAddress, 1000)
	h.read(t, h.ctx.WithIsCheckTx(true), sdk.AccAddress(bytes.Repeat([]byte{0x43}, 20)), 1000)
	_, input = h.observation(t, 2, 1010)
	h.deliver(t, h.signer, h.p.Address(), input, true)
	h.read(t, h.ctx, h.other.AccountAddress, 1010)
	require.Equal(t, 2, h.auditCount(t))
}

func TestRepair_197_WeightedMedian(t *testing.T) {
	f := fixture(t)
	live, err := oracle.NewKeeper(f.host, f.hostKey.Name(), f.exchangeKey.Name())
	require.NoError(t, err)
	maximum := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	f.p, err = oracle.NewPrecompile(live, &testkeeper.EVMTestApp.EvmKeeper, f.upgrade, "oracle-merge", 32, maximum)
	require.NoError(t, err)
	large := new(big.Int).Lsh(big.NewInt(1), 255)
	for _, row := range []struct {
		name       string
		feeds      []oracle.Feed
		price      int64
		confidence *big.Int
	}{
		{"odd majority", []oracle.Feed{feed(10, 2), feed(30, 3)}, 30, big.NewInt(2)},
		{"even tie", []oracle.Feed{feed(30, 2), feed(10, 2)}, 10, big.NewInt(2)},
		{"unit weights", []oracle.Feed{feed(40, 1), feed(10, 1), feed(30, 1)}, 30, big.NewInt(1)},
		{"small odd", []oracle.Feed{feed(10, 1), feed(30, 2)}, 30, big.NewInt(1)},
		{"large odd", []oracle.Feed{{Price: big.NewInt(10), Confidence: new(big.Int).Sub(large, big.NewInt(1)), Timestamp: big.NewInt(0)}, {Price: big.NewInt(30), Confidence: large, Timestamp: big.NewInt(0)}}, 30, new(big.Int).Sub(large, big.NewInt(1))},
		{"large even sum exceeds uint256", []oracle.Feed{{Price: big.NewInt(30), Confidence: maximum, Timestamp: big.NewInt(0)}, {Price: big.NewInt(10), Confidence: maximum, Timestamp: big.NewInt(0)}}, 10, maximum},
	} {
		t.Run(row.name, func(t *testing.T) {
			before, err := json.Marshal(row.feeds)
			require.NoError(t, err)
			price, confidence, err := oracle.Aggregate(row.feeds, 32)
			require.NoError(t, err)
			require.Equal(t, big.NewInt(row.price), price)
			require.Equal(t, row.confidence, confidence)
			output, err := call(t, f, f.ctx, common.Address{}, oracle.AggregateMethod, row.feeds)
			require.NoError(t, err)
			values, err := f.p.GetABI().Unpack(oracle.AggregateMethod, output)
			require.NoError(t, err)
			require.Equal(t, []interface{}{big.NewInt(row.price), row.confidence}, values)
			price.SetInt64(99)
			confidence.SetInt64(99)
			after, err := json.Marshal(row.feeds)
			require.NoError(t, err)
			require.Equal(t, before, after)
		})
	}
}

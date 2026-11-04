package oracle_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"sort"
	"testing"
	"time"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	blocktypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	exchangetypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxexchange/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	ledgertypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	agentcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/oracle"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/kv"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

// The application test helper uses TempDir as its durable home. Reusing that
// home lets SetupWithDB load the committed node instead of initializing genesis.
type repairM1425Home struct {
	testing.TB
	home string
}

func (h repairM1425Home) TempDir() string { return h.home }

type repairM1425Node struct {
	*repair197Node
	home  repairM1425Home
	dbDir string
}

func newRepairM1425Node(t *testing.T) *repairM1425Node {
	t.Helper()
	h := &repairM1425Node{repair197Node: &repair197Node{}, home: repairM1425Home{t, t.TempDir()}, dbDir: t.TempDir()}
	h.open(t, false)
	t.Cleanup(func() {
		if h.a != nil {
			require.NoError(t, h.a.Close())
		}
	})
	h.a.UpgradeKeeper.SetDone(h.ctx.WithBlockHeight(10), exchangetypes.RetiredFromUpgrade)
	h.a.UpgradeKeeper.SetDone(h.ctx.WithBlockHeight(11), precompiles.MergeUpgrade)
	h.a.RefreshPrecompiles(h.ctx)
	h.selectPrecompile(t)
	h.signer, h.other = utils.NewSigner(), utils.NewSigner()
	h.feed = [32]byte{0x14, 0x25}
	h.private = ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x45}, ed25519.SeedSize))
	copy(h.public[:], h.private.Public().(ed25519.PublicKey))
	h.did = []byte("did:layerx:" + hex.EncodeToString(h.public[:]))
	for _, account := range []utils.TestAcct{h.signer, h.other} {
		h.a.EvmKeeper.SetAddressMapping(h.ctx, account.AccountAddress, account.EvmAddress)
		funds := sdk.NewCoins(sdk.NewInt64Coin("uhpx", 10_000_000_000))
		require.NoError(t, h.a.BankKeeper.MintCoins(h.ctx, "mint", funds))
		require.NoError(t, h.a.BankKeeper.SendCoinsFromModuleToAccount(h.ctx, "mint", account.AccountAddress, funds))
	}
	h.bind(t, h.signer, h.private)
	h.register(t, h.private)
	h.createMarket(t)
	return h
}

func (h *repairM1425Node) open(t *testing.T, restart bool) {
	t.Helper()
	db, err := sdk.NewLevelDB("oracle", h.dbDir)
	require.NoError(t, err)
	h.a = app.SetupWithDB(h.home, db, restart, true, false)
	header := tmtypes.Header{ChainID: "pax-test", Height: 12, Time: time.Unix(1_800_000_000, 987_000_000).UTC()}
	if restart {
		h.ctx = sdk.NewContext(h.a.CommitMultiStore().CacheMultiStore(), header, false)
	} else {
		h.ctx = h.a.GetContextForDeliverTx(nil).WithBlockHeader(header)
	}
	h.ctx = h.ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(h.ctx))
}

func (h *repairM1425Node) selectPrecompile(t *testing.T) {
	t.Helper()
	var ok bool
	h.p, ok = h.a.EvmKeeper.CustomPrecompiles(h.ctx)[common.HexToAddress(oracle.OracleAddress)].(precompiles.IPrecompile)
	require.True(t, ok)
	require.Equal(t, oracle.PrecompileName, h.p.GetName())
	keepers := h.a.GetPrecompileKeepers().(*app.PrecompileKeepers)
	require.Same(t, h.a.LayerXExchangeKeeper, keepers.LayerXExchangeK())
	require.Same(t, h.a.LayerXHost, keepers.LayerXHost())
	retired, err := h.a.LayerXExchangeKeeper.Retired(h.ctx)
	require.NoError(t, err)
	require.True(t, retired)
	require.True(t, h.a.UpgradeKeeper.IsUpgradeActiveAtHeight(h.ctx, precompiles.MergeUpgrade, h.ctx.BlockHeight()))
}

func (h *repairM1425Node) execute(t *testing.T, kind uint32, payload []byte) lxphost.Receipt {
	t.Helper()
	identitySequence, err := h.a.LayerXHost.ReadIdentitySequence(h.ctx, h.did)
	require.NoError(t, err)
	now := uint64(h.ctx.BlockTime().UnixMilli())
	activity := lxphost.Activity{ProtocolVersion: 3, Type: kind, ActorDID: h.did, AuthorityKey: h.public,
		AccountSequence: identitySequence, NotBefore: now - 1000, NotAfter: now + 1000,
		IdempotencyKey: sha256.Sum256(append(binary.BigEndian.AppendUint32(nil, kind), payload...)), Payload: payload}
	digest, err := h.a.LayerXHost.SigningHash(activity)
	require.NoError(t, err)
	activity.Signature = ed25519.Sign(h.private, digest[:])
	id, err := lxphost.DIDID(h.did)
	require.NoError(t, err)
	sequence, err := lxphost.NextSequence(h.ctx, h.a.GetKey(app.LayerXHostStoreKey))
	require.NoError(t, err)
	receipt, err := h.a.LayerXHost.Execute(lxphost.WithSequence(h.ctx, sequence), activity,
		lxphost.KernelAuthority{Actor: id, Principal: id, VerifiedKey: h.public})
	require.NoError(t, err)
	require.Zero(t, receipt.ResultCode, receipt.ResultCode.Error())
	require.NotEmpty(t, receipt.Encoded)
	return receipt
}

func (h *repairM1425Node) createMarket(t *testing.T) {
	t.Helper()
	id, err := lxphost.DIDID(h.did)
	require.NoError(t, err)
	salt := [32]byte{0x25}
	asset := sha256.Sum256(append(append([]byte("LX:ASSET:v1"), id[:]...), salt[:]...))
	registration, err := ledgertypes.EncodeRegisterPayload(ledgertypes.RegisterArgs{AssetID: asset, Salt: salt,
		Symbol: "USD", Name: "Oracle quote", Decimals: 6, SupplyCap: sdk.NewInt(1_000_000), IssuerKind: 2, CustodyReference: []byte("reserve")})
	require.NoError(t, err)
	h.execute(t, 0x00010001, registration)

	// Insurance is a genesis peer. The market and its three pool accounts are
	// created by the native production market-create handler, never store.Set.
	name := []byte("system:insurance")
	insurance, err := codec.DeriveAccountID(name)
	require.NoError(t, err)
	field := func(out, value []byte) []byte {
		return append(binary.BigEndian.AppendUint32(out, uint32(len(value))), value...)
	}
	record := field(nil, insurance[:])
	record = field(record, name)
	record = append(record, 9)
	record = append(record, make([]byte, 16)...)
	record = field(record, asset[:])
	record = append(record, 1)
	record = binary.BigEndian.AppendUint64(record, 0)
	record = binary.BigEndian.AppendUint64(record, 1)
	record = append(record, 0, 0)
	record = field(record, make([]byte, 32))
	record = append(record, 0)
	require.NoError(t, lxphost.InitStoreGenesis(h.ctx, h.a.GetKey(app.LayerXHostStoreKey), lxphost.StoreGenesis{
		Records: []kv.Pair{{Key: append([]byte{6}, insurance[:]...), Value: record}},
	}))

	market := make([]byte, 622)
	copy(market[:32], h.feed[:])
	copy(market[32:64], asset[:])
	copy(market[64:96], id[:])
	for i, name := range []string{"system:liquidity:" + hex.EncodeToString(h.feed[:]),
		"system:funding:" + hex.EncodeToString(h.feed[:]) + ":long",
		"system:funding:" + hex.EncodeToString(h.feed[:]) + ":short", "system:insurance"} {
		account, err := codec.DeriveAccountID([]byte(name))
		require.NoError(t, err)
		copy(market[96+i*32:128+i*32], account[:])
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
	key := append([]byte{1, 0, 6, 'm', 'a', 'r', 'k', 'e', 't', ':'}, h.feed[:]...)
	require.Nil(t, h.ctx.KVStore(h.a.GetKey(app.LayerXHostStoreKey)).Get(key))
	h.execute(t, 0x00060001, market)
	require.Equal(t, market, h.ctx.KVStore(h.a.GetKey(app.LayerXHostStoreKey)).Get(key))
}

func (h *repairM1425Node) committed(t *testing.T) oracle.OraclePrice {
	t.Helper()
	k, err := oracle.NewKeeper(h.a.LayerXHost, app.LayerXHostStoreKey, exchangetypes.StoreKey)
	require.NoError(t, err)
	price, found, err := k.CommittedOraclePrice(h.ctx, h.feed)
	require.NoError(t, err)
	require.True(t, found)
	return price
}

func (h *repairM1425Node) signedObservation(t *testing.T, sequence uint64, price *big.Int, confidence uint64) (lxphost.Activity, []byte) {
	t.Helper()
	identitySequence, err := h.a.LayerXHost.ReadIdentitySequence(h.ctx, h.did)
	require.NoError(t, err)
	now := uint64(h.ctx.BlockTime().UnixMilli())
	activity, err := oracle.SignObservation(h.a.LayerXHost, h.private, oracle.Observation{
		FeedID: h.feed, Price: price, Sequence: sequence, ObservedAt: now, SourceIdentifier: 1,
	}, identitySequence, now-1000, now+1000)
	require.NoError(t, err)
	input, err := oracle.PackSubmitPrice(h.feed, price, new(big.Int).SetUint64(confidence), activity)
	require.NoError(t, err)
	return activity, input
}

func (h *repairM1425Node) readPrice(t *testing.T, ctx sdk.Context, caller sdk.AccAddress, price *big.Int) {
	t.Helper()
	input, err := h.p.GetABI().Pack(oracle.GetValidatorPriceMethod, h.feed)
	require.NoError(t, err)
	to := h.p.Address()
	output, err := h.a.EvmKeeper.StaticCallEVM(ctx, caller, &to, input)
	require.NoError(t, err)
	values, err := h.p.GetABI().Unpack(oracle.GetValidatorPriceMethod, output)
	require.NoError(t, err)
	require.Equal(t, []interface{}{price, big.NewInt(1), big.NewInt(ctx.BlockTime().Unix())}, values)
}

func TestRepairM_1425_ProductionNode(t *testing.T) {
	h := newRepairM1425Node(t)
	activity, input := h.observation(t, 1, 1000)
	h.deliver(t, h.signer, h.p.Address(), input, true)
	h.read(t, h.ctx, h.other.AccountAddress, 1000)
	h.read(t, h.ctx.WithIsCheckTx(true), sdk.AccAddress(bytes.Repeat([]byte{0x42}, 20)), 1000)
	committed := h.committed(t)
	require.Equal(t, big.NewInt(1000), committed.Price.BigInt())
	require.Equal(t, uint64(h.ctx.BlockTime().UnixMilli()), committed.ObservedAt)
	require.Equal(t, uint64(1), committed.ObservationSequence)
	require.NotEqual(t, [32]byte{}, committed.SourceSetDigest)
	var recorded struct {
		Validator  sdk.ValAddress
		Confidence uint64
		Activity   lxphost.Activity
		Receipt    lxphost.Receipt
	}
	require.NoError(t, json.Unmarshal(h.ctx.KVStore(h.a.GetKey(exchangetypes.StoreKey)).Get(append([]byte("oracle/activity/"), activity.IdempotencyKey[:]...)), &recorded))
	require.Equal(t, activity, recorded.Activity)
	validator := sha256.Sum256(h.public[:])
	require.Equal(t, sdk.ValAddress(validator[:20]), recorded.Validator)
	require.Equal(t, uint64(5), recorded.Confidence)
	require.Zero(t, recorded.Receipt.ResultCode)
	require.NotEmpty(t, recorded.Receipt.Encoded)
	require.Equal(t, 1, h.auditCount(t))

	// Exercise the actual governance producer and registered router. Its
	// retirement refusal is an out-of-scope wiring request, not bypassed here.
	before := h.a.LayerXExchangeKeeper.GetParams(h.ctx)
	params := before
	params.Markets = append([]exchangetypes.Market(nil), before.Markets...)
	params.Markets = append(params.Markets, exchangetypes.Market{MarketId: hex.EncodeToString(h.feed[:]), MarginAssetId: hex.EncodeToString(h.feed[:]), Enabled: true})
	proposal, err := govtypes.NewLayerXProposal("Configure oracle market", "Configure a native oracle market through node governance",
		&exchangetypes.MsgUpdateParams{Authority: govtypes.GovernanceAuthority(), Params: params})
	require.NoError(t, err)
	require.NoError(t, proposal.ValidateBasic())
	handler := h.a.GovKeeper.Router().GetRoute(proposal.ProposalRoute())
	require.NotNil(t, handler)
	require.ErrorIs(t, handler(h.ctx, proposal), exchangetypes.ErrRetired)
	require.Equal(t, before, h.a.LayerXExchangeKeeper.GetParams(h.ctx))
	require.Equal(t, committed, h.committed(t))
}

func TestRepairM_1425_ReopenCommittedApplication(t *testing.T) {
	repairM1425RecoveryProof(t)
}

func repairM1425RecoveryProof(t *testing.T) {
	t.Helper()
	h := newRepairM1425Node(t)
	wire := &proofOracleHarness{node: h, p: h.p}
	price := new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 80), big.NewInt(57))
	activity, input := h.signedObservation(t, 1, price, 5)
	wire.refuse(t, h.ctx, h.signer.EvmAddress, oracle.GetValidatorPriceMethod, true, agentcommon.ErrOracleNoFeeds, h.feed)
	wire.refuse(t, h.ctx, h.signer.EvmAddress, oracle.GetValidatorPriceMethod, true, agentcommon.ErrOracleNoFeeds, [32]byte{0xff})
	// Signed transport is mandatory, and refusal must not create state.
	h.deliver(t, h.other, h.p.Address(), input, false)
	otherKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x46}, ed25519.SeedSize))
	h.bind(t, h.other, otherKey)
	h.register(t, otherKey)
	var otherPublic [32]byte
	copy(otherPublic[:], otherKey.Public().(ed25519.PublicKey))
	otherDID := []byte("did:layerx:" + hex.EncodeToString(otherPublic[:]))
	sequence, err := h.a.LayerXHost.ReadIdentitySequence(h.ctx, otherDID)
	require.NoError(t, err)
	now := uint64(h.ctx.BlockTime().UnixMilli())
	denied, err := oracle.SignObservation(h.a.LayerXHost, otherKey, oracle.Observation{FeedID: h.feed, Price: price, Sequence: 1, ObservedAt: now, SourceIdentifier: 1}, sequence, now-1000, now+1000)
	require.NoError(t, err)
	deniedInput, err := oracle.PackSubmitPrice(h.feed, price, big.NewInt(5), denied)
	require.NoError(t, err)
	output, callErr, cause := wire.cross(t, h.ctx, h.other.EvmAddress, deniedInput, false)
	require.ErrorIs(t, callErr, vm.ErrExecutionReverted)
	require.ErrorIs(t, cause, agentcommon.ErrOracleNotSigner)
	require.Contains(t, revertText(t, output), agentcommon.ErrOracleNotSigner.Error())
	for _, guard := range []struct {
		static, delegate bool
		code             agentcommon.ErrorCode
	}{
		{true, false, agentcommon.ErrReadOnly}, {false, true, agentcommon.ErrDelegatecall},
	} {
		db := state.NewDBImpl(h.ctx, &h.a.EvmKeeper, false)
		output, err := h.p.Run(&vm.EVM{StateDB: db}, h.signer.EvmAddress, h.p.Address(), input, nil, guard.static, guard.delegate, nil)
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		require.ErrorIs(t, db.GetPrecompileError(), guard.code)
		require.Contains(t, revertText(t, output), guard.code.Error())
	}
	require.Zero(t, h.auditCount(t))
	require.Len(t, input, 411)
	require.Equal(t, uint64(42330), h.p.RequiredGas(input))
	output, callErr, cause = wire.cross(t, h.ctx, h.signer.EvmAddress, input, false)
	require.NoError(t, callErr, "%s", output)
	require.NoError(t, cause)
	require.Empty(t, output)
	_, err = h.p.GetABI().Unpack(oracle.SubmitPriceMethod, output)
	require.NoError(t, err)
	var recorded struct {
		Validator  sdk.ValAddress
		Confidence uint64
		Activity   lxphost.Activity
		Receipt    lxphost.Receipt
	}
	require.NoError(t, json.Unmarshal(h.ctx.KVStore(h.a.GetKey(exchangetypes.StoreKey)).Get(append([]byte("oracle/activity/"), activity.IdempotencyKey[:]...)), &recorded))
	validator := sha256.Sum256(h.public[:])
	require.Equal(t, sdk.ValAddress(validator[:20]), recorded.Validator)
	require.Equal(t, uint64(5), recorded.Confidence)
	require.Equal(t, activity, recorded.Activity)
	require.Zero(t, recorded.Receipt.ResultCode)
	require.NotEmpty(t, recorded.Receipt.Encoded)
	h.readPrice(t, h.ctx, h.other.AccountAddress, price)
	before := h.committed(t)
	require.Equal(t, uint64(1), before.ObservationSequence)
	require.NotEqual(t, [32]byte{}, before.SourceSetDigest)
	previousApp, previousHost, previousPrecompile := h.a, h.a.LayerXHost, h.p
	header := h.ctx.BlockHeader()
	validatorsHash := sha256.Sum256(h.public[:])
	block := blocktypes.MakeBlock(header.Height, nil, &blocktypes.Commit{}, nil)
	block.ChainID, block.Time = header.ChainID, header.Time
	block.ValidatorsHash, block.NextValidatorsHash = validatorsHash[:], validatorsHash[:]
	block.ProposerAddress = validatorsHash[:20]
	require.NoError(t, block.Header.ValidateBasic())
	header = *block.Header.ToProto()
	blockHash := []byte(block.Hash())
	require.NotEmpty(t, blockHash)
	h.ctx = h.ctx.WithBlockHeader(header).WithHeaderHash(blockHash)
	require.NoError(t, h.a.SetSealedStateToCommit(h.ctx, h.ctx.MultiStore().(sdk.CacheMultiStore)))
	_, err = h.a.Commit(context.Background())
	require.NoError(t, err)
	commit := h.a.CommitMultiStore().LastCommitID()
	require.NotEmpty(t, commit.Hash)
	require.NoError(t, h.a.Close())
	h.a, h.p = nil, nil
	h.ctx = sdk.Context{}
	h.open(t, true)
	require.NotSame(t, previousApp, h.a)
	require.NotSame(t, previousHost, h.a.LayerXHost)
	require.Equal(t, commit, h.a.CommitMultiStore().LastCommitID())
	require.Equal(t, blockHash, h.a.LastBlockID().Hash)
	h.ctx = h.ctx.WithBlockHeader(header)
	h.a.RefreshPrecompiles(h.ctx)
	h.selectPrecompile(t)
	require.NotSame(t, previousPrecompile, h.p)
	wire.p = h.p
	// Ordinary public EVM reads happen before any subsequent observation or
	// replay binding, using the new application's registered dependencies.
	h.readPrice(t, h.ctx, h.other.AccountAddress, price)
	h.readPrice(t, h.ctx.WithIsCheckTx(true), sdk.AccAddress(bytes.Repeat([]byte{0x43}, 20)), price)
	wire.observe(t, h.ctx, oracle.GetValidatorPriceMethod, []interface{}{price, big.NewInt(1), big.NewInt(h.ctx.BlockTime().Unix())}, h.feed)
	require.Equal(t, uint64(1108), h.p.RequiredGas(wire.input(t, oracle.GetValidatorPriceMethod, h.feed)))
	require.Equal(t, before, h.committed(t), "price, millisecond timestamp, sequence and source identity survive reopening")
	require.Equal(t, 1, h.auditCount(t))
	nextPrice := new(big.Int).Add(price, big.NewInt(7))
	_, next := h.signedObservation(t, 2, nextPrice, 6)
	output, callErr, cause = wire.cross(t, h.ctx, h.signer.EvmAddress, next, false)
	require.NoError(t, callErr, "%s", output)
	require.NoError(t, cause)
	h.readPrice(t, h.ctx, h.other.AccountAddress, nextPrice)
	wire.observe(t, h.ctx, oracle.GetValidatorPriceMethod, []interface{}{nextPrice, big.NewInt(1), big.NewInt(h.ctx.BlockTime().Unix())}, h.feed)
	after := h.committed(t)
	require.Equal(t, nextPrice, after.Price.BigInt())
	require.Equal(t, uint64(2), after.ObservationSequence)
	require.Equal(t, before.ObservedAt, after.ObservedAt)
	require.Equal(t, before.SourceSetDigest, after.SourceSetDigest)
	require.Equal(t, 2, h.auditCount(t))

	// Preserve the original proof's native-decoder negative on producer state.
	key := append([]byte{1, 0, 6, 'o', 'r', 'a', 'c', 'l', 'e', ':'}, h.feed[:]...)
	corrupt, _ := h.ctx.CacheContext()
	produced := corrupt.KVStore(h.a.GetKey(app.LayerXHostStoreKey)).Get(key)
	require.NotEmpty(t, produced)
	corrupt.KVStore(h.a.GetKey(app.LayerXHostStoreKey)).Set(key, produced[:len(produced)-1])
	output, callErr, cause = wire.cross(t, corrupt, h.signer.EvmAddress, wire.input(t, oracle.GetValidatorPriceMethod, h.feed), true)
	require.ErrorIs(t, callErr, vm.ErrExecutionReverted)
	require.ErrorIs(t, cause, lxphost.Code(-3))
	require.Contains(t, revertText(t, output), "LXP_ERR_NON_CANONICAL")
	require.Equal(t, after, h.committed(t))
}

func TestRepairM_1425_WeightedMedianReference(t *testing.T) {
	h := newRepairM1425Node(t)
	for _, row := range []struct {
		name            string
		prices, weights []int64
	}{
		{"floor one of three", []int64{10, 30}, []int64{1, 2}},
		{"floor two of five", []int64{30, 10}, []int64{3, 2}},
		{"floor three of seven", []int64{10, 30}, []int64{3, 4}},
		{"several feeds reach floor", []int64{40, 20, 10}, []int64{4, 2, 1}},
		{"exact even half", []int64{30, 10}, []int64{2, 2}},
		{"zero weight", []int64{1, 30, 10}, []int64{0, 3, 2}},
	} {
		t.Run(row.name, func(t *testing.T) {
			var votes []int64
			feeds := make([]oracle.Feed, len(row.prices))
			minimum := int64(1 << 62)
			for i, price := range row.prices {
				feeds[i] = feed(price, row.weights[i])
				for weight := int64(0); weight < row.weights[i]; weight++ {
					votes = append(votes, price)
				}
				if row.weights[i] > 0 && row.weights[i] < minimum {
					minimum = row.weights[i]
				}
			}
			sort.Slice(votes, func(i, j int) bool { return votes[i] < votes[j] })
			want := big.NewInt(votes[(len(votes)-1)/2])
			before, err := json.Marshal(feeds)
			require.NoError(t, err)
			price, confidence, err := oracle.Aggregate(feeds, oracle.MergeMaxFeeds)
			require.NoError(t, err)
			require.Equal(t, want, price)
			require.Equal(t, big.NewInt(minimum), confidence)
			input, err := h.p.GetABI().Pack(oracle.AggregateMethod, feeds)
			require.NoError(t, err)
			to := h.p.Address()
			output, err := h.a.EvmKeeper.StaticCallEVM(h.ctx.WithIsCheckTx(true), h.other.AccountAddress, &to, input)
			require.NoError(t, err)
			values, err := h.p.GetABI().Unpack(oracle.AggregateMethod, output)
			require.NoError(t, err)
			require.Equal(t, []interface{}{want, big.NewInt(minimum)}, values)
			price.SetInt64(999)
			confidence.SetInt64(999)
			after, err := json.Marshal(feeds)
			require.NoError(t, err)
			require.Equal(t, before, after)
		})
	}
	t.Run("existing refusal contract", TestOracleAggregateRefusals)
	t.Run("arbitrary precision and immutability", TestRepair_197_WeightedMedian)
}

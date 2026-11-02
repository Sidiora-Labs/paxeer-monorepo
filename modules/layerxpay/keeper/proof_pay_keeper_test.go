// Proves: Keeper.SetParams and Keeper.Submit -> SDK KVStore and lxphost.HostExecute -> Querier.Params, Keeper.GetOffer and codec.DecodeReceipt
//
// NewKeeper and the real Keeper value connect all five pay-keeper exports to
// consumers. Identity registration, EVM binding, asset registration and service
// execution use the repository's producers; only genesis configuration is seeded.
// The host, database and keeper are stopped and reopened before reading results.
// Repairs: keeper.go NewKeeper accepted a missing EVM binding reader and later
// panicked in AuthorityFor; execute.go Submit discarded the typed unknown-activity
// code, preventing consumers from identifying that refusal with errors.Is.
// Existing corruption behavior outside scope is preserved: params.go GetParams
// and service_views.go GetOffer panic when their stored producer bytes cannot be
// decoded. Those consumers must refuse corruption rather than return defaults.
package keeper

import (
	"bytes"
	"crypto/sha256"
	"sync"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	transferkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/transfer/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	custodykeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/keeper"
	custodytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	ledgertypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	sdkcodec "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/kv"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

// This opens actual persistent stores, not a copy of the keeper's exported state.
func proofPayKeeperOpen(t *testing.T, directory string) (*payHarness, func()) {
	t.Helper()
	db, err := dbm.NewDB("pay-proof", dbm.GoLevelDBBackend, directory)
	require.NoError(t, err, "modules/layerxpay/keeper: open producer database")
	h := &payHarness{
		cdc:      sdkcodec.NewProtoCodec(codectypes.NewInterfaceRegistry()),
		storeKey: sdk.NewKVStoreKey(types.StoreKey),
		hostKey:  sdk.NewKVStoreKey("lxphost"),
	}
	evmKey := sdk.NewKVStoreKey(evmtypes.StoreKey)
	custodyKey := sdk.NewKVStoreKey(custodytypes.StoreKey)
	h.cms = store.NewCommitMultiStore(db)
	var once sync.Once
	closeProducer := func() {
		once.Do(func() {
			if h.host != nil {
				h.host.Close()
			}
			require.NoError(t, h.cms.Close())
		})
	}
	t.Cleanup(closeProducer)
	for _, key := range []*sdk.KVStoreKey{h.storeKey, h.hostKey, evmKey, custodyKey} {
		h.cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, nil)
	}
	require.NoError(t, h.cms.LoadLatestVersion())
	h.ctx = sdk.NewContext(h.cms, tmproto.Header{
		ChainID: "hyperpax_125-1", Height: 1, Time: time.UnixMilli(payHarnessTimeMs),
	}, false)
	h.host, err = lxphost.NewHost(lxphost.Config{
		StoreKey: h.hostKey, NetworkID: 7, ParameterVersion: 1,
		TimestampWindowMs: 60000, GasLimit: 1000000,
	})
	require.NoError(t, err, "modules/lxphost: start real kernel producer")
	subspace := paramtypes.NewSubspace(h.cdc, sdkcodec.NewLegacyAmino(), nil, nil, evmtypes.ModuleName)
	h.evm = evmkeeper.NewKeeper(evmKey, nil, subspace, nil, nil, nil, nil, transferkeeper.Keeper{}, nil, nil, nil)
	h.custody = custodykeeper.NewKeeper(h.cdc, custodyKey, nil, nil, nil, nil)
	h.keeper, err = NewKeeper(h.cdc, h.storeKey, h.hostKey, h.host, h.evm, h.custody, "pay-governance")
	require.NoError(t, err)
	return h, closeProducer
}

func TestProofPayKeeper(t *testing.T) {
	directory := t.TempDir()
	h, stop := proofPayKeeperOpen(t, directory)
	provider := h.registerActor(t, 1)
	other := h.registerActor(t, 2)
	require.Same(t, h.host, h.keeper.Host())
	require.Equal(t, "pay-governance", h.keeper.Authority())

	invalid, err := NewKeeper(h.cdc, h.storeKey, h.hostKey, h.host, nil, h.custody, h.keeper.Authority())
	require.EqualError(t, err, "layerxpay: missing EVM binding reader")
	require.Equal(t, Keeper{}, invalid)
	query := Querier{Keeper: h.keeper}
	for i := 0; i < 2; i++ {
		response, err := query.Params(sdk.WrapSDKContext(h.ctx), &types.QueryParamsRequest{})
		require.NoError(t, err)
		require.Equal(t, types.DefaultParams(), response.Params)
	}
	require.Empty(t, payStoreSnapshot(h.ctx, h.storeKey))

	gateKey, err := types.KernelModuleEnableKey(types.ServiceModuleID)
	require.NoError(t, err)
	gate := make([]byte, 32)
	gate[31] = 1
	require.NoError(t, h.keeper.InitGenesis(h.ctx, types.GenesisState{
		Params: types.DefaultParams(), KernelRecords: []kv.Pair{{Key: gateKey, Value: gate}},
	}))
	params := types.Params{ServiceEnabled: true}
	h.keeper.SetParams(h.ctx, params)
	require.Equal(t, params, h.keeper.GetParams(h.ctx))

	// Register the referenced asset through the real signed host activity.
	salt := [32]byte{0x82}
	assetID := sha256.Sum256(bytes.Join([][]byte{[]byte("LX:ASSET:v1"), provider.id[:], salt[:]}, nil))
	registration, err := ledgertypes.EncodeRegisterPayload(ledgertypes.RegisterArgs{
		AssetID: assetID, Salt: salt, Symbol: "PAY", Name: "Pay proof",
		Decimals: 6, SupplyCap: sdk.NewInt(1000), IssuerKind: 1,
	})
	require.NoError(t, err)
	paySubmitExecute(t, h, provider, 0x00010001, registration, [32]byte{0x83})
	last, err := h.host.ReadLastSequence(h.ctx, provider.did)
	require.NoError(t, err)
	identitySequence, err := h.host.ReadIdentitySequence(h.ctx, provider.did)
	require.NoError(t, err)
	moduleBefore := payStoreSnapshot(h.ctx, h.storeKey)
	id, terms, spec := [32]byte{0x84}, [32]byte{0x85}, [32]byte{0x86}
	price := codec.U128{Lo: 100}
	deadline, expiry := uint64(payHarnessTimeMs+60000), uint64(payHarnessTimeMs+120000)
	produced, err := h.keeper.ServiceOfferPublish(h.ctx, provider.address, id, assetID,
		price, terms, spec, deadline, 30000, 45000, 2, expiry)
	require.NoError(t, err)
	require.Zero(t, produced.ResultCode)
	require.Equal(t, last+1, produced.GlobalSequence)
	require.NotEqual(t, produced.PreviousStateRoot, produced.ResultingStateRoot)
	require.NotEmpty(t, produced.Encoded)
	require.Equal(t, moduleBefore, payStoreSnapshot(h.ctx, h.storeKey))
	_, principal, err := lxphost.MainAccountID(provider.public)
	require.NoError(t, err)
	want := types.ServiceOffer{
		OfferID: id, OfferingAgent: principal, AssetID: assetID, Price: price,
		TermsHash: terms, DeliverableSpecHash: spec, DeliveryDeadlineMs: deadline,
		AcceptanceWindowMs: 30000, DisputeWindowMs: 45000, DefaultOutcome: 2,
		OfferExpiryMs: expiry, GlobalSequence: produced.GlobalSequence,
	}
	for i := 0; i < 2; i++ {
		decoded, err := codec.DecodeReceipt(produced.Encoded)
		require.NoError(t, err)
		require.Equal(t, types.ServiceModuleID, decoded.ModuleID)
		require.Equal(t, produced.GlobalSequence, decoded.GlobalSequence)
		require.Equal(t, produced.ResultingStateRoot, decoded.ResultingStateRoot)
		require.Zero(t, decoded.ResultCode)
		require.NotEmpty(t, decoded.Effects)
		require.NotEqual(t, [32]byte{}, decoded.ActivityID)
		require.Equal(t, produced.Encoded, decoded.CanonicalBytes())
		want.ActivityID = decoded.ActivityID
	}
	sequence, err := h.host.ReadIdentitySequence(h.ctx, provider.did)
	require.NoError(t, err)
	require.Equal(t, identitySequence+1, sequence)
	producerState := payStoreSnapshot(h.ctx, h.hostKey)
	h.cms.Commit(true)
	stop()

	consumer, _ := proofPayKeeperOpen(t, directory)
	require.NotSame(t, h.host, consumer.host)
	require.NotSame(t, h.cms, consumer.cms)
	require.Equal(t, producerState, payStoreSnapshot(consumer.ctx, consumer.hostKey))
	require.Equal(t, moduleBefore, payStoreSnapshot(consumer.ctx, consumer.storeKey))
	query = Querier{Keeper: consumer.keeper}
	for i := 0; i < 2; i++ {
		response, err := query.Params(sdk.WrapSDKContext(consumer.ctx), &types.QueryParamsRequest{})
		require.NoError(t, err)
		require.Equal(t, params, response.Params)
		offer, found := consumer.keeper.GetOffer(consumer.ctx, id)
		require.True(t, found)
		require.Equal(t, want, offer)
	}

	// The reopened EVM binding and identity stores authorize the real caller.
	refused, err := consumer.keeper.ServiceOfferWithdraw(consumer.ctx, other.address, id)
	require.ErrorIs(t, err, lxphost.Code(-207))
	require.Equal(t, lxphost.Code(-207), refused.ResultCode)
	decodedRefusal, decodeErr := codec.DecodeReceipt(refused.Encoded)
	require.NoError(t, decodeErr)
	require.Equal(t, int32(-207), decodedRefusal.ResultCode)
	reason, decodeErr := abi.UnpackRevert(pcommon.LXPReason(err))
	require.NoError(t, decodeErr)
	require.Contains(t, reason, "LXP_UNAUTHORIZED_DEBIT")
	require.Equal(t, producerState, payStoreSnapshot(consumer.ctx, consumer.hostKey))
	require.Equal(t, moduleBefore, payStoreSnapshot(consumer.ctx, consumer.storeKey))
	_, err = consumer.keeper.Submit(consumer.ctx, common.Address{}, "serviceOfferWithdraw", nil, id)
	require.ErrorIs(t, err, types.ErrUnboundCaller)
	_, err = consumer.keeper.Submit(consumer.ctx, provider.address, "unknown-pay-method", nil, id)
	require.ErrorIs(t, err, lxphost.Code(-106))
	require.ErrorContains(t, err, "LXP_UNKNOWN_ACTIVITY")

	params.Paused = true
	consumer.keeper.SetParams(consumer.ctx, params)
	for i := 0; i < 2; i++ {
		response, err := query.Params(sdk.WrapSDKContext(consumer.ctx), &types.QueryParamsRequest{})
		require.NoError(t, err)
		require.Equal(t, params, response.Params)
	}
	_, err = consumer.keeper.ServiceOfferPublish(consumer.ctx, provider.address, [32]byte{0x87},
		assetID, price, terms, spec, deadline, 30000, 45000, 2, expiry)
	require.ErrorIs(t, err, govtypes.ErrPausedScope)
	require.Equal(t, producerState, payStoreSnapshot(consumer.ctx, consumer.hostKey))
	withdrawn, err := consumer.keeper.ServiceOfferWithdraw(consumer.ctx, provider.address, id)
	require.NoError(t, err)
	require.Equal(t, produced.GlobalSequence+1, withdrawn.GlobalSequence)
	require.NotEqual(t, produced.Encoded, withdrawn.Encoded)
	want.Withdrawn = true
	for i := 0; i < 2; i++ {
		offer, found := consumer.keeper.GetOffer(consumer.ctx, id)
		require.True(t, found)
		require.Equal(t, want, offer)
	}

	// Damage actual producer output only in discarded store branches.
	t.Run("malformed_producer_output", func(t *testing.T) {
		decoded, err := codec.DecodeReceipt(produced.Encoded[:len(produced.Encoded)-1])
		require.Error(t, err)
		require.Nil(t, decoded)
		branch, _ := consumer.ctx.CacheContext()
		raw := branch.KVStore(consumer.storeKey).Get(types.ParamsKey)
		require.NotEmpty(t, raw)
		branch.KVStore(consumer.storeKey).Set(types.ParamsKey, bytes.Clone(raw[:len(raw)-1]))
		require.Panics(t, func() {
			_, _ = query.Params(sdk.WrapSDKContext(branch), &types.QueryParamsRequest{})
		})
		branch, _ = consumer.ctx.CacheContext()
		key := types.KernelKVKey(types.ServiceModuleID, append([]byte("offer::1"), id[:]...))
		raw = branch.KVStore(consumer.hostKey).Get(key)
		require.Len(t, raw, 252, "src/modules/service must produce the stored offer")
		branch.KVStore(consumer.hostKey).Set(key, bytes.Clone(raw[:len(raw)-1]))
		require.Panics(t, func() { consumer.keeper.GetOffer(branch, id) })
		offer, found := consumer.keeper.GetOffer(consumer.ctx, id)
		require.True(t, found)
		require.Equal(t, want, offer)
	})
}

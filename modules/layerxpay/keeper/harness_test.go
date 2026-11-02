package keeper

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	transferkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/interchain/modules/apps/transfer/keeper"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	custodykeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/keeper"
	custodytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

const payHarnessTimeMs int64 = 1700000000000

type payActor struct {
	private ed25519.PrivateKey
	public  [32]byte
	did     []byte
	id      [32]byte
	address common.Address
}

func (a payActor) authority() lxphost.KernelAuthority {
	return lxphost.KernelAuthority{Actor: a.id, Principal: a.id, VerifiedKey: a.public}
}

type payHarness struct {
	ctx      sdk.Context
	cms      sdk.CommitMultiStore
	cdc      codec.BinaryCodec
	storeKey *sdk.KVStoreKey
	hostKey  *sdk.KVStoreKey
	host     *lxphost.Host
	evm      *evmkeeper.Keeper
	custody  *custodykeeper.Keeper
	keeper   Keeper
	actors   []payActor
	sequence uint64
}

func newPayHarness(t *testing.T) *payHarness {
	t.Helper()
	h := &payHarness{
		cdc:      codec.NewProtoCodec(codectypes.NewInterfaceRegistry()),
		storeKey: sdk.NewKVStoreKey(types.StoreKey),
		hostKey:  sdk.NewKVStoreKey("lxphost"),
	}
	evmKey := sdk.NewKVStoreKey(evmtypes.StoreKey)
	custodyKey := sdk.NewKVStoreKey(custodytypes.StoreKey)
	db := dbm.NewMemDB()
	h.cms = store.NewCommitMultiStore(db)
	for _, key := range []*sdk.KVStoreKey{h.storeKey, h.hostKey, evmKey, custodyKey} {
		h.cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	}
	require.NoError(t, h.cms.LoadLatestVersion())
	h.ctx = sdk.NewContext(h.cms, tmproto.Header{
		ChainID: "hyperpax_125-1", Height: 1, Time: time.UnixMilli(payHarnessTimeMs),
	}, false)
	var err error
	h.host, err = lxphost.NewHost(lxphost.Config{
		StoreKey: h.hostKey, NetworkID: 7, ParameterVersion: 1,
		TimestampWindowMs: 60000, GasLimit: 1000000,
	})
	require.NoError(t, err)
	t.Cleanup(h.host.Close)
	subspace := paramtypes.NewSubspace(h.cdc, codec.NewLegacyAmino(), nil, nil, evmtypes.ModuleName)
	h.evm = evmkeeper.NewKeeper(evmKey, nil, subspace, nil, nil, nil, nil, transferkeeper.Keeper{}, nil, nil, nil)
	h.custody = custodykeeper.NewKeeper(h.cdc, custodyKey, nil, nil, nil, nil)
	h.keeper, err = NewKeeper(h.cdc, h.storeKey, h.hostKey, h.host, h.evm, h.custody, "pay-governance")
	require.NoError(t, err)
	h.actors = []payActor{h.registerActor(t, 1), h.registerActor(t, 2)}
	return h
}

func (h *payHarness) registerActor(t *testing.T, seed byte) payActor {
	t.Helper()
	a := payActor{private: ed25519.NewKeyFromSeed(bytes.Repeat([]byte{seed}, ed25519.SeedSize))}
	copy(a.public[:], a.private.Public().(ed25519.PublicKey))
	a.did = []byte(evmtypes.LayerXDid(a.public))
	a.address = common.BytesToAddress([]byte{seed})
	var err error
	a.id, err = lxphost.DIDID(a.did)
	require.NoError(t, err)
	message, err := evmtypes.LayerXBindMessage(h.evm.ChainID(h.ctx), a.address, 0)
	require.NoError(t, err)
	var signature [64]byte
	copy(signature[:], ed25519.Sign(a.private, message))
	nonce, err := h.evm.BindLayerX(h.ctx, a.address, a.public, signature)
	require.NoError(t, err)
	require.Zero(t, nonce)
	bound, found := h.keeper.evm.GetLayerXDid(h.ctx, a.address)
	require.True(t, found)
	require.Equal(t, a.public, bound)
	payload := append([]byte{0x71, 1, 0, 2}, a.id[:]...)
	payload = append(payload, a.public[:]...)
	activity := lxphost.Activity{
		ProtocolVersion: 3, Type: 0x00070001, ActorDID: a.did, AuthorityKey: a.public,
		AccountSequence: 0, NotBefore: uint64(payHarnessTimeMs - 1000), NotAfter: uint64(payHarnessTimeMs + 1000),
		IdempotencyKey: sha256.Sum256(a.did), Payload: payload,
	}
	hash, err := h.host.SigningHash(activity)
	require.NoError(t, err)
	activity.Signature = ed25519.Sign(a.private, hash[:])
	h.sequence++
	h.ctx = lxphost.WithSequence(h.ctx, h.sequence)
	receipt, err := h.keeper.Host().Execute(h.ctx, activity, a.authority())
	require.NoError(t, err)
	require.Equal(t, lxphost.Code(0), receipt.ResultCode)
	require.Equal(t, h.sequence, receipt.GlobalSequence)
	require.NotEqual(t, receipt.PreviousStateRoot, receipt.ResultingStateRoot)
	identity, err := h.host.ReadIdentityState(h.ctx, a.did)
	require.NoError(t, err)
	require.Equal(t, a.public, identity.PrimaryKey)
	return a
}

func payStoreSnapshot(ctx sdk.Context, key sdk.StoreKey) map[string][]byte {
	values := make(map[string][]byte)
	iterator := ctx.KVStore(key).Iterator(nil, nil)
	defer iterator.Close()
	for ; iterator.Valid(); iterator.Next() {
		values[string(iterator.Key())] = bytes.Clone(iterator.Value())
	}
	return values
}

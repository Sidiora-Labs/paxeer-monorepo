package keeper

import (
	"crypto/ed25519"
	"encoding/binary"
	"encoding/hex"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	custodytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

// These fixtures expose real persistent keepers to external ABI consumers.
type RepairM1304World struct {
	Keeper                      *Keeper
	Context                     sdk.Context
	HostKey, PayKey             sdk.StoreKey
	Provider, Buyer             common.Address
	Agreement, Commitment, Spec [32]byte
	Token                       common.Address
	Asset                       [32]byte
	directory                   string
	harness                     *payHarness
	stop                        func()
}

func OpenRepairM1304World(t *testing.T, streams bool) *RepairM1304World {
	t.Helper()
	directory := t.TempDir()
	h, stop := proofPayKeeperOpen(t, directory)
	h.actors = []payActor{h.registerActor(t, 1), h.registerActor(t, 2)}
	w := &RepairM1304World{Keeper: &h.keeper, Context: h.ctx, HostKey: h.hostKey, PayKey: h.storeKey,
		Provider: h.actors[0].address, Buyer: h.actors[1].address, Agreement: [32]byte{0x82}, Commitment: [32]byte{0x83}, Spec: [32]byte{0x85},
		directory: directory, harness: h, stop: stop}
	if streams {
		args := payEscrowFundedArgs(t, h)
		w.Asset = args.AssetID
		w.Token = common.HexToAddress("0x1304")
		require.NoError(t, h.custody.SetAsset(h.ctx, custodytypes.AssetMapping{AssetId: hex.EncodeToString(w.Asset[:]), Denom: "repair1304", Pointer: w.Token.Hex(), Enabled: true}))
		h.keeper.SetParams(h.ctx, types.Params{StreamEnabled: true})
		gate, err := types.KernelModuleEnableKey(types.StreamModuleID)
		require.NoError(t, err)
		h.ctx.KVStore(h.hostKey).Set(gate, payGateEnabledRecord())
		w.Context = h.ctx.WithEVMTxHash(common.HexToHash("0x1304"))
		return w
	}
	h.keeper.SetParams(h.ctx, types.Params{ServiceEnabled: true})
	gate, err := types.KernelModuleEnableKey(types.ServiceModuleID)
	require.NoError(t, err)
	h.ctx.KVStore(h.hostKey).Set(gate, payGateEnabledRecord())
	_, err = h.keeper.ServiceOfferPublish(h.ctx, w.Provider, [32]byte{0x81}, [32]byte{0x86}, codec.U128{Lo: 50}, [32]byte{0x84}, w.Spec,
		uint64(payHarnessTimeMs)+100000, 20000, 30000, 1, uint64(payHarnessTimeMs)+90000)
	require.NoError(t, err)
	_, err = h.keeper.ServiceAgreementPropose(h.ctx, w.Buyer, w.Agreement, [32]byte{0x81}, [32]byte{0x84}, [32]byte{0x87}, [32]byte{2})
	require.NoError(t, err)
	_, err = h.keeper.ServiceAgreementAccept(h.ctx, w.Provider, w.Agreement, [32]byte{3})
	require.NoError(t, err)
	_, err = h.keeper.ServiceCommitTask(h.ctx, w.Provider, w.Commitment, w.Agreement, [32]byte{0x88}, [32]byte{}, uint64(payHarnessTimeMs)+100000, 1000, [32]byte{4})
	require.NoError(t, err)
	return w
}

func (w *RepairM1304World) Reopen(t *testing.T) {
	t.Helper()
	at, hash := w.Context.BlockTime(), w.Context.EVMTxHash()
	w.harness.cms.Commit(true)
	w.stop()
	h, stop := proofPayKeeperOpen(t, w.directory)
	w.harness, w.stop, w.Keeper = h, stop, &h.keeper
	w.Context = h.ctx.WithBlockTime(at).WithEVMTxHash(hash)
	w.HostKey, w.PayKey = h.hostKey, h.storeKey
}

func (w *RepairM1304World) Advance(seconds int64) {
	w.Context = w.Context.WithBlockTime(time.UnixMilli(payHarnessTimeMs + seconds*1000))
}

func ProveRepairM1304AdditionalServiceRecords(t *testing.T) {
	t.Helper()
	// Tool execution uses the production signature path, not a hand-built stored record.
	h, agreement, commitment, _ := serviceDeliveryHarness(t)
	provider := h.actors[0]
	_, principal, err := lxphost.MainAccountID(provider.public)
	require.NoError(t, err)
	execution := make([]byte, types.ServiceExecutionBytes)
	for i, field := range [][32]byte{{0xc1}, {0xc2}, agreement, commitment, {0xc3}, {0xc4}, {0xc5}} {
		copy(execution[32*i:32*(i+1)], field[:])
	}
	binary.BigEndian.PutUint64(execution[224:232], uint64(payHarnessTimeMs)-100)
	binary.BigEndian.PutUint64(execution[232:240], uint64(payHarnessTimeMs))
	binary.BigEndian.PutUint64(execution[240:248], 100)
	copy(execution[248:280], principal[:])
	reference := [32]byte{0xc6}
	copy(execution[280:312], reference[:])
	copy(execution[312:344], provider.public[:])
	message := append([]byte("LXP:SERVICE:ATTEST:v1"), execution[:344]...)
	digest, err := codec.DomainHash(codec.DomainSignaturePreimage, message)
	require.NoError(t, err)
	copy(execution[344:408], ed25519.Sign(provider.private, digest[:]))
	_, err = h.keeper.ServiceToolExecAttest(h.ctx, provider.address, execution, [32]byte{0xc2})
	require.NoError(t, err)
	got, found, err := h.keeper.ServiceViews().GetExecution(h.ctx, [32]byte{0xc1})
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, message, got.CanonicalPayload[:got.CanonicalPayloadLength])
	key := serviceKey("execut:1", [32]byte{0xc1})
	h.ctx.KVStore(h.hostKey).Set(key, []byte{1})
	before := payStoreSnapshot(h.ctx, h.hostKey)
	_, found, err = h.keeper.ServiceViews().GetExecution(h.ctx, [32]byte{0xc1})
	require.False(t, found)
	require.ErrorIs(t, err, ErrServiceCorruptState)
	require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))
	h.ctx.KVStore(h.hostKey).Delete(key)
	_, found, err = h.keeper.ServiceViews().GetExecution(h.ctx, [32]byte{0xc1})
	require.False(t, found)
	require.NoError(t, err)

	// The committed escrow fixture is the dispute peer; the dispute writer itself
	// runs its real authority, linkage and kernel paths.
	h, _, _, agreement, dispute := serviceDisputeSetup(t, true, false)
	receipt, err := h.keeper.ServiceDisputeOpen(h.ctx, h.actors[0].address, dispute, agreement, [][32]byte{{0xd1}})
	require.NoError(t, err)
	record, found, err := h.keeper.ServiceViews().GetDispute(h.ctx, dispute)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, receipt.GlobalSequence, record.GlobalSequence)
	key = serviceKey("disput:1", dispute)
	h.ctx.KVStore(h.hostKey).Set(key, []byte{1})
	before = payStoreSnapshot(h.ctx, h.hostKey)
	_, found, err = h.keeper.ServiceViews().GetDispute(h.ctx, dispute)
	require.False(t, found)
	require.ErrorIs(t, err, ErrServiceCorruptState)
	require.Equal(t, before, payStoreSnapshot(h.ctx, h.hostKey))
	h.ctx.KVStore(h.hostKey).Delete(key)
	_, found, err = h.keeper.ServiceViews().GetDispute(h.ctx, dispute)
	require.False(t, found)
	require.NoError(t, err)
}

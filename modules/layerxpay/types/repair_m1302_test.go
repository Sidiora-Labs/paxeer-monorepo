package types_test

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math/big"
	"testing"

	custodytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/kv"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func repairM1302Snapshot(h *proofViewsHarness) map[string]map[string][]byte {
	snapshot := make(map[string]map[string][]byte)
	for _, key := range []*sdk.KVStoreKey{h.key, h.payKey, h.evmKey, h.custodyKey} {
		records := make(map[string][]byte)
		it := h.ctx.KVStore(key).Iterator(nil, nil)
		for ; it.Valid(); it.Next() {
			records[string(it.Key())] = bytes.Clone(it.Value())
		}
		_ = it.Close()
		snapshot[key.Name()] = records
	}
	return snapshot
}

func TestRepairM_1302_OpaqueGenesis(t *testing.T) {
	proofViewsRun(t, func(producer *proofViewsHarness) {
		escrow, found, err := producer.views.GetEscrow(producer.ctx, [32]byte{20})
		require.NoError(t, err)
		require.True(t, found)
		token := common.HexToAddress("0x1234")
		require.NoError(t, producer.custody.SetAsset(producer.ctx, custodytypes.AssetMapping{
			AssetId: custodytypes.Hash32(escrow.AssetID), Denom: "repaircoin", Pointer: token.Hex(), Enabled: true,
		}))
		start := proofViewsTime / 1000
		producer.ctx = producer.ctx.WithEVMTxHash(common.HexToHash("0x1302"))
		streamID, err := producer.views.OpenAgentStream(producer.ctx, common.BytesToAddress([]byte{1}), common.BytesToAddress([]byte{2}), token,
			big.NewInt(1), start, start+60, big.NewInt(10))
		require.NoError(t, err)
		_, _, err = producer.views.CloseAgentStream(producer.ctx, common.BytesToAddress([]byte{1}), streamID)
		require.NoError(t, err)
		var streamKeyID [32]byte
		streamID.FillBytes(streamKeyID[:])
		wantAgent, found, err := producer.views.AgentStreamView(producer.ctx, streamID)
		require.NoError(t, err)
		require.True(t, found)
		exported := producer.views.ExportGenesis(producer.ctx)
		require.NoError(t, exported.Validate())
		wire, err := json.Marshal(exported)
		require.NoError(t, err)
		clone := func() types.GenesisState {
			var g types.GenesisState
			require.NoError(t, json.Unmarshal(wire, &g))
			return g
		}
		destination := &proofViewsHarness{db: dbm.NewMemDB(), key: sdk.NewKVStoreKey("lxphost"), payKey: sdk.NewKVStoreKey(types.StoreKey), evmKey: sdk.NewKVStoreKey("evm")}
		destination.open(t)
		defer func() { destination.host.Close(); require.NoError(t, destination.db.Close()) }()
		destination.views.SetParams(destination.ctx, types.DefaultParams())
		destination.ctx.KVStore(destination.key).Set([]byte("existing-host"), []byte("preserve"))
		destination.ctx.KVStore(destination.payKey).Set([]byte("existing-pay"), []byte("preserve"))
		before := repairM1302Snapshot(destination)
		record := func(g *types.GenesisState, module uint16, prefix string) *kv.Pair {
			keyPrefix := types.KernelKVKey(module, []byte(prefix))
			for i := range g.KernelRecords {
				if bytes.HasPrefix(g.KernelRecords[i].Key, keyPrefix) {
					return &g.KernelRecords[i]
				}
			}
			t.Fatalf("real export is missing %d/%s", module, prefix)
			return nil
		}
		refuse := func(t *testing.T, g types.GenesisState) {
			t.Helper()
			input, err := json.Marshal(g)
			require.NoError(t, err)
			require.ErrorIs(t, g.Validate(), types.ErrInvalidGenesis)
			require.ErrorIs(t, destination.views.InitGenesis(destination.ctx, g), types.ErrInvalidGenesis)
			require.Equal(t, before, repairM1302Snapshot(destination), "import changed stores before refusing the snapshot")
			after, err := json.Marshal(g)
			require.NoError(t, err)
			require.Equal(t, input, after, "validation changed the recovery input")
		}
		for _, family := range []struct {
			name                    string
			module                  uint16
			prefix                  string
			idOffset, versionOffset int
		}{
			{"escrow", 2, "hold:", 0, -1}, {"escrow_result", 2, "result:", -1, 243},
			{"budget", 3, "budget:", 2, 1}, {"stream", 4, "stream:", 0, -1}, {"stream_result", 4, "result:", -1, -1},
			{"offer", 5, "offer::1", 1, 0}, {"agreement", 5, "agree::1", 1, 0}, {"commitment", 5, "commit:1", 1, 0},
			{"execution", 5, "execut:1", 1, 0}, {"delivery", 5, "delivr:1", 1, 0},
			{"dispute", 5, "disput:1", 1, 0}, {"progress", 5, "progrs:1", 1, 0},
		} {
			t.Run(family.name, func(t *testing.T) {
				for _, damage := range []string{"truncated", "trailing", "key_length", "zero_key_id", "body_id", "version"} {
					// Unversioned records and replay-key records have no such field to corrupt.
					if damage == "version" && family.versionOffset < 0 || damage == "body_id" && family.idOffset < 0 {
						continue
					}
					t.Run(damage, func(t *testing.T) {
						g := clone()
						r := record(&g, family.module, family.prefix)
						switch damage {
						case "truncated":
							r.Value = r.Value[:1]
						case "trailing":
							r.Value = append(r.Value, 0)
						case "key_length":
							r.Key = append(r.Key, 0)
						case "zero_key_id":
							clear(r.Key[len(r.Key)-32:])
						case "body_id":
							r.Value[family.idOffset] ^= 0x80
						case "version":
							r.Value[family.versionOffset] = 0xff
						}
						refuse(t, g)
					})
				}
			})
		}
		for _, damage := range []string{"missing_item", "orphan_item", "item_index", "item_version", "item_length", "header_count", "header_version_key", "metadata_orphan", "metadata_mode", "metadata_rate_unit", "metadata_zero_payer", "metadata_same_participants", "result_without_hold", "result_without_stream"} {
			t.Run(damage, func(t *testing.T) {
				g := clone()
				remove := func(key []byte) {
					for i, r := range g.KernelRecords {
						if bytes.Equal(key, r.Key) {
							g.KernelRecords = append(g.KernelRecords[:i], g.KernelRecords[i+1:]...)
							return
						}
					}
					t.Fatal("missing removal target")
				}
				switch damage {
				case "missing_item":
					remove(bytes.Clone(record(&g, 5, "delitm:1").Key))
				case "orphan_item":
					remove(bytes.Clone(record(&g, 5, "delivr:1").Key))
				case "item_index":
					r := record(&g, 5, "delitm:1")
					r.Key[len(r.Key)-1] = 1
				case "item_version":
					record(&g, 5, "delitm:1").Value[0] = 2
				case "item_length":
					r := record(&g, 5, "delitm:1")
					r.Value = r.Value[:72]
				case "header_count":
					record(&g, 5, "delivr:1").Value[129] = 2
				case "header_version_key":
					record(&g, 5, "delivr:1").Key[10] = '2'
				case "metadata_orphan":
					g.AgentStreamMetadata[0].Key[len(g.AgentStreamMetadata[0].Key)-1] ^= 1
				case "metadata_mode":
					r := record(&g, 4, "stream:")
					r.Value[160] = types.StreamModeMetered
				case "metadata_rate_unit":
					binary.BigEndian.PutUint64(record(&g, 4, "stream:").Value[177:185], 1)
				case "metadata_zero_payer":
					clear(g.AgentStreamMetadata[0].Value[:20])
				case "metadata_same_participants":
					copy(g.AgentStreamMetadata[0].Value[20:40], g.AgentStreamMetadata[0].Value[:20])
				case "result_without_hold":
					remove(bytes.Clone(record(&g, 2, "hold:").Key))
				case "result_without_stream":
					remove(bytes.Clone(record(&g, 4, "stream:").Key))
				}
				refuse(t, g)
			})
		}
		require.NoError(t, destination.views.InitGenesis(destination.ctx, clone()))
		require.Equal(t, exported, destination.views.ExportGenesis(destination.ctx))
		destination.cms.Commit(true)
		destination.host.Close()
		destination.open(t)
		for read := 0; read < 2; read++ {
			require.Equal(t, exported, destination.views.ExportGenesis(destination.ctx))
			agent, found, err := destination.views.AgentStreamView(destination.ctx, streamID)
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, wantAgent, agent)
			stream, err := destination.views.GetStream(destination.ctx, streamKeyID)
			require.NoError(t, err)
			require.Equal(t, streamKeyID, stream.StreamID)
			offer, found, err := destination.views.ServiceViews().GetOffer(destination.ctx, [32]byte{30})
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, [32]byte{30}, offer.OfferID)
			delivery, found, err := destination.views.ServiceViews().GetDelivery(destination.ctx, [32]byte{47})
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, []types.ServiceDeliverable{{Hash: [32]byte{23}, ArtifactSize: 4096, AvailabilityReference: [32]byte{43}}}, delivery.Deliverables)
			got, found, err := destination.views.GetEscrow(destination.ctx, [32]byte{20})
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, escrow, got)
			budget, found, err := destination.views.GetBudget(destination.ctx, [32]byte{21})
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, [32]byte{21}, budget.BudgetID)
			agreement, found, err := destination.views.ServiceViews().GetAgreement(destination.ctx, [32]byte{31})
			require.NoError(t, err)
			require.True(t, found)
			wantAgreement, found, err := producer.views.ServiceViews().GetAgreement(producer.ctx, [32]byte{31})
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, wantAgreement, agreement)
			commitment, found, err := destination.views.ServiceViews().GetCommitment(destination.ctx, [32]byte{32})
			require.NoError(t, err)
			require.True(t, found)
			wantCommitment, found, err := producer.views.ServiceViews().GetCommitment(producer.ctx, [32]byte{32})
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, wantCommitment, commitment)
			execution, found, err := destination.views.ServiceViews().GetExecution(destination.ctx, [32]byte{38})
			require.NoError(t, err)
			require.True(t, found)
			wantExecution, found, err := producer.views.ServiceViews().GetExecution(producer.ctx, [32]byte{38})
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, wantExecution, execution)
			dispute, found, err := destination.views.ServiceViews().GetDispute(destination.ctx, [32]byte{50})
			require.NoError(t, err)
			require.True(t, found)
			wantDispute, found, err := producer.views.ServiceViews().GetDispute(producer.ctx, [32]byte{50})
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, wantDispute, dispute)
			progress, found, err := destination.views.ServiceViews().GetProgress(destination.ctx, [32]byte{44})
			require.NoError(t, err)
			require.True(t, found)
			wantProgress, found, err := producer.views.ServiceViews().GetProgress(producer.ctx, [32]byte{44})
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, wantProgress, progress)
			result, found, err := destination.views.GetEscrowResult(destination.ctx, [32]byte{20})
			require.NoError(t, err)
			require.True(t, found)
			wantResult, found, err := producer.views.GetEscrowResult(producer.ctx, [32]byte{20})
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, wantResult, result)
			require.Equal(t, []byte("preserve"), destination.ctx.KVStore(destination.key).Get([]byte("existing-host")))
			require.Equal(t, []byte("preserve"), destination.ctx.KVStore(destination.payKey).Get([]byte("existing-pay")))
		}
		authority, err := producer.views.AuthorityFor(producer.ctx, common.BytesToAddress([]byte{1}))
		require.NoError(t, err)
		producer.sequence, err = producer.host.ReadLastSequence(producer.ctx, authority.ActorDID)
		require.NoError(t, err)
	})
}

func TestRepairM_1302_EscrowRootProof(t *testing.T) {
	proofViewsRun(t, nil)
}

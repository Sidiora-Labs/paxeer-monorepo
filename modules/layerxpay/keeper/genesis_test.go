package keeper

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math/big"
	"sort"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/kv"
	"github.com/stretchr/testify/require"
)

func TestPayGenesisKeepsAgentStreamMetadata(t *testing.T) {
	for _, kind := range []string{"native", "pointer"} {
		t.Run(kind, func(t *testing.T) {
			h, _, token := newAgentStreamHarness(t, kind)
			payer, payee := h.actors[0].address, h.actors[1].address
			start := uint64(payHarnessTimeMs / 1000)
			id, err := h.keeper.OpenAgentStream(h.ctx, payer, payee, token, big.NewInt(2), start, start+100, big.NewInt(50))
			require.NoError(t, err)
			before, found, err := h.keeper.AgentStreamView(h.ctx, id)
			require.NoError(t, err)
			require.True(t, found)
			exported := h.keeper.ExportGenesis(h.ctx)
			require.NoError(t, exported.Validate())
			var streamID [32]byte
			id.FillBytes(streamID[:])
			metadataKey := agentStreamMetadataKey(streamID)
			metadataValue := append(append(append([]byte{}, payer.Bytes()...), payee.Bytes()...), token.Bytes()...)
			require.Equal(t, []kv.Pair{{Key: metadataKey, Value: metadataValue}}, exported.AgentStreamMetadata)

			cloned := h.keeper.ExportGenesis(h.ctx)
			cloned.AgentStreamMetadata[0].Key[0] ^= 0xff
			cloned.AgentStreamMetadata[0].Value[0] ^= 0xff
			require.Equal(t, metadataValue, h.ctx.KVStore(h.storeKey).Get(metadataKey))
			require.Equal(t, exported, h.keeper.ExportGenesis(h.ctx))

			h.ctx.KVStore(h.storeKey).Delete(metadataKey)
			_, _, err = h.keeper.AgentStreamView(h.ctx, id)
			require.Error(t, err)
			removed := 0
			for _, record := range exported.KernelRecords {
				if bytes.HasPrefix(record.Key, types.StreamKVPrefix) {
					h.ctx.KVStore(h.hostKey).Delete(record.Key)
					removed++
				}
			}
			require.Positive(t, removed)
			_, found, err = h.keeper.AgentStreamView(h.ctx, id)
			require.NoError(t, err)
			require.False(t, found)
			require.NoError(t, h.keeper.InitGenesis(h.ctx, *exported))
			require.Equal(t, exported, h.keeper.ExportGenesis(h.ctx))
			require.Equal(t, metadataValue, h.ctx.KVStore(h.storeKey).Get(metadataKey))
			after, found, err := h.keeper.AgentStreamView(h.ctx, id)
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, before, after)
			require.Equal(t, payer, after.Payer)
			require.Equal(t, payee, after.Payee)
			require.Equal(t, token, after.Token)

			h.ctx = h.ctx.WithBlockHeight(2).WithBlockTime(time.UnixMilli(payHarnessTimeMs + 5000))
			paid, err := h.keeper.SettleAgentStream(h.ctx, payee, id)
			require.NoError(t, err)
			require.Equal(t, big.NewInt(10), paid)
			after, found, err = h.keeper.AgentStreamView(h.ctx, id)
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, paid, after.Settled)
		})
	}
}

func TestPayGenesisRefusesMalformedAgentStreamMetadata(t *testing.T) {
	h, _, token := newAgentStreamHarness(t, "native")
	start := uint64(payHarnessTimeMs / 1000)
	id, err := h.keeper.OpenAgentStream(h.ctx, h.actors[0].address, h.actors[1].address, token, big.NewInt(2), start, start+100, big.NewInt(50))
	require.NoError(t, err)
	gs := h.keeper.ExportGenesis(h.ctx)
	require.NoError(t, ValidateOwnedGenesis(*gs))
	require.Len(t, gs.AgentStreamMetadata, 1)
	valid := gs.AgentStreamMetadata[0]
	key, value := valid.Key, valid.Value
	var streamID [32]byte
	id.FillBytes(streamID[:])
	streamKey := types.KernelKVKey(types.StreamModuleID, append([]byte("stream:"), streamID[:]...))
	streamBody := bytes.Clone(h.ctx.KVStore(h.hostKey).Get(streamKey))
	wrongPrefix := bytes.Clone(key)
	wrongPrefix[0] ^= 0xff
	for _, tc := range []struct {
		name    string
		records []kv.Pair
		detail  string
	}{
		{"short_value", []kv.Pair{{Key: key, Value: value[:59]}}, "agent stream metadata 0 has an invalid value length"},
		{"long_value", []kv.Pair{{Key: key, Value: append(bytes.Clone(value), 1)}}, "agent stream metadata 0 has an invalid value length"},
		{"short_key", []kv.Pair{{Key: key[:len(key)-1], Value: value}}, "agent stream metadata 0 has an invalid key"},
		{"long_key", []kv.Pair{{Key: append(bytes.Clone(key), 1), Value: value}}, "agent stream metadata 0 has an invalid key"},
		{"wrong_prefix", []kv.Pair{{Key: wrongPrefix, Value: value}}, "agent stream metadata 0 has an invalid key"},
		{"zero_id", []kv.Pair{{Key: append([]byte(types.AgentStreamMetadataPrefix), make([]byte, 32)...), Value: value}}, "agent stream metadata 0 has a zero stream id"},
		{"duplicate_key", []kv.Pair{valid, {Key: bytes.Clone(key), Value: make([]byte, 60)}}, "duplicate agent stream metadata 1"},
		{"zero_payer", []kv.Pair{{Key: key, Value: append(make([]byte, 20), value[20:]...)}}, "agent stream metadata 0 has invalid participants"},
		{"zero_payee", []kv.Pair{{Key: key, Value: append(append(bytes.Clone(value[:20]), make([]byte, 20)...), value[40:]...)}}, "agent stream metadata 0 has invalid participants"},
		{"same_participants", []kv.Pair{{Key: key, Value: append(append(bytes.Clone(value[:20]), value[:20]...), value[40:]...)}}, "agent stream metadata 0 has invalid participants"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			invalid := *gs
			invalid.AgentStreamMetadata = tc.records
			require.ErrorIs(t, invalid.Validate(), types.ErrInvalidGenesis)
			require.EqualError(t, invalid.Validate(), tc.detail+": invalid genesis")
			moduleBefore := payStoreSnapshot(h.ctx, h.storeKey)
			hostBefore := payStoreSnapshot(h.ctx, h.hostKey)
			require.ErrorIs(t, h.keeper.InitGenesis(h.ctx, invalid), types.ErrInvalidGenesis)
			require.Equal(t, moduleBefore, payStoreSnapshot(h.ctx, h.storeKey))
			require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
		})
	}
	t.Run("missing_stream", func(t *testing.T) {
		invalid := *types.DefaultGenesis()
		invalid.AgentStreamMetadata = []kv.Pair{valid}
		require.ErrorIs(t, invalid.Validate(), types.ErrInvalidGenesis)
		require.EqualError(t, invalid.Validate(), "agent stream metadata 0 has no matching agent stream: invalid genesis")
		moduleBefore, hostBefore := payStoreSnapshot(h.ctx, h.storeKey), payStoreSnapshot(h.ctx, h.hostKey)
		require.ErrorIs(t, h.keeper.InitGenesis(h.ctx, invalid), types.ErrInvalidGenesis)
		require.Equal(t, moduleBefore, payStoreSnapshot(h.ctx, h.storeKey))
		require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
	})
	t.Run("record_limit", func(t *testing.T) {
		bounded := *types.DefaultGenesis()
		for i := 1; i <= types.MaxKernelRecords+1; i++ {
			var nextID [32]byte
			binary.BigEndian.PutUint64(nextID[24:], uint64(i))
			nextBody := bytes.Clone(streamBody)
			copy(nextBody[:32], nextID[:])
			bounded.KernelRecords = append(bounded.KernelRecords, kv.Pair{
				Key: types.KernelKVKey(types.StreamModuleID, append([]byte("stream:"), nextID[:]...)), Value: nextBody,
			})
			bounded.AgentStreamMetadata = append(bounded.AgentStreamMetadata, kv.Pair{
				Key: agentStreamMetadataKey(nextID), Value: bytes.Clone(value),
			})
		}
		require.ErrorIs(t, bounded.Validate(), types.ErrInvalidGenesis)
		require.EqualError(t, bounded.Validate(), "agent stream metadata count 513 exceeds 512: invalid genesis")
		bounded.AgentStreamMetadata = bounded.AgentStreamMetadata[:types.MaxKernelRecords]
		require.NoError(t, bounded.Validate())
		require.NoError(t, ValidateOwnedGenesis(bounded))
	})
}

func TestPayGenesisRoundTrip(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		h := newPayHarness(t)
		hostBefore := payStoreSnapshot(h.ctx, h.hostKey)
		recordsBefore := h.keeper.ExportGenesis(h.ctx).KernelRecords
		require.NoError(t, h.keeper.InitGenesis(h.ctx, *types.DefaultGenesis()))
		require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
		exported := h.keeper.ExportGenesis(h.ctx)
		require.Equal(t, types.DefaultParams(), exported.Params)
		require.Equal(t, recordsBefore, exported.KernelRecords)
		require.NoError(t, exported.Validate())
		require.Len(t, payStoreSnapshot(h.ctx, h.storeKey), 2)
		require.Equal(t, []byte{1}, h.ctx.KVStore(h.storeKey).Get([]byte(serviceIndexVersionKey)))
	})

	t.Run("records", func(t *testing.T) {
		h := newPayHarness(t)
		gs := *h.keeper.ExportGenesis(h.ctx)
		gs.Params = types.Params{EscrowEnabled: true, BudgetEnabled: true, StreamEnabled: true, ServiceEnabled: true}
		for _, moduleID := range []uint16{types.EscrowModuleID, types.BudgetModuleID, types.StreamModuleID, types.ServiceModuleID} {
			key, err := types.KernelModuleEnableKey(moduleID)
			require.NoError(t, err)
			gs.KernelRecords = append(gs.KernelRecords, kv.Pair{Key: key, Value: payGateEnabledRecord()})
		}
		for moduleID := uint16(1); moduleID <= 11; moduleID++ {
			for _, key := range [][]byte{{0xff, 0x00}, {0x00, 0xff}} {
				if moduleID == types.EscrowModuleID {
					wire, record := escrowViewRecord()
					record.EscrowID[31] = key[0]
					copy(wire[:32], record.EscrowID[:])
					gs.KernelRecords = append(gs.KernelRecords, kv.Pair{
						Key: types.KernelKVKey(moduleID, append([]byte("hold:"), record.EscrowID[:]...)), Value: wire,
					})
					continue
				}
				if moduleID == types.ServiceModuleID {
					// Service bytes are now owned and must pass their real record decoder.
					fixture := serviceViewFixtures(t)[0]
					id := fixture.id
					id[31] = key[0]
					wire := bytes.Clone(fixture.wire)
					copy(wire[1:33], id[:])
					gs.KernelRecords = append(gs.KernelRecords, kv.Pair{Key: serviceKey(fixture.prefix, id), Value: wire})
					continue
				}
				record := kv.Pair{Key: types.KernelKVKey(moduleID, key), Value: []byte{byte(moduleID), key[0], key[1]}}
				require.Equal(t, append([]byte{0x01, 0x00, byte(moduleID)}, key...), record.Key)
				gs.KernelRecords = append(gs.KernelRecords, record)
			}
		}
		gs.KernelRecords = append(gs.KernelRecords,
			kv.Pair{Key: types.KernelKVKey(types.StreamModuleID, bytes.Repeat([]byte{0xa5}, types.MaxKernelKeyBytes)), Value: bytes.Repeat([]byte{0x5a}, types.MaxKernelValueBytes)},
			kv.Pair{Key: types.KernelKVKey(types.BudgetModuleID, []byte("empty")), Value: []byte{}},
			kv.Pair{Key: types.KernelKVKey(types.BudgetModuleID, []byte("nil"))},
		)
		sort.Slice(gs.KernelRecords, func(i, j int) bool { return bytes.Compare(gs.KernelRecords[i].Key, gs.KernelRecords[j].Key) > 0 })
		hostWant := payStoreSnapshot(h.ctx, h.hostKey)
		var recordsWant []kv.Pair
		for _, record := range gs.KernelRecords {
			value := append([]byte{}, record.Value...)
			hostWant[string(record.Key)] = value
			switch binary.BigEndian.Uint16(record.Key[1:3]) {
			case 2, 3, 4, 5, 7:
				recordsWant = append(recordsWant, kv.Pair{Key: bytes.Clone(record.Key), Value: value})
			}
		}
		sort.Slice(recordsWant, func(i, j int) bool { return bytes.Compare(recordsWant[i].Key, recordsWant[j].Key) < 0 })
		require.NoError(t, h.keeper.InitGenesis(h.ctx, gs))
		require.Equal(t, gs.Params, h.keeper.GetParams(h.ctx))
		require.Equal(t, hostWant, payStoreSnapshot(h.ctx, h.hostKey))
		moduleBefore := payStoreSnapshot(h.ctx, h.storeKey)
		require.Len(t, moduleBefore, 2)
		require.Equal(t, []byte{1}, moduleBefore[serviceIndexVersionKey])
		require.Contains(t, moduleBefore, string(types.ParamsKey))
		for _, method := range []string{"escrowOpen", "budgetCreate", "streamOpen", "serviceOfferPublish"} {
			require.Contains(t, types.MethodActivity, method)
			require.NoError(t, h.keeper.CheckWrite(h.ctx, method))
		}
		exported := h.keeper.ExportGenesis(h.ctx)
		require.Equal(t, &types.GenesisState{Params: gs.Params, KernelRecords: recordsWant}, exported)
		require.NoError(t, exported.Validate())
		require.Equal(t, hostWant, payStoreSnapshot(h.ctx, h.hostKey))
		require.Equal(t, moduleBefore, payStoreSnapshot(h.ctx, h.storeKey))
		for i := 1; i < len(exported.KernelRecords); i++ {
			require.Negative(t, bytes.Compare(exported.KernelRecords[i-1].Key, exported.KernelRecords[i].Key))
		}

		restored := newPayHarness(t)
		require.NoError(t, restored.keeper.InitGenesis(restored.ctx, *exported))
		require.Equal(t, exported, restored.keeper.ExportGenesis(restored.ctx))
		restored.registerActor(t, 3)
		for _, record := range recordsWant {
			require.Equal(t, record.Value, restored.ctx.KVStore(restored.hostKey).Get(record.Key))
		}
		exported.KernelRecords[0].Key[0] ^= 0xff
		exported.KernelRecords[0].Value[0] ^= 0xff
		require.Equal(t, recordsWant, h.keeper.ExportGenesis(h.ctx).KernelRecords)
	})
}

func TestPayGenesisRefusesGateDrift(t *testing.T) {
	h := newPayHarness(t)
	h.keeper.SetParams(h.ctx, types.Params{Paused: true, EscrowEnabled: true, BudgetEnabled: true, StreamEnabled: true, ServiceEnabled: true})
	for _, family := range []struct {
		name   string
		id     uint16
		params types.Params
	}{
		{"escrow", types.EscrowModuleID, types.Params{EscrowEnabled: true}},
		{"budget", types.BudgetModuleID, types.Params{BudgetEnabled: true}},
		{"stream", types.StreamModuleID, types.Params{StreamEnabled: true}},
		{"service", types.ServiceModuleID, types.Params{ServiceEnabled: true}},
	} {
		key, err := types.KernelModuleEnableKey(family.id)
		require.NoError(t, err)
		h.ctx.KVStore(h.hostKey).Set(key, payGateEnabledRecord())
		for _, gate := range []struct {
			name  string
			value []byte
		}{
			{"absent", nil}, {"disabled", make([]byte, 32)}, {"malformed", []byte{1}},
		} {
			t.Run(family.name+"/"+gate.name, func(t *testing.T) {
				wire, hold := escrowViewRecord()
				gs := types.GenesisState{Params: family.params, KernelRecords: []kv.Pair{
					{Key: types.KernelKVKey(types.EscrowModuleID, append([]byte("hold:"), hold.EscrowID[:]...)), Value: wire},
				}}
				if gate.value != nil {
					gs.KernelRecords = append(gs.KernelRecords, kv.Pair{Key: key, Value: gate.value})
				}
				moduleBefore := payStoreSnapshot(h.ctx, h.storeKey)
				hostBefore := payStoreSnapshot(h.ctx, h.hostKey)
				err := h.keeper.InitGenesis(h.ctx, gs)
				require.ErrorIs(t, err, types.ErrInvalidGenesis)
				if gate.name == "malformed" {
					require.ErrorContains(t, err, "kernel record 1 has a non-canonical enable flag")
				} else {
					require.ErrorContains(t, err, family.name+" enabled while kernel genesis gate is clear")
				}
				require.Equal(t, moduleBefore, payStoreSnapshot(h.ctx, h.storeKey))
				require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
			})
		}
	}

	t.Run("per_record_bounds", func(t *testing.T) {
		gs := types.GenesisState{Params: types.Params{}, KernelRecords: make([]kv.Pair, types.MaxKernelRecords+1)}
		fixture := serviceViewFixtures(t)[0]
		for i := range gs.KernelRecords {
			var id [32]byte
			binary.BigEndian.PutUint64(id[24:], uint64(i+1))
			wire := bytes.Clone(fixture.wire)
			copy(wire[1:33], id[:])
			gs.KernelRecords[i] = kv.Pair{Key: serviceKey(fixture.prefix, id), Value: wire}
		}
		moduleBefore := payStoreSnapshot(h.ctx, h.storeKey)
		hostBefore := payStoreSnapshot(h.ctx, h.hostKey)
		last := &gs.KernelRecords[len(gs.KernelRecords)-1]
		canonical := last.Value
		last.Value = make([]byte, types.MaxKernelValueBytes+1)
		err := h.keeper.InitGenesis(h.ctx, gs)
		require.ErrorIs(t, err, types.ErrInvalidGenesis)
		require.Contains(t, err.Error(), "kernel record 512 value exceeds 1024 bytes")
		require.Equal(t, moduleBefore, payStoreSnapshot(h.ctx, h.storeKey))
		require.Equal(t, hostBefore, payStoreSnapshot(h.ctx, h.hostKey))
		last.Value = canonical
		require.NoError(t, h.keeper.InitGenesis(h.ctx, gs))
		require.Equal(t, gs.Params, h.keeper.GetParams(h.ctx))
		for _, record := range gs.KernelRecords {
			require.Equal(t, record.Value, h.ctx.KVStore(h.hostKey).Get(record.Key))
			require.Nil(t, h.ctx.KVStore(h.storeKey).Get(record.Key))
		}
	})
}

func TestPayGenesisKeepsEscrowReceipts(t *testing.T) {
	source, _ := proofPayKeeperOpen(t, t.TempDir())
	source.actors = []payActor{source.registerActor(t, 1), source.registerActor(t, 2)}
	id, partialKey, releaseKey := [32]byte{0xe1}, [32]byte{0xe2}, [32]byte{0xe3}
	payload := paySubmitFundedEscrow(t, source, id)
	opened, err := source.keeper.Submit(source.ctx, source.actors[0].address, "escrowOpen", payload, id)
	require.NoError(t, err)
	partial, err := source.keeper.EscrowPartialCapture(source.ctx, source.actors[1].address, id, codec.U128{Lo: 20}, partialKey)
	require.NoError(t, err)
	released, err := source.keeper.EscrowRelease(source.ctx, source.actors[0].address, id, releaseKey)
	require.NoError(t, err)
	produced := []lxphost.Receipt{opened, partial, released}
	for i := 0; i < types.MaxKernelRecords; i++ {
		replay, err := source.keeper.EscrowRelease(source.ctx, source.actors[0].address, id, releaseKey)
		require.NoError(t, err)
		require.Empty(t, replay.Transfers)
		require.Greater(t, replay.GlobalSequence, produced[len(produced)-1].GlobalSequence)
		produced = append(produced, replay)
	}
	hold, found, err := source.keeper.GetEscrow(source.ctx, id)
	require.NoError(t, err)
	require.True(t, found)
	partialResult, found, err := source.keeper.GetEscrowResult(source.ctx, partialKey)
	require.NoError(t, err)
	require.True(t, found)
	releaseResult, found, err := source.keeper.GetEscrowResult(source.ctx, releaseKey)
	require.NoError(t, err)
	require.True(t, found)
	require.NotEqual(t, [32]byte{}, releaseResult.TransferSetRoot)
	checkReceipts := func(h *payHarness) {
		t.Helper()
		for i, receipt := range produced {
			canonical, err := codec.DecodeReceipt(receipt.Encoded)
			require.NoError(t, err)
			stored, found, err := h.keeper.GetEscrowReceipt(h.ctx, receipt.GlobalSequence)
			require.NoError(t, err)
			require.True(t, found, "sequence %d", receipt.GlobalSequence)
			require.Equal(t, receipt.GlobalSequence, stored.GlobalSequence)
			require.Equal(t, receipt.Encoded, stored.CanonicalBytes())
			require.Equal(t, canonical.TransferSetRoot, stored.TransferSetRoot)
			require.Equal(t, canonical.Effects, stored.Effects)
			require.Equal(t, canonical.Digest(), stored.Digest())
			if i == 1 {
				require.Equal(t, partialResult.TransferSetRoot, stored.TransferSetRoot)
			} else if i >= 2 {
				require.Equal(t, releaseResult.TransferSetRoot, stored.TransferSetRoot)
			}
		}
	}
	checkReceipts(source)
	standard := source.keeper.ExportGenesis(source.ctx)
	recovery := source.keeper.ExportRecoveryGenesis(source.ctx)
	host, err := lxphost.ExportStoreGenesis(source.ctx, source.hostKey)
	require.NoError(t, err)
	require.NoError(t, host.Validate())
	require.NoError(t, ValidateOwnedGenesis(*standard))
	require.NoError(t, recovery.Validate())
	require.Len(t, standard.EscrowReceipts, len(produced))
	require.Equal(t, uint64(len(produced)), recovery.EscrowReceiptCount)
	receiptChunks := 0
	for _, chunk := range recovery.Chunks {
		require.LessOrEqual(t, len(chunk.KernelRecords)+len(chunk.AgentStreamMetadata)+len(chunk.EscrowReceipts), types.MaxKernelRecords)
		if len(chunk.EscrowReceipts) > 0 {
			receiptChunks++
		}
	}
	require.Greater(t, receiptChunks, 1)
	for i, pair := range standard.EscrowReceipts {
		require.Equal(t, escrowReceiptKey(produced[i].GlobalSequence), pair.Key)
		require.Equal(t, produced[i].Encoded, pair.Value)
	}
	detached := source.keeper.ExportGenesis(source.ctx)
	detached.EscrowReceipts[0].Key[0] ^= 0xff
	detached.EscrowReceipts[0].Value[0] ^= 0xff
	detachedRecovery := source.keeper.ExportRecoveryGenesis(source.ctx)
	detachedRecovery.Chunks[0].EscrowReceipts[0].Key[0] ^= 0xff
	detachedRecovery.Chunks[0].EscrowReceipts[0].Value[0] ^= 0xff
	require.Equal(t, standard, source.keeper.ExportGenesis(source.ctx))
	require.Equal(t, recovery, source.keeper.ExportRecoveryGenesis(source.ctx))
	checkReceipts(source)

	mutations := []struct {
		name  string
		apply func(*kv.Pair)
	}{
		{"short_key", func(p *kv.Pair) { p.Key = p.Key[:len(p.Key)-1] }},
		{"long_key", func(p *kv.Pair) { p.Key = append(p.Key, 0) }},
		{"foreign_key", func(p *kv.Pair) { p.Key[0] ^= 0xff }},
		{"sequence_mismatch", func(p *kv.Pair) { p.Key[len(p.Key)-1] ^= 1 }},
		{"zero_sequence", func(p *kv.Pair) { clear(p.Key[len(types.EscrowReceiptPrefix):]) }},
		{"truncated_body", func(p *kv.Pair) { p.Value = p.Value[:len(p.Value)-1] }},
		{"trailing_body", func(p *kv.Pair) { p.Value = append(p.Value, 0) }},
		{"oversized_body", func(p *kv.Pair) { p.Value = make([]byte, codec.MaxMessageBytes+1) }},
		{"foreign_module", func(p *kv.Pair) {
			r, err := codec.DecodeReceipt(p.Value)
			require.NoError(t, err)
			offset := 4 + 2 + 36 + 8 + 36 + 36 + 36 + 4 + 4
			for _, event := range r.Effects {
				offset += 2 + 2 + 2 + 1 + 1 + 36 + 4 + len(event.Body)
			}
			offset += 16 + 36
			binary.BigEndian.PutUint16(p.Value[offset:offset+2], types.StreamModuleID)
		}},
		{"failed_receipt", func(p *kv.Pair) { binary.BigEndian.PutUint32(p.Value[158:162], 1) }},
		{"economic_root", func(p *kv.Pair) {
			r, err := codec.DecodeReceipt(p.Value)
			require.NoError(t, err)
			offset := bytes.LastIndex(p.Value, r.TransferSetRoot[:])
			require.GreaterOrEqual(t, offset, 0)
			p.Value[offset] ^= 1
		}},
		{"signed_receipt", func(p *kv.Pair) {
			p.Value[len(p.Value)-1] = 1
			p.Value = binary.BigEndian.AppendUint32(p.Value, 64)
			p.Value = append(p.Value, make([]byte, 64)...)
		}},
	}
	for _, kind := range []string{"genesis", "recovery"} {
		t.Run(kind, func(t *testing.T) {
			directory := t.TempDir()
			destination, stop := proofPayKeeperOpen(t, directory)
			type transport struct {
				Host     lxphost.StoreGenesis `json:"host"`
				Pay      *types.GenesisState  `json:"pay,omitempty"`
				Recovery *RecoveryGenesis     `json:"recovery,omitempty"`
			}
			envelope := transport{Host: *host}
			if kind == "genesis" {
				envelope.Pay = standard
			} else {
				envelope.Recovery = recovery
			}
			wire, err := json.Marshal(envelope)
			require.NoError(t, err)
			decode := func() transport {
				t.Helper()
				var imported transport
				require.NoError(t, json.Unmarshal(wire, &imported))
				return imported
			}
			initPay := func(h *payHarness, imported transport) error {
				if imported.Pay != nil {
					return h.keeper.InitGenesis(h.ctx, *imported.Pay)
				}
				return h.keeper.InitRecoveryGenesis(h.ctx, *imported.Recovery)
			}
			reject := func(imported transport) {
				t.Helper()
				beforeHost := payStoreSnapshot(destination.ctx, destination.hostKey)
				beforePay := payStoreSnapshot(destination.ctx, destination.storeKey)
				require.ErrorIs(t, initPay(destination, imported), types.ErrInvalidGenesis)
				require.Equal(t, beforeHost, payStoreSnapshot(destination.ctx, destination.hostKey))
				require.Equal(t, beforePay, payStoreSnapshot(destination.ctx, destination.storeKey))
			}
			for _, mutation := range mutations {
				t.Run(mutation.name, func(t *testing.T) {
					imported := decode()
					if imported.Pay != nil {
						mutation.apply(&imported.Pay.EscrowReceipts[0])
						require.ErrorIs(t, imported.Pay.Validate(), types.ErrInvalidGenesis)
					} else {
						mutation.apply(&imported.Recovery.Chunks[0].EscrowReceipts[0])
						imported.Recovery.Checksum = imported.Recovery.checksum()
					}
					reject(imported)
				})
			}
			t.Run("duplicate", func(t *testing.T) {
				imported := decode()
				if imported.Pay != nil {
					imported.Pay.EscrowReceipts = append(imported.Pay.EscrowReceipts, imported.Pay.EscrowReceipts[0])
				} else {
					g := imported.Recovery
					g.Chunks[1].EscrowReceipts[0] = g.Chunks[0].EscrowReceipts[len(g.Chunks[0].EscrowReceipts)-1]
					g.Checksum = g.checksum()
				}
				reject(imported)
			})
			if kind == "recovery" {
				for _, alteration := range []struct {
					name  string
					apply func(*RecoveryGenesis)
				}{
					{"receipt_count", func(g *RecoveryGenesis) { g.EscrowReceiptCount++; g.Checksum = g.checksum() }},
					{"kernel_count", func(g *RecoveryGenesis) { g.KernelRecordCount++; g.Checksum = g.checksum() }},
					{"metadata_count", func(g *RecoveryGenesis) { g.MetadataRecordCount++; g.Checksum = g.checksum() }},
					{"checksum", func(g *RecoveryGenesis) { g.Checksum[0] ^= 1 }},
					{"receipt_bytes_checksum", func(g *RecoveryGenesis) {
						p := &g.Chunks[0].EscrowReceipts[0]
						p.Value[len(p.Value)-2] ^= 1
					}},
					{"receipt_order", func(g *RecoveryGenesis) {
						rows := g.Chunks[0].EscrowReceipts
						rows[0], rows[1] = rows[1], rows[0]
						g.Checksum = g.checksum()
					}},
					{"chunk_overflow", func(g *RecoveryGenesis) {
						g.Chunks[0].EscrowReceipts = append(g.Chunks[0].EscrowReceipts, g.Chunks[1].EscrowReceipts[0])
						g.Chunks[1].EscrowReceipts = g.Chunks[1].EscrowReceipts[1:]
						g.Checksum = g.checksum()
					}},
					{"missing_chunk", func(g *RecoveryGenesis) { g.Chunks = g.Chunks[:1]; g.Checksum = g.checksum() }},
				} {
					t.Run(alteration.name, func(t *testing.T) {
						imported := decode()
						alteration.apply(imported.Recovery)
						reject(imported)
					})
				}
			}
			imported := decode()
			require.NoError(t, lxphost.InitStoreGenesis(destination.ctx, destination.hostKey, imported.Host))
			require.NoError(t, initPay(destination, imported))
			checkReceipts(destination)
			destination.cms.Commit(true)
			stop()
			reopened, _ := proofPayKeeperOpen(t, directory)
			checkReceipts(reopened)
			restored, found, err := reopened.keeper.GetEscrow(reopened.ctx, id)
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, hold, restored)
			for key, want := range map[[32]byte]types.EscrowResult{partialKey: partialResult, releaseKey: releaseResult} {
				got, found, err := reopened.keeper.GetEscrowResult(reopened.ctx, key)
				require.NoError(t, err)
				require.True(t, found)
				require.Equal(t, want, got)
			}
			restoredHost, err := lxphost.ExportStoreGenesis(reopened.ctx, reopened.hostKey)
			require.NoError(t, err)
			require.Equal(t, host, restoredHost)
			require.Equal(t, standard, reopened.keeper.ExportGenesis(reopened.ctx))
			require.Equal(t, recovery, reopened.keeper.ExportRecoveryGenesis(reopened.ctx))
		})
	}
}

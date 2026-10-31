package layerxprograms_test

import (
	"math/big"
	"reflect"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	"github.com/stretchr/testify/require"
)

func TestProgramsSubmitCarriesReceiptAuthorities(t *testing.T) {
	h, args, payload := programsCallFixture(t)
	ctx := h.stateDB.Ctx()
	height := uint64(ctx.BlockHeight())
	registryMethod := lp.ProgramsABI.Methods["registryRead"]
	registryPayload, err := lp.EncodeRegistryReadPayload(args[0].([32]byte), []byte("manifest"))
	require.NoError(t, err)
	var authorities []lp.ReceiptAuthority
	for _, sourceHeight := range []uint64{height - 2, height - 1} {
		receipt, err := lp.Submit(ctx.WithBlockHeight(int64(sourceHeight)), h.executor,
			lp.NewCallContext(h.caller, &registryMethod), 0x00090004, registryPayload, [16]byte{})
		require.NoError(t, err)
		require.Zero(t, receipt.ResultCode)
		require.NotEmpty(t, receipt.Encoded)
		decoded, err := codec.DecodeReceipt(receipt.Encoded)
		require.NoError(t, err)
		require.Equal(t, receipt.Encoded, decoded.CanonicalBytes())
		authorities = append(authorities, lp.ReceiptAuthority{
			CanonicalReceipt: receipt.Encoded, BlockHeight: sourceHeight,
		})
	}
	method := lp.ProgramsABI.Methods["call"]
	call := lp.NewCallContext(h.caller, &method)
	var feeLimit [16]byte
	args[8].(*big.Int).FillBytes(feeLimit[:])
	readHash := func(value reflect.Value) [32]byte {
		var hash [32]byte
		require.Equal(t, len(hash), value.Len())
		for i := range hash {
			hash[i] = byte(value.Index(i).Uint())
		}
		return hash
	}
	for _, test := range []struct {
		name        string
		authorities []lxphost.ReceiptAuthority
	}{
		{name: "past", authorities: authorities[1:]},
		{name: "every receipt", authorities: authorities},
		{name: "empty after receipts", authorities: []lxphost.ReceiptAuthority{}},
		{name: "none"},
	} {
		t.Run(test.name, func(t *testing.T) {
			receipt, err := lp.Submit(ctx, h.executor, call, 0x00090003, payload, feeLimit, test.authorities...)
			require.NoError(t, err)
			require.Zero(t, receipt.ResultCode)
			require.NotNil(t, receipt.ProgramOutcome)
			require.Zero(t, receipt.ProgramOutcome.Status)
			node := reflect.ValueOf(h.host).Elem().FieldByName("node").Elem()
			indexPointer := node.FieldByName("execution").FieldByName("verified_receipts")
			require.False(t, indexPointer.IsNil())
			require.Equal(t, node.FieldByName("verified_receipts").Addr().Pointer(), indexPointer.Pointer())
			index := indexPointer.Elem()
			require.Equal(t, uint64(len(test.authorities)), index.FieldByName("authority_count").Uint())
			want := make(map[[32]byte]lxphost.ReceiptAuthority, len(test.authorities))
			for _, authority := range test.authorities {
				decoded, err := codec.DecodeReceipt(authority.CanonicalReceipt)
				require.NoError(t, err)
				want[decoded.Digest()] = authority
			}
			for i := range test.authorities {
				facts := index.FieldByName("authority_entries").Index(i)
				digest := readHash(facts.FieldByName("receipt_digest"))
				authority, exists := want[digest]
				require.True(t, exists, "kernel indexed an uncited receipt: %x", digest)
				decoded, err := codec.DecodeReceipt(authority.CanonicalReceipt)
				require.NoError(t, err)
				require.Equal(t, authority.BlockHeight, facts.FieldByName("block_height").Uint())
				require.Equal(t, uint64(2), facts.FieldByName("version").Uint())
				require.Equal(t, uint64(7), facts.FieldByName("network_id").Uint())
				require.Equal(t, uint64(decoded.ProtocolVersion), facts.FieldByName("protocol_version").Uint())
				require.Equal(t, uint64(decoded.ModuleID), facts.FieldByName("module_id").Uint())
				require.Equal(t, decoded.ActivityID, readHash(facts.FieldByName("activity_id")))
				delete(want, digest)
			}
			require.Empty(t, want)
		})
	}
	for _, refusedHeight := range []uint64{height, height + 1} {
		t.Run(new(big.Int).SetUint64(refusedHeight).String(), func(t *testing.T) {
			before := h.dump(ctx)
			cited := append([]lxphost.ReceiptAuthority(nil), authorities[:1]...)
			cited = append(cited, lxphost.ReceiptAuthority{
				CanonicalReceipt: authorities[1].CanonicalReceipt, BlockHeight: refusedHeight,
			})
			receipt, err := lp.Submit(ctx, h.executor, call, 0x00090003, payload, feeLimit, cited...)
			require.ErrorIs(t, err, lxphost.Code(-3))
			require.EqualError(t, err, "LXP_ERR_NON_CANONICAL")
			require.Equal(t, lxphost.Receipt{}, receipt)
			require.Equal(t, before, h.dump(ctx))
		})
	}
}

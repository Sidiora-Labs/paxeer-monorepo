package layerxprograms_test

import (
	"reflect"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestProgramsCallWithReceiptsCarriesAuthorities(t *testing.T) {
	for _, web := range []bool{false, true} {
		name := "call handlers"
		if web {
			name = "web request handlers"
		}
		t.Run(name, func(t *testing.T) {
			h, args, _ := programsCallFixture(t)
			if web {
				lp.ConfigureXWeb(h.executor, nil)
			}
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
				method      string
				authorities []lp.ReceiptAuthority
			}{
				{name: "one past receipt", method: "callWithReceipts", authorities: authorities[1:]},
				{name: "every receipt", method: "callWithReceipts", authorities: authorities},
				{name: "empty after receipts", method: "callWithReceipts", authorities: []lp.ReceiptAuthority{}},
				{name: "same call without authorities", method: "call"},
			} {
				t.Run(test.name, func(t *testing.T) {
					callArgs := append([]interface{}(nil), args...)
					if test.method == "callWithReceipts" {
						callArgs = append(callArgs, test.authorities)
					}
					input, err := lp.ProgramsABI.Pack(test.method, callArgs...)
					require.NoError(t, err)
					const supplied uint64 = 20000000
					output, remaining, err := h.evm.Call(h.caller, h.precompile.Address(), input, supplied, uint256.NewInt(0))
					if err != nil {
						reason, decodeErr := abi.UnpackRevert(output)
						require.NoError(t, err, "%s reverted: %s (decode: %v)", test.method, reason, decodeErr)
					}
					require.NoError(t, h.stateDB.GetPrecompileError())
					require.Equal(t, lp.GasTable[test.method].Gas(input[4:]), supplied-remaining)
					values, err := lp.ProgramsABI.Unpack(test.method, output)
					require.NoError(t, err)
					require.Equal(t, int32(0), values[0])
					require.Equal(t, uint8(1), values[1])
					require.NotEmpty(t, values[2])
					node := reflect.ValueOf(h.host).Elem().FieldByName("node").Elem()
					indexPointer := node.FieldByName("execution").FieldByName("verified_receipts")
					require.False(t, indexPointer.IsNil())
					require.Equal(t, node.FieldByName("verified_receipts").Addr().Pointer(), indexPointer.Pointer())
					index := indexPointer.Elem()
					require.Equal(t, uint64(len(test.authorities)), index.FieldByName("authority_count").Uint())
					want := make(map[[32]byte]lp.ReceiptAuthority, len(test.authorities))
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
		})
	}
}

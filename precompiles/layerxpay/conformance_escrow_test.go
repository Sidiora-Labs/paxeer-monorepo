package layerxpay_test

import (
	"bytes"
	"math/big"
	"reflect"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	paytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/kv"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func escrowConformanceRows(t *testing.T, h *payPrecompileHarness) []kv.Pair {
	t.Helper()
	iterator := h.db.Ctx().KVStore(escrowTestHostKey(t, h)).Iterator(nil, nil)
	var rows []kv.Pair
	for ; iterator.Valid(); iterator.Next() {
		require.NoError(t, iterator.Error())
		rows = append(rows, kv.Pair{Key: bytes.Clone(iterator.Key()), Value: bytes.Clone(iterator.Value())})
	}
	require.EqualError(t, iterator.Error(), "invalid cacheMergeIterator")
	require.NoError(t, iterator.Close())
	return rows
}

func escrowConformanceReceipt(t *testing.T, host *lxphost.Host, length int) *codec.Receipt {
	t.Helper()
	node := reflect.ValueOf(host).Elem().FieldByName("node")
	require.False(t, node.IsNil())
	arena := node.Elem().FieldByName("arena_bytes")
	require.Equal(t, reflect.Array, arena.Kind())
	require.Positive(t, length)
	require.LessOrEqual(t, length, arena.Len())
	require.GreaterOrEqual(t, node.Elem().FieldByName("arena").FieldByName("offset").Uint(), uint64(length))
	encoded := make([]byte, length)
	for i := range encoded {
		encoded[i] = byte(arena.Index(i).Uint())
	}
	receipt, err := codec.DecodeReceipt(encoded)
	require.NoError(t, err)
	return receipt
}

func TestPayEscrowPathMatchesHostExecute(t *testing.T) {
	for _, scenario := range []string{"capture", "partialCapture", "release", "timeout"} {
		t.Run(scenario, func(t *testing.T) {
			f := newEscrowTestFixture(t)
			precompile, direct := f.h, newPayPrecompileHarness(t)
			require.Equal(t, common.HexToAddress("0x101b"), precompile.precompile.Address())
			seed := escrowConformanceRows(t, precompile)
			directStore := direct.db.Ctx().KVStore(escrowTestHostKey(t, direct))
			for _, row := range escrowConformanceRows(t, direct) {
				directStore.Delete(row.Key)
			}
			for _, row := range seed {
				directStore.Set(row.Key, bytes.Clone(row.Value))
			}
			require.Equal(t, seed, escrowConformanceRows(t, direct))
			initialSequence, err := precompile.keeper.Host().ReadLastSequence(precompile.db.Ctx(), f.owner.did)
			require.NoError(t, err)
			expectedSequence := initialSequence
			id := [32]byte{0x40, 0x01}
			args := f.openArgs(t, id)
			var resultKeys [][32]byte

			execute := func(t *testing.T, actor escrowTestActor, method string, kind uint32, payload []byte,
				key [32]byte, state uint8, captured, locked uint64, callArgs ...interface{}) {
				t.Helper()
				ctx, host := direct.db.Ctx(), direct.keeper.Host()
				last, err := host.ReadLastSequence(ctx, actor.did)
				require.NoError(t, err)
				require.Equal(t, expectedSequence, last)
				sequence, err := host.ReadIdentitySequence(ctx, actor.did)
				require.NoError(t, err)
				now := uint64(ctx.BlockTime().UnixMilli())
				want, err := host.HostExecute(lxphost.WithSequence(ctx, last+1), lxphost.Activity{
					ProtocolVersion: 3, Type: kind, ActorDID: actor.did, AuthorityKey: actor.public,
					AccountSequence: sequence, NotBefore: now, NotAfter: now + host.TimestampWindowMs(),
					IdempotencyKey: key, Payload: payload,
				}, lxphost.KernelAuthority{
					Bound: true, Actor: actor.actor, Principal: actor.account, VerifiedKey: actor.public,
				})
				require.NoError(t, err)
				require.Zero(t, want.ResultCode)
				expectedSequence++
				require.Equal(t, expectedSequence, want.GlobalSequence)
				require.NotEqual(t, [32]byte{}, want.ResultingStateRoot)
				require.NotEqual(t, want.PreviousStateRoot, want.ResultingStateRoot)

				input, err := precompile.precompile.GetABI().Pack(method, callArgs...)
				require.NoError(t, err)
				output, _, err := precompile.evm.Call(actor.caller, precompile.precompile.Address(), input, 2000000, uint256.NewInt(0))
				require.NoError(t, err, "%s: %x", method, output)
				require.Empty(t, output)
				got := escrowConformanceReceipt(t, precompile.keeper.Host(), len(want.Encoded))
				require.Zero(t, got.ResultCode)
				require.Equal(t, expectedSequence, got.GlobalSequence)
				require.Equal(t, want.PreviousStateRoot, got.PreviousStateRoot)
				require.Equal(t, want.ResultingStateRoot, got.ResultingStateRoot)
				require.Equal(t, want.Encoded, got.CanonicalBytes())
				require.Equal(t, escrowConformanceRows(t, direct), escrowConformanceRows(t, precompile))

				record, found := precompile.keeper.GetEscrow(precompile.db.Ctx(), id)
				require.True(t, found)
				wantRecord, found := direct.keeper.GetEscrow(ctx, id)
				require.True(t, found)
				require.Equal(t, wantRecord, record)
				require.Equal(t, paytypes.EscrowRecord{
					EscrowID: id, Owner: f.owner.account, EscrowAccount: args[2].([32]byte),
					Beneficiary: f.beneficiary.account, Arbiter: f.arbiter.account, AssetID: f.asset,
					LockedAmount: codec.U128{Lo: locked}, CapturedAmount: codec.U128{Lo: captured}, State: state,
					ExpiryMs: args[7].(uint64), DisputeWindowMs: args[8].(uint64),
					TermsHash: args[9].([32]byte), AgreementReference: args[10].([32]byte),
				}, record)
				resultKeys = append(resultKeys, key)
				for _, resultKey := range resultKeys {
					result, found := precompile.keeper.GetEscrowResult(precompile.db.Ctx(), resultKey)
					require.True(t, found)
					wantResult, found := direct.keeper.GetEscrowResult(ctx, resultKey)
					require.True(t, found)
					require.Equal(t, wantResult, result)
					if resultKey == key {
						require.Equal(t, id, result.EscrowID)
						require.Equal(t, uint16(kind), result.Ordinal)
						require.Equal(t, state, result.StateAfter)
						require.Equal(t, record.CapturedAmount, result.CapturedAfter)
						require.Equal(t, record.LockedAmount, result.LockedAfter)
						require.Equal(t, expectedSequence, result.GlobalSequence)
						require.Equal(t, now, result.TimestampMs)
					}
				}
				for _, h := range []*payPrecompileHarness{precompile, direct} {
					for _, a := range []escrowTestActor{f.owner, f.beneficiary, f.arbiter} {
						last, err := h.keeper.Host().ReadLastSequence(h.db.Ctx(), a.did)
						require.NoError(t, err)
						require.Equal(t, expectedSequence, last)
						identitySequence, err := h.keeper.Host().ReadIdentitySequence(h.db.Ctx(), a.did)
						require.NoError(t, err)
						wantIdentitySequence, err := host.ReadIdentitySequence(ctx, a.did)
						require.NoError(t, err)
						require.Equal(t, wantIdentitySequence, identitySequence)
						if a.actor == actor.actor {
							require.Equal(t, sequence+1, identitySequence)
						}
					}
					ownerBalance := uint64(1000 - captured - locked)
					for _, balance := range []struct {
						account [32]byte
						amount  uint64
					}{{record.Owner, ownerBalance}, {record.Beneficiary, captured}, {record.EscrowAccount, locked}} {
						value, err := h.keeper.Host().ReadBalance(h.db.Ctx(), balance.account, f.asset)
						require.NoError(t, err)
						require.Equal(t, sdk.NewIntFromUint64(balance.amount), value)
					}
				}
			}

			openPayload := paytypes.EncodeEscrowOpen(id, f.owner.account, args[2].([32]byte),
				f.beneficiary.account, f.arbiter.account, f.asset, codec.U128{Lo: 100},
				args[7].(uint64), args[8].(uint64), args[9].([32]byte), args[10].([32]byte))
			execute(t, f.owner, "escrowOpen", paytypes.ActivityEscrowOpen, openPayload, id, 1, 0, 100, args...)
			switch scenario {
			case "capture":
				key := [32]byte{0x41}
				execute(t, f.beneficiary, "escrowCapture", paytypes.ActivityEscrowCapture,
					paytypes.EncodeEscrowCapture(id, codec.U128{Lo: 100}, key), key, 3, 100, 0, id, big.NewInt(100), key)
			case "partialCapture":
				key := [32]byte{0x42}
				execute(t, f.beneficiary, "escrowPartialCapture", paytypes.ActivityEscrowPartialCapture,
					paytypes.EncodeEscrowCapture(id, codec.U128{Lo: 25}, key), key, 2, 25, 75, id, big.NewInt(25), key)
				key = [32]byte{0x43}
				execute(t, f.beneficiary, "escrowCapture", paytypes.ActivityEscrowCapture,
					paytypes.EncodeEscrowCapture(id, codec.U128{Lo: 75}, key), key, 3, 100, 0, id, big.NewInt(75), key)
			case "release":
				key := [32]byte{0x44}
				execute(t, f.owner, "escrowRelease", paytypes.ActivityEscrowRelease,
					paytypes.EncodeEscrowRelease(id, key), key, 4, 0, 0, id, key)
			case "timeout":
				for _, h := range []*payPrecompileHarness{precompile, direct} {
					h.db.WithCtx(h.db.Ctx().WithBlockTime(time.UnixMilli(int64(args[7].(uint64)))))
				}
				key := [32]byte{0x45}
				execute(t, f.beneficiary, "escrowTimeout", paytypes.ActivityEscrowTimeout,
					paytypes.EncodeEscrowRelease(id, key), key, 7, 0, 0, id, key)
			}
		})
	}
}

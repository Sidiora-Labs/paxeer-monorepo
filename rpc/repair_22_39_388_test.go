//go:build cgo

package evmrpc_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	"github.com/stretchr/testify/require"
)

func TestRepair22_39_388_BalanceSightAfterBlockEndTransition(t *testing.T) {
	wasm := repair478PackageGuest(t, repair478RawGuest(t, "")).packaged
	for _, candidate := range []struct {
		name string
		want lxphost.Code
	}{
		{name: "last_activity_of_H", want: lxphost.Code(-3)},
		{name: "transition_bytes"},
		{name: "anchor_before_transition", want: lxphost.Code(-3)},
		{name: "anchor_first_in_next_block", want: lxphost.Code(-3)},
		{name: "kernel_control", want: lxphost.Code(0)},
	} {
		t.Run(candidate.name, func(t *testing.T) {
			repair478RunWithProbe(t, wasm, repair478CPUAllowance, false, "", func(f repair478ProbeFixture) {
				ctx := f.Context
				var named lxphost.Receipt
				if candidate.name == "anchor_before_transition" {
					named = f.Execute(0x00090006, f.Registration)
				}
				last, err := f.Host.ReadLastSequence(ctx, f.DID)
				require.NoError(t, err)
				transition := FinalizeProgramsOccupancy(t, f.Host, ctx, last+1)
				transitionSequence, err := f.Host.ReadLastSequence(ctx, f.DID)
				require.NoError(t, err)
				require.Equal(t, last+1, transitionSequence)
				if candidate.name == "transition_bytes" {
					_, err := codec.DecodeReceipt(transition)
					require.ErrorIs(t, err, codec.ErrReceiptShape)
					require.EqualError(t, err, "layerx codec: not a full protocol receipt")
					t.Logf("candidate=%s named_sequence=%d next_sequence=%d decoder=%v", candidate.name, transitionSequence, transitionSequence+1, err)
					return
				}
				if candidate.name == "kernel_control" {
					// This is the kernel case's artificial order: the anchor follows
					// the transition, even though the transition ends a live block.
					named = f.Execute(0x00090006, f.Registration)
				}
				ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 1)
				ctx = ctx.WithBlockTime(time.UnixMilli(seqVecTimeBase + ctx.BlockHeight()*seqVecBlockMs))
				if candidate.name == "anchor_first_in_next_block" {
					named = f.ExecuteAt(ctx, 0x00090006, f.Registration)
				}
				if candidate.name == "last_activity_of_H" {
					named = f.Created
				}
				decoded, err := codec.DecodeReceipt(named.Encoded)
				require.NoError(t, err)
				require.Equal(t, named.GlobalSequence, decoded.GlobalSequence)
				digest := decoded.Digest()
				rewards := [32]byte(f.Fixture["rewards"])
				caps := []byte{0, 5, 3, 5}
				caps = append(caps, f.Asset[:]...)
				caps = append(caps, rewards[:]...)
				caps = append(caps, make([]byte, 8)...)
				caps = binary.BigEndian.AppendUint64(caps, 100)
				caps = append(caps, 10)
				caps = append(caps, rewards[:]...)
				caps = append(caps, f.Asset[:]...)
				caps = append(caps, digest[:]...)
				caps = append(caps, 7, 8)
				budget := [7]uint64{repair478CPUAllowance, 16777216, 1048576, 1048576, 64, 1048576, 4096}
				fund, err := lp.EncodeCallPayload(repair478Program, 5, "layerx_call", f.Fixture["fund"], caps, []byte("LayerX/programs/access-declaration/v1\x00\x00"), 16384, budget)
				require.NoError(t, err)
				sequence, err := f.Host.ReadIdentitySequence(ctx, f.DID)
				require.NoError(t, err)
				last, err = f.Host.ReadLastSequence(ctx, f.DID)
				require.NoError(t, err)
				var feeLimit [16]byte
				binary.BigEndian.PutUint64(feeLimit[8:], 100000000)
				timestamp := uint64(ctx.BlockTime().UnixMilli())
				activity := lxphost.Activity{
					ProtocolVersion: codec.StateCommitmentProtocolVersion, Type: 0x00090003,
					ActorDID: f.DID, AuthorityKey: f.PublicKey, AccountSequence: sequence,
					NotBefore: timestamp, NotAfter: timestamp, IdempotencyKey: sha256.Sum256(fund),
					Payload: fund, FeeLimit: feeLimit,
					ReceiptAuthorities: []lxphost.ReceiptAuthority{{CanonicalReceipt: named.Encoded, BlockHeight: uint64(f.Context.BlockHeight())}},
				}
				if candidate.name == "anchor_first_in_next_block" {
					activity.ReceiptAuthorities[0].BlockHeight = uint64(ctx.BlockHeight())
				}
				receipt, callErr := f.Host.Execute(lxphost.WithSequence(ctx, last+1), activity, f.Authority)
				var result lxphost.Code
				if callErr != nil {
					require.True(t, errors.As(callErr, &result), "host error: %v", callErr)
				} else {
					result = receipt.ResultCode
				}
				t.Logf("candidate=%s named_sequence=%d next_sequence=%d result=%d (%s)", candidate.name, named.GlobalSequence, last+1, result, result.Error())
				require.Equal(t, candidate.want, result)
				if candidate.name == "kernel_control" {
					require.NoError(t, callErr)
					require.NotNil(t, receipt.ProgramOutcome)
					require.Zero(t, receipt.ProgramOutcome.Status)
					require.Equal(t, uint8(1), receipt.ProgramOutcome.Kind)
					require.Len(t, receipt.Transfers, 1)
					require.Equal(t, f.Asset, receipt.Transfers[0].Asset)
					require.Equal(t, rewards, receipt.Transfers[0].To)
					require.Equal(t, int64(100), receipt.Transfers[0].Amount.Int64())
					require.NotEqual(t, bytes.Repeat([]byte{0}, 32), digest[:])
				}
			})
		})
	}
}

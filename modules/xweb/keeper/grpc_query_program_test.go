package keeper

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestQueryProgramRequestAnswersTheRecordAndDenom(t *testing.T) {
	s := newProgramFulfilSuite(t, 1)
	s.record(programFulfilP, 3, types.KindSearch, programFulfilPayload, sidProgramAsset, sdk.NewInt(75))
	server := NewQueryServer(s.k)
	goCtx := sdk.WrapSDKContext(s.ctx)
	before := s.state()
	answer, err := server.ProgramRequest(goCtx, &types.QueryProgramRequestRequest{
		ProgramId: programFulfilP[:], RequestId: 3,
	})
	require.NoError(t, err)
	require.NotNil(t, answer)
	require.Len(t, answer.Record, 123)
	require.Equal(t, s.ctx.KVStore(s.k.storeKey).Get(ProgramPendingKey(programFulfilP, 3)), answer.Record)
	require.Equal(t, sidProgramAsset.denom, answer.FeeDenom)
	record, err := DecodeProgramPendingRecord(answer.Record)
	require.NoError(t, err)
	require.Equal(t, ProgramPendingRecord{
		Kind: types.KindSearch, PayloadHash: types.Keccak(programFulfilPayload),
		FeeAsset: sidProgramAsset.id, FeeAccount: ProgramFeeAccountID(sidProgramAsset.id),
		Amount: sdk.NewInt(75), Sequence: 7,
	}, record)
	require.Equal(t, before, s.state())

	otherProgram := programFulfilP
	otherProgram[31] ^= 1
	for _, req := range []*types.QueryProgramRequestRequest{
		{ProgramId: programFulfilP[:], RequestId: 4},
		{ProgramId: otherProgram[:], RequestId: 3},
	} {
		answer, err = server.ProgramRequest(goCtx, req)
		require.Nil(t, answer)
		require.Equal(t, codes.NotFound, status.Code(err))
		require.Equal(t, "LXP_UNKNOWN_FIELD", status.Convert(err).Message())
	}
	for _, width := range []int{0, 1, 31, 33, 64} {
		answer, err = server.ProgramRequest(context.Background(), &types.QueryProgramRequestRequest{
			ProgramId: make([]byte, width), RequestId: 3,
		})
		require.Nil(t, answer)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	}
	answer, err = server.ProgramRequest(context.Background(), nil)
	require.Nil(t, answer)
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	require.NoError(t, s.k.FulfilProgram(s.ctx, s.signed(s.observation(programFulfilP, 3, []byte("answer"), 6), s.keys...)))
	answer, err = server.ProgramRequest(goCtx, &types.QueryProgramRequestRequest{
		ProgramId: programFulfilP[:], RequestId: 3,
	})
	require.NoError(t, err)
	require.Equal(t, byte(1), answer.Record[122])
	require.Equal(t, sidProgramAsset.denom, answer.FeeDenom)
	require.Equal(t, s.ctx.KVStore(s.k.storeKey).Get(ProgramPendingKey(programFulfilP, 3)), answer.Record)

	s.ctx.KVStore(s.k.storeKey).Delete(ProgramFeeDenomKey(sidProgramAsset.id))
	answer, err = server.ProgramRequest(goCtx, &types.QueryProgramRequestRequest{ProgramId: programFulfilP[:], RequestId: 3})
	require.Nil(t, answer)
	require.Equal(t, codes.NotFound, status.Code(err))

	s.ctx.KVStore(s.k.storeKey).Set(ProgramPendingKey(programFulfilP, 3), []byte{ProgramPendingRecordVersion})
	before = s.state()
	answer, err = server.ProgramRequest(goCtx, &types.QueryProgramRequestRequest{ProgramId: programFulfilP[:], RequestId: 3})
	require.Nil(t, answer)
	require.Equal(t, codes.DataLoss, status.Code(err))
	require.Equal(t, "LXP_NON_CANONICAL", status.Convert(err).Message())
	require.Equal(t, before, s.state())
}

func TestQueryProgramResultAnswersTheCommittedAnswer(t *testing.T) {
	for _, size := range []int{0, 1, 1024, 2500, types.MaxResponseBytes} {
		t.Run(fmt.Sprintf("response_bytes_%d", size), func(t *testing.T) {
			s := newProgramFulfilSuite(t, 1)
			s.record(programFulfilP, 3, types.KindSearch, programFulfilPayload, paxProgramAsset, sdk.NewInt(75))
			server := NewQueryServer(s.k)
			goCtx := sdk.WrapSDKContext(s.ctx)
			req := &types.QueryProgramResultRequest{ProgramId: programFulfilP[:], RequestId: 3}
			answer, err := server.ProgramResult(goCtx, req)
			require.Nil(t, answer)
			require.Equal(t, codes.NotFound, status.Code(err))
			require.Equal(t, "LXP_UNKNOWN_FIELD", status.Convert(err).Message())

			response := bytes.Repeat([]byte{0xa5}, size)
			observation := s.observation(programFulfilP, 3, response, uint32(size+100))
			require.NoError(t, s.k.FulfilProgram(s.ctx, s.signed(observation, s.keys...)))
			before := s.state()
			answer, err = server.ProgramResult(goCtx, req)
			require.NoError(t, err)
			require.Equal(t, &types.QueryProgramResultResponse{
				ContentDigest: programFulfilDigest, FullLength: uint32(size + 100),
				ResponseLength: uint32(size), Response: response,
			}, answer)
			require.Equal(t, before, s.state())

			otherProgram := programFulfilP
			otherProgram[31] ^= 1
			for _, missing := range []*types.QueryProgramResultRequest{
				{ProgramId: otherProgram[:], RequestId: 3},
				{ProgramId: programFulfilP[:], RequestId: 4},
			} {
				answer, err = server.ProgramResult(goCtx, missing)
				require.Nil(t, answer)
				require.Equal(t, codes.NotFound, status.Code(err))
				require.Equal(t, "LXP_UNKNOWN_FIELD", status.Convert(err).Message())
			}
		})
	}

	s := newProgramFulfilSuite(t, 1)
	server := NewQueryServer(s.k)
	for _, width := range []int{0, 1, 31, 33, 64} {
		answer, err := server.ProgramResult(context.Background(), &types.QueryProgramResultRequest{
			ProgramId: make([]byte, width), RequestId: 3,
		})
		require.Nil(t, answer)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	}
	answer, err := server.ProgramResult(context.Background(), nil)
	require.Nil(t, answer)
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	for _, corruption := range []string{"short_header", "invalid_lengths", "missing_chunk", "long_chunk"} {
		t.Run(corruption, func(t *testing.T) {
			s := newProgramFulfilSuite(t, 1)
			s.record(programFulfilP, 3, types.KindSearch, programFulfilPayload, paxProgramAsset, sdk.NewInt(75))
			observation := s.observation(programFulfilP, 3, bytes.Repeat([]byte{0x42}, 2500), 3000)
			require.NoError(t, s.k.FulfilProgram(s.ctx, s.signed(observation, s.keys...)))
			store := s.ctx.KVStore(s.k.storeKey)
			headerKey := ProgramAnswerKey(programFulfilP, 3, 0)
			chunkKey := ProgramAnswerKey(programFulfilP, 3, 2)
			switch corruption {
			case "short_header":
				store.Set(headerKey, bytes.Clone(store.Get(headerKey)[:39]))
			case "invalid_lengths":
				header := bytes.Clone(store.Get(headerKey))
				binary.BigEndian.PutUint32(header[32:36], 2499)
				store.Set(headerKey, header)
			case "missing_chunk":
				store.Delete(chunkKey)
			case "long_chunk":
				store.Set(chunkKey, append(bytes.Clone(store.Get(chunkKey)), 0))
			}
			before := s.state()
			answer, err := NewQueryServer(s.k).ProgramResult(sdk.WrapSDKContext(s.ctx), &types.QueryProgramResultRequest{
				ProgramId: programFulfilP[:], RequestId: 3,
			})
			require.Nil(t, answer)
			require.Equal(t, codes.DataLoss, status.Code(err))
			require.Equal(t, "LXP_NON_CANONICAL", status.Convert(err).Message())
			require.Equal(t, before, s.state())
		})
	}
}

func TestQueryProgramCountersAnswersTheCounters(t *testing.T) {
	s := newProgramFulfilSuite(t, 1)
	server := NewQueryServer(s.k)
	goCtx := sdk.WrapSDKContext(s.ctx)
	check := func(pending, committed uint64) {
		t.Helper()
		before := s.state()
		answer, err := server.ProgramCounters(goCtx, &types.QueryProgramCountersRequest{})
		require.NoError(t, err)
		require.Equal(t, &types.QueryProgramCountersResponse{
			Pending: pending, Committed: committed, PendingCapacity: 256, CommittedCapacity: 64,
		}, answer)
		require.Equal(t, s.k.ProgramPendingCount(s.ctx), answer.Pending)
		require.Equal(t, s.k.ProgramCommittedCount(s.ctx), answer.Committed)
		require.Equal(t, before, s.state())
	}
	check(0, 0)
	for id := uint64(1); id <= 3; id++ {
		s.record(programFulfilP, id, types.KindSearch, programFulfilPayload, paxProgramAsset, sdk.NewInt(75))
		check(id, 0)
	}
	for id := uint64(1); id <= 2; id++ {
		require.NoError(t, s.k.FulfilProgram(s.ctx, s.signed(s.observation(programFulfilP, id, []byte("answer"), 6), s.keys...)))
		check(3-id, id)
	}
	answer, err := server.ProgramCounters(context.Background(), nil)
	require.Nil(t, answer)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

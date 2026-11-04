package types_test

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/keeper"
	xwebtestutil "github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/testutil"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

// Exercise every wire kind in both pending and fulfilled records. Corrupting
// the kind of an otherwise canonical producer record reproduces the audited
// genesis input without relying on the repaired encoder to admit bad data.
func TestRepair_129_ProgramKindPolicy(t *testing.T) {
	for kind := 0; kind <= 255; kind++ {
		for _, fulfilled := range []bool{false, true} {
			t.Run(fmt.Sprintf("kind_%d/fulfilled_%t", kind, fulfilled), func(t *testing.T) {
				record := keeper.ProgramPendingRecord{
					Kind: types.KindFetch, Amount: sdk.OneInt(), Fulfilled: fulfilled,
					FeeAsset: types.Hash32{9}, Sequence: 7,
				}
				raw, err := record.Bytes()
				require.NoError(t, err)
				raw[1] = uint8(kind)
				record.Kind = uint8(kind)
				encoded, encodeErr := record.Bytes()
				decoded, decodeErr := keeper.DecodeProgramPendingRecord(raw)
				g := types.DefaultGenesis()
				g.ProgramPending = []types.ProgramPendingEntry{{ProgramID: types.Hash32{1}, RequestID: 7, Record: raw}}
				g.ProgramFeeDenoms = []types.ProgramFeeDenomEntry{{Asset: record.FeeAsset, Denom: types.ProgramFeeDenomPAX}}
				if fulfilled {
					g.ProgramCommittedCount = 1
					g.ProgramAnswers = []types.ProgramAnswerEntry{{ProgramID: types.Hash32{1}, RequestID: 7, Value: make([]byte, types.ProgramAnswerHeaderBytes)}}
				} else {
					g.ProgramPendingCount = 1
				}
				genesisJSON, err := json.Marshal(g)
				require.NoError(t, err)
				validateErr := (xweb.AppModuleBasic{}).ValidateGenesis(nil, nil, genesisJSON)
				supported := uint8(kind) == types.KindFetch || uint8(kind) == types.KindSearch
				require.Equal(t, supported, types.ProgramKind(uint8(kind)))
				if supported {
					require.NoError(t, encodeErr)
					require.Equal(t, raw, encoded)
					require.NoError(t, decodeErr)
					require.Equal(t, record, decoded)
					require.NoError(t, validateErr)
				} else {
					require.EqualError(t, encodeErr, "LXP_NON_CANONICAL")
					require.Nil(t, encoded)
					require.EqualError(t, decodeErr, "LXP_NON_CANONICAL")
					require.ErrorIs(t, validateErr, types.ErrInvalidGenesis)
				}
			})
		}
	}
}

// Real request admission, bank transfers and signed fulfilments produce the
// state that the module exports to JSON and imports into a fresh store.
func TestRepair_129_ModuleJSONRoundTrip(t *testing.T) {
	testApp := app.Setup(t, false, false, false)
	ctx := testApp.GetContextForDeliverTx(nil).WithBlockHeight(8)
	k, ctx := xwebtestutil.NewKeeper(testApp, ctx)
	testApp.UpgradeKeeper.SetDone(ctx, keeper.ProgramOriginPlan)
	attestors := xwebtestutil.Attestors(1)
	g := types.DefaultGenesis()
	g.Paused = false
	g.Params.KernelNetworkId = 125
	g.Params.ProgramFeeDenoms = []string{types.ProgramFeeDenomPAX}
	g.Attestors = xwebtestutil.Set(attestors, 1)
	k.InitGenesis(ctx, *g)
	require.True(t, k.ProgramOriginActive(ctx))
	principal := sdk.AccAddress(bytes.Repeat([]byte{0xa1}, 20))
	asset := types.Hash32{9}
	programID := types.Hash32{0x71}
	paid := sdk.NewInt(10)
	funding := sdk.NewCoins(sdk.NewCoin(types.ProgramFeeDenomPAX, paid.MulRaw(4)))
	require.NoError(t, testApp.BankKeeper.MintCoins(ctx, "evm", funding))
	require.NoError(t, testApp.BankKeeper.SendCoinsFromModuleToAccount(ctx, "evm", principal, funding))
	payload := []byte("https://example.com/data")
	response := bytes.Repeat([]byte("answer"), 200)
	for _, kind := range []uint8{types.KindFetch, types.KindSearch} {
		for _, fulfilled := range []bool{false, true} {
			id := uint64(kind) * 2
			if fulfilled {
				id++
			}
			call := &keeper.ProgramCall{
				ProgramID: programID, Principal: principal, Sequence: id,
				Legs: []keeper.ProgramLeg{{From: principal, To: keeper.ProgramFeeAccount(asset),
					Asset: asset, Denom: types.ProgramFeeDenomPAX, Amount: paid}},
			}
			data := binary.BigEndian.AppendUint64(nil, id)
			data = append(data, kind)
			data = binary.BigEndian.AppendUint32(data, uint32(len(payload)))
			data = append(data, payload...)
			cached, write := ctx.CacheContext()
			require.NoError(t, testApp.BankKeeper.SendCoins(cached, principal, keeper.ProgramFeeAccount(asset), sdk.NewCoins(sdk.NewCoin(types.ProgramFeeDenomPAX, paid))))
			require.NoError(t, k.RequestFromProgram(cached, call, []byte(keeper.ProgramRequestTopic), data))
			write()
			if fulfilled {
				observation := types.Observation{
					Origin: types.OriginProgram, NetworkID: 125, ProgramID: programID, RequestID: id,
					Kind: kind, PayloadHash: types.Keccak(payload), ContentDigest: types.Keccak(response),
					FullLength: uint32(len(response)), Response: response,
				}
				digest := types.Digest(types.Attestation{
					Origin: observation.Origin, NetworkID: big.NewInt(125), Requester: programID, RequestID: id,
					Kind: kind, PayloadHash: observation.PayloadHash, ContentDigest: observation.ContentDigest,
					ResponseHash: types.Keccak(response), FullLength: observation.FullLength,
				})
				observation.Signatures = xwebtestutil.Sign(digest, attestors...)
				raw, err := types.EncodeObservation(observation)
				require.NoError(t, err)
				require.NoError(t, k.FulfilProgram(ctx, raw))
			}
		}
	}

	// Origin 1 still admits kind 4 through the actual request handler and
	// preserves it across the same module JSON interface.
	requester := common.HexToAddress("0x0000000000000000000000000000000000000129")
	payer := testApp.EvmKeeper.GetPaxAddressOrDefault(ctx, requester)
	fee := sdk.NewCoins(sdk.NewCoin(types.ProgramFeeDenomPAX, g.Params.Fee))
	require.NoError(t, testApp.BankKeeper.MintCoins(ctx, "evm", fee))
	require.NoError(t, testApp.BankKeeper.SendCoinsFromModuleToAccount(ctx, "evm", payer, fee))
	receipt, err := types.EncodeStorageReceiptPayload(types.StorageReceiptPayload{
		ObjectDigest: types.Keccak([]byte("stored object")), BaseURL: "https://storage.example/bucket",
	})
	require.NoError(t, err)
	id, err := k.Request(ctx, requester, types.KindStorageReceipt, receipt, 100_000, g.Params.Fee)
	require.NoError(t, err)
	originOne, found := k.GetRequest(ctx, id)
	require.True(t, found)
	require.Equal(t, types.KindStorageReceipt, originOne.Kind)
	require.Equal(t, types.Keccak(receipt), originOne.PayloadHash)
	require.NoError(t, originOne.Validate())

	module := xweb.NewAppModule(k)
	exported := module.ExportGenesis(ctx, nil)
	require.NoError(t, module.ValidateGenesis(nil, nil, exported))
	before := k.ExportGenesis(ctx)
	require.Len(t, before.ProgramPending, 4)
	require.Equal(t, uint64(2), before.ProgramPendingCount)
	require.Equal(t, uint64(2), before.ProgramCommittedCount)

	restoredKeeper, restoredCtx := xwebtestutil.NewKeeper(testApp, ctx)
	restoredModule := xweb.NewAppModule(restoredKeeper)
	require.Empty(t, restoredKeeper.ProgramPendingEntries(restoredCtx))
	require.NotPanics(t, func() { restoredModule.InitGenesis(restoredCtx, nil, exported) })
	require.JSONEq(t, string(exported), string(restoredModule.ExportGenesis(restoredCtx, nil)))
	require.Equal(t, before, restoredKeeper.ExportGenesis(restoredCtx))
	restoredOriginOne, found := restoredKeeper.GetRequest(restoredCtx, id)
	require.True(t, found)
	require.Equal(t, originOne, restoredOriginOne)
	for _, entry := range before.ProgramPending {
		expected, err := keeper.DecodeProgramPendingRecord(entry.Record)
		require.NoError(t, err)
		actual, found, err := restoredKeeper.GetProgramPendingRecord(restoredCtx, entry.ProgramID, entry.RequestID)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, expected, actual)
		if actual.Fulfilled {
			result, err := restoredKeeper.GetProgramResult(restoredCtx, entry.ProgramID, entry.RequestID)
			require.NoError(t, err)
			require.Equal(t, response, result.Response)
			require.Equal(t, types.Keccak(response), result.ContentDigest)
			require.Equal(t, uint32(len(response)), result.FullLength)
			require.Equal(t, uint32(len(response)), result.ResponseLength)
		}
	}
}

package layerxpay_test

import (
	"encoding/binary"
	"fmt"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	paytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestRepair_132_DefaultEventLoss(t *testing.T) {
	for _, method := range []string{"serviceAccept", "serviceReject"} {
		for requested := 0; requested < 2; requested++ {
			t.Run(fmt.Sprintf("%s_requested_%d", method, requested), func(t *testing.T) {
				first := newServiceTestFixture(t)
				h := first.h
				serviceTestForm(t, first, 1)
				serviceTestCommit(t, first)
				serviceTestDeliver(t, first)
				second := &serviceTestFixture{
					escrowTestFixture: first.escrowTestFixture,
					offer:             [32]byte{0xa1}, agreement: [32]byte{0xa2}, commitment: [32]byte{0xa3},
					escrow: [32]byte{0xa4}, terms: [32]byte{0xa5}, spec: [32]byte{0xa6}, deadline: first.deadline,
				}
				a := h.precompile.GetABI()
				openArgs := second.openArgs(t, second.escrow)
				openArgs[7] = second.deadline + 10000
				openArgs[9], openArgs[10] = second.terms, second.agreement
				input, err := a.Pack("escrowOpen", openArgs...)
				require.NoError(t, err)
				output, _, err := h.evm.Call(second.owner.caller, h.precompile.Address(), input, 2000000, uint256.NewInt(0))
				require.NoError(t, err, "%x", output)
				serviceTestForm(t, second, 2)
				serviceTestCommit(t, second)
				serviceTestCall(t, second, second.beneficiary.caller, "serviceDeliver", [32]byte{0xa8}, second.agreement,
					[]serviceTestDeliverable{{second.spec, 4096, [32]byte{0xa9}}})

				ids := [][32]byte{first.agreement, second.agreement}
				before := make([]paytypes.ServiceAgreement, len(ids))
				var now uint64
				for i, id := range ids {
					var found bool
					before[i], found = h.keeper.GetAgreement(h.db.Ctx(), id)
					require.True(t, found)
					require.Equal(t, uint8(3), before[i].State)
					require.False(t, before[i].DefaultApplied)
					if before[i].AcceptanceWindowEndMs >= now {
						now = before[i].AcceptanceWindowEndMs + 1
					}
				}
				h.db.WithCtx(h.db.Ctx().WithBlockTime(time.UnixMilli(int64(now))))

				// Produce the reference receipt through the real keeper on a discarded branch.
				branch, _ := h.db.Ctx().CacheContext()
				receipt, err := h.keeper.ServiceApplyDefaultOutcomes(branch)
				require.NoError(t, err)
				require.Zero(t, receipt.ResultCode)
				require.NotZero(t, receipt.GlobalSequence)
				require.Equal(t, now, receipt.Timestamp)
				require.NotEqual(t, receipt.PreviousStateRoot, receipt.ResultingStateRoot)
				require.Len(t, receipt.Events, 2)
				decoded, err := codec.DecodeReceipt(receipt.Encoded)
				require.NoError(t, err)
				require.Equal(t, paytypes.ServiceModuleID, decoded.ModuleID)
				require.Len(t, decoded.Effects, 2)
				for i, id := range ids {
					unchanged, found := h.keeper.GetAgreement(h.db.Ctx(), id)
					require.True(t, found)
					require.Equal(t, before[i], unchanged)
				}

				args := []interface{}{ids[requested]}
				if method == "serviceReject" {
					args = append(args, uint16(7), [][32]byte{first.spec})
				}
				input, err = a.Pack(method, args...)
				require.NoError(t, err)
				logCount := len(h.db.GetAllLogs())
				output, _, err = h.evm.Call(first.owner.caller, h.precompile.Address(), input, 2000000, uint256.NewInt(0))
				require.NoError(t, err, "%x", output)
				require.Empty(t, output)
				logs := h.db.GetAllLogs()[logCount:]
				require.Len(t, logs, 2, "one EVM call must transport both committed default outcomes")
				event := a.Events["DefaultOutcomeApplied"]
				seen := map[[32]byte]bool{}
				committed := make(map[[32]byte]paytypes.ServiceAgreement)
				for i, log := range logs {
					wire := receipt.Events[i]
					require.Equal(t, uint16(14), wire.Code)
					require.Len(t, wire.Body, 50)
					id := [32]byte(wire.Body[:32])
					require.Contains(t, ids, id)
					require.False(t, seen[id])
					seen[id] = true
					require.Equal(t, h.precompile.Address(), log.Address)
					require.Equal(t, []common.Hash{event.ID, common.Hash(id)}, log.Topics)
					values, err := event.Inputs.NonIndexed().Unpack(log.Data)
					require.NoError(t, err)
					agreement, found := h.keeper.GetAgreement(h.db.Ctx(), id)
					require.True(t, found)
					require.Equal(t, uint8(3)+agreement.DefaultOutcome, agreement.State)
					require.True(t, agreement.DefaultApplied)
					require.Equal(t, receipt.GlobalSequence, agreement.OutcomeSequence)
					require.Equal(t, now, agreement.OutcomeTimestampMs)
					require.Equal(t, agreement.State, wire.Body[32])
					require.Equal(t, byte(1), wire.Body[33])
					require.Equal(t, agreement.OutcomeSequence, binary.BigEndian.Uint64(wire.Body[34:42]))
					require.Equal(t, agreement.OutcomeTimestampMs, binary.BigEndian.Uint64(wire.Body[42:50]))
					require.Equal(t, wire.Code, decoded.Effects[i].EventType)
					require.Equal(t, wire.Body, decoded.Effects[i].Body)
					require.Equal(t, []interface{}{agreement.State, true, agreement.OutcomeSequence, agreement.OutcomeTimestampMs}, values)
					committed[id] = agreement
				}
				require.Len(t, seen, 2)

				query := func(id [32]byte) []byte {
					t.Helper()
					input, err := a.Pack("getAgreement", id)
					require.NoError(t, err)
					output, _, err := h.evm.StaticCall(first.owner.caller, h.precompile.Address(), input, 2000000)
					require.NoError(t, err)
					values, err := a.Methods["getAgreement"].Outputs.Unpack(output)
					require.NoError(t, err)
					require.Len(t, values, 1)
					fields := precompileServiceTupleValues(values[0]).([]interface{})
					agreement := committed[id]
					require.Equal(t, id, fields[0])
					require.Equal(t, agreement.DefaultOutcome, fields[9])
					require.Equal(t, agreement.State, fields[10])
					require.Equal(t, true, fields[14])
					require.Equal(t, agreement.OutcomeSequence, fields[15])
					require.Equal(t, agreement.OutcomeTimestampMs, fields[16])
					return output
				}
				queries := map[[32]byte][]byte{}
				for _, id := range ids {
					queries[id] = query(id)
				}
				layerxpayProofReopen(t, h)
				for _, id := range ids {
					recovered, found := h.keeper.GetAgreement(h.db.Ctx(), id)
					require.True(t, found)
					require.Equal(t, committed[id], recovered)
					require.Equal(t, queries[id], query(id))
					require.Equal(t, queries[id], query(id))
				}
			})
		}
	}
}

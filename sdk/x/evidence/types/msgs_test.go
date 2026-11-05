package types_test

import (
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/evidence/exported"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/evidence/types"
)

func testMsgSubmitEvidence(t *testing.T, e exported.Evidence, s sdk.AccAddress) exported.MsgSubmitEvidenceI {
	msg, err := types.NewMsgSubmitEvidence(s, e)
	require.NoError(t, err)
	return msg
}

func TestMsgSubmitEvidence(t *testing.T) {
	pk := ed25519.GenPrivKey()
	submitter := sdk.AccAddress("test________________")

	testCases := []struct {
		msg       sdk.Msg
		submitter sdk.AccAddress
		expectErr bool
	}{
		{
			testMsgSubmitEvidence(t, types.FromABCIEvidence(abci.Evidence{
				Type:      abci.MisbehaviorType_DUPLICATE_VOTE,
				Height:    0,
				Time:      time.Now().UTC(),
				Validator: abci.Validator{Address: pk.PubKey().Address(), Power: 100},
			}), submitter),
			submitter,
			true,
		},
		{
			testMsgSubmitEvidence(t, types.FromABCIEvidence(abci.Evidence{
				Type:      abci.MisbehaviorType_DUPLICATE_VOTE,
				Height:    10,
				Time:      time.Now().UTC(),
				Validator: abci.Validator{Address: pk.PubKey().Address(), Power: 100},
			}), submitter),
			submitter,
			false,
		},
	}

	malformed := types.FromABCIEvidence(abci.Evidence{
		Type:      abci.MisbehaviorType_DUPLICATE_VOTE,
		Height:    10,
		Time:      time.Now().UTC(),
		Validator: abci.Validator{Address: pk.PubKey().Address(), Power: 100},
	}).(*types.Equivocation)
	malformed.ConsensusAddress = pk.PubKey().Address().String()
	testCases = append(testCases, struct {
		msg       sdk.Msg
		submitter sdk.AccAddress
		expectErr bool
	}{testMsgSubmitEvidence(t, malformed, submitter), submitter, true})

	for i, tc := range testCases {
		require.Equal(t, sdk.MsgTypeURL(&types.MsgSubmitEvidence{}), sdk.MsgTypeURL(tc.msg), "unexpected result for tc #%d", i)
		require.Equal(t, tc.expectErr, tc.msg.ValidateBasic() != nil, "unexpected result for tc #%d", i)

		if !tc.expectErr {
			require.Equal(t, tc.msg.GetSigners(), []sdk.AccAddress{tc.submitter}, "unexpected result for tc #%d", i)
			stored := repairM604MessageRoundTrip(t, tc.msg.(*types.MsgSubmitEvidence))
			require.Equal(t, []byte(pk.PubKey().Address()), stored.GetConsensusAddress().Bytes())
		}
	}
}

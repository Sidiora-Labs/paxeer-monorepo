package perps_test

import (
	"encoding/json"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestAppModuleValidateGenesisRunsIndexChecks(t *testing.T) {
	am := perps.NewAppModuleBasic()

	t.Run("default genesis", func(t *testing.T) {
		require.NoError(t, am.ValidateGenesis(nil, nil, am.DefaultGenesis(nil)))
	})

	t.Run("supplied index absent from rebuilt indexes", func(t *testing.T) {
		gs := types.DefaultGenesis()
		account := types.Account{
			Address:           common.HexToAddress("0x0000000000000000000000000000000000000001"),
			MarginRatioBucket: 7,
		}
		gs.Accounts = []types.Account{account}
		gs.MarginRatioEntries = []types.MarginRatioEntry{{Bucket: account.MarginRatioBucket, Account: account.Address}}
		require.NoError(t, types.ValidateGenesis(*gs))
		wantErr := keeper.ValidateGenesis(*gs)
		require.ErrorIs(t, wantErr, types.ErrInvalidGenesis)
		require.Contains(t, wantErr.Error(), "1 margin ratio entries where the records give 0")

		encoded, err := json.Marshal(gs)
		require.NoError(t, err)
		err = am.ValidateGenesis(nil, nil, encoded)
		require.ErrorIs(t, err, types.ErrInvalidGenesis)
		require.EqualError(t, err, wantErr.Error())

		gs.MarginRatioEntries = nil
		encoded, err = json.Marshal(gs)
		require.NoError(t, err)
		require.NoError(t, am.ValidateGenesis(nil, nil, encoded))
	})

	t.Run("malformed genesis preserves wrapped error", func(t *testing.T) {
		err := am.ValidateGenesis(nil, nil, json.RawMessage(`{"params":`))
		require.ErrorIs(t, err, types.ErrInvalidGenesis)
		require.Contains(t, err.Error(), "failed to unmarshal perps genesis state")
		var syntaxErr *json.SyntaxError
		require.ErrorAs(t, err, &syntaxErr)
	})
}

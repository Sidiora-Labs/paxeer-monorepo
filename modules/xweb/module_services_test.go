package xweb_test

import (
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/stretchr/testify/require"
)

func TestModuleServesTheXWebQueryService(t *testing.T) {
	testApp := app.Setup(t, false, false, false)
	const path = "/paxprotocol.paxchain.xweb.Query/Params"
	handler := testApp.GRPCQueryRouter().Route(path)
	require.NotNil(t, handler)

	request, err := (&types.QueryParamsRequest{}).Marshal()
	require.NoError(t, err)
	response, err := handler(testApp.GetContextForDeliverTx([]byte{}), abci.RequestQuery{
		Path: path,
		Data: request,
	})
	require.NoError(t, err)
	var answer types.QueryParamsResponse
	require.NoError(t, answer.Unmarshal(response.Value))
	require.Equal(t, types.DefaultGenesis().Params, answer.Params)

	command := (xweb.AppModuleBasic{}).GetQueryCmd()
	require.NotNil(t, command)
	require.Equal(t, types.ModuleName, command.Name())
}

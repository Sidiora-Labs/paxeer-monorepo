package gov_test

import (
	"encoding/hex"
	"math/big"
	"strings"
	"testing"

	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/ante"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/gov"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	paramproposal "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types/proposal"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

const repairM1267ParameterProposalJSON = `{
	"title": "Gov Param Change",
	"description": "Update quorum to 0.45",
	"type": "ParameterChange",
	"changes": [{
		"subspace": "gov",
		"key": "tallyparams",
		"value": {
			"quorum": "0.45",
			"expedited_quorum": "0.667",
			"threshold": "0.5",
			"expedited_threshold": "0.667",
			"veto_threshold": "0.334"
		}
	}],
	"is_expedited": false
}`

const repairM1267EmptyTypeProposalJSON = `{
	"title": "Default Text Proposal",
	"description": "An empty type defaults to Text even when changes are supplied",
	"type": "",
	"changes": [{"subspace": "gov", "key": "tallyparams", "value": {"quorum": "0.45"}}]
}`

func TestRepairM_1267_ParameterChange(t *testing.T) {
	ctx, proposalID := submitRepairM1267Proposal(t, repairM1267ParameterProposalJSON)
	assertRepairM1267ParameterChange(t, ctx, proposalID)
}

func TestRepairM_1267_EmptyTypeTextDefault(t *testing.T) {
	ctx, proposalID := submitRepairM1267Proposal(t, repairM1267EmptyTypeProposalJSON)
	assertRepairM1267TextDefault(t, ctx, proposalID)
}

func TestRepairM_1267_OmittedTypeTextDefault(t *testing.T) {
	ctx, proposalID := submitRepairM1267Proposal(t, strings.Replace(repairM1267EmptyTypeProposalJSON, `"type": "",`, "", 1))
	assertRepairM1267TextDefault(t, ctx, proposalID)
}

func submitRepairM1267Proposal(t *testing.T, proposalJSON string) (sdk.Context, uint64) {
	t.Helper()
	testApp := testkeeper.EVMTestApp
	ctx, _ := testApp.NewContext(false, tmtypes.Header{}).WithBlockHeight(3).CacheContext()
	k := &testApp.EvmKeeper
	privKey := testkeeper.MockPrivateKey()
	paxAddr, evmAddr := testkeeper.PrivateKeyToAddresses(privKey)
	k.SetAddressMapping(ctx, paxAddr, evmAddr)
	coins := sdk.NewCoins(sdk.NewCoin(k.GetBaseDenom(ctx), sdk.NewInt(20_000_000)))
	require.NoError(t, k.BankKeeper().MintCoins(ctx, evmtypes.ModuleName, coins))
	require.NoError(t, k.BankKeeper().SendCoinsFromModuleToAccount(ctx, evmtypes.ModuleName, paxAddr, coins))
	proposalID, err := testApp.GovKeeper.GetProposalID(ctx)
	require.NoError(t, err)
	before := testApp.GovKeeper.GetTallyParams(ctx)

	abi := pcommon.MustGetABI(f, "abi.json")
	input, err := abi.Pack(gov.SubmitProposalMethod, proposalJSON)
	require.NoError(t, err)
	address := common.HexToAddress(gov.GovAddress)
	key, err := crypto.HexToECDSA(hex.EncodeToString(privKey.Bytes()))
	require.NoError(t, err)
	chainCfg := evmtypes.DefaultChainConfig().EthereumConfig(k.ChainID(ctx))
	signer := ethtypes.MakeSigner(chainCfg, big.NewInt(ctx.BlockHeight()), uint64(ctx.BlockTime().Unix()))
	tx, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.LegacyTx{
		GasPrice: big.NewInt(1_000_000_000_000),
		Gas:      20_000_000,
		To:       &address,
		Value:    big.NewInt(1_000_000_000_000_000_000),
		Data:     input,
	}), signer, key)
	require.NoError(t, err)
	wrapped, err := ethtx.NewLegacyTx(tx)
	require.NoError(t, err)
	msg, err := evmtypes.NewMsgEVMTransaction(wrapped)
	require.NoError(t, err)
	require.NoError(t, ante.Preprocess(ctx, msg, k.ChainID(ctx), false))
	response, err := keeper.NewMsgServerImpl(k).EVMTransaction(sdk.WrapSDKContext(ctx), msg)
	require.NoError(t, err)
	require.NotNil(t, response)
	require.Empty(t, response.VmError)
	output, err := abi.Methods[gov.SubmitProposalMethod].Outputs.Unpack(response.ReturnData)
	require.NoError(t, err)
	require.Equal(t, []interface{}{proposalID}, output)
	require.True(t, before.Equal(testApp.GovKeeper.GetTallyParams(ctx)), "submission must not execute the changes")
	return ctx, proposalID
}

func assertRepairM1267ParameterChange(t *testing.T, ctx sdk.Context, proposalID uint64) {
	t.Helper()
	k := testkeeper.EVMTestApp.GovKeeper
	stored, found := k.GetProposal(ctx, proposalID)
	require.True(t, found)
	content, ok := stored.GetContent().(*paramproposal.ParameterChangeProposal)
	require.True(t, ok, "stored proposal must be ParameterChangeProposal, got %T", stored.GetContent())
	require.Equal(t, "Gov Param Change", content.Title)
	require.Equal(t, "Update quorum to 0.45", content.Description)
	require.False(t, stored.IsExpedited)
	require.Len(t, content.Changes, 1)
	require.Equal(t, govtypes.ModuleName, content.Changes[0].Subspace)
	require.Equal(t, string(govtypes.ParamStoreKeyTallyParams), content.Changes[0].Key)
	require.JSONEq(t, `{"quorum":"0.45","expedited_quorum":"0.667","threshold":"0.5","expedited_threshold":"0.667","veto_threshold":"0.334"}`, content.Changes[0].Value)
	before := k.GetTallyParams(ctx)
	want := govtypes.NewTallyParams(sdk.NewDecWithPrec(45, 2), sdk.NewDecWithPrec(667, 3), sdk.NewDecWithPrec(5, 1), sdk.NewDecWithPrec(667, 3), sdk.NewDecWithPrec(334, 3))
	require.False(t, before.Quorum.Equal(want.Quorum), "fixture must change the existing quorum")
	executionCtx, _ := ctx.CacheContext()
	require.NoError(t, k.Router().GetRoute(stored.ProposalRoute())(executionCtx, content))
	got := k.GetTallyParams(executionCtx)
	require.True(t, want.Equal(got), "unexpected tally params: %s", got.String())
	require.True(t, before.Equal(k.GetTallyParams(ctx)), "execution fixture must remain isolated")
}

func assertRepairM1267TextDefault(t *testing.T, ctx sdk.Context, proposalID uint64) {
	t.Helper()
	k := testkeeper.EVMTestApp.GovKeeper
	stored, found := k.GetProposal(ctx, proposalID)
	require.True(t, found)
	content, ok := stored.GetContent().(*govtypes.TextProposal)
	require.True(t, ok, "empty or omitted type must store TextProposal, got %T", stored.GetContent())
	require.Equal(t, "Default Text Proposal", content.Title)
	require.Equal(t, "An empty type defaults to Text even when changes are supplied", content.Description)
	before := k.GetTallyParams(ctx)
	require.NoError(t, k.Router().GetRoute(stored.ProposalRoute())(ctx, content))
	require.True(t, before.Equal(k.GetTallyParams(ctx)), "Text execution must not apply the supplied changes")
}

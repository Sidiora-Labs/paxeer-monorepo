package rest_test

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gogo/gateway"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	paramkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/keeper"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	stakingkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/keeper"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
)

// The norace suite imports the native node. Check its repaired call here while
// the gateway test below exercises the real keeper without native artifacts.
func TestRepairM_681_NetworkConfig(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "grpc_query_test.go", nil, 0)
	require.NoError(t, err)
	var setup *ast.FuncDecl
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "SetupSuite" {
			setup = fn
		}
	}
	require.NotNil(t, setup)
	calls := 0
	ast.Inspect(setup.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "DefaultConfig" {
			return true
		}
		pkg, ok := selector.X.(*ast.Ident)
		require.True(t, ok)
		require.Equal(t, "network", pkg.Name)
		require.Len(t, call.Args, 1, "DefaultConfig must receive the suite testing handle")
		arg, ok := call.Args[0].(*ast.CallExpr)
		require.True(t, ok)
		require.Empty(t, arg.Args)
		method, ok := arg.Fun.(*ast.SelectorExpr)
		require.True(t, ok)
		require.Equal(t, "T", method.Sel.Name)
		receiver, ok := method.X.(*ast.Ident)
		require.True(t, ok)
		require.Equal(t, setup.Recv.List[0].Names[0].Name, receiver.Name)
		calls++
		return true
	})
	require.Equal(t, 1, calls)
}

func TestRepairM_681_GovernanceGateway(t *testing.T) {
	registry := codectypes.NewInterfaceRegistry()
	authtypes.RegisterInterfaces(registry)
	banktypes.RegisterInterfaces(registry)
	types.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	amino := codec.NewLegacyAmino()
	authtypes.RegisterLegacyAminoCodec(amino)
	banktypes.RegisterLegacyAminoCodec(amino)
	types.RegisterLegacyAminoCodec(amino)
	govKey := sdk.NewKVStoreKey(types.StoreKey)
	authKey := sdk.NewKVStoreKey(authtypes.StoreKey)
	bankKey := sdk.NewKVStoreKey(banktypes.StoreKey)
	stakingKey := sdk.NewKVStoreKey(stakingtypes.StoreKey)
	paramsKey := sdk.NewKVStoreKey(paramtypes.StoreKey)
	transientKey := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	db := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	cms := store.NewCommitMultiStore(db)
	for _, key := range []*sdk.KVStoreKey{govKey, authKey, bankKey, stakingKey, paramsKey} {
		cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	}
	cms.MountStoreWithDB(transientKey, sdk.StoreTypeTransient, db)
	require.NoError(t, cms.LoadLatestVersion())
	ctx := sdk.NewContext(cms, tmproto.Header{Height: 1, Time: time.Unix(100, 0).UTC()}, false)
	pk := paramkeeper.NewKeeper(cdc, amino, paramsKey, transientKey)
	ak := authkeeper.NewAccountKeeper(cdc, authKey, pk.Subspace(authtypes.ModuleName), authtypes.ProtoBaseAccount,
		map[string][]string{
			types.ModuleName:               {authtypes.Burner},
			stakingtypes.BondedPoolName:    {authtypes.Burner, authtypes.Staking},
			stakingtypes.NotBondedPoolName: {authtypes.Burner, authtypes.Staking},
		})
	bk := bankkeeper.NewBaseKeeper(cdc, bankKey, ak, pk.Subspace(banktypes.ModuleName), map[string]bool{})
	sk := stakingkeeper.NewKeeper(cdc, stakingKey, ak, bk, pk.Subspace(stakingtypes.ModuleName))
	sk.SetParams(ctx, stakingtypes.DefaultParams())
	k := keeper.NewKeeper(cdc, govKey, pk.Subspace(types.ModuleName).WithKeyTable(types.ParamKeyTable()),
		ak, bk, sk, pk, types.NewRouter().AddRoute(types.RouterKey, types.ProposalHandler))
	depositParams := types.DefaultDepositParams()
	k.SetDepositParams(ctx, depositParams)
	k.SetVotingParams(ctx, types.DefaultVotingParams())
	k.SetTallyParams(ctx, types.DefaultTallyParams())
	k.SetProposalID(ctx, types.DefaultGenesisState().StartingProposalId)
	address := sdk.AccAddress([]byte("repair-m681-voter___1"))
	ak.SetAccount(ctx, ak.NewAccountWithAddress(ctx, address))
	genesis := banktypes.DefaultGenesisState()
	genesis.Supply = depositParams.MinDeposit.Add(depositParams.MinDeposit...)
	genesis.Balances = []banktypes.Balance{{Address: address.String(), Coins: genesis.Supply}}
	bk.InitGenesis(ctx, genesis)

	proposals := make([]types.Proposal, 3)
	options := []types.WeightedVoteOptions{
		types.NewNonSplitVoteOption(types.OptionYes),
		nil,
		{
			{Option: types.OptionYes, Weight: sdk.NewDecWithPrec(60, 2)},
			{Option: types.OptionNo, Weight: sdk.NewDecWithPrec(30, 2)},
			{Option: types.OptionAbstain, Weight: sdk.NewDecWithPrec(5, 2)},
			{Option: types.OptionNoWithVeto, Weight: sdk.NewDecWithPrec(5, 2)},
		},
	}
	for i := range proposals {
		proposal, err := k.SubmitProposal(ctx, types.NewTextProposal(fmt.Sprintf("Text Proposal %d", i+1), "Gateway regression", false))
		require.NoError(t, err)
		if i != 1 {
			activated, err := k.AddDeposit(ctx, proposal.ProposalId, address, depositParams.MinDeposit)
			require.NoError(t, err)
			require.True(t, activated)
			require.NoError(t, k.AddVote(ctx, proposal.ProposalId, address, options[i]))
		}
		stored, found := k.GetProposal(ctx, proposal.ProposalId)
		require.True(t, found)
		proposals[i] = stored
	}
	queries := baseapp.NewQueryServerTestHelper(ctx, registry)
	types.RegisterQueryServer(queries, k)
	mux := runtime.NewServeMux(
		runtime.WithMarshalerOption(runtime.MIMEWildcard, &gateway.JSONPb{OrigName: true, EmitDefaults: true, AnyResolver: registry}),
		runtime.WithProtoErrorHandler(runtime.DefaultHTTPProtoErrorHandler),
	)
	require.NoError(t, types.RegisterQueryHandlerClient(context.Background(), mux, types.NewQueryClient(queries)))
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	singleVote := types.NewVote(1, address, options[0])
	singleVote.Option = types.OptionYes // The query consumer populates the legacy field.
	get := func(t *testing.T, path string, status int) []byte {
		t.Helper()
		response, err := server.Client().Get(server.URL + "/cosmos/gov/v1beta1/" + path)
		require.NoError(t, err)
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		require.Equal(t, status, response.StatusCode, "%s: %s", path, body)
		return body
	}
	for _, tc := range []struct {
		name, path string
		got, want  codec.ProtoMarshaler
	}{
		{"proposal", "proposals/1", &types.QueryProposalResponse{}, &types.QueryProposalResponse{Proposal: proposals[0]}},
		{"deposit_period", "proposals?proposal_status=1", &types.QueryProposalsResponse{}, &types.QueryProposalsResponse{Proposals: proposals[1:2]}},
		{"vote", "proposals/1/votes/" + address.String(), &types.QueryVoteResponse{}, &types.QueryVoteResponse{Vote: singleVote}},
		{"split_vote", "proposals/3/votes/" + address.String(), &types.QueryVoteResponse{}, &types.QueryVoteResponse{Vote: types.NewVote(3, address, options[2])}},
		{"votes", "proposals/1/votes", &types.QueryVotesResponse{}, &types.QueryVotesResponse{Votes: []types.Vote{singleVote}}},
		{"deposit", "proposals/1/deposits/" + address.String(), &types.QueryDepositResponse{}, &types.QueryDepositResponse{Deposit: types.NewDeposit(1, address, depositParams.MinDeposit)}},
		{"deposits", "proposals/1/deposits", &types.QueryDepositsResponse{}, &types.QueryDepositsResponse{Deposits: []types.Deposit{types.NewDeposit(1, address, depositParams.MinDeposit)}}},
		{"deposit_params", "params/deposit", &types.QueryParamsResponse{}, &types.QueryParamsResponse{DepositParams: depositParams}},
		{"voting_params", "params/voting", &types.QueryParamsResponse{}, &types.QueryParamsResponse{VotingParams: types.DefaultVotingParams()}},
		{"tally_params", "params/tallying", &types.QueryParamsResponse{}, &types.QueryParamsResponse{TallyParams: types.DefaultTallyParams()}},
		{"tally", "proposals/2/tally", &types.QueryTallyResultResponse{}, &types.QueryTallyResultResponse{Tally: types.EmptyTallyResult()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := get(t, tc.path, http.StatusOK)
			require.NoError(t, cdc.UnmarshalAsJSON(body, tc.got))
			// Pagination metadata is asserted separately below.
			switch got := tc.got.(type) {
			case *types.QueryProposalsResponse:
				require.NotNil(t, got.Pagination)
				require.Empty(t, got.Pagination.NextKey)
				require.Zero(t, got.Pagination.Total)
				got.Pagination = nil
			case *types.QueryVotesResponse:
				require.NotNil(t, got.Pagination)
				require.Empty(t, got.Pagination.NextKey)
				require.Zero(t, got.Pagination.Total)
				got.Pagination = nil
			case *types.QueryDepositsResponse:
				require.NotNil(t, got.Pagination)
				require.Empty(t, got.Pagination.NextKey)
				require.Zero(t, got.Pagination.Total)
				got.Pagination = nil
			}
			wantBytes, err := cdc.Marshal(tc.want)
			require.NoError(t, err)
			gotBytes, err := cdc.Marshal(tc.got)
			require.NoError(t, err)
			require.Equal(t, wantBytes, gotBytes)
		})
	}
	t.Run("all_proposals", func(t *testing.T) {
		var got types.QueryProposalsResponse
		require.NoError(t, cdc.UnmarshalAsJSON(get(t, "proposals?pagination.count_total=true", http.StatusOK), &got))
		require.Len(t, got.Proposals, len(proposals))
		for i := range proposals {
			wantBytes, err := cdc.Marshal(&proposals[i])
			require.NoError(t, err)
			gotBytes, err := cdc.Marshal(&got.Proposals[i])
			require.NoError(t, err)
			require.Equal(t, wantBytes, gotBytes)
		}
		require.NotNil(t, got.Pagination)
		require.Equal(t, uint64(3), got.Pagination.Total)
		require.Empty(t, got.Pagination.NextKey)
	})
	for _, tc := range []struct {
		path         string
		status, code int
	}{
		{"proposals/0", http.StatusBadRequest, 3},
		{"proposals/10", http.StatusNotFound, 5},
		{"proposals/1/votes/wrongVoterAddress", http.StatusInternalServerError, 2},
		{"proposals/1/deposits/wrongDepositorAddress", http.StatusInternalServerError, 2},
		{"params/invalid", http.StatusBadRequest, 3},
	} {
		t.Run(tc.path, func(t *testing.T) {
			body := get(t, tc.path, tc.status)
			var got struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			}
			require.NoError(t, json.Unmarshal(body, &got))
			require.Equal(t, tc.code, got.Code)
			require.NotEmpty(t, got.Message)
		})
	}
}

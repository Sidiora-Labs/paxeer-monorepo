package baseapp_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	storev2rootmulti "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/storev2/rootmulti"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/query"
	txtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	distrkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/distribution/keeper"
	distrtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/distribution/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	stakingkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/keeper"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	storageconfig "github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	wasmapp "github.com/Sidiora-Labs/Paxeer-X-Network/wasm/app"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/encoding"
	grpcstatus "google.golang.org/grpc/status"
)

var errQueryUnknownMarket = sdkerrors.Register("baseapptest", 12, "unknown market")

func newIssue1678QueryApp(t *testing.T, decoder sdk.TxDecoder) *baseapp.BaseApp {
	t.Helper()
	scConfig := storageconfig.DefaultStateCommitConfig()
	scConfig.MemIAVLConfig.SnapshotInterval = 1
	scConfig.MemIAVLConfig.SnapshotMinTimeInterval = 0
	scConfig.MemIAVLConfig.AsyncCommitBuffer = 0
	cms := storev2rootmulti.NewStore(t.TempDir(), scConfig, storageconfig.StateStoreConfig{}, nil)
	app := baseapp.NewBaseApp(t.Name(), dbm.NewMemDB(), decoder, nil, &testutil.TestAppOpts{})
	app.SetCMS(cms)
	t.Cleanup(func() { require.NoError(t, app.Close()) })
	return app
}

func TestHandleQueryGRPCKeepsModuleCodespace(t *testing.T) {
	app := newIssue1678QueryApp(t, nil)
	app.MountStores(sdk.NewKVStoreKey("query"))
	require.NoError(t, app.LoadLatestVersion())
	_, err := app.InitChain(context.Background(), &abci.RequestInitChain{})
	require.NoError(t, err)
	ctx := app.NewContext(false, tmproto.Header{ChainID: app.ChainID, Height: 1})
	require.NoError(t, app.SetSealedStateToCommit(ctx, ctx.MultiStore().(sdk.CacheMultiStore)))
	_, err = app.Commit(context.Background())
	require.NoError(t, err)
	ctx, err = app.CreateQueryContext(1, false)
	require.NoError(t, err)
	require.Equal(t, int64(1), ctx.BlockHeight())

	testCases := []struct {
		name      string
		err       error
		codespace string
		code      uint32
		log       string
	}{
		{"registered module error", errQueryUnknownMarket, "baseapptest", 12, "unknown market"},
		{"wrapped module error", sdkerrors.Wrap(errQueryUnknownMarket, "query market"), "baseapptest", 12, "query market: unknown market"},
		{"plain error", errors.New("plain private backend detail"), sdkerrors.UndefinedCodespace, 1, "internal error"},
		{"grpc internal", grpcstatus.Error(codes.Internal, "private database connection"), sdkerrors.UndefinedCodespace, 1, "internal error"},
		{"grpc unknown", grpcstatus.Error(codes.Unknown, "private query failure"), sdkerrors.UndefinedCodespace, 1, "internal error"},
		{"grpc not found", grpcstatus.Error(codes.NotFound, "missing private key"), "sdk", sdkerrors.ErrKeyNotFound.ABCICode(), "key not found"},
		{"grpc invalid argument", grpcstatus.Error(codes.InvalidArgument, "invalid private parameter"), "sdk", sdkerrors.ErrInvalidRequest.ABCICode(), "invalid request"},
		{"grpc failed precondition", grpcstatus.Error(codes.FailedPrecondition, "private precondition"), "sdk", sdkerrors.ErrInvalidRequest.ABCICode(), "invalid request"},
		{"grpc unauthenticated", grpcstatus.Error(codes.Unauthenticated, "private credentials"), "sdk", sdkerrors.ErrUnauthorized.ABCICode(), "unauthorized"},
		{"grpc unimplemented", grpcstatus.Error(codes.Unimplemented, "private service"), "sdk", sdkerrors.ErrUnknownRequest.ABCICode(), "unknown request"},
	}

	for _, trace := range []bool{false, true} {
		baseapp.SetTrace(trace)(app)
		for i, tc := range testCases {
			t.Run(fmt.Sprintf("%s/trace=%t", tc.name, trace), func(t *testing.T) {
				path := fmt.Sprintf("/baseapptest.Query%dTrace%t/Error", i, trace)
				app.GRPCQueryRouter().RegisterService(&grpc.ServiceDesc{
					ServiceName: path[1 : len(path)-len("/Error")],
					Methods: []grpc.MethodDesc{{
						MethodName: "Error",
						Handler: func(_ interface{}, _ context.Context, _ func(interface{}) error, _ grpc.UnaryServerInterceptor) (interface{}, error) {
							return nil, tc.err
						},
					}},
				}, nil)
				_, routedErr := app.GRPCQueryRouter().Route(path)(ctx, abci.RequestQuery{Height: 1})
				require.Same(t, tc.err, routedErr)
				res, err := app.Query(context.Background(), &abci.RequestQuery{Path: path, Height: 1})
				require.NoError(t, err)
				require.Equal(t, tc.codespace, res.Codespace)
				require.Equal(t, tc.code, res.Code)
				require.Equal(t, int64(1), res.Height)
				expectedLog := tc.log
				if trace {
					expectedLog = fmt.Sprintf("%+v", tc.err)
				}
				require.Equal(t, expectedLog, res.Log)
				require.Empty(t, res.Value)
				require.Nil(t, res.ProofOps)
			})
		}
	}
}

func TestIssue1678GRPCQueryRedaction(t *testing.T) {
	encodingConfig := wasmapp.MakeEncodingConfig()
	registry := encodingConfig.InterfaceRegistry
	cdc := encodingConfig.Marshaler
	protoCodec := encoding.GetCodec("proto")
	app := newIssue1678QueryApp(t, encodingConfig.TxConfig.TxDecoder())
	keys := sdk.NewKVStoreKeys(authtypes.StoreKey, banktypes.StoreKey, stakingtypes.StoreKey, distrtypes.StoreKey, paramtypes.StoreKey)
	tkeys := sdk.NewTransientStoreKeys(paramtypes.TStoreKey)
	for _, key := range keys {
		app.MountStores(key)
	}
	app.MountTransientStores(tkeys)
	require.NoError(t, app.LoadLatestVersion())

	subspace := func(module string) paramtypes.Subspace {
		return paramtypes.NewSubspace(cdc, codec.NewLegacyAmino(), keys[paramtypes.StoreKey], tkeys[paramtypes.TStoreKey], module)
	}
	ak := authkeeper.NewAccountKeeper(cdc, keys[authtypes.StoreKey], subspace(authtypes.ModuleName), authtypes.ProtoBaseAccount, map[string][]string{
		stakingtypes.BondedPoolName: nil, stakingtypes.NotBondedPoolName: nil, distrtypes.ModuleName: nil,
	})
	bk := bankkeeper.NewBaseKeeper(cdc, keys[banktypes.StoreKey], ak, subspace(banktypes.ModuleName), nil)
	sk := stakingkeeper.NewKeeper(cdc, keys[stakingtypes.StoreKey], ak, bk, subspace(stakingtypes.ModuleName))
	dk := distrkeeper.NewKeeper(cdc, keys[distrtypes.StoreKey], subspace(distrtypes.ModuleName), ak, bk, sk, authtypes.FeeCollectorName, nil)
	app.GRPCQueryRouter().SetInterfaceRegistry(registry)
	authtypes.RegisterQueryServer(app.GRPCQueryRouter(), ak)
	distrtypes.RegisterQueryServer(app.GRPCQueryRouter(), dk)
	serviceApp := &wasmapp.WasmApp{BaseApp: app}
	serviceApp.RegisterLocalServices(nil, encodingConfig.TxConfig)

	address := sdk.AccAddress([]byte("query-redaction-user"))
	missingAddress := sdk.AccAddress([]byte("query-missing-user!!"))
	validatorAddress := sdk.ValAddress(address)
	account := authtypes.NewBaseAccountWithAddress(address)
	validator, err := stakingtypes.NewValidator(validatorAddress, ed25519.GenPrivKey().PubKey(), stakingtypes.Description{})
	require.NoError(t, err)
	_, err = app.InitChain(context.Background(), &abci.RequestInitChain{})
	require.NoError(t, err)
	ctx := app.NewContext(false, tmproto.Header{ChainID: app.ChainID, Height: 1})
	ak.SetAccount(ctx, account)
	sk.SetValidator(ctx, validator)
	require.NoError(t, app.SetSealedStateToCommit(ctx, ctx.MultiStore().(sdk.CacheMultiStore)))
	_, err = app.Commit(context.Background())
	require.NoError(t, err)

	accountAny, err := codectypes.NewAnyWithValue(account)
	require.NoError(t, err)
	successValue, err := protoCodec.Marshal(&authtypes.QueryAccountResponse{Account: accountAny})
	require.NoError(t, err)
	_, parseErr := sdk.AccAddressFromBech32("private-backend-address")
	require.Error(t, parseErr)
	simulateRequest := &txtypes.SimulateRequest{TxBytes: []byte{0xff}}
	simulateData, err := protoCodec.Marshal(simulateRequest)
	require.NoError(t, err)
	queryCtx, err := app.CreateQueryContext(1, false)
	require.NoError(t, err)
	_, simulateErr := app.GRPCQueryRouter().Route("/cosmos.tx.v1beta1.Service/Simulate")(queryCtx, abci.RequestQuery{Data: simulateData, Height: 1})
	require.Equal(t, codes.Unknown, grpcstatus.Code(simulateErr))

	testCases := []struct {
		name       string
		path       string
		request    interface{}
		data       []byte
		codespace  string
		code       uint32
		log        string
		diagnostic string
		value      []byte
	}{
		{name: "protobuf decoder failure", path: "/cosmos.auth.v1beta1.Query/Account", data: []byte{0x0a, 0xff}, codespace: sdkerrors.UndefinedCodespace, code: 1, log: "internal error", diagnostic: io.ErrUnexpectedEOF.Error()},
		{name: "unknown address parse failure", path: "/cosmos.auth.v1beta1.Query/Account", request: &authtypes.QueryAccountRequest{Address: "private-backend-address"}, codespace: sdkerrors.UndefinedCodespace, code: 1, log: "internal error", diagnostic: parseErr.Error()},
		{name: "grpc internal pagination failure", path: "/cosmos.auth.v1beta1.Query/Accounts", request: &authtypes.QueryAccountsRequest{Pagination: &query.PageRequest{Key: []byte("private-key"), Offset: 1}}, codespace: sdkerrors.UndefinedCodespace, code: 1, log: "internal error", diagnostic: grpcstatus.Error(codes.Internal, "paginate: invalid request, either offset or key is expected, got both").Error()},
		{name: "grpc unknown simulation failure", path: "/cosmos.tx.v1beta1.Service/Simulate", request: simulateRequest, codespace: sdkerrors.UndefinedCodespace, code: 1, log: "internal error", diagnostic: simulateErr.Error()},
		{name: "client invalid argument", path: "/cosmos.auth.v1beta1.Query/Account", request: &authtypes.QueryAccountRequest{}, codespace: "sdk", code: sdkerrors.ErrInvalidRequest.ABCICode(), log: "invalid request", diagnostic: grpcstatus.Error(codes.InvalidArgument, "Address cannot be empty").Error()},
		{name: "client not found", path: "/cosmos.auth.v1beta1.Query/Account", request: &authtypes.QueryAccountRequest{Address: missingAddress.String()}, codespace: "sdk", code: sdkerrors.ErrKeyNotFound.ABCICode(), log: "key not found", diagnostic: grpcstatus.Errorf(codes.NotFound, "account %s not found", missingAddress).Error()},
		{name: "registered distribution error", path: "/cosmos.distribution.v1beta1.Query/DelegationRewards", request: &distrtypes.QueryDelegationRewardsRequest{DelegatorAddress: address.String(), ValidatorAddress: validatorAddress.String()}, codespace: distrtypes.ModuleName, code: distrtypes.ErrNoDelegationExists.ABCICode(), log: "delegation does not exist", diagnostic: "delegation does not exist"},
		{name: "success", path: "/cosmos.auth.v1beta1.Query/Account", request: &authtypes.QueryAccountRequest{Address: address.String()}, value: successValue},
	}

	for _, trace := range []bool{false, true} {
		baseapp.SetTrace(trace)(app)
		for _, tc := range testCases {
			t.Run(fmt.Sprintf("%s/trace=%t", tc.name, trace), func(t *testing.T) {
				data := tc.data
				if tc.request != nil {
					var err error
					data, err = protoCodec.Marshal(tc.request)
					require.NoError(t, err)
				}
				require.NotNil(t, app.GRPCQueryRouter().Route(tc.path))
				response, err := app.Query(context.Background(), &abci.RequestQuery{Path: tc.path, Data: data})
				require.NoError(t, err)
				require.NotNil(t, response)
				wire, err := response.Marshal()
				require.NoError(t, err)
				var decoded abci.ResponseQuery
				require.NoError(t, decoded.Unmarshal(wire))
				require.Equal(t, *response, decoded)
				require.Equal(t, tc.codespace, decoded.Codespace)
				require.Equal(t, tc.code, decoded.Code)
				require.Equal(t, int64(1), decoded.Height)
				expectedLog := tc.log
				if trace {
					expectedLog = tc.diagnostic
				}
				require.Equal(t, expectedLog, decoded.Log)
				require.Equal(t, tc.value, decoded.Value)
				require.Nil(t, decoded.ProofOps)
				if !trace && tc.diagnostic != tc.log {
					require.NotContains(t, string(wire), tc.diagnostic)
				}
				if tc.code == 0 {
					var result authtypes.QueryAccountResponse
					require.NoError(t, protoCodec.Unmarshal(decoded.Value, &result))
					require.Equal(t, accountAny.TypeUrl, result.Account.TypeUrl)
					require.Equal(t, accountAny.Value, result.Account.Value)
					var decodedAccount authtypes.AccountI
					require.NoError(t, cdc.UnpackAny(result.Account, &decodedAccount))
					require.Equal(t, account, decodedAccount)
				}
			})
		}
	}
}

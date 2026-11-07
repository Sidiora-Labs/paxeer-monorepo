package keeper_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	rpchttp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/http"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	rpcserver "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
	paxapp "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	cryptotypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/api"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/bech32/legacybech32"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/slashing"
	slashingcli "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/slashing/client/cli"
	slashingrest "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/slashing/client/rest"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/slashing/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/slashing/testslashing"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/slashing/types"
	stakingkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/keeper"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type repair284Store struct {
	cms      sdk.CommitMultiStore
	ctx      sdk.Context
	k        keeper.Keeper
	staking  stakingkeeper.Keeper
	bank     bankkeeper.BaseKeeper
	cdc      codec.Codec
	amino    *codec.LegacyAmino
	registry codectypes.InterfaceRegistry
}

func openRepair284Store(t *testing.T, dir string) *repair284Store {
	t.Helper()
	db, err := dbm.NewGoLevelDB("slashing", dir)
	require.NoError(t, err)
	cms := store.NewCommitMultiStore(db)
	keys := sdk.NewKVStoreKeys(types.StoreKey, stakingtypes.StoreKey, authtypes.StoreKey, banktypes.StoreKey, paramtypes.StoreKey)
	for _, key := range keys {
		cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, nil)
	}
	tkey := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	cms.MountStoreWithDB(tkey, sdk.StoreTypeTransient, nil)
	require.NoError(t, cms.LoadLatestVersion())
	ctx := sdk.NewContext(cms, tmproto.Header{Height: 1, Time: time.Unix(1000, 0).UTC()}, false)
	enc := paxapp.MakeEncodingConfig()
	subspace := func(name string) paramtypes.Subspace {
		return paramtypes.NewSubspace(enc.Marshaler, enc.Amino, keys[paramtypes.StoreKey], tkey, name)
	}
	ak := authkeeper.NewAccountKeeper(enc.Marshaler, keys[authtypes.StoreKey], subspace(authtypes.ModuleName), authtypes.ProtoBaseAccount,
		map[string][]string{stakingtypes.BondedPoolName: {authtypes.Minter, authtypes.Burner}, stakingtypes.NotBondedPoolName: {authtypes.Burner}})
	bk := bankkeeper.NewBaseKeeper(enc.Marshaler, keys[banktypes.StoreKey], ak, subspace(banktypes.ModuleName), map[string]bool{})
	sk := stakingkeeper.NewKeeper(enc.Marshaler, keys[stakingtypes.StoreKey], ak, bk, subspace(stakingtypes.ModuleName))
	k := keeper.NewKeeper(enc.Marshaler, keys[types.StoreKey], sk, subspace(types.ModuleName))
	if cms.LastCommitID().Version == 0 {
		ak.SetParams(ctx, authtypes.DefaultParams())
		bk.SetParams(ctx, banktypes.DefaultParams())
		sk.SetParams(ctx, stakingtypes.DefaultParams())
		k.SetParams(ctx, types.DefaultParams())
	}
	return &repair284Store{cms, ctx, k, sk, bk, enc.Marshaler, enc.Amino, enc.InterfaceRegistry}
}

func (s *repair284Store) addValidator(t *testing.T) cryptotypes.PubKey {
	t.Helper()
	pk := ed25519.GenPrivKey().PubKey()
	v, err := stakingtypes.NewValidator(sdk.ValAddress(pk.Address()), pk, stakingtypes.Description{})
	require.NoError(t, err)
	v.Status = stakingtypes.Bonded
	v.Tokens = s.staking.TokensFromConsensusPower(s.ctx, 100)
	v.DelegatorShares = v.Tokens.ToDec()
	s.staking.SetValidator(s.ctx, v)
	require.NoError(t, s.staking.SetValidatorByConsAddr(s.ctx, v))
	s.staking.SetValidatorByPowerIndex(s.ctx, v)
	require.NoError(t, s.bank.MintCoins(s.ctx, stakingtypes.BondedPoolName, sdk.NewCoins(sdk.NewCoin(s.staking.BondDenom(s.ctx), v.Tokens))))
	require.NoError(t, s.k.AfterValidatorCreated(s.ctx, v.GetOperator()))
	s.k.AfterValidatorBonded(s.ctx, sdk.ConsAddress(pk.Address()), v.GetOperator())
	return pk
}

func (s *repair284Store) vote(pk cryptotypes.PubKey, signed bool) {
	s.ctx = s.ctx.WithBlockHeight(s.ctx.BlockHeight() + 1)
	slashing.BeginBlocker(s.ctx, testslashing.CreateBeginBlockReq(pk.Address(), 100, signed).LastCommitInfo.Votes, s.k)
}

func (s *repair284Store) assertWindow(t *testing.T, pk cryptotypes.PubKey, expected []bool, index int64) {
	t.Helper()
	addr := sdk.ConsAddress(pk.Address())
	info, found := s.k.GetValidatorSigningInfo(s.ctx, addr)
	require.True(t, found)
	missed, found := s.k.GetValidatorMissedBlocks(s.ctx, addr)
	require.True(t, found)
	require.Equal(t, int64(len(expected)), missed.WindowSize)
	require.Equal(t, expected, s.k.ParseBitGroupsToBoolArray(missed.MissedBlocks, missed.WindowSize))
	var count int64
	for _, bit := range expected {
		if bit {
			count++
		}
	}
	require.Equal(t, count, info.MissedBlocksCounter)
	require.Equal(t, index, info.IndexOffset)
}

func TestRepair_284_WindowExpansionRestartPenalty(t *testing.T) {
	for _, old := range []int64{1, 5, 65} {
		t.Run(fmt.Sprintf("window_%d", old), func(t *testing.T) {
			dir := t.TempDir()
			s := openRepair284Store(t, dir)
			params := s.k.GetParams(s.ctx)
			params.SignedBlocksWindow = old
			params.MinSignedPerWindow = sdk.ZeroDec()
			params.SlashFractionDowntime = sdk.NewDecWithPrec(1, 2)
			s.k.SetParams(s.ctx, params)
			pks := make([]cryptotypes.PubKey, old)
			original := make([]bool, old)
			for i := range original {
				original[i] = int64(i) == old-1 || i%3 == 0
			}
			for offset := int64(0); offset < old; offset++ {
				pks[offset] = s.addValidator(t)
				for n := int64(0); n < old+offset; n++ {
					s.vote(pks[offset], !original[n%old])
				}
				s.assertWindow(t, pks[offset], original, offset)
			}
			window := old + 3
			params.SignedBlocksWindow = window
			params.MinSignedPerWindow = sdk.NewDecWithPrec(5, 1)
			s.k.SetParams(s.ctx, params)
			expected := make([][]bool, old)
			for offset, pk := range pks {
				bits := make([]bool, window)
				copy(bits[:offset], original[:offset])
				copy(bits[offset+3:], original[offset:])
				expected[offset] = bits
				s.vote(pk, true)
				s.assertWindow(t, pk, bits, (int64(offset)+1)%window)
			}
			commit := s.cms.Commit(true)
			require.Greater(t, commit.Version, int64(0))
			require.NoError(t, s.cms.Close())
			s = openRepair284Store(t, dir)
			defer func() { require.NoError(t, s.cms.Close()) }()
			require.Equal(t, commit, s.cms.LastCommitID())
			require.Equal(t, params, s.k.GetParams(s.ctx))
			s.ctx = s.ctx.WithBlockHeight(10000)
			for offset, pk := range pks {
				s.assertWindow(t, pk, expected[offset], (int64(offset)+1)%window)
				before, found := s.staking.GetValidatorByConsAddr(s.ctx, sdk.ConsAddress(pk.Address()))
				require.True(t, found)
				require.False(t, before.Jailed)
				for n := int64(0); n < window; n++ {
					s.vote(pk, true)
					index := (int64(offset) + 1 + n) % window
					expected[offset][index] = false
					s.assertWindow(t, pk, expected[offset], (index+1)%window)
					v, ok := s.staking.GetValidatorByConsAddr(s.ctx, sdk.ConsAddress(pk.Address()))
					require.True(t, ok)
					require.False(t, v.Jailed)
					require.Equal(t, before.Tokens, v.Tokens)
				}
				maxMissed := window - s.k.MinSignedPerWindow(s.ctx)
				for n := int64(0); n < maxMissed; n++ {
					s.vote(pk, false)
					index := (int64(offset) + 1 + n) % window
					expected[offset][index] = true
					s.assertWindow(t, pk, expected[offset], (index+1)%window)
					v, ok := s.staking.GetValidatorByConsAddr(s.ctx, sdk.ConsAddress(pk.Address()))
					require.True(t, ok)
					require.False(t, v.Jailed)
					require.Equal(t, before.Tokens, v.Tokens)
				}
				s.vote(pk, false)
				v, ok := s.staking.GetValidatorByConsAddr(s.ctx, sdk.ConsAddress(pk.Address()))
				require.True(t, ok)
				require.True(t, v.Jailed)
				require.Equal(t, before.Tokens.Sub(s.staking.TokensFromConsensusPower(s.ctx, 1)), v.Tokens)
				info, found := s.k.GetValidatorSigningInfo(s.ctx, sdk.ConsAddress(pk.Address()))
				require.True(t, found)
				require.Zero(t, info.MissedBlocksCounter)
				require.Zero(t, info.IndexOffset)
				require.Equal(t, s.ctx.BlockTime().Add(params.DowntimeJailDuration), info.JailedUntil)
				_, found = s.k.GetValidatorMissedBlocks(s.ctx, sdk.ConsAddress(pk.Address()))
				require.False(t, found)
			}
		})
	}
}

func TestRepair_284_LegacyConsumerEncoding(t *testing.T) {
	s := openRepair284Store(t, t.TempDir())
	defer func() { require.NoError(t, s.cms.Close()) }()
	pk := s.addValidator(t)
	s.vote(pk, false)
	addr := sdk.ConsAddress(pk.Address())
	want, found := s.k.GetValidatorSigningInfo(s.ctx, addr)
	require.True(t, found)
	legacy := keeper.NewQuerier(s.k, s.amino)
	queryHelper := baseapp.NewQueryServerTestHelper(s.ctx, s.registry)
	types.RegisterQueryServer(queryHelper, s.k)
	queryClient := types.NewQueryClient(queryHelper)

	// The actual CLI emits a protobuf request over the real JSON-RPC client.
	// The server routes it to the real gRPC router and sends the same decoded
	// request through the legacy codec and querier, proving their key agreement.
	transport := http.NewServeMux()
	rpcserver.RegisterRPCFuncs(transport, map[string]*rpcserver.RPCFunc{
		"abci_query": rpcserver.NewRPCFunc(func(_ context.Context, req *coretypes.RequestABCIQuery) (*coretypes.ResultABCIQuery, error) {
			route := queryHelper.Route(req.Path)
			require.NotNil(t, route)
			var input types.QuerySigningInfoRequest
			require.NoError(t, s.cdc.Unmarshal(req.Data, &input))
			_, err := sdk.ConsAddressFromBech32(input.ConsAddress)
			require.NoError(t, err)
			data, err := s.amino.MarshalAsJSON(input)
			require.NoError(t, err)
			output, err := legacy(s.ctx, []string{types.QuerySigningInfo}, abci.RequestQuery{Data: data})
			if input.ConsAddress == addr.String() {
				require.NoError(t, err)
				var actual types.ValidatorSigningInfo
				require.NoError(t, s.amino.UnmarshalAsJSON(output, &actual))
				require.Equal(t, want, actual)
			} else {
				require.ErrorIs(t, err, types.ErrNoSigningInfoFound)
				require.Nil(t, output)
			}
			response, err := route(s.ctx, abci.RequestQuery{Data: req.Data, Height: int64(req.Height)})
			return &coretypes.ResultABCIQuery{Response: response}, err
		}),
	})
	rpc := httptest.NewServer(transport)
	defer rpc.Close()
	node, err := rpchttp.New(rpc.URL)
	require.NoError(t, err)
	var output bytes.Buffer
	clientCtx := client.Context{}.WithClient(node).WithCodec(s.cdc).WithLegacyAmino(s.amino).WithInterfaceRegistry(s.registry).WithOutput(&output).WithOutputFormat("json")
	cmd := slashingcli.GetCmdQuerySigningInfo()
	cmd.SetContext(context.WithValue(context.Background(), client.ClientContextKey, &client.Context{}))
	require.NoError(t, client.SetCmdClientContext(cmd, clientCtx))
	pubJSON, err := s.cdc.MarshalInterfaceJSON(pk)
	require.NoError(t, err)
	cmd.SetArgs([]string{string(pubJSON)})
	require.NoError(t, cmd.Execute())
	var cliInfo types.ValidatorSigningInfo
	require.NoError(t, s.cdc.UnmarshalAsJSON(output.Bytes(), &cliInfo))
	require.Equal(t, want, cliInfo)

	grpcInfo, err := queryClient.SigningInfo(context.Background(), &types.QuerySigningInfoRequest{ConsAddress: addr.String()})
	require.NoError(t, err)
	require.Equal(t, want, grpcInfo.ValSigningInfo)
	gateway := api.New(clientCtx).GRPCGatewayRouter
	require.NoError(t, types.RegisterQueryHandlerClient(context.Background(), gateway, queryClient))
	recorder := httptest.NewRecorder()
	gateway.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/cosmos/slashing/v1beta1/signing_infos/"+addr.String(), nil))
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var restInfo types.QuerySigningInfoResponse
	require.NoError(t, s.cdc.UnmarshalAsJSON(recorder.Body.Bytes(), &restInfo))
	require.Equal(t, want, restInfo.ValSigningInfo)

	unknownPK := ed25519.GenPrivKey().PubKey()
	unknown := sdk.ConsAddress(unknownPK.Address()).String()
	unknownJSON, err := s.cdc.MarshalInterfaceJSON(unknownPK)
	require.NoError(t, err)
	for _, input := range []string{string(unknownJSON), "malformed"} {
		cmd := slashingcli.GetCmdQuerySigningInfo()
		cmd.SetContext(context.WithValue(context.Background(), client.ClientContextKey, &client.Context{}))
		require.NoError(t, client.SetCmdClientContext(cmd, clientCtx))
		cmd.SetArgs([]string{input})
		require.Error(t, cmd.Execute())
	}
	for _, address := range []string{"", "malformed", pk.Address().String(), sdk.AccAddress(pk.Address()).String(), addr.String()[:len(addr.String())-1], unknown} {
		t.Run(fmt.Sprintf("address_%s", address), func(t *testing.T) {
			data, err := s.amino.MarshalAsJSON(types.QuerySigningInfoRequest{ConsAddress: address})
			require.NoError(t, err)
			result, legacyErr := legacy(s.ctx, []string{types.QuerySigningInfo}, abci.RequestQuery{Data: data})
			require.Nil(t, result)
			_, grpcErr := queryClient.SigningInfo(context.Background(), &types.QuerySigningInfoRequest{ConsAddress: address})
			require.Error(t, grpcErr)
			recorder := httptest.NewRecorder()
			gateway.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/cosmos/slashing/v1beta1/signing_infos/"+address, nil))
			require.NotEqual(t, http.StatusOK, recorder.Code)
			if address == unknown {
				require.ErrorIs(t, legacyErr, types.ErrNoSigningInfoFound)
				require.Equal(t, codes.NotFound, status.Code(grpcErr))
				require.Equal(t, http.StatusNotFound, recorder.Code)
			} else {
				require.ErrorIs(t, legacyErr, sdkerrors.ErrInvalidAddress)
				require.False(t, errors.Is(legacyErr, types.ErrNoSigningInfoFound))
			}
		})
	}
	result, err := legacy(s.ctx, []string{types.QuerySigningInfo}, abci.RequestQuery{Data: []byte("{")})
	require.Nil(t, result)
	require.ErrorIs(t, err, sdkerrors.ErrJSONUnmarshal)

	// The out-of-scope legacy REST producer still emits hex. Exercise it and
	// record the required owner wiring, rather than silently accepting hex.
	legacyTransport := http.NewServeMux()
	rpcserver.RegisterRPCFuncs(legacyTransport, map[string]*rpcserver.RPCFunc{
		"abci_query": rpcserver.NewRPCFunc(func(_ context.Context, req *coretypes.RequestABCIQuery) (*coretypes.ResultABCIQuery, error) {
			require.Equal(t, "custom/slashing/signingInfo", req.Path)
			var input types.QuerySigningInfoRequest
			require.NoError(t, s.amino.UnmarshalAsJSON(req.Data, &input))
			require.Equal(t, pk.Address().String(), input.ConsAddress)
			_, err := legacy(s.ctx, []string{types.QuerySigningInfo}, abci.RequestQuery{Data: req.Data})
			require.ErrorIs(t, err, sdkerrors.ErrInvalidAddress)
			codeSpace, code, log := sdkerrors.ABCIInfo(err, false)
			return &coretypes.ResultABCIQuery{Response: abci.ResponseQuery{Codespace: codeSpace, Code: code, Log: log}}, nil
		}),
	})
	legacyRPC := httptest.NewServer(legacyTransport)
	defer legacyRPC.Close()
	legacyNode, err := rpchttp.New(legacyRPC.URL)
	require.NoError(t, err)
	legacyREST := mux.NewRouter()
	slashingrest.RegisterHandlers(clientCtx.WithClient(legacyNode), legacyREST)
	pubBech32, err := legacybech32.MarshalPubKey(legacybech32.ConsPK, pk)
	require.NoError(t, err)
	recorder = httptest.NewRecorder()
	legacyREST.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/slashing/validators/"+pubBech32+"/signing_info", nil))
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.True(t, strings.Contains(recorder.Body.String(), "invalid address"), recorder.Body.String())
}

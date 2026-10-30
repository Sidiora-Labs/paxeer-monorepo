package keeper_test

import (
	"bytes"
	"crypto/ed25519"
	moduleimpl "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/cachemulti"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/dbadapter"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/proto"

	"encoding/json"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/testvectors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

const (
	bond           = int64(5_000_000)
	unbondingDelay = uint64(1000)
)

type suite struct {
	t         *testing.T
	app       *app.App
	ctx       sdk.Context
	k         keeper.Keeper
	authority sdk.AccAddress
	reporter  sdk.AccAddress
	operators []sdk.AccAddress
	ids       [][32]byte
	signers   [][20]byte
	fixture   testvectors.Fixture
}

func account(tag byte) sdk.AccAddress {
	raw := make([]byte, 20)
	for i := range raw {
		raw[i] = tag
	}
	return sdk.AccAddress(raw)
}

func (s *suite) fund(to sdk.AccAddress, amount int64) {
	s.t.Helper()
	coins := sdk.NewCoins(sdk.NewCoin("uhpx", sdk.NewInt(amount)))
	require.NoError(s.t, s.app.BankKeeper.MintCoins(s.ctx, "evm", coins))
	require.NoError(s.t, s.app.BankKeeper.SendCoinsFromModuleToAccount(s.ctx, "evm", to, coins))
}

func (s *suite) balance(of sdk.AccAddress) sdk.Int {
	return s.app.BankKeeper.GetBalance(s.ctx, of, "uhpx").Amount
}

func newSuite(t *testing.T, mutate func(*types.Params)) *suite {
	t.Helper()
	testApp := app.Setup(t, false, false, false)
	ctx := testApp.GetContextForDeliverTx([]byte{}).WithBlockHeight(8).WithBlockTime(time.Unix(1_800_000_000, 0))
	stores := make(map[storetypes.StoreKey]storetypes.CacheWrapper)
	keys := make(map[string]storetypes.StoreKey)
	for _, key := range ctx.MultiStore().StoreKeys() {
		if key.Name() == types.StoreKey {
			continue
		}
		stores[key] = ctx.MultiStore().GetKVStore(key)
		keys[key.Name()] = key
	}
	anchorKey := sdk.NewKVStoreKey(types.StoreKey)
	anchorDB := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, anchorDB.Close()) })
	stores[anchorKey] = dbadapter.Store{DB: anchorDB}
	keys[anchorKey.Name()] = anchorKey
	cacheDB := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, cacheDB.Close()) })
	ctx = ctx.WithMultiStore(cachemulti.NewStore(cacheDB, stores, keys, nil, nil, nil, 0))
	testApp.LayerXAnchorKeeper = keeper.NewKeeper(anchorKey, testApp.AccountKeeper, testApp.BankKeeper)
	return initializeSuite(t, testApp, ctx, mutate)
}

func initializeSuite(t *testing.T, testApp *app.App, ctx sdk.Context, mutate func(*types.Params)) *suite {
	t.Helper()
	s := &suite{t: t, app: testApp, ctx: ctx, k: testApp.LayerXAnchorKeeper, authority: account(0xa1), reporter: account(0xa2)}
	params := types.DefaultParams(s.authority.String())
	params.Threshold = 2
	params.UnbondingDelaySeconds = unbondingDelay
	if mutate != nil {
		mutate(&params)
	}
	require.NoError(t, s.k.SetParams(s.ctx, params))

	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x51}, ed25519.SeedSize))
	var publicKey [32]byte
	copy(publicKey[:], key.Public().(ed25519.PublicKey))
	authorization := types.SequencerAuthorization{SequencerID: types.Hash32{1}, PublicKey: publicKey, FirstBatchNumber: 1, LastBatchNumber: 8}
	require.ErrorIs(t, s.k.SetSequencerAuthorization(s.ctx, s.reporter, authorization), types.ErrUnauthorized)
	require.NoError(t, s.k.SetSequencerAuthorization(s.ctx, s.authority, authorization))

	for index := 0; index < 3; index++ {
		id, signer := [32]byte{byte(index + 1)}, [20]byte{byte(index + 1)}
		operator := account(byte(0xb0 + index))
		s.fund(operator, 2*bond)
		s.ids, s.signers, s.operators = append(s.ids, id), append(s.signers, signer), append(s.operators, operator)
	}
	return s
}

func (s *suite) bondAll() {
	s.t.Helper()
	for index := range s.ids {
		registered, err := s.k.RegisterGuarantor(s.ctx, s.operators[index], s.ids[index], s.signers[index], sdk.NewInt(bond))
		require.NoError(s.t, err)
		require.Equal(s.t, types.GuarantorPending, registered.Status)
		require.NoError(s.t, s.k.ActivateGuarantor(s.ctx, s.authority, s.ids[index]))
	}
	s.invariant()
}

func (s *suite) invariant() {
	s.t.Helper()
	message, broken := keeper.BalanceInvariant(s.k)(s.ctx)
	require.False(s.t, broken, message)
}

func (s *suite) vector(section, name string) testvectors.Vector {
	s.t.Helper()
	for _, v := range s.fixture[section] {
		if v.Name == name {
			return v
		}
	}
	s.t.Fatalf("no vector %s/%s", section, name)
	return testvectors.Vector{}
}

func (s *suite) submit(name string) (types.Checkpoint, error) {
	s.t.Helper()
	v := s.vector("checkpoints", name)
	header, err := v.Bytes("header")
	require.NoError(s.t, err)
	raw, err := v.Bytes("header_signature")
	require.NoError(s.t, err)
	var signature [64]byte
	copy(signature[:], raw)
	certificate, err := v.Bytes("certificate")
	require.NoError(s.t, err)
	return s.k.SubmitCheckpoint(s.ctx, s.reporter, header, signature, certificate)
}

func (s *suite) attestation(name string) []byte {
	s.t.Helper()
	raw, err := s.vector("attestations", name).Bytes("attestation")
	require.NoError(s.t, err)
	return raw
}

func (s *suite) advance(seconds uint64) {
	s.ctx = s.ctx.WithBlockTime(s.ctx.BlockTime().Add(time.Duration(seconds) * time.Second)).WithBlockHeight(s.ctx.BlockHeight() + 1)
}

func filter(entries []types.UnbondingEntry, id [32]byte) []types.UnbondingEntry {
	var out []types.UnbondingEntry
	for _, entry := range entries {
		if entry.GuarantorID == types.Hash32(id) {
			out = append(out, entry)
		}
	}
	return out
}

func TestDefaultGenesisIsValid(t *testing.T) {
	require.NoError(t, types.DefaultGenesis().Validate())
	bad := types.DefaultGenesis()
	bad.Params.Threshold = 0
	require.Error(t, bad.Validate())
	bad = types.DefaultGenesis()
	bad.HasLatestFinalized = true
	require.Error(t, bad.Validate())
}

func parameterRouter(t *testing.T, k keeper.Keeper, upgrades keeper.UpgradeReader) (*baseapp.MsgServiceRouter, *codec.ProtoCodec) {
	t.Helper()
	registry := cdctypes.NewInterfaceRegistry()
	sdk.RegisterInterfaces(registry)
	moduleimpl.AppModuleBasic{}.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	router := baseapp.NewMsgServiceRouter()
	router.SetInterfaceRegistry(registry)
	moduleimpl.NewAppModule(k, upgrades).RegisterServices(module.NewConfigurator(cdc, router, baseapp.NewGRPCQueryRouter()))
	return router, cdc
}
func parameterRoute(t *testing.T, router *baseapp.MsgServiceRouter, cdc *codec.ProtoCodec, ctx sdk.Context, msg *types.MsgUpdateParams) error {
	t.Helper()
	raw, err := cdc.MarshalInterface(msg)
	require.NoError(t, err)
	var decoded sdk.Msg
	require.NoError(t, cdc.UnmarshalInterface(raw, &decoded))
	require.Equal(t, "/paxprotocol.paxchain.layerxanchor.MsgUpdateParams", sdk.MsgTypeURL(decoded))
	raw2, err := cdc.MarshalInterface(decoded)
	require.NoError(t, err)
	require.Equal(t, raw, raw2)
	handler := router.Handler(decoded)
	require.NotNil(t, handler)
	_, err = handler(ctx, decoded)
	return err
}
func TestMsgUpdateParamsGovernanceRoute(t *testing.T) {
	s := newSuite(t, nil)
	k, ctx := s.k, s.ctx
	authority := s.authority.String()
	router, cdc := parameterRouter(t, k, &s.app.UpgradeKeeper)
	params := k.GetParams(ctx)
	params.ReporterShare = sdk.NewDecWithPrec(2, 1)
	params.PermissionlessRegistration = !params.PermissionlessRegistration
	require.NoError(t, parameterRoute(t, router, cdc, ctx, &types.MsgUpdateParams{Authority: authority, Params: params}))
	require.Equal(t, params, k.GetParams(ctx))
	eventResult, eventErr := router.Handler(&types.MsgUpdateParams{})(ctx, &types.MsgUpdateParams{Authority: authority, Params: params})
	require.NoError(t, eventErr)
	events := eventResult.Events
	require.NotEmpty(t, events)
	require.Equal(t, types.EventParamsUpdated, events[len(events)-1].Type)
	response, err := keeper.NewMsgServerImpl(k).UpdateParams(sdk.WrapSDKContext(ctx), &types.MsgUpdateParams{Authority: authority, Params: params})
	require.NoError(t, err)
	require.NotNil(t, response)
}
func TestMsgUpdateParamsCodecAndSigners(t *testing.T) {
	s := newSuite(t, nil)
	k, ctx := s.k, s.ctx
	authority := s.authority.String()
	_, cdc := parameterRouter(t, k, &s.app.UpgradeKeeper)
	msg := &types.MsgUpdateParams{Authority: authority, Params: k.GetParams(ctx)}
	require.NoError(t, msg.ValidateBasic())
	signer, err := sdk.AccAddressFromBech32(authority)
	require.NoError(t, err)
	require.Equal(t, []sdk.AccAddress{signer}, msg.GetSigners())
	require.Equal(t, types.RouterKey, msg.Route())
	require.Equal(t, types.TypeMsgUpdateParams, msg.Type())
	require.Equal(t, sdk.MustSortJSON(types.ModuleCdc.MustMarshalJSON(msg)), msg.GetSignBytes())
	raw, err := proto.Marshal(msg)
	require.NoError(t, err)
	var decoded types.MsgUpdateParams
	require.NoError(t, proto.Unmarshal(raw, &decoded))
	require.Equal(t, *msg, decoded)
	raw, err = cdc.MarshalAsJSON(msg)
	require.NoError(t, err)
	require.NoError(t, cdc.UnmarshalAsJSON(raw, &decoded))
	require.Equal(t, *msg, decoded)
	amino := codec.NewLegacyAmino()
	sdk.RegisterLegacyAminoCodec(amino)
	moduleimpl.AppModuleBasic{}.RegisterLegacyAminoCodec(amino)
	raw, err = amino.MarshalAsJSON(msg)
	require.NoError(t, err)
	require.NoError(t, amino.UnmarshalAsJSON(raw, &decoded))
	require.Equal(t, *msg, decoded)
	invalid := *msg
	invalid.Params.SlashFractionAvailability = sdk.Dec{}
	require.Error(t, invalid.ValidateBasic())
	_, err = keeper.NewMsgServerImpl(k).UpdateParams(sdk.WrapSDKContext(ctx), &invalid)
	require.Error(t, err)
	require.Equal(t, msg.Params, k.GetParams(ctx))
	preserved := msg.Params
	require.Error(t, preserved.UnmarshalJSONPB(&jsonpb.Unmarshaler{}, []byte(`{"unknown_field":true}`)))
	require.Equal(t, msg.Params, preserved)
	raw, err = json.Marshal(msg.Params)
	require.NoError(t, err)
	require.Error(t, preserved.UnmarshalJSONPB(nil, append(raw, []byte(` {}`)...)))
	require.Equal(t, msg.Params, preserved)
	address := types.AnchorPrecompileAddress
	saved := address
	for _, length := range []int{0, 19, 21} {
		require.Error(t, address.Unmarshal(make([]byte, length)))
		require.Equal(t, saved, address)
	}
	_, err = address.MarshalTo(make([]byte, 19))
	require.Error(t, err)
	encoded, err := address.Marshal()
	require.NoError(t, err)
	require.Len(t, encoded, 20)
	var other types.Address20
	require.NoError(t, other.Unmarshal(encoded))
	require.Equal(t, address, other)
}
func TestMsgUpdateParamsRefusals(t *testing.T) {
	s := newSuite(t, nil)
	k, ctx := s.k, s.ctx
	authority := s.authority.String()
	router, cdc := parameterRouter(t, k, &s.app.UpgradeKeeper)
	before := k.GetParams(ctx)
	outsider := sdk.AccAddress(bytes.Repeat([]byte{0xfe}, 20)).String()
	for _, caller := range []string{outsider, "invalid-bech32"} {
		msg := &types.MsgUpdateParams{Authority: caller, Params: before}
		require.Error(t, parameterRoute(t, router, cdc, ctx, msg))
		_, err := keeper.NewMsgServerImpl(k).UpdateParams(sdk.WrapSDKContext(ctx), msg)
		require.Error(t, err)
		require.Equal(t, before, k.GetParams(ctx))
		if caller == "invalid-bech32" {
			require.Empty(t, msg.GetSigners())
			require.Error(t, msg.ValidateBasic())
		}
	}
	cases := []struct {
		name   string
		mutate func(*types.Params)
	}{
		{"authority", func(p *types.Params) { p.Authority = "invalid" }},
		{"denom", func(p *types.Params) { p.BondDenom = "!" }},
		{"threshold-zero", func(p *types.Params) { p.Threshold = 0 }},
		{"threshold-high", func(p *types.Params) { p.Threshold = 33 }},
		{"min-bond-nil", func(p *types.Params) { p.MinBond = sdk.Int{} }},
		{"min-bond-zero", func(p *types.Params) { p.MinBond = sdk.ZeroInt() }},
		{"challenge-bond-negative", func(p *types.Params) { p.ChallengeBond = sdk.NewInt(-1) }},
		{"attestation-delay", func(p *types.Params) { p.MaxAttestationDelayMs = 0 }},
		{"settlement", func(p *types.Params) { p.SettlementContract = types.Address20{} }},
		{"equivocation", func(p *types.Params) { p.SlashFractionEquivocation = sdk.NewDec(2) }},
		{"fraud", func(p *types.Params) { p.SlashFractionFraud = sdk.NewDec(-1) }},
		{"availability", func(p *types.Params) { p.SlashFractionAvailability = sdk.NewDec(-1) }},
		{"reporter", func(p *types.Params) { p.ReporterShare = sdk.NewDec(2) }},
		{"destination", func(p *types.Params) { p.SlashDestination = "invalid" }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			params := before
			test.mutate(&params)
			msg := &types.MsgUpdateParams{Authority: authority, Params: params}
			require.Error(t, msg.ValidateBasic())
			require.Error(t, parameterRoute(t, router, cdc, ctx, msg))
			_, err := keeper.NewMsgServerImpl(k).UpdateParams(sdk.WrapSDKContext(ctx), msg)
			require.Error(t, err)
			require.Equal(t, before, k.GetParams(ctx))
		})
	}
	_, err := keeper.NewMsgServerImpl(k).UpdateParams(sdk.WrapSDKContext(ctx), nil)
	require.Error(t, err)
	require.Equal(t, before, k.GetParams(ctx))
}

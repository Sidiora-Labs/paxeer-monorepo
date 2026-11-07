package api_test

import (
	"bytes"
	"encoding/json"
	"testing"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/stretchr/testify/require"
	tmdb "github.com/tendermint/tm-db"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/types/descriptorpb"
)

type repairM1207Chain struct {
	store   storetypes.CommitMultiStore
	ctx     sdk.Context
	cdc     *codec.ProtoCodec
	bank    bankkeeper.BaseKeeper
	factory keeper.Keeper
	module  tokenfactory.AppModule
}

func openRepairM1207Chain(t *testing.T, dir string) repairM1207Chain {
	t.Helper()
	db, err := tmdb.NewGoLevelDB("genesis", dir)
	require.NoError(t, err)
	ms := store.NewCommitMultiStore(db)
	keys := sdk.NewKVStoreKeys(types.StoreKey, authtypes.StoreKey, banktypes.StoreKey, paramtypes.StoreKey)
	for _, key := range keys {
		ms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	}
	transient := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	ms.MountStoreWithDB(transient, sdk.StoreTypeTransient, nil)
	require.NoError(t, ms.LoadLatestVersion())
	registry := codectypes.NewInterfaceRegistry()
	authtypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	subspace := func(name string) paramtypes.Subspace {
		return paramtypes.NewSubspace(cdc, codec.NewLegacyAmino(), keys[paramtypes.StoreKey], transient, name)
	}
	accounts := authkeeper.NewAccountKeeper(cdc, keys[authtypes.StoreKey], subspace(authtypes.ModuleName), authtypes.ProtoBaseAccount,
		map[string][]string{types.ModuleName: {authtypes.Minter, authtypes.Burner}})
	bank := bankkeeper.NewBaseKeeper(cdc, keys[banktypes.StoreKey], accounts, subspace(banktypes.ModuleName), map[string]bool{})
	factory := keeper.NewKeeper(cdc, keys[types.StoreKey], subspace(types.ModuleName), accounts, bank)
	ctx := sdk.NewContext(ms, tmproto.Header{Height: 1}, false)
	accounts.SetParams(ctx, authtypes.DefaultParams())
	bank.SetParams(ctx, banktypes.DefaultParams())
	return repairM1207Chain{store: ms, ctx: ctx, cdc: cdc, bank: bank, factory: factory,
		module: tokenfactory.NewAppModule(factory, accounts, bank)}
}

func repairM1207WireFields(t *testing.T, wire []byte) map[protowire.Number][][]byte {
	t.Helper()
	fields := make(map[protowire.Number][][]byte)
	for len(wire) > 0 {
		number, kind, n := protowire.ConsumeTag(wire)
		require.Positive(t, n, "invalid protobuf tag")
		require.Equal(t, protowire.BytesType, kind, "GenesisState field %d wire type", number)
		wire = wire[n:]
		value, n := protowire.ConsumeBytes(wire)
		require.Positive(t, n, "invalid GenesisState field %d", number)
		fields[number] = append(fields[number], value)
		wire = wire[n:]
	}
	return fields
}

func TestRepairM_1207_TokenfactoryGenesisRegistry(t *testing.T) {
	// Parse the committed source in-process, without buf or a subprocess toolchain.
	parser := protoparse.Parser{ImportPaths: []string{"."}}
	sources, err := parser.ParseFilesButDoNotLink("tokenfactory/genesis.proto")
	require.NoError(t, err)
	require.Len(t, sources, 1)
	generated := registeredFile(t, "tokenfactory/genesis.proto")
	for _, file := range []*descriptorpb.FileDescriptorProto{sources[0], generated} {
		require.Equal(t, "paxprotocol.paxchain.tokenfactory", file.GetPackage())
		var genesis *descriptorpb.DescriptorProto
		for _, message := range file.GetMessageType() {
			if message.GetName() == "GenesisState" {
				genesis = message
			}
		}
		require.NotNil(t, genesis)
		require.Len(t, genesis.GetField(), 3, "complete GenesisState signature")
		for i, want := range []struct {
			name     string
			kind     descriptorpb.FieldDescriptorProto_Type
			label    descriptorpb.FieldDescriptorProto_Label
			typeName string
		}{
			{"params", descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL, "Params"},
			{"factory_denoms", descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, descriptorpb.FieldDescriptorProto_LABEL_REPEATED, "GenesisDenom"},
			{"paused_denoms", descriptorpb.FieldDescriptorProto_TYPE_STRING, descriptorpb.FieldDescriptorProto_LABEL_REPEATED, ""},
		} {
			field := genesis.GetField()[i]
			require.Equal(t, want.name, field.GetName())
			require.Equal(t, int32(i+1), field.GetNumber())
			if file == generated || want.typeName == "" {
				require.Equal(t, want.kind, field.GetType())
			}
			if file == generated && want.typeName != "" {
				require.Equal(t, ".paxprotocol.paxchain.tokenfactory."+want.typeName, field.GetTypeName())
			} else {
				require.Equal(t, want.typeName, field.GetTypeName())
			}
			require.Equal(t, want.label, field.GetLabel())
		}
	}

	dir := t.TempDir()
	source := openRepairM1207Chain(t, dir)
	source.factory.InitGenesis(source.ctx, *types.DefaultGenesis())
	creator := sdk.AccAddress(bytes.Repeat([]byte{0x71}, 20)).String()
	server := keeper.NewMsgServerImpl(source.factory)
	paused, err := server.CreateDenom(sdk.WrapSDKContext(source.ctx), types.NewMsgCreateDenom(creator, "paused"))
	require.NoError(t, err)
	active, err := server.CreateDenom(sdk.WrapSDKContext(source.ctx), types.NewMsgCreateDenom(creator, "active"))
	require.NoError(t, err)
	_, err = server.SetDenomPaused(sdk.WrapSDKContext(source.ctx), types.NewMsgSetDenomPaused(creator, paused.NewTokenDenom, true))
	require.NoError(t, err)
	source.factory.SetParams(source.ctx, types.Params{DenomAllowlistMaxSize: 17})
	expected := source.factory.ExportGenesis(source.ctx)
	require.Equal(t, []string{paused.NewTokenDenom}, expected.GetPausedDenoms())
	commit := source.store.Commit(true)
	require.NoError(t, source.store.Close())

	producer := openRepairM1207Chain(t, dir)
	defer func() { require.NoError(t, producer.store.Close()) }()
	require.Equal(t, commit, producer.store.LastCommitID())
	require.Equal(t, expected, producer.factory.ExportGenesis(producer.ctx))
	jsonWire := producer.module.ExportGenesis(producer.ctx, producer.cdc)
	var jsonFields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(jsonWire, &jsonFields))
	var pausedJSON []string
	require.NoError(t, json.Unmarshal(jsonFields["paused_denoms"], &pausedJSON))
	require.Equal(t, expected.GetPausedDenoms(), pausedJSON)
	var message types.GenesisState
	require.NoError(t, producer.cdc.UnmarshalAsJSON(jsonWire, &message))
	wire, err := message.Marshal()
	require.NoError(t, err)
	fields := repairM1207WireFields(t, wire)
	require.Len(t, fields, 3)
	require.Len(t, fields[1], 1)
	require.Len(t, fields[2], 2)
	require.Equal(t, [][]byte{[]byte(paused.NewTokenDenom)}, fields[3])
	var received types.GenesisState
	require.NoError(t, received.Unmarshal(wire))
	require.Equal(t, expected, &received)

	targetDir := t.TempDir()
	target := openRepairM1207Chain(t, targetDir)
	consumerJSON := target.cdc.MustMarshalJSON(&received)
	require.NoError(t, target.module.ValidateGenesis(target.cdc, nil, consumerJSON))
	require.Empty(t, target.module.InitGenesis(target.ctx, target.cdc, consumerJSON))
	targetCommit := target.store.Commit(true)
	require.NoError(t, target.store.Close())
	consumer := openRepairM1207Chain(t, targetDir)
	defer func() { require.NoError(t, consumer.store.Close()) }()
	require.Equal(t, targetCommit, consumer.store.LastCommitID())
	require.Equal(t, expected, consumer.factory.ExportGenesis(consumer.ctx))
	require.Equal(t, uint32(17), consumer.factory.GetDenomAllowListMaxSize(consumer.ctx))
	query, err := consumer.factory.DenomPaused(sdk.WrapSDKContext(consumer.ctx), &types.QueryDenomPausedRequest{Denom: paused.NewTokenDenom})
	require.NoError(t, err)
	require.True(t, query.Paused)
	require.False(t, consumer.factory.IsDenomPaused(consumer.ctx, active.NewTokenDenom))
	consumerServer := keeper.NewMsgServerImpl(consumer.factory)
	_, err = consumerServer.Mint(sdk.WrapSDKContext(consumer.ctx), types.NewMsgMint(creator, sdk.NewInt64Coin(paused.NewTokenDenom, 9)))
	require.ErrorIs(t, err, types.ErrDenomPaused)
	require.True(t, consumer.bank.GetBalance(consumer.ctx, sdk.MustAccAddressFromBech32(creator), paused.NewTokenDenom).IsZero())
	_, err = consumerServer.Mint(sdk.WrapSDKContext(consumer.ctx), types.NewMsgMint(creator, sdk.NewInt64Coin(active.NewTokenDenom, 9)))
	require.NoError(t, err)
	require.Equal(t, sdk.NewInt(9), consumer.bank.GetBalance(consumer.ctx, sdk.MustAccAddressFromBech32(creator), active.NewTokenDenom).Amount)
}

func TestRepairM_1207_AbsentPausedDenoms(t *testing.T) {
	source := openRepairM1207Chain(t, t.TempDir())
	defer func() { require.NoError(t, source.store.Close()) }()
	source.factory.InitGenesis(source.ctx, *types.DefaultGenesis())
	creator := sdk.AccAddress(bytes.Repeat([]byte{0x72}, 20)).String()
	created, err := keeper.NewMsgServerImpl(source.factory).CreateDenom(sdk.WrapSDKContext(source.ctx), types.NewMsgCreateDenom(creator, "legacy"))
	require.NoError(t, err)
	produced := source.factory.ExportGenesis(source.ctx)
	require.Empty(t, produced.GetPausedDenoms())
	wire, err := produced.Marshal()
	require.NoError(t, err)
	fields := repairM1207WireFields(t, wire)
	require.Len(t, fields[1], 1)
	require.Len(t, fields[2], 1)
	require.NotContains(t, fields, protowire.Number(3))
	var received types.GenesisState
	require.NoError(t, received.Unmarshal(wire))
	require.Empty(t, received.GetPausedDenoms())
	require.Equal(t, produced.GetParams(), received.GetParams())
	require.Equal(t, produced.GetFactoryDenoms(), received.GetFactoryDenoms())

	// Omit only the empty optional JSON field from real producer output.
	var fieldsJSON map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(source.module.ExportGenesis(source.ctx, source.cdc), &fieldsJSON))
	delete(fieldsJSON, "paused_denoms")
	legacyJSON, err := json.Marshal(fieldsJSON)
	require.NoError(t, err)
	var legacy types.GenesisState
	require.NoError(t, source.cdc.UnmarshalAsJSON(legacyJSON, &legacy))
	require.Empty(t, legacy.GetPausedDenoms())
	require.Equal(t, received.GetFactoryDenoms(), legacy.GetFactoryDenoms())
	for _, payload := range [][]byte{source.cdc.MustMarshalJSON(&received), legacyJSON} {
		target := openRepairM1207Chain(t, t.TempDir())
		require.NoError(t, target.module.ValidateGenesis(target.cdc, nil, payload))
		require.Empty(t, target.module.InitGenesis(target.ctx, target.cdc, payload))
		require.Empty(t, target.factory.ExportGenesis(target.ctx).GetPausedDenoms())
		query, err := target.factory.DenomPaused(sdk.WrapSDKContext(target.ctx), &types.QueryDenomPausedRequest{Denom: created.NewTokenDenom})
		require.NoError(t, err)
		require.False(t, query.Paused)
		_, err = keeper.NewMsgServerImpl(target.factory).Mint(sdk.WrapSDKContext(target.ctx), types.NewMsgMint(creator, sdk.NewInt64Coin(created.NewTokenDenom, 11)))
		require.NoError(t, err)
		require.Equal(t, sdk.NewInt(11), target.bank.GetBalance(target.ctx, sdk.MustAccAddressFromBech32(creator), created.NewTokenDenom).Amount)
		require.NoError(t, target.store.Close())
	}
}

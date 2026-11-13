# Tokenfactory contract

The [tokenfactory module](../../../modules/tokenfactory) creates and manages native denominations, authority metadata and issuer bindings. Its keeper, messages, queries and CLI declarations are listed below. Bank-path pause handling and issuer authority are implemented in the referenced keeper and type packages.

The tables identify code declarations and configuration names. Activation, authentication and transport registration are defined by the linked implementations.

## cli

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `paxd tx tokenfactory` | [modules/tokenfactory/client/cli/tx.go](../../../modules/tokenfactory/client/cli/tx.go) |
| source | — | [modules/tokenfactory/client/cli/query.go](../../../modules/tokenfactory/client/cli/query.go) |

## errors

| Kind | Declaration | Source |
|---|---|---|
| source | — | [modules/tokenfactory/types/errors.go](../../../modules/tokenfactory/types/errors.go) |

## events

| Kind | Declaration | Source |
|---|---|---|
| `event` | `AttributePaused = "paused"` | [modules/tokenfactory/types/events.go](../../../modules/tokenfactory/types/events.go) |
| source | — | [modules/tokenfactory/types/msgs.go](../../../modules/tokenfactory/types/msgs.go) |

## genesis

| Kind | Declaration | Source |
|---|---|---|
| `genesis_param` | `message Params {` | [api/tokenfactory/params.proto](../../../api/tokenfactory/params.proto) |
| `genesis_param` | `message GenesisState {` | [api/tokenfactory/genesis.proto](../../../api/tokenfactory/genesis.proto) |
| `go_exported` | `func (k Keeper) ExportGenesis(ctx sdk.Context) *types.GenesisState` | [modules/tokenfactory/keeper/genesis.go](../../../modules/tokenfactory/keeper/genesis.go) |
| `go_exported` | `func (gs GenesisState) Validate() error` | [modules/tokenfactory/types/genesis.go](../../../modules/tokenfactory/types/genesis.go) |
| `go_exported` | `func (k Keeper) InitGenesis(ctx sdk.Context, genState types.GenesisState)` | [modules/tokenfactory/keeper/genesis.go](../../../modules/tokenfactory/keeper/genesis.go) |
| `proto_message` | `message GenesisState {` | [api/tokenfactory/genesis.proto](../../../api/tokenfactory/genesis.proto) |

## go-api

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func GetCmdDenomAuthorityMetadata() *cobra.Command` | [modules/tokenfactory/client/cli/query.go](../../../modules/tokenfactory/client/cli/query.go) |
| `go_exported` | `func GetCmdDenomsFromCreator() *cobra.Command` | [modules/tokenfactory/client/cli/query.go](../../../modules/tokenfactory/client/cli/query.go) |
| `go_exported` | `func GetParams() *cobra.Command` | [modules/tokenfactory/client/cli/query.go](../../../modules/tokenfactory/client/cli/query.go) |
| `go_exported` | `func NewBurnCmd() *cobra.Command` | [modules/tokenfactory/client/cli/tx.go](../../../modules/tokenfactory/client/cli/tx.go) |
| `go_exported` | `func NewChangeAdminCmd() *cobra.Command` | [modules/tokenfactory/client/cli/tx.go](../../../modules/tokenfactory/client/cli/tx.go) |
| `go_exported` | `func NewCreateDenomCmd() *cobra.Command` | [modules/tokenfactory/client/cli/tx.go](../../../modules/tokenfactory/client/cli/tx.go) |
| `go_exported` | `func NewMintCmd() *cobra.Command` | [modules/tokenfactory/client/cli/tx.go](../../../modules/tokenfactory/client/cli/tx.go) |
| `go_exported` | `func NewSetDenomMetadataCmd() *cobra.Command` | [modules/tokenfactory/client/cli/tx.go](../../../modules/tokenfactory/client/cli/tx.go) |
| `go_exported` | `func NewUpdateDenomCmd() *cobra.Command` | [modules/tokenfactory/client/cli/tx.go](../../../modules/tokenfactory/client/cli/tx.go) |
| `go_exported` | `func ParseAllowListJSON(allowListFile string, queryClient evmtypes.QueryClient) (banktypes.AllowList, error)` | [modules/tokenfactory/client/cli/tx.go](../../../modules/tokenfactory/client/cli/tx.go) |
| `go_exported` | `func ParseMetadataJSON(cdc *codec.LegacyAmino, metadataFile string) (banktypes.Metadata, error)` | [modules/tokenfactory/client/cli/tx.go](../../../modules/tokenfactory/client/cli/tx.go) |
| `go_exported` | `func EncodeTokenFactoryBurn(rawMsg json.RawMessage, sender sdk.AccAddress) ([]sdk.Msg, error)` | [modules/tokenfactory/client/wasm/encoder.go](../../../modules/tokenfactory/client/wasm/encoder.go) |
| `go_exported` | `func EncodeTokenFactoryChangeAdmin(rawMsg json.RawMessage, sender sdk.AccAddress) ([]sdk.Msg, error)` | [modules/tokenfactory/client/wasm/encoder.go](../../../modules/tokenfactory/client/wasm/encoder.go) |
| `go_exported` | `func EncodeTokenFactoryCreateDenom(rawMsg json.RawMessage, sender sdk.AccAddress) ([]sdk.Msg, error)` | [modules/tokenfactory/client/wasm/encoder.go](../../../modules/tokenfactory/client/wasm/encoder.go) |
| `go_exported` | `func EncodeTokenFactoryMint(rawMsg json.RawMessage, sender sdk.AccAddress) ([]sdk.Msg, error)` | [modules/tokenfactory/client/wasm/encoder.go](../../../modules/tokenfactory/client/wasm/encoder.go) |
| `go_exported` | `func EncodeTokenFactorySetMetadata(rawMsg json.RawMessage, sender sdk.AccAddress) ([]sdk.Msg, error)` | [modules/tokenfactory/client/wasm/encoder.go](../../../modules/tokenfactory/client/wasm/encoder.go) |
| `go_exported` | `func NewTokenFactoryWasmQueryHandler(keeper *tokenfactorykeeper.Keeper) *TokenFactoryWasmQueryHandler` | [modules/tokenfactory/client/wasm/query.go](../../../modules/tokenfactory/client/wasm/query.go) |
| `go_exported` | `func (handler TokenFactoryWasmQueryHandler) GetDenomAuthorityMetadata(ctx sdk.Context, req *types.QueryDenomAuthorityMetadataRequest) (*types.QueryDenomAuthorityMetadataResponse, error)` | [modules/tokenfactory/client/wasm/query.go](../../../modules/tokenfactory/client/wasm/query.go) |
| `go_exported` | `func (handler TokenFactoryWasmQueryHandler) GetDenomsFromCreator(ctx sdk.Context, req *types.QueryDenomsFromCreatorRequest) (*types.QueryDenomsFromCreatorResponse, error)` | [modules/tokenfactory/client/wasm/query.go](../../../modules/tokenfactory/client/wasm/query.go) |
| `go_exported` | `type TokenFactoryWasmQueryHandler struct` | [modules/tokenfactory/client/wasm/query.go](../../../modules/tokenfactory/client/wasm/query.go) |
| `go_exported` | `func (k Keeper) CreateDenom(ctx sdk.Context, creatorAddr string, subdenom string) (newTokenDenom string, err error)` | [modules/tokenfactory/keeper/createdenom.go](../../../modules/tokenfactory/keeper/createdenom.go) |
| `go_exported` | `func (k Keeper) CreateModuleAccount(ctx sdk.Context)` | [modules/tokenfactory/keeper/keeper.go](../../../modules/tokenfactory/keeper/keeper.go) |
| `go_exported` | `func (k Keeper) DenomAllowList(c context.Context, req *types.QueryDenomAllowListRequest) (*types.QueryDenomAllowListResponse, error)` | [modules/tokenfactory/keeper/grpc_query.go](../../../modules/tokenfactory/keeper/grpc_query.go) |
| `go_exported` | `func (k Keeper) DenomAuthorityMetadata(ctx context.Context, req *types.QueryDenomAuthorityMetadataRequest) (*types.QueryDenomAuthorityMetadataResponse, error)` | [modules/tokenfactory/keeper/grpc_query.go](../../../modules/tokenfactory/keeper/grpc_query.go) |
| `go_exported` | `func (k Keeper) DenomMetadata(c context.Context, req *types.QueryDenomMetadataRequest) (*types.QueryDenomMetadataResponse, error)` | [modules/tokenfactory/keeper/grpc_query.go](../../../modules/tokenfactory/keeper/grpc_query.go) |
| `go_exported` | `func (k Keeper) DenomsFromCreator(ctx context.Context, req *types.QueryDenomsFromCreatorRequest) (*types.QueryDenomsFromCreatorResponse, error)` | [modules/tokenfactory/keeper/grpc_query.go](../../../modules/tokenfactory/keeper/grpc_query.go) |
| `go_exported` | `func (k Keeper) GetAllDenomsFromCreator(ctx sdk.Context, creator string) []string` | [modules/tokenfactory/keeper/creators.go](../../../modules/tokenfactory/keeper/creators.go) |
| `go_exported` | `func (k Keeper) GetAllDenomsIterator(ctx sdk.Context) sdk.Iterator` | [modules/tokenfactory/keeper/creators.go](../../../modules/tokenfactory/keeper/creators.go) |
| `go_exported` | `func (k Keeper) GetAuthorityMetadata(ctx sdk.Context, denom string) (types.DenomAuthorityMetadata, error)` | [modules/tokenfactory/keeper/admins.go](../../../modules/tokenfactory/keeper/admins.go) |
| `go_exported` | `func (k Keeper) GetCreatorPrefixStore(ctx sdk.Context, creator string) sdk.KVStore` | [modules/tokenfactory/keeper/keeper.go](../../../modules/tokenfactory/keeper/keeper.go) |
| `go_exported` | `func (k Keeper) GetCreatorsPrefixStore(ctx sdk.Context) sdk.KVStore` | [modules/tokenfactory/keeper/keeper.go](../../../modules/tokenfactory/keeper/keeper.go) |
| `go_exported` | `func (k Keeper) GetDenomAllowListMaxSize(ctx sdk.Context) uint32` | [modules/tokenfactory/keeper/keeper.go](../../../modules/tokenfactory/keeper/keeper.go) |
| `go_exported` | `func (k Keeper) GetDenomPrefixStore(ctx sdk.Context, denom string) sdk.KVStore` | [modules/tokenfactory/keeper/keeper.go](../../../modules/tokenfactory/keeper/keeper.go) |
| `go_exported` | `func (k Keeper) GetParams(ctx sdk.Context) (params types.Params)` | [modules/tokenfactory/keeper/params.go](../../../modules/tokenfactory/keeper/params.go) |
| `go_exported` | `func (k Keeper) IsDenomPaused(ctx sdk.Context, denom string) bool` | [modules/tokenfactory/keeper/pause.go](../../../modules/tokenfactory/keeper/pause.go) |
| `go_exported` | `func (k Keeper) IssuerDid(ctx sdk.Context, denom string) (string, error)` | [modules/tokenfactory/keeper/admins.go](../../../modules/tokenfactory/keeper/admins.go) |
| `go_exported` | `func (k Keeper) Params(ctx context.Context, _ *types.QueryParamsRequest) (*types.QueryParamsResponse, error)` | [modules/tokenfactory/keeper/grpc_query.go](../../../modules/tokenfactory/keeper/grpc_query.go) |
| `go_exported` | `func (k Keeper) SendEnabledChecker(ctx sdk.Context, denom string) bool` | [modules/tokenfactory/keeper/pause.go](../../../modules/tokenfactory/keeper/pause.go) |
| `go_exported` | `func (k Keeper) SetParams(ctx sdk.Context, params types.Params)` | [modules/tokenfactory/keeper/params.go](../../../modules/tokenfactory/keeper/params.go) |
| `go_exported` | `type Migrator struct` | [modules/tokenfactory/keeper/migrations.go](../../../modules/tokenfactory/keeper/migrations.go) |
| `go_exported` | `func (m Migrator) Migrate2to3(ctx sdk.Context) error` | [modules/tokenfactory/keeper/migrations.go](../../../modules/tokenfactory/keeper/migrations.go) |
| `go_exported` | `func (m Migrator) Migrate3to4(ctx sdk.Context) error` | [modules/tokenfactory/keeper/migrations.go](../../../modules/tokenfactory/keeper/migrations.go) |
| `go_exported` | `func (m Migrator) Migrate4to5(ctx sdk.Context) error` | [modules/tokenfactory/keeper/migrations.go](../../../modules/tokenfactory/keeper/migrations.go) |
| `go_exported` | `func (m Migrator) SetMetadata(denomMetadata *banktypes.Metadata)` | [modules/tokenfactory/keeper/migrations.go](../../../modules/tokenfactory/keeper/migrations.go) |
| `go_exported` | `func NewMigrator(keeper Keeper) Migrator` | [modules/tokenfactory/keeper/migrations.go](../../../modules/tokenfactory/keeper/migrations.go) |
| `go_exported` | `func NewMsgServerImpl(keeper Keeper) types.MsgServer` | [modules/tokenfactory/keeper/msg_server.go](../../../modules/tokenfactory/keeper/msg_server.go) |
| `go_exported` | `func (am AppModule) ExportGenesis(ctx sdk.Context, cdc codec.JSONCodec) json.RawMessage` | [modules/tokenfactory/module.go](../../../modules/tokenfactory/module.go) |
| `go_exported` | `func (am AppModule) ExportGenesisStream(ctx sdk.Context, cdc codec.JSONCodec) <-chan json.RawMessage` | [modules/tokenfactory/module.go](../../../modules/tokenfactory/module.go) |
| `go_exported` | `func (am AppModule) GenerateGenesisState(simState *module.SimulationState)` | [modules/tokenfactory/module.go](../../../modules/tokenfactory/module.go) |
| `go_exported` | `func (am AppModule) InitGenesis(ctx sdk.Context, cdc codec.JSONCodec, gs json.RawMessage) []abci.ValidatorUpdate` | [modules/tokenfactory/module.go](../../../modules/tokenfactory/module.go) |
| `go_exported` | `type AppModule struct` | [modules/tokenfactory/module.go](../../../modules/tokenfactory/module.go) |
| `go_exported` | `func (am AppModule) LegacyQuerierHandler(_ *codec.LegacyAmino) sdk.Querier` | [modules/tokenfactory/module.go](../../../modules/tokenfactory/module.go) |
| `go_exported` | `func (am AppModule) Name() string` | [modules/tokenfactory/module.go](../../../modules/tokenfactory/module.go) |
| `go_exported` | `func (am AppModule) ProposalContents(_ module.SimulationState) []simtypes.WeightedProposalContent` | [modules/tokenfactory/module.go](../../../modules/tokenfactory/module.go) |
| `go_exported` | `func (am AppModule) RandomizedParams(_ *rand.Rand) []simtypes.ParamChange` | [modules/tokenfactory/module.go](../../../modules/tokenfactory/module.go) |
| `go_exported` | `func (am AppModule) RegisterServices(cfg module.Configurator)` | [modules/tokenfactory/module.go](../../../modules/tokenfactory/module.go) |
| `go_exported` | `func (am AppModule) RegisterStoreDecoder(_ sdk.StoreDecoderRegistry)` | [modules/tokenfactory/module.go](../../../modules/tokenfactory/module.go) |
| `go_exported` | `func (am AppModule) Route() sdk.Route` | [modules/tokenfactory/module.go](../../../modules/tokenfactory/module.go) |
| `go_exported` | `func (am AppModule) WeightedOperations(_ module.SimulationState) []simtypes.WeightedOperation` | [modules/tokenfactory/module.go](../../../modules/tokenfactory/module.go) |
| `go_exported` | `func (a AppModuleBasic) GetTxCmd() *cobra.Command` | [modules/tokenfactory/module.go](../../../modules/tokenfactory/module.go) |
| `go_exported` | `type AppModuleBasic struct{}` | [modules/tokenfactory/module.go](../../../modules/tokenfactory/module.go) |
| `go_exported` | `func (a AppModuleBasic) RegisterInterfaces(reg cdctypes.InterfaceRegistry)` | [modules/tokenfactory/module.go](../../../modules/tokenfactory/module.go) |
| `go_exported` | `func (am AppModuleBasic) ValidateGenesisStream(cdc codec.JSONCodec, config client.TxEncodingConfig, genesisCh <-chan json.RawMessage) error` | [modules/tokenfactory/module.go](../../../modules/tokenfactory/module.go) |
| `go_exported` | `func (AppModuleBasic) DefaultGenesis(cdc codec.JSONCodec) json.RawMessage` | [modules/tokenfactory/module.go](../../../modules/tokenfactory/module.go) |
| `go_exported` | `func (AppModuleBasic) GetQueryCmd() *cobra.Command` | [modules/tokenfactory/module.go](../../../modules/tokenfactory/module.go) |
| `go_exported` | `func (AppModuleBasic) Name() string` | [modules/tokenfactory/module.go](../../../modules/tokenfactory/module.go) |
| `go_exported` | `func (AppModuleBasic) RegisterGRPCGatewayRoutes(clientCtx client.Context, mux *runtime.ServeMux)` | [modules/tokenfactory/module.go](../../../modules/tokenfactory/module.go) |
| `go_exported` | `func (AppModuleBasic) RegisterLegacyAminoCodec(cdc *codec.LegacyAmino)` | [modules/tokenfactory/module.go](../../../modules/tokenfactory/module.go) |
| `go_exported` | `func (AppModuleBasic) RegisterRESTRoutes(_ client.Context, _ *mux.Router)` | [modules/tokenfactory/module.go](../../../modules/tokenfactory/module.go) |
| `go_exported` | `func (AppModuleBasic) ValidateGenesis(cdc codec.JSONCodec, _ client.TxEncodingConfig, bz json.RawMessage) error` | [modules/tokenfactory/module.go](../../../modules/tokenfactory/module.go) |
| `go_exported` | `func (AppModule) ConsensusVersion() uint64 { return 5 }` | [modules/tokenfactory/module.go](../../../modules/tokenfactory/module.go) |
| `go_exported` | `func (AppModule) QuerierRoute() string { return types.QuerierRoute }` | [modules/tokenfactory/module.go](../../../modules/tokenfactory/module.go) |
| `go_exported` | `func NewAppModule(` | [modules/tokenfactory/module.go](../../../modules/tokenfactory/module.go) |
| `go_exported` | `func NewAppModuleBasic() AppModuleBasic` | [modules/tokenfactory/module.go](../../../modules/tokenfactory/module.go) |
| `go_exported` | `func NewProposalHandler(_ keeper.Keeper) govtypes.Handler` | [modules/tokenfactory/handler.go](../../../modules/tokenfactory/handler.go) |
| `go_exported` | `type AccountKeeper interface` | [modules/tokenfactory/types/expected_keepers.go](../../../modules/tokenfactory/types/expected_keepers.go) |
| `go_exported` | `func DeconstructDenom(denom string) (creator string, subdenom string, err error)` | [modules/tokenfactory/types/denoms.go](../../../modules/tokenfactory/types/denoms.go) |
| `go_exported` | `func DefaultGenesis() *GenesisState` | [modules/tokenfactory/types/genesis.go](../../../modules/tokenfactory/types/genesis.go) |
| `go_exported` | `func DefaultParams() Params` | [modules/tokenfactory/types/params.go](../../../modules/tokenfactory/types/params.go) |
| `go_exported` | `func (metadata DenomAuthorityMetadata) Validate() error` | [modules/tokenfactory/types/authority_metadata.go](../../../modules/tokenfactory/types/authority_metadata.go) |
| `go_exported` | `func GetCreatorPrefix(creator string) []byte` | [modules/tokenfactory/types/keys.go](../../../modules/tokenfactory/types/keys.go) |
| `go_exported` | `func GetCreatorsPrefix() []byte` | [modules/tokenfactory/types/keys.go](../../../modules/tokenfactory/types/keys.go) |
| `go_exported` | `func GetDenomPausedKey(denom string) []byte` | [modules/tokenfactory/types/keys.go](../../../modules/tokenfactory/types/keys.go) |
| `go_exported` | `func GetDenomPrefixStore(denom string) []byte` | [modules/tokenfactory/types/keys.go](../../../modules/tokenfactory/types/keys.go) |
| `go_exported` | `func GetTokenDenom(creator, subdenom string) (string, error)` | [modules/tokenfactory/types/denoms.go](../../../modules/tokenfactory/types/denoms.go) |
| `go_exported` | `func (m MsgBurn) GetSignBytes() []byte` | [modules/tokenfactory/types/msgs.go](../../../modules/tokenfactory/types/msgs.go) |
| `go_exported` | `func (m MsgBurn) GetSigners() []sdk.AccAddress` | [modules/tokenfactory/types/msgs.go](../../../modules/tokenfactory/types/msgs.go) |
| `go_exported` | `func (m MsgBurn) Route() string { return RouterKey }` | [modules/tokenfactory/types/msgs.go](../../../modules/tokenfactory/types/msgs.go) |
| `go_exported` | `func (m MsgBurn) Type() string  { return TypeMsgBurn }` | [modules/tokenfactory/types/msgs.go](../../../modules/tokenfactory/types/msgs.go) |
| `go_exported` | `func (m MsgBurn) ValidateBasic() error` | [modules/tokenfactory/types/msgs.go](../../../modules/tokenfactory/types/msgs.go) |
| `go_exported` | `func (m MsgChangeAdmin) GetSignBytes() []byte` | [modules/tokenfactory/types/msgs.go](../../../modules/tokenfactory/types/msgs.go) |
| `go_exported` | `func (m MsgChangeAdmin) GetSigners() []sdk.AccAddress` | [modules/tokenfactory/types/msgs.go](../../../modules/tokenfactory/types/msgs.go) |
| `go_exported` | `func (m MsgChangeAdmin) Route() string { return RouterKey }` | [modules/tokenfactory/types/msgs.go](../../../modules/tokenfactory/types/msgs.go) |
| `go_exported` | `func (m MsgChangeAdmin) Type() string  { return TypeMsgChangeAdmin }` | [modules/tokenfactory/types/msgs.go](../../../modules/tokenfactory/types/msgs.go) |
| `go_exported` | `func (m MsgChangeAdmin) ValidateBasic() error` | [modules/tokenfactory/types/msgs.go](../../../modules/tokenfactory/types/msgs.go) |
| `go_exported` | `func (m MsgCreateDenom) GetSignBytes() []byte` | [modules/tokenfactory/types/msgs.go](../../../modules/tokenfactory/types/msgs.go) |
| `go_exported` | `func (m MsgCreateDenom) GetSigners() []sdk.AccAddress` | [modules/tokenfactory/types/msgs.go](../../../modules/tokenfactory/types/msgs.go) |
| `go_exported` | `func (m MsgCreateDenom) Route() string { return RouterKey }` | [modules/tokenfactory/types/msgs.go](../../../modules/tokenfactory/types/msgs.go) |
| `go_exported` | `func (m MsgCreateDenom) Type() string  { return TypeMsgCreateDenom }` | [modules/tokenfactory/types/msgs.go](../../../modules/tokenfactory/types/msgs.go) |
| `go_exported` | `func (m MsgCreateDenom) ValidateBasic() error` | [modules/tokenfactory/types/msgs.go](../../../modules/tokenfactory/types/msgs.go) |
| `go_exported` | `func (m MsgMint) GetSignBytes() []byte` | [modules/tokenfactory/types/msgs.go](../../../modules/tokenfactory/types/msgs.go) |
| `go_exported` | `func (m MsgMint) GetSigners() []sdk.AccAddress` | [modules/tokenfactory/types/msgs.go](../../../modules/tokenfactory/types/msgs.go) |
| `go_exported` | `func (m MsgMint) Route() string { return RouterKey }` | [modules/tokenfactory/types/msgs.go](../../../modules/tokenfactory/types/msgs.go) |
| `go_exported` | `func (m MsgMint) Type() string  { return TypeMsgMint }` | [modules/tokenfactory/types/msgs.go](../../../modules/tokenfactory/types/msgs.go) |
| `go_exported` | `func (m MsgMint) ValidateBasic() error` | [modules/tokenfactory/types/msgs.go](../../../modules/tokenfactory/types/msgs.go) |
| `go_exported` | `func (m MsgSetDenomMetadata) GetSignBytes() []byte` | [modules/tokenfactory/types/msgs.go](../../../modules/tokenfactory/types/msgs.go) |
| `go_exported` | `func (m MsgSetDenomMetadata) GetSigners() []sdk.AccAddress` | [modules/tokenfactory/types/msgs.go](../../../modules/tokenfactory/types/msgs.go) |
| `go_exported` | `func (m MsgSetDenomMetadata) Route() string { return RouterKey }` | [modules/tokenfactory/types/msgs.go](../../../modules/tokenfactory/types/msgs.go) |
| `go_exported` | `func (m MsgSetDenomMetadata) Type() string  { return TypeMsgSetDenomMetadata }` | [modules/tokenfactory/types/msgs.go](../../../modules/tokenfactory/types/msgs.go) |
| `go_exported` | `func (m MsgSetDenomMetadata) ValidateBasic() error` | [modules/tokenfactory/types/msgs.go](../../../modules/tokenfactory/types/msgs.go) |
| `go_exported` | `func (m MsgSetDenomPaused) GetSignBytes() []byte` | [modules/tokenfactory/types/msgs.go](../../../modules/tokenfactory/types/msgs.go) |
| `go_exported` | `func (m MsgSetDenomPaused) GetSigners() []sdk.AccAddress` | [modules/tokenfactory/types/msgs.go](../../../modules/tokenfactory/types/msgs.go) |
| `go_exported` | `func (m MsgSetDenomPaused) Route() string { return RouterKey }` | [modules/tokenfactory/types/msgs.go](../../../modules/tokenfactory/types/msgs.go) |
| `go_exported` | `func (m MsgSetDenomPaused) Type() string  { return TypeMsgSetDenomPaused }` | [modules/tokenfactory/types/msgs.go](../../../modules/tokenfactory/types/msgs.go) |
| `go_exported` | `func (m MsgSetDenomPaused) ValidateBasic() error` | [modules/tokenfactory/types/msgs.go](../../../modules/tokenfactory/types/msgs.go) |
| `go_exported` | `func (m MsgUpdateDenom) GetSignBytes() []byte` | [modules/tokenfactory/types/msgs.go](../../../modules/tokenfactory/types/msgs.go) |
| `go_exported` | `func (m MsgUpdateDenom) GetSigners() []sdk.AccAddress` | [modules/tokenfactory/types/msgs.go](../../../modules/tokenfactory/types/msgs.go) |
| `go_exported` | `func (m MsgUpdateDenom) Route() string { return RouterKey }` | [modules/tokenfactory/types/msgs.go](../../../modules/tokenfactory/types/msgs.go) |
| `go_exported` | `func (m MsgUpdateDenom) Type() string  { return TypeMsgUpdateDenom }` | [modules/tokenfactory/types/msgs.go](../../../modules/tokenfactory/types/msgs.go) |
| `go_exported` | `func (m MsgUpdateDenom) ValidateBasic() error` | [modules/tokenfactory/types/msgs.go](../../../modules/tokenfactory/types/msgs.go) |
| `go_exported` | `func NewMsgBurn(sender string, amount sdk.Coin) *MsgBurn` | [modules/tokenfactory/types/msgs.go](../../../modules/tokenfactory/types/msgs.go) |
| `go_exported` | `func NewMsgChangeAdmin(sender, denom, newAdmin string) *MsgChangeAdmin` | [modules/tokenfactory/types/msgs.go](../../../modules/tokenfactory/types/msgs.go) |
| `go_exported` | `func NewMsgCreateDenom(sender, subdenom string) *MsgCreateDenom` | [modules/tokenfactory/types/msgs.go](../../../modules/tokenfactory/types/msgs.go) |
| `go_exported` | `func NewMsgMint(sender string, amount sdk.Coin) *MsgMint` | [modules/tokenfactory/types/msgs.go](../../../modules/tokenfactory/types/msgs.go) |
| `go_exported` | `func NewMsgSetDenomMetadata(sender string, metadata banktypes.Metadata) *MsgSetDenomMetadata` | [modules/tokenfactory/types/msgs.go](../../../modules/tokenfactory/types/msgs.go) |
| `go_exported` | `func NewMsgSetDenomPaused(sender, denom string, paused bool) *MsgSetDenomPaused` | [modules/tokenfactory/types/msgs.go](../../../modules/tokenfactory/types/msgs.go) |
| `go_exported` | `func NewMsgUpdateDenom(sender, denom string, allowList *banktypes.AllowList) *MsgUpdateDenom` | [modules/tokenfactory/types/msgs.go](../../../modules/tokenfactory/types/msgs.go) |
| `go_exported` | `func NewTokenFactoryDenomMintCoinsRestriction() bankkeeper.MintingRestrictionFn` | [modules/tokenfactory/types/denoms.go](../../../modules/tokenfactory/types/denoms.go) |
| `go_exported` | `func ParamKeyTable() paramtypes.KeyTable` | [modules/tokenfactory/types/params.go](../../../modules/tokenfactory/types/params.go) |
| `go_exported` | `func (p *Params) ParamSetPairs() paramtypes.ParamSetPairs` | [modules/tokenfactory/types/params.go](../../../modules/tokenfactory/types/params.go) |
| `go_exported` | `func (p Params) Validate() error` | [modules/tokenfactory/types/params.go](../../../modules/tokenfactory/types/params.go) |
| `go_exported` | `func RegisterCodec(cdc *codec.LegacyAmino)` | [modules/tokenfactory/types/codec.go](../../../modules/tokenfactory/types/codec.go) |
| `go_exported` | `func RegisterInterfaces(registry cdctypes.InterfaceRegistry)` | [modules/tokenfactory/types/codec.go](../../../modules/tokenfactory/types/codec.go) |

## invariants

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func (am AppModule) RegisterInvariants(ir sdk.InvariantRegistry) {` | [modules/tokenfactory/module.go](../../../modules/tokenfactory/module.go) |
| `go_exported` | `func PausedDenomsInvariant(k Keeper) sdk.Invariant` | [modules/tokenfactory/keeper/invariants.go](../../../modules/tokenfactory/keeper/invariants.go) |

## issuer-did

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func (k Keeper) BurnByIssuer(ctx sdk.Context, callerDid string, amount sdk.Coin, from string) error` | [modules/tokenfactory/keeper/issuer_supply.go](../../../modules/tokenfactory/keeper/issuer_supply.go) |
| `go_exported` | `type IssuerKeeper interface {` | [modules/tokenfactory/types/expected_issuer.go](../../../modules/tokenfactory/types/expected_issuer.go) |
| `go_exported` | `func (k Keeper) MintByIssuer(ctx sdk.Context, callerDid string, amount sdk.Coin, to string) error` | [modules/tokenfactory/keeper/issuer_supply.go](../../../modules/tokenfactory/keeper/issuer_supply.go) |
| `go_exported` | `func (k Keeper) SetDenomPausedByIssuer(ctx sdk.Context, denom, callerDid string, paused bool) error` | [modules/tokenfactory/keeper/issuer.go](../../../modules/tokenfactory/keeper/issuer.go) |
| `go_exported` | `func (k Keeper) SetIssuerDid(ctx sdk.Context, denom string, issuerDid string) error` | [modules/tokenfactory/keeper/admins.go](../../../modules/tokenfactory/keeper/admins.go) |

## keeper

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `type BankKeeper interface` | [modules/tokenfactory/types/expected_keepers.go](../../../modules/tokenfactory/types/expected_keepers.go) |
| `go_exported` | `func NewKeeper(` | [modules/tokenfactory/keeper/keeper.go](../../../modules/tokenfactory/keeper/keeper.go) |

## pause-authority

| Kind | Declaration | Source |
|---|---|---|
| source | — | [api/tokenfactory/tx.proto](../../../api/tokenfactory/tx.proto) |

<a id="pause-bank-paths"></a>

## pause-store

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func (k Keeper) GetPausedDenoms(ctx sdk.Context) []string` | [modules/tokenfactory/keeper/pause.go](../../../modules/tokenfactory/keeper/pause.go) |
| `storage_key` | `DenomAllowListMaxSizeKey = []byte("allowlistmaxsize")` | [modules/tokenfactory/types/keys.go](../../../modules/tokenfactory/types/keys.go) |

## queries

| Kind | Declaration | Source |
|---|---|---|
| `proto_message` | `message QueryDenomPausedRequest {` | [api/tokenfactory/query.proto](../../../api/tokenfactory/query.proto) |
| `proto_message` | `message QueryDenomPausedResponse {` | [api/tokenfactory/query.proto](../../../api/tokenfactory/query.proto) |

## wasm

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func EncodeTokenFactorySetDenomPaused(rawMsg json.RawMessage, sender sdk.AccAddress) ([]sdk.Msg, error)` | [modules/tokenfactory/client/wasm/encoder.go](../../../modules/tokenfactory/client/wasm/encoder.go) |
| `go_exported` | `type PaxTokenFactoryQuery struct {` | [modules/tokenfactory/client/wasm/bindings/queries.go](../../../modules/tokenfactory/client/wasm/bindings/queries.go) |
| `go_exported` | `func (handler TokenFactoryWasmQueryHandler) GetDenomPaused(ctx sdk.Context, req *types.QueryDenomPausedRequest) (*types.QueryDenomPausedResponse, error)` | [modules/tokenfactory/client/wasm/query.go](../../../modules/tokenfactory/client/wasm/query.go) |
| source | — | [modules/tokenfactory/client/wasm/bindings/msgs.go](../../../modules/tokenfactory/client/wasm/bindings/msgs.go) |

package evidence_test

import (
	"bytes"
	"context"
	stded25519 "crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/grpc/gogoreflection"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/query"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/evidence"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/evidence/exported"
	evidencekeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/evidence/keeper"
	evidencetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/evidence/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	slashingkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/slashing/keeper"
	"github.com/gogo/gateway"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/jhump/protoreflect/grpcreflect"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// This function is the package documentation's setup example, compiled here and
// compared structurally with doc.go so the example cannot drift independently.
func configureEvidence(
	appCodec codec.Codec, storeKey sdk.StoreKey,
	stakingKeeper evidencetypes.StakingKeeper, slashingKeeper evidencetypes.SlashingKeeper,
	evidenceHandler evidencetypes.Handler,
) (*evidencekeeper.Keeper, *module.Manager) {
	evidenceKeeper := evidencekeeper.NewKeeper(appCodec, storeKey, stakingKeeper, slashingKeeper)
	evidenceRouter := evidencetypes.NewRouter().
		AddRoute(evidencetypes.RouteEquivocation, evidenceHandler)
	evidenceKeeper.SetRouter(evidenceRouter)
	mm := module.NewManager(evidence.NewAppModule(*evidenceKeeper))
	return evidenceKeeper, mm
}

type repairM605Harness struct {
	cdc         *codec.ProtoCodec
	registry    codectypes.InterfaceRegistry
	ctx         sdk.Context
	keeper      *evidencekeeper.Keeper
	manager     *module.Manager
	msgRouter   *baseapp.MsgServiceRouter
	queryHelper *baseapp.QueryServiceTestHelper
}

func repairM605Setup(t *testing.T) repairM605Harness {
	t.Helper()
	registry := codectypes.NewInterfaceRegistry()
	module.NewBasicManager(evidence.AppModuleBasic{}).RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	db := dbm.NewMemDB()
	cms := store.NewCommitMultiStore(db)
	t.Cleanup(func() { require.NoError(t, cms.Close()) })
	evidenceKey := sdk.NewKVStoreKey(evidencetypes.StoreKey)
	slashingKey := sdk.NewKVStoreKey("slashing")
	paramsKey := sdk.NewKVStoreKey(paramtypes.StoreKey)
	transientKey := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	for _, key := range []*sdk.KVStoreKey{evidenceKey, slashingKey, paramsKey} {
		cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	}
	cms.MountStoreWithDB(transientKey, sdk.StoreTypeTransient, db)
	require.NoError(t, cms.LoadLatestVersion())
	ctx := sdk.NewContext(cms, tmproto.Header{Height: 101, Time: time.Date(2006, 1, 2, 16, 4, 5, 0, time.UTC)}, false)
	slashing := slashingkeeper.NewKeeper(cdc, slashingKey, nil,
		paramtypes.NewSubspace(cdc, codec.NewLegacyAmino(), paramsKey, transientKey, "slashing"))
	var k *evidencekeeper.Keeper
	// This adapter invokes production equivocation handling. The real slashing
	// store has no validator pubkey, so that handler takes its documented ignore
	// path; the production submission keeper then persists the routed evidence.
	handler := func(ctx sdk.Context, e exported.Evidence) error {
		if err := e.ValidateBasic(); err != nil {
			return err
		}
		k.HandleEquivocationEvidence(ctx, e.(*evidencetypes.Equivocation))
		return nil
	}
	k, manager := configureEvidence(cdc, evidenceKey, nil, slashing, handler)
	msgRouter := baseapp.NewMsgServiceRouter()
	msgRouter.SetInterfaceRegistry(registry)
	queryHelper := baseapp.NewQueryServerTestHelper(ctx, registry)
	manager.RegisterServices(module.NewConfigurator(cdc, msgRouter, queryHelper))
	return repairM605Harness{cdc, registry, ctx, k, manager, msgRouter, queryHelper}
}

func repairM605Producer(t *testing.T) abci.Evidence {
	t.Helper()
	// RFC 8032 section 7.1's public test vector, used only as a validator fixture.
	seed, err := hex.DecodeString("9d61b19deffd5a60ba844af492ec2cc44449c5697b326919703bac031cae7f60")
	require.NoError(t, err)
	key := &ed25519.PrivKey{Key: stded25519.NewKeyFromSeed(seed)}
	return abci.Evidence{
		Type:   abci.MisbehaviorType_DUPLICATE_VOTE,
		Height: 100, Time: time.Date(2006, 1, 2, 15, 4, 5, 0, time.UTC),
		Validator:        abci.Validator{Address: key.PubKey().Address(), Power: 1000000},
		TotalVotingPower: 2000000,
	}
}

func repairM605Stored(t *testing.T, h repairM605Harness, producer abci.Evidence) *evidencetypes.Equivocation {
	t.Helper()
	source := evidencetypes.FromABCIEvidence(producer)
	stored, found := h.keeper.GetEvidence(h.ctx, source.Hash())
	require.True(t, found)
	require.Equal(t, source, stored)
	decoded := stored.(*evidencetypes.Equivocation)
	require.Equal(t, []byte(producer.Validator.Address), decoded.GetConsensusAddress().Bytes())
	return decoded
}

func repairM605Submit(t *testing.T, h repairM605Harness, producer abci.Evidence) *evidencetypes.MsgSubmitEvidenceResponse {
	t.Helper()
	msg, err := evidencetypes.NewMsgSubmitEvidence(sdk.AccAddress(producer.Validator.Address), evidencetypes.FromABCIEvidence(producer))
	require.NoError(t, err)
	var decoded evidencetypes.MsgSubmitEvidence
	require.NoError(t, h.cdc.Unmarshal(h.cdc.MustMarshal(msg), &decoded))
	require.NoError(t, decoded.ValidateBasic())
	route := h.msgRouter.Handler(&decoded)
	require.NotNil(t, route)
	result, err := route(h.ctx, &decoded)
	require.NoError(t, err)
	require.NotNil(t, result)
	var response evidencetypes.MsgSubmitEvidenceResponse
	require.NoError(t, h.cdc.Unmarshal(result.Data, &response))
	require.Equal(t, evidencetypes.FromABCIEvidence(producer).Hash().Bytes(), response.Hash)
	repairM605Stored(t, h, producer)
	return &response
}

func repairM605Read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

func TestRepairM_605_AtlasPurpose(t *testing.T) {
	atlas := repairM605Read(t, "atlas/atlas-v0.41.x.md")
	for _, claim := range []string{"stores evidence", "registered in-process handlers", "staking and slashing", "slash, jail and tombstone", "Upstream provenance", "Cosmos SDK v0.41", "empty sealed"} {
		require.Contains(t, atlas, claim)
	}
	require.NotContains(t, atlas, "multi-asset coin transfers")
	h := repairM605Setup(t)
	producer := repairM605Producer(t)
	evidence.BeginBlocker(h.ctx, []abci.Misbehavior{abci.Misbehavior(producer)}, *h.keeper)
	require.Empty(t, h.keeper.GetAllEvidence(h.ctx), "unknown-validator consensus evidence follows the documented ignore path")
	repairM605Submit(t, h, producer)
	response, err := evidencetypes.NewQueryClient(h.queryHelper).Evidence(context.Background(), &evidencetypes.QueryEvidenceRequest{
		EvidenceHash: evidencetypes.FromABCIEvidence(producer).Hash(),
	})
	require.NoError(t, err)
	var decoded exported.Evidence
	require.NoError(t, h.cdc.UnpackAny(response.Evidence, &decoded))
	require.Equal(t, evidencetypes.FromABCIEvidence(producer), decoded)
}

func TestRepairM_605_SetupExample(t *testing.T) {
	fset := token.NewFileSet()
	docFile, err := parser.ParseFile(fset, "doc.go", nil, parser.ParseComments)
	require.NoError(t, err)
	require.NotNil(t, docFile.Doc)
	doc := docFile.Doc.Text()
	start := strings.Index(doc, "import (")
	require.GreaterOrEqual(t, start, 0)
	example, err := parser.ParseFile(fset, "example.go", "package evidence_test\n"+doc[start:], 0)
	require.NoError(t, err)
	compiled, err := parser.ParseFile(fset, "repair_m605_test.go", nil, 0)
	require.NoError(t, err)
	function := func(file *ast.File) *ast.FuncDecl {
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "configureEvidence" {
				return fn
			}
		}
		t.Fatal("configureEvidence must be declared")
		return nil
	}
	var documented, executable bytes.Buffer
	require.NoError(t, format.Node(&documented, fset, function(example)))
	require.NoError(t, format.Node(&executable, fset, function(compiled)))
	require.Equal(t, documented.String(), executable.String(), "the documented setup is the compiled and executed setup")
	h := repairM605Setup(t)
	require.Contains(t, h.manager.Modules, evidencetypes.ModuleName)
	require.NotNil(t, h.queryHelper.Route("/cosmos.evidence.v1beta1.Query/Evidence"))
	require.Panics(t, func() { h.keeper.SetRouter(evidencetypes.NewRouter()) })
	repairM605Submit(t, h, repairM605Producer(t))
}

func TestRepairM_605_GenesisFixture(t *testing.T) {
	h := repairM605Setup(t)
	producer := repairM605Producer(t)
	genesis := evidencetypes.NewGenesisState([]exported.Evidence{evidencetypes.FromABCIEvidence(producer)})
	var decoded evidencetypes.GenesisState
	require.NoError(t, h.cdc.UnmarshalAsJSON(h.cdc.MustMarshalJSON(genesis), &decoded))
	require.NoError(t, decoded.Validate())
	evidence.InitGenesis(h.ctx, *h.keeper, &decoded)
	repairM605Stored(t, h, producer)
	exportedState := evidence.ExportGenesis(h.ctx, *h.keeper)
	other := repairM605Setup(t)
	var exportedDecoded evidencetypes.GenesisState
	require.NoError(t, other.cdc.Unmarshal(other.cdc.MustMarshal(exportedState), &exportedDecoded))
	evidence.InitGenesis(other.ctx, *other.keeper, &exportedDecoded)
	repairM605Stored(t, other, producer)

	malformed := evidencetypes.FromABCIEvidence(producer).(*evidencetypes.Equivocation)
	malformed.ConsensusAddress = hex.EncodeToString(producer.Validator.Address)
	invalid := evidencetypes.NewGenesisState([]exported.Evidence{malformed})
	var invalidDecoded evidencetypes.GenesisState
	require.NoError(t, h.cdc.UnmarshalAsJSON(h.cdc.MustMarshalJSON(invalid), &invalidDecoded))
	require.ErrorIs(t, invalidDecoded.Validate(), evidencetypes.ErrInvalidEvidence)
	empty := repairM605Setup(t)
	require.Panics(t, func() { evidence.InitGenesis(empty.ctx, *empty.keeper, &invalidDecoded) })
	require.Empty(t, empty.keeper.GetAllEvidence(empty.ctx))
	TestGenesisTestSuite(t)
}

func TestRepairM_605_HandlerFixture(t *testing.T) {
	h := repairM605Setup(t)
	producer := repairM605Producer(t)
	repairM605Submit(t, h, producer)
	malformed := evidencetypes.FromABCIEvidence(producer).(*evidencetypes.Equivocation)
	malformed.ConsensusAddress = hex.EncodeToString(producer.Validator.Address)
	msg, err := evidencetypes.NewMsgSubmitEvidence(sdk.AccAddress(producer.Validator.Address), malformed)
	require.NoError(t, err)
	var decoded evidencetypes.MsgSubmitEvidence
	require.NoError(t, h.cdc.Unmarshal(h.cdc.MustMarshal(msg), &decoded))
	require.ErrorIs(t, decoded.ValidateBasic(), evidencetypes.ErrInvalidEvidence)
	empty := repairM605Setup(t)
	result, err := evidence.NewHandler(*empty.keeper)(empty.ctx, &decoded)
	require.ErrorIs(t, err, evidencetypes.ErrInvalidEvidence)
	require.Nil(t, result)
	require.Empty(t, empty.keeper.GetAllEvidence(empty.ctx))
	require.Empty(t, empty.ctx.EventManager().Events())
	TestHandlerTestSuite(t)
}

func repairM605QueryClient(t *testing.T, h repairM605Harness) evidencetypes.QueryClient {
	t.Helper()
	return evidencetypes.NewQueryClient(repairM605QueryConn(t, h))
}

func repairM605QueryConn(t *testing.T, h repairM605Harness) *grpc.ClientConn {
	t.Helper()
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer(grpc.UnaryInterceptor(func(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		return handler(sdk.WrapSDKContext(h.ctx.WithContext(ctx)), req)
	}))
	evidencetypes.RegisterQueryServer(server, h.keeper)
	gogoreflection.Register(server)
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); require.NoError(t, <-served) })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, "passthrough:///evidence-fixture",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	return conn
}

func repairM605JSONBlocks(t *testing.T) []string {
	t.Helper()
	matches := regexp.MustCompile("(?s)```json\\n(.*?)\\n```").FindAllStringSubmatch(repairM605Read(t, "spec/07_client.md"), -1)
	require.Len(t, matches, 3)
	return []string{matches[0][1], matches[1][1], matches[2][1]}
}

func TestRepairM_605_ResponseShapes(t *testing.T) {
	h := repairM605Setup(t)
	producer := repairM605Producer(t)
	repairM605Submit(t, h, producer)
	queryClient := repairM605QueryClient(t, h)
	single, err := queryClient.Evidence(context.Background(), &evidencetypes.QueryEvidenceRequest{EvidenceHash: evidencetypes.FromABCIEvidence(producer).Hash()})
	require.NoError(t, err)
	all, err := queryClient.AllEvidence(context.Background(), &evidencetypes.QueryAllEvidenceRequest{Pagination: &query.PageRequest{CountTotal: true}})
	require.NoError(t, err)
	require.Len(t, all.Evidence, 1)
	require.Equal(t, uint64(1), all.Pagination.Total)
	blocks := repairM605JSONBlocks(t)
	var output bytes.Buffer
	cliCtx := client.Context{}.WithCodec(h.cdc).WithInterfaceRegistry(h.registry).WithOutputFormat("json").WithOutput(&output)
	require.NoError(t, cliCtx.PrintProto(single.Evidence))
	require.JSONEq(t, blocks[0], output.String())
	output.Reset()
	require.NoError(t, cliCtx.PrintProto(all))
	require.JSONEq(t, blocks[1], output.String())
	require.JSONEq(t, blocks[2], string(h.cdc.MustMarshalJSON(single)))

	// Use the node API's current marshaler with the generated HTTP gateway and
	// a real gRPC connection to the keeper, rather than constructing responses.
	marshaler := &gateway.JSONPb{OrigName: true, EmitDefaults: true, Indent: "  ", AnyResolver: h.registry}
	mux := runtime.NewServeMux(runtime.WithMarshalerOption(runtime.MIMEWildcard, marshaler))
	require.NoError(t, evidencetypes.RegisterQueryHandlerClient(context.Background(), mux, queryClient))
	for _, tc := range []struct{ path, want string }{
		{"/cosmos/evidence/v1beta1/evidence/" + url.PathEscape(base64.StdEncoding.EncodeToString(evidencetypes.FromABCIEvidence(producer).Hash())), blocks[2]},
		{"/cosmos/evidence/v1beta1/evidence?pagination.limit=50&pagination.count_total=true", blocks[1]},
	} {
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, tc.path, nil))
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		require.JSONEq(t, tc.want, recorder.Body.String())
	}
	for _, packed := range all.Evidence {
		var decoded exported.Evidence
		require.NoError(t, h.cdc.UnpackAny(packed, &decoded))
		require.Equal(t, evidencetypes.FromABCIEvidence(producer), decoded)
		require.Equal(t, "/cosmos.evidence.v1beta1.Equivocation", packed.TypeUrl)
	}
	empty := repairM605Setup(t)
	emptyResponse, err := evidencetypes.NewQueryClient(empty.queryHelper).AllEvidence(context.Background(), &evidencetypes.QueryAllEvidenceRequest{Pagination: &query.PageRequest{CountTotal: true}})
	require.NoError(t, err)
	require.JSONEq(t, `{"evidence":[],"pagination":{"next_key":null,"total":"0"}}`, string(empty.cdc.MustMarshalJSON(emptyResponse)))
}

func TestRepairM_605_GRPCHash(t *testing.T) {
	h := repairM605Setup(t)
	producer := repairM605Producer(t)
	repairM605Submit(t, h, producer)
	doc := repairM605Read(t, "spec/07_client.md")
	match := regexp.MustCompile(`-d '(\{"evidence_hash":"[^"]+"\})'`).FindStringSubmatch(doc)
	require.Len(t, match, 2)
	conn := repairM605QueryConn(t, h)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	reflectionClient := grpcreflect.NewClientAuto(ctx, conn)
	defer reflectionClient.Reset()
	service, err := reflectionClient.ResolveService("cosmos.evidence.v1beta1.Query")
	require.NoError(t, err)
	method := service.FindMethodByName("Evidence")
	require.NotNil(t, method)
	descriptor := method.GetInputType()
	require.Equal(t, "cosmos.evidence.v1beta1.QueryEvidenceRequest", descriptor.GetFullyQualifiedName())
	request := repairM605ProtobufJSONRequest(t, descriptor, match[1])
	source := evidencetypes.FromABCIEvidence(producer)
	require.Equal(t, source.Hash(), request.EvidenceHash)
	hexMatch := regexp.MustCompile(`hex: +([0-9A-F]{64})`).FindStringSubmatch(doc)
	require.Len(t, hexMatch, 2)
	hexBytes, err := hex.DecodeString(hexMatch[1])
	require.NoError(t, err)
	require.Equal(t, hexBytes, request.EvidenceHash.Bytes())
	require.Contains(t, doc, "paxd query evidence "+hexMatch[1])
	var jsonRequest map[string]string
	require.NoError(t, json.Unmarshal([]byte(match[1]), &jsonRequest))
	require.Contains(t, doc, "base64: "+jsonRequest["evidence_hash"])
	require.Equal(t, base64.StdEncoding.EncodeToString(source.Hash()), jsonRequest["evidence_hash"])
	queryClient := evidencetypes.NewQueryClient(conn)
	response, err := queryClient.Evidence(context.Background(), &request)
	require.NoError(t, err)
	var decoded exported.Evidence
	require.NoError(t, h.cdc.UnpackAny(response.Evidence, &decoded))
	require.Equal(t, source, decoded)
	require.Equal(t, []byte(producer.Validator.Address), decoded.(*evidencetypes.Equivocation).GetConsensusAddress().Bytes())
	require.JSONEq(t, repairM605JSONBlocks(t)[2], string(h.cdc.MustMarshalJSON(response)))
	// Hex characters can themselves be legal base64. Prove that reusing the CLI
	// literal addresses different bytes, rather than silently resolving it.
	wrong := repairM605ProtobufJSONRequest(t, descriptor, `{"evidence_hash":"`+hexMatch[1]+`"}`)
	require.NotEqual(t, request.EvidenceHash, wrong.EvidenceHash)
	_, err = queryClient.Evidence(context.Background(), &wrong)
	require.Equal(t, codes.NotFound, status.Code(err))
}

// grpcurl uses descriptor-driven protobuf JSON. Resolve the descriptor through
// the node's gogo reflection service, decode with that same message implementation,
// then cross protobuf bytes into the generated SDK request.
func repairM605ProtobufJSONRequest(t *testing.T, descriptor *desc.MessageDescriptor, input string) evidencetypes.QueryEvidenceRequest {
	t.Helper()
	message := dynamic.NewMessage(descriptor)
	require.NoError(t, message.UnmarshalJSON([]byte(input)))
	wire, err := message.Marshal()
	require.NoError(t, err)
	var request evidencetypes.QueryEvidenceRequest
	require.NoError(t, request.Unmarshal(wire))
	return request
}

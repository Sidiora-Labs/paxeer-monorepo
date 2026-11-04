package rosetta_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	ros "github.com/coinbase/rosetta-sdk-go/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/rosetta"
	crgerrs "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/rosetta/lib/errors"
	rosserver "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/rosetta/lib/server"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	grpctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/grpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/query"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx/signing"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authsigning "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/signing"
	authtx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/tx"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	bank "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
)

type repairM1312Query struct {
	method string
	key    []byte
	height []string
}
type repairM1312Harness struct {
	client      *rosetta.Client
	converter   rosetta.Converter
	txConfig    client.TxConfig
	keys        []*secp256k1.PrivKey
	addresses   []sdk.AccAddress
	ctx         sdk.Context
	auth        authkeeper.AccountKeeper
	bank        bankkeeper.BaseKeeper
	url         string
	network     *ros.NetworkIdentifier
	mu          sync.Mutex
	queries     []repairM1312Query
	fault       atomic.Int32
	supplyCalls atomic.Int32
	statusCalls atomic.Int32
}

// The bank/auth producer is a real keeper over an in-memory multistore. Only
// consensus status is a recorded wire transcript; no consensus node is needed.
func repairM1312Start(t *testing.T) *repairM1312Harness {
	t.Helper()
	cdc, ir := rosetta.MakeCodec()
	sdk.RegisterInterfaces(ir)
	cfg := authtx.NewTxConfig(cdc, authtx.DefaultSignModes)
	h := &repairM1312Harness{txConfig: cfg, converter: rosetta.NewConverter(cdc, ir, cfg), network: &ros.NetworkIdentifier{Blockchain: "paxeer", Network: "repair-m1312"}}
	db := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	ms := store.NewCommitMultiStore(db)
	bankKey, authKey, paramsKey := sdk.NewKVStoreKey(bank.StoreKey), sdk.NewKVStoreKey(authtypes.StoreKey), sdk.NewKVStoreKey(paramtypes.StoreKey)
	transientKey := sdk.NewTransientStoreKey(paramtypes.TStoreKey)
	for _, key := range []*storetypes.KVStoreKey{bankKey, authKey, paramsKey} {
		ms.MountStoreWithDB(key, storetypes.StoreTypeIAVL, nil)
	}
	ms.MountStoreWithDB(transientKey, storetypes.StoreTypeTransient, nil)
	require.NoError(t, ms.LoadLatestVersion())
	h.ctx = sdk.NewContext(ms, tmproto.Header{Height: 42, ChainID: h.network.Network}, false)
	amino := codec.NewLegacyAmino()
	h.auth = authkeeper.NewAccountKeeper(cdc, authKey, paramtypes.NewSubspace(cdc, amino, paramsKey, transientKey, authtypes.ModuleName), authtypes.ProtoBaseAccount, map[string][]string{})
	h.bank = bankkeeper.NewBaseKeeper(cdc, bankKey, h.auth, paramtypes.NewSubspace(cdc, amino, paramsKey, transientKey, bank.ModuleName), nil)
	for i := 0; i < 4; i++ {
		seed := make([]byte, 32)
		seed[31] = byte(i + 1)
		key := &secp256k1.PrivKey{Key: seed}
		addr := sdk.AccAddress(key.PubKey().Address())
		h.keys = append(h.keys, key)
		h.addresses = append(h.addresses, addr)
		h.auth.SetAccount(h.ctx, authtypes.NewBaseAccount(addr, key.PubKey(), uint64(i+10), uint64(i+20)))
	}
	var assets sdk.Coins
	for i := 0; i < query.DefaultLimit*2+7; i++ {
		assets = append(assets, sdk.NewInt64Coin(fmt.Sprintf("asset%04d", i), int64(i+1)))
	}
	h.bank.InitGenesis(h.ctx, &bank.GenesisState{Params: bank.DefaultParams(), Balances: []bank.Balance{{Address: h.addresses[0].String(), Coins: assets}, {Address: h.addresses[1].String(), Coins: sdk.NewCoins(sdk.NewInt64Coin("zeroasset", 9))}}})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	peer := grpc.NewServer(grpc.UnaryInterceptor(func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		var page *query.PageRequest
		switch q := req.(type) {
		case *bank.QueryAllBalancesRequest:
			page = q.Pagination
		case *bank.QueryTotalSupplyRequest:
			page = q.Pagination
		}
		if page != nil {
			md, _ := metadata.FromIncomingContext(ctx)
			h.mu.Lock()
			h.queries = append(h.queries, repairM1312Query{method: info.FullMethod, key: bytes.Clone(page.Key), height: append([]string(nil), md.Get(grpctypes.GRPCBlockHeightHeader)...)})
			h.mu.Unlock()
		}
		response, err := handler(sdk.WrapSDKContext(h.ctx.WithContext(ctx)), req)
		if err != nil {
			return nil, err
		}
		// Inject a corrupt continuation into real keeper responses to prove the
		// production client refuses looping/unbounded peers rather than truncating.
		switch q := response.(type) {
		case *bank.QueryAllBalancesResponse:
			if h.fault.Load() == 1 {
				q.Pagination.NextKey = []byte("asset0000")
			}
		case *bank.QueryTotalSupplyResponse:
			n := h.supplyCalls.Add(1)
			if h.fault.Load() == 2 {
				q.Pagination.NextKey = []byte("asset0000")
			}
			if h.fault.Load() == 3 {
				q.Pagination.NextKey = []byte(fmt.Sprintf("cursor%08d", n))
			}
		}
		return response, nil
	}))
	bank.RegisterQueryServer(peer, h.bank)
	authtypes.RegisterQueryServer(peer, h.auth)
	served := make(chan error, 1)
	go func() { served <- peer.Serve(listener) }()
	t.Cleanup(func() { peer.Stop(); require.NoError(t, <-served) })
	rpc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if request.Method != "status" {
			http.Error(w, "unexpected transcript method", 400)
			return
		}
		h.statusCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		// The request ID is echoed; result is the committed consensus status fixture.
		_, _ = fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"result":{"node_info":{"network":"repair-m1312"},"sync_info":{"latest_block_height":"42"}}}`, request.ID)
	}))
	t.Cleanup(rpc.Close)
	h.client, err = rosetta.NewClient(&rosetta.Config{Codec: cdc, InterfaceRegistry: ir, GRPCEndpoint: listener.Addr().String(), TendermintRPC: rpc.URL})
	require.NoError(t, err)
	require.Contains(t, h.client.SupportedOperations(), sdk.MsgTypeURL(&bank.MsgMultiSend{}))
	require.NoError(t, h.client.Bootstrap())
	reservation, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := reservation.Addr().String()
	require.NoError(t, reservation.Close())
	server, err := rosserver.NewServer(rosserver.Settings{Client: h.client, Network: h.network, Listen: addr, Offline: true})
	require.NoError(t, err)
	stopped := make(chan error, 1)
	go func() { stopped <- server.Start() }()
	h.url = "http://" + addr
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, server.Shutdown(ctx))
		require.NoError(t, <-stopped)
	})
	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	}, 5*time.Second, 10*time.Millisecond)
	return h
}

func (h *repairM1312Harness) post(t *testing.T, path string, request, response interface{}) int {
	t.Helper()
	data, err := json.Marshal(request)
	require.NoError(t, err)
	httpClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := httpClient.Post(h.url+path, "application/json", bytes.NewReader(data))
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	if resp.StatusCode != http.StatusOK {
		t.Logf("%s: %s", path, body)
	}
	require.NoError(t, json.Unmarshal(body, response), "%s: %s", path, body)
	return resp.StatusCode
}

func (h *repairM1312Harness) operations(t *testing.T, n int) []*ros.Operation {
	t.Helper()
	var inputs []bank.Input
	for i := 0; i < n; i++ {
		inputs = append(inputs, bank.NewInput(h.addresses[i], sdk.NewCoins(sdk.NewInt64Coin("asset0000", 1))))
	}
	msg := bank.NewMsgMultiSend(inputs, []bank.Output{bank.NewOutput(h.addresses[0], sdk.NewCoins(sdk.NewInt64Coin("asset0000", int64(n))))})
	ops, err := h.converter.ToRosetta().Ops("", msg)
	require.NoError(t, err)
	for _, op := range ops {
		op.Status = nil
	}
	return rosetta.AddOperationIndexes(ops, nil)
}

func (h *repairM1312Harness) payloads(t *testing.T, n int, gas uint64) (*ros.ConstructionPayloadsResponse, *rosetta.ConstructionMetadata) {
	t.Helper()
	var pre ros.ConstructionPreprocessResponse
	require.Equal(t, http.StatusOK, h.post(t, "/construction/preprocess", &ros.ConstructionPreprocessRequest{NetworkIdentifier: h.network, Operations: h.operations(t, n), Metadata: map[string]interface{}{"gas_limit": strconv.FormatUint(gas, 10), "gas_price": "1asset0000", "memo": "exact metadata"}}, &pre))
	options, err := json.Marshal(pre.Options)
	require.NoError(t, err)
	var wireOptions map[string]interface{}
	require.NoError(t, json.Unmarshal(options, &wireOptions))
	metadata, err := h.client.ConstructionMetadataFromOptions(context.Background(), wireOptions)
	require.NoError(t, err)
	// Traverse the same standard JSON map transport as /construction/metadata.
	encoded, err := json.Marshal(&ros.ConstructionMetadataResponse{Metadata: metadata})
	require.NoError(t, err)
	var response ros.ConstructionMetadataResponse
	require.NoError(t, json.Unmarshal(encoded, &response))
	var meta rosetta.ConstructionMetadata
	require.NoError(t, meta.FromMetadata(response.Metadata))
	keys := make([]*ros.PublicKey, n)
	for i := range keys {
		keys[i] = &ros.PublicKey{Bytes: h.keys[i].PubKey().Bytes(), CurveType: ros.Secp256k1}
	}
	var payloads ros.ConstructionPayloadsResponse
	require.Equal(t, http.StatusOK, h.post(t, "/construction/payloads", &ros.ConstructionPayloadsRequest{NetworkIdentifier: h.network, Operations: h.operations(t, n), Metadata: response.Metadata, PublicKeys: keys}, &payloads))
	require.Len(t, payloads.Payloads, n)
	return &payloads, &meta
}

func (h *repairM1312Harness) signatures(t *testing.T, payloads *ros.ConstructionPayloadsResponse) []*ros.Signature {
	t.Helper()
	sigs := make([]*ros.Signature, len(payloads.Payloads))
	for i, payload := range payloads.Payloads {
		key, _ := btcec.PrivKeyFromBytes(h.keys[i].Key)
		sigs[i] = &ros.Signature{SigningPayload: payload, PublicKey: &ros.PublicKey{Bytes: h.keys[i].PubKey().Bytes(), CurveType: ros.Secp256k1}, SignatureType: ros.Ecdsa, Bytes: ecdsa.SignCompact(key, payload.Bytes, true)[1:]}
	}
	return sigs
}

func TestRepairM_1312_BankPagination(t *testing.T) {
	h := repairM1312Start(t)
	for _, latest := range []bool{false, true} {
		t.Run(fmt.Sprintf("latest=%t", latest), func(t *testing.T) {
			h.mu.Lock()
			h.queries = nil
			h.mu.Unlock()
			height := int64(42)
			heightPtr := &height
			if latest {
				heightPtr = nil
			}
			ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs(grpctypes.GRPCBlockHeightHeader, "999"))
			amounts, err := h.client.Balances(ctx, h.addresses[0].String(), heightPtr)
			require.NoError(t, err)
			expected := h.bank.GetAllBalances(h.ctx, h.addresses[0])
			require.Len(t, amounts, len(expected)+1)
			for i, coin := range expected {
				require.Equal(t, coin.Denom, amounts[i].Currency.Symbol)
				require.Equal(t, coin.Amount.String(), amounts[i].Value)
			}
			require.Equal(t, "zeroasset", amounts[len(expected)].Currency.Symbol)
			require.Equal(t, "0", amounts[len(expected)].Value)
			h.mu.Lock()
			queries := append([]repairM1312Query(nil), h.queries...)
			h.mu.Unlock()
			require.Len(t, queries, 6)
			for i, q := range queries {
				require.Equal(t, []string{"42"}, q.height)
				if i%3 == 0 {
					require.Empty(t, q.key)
				} else {
					require.NotEmpty(t, q.key)
				}
			}
			require.NotEqual(t, queries[1].key, queries[2].key)
			require.NotEqual(t, queries[4].key, queries[5].key)
		})
	}
	require.EqualValues(t, 1, h.statusCalls.Load())
	for _, fault := range []int32{1, 2, 3} {
		t.Run(fmt.Sprintf("corrupt-continuation-%d", fault), func(t *testing.T) {
			h.fault.Store(fault)
			h.supplyCalls.Store(0)
			height := int64(42)
			amounts, err := h.client.Balances(context.Background(), h.addresses[0].String(), &height)
			require.ErrorIs(t, err, crgerrs.ErrBadGateway)
			require.Nil(t, amounts)
			if fault == 3 {
				require.EqualValues(t, 1000, h.supplyCalls.Load())
			}
		})
	}
}

func TestRepairM_1312_OperationIndex(t *testing.T) {
	h := repairM1312Start(t)
	for _, n := range []int{2, 3, 4} {
		t.Run(fmt.Sprintf("signers-%d", n), func(t *testing.T) {
			ops := h.operations(t, n)
			// A trailing independent message must be processed exactly once.
			tail, err := h.converter.ToRosetta().Ops("", &bank.MsgSend{FromAddress: h.addresses[0].String(), ToAddress: h.addresses[1].String(), Amount: sdk.NewCoins(sdk.NewInt64Coin("asset0000", 7))})
			require.NoError(t, err)
			tx, err := h.converter.ToSDK().UnsignedTx(append(append([]*ros.Operation(nil), ops...), tail...))
			require.NoError(t, err)
			require.Len(t, tx.GetMsgs(), 2)
			require.Len(t, tx.GetMsgs()[0].GetSigners(), n)
			require.Equal(t, h.addresses[:n], tx.GetMsgs()[0].GetSigners())
			require.Equal(t, tail[0].Type, sdk.MsgTypeURL(tx.GetMsgs()[1]))
			for cut := 1; cut < n; cut++ {
				require.NotPanics(t, func() {
					tx, err := h.converter.ToSDK().UnsignedTx(ops[:cut])
					require.ErrorIs(t, err, crgerrs.ErrBadArgument)
					require.Nil(t, tx)
				})
			}
			for sibling := 1; sibling < n; sibling++ {
				for _, mutation := range []string{"type", "metadata", "account", "nil"} {
					t.Run(fmt.Sprintf("sibling-%d-%s", sibling, mutation), func(t *testing.T) {
						altered := h.operations(t, n)
						copyOp := *altered[sibling]
						altered[sibling] = &copyOp
						switch mutation {
						case "type":
							copyOp.Type = "wrong"
						case "metadata":
							copyOp.Metadata = map[string]interface{}{"wrong": "metadata"}
						case "account":
							copyOp.Account = &ros.AccountIdentifier{Address: h.addresses[0].String()}
						case "nil":
							altered[sibling] = nil
						}
						require.NotPanics(t, func() {
							tx, err := h.converter.ToSDK().UnsignedTx(altered)
							require.ErrorIs(t, err, crgerrs.ErrBadArgument)
							require.Nil(t, tx)
						})
					})
				}
			}
		})
	}
}

func TestRepairM_1312_CombineIdentity(t *testing.T) {
	h := repairM1312Start(t)
	payloads, meta := h.payloads(t, 2, 1000000)
	combine := func(t *testing.T, sigs []*ros.Signature, valid bool) {
		t.Helper()
		req := &ros.ConstructionCombineRequest{NetworkIdentifier: h.network, UnsignedTransaction: payloads.UnsignedTransaction, Signatures: sigs}
		if !valid {
			var response ros.Error
			require.Equal(t, http.StatusInternalServerError, h.post(t, "/construction/combine", req, &response))
			require.Equal(t, crgerrs.ToRosetta(crgerrs.ErrInvalidTransaction).Code, response.Code)
			return
		}
		var response ros.ConstructionCombineResponse
		require.Equal(t, http.StatusOK, h.post(t, "/construction/combine", req, &response))
		raw, err := hex.DecodeString(response.SignedTransaction)
		require.NoError(t, err)
		tx, err := h.txConfig.TxDecoder()(raw)
		require.NoError(t, err)
		signed := tx.(authsigning.Tx)
		actual, err := signed.GetSignaturesV2()
		require.NoError(t, err)
		require.Len(t, actual, 2)
		for i, sig := range actual {
			require.Equal(t, h.keys[i].PubKey(), sig.PubKey)
			require.Equal(t, meta.SignersData[i].Sequence, sig.Sequence)
			require.NoError(t, authsigning.VerifySignature(sig.PubKey, authsigning.SignerData{ChainID: meta.ChainID, AccountNumber: meta.SignersData[i].AccountNumber, Sequence: sig.Sequence}, sig.Data, h.txConfig.SignModeHandler(), signed))
		}
		var parsed ros.ConstructionParseResponse
		require.Equal(t, http.StatusOK, h.post(t, "/construction/parse", &ros.ConstructionParseRequest{NetworkIdentifier: h.network, Signed: true, Transaction: response.SignedTransaction}, &parsed))
		require.Len(t, parsed.AccountIdentifierSigners, 2)
		for i, addr := range h.addresses[:2] {
			require.Equal(t, addr.String(), parsed.AccountIdentifierSigners[i].Address)
		}
	}
	combine(t, h.signatures(t, payloads), true)
	for _, mutation := range []string{"wrong-key", "reordered", "wrong-payload", "wrong-account", "wrong-curve", "wrong-type", "wrong-payload-type", "bad-signature", "other-transaction"} {
		t.Run(mutation, func(t *testing.T) {
			sigs := h.signatures(t, payloads)
			// Clone the payload so no refusal mutates the producer's next case.
			payload := *sigs[0].SigningPayload
			payload.Bytes = bytes.Clone(payload.Bytes)
			sigs[0].SigningPayload = &payload
			switch mutation {
			case "wrong-key":
				sigs[0].PublicKey.Bytes = h.keys[2].PubKey().Bytes()
				otherKey, _ := btcec.PrivKeyFromBytes(h.keys[2].Key)
				sigs[0].Bytes = ecdsa.SignCompact(otherKey, payload.Bytes, true)[1:]
			case "reordered":
				sigs[0], sigs[1] = sigs[1], sigs[0]
			case "wrong-payload":
				payload.Bytes[0] ^= 1
				key, _ := btcec.PrivKeyFromBytes(h.keys[0].Key)
				sigs[0].Bytes = ecdsa.SignCompact(key, payload.Bytes, true)[1:]
			case "wrong-account":
				payload.AccountIdentifier = &ros.AccountIdentifier{Address: h.addresses[2].String()}
			case "wrong-curve":
				sigs[0].PublicKey.CurveType = ros.Edwards25519
			case "wrong-type":
				sigs[0].SignatureType = ros.Ed25519
				payload.SignatureType = ros.Ed25519
			case "wrong-payload-type":
				payload.SignatureType = ""
			case "bad-signature":
				sigs[0].Bytes[0] ^= 1
			case "other-transaction":
				other, _ := h.payloads(t, 2, 1000001)
				sigs = h.signatures(t, other)
			}
			combine(t, sigs, false)
		})
	}
	var parsed ros.ConstructionParseResponse
	require.Equal(t, http.StatusOK, h.post(t, "/construction/parse", &ros.ConstructionParseRequest{NetworkIdentifier: h.network, Signed: false, Transaction: payloads.UnsignedTransaction}, &parsed))
	require.Len(t, parsed.Operations, 2)
}

func TestRepairM_1312_MetadataPrecision(t *testing.T) {
	h := repairM1312Start(t)
	for _, value := range []uint64{0, 1<<53 - 1, 1 << 53, 1<<53 + 1, math.MaxUint64} {
		t.Run(strconv.FormatUint(value, 10), func(t *testing.T) {
			// Production auth query packs and unpacks the persisted account with exact values.
			for i := range h.keys[:2] {
				h.auth.SetAccount(h.ctx, authtypes.NewBaseAccount(h.addresses[i], h.keys[i].PubKey(), value, value))
			}
			gas := value
			if gas == 0 {
				gas = 1
			} // preprocess requires a positive gas limit.
			payloads, meta := h.payloads(t, 2, gas)
			require.Equal(t, gas, meta.GasLimit)
			encoded, err := hex.DecodeString(payloads.UnsignedTransaction)
			require.NoError(t, err)
			var unsigned rosetta.UnsignedTransaction
			require.NoError(t, json.Unmarshal(encoded, &unsigned))
			tx, err := h.txConfig.TxDecoder()(unsigned.Transaction)
			require.NoError(t, err)
			signed := tx.(authsigning.Tx)
			require.Equal(t, gas, signed.GetGas())
			sigs, err := signed.GetSignaturesV2()
			require.NoError(t, err)
			for i, data := range meta.SignersData {
				require.Equal(t, value, data.AccountNumber)
				require.Equal(t, value, data.Sequence)
				require.Equal(t, value, sigs[i].Sequence)
				canonical, err := h.txConfig.SignModeHandler().GetSignBytes(signing.SignMode_SIGN_MODE_LEGACY_AMINO_JSON, authsigning.SignerData{ChainID: h.network.Network, AccountNumber: value, Sequence: value}, tx)
				require.NoError(t, err)
				digest := sha256.Sum256(canonical)
				require.Equal(t, digest[:], payloads.Payloads[i].Bytes)
			}
			direct := rosetta.ConstructionMetadata{ChainID: h.network.Network, GasLimit: value, SignersData: []*rosetta.SignerData{{AccountNumber: value, Sequence: value}}}
			metadata, err := direct.ToMetadata()
			require.NoError(t, err)
			require.Equal(t, strconv.FormatUint(value, 10), metadata["gas_limit"])
			wire, err := json.Marshal(metadata)
			require.NoError(t, err)
			var decoded map[string]interface{}
			require.NoError(t, json.Unmarshal(wire, &decoded))
			var exact rosetta.ConstructionMetadata
			require.NoError(t, exact.FromMetadata(decoded))
			require.Equal(t, direct, exact)
		})
	}
	for _, bad := range []interface{}{float64(1<<53 + 2), "18446744073709551616", "-1", "1.5"} {
		var meta rosetta.ConstructionMetadata
		require.Error(t, meta.FromMetadata(map[string]interface{}{"gas_limit": bad}))
	}
}

func TestRepairM_1312_PreservedSuite(t *testing.T) { TestConverterTestSuite(t) }

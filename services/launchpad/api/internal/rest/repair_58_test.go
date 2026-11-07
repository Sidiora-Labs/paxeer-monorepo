package rest

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/ethclient/simulated"
	"github.com/ethereum/go-ethereum/node"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/broker"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

func TestRepair_58_SearchErrors(t *testing.T) {
	frozenBytes, err := os.ReadFile("testdata/contracts/v2/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	var frozen struct {
		Components struct {
			Schemas map[string]json.RawMessage `json:"schemas"`
		} `json:"components"`
	}
	if err := json.Unmarshal(frozenBytes, &frozen); err != nil {
		t.Fatal(err)
	}
	var errorFields []string
	for _, version := range []string{"sdk", "sdk-v2"} {
		source := filepath.Join("../../../../../sdk/launchpad/packages", version, "src/schemas.gen.json")
		raw, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		var sdk struct {
			Definitions map[string]json.RawMessage `json:"$defs"`
		}
		if err := json.Unmarshal(raw, &sdk); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"Error", "ErrorDetails", "FieldError"} {
			var actual, expected any
			if err := json.Unmarshal(sdk.Definitions[name], &actual); err != nil {
				t.Fatal(err)
			}
			normalized := strings.ReplaceAll(string(frozen.Components.Schemas[name]), "#/components/schemas/", "#/$defs/")
			if err := json.Unmarshal([]byte(normalized), &expected); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(actual, expected) {
				t.Fatalf("%s %s schema differs from the frozen REST contract", version, name)
			}
		}
		var errorSchema struct {
			Required []string `json:"required"`
		}
		if err := json.Unmarshal(sdk.Definitions["Error"], &errorSchema); err != nil {
			t.Fatal(err)
		}
		errorFields = errorSchema.Required
		if len(errorFields) != 7 {
			t.Fatalf("%s Error requires %d fields, want 7", version, len(errorFields))
		}
	}
	for _, failure := range []string{"unavailable", "header_timeout", "body_timeout"} {
		t.Run(failure, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if failure == "body_timeout" {
					w.Header().Set("Content-Type", "application/json")
					_, _ = io.WriteString(w, `{"data":`)
					w.(http.Flusher).Flush()
				}
				<-r.Context().Done()
			}))
			defer upstream.Close()
			if failure == "unavailable" {
				upstream.Close()
			}
			router := chi.NewRouter()
			if err := RegisterV2SearchProxy(router, upstream.URL); err != nil {
				t.Fatal(err)
			}
			requestID := uuid.NewString()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Request-ID", requestID)
				router.ServeHTTP(w, r)
			}))
			defer server.Close()
			client := &http.Client{Timeout: 5 * time.Second}
			response, err := client.Get(server.URL + "/v2/search?q=token")
			if err != nil {
				t.Fatal(err)
			}
			body, err := io.ReadAll(response.Body)
			_ = response.Body.Close()
			if err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != http.StatusServiceUnavailable {
				t.Fatalf("status=%d body=%s", response.StatusCode, body)
			}
			if err := interfaces.ValidateJSON("Error", body); err != nil {
				t.Fatalf("frozen Error decoder: %v body=%s", err, body)
			}
			var envelope APIError
			if err := json.Unmarshal(body, &envelope); err != nil {
				t.Fatal(err)
			}
			if envelope.RequestID != requestID || envelope.Code != "DEPENDENCY_UNAVAILABLE" || envelope.Message != v2ErrorRules["DEPENDENCY_UNAVAILABLE"].Message || !envelope.Retryable || envelope.RetryAfterMS == nil || *envelope.RetryAfterMS != 1000 || envelope.FieldErrors == nil || len(envelope.FieldErrors) != 0 || envelope.Details != nil {
				t.Fatalf("incomplete error metadata: %s", body)
			}
			for header, want := range map[string]string{"X-Request-ID": requestID, "X-Schema-Version": "2", "Retry-After": "1", "Content-Type": "application/json", "Cache-Control": "no-store"} {
				if got := response.Header.Get(header); got != want {
					t.Errorf("%s=%q, want %q", header, got, want)
				}
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(body, &fields); err != nil {
				t.Fatal(err)
			}
			if len(fields) != len(errorFields) {
				t.Fatalf("error fields differ from the SDK Error table: %s", body)
			}
			for _, field := range errorFields {
				if _, ok := fields[field]; !ok {
					t.Fatalf("missing SDK Error field %s: %s", field, body)
				}
			}
			decoded, err := interfaces.DecodeError(body)
			if err != nil {
				t.Fatalf("generated frozen Error consumer: %v body=%s", err, body)
			}
			if decoded.Code != "DEPENDENCY_UNAVAILABLE" || decoded.Message != "Authoritative dependency temporarily unavailable" || decoded.RequestId != requestID || !decoded.Retryable || decoded.RetryAfterMs == nil || *decoded.RetryAfterMs != 1000 || decoded.FieldErrors == nil || len(decoded.FieldErrors) != 0 || decoded.Details != nil {
				t.Fatalf("generated consumer lost error metadata: %+v", decoded)
			}
			if string(fields["fieldErrors"]) != "[]" || string(fields["details"]) != "null" {
				t.Fatalf("empty error metadata differs from the SDK contract: %s", body)
			}
		})
	}
}

type repair58Chain struct {
	backend *simulated.Backend
	key     *ecdsa.PrivateKey
	nonce   uint64
	origin  string
	emitter common.Address
	token   common.Address
}

func newRepair58Chain(t *testing.T) *repair58Chain {
	t.Helper()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	backend := simulated.NewBackend(types.GenesisAlloc{crypto.PubkeyToAddress(key.PublicKey): {Balance: new(big.Int).Exp(big.NewInt(10), big.NewInt(21), nil)}}, func(n *node.Config, eth *ethconfig.Config) {
		chainConfig := *eth.Genesis.Config
		chainConfig.ChainID = big.NewInt(125)
		eth.Genesis.Config = &chainConfig
		n.HTTPHost, n.HTTPPort, n.HTTPModules = "127.0.0.1", port, []string{"eth"}
	})
	t.Cleanup(func() { _ = backend.Close() })
	chain := &repair58Chain{backend: backend, key: key, origin: "http://127.0.0.1:" + strconv.Itoa(port)}
	// Runtime copies calldata[96:] and emits LOG3 with the first three words.
	chain.emitter = chain.deploy(t, []byte{0x60, 0x60, 0x36, 0x03, 0x80, 0x60, 0x60, 0x60, 0x00, 0x37, 0x60, 0x40, 0x35, 0x60, 0x20, 0x35, 0x60, 0x00, 0x35, 0x83, 0x60, 0x00, 0xa3, 0x00})
	chain.token = chain.deploy(t, []byte{0x60, 0x12, 0x60, 0x00, 0x52, 0x60, 0x20, 0x60, 0x00, 0xf3})
	return chain
}

func (c *repair58Chain) send(t *testing.T, to *common.Address, data []byte) *types.Receipt {
	t.Helper()
	ctx := context.Background()
	client := c.backend.Client()
	tip, err := client.SuggestGasTipCap(ctx)
	if err != nil {
		t.Fatal(err)
	}
	head, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	fee := new(big.Int).Add(tip, new(big.Int).Mul(head.BaseFee, big.NewInt(2)))
	tx := types.NewTx(&types.DynamicFeeTx{ChainID: big.NewInt(125), Nonce: c.nonce, GasTipCap: tip, GasFeeCap: fee, Gas: 300000, To: to, Data: data})
	signed, err := types.SignTx(tx, types.LatestSignerForChainID(big.NewInt(125)), c.key)
	if err != nil {
		t.Fatal(err)
	}
	if err = client.SendTransaction(ctx, signed); err != nil {
		t.Fatal(err)
	}
	c.nonce++
	c.backend.Commit()
	receipt, err := client.TransactionReceipt(ctx, signed.Hash())
	if err != nil || receipt.Status != types.ReceiptStatusSuccessful {
		t.Fatalf("mined receipt: %v %+v", err, receipt)
	}
	return receipt
}

func (c *repair58Chain) deploy(t *testing.T, runtime []byte) common.Address {
	t.Helper()
	n := byte(len(runtime))
	init := append([]byte{0x60, n, 0x60, 0x0c, 0x60, 0x00, 0x39, 0x60, n, 0x60, 0x00, 0xf3}, runtime...)
	return c.send(t, nil, init).ContractAddress
}

func TestRepair_58_ReceiptRecoveryAfterRestart(t *testing.T) {
	for _, fault := range []string{"rpc_error", "null_receipt"} {
		t.Run(fault, func(t *testing.T) {
			ctx := context.Background()
			dsn, pool := internaltest.NewPostgresWithDSN(t)
			raw, err := os.ReadFile("../../../indexer/migrations/00001_init.sql")
			if err != nil {
				t.Fatal(err)
			}
			up := strings.SplitN(string(raw), "-- +goose Down", 2)[0]
			applied := 0
			for _, block := range strings.Split(up, "-- +goose StatementBegin") {
				statement := strings.SplitN(block, "-- +goose StatementEnd", 2)[0]
				if strings.Contains(statement, `CREATE TABLE "indexer"."swaps"`) || strings.Contains(statement, `CREATE TABLE "indexer"."cursors"`) {
					if _, err := pool.Exec(ctx, statement); err != nil {
						t.Fatal(err)
					}
					applied++
				}
			}
			if applied != 2 {
				t.Fatalf("indexer migration applied %d tables", applied)
			}
			chain := newRepair58Chain(t)
			target, err := url.Parse(chain.origin)
			if err != nil {
				t.Fatal(err)
			}
			proxy := httputil.NewSingleHostReverseProxy(target)
			direct := proxy.Director
			proxy.Director = func(request *http.Request) {
				direct(request)
				// Inspect the real receipt bytes before replacing one batch result.
				// Forwarded gzip negotiation otherwise leaves this hop compressed.
				request.Header.Set("Accept-Encoding", "identity")
			}
			var failing atomic.Bool
			var faults atomic.Int32
			var unresolved string
			proxy.ModifyResponse = func(response *http.Response) error {
				if !failing.Load() {
					return nil
				}
				body, err := io.ReadAll(response.Body)
				_ = response.Body.Close()
				if err != nil {
					return err
				}
				var batch []map[string]json.RawMessage
				if err := json.Unmarshal(body, &batch); err == nil {
					for _, item := range batch {
						var receipt struct {
							TransactionHash string `json:"transactionHash"`
						}
						if json.Unmarshal(item["result"], &receipt) == nil && receipt.TransactionHash == unresolved {
							faults.Add(1)
							if fault == "rpc_error" {
								delete(item, "result")
								item["error"] = json.RawMessage(`{"code":-32000,"message":"temporary receipt failure"}`)
							} else {
								item["result"] = json.RawMessage(`null`)
							}
						}
					}
					body, err = json.Marshal(batch)
					if err != nil {
						return err
					}
				}
				response.Body = io.NopCloser(bytes.NewReader(body))
				response.ContentLength = int64(len(body))
				response.Header.Set("Content-Length", strconv.Itoa(len(body)))
				return nil
			}
			rpcServer := httptest.NewServer(proxy)
			defer rpcServer.Close()
			const poolAddress = "0x7a7a7a7a7a7a7a7a7a7a7a7a7a7a7a7a7a7a7a7a"
			poolID := common.HexToHash("0x" + strings.Repeat("7a", 32))
			sender := crypto.PubkeyToAddress(chain.key.PublicKey)
			if _, err := pool.Exec(ctx, `INSERT INTO indexer.pools(pool_address,token_address,creator,optical,pool_id,created_at,created_block,tx_hash) VALUES($1,$2,$3,$4,$5,1760000000,1,$6)`, poolAddress, strings.ToLower(chain.token.Hex()), strings.ToLower(sender.Hex()), zeroAddressHex, poolID.Hex(), common.Hash{}.Hex()); err != nil {
				t.Fatal(err)
			}
			indexSwap := func() *types.Receipt {
				data := append([]byte{}, common.HexToHash(swapEventTopic).Bytes()...)
				data = append(data, poolID.Bytes()...)
				data = append(data, common.BytesToHash(sender.Bytes()).Bytes()...)
				for _, v := range []int64{1, 1000000, 2000000000000000000, 3000, 1, 0, 0} {
					data = append(data, common.LeftPadBytes(big.NewInt(v).Bytes(), 32)...)
				}
				receipt := chain.send(t, &chain.emitter, data)
				if len(receipt.Logs) != 1 {
					t.Fatalf("real Swap logs: %+v", receipt.Logs)
				}
				header, err := chain.backend.Client().HeaderByNumber(ctx, receipt.BlockNumber)
				if err != nil {
					t.Fatal(err)
				}
				hash := strings.ToLower(receipt.TxHash.Hex())
				if _, err := pool.Exec(ctx, `INSERT INTO indexer.swaps(id,pool_id,pool_address,sender,is_buy,amount_in,amount_out,fee,price,block_number,block_timestamp,tx_hash,log_index) VALUES($1,$2,$3,$4,true,'1000000','2000000000000000000','3000','1',$5,$6,$7,$8)`, hash+"-0", poolID.Hex(), poolAddress, strings.ToLower(sender.Hex()), receipt.BlockNumber.Int64(), int64(header.Time), hash, int32(receipt.Logs[0].Index)); err != nil {
					t.Fatal(err)
				}
				if _, err := pool.Exec(ctx, `INSERT INTO indexer.cursors(chain_id,last_processed_block) VALUES(125,$1) ON CONFLICT(chain_id) DO UPDATE SET last_processed_block=EXCLUDED.last_processed_block`, receipt.BlockNumber.Int64()); err != nil {
					t.Fatal(err)
				}
				return receipt
			}
			open := func() *DevelopmentStream {
				d, err := NewDevelopmentStream(ctx, DevelopmentStreamOptions{Pool: pool, Store: store.New(pool, nil), ChainRPCURL: rpcServer.URL, SigningKey: bytes.Repeat([]byte{58}, 32), Authority: sharedauth.HTTPStreamAuthority{}})
				if err != nil {
					t.Fatal(err)
				}
				return d
			}
			first := indexSwap()
			d := open()
			if n, err := d.projectTrades(ctx, map[string]bool{}); err != nil || n != 1 {
				t.Fatalf("seed projection: %d %v", n, err)
			}
			topics := []string{developmentStreamTradesPrefix + poolAddress}
			binding := sharedauth.StreamTicketBinding{SchemaMajor: 2, Topics: topics, FilterDigest: sharedauth.StreamFilterDigest(topics), Origin: "https://web.example", Endpoint: "/v2/stream"}
			principal := sharedauth.StreamPrincipal{}
			snapshot, err := d.Stream.Snapshot(ctx, principal, binding)
			if err != nil {
				t.Fatal(err)
			}
			var bootstrap struct {
				ResumeCursor string `json:"resumeCursor"`
			}
			if err := json.Unmarshal(snapshot, &bootstrap); err != nil {
				t.Fatal(err)
			}
			second, third := indexSwap(), indexSwap()
			unresolved = strings.ToLower(second.TxHash.Hex())
			failing.Store(true)
			assertHeld := func(d *DevelopmentStream) {
				t.Helper()
				if n, err := d.projectTrades(ctx, map[string]bool{}); err == nil {
					t.Fatalf("unresolved receipt was accepted: projected=%d injected_faults=%d", n, faults.Load())
				}
				block, logIndex, err := d.cursor(ctx, pool, false)
				if err != nil || block != first.BlockNumber.Int64() || logIndex != int32(first.Logs[0].Index) {
					t.Fatalf("cursor crossed unresolved swap: (%d,%d) %v", block, logIndex, err)
				}
				var count int
				if err := pool.QueryRow(ctx, `SELECT count(*) FROM api_stream_dev.stream_delivery_outbox o JOIN api_stream_dev.stream_projection p USING(partition_id) WHERE p.topic=$1`, topics[0]).Scan(&count); err != nil || count != 1 {
					t.Fatalf("partial batch committed: %d %v", count, err)
				}
			}
			assertHeld(d)
			restart := func() {
				d.market.chain.client.Close()
				pool.Close()
				var err error
				pool, err = pgxpool.New(ctx, dsn)
				if err != nil {
					t.Fatal(err)
				}
				d = open()
			}
			restart()
			defer func() { d.market.chain.client.Close(); pool.Close() }()
			assertHeld(d)
			if faults.Load() != 2 {
				t.Fatalf("per-item faults exercised %d times", faults.Load())
			}
			failing.Store(false)
			if n, err := d.projectTrades(ctx, map[string]bool{}); err != nil || n != 2 {
				t.Fatalf("recovery projection: %d %v", n, err)
			}
			block, logIndex, err := d.cursor(ctx, pool, false)
			if err != nil || block != third.BlockNumber.Int64() || logIndex != int32(third.Logs[0].Index) {
				t.Fatalf("recovered cursor: (%d,%d) %v", block, logIndex, err)
			}
			restart()
			if n, err := d.projectTrades(ctx, map[string]bool{}); err != nil || n != 0 {
				t.Fatalf("restart duplicated projection: %d %v", n, err)
			}
			replay, err := d.Stream.Resume(ctx, principal, binding, bootstrap.ResumeCursor)
			if err != nil {
				t.Fatal(err)
			}
			frames, err := replay.Batch(ctx)
			if err != nil || len(frames) != 2 {
				t.Fatalf("recovered consumer replay: %d %v", len(frames), err)
			}
			for i, receipt := range []*types.Receipt{second, third} {
				if err := interfaces.ValidateJSON("MarketTradesEvent", frames[i]); err != nil {
					t.Fatal(err)
				}
				var frame broker.Frame
				if err := json.Unmarshal(frames[i], &frame); err != nil {
					t.Fatal(err)
				}
				wantID := fmt.Sprintf("125:%s:%d", strings.ToLower(receipt.TxHash.Hex()), receipt.Logs[0].Index)
				if frame.EventID != wantID || frame.Sequence != strconv.Itoa(i+2) || frame.Revision != strconv.Itoa(i+2) {
					t.Fatalf("recovered frame: %+v, want %s", frame, wantID)
				}
			}
			if frames, err := replay.Batch(ctx); err != nil || len(frames) != 0 {
				t.Fatalf("duplicate consumer delivery: %d %v", len(frames), err)
			}
			var count int
			if err := pool.QueryRow(ctx, `SELECT count(*) FROM api_stream_dev.stream_delivery_outbox o JOIN api_stream_dev.stream_projection p USING(partition_id) WHERE p.topic=$1`, topics[0]).Scan(&count); err != nil || count != 3 {
				t.Fatalf("exactly-once durable trades: %d %v", count, err)
			}
		})
	}
}

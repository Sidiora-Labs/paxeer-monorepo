package evmrpc_test

// Task 22.33.756, audit 22.30.172. Closure/proof record (kept inside the allowed test path):
// ws-pending-lifecycle: TestRepairM_756_WsPendingLifecycle
// ws-successor-fee-tolerance: TestRepairM_756_WsSuccessorFeeTolerance
// runtime-secret-file: TestRepairM_756_RuntimeSecretFile
// runtime-restart-binding: TestRepairM_756_RuntimeRestartBinding
// tx-byte-failures-ignored: TestRepairM_756_TxByteFailuresIgnored
// load-send-failures-ignored: TestRepairM_756_LoadSendFailuresIgnored
// wasm-error-is-disabled: TestRepairM_756_WasmErrorIsDisabled
// wasm-client-cache-key: TestRepairM_756_WasmClientCacheKey
// Baseline: not run; the ordered implementation-first/hf-done-only instructions
// supersede the embedded pre-change verify step. No baseline failure is claimed.
// Wiring requests: none. The synchronous runtime API is preserved for its callers.
// Qualification finding: 06cf4897e294cc56e449478a291ddb6e6d4b9b0b;
// cg spec done 22.33.756 -f layerx-merge --root /work/paxeer-monorepo;
// exit 1; /out/logs/verify-1.log (full output /tmp/repair-m756.log).
// Goja rejected numeric separators in the adapter's input; normalize their
// spelling as well as type syntax, without changing numeric values or checks.
// Evidence/revision: the runner's private hf-build/hf-done ledger records the exact
// candidate, command, exit and log; no unexecuted gate is claimed here.
//
// The gate embeds the existing Goja interpreter, not a Node subprocess. It erases
// TypeScript annotations from the production functions below, without replacing
// their bodies. Host bindings use real OS files, timers, HTTP/WebSocket sockets,
// and the committed geth transaction/signature codecs. Numeric JS fixtures are
// restricted to exactly representable integers; the BigInt host binding refuses
// unsafe values. Signature r/s and raw bytes remain full-width hex strings.

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	rpcconfig "github.com/Sidiora-Labs/Paxeer-X-Network/rpc/config"
	rpcfixtures "github.com/Sidiora-Labs/Paxeer-X-Network/rpc/tests"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/hd"
	secp "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	paramproposal "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types/proposal"
	wasmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/types"
	bip39 "github.com/cosmos/go-bip39"
	"github.com/dop251/goja"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/tracing"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	evmruntime "github.com/ethereum/go-ethereum/core/vm/runtime"
	"github.com/ethereum/go-ethereum/crypto"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/gorilla/websocket"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

const repair756Utils = "../tests/integration/rpc_tests/utils/"

// Only type syntax is erased. Each function's executable body comes from the
// maintained TS file at gate time, so changing a repair changes what this executes.
func repair756Function(t *testing.T, file, name, jsSignature string) string {
	t.Helper()
	raw, err := os.ReadFile(repair756Utils + file)
	require.NoError(t, err)
	source := string(raw)
	start := strings.Index(source, "function "+name+"(")
	require.GreaterOrEqual(t, start, 0, name)
	body := strings.Index(source[start:], "{\n") + start
	require.Greater(t, body, start, name)
	end := strings.Index(source[body:], "\n}") + body + 2
	require.Greater(t, end, body, name)
	return jsSignature + " " + repair756Erase(source[body:end]) + "\n"
}

func repair756Erase(s string) string {
	s = strings.NewReplacer(
		"(value: any)", "(value)", "(auth: any)", "(auth)",
		"(method: string, params: unknown[]): Promise<any>", "(method, params)",
		"async (key: string): Promise<any>", "async (key)",
		"(outcome): outcome is Extract<LoadOutcome, { kind: 'included' }> =>", "(outcome) =>",
		" as any[]", "", " as ethers.TransactionLike", "", " as 0 | 1", "",
		"let fd: number | undefined", "let fd", "let stored: StoredRuntime", "let stored",
		"const release = (): void =>", "const release = () =>",
		"const outcomes: LoadOutcome[]", "const outcomes",
		"async (signer, index): Promise<LoadOutcome> =>", "async (signer, index) =>",
		"let stage: 'balance' | 'send' | 'receipt'", "let stage",
		"async (): Promise<bigint> =>", "async () =>",
	).Replace(s)
	// This Goja version predates bigint primitives. Every fixture number must
	// pass the exact-integer host check; no signature scalar is converted.
	s = regexp.MustCompile(`\b([0-9][0-9_]*)n\b`).ReplaceAllString(s, "$1")
	return regexp.MustCompile(`\b[0-9][0-9_]*\b`).ReplaceAllStringFunc(s, func(literal string) string {
		return strings.ReplaceAll(literal, "_", "")
	})
}

func repair756WS(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(repair756Utils + "subscribeUtils.ts")
	require.NoError(t, err)
	source := string(raw)
	start := strings.Index(source, "export class WsClient {")
	require.GreaterOrEqual(t, start, 0)
	end := start + strings.Index(source[start:], "\n}") + 2
	s := strings.TrimPrefix(source[start:end], "export ")
	s = strings.NewReplacer(
		"private readonly ", "", "private ", "",
		"new Map<number, PendingCall>()", "new Map()", "new Map<string, any[]>()", "new Map()",
		"failure: Error | undefined;", "failure;",
		"socket: any,", "socket,", "callTimeoutMs: number,", "callTimeoutMs,", "maxBuffer: number,", "maxBuffer,",
		") {\n        socket.onmessage", ") {\n        this.socket = socket; this.callTimeoutMs = callTimeoutMs; this.maxBuffer = maxBuffer;\n        socket.onmessage",
		"(ev: any)", "(ev)", "(globalThis as any)", "globalThis", "const Impl: any", "const Impl",
		"new Promise<WsClient>", "new Promise",
		"url: string", "url", "): Promise<WsClient>", ")",
		"error: Error", "error", "raw: string", "raw", "): void", ")", "let msg: any", "let msg",
		"this.pending.get(msg.id)!", "this.pending.get(msg.id)",
		"method: string", "method", "params: unknown[]", "params", "): Promise<any>", ")",
		"): Promise<string>", ")", "subId: string", "subId", "want: number", "want",
		"): Promise<any[]>", ")", "): Promise<boolean>", ")",
	).Replace(s)
	return repair756Erase(s) + "\n"
}

type repair756Runtime struct {
	t         *testing.T
	vm        *goja.Runtime
	events    chan func()
	timers    map[int]*time.Timer
	nextTimer int
	http      *http.Client
	sockets   []*websocket.Conn
}

func repair756NewRuntime(t *testing.T) *repair756Runtime {
	t.Helper()
	h := &repair756Runtime{t: t, vm: goja.New(), events: make(chan func(), 4096), timers: make(map[int]*time.Timer), http: &http.Client{Timeout: 3 * time.Second}}
	t.Cleanup(func() {
		for _, timer := range h.timers {
			timer.Stop()
		}
		for _, conn := range h.sockets {
			_ = conn.Close()
		}
		h.http.CloseIdleConnections()
	})
	h.set("setTimeout", func(call goja.FunctionCall) goja.Value {
		fn, ok := goja.AssertFunction(call.Argument(0))
		if !ok {
			panic(h.vm.NewTypeError("timer callback"))
		}
		h.nextTimer++
		id := h.nextTimer
		h.timers[id] = time.AfterFunc(time.Duration(call.Argument(1).ToInteger())*time.Millisecond, func() {
			h.events <- func() {
				if _, active := h.timers[id]; !active {
					return
				}
				delete(h.timers, id)
				_, err := fn(goja.Undefined())
				require.NoError(t, err)
			}
		})
		return h.vm.ToValue(id)
	})
	h.set("clearTimeout", func(id int) {
		if timer := h.timers[id]; timer != nil {
			timer.Stop()
			delete(h.timers, id)
		}
	})
	h.set("assert", map[string]any{
		"equal": func(a, b goja.Value, message string) {
			if !a.StrictEquals(b) {
				panic(h.vm.NewGoError(fmt.Errorf("%s: %v != %v", message, a, b)))
			}
		},
		"notEqual": func(a, b goja.Value, message string) {
			if a.StrictEquals(b) {
				panic(h.vm.NewGoError(fmt.Errorf("%s: unexpectedly equal", message)))
			}
		},
		"deepEqual": func(a, b goja.Value, message string) {
			if !reflect.DeepEqual(a.Export(), b.Export()) {
				panic(h.vm.NewGoError(fmt.Errorf("%s: values differ", message)))
			}
		},
	})
	h.run(`assert.match = (value, pattern, message) => { if (typeof value !== 'string' || !pattern.test(value)) throw new Error(message); };
        function check(value, message) { if (!value) throw new Error(message || 'assertion failed'); }
        const sleep = ms => new Promise(resolve => setTimeout(resolve, ms));
        const AbortSignal = { timeout: timeoutMs => ({ timeoutMs }) };
        const console = { info: () => {} };`)
	h.set("BigInt", func(value goja.Value) goja.Value {
		n := h.big(value.String())
		if !n.IsInt64() || n.Cmp(big.NewInt(9007199254740991)) > 0 || n.Cmp(big.NewInt(-9007199254740991)) < 0 {
			panic(h.vm.NewTypeError("fixture integer is outside the exact JS number range"))
		}
		return h.vm.ToValue(n.Int64())
	})
	h.set("URL", func(call goja.ConstructorCall) *goja.Object {
		base, err := http.NewRequest("GET", call.Argument(1).String(), nil)
		if err != nil {
			panic(h.vm.NewGoError(err))
		}
		rel, err := http.NewRequest("GET", call.Argument(0).String(), nil)
		if err != nil {
			panic(h.vm.NewGoError(err))
		}
		resolved := base.URL.ResolveReference(rel.URL).String()
		require.NoError(t, call.This.Set("toString", func() string { return resolved }))
		return call.This
	})
	h.set("fetch", func(call goja.FunctionCall) goja.Value {
		method, body := "GET", ""
		if !goja.IsUndefined(call.Argument(1)) {
			options := call.Argument(1).ToObject(h.vm)
			if v := options.Get("method"); v != nil && !goja.IsUndefined(v) {
				method = v.String()
			}
			if v := options.Get("body"); v != nil && !goja.IsUndefined(v) {
				body = v.String()
			}
		}
		request, err := http.NewRequest(method, call.Argument(0).String(), strings.NewReader(body))
		if err != nil {
			return h.rejected(err)
		}
		request.Header.Set("Content-Type", "application/json")
		response, err := h.http.Do(request)
		if err != nil {
			return h.rejected(err)
		}
		defer response.Body.Close()
		data, err := io.ReadAll(io.LimitReader(response.Body, 1024*1024))
		if err != nil {
			return h.rejected(err)
		}
		obj := h.vm.NewObject()
		require.NoError(t, obj.Set("ok", response.StatusCode >= 200 && response.StatusCode < 300))
		require.NoError(t, obj.Set("status", response.StatusCode))
		require.NoError(t, obj.Set("json", func() goja.Value {
			value, err := h.vm.RunString("JSON.parse(" + strconv.Quote(string(data)) + ")")
			if err != nil {
				return h.rejected(err)
			}
			return h.resolved(value)
		}))
		return h.resolved(obj)
	})
	h.set("WebSocket", func(call goja.ConstructorCall) *goja.Object {
		obj := call.This
		var socket *websocket.Conn
		require.NoError(t, obj.Set("close", func() {
			if socket != nil {
				_ = socket.Close()
			}
		}))
		conn, _, err := websocket.DefaultDialer.Dial(call.Argument(0).String(), nil)
		if err != nil {
			h.events <- func() { h.fire(obj, "onerror", goja.Undefined()) }
			return obj
		}
		socket = conn
		h.sockets = append(h.sockets, conn)
		require.NoError(t, obj.Set("send", func(text string) {
			if err := conn.WriteMessage(websocket.TextMessage, []byte(text)); err != nil {
				panic(h.vm.NewGoError(err))
			}
		}))
		require.NoError(t, obj.Set("close", func() { _ = conn.Close() }))
		h.events <- func() { h.fire(obj, "onopen", goja.Undefined()) }
		go func() {
			for {
				_, data, err := conn.ReadMessage()
				if err != nil {
					h.events <- func() {
						if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
							h.fire(obj, "onerror", goja.Undefined())
						}
						h.fire(obj, "onclose", goja.Undefined())
					}
					return
				}
				text := string(data)
				h.events <- func() { event := h.vm.NewObject(); _ = event.Set("data", text); h.fire(obj, "onmessage", event) }
			}
		}()
		return obj
	})
	h.run(repair756WS(t))
	return h
}

func (h *repair756Runtime) set(name string, value any) {
	h.t.Helper()
	require.NoError(h.t, h.vm.Set(name, value))
}
func (h *repair756Runtime) run(source string) goja.Value {
	h.t.Helper()
	v, err := h.vm.RunString(source)
	require.NoError(h.t, err, "JavaScript production path")
	return v
}
func (h *repair756Runtime) put(name string, value any) {
	h.t.Helper()
	raw, err := json.Marshal(value)
	require.NoError(h.t, err)
	h.run("globalThis[" + strconv.Quote(name) + "] = JSON.parse(" + strconv.Quote(string(raw)) + ");")
}
func (h *repair756Runtime) fire(obj *goja.Object, event string, value goja.Value) {
	h.t.Helper()
	if fn, ok := goja.AssertFunction(obj.Get(event)); ok {
		_, err := fn(obj, value)
		require.NoError(h.t, err)
	}
}
func (h *repair756Runtime) resolved(value goja.Value) goja.Value {
	p, resolve, _ := h.vm.NewPromise()
	resolve(value)
	return h.vm.ToValue(p)
}
func (h *repair756Runtime) rejected(err error) goja.Value {
	p, _, reject := h.vm.NewPromise()
	reject(h.vm.NewGoError(err))
	return h.vm.ToValue(p)
}
func (h *repair756Runtime) await(source string) goja.Value {
	h.t.Helper()
	v := h.run(source)
	p, ok := v.Export().(*goja.Promise)
	require.True(h.t, ok, "expected promise: %s", source)
	deadline := time.NewTimer(8 * time.Second)
	defer deadline.Stop()
	for p.State() == goja.PromiseStatePending {
		select {
		case event := <-h.events:
			event()
		case <-deadline.C:
			h.t.Fatal("production JS promise did not settle:", source)
		}
	}
	require.Equal(h.t, goja.PromiseStateFulfilled, p.State(), "promise rejected: %s", p.Result())
	return p.Result()
}
func (h *repair756Runtime) load(file, name, signature string) {
	h.t.Helper()
	h.run(repair756Function(h.t, file, name, signature))
}
func (h *repair756Runtime) big(value string) *big.Int {
	n, ok := new(big.Int).SetString(value, 0)
	if !ok {
		panic(h.vm.NewTypeError("invalid integer fixture"))
	}
	return n
}

func TestRepairM_756_WsPendingLifecycle(t *testing.T) {
	var connections sync.WaitGroup
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		connections.Add(1)
		defer connections.Done()
		defer conn.Close()
		for {
			var request struct {
				ID     int      `json:"id"`
				Method string   `json:"method"`
				Params []string `json:"params"`
			}
			if conn.ReadJSON(&request) != nil {
				return
			}
			mode := ""
			if len(request.Params) > 0 {
				mode = request.Params[0]
			}
			if mode == "disconnect" {
				return
			}
			if mode == "silent" {
				continue
			}
			result := any("0x01")
			if request.Method == "eth_unsubscribe" {
				result = true
			}
			if conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result}) != nil {
				return
			}
			if mode == "stream" || mode == "overflow" || request.Method == "eth_unsubscribe" {
				for i := 0; i < 3; i++ {
					if conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "method": "eth_subscription", "params": map[string]any{"subscription": "0x01", "result": i}}) != nil {
						return
					}
				}
			}
		}
	}))
	defer server.Close()
	h := repair756NewRuntime(t)
	h.put("url", "ws"+strings.TrimPrefix(server.URL, "http"))
	h.await(`(async () => {
        const timed = await WsClient.open(url, 500, 40, 4);
        const failures = await Promise.allSettled([timed.subscribe(['silent']), timed.subscribe(['silent'])]);
        check(failures.every(result => result.status === 'rejected' && /deadline/.test(String(result.reason))), 'all pending calls must reject at deadline');
        check(timed.pending.size === 0 && timed.buffers.size === 0, 'deadline cleanup');
        const disconnected = await WsClient.open(url, 500, 500, 4);
        const closed = await Promise.allSettled([disconnected.subscribe(['silent']), disconnected.subscribe(['disconnect'])]);
        check(closed.every(result => result.status === 'rejected'), 'close rejects every pending request');
        check(disconnected.pending.size === 0, 'close clears pending requests');
        const stream = await WsClient.open(url, 500, 500, 4);
        const id = await stream.subscribe(['stream']);
        const first = await stream.waitFor(id, 2, 500);
        check(first.length === 2 && first[0] === 0 && first[1] === 1, 'ordered transport delivery');
        const last = await stream.waitFor(id, 1, 500);
        check(last[0] === 2 && stream.buffers.get(id).length === 0, 'notifications are consumed');
        check(await stream.unsubscribe(id), 'unsubscribe transport result');
        await sleep(40);
        check(stream.buffers.size === 0, 'late notifications must not recreate released buffers');
        stream.close();
        const overflow = await WsClient.open(url, 500, 500, 2);
        const overflowId = await overflow.subscribe(['overflow']);
        await sleep(40);
        const failed = await Promise.allSettled([overflow.waitFor(overflowId, 1, 500)]);
        check(failed[0].status === 'rejected' && /overflow/.test(String(failed[0].reason)), 'overflow is an explicit failure');
        check(overflow.pending.size === 0 && overflow.buffers.size === 0, 'overflow releases memory');
        const waiter = await WsClient.open(url, 500, 500, 4);
        const waitingId = await waiter.subscribe(['empty']);
        const pendingWait = waiter.waitFor(waitingId, 1, 500);
        waiter.close();
        const stopped = await Promise.allSettled([pendingWait]);
        check(stopped[0].status === 'rejected', 'close rejects a notification waiter');
    })()`)
}

// Supply real Node host operations to the embedded interpreter, not an in-memory
// substitute for the repaired persistence component.
func repair756Files(h *repair756Runtime, root string) {
	t := h.t
	files := map[int]*os.File{}
	t.Cleanup(func() {
		for _, f := range files {
			_ = f.Close()
		}
	})
	wrap := func(err error) {
		if err != nil {
			panic(h.vm.NewGoError(err))
		}
	}
	file := func(fd int) *os.File {
		f := files[fd]
		if f == nil {
			panic(h.vm.NewTypeError("closed descriptor"))
		}
		return f
	}
	h.set("fs", map[string]any{
		"constants":  map[string]int{"O_WRONLY": syscall.O_WRONLY, "O_RDONLY": syscall.O_RDONLY, "O_CREAT": syscall.O_CREAT, "O_EXCL": syscall.O_EXCL, "O_NOFOLLOW": syscall.O_NOFOLLOW, "O_DIRECTORY": syscall.O_DIRECTORY},
		"existsSync": func(path string) bool { _, err := os.Lstat(path); return err == nil },
		"lstatSync": func(path string) map[string]any {
			stat, err := os.Lstat(path)
			wrap(err)
			return map[string]any{"isFile": func() bool { return stat.Mode().IsRegular() }, "isSymbolicLink": func() bool { return stat.Mode()&os.ModeSymlink != 0 }, "nlink": stat.Sys().(*syscall.Stat_t).Nlink}
		},
		"mkdirSync": func(path string, opts map[string]any) {
			wrap(os.MkdirAll(path, os.FileMode(int(opts["mode"].(int64)))))
		},
		"chmodSync": func(path string, mode int) { wrap(os.Chmod(path, os.FileMode(mode))) },
		"openSync": func(call goja.FunctionCall) goja.Value {
			mode := os.FileMode(0)
			if !goja.IsUndefined(call.Argument(2)) {
				mode = os.FileMode(call.Argument(2).ToInteger())
			}
			f, err := os.OpenFile(call.Argument(0).String(), int(call.Argument(1).ToInteger()), mode)
			wrap(err)
			fd := int(f.Fd())
			files[fd] = f
			return h.vm.ToValue(fd)
		},
		"fchmodSync":    func(fd, mode int) { wrap(file(fd).Chmod(os.FileMode(mode))) },
		"writeFileSync": func(fd int, data, encoding string) { _, err := io.WriteString(file(fd), data); wrap(err) },
		"readFileSync": func(fd int, encoding string) string {
			data, err := io.ReadAll(file(fd))
			wrap(err)
			return string(data)
		},
		"fsyncSync":  func(fd int) { wrap(file(fd).Sync()) },
		"closeSync":  func(fd int) { wrap(file(fd).Close()); delete(files, fd) },
		"renameSync": func(old, new string) { wrap(os.Rename(old, new)) },
		"unlinkSync": func(path string) { wrap(os.Remove(path)) },
	})
	h.set("path", map[string]any{
		"sep": string(filepath.Separator), "dirname": filepath.Dir,
		"join":    func(parts ...string) string { return filepath.Join(parts...) },
		"resolve": func(parts ...string) string { return filepath.Join(parts...) },
		"parse":   func(path string) map[string]string { return map[string]string{"root": string(filepath.Separator)} },
	})
	h.set("os", map[string]any{"homedir": func() string { return root }, "tmpdir": os.TempDir})
	h.set("process", map[string]any{"cwd": func() string { return filepath.Join(root, "suite") }})
	h.set("RuntimeStatePath", "runtime/runtime.json")
	h.set("randomUUID", func() string { b := make([]byte, 16); _, err := rand.Read(b); wrap(err); return hex.EncodeToString(b) })
	h.set("createHash", func(algorithm string) map[string]any {
		require.Equal(t, "sha256", algorithm)
		digest := sha256.New()
		var value map[string]any
		value = map[string]any{"update": func(text string) map[string]any { _, err := digest.Write([]byte(text)); wrap(err); return value }, "digest": func(encoding string) string { return hex.EncodeToString(digest.Sum(nil)) }}
		return value
	})
	h.bindEthers()
}

func (h *repair756Runtime) json(value any) goja.Value {
	h.t.Helper()
	raw, err := json.Marshal(value)
	require.NoError(h.t, err)
	return h.run("JSON.parse(" + strconv.Quote(string(raw)) + ")")
}
func (h *repair756Runtime) bindEthers() {
	ethers := h.vm.NewObject()
	require.NoError(h.t, ethers.Set("toQuantity", func(value goja.Value) string { return hexutil.EncodeBig(h.big(value.String())) }))
	require.NoError(h.t, ethers.Set("keccak256", func(data string) string {
		raw, err := hexutil.Decode(data)
		if err != nil {
			panic(h.vm.NewGoError(err))
		}
		return crypto.Keccak256Hash(raw).Hex()
	}))
	require.NoError(h.t, ethers.Set("dataLength", func(data string) int {
		raw, err := hexutil.Decode(data)
		if err != nil {
			panic(h.vm.NewGoError(err))
		}
		return len(raw)
	}))
	require.NoError(h.t, ethers.Set("Wallet", func(call goja.ConstructorCall) *goja.Object {
		key, err := crypto.HexToECDSA(strings.TrimPrefix(call.Argument(0).String(), "0x"))
		if err != nil {
			panic(h.vm.NewGoError(err))
		}
		_ = call.This.Set("address", crypto.PubkeyToAddress(key.PublicKey).Hex())
		return call.This
	}))
	require.NoError(h.t, ethers.Set("Mnemonic", map[string]any{"isValidMnemonic": bip39.IsMnemonicValid}))
	require.NoError(h.t, ethers.Set("Signature", map[string]any{"from": func(value goja.Value) goja.Value {
		object := value.ToObject(h.vm)
		r := h.big(object.Get("r").String())
		s := h.big(object.Get("s").String())
		v := object.Get("yParity").ToInteger()
		if v < 0 || v > 1 || !crypto.ValidateSignatureValues(byte(v), r, s, true) {
			panic(h.vm.NewTypeError("invalid signature"))
		}
		return h.json(map[string]any{"r": hexutil.Encode(common.LeftPadBytes(r.Bytes(), 32)), "s": hexutil.Encode(common.LeftPadBytes(s.Bytes(), 32)), "yParity": v})
	}}))
	require.NoError(h.t, ethers.Set("Transaction", map[string]any{"from": h.transactionFrom}))
	h.set("ethers", ethers)
}

func (h *repair756Runtime) transactionFrom(value goja.Value) goja.Value {
	fail := func(err error) {
		if err != nil {
			panic(h.vm.NewGoError(err))
		}
	}
	var transaction *ethtypes.Transaction
	if raw, ok := value.Export().(string); ok {
		data, err := hexutil.Decode(raw)
		fail(err)
		transaction = new(ethtypes.Transaction)
		fail(transaction.UnmarshalBinary(data))
	} else {
		object := value.ToObject(h.vm)
		integer := func(name string) *big.Int {
			v := object.Get(name)
			if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
				return new(big.Int)
			}
			return h.big(v.String())
		}
		chain := integer("chainId")
		to := common.HexToAddress(object.Get("to").String())
		var destination *common.Address
		if !goja.IsNull(object.Get("to")) && !goja.IsUndefined(object.Get("to")) {
			destination = &to
		}
		data, err := hexutil.Decode(object.Get("data").String())
		fail(err)
		var access ethtypes.AccessList
		if v := object.Get("accessList"); v != nil && !goja.IsUndefined(v) {
			raw, err := json.Marshal(v.Export())
			fail(err)
			fail(json.Unmarshal(raw, &access))
		}
		switch integer("type").Uint64() {
		case 0:
			transaction = ethtypes.NewTx(&ethtypes.LegacyTx{Nonce: integer("nonce").Uint64(), GasPrice: integer("gasPrice"), Gas: integer("gasLimit").Uint64(), To: destination, Value: integer("value"), Data: data})
		case 1:
			transaction = ethtypes.NewTx(&ethtypes.AccessListTx{ChainID: chain, Nonce: integer("nonce").Uint64(), GasPrice: integer("gasPrice"), Gas: integer("gasLimit").Uint64(), To: destination, Value: integer("value"), Data: data, AccessList: access})
		case 2:
			transaction = ethtypes.NewTx(&ethtypes.DynamicFeeTx{ChainID: chain, Nonce: integer("nonce").Uint64(), GasTipCap: integer("maxPriorityFeePerGas"), GasFeeCap: integer("maxFeePerGas"), Gas: integer("gasLimit").Uint64(), To: destination, Value: integer("value"), Data: data, AccessList: access})
		case 4:
			entries := object.Get("authorizationList").ToObject(h.vm)
			authorizations := make([]ethtypes.SetCodeAuthorization, entries.Get("length").ToInteger())
			for i := range authorizations {
				entry := entries.Get(strconv.Itoa(i)).ToObject(h.vm)
				signature := entry.Get("signature").ToObject(h.vm)
				authorizations[i] = ethtypes.SetCodeAuthorization{ChainID: *uint256.MustFromBig(h.big(entry.Get("chainId").String())), Address: common.HexToAddress(entry.Get("address").String()), Nonce: h.big(entry.Get("nonce").String()).Uint64(), V: byte(signature.Get("yParity").ToInteger()), R: *uint256.MustFromBig(h.big(signature.Get("r").String())), S: *uint256.MustFromBig(h.big(signature.Get("s").String()))}
			}
			transaction = ethtypes.NewTx(&ethtypes.SetCodeTx{ChainID: uint256.MustFromBig(chain), Nonce: integer("nonce").Uint64(), GasTipCap: uint256.MustFromBig(integer("maxPriorityFeePerGas")), GasFeeCap: uint256.MustFromBig(integer("maxFeePerGas")), Gas: integer("gasLimit").Uint64(), To: to, Value: uint256.MustFromBig(integer("value")), Data: data, AccessList: access, AuthList: authorizations})
		default:
			panic(h.vm.NewTypeError("unsupported envelope"))
		}
		signature := object.Get("signature").ToObject(h.vm)
		sig := append(common.LeftPadBytes(h.big(signature.Get("r").String()).Bytes(), 32), common.LeftPadBytes(h.big(signature.Get("s").String()).Bytes(), 32)...)
		sig = append(sig, byte(signature.Get("yParity").ToInteger()))
		var signer ethtypes.Signer = ethtypes.LatestSignerForChainID(chain)
		if transaction.Type() == 0 && chain.Sign() == 0 {
			signer = ethtypes.HomesteadSigner{}
		}
		transaction, err = transaction.WithSignature(signer, sig)
		fail(err)
	}
	raw, err := transaction.MarshalBinary()
	fail(err)
	result := map[string]any{"hash": transaction.Hash().Hex(), "serialized": hexutil.Encode(raw)}
	if transaction.Type() == 4 {
		entries := make([]any, 0, len(transaction.SetCodeAuthorizations()))
		for _, auth := range transaction.SetCodeAuthorizations() {
			entries = append(entries, map[string]any{"address": auth.Address.Hex(), "chainId": auth.ChainID.Uint64(), "nonce": auth.Nonce, "signature": map[string]any{"r": hexutil.Encode(common.LeftPadBytes(auth.R.Bytes(), 32)), "s": hexutil.Encode(common.LeftPadBytes(auth.S.Bytes(), 32)), "yParity": auth.V}})
		}
		result["authorizationList"] = entries
	}
	return h.json(result)
}

// Real Pax application/keeper and production RPC handlers. The existing consensus
// fixture records the application-produced blocks/results; it is not the repair.
type repair756Chain struct {
	application *app.App
	consensus   *rpcfixtures.MockClient
	http        *httptest.Server
	ws          *httptest.Server
	client      *gethrpc.Client
	notifier    *evmrpc.BlockHeaderNotifier
	height      int64
	parent      common.Hash
}

func repair756NewChain(t *testing.T) *repair756Chain {
	t.Helper()
	application, consensus := rpcfixtures.SetupCommittedHeaderFixture(t)
	t.Cleanup(func() { require.NoError(t, application.Close()) })
	config := rpcconfig.DefaultConfig
	config.HTTPPort = 0
	config.WSPort = 0
	config.IPRateLimitRPS = 0
	txConfig := func(int64) client.TxConfig { return application.GetTxConfig() }
	service, err := evmrpc.NewEVMHTTPServer(config, consensus, &application.EvmKeeper, application.BeginBlockKeepers, application.BaseApp, application.TracerAnteHandler, application.RPCContextProvider, txConfig, "", nil)
	require.NoError(t, err)
	t.Cleanup(service.Stop)
	server := httptest.NewServer(service.(http.Handler))
	t.Cleanup(server.Close)
	rpcClient, err := gethrpc.DialHTTP(server.URL)
	require.NoError(t, err)
	t.Cleanup(rpcClient.Close)
	notifier := evmrpc.NewBlockHeaderNotifier(8)
	wsService, err := evmrpc.NewEVMWebSocketServer(config, consensus, &application.EvmKeeper, application.BeginBlockKeepers, application.BaseApp, application.TracerAnteHandler, application.RPCContextProvider, txConfig, "", nil, notifier)
	require.NoError(t, err)
	t.Cleanup(wsService.Stop)
	wsServer := httptest.NewServer(wsService.(http.Handler))
	t.Cleanup(wsServer.Close)
	one := int64(1)
	block, err := consensus.Block(context.Background(), &one)
	require.NoError(t, err)
	results, err := consensus.BlockResults(context.Background(), &one)
	require.NoError(t, err)
	consensus.RecordCommittedBlock(block, results)
	store := application.EvmKeeper.ReceiptStore()
	require.NotNil(t, store)
	require.NoError(t, store.SetLatestVersion(1))
	require.NoError(t, store.SetEarliestVersion(1))
	return &repair756Chain{application: application, consensus: consensus, http: server, ws: wsServer, client: rpcClient, notifier: notifier, height: 1, parent: common.BytesToHash(block.BlockID.Hash)}
}
func (f *repair756Chain) commit(t *testing.T, txs [][]byte, update func(sdk.Context)) {
	t.Helper()
	height := f.height + 1
	data := tmtypes.Data{}
	for _, tx := range txs {
		data.Txs = append(data.Txs, tmtypes.Tx(tx))
	}
	header := tmtypes.Header{ChainID: f.application.ChainID, Height: height, Time: time.Now().UTC(), LastBlockID: tmtypes.BlockID{Hash: f.parent.Bytes()}, AppHash: bytes.Clone(f.application.LastCommitID().Hash), DataHash: data.Txs.Hash(), LastResultsHash: common.HexToHash("0x03").Bytes(), ProposerAddress: common.HexToAddress("0x0405").Bytes(), ValidatorsHash: common.HexToHash("0x06").Bytes()}
	hash := common.BigToHash(big.NewInt(height + 100))
	request := &abci.RequestFinalizeBlock{Hash: hash.Bytes(), Header: header.ToProto(), Txs: txs}
	response, err := f.application.FinalizeBlock(context.Background(), request)
	require.NoError(t, err)
	for _, result := range response.TxResults {
		require.Equal(t, abci.CodeTypeOK, result.Code, result.Log)
	}
	if update != nil {
		update(f.application.GetContextForDeliverTx(nil))
	}
	f.notifier.Stash(request, response)
	_, err = f.application.Commit(context.Background())
	require.NoError(t, err)
	block := &coretypes.ResultBlock{BlockID: tmtypes.BlockID{Hash: hash.Bytes()}, Block: &tmtypes.Block{Header: header, Data: data, LastCommit: &tmtypes.Commit{Height: height - 1}}}
	f.consensus.RecordCommittedBlock(block, &coretypes.ResultBlockResults{Height: height, TxsResults: response.TxResults})
	store := f.application.EvmKeeper.ReceiptStore()
	require.NotNil(t, store)
	require.NoError(t, store.SetLatestVersion(height))
	require.NoError(t, store.SetEarliestVersion(1))
	for _, raw := range txs {
		decoded, err := f.application.GetTxConfig().TxDecoder()(raw)
		require.NoError(t, err)
		for _, message := range decoded.GetMsgs() {
			if evm, ok := message.(*evmtypes.MsgEVMTransaction); ok {
				transaction, _ := evm.AsTransaction()
				require.NotNil(t, transaction)
				require.Eventually(t, func() bool {
					_, err := f.application.EvmKeeper.GetReceipt(f.application.RPCContextProvider(evmrpc.LatestCtxHeight), transaction.Hash())
					return err == nil
				}, 5*time.Second, time.Millisecond)
			}
		}
	}
	f.height = height
	f.parent = hash
	require.True(t, f.notifier.PublishStashed())
}
func repair756Provider(h *repair756Runtime, c *gethrpc.Client) {
	h.set("provider", map[string]any{"send": func(method string, params []any) goja.Value {
		var result any
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := c.CallContext(ctx, &result, method, params...); err != nil {
			return h.rejected(err)
		}
		return h.resolved(h.json(result))
	}})
}
func repair756CanonicalFunctions(h *repair756Runtime) {
	raw, err := os.ReadFile(repair756Utils + "subscribeUtils.ts")
	require.NoError(h.t, err)
	s := string(raw)
	a := strings.Index(s, "export const NEW_HEAD_FIELDS")
	b := a + strings.Index(s[a:], "/** Assert a newHeads")
	require.GreaterOrEqual(h.t, a, 0)
	require.Greater(h.t, b, a)
	h.run(strings.NewReplacer("export ", "", " as const", "").Replace(s[a:b]))
	raw, err = os.ReadFile(repair756Utils + "format.ts")
	require.NoError(h.t, err)
	for _, line := range strings.Split(string(raw), "\n") {
		if regexp.MustCompile(`^export const (HEX_QUANTITY|ADDRESS|HEX_DATA|HASH32|BLOOM256|NONCE8) =`).MatchString(line) {
			h.run(strings.TrimPrefix(line, "export "))
		}
	}
	h.load("subscribeUtils.ts", "assertNewHeadSchema", "function assertNewHeadSchema(head)")
	h.load("subscribeUtils.ts", "assertNewHeadMatchesBlock", "function assertNewHeadMatchesBlock(head, block)")
	h.load("subscribeUtils.ts", "assertNewHeadBaseFeeTracksChain", "async function assertNewHeadBaseFeeTracksChain(head, provider)")
}
func TestRepairM_756_WsSuccessorFeeTolerance(t *testing.T) {
	f := repair756NewChain(t)
	h := repair756NewRuntime(t)
	repair756CanonicalFunctions(h)
	repair756Provider(h, f.client)
	h.put("wsURL", "ws"+strings.TrimPrefix(f.ws.URL, "http"))
	h.await(`(async () => {globalThis.headClient = await WsClient.open(wsURL); globalThis.headId = await headClient.subscribe(['newHeads']);})()`)
	_, evmTxs := rpcfixtures.CommittedHeaderTransactions(t, f.application)
	f.commit(t, evmTxs, func(ctx sdk.Context) { f.application.EvmKeeper.SetNextBaseFeePerGas(ctx, sdk.NewDec(777777777)) })
	h.await(`(async () => { globalThis.announced = (await headClient.waitFor(headId, 1, 3000))[0]; await assertNewHeadBaseFeeTracksChain(announced, provider); })()`)
	require.NotEqual(t, "0x0", h.run("announced.gasUsed").String(), "real executed gas must be compared")
	require.NotEqual(t, hexutil.Encode(make([]byte, 256)), h.run("announced.logsBloom").String(), "real emitted logs must be compared")
	f.commit(t, nil, nil)
	h.await(`(async () => {
        await assertNewHeadBaseFeeTracksChain(announced, provider);
        const successor = (await headClient.waitFor(headId, 1, 3000))[0];
        check(successor.baseFeePerGas !== announced.baseFeePerGas, 'fixture must have unequal adjacent fees');
        const wrongFee = { ...announced, baseFeePerGas: successor.baseFeePerGas };
        const failure = await Promise.allSettled([assertNewHeadBaseFeeTracksChain(wrongFee, provider)]);
        check(failure[0].status === 'rejected', 'successor fee must never validate this height');
        const block = await provider.send('eth_getBlockByNumber', [announced.number, false]);
        for (const field of NEW_HEAD_FIELDS) {
            const changed = { ...announced };
            const value = announced[field];
            changed[field] = value === '0x' ? '0x00' : value.slice(0,-1) + (value.endsWith('1') ? '2' : '1');
            let failed = false;
            try { assertNewHeadMatchesBlock(changed, block); } catch { failed = true; }
            check(failed, 'complete field equality: ' + field);
        }
        headClient.close();
    })()`)
}

func repair756SnapshotSource(t *testing.T) string {
	raw, err := os.ReadFile(repair756Utils + "testUtils.ts")
	require.NoError(t, err)
	s := string(raw)
	marker := "const RUNTIME_SNAPSHOT_SOURCE = String.raw`"
	a := strings.Index(s, marker)
	require.GreaterOrEqual(t, a, 0)
	a += len(marker)
	b := strings.Index(s[a:], "`;\n")
	require.Greater(t, b, 0)
	return s[a : a+b]
}
func repair756RuntimeState(t *testing.T, h *repair756Runtime, f *repair756Chain, root string) {
	repair756Files(h, root)
	h.put("Endpoints", map[string]any{"pax": map[string]string{"evmRpc": f.http.URL}, "eth": map[string]string{"geth": f.http.URL}})
	for _, function := range []struct{ name, signature string }{
		{"bootstrapRequired", "function bootstrapRequired()"}, {"stateAbsPath", "function stateAbsPath()"},
		{"validateRuntimeState", "function validateRuntimeState(state)"}, {"captureRuntimeBinding", "function captureRuntimeBinding(state, previous)"},
		{"secureRuntimeDirectory", "function secureRuntimeDirectory(abs)"}, {"writeRuntimeState", "function writeRuntimeState(state)"}, {"readRuntimeState", "function readRuntimeState()"},
	} {
		h.load("testUtils.ts", function.name, function.signature)
	}
	h.set("queryRuntimeSnapshot", func(request goja.Value) goja.Value {
		// Node's worker boundary is another independent interpreter. It runs the
		// exact production guest source and real loopback HTTP, without toolchains.
		worker := repair756NewRuntime(t)
		worker.run(repair756Erase(repair756SnapshotSource(t)))
		worker.put("request", request.Export())
		return h.json(worker.await("runtimeChainSnapshot(request)").Export())
	})
}
func repair756State(t *testing.T) map[string]any {
	t.Helper()
	key := common.LeftPadBytes([]byte{1}, 32)
	private, err := crypto.ToECDSA(key)
	require.NoError(t, err)
	address := crypto.PubkeyToAddress(private.PublicKey).Hex()
	account := map[string]string{"address": address, "privateKey": hexutil.Encode(key)}
	contract := common.HexToAddress("0x1460").Hex()
	return map[string]any{"bootstrappedAt": "2026-01-01T00:00:00.000Z", "chainIds": map[string]int{"pax": 125, "eth": 125}, "blocks": map[string]int{"paxBeforeDeploy": 0, "paxErc20Deploy": 1, "paxAfterDeploy": 1, "ethAtBootstrap": 1, "ethErc20Deploy": 1}, "contracts": map[string]string{"erc20": contract, "erc20Geth": contract, "simpleAccount7702": contract, "gasBurner": contract}, "funded": map[string]any{"admin": address, "adminMnemonic": "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about", "gethAdmin": account, "pool": []any{account}}}
}
func TestRepairM_756_RuntimeSecretFile(t *testing.T) {
	f := repair756NewChain(t)
	root := t.TempDir()
	h := repair756NewRuntime(t)
	repair756RuntimeState(t, h, f, root)
	h.put("state", repair756State(t))
	h.run("writeRuntimeState(state)")
	abs := h.run("stateAbsPath()").String()
	require.NotContains(t, abs, "reports")
	require.NotContains(t, abs, filepath.Join(root, "suite"), "default secrets are outside the checkout")
	checkModes := func() {
		directory, err := os.Stat(filepath.Dir(abs))
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0700), directory.Mode().Perm())
		file, err := os.Stat(abs)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0600), file.Mode().Perm())
	}
	checkModes()
	require.NoError(t, os.Chmod(filepath.Dir(abs), 0755))
	require.NoError(t, os.Chmod(abs, 0644))
	h.run("readRuntimeState()")
	checkModes()
	// An independent filesystem consumer must always see a complete old or new
	// record while the real writer fsyncs and atomically replaces the file.
	stop := make(chan struct{})
	observed := make(chan error, 1)
	go func() {
		for {
			select {
			case <-stop:
				observed <- nil
				return
			default:
			}
			raw, err := os.ReadFile(abs)
			if err != nil {
				observed <- err
				return
			}
			var stored map[string]any
			if err = json.Unmarshal(raw, &stored); err != nil {
				observed <- err
				return
			}
			if stored["schemaVersion"] != float64(1) {
				observed <- fmt.Errorf("incomplete runtime record")
				return
			}
		}
	}()
	h.run("writeRuntimeState(state)")
	close(stop)
	require.NoError(t, <-observed)
	checkModes()
	entries, err := os.ReadDir(filepath.Dir(abs))
	require.NoError(t, err)
	require.Len(t, entries, 1, "temporary secret files must be cleaned up")
	h.run(`RuntimeStatePath = 'reports/runtime.json'; let rejected = false; try {stateAbsPath();} catch {rejected = true;} check(rejected, 'report path must reject secrets'); RuntimeStatePath = 'runtime/runtime.json';`)
	target := filepath.Join(root, "victim")
	require.NoError(t, os.WriteFile(target, []byte("untouched"), 0644))
	require.NoError(t, os.Remove(abs))
	require.NoError(t, os.Symlink(target, abs))
	h.run(`let linkRejected=false;try{writeRuntimeState(state);}catch{linkRejected=true;}check(linkRejected,'symlink must be refused');`)
	raw, err := os.ReadFile(target)
	require.NoError(t, err)
	require.Equal(t, "untouched", string(raw))
}
func TestRepairM_756_RuntimeRestartBinding(t *testing.T) {
	f := repair756NewChain(t)
	root := t.TempDir()
	writer := repair756NewRuntime(t)
	repair756RuntimeState(t, writer, f, root)
	writer.put("state", repair756State(t))
	writer.run("writeRuntimeState(state)")
	abs := writer.run("stateAbsPath()").String()
	original, err := os.ReadFile(abs)
	require.NoError(t, err)
	// Stop the original consumer, reopen the persisted state in a fresh runtime,
	// and verify it against actual current production RPC state before use.
	writer.http.CloseIdleConnections()
	writer.vm = nil
	reopened := repair756NewRuntime(t)
	repair756RuntimeState(t, reopened, f, root)
	recovered := reopened.run("JSON.stringify(readRuntimeState())").String()
	expected, err := json.Marshal(repair756State(t))
	require.NoError(t, err)
	require.JSONEq(t, string(expected), recovered)
	mustReject := func() {
		reopened.run(`(()=>{let staleRejected=false;try{readRuntimeState();}catch(error){staleRejected=/bootstrap required/.test(String(error));}check(staleRejected,'stale state must require bootstrap');})()`)
	}
	var stored map[string]any
	require.NoError(t, json.Unmarshal(original, &stored))
	stored["schemaVersion"] = 2
	raw, err := json.Marshal(stored)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(abs, raw, 0600))
	mustReject()
	require.NoError(t, os.WriteFile(abs, original, 0600))
	reopened.run("Endpoints.pax.evmRpc += '/replacement'")
	mustReject()
	reopened.put("Endpoints", map[string]any{"pax": map[string]string{"evmRpc": f.http.URL}, "eth": map[string]string{"geth": f.http.URL}})
	// Corrupt a stored genesis identity and ensure no process cache bypasses it.
	require.NoError(t, json.Unmarshal(original, &stored))
	chains := stored["binding"].(map[string]any)["chains"].([]any)
	chains[0].(map[string]any)["genesis"] = common.HexToHash("0x99").Hex()
	raw, err = json.Marshal(stored)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(abs, raw, 0600))
	mustReject()
	require.NoError(t, os.WriteFile(abs, original, 0600))
	reopened.run("readRuntimeState()")
	// Replace the deployed code through the real keeper and commit it. Reopen
	// again: the same endpoint and chain id must not bless a stale deployment.
	f.commit(t, nil, func(ctx sdk.Context) {
		f.application.EvmKeeper.SetCode(ctx, common.HexToAddress("0x1460"), common.FromHex("0x60006000fd"))
	})
	var currentCode string
	require.NoError(t, f.client.CallContext(context.Background(), &currentCode, "eth_getCode", common.HexToAddress("0x1460"), "latest"))
	require.Equal(t, "0x60006000fd", currentCode)
	restarted := repair756NewRuntime(t)
	repair756RuntimeState(t, restarted, f, root)
	restarted.run(`let codeRejected=false;try{readRuntimeState();}catch(error){codeRejected=/bootstrap required/.test(String(error));}check(codeRejected,'replaced deployed code must invalidate reopened fixtures');`)
}

func TestRepairM_756_TxByteFailuresIgnored(t *testing.T) {
	h := repair756NewRuntime(t)
	h.bindEthers()
	h.load("txUtils.ts", "reconstructTx", "function reconstructTx(tx)")
	h.load("txUtils.ts", "assertActualBytesAndSize", "function assertActualBytesAndSize(block)")
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	to := common.HexToAddress("0x1460")
	chain := big.NewInt(125)
	authorization, err := ethtypes.SignSetCode(key, ethtypes.SetCodeAuthorization{ChainID: *uint256.NewInt(125), Address: to, Nonce: 1})
	require.NoError(t, err)
	inner := []ethtypes.TxData{
		&ethtypes.LegacyTx{Nonce: 0, To: &to, Gas: 21000, GasPrice: big.NewInt(100), Value: big.NewInt(1)},
		&ethtypes.AccessListTx{ChainID: chain, Nonce: 1, To: &to, Gas: 30000, GasPrice: big.NewInt(100), Value: big.NewInt(2), AccessList: ethtypes.AccessList{{Address: to, StorageKeys: []common.Hash{common.HexToHash("0x1")}}}},
		&ethtypes.DynamicFeeTx{ChainID: chain, Nonce: 2, To: &to, Gas: 21000, GasFeeCap: big.NewInt(100), GasTipCap: big.NewInt(2), Value: big.NewInt(3)},
		&ethtypes.SetCodeTx{ChainID: uint256.NewInt(125), Nonce: 3, To: to, Gas: 100000, GasFeeCap: uint256.NewInt(100), GasTipCap: uint256.NewInt(2), Value: uint256.NewInt(4), AuthList: []ethtypes.SetCodeAuthorization{authorization}},
	}
	transactions := make([]*ethtypes.Transaction, 0, len(inner))
	rpcTransactions := make([]any, 0, len(inner))
	total := 0
	for _, data := range inner {
		signed, err := ethtypes.SignTx(ethtypes.NewTx(data), ethtypes.LatestSignerForChainID(chain), key)
		require.NoError(t, err)
		raw, err := signed.MarshalBinary()
		require.NoError(t, err)
		total += len(raw)
		transactions = append(transactions, signed)
		rpcBytes, err := json.Marshal(signed)
		require.NoError(t, err)
		var rpcTx map[string]any
		require.NoError(t, json.Unmarshal(rpcBytes, &rpcTx))
		rpcTransactions = append(rpcTransactions, rpcTx)
	}
	block := ethtypes.NewBlockWithHeader(&ethtypes.Header{Number: big.NewInt(1), GasLimit: 1000000, Difficulty: new(big.Int)}).WithBody(ethtypes.Body{Transactions: transactions})
	h.put("block", map[string]any{"transactions": rpcTransactions, "size": hexutil.EncodeUint64(uint64(block.Size()))})
	h.put("totalBytes", total)
	h.run(`const verified = assertActualBytesAndSize(block); check(verified.verified === 4 && verified.txBytes === totalBytes, 'all signed envelopes including authorization lists must round-trip');
        for (let index=0; index<block.transactions.length; index++) {
            const corrupted = JSON.parse(JSON.stringify(block)); corrupted.transactions[index].hash = '0x' + '99'.repeat(32);
            let rejected = false; try { assertActualBytesAndSize(corrupted); } catch { rejected=true; }
            check(rejected, 'one corrupt hash cannot hide behind good transactions');
            const malformed = JSON.parse(JSON.stringify(block)); delete malformed.transactions[index].r;
            rejected=false; try {assertActualBytesAndSize(malformed);} catch {rejected=true;}
            check(rejected,'one malformed signed envelope must fail');
        }
        const wrongAuthorization = JSON.parse(JSON.stringify(block)); wrongAuthorization.transactions[3].authorizationList[0].nonce='0x2';
        let authRejected=false;try{assertActualBytesAndSize(wrongAuthorization);}catch{authRejected=true;}check(authRejected,'type-4 authorization bytes must affect signed hash');
        const missingAuthorization=JSON.parse(JSON.stringify(block));delete missingAuthorization.transactions[3].authorizationList;
        let missingRejected=false;try{assertActualBytesAndSize(missingAuthorization);}catch{missingRejected=true;}check(missingRejected,'missing authorization list is a failure');
        const hashOnly={...block,transactions:[block.transactions[0].hash]};let hashOnlyRejected=false;try{assertActualBytesAndSize(hashOnly);}catch{hashOnlyRejected=true;}check(hashOnlyRejected,'full signed byte proof cannot omit hash-only transactions');`)
}

func TestRepairM_756_WasmErrorIsDisabled(t *testing.T) {
	f := repair756NewChain(t)
	var mode atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/cosmos/params/v1beta1/params", r.URL.Path)
		require.Equal(t, "wasm", r.URL.Query().Get("subspace"))
		switch mode.Load() {
		case 1:
			http.Error(w, "query unavailable", 503)
			return
		case 2:
			_, _ = io.WriteString(w, "not JSON")
			return
		case 3:
			_, _ = io.WriteString(w, `{"param":{"subspace":"wasm","key":"uploadAccess","value":"{}"}}`)
			return
		}
		// Produce the REST envelope using the actual parameter query keeper and
		// committed wasm policy, including its JSON-inside-JSON value encoding.
		response, err := f.application.ParamsKeeper.Params(sdk.WrapSDKContext(f.application.RPCContextProvider(evmrpc.LatestCtxHeight)), &paramproposal.QueryParamsRequest{Subspace: "wasm", Key: r.URL.Query().Get("key")})
		require.NoError(t, err)
		require.NoError(t, json.NewEncoder(w).Encode(response))
	}))
	defer server.Close()
	h := repair756NewRuntime(t)
	h.put("Endpoints", map[string]any{"pax": map[string]string{"rest": server.URL}})
	h.load("wasmUtils.ts", "wasmCapability", "function wasmCapability(params)")
	h.load("wasmUtils.ts", "isWasmEnabled", "async function isWasmEnabled()")
	require.True(t, h.await("isWasmEnabled()").ToBoolean(), "selected-chain Everybody/Everybody policy")
	for _, failure := range []int32{1, 2, 3} {
		mode.Store(failure)
		h.await(`(async()=>{const result=await Promise.allSettled([isWasmEnabled()]);check(result[0].status==='rejected','transport and decoding failures must propagate');})()`)
	}
	mode.Store(0)
	f.commit(t, nil, func(ctx sdk.Context) {
		params := f.application.WasmKeeper.GetParams(ctx)
		params.CodeUploadAccess = wasmtypes.AllowNobody
		f.application.WasmKeeper.SetParams(ctx, params)
	})
	require.False(t, h.await("isWasmEnabled()").ToBoolean(), "an explicit committed disabled policy is optional")
	server.Close()
	h.await(`(async()=>{const result=await Promise.allSettled([isWasmEnabled()]);check(result[0].status==='rejected','unreachable selected chain must not disable the test');})()`)
}

func TestRepairM_756_WasmClientCacheKey(t *testing.T) {
	f := repair756NewChain(t)
	alternate := httptest.NewServer(f.http.Config.Handler)
	t.Cleanup(alternate.Close)
	h := repair756NewRuntime(t)
	h.put("alternateEndpoint", alternate.URL)
	h.put("originalEndpoint", f.http.URL)
	var connected, disconnected atomic.Int32
	h.run("let adminClient;")
	h.put("Endpoints", map[string]any{"pax": map[string]string{"cosmosRpc": f.http.URL}})
	h.set("PAX_HD_PATH", "m/44'/118'/0'/0/0")
	h.set("stringToPath", func(value string) string { return value })
	h.set("DirectSecp256k1HdWallet", map[string]any{"fromMnemonic": func(mnemonic string, options map[string]any) goja.Value {
		// Real BIP39/BIP32/secp256k1 signer identity, matching CosmJS' path.
		derived, err := hd.Secp256k1.Derive()(mnemonic, "", "m/44'/118'/0'/0/0")
		if err != nil {
			return h.rejected(err)
		}
		private := secp.PrivKey{Key: derived}
		address, err := sdk.Bech32ifyAddressBytes("pax", private.PubKey().Address())
		if err != nil {
			return h.rejected(err)
		}
		wallet := h.vm.NewObject()
		_ = wallet.Set("getAccounts", func() goja.Value { return h.resolved(h.json([]any{map[string]string{"address": address}})) })
		return h.resolved(wallet)
	}})
	h.set("SigningCosmWasmClient", map[string]any{"connectWithSigner": func(endpoint string, wallet goja.Value) goja.Value {
		connection, err := gethrpc.DialHTTP(endpoint)
		if err != nil {
			return h.rejected(err)
		}
		var chain string
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err = connection.CallContext(ctx, &chain, "eth_chainId"); err != nil {
			connection.Close()
			return h.rejected(err)
		}
		connected.Add(1)
		client := h.vm.NewObject()
		var closed bool
		_ = client.Set("disconnect", func() {
			if !closed {
				closed = true
				connection.Close()
				disconnected.Add(1)
			}
		})
		return h.resolved(client)
	}})
	h.set("connectionCount", func() int32 { return connected.Load() })
	h.set("disconnectionCount", func() int32 { return disconnected.Load() })
	h.load("wasmUtils.ts", "discardAdmin", "function discardAdmin(entry)")
	h.load("wasmUtils.ts", "leaseAdminConnection", "async function leaseAdminConnection(endpoint, address, connect)")
	h.load("wasmUtils.ts", "adminCosmWasm", "async function adminCosmWasm(mnemonic)")
	// Public BIP39 test vectors, never credentials. Cache keys must contain only
	// the endpoint and public identity, even while a pending lease is replaced.
	h.put("mnemonicA", "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about")
	h.put("mnemonicB", "legal winner thank year wave sausage worth useful legal winner thank yellow")
	h.await(`(async()=>{
        const [first, same] = await Promise.all([adminCosmWasm(mnemonicA), adminCosmWasm(mnemonicA)]);
        check(first.client === same.client && first.address === same.address && connectionCount()===1,'same endpoint/signer shares connection');
        check(!adminClient.key.includes(mnemonicA),'cache keys must not contain mnemonic');
        const changed=await adminCosmWasm(mnemonicB);
        check(changed.address!==first.address && changed.client!==first.client,'new mnemonic selects its own signer');
        check(disconnectionCount()===0,'an active old signer must remain connected');
        first.release();check(disconnectionCount()===0,'other lease is active');same.release();await Promise.resolve();
        check(disconnectionCount()===1,'discarded idle signer must disconnect');same.release();
        check(disconnectionCount()===1,'release is idempotent');
        changed.release();
        Endpoints.pax.cosmosRpc = alternateEndpoint;
        const moved=await adminCosmWasm(mnemonicB);
        check(moved.address===changed.address && moved.client!==changed.client,'same signer at a new endpoint needs a new connection');
        await Promise.resolve();check(disconnectionCount()===2,'replaced endpoint is disconnected');
        moved.release();discardAdmin(adminClient);adminClient=undefined;await Promise.resolve();
        check(disconnectionCount()===3,'discarded final connection is closed');
    })()`)
	// A failed connection must not poison the cache; a later call retries it.
	h.run("Endpoints.pax.cosmosRpc = originalEndpoint")
	f.http.Close()
	h.await(`(async()=>{
        const failures=await Promise.allSettled([adminCosmWasm(mnemonicA)]);
        check(failures[0].status==='rejected' && adminClient===undefined,'failed cache entry is removed');
        check(!String(failures[0].reason).includes(mnemonicA),'failure diagnostics cannot print the mnemonic');
    })()`)
}

// A loopback Ethereum peer backed by the committed geth state/VM/transaction
// processor. Successful receipts and reverts come from actual EVM execution.
type repair756LoadAPI struct {
	mu       sync.Mutex
	state    *state.StateDB
	header   *ethtypes.Header
	receipts map[common.Hash]*ethtypes.Receipt
}

func (api *repair756LoadAPI) GetBlockByNumber(_ context.Context, _ string, _ bool) (*ethtypes.Header, error) {
	api.mu.Lock()
	defer api.mu.Unlock()
	return ethtypes.CopyHeader(api.header), nil
}
func (api *repair756LoadAPI) GetBalance(_ context.Context, address common.Address, _ string) (*hexutil.Big, error) {
	api.mu.Lock()
	defer api.mu.Unlock()
	return (*hexutil.Big)(api.state.GetBalance(address).ToBig()), nil
}
func (api *repair756LoadAPI) GetTransactionCount(_ context.Context, address common.Address, _ string) (hexutil.Uint64, error) {
	api.mu.Lock()
	defer api.mu.Unlock()
	return hexutil.Uint64(api.state.GetNonce(address)), nil
}
func (api *repair756LoadAPI) GetTransactionReceipt(_ context.Context, hash common.Hash) (*ethtypes.Receipt, error) {
	api.mu.Lock()
	defer api.mu.Unlock()
	receipt := api.receipts[hash]
	if receipt == nil {
		return nil, nil
	}
	// Ethereum RPC represents an empty log list as [], including on a real
	// reverted transaction. Preserve every execution field from the processor.
	wire := *receipt
	wire.Logs = append(make([]*ethtypes.Log, 0, len(receipt.Logs)), receipt.Logs...)
	return &wire, nil
}
func (api *repair756LoadAPI) SendRawTransaction(_ context.Context, raw hexutil.Bytes) (common.Hash, error) {
	api.mu.Lock()
	defer api.mu.Unlock()
	transaction := new(ethtypes.Transaction)
	if err := transaction.UnmarshalBinary(raw); err != nil {
		return common.Hash{}, err
	}
	header := ethtypes.CopyHeader(api.header)
	header.Number = new(big.Int).Add(header.Number, big.NewInt(1))
	header.Time++
	cfg := &evmruntime.Config{State: api.state, BlockNumber: header.Number, Time: header.Time, GasLimit: header.GasLimit, BaseFee: header.BaseFee}
	evmruntime.SetDefaults(cfg)
	cfg.ChainConfig.ChainID = big.NewInt(125)
	environment := evmruntime.NewEnv(cfg)
	api.state.SetTxContext(transaction.Hash(), 0)
	snapshot := api.state.Snapshot()
	usedGas := uint64(0)
	receipt, err := core.ApplyTransaction(environment, new(core.GasPool).AddGas(header.GasLimit), api.state, header, transaction, &usedGas)
	if err != nil {
		api.state.RevertToSnapshot(snapshot)
		return common.Hash{}, err
	}
	api.header = header
	api.receipts[transaction.Hash()] = receipt
	return transaction.Hash(), nil
}
func repair756LoadSigner(h *repair756Runtime, rpcClient *gethrpc.Client, key *ecdsa.PrivateKey, chain int64) goja.Value {
	address := crypto.PubkeyToAddress(key.PublicKey)
	balance := func() goja.Value {
		var result hexutil.Big
		if err := rpcClient.CallContext(context.Background(), &result, "eth_getBalance", address, "latest"); err != nil {
			return h.rejected(err)
		}
		value := (*big.Int)(&result)
		if !value.IsInt64() || value.Cmp(big.NewInt(9007199254740991)) > 0 {
			panic(h.vm.NewTypeError("unsafe fixture balance"))
		}
		return h.resolved(h.vm.ToValue(value.Int64()))
	}
	send := func(request *goja.Object) goja.Value {
		var nonce hexutil.Uint64
		if err := rpcClient.CallContext(context.Background(), &nonce, "eth_getTransactionCount", address, "pending"); err != nil {
			return h.rejected(err)
		}
		data, err := hexutil.Decode(request.Get("data").String())
		if err != nil {
			return h.rejected(err)
		}
		to := common.HexToAddress(request.Get("to").String())
		transaction := ethtypes.NewTx(&ethtypes.DynamicFeeTx{ChainID: big.NewInt(chain), Nonce: uint64(nonce), To: &to, Data: data, Gas: uint64(request.Get("gasLimit").ToInteger()), GasFeeCap: big.NewInt(request.Get("maxFeePerGas").ToInteger()), GasTipCap: big.NewInt(request.Get("maxPriorityFeePerGas").ToInteger()), Value: new(big.Int)})
		signed, err := ethtypes.SignTx(transaction, ethtypes.LatestSignerForChainID(big.NewInt(chain)), key)
		if err != nil {
			return h.rejected(err)
		}
		raw, err := signed.MarshalBinary()
		if err != nil {
			return h.rejected(err)
		}
		var hash common.Hash
		if err = rpcClient.CallContext(context.Background(), &hash, "eth_sendRawTransaction", hexutil.Bytes(raw)); err != nil {
			return h.rejected(err)
		}
		result := h.vm.NewObject()
		_ = result.Set("hash", hash.Hex())
		_ = result.Set("wait", func(confirmations, timeout int) goja.Value {
			require.Equal(h.t, 1, confirmations)
			require.Equal(h.t, 60000, timeout)
			var receipt *ethtypes.Receipt
			if err := rpcClient.CallContext(context.Background(), &receipt, "eth_getTransactionReceipt", hash); err != nil {
				return h.rejected(err)
			}
			if receipt == nil {
				return h.resolved(goja.Null())
			}
			return h.resolved(h.json(map[string]any{"hash": receipt.TxHash.Hex(), "status": receipt.Status, "blockNumber": receipt.BlockNumber.Int64(), "gasUsed": receipt.GasUsed}))
		})
		return h.resolved(result)
	}
	signer := h.vm.NewObject()
	_ = signer.Set("balance", balance)
	_ = signer.Set("wallet", map[string]any{"sendTransaction": send})
	return signer
}
func TestRepairM_756_LoadSendFailuresIgnored(t *testing.T) {
	database, err := state.New(ethtypes.EmptyRootHash, state.NewDatabaseForTesting())
	require.NoError(t, err)
	burner := common.HexToAddress("0x1460")
	reverter := common.HexToAddress("0x1461")
	database.SetCode(burner, common.FromHex("0x600160005500"))
	database.SetCode(reverter, common.FromHex("0x60006000fd"))
	keys := make([]*ecdsa.PrivateKey, 3)
	for i := range keys {
		keys[i], err = crypto.GenerateKey()
		require.NoError(t, err)
		if i < 2 {
			database.SetBalance(crypto.PubkeyToAddress(keys[i].PublicKey), uint256.NewInt(1000000000), tracing.BalanceChangeUnspecified)
		}
	}
	peer := &repair756LoadAPI{state: database, header: &ethtypes.Header{Number: new(big.Int), Time: 1700000000, GasLimit: 30000000, BaseFee: big.NewInt(100), Difficulty: new(big.Int)}, receipts: make(map[common.Hash]*ethtypes.Receipt)}
	rpcServer := gethrpc.NewServer()
	require.NoError(t, rpcServer.RegisterName("eth", peer))
	t.Cleanup(rpcServer.Stop)
	server := httptest.NewServer(rpcServer)
	t.Cleanup(server.Close)
	rpcClient, err := gethrpc.DialHTTP(server.URL)
	require.NoError(t, err)
	t.Cleanup(rpcClient.Close)
	h := repair756NewRuntime(t)
	h.bindEthers()
	repair756Provider(h, rpcClient)
	const burnerABI = `[{"name":"burnGasIterations","type":"function","stateMutability":"nonpayable","inputs":[{"name":"salt","type":"uint256"},{"name":"iterations","type":"uint256"}],"outputs":[]}]`
	parsed, err := abi.JSON(strings.NewReader(burnerABI))
	require.NoError(t, err)
	h.set("abiOf", func(file, name string) string {
		require.Equal(t, "GasBurner.sol", file)
		require.Equal(t, "RealGasBurner", name)
		return burnerABI
	})
	ethers := h.vm.Get("ethers").ToObject(h.vm)
	require.NoError(t, ethers.Set("Interface", func(call goja.ConstructorCall) *goja.Object {
		require.Equal(t, burnerABI, call.Argument(0).String())
		_ = call.This.Set("encodeFunctionData", func(method string, values []goja.Value) string {
			args := make([]any, len(values))
			for i, value := range values {
				args[i] = h.big(value.String())
			}
			data, err := parsed.Pack(method, args...)
			if err != nil {
				panic(h.vm.NewGoError(err))
			}
			return hexutil.Encode(data)
		})
		return call.This
	}))
	h.set("signerA", repair756LoadSigner(h, rpcClient, keys[0], 125))
	h.set("signerB", repair756LoadSigner(h, rpcClient, keys[1], 125))
	h.set("emptySigner", repair756LoadSigner(h, rpcClient, keys[2], 125))
	h.set("wrongChainSigner", repair756LoadSigner(h, rpcClient, keys[0], 126))
	h.put("runtime", map[string]any{"contracts": map[string]string{"gasBurner": burner.Hex()}})
	h.put("reverter", reverter.Hex())
	h.load("txUtils.ts", "requireIncludedLoad", "function requireIncludedLoad(outcomes, intended)")
	h.load("txUtils.ts", "burnGasBurst", "async function burnGasBurst(provider, runtime, signers, opts = {})")
	h.await(`(async()=>{
        let roundsObserved=0;
        const options={rounds:2,gasLimit:100000,iterations:1,tip:0,onRound:async()=>{roundsObserved++;}};
        const loaded=await burnGasBurst(provider,runtime,[signerA,signerB],options);
        check(loaded.beforeBaseFee===100 && loaded.minBlock===1 && loaded.maxBlock===4 && roundsObserved===2,'intended successful included workload');
        const empty=await Promise.allSettled([burnGasBurst(provider,runtime,[emptySigner],options)]);
        check(empty[0].status==='rejected' && /balance: insufficient balance/.test(String(empty[0].reason)),'underfunded signer must not be silently skipped');
        const rejected=await Promise.allSettled([burnGasBurst(provider,runtime,[wrongChainSigner],options)]);
        check(rejected[0].status==='rejected' && /send: operation rejected/.test(String(rejected[0].reason)),'real signature/chain rejection must fail load setup');
        const reverted=await Promise.allSettled([burnGasBurst(provider,{contracts:{gasBurner:reverter}},[signerA],options)]);
        check(reverted[0].status==='rejected' && /receipt:/.test(String(reverted[0].reason)),'included reverted execution is not successful load');
        const bounded=await Promise.allSettled([burnGasBurst(provider,runtime,Array(12).fill(emptySigner),options)]);
        const diagnostic=String(bounded[0].reason);
        check(bounded[0].status==='rejected' && /failures=12/.test(diagnostic) && diagnostic.length<1200 && !diagnostic.includes('signer=8'),'failure diagnostics retain total and bounded samples');
        const noSigners=await Promise.allSettled([burnGasBurst(provider,runtime,[],options)]);
        check(noSigners[0].status==='rejected','empty workload cannot return a zero height');
    })()`)
	require.Len(t, peer.receipts, 5, "four successful executions plus the real reverted transaction")
}

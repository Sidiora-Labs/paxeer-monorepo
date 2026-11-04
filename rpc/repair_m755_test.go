package evmrpc_test

import (
	"context"
	"encoding/base64"
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
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	proposal "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types/proposal"
	"github.com/dop251/goja"
	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/parser"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm/runtime"
	"github.com/stretchr/testify/require"
)

// Task 22.33.755 closure map:
// rpc-request-deadline -> TestRepairM_755_RequestDeadline
// rpc-parameter-failure-mask -> TestRepairM_755_Parameters
// rpc-default-chain-id -> TestRepairM_755_NetworkIdentity
// cosmos-docker-chain-binding -> TestRepairM_755_DockerChainBinding
// feehistory-optional-formula -> TestRepairM_755_FeeHistory
// gasprice-neighborhood-formula -> TestRepairM_755_GasPrice
// gasprice-neighbor-unit-contract -> TestRepairM_755_NeighborUnitContract
// log0-shape -> TestRepairM_755_Log0
// Baseline was not run: the task's ordered gates forbid pre-implementation verification.
// Prior attempt: RequestDeadline, PollingDeadline and GasPrice failed while parsing
// waitUntil in the test's TypeScript lowering, before their behavior could execute.
// Wiring request: domain 16 must update tests/integration/rpc_tests/eth/eth_chainId.spec.ts's
// independent historical default to consume the merged identity in utils/constants.ts.
// Spec records are runner-owned and outside this task's permitted edit scope.

// These tests execute the production TypeScript function bodies in the existing
// in-process JS engine. Only type annotations are erased. The engine predates
// BigInt, so its arithmetic expressions are lowered to Go's arbitrary precision
// integers; no fee calculation or helper implementation is reproduced here.
type repair755JS struct {
	t         *testing.T
	vm        *goja.Runtime
	events    chan func()
	done      chan struct{}
	timers    map[int64]*time.Timer
	nextTimer int64
	cancels   []context.CancelFunc
}

func repair755Source(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "tests", "integration", "rpc_tests", "utils", name))
	require.NoError(t, err)
	return string(data)
}

func repair755Matching(s string, start int, open, close byte) int {
	depth := 0
	var quote byte
	escaped := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '\'' || c == '"' || c == '`' {
			quote = c
			continue
		}
		if c == open {
			depth++
		}
		if c == close {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	panic("unclosed source delimiter")
}

func repair755EraseParams(s string) string {
	var parts []string
	begin, round, square, curly, angle := 0, 0, 0, 0, 0
	for i, c := range s + "," {
		switch c {
		case '(':
			round++
		case ')':
			round--
		case '[':
			square++
		case ']':
			square--
		case '{':
			curly++
		case '}':
			curly--
		case '<':
			angle++
		case '>':
			if angle > 0 && (i == 0 || s[i-1] != '=') {
				angle--
			}
		case ',':
			if round == 0 && square == 0 && curly == 0 && angle == 0 {
				part := strings.TrimSpace(s[begin:i])
				begin = i + 1
				colon := strings.Index(part, ":")
				if colon >= 0 {
					name := strings.TrimSuffix(strings.TrimSpace(part[:colon]), "?")
					// A default assignment follows the type; function arrows do not.
					tail := part[colon+1:]
					assignment := regexp.MustCompile(`\s*=[^=>]`).FindStringIndex(tail)
					if assignment != nil {
						name += tail[assignment[0]:]
					}
					part = name
				}
				if part != "" {
					parts = append(parts, part)
				}
			}
		}
	}
	return strings.Join(parts, ", ")
}

func repair755Function(t *testing.T, filename, name string) string {
	t.Helper()
	source := repair755Source(t, filename)
	re := regexp.MustCompile(`(?:export\s+)?(async\s+)?function\s+` + regexp.QuoteMeta(name) + `\b\s*`)
	loc := re.FindStringSubmatchIndex(source)
	require.NotNil(t, loc, "%s declares %s", filename, name)
	start := loc[1]
	if source[start] == '<' {
		start = repair755Matching(source, start, '<', '>') + 1
	}
	for start < len(source) && strings.ContainsRune(" \t\r\n", rune(source[start])) {
		start++
	}
	require.Less(t, start, len(source))
	require.Equal(t, byte('('), source[start], "%s parameter list", name)
	end := repair755Matching(source, start, '(', ')')
	body := end + 1
	angle := 0
	for ; body < len(source); body++ {
		if source[body] == '<' {
			angle++
		}
		if source[body] == '>' && angle > 0 {
			angle--
		}
		if source[body] == '{' && angle == 0 {
			// An object return type is followed by the actual function body.
			prefix := strings.TrimSpace(source[end+1 : body])
			if prefix == ":" {
				body = repair755Matching(source, body, '{', '}')
				continue
			}
			break
		}
	}
	last := repair755Matching(source, body, '{', '}')
	async := ""
	if loc[2] >= 0 {
		async = "async "
	}
	code := async + "function " + name + "(" + repair755EraseParams(source[start+1:end]) + ") " + source[body:last+1]
	for _, pair := range [][2]string{
		{" as Record<string, unknown>", ""},
		{" as { HostIp: string; HostPort: string }[] | null", ""},
		{" as const", ""}, {" as T", ""},
	} {
		code = strings.ReplaceAll(code, pair[0], pair[1])
	}
	code = regexp.MustCompile(`\s+as\s+\{\s*HostIp:\s*string;\s*HostPort:\s*string;?\s*\}\[\]\s*\|\s*null`).ReplaceAllString(code, "")
	code = regexp.MustCompile(`\bas Promise<[^;\n]+>`).ReplaceAllString(code, "")
	code = regexp.MustCompile(`:\s*ReturnType\s*<\s*typeof\s+setTimeout\s*>\s*\|\s*undefined`).ReplaceAllString(code, "")
	code = regexp.MustCompile(`:\s*Promise<(?:any|string|number|never|void|boolean|T)>`).ReplaceAllString(code, "")
	code = regexp.MustCompile(`:\s*(?:string|number|unknown|any)(?:\[\])?(\s*[,)=;])`).ReplaceAllString(code, "$1")
	code = regexp.MustCompile(`:\s*(?:string|number|unknown|any)(\s*=)`).ReplaceAllString(code, "$1")
	code = regexp.MustCompile(`([A-Za-z_$][\w$]*)<(?:[^<>]|<[^<>]*>)+>\s*\(`).ReplaceAllString(code, "$1(")
	code = regexp.MustCompile(`([\w\]])!([.\[])`).ReplaceAllString(code, "$1$2")
	return code
}

// Traverse syntax nodes, lowering arithmetic rather than substituting a copied
// implementation. Logical operators retain JavaScript's short-circuit semantics.
func repair755Parse(t *testing.T, source string) *ast.Program {
	t.Helper()
	program, err := parser.ParseFile(nil, "production.ts", source, 0)
	if err == nil {
		return program
	}
	lines := strings.Split(source, "\n")
	if location := regexp.MustCompile(`Line ([0-9]+):`).FindStringSubmatch(err.Error()); location != nil {
		line, conversionErr := strconv.Atoi(location[1])
		if conversionErr == nil && line > 0 && line <= len(lines) {
			t.Fatalf("TypeScript lowering did not parse: %v\noffending lowered line %d: %s", err, line, lines[line-1])
		}
	}
	var numbered strings.Builder
	for i, line := range lines {
		fmt.Fprintf(&numbered, "%4d | %s\n", i+1, line)
	}
	t.Fatalf("TypeScript lowering did not parse: %v\n%s", err, numbered.String())
	return nil
}

func repair755Lower(t *testing.T, source string) string {
	t.Helper()
	source = regexp.MustCompile(`\b([0-9][0-9_]*)n\b`).ReplaceAllStringFunc(source, func(s string) string {
		return "BigInt(" + strconv.Quote(strings.ReplaceAll(strings.TrimSuffix(s, "n"), "_", "")) + ")"
	})
	source = regexp.MustCompile(`\b[0-9]+(?:_[0-9]+)+\b`).ReplaceAllStringFunc(source, func(s string) string {
		return strings.ReplaceAll(s, "_", "")
	})
	program := repair755Parse(t, source)
	var nodes []*ast.BinaryExpression
	seen := map[uintptr]bool{}
	var visit func(reflect.Value)
	visit = func(v reflect.Value) {
		if !v.IsValid() {
			return
		}
		switch v.Kind() {
		case reflect.Interface:
			if !v.IsNil() {
				visit(v.Elem())
			}
		case reflect.Pointer:
			if v.IsNil() || seen[v.Pointer()] {
				return
			}
			seen[v.Pointer()] = true
			if v.CanInterface() {
				if b, ok := v.Interface().(*ast.BinaryExpression); ok && strings.Contains("|+|-|*|/|%|>|>=|<|<=|===|!==|==|!=|", "|"+b.Operator.String()+"|") {
					nodes = append(nodes, b)
				}
			}
			visit(v.Elem())
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				if v.Type().Field(i).IsExported() {
					visit(v.Field(i))
				}
			}
		case reflect.Slice:
			for i := 0; i < v.Len(); i++ {
				visit(v.Index(i))
			}
		}
	}
	visit(reflect.ValueOf(program))
	// This goja version ends a conditional's span at its test expression.
	// Follow the alternate through enclosing binaries to retain the whole operand.
	var expressionEnd func(ast.Node) int
	expressionEnd = func(node ast.Node) int {
		switch expression := node.(type) {
		case *ast.ConditionalExpression:
			return expressionEnd(expression.Alternate)
		case *ast.BinaryExpression:
			return expressionEnd(expression.Right)
		default:
			return int(node.Idx1()) - 1
		}
	}
	// AST positions omit grouping parentheses. Include any unmatched grouping
	// delimiters before replacing a binary expression with a function call.
	span := func(node ast.Node) (int, int) {
		left, right := int(node.Idx0())-1, expressionEnd(node)
		balance, minimum := 0, 0
		var quote byte
		escaped := false
		for i := left; i < right; i++ {
			c := source[i]
			if quote != 0 {
				if escaped {
					escaped = false
					continue
				}
				if c == '\\' {
					escaped = true
					continue
				}
				if c == quote {
					quote = 0
				}
				continue
			}
			if c == '\'' || c == '"' || c == '`' {
				quote = c
				continue
			}
			if c == '(' {
				balance++
			}
			if c == ')' {
				balance--
				if balance < minimum {
					minimum = balance
				}
			}
		}
		for need := -minimum; need > 0; {
			left--
			require.GreaterOrEqual(t, left, 0, "missing leading expression group")
			if source[left] == '(' {
				need--
			}
		}
		for need := balance - minimum; need > 0; {
			require.Less(t, right, len(source), "missing trailing expression group")
			if source[right] == ')' {
				need--
			}
			right++
		}
		return left, right
	}
	sort.Slice(nodes, func(i, j int) bool {
		il, ir := span(nodes[i])
		jl, jr := span(nodes[j])
		if il == jl {
			return ir > jr
		}
		return il < jl
	})
	var render func(int, int) string
	render = func(start, end int) string {
		var out strings.Builder
		at := start
		for _, b := range nodes {
			l, r := span(b)
			if l < at || r > end {
				continue
			}
			out.WriteString(source[at:l])
			ll, lr := span(b.Left)
			rl, rr := span(b.Right)
			out.WriteString("__binary(" + strconv.Quote(b.Operator.String()) + "," + render(ll, lr) + "," + render(rl, rr) + ")")
			at = r
		}
		out.WriteString(source[at:end])
		return out.String()
	}
	lowered := render(0, len(source))
	repair755Parse(t, lowered)
	return lowered
}

func (j *repair755JS) eval(code string) goja.Value {
	j.t.Helper()
	lowered := repair755Lower(j.t, code)
	value, err := j.vm.RunString(lowered)
	require.NoError(j.t, err, "%s", lowered)
	return value
}

func (j *repair755JS) load(filename string, names ...string) {
	j.t.Helper()
	for _, name := range names {
		j.eval(repair755Function(j.t, filename, name))
	}
}

func (j *repair755JS) await(code string) (goja.Value, error) {
	j.t.Helper()
	value, err := j.vm.RunString(repair755Lower(j.t, code))
	if err != nil {
		return nil, err
	}
	p, ok := value.Export().(*goja.Promise)
	if !ok {
		return value, nil
	}
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	for p.State() == goja.PromiseStatePending {
		select {
		case event := <-j.events:
			event()
		case <-deadline.C:
			return nil, fmt.Errorf("in-process JS promise failed to settle: %s", code)
		}
	}
	if p.State() == goja.PromiseStateRejected {
		return nil, fmt.Errorf("%s", p.Result().String())
	}
	return p.Result(), nil
}

func (j *repair755JS) enqueue(fn func()) {
	select {
	case j.events <- fn:
	case <-j.done:
	}
}

func repair755Big(v goja.Value) (*big.Int, bool) {
	if v == nil || goja.IsNull(v) || goja.IsUndefined(v) {
		return nil, false
	}
	object, ok := v.(*goja.Object)
	if !ok {
		return nil, false
	}
	field := object.Get("__bigint")
	if field == nil || goja.IsUndefined(field) {
		return nil, false
	}
	n, ok := new(big.Int).SetString(field.String(), 10)
	return n, ok
}

func newRepair755JS(t *testing.T) *repair755JS {
	t.Helper()
	j := &repair755JS{t: t, vm: goja.New(), events: make(chan func(), 256), done: make(chan struct{}), timers: map[int64]*time.Timer{}}
	t.Cleanup(func() {
		close(j.done)
		for _, timer := range j.timers {
			timer.Stop()
		}
		for _, cancel := range j.cancels {
			cancel()
		}
	})
	require.NoError(t, j.vm.Set("setTimeout", func(call goja.FunctionCall) goja.Value {
		callback, ok := goja.AssertFunction(call.Argument(0))
		if !ok {
			panic(j.vm.NewTypeError("timer callback"))
		}
		j.nextTimer++
		id := j.nextTimer
		j.timers[id] = time.AfterFunc(time.Duration(call.Argument(1).ToInteger())*time.Millisecond, func() {
			j.enqueue(func() { delete(j.timers, id); _, err := callback(goja.Undefined()); require.NoError(t, err) })
		})
		return j.vm.ToValue(id)
	}))
	require.NoError(t, j.vm.Set("clearTimeout", func(call goja.FunctionCall) goja.Value {
		id := call.Argument(0).ToInteger()
		if timer := j.timers[id]; timer != nil {
			timer.Stop()
			delete(j.timers, id)
		}
		return goja.Undefined()
	}))
	require.NoError(t, j.vm.Set("__integer", func(value string) string {
		n, ok := new(big.Int).SetString(value, 0)
		if !ok {
			n, ok = new(big.Int).SetString(value, 10)
		}
		if !ok {
			panic(j.vm.NewTypeError("invalid bigint"))
		}
		return n.String()
	}))
	_, err := j.vm.RunString(`
        function BigInt(value) { const text=__integer(String(value)); return {__bigint:text,toString(){return text;}}; }
        function __ordinaryBinary(op,a,b) { switch(op) {
            case '+':return a+b; case '-':return a-b; case '*':return a*b; case '/':return a/b; case '%':return a%b;
            case '>':return a>b; case '>=':return a>=b; case '<':return a<b; case '<=':return a<=b;
            case '===':return a===b; case '!==':return a!==b; case '==':return a==b; case '!=':return a!=b;
        } throw new Error('unsupported operator '+op); }
        class AbortController {
            constructor(){this.signal={aborted:false,listeners:[],addEventListener(event,fn){this.listeners.push(fn);},removeEventListener(event,fn){this.listeners=this.listeners.filter(f=>f!==fn);}};}
            abort(){if(this.signal.aborted)return;this.signal.aborted=true;for(const fn of this.signal.listeners.slice())fn();}
        }
        function sleep(ms){return new Promise(resolve=>setTimeout(resolve,ms));}
        function expect(actual,label='assertion') {
            let negate=false,deep=false;
            const chain={};
            for(const key of ['to','be','have'])Object.defineProperty(chain,key,{get(){return chain;}});
            Object.defineProperty(chain,'not',{get(){negate=!negate;return chain;}});
            Object.defineProperty(chain,'deep',{get(){deep=true;return chain;}});
            function check(ok){if(ok===negate)throw new Error(label+': assertion failed');return chain;}
            chain.equal=expected=>check(deep?JSON.stringify(actual)===JSON.stringify(expected):__binary('===',actual,expected));
            chain.within=(lo,hi)=>check(actual>=lo&&actual<=hi);
            chain.closeTo=(expected,delta)=>check(Math.abs(actual-expected)<=delta);
            chain.property=key=>check(actual!=null&&Object.prototype.hasOwnProperty.call(actual,key));
            chain.match=pattern=>check(typeof actual==='string'&&pattern.test(actual));
            chain.an=chain.a=type=>check(type==='array'?Array.isArray(actual):typeof actual===type);
            chain.throw=message=>{let error;try{actual();}catch(e){error=e;}return check(!!error&&(message===undefined||String(error).includes(message)));};
            return chain;
        }
        const ethers={toQuantity:n=>'0x'+Number(n).toString(16),toNumber:Number,parseUnits:(v,u)=>{if(u!=='gwei')throw new Error('unit');return BigInt(v+'000000000');}};
        const Endpoints={pax:{evmRpc:'',cosmosRpc:''}};
    `)
	require.NoError(t, err)
	ordinary, ok := goja.AssertFunction(j.vm.Get("__ordinaryBinary"))
	require.True(t, ok)
	require.NoError(t, j.vm.Set("__binary", func(call goja.FunctionCall) goja.Value {
		op := call.Argument(0).String()
		a, b := call.Argument(1), call.Argument(2)
		x, xb := repair755Big(a)
		y, yb := repair755Big(b)
		if !xb && !yb {
			result, err := ordinary(goja.Undefined(), call.Argument(0), a, b)
			if err != nil {
				panic(err)
			}
			return result
		}
		if !xb || !yb {
			if op == "===" || op == "==" {
				return j.vm.ToValue(false)
			}
			if op == "!==" || op == "!=" {
				return j.vm.ToValue(true)
			}
			panic(j.vm.NewTypeError("mixed bigint operands"))
		}
		cmp := x.Cmp(y)
		switch op {
		case "===", "==":
			return j.vm.ToValue(cmp == 0)
		case "!==", "!=":
			return j.vm.ToValue(cmp != 0)
		case ">":
			return j.vm.ToValue(cmp > 0)
		case ">=":
			return j.vm.ToValue(cmp >= 0)
		case "<":
			return j.vm.ToValue(cmp < 0)
		case "<=":
			return j.vm.ToValue(cmp <= 0)
		case "+":
			x.Add(x, y)
		case "-":
			x.Sub(x, y)
		case "*":
			x.Mul(x, y)
		case "/":
			x.Quo(x, y)
		case "%":
			x.Rem(x, y)
		default:
			panic(j.vm.NewTypeError("unsupported bigint operator"))
		}
		ctor, _ := goja.AssertFunction(j.vm.Get("BigInt"))
		result, err := ctor(goja.Undefined(), j.vm.ToValue(x.String()))
		if err != nil {
			panic(err)
		}
		return result
	}))
	j.installFetch()
	return j
}

func (j *repair755JS) installFetch() {
	require.NoError(j.t, j.vm.Set("fetch", func(call goja.FunctionCall) goja.Value {
		url := call.Argument(0).String()
		options := call.Argument(1).ToObject(j.vm)
		ctx, cancel := context.WithCancel(context.Background())
		j.cancels = append(j.cancels, cancel)
		signal := options.Get("signal")
		if signal != nil && !goja.IsUndefined(signal) {
			object := signal.ToObject(j.vm)
			if object.Get("aborted").ToBoolean() {
				cancel()
			}
			add, _ := goja.AssertFunction(object.Get("addEventListener"))
			_, err := add(object, j.vm.ToValue("abort"), j.vm.ToValue(func() { cancel() }))
			require.NoError(j.t, err)
		}
		method := options.Get("method")
		methodName := "GET"
		if method != nil && !goja.IsUndefined(method) {
			methodName = method.String()
		}
		body := ""
		if v := options.Get("body"); v != nil && !goja.IsUndefined(v) {
			body = v.String()
		}
		p, resolve, reject := j.vm.NewPromise()
		go func() {
			req, err := http.NewRequestWithContext(ctx, methodName, url, strings.NewReader(body))
			var response *http.Response
			var data []byte
			if err == nil {
				req.Header.Set("Content-Type", "application/json")
				response, err = http.DefaultClient.Do(req)
			}
			if err == nil {
				data, err = io.ReadAll(response.Body)
				response.Body.Close()
			}
			j.enqueue(func() {
				if err != nil {
					reject(j.vm.NewGoError(err))
					return
				}
				result := j.vm.NewObject()
				require.NoError(j.t, result.Set("ok", response.StatusCode >= 200 && response.StatusCode < 300))
				require.NoError(j.t, result.Set("status", response.StatusCode))
				require.NoError(j.t, result.Set("json", func() goja.Value {
					value, e := j.vm.RunString("JSON.parse(" + strconv.Quote(string(data)) + ")")
					if e != nil {
						panic(e)
					}
					return value
				}))
				resolve(result)
			})
		}()
		return j.vm.ToValue(p)
	}))
}

func (j *repair755JS) constant(filename, name string) {
	j.t.Helper()
	statement := regexp.MustCompile(`export const ` + name + ` = [^;]+;`).FindString(repair755Source(j.t, filename))
	require.NotEmpty(j.t, statement)
	j.eval(strings.TrimPrefix(statement, "export "))
}

func (j *repair755JS) network(evm, cosmos string) {
	j.eval("Endpoints.pax.evmRpc=" + strconv.Quote(evm) + "; Endpoints.pax.cosmosRpc=" + strconv.Quote(cosmos) + ";")
	j.load("chainUtils.ts", "requestJson", "rawJsonRpc", "rpcResult")
	j.constant("constants.ts", "MERGED_EVM_CHAIN_ID")
	j.load("constants.ts", "assertMergedEvmChainId")
	j.load("chainUtils.ts", "queryPaxChainHeight", "validateEip1559Params")
}

func TestRepairM_755_RequestDeadline(t *testing.T) {
	for _, bodyStall := range []bool{false, true} {
		t.Run(fmt.Sprint(bodyStall), func(t *testing.T) {
			canceled := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if _, err := io.Copy(io.Discard, r.Body); err != nil {
					t.Errorf("read request body: %v", err)
					return
				}
				if bodyStall {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(200)
					w.(http.Flusher).Flush()
					_, _ = w.Write([]byte(`{"jsonrpc":"2.0",`))
					w.(http.Flusher).Flush()
				}
				<-r.Context().Done()
				close(canceled)
			}))
			t.Cleanup(server.Close)
			j := newRepair755JS(t)
			j.network(server.URL, server.URL)
			j.load("chainUtils.ts", "waitUntil")
			start := time.Now()
			_, err := j.await("rawJsonRpc(" + strconv.Quote(server.URL) + ",'eth_chainId',[],1,{timeoutMs:35})")
			require.Error(t, err)
			require.Less(t, time.Since(start), time.Second)
			select {
			case <-canceled:
			case <-time.After(time.Second):
				t.Fatal("deadline did not cancel the real HTTP request")
			}
		})
	}
	j := newRepair755JS(t)
	j.load("chainUtils.ts", "waitUntil")
	start := time.Now()
	_, err := j.await("waitUntil(()=>new Promise(()=>{}),{timeoutMs:35,label:'hung callback'})")
	require.ErrorContains(t, err, "timed out")
	require.Less(t, time.Since(start), time.Second)
	start = time.Now()
	_, err = j.await("waitUntil(async()=>{throw new Error('poll failure');},{timeoutMs:35,intervalMs:5})")
	require.ErrorContains(t, err, "timed out")
	require.ErrorContains(t, err, "poll failure", "lowering preserves the conditional timeout detail")
	require.Less(t, time.Since(start), time.Second)
	_, err = j.await("waitUntil((signal,deadline)=>Promise.resolve(signal.aborted?null:deadline>Date.now()),{timeoutMs:100})")
	require.NoError(t, err)
	value, err := j.await("waitUntil(async()=>42)")
	require.NoError(t, err)
	require.Equal(t, int64(42), value.ToInteger(), "typed default polling options survive erasure")
	require.Empty(t, j.timers)
}

func TestRepairM_755_PollingDeadline(t *testing.T) {
	canceled := make(chan struct{})
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			t.Errorf("read polling request body: %v", err)
			return
		}
		calls++
		if calls == 1 {
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":null}`))
			return
		}
		<-r.Context().Done()
		close(canceled)
	}))
	t.Cleanup(server.Close)
	j := newRepair755JS(t)
	j.network(server.URL, server.URL)
	j.load("chainUtils.ts", "waitUntil")
	start := time.Now()
	_, err := j.await("waitUntil(async(signal,deadline)=>{const response=await rawJsonRpc(" + strconv.Quote(server.URL) + ",'eth_blockNumber',[],1,{signal,deadline,timeoutMs:10000});return response.result;},{timeoutMs:80,intervalMs:5,label:'bounded attempts'})")
	require.ErrorContains(t, err, "timed out")
	require.Less(t, time.Since(start), time.Second)
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("poll did not abort its in-flight attempt")
	}
	require.Equal(t, 2, calls)
}

// The protobuf bridge uses the repository's generated codecs on the wire. It
// exposes the same encode/finish/decode interface used by cosmjs-types.
func (j *repair755JS) installCodec() {
	require.NoError(j.t, j.vm.Set("__buffer", func(call goja.FunctionCall) goja.Value {
		var data []byte
		if call.Argument(1).String() == "base64" {
			decoded, err := base64.StdEncoding.DecodeString(call.Argument(0).String())
			if err != nil {
				panic(j.vm.NewGoError(err))
			}
			data = decoded
		} else {
			var ok bool
			data, ok = call.Argument(0).Export().([]byte)
			if !ok {
				panic(j.vm.NewTypeError("byte buffer expected"))
			}
		}
		buffer := j.vm.NewObject()
		require.NoError(j.t, buffer.Set("__bytes", data))
		require.NoError(j.t, buffer.Set("toString", func(encoding string) string {
			if encoding != "hex" {
				panic(j.vm.NewTypeError("encoding"))
			}
			return hex.EncodeToString(data)
		}))
		return buffer
	}))
	require.NoError(j.t, j.vm.Set("__encodeParams", func(call goja.FunctionCall) goja.Value {
		input := call.Argument(0).ToObject(j.vm)
		request := &proposal.QueryParamsRequest{Subspace: input.Get("subspace").String(), Key: input.Get("key").String()}
		data, err := request.Marshal()
		if err != nil {
			panic(j.vm.NewGoError(err))
		}
		output := j.vm.NewObject()
		require.NoError(j.t, output.Set("finish", func() []byte { return data }))
		return output
	}))
	require.NoError(j.t, j.vm.Set("__decodeParams", func(call goja.FunctionCall) goja.Value {
		data, ok := call.Argument(0).ToObject(j.vm).Get("__bytes").Export().([]byte)
		if !ok {
			panic(j.vm.NewTypeError("byte buffer expected"))
		}
		response := new(proposal.QueryParamsResponse)
		if err := response.Unmarshal(data); err != nil {
			panic(j.vm.NewGoError(err))
		}
		data, err := json.Marshal(response)
		if err != nil {
			panic(j.vm.NewGoError(err))
		}
		value, err := j.vm.RunString("JSON.parse(" + strconv.Quote(string(data)) + ")")
		if err != nil {
			panic(err)
		}
		return value
	}))
	j.eval("const Buffer={from:__buffer}; const QueryParamsRequest={encode:__encodeParams}; const QueryParamsResponse={decode:__decodeParams};")
	j.load("chainUtils.ts", "queryEip1559Params")
}

type repair755Chain struct {
	app         *app.App
	ctx         sdk.Context
	server      *httptest.Server
	mu          sync.Mutex
	fault       string
	queries     int
	heightReads int
	base, next  *big.Int
	gas         uint64
}

func newRepair755Chain(t *testing.T) *repair755Chain {
	t.Helper()
	a := app.Setup(t, false, false, false)
	ctx := a.GetContextForDeliverTx(nil).WithBlockHeight(42).WithChainID("hyperpax_125-1").WithConsensusParams(&tmproto.ConsensusParams{Block: &tmproto.BlockParams{MaxGas: 1000000}})
	a.EvmKeeper.SetNextBaseFeePerGas(ctx, sdk.NewDec(2000000000))
	space, ok := a.ParamsKeeper.GetSubspace("baseapp")
	require.True(t, ok)
	space.Set(ctx, []byte("BlockParams"), *ctx.ConsensusParams().Block)
	c := &repair755Chain{app: a, ctx: ctx, base: big.NewInt(2000000000), gas: 125000}
	c.next = a.EvmKeeper.AdjustDynamicBaseFeePerGas(ctx, c.gas).TruncateInt().BigInt()
	c.server = httptest.NewServer(http.HandlerFunc(c.serve))
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	t.Cleanup(c.server.Close)
	return c
}

func (c *repair755Chain) setFault(value string) { c.mu.Lock(); defer c.mu.Unlock(); c.fault = value }

func (c *repair755Chain) serve(w http.ResponseWriter, r *http.Request) {
	var request struct {
		ID     any             `json:"id"`
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	var result any
	blockHash := strings.Repeat("ab", 32)
	switch request.Method {
	case "eth_blockNumber":
		c.heightReads++
		result = "0x2a"
		if c.heightReads > 2 {
			result = "0x2b"
		}
	case "eth_gasPrice":
		api := evmrpc.NewInfoAPI(nil, &c.app.EvmKeeper, func(int64) sdk.Context { return c.ctx }, nil, "", 1024, evmrpc.ConnectionTypeHTTP, nil, nil)
		price, err := evmrpc.GasPriceHelper(api, context.Background(), c.next, c.gas, big.NewInt(0))
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		result = "0x" + price.ToInt().Text(16)
	case "eth_chainId":
		result = "0x7d"
		if c.fault == "chain-id" {
			result = "0xae3f2"
		}
	case "block":
		result = map[string]any{"block_id": map[string]any{"hash": blockHash}, "block": map[string]any{"header": map[string]any{"height": "42", "chain_id": "hyperpax_125-1"}}}
	case "eth_getBlockByNumber":
		var args []json.RawMessage
		if err := json.Unmarshal(request.Params, &args); err != nil || len(args) < 1 {
			http.Error(w, "invalid block args", 400)
			return
		}
		var tag string
		if err := json.Unmarshal(args[0], &tag); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		height := int64(42)
		fee := c.base
		if tag == "0x2b" {
			height = 43
			fee = c.next
		}
		hash := "0x" + blockHash
		if c.fault == "block-hash" {
			hash = "0x" + strings.Repeat("cd", 32)
		}
		result = map[string]any{"number": fmt.Sprintf("0x%x", height), "hash": hash, "gasUsed": fmt.Sprintf("0x%x", c.gas), "gasLimit": "0xf4240", "baseFeePerGas": "0x" + fee.Text(16)}
	case "genesis":
		result = map[string]any{"genesis": map[string]any{"chain_id": "hyperpax_125-1", "genesis_time": "2025-01-01T00:00:00Z", "app_state": map[string]any{"bank": map[string]any{"balances": []any{}}, "evm": map[string]any{"params": map[string]any{}}}}}
	case "abci_query":
		c.queries++
		var params coretypes.RequestABCIQuery
		if err := json.Unmarshal(request.Params, &params); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if params.Path != "/cosmos.params.v1beta1.Query/Params" || int64(params.Height) != 42 || params.Prove {
			http.Error(w, "unpinned query", 400)
			return
		}
		data := []byte(params.Data)
		query := new(proposal.QueryParamsRequest)
		if err := query.Unmarshal(data); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		response, err := c.app.ParamsKeeper.Params(sdk.WrapSDKContext(c.ctx), query)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		switch c.fault {
		case "http":
			http.Error(w, "parameter service failed", 500)
			return
		case "decode":
			response.Param.Value = "broken JSON"
		case "numeric":
			response.Param.Value = `"1junk"`
		case "wrong-key":
			response.Param.Key = "other-key"
		}
		data, err = response.Marshal()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		if c.fault == "protobuf" {
			data = []byte{255}
		}
		code := 0
		if c.fault == "abci" {
			code = 7
		}
		height := "42"
		if c.fault == "height" {
			height = "41"
		}
		result = map[string]any{"response": map[string]any{"code": code, "height": height, "value": base64.StdEncoding.EncodeToString(data)}}
	default:
		http.Error(w, "unexpected method "+request.Method, 400)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result})
}

func TestRepairM_755_Parameters(t *testing.T) {
	chain := newRepair755Chain(t)
	j := newRepair755JS(t)
	j.network(chain.server.URL, chain.server.URL)
	j.installCodec()
	value, err := j.await("queryEip1559Params()")
	require.NoError(t, err)
	object := value.ToObject(j.vm)
	require.Equal(t, int64(1000000), object.Get("blockGasLimit").ToInteger())
	require.Equal(t, int64(chain.app.EvmKeeper.GetTargetGasUsedPerBlock(chain.ctx)), object.Get("targetGasUsedPerBlock").ToInteger())
	require.Equal(t, chain.app.EvmKeeper.GetMinimumFeePerGas(chain.ctx).TruncateInt().Int64(), object.Get("minFeePerGas").ToInteger())
	require.Equal(t, 6, chain.queries)
	for _, fault := range []string{"http", "abci", "decode", "numeric", "wrong-key", "protobuf", "height", "chain-id", "block-hash"} {
		t.Run(fault, func(t *testing.T) {
			chain.setFault(fault)
			_, err := j.await("queryEip1559Params()")
			require.Error(t, err, "mandatory fee parameter errors must never become null")
		})
	}
	chain.setFault("")
	for _, bad := range []string{
		"null",
		"{blockGasLimit:0,targetGasUsedPerBlock:1,minFeePerGas:1,maxFeePerGas:2,maxUpwardAdjustment:0.1,maxDownwardAdjustment:0.1}",
		"{blockGasLimit:10,targetGasUsedPerBlock:10,minFeePerGas:1,maxFeePerGas:2,maxUpwardAdjustment:0.1,maxDownwardAdjustment:0.1}",
		"{blockGasLimit:10,targetGasUsedPerBlock:1,minFeePerGas:3,maxFeePerGas:2,maxUpwardAdjustment:0.1,maxDownwardAdjustment:0.1}",
		"{blockGasLimit:10,targetGasUsedPerBlock:1,minFeePerGas:1,maxFeePerGas:2,maxUpwardAdjustment:NaN,maxDownwardAdjustment:0.1}",
	} {
		_, err := j.await("validateEip1559Params(" + bad + ")")
		require.Error(t, err)
	}
}

func TestRepairM_755_NetworkIdentity(t *testing.T) {
	j := newRepair755JS(t)
	source := repair755Source(t, "constants.ts")
	for _, name := range []string{"MERGED_EVM_CHAIN_ID", "DEFAULT_EVM_CHAIN_ID", "HISTORICAL_DEVNET_EVM_CHAIN_ID"} {
		line := regexp.MustCompile(`export const ` + name + ` = [^;]+;`).FindString(source)
		require.NotEmpty(t, line)
		j.eval(strings.TrimPrefix(line, "export "))
	}
	j.load("constants.ts", "assertMergedEvmChainId")
	require.Equal(t, int64(125), j.eval("DEFAULT_EVM_CHAIN_ID").ToInteger())
	require.Equal(t, int64(713714), j.eval("HISTORICAL_DEVNET_EVM_CHAIN_ID").ToInteger())
	for _, value := range []string{"125", "'0x7d'", "'125'"} {
		_, err := j.await("assertMergedEvmChainId(" + value + ")")
		require.NoError(t, err)
	}
	for _, value := range []string{"713714", "'0xae3f2'", "null", "''", "125.5", "'125junk'", "{}"} {
		_, err := j.await("assertMergedEvmChainId(" + value + ")")
		require.Error(t, err)
	}
}

func TestRepairM_755_DockerChainBinding(t *testing.T) {
	chain := newRepair755Chain(t)
	j := newRepair755JS(t)
	j.network(chain.server.URL, chain.server.URL)
	j.load("cosmosUtils.ts", "canonicalGenesis", "assertSameChainGenesis", "assertDockerChainBinding", "isPaxDocker", "fundAdminOnPax")
	j.eval(`const genesis={chain_id:'hyperpax_125-1',genesis_time:'2025-01-01T00:00:00Z',app_state:{bank:{balances:[]},evm:{params:{}}}};`)
	var genesis map[string]any
	require.NoError(t, json.Unmarshal([]byte(j.eval("JSON.stringify(genesis)").String()), &genesis))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"genesis": genesis}})
	}))
	t.Cleanup(server.Close)
	// Docker inspect is a committed transport transcript, not an available Docker daemon.
	_, port, _ := strings.Cut(strings.TrimPrefix(server.URL, "http://"), ":")
	require.NoError(t, j.vm.Set("inspectTranscript", `{"26657/tcp":[{"HostIp":"127.0.0.1","HostPort":"`+port+`"}]}`))
	j.eval(`const DOCKER_NODE='pax-node-0'; const exec=async command=>{
        if(command==="docker ps --filter 'name=pax-node-0' --format '{{.Names}}'")return {stdout:'pax-node-0\n'};
        if(command!=="docker inspect --format '{{json .NetworkSettings.Ports}}' pax-node-0")throw new Error('unexpected command');
        return {stdout:inspectTranscript};
    }; const provider={getBalance:async()=>0n,send:(method,params)=>rpcResult(Endpoints.pax.evmRpc,method,params)};`)
	_, err := j.await("assertDockerChainBinding(provider)")
	require.NoError(t, err)
	genesis["genesis_time"] = "2025-01-02T00:00:00Z"
	_, err = j.await("assertDockerChainBinding(provider)")
	require.ErrorContains(t, err, "different genesis identities")
	_, err = j.await("fundAdminOnPax('0x0000000000000000000000000000000000000001','fixture',provider,'1000000')")
	require.ErrorContains(t, err, "different genesis identities")
	genesis["genesis_time"] = "2025-01-01T00:00:00Z"
	for _, change := range []string{
		"({...genesis,chain_id:'other'})",
		"({...genesis,genesis_time:'2025-01-02T00:00:00Z'})",
		"({...genesis,app_state:{bank:{balances:[{address:'pax1fixture',coins:[]}]}}})",
		"null",
	} {
		_, err := j.await("assertSameChainGenesis(genesis," + change + ")")
		require.Error(t, err)
	}
	_, err = j.await("assertSameChainGenesis(genesis,{app_state:genesis.app_state,genesis_time:genesis.genesis_time,chain_id:genesis.chain_id})")
	require.NoError(t, err)
	chain.setFault("block-hash")
	_, err = j.await("assertDockerChainBinding(provider)")
	require.ErrorContains(t, err, "committed block")
}

func TestRepairM_755_FeeHistory(t *testing.T) {
	chain := newRepair755Chain(t)
	j := newRepair755JS(t)
	j.network(chain.server.URL, chain.server.URL)
	j.installCodec()
	j.load("chainUtils.ts", "nextBaseFeePax", "blockGasInfo")
	j.load("feeHistoryUtils.ts", "verifyFeeHistorySeries")
	j.eval(`const provider={send:(method,params)=>rpcResult(Endpoints.pax.evmRpc,method,params),getBlockNumber:async()=>43};`)
	j.eval(fmt.Sprintf("const history={oldest:42,baseFeePerGas:[BigInt('%s'),BigInt('%s')],gasUsedRatio:[0.125]};", chain.base, chain.next))
	params, err := j.await("queryEip1559Params()")
	require.NoError(t, err)
	require.NoError(t, j.vm.Set("params", params))
	_, err = j.await("verifyFeeHistorySeries(provider,history,42,[],'pax',params)")
	require.NoError(t, err)
	_, err = j.await("verifyFeeHistorySeries(provider,history,42,[],'pax',null)")
	require.ErrorContains(t, err, "require validated")
	j.eval("history.baseFeePerGas[1]=history.baseFeePerGas[1]+1000n;")
	_, err = j.await("verifyFeeHistorySeries(provider,history,42,[],'pax',params)")
	require.ErrorContains(t, err, "base-fee transition")
}

func TestRepairM_755_GasPrice(t *testing.T) {
	chain := newRepair755Chain(t)
	api := evmrpc.NewInfoAPI(nil, &chain.app.EvmKeeper, func(int64) sdk.Context { return chain.ctx }, nil, "", 1024, evmrpc.ConnectionTypeHTTP, nil, nil)
	j := newRepair755JS(t)
	j.network(chain.server.URL, chain.server.URL)
	j.constant("gasPriceUtils.ts", "DEFAULT_PRIORITY_FEE_WEI")
	j.load("chainUtils.ts", "waitUntil", "blockGasInfo")
	j.load("gasPriceUtils.ts", "gasPrice", "gasPriceAtStableBlock", "assertPaxGasPriceTracksBlocks", "assertPaxGasPriceTracks")
	j.eval("const provider={send:(method,params)=>rpcResult(Endpoints.pax.evmRpc,method,params),getBlockNumber:()=>{throw new Error('cached height reader used');}};")
	sampled, err := j.await("gasPriceAtStableBlock(provider)")
	require.NoError(t, err)
	require.Equal(t, int64(42), sampled.ToObject(j.vm).Get("block").ToInteger())
	require.NoError(t, j.vm.Set("sampled", sampled))
	_, err = j.await("assertPaxGasPriceTracks(provider,sampled.gasPrice,sampled.block)")
	require.NoError(t, err)
	for _, gas := range []uint64{0, 790000, 800000, 800001} {
		price, err := evmrpc.GasPriceHelper(api, context.Background(), chain.next, gas, big.NewInt(0))
		require.NoError(t, err)
		j.eval(fmt.Sprintf("var blocks=[{number:41,baseFee:1n,gasUsed:1000000n,gasLimit:1000000n},{number:42,baseFee:BigInt('%s'),gasUsed:BigInt('%d'),gasLimit:1000000n},{number:43,baseFee:BigInt('%s'),gasUsed:1000000n,gasLimit:1000000n},{number:44,baseFee:9999999999n,gasUsed:1000000n,gasLimit:1000000n}];", chain.base, gas, chain.next))
		_, err = j.await(fmt.Sprintf("assertPaxGasPriceTracksBlocks(BigInt('%s'),42,blocks)", price.ToInt()))
		require.NoError(t, err)
		if gas <= 800000 {
			for _, wrong := range []*big.Int{new(big.Int).Sub(price.ToInt(), big.NewInt(1)), new(big.Int).Add(price.ToInt(), big.NewInt(1)), chain.next, chain.base} {
				_, err = j.await(fmt.Sprintf("assertPaxGasPriceTracksBlocks(BigInt('%s'),42,blocks)", wrong))
				require.Error(t, err)
			}
		} else {
			_, err = j.await(fmt.Sprintf("assertPaxGasPriceTracksBlocks(BigInt('%s'),42,blocks)", new(big.Int).Sub(chain.next, big.NewInt(1))))
			require.Error(t, err)
		}
	}
}

func TestRepairM_755_NeighborUnitContract(t *testing.T) {
	j := newRepair755JS(t)
	j.constant("gasPriceUtils.ts", "DEFAULT_PRIORITY_FEE_WEI")
	j.load("gasPriceUtils.ts", "assertPaxGasPriceTracksBlocks")
	for _, name := range []string{
		"busyUncongestedBlockRequiresBufferedBaseFeeAndDefaultPriorityFee",
		"emptyBlockRequiresIdleFloorIncludingDefaultPriorityFee",
		"congestionBoundaryPreservesExactFormulaAndMinimumBaseFee",
		"sampledNextFeeRejectsEveryWrongNeighbourPrice",
		"integerArithmeticPreservesLargeFeesAndTruncation",
	} {
		t.Run(name, func(t *testing.T) {
			j.load("gasPriceUtils.unit.spec.ts", name)
			_, err := j.await(name + "()")
			require.NoError(t, err)
		})
	}
}

func TestRepairM_755_Log0(t *testing.T) {
	source := repair755Source(t, "logsUtils.ts")
	match := regexp.MustCompile(`ZERO_TOPIC_INIT_CODE = '(0x[0-9a-f]+)'`).FindStringSubmatch(source)
	require.Len(t, match, 2)
	initCode, err := hex.DecodeString(strings.TrimPrefix(match[1], "0x"))
	require.NoError(t, err)
	config := &runtime.Config{GasLimit: 1000000}
	code, address, _, err := runtime.Create(initCode, config)
	require.NoError(t, err)
	require.NotEmpty(t, code)
	_, _, err = runtime.Call(address, nil, config)
	require.NoError(t, err)
	state, ok := config.State.(interface{ Logs() []*ethtypes.Log })
	require.True(t, ok)
	logs := state.Logs()
	require.Len(t, logs, 1)
	require.Empty(t, logs[0].Topics)
	require.Equal(t, common.LeftPadBytes([]byte{42}, 32), logs[0].Data)
	logs[0].BlockNumber = 42
	logs[0].BlockHash = common.HexToHash("0x42")
	logs[0].TxHash = common.HexToHash("0x755")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": logs})
	}))
	t.Cleanup(server.Close)
	j := newRepair755JS(t)
	j.network(server.URL, server.URL)
	for _, name := range []string{"ADDRESS_LOWER", "HASH32", "HEX_DATA", "HEX_QUANTITY"} {
		line := regexp.MustCompile(`export const ` + name + ` = [^;]+;`).FindString(repair755Source(t, "format.ts"))
		require.NotEmpty(t, line)
		j.eval(strings.TrimPrefix(line, "export "))
	}
	keys := regexp.MustCompile(`(?s)export const CORE_LOG_KEYS = (\[.*?\]) as const;`).FindStringSubmatch(source)
	require.Len(t, keys, 2)
	j.eval("const CORE_LOG_KEYS=" + keys[1] + ";")
	j.load("logsUtils.ts", "expectLogShape", "assertLogMatchesReceipt")
	result, err := j.await("rpcResult(" + strconv.Quote(server.URL) + ",'eth_getLogs',[{}])")
	require.NoError(t, err)
	require.NoError(t, j.vm.Set("logs", result))
	_, err = j.await("expectLogShape(logs[0]); assertLogMatchesReceipt(logs[0],{transactionHash:logs[0].transactionHash,blockHash:logs[0].blockHash,blockNumber:logs[0].blockNumber,transactionIndex:logs[0].transactionIndex,logs:logs});")
	require.NoError(t, err)
	for count := 1; count <= 4; count++ {
		_, err = j.await(fmt.Sprintf("expectLogShape({...logs[0],topics:Array(%d).fill('0x'+'ab'.repeat(32))})", count))
		require.NoError(t, err)
	}
	for _, topics := range []string{"Array(5).fill('0x'+'ab'.repeat(32))", "['0x01']", "['0x'+'zz'.repeat(32)]", "null"} {
		_, err := j.await("expectLogShape({...logs[0],topics:" + topics + "})")
		require.Error(t, err)
	}
}

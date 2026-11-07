// Package runner is a Go-native driver for the YAML integration test suite,
//
// It integrates with standard `go test`, so each test case becomes a subtest
// with proper failure attribution, -run filtering, and -v output.
//
// Requires a running Docker cluster. Start one with `make docker-cluster-start`.
//
// Run a single module:
//
//	go test -tags yaml_integration -v ./tests/integration/runner/... -run TestBankModule
//
// Run everything:
//
//	go test -tags yaml_integration -v ./tests/integration/runner/...
package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// TestCase mirrors the YAML schema used by the existing test files.
type TestCase struct {
	Name      string     `yaml:"name"`
	Inputs    []Input    `yaml:"inputs"`
	Verifiers []Verifier `yaml:"verifiers"`
}

// Input is one command step within a test case.
type Input struct {
	Cmd              string `yaml:"cmd"`
	Env              string `yaml:"env,omitempty"`                // capture trimmed stdout into this name
	Node             string `yaml:"node,omitempty"`               // docker container; defaults to Options.DefaultContainer
	ExpectedExitCode int    `yaml:"expected_exit_code,omitempty"` // defaults to successful exit (0)
}

// Verifier checks the accumulated env map after all inputs have run.
type Verifier struct {
	Type   string `yaml:"type"` // "eval" or "regex"
	Expr   string `yaml:"expr"`
	Result string `yaml:"result,omitempty"` // env var to match (regex only)
}

// Options controls how RunFile executes commands.
type Options struct {
	// Context cancels command execution; otherwise the test context is used.
	Context context.Context
	// CommandTimeout bounds each command, including output collection. Defaults to two minutes.
	CommandTimeout time.Duration
	// DefaultContainer is the docker container used when an Input has no Node set.
	DefaultContainer string
	// ExtraPath is appended to PATH inside docker containers.
	ExtraPath string
	// Shell is the shell used to execute commands (e.g. "sh", "bash").
	// Resolved via PATH at runtime. Defaults to "sh".
	Shell string
}

// Option is a functional option for Options.
type Option func(*Options)

// WithContainer sets the default docker container for inputs that don't specify one.
func WithContainer(container string) Option {
	return func(o *Options) { o.DefaultContainer = container }
}

// WithExtraPath overrides the PATH suffix injected into docker containers.
func WithExtraPath(path string) Option {
	return func(o *Options) { o.ExtraPath = path }
}

// WithShell overrides the shell used to execute commands. Resolved via PATH at runtime.
func WithShell(shell string) Option {
	return func(o *Options) { o.Shell = shell }
}

// WithContext adds caller cancellation to command execution.
func WithContext(ctx context.Context) Option {
	return func(o *Options) { o.Context = ctx }
}

// WithCommandTimeout sets the positive per-command execution limit.
func WithCommandTimeout(timeout time.Duration) Option {
	return func(o *Options) { o.CommandTimeout = timeout }
}

func newOptions(opts []Option) Options {
	var o Options
	//applying default options
	for _, f := range []Option{WithContainer("pax-node-0"), WithExtraPath("/root/go/bin:/root/.foundry/bin"), WithShell("bash"), WithCommandTimeout(2 * time.Minute)} {
		f(&o)
	}
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// RunFile reads path as a YAML list of TestCases and runs each as a subtest of t.
func RunFile(t *testing.T, path string, opts ...Option) {
	t.Helper()
	o := newOptions(opts)
	data, err := os.ReadFile(path) //nolint:gosec
	require.NoError(t, err, "read %s: %v", path, err)
	var cases []TestCase
	require.NoError(t, yaml.Unmarshal(data, &cases), "unmarshal %s: %v", path, err)
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			runCase(t, tc, o)
		})
	}
}

func runCase(t *testing.T, tc TestCase, opts Options) {
	t.Helper()
	envMap := make(map[string]string)

	for i, inp := range tc.Inputs {
		container := inp.Node
		if container == "" {
			container = opts.DefaultContainer
		}
		out, err := execCmd(t, inp, container, envMap, opts)
		t.Logf("[%d] $ %s\n    => %s", i, inp.Cmd, out)
		require.NoError(t, err, "input[%d] failed: %v", i, err)
		if inp.Env != "" {
			envMap[inp.Env] = out
		}
	}

	for _, v := range tc.Verifiers {
		if err := verify(v, envMap); err != nil {
			t.Errorf("verifier %s %q: %v", v.Type, v.Expr, err)
		}
	}
}

// execCmd preserves stdout, enforces the declared exit status and reports stderr.
// Cancellation and launch failures can never satisfy an expected negative status.
func execCmd(t *testing.T, inp Input, container string, envMap map[string]string, opts Options) (string, error) {
	t.Helper()
	if inp.ExpectedExitCode < 0 || inp.ExpectedExitCode > 255 {
		return "", fmt.Errorf("expected_exit_code must be between 0 and 255: %d", inp.ExpectedExitCode)
	}
	if opts.CommandTimeout <= 0 {
		return "", fmt.Errorf("command timeout must be positive")
	}
	ctx, cancel := context.WithTimeout(t.Context(), opts.CommandTimeout)
	defer cancel()
	if opts.Context != nil {
		stop := context.AfterFunc(opts.Context, cancel)
		defer stop()
		if opts.Context.Err() != nil {
			cancel()
		}
	}
	var c *exec.Cmd
	if container != "" {
		args := make([]string, 1, 1+2*len(envMap)+4)
		args[0] = "exec"
		for k, v := range envMap {
			args = append(args, "-e", k+"="+v)
		}
		args = append(args, container, opts.Shell, "-c", "export PATH=$PATH:"+opts.ExtraPath+" && "+inp.Cmd)
		c = exec.CommandContext(ctx, "docker", args...) //nolint:gosec
	} else {
		c = exec.CommandContext(ctx, opts.Shell, "-c", inp.Cmd) //nolint:gosec
		c.Env = append(os.Environ(), envMapSlice(envMap)...)
	}
	// Descendants may inherit output pipes even after the command is killed.
	c.WaitDelay = time.Second
	var stderr bytes.Buffer
	c.Stderr = &stderr
	out, err := c.Output()
	stdout := strings.TrimSpace(string(out))
	if ctx.Err() != nil {
		return stdout, fmt.Errorf("command canceled; stderr: %s: %w", strings.TrimSpace(stderr.String()), errors.Join(ctx.Err(), err))
	}
	status := 0
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() < 0 {
			return stdout, fmt.Errorf("command failed; stderr: %s: %w", strings.TrimSpace(stderr.String()), err)
		}
		status = exit.ExitCode()
	}
	if status != inp.ExpectedExitCode {
		if err == nil {
			err = fmt.Errorf("unexpected successful exit")
		}
		return stdout, fmt.Errorf("exit status %d (expected %d); stderr: %s: %w", status, inp.ExpectedExitCode, strings.TrimSpace(stderr.String()), err)
	}
	return stdout, nil
}

// envMapSlice converts envMap to a slice of "K=V" strings suitable for exec.Cmd.Env.
func envMapSlice(envMap map[string]string) []string {
	s := make([]string, 0, len(envMap))
	for k, v := range envMap {
		s = append(s, k+"="+v)
	}
	return s
}

// verify evaluates a single Verifier against the accumulated env map.
func verify(v Verifier, envMap map[string]string) error {
	switch v.Type {
	case "eval":
		ok, err := evalExpr(v.Expr, envMap)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("false: %s%s", v.Expr, fmtRelevant(v.Expr, envMap))
		}
		return nil
	case "regex":
		val, exists := envMap[v.Result]
		if !exists {
			return fmt.Errorf("missing regex result %q", v.Result)
		}
		// Python re.match starts at offset zero, even with multiline anchors.
		pattern, err := regexp.Compile("\\A(?:" + v.Expr + ")")
		if err != nil {
			return fmt.Errorf("bad pattern %q: %w", v.Expr, err)
		}
		if !pattern.MatchString(strings.TrimSpace(val)) {
			return fmt.Errorf("pattern %q did not match %q", v.Expr, val)
		}
		return nil
	default:
		return fmt.Errorf("unknown verifier type %q", v.Type)
	}
}

// Expressions use whitespace-delimited tokens, with quoted strings kept intact.
// Precedence, from low to high: or, and, comparisons, +/-, */ (left associative).
// OR retains the harness contract: report the first error unless a branch is true.
type exprToken struct {
	text   string
	quoted bool
}

func exprTokens(expr string) ([]exprToken, error) {
	var tokens []exprToken
	for i := 0; i < len(expr); {
		if unicode.IsSpace(rune(expr[i])) {
			i++
			continue
		}
		start := i
		if expr[i] == '\'' || expr[i] == '"' {
			quote := expr[i]
			i++
			var value strings.Builder
			for i < len(expr) && expr[i] != quote {
				if expr[i] == '\\' {
					i++
					if i == len(expr) {
						return nil, fmt.Errorf("unterminated quoted string")
					}
				}
				value.WriteByte(expr[i])
				i++
			}
			if i == len(expr) {
				return nil, fmt.Errorf("unterminated quoted string")
			}
			i++
			if i < len(expr) && !unicode.IsSpace(rune(expr[i])) {
				return nil, fmt.Errorf("expected whitespace after quoted string")
			}
			tokens = append(tokens, exprToken{text: value.String(), quoted: true})
		} else {
			for i < len(expr) && !unicode.IsSpace(rune(expr[i])) {
				i++
			}
			tokens = append(tokens, exprToken{text: expr[start:i]})
		}
	}
	return tokens, nil
}

func tokenIs(t exprToken, op string) bool { return !t.quoted && t.text == op }

func comparisonOp(t exprToken) bool {
	if t.quoted {
		return false
	}
	switch t.text {
	case "==", "!=", ">", "<", ">=", "<=":
		return true
	}
	return false
}

// Validate operator structure before short circuiting, so dangling operators
// cannot disappear behind a true OR or false AND branch.
func validateExprTokens(tokens []exprToken) error {
	if len(tokens) == 0 {
		return fmt.Errorf("empty expression")
	}
	start := 0
	for i := 0; i <= len(tokens); i++ {
		if i != len(tokens) && !tokenIs(tokens[i], "or") && !tokenIs(tokens[i], "and") {
			continue
		}
		branch := tokens[start:i]
		if len(branch) == 0 {
			return fmt.Errorf("missing operand for boolean operator")
		}
		cmp := -1
		for j, token := range branch {
			if comparisonOp(token) {
				if cmp >= 0 {
					return fmt.Errorf("multiple comparison operators")
				}
				cmp = j
			}
		}
		if cmp >= 0 {
			for _, side := range [][]exprToken{branch[:cmp], branch[cmp+1:]} {
				if len(side) == 0 || len(side)%2 == 0 {
					return fmt.Errorf("missing arithmetic operand")
				}
				for j, token := range side {
					op := !token.quoted && strings.Contains("+-*/", token.text) && len(token.text) == 1
					if j%2 == 0 && op {
						return fmt.Errorf("missing arithmetic operand")
					}
					if j%2 == 1 && !op {
						return fmt.Errorf("invalid arithmetic operator %q", token.text)
					}
				}
			}
		}
		start = i + 1
	}
	return nil
}

func evalExpr(expr string, envMap map[string]string) (bool, error) {
	tokens, err := exprTokens(expr)
	if err != nil {
		return false, err
	}
	if err := validateExprTokens(tokens); err != nil {
		return false, err
	}
	return evalBoolTokens(tokens, envMap)
}

func evalBoolTokens(tokens []exprToken, envMap map[string]string) (bool, error) {
	// Split the lowest-precedence operator first.
	for _, op := range []string{"or", "and"} {
		start := 0
		var firstErr error
		found := false
		for i := 0; i <= len(tokens); i++ {
			if i != len(tokens) && !tokenIs(tokens[i], op) {
				continue
			}
			if i == len(tokens) && !found {
				break
			}
			found = true
			ok, err := evalBoolTokens(tokens[start:i], envMap)
			if op == "and" {
				if err != nil || !ok {
					return false, err
				}
			} else {
				if err == nil && ok {
					return true, nil
				}
				if firstErr == nil {
					firstErr = err
				}
			}
			start = i + 1
		}
		if found {
			return op == "and", firstErr
		}
	}
	return cmpTokens(tokens, envMap)
}

func cmpExpr(expr string, envMap map[string]string) (bool, error) {
	tokens, err := exprTokens(expr)
	if err != nil {
		return false, err
	}
	if err := validateExprTokens(tokens); err != nil {
		return false, err
	}
	return cmpTokens(tokens, envMap)
}

func cmpTokens(tokens []exprToken, envMap map[string]string) (bool, error) {
	for i, token := range tokens {
		if !comparisonOp(token) {
			continue
		}
		resolve := func(side []exprToken) []string {
			values := make([]string, len(side))
			for j, t := range side {
				values[j] = t.text
				if j%2 == 0 && !t.quoted {
					if v, ok := envMap[t.text]; ok {
						values[j] = v
					}
				}
			}
			return values
		}
		lhs, err := evalArith(resolve(tokens[:i]))
		if err != nil {
			return false, err
		}
		rhs, err := evalArith(resolve(tokens[i+1:]))
		if err != nil {
			return false, err
		}
		return cmpValues(lhs, rhs, token.text)
	}
	var words []string
	for _, token := range tokens {
		words = append(words, token.text)
	}
	return false, fmt.Errorf("no operator in %q", strings.Join(words, " "))
}

// arithmeticValue keeps integer operations exact; division and decimal operands
// use floating point, as in the Python runner.
type arithmeticValue struct {
	integer  *big.Int
	floating float64
}

func parseArithmeticValue(text string) (arithmeticValue, error) {
	if n, ok := new(big.Int).SetString(text, 10); ok {
		return arithmeticValue{integer: n}, nil
	}
	f, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return arithmeticValue{}, fmt.Errorf("non-numeric arithmetic operand %q", text)
	}
	return arithmeticValue{floating: f}, nil
}

func (v arithmeticValue) asFloat() float64 {
	if v.integer == nil {
		return v.floating
	}
	f, _ := new(big.Float).SetInt(v.integer).Float64()
	return f
}

func applyArithmetic(a, b arithmeticValue, op string) (arithmeticValue, error) {
	switch op {
	case "+", "-", "*", "/":
	default:
		return arithmeticValue{}, fmt.Errorf("invalid arithmetic operator %q", op)
	}
	if a.integer != nil && b.integer != nil && op != "/" {
		result := new(big.Int)
		switch op {
		case "+":
			result.Add(a.integer, b.integer)
		case "-":
			result.Sub(a.integer, b.integer)
		case "*":
			result.Mul(a.integer, b.integer)
		}
		return arithmeticValue{integer: result}, nil
	}
	x, y := a.asFloat(), b.asFloat()
	switch op {
	case "+":
		x += y
	case "-":
		x -= y
	case "*":
		x *= y
	case "/":
		x /= y
	}
	return arithmeticValue{floating: x}, nil
}

// evalArith applies multiplication/division before addition/subtraction, keeping
// operators within each precedence level left associative.
func evalArith(tokens []string) (string, error) {
	if len(tokens) == 0 {
		return "", fmt.Errorf("empty side of comparison")
	}
	if len(tokens)%2 == 0 {
		return "", fmt.Errorf("missing arithmetic operand in %v", tokens)
	}
	if len(tokens) == 1 {
		return tokens[0], nil
	}
	term, err := parseArithmeticValue(tokens[0])
	if err != nil {
		return "", err
	}
	// Fold each multiplicative term first, then the additive terms in order.
	var terms []arithmeticValue
	var additive []string
	for i := 1; i < len(tokens); i += 2 {
		operand, err := parseArithmeticValue(tokens[i+1])
		if err != nil {
			return "", err
		}
		switch tokens[i] {
		case "*", "/":
			if tokens[i] == "/" && operand.asFloat() == 0 {
				return "", fmt.Errorf("division by zero in %v", tokens)
			}
			term, err = applyArithmetic(term, operand, tokens[i])
			if err != nil {
				return "", err
			}
		case "+", "-":
			terms = append(terms, term)
			additive = append(additive, tokens[i])
			term = operand
		default:
			return "", fmt.Errorf("invalid arithmetic operator %q", tokens[i])
		}
	}
	terms = append(terms, term)
	result := terms[0]
	for i, op := range additive {
		result, err = applyArithmetic(result, terms[i+1], op)
		if err != nil {
			return "", err
		}
	}
	if result.integer != nil {
		return result.integer.String(), nil
	}
	return strconv.FormatFloat(result.floating, 'g', -1, 64), nil
}

// cmpValues compares a and b with op. Tries big.Int (for large integers without
// float64 precision loss), then float64, then string.
func cmpValues(a, b, op string) (bool, error) {
	a, b = strings.TrimSpace(a), strings.TrimSpace(b)

	ai, bi := new(big.Int), new(big.Int)
	if _, ok1 := ai.SetString(a, 10); ok1 {
		if _, ok2 := bi.SetString(b, 10); ok2 {
			return applyOp(ai.Cmp(bi), op)
		}
	}

	fa, errA := strconv.ParseFloat(a, 64)
	fb, errB := strconv.ParseFloat(b, 64)
	if errA == nil && errB == nil {
		c := 0
		if fa < fb {
			c = -1
		} else if fa > fb {
			c = 1
		}
		return applyOp(c, op)
	}

	return applyOp(strings.Compare(a, b), op)
}

func applyOp(cmp int, op string) (bool, error) {
	switch op {
	case "==":
		return cmp == 0, nil
	case "!=":
		return cmp != 0, nil
	case ">":
		return cmp > 0, nil
	case "<":
		return cmp < 0, nil
	case ">=":
		return cmp >= 0, nil
	case "<=":
		return cmp <= 0, nil
	}
	return false, fmt.Errorf("unknown operator %q", op)
}

// fmtRelevant returns a diagnostic string listing env vars referenced in expr.
func fmtRelevant(expr string, envMap map[string]string) string {
	var refs []string
	for k, v := range envMap {
		if strings.Contains(expr, k) {
			refs = append(refs, k+"="+v)
		}
	}
	if len(refs) == 0 {
		return ""
	}
	return " [" + strings.Join(refs, ", ") + "]"
}

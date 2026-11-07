package layerxbridge_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"math/big"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxbridge"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestBridgeMethodConstantsCoverEveryABIMethod(t *testing.T) {
	constants := []string{
		layerxbridge.BridgeInMethod,
		layerxbridge.BridgeOutMethod,
		layerxbridge.GetChainMethod,
		layerxbridge.GetAttestorsMethod,
		layerxbridge.GetCapMethod,
		layerxbridge.IsPausedMethod,
		layerxbridge.IsNullifiedMethod,
	}
	source, err := parser.ParseFile(token.NewFileSet(), "layerxbridge.go", nil, 0)
	require.NoError(t, err)
	var declared []string
	seen := make(map[string]string)
	for _, declaration := range source.Decls {
		group, ok := declaration.(*ast.GenDecl)
		if !ok || group.Tok != token.CONST {
			continue
		}
		for _, spec := range group.Specs {
			value := spec.(*ast.ValueSpec)
			for i, name := range value.Names {
				if !strings.HasSuffix(name.Name, "Method") {
					continue
				}
				require.Len(t, value.Values, len(value.Names), name.Name)
				literal, ok := value.Values[i].(*ast.BasicLit)
				require.True(t, ok, "%s must name an ABI method with a string constant", name.Name)
				require.Equal(t, token.STRING, literal.Kind, name.Name)
				method, err := strconv.Unquote(literal.Value)
				require.NoError(t, err, name.Name)
				require.NotContains(t, seen, method, "duplicate method constant %s", name.Name)
				seen[method] = name.Name
				declared = append(declared, method)
			}
		}
	}
	require.ElementsMatch(t, declared, constants, "collect every declared *Method constant")

	data, err := os.ReadFile("abi.json")
	require.NoError(t, err)
	contractABI, err := abi.JSON(strings.NewReader(string(data)))
	require.NoError(t, err)
	var methods []string
	for name := range contractABI.Methods {
		methods = append(methods, name)
	}
	require.ElementsMatch(t, methods, constants, "method constants must exactly cover the ABI")

	h := newHarness(t)
	h.activate()
	h.precompile = layerxbridge.NewPrecompileWithKeeper(&h.keeper)
	var dispatched []string
	cases := []struct {
		name       string
		args       []interface{}
		writes     uint64
		signatures uint64
		want       []interface{}
	}{
		{
			name:   layerxbridge.BridgeInMethod,
			args:   h.bridgeInArgs(7, 100, h.attestors[0], h.attestors[1]),
			writes: 8, signatures: 2,
			want: []interface{}{h.denom},
		},
		{
			name:   layerxbridge.BridgeOutMethod,
			args:   []interface{}{chainID, asset, big.NewInt(40), remote},
			writes: 6,
			want:   []interface{}{uint64(1)},
		},
		{
			name: layerxbridge.GetChainMethod,
			args: []interface{}{chainID},
			want: []interface{}{true, vault, uint64(64), true},
		},
		{
			name: layerxbridge.GetAttestorsMethod,
			want: []interface{}{
				[]common.Address{common.Address(h.attestors[0].Signer), common.Address(h.attestors[1].Signer),
					common.Address(h.attestors[2].Signer)},
				[]*big.Int{big.NewInt(1_000_000), big.NewInt(1_000_000), big.NewInt(1_000_000)},
				uint32(2),
			},
		},
		{
			name: layerxbridge.GetCapMethod,
			args: []interface{}{chainID, asset},
			want: []interface{}{h.denom, big.NewInt(1500), big.NewInt(1000), big.NewInt(60)},
		},
		{
			name: layerxbridge.IsPausedMethod,
			want: []interface{}{false},
		},
		{
			name: layerxbridge.IsNullifiedMethod,
			args: []interface{}{chainID, [32]byte{0xde, 0xad, 7}, uint64(7)},
			want: []interface{}{true},
		},
	}
	for _, c := range cases {
		dispatched = append(dispatched, c.name)
	}
	require.ElementsMatch(t, methods, dispatched, "every ABI method needs a gas row and dispatch call")
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			method, ok := contractABI.Methods[c.name]
			require.True(t, ok)
			require.Equal(t, c.writes, layerxbridge.Writes(c.name))
			if c.writes == 0 {
				require.True(t, method.IsConstant(), "zero-write rows must be views")
			} else {
				require.False(t, method.IsConstant(), "bridge mutations must have write rows")
			}
			packed, err := method.Inputs.Pack(c.args...)
			require.NoError(t, err)
			input := append(append([]byte(nil), method.ID...), packed...)
			wantGas := uint64(3000) + 16*uint64(len(packed)) + 3000*c.signatures + 5000*c.writes
			require.Equal(t, wantGas, h.precompile.RequiredGas(input))

			result, err := h.precompile.Run(h.evm, h.caller, h.caller, input, nil, c.writes == 0, false, nil)
			if err != nil {
				reason, unpackErr := abi.UnpackRevert(result)
				require.NoError(t, unpackErr)
				require.NotContains(t, strings.ToLower(reason), "unknown method", c.name)
				require.NoError(t, err, "%s reverted: %s", c.name, reason)
			}
			out, err := method.Outputs.Unpack(result)
			require.NoError(t, err)
			require.Equal(t, c.want, out, "ABI method must reach its keeper-backed dispatch")
		})
	}
}

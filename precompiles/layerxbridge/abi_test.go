package layerxbridge_test

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func TestBridgeSolidityInterfaceMatchesTheABI(t *testing.T) {
	contractMethods := map[string]string{
		"bridgeIn":     "bridgeIn(uint64,address,bytes32,uint64,bytes32,address,uint256,bytes[])",
		"bridgeOut":    "bridgeOut(uint64,address,uint256,address)",
		"getAttestors": "getAttestors()",
		"getCap":       "getCap(uint64,address)",
		"getChain":     "getChain(uint64)",
		"isNullified":  "isNullified(uint64,bytes32,uint64)",
		"isPaused":     "isPaused()",
	}
	contractEvents := map[string]string{
		"BridgeIn":  "BridgeIn(uint64,bytes32,address,uint64,address,uint256,string)",
		"BridgeOut": "BridgeOut(uint64,address,uint256,address,uint64)",
	}

	encoded, err := os.ReadFile("abi.json")
	require.NoError(t, err)
	parsed, err := abi.JSON(strings.NewReader(string(encoded)))
	require.NoError(t, err)
	require.Len(t, parsed.Methods, len(contractMethods))
	require.Len(t, parsed.Events, len(contractEvents))

	source, err := os.ReadFile("LayerXBridge.sol")
	require.NoError(t, err)
	text := regexp.MustCompile(`(?s)/\*.*?\*/|//[^\n]*`).ReplaceAllString(string(source), "")
	interfaces := regexp.MustCompile(`(?s)\binterface\s+ILayerXBridge\s*\{([^{}]*)\}`).FindAllStringSubmatch(text, -1)
	require.Len(t, interfaces, 1, "expected exactly one ILayerXBridge interface")

	functionDeclaration := regexp.MustCompile(`(?s)^function\s+(\w+)\s*\(([^)]*)\)\s+external(?:\s+(view|pure|payable))?(?:\s+returns\s*\(([^)]*)\))?$`)
	eventDeclaration := regexp.MustCompile(`(?s)^event\s+(\w+)\s*\(([^)]*)\)(?:\s+(anonymous))?$`)
	seenMethods := make(map[string]bool)
	seenEvents := make(map[string]bool)
	declarations := strings.Split(interfaces[0][1], ";")
	require.Empty(t, strings.TrimSpace(declarations[len(declarations)-1]), "unterminated interface declaration")
	for _, declaration := range declarations[:len(declarations)-1] {
		declaration = strings.TrimSpace(declaration)
		require.NotEmpty(t, declaration, "empty interface declaration")
		switch {
		case functionDeclaration.MatchString(declaration):
			parts := functionDeclaration.FindStringSubmatch(declaration)
			name := parts[1]
			require.Contains(t, contractMethods, name, "unexpected Solidity method")
			require.False(t, seenMethods[name], "duplicate Solidity method %s", name)
			seenMethods[name] = true
			t.Run("function_"+name, func(t *testing.T) {
				mutability := parts[3]
				if mutability == "" {
					mutability = "nonpayable"
				}
				want := abi.NewMethod(name, name, abi.Function, mutability, false, false,
					bridgeSolidityArguments(t, parts[2]), bridgeSolidityArguments(t, parts[4]))
				require.Equal(t, contractMethods[name], want.Sig)
				got, ok := parsed.Methods[name]
				require.True(t, ok, "ABI is missing Solidity method %s", name)
				require.Equal(t, want.Sig, got.Sig)
				require.Equal(t, crypto.Keccak256([]byte(want.Sig))[:4], got.ID)
				require.Equal(t, want.StateMutability, got.StateMutability)
				require.Equal(t, want.Inputs, got.Inputs)
				require.Equal(t, want.Outputs, got.Outputs)
				resolved, err := parsed.MethodById(want.ID)
				require.NoError(t, err)
				require.Equal(t, name, resolved.Name)
			})
		case eventDeclaration.MatchString(declaration):
			parts := eventDeclaration.FindStringSubmatch(declaration)
			name := parts[1]
			require.Contains(t, contractEvents, name, "unexpected Solidity event")
			require.False(t, seenEvents[name], "duplicate Solidity event %s", name)
			seenEvents[name] = true
			t.Run("event_"+name, func(t *testing.T) {
				want := abi.NewEvent(name, name, parts[3] == "anonymous", bridgeSolidityArguments(t, parts[2]))
				require.Equal(t, contractEvents[name], want.Sig)
				got, ok := parsed.Events[name]
				require.True(t, ok, "ABI is missing Solidity event %s", name)
				require.Equal(t, want.Sig, got.Sig)
				require.Equal(t, crypto.Keccak256Hash([]byte(want.Sig)), got.ID)
				require.Equal(t, want.Anonymous, got.Anonymous)
				require.Equal(t, want.Inputs, got.Inputs)
				resolved, err := parsed.EventByID(want.ID)
				require.NoError(t, err)
				require.Equal(t, name, resolved.Name)
			})
		default:
			t.Fatalf("unrecognized ILayerXBridge declaration: %s", declaration)
		}
	}
	require.Len(t, seenMethods, len(contractMethods), "Solidity method set differs from the ABI")
	require.Len(t, seenEvents, len(contractEvents), "Solidity event set differs from the ABI")
}

func bridgeSolidityArguments(t *testing.T, declaration string) abi.Arguments {
	t.Helper()
	args := make(abi.Arguments, 0)
	if strings.TrimSpace(declaration) == "" {
		return args
	}
	for _, parameter := range strings.Split(declaration, ",") {
		parts := strings.Fields(parameter)
		require.NotEmpty(t, parts, "empty parameter in %s", declaration)
		typ, err := abi.NewType(parts[0], parts[0], nil)
		require.NoError(t, err, "invalid Solidity type in %s", parameter)
		argument := abi.Argument{Type: typ}
		for _, part := range parts[1:] {
			switch part {
			case "calldata", "memory", "storage":
			case "indexed":
				require.False(t, argument.Indexed, "duplicate indexed modifier in %s", parameter)
				argument.Indexed = true
			default:
				require.Empty(t, argument.Name, "unrecognized parameter syntax: %s", parameter)
				argument.Name = part
			}
		}
		args = append(args, argument)
	}
	return args
}

package layerxprograms_test

import (
	"encoding/hex"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func TestProgramsABIEventTopics(t *testing.T) {
	parsed := programsABI(t)
	events := []struct {
		name   string
		fields string
		topic  string
	}{
		{"ProgramDeployed", "bytes32 programId,bytes32 codeHash", "0x2765c1020c93cab47e389edaa5b5c68834bdd76f7a5d384adb460fc86e688709"},
		{"ProgramUpgraded", "bytes32 programId,bytes32 oldCodeHash,bytes32 newCodeHash", "0x36b27ad7ada15e27c12e726d56c3dbd2aaa084a9250e5282c7cd38eb78a1a8ab"},
		{"RegistryRead", "bytes32 programId", "0x74f03c32873ceda1e22e1bff01e678b798a6e7c3f6b64ea0547a965b6c33a900"},
		{"ProgramTransferred", "bytes32 programId,bytes32 transferSetRoot", "0x079c617c449e4a58644f3328aa041f2e65a1b6f281b13b8719ea21c3449aac6a"},
		{"ProgramGuestEvent", "bytes32 programId,bytes32 principal,bytes32 activityId,uint64 framePath,uint8 depth,uint32 eventIndex,uint16 topicLength,bytes32 topicDigest,uint32 dataLength,bytes32 dataDigest", "0x23a84d3a8c8e7d232703d9715c64dba47a94a94c87c46fe61532e5a609e761d1"},
		{"ProgramCallOutcome", "bytes32 programId,bytes32 principal,bytes32 activityId,uint64 framePath,uint8 depth,uint16 runtimeVersion,uint16 abiVersion,uint32 feeScheduleVersion,uint32 meteringVersion,int32 terminalResult,bytes32 transferSetRoot,bytes32 callGraphDigest,bytes32 terminalDetailDigest,bytes32 eventEnvelopeDigest", "0xcfe5297f03a2951065e463e9e0c2e3686bc0204f727eeec1a52357b5dd4a27a0"},
		{"ProgramAccountRegistered", "bytes32 programId,bytes32 accountId,bytes32 assetId,bytes32 seedDigest,uint16 seedLength,uint64 registeredSequence", "0x25814f9b8786afbe9705da19656abcc9aa1f0afc96a17e02b7a0f5a85de27b5a"},
		{"ExitRouteSet", "bytes32 programId,bytes32 accountId,bytes32 assetId,bytes32 destination", "0x6b9aceca11d63f8d54de57f0d38646082c9375dc119465a34e7099ec6f92ac57"},
		{"ProgramDeprecated", "bytes32 programId,uint8 priorStatus,uint8 targetStatus,bytes32 principal,bytes32 exitProgram,uint64 deadline,uint64 sequence,uint16 accountCount,uint16 liveCount,bytes32 accountRoot", "0x769e024ff1115e1c26a069148f332c81f2578f4ea67dde7d5076a9e9fd5e1eb8"},
		{"ProgramTombstoned", "bytes32 programId,uint8 priorStatus,uint8 targetStatus,bytes32 principal,bytes32 exitProgram,uint64 deadline,uint64 sequence,uint16 accountCount,uint16 liveCount,bytes32 accountRoot", "0x47fc53b89ce902506c8f3a0424a87f764a54dcb5ccb9c985639991a3a55b410c"},
		{"ProgramValueExited", "bytes32 programId,bytes32 accountId,bytes32 transferSetRoot", "0xf07e690dd4cedc62197bec125b1418cfee68758e9c183deac3b50902e52f59ca"},
		{"SandboxUsageReceipt", "bytes32 activityId,uint64 expectedSequence,uint16 chunkIndex,uint16 chunkCount,uint32 receiptLength,bytes chunk", "0x5c8386df0628270b0285d3f16695d6147ce4a66e048cbef0d572dae8ac4d2ba7"},
		{"SandboxDestroyed", "bytes32 leaseId,uint8 part,bytes terminalFrame", "0x88612a15e0cf0a2b8bfa1e89a7ad1a7284828c20680de0bb004210052506dc10"},
		{"StorageRetired", "bytes32 activityId,uint8 frameIndex,bytes frame", "0xb8fb07925225ab6c059c761581d51adeb4b7017ea3b0523efded11df61bed65b"},
	}
	require.Len(t, parsed.Events, 14)
	source, err := os.ReadFile("LayerXPrograms.sol")
	require.NoError(t, err)
	declarations := regexp.MustCompile(`(?s)\bevent\s+(\w+)\s*\(([^)]*)\)\s*;`).FindAllStringSubmatch(string(source), -1)
	require.Len(t, declarations, 14)
	for i, want := range events {
		t.Run(want.name, func(t *testing.T) {
			event, ok := parsed.Events[want.name]
			require.True(t, ok)
			require.False(t, event.Anonymous)
			fields := strings.Split(want.fields, ",")
			require.Len(t, event.Inputs, len(fields))
			types := make([]string, len(fields))
			solidityFields := make([]string, len(fields))
			for j, field := range fields {
				parts := strings.Fields(field)
				types[j] = parts[0]
				require.Equal(t, parts[0], event.Inputs[j].Type.String())
				require.Equal(t, parts[1], event.Inputs[j].Name)
				require.Equal(t, j == 0, event.Inputs[j].Indexed, field)
				solidityFields[j] = field
				if j == 0 {
					solidityFields[j] = parts[0] + " indexed " + parts[1]
				}
			}
			require.Equal(t, want.name+"("+strings.Join(types, ",")+")", event.Sig)
			require.Equal(t, want.topic, event.ID.Hex())
			require.Equal(t, crypto.Keccak256Hash([]byte(event.Sig)), event.ID)
			resolved, err := parsed.EventByID(event.ID)
			require.NoError(t, err)
			require.Equal(t, want.name, resolved.Name)
			require.Equal(t, want.name, declarations[i][1])
			require.Equal(t, strings.Join(strings.Fields(strings.Join(solidityFields, ", ")), " "), strings.Join(strings.Fields(declarations[i][2]), " "))
		})
	}
}

func TestProgramsABIKeepsThirtyFiveMethods(t *testing.T) {
	parsed := programsABI(t)
	require.Len(t, parsed.Methods, 35)
	require.Len(t, d2Table, 35)
	for name, want := range d2Table {
		method, ok := parsed.Methods[name]
		require.True(t, ok, name)
		require.Equal(t, want.selector, hex.EncodeToString(method.ID), name)
		mutability := "nonpayable"
		if want.class == classRead {
			mutability = "view"
		}
		require.Equal(t, mutability, method.StateMutability, name)
		resolved, err := parsed.MethodById(method.ID)
		require.NoError(t, err)
		require.Equal(t, name, resolved.Name)
	}
	deploy := parsed.Methods["deploy"]
	require.Len(t, deploy.Inputs, 7)
	require.Equal(t, "deploy(bytes32,uint16,uint8,bytes32,bytes32,bytes,bytes)", deploy.Sig)
	for _, input := range deploy.Inputs {
		require.NotEqual(t, "feeLimit", input.Name)
	}
	for _, name := range []string{"call", "simulateCall", "upgrade", "sandboxExecute"} {
		inputs := parsed.Methods[name].Inputs
		require.NotEmpty(t, inputs, name)
		last := inputs[len(inputs)-1]
		require.Equal(t, "feeLimit", last.Name, name)
		require.Equal(t, "uint128", last.Type.String(), name)
	}

	source, err := os.ReadFile("LayerXPrograms.sol")
	require.NoError(t, err)
	require.Contains(t, string(source), "interface ILayerXPrograms {")
	require.Contains(t, string(source), "address constant LAYERX_PROGRAMS_ADDRESS = 0x000000000000000000000000000000000000101D;")
	functions := regexp.MustCompile(`(?s)\bfunction\s+(\w+)\s*\(([^)]*)\)\s+external(?:\s+(view))?(?:\s+returns\s*\(([^)]*)\))?\s*;`).FindAllStringSubmatch(string(source), -1)
	require.Len(t, functions, 35)
	seen := make(map[string]bool)
	for _, declaration := range functions {
		name := declaration[1]
		require.False(t, seen[name], name)
		seen[name] = true
		method, ok := parsed.Methods[name]
		require.True(t, ok, name)
		require.Equal(t, method.Sig, name+"("+strings.Join(programsSolidityTypes(declaration[2]), ",")+")")
		require.Equal(t, method.StateMutability == "view", declaration[3] == "view", name)
		outputTypes := make([]string, 0, len(method.Outputs))
		for _, output := range method.Outputs {
			outputTypes = append(outputTypes, output.Type.String())
		}
		require.Equal(t, outputTypes, programsSolidityTypes(declaration[4]), name)
	}
	structs := regexp.MustCompile(`(?s)\bstruct\s+TransferLeg\s*\{([^}]*)\}`).FindAllStringSubmatch(string(source), -1)
	require.Len(t, structs, 1)
	require.Equal(t, "bytes32 from; bytes32 asset; bytes32 to; uint128 amount;", strings.Join(strings.Fields(structs[0][1]), " "))
}

func programsSolidityTypes(parameters string) []string {
	types := make([]string, 0)
	if strings.TrimSpace(parameters) == "" {
		return types
	}
	for _, parameter := range strings.Split(parameters, ",") {
		typ := strings.Fields(parameter)[0]
		if typ == "TransferLeg[]" {
			typ = "(bytes32,bytes32,bytes32,uint128)[]"
		}
		types = append(types, typ)
	}
	return types
}

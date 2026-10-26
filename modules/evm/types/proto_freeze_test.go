package types_test

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"text/scanner"

	_ "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	gogoproto "github.com/gogo/protobuf/proto"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

type evmProtoSurface map[string]map[string]int32

func TestEVMProtoSurfaceFrozen(t *testing.T) {
	want := map[string]evmProtoSurface{
		"evm/config.proto": {
			"paxprotocol.paxchain.evm.ChainConfig": {
				"cancun_time": 1, "prague_time": 2, "verkle_time": 3, "pax_sstore_set_gas_eip2200": 4,
			},
		},
		"evm/enums.proto": {},
		"evm/genesis.proto": {
			"paxprotocol.paxchain.evm.AddressAssociation": {"pax_address": 1, "eth_address": 2},
			"paxprotocol.paxchain.evm.Code":               {"address": 1, "code": 2},
			"paxprotocol.paxchain.evm.ContractState":      {"address": 1, "key": 2, "value": 3},
			"paxprotocol.paxchain.evm.Nonce":              {"address": 1, "nonce": 2},
			"paxprotocol.paxchain.evm.Serialized":         {"prefix": 1, "key": 2, "value": 3},
			"paxprotocol.paxchain.evm.GenesisState": {
				"params": 1, "address_associations": 2, "codes": 3, "states": 4, "nonces": 5, "serialized": 6,
			},
		},
		"evm/gov.proto": {
			"paxprotocol.paxchain.evm.AddERCNativePointerProposal":   {"title": 1, "description": 2, "token": 3, "pointer": 4, "version": 5},
			"paxprotocol.paxchain.evm.AddERCCW20PointerProposal":     {"title": 1, "description": 2, "pointee": 3, "pointer": 4, "version": 5},
			"paxprotocol.paxchain.evm.AddERCCW721PointerProposal":    {"title": 1, "description": 2, "pointee": 3, "pointer": 4, "version": 5},
			"paxprotocol.paxchain.evm.AddERCCW1155PointerProposal":   {"title": 1, "description": 2, "pointee": 3, "pointer": 4, "version": 5},
			"paxprotocol.paxchain.evm.AddCWERC20PointerProposal":     {"title": 1, "description": 2, "pointee": 3, "pointer": 4, "version": 5},
			"paxprotocol.paxchain.evm.AddCWERC721PointerProposal":    {"title": 1, "description": 2, "pointee": 3, "pointer": 4, "version": 5},
			"paxprotocol.paxchain.evm.AddCWERC1155PointerProposal":   {"title": 1, "description": 2, "pointee": 3, "pointer": 4, "version": 5},
			"paxprotocol.paxchain.evm.AddERCNativePointerProposalV2": {"title": 1, "description": 2, "token": 3, "name": 4, "symbol": 5, "decimals": 6},
			"paxprotocol.paxchain.evm.PointerBindingProposal":        {"title": 1, "description": 2, "messages": 3},
		},
		"evm/params.proto": {
			"paxprotocol.paxchain.evm.Params": {
				"priority_normalizer":                          2,
				"base_fee_per_gas":                             3,
				"minimum_fee_per_gas":                          4,
				"whitelisted_cw_code_hashes_for_delegate_call": 8,
				"deliver_tx_hook_wasm_gas_limit":               9,
				"max_dynamic_base_fee_upward_adjustment":       10,
				"max_dynamic_base_fee_downward_adjustment":     11,
				"target_gas_used_per_block":                    12,
				"maximum_fee_per_gas":                          13,
				"register_pointer_disabled":                    14,
				"pax_sstore_set_gas_eip2200":                   15,
				"allowed_fee_denoms":                           16,
				"max_fee_token_spread":                         17,
				"fee_token_enabled":                            18,
				"fee_token_distribution":                       19,
				"max_fee_token_rate_age":                       20,
			},
			"paxprotocol.paxchain.evm.ParamsPreV580": {
				"priority_normalizer": 2, "base_fee_per_gas": 3, "minimum_fee_per_gas": 4,
				"whitelisted_cw_code_hashes_for_delegate_call": 8,
			},
			"paxprotocol.paxchain.evm.ParamsPreV600": {
				"priority_normalizer": 2, "base_fee_per_gas": 3, "minimum_fee_per_gas": 4,
				"deliver_tx_hook_wasm_gas_limit": 5, "whitelisted_cw_code_hashes_for_delegate_call": 8,
			},
			"paxprotocol.paxchain.evm.ParamsPreV601": {
				"priority_normalizer": 2, "base_fee_per_gas": 3, "minimum_fee_per_gas": 4,
				"whitelisted_cw_code_hashes_for_delegate_call": 8, "deliver_tx_hook_wasm_gas_limit": 9,
				"max_dynamic_base_fee_upward_adjustment": 10, "max_dynamic_base_fee_downward_adjustment": 11,
				"target_gas_used_per_block": 12,
			},
			"paxprotocol.paxchain.evm.ParamsPreV606": {
				"priority_normalizer": 2, "base_fee_per_gas": 3, "minimum_fee_per_gas": 4,
				"whitelisted_cw_code_hashes_for_delegate_call": 8, "deliver_tx_hook_wasm_gas_limit": 9,
				"max_dynamic_base_fee_upward_adjustment": 10, "max_dynamic_base_fee_downward_adjustment": 11,
				"target_gas_used_per_block": 12, "maximum_fee_per_gas": 13,
			},
			"paxprotocol.paxchain.evm.AllowedFeeDenom": {"denom": 1, "rate": 3, "rate_update_height": 4},
		},
		"evm/query.proto": {
			"paxprotocol.paxchain.evm.QueryPaxAddressByEVMAddressRequest":  {"evm_address": 1},
			"paxprotocol.paxchain.evm.QueryPaxAddressByEVMAddressResponse": {"pax_address": 1, "associated": 2},
			"paxprotocol.paxchain.evm.QueryEVMAddressByPaxAddressRequest":  {"pax_address": 1},
			"paxprotocol.paxchain.evm.QueryEVMAddressByPaxAddressResponse": {"evm_address": 1, "associated": 2},
			"paxprotocol.paxchain.evm.QueryStaticCallRequest":              {"data": 1, "to": 2},
			"paxprotocol.paxchain.evm.QueryStaticCallResponse":             {"data": 1},
			"paxprotocol.paxchain.evm.QueryPointerRequest":                 {"pointer_type": 1, "pointee": 2},
			"paxprotocol.paxchain.evm.QueryPointerResponse":                {"pointer": 1, "version": 2, "exists": 3},
			"paxprotocol.paxchain.evm.QueryPointerVersionRequest":          {"pointer_type": 1},
			"paxprotocol.paxchain.evm.QueryPointerVersionResponse":         {"version": 1, "cw_code_id": 2},
			"paxprotocol.paxchain.evm.QueryPointeeRequest":                 {"pointer_type": 1, "pointer": 2},
			"paxprotocol.paxchain.evm.QueryPointeeResponse":                {"pointee": 1, "version": 2, "exists": 3},
		},
		"evm/receipt.proto": {
			"paxprotocol.paxchain.evm.Log": {"address": 1, "topics": 2, "data": 3, "index": 4, "synthetic": 5},
			"paxprotocol.paxchain.evm.Receipt": {
				"tx_type": 1, "cumulative_gas_used": 2, "contract_address": 3, "tx_hash_hex": 4,
				"gas_used": 5, "effective_gas_price": 6, "block_number": 7, "transaction_index": 8,
				"status": 9, "from": 10, "to": 11, "vm_error": 12, "logs": 13, "logsBloom": 14,
			},
		},
		"evm/tx.proto": {
			"paxprotocol.paxchain.evm.MsgEVMTransaction":                   {"data": 1, "derived": 2},
			"paxprotocol.paxchain.evm.MsgEVMTransactionResponse":           {"gas_used": 1, "vm_error": 2, "return_data": 3, "hash": 4, "logs": 5},
			"paxprotocol.paxchain.evm.MsgInternalEVMCall":                  {"sender": 1, "value": 2, "to": 3, "data": 4},
			"paxprotocol.paxchain.evm.MsgInternalEVMCallResponse":          {},
			"paxprotocol.paxchain.evm.MsgInternalEVMDelegateCall":          {"sender": 1, "codeHash": 2, "to": 3, "data": 4, "fromContract": 5},
			"paxprotocol.paxchain.evm.MsgInternalEVMDelegateCallResponse":  {},
			"paxprotocol.paxchain.evm.MsgSend":                             {"from_address": 1, "to_address": 2, "amount": 3},
			"paxprotocol.paxchain.evm.MsgSendResponse":                     {},
			"paxprotocol.paxchain.evm.MsgRegisterPointer":                  {"sender": 1, "pointer_type": 2, "erc_address": 3},
			"paxprotocol.paxchain.evm.MsgRegisterPointerResponse":          {"pointer_address": 1},
			"paxprotocol.paxchain.evm.MsgAssociateContractAddress":         {"sender": 1, "address": 2},
			"paxprotocol.paxchain.evm.MsgAssociateContractAddressResponse": {},
			"paxprotocol.paxchain.evm.MsgAssociate":                        {"sender": 1, "custom_message": 2},
			"paxprotocol.paxchain.evm.MsgAssociateResponse":                {},
			"paxprotocol.paxchain.evm.MsgClaim":                            {"sender": 1, "claimer": 2},
			"paxprotocol.paxchain.evm.Asset":                               {"asset_type": 1, "contract_address": 2, "denom": 3},
			"paxprotocol.paxchain.evm.MsgClaimSpecific":                    {"sender": 1, "claimer": 2, "assets": 3},
			"paxprotocol.paxchain.evm.MsgBindERCNativePointer":             {"authority": 1, "token": 2, "pointer": 3, "version": 4},
			"paxprotocol.paxchain.evm.MsgBindERCNativePointerResponse":     {},
		},
		"evm/types.proto": {
			"paxprotocol.paxchain.evm.Whitelist":    {"hashes": 1},
			"paxprotocol.paxchain.evm.DeferredInfo": {"tx_index": 1, "tx_hash": 2, "tx_bloom": 3, "surplus": 4, "error": 5},
		},
	}

	apiRoot := filepath.Join("..", "..", "..", "api")
	paths, err := filepath.Glob(filepath.Join(apiRoot, "evm", "*.proto"))
	if err != nil {
		t.Fatal(err)
	}
	files := make(map[string]bool, len(paths)+len(want))
	for _, path := range paths {
		files["evm/"+filepath.Base(path)] = true
	}
	for file := range want {
		if !files[file] {
			t.Errorf("removed proto file %s", file)
			files[file] = false
		}
	}
	names := make([]string, 0, len(files))
	for file := range files {
		names = append(names, file)
	}
	sort.Strings(names)
	for _, file := range names {
		t.Run(file, func(t *testing.T) {
			expected, exists := want[file]
			if !exists {
				t.Errorf("added proto file %s", file)
			}
			t.Run("source", func(t *testing.T) {
				if !files[file] {
					compareEVMProtoSurface(t, nil, expected)
					return
				}
				descriptor := readEVMProtoSource(t, filepath.Join(apiRoot, filepath.FromSlash(file)))
				compareEVMProtoSurface(t, readEVMProtoSurface(t, descriptor), expected)
			})
			t.Run("registered", func(t *testing.T) {
				data := gogoproto.FileDescriptor(file)
				if len(data) == 0 {
					t.Fatalf("missing registered descriptor for %s", file)
				}
				reader, err := gzip.NewReader(bytes.NewReader(data))
				if err != nil {
					t.Fatal(err)
				}
				raw, err := io.ReadAll(reader)
				closeErr := reader.Close()
				if err != nil {
					t.Fatal(err)
				}
				if closeErr != nil {
					t.Fatal(closeErr)
				}
				var descriptor descriptorpb.FileDescriptorProto
				if err := proto.Unmarshal(raw, &descriptor); err != nil {
					t.Fatal(err)
				}
				if descriptor.GetName() != file {
					t.Errorf("registered file name changed: got %q, want %q", descriptor.GetName(), file)
				}
				compareEVMProtoSurface(t, readEVMProtoSurface(t, &descriptor), expected)
			})
		})
	}
}

func readEVMProtoSource(t *testing.T, path string) *descriptorpb.FileDescriptorProto {
	t.Helper()
	source, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	var lexer scanner.Scanner
	lexer.Init(source)
	lexer.Filename = path
	lexer.Mode = scanner.ScanIdents | scanner.ScanInts | scanner.ScanStrings | scanner.ScanChars | scanner.ScanComments | scanner.SkipComments
	lexer.Error = func(s *scanner.Scanner, msg string) {
		t.Errorf("%s: %s", s.Position, msg)
	}
	var tokens []string
	for token := lexer.Scan(); token != scanner.EOF; token = lexer.Scan() {
		tokens = append(tokens, lexer.TokenText())
	}
	if lexer.ErrorCount != 0 {
		t.FailNow()
	}
	file := &descriptorpb.FileDescriptorProto{}
	pos := 0
	take := func() string {
		if pos == len(tokens) {
			t.Fatalf("%s: unexpected end of proto source", path)
		}
		token := tokens[pos]
		pos++
		return token
	}
	expect := func(want string) {
		if got := take(); got != want {
			t.Fatalf("%s: got token %q, want %q", path, got, want)
		}
	}
	var block func(*descriptorpb.DescriptorProto, bool)
	block = func(message *descriptorpb.DescriptorProto, nested bool) {
		for pos < len(tokens) {
			token := take()
			switch token {
			case "}":
				if !nested {
					t.Fatalf("%s: unexpected closing brace", path)
				}
				return
			case ";":
				continue
			case "package":
				var name []string
				for token := take(); token != ";"; token = take() {
					name = append(name, token)
				}
				file.Package = proto.String(strings.Join(name, ""))
			case "message":
				child := &descriptorpb.DescriptorProto{Name: proto.String(take())}
				expect("{")
				if message == nil {
					file.MessageType = append(file.MessageType, child)
				} else {
					message.NestedType = append(message.NestedType, child)
				}
				block(child, true)
			case "oneof":
				if message == nil {
					t.Fatalf("%s: oneof outside a message", path)
				}
				take()
				expect("{")
				block(message, true)
			case "enum", "service", "extend":
				take()
				expect("{")
				for depth := 1; depth > 0; {
					switch take() {
					case "{":
						depth++
					case "}":
						depth--
					}
				}
			default:
				statement := []string{token}
				for braces, brackets := 0, 0; ; {
					next := take()
					if next == ";" && braces == 0 && brackets == 0 {
						break
					}
					switch next {
					case "{":
						braces++
					case "}":
						braces--
					case "[":
						brackets++
					case "]":
						brackets--
					}
					if braces < 0 || brackets < 0 {
						t.Fatalf("%s: unbalanced field declaration %v", path, statement)
					}
					statement = append(statement, next)
				}
				if message == nil || token == "option" || token == "reserved" || token == "extensions" {
					continue
				}
				equal := -1
				for i, token := range statement {
					if token == "=" {
						equal = i
						break
					}
				}
				if equal < 2 || equal+1 >= len(statement) {
					t.Fatalf("%s: unsupported field declaration %v", path, statement)
				}
				number, err := strconv.ParseInt(statement[equal+1], 0, 32)
				if err != nil || number <= 0 {
					t.Fatalf("%s: invalid field number in %v: %v", path, statement, err)
				}
				message.Field = append(message.Field, &descriptorpb.FieldDescriptorProto{
					Name: proto.String(statement[equal-1]), Number: proto.Int32(int32(number)),
				})
			}
		}
		if nested {
			t.Fatalf("%s: unclosed message or oneof", path)
		}
	}
	block(nil, false)
	if file.GetPackage() == "" {
		t.Fatalf("%s: missing proto package", path)
	}
	return file
}

func readEVMProtoSurface(t *testing.T, file *descriptorpb.FileDescriptorProto) evmProtoSurface {
	t.Helper()
	surface := make(evmProtoSurface)
	var visit func(string, []*descriptorpb.DescriptorProto)
	visit = func(parent string, messages []*descriptorpb.DescriptorProto) {
		for _, message := range messages {
			name := parent + "." + message.GetName()
			if _, exists := surface[name]; exists {
				t.Errorf("duplicate message %s", name)
			}
			fields := make(map[string]int32)
			for _, field := range message.GetField() {
				if _, exists := fields[field.GetName()]; exists {
					t.Errorf("duplicate field %s.%s", name, field.GetName())
				}
				fields[field.GetName()] = field.GetNumber()
			}
			surface[name] = fields
			visit(name, message.GetNestedType())
		}
	}
	visit(file.GetPackage(), file.GetMessageType())
	return surface
}

func compareEVMProtoSurface(t *testing.T, got, want evmProtoSurface) {
	t.Helper()
	flatten := func(surface evmProtoSurface) map[string]int32 {
		entries := make(map[string]int32)
		for message, fields := range surface {
			entries[message] = 0
			for field, number := range fields {
				entries[message+"."+field] = number
			}
		}
		return entries
	}
	actual, expected := flatten(got), flatten(want)
	keys := make(map[string]bool, len(actual)+len(expected))
	for name := range actual {
		keys[name] = true
	}
	for name := range expected {
		keys[name] = true
	}
	names := make([]string, 0, len(keys))
	for name := range keys {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		gotNumber, present := actual[name]
		wantNumber, frozen := expected[name]
		switch {
		case !frozen:
			t.Errorf("added entry %s (number %d)", name, gotNumber)
		case !present:
			t.Errorf("removed entry %s (frozen number %d)", name, wantNumber)
		case gotNumber != wantNumber:
			t.Errorf("changed entry %s: number %d, want %d", name, gotNumber, wantNumber)
		}
	}
}

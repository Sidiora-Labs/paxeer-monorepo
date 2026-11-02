package api_test

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	xevmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/xevm/types"
	xethtx "github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/xevm/types/ethtx"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	ethtx "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	gogoproto "github.com/gogo/protobuf/proto"
	"google.golang.org/protobuf/proto"
)

type xevmDescriptor func() ([]byte, []int)

type xevmShadowFile struct {
	name string
	evm  xevmDescriptor
	xevm xevmDescriptor
}

func (file xevmShadowFile) FileDescriptor(t *testing.T) {
	t.Helper()
	evmBytes, _ := file.evm()
	registered := registeredFile(t, file.name)
	if !bytes.Equal(gogoproto.FileDescriptor(file.name), evmBytes) {
		t.Errorf("%s: registered gzip descriptor differs from modules/evm/types", file.name)
	}
	if !proto.Equal(registered, decodeFile(t, evmBytes)) {
		t.Errorf("%s: decoded registry descriptor differs from modules/evm/types", file.name)
	}
}

func (file xevmShadowFile) RegisterFile(t *testing.T) {
	t.Helper()
	file.FileDescriptor(t)
	xevmBytes, _ := file.xevm()
	xevmFile := decodeFile(t, xevmBytes)
	name := xevmFile.GetName()
	if name == "" || strings.HasPrefix(name, "evm/") || strings.HasPrefix(name, "eth/") {
		t.Fatalf("%s: xevm descriptor still uses a shared file name %q", file.name, name)
	}
	wantName := "xevm/" + strings.TrimPrefix(file.name, "evm/")
	wantPackage := "paxprotocol.paxchain.gigaevm"
	wantGoPackage := "github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/xevm/types"
	if file.name == "eth/tx.proto" {
		wantName = "xevm/ethtx/tx.proto"
		wantPackage = "paxprotocol.paxchain.gigaeth"
		wantGoPackage += "/ethtx"
	}
	if name != wantName {
		t.Errorf("%s: xevm descriptor file name = %q, want %q", file.name, name, wantName)
	}
	xevmRegistered := registeredFile(t, name)
	if !bytes.Equal(gogoproto.FileDescriptor(name), xevmBytes) {
		t.Errorf("%s: registered gzip descriptor differs from engine/deps/xevm/types", name)
	}
	if !proto.Equal(xevmRegistered, xevmFile) {
		t.Errorf("%s: decoded registry descriptor differs from engine/deps/xevm/types", name)
	}
	if xevmFile.GetPackage() != wantPackage {
		t.Errorf("%s: descriptor package = %q, want %q", name, xevmFile.GetPackage(), wantPackage)
	}
	if len(xevmFile.GetMessageType())+len(xevmFile.GetEnumType()) == 0 {
		t.Fatalf("%s: descriptor has no registered types to check", name)
	}
	for _, message := range xevmFile.GetMessageType() {
		fullName := xevmFile.GetPackage() + "." + message.GetName()
		goType := gogoproto.MessageType(fullName)
		if goType == nil {
			t.Errorf("%s: descriptor message %q has no matching RegisterType entry", name, fullName)
			continue
		}
		if goType.Kind() != reflect.Ptr || goType.Elem().PkgPath() != wantGoPackage {
			t.Errorf("%s: RegisterType(%q) resolves to %v, want a type from %s", name, fullName, goType, wantGoPackage)
		}
	}
	for _, enum := range xevmFile.GetEnumType() {
		fullName := xevmFile.GetPackage() + "." + enum.GetName()
		wantValues := make(map[string]int32, len(enum.GetValue()))
		for _, value := range enum.GetValue() {
			wantValues[value.GetName()] = value.GetNumber()
		}
		if values := gogoproto.EnumValueMap(fullName); !reflect.DeepEqual(values, wantValues) {
			t.Errorf("%s: descriptor enum %q differs from RegisterEnum: got %v, want %v", name, fullName, values, wantValues)
		}
	}
}

func TestXevmDescriptorsDoNotShadowEvmFiles(t *testing.T) {
	files := []xevmShadowFile{
		{"evm/config.proto", (&evmtypes.ChainConfig{}).Descriptor, (&xevmtypes.ChainConfig{}).Descriptor},
		{"evm/enums.proto", evmtypes.PointerType_ERC20.EnumDescriptor, xevmtypes.PointerType_ERC20.EnumDescriptor},
		{"evm/genesis.proto", (&evmtypes.AddressAssociation{}).Descriptor, (&xevmtypes.AddressAssociation{}).Descriptor},
		{"evm/gov.proto", (&evmtypes.AddERCNativePointerProposal{}).Descriptor, (&xevmtypes.AddERCNativePointerProposal{}).Descriptor},
		{"evm/params.proto", (&evmtypes.Params{}).Descriptor, (&xevmtypes.Params{}).Descriptor},
		{"evm/query.proto", (&evmtypes.QueryPaxAddressByEVMAddressRequest{}).Descriptor, (&xevmtypes.QueryPaxAddressByEVMAddressRequest{}).Descriptor},
		{"evm/receipt.proto", (&evmtypes.Log{}).Descriptor, (&xevmtypes.Log{}).Descriptor},
		{"evm/tx.proto", (&evmtypes.MsgEVMTransaction{}).Descriptor, (&xevmtypes.MsgEVMTransaction{}).Descriptor},
		{"evm/types.proto", (&evmtypes.Whitelist{}).Descriptor, (&xevmtypes.Whitelist{}).Descriptor},
		{"eth/tx.proto", (&ethtx.AccessTuple{}).Descriptor, (&xethtx.AccessTuple{}).Descriptor},
	}
	for _, file := range files {
		t.Run(file.name, file.RegisterFile)
	}

	t.Run("TxDataAny", func(t *testing.T) {
		app, _, _ := appQueryHelper(t)
		registry := app.InterfaceRegistry()
		xevmtypes.RegisterInterfaces(registry)

		transactions := []struct {
			name string
			evm  ethtx.TxData
			xevm xethtx.TxData
		}{
			{"DynamicFeeTx", &ethtx.DynamicFeeTx{Nonce: 17, Data: []byte{1, 2, 3}}, &xethtx.DynamicFeeTx{Nonce: 17, Data: []byte{1, 2, 3}}},
			{"AccessListTx", &ethtx.AccessListTx{Nonce: 18}, &xethtx.AccessListTx{Nonce: 18}},
			{"LegacyTx", &ethtx.LegacyTx{Nonce: 19}, &xethtx.LegacyTx{Nonce: 19}},
			{"BlobTx", &ethtx.BlobTx{Nonce: 20}, &xethtx.BlobTx{Nonce: 20}},
			{"AssociateTx", &ethtx.AssociateTx{CustomMessage: "descriptor namespace regression"}, &xethtx.AssociateTx{CustomMessage: "descriptor namespace regression"}},
			{"SetCodeTx", &ethtx.SetCodeTx{Nonce: 21}, &xethtx.SetCodeTx{Nonce: 21}},
		}
		for _, transaction := range transactions {
			t.Run(transaction.name, func(t *testing.T) {
				for _, domain := range []struct {
					name      string
					ifaceName string
					typeURL   string
					message   gogoproto.Message
					pack      func() (*codectypes.Any, error)
				}{
					{"evm", "paxprotocol.paxchain.evm.TxData", "/paxprotocol.paxchain.eth." + transaction.name, transaction.evm, func() (*codectypes.Any, error) { return evmtypes.PackTxData(transaction.evm) }},
					{"xevm", "paxprotocol.paxchain.gigaevm.TxData", "/paxprotocol.paxchain.gigaeth." + transaction.name, transaction.xevm, func() (*codectypes.Any, error) { return xevmtypes.PackTxData(transaction.xevm) }},
				} {
					t.Run(domain.name, func(t *testing.T) {
						packed, err := domain.pack()
						if err != nil {
							t.Fatalf("pack TxData: %v", err)
						}
						if packed.TypeUrl != domain.typeURL {
							t.Fatalf("TxData type URL = %q, want %q", packed.TypeUrl, domain.typeURL)
						}
						found := false
						for _, typeURL := range registry.ListImplementations(domain.ifaceName) {
							found = found || typeURL == domain.typeURL
						}
						if !found {
							t.Errorf("%s: codec did not register implementation %s", domain.ifaceName, domain.typeURL)
						}
						resolved, err := registry.Resolve(domain.typeURL)
						if err != nil {
							t.Fatalf("resolve TxData type URL: %v", err)
						}
						if reflect.TypeOf(resolved) != reflect.TypeOf(domain.message) {
							t.Errorf("%s resolves to %T, want %T", domain.typeURL, resolved, domain.message)
						}
						wireAny := &codectypes.Any{TypeUrl: packed.TypeUrl, Value: append([]byte(nil), packed.Value...)}
						var unpacked gogoproto.Message
						if domain.name == "xevm" {
							var txData xethtx.TxData
							err = registry.UnpackAny(wireAny, &txData)
							unpacked = txData
						} else {
							var txData ethtx.TxData
							err = registry.UnpackAny(wireAny, &txData)
							unpacked = txData
						}
						if err != nil {
							t.Fatalf("unpack TxData with app registry: %v", err)
						}
						if reflect.TypeOf(unpacked) != reflect.TypeOf(domain.message) || !gogoproto.Equal(unpacked, domain.message) {
							t.Errorf("%s unpacked as %T %v, want %T %v", domain.typeURL, unpacked, unpacked, domain.message, domain.message)
						}
					})
				}
			})
		}
	})
}

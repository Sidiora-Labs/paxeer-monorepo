package receipt

import (
	"bytes"
	"encoding/json"
	"math/big"
	"reflect"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
)

func deploymentBoundsBatch(target common.Address, calls [][]byte) ([]byte, []byte) {
	var code, input []byte
	var failures []int
	push := func(n int) { code = append(code, 0x62, byte(n>>16), byte(n>>8), byte(n)) }
	for _, call := range calls {
		push(len(call))
		code = append(code, 0x36, 0x60, 0, 0x37)
		prefix := bytes.TrimRight(call, "\x00")
		push(len(prefix))
		push(len(input))
		code = append(code, 0x60, 0, 0x37)
		code = append(code, 0x60, 0, 0x60, 0)
		push(len(call))
		code = append(code, 0x60, 0, 0x60, 0, 0x73)
		code = append(code, target[:]...)
		code = append(code, 0x5a, 0xf1, 0x15, 0x61, 0, 0, 0x57)
		failures = append(failures, len(code)-3)
		input = append(input, prefix...)
	}
	code = append(code, 0x00)
	for _, offset := range failures {
		code[offset], code[offset+1] = byte(len(code)>>8), byte(len(code))
	}
	code = append(code, 0x5b, 0x60, 0, 0x60, 0, 0xfd)
	init := []byte{0x61, byte(len(code) >> 8), byte(len(code)), 0x61, 0, 0x0f, 0x60, 0, 0x39, 0x61, byte(len(code) >> 8), byte(len(code)), 0x60, 0, 0xf3}
	return append(init, code...), input
}

func TestDeploymentLogBoundsReceipt(t *testing.T) {
	n, options, _ := deployPrivateQualification(t)
	emitterABI, _ := privateArtifact(t, "EventEmitter")
	pack := func(method string, args ...any) []byte {
		t.Helper()
		data, err := emitterABI.Pack(method, args...)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	emptyABI, err := abi.JSON(strings.NewReader(`[]`))
	if err != nil {
		t.Fatal(err)
	}
	emit := func(calls [][]byte) (*QualifiedReceipt, *types.Receipt, *RPCQualification) {
		t.Helper()
		code, input := deploymentBoundsBatch(common.HexToAddress(options.Roles["EventEmitter"].Address), calls)
		address, tx, caller, err := bind.DeployContract(n.auth, emptyABI, code, n.client)
		if err != nil {
			t.Fatal(err)
		}
		n.mined(t, tx)
		n.transact(t, "EventEmitter", "setAuthorizedEmitter", address, true)
		var selector [4]byte
		copy(selector[:], emitterABI.Methods["emitGovernance"].ID)
		n.transact(t, "EventEmitter", "setTypedEmitter", address, selector, true)
		authority, err := NewPrivateFixtureRPCQualification(n.ctx, n.client, options)
		if err != nil {
			t.Fatal(err)
		}
		n.auth.GasLimit = 16000000
		tx, err = caller.RawTransact(n.auth, input)
		n.auth.GasLimit = 0
		if err != nil {
			t.Fatal(err)
		}
		mined, err := bind.WaitMined(n.ctx, n.client, tx)
		if err != nil {
			t.Fatal(err)
		}
		if mined.Status != types.ReceiptStatusSuccessful {
			t.Fatalf("emitter transaction status=%d gas=%d limit=%d", mined.Status, mined.GasUsed, tx.Gas())
		}
		if len(mined.Logs) != len(calls) {
			t.Fatalf("emitter logs: %d, calls: %d", len(mined.Logs), len(calls))
		}
		bundle, err := authority.Bundle(n.ctx, mined.TxHash)
		if err != nil {
			t.Fatal(err)
		}
		if err := authority.ValidateReceipt(n.ctx, bundle); err != nil {
			t.Fatal(err)
		}
		q, err := Qualify(n.ctx, authority.Registry(), authority, bundle)
		if err != nil {
			t.Fatal(err)
		}
		return q, mined, authority
	}
	legal := pack("emitGovernance", uint8(1), [32]byte{}, n.owner, []byte{1, 2})
	dataType := emitterABI.Methods["emitEventLog"].Inputs[1].Type
	emptyData := reflect.New(dataType.GetType()).Elem().Interface()
	q, _, _ := emit([][]byte{
		pack("emitEventLog", string([]byte{0xff}), emptyData),
		pack("emitEventLog", "name\x00", emptyData),
		pack("emitGovernance", uint8(1), [32]byte{}, n.owner, make([]byte, decode.MaxFieldBytes+1)),
		legal,
	})
	events, err := q.QualifiedEvents(n.ctx)
	if err != nil || len(events) != 4 {
		t.Fatalf("qualified events: %d: %v", len(events), err)
	}
	for i, cause := range []string{"withheld:eventName:invalid_utf8", "withheld:eventName:nul", "withheld:payload:max_field_bytes"} {
		field, empty := "eventName", ""
		if i == 2 {
			field, empty = "payload", "0x"
		}
		if events[i].Status != "decoded" || events[i].Event.Args[field] != empty || !reflect.DeepEqual(events[i].Alerts, []string{cause}) {
			t.Fatalf("withheld field %d: %+v", i, events[i])
		}
	}
	if last := events[3]; last.Status != "decoded" || last.EventName != "Governance" || last.Event.Args["payload"] != "0x0102" || len(last.Alerts) != 0 {
		t.Fatalf("lawful log after withheld fields: %+v", last)
	}
	q, mined, authority := emit([][]byte{
		pack("emitGovernance", uint8(1), [32]byte{}, n.owner, make([]byte, decode.MaxEnvelopeBytes+1)),
		legal,
	})
	bounded, err := authority.Registry().Decode(*mined.Logs[0])
	if err != nil || bounded.Status != "quarantined" || bounded.FailureReason != "bounds" || bounded.Event != nil || bounded.RawSummary == nil || bounded.RawSummary.DataBytes != len(mined.Logs[0].Data) || bounded.Raw.Data != "" || len(bounded.Raw.Topics) != 0 {
		t.Fatalf("bounded receipt log: %+v: %v", bounded, err)
	}
	events, err = q.QualifiedEvents(n.ctx)
	if err != nil || len(events) != 1 {
		t.Fatalf("qualified events after quarantine: %d: %v", len(events), err)
	}
	if last := events[0]; last.Status != "decoded" || last.EventName != "Governance" || last.Event.Args["payload"] != "0x0102" || len(last.Alerts) != 0 {
		t.Fatalf("lawful log after bounds: %+v", last)
	}
	logs, err := q.SourceLogs(n.ctx)
	if err != nil || len(logs) != 2 || !bytes.Equal(logs[0].Data, mined.Logs[0].Data) {
		t.Fatalf("receipt retains original data: logs=%d: %v", len(logs), err)
	}
	exerciseDeploymentLogBoundsFactoryReceipts(t, n, options)
}

func exerciseDeploymentLogBoundsFactoryReceipts(t *testing.T, n *privateNodeFixture, options PrivateFixtureRPCOptions) {
	t.Helper()
	factory := common.HexToAddress(n.roles["SidioraFactory"].Address)
	for _, binding := range []struct {
		role, name string
		account    common.Address
	}{
		{"PoolRegistry", "FACTORY_ROLE", factory}, {"FeeAccumulator", "FACTORY_ROLE", factory}, {"SidioraNFT", "MINTER_ROLE", factory}, {"SidioraFactory", "ROUTER_ROLE", n.owner},
	} {
		n.transact(t, binding.role, "grantRole", crypto.Keccak256Hash([]byte(binding.name)), binding.account)
	}
	n.transact(t, "EventEmitter", "setSidioraFactory", factory)
	n.transact(t, "EventEmitter", "setAuthorizedEmitter", factory, true)
	emitterABI, _ := privateArtifact(t, "EventEmitter")
	for _, method := range []string{"emitMarketCreated", "emitTokenDeployed"} {
		var selector [4]byte
		copy(selector[:], emitterABI.Methods[method].ID)
		n.transact(t, "EventEmitter", "setTypedEmitter", factory, selector, true)
	}
	factoryABI, _ := privateArtifact(t, "SidioraFactory")
	var catalog decode.Catalog
	if err := json.Unmarshal(options.CatalogJSON, &catalog); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, tokenName, symbol, field, cause string }{
		{"invalid UTF8", string([]byte{0xff}), "COIN", "name", "invalid_utf8"},
		{"zero character", "coin\x00", "COIN", "name", "nul"},
		{"long symbol", "Coin", strings.Repeat("\x00", decode.MaxFieldBytes+1), "symbol", "max_field_bytes"},
		{"empty fields", "", "", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.cause == "max_field_bytes" {
				tokenABI, code := privateArtifact(t, "SidioraERC20")
				constructor, err := tokenABI.Pack("", tc.tokenName, tc.symbol, big.NewInt(1), factory)
				if err != nil {
					t.Fatal(err)
				}
				if size := len(code) + len(constructor); size <= params.MaxInitCodeSize {
					t.Fatalf("oversized text init code: %d <= %d", size, params.MaxInitCodeSize)
				}
				auth := *n.auth
				auth.GasLimit = 12000000
				tx, err := n.contracts["SidioraFactory"].Transact(&auth, "createMarketFor", n.owner, tc.tokenName, tc.symbol, uint8(0), common.Address{})
				if err != nil {
					t.Fatal(err)
				}
				created, err := bind.WaitMined(n.ctx, n.client, tx)
				if err != nil {
					t.Fatal(err)
				}
				if created.Status != types.ReceiptStatusFailed || len(created.Logs) != 0 {
					t.Fatalf("init code limit must prevent creation: status=%d logs=%d", created.Status, len(created.Logs))
				}
				return
			}
			n.auth.GasLimit = 12000000
			created := n.transact(t, "SidioraFactory", "createMarketFor", n.owner, tc.tokenName, tc.symbol, uint8(0), common.Address{})
			n.auth.GasLimit = 0
			var token, pool common.Address
			for _, log := range created.Logs {
				if log.Address == factory && len(log.Topics) == 4 && log.Topics[0] == factoryABI.Events["MarketCreated"].ID {
					token, pool = common.BytesToAddress(log.Topics[1][:]), common.BytesToAddress(log.Topics[2][:])
				}
			}
			if token == (common.Address{}) || pool == (common.Address{}) {
				t.Fatal("factory creation addresses absent")
			}
			bound := options
			bound.Discovery = nil
			for _, item := range []struct {
				role, kind string
				address    common.Address
			}{{"SidioraERC20", "", token}, {"SidioraPool", "beacon_proxy", pool}} {
				identity := n.identity(t, item.address, "direct")
				for _, contract := range catalog.Contracts {
					if contract.Name == item.role {
						bound.Discovery = append(bound.Discovery, RPCDiscovery{Role: item.role, Kind: item.kind, ABIRevision: contract.ABIRevision, RuntimeHash: identity.RuntimeHash})
					}
				}
			}
			authority, err := NewPrivateFixtureRPCQualification(n.ctx, n.client, bound)
			if err != nil {
				t.Fatal(err)
			}
			authority, err = authority.discover(n.ctx, types.Receipts{created})
			if err != nil {
				t.Fatal(err)
			}
			bundle, err := authority.Bundle(n.ctx, created.TxHash)
			if err != nil {
				t.Fatal(err)
			}
			if err := authority.ValidateReceipt(n.ctx, bundle); err != nil {
				t.Fatal(err)
			}
			q, err := Qualify(n.ctx, authority.Registry(), authority, bundle)
			if err != nil {
				t.Fatal(err)
			}
			envelopes, err := q.QualifiedEvents(n.ctx)
			if err != nil {
				t.Fatal(err)
			}
			foundToken, foundMarket := false, false
			for _, env := range envelopes {
				if env.Status != "decoded" || env.Event == nil {
					t.Fatal("qualified event status required")
				}
				if env.EmitterRole == "SidioraFactory" && env.EventName == "MarketCreated" {
					foundMarket = true
				}
				if env.EventName != "TokenDeployed" {
					continue
				}
				foundToken = true
				name, symbol := tc.tokenName, tc.symbol
				var alerts []string
				if tc.field != "" {
					alerts = []string{"withheld:" + tc.field + ":" + tc.cause}
				}
				if tc.field == "name" {
					name = ""
				}
				if tc.field == "symbol" {
					symbol = ""
				}
				if env.Event.Args["name"] != name || env.Event.Args["symbol"] != symbol || env.Event.Args["decimals"] != "6" || !reflect.DeepEqual(env.Alerts, alerts) {
					t.Fatalf("qualified token metadata: %+v", env)
				}
			}
			if !foundToken || !foundMarket {
				t.Fatalf("factory receipt events: token=%t market=%t", foundToken, foundMarket)
			}
		})
	}
}

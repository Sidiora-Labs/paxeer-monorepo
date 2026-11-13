package main

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"sort"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	anchor "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxanchor"
	verify "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxverify"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
)

// zeroABIValue supplies canonical arguments to retired operations. Refusal
// must come from the retirement gate, not from malformed calldata.
func zeroABIValue(t reflect.Type) reflect.Value {
	switch t.Kind() {
	case reflect.Ptr:
		v := reflect.New(t.Elem())
		v.Elem().Set(zeroABIValue(t.Elem()))
		return v
	case reflect.Struct:
		v := reflect.New(t).Elem()
		for i := 0; i < t.NumField(); i++ {
			if v.Field(i).CanSet() {
				v.Field(i).Set(zeroABIValue(t.Field(i).Type))
			}
		}
		return v
	case reflect.Array:
		v := reflect.New(t).Elem()
		for i := 0; i < v.Len(); i++ {
			v.Index(i).Set(zeroABIValue(t.Elem()))
		}
		return v
	default:
		return reflect.Zero(t)
	}
}

// retiredOperation checks the real historical contracts across their retirement
// heights. Their retained references are inventoried in ../repair_22_39_275.kvx;
// replacing these probes would lose the exact ABI refusal checks.
func retiredOperation(a *app.App, ctx sdk.Context, entry forkPrecompile, method abi.Method) bool {
	merged := a.UpgradeKeeper.IsUpgradeActiveAtHeight(ctx, app.MergeUpgrade, ctx.BlockHeight())
	switch entry.Name {
	case "layerxverify":
		return merged
	case "layerxcustody":
		return a.LayerXCustodyKeeper.Retired(ctx)
	case "layerxanchor":
		return a.UpgradeKeeper.IsUpgradeActiveAtHeight(ctx, anchor.StoreDeleteUpgrade, ctx.BlockHeight()) || (merged && !method.IsConstant())
	}
	return false
}

// supportedOperation obtains a deterministic call and its expected, typed
// result from the same committed keeper state that the EVM must expose.
func supportedOperation(a *app.App, ctx sdk.Context, entry forkPrecompile, method string) (args, want []interface{}, err error) {
	switch entry.Name + "." + method {
	case "layerxverify.verifyEd25519":
		private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x81}, ed25519.SeedSize))
		var public [32]byte
		copy(public[:], private.Public().(ed25519.PublicKey))
		message := []byte("upgrade-replay historical raw signature")
		return []interface{}{public, verify.RawMessageDomain, message, ed25519.Sign(private, message)}, []interface{}{true}, nil
	case "layerxcustody.depositCount":
		return nil, []interface{}{a.LayerXCustodyKeeper.GetDepositCount(ctx)}, nil
	case "layerxanchor.threshold":
		return nil, []interface{}{a.LayerXAnchorKeeper.GetParams(ctx).Threshold}, nil
	case "layerxledger.accountId":
		owner, asset := [32]byte{0x81}, [32]byte{0x11}
		_, id, err := lxphost.AssetAccountID(owner, asset)
		return []interface{}{owner, asset}, []interface{}{id}, err
	case "layerxpay.progressHighWater":
		id := [32]byte{0x81, 0x11}
		value, _ := a.LayerXPayKeeper.ProgressHighWater(ctx, id)
		return []interface{}{id}, []interface{}{value}, nil
	}
	return nil, nil, fmt.Errorf("no replay operation for %s.%s", entry.Name, method)
}

func assertPrecompileOperations(c *checks, a *app.App, ctx sdk.Context, entry forkPrecompile, contract vm.PrecompiledContract, results map[string][]byte) {
	if len(entry.Operations) == 0 {
		return
	}
	p, ok := contract.(precompiles.IPrecompile)
	if !c.assert(ok, "%s exposes its production ABI", entry.Name) {
		return
	}
	if !c.assert(p.Address() == common.HexToAddress(entry.Address), "%s serves the contract at its declared address", entry.Name) {
		return
	}
	contractABI := p.GetABI()
	names := append([]string(nil), entry.Operations...)
	for name, method := range contractABI.Methods {
		if retiredOperation(a, ctx, entry, method) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	previous := ""
	for _, name := range names {
		if name == previous {
			continue
		}
		previous = name
		method, ok := contractABI.Methods[name]
		if !c.assert(ok, "%s supports declared operation %s", entry.Name, name) {
			continue
		}
		retired := retiredOperation(a, ctx, entry, method)
		var args, want []interface{}
		var err error
		if retired {
			for _, input := range method.Inputs {
				args = append(args, zeroABIValue(input.Type.GetType()).Interface())
			}
		} else {
			args, want, err = supportedOperation(a, ctx, entry, name)
			if !c.assert(err == nil, "%s.%s expected result: %v", entry.Name, name, err) {
				continue
			}
		}
		input, err := contractABI.Pack(name, args...)
		if !c.assert(err == nil, "%s.%s ABI-encodes: %v", entry.Name, name, err) {
			continue
		}
		// A fresh real StateDB isolates each probe; calls here are views or
		// retired writes and must never become writes to the replayed block.
		db := state.NewDBImpl(ctx, &a.EvmKeeper, false)
		evm := &vm.EVM{StateDB: db}
		output, _, callErr := vm.RunPrecompiledContract(contract, evm, common.Address{}, common.Address{}, input, 10_000_000, new(big.Int), nil, method.IsConstant(), false)
		key := entry.Name + "." + name
		results[key] = bytes.Clone(output)
		if retired {
			reason, decodeErr := abi.UnpackRevert(output)
			expected := pcommon.RevertReason(errors.New("LXP_MODULE_DISABLED: module disabled; " + p.GetName() + " retired"))
			c.assert(errors.Is(callErr, vm.ErrExecutionReverted) && decodeErr == nil && bytes.Equal(output, expected),
				"%s returns its exact retired refusal (error %v, reason %q)", key, callErr, reason)
		} else {
			expected, packErr := method.Outputs.Pack(want...)
			c.assert(callErr == nil && packErr == nil && bytes.Equal(output, expected), "%s executes successfully with the expected result (error %v, output %x, expected %x)", key, callErr, output, expected)
		}
		c.assert(db.Err() == nil, "%s leaves no state database error: %v", key, db.Err())
	}
}

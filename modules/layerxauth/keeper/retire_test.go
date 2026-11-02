package keeper_test

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxauth"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	storekeys "github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/keys"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestAuthModuleStoreHasNoIdempotencyKeys(t *testing.T) {
	for _, kind := range []byte{2, 3} {
		name := "session"
		if kind == 3 {
			name = "capability"
		}
		t.Run(name, func(t *testing.T) {
			a := app.Setup(t, false, true, false)
			t.Cleanup(func() { require.NoError(t, a.Close()) })
			ctx := a.GetContextForDeliverTx(nil).WithChainID("hyperpax_125-1").WithBlockHeight(100).
				WithBlockTime(time.Unix(1_800_000_000, 0).UTC())
			a.UpgradeKeeper.SetDone(ctx, app.MergeUpgrade)
			a.RefreshPrecompiles(ctx)
			address := common.HexToAddress(layerxauth.AuthAddress)
			contract, ok := a.EvmKeeper.CustomPrecompiles(ctx)[address]
			require.True(t, ok, "node must serve 0x101C after the merge plan")
			served, ok := contract.(precompiles.IPrecompile)
			require.True(t, ok, "node-served contract must expose its published ABI")
			require.Equal(t, layerxauth.PrecompileName, served.GetName())
			provider, ok := a.GetPrecompileKeepers().(layerxauth.AuthKeepers)
			require.True(t, ok, "node must supply the layerxauth keeper to its served precompile")
			k := provider.LayerXAuthK()
			require.NotNil(t, k)
			require.False(t, k.GetParams(ctx).Paused)
			caller := common.HexToAddress("0x2111010000000000000000000000000000000001")
			private := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
			var public [32]byte
			copy(public[:], private.Public().(ed25519.PublicKey))
			message, err := evmtypes.LayerXBindMessage(a.EvmKeeper.ChainID(ctx), caller, 0)
			require.NoError(t, err)
			var signature [64]byte
			copy(signature[:], ed25519.Sign(private, message))
			_, err = a.EvmKeeper.BindLayerX(ctx, caller, public, signature)
			require.NoError(t, err)
			actor, err := k.Actor(ctx, caller)
			require.NoError(t, err)
			hostKey := a.GetKey(app.LayerXHostStoreKey)
			require.NotNil(t, hostKey)
			host := a.LayerXHost
			require.NotNil(t, host)
			timestamp := uint64(ctx.BlockTime().UnixMilli())
			registration := append(append([]byte{0x71, 1, 0, 2}, actor.Authority.Actor[:]...), public[:]...)
			activity := lxphost.Activity{ProtocolVersion: 3, Type: 0x00070001, ActorDID: actor.DID,
				AuthorityKey: public, NotBefore: timestamp, NotAfter: timestamp,
				IdempotencyKey: [32]byte{0x41}, Payload: registration}
			digest, err := host.SigningHash(activity)
			require.NoError(t, err)
			activity.Signature = ed25519.Sign(private, digest[:])
			receipt, err := host.Execute(lxphost.WithSequence(ctx, 41), activity, actor.Authority)
			require.NoError(t, err)
			require.Equal(t, lxphost.Code(0), receipt.ResultCode)
			db := state.NewDBImpl(ctx.WithEventManager(sdk.NewEventManager()), &a.EvmKeeper, true)
			ctx = db.Ctx()
			evm := &vm.EVM{StateDB: db, TxContext: vm.TxContext{Origin: caller}}
			abi := served.GetABI()
			call := func(name string, args ...interface{}) ([]byte, error) {
				t.Helper()
				input, err := abi.Pack(name, args...)
				require.NoError(t, err)
				return served.Run(evm, caller, address, input, nil, abi.Methods[name].IsConstant(), false, nil)
			}
			delegateCaller := common.HexToAddress("0x2111010000000000000000000000000000000002")
			delegatePrivate := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{72}, ed25519.SeedSize))
			var delegate [32]byte
			copy(delegate[:], delegatePrivate.Public().(ed25519.PublicKey))
			message, err = evmtypes.LayerXBindMessage(a.EvmKeeper.ChainID(ctx), delegateCaller, 0)
			require.NoError(t, err)
			copy(signature[:], ed25519.Sign(delegatePrivate, message))
			_, err = a.EvmKeeper.BindLayerX(ctx, delegateCaller, delegate, signature)
			require.NoError(t, err)
			moduleKey := a.GetKey(storekeys.LayerXAuthStoreKey)
			require.NotNil(t, moduleKey)
			paramsBefore := bytes.Clone(ctx.KVStore(moduleKey).Get(types.ParamsKey))
			require.True(t, ctx.KVStore(moduleKey).Has(types.ParamsKey))
			var storedParams types.Params
			require.NoError(t, a.AppCodec().Unmarshal(paramsBefore, &storedParams))
			require.Equal(t, types.DefaultParams(), storedParams)
			assertParamsOnly := func(stage string) {
				t.Helper()
				iterator := ctx.KVStore(moduleKey).Iterator(nil, nil)
				defer func() { require.NoError(t, iterator.Close()) }()
				var keys [][]byte
				for ; iterator.Valid(); iterator.Next() {
					key := bytes.Clone(iterator.Key())
					require.NotEmpty(t, key, stage)
					require.NotContains(t, []byte{0x02, 0x03}, key[0], "%s: retired idempotency key %x", stage, key)
					require.Equal(t, types.ParamsKey, key, "%s: unexpected module key %x", stage, key)
					require.Equal(t, paramsBefore, iterator.Value(), stage)
					keys = append(keys, key)
				}
				require.Equal(t, [][]byte{{0x01}}, keys, stage)
			}
			assertParamsOnly("before grant")
			body := retirementAuthGrantBody(actor.Authority.Actor, delegate, timestamp, kind)
			method := "grantSession"
			args := []interface{}{body, uint64(100), [32]byte{0x42}}
			if kind == 3 {
				method = "grantCapability"
				args = []interface{}{body}
			}
			data, err := call(method, args...)
			require.NoError(t, err, "%s returned %x", method, data)
			values, err := abi.Methods[method].Outputs.Unpack(data)
			require.NoError(t, err)
			require.Len(t, values, 1)
			grantID := values[0].([32]byte)
			stored, err := k.GetGrant(ctx, grantID)
			require.NoError(t, err)
			require.Equal(t, body, stored)
			sequence, err := lxphost.NextSequence(ctx, hostKey)
			require.NoError(t, err)
			live, reason, err := k.IsLive(ctx, grantID, sequence-1, timestamp)
			require.NoError(t, err)
			require.True(t, live)
			require.Empty(t, reason)
			assertParamsOnly("after grant")
			logCount := len(db.GetAllLogs())
			require.Positive(t, logCount)
			data, err = call("revokeGrant", grantID, uint8(1), sequence)
			require.NoError(t, err)
			require.Empty(t, data)
			live, reason, err = k.IsLive(ctx, grantID, sequence, timestamp)
			require.NoError(t, err)
			require.False(t, live)
			require.Equal(t, "LXP_AUTH_REVOKED", reason)
			revoked, err := host.ReadGrant(ctx, grantID)
			require.NoError(t, err)
			require.True(t, revoked.Revoked)
			require.Equal(t, sequence, revoked.RevokedAtSequence)
			require.Len(t, db.GetAllLogs(), logCount+2)
			require.Equal(t, abi.Events["GrantRevoked"].ID, db.GetAllLogs()[logCount].Topics[0])
			assertParamsOnly("after revoke")
		})
	}
}

func retirementAuthGrantBody(didID, delegate [32]byte, timestamp uint64, kind byte) []byte {
	body := []byte{0, 1, 0x20, 1, 1}
	field := func(value []byte) {
		body = binary.BigEndian.AppendUint32(body, uint32(len(value)))
		body = append(body, value...)
	}
	u64 := func(value uint64) { body = binary.BigEndian.AppendUint64(body, value) }
	u128 := func(value uint64) { u64(0); u64(value) }
	field(didID[:])
	field(didID[:])
	body = append(body, kind)
	field(delegate[:])
	u64(1 << 9)
	body = binary.BigEndian.AppendUint16(body, 4)
	body = binary.BigEndian.AppendUint16(body, 5)
	if kind == 2 {
		field(make([]byte, 32))
		body = append(body, make([]byte, 96)...)
		field(make([]byte, 32))
	} else {
		field(bytes.Repeat([]byte{0x11}, 32))
		u128(10)
		u128(20)
		u128(0)
		u64(1000)
		u128(15)
		u128(0)
		u64(timestamp - 1000)
		field(bytes.Repeat([]byte{0x22}, 32))
	}
	u64(timestamp - 1000)
	u64(timestamp + 1000)
	u64(41)
	body = append(body, 0)
	u64(0)
	field(make([]byte, ed25519.SignatureSize))
	return body
}

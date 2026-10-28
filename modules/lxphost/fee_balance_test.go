//go:build cgo

package lxphost

import (
	"crypto/ed25519"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"
)

func payerFeeAccount(t *testing.T, chain *testChain, actor testActor, balance [16]byte, key [32]byte) [32]byte {
	t.Helper()
	name, id, err := MainAccountID(actor.public)
	require.NoError(t, err)
	field := func(out, value []byte) []byte {
		return append(binary.BigEndian.AppendUint32(out, uint32(len(value))), value...)
	}
	account := field(nil, id[:])
	account = field(account, []byte(name))
	account = append(account, byte(accountKindMain))
	account = append(account, balance[:]...)
	asset := [32]byte{0x91}
	account = field(account, asset[:])
	account = append(account, 1)
	account = binary.BigEndian.AppendUint64(account, 0)
	account = binary.BigEndian.AppendUint64(account, 41)
	account = append(account, 0, 0)
	account = field(account, key[:])
	if key == [32]byte{} {
		account = append(account, 0)
	} else {
		account = append(account, 1)
	}
	chain.context(1, bindingBlockTime, 42).KVStore(chain.key).Set(accountKey(id), account)
	row := parseAccountRow(t, accountKey(id), account)
	require.Equal(t, name, row.name)
	require.Equal(t, uint8(accountKindMain), row.kind)
	return id
}

func TestHostExecuteFeeBalanceFromPayerMainAccount(t *testing.T) {
	chain := newTestChain(t)
	host := chain.host(t)
	actor := newTestActor(t, 81)
	registered := execute(t, host, chain.context(1, bindingBlockTime, 41),
		actor.register(t, host, 0, bindingBlockTime), actor.authority())
	require.Equal(t, Code(0), registered.ResultCode)
	const balance = uint64(1000)
	var amount [16]byte
	binary.BigEndian.PutUint64(amount[8:], balance)
	id := payerFeeAccount(t, chain, actor, amount, actor.public)
	chain.commit()
	require.Equal(t, balance, accountRows(t, chain)[id].balance)
	ctx := chain.context(2, bindingBlockTime, 42)
	before := chain.dump()
	for _, tc := range []struct {
		name  string
		limit uint64
	}{
		{"below_balance", balance - 1},
		{"equal_balance", balance},
		{"above_balance", balance + 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			activity, _ := bindingAssetRegister(t, host, actor, 1)
			binary.BigEndian.PutUint64(activity.FeeLimit[8:], tc.limit)
			hash, err := host.SigningHash(activity)
			require.NoError(t, err)
			activity.Signature = ed25519.Sign(actor.private, hash[:])
			cached, _ := ctx.CacheContext()
			receipt, err := host.Execute(cached, activity, actor.authority())
			if tc.limit > balance {
				require.Equal(t, Code(-602), err)
				require.EqualError(t, err, "LXP_ERR_FEE_UNPAYABLE")
				require.Equal(t, Receipt{}, receipt)
				simulated, err := host.Simulate(ctx, activity, actor.authority())
				require.Equal(t, Code(-602), err)
				require.EqualError(t, err, "LXP_ERR_FEE_UNPAYABLE")
				require.Equal(t, Receipt{}, simulated)
			} else {
				require.NoError(t, err)
				require.NotEqual(t, Code(-602), receipt.ResultCode)
				require.Equal(t, Code(0), receipt.ResultCode)
				simulated, err := host.Simulate(ctx, activity, actor.authority())
				require.NoError(t, err)
				require.Equal(t, receipt.ResultCode, simulated.ResultCode)
			}
			require.Equal(t, before, chain.dump())
		})
	}
	for _, tc := range []struct {
		name string
		key  [32]byte
		code Code
	}{
		{"wrong_account_key", newTestActor(t, 82).public, Code(-201)},
		{"missing_account_key", [32]byte{}, Code(-201)},
		{"missing_account", actor.public, Code(-602)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payerFeeAccount(t, chain, actor, amount, tc.key)
			if tc.name == "missing_account" {
				ctx.KVStore(chain.key).Delete(accountKey(id))
			}
			activity, _ := bindingAssetRegister(t, host, actor, 1)
			activity.FeeLimit[15] = 1
			authority := actor.authority()
			authority.Bound = true
			before := chain.dump()
			cached, _ := ctx.CacheContext()
			_, err := host.Execute(cached, activity, authority)
			require.Equal(t, tc.code, err)
			if tc.code == Code(-201) {
				_, err = host.Simulate(ctx, activity, authority)
				require.Equal(t, tc.code, err)
			}
			if tc.name == "missing_account" {
				_, err = host.Simulate(ctx, activity, authority)
				require.Equal(t, tc.code, err)
			}
			require.Equal(t, before, chain.dump())
		})
	}
}

func TestHostSimulateBindsAuthorityLikeExecute(t *testing.T) {
	chain := newTestChain(t)
	host := chain.host(t)
	actor := newTestActor(t, 83)
	activity, _ := bindingAssetRegister(t, host, actor, 0)
	require.Empty(t, activity.Signature)
	authority := actor.authority()
	authority.Bound = true
	require.True(t, binding_authority(activity, authority))
	ctx := chain.context(1, bindingBlockTime, 41)
	before := chain.dump()
	simulated, err := host.Simulate(ctx, activity, authority)
	require.NoError(t, err)
	require.NotEqual(t, Code(-201), simulated.ResultCode)
	require.Equal(t, before, chain.dump())
	executed, err := host.Execute(ctx, activity, authority)
	require.NoError(t, err)
	require.Equal(t, Code(0), executed.ResultCode)
	require.Equal(t, executed.ResultCode, simulated.ResultCode)
	require.NotEqual(t, Code(-201), executed.ResultCode)
}

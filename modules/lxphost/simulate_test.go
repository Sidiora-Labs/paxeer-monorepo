//go:build cgo

package lxphost

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func simulateFixture(t *testing.T, refused bool) (*testChain, *Host, testActor, Activity, KernelAuthority) {
	t.Helper()
	chain := newTestChain(t)
	host := chain.host(t)
	actor := newTestActor(t, 0x63)
	authority := actor.authority()
	authority.AuthorityHash = sha256.Sum256(actor.public[:])
	require.NotEqual(t, [32]byte{}, authority.AuthorityHash)
	asset := newTestAsset(t, actor, 0x55)
	resolutionSeedMetering(t, chain)
	for i, activity := range []Activity{
		actor.register(t, host, 0, resolutionBlockTime),
		actor.registerAsset(t, host, 1, resolutionBlockTime, asset, 0x55, 1000000000),
		actor.openAssetAccount(t, host, 2, resolutionBlockTime, asset),
		actor.mint(t, host, 3, resolutionBlockTime, asset, 500000000),
	} {
		require.Equal(t, [16]byte{}, activity.FeeLimit)
		receipt := execute(t, host, chain.context(1, resolutionBlockTime, uint64(41+i)), activity, authority)
		require.Equal(t, Code(0), receipt.ResultCode)
	}

	store := chain.context(1, resolutionBlockTime, 45).KVStore(chain.key)
	fee := binary.BigEndian.AppendUint32([]byte("LXFR1"), 1)
	for i := 0; i < 7; i++ {
		fee = binary.BigEndian.AppendUint64(fee, 1)
	}
	fee = append(fee, asset.id[:]...)
	for _, value := range []uint64{1000, 8, 1, 8, 1, 100, 1, 0, 1} {
		fee = binary.BigEndian.AppendUint64(fee, value)
	}
	fee = append(fee, bytes.Repeat([]byte{0xa5}, 32)...)
	fee = append(fee, make([]byte, 16)...)
	require.Len(t, fee, 217)
	store.Set(moduleKVKey(9, []byte("progfee/active/v1")), fee)
	store.Set(moduleKVKey(9, binary.BigEndian.AppendUint32([]byte("progfee/history/v1/"), 1)), fee)

	// Seed the canonical genesis fee account with an empty balance.
	treasury, err := kernelAccountID([]byte("system:fees"))
	require.NoError(t, err)
	field := func(out, value []byte) []byte {
		return append(binary.BigEndian.AppendUint32(out, uint32(len(value))), value...)
	}
	account := field(nil, treasury[:])
	account = field(account, []byte("system:fees"))
	account = append(account, 10)
	account = append(account, make([]byte, 16)...)
	account = field(account, asset.id[:])
	account = append(account, 1)
	account = append(account, make([]byte, 16)...)
	account = append(account, 1, 0)
	account = field(account, make([]byte, 32))
	account = append(account, 0)
	store.Set(accountKey(treasury), account)

	// This Wasm guest exports the reserve and call ABI and returns success.
	wasm := []byte{
		0, 97, 115, 109, 1, 0, 0, 0, 1, 12, 2, 96, 2, 127, 127, 1, 127, 96, 1, 127, 1, 127,
		3, 3, 2, 0, 1, 5, 3, 1, 0, 1, 7, 41, 3,
		11, 'l', 'a', 'y', 'e', 'r', 'x', '_', 'c', 'a', 'l', 'l', 0, 0,
		14, 'l', 'a', 'y', 'e', 'r', 'x', '_', 'r', 'e', 's', 'e', 'r', 'v', 'e', 0, 1,
		6, 'm', 'e', 'm', 'o', 'r', 'y', 2, 0, 10, 11, 2, 4, 0, 65, 0, 11, 4, 0, 65, 0, 11,
	}
	program := repeated(0x71)
	codeHash := sha256.Sum256(wasm)
	iface := append([]byte("LayerX/program-interface/v1\x00"), codeHash[:]...)
	iface = binary.BigEndian.AppendUint16(iface, 1)
	iface = binary.BigEndian.AppendUint16(iface, 1)
	iface = binary.BigEndian.AppendUint16(iface, 11)
	iface = append(iface, "layerx_call"...)
	iface = append(iface, 0, 0, 0, 0, 1, 0x20, 0, 0, 0, 64, 1, 0x20, 0, 0, 0, 64, 0, 0, 0, 0, 0, 0)
	deploy := append([]byte(nil), program[:]...)
	deploy = append(deploy, 0, 1, 1, 0)
	deploy = append(deploy, actor.id[:]...)
	deploy = append(deploy, codeHash[:]...)
	deploy = binary.BigEndian.AppendUint32(deploy, uint32(len(wasm)))
	deploy = binary.BigEndian.AppendUint32(deploy, uint32(len(iface)))
	deploy = append(deploy, iface...)
	deploy = append(deploy, wasm...)
	deployed := execute(t, host, chain.context(1, resolutionBlockTime, 45),
		actor.activity(t, host, resolutionDeployType, 4, resolutionBlockTime, deploy), authority)
	require.Equal(t, Code(0), deployed.ResultCode)
	store.Set(moduleKVKey(9, append([]byte("program-owner\x00"), program[:]...)), append([]byte{1}, actor.id[:]...))

	if refused {
		program = repeated(0x72) // An unknown program is a module refusal.
	}
	access := []byte("LayerX/programs/access-declaration/v1\x00\x00")
	payload := append([]byte(nil), program[:]...)
	payload = binary.BigEndian.AppendUint16(payload, 1)
	payload = binary.BigEndian.AppendUint16(payload, 11)
	payload = binary.BigEndian.AppendUint32(payload, 0)
	payload = binary.BigEndian.AppendUint16(payload, 2)
	payload = binary.BigEndian.AppendUint32(payload, uint32(len(access)))
	payload = binary.BigEndian.AppendUint32(payload, 16)
	for _, budget := range []uint64{1000000, 16777216, 1048576, 1048576, 64, 1048576, 4096} {
		payload = binary.BigEndian.AppendUint64(payload, budget)
	}
	payload = append(payload, "layerx_call"...)
	payload = append(payload, 0, 0)
	payload = append(payload, access...)
	activity := actor.activity(t, host, 0x00090003, 5, resolutionBlockTime, payload)
	require.Equal(t, [16]byte{}, activity.FeeLimit)
	return chain, host, actor, activity, authority
}

func TestSimulateMatchesExecuteReceipt(t *testing.T) {
	for _, tc := range []struct {
		name    string
		refused bool
		bound   bool
	}{
		{name: "zero_fee_budget"},
		{name: "refusal", refused: true},
		{name: "bound_unsigned_zero_fee_budget", bound: true},
		{name: "bound_unsigned_refusal", refused: true, bound: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			chain, host, _, activity, authority := simulateFixture(t, tc.refused)
			if tc.bound {
				activity.Signature = nil
				authority.Bound = true
				require.True(t, binding_authority(activity, authority))
			}
			ctx := chain.context(2, resolutionBlockTime, 46)
			// Execute on a sibling cache gives both calls the same initial state.
			executedCtx, _ := ctx.CacheContext()
			executed, err := host.Execute(executedCtx, activity, authority)
			require.NoError(t, err)
			expectedCode := Code(-207)
			if tc.refused {
				expectedCode = Code(-7)
			}
			require.Equal(t, expectedCode, executed.ResultCode)
			require.NotEmpty(t, executed.Encoded)
			simulated, err := host.Simulate(ctx, activity, authority)
			require.NoError(t, err)
			require.Equal(t, executed.ResultCode, simulated.ResultCode)
			require.Equal(t, executed.PreviousStateRoot, simulated.PreviousStateRoot)
			require.Equal(t, executed.ResultingStateRoot, simulated.ResultingStateRoot)
			require.Equal(t, executed.Encoded, simulated.Encoded)
			require.Equal(t, executed, simulated)
			executedWire, err := codec.DecodeReceipt(executed.Encoded)
			require.NoError(t, err)
			simulatedWire, err := codec.DecodeReceipt(simulated.Encoded)
			require.NoError(t, err)
			require.Equal(t, executedWire.Effects, simulatedWire.Effects)
			require.NotNil(t, simulated.ProgramOutcome)
			require.Equal(t, sdk.ZeroInt(), simulated.FeeCharged)
			require.NotEqual(t, simulated.PreviousStateRoot, simulated.ResultingStateRoot)
		})
	}
}

func TestSimulateLeavesStoreUnchanged(t *testing.T) {
	for _, bound := range []bool{false, true} {
		name := "signed"
		if bound {
			name = "bound_unsigned"
		}
		t.Run(name, func(t *testing.T) {
			chain, host, actor, activity, authority := simulateFixture(t, false)
			if bound {
				activity.Signature = nil
				authority.Bound = true
				require.True(t, binding_authority(activity, authority))
			}
			root := chain.commit()
			before := chain.dump()
			sequence := append([]byte(nil), chain.get([]byte{lastSequenceKey})...)
			actorSequence := append([]byte(nil), chain.get(identitySequenceKey(actor.id))...)
			ctx := chain.context(2, resolutionBlockTime, 46)

			for i := 0; i < 2; i++ {
				receipt, err := host.Simulate(ctx, activity, authority)
				require.Equal(t, before, chain.dump())
				require.Equal(t, sequence, chain.get([]byte{lastSequenceKey}))
				require.Equal(t, actorSequence, chain.get(identitySequenceKey(actor.id)))
				next, sequenceErr := NextSequence(ctx, chain.key)
				require.NoError(t, sequenceErr)
				require.Equal(t, uint64(46), next)
				require.Equal(t, root, chain.commit())
				require.NoError(t, err)
				require.Equal(t, Code(-207), receipt.ResultCode)
			}

			executed := execute(t, host, ctx, activity, authority)
			require.Equal(t, Code(-207), executed.ResultCode)
			require.Equal(t, uint64(46), executed.GlobalSequence)
			require.Equal(t, uint64(46), binary.BigEndian.Uint64(chain.get([]byte{lastSequenceKey})))
			require.NotEqual(t, root, chain.commit())
		})
	}
}

func TestSimulateBindingAuthorityRefusals(t *testing.T) {
	chain, host, actor, signed, authority := simulateFixture(t, false)
	intruder := newTestActor(t, 0x64)
	root := chain.commit()
	before := chain.dump()
	sequence := append([]byte(nil), chain.get([]byte{lastSequenceKey})...)
	actorSequence := append([]byte(nil), chain.get(identitySequenceKey(actor.id))...)
	ctx := chain.context(2, resolutionBlockTime, 46)

	for _, tc := range []struct {
		name   string
		change func(*Activity, *KernelAuthority)
		code   Code
	}{
		{
			name: "unbound_unsigned",
			change: func(activity *Activity, authority *KernelAuthority) {
				authority.Bound = false
			},
			code: Code(-201),
		},
		{
			name: "wrong_verified_key",
			change: func(activity *Activity, authority *KernelAuthority) {
				authority.VerifiedKey = intruder.public
			},
			code: Code(-201),
		},
		{
			name: "wrong_actor",
			change: func(activity *Activity, authority *KernelAuthority) {
				authority.Actor = intruder.id
			},
			code: Code(-204),
		},
		{
			name: "wrong_principal",
			change: func(activity *Activity, authority *KernelAuthority) {
				authority.Principal = intruder.id
			},
			code: Code(-204),
		},
		{
			name: "unregistered_owner_key",
			change: func(activity *Activity, authority *KernelAuthority) {
				activity.AuthorityKey = intruder.public
				authority.VerifiedKey = intruder.public
			},
			code: Code(-204),
		},
		{
			name: "unbound_bad_signature",
			change: func(activity *Activity, authority *KernelAuthority) {
				activity.Signature = append([]byte(nil), signed.Signature...)
				activity.Signature[0] ^= 1
				authority.Bound = false
			},
			code: Code(-201),
		},
		{
			name: "bound_bad_signature",
			change: func(activity *Activity, authority *KernelAuthority) {
				activity.Signature = append([]byte(nil), signed.Signature...)
				activity.Signature[0] ^= 1
			},
			code: Code(-201),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			activity, candidate := signed, authority
			activity.Signature = nil
			candidate.Bound = true
			tc.change(&activity, &candidate)

			executedCtx, _ := ctx.CacheContext()
			executed, executeErr := host.Execute(executedCtx, activity, candidate)
			require.Equal(t, tc.code, executeErr)
			require.Equal(t, Receipt{}, executed)
			simulated, simulateErr := host.Simulate(ctx, activity, candidate)
			require.Equal(t, tc.code, simulateErr)
			require.Equal(t, Receipt{}, simulated)
			require.Equal(t, before, chain.dump())
			require.Equal(t, sequence, chain.get([]byte{lastSequenceKey}))
			require.Equal(t, actorSequence, chain.get(identitySequenceKey(actor.id)))
			next, err := NextSequence(ctx, chain.key)
			require.NoError(t, err)
			require.Equal(t, uint64(46), next)
			require.Equal(t, root, chain.commit())
		})
	}
}

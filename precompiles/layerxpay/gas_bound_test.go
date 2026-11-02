package layerxpay_test

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"math"
	"math/big"
	"sort"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	paytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	lp "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxpay"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPayChargedGasCoversHostStoreWork(t *testing.T) {
	f := newEscrowTestFixture(t)
	h := f.h
	h.keeper.SetParams(h.db.Ctx(), paytypes.Params{EscrowEnabled: true, BudgetEnabled: true, StreamEnabled: true, ServiceEnabled: true})
	a := h.precompile.GetABI()
	seen := make(map[string]bool)
	call := func(t *testing.T, caller common.Address, name string, args ...interface{}) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			input, err := a.Pack(name, args...)
			require.NoError(t, err)
			method, err := a.MethodById(input[:4])
			require.NoError(t, err)
			require.Equal(t, "nonpayable", method.StateMutability)
			row, ok := lp.MethodGas[name]
			require.True(t, ok, "%s must have a gas table entry", name)
			require.True(t, row.Write, "%s must be charged as a write", name)
			decoded, err := method.Inputs.Unpack(input[4:])
			require.NoError(t, err)
			ctx := h.db.Ctx()
			before, err := h.keeper.Host().ReadLastSequence(ctx, f.owner.did)
			require.NoError(t, err)
			rev := h.db.Snapshot()
			meter := sdk.NewGasMeter(math.MaxUint64, 1, 1)
			h.db.WithCtx(h.db.Ctx().WithGasMeter(meter))
			output, err := h.precompile.GetExecutor().Execute(h.db.Ctx(), method, caller, caller, decoded, big.NewInt(0), false, h.evm, nil)
			consumed := meter.GasConsumed()
			h.db.RevertToSnapshot(rev)
			require.NoError(t, err, "%s must execute its host writes", name)
			require.Empty(t, output)
			require.NotZero(t, consumed, "%s must meter real store work", name)
			const supplied uint64 = 2000000
			output, left, err := h.evm.Call(caller, h.precompile.Address(), input, supplied, uint256.NewInt(0))
			require.NoError(t, err, "%s must execute through the served precompile: %x", name, output)
			require.Empty(t, output)
			require.LessOrEqual(t, left, supplied, "%s remaining EVM gas", name)
			after, err := h.keeper.Host().ReadLastSequence(h.db.Ctx(), f.owner.did)
			require.NoError(t, err)
			require.Greater(t, after, before, "%s must commit a fresh host activity", name)
			seen[name] = true
			charged := testkeeper.EVMTestApp.EvmKeeper.GetCosmosGasLimitFromEVMGas(h.db.Ctx(), supplied-left)
			assert.Equal(t, row.Gas(input[4:]), h.precompile.RequiredGas(input), "%s gas table row", name)
			assert.GreaterOrEqual(t, charged, consumed, "%s: charged %d gas, measured %d host store gas", name, charged, consumed)
			t.Logf("%s: charged=%d host_store=%d", name, charged, consumed)
			servedCtx := h.db.Ctx()
			servedCtx.MultiStore().(sdk.CacheMultiStore).Write()
			ctx.EventManager().EmitEvents(servedCtx.EventManager().Events())
			h.db.WithCtx(ctx)
		})
	}
	group := func(name string, run func(*testing.T)) {
		t.Run(name, func(t *testing.T) {
			ctx := h.db.Ctx()
			snapshot := h.db.Snapshot()
			defer func() {
				h.db.RevertToSnapshot(snapshot)
				h.db.WithCtx(ctx)
			}()
			run(t)
		})
	}

	group("escrow", func(t *testing.T) {
		for i, terminal := range []string{"escrowCapture", "escrowRelease", "escrowTimeout", "escrowDisputeResolve"} {
			id := [32]byte{0x40, byte(i)}
			args := f.openArgs(t, id)
			call(t, f.owner.caller, "escrowOpen", args...)
			switch terminal {
			case "escrowCapture":
				call(t, f.beneficiary.caller, "escrowPartialCapture", id, big.NewInt(25), [32]byte{0x41})
				call(t, f.beneficiary.caller, terminal, id, big.NewInt(75), [32]byte{0x42})
			case "escrowRelease":
				call(t, f.owner.caller, terminal, id, [32]byte{0x43})
			case "escrowTimeout":
				h.db.WithCtx(h.db.Ctx().WithBlockTime(time.UnixMilli(int64(args[7].(uint64)))))
				call(t, f.beneficiary.caller, terminal, id, [32]byte{0x44})
			case "escrowDisputeResolve":
				call(t, f.beneficiary.caller, "escrowDisputeOpen", id)
				call(t, f.arbiter.caller, terminal, id, uint32(3333), [32]byte{0x45})
			}
		}
	})

	group("budget", func(t *testing.T) {
		id := [32]byte{0x50}
		args := budgetTestArgs(t, f, id)
		call(t, f.owner.caller, "budgetCreate", args...)
		call(t, f.owner.caller, "budgetFund", id, big.NewInt(50))
		call(t, f.owner.caller, "budgetAmend", id, big.NewInt(280), big.NewInt(30), args[9].(uint64)+60000, uint8(2))
		call(t, f.owner.caller, "budgetDelegateAdd", id, f.beneficiary.actor)
		call(t, f.owner.caller, "budgetSpend", id, f.beneficiary.account, big.NewInt(20))
		r := budgetTestRecord(t, h, id)
		capability := make([]byte, 195)
		capability[1] = 1
		copy(capability[2:34], f.beneficiary.actor[:])
		copy(capability[34:66], f.asset[:])
		copy(capability[66:98], f.beneficiary.account[:])
		copy(capability[98:130], r.PurposeHash[:])
		perSpend, total := (codec.U128{Lo: 25}).Bytes(), (codec.U128{Lo: 100}).Bytes()
		copy(capability[130:146], perSpend[:])
		copy(capability[146:162], total[:])
		binary.BigEndian.PutUint64(capability[178:186], args[9].(uint64))
		capabilityKey := paytypes.KernelKVKey(paytypes.BudgetModuleID, append(append([]byte("budget-delegate:"), id[:]...), f.beneficiary.actor[:]...))
		h.db.Ctx().KVStore(escrowTestHostKey(t, h)).Set(capabilityKey, capability)
		call(t, f.beneficiary.caller, "budgetDelegateSpend", id, f.beneficiary.account, big.NewInt(10))
		grant, _ := budgetTestGrant(t, f, r.PurposeHash)
		escrowTestExecute(t, h, f.owner, 0x00010007, grant, [32]byte{0x51})
		call(t, f.beneficiary.caller, "budgetPull", id, f.beneficiary.account, big.NewInt(30), grant)
		call(t, f.owner.caller, "budgetDelegateRemove", id, f.beneficiary.actor)
		call(t, f.owner.caller, "budgetDefund", id, big.NewInt(30))
		call(t, f.owner.caller, "budgetClose", id, uint64(1))
		id = [32]byte{0x52}
		call(t, f.owner.caller, "budgetCreate", budgetTestArgs(t, f, id)...)
		call(t, f.owner.caller, "budgetSpend", id, f.beneficiary.account, big.NewInt(10))
		call(t, f.owner.caller, "budgetRevoke", id, uint64(3))
	})

	group("native_budget", func(t *testing.T) {
		native := *f
		native.owner = escrowTestRegister(t, h, [32]byte{0xc6}, common.HexToAddress("0xa2284"), true)
		native.beneficiary = escrowTestRegister(t, h, [32]byte{0xc7}, common.HexToAddress("0xa2285"), true)
		salt := [32]byte{0xc0}
		native.asset = sha256.Sum256(append(append([]byte("LX:ASSET:v1"), native.owner.actor[:]...), salt[:]...))
		native.owner.account = escrowTestMain(t, h, native.owner, f.asset)
		native.beneficiary.account = escrowTestMain(t, h, native.beneficiary, native.asset)
		registration := append(append([]byte{0, 1}, native.asset[:]...), salt[:]...)
		registration = append(registration, 1, 'N', 1, 'N', 6)
		registration = binary.BigEndian.AppendUint64(registration, 0)
		registration = binary.BigEndian.AppendUint64(registration, 1000)
		registration = append(registration, 1, 0)
		escrowTestExecute(t, h, native.owner, 0x00010001, registration, [32]byte{0xc4})
		escrowTestExecute(t, h, native.owner, 0x00010004, append([]byte{0, 1}, native.asset[:]...), [32]byte{0xc5})
		sourceName, source, err := lxphost.AssetAccountID(native.owner.public, native.asset)
		require.NoError(t, err)
		mint := append(append([]byte{0, 1}, native.asset[:]...), source[:]...)
		mint = binary.BigEndian.AppendUint64(mint, 0)
		mint = binary.BigEndian.AppendUint64(mint, 400)
		escrowTestExecute(t, h, native.owner, 0x0001000a, mint, [32]byte{0xc1})
		sourceBefore, err := h.keeper.Host().ReadAccount(h.db.Ctx(), sourceName, source)
		require.NoError(t, err)
		id := [32]byte{0xc2}
		args := append(budgetTestArgs(t, &native, id), source, sourceBefore.NextSequence)
		call(t, native.owner.caller, "budgetCreateNative", args...)
		call(t, native.owner.caller, "budgetFundNative", id, big.NewInt(50), sourceBefore.NextSequence+1)
	})

	group("stream", func(t *testing.T) {
		id := [32]byte{0x60}
		args := streamTestArgs(t, f, id, paytypes.StreamModeTime, 100)
		now := int64(args[7].(uint64))
		call(t, f.owner.caller, "streamOpen", args...)
		call(t, f.owner.caller, "streamTopUp", id, big.NewInt(50))
		h.db.WithCtx(h.db.Ctx().WithBlockTime(time.UnixMilli(now + 5000)))
		call(t, f.beneficiary.caller, "streamSettle", id, [32]byte{0x61})
		h.db.WithCtx(h.db.Ctx().WithBlockTime(time.UnixMilli(now + 7500)))
		call(t, f.owner.caller, "streamPause", id)
		h.db.WithCtx(h.db.Ctx().WithBlockTime(time.UnixMilli(now + 17500)))
		call(t, f.owner.caller, "streamResume", id)
		h.db.WithCtx(h.db.Ctx().WithBlockTime(time.UnixMilli(now + 20000)))
		call(t, f.owner.caller, "streamClose", id, [32]byte{0x62})
		id = [32]byte{0x63}
		call(t, f.owner.caller, "streamOpen", streamTestArgs(t, f, id, paytypes.StreamModeMetered, 100)...)
		call(t, f.owner.caller, "streamMeter", id, uint64(15), f.beneficiary.public, streamTestSignature(t, f.beneficiary, id, 15))
	})

	group("service", func(t *testing.T) {
		s := &serviceTestFixture{escrowTestFixture: f, offer: [32]byte{0x81}, agreement: [32]byte{0x82}, commitment: [32]byte{0x83}, escrow: [32]byte{0x84}, terms: [32]byte{0x85}, spec: [32]byte{0x86}}
		s.deadline = uint64(h.db.Ctx().BlockTime().UnixMilli()) + 1000
		args := f.openArgs(t, s.escrow)
		args[7], args[9], args[10] = s.deadline+10000, s.terms, s.agreement
		call(t, f.owner.caller, "escrowOpen", args...)
		offerArgs := []interface{}{s.offer, f.asset, big.NewInt(100), s.terms, s.spec, s.deadline, uint64(1000), uint64(3000), uint8(2), s.deadline}
		call(t, f.beneficiary.caller, "serviceOfferPublish", offerArgs...)
		call(t, f.owner.caller, "serviceAgreementPropose", s.agreement, s.offer, s.terms, s.escrow)
		call(t, f.beneficiary.caller, "serviceAgreementAccept", s.agreement)
		call(t, f.beneficiary.caller, "serviceCommitTask", s.commitment, s.agreement, [32]byte{0x87}, s.escrow, s.deadline, uint64(1000))
		call(t, f.beneficiary.caller, "serviceProgressReport", [32]byte{0x90}, s.commitment, [32]byte{0x92}, [32]byte{0x93}, uint32(5000))
		execution := make([]byte, paytypes.ServiceExecutionBytes)
		for i, field := range [][32]byte{{0xb1}, {0xb2}, s.agreement, s.commitment, {0xb3}, {0xb4}, {0xb5}} {
			copy(execution[32*i:32*(i+1)], field[:])
		}
		now := uint64(h.db.Ctx().BlockTime().UnixMilli())
		binary.BigEndian.PutUint64(execution[224:232], now-100)
		binary.BigEndian.PutUint64(execution[232:240], now)
		binary.BigEndian.PutUint64(execution[240:248], 250)
		copy(execution[248:280], f.beneficiary.account[:])
		availability := [32]byte{0xb6}
		copy(execution[280:312], availability[:])
		copy(execution[312:344], f.beneficiary.public[:])
		digest, err := codec.DomainHash(codec.DomainSignaturePreimage, append([]byte("LXP:SERVICE:ATTEST:v1"), execution[:344]...))
		require.NoError(t, err)
		copy(execution[344:408], ed25519.Sign(f.beneficiary.private, digest[:]))
		call(t, f.beneficiary.caller, "serviceToolExecAttest", execution)
		snapshot := h.db.Snapshot()
		call(t, f.beneficiary.caller, "serviceCommitAbandon", s.commitment, uint16(7))
		h.db.RevertToSnapshot(snapshot)
		items := []serviceTestDeliverable{{s.spec, 4096, [32]byte{0x89}}, {s.spec, 1024, [32]byte{0x8a}}}
		call(t, f.beneficiary.caller, "serviceDeliver", [32]byte{0x88}, s.agreement, items)
		snapshot = h.db.Snapshot()
		call(t, f.owner.caller, "serviceAccept", s.agreement)
		h.db.RevertToSnapshot(snapshot)
		call(t, f.owner.caller, "serviceReject", s.agreement, uint16(7), [][32]byte{s.spec})
		call(t, f.owner.caller, "serviceDisputeOpen", [32]byte{0xa0}, s.agreement, [][32]byte{{0xa2}, {0xa1}})
		call(t, f.beneficiary.caller, "escrowDisputeOpen", s.escrow)
		call(t, f.arbiter.caller, "serviceDisputeResolve", [32]byte{0xa0}, uint16(2), uint32(3333), s.escrow)
		offerArgs[0] = [32]byte{0xb0}
		call(t, f.beneficiary.caller, "serviceOfferPublish", offerArgs...)
		call(t, f.beneficiary.caller, "serviceOfferWithdraw", offerArgs[0])
	})

	var writes []string
	for name, method := range a.Methods {
		if method.StateMutability == "nonpayable" {
			writes = append(writes, name)
		}
	}
	sort.Strings(writes)
	for _, name := range writes {
		assert.True(t, seen[name], "%s needs a successful metered write", name)
	}
	assert.Len(t, seen, len(writes))
}

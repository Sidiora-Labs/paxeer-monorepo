package layerxanchor_test

import (
	"bytes"
	"crypto/ed25519"
	"math/big"
	"testing"
	"time"

	gigaprecompiles "github.com/Sidiora-Labs/Paxeer-X-Network/engine/executor/precompiles"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/testvectors"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	anchorkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor/keeper"
	anchortypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxanchor"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

const (
	bond           = int64(5_000_000)
	unbondingDelay = uint64(1000)
)

var weiPerUnit = big.NewInt(1_000_000_000_000)

type party struct {
	account sdk.AccAddress
	address common.Address
}

type harness struct {
	t          *testing.T
	app        *app.App
	stateDB    *state.DBImpl
	evm        *vm.EVM
	precompile *pcommon.Precompile
	anchor     anchorkeeper.Keeper
	fixture    testvectors.Fixture
	authority  party
	reporter   party
	operators  []party
	ids        [][32]byte
	signers    []common.Address
	reason     string
}

func (h *harness) ctx() sdk.Context { return h.stateDB.Ctx() }

func (h *harness) party(tag byte, associated bool) party {
	h.t.Helper()
	p := party{account: make(sdk.AccAddress, 20)}
	for i := range p.account {
		p.account[i] = tag
	}
	if associated {
		for i := range p.address {
			p.address[i] = tag ^ 0xff
		}
		h.app.EvmKeeper.SetAddressMapping(h.ctx(), p.account, p.address)
	} else {
		p.address = common.BytesToAddress(p.account)
	}
	return p
}

func (h *harness) fund(to sdk.AccAddress, amount int64) {
	h.t.Helper()
	coins := sdk.NewCoins(sdk.NewCoin("uhpx", sdk.NewInt(amount)))
	require.NoError(h.t, h.app.BankKeeper.MintCoins(h.ctx(), "evm", coins))
	require.NoError(h.t, h.app.BankKeeper.SendCoinsFromModuleToAccount(h.ctx(), "evm", to, coins))
}

func (h *harness) balance(of sdk.AccAddress) int64 {
	return h.app.BankKeeper.GetBalance(h.ctx(), of, "uhpx").Amount.Int64()
}

func newHarness(t *testing.T, mutate func(*anchortypes.Params)) *harness {
	t.Helper()
	testApp := app.Setup(t, false, true, false)
	ctx := testApp.GetContextForDeliverTx([]byte{}).WithBlockHeight(8).WithBlockTime(time.Unix(1_800_000_000, 0))
	h := &harness{t: t, app: testApp, anchor: testApp.LayerXAnchorKeeper}
	h.stateDB = state.NewDBImpl(ctx, &testApp.EvmKeeper, true)
	h.evm = &vm.EVM{StateDB: h.stateDB}
	p, err := layerxanchor.NewPrecompile(testApp.GetPrecompileKeepers())
	require.NoError(t, err)
	h.precompile = p
	h.authority, h.reporter = h.party(0xa1, true), h.party(0xa2, false)

	params := anchortypes.DefaultParams(h.authority.account.String())
	require.Equal(t, common.HexToAddress(layerxanchor.LayerXAnchorAddress), common.Address(params.SettlementContract))
	params.Threshold = 2
	params.UnbondingDelaySeconds = unbondingDelay
	if mutate != nil {
		mutate(&params)
	}
	require.NoError(t, h.anchor.SetParams(h.ctx(), params))

	for index := 0; index < 3; index++ {
		id, signer := [32]byte{byte(index + 1)}, common.Address{byte(index + 1)}
		operator := h.party(byte(0xb0+index), true)
		h.fund(operator.account, 2*bond)
		h.ids, h.signers, h.operators = append(h.ids, id), append(h.signers, signer), append(h.operators, operator)
	}
	return h
}

func (h *harness) method(name string) *abi.Method {
	h.t.Helper()
	m, ok := h.precompile.GetABI().Methods[name]
	require.True(h.t, ok, name)
	return &m
}

func (h *harness) input(name string, args ...interface{}) []byte {
	h.t.Helper()
	m := h.method(name)
	packed, err := m.Inputs.Pack(args...)
	require.NoError(h.t, err)
	return append(append([]byte(nil), m.ID...), packed...)
}

// run enters through the EVM precompile entry point as the interpreter does:
// the call value reaches the precompile address first, and a revert restores
// the snapshot taken before the call.
func (h *harness) run(from party, units int64, readOnly bool, name string, args ...interface{}) ([]interface{}, error) {
	h.t.Helper()
	snapshot := h.stateDB.Snapshot()
	var value *big.Int
	if units != 0 {
		value = new(big.Int).Mul(big.NewInt(units), weiPerUnit)
		precompileAccount := h.app.EvmKeeper.GetPaxAddressOrDefault(h.ctx(), h.precompile.Address())
		require.NoError(h.t, h.app.BankKeeper.SendCoins(h.ctx(), from.account, precompileAccount,
			sdk.NewCoins(sdk.NewCoin("uhpx", sdk.NewInt(units)))))
	}
	res, err := h.precompile.Run(h.evm, from.address, from.address, h.input(name, args...), value, readOnly, false, nil)
	if err != nil {
		require.ErrorIs(h.t, err, vm.ErrExecutionReverted)
		reason, unpackErr := abi.UnpackRevert(res)
		require.NoError(h.t, unpackErr)
		require.NotEmpty(h.t, reason)
		h.reason = reason
		h.stateDB.RevertToSnapshot(snapshot)
		return nil, err
	}
	out, err := h.method(name).Outputs.Unpack(res)
	require.NoError(h.t, err)
	return out, nil
}

func (h *harness) call(from party, name string, args ...interface{}) ([]interface{}, error) {
	return h.run(from, 0, false, name, args...)
}

func (h *harness) view(name string, args ...interface{}) []interface{} {
	h.t.Helper()
	out, err := h.run(h.reporter, 0, true, name, args...)
	require.NoError(h.t, err)
	return out
}

func (h *harness) logs(event string) []*ethtypes.Log {
	h.t.Helper()
	id := h.precompile.GetABI().Events[event].ID
	var out []*ethtypes.Log
	for _, log := range h.stateDB.GetAllLogs() {
		if log.Topics[0] == id {
			require.Equal(h.t, h.precompile.Address(), log.Address)
			out = append(out, log)
		}
	}
	return out
}

func (h *harness) invariant() {
	h.t.Helper()
	message, broken := anchorkeeper.BalanceInvariant(h.anchor)(h.ctx())
	require.False(h.t, broken, message)
}

func (h *harness) vector(section, name string) testvectors.Vector {
	h.t.Helper()
	for _, v := range h.fixture[section] {
		if v.Name == name {
			return v
		}
	}
	h.t.Fatalf("no vector %s/%s", section, name)
	return testvectors.Vector{}
}

func (h *harness) checkpointArgs(name string) []interface{} {
	h.t.Helper()
	v := h.vector("checkpoints", name)
	header, err := v.Bytes("header")
	require.NoError(h.t, err)
	signature, err := v.Bytes("header_signature")
	require.NoError(h.t, err)
	certificate, err := v.Bytes("certificate")
	require.NoError(h.t, err)
	return []interface{}{header, signature, certificate}
}

func (h *harness) submit(name string) ([]interface{}, error) {
	return h.call(h.reporter, layerxanchor.SubmitCheckpointMethod, h.checkpointArgs(name)...)
}

func (h *harness) attestation(name string) []byte {
	h.t.Helper()
	raw, err := h.vector("attestations", name).Bytes("attestation")
	require.NoError(h.t, err)
	return raw
}

func (h *harness) advance(seconds uint64) {
	ctx := h.ctx()
	h.stateDB.WithCtx(ctx.WithBlockTime(ctx.BlockTime().Add(time.Duration(seconds) * time.Second)).WithBlockHeight(ctx.BlockHeight() + 1))
}

// bootstrap bonds and activates the three guarantors and authorizes the
// sequencer, all through the precompile.
func (h *harness) bootstrap() {
	h.t.Helper()
	for index := range h.ids {
		_, err := h.run(h.operators[index], bond, false, layerxanchor.RegisterGuarantorMethod, h.ids[index], h.signers[index])
		require.NoError(h.t, err)
		_, err = h.call(h.operators[index], layerxanchor.ActivateGuarantorMethod, h.ids[index])
		require.Error(h.t, err)
		_, err = h.call(h.authority, layerxanchor.ActivateGuarantorMethod, h.ids[index])
		require.NoError(h.t, err)
	}
	sequencerID := [32]byte{1}
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x51}, ed25519.SeedSize))
	var sequencerKey [32]byte
	copy(sequencerKey[:], key.Public().(ed25519.PublicKey))
	var err error
	_, err = h.call(h.reporter, layerxanchor.SetSequencerAuthorizationMethod, sequencerID, sequencerKey, uint64(1), uint64(8))
	require.Error(h.t, err)
	_, err = h.call(h.authority, layerxanchor.SetSequencerAuthorizationMethod, sequencerID, sequencerKey, uint64(1), uint64(8))
	require.NoError(h.t, err)
	require.Len(h.t, h.logs("GuarantorRegistered"), 3)
	require.Len(h.t, h.logs("GuarantorActivated"), 3)
	require.Len(h.t, h.logs("SequencerAuthorized"), 1)
	h.invariant()
}

func TestCastAuthoritySurvivesAssociation(t *testing.T) {
	var deployer, stranger party
	h := newHarness(t, func(p *anchortypes.Params) {
		for i := range deployer.address {
			deployer.address[i], stranger.address[i] = 0xd1, 0xd2
		}
		p.Authority = sdk.AccAddress(deployer.address[:]).String()
	})
	deployer.account, stranger.account = sdk.AccAddress(deployer.address[:]), sdk.AccAddress(stranger.address[:])
	operator := h.operators[0]

	_, err := h.run(operator, bond, false, layerxanchor.RegisterGuarantorMethod, h.ids[0], h.signers[0])
	require.NoError(t, err)
	require.Equal(t, bond, h.balance(operator.account))
	status := func(id [32]byte) *layerxanchor.GuarantorView {
		return abi.ConvertType(h.view(layerxanchor.GuarantorMethod, id)[0], new(layerxanchor.GuarantorView)).(*layerxanchor.GuarantorView)
	}
	require.Equal(t, anchortypes.GuarantorPending, status(h.ids[0]).Status)

	associated := make(sdk.AccAddress, 20)
	for i := range associated {
		associated[i] = 0xd3
	}
	h.app.EvmKeeper.SetAddressMapping(h.ctx(), associated, deployer.address)
	account, ok := h.app.EvmKeeper.GetPaxAddress(h.ctx(), deployer.address)
	require.True(t, ok)
	require.NotEqual(t, deployer.account, account)

	_, err = h.call(stranger, layerxanchor.ActivateGuarantorMethod, h.ids[0])
	require.Error(t, err)
	_, err = h.call(operator, layerxanchor.ActivateGuarantorMethod, h.ids[0])
	require.Error(t, err)
	require.Equal(t, anchortypes.GuarantorPending, status(h.ids[0]).Status)
	_, err = h.call(deployer, layerxanchor.ActivateGuarantorMethod, h.ids[0])
	require.NoError(t, err)
	require.Equal(t, anchortypes.GuarantorActive, status(h.ids[0]).Status)
	require.True(t, status(h.ids[0]).Eligible)
	require.Equal(t, big.NewInt(bond), status(h.ids[0]).Bond)
	sequencer := [32]byte{7}
	_, err = h.call(stranger, layerxanchor.SetSequencerAuthorizationMethod, sequencer, [32]byte{8}, uint64(1), uint64(8))
	require.Error(t, err)
	_, err = h.call(deployer, layerxanchor.SetSequencerAuthorizationMethod, sequencer, [32]byte{8}, uint64(1), uint64(8))
	require.NoError(t, err)
	h.invariant()
}

func TestCallDiscipline(t *testing.T) {
	h := newHarness(t, nil)
	h.bootstrap()
	// State changes are refused from staticcall.
	_, err := h.run(h.reporter, 0, true, layerxanchor.SubmitCheckpointMethod, []byte{1}, make([]byte, 64), []byte{2})
	require.Error(t, err)
	require.Equal(t, anchortypes.CheckpointUnknown, h.view(layerxanchor.StatusOfMethod, uint64(1))[0])
	// Value is refused by every non-payable method.
	h.fund(h.reporter.account, 5)
	_, err = h.run(h.reporter, 1, false, layerxanchor.SubmitCheckpointMethod, []byte{1}, make([]byte, 64), []byte{2})
	require.Error(t, err)
	_, err = h.run(h.reporter, 1, false, layerxanchor.ThresholdMethod)
	require.Error(t, err)
	// Delegatecall is refused.
	_, err = h.precompile.Run(h.evm, h.reporter.address, h.reporter.address, h.input(layerxanchor.ThresholdMethod), nil, true, true, nil)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	// A bond below the minimum and an unassociated operator are refused.
	stranger := h.party(0xd1, true)
	h.fund(stranger.account, bond)
	_, err = h.run(stranger, 999_999, false, layerxanchor.RegisterGuarantorMethod, [32]byte{0xee}, common.Address{0xee})
	require.Error(t, err)
	require.Equal(t, bond, h.balance(stranger.account))
	h.fund(h.reporter.account, bond)
	_, err = h.run(h.reporter, bond, false, layerxanchor.RegisterGuarantorMethod, [32]byte{0xee}, common.Address{0xee})
	require.Error(t, err)
	_, err = h.run(stranger, bond, false, layerxanchor.RegisterGuarantorMethod, h.ids[0], common.Address{0xee})
	require.Error(t, err)
	_, err = h.run(stranger, bond, false, layerxanchor.RegisterGuarantorMethod, [32]byte{0xee}, h.signers[0])
	require.Error(t, err)
	h.invariant()
}

func TestGasFormula(t *testing.T) {
	h := newHarness(t, nil)
	for _, size := range []int{0, 1, 274, 4096} {
		payload := bytes.Repeat([]byte{0xff}, size)
		checkpoint := h.input(layerxanchor.SubmitCheckpointMethod, payload, make([]byte, 64), payload)
		require.Equal(t, layerxanchor.WriteBaseGas, h.precompile.RequiredGas(checkpoint))
		equivocation := h.input(layerxanchor.SubmitEquivocationMethod, payload, payload)
		require.Equal(t, layerxanchor.WriteBaseGas, h.precompile.RequiredGas(equivocation))
	}
	malformed := append([]byte(nil), h.method(layerxanchor.SubmitCheckpointMethod).ID...)
	require.Equal(t, layerxanchor.WriteBaseGas, h.precompile.RequiredGas(malformed))
	availability := h.input(layerxanchor.SubmitAvailabilityMethod, []byte{1})
	require.Equal(t, layerxanchor.Gas(false, uint64(len(availability)-4), 1, 0), h.precompile.RequiredGas(availability))
	view := h.input(layerxanchor.StatusOfMethod, uint64(1))
	require.Equal(t, layerxanchor.ViewBaseGas+layerxanchor.GasPerByte*32, h.precompile.RequiredGas(view))
}

func TestWiring(t *testing.T) {
	address := common.HexToAddress(layerxanchor.LayerXAnchorAddress)
	require.Equal(t, common.HexToAddress("0x0000000000000000000000000000000000001014"), address)
	require.True(t, evmkeeper.IsPayablePrecompile(&address))
	require.Contains(t, gigaprecompiles.AllCustomPrecompilesFailFast, address)
}

func TestRevertCarriesReason(t *testing.T) {
	h := newHarness(t, nil)
	_, err := h.call(h.reporter, layerxanchor.ActivateGuarantorMethod, h.ids[0])
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	require.Contains(t, h.reason, anchortypes.ErrUnauthorized.Error())

	h.fund(h.reporter.account, 5)
	_, err = h.run(h.reporter, 1, false, layerxanchor.ThresholdMethod)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	require.NotEmpty(t, h.reason)

	res, err := h.precompile.Run(h.evm, h.reporter.address, h.reporter.address, []byte{0xde, 0xad, 0xbe, 0xef}, nil, false, false, nil)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	reason, err := abi.UnpackRevert(res)
	require.NoError(t, err)
	require.NotEmpty(t, reason)
}

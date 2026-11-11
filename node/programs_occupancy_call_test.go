package app

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math"
	"math/big"
	"path/filepath"
	"strings"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	programs "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxprograms"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

var programsOccupancyLargestBudget = [7]uint64{1000000, 16777216, 1048576, 1048576, 64, 1048576, 4096}
var programsOccupancyQuotedBudget = [7]uint64{1000000, 16777216, 1048576, 1048576, 64, 1048576, 2}

func programsOccupancyGuest() []byte {
	section := func(id byte, body []byte) []byte { return append([]byte{id, byte(len(body))}, body...) }
	label := func(s string) []byte { return append([]byte{byte(len(s))}, s...) }
	guest := []byte{0, 97, 115, 109, 1, 0, 0, 0}
	guest = append(guest, section(1, []byte{3, 0x60, 5, 0x7f, 0x7f, 0x7f, 0x7f, 0x7f, 1, 0x7f, 0x60, 1, 0x7f, 1, 0x7f, 0x60, 2, 0x7f, 0x7f, 1, 0x7f})...)
	imports := append([]byte{1}, label("layerx_v2")...)
	imports = append(imports, label("storage_write_scoped")...)
	guest = append(guest, section(2, append(imports, 0, 0))...)
	guest = append(guest, section(3, []byte{2, 1, 2})...)
	guest = append(guest, section(5, []byte{1, 1, 1, 1})...)
	exports := append([]byte{3}, label("layerx_reserve")...)
	exports = append(exports, 0, 1)
	exports = append(exports, label("layerx_call")...)
	exports = append(exports, 0, 2)
	exports = append(exports, label("memory")...)
	guest = append(guest, section(7, append(exports, 2, 0))...)
	call := []byte{0, 0x41, 2, 0x20, 0, 0x41, 1, 0x41, 1, 0x41, 1, 0x10, 0, 0x1a, 0x41, 0, 0x0b}
	code := append([]byte{2, 4, 0, 0x41, 0, 0x0b, byte(len(call))}, call...)
	guest = append(guest, section(10, code)...)
	guest = append(guest, section(11, []byte{1, 0, 0x41, 0, 0x0b, 2, 'k', 'v'})...)
	return guest
}

func programsOccupancyFixture(t *testing.T) *hostedProgramsABIFixture {
	t.Helper()
	f := newHostedProgramsABIFixture(t, 1)
	programsOccupancyUpgradeGuest(t, f)
	programsBlockEndPrepareMerge(t, f)
	return f
}

func programsOccupancyUpgradeGuest(t *testing.T, f *hostedProgramsABIFixture) {
	t.Helper()
	section := func(id byte, body []byte) []byte { return append([]byte{id, byte(len(body))}, body...) }
	label := func(s string) []byte { return append([]byte{byte(len(s))}, s...) }
	previous := []byte{0, 97, 115, 109, 1, 0, 0, 0}
	previous = append(previous, section(1, []byte{2, 0x60, 1, 0x7f, 1, 0x7f, 0x60, 2, 0x7f, 0x7f, 1, 0x7f})...)
	previous = append(previous, section(3, []byte{2, 0, 1})...)
	previous = append(previous, section(5, []byte{1, 1, 1, 1})...)
	exports := append(append([]byte{3}, label("layerx_reserve")...), 0, 0)
	exports = append(append(exports, label("layerx_call")...), 0, 1)
	exports = append(append(exports, label("memory")...), 2, 0)
	previous = append(previous, section(7, exports)...)
	previous = append(previous, section(10, []byte{2, 4, 0, 0x41, 0, 0x0b, 4, 0, 0x41, 0, 0x0b})...)
	guest := programsOccupancyGuest()
	upgrade, err := programs.EncodeUpgradePayload(f.program, 5, 0,
		sha256.Sum256(previous), sha256.Sum256(guest), nil, nil, guest)
	require.NoError(t, err)
	require.Zero(t, f.execute(t, 0x00090002, upgrade).ResultCode)
}

func programsOccupancyQuote(t *testing.T, f *hostedProgramsABIFixture, budget [7]uint64) *big.Int {
	t.Helper()
	address := programs.Address
	served := f.app.EvmKeeper.CustomPrecompiles(f.ctx)
	contract, ok := served[address].(precompiles.IPrecompile)
	require.True(t, ok)
	definition := contract.GetABI()
	input, err := definition.Pack("quoteCallFee", budget)
	require.NoError(t, err)
	db := state.NewDBImpl(f.ctx, &f.app.EvmKeeper, false)
	block, err := f.app.EvmKeeper.GetVMBlockContext(f.ctx, core.GasPool(math.MaxUint64))
	require.NoError(t, err)
	chain := evmtypes.DefaultChainConfig().EthereumConfig(f.app.EvmKeeper.ChainID(f.ctx))
	evm := vm.NewEVM(*block, db, chain, vm.Config{}, served)
	output, _, err := evm.StaticCall(common.HexToAddress("0xa111"), address, input, 2000000)
	require.NoError(t, err, "%v", db.GetPrecompileError())
	values, err := definition.Methods["quoteCallFee"].Outputs.Unpack(output)
	require.NoError(t, err)
	require.Equal(t, uint32(1), values[0])
	require.Positive(t, values[1].(*big.Int).Sign())
	return values[1].(*big.Int)
}

func programsOccupancyExecute(t *testing.T, f *hostedProgramsABIFixture, key byte, budget [7]uint64, limit *big.Int) lxphost.Receipt {
	t.Helper()
	payload, err := programs.EncodeCallPayload(f.program, 5, "layerx_call", []byte{key},
		[]byte{0, 1, 8}, []byte("LayerX/programs/access-declaration/v1\x00\x00"), 16, budget)
	require.NoError(t, err)
	sequence, err := f.app.LayerXHost.ReadIdentitySequence(f.ctx, f.did)
	require.NoError(t, err)
	next, err := lxphost.NextSequence(f.ctx, f.app.GetKey(LayerXHostStoreKey))
	require.NoError(t, err)
	timestamp := uint64(f.ctx.BlockTime().UnixMilli())
	material := binary.BigEndian.AppendUint32(binary.BigEndian.AppendUint64(nil, next), 0x00090003)
	activity := lxphost.Activity{
		ProtocolVersion: codec.StateCommitmentProtocolVersion, Type: 0x00090003,
		ActorDID: f.did, AuthorityKey: f.authority.VerifiedKey,
		AccountSequence: sequence, NotBefore: timestamp, NotAfter: timestamp,
		IdempotencyKey: sha256.Sum256(material), Payload: payload,
	}
	limit.FillBytes(activity.FeeLimit[:])
	hash, err := f.app.LayerXHost.SigningHash(activity)
	require.NoError(t, err)
	activity.Signature = ed25519.Sign(f.private, hash[:])
	receipt, err := f.app.LayerXHost.Execute(lxphost.WithSequence(f.ctx, next), activity, f.authority)
	require.NoError(t, err)
	return receipt
}

func programsOccupancyCallBlock(t *testing.T, f *hostedProgramsABIFixture, height int64,
	actor *hostedProgramsABIFixture, key byte, budget [7]uint64, limit *big.Int, want lxphost.Code) {
	t.Helper()
	f.ctx = f.ctx.WithBlockHeight(height)
	req := programsBlockEndRequest(f, height)
	before, err := lxphost.NextSequence(f.ctx, f.app.GetKey(LayerXHostStoreKey))
	require.NoError(t, err)
	proposal, err := f.app.ProcessProposal(context.Background(), &abci.RequestProcessProposal{Hash: req.Hash, Header: req.Header})
	require.NoError(t, err)
	require.Equal(t, abci.ResponseProcessProposal_ACCEPT, proposal.Status)
	info := f.app.GetOptimisticProcessingInfo()
	require.NotNil(t, info.Completion)
	select {
	case <-info.Completion:
	case <-time.After(30 * time.Second):
		t.Fatal("block processing did not finish")
	}
	info = f.app.GetOptimisticProcessingInfo()
	require.False(t, info.Aborted)
	f.ctx = info.StateCtx.WithBlockHeader(*req.Header)
	actor.ctx = f.ctx
	actor.app = f.app
	rowsBefore := programsOccupancyCallRows(f)
	receipt := programsOccupancyExecute(t, actor, key, budget, limit)
	require.Equal(t, want, receipt.ResultCode, "%s, outcome: %+v", receipt.ResultCode.Error(), receipt.ProgramOutcome)
	if want == 0 {
		require.NotNil(t, receipt.ProgramOutcome)
		require.Zero(t, receipt.ProgramOutcome.Status)
	} else {
		require.Equal(t, rowsBefore, programsOccupancyCallRows(f))
	}
	_, err = f.app.FinalizeBlock(context.Background(), req)
	require.NoError(t, err)
	programsBlockEndCommit(t, f)
	after, err := lxphost.NextSequence(f.app.GetCheckCtx(), f.app.GetKey(LayerXHostStoreKey))
	require.NoError(t, err)
	require.Equal(t, before+2, after)
}

func programsOccupancyRows(f *hostedProgramsABIFixture) map[string][]byte {
	return programsOccupancySelectedRows(f, false)
}

func programsOccupancyCallRows(f *hostedProgramsABIFixture) map[string][]byte {
	return programsOccupancySelectedRows(f, true)
}

func programsOccupancySelectedRows(f *hostedProgramsABIFixture, includeOccupancy bool) map[string][]byte {
	rows := make(map[string][]byte)
	iter := f.ctx.KVStore(f.app.GetKey(LayerXHostStoreKey)).Iterator(nil, nil)
	for ; iter.Valid(); iter.Next() {
		if strings.Contains(string(iter.Key()), "progstor") ||
			(includeOccupancy && strings.Contains(string(iter.Key()), "progocc")) {
			rows[string(iter.Key())] = bytes.Clone(iter.Value())
		}
	}
	_ = iter.Close()
	return rows
}

func programsOccupancyOtherOwner(t *testing.T, f *hostedProgramsABIFixture) [32]byte {
	t.Helper()
	other := &hostedProgramsABIFixture{app: f.app, ctx: f.ctx, program: [32]byte{0x77}, proposer: f.proposer}
	other.private = ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x74}, ed25519.SeedSize))
	copy(other.authority.VerifiedKey[:], other.private.Public().(ed25519.PublicKey))
	other.did = []byte("did:layerx:" + hex.EncodeToString(other.authority.VerifiedKey[:]))
	actor, err := lxphost.DIDID(other.did)
	require.NoError(t, err)
	other.authority.Actor, other.authority.Principal = actor, actor
	other.authority.AuthorityHash = sha256.Sum256(other.authority.VerifiedKey[:])
	register := append([]byte{0x71, 1, 0, 2}, actor[:]...)
	register = append(register, other.authority.VerifiedKey[:]...)
	require.Zero(t, other.execute(t, 0x00070001, register).ResultCode)
	guest := programsOccupancyGuest()
	hash := sha256.Sum256(guest)
	deploy, err := programs.EncodeDeployPayload(other.program, 2, 1, actor, hash, nil, guest)
	require.NoError(t, err)
	require.Zero(t, other.execute(t, 0x00090001, deploy).ResultCode)
	upgrade, err := programs.EncodeUpgradePayload(other.program, 5, 0, hash, hash, nil, nil, guest)
	require.NoError(t, err)
	require.Zero(t, other.execute(t, 0x00090002, upgrade).ResultCode)
	return other.program
}

func TestCallAddingBytesAtTheQuotedFeeCommitsAndBlocksApply(t *testing.T) {
	f := programsOccupancyFixture(t)
	budget := programsOccupancyQuotedBudget
	limit := programsOccupancyQuote(t, f, budget)
	before := programsOccupancyRows(f)
	programsOccupancyCallBlock(t, f, mergeHeight, f, 'a', budget, limit, 0)
	require.NotEqual(t, before, programsOccupancyRows(f))
	programsBlockEndIdleBlocks(t, f, mergeHeight+1, 3)
}

func TestCallAtTheLargestBudgetCommitsAndBlocksApply(t *testing.T) {
	f := programsOccupancyFixture(t)
	budget := programsOccupancyLargestBudget
	programsOccupancyCallBlock(t, f, mergeHeight, f, 'a', budget, programsOccupancyQuote(t, f, budget), 0)
	programsBlockEndIdleBlocks(t, f, mergeHeight+1, 3)
}

func TestCallShortOfTheOccupancyResponsibilityReverts(t *testing.T) {
	f := programsOccupancyFixture(t)
	budget := programsOccupancyQuotedBudget
	limit := new(big.Int).Sub(programsOccupancyQuote(t, f, budget), new(big.Int).SetUint64(budget[6]))
	before := programsOccupancyRows(f)
	programsOccupancyCallBlock(t, f, mergeHeight, f, 'a', budget, limit, lxphost.Code(-207))
	require.Equal(t, before, programsOccupancyRows(f))
	programsBlockEndIdleBlocks(t, f, mergeHeight+1, 3)
}

func TestCallByAPayerThatIsNotTheOwnerReverts(t *testing.T) {
	f := newHostedProgramsABIFixture(t, 1)
	programsOccupancyUpgradeGuest(t, f)
	f.program = programsOccupancyOtherOwner(t, f)
	programsBlockEndPrepareMerge(t, f)
	budget := programsOccupancyQuotedBudget
	before := programsOccupancyRows(f)
	programsOccupancyCallBlock(t, f, mergeHeight, f, 'a', budget, programsOccupancyQuote(t, f, budget), lxphost.Code(-207))
	require.Equal(t, before, programsOccupancyRows(f))
	programsBlockEndIdleBlocks(t, f, mergeHeight+1, 3)
}

func TestBlocksApplyAcrossRestartAfterOccupancyCalls(t *testing.T) {
	f := programsOccupancyFixture(t)
	budget := programsOccupancyQuotedBudget
	quote := programsOccupancyQuote(t, f, budget)
	programsOccupancyCallBlock(t, f, mergeHeight, f, 'a', budget, quote, 0)
	programsOccupancyCallBlock(t, f, mergeHeight+1, f, 'b', budget,
		new(big.Int).Sub(programsOccupancyQuote(t, f, budget), new(big.Int).SetUint64(budget[6])), lxphost.Code(-207))
	stored := programsOccupancyRows(f)
	last, err := lxphost.NextSequence(f.app.GetCheckCtx(), f.app.GetKey(LayerXHostStoreKey))
	require.NoError(t, err)
	info, err := f.app.UpgradeKeeper.GetUpgradeInfoPath()
	require.NoError(t, err)
	home := filepath.Dir(filepath.Dir(info))
	require.NoError(t, f.app.Close())
	f.app = New(dbm.NewMemDB(), nil, true, map[int64]bool{}, home, 1, true, config.TestConfig(),
		MakeEncodingConfig(), wasm.EnableAllProposals, TestAppOpts{}, EmptyWasmOpts, nil)
	f.ctx = f.app.NewUncachedContext(false, tmproto.Header{Height: mergeHeight + 2, Time: f.ctx.BlockTime(), ChainID: f.ctx.ChainID()})
	require.Equal(t, stored, programsOccupancyRows(f))
	restored, err := lxphost.NextSequence(f.ctx, f.app.GetKey(LayerXHostStoreKey))
	require.NoError(t, err)
	require.Equal(t, last, restored)
	programsBlockEndIdleBlocks(t, f, mergeHeight+2, 7)
}

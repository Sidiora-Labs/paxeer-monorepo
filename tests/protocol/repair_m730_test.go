package protocol_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/config"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	sdked25519 "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/protocol/internal/qualification"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

func clockTree730(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, name := range qualification.ClockRoots {
		directory := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(directory, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(directory, "transition.c"), []byte("int transition(void) { return 0; }\n"), 0644))
	}
	return root
}

func TestRepairM_730_ClockPrerequisites(t *testing.T) {
	t.Run("exhaustive_clean_inputs", func(t *testing.T) {
		require.NoError(t, qualification.ScanClockSources(clockTree730(t)))
	})
	t.Run("unavailable_rg", func(t *testing.T) {
		_, err := qualification.FindRG(t.TempDir())
		require.ErrorContains(t, err, "requires an executable rg")
		_, err = qualification.FindRG("")
		require.Error(t, err)
	})
	t.Run("unexecutable_rg", func(t *testing.T) {
		directory := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(directory, "rg"), nil, 0600))
		_, err := qualification.FindRG(directory)
		require.Error(t, err)
	})
	for _, name := range qualification.ClockRoots {
		t.Run("missing_"+name, func(t *testing.T) {
			root := clockTree730(t)
			require.NoError(t, os.RemoveAll(filepath.Join(root, name)))
			require.ErrorContains(t, qualification.ScanClockSources(root), name)
		})
	}
	for _, fixture := range []string{"unreadable_root", "unsearchable_root", "unreadable_nested", "unreadable_source", "file_as_root", "source_symlink"} {
		t.Run(fixture, func(t *testing.T) {
			root := clockTree730(t)
			directory := filepath.Join(root, qualification.ClockRoots[0])
			source := filepath.Join(directory, "transition.c")
			switch fixture {
			case "unreadable_root":
				require.NoError(t, os.Chmod(directory, 0111))
				t.Cleanup(func() { require.NoError(t, os.Chmod(directory, 0755)) })
			case "unsearchable_root":
				require.NoError(t, os.Chmod(directory, 0444))
				t.Cleanup(func() { require.NoError(t, os.Chmod(directory, 0755)) })
			case "unreadable_nested":
				nested := filepath.Join(directory, "nested")
				require.NoError(t, os.Mkdir(nested, 0000))
				t.Cleanup(func() { require.NoError(t, os.Chmod(nested, 0755)) })
			case "unreadable_source":
				require.NoError(t, os.Chmod(source, 0000))
			case "file_as_root":
				require.NoError(t, os.RemoveAll(directory))
				require.NoError(t, os.WriteFile(directory, []byte("not a directory"), 0644))
			case "source_symlink":
				require.NoError(t, os.Symlink(source, filepath.Join(directory, "linked.c")))
			}
			require.Error(t, qualification.ScanClockSources(root))
		})
	}
}

func TestRepairM_730_ClockScanResults(t *testing.T) {
	require.NoError(t, qualification.CheckRGExit(1, ""))
	for _, status := range []int{0, 2, 126, 127, -1} {
		require.Error(t, qualification.CheckRGExit(status, "scan diagnostic"), "rg status %d must refuse", status)
	}
	require.Error(t, qualification.CheckRGExit(1, "an input could not be read"))
	for _, forbidden := range []string{"time(NULL)", "gettimeofday (&now, 0)", "clock_gettime(CLOCK_REALTIME, &now)", "rand()", "# include < time.h >"} {
		t.Run(forbidden, func(t *testing.T) {
			root := clockTree730(t)
			path := filepath.Join(root, qualification.ClockRoots[3], ".hidden", "clock.c")
			require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
			require.NoError(t, os.WriteFile(path, []byte(forbidden), 0644))
			require.ErrorContains(t, qualification.ScanClockSources(root), "forbidden")
		})
	}
	root := clockTree730(t)
	path := filepath.Join(root, qualification.ClockRoots[0], "transition.c")
	require.NoError(t, os.WriteFile(path, []byte("int runtime(void); int my_rand(void); int clock_gettime_ns(void);"), 0644))
	require.NoError(t, qualification.ScanClockSources(root))
}

const policyRevision730 = "264615cba89456c37eeb19535cb16b81846f6a68"
const policyChain730 = "pax-test"
const policyPrice730 = int64(500_000_000_000)

// Every observation below is read from the retained App, FeeFloor, Executor,
// sequencer keeper and committed EVM receipt. No RPC or executable is involved.
type policyHarness730 struct {
	app       *app.App
	exec      *sequencer.FeeFloor
	sender    utils.TestAcct
	to        common.Address
	contract  common.Address
	timestamp time.Time
	key       ed25519.PrivateKey
	keyID     [32]byte
}

func newPolicyHarness730(t *testing.T, sender utils.TestAcct, recipient common.Address, funded bool) *policyHarness730 {
	t.Helper()
	tm := time.Unix(1700000000, 0).UTC()
	validator := sdked25519.GenPrivKeyFromSecret([]byte("repair m730 retained node"))
	wrapper := app.NewTestWrapper(t, tm, validator.PubKey(), false, func(base *baseapp.BaseApp) {
		base.SetOccEnabled(false)
		base.SetConcurrencyWorkers(1)
	})
	a := wrapper.App
	params := *app.DefaultConsensusParams
	params.Block = &tmproto.BlockParams{MaxBytes: params.Block.MaxBytes, MaxGas: params.Block.MaxGas, MaxGasWanted: params.Block.MaxGas}
	a.StoreConsensusParams(wrapper.Ctx, &params)
	a.EvmKeeper.SetAddressMapping(wrapper.Ctx, sender.AccountAddress, sender.EvmAddress)
	if funded {
		wrapper.FundAcc(sender.AccountAddress, sdk.NewCoins(sdk.NewCoin("uhpx", sdk.NewInt(1_000_000_000_000))))
	}
	contract := common.HexToAddress("0x730")
	// Store first, then revert: a failure receipt must prove both storage and
	// value-transfer rollback while retaining the fee and sender nonce.
	a.EvmKeeper.SetCode(wrapper.Ctx, contract, common.FromHex("0x600160005560006000fd"))
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x73}, ed25519.SeedSize))
	keyID := [32]byte{0x73}
	require.NoError(t, a.SequencerKeeper.SetReceiptKeys(wrapper.Ctx, seqkeeper.ReceiptKeys{{
		KeyId: keyID[:], PublicKey: key.Public().(ed25519.PublicKey), FirstHeight: 1, LastHeight: math.MaxUint64,
	}}))
	require.NoError(t, a.SequencerKeeper.SetActivationHeight(wrapper.Ctx, 2))
	a.UpgradeKeeper.SetDone(wrapper.Ctx.WithBlockHeight(1), app.MergeUpgrade)
	_, err := a.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{Header: &tmproto.Header{ChainID: policyChain730, Height: 1}})
	require.NoError(t, err)
	a.SetDeliverStateToCommit()
	_, err = a.Commit(context.Background())
	require.NoError(t, err)
	exec, err := sequencer.NewExecutor(a, sequencer.Options{ChainID: policyChain730, ProposerAddress: validator.PubKey().Address().Bytes()})
	require.NoError(t, err)
	floor, err := sequencer.NewFeeFloor(exec, a, "0.01uhpx", a.ParamsKeeper, a.OracleKeeper, a.PricefeedKeeper, &a.EvmKeeper)
	require.NoError(t, err)
	require.NoError(t, floor.Open(2, tm.Add(time.Second).UnixMilli()))
	return &policyHarness730{a, floor, sender, recipient, contract, tm.Add(time.Second), key, keyID}
}

func (h *policyHarness730) snapshot(t *testing.T) sdk.Context {
	t.Helper()
	ctx, err := h.exec.Snapshot()
	require.NoError(t, err)
	return ctx
}

func (h *policyHarness730) observe(ctx sdk.Context, sequence uint64) qualification.PolicyState {
	balance := func(address common.Address) string {
		return h.app.EvmKeeper.GetBalance(ctx, h.app.EvmKeeper.GetPaxAddressOrDefault(ctx, address)).String()
	}
	storage := h.app.EvmKeeper.GetState(ctx, h.contract, common.Hash{})
	return qualification.PolicyState{Nonce: h.app.EvmKeeper.GetNonce(ctx, h.sender.EvmAddress), Sequence: sequence,
		Payer: balance(h.sender.EvmAddress), Recipient: balance(h.to), Storage: hex.EncodeToString(storage[:])}
}

func policyTransaction730(t *testing.T, sender utils.TestAcct, to common.Address, chain int64, nonce uint64, price int64, gas uint64, invalidSignature bool) (*ethtypes.Transaction, []byte) {
	t.Helper()
	signer := ethtypes.LatestSignerForChainID(big.NewInt(chain))
	unsigned := ethtypes.NewTx(&ethtypes.LegacyTx{Nonce: nonce, GasPrice: big.NewInt(price), Gas: gas, To: &to, Value: big.NewInt(1)})
	var signed *ethtypes.Transaction
	var err error
	if invalidSignature {
		signed, err = unsigned.WithSignature(signer, make([]byte, 65))
	} else {
		signed, err = ethtypes.SignTx(unsigned, signer, sender.EvmPrivateKey)
	}
	require.NoError(t, err)
	data, err := ethtx.NewTxDataFromTx(signed)
	require.NoError(t, err)
	msg, err := evmtypes.NewMsgEVMTransaction(data)
	require.NoError(t, err)
	config := app.MakeEncodingConfig().TxConfig
	builder := config.NewTxBuilder()
	require.NoError(t, builder.SetMsgs(msg))
	builder.SetGasLimit(gas)
	encoded, err := config.TxEncoder()(builder.GetTx())
	require.NoError(t, err)
	return signed, encoded
}

func (h *policyHarness730) commit(t *testing.T) (*sequencer.Batch, sdk.Context, *abci.ResponseFinalizeBlock) {
	t.Helper()
	batch, err := h.exec.Seal(2)
	require.NoError(t, err)
	fields := sequencer.BatchFields(batch)
	msg := seqtypes.MsgSequenceBatch{Height: fields.Height, FromSeq: fields.FromSeq, ToSeq: fields.ToSeq,
		PrevSeqHash: fields.PrevSeqHash[:], LastSeqHash: fields.LastSeqHash[:], TxRoot: fields.TxRoot[:], KeyId: h.keyID[:]}
	digest, err := msg.SignBytes(policyChain730)
	require.NoError(t, err)
	msg.Sig = ed25519.Sign(h.key, digest[:])
	tx0, err := app.EncodeSequenceBatch(msg)
	require.NoError(t, err)
	txs := append([][]byte{tx0}, batch.Txs...)
	// FinalizeBlock replays the real encoded batch and signed transaction bytes
	// through normal consensus execution, then Commit persists the result.
	result, err := h.app.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{
		Txs: txs, Header: &tmproto.Header{ChainID: policyChain730, Height: 2, Time: h.timestamp},
	})
	require.NoError(t, err)
	require.Len(t, result.TxResults, len(txs))
	require.Zero(t, result.TxResults[0].Code, result.TxResults[0].Log)
	h.app.SetDeliverStateToCommit()
	_, err = h.app.Commit(context.Background())
	require.NoError(t, err)
	ctx := sdk.NewContext(h.app.CommitMultiStore().CacheMultiStore(), tmproto.Header{ChainID: policyChain730, Height: 2}, false)
	return batch, ctx, result
}

func producePolicyCase730(t *testing.T, name string, sender utils.TestAcct, to common.Address) qualification.PolicyCase {
	t.Helper()
	h := newPolicyHarness730(t, sender, to, name != "fee_unpayable")
	nonce, chain, price, gas := uint64(0), int64(config.DefaultChainID), policyPrice730, uint64(21000)
	if name == "sequence_gap" {
		nonce = 1
	}
	if name == "wrong_network" {
		chain++
	}
	if name == "fee_below_floor" {
		price = 1
	}
	if name == "execution_failure" {
		h.to = h.contract
		gas = 100000
	}
	signed, tx := policyTransaction730(t, sender, h.to, chain, nonce, price, gas, name == "bad_signature")
	if name == "malformed" {
		tx = []byte{0xff}
	}
	var lastSequence uint64
	if name == "sequence_reused" {
		accepted, err := h.exec.Accept(tx)
		require.NoError(t, err)
		require.Zero(t, accepted.Result.Code, accepted.Result.Log)
		lastSequence = accepted.Seq
	}
	row := qualification.PolicyCase{Name: name, Transaction: hex.EncodeToString(tx), Before: h.observe(h.snapshot(t), lastSequence), GasPrice: big.NewInt(price).String(), GasLimit: gas, Value: "1"}
	accepted, err := h.exec.Accept(tx)
	switch name {
	case "success", "execution_failure", "historical_replay":
		require.NoError(t, err)
		require.NotNil(t, accepted)
		row.Admitted, row.Code, row.GasUsed = true, accepted.Result.Code, uint64(accepted.Result.GasUsed)
		lastSequence = accepted.Seq
		receipt, receiptErr := h.app.EvmKeeper.GetTransientReceipt(h.snapshot(t), signed.Hash(), uint64(accepted.TxIndex))
		require.NoError(t, receiptErr)
		status := receipt.Status
		row.ReceiptStatus = &status
		require.Equal(t, row.GasUsed, receipt.GasUsed)
		require.Equal(t, uint64(price), receipt.EffectiveGasPrice)
		if name == "execution_failure" {
			require.Equal(t, uint32(ethtypes.ReceiptStatusFailed), receipt.Status)
			require.Contains(t, accepted.Result.EvmTxInfo.VmError, "revert")
		} else {
			require.Equal(t, uint32(ethtypes.ReceiptStatusSuccessful), receipt.Status)
		}
	default:
		require.Error(t, err)
		require.Nil(t, accepted)
		_, row.Code, _ = sdkerrors.ABCIInfo(err, false)
		require.NotZero(t, row.Code)
		switch name {
		case "sequence_gap", "sequence_reused":
			require.ErrorIs(t, err, sdkerrors.ErrWrongSequence)
		case "wrong_network":
			require.ErrorIs(t, err, sdkerrors.ErrInvalidChainID)
		case "fee_unpayable":
			require.ErrorIs(t, err, sdkerrors.ErrInsufficientFunds)
		}
	}
	row.After = h.observe(h.snapshot(t), lastSequence)
	batch, committed, results := h.commit(t)
	require.Equal(t, lastSequence, batch.LastSeq)
	sequence, err := h.app.SequencerKeeper.LastSeq(committed)
	require.NoError(t, err)
	row.Committed = h.observe(committed, sequence)
	if row.Admitted {
		require.Len(t, batch.Txs, 1)
		require.Equal(t, tx, batch.Txs[0])
		require.Equal(t, accepted.Result, results.TxResults[1])
		receipt, err := h.app.EvmKeeper.GetReceipt(committed, signed.Hash())
		require.NoError(t, err)
		require.Equal(t, *row.ReceiptStatus, receipt.Status)
		row.CommittedCode = results.TxResults[1].Code
		row.CommittedGasUsed = receipt.GasUsed
	} else if name == "sequence_reused" {
		require.Len(t, batch.Txs, 1, "replay refusal must not add a second transaction or receipt")
	} else {
		require.Empty(t, batch.Txs, "refused input must not reach the committed batch")
	}
	return row
}

func TestRepairM_730_RetainedNodePolicy(t *testing.T) {
	previousOCC := app.EnableOCC
	app.EnableOCC = false
	t.Cleanup(func() { app.EnableOCC = previousOCC })
	sender := utils.NewSigner()
	to := common.HexToAddress("0x730001")
	evidence := qualification.PolicyEvidence{Schema: qualification.EvidenceSchema, Revision: policyRevision730}
	for _, name := range qualification.PolicyCases {
		t.Run(name, func(t *testing.T) {
			evidence.Cases = append(evidence.Cases, producePolicyCase730(t, name, sender, to))
		})
	}
	require.Len(t, evidence.Cases, len(qualification.PolicyCases))
	encoded, err := json.Marshal(evidence)
	require.NoError(t, err)
	decoded, err := qualification.ReadPolicyEvidence(bytes.NewReader(encoded), policyRevision730)
	require.NoError(t, err)
	require.Equal(t, evidence, *decoded)
	// Persist the real producer document alongside the private gate log. This
	// is evidence for this focused run, not a claim of a separate CLI gate.
	require.NoError(t, os.WriteFile("/tmp/repair-m730-node-evidence.json", encoded, 0600))
	for _, mutation := range []string{"missing", "duplicate", "wrong_revision", "refusal_nonce", "refusal_fee", "refusal_receipt", "execution_fee", "fee_limit", "failure_effect", "commit_drift", "receipt_status", "replay_bytes", "replay_gas", "unknown_field", "trailing_document"} {
		t.Run("reject_"+mutation, func(t *testing.T) {
			var changed qualification.PolicyEvidence
			require.NoError(t, json.Unmarshal(encoded, &changed))
			switch mutation {
			case "missing":
				changed.Cases = changed.Cases[1:]
			case "duplicate":
				changed.Cases[1] = changed.Cases[0]
			case "wrong_revision":
				changed.Revision = strings.Repeat("0", 40)
			case "refusal_nonce":
				changed.Cases[0].After.Nonce++
			case "refusal_fee":
				changed.Cases[0].After.Payer = "1"
			case "refusal_receipt":
				status := uint32(0)
				changed.Cases[0].ReceiptStatus = &status
			case "execution_fee":
				changed.Cases[7].GasPrice = "1"
			case "fee_limit":
				changed.Cases[7].GasLimit = changed.Cases[7].GasUsed - 1
			case "failure_effect":
				changed.Cases[8].After.Storage = strings.Repeat("01", 32)
			case "commit_drift":
				changed.Cases[7].Committed.Nonce++
			case "receipt_status":
				*changed.Cases[8].ReceiptStatus = 1
			case "replay_bytes":
				changed.Cases[9].Transaction = "00"
			case "replay_gas":
				changed.Cases[9].GasUsed++
			}
			invalid, err := json.Marshal(changed)
			require.NoError(t, err)
			if mutation == "unknown_field" {
				invalid = append([]byte(`{"unverified":true,`), invalid[1:]...)
			}
			if mutation == "trailing_document" {
				invalid = append(invalid, []byte(" {}")...)
			}
			_, err = qualification.ReadPolicyEvidence(bytes.NewReader(invalid), policyRevision730)
			require.Error(t, err)
		})
	}
}

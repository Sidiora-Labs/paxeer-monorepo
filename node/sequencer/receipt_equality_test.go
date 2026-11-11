package sequencer_test

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmed25519 "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/config"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	bridgetypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxbridge/types"
	ledgertypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	paykeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/keeper"
	paytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxpay"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	sdked25519 "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	sdksecp256k1 "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

const (
	receiptModuleID = uint16(12)
	createGas       = 200_000
	callGas         = 100_000
	sidFeeGas       = 30_000
	sidConverted    = 32_697
	sidFunding      = 1_000_000
	vectorHeight    = int64(26_400_000)
)

var (
	// logRuntime emits LOG2(0x2a word; 1, 2), LOG4(0x2a 00 00 00 00; 3, 4, 5, 6) and LOG0().
	logRuntime = mustHex("602a600052" + "6002600160206000a2" + "600660056004600360056" + "01fa4" + "60006000a0" + "00")
	logInit    = append(mustHex("6021600c60003960216000f3"), logRuntime...)
	// errorX is the ABI encoding of Error("x").
	errorX = mustHex("08c379a0" +
		"0000000000000000000000000000000000000000000000000000000000000020" +
		"0000000000000000000000000000000000000000000000000000000000000001" +
		"7800000000000000000000000000000000000000000000000000000000000000")
	revertInit    = append(mustHex("6064600c60003960646000fd"), errorX...)
	sponsorAddr   = common.BytesToAddress(repeat(0x5a, 20))
	sponsorToken  = common.BytesToAddress(repeat(0x7b, 20))
	sponsoredSig  = crypto.Keccak256Hash([]byte("Sponsored(address,address,uint256,uint256)"))
	sponsoredInit = mustHex("73" + hex.EncodeToString(sponsorToken[:]) + "73" + hex.EncodeToString(sponsorAddr[:]) +
		"7f" + hex.EncodeToString(sponsoredSig[:]) + "60406000a3" + "60006000f3")
	testKeySeed      = label("sequencer receipt test key")
	consensusKeySeed = label("sequencer receipt test consensus key")
	testKeyID        = [32]byte(label("sequencer receipt test key id"))
)

func mustHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

func label(s string) []byte { h := sha256.Sum256([]byte(s)); return h[:] }

func repeat(b byte, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = b
	}
	return out
}

func addr32(a common.Address) [32]byte {
	var w [32]byte
	copy(w[12:], a[:])
	return w
}

func receiptWord(b ...byte) [32]byte {
	var w [32]byte
	copy(w[32-len(b):], b)
	return w
}

// wireVector reads a shared receipt vector from the layerx-wire crate.
func wireVector(t *testing.T, name string) []byte {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("..", "..", "services", "agent", "crates", "layerx-wire", "src", "receipt.rs"))
	require.NoError(t, err)
	m := regexp.MustCompile(`const ` + name + `: &str =\s*"([0-9a-f]+)"`).FindSubmatch(src)
	require.NotNil(t, m, name)
	return mustHex(string(m[1]))
}

func writeSeed(t *testing.T, dir, name string, seed []byte, mode os.FileMode) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(hex.EncodeToString(seed)+"\n"), mode))
	require.NoError(t, os.Chmod(path, mode))
	return path
}

func writeConsensusKey(t *testing.T, dir string, seed []byte) (string, []byte) {
	t.Helper()
	priv, err := tmed25519.SecretKeyFromSecretBytes(ed25519.NewKeyFromSeed(seed))
	require.NoError(t, err)
	path := filepath.Join(dir, "priv_validator_key.json")
	pv := privval.NewFilePV(priv, path, filepath.Join(dir, "priv_validator_state.json"))
	require.NoError(t, pv.Key.Save())
	return path, pv.Key.PubKey.Bytes()
}

func publicOf(seed []byte) [32]byte {
	return [32]byte(ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey))
}

// assertReceiptKeySignature exercises the receipt consumer with a loaded key.
func assertReceiptKeySignature(t *testing.T, key *sequencer.ReceiptKey, in *sequencer.ReceiptInput) []byte {
	t.Helper()
	signed, err := key.SignReceipt(in)
	require.NoError(t, err)
	verified, err := verify.ReceiptSignature(signed, key.PublicKey())
	require.NoError(t, err)
	unsigned, err := sequencer.BuildReceipt(in, verified.Receipt.ContextHash)
	require.NoError(t, err)
	require.Equal(t, codec.ReceiptDigest(unsigned), verified.Digest)
	return signed
}

func loadKey(t *testing.T, seed []byte, pins []verify.SequencerAuthorization, height int64) (*sequencer.ReceiptKey, error) {
	t.Helper()
	dir := t.TempDir()
	consensus, _ := writeConsensusKey(t, dir, consensusKeySeed)
	return sequencer.LoadReceiptKey(writeSeed(t, dir, "receipt_key", seed, 0o600), consensus, pins, height)
}

func vectorKey(t *testing.T) *sequencer.ReceiptKey {
	t.Helper()
	key, err := loadKey(t, label("paxeer receipt vector key seed"), []verify.SequencerAuthorization{{
		SequencerID:      [32]byte(repeat(0x33, 32)),
		PublicKey:        [32]byte(wireVector(t, "PINNED_RECEIPT_KEY")),
		FirstBatchNumber: 1,
		LastBatchNumber:  30_000_000,
	}}, vectorHeight)
	require.NoError(t, err)
	return key
}

func bigInt(s string) *big.Int {
	v, ok := new(big.Int).SetString(s, 10)
	if !ok {
		panic(s)
	}
	return v
}

// vectorInput holds the fields of the shared layerx-wire receipt vector.
func vectorInput() *sequencer.ReceiptInput {
	prev := [32]byte(label("paxeer receipt vector previous sequence"))
	activity := [32]byte(label("paxeer receipt vector transaction"))
	transfer := receiptWord(label("Transfer(address,address,uint256)")...)
	from := common.BytesToAddress(repeat(0xaa, 20))
	to := common.BytesToAddress(repeat(0xbb, 20))
	holder := addr32(from)
	return &sequencer.ReceiptInput{
		Accepted: &sequencer.Accepted{
			Seq:         7,
			PrevSeqHash: prev,
			SeqHash:     expectedSeqHash(prev, 7, activity),
			ActivityID:  activity,
			Height:      vectorHeight,
			TxIndex:     2,
			TimestampMs: 1_750_000_000_250,
		},
		EVM: &evmtypes.Receipt{
			Status:            1,
			GasUsed:           51234,
			CumulativeGasUsed: 73456,
			EffectiveGasPrice: 1_000_000_000,
			To:                to.Hex(),
			Logs: []*evmtypes.Log{
				{Address: common.BytesToAddress(repeat(0x11, 20)).Hex(), Topics: []string{common.Hash(transfer).Hex(), common.Hash(holder).Hex()}, Data: append(make([]byte, 31), 0x2a)},
				{Address: common.BytesToAddress(repeat(0x22, 20)).Hex()},
			},
		},
		Operation:         2,
		ReturnData:        append(make([]byte, 31), 1),
		PaymentDenom:      "upax",
		FeeDenom:          "upax",
		FeeCharged:        bigInt("51234000000000"),
		Sponsor:           sponsorAddr,
		Amount:            bigInt("1000000000000000000"),
		From:              from,
		FromBalanceBefore: bigInt("5000000000000000000"),
		FromBalanceAfter:  bigInt("3999948766000000000"),
		FromSequence:      41,
		To:                to,
		ToBalanceBefore:   bigInt("2000000000000000000"),
		ToBalanceAfter:    bigInt("3000000000000000000"),
		ModuleID:          receiptModuleID,
	}
}

type receiptHarness struct {
	*harness
	key      *sequencer.ReceiptKey
	r        *sequencer.Receipter
	openMs   int64
	accepted int
}

// newReceiptHarness builds the App as newHarness does, runs setup on the
// genesis context before height 1 commits, continues the sequence chain from
// its activation value (seq 0, zero prevSeqHash), opens height 2 at openMs (or
// one second after genesis when zero) and signs with the test key pinned for
// heights 1..1000.
func newReceiptHarness(t *testing.T, openMs int64, setup func(ctx sdk.Context, a *app.App), funded ...utils.TestAcct) *receiptHarness {
	t.Helper()
	return newReceiptHarnessWithPrecompiles(t, openMs, false, setup, nil, funded...)
}

func newReceiptHarnessWithPrecompiles(t *testing.T, openMs int64, enabled bool, setup, afterFinalize func(ctx sdk.Context, a *app.App), funded ...utils.TestAcct) *receiptHarness {
	t.Helper()
	tm := time.Now().UTC()
	valPub := sdked25519.GenPrivKey().PubKey()
	if enabled {
		tm = time.Unix(1_700_000_000, 0).UTC()
		valPub = (&sdked25519.PrivKey{Key: ed25519.NewKeyFromSeed(label("restart validator"))}).PubKey()
	}
	tw := app.NewTestWrapper(t, tm, valPub, enabled)
	a := tw.App
	require.NoError(t, a.SequencerKeeper.SetLeaders(tw.Ctx, [][32]byte{[32]byte(valPub.Bytes())}))
	public := publicOf(testKeySeed)
	require.NoError(t, a.SequencerKeeper.SetReceiptKeys(tw.Ctx, seqkeeper.ReceiptKeys{{
		KeyId: testKeyID[:], PublicKey: public[:], FirstHeight: 1, LastHeight: 1000,
	}}))
	for _, acct := range funded {
		a.EvmKeeper.SetAddressMapping(tw.Ctx, acct.AccountAddress, acct.EvmAddress)
		tw.FundAcc(acct.AccountAddress, sdk.NewCoins(sdk.NewCoin("uhpx", sdk.NewInt(fundingUhpx))))
	}
	if setup != nil {
		setup(tw.Ctx, a)
	}
	_, err := a.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{
		Header: &tmproto.Header{ChainID: testChainID, Height: committedTip},
	})
	require.NoError(t, err)
	if afterFinalize != nil {
		blockMs := openMs
		if blockMs == 0 {
			blockMs = tm.Add(time.Second).UnixMilli()
		}
		ctx := a.GetContextForDeliverTx(nil).WithChainID(testChainID).
			WithBlockHeight(openHeight).WithBlockTime(time.UnixMilli(blockMs).UTC())
		setupCtx, write := ctx.CacheContext()
		afterFinalize(setupCtx, a)
		write()
	}
	a.SetDeliverStateToCommit()
	_, err = a.Commit(context.Background())
	require.NoError(t, err)
	proposer := valPub.Address().Bytes()
	exec, err := sequencer.NewExecutor(a, sequencer.Options{ChainID: testChainID, ProposerAddress: proposer})
	require.NoError(t, err)
	h := &harness{t: t, app: a, exec: exec, proposer: proposer, genesis: tm}
	if openMs == 0 {
		openMs = h.open()
	} else {
		require.NoError(t, h.exec.Open(openHeight, openMs))
	}
	key, err := loadKey(t, testKeySeed, []verify.SequencerAuthorization{{
		SequencerID: testKeyID, PublicKey: publicOf(testKeySeed), FirstBatchNumber: 1, LastBatchNumber: 1000,
	}}, openHeight)
	require.NoError(t, err)
	r, err := sequencer.NewReceipter(h.exec, h.app, &h.app.EvmKeeper, key, receiptModuleID)
	require.NoError(t, err)
	return &receiptHarness{harness: h, key: key, r: r, openMs: openMs}
}

func (rh *receiptHarness) accept(t *testing.T, bz []byte) (*sequencer.Accepted, *codec.Receipt, []byte) {
	t.Helper()
	acc, signed, err := rh.r.Accept(bz)
	require.NoError(t, err)
	rh.accepted++
	verified, err := verify.ReceiptSignature(signed, rh.key.PublicKey())
	require.NoError(t, err)
	return acc, verified.Receipt, signed
}

func (rh *receiptHarness) transient(t *testing.T, etx *ethtypes.Transaction, acc *sequencer.Accepted) *evmtypes.Receipt {
	t.Helper()
	r, err := rh.app.EvmKeeper.GetTransientReceipt(rh.openCtx(), etx.Hash(), uint64(acc.TxIndex))
	require.NoError(t, err)
	return r
}

func evmTx(t *testing.T, from utils.TestAcct, data ethtypes.TxData) (*ethtypes.Transaction, []byte) {
	t.Helper()
	signed, err := ethtypes.SignNewTx(from.EvmPrivateKey, ethtypes.LatestSignerForChainID(big.NewInt(config.DefaultChainID)), data)
	require.NoError(t, err)
	txData, err := ethtx.NewTxDataFromTx(signed)
	require.NoError(t, err)
	msg, err := evmtypes.NewMsgEVMTransaction(txData)
	require.NoError(t, err)
	txConfig := app.MakeEncodingConfig().TxConfig
	builder := txConfig.NewTxBuilder()
	require.NoError(t, builder.SetMsgs(msg))
	builder.SetGasLimit(signed.Gas())
	bz, err := txConfig.TxEncoder()(builder.GetTx())
	require.NoError(t, err)
	return signed, bz
}

func receiptLegacyTx(nonce uint64, to *common.Address, value *big.Int, gas uint64, data []byte) *ethtypes.LegacyTx {
	return &ethtypes.LegacyTx{Nonce: nonce, GasPrice: big.NewInt(gasPriceWei), Gas: gas, To: to, Value: value, Data: data}
}

func dynamicTx(nonce uint64, to *common.Address, value *big.Int, gas uint64, data []byte) *ethtypes.DynamicFeeTx {
	return &ethtypes.DynamicFeeTx{
		ChainID: big.NewInt(config.DefaultChainID), Nonce: nonce, GasTipCap: big.NewInt(gasPriceWei),
		GasFeeCap: big.NewInt(gasPriceWei), Gas: gas, To: to, Value: value, Data: data,
	}
}

func summaryBody(gasUsed, cumulative, price uint64, txIndex uint32, contract, sponsor common.Address, vmError string, returnLen, logs uint32, denom string) []byte {
	b := binary.BigEndian.AppendUint64(nil, gasUsed)
	b = binary.BigEndian.AppendUint64(b, cumulative)
	b = binary.BigEndian.AppendUint64(b, price)
	b = binary.BigEndian.AppendUint32(b, txIndex)
	b = append(b, contract[:]...)
	b = append(b, sponsor[:]...)
	vm := sha256.Sum256([]byte(vmError))
	b = append(b, vm[:]...)
	b = binary.BigEndian.AppendUint32(b, returnLen)
	b = binary.BigEndian.AppendUint32(b, logs)
	b = append(b, uint8(len(denom)))
	return append(b, denom...)
}

func logBody(addr common.Address, data []byte, topics ...[32]byte) []byte {
	b := append([]byte(nil), addr[:]...)
	b = append(b, uint8(len(topics)))
	for _, tp := range topics {
		b = append(b, tp[:]...)
	}
	d := sha256.Sum256(data)
	return append(b, d[:]...)
}

func u128(v *big.Int) codec.U128 {
	var b [16]byte
	v.FillBytes(b[:])
	return codec.U128{Hi: binary.BigEndian.Uint64(b[:8]), Lo: binary.BigEndian.Uint64(b[8:])}
}

func TestReceiptHeaderAndOrder(t *testing.T) {
	signed, err := vectorKey(t).SignReceipt(vectorInput())
	require.NoError(t, err)
	require.Equal(t, hex.EncodeToString(wireVector(t, "GO_SIGNED_RECEIPT")), hex.EncodeToString(signed))
	require.Equal(t, []byte{0x00, 0x02, 0x52, 0x01, 0x00, 0x02}, signed[:6])

	a, b := utils.NewSigner(), utils.NewSigner()
	rh := newReceiptHarness(t, 0, nil, a, b)
	_, bz := transfer(t, a, 0, b.EvmAddress, big.NewInt(1))
	_, rec, raw := rh.accept(t, bz)
	require.Equal(t, []byte{0x00, 0x02, 0x52, 0x01, 0x00, 0x02}, raw[:6])
	require.Equal(t, uint16(2), rec.ProtocolVersion)
	require.Nil(t, rec.Supply)
	require.Nil(t, rec.ProgramOutcome)
	require.NotNil(t, rec.SequencerSignature)
	require.Equal(t, raw, rec.CanonicalBytes())
}

func TestReceiptActivityIdIsTxHash(t *testing.T) {
	a, b := utils.NewSigner(), utils.NewSigner()
	rh := newReceiptHarness(t, 0, nil, a, b)
	etx, bz := transfer(t, a, 0, b.EvmAddress, big.NewInt(1))
	acc, rec, _ := rh.accept(t, bz)
	require.Equal(t, [32]byte(etx.Hash()), rec.ActivityID)
	require.Equal(t, acc.ActivityID, rec.ActivityID)

	cosmos := signCosmos(t, rh.openCtx(), rh.harness, a, 200_000,
		banktypes.NewMsgSend(a.AccountAddress, b.AccountAddress, sdk.NewCoins(sdk.NewCoin("uhpx", sdk.NewInt(5)))))
	_, rec, _ = rh.accept(t, cosmos)
	require.Equal(t, sha256.Sum256(cosmos), rec.ActivityID)
	require.Equal(t, sequencer.CosmosOperation, rec.Operation)

	in := vectorInput()
	in.Accepted.ActivityID = [32]byte{}
	in.Accepted.SeqHash = expectedSeqHash(in.Accepted.PrevSeqHash, in.Accepted.Seq, [32]byte{})
	_, err := sequencer.BuildReceipt(in, testKeyID)
	require.ErrorIs(t, err, sequencer.ErrReceiptInput)
}

func TestCurrentReceiptRestartFixture(t *testing.T) {
	signer := func(name string) utils.TestAcct {
		private := &sdksecp256k1.PrivKey{Key: label(name)}
		evmKey, err := crypto.ToECDSA(private.Bytes())
		require.NoError(t, err)
		return utils.TestAcct{PrivateKey: private, PublicKey: private.PubKey(),
			AccountAddress: sdk.AccAddress(private.PubKey().Address()),
			EvmPrivateKey:  evmKey, EvmAddress: crypto.PubkeyToAddress(evmKey.PublicKey)}
	}
	sender, recipient := signer("restart sender"), signer("restart recipient")
	var envelopes [][]byte
	var capture map[string]interface{}
	budgetID := [32]byte(repeat(0x51, 32))
	var budgetRecipient [32]byte
	rh := newReceiptHarnessWithPrecompiles(t, 0, true, func(ctx sdk.Context, a *app.App) {
		a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(committedTip), app.MergeUpgrade)
	}, func(ctx sdk.Context, a *app.App) {
		budgetRecipient = setupCurrentReceiptRestartBudget(t, ctx, a, sender, recipient, budgetID)
	}, sender)
	value := big.NewInt(25)
	payABI, err := layerxpay.GetABI()
	require.NoError(t, err)
	calldata, err := payABI.Pack("budgetSpend", budgetID, budgetRecipient, value)
	require.NoError(t, err)
	pay := common.HexToAddress(layerxpay.PayAddress)
	require.Contains(t, rh.app.EvmKeeper.CustomPrecompiles(rh.openCtx()), pay)
	require.NoError(t, rh.app.LayerXPayKeeper.CheckWrite(rh.openCtx(), "budgetSpend"),
		"committed restart setup must enable the budget module in the executing block")
	initialBudget, found, err := rh.app.LayerXPayKeeper.GetBudget(rh.openCtx(), budgetID)
	require.NoError(t, err)
	require.True(t, found, "committed restart budget must be visible in the executing block")
	require.Zero(t, initialBudget.SpentThisPeriod)

	for nonce, call := range []struct {
		to    common.Address
		data  []byte
		value *big.Int
	}{
		{to: recipient.EvmAddress, value: value},
		{to: pay, data: calldata, value: new(big.Int)},
	} {
		transaction, envelope := evmTx(t, sender, dynamicTx(uint64(nonce), &call.to, call.value, 500_000, call.data))
		envelopes = append(envelopes, envelope)
		raw, err := transaction.MarshalBinary()
		require.NoError(t, err)
		var decoded ethtypes.Transaction
		require.NoError(t, decoded.UnmarshalBinary(raw))
		require.Equal(t, ethtypes.DynamicFeeTxType, int(decoded.Type()))
		require.Equal(t, transaction.Hash(), crypto.Keccak256Hash(raw))
		require.Equal(t, transaction.Hash(), decoded.Hash())
		require.NoError(t, rh.app.LayerXPayKeeper.CheckWrite(rh.openCtx(), "budgetSpend"),
			"restart budget enablement must survive accepted transactions")
		accepted, receipt, signed := rh.accept(t, envelope)
		t.Logf("restart capture: hash=%s transaction_value=%s requested_amount=%s receipt_amount=%v result_code=%d", decoded.Hash(), decoded.Value(), value, receipt.Amount, receipt.ResultCode)
		if accepted.Result.Code != abci.CodeTypeOK {
			var result sdk.TxMsgData
			require.NoError(t, result.Unmarshal(accepted.Result.Data))
			require.Len(t, result.Data, 1)
			var response evmtypes.MsgEVMTransactionResponse
			require.NoError(t, response.Unmarshal(result.Data[0].Data))
			reason, err := abi.UnpackRevert(response.ReturnData)
			require.NoError(t, err)
			t.Logf("producer refused nonzero restart capture: to=%s value=%s code=%d vm_error=%s revert=%s", call.to, decoded.Value(), accepted.Result.Code, response.VmError, reason)
		}
		require.Equal(t, abci.CodeTypeOK, accepted.Result.Code)
		require.Equal(t, [32]byte(decoded.Hash()), accepted.ActivityID)
		require.Equal(t, accepted.ActivityID, receipt.ActivityID)
		require.Equal(t, codec.U128{Lo: decoded.Value().Uint64()}, receipt.Amount)
		require.Equal(t, signed, receipt.CanonicalBytes())
		executed := rh.transient(t, transaction, accepted)
		require.Zero(t, receipt.ResultCode,
			"nonzero restart capture requires successful execution: to=%s value=%s vm_error=%s; Receipter.beforeEVM signs tx.Value(), but layerxpay.Executor.Execute calls ValidateNonPayable before budgetSpend",
			call.to, decoded.Value(), executed.VmError)
		if call.to == pay {
			budget, found, err := rh.app.LayerXPayKeeper.GetBudget(rh.openCtx(), budgetID)
			require.NoError(t, err)
			require.True(t, found, "restart capture requires producer-derived budget state")
			t.Logf("restart budget producer: spent=%v receipt_amount=%v transaction_value=%s", budget.SpentThisPeriod, receipt.Amount, decoded.Value())
			require.Equal(t, u128(value), budget.SpentThisPeriod)
			require.Zero(t, decoded.Value().Sign(), "budgetSpend is non-payable")
			require.Zero(t, receipt.Amount, "Receipter.beforeEVM records tx.Value(), not the calldata spend")
			var result sdk.TxMsgData
			require.NoError(t, result.Unmarshal(accepted.Result.Data))
			require.Len(t, result.Data, 1)
			var response evmtypes.MsgEVMTransactionResponse
			require.NoError(t, response.Unmarshal(result.Data[0].Data))
			body := receipt.Effects[0].Body
			public := rh.key.PublicKey()
			fee := receipt.FeeCharged.Bytes()
			view := &evmrpc.SequencerReceiptResult{
				Rung: evmrpc.RungReceipted, TxHash: decoded.Hash(), Nonce: hexutil.Uint64(decoded.Nonce()),
				SequencerReceiptView: &evmrpc.SequencerReceiptView{
					Seq: hexutil.Uint64(receipt.GlobalSequence), Height: hexutil.Uint64(accepted.Height),
					Index: hexutil.Uint(accepted.TxIndex), Timestamp: hexutil.Uint64(receipt.Timestamp),
					Status: hexutil.Uint64(executed.Status), ReturnData: hexutil.Bytes(response.ReturnData),
					Logs:    make([]evmrpc.SequencerReceiptLog, 0, len(executed.Logs)),
					GasUsed: hexutil.Uint64(executed.GasUsed), CumulativeGasUsed: hexutil.Uint64(binary.BigEndian.Uint64(body[8:16])),
					EffectiveGasPrice: hexutil.Uint64(executed.EffectiveGasPrice), PaymentDenom: "uhpx", FeeDenom: string(body[109:]),
					FeeAmount: (*hexutil.Big)(new(big.Int).SetBytes(fee[:])), Sponsor: common.BytesToAddress(body[48:68]),
					From: sender.EvmAddress, To: pay, ContractAddress: common.BytesToAddress(body[28:48]),
					VMError: executed.VmError, KeyID: common.Hash(receipt.ContextHash), SigningKey: public[:], Receipt: signed,
				},
			}
			for i, log := range executed.Logs {
				topics := make([]common.Hash, len(log.Topics))
				for j, topic := range log.Topics {
					topics[j] = common.HexToHash(topic)
				}
				view.Logs = append(view.Logs, evmrpc.SequencerReceiptLog{
					Address: common.HexToAddress(log.Address), Topics: topics, Data: log.Data, LogIndex: hexutil.Uint(i),
				})
			}
			pins, err := rh.app.SequencerKeeper.ReceiptKeys(rh.openCtx())
			require.NoError(t, err)
			require.Len(t, pins, 1)
			keys := make(seqkeeper.ReceiptKeys, len(pins))
			for i, pin := range pins {
				keys[i] = seqkeeper.ReceiptKeyEntry{KeyId: pin.KeyId[:], PublicKey: pin.PublicKey[:], FirstHeight: pin.FirstHeight, LastHeight: pin.LastHeight}
			}
			require.NoError(t, view.Verify(keys))
			capture = map[string]interface{}{
				"signed_transaction": hexutil.Encode(raw), "transaction_value": decoded.Value().String(),
				"receipt_amount": new(big.Int).SetBytes(sliceU128(receipt.Amount)).String(), "budget_spent": new(big.Int).SetBytes(sliceU128(budget.SpentThisPeriod)).String(), "budget_id": hexutil.Encode(budgetID[:]),
				"receipt_view": view,
				"receipt_keys": map[string]interface{}{"keys": []interface{}{map[string]interface{}{
					"keyId": hexutil.Encode(pins[0].KeyId[:]), "publicKey": hexutil.Encode(pins[0].PublicKey[:]),
					"firstHeight": pins[0].FirstHeight, "lastHeight": pins[0].LastHeight,
				}}},
			}
		} else {
			requireWei(t, value, decoded.Value())
			require.Positive(t, decoded.Value().Sign())
		}
	}
	response, err := rh.app.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{
		Txs: envelopes, Header: &tmproto.Header{ChainID: testChainID, Height: openHeight, Time: time.UnixMilli(rh.openMs).UTC()},
	})
	require.NoError(t, err)
	require.Len(t, response.TxResults, len(envelopes))
	for _, result := range response.TxResults {
		require.Equal(t, abci.CodeTypeOK, result.Code)
	}
	rh.app.SetDeliverStateToCommit()
	_, err = rh.app.Commit(context.Background())
	require.NoError(t, err)
	committed := sdk.NewContext(rh.app.CommitMultiStore().CacheMultiStore(), tmproto.Header{
		ChainID: testChainID, Height: openHeight, Time: time.UnixMilli(rh.openMs).UTC()}, false)
	budget, found, err := rh.app.LayerXPayKeeper.GetBudget(committed, budgetID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, u128(value), budget.SpentThisPeriod)
	capture["budget_spent"] = new(big.Int).SetBytes(sliceU128(budget.SpentThisPeriod)).String()
	budgetKey := append([]byte("budget:"), budgetID[:]...)
	record := committed.KVStore(rh.app.GetKey(app.LayerXHostStoreKey)).Get(paytypes.KernelKVKey(paytypes.BudgetModuleID, budgetKey))
	capturedBudget, err := paytypes.DecodeBudgetRecord(record)
	require.NoError(t, err)
	require.Equal(t, budget, capturedBudget)
	capture["budget_record"] = hexutil.Encode(record)
	precompile, err := layerxpay.NewPrecompile(rh.app.LayerXPayKeeper)
	require.NoError(t, err)
	method := payABI.Methods["getBudget"]
	result, err := precompile.GetExecutor().Execute(committed, &method, common.Address{}, common.Address{}, []interface{}{budgetID}, new(big.Int), true, nil, nil)
	require.NoError(t, err)
	capture["budget_result"] = hexutil.Encode(result)
	encoded, err := json.MarshalIndent(capture, "", "  ")
	require.NoError(t, err)
	t.Logf("CURRENT_RECEIPT_RESTART_CAPTURE=%s", mustRestartCaptureJSON(t, capture))
	path := filepath.Join("..", "..", "services", "agent", "crates", "layerx-agentd", "tests", "fixtures", "current-receipt-restart.json")
	if os.Getenv("UPDATE_CURRENT_RECEIPT_RESTART_FIXTURE") == "1" {
		require.NoError(t, os.WriteFile(path, append(encoded, '\n'), 0o600))
	}
	expected, err := os.ReadFile(path)
	require.NoError(t, err, "commit the node-produced capture printed above before consumer replay")
	require.JSONEq(t, string(expected), string(encoded), "committed restart capture must match real execution and committed keeper state")
}

func mustRestartCaptureJSON(t *testing.T, capture map[string]interface{}) string {
	t.Helper()
	encoded, err := json.Marshal(capture)
	require.NoError(t, err)
	return string(encoded)
}

func setupCurrentReceiptRestartBudget(t *testing.T, ctx sdk.Context, a *app.App, sender, recipient utils.TestAcct, budgetID [32]byte) [32]byte {
	t.Helper()
	private := ed25519.NewKeyFromSeed(label("restart budget owner"))
	public := [32]byte(private.Public().(ed25519.PublicKey))
	did := []byte(evmtypes.LayerXDid(public))
	actor, err := lxphost.DIDID(did)
	require.NoError(t, err)
	message, err := evmtypes.LayerXBindMessage(a.EvmKeeper.ChainID(ctx), sender.EvmAddress, 0)
	require.NoError(t, err)
	_, err = a.EvmKeeper.BindLayerX(ctx, sender.EvmAddress, public, [64]byte(ed25519.Sign(private, message)))
	require.NoError(t, err)
	boundKey, bound := a.EvmKeeper.GetLayerXDid(ctx, sender.EvmAddress)
	require.True(t, bound)
	require.Equal(t, public, boundKey)
	hostKey := a.GetKey(app.LayerXHostStoreKey)
	execute := func(activityType uint32, payload []byte) {
		sequence, err := a.LayerXHost.ReadIdentitySequence(ctx, did)
		require.NoError(t, err)
		next, err := lxphost.NextSequence(ctx, hostKey)
		require.NoError(t, err)
		activity := lxphost.Activity{
			ProtocolVersion: 3, Type: activityType, ActorDID: did, AuthorityKey: public,
			AccountSequence: sequence, NotBefore: uint64(ctx.BlockTime().UnixMilli()),
			NotAfter:       uint64(ctx.BlockTime().UnixMilli()) + a.LayerXHost.TimestampWindowMs(),
			IdempotencyKey: sha256.Sum256(binary.BigEndian.AppendUint64(nil, next)), Payload: payload,
		}
		digest, err := a.LayerXHost.SigningHash(activity)
		require.NoError(t, err)
		activity.Signature = ed25519.Sign(private, digest[:])
		receipt, err := a.LayerXHost.HostExecute(lxphost.WithSequence(ctx, next), activity, lxphost.KernelAuthority{
			Actor: actor, Principal: actor, VerifiedKey: public,
		})
		require.NoError(t, err)
		require.Zero(t, receipt.ResultCode)
	}
	registration := append(append([]byte{0x71, 1, 0, 2}, actor[:]...), public[:]...)
	execute(0x00070001, registration)
	salt := [32]byte{0x73}
	asset := sha256.Sum256(append(append([]byte("LX:ASSET:v1"), actor[:]...), salt[:]...))
	require.NoError(t, a.LayerXLedgerKeeper.SetParams(ctx, ledgertypes.Params{}))
	ledgerActor, err := a.LayerXLedgerKeeper.Actor(ctx, sender.EvmAddress)
	require.NoError(t, err)
	registered, err := a.LayerXLedgerKeeper.RegisterAsset(ctx, ledgerActor, ledgertypes.RegisterArgs{
		AssetID: asset, Salt: salt, Symbol: "T", Name: "T", Decimals: 6,
		SupplyCap: sdk.NewInt(1000), IssuerKind: ledgertypes.AssetIssuerNative,
	}, [32]byte{0x74})
	require.NoError(t, err)
	require.Zero(t, registered.Receipt.ResultCode)
	opened, err := a.LayerXLedgerKeeper.OpenAccount(ctx, ledgerActor, asset, [32]byte{0x75})
	require.NoError(t, err)
	require.Zero(t, opened.Receipt.ResultCode)
	sourceName, source, err := lxphost.AssetAccountID(public, asset)
	require.NoError(t, err)
	minted, err := a.LayerXLedgerKeeper.Mint(ctx, ledgerActor, asset, source, sdk.NewInt(1000), [32]byte{0x76})
	require.NoError(t, err)
	require.Zero(t, minted.Receipt.ResultCode)
	mainName, main, err := lxphost.MainAccountID(public)
	require.NoError(t, err)
	field := func(out, value []byte) []byte {
		return append(binary.BigEndian.AppendUint32(out, uint32(len(value))), value...)
	}
	putMain := func(key [32]byte) (string, [32]byte) {
		name, id, err := lxphost.MainAccountID(key)
		require.NoError(t, err)
		account := field(nil, id[:])
		account = field(account, []byte(name))
		account = append(account, 1)
		account = append(account, make([]byte, 16)...)
		account = field(account, asset[:])
		account = append(account, 1)
		account = binary.BigEndian.AppendUint64(account, 0)
		account = binary.BigEndian.AppendUint64(account, 1)
		account = append(account, 0, 0)
		account = field(account, key[:])
		ctx.KVStore(hostKey).Set(append([]byte{6}, id[:]...), append(account, 1))
		return name, id
	}
	putMain(public)
	a.LayerXPayKeeper.SetParams(ctx, paytypes.Params{BudgetEnabled: true})
	gate, err := paytypes.KernelModuleEnableKey(paytypes.BudgetModuleID)
	require.NoError(t, err)
	enabled := [32]byte{31: 1}
	ctx.KVStore(hostKey).Set(gate, enabled[:])
	budgetAccount, err := codec.DeriveAccountID([]byte("agent:" + string(did) + ":budget:" + hex.EncodeToString(budgetID[:])))
	require.NoError(t, err)
	sourceAccount, err := a.LayerXHost.ReadAccount(ctx, sourceName, source)
	require.NoError(t, err)
	require.Equal(t, source, sourceAccount.ID)
	require.True(t, sourceAccount.HasAuthorityKey)
	require.Equal(t, public, sourceAccount.AuthorityKey)
	fundingKey := [32]byte{0x77}
	funding := ledgertypes.SendArgs{
		From: source, To: main, Asset: asset, Amount: sdk.NewInt(1000),
		Sequence: sourceAccount.NextSequence, IdempotencyKey: fundingKey,
		ExpiresAt: uint64(ctx.BlockTime().UnixMilli()) + 10000,
		PublicKey: public, NetworkID: uint32(config.DefaultChainID), ProtocolVersion: 3,
	}
	transfer, err := ledgertypes.EncodeSendPayload(funding)
	require.NoError(t, err)
	funding.ContextHash, err = codec.DomainHash(codec.DomainContextHash,
		append(append([]byte(nil), transfer[4:116]...), fundingKey[:]...))
	require.NoError(t, err)
	transfer, err = ledgertypes.EncodeSendPayload(funding)
	require.NoError(t, err)
	funded, err := a.LayerXLedgerKeeper.Send(ctx, ledgerActor, transfer, fundingKey)
	require.NoError(t, err)
	require.Zero(t, funded.Receipt.ResultCode)
	balance, err := a.LayerXHost.ReadBalance(ctx, main, asset)
	require.NoError(t, err)
	require.Equal(t, sdk.NewInt(1000), balance)
	ownerAccount, err := a.LayerXHost.ReadAccount(ctx, mainName, main)
	require.NoError(t, err)
	identitySequence, err := a.LayerXHost.ReadIdentitySequence(ctx, did)
	require.NoError(t, err)
	require.Equal(t, openHeight, ctx.BlockHeight())
	require.Equal(t, uint64(5), identitySequence)
	require.Equal(t, main, ownerAccount.ID)
	require.True(t, ownerAccount.HasAuthorityKey)
	require.Equal(t, public, ownerAccount.AuthorityKey)
	authority, err := a.LayerXPayKeeper.AuthorityFor(ctx, sender.EvmAddress)
	require.NoError(t, err)
	require.Equal(t, did, authority.ActorDID)
	require.Equal(t, actor, authority.Kernel.Actor)
	require.Equal(t, public, authority.AuthorityKey)
	require.Equal(t, public, authority.Kernel.VerifiedKey)
	args := paykeeper.BudgetCreateArgs{
		BudgetID: budgetID, BudgetAccount: budgetAccount, AssetID: asset,
		PerPeriodLimit: codec.U128{Lo: 1000}, Amount: codec.U128{Lo: 1000},
		PeriodLengthMs: 60000, PeriodStartMs: uint64(ctx.BlockTime().UnixMilli()),
		ExpiryMs: uint64(ctx.BlockTime().UnixMilli()) + 120000, RolloverPolicy: 1,
	}
	payload, err := paytypes.EncodeBudgetCreate(args.BudgetID, args.BudgetAccount, args.AssetID, args.PurposeHash,
		args.PerPeriodLimit, args.CarryCap, args.Amount, args.PeriodLengthMs, args.PeriodStartMs,
		args.ExpiryMs, args.RevocationSequence, args.RolloverPolicy)
	require.NoError(t, err)
	require.Len(t, payload, 211)
	require.Equal(t, uint16(1), binary.BigEndian.Uint16(payload[:2]))
	mapping := paytypes.MethodActivity["budgetCreate"]
	require.Equal(t, uint32(0x00030001), mapping.Type)
	last, err := a.LayerXHost.ReadLastSequence(ctx, did)
	require.NoError(t, err)
	activity := lxphost.Activity{
		ProtocolVersion: 3, Type: mapping.Type, ActorDID: authority.ActorDID,
		AuthorityKey: authority.AuthorityKey, AccountSequence: identitySequence,
		NotBefore:      uint64(ctx.BlockTime().UnixMilli()),
		NotAfter:       uint64(ctx.BlockTime().UnixMilli()) + a.LayerXHost.TimestampWindowMs(),
		IdempotencyKey: budgetID, Payload: payload,
	}
	authority.Kernel.Bound, authority.Kernel.Principal = true, main
	expectedCtx, _ := ctx.CacheContext()
	expected, expectedErr := a.LayerXHost.HostExecute(lxphost.WithSequence(expectedCtx, last+1), activity, authority.Kernel)
	created, err := a.LayerXPayKeeper.BudgetCreate(ctx, sender.EvmAddress, args)
	diagnostic := func(receipt lxphost.Receipt) string {
		var binding [32]byte
		if len(receipt.Encoded) != 0 {
			decoded, decodeErr := codec.DecodeReceipt(receipt.Encoded)
			require.NoError(t, decodeErr)
			binding = decoded.ActivityID
		}
		return fmt.Sprintf("activity_type=%#x binding=%x account_id=%x verified_key=%x next_sequence=%d actor_did=%s",
			activity.Type, binding, ownerAccount.ID, authority.Kernel.VerifiedKey, ownerAccount.NextSequence, activity.ActorDID)
	}
	facts := fmt.Sprintf("budgetCreate at height=%d encoding_version=%d identity_sequence=%d global_sequence=%d: restart supplies {%s}; keeper case supplies {%s}; expected_error=%v actual_error=%v",
		ctx.BlockHeight(), binary.BigEndian.Uint16(payload[:2]), identitySequence, last+1,
		diagnostic(created), diagnostic(expected), expectedErr, err)
	require.NoError(t, err, facts)
	require.Zero(t, created.ResultCode, facts)
	require.NoError(t, expectedErr, facts)
	require.Zero(t, expected.ResultCode, facts)
	require.Equal(t, expected, created, facts)
	expectedBudget, expectedFound, err := a.LayerXPayKeeper.GetBudget(expectedCtx, budgetID)
	require.NoError(t, err)
	require.True(t, expectedFound)
	budget, found, err := a.LayerXPayKeeper.GetBudget(ctx, budgetID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, expectedBudget, budget)
	recipientPrivate := ed25519.NewKeyFromSeed(label("restart budget recipient"))
	recipientPublic := [32]byte(recipientPrivate.Public().(ed25519.PublicKey))
	recipientDID := []byte(evmtypes.LayerXDid(recipientPublic))
	recipientActor, err := lxphost.DIDID(recipientDID)
	require.NoError(t, err)
	message, err = evmtypes.LayerXBindMessage(a.EvmKeeper.ChainID(ctx), recipient.EvmAddress, 0)
	require.NoError(t, err)
	_, err = a.EvmKeeper.BindLayerX(ctx, recipient.EvmAddress, recipientPublic, [64]byte(ed25519.Sign(recipientPrivate, message)))
	require.NoError(t, err)
	boundRecipient, bound := a.EvmKeeper.GetLayerXDid(ctx, recipient.EvmAddress)
	require.True(t, bound)
	require.Equal(t, recipientPublic, boundRecipient)
	next, err := lxphost.NextSequence(ctx, hostKey)
	require.NoError(t, err)
	registration = append(append([]byte{0x71, 1, 0, 2}, recipientActor[:]...), recipientPublic[:]...)
	recipientActivity := lxphost.Activity{
		ProtocolVersion: 3, Type: 0x00070001, ActorDID: recipientDID, AuthorityKey: recipientPublic,
		NotBefore:      uint64(ctx.BlockTime().UnixMilli()),
		NotAfter:       uint64(ctx.BlockTime().UnixMilli()) + a.LayerXHost.TimestampWindowMs(),
		IdempotencyKey: sha256.Sum256(recipientDID), Payload: registration,
	}
	digest, err := a.LayerXHost.SigningHash(recipientActivity)
	require.NoError(t, err)
	recipientActivity.Signature = ed25519.Sign(recipientPrivate, digest[:])
	registeredRecipient, err := a.LayerXHost.HostExecute(lxphost.WithSequence(ctx, next), recipientActivity, lxphost.KernelAuthority{
		Actor: recipientActor, Principal: recipientActor, VerifiedKey: recipientPublic,
	})
	require.NoError(t, err)
	require.Zero(t, registeredRecipient.ResultCode)
	recipientName, recipientMain := putMain(recipientPublic)
	recipientAccount, err := a.LayerXHost.ReadAccount(ctx, recipientName, recipientMain)
	require.NoError(t, err)
	require.Equal(t, recipientMain, recipientAccount.ID)
	require.True(t, recipientAccount.HasAuthorityKey)
	require.Equal(t, recipientPublic, recipientAccount.AuthorityKey)
	kind, canonical := codec.AccountKind([]byte(recipientName))
	require.True(t, canonical)
	require.Equal(t, uint8(1), uint8(kind), "budgetSpend requires an agent-main recipient for this non-native-source budget")
	require.Equal(t, kind, recipientAccount.Kind)
	budgetAccountName := "agent:" + string(did) + ":budget:" + hex.EncodeToString(budgetID[:])
	budgetAccountRecord, err := a.LayerXHost.ReadAccount(ctx, budgetAccountName, budget.BudgetAccount)
	require.NoError(t, err)
	require.Equal(t, uint8(2), budgetAccountRecord.Kind)
	require.Equal(t, budget.Owner, main, "the owner spend path does not load delegate capabilities")
	t.Logf("restart budgetSpend setup: budget_account=%x recipient_evm=%s recipient_main=%x recipient_kind=%d native_source=%t owner=%x signer_main=%x spent=%v limit=%v",
		budget.BudgetAccount, recipient.EvmAddress, recipientMain, kind, budget.NativeSource, budget.Owner, main, budget.SpentThisPeriod, budget.PerPeriodLimit)
	return recipientMain
}

func TestReceiptSeqHashChain(t *testing.T) {
	a, b := utils.NewSigner(), utils.NewSigner()
	rh := newReceiptHarness(t, 0, nil, a, b)
	prev := [32]byte{}
	for i := uint64(0); i < 3; i++ {
		etx, bz := transfer(t, a, i, b.EvmAddress, big.NewInt(1))
		acc, rec, _ := rh.accept(t, bz)
		require.Equal(t, i+1, rec.GlobalSequence)
		require.Equal(t, prev, rec.PreviousStateRoot)
		want := expectedSeqHash(prev, i+1, [32]byte(etx.Hash()))
		require.Equal(t, want, rec.ResultingStateRoot)
		require.Equal(t, acc.SeqHash, rec.ResultingStateRoot)
		prev = want
	}

	in := vectorInput()
	in.Accepted.SeqHash[0] ^= 1
	_, err := sequencer.BuildReceipt(in, testKeyID)
	require.ErrorIs(t, err, sequencer.ErrReceiptInput)
	in = vectorInput()
	in.Accepted.Seq = 0
	_, err = sequencer.BuildReceipt(in, testKeyID)
	require.ErrorIs(t, err, sequencer.ErrReceiptInput)
}

func TestReceiptResultAndActivityRoot(t *testing.T) {
	a := utils.NewSigner()
	rh := newReceiptHarness(t, 0, nil, a)
	_, bz := evmTx(t, a, receiptLegacyTx(0, nil, big.NewInt(0), createGas, logInit))
	_, rec, _ := rh.accept(t, bz)
	require.Equal(t, int32(0), rec.ResultCode)
	require.Equal(t, sha256.Sum256(logRuntime), rec.ActivityRoot)

	contract := crypto.CreateAddress(a.EvmAddress, 0)
	_, bz = evmTx(t, a, receiptLegacyTx(1, &contract, big.NewInt(0), callGas, nil))
	_, rec, _ = rh.accept(t, bz)
	require.Equal(t, int32(0), rec.ResultCode)
	require.Equal(t, sha256.Sum256(nil), rec.ActivityRoot)

	etx, bz := evmTx(t, a, receiptLegacyTx(2, nil, big.NewInt(0), createGas, revertInit))
	acc, rec, _ := rh.accept(t, bz)
	require.Equal(t, int32(1), rec.ResultCode)
	require.Equal(t, sha256.Sum256(errorX), rec.ActivityRoot)
	exec := rh.transient(t, etx, acc)
	require.Equal(t, uint32(0), exec.Status)
	require.NotEmpty(t, exec.VmError)
	vm := sha256.Sum256([]byte(exec.VmError))
	require.Equal(t, vm[:], rec.Effects[0].Body[68:100])
	require.Equal(t, uint32(len(errorX)), binary.BigEndian.Uint32(rec.Effects[0].Body[100:104]))
}

func TestReceiptFeeFields(t *testing.T) {
	a, b, sid := utils.NewSigner(), utils.NewSigner(), utils.NewSigner()
	denom := bridgetypes.SidioraDenom()
	rh := newReceiptHarness(t, 0, func(ctx sdk.Context, a *app.App) {
		params := a.EvmKeeper.GetParams(ctx)
		params.FeeTokenEnabled = true
		params.MaxFeeTokenRateAge = evmtypes.DefaultMaxFeeTokenRateAge
		params.AllowedFeeDenoms = []evmtypes.AllowedFeeDenom{{
			Denom: denom, Rate: sdk.NewDec(evmtypes.InitialSidioraBaseUnitsPerPax), RateUpdateHeight: committedTip,
		}}
		a.EvmKeeper.SetParams(ctx, params)
		coins := sdk.NewCoins(sdk.NewCoin(denom, sdk.NewInt(sidFunding)))
		require.NoError(t, a.BankKeeper.MintCoins(ctx, "mint", coins))
		require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, "mint", sid.AccountAddress, coins))
		require.NoError(t, a.EvmKeeper.SetAccountFeeDenom(ctx, sid.EvmAddress, denom))
	}, a, b, sid)

	value := big.NewInt(onePaxInWei)
	_, bz := transfer(t, a, 0, b.EvmAddress, value)
	_, rec, _ := rh.accept(t, bz)
	nativeFee := new(big.Int).Mul(big.NewInt(transferGas), big.NewInt(gasPriceWei))
	require.Equal(t, u128(nativeFee), rec.FeeCharged)
	spent := new(big.Int).Sub(new(big.Int).SetBytes(sliceU128(rec.FromBalanceBefore)), new(big.Int).SetBytes(sliceU128(rec.FromBalanceAfter)))
	requireWei(t, new(big.Int).Add(nativeFee, value), spent)
	require.Equal(t, sha256.Sum256([]byte("uhpx")), rec.Asset)
	require.Equal(t, "uhpx", string(rec.Effects[0].Body[109:]))

	open := rh.openCtx()
	require.True(t, rh.app.EvmKeeper.GetFeeTokenEnabled(open))
	require.Equal(t, denom, rh.app.EvmKeeper.GetAccountFeeDenom(open, sid.EvmAddress))
	sidBefore := rh.app.BankKeeper.GetBalance(open, sid.AccountAddress, denom).Amount
	require.Equal(t, sdk.NewInt(sidFunding), sidBefore)
	_, bz = evmTx(t, sid, receiptLegacyTx(0, &b.EvmAddress, big.NewInt(0), sidFeeGas, nil))
	_, rec, _ = rh.accept(t, bz)
	sidAfter := rh.app.BankKeeper.GetBalance(rh.openCtx(), sid.AccountAddress, denom).Amount
	require.Equal(t, sdk.NewInt(sidConverted), sidBefore.Sub(sidAfter))
	require.Equal(t, u128(big.NewInt(sidConverted)), rec.FeeCharged)
	require.Equal(t, sha256.Sum256([]byte("uhpx")), rec.Asset)
	require.Equal(t, denom, string(rec.Effects[0].Body[109:]))
	require.Equal(t, uint8(len(denom)), rec.Effects[0].Body[108])

	cosmos := signCosmos(t, rh.openCtx(), rh.harness, a, 200_000,
		banktypes.NewMsgSend(a.AccountAddress, b.AccountAddress, sdk.NewCoins(sdk.NewCoin("uhpx", sdk.NewInt(5)))))
	_, rec, _ = rh.accept(t, cosmos)
	require.Equal(t, u128(big.NewInt(cosmosFee)), rec.FeeCharged)
	require.Equal(t, sha256.Sum256([]byte("uhpx")), rec.Asset)
}

func sliceU128(v codec.U128) []byte {
	b := v.Bytes()
	return b[:]
}

func TestReceiptBatchAndModuleFields(t *testing.T) {
	a, b, c, d := utils.NewSigner(), utils.NewSigner(), utils.NewSigner(), utils.NewSigner()
	rh := newReceiptHarness(t, 0, nil, a, b, c, d)
	contract := crypto.CreateAddress(a.EvmAddress, 0)
	auth, err := ethtypes.SignSetCode(c.EvmPrivateKey, ethtypes.SetCodeAuthorization{
		ChainID: *uint256.NewInt(uint64(config.DefaultChainID)), Address: contract, Nonce: 0,
	})
	require.NoError(t, err)
	type opTx struct {
		bz []byte
		op uint8
	}
	var txs []opTx
	_, bz := evmTx(t, a, receiptLegacyTx(0, nil, big.NewInt(0), createGas, logInit))
	txs = append(txs, opTx{bz, ethtypes.LegacyTxType})
	_, bz = evmTx(t, a, dynamicTx(1, &b.EvmAddress, big.NewInt(1), transferGas, nil))
	txs = append(txs, opTx{bz, ethtypes.DynamicFeeTxType})
	_, bz = evmTx(t, b, &ethtypes.SetCodeTx{
		ChainID: uint256.NewInt(uint64(config.DefaultChainID)), Nonce: 0, GasTipCap: uint256.NewInt(gasPriceWei),
		GasFeeCap: uint256.NewInt(gasPriceWei), Gas: callGas, To: c.EvmAddress, Value: uint256.NewInt(0),
		AuthList: []ethtypes.SetCodeAuthorization{auth},
	})
	txs = append(txs, opTx{bz, ethtypes.SetCodeTxType})
	cosmos := signCosmos(t, rh.openCtx(), rh.harness, d, 200_000,
		banktypes.NewMsgSend(d.AccountAddress, b.AccountAddress, sdk.NewCoins(sdk.NewCoin("uhpx", sdk.NewInt(5)))))
	txs = append(txs, opTx{cosmos, sequencer.CosmosOperation})

	var batch [32]byte
	binary.BigEndian.PutUint64(batch[24:], uint64(openHeight))
	for i, tx := range txs {
		acc, rec, _ := rh.accept(t, tx.bz)
		require.Equal(t, tx.op, rec.Operation, "tx %d", i)
		require.Equal(t, batch, rec.BatchID)
		require.Equal(t, receiptModuleID, rec.ModuleID)
		require.Equal(t, uint32(1), rec.ModuleVersion)
		require.Equal(t, uint32(0), rec.ParameterVersion)
		require.Equal(t, uint64(rh.openMs), rec.Timestamp)
		require.Equal(t, testKeyID, rec.ContextHash)
		require.Equal(t, uint32(i+1), binary.BigEndian.Uint32(rec.Effects[0].Body[24:28]))
		require.Equal(t, i+1, acc.TxIndex)
		for _, e := range rec.Effects {
			require.Equal(t, receiptModuleID, e.ModuleID)
		}
	}

	_, err = sequencer.NewReceipter(rh.exec, rh.app, &rh.app.EvmKeeper, rh.key, sequencer.KernelReservedModules)
	require.ErrorIs(t, err, sequencer.ErrReceiptInput)
	in := vectorInput()
	in.ModuleID = 11
	_, err = sequencer.BuildReceipt(in, testKeyID)
	require.ErrorIs(t, err, sequencer.ErrReceiptInput)
}

func TestReceiptAccountFields(t *testing.T) {
	a, b, c := utils.NewSigner(), utils.NewSigner(), utils.NewSigner()
	rh := newReceiptHarness(t, 0, nil, a, b, c)
	funded := new(big.Int).Mul(big.NewInt(fundingUhpx), big.NewInt(weiPerUhpx))
	value := big.NewInt(onePaxInWei)
	fee := new(big.Int).Mul(big.NewInt(transferGas), big.NewInt(gasPriceWei))

	_, bz := transfer(t, a, 0, b.EvmAddress, value)
	_, rec, _ := rh.accept(t, bz)
	require.Equal(t, addr32(a.EvmAddress), rec.From)
	require.Equal(t, addr32(b.EvmAddress), rec.To)
	require.Equal(t, u128(value), rec.Amount)
	require.Equal(t, uint64(0), rec.FromSequence)
	require.Equal(t, u128(funded), rec.FromBalanceBefore)
	require.Equal(t, u128(new(big.Int).Sub(funded, new(big.Int).Add(value, fee))), rec.FromBalanceAfter)
	require.Equal(t, u128(funded), rec.ToBalanceBefore)
	require.Equal(t, u128(new(big.Int).Add(funded, value)), rec.ToBalanceAfter)
	require.Equal(t, [32]byte{}, rec.AuthorizationHash)
	require.Equal(t, [32]byte{}, rec.TransferSetRoot)

	_, bz = evmTx(t, a, receiptLegacyTx(1, nil, big.NewInt(0), createGas, logInit))
	_, rec, _ = rh.accept(t, bz)
	contract := crypto.CreateAddress(a.EvmAddress, 1)
	require.Equal(t, addr32(contract), rec.To)
	require.Equal(t, uint64(1), rec.FromSequence)
	require.Equal(t, codec.U128{}, rec.ToBalanceBefore)
	require.Equal(t, codec.U128{}, rec.ToBalanceAfter)

	auth, err := ethtypes.SignSetCode(c.EvmPrivateKey, ethtypes.SetCodeAuthorization{
		ChainID: *uint256.NewInt(uint64(config.DefaultChainID)), Address: contract, Nonce: 0,
	})
	require.NoError(t, err)
	etx, bz := evmTx(t, a, &ethtypes.SetCodeTx{
		ChainID: uint256.NewInt(uint64(config.DefaultChainID)), Nonce: 2, GasTipCap: uint256.NewInt(gasPriceWei),
		GasFeeCap: uint256.NewInt(gasPriceWei), Gas: callGas, To: c.EvmAddress, Value: uint256.NewInt(0),
		AuthList: []ethtypes.SetCodeAuthorization{auth},
	})
	_, rec, _ = rh.accept(t, bz)
	enc, err := rlp.EncodeToBytes(etx.SetCodeAuthorizations())
	require.NoError(t, err)
	require.Equal(t, [32]byte(crypto.Keccak256Hash(enc)), rec.AuthorizationHash)
	require.NotEqual(t, [32]byte{}, rec.AuthorizationHash)
	require.Equal(t, addr32(c.EvmAddress), rec.To)
	require.Equal(t, uint64(2), rec.FromSequence)
	require.Equal(t, uint8(ethtypes.SetCodeTxType), rec.Operation)
}

func TestReceiptSummaryEffect(t *testing.T) {
	a := utils.NewSigner()
	rh := newReceiptHarness(t, 0, nil, a)
	var cumulative uint64
	contract := crypto.CreateAddress(a.EvmAddress, 0)
	cases := []struct {
		data     ethtypes.TxData
		contract common.Address
		sponsor  common.Address
		retLen   uint32
		logs     uint32
	}{
		{receiptLegacyTx(0, nil, big.NewInt(0), createGas, logInit), contract, common.Address{}, uint32(len(logRuntime)), 0},
		{receiptLegacyTx(1, &contract, big.NewInt(0), callGas, nil), common.Address{}, common.Address{}, 0, 3},
		{receiptLegacyTx(2, nil, big.NewInt(0), createGas, sponsoredInit), crypto.CreateAddress(a.EvmAddress, 2), sponsorAddr, 0, 1},
	}
	for i, c := range cases {
		etx, bz := evmTx(t, a, c.data)
		acc, rec, _ := rh.accept(t, bz)
		exec := rh.transient(t, etx, acc)
		cumulative += exec.GasUsed
		e := rec.Effects[0]
		require.Equal(t, uint8(1), e.Kind, "case %d", i)
		require.Equal(t, uint16(0), e.Ordinal)
		require.Equal(t, uint16(0), e.EventType)
		require.False(t, e.Monetary)
		require.Equal(t, [32]byte{}, e.TransferSetRoot)
		want := summaryBody(exec.GasUsed, cumulative, gasPriceWei, uint32(acc.TxIndex), c.contract, c.sponsor, "", c.retLen, c.logs, "uhpx")
		require.Equal(t, hex.EncodeToString(want), hex.EncodeToString(e.Body), "case %d", i)
		require.LessOrEqual(t, len(e.Body), 191)
		require.Len(t, rec.Effects, int(c.logs)+1)
	}

	in := vectorInput()
	body, err := sequencer.SummaryEffect(in)
	require.NoError(t, err)
	require.Len(t, body, 113)
	in.FeeDenom = string(repeat('a', 65))
	_, err = sequencer.SummaryEffect(in)
	require.ErrorIs(t, err, sequencer.ErrReceiptInput)
}

func TestReceiptLogEffects(t *testing.T) {
	a := utils.NewSigner()
	rh := newReceiptHarness(t, 0, nil, a)
	_, bz := evmTx(t, a, receiptLegacyTx(0, nil, big.NewInt(0), createGas, logInit))
	rh.accept(t, bz)
	contract := crypto.CreateAddress(a.EvmAddress, 0)
	etx, bz := evmTx(t, a, receiptLegacyTx(1, &contract, big.NewInt(0), callGas, nil))
	acc, rec, _ := rh.accept(t, bz)
	wantData := receiptWord(0x2a)
	want := [][]byte{
		logBody(contract, wantData[:], receiptWord(1), receiptWord(2)),
		logBody(contract, []byte{0x2a, 0, 0, 0, 0}, receiptWord(3), receiptWord(4), receiptWord(5), receiptWord(6)),
		logBody(contract, nil),
	}
	require.Len(t, rec.Effects, 4)
	exec := rh.transient(t, etx, acc)
	for i, w := range want {
		e := rec.Effects[i+1]
		require.Equal(t, uint8(3), e.Kind)
		require.Equal(t, uint16(i+1), e.Ordinal)
		require.Equal(t, uint16(0), e.EventType)
		require.Equal(t, hex.EncodeToString(w), hex.EncodeToString(e.Body))
		body, err := sequencer.LogEffect(exec.Logs[i])
		require.NoError(t, err)
		require.Equal(t, w, body)
	}
	require.Len(t, want[1], 181)

	five := &evmtypes.Log{Address: contract.Hex(), Topics: make([]string, 5)}
	for i := range five.Topics {
		five.Topics[i] = common.Hash(receiptWord(byte(i))).Hex()
	}
	_, err := sequencer.LogEffect(five)
	require.ErrorIs(t, err, sequencer.ErrReceiptInput)
	_, err = sequencer.LogEffect(&evmtypes.Log{Address: contract.Hex(), Topics: []string{"0x01"}})
	require.ErrorIs(t, err, sequencer.ErrReceiptInput)
	_, err = sequencer.LogEffect(&evmtypes.Log{Address: "0x1234"})
	require.ErrorIs(t, err, sequencer.ErrReceiptInput)

	// A transaction with more than 511 logs is encoded in full.
	unsigned, err := sequencer.BuildReceipt(largeFixtureLogsInput(600, contract), testKeyID)
	require.NoError(t, err)
	require.Equal(t, uint32(601), binary.BigEndian.Uint32(unsigned[162:166]))
}

// largeFixtureLogsInput is the shared receipt vector carrying n logs of contract.
func largeFixtureLogsInput(n int, contract common.Address) *sequencer.ReceiptInput {
	in := vectorInput()
	in.EVM.Logs = make([]*evmtypes.Log, n)
	for i := range in.EVM.Logs {
		in.EVM.Logs[i] = &evmtypes.Log{Address: contract.Hex(), Data: []byte{byte(i)}}
	}
	return in
}

func TestReceiptLargeFixtureVerifies(t *testing.T) {
	key := vectorKey(t)
	pub := key.PublicKey()
	contract := common.BytesToAddress(repeat(0x44, 20))
	// 600 logs exceed MaxEffects; 512 logs give 513 effects, the first count
	// MaxEffects refuses.
	for _, logs := range []int{600, 512} {
		signed, err := key.SignReceipt(largeFixtureLogsInput(logs, contract))
		require.NoError(t, err)
		require.Equal(t, uint32(logs+1), binary.BigEndian.Uint32(signed[162:166]))
		verified, err := verify.ReceiptSignature(signed, pub)
		require.NoError(t, err, "%d logs", logs)
		require.Len(t, verified.Receipt.Effects, logs+1)
		require.Equal(t, receiptModuleID, verified.Receipt.ModuleID)
		require.Equal(t, sequencer.ReceiptLayoutVersion, verified.Receipt.ModuleVersion)
		for i, e := range verified.Receipt.Effects {
			require.Equal(t, uint16(i), e.Ordinal)
			require.Equal(t, receiptModuleID, e.ModuleID)
		}
		tampered := append([]byte(nil), signed...)
		tampered[len(tampered)-1] ^= 1
		_, err = verify.ReceiptSignature(tampered, pub)
		require.ErrorIs(t, err, verify.ErrSequencerSignature)
	}
}

func TestReceiptEffectsNonMonetary(t *testing.T) {
	a, b := utils.NewSigner(), utils.NewSigner()
	rh := newReceiptHarness(t, 0, nil, a, b)
	contract := crypto.CreateAddress(a.EvmAddress, 0)
	_, create := evmTx(t, a, receiptLegacyTx(0, nil, big.NewInt(0), createGas, logInit))
	_, call := evmTx(t, a, receiptLegacyTx(1, &contract, big.NewInt(0), callGas, nil))
	_, revert := evmTx(t, a, receiptLegacyTx(2, nil, big.NewInt(0), createGas, revertInit))
	_, send := transfer(t, b, 0, a.EvmAddress, big.NewInt(1))
	for _, bz := range [][]byte{create, call, revert, send} {
		_, rec, _ := rh.accept(t, bz)
		for i, e := range rec.Effects {
			require.False(t, e.Monetary)
			require.Equal(t, [32]byte{}, e.TransferSetRoot)
			if i == 0 {
				require.Equal(t, uint8(1), e.Kind)
			} else {
				require.Equal(t, uint8(3), e.Kind)
			}
		}
	}
	unsigned, err := sequencer.BuildReceipt(vectorInput(), testKeyID)
	require.NoError(t, err)
	// The monetary byte follows module_id, ordinal, event_type and kind.
	for _, off := range []int{166, 166 + 48 + 113, 166 + 48 + 113 + 48 + 117} {
		require.Equal(t, byte(0), unsigned[off+7])
	}
}

func TestReceiptDigestMatchesWireCrate(t *testing.T) {
	unsigned, err := sequencer.BuildReceipt(vectorInput(), [32]byte(repeat(0x33, 32)))
	require.NoError(t, err)
	digest := codec.ReceiptDigest(unsigned)
	require.Equal(t, hex.EncodeToString(wireVector(t, "GO_RECEIPT_DIGEST")), hex.EncodeToString(digest[:]))
	require.Equal(t, sha256.Sum256(append([]byte("LXP/v1/receipt\x00"), unsigned...)), digest)
	signed := wireVector(t, "GO_SIGNED_RECEIPT")
	require.Equal(t, signed[:len(signed)-69], unsigned[:len(unsigned)-1])
	require.Equal(t, byte(0), unsigned[len(unsigned)-1])
	rec, err := codec.DecodeReceipt(signed)
	require.NoError(t, err)
	require.Equal(t, digest, rec.Digest())
}

func TestReceiptKeySeparateFromConsensusKey(t *testing.T) {
	dir := t.TempDir()
	consensusFile, consensusPub := writeConsensusKey(t, dir, consensusKeySeed)
	pins := []verify.SequencerAuthorization{
		{SequencerID: testKeyID, PublicKey: publicOf(testKeySeed), FirstBatchNumber: 1, LastBatchNumber: 10},
		{SequencerID: [32]byte(label("consensus id")), PublicKey: publicOf(consensusKeySeed), FirstBatchNumber: 1, LastBatchNumber: 10},
	}
	_, err := sequencer.LoadReceiptKey(writeSeed(t, dir, "reused", consensusKeySeed, 0o600), consensusFile, pins, 2)
	require.ErrorIs(t, err, sequencer.ErrReceiptKeyIsConsensusKey)
	_, err = sequencer.LoadReceiptKey(consensusFile, consensusFile, pins, 2)
	require.ErrorIs(t, err, sequencer.ErrReceiptKeyIsConsensusKey)
	_, err = sequencer.LoadReceiptKey(writeSeed(t, dir, "exposed", testKeySeed, 0o644), consensusFile, pins, 2)
	require.ErrorIs(t, err, sequencer.ErrReceiptKeyFile)
	short := filepath.Join(dir, "short")
	require.NoError(t, os.WriteFile(short, []byte("abcd\n"), 0o600))
	_, err = sequencer.LoadReceiptKey(short, consensusFile, pins, 2)
	require.ErrorIs(t, err, sequencer.ErrReceiptKeyFile)

	key, err := sequencer.LoadReceiptKey(writeSeed(t, dir, "receipt_key", testKeySeed, 0o600), consensusFile, pins, 2)
	require.NoError(t, err)
	pub := key.PublicKey()
	require.Equal(t, publicOf(testKeySeed), pub)
	require.NotEqual(t, consensusPub, pub[:])
	in := vectorInput()
	in.Accepted.Height = 2
	signed := assertReceiptKeySignature(t, key, in)
	_, err = verify.ReceiptSignature(signed, [32]byte(consensusPub))
	require.ErrorIs(t, err, verify.ErrSequencerSignature)
}

func TestReceiptSignatureEncoding(t *testing.T) {
	key := vectorKey(t)
	in := vectorInput()
	unsigned, err := sequencer.BuildReceipt(in, [32]byte(repeat(0x33, 32)))
	require.NoError(t, err)
	signed, err := key.SignReceipt(in)
	require.NoError(t, err)
	require.Len(t, signed, len(unsigned)-1+1+4+64)
	tail := signed[len(unsigned)-1:]
	require.Equal(t, byte(1), tail[0])
	require.Equal(t, uint32(64), binary.BigEndian.Uint32(tail[1:5]))
	digest := codec.ReceiptDigest(unsigned)
	pub := key.PublicKey()
	require.True(t, ed25519.Verify(pub[:], digest[:], tail[5:]))
	require.Equal(t, [32]byte(wireVector(t, "PINNED_RECEIPT_KEY")), pub)

	verified, err := verify.ReceiptSignature(signed, pub)
	require.NoError(t, err)
	require.Equal(t, digest, verified.Digest)
	_, err = verify.ReceiptSignature(unsigned, pub)
	require.ErrorIs(t, err, verify.ErrReceiptMissingSignature)
	tampered := append([]byte(nil), signed...)
	tampered[len(tampered)-1] ^= 1
	_, err = verify.ReceiptSignature(tampered, pub)
	require.ErrorIs(t, err, verify.ErrSequencerSignature)
}

func TestReceiptKeyMustBePinned(t *testing.T) {
	pub := publicOf(testKeySeed)
	_, err := loadKey(t, testKeySeed, nil, openHeight)
	require.ErrorIs(t, err, sequencer.ErrReceiptKeyNotPinned)
	_, err = loadKey(t, testKeySeed, []verify.SequencerAuthorization{
		{SequencerID: testKeyID, PublicKey: publicOf(label("another host")), FirstBatchNumber: 1, LastBatchNumber: 100},
	}, openHeight)
	require.ErrorIs(t, err, sequencer.ErrReceiptKeyNotPinned)
	_, err = loadKey(t, testKeySeed, []verify.SequencerAuthorization{
		{SequencerID: testKeyID, PublicKey: pub, FirstBatchNumber: 3, LastBatchNumber: 100},
	}, openHeight)
	require.ErrorIs(t, err, sequencer.ErrReceiptKeyNotPinned)
	_, err = loadKey(t, testKeySeed, []verify.SequencerAuthorization{
		{SequencerID: testKeyID, PublicKey: pub, FirstBatchNumber: 100, LastBatchNumber: 1},
	}, openHeight)
	require.ErrorIs(t, err, sequencer.ErrReceiptKeyPin)
	_, err = loadKey(t, testKeySeed, []verify.SequencerAuthorization{
		{PublicKey: pub, FirstBatchNumber: 1, LastBatchNumber: 100},
	}, openHeight)
	require.ErrorIs(t, err, sequencer.ErrReceiptKeyPin)

	// Rotation: K1's entry closes at 4, the next key id covers 5 onwards.
	nextID := [32]byte(label("rotated key id"))
	key, err := loadKey(t, testKeySeed, []verify.SequencerAuthorization{
		{SequencerID: testKeyID, PublicKey: pub, FirstBatchNumber: 1, LastBatchNumber: 4},
		{SequencerID: nextID, PublicKey: pub, FirstBatchNumber: 5, LastBatchNumber: 9},
	}, openHeight)
	require.NoError(t, err)
	id, err := key.KeyID(4)
	require.NoError(t, err)
	require.Equal(t, testKeyID, id)
	id, err = key.KeyID(5)
	require.NoError(t, err)
	require.Equal(t, nextID, id)
	in := vectorInput()
	in.Accepted.Height = 10
	_, err = key.SignReceipt(in)
	require.ErrorIs(t, err, sequencer.ErrReceiptKeyNotPinned)

	// A key pinned only below the open height refuses to receipt it.
	a, b := utils.NewSigner(), utils.NewSigner()
	h := newHarness(t, a, b)
	h.open()
	stale, err := loadKey(t, testKeySeed, []verify.SequencerAuthorization{
		{SequencerID: testKeyID, PublicKey: pub, FirstBatchNumber: 1, LastBatchNumber: 1},
	}, committedTip)
	require.NoError(t, err)
	r, err := sequencer.NewReceipter(h.exec, h.app, &h.app.EvmKeeper, stale, receiptModuleID)
	require.NoError(t, err)
	_, bz := transfer(t, a, 0, b.EvmAddress, big.NewInt(1))
	acc, signed, err := r.Accept(bz)
	require.ErrorIs(t, err, sequencer.ErrReceiptKeyNotPinned)
	require.Nil(t, acc)
	require.Nil(t, signed)
	batch, err := h.exec.Seal(openHeight)
	require.NoError(t, err)
	require.Empty(t, batch.Txs)
}

// mixedTxs builds 200 EVM transactions from four senders: log-contract
// creations and calls, legacy and dynamic-fee transfers and reverting creations.
func mixedTxs(t *testing.T, senders []utils.TestAcct) ([][]byte, []*ethtypes.Transaction) {
	var txs [][]byte
	var etxs []*ethtypes.Transaction
	for n := uint64(0); n < 50; n++ {
		for s, from := range senders {
			contract := crypto.CreateAddress(from.EvmAddress, 0)
			peer := senders[(s+1)%len(senders)].EvmAddress
			var data ethtypes.TxData
			switch {
			case n == 0:
				data = receiptLegacyTx(n, nil, big.NewInt(0), createGas, logInit)
			case n%5 == 1:
				data = receiptLegacyTx(n, &contract, big.NewInt(0), callGas, nil)
			case n%5 == 2:
				data = dynamicTx(n, &peer, big.NewInt(int64(n)), transferGas, nil)
			case n%5 == 3:
				data = dynamicTx(n, nil, big.NewInt(0), createGas, revertInit)
			case n%5 == 4:
				data = receiptLegacyTx(n, &peer, big.NewInt(int64(s+1)), transferGas, nil)
			default:
				data = dynamicTx(n, &contract, big.NewInt(0), callGas, nil)
			}
			etx, bz := evmTx(t, from, data)
			txs = append(txs, bz)
			etxs = append(etxs, etx)
		}
	}
	return txs, etxs
}

func TestReceiptEqualsCommittedReceipt(t *testing.T) {
	senders := []utils.TestAcct{utils.NewSigner(), utils.NewSigner(), utils.NewSigner(), utils.NewSigner()}
	txs, etxs := mixedTxs(t, senders)
	require.Len(t, txs, 200)

	first := newReceiptHarness(t, 0, nil, senders...)
	receipts := make([][]byte, len(txs))
	decoded := make([]*codec.Receipt, len(txs))
	for i, bz := range txs {
		_, decoded[i], receipts[i] = first.accept(t, bz)
	}

	// The same transactions on a fresh executor give byte-identical receipts.
	replay := newReceiptHarness(t, first.openMs, nil, senders...)
	for i, bz := range txs {
		_, _, again := replay.accept(t, bz)
		require.Equal(t, receipts[i], again, "tx %d", i)
	}

	// The committed block of the same transactions holds the same results.
	twin := newHarness(t, senders...)
	resp, err := twin.app.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{
		Txs:    txs,
		Header: &tmproto.Header{ChainID: testChainID, Height: openHeight, Time: time.UnixMilli(first.openMs).UTC()},
	})
	require.NoError(t, err)
	require.Len(t, resp.TxResults, len(txs))
	twin.app.SetDeliverStateToCommit()
	_, err = twin.app.Commit(context.Background())
	require.NoError(t, err)
	ctx := sdk.NewContext(twin.app.CommitMultiStore().CacheMultiStore(), tmproto.Header{ChainID: testChainID, Height: openHeight}, false)

	reverted := 0
	for i, etx := range etxs {
		committed, err := twin.app.EvmKeeper.GetReceiptWithRetry(ctx, etx.Hash(), 5)
		require.NoError(t, err, "tx %d", i)
		rec := decoded[i]
		_, err = verify.ReceiptSignature(receipts[i], first.key.PublicKey())
		require.NoError(t, err)
		require.Equal(t, [32]byte(etx.Hash()), rec.ActivityID)
		require.Equal(t, int32(1-committed.Status), rec.ResultCode, "tx %d", i)
		if committed.Status == 0 {
			reverted++
		}
		body := rec.Effects[0].Body
		require.Equal(t, committed.GasUsed, binary.BigEndian.Uint64(body[0:8]), "tx %d gasUsed", i)
		require.Equal(t, committed.CumulativeGasUsed, binary.BigEndian.Uint64(body[8:16]), "tx %d cumulativeGasUsed", i)
		require.Equal(t, committed.EffectiveGasPrice, binary.BigEndian.Uint64(body[16:24]), "tx %d effectiveGasPrice", i)
		var contract common.Address
		if committed.To == "" {
			contract = common.HexToAddress(committed.ContractAddress)
		}
		require.Equal(t, contract[:], body[28:48], "tx %d contractAddress", i)
		require.Equal(t, uint32(len(committed.Logs)), binary.BigEndian.Uint32(body[104:108]))
		require.Len(t, rec.Effects, len(committed.Logs)+1)
		for j, log := range committed.Logs {
			want, err := sequencer.LogEffect(log)
			require.NoError(t, err)
			require.Equal(t, want, rec.Effects[j+1].Body, "tx %d log %d", i, j)
		}
	}
	require.Equal(t, 40, reverted)
}

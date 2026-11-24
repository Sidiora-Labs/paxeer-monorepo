package cmd

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	chainutils "github.com/Sidiora-Labs/Paxeer-X-Network/utils"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func applyCheckCommandHome(t *testing.T, prepare ...func(*app.TestWrapper, utils.TestAcct)) (string, string, string) {
	t.Helper()
	previous := app.EnableOCC
	app.EnableOCC = false
	t.Cleanup(func() { app.EnableOCC = previous })
	at := time.Unix(1_800_000_000, 0).UTC()
	validator := ed25519.GenPrivKey().PubKey()
	w := app.NewTestWrapper(t, at, validator, true, func(ba *baseapp.BaseApp) { ba.SetOccEnabled(false); ba.SetConcurrencyWorkers(1) })
	validatorAddress := sdk.ValAddress(validator.Address())
	delegation := sdk.NewCoin(w.App.StakingKeeper.BondDenom(w.Ctx), w.App.StakingKeeper.PowerReduction(w.Ctx).MulRaw(100))
	w.FundAcc(sdk.AccAddress(validatorAddress), sdk.NewCoins(delegation))
	_, err := staking.NewHandler(w.App.StakingKeeper)(w.Ctx, stakingtypes.NewMsgDelegate(sdk.AccAddress(validatorAddress), validatorAddress, delegation))
	require.NoError(t, err)
	path, err := w.App.UpgradeKeeper.GetUpgradeInfoPath()
	require.NoError(t, err)
	home := filepath.Dir(filepath.Dir(path))
	require.NoError(t, os.MkdirAll(filepath.Join(home, "config"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(home, "config", "genesis.json"), []byte(`{"chain_id":"pax-test"}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(home, app.ApplyCheckMarker), nil, 0o600))
	payer, recipient := utils.NewSigner(), utils.NewSigner()
	w.App.EvmKeeper.SetAddressMapping(w.Ctx, payer.AccountAddress, payer.EvmAddress)
	w.FundAcc(payer.AccountAddress, sdk.NewCoins(sdk.NewInt64Coin("uhpx", 10_000_000_000)))
	params := w.App.GetConsensusParams(w.Ctx)
	params.Block.MaxGasWanted = params.Block.MaxGas
	w.App.StoreConsensusParams(w.Ctx, params)
	for _, setup := range prepare {
		setup(w, payer)
	}
	_, err = w.App.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{Header: &tmproto.Header{ChainID: "pax-test", Height: 1, Time: at, ProposerAddress: validator.Address()}})
	require.NoError(t, err)
	_, err = w.App.Commit(context.Background())
	require.NoError(t, err)
	require.Len(t, w.App.StakingKeeper.GetLastValidators(w.App.GetCheckCtx()), 1)
	sign := func(nonce uint64) string {
		t.Helper()
		tx, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.LegacyTx{Nonce: nonce, GasPrice: big.NewInt(500_000_000_000), Gas: 100_000, To: &recipient.EvmAddress, Value: big.NewInt(1)}),
			ethtypes.LatestSignerForChainID(w.App.EvmKeeper.ChainID(w.App.GetCheckCtx())), payer.EvmPrivateKey)
		require.NoError(t, err)
		raw, err := tx.MarshalBinary()
		require.NoError(t, err)
		return "0x" + hex.EncodeToString(raw)
	}
	valid, refused := sign(0), sign(99)
	require.NoError(t, w.App.Close())
	return home, valid, refused
}

func TestApplyCheck(t *testing.T) {
	if encoded := os.Getenv("PAXEER_APPLYCHECK_COMMAND_ARGS"); encoded != "" {
		var args []string
		require.NoError(t, json.Unmarshal([]byte(encoded), &args))
		root, _ := NewRootCmd()
		root.SetArgs(args)
		require.NoError(t, root.Execute())
		return
	}
	root, _ := NewRootCmd()
	command, rest, err := root.Find([]string{"applycheck"})
	require.NoError(t, err)
	require.Empty(t, rest)
	require.Equal(t, "applycheck", command.Name())
	home, valid, refused := applyCheckCommandHome(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	run := func(args []string, want int) []byte {
		t.Helper()
		encoded, err := json.Marshal(append([]string{"applycheck"}, args...))
		require.NoError(t, err)
		process := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestApplyCheck$")
		process.Env = append(os.Environ(), "PAXEER_APPLYCHECK_COMMAND_ARGS="+string(encoded))
		output, err := process.CombinedOutput()
		code := 0
		if err != nil {
			exit, ok := err.(*exec.ExitError)
			require.True(t, ok, "%v: %s", err, output)
			code = exit.ExitCode()
		}
		require.Equal(t, want, code, "%s", output)
		return output
	}
	t.Run("marker", func(t *testing.T) {
		missing := t.TempDir()
		output := run([]string{"--home", missing, "--tx", valid}, 2)
		require.Contains(t, string(output), "applycheck-copy")
		entries, err := os.ReadDir(missing)
		require.NoError(t, err)
		require.Empty(t, entries)
	})
	t.Run("stand_in_credit_usage", func(t *testing.T) {
		token := "0x85FcD13735F4309833A503EE804ea32395851479"
		account := "0x0000000000000000000000000000000000001234"
		validCredit := token + ":" + account + ":1"
		for _, values := range [][]string{
			{"invalid"}, {token + ":" + account}, {"bad:" + account + ":1"}, {token + ":bad:1"},
			{token + ":" + account + ":"}, {token + ":" + account + ":-1"}, {token + ":" + account + ":1.5"},
			{token + ":" + account + ":1e6"}, {token + ":" + account + ":0"}, {token + ":" + account + ":0x10"},
			{token + ":" + account + ":" + new(big.Int).Lsh(big.NewInt(1), 256).String()},
			{validCredit, validCredit}, {validCredit, strings.ToLower(validCredit)},
		} {
			args := []string{"--home", filepath.Join(t.TempDir(), "unopened"), "--tx", valid}
			for _, value := range values {
				args = append(args, "--stand-in-credit", value)
			}
			require.Contains(t, string(run(args, 2)), "stand-in-credit")
		}
		missing := t.TempDir()
		require.Contains(t, string(run([]string{"--home", missing, "--tx", valid, "--stand-in-credit", validCredit}, 2)), "applycheck-copy")
		entries, err := os.ReadDir(missing)
		require.NoError(t, err)
		require.Empty(t, entries)
		help := run([]string{"--help"}, 0)
		require.Contains(t, string(help), "TOKEN:ACCOUNT:AMOUNT")
	})
	t.Run("empty", func(t *testing.T) {
		require.Contains(t, string(run([]string{"--home", home}, 2)), "at least one transaction")
	})
	t.Run("agrees", func(t *testing.T) {
		directory := t.TempDir()
		input, output := filepath.Join(directory, "transactions.txt"), filepath.Join(directory, "report.json")
		decoded, err := app.DecodeApplyCheckTransaction(valid)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(input, []byte(base64.StdEncoding.EncodeToString(decoded)+"\n"), 0o600))
		run([]string{"--home", home, "--tx-file", input, "--block-time", "2027-01-15T08:00:02Z", "--output", output}, 0)
		data, err := os.ReadFile(output)
		require.NoError(t, err)
		var report app.ApplyCheckReport
		require.NoError(t, json.Unmarshal(data, &report))
		require.True(t, report.Comparison.Equal, "%s", data)
		require.Equal(t, "pax-test", report.ChainID)
		require.Equal(t, int64(2), report.Height)
		require.Len(t, report.Forms, 3)
		require.Equal(t, "registered", report.ReceiptKeyMode)
		require.Equal(t, "follower_fast", report.Forms[1].Name)
		require.Contains(t, []string{"loaded", "absent"}, report.Forms[1].EngineLibrary)
		require.Equal(t, time.Date(2027, 1, 15, 8, 0, 2, 0, time.UTC), report.Time)
	})
	t.Run("stand_in_receipt_key", func(t *testing.T) {
		output := filepath.Join(t.TempDir(), "report.json")
		run([]string{"--home", home, "--tx", valid, "--stand-in-receipt-key", "--output", output}, 0)
		data, err := os.ReadFile(output)
		require.NoError(t, err)
		var report app.ApplyCheckReport
		require.NoError(t, json.Unmarshal(data, &report))
		require.True(t, report.Comparison.Equal, "%s", data)
		require.Equal(t, "stand-in", report.ReceiptKeyMode)
		require.Len(t, report.Forms, 3)
		require.Contains(t, []string{"loaded", "absent"}, report.Forms[1].EngineLibrary)
	})
	t.Run("differs", func(t *testing.T) {
		output := filepath.Join(t.TempDir(), "report.json")
		run([]string{"--home", home, "--tx", refused, "--output", output}, 1)
		data, err := os.ReadFile(output)
		require.NoError(t, err)
		var report app.ApplyCheckReport
		require.NoError(t, json.Unmarshal(data, &report))
		require.False(t, report.Comparison.Equal)
		require.Equal(t, "sequencer", report.Comparison.Form)
		require.Equal(t, 1, report.Comparison.Transaction)
	})
	t.Run("output_preserves_home", func(t *testing.T) {
		output := run([]string{"--home", home, "--tx", valid, "--output", filepath.Join(home, "report.json")}, 2)
		require.Contains(t, string(output), "outside the copied home")
		_, err := os.Stat(filepath.Join(home, "report.json"))
		require.True(t, os.IsNotExist(err))
	})
	t.Run("stdout", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		exitCode := 0
		cmd := newApplyCheckCmd(home, func(code int) { exitCode = code })
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{"--tx", valid})
		require.NoError(t, cmd.Execute())
		require.Zero(t, exitCode, "%s", stderr.String())
		var report app.ApplyCheckReport
		require.NoError(t, json.Unmarshal(stdout.Bytes(), &report))
		require.True(t, report.Comparison.Equal)
	})
}

func TestApplyCheckStandInCreditCommand(t *testing.T) {
	var native, trader common.Address
	contract := common.HexToAddress("0x85FcD13735F4309833A503EE804ea32395851479")
	home, valid, _ := applyCheckCommandHome(t, func(w *app.TestWrapper, payer utils.TestAcct) {
		trader = payer.EvmAddress
		require.NoError(t, w.App.EvmKeeper.RunWithOneOffEVMInstance(w.Ctx, func(evm *vm.EVM) error {
			var err error
			native, err = w.App.EvmKeeper.UpsertERCNativePointer(w.Ctx, evm, "ucheck", chainutils.ERCMetadata{Name: "Check token", Symbol: "CHECK", Decimals: 6})
			return err
		}, func(_, _ string) {}))
		raw, err := os.ReadFile("../../../precompiles/perps/testdata/usdl_runtime.hex")
		require.NoError(t, err)
		w.App.EvmKeeper.SetCode(w.Ctx, contract, common.FromHex(strings.TrimSpace(string(raw))))
		slot := crypto.Keccak256Hash(common.LeftPadBytes(trader.Bytes(), 32), make([]byte, 32))
		w.App.EvmKeeper.SetState(w.Ctx, contract, slot, common.BigToHash(big.NewInt(17)))
	})
	var stdout, stderr bytes.Buffer
	code := 0
	command := newApplyCheckCmd(home, func(exit int) { code = exit })
	command.SetOut(&stdout)
	command.SetErr(&stderr)
	command.SetArgs([]string{"--tx", valid, "--stand-in-receipt-key",
		"--stand-in-credit", fmt.Sprintf("%s:%s:9", contract, trader),
		"--stand-in-credit", fmt.Sprintf("%s:%s:23", native, trader),
		"--stand-in-credit", fmt.Sprintf("%s:%s:5", contract, native)})
	require.NoError(t, command.Execute())
	require.Zero(t, code, "%s", stderr.String())
	var report app.ApplyCheckReport
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &report))
	require.True(t, report.Comparison.Equal, "%s", stdout.String())
	require.Len(t, report.Forms, 3)
	for _, form := range report.Forms {
		require.Equal(t, report.Forms[0].StandInCredits, form.StandInCredits)
		require.Len(t, form.StandInCredits, 3)
		require.Equal(t, []app.ApplyCheckCreditReport{
			{Token: contract, Account: trader, Amount: "9", BalanceBefore: "17", BalanceAfter: "26", BalanceSource: "contract_storage", TotalSupplyMayBeUnchanged: true},
			{Token: native, Account: trader, Amount: "23", BalanceBefore: "0", BalanceAfter: "23", BalanceSource: "ledger"},
			{Token: contract, Account: native, Amount: "5", BalanceBefore: "0", BalanceAfter: "5", BalanceSource: "contract_storage", TotalSupplyMayBeUnchanged: true},
		}, form.StandInCredits)
		require.Equal(t, uint32(1), *form.Results[0].Status)
	}
}

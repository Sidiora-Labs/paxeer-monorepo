package app_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	perpskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/keeper"
	perpstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	pricefeedtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	seqkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/keeper"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	perpsabi "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/perps/abi"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	sdked25519 "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	paramtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	chainutils "github.com/Sidiora-Labs/Paxeer-X-Network/utils"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

type applyCheckFixture struct {
	home     string
	txs      [][]byte
	signed   []*ethtypes.Transaction
	commit   sdk.CommitID
	contract string
	time     time.Time
}

func newApplyCheckFixture(t *testing.T, active bool, prepare ...func(*app.TestWrapper, utils.TestAcct)) applyCheckFixture {
	t.Helper()
	previous := app.EnableOCC
	app.EnableOCC = false
	t.Cleanup(func() { app.EnableOCC = previous })
	at := time.Unix(1_800_000_000, 0).UTC()
	validator := sdked25519.GenPrivKey().PubKey()
	w := app.NewTestWrapper(t, at, validator, true, func(ba *baseapp.BaseApp) { ba.SetOccEnabled(false); ba.SetConcurrencyWorkers(1) })
	a := w.App
	validatorAddress := sdk.ValAddress(validator.Address())
	delegation := sdk.NewCoin(a.StakingKeeper.BondDenom(w.Ctx), a.StakingKeeper.PowerReduction(w.Ctx).MulRaw(100))
	w.FundAcc(sdk.AccAddress(validatorAddress), sdk.NewCoins(delegation))
	_, err := staking.NewHandler(a.StakingKeeper)(w.Ctx, stakingtypes.NewMsgDelegate(sdk.AccAddress(validatorAddress), validatorAddress, delegation))
	require.NoError(t, err)
	path, err := a.UpgradeKeeper.GetUpgradeInfoPath()
	require.NoError(t, err)
	home := filepath.Dir(filepath.Dir(path))
	payer, fresh, chain := utils.NewSigner(), utils.NewSigner(), utils.NewSigner()
	a.EvmKeeper.SetAddressMapping(w.Ctx, payer.AccountAddress, payer.EvmAddress)
	w.FundAcc(payer.AccountAddress, sdk.NewCoins(sdk.NewInt64Coin("uhpx", 10_000_000_000)))
	w.FundAcc(chain.AccountAddress, sdk.NewCoins(sdk.NewInt64Coin("uhpx", 10_000_000_000)))
	require.Nil(t, a.AccountKeeper.GetAccount(w.Ctx, sdk.AccAddress(fresh.EvmAddress[:])))
	key := append([]byte(a.EvmKeeper.Paramstore.Name()+"/"), evmtypes.KeyPaxSstoreSetGasEIP2200...)
	w.Ctx.KVStore(a.GetKey(paramtypes.StoreKey)).Delete(key)
	require.False(t, a.EvmKeeper.Paramstore.Has(w.Ctx, evmtypes.KeyPaxSstoreSetGasEIP2200))
	params := a.GetConsensusParams(w.Ctx)
	params.Block.MaxGasWanted = params.Block.MaxGas
	a.StoreConsensusParams(w.Ctx, params)
	require.NoError(t, os.MkdirAll(filepath.Join(home, "config"), 0o700))
	configuration := ""
	if active {
		pv, err := privval.GenFilePV(filepath.Join(home, "config", "priv_validator_key.json"), filepath.Join(home, "data", "priv_validator_state.json"), "")
		require.NoError(t, err)
		require.NoError(t, pv.Save())
		seed := bytes.Repeat([]byte{0x54}, ed25519.SeedSize)
		public := ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)
		require.NoError(t, a.SequencerKeeper.SetReceiptKeys(w.Ctx, seqkeeper.ReceiptKeys{{KeyId: bytes.Repeat([]byte{0x53}, 32), PublicKey: public, FirstHeight: 1, LastHeight: 100}}))
		require.NoError(t, a.SequencerKeeper.SetActivationHeight(w.Ctx, 2))
		a.UpgradeKeeper.SetDone(w.Ctx, app.MergeUpgrade)
		require.NoError(t, os.WriteFile(filepath.Join(home, "receipt-key"), []byte(hex.EncodeToString(seed)), 0o600))
		configuration = "[sequencer]\nreceipt-key-file = 'receipt-key'\n"
	}
	require.NoError(t, os.WriteFile(filepath.Join(home, "config", "app.toml"), []byte(configuration), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(home, "config", "genesis.json"), []byte(`{"chain_id":"pax-test"}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(home, app.ApplyCheckMarker), nil, 0o600))
	for _, setup := range prepare {
		setup(w, payer)
	}
	_, err = a.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{Header: &tmproto.Header{ChainID: "pax-test", Height: 1, Time: at, ProposerAddress: validator.Address()}})
	require.NoError(t, err)
	_, err = a.Commit(context.Background())
	require.NoError(t, err)
	require.Len(t, a.StakingKeeper.GetLastValidators(a.GetCheckCtx()), 1)
	require.False(t, a.EvmKeeper.Paramstore.Has(a.GetCheckCtx(), evmtypes.KeyPaxSstoreSetGasEIP2200))
	f := applyCheckFixture{home: home, commit: a.LastCommitID(), time: at}
	f.commit.Hash = bytes.Clone(f.commit.Hash)
	encode := func(signer utils.TestAcct, nonce uint64, to *[20]byte, data []byte, value *big.Int) []byte {
		t.Helper()
		var transaction ethtypes.LegacyTx
		transaction.Nonce, transaction.GasPrice, transaction.Gas, transaction.Value, transaction.Data = nonce, big.NewInt(500_000_000_000), 1_000_000, value, data
		if to != nil {
			address := signer.EvmAddress
			copy(address[:], to[:])
			transaction.To = &address
		}
		signed, err := ethtypes.SignTx(ethtypes.NewTx(&transaction), ethtypes.LatestSignerForChainID(a.EvmKeeper.ChainID(a.GetCheckCtx())), signer.EvmPrivateKey)
		require.NoError(t, err)
		raw, err := signed.MarshalBinary()
		require.NoError(t, err)
		encoded, err := app.DecodeApplyCheckTransaction("0x" + hex.EncodeToString(raw))
		require.NoError(t, err)
		f.signed = append(f.signed, signed)
		return encoded
	}
	freshAddress := [20]byte(fresh.EvmAddress)
	f.txs = append(f.txs, encode(payer, 0, &freshAddress, nil, big.NewInt(2_000_000_000_000_000_000)))
	constructor, err := hex.DecodeString("60016000556002600155600360025560006000a0600b6020600039600b6000f3600460035560006000a000")
	require.NoError(t, err)
	f.txs = append(f.txs, encode(payer, 1, nil, constructor, new(big.Int)))
	f.contract = crypto.CreateAddress(payer.EvmAddress, 1).Hex()
	destination := [20]byte(payer.EvmAddress)
	f.txs = append(f.txs, encode(fresh, 0, &destination, nil, big.NewInt(1)))
	plainCreation, err := hex.DecodeString("6001600c60003960016000f300")
	require.NoError(t, err)
	f.txs = append(f.txs, encode(payer, 2, nil, plainCreation, new(big.Int)))
	account := a.AccountKeeper.GetAccount(a.GetCheckCtx(), chain.AccountAddress)
	require.NotNil(t, account)
	txConfig := app.MakeEncodingConfig().TxConfig
	chainTx, err := app.GenTx(txConfig, []sdk.Msg{banktypes.NewMsgSend(chain.AccountAddress, payer.AccountAddress, sdk.NewCoins(sdk.NewInt64Coin("uhpx", 7)))},
		sdk.NewCoins(sdk.NewInt64Coin("uhpx", 100_000)), 200_000, "pax-test", []uint64{account.GetAccountNumber()}, []uint64{account.GetSequence()}, chain.PrivateKey)
	require.NoError(t, err)
	encoded, err := txConfig.TxEncoder()(chainTx)
	require.NoError(t, err)
	f.txs = append(f.txs, encoded)
	require.NoError(t, a.Close())
	return f
}

type applyCheckSeam struct {
	sequencer.ApplyCheck
	change func(*abci.ResponseFinalizeBlock)
}

func (s applyCheckSeam) Execute(ctx context.Context, a *app.App, req *abci.RequestFinalizeBlock) (*abci.ResponseFinalizeBlock, error) {
	response, err := s.ApplyCheck.Execute(ctx, a, req)
	if err == nil {
		s.change(response)
	}
	return response, err
}

func TestApplyCheck(t *testing.T) {
	for _, active := range []bool{false, true} {
		t.Run(fmt.Sprintf("sequencing_%t", active), func(t *testing.T) {
			fixture := newApplyCheckFixture(t, active)
			before := applyCheckHomeContents(t, fixture.home)
			report, err := app.ApplyCheckHome(context.Background(), fixture.home, fixture.txs, time.Time{}, sequencer.ApplyCheck{})
			require.NoError(t, err)
			encoded, err := json.Marshal(report)
			require.NoError(t, err)
			require.True(t, report.Comparison.Equal, "%s", encoded)
			require.Equal(t, "pax-test", report.ChainID)
			require.Equal(t, int64(2), report.Height)
			require.Equal(t, fixture.time.Add(time.Second), report.Time)
			require.Equal(t, fixture.commit.Version, report.CommittedHeight)
			require.Equal(t, hex.EncodeToString(fixture.commit.Hash), report.CommittedAppHash)
			require.Len(t, report.Forms, 3)
			for _, check := range []struct {
				field  string
				mutate func(*app.ApplyCheckResult)
			}{
				{"code", func(r *app.ApplyCheckResult) { r.Code++ }},
				{"codespace", func(r *app.ApplyCheckResult) { r.Codespace = "different" }},
				{"gas_wanted", func(r *app.ApplyCheckResult) { r.GasWanted++ }},
				{"gas_used", func(r *app.ApplyCheckResult) { r.GasUsed++ }},
				{"events[0].type", func(r *app.ApplyCheckResult) { r.Events[0].Type = "different" }},
				{"status", func(r *app.ApplyCheckResult) { *r.Status = 0 }},
				{"logs[0].index", func(r *app.ApplyCheckResult) { r.Logs[0].Index++ }},
				{"contract_address", func(r *app.ApplyCheckResult) { r.ContractAddress = "different" }},
				{"receipt.gas_used", func(r *app.ApplyCheckResult) { r.Receipt.GasUsed++ }},
			} {
				var changed app.ApplyCheckReport
				require.NoError(t, json.Unmarshal(encoded, &changed))
				check.mutate(&changed.Forms[1].Results[1])
				comparison := app.CompareApplyCheck(changed.Forms)
				require.False(t, comparison.Equal)
				require.Equal(t, 2, comparison.Transaction)
				require.Equal(t, check.field, comparison.Field)
			}
			for _, form := range report.Forms {
				if form.Name == "follower_fast" {
					require.Contains(t, []string{"loaded", "absent"}, form.EngineLibrary)
				} else {
					require.Empty(t, form.EngineLibrary)
				}
				require.Empty(t, form.Error)
				require.Empty(t, form.Panic)
				require.NotEmpty(t, form.AppHash)
				require.Len(t, form.Results, 5)
				for index, result := range form.Results {
					require.Zero(t, result.Code, "transaction %d", index+1)
					require.Positive(t, result.GasWanted)
					require.Positive(t, result.GasUsed)
					require.NotEmpty(t, result.Events)
					if index < len(fixture.signed) {
						require.Empty(t, result.ReceiptError)
						require.NotNil(t, result.Receipt)
						require.NotNil(t, result.Status)
						require.Equal(t, uint32(1), *result.Status)
						require.Equal(t, fixture.signed[index].Hash().Hex(), result.Receipt.TxHashHex)
					} else {
						require.Nil(t, result.Receipt)
					}
				}
				require.Equal(t, fixture.contract, form.Results[1].ContractAddress)
				require.Len(t, form.Results[1].Logs, 1)
			}
			repeated, err := app.ApplyCheckHome(context.Background(), fixture.home, fixture.txs, time.Time{}, sequencer.ApplyCheck{})
			require.NoError(t, err)
			require.Equal(t, report, repeated)
			require.Equal(t, before, applyCheckHomeContents(t, fixture.home))
			reopened := app.New(dbm.NewMemDB(), nil, true, map[int64]bool{}, fixture.home, 0, true, tmconfig.TestConfig(), app.MakeEncodingConfig(), nil, app.TestAppOpts{}, app.EmptyWasmOpts, nil)
			require.Equal(t, fixture.commit, reopened.LastCommitID())
			require.Equal(t, int64(1), reopened.LastBlockHeight())
			require.NoError(t, reopened.Close())
			seam := applyCheckSeam{change: func(response *abci.ResponseFinalizeBlock) {
				response.TxResults[len(response.TxResults)-len(fixture.txs)+1].GasWanted++
			}}
			different, err := app.ApplyCheckHome(context.Background(), fixture.home, fixture.txs, fixture.time.Add(2*time.Second), seam)
			require.NoError(t, err)
			require.False(t, different.Comparison.Equal)
			require.Equal(t, 2, different.Comparison.Transaction)
			require.Equal(t, "gas_wanted", different.Comparison.Field)
			seam.change = func(*abci.ResponseFinalizeBlock) { panic("applycheck execution panic") }
			panicked, err := app.ApplyCheckHome(context.Background(), fixture.home, fixture.txs, time.Time{}, seam)
			require.NoError(t, err)
			require.False(t, panicked.Comparison.Equal)
			require.Equal(t, "panic", panicked.Comparison.Field)
			require.Equal(t, "sequencer", panicked.Comparison.Form)
			require.Equal(t, "applycheck execution panic", panicked.Forms[0].Panic)
		})
	}
}

func applyCheckHomeContents(t *testing.T, home string) map[string]string {
	t.Helper()
	contents := make(map[string]string)
	require.NoError(t, filepath.WalkDir(home, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(home, path)
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			contents[rel] = link
			return err
		}
		data, err := os.ReadFile(path)
		contents[rel] = hex.EncodeToString(data)
		return err
	}))
	return contents
}

func TestApplyCheckStoreLock(t *testing.T) {
	if home := os.Getenv("PAXEER_APPLYCHECK_LOCK_HOME"); home != "" {
		a := app.New(dbm.NewMemDB(), nil, true, map[int64]bool{}, home, 0, true, tmconfig.TestConfig(), app.MakeEncodingConfig(), nil, app.TestAppOpts{}, app.EmptyWasmOpts, nil)
		fmt.Println("applycheck-store-open")
		_, err := io.Copy(io.Discard, os.Stdin)
		require.NoError(t, err)
		require.NoError(t, a.Close())
		return
	}
	fixture := newApplyCheckFixture(t, false)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	child := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestApplyCheckStoreLock$")
	child.Env = append(os.Environ(), "PAXEER_APPLYCHECK_LOCK_HOME="+fixture.home)
	stdout, err := child.StdoutPipe()
	require.NoError(t, err)
	stdin, err := child.StdinPipe()
	require.NoError(t, err)
	child.Stderr = os.Stderr
	require.NoError(t, child.Start())
	t.Cleanup(func() { _ = stdin.Close(); _ = child.Wait() })
	scanner := bufio.NewScanner(stdout)
	opened := false
	for scanner.Scan() {
		if scanner.Text() == "applycheck-store-open" {
			opened = true
			break
		}
	}
	require.NoError(t, scanner.Err())
	require.True(t, opened)
	_, err = app.ApplyCheckHome(context.Background(), fixture.home, fixture.txs, time.Time{}, sequencer.ApplyCheck{})
	require.ErrorContains(t, err, "stores are held by another process")
	require.NoError(t, stdin.Close())
	_, err = io.Copy(io.Discard, stdout)
	require.NoError(t, err)
}

func TestApplyCheckInput(t *testing.T) {
	fixture := newApplyCheckFixture(t, false)
	chainTransaction := fixture.txs[len(fixture.txs)-1]
	chain := base64.StdEncoding.EncodeToString(chainTransaction)
	decoded, err := app.DecodeApplyCheckTransaction(chain)
	require.NoError(t, err)
	require.Equal(t, chainTransaction, decoded)
	for _, value := range []string{"", "invalid", "0xdeadbeef", base64.StdEncoding.EncodeToString([]byte{1, 2, 3})} {
		_, err := app.DecodeApplyCheckTransaction(value)
		require.Error(t, err)
	}
	_, err = app.ApplyCheckHome(context.Background(), fixture.home, nil, time.Time{}, sequencer.ApplyCheck{})
	require.ErrorContains(t, err, "at least one transaction")
	_, err = app.ApplyCheckHome(context.Background(), fixture.home, fixture.txs, fixture.time, sequencer.ApplyCheck{})
	require.ErrorContains(t, err, "after the last committed block")
}

func newApplyCheckCollateralFixture(t *testing.T, active, native, unregistered bool) (applyCheckFixture, app.ApplyCheckCredit) {
	t.Helper()
	var trader utils.TestAcct
	token := common.HexToAddress(perpstypes.DefaultCollateralToken)
	fixture := newApplyCheckFixture(t, active, func(w *app.TestWrapper, payer utils.TestAcct) {
		a, ctx := w.App, w.Ctx
		trader = payer
		if native {
			require.NoError(t, a.EvmKeeper.RunWithOneOffEVMInstance(ctx, func(evm *vm.EVM) error {
				var err error
				token, err = a.EvmKeeper.UpsertERCNativePointer(ctx, evm, "ucheck", chainutils.ERCMetadata{Name: "Check collateral", Symbol: "CHECK", Decimals: 6})
				return err
			}, func(_, _ string) {}))
			if unregistered {
				_, version, exists := a.EvmKeeper.GetERC20NativePointer(ctx, "ucheck")
				require.True(t, exists)
				a.EvmKeeper.DeleteERC20NativePointer(ctx, "ucheck", version)
			}
		} else {
			raw, err := os.ReadFile("../precompiles/perps/testdata/usdl_runtime.hex")
			require.NoError(t, err)
			a.EvmKeeper.SetCode(ctx, token, common.FromHex(strings.TrimSpace(string(raw))))
		}
		params := perpstypes.DefaultParams()
		params.CollateralToken = token
		require.NoError(t, a.PerpsKeeper.UpdateParams(ctx, a.PerpsKeeper.Authority(), params))
		require.NoError(t, a.PerpsKeeper.SetMarket(ctx, a.PerpsKeeper.Authority(), perpstypes.Market{
			ID: 0, Symbol: "BTC", Name: "Bitcoin", OracleSymbol: "BTC", SzDecimals: 5, PxDecimals: 1,
			MaxLeverage: 1000, TierClass: perpstypes.TierClassA, ImpactNotionalUsd: sdk.NewIntWithDecimal(20_000, 18),
			OiCapUsd: sdk.NewIntWithDecimal(500_000_000, 18), FundingCapPpmPerHour: 40_000, Enabled: true,
		}))
		aggregate, err := json.Marshal(pricefeedtypes.Aggregate{IndexPrice: sdk.NewIntWithDecimal(60_000, 18), MarkPrice: sdk.NewIntWithDecimal(60_000, 18),
			Basis: sdk.ZeroInt(), TimestampMs: uint64(ctx.BlockTime().UnixMilli()), Height: 1, Sources: 1})
		require.NoError(t, err)
		ctx.KVStore(a.GetKey(pricefeedtypes.StoreKey)).Set(pricefeedtypes.AggregateKey(0), aggregate)
		_, err = a.PerpsKeeper.UpdatePrices(ctx)
		require.NoError(t, err)
		maker := utils.NewSigner().EvmAddress
		_, err = a.PerpsKeeper.Deposit(ctx, maker, *uint256.NewInt(100_000_000_000))
		require.NoError(t, err)
		module := a.PerpsKeeper.ModuleEVMAddress()
		if native {
			funds := sdk.NewCoins(sdk.NewInt64Coin("ucheck", 100_000_000_000))
			require.NoError(t, a.BankKeeper.MintCoins(ctx, "evm", funds))
			require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, "evm", a.EvmKeeper.GetPaxAddressOrDefault(ctx, module), funds))
		} else {
			a.EvmKeeper.SetState(ctx, token, gigaPerpsTokenSlot(module, 0), common.BigToHash(big.NewInt(100_000_000_000)))
		}
		_, err = a.PerpsKeeper.PlaceOrder(ctx, maker, perpskeeper.OrderParams{MarketID: 0, IsLong: false,
			OrderType: uint8(perpstypes.OrderTypeLimit), TimeInForce: uint8(perpstypes.TimeInForceGTC),
			SizeUsd: gigaPerpsWord(gigaPerpsUSD(200_000)), Leverage: gigaPerpsWord(gigaPerpsUSD(20)), LimitPrice: gigaPerpsWord(gigaPerpsUSD(60_100))})
		require.NoError(t, err)
		// The trader holds gas currency but starts without collateral.
		data := append(crypto.Keccak256([]byte("balanceOf(address)"))[:4], common.LeftPadBytes(trader.EvmAddress.Bytes(), 32)...)
		held, err := a.EvmKeeper.StaticCallEVM(ctx, trader.AccountAddress, &token, data)
		require.NoError(t, err)
		require.Len(t, held, 32)
		require.Zero(t, new(big.Int).SetBytes(held).Sign())
	})
	parsed, err := perpsabi.ABI()
	require.NoError(t, err)
	tokenABI, err := abi.JSON(strings.NewReader(`[{"type":"function","name":"approve","inputs":[{"type":"address"},{"type":"uint256"}],"outputs":[{"type":"bool"}]}]`))
	require.NoError(t, err)
	approve, err := tokenABI.Pack("approve", gigaPerpsAddress, big.NewInt(1_000_000_000))
	require.NoError(t, err)
	deposit, err := parsed.Pack("deposit", token, big.NewInt(1_000_000_000))
	require.NoError(t, err)
	order := gigaPerpsOrder{MarketId: new(big.Int), IsLong: true, OrderType: uint8(perpstypes.OrderTypeMarket),
		TimeInForce: uint8(perpstypes.TimeInForceIOC), SizeUsd: gigaPerpsUSD(600), Leverage: gigaPerpsUSD(10), LimitPrice: new(big.Int),
		TriggerPrice: new(big.Int), AcceptablePrice: gigaPerpsUSD(60_300), CollateralAmount: new(big.Int), TakeProfitPrice: new(big.Int), StopLossPrice: new(big.Int)}
	place, err := parsed.Pack("placeOrder", order)
	require.NoError(t, err)
	chainID := fixture.signed[0].ChainId()
	fixture.txs, fixture.signed = nil, nil
	for i, call := range []struct {
		to   common.Address
		data []byte
	}{{token, approve}, {gigaPerpsAddress, deposit}, {gigaPerpsAddress, place}} {
		signed, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.LegacyTx{Nonce: uint64(i), GasPrice: big.NewInt(500_000_000_000), Gas: gigaPerpsGas,
			To: &call.to, Value: new(big.Int), Data: call.data}), ethtypes.LatestSignerForChainID(chainID), trader.EvmPrivateKey)
		require.NoError(t, err)
		raw, err := signed.MarshalBinary()
		require.NoError(t, err)
		encoded, err := app.DecodeApplyCheckTransaction(hex.EncodeToString(raw))
		require.NoError(t, err)
		fixture.txs = append(fixture.txs, encoded)
		fixture.signed = append(fixture.signed, signed)
	}
	return fixture, app.ApplyCheckCredit{Token: token, Account: trader.EvmAddress, Amount: big.NewInt(1_000_000_000)}
}

func TestApplyCheckStandInCredit(t *testing.T) {
	for _, active := range []bool{false, true} {
		for _, native := range []bool{false, true} {
			t.Run(fmt.Sprintf("sequencing_%t_native_%t", active, native), func(t *testing.T) {
				fixture, credit := newApplyCheckCollateralFixture(t, active, native, false)
				before := applyCheckHomeContents(t, fixture.home)
				run := func(credits []app.ApplyCheckCredit) *app.ApplyCheckReport {
					key, err := sequencer.NewApplyCheckReceiptKey()
					require.NoError(t, err)
					report, err := app.ApplyCheckHomeWithOptions(context.Background(), fixture.home, fixture.txs, time.Time{},
						sequencer.ApplyCheck{ReceiptKey: key}, app.ApplyCheckOptions{StandInReceiptKey: true, StandInCredits: credits})
					require.NoError(t, err)
					data, err := json.Marshal(report)
					require.NoError(t, err)
					require.True(t, report.Comparison.Equal, "%s", data)
					require.Len(t, report.Forms, 3)
					return report
				}
				refused := run(nil)
				for _, form := range refused.Forms {
					require.Len(t, form.Results, 3)
					require.Equal(t, uint32(1), *form.Results[0].Status)
					require.Equal(t, uint32(0), *form.Results[1].Status)
					require.Equal(t, uint32(0), *form.Results[2].Status)
					require.Empty(t, form.StandInCredits)
				}
				credited := run([]app.ApplyCheckCredit{credit})
				for _, form := range credited.Forms {
					require.Empty(t, form.Error)
					require.Empty(t, form.Panic)
					require.Len(t, form.Results, 3)
					for _, result := range form.Results {
						require.Zero(t, result.Code)
						require.Empty(t, result.ReceiptError)
						require.NotNil(t, result.Status)
						require.Equal(t, uint32(1), *result.Status)
					}
					require.Len(t, form.StandInCredits, 1)
					got := form.StandInCredits[0]
					require.Equal(t, credit.Token, got.Token)
					require.Equal(t, credit.Account, got.Account)
					require.Equal(t, credit.Amount.String(), got.Amount)
					require.Equal(t, "0", got.BalanceBefore)
					require.Equal(t, credit.Amount.String(), got.BalanceAfter)
					require.Equal(t, !native, got.TotalSupplyMayBeUnchanged)
					require.Equal(t, map[bool]string{true: "ledger", false: "contract_storage"}[native], got.BalanceSource)
					require.Equal(t, credited.Forms[0].StandInCredits, form.StandInCredits)
					require.Equal(t, credited.Forms[0].AppHash, form.AppHash)
					require.NotEqual(t, refused.Forms[0].AppHash, form.AppHash)
				}
				require.Equal(t, before, applyCheckHomeContents(t, fixture.home))
				require.NoError(t, os.Remove(filepath.Join(fixture.home, app.ApplyCheckMarker)))
				_, err := app.ApplyCheckHomeWithOptions(context.Background(), fixture.home, fixture.txs, time.Time{}, sequencer.ApplyCheck{}, app.ApplyCheckOptions{StandInCredits: []app.ApplyCheckCredit{credit}})
				require.ErrorContains(t, err, "regular marker file")
			})
		}
	}
}

func TestApplyCheckStandInCreditStorageNotFound(t *testing.T) {
	fixture, credit := newApplyCheckCollateralFixture(t, false, true, true)
	before := applyCheckHomeContents(t, fixture.home)
	report, err := app.ApplyCheckHomeWithOptions(context.Background(), fixture.home, fixture.txs, time.Time{}, sequencer.ApplyCheck{}, app.ApplyCheckOptions{StandInCredits: []app.ApplyCheckCredit{credit}})
	require.ErrorContains(t, err, credit.Token.Hex())
	require.ErrorContains(t, err, "balance storage not found")
	require.Nil(t, report)
	require.Equal(t, before, applyCheckHomeContents(t, fixture.home))
	reopened := app.New(dbm.NewMemDB(), nil, true, map[int64]bool{}, fixture.home, 0, true, tmconfig.TestConfig(), app.MakeEncodingConfig(), nil, app.TestAppOpts{}, app.EmptyWasmOpts, nil)
	require.Equal(t, fixture.commit, reopened.LastCommitID())
	require.Zero(t, reopened.EvmKeeper.GetNonce(reopened.GetCheckCtx(), credit.Account))
	require.NoError(t, reopened.Close())
}

func TestApplyCheckCreditValidation(t *testing.T) {
	token := common.HexToAddress(perpstypes.DefaultCollateralToken)
	account := common.HexToAddress("0x0000000000000000000000000000000000001234")
	for _, amount := range []*big.Int{nil, new(big.Int), big.NewInt(-1), new(big.Int).Lsh(big.NewInt(1), 256)} {
		_, err := app.ApplyCheckHomeWithOptions(context.Background(), filepath.Join(t.TempDir(), "unopened"), [][]byte{{1}}, time.Time{}, sequencer.ApplyCheck{}, app.ApplyCheckOptions{
			StandInCredits: []app.ApplyCheckCredit{{Token: token, Account: account, Amount: amount}},
		})
		require.ErrorContains(t, err, "positive uint256")
	}
	credit := fmt.Sprintf("%s:%s:1", token, account)
	_, err := app.ParseApplyCheckCredits([]string{credit, strings.ToLower(credit)})
	require.ErrorContains(t, err, "repeated")
	parsed, err := app.ParseApplyCheckCredits([]string{credit})
	require.NoError(t, err)
	require.Equal(t, []app.ApplyCheckCredit{{Token: token, Account: account, Amount: big.NewInt(1)}}, parsed)
}

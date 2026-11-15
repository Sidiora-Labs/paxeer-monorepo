package app_test

import (
	"math/big"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func concurrentFirstUse(t *testing.T, order []string, reverts bool) {
	t.Helper()
	keys := sequencedExecutionSigningKeys(t)
	h := newSequencedExecutionChain(t, keys, 0, false)
	initial := h.initialContext()
	first, second, established := utils.NewSigner(), utils.NewSigner(), utils.NewSigner()
	funds := sdk.NewCoins(sdk.NewInt64Coin("uhpx", 1_000_000_000_000_000), sdk.NewInt64Coin("uusdc", 123_456))
	for _, sender := range []utils.TestAcct{first, second} {
		cast := sdk.AccAddress(sender.EvmAddress[:])
		require.False(t, cast.Equals(sender.AccountAddress))
		require.NoError(t, h.a.BankKeeper.MintCoins(initial, "mint", funds))
		require.NoError(t, h.a.BankKeeper.SendCoinsFromModuleToAccount(initial, "mint", cast, funds))
		_, associated := h.a.EvmKeeper.GetEVMAddress(initial, sender.AccountAddress)
		require.False(t, associated)
	}
	sequencedExecutionFund(t, h.a, initial, established)
	recipient := common.HexToAddress("0xbeef")
	contract := common.HexToAddress("0xabcdef")
	h.a.EvmKeeper.SetCode(initial, contract, []byte{0x60, 0, 0x60, 0, 0xfd})
	h.commitInitial(t, false)
	newContext := func() sdk.Context {
		ctx := sdk.NewContext(h.a.CommitMultiStore().CacheMultiStore(), tmproto.Header{
			ChainID: chainsBlockChainID, Height: 2, Time: sequencedExecutionTime.Add(time.Second), ProposerAddress: keys.proposer,
		}, false)
		return ctx.WithConsensusParams(h.a.GetConsensusParams(ctx))
	}
	ordinary, concurrent := newContext(), newContext()
	to := recipient
	if reverts {
		to = contract
	}
	firstTx, _ := sequencedExecutionTx(t, first, big.NewInt(125), false, 0, 100_000, to, nil)
	secondTx, _ := sequencedExecutionTx(t, second, big.NewInt(125), true, 0, 100_000, recipient, nil)
	establishedTx, _ := sequencedExecutionTx(t, established, big.NewInt(125), true, 0, 21_000, recipient, nil)
	cosmos := revocationAnteTx(t, h.a, h.a.GetTxConfig(), ordinary, established.PrivateKey, &banktypes.MsgSend{
		FromAddress: established.AccountAddress.String(), ToAddress: sdk.AccAddress(recipient[:]).String(),
		Amount: sdk.NewCoins(sdk.NewInt64Coin("uhpx", 7)),
	})
	cosmosTx, err := h.a.GetTxConfig().TxEncoder()(cosmos)
	require.NoError(t, err)
	available := map[string][]byte{"first": firstTx, "second": secondTx, "evm": establishedTx, "cosmos": cosmosTx}
	var txs [][]byte
	for _, name := range order {
		txs = append(txs, available[name])
	}
	decode := func() []sdk.Tx {
		typed := make([]sdk.Tx, len(txs))
		for i, raw := range txs {
			var err error
			typed[i], err = h.a.GetTxConfig().TxDecoder()(raw)
			require.NoError(t, err)
		}
		return typed
	}
	for _, ctx := range []sdk.Context{ordinary, concurrent} {
		_, associated := h.a.EvmKeeper.GetEVMAddress(ctx, first.AccountAddress)
		require.False(t, associated)
		require.Equal(t, funds, h.a.BankKeeper.GetAllBalances(ctx, sdk.AccAddress(first.EvmAddress[:])))
	}
	want := h.a.ProcessTxsSynchronousV2(ordinary, txs, decode(), 0)
	require.True(t, h.a.GigaExecutorEnabled)
	require.True(t, h.a.GigaOCCEnabled)
	got, _, err := h.a.ExecuteTxsConcurrently(concurrent, txs, decode(), 0)
	require.NoError(t, err)
	require.Len(t, got, len(txs))
	require.Equal(t, want, got)
	wantCommitment, err := abci.MarshalTxResults(want)
	require.NoError(t, err)
	gotCommitment, err := abci.MarshalTxResults(got)
	require.NoError(t, err)
	require.Equal(t, wantCommitment, gotCommitment)
	for i, result := range got {
		require.NotEqual(t, "giga-infrastructure", result.Codespace)
		require.Positive(t, result.GasUsed)
		require.Equal(t, want[i].GasWanted, result.GasWanted)
		require.Equal(t, want[i].GasUsed, result.GasUsed)
		if order[i] == "first" && reverts {
			require.Equal(t, sdkerrors.ErrEVMVMError.ABCICode(), result.Code)
			require.NotNil(t, result.EvmTxInfo)
			require.Contains(t, result.EvmTxInfo.VmError, vm.ErrExecutionReverted.Error())
		} else {
			require.Zero(t, result.Code, "%s: %s", order[i], result.Log)
		}
	}
	require.Equal(t, sequencedExecutionStores(t, ordinary), sequencedExecutionStores(t, concurrent))
	for i, name := range order {
		var sender utils.TestAcct
		switch name {
		case "first":
			sender = first
		case "second":
			sender = second
		default:
			continue
		}
		mapped, associated := h.a.EvmKeeper.GetEVMAddress(concurrent, sender.AccountAddress)
		require.True(t, associated)
		require.Equal(t, sender.EvmAddress, mapped)
		require.True(t, h.a.BankKeeper.GetAllBalances(concurrent, sdk.AccAddress(sender.EvmAddress[:])).IsZero())
		require.Equal(t, funds.AmountOf("uusdc"), h.a.BankKeeper.GetBalance(concurrent, sender.AccountAddress, "uusdc").Amount)
		require.Equal(t, uint64(1), h.a.EvmKeeper.GetNonce(concurrent, sender.EvmAddress), "transaction %d", i)
	}
}

func TestConcurrentFirstUseAlone(t *testing.T) {
	concurrentFirstUse(t, []string{"first"}, false)
}

func TestConcurrentFirstUseAfterEVM(t *testing.T) {
	concurrentFirstUse(t, []string{"evm", "first"}, false)
}

func TestConcurrentFirstUseBeforeCosmos(t *testing.T) {
	concurrentFirstUse(t, []string{"first", "cosmos"}, false)
}

func TestConcurrentFirstUseAfterCosmos(t *testing.T) {
	concurrentFirstUse(t, []string{"cosmos", "first"}, false)
}

func TestConcurrentFirstUseTwoSenders(t *testing.T) {
	concurrentFirstUse(t, []string{"first", "second"}, false)
}

func TestConcurrentFirstUseRevert(t *testing.T) {
	concurrentFirstUse(t, []string{"first"}, true)
}

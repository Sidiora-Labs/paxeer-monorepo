package sequencer_test

import (
	"fmt"
	"math/big"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/config"
	chainstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/stretchr/testify/require"
)

func admissionParams(maxGas, maxWanted, maxBytes int64) func(sdk.Context, *app.App) {
	return func(ctx sdk.Context, a *app.App) {
		cp := *app.DefaultConsensusParams
		cp.Block = &tmproto.BlockParams{MaxGas: maxGas, MaxGasWanted: maxWanted, MaxBytes: maxBytes}
		a.StoreConsensusParams(ctx, &cp)
	}
}

func admissionTx0(t *testing.T) []byte {
	t.Helper()
	msg := signSealBatch(t, sequencer.SequenceBatchFields{
		Height: uint64(openHeight), FromSeq: 1, ToSeq: 1,
	}, testKeySeed, testKeyID)
	bz, err := app.EncodeSequenceBatch(msg)
	require.NoError(t, err)
	return bz
}

func admissionChainID(t *testing.T, chainID int64) {
	t.Helper()
	previous, existed := config.ChainIDMapping[testChainID]
	config.ChainIDMapping[testChainID] = chainID
	t.Cleanup(func() {
		if existed {
			config.ChainIDMapping[testChainID] = previous
		} else {
			delete(config.ChainIDMapping, testChainID)
		}
	})
}

func newAllowanceHarness(t *testing.T, maxGas int64, domains map[uint32]chainstypes.Domain, funded ...utils.TestAcct) *sourceHarness {
	t.Helper()
	return newSourceHarness(t, openHeight, func(ctx sdk.Context, a *app.App) {
		admissionParams(maxGas, maxGas, 200000)(ctx, a)
		genesis := chainstypes.DefaultGenesis()
		genesis.Params.MinGasAllowance = 2
		genesis.Params.MinByteAllowance = 1
		for id, domain := range domains {
			domain.Id = id
			domain.Name = fmt.Sprintf("chain-%d", id)
			domain.TargetGasPerBlock = domain.GasAllowance / 2
			domain.Bond = sdk.ZeroInt()
			genesis.Domains = append(genesis.Domains, domain)
			genesis.NextBaseFees[id] = sdk.ZeroDec()
			if id >= genesis.NextID {
				genesis.NextID = id + 1
			}
		}
		require.NoError(t, a.LayerXChainsKeeper.InitGenesis(ctx, *genesis))
	}, funded...)
}

func TestAcceptRefusesPastChainGasAllowance(t *testing.T) {
	t.Run("usage and delivery rollback", func(t *testing.T) {
		chainID := chainstypes.ChainIDOf(7).Int64()
		admissionChainID(t, 125)
		sender, other, recipient := utils.NewSigner(), utils.NewSigner(), utils.NewSigner()
		h := newAllowanceHarness(t, 20*transferGas, map[uint32]chainstypes.Domain{
			7: {Status: chainstypes.StatusActive, GasAllowance: 2 * transferGas, ByteAllowance: 200000},
			8: {Status: chainstypes.StatusActive, GasAllowance: transferGas, ByteAllowance: 200000},
		}, sender, other)
		h.open()
		_, gap := signEVM(t, sender, chainID, 1, recipient.EvmAddress, big.NewInt(1))
		_, err := h.exec.Accept(gap)
		requireRefused(t, err, sdkerrors.ErrWrongSequence)
		var accepted [][]byte
		for nonce := uint64(0); nonce < 2; nonce++ {
			signed, bz := signEVM(t, sender, chainID, nonce, recipient.EvmAddress, big.NewInt(1))
			require.Equal(t, chainstypes.ChainIDOf(7), signed.ChainId())
			acc, err := h.exec.Accept(bz)
			require.NoError(t, err)
			require.Equal(t, int64(transferGas), acc.Result.GasWanted)
			accepted = append(accepted, bz)
			if nonce == 0 {
				_, gap := signEVM(t, sender, chainID, 2, recipient.EvmAddress, big.NewInt(1))
				_, err := h.exec.Accept(gap)
				requireRefused(t, err, sdkerrors.ErrWrongSequence)
			}
		}
		before := h.weiBalance(h.openCtx(), sender.EvmAddress)
		_, next := signEVM(t, sender, chainID, 2, recipient.EvmAddress, big.NewInt(1))
		acc, err := h.exec.Accept(next)
		require.Nil(t, acc)
		require.ErrorIs(t, err, sequencer.ErrBlockFull)
		require.ErrorContains(t, err, fmt.Sprintf("chain %d gas wanted %d exceeds its allowance %d", 7, 3*transferGas, 2*transferGas))
		requireWei(t, before, h.weiBalance(h.openCtx(), sender.EvmAddress))
		require.Equal(t, uint64(2), h.app.EvmKeeper.GetNonce(h.openCtx(), sender.EvmAddress))
		_, bz := signEVM(t, other, chainstypes.ChainIDOf(8).Int64(), 0, recipient.EvmAddress, big.NewInt(1))
		_, err = h.exec.Accept(bz)
		require.NoError(t, err)
		accepted = append(accepted, bz)
		batch, err := h.exec.Seal(openHeight)
		require.NoError(t, err)
		require.Equal(t, accepted, batch.Txs)
		require.Equal(t, sourceLastSeq+3, batch.LastSeq)
	})
	t.Run("unknown and inactive", func(t *testing.T) {
		admissionChainID(t, 125)
		sender, recipient := utils.NewSigner(), utils.NewSigner()
		domains := map[uint32]chainstypes.Domain{}
		for id, status := range []uint8{chainstypes.StatusPaused, chainstypes.StatusGovernancePaused, chainstypes.StatusRetired} {
			domains[uint32(id+1)] = chainstypes.Domain{Status: status, GasAllowance: transferGas, ByteAllowance: 200000}
		}
		h := newAllowanceHarness(t, 20*transferGas, domains, sender)
		h.open()
		before := h.weiBalance(h.openCtx(), sender.EvmAddress)
		for _, chainID := range []int64{126, int64(chainstypes.VirtualChainIDBase), chainstypes.ChainIDOf(1).Int64(), chainstypes.ChainIDOf(2).Int64(), chainstypes.ChainIDOf(3).Int64(), chainstypes.ChainIDOf(999).Int64()} {
			_, bz := signEVM(t, sender, chainID, 0, recipient.EvmAddress, big.NewInt(1))
			acc, err := h.exec.Accept(bz)
			require.Nil(t, acc)
			require.ErrorIs(t, err, sequencer.ErrNotAdmitted)
			if domainID, virtual := chainstypes.DomainOfChainID(big.NewInt(chainID)); virtual && domains[domainID].Status != 0 {
				require.ErrorIs(t, err, chainstypes.ErrDomainNotActive)
				require.NotErrorIs(t, err, sdkerrors.ErrInvalidChainID)
			} else {
				require.ErrorIs(t, err, sdkerrors.ErrInvalidChainID)
				require.NotErrorIs(t, err, chainstypes.ErrDomainNotActive)
			}
			require.NotErrorIs(t, err, sequencer.ErrBlockFull)
			require.ErrorContains(t, err, fmt.Sprintf("chain %d", chainID))
		}
		requireWei(t, before, h.weiBalance(h.openCtx(), sender.EvmAddress))
		require.Zero(t, h.app.EvmKeeper.GetNonce(h.openCtx(), sender.EvmAddress))
		batch, err := h.exec.Seal(openHeight)
		require.NoError(t, err)
		require.Empty(t, batch.Txs)
	})
	t.Run("application registry", func(t *testing.T) {
		h := newSourceHarness(t, openHeight, func(ctx sdk.Context, a *app.App) {
			genesis := chainstypes.DefaultGenesis()
			for id, status := range []uint8{chainstypes.StatusActive, chainstypes.StatusPaused, chainstypes.StatusGovernancePaused, chainstypes.StatusRetired} {
				domainID := uint32(id + 1)
				genesis.Domains = append(genesis.Domains, chainstypes.Domain{
					Id: domainID, Name: fmt.Sprintf("chain-%d", domainID), Status: status,
					GasAllowance: 500000, ByteAllowance: 65536, TargetGasPerBlock: 250000, Bond: sdk.ZeroInt(),
				})
				genesis.NextBaseFees[domainID] = sdk.ZeroDec()
			}
			genesis.NextID = 5
			require.NoError(t, a.LayerXChainsKeeper.InitGenesis(ctx, *genesis))
		})
		h.open()
		ctx := h.openCtx()
		for id := uint32(1); id <= 4; id++ {
			got, gas, bytes, active, known := h.app.DomainAllowance(ctx, chainstypes.ChainIDOf(id))
			require.Equal(t, id, got)
			require.Equal(t, uint64(500000), gas)
			require.Equal(t, uint64(65536), bytes)
			require.Equal(t, id == 1, active)
			require.True(t, known)
		}
		id, gas, bytes, active, known := h.app.DomainAllowance(ctx, big.NewInt(125))
		require.Zero(t, id)
		require.Zero(t, gas)
		require.Zero(t, bytes)
		require.True(t, active)
		require.True(t, known)
		for _, chainID := range []*big.Int{nil, big.NewInt(0), big.NewInt(-1), big.NewInt(126), new(big.Int).SetUint64(chainstypes.VirtualChainIDBase), chainstypes.ChainIDOf(999), new(big.Int).Lsh(big.NewInt(1), 100)} {
			_, _, _, active, known := h.app.DomainAllowance(ctx, chainID)
			require.False(t, active)
			require.False(t, known)
		}
	})
}

func TestChainAllowanceDoesNotLimitRootTransactions(t *testing.T) {
	admissionChainID(t, 125)
	sender, root, recipient := utils.NewSigner(), utils.NewSigner(), utils.NewSigner()
	h := newAllowanceHarness(t, 4*transferGas, map[uint32]chainstypes.Domain{
		1: {Status: chainstypes.StatusActive, GasAllowance: transferGas, ByteAllowance: 200000},
	}, sender, root)
	h.open()
	_, first := signEVM(t, sender, chainstypes.ChainIDOf(1).Int64(), 0, recipient.EvmAddress, big.NewInt(1))
	_, err := h.exec.Accept(first)
	require.NoError(t, err)
	_, next := signEVM(t, sender, chainstypes.ChainIDOf(1).Int64(), 1, recipient.EvmAddress, big.NewInt(1))
	_, err = h.exec.Accept(next)
	require.ErrorIs(t, err, sequencer.ErrBlockFull)
	admissionChainID(t, 125)
	accepted := [][]byte{first}
	for nonce := uint64(0); nonce < 3; nonce++ {
		_, bz := signEVM(t, root, 125, nonce, recipient.EvmAddress, big.NewInt(1))
		acc, err := h.exec.Accept(bz)
		require.NoError(t, err)
		require.Equal(t, int64(transferGas), acc.Result.GasWanted)
		accepted = append(accepted, bz)
	}
	_, overflow := signEVM(t, root, 125, 3, recipient.EvmAddress, big.NewInt(1))
	acc, err := h.exec.Accept(overflow)
	require.Nil(t, acc)
	require.ErrorIs(t, err, sequencer.ErrBlockFull)
	require.ErrorContains(t, err, "Block.MaxGas")
	batch, err := h.exec.Seal(openHeight)
	require.NoError(t, err)
	require.Equal(t, accepted, batch.Txs)
}

func TestAcceptRefusesPastChainByteAllowance(t *testing.T) {
	for _, short := range []uint64{0, 1} {
		t.Run(fmt.Sprintf("%d bytes short", short), func(t *testing.T) {
			chainID := chainstypes.ChainIDOf(7).Int64()
			admissionChainID(t, 125)
			sender, other, recipient := utils.NewSigner(), utils.NewSigner(), utils.NewSigner()
			_, first := signEVM(t, sender, chainID, 0, recipient.EvmAddress, big.NewInt(1))
			_, second := signEVM(t, sender, chainID, 1, recipient.EvmAddress, big.NewInt(1))
			allowance := uint64(len(first)+len(second)) - short
			h := newAllowanceHarness(t, 20*transferGas, map[uint32]chainstypes.Domain{
				7: {Status: chainstypes.StatusActive, GasAllowance: 20 * transferGas, ByteAllowance: allowance},
				8: {Status: chainstypes.StatusActive, GasAllowance: 20 * transferGas, ByteAllowance: 200000},
			}, sender, other)
			h.open()
			_, gap := signEVM(t, sender, chainID, 2, recipient.EvmAddress, big.NewInt(1))
			_, err := h.exec.Accept(gap)
			requireRefused(t, err, sdkerrors.ErrWrongSequence)
			_, err = h.exec.Accept(first)
			require.NoError(t, err)
			accepted := [][]byte{first}
			next := second
			wanted := uint64(len(first) + len(second))
			if short == 0 {
				_, err = h.exec.Accept(second)
				require.NoError(t, err)
				accepted = append(accepted, second)
				_, next = signEVM(t, sender, chainID, 2, recipient.EvmAddress, big.NewInt(1))
				wanted += uint64(len(next))
			}
			before := h.weiBalance(h.openCtx(), sender.EvmAddress)
			acc, err := h.exec.Accept(next)
			require.Nil(t, acc)
			require.ErrorIs(t, err, sequencer.ErrBlockFull)
			require.ErrorContains(t, err, fmt.Sprintf("chain %d bytes wanted %d exceeds its allowance %d", 7, wanted, allowance))
			requireWei(t, before, h.weiBalance(h.openCtx(), sender.EvmAddress))
			require.Equal(t, uint64(len(accepted)), h.app.EvmKeeper.GetNonce(h.openCtx(), sender.EvmAddress))
			_, bz := signEVM(t, other, chainstypes.ChainIDOf(8).Int64(), 0, recipient.EvmAddress, big.NewInt(1))
			_, err = h.exec.Accept(bz)
			require.NoError(t, err)
			accepted = append(accepted, bz)
			admissionChainID(t, 125)
			_, root := signEVM(t, sender, 125, uint64(len(accepted)-1), recipient.EvmAddress, big.NewInt(1))
			_, err = h.exec.Accept(root)
			require.NoError(t, err)
			accepted = append(accepted, root)
			batch, err := h.exec.Seal(openHeight)
			require.NoError(t, err)
			require.Equal(t, accepted, batch.Txs)
		})
	}
}

func TestAcceptRefusesPastMaxGasWanted(t *testing.T) {
	admissionChainID(t, 125)
	for _, tc := range []struct {
		name              string
		maxGas, maxWanted int64
		limit             string
	}{
		{"max gas", 2*transferGas - 1, 10 * transferGas, "Block.MaxGas"},
		{"max wanted", 10 * transferGas, 2*transferGas - 1, "Block.MaxGasWanted"},
		{"zero gas", 0, 10 * transferGas, "Block.MaxGas"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sender, recipient := utils.NewSigner(), utils.NewSigner()
			h := newSourceHarness(t, openHeight, admissionParams(tc.maxGas, tc.maxWanted, 200000), sender)
			h.open()
			_, first := signEVM(t, sender, 125, 0, recipient.EvmAddress, big.NewInt(1))
			var accepted [][]byte
			if tc.maxGas != 0 {
				acc, err := h.exec.Accept(first)
				require.NoError(t, err)
				require.Equal(t, int64(transferGas), acc.Result.GasWanted)
				accepted = append(accepted, first)
			}
			ctx := h.openCtx()
			before := h.weiBalance(ctx, sender.EvmAddress)
			_, next := signEVM(t, sender, 125, uint64(len(accepted)), recipient.EvmAddress, big.NewInt(1))
			acc, err := h.exec.Accept(next)
			require.Nil(t, acc)
			require.ErrorIs(t, err, sequencer.ErrBlockFull)
			require.ErrorContains(t, err, tc.limit)
			requireWei(t, before, h.weiBalance(h.openCtx(), sender.EvmAddress))
			require.Equal(t, uint64(len(accepted)), h.app.EvmKeeper.GetNonce(h.openCtx(), sender.EvmAddress))
			batch, err := h.exec.Seal(openHeight)
			require.NoError(t, err)
			require.Equal(t, accepted, batch.Txs)
			require.Equal(t, sourceLastSeq+uint64(len(accepted)), batch.LastSeq)
		})
	}
	t.Run("Cosmos gas wanted and failed delivery", func(t *testing.T) {
		payer, sender, recipient := utils.NewSigner(), utils.NewSigner(), utils.NewSigner()
		h := newSourceHarness(t, openHeight, admissionParams(200000+transferGas, 1000000, 200000), payer, sender)
		h.open()
		_, gap := signEVM(t, sender, 125, 1, recipient.EvmAddress, big.NewInt(1))
		_, err := h.exec.Accept(gap)
		requireRefused(t, err, sdkerrors.ErrWrongSequence)
		bz := signCosmos(t, h.openCtx(), h.harness, payer, 200000,
			banktypes.NewMsgSend(payer.AccountAddress, recipient.AccountAddress, sdk.NewCoins(sdk.NewInt64Coin("uhpx", 1))))
		acc, err := h.exec.Accept(bz)
		require.NoError(t, err)
		require.Less(t, acc.Result.GasUsed, acc.Result.GasWanted)
		_, first := signEVM(t, sender, 125, 0, recipient.EvmAddress, big.NewInt(1))
		acc, err = h.exec.Accept(first)
		require.NoError(t, err)
		require.Equal(t, sourceLastSeq+2, acc.Seq)
		_, err = h.exec.Accept(gap)
		require.ErrorIs(t, err, sequencer.ErrBlockFull)
	})
}

func TestAcceptIgnoresUnsetMaxGasWanted(t *testing.T) {
	admissionChainID(t, 125)
	for _, maxWanted := range []int64{0, -1, -2} {
		t.Run(fmt.Sprintf("max wanted %d", maxWanted), func(t *testing.T) {
			sender, recipient := utils.NewSigner(), utils.NewSigner()
			h := newSourceHarness(t, openHeight, admissionParams(2*transferGas, maxWanted, 200000), sender)
			h.open()
			var accepted [][]byte
			for nonce := uint64(0); nonce < 2; nonce++ {
				_, bz := signEVM(t, sender, 125, nonce, recipient.EvmAddress, big.NewInt(1))
				acc, err := h.exec.Accept(bz)
				require.NoError(t, err)
				require.Equal(t, uint32(0), acc.Result.Code, acc.Result.Log)
				require.Equal(t, int64(transferGas), acc.Result.GasWanted)
				require.Equal(t, sourceLastSeq+nonce+1, acc.Seq)
				accepted = append(accepted, bz)
			}
			before := h.weiBalance(h.openCtx(), sender.EvmAddress)
			_, next := signEVM(t, sender, 125, 2, recipient.EvmAddress, big.NewInt(1))
			acc, err := h.exec.Accept(next)
			require.Nil(t, acc)
			require.ErrorIs(t, err, sequencer.ErrBlockFull)
			require.ErrorContains(t, err, fmt.Sprintf("gas wanted %d exceeds Block.MaxGas %d", 3*transferGas, 2*transferGas))
			requireWei(t, before, h.weiBalance(h.openCtx(), sender.EvmAddress))
			require.Equal(t, uint64(2), h.app.EvmKeeper.GetNonce(h.openCtx(), sender.EvmAddress))
			batch, err := h.exec.Seal(openHeight)
			require.NoError(t, err)
			require.Equal(t, accepted, batch.Txs)
			require.Equal(t, sourceLastSeq+2, batch.LastSeq)
		})
	}
}

func TestAcceptRefusesPastMaxBlockBytes(t *testing.T) {
	admissionChainID(t, 125)
	for _, short := range []int64{0, 1} {
		t.Run(big.NewInt(short).String()+" bytes short", func(t *testing.T) {
			sender, recipient := utils.NewSigner(), utils.NewSigner()
			_, first := signEVM(t, sender, 125, 0, recipient.EvmAddress, big.NewInt(1))
			_, second := signEVM(t, sender, 125, 1, recipient.EvmAddress, big.NewInt(1))
			reserve := int64(len(admissionTx0(t)))
			maxBytes := reserve + int64(len(first)+len(second)) - short
			h := newSourceHarness(t, openHeight, admissionParams(1000000, 1000000, maxBytes), sender)
			h.open()
			_, err := h.exec.Accept(first)
			require.NoError(t, err)
			want := [][]byte{first}
			if short == 0 {
				acc, err := h.exec.Accept(second)
				require.NoError(t, err)
				require.Equal(t, sourceLastSeq+2, acc.Seq)
				want = append(want, second)
			}
			before := h.weiBalance(h.openCtx(), sender.EvmAddress)
			_, next := signEVM(t, sender, 125, uint64(len(want)), recipient.EvmAddress, big.NewInt(1))
			acc, err := h.exec.Accept(next)
			require.Nil(t, acc)
			require.ErrorIs(t, err, sequencer.ErrBlockFull)
			require.ErrorContains(t, err, "Block.MaxBytes")
			requireWei(t, before, h.weiBalance(h.openCtx(), sender.EvmAddress))
			require.Equal(t, uint64(len(want)), h.app.EvmKeeper.GetNonce(h.openCtx(), sender.EvmAddress))
			batch, err := h.exec.Seal(openHeight)
			require.NoError(t, err)
			require.Equal(t, want, batch.Txs)
			require.Equal(t, sourceLastSeq+uint64(len(want)), batch.LastSeq)
			if short == 0 {
				require.Equal(t, maxBytes, reserve+int64(len(first)+len(second)))
			}
		})
	}
}

func TestSealedBlockPassesCheckTotalBlockGas(t *testing.T) {
	admissionChainID(t, 125)
	sender, recipient := utils.NewSigner(), utils.NewSigner()
	_, first := signEVM(t, sender, 125, 0, recipient.EvmAddress, big.NewInt(1))
	_, second := signEVM(t, sender, 125, 1, recipient.EvmAddress, big.NewInt(1))
	maxBytes := int64(len(admissionTx0(t)) + len(first) + len(second))
	h := newSourceHarness(t, openHeight, func(ctx sdk.Context, a *app.App) {
		admissionParams(2*transferGas, 2*transferGas, maxBytes)(ctx, a)
		require.NoError(t, a.SequencerKeeper.SetActivationHeight(ctx, openHeight))
	}, sender)
	ms := h.open()
	for _, bz := range [][]byte{first, second} {
		acc, err := h.exec.Accept(bz)
		require.NoError(t, err)
		require.Equal(t, uint32(0), acc.Result.Code, acc.Result.Log)
	}
	_, third := signEVM(t, sender, 125, 2, recipient.EvmAddress, big.NewInt(1))
	_, err := h.exec.Accept(third)
	require.ErrorIs(t, err, sequencer.ErrBlockFull)
	batch, err := h.exec.Seal(openHeight)
	require.NoError(t, err)
	require.Equal(t, int64(2*transferGas), batch.Results[0].GasWanted+batch.Results[1].GasWanted)
	msg := signSealBatch(t, sequencer.BatchFields(batch), testKeySeed, testKeyID)
	sealed, tx0, err := sequencer.SealBatch(h.app, batch, msg)
	require.NoError(t, err)
	require.Equal(t, maxBytes, int64(len(tx0)+len(first)+len(second)))
	require.Equal(t, abci.ResponseProcessProposal_ACCEPT,
		resolutionProposalFrom(t, h.app, openHeight, time.UnixMilli(ms), sealed.Txs, h.proposer))
}

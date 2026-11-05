package evm_test

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	tmed25519 "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types/ethtx"
	chainstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	appante "github.com/Sidiora-Labs/Paxeer-X-Network/node/ante"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/antedecorators"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

const repairM1244BloomCutoff int64 = 2000

type repairM1244RecoveryState struct {
	currentFee  sdk.Dec
	nextFee     sdk.Dec
	blockBloom  ethtypes.Bloom
	evmBloom    ethtypes.Bloom
	legacyBloom ethtypes.Bloom
}

func repairM1244Bloom(id byte) ethtypes.Bloom {
	return ethtypes.CreateBloom(&ethtypes.Receipt{Logs: []*ethtypes.Log{{
		Address: common.BytesToAddress([]byte{id}),
		Topics:  []common.Hash{common.BytesToHash([]byte{id, id})},
	}}})
}

func repairM1244AssertRecovered(t *testing.T, k *keeper.Keeper, ctx sdk.Context, want repairM1244RecoveryState) {
	t.Helper()
	require.Equal(t, want.currentFee, k.GetCurrBaseFeePerGas(ctx))
	require.Equal(t, want.nextFee, k.GetNextBaseFeePerGas(ctx))
	require.True(t, k.GetNextBaseFeePerGas(ctx).GT(k.GetMinimumFeePerGas(ctx)))
	require.Equal(t, want.nextFee.TruncateInt().BigInt(), appante.GetBaseFee(ctx, k, k.UpgradeKeeper()))
	require.Equal(t, repairM1244BloomCutoff, k.GetLegacyBlockBloomCutoffHeight(ctx))
	require.Equal(t, want.blockBloom, k.GetBlockBloom(ctx))
	require.Equal(t, want.evmBloom, k.GetEvmOnlyBlockBloom(ctx))
	for _, height := range []int64{1, int64(evm.GENESIS_EXPORT_STREAM_SERIALIZED_LEN_MAX + 2), repairM1244BloomCutoff - 1, repairM1244BloomCutoff} {
		require.Equal(t, want.legacyBloom, k.GetLegacyBlockBloom(ctx, height))
	}

	// Read the historical format without the current-height singleton blooms.
	// The row at the cutoff deliberately remains present: the recovered cutoff
	// must prevent either consumer from treating it as a legacy block bloom.
	legacyCtx, _ := ctx.CacheContext()
	legacyCtx.KVStore(k.GetStoreKey()).Delete(types.BlockBloomPrefix)
	legacyCtx.KVStore(k.GetStoreKey()).Delete(types.EvmOnlyBlockBloomPrefix)
	for _, height := range []int64{repairM1244BloomCutoff - 1, repairM1244BloomCutoff, repairM1244BloomCutoff + 1} {
		wantBloom := ethtypes.Bloom{}
		if height < repairM1244BloomCutoff {
			wantBloom = want.legacyBloom
		}
		require.Equal(t, wantBloom, k.GetBlockBloom(legacyCtx.WithBlockHeight(height)))
		require.Equal(t, wantBloom, k.GetEvmOnlyBlockBloom(legacyCtx.WithBlockHeight(height)))
	}
	// Historical reads above must not alter the recovered current-format state.
	require.Equal(t, want.blockBloom, k.GetBlockBloom(ctx))
	require.Equal(t, want.evmBloom, k.GetEvmOnlyBlockBloom(ctx))
}

func repairM1244Open(t *testing.T, dir, home string) (*app.App, func()) {
	t.Helper()
	db, err := dbm.NewGoLevelDB("fee-bloom-recovery", dir)
	require.NoError(t, err)
	a := app.New(db, nil, true, map[int64]bool{}, home, 1, false,
		config.TestConfig(), app.MakeEncodingConfig(), wasm.EnableAllProposals,
		app.TestAppOpts{}, app.EmptyWasmOpts, nil)
	closed := false
	closeApp := func() {
		t.Helper()
		if !closed {
			closed = true
			require.NoError(t, a.Close())
		}
	}
	t.Cleanup(closeApp)
	return a, closeApp
}

func repairM1244RestartRecovery(t *testing.T, streamed bool) {
	t.Helper()
	source := app.Setup(t, false, false, false)
	ctx := source.GetContextForDeliverTx(nil).WithBlockHeight(repairM1244BloomCutoff)
	k := &source.EvmKeeper
	want := repairM1244RecoveryState{
		currentFee: k.GetMinimumFeePerGas(ctx).MulInt64(3).Add(sdk.MustNewDecFromStr("0.125")),
		nextFee:    k.GetMinimumFeePerGas(ctx).MulInt64(5).Add(sdk.MustNewDecFromStr("0.875")),
		blockBloom: repairM1244Bloom(1), evmBloom: repairM1244Bloom(2), legacyBloom: repairM1244Bloom(3),
	}
	k.SetCurrBaseFeePerGas(ctx, want.currentFee)
	k.SetNextBaseFeePerGas(ctx, want.nextFee)
	k.SetBlockBloom(ctx, []ethtypes.Bloom{want.blockBloom})
	k.SetEvmOnlyBlockBloom(ctx, []ethtypes.Bloom{want.evmBloom})
	k.SetLegacyBlockBloomCutoffHeight(ctx)
	// Committed historical-format fixture: the old writer stored the real bloom
	// bytes under BlockBloomKey(height). There is no longer a legacy setter.
	for height := int64(1); height <= int64(evm.GENESIS_EXPORT_STREAM_SERIALIZED_LEN_MAX+2); height++ {
		ctx.KVStore(k.GetStoreKey()).Set(types.BlockBloomKey(height), want.legacyBloom.Bytes())
	}
	for _, height := range []int64{repairM1244BloomCutoff - 1, repairM1244BloomCutoff} {
		ctx.KVStore(k.GetStoreKey()).Set(types.BlockBloomKey(height), want.legacyBloom.Bytes())
	}
	repairM1244AssertRecovered(t, k, ctx, want)
	cdc := source.AppCodec()
	module := evm.NewAppModule(cdc, k)
	var raw []json.RawMessage
	if streamed {
		for chunk := range module.ExportGenesisStream(ctx, cdc) {
			raw = append(raw, chunk)
		}
	} else {
		raw = []json.RawMessage{module.ExportGenesis(ctx, cdc)}
	}
	var entries []*types.Serialized
	var bloomChunkSizes []int
	for _, chunk := range raw {
		var decoded types.GenesisState
		cdc.MustUnmarshalJSON(chunk, &decoded)
		require.NoError(t, decoded.Validate())
		entries = append(entries, decoded.Serialized...)
		count := 0
		for _, entry := range decoded.Serialized {
			if bytes.Equal(entry.Prefix, types.BlockBloomPrefix) {
				count++
			}
		}
		if count > 0 {
			bloomChunkSizes = append(bloomChunkSizes, count)
		}
	}
	for _, prefix := range [][]byte{types.BaseFeePerGasPrefix, types.NextBaseFeePerGasPrefix, types.LegacyBlockBloomCutoffHeightKey, types.BlockBloomPrefix, types.EvmOnlyBlockBloomPrefix} {
		found := 0
		for _, entry := range entries {
			if bytes.Equal(entry.Prefix, prefix) && len(entry.Key) == 0 {
				found++
				require.Equal(t, ctx.KVStore(k.GetStoreKey()).Get(prefix), entry.Value)
			}
		}
		require.Equal(t, 1, found, "singleton prefix %x must survive the codec exactly once", prefix)
	}
	if streamed {
		require.Equal(t, []int{evm.GENESIS_EXPORT_STREAM_SERIALIZED_LEN_MAX + 1, 4}, bloomChunkSizes)
	}
	require.NoError(t, source.Close())

	dir, home := t.TempDir(), t.TempDir()
	target, closeTarget := repairM1244Open(t, dir, home)
	genesis, err := json.Marshal(app.NewDefaultGenesisState(target.AppCodec()))
	require.NoError(t, err)
	_, err = target.InitChain(context.Background(), &abci.RequestInitChain{
		Time: time.Unix(1, 0), ConsensusParams: app.DefaultConsensusParams,
		ChainId: "pax-test", AppStateBytes: genesis,
	})
	require.NoError(t, err)
	ctx = target.GetContextForDeliverTx(nil).WithBlockHeight(repairM1244BloomCutoff)
	k = &target.EvmKeeper
	for _, prefix := range [][]byte{types.BaseFeePerGasPrefix, types.NextBaseFeePerGasPrefix, types.LegacyBlockBloomCutoffHeightKey, types.BlockBloomPrefix, types.EvmOnlyBlockBloomPrefix} {
		require.False(t, ctx.KVStore(k.GetStoreKey()).Has(prefix))
	}
	module = evm.NewAppModule(cdc, k)
	if streamed {
		chunks := make(chan json.RawMessage, len(raw))
		for _, chunk := range raw {
			chunks <- chunk
		}
		close(chunks)
		module.InitGenesisStream(ctx, cdc, chunks)
	} else {
		module.InitGenesis(ctx, cdc, raw[0])
	}
	repairM1244AssertRecovered(t, k, ctx, want)
	// The node requires the decided block identity when reopening committed
	// state. Persist it through the same pre-commit hook used by BaseApp.Commit.
	validators := tmtypes.NewValidatorSet([]*tmtypes.Validator{tmtypes.NewValidator(tmed25519.GenerateSecretKey().Public(), 1)})
	block := tmtypes.MakeBlock(1, nil, &tmtypes.Commit{}, nil)
	block.ChainID = "pax-test"
	block.Time = time.Unix(1, 0)
	block.ValidatorsHash = validators.Hash()
	block.NextValidatorsHash = validators.Hash()
	blockHash := block.Hash()
	require.NotEmpty(t, blockHash)
	require.NoError(t, target.HandlePreCommit(ctx.WithHeaderHash(blockHash)))
	target.SetDeliverStateToCommit()
	commit := target.WriteState().Commit(true)
	require.Positive(t, commit.Version)
	closeTarget()

	reopened, _ := repairM1244Open(t, dir, home)
	require.Equal(t, commit, reopened.LastCommitID())
	require.Equal(t, []byte(blockHash), reopened.LastBlockID().Hash)
	ctx = reopened.NewUncachedContext(false, tmproto.Header{ChainID: "pax-test", Height: repairM1244BloomCutoff})
	repairM1244AssertRecovered(t, &reopened.EvmKeeper, ctx, want)
}

func TestRepairM_1244_OrdinaryRestartRecovery(t *testing.T) {
	repairM1244RestartRecovery(t, false)
}

func TestRepairM_1244_StreamRestartRecovery(t *testing.T) {
	repairM1244RestartRecovery(t, true)
}

func repairM1244Fund(t *testing.T, a *app.App, ctx sdk.Context, address common.Address, amount sdk.Int) {
	t.Helper()
	coins := sdk.NewCoins(sdk.NewCoin(a.EvmKeeper.GetBaseDenom(ctx), amount))
	require.NoError(t, a.BankKeeper.MintCoins(ctx, types.ModuleName, coins))
	require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, sdk.AccAddress(address.Bytes()), coins))
}

func TestRepairM_1244_AntePipelineGuide(t *testing.T) {
	guide, err := os.ReadFile("AGENTS.md")
	require.NoError(t, err)
	pipeline := strings.Split(strings.Split(string(guide), "## Ante Handler Pipeline\n")[1], "\n---")[0]
	previous := -1
	for _, stage := range []string{"EVMNoCosmosFieldsDecorator", "EVMChainIDDecorator", "EVMPreprocessDecorator", "BasicDecorator", "EVMFeeCheckDecorator", "EVMDomainSigVerifyDecorator", "GasDecorator"} {
		position := strings.Index(pipeline, "**`"+stage+"`**")
		require.Greater(t, position, previous, "documented mounted stage %s", stage)
		previous = position
	}
	for _, detail := range []string{"StatelessChecks", "before `BuyGas`", "clearing any inherited tenant context", "read-only", "**Cosmos** ante chain"} {
		require.Contains(t, pipeline, detail)
	}

	a := app.Setup(t, false, false, false)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	base := a.GetContextForDeliverTx(nil).WithChainID("hyperpax_125-1").WithBlockHeight(100).WithBlockTime(time.Unix(1_800_000_000, 0))
	base, _ = base.CacheContext()
	k := &a.EvmKeeper
	rootFee := k.GetMinimumFeePerGas(base).MulInt64(2)
	domainFee := rootFee.MulInt64(2)
	k.SetNextBaseFeePerGas(base, rootFee)
	operator := common.HexToAddress("0x1244")
	params := a.LayerXChainsKeeper.GetParams(base)
	bond := params.MinBond.Add(params.CreationFee)
	repairM1244Fund(t, a, base, operator, bond.MulRaw(2))
	active, err := a.LayerXChainsKeeper.Create(base, operator, "repair-active", params.MinGasAllowance, params.MinByteAllowance, 0, bond, domainFee)
	require.NoError(t, err)
	paused, err := a.LayerXChainsKeeper.Create(base, operator, "repair-paused", params.MinGasAllowance, params.MinByteAllowance, 0, bond, domainFee)
	require.NoError(t, err)
	require.NoError(t, a.LayerXChainsKeeper.Pause(base, operator, paused.Id))
	private, err := crypto.ToECDSA(bytes.Repeat([]byte{0x44}, 32))
	require.NoError(t, err)
	sender := crypto.PubkeyToAddress(private.PublicKey)
	to := common.HexToAddress("0x5678")
	rootID := k.ChainID(base)
	for _, optimized := range []bool{false, true} {
		mode := "mounted"
		if optimized {
			mode = "optimized-checktx"
		}
		for _, tenant := range []bool{false, true} {
			chainID, fee, domainID := rootID, rootFee, uint32(0)
			if tenant {
				chainID, fee, domainID = chainstypes.ChainIDOf(active.Id), domainFee, active.Id
			}
			for _, tc := range []struct {
				name   string
				check  bool
				nonce  uint64
				funded bool
				lowFee bool
				want   error
			}{
				{"current-check", true, 4, true, false, nil},
				{"pending-check", true, 5, true, false, nil},
				{"stale-check", true, 3, true, false, sdkerrors.ErrWrongSequence},
				{"fee-before-nonce", true, 3, true, true, sdkerrors.ErrInsufficientFee},
				{"buygas-before-pending-nonce", true, 3, false, false, sdkerrors.ErrInsufficientFunds},
				{"current-delivery", false, 4, true, false, nil},
				{"future-delivery", false, 5, true, false, sdkerrors.ErrWrongSequence},
				{"delivery-stateless-before-buygas", false, 3, false, false, sdkerrors.ErrWrongSequence},
			} {
				if optimized && !tc.check {
					continue // The optimized entry point is only mounted for CheckTx.
				}
				t.Run(mode+"/"+chainID.String()+"/"+tc.name, func(t *testing.T) {
					ctx, _ := base.WithIsCheckTx(tc.check).CacheContext()
					// Every entry must select its signed domain rather than inherit this one.
					ctx = chainstypes.WithDomain(ctx, paused.Id)
					selected := chainstypes.WithDomain(ctx, domainID)
					k.SetNonce(selected, sender, 4)
					funds := sdk.NewInt(1_000_000_000)
					if tc.funded {
						repairM1244Fund(t, a, selected, sender, funds)
					}
					cap := fee.TruncateInt().BigInt()
					if tc.lowFee {
						cap.Sub(cap, big.NewInt(1))
					}
					signed, err := ethtypes.SignTx(ethtypes.NewTx(&ethtypes.DynamicFeeTx{
						ChainID: chainID, Nonce: tc.nonce, Gas: 21000, GasFeeCap: cap, To: &to,
					}), ethtypes.LatestSignerForChainID(chainID), private)
					require.NoError(t, err)
					data, err := ethtx.NewTxDataFromTx(signed)
					require.NoError(t, err)
					msg, err := types.NewMsgEVMTransaction(data)
					require.NoError(t, err)
					builder := a.GetTxConfig().NewTxBuilder()
					require.NoError(t, builder.SetMsgs(msg))
					// Use the node codec as the transport, including its interface unpacking.
					encoded, err := a.GetTxConfig().TxEncoder()(builder.GetTx())
					require.NoError(t, err)
					tx, err := a.GetTxConfig().TxDecoder()(encoded)
					require.NoError(t, err)
					var out sdk.Context
					if optimized {
						out, err = appante.EvmCheckTxAnte(ctx, tx, &a.UpgradeKeeper, k, a.LayerXChainsKeeper)
					} else {
						out, err = a.AnteHandler(ctx, tx, false)
					}
					if tc.want != nil {
						require.ErrorIs(t, err, tc.want)
						return
					}
					require.NoError(t, err)
					id, isTenant := chainstypes.DomainFrom(out)
					require.Equal(t, domainID, id)
					require.Equal(t, tenant, isTenant)
					require.True(t, out.IsEVM())
					require.Equal(t, sender, out.EVMSenderAddress())
					require.Equal(t, tc.nonce, out.EVMNonce())
					require.Equal(t, signed.Hash(), out.EVMTxHash())
					wantBalance := new(big.Int).Mul(funds.BigInt(), state.UhpxToSweiMultiplier)
					if !tc.check {
						wantBalance.Sub(wantBalance, new(big.Int).Mul(fee.TruncateInt().BigInt(), big.NewInt(21000)))
					}
					require.Equal(t, wantBalance, k.GetBalance(out, out.PaxSenderAddress()))
					require.Equal(t, uint64(4), k.GetNonce(out, sender))
				})
			}
		}
		for _, tc := range []struct {
			name    string
			chainID *big.Int
			want    error
		}{
			{"unknown", chainstypes.ChainIDOf(paused.Id + 1), sdkerrors.ErrInvalidChainID},
			{"paused", chainstypes.ChainIDOf(paused.Id), chainstypes.ErrDomainNotActive},
		} {
			t.Run(mode+"/admission/"+tc.name, func(t *testing.T) {
				ctx, _ := base.WithIsCheckTx(true).CacheContext()
				// An unsigned, underpriced envelope also proves domain refusal occurs
				// before signature recovery, association or fee validation.
				data, err := ethtx.NewTxDataFromTx(ethtypes.NewTx(&ethtypes.DynamicFeeTx{ChainID: tc.chainID, Gas: 21000, GasFeeCap: big.NewInt(1), To: &to}))
				require.NoError(t, err)
				msg, err := types.NewMsgEVMTransaction(data)
				require.NoError(t, err)
				builder := a.GetTxConfig().NewTxBuilder()
				require.NoError(t, builder.SetMsgs(msg))
				if optimized {
					_, err = appante.EvmCheckTxAnte(ctx, builder.GetTx(), &a.UpgradeKeeper, k, a.LayerXChainsKeeper)
				} else {
					_, err = a.AnteHandler(ctx, builder.GetTx(), false)
				}
				require.ErrorIs(t, err, tc.want)
				require.Nil(t, msg.Derived)
				_, associated := k.GetPaxAddress(chainstypes.WithDomain(ctx, active.Id), sender)
				require.False(t, associated)
			})
		}
	}
}

func TestRepairM_1244_AssociatePaths(t *testing.T) {
	a := app.Setup(t, false, false, false)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	base := a.GetContextForDeliverTx(nil).WithChainID("hyperpax_125-1").WithBlockHeight(100)
	private, err := crypto.ToECDSA(bytes.Repeat([]byte{0x45}, 32))
	require.NoError(t, err)
	sender := crypto.PubkeyToAddress(private.PublicKey)
	paxPrivate := secp256k1.PrivKey{Key: crypto.FromECDSA(private)}
	paxAddress := sdk.AccAddress(paxPrivate.PubKey().Address())
	const message = "repair-m1244-association"
	signature, err := crypto.Sign(crypto.Keccak256([]byte(message)), private)
	require.NoError(t, err)
	for _, optimized := range []bool{false, true} {
		mode := "mounted"
		if optimized {
			mode = "optimized-checktx"
		}
		for _, scenario := range []string{"no-balance", "wrapper-memo", "associate", "already-associated"} {
			t.Run(mode+"/"+scenario, func(t *testing.T) {
				ctx, _ := base.WithIsCheckTx(optimized).CacheContext()
				k := &a.EvmKeeper
				if scenario != "no-balance" {
					repairM1244Fund(t, a, ctx, sender, sdk.NewInt(1))
				}
				if scenario == "already-associated" {
					k.SetAddressMapping(ctx, paxAddress, sender)
				}
				msg, err := types.NewMsgEVMTransaction(&ethtx.AssociateTx{
					V: big.NewInt(int64(signature[64])).Bytes(), R: signature[:32], S: signature[32:64], CustomMessage: message,
				})
				require.NoError(t, err)
				builder := a.GetTxConfig().NewTxBuilder()
				require.NoError(t, builder.SetMsgs(msg))
				if scenario == "wrapper-memo" {
					builder.SetMemo("must be rejected before association")
				}
				var out sdk.Context
				if optimized {
					out, err = appante.EvmCheckTxAnte(ctx, builder.GetTx(), &a.UpgradeKeeper, k, a.LayerXChainsKeeper)
				} else {
					out, err = a.AnteHandler(ctx, builder.GetTx(), false)
				}
				switch scenario {
				case "no-balance":
					require.ErrorIs(t, err, sdkerrors.ErrInsufficientFunds)
				case "wrapper-memo":
					require.Error(t, err)
					require.Nil(t, msg.Derived)
				case "already-associated":
					require.ErrorIs(t, err, sdkerrors.ErrInvalidRequest)
				case "associate":
					require.NoError(t, err)
					require.Equal(t, int64(antedecorators.EVMAssociatePriority), out.Priority())
					_, isTenant := chainstypes.DomainFrom(out)
					require.False(t, isTenant)
					require.Equal(t, state.UhpxToSweiMultiplier, k.GetBalance(out, k.GetPaxAddressOrDefault(out, sender)))
					require.Zero(t, k.GetNonce(out, sender))
				}
				associatedAddress, associated := k.GetPaxAddress(ctx, sender)
				wantAssociation := scenario == "already-associated" || (scenario == "associate" && !optimized)
				require.Equal(t, wantAssociation, associated)
				if wantAssociation {
					require.Equal(t, paxAddress, associatedAddress)
				}
			})
		}
	}
}

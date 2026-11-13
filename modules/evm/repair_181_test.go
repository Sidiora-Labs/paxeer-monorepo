package evm_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	tmed25519 "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

type repair181Binding struct {
	address common.Address
	key     [32]byte
	private ed25519.PrivateKey
}

func repair181Bind(t *testing.T, k *keeper.Keeper, ctx sdk.Context, id byte) repair181Binding {
	t.Helper()
	binding := repair181Binding{address: common.BytesToAddress([]byte{id})}
	binding.private = ed25519.NewKeyFromSeed(bytes.Repeat([]byte{id}, ed25519.SeedSize))
	copy(binding.key[:], binding.private.Public().(ed25519.PublicKey))
	message, err := types.LayerXBindMessage(k.ChainID(ctx), binding.address, 0)
	require.NoError(t, err)
	var signature [64]byte
	copy(signature[:], ed25519.Sign(binding.private, message))
	nonce, err := k.BindLayerX(ctx, binding.address, binding.key, signature)
	require.NoError(t, err)
	require.Zero(t, nonce)
	return binding
}

func repair181Export(t *testing.T) (codec.Codec, json.RawMessage, []json.RawMessage, []repair181Binding) {
	t.Helper()
	a := app.Setup(t, false, false, false)
	ctx := a.GetContextForDeliverTx(nil)
	k := &a.EvmKeeper
	bindings := []repair181Binding{
		repair181Bind(t, k, ctx, 1),
		repair181Bind(t, k, ctx, 2),
		repair181Bind(t, k, ctx, 3),
	}
	_, nonce, err := k.UnbindLayerX(ctx, bindings[2].address)
	require.NoError(t, err)
	require.Equal(t, uint64(1), nonce)
	k.SetCode(ctx, bindings[0].address, []byte{0x60, 0x00, 0x00})
	k.SetState(ctx, bindings[0].address, common.HexToHash("0x01"), common.HexToHash("0x1234"))
	k.SetNonce(ctx, bindings[0].address, 9)
	require.NoError(t, k.ValidateLayerXBindings(ctx))
	cdc := a.AppCodec()
	module := evm.NewAppModule(cdc, k)
	ordinary := module.ExportGenesis(ctx, cdc)
	var chunks []json.RawMessage
	for chunk := range module.ExportGenesisStream(ctx, cdc) {
		chunks = append(chunks, chunk)
	}
	require.Greater(t, len(chunks), 1)
	require.NoError(t, a.Close())
	return cdc, ordinary, chunks, bindings
}

func repair181Channel(chunks []json.RawMessage) <-chan json.RawMessage {
	ch := make(chan json.RawMessage, len(chunks))
	for _, chunk := range chunks {
		ch <- chunk
	}
	close(ch)
	return ch
}

func repair181AssertRecovered(t *testing.T, k *keeper.Keeper, ctx sdk.Context, bindings []repair181Binding) {
	t.Helper()
	require.NoError(t, k.ValidateLayerXBindings(ctx))
	for _, binding := range bindings[:2] {
		key, bound := k.GetLayerXDid(ctx, binding.address)
		require.True(t, bound)
		require.Equal(t, binding.key, key)
		address, bound := k.GetEVMAddressByLayerXDid(ctx, binding.key)
		require.True(t, bound)
		require.Equal(t, binding.address, address)
		require.Equal(t, uint64(1), k.GetLayerXBindNonce(ctx, binding.address))
	}
	released := bindings[2]
	_, bound := k.GetLayerXDid(ctx, released.address)
	require.False(t, bound)
	_, bound = k.GetEVMAddressByLayerXDid(ctx, released.key)
	require.False(t, bound)
	require.Equal(t, uint64(2), k.GetLayerXBindNonce(ctx, released.address))
	require.Equal(t, []byte{0x60, 0x00, 0x00}, k.GetCode(ctx, bindings[0].address))
	require.Equal(t, common.HexToHash("0x1234"), k.GetState(ctx, bindings[0].address, common.HexToHash("0x01")))
	require.Equal(t, uint64(9), k.GetNonce(ctx, bindings[0].address))
	message, err := types.LayerXBindMessage(k.ChainID(ctx), released.address, 0)
	require.NoError(t, err)
	var signature [64]byte
	copy(signature[:], ed25519.Sign(released.private, message))
	_, err = k.BindLayerX(ctx, released.address, released.key, signature)
	require.ErrorIs(t, err, types.ErrLayerXSignature)
	require.Equal(t, uint64(2), k.GetLayerXBindNonce(ctx, released.address))
}

func repair181Open(db dbm.DB, home string) *app.App {
	return app.New(db, nil, true, map[int64]bool{}, home, 1, false,
		config.TestConfig(), app.MakeEncodingConfig(), wasm.EnableAllProposals,
		app.TestAppOpts{}, app.EmptyWasmOpts, nil)
}

func repair181RestartRecovery(t *testing.T, streamed bool) {
	t.Helper()
	cdc, ordinary, chunks, bindings := repair181Export(t)
	dir := t.TempDir()
	db, err := dbm.NewGoLevelDB("recovery", dir)
	require.NoError(t, err)
	home := t.TempDir()
	a := repair181Open(db, home)
	genesis, err := json.Marshal(app.NewDefaultGenesisState(a.AppCodec()))
	require.NoError(t, err)
	_, err = a.InitChain(context.Background(), &abci.RequestInitChain{
		Time: time.Unix(1, 0), ConsensusParams: app.DefaultConsensusParams,
		ChainId: "pax-test", AppStateBytes: genesis,
	})
	require.NoError(t, err)
	ctx := a.GetContextForDeliverTx(nil)
	module := evm.NewAppModule(cdc, &a.EvmKeeper)
	_, bound := a.EvmKeeper.GetLayerXDid(ctx, bindings[0].address)
	require.False(t, bound)
	if streamed {
		require.NotPanics(t, func() { module.InitGenesisStream(ctx, cdc, repair181Channel(chunks)) })
	} else {
		require.NotPanics(t, func() { module.InitGenesis(ctx, cdc, ordinary) })
	}
	repair181AssertRecovered(t, &a.EvmKeeper, ctx, bindings)
	// Persist the decided identity through the production pre-commit hook,
	// just as BaseApp.Commit does before committing the multistore.
	validators := tmtypes.NewValidatorSet([]*tmtypes.Validator{tmtypes.NewValidator(tmed25519.GenerateSecretKey().Public(), 1)})
	block := tmtypes.MakeBlock(1, nil, &tmtypes.Commit{}, nil)
	block.ChainID = "pax-test"
	block.Time = time.Unix(1, 0)
	block.ValidatorsHash = validators.Hash()
	block.NextValidatorsHash = validators.Hash()
	blockHash := block.Hash()
	require.NotEmpty(t, blockHash)
	require.NoError(t, a.HandlePreCommit(ctx.WithHeaderHash(blockHash)))
	a.SetDeliverStateToCommit()
	commit := a.WriteState().Commit(true)
	require.Positive(t, commit.Version)
	require.NoError(t, a.Close())

	db, err = dbm.NewGoLevelDB("recovery", dir)
	require.NoError(t, err)
	reopened := repair181Open(db, home)
	t.Cleanup(func() { require.NoError(t, reopened.Close()) })
	require.Equal(t, commit, reopened.LastCommitID())
	require.Equal(t, []byte(blockHash), reopened.LastBlockID().Hash)
	ctx = reopened.NewUncachedContext(false, tmproto.Header{ChainID: "pax-test", Height: commit.Version})
	repair181AssertRecovered(t, &reopened.EvmKeeper, ctx, bindings)
}

func TestRepair_181_OrdinaryRestartRecovery(t *testing.T) {
	repair181RestartRecovery(t, false)
}

func TestRepair_181_StreamRestartRecovery(t *testing.T) {
	repair181RestartRecovery(t, true)
}

func TestRepair_181_RejectInconsistentRecovery(t *testing.T) {
	cdc, ordinary, chunks, bindings := repair181Export(t)
	a := app.Setup(t, false, false, false)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	origin := a.GetContextForDeliverTx(nil)
	module := evm.NewAppModule(cdc, &a.EvmKeeper)
	cases := []struct {
		name   string
		prefix []byte
		key    []byte
		value  []byte
	}{
		{"missing_reverse", types.LayerXDidToEVMAddressKeyPrefix, bindings[0].key[:], nil},
		{"missing_forward", types.EVMAddressToLayerXDidKeyPrefix, bindings[0].address[:], nil},
		{"conflicting_reverse", types.LayerXDidToEVMAddressKeyPrefix, bindings[0].key[:], common.BytesToAddress([]byte{99}).Bytes()},
		{"conflicting_forward", types.EVMAddressToLayerXDidKeyPrefix, bindings[0].address[:], bindings[2].key[:]},
		{"duplicate_did", types.EVMAddressToLayerXDidKeyPrefix, bindings[0].address[:], bindings[1].key[:]},
		{"duplicate_address", types.LayerXDidToEVMAddressKeyPrefix, bindings[0].key[:], bindings[1].address[:]},
	}
	for _, tc := range cases {
		for _, streamed := range []bool{false, true} {
			mode := "ordinary"
			input := []json.RawMessage{ordinary}
			if streamed {
				mode, input = "stream", chunks
			}
			t.Run(tc.name+"/"+mode, func(t *testing.T) {
				ctx := origin.WithMultiStore(origin.MultiStore().CacheMultiStore())
				mutated := make([]json.RawMessage, 0, len(input))
				replaced := 0
				for _, raw := range input {
					var state types.GenesisState
					cdc.MustUnmarshalJSON(raw, &state)
					var rows []*types.Serialized
					for _, row := range state.Serialized {
						if bytes.Equal(row.Prefix, tc.prefix) && bytes.Equal(row.Key, tc.key) {
							replaced++
							if tc.value == nil {
								continue
							}
							row.Value = tc.value
						}
						require.NoError(t, types.ValidateLayerXGenesisEntry(row.Prefix, row.Key, row.Value))
						rows = append(rows, row)
					}
					state.Serialized = rows
					mutated = append(mutated, cdc.MustMarshalJSON(&state))
				}
				require.Equal(t, 1, replaced)
				require.PanicsWithError(t, types.ErrLayerXGenesisBinding.Error(), func() {
					if streamed {
						module.InitGenesisStream(ctx, cdc, repair181Channel(mutated))
					} else {
						module.InitGenesis(ctx, cdc, mutated[0])
					}
				})
			})
		}
	}
}

package tx

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	tmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/node"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/local"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	sequencertypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/query"
	txtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
)

func repair626Transactions(t *testing.T) (client.TxConfig, codectypes.InterfaceRegistry, tmtypes.Txs) {
	t.Helper()
	registry := codectypes.NewInterfaceRegistry()
	banktypes.RegisterInterfaces(registry)
	cfg := NewTxConfig(codec.NewProtoCodec(registry), DefaultSignModes)
	from := sdk.AccAddress(bytes.Repeat([]byte{1}, 20))
	to := sdk.AccAddress(bytes.Repeat([]byte{2}, 20))
	var txs tmtypes.Txs
	for i, memo := range []string{"first transaction", "second transaction"} {
		builder := cfg.NewTxBuilder()
		require.NoError(t, builder.SetMsgs(banktypes.NewMsgSend(from, to, sdk.NewCoins(sdk.NewInt64Coin("atom", int64(i+1))))))
		builder.SetMemo(memo)
		builder.SetGasLimit(200000)
		wire, err := cfg.TxEncoder()(builder.GetTx())
		require.NoError(t, err)
		txs = append(txs, wire)
	}
	return cfg, registry, txs
}

func repair626SequenceBatch(t *testing.T, txs tmtypes.Txs) tmtypes.Tx {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	keyID := sha256.Sum256(publicKey)
	activityIDs := make([][32]byte, len(txs))
	for i, tx := range txs {
		activityIDs[i] = sha256.Sum256(tx)
	}
	root := sequencertypes.TxRoot(activityIDs)
	batch := sequencertypes.MsgSequenceBatch{
		Height: 1, FromSeq: 1, ToSeq: uint64(len(txs)),
		PrevSeqHash: make([]byte, sequencertypes.HashLength),
		LastSeqHash: make([]byte, sequencertypes.HashLength),
		TxRoot:      root[:], KeyId: keyID[:],
	}
	digest, err := batch.SignBytes("block-route")
	require.NoError(t, err)
	batch.Sig = ed25519.Sign(privateKey, digest[:])
	wire, err := sequencertypes.EncodeSequenceBatchTx(batch)
	require.NoError(t, err)
	return wire
}

func repair626BlockService(t *testing.T, cfg client.TxConfig, registry codectypes.InterfaceRegistry, entries tmtypes.Txs) (txtypes.ServiceClient, *tmproto.Block, tmproto.BlockID) {
	t.Helper()
	conf, err := tmconfig.ResetTestRoot(t.TempDir(), "block-route")
	require.NoError(t, err)
	conf.Mode = tmconfig.ModeSeed
	conf.P2P.PexReactor = true
	genesis, err := tmtypes.GenesisDocFromFile(conf.GenesisFile())
	require.NoError(t, err)
	seed, err := node.New(t.Context(), conf, func() {}, kvstore.NewApplication(), genesis, nil, node.NoOpMetricsProvider(), tmtypes.DefaultConsensusPolicy())
	require.NoError(t, err)
	t.Cleanup(seed.(interface{ OnStop() }).OnStop)
	localClient, err := local.New(seed)
	require.NoError(t, err)
	block := tmtypes.MakeBlock(1, entries, &tmtypes.Commit{}, nil)
	block.Header.ChainID = genesis.ChainID
	block.Header.Time = genesis.GenesisTime
	block.Header.ProposerAddress = genesis.Validators[0].PubKey.Address()
	block.Header.ValidatorsHash = tmtypes.NewValidatorSet([]*tmtypes.Validator{tmtypes.NewValidator(genesis.Validators[0].PubKey, genesis.Validators[0].Power)}).Hash()
	require.NoError(t, block.ValidateBasic(tmtypes.DefaultConsensusPolicy()))
	parts, err := block.MakePartSet(tmtypes.BlockPartSizeBytes)
	require.NoError(t, err)
	blockID := tmtypes.BlockID{Hash: block.Hash(), PartSetHeader: parts.Header()}
	store, ok := seed.RPCEnvironment().BlockStore.(interface {
		SaveBlock(*tmtypes.Block, *tmtypes.PartSet, *tmtypes.Commit)
	})
	require.True(t, ok)
	store.SaveBlock(block, parts, &tmtypes.Commit{Height: 1, BlockID: blockID})
	protoBlock, err := block.ToProto()
	require.NoError(t, err)
	wireBlock, err := protoBlock.Marshal()
	require.NoError(t, err)
	protoBlock = new(tmproto.Block)
	require.NoError(t, protoBlock.Unmarshal(wireBlock))
	helper := baseapp.NewQueryServerTestHelper(sdk.NewContext(nil, tmproto.Header{Height: 1}, false), registry)
	RegisterTxService(helper, localClient, cfg, nil, registry)
	return txtypes.NewServiceClient(helper), protoBlock, blockID.ToProto()
}

func TestRepair_626_SequenceBatchPagination(t *testing.T) {
	cfg, registry, txs := repair626Transactions(t)
	batch := repair626SequenceBatch(t, txs)
	for _, layout := range []struct {
		name    string
		entries tmtypes.Txs
	}{
		{"batch_first", tmtypes.Txs{batch, txs[0], txs[1]}},
		{"batch_between", tmtypes.Txs{txs[0], batch, txs[1]}},
		{"batch_last", tmtypes.Txs{txs[0], txs[1], batch}},
	} {
		t.Run(layout.name, func(t *testing.T) {
			service, block, blockID := repair626BlockService(t, cfg, registry, layout.entries)
			for _, tc := range []struct {
				name    string
				page    *query.PageRequest
				indices []int
			}{
				{"default", nil, []int{0, 1}},
				{"forward", &query.PageRequest{Limit: 2}, []int{0, 1}},
				{"first_page", &query.PageRequest{Limit: 1}, []int{0}},
				{"offset", &query.PageRequest{Offset: 1, Limit: 1}, []int{1}},
				{"reverse", &query.PageRequest{Reverse: true}, []int{1, 0}},
				{"reverse_first_page", &query.PageRequest{Limit: 1, Reverse: true}, []int{1}},
				{"reverse_offset", &query.PageRequest{Offset: 1, Limit: 1, Reverse: true}, []int{0}},
			} {
				t.Run(tc.name, func(t *testing.T) {
					response, err := service.GetBlockWithTxs(context.Background(), &txtypes.GetBlockWithTxsRequest{Height: 1, Pagination: tc.page})
					require.NoError(t, err)
					require.NotNil(t, response.Pagination)
					require.Equal(t, uint64(2), response.Pagination.Total)
					require.Equal(t, blockID, *response.BlockId)
					require.Equal(t, block, response.Block)
					require.Len(t, response.Txs, len(tc.indices))
					for i, index := range tc.indices {
						decoded, err := cfg.TxDecoder()(txs[index])
						require.NoError(t, err)
						expected, err := decoded.(protoTxProvider).GetProtoTx().Marshal()
						require.NoError(t, err)
						actual, err := response.Txs[i].Marshal()
						require.NoError(t, err)
						require.Equal(t, expected, actual)
					}
				})
			}
			for _, offset := range []uint64{2, 3, math.MaxUint64} {
				_, err := service.GetBlockWithTxs(context.Background(), &txtypes.GetBlockWithTxsRequest{Height: 1, Pagination: &query.PageRequest{Offset: offset, Limit: 1}})
				require.ErrorContains(t, err, "out of range: cannot paginate 2 txs")
			}
		})
	}
}

func TestRepair_626_SequenceBatchOnly(t *testing.T) {
	cfg, registry, _ := repair626Transactions(t)
	batch := repair626SequenceBatch(t, nil)
	for _, tc := range []struct {
		name  string
		entry tmtypes.Tx
	}{
		{"encoded_batch", batch},
		{"domain_prefix", []byte(sequencertypes.SequenceBatchDomain)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service, block, blockID := repair626BlockService(t, cfg, registry, tmtypes.Txs{tc.entry})
			for _, page := range []*query.PageRequest{nil, {}, {Limit: 1}, {Reverse: true}, {Limit: query.MaxLimit}} {
				response, err := service.GetBlockWithTxs(context.Background(), &txtypes.GetBlockWithTxsRequest{Height: 1, Pagination: page})
				require.NoError(t, err)
				require.Empty(t, response.Txs)
				require.NotNil(t, response.Pagination)
				require.Zero(t, response.Pagination.Total)
				require.Equal(t, blockID, *response.BlockId)
				require.Equal(t, block, response.Block)
			}
			_, err := service.GetBlockWithTxs(context.Background(), &txtypes.GetBlockWithTxsRequest{Height: 1, Pagination: &query.PageRequest{Offset: 1}})
			require.ErrorContains(t, err, "out of range: cannot paginate 0 txs")
		})
	}
}

func TestRepair_626_InvalidEntry(t *testing.T) {
	cfg, registry, txs := repair626Transactions(t)
	batch := repair626SequenceBatch(t, txs)
	for _, tc := range []struct {
		name  string
		entry tmtypes.Tx
	}{
		{"invalid_transaction", []byte{0xff}},
		{"incomplete_domain", []byte(sequencertypes.SequenceBatchDomain[:len(sequencertypes.SequenceBatchDomain)-1])},
		{"domain_inside_entry", append([]byte{0xff}, batch...)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, decodeErr := cfg.TxDecoder()(tc.entry)
			require.Error(t, decodeErr)
			require.True(t, sdkerrors.ErrTxDecode.Is(decodeErr))
			service, _, _ := repair626BlockService(t, cfg, registry, tmtypes.Txs{batch, txs[0], tc.entry, txs[1]})
			for _, page := range []*query.PageRequest{nil, {Offset: 1, Limit: 1}, {Reverse: true}, {Offset: 1, Limit: 1, Reverse: true}} {
				response, err := service.GetBlockWithTxs(context.Background(), &txtypes.GetBlockWithTxsRequest{Height: 1, Pagination: page})
				require.ErrorContains(t, err, decodeErr.Error())
				require.Nil(t, response)
			}
		})
	}
}

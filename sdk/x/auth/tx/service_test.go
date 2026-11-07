package tx

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	tmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/node"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/local"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/query"
	txtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/tx"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
)

func TestGetBlockWithTxsReversePagination(t *testing.T) {
	testGetBlockWithTxsReversePagination(t)
}

func testGetBlockWithTxsReversePagination(t *testing.T) {
	for _, size := range []int{1, 5} {
		t.Run(fmt.Sprintf("block_size_%d", size), func(t *testing.T) {
			registry := codectypes.NewInterfaceRegistry()
			banktypes.RegisterInterfaces(registry)
			txConfig := NewTxConfig(codec.NewProtoCodec(registry), DefaultSignModes)
			conf, err := tmconfig.ResetTestRoot(t.TempDir(), "repair303")
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

			from := sdk.AccAddress(genesis.Validators[0].PubKey.Address())
			to := sdk.AccAddress([]byte("repair303-recipient!"))
			txs := make([]tmtypes.Tx, size)
			for i := range txs {
				builder := txConfig.NewTxBuilder()
				require.NoError(t, builder.SetMsgs(banktypes.NewMsgSend(from, to, sdk.NewCoins(sdk.NewInt64Coin("atom", int64(i+1))))))
				builder.SetMemo(fmt.Sprintf("transaction-%d", i))
				builder.SetGasLimit(200000)
				txs[i], err = txConfig.TxEncoder()(builder.GetTx())
				require.NoError(t, err)
			}
			block := tmtypes.MakeBlock(1, txs, &tmtypes.Commit{}, nil)
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

			helper := baseapp.NewQueryServerTestHelper(sdk.NewContext(nil, tmproto.Header{Height: 1}, false), registry)
			RegisterTxService(helper, localClient, txConfig, nil, registry)
			queryClient := txtypes.NewServiceClient(helper)
			fetch := func(offset, limit uint64, reverse bool) *txtypes.GetBlockWithTxsResponse {
				t.Helper()
				response, err := queryClient.GetBlockWithTxs(context.Background(), &txtypes.GetBlockWithTxsRequest{
					Height:     1,
					Pagination: &query.PageRequest{Offset: offset, Limit: limit, Reverse: reverse},
				})
				require.NoError(t, err)
				require.Equal(t, uint64(size), response.Pagination.Total)
				require.Equal(t, blockID.ToProto(), *response.BlockId)
				require.Equal(t, int64(1), response.Block.Header.Height)
				require.Len(t, response.Block.Data.Txs, size)
				return response
			}
			assertTx := func(actual *txtypes.Tx, index int) {
				t.Helper()
				decoded, err := txConfig.TxDecoder()(txs[index])
				require.NoError(t, err)
				expectedBytes, err := decoded.(protoTxProvider).GetProtoTx().Marshal()
				require.NoError(t, err)
				actualBytes, err := actual.Marshal()
				require.NoError(t, err)
				require.Equal(t, expectedBytes, actualBytes)
				require.NoError(t, actual.UnpackInterfaces(registry))
				require.Equal(t, decoded.(protoTxProvider).GetProtoTx(), actual)
			}

			defaultPage := fetch(0, 0, true)
			require.Len(t, defaultPage.Txs, size)
			for i, actual := range defaultPage.Txs {
				assertTx(actual, size-1-i)
			}
			for _, limit := range []uint64{1, 2, uint64(size), uint64(size + 1)} {
				var seen []string
				for offset := uint64(0); offset < uint64(size); offset += limit {
					page := fetch(offset, limit, true)
					want := limit
					if remaining := uint64(size) - offset; remaining < want {
						want = remaining
					}
					require.Len(t, page.Txs, int(want))
					for i, actual := range page.Txs {
						assertTx(actual, size-1-int(offset)-i)
						seen = append(seen, actual.Body.Memo)
					}
				}
				require.Len(t, seen, size)
				for i, memo := range seen {
					require.Equal(t, fmt.Sprintf("transaction-%d", size-1-i), memo)
				}
			}
			forward := fetch(0, 0, false)
			require.Len(t, forward.Txs, size)
			for i, actual := range forward.Txs {
				assertTx(actual, i)
			}
			for _, offset := range []uint64{uint64(size), math.MaxUint64} {
				_, err := queryClient.GetBlockWithTxs(context.Background(), &txtypes.GetBlockWithTxsRequest{
					Height: 1, Pagination: &query.PageRequest{Offset: offset, Limit: 1, Reverse: true},
				})
				require.ErrorContains(t, err, "out of range")
			}
		})
	}
}

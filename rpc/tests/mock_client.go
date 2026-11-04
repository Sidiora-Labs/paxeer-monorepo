package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto/ed25519"
	tmbytes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/bytes"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/mock"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	paxutils "github.com/Sidiora-Labs/Paxeer-X-Network/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/gogo/protobuf/proto"
)

type MockClient struct {
	mock.Client
	blocks                 [][][]byte
	headers                []*tmtypes.Header
	committedAppHashes     [][]byte
	committedResultsHashes [][]byte
	txResults              [][]*abci.ExecTxResult
	consParamUpdates       []*tmproto.ConsensusParams
	events                 [][]abci.Event

	mockedBlockResults        map[int64]*coretypes.ResultBlock
	mockedBlockByHashResults  map[string]*coretypes.ResultBlock
	mockedBlockResultsResults map[int64]*coretypes.ResultBlockResults
	mockedValidators          map[int64]*coretypes.ResultValidators
	mockedGenesis             *coretypes.ResultGenesis
}

func (c *MockClient) EvmNextPendingNonce(_ common.Address) uint64 {
	return 0
}

func (*MockClient) EvmTxByHash(common.Hash) (tmtypes.Tx, bool) {
	return nil, false
}

func (c *MockClient) EvmProxy(common.Address) (*url.URL, bool) {
	return nil, false
}

func (c *MockClient) Block(_ context.Context, h *int64) (*coretypes.ResultBlock, error) {
	if c.mockedBlockResults != nil {
		blockNum := int64(-1)
		if h != nil {
			blockNum = *h
		}
		res, ok := c.mockedBlockResults[blockNum]
		if !ok {
			return nil, errors.New("not found")
		}
		return res, nil
	}
	if h == nil {
		return c.getBlock(int64(len(c.blocks))), nil
	}
	return c.getBlock(*h), nil
}

func (c *MockClient) BlockByHash(_ context.Context, hash tmbytes.HexBytes) (*coretypes.ResultBlock, error) {
	if c.mockedBlockByHashResults != nil {
		res, ok := c.mockedBlockByHashResults[hash.String()]
		if !ok {
			return nil, errors.New("not found")
		}
		return res, nil
	}
	for i := range c.blocks {
		height := i + 1
		rb := c.getBlock(int64(height))
		if bytes.Equal(rb.BlockID.Hash, hash) {
			return rb, nil
		}
	}
	return nil, errors.New("not found")
}

func (c *MockClient) getBlock(i int64) *coretypes.ResultBlock {
	if i < 1 || i > int64(len(c.headers)) {
		return nil
	}
	header, err := tmtypes.HeaderFromProto(proto.Clone(c.headers[i-1].ToProto()).(*tmproto.Header))
	if err != nil {
		panic(err)
	}
	return &coretypes.ResultBlock{
		BlockID: tmtypes.BlockID{Hash: header.Hash()},
		Block: &tmtypes.Block{
			Data:       tmtypes.Data{Txs: paxutils.Map(c.blocks[i-1], func(tx []byte) tmtypes.Tx { return tmtypes.Tx(bytes.Clone(tx)) })},
			Header:     header,
			LastCommit: &tmtypes.Commit{Height: i - 1},
		},
	}
}

func (c *MockClient) Header(ctx context.Context, h *int64) (*coretypes.ResultHeader, error) {
	block, err := c.Block(ctx, h)
	if err != nil {
		return nil, err
	}
	if block == nil || block.Block == nil {
		return nil, errors.New("not found")
	}
	return &coretypes.ResultHeader{Header: &block.Block.Header}, nil
}

func (c *MockClient) Genesis(context.Context) (*coretypes.ResultGenesis, error) {
	if c.mockedGenesis != nil {
		return c.mockedGenesis, nil
	}
	return &coretypes.ResultGenesis{Genesis: &tmtypes.GenesisDoc{InitialHeight: 1}}, nil
}

func (c *MockClient) BlockResults(_ context.Context, height *int64) (*coretypes.ResultBlockResults, error) {
	if c.mockedBlockResultsResults != nil {
		blockNum := int64(-1)
		if height != nil {
			blockNum = *height
		}
		res, ok := c.mockedBlockResultsResults[blockNum]
		if !ok {
			return nil, errors.New("not found")
		}
		return res, nil
	}
	return &coretypes.ResultBlockResults{
		TxsResults:            c.txResults[*height-1],
		ConsensusParamUpdates: c.consParamUpdates[*height-1],
	}, nil
}

func (c *MockClient) Status(context.Context) (*coretypes.ResultStatus, error) {
	latest := int64(len(c.blocks))
	if c.mockedBlockResults != nil {
		for h := range c.mockedBlockResults {
			if h > latest {
				latest = h
			}
		}
	}
	if latest <= 0 {
		latest = 1
	}
	return &coretypes.ResultStatus{
		SyncInfo: coretypes.SyncInfo{
			LatestBlockHeight:   latest,
			EarliestBlockHeight: 1,
		},
	}, nil
}

func (c *MockClient) recordBlockResult(txResults []*abci.ExecTxResult, consParamUpdates *tmproto.ConsensusParams, events []abci.Event) {
	c.txResults = append(c.txResults, txResults)
	c.consParamUpdates = append(c.consParamUpdates, consParamUpdates)
	c.events = append(c.events, events)
}

func (c *MockClient) Validators(ctx context.Context, height *int64, page, perPage *int) (*coretypes.ResultValidators, error) {
	var captured *coretypes.ResultValidators
	if c.mockedValidators != nil {
		blockNum := int64(-1)
		if height != nil {
			blockNum = *height
		}
		res, ok := c.mockedValidators[blockNum]
		if !ok {
			return nil, errors.New("not found")
		}
		captured = res
	} else {
		validator := tmtypes.NewValidator(ed25519.TestSecretKey([]byte("rpc-test-validator")).Public(), 1)
		captured = &coretypes.ResultValidators{Validators: []*tmtypes.Validator{validator}, Count: 1, Total: 1}
		if height != nil {
			captured.BlockHeight = *height
		}
	}
	// Each trace contains a single recorded page. Replay used only these
	// captured validators, so they form the complete set for this fixture.
	total := len(captured.Validators)
	pageNumber, pageSize := 1, 30
	if page != nil {
		pageNumber = *page
	}
	if perPage != nil && *perPage > 0 {
		pageSize = *perPage
	}
	if pageSize > 100 {
		pageSize = 100
	}
	pages := (total + pageSize - 1) / pageSize
	if pageNumber < 1 || pageNumber > pages {
		return nil, fmt.Errorf("validator page %d is outside range [1, %d]", pageNumber, pages)
	}
	start := (pageNumber - 1) * pageSize
	end := min(start+pageSize, total)
	res := *captured
	res.Validators = append([]*tmtypes.Validator(nil), captured.Validators[start:end]...)
	res.Count = total
	res.Total = total
	return &res, nil
}

func (c *MockClient) Events(_ context.Context, req *coretypes.RequestEvents) (*coretypes.ResultEvents, error) {
	if strings.Contains(req.Filter.Query, "tm.event = 'NewBlock'") {
		var cursor int
		var err error
		if req.After != "" {
			cursor, err = strconv.Atoi(req.After)
			if err != nil {
				panic("invalid cursor")
			}
		} else {
			cursor = 0
		}
		res := &coretypes.ResultEvents{Oldest: strconv.FormatInt(int64(cursor), 10), Items: make([]*coretypes.EventItem, 0, req.MaxItems)}
		for ; cursor < len(c.blocks); cursor++ {
			if len(res.Items) >= req.MaxItems {
				res.More = true
				break
			}
			block := c.getBlock(int64(cursor + 1))
			data := tmtypes.EventDataNewBlock{
				Block:   block.Block,
				BlockID: block.BlockID,
			}
			eventData, err := json.Marshal(data)
			if err != nil {
				panic(err)
			}
			wrappedData := evmrpc.EventItemDataWrapper{
				Type:  "NewBlock",
				Value: eventData,
			}
			bz, err := json.Marshal(wrappedData)
			if err != nil {
				panic(err)
			}
			res.Items = append(res.Items, &coretypes.EventItem{
				Cursor: strconv.FormatInt(int64(cursor), 10),
				Data:   bz,
				Event:  "event",
			})
		}
		res.Newest = strconv.FormatInt(int64(cursor-1), 10)
		return res, nil
	} else {
		panic(fmt.Sprintf("event query %s is not expected to be made by EVM RPC", req.Filter.Query))
	}
}

func (c *MockClient) UnconfirmedTxs(context.Context, *int, *int) (*coretypes.ResultUnconfirmedTxs, error) {
	return &coretypes.ResultUnconfirmedTxs{}, nil
}

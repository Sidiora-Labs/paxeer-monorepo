package utils

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	ctypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
)

const (
	defaultPage  = 1
	defaultLimit = 30 // should be consistent with tendermint/tendermint/rpc/core/pipe.go:19
)

// Proposer contains metadata of a governance proposal used for querying a
// proposer.
type Proposer struct {
	ProposalID uint64 `json:"proposal_id" yaml:"proposal_id"`
	Proposer   string `json:"proposer" yaml:"proposer"`
}

// NewProposer returns a new Proposer given id and proposer
func NewProposer(proposalID uint64, proposer string) Proposer {
	return Proposer{proposalID, proposer}
}

func (p Proposer) String() string {
	return fmt.Sprintf("Proposal with ID %d was proposed by %s", p.ProposalID, p.Proposer)
}

// QueryDepositsByTxQuery reconstructs aggregate deposits from successful indexed transactions.
func QueryDepositsByTxQuery(ctx context.Context, clientCtx client.Context, params types.QueryProposalParams) ([]byte, error) {
	deposits, err := queryDeposits(ctx, clientCtx, params.ProposalID, "")
	if err != nil {
		return nil, err
	}
	return clientCtx.LegacyAmino.MarshalAsJSON(deposits)
}

// QueryVotesByTxQuery reconstructs the last successful vote per voter before pagination.
func QueryVotesByTxQuery(ctx context.Context, clientCtx client.Context, params types.QueryProposalVotesParams) ([]byte, error) {
	votes, err := queryVotes(ctx, clientCtx, params.ProposalID, "")
	if err != nil {
		return nil, err
	}
	start, end := client.Paginate(len(votes), params.Page, params.Limit, 100)
	if start < 0 || end < 0 {
		votes = []types.Vote{}
	} else {
		votes = votes[start:end]
	}
	return clientCtx.LegacyAmino.MarshalAsJSON(votes)
}

// QueryVoteByTxQuery queries the last successful vote of the requested account.
func QueryVoteByTxQuery(ctx context.Context, clientCtx client.Context, params types.QueryVoteParams) ([]byte, error) {
	votes, err := queryVotes(ctx, clientCtx, params.ProposalID, params.Voter.String())
	if err != nil {
		return nil, err
	}
	if len(votes) > 0 {
		return clientCtx.Codec.MarshalAsJSON(&votes[0])
	}
	return nil, fmt.Errorf("address '%s' did not vote on proposalID %d", params.Voter, params.ProposalID)
}

// QueryDepositByTxQuery queries the aggregate deposit of the requested account.
func QueryDepositByTxQuery(ctx context.Context, clientCtx client.Context, params types.QueryDepositParams) ([]byte, error) {
	deposits, err := queryDeposits(ctx, clientCtx, params.ProposalID, params.Depositor.String())
	if err != nil {
		return nil, err
	}
	if len(deposits) > 0 {
		return clientCtx.Codec.MarshalAsJSON(&deposits[0])
	}
	return nil, fmt.Errorf("address '%s' did not deposit to proposalID %d", params.Depositor, params.ProposalID)
}

// QueryProposerByTxQuery queries the proposer identified by the submission's message log.
func QueryProposerByTxQuery(ctx context.Context, clientCtx client.Context, proposalID uint64) (Proposer, error) {
	deposit, err := queryInitialDepositByTxQuery(ctx, clientCtx, proposalID)
	if err != nil {
		return Proposer{}, err
	}
	return NewProposer(proposalID, deposit.Depositor), nil
}

func queryDeposits(ctx context.Context, clientCtx client.Context, proposalID uint64, depositor string) ([]types.Deposit, error) {
	initial, err := queryInitialDepositByTxQuery(ctx, clientCtx, proposalID)
	if err != nil {
		return nil, err
	}
	var deposits []types.Deposit
	positions := make(map[string]int)
	add := func(owner string, amount sdk.Coins) {
		if (depositor != "" && owner != depositor) || amount.IsZero() {
			return
		}
		if i, ok := positions[owner]; ok {
			deposits[i].Amount = deposits[i].Amount.Add(amount...)
		} else {
			positions[owner] = len(deposits)
			deposits = append(deposits, types.Deposit{ProposalId: proposalID, Depositor: owner, Amount: amount})
		}
	}
	add(initial.Depositor, initial.Amount)
	result, err := combineEvents(ctx, clientCtx, historyEvents(proposalID, types.EventTypeProposalDeposit, depositor,
		types.TypeMsgDeposit, sdk.MsgTypeURL(&types.MsgDeposit{}))...)
	if err != nil {
		return nil, err
	}
	for _, info := range result.Txs {
		for _, msg := range info.GetTx().GetMsgs() {
			if dep, ok := msg.(*types.MsgDeposit); ok && dep.ProposalId == proposalID {
				add(dep.Depositor, dep.Amount)
			}
		}
	}
	return deposits, nil
}

func queryVotes(ctx context.Context, clientCtx client.Context, proposalID uint64, voter string) ([]types.Vote, error) {
	result, err := combineEvents(ctx, clientCtx, historyEvents(proposalID, types.EventTypeProposalVote, voter,
		types.TypeMsgVote, sdk.MsgTypeURL(&types.MsgVote{}),
		types.TypeMsgVoteWeighted, sdk.MsgTypeURL(&types.MsgVoteWeighted{}))...)
	if err != nil {
		return nil, err
	}
	var history []types.Vote
	last := make(map[string]int)
	for _, info := range result.Txs {
		for _, msg := range info.GetTx().GetMsgs() {
			var vote types.Vote
			switch msg := msg.(type) {
			case *types.MsgVote:
				vote = types.Vote{ProposalId: msg.ProposalId, Voter: msg.Voter, Options: types.NewNonSplitVoteOption(msg.Option)}
			case *types.MsgVoteWeighted:
				vote = types.Vote{ProposalId: msg.ProposalId, Voter: msg.Voter, Options: msg.Options}
			default:
				continue
			}
			if vote.ProposalId != proposalID || (voter != "" && vote.Voter != voter) {
				continue
			}
			last[vote.Voter] = len(history)
			history = append(history, vote)
		}
	}
	var votes []types.Vote
	for i, vote := range history {
		if last[vote.Voter] == i {
			votes = append(votes, vote)
		}
	}
	return votes, nil
}

func historyEvents(proposalID uint64, eventType, account string, actions ...string) [][]string {
	groups := make([][]string, 0, len(actions))
	for _, action := range actions {
		events := []string{
			fmt.Sprintf("%s.%s='%s'", sdk.EventTypeMessage, sdk.AttributeKeyAction, action),
			fmt.Sprintf("%s.%s='%d'", eventType, types.AttributeKeyProposalID, proposalID),
		}
		if account != "" {
			events = append(events, fmt.Sprintf("%s.%s='%s'", sdk.EventTypeMessage, sdk.AttributeKeySender, account))
		}
		groups = append(groups, events)
	}
	return groups
}

// QueryProposalByID takes a proposalID and returns a proposal
func QueryProposalByID(proposalID uint64, clientCtx client.Context, queryRoute string) ([]byte, error) {
	params := types.NewQueryProposalParams(proposalID)
	bz, err := clientCtx.LegacyAmino.MarshalAsJSON(params)
	if err != nil {
		return nil, err
	}

	res, _, err := clientCtx.QueryWithData(fmt.Sprintf("custom/%s/proposal", queryRoute), bz)
	if err != nil {
		return nil, err
	}

	return res, err
}

// combineEvents exhausts each event stream independently, deduplicates transactions
// and merges successful results in block/transaction order before decoding messages.
func combineEvents(ctx context.Context, clientCtx client.Context, eventGroups ...[]string) (*sdk.SearchTxsResult, error) {
	node, err := clientCtx.GetNode()
	if err != nil {
		return nil, err
	}
	var txs []*ctypes.ResultTx
	seen := make(map[string]bool)
	for _, events := range eventGroups {
		for page := defaultPage; ; page++ {
			limit := defaultLimit
			result, err := node.TxSearch(ctx, strings.Join(events, " AND "), true, &page, &limit, "asc")
			if err != nil {
				return nil, err
			}
			for _, tx := range result.Txs {
				hash := tx.Hash.String()
				if !seen[hash] && tx.TxResult.Code == 0 {
					seen[hash] = true
					txs = append(txs, tx)
				}
			}
			if page*defaultLimit >= result.TotalCount {
				break
			}
			if len(result.Txs) == 0 {
				return nil, fmt.Errorf("transaction index returned an empty page before exhausting event stream")
			}
		}
	}
	sort.Slice(txs, func(i, j int) bool {
		if txs[i].Height == txs[j].Height {
			return txs[i].Index < txs[j].Index
		}
		return txs[i].Height < txs[j].Height
	})
	allTxs := make([]*sdk.TxResponse, 0, len(txs))
	blocks := make(map[int64]*ctypes.ResultBlock)
	for _, tx := range txs {
		decoded, err := clientCtx.TxConfig.TxDecoder()(tx.Tx)
		if err != nil {
			return nil, err
		}
		anyTx, ok := decoded.(interface{ AsAny() *codectypes.Any })
		if !ok {
			return nil, fmt.Errorf("expecting a type implementing intoAny, got: %T", decoded)
		}
		block, ok := blocks[tx.Height]
		if !ok {
			block, err = node.Block(ctx, &tx.Height)
			if err != nil {
				return nil, err
			}
			blocks[tx.Height] = block
		}
		allTxs = append(allTxs, sdk.NewResponseResultTx(tx, anyTx.AsAny(), block.Block.Time.Format(time.RFC3339)))
	}
	return &sdk.SearchTxsResult{Txs: allTxs}, nil
}

// queryInitialDepositByTxQuery binds each submission to its message-scoped proposal event.
func queryInitialDepositByTxQuery(ctx context.Context, clientCtx client.Context, proposalID uint64) (types.Deposit, error) {
	result, err := combineEvents(ctx, clientCtx, historyEvents(proposalID, types.EventTypeSubmitProposal, "",
		types.TypeMsgSubmitProposal, sdk.MsgTypeURL(&types.MsgSubmitProposal{}))...)
	if err != nil {
		return types.Deposit{}, err
	}
	for _, info := range result.Txs {
		for i, msg := range info.GetTx().GetMsgs() {
			submission, ok := msg.(*types.MsgSubmitProposal)
			if !ok {
				continue
			}
			for _, log := range info.Logs {
				if log.MsgIndex != uint32(i) {
					continue
				}
				for _, event := range log.Events {
					if event.Type != types.EventTypeSubmitProposal {
						continue
					}
					for _, attribute := range event.Attributes {
						if attribute.Key == types.AttributeKeyProposalID && attribute.Value == strconv.FormatUint(proposalID, 10) {
							return types.Deposit{ProposalId: proposalID, Depositor: submission.Proposer, Amount: submission.InitialDeposit}, nil
						}
					}
				}
			}
		}
	}
	return types.Deposit{}, sdkerrors.ErrNotFound.Wrapf("failed to find the initial deposit for proposalID %d", proposalID)
}

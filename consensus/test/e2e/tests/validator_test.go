package e2e_test

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/crypto"
	cstypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/consensus/types"
	e2e "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/test/e2e/pkg"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

// Tests that validator sets are available and correct according to
// scheduled validator updates.
func TestValidator_Sets(t *testing.T) {
	testNode(t, func(ctx context.Context, t *testing.T, node e2e.Node) {
		client, err := node.Client()
		require.NoError(t, err)
		status, err := client.Status(ctx)
		require.NoError(t, err)

		first := status.SyncInfo.EarliestBlockHeight

		// for nodes that have to catch up, we should only
		// check the validator sets for nodes after this
		// point, to avoid inconsistencies with backfill.
		if node.StartAt > first {
			first = node.StartAt
		}

		last := status.SyncInfo.LatestBlockHeight

		// skip first block if node is pruning blocks, to avoid race conditions
		if node.RetainBlocks > 0 {
			first++
		}

		valSchedule := newValidatorSchedule(*node.Testnet)
		require.NoError(t, valSchedule.Increment(first-node.Testnet.InitialHeight))

		for h := first; h <= last; h++ {
			validators := []*types.Validator{}
			perPage := 100
			for page := 1; ; page++ {
				resp, err := client.Validators(ctx, &(h), &(page), &perPage)
				require.NoError(t, err)
				validators = append(validators, resp.Validators...)
				if len(validators) == resp.Total {
					break
				}
			}
			require.Equal(t, valSchedule.Set.Validators, validators,
				"incorrect validator set at height %v", h)
			require.NoError(t, valSchedule.Increment(1))
		}
	})
}

// Tests that a validator proposes blocks when it's supposed to. It tolerates some
// missed blocks, e.g. due to testnet perturbations.
func TestValidator_Propose(t *testing.T) {
	ctx := t.Context()

	blocks := fetchBlockChain(ctx, t)
	commits := fetchProposalCommits(ctx, t, blocks)
	testNode(t, func(ctx context.Context, t *testing.T, node e2e.Node) {
		if node.Mode != e2e.ModeValidator {
			return
		}
		expectCount, proposeCount, err := checkValidatorProposals(node, blocks, commits)
		require.NoError(t, err)

		require.False(t, proposeCount == 0 && expectCount > 0,
			"node did not propose any blocks (expected %v)", expectCount)
		if expectCount > 5 {
			require.GreaterOrEqual(t, proposeCount, 3, "validator didn't propose even 3 blocks")
		}
	})
}

// fetchProposalCommits uses the next block's commit and obtains the tip's commit
// from an archive node, so pruned nodes can still be qualified against all blocks.
func fetchProposalCommits(ctx context.Context, t *testing.T, blocks []*types.Block) map[int64]*types.Commit {
	t.Helper()
	commits := make(map[int64]*types.Commit, len(blocks))
	for _, block := range blocks[1:] {
		require.NotNil(t, block.LastCommit)
		commits[block.LastCommit.Height] = block.LastCommit
	}
	tip := blocks[len(blocks)-1]
	for _, node := range loadTestnet(t).ArchiveNodes() {
		client, err := node.Client()
		require.NoError(t, err)
		status, err := client.Status(ctx)
		require.NoError(t, err)
		if status.SyncInfo.LatestBlockHeight < tip.Height {
			continue
		}
		result, err := client.Commit(ctx, &tip.Height)
		require.NoError(t, err)
		require.NotNil(t, result.Commit)
		commits[tip.Height] = result.Commit
		return commits
	}
	t.Fatal("no archive node has the proposal tip commit")
	return nil
}

func proposerPin(testnet *e2e.Testnet) (cstypes.LeaderPin, error) {
	if len(testnet.LeaderPin) == 0 {
		return cstypes.LeaderPin{}, nil
	}
	keys := make([]crypto.PubKey, len(testnet.LeaderPin))
	for i, node := range testnet.LeaderPin {
		keys[i] = node.PrivvalKey.Public()
	}
	return cstypes.NewLeaderPin(testnet.LeaderPinHeight, keys)
}

func checkValidatorProposals(node e2e.Node, blocks []*types.Block, commits map[int64]*types.Commit) (int, int, error) {
	pin, err := proposerPin(node.Testnet)
	if err != nil {
		return 0, 0, err
	}
	schedule := newValidatorSchedule(*node.Testnet)
	address := node.PrivvalKey.Public().Address()
	expectCount, proposeCount := 0, 0
	for _, block := range blocks {
		if block.Height < schedule.height {
			return 0, 0, fmt.Errorf("proposal heights are out of order: %d", block.Height)
		}
		if err := schedule.Increment(block.Height - schedule.height); err != nil {
			return 0, 0, err
		}
		commit := commits[block.Height]
		if commit == nil || commit.Height != block.Height || commit.Round < 0 || !bytes.Equal(commit.BlockID.Hash, block.Hash()) {
			return 0, 0, fmt.Errorf("missing or mismatched proposal commit at height %d", block.Height)
		}
		round := cstypes.RoundState{HRS: cstypes.HRS{Height: block.Height, Round: commit.Round}, Validators: schedule.Set, LeaderPin: pin}
		// A valid block can be carried from an earlier round. Its header keeps
		// the original creator, rather than the proposer of the final round.
		validCreator := false
		for r := int32(0); ; r++ {
			round.Round = r
			if bytes.Equal(round.Leader().Address(), block.ProposerAddress) {
				validCreator = true
				break
			}
			if r == commit.Round {
				break
			}
		}
		if !validCreator {
			return 0, 0, fmt.Errorf("unexpected proposer at height %d through round %d: %X", block.Height, commit.Round, block.ProposerAddress)
		}
		// Count round-zero opportunities while allowing perturbations to move a
		// proposal to a later round. The actual proposer is checked at that round.
		round.Round = 0
		if bytes.Equal(round.Leader().Address(), address) {
			expectCount++
			if bytes.Equal(block.ProposerAddress, address) {
				proposeCount++
			}
		}
	}
	return expectCount, proposeCount, nil
}

// Tests that a validator signs blocks when it's supposed to. It tolerates some
// missed blocks, e.g. due to testnet perturbations.
func TestValidator_Sign(t *testing.T) {
	ctx := t.Context()

	blocks := fetchBlockChain(ctx, t)
	testNode(t, func(ctx context.Context, t *testing.T, node e2e.Node) {
		if node.Mode != e2e.ModeValidator {
			return
		}
		address := node.PrivvalKey.Public().Address()
		valSchedule := newValidatorSchedule(*node.Testnet)

		expectCount := 0
		signCount := 0
		for _, block := range blocks[1:] { // Skip first block, since it has no signatures
			signed := false
			for _, sig := range block.LastCommit.Signatures {
				if bytes.Equal(sig.ValidatorAddress, address) {
					signed = true
					break
				}
			}
			if valSchedule.Set.HasAddress(address) {
				expectCount++
				if signed {
					signCount++
				}
			} else {
				require.False(t, signed, "unexpected signature for block %v", block.LastCommit.Height)
			}
			require.NoError(t, valSchedule.Increment(1))
		}

		require.False(t, signCount == 0 && expectCount > 0,
			"validator did not sign any blocks (expected %v)", expectCount)
		if expectCount > 7 {
			require.GreaterOrEqual(t, signCount, 3, "validator didn't sign even 3 blocks (expected %v)", expectCount)
		}
	})
}

// validatorSchedule is a validator set iterator, which takes into account
// validator set updates.
type validatorSchedule struct {
	Set     *types.ValidatorSet
	height  int64
	updates map[int64]map[*e2e.Node]int64
}

func newValidatorSchedule(testnet e2e.Testnet) *validatorSchedule {
	valMap := testnet.Validators                  // genesis validators
	if v, ok := testnet.ValidatorUpdates[0]; ok { // InitChain validators
		valMap = v
	}
	return &validatorSchedule{
		height:  testnet.InitialHeight,
		Set:     types.NewValidatorSet(makeVals(valMap)),
		updates: testnet.ValidatorUpdates,
	}
}

func (s *validatorSchedule) Increment(heights int64) error {
	for i := int64(0); i < heights; i++ {
		s.height++
		if s.height > 2 {
			// validator set updates are offset by 2, since they only take effect
			// two blocks after they're returned.
			if update, ok := s.updates[s.height-2]; ok {
				if err := s.Set.UpdateWithChangeSet(makeVals(update)); err != nil {
					return err
				}
			}
		}
		s.Set.IncrementProposerPriority(1)
	}
	return nil
}

func makeVals(valMap map[*e2e.Node]int64) []*types.Validator {
	vals := make([]*types.Validator, 0, len(valMap))
	for node, power := range valMap {
		vals = append(vals, types.NewValidator(node.PrivvalKey.Public(), power))
	}
	return vals
}

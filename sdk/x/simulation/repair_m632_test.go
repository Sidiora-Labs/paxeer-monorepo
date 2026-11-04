package simulation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	cryptoproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/crypto"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	simtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/simulation"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func repair632Params(t *testing.T) Params {
	t.Helper()
	matrix, err := CreateTransitionMatrix([][]int{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}})
	require.NoError(t, err)
	return Params{initialLivenessWeightings: []int{1, 0, 0}, livenessTransitionMatrix: matrix}
}

func repair632Validator(power int64) abci.ValidatorUpdate {
	return abci.ValidatorUpdate{PubKey: cryptoproto.PublicKey{Sum: &cryptoproto.PublicKey_Ed25519{Ed25519: bytes.Repeat([]byte{7}, 32)}}, Power: power}
}

func repair632App(t *testing.T) (*baseapp.BaseApp, *sdk.KVStoreKey) {
	t.Helper()
	app := baseapp.NewBaseApp(t.Name(), dbm.NewMemDB(), nil, nil, &testutil.TestAppOpts{})
	key := sdk.NewKVStoreKey("repair632")
	app.MountStores(key)
	app.SetInitChainer(func(ctx sdk.Context, req abci.RequestInitChain) abci.ResponseInitChain {
		ctx.KVStore(key).Set([]byte("genesis-chain"), []byte(req.ChainId))
		return abci.ResponseInitChain{Validators: []abci.ValidatorUpdate{repair632Validator(10)}}
	})
	return app, key
}

func repair632Config(t *testing.T) SimulationConfig {
	t.Helper()
	return SimulationConfig{
		Seed: 31, NumBlocks: 3, BlockSize: 1, Params: repair632Params(t),
		Genesis:      abci.RequestInitChain{ChainId: "repair632", Time: time.Unix(1700000000, 0).UTC()},
		LogDirectory: t.TempDir(),
	}
}

func TestRepairM_632_SimulationLogOverwrite(t *testing.T) {
	directory := t.TempDir()
	const count = 24
	paths := make([]string, count)
	errs := make([]error, count)
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			writer := &StandardLogWriter{Directory: directory}
			writer.AddEntry(BeginBlockEntry(int64(i + 1)))
			paths[i], errs[i] = writer.PrintLogs()
		}(i)
	}
	wg.Wait()
	seen := make(map[string]bool)
	for i, name := range paths {
		require.NoError(t, errs[i])
		require.False(t, seen[name], "exports must use exclusive, distinct paths")
		seen[name] = true
		// Reopen after every writer has closed: later exports must preserve earlier evidence.
		content, err := os.ReadFile(name)
		require.NoError(t, err)
		var entry OperationEntry
		require.NoError(t, json.Unmarshal(content, &entry))
		require.Equal(t, int64(i+1), entry.Height)
	}
	files, err := os.ReadDir(directory)
	require.NoError(t, err)
	require.Len(t, files, count)
	f, err := os.CreateTemp(directory, "closed-*")
	require.NoError(t, err)
	require.NoError(t, f.Close())
	err = writeLogEntries(f, []OperationEntry{BeginBlockEntry(1)})
	require.ErrorIs(t, err, os.ErrClosed)
	joined, ok := err.(interface{ Unwrap() []error })
	require.True(t, ok)
	require.Len(t, joined.Unwrap(), 2, "both write and close errors must survive")
	broken := &StandardLogWriter{Directory: filepath.Join(paths[0], "impossible")}
	name, err := broken.PrintLogs()
	require.Error(t, err)
	require.Empty(t, name)
	f, err = os.CreateTemp(directory, "invalid-json-*")
	require.NoError(t, err)
	require.Error(t, writeLogEntries(f, []OperationEntry{{Operation: json.RawMessage("{")}}))
	_, err = f.WriteString("already closed")
	require.ErrorIs(t, err, os.ErrClosed)
}

func TestRepairM_632_ValidatorPowerUpdate(t *testing.T) {
	p := repair632Params(t)
	p.initialLivenessWeightings = []int{0, 0, 1}
	p.evidenceFraction = .9
	r := rand.New(rand.NewSource(14))
	events := NewEventStats()
	validators := newMockValidators(r, []abci.ValidatorUpdate{repair632Validator(10)}, p)
	validators = updateValidators(t, r, p, validators, []abci.ValidatorUpdate{repair632Validator(45)}, events.Tally)
	for _, val := range validators {
		require.Equal(t, 2, val.livenessState)
	}
	found := false
	for i := 0; i < 20; i++ {
		request, err := RandomRequestBeginBlock(r, p, validators, VoteHistory{}, events.Tally, tmproto.Header{Height: int64(i + 1), Time: time.Unix(int64(i), 0)})
		require.NoError(t, err)
		require.Len(t, request.LastCommitInfo.Votes, 1)
		require.Equal(t, int64(45), request.LastCommitInfo.Votes[0].Validator.Power)
		require.False(t, request.LastCommitInfo.Votes[0].SignedLastBlock)
		for _, evidence := range request.ByzantineValidators {
			found = true
			require.Equal(t, int64(45), evidence.Validator.Power)
			require.Equal(t, int64(45), evidence.TotalVotingPower)
		}
	}
	require.True(t, found, "power change must reach generated evidence")
	require.Equal(t, 1, events["end_block"]["validator_updates"]["updated"])
}

func TestRepairM_632_ValidatorLivenessUpdate(t *testing.T) {
	p := repair632Params(t)
	r := rand.New(rand.NewSource(3))
	validators := newMockValidators(r, []abci.ValidatorUpdate{repair632Validator(10)}, p)
	events := NewEventStats()
	first, err := RandomRequestBeginBlock(r, p, validators, VoteHistory{}, events.Tally, tmproto.Header{Height: 1})
	require.NoError(t, err)
	require.True(t, first.LastCommitInfo.Votes[0].SignedLastBlock)
	p.livenessTransitionMatrix, err = CreateTransitionMatrix([][]int{{0, 0, 1}, {0, 1, 0}, {1, 0, 0}})
	require.NoError(t, err)
	for i, signed := range []bool{false, true, false, true} {
		request, err := RandomRequestBeginBlock(r, p, validators, VoteHistory{}, events.Tally, tmproto.Header{Height: int64(i + 2)})
		require.NoError(t, err)
		require.Equal(t, signed, request.LastCommitInfo.Votes[0].SignedLastBlock)
		for _, val := range validators {
			if signed {
				require.Equal(t, 0, val.livenessState)
			} else {
				require.Equal(t, 2, val.livenessState)
			}
		}
	}
}

func TestRepairM_632_EvidenceHistoryBounds(t *testing.T) {
	p := repair632Params(t)
	r := rand.New(rand.NewSource(8))
	events := NewEventStats()
	validators := newMockValidators(r, []abci.ValidatorUpdate{repair632Validator(10)}, p)
	history := VoteHistory{StartHeight: 40}
	for h := int64(40); h <= 42; h++ {
		request, err := RandomRequestBeginBlock(r, p, validators, VoteHistory{}, events.Tally, tmproto.Header{Height: h, Time: time.Unix(h, 0)})
		require.NoError(t, err)
		history.Times = append(history.Times, request.Header.Time)
		history.Votes = append(history.Votes, request.LastCommitInfo.Votes)
	}
	history.Votes[1] = nil
	validators = updateValidators(t, r, p, validators, []abci.ValidatorUpdate{repair632Validator(0)}, events.Tally)
	require.Empty(t, validators)
	p.evidenceFraction, p.pastEvidenceFraction = .9, 1
	heights := make(map[int64]bool)
	for i := 0; i < 40; i++ {
		request, err := RandomRequestBeginBlock(r, p, validators, history, events.Tally, tmproto.Header{Height: 43, Time: time.Unix(43, 0)})
		require.NoError(t, err)
		require.Empty(t, request.LastCommitInfo.Votes)
		for _, evidence := range request.ByzantineValidators {
			require.Contains(t, []int64{40, 42}, evidence.Height)
			require.Equal(t, time.Unix(evidence.Height, 0), evidence.Time)
			require.Equal(t, int64(10), evidence.Validator.Power)
			require.Equal(t, int64(10), evidence.TotalVotingPower)
			heights[evidence.Height] = true
		}
	}
	require.Len(t, heights, 2)
	for _, invalid := range []VoteHistory{
		{StartHeight: 40, Times: history.Times, Votes: history.Votes[:1]},
		{StartHeight: 0, Times: history.Times, Votes: history.Votes},
		{StartHeight: 42, Times: history.Times, Votes: history.Votes},
	} {
		_, err := RandomRequestBeginBlock(r, p, validators, invalid, events.Tally, tmproto.Header{Height: 43})
		require.ErrorContains(t, err, "vote history")
	}
	empty := VoteHistory{StartHeight: 42, Times: []time.Time{{}}, Votes: [][]abci.VoteInfo{nil}}
	for i := 0; i < 20; i++ {
		req, err := RandomRequestBeginBlock(r, p, validators, empty, events.Tally, tmproto.Header{Height: 43})
		require.NoError(t, err)
		require.Empty(t, req.ByzantineValidators)
	}
	require.Positive(t, events["begin_block"]["evidence"]["history_unavailable"])
	// A short history is valid even when its available range ends well before the header.
	req, err := RandomRequestBeginBlock(r, p, validators, history, events.Tally, tmproto.Header{Height: 1000})
	require.NoError(t, err)
	for _, ev := range req.ByzantineValidators {
		require.Contains(t, []int64{40, 42}, ev.Height)
	}
}

func TestRepairM_632_InvalidOperationWeights(t *testing.T) {
	op := simtypes.Operation(func(*rand.Rand, *baseapp.BaseApp, sdk.Context, []simtypes.Account, string) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		return simtypes.OperationMsg{}, nil, nil
	})
	maxInt := int(^uint(0) >> 1)
	var typedNil *WeightedOperation
	cases := []struct {
		name    string
		ops     WeightedOperations
		message string
	}{
		{"empty", nil, "total must be positive"},
		{"disabled", WeightedOperations{NewWeightedOperation(0, nil)}, "total must be positive"},
		{"negative", WeightedOperations{NewWeightedOperation(2, op), NewWeightedOperation(-1, op)}, "operation 1: negative"},
		{"overflow", WeightedOperations{NewWeightedOperation(maxInt, op), NewWeightedOperation(1, op)}, "overflows"},
		{"nil selected", WeightedOperations{NewWeightedOperation(1, nil)}, "nil operation"},
		{"nil weighted", WeightedOperations{nil}, "nil weighted operation"},
		{"typed nil", WeightedOperations{typedNil}, "nil weighted operation"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			config := repair632Config(t)
			config.Operations = tc.ops
			_, err := Simulate(t, nil, config)
			require.ErrorContains(t, err, tc.message, "weights must fail before application execution")
		})
	}
	selectOp, err := (WeightedOperations{NewWeightedOperation(0, nil), NewWeightedOperation(1, op), NewWeightedOperation(0, nil)}).getSelectOpFn()
	require.NoError(t, err)
	require.NotNil(t, selectOp(rand.New(rand.NewSource(1))))
}

func TestRepairM_632_WeightedSelectionBoundary(t *testing.T) {
	for _, weights := range [][]int{{1, 1}, {0, 1, 1}, {1, 0, 1}, {1, 1, 0}, {0, 2, 0, 3, 0}} {
		t.Run(fmt.Sprint(weights), func(t *testing.T) {
			var ops WeightedOperations
			total := 0
			for i, weight := range weights {
				index := i
				var op simtypes.Operation
				if weight > 0 {
					op = func(*rand.Rand, *baseapp.BaseApp, sdk.Context, []simtypes.Account, string) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
						return simtypes.OperationMsg{Name: fmt.Sprint(index)}, nil, nil
					}
				}
				ops = append(ops, NewWeightedOperation(weight, op))
				total += weight
			}
			selector, err := ops.getSelectOpFn()
			require.NoError(t, err)
			seen := make(map[int]bool)
			for seed := int64(0); seed < 256; seed++ {
				draw := rand.New(rand.NewSource(seed)).Intn(total)
				seen[draw] = true
				expected := 0
				for draw >= weights[expected] {
					draw -= weights[expected]
					expected++
				}
				op := selector(rand.New(rand.NewSource(seed)))
				require.NotNil(t, op)
				msg, _, err := op(nil, nil, sdk.Context{}, nil, "")
				require.NoError(t, err)
				require.Equal(t, fmt.Sprint(expected), msg.Name)
			}
			require.Len(t, seen, total, "deterministic draws must cover every half-open interval boundary")
		})
	}
}

func TestRepairM_632_LostTimedOperations(t *testing.T) {
	app, key := repair632App(t)
	config := repair632Config(t)
	config.NumBlocks = 4
	config.BeginBlock = func(ctx sdk.Context, req abci.RequestBeginBlock) error {
		ctx.KVStore(key).Set([]byte("height"), []byte(fmt.Sprint(req.Header.Height)))
		return nil
	}
	var future []simtypes.FutureOperation
	// Force repeated slice growth, interleave deadlines, and retain FIFO for ties.
	for i := 0; i < 48; i++ {
		index := i
		due := config.Genesis.Time.Add(time.Duration(1+i%3) * AverageBlockTime)
		future = append(future, simtypes.FutureOperation{BlockTime: due, Op: func(_ *rand.Rand, _ *baseapp.BaseApp, ctx sdk.Context, _ []simtypes.Account, _ string) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
			require.False(t, ctx.BlockTime().Before(due))
			store := ctx.KVStore(key)
			store.Set([]byte("executed"), append(store.Get([]byte("executed")), []byte(fmt.Sprintf("%d,", index))...))
			return simtypes.OperationMsg{Name: fmt.Sprint(index), OK: true}, nil, nil
		}})
	}
	heightOp := func(_ *rand.Rand, _ *baseapp.BaseApp, ctx sdk.Context, _ []simtypes.Account, _ string) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		require.Equal(t, int64(2), ctx.BlockHeight())
		ctx.KVStore(key).Set([]byte("height-operation"), []byte("done"))
		return simtypes.OperationMsg{Name: "height-operation", OK: true}, nil, nil
	}
	config.Operations = WeightedOperations{NewWeightedOperation(1, func(_ *rand.Rand, _ *baseapp.BaseApp, ctx sdk.Context, _ []simtypes.Account, _ string) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		if ctx.BlockHeight() == 1 {
			return simtypes.OperationMsg{Name: "schedule", OK: true}, append(future, simtypes.FutureOperation{BlockHeight: 2, BlockTime: config.Genesis.Time.Add(100 * time.Hour), Op: heightOp}), nil
		}
		return simtypes.OperationMsg{Name: "tick", OK: true}, nil, nil
	})}
	result, err := Simulate(t, app, config)
	require.NoError(t, err)
	require.Equal(t, 53, result.Operations)
	require.Equal(t, 4, result.Blocks)
	require.Equal(t, int64(4), app.LastBlockHeight())
	indices := make([]int, 48)
	for i := range indices {
		indices[i] = i
	}
	sort.SliceStable(indices, func(i, j int) bool { return indices[i]%3 < indices[j]%3 })
	var expected strings.Builder
	for _, i := range indices {
		fmt.Fprintf(&expected, "%d,", i)
	}
	query := app.NewUncachedContext(false, tmproto.Header{ChainID: config.Genesis.ChainId, Height: app.LastBlockHeight()})
	require.Equal(t, int64(4), query.BlockHeight())
	require.Equal(t, config.Genesis.ChainId, string(query.KVStore(key).Get([]byte("genesis-chain"))))
	require.Equal(t, expected.String(), string(query.KVStore(key).Get([]byte("executed"))))
	require.Equal(t, "done", string(query.KVStore(key).Get([]byte("height-operation"))))
	data, err := os.ReadFile(result.LogPath)
	require.NoError(t, err)
	require.Equal(t, 49, bytes.Count(data, []byte(`"entry_kind":"queued_msg"`)))
}

func TestRepairM_632_MissingSimulationDriver(t *testing.T) {
	app, key := repair632App(t)
	config := repair632Config(t)
	config.Params.evidenceFraction = .9
	config.Genesis.InitialHeight = 40
	app.SetEndBlocker(func(ctx sdk.Context, req abci.RequestEndBlock) abci.ResponseEndBlock {
		ctx.KVStore(key).Set([]byte("ended"), []byte(fmt.Sprint(req.Height)))
		power := int64(25)
		if req.Height >= 41 {
			power = 0
		}
		if req.Height == 42 {
			return abci.ResponseEndBlock{}
		}
		return abci.ResponseEndBlock{ValidatorUpdates: []abci.ValidatorUpdate{repair632Validator(power)}}
	})
	config.BeginBlock = func(ctx sdk.Context, req abci.RequestBeginBlock) error {
		require.Equal(t, ctx.BlockHeight(), req.Header.Height)
		require.Equal(t, req.LastCommitInfo.Votes, ctx.VoteInfos())
		if ctx.BlockHeight() == 40 {
			require.Equal(t, int64(10), req.LastCommitInfo.Votes[0].Validator.Power)
		}
		if ctx.BlockHeight() == 41 {
			require.Equal(t, int64(25), req.LastCommitInfo.Votes[0].Validator.Power)
		}
		if ctx.BlockHeight() == 42 {
			require.Empty(t, req.LastCommitInfo.Votes)
		}
		for _, ev := range req.ByzantineValidators {
			require.Equal(t, req.LastCommitInfo.Votes[0].Validator.Power, ev.TotalVotingPower)
		}
		ctx.KVStore(key).Set([]byte("begun"), []byte(fmt.Sprint(ctx.BlockHeight())))
		return nil
	}
	config.Operations = WeightedOperations{NewWeightedOperation(1, func(_ *rand.Rand, actual *baseapp.BaseApp, ctx sdk.Context, _ []simtypes.Account, chain string) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		require.Same(t, app, actual)
		require.Equal(t, config.Genesis.ChainId, chain)
		require.Equal(t, fmt.Sprint(ctx.BlockHeight()), string(ctx.KVStore(key).Get([]byte("begun"))))
		ctx.KVStore(key).Set([]byte("operation"), []byte(fmt.Sprint(ctx.BlockHeight())))
		return simtypes.OperationMsg{Name: "real-store-operation", OK: true}, nil, nil
	})}
	result, err := Simulate(t, app, config)
	require.NoError(t, err)
	require.Equal(t, 3, result.Blocks)
	require.Equal(t, 3, result.Operations)
	require.Equal(t, int64(42), app.LastBlockHeight())
	require.Equal(t, 1, result.Events["end_block"]["validator_updates"]["updated"])
	require.Equal(t, 1, result.Events["end_block"]["validator_updates"]["kicked"])
	query := app.NewUncachedContext(false, tmproto.Header{ChainID: config.Genesis.ChainId, Height: app.LastBlockHeight()})
	require.Equal(t, int64(42), query.BlockHeight())
	require.Equal(t, config.Genesis.ChainId, string(query.KVStore(key).Get([]byte("genesis-chain"))))
	for _, name := range []string{"begun", "ended", "operation"} {
		require.Equal(t, "42", string(query.KVStore(key).Get([]byte(name))))
	}
	data, err := os.ReadFile(result.LogPath)
	require.NoError(t, err)
	require.Equal(t, 3, bytes.Count(data, []byte(`"entry_kind":"begin_block"`)))
	require.Equal(t, 3, bytes.Count(data, []byte(`"entry_kind":"end_block"`)))
}

func TestRepairM_632_DriverReportsErrors(t *testing.T) {
	sentinel := errors.New("application operation failed")
	app, _ := repair632App(t)
	config := repair632Config(t)
	config.BeginBlock = func(sdk.Context, abci.RequestBeginBlock) error { return nil }
	config.Operations = WeightedOperations{NewWeightedOperation(1, func(*rand.Rand, *baseapp.BaseApp, sdk.Context, []simtypes.Account, string) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		return simtypes.OperationMsg{}, nil, sentinel
	})}
	result, err := Simulate(t, app, config)
	require.ErrorIs(t, err, sentinel)
	require.Zero(t, result.Blocks)
	require.NotEmpty(t, result.LogPath)
	_, err = os.ReadFile(result.LogPath)
	require.NoError(t, err)
	app, _ = repair632App(t)
	config = repair632Config(t)
	config.BeginBlock = func(sdk.Context, abci.RequestBeginBlock) error { return sentinel }
	config.Operations = WeightedOperations{NewWeightedOperation(1, func(*rand.Rand, *baseapp.BaseApp, sdk.Context, []simtypes.Account, string) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		t.Fatal("operation must not run after begin error")
		return simtypes.OperationMsg{}, nil, nil
	})}
	file, err := os.CreateTemp(t.TempDir(), "blocked-directory")
	require.NoError(t, err)
	require.NoError(t, file.Close())
	config.LogDirectory = file.Name()
	result, err = Simulate(t, app, config)
	require.ErrorIs(t, err, sentinel)
	var pathErr *os.PathError
	require.ErrorAs(t, err, &pathErr, "driver must preserve the log export failure alongside application failure")
	require.Empty(t, result.LogPath)
}

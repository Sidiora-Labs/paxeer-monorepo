package common_test

import (
	"io"
	"math/big"
	"os"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/addr"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestRunAndCalculateGasWritesNoStdout(t *testing.T) {
	k := &testkeeper.EVMTestApp.EvmKeeper
	ctx, _ := testkeeper.EVMTestApp.GetContextForDeliverTx(nil).CacheContext()
	stateDB := state.NewDBImpl(ctx, k, false)
	precompile, err := addr.NewPrecompile(testkeeper.EVMTestApp.GetPrecompileKeepers())
	require.NoError(t, err)

	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	stdout := os.Stdout
	t.Cleanup(func() {
		os.Stdout = stdout
		writer.Close()
		reader.Close()
	})
	type capture struct {
		data []byte
		err  error
	}
	captured := make(chan capture, 1)
	go func() {
		data, readErr := io.ReadAll(reader)
		captured <- capture{data: data, err: readErr}
	}()

	os.Stdout = writer
	ret, remainingGas, callErr := precompile.RunAndCalculateGas(
		&vm.EVM{StateDB: stateDB}, ethcommon.Address{}, ethcommon.Address{},
		[]byte{0x01}, 100_000, big.NewInt(0), nil, false, false,
	)
	os.Stdout = stdout
	require.NoError(t, writer.Close())
	output := <-captured

	require.NoError(t, output.err)
	require.Empty(t, output.data, "failed precompile call wrote to stdout: %q", output.data)
	require.ErrorIs(t, callErr, vm.ErrExecutionReverted)
	require.EqualError(t, stateDB.GetPrecompileError(), "input too short to extract method ID")
	require.Nil(t, ret)
	require.Zero(t, remainingGas)
}

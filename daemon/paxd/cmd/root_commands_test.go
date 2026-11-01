package cmd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRootCommandListsSequencerCommands(t *testing.T) {
	rootCmd, _ := NewRootCmd()
	for _, path := range [][]string{
		{"sequencer"},
		{"sequencer", "gen-receipt-key"},
		{"add-genesis-receipt-key"},
		{"add-genesis-sequencer-leader"},
	} {
		t.Run(strings.Join(path, " "), func(t *testing.T) {
			command, remaining, err := rootCmd.Find(path)
			require.NoError(t, err)
			require.NotNil(t, command)
			require.Empty(t, remaining)
			require.Equal(t, rootCmd.Name()+" "+strings.Join(path, " "), command.CommandPath())
		})
	}
}

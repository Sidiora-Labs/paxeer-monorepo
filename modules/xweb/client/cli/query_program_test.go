package cli_test

import (
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/client/cli"
)

func TestQueryProgramResultCmdRefusesAShortProgramId(t *testing.T) {
	programID := "0x" + strings.Repeat("ab", 32)
	for _, constructor := range []struct {
		name   string
		newCmd func() *cobra.Command
	}{
		{"program-result", cli.NewQueryProgramResultCmd},
		{"program-request", cli.NewQueryProgramRequestCmd},
	} {
		t.Run(constructor.name, func(t *testing.T) {
			for _, tc := range []struct {
				name string
				args []string
				want string
			}{
				{"short", []string{"0x01", "1"}, "the program id must be exactly 32 bytes (got 1)"},
				{"31 bytes", []string{"0x" + strings.Repeat("ab", 31), "1"}, "the program id must be exactly 32 bytes (got 31)"},
				{"33 bytes", []string{programID + "ab", "1"}, "the program id must be exactly 32 bytes (got 33)"},
				{"empty", []string{"0x", "1"}, "the program id must be exactly 32 bytes (got 0)"},
				{"missing prefix", []string{strings.Repeat("ab", 32), "1"}, "the program id must be 0x-prefixed"},
				{"invalid hex", []string{"0x" + strings.Repeat("zz", 32), "1"}, "the program id"},
				{"odd hex", []string{programID + "a", "1"}, "the program id"},
				{"negative request", []string{programID, "-1"}, "the request id"},
				{"overflow request", []string{programID, "18446744073709551616"}, "the request id"},
				{"fractional request", []string{programID, "1.5"}, "the request id"},
				{"nondecimal request", []string{programID, "0x01"}, "the request id"},
				{"empty request", []string{programID, ""}, "the request id"},
				{"missing request", []string{programID}, "accepts 2 arg(s), received 1"},
				{"extra argument", []string{programID, "1", "extra"}, "accepts 2 arg(s), received 3"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					cmd := constructor.newCmd()
					cmd.SetOut(io.Discard)
					cmd.SetErr(io.Discard)
					cmd.SetArgs(append([]string{"--"}, tc.args...))
					err := cmd.Execute()
					require.ErrorContains(t, err, tc.want)
					if strings.Contains(tc.want, "exactly 32 bytes") {
						require.EqualError(t, err, tc.want)
					}
				})
			}
		})
	}

	t.Run("counters rejects arguments", func(t *testing.T) {
		cmd := cli.NewQueryProgramCountersCmd()
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"extra"})
		require.Error(t, cmd.Execute())
	})
}

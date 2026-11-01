package cli_test

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/client/cli"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
)

func TestAgentQueryCmdArgs(t *testing.T) {
	root := cli.GetQueryCmd()
	require.Equal(t, types.ModuleName, root.Name())
	names := make([]string, 0, len(root.Commands()))
	for _, cmd := range root.Commands() {
		names = append(names, cmd.Name())
		require.NotNil(t, cmd.RunE, cmd.Name())
		for _, flag := range []string{flags.FlagNode, flags.FlagHeight, flags.FlagOutput} {
			require.NotNil(t, cmd.Flags().Lookup(flag), "%s --%s", cmd.Name(), flag)
		}
	}
	require.ElementsMatch(t, []string{"params", "job", "pending"}, names)

	creator := "0x52908400098527886E0F7030069857D2E4169EE7"
	for _, tc := range []struct {
		name    string
		command string
		args    []string
		want    string
		parse   error
	}{
		{name: "params no args", command: "params"},
		{name: "params extra", command: "params", args: []string{"extra"}, want: "unknown command \"extra\" for \"layerxagent params\""},
		{name: "job missing", command: "job", want: "accepts 1 arg(s), received 0"},
		{name: "job extra", command: "job", args: []string{"1", "2"}, want: "accepts 1 arg(s), received 2"},
		{name: "job zero", command: "job", args: []string{"0"}},
		{name: "job one", command: "job", args: []string{"1"}},
		{name: "job max uint64", command: "job", args: []string{"18446744073709551615"}},
		{name: "job leading zeros", command: "job", args: []string{"0010"}},
		{name: "job empty", command: "job", args: []string{""}, parse: strconv.ErrSyntax},
		{name: "job text", command: "job", args: []string{"abc"}, parse: strconv.ErrSyntax},
		{name: "job negative", command: "job", args: []string{"-1"}, parse: strconv.ErrSyntax},
		{name: "job fraction", command: "job", args: []string{"1.5"}, parse: strconv.ErrSyntax},
		{name: "job hex", command: "job", args: []string{"0x01"}, parse: strconv.ErrSyntax},
		{name: "job whitespace", command: "job", args: []string{" 1"}, parse: strconv.ErrSyntax},
		{name: "job overflow", command: "job", args: []string{"18446744073709551616"}, parse: strconv.ErrRange},
		{name: "pending missing", command: "pending", want: "accepts 1 arg(s), received 0"},
		{name: "pending extra", command: "pending", args: []string{creator, creator}, want: "accepts 1 arg(s), received 2"},
		{name: "pending checksummed", command: "pending", args: []string{creator}},
		{name: "pending lowercase", command: "pending", args: []string{strings.ToLower(creator)}},
		{name: "pending zero address", command: "pending", args: []string{"0x" + strings.Repeat("0", 40)}},
		{name: "pending empty", command: "pending", args: []string{""}, want: "address"},
		{name: "pending missing prefix", command: "pending", args: []string{creator[2:]}, want: "address"},
		{name: "pending short", command: "pending", args: []string{"0x01"}, want: "address"},
		{name: "pending long", command: "pending", args: []string{creator + "00"}, want: "address"},
		{name: "pending nonhex", command: "pending", args: []string{"0x" + strings.Repeat("z", 40)}, want: "address"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := cli.GetQueryCmd()
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			args := []string{tc.command, "--node", "", "--"}
			cmd.SetArgs(append(args, tc.args...))
			err := cmd.Execute()
			switch {
			case tc.parse != nil:
				var parseErr *strconv.NumError
				require.ErrorAs(t, err, &parseErr)
				require.ErrorIs(t, err, tc.parse)
				require.Equal(t, "ParseUint", parseErr.Func)
				require.Equal(t, tc.args[0], parseErr.Num)
				require.ErrorContains(t, err, "the job id")
			case tc.want == "address":
				require.EqualError(t, err, fmt.Sprintf("the creator %q must be a 0x-prefixed 20-byte hex address", tc.args[0]))
			case tc.want != "":
				require.EqualError(t, err, tc.want)
			default:
				require.EqualError(t, err, "no RPC client is defined in offline mode")
			}
		})
	}
}

package cli_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"testing"

	"github.com/spf13/cobra"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/client/cli"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
)

func TestQueryRequestCmdRefusesANonIntegerId(t *testing.T) {
	for _, newCmd := range []func() *cobra.Command{cli.NewQueryRequestCmd, cli.NewQueryResultCmd} {
		t.Run(newCmd().Name(), func(t *testing.T) {
			for _, tc := range []struct {
				name string
				args []string
				want error
			}{
				{name: "non-integer", args: []string{"abc"}, want: strconv.ErrSyntax},
				{name: "empty", args: []string{""}, want: strconv.ErrSyntax},
				{name: "fraction", args: []string{"1.5"}, want: strconv.ErrSyntax},
				{name: "negative", args: []string{"--", "-1"}, want: strconv.ErrSyntax},
				{name: "overflow", args: []string{"18446744073709551616"}, want: strconv.ErrRange},
				{name: "missing", args: []string{}},
				{name: "extra", args: []string{"1", "2"}},
			} {
				t.Run(tc.name, func(t *testing.T) {
					cmd := newCmd()
					cmd.SetOut(io.Discard)
					cmd.SetErr(io.Discard)
					cmd.SetArgs(tc.args)
					clientCtx := client.Context{}
					err := cmd.ExecuteContext(context.WithValue(context.Background(), client.ClientContextKey, &clientCtx))
					if err == nil {
						t.Fatal("expected an argument error without a node")
					}
					if tc.want != nil {
						var parseErr *strconv.NumError
						if !errors.As(err, &parseErr) || !errors.Is(err, tc.want) {
							t.Fatalf("expected uint64 parse error %v, got %v", tc.want, err)
						}
						input := tc.args[len(tc.args)-1]
						if parseErr.Func != "ParseUint" || parseErr.Num != input {
							t.Fatalf("unexpected parse error: %#v", parseErr)
						}
					} else if want := fmt.Sprintf("accepts 1 arg(s), received %d", len(tc.args)); err.Error() != want {
						t.Fatalf("expected argument-count error %q, got %v", want, err)
					}
				})
			}
		})
	}
}
